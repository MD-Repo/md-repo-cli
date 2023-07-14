package commons

import (
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"

	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	"golang.org/x/term"
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

	if !appConfig.NoPassword {
		password := appConfig.Password
		for len(password) == 0 {
			fmt.Print("Password: ")
			bytePassword, err := term.ReadPassword(int(syscall.Stdin))
			if err != nil {
				return false, xerrors.Errorf("failed to read password: %w", err)
			}

			fmt.Print("\n")
			password = string(bytePassword)

			if len(password) == 0 {
				fmt.Println("Please provide password")
				fmt.Println("")
			} else {
				updated = true
			}
		}
		appConfig.Password = password
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

	appConfig.NoPassword = configTypeIn.NoPassword
	appConfig.Password = configTypeIn.Password

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
