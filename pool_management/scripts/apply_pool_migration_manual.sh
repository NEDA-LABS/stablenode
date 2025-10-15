#!/bin/bash
# Apply Pool Management Migration - Manual SQL Approach
# This creates the migration SQL manually and applies it with Atlas

set -e  # Exit on error

echo "========================================="
echo "Pool Management Migration"
echo "Manual SQL Approach - No Docker Required"
echo "========================================="
echo ""

# Check if DATABASE_URL is set
if [ -z "$DATABASE_URL" ]; then
    echo "❌ ERROR: DATABASE_URL is not set"
    echo ""
    echo "Run this first:"
    echo "  source pool_management/scripts/export_db_url.sh"
    exit 1
fi

echo "✓ DATABASE_URL is set"
echo ""

# Navigate to project root
cd /home/commendatore/Desktop/NEDA/rails/aggregator

# Step 1: Generate ent code
echo "Step 1: Generating ent code from schema..."
echo "-------------------------------------------"
go generate ./ent

if [ $? -eq 0 ]; then
    echo "✓ Ent code generated successfully"
else
    echo "❌ Failed to generate ent code"
    exit 1
fi
echo ""

# Step 2: Create migration SQL file manually
echo "Step 2: Creating migration SQL file..."
echo "---------------------------------------"

TIMESTAMP=$(date +%Y%m%d%H%M%S)
MIGRATION_FILE="ent/migrate/migrations/${TIMESTAMP}_add_pool_management.sql"

cat > "$MIGRATION_FILE" << 'EOF'
-- Add pool management fields to receive_addresses table
-- Generated: 2025-10-13

-- Add deployment tracking fields
ALTER TABLE receive_addresses 
  ADD COLUMN IF NOT EXISTS is_deployed BOOLEAN NOT NULL DEFAULT false,
  ADD COLUMN IF NOT EXISTS deployment_block BIGINT,
  ADD COLUMN IF NOT EXISTS deployment_tx_hash VARCHAR(70),
  ADD COLUMN IF NOT EXISTS deployed_at TIMESTAMPTZ;

-- Add network identification fields
ALTER TABLE receive_addresses
  ADD COLUMN IF NOT EXISTS network_identifier VARCHAR,
  ADD COLUMN IF NOT EXISTS chain_id BIGINT;

-- Add pool management fields
ALTER TABLE receive_addresses
  ADD COLUMN IF NOT EXISTS assigned_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS recycled_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS times_used INTEGER NOT NULL DEFAULT 0;

-- Add comments for documentation
COMMENT ON COLUMN receive_addresses.is_deployed IS 'Whether the smart account is deployed on-chain';
COMMENT ON COLUMN receive_addresses.deployment_block IS 'Block number where account was deployed';
COMMENT ON COLUMN receive_addresses.deployment_tx_hash IS 'Transaction hash of deployment';
COMMENT ON COLUMN receive_addresses.deployed_at IS 'Timestamp when deployed';
COMMENT ON COLUMN receive_addresses.network_identifier IS 'Network identifier (e.g., base-sepolia)';
COMMENT ON COLUMN receive_addresses.chain_id IS 'Chain ID (e.g., 84532)';
COMMENT ON COLUMN receive_addresses.assigned_at IS 'When address was assigned to an order';
COMMENT ON COLUMN receive_addresses.recycled_at IS 'When address was returned to pool';
COMMENT ON COLUMN receive_addresses.times_used IS 'Number of times address has been reused';

-- Update status enum to include pool management values
-- First, check if the new values already exist
DO $$
BEGIN
    -- Add new enum values if they don't exist
    IF NOT EXISTS (SELECT 1 FROM pg_enum WHERE enumlabel = 'pool_ready' AND enumtypid = 'receive_addresses_status'::regtype) THEN
        ALTER TYPE receive_addresses_status ADD VALUE 'pool_ready';
    END IF;
    
    IF NOT EXISTS (SELECT 1 FROM pg_enum WHERE enumlabel = 'pool_assigned' AND enumtypid = 'receive_addresses_status'::regtype) THEN
        ALTER TYPE receive_addresses_status ADD VALUE 'pool_assigned';
    END IF;
    
    IF NOT EXISTS (SELECT 1 FROM pg_enum WHERE enumlabel = 'pool_processing' AND enumtypid = 'receive_addresses_status'::regtype) THEN
        ALTER TYPE receive_addresses_status ADD VALUE 'pool_processing';
    END IF;
    
    IF NOT EXISTS (SELECT 1 FROM pg_enum WHERE enumlabel = 'pool_completed' AND enumtypid = 'receive_addresses_status'::regtype) THEN
        ALTER TYPE receive_addresses_status ADD VALUE 'pool_completed';
    END IF;
END$$;

-- Create indexes for efficient pool queries
CREATE INDEX IF NOT EXISTS idx_receive_addresses_pool_lookup 
  ON receive_addresses (status, is_deployed, network_identifier);

CREATE INDEX IF NOT EXISTS idx_receive_addresses_chain_status 
  ON receive_addresses (chain_id, status);

CREATE INDEX IF NOT EXISTS idx_receive_addresses_times_used 
  ON receive_addresses (times_used);
EOF

echo "✓ Migration file created: $MIGRATION_FILE"
echo ""

# Step 3: Show the migration
echo "Step 3: Review migration SQL..."
echo "-------------------------------"
echo "File: $MIGRATION_FILE"
echo ""
cat "$MIGRATION_FILE"
echo ""

# Step 4: Prompt for confirmation
echo "========================================="
read -p "Apply this migration to database? (y/N): " -n 1 -r
echo ""
echo "========================================="

if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "❌ Migration cancelled"
    echo "Migration file saved at: $MIGRATION_FILE"
    echo "You can apply it manually with:"
    echo "  psql \"\$DATABASE_URL\" -f $MIGRATION_FILE"
    exit 0
fi

# Step 5: Apply migration directly with psql
echo ""
echo "Step 5: Applying migration..."
echo "-----------------------------"

psql "$DATABASE_URL" -f "$MIGRATION_FILE"

if [ $? -eq 0 ]; then
    echo ""
    echo "✓ Migration applied successfully"
else
    echo ""
    echo "❌ Migration failed"
    exit 1
fi

# Step 6: Record in Atlas revisions (optional but recommended)
echo ""
echo "Step 6: Recording migration in Atlas..."
echo "----------------------------------------"

# Create atlas migration hash
MIGRATION_HASH=$(sha256sum "$MIGRATION_FILE" | awk '{print $1}')

# Record the migration in atlas_schema_revisions
psql "$DATABASE_URL" << EOSQL
INSERT INTO atlas_schema_revisions.atlas_schema_revisions (
    version,
    description,
    type,
    applied,
    total,
    executed_at,
    execution_time,
    hash,
    operator_version
) VALUES (
    '${TIMESTAMP}',
    'add_pool_management',
    2,
    1,
    1,
    NOW(),
    0,
    '${MIGRATION_HASH}',
    'manual'
) ON CONFLICT (version) DO NOTHING;
EOSQL

echo "✓ Migration recorded in Atlas revisions"
echo ""

# Step 7: Verify changes
echo "Step 7: Verifying changes..."
echo "----------------------------"
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

echo ""
echo "========================================="
echo "✓✓✓ Pool management fields added!"
echo "========================================="
echo ""
echo "Next steps:"
echo "1. cd pool_management"
echo "2. make build"
echo "3. make create NETWORK=base-sepolia COUNT=5"
echo ""
