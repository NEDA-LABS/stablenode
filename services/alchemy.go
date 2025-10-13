package services

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	ethereumtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	fastshot "github.com/opus-domini/fast-shot"
	"github.com/NEDA-LABS/stablenode/config"
	"github.com/NEDA-LABS/stablenode/ent"
	"github.com/NEDA-LABS/stablenode/ent/network"
	"github.com/NEDA-LABS/stablenode/ent/receiveaddress"
	"github.com/NEDA-LABS/stablenode/storage"
	stablenodtypes "github.com/NEDA-LABS/stablenode/types"
	"github.com/NEDA-LABS/stablenode/utils"
	cryptoUtils "github.com/NEDA-LABS/stablenode/utils/crypto"
	"github.com/NEDA-LABS/stablenode/utils/logger"
	"github.com/spf13/viper"
)

// AlchemyService provides functionality for interacting with Alchemy APIs
// This is an alternative to EngineService for EVM-only operations
type AlchemyService struct {
	config *config.AlchemyConfiguration
}

// NewAlchemyService creates a new instance of AlchemyService
func NewAlchemyService() *AlchemyService {
	return &AlchemyService{
		config: config.AlchemyConfig(),
	}
}

// CreateSmartAccount creates a new ERC-4337 smart contract account using Alchemy
// Note: With Alchemy, we don't need to "create" the account via API - we compute it deterministically
// The account gets deployed automatically when the first transaction is sent to it
// Returns: address, encryptedSalt, error
func (s *AlchemyService) CreateSmartAccount(ctx context.Context, chainID int64, ownerAddress string) (string, []byte, error) {
	// Generate a unique salt from timestamp and random data
	// This ensures each receive address is unique
	salt := s.generateUniqueSalt()
	
	// Compute the smart account address deterministically using CREATE2
	smartAccountAddress := s.computeSmartAccountAddressWithSalt(ownerAddress, chainID, salt)
	
	// Encrypt the salt for storage
	// We need to store the salt to be able to compute initCode later
	saltBytes := salt[:] // Convert [32]byte to []byte
	encryptedSalt, err := cryptoUtils.EncryptPlain(saltBytes)
	if err != nil {
		return "", nil, fmt.Errorf("failed to encrypt salt: %w", err)
	}
	
	logger.WithFields(logger.Fields{
		"Owner":        ownerAddress,
		"SmartAccount": smartAccountAddress,
		"Salt":         fmt.Sprintf("0x%x", salt),
		"EncryptedSaltLength": len(encryptedSalt),
		"Method":       "Deterministic CREATE2",
	}).Infof("Generated smart account address via Alchemy with encrypted salt")

	return smartAccountAddress, encryptedSalt, nil
}

// getSmartAccountInitCode generates the initCode for smart account deployment
// Takes the salt as a hex string (without 0x prefix)
func (s *AlchemyService) getSmartAccountInitCode(ownerAddress string, saltHex string) string {
	// Use Alchemy's Light Account Factory v2.0.0 (latest)
	// Factory address: 0x0000000000400CdFef5E2714E63d8040b700BC24
	factoryAddress := "0000000000400CdFef5E2714E63d8040b700BC24"
	
	// Function selector for createAccount(address owner, uint256 salt)
	functionSelector := "5fbfb9cf"
	
	// Encode owner address (32 bytes padded)
	ownerPadded := "000000000000000000000000" + strings.TrimPrefix(ownerAddress, "0x")
	
	// Use the provided salt (32 bytes)
	salt := strings.TrimPrefix(saltHex, "0x")
	// Ensure salt is 64 characters (32 bytes in hex)
	if len(salt) < 64 {
		salt = strings.Repeat("0", 64-len(salt)) + salt // Left pad with zeros
	}
	
	// Combine: 0x + factory + selector + owner + salt
	initCode := "0x" + factoryAddress + functionSelector + ownerPadded + salt
	
	return initCode
}

// computeSmartAccountAddress computes the deterministic smart account address using CREATE2
func (s *AlchemyService) computeSmartAccountAddress(ownerAddress string, chainID int64) string {
	// Alchemy Light Account Factory v2.0.0
	factoryAddress := common.HexToAddress("0x0000000000400CdFef5E2714E63d8040b700BC24")
	implementationAddress := common.HexToAddress("0x8E8e658E22B12ada97B402fF0b044D6A325013C7")
	
	// Salt is typically 0 for the first account
	salt := [32]byte{} // 32 bytes of zeros
	
	// Owner address (cleaned)
	owner := common.HexToAddress(ownerAddress)
	
	// Compute the init code hash for the proxy
	// The Light Account uses a minimal proxy pattern (EIP-1167)
	// Init code for minimal proxy: 0x3d602d80600a3d3981f3363d3d373d3d3d363d73 + implementation + 0x5af43d82803e903d91602b57fd5bf3
	
	// Build the minimal proxy bytecode
	proxyPrefix := common.Hex2Bytes("3d602d80600a3d3981f3363d3d373d3d3d363d73")
	proxySuffix := common.Hex2Bytes("5af43d82803e903d91602b57fd5bf3")
	
	// Combine: prefix + implementation address + suffix
	initCode := append(proxyPrefix, implementationAddress.Bytes()...)
	initCode = append(initCode, proxySuffix...)
	
	// Add the owner address as constructor parameter (ABI encoded)
	// For Light Account, the constructor takes the owner address
	ownerParam := common.LeftPadBytes(owner.Bytes(), 32)
	initCode = append(initCode, ownerParam...)
	
	// Hash the init code
	initCodeHash := crypto.Keccak256(initCode)
	
	// CREATE2 formula: keccak256(0xff ++ factory ++ salt ++ keccak256(initCode))
	data := make([]byte, 0, 1+20+32+32)
	data = append(data, 0xff)
	data = append(data, factoryAddress.Bytes()...)
	data = append(data, salt[:]...)
	data = append(data, initCodeHash...)
	
	// Hash and take last 20 bytes as address
	hash := crypto.Keccak256(data)
	address := common.BytesToAddress(hash[12:])
	
	return address.Hex()
}

// generateUniqueSalt generates a unique salt for CREATE2 deployment
func (s *AlchemyService) generateUniqueSalt() [32]byte {
	// Use current timestamp + random bytes to ensure uniqueness
	timestamp := time.Now().UnixNano()
	
	// Create a hash of timestamp + random data
	hash := crypto.Keccak256Hash(
		[]byte(fmt.Sprintf("%d", timestamp)),
		crypto.Keccak256([]byte(fmt.Sprintf("%d", time.Now().UnixMicro()))),
	)
	
	return hash
}

// computeSmartAccountAddressWithSalt computes the deterministic smart account address using CREATE2 with a custom salt
func (s *AlchemyService) computeSmartAccountAddressWithSalt(ownerAddress string, chainID int64, salt [32]byte) string {
	// Alchemy Light Account Factory v2.0.0
	factoryAddress := common.HexToAddress("0x0000000000400CdFef5E2714E63d8040b700BC24")
	
	// Instead of computing ourselves, call the factory's getAddress function
	// This ensures we get the exact same address that will be deployed
	ctx := context.Background()
	
	// Get network RPC endpoint
	network, err := storage.Client.Network.
		Query().
		Where(network.ChainIDEQ(chainID)).
		Only(ctx)
	if err != nil {
		logger.Errorf("Failed to get network for address computation: %v", err)
		return ""
	}
	
	// Encode the getAddress(address,uint256) call
	// Function selector: 0x8cb84e18
	functionSelector := "8cb84e18"
	ownerPadded := "000000000000000000000000" + strings.TrimPrefix(ownerAddress, "0x")
	saltHex := fmt.Sprintf("%064x", salt)
	
	callData := "0x" + functionSelector + ownerPadded + saltHex
	
	// Call the factory contract
	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "eth_call",
		"params": []interface{}{
			map[string]interface{}{
				"to":   factoryAddress.Hex(),
				"data": callData,
			},
			"latest",
		},
		"id": 1,
	}
	
	url := fmt.Sprintf("%s/%s", network.RPCEndpoint, s.config.APIKey)
	res, err := fastshot.NewClient(url).
		Config().SetTimeout(10 * time.Second).
		Header().AddAll(map[string]string{
			"Accept":       "application/json",
			"Content-Type": "application/json",
		}).Build().POST("").
		Body().AsJSON(payload).Send()
	
	if err != nil {
		logger.Errorf("Failed to call factory getAddress: %v", err)
		return ""
	}
	
	data, err := utils.ParseJSONResponse(res.RawResponse)
	if err != nil {
		logger.Errorf("Failed to parse getAddress response: %v", err)
		return ""
	}
	
	if data["error"] != nil {
		logger.Errorf("Factory getAddress returned error: %v", data["error"])
		return ""
	}
	
	// Result is the address (32 bytes, last 20 bytes are the address)
	resultHex := data["result"].(string)
	address := "0x" + resultHex[len(resultHex)-40:]
	
	logger.WithFields(logger.Fields{
		"Owner":   ownerAddress,
		"Salt":    fmt.Sprintf("0x%x", salt),
		"Address": address,
	}).Info("Computed smart account address via factory getAddress")
	
	return address
}

// packUserOperationV07 converts a UserOperation to v0.7 RPC format for EntryPoint v0.7
func (s *AlchemyService) packUserOperationV07(userOp map[string]interface{}) map[string]interface{} {
	logger.WithFields(logger.Fields{
		"UnpackedUserOp": userOp,
	}).Info("Converting UserOperation to v0.7 RPC format")
	
	// For v0.7 RPC format, we need to split initCode into factory + factoryData
	// and paymasterAndData into paymaster + paymasterVerificationGasLimit + paymasterPostOpGasLimit + paymasterData
	
	// Get initCode with nil check
	initCode := "0x"
	if userOp["initCode"] != nil {
		initCode = userOp["initCode"].(string)
	}
	
	var factory, factoryData string
	if initCode == "0x" || len(initCode) <= 2 {
		factory = "0x0000000000000000000000000000000000000000"
		factoryData = "0x"
	} else {
		// initCode = factory (20 bytes) + factoryData
		factory = "0x" + initCode[2:42] // First 20 bytes (40 hex chars)
		factoryData = "0x" + initCode[42:] // Rest is factoryData
	}
	
	// Get paymasterAndData with nil check
	paymasterAndData := "0x"
	if userOp["paymasterAndData"] != nil {
		paymasterAndData = userOp["paymasterAndData"].(string)
	}
	
	// Build v0.7 RPC format UserOp
	v07UserOp := map[string]interface{}{
		"sender":                 userOp["sender"],
		"nonce":                  userOp["nonce"],
		"factory":                factory,
		"factoryData":            factoryData,
		"callData":               userOp["callData"],
		"callGasLimit":           userOp["callGasLimit"],
		"verificationGasLimit":   userOp["verificationGasLimit"],
		"preVerificationGas":     userOp["preVerificationGas"],
		"maxFeePerGas":           userOp["maxFeePerGas"],
		"maxPriorityFeePerGas":   userOp["maxPriorityFeePerGas"],
		"signature":              userOp["signature"],
	}
	
	// Add paymaster fields only if paymaster is being used
	if paymasterAndData != "" && paymasterAndData != "0x" && len(paymasterAndData) > 2 {
		// paymasterAndData = paymaster (20 bytes) + paymasterVerificationGasLimit (16 bytes) + paymasterPostOpGasLimit (16 bytes) + paymasterData
		paymaster := "0x" + paymasterAndData[2:42] // First 20 bytes
		paymasterVerificationGasLimit := "0x" + paymasterAndData[42:74] // Next 16 bytes (32 hex chars)
		paymasterPostOpGasLimit := "0x" + paymasterAndData[74:106] // Next 16 bytes
		paymasterData := "0x" + paymasterAndData[106:] // Rest is paymasterData
		
		v07UserOp["paymaster"] = paymaster
		v07UserOp["paymasterVerificationGasLimit"] = paymasterVerificationGasLimit
		v07UserOp["paymasterPostOpGasLimit"] = paymasterPostOpGasLimit
		v07UserOp["paymasterData"] = paymasterData
	}
	
	logger.WithFields(logger.Fields{
		"V07UserOp": v07UserOp,
		"Sender":    v07UserOp["sender"],
		"Signature": v07UserOp["signature"],
	}).Info("Converted UserOperation to v0.7 RPC format")
	
	return v07UserOp
}

// SendUserOperation sends a user operation (transaction) via Alchemy's bundler
func (s *AlchemyService) SendUserOperation(ctx context.Context, chainID int64, userOp map[string]interface{}) (string, error) {
	// Get network to use chain-specific RPC endpoint
	network, err := storage.Client.Network.
		Query().
		Where(network.ChainIDEQ(chainID)).
		Only(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get network for chain %d: %w", chainID, err)
	}
	
	// Convert to PackedUserOperation format for EntryPoint v0.7
	packedUserOp := s.packUserOperationV07(userOp)
	
	// Use the network's RPC endpoint and append API key
	url := fmt.Sprintf("%s/%s", network.RPCEndpoint, s.config.APIKey)
	
	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "eth_sendUserOperation",
		"params": []interface{}{
			packedUserOp,
			"0x0000000071727De22E5E9d8baF0edAc6f37da032", // EntryPoint v0.7
		},
		"id": 1,
	}

	// Marshal payload to see exact request
	payloadJSON, _ := json.Marshal(payload)
	
	logger.WithFields(logger.Fields{
		"ChainID":       chainID,
		"URL":           url,
		"PackedUserOp":  packedUserOp,
		"PayloadJSON":   string(payloadJSON),
	}).Info("Sending UserOperation to Alchemy")

	res, err := fastshot.NewClient(url).
		Config().SetTimeout(30 * time.Second).
		Header().AddAll(map[string]string{
			"Accept":       "application/json",
			"Content-Type": "application/json",
		}).Build().POST("").
		Body().AsJSON(payload).Send()
	
	if err != nil {
		return "", fmt.Errorf("failed to send user operation: %w", err)
	}

	// Log raw response first
	bodyBytes, _ := io.ReadAll(res.RawResponse.Body)
	res.RawResponse.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	
	logger.WithFields(logger.Fields{
		"StatusCode":   res.StatusCode(),
		"ResponseBody": string(bodyBytes),
		"Headers":      res.RawResponse.Header,
	}).Info("Received eth_sendUserOperation response")

	data, err := utils.ParseJSONResponse(res.RawResponse)
	if err != nil {
		logger.WithFields(logger.Fields{
			"StatusCode": res.StatusCode(),
			"Body":       string(bodyBytes),
		}).Error("Failed to parse Alchemy response")
		return "", fmt.Errorf("failed to parse JSON response (status %d): %s", res.StatusCode(), string(bodyBytes))
	}

	if data["error"] != nil {
		errorData, _ := json.Marshal(data["error"])
		logger.WithFields(logger.Fields{
			"Error": string(errorData),
		}).Error("Alchemy returned error for UserOperation")
		return "", fmt.Errorf("user operation failed: %s", string(errorData))
	}

	userOpHash := data["result"].(string)
	return userOpHash, nil
}

// GetUserOperationReceipt gets the receipt for a user operation
func (s *AlchemyService) GetUserOperationReceipt(ctx context.Context, chainID int64, userOpHash string) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/%s", s.config.BaseURL, s.config.APIKey)
	
	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "eth_getUserOperationReceipt",
		"params":  []interface{}{userOpHash},
		"id":      1,
	}

	res, err := fastshot.NewClient(url).
		Config().SetTimeout(30 * time.Second).
		Header().AddAll(map[string]string{
			"Accept":       "application/json",
			"Content-Type": "application/json",
		}).Build().POST("").
		Body().AsJSON(payload).Send()
	
	if err != nil {
		return nil, fmt.Errorf("failed to get user operation receipt: %w", err)
	}

	data, err := utils.ParseJSONResponse(res.RawResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	if data["result"] == nil {
		return nil, fmt.Errorf("user operation not found or not mined yet")
	}

	return data["result"].(map[string]interface{}), nil
}

// WaitForUserOperationMined waits for a user operation to be mined
func (s *AlchemyService) WaitForUserOperationMined(ctx context.Context, chainID int64, userOpHash string, timeout time.Duration) (map[string]interface{}, error) {
	start := time.Now()
	for {
		receipt, err := s.GetUserOperationReceipt(ctx, chainID, userOpHash)
		if err == nil && receipt != nil {
			return receipt, nil
		}

		elapsed := time.Since(start)
		if elapsed >= timeout {
			return nil, fmt.Errorf("user operation mining timeout after %v", timeout)
		}

		time.Sleep(2 * time.Second)
	}
}

// GetLatestBlock fetches the latest block number for a given chain ID using Alchemy RPC
func (s *AlchemyService) GetLatestBlock(ctx context.Context, chainID int64) (int64, error) {
	url := fmt.Sprintf("%s/%s", s.config.BaseURL, s.config.APIKey)
	
	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "eth_blockNumber",
		"params":  []interface{}{},
		"id":      1,
	}

	res, err := fastshot.NewClient(url).
		Config().SetTimeout(30 * time.Second).
		Header().AddAll(map[string]string{
			"Accept":       "application/json",
			"Content-Type": "application/json",
		}).Build().POST("").
		Body().AsJSON(payload).Send()
	
	if err != nil {
		return 0, fmt.Errorf("failed to get latest block: %w", err)
	}

	data, err := utils.ParseJSONResponse(res.RawResponse)
	if err != nil {
		return 0, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	blockNumberHex := data["result"].(string)
	blockNumber, err := strconv.ParseInt(strings.TrimPrefix(blockNumberHex, "0x"), 16, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse block number: %w", err)
	}

	return blockNumber, nil
}

// GetContractEvents fetches contract events using Alchemy's enhanced APIs
func (s *AlchemyService) GetContractEvents(ctx context.Context, chainID int64, contractAddress string, fromBlock, toBlock int64, topics []string) ([]interface{}, error) {
	url := fmt.Sprintf("%s/%s", s.config.BaseURL, s.config.APIKey)
	
	// Convert block numbers to hex
	fromBlockHex := fmt.Sprintf("0x%x", fromBlock)
	toBlockHex := fmt.Sprintf("0x%x", toBlock)
	
	// Convert topics to proper format
	var topicsArray []interface{}
	for _, topic := range topics {
		if topic != "" {
			topicsArray = append(topicsArray, topic)
		}
	}

	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "eth_getLogs",
		"params": []interface{}{
			map[string]interface{}{
				"address":   contractAddress,
				"fromBlock": fromBlockHex,
				"toBlock":   toBlockHex,
				"topics":    topicsArray,
			},
		},
		"id": 1,
	}

	res, err := fastshot.NewClient(url).
		Config().SetTimeout(60 * time.Second).
		Header().AddAll(map[string]string{
			"Accept":       "application/json",
			"Content-Type": "application/json",
		}).Build().POST("").
		Body().AsJSON(payload).Send()
	
	if err != nil {
		return nil, fmt.Errorf("failed to get contract events: %w", err)
	}

	data, err := utils.ParseJSONResponse(res.RawResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	if data["result"] == nil {
		return nil, fmt.Errorf("no events found")
	}

	logs := data["result"].([]interface{})
	
	// Convert to the same format as your existing event processing
	var events []interface{}
	for _, log := range logs {
		logMap := log.(map[string]interface{})
		
		// Convert hex block number to decimal
		blockNumberHex := logMap["blockNumber"].(string)
		blockNumber, _ := strconv.ParseInt(strings.TrimPrefix(blockNumberHex, "0x"), 16, 64)
		
		event := map[string]interface{}{
			"block_number":     float64(blockNumber),
			"transaction_hash": logMap["transactionHash"].(string),
			"log_index":        logMap["logIndex"],
			"address":          logMap["address"].(string),
			"topics":           logMap["topics"],
			"data":             logMap["data"].(string),
			"decoded": map[string]interface{}{
				"indexed_params":     make(map[string]interface{}),
				"non_indexed_params": make(map[string]interface{}),
			},
		}
		events = append(events, event)
	}

	// Process events to decode them (reuse existing utility)
	if len(events) > 0 {
		err = utils.ProcessRPCEventsBySignature(events)
		if err != nil {
			return nil, fmt.Errorf("failed to process events: %w", err)
		}
	}

	return events, nil
}

// EstimateGas estimates gas for a transaction using Alchemy
func (s *AlchemyService) EstimateGas(ctx context.Context, chainID int64, from, to, data string, value *big.Int) (uint64, error) {
	url := fmt.Sprintf("%s/%s", s.config.BaseURL, s.config.APIKey)
	
	valueHex := "0x0"
	if value != nil && value.Cmp(big.NewInt(0)) > 0 {
		valueHex = "0x" + value.Text(16)
	}

	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "eth_estimateGas",
		"params": []interface{}{
			map[string]interface{}{
				"from":  from,
				"to":    to,
				"data":  data,
				"value": valueHex,
			},
		},
		"id": 1,
	}

	res, err := fastshot.NewClient(url).
		Config().SetTimeout(30 * time.Second).
		Header().AddAll(map[string]string{
			"Accept":       "application/json",
			"Content-Type": "application/json",
		}).Build().POST("").
		Body().AsJSON(payload).Send()
	
	if err != nil {
		return 0, fmt.Errorf("failed to estimate gas: %w", err)
	}

	data_resp, err := utils.ParseJSONResponse(res.RawResponse)
	if err != nil {
		return 0, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	gasHex := data_resp["result"].(string)
	gas, err := strconv.ParseUint(strings.TrimPrefix(gasHex, "0x"), 16, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse gas estimate: %w", err)
	}

	return gas, nil
}

// GetSupportedChains returns a list of EVM chains supported by this service
func (s *AlchemyService) GetSupportedChains(ctx context.Context) ([]*ent.Network, error) {
	// Fetch only EVM networks (exclude Tron)
	networks, err := storage.Client.Network.
		Query().
		Where(network.Not(network.IdentifierHasPrefix("tron"))).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch EVM networks: %w", err)
	}

	return networks, nil
}

// SendTransactionBatch sends a batch of transactions
// Automatically detects if the address is an EOA or smart account and uses the appropriate method
func (s *AlchemyService) SendTransactionBatch(ctx context.Context, chainID int64, address string, txPayload []map[string]interface{}) (string, error) {
	// Determine if this is a smart account or EOA
	// Check database first - if address has a salt, it's a smart account (even if not deployed yet)
	isSmartAccount := false
	receiveAddr, err := storage.Client.ReceiveAddress.
		Query().
		Where(receiveaddress.AddressEQ(address)).
		Only(ctx)
	
	if err == nil && receiveAddr.Salt != nil && len(receiveAddr.Salt) > 0 {
		// Has salt = smart account
		isSmartAccount = true
		logger.WithFields(logger.Fields{
			"Address": address,
		}).Infof("Address identified as smart account (has salt in database)")
	} else {
		// No salt = check if it's a deployed contract
		isContract, err := s.isAccountDeployed(ctx, chainID, address)
		if err != nil {
			logger.Warnf("Failed to check if address is contract: %v", err)
			isContract = false
		}
		
		if isContract {
			// It's a deployed contract, treat as smart account
			isSmartAccount = true
			logger.WithFields(logger.Fields{
				"Address": address,
			}).Infof("Address identified as smart account (deployed contract)")
		}
	}

	if isSmartAccount {
		// Address is a smart account - use UserOperation
		logger.WithFields(logger.Fields{
			"Address": address,
			"ChainID": chainID,
		}).Infof("Sending transaction via UserOperation (smart account)")
		return s.sendUserOperationBatch(ctx, chainID, address, txPayload)
	}

	// Address is an EOA - use regular transaction
	logger.WithFields(logger.Fields{
		"Address": address,
		"ChainID": chainID,
	}).Infof("Sending transaction via EOA (regular transaction)")
	return s.sendEOATransactionBatch(ctx, chainID, address, txPayload)
}

// deploySmartAccount deploys a smart account by sending a UserOp with only initCode
func (s *AlchemyService) deploySmartAccount(ctx context.Context, chainID int64, smartAccountAddress string) error {
	// Get owner address and salt
	ownerAddress := viper.GetString("SMART_ACCOUNT_OWNER_ADDRESS")
	if ownerAddress == "" {
		return fmt.Errorf("SMART_ACCOUNT_OWNER_ADDRESS not configured")
	}
	
	// Retrieve the salt from database
	receiveAddr, err := storage.Client.ReceiveAddress.
		Query().
		Where(receiveaddress.AddressEQ(strings.ToLower(smartAccountAddress))).
		Only(ctx)
	
	if err != nil {
		return fmt.Errorf("failed to get receive address for salt: %w", err)
	}
	
	if len(receiveAddr.Salt) == 0 {
		return fmt.Errorf("no salt found for smart account %s - cannot generate initCode", smartAccountAddress)
	}
	
	// Decrypt the salt
	saltBytes, err := cryptoUtils.DecryptPlain(receiveAddr.Salt)
	if err != nil {
		return fmt.Errorf("failed to decrypt salt: %w", err)
	}
	
	// Convert salt to hex string
	saltHex := common.Bytes2Hex(saltBytes)
	
	initCode := s.getSmartAccountInitCode(ownerAddress, saltHex)
	
	// Verify that the initCode will deploy to the expected address
	logger.WithFields(logger.Fields{
		"SmartAccount": smartAccountAddress,
		"Owner":        ownerAddress,
		"Salt":         saltHex,
		"InitCode":     initCode[:66] + "...", // Log first 66 chars
	}).Info("Generated initCode for deployment")
	
	// Create a simple UserOp that just deploys the account (no execution)
	userOp := map[string]interface{}{
		"sender":               smartAccountAddress,
		"nonce":                "0x0",
		"initCode":             initCode,
		"callData":             "0x", // No execution, just deployment
		"callGasLimit":         "0x7530", // 30k gas minimum even for empty callData
		"verificationGasLimit": "0x493e0", // 300k gas limit for verification (deployment needs more)
		"preVerificationGas":   "0x10000",  // 65536 gas
		"maxFeePerGas":         "0x59682f00",
		"maxPriorityFeePerGas": "0x59682f00",
		"paymasterAndData":     "0x",
		"signature":            "0x",
	}
	
	// Request paymaster sponsorship for deployment if gas policy is configured
	if s.config.GasPolicyID != "" {
		// Create a UserOp for paymaster request with initial gas estimates
		// Alchemy will refine these estimates and return optimized values
		minimalUserOp := map[string]interface{}{
			"sender":   userOp["sender"],
			"nonce":    userOp["nonce"],
			"initCode": userOp["initCode"],
			"callData": userOp["callData"],
			// Add initial gas estimates (Alchemy will optimize these)
			"callGasLimit":         userOp["callGasLimit"],         // 30k default for deployment
			"verificationGasLimit": userOp["verificationGasLimit"], // 300k for deployment
			"preVerificationGas":   userOp["preVerificationGas"],   // 65536 default
			"maxFeePerGas":         userOp["maxFeePerGas"],         // From gas oracle
			"maxPriorityFeePerGas": userOp["maxPriorityFeePerGas"], // From gas oracle
		}
		
		// Log the UserOp being sent
		minimalJSON, _ := json.Marshal(minimalUserOp)
		logger.WithFields(logger.Fields{
			"SmartAccount": smartAccountAddress,
			"UserOpWithGas": string(minimalJSON),
		}).Info("Sending UserOp to paymaster for deployment")
		
		result, err := s.getPaymasterData(ctx, chainID, minimalUserOp)
		if err != nil {
			logger.WithFields(logger.Fields{
				"Error": err.Error(),
				"SmartAccount": smartAccountAddress,
			}).Warn("Failed to get paymaster data for deployment, will try without sponsorship")
		} else {
			// Apply all gas estimates and paymaster data from the response
			if paymasterAndData, ok := result["paymasterAndData"].(string); ok {
				userOp["paymasterAndData"] = paymasterAndData
			}
			if callGasLimit, ok := result["callGasLimit"].(string); ok {
				userOp["callGasLimit"] = callGasLimit
			}
			if verificationGasLimit, ok := result["verificationGasLimit"].(string); ok {
				userOp["verificationGasLimit"] = verificationGasLimit
			}
			if preVerificationGas, ok := result["preVerificationGas"].(string); ok {
				userOp["preVerificationGas"] = preVerificationGas
			}
			if maxFeePerGas, ok := result["maxFeePerGas"].(string); ok {
				userOp["maxFeePerGas"] = maxFeePerGas
			}
			if maxPriorityFeePerGas, ok := result["maxPriorityFeePerGas"].(string); ok {
				userOp["maxPriorityFeePerGas"] = maxPriorityFeePerGas
			}
			logger.WithFields(logger.Fields{
				"SmartAccount": smartAccountAddress,
			}).Info("Paymaster sponsorship and gas estimates obtained for deployment")
		}
	}
	
	// Sign the deployment UserOp
	signature, err := s.signUserOperation(ctx, chainID, userOp)
	if err != nil {
		logger.WithFields(logger.Fields{
			"SmartAccount": smartAccountAddress,
			"Error": err.Error(),
		}).Error("Failed to sign deployment UserOperation")
		return fmt.Errorf("failed to sign deployment user operation: %w", err)
	}
	userOp["signature"] = signature
	
	logger.WithFields(logger.Fields{
		"SmartAccount": smartAccountAddress,
		"Signature":    signature,
		"SignatureLength": len(signature),
	}).Info("Deployment UserOperation signed successfully")
	
	// Send the deployment UserOp
	userOpHash, err := s.SendUserOperation(ctx, chainID, userOp)
	if err != nil {
		return fmt.Errorf("failed to send deployment user operation: %w", err)
	}
	
	logger.WithFields(logger.Fields{
		"SmartAccount": smartAccountAddress,
		"UserOpHash":   userOpHash,
	}).Info("Deployment UserOp sent, waiting for confirmation")
	
	// Wait for deployment to be mined
	_, err = s.WaitForUserOperationMined(ctx, chainID, userOpHash, 60*time.Second)
	if err != nil {
		return fmt.Errorf("deployment user operation failed: %w", err)
	}
	
	return nil
}

// sendUserOperationBatch sends a batch of transactions as a single user operation (for smart accounts)
func (s *AlchemyService) sendUserOperationBatch(ctx context.Context, chainID int64, smartAccountAddress string, txPayload []map[string]interface{}) (string, error) {
	// For batching, we need to encode multiple calls into a single user operation
	// This is a simplified implementation - in practice, you'd use proper ABI encoding
	
	var callData string
	if len(txPayload) == 1 {
		// Single transaction
		tx := txPayload[0]
		callData = tx["data"].(string)
	} else {
		// Multiple transactions - encode as batch call
		// This would typically use a multicall contract or batch execution
		callData = s.encodeBatchCallData(txPayload)
	}
	
	logger.WithFields(logger.Fields{
		"SmartAccount": smartAccountAddress,
		"CallDataLength": len(callData),
		"CallData": callData,
		"TxPayloadCount": len(txPayload),
	}).Info("Encoded callData for UserOp")

	// Check if smart account is deployed
	isDeployed, err := s.isAccountDeployed(ctx, chainID, smartAccountAddress)
	if err != nil {
		logger.Warnf("Failed to check if account is deployed: %v", err)
		isDeployed = false // Assume not deployed if check fails
	}

	// Get initCode if account is not deployed (will be included in the execution UserOp)
	var initCode string
	if !isDeployed {
		logger.WithFields(logger.Fields{
			"SmartAccount": smartAccountAddress,
		}).Info("Smart account not deployed - will deploy + execute in ONE transaction")
		
		// Retrieve the salt from database
		receiveAddr, err := storage.Client.ReceiveAddress.
			Query().
			Where(receiveaddress.AddressEQ(strings.ToLower(smartAccountAddress))).
			Only(ctx)
		
		if err != nil {
			return "", fmt.Errorf("failed to get receive address for salt: %w", err)
		}
		
		if len(receiveAddr.Salt) == 0 {
			return "", fmt.Errorf("no salt found for smart account %s - cannot generate initCode", smartAccountAddress)
		}
		
		// Decrypt the salt
		saltBytes, err := cryptoUtils.DecryptPlain(receiveAddr.Salt)
		if err != nil {
			return "", fmt.Errorf("failed to decrypt salt: %w", err)
		}
		
		// Convert salt to hex string
		saltHex := common.Bytes2Hex(saltBytes)
		
		// Get owner address (same as used in deployment)
		ownerAddress := viper.GetString("SMART_ACCOUNT_OWNER_ADDRESS")
		if ownerAddress == "" {
			return "", fmt.Errorf("SMART_ACCOUNT_OWNER_ADDRESS not configured")
		}
		
		initCode = s.getSmartAccountInitCode(ownerAddress, saltHex)
		
		logger.WithFields(logger.Fields{
			"SmartAccount": smartAccountAddress,
			"InitCodeLength": len(initCode),
		}).Info("Generated initCode for deployment + execution in single transaction")
	} else {
		initCode = "0x" // Account already deployed, no initCode needed
		logger.WithFields(logger.Fields{
			"SmartAccount": smartAccountAddress,
		}).Info("Smart account already deployed, executing transactions only")
	}

	// Create user operation
	// Adjust gas limits based on whether we're deploying or not
	var verificationGasLimit string
	if !isDeployed {
		// Higher gas limit for deployment + execution
		verificationGasLimit = "0x493e0" // 300k gas for deployment verification
	} else {
		// Lower gas limit for execution only
		verificationGasLimit = "0x30d40" // 200k gas for verification
	}
	
	userOp := map[string]interface{}{
		"sender":               smartAccountAddress,
		"nonce":                "0x0", // Should be fetched from the smart account
		"initCode":             initCode, // Includes deployment code if needed (deploy + execute in ONE tx)
		"callData":             callData,
		"callGasLimit":         "0x186a0", // 100k gas limit - should be estimated
		"verificationGasLimit": verificationGasLimit,
		"preVerificationGas":   "0x10000",  // 65536 gas - increased from 21k to meet Alchemy's minimum
		"maxFeePerGas":         "0x59682f00", // Should be fetched from gas oracle
		"maxPriorityFeePerGas": "0x59682f00",
		"paymasterAndData":     "0x", // Empty unless using paymaster
		"signature":            "0x", // Will be filled by the signer
	}

	// If gas policy is configured, request paymaster data and gas estimates
	if s.config.GasPolicyID != "" {
		// Create a UserOp for paymaster request with initial gas estimates
		// Alchemy will refine these estimates and return optimized values
		minimalUserOp := map[string]interface{}{
			"sender":   userOp["sender"],
			"nonce":    userOp["nonce"],
			"initCode": userOp["initCode"],
			"callData": userOp["callData"],
			// Add initial gas estimates (Alchemy will optimize these)
			"callGasLimit":         userOp["callGasLimit"],         // 100k default
			"verificationGasLimit": userOp["verificationGasLimit"], // From earlier calculation
			"preVerificationGas":   userOp["preVerificationGas"],   // 65536 default
			"maxFeePerGas":         userOp["maxFeePerGas"],         // From gas oracle
			"maxPriorityFeePerGas": userOp["maxPriorityFeePerGas"], // From gas oracle
		}
		
		result, err := s.getPaymasterData(ctx, chainID, minimalUserOp)
		if err != nil {
			logger.Warnf("Failed to get paymaster data: %v", err)
		} else {
			// Apply all gas estimates and paymaster data from the response
			if paymasterAndData, ok := result["paymasterAndData"].(string); ok {
				userOp["paymasterAndData"] = paymasterAndData
			}
			if callGasLimit, ok := result["callGasLimit"].(string); ok {
				userOp["callGasLimit"] = callGasLimit
			}
			if verificationGasLimit, ok := result["verificationGasLimit"].(string); ok {
				userOp["verificationGasLimit"] = verificationGasLimit
			}
			if preVerificationGas, ok := result["preVerificationGas"].(string); ok {
				userOp["preVerificationGas"] = preVerificationGas
			}
			if maxFeePerGas, ok := result["maxFeePerGas"].(string); ok {
				userOp["maxFeePerGas"] = maxFeePerGas
			}
			if maxPriorityFeePerGas, ok := result["maxPriorityFeePerGas"].(string); ok {
				userOp["maxPriorityFeePerGas"] = maxPriorityFeePerGas
			}
		}
	}

	// Sign the user operation
	signature, err := s.signUserOperation(ctx, chainID, userOp)
	if err != nil {
		return "", fmt.Errorf("failed to sign user operation: %w", err)
	}
	userOp["signature"] = signature

	logger.WithFields(logger.Fields{
		"SmartAccount": smartAccountAddress,
		"Signature":    signature,
		"SignatureLength": len(signature),
	}).Info("UserOperation signed successfully")

	// Send the user operation
	userOpHash, err := s.SendUserOperation(ctx, chainID, userOp)
	if err != nil {
		return "", fmt.Errorf("failed to send batch transaction: %w", err)
	}

	logger.WithFields(logger.Fields{
		"ChainID":      chainID,
		"SmartAccount": smartAccountAddress,
		"UserOpHash":   userOpHash,
		"BatchSize":    len(txPayload),
	}).Infof("Sent transaction batch via Alchemy")

	return userOpHash, nil
}

// sendEOATransactionBatch sends transactions from an EOA using eth_sendRawTransaction
// This requires the private key to be available (stored encrypted in database)
func (s *AlchemyService) sendEOATransactionBatch(ctx context.Context, chainID int64, fromAddress string, txPayload []map[string]interface{}) (string, error) {
	// For EOA, we need to send each transaction separately (no batching without multicall)
	// For now, send transactions sequentially
	
	if len(txPayload) == 0 {
		return "", fmt.Errorf("no transactions to send")
	}

	// Retrieve the encrypted private key from database
	receiveAddr, err := storage.Client.ReceiveAddress.
		Query().
		Where(receiveaddress.AddressEQ(fromAddress)).
		Only(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get receive address from database: %w", err)
	}

	if len(receiveAddr.Salt) == 0 {
		return "", fmt.Errorf("no private key found for address %s - this might be a Thirdweb smart account", fromAddress)
	}

	// Decrypt the private key
	privateKeyBytes, err := cryptoUtils.DecryptPlain(receiveAddr.Salt)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt private key: %w", err)
	}

	// Convert to ECDSA private key
	privateKey, err := crypto.ToECDSA(privateKeyBytes)
	if err != nil {
		return "", fmt.Errorf("failed to parse private key: %w", err)
	}

	logger.WithFields(logger.Fields{
		"From":      fromAddress,
		"ChainID":   chainID,
		"TxCount":   len(txPayload),
	}).Infof("Sending EOA transactions")

	// Send each transaction
	var lastTxHash string
	for i, tx := range txPayload {
		txHash, err := s.sendEOATransaction(ctx, chainID, privateKey, tx)
		if err != nil {
			return "", fmt.Errorf("failed to send transaction %d: %w", i, err)
		}
		lastTxHash = txHash
		
		logger.WithFields(logger.Fields{
			"TxHash": txHash,
			"Index":  i,
		}).Infof("Sent EOA transaction")
	}

	return lastTxHash, nil
}

// encodeBatchCallData encodes multiple transactions into a single call data using executeBatch
func (s *AlchemyService) encodeBatchCallData(txPayload []map[string]interface{}) string {
	// Alchemy Light Account has an executeBatch function:
	// function executeBatch(Call[] calldata calls) external payable
	// where Call is: struct Call { address target; uint256 value; bytes data; }
	
	// Function selector for executeBatch(Call[])
	// executeBatch((address,uint256,bytes)[])
	functionSelector := "18dfb3c7"
	
	// For ABI encoding of dynamic array of structs:
	// 1. Offset to array data (always 0x20 = 32 bytes)
	// 2. Array length
	// 3. For each struct: offset to its data field (relative to start of array data)
	// 4. All the actual data bytes
	
	var result []byte
	
	// Array length
	arrayLength := common.LeftPadBytes(big.NewInt(int64(len(txPayload))).Bytes(), 32)
	result = append(result, arrayLength...)
	
	// Calculate offset for each struct's data field
	// After array length, we have len(txPayload) structs, each with 3 fields (target, value, data_offset)
	// So the first data starts at: len(txPayload) * 3 * 32 bytes
	baseDataOffset := int64(len(txPayload) * 3 * 32)
	
	var structData []byte
	var allDataBytes []byte
	currentDataOffset := baseDataOffset
	
	for _, tx := range txPayload {
		target := common.HexToAddress(tx["to"].(string))
		value := big.NewInt(0)
		if v, ok := tx["value"].(string); ok && v != "0" {
			value.SetString(strings.TrimPrefix(v, "0x"), 16)
		}
		data := common.Hex2Bytes(strings.TrimPrefix(tx["data"].(string), "0x"))
		
		// Add struct fields: target, value, data_offset
		structData = append(structData, common.LeftPadBytes(target.Bytes(), 32)...)
		structData = append(structData, common.LeftPadBytes(value.Bytes(), 32)...)
		structData = append(structData, common.LeftPadBytes(big.NewInt(currentDataOffset).Bytes(), 32)...)
		
		// Encode the data bytes: length + data (padded to 32-byte boundary)
		dataLength := common.LeftPadBytes(big.NewInt(int64(len(data))).Bytes(), 32)
		dataPadded := data
		if len(data)%32 != 0 {
			padding := make([]byte, 32-(len(data)%32))
			dataPadded = append(data, padding...)
		}
		
		allDataBytes = append(allDataBytes, dataLength...)
		allDataBytes = append(allDataBytes, dataPadded...)
		
		// Update offset for next data field
		currentDataOffset += int64(32 + len(dataPadded))
	}
	
	// Combine: array_length + struct_data + all_data_bytes
	result = append(result, structData...)
	result = append(result, allDataBytes...)
	
	return "0x" + functionSelector + common.Bytes2Hex(result)
}

// signUserOperation signs a UserOperation with the owner's private key
func (s *AlchemyService) signUserOperation(ctx context.Context, chainID int64, userOp map[string]interface{}) (string, error) {
	logger.WithFields(logger.Fields{
		"ChainID": chainID,
		"Sender":  userOp["sender"],
	}).Info("Starting UserOperation signing")
	
	// Get owner private key
	ownerPrivateKey := viper.GetString("SMART_ACCOUNT_OWNER_PRIVATE_KEY")
	if ownerPrivateKey == "" {
		logger.WithFields(logger.Fields{}).Error("SMART_ACCOUNT_OWNER_PRIVATE_KEY not configured")
		return "", fmt.Errorf("SMART_ACCOUNT_OWNER_PRIVATE_KEY not configured")
	}
	
	logger.WithFields(logger.Fields{}).Info("Private key found, proceeding with signing")
	
	// Parse private key
	privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(ownerPrivateKey, "0x"))
	if err != nil {
		return "", fmt.Errorf("failed to parse private key: %w", err)
	}
	
	// Get the UserOp hash from the EntryPoint contract
	// For ERC-4337, the hash is: keccak256(abi.encode(userOpHash, entryPoint, chainId))
	entryPoint := common.HexToAddress("0x0000000071727De22E5E9d8baF0edAc6f37da032") // EntryPoint v0.7
	
	// Pack the UserOp for hashing (following ERC-4337 v0.7 spec)
	// For v0.7, gas fields are packed into bytes32:
	// - accountGasLimits = verificationGasLimit (16 bytes) + callGasLimit (16 bytes)
	// - gasFees = maxPriorityFeePerGas (16 bytes) + maxFeePerGas (16 bytes)
	// Hash = keccak256(abi.encode(
	//   sender, nonce, keccak256(initCode), keccak256(callData),
	//   accountGasLimits, preVerificationGas, gasFees,
	//   keccak256(paymasterAndData)
	// ))
	
	sender := common.HexToAddress(userOp["sender"].(string))
	nonce := new(big.Int)
	nonce.SetString(strings.TrimPrefix(userOp["nonce"].(string), "0x"), 16)
	
	initCode := common.Hex2Bytes(strings.TrimPrefix(userOp["initCode"].(string), "0x"))
	initCodeHash := crypto.Keccak256Hash(initCode)
	
	callData := common.Hex2Bytes(strings.TrimPrefix(userOp["callData"].(string), "0x"))
	callDataHash := crypto.Keccak256Hash(callData)
	
	callGasLimit := new(big.Int)
	callGasLimit.SetString(strings.TrimPrefix(userOp["callGasLimit"].(string), "0x"), 16)
	
	verificationGasLimit := new(big.Int)
	verificationGasLimit.SetString(strings.TrimPrefix(userOp["verificationGasLimit"].(string), "0x"), 16)
	
	preVerificationGas := new(big.Int)
	preVerificationGas.SetString(strings.TrimPrefix(userOp["preVerificationGas"].(string), "0x"), 16)
	
	maxFeePerGas := new(big.Int)
	maxFeePerGas.SetString(strings.TrimPrefix(userOp["maxFeePerGas"].(string), "0x"), 16)
	
	maxPriorityFeePerGas := new(big.Int)
	maxPriorityFeePerGas.SetString(strings.TrimPrefix(userOp["maxPriorityFeePerGas"].(string), "0x"), 16)
	
	paymasterAndData := common.Hex2Bytes(strings.TrimPrefix(userOp["paymasterAndData"].(string), "0x"))
	paymasterAndDataHash := crypto.Keccak256Hash(paymasterAndData)
	
	// Pack accountGasLimits: verificationGasLimit (16 bytes) + callGasLimit (16 bytes)
	accountGasLimits := make([]byte, 32)
	copy(accountGasLimits[0:16], common.LeftPadBytes(verificationGasLimit.Bytes(), 16))
	copy(accountGasLimits[16:32], common.LeftPadBytes(callGasLimit.Bytes(), 16))
	
	// Pack gasFees: maxPriorityFeePerGas (16 bytes) + maxFeePerGas (16 bytes)
	gasFees := make([]byte, 32)
	copy(gasFees[0:16], common.LeftPadBytes(maxPriorityFeePerGas.Bytes(), 16))
	copy(gasFees[16:32], common.LeftPadBytes(maxFeePerGas.Bytes(), 16))
	
	logger.WithFields(logger.Fields{
		"AccountGasLimits": "0x" + common.Bytes2Hex(accountGasLimits),
		"GasFees":          "0x" + common.Bytes2Hex(gasFees),
		"VerificationGas":  verificationGasLimit.String(),
		"CallGas":          callGasLimit.String(),
	}).Info("Packed gas fields for v0.7 hash")
	
	// Pack the UserOp hash according to ERC-4337 v0.7 (proper ABI encoding)
	var packed []byte
	packed = append(packed, common.LeftPadBytes(sender.Bytes(), 32)...)
	packed = append(packed, common.LeftPadBytes(nonce.Bytes(), 32)...)
	packed = append(packed, initCodeHash.Bytes()...)
	packed = append(packed, callDataHash.Bytes()...)
	packed = append(packed, accountGasLimits...)
	packed = append(packed, common.LeftPadBytes(preVerificationGas.Bytes(), 32)...)
	packed = append(packed, gasFees...)
	packed = append(packed, paymasterAndDataHash.Bytes()...)
	
	userOpHash := crypto.Keccak256Hash(packed)
	
	// Create the final hash with entryPoint and chainId
	chainIDBig := big.NewInt(chainID)
	var finalPacked []byte
	finalPacked = append(finalPacked, userOpHash.Bytes()...)
	finalPacked = append(finalPacked, common.LeftPadBytes(entryPoint.Bytes(), 32)...)
	finalPacked = append(finalPacked, common.LeftPadBytes(chainIDBig.Bytes(), 32)...)
	
	finalHash := crypto.Keccak256Hash(finalPacked)
	
	logger.WithFields(logger.Fields{
		"UserOpHash": userOpHash.Hex(),
		"FinalHash":  finalHash.Hex(),
		"EntryPoint": entryPoint.Hex(),
		"ChainID":    chainID,
	}).Info("Computed UserOp hash for signing")
	
	// Sign the hash
	signature, err := crypto.Sign(finalHash.Bytes(), privateKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign user operation: %w", err)
	}
	
	// Adjust v value for Ethereum (27 or 28)
	if signature[64] < 27 {
		signature[64] += 27
	}
	
	// For Light Account v2, prepend signature type byte
	// 0x00 = EOA signature (ECDSA)
	// 0x01 = Contract signature
	// 0x02 = WebAuthn signature
	signatureWithType := append([]byte{0x00}, signature...)
	
	finalSignature := "0x" + common.Bytes2Hex(signatureWithType)
	
	logger.WithFields(logger.Fields{
		"SignatureLength": len(finalSignature),
		"Signature":       finalSignature,
	}).Info("UserOperation signed successfully")
	
	return finalSignature, nil
}

// getMapKeys is a helper function to get all keys from a map
func getMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// getPaymasterData requests paymaster and data from Alchemy Gas Manager
// Returns the full result including gas estimates and paymasterAndData
func (s *AlchemyService) getPaymasterData(ctx context.Context, chainID int64, userOp map[string]interface{}) (map[string]interface{}, error) {
	// DEBUG: Log the incoming userOp BEFORE any processing
	userOpJSON, _ := json.Marshal(userOp)
	logger.WithFields(logger.Fields{
		"ChainID": chainID,
		"UserOpRaw": string(userOpJSON),
		"UserOpKeys": fmt.Sprintf("%v", getMapKeys(userOp)),
	}).Info("[DEBUG] getPaymasterData called with userOp")
	
	// Validate required fields
	requiredFields := []string{"sender", "nonce", "callData", "callGasLimit", "verificationGasLimit", "preVerificationGas", "maxFeePerGas", "maxPriorityFeePerGas"}
	for _, field := range requiredFields {
		if userOp[field] == nil {
			logger.WithFields(logger.Fields{
				"MissingField": field,
				"UserOp": string(userOpJSON),
			}).Error("[DEBUG] Missing required field in userOp")
			return nil, fmt.Errorf("missing required field '%s' in userOp", field)
		}
	}
	
	// Get network to use chain-specific RPC endpoint
	net, err := storage.Client.Network.
		Query().
		Where(network.ChainIDEQ(chainID)).
		Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get network for chain %d: %w", chainID, err)
	}
	
	// Use the network's RPC endpoint and append API key
	url := fmt.Sprintf("%s/%s", net.RPCEndpoint, s.config.APIKey)
	
	// Convert to v0.7 RPC format for paymaster request
	v07UserOp := s.packUserOperationV07(userOp)
	
	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "alchemy_requestGasAndPaymasterAndData",
		"params": []interface{}{
			map[string]interface{}{
				"policyId":      s.config.GasPolicyID,
				"entryPoint":    "0x0000000071727De22E5E9d8baF0edAc6f37da032", // EntryPoint v0.7
				"userOperation": v07UserOp,
				// Dummy signature for Light Account v2: 0x00 (EOA type) + 65 bytes of dummy signature
				"dummySignature": "0x00fffffffffffffffffffffffffffffff0000000000000000000000000000000007aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa1c",
			},
		},
		"id": 1,
	}

	// Log the request payload for debugging
	payloadJSON, _ := json.Marshal(payload)
	
	// Pretty print the v07UserOp for easier debugging
	v07UserOpJSON, _ := json.MarshalIndent(v07UserOp, "", "  ")
	
	logger.WithFields(logger.Fields{
		"ChainID":  chainID,
		"URL":      url,
		"PolicyID": s.config.GasPolicyID,
		"RequestLength": len(string(payloadJSON)),
		"Sender": v07UserOp["sender"],
		"Nonce": v07UserOp["nonce"],
		"Factory": v07UserOp["factory"],
		"FactoryDataLength": len(fmt.Sprintf("%v", v07UserOp["factoryData"])),
		"CallDataLength": len(fmt.Sprintf("%v", v07UserOp["callData"])),
		"CallGasLimit": v07UserOp["callGasLimit"],
		"VerificationGasLimit": v07UserOp["verificationGasLimit"],
		"PreVerificationGas": v07UserOp["preVerificationGas"],
	}).Info("[DEBUG] Requesting paymaster data from Alchemy")
	
	// Log the full v07UserOp structure
	logger.Infof("[DEBUG] Full v07UserOp:\n%s", string(v07UserOpJSON))
	
	// Log the full request to a separate log line to avoid truncation
	logger.Infof("[DEBUG] Full paymaster request: %s", string(payloadJSON))

	res, err := fastshot.NewClient(url).
		Config().SetTimeout(30 * time.Second).
		Header().AddAll(map[string]string{
			"Accept":       "application/json",
			"Content-Type": "application/json",
		}).Build().POST("").
		Body().AsJSON(payload).Send()
	
	if err != nil {
		return nil, fmt.Errorf("failed to get paymaster data: %w", err)
	}

	// Read response body first for better error handling
	bodyBytes, _ := io.ReadAll(res.RawResponse.Body)
	res.RawResponse.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	
	logger.WithFields(logger.Fields{
		"StatusCode":   res.StatusCode(),
		"ResponseBody": string(bodyBytes),
		"Headers":      res.RawResponse.Header,
	}).Info("Received paymaster response")
	
	data, err := utils.ParseJSONResponse(res.RawResponse)
	if err != nil {
		logger.WithFields(logger.Fields{
			"StatusCode": res.StatusCode(),
			"Body":       string(bodyBytes),
		}).Error("Failed to parse paymaster response")
		return nil, fmt.Errorf("failed to parse paymaster response (status %d): %s", res.StatusCode(), string(bodyBytes))
	}

	if data["error"] != nil {
		errorJSON, _ := json.Marshal(data["error"])
		
		// Extract detailed error information
		errorMap, _ := data["error"].(map[string]interface{})
		errorCode := ""
		errorMessage := ""
		errorData := ""
		revertData := ""
		
		if errorMap != nil {
			if code, ok := errorMap["code"]; ok {
				errorCode = fmt.Sprintf("%v", code)
			}
			if msg, ok := errorMap["message"]; ok {
				errorMessage = fmt.Sprintf("%v", msg)
			}
			if errData, ok := errorMap["data"].(map[string]interface{}); ok {
				errorDataJSON, _ := json.Marshal(errData)
				errorData = string(errorDataJSON)
				if reason, ok := errData["reason"]; ok {
					revertData = fmt.Sprintf("%v", reason)
				}
			}
		}
		
		logger.WithFields(logger.Fields{
			"ErrorFull": string(errorJSON),
			"ErrorCode": errorCode,
			"ErrorMessage": errorMessage,
			"ErrorData": errorData,
			"RevertData": revertData,
			"UserOpSender": v07UserOp["sender"],
			"UserOpNonce": v07UserOp["nonce"],
			"UserOpFactory": v07UserOp["factory"],
			"UserOpCallData": fmt.Sprintf("%v", v07UserOp["callData"])[:100] + "...",
		}).Error("[DEBUG] Paymaster request returned error - AA23 means validation/creation failed")
		
		// Provide helpful error context
		if errorMessage == "AA23 reverted" || revertData == "AA23 reverted" {
			logger.Errorf("[DEBUG] AA23 Error Analysis:")
			logger.Errorf("  - Check if smart account exists (if nonce > 0, it should exist)")
			logger.Errorf("  - Check if factory/factoryData is correct (if nonce = 0)")
			logger.Errorf("  - Check if owner address in factoryData is correct")
			logger.Errorf("  - Check if callData is properly encoded")
			logger.Errorf("  - Check if gas limits are sufficient")
		}
		
		return nil, fmt.Errorf("paymaster request failed: %v", data["error"])
	}

	result := data["result"].(map[string]interface{})
	
	// Log the full result for debugging
	resultJSON, _ := json.Marshal(result)
	logger.WithFields(logger.Fields{
		"Result": string(resultJSON),
	}).Info("Received paymaster and gas data from Alchemy")
	
	return result, nil
}

// GetTransactionStatus gets the status of a user operation (equivalent to GetTransactionStatus)
func (s *AlchemyService) GetTransactionStatus(ctx context.Context, userOpHash string, chainID int64) (map[string]interface{}, error) {
	receipt, err := s.GetUserOperationReceipt(ctx, chainID, userOpHash)
	if err != nil {
		return nil, err
	}

	// Convert to format similar to Thirdweb's response
	status := map[string]interface{}{
		"id":              userOpHash,
		"transactionHash": receipt["transactionHash"],
		"blockNumber":     receipt["blockNumber"],
		"from":            receipt["sender"],
		"executionResult": map[string]interface{}{
			"status": "CONFIRMED",
			"error":  nil,
		},
	}

	return status, nil
}

// IsHealthy checks if the Alchemy service is healthy and responsive
func (s *AlchemyService) IsHealthy(ctx context.Context) bool {
	// Test with a simple eth_blockNumber call
	_, err := s.GetLatestBlock(ctx, 1) // Ethereum mainnet
	return err == nil
}

// ============================================================================
// WEBHOOK MANAGEMENT
// ============================================================================

// AlchemyWebhookResponse represents the response from Alchemy webhook creation
type AlchemyWebhookResponse struct {
	Data struct {
		ID          string   `json:"id"`
		Network     string   `json:"network"`
		WebhookType string   `json:"webhook_type"`
		WebhookURL  string   `json:"webhook_url"`
		IsActive    bool     `json:"is_active"`
		TimeCreated int64    `json:"time_created"`
		Addresses   []string `json:"addresses"`
		Version     string   `json:"version"`
		SigningKey  string   `json:"signing_key"`
	} `json:"data"`
}

// AlchemyWebhookRequest represents the request to create an Alchemy webhook
type AlchemyWebhookRequest struct {
	Network     string   `json:"network"`
	WebhookType string   `json:"webhook_type"`
	WebhookURL  string   `json:"webhook_url"`
	Addresses   []string `json:"addresses"`
}

// CreateAddressActivityWebhook creates an Address Activity webhook for monitoring receive addresses
func (s *AlchemyService) CreateAddressActivityWebhook(ctx context.Context, chainID int64, addresses []string, webhookURL string) (webhookID string, signingKey string, err error) {
	// Map chain ID to Alchemy network identifier
	networkID, err := s.getAlchemyNetworkID(chainID)
	if err != nil {
		return "", "", fmt.Errorf("unsupported chain ID %d: %w", chainID, err)
	}

	// Prepare request payload
	payload := AlchemyWebhookRequest{
		Network:     networkID,
		WebhookType: "ADDRESS_ACTIVITY",
		WebhookURL:  webhookURL,
		Addresses:   addresses,
	}

	// Create HTTP client with auth token for webhook management
	client := fastshot.NewClient("https://dashboard.alchemy.com").
		Header().Add("X-Alchemy-Token", s.config.AuthToken).
		Build()

	// Make API request
	resp, err := client.POST("/api/create-webhook").
		Header().AddContentType("application/json").
		Body().AsJSON(payload).
		Send()

	if err != nil {
		return "", "", fmt.Errorf("failed to create Alchemy webhook: %w", err)
	}

	if resp.StatusCode() != 200 {
		return "", "", fmt.Errorf("Alchemy webhook creation failed with status %d", resp.StatusCode())
	}

	// Parse response
	data, err := utils.ParseJSONResponse(resp.RawResponse)
	if err != nil {
		return "", "", fmt.Errorf("failed to parse webhook response: %w", err)
	}

	webhookData := data["data"].(map[string]interface{})
	webhookID = webhookData["id"].(string)
	signingKey = webhookData["signing_key"].(string)

	logger.WithFields(logger.Fields{
		"WebhookID":   webhookID,
		"Network":     webhookData["network"],
		"Addresses":   addresses,
		"WebhookURL":  webhookURL,
		"SigningKey":  signingKey,
	}).Infof("Created Alchemy Address Activity webhook")

	return webhookID, signingKey, nil
}

// AddAddressesToWebhook adds new addresses to an existing webhook
func (s *AlchemyService) AddAddressesToWebhook(ctx context.Context, webhookID string, addresses []string) error {
	// Prepare request payload
	payload := map[string]interface{}{
		"webhook_id":           webhookID,
		"addresses_to_add":     addresses,
		"addresses_to_remove":  []string{},
	}

	// Create HTTP client with auth token
	client := fastshot.NewClient("https://dashboard.alchemy.com").
		Header().Add("X-Alchemy-Token", s.config.AuthToken).
		Build()

	// Make API request
	resp, err := client.PATCH("/api/update-webhook-addresses").
		Header().AddContentType("application/json").
		Body().AsJSON(payload).
		Send()

	if err != nil {
		return fmt.Errorf("failed to add addresses to webhook: %w", err)
	}

	if resp.StatusCode() != 200 {
		return fmt.Errorf("failed to add addresses with status %d", resp.StatusCode())
	}

	logger.WithFields(logger.Fields{
		"WebhookID": webhookID,
		"Addresses": addresses,
	}).Infof("Added addresses to Alchemy webhook")

	return nil
}

// RemoveAddressesFromWebhook removes addresses from an existing webhook
func (s *AlchemyService) RemoveAddressesFromWebhook(ctx context.Context, webhookID string, addresses []string) error {
	// Prepare request payload
	payload := map[string]interface{}{
		"webhook_id":           webhookID,
		"addresses_to_add":     []string{},
		"addresses_to_remove":  addresses,
	}

	// Create HTTP client with auth token
	client := fastshot.NewClient("https://dashboard.alchemy.com").
		Header().Add("X-Alchemy-Token", s.config.AuthToken).
		Build()

	// Make API request
	resp, err := client.PATCH("/api/update-webhook-addresses").
		Header().AddContentType("application/json").
		Body().AsJSON(payload).
		Send()

	if err != nil {
		return fmt.Errorf("failed to remove addresses from webhook: %w", err)
	}

	if resp.StatusCode() != 200 {
		return fmt.Errorf("failed to remove addresses with status %d", resp.StatusCode())
	}

	logger.WithFields(logger.Fields{
		"WebhookID": webhookID,
		"Addresses": addresses,
	}).Infof("Removed addresses from Alchemy webhook")

	return nil
}

// DeleteWebhook deletes an Alchemy webhook
func (s *AlchemyService) DeleteWebhook(ctx context.Context, webhookID string) error {
	// Create HTTP client with auth token
	client := fastshot.NewClient("https://dashboard.alchemy.com").
		Header().Add("X-Alchemy-Token", s.config.AuthToken).
		Build()

	// Make API request
	resp, err := client.DELETE(fmt.Sprintf("/api/delete-webhook?webhook_id=%s", webhookID)).
		Send()

	if err != nil {
		return fmt.Errorf("failed to delete webhook: %w", err)
	}

	if resp.StatusCode() != 200 {
		return fmt.Errorf("failed to delete webhook with status %d", resp.StatusCode())
	}

	logger.WithFields(logger.Fields{
		"WebhookID": webhookID,
	}).Infof("Deleted Alchemy webhook")

	return nil
}

// getAlchemyNetworkID maps chain IDs to Alchemy network identifiers
func (s *AlchemyService) getAlchemyNetworkID(chainID int64) (string, error) {
	networkMap := map[int64]string{
		1:     "ETH_MAINNET",
		11155111: "ETH_SEPOLIA",
		137:   "MATIC_MAINNET",
		80002: "MATIC_AMOY",
		42161: "ARB_MAINNET",
		421614: "ARB_SEPOLIA",
		10:    "OPT_MAINNET",
		11155420: "OPT_SEPOLIA",
		8453:  "BASE_MAINNET",
		84532: "BASE_SEPOLIA",
		56:    "BNB_MAINNET",
		97:    "BNB_TESTNET",
	}

	networkID, exists := networkMap[chainID]
	if !exists {
		return "", fmt.Errorf("unsupported chain ID: %d", chainID)
	}

	return networkID, nil
}

// isAccountDeployed checks if a smart account has been deployed on-chain
func (s *AlchemyService) isAccountDeployed(ctx context.Context, chainID int64, address string) (bool, error) {
	// Get RPC URL based on chain ID
	var rpcURL string
	switch chainID {
	case 84532: // Base Sepolia
		rpcURL = "https://sepolia.base.org"
	case 8453: // Base Mainnet
		rpcURL = "https://mainnet.base.org"
	case 11155111: // Ethereum Sepolia
		rpcURL = "https://sepolia.infura.io/v3/9aa3d95b3bc440fa88ea12eaa4456161"
	case 1: // Ethereum Mainnet
		rpcURL = "https://mainnet.infura.io/v3/9aa3d95b3bc440fa88ea12eaa4456161"
	default:
		// Try to get from database if available
		if storage.Client != nil {
			net, err := storage.Client.Network.
				Query().
				Where(network.ChainIDEQ(chainID)).
				Only(ctx)
			if err != nil {
				return false, fmt.Errorf("failed to get network: %w", err)
			}
			rpcURL = net.RPCEndpoint
		} else {
			return false, fmt.Errorf("unsupported chain ID: %d", chainID)
		}
	}

	// Use eth_getCode to check if there's contract code at the address
	url := rpcURL
	
	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "eth_getCode",
		"params":  []interface{}{address, "latest"},
		"id":      1,
	}

	res, err := fastshot.NewClient(url).
		Config().SetTimeout(10 * time.Second).
		Header().AddAll(map[string]string{
			"Accept":       "application/json",
			"Content-Type": "application/json",
		}).Build().POST("").
		Body().AsJSON(payload).Send()
	
	if err != nil {
		return false, fmt.Errorf("failed to check account deployment: %w", err)
	}

	data, err := utils.ParseJSONResponse(res.RawResponse)
	if err != nil {
		return false, fmt.Errorf("failed to parse response: %w", err)
	}

	if data["error"] != nil {
		return false, fmt.Errorf("RPC error: %v", data["error"])
	}

	// Get the code result
	code, ok := data["result"].(string)
	if !ok {
		return false, fmt.Errorf("invalid response format")
	}

	// If code is "0x" or empty, the account is not deployed
	// If code has content, the account is deployed
	isDeployed := code != "0x" && code != "" && code != "0x0"

	logger.WithFields(logger.Fields{
		"Address":    address,
		"ChainID":    chainID,
		"Code":       code,
		"IsDeployed": isDeployed,
	}).Debugf("Checked smart account deployment status")

	return isDeployed, nil
}

// sendEOATransaction signs and sends a single transaction from an EOA
func (s *AlchemyService) sendEOATransaction(ctx context.Context, chainID int64, privateKey *ecdsa.PrivateKey, txPayload map[string]interface{}) (string, error) {
	// Get RPC URL
	net, err := storage.Client.Network.
		Query().
		Where(network.ChainIDEQ(chainID)).
		Only(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get network: %w", err)
	}

	// Parse transaction parameters
	toAddress := common.HexToAddress(txPayload["to"].(string))
	data := common.FromHex(txPayload["data"].(string))
	value := big.NewInt(0)
	if txPayload["value"].(string) != "0" {
		value.SetString(txPayload["value"].(string), 0)
	}

	// Get nonce
	fromAddress := crypto.PubkeyToAddress(privateKey.PublicKey)
	nonce, err := s.getNonce(ctx, net.RPCEndpoint, fromAddress.Hex())
	if err != nil {
		return "", fmt.Errorf("failed to get nonce: %w", err)
	}

	// Get gas price
	gasPrice, err := s.getGasPrice(ctx, net.RPCEndpoint)
	if err != nil {
		return "", fmt.Errorf("failed to get gas price: %w", err)
	}

	// Estimate gas limit
	gasLimit := uint64(300000) // Default gas limit

	// Create transaction
	tx := types.NewTransaction(
		nonce,
		toAddress,
		value,
		gasLimit,
		gasPrice,
		data,
	)

	// Sign transaction
	signer := types.NewEIP155Signer(big.NewInt(chainID))
	signedTx, err := types.SignTx(tx, signer, privateKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign transaction: %w", err)
	}

	// Encode signed transaction
	var rawTxBytes []byte
	rawTxBytes, err = signedTx.MarshalBinary()
	if err != nil {
		return "", fmt.Errorf("failed to encode transaction: %w", err)
	}
	rawTxHex := "0x" + common.Bytes2Hex(rawTxBytes)

	// Send raw transaction
	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "eth_sendRawTransaction",
		"params":  []interface{}{rawTxHex},
		"id":      1,
	}

	res, err := fastshot.NewClient(net.RPCEndpoint).
		Config().SetTimeout(30 * time.Second).
		Header().AddAll(map[string]string{
			"Accept":       "application/json",
			"Content-Type": "application/json",
		}).Build().POST("").
		Body().AsJSON(payload).Send()

	if err != nil {
		return "", fmt.Errorf("failed to send transaction: %w", err)
	}

	data2, err := utils.ParseJSONResponse(res.RawResponse)
	if err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if data2["error"] != nil {
		return "", fmt.Errorf("RPC error: %v", data2["error"])
	}

	txHash := data2["result"].(string)
	return txHash, nil
}

// getNonce gets the transaction count (nonce) for an address
func (s *AlchemyService) getNonce(ctx context.Context, rpcURL, address string) (uint64, error) {
	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "eth_getTransactionCount",
		"params":  []interface{}{address, "pending"},
		"id":      1,
	}

	res, err := fastshot.NewClient(rpcURL).
		Config().SetTimeout(10 * time.Second).
		Header().AddAll(map[string]string{
			"Accept":       "application/json",
			"Content-Type": "application/json",
		}).Build().POST("").
		Body().AsJSON(payload).Send()

	if err != nil {
		return 0, fmt.Errorf("failed to get nonce: %w", err)
	}

	data, err := utils.ParseJSONResponse(res.RawResponse)
	if err != nil {
		return 0, fmt.Errorf("failed to parse response: %w", err)
	}

	if data["error"] != nil {
		return 0, fmt.Errorf("RPC error: %v", data["error"])
	}

	nonceHex := data["result"].(string)
	nonce, err := strconv.ParseUint(nonceHex[2:], 16, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse nonce: %w", err)
	}

	return nonce, nil
}

// getGasPrice gets the current gas price
func (s *AlchemyService) getGasPrice(ctx context.Context, rpcURL string) (*big.Int, error) {
	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "eth_gasPrice",
		"params":  []interface{}{},
		"id":      1,
	}

	res, err := fastshot.NewClient(rpcURL).
		Config().SetTimeout(10 * time.Second).
		Header().AddAll(map[string]string{
			"Accept":       "application/json",
			"Content-Type": "application/json",
		}).Build().POST("").
		Body().AsJSON(payload).Send()

	if err != nil {
		return nil, fmt.Errorf("failed to get gas price: %w", err)
	}

	data, err := utils.ParseJSONResponse(res.RawResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if data["error"] != nil {
		return nil, fmt.Errorf("RPC error: %v", data["error"])
	}

	gasPriceHex := data["result"].(string)
	gasPrice := new(big.Int)
	gasPrice.SetString(gasPriceHex[2:], 16)

	return gasPrice, nil
}

// GetAddressTransactionHistory fetches transaction history for an address using Alchemy's alchemy_getAssetTransfers API
func (s *AlchemyService) GetAddressTransactionHistory(ctx context.Context, chainID int64, walletAddress string, limit int, fromBlock int64, toBlock int64) ([]map[string]interface{}, error) {
	// Get network to use chain-specific RPC endpoint
	network, err := storage.Client.Network.
		Query().
		Where(network.ChainIDEQ(chainID)).
		Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get network for chain %d: %w", chainID, err)
	}
	
	// Use the network's RPC endpoint and append API key
	url := fmt.Sprintf("%s/%s", network.RPCEndpoint, s.config.APIKey)
	
	// Build params for alchemy_getAssetTransfers
	params := map[string]interface{}{
		"toAddress": walletAddress,
		"category":  []string{"erc20"},
		"maxCount":  fmt.Sprintf("0x%x", limit),
		"order":     "desc",
	}
	
	// Add block range if specified
	if fromBlock > 0 {
		params["fromBlock"] = fmt.Sprintf("0x%x", fromBlock)
	}
	if toBlock > 0 {
		params["toBlock"] = fmt.Sprintf("0x%x", toBlock)
	}
	
	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "alchemy_getAssetTransfers",
		"params":  []interface{}{params},
		"id":      1,
	}
	
	res, err := fastshot.NewClient(url).
		Config().SetTimeout(30 * time.Second).
		Header().AddAll(map[string]string{
			"Accept":       "application/json",
			"Content-Type": "application/json",
		}).Build().POST("").
		Body().AsJSON(payload).Send()
	
	if err != nil {
		return nil, fmt.Errorf("failed to get asset transfers: %w", err)
	}
	
	data, err := utils.ParseJSONResponse(res.RawResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}
	
	if data["error"] != nil {
		return nil, fmt.Errorf("alchemy API error: %v", data["error"])
	}
	
	result := data["result"].(map[string]interface{})
	transfers := result["transfers"].([]interface{})
	
	if len(transfers) == 0 {
		return []map[string]interface{}{}, nil
	}
	
	// Convert Alchemy format to Etherscan-compatible format
	transactions := make([]map[string]interface{}, len(transfers))
	for i, transfer := range transfers {
		t := transfer.(map[string]interface{})
		transactions[i] = map[string]interface{}{
			"hash":             t["hash"],
			"from":             t["from"],
			"to":               t["to"],
			"value":            t["value"],
			"tokenSymbol":      t["asset"],
			"tokenDecimal":     "6", // Default to 6 for USDC, should be fetched from token metadata
			"blockNumber":      t["blockNum"],
			"timeStamp":        "", // Alchemy doesn't provide timestamp in this API
			"contractAddress":  t["rawContract"].(map[string]interface{})["address"],
		}
	}
	
	return transactions, nil
}

// GetContractEventsRPC fetches contract events using RPC
func (s *AlchemyService) GetContractEventsRPC(ctx context.Context, rpcEndpoint string, contractAddress string, fromBlock int64, toBlock int64, topics []string, txHash string) ([]interface{}, error) {
	// Build full RPC URL with API key
	fullRPCURL := utils.BuildRPCURL(rpcEndpoint)
	
	// Create RPC client
	client, err := stablenodtypes.NewEthClient(fullRPCURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create RPC client: %w", err)
	}

	var logs []ethereumtypes.Log

	// Determine which event signatures to filter for based on topics
	var eventSignatures []string
	if len(topics) > 0 && topics[0] == utils.TransferEventSignature {
		eventSignatures = []string{utils.TransferEventSignature}
	} else {
		// Default to gateway event signatures
		eventSignatures = []string{
			utils.OrderCreatedEventSignature,
			utils.OrderSettledEventSignature,
			utils.OrderRefundedEventSignature,
		}
	}

	if txHash != "" {
		// Get specific transaction receipt
		receipt, err := client.TransactionReceipt(ctx, common.HexToHash(txHash))
		if err != nil {
			return nil, fmt.Errorf("failed to get transaction receipt: %w", err)
		}

		// Filter logs from receipt
		for _, log := range receipt.Logs {
			if log.Address == common.HexToAddress(contractAddress) {
				if len(log.Topics) > 0 {
					eventSignature := log.Topics[0].Hex()
					for _, signature := range eventSignatures {
						if eventSignature == signature {
							logs = append(logs, *log)
							break
						}
					}
				}
			}
		}
	} else {
		if fromBlock == 0 || toBlock == 0 {
			return nil, fmt.Errorf("fromBlock and toBlock must be provided")
		}

		// Use block range filtering
		filterQuery := ethereum.FilterQuery{
			FromBlock: big.NewInt(fromBlock),
			ToBlock:   big.NewInt(toBlock),
			Addresses: []common.Address{common.HexToAddress(contractAddress)},
			Topics:    [][]common.Hash{},
		}

		// Add additional topics if provided
		for _, topic := range topics {
			if topic != "" {
				filterQuery.Topics = append(filterQuery.Topics, []common.Hash{common.HexToHash(topic)})
			}
		}

		// Get all logs from the contract
		allLogs, err := client.FilterLogs(ctx, filterQuery)
		if err != nil {
			return nil, fmt.Errorf("failed to get logs: %w", err)
		}

		// Filter for specified event signatures
		for _, log := range allLogs {
			if len(log.Topics) > 0 {
				eventSignature := log.Topics[0].Hex()
				for _, signature := range eventSignatures {
					if eventSignature == signature {
						logs = append(logs, log)
						break
					}
				}
			}
		}
	}

	// Convert logs to standard format
	var events []interface{}
	for _, log := range logs {
		event := map[string]interface{}{
			"block_number":     float64(log.BlockNumber),
			"transaction_hash": log.TxHash.Hex(),
			"log_index":        float64(log.Index),
			"address":          log.Address.Hex(),
			"topics":           log.Topics,
			"data":             log.Data,
			"decoded": map[string]interface{}{
				"indexed_params":     make(map[string]interface{}),
				"non_indexed_params": make(map[string]interface{}),
			},
		}
		events = append(events, event)
	}

	// Decode events based on their signatures
	if len(events) > 0 {
		err = utils.ProcessRPCEventsBySignature(events)
		if err != nil {
			return nil, fmt.Errorf("failed to process RPC events: %w", err)
		}
	}

	return events, nil
}

// GetContractEventsWithFallback tries RPC to get contract events
func (s *AlchemyService) GetContractEventsWithFallback(ctx context.Context, network *ent.Network, contractAddress string, fromBlock int64, toBlock int64, topics []string, txHash string, eventPayload map[string]string) ([]interface{}, error) {
	logger.WithFields(logger.Fields{
		"TxHash":          txHash,
		"ContractAddress": contractAddress,
		"Topics":          topics,
		"Network":         network.Identifier,
	}).Debug("GetContractEventsWithFallback called")

	// Use RPC to get contract events
	events, err := s.GetContractEventsRPC(ctx, network.RPCEndpoint, contractAddress, fromBlock, toBlock, topics, txHash)
	if err != nil {
		logger.WithFields(logger.Fields{
			"TxHash":          txHash,
			"ContractAddress": contractAddress,
			"Error":           err.Error(),
		}).Warn("RPC failed to get contract events")
		return nil, fmt.Errorf("RPC failed to get contract events: %w", err)
	}

	logger.WithFields(logger.Fields{
		"TxHash":          txHash,
		"ContractAddress": contractAddress,
		"EventsFound":     len(events),
	}).Debug("GetContractEventsWithFallback completed")

	return events, nil
}
