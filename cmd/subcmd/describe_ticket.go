package subcmd

import (
	"fmt"

	"github.com/MD-Repo/md-repo-cli/cmd/flag"
	"github.com/MD-Repo/md-repo-cli/commons"
	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

var describeGetTicketCmd = &cobra.Command{
	Use:     "desc_get_ticket [mdrepo_ticket]",
	Short:   "Describe MD-Repo get ticket",
	Aliases: []string{"show_get_ticket", "describe_get_ticket"},
	RunE:    processDescribeGetTicketCommand,
	Args:    cobra.ExactArgs(1),
}

func AddDescribeGetTicketCommand(rootCmd *cobra.Command) {
	// attach common flags
	flag.SetCommonFlags(describeGetTicketCmd)

	rootCmd.AddCommand(describeGetTicketCmd)
}

func processDescribeGetTicketCommand(command *cobra.Command, args []string) error {
	cont, err := flag.ProcessCommonFlags(command)
	if err != nil {
		return xerrors.Errorf("failed to process common flags: %w", err)
	}

	if !cont {
		return nil
	}

	// handle local flags
	_, err = commons.InputMissingFields()
	if err != nil {
		return xerrors.Errorf("failed to input missing fields: %w", err)
	}

	ticketString := args[0]

	mdRepoTickets, err := commons.ReadTicketsFromStringOrDownloadHash(commons.GetConfig(), ticketString)
	if err != nil {
		return xerrors.Errorf("failed to read ticket %s: %w", ticketString, err)
	}

	// display
	fmt.Printf("Ticket: %s\n", ticketString)

	printTicketTextGridHead()

	for _, mdRepoTicket := range mdRepoTickets {
		printTicketTextGridRow(mdRepoTicket.IRODSTicket, mdRepoTicket.IRODSDataPath)
	}

	return nil
}

var describeSubmitTicketCmd = &cobra.Command{
	Use:     "desc_submit_ticket [mdrepo_ticket]",
	Short:   "Describe MD-Repo submit ticket",
	Aliases: []string{"show_submit_ticket", "describe_submit_ticket"},
	RunE:    processDescribeSubmitTicketCommand,
	Args:    cobra.ExactArgs(1),
}

func AddDescribeSubmitTicketCommand(rootCmd *cobra.Command) {
	// attach common flags
	flag.SetCommonFlags(describeSubmitTicketCmd)

	rootCmd.AddCommand(describeSubmitTicketCmd)
}

func processDescribeSubmitTicketCommand(command *cobra.Command, args []string) error {
	cont, err := flag.ProcessCommonFlags(command)
	if err != nil {
		return xerrors.Errorf("failed to process common flags: %w", err)
	}

	if !cont {
		return nil
	}

	// handle local flags
	_, err = commons.InputMissingFields()
	if err != nil {
		return xerrors.Errorf("failed to input missing fields: %w", err)
	}

	ticketString := args[0]

	mdRepoTickets, err := commons.ReadTicketsFromString(commons.GetConfig(), ticketString)
	if err != nil {
		return xerrors.Errorf("failed to read ticket %s: %w", ticketString, err)
	}

	// display
	fmt.Printf("Ticket: %s\n", ticketString)

	printTicketTextGridHead()

	for _, mdRepoTicket := range mdRepoTickets {
		printTicketTextGridRow(mdRepoTicket.IRODSTicket, mdRepoTicket.IRODSDataPath)
	}

	return nil
}

func printTicketTextGridHead() {
	printTicketTextGridRow("IRODS_TICKET", "IRODS_PATH")
}

func printTicketTextGridRow(irodsTicket string, irodsPath string) {
	fmt.Printf("%-50s\t%s\n", irodsTicket, irodsPath)
}
