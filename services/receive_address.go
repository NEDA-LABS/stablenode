package services

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/NEDA-LABS/stablenode/config"
	cryptoUtils "github.com/NEDA-LABS/stablenode/utils/crypto"
	"github.com/NEDA-LABS/stablenode/utils/logger"
	tronWallet "github.com/paycrest/tron-wallet"
	tronEnums "github.com/paycrest/tron-wallet/enums"
	"github.com/spf13/viper"
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
// Returns: address, encryptedSalt (for Alchemy smart accounts), error
func (s *ReceiveAddressService) CreateSmartAddress(ctx context.Context, label string) (string, []byte, error) {
	// Check if we should use Alchemy for receive addresses
	if viper.GetBool("USE_ALCHEMY_FOR_RECEIVE_ADDRESSES") {
		useSmartAccounts := viper.GetBool("USE_ALCHEMY_SMART_ACCOUNTS")
		
		if useSmartAccounts {
			// Create Alchemy smart account (with gas sponsorship support)
			logger.WithFields(logger.Fields{
				"Label": label,
			}).Infof("Creating Alchemy smart account for receive address")

			// Get chain ID (default to Base Sepolia)
			chainID := viper.GetInt64("DEFAULT_CHAIN_ID")
			if chainID == 0 {
				chainID = 84532 // Base Sepolia
			}

			// Get owner address (the account that will control all receive addresses)
			ownerAddress := viper.GetString("SMART_ACCOUNT_OWNER_ADDRESS")
			if ownerAddress == "" {
				return "", nil, fmt.Errorf("SMART_ACCOUNT_OWNER_ADDRESS not configured")
			}

			// Create smart account via Alchemy
			address, salt, err := s.serviceManager.CreateServerWallet(ctx, label, chainID, ownerAddress)
			logger.WithFields(logger.Fields{
				"address": address,
				"saltLength": len(salt),
				"saltIsNil": salt == nil,
			}).Infof("CreateServerWallet returned")
			return address, salt, err
		} else {
			// Create EOA (simpler, no gas sponsorship)
			logger.WithFields(logger.Fields{
				"Label": label,
			}).Infof("Creating EOA receive address for Alchemy")

			// Generate new EOA (returns address and encrypted private key)
			return s.CreateEVMAddress(ctx)
		}
	}

	// Fallback to Thirdweb Engine (no private key needed)
	logger.WithFields(logger.Fields{
		"Label": label,
	}).Infof("Creating receive address via Thirdweb Engine")

	address, err := s.engineService.CreateServerWallet(ctx, label)
	return address, nil, err
}

// CreateEVMAddress generates a new EOA (Externally Owned Account) for EVM chains
// Returns the address and encrypted private key (to be stored in salt field)
func (s *ReceiveAddressService) CreateEVMAddress(ctx context.Context) (string, []byte, error) {
	// Generate a new Ethereum private key
	privateKey, err := crypto.GenerateKey()
	if err != nil {
		return "", nil, fmt.Errorf("failed to generate private key: %w", err)
	}

	// Get the address from the private key
	address := crypto.PubkeyToAddress(privateKey.PublicKey).Hex()

	// Get private key bytes
	privateKeyBytes := crypto.FromECDSA(privateKey)

	// Encrypt private key
	privateKeyEncrypted, err := cryptoUtils.EncryptPlain(privateKeyBytes)
	if err != nil {
		return "", nil, fmt.Errorf("failed to encrypt private key: %w", err)
	}

	logger.WithFields(logger.Fields{
		"Address": address,
	}).Infof("Generated new EOA receive address")

	return address, privateKeyEncrypted, nil
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
