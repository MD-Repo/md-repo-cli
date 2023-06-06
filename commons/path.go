package commons

import (
	"os"
	"path"
	"path/filepath"
	"strings"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
)

func MakeIRODSLandingPath(irodsPath string) string {
	if strings.HasPrefix(irodsPath, mdRepoLandingPath) {
		// clean
		return path.Clean(irodsPath)
	}

	// calculate from relative path
	newPath := path.Join(mdRepoLandingPath, irodsPath)
	return path.Clean(newPath)
}

func MakeIRODSReleasePath(irodsPath string) string {
	if strings.HasPrefix(irodsPath, mdRepoReleasePath) {
		// clean
		return path.Clean(irodsPath)
	}

	// calculate from relative path
	newPath := path.Join(mdRepoReleasePath, irodsPath)
	return path.Clean(newPath)
}

func MakeLocalPath(localPath string) string {
	absLocalPath, err := filepath.Abs(localPath)
	if err != nil {
		return filepath.Clean(localPath)
	}

	return filepath.Clean(absLocalPath)
}

func MakeTargetIRODSFilePath(filesystem *irodsclient_fs.FileSystem, source string, target string) string {
	if filesystem.ExistsDir(target) {
		// make full file name for target
		filename := GetBasename(source)
		return path.Join(target, filename)
	}
	return target
}

func MakeTargetLocalFilePath(source string, target string) string {
	st, err := os.Stat(target)
	if err == nil {
		if st.IsDir() {
			// make full file name for target
			filename := GetBasename(source)
			return filepath.Join(target, filename)
		}
	}
	return target
}

func GetFileExtension(path string) string {
	base := GetBasename(path)

	idx := strings.Index(base, ".")
	if idx >= 0 {
		return path[idx:]
	}
	return path
}

func GetBasename(path string) string {
	idx1 := strings.LastIndex(path, string(os.PathSeparator))
	idx2 := strings.LastIndex(path, "/")

	if idx1 < 0 && idx2 < 0 {
		return "."
	}

	if idx1 >= idx2 {
		return path[idx1+1:]
	}
	return path[idx2+1:]
}

func ExistFile(path string) bool {
	st, err := os.Stat(path)
	if err != nil {
		return false
	}

	if !st.IsDir() {
		return true
	}
	return false
}
