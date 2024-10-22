package commons

import (
	"io"
	"os"

	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
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

func SetDefaultConfigIfEmpty() {
	if appConfig == nil {
		appConfig = GetDefaultConfig()
	}
}

func GetAccount(ticket *MDRepoTicket) (*irodsclient_types.IRODSAccount, error) {
	ticketString := ""
	if ticket != nil {
		ticketString = ticket.IRODSTicket
	}

	return &irodsclient_types.IRODSAccount{
		AuthenticationScheme:    irodsclient_types.AuthSchemeNative,
		ClientServerNegotiation: false,
		CSNegotiationPolicy:     irodsclient_types.CSNegotiationPolicyRequestTCP,
		Host:                    mdRepoHost,
		Port:                    mdRepoPort,
		ClientUser:              mdRepoUser,
		ClientZone:              mdRepoZone,
		ProxyUser:               mdRepoUser,
		ProxyZone:               mdRepoZone,
		Password:                mdRepoUserPassword,
		Ticket:                  ticketString,
		DefaultResource:         "",
		DefaultHashScheme:       irodsclient_types.HashSchemeDefault,
		PamTTL:                  1,
		SSLConfiguration:        nil,
	}, nil
}

// InputMissingFields inputs missing fields
func InputMissingFields() (bool, error) {
	updated := false

	if len(appConfig.TicketString) == 0 && len(appConfig.Token) == 0 {
		token := appConfig.Token
		for len(token) == 0 {
			token = Input("Input token")
			if len(token) == 0 {
				PrintErrorf("Error! Please type token.")
			} else {
				updated = true
				appConfig.Token = token
			}
		}
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

	appConfig.TicketString = configTypeIn.TicketString

	return nil
}

// InputOrcID inputs ORCID
func InputOrcID() string {
	return Input("Input ORC-ID")
}

// InputSimulationNo inputs simulation no
func InputSimulationNo() int {
	return InputInt("Number of simulations expected")
}
