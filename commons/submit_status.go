package commons

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/cyverse/go-irodsclient/fs"
)

type SubmitStatus string

const (
	SubmitStatusUnknown    SubmitStatus = "unknown"
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
	case "", string(SubmitStatusUnknown):
		*s = SubmitStatusUnknown
	case string(SubmitStatusInProgress):
		*s = SubmitStatusInProgress
	case string(SubmitStatusErrored):
		*s = SubmitStatusErrored
	case string(SubmitStatusCompleted):
		*s = SubmitStatusCompleted
	default:
		return errors.Errorf("invalid status format %q", s)
	}

	return nil
}

type SubmitStatusFile struct {
	TotalFileNumer int64               `json:"total_filenum"`
	TotalFileSize  int64               `json:"total_filesize"`
	Token          string              `json:"token"`
	Status         SubmitStatus        `json:"status"`
	Files          []SubmitStatusEntry `json:"files"`
	Time           time.Time           `json:"time"`
}

func NewSubmitStatusFile() *SubmitStatusFile {
	return &SubmitStatusFile{
		TotalFileNumer: 0,
		TotalFileSize:  0,
		Token:          "",
		Status:         SubmitStatusUnknown,
		Files:          []SubmitStatusEntry{},
		Time:           time.Now().UTC(),
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

func getAllAvailableStatusFilenames() []string {
	status := []SubmitStatus{SubmitStatusUnknown, SubmitStatusInProgress, SubmitStatusErrored, SubmitStatusCompleted}
	names := []string{}

	for _, st := range status {
		names = append(names, getStatusFilename(st))
	}

	return names
}

func (s *SubmitStatusFile) GetStatusFilename() string {
	return getStatusFilename(s.Status)
}

func getStatusFilename(status SubmitStatus) string {
	return fmt.Sprintf(submissionStatusFilename, status)
}

func (s *SubmitStatusFile) CreateStatusFile(filesystem *fs.FileSystem, dataRootPath string) error {
	statusFileName := s.GetStatusFilename()
	statusFilePath := MakeTargetIRODSFilePath(filesystem, statusFileName, dataRootPath, true)

	jsonBytes, err := json.Marshal(s)
	if err != nil {
		return errors.Wrapf(err, "failed to marshal submit status file to json")
	}

	// Note: We cannot remove old status files. Ticket does not support delete/move/rename operations
	// remove old status files
	//existingDirEntries, err := filesystem.List(dataRootPath)
	//if err != nil {
	//	return errors.Wrapf(err, "failed to list target directory")
	//}

	//for _, existingDirEntry := range existingDirEntries {
	//	if IsStatusFile(existingDirEntry.Name) {
	//		err = filesystem.RemoveFile(existingDirEntry.Path, true)
	//		if err != nil {
	//			return errors.Wrapf(err, "failed to delete stale submit status file %q", existingDirEntry.Path)
	//		}
	//	}
	//}

	// upload
	jsonBytesBuffer := bytes.Buffer{}
	_, err = jsonBytesBuffer.Write(jsonBytes)
	if err != nil {
		return errors.Wrapf(err, "failed to write submit status to buffer")
	}

	// we do not truncate status file as it should be empty
	_, err = filesystem.UploadFileFromBuffer(&jsonBytesBuffer, statusFilePath, "", false, true, true, nil)
	if err != nil {
		return errors.Wrapf(err, "failed to create submit status file %q", statusFilePath)
	}

	return nil
}

func IsStatusFile(filename string) bool {
	availableStatusFilenames := getAllAvailableStatusFilenames()
	for _, availableStatusFilename := range availableStatusFilenames {
		if filename == availableStatusFilename {
			return true
		}
	}

	return false
}

type SubmitStatusEntry struct {
	IRODSPath string `json:"irods_path"`
	Size      int64  `json:"size"`
	MD5Hash   string `json:"md5_hash"`
}
