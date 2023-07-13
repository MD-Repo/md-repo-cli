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
	Password        string
	PlainTextTicket bool
}

const (
	IRODSEnvironmentFileEnvKey string = "IRODS_ENVIRONMENT_FILE"
)

var (
	commonFlagValues CommonFlagValues
)

func SetCommonFlags(command *cobra.Command) {
	command.Flags().BoolVarP(&commonFlagValues.ShowVersion, "version", "v", false, "Print version")
	command.Flags().BoolVarP(&commonFlagValues.ShowHelp, "help", "h", false, "Print help")
	command.Flags().BoolVarP(&commonFlagValues.DebugMode, "debug", "d", false, "Enable debug mode")
	command.Flags().StringVar(&commonFlagValues.logLevelInput, "log_level", "", "Set log level")
	command.Flags().StringVar(&commonFlagValues.Password, "password", "", "Set password (not secure)")
	command.Flags().BoolVar(&commonFlagValues.PlainTextTicket, "plaintext_ticket", false, "Use ticket in plaintext")

	command.Flags().MarkHidden("plaintext_ticket")

	command.MarkFlagsMutuallyExclusive("debug", "version")
	command.MarkFlagsMutuallyExclusive("log_level", "version")
	command.MarkFlagsMutuallyExclusive("resource", "version")
	command.MarkFlagsMutuallyExclusive("ticket", "version")
	command.MarkFlagsMutuallyExclusive("session", "version")
}

func GetCommonFlagValues(command *cobra.Command) *CommonFlagValues {
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
	myCommonFlagValues := GetCommonFlagValues(command)
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

	commons.SetDefaultConfigIfEmpty()

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

	appConfig := commons.GetConfig()
	if myCommonFlagValues.PlainTextTicket {
		appConfig.NoPassword = true
	}

	if !myCommonFlagValues.PlainTextTicket {
		appConfig.Password = myCommonFlagValues.Password
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
