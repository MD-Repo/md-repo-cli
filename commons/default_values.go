package commons

import (
	"time"

	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
)

const (
	mdRepoPackagePath string = "MD-Repo/md-repo-cli"

	ClientProgramName               string                     = "md-repo-cli"
	FilesystemTimeout               irodsclient_types.Duration = irodsclient_types.Duration(10 * time.Minute)
	transferThreadNumDefault        int                        = 5
	transferThreadNumPerFileDefault int                        = 5
	tcpBufferSizeStringDefault      string                     = "1MiB"

	// iRODS configuration
	// Prod
	mdRepoHost            string = "data.cyverse.org"
	mdRepoPort            int    = 1247
	mdRepoZone            string = "iplant"
	mdRepoUser            string = "md-uploader"
	mdRepoUserPassword    string = ""
	mdRepoHashScheme      string = "MD5"
	mdRepoWebDAVServerURL string = "https://data.cyverse.org"
	mdRepoWebDAVPrefix    string = "/dav-anon"

	mdRepoHome        string = "/" + mdRepoZone + "/home/shared/mdrepo/prod"
	mdRepoLandingPath string = mdRepoHome + "/landing"
	mdRepoReleasePath string = mdRepoHome + "/release"

	mdRepoURL               string = "https://mdrepo.org"
	mdRepoGetTicketApi      string = "/api/v1/get_ticket"
	mdRepoVerifyMetadataApi string = "/api/v1/verify_metadata"

	submissionStatusFilename   string = "mdrepo-submission.%s.json"
	SubmissionMetadataFilename string = "mdrepo-metadata.toml"

	MaxSimulationSubmissionSize string = "40GiB"
)

func GetDefaultTCPBufferSize() int {
	size, _ := ParseSize(GetDefaultTCPBufferSizeString())
	return int(size)
}

func GetDefaultTCPBufferSizeString() string {
	return tcpBufferSizeStringDefault
}

func GetDefaultTransferThreadNum() int {
	return transferThreadNumDefault
}

func GetDefaultTransferThreadNumPerFile() int {
	return transferThreadNumPerFileDefault
}

func GetMaxSimulationSubmissionSize() int64 {
	size, _ := ParseSize(MaxSimulationSubmissionSize)
	return int64(size)
}
