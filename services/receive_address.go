package services

import (
	"context"
	"fmt"

	"github.com/NEDA-LABS/stablenode/config"
	"github.com/NEDA-LABS/stablenode/utils/logger"
	cryptoUtils "github.com/NEDA-LABS/stablenode/utils/crypto"
	"github.com/spf13/viper"
	tronWallet "github.com/paycrest/tron-wallet"
	tronEnums "github.com/paycrest/tron-wallet/enums"
)

// ReceiveAddressService provides functionality related to managing receive addresses
type ReceiveAddressService struct {
	engineService  *EngineService
	serviceManager *ServiceManager
}

// NewReceiveAddressService creates a new instance of ReceiveAddressService.
func NewReceiveAddressService() *ReceiveAddressService {
	return &ReceiveAddressService{
		engineService:  NewEngineService(),
		serviceManager: NewServiceManager(),
	}
}

// CreateSmartAddress function generates and saves a new EIP-4337 smart contract account address
// Seamlessly switches between Thirdweb and Alchemy based on configuration
func (s *ReceiveAddressService) CreateSmartAddress(ctx context.Context, label string) (string, error) {
	// Check if we should use Alchemy for receive addresses
	if viper.GetBool("USE_ALCHEMY_FOR_RECEIVE_ADDRESSES") {
		// Get chain ID from context or use default (Base Sepolia for now)
		chainID := int64(84532) // Base Sepolia - you can make this dynamic
		
		// Use the configured owner address for all receive addresses
		ownerAddress := viper.GetString("SMART_ACCOUNT_OWNER_ADDRESS")
		if ownerAddress == "" {
			logger.Warnf("SMART_ACCOUNT_OWNER_ADDRESS not set, falling back to Thirdweb")
			return s.engineService.CreateServerWallet(ctx, label)
		}
		
		// Create smart account via Alchemy
		logger.WithFields(logger.Fields{
			"Label":   label,
			"ChainID": chainID,
			"Owner":   ownerAddress,
		}).Infof("Creating receive address via Alchemy")
		
		return s.serviceManager.CreateServerWallet(ctx, label, chainID, ownerAddress)
	}
	
	// Fallback to Thirdweb Engine
	logger.WithFields(logger.Fields{
		"Label": label,
	}).Infof("Creating receive address via Thirdweb Engine")
	
	return s.engineService.CreateServerWallet(ctx, label)
}

// CreateTronAddress generates and saves a new Tron address
func (s *ReceiveAddressService) CreateTronAddress(ctx context.Context) (string, []byte, error) {
	serverConf := config.ServerConfig()
	var nodeUrl tronEnums.Node
	if serverConf.Environment == "production" {
		nodeUrl = tronEnums.MAIN_NODE
	} else {
		nodeUrl = tronEnums.SHASTA_NODE
	}

	// Generate a new Tron address
	wallet := tronWallet.GenerateTronWallet(nodeUrl)

	// Encrypt private key
	privateKeyEncrypted, err := cryptoUtils.EncryptPlain([]byte(wallet.PrivateKey))
	if err != nil {
		return "", nil, fmt.Errorf("failed to encrypt salt: %w", err)
	}

	return wallet.AddressBase58, privateKeyEncrypted, nil
}
