-- Remove unique constraint on receive_addresses.address to allow address reuse
-- This enables pool addresses to be used by multiple orders simultaneously

-- Drop the unique index/constraint on address field
DROP INDEX IF EXISTS receive_addresses_address_key;
ALTER TABLE receive_addresses DROP CONSTRAINT IF EXISTS receive_addresses_address_key;

-- Add a non-unique index for performance (optional but recommended)
CREATE INDEX IF NOT EXISTS idx_receive_addresses_address ON receive_addresses(address);
