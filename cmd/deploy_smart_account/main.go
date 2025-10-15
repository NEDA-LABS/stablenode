package main

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/spf13/viper"
)

func main() {
	fmt.Println("=== Alchemy Smart Account Deployment ===")
	fmt.Println()

	// Load configuration
	viper.SetConfigName(".env")
	viper.SetConfigType("env")
	viper.AddConfigPath(".")
	viper.AddConfigPath("../..")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("Error reading config: %v", err)
	}

	// Get configuration
	ownerAddress := viper.GetString("SMART_ACCOUNT_OWNER_ADDRESS")
	ownerPrivateKey := viper.GetString("SMART_ACCOUNT_OWNER_PRIVATE_KEY")
	alchemyAPIKey := viper.GetString("ALCHEMY_API_KEY")
	rpcURL := fmt.Sprintf("https://base-sepolia.g.alchemy.com/v2/%s", alchemyAPIKey)

	if ownerAddress == "" || ownerPrivateKey == "" {
		log.Fatal("SMART_ACCOUNT_OWNER_ADDRESS and SMART_ACCOUNT_OWNER_PRIVATE_KEY must be set in .env")
	}

	fmt.Printf("Owner Address: %s\n", ownerAddress)
	fmt.Printf("RPC URL: %s\n", rpcURL)
	fmt.Println()

	// Connect to Base Sepolia
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		log.Fatalf("Failed to connect to Base Sepolia: %v", err)
	}
	defer client.Close()

	// Verify connection
	chainID, err := client.ChainID(context.Background())
	if err != nil {
		log.Fatalf("Failed to get chain ID: %v", err)
	}
	fmt.Printf("Connected to Chain ID: %s (Base Sepolia)\n", chainID.String())

	// Parse private key
	privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(ownerPrivateKey, "0x"))
	if err != nil {
		log.Fatalf("Invalid private key: %v", err)
	}

	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		log.Fatal("Error casting public key to ECDSA")
	}

	fromAddress := crypto.PubkeyToAddress(*publicKeyECDSA)
	fmt.Printf("Derived Address: %s\n", fromAddress.Hex())

	if strings.ToLower(fromAddress.Hex()) != strings.ToLower(ownerAddress) {
		log.Fatalf("Private key doesn't match owner address!\nExpected: %s\nGot: %s", ownerAddress, fromAddress.Hex())
	}

	// Check balance
	balance, err := client.BalanceAt(context.Background(), fromAddress, nil)
	if err != nil {
		log.Fatalf("Failed to get balance: %v", err)
	}
	fmt.Printf("Owner Balance: %s ETH\n", weiToEther(balance))
	fmt.Println()

	if balance.Cmp(big.NewInt(0)) == 0 {
		log.Fatal("❌ Owner wallet has no ETH! Please fund it from: https://www.alchemy.com/faucets/base-sepolia")
	}

	// Smart Account Factory details
	factoryAddress := common.HexToAddress("0x0000000000400CdFef5E2714E63d8040b700BC24")
	salt := big.NewInt(0) // First account

	fmt.Println("=== Deployment Configuration ===")
	fmt.Printf("Factory: %s (Light Account v2.0.0)\n", factoryAddress.Hex())
	fmt.Printf("Owner: %s\n", ownerAddress)
	fmt.Printf("Salt: %s\n", salt.String())
	fmt.Println()

	// Compute expected smart account address
	expectedAddress := computeSmartAccountAddress(factoryAddress, common.HexToAddress(ownerAddress), salt)
	fmt.Printf("🎯 Expected Smart Account Address: %s\n", expectedAddress.Hex())
	fmt.Println()

	// Check if account already exists
	code, err := client.CodeAt(context.Background(), expectedAddress, nil)
	if err != nil {
		log.Fatalf("Failed to check account code: %v", err)
	}

	if len(code) > 0 {
		fmt.Println("✅ Smart account is already deployed!")
		fmt.Printf("   Address: %s\n", expectedAddress.Hex())
		fmt.Printf("   Code Size: %d bytes\n", len(code))
		fmt.Printf("   Explorer: https://sepolia.basescan.org/address/%s\n", expectedAddress.Hex())
		return
	}

	fmt.Println("📝 Smart account not yet deployed. Deploying now...")
	fmt.Println()

	// Deploy the account by calling factory.createAccount(owner, salt)
	err = deploySmartAccount(client, privateKey, factoryAddress, common.HexToAddress(ownerAddress), salt, chainID)
	if err != nil {
		log.Fatalf("Deployment failed: %v", err)
	}

	fmt.Println()
	fmt.Println("=== Deployment Successful! ===")
	fmt.Printf("Smart Account Address: %s\n", expectedAddress.Hex())
	fmt.Printf("Explorer: https://sepolia.basescan.org/address/%s\n", expectedAddress.Hex())
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("1. Verify the contract on Base Sepolia explorer")
	fmt.Println("2. Fund the smart account with testnet ETH")
	fmt.Println("3. Test sending a transaction")
}

func deploySmartAccount(client *ethclient.Client, privateKey *ecdsa.PrivateKey, factoryAddress, owner common.Address, salt *big.Int, chainID *big.Int) error {
	ctx := context.Background()

	// Get nonce
	fromAddress := crypto.PubkeyToAddress(*privateKey.Public().(*ecdsa.PublicKey))
	nonce, err := client.PendingNonceAt(ctx, fromAddress)
	if err != nil {
		return fmt.Errorf("failed to get nonce: %w", err)
	}

	// Get gas price
	gasPrice, err := client.SuggestGasPrice(ctx)
	if err != nil {
		return fmt.Errorf("failed to get gas price: %w", err)
	}

	// Encode function call: createAccount(address owner, uint256 salt)
	// Function selector: 0x5fbfb9cf
	functionSelector := []byte{0x5f, 0xbf, 0xb9, 0xcf}
	
	// Encode parameters
	ownerBytes := common.LeftPadBytes(owner.Bytes(), 32)
	saltBytes := common.LeftPadBytes(salt.Bytes(), 32)
	
	data := append(functionSelector, ownerBytes...)
	data = append(data, saltBytes...)

	// Estimate gas
	gasLimit, err := client.EstimateGas(ctx, ethereum.CallMsg{
		From:     fromAddress,
		To:       &factoryAddress,
		GasPrice: gasPrice,
		Value:    big.NewInt(0),
		Data:     data,
	})
	if err != nil {
		return fmt.Errorf("failed to estimate gas: %w", err)
	}

	// Add 20% buffer to gas limit
	gasLimit = gasLimit * 120 / 100

	fmt.Printf("Transaction Details:\n")
	fmt.Printf("  From: %s\n", fromAddress.Hex())
	fmt.Printf("  To: %s (Factory)\n", factoryAddress.Hex())
	fmt.Printf("  Nonce: %d\n", nonce)
	fmt.Printf("  Gas Limit: %d\n", gasLimit)
	fmt.Printf("  Gas Price: %s Gwei\n", weiToGwei(gasPrice))
	fmt.Printf("  Estimated Cost: %s ETH\n", weiToEther(new(big.Int).Mul(gasPrice, big.NewInt(int64(gasLimit)))))
	fmt.Println()

	// Create transaction
	tx := types.NewTransaction(
		nonce,
		factoryAddress,
		big.NewInt(0),
		gasLimit,
		gasPrice,
		data,
	)

	// Sign transaction
	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), privateKey)
	if err != nil {
		return fmt.Errorf("failed to sign transaction: %w", err)
	}

	// Send transaction
	fmt.Println("📤 Sending deployment transaction...")
	err = client.SendTransaction(ctx, signedTx)
	if err != nil {
		return fmt.Errorf("failed to send transaction: %w", err)
	}

	txHash := signedTx.Hash().Hex()
	fmt.Printf("✅ Transaction sent: %s\n", txHash)
	fmt.Printf("   Explorer: https://sepolia.basescan.org/tx/%s\n", txHash)
	fmt.Println()

	// Wait for confirmation
	fmt.Println("⏳ Waiting for confirmation...")
	receipt, err := waitForReceipt(client, signedTx.Hash(), 2*time.Minute)
	if err != nil {
		return fmt.Errorf("failed to get receipt: %w", err)
	}

	if receipt.Status == 1 {
		fmt.Printf("✅ Transaction confirmed in block %d\n", receipt.BlockNumber.Uint64())
		fmt.Printf("   Gas Used: %d\n", receipt.GasUsed)
	} else {
		return fmt.Errorf("transaction failed (status: %d)", receipt.Status)
	}

	return nil
}

func waitForReceipt(client *ethclient.Client, txHash common.Hash, timeout time.Duration) (*types.Receipt, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("timeout waiting for transaction")
		case <-ticker.C:
			receipt, err := client.TransactionReceipt(ctx, txHash)
			if err == nil {
				return receipt, nil
			}
		}
	}
}

func computeSmartAccountAddress(factory, owner common.Address, salt *big.Int) common.Address {
	// Call factory.getAddress(owner, salt) via RPC to get the correct address
	// Function selector for getAddress(address,uint256): 0x8cb84e18
	
	alchemyAPIKey := viper.GetString("ALCHEMY_API_KEY")
	rpcURL := fmt.Sprintf("https://base-sepolia.g.alchemy.com/v2/%s", alchemyAPIKey)
	
	// Encode call data
	ownerPadded := common.LeftPadBytes(owner.Bytes(), 32)
	saltPadded := common.LeftPadBytes(salt.Bytes(), 32)
	callData := "0x8cb84e18" + hex.EncodeToString(ownerPadded) + hex.EncodeToString(saltPadded)
	
	// Make RPC call
	payload := fmt.Sprintf(`{"jsonrpc":"2.0","method":"eth_call","params":[{"to":"%s","data":"%s"},"latest"],"id":1}`, 
		factory.Hex(), callData)
	
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(rpcURL, "application/json", strings.NewReader(payload))
	if err != nil {
		log.Printf("Warning: Failed to call factory.getAddress: %v", err)
		return common.Address{}
	}
	defer resp.Body.Close()
	
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("Warning: Failed to parse RPC response: %v", err)
		return common.Address{}
	}
	
	if result["error"] != nil {
		log.Printf("Warning: RPC error: %v", result["error"])
		return common.Address{}
	}
	
	addressHex, ok := result["result"].(string)
	if !ok || addressHex == "" {
		log.Printf("Warning: Invalid response from factory.getAddress")
		return common.Address{}
	}
	
	return common.HexToAddress(addressHex)
}

func weiToEther(wei *big.Int) string {
	ether := new(big.Float).SetInt(wei)
	ether = ether.Quo(ether, big.NewFloat(1e18))
	return ether.Text('f', 6)
}

func weiToGwei(wei *big.Int) string {
	gwei := new(big.Float).SetInt(wei)
	gwei = gwei.Quo(gwei, big.NewFloat(1e9))
	return gwei.Text('f', 2)
}
