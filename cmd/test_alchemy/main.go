package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/NEDA-LABS/stablenode/config"
	networkent "github.com/NEDA-LABS/stablenode/ent/network"
	"github.com/NEDA-LABS/stablenode/services"
	"github.com/NEDA-LABS/stablenode/storage"
	"github.com/spf13/viper"
)

func main() {
	// Load environment variables
	viper.SetConfigFile(".env")
	if err := viper.ReadInConfig(); err != nil {
		fmt.Printf("Error reading config file: %v\n", err)
		os.Exit(1)
	}

	// Initialize database connection
	DSN := config.DBConfig()
	if err := storage.DBConnection(DSN); err != nil {
		fmt.Printf("Database connection failed: %v\n", err)
		os.Exit(1)
	}
	defer storage.GetClient().Close()

	ctx := context.Background()

	// Test parameters
	chainID := int64(84532)      // Base Sepolia
	ownerAddress := viper.GetString("SMART_ACCOUNT_OWNER_ADDRESS")
	
	if ownerAddress == "" {
		fmt.Println("❌ SMART_ACCOUNT_OWNER_ADDRESS not set in .env")
		os.Exit(1)
	}

	fmt.Println("🧪 Testing Alchemy Smart Account Creation")
	fmt.Println("==========================================")
	fmt.Printf("Chain ID: %d\n", chainID)
	fmt.Printf("Owner Address: %s\n", ownerAddress)
	fmt.Println()

	// Get network info
	network, err := storage.GetClient().Network.
		Query().
		Where(networkent.ChainIDEQ(chainID)).
		Only(ctx)
	
	if err != nil {
		fmt.Printf("❌ Network not found: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Network: %s\n", network.Identifier)
	fmt.Printf("RPC Endpoint: %s\n", network.RPCEndpoint)
	fmt.Println()

	// Create Alchemy service
	alchemyService := services.NewAlchemyService()

	// Test smart account creation
	fmt.Println("📝 Creating smart account...")
	smartAccountAddress, err := alchemyService.CreateSmartAccount(ctx, chainID, ownerAddress)
	
	if err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✅ Success!")
	fmt.Printf("Smart Account Address: %s\n", smartAccountAddress)
	
	// Pretty print the result
	result := map[string]interface{}{
		"chainID":             chainID,
		"ownerAddress":        ownerAddress,
		"smartAccountAddress": smartAccountAddress,
		"network":             network.Identifier,
	}
	
	jsonOutput, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println("\n📋 Result:")
	fmt.Println(string(jsonOutput))
}
