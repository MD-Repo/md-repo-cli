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

	aesIV                        string = "4e2f34041d564ed8"
	aesPadding                   string = "671ff9e1f816451b"
	pbkdf2SHA256HasherSalt       string = "8675309"
	pbkdf2SHA256HasherIterations int    = 260000

	mdRepoStatusFilename string = "MD_REPO_STATUS.json"
)
