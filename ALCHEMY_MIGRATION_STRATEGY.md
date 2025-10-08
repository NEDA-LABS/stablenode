# Alchemy Migration Strategy

## Current Thirdweb Usage Analysis

### **Pattern 1: Aggregator Smart Account (Operational)**
- **Address**: `0x03Ff9504c7067980c1637BF9400E7b7e3655782c`
- **Purpose**: Execute order operations (settle, refund, create)
- **Usage**: Single permanent account across ALL chains
- **Key Management**: Thirdweb Engine vault
- **Cost**: High (Thirdweb Engine fees per transaction)

### **Pattern 2: Receive Addresses (Per-Order)**
- **Creation**: `engineService.CreateServerWallet(ctx, label)`
- **Purpose**: Receive user deposits for each order
- **Usage**: Temporary, one per order
- **Key Management**: Thirdweb Engine vault
- **Cost**: High (Thirdweb Engine fees per wallet creation)

## Migration Options

### **Option 1: Hybrid Approach (Recommended)**

Keep existing EOA for operational account, use Alchemy for receive addresses:

```bash
# Operational Account (Keep existing for multi-chain)
AGGREGATOR_SMART_ACCOUNT=0x03Ff9504c7067980c1637BF9400E7b7e3655782c

# Alchemy for receive addresses (per-chain)
USE_ALCHEMY_FOR_RECEIVE_ADDRESSES=true
ALCHEMY_OWNER_ADDRESS=0xFb84E5503bD20526f2579193411Dd0993d080775
ALCHEMY_OWNER_PRIVATE_KEY=0x...
```

**Benefits:**
- ✅ Keep multi-chain operational account
- ✅ Reduce costs for receive address generation
- ✅ Gradual migration
- ✅ Lower risk

### **Option 2: Full Alchemy Migration**

Replace both patterns with Alchemy smart accounts:

```bash
# Per-chain operational accounts
AGGREGATOR_SMART_ACCOUNT_BASE=0x8493c7FF99dedD3da3eaCDC56ff474c12Ac3e67D
AGGREGATOR_SMART_ACCOUNT_ETHEREUM=0x... (deploy separately)
AGGREGATOR_SMART_ACCOUNT_POLYGON=0x... (deploy separately)

# Alchemy for receive addresses
USE_ALCHEMY_SERVICE=true
```

**Benefits:**
- ✅ Maximum cost savings
- ✅ Advanced features (batching, gas sponsorship)
- ❌ Need to deploy on each chain
- ❌ Higher complexity

### **Option 3: Receive Addresses Only**

Keep existing operational account, migrate only receive addresses:

```bash
# Keep existing operational account
AGGREGATOR_SMART_ACCOUNT=0x03Ff9504c7067980c1637BF9400E7b7e3655782c

# Use Alchemy only for receive addresses
USE_ALCHEMY_FOR_RECEIVE_ADDRESSES=true
```

## Implementation Plan

### **Phase 1: Receive Address Migration (Low Risk)**

1. **Update ReceiveAddressService**:
```go
func (s *ReceiveAddressService) CreateSmartAddress(ctx context.Context, label string, chainID int64) (string, error) {
    if viper.GetBool("USE_ALCHEMY_FOR_RECEIVE_ADDRESSES") {
        // Generate deterministic owner for this order
        orderOwner := s.generateOrderOwner(label)
        return s.alchemyService.CreateSmartAccount(ctx, chainID, orderOwner)
    }
    
    // Fallback to Thirdweb
    return s.engineService.CreateServerWallet(ctx, label)
}
```

2. **Owner Generation Strategy**:
```go
func (s *ReceiveAddressService) generateOrderOwner(label string) string {
    // Option A: Derive from master key + label
    masterKey := viper.GetString("ALCHEMY_MASTER_PRIVATE_KEY")
    derivedKey := crypto.DeriveKey(masterKey, label)
    return crypto.PubkeyToAddress(derivedKey.PublicKey).Hex()
    
    // Option B: Use single owner for all (simpler)
    return viper.GetString("ALCHEMY_OWNER_ADDRESS")
}
```

### **Phase 2: Operational Account Migration (Higher Risk)**

1. **Deploy operational accounts per chain**
2. **Update order service to use chain-specific accounts**
3. **Migrate existing orders gradually**

## Cost Analysis

### **Current Thirdweb Costs**
- **Wallet Creation**: ~$0.10-0.50 per wallet
- **Transaction Fees**: Engine fees + gas
- **Key Management**: Included in Engine

### **Alchemy Costs**
- **Wallet Creation**: Gas cost only (~$0.01-0.05)
- **Transaction Fees**: Gas only (no Engine fees)
- **Key Management**: Self-managed

### **Estimated Savings**
- **Receive Addresses**: 80-90% cost reduction
- **Transactions**: 50-70% cost reduction
- **Total**: Significant savings for high-volume operations

## Security Considerations

### **Key Management**
```bash
# Current (Thirdweb Engine Vault)
- All keys stored in Thirdweb's vault
- Accessed via Engine API
- Managed by Thirdweb

# New (Self-Managed)
- Keys stored in your environment
- Direct cryptographic operations
- You manage security
```

### **Risk Mitigation**
1. **Environment Security**: Secure .env files
2. **Key Rotation**: Regular key updates
3. **Access Control**: Limit who can access keys
4. **Monitoring**: Track all transactions

## Recommended Implementation

### **Start with Option 1 (Hybrid)**

1. **Keep existing operational account** for multi-chain compatibility
2. **Migrate receive addresses** to Alchemy for cost savings
3. **Test thoroughly** on testnet
4. **Monitor performance** and costs
5. **Consider full migration** after validation

### **Configuration**
```bash
# Operational (existing)
AGGREGATOR_SMART_ACCOUNT=0x03Ff9504c7067980c1637BF9400E7b7e3655782c

# Receive addresses (new)
USE_ALCHEMY_FOR_RECEIVE_ADDRESSES=true
ALCHEMY_OWNER_ADDRESS=0xFb84E5503bD20526f2579193411Dd0993d080775
ALCHEMY_OWNER_PRIVATE_KEY=0x...

# Per-chain smart accounts (future)
SMART_ACCOUNT_BASE_SEPOLIA=0x8493c7FF99dedD3da3eaCDC56ff474c12Ac3e67D
```

## Next Steps

1. **Implement hybrid receive address service**
2. **Test on Base Sepolia**
3. **Measure cost savings**
4. **Plan operational account migration**
5. **Deploy to production gradually**

---
**Recommendation**: Start with **Option 1 (Hybrid)** for lowest risk and immediate cost savings on receive addresses.
