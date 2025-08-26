package commons

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/hashicorp/go-multierror"
	log "github.com/sirupsen/logrus"
	"golang.org/x/xerrors"
)

type MDRepoSubmitMetadata struct {
	MetadataFilePath string `toml:"-"`
	SubmissionPath   string `toml:"-"`

	Initial map[string]interface{} `toml:"initial"`

	RequiredFiles   map[string]string   `toml:"required_files"`
	AdditionalFiles []map[string]string `toml:"additional_files"`
}

type MDRepoVerifySubmitMetadataRequest struct {
	LocalDataDirPath string `json:"directory"`
	MetadataTOML     string `json:"toml"`
	Token            string `json:"token"`
}

type MDRepoVerifySubmitMetadataResponse struct {
	LocalDataDirPath string   `json:"directory"`
	Valid            bool     `json:"valid"`
	Errors           []string `json:"errors"`
}

func GetSubmitMetadataPath(dirPath string) string {
	return filepath.Join(dirPath, SubmissionMetadataFilename)
}

func HasSubmitMetadataInDir(dirPath string) bool {
	metadataPath := GetSubmitMetadataPath(dirPath)
	metadataStat, err := os.Stat(metadataPath)
	if err == nil {
		if !metadataStat.IsDir() && metadataStat.Size() > 0 {
			return true
		}
		return false
	}

	return false
}

func ParseSubmitMetadataFile(filePath string) (*MDRepoSubmitMetadata, error) {
	metadata := MDRepoSubmitMetadata{
		MetadataFilePath: filePath,
		SubmissionPath:   filepath.Dir(filePath),
	}

	_, err := toml.DecodeFile(filePath, &metadata)
	if err != nil {
		return nil, xerrors.Errorf("failed to parse submission metadata at %q: %w", filePath, err)
	}

	return &metadata, nil
}

func ParseSubmitMetadataDir(dirPath string) (*MDRepoSubmitMetadata, error) {
	metadata := MDRepoSubmitMetadata{
		MetadataFilePath: filepath.Join(dirPath, SubmissionMetadataFilename),
		SubmissionPath:   dirPath,
	}

	_, err := toml.DecodeFile(metadata.MetadataFilePath, &metadata)
	if err != nil {
		return nil, xerrors.Errorf("failed to parse submission metadata at %q: %w", metadata.MetadataFilePath, err)
	}

	return &metadata, nil
}

func ParseSubmitMetadataString(metadataString string) (*MDRepoSubmitMetadata, error) {
	metadata := MDRepoSubmitMetadata{
		MetadataFilePath: "",
		SubmissionPath:   "",
	}

	_, err := toml.Decode(metadataString, &metadata)
	if err != nil {
		return nil, xerrors.Errorf("failed to parse submission metadata: %w", err)
	}

	return &metadata, nil
}

func (meta *MDRepoSubmitMetadata) GetOrcID() (string, error) {
	if leadContributorOrcID, ok := meta.Initial["lead_contributor_orcid"]; ok {
		return leadContributorOrcID.(string), nil
	}

	if primaryContributorOrcID, ok := meta.Initial["primary_contributor_orcid"]; ok {
		return string(primaryContributorOrcID.(string)), nil
	}

	return "", xerrors.Errorf("no ORCID found")
}

func (meta *MDRepoSubmitMetadata) hasLocalFile(filePath string) bool {
	st, err := os.Stat(filePath)
	if err == nil {
		return !st.IsDir()
	}

	return !os.IsNotExist(err)
}

func (meta *MDRepoSubmitMetadata) ValidateFiles() error {
	logger := log.WithFields(log.Fields{
		"package":  "commons",
		"struct":   "MDRepoSubmitMetadata",
		"function": "ValidateFiles",
	})

	var allErrors error

	hasTrajectory := false
	hasStructure := false
	hasTopology := false

	for filekey, file := range meta.RequiredFiles {
		if filekey == "trajectory_file_name" {
			hasTrajectory = true
		}
		if filekey == "structure_file_name" {
			hasStructure = true
		}
		if filekey == "topology_file_name" {
			hasTopology = true
		}

		if !meta.hasLocalFile(file) {
			errObj := xerrors.Errorf("required file %q - %q not found: %w", filekey, filepath.Join(meta.SubmissionPath, file), InvalidSubmitMetadataError)
			logger.Error(errObj)
			allErrors = multierror.Append(allErrors, errObj)
		}
	}

	if !hasTrajectory {
		errObj := xerrors.Errorf("field 'trajectory_file_name' not found: %w", InvalidSubmitMetadataError)
		logger.Error(errObj)
		allErrors = multierror.Append(allErrors, errObj)
	}

	if !hasStructure {
		errObj := xerrors.Errorf("field 'structure_file_name' not found: %w", InvalidSubmitMetadataError)
		logger.Error(errObj)
		allErrors = multierror.Append(allErrors, errObj)
	}

	if !hasTopology {
		errObj := xerrors.Errorf("field 'topology_file_name' not found: %w", InvalidSubmitMetadataError)
		logger.Error(errObj)
		allErrors = multierror.Append(allErrors, errObj)
	}

	for _, additionalFile := range meta.AdditionalFiles {
		for filekey, file := range additionalFile {
			if filekey == "additional_file_name" {
				if !meta.hasLocalFile(file) {
					errObj := xerrors.Errorf("additional file %q - %q not found: %w", filekey, filepath.Join(meta.SubmissionPath, file), InvalidSubmitMetadataError)
					logger.Error(errObj)
					allErrors = multierror.Append(allErrors, errObj)
				}
			}
		}
	}

	if allErrors != nil {
		return xerrors.Errorf("failed to validate required and additional files listed in submission metadata: %w", allErrors)
	}

	return nil
}

func (meta *MDRepoSubmitMetadata) GetFiles() []string {
	var files []string
	for _, v := range meta.RequiredFiles {
		files = append(files, v)
	}

	for _, additionalFile := range meta.AdditionalFiles {
		for k, v := range additionalFile {
			if k == "additional_file_name" {
				files = append(files, v)
			}
		}
	}
	return files
}

func VerifySubmitMetadataViaServer(sourcePaths []string, serviceURL string, token string) error {
	logger := log.WithFields(log.Fields{
		"package":  "commons",
		"function": "VerifySubmitMetadataViaServer",
	})

	apiURL := mdRepoURL + mdRepoVerifyMetadataApi
	if len(serviceURL) > 0 {
		if !strings.HasPrefix(serviceURL, "http") {
			return xerrors.Errorf("failed to make API endpoint URL from non-http/s URL %q", serviceURL)
		}

		apiURL = strings.TrimRight(serviceURL, "/") + mdRepoVerifyMetadataApi
	}

	logger.Debugf("Requesting to API server at %q", apiURL)

	req, err := http.NewRequest("POST", apiURL, nil)
	if err != nil {
		return xerrors.Errorf("failed to create a new request to verify submit metadata: %w", err)
	}

	verifyRequests := []MDRepoVerifySubmitMetadataRequest{}
	for _, sourcePath := range sourcePaths {
		metadataPath := filepath.Join(sourcePath, SubmissionMetadataFilename)
		metadataBytes, err := os.ReadFile(metadataPath)
		if err != nil {
			return xerrors.Errorf("failed to read submit metadata %q: %w", metadataPath, err)
		}

		verifyRequest := MDRepoVerifySubmitMetadataRequest{
			LocalDataDirPath: sourcePath,
			MetadataTOML:     string(metadataBytes),
			Token:            token,
		}

		verifyRequests = append(verifyRequests, verifyRequest)
	}

	verifyRequestsJSON, err := json.Marshal(verifyRequests)
	if err != nil {
		return xerrors.Errorf("failed to marshal submit metadata verify request to JSON: %w", err)
	}

	verifyRequestsJSONString := string(verifyRequestsJSON)

	req.Body = io.NopCloser(strings.NewReader(verifyRequestsJSONString))
	req.Header.Add("Accept", "*/*")
	req.Header.Add("Content-Type", "text/plain")
	req.ContentLength = int64(len(verifyRequestsJSONString))

	client := &http.Client{}
	transport := &http.Transport{
		Proxy:              http.ProxyFromEnvironment,
		DisableCompression: true,
	}
	client.Transport = transport

	resp, err := client.Do(req)
	if err != nil {
		return xerrors.Errorf("failed to perform http post to verify submit metadata: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return xerrors.Errorf("failed to verify submit metadata, http error %q", resp.Status)
	}

	verifyResponseBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return xerrors.Errorf("failed to verify submit metadata, read failed: %w", err)
	}

	verifyResponses := []MDRepoVerifySubmitMetadataResponse{}
	err = json.Unmarshal(verifyResponseBytes, &verifyResponses)
	if err != nil {
		return xerrors.Errorf("failed to unmarshal submit metadata verify response from JSON: %w", err)
	}

	var verifyError error
	valid := true
	for _, verifyResponse := range verifyResponses {
		if !verifyResponse.Valid {
			if len(verifyResponse.Errors) > 0 {
				// error
				for _, verifyRespnseError := range verifyResponse.Errors {
					verifyErrorObj := xerrors.Errorf("%s, path %q: %w", verifyRespnseError, verifyResponse.LocalDataDirPath, InvalidSubmitMetadataError)
					logger.Error(verifyErrorObj)

					verifyError = multierror.Append(verifyError, verifyErrorObj)
				}
			} else {
				verifyErrorObj := xerrors.Errorf("invalid submit metadata, path %q: %w", verifyResponse.LocalDataDirPath, InvalidSubmitMetadataError)
				logger.Error(verifyErrorObj)

				verifyError = multierror.Append(verifyError, verifyErrorObj)
			}

			valid = false
		}
	}

	if valid {
		return nil
	}

	return verifyError
}
