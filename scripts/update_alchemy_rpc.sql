-- Update RPC endpoints to use Alchemy base URLs (without API keys)
-- API keys are now loaded from ALCHEMY_API_KEY environment variable at runtime
-- This improves security and makes key rotation easier

-- Base Sepolia
UPDATE networks 
SET rpc_endpoint = 'https://base-sepolia.g.alchemy.com/v2' 
WHERE chain_id = 84532;

-- Ethereum Sepolia
UPDATE networks 
SET rpc_endpoint = 'https://eth-sepolia.g.alchemy.com/v2' 
WHERE chain_id = 11155111;

-- Arbitrum Sepolia
UPDATE networks 
SET rpc_endpoint = 'https://arb-sepolia.g.alchemy.com/v2' 
WHERE chain_id = 421614;

-- Show updated networks (should NOT contain API keys)
SELECT chain_id, identifier, rpc_endpoint FROM networks WHERE chain_id IN (84532, 11155111, 421614);

-- IMPORTANT: Ensure ALCHEMY_API_KEY is set in your .env file
-- The system will automatically append the API key at runtime
