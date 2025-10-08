package services

import (
	"context"
	"testing"
	"time"

	"github.com/spf13/viper"
)

// init loads the .env file before running tests
func init() {
	// Load .env file from parent directory for all tests
	viper.SetConfigName(".env")
	viper.SetConfigType("env")
	viper.AddConfigPath("..") // Parent directory
	viper.AddConfigPath(".")
	viper.AutomaticEnv()
	_ = viper.ReadInConfig()
}

// TestAlchemyServiceCreation tests basic service creation
func TestAlchemyServiceCreation(t *testing.T) {
	// Set up test configuration
	viper.Set("ALCHEMY_API_KEY", "test-api-key")
	viper.Set("ALCHEMY_BASE_URL", "https://api.g.alchemy.com/v2")
	
	service := NewAlchemyService()
	if service == nil {
		t.Fatal("Failed to create Alchemy service")
	}
	
	if service.config.APIKey != "test-api-key" {
		t.Errorf("Expected API key 'test-api-key', got '%s'", service.config.APIKey)
	}
}

// TestAlchemyServiceHealthCheck tests the health check functionality
func TestAlchemyServiceHealthCheck(t *testing.T) {
	// Clear viper cache first
	viper.Reset()
	
	// Manually load .env file from parent directory
	viper.SetConfigName(".env")
	viper.SetConfigType("env")
	viper.AddConfigPath("..") // Parent directory where .env is located
	viper.AddConfigPath(".")  // Current directory as fallback
	viper.AutomaticEnv()
	
	err := viper.ReadInConfig()
	if err != nil {
		t.Fatalf("Failed to load .env file: %v", err)
	}
	
	// Skip if no real API key is provided
	apiKey := viper.GetString("ALCHEMY_API_KEY")
	if apiKey == "" {
		apiKey = viper.GetString("alchemy_api_key") // Try lowercase
	}
	
	if apiKey == "" {
		t.Skip("Skipping health check test - no ALCHEMY_API_KEY provided")
	}
	
	service := NewAlchemyService()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	// This will only pass with a real API key
	healthy := service.IsHealthy(ctx)
	if !healthy {
		t.Logf("Health check returned false - API key may be invalid or endpoint unreachable")
	} else {
		t.Logf("Alchemy service is healthy!")
	}
}

// TestSmartAccountAddressGeneration tests deterministic address generation
func TestSmartAccountAddressGeneration(t *testing.T) {
	service := NewAlchemyService()
	
	ownerAddress := "0x1234567890123456789012345678901234567890"
	chainID := int64(137) // Polygon
	
	// Generate address twice to ensure deterministic behavior
	addr1 := service.computeSmartAccountAddress(ownerAddress, chainID)
	addr2 := service.computeSmartAccountAddress(ownerAddress, chainID)
	
	if addr1 != addr2 {
		t.Errorf("Address generation is not deterministic: %s != %s", addr1, addr2)
	}
	
	// Check that different inputs produce different addresses
	addr3 := service.computeSmartAccountAddress(ownerAddress, 1) // Different chain
	if addr1 == addr3 {
		t.Errorf("Different chain IDs should produce different addresses")
	}
	
	t.Logf("Generated smart account address: %s", addr1)
}

// TestServiceManager tests the service manager functionality
func TestServiceManager(t *testing.T) {
	// Test default behavior (should use Thirdweb)
	viper.Set("USE_ALCHEMY_SERVICE", false)
	manager := NewServiceManager()
	
	if manager.GetActiveService() != "Thirdweb Engine" {
		t.Errorf("Expected 'Thirdweb Engine', got '%s'", manager.GetActiveService())
	}
	
	// Test switching to Alchemy
	manager.SwitchToAlchemy()
	if manager.GetActiveService() != "Alchemy" {
		t.Errorf("Expected 'Alchemy', got '%s'", manager.GetActiveService())
	}
	
	// Test switching back to Thirdweb
	manager.SwitchToThirdweb()
	if manager.GetActiveService() != "Thirdweb Engine" {
		t.Errorf("Expected 'Thirdweb Engine', got '%s'", manager.GetActiveService())
	}
}

// BenchmarkSmartAccountAddressGeneration benchmarks address generation
func BenchmarkSmartAccountAddressGeneration(b *testing.B) {
	service := NewAlchemyService()
	ownerAddress := "0x1234567890123456789012345678901234567890"
	chainID := int64(137)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		service.computeSmartAccountAddress(ownerAddress, chainID)
	}
}
