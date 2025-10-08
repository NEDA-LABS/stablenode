# Alchemy Integration Setup Guide

## Quick Start

### 1. Get Alchemy Credentials
1. Go to [Alchemy Dashboard](https://dashboard.alchemy.com/)
2. Create a new app for your target chain (e.g., Polygon, Ethereum)
3. Copy your **API Key** from the app dashboard
4. (Optional) Set up **Gas Manager** policy for transaction sponsorship

### 2. Configure Environment Variables
Add these to your `.env` file:

```bash
# Alchemy Configuration
ALCHEMY_API_KEY=your_actual_alchemy_api_key
ALCHEMY_BASE_URL=https://api.g.alchemy.com/v2
ALCHEMY_GAS_POLICY_ID=your_gas_policy_id  # Optional

# Switch to Alchemy service
USE_ALCHEMY_SERVICE=true
```

### 3. Test the Integration

#### Run Basic Tests
```bash
# Test service creation and configuration
go test ./services -v -run TestAlchemyServiceCreation

# Test health check (requires valid API key)
go test ./services -v -run TestAlchemyServiceHealthCheck

# Test service manager
go test ./services -v -run TestServiceManager
```

#### Test Smart Account Creation
```bash
# This will test deterministic address generation
go test ./services -v -run TestSmartAccountAddressGeneration
```

### 4. Switch Between Services

#### Use Alchemy (New)
```bash
# In your .env file
USE_ALCHEMY_SERVICE=true
```

#### Use Thirdweb (Original)
```bash
# In your .env file
USE_ALCHEMY_SERVICE=false
```

### 5. Verify Service is Active
When you start your application, you should see:
```
Using blockchain service: Alchemy
```
or
```
Using blockchain service: Thirdweb Engine
```

## API Differences

### Smart Account Creation

**Thirdweb Engine:**
```go
address, err := engineService.CreateServerWallet(ctx, "label")
```

**Alchemy:**
```go
address, err := alchemyService.CreateSmartAccount(ctx, chainID, ownerAddress)
```

**Service Manager (Unified):**
```go
address, err := serviceManager.CreateServerWallet(ctx, "label", chainID, ownerAddress)
```

### Transaction Sending

Both services use the same interface through the service manager:
```go
txHash, err := serviceManager.SendTransactionBatch(ctx, chainID, address, txPayload)
```

## Troubleshooting

### Common Issues

1. **"Invalid API Key" Error**
   - Verify your `ALCHEMY_API_KEY` is correct
   - Ensure the API key has the necessary permissions

2. **"Chain not supported" Error**
   - Check if your target chain is supported by Alchemy
   - Verify the chain ID in your network configuration

3. **Gas estimation failures**
   - Ensure your smart account has sufficient balance
   - Check if Gas Manager policy is properly configured

### Health Check
```bash
# Test if Alchemy service is responding
curl -X POST https://api.g.alchemy.com/v2/YOUR_API_KEY \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}'
```

## Migration Checklist

- [ ] Alchemy API key configured
- [ ] Service manager tests passing
- [ ] Smart account creation working
- [ ] Transaction sending functional
- [ ] Event listening operational
- [ ] Performance meets requirements

## Rollback Plan

If issues arise:
1. Set `USE_ALCHEMY_SERVICE=false` in `.env`
2. Restart the application
3. Service will automatically switch back to Thirdweb Engine

## Next Steps

1. **Test on Testnet**: Create smart accounts and send test transactions
2. **Performance Testing**: Compare response times with Thirdweb
3. **Gas Optimization**: Configure Gas Manager policies
4. **Production Migration**: Gradually migrate traffic to Alchemy

---
**Status**: Ready for testing
**Last Updated**: 2025-10-08
