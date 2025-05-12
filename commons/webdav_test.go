package commons

import (
	"encoding/hex"
	"os"
	"testing"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	"github.com/stretchr/testify/assert"
)

func TestWebDAV(t *testing.T) {
	t.Run("test DownloadFileFromWebDAV", testDownloadFileFromWebDAV)
}

func testDownloadFileFromWebDAV(t *testing.T) {
	checksumBytes, _ := hex.DecodeString("d8f0c00cecd46e8efc9fe283228167a2")
	sourceEntry := &irodsclient_fs.Entry{
		ID:                12345,
		Path:              "/iplant/home/iychoi/abc.txt",
		Name:              "abc.txt",
		Size:              19,
		CheckSum:          checksumBytes,
		CheckSumAlgorithm: "MD5",
	}

	localPath := "/tmp/test.txt"
	ticket := "d0h20krj1d295436l270"

	callback := func(progress int64, total int64) {
		// This is a dummy callback function
		t.Logf("Progress: %d/%d", progress, total)
	}

	transferResult, err := DownloadFileWebDAV(sourceEntry, localPath, ticket, callback)
	assert.NoError(t, err)

	os.Remove(localPath) // Clean up the test file

	assert.Equal(t, sourceEntry.Path, transferResult.IRODSPath)
	t.Log("Transfer Result:", transferResult)
}
