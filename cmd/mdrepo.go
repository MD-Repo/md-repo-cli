package main

import (
	"errors"
	"fmt"
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
	log.SetFormatter(&log.TextFormatter{
		TimestampFormat: "2006-01-02 15:04:05.000",
		FullTimestamp:   true,
	})

	log.SetLevel(log.FatalLevel)

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

		if os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "File or dir not found!\n")
		} else if irodsclient_types.IsConnectionConfigError(err) {
			var connectionConfigError *irodsclient_types.ConnectionConfigError
			if errors.As(err, &connectionConfigError) {
				fmt.Fprintf(os.Stderr, "Failed to establish a connection to MD-Repo data server (host: '%s', port: '%d')!\n", connectionConfigError.Config.Host, connectionConfigError.Config.Port)
			} else {
				fmt.Fprintf(os.Stderr, "Failed to establish a connection to MD-Repo data server!\n")
			}
		} else if irodsclient_types.IsConnectionError(err) {
			fmt.Fprintf(os.Stderr, "Failed to establish a connection to MD-Repo data server!\n")
		} else if irodsclient_types.IsConnectionPoolFullError(err) {
			var connectionPoolFullError *irodsclient_types.ConnectionPoolFullError
			if errors.As(err, &connectionPoolFullError) {
				fmt.Fprintf(os.Stderr, "Failed to establish a new connection to MD-Repo data server as connection pool is full (occupied: %d, max: %d)!\n", connectionPoolFullError.Occupied, connectionPoolFullError.Max)
			} else {
				fmt.Fprintf(os.Stderr, "Failed to establish a new connection to MD-Repo data server as connection pool is full!\n")
			}
		} else if irodsclient_types.IsAuthError(err) {
			var authError *irodsclient_types.AuthError
			if errors.As(err, &authError) {
				fmt.Fprintf(os.Stderr, "Authentication failed (auth scheme: '%s', username: '%s', zone: '%s')!\n", authError.Config.AuthenticationScheme, authError.Config.ClientUser, authError.Config.ClientZone)
			} else {
				fmt.Fprintf(os.Stderr, "Authentication failed!\n")
			}
		} else if irodsclient_types.IsFileNotFoundError(err) {
			var fileNotFoundError *irodsclient_types.FileNotFoundError
			if errors.As(err, &fileNotFoundError) {
				fmt.Fprintf(os.Stderr, "File or dir '%s' not found!\n", fileNotFoundError.Path)
			} else {
				fmt.Fprintf(os.Stderr, "File or dir not found!\n")
			}
		} else if irodsclient_types.IsCollectionNotEmptyError(err) {
			var collectionNotEmptyError *irodsclient_types.CollectionNotEmptyError
			if errors.As(err, &collectionNotEmptyError) {
				fmt.Fprintf(os.Stderr, "Dir '%s' not empty!\n", collectionNotEmptyError.Path)
			} else {
				fmt.Fprintf(os.Stderr, "Dir not empty!\n")
			}
		} else if irodsclient_types.IsFileAlreadyExistError(err) {
			var fileAlreadyExistError *irodsclient_types.FileAlreadyExistError
			if errors.As(err, &fileAlreadyExistError) {
				fmt.Fprintf(os.Stderr, "File or dir '%s' already exist!\n", fileAlreadyExistError.Path)
			} else {
				fmt.Fprintf(os.Stderr, "File or dir already exist!\n")
			}
		} else if irodsclient_types.IsTicketNotFoundError(err) {
			var ticketNotFoundError *irodsclient_types.TicketNotFoundError
			if errors.As(err, &ticketNotFoundError) {
				fmt.Fprintf(os.Stderr, "Ticket '%s' not found!\n", ticketNotFoundError.Ticket)
			} else {
				fmt.Fprintf(os.Stderr, "Ticket not found!\n")
			}
		} else if irodsclient_types.IsUserNotFoundError(err) {
			var userNotFoundError *irodsclient_types.UserNotFoundError
			if errors.As(err, &userNotFoundError) {
				fmt.Fprintf(os.Stderr, "User '%s' not found!\n", userNotFoundError.Name)
			} else {
				fmt.Fprintf(os.Stderr, "User not found!\n")
			}
		} else if irodsclient_types.IsIRODSError(err) {
			var irodsError *irodsclient_types.IRODSError
			if errors.As(err, &irodsError) {
				fmt.Fprintf(os.Stderr, "MD-Repo data server error (code: '%d', message: '%s')\n", irodsError.Code, irodsError.Error())
			} else {
				fmt.Fprintf(os.Stderr, "MD-Repo data server error!\n")
			}
		} else if commons.IsInvalidTicketError(err) {
			var invalidTicketError *commons.InvalidTicketError
			if errors.As(err, &invalidTicketError) {
				fmt.Fprintf(os.Stderr, "MD-Repo ticket '%s' is invalid!\n", invalidTicketError.Ticket)
			} else {
				fmt.Fprintf(os.Stderr, "MD-Repo ticket is invalid!\n")
			}
		} else if errors.Is(err, commons.TokenNotProvidedError) {
			fmt.Fprintf(os.Stderr, "MD-Repo token is not provided!\n")
		} else if commons.IsMDRepoServiceError(err) {
			var serviceError *commons.MDRepoServiceError
			if errors.As(err, &serviceError) {
				fmt.Fprintf(os.Stderr, "%s\n", serviceError.Message)
			} else {
				fmt.Fprintf(os.Stderr, "MD-Repo service error!\n")
			}
		} else if merr, ok := err.(*multierror.Error); ok {
			for _, merrElem := range merr.Errors {
				fmt.Fprintf(os.Stderr, "%s\n", merrElem.Error())
			}
		} else if commons.IsSimulationNoNotMatchingError(err) {
			var matchingError *commons.SimulationNoNotMatchingError
			if errors.As(err, &matchingError) {
				fmt.Fprintf(os.Stderr, "MD-Repo simulation number not matching error!\n")
				fmt.Fprintf(os.Stderr, "%s\n", matchingError.Error())

				if len(matchingError.ValidSimulationPaths) > 0 {
					fmt.Fprintf(os.Stderr, "the simulations found:\n")
					for sourceIdx, sourcePath := range matchingError.ValidSimulationPaths {
						fmt.Fprintf(os.Stderr, "[%d] %s\n", sourceIdx+1, sourcePath)
					}
				}

				if len(matchingError.InvalidSimulationPaths) > 0 {
					fmt.Fprintf(os.Stderr, "the directories ignored due to lack of metadata file:\n")
					for sourceIdx, sourcePath := range matchingError.InvalidSimulationPaths {
						fmt.Fprintf(os.Stderr, "[%d] %s\n", sourceIdx+1, sourcePath)
					}
				}
			} else {
				fmt.Fprintf(os.Stderr, "MD-Repo simulation number not matching error!\n")
			}
		} else {
			fmt.Fprintf(os.Stderr, "Unexpected error!\nError Trace:\n  - %+v\n", err)
		}

		os.Exit(1)
	}
}
