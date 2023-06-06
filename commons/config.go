package commons

import (
	"golang.org/x/xerrors"

	"gopkg.in/yaml.v2"
)

type Config struct {
	Password string
}

func GetDefaultConfig() *Config {
	return &Config{
		Password: "",
	}
}

func (config *Config) GetMDRepoTicket(ticket string) (*MDRepoTicket, error) {
	ticketObj, err := DecodeMDRepoTicket(ticket, config.Password)
	if err != nil {
		return nil, err
	}

	return ticketObj, nil
}

type ConfigTypeIn struct {
	Password string `yaml:"irods_user_password,omitempty"`
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
