package flag

import (
	"github.com/spf13/cobra"
)

type TokenFlagValues struct {
	Token        string
	TicketString string
	ServiceURL   string
}

var (
	tokenFlagValues TokenFlagValues
)

func SetTokenFlags(command *cobra.Command) {
	command.Flags().StringVarP(&tokenFlagValues.Token, "token", "t", "", "Specify token")
	command.Flags().StringVar(&tokenFlagValues.ServiceURL, "svc_url", "", "Specify service url (use default if not provided)")

	command.Flags().StringVar(&tokenFlagValues.TicketString, "ticket_string", "", "Specify ticket string")
	command.Flags().MarkHidden("ticket_string")

	command.MarkFlagsMutuallyExclusive("token", "ticket_string")
}

func GetTokenFlagValues() *TokenFlagValues {
	return &tokenFlagValues
}
