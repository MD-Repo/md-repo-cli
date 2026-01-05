package commons

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/cockroachdb/errors"
	log "github.com/sirupsen/logrus"
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
		return nil, errors.Wrapf(err, "failed to parse submission metadata at %q", filePath)
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
		return nil, errors.Wrapf(err, "failed to parse submission metadata at %q", metadata.MetadataFilePath)
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
		return nil, errors.Wrapf(err, "failed to parse submission metadata")
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

	return "", errors.Errorf("no ORCID found")
}

func (meta *MDRepoSubmitMetadata) hasLocalFileAndReturnStat(filePath string) (bool, os.FileInfo) {
	st, err := os.Stat(filePath)
	if err == nil {
		return !st.IsDir(), st
	}

	return !os.IsNotExist(err), nil
}

func (meta *MDRepoSubmitMetadata) ValidateFiles() error {
	logger := log.WithFields(log.Fields{})

	invalidSubmitMetadataError := &InvalidSubmitMetadataError{}

	hasTrajectory := false
	hasStructure := false
	hasTopology := false

	totalFileSize := int64(0)

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

		absFilepath := filepath.Join(meta.SubmissionPath, file)

		fileExist, stat := meta.hasLocalFileAndReturnStat(absFilepath)
		if fileExist {
			if stat != nil {
				totalFileSize += stat.Size()
			}
		} else {
			newErr := errors.Errorf("required file %q described in metadata %q not found", filekey, absFilepath)
			logger.Error(newErr)
			invalidSubmitMetadataError.Add(newErr)
		}
	}

	if !hasTrajectory {
		newErr := errors.Errorf("field 'trajectory_file_name' not found")
		logger.Error(newErr)
		invalidSubmitMetadataError.Add(newErr)
	}

	if !hasStructure {
		newErr := errors.Errorf("field 'structure_file_name' not found")
		logger.Error(newErr)
		invalidSubmitMetadataError.Add(newErr)
	}

	if !hasTopology {
		newErr := errors.Errorf("field 'topology_file_name' not found")
		logger.Error(newErr)
		invalidSubmitMetadataError.Add(newErr)
	}

	for _, additionalFile := range meta.AdditionalFiles {
		for filekey, file := range additionalFile {
			if filekey == "additional_file_name" {
				absFilepath := filepath.Join(meta.SubmissionPath, file)

				fileExist, stat := meta.hasLocalFileAndReturnStat(absFilepath)
				if fileExist {
					if stat != nil {
						totalFileSize += stat.Size()
					}
				} else {
					newErr := errors.Errorf("additional file %q described in metadata %q not found", filekey, absFilepath)
					logger.Error(newErr)
					invalidSubmitMetadataError.Add(newErr)
				}
			}
		}
	}

	maxSimulationSize := GetMaxSimulationSubmissionSize()
	if totalFileSize > maxSimulationSize {
		newErr := errors.Errorf("total size of each simulation must not exceed %d bytes, current %d", maxSimulationSize, totalFileSize)
		logger.Error(newErr)
		invalidSubmitMetadataError.Add(newErr)
	}

	if invalidSubmitMetadataError.ErrorLen() > 0 {
		return errors.Wrapf(invalidSubmitMetadataError, "failed to validate required and additional files listed in submission metadata")
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
		"source_paths": sourcePaths,
		"service_url":  serviceURL,
		"token":        token,
	})

	apiURL := mdRepoURL + mdRepoVerifyMetadataApi
	if len(serviceURL) > 0 {
		if !strings.HasPrefix(serviceURL, "http") {
			return errors.Errorf("failed to make API endpoint URL from non-http/s URL %q", serviceURL)
		}

		apiURL = strings.TrimRight(serviceURL, "/") + mdRepoVerifyMetadataApi
	}

	logger.Debugf("Requesting to API server at %q", apiURL)

	req, err := http.NewRequest("POST", apiURL, nil)
	if err != nil {
		return errors.Wrapf(err, "failed to create a new request to verify submit metadata")
	}

	verifyRequests := []MDRepoVerifySubmitMetadataRequest{}
	for _, sourcePath := range sourcePaths {
		metadataPath := filepath.Join(sourcePath, SubmissionMetadataFilename)
		metadataBytes, err := os.ReadFile(metadataPath)
		if err != nil {
			return errors.Wrapf(err, "failed to read submit metadata %q", metadataPath)
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
		return errors.Wrapf(err, "failed to marshal submit metadata verify request to JSON")
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
		return errors.Wrapf(err, "failed to perform http post to verify submit metadata")
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return errors.Errorf("failed to verify submit metadata, http error %q", resp.Status)
	}

	verifyResponseBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return errors.Wrapf(err, "failed to verify submit metadata, read failed")
	}

	verifyResponses := []MDRepoVerifySubmitMetadataResponse{}
	err = json.Unmarshal(verifyResponseBytes, &verifyResponses)
	if err != nil {
		return errors.Wrapf(err, "failed to unmarshal submit metadata verify response from JSON")
	}

	verifyErrors := &InvalidSubmitMetadataError{}
	valid := true
	for _, verifyResponse := range verifyResponses {
		if !verifyResponse.Valid {
			if len(verifyResponse.Errors) > 0 {
				// error
				for _, verifyResponseError := range verifyResponse.Errors {
					newErr := errors.Errorf("%s, path %q", verifyResponseError, verifyResponse.LocalDataDirPath)
					logger.Error(newErr)
					verifyErrors.Add(newErr)
				}
			} else {
				newErr := errors.Errorf("invalid submit metadata, path %q", verifyResponse.LocalDataDirPath)
				logger.Error(newErr)
				verifyErrors.Add(newErr)
			}

			valid = false
		}
	}

	if valid {
		return nil
	}

	if verifyErrors.ErrorLen() == 0 {
		newErr := errors.Errorf("submit metadata verification failed with unknown error")
		logger.Error(newErr)
		verifyErrors.Add(newErr)
	}

	return errors.Wrapf(verifyErrors, "submit metadata verification failed")
}
