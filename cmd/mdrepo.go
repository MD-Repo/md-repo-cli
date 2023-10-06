package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/MD-Repo/md-repo-cli/cmd/flag"
	"github.com/MD-Repo/md-repo-cli/cmd/subcmd"
	"github.com/MD-Repo/md-repo-cli/commons"
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

		if irodsclient_types.IsConnectionConfigError(err) || irodsclient_types.IsConnectionError(err) {
			fmt.Fprintf(os.Stderr, "Failed to establish a connection to MD-Repo data server!\n")
		} else if irodsclient_types.IsAuthError(err) {
			fmt.Fprintf(os.Stderr, "Authentication failed!\n")
		} else if errors.Is(err, commons.InvalidTicketError) {
			fmt.Fprintf(os.Stderr, "Invalid ticket string!\n")
		} else if errors.Is(err, commons.InvalidTokenError) {
			fmt.Fprintf(os.Stderr, "Invalid token!\n")
		} else if errors.Is(err, commons.TokenNotProvidedError) {
			fmt.Fprintf(os.Stderr, "Token not provided!\n")
		} else if errors.Is(err, commons.SimulationNoNotMatchingError) {
			fmt.Fprintf(os.Stderr, "Simulation number not match!\n")
			fmt.Fprintf(os.Stderr, "> %s\n", err.Error())
		} else if errors.Is(err, commons.InvalidOrcIDError) {
			fmt.Fprintf(os.Stderr, "Invalid ORC-ID!\n")
		} else {
			fmt.Fprintf(os.Stderr, "Error: %s\nError Trace:\n  - %+v\n", err.Error(), err)
		}

		os.Exit(1)
	}
}
