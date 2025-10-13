package main

import (
	"context"
	"fmt"
	"os"

	"github.com/NEDA-LABS/stablenode/config"
	"github.com/NEDA-LABS/stablenode/ent/network"
	"github.com/NEDA-LABS/stablenode/ent/token"
	"github.com/NEDA-LABS/stablenode/storage"
	"github.com/NEDA-LABS/stablenode/utils/logger"
	"github.com/shopspring/decimal"
	"github.com/spf13/viper"
)

// Withdraw funds from a receive address back to a destination address
// Usage: go run cmd/withdraw_funds/main.go <receive_address> <destination_address> <amount> <token_symbol> <network>

func main() {
	fmt.Println("💰 Withdraw Funds from Receive Address")
	fmt.Println("======================================")
	fmt.Println()

	// Load configuration
	viper.SetConfigFile(".env")
	viper.SetConfigType("env")
	if err := viper.ReadInConfig(); err != nil {
		logger.Fatalf("Failed to read .env: %v", err)
	}
	viper.AutomaticEnv()

	// Connect to database
	DSN := config.DBConfig()
	if err := storage.DBConnection(DSN); err != nil {
		logger.Fatalf("Database connection failed: %s", err)
	}
	defer storage.GetClient().Close()

	// Parse command line arguments
	if len(os.Args) < 6 {
		fmt.Println("Usage: go run cmd/withdraw_funds/main.go <receive_address> <destination_address> <amount> <token_symbol> <network>")
		fmt.Println()
		fmt.Println("Example:")
		fmt.Println("  go run cmd/withdraw_funds/main.go \\")
		fmt.Println("    0x1234... \\")
		fmt.Println("    0x5678... \\")
		fmt.Println("    0.5 \\")
		fmt.Println("    DAI \\")
		fmt.Println("    base-sepolia")
		os.Exit(1)
	}

	receiveAddress := os.Args[1]
	destinationAddress := os.Args[2]
	amountStr := os.Args[3]
	tokenSymbol := os.Args[4]
	networkIdentifier := os.Args[5]

	// Parse amount
	amount, err := decimal.NewFromString(amountStr)
	if err != nil {
		logger.Fatalf("Invalid amount: %v", err)
	}

	fmt.Printf("From:    %s\n", receiveAddress)
	fmt.Printf("To:      %s\n", destinationAddress)
	fmt.Printf("Amount:  %s %s\n", amount, tokenSymbol)
	fmt.Printf("Network: %s\n", networkIdentifier)
	fmt.Println()

	// Get network from database
	ctx := context.Background()
	networkEntity, err := storage.Client.Network.
		Query().
		Where(network.IdentifierEQ(networkIdentifier)).
		Only(ctx)

	if err != nil {
		logger.Fatalf("Network not found: %v", err)
	}

	// Get token from database
	tokenEntity, err := storage.Client.Token.
		Query().
		Where(
			token.SymbolEQ(tokenSymbol),
			token.HasNetworkWith(network.IDEQ(networkEntity.ID)),
		).
		Only(ctx)

	if err != nil {
		logger.Fatalf("Token not found: %v", err)
	}

	fmt.Printf("Token Contract: %s\n", tokenEntity.ContractAddress)
	fmt.Printf("Chain ID: %d\n", networkEntity.ChainID)
	fmt.Println()

	// Convert amount to wei (smallest unit)
	amountWei := amount.Mul(decimal.NewFromInt(10).Pow(decimal.NewFromInt(int64(tokenEntity.Decimals))))

	fmt.Println("Sending transaction...")
	fmt.Println("⚠️  Note: SendTransaction method needs to be implemented in AlchemyService")
	fmt.Printf("From: %s\n", receiveAddress)
	fmt.Printf("To: %s\n", destinationAddress)
	fmt.Printf("Token: %s\n", tokenEntity.ContractAddress)
	fmt.Printf("Amount: %s\n", amountWei.String())

	// TODO: Implement SendTransaction in AlchemyService
	// For now, just show what would be sent
	txHash := "0x0000000000000000000000000000000000000000000000000000000000000000"
	err = fmt.Errorf("SendTransaction not yet implemented")

	if err != nil {
		logger.Warnf("Transaction not sent: %v", err)
		logger.Infof("Use Alchemy Dashboard to withdraw manually")
		return
	}

	fmt.Println()
	fmt.Printf("✅ Transaction sent!\n")
	fmt.Printf("Transaction Hash: %s\n", txHash)
	fmt.Printf("View on explorer: https://%s.etherscan.io/tx/%s\n", networkIdentifier, txHash)
	fmt.Println()
	fmt.Println("Note: Wait for transaction confirmation before checking balance")
}
