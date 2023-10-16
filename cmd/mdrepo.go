package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/MD-Repo/md-repo-cli/cmd/flag"
	"github.com/MD-Repo/md-repo-cli/cmd/subcmd"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
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
			fmt.Fprintf(os.Stderr, "Failed to establish a connection to MD-Repo data server!\n")
		} else if irodsclient_types.IsConnectionError(err) {
			fmt.Fprintf(os.Stderr, "Failed to establish a connection to MD-Repo data server!\n")
		} else if irodsclient_types.IsAuthError(err) {
			fmt.Fprintf(os.Stderr, "Authentication failed!\n")
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
		} else if irodsclient_types.IsIRODSError(err) {
			var irodsError *irodsclient_types.IRODSError
			if errors.As(err, &irodsError) {
				fmt.Fprintf(os.Stderr, "MD-Repo data server error: %s\n", irodsError.Error())
			} else {
				fmt.Fprintf(os.Stderr, "MD-Repo data server error!\n")
			}
		}

		fmt.Fprintf(os.Stderr, "\nError Trace:\n  - %+v\n", err)
		os.Exit(1)
	}
}
