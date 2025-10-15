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
