package mdrepo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	common_path "github.com/MD-Repo/md-repo-cli/commons/path"
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

func getAllAvailableStatusFilenames() []string {
	status := []SubmitStatus{SubmitStatusUnknown, SubmitStatusInProgress, SubmitStatusErrored, SubmitStatusCompleted}
	names := []string{}

	for _, st := range status {
		names = append(names, getStatusFilename(st))
	}

	return names
}

func getStatusFilename(status SubmitStatus) string {
	return fmt.Sprintf(SubmissionStatusFilename, status)
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

type SubmitStatusFileWriter struct {
	FileSystem      *fs.FileSystem
	DataRootPath    string
	Token           string
	TotalFileNumber int64
	TotalFileSize   int64
	Status          SubmitStatus
	Files           []SubmitStatusEntry
	Time            time.Time
}

func NewSubmitStatusFileWriter(filesystem *fs.FileSystem, token string, dataRootPath string) *SubmitStatusFileWriter {
	return &SubmitStatusFileWriter{
		FileSystem:   filesystem,
		Token:        token,
		DataRootPath: dataRootPath,
		Status:       SubmitStatusUnknown,
		Files:        []SubmitStatusEntry{},
		Time:         time.Time{},
	}
}

func (s *SubmitStatusFileWriter) SetInProgress() {
	s.Status = SubmitStatusInProgress
	s.Time = time.Now().UTC()
}

func (s *SubmitStatusFileWriter) SetErrored() {
	s.Status = SubmitStatusErrored
	s.Time = time.Now().UTC()
}

func (s *SubmitStatusFileWriter) SetCompleted() {
	s.Status = SubmitStatusCompleted
	s.Time = time.Now().UTC()
}

func (s *SubmitStatusFileWriter) AddFile(f SubmitStatusEntry) {
	s.TotalFileNumber++
	s.TotalFileSize += f.Size
	s.Files = append(s.Files, f)
}

type SubmitStatusEntry struct {
	IRODSPath string `json:"irods_path"`
	Size      int64  `json:"size"`
	MD5Hash   string `json:"md5_hash"`
}

type SubmitStatusFile struct {
	TotalFileNumber int64               `json:"total_filenum"`
	TotalFileSize   int64               `json:"total_filesize"`
	Status          SubmitStatus        `json:"status"`
	Token           string              `json:"token"`
	Files           []SubmitStatusEntry `json:"files"`
	Time            time.Time           `json:"time"`
}

func (s *SubmitStatusFileWriter) GetStatusFilename() string {
	return getStatusFilename(s.Status)
}

func (s *SubmitStatusFileWriter) CreateStatusFile() error {
	statusFileName := s.GetStatusFilename()
	statusFilePath := common_path.MakeIRODSTargetFilePath(s.FileSystem, statusFileName, s.DataRootPath)

	f := SubmitStatusFile{
		TotalFileNumber: s.TotalFileNumber,
		TotalFileSize:   s.TotalFileSize,
		Status:          s.Status,
		Token:           s.Token,
		Files:           s.Files,
		Time:            s.Time,
	}

	jsonBytes, err := json.Marshal(f)
	if err != nil {
		return errors.Wrapf(err, "failed to marshal submit status file to json")
	}

	// Note: We cannot remove old status files. Ticket does not support delete/move/rename operations
	// remove old status files
	//existingDirEntries, err := s.FileSystem.List(s.DataRootPath)
	//if err != nil {
	//	return errors.Wrapf(err, "failed to list target directory")
	//}

	//for _, existingDirEntry := range existingDirEntries {
	//	if IsStatusFile(existingDirEntry.Name) {
	//		err = s.FileSystem.RemoveFile(existingDirEntry.Path, true)
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
	_, err = s.FileSystem.UploadFileFromBuffer(&jsonBytesBuffer, statusFilePath, "", false, false, nil)
	if err != nil {
		return errors.Wrapf(err, "failed to create submit status file %q", statusFilePath)
	}

	return nil
}
