package services

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/viper"
	"github.com/NEDA-LABS/stablenode/utils/logger"
)

// ServiceManager manages switching between different blockchain service providers
type ServiceManager struct {
	engineService  *EngineService
	alchemyService *AlchemyService
	useAlchemy     bool
}

// NewServiceManager creates a new service manager
func NewServiceManager() *ServiceManager {
	return &ServiceManager{
		engineService:  NewEngineService(),
		alchemyService: NewAlchemyService(),
		useAlchemy:     viper.GetBool("USE_ALCHEMY_SERVICE"),
	}
}

// CreateServerWallet creates a smart contract account using the active service
func (sm *ServiceManager) CreateServerWallet(ctx context.Context, label string, chainID int64, ownerAddress string) (string, error) {
	if sm.useAlchemy {
		logger.Infof("Creating smart account via Alchemy for chain %d", chainID)
		return sm.alchemyService.CreateSmartAccount(ctx, chainID, ownerAddress)
	}
	
	logger.Infof("Creating server wallet via Thirdweb Engine")
	return sm.engineService.CreateServerWallet(ctx, label)
}

// SendTransactionBatch sends a batch of transactions using the active service
func (sm *ServiceManager) SendTransactionBatch(ctx context.Context, chainID int64, address string, txPayload []map[string]interface{}) (string, error) {
	if sm.useAlchemy {
		logger.WithFields(logger.Fields{
			"ChainID":   chainID,
			"Address":   address,
			"BatchSize": len(txPayload),
		}).Infof("Sending transaction batch via Alchemy")
		return sm.alchemyService.SendTransactionBatch(ctx, chainID, address, txPayload)
	}
	
	logger.WithFields(logger.Fields{
		"ChainID":   chainID,
		"Address":   address,
		"BatchSize": len(txPayload),
	}).Infof("Sending transaction batch via Thirdweb Engine")
	return sm.engineService.SendTransactionBatch(ctx, chainID, address, txPayload)
}

// GetTransactionStatus gets transaction status using the active service
func (sm *ServiceManager) GetTransactionStatus(ctx context.Context, transactionID string, chainID int64) (map[string]interface{}, error) {
	if sm.useAlchemy {
		return sm.alchemyService.GetTransactionStatus(ctx, transactionID, chainID)
	}
	
	return sm.engineService.GetTransactionStatus(ctx, transactionID)
}

// WaitForTransactionMined waits for transaction to be mined using the active service
func (sm *ServiceManager) WaitForTransactionMined(ctx context.Context, transactionID string, chainID int64, timeout time.Duration) (map[string]interface{}, error) {
	if sm.useAlchemy {
		return sm.alchemyService.WaitForUserOperationMined(ctx, chainID, transactionID, timeout)
	}
	
	return sm.engineService.WaitForTransactionMined(ctx, transactionID, timeout)
}

// GetLatestBlock gets the latest block using the active service
func (sm *ServiceManager) GetLatestBlock(ctx context.Context, chainID int64) (int64, error) {
	if sm.useAlchemy {
		return sm.alchemyService.GetLatestBlock(ctx, chainID)
	}
	
	return sm.engineService.GetLatestBlock(ctx, chainID)
}

// GetContractEvents gets contract events using the active service
func (sm *ServiceManager) GetContractEvents(ctx context.Context, chainID int64, contractAddress string, fromBlock, toBlock int64, topics []string) ([]interface{}, error) {
	if sm.useAlchemy {
		return sm.alchemyService.GetContractEvents(ctx, chainID, contractAddress, fromBlock, toBlock, topics)
	}
	
	// For Thirdweb, convert parameters to their expected format
	payload := map[string]string{
		"from_block": fmt.Sprintf("%d", fromBlock),
		"to_block":   fmt.Sprintf("%d", toBlock),
	}
	
	// Add topics if provided
	for i, topic := range topics {
		if topic != "" {
			payload[fmt.Sprintf("topic%d", i)] = topic
		}
	}
	
	return sm.engineService.GetContractEvents(ctx, chainID, contractAddress, payload)
}

// IsHealthy checks if the active service is healthy
func (sm *ServiceManager) IsHealthy(ctx context.Context) bool {
	if sm.useAlchemy {
		return sm.alchemyService.IsHealthy(ctx)
	}
	
	// For Thirdweb, we'll do a simple latest block check
	_, err := sm.engineService.GetLatestBlock(ctx, 1) // Ethereum mainnet
	return err == nil
}

// GetActiveService returns the name of the currently active service
func (sm *ServiceManager) GetActiveService() string {
	if sm.useAlchemy {
		return "Alchemy"
	}
	return "Thirdweb Engine"
}

// SwitchToAlchemy switches to using Alchemy service
func (sm *ServiceManager) SwitchToAlchemy() {
	sm.useAlchemy = true
	logger.Infof("Switched to Alchemy service")
}

// SwitchToThirdweb switches to using Thirdweb Engine service
func (sm *ServiceManager) SwitchToThirdweb() {
	sm.useAlchemy = false
	logger.Infof("Switched to Thirdweb Engine service")
}

// GetEngineService returns the Thirdweb Engine service (for backward compatibility)
func (sm *ServiceManager) GetEngineService() *EngineService {
	return sm.engineService
}

// GetAlchemyService returns the Alchemy service
func (sm *ServiceManager) GetAlchemyService() *AlchemyService {
	return sm.alchemyService
}
