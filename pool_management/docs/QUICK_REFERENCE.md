# Receive Address Pool - Quick Reference Card

## 🚀 Quick Start (3 Commands)

```bash
# 1. Create addresses
make -f Makefile.pool create NETWORK=base-sepolia COUNT=10

# 2. Deploy them
make -f Makefile.pool deploy \
  POOL_FILE_INPUT=pool_base-sepolia_*.json \
  RPC_URL=$BASE_SEPOLIA_RPC \
  PRIVATE_KEY=$DEPLOYER_PRIVATE_KEY

# 3. Mark as deployed
make -f Makefile.pool mark-deployed \
  DEPLOY_RESULTS_INPUT=deployment_base-sepolia_*.json
```

## 📋 One-Line Full Deploy

```bash
make -f Makefile.pool full-deploy \
  NETWORK=base-sepolia \
  COUNT=10 \
  RPC_URL=$BASE_SEPOLIA_RPC \
  PRIVATE_KEY=$DEPLOYER_PRIVATE_KEY
```

## 🛠️ Manual Commands

### Create Addresses
```bash
./bin/create_receive_pool \
  --count 10 \
  --chain-id 84532 \
  --network base-sepolia \
  --save-db \
  --output pool.json
```

### Deploy Addresses
```bash
./bin/deploy_pool_addresses \
  --input pool.json \
  --private-key $PRIVATE_KEY \
  --rpc-url $RPC_URL \
  --output deployment_results.json
```

### Mark Deployed
```bash
./bin/mark_deployed \
  --input deployment_results.json \
  --status pool_ready
```

## 🔍 Verification

### Check Database
```sql
-- Pool status
SELECT status, COUNT(*) 
FROM receive_addresses 
WHERE is_deployed = true 
GROUP BY status;

-- Available addresses
SELECT COUNT(*) 
FROM receive_addresses 
WHERE status = 'pool_ready' AND is_deployed = true;
```

### Check On-Chain
```bash
# Verify address is deployed
cast code 0xYourAddress --rpc-url $RPC_URL

# Should return bytecode if deployed
# Returns 0x if not deployed
```

### Using Makefile
```bash
# Verify pool status
make -f Makefile.pool verify NETWORK=base-sepolia

# Verify specific address
make -f Makefile.pool verify-address \
  ADDRESS=0x123... \
  RPC_URL=$RPC_URL
```

## 📁 File Structure

```
pool_base-sepolia_20251013_150000.json     # Generated addresses
deployment_base-sepolia_20251013_150100.json  # Deployment results
```

## 🔑 Environment Variables

```bash
# Required for deployment
export DEPLOYER_PRIVATE_KEY="your_private_key"
export BASE_SEPOLIA_RPC="https://base-sepolia.g.alchemy.com/v2/..."
export DATABASE_URL="postgresql://..."

# Optional
export OWNER_ADDRESS="0xYourOwnerAddress"
```

## ⚙️ Common Options

### Create Options
- `--count`: Number of addresses (default: 10)
- `--chain-id`: Chain ID (default: 84532)
- `--network`: Network name (default: base-sepolia)
- `--owner`: Owner address
- `--save-db`: Save to database
- `--output`: Output JSON file

### Deploy Options
- `--input`: Input JSON file
- `--private-key`: Deployer private key
- `--rpc-url`: RPC endpoint
- `--output`: Results JSON file
- `--dry-run`: Simulate only
- `--start`: Start index
- `--end`: End index
- `--gas-price`: Gas price in gwei
- `--max-fee`: Max fee (EIP-1559)
- `--max-priority-fee`: Priority fee (EIP-1559)

### Mark Deployed Options
- `--input`: Deployment results file
- `--status`: Status to set (pool_ready/unused)
- `--dry-run`: Preview changes

## 🎯 Common Tasks

### Deploy for Multiple Networks
```bash
# Base Sepolia
make -f Makefile.pool base-sepolia

# Base Mainnet
make -f Makefile.pool base-mainnet

# Ethereum Sepolia
make -f Makefile.pool ethereum-sepolia
```

### Deploy in Batches
```bash
# Deploy first 5
./bin/deploy_pool_addresses \
  --input pool.json \
  --private-key $KEY \
  --rpc-url $RPC \
  --start 0 --end 5

# Deploy next 5
./bin/deploy_pool_addresses \
  --input pool.json \
  --private-key $KEY \
  --rpc-url $RPC \
  --start 5 --end 10
```

### Resume Failed Deployment
```bash
# Check which addresses failed
jq '.[] | select(.success == false)' deployment_results.json

# Deploy only failed ones (manually extract to new file)
# Or re-deploy with --start and --end
```

## 🐛 Troubleshooting

### "Address not in database"
```bash
# Re-create with --save-db
./bin/create_receive_pool --count 10 --save-db
```

### "Insufficient balance"
```bash
# Check balance
cast balance $DEPLOYER_ADDRESS --rpc-url $RPC_URL
```

### "Nonce too low"
```bash
# Deploy in smaller batches
--start 0 --end 5
```

### "Transaction reverted"
```bash
# Check if already deployed
cast code 0xAddress --rpc-url $RPC_URL

# Try dry-run first
./bin/deploy_pool_addresses --input pool.json --dry-run
```

## 💡 Tips

1. **Always dry-run first**
   ```bash
   --dry-run
   ```

2. **Save results with timestamps**
   ```bash
   deployment_$(date +%Y%m%d_%H%M%S).json
   ```

3. **Deploy 5-10 at a time**
   ```bash
   --start 0 --end 5
   ```

4. **Check pool health regularly**
   ```bash
   make -f Makefile.pool verify
   ```

5. **Keep backups of JSON files**
   ```bash
   cp pool.json pool_backup.json
   ```

## 📊 Cost Estimates

| Network | Gas/Deploy | Cost per Address | 10 Addresses |
|---------|-----------|------------------|--------------|
| Base Sepolia | 100-300k | ~$0 (testnet) | ~$0 |
| Base Mainnet | 100-300k | $2-6 (10 gwei) | $20-60 |
| Ethereum | 100-300k | $5-15 (50 gwei) | $50-150 |

## 📞 Help

```bash
# Show all Makefile commands
make -f Makefile.pool help

# Show command help
./bin/create_receive_pool --help
./bin/deploy_pool_addresses --help
./bin/mark_deployed --help
```

## 📝 Full Documentation

- **Complete Guide**: `MANUAL_DEPLOYMENT_GUIDE.md`
- **Implementation Plan**: `RECEIVE_ADDRESS_POOL_IMPLEMENTATION.md`
- **Quick Start**: `RECEIVE_POOL_QUICKSTART.md`
- **Architecture**: `RECEIVE_POOL_ARCHITECTURE.md`
