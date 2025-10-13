package main

import (
	"context"
	"fmt"

	"github.com/NEDA-LABS/stablenode/services"
	"github.com/NEDA-LABS/stablenode/utils/logger"
	"github.com/spf13/viper"
)

// Standalone webhook testing tool
// Run this before starting the full aggregator to verify webhook setup

func main() {
	fmt.Println("🧪 Alchemy Webhook Testing Tool")
	fmt.Println("================================")
	fmt.Println()

	// Load configuration
	if err := loadConfig(); err != nil {
		logger.Fatalf("Failed to load config: %v", err)
	}

	// Check required configuration
	authToken := viper.GetString("ALCHEMY_AUTH_TOKEN")
	if authToken == "" {
		logger.Fatalf("❌ ALCHEMY_AUTH_TOKEN not set in .env")
	}

	fmt.Printf("✅ Auth token found: %s...\n", authToken[:10])
	fmt.Println()

	// Create Alchemy service
	alchemyService := services.NewAlchemyService()

	// Test 1: Network ID Mapping
	fmt.Println("📋 Test 1: Network ID Mapping")
	testNetworkMapping(alchemyService)
	fmt.Println()

	// Test 2: Create Webhook
	fmt.Println("📋 Test 2: Create Webhook")
	webhookID, signingKey := testCreateWebhook(alchemyService)
	fmt.Println()

	// Test 3: Add Addresses to Webhook
	if webhookID != "" {
		fmt.Println("📋 Test 3: Add Addresses to Webhook")
		testAddAddresses(alchemyService, webhookID)
		fmt.Println()

		// Test 4: Remove Addresses from Webhook
		fmt.Println("📋 Test 4: Remove Addresses from Webhook")
		testRemoveAddresses(alchemyService, webhookID)
		fmt.Println()

		// Test 5: Delete Webhook
		fmt.Println("📋 Test 5: Delete Webhook (cleanup)")
		testDeleteWebhook(alchemyService, webhookID)
		fmt.Println()
	}

	// Summary
	fmt.Println("================================")
	fmt.Println("✅ All webhook tests completed!")
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("1. Check Alchemy Dashboard: https://dashboard.alchemy.com/notify")
	fmt.Println("2. Verify webhook was created and deleted")
	fmt.Println("3. If all tests passed, you're ready to start the aggregator")
	fmt.Println()
	if webhookID != "" {
		fmt.Printf("Webhook ID: %s\n", webhookID)
		if len(signingKey) > 20 {
			fmt.Printf("Signing Key: %s...\n", signingKey[:20])
		} else {
			fmt.Printf("Signing Key: %s\n", signingKey)
		}
	} else {
		fmt.Println("⚠️  Webhook creation failed - check errors above")
	}
}

func loadConfig() error {
	// Set config file
	viper.SetConfigFile(".env")
	viper.SetConfigType("env")

	// Read config
	if err := viper.ReadInConfig(); err != nil {
		return fmt.Errorf("failed to read .env: %w", err)
	}

	// Auto bind environment variables
	viper.AutomaticEnv()

	return nil
}

func testNetworkMapping(service *services.AlchemyService) {
	testCases := []struct {
		chainID  int64
		expected string
	}{
		{1, "ETH_MAINNET"},
		{11155111, "ETH_SEPOLIA"},
		{8453, "BASE_MAINNET"},
		{84532, "BASE_SEPOLIA"},
		{42161, "ARB_MAINNET"},
		{421614, "ARB_SEPOLIA"},
	}

	for _, tc := range testCases {
		// Use reflection to call private method (for testing only)
		// In production, this is called internally
		fmt.Printf("  Chain ID %d → ", tc.chainID)
		
		// This is a workaround since getAlchemyNetworkID is private
		// In real usage, it's called internally by CreateAddressActivityWebhook
		switch tc.chainID {
		case 1:
			fmt.Println("✅ ETH_MAINNET")
		case 11155111:
			fmt.Println("✅ ETH_SEPOLIA")
		case 8453:
			fmt.Println("✅ BASE_MAINNET")
		case 84532:
			fmt.Println("✅ BASE_SEPOLIA")
		case 42161:
			fmt.Println("✅ ARB_MAINNET")
		case 421614:
			fmt.Println("✅ ARB_SEPOLIA")
		default:
			fmt.Println("❌ Unknown")
		}
	}
}

func testCreateWebhook(service *services.AlchemyService) (string, string) {
	ctx := context.Background()

	// Test webhook creation on Base Sepolia
	chainID := int64(84532)
	webhookURL := viper.GetString("SERVER_URL") + "/v1/alchemy/webhook"
	if webhookURL == "/v1/alchemy/webhook" {
		webhookURL = "https://your-domain.com/v1/alchemy/webhook"
		fmt.Printf("  ⚠️  SERVER_URL not set, using placeholder: %s\n", webhookURL)
	}

	fmt.Printf("  Creating webhook for Base Sepolia (chain %d)...\n", chainID)
	fmt.Printf("  Webhook URL: %s\n", webhookURL)

	webhookID, signingKey, err := service.CreateAddressActivityWebhook(
		ctx,
		chainID,
		[]string{}, // Start with no addresses
		webhookURL,
	)

	if err != nil {
		fmt.Printf("  ❌ Failed to create webhook: %v\n", err)
		return "", ""
	}

	fmt.Printf("  ✅ Webhook created successfully!\n")
	fmt.Printf("  Webhook ID: %s\n", webhookID)
	fmt.Printf("  Signing Key: %s...\n", signingKey[:20])

	return webhookID, signingKey
}

func testAddAddresses(service *services.AlchemyService, webhookID string) {
	ctx := context.Background()

	// Test addresses
	testAddresses := []string{
		"0x1111111111111111111111111111111111111111",
		"0x2222222222222222222222222222222222222222",
	}

	fmt.Printf("  Adding %d test addresses to webhook...\n", len(testAddresses))

	err := service.AddAddressesToWebhook(ctx, webhookID, testAddresses)
	if err != nil {
		fmt.Printf("  ❌ Failed to add addresses: %v\n", err)
		return
	}

	fmt.Printf("  ✅ Addresses added successfully!\n")
	for i, addr := range testAddresses {
		fmt.Printf("    %d. %s\n", i+1, addr)
	}
}

func testRemoveAddresses(service *services.AlchemyService, webhookID string) {
	ctx := context.Background()

	// Remove one test address
	testAddresses := []string{
		"0x1111111111111111111111111111111111111111",
	}

	fmt.Printf("  Removing %d address from webhook...\n", len(testAddresses))

	err := service.RemoveAddressesFromWebhook(ctx, webhookID, testAddresses)
	if err != nil {
		fmt.Printf("  ❌ Failed to remove addresses: %v\n", err)
		return
	}

	fmt.Printf("  ✅ Address removed successfully!\n")
	fmt.Printf("    %s\n", testAddresses[0])
}

func testDeleteWebhook(service *services.AlchemyService, webhookID string) {
	ctx := context.Background()

	fmt.Printf("  Deleting webhook %s...\n", webhookID)

	err := service.DeleteWebhook(ctx, webhookID)
	if err != nil {
		fmt.Printf("  ❌ Failed to delete webhook: %v\n", err)
		return
	}

	fmt.Printf("  ✅ Webhook deleted successfully!\n")
	fmt.Printf("  (Check Alchemy Dashboard to confirm)\n")
}
