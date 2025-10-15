package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/NEDA-LABS/stablenode/config"
	"github.com/NEDA-LABS/stablenode/ent/receiveaddress"
	"github.com/NEDA-LABS/stablenode/storage"
	"math/big"
	"strings"

	"github.com/NEDA-LABS/stablenode/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/opus-domini/fast-shot"
	"github.com/spf13/viper"
)

type PoolAddress struct {
	Address           string `json:"address"`
	Salt              string `json:"salt"`
	OwnerAddress      string `json:"owner_address"`
	InitCode          string `json:"init_code"`
	FactoryAddress    string `json:"factory_address"`
	FactoryData       string `json:"factory_data"`
	NetworkIdentifier string `json:"network_identifier"`
	ChainID           int64  `json:"chain_id"`
	DeployCommand     string `json:"deploy_command"`
}

func main() {
	fmt.Println("=== Light Account v0.7 Pool Deployment Tool ===\n")

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

	// Get configuration from environment
	ownerAddress := viper.GetString("SMART_ACCOUNT_OWNER_ADDRESS")
	networkIdentifier := viper.GetString("NETWORK_IDENTIFIER")
	chainID := viper.GetInt64("CHAIN_ID")
	numAddresses := 3 // Default to 3 addresses

	if ownerAddress == "" {
		log.Fatal("SMART_ACCOUNT_OWNER_ADDRESS not set in .env")
	}
	if networkIdentifier == "" {
		networkIdentifier = "base-sepolia"
		chainID = 84532
		fmt.Printf("Using default network: %s (chain ID: %d)\n", networkIdentifier, chainID)
	}

	// Validate owner address
	if !common.IsHexAddress(ownerAddress) {
		log.Fatalf("Invalid owner address: %s", ownerAddress)
	}

	fmt.Printf("Owner Address: %s\n", ownerAddress)
	fmt.Printf("Network: %s (Chain ID: %d)\n", networkIdentifier, chainID)
	fmt.Printf("Number of addresses to generate: %d\n\n", numAddresses)

	// Light Account Factory v2.0.0 for EntryPoint v0.7
	factoryAddress := "0x0000000000400CdFef5E2714E63d8040b700BC24"
	
	var poolAddresses []PoolAddress

	for i := 0; i < numAddresses; i++ {
		fmt.Printf("Generating address %d/%d...\n", i+1, numAddresses)

		// Generate random salt (32 bytes)
		saltBytes := make([]byte, 32)
		_, err := rand.Read(saltBytes)
		if err != nil {
			log.Fatalf("Failed to generate salt: %v", err)
		}
		saltHex := "0x" + hex.EncodeToString(saltBytes)

		// Compute smart account address using CREATE2
		// Light Account factory uses: createAccount(owner, salt)
		// Function selector: 0x5fbfb9cf
		functionSelector := "5fbfb9cf"
		
		// Encode owner address (32 bytes, left-padded)
		ownerBytes := common.HexToAddress(ownerAddress).Bytes()
		ownerPadded := common.LeftPadBytes(ownerBytes, 32)
		
		// Encode salt (32 bytes)
		saltBytes32, _ := hex.DecodeString(saltHex[2:])
		
		// Factory data = function selector + owner + salt
		factoryData := "0x" + functionSelector + hex.EncodeToString(ownerPadded) + hex.EncodeToString(saltBytes32)
		
		// InitCode = factory address + factory data (without 0x)
		initCode := factoryAddress + factoryData[2:]
		
		// Compute CREATE2 address
		// address = keccak256(0xff ++ factory ++ salt ++ keccak256(initCode))[12:]
		smartAccountAddress := computeCreate2Address(factoryAddress, saltHex, initCode)

		fmt.Printf("  Address: %s\n", smartAccountAddress)
		fmt.Printf("  Salt: %s\n", saltHex)

		poolAddress := PoolAddress{
			Address:           smartAccountAddress,
			Salt:              saltHex,
			OwnerAddress:      ownerAddress,
			InitCode:          initCode,
			FactoryAddress:    factoryAddress,
			FactoryData:       factoryData,
			NetworkIdentifier: networkIdentifier,
			ChainID:           chainID,
			DeployCommand:     fmt.Sprintf(`cast send %s "%s" --rpc-url %s --private-key $PRIVATE_KEY`, factoryAddress, factoryData, networkIdentifier),
		}

		poolAddresses = append(poolAddresses, poolAddress)

		// Save to database
		_, err = storage.Client.ReceiveAddress.
			Create().
			SetAddress(smartAccountAddress).
			SetSalt(saltBytes). // Store raw bytes for later use
			SetStatus(receiveaddress.StatusUnused). // Will be set to pool_ready after deployment
			SetIsDeployed(false).
			SetNetworkIdentifier(networkIdentifier).
			SetChainID(chainID).
			SetTimesUsed(0).
			Save(ctx)

		if err != nil {
			log.Printf("Warning: Failed to save address %s to database: %v", smartAccountAddress, err)
		} else {
			fmt.Printf("  ✓ Saved to database\n")
		}
		fmt.Println()
	}

	// Save to JSON file
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("pool_%s_%s.json", networkIdentifier, timestamp)
	filepath := fmt.Sprintf("pool_management/%s", filename)

	jsonData, err := json.MarshalIndent(poolAddresses, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal JSON: %v", err)
	}

	err = os.WriteFile(filepath, jsonData, 0644)
	if err != nil {
		log.Fatalf("Failed to write JSON file: %v", err)
	}

	fmt.Printf("✓ Pool configuration saved to: %s\n\n", filepath)

	// Print deployment instructions
	fmt.Println("=== Deployment Instructions ===")
	fmt.Println("1. Deploy the smart accounts using the commands in the JSON file")
	fmt.Println("2. After deployment, mark them as deployed:")
	fmt.Printf("   go run pool_management/cmd/mark_deployed/main.go %s\n\n", filepath)
	
	fmt.Println("=== Quick Deploy All (Base Sepolia) ===")
	fmt.Println("for addr in", poolAddresses[0].Address, poolAddresses[1].Address, poolAddresses[2].Address, "; do")
	fmt.Printf("  cast send %s \"%s\" --rpc-url base-sepolia --private-key $PRIVATE_KEY\n", factoryAddress, poolAddresses[0].FactoryData)
	fmt.Println("  sleep 2")
	fmt.Println("done")
}

// computeCreate2Address calculates the CREATE2 address by calling the factory contract
func computeCreate2Address(factory, salt, initCode string) string {
	// Call the factory's getAddress(owner, salt) function via RPC
	// Function selector for getAddress(address,uint256): 0x8cb84e18
	
	// Extract owner and salt from factory data
	factoryDataHex := initCode[len(factory):]
	
	// Parse owner from factory data (after function selector)
	ownerHex := "0x" + strings.TrimLeft(factoryDataHex[8:72], "0")
	if ownerHex == "0x" {
		ownerHex = "0x0"
	}
	
	// Parse salt
	saltHex := "0x" + strings.TrimLeft(factoryDataHex[72:136], "0")
	if saltHex == "0x" {
		saltHex = "0x0"
	}
	
	// Build RPC call data for getAddress(owner, salt)
	// Encode: function selector + padded owner + padded salt
	ownerBytes := common.HexToAddress(ownerHex).Bytes()
	ownerPadded := common.LeftPadBytes(ownerBytes, 32)
	
	saltBigInt := new(big.Int)
	saltBigInt.SetString(strings.TrimPrefix(saltHex, "0x"), 16)
	saltBytes := saltBigInt.Bytes()
	saltPadded := common.LeftPadBytes(saltBytes, 32)
	
	callData := "0x8cb84e18" + hex.EncodeToString(ownerPadded) + hex.EncodeToString(saltPadded)
	
	// Make RPC call
	rpcURL := viper.GetString("BASE_SEPOLIA_RPC_URL")
	if rpcURL == "" {
		apiKey := viper.GetString("ALCHEMY_API_KEY")
		if apiKey == "" {
			log.Printf("Warning: No RPC URL configured, cannot compute address")
			return "0x0000000000000000000000000000000000000000"
		}
		rpcURL = "https://base-sepolia.g.alchemy.com/v2/" + apiKey
	}
	
	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "eth_call",
		"params": []interface{}{
			map[string]string{
				"to":   factory,
				"data": callData,
			},
			"latest",
		},
		"id": 1,
	}
	
	res, err := fastshot.NewClient(rpcURL).
		Config().SetTimeout(10 * time.Second).
		Header().AddAll(map[string]string{
			"Accept":       "application/json",
			"Content-Type": "application/json",
		}).Build().POST("").
		Body().AsJSON(payload).Send()
	
	if err != nil {
		log.Printf("Warning: Failed to call factory.getAddress: %v", err)
		return "0x0000000000000000000000000000000000000000"
	}
	
	result, err := utils.ParseJSONResponse(res.RawResponse)
	if err != nil {
		log.Printf("Warning: Failed to parse RPC response: %v", err)
		return "0x0000000000000000000000000000000000000000"
	}
	
	// Parse result
	if result["error"] != nil {
		log.Printf("Warning: RPC error: %v", result["error"])
		return "0x0000000000000000000000000000000000000000"
	}
	
	addressHex, ok := result["result"].(string)
	if !ok || addressHex == "" {
		log.Printf("Warning: Invalid response from factory.getAddress")
		return "0x0000000000000000000000000000000000000000"
	}
	
	// Convert from padded hex to address
	address := common.HexToAddress(addressHex)
	return address.Hex()
}
