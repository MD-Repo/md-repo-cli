package commons

import "time"

const (
	mdRepoPackagePath string = "MD-Repo/md-repo-cli"

	clientProgramName string        = "md-repo-cli"
	connectionTimeout time.Duration = 10 * time.Minute
	filesystemTimeout time.Duration = 10 * time.Minute

	TransferThreadNumDefault   int    = 5
	UploadThreadNumMax         int    = 20
	TcpBufferSizeDefault       int    = 4 * 1024 * 1024
	TcpBufferSizeStringDefault string = "4MB"

	RedirectToResourceMinSize int64 = 1024 * 1024 * 1024 // 1GB
	ParallelUploadMinSize     int64 = 80 * 1024 * 1024   // 80MB

	// iRODS configuration
	// Prod
	mdRepoHost         string = "data.cyverse.org"
	mdRepoPort         int    = 1247
	mdRepoZone         string = "iplant"
	mdRepoUser         string = "md-uploader"
	mdRepoUserPassword string = ""

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
