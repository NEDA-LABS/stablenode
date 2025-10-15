# Pool Management Setup Guide

## 📁 Directory Structure

```
pool_management/
├── bin/                              # Built binaries (created by make build)
│   ├── create_receive_pool
│   ├── deploy_pool_addresses
│   └── mark_deployed
├── cmd/                              # Source code for tools
│   ├── create_receive_pool/
│   │   └── main.go
│   ├── deploy_pool_addresses/
│   │   └── main.go
│   └── mark_deployed/
│       └── main.go
├── docs/                             # Documentation
│   ├── QUICK_REFERENCE.md           # ⭐ Start here for commands
│   ├── QUICKSTART.md                # Fast setup guide
│   ├── MANUAL_DEPLOYMENT.md         # Complete deployment guide
│   ├── IMPLEMENTATION_GUIDE.md      # Full system design
│   └── ARCHITECTURE.md              # System diagrams
├── migrations/                       # Database migrations
│   └── add_receive_address_pool.sql # Schema changes
├── scripts/                          # Helper scripts (future)
├── Makefile                          # Easy-to-use commands
├── README.md                         # Main documentation
└── SETUP.md                          # This file
```

## 🚀 Quick Setup (5 minutes)

### Step 1: Run Database Migration

**IMPORTANT:** Run this FIRST before building the tools!

```bash
cd /home/commendatore/Desktop/NEDA/rails/aggregator

# Run the migration
psql $DATABASE_URL -f pool_management/migrations/add_receive_address_pool.sql

# Verify migration
psql $DATABASE_URL -c "\d receive_addresses"
```

This adds the required fields:
- `is_deployed`, `deployment_block`, `deployment_tx_hash`, `deployed_at`
- `network_identifier`, `chain_id`
- `assigned_at`, `recycled_at`, `times_used`
- New status values for pool management

### Step 2: Regenerate Ent Code (if using ent)

If you're using ent for database management:

```bash
cd /home/commendatore/Desktop/NEDA/rails/aggregator

# Update the schema file first
# Edit ent/schema/receiveaddress.go with new fields from docs/IMPLEMENTATION_GUIDE.md

# Then regenerate
go generate ./ent
```

### Step 3: Build Tools

```bash
cd pool_management

# Build all tools
make build

# Verify binaries created
ls -lh bin/
```

### Step 4: Test

```bash
# Create 1 test address (dry run - no DB save)
./bin/create_receive_pool \
  --count 1 \
  --chain-id 84532 \
  --network base-sepolia \
  --output test_pool.json

# Check output
cat test_pool.json | jq '.'
```

## 📝 Note About Lint Errors

You may see lint errors in the IDE for these files:
- `cmd/create_receive_pool/main.go`
- `cmd/deploy_pool_addresses/main.go`
- `cmd/mark_deployed/main.go`

**These are expected!** The errors occur because:

1. **Schema not updated yet** - The new fields (`is_deployed`, etc.) don't exist in your current schema
2. **Import ordering** - Minor style issues that don't affect functionality

**These will be resolved when you:**
1. Run the database migration (Step 1 above)
2. Update your ent schema and regenerate (Step 2 above)
3. Rebuild the tools (Step 3 above)

You can safely ignore these errors until you've completed the setup steps.

## 🔍 Verification

### Check Database Schema

After migration, verify the new fields exist:

```sql
SELECT column_name, data_type, is_nullable 
FROM information_schema.columns 
WHERE table_name = 'receive_addresses'
ORDER BY ordinal_position;
```

Should show new columns:
- `is_deployed` (boolean)
- `deployment_block` (bigint)
- `deployment_tx_hash` (varchar)
- `deployed_at` (timestamp)
- `network_identifier` (varchar)
- `chain_id` (bigint)
- etc.

### Test Build

```bash
cd pool_management

# Clean build
make clean-bin
make build

# Should complete without errors
# Binaries should exist in bin/
```

### Test Create (No DB)

```bash
# Generate 1 address without saving to DB
./bin/create_receive_pool \
  --count 1 \
  --output test.json

# Should create test.json with address details
cat test.json | jq '.[]'
```

## 🎯 Next Steps

After setup is complete:

1. **Read** [docs/QUICK_REFERENCE.md](docs/QUICK_REFERENCE.md) for commands
2. **Follow** [docs/QUICKSTART.md](docs/QUICKSTART.md) for your first deployment
3. **Deploy** a test pool (5 addresses) to Base Sepolia
4. **Integrate** with your payment order system

## 🐛 Troubleshooting Setup

### Build Errors

```bash
# Missing dependencies
cd /home/commendatore/Desktop/NEDA/rails/aggregator
go mod download
go mod tidy

# Try build again
cd pool_management
make build
```

### Migration Errors

```bash
# Check if already run
psql $DATABASE_URL -c "
SELECT column_name FROM information_schema.columns 
WHERE table_name = 'receive_addresses' 
AND column_name = 'is_deployed';
"

# If exists, migration already applied
# If not exists, run migration again
```

### Schema Errors

```bash
# Regenerate ent code
cd /home/commendatore/Desktop/NEDA/rails/aggregator
go generate ./ent

# Check for errors in ent/schema/receiveaddress.go
```

## 📋 Setup Checklist

- [ ] Database migration run successfully
- [ ] New columns exist in `receive_addresses` table
- [ ] Ent schema updated (if using ent)
- [ ] Ent code regenerated (if using ent)
- [ ] Tools built successfully (`make build`)
- [ ] Binaries exist in `bin/` directory
- [ ] Test address generation works
- [ ] No build errors

Once all items are checked, you're ready to proceed with deployment!

## 📚 Where to Go Next

| Task | Document |
|------|----------|
| Learn commands | [docs/QUICK_REFERENCE.md](docs/QUICK_REFERENCE.md) |
| Deploy test pool | [docs/QUICKSTART.md](docs/QUICKSTART.md) |
| Production deploy | [docs/MANUAL_DEPLOYMENT.md](docs/MANUAL_DEPLOYMENT.md) |
| Understand system | [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) |
| Full details | [docs/IMPLEMENTATION_GUIDE.md](docs/IMPLEMENTATION_GUIDE.md) |
