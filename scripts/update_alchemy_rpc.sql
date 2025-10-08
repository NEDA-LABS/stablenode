-- Update RPC endpoints to use Alchemy instead of Infura
-- For networks where you want to use Alchemy

-- Base Sepolia
UPDATE networks 
SET rpc_endpoint = 'https://base-sepolia.g.alchemy.com/v2' 
WHERE chain_id = 84532;

-- Ethereum Sepolia (if needed)
UPDATE networks 
SET rpc_endpoint = 'https://eth-sepolia.g.alchemy.com/v2' 
WHERE chain_id = 11155111;

-- Arbitrum Sepolia (if needed)
UPDATE networks 
SET rpc_endpoint = 'https://arb-sepolia.g.alchemy.com/v2' 
WHERE chain_id = 421614;

-- Show updated networks
SELECT chain_id, identifier, rpc_endpoint FROM networks WHERE chain_id IN (84532, 11155111, 421614);
