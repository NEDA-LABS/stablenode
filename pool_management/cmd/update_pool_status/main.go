package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/NEDA-LABS/stablenode/config"
	"github.com/NEDA-LABS/stablenode/ent/receiveaddress"
	"github.com/NEDA-LABS/stablenode/storage"
)

type PoolAddress struct {
	Address           string `json:"address"`
	Salt              string `json:"salt"`
	OwnerAddress      string `json:"owner_address"`
	NetworkIdentifier string `json:"network_identifier"`
	ChainID           int64  `json:"chain_id"`
}

func main() {
	fmt.Println("=== Update Pool Address Status ===\n")

	if len(os.Args) < 2 {
		log.Fatal("Usage: go run main.go <pool_json_file>")
	}

	jsonFile := os.Args[1]
	fmt.Printf("Loading pool addresses from: %s\n\n", jsonFile)

	// Load pool addresses from JSON
	file, err := os.Open(jsonFile)
	if err != nil {
		log.Fatalf("Failed to open file: %v", err)
	}
	defer file.Close()

	var poolAddresses []PoolAddress
	if err := json.NewDecoder(file).Decode(&poolAddresses); err != nil {
		log.Fatalf("Failed to parse JSON: %v", err)
	}

	fmt.Printf("Found %d addresses in JSON file\n\n", len(poolAddresses))

	// Load configuration
	if err := config.SetupConfig(); err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Connect to database
	DSN := config.DBConfig()
	if err := storage.DBConnection(DSN); err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}
	defer storage.Client.Close()

	ctx := context.Background()

	// Update each address
	updated := 0
	skipped := 0
	errors := 0

	for i, poolAddr := range poolAddresses {
		fmt.Printf("[%d/%d] Processing: %s\n", i+1, len(poolAddresses), poolAddr.Address)

		// Find all receive_address rows with this address
		addresses, err := storage.Client.ReceiveAddress.
			Query().
			Where(receiveaddress.AddressEQ(poolAddr.Address)).
			All(ctx)

		if err != nil {
			fmt.Printf("  ✗ Error querying database: %v\n", err)
			errors++
			continue
		}

		if len(addresses) == 0 {
			fmt.Printf("  ⚠️  Address not found in database\n")
			skipped++
			continue
		}

		fmt.Printf("  Found %d row(s) in database\n", len(addresses))

		// Update all rows with this address
		for _, addr := range addresses {
			// Check if already marked as deployed and pool_ready
			if addr.IsDeployed && addr.Status == receiveaddress.StatusPoolReady {
				fmt.Printf("  ℹ️  Row ID %s already marked as deployed and pool_ready\n", addr.ID)
				skipped++
				continue
			}

			// Update the address
			_, err = addr.Update().
				SetIsDeployed(true).
				SetStatus(receiveaddress.StatusPoolReady).
				SetDeployedAt(time.Now()).
				Save(ctx)

			if err != nil {
				fmt.Printf("  ✗ Failed to update row ID %s: %v\n", addr.ID, err)
				errors++
				continue
			}

			fmt.Printf("  ✓ Updated row ID %s to pool_ready\n", addr.ID)
			updated++
		}
		fmt.Println()
	}

	// Print summary
	fmt.Println("=====================================")
	fmt.Println("UPDATE SUMMARY")
	fmt.Println("=====================================")
	fmt.Printf("Total addresses:     %d\n", len(poolAddresses))
	fmt.Printf("Rows updated:        %d\n", updated)
	fmt.Printf("Rows skipped:        %d\n", skipped)
	fmt.Printf("Errors:              %d\n", errors)
	fmt.Println("=====================================\n")

	// Show current pool status
	fmt.Println("Current Pool Status:")
	fmt.Println("-------------------------------------")

	ready, err := storage.Client.ReceiveAddress.
		Query().
		Where(
			receiveaddress.StatusEQ(receiveaddress.StatusPoolReady),
			receiveaddress.IsDeployedEQ(true),
		).
		Count(ctx)

	if err == nil {
		fmt.Printf("Pool Ready:          %d\n", ready)
	}

	assigned, err := storage.Client.ReceiveAddress.
		Query().
		Where(
			receiveaddress.StatusEQ(receiveaddress.StatusPoolAssigned),
		).
		Count(ctx)

	if err == nil {
		fmt.Printf("Pool Assigned:       %d\n", assigned)
	}

	used, err := storage.Client.ReceiveAddress.
		Query().
		Where(
			receiveaddress.StatusEQ(receiveaddress.StatusUsed),
		).
		Count(ctx)

	if err == nil {
		fmt.Printf("Used:                %d\n", used)
	}

	fmt.Println("=====================================")
}
