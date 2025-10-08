# Smart Account Usage Guide

## Your Deployed Smart Account

**Smart Account Address**: `0x8493c7FF99dedD3da3eaCDC56ff474c12Ac3e67D`  
**Owner Address**: `0xFb84E5503bD20526f2579193411Dd0993d080775`  
**Chain**: Base Sepolia (84532)

## Configuration

### Add to your `.env` file:

```bash
# Smart Account Configuration
SMART_ACCOUNT_ADDRESS=0x8493c7FF99dedD3da3eaCDC56ff474c12Ac3e67D
SMART_ACCOUNT_OWNER_ADDRESS=0xFb84E5503bD20526f2579193411Dd0993d080775
SMART_ACCOUNT_OWNER_PRIVATE_KEY=0x...  # Keep secure!

# For production, you might want to replace:
# AGGREGATOR_SMART_ACCOUNT=0x8493c7FF99dedD3da3eaCDC56ff474c12Ac3e67D
```

## Use Cases

### **1. As Your Aggregator Account**
Replace your current aggregator smart account with this one:

```go
// In your order processing code
smartAccountAddress := viper.GetString("SMART_ACCOUNT_ADDRESS")

// Use it to execute transactions
txHash, err := serviceManager.SendTransactionBatch(
    ctx,
    chainID,
    smartAccountAddress,  // Your smart account
    txPayload,
)
```

### **2. For User Wallets**
Create smart accounts for each user:

```go
// When a user signs up
userOwnerAddress := user.WalletAddress
smartAccountAddress, err := alchemyService.CreateSmartAccount(
    ctx,
    chainID,
    userOwnerAddress,
)

// Store in database
user.SmartAccountAddress = smartAccountAddress
```

### **3. For Transaction Batching**
Send multiple operations in one transaction:

```go
// Batch multiple operations
txPayload := []map[string]interface{}{
    {
        "to":   "0xTokenContract...",
        "data": "0x...", // approve()
    },
    {
        "to":   "0xDEXContract...",
        "data": "0x...", // swap()
    },
}

// Execute as single transaction
txHash, err := alchemyService.SendTransactionBatch(
    ctx,
    chainID,
    smartAccountAddress,
    txPayload,
)
```

## Transaction Flow

### **Traditional EOA (Old Way)**
```
User → Signs Transaction → Blockchain
```

### **Smart Account (New Way)**
```
Owner → Signs UserOperation → Bundler → Smart Account → Blockchain
                                ↓
                          (Optional) Paymaster
                          (Sponsors Gas)
```

## Key Differences

| Feature | EOA (Regular Wallet) | Smart Account |
|---------|---------------------|---------------|
| **Gas Payment** | Must pay from wallet | Can be sponsored |
| **Batching** | One tx at a time | Multiple ops in one tx |
| **Recovery** | Lost key = lost funds | Can add recovery mechanisms |
| **Programmability** | No logic | Custom logic possible |
| **Cost** | Lower gas | Slightly higher gas |

## Common Operations

### **1. Check Balance**
```bash
# Check smart account balance
curl https://base-sepolia.g.alchemy.com/v2/YOUR_API_KEY \
  -X POST \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc":"2.0",
    "method":"eth_getBalance",
    "params":["0x8493c7FF99dedD3da3eaCDC56ff474c12Ac3e67D", "latest"],
    "id":1
  }'
```

### **2. Fund the Account**
```bash
# Send testnet ETH to your smart account
# From: Your owner wallet
# To: 0x8493c7FF99dedD3da3eaCDC56ff474c12Ac3e67D
# Amount: 0.01 ETH (for testing)
```

### **3. Send Transaction**
```go
// Example: Transfer tokens
service := services.NewAlchemyService()

txPayload := []map[string]interface{}{
    {
        "to":    "0xRecipientAddress...",
        "value": "1000000000000000", // 0.001 ETH in wei
        "data":  "0x",
    },
}

txHash, err := service.SendTransactionBatch(
    ctx,
    84532, // Base Sepolia
    "0x8493c7FF99dedD3da3eaCDC56ff474c12Ac3e67D",
    txPayload,
)
```

## Integration Examples

### **Example 1: Replace Aggregator Account**

**Before (Thirdweb):**
```go
aggregatorAccount := viper.GetString("AGGREGATOR_SMART_ACCOUNT")
// 0x03Ff9504c7067980c1637BF9400E7b7e3655782c
```

**After (Alchemy):**
```go
aggregatorAccount := viper.GetString("SMART_ACCOUNT_ADDRESS")
// 0x8493c7FF99dedD3da3eaCDC56ff474c12Ac3e67D
```

### **Example 2: Create Smart Accounts for Users**

```go
func CreateUserSmartAccount(ctx context.Context, userAddress string) (string, error) {
    service := services.NewAlchemyService()
    
    // Compute deterministic address
    smartAccountAddress := service.computeSmartAccountAddress(
        userAddress,
        84532, // Base Sepolia
    )
    
    // Check if already deployed
    code, err := checkCode(smartAccountAddress)
    if len(code) > 0 {
        return smartAccountAddress, nil // Already deployed
    }
    
    // Deploy via first transaction (lazy deployment)
    return smartAccountAddress, nil
}
```

### **Example 3: Gas-Sponsored Transactions**

```go
// Configure Gas Manager policy in Alchemy dashboard
// Then transactions will be automatically sponsored

service := services.NewAlchemyService()
// Gas Manager policy ID is in .env: ALCHEMY_GAS_POLICY_ID

txHash, err := service.SendTransactionBatch(
    ctx,
    chainID,
    smartAccountAddress,
    txPayload,
)
// User pays no gas! 🎉
```

## Security Best Practices

1. **Never commit private keys** - Use environment variables
2. **Use different accounts** for testnet vs mainnet
3. **Limit smart account permissions** - Only give necessary access
4. **Monitor transactions** - Set up alerts for unusual activity
5. **Test on testnet first** - Always test before mainnet

## Troubleshooting

### **Issue: "Insufficient funds"**
**Solution**: Fund the smart account with ETH
```bash
# Send ETH to: 0x8493c7FF99dedD3da3eaCDC56ff474c12Ac3e67D
```

### **Issue: "Invalid signature"**
**Solution**: Ensure you're signing with the correct owner private key

### **Issue: "Account not deployed"**
**Solution**: Deploy the account first (already done for you!)

## Next Steps

1. ✅ Smart account deployed
2. [ ] Fund the smart account with testnet ETH
3. [ ] Send a test transaction
4. [ ] Integrate into your application
5. [ ] Test with real use cases
6. [ ] Deploy to mainnet (when ready)

---
**Your Smart Account**: https://sepolia.basescan.org/address/0x8493c7FF99dedD3da3eaCDC56ff474c12Ac3e67D
