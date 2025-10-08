package config

import (
	"github.com/spf13/viper"
)

// AlchemyConfiguration holds the configuration for Alchemy integration
type AlchemyConfiguration struct {
	APIKey      string
	BaseURL     string
	GasPolicyID string // Optional - for gas sponsorship
}

// AlchemyConfig returns the Alchemy configuration
func AlchemyConfig() *AlchemyConfiguration {
	return &AlchemyConfiguration{
		APIKey:      viper.GetString("ALCHEMY_API_KEY"),
		BaseURL:     viper.GetString("ALCHEMY_BASE_URL"),
		GasPolicyID: viper.GetString("ALCHEMY_GAS_POLICY_ID"),
	}
}
