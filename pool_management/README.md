# Receive Address Pool Management

This directory contains everything needed to create, deploy, and manage a pool of pre-deployed receive addresses.

## 📁 Directory Structure

```
pool_management/
├── cmd/                          # Command-line tools
│   ├── create_receive_pool/      # Generate addresses
│   ├── deploy_pool_addresses/    # Deploy to blockchain
│   └── mark_deployed/            # Update database
├── docs/                         # Documentation
│   ├── QUICK_REFERENCE.md        # Quick command reference ⭐ START HERE
│   ├── QUICKSTART.md             # Fast implementation guide
│   ├── MANUAL_DEPLOYMENT.md      # Complete deployment guide
│   ├── IMPLEMENTATION_GUIDE.md   # Full implementation details
│   └── ARCHITECTURE.md           # System architecture diagrams
├── migrations/                   # Database migrations
│   └── add_receive_address_pool.sql
├── scripts/                      # Helper scripts (future use)
├── Makefile                      # Easy-to-use commands
└── README.md                     # This file
```

## 🚀 Quick Start

### 1. Build Tools

```bash
cd pool_management
make build
```

### 2. Create & Deploy Pool

```bash
# Option A: All in one command
make full-deploy \
  NETWORK=base-sepolia \
  COUNT=10 \
  RPC_URL=$BASE_SEPOLIA_RPC \
  PRIVATE_KEY=$DEPLOYER_PRIVATE_KEY

# Option B: Step by step
make create NETWORK=base-sepolia COUNT=10
make deploy POOL_FILE_INPUT=pool_*.json RPC_URL=$RPC PRIVATE_KEY=$KEY
make mark-deployed DEPLOY_RESULTS_INPUT=deployment_*.json
```

### 3. Verify

```bash
make verify NETWORK=base-sepolia
```

## 📚 Documentation

| Document | Purpose | When to Read |
|----------|---------|--------------|
| **[QUICK_REFERENCE.md](docs/QUICK_REFERENCE.md)** | Command cheat sheet | Quick lookups ⭐ |
| **[QUICKSTART.md](docs/QUICKSTART.md)** | 1-2 hour implementation | Getting started |
| **[MANUAL_DEPLOYMENT.md](docs/MANUAL_DEPLOYMENT.md)** | Complete deployment guide | Step-by-step deployment |
| **[IMPLEMENTATION_GUIDE.md](docs/IMPLEMENTATION_GUIDE.md)** | Full system design | Deep dive & planning |
| **[ARCHITECTURE.md](docs/ARCHITECTURE.md)** | System diagrams | Understanding flow |

## 🛠️ Tools

### create_receive_pool

Generates receive addresses using CREATE2 (same logic as your existing system).

```bash
./bin/create_receive_pool \
  --count 10 \
  --chain-id 84532 \
  --network base-sepolia \
  --output pool.json \
  --save-db
```

**Output:** JSON file with addresses, salt, initCode, deployment info

### deploy_pool_addresses

Deploys addresses to blockchain using your private key.

```bash
./bin/deploy_pool_addresses \
  --input pool.json \
  --private-key $PRIVATE_KEY \
  --rpc-url $RPC_URL \
  --output deployment_results.json
```

**Features:**
- Batch deployment
- Dry-run mode
- Resume capability
- Gas estimation
- EIP-1559 support

### mark_deployed

Updates database after successful deployment.

```bash
./bin/mark_deployed \
  --input deployment_results.json \
  --status pool_ready
```

**Updates:**
- `is_deployed = true`
- `status = 'pool_ready'`
- `deployment_tx_hash`
- `deployment_block`
- `deployed_at`

## 📋 Common Tasks

### Deploy Pool for Production

```bash
# Base Sepolia (testnet)
make base-sepolia

# Base Mainnet (production)
make base-mainnet

# Ethereum Sepolia
make ethereum-sepolia
```

### Check Pool Status

```bash
# Database check
make verify NETWORK=base-sepolia

# On-chain verification
make verify-address ADDRESS=0x123... RPC_URL=$RPC
```

### Resume Failed Deployment

```bash
# Check what failed
jq '.[] | select(.success == false)' deployment_results.json

# Deploy specific range
make deploy \
  POOL_FILE_INPUT=pool.json \
  START=5 END=10 \
  RPC_URL=$RPC PRIVATE_KEY=$KEY
```

### Clean Up

```bash
# Remove generated files
make clean

# Remove built binaries
make clean-bin
```

## 🔧 Configuration

### Environment Variables

Set these in your shell or `.env`:

```bash
export DEPLOYER_PRIVATE_KEY="your_private_key_here"
export BASE_SEPOLIA_RPC="https://base-sepolia.g.alchemy.com/v2/..."
export BASE_MAINNET_RPC="https://base.g.alchemy.com/v2/..."
export DATABASE_URL="postgresql://..."
```

### Makefile Variables

Override defaults:

```bash
make create \
  NETWORK=base-sepolia \    # Network name
  CHAIN_ID=84532 \          # Chain ID
  COUNT=10 \                # Number of addresses
  OWNER=0x... \             # Owner address
  RPC_URL=$RPC \            # RPC endpoint
  PRIVATE_KEY=$KEY          # Deployer key
```

## 🗄️ Database Migration

Before using the pool system, run the migration:

```bash
# Using psql
psql $DATABASE_URL -f migrations/add_receive_address_pool.sql

# Or using Atlas (if using ent)
atlas migrate apply --url $DATABASE_URL
```

**Adds:**
- `is_deployed` field
- `deployment_block` field
- `deployment_tx_hash` field
- `deployed_at` field
- `network_identifier` field
- `chain_id` field
- `assigned_at` field
- `recycled_at` field
- `times_used` field
- New status values: `pool_ready`, `pool_assigned`, `pool_processing`, `pool_completed`
- Indexes for efficient queries

## 🎯 Integration with Main App

After deploying your pool, integrate with your payment order system:

### 1. Update Order Creation

In `controllers/sender/sender.go`:

```go
// OLD: Create new address each time
// receiveAddress, err := ctrl.receiveAddressService.CreateSmartAddress(...)

// NEW: Get from pool
poolService := services.NewReceiveAddressPoolService()
receiveAddress, err := poolService.GetAvailableAddress(ctx, chainID, networkIdentifier)
```

### 2. Add Recycling

After order completion:

```go
if order.Edges.ReceiveAddress != nil {
    poolService.RecycleAddress(ctx, order.Edges.ReceiveAddress.ID)
}
```

### 3. Background Maintenance

Add to `tasks/tasks.go`:

```go
// Run every 10 minutes
c.AddFunc("*/10 * * * *", func() {
    poolService.MaintainPoolSize(ctx, chainID, networkIdentifier)
})
```

## 📊 Monitoring

### Pool Health Checks

```sql
-- Available addresses
SELECT COUNT(*) FROM receive_addresses 
WHERE status = 'pool_ready' AND is_deployed = true;

-- Usage statistics
SELECT 
    network_identifier,
    status,
    COUNT(*) as count,
    AVG(times_used) as avg_reuse
FROM receive_addresses
WHERE is_deployed = true
GROUP BY network_identifier, status;
```

### Alerts to Set Up

- **Pool Low**: < 2 available addresses
- **Pool Exhausted**: 0 available addresses
- **High Reuse**: Any address used > 50 times
- **Stuck Assignment**: Address assigned > 1 hour

## 💰 Cost Estimates

| Network | Gas per Deploy | Cost per Address | 10 Addresses |
|---------|---------------|------------------|--------------|
| Base Sepolia | 100-300k | ~$0 (testnet) | ~$0 |
| Base Mainnet | 100-300k | $2-6 @ 10 gwei | $20-60 |
| Ethereum Mainnet | 100-300k | $5-15 @ 50 gwei | $50-150 |

**Note:** With pool reuse (100x per address), cost per order is negligible!

## 🐛 Troubleshooting

### Build Issues

```bash
# Missing dependencies
go mod download
go mod tidy

# Rebuild from scratch
make clean-bin
make build
```

### Deployment Failures

```bash
# Check balance
cast balance $DEPLOYER_ADDRESS --rpc-url $RPC_URL

# Verify gas settings
make deploy-dry-run POOL_FILE_INPUT=pool.json

# Deploy in smaller batches
make deploy START=0 END=5 ...
```

### Database Issues

```bash
# Check if migration ran
psql $DATABASE_URL -c "\d receive_addresses"

# Manually run migration
psql $DATABASE_URL -f migrations/add_receive_address_pool.sql
```

## 📞 Getting Help

1. Check **[QUICK_REFERENCE.md](docs/QUICK_REFERENCE.md)** for common commands
2. Read **[MANUAL_DEPLOYMENT.md](docs/MANUAL_DEPLOYMENT.md)** for detailed steps
3. See **[ARCHITECTURE.md](docs/ARCHITECTURE.md)** for system design
4. Run `make help` for available commands

## 🎓 Learning Path

1. **Start here**: Read [QUICK_REFERENCE.md](docs/QUICK_REFERENCE.md)
2. **Quick test**: Follow [QUICKSTART.md](docs/QUICKSTART.md) (1-2 hours)
3. **Production deploy**: Use [MANUAL_DEPLOYMENT.md](docs/MANUAL_DEPLOYMENT.md)
4. **Deep understanding**: Study [IMPLEMENTATION_GUIDE.md](docs/IMPLEMENTATION_GUIDE.md)
5. **System design**: Review [ARCHITECTURE.md](docs/ARCHITECTURE.md)

## 🔄 Version History

- **v1.0** - Initial release
  - Address generation
  - Manual deployment
  - Database integration
  - Pool management

## 📝 License

Same as parent project.

---

**Questions?** Check the docs or run `make help`
