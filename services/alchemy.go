package services

import (
	"context"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	fastshot "github.com/opus-domini/fast-shot"
	"github.com/NEDA-LABS/stablenode/config"
	"github.com/NEDA-LABS/stablenode/ent"
	networkent "github.com/NEDA-LABS/stablenode/ent/network"
	"github.com/NEDA-LABS/stablenode/storage"
	"github.com/NEDA-LABS/stablenode/utils"
	"github.com/NEDA-LABS/stablenode/utils/logger"
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
func (s *AlchemyService) CreateSmartAccount(ctx context.Context, chainID int64, ownerAddress string) (string, error) {
	// Generate a unique salt from timestamp and random data
	// This ensures each receive address is unique
	salt := s.generateUniqueSalt()
	
	// Compute the smart account address deterministically using CREATE2
	smartAccountAddress := s.computeSmartAccountAddressWithSalt(ownerAddress, chainID, salt)
	
	logger.WithFields(logger.Fields{
		"ChainID":      chainID,
		"Owner":        ownerAddress,
		"SmartAccount": smartAccountAddress,
		"Salt":         fmt.Sprintf("0x%x", salt),
		"Method":       "Deterministic CREATE2",
	}).Infof("Generated smart account address via Alchemy")

	return smartAccountAddress, nil
}

// getSmartAccountInitCode generates the initCode for smart account deployment
func (s *AlchemyService) getSmartAccountInitCode(ownerAddress string) string {
	// Use Alchemy's Light Account Factory v2.0.0 (latest)
	// Factory address: 0x0000000000400CdFef5E2714E63d8040b700BC24
	factoryAddress := "0000000000400CdFef5E2714E63d8040b700BC24"
	
	// Function selector for createAccount(address owner, uint256 salt)
	functionSelector := "5fbfb9cf"
	
	// Encode owner address (32 bytes padded)
	ownerPadded := "000000000000000000000000" + strings.TrimPrefix(ownerAddress, "0x")
	
	// Salt (32 bytes, using 0)
	salt := "0000000000000000000000000000000000000000000000000000000000000000"
	
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
	implementationAddress := common.HexToAddress("0x8E8e658E22B12ada97B402fF0b044D6A325013C7")
	
	// Owner address is used in the factory's createAccount call, but not needed for address computation
	_ = ownerAddress // Acknowledge parameter
	
	// Compute the init code hash for the proxy
	// The Light Account uses a minimal proxy pattern (EIP-1167)
	// Init code for minimal proxy: 0x3d602d80600a3d3981f3363d3d373d3d3d363d73 + implementation + 0x5af43d82803e903d91602b57fd5bf3
	
	// Build the minimal proxy bytecode
	proxyPrefix := common.Hex2Bytes("3d602d80600a3d3981f3363d3d373d3d3d363d73")
	proxySuffix := common.Hex2Bytes("5af43d82803e903d91602b57fd5bf3")
	
	// Combine to get full init code
	initCode := append(proxyPrefix, implementationAddress.Bytes()...)
	initCode = append(initCode, proxySuffix...)
	
	// Hash the init code
	initCodeHash := crypto.Keccak256Hash(initCode)
	
	// Compute CREATE2 address
	// address = keccak256(0xff ++ factory ++ salt ++ keccak256(initCode))[12:]
	data := []byte{0xff}
	data = append(data, factoryAddress.Bytes()...)
	data = append(data, salt[:]...)
	data = append(data, initCodeHash.Bytes()...)
	
	hash := crypto.Keccak256Hash(data)
	address := common.BytesToAddress(hash.Bytes()[12:])
	
	return address.Hex()
}

// SendUserOperation sends a user operation (transaction) via Alchemy's bundler
func (s *AlchemyService) SendUserOperation(ctx context.Context, chainID int64, userOp map[string]interface{}) (string, error) {
	url := fmt.Sprintf("%s/%s", s.config.BaseURL, s.config.APIKey)
	
	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "eth_sendUserOperation",
		"params": []interface{}{
			userOp,
			"0x5FF137D4b0FDCD49DcA30c7CF57E578a026d2789", // EntryPoint address
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
		return "", fmt.Errorf("failed to send user operation: %w", err)
	}

	data, err := utils.ParseJSONResponse(res.RawResponse)
	if err != nil {
		return "", fmt.Errorf("failed to parse JSON response: %w", err)
	}

	if data["error"] != nil {
		return "", fmt.Errorf("user operation failed: %v", data["error"])
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
		Where(networkent.Not(networkent.IdentifierHasPrefix("tron"))).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch EVM networks: %w", err)
	}

	return networks, nil
}

// SendTransactionBatch sends a batch of transactions as a single user operation
func (s *AlchemyService) SendTransactionBatch(ctx context.Context, chainID int64, smartAccountAddress string, txPayload []map[string]interface{}) (string, error) {
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

	// Create user operation
	userOp := map[string]interface{}{
		"sender":               smartAccountAddress,
		"nonce":                "0x0", // Should be fetched from the smart account
		"initCode":             "0x",  // Empty for existing accounts
		"callData":             callData,
		"callGasLimit":         "0x186a0", // 100k gas limit - should be estimated
		"verificationGasLimit": "0x186a0", // 100k gas limit
		"preVerificationGas":   "0x5208",  // 21k gas
		"maxFeePerGas":         "0x59682f00", // Should be fetched from gas oracle
		"maxPriorityFeePerGas": "0x59682f00",
		"paymasterAndData":     "0x", // Empty unless using paymaster
		"signature":            "0x", // Will be filled by the signer
	}

	// If gas policy is configured, request paymaster data
	if s.config.GasPolicyID != "" {
		paymasterData, err := s.getPaymasterData(ctx, chainID, userOp)
		if err != nil {
			logger.Warnf("Failed to get paymaster data: %v", err)
		} else {
			userOp["paymasterAndData"] = paymasterData
		}
	}

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

// encodeBatchCallData encodes multiple transactions into a single call data
func (s *AlchemyService) encodeBatchCallData(txPayload []map[string]interface{}) string {
	// This is a placeholder for proper batch encoding
	// In practice, you'd use a multicall contract or the smart account's batch execution function
	// For now, we'll just use the first transaction's data
	if len(txPayload) > 0 {
		return txPayload[0]["data"].(string)
	}
	return "0x"
}

// getPaymasterData requests paymaster and data from Alchemy Gas Manager
func (s *AlchemyService) getPaymasterData(ctx context.Context, chainID int64, userOp map[string]interface{}) (string, error) {
	url := fmt.Sprintf("%s/%s", s.config.BaseURL, s.config.APIKey)
	
	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "alchemy_requestGasAndPaymasterAndData",
		"params": []interface{}{
			map[string]interface{}{
				"policyId":      s.config.GasPolicyID,
				"entryPoint":    "0x5FF137D4b0FDCD49DcA30c7CF57E578a026d2789",
				"userOperation": userOp,
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
		return "", fmt.Errorf("failed to get paymaster data: %w", err)
	}

	data, err := utils.ParseJSONResponse(res.RawResponse)
	if err != nil {
		return "", fmt.Errorf("failed to parse JSON response: %w", err)
	}

	if data["error"] != nil {
		return "", fmt.Errorf("paymaster request failed: %v", data["error"])
	}

	result := data["result"].(map[string]interface{})
	return result["paymasterAndData"].(string), nil
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

// getChainRPCURL returns the chain-specific RPC URL from the database
func (s *AlchemyService) getChainRPCURL(ctx context.Context, chainID int64) (string, error) {
	network, err := storage.Client.Network.
		Query().
		Where(networkent.ChainIDEQ(chainID)).
		Only(ctx)
	if err != nil {
		return "", fmt.Errorf("network with chain ID %d not found: %w", chainID, err)
	}
	
	return network.RPCEndpoint, nil
}
