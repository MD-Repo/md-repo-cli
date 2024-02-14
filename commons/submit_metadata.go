package commons

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/go-multierror"
	log "github.com/sirupsen/logrus"
	"golang.org/x/xerrors"
)

type MDRepoVarifySubmitMetadataRequest struct {
	LocalDataDirPath string `json:"directory"`
	MetadataTOML     string `json:"toml"`
	Token            string `json:"token"`
}

type MDRepoVarifySubmitMetadataResponse struct {
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

func ReadOrcIDFromSubmitMetadataString(metadataString string) (string, error) {
	// we will not use toml parsers as they are not stable
	metadataLines := strings.Split(metadataString, "\n")
	for _, metadataLine := range metadataLines {
		metadataKV := strings.Split(metadataLine, "=")
		if len(metadataKV) == 2 {
			key := strings.ToLower(strings.TrimSpace(metadataKV[0]))
			if key == "lead_contributor_orcid" || key == "primary_contributor_orcid" {
				return strings.Trim(strings.TrimSpace(metadataKV[1]), "\"'"), nil
			}
		}
	}

	return "", xerrors.Errorf("unable to find 'lead_contributor_orcid' node")
}

func ReadOrcIDFromSubmitMetadataFile(filePath string) (string, error) {
	metadataBytes, err := os.ReadFile(filePath)
	if err != nil {
		return "", xerrors.Errorf("failed to read submission metadata at %s: %w", filePath, err)
	}

	return ReadOrcIDFromSubmitMetadataString(string(metadataBytes))
}

func ReadOrcIDFromSubmitMetadataFileInDir(dirPath string) (string, error) {
	metadataPath := GetSubmitMetadataPath(dirPath)
	return ReadOrcIDFromSubmitMetadataFile(metadataPath)
}

func VerifySubmitMetadata(sourcePaths []string, serviceURL string, token string) error {
	logger := log.WithFields(log.Fields{
		"package":  "commons",
		"function": "VerifySubmitMetadata",
	})

	apiURL := mdRepoURL + mdRepoVerifyMetadataApi
	if len(serviceURL) > 0 {
		if !strings.HasPrefix(serviceURL, "http") {
			return xerrors.Errorf("failed to make API endpoint URL from non-http/s URL '%s'", serviceURL)
		}

		apiURL = strings.TrimRight(serviceURL, "/") + mdRepoVerifyMetadataApi
	}

	logger.Debugf("Requesting to API server at '%s'", apiURL)

	req, err := http.NewRequest("POST", apiURL, nil)
	if err != nil {
		return xerrors.Errorf("failed to create a new request to verify submit metadata: %w", err)
	}

	verifyRequests := []MDRepoVarifySubmitMetadataRequest{}
	for _, sourcePath := range sourcePaths {
		metadataPath := filepath.Join(sourcePath, SubmissionMetadataFilename)
		metadataBytes, err := os.ReadFile(metadataPath)
		if err != nil {
			return xerrors.Errorf("failed to read submit metadata %s: %w", metadataPath, err)
		}

		verifyRequest := MDRepoVarifySubmitMetadataRequest{
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
		return xerrors.Errorf("failed to verify submit metadata, http error %s", resp.Status)
	}

	verifyResponseBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return xerrors.Errorf("failed to verify submit metadata, read failed: %w", err)
	}

	verifyResponses := []MDRepoVarifySubmitMetadataResponse{}
	err = json.Unmarshal(verifyResponseBytes, &verifyResponses)
	if err != nil {
		return xerrors.Errorf("failed to unmarshal submit metadata verify response from JSON: %w", err)
	}

	var verifyError error
	valid := true
	for _, verifyResponse := range verifyResponses {
		if !verifyResponse.Valid {
			if verifyResponse.Errors != nil && len(verifyResponse.Errors) > 0 {
				// error
				for _, verifyRespnseError := range verifyResponse.Errors {
					verifyErrorObj := xerrors.Errorf("%s, path %s: %w", verifyRespnseError, verifyResponse.LocalDataDirPath, InvalidSubmitMetadataError)
					logger.Error(verifyErrorObj)

					verifyError = multierror.Append(verifyError, verifyErrorObj)
				}
			} else {
				verifyErrorObj := xerrors.Errorf("invalid submit metadata, path %s: %w", verifyResponse.LocalDataDirPath, InvalidSubmitMetadataError)
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
