package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/NEDA-LABS/stablenode/ent/receiveaddress"
	"github.com/NEDA-LABS/stablenode/storage"
	cryptoUtils "github.com/NEDA-LABS/stablenode/utils/crypto"
)

const (
	// Light Account Factory v2.0.0
	FactoryAddress         = "0x0000000000400CdFef5E2714E63d8040b700BC24"
	ImplementationAddress  = "0x8E8e658E22B12ada97B402fF0b044D6A325013C7"
	
	// Default owner address for pool addresses
	DefaultOwnerAddress = "0xFb84E5503bD20526f2579193411Dd0993d080775"
)

// AddressInfo holds the generated address information
type AddressInfo struct {
	Address         string `json:"address"`
	Salt            string `json:"salt"`
	OwnerAddress    string `json:"owner_address"`
	InitCode        string `json:"init_code"`
	FactoryAddress  string `json:"factory_address"`
	FactoryData     string `json:"factory_data"`
	NetworkID       string `json:"network_identifier"`
	ChainID         int64  `json:"chain_id"`
	DeployCommand   string `json:"deploy_command"`
}

func main() {
	// Parse flags
	count := flag.Int("count", 10, "Number of addresses to create")
	chainID := flag.Int64("chain-id", 84532, "Chain ID (default: Base Sepolia)")
	networkID := flag.String("network", "base-sepolia", "Network identifier")
	owner := flag.String("owner", DefaultOwnerAddress, "Owner address for the smart accounts")
	outputFile := flag.String("output", "pool_addresses.json", "Output JSON file with address details")
	saveToDb := flag.Bool("save-db", false, "Save addresses to database")
	flag.Parse()

	log.Printf("Creating %d receive addresses for chain %d (%s)", *count, *chainID, *networkID)

	// Initialize storage if saving to DB
	if *saveToDb {
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
	}

	ctx := context.Background()
	addresses := make([]AddressInfo, 0, *count)

	// Generate addresses
	for i := 0; i < *count; i++ {
		log.Printf("Generating address %d/%d...", i+1, *count)

		addressInfo, err := generateSmartAccountAddress(*owner, *chainID, *networkID)
		if err != nil {
			log.Printf("ERROR: Failed to generate address %d: %v", i+1, err)
			continue
		}

		addresses = append(addresses, *addressInfo)

		// Save to database if requested
		if *saveToDb {
			if err := saveAddressToDatabase(ctx, addressInfo); err != nil {
				log.Printf("WARNING: Failed to save address to database: %v", err)
			} else {
				log.Printf("✓ Saved to database: %s", addressInfo.Address)
			}
		}

		log.Printf("✓ Generated: %s", addressInfo.Address)
	}

	// Save to JSON file
	if err := saveToJSON(addresses, *outputFile); err != nil {
		log.Fatalf("Failed to save addresses to file: %v", err)
	}

	log.Printf("\n✓ Successfully created %d addresses", len(addresses))
	log.Printf("✓ Address details saved to: %s", *outputFile)
	
	// Print deployment instructions
	printDeploymentInstructions(addresses, *outputFile)
}

// generateSmartAccountAddress generates a smart account address using CREATE2
func generateSmartAccountAddress(ownerAddress string, chainID int64, networkIdentifier string) (*AddressInfo, error) {
	// Generate unique salt
	salt, err := generateUniqueSalt()
	if err != nil {
		return nil, fmt.Errorf("failed to generate salt: %w", err)
	}

	// Compute smart account address using CREATE2
	address := computeSmartAccountAddress(ownerAddress, salt)

	// Generate initCode
	initCode := getSmartAccountInitCode(ownerAddress, salt)

	// Split initCode into factory + factoryData for v0.7
	factoryAddr := "0x" + initCode[2:42]  // First 20 bytes (factory address)
	factoryData := "0x" + initCode[42:]    // Rest is factory data

	return &AddressInfo{
		Address:         address,
		Salt:            fmt.Sprintf("0x%064x", salt),
		OwnerAddress:    ownerAddress,
		InitCode:        initCode,
		FactoryAddress:  factoryAddr,
		FactoryData:     factoryData,
		NetworkID:       networkIdentifier,
		ChainID:         chainID,
		DeployCommand:   generateDeployCommand(address, initCode, networkIdentifier),
	}, nil
}

// generateUniqueSalt generates a unique 32-byte salt
func generateUniqueSalt() ([32]byte, error) {
	var salt [32]byte
	
	// Use current timestamp + random bytes
	timestamp := time.Now().UnixNano()
	
	// Create hash of timestamp + random data
	randomBytes := make([]byte, 32)
	if _, err := rand.Read(randomBytes); err != nil {
		return salt, err
	}
	
	hash := crypto.Keccak256Hash(
		[]byte(fmt.Sprintf("%d", timestamp)),
		randomBytes,
	)
	
	copy(salt[:], hash[:])
	return salt, nil
}

// computeSmartAccountAddress computes the CREATE2 address
func computeSmartAccountAddress(ownerAddress string, salt [32]byte) string {
	factoryAddr := common.HexToAddress(FactoryAddress)
	implementationAddr := common.HexToAddress(ImplementationAddress)
	owner := common.HexToAddress(ownerAddress)

	// Build minimal proxy bytecode (EIP-1167)
	proxyPrefix := common.Hex2Bytes("3d602d80600a3d3981f3363d3d373d3d3d363d73")
	proxySuffix := common.Hex2Bytes("5af43d82803e903d91602b57fd5bf3")

	// Combine: prefix + implementation + suffix + owner (constructor param)
	initCode := append(proxyPrefix, implementationAddr.Bytes()...)
	initCode = append(initCode, proxySuffix...)
	ownerParam := common.LeftPadBytes(owner.Bytes(), 32)
	initCode = append(initCode, ownerParam...)

	// Hash the init code
	initCodeHash := crypto.Keccak256(initCode)

	// CREATE2 formula: keccak256(0xff ++ factory ++ salt ++ keccak256(initCode))
	data := make([]byte, 0, 1+20+32+32)
	data = append(data, 0xff)
	data = append(data, factoryAddr.Bytes()...)
	data = append(data, salt[:]...)
	data = append(data, initCodeHash...)

	// Hash and take last 20 bytes
	hash := crypto.Keccak256(data)
	address := common.BytesToAddress(hash[12:])

	return address.Hex()
}

// getSmartAccountInitCode generates the initCode for deployment
func getSmartAccountInitCode(ownerAddress string, salt [32]byte) string {
	// Factory address (without 0x)
	factory := "0000000000400CdFef5E2714E63d8040b700BC24"
	
	// Function selector for createAccount(address owner, uint256 salt)
	selector := "5fbfb9cf"
	
	// Encode owner address (32 bytes padded)
	ownerPadded := fmt.Sprintf("%064s", common.HexToAddress(ownerAddress).Hex()[2:])
	
	// Encode salt (32 bytes)
	saltHex := fmt.Sprintf("%064x", salt)
	
	// Combine: 0x + factory + selector + owner + salt
	return "0x" + factory + selector + ownerPadded + saltHex
}

// generateDeployCommand generates a cast command for deployment
func generateDeployCommand(address, initCode, network string) string {
	factory := initCode[2:42]       // Factory address
	callData := "0x" + initCode[42:] // Factory call data
	
	return fmt.Sprintf("cast send %s \"%s\" --rpc-url %s --private-key $PRIVATE_KEY",
		"0x"+factory,
		callData,
		network,
	)
}

// saveToJSON saves addresses to a JSON file
func saveToJSON(addresses []AddressInfo, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(addresses)
}

// saveAddressToDatabase saves address to database
func saveAddressToDatabase(ctx context.Context, info *AddressInfo) error {
	// Get salt bytes for storage
	saltBytes := common.Hex2Bytes(info.Salt[2:])
	encryptedSalt, err := cryptoUtils.EncryptPlain(saltBytes)
	if err != nil {
		return fmt.Errorf("failed to encrypt salt: %w", err)
	}

	// Create receive address entry
	_, err = storage.Client.ReceiveAddress.
		Create().
		SetAddress(info.Address).
		SetSalt(encryptedSalt).
		SetStatus(receiveaddress.StatusUnused). // Will update to pool_ready after deployment
		SetIsDeployed(false).                    // Not deployed yet
		SetChainID(info.ChainID).
		SetNetworkIdentifier(info.NetworkID).
		SetTimesUsed(0).
		Save(ctx)

	if err != nil {
		return fmt.Errorf("failed to save to database: %w", err)
	}

	return nil
}

// printDeploymentInstructions prints instructions for deploying the addresses
func printDeploymentInstructions(addresses []AddressInfo, outputFile string) {
	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("DEPLOYMENT INSTRUCTIONS")
	fmt.Println(strings.Repeat("=", 80))
	
	fmt.Println("\n📋 Option 1: Deploy Using Cast (Foundry)")
	fmt.Println("   Install: https://book.getfoundry.sh/getting-started/installation")
	fmt.Println("\n   Commands:")
	for i, addr := range addresses {
		if i < 3 { // Show first 3 as examples
			fmt.Printf("\n   # Deploy address %d: %s\n", i+1, addr.Address)
			fmt.Printf("   %s\n", addr.DeployCommand)
		}
	}
	if len(addresses) > 3 {
		fmt.Printf("\n   ... and %d more (see full commands in %s)\n", len(addresses)-3, outputFile)
	}

	fmt.Println("\n📋 Option 2: Deploy Using Tenderly")
	fmt.Println("   1. Go to https://dashboard.tenderly.co/")
	fmt.Println("   2. Navigate to Simulator")
	fmt.Println("   3. For each address:")
	fmt.Println("      - From: Your deployer address")
	fmt.Printf("      - To: %s (Factory)\n", FactoryAddress)
	fmt.Println("      - Input Data: Use 'factoryData' from JSON")
	fmt.Println("      - Execute transaction")

	fmt.Println("\n📋 Option 3: Use the deployment script")
	fmt.Println("   go run cmd/deploy_pool_addresses/main.go \\")
	fmt.Printf("     --input %s \\\n", outputFile)
	fmt.Println("     --private-key $PRIVATE_KEY \\")
	fmt.Println("     --rpc-url $RPC_URL")

	fmt.Println("\n📋 After Deployment:")
	fmt.Println("   1. Run the update script to mark addresses as deployed:")
	fmt.Println("      go run cmd/mark_deployed/main.go \\")
	fmt.Printf("        --input %s \\\n", outputFile)
	fmt.Println("        --tx-hashes 0xabc...,0xdef...")
	fmt.Println("\n   2. Or manually update database:")
	fmt.Println("      UPDATE receive_addresses")
	fmt.Println("      SET is_deployed = true,")
	fmt.Println("          status = 'pool_ready',")
	fmt.Println("          deployment_tx_hash = '0x...',")
	fmt.Println("          deployed_at = NOW()")
	fmt.Println("      WHERE address = '0x...'")

	fmt.Println("\n" + strings.Repeat("=", 80))
}
