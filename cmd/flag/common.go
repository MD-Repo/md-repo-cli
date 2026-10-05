package flag

import (
	"io"

	"github.com/MD-Repo/md-repo-cli/commons"
	"github.com/MD-Repo/md-repo-cli/commons/config"
	"github.com/MD-Repo/md-repo-cli/commons/terminal"
	"github.com/cockroachdb/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gopkg.in/natefinch/lumberjack.v2"
)

type CommonFlagValues struct {
	ShowVersion     bool
	ShowHelp        bool
	DebugMode       bool
	Quiet           bool
	logLevelInput   string
	LogLevel        log.Level
	LogLevelUpdated bool
	LogFile         string
	LogTerminal     bool
	Timeout         int
	TimeoutUpdated  bool
}

var (
	commonFlagValues CommonFlagValues
)

func SetCommonFlags(command *cobra.Command) {
	command.Flags().BoolVarP(&commonFlagValues.ShowVersion, "version", "v", false, "Display version information")
	command.Flags().BoolVarP(&commonFlagValues.ShowHelp, "help", "h", false, "Display help information about available commands and options")
	command.Flags().BoolVarP(&commonFlagValues.DebugMode, "debug", "d", false, "Enable verbose debug output for troubleshooting")
	command.Flags().BoolVarP(&commonFlagValues.Quiet, "quiet", "q", false, "Suppress all non-error output messages")
	command.Flags().StringVar(&commonFlagValues.logLevelInput, "log_level", "", "Set logging verbosity level (e.g., INFO, WARN, ERROR, DEBUG)")
	command.Flags().StringVar(&commonFlagValues.LogFile, "log_file", "", "Specify file path for logging output")
	command.Flags().BoolVarP(&commonFlagValues.LogTerminal, "log_terminal", "", false, "Enable logging to terminal")
	command.Flags().IntVarP(&commonFlagValues.Timeout, "timeout", "", config.GetDefaultFilesystemTimeoutInSeconds(), "Specify timeout duration in seconds")

	command.MarkFlagsMutuallyExclusive("quiet", "version")
	command.MarkFlagsMutuallyExclusive("log_level", "version")
	command.MarkFlagsMutuallyExclusive("debug", "quiet", "log_level")
}

func GetCommonFlagValues(command *cobra.Command) *CommonFlagValues {
	if command.Flags().Changed("timeout") {
		commonFlagValues.TimeoutUpdated = true
	}

	return &commonFlagValues
}

func getLogrusLogLevel(irodsLogLevel int) log.Level {
	switch irodsLogLevel {
	case 0:
		return log.PanicLevel
	case 1:
		return log.FatalLevel
	case 2, 3:
		return log.ErrorLevel
	case 4, 5, 6:
		return log.WarnLevel
	case 7:
		return log.InfoLevel
	case 8:
		return log.DebugLevel
	case 9, 10:
		return log.TraceLevel
	}

	if irodsLogLevel < 0 {
		return log.PanicLevel
	}

	return log.TraceLevel
}

func setLogLevel(command *cobra.Command) error {
	myCommonFlagValues := GetCommonFlagValues(command)
	if len(myCommonFlagValues.logLevelInput) > 0 {
		level, err := log.ParseLevel(myCommonFlagValues.logLevelInput)
		if err != nil {
			return errors.Wrapf(err, "invalid log level %q", myCommonFlagValues.logLevelInput)
		}
		myCommonFlagValues.LogLevel = level
		myCommonFlagValues.LogLevelUpdated = true
	}

	if myCommonFlagValues.Quiet {
		log.SetLevel(log.FatalLevel)
	} else if myCommonFlagValues.DebugMode {
		log.SetLevel(log.DebugLevel)
	} else {
		if myCommonFlagValues.LogLevelUpdated {
			log.SetLevel(myCommonFlagValues.LogLevel)
		}
	}
	return nil
}

func getLogWriter(logFile string) io.WriteCloser {
	if len(logFile) > 0 {
		return &lumberjack.Logger{
			Filename:   logFile,
			MaxSize:    50, // 50MB
			MaxBackups: 5,
			MaxAge:     30, // 30 days
			Compress:   false,
		}
	}

	return nil
}

func ProcessCommonFlags(command *cobra.Command) (bool, error) {
	myCommonFlagValues := GetCommonFlagValues(command)

	if err := setLogLevel(command); err != nil {
		return false, err
	}

	if myCommonFlagValues.ShowHelp {
		command.Usage()
		return false, nil // stop here
	}

	if myCommonFlagValues.ShowVersion {
		err := printVersion()
		return false, err
	}

	if len(myCommonFlagValues.LogFile) > 0 {
		fileLogWriter := getLogWriter(myCommonFlagValues.LogFile)

		if myCommonFlagValues.LogTerminal {
			// use multi output - to output to file and stdout
			mw := io.MultiWriter(terminal.GetTerminalWriter(), fileLogWriter)
			log.SetOutput(mw)
		} else {
			// use file log writer
			log.SetOutput(fileLogWriter)
		}
	}

	// prioritize log level user set via command-line argument
	if err := setLogLevel(command); err != nil {
		return false, err
	}

	return true, nil // continue
}

func printVersion() error {
	info, err := commons.GetVersionJSON()
	if err != nil {
		return errors.Wrapf(err, "failed to get version json")
	}

	terminal.Println(info)
	return nil
}
