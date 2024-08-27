package commons

import (
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	"golang.org/x/xerrors"
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

func MakeTargetIRODSFilePath(filesystem *irodsclient_fs.FileSystem, source string, target string) string {
	if filesystem.ExistsDir(target) {
		// make full file name for target
		filename := GetBasename(source)
		return path.Join(target, filename)
	}
	return target
}

func MakeTargetLocalFilePath(source string, target string) string {
	realTarget, err := ResolveSymlink(target)
	if err != nil {
		return target
	}

	st, err := os.Stat(realTarget)
	if err == nil {
		if st.IsDir() {
			// make full file name for target
			filename := GetBasename(source)
			return filepath.Join(target, filename)
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

// GetParentDirs returns all parent dirs
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

// GetParentLocalDirs returns all parent dirs
func GetParentLocalDirs(p string) []string {
	parents := []string{}

	if p == string(os.PathSeparator) || p == "." {
		return parents
	}

	curPath := p
	for len(curPath) > 0 && curPath != string(os.PathSeparator) && curPath != "." {
		curDir := filepath.Dir(curPath)
		if len(curDir) > 0 && curDir != "." {
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

func MarkPathMap(pathMap map[string]bool, p string) {
	dirs := GetParentIRODSDirs(p)
	for _, dir := range dirs {
		pathMap[dir] = true
	}

	pathMap[p] = true
}

func ResolveSymlink(p string) (string, error) {
	st, err := os.Lstat(p)
	if err != nil {
		return "", xerrors.Errorf("failed to lstat path %q: %w", p, err)
	}

	if st.Mode()&os.ModeSymlink == os.ModeSymlink {
		// symlink
		new_p, err := filepath.EvalSymlinks(p)
		if err != nil {
			return "", xerrors.Errorf("failed to evaluate symlink path %q: %w", p, err)
		}

		// follow recursively
		new_pp, err := ResolveSymlink(new_p)
		if err != nil {
			return "", xerrors.Errorf("failed to evaluate symlink path %q: %w", new_p, err)
		}

		return new_pp, nil
	}
	return p, nil
}
