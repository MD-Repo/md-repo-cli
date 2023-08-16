package subcmd

import (
	"fmt"
	"strings"

	"github.com/MD-Repo/md-repo-cli/cmd/flag"
	"github.com/MD-Repo/md-repo-cli/commons"
	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

var describeTicketCmd = &cobra.Command{
	Use:     "describe_ticket [mdrepo_ticket]",
	Short:   "Describe MD-Repo ticket",
	Aliases: []string{"show_ticket", "desc_ticket"},
	RunE:    processDescribeTicketCommand,
	Args:    cobra.ExactArgs(1),
}

func AddDescribeTicketCommand(rootCmd *cobra.Command) {
	// attach common flags
	flag.SetCommonFlags(describeTicketCmd)

	rootCmd.AddCommand(describeTicketCmd)
}

func processDescribeTicketCommand(command *cobra.Command, args []string) error {
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

	ticketString := strings.TrimSpace(args[0])

	mdRepoTickets, err := commons.GetConfig().GetMDRepoTickets(ticketString)
	if err != nil {
		return xerrors.Errorf("failed to parse MD-Repo Ticket: %w", err)
	}

	if len(mdRepoTickets) == 0 {
		return xerrors.Errorf("failed to parse MD-Repo Ticket. No ticket is provided")
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
