package commons

import (
	"context"
	"os"
	"runtime"

	"github.com/cockroachdb/errors"
	selfupdate "github.com/creativeprojects/go-selfupdate"
)

func CheckNewRelease() (*selfupdate.Release, error) {
	latest, found, err := selfupdate.DetectLatest(context.Background(), selfupdate.ParseSlug(mdRepoPackagePath))
	if err != nil {
		return nil, errors.Wrapf(err, "error occurred while detecting version")
	}

	if !found {
		return nil, errors.Errorf("latest version for %s/%s is not found from github repository %q", runtime.GOOS, runtime.GOARCH, mdRepoPackagePath)
	}

	return latest, nil
}

func SelfUpgrade(release *selfupdate.Release) error {
	exe, err := os.Executable()
	if err != nil {
		return errors.Errorf("failed to locate executable path")
	}

	if err := selfupdate.UpdateTo(context.Background(), release.AssetURL, release.AssetName, exe); err != nil {
		return errors.Wrapf(err, "error occurred while updating binary")
	}

	return nil
}
