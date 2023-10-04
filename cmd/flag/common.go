package flag

import (
	"fmt"

	"github.com/MD-Repo/md-repo-cli/commons"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

type CommonFlagValues struct {
	ShowVersion     bool
	ShowHelp        bool
	DebugMode       bool
	logLevelInput   string
	LogLevel        log.Level
	LogLevelUpdated bool
}

var (
	commonFlagValues CommonFlagValues
)

func SetCommonFlags(command *cobra.Command) {
	command.Flags().BoolVarP(&commonFlagValues.ShowVersion, "version", "v", false, "Print version")
	command.Flags().BoolVarP(&commonFlagValues.ShowHelp, "help", "h", false, "Print help")
	command.Flags().BoolVarP(&commonFlagValues.DebugMode, "debug", "d", false, "Enable debug mode")
	command.Flags().StringVar(&commonFlagValues.logLevelInput, "log_level", "", "Set log level")

	command.MarkFlagsMutuallyExclusive("debug", "version")
	command.MarkFlagsMutuallyExclusive("log_level", "version")
}

func GetCommonFlagValues() *CommonFlagValues {
	if len(commonFlagValues.logLevelInput) > 0 {
		lvl, err := log.ParseLevel(commonFlagValues.logLevelInput)
		if err != nil {
			lvl = log.InfoLevel
		}
		commonFlagValues.LogLevel = lvl
		commonFlagValues.LogLevelUpdated = true
	}

	return &commonFlagValues
}

func ProcessCommonFlags(command *cobra.Command) (bool, error) {
	myCommonFlagValues := GetCommonFlagValues()
	retryFlagValues := GetRetryFlagValues()

	if myCommonFlagValues.DebugMode {
		log.SetLevel(log.DebugLevel)
	} else {
		if myCommonFlagValues.LogLevelUpdated {
			log.SetLevel(myCommonFlagValues.LogLevel)
		}
	}

	if myCommonFlagValues.ShowHelp {
		command.Usage()
		return false, nil // stop here
	}

	if myCommonFlagValues.ShowVersion {
		printVersion()
		return false, nil // stop here
	}

	// re-configure level
	if myCommonFlagValues.DebugMode {
		log.SetLevel(log.DebugLevel)
	} else {
		if myCommonFlagValues.LogLevelUpdated {
			log.SetLevel(myCommonFlagValues.LogLevel)
		}
	}

	if retryFlagValues.RetryChild {
		// read from stdin
		err := commons.InputMissingFieldsFromStdin()
		if err != nil {
			return false, xerrors.Errorf("failed to load config from stdin: %w", err) // stop here
		}
	}

	return true, nil // contiue
}

func printVersion() error {
	info, err := commons.GetVersionJSON()
	if err != nil {
		return xerrors.Errorf("failed to get version json: %w", err)
	}

	fmt.Println(info)
	return nil
}
