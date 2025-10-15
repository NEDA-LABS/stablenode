package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/NEDA-LABS/stablenode/ent"
	"github.com/NEDA-LABS/stablenode/ent/receiveaddress"
	"github.com/NEDA-LABS/stablenode/storage"
)

// DeploymentResult matches the structure from deploy_pool_addresses
type DeploymentResult struct {
	Address     string `json:"address"`
	TxHash      string `json:"tx_hash"`
	BlockNumber uint64 `json:"block_number"`
	Success     bool   `json:"success"`
	Error       string `json:"error,omitempty"`
	GasUsed     uint64 `json:"gas_used"`
}

func main() {
	// Parse flags
	inputFile := flag.String("input", "deployment_results.json", "Input JSON file with deployment results")
	dryRun := flag.Bool("dry-run", false, "Show what would be updated without making changes")
	status := flag.String("status", "pool_ready", "Status to set (pool_ready, unused)")
	flag.Parse()

	log.Printf("Loading deployment results from: %s", *inputFile)

	// Load deployment results
	results, err := loadDeploymentResults(*inputFile)
	if err != nil {
		log.Fatalf("Failed to load deployment results: %v", err)
	}

	log.Printf("Loaded %d deployment results", len(results))

	// Filter successful deployments
	successful := make([]DeploymentResult, 0)
	for _, r := range results {
		if r.Success {
			successful = append(successful, r)
		}
	}

	if len(successful) == 0 {
		log.Fatal("No successful deployments found")
	}

	log.Printf("Found %d successful deployments", len(successful))

	if *dryRun {
		log.Println("\n🔍 DRY RUN MODE - No changes will be made\n")
	}

	// Initialize storage
	dsn := fmt.Sprintf("postgresql://%s:%s@%s:%s/%s?sslmode=%s",
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_NAME"),
		os.Getenv("SSL_MODE"),
	)
	if err := storage.DBConnection(dsn); err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}
	defer storage.Client.Close()

	ctx := context.Background()

	// Update database for each successful deployment
	updated := 0
	skipped := 0
	errors := 0

	for i, result := range successful {
		log.Printf("\n[%d/%d] Processing: %s", i+1, len(successful), result.Address)

		if *dryRun {
			// Dry run - just show what would be updated
			log.Printf("  Would update:")
			log.Printf("    - is_deployed: true")
			log.Printf("    - status: %s", *status)
			log.Printf("    - deployment_tx_hash: %s", result.TxHash)
			log.Printf("    - deployment_block: %d", result.BlockNumber)
			log.Printf("    - deployed_at: %s", time.Now().Format(time.RFC3339))
			updated++
			continue
		}

		// Find the receive address in database
		addr, err := storage.Client.ReceiveAddress.
			Query().
			Where(receiveaddress.AddressEQ(result.Address)).
			Only(ctx)

		if err != nil {
			if ent.IsNotFound(err) {
				log.Printf("  ⚠️  Address not found in database: %s", result.Address)
				skipped++
				continue
			}
			log.Printf("  ✗ Error querying database: %v", err)
			errors++
			continue
		}

		// Check if already marked as deployed
		if addr.IsDeployed {
			log.Printf("  ℹ️  Already marked as deployed (tx: %s)", addr.DeploymentTxHash)
			skipped++
			continue
		}

		// Update the address
		_, err = addr.Update().
			SetIsDeployed(true).
			SetStatus(receiveaddress.Status(*status)).
			SetDeploymentTxHash(result.TxHash).
			SetDeploymentBlock(int64(result.BlockNumber)).
			SetDeployedAt(time.Now()).
			Save(ctx)

		if err != nil {
			log.Printf("  ✗ Failed to update: %v", err)
			errors++
			continue
		}

		log.Printf("  ✓ Updated successfully")
		log.Printf("    - Status: %s", *status)
		log.Printf("    - TxHash: %s", result.TxHash)
		log.Printf("    - Block: %d", result.BlockNumber)
		updated++
	}

	// Print summary
	printSummary(len(successful), updated, skipped, errors, *dryRun)

	if !*dryRun {
		// Verify the updates
		verifyPoolStatus(ctx)
	}
}

// loadDeploymentResults loads deployment results from JSON
func loadDeploymentResults(filename string) ([]DeploymentResult, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var results []DeploymentResult
	if err := json.NewDecoder(file).Decode(&results); err != nil {
		return nil, err
	}

	return results, nil
}

// printSummary prints update summary
func printSummary(total, updated, skipped, errors int, dryRun bool) {
	action := "Updated"
	if dryRun {
		action = "Would update"
	}

	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("UPDATE SUMMARY")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Printf("Total successful deployments: %d\n", total)
	fmt.Printf("%s:                        %d\n", action, updated)
	fmt.Printf("Skipped (already deployed):   %d\n", skipped)
	fmt.Printf("Errors:                       %d\n", errors)
	fmt.Println(strings.Repeat("=", 80))
}

// verifyPoolStatus verifies the pool status after updates
func verifyPoolStatus(ctx context.Context) {
	log.Println("\n📊 Verifying pool status...")

	// Count by status
	statuses := []string{"pool_ready", "pool_assigned", "pool_processing", "pool_completed", "unused", "used"}

	fmt.Println("\nReceive Address Pool Status:")
	fmt.Println(strings.Repeat("-", 50))

	totalDeployed := 0
	for _, status := range statuses {
		count, err := storage.Client.ReceiveAddress.
			Query().
			Where(
				receiveaddress.StatusEQ(receiveaddress.Status(status)),
				receiveaddress.IsDeployedEQ(true),
			).
			Count(ctx)

		if err != nil {
			log.Printf("Error counting %s: %v", status, err)
			continue
		}

		if count > 0 {
			fmt.Printf("%-20s: %d\n", status, count)
			totalDeployed += count
		}
	}

	fmt.Println(strings.Repeat("-", 50))
	fmt.Printf("%-20s: %d\n", "Total Deployed", totalDeployed)

	// Count not deployed
	notDeployed, err := storage.Client.ReceiveAddress.
		Query().
		Where(receiveaddress.IsDeployedEQ(false)).
		Count(ctx)

	if err == nil && notDeployed > 0 {
		fmt.Printf("%-20s: %d\n", "Not Deployed", notDeployed)
	}

	// Show available pool size per network
	fmt.Println("\nAvailable Pool Size by Network:")
	fmt.Println(strings.Repeat("-", 50))

	// This requires a more complex query - simplified version
	ready, err := storage.Client.ReceiveAddress.
		Query().
		Where(
			receiveaddress.StatusEQ(receiveaddress.StatusPoolReady),
			receiveaddress.IsDeployedEQ(true),
		).
		All(ctx)

	if err == nil {
		networkCounts := make(map[string]int)
		for _, addr := range ready {
			networkCounts[addr.NetworkIdentifier]++
		}

		for network, count := range networkCounts {
			fmt.Printf("%-20s: %d addresses ready\n", network, count)
		}
	}

	fmt.Println(strings.Repeat("-", 50))
}
