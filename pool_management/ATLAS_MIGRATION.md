# Pool Management Atlas Migration Guide

This guide shows how to apply the pool management schema changes using Atlas, following your existing setup from `docs/atlas-supabase-migration.md`.

## Overview

We'll use Atlas to:
1. Generate ent code from updated schema
2. Create a migration diff for just the pool management changes
3. Apply it to your Supabase database

## Prerequisites

✅ Schema already updated in `ent/schema/receiveaddress.go`  
✅ Atlas CLI installed  
✅ `DATABASE_URL` environment variable set  

## Quick Start (Automated)

```bash
cd /home/commendatore/Desktop/NEDA/rails/aggregator

# Set your database URL
export DATABASE_URL="postgresql://user.project:password@aws-1-eu-central-1.pooler.supabase.com:6543/dbname?sslmode=require"

# Run the migration script
./pool_management/scripts/apply_pool_migration.sh
```

The script will:
1. Generate ent code
2. Create Atlas migration
3. Show you the changes
4. Ask for confirmation
5. Apply to database
6. Verify the changes

## Manual Steps (Step by Step)

If you prefer to do it manually or need more control:

### Step 1: Verify Schema Changes

```bash
cd /home/commendatore/Desktop/NEDA/rails/aggregator

# Check the updated schema
cat ent/schema/receiveaddress.go | grep -A 5 "is_deployed\|pool_ready"
```

Should show the new fields and status values.

### Step 2: Generate Ent Code

```bash
# Generate ent code from schema
go generate ./ent
```

This creates Go code for the new fields. You should see updates in:
- `ent/receiveaddress/receiveaddress.go`
- `ent/receiveaddress/where.go`
- `ent/receiveaddress_create.go`
- `ent/receiveaddress_update.go`

### Step 3: Create Atlas Migration

```bash
# Create migration diff for the changes
atlas migrate diff add_pool_management \
  --dir "file://ent/migrate/migrations" \
  --to "ent://ent/schema" \
  --dev-url "docker://postgres/15/test?search_path=public"
```

This creates a new migration file in `ent/migrate/migrations/` with a timestamp.

**What this does:**
- Compares current schema to database
- Generates SQL for ONLY the differences
- Creates a new `.sql` file

### Step 4: Review the Migration

```bash
# Find the latest migration
ls -lt ent/migrate/migrations/*.sql | head -1

# View it
cat ent/migrate/migrations/20251013_add_pool_management.sql
```

Expected changes:
- Add `is_deployed` BOOLEAN column
- Add `deployment_block` BIGINT column
- Add `deployment_tx_hash` VARCHAR(70) column
- Add `deployed_at` TIMESTAMP column
- Add `network_identifier` VARCHAR column
- Add `chain_id` BIGINT column
- Add `assigned_at` TIMESTAMP column
- Add `recycled_at` TIMESTAMP column
- Add `times_used` INTEGER column
- Update `status` enum with new values
- Create indexes for pool queries

### Step 5: Apply Migration with Atlas

Using your existing Atlas setup (from `docs/atlas-supabase-migration.md`):

```bash
# Apply the migration
atlas migrate apply \
  --dir "file://ent/migrate/migrations" \
  --url "$DATABASE_URL" \
  --allow-dirty \
  --revisions-schema atlas_schema_revisions
```

**Flags explained:**
- `--allow-dirty`: Allow running on Supabase (has system schemas)
- `--revisions-schema atlas_schema_revisions`: Use your existing revisions table

### Step 6: Verify Changes

```bash
# Check new columns exist
psql "$DATABASE_URL" -c "
SELECT column_name, data_type, is_nullable 
FROM information_schema.columns 
WHERE table_name = 'receive_addresses'
ORDER BY ordinal_position;
"

# Check new status values
psql "$DATABASE_URL" -c "
SELECT 
    e.enumlabel AS status_value
FROM pg_type t 
JOIN pg_enum e ON t.oid = e.enumtypid  
JOIN pg_catalog.pg_namespace n ON n.oid = t.typnamespace
WHERE t.typname = 'receive_addresses_status';
"

# Check indexes created
psql "$DATABASE_URL" -c "
SELECT indexname, indexdef 
FROM pg_indexes 
WHERE tablename = 'receive_addresses';
"
```

## Troubleshooting

### Issue: "schema is not clean"

**Cause:** Atlas detected unexpected changes

**Solution:**
```bash
# Check what's different
atlas schema diff \
  --from "$DATABASE_URL" \
  --to "ent://ent/schema"

# If safe, use --allow-dirty flag (already in commands above)
```

### Issue: "migration already applied"

**Cause:** Migration was already run

**Solution:**
```bash
# Check migration status
atlas migrate status \
  --dir "file://ent/migrate/migrations" \
  --url "$DATABASE_URL" \
  --revisions-schema atlas_schema_revisions

# If needed, mark as applied without running
atlas migrate set 20251013_add_pool_management \
  --dir "file://ent/migrate/migrations" \
  --url "$DATABASE_URL" \
  --revisions-schema atlas_schema_revisions
```

### Issue: Build errors after migration

**Cause:** Ent code not regenerated or Go modules out of sync

**Solution:**
```bash
# Regenerate ent code
go generate ./ent

# Update modules
go mod tidy

# Rebuild pool tools
cd pool_management
make clean-bin
make build
```

### Issue: Enum constraint violation

**Cause:** Existing data has status values not in the new enum

**Solution:**
```bash
# Check existing status values
psql "$DATABASE_URL" -c "
SELECT status, COUNT(*) 
FROM receive_addresses 
GROUP BY status;
"

# All existing values (unused, used, expired) are in the new enum
# Should not be an issue
```

## Verification Checklist

After migration, verify:

- [ ] New columns exist in `receive_addresses` table
- [ ] Status enum includes pool values (`pool_ready`, `pool_assigned`, etc.)
- [ ] Indexes created for efficient queries
- [ ] Old data still accessible (status values unchanged)
- [ ] Ent code regenerated (no build errors)
- [ ] Pool tools build successfully: `cd pool_management && make build`

## Next Steps After Migration

1. **Build pool tools**
   ```bash
   cd pool_management
   make build
   ```

2. **Create test addresses**
   ```bash
   make create NETWORK=base-sepolia COUNT=5
   ```

3. **Deploy them**
   ```bash
   make deploy \
     POOL_FILE_INPUT=pool_*.json \
     RPC_URL=$BASE_SEPOLIA_RPC \
     PRIVATE_KEY=$DEPLOYER_PRIVATE_KEY
   ```

4. **Mark as deployed**
   ```bash
   make mark-deployed DEPLOY_RESULTS_INPUT=deployment_*.json
   ```

5. **Verify pool**
   ```bash
   make verify NETWORK=base-sepolia
   ```

## Migration Timeline

Expected time:
- **Generate ent code**: 10 seconds
- **Create migration**: 30 seconds
- **Review migration**: 2 minutes
- **Apply migration**: 5-30 seconds
- **Verify**: 1 minute
- **Total**: ~5 minutes

## Rollback (If Needed)

If you need to rollback the migration:

```bash
# Check migration status
atlas migrate status \
  --dir "file://ent/migrate/migrations" \
  --url "$DATABASE_URL" \
  --revisions-schema atlas_schema_revisions

# Rollback to previous version
atlas migrate down \
  --dir "file://ent/migrate/migrations" \
  --url "$DATABASE_URL" \
  --revisions-schema atlas_schema_revisions
```

**Note:** Rollback will remove the new columns. Make sure no pool addresses are in use.

## Comparison: Atlas vs Manual SQL

| Method | Pros | Cons |
|--------|------|------|
| **Atlas** (Recommended) | Version controlled, reversible, tracks changes | Requires Atlas CLI |
| **Manual SQL** | Direct control, simple | No version tracking, harder to rollback |

You're using Atlas, which is the better approach for production systems!

## References

- Your existing setup: `docs/atlas-supabase-migration.md`
- Manual SQL fallback: `pool_management/migrations/add_receive_address_pool.sql`
- Ent docs: https://entgo.io/docs/migrate
- Atlas docs: https://atlasgo.io/
