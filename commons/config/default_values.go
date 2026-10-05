package config

import (
	"time"

	"github.com/MD-Repo/md-repo-cli/commons/types"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
)

const (
	MDRepoPackagePath string = "MD-Repo/md-repo-cli"
	ClientProgramName string = "md-repo-cli"

	FilesystemTimeout               irodsclient_types.Duration = irodsclient_types.Duration(10 * time.Minute)
	LongFilesystemTimeout           irodsclient_types.Duration = irodsclient_types.Duration(15 * time.Minute) // exceptionally long timeout for listing dirs or users
	transferThreadNumDefault        int                        = 5
	transferThreadNumPerFileDefault int                        = 5
	tcpSendBufferSizeStringDefault  string                     = "0"
	tcpRecvBufferSizeStringDefault  string                     = "0"

	// iRODS configuration
	// Prod
	MDRepoHost            string = "data.cyverse.org"
	MDRepoPort            int    = 1247
	MDRepoZone            string = "iplant"
	MDRepoUser            string = "anonymous"
	MDRepoUserPassword    string = ""
	MDRepoHashScheme      string = "MD5"
	MDRepoWebDAVServerURL string = "https://data.cyverse.org"
	MDRepoWebDAVPrefix    string = "/dav-anon"

	mdRepoHome        string = "/" + MDRepoZone + "/home/shared/mdrepo/prod"
	MDRepoLandingPath string = mdRepoHome + "/landing"
	MDRepoReleasePath string = mdRepoHome + "/release"

	MDRepoURL               string = "https://mdrepo.org"
	MDRepoGetTicketApi      string = "/api/v1/get_ticket"
	MDRepoVerifyMetadataApi string = "/api/v1/verify_metadata"

	MaxSimulationSubmissionSize string = "40GB"
)

func GetDefaultFilesystemTimeoutInSeconds() int {
	return int(FilesystemTimeout / irodsclient_types.Duration(time.Second))
}

func GetDefaultTCPSendBufferSize() int {
	size, _ := types.ParseSize(GetDefaultTCPSendBufferSizeString())
	return int(size)
}

func GetDefaultTCPSendBufferSizeString() string {
	return tcpSendBufferSizeStringDefault
}

func GetDefaultTCPRecvBufferSize() int {
	size, _ := types.ParseSize(GetDefaultTCPRecvBufferSizeString())
	return int(size)
}

func GetDefaultTCPRecvBufferSizeString() string {
	return tcpRecvBufferSizeStringDefault
}

func GetDefaultTransferThreadNum() int {
	return transferThreadNumDefault
}

func GetDefaultTransferThreadNumPerFile() int {
	return transferThreadNumPerFileDefault
}

func GetMaxSimulationSubmissionSize() int64 {
	size, _ := types.ParseSize(MaxSimulationSubmissionSize)
	return int64(size)
}
