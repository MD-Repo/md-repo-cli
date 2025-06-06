package commons

import (
	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_conn "github.com/cyverse/go-irodsclient/irods/connection"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	"golang.org/x/xerrors"
)

// GetIRODSFSClient returns a file system client
func GetIRODSFSClient(account *irodsclient_types.IRODSAccount) (*irodsclient_fs.FileSystem, error) {
	fsConfig := irodsclient_fs.NewFileSystemConfig(ClientProgramName)

	// set tcp buffer size
	fsConfig.MetadataConnection.TcpBufferSize = GetDefaultTCPBufferSize()
	fsConfig.IOConnection.TcpBufferSize = GetDefaultTCPBufferSize()

	return irodsclient_fs.NewFileSystem(account, fsConfig)
}

// GetIRODSFSClientForLargeFileIO returns a file system client
func GetIRODSFSClientForLargeFileIO(account *irodsclient_types.IRODSAccount, maxIOConnection int, tcpBufferSize int) (*irodsclient_fs.FileSystem, error) {
	fsConfig := irodsclient_fs.NewFileSystemConfig(ClientProgramName)

	// max connection for io
	if maxIOConnection < irodsclient_fs.FileSystemIOConnectionMaxNumberDefault {
		maxIOConnection = irodsclient_fs.FileSystemIOConnectionMaxNumberDefault
	}
	fsConfig.IOConnection.MaxNumber = maxIOConnection

	// set tcp buffer size
	fsConfig.MetadataConnection.TcpBufferSize = tcpBufferSize
	fsConfig.IOConnection.TcpBufferSize = tcpBufferSize

	return irodsclient_fs.NewFileSystem(account, fsConfig)
}

// GetIRODSConnection returns a connection
func GetIRODSConnection(account *irodsclient_types.IRODSAccount) (*irodsclient_conn.IRODSConnection, error) {
	connConfig := irodsclient_conn.IRODSConnectionConfig{
		ApplicationName: ClientProgramName,
	}

	conn, err := irodsclient_conn.NewIRODSConnection(account, &connConfig)
	if err != nil {
		return nil, xerrors.Errorf("failed to create a connection: %w", err)
	}

	err = conn.Connect()
	if err != nil {
		return nil, xerrors.Errorf("failed to connect: %w", err)
	}

	return conn, nil
}
