-- Add USDC token on Base Sepolia
-- Base Sepolia USDC contract address: 0x036CbD53842c5426634e7929541eC2318f3dCF7e

-- 1. Add USDC token to Base Sepolia network
INSERT INTO "public"."tokens" ("id", "created_at", "updated_at", "symbol", "contract_address", "decimals", "is_enabled", "network_tokens", "base_currency") 
VALUES (
    55834574854, -- New token ID (increment from last DAI token)
    NOW(), 
    NOW(), 
    'USDC', 
    '0x036CbD53842c5426634e7929541eC2318f3dCF7e', -- Base Sepolia USDC contract
    6, -- USDC has 6 decimals
    true, -- Enable the token
    17179869187, -- Base Sepolia network ID
    'USD'
);

-- 2. Add provider order token configuration for USDC
-- This allows the provider 'AtGaDPqT' to accept USDC orders
INSERT INTO "public"."provider_order_tokens" ("id", "created_at", "updated_at", "fixed_conversion_rate", "floating_conversion_rate", "conversion_rate_type", "max_order_amount", "min_order_amount", "provider_profile_order_tokens", "address", "network", "fiat_currency_provider_order_tokens", "token_provider_order_tokens", "rate_slippage") 
VALUES (
    33, -- New provider order token ID
    NOW(), 
    NOW(), 
    0, -- Fixed rate (0 means not used)
    0, -- Floating rate (0 means market rate)
    'floating', -- Use floating/market rates
    900, -- Max order amount (same as DAI config)
    0.5, -- Min order amount (same as DAI config)
    'AtGaDPqT', -- Provider ID
    '0x409689E3008d43a9eb439e7B275749D4a71D8E2D', -- Provider's address
    'base-sepolia', -- Network identifier
    '5a349408-ebcf-4c7e-98c7-46b6596e0b27', -- NGN fiat currency ID
    55834574854, -- USDC token ID (from step 1)
    0 -- Rate slippage tolerance
);

-- 3. Add sender order token configuration for USDC (if you want senders to use USDC)
-- This allows the sender profile to create orders with USDC
INSERT INTO "public"."sender_order_tokens" ("id", "created_at", "updated_at", "fee_percent", "fee_address", "refund_address", "sender_profile_order_tokens", "token_sender_order_tokens") 
VALUES (
    81604378635, -- New sender order token ID (incremented from 81604378634)
    NOW(), 
    NOW(), 
    0, -- Fee percentage
    '0x409689E3008d43a9eb439e7B275749D4a71D8E2D', -- Fee address
    '0x409689E3008d43a9eb439e7B275749D4a71D8E2D', -- Refund address
    'e93a1cba-832f-4a7c-aab5-929a53c84324', -- Sender profile ID
    55834574854 -- USDC token ID
);

-- Verification queries to check the setup
-- Run these after the inserts to verify everything is configured correctly:

-- Check if USDC token was added
SELECT * FROM tokens WHERE symbol = 'USDC' AND network_tokens = 17179869187;

-- Check if provider can accept USDC orders
SELECT pot.*, t.symbol, fc.code as fiat_currency 
FROM provider_order_tokens pot
JOIN tokens t ON pot.token_provider_order_tokens = t.id
JOIN fiat_currencies fc ON pot.fiat_currency_provider_order_tokens = fc.id
WHERE pot.provider_profile_order_tokens = 'AtGaDPqT' AND t.symbol = 'USDC';

-- Check if sender can create USDC orders
SELECT sot.*, t.symbol 
FROM sender_order_tokens sot
JOIN tokens t ON sot.token_sender_order_tokens = t.id
WHERE sot.sender_profile_order_tokens = 'e93a1cba-832f-4a7c-aab5-929a53c84324' AND t.symbol = 'USDC';
