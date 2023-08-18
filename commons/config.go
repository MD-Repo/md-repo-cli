package commons

import (
	"strings"

	"golang.org/x/xerrors"

	"gopkg.in/yaml.v2"
)

type Config struct {
	NoPassword bool
	Password   string
}

func GetDefaultConfig() *Config {
	return &Config{
		NoPassword: false,
		Password:   "",
	}
}

func (config *Config) GetMDRepoTickets(ticket string) ([]MDRepoTicket, error) {
	ticket = strings.TrimSpace(ticket)

	if config.NoPassword {
		// plaintext ticket string
		return GetMDRepoTicketsFromPlainText(ticket)
	}

	return DecodeMDRepoTickets(ticket, config.Password)
}

func (config *Config) ToConfigTypeIn() *ConfigTypeIn {
	return &ConfigTypeIn{
		NoPassword: config.NoPassword,
		Password:   config.Password,
	}
}

type ConfigTypeIn struct {
	NoPassword bool   `yaml:"no_password,omitempty"`
	Password   string `yaml:"irods_user_password,omitempty"`
}

// NewConfigTypeInFromYAML creates ConfigTypeIn from YAML
func NewConfigTypeInFromYAML(yamlBytes []byte) (*ConfigTypeIn, error) {
	config := &ConfigTypeIn{}

	err := yaml.Unmarshal(yamlBytes, config)
	if err != nil {
		return nil, xerrors.Errorf("failed to unmarshal YAML: %w", err)
	}

	return config, nil
}

func (config *ConfigTypeIn) ToYAML() ([]byte, error) {
	yamlBytes, err := yaml.Marshal(config)
	if err != nil {
		return nil, xerrors.Errorf("failed to marshal to YAML: %w", err)
	}
	return yamlBytes, nil
}
