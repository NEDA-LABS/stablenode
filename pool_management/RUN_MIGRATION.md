# Run Pool Management Migration - Step by Step

## Quick Start (Copy & Paste)

### Option 1: Simple Version (No Docker Required) ⭐ RECOMMENDED

```bash
# 1. Navigate to project root
cd /home/commendatore/Desktop/NEDA/rails/aggregator

# 2. Export DATABASE_URL from your .env file
source pool_management/scripts/export_db_url.sh

# 3. Verify connection
psql "$DATABASE_URL" -c '\conninfo'

# 4. Run the simple migration script
./pool_management/scripts/apply_pool_migration_simple.sh
```

### Option 2: Full Version (Creates Migration Files)

```bash
# Same steps 1-3, then:
./pool_management/scripts/apply_pool_migration.sh
```

**Use Option 1 if:**
- You don't have Docker installed
- You just want to apply the changes quickly
- You're not tracking migrations in version control

**Use Option 2 if:**
- You want to create migration files for version control
- You need to review SQL before applying
- You're following the full Atlas workflow

That's it! The script will handle everything.

---

## Detailed Steps

### Step 1: Export DATABASE_URL

Your database credentials are in `.env` file. We'll construct the DATABASE_URL from them:

```bash
cd /home/commendatore/Desktop/NEDA/rails/aggregator

# Source the export script
source pool_management/scripts/export_db_url.sh
```

**What this does:**
- Reads `DB_NAME`, `DB_USER`, `DB_PASSWORD`, `DB_HOST`, `DB_PORT`, `SSL_MODE` from `.env`
- Constructs: `postgresql://user:password@host:port/dbname?sslmode=require`
- Exports as `DATABASE_URL` environment variable

**Expected output:**
```
✓ DATABASE_URL exported successfully

Database connection details:
  Host: db.xxxxxxxxxxxxx.supabase.co
  Port: 5432
  Database: postgres
  User: postgres.xxxxxxxxxxxxx
  SSL Mode: require

DATABASE_URL is now available in your shell session.
```

### Step 2: Verify Connection

Test that you can connect to the database:

```bash
psql "$DATABASE_URL" -c '\conninfo'
```

**Expected output:**
```
You are connected to database "postgres" as user "postgres.xxxxxxxxxxxxx" on host "db.xxxxxxxxxxxxx.supabase.co" (address "x.x.x.x") at port "5432".
SSL connection (protocol: TLSv1.3, cipher: TLS_AES_256_GCM_SHA384, compression: off)
```

If this fails, check your `.env` file has correct database credentials.

### Step 3: Run Migration (Option A - Automated)

Run the automated migration script:

```bash
./pool_management/scripts/apply_pool_migration.sh
```

**What the script does:**

1. **Checks DATABASE_URL** is set
2. **Generates ent code** from schema (`go generate ./ent`)
3. **Creates Atlas migration** (diff between schema and database)
4. **Shows you the changes** (preview of SQL)
5. **Asks for confirmation** (you can review before applying)
6. **Applies migration** using Atlas
7. **Verifies changes** (checks new columns exist)

**Expected flow:**

```
=========================================
Pool Management Migration with Atlas
=========================================

✓ DATABASE_URL is set

Step 1: Generating ent code...
------------------------------
✓ Ent code generated successfully

Step 2: Creating Atlas migration diff...
----------------------------------------
✓ Migration diff created

Step 3: Review migration...
---------------------------
Latest migration: ent/migrate/migrations/20251013192000_add_pool_management.sql

Preview (first 50 lines):
-- Add new columns to receive_addresses
ALTER TABLE receive_addresses 
  ADD COLUMN is_deployed BOOLEAN DEFAULT false,
  ADD COLUMN deployment_block BIGINT,
  ...

=========================================
Apply this migration to database? (y/N): 
```

Type `y` and press Enter to proceed.

```
Step 5: Applying migration to Supabase...
------------------------------------------
Migrating to version 20251013192000 (1 migration)
  -> 20251013192000_add_pool_management.sql ........................ ok (123ms)
  -------------------------
  -> 1 migration ok (123ms)

=========================================
✓ Migration applied successfully!
=========================================

Step 6: Verifying changes...
----------------------------
     column_name      |     data_type      | is_nullable
----------------------+--------------------+-------------
 assigned_at          | timestamp          | YES
 chain_id             | bigint             | YES
 deployed_at          | timestamp          | YES
 deployment_block     | bigint             | YES
 deployment_tx_hash   | varchar(70)        | YES
 is_deployed          | boolean            | NO
 network_identifier   | varchar            | YES
 recycled_at          | timestamp          | YES
 times_used           | integer            | NO

=========================================
✓✓✓ Pool management fields added!
=========================================

Next steps:
1. cd pool_management
2. make build
3. make create NETWORK=base-sepolia COUNT=5
```

---

## Alternative: Manual Steps (Option B)

If you prefer to run each step manually:

### 1. Export DATABASE_URL
```bash
source pool_management/scripts/export_db_url.sh
```

### 2. Generate ent code
```bash
cd /home/commendatore/Desktop/NEDA/rails/aggregator
go generate ./ent
```

### 3. Create Atlas migration
```bash
atlas migrate diff add_pool_management \
  --dir "file://ent/migrate/migrations" \
  --to "ent://ent/schema" \
  --dev-url "docker://postgres/15/test?search_path=public"
```

### 4. Review the migration
```bash
# Find latest migration
ls -lt ent/migrate/migrations/*.sql | head -1

# View it
cat $(ls -t ent/migrate/migrations/*.sql | head -1)
```

### 5. Apply with Atlas
```bash
atlas migrate apply \
  --dir "file://ent/migrate/migrations" \
  --url "$DATABASE_URL" \
  --allow-dirty \
  --revisions-schema atlas_schema_revisions
```

### 6. Verify
```bash
psql "$DATABASE_URL" -c "
SELECT column_name, data_type, is_nullable 
FROM information_schema.columns 
WHERE table_name = 'receive_addresses'
AND column_name IN (
    'is_deployed', 
    'deployment_block', 
    'deployment_tx_hash',
    'deployed_at',
    'network_identifier',
    'chain_id',
    'assigned_at',
    'recycled_at',
    'times_used'
)
ORDER BY column_name;
"
```

---

## After Migration: Build & Test

Once migration is complete:

```bash
# Navigate to pool management
cd pool_management

# Build the tools
make build

# Create 5 test addresses
make create NETWORK=base-sepolia COUNT=5

# Check the generated file
ls -lh pool_*.json
cat pool_*.json | jq '.[0]'
```

---

## Troubleshooting

### Issue: "DATABASE_URL is not set"

**Solution:**
```bash
# Make sure you sourced (not just ran) the script
source pool_management/scripts/export_db_url.sh

# Verify it's set
echo $DATABASE_URL
```

### Issue: "psql: command not found"

**Solution:**
```bash
# Install PostgreSQL client
sudo apt update
sudo apt install postgresql-client-16
```

### Issue: "atlas: command not found"

**Solution:**
```bash
# Install Atlas
curl -sSf https://atlasgo.sh | sh

# Or check your existing installation script
./install_atlas.sh
```

### Issue: "connection refused"

**Solution:**
- Check your `.env` file has correct `DB_HOST`, `DB_PORT`, `DB_PASSWORD`
- Verify you can access Supabase from your network
- Check if using pooler endpoint (port 6543) or direct (port 5432)

### Issue: "permission denied"

**Solution:**
```bash
# Make scripts executable
chmod +x pool_management/scripts/*.sh
```

### Issue: "migration already applied"

**Solution:**
```bash
# Check migration status
atlas migrate status \
  --dir "file://ent/migrate/migrations" \
  --url "$DATABASE_URL" \
  --revisions-schema atlas_schema_revisions

# If already applied, you're good! Skip to building tools.
```

---

## What Gets Changed

The migration adds these columns to `receive_addresses` table:

| Column | Type | Purpose |
|--------|------|---------|
| `is_deployed` | BOOLEAN | Whether account is deployed on-chain |
| `deployment_block` | BIGINT | Block number of deployment |
| `deployment_tx_hash` | VARCHAR(70) | Transaction hash of deployment |
| `deployed_at` | TIMESTAMP | When deployed |
| `network_identifier` | VARCHAR | Network name (e.g., "base-sepolia") |
| `chain_id` | BIGINT | Chain ID (e.g., 84532) |
| `assigned_at` | TIMESTAMP | When assigned to an order |
| `recycled_at` | TIMESTAMP | When returned to pool |
| `times_used` | INTEGER | Number of times reused |

Plus:
- Updates `status` enum with new values: `pool_ready`, `pool_assigned`, `pool_processing`, `pool_completed`
- Adds indexes for efficient pool queries

**Existing data is NOT affected** - old addresses keep their status values.

---

## Summary

**Quick version:**
```bash
cd /home/commendatore/Desktop/NEDA/rails/aggregator
source pool_management/scripts/export_db_url.sh
./pool_management/scripts/apply_pool_migration.sh
```

**Next steps after migration:**
```bash
cd pool_management
make build
make create NETWORK=base-sepolia COUNT=5
```

That's it! 🚀
