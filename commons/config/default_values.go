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
	TransferThreadNumDefault        int                        = 5
	TransferThreadNumPerFileDefault int                        = 5
	TcpBufferSizeStringDefault      string                     = "0"

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

func GetDefaultTCPBufferSize() int {
	size, _ := types.ParseSize(TcpBufferSizeStringDefault)
	return int(size)
}

func GetDefaultTCPBufferSizeString() string {
	return TcpBufferSizeStringDefault
}

func GetDefaultTransferThreadNum() int {
	return TransferThreadNumDefault
}

func GetDefaultTransferThreadNumPerFile() int {
	return TransferThreadNumPerFileDefault
}

func GetMaxSimulationSubmissionSize() int64 {
	size, _ := types.ParseSize(MaxSimulationSubmissionSize)
	return int64(size)
}
