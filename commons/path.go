package commons

import (
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cockroachdb/errors"
	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	log "github.com/sirupsen/logrus"
)

func MakeIRODSLandingPath(irodsPath string) string {
	if strings.HasPrefix(irodsPath, "/") {
		// absolute path
		return path.Clean(irodsPath)
	}

	// calculate from relative path
	newPath := path.Join(mdRepoLandingPath, irodsPath)
	return path.Clean(newPath)
}

func MakeIRODSReleasePath(irodsPath string) string {
	if strings.HasPrefix(irodsPath, "/") {
		// absolute path
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

func MakeTargetIRODSFilePath(filesystem *irodsclient_fs.FileSystem, source string, target string, createSub bool) string {
	if createSub {
		if filesystem.ExistsDir(target) {
			// make full file name for target
			filename := GetBasename(source)
			return path.Join(target, filename)
		}
	}

	return target
}

func MakeTargetLocalFilePath(source string, target string, createSub bool) string {
	realTarget, err := ResolveSymlink(target)
	if err != nil {
		return target
	}

	st, err := os.Stat(realTarget)
	if err == nil {
		if createSub {
			if st.IsDir() {
				// make full file name for target
				filename := GetBasename(source)
				return filepath.Join(target, filename)
			}
		}

	}
	return target
}

func GetFileExtension(p string) string {
	base := GetBasename(p)

	idx := strings.Index(base, ".")
	if idx >= 0 {
		return p[idx:]
	}
	return p
}

func GetIRODSPathDirname(path string) string {
	p := strings.TrimRight(path, "/")
	idx := strings.LastIndex(p, "/")

	if idx < 0 {
		return p
	} else if idx == 0 {
		return "/"
	} else {
		return p[:idx]
	}
}

func GetIRODSPathBasename(path string) string {
	p := strings.TrimRight(path, "/")
	idx := strings.LastIndex(p, "/")

	if idx < 0 {
		return p
	} else {
		return p[idx+1:]
	}
}

func GetBasename(p string) string {
	p = strings.TrimRight(p, string(os.PathSeparator))
	p = strings.TrimRight(p, "/")

	idx1 := strings.LastIndex(p, string(os.PathSeparator))
	idx2 := strings.LastIndex(p, "/")

	if idx1 < 0 && idx2 < 0 {
		return p
	}

	if idx1 >= idx2 {
		return p[idx1+1:]
	}
	return p[idx2+1:]
}

// GetParentIRODSDirs returns all parent dirs
func GetParentIRODSDirs(p string) []string {
	parents := []string{}

	if p == "/" {
		return parents
	}

	curPath := p
	for len(curPath) > 0 && curPath != "/" {
		curDir := path.Dir(curPath)
		if len(curDir) > 0 {
			parents = append(parents, curDir)
		}

		curPath = curDir
	}

	// sort
	sort.Slice(parents, func(i int, j int) bool {
		return len(parents[i]) < len(parents[j])
	})

	return parents
}

// GetParentLocalDirs returns all parent dirs
func GetParentLocalDirs(p string) []string {
	logger := log.WithFields(log.Fields{
		"local_path": p,
	})

	parents := []string{}

	if p == string(os.PathSeparator) {
		return parents
	}

	absPath, _ := filepath.Abs(p)
	if filepath.Dir(absPath) == absPath {
		return parents
	}

	curPath := absPath
	logger.Infof("curPath = %s", curPath)
	for len(curPath) > 0 {
		curDir := filepath.Dir(curPath)
		if curDir == curPath {
			// root
			break
		}

		if len(curDir) > 0 {
			parents = append(parents, curDir)
		}

		curPath = curDir
	}

	// sort
	sort.Slice(parents, func(i int, j int) bool {
		return len(parents[i]) < len(parents[j])
	})

	return parents
}

func FirstDelimeterIndex(p string) int {
	idx1 := strings.Index(p, string(os.PathSeparator))
	idx2 := strings.Index(p, "/")

	if idx1 < 0 && idx2 < 0 {
		return idx1
	}

	if idx1 < 0 {
		return idx2
	}

	if idx2 < 0 {
		return idx1
	}

	if idx1 <= idx2 {
		return idx1
	}

	return idx2
}

func LastDelimeterIndex(p string) int {
	idx1 := strings.LastIndex(p, string(os.PathSeparator))
	idx2 := strings.LastIndex(p, "/")

	if idx1 >= idx2 {
		return idx1
	}

	return idx2
}

func GetDir(p string) string {
	idx1 := strings.LastIndex(p, string(os.PathSeparator))
	idx2 := strings.LastIndex(p, "/")

	if idx1 < 0 && idx2 < 0 {
		return "/"
	}

	if idx1 >= idx2 {
		return p[:idx1]
	}
	return p[:idx2]
}

func commonPrefix(sep byte, paths ...string) string {
	// Handle special cases.
	switch len(paths) {
	case 0:
		return ""
	case 1:
		return path.Clean(paths[0])
	}

	c := []byte(path.Clean(paths[0]))
	c = append(c, sep)

	// Ignore the first path since it's already in c
	for _, v := range paths[1:] {
		// Clean up each path before testing it
		v = path.Clean(v) + string(sep)

		// Find the first non-common byte and truncate c
		if len(v) < len(c) {
			c = c[:len(v)]
		}
		for i := 0; i < len(c); i++ {
			if v[i] != c[i] {
				c = c[:i]
				break
			}
		}
	}

	// Remove trailing non-separator characters and the final separator
	for i := len(c) - 1; i >= 0; i-- {
		if c[i] == sep {
			c = c[:i]
			break
		}
	}

	return string(c)
}

func GetCommonRootLocalDirPath(paths []string) (string, error) {
	absPaths := make([]string, len(paths))

	// get abs paths
	for idx, path := range paths {
		absPath, err := filepath.Abs(path)
		if err != nil {
			return "", errors.Wrapf(err, "failed to compute absolute path for %q", path)
		}
		absPaths[idx] = absPath
	}

	// find shortest path
	commonRoot := commonPrefix(filepath.Separator, absPaths...)

	commonRootStat, err := os.Stat(commonRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return "", errors.Join(err, irodsclient_types.NewFileNotFoundError(commonRoot))
		}

		return "", errors.Wrapf(err, "failed to stat %q", commonRoot)
	}

	if commonRootStat.IsDir() {
		return commonRoot, nil
	}
	return filepath.Dir(commonRoot), nil
}

func ExpandHomeDir(p string) (string, error) {
	// resolve "~/"
	if p == "~" {
		homedir, err := os.UserHomeDir()
		if err != nil {
			return "", errors.Wrap(err, "failed to get user home directory")
		}

		return filepath.Abs(homedir)
	} else if strings.HasPrefix(p, "~/") {
		homedir, err := os.UserHomeDir()
		if err != nil {
			return "", errors.Wrap(err, "failed to get user home directory")
		}

		p = filepath.Join(homedir, p[2:])
		return filepath.Abs(p)
	}

	return filepath.Abs(p)
}

func ExistFile(p string) bool {
	realPath, err := ResolveSymlink(p)
	if err != nil {
		return false
	}

	st, err := os.Stat(realPath)
	if err != nil {
		return false
	}

	if !st.IsDir() {
		return true
	}
	return false
}

func MarkLocalPathMap(pathMap map[string]bool, p string) {
	dirs := GetParentLocalDirs(p)

	for _, dir := range dirs {
		pathMap[dir] = true
	}

	pathMap[p] = true
}

func MarkIRODSPathMap(pathMap map[string]bool, p string) {
	dirs := GetParentIRODSDirs(p)

	for _, dir := range dirs {
		pathMap[dir] = true
	}

	pathMap[p] = true
}

func ResolveSymlink(p string) (string, error) {
	st, err := os.Lstat(p)
	if err != nil {
		return "", errors.Wrapf(err, "failed to lstat path %q", p)
	}

	if st.Mode()&os.ModeSymlink == os.ModeSymlink {
		// symlink
		new_p, err := filepath.EvalSymlinks(p)
		if err != nil {
			return "", errors.Wrapf(err, "failed to evaluate symlink path %q", p)
		}

		// follow recursively
		new_pp, err := ResolveSymlink(new_p)
		if err != nil {
			return "", errors.Wrapf(err, "failed to evaluate symlink path %q", new_p)
		}

		return new_pp, nil
	}
	return p, nil
}
