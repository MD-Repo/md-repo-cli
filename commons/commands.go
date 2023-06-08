package commons

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"syscall"

	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/term"
	"golang.org/x/xerrors"
)

var (
	appConfig *Config
)

func GetConfig() *Config {
	if appConfig == nil {
		appConfig = GetDefaultConfig()
	}

	return appConfig
}

func GetAccount(ticket *MDRepoTicket) (*irodsclient_types.IRODSAccount, error) {
	return &irodsclient_types.IRODSAccount{
		AuthenticationScheme:    irodsclient_types.AuthSchemeNative,
		ClientServerNegotiation: false,
		CSNegotiationPolicy:     irodsclient_types.CSNegotiationRequireTCP,
		Host:                    mdRepoHost,
		Port:                    mdRepoPort,
		ClientUser:              mdRepoUser,
		ClientZone:              mdRepoZone,
		ProxyUser:               mdRepoUser,
		ProxyZone:               mdRepoZone,
		Password:                mdRepoUserPassword,
		Ticket:                  ticket.IRODSTicket,
		DefaultResource:         "",
		PamTTL:                  1,
		SSLConfiguration:        nil,
	}, nil
}

func SetCommonFlags(command *cobra.Command) {
	command.Flags().BoolP("version", "v", false, "Print version")
	command.Flags().BoolP("help", "h", false, "Print help")
	command.Flags().BoolP("debug", "d", false, "Enable debug mode")
	command.Flags().String("password", "", "Set password")
	command.Flags().String("log_level", "", "Set log level")

	// this is hidden
	command.Flags().Bool("plaintext_ticket", false, "Use ticket in plaintext")
	command.Flags().MarkHidden("plaintext_ticket")
	command.Flags().Bool("retry_child", false, "Set this to retry child process")
	command.Flags().MarkHidden("retry_child")
}

func ProcessCommonFlags(command *cobra.Command) (bool, error) {
	logLevel := ""
	logLevelFlag := command.Flags().Lookup("log_level")
	if logLevelFlag != nil {
		logLevelStr := logLevelFlag.Value.String()
		if len(logLevelStr) > 0 {
			lvl, err := log.ParseLevel(logLevelStr)
			if err != nil {
				lvl = log.InfoLevel
			}

			log.SetLevel(lvl)
			logLevel = logLevelStr
		}
	}

	debug := false
	debugFlag := command.Flags().Lookup("debug")
	if debugFlag != nil {
		debugValue, err := strconv.ParseBool(debugFlag.Value.String())
		if err != nil {
			debugValue = false
		}

		if debugValue {
			log.SetLevel(log.DebugLevel)
		}

		debug = debugValue
	}

	helpFlag := command.Flags().Lookup("help")
	if helpFlag != nil {
		help, err := strconv.ParseBool(helpFlag.Value.String())
		if err != nil {
			help = false
		}

		if help {
			PrintHelp(command)
			return false, nil // stop here
		}
	}

	versionFlag := command.Flags().Lookup("version")
	if versionFlag != nil {
		version, err := strconv.ParseBool(versionFlag.Value.String())
		if err != nil {
			version = false
		}

		if version {
			printVersion()
			return false, nil // stop here
		}
	}

	if appConfig == nil {
		appConfig = GetDefaultConfig()
	}

	// re-configure level
	if len(logLevel) > 0 {
		lvl, err := log.ParseLevel(logLevel)
		if err != nil {
			lvl = log.InfoLevel
		}

		log.SetLevel(lvl)
	}
	if debug {
		log.SetLevel(log.DebugLevel)
	}

	retryChild := false
	retryChildFlag := command.Flags().Lookup("retry_child")
	if retryChildFlag != nil {
		retryChildValue, err := strconv.ParseBool(retryChildFlag.Value.String())
		if err != nil {
			retryChildValue = false
		}

		retryChild = retryChildValue
	}

	if retryChild {
		// read from stdin
		err := InputMissingFieldsFromStdin()
		if err != nil {
			return false, xerrors.Errorf("failed to load config from stdin: %w", err) // stop here
		}
	}

	plaintextTicketFlag := command.Flags().Lookup("plaintext_ticket")
	if plaintextTicketFlag != nil {
		plaintextTicketValue, err := strconv.ParseBool(plaintextTicketFlag.Value.String())
		if err != nil {
			appConfig.NoPassword = false
		} else {
			appConfig.NoPassword = plaintextTicketValue
		}
	}

	if !appConfig.NoPassword {
		passwordFlag := command.Flags().Lookup("password")
		if passwordFlag != nil {
			// load to global variable
			appConfig.Password = passwordFlag.Value.String()
		}
	}

	return true, nil // contiue
}

// InputMissingFields inputs missing fields
func InputMissingFields() (bool, error) {
	updated := false

	if !appConfig.NoPassword {
		password := appConfig.Password
		for len(password) == 0 {
			fmt.Print("MDRepo User Password: ")
			bytePassword, err := term.ReadPassword(int(syscall.Stdin))
			if err != nil {
				return false, xerrors.Errorf("failed to read password: %w", err)
			}

			fmt.Print("\n")
			password = string(bytePassword)

			if len(password) == 0 {
				fmt.Println("Please provide password")
				fmt.Println("")
			} else {
				updated = true
			}
		}
		appConfig.Password = password
	}

	return updated, nil
}

// InputMissingFieldsFromStdin inputs missing fields
func InputMissingFieldsFromStdin() error {
	// read from stdin
	stdinBytes, err := io.ReadAll(os.Stdin)
	if err != nil {
		return xerrors.Errorf("failed to read missing config values from stdin: %w", err)
	}

	configTypeIn, err := NewConfigTypeInFromYAML(stdinBytes)
	if err != nil {
		return xerrors.Errorf("failed to read missing config values: %w", err)
	}

	appConfig.NoPassword = configTypeIn.NoPassword
	appConfig.Password = configTypeIn.Password

	return nil
}

func printVersion() error {
	info, err := GetVersionJSON()
	if err != nil {
		return xerrors.Errorf("failed to get version json: %w", err)
	}

	fmt.Println(info)
	return nil
}

func PrintHelp(command *cobra.Command) error {
	return command.Usage()
}

// InputYN inputs Y or N
// true for Y, false for N
func InputYN(msg string) bool {
	userInput := ""

	for {
		fmt.Printf("%s [y/n]: ", msg)

		fmt.Scanln(&userInput)
		userInput = strings.ToLower(userInput)

		if userInput == "y" || userInput == "yes" {
			return true
		} else if userInput == "n" || userInput == "no" {
			return false
		}
	}
}
