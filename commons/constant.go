package commons

import "time"

const (
	mdRepoPackagePath string = "MD-Repo/md-repo-cli"

	clientProgramName string        = "md-repo-cli"
	connectionTimeout time.Duration = 10 * time.Minute
	filesystemTimeout time.Duration = 10 * time.Minute

	TransferTreadNumDefault    int    = 5
	UploadTreadNumMax          int    = 20
	TcpBufferSizeDefault       int    = 4 * 1024 * 1024
	TcpBufferSizeStringDefault string = "4MB"

	// iRODS configuration
	// Prod
	mdRepoHost         string = "data.cyverse.org"
	mdRepoPort         int    = 1247
	mdRepoZone         string = "iplant"
	mdRepoUser         string = "md-uploader"
	mdRepoUserPassword string = ""

	mdRepoHome        string = "/" + mdRepoZone + "/home/md-dev"
	mdRepoLandingPath string = mdRepoHome + "/landing"
	mdRepoReleasePath string = mdRepoHome + "/release"

	mdRepoGetTicketApiUrl string = "http://128.196.65.71:8000/api/get_ticket"

	submissionStatusFilename   string = "mdrepo-submission.%s.json"
	SubmissionMetadataFilename string = "mdrepo-metadata.toml"
)
