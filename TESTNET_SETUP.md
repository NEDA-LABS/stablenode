# Base Sepolia Testnet Setup Guide

## Prerequisites

### 1. Get Testnet ETH
You need Base Sepolia ETH for gas fees:

**Option A: Base Sepolia Faucet**
- Visit: https://www.alchemy.com/faucets/base-sepolia
- Connect wallet and request testnet ETH

**Option B: Bridge from Sepolia**
- Get Sepolia ETH from https://sepoliafaucet.com/
- Bridge to Base Sepolia at https://bridge.base.org/

### 2. Create Owner Wallet
You need an EOA (Externally Owned Account) to own the smart account:

```bash
# Generate a new wallet or use existing
# Save the private key securely - this will control your smart accounts
```

### 3. Configure Environment
Add to your `.env`:

```bash
# Base Sepolia Configuration
BASE_SEPOLIA_CHAIN_ID=84532
BASE_SEPOLIA_RPC_URL=https://base-sepolia.g.alchemy.com/v2/YOUR_API_KEY

# Smart Account Configuration
SMART_ACCOUNT_OWNER_ADDRESS=0x...  # Your EOA address
SMART_ACCOUNT_OWNER_PRIVATE_KEY=0x...  # Keep this secure!

# Alchemy Light Account Factory (v2.0.0) - LATEST
LIGHT_ACCOUNT_FACTORY=0x0000000000400CdFef5E2714E63d8040b700BC24
LIGHT_ACCOUNT_IMPLEMENTATION=0x8E8e658E22B12ada97B402fF0b044D6A325013C7
```

## Smart Account Creation Flow

### Step 1: Compute Smart Account Address
The address is deterministic based on:
- Factory contract address
- Owner address  
- Salt (usually 0 for first account)

### Step 2: Deploy Smart Account
Two options:

**Option A: Deploy via UserOperation**
- Send a UserOp with `initCode`
- Account is deployed when first transaction is sent

**Option B: Direct Factory Call**
- Call factory's `createAccount(owner, salt)` function
- Account is deployed immediately

### Step 3: Verify Deployment
Check on Base Sepolia explorer:
- https://sepolia.basescan.org/address/YOUR_SMART_ACCOUNT_ADDRESS

## Testing Checklist

- [ ] Owner wallet has Base Sepolia ETH
- [ ] Alchemy API key configured
- [ ] Base Sepolia RPC endpoint working
- [ ] Smart account address computed
- [ ] Account deployment transaction sent
- [ ] Account verified on explorer
- [ ] Test transaction sent from smart account

## Alchemy Light Account Details

**Factory Contract**: `0x0000000000400CdFef5E2714E63d8040b700BC24` (v2.0.0)
**Implementation**: `0x8E8e658E22B12ada97B402fF0b044D6A325013C7` (v2.0.0)
- Deployed on Base Sepolia
- Creates ERC-4337 compatible accounts
- Latest version with improved gas efficiency
- Enhanced security features

**Features**:
- Single owner
- Session keys support
- Batch transactions
- Gas sponsorship compatible

## Common Issues

### Issue 1: "Insufficient funds"
**Solution**: Ensure owner wallet has Base Sepolia ETH

### Issue 2: "Invalid signature"
**Solution**: Check that you're signing with the correct owner private key

### Issue 3: "Account already deployed"
**Solution**: Account can only be deployed once per owner+salt combination

## Next Steps After Deployment

1. **Fund the smart account** - Send some testnet ETH to it
2. **Test transactions** - Send a test transaction
3. **Test batching** - Send multiple transactions in one UserOp
4. **Test gas sponsorship** - Configure Gas Manager policy

## Useful Links

- **Base Sepolia Explorer**: https://sepolia.basescan.org/
- **Alchemy Dashboard**: https://dashboard.alchemy.com/
- **Base Sepolia Faucet**: https://www.alchemy.com/faucets/base-sepolia
- **ERC-4337 Docs**: https://eips.ethereum.org/EIPS/eip-4337
