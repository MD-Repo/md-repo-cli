package commons

import (
	"fmt"
	"io"
	"os"
	"strings"

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
		CSNegotiationPolicy:     irodsclient_types.CSNegotiationRequireTCP,
		Host:                    mdRepoHost,
		Port:                    mdRepoPort,
		ClientUser:              mdRepoUser,
		ClientZone:              mdRepoZone,
		ProxyUser:               mdRepoUser,
		ProxyZone:               mdRepoZone,
		Password:                mdRepoUserPassword,
		Ticket:                  ticketString,
		DefaultResource:         "",
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
			fmt.Print("Input token: ")
			fmt.Scanln(&token)
			fmt.Print("\n")

			if len(token) == 0 {
				fmt.Println("Error! Please type token.")
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

// InputOrcID inputs ORCID
func InputOrcID() string {
	var orcID string
	fmt.Print("Input ORC-ID: ")
	fmt.Scanln(&orcID)

	return orcID
}

// InputSimulationNo inputs simulation no
func InputSimulationNo() int {
	var simulationNo int
	fmt.Print("Number of simulations expected: ")
	fmt.Scanln(&simulationNo)

	return simulationNo
}
