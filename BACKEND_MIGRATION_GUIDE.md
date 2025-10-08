# Backend Migration Guide: Thirdweb to Alchemy

## Summary: Your Backend Will Function Exactly the Same

✅ **Same API calls**  
✅ **Same functionality**  
✅ **No code changes needed in handlers/controllers**  
✅ **Just configuration changes**

---

## Cost Comparison: Subscription Costs

### **Current: Thirdweb Engine**
- **Subscription**: $99-999/month
- **Features**: Managed key vault, wallet creation, transaction API
- **Total Monthly**: $99-999 + gas fees

### **After: Alchemy**  
- **Subscription**: $0-49/month (free tier covers most use cases)
- **Features**: Same functionality, you manage keys
- **Total Monthly**: $0-49 + gas fees

### **💰 Savings: $99-950/month**

---

## What is the Previous Aggregator Smart Account?

I checked `0x03Ff9504c7067980c1637BF9400E7b7e3655782c`:

✅ **It IS a smart contract** (ERC-4337 account)
- Created via Thirdweb Engine
- Uses upgradeable proxy pattern
- Controlled by keys in Thirdweb's vault
- **Same type as what you're creating with Alchemy**

**Key Point**: Thirdweb and Alchemy both create ERC-4337 smart accounts. The difference is:
- **Thirdweb**: They manage keys ($99-999/month)
- **Alchemy**: You manage keys ($0-49/month)

---

## Migration Options

### **Option 1: Gradual Migration (Recommended)**

**Phase 1**: Migrate receive addresses only
```bash
# .env configuration
USE_ALCHEMY_FOR_RECEIVE_ADDRESSES=true
SMART_ACCOUNT_OWNER_ADDRESS=0xFb84E5503bD20526f2579193411Dd0993d080775
SMART_ACCOUNT_OWNER_PRIVATE_KEY=0x...

# Keep existing operational account (for now)
AGGREGATOR_SMART_ACCOUNT=0x03Ff9504c7067980c1637BF9400E7b7e3655782c
```

**Benefits**:
- ✅ Immediate cost savings on wallet creation
- ✅ Keep existing operational flow
- ✅ Low risk
- ✅ Easy rollback

**Phase 2**: Migrate operational account later
```bash
# After testing Phase 1, replace operational account
AGGREGATOR_SMART_ACCOUNT=0x8493c7FF99dedD3da3eaCDC56ff474c12Ac3e67D  # Your new account
```

### **Option 2: Full Migration**

Switch everything at once:
```bash
USE_ALCHEMY_SERVICE=true
AGGREGATOR_SMART_ACCOUNT=0x8493c7FF99dedD3da3eaCDC56ff474c12Ac3e67D
```

---

## How Backend Functions Remain the Same

### **Before (Thirdweb)**
```go
// In your handlers/controllers
receiveAddressService := services.NewReceiveAddressService()
address, err := receiveAddressService.CreateSmartAddress(ctx, "order-123")
// Returns: 0xABC... (from Thirdweb)
```

### **After (Alchemy) - SAME CODE!**
```go
// In your handlers/controllers - NO CHANGES NEEDED
receiveAddressService := services.NewReceiveAddressService()
address, err := receiveAddressService.CreateSmartAddress(ctx, "order-123")
// Returns: 0xXYZ... (from Alchemy)
```

**The interface stays the same!** The service internally switches based on your `.env` configuration.

---

## Configuration Examples

### **Scenario A: Keep Everything on Thirdweb**
```bash
# Current setup - no changes
USE_ALCHEMY_SERVICE=false
USE_ALCHEMY_FOR_RECEIVE_ADDRESSES=false
AGGREGATOR_SMART_ACCOUNT=0x03Ff9504c7067980c1637BF9400E7b7e3655782c
```

**Cost**: $99-999/month + gas

### **Scenario B: Alchemy for Receive Addresses Only**
```bash
# Migrate receive addresses, keep operational account
USE_ALCHEMY_SERVICE=false
USE_ALCHEMY_FOR_RECEIVE_ADDRESSES=true
SMART_ACCOUNT_OWNER_ADDRESS=0xFb84E5503bD20526f2579193411Dd0993d080775
SMART_ACCOUNT_OWNER_PRIVATE_KEY=0x...
AGGREGATOR_SMART_ACCOUNT=0x03Ff9504c7067980c1637BF9400E7b7e3655782c
```

**Cost**: $0-49/month + gas (saves $50-950/month)

### **Scenario C: Full Alchemy Migration**
```bash
# Everything on Alchemy
USE_ALCHEMY_SERVICE=true
SMART_ACCOUNT_OWNER_ADDRESS=0xFb84E5503bD20526f2579193411Dd0993d080775
SMART_ACCOUNT_OWNER_PRIVATE_KEY=0x...
AGGREGATOR_SMART_ACCOUNT=0x8493c7FF99dedD3da3eaCDC56ff474c12Ac3e67D
```

**Cost**: $0-49/month + gas (saves $99-950/month)

---

## Testing the Migration

### **1. Test Receive Address Creation**
```bash
# Set in .env
USE_ALCHEMY_FOR_RECEIVE_ADDRESSES=true

# Run your tests
go test ./services -v -run TestCreateSmartAddress
```

### **2. Test with Existing Order Flow**
```bash
# Start your server
docker-compose up

# Create a test order
curl -X POST http://localhost:8080/v1/orders \
  -H "Content-Type: application/json" \
  -d '{"amount": "100", "token": "USDC", ...}'

# Check logs - should see "Creating receive address via Alchemy"
```

### **3. Rollback if Needed**
```bash
# Simply change back
USE_ALCHEMY_FOR_RECEIVE_ADDRESSES=false

# Restart
docker-compose restart
```

---

## What Stays the Same

✅ **API endpoints**: No changes  
✅ **Database schema**: No changes  
✅ **Handler code**: No changes  
✅ **Response formats**: No changes  
✅ **Order flow**: No changes  
✅ **Transaction execution**: No changes  

## What Changes

🔧 **Configuration**: `.env` flags  
🔧 **Service implementation**: Internal switching logic  
🔧 **Key management**: You manage keys instead of Thirdweb  
🔧 **Monthly costs**: $99-950 less per month  

---

## Security Considerations

### **Thirdweb (Current)**
- Keys stored in Thirdweb vault
- Accessed via API
- Thirdweb manages security
- **Cost**: $99-999/month for this service

### **Alchemy (New)**
- Keys in your `.env` file
- Direct cryptographic operations
- You manage security
- **Cost**: $0 for key management

### **Best Practices**
```bash
# 1. Secure your .env
chmod 600 .env

# 2. Never commit keys to git
# .gitignore already includes .env

# 3. Use different keys for testnet/mainnet
SMART_ACCOUNT_OWNER_PRIVATE_KEY_TESTNET=0x...
SMART_ACCOUNT_OWNER_PRIVATE_KEY_MAINNET=0x...

# 4. Rotate keys periodically
# Deploy new smart account with new owner
```

---

## Recommended Migration Path

### **Week 1: Setup**
1. ✅ Deploy smart account on Base Sepolia (Done!)
2. ✅ Update `receive_address.go` (Done!)
3. ✅ Add configuration flags (Done!)

### **Week 2: Testing**
1. Enable `USE_ALCHEMY_FOR_RECEIVE_ADDRESSES=true` on testnet
2. Create test orders
3. Verify receive addresses are created
4. Monitor logs and errors

### **Week 3: Production Rollout**
1. Enable on production: `USE_ALCHEMY_FOR_RECEIVE_ADDRESSES=true`
2. Monitor for 1 week
3. Verify cost savings

### **Week 4+: Full Migration** (Optional)
1. Deploy operational accounts per chain
2. Update `AGGREGATOR_SMART_ACCOUNT` per chain
3. Enable `USE_ALCHEMY_SERVICE=true`
4. Cancel Thirdweb subscription 🎉

---

## FAQ

**Q: Will my existing orders break?**  
A: No. Only NEW receive addresses will use Alchemy. Existing addresses continue working.

**Q: Can I switch back to Thirdweb?**  
A: Yes. Just set `USE_ALCHEMY_FOR_RECEIVE_ADDRESSES=false` and restart.

**Q: Do I need to change database schema?**  
A: No. Addresses are still just strings stored the same way.

**Q: What about Tron addresses?**  
A: Tron continues using the existing `CreateTronAddress()` method. No changes.

**Q: Will transaction fees change?**  
A: No. Gas fees remain the same. You just save on Thirdweb subscription costs.

**Q: Is the Alchemy account the same as Thirdweb's?**  
A: Yes, both create ERC-4337 smart contract accounts. Functionally identical.

---

## Support

If you encounter issues:
1. Check logs for errors
2. Verify `.env` configuration
3. Test on testnet first
4. Rollback if needed (just toggle the flag)

**Current Status**: ✅ Ready to migrate  
**Risk Level**: Low (gradual migration with easy rollback)  
**Estimated Savings**: $99-950/month
