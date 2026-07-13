package config

import (
	"github.com/MD-Repo/md-repo-cli/commons/terminal"
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

// InputMissingFields inputs missing fields
func InputMissingFields() (bool, error) {
	updated := false

	if len(appConfig.TicketString) == 0 && len(appConfig.Token) == 0 {
		token := appConfig.Token
		for len(token) == 0 {
			token = terminal.Input("Input token")
			if len(token) == 0 {
				terminal.PrintErrorf("Error! Please type token.")
			} else {
				updated = true
				appConfig.Token = token
			}
		}
	}

	return updated, nil
}
