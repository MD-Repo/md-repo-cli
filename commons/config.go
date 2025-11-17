package commons

import (
	"github.com/cockroachdb/errors"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Token        string
	TicketString string
}

func GetDefaultConfig() *Config {
	return &Config{
		Token:        "",
		TicketString: "",
	}
}

func (config *Config) ToConfigTypeIn() *ConfigTypeIn {
	return &ConfigTypeIn{
		TicketString: config.TicketString,
	}
}

type ConfigTypeIn struct {
	TicketString string `yaml:"ticket_string,omitempty"`
}

// NewConfigTypeInFromYAML creates ConfigTypeIn from YAML
func NewConfigTypeInFromYAML(yamlBytes []byte) (*ConfigTypeIn, error) {
	config := &ConfigTypeIn{}

	err := yaml.Unmarshal(yamlBytes, config)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to unmarshal YAML")
	}

	return config, nil
}

// ToYAML converts to YAML bytes
func (config *ConfigTypeIn) ToYAML() ([]byte, error) {
	yamlBytes, err := yaml.Marshal(config)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to marshal to YAML")
	}
	return yamlBytes, nil
}
