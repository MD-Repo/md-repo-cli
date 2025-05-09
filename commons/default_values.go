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
	tcpBufferSizeStringDefault      string                     = "1MB"

	// iRODS configuration
	// Prod
	mdRepoHost            string = "data.cyverse.org"
	mdRepoPort            int    = 1247
	mdRepoZone            string = "iplant"
	mdRepoUser            string = "md-uploader"
	mdRepoUserPassword    string = ""
	mdRepoWebDAVServerURL string = "https://data.cyverse.org"
	mdRepoWebDAVPrefix    string = "/dav-anon"

	mdRepoHome        string = "/" + mdRepoZone + "/home/shared/mdrepo/prod"
	mdRepoLandingPath string = mdRepoHome + "/landing"
	mdRepoReleasePath string = mdRepoHome + "/release"

	//mdRepoURL string = "http://128.196.65.71:8000"
	mdRepoURL               string = "https://mdrepo.org"
	mdRepoGetTicketApi      string = "/api/v1/get_ticket"
	mdRepoVerifyMetadataApi string = "/api/v1/verify_metadata"

	submissionStatusFilename   string = "mdrepo-submission.%s.json"
	SubmissionMetadataFilename string = "mdrepo-metadata.toml"
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
