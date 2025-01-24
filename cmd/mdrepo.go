package main

import (
	"errors"
	"net/http"
	"os"

	"github.com/MD-Repo/md-repo-cli/cmd/flag"
	"github.com/MD-Repo/md-repo-cli/cmd/subcmd"
	"github.com/MD-Repo/md-repo-cli/commons"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	"github.com/hashicorp/go-multierror"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:           "mdrepo [subcommand]",
	Short:         "MD-Repo command-line tool",
	RunE:          processCommand,
	SilenceUsage:  true,
	SilenceErrors: true,
	CompletionOptions: cobra.CompletionOptions{
		DisableDefaultCmd:   true,
		DisableNoDescFlag:   true,
		DisableDescriptions: true,
		HiddenDefaultCmd:    true,
	},
}

func Execute() error {
	return rootCmd.Execute()
}

func processCommand(command *cobra.Command, args []string) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "processCommand",
	})

	cont, err := flag.ProcessCommonFlags(command)
	if err != nil {
		logger.Errorf("%+v", err)
	}

	if !cont {
		return err
	}

	// if nothing is given
	command.Usage()

	return nil
}

func main() {
	commons.InitTerminalOutput()

	log.SetFormatter(&log.TextFormatter{
		TimestampFormat: "2006-01-02 15:04:05.000",
		FullTimestamp:   true,
	})

	log.SetLevel(log.FatalLevel)
	log.SetOutput(commons.GetTerminalWriter())

	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "main",
	})

	// attach common flags
	flag.SetCommonFlags(rootCmd)

	// add sub commands
	subcmd.AddGetCommand(rootCmd)
	subcmd.AddSubmitCommand(rootCmd)
	subcmd.AddSubmitListCommand(rootCmd)
	subcmd.AddUpgradeCommand(rootCmd)

	err := Execute()
	if err != nil {
		logger.Errorf("%+v", err)

		if flag.GetCommonFlagValues(rootCmd).DebugMode {
			commons.PrintErrorf("%+v\n", err)
		}

		if os.IsNotExist(err) {
			commons.PrintErrorf("File or directory not found!\n")
		} else if irodsclient_types.IsConnectionConfigError(err) {
			var connectionConfigError *irodsclient_types.ConnectionConfigError
			if errors.As(err, &connectionConfigError) {
				commons.PrintErrorf("Failed to establish a connection to MD-Repo data server (host: %q, port: %d)!\nWrong MD-Repo data server configuration.\n", connectionConfigError.Config.Host, connectionConfigError.Config.Port)
			} else {
				commons.PrintErrorf("Failed to establish a connection to MD-Repo data server!\nWrong MD-Repo data server configuration.\n")
			}
		} else if irodsclient_types.IsConnectionError(err) {
			printNetworkError()
		} else if irodsclient_types.IsConnectionPoolFullError(err) {
			var connectionPoolFullError *irodsclient_types.ConnectionPoolFullError
			if errors.As(err, &connectionPoolFullError) {
				commons.PrintErrorf("Failed to establish a new connection to MD-Repo data server as connection pool is full (occupied: %d, max: %d)!\n", connectionPoolFullError.Occupied, connectionPoolFullError.Max)
			} else {
				commons.PrintErrorf("Failed to establish a new connection to MD-Repo data server as connection pool is full!\n")
			}
		} else if irodsclient_types.IsAuthError(err) {
			var authError *irodsclient_types.AuthError
			if errors.As(err, &authError) {
				commons.PrintErrorf("Authentication failed (auth scheme: %q, username: %q, zone: %q)!\n", authError.Config.AuthenticationScheme, authError.Config.ClientUser, authError.Config.ClientZone)
			} else {
				commons.PrintErrorf("Authentication failed!\n")
			}
		} else if irodsclient_types.IsFileNotFoundError(err) {
			var fileNotFoundError *irodsclient_types.FileNotFoundError
			if errors.As(err, &fileNotFoundError) {
				commons.PrintErrorf("File or directory %q is not found!\n", fileNotFoundError.Path)
			} else {
				commons.PrintErrorf("File or directory is not found!\n")
			}
		} else if irodsclient_types.IsCollectionNotEmptyError(err) {
			var collectionNotEmptyError *irodsclient_types.CollectionNotEmptyError
			if errors.As(err, &collectionNotEmptyError) {
				commons.PrintErrorf("Directory %q is not empty!\n", collectionNotEmptyError.Path)
			} else {
				commons.PrintErrorf("Directory is not empty!\n")
			}
		} else if irodsclient_types.IsFileAlreadyExistError(err) {
			var fileAlreadyExistError *irodsclient_types.FileAlreadyExistError
			if errors.As(err, &fileAlreadyExistError) {
				commons.PrintErrorf("File or directory %q already exists!\n", fileAlreadyExistError.Path)
			} else {
				commons.PrintErrorf("File or directory already exists!\n")
			}
		} else if irodsclient_types.IsTicketNotFoundError(err) {
			var ticketNotFoundError *irodsclient_types.TicketNotFoundError
			if errors.As(err, &ticketNotFoundError) {
				commons.PrintErrorf("Ticket %q is not found!\n", ticketNotFoundError.Ticket)
			} else {
				commons.PrintErrorf("Ticket is not found!\n")
			}
		} else if irodsclient_types.IsUserNotFoundError(err) {
			var userNotFoundError *irodsclient_types.UserNotFoundError
			if errors.As(err, &userNotFoundError) {
				commons.PrintErrorf("User %q is not found!\n", userNotFoundError.Name)
			} else {
				commons.PrintErrorf("User is not found!\n")
			}
		} else if irodsclient_types.IsIRODSError(err) {
			var irodsError *irodsclient_types.IRODSError
			if errors.As(err, &irodsError) {
				commons.PrintErrorf("MD-Repo data server error (code: '%d', message: %q)\n", irodsError.Code, irodsError.Error())
			} else {
				commons.PrintErrorf("MD-Repo data server error!\n")
			}
		} else if commons.IsDialHTTPError(err) {
			printNetworkError()
		} else if commons.IsInvalidTicketError(err) {
			var invalidTicketError *commons.InvalidTicketError
			if errors.As(err, &invalidTicketError) {
				commons.PrintErrorf("MD-Repo ticket %q is invalid!\n", invalidTicketError.Ticket)
			} else {
				commons.PrintErrorf("MD-Repo ticket is invalid!\n")
			}
		} else if errors.Is(err, commons.TokenNotProvidedError) {
			commons.PrintErrorf("MD-Repo token is not provided!\n")
		} else if commons.IsMDRepoServiceError(err) {
			var serviceError *commons.MDRepoServiceError
			if errors.As(err, &serviceError) {
				commons.PrintErrorf("%s\n", serviceError.Message)
			} else {
				commons.PrintErrorf("MD-Repo service error!\nMD-Repo server might be temporarily unavailable.\nPlease try again in a few minutes.\n")
			}
		} else if merr, ok := err.(*multierror.Error); ok {
			for _, merrElem := range merr.Errors {
				commons.PrintErrorf("%s\n", merrElem.Error())
			}
		} else if commons.IsSimulationNoNotMatchingError(err) {
			var matchingError *commons.SimulationNoNotMatchingError
			if errors.As(err, &matchingError) {
				commons.PrintErrorf("MD-Repo simulation number not matching error!\n")
				commons.PrintErrorf("%s\n", matchingError.Error())

				if len(matchingError.ValidSimulationPaths) > 0 {
					commons.PrintErrorf("the simulations found:\n")
					for sourceIdx, sourcePath := range matchingError.ValidSimulationPaths {
						commons.PrintErrorf("[%d] %s\n", sourceIdx+1, sourcePath)
					}
				}

				if len(matchingError.InvalidSimulationPaths) > 0 {
					commons.PrintErrorf("the directories ignored due to lack of metadata file:\n")
					for sourceIdx, sourcePath := range matchingError.InvalidSimulationPaths {
						commons.PrintErrorf("[%d] %s\n", sourceIdx+1, sourcePath)
					}
				}

			} else {
				commons.PrintErrorf("MD-Repo simulation number not matching error!\n")
			}
		} else if commons.IsNotDirError(err) {
			var notDirError *commons.NotDirError
			if errors.As(err, &notDirError) {
				commons.PrintErrorf("Destination %q is not a directory!\n", notDirError.Path)
			} else {
				commons.PrintErrorf("Destination is not a directory!\n")
			}
		} else if commons.IsNotFileError(err) {
			var notFileError *commons.NotFileError
			if errors.As(err, &notFileError) {
				commons.PrintErrorf("Destination %q is not a file!\n", notFileError.Path)
			} else {
				commons.PrintErrorf("Destination is not a file!\n")
			}
		} else {
			commons.PrintErrorf("Unexpected error!\nError Trace:\n  - %+v\n", err)
		}

		os.Exit(1)
	}
}

func printNetworkError() {
	commons.PrintErrorf("Failed to establish a connection to MD-Repo data server!\n")

	// check if internet works
	_, err := http.Get("https://www.google.com")
	if err != nil {
		commons.PrintErrorf("No Internet access.\nCheck internet connectivity.\n")
		return
	}
	commons.Printf("Tested Internet access via www.google.com - OK.\n")

	// check if datastore is under maintenance
	_, err = http.Get("https://data.cyverse.org/dav/iplant/commons/community_released")
	if err != nil {
		commons.PrintErrorf("MD-Repo data server might be temporarily unavailable.\nCheck if CyVerse Data Store is under maintenance.\nhttps://cyverse.org/maintenance\n")
		return
	}
	commons.Printf("Tested MD-Repo data server access via https://data.cyverse.org - OK.\n")

	commons.PrintErrorf("Verify that your firewall allows access on port 1247 (perhaps request this from network admin)\n")
}
