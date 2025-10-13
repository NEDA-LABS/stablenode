package utils

import (
	"fmt"
	"strings"
	
	"github.com/spf13/viper"
)

// BuildRPCURL constructs the full RPC URL by appending API key from environment if needed
func BuildRPCURL(baseURL string) string {
	// If URL already contains an API key (has more than 3 path segments), return as-is
	if strings.Count(baseURL, "/") > 3 {
		return baseURL
	}

	// Check if this is an Alchemy URL
	if strings.Contains(baseURL, "alchemy.com") {
		alchemyAPIKey := viper.GetString("ALCHEMY_API_KEY")
		if alchemyAPIKey == "" {
			// Log warning but return base URL (will fail with 401)
			fmt.Println("WARNING: ALCHEMY_API_KEY not set in environment")
			return baseURL
		}
		// Append API key to Alchemy URL
		return fmt.Sprintf("%s/%s", strings.TrimSuffix(baseURL, "/"), alchemyAPIKey)
	}

	// Check if this is an Infura URL
	if strings.Contains(baseURL, "infura.io") {
		infuraAPIKey := viper.GetString("INFURA_API_KEY")
		if infuraAPIKey != "" {
			// Replace placeholder or append API key
			if strings.Contains(baseURL, "YOUR_INFURA_KEY") {
				return strings.Replace(baseURL, "YOUR_INFURA_KEY", infuraAPIKey, 1)
			}
			return fmt.Sprintf("%s/%s", strings.TrimSuffix(baseURL, "/"), infuraAPIKey)
		}
	}

	// For other RPC providers, return as-is
	return baseURL
}

// GetAlchemyAPIKey returns the Alchemy API key from environment
func GetAlchemyAPIKey() string {
	return viper.GetString("ALCHEMY_API_KEY")
}

// GetInfuraAPIKey returns the Infura API key from environment
func GetInfuraAPIKey() string {
	return viper.GetString("INFURA_API_KEY")
}
