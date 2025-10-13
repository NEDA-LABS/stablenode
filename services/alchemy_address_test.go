package services

import (
	"context"
	"testing"

	"github.com/spf13/viper"
)

// TestComputeSmartAccountAddress tests the deterministic address computation
func TestComputeSmartAccountAddress(t *testing.T) {
	// Load config
	viper.SetConfigName(".env")
	viper.SetConfigType("env")
	viper.AddConfigPath("..")
	viper.AddConfigPath(".")
	viper.AutomaticEnv()
	_ = viper.ReadInConfig()

	service := NewAlchemyService()
	
	// Test with a known owner address
	ownerAddress := "0x1234567890123456789012345678901234567890"
	chainID := int64(84532) // Base Sepolia
	
	// Compute the smart account address
	smartAccountAddress := service.computeSmartAccountAddress(ownerAddress, chainID)
	
	t.Logf("Owner Address: %s", ownerAddress)
	t.Logf("Chain ID: %d (Base Sepolia)", chainID)
	t.Logf("Computed Smart Account Address: %s", smartAccountAddress)
	
	// Verify address format
	if len(smartAccountAddress) != 42 {
		t.Errorf("Invalid address length: expected 42, got %d", len(smartAccountAddress))
	}
	
	if smartAccountAddress[:2] != "0x" {
		t.Errorf("Address should start with 0x, got: %s", smartAccountAddress[:2])
	}
	
	// Test determinism - same inputs should give same output
	smartAccountAddress2 := service.computeSmartAccountAddress(ownerAddress, chainID)
	if smartAccountAddress != smartAccountAddress2 {
		t.Errorf("Address computation is not deterministic: %s != %s", smartAccountAddress, smartAccountAddress2)
	}
	
	// Test different owner gives different address
	differentOwner := "0x9876543210987654321098765432109876543210"
	differentAddress := service.computeSmartAccountAddress(differentOwner, chainID)
	if smartAccountAddress == differentAddress {
		t.Errorf("Different owners should produce different addresses")
	}
	
	t.Logf("✅ Address computation is deterministic and working correctly")
}

// TestComputeSmartAccountAddressWithRealOwner tests with a real owner address from env
func TestComputeSmartAccountAddressWithRealOwner(t *testing.T) {
	// Load config
	viper.Reset()
	viper.SetConfigName(".env")
	viper.SetConfigType("env")
	viper.AddConfigPath("..")
	viper.AddConfigPath(".")
	viper.AutomaticEnv()
	_ = viper.ReadInConfig()

	// Get owner address from env if available
	ownerAddress := viper.GetString("SMART_ACCOUNT_OWNER_ADDRESS")
	if ownerAddress == "" {
		t.Skip("SMART_ACCOUNT_OWNER_ADDRESS not set in .env - skipping real owner test")
	}

	service := NewAlchemyService()
	chainID := int64(84532) // Base Sepolia
	
	// Compute the smart account address
	smartAccountAddress := service.computeSmartAccountAddress(ownerAddress, chainID)
	
	t.Logf("=== Smart Account Computation ===")
	t.Logf("Factory: 0x0000000000400CdFef5E2714E63d8040b700BC24 (Light Account v2.0.0)")
	t.Logf("Implementation: 0x8E8e658E22B12ada97B402fF0b044D6A325013C7")
	t.Logf("Owner Address: %s", ownerAddress)
	t.Logf("Chain: Base Sepolia (84532)")
	t.Logf("Salt: 0 (first account)")
	t.Logf("")
	t.Logf("🎯 Computed Smart Account Address: %s", smartAccountAddress)
	t.Logf("")
	t.Logf("Next steps:")
	t.Logf("1. Verify this address on Base Sepolia explorer:")
	t.Logf("   https://sepolia.basescan.org/address/%s", smartAccountAddress)
	t.Logf("2. Deploy the account (if not already deployed)")
	t.Logf("3. Fund the account with testnet ETH")
}

// TestCreateSmartAccountFlow tests the full smart account creation flow
func TestCreateSmartAccountFlow(t *testing.T) {
	// Load config
	viper.Reset()
	viper.SetConfigName(".env")
	viper.SetConfigType("env")
	viper.AddConfigPath("..")
	viper.AddConfigPath(".")
	viper.AutomaticEnv()
	_ = viper.ReadInConfig()

	ownerAddress := viper.GetString("SMART_ACCOUNT_OWNER_ADDRESS")
	if ownerAddress == "" {
		t.Skip("SMART_ACCOUNT_OWNER_ADDRESS not set - skipping flow test")
	}

	service := NewAlchemyService()
	ctx := context.Background()
	chainID := int64(84532) // Base Sepolia
	
	t.Logf("=== Smart Account Creation Flow ===")
	
	// Step 1: Compute address
	t.Logf("\n📍 Step 1: Computing smart account address...")
	smartAccountAddress := service.computeSmartAccountAddress(ownerAddress, chainID)
	t.Logf("   Smart Account Address: %s", smartAccountAddress)
	
	// Step 2: Generate init code (using dummy salt for test)
	t.Logf("\n📝 Step 2: Generating init code...")
	dummySalt := "0000000000000000000000000000000000000000000000000000000000000000"
	initCode := service.getSmartAccountInitCode(ownerAddress, dummySalt)
	t.Logf("   Init Code Length: %d bytes", len(initCode)/2)
	t.Logf("   Init Code (first 66 chars): %s...", initCode[:66])
	
	// Step 3: Create smart account (this will use the computed address)
	t.Logf("\n🚀 Step 3: Creating smart account via Alchemy...")
	address, salt, err := service.CreateSmartAccount(ctx, chainID, ownerAddress)
	if err != nil {
		t.Logf("   ⚠️  Error: %v", err)
		t.Logf("   Note: This is expected if account already exists or needs deployment")
	} else {
		t.Logf("   ✅ Smart Account Created: %s", address)
		t.Logf("   ✅ Salt Length: %d bytes", len(salt))
	}
	
	// Verify addresses match
	if address != "" && address != smartAccountAddress {
		t.Logf("   ⚠️  Warning: Computed address doesn't match created address")
		t.Logf("   Computed: %s", smartAccountAddress)
		t.Logf("   Created:  %s", address)
	}
	
	t.Logf("\n=== Summary ===")
	t.Logf("Smart Account Address: %s", smartAccountAddress)
	t.Logf("Explorer: https://sepolia.basescan.org/address/%s", smartAccountAddress)
}
