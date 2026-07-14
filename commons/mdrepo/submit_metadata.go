package mdrepo

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/MD-Repo/md-repo-cli/commons/config"
	"github.com/MD-Repo/md-repo-cli/commons/types"
	"github.com/cockroachdb/errors"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	log "github.com/sirupsen/logrus"
)

const (
	SubmissionStatusFilename   string = "mdrepo-submission.%s.json"
	SubmissionMetadataFilename string = "mdrepo-metadata.toml"
)

type MDRepoSubmitMetadata struct {
	MetadataFilePath string `toml:"-"`
	SubmissionPath   string `toml:"-"`

	LeadContributorOrcid string              `toml:"lead_contributor_orcid"`
	TrajectoryFileNames  []string            `toml:"trajectory_file_names"`
	StructureFileName    string              `toml:"structure_file_name"`
	TopologyFileName     string              `toml:"topology_file_name"`
	AdditionalFiles      []map[string]string `toml:"additional_files"`
}

type MDRepoVerifySubmitMetadataRequest struct {
	LocalDataDirPath string `json:"directory"`
	MetadataTOML     string `json:"toml"`
	Token            string `json:"token"`
	NoID             bool   `json:"no_id"`
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

func ValidateSubmissionSourcePath(sourcePath string) error {
	st, err := os.Stat(sourcePath)
	if err != nil {
		if os.IsNotExist(err) {
			return errors.Join(err, irodsclient_types.NewFileNotFoundError(sourcePath))
		}

		return errors.Wrapf(err, "Failed to stat source %q", sourcePath)
	}

	if !st.IsDir() {
		return types.NewNotDirError(sourcePath)
	}

	// check if source path has metadata in it
	if !HasSubmitMetadataInDir(sourcePath) {
		// metadata path not exist?
		return errors.Errorf("source %q must have submit metadata", sourcePath)
	}

	entries, err := os.ReadDir(sourcePath)
	if err != nil {
		return errors.Wrapf(err, "Failed to readdir source %q", sourcePath)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			// found dir
			return types.NewNotFileError(filepath.Join(sourcePath, entry.Name()))
		}
	}

	return nil
}

func ParseSubmitMetadataFile(filePath string) (*MDRepoSubmitMetadata, error) {
	metadata := MDRepoSubmitMetadata{
		MetadataFilePath: filePath,
		SubmissionPath:   filepath.Dir(filePath),
	}

	_, err := toml.DecodeFile(filePath, &metadata)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to parse submission metadata at %q", filePath)
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
	metadata := MDRepoSubmitMetadata{}

	_, err := toml.Decode(metadataString, &metadata)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse submission metadata")
	}

	return &metadata, nil
}

func (meta *MDRepoSubmitMetadata) GetOrcID() (string, error) {
	if meta.LeadContributorOrcid != "" {
		return meta.LeadContributorOrcid, nil
	}

	return "", errors.Errorf("missing field 'lead_contributor_orcid'")
}

func (meta *MDRepoSubmitMetadata) hasLocalFileAndReturnStat(filePath string) (bool, os.FileInfo) {
	st, err := os.Stat(filePath)
	if err == nil {
		return !st.IsDir(), st
	}

	return false, nil
}

func (meta *MDRepoSubmitMetadata) ValidateFiles() error {
	logger := log.WithFields(log.Fields{})

	invalidSubmitMetadataError := &types.InvalidSubmitMetadataError{}

	hasTrajectory := len(meta.TrajectoryFileNames) > 0
	hasStructure := false
	hasTopology := false

	totalFileSize := int64(0)

	for _, file := range meta.TrajectoryFileNames {
		absFilepath := filepath.Join(meta.SubmissionPath, file)

		st, err := os.Stat(absFilepath)
		if err != nil {
			newErr := errors.Wrapf(err, "cannot access trajectory file %q described in metadata", absFilepath)
			logger.Error(newErr)
			invalidSubmitMetadataError.Add(newErr)
			continue
		}

		if st == nil {
			newErr := errors.Errorf("cannot stat trajectory file %q described in metadata", absFilepath)
			logger.Error(newErr)
			invalidSubmitMetadataError.Add(newErr)
			continue
		}

		if st.IsDir() {
			newErr := errors.Errorf("trajectory file %q described in metadata is a directory", absFilepath)
			logger.Error(newErr)
			invalidSubmitMetadataError.Add(newErr)
			continue
		}

		totalFileSize += st.Size()
	}

	var requiredFiles = map[string]string{
		"structure_file_name": meta.StructureFileName,
		"topology_file_name":  meta.TopologyFileName,
	}

	for filekey, file := range requiredFiles {
		absFilepath := filepath.Join(meta.SubmissionPath, file)

		st, err := os.Stat(absFilepath)
		if err != nil {
			newErr := errors.Wrapf(err, "cannot access required file %q described in metadata %q", absFilepath, filekey)
			logger.Error(newErr)
			invalidSubmitMetadataError.Add(newErr)
			continue
		}

		if st == nil {
			newErr := errors.Errorf("cannot stat required file %q described in metadata %q", absFilepath, filekey)
			logger.Error(newErr)
			invalidSubmitMetadataError.Add(newErr)
			continue
		}

		if st.IsDir() {
			newErr := errors.Errorf("required file %q described in metadata %q is a directory", absFilepath, filekey)
			logger.Error(newErr)
			invalidSubmitMetadataError.Add(newErr)
			continue
		}

		totalFileSize += st.Size()
		if filekey == "trajectory_file_name" {
			hasTrajectory = true
		}
		if filekey == "structure_file_name" {
			hasStructure = true
		}
		if filekey == "topology_file_name" {
			hasTopology = true
		}
	}

	if !hasTrajectory {
		newErr := errors.Errorf("field 'trajectory_file_names' not found or empty")
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

	allFiles := []string{}
	for _, v := range meta.TrajectoryFileNames {
		allFiles = append(allFiles, v)
	}

	for _, v := range requiredFiles {
		allFiles = append(allFiles, v)
	}

	for _, additionalFile := range meta.AdditionalFiles {
		for filekey, file := range additionalFile {
			if filekey == "file_name" {
				absFilepath := filepath.Join(meta.SubmissionPath, file)

				st, err := os.Stat(absFilepath)
				if err != nil {
					newErr := errors.Wrapf(err, "cannot access additional file %q described in metadata %q", absFilepath, filekey)
					logger.Error(newErr)
					invalidSubmitMetadataError.Add(newErr)
					continue
				}

				if st == nil {
					newErr := errors.Errorf("cannot stat additional file %q described in metadata %q", absFilepath, filekey)
					logger.Error(newErr)
					invalidSubmitMetadataError.Add(newErr)
					continue
				}

				if st.IsDir() {
					newErr := errors.Errorf("additional file %q described in metadata %q is a directory", absFilepath, filekey)
					logger.Error(newErr)
					invalidSubmitMetadataError.Add(newErr)
					continue
				}

				totalFileSize += st.Size()
				allFiles = append(allFiles, file)
			}
		}
	}

	allFilesMap := map[string]bool{}
	for _, f := range allFiles {
		if _, ok := allFilesMap[f]; ok {
			// exist
			newErr := errors.Errorf("the file %q is used multiple times in the metadata", f)
			logger.Error(newErr)
			invalidSubmitMetadataError.Add(newErr)
		} else {
			// add
			allFilesMap[f] = true
		}
	}

	maxSimulationSize := config.GetMaxSimulationSubmissionSize()
	if totalFileSize > maxSimulationSize {
		totalFileSizeString := types.SizeString(int64(totalFileSize))
		maxSimulationSizeString := types.SizeString(int64(maxSimulationSize))

		newErr := errors.Errorf("total size of each simulation must not exceed %s (%d bytes), current %s (%d bytes)", maxSimulationSizeString, maxSimulationSize, totalFileSizeString, totalFileSize)
		logger.Error(newErr)
		invalidSubmitMetadataError.Add(newErr)
	}

	if invalidSubmitMetadataError.ErrorLen() > 0 {
		return errors.Wrapf(invalidSubmitMetadataError, "failed to validate required and additional files listed in submission metadata")
	}

	return nil
}

func (meta *MDRepoSubmitMetadata) GetFiles() []string {
	files := append([]string{}, meta.TrajectoryFileNames...)
	files = append(files, meta.StructureFileName, meta.TopologyFileName)

	for _, additionalFile := range meta.AdditionalFiles {
		for k, v := range additionalFile {
			if k == "file_name" {
				files = append(files, v)
			}
		}
	}
	return files
}

func VerifySubmitMetadataViaServer(sourcePaths []string, serviceURL string, token string, noID bool) error {
	logger := log.WithFields(log.Fields{
		"source_paths": sourcePaths,
		"service_url":  serviceURL,
		"token":        token,
	})

	apiURL := config.MDRepoURL + config.MDRepoVerifyMetadataApi
	if len(serviceURL) > 0 {
		if !strings.HasPrefix(serviceURL, "http") {
			return errors.Errorf("failed to make API endpoint URL from non-http/s URL %q", serviceURL)
		}

		apiURL = strings.TrimRight(serviceURL, "/") + config.MDRepoVerifyMetadataApi
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
			NoID:             noID,
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

	verifyErrors := &types.InvalidSubmitMetadataError{}
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
