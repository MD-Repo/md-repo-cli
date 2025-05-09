package commons

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"time"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_common "github.com/cyverse/go-irodsclient/irods/common"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	irodsclient_util "github.com/cyverse/go-irodsclient/irods/util"
	"github.com/studio-b12/gowebdav"
	"golang.org/x/xerrors"
)

func GetWebDAVPathForIRODSPath(irodsPath string, ticket string) string {
	return mdRepoWebDAVPrefix + irodsPath + "?ticket=" + ticket
}

func DownloadFileWebDAV(sourceEntry *irodsclient_fs.Entry, localPath string, ticket string, callback irodsclient_common.TrackerCallBack) (*irodsclient_fs.FileTransferResult, error) {
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
	streamReader, err := client.ReadStream(webdavPath)
	if err != nil {
		return fileTransferResult, xerrors.Errorf("failed to read file %s from WebDAV server: %w", webdavPath, err)
	}
	defer streamReader.Close()

	actualWritten, err := downloadToLocalWithTrackerCallBack(streamReader, localFilePath, sourceEntry.Size, callback)
	if err != nil {
		return fileTransferResult, xerrors.Errorf("failed to download file %s from WebDAV server: %w", webdavPath, err)
	}

	fileTransferResult.LocalSize = actualWritten

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

func downloadToLocalWithTrackerCallBack(reader io.ReadCloser, localPath string, fileSize int64, callback irodsclient_common.TrackerCallBack) (int64, error) {
	f, err := os.Create(localPath)
	if err != nil {
		return 0, xerrors.Errorf("failed to create local file %s: %w", localPath, err)
	}
	defer f.Close()

	// actual download here
	if callback != nil {
		callback(0, fileSize)
	}

	sizeLeft := fileSize
	actualRead := int64(0)
	actualWritten := int64(0)

	buffer := make([]byte, 64*1024) // 64KB buffer
	for sizeLeft > 0 {
		sizeRead, err := reader.Read(buffer)

		if sizeRead > 0 {
			sizeLeft -= int64(sizeRead)
			actualRead += int64(sizeRead)

			sizeWritten, writeErr := f.Write(buffer[:sizeRead])
			if writeErr != nil {
				return actualWritten, xerrors.Errorf("failed to write to local file %s: %w", localPath, writeErr)
			}

			actualWritten += int64(sizeWritten)

			if callback != nil {
				callback(actualRead, fileSize)
			}
		}

		if err != nil {
			if err == io.EOF {
				break
			}

			return actualWritten, xerrors.Errorf("failed to read from reader: %w", err)
		}
	}

	if actualWritten != fileSize {
		return actualWritten, xerrors.Errorf("file size mismatch: expected %d, got %d", fileSize, actualWritten)
	}

	return actualWritten, nil
}
