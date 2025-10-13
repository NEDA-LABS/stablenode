package main

import (
	"context"
	"fmt"

	"github.com/NEDA-LABS/stablenode/config"
	"github.com/NEDA-LABS/stablenode/ent"
	"github.com/NEDA-LABS/stablenode/storage"
	"github.com/NEDA-LABS/stablenode/utils/logger"
	"github.com/shopspring/decimal"
	"github.com/spf13/viper"
)

// List all receive addresses and their balances

func main() {
	fmt.Println("📊 Receive Address Balances")
	fmt.Println("============================")
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

	ctx := context.Background()

	// Get all receive addresses
	addresses, err := storage.Client.ReceiveAddress.
		Query().
		WithPaymentOrder(func(q *ent.PaymentOrderQuery) {
			q.WithToken(func(tq *ent.TokenQuery) {
				tq.WithNetwork()
			})
		}).
		All(ctx)

	if err != nil {
		logger.Fatalf("Failed to fetch addresses: %v", err)
	}

	if len(addresses) == 0 {
		fmt.Println("No Alchemy receive addresses found")
		return
	}

	fmt.Printf("Found %d receive addresses\n\n", len(addresses))

	totalValue := decimal.Zero
	addressesWithFunds := 0

	for i, addr := range addresses {
		if addr.Edges.PaymentOrder == nil {
			continue
		}

		order := addr.Edges.PaymentOrder
		token := order.Edges.Token
		network := token.Edges.Network

		fmt.Printf("%d. Address: %s\n", i+1, addr.Address)
		fmt.Printf("   Network: %s\n", network.Identifier)
		fmt.Printf("   Token: %s\n", token.Symbol)
		fmt.Printf("   Order ID: %s\n", order.ID)

		// Get balance
		balance, err := getBalance(ctx, network, token, addr.Address)
		if err != nil {
			fmt.Printf("   Balance: Error - %v\n", err)
		} else {
			fmt.Printf("   Balance: %s %s\n", balance, token.Symbol)
			if balance.GreaterThan(decimal.Zero) {
				addressesWithFunds++
				totalValue = totalValue.Add(balance)
			}
		}
		fmt.Println()
	}

	fmt.Println("============================")
	fmt.Printf("Addresses with funds: %d\n", addressesWithFunds)
	fmt.Printf("Total tokens: %s\n", totalValue)
	fmt.Println()

	if addressesWithFunds > 0 {
		fmt.Println("To withdraw funds, use:")
		fmt.Println("  go run cmd/withdraw_funds/main.go <address> <destination> <amount> <token> <network>")
	}
}

func getBalance(ctx context.Context, network *ent.Network, token *ent.Token, address string) (decimal.Decimal, error) {
	// This would use the polling service's getTokenBalance method
	// For now, return zero as placeholder
	// You'll need to expose this method or duplicate the logic
	_ = ctx
	_ = network
	_ = token
	_ = address
	return decimal.Zero, fmt.Errorf("balance check not implemented - use polling service")
}
