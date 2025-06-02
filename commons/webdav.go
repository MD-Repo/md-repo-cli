package commons

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/avast/retry-go"
	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_common "github.com/cyverse/go-irodsclient/irods/common"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	irodsclient_util "github.com/cyverse/go-irodsclient/irods/util"
	log "github.com/sirupsen/logrus"
	"github.com/studio-b12/gowebdav"
	"golang.org/x/xerrors"
)

func GetWebDAVPathForIRODSPath(irodsPath string, ticket string) string {
	return mdRepoWebDAVPrefix + irodsPath + "?ticket=" + ticket
}

func DownloadFileWebDAV(sourceEntry *irodsclient_fs.Entry, localPath string, ticket string, callback irodsclient_common.TrackerCallBack) (*irodsclient_fs.FileTransferResult, error) {
	logger := log.WithFields(log.Fields{
		"package":  "commons",
		"function": "DownloadFileWebDAV",
	})

	irodsSrcPath := irodsclient_util.GetCorrectIRODSPath(sourceEntry.Path)
	localDestPath := irodsclient_util.GetCorrectLocalPath(localPath)

	localFilePath := localDestPath

	fileTransferResult := &irodsclient_fs.FileTransferResult{}
	fileTransferResult.IRODSPath = irodsSrcPath
	fileTransferResult.StartTime = time.Now()

	stat, err := os.Stat(localDestPath)
	if err != nil {
		if os.IsNotExist(err) {
			// file not exists, it's a file
			// pass
		} else {
			return fileTransferResult, err
		}
	} else {
		if stat.IsDir() {
			irodsFileName := irodsclient_util.GetIRODSPathFileName(irodsSrcPath)
			localFilePath = filepath.Join(localDestPath, irodsFileName)
		}
	}

	fileTransferResult.LocalPath = localFilePath
	fileTransferResult.IRODSCheckSumAlgorithm = sourceEntry.CheckSumAlgorithm
	fileTransferResult.IRODSCheckSum = sourceEntry.CheckSum
	fileTransferResult.IRODSSize = sourceEntry.Size

	if len(sourceEntry.CheckSum) == 0 {
		return fileTransferResult, xerrors.Errorf("failed to get checksum of the source file for path %q", irodsSrcPath)
	}

	client := gowebdav.NewClient(mdRepoWebDAVServerURL, "anonymous", "")
	err = client.Connect()
	if err != nil {
		return fileTransferResult, xerrors.Errorf("failed to connect to WebDAV server: %w", err)
	}

	// download the file
	webdavPath := GetWebDAVPathForIRODSPath(irodsSrcPath, ticket)

	offset := int64(0)
	readSize := sourceEntry.Size

	for offset < sourceEntry.Size {
		download := func() error {
			readSize = sourceEntry.Size - offset

			logger.Debugf("downloading file %s (offset %d, length %d) from WebDAV server", webdavPath, offset, readSize)

			reader, readErr := client.ReadStreamRange(webdavPath, offset, readSize)
			if readErr != nil {
				return xerrors.Errorf("failed to read stream range of file %s (offset %d, length %d) from WebDAV server: %w", webdavPath, offset, readSize, readErr)
			}
			defer reader.Close()

			newOffset, downloadErr := downloadToLocalWithTrackerCallBack(reader, localFilePath, offset, readSize, sourceEntry.Size, callback)
			if downloadErr != nil {
				logger.WithError(downloadErr).Debugf("failed to download file %s (offset %d, length %d) from WebDAV server", webdavPath, offset, readSize)

				// if the download failed, we need to update the offset
				offset = newOffset
				return xerrors.Errorf("failed to download file %s (offset %d, length %d) from WebDAV server: %w", webdavPath, offset, readSize, downloadErr)
			}

			offset = newOffset
			return nil
		}

		// retry download in case of failure
		// we retry 3 times with 5 seconds delay between attempts
		retryErr := retry.Do(download, retry.Attempts(3), retry.Delay(5*time.Second), retry.LastErrorOnly(true))
		if retryErr != nil {
			return fileTransferResult, xerrors.Errorf("failed to download file %s (offset %d, length %d) from WebDAV server after 3 attempts: %w", webdavPath, offset, readSize, retryErr)
		}
	}

	fileTransferResult.LocalSize = offset

	localHash, err := calculateLocalFileHash(localPath, sourceEntry.CheckSumAlgorithm)
	if err != nil {
		return fileTransferResult, xerrors.Errorf("failed to calculate hash of local file %s with alg %s: %w", localPath, sourceEntry.CheckSumAlgorithm, err)
	}

	fileTransferResult.LocalCheckSumAlgorithm = sourceEntry.CheckSumAlgorithm
	fileTransferResult.LocalCheckSum = localHash

	if !bytes.Equal(sourceEntry.CheckSum, localHash) {
		return fileTransferResult, xerrors.Errorf("checksum verification failed, download failed")
	}

	fileTransferResult.EndTime = time.Now()

	return fileTransferResult, nil
}

func calculateLocalFileHash(localPath string, algorithm irodsclient_types.ChecksumAlgorithm) ([]byte, error) {
	// verify checksum
	hashBytes, err := irodsclient_util.HashLocalFile(localPath, string(algorithm))
	if err != nil {
		return nil, xerrors.Errorf("failed to get %q hash of %q: %w", algorithm, localPath, err)
	}

	return hashBytes, nil
}

func downloadToLocalWithTrackerCallBack(reader io.ReadCloser, localPath string, offset int64, readLength int64, fileSize int64, callback irodsclient_common.TrackerCallBack) (int64, error) {
	f, err := os.OpenFile(localPath, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		return offset, xerrors.Errorf("failed to open local file %s: %w", localPath, err)
	}
	defer f.Close()

	newOffset, err := f.Seek(offset, io.SeekStart)
	if err != nil {
		return offset, xerrors.Errorf("failed to seek to offset %d in local file %s: %w", offset, localPath, err)
	}

	if newOffset != offset {
		return offset, xerrors.Errorf("failed to seek to offset %d in local file %s, current offset is %d", offset, localPath, newOffset)
	}

	if callback != nil {
		callback(offset, fileSize)
	}

	sizeLeft := readLength
	actualRead := int64(0)
	actualWrite := int64(0)

	buffer := make([]byte, 64*1024) // 64KB buffer
	for sizeLeft > 0 {
		sizeRead, err := reader.Read(buffer)

		if sizeRead > 0 {
			sizeLeft -= int64(sizeRead)
			actualRead += int64(sizeRead)

			sizeWritten, writeErr := f.Write(buffer[:sizeRead])
			if writeErr != nil {
				return offset + actualWrite, xerrors.Errorf("failed to write to local file %s: %w", localPath, writeErr)
			}

			if sizeWritten != sizeRead {
				return offset + actualWrite, xerrors.Errorf("failed to write all bytes to local file %s, expected %d, got %d", localPath, sizeRead, sizeWritten)
			}

			actualWrite += int64(sizeWritten)

			if callback != nil {
				callback(offset+actualWrite, fileSize)
			}
		}

		if err != nil {
			if err == io.EOF {
				break
			}

			return offset + actualWrite, xerrors.Errorf("failed to read from reader: %w", err)
		}
	}

	if actualWrite != readLength {
		return offset + actualWrite, xerrors.Errorf("file size mismatch: expected %d, got %d", readLength, actualRead)
	}

	return offset + actualWrite, nil
}
