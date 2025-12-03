package subcmd

import (
	"runtime"

	"github.com/MD-Repo/md-repo-cli/cmd/flag"
	"github.com/MD-Repo/md-repo-cli/commons"
	"github.com/cockroachdb/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Upgrade MD-Repo command-line tool to the latest available version",
	Long:  `This command upgrades MD-Repo command-line tool to the latest version available.`,
	RunE:  processUpgradeCommand,
	Args:  cobra.NoArgs,
}

func AddUpgradeCommand(rootCmd *cobra.Command) {
	// attach common flags
	flag.SetCommonFlags(upgradeCmd)

	flag.SetCheckVersionFlags(upgradeCmd)

	rootCmd.AddCommand(upgradeCmd)
}

func processUpgradeCommand(command *cobra.Command, args []string) error {
	upgrade, err := NewUpgradeCommand(command, args)
	if err != nil {
		return err
	}

	return upgrade.Process()
}

type UpgradeCommand struct {
	command *cobra.Command

	commonFlagValues       *flag.CommonFlagValues
	checkVersionFlagValues *flag.CheckVersionFlagValues
}

func NewUpgradeCommand(command *cobra.Command, args []string) (*UpgradeCommand, error) {
	upgrade := &UpgradeCommand{
		command: command,

		commonFlagValues:       flag.GetCommonFlagValues(command),
		checkVersionFlagValues: flag.GetCheckVersionFlagValues(),
	}

	return upgrade, nil
}

func (upgrade *UpgradeCommand) Process() error {
	cont, err := flag.ProcessCommonFlags(upgrade.command)
	if err != nil {
		return errors.Wrapf(err, "failed to process common flags")
	}

	if !cont {
		return nil
	}

	err = upgrade.upgrade(upgrade.checkVersionFlagValues.Silent, upgrade.checkVersionFlagValues.Check)
	if err != nil {
		return errors.Wrapf(err, "failed to upgrade to new release")
	}

	return nil
}

func (upgrade *UpgradeCommand) upgrade(silent bool, checkOnly bool) error {
	logger := log.WithFields(log.Fields{
		"silent":     silent,
		"check_only": checkOnly,
	})

	myVersion := commons.GetClientVersion()
	if !silent {
		logger.Infof("Current client version installed: %s\n", myVersion)
		commons.Printf("Current client version installed: %s\n", myVersion)
	}

	newRelease, err := commons.CheckNewRelease()
	if err != nil {
		return errors.Wrapf(err, "failed to check new release")
	}

	if !silent {
		logger.Infof("Latest release version available for %s/%s: v%s\n", runtime.GOOS, runtime.GOARCH, newRelease.Version())
		logger.Infof("Latest release URL: %s\n", newRelease.URL)
		commons.Printf("Latest release version available for %s/%s: v%s\n", runtime.GOOS, runtime.GOARCH, newRelease.Version())
		commons.Printf("Latest release URL: %s\n", newRelease.URL)
	}

	if commons.HasNewRelease(myVersion, newRelease.Version()) {
		logger.Infof("Found a new version v%s available\n", newRelease.Version())
		commons.Printf("Found a new version v%s available\n", newRelease.Version())
	} else {
		if !silent {
			logger.Infof("Current client version installed is up-to-date [%s]\n", myVersion)
			commons.Printf("Current client version installed is up-to-date [%s]\n", myVersion)
		}
		return nil
	}

	if checkOnly {
		return nil
	}

	commons.Printf("Upgrading to the latest version v%s\n", newRelease.Version())

	err = commons.SelfUpgrade(newRelease)
	if err != nil {
		return errors.Wrapf(err, "failed to upgrade to the new release")
	}

	commons.Printf("Upgrade from %s to v%s has done successfully!\n", myVersion, newRelease.Version())
	return nil
}
