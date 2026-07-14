package path

import (
	"path"
	"strings"

	"github.com/MD-Repo/md-repo-cli/commons/config"
)

func MakeIRODSLandingPath(irodsPath string) string {
	if strings.HasPrefix(irodsPath, "/") {
		// absolute path
		return path.Clean(irodsPath)
	}

	// calculate from relative path
	newPath := path.Join(config.MDRepoLandingPath, irodsPath)
	return path.Clean(newPath)
}

func MakeIRODSReleasePath(irodsPath string) string {
	if strings.HasPrefix(irodsPath, "/") {
		// absolute path
		return path.Clean(irodsPath)
	}

	// calculate from relative path
	newPath := path.Join(config.MDRepoReleasePath, irodsPath)
	return path.Clean(newPath)
}
