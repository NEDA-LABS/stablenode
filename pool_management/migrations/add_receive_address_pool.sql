-- Migration: Add Receive Address Pool Support
-- Date: 2025-10-13
-- Description: Adds fields to support pre-deployed address pool management

-- Add new fields for pool management
ALTER TABLE receive_addresses 
ADD COLUMN IF NOT EXISTS is_deployed BOOLEAN DEFAULT FALSE,
ADD COLUMN IF NOT EXISTS deployment_block BIGINT,
ADD COLUMN IF NOT EXISTS deployment_tx_hash VARCHAR(70),
ADD COLUMN IF NOT EXISTS deployed_at TIMESTAMP,
ADD COLUMN IF NOT EXISTS network_identifier VARCHAR(50),
ADD COLUMN IF NOT EXISTS chain_id BIGINT,
ADD COLUMN IF NOT EXISTS assigned_at TIMESTAMP,
ADD COLUMN IF NOT EXISTS recycled_at TIMESTAMP,
ADD COLUMN IF NOT EXISTS times_used INTEGER DEFAULT 0;

-- Update status enum to include pool statuses
-- Note: This depends on your database. For PostgreSQL:
ALTER TABLE receive_addresses 
ALTER COLUMN status TYPE VARCHAR(20);

-- Update status field to support new values (will not affect existing data)
COMMENT ON COLUMN receive_addresses.status IS 'Valid values: pool_ready, pool_assigned, pool_processing, pool_completed, unused, used, expired';

-- Create indexes for efficient pool queries
CREATE INDEX IF NOT EXISTS idx_receive_addresses_pool_lookup 
ON receive_addresses(status, is_deployed, network_identifier) 
WHERE is_deployed = TRUE;

CREATE INDEX IF NOT EXISTS idx_receive_addresses_chain_status 
ON receive_addresses(chain_id, status);

CREATE INDEX IF NOT EXISTS idx_receive_addresses_reuse_limit 
ON receive_addresses(times_used) 
WHERE status = 'pool_ready';

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

-- Create view for pool statistics
CREATE OR REPLACE VIEW receive_address_pool_stats AS
SELECT 
    network_identifier,
    chain_id,
    status,
    COUNT(*) as count,
    AVG(times_used) as avg_times_used,
    MAX(times_used) as max_times_used,
    MIN(assigned_at) as oldest_assigned,
    MAX(recycled_at) as latest_recycled
FROM receive_addresses
WHERE is_deployed = TRUE
GROUP BY network_identifier, chain_id, status;

-- Grant permissions (adjust as needed for your setup)
-- GRANT SELECT, INSERT, UPDATE ON receive_addresses TO your_app_user;
-- GRANT SELECT ON receive_address_pool_stats TO your_app_user;

-- Verification queries
-- Run these after migration to verify

-- Check new columns exist
-- SELECT column_name, data_type, is_nullable 
-- FROM information_schema.columns 
-- WHERE table_name = 'receive_addresses'
-- ORDER BY ordinal_position;

-- Check indexes created
-- SELECT indexname, indexdef 
-- FROM pg_indexes 
-- WHERE tablename = 'receive_addresses';
