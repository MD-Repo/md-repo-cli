package commons

import (
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/xerrors"
)

func HasSubmitMetadataInDir(dirPath string) bool {
	metadataPath := filepath.Join(dirPath, SubmissionMetadataFilename)
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
	metadataPath := filepath.Join(dirPath, SubmissionMetadataFilename)
	return ReadOrcIDFromSubmitMetadataFile(metadataPath)
}
