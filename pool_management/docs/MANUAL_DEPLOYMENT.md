# Manual Receive Address Pool Deployment Guide

This guide shows you how to create, deploy, and manage a pool of pre-deployed receive addresses **without** using Alchemy's automated deployment.

## Overview

We've created three scripts to handle the complete lifecycle:

1. **`create_receive_pool`** - Generates addresses using same logic as receive addresses
2. **`deploy_pool_addresses`** - Deploys addresses to blockchain
3. **`mark_deployed`** - Updates database after deployment

---

## Step 1: Generate Addresses

### Build the Script

```bash
cd /home/commendatore/Desktop/NEDA/rails/aggregator

# Build the creation script
go build -o bin/create_receive_pool ./cmd/create_receive_pool
```

### Generate Addresses

```bash
# Generate 10 addresses for Base Sepolia
./bin/create_receive_pool \
  --count 10 \
  --chain-id 84532 \
  --network base-sepolia \
  --owner 0xYourOwnerAddress \
  --output pool_addresses_base_sepolia.json \
  --save-db

# Options:
#   --count: Number of addresses to create (default: 10)
#   --chain-id: Chain ID (default: 84532 = Base Sepolia)
#   --network: Network identifier (default: base-sepolia)
#   --owner: Owner address for smart accounts (default: 0xFb84...)
#   --output: Output JSON file (default: pool_addresses.json)
#   --save-db: Save to database (default: false)
```

### What This Does

- ✅ Generates unique salt for each address
- ✅ Computes CREATE2 address (same as Alchemy would)
- ✅ Creates initCode for deployment
- ✅ Saves to JSON file with all deployment info
- ✅ Optionally saves to database (status = 'unused', is_deployed = false)

### Output Format

The JSON file contains all info needed for deployment:

```json
[
  {
    "address": "0x1c0d91c545ee0ccfd19b6b1b738a67504fa86e4f",
    "salt": "0xabc123...",
    "owner_address": "0xFb84E5503bD20526f2579193411Dd0993d08077519b6f7",
    "init_code": "0x0000000000400CdFef5E2714E63d8040b700BC245fbfb9cf...",
    "factory_address": "0x0000000000400CdFef5E2714E63d8040b700BC24",
    "factory_data": "0x5fbfb9cf000000000000000000000000Fb84E5503bD2...",
    "network_identifier": "base-sepolia",
    "chain_id": 84532,
    "deploy_command": "cast send 0x00000... \"0x5fbfb...\" --rpc-url ..."
  }
]
```

---

## Step 2: Deploy Addresses

You have multiple options for deployment:

### Option A: Automated Deployment (Recommended)

Build and use the deployment script:

```bash
# Build the deployment script
go build -o bin/deploy_pool_addresses ./cmd/deploy_pool_addresses

# Deploy all addresses
./bin/deploy_pool_addresses \
  --input pool_addresses_base_sepolia.json \
  --private-key YOUR_PRIVATE_KEY \
  --rpc-url https://base-sepolia.g.alchemy.com/v2/YOUR_API_KEY \
  --output deployment_results.json

# Options:
#   --input: Input JSON file with addresses
#   --private-key: Private key for deployment (without 0x)
#   --rpc-url: RPC URL for the network
#   --output: Output file for results (default: deployment_results.json)
#   --gas-price: Gas price in gwei (0 = auto)
#   --max-fee: Max fee per gas in gwei (EIP-1559, 0 = auto)
#   --max-priority-fee: Max priority fee (EIP-1559, 0 = auto)
#   --dry-run: Simulate without sending transactions
#   --start: Start index (for resuming, default: 0)
#   --end: End index (for subset, default: all)
```

### Deploy in Batches (If Needed)

```bash
# Deploy first 5 addresses
./bin/deploy_pool_addresses \
  --input pool_addresses_base_sepolia.json \
  --private-key $PRIVATE_KEY \
  --rpc-url $RPC_URL \
  --start 0 \
  --end 5 \
  --output deployment_results_batch1.json

# Deploy next 5 addresses
./bin/deploy_pool_addresses \
  --input pool_addresses_base_sepolia.json \
  --private-key $PRIVATE_KEY \
  --rpc-url $RPC_URL \
  --start 5 \
  --end 10 \
  --output deployment_results_batch2.json
```

### Dry Run First

```bash
# Test without actually deploying
./bin/deploy_pool_addresses \
  --input pool_addresses_base_sepolia.json \
  --private-key $PRIVATE_KEY \
  --rpc-url $RPC_URL \
  --dry-run
```

### Option B: Manual Deployment with Cast (Foundry)

```bash
# Install Foundry if not already installed
curl -L https://foundry.paradigm.xyz | bash
foundryup

# Deploy each address manually using cast
# The deploy_command is in the JSON file

# Example for first address:
cast send 0x0000000000400CdFef5E2714E63d8040b700BC24 \
  "0x5fbfb9cf000000000000000000000000Fb84E5503bD20526f2579193411Dd0993d080775..." \
  --rpc-url $RPC_URL \
  --private-key $PRIVATE_KEY
```

### Option C: Manual Deployment via Tenderly

1. Go to https://dashboard.tenderly.co/
2. Navigate to **Simulator**
3. For each address from JSON:
   - **From**: Your deployer address
   - **To**: `0x0000000000400CdFef5E2714E63d8040b700BC24` (Factory)
   - **Input Data**: Use the `factory_data` from JSON
   - Click **Simulate** to test
   - Click **Execute** to deploy

### Option D: Using Alchemy Account Kit Dashboard

1. Go to https://dashboard.alchemy.com/accounts
2. Use the dashboard to deploy accounts
3. Note the transaction hashes

---

## Step 3: Mark Addresses as Deployed

After deployment, update the database:

### Build the Script

```bash
# Build the mark deployed script
go build -o bin/mark_deployed ./cmd/mark_deployed
```

### Update Database

```bash
# Update from deployment results
./bin/mark_deployed \
  --input deployment_results.json \
  --status pool_ready

# Options:
#   --input: Deployment results JSON file
#   --status: Status to set (default: pool_ready)
#     Options: pool_ready, unused
#   --dry-run: Show what would be updated without changes
```

### Dry Run First

```bash
# Check what will be updated
./bin/mark_deployed \
  --input deployment_results.json \
  --dry-run
```

### What This Does

For each successful deployment:
- ✅ Sets `is_deployed = true`
- ✅ Sets `status = 'pool_ready'`
- ✅ Sets `deployment_tx_hash`
- ✅ Sets `deployment_block`
- ✅ Sets `deployed_at = NOW()`
- ✅ Verifies pool status after update

---

## Complete Workflow Example

### Full End-to-End Example for Base Sepolia

```bash
# Step 1: Generate 10 addresses
./bin/create_receive_pool \
  --count 10 \
  --chain-id 84532 \
  --network base-sepolia \
  --output pool_base_sepolia.json \
  --save-db

# Step 2: Review generated addresses
cat pool_base_sepolia.json | jq '.[].address'

# Step 3: Deploy with dry run first
./bin/deploy_pool_addresses \
  --input pool_base_sepolia.json \
  --private-key $PRIVATE_KEY \
  --rpc-url $BASE_SEPOLIA_RPC \
  --dry-run

# Step 4: Deploy for real
./bin/deploy_pool_addresses \
  --input pool_base_sepolia.json \
  --private-key $PRIVATE_KEY \
  --rpc-url $BASE_SEPOLIA_RPC \
  --output deployment_results_base_sepolia.json

# Step 5: Verify deployments on block explorer
# Check addresses in deployment_results_base_sepolia.json

# Step 6: Update database (dry run)
./bin/mark_deployed \
  --input deployment_results_base_sepolia.json \
  --dry-run

# Step 7: Update database for real
./bin/mark_deployed \
  --input deployment_results_base_sepolia.json \
  --status pool_ready

# Step 8: Verify pool status
# The script will show pool statistics
```

---

## Multi-Network Deployment

Deploy pools for multiple networks:

```bash
#!/bin/bash
# deploy_all_networks.sh

NETWORKS=(
  "base-sepolia:84532:$BASE_SEPOLIA_RPC"
  "base-mainnet:8453:$BASE_MAINNET_RPC"
  "ethereum-sepolia:11155111:$ETH_SEPOLIA_RPC"
)

for network_config in "${NETWORKS[@]}"; do
  IFS=':' read -r network chain_id rpc_url <<< "$network_config"
  
  echo "Deploying pool for $network (Chain ID: $chain_id)"
  
  # Generate addresses
  ./bin/create_receive_pool \
    --count 10 \
    --chain-id $chain_id \
    --network $network \
    --output pool_${network}.json \
    --save-db
  
  # Deploy
  ./bin/deploy_pool_addresses \
    --input pool_${network}.json \
    --private-key $PRIVATE_KEY \
    --rpc-url $rpc_url \
    --output deployment_${network}.json
  
  # Mark deployed
  ./bin/mark_deployed \
    --input deployment_${network}.json \
    --status pool_ready
  
  echo "✓ Pool deployed for $network"
  echo ""
done
```

---

## Verification

### Check On-Chain

```bash
# Check if address is deployed (has code)
cast code 0xYourGeneratedAddress --rpc-url $RPC_URL

# Should return bytecode if deployed
# Returns 0x if not deployed
```

### Check Database

```sql
-- Check pool status
SELECT 
    network_identifier,
    status,
    is_deployed,
    COUNT(*) as count
FROM receive_addresses
WHERE is_deployed = true
GROUP BY network_identifier, status, is_deployed;

-- Check specific address
SELECT * FROM receive_addresses 
WHERE address = '0x1c0d91c545ee0ccfd19b6b1b738a67504fa86e4f';

-- Addresses ready for use
SELECT COUNT(*) FROM receive_addresses
WHERE status = 'pool_ready' AND is_deployed = true;
```

---

## Troubleshooting

### Issue: "Failed to deploy"

**Check:**
- Sufficient balance in deployer account
- Correct RPC URL
- Network connectivity
- Gas price settings

**Solution:**
```bash
# Check deployer balance
cast balance $DEPLOYER_ADDRESS --rpc-url $RPC_URL

# Try with higher gas price
./bin/deploy_pool_addresses \
  --input pool.json \
  --private-key $PRIVATE_KEY \
  --rpc-url $RPC_URL \
  --gas-price 50  # 50 gwei
```

### Issue: "Address not found in database"

**Cause:** Address wasn't saved to DB during creation

**Solution:**
```bash
# Re-create with --save-db flag
./bin/create_receive_pool \
  --count 10 \
  --save-db \
  --output pool.json
```

Or manually insert:
```sql
INSERT INTO receive_addresses (
    address, 
    is_deployed, 
    status, 
    chain_id, 
    network_identifier,
    created_at,
    updated_at
) VALUES (
    '0xYourAddress',
    false,
    'unused',
    84532,
    'base-sepolia',
    NOW(),
    NOW()
);
```

### Issue: "Nonce too low"

**Cause:** Multiple deployments in progress or previous transaction pending

**Solution:**
```bash
# Wait a few seconds between deployments (built into script)
# Or deploy in smaller batches

./bin/deploy_pool_addresses \
  --input pool.json \
  --private-key $PRIVATE_KEY \
  --rpc-url $RPC_URL \
  --start 0 \
  --end 5  # Deploy 5 at a time
```

### Issue: Transaction reverts

**Check:**
1. Address already deployed?
   ```bash
   cast code 0xAddress --rpc-url $RPC_URL
   ```

2. Factory contract correct?
   ```bash
   # Should be: 0x0000000000400CdFef5E2714E63d8040b700BC24
   ```

3. initCode valid?
   - Check factory_data in JSON is not truncated

---

## Cost Estimation

### Gas Usage

- **Deployment per address**: ~100k-300k gas
- **Example costs** (at 10 gwei gas price):
  - 100k gas = 0.001 ETH (~$2)
  - 300k gas = 0.003 ETH (~$6)
  
### For 10 addresses

- **Total gas**: ~1M-3M gas
- **Cost**: ~$20-60 (at 10 gwei, $2000 ETH)

### Batch Deployment

```bash
# Estimate cost before deploying
./bin/deploy_pool_addresses \
  --input pool.json \
  --private-key $PRIVATE_KEY \
  --rpc-url $RPC_URL \
  --dry-run

# Check gas estimates in output
```

---

## Best Practices

1. **Always dry-run first**
   ```bash
   --dry-run
   ```

2. **Deploy in batches** (5-10 at a time)
   ```bash
   --start 0 --end 5
   ```

3. **Save results**
   ```bash
   --output deployment_results_$(date +%Y%m%d_%H%M%S).json
   ```

4. **Backup JSON files**
   ```bash
   cp pool_addresses.json pool_addresses_backup_$(date +%Y%m%d).json
   ```

5. **Verify on block explorer** after deployment

6. **Test with 1 address first**
   ```bash
   --count 1
   ```

7. **Use environment variables for secrets**
   ```bash
   export PRIVATE_KEY="your_key_here"
   export RPC_URL="your_rpc_here"
   ```

---

## Monitoring Pool Health

### Check Pool Size

```bash
# Query database
psql $DATABASE_URL -c "
SELECT 
    network_identifier,
    COUNT(*) FILTER (WHERE status = 'pool_ready') as ready,
    COUNT(*) FILTER (WHERE status = 'pool_assigned') as assigned,
    COUNT(*) FILTER (WHERE status = 'pool_processing') as processing
FROM receive_addresses
WHERE is_deployed = true
GROUP BY network_identifier;
"
```

### Replenish Pool

If pool size drops below threshold:

```bash
# Check current size
READY_COUNT=$(psql $DATABASE_URL -t -c "
SELECT COUNT(*) FROM receive_addresses 
WHERE status = 'pool_ready' AND is_deployed = true;
")

# If below 5, create more
if [ $READY_COUNT -lt 5 ]; then
  echo "Pool low, creating 5 more addresses"
  ./bin/create_receive_pool --count 5 --save-db
  # Deploy and mark them...
fi
```

---

## Summary

### Quick Reference

| Task | Command |
|------|---------|
| Generate addresses | `./bin/create_receive_pool --count 10 --save-db` |
| Deploy addresses | `./bin/deploy_pool_addresses --input pool.json --private-key $KEY --rpc-url $RPC` |
| Mark deployed | `./bin/mark_deployed --input deployment_results.json` |
| Check pool | `psql $DB -c "SELECT * FROM receive_addresses WHERE status='pool_ready'"` |
| Verify on-chain | `cast code 0xAddress --rpc-url $RPC` |

### Files Generated

- `pool_addresses.json` - Generated addresses with deployment info
- `deployment_results.json` - Deployment transaction results
- Database entries with pool status

### Next Steps

After deployment:
1. Update order creation to use pool (see `RECEIVE_POOL_QUICKSTART.md`)
2. Add recycling after order completion
3. Set up pool monitoring
4. Add auto-replenishment task
