package commons

import (
	"encoding/json"
	"fmt"
	"strings"
)

type SubmitStatus string

const (
	SubmitStatusUnknown    SubmitStatus = ""
	SubmitStatusInProgress SubmitStatus = "inprogress"
	SubmitStatusErrored    SubmitStatus = "errored"
	SubmitStatusCompleted  SubmitStatus = "completed"
)

func (s SubmitStatus) String() string {
	return string(s)
}

func (s SubmitStatus) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

func (s *SubmitStatus) UnmarshalJSON(b []byte) error {
	var ss string
	err := json.Unmarshal(b, &ss)
	if err != nil {
		return err
	}

	// Validate and convert the value.
	switch strings.ToLower(ss) {
	case "":
		*s = SubmitStatusUnknown
	case string(SubmitStatusInProgress):
		*s = SubmitStatusInProgress
	case string(SubmitStatusErrored):
		*s = SubmitStatusErrored
	case string(SubmitStatusCompleted):
		*s = SubmitStatusCompleted
	default:
		return fmt.Errorf("invalid status format: %s", s)
	}

	return nil
}

type SubmitStatusFile struct {
	TotalFileNumer int64               `json:"total_filenum"`
	TotalFileSize  int64               `json:"total_filesize"`
	Status         SubmitStatus        `json:"status"`
	Files          []SubmitStatusEntry `json:"files"`
}

func NewSubmitStatusFile() *SubmitStatusFile {
	return &SubmitStatusFile{
		TotalFileNumer: 0,
		TotalFileSize:  0,
		Status:         SubmitStatusUnknown,
		Files:          []SubmitStatusEntry{},
	}
}

func (s *SubmitStatusFile) SetInProgress() {
	s.Status = SubmitStatusInProgress
}

func (s *SubmitStatusFile) SetErrored() {
	s.Status = SubmitStatusErrored
}

func (s *SubmitStatusFile) SetCompleted() {
	s.Status = SubmitStatusCompleted
}

func (s *SubmitStatusFile) AddFile(f SubmitStatusEntry) {
	s.TotalFileNumer++
	s.TotalFileSize += f.Size
	s.Files = append(s.Files, f)
}

type SubmitStatusEntry struct {
	IRODSPath string `json:"irods_path"`
	Size      int64  `json:"size"`
	MD5Hash   string `json:"md5_hash"`
}

func GetMDRepoStatusFilename() string {
	return mdRepoStatusFilename
}
