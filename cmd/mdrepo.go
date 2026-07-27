package main

import (
	"fmt"
	"net/http"
	"os"
	"sync"

	"github.com/MD-Repo/md-repo-cli/cmd/flag"
	"github.com/MD-Repo/md-repo-cli/cmd/subcmd"
	"github.com/MD-Repo/md-repo-cli/commons"
	"github.com/MD-Repo/md-repo-cli/commons/terminal"
	"github.com/MD-Repo/md-repo-cli/commons/types"
	"github.com/MD-Repo/md-repo-cli/commons/upgrade"
	"github.com/cockroachdb/errors"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:           "mdrepo <subcommand> [flags]",
	Short:         fmt.Sprintf("MD-Repo command-line tool (%s)", commons.GetClientVersion()),
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
		"command": command.Name(),
		"args":    args,
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
	terminal.InitTerminalOutput()

	log.SetFormatter(&log.TextFormatter{
		TimestampFormat: "2006-01-02 15:04:05.000",
		FullTimestamp:   true,
	})

	log.SetLevel(log.FatalLevel)
	log.SetReportCaller(true)
	log.SetOutput(terminal.GetTerminalWriter())

	logger := log.WithFields(log.Fields{})

	// attach common flags
	flag.SetCommonFlags(rootCmd)

	// add sub commands
	subcmd.AddGetCommand(rootCmd)
	subcmd.AddSubmitCommand(rootCmd)
	subcmd.AddSubmitListCommand(rootCmd)
	subcmd.AddUpgradeCommand(rootCmd)

	// check for a new release in the background while the command runs
	notifyIfNewRelease := func() {} // do nothing
	wg := sync.WaitGroup{}
	wg.Add(1)

	go func() {
		defer wg.Done()
		release, err := upgrade.CheckNewRelease()
		if err == nil {
			if commons.HasNewRelease(commons.GetClientVersion(), release.Version()) {
				// found a new release
				notifyIfNewRelease = func() {
					terminal.Printf("A newer version v%s is available. Run 'mdrepo upgrade' to update.\n", release.Version())
				}
			}
			return
		}
	}()

	err := Execute()
	if err != nil {
		logger.Errorf("%+v", err)

		if flag.GetCommonFlagValues(rootCmd).DebugMode {
			terminal.PrintErrorf("%+v\n", err)
		}

		if os.IsNotExist(err) {
			terminal.PrintErrorf("File or directory not found!\n")
		} else if irodsclient_types.IsConnectionConfigError(err) {
			var connectionConfigError *irodsclient_types.ConnectionConfigError
			if errors.As(err, &connectionConfigError) {
				terminal.PrintErrorf("Failed to establish a connection to MD-Repo data server (host: %q, port: %d)!\nWrong MD-Repo data server configuration.\n", connectionConfigError.Account.Host, connectionConfigError.Account.Port)
			} else {
				terminal.PrintErrorf("Failed to establish a connection to MD-Repo data server!\nWrong MD-Repo data server configuration.\n")
			}
		} else if irodsclient_types.IsConnectionError(err) {
			printIRODSNetworkError()
		} else if irodsclient_types.IsConnectionPoolFullError(err) {
			var connectionPoolFullError *irodsclient_types.ConnectionPoolFullError
			if errors.As(err, &connectionPoolFullError) {
				terminal.PrintErrorf("Failed to establish a new connection to MD-Repo data server as connection pool is full (occupied: %d, max: %d)!\n", connectionPoolFullError.Occupied, connectionPoolFullError.Max)
			} else {
				terminal.PrintErrorf("Failed to establish a new connection to MD-Repo data server as connection pool is full!\n")
			}
		} else if irodsclient_types.IsAuthError(err) {
			var authError *irodsclient_types.AuthError
			if errors.As(err, &authError) {
				terminal.PrintErrorf("Authentication failed (auth scheme: %q, username: %q, zone: %q)!\n", authError.Config.AuthenticationScheme, authError.Config.ClientUser, authError.Config.ClientZone)
			} else {
				terminal.PrintErrorf("Authentication failed!\n")
			}
		} else if irodsclient_types.IsFileNotFoundError(err) {
			var fileNotFoundError *irodsclient_types.FileNotFoundError
			if errors.As(err, &fileNotFoundError) {
				terminal.PrintErrorf("File or directory %q is not found!\n", fileNotFoundError.Path)
			} else {
				terminal.PrintErrorf("File or directory is not found!\n")
			}
		} else if irodsclient_types.IsCollectionNotEmptyError(err) {
			var collectionNotEmptyError *irodsclient_types.CollectionNotEmptyError
			if errors.As(err, &collectionNotEmptyError) {
				terminal.PrintErrorf("Directory %q is not empty!\n", collectionNotEmptyError.Path)
			} else {
				terminal.PrintErrorf("Directory is not empty!\n")
			}
		} else if irodsclient_types.IsFileAlreadyExistError(err) {
			var fileAlreadyExistError *irodsclient_types.FileAlreadyExistError
			if errors.As(err, &fileAlreadyExistError) {
				terminal.PrintErrorf("File or directory %q already exists!\n", fileAlreadyExistError.Path)
			} else {
				terminal.PrintErrorf("File or directory already exists!\n")
			}
		} else if irodsclient_types.IsTicketNotFoundError(err) {
			var ticketNotFoundError *irodsclient_types.TicketNotFoundError
			if errors.As(err, &ticketNotFoundError) {
				terminal.PrintErrorf("Ticket %q is not found!\n", ticketNotFoundError.Ticket)
			} else {
				terminal.PrintErrorf("Ticket is not found!\n")
			}
		} else if irodsclient_types.IsUserNotFoundError(err) {
			var userNotFoundError *irodsclient_types.UserNotFoundError
			if errors.As(err, &userNotFoundError) {
				terminal.PrintErrorf("User %q is not found!\n", userNotFoundError.Name)
			} else {
				terminal.PrintErrorf("User is not found!\n")
			}
		} else if irodsclient_types.IsIRODSError(err) {
			var irodsError *irodsclient_types.IRODSError
			if errors.As(err, &irodsError) {
				terminal.PrintErrorf("MD-Repo data server error (code: '%d', message: %q)\n", irodsError.Code, irodsError.Error())
			} else {
				terminal.PrintErrorf("MD-Repo data server error!\n")
			}
		} else if types.IsWebDAVError(err) {
			var webDAVError *types.WebDAVError
			if errors.As(err, &webDAVError) {
				printWebdavError(webDAVError.URL, webDAVError.ErrorCode)
			} else {
				printWebdavError("", 0)
			}
		} else if types.IsInvalidTicketError(err) {
			var invalidTicketError *types.InvalidTicketError
			if errors.As(err, &invalidTicketError) {
				terminal.PrintErrorf("MD-Repo ticket %q is invalid!\n", invalidTicketError.Ticket)
			} else {
				terminal.PrintErrorf("MD-Repo ticket is invalid!\n")
			}
		} else if types.IsTokenNotProvidedError(err) {
			var tokenNotProvidedError *types.TokenNotProvidedError
			if errors.As(err, &tokenNotProvidedError) {
				terminal.PrintErrorf("MD-Repo token is not provided!\n")
			} else {
				terminal.PrintErrorf("MD-Repo token is not provided!\n")
			}
		} else if types.IsMDRepoServiceError(err) {
			var serviceError *types.MDRepoServiceError
			if errors.As(err, &serviceError) {
				terminal.PrintErrorf("%s\n", serviceError.Message)
			} else {
				terminal.PrintErrorf("MD-Repo service error!\nMD-Repo server might be temporarily unavailable.\nPlease try again in a few minutes.\n")
			}
		} else if types.IsInvalidOrcIDError(err) {
			var invalidOrcIDError *types.InvalidOrcIDError
			if errors.As(err, &invalidOrcIDError) {
				terminal.PrintErrorf("Invalid ORC-ID: %q (expected %q)\n", invalidOrcIDError.FoundOrcID, invalidOrcIDError.RequestedOrcID)
			} else {
				terminal.PrintErrorf("Invalid ORC-ID\n")
			}
		} else if types.IsInvalidSubmitMetadataError(err) {
			var submitMetadataError *types.InvalidSubmitMetadataError
			if errors.As(err, &submitMetadataError) {
				terminal.PrintErrorf("%s", submitMetadataError.Error())
			} else {
				terminal.PrintErrorf("submit metadata error!\n")
			}
		} else if types.IsSimulationNoNotMatchingError(err) {
			var matchingError *types.SimulationNoNotMatchingError
			if errors.As(err, &matchingError) {
				terminal.PrintErrorf("MD-Repo simulation number not matching error!\n")
				terminal.PrintErrorf("%s\n", matchingError.Error())

				if len(matchingError.ValidSimulationPaths) > 0 {
					terminal.PrintErrorf("the simulations found:\n")
					for sourceIdx, sourcePath := range matchingError.ValidSimulationPaths {
						terminal.PrintErrorf("[%d] %s\n", sourceIdx+1, sourcePath)
					}
				}

				if len(matchingError.InvalidSimulationPaths) > 0 {
					terminal.PrintErrorf("the directories ignored due to lack of metadata file:\n")
					for sourceIdx, sourcePath := range matchingError.InvalidSimulationPaths {
						if len(matchingError.InvalidSimulationPathsErrors) > sourceIdx {
							terminal.PrintErrorf("[%d] %s: %s\n", sourceIdx+1, sourcePath, matchingError.InvalidSimulationPathsErrors[sourceIdx])
						} else {
							terminal.PrintErrorf("[%d] %s\n", sourceIdx+1, sourcePath)
						}
					}
				}
			} else {
				terminal.PrintErrorf("MD-Repo simulation number not matching error!\n")
			}
		} else if types.IsNotDirError(err) {
			var notDirError *types.NotDirError
			if errors.As(err, &notDirError) {
				terminal.PrintErrorf("Destination %q is not a directory!\n", notDirError.Path)
			} else {
				terminal.PrintErrorf("Destination is not a directory!\n")
			}
		} else if types.IsNotFileError(err) {
			var notFileError *types.NotFileError
			if errors.As(err, &notFileError) {
				terminal.PrintErrorf("Destination %q is not a file!\n", notFileError.Path)
			} else {
				terminal.PrintErrorf("Destination is not a file!\n")
			}
		} else {
			terminal.PrintErrorf("Unexpected error!\nError Trace:\n  - %+v\n", err)
		}
	}

	wg.Wait()
	notifyIfNewRelease()

	if err != nil {
		os.Exit(1)
	}
}

func printIRODSNetworkError() {
	terminal.PrintErrorf("Failed to establish a connection to MD-Repo data server!\n")

	// check if internet works
	_, err := http.Get("https://www.google.com")
	if err != nil {
		terminal.PrintErrorf("No Internet access.\nCheck internet connectivity.\n")
		return
	}
	terminal.Printf("Tested Internet access via www.google.com - OK.\n")

	// check if datastore is under maintenance
	_, err = http.Get("https://data.cyverse.org/dav/iplant/commons/community_released")
	if err != nil {
		terminal.PrintErrorf("MD-Repo data server might be temporarily unavailable.\nCheck if CyVerse Data Store is under maintenance.\nhttps://cyverse.org/maintenance\n")
		return
	}
	terminal.Printf("Tested MD-Repo data server access via https://data.cyverse.org - OK.\n")

	terminal.PrintErrorf("Verify that your firewall allows access on port 1247 (perhaps request this from network admin)\n")
}

func printMDRepoServerNetworkError(url string) {
	terminal.PrintErrorf("Failed to establish a HTTP connection to MD-Repo server %s!\n", url)

	// check if internet works
	_, err := http.Get("https://www.google.com")
	if err != nil {
		terminal.PrintErrorf("No Internet access.\nCheck internet connectivity.\n")
		return
	}
	terminal.Printf("Tested Internet access via www.google.com - OK.\n")
}

func printWebdavError(url string, errorCode int) {
	if url == "" || errorCode == 0 {
		terminal.PrintErrorf("Failed to access MD-Repo server via WebDAV protocol.\n")
		return
	}

	terminal.PrintErrorf("Failed to access MD-Repo server %q via WebDAV protocol (error code: %d).\n", url, errorCode)

	if errorCode == http.StatusForbidden {
		terminal.PrintErrorf("Your MD-Repo server access might be blocked.\n")

		terminal.PrintErrorf("Your IP address might be blocked.\nVisit 'https://unblockme.cyverse.org' to unblock your IP address.\nPlease contact us at 'help@mdrepo.org' if it did not help.\n")
		return
	}
}
