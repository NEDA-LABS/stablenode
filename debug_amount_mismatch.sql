-- Debug amount vs amount_paid mismatch
-- Simplified queries that should work with any schema

-- Query 1: Get problematic orders with basic info
SELECT 
    id,
    status,
    amount,
    amount_paid,
    (amount_paid - amount) as difference,
    network_fee,
    sender_fee,
    receive_address_text,
    created_at
FROM payment_orders
WHERE amount != amount_paid
AND amount_paid > 0
ORDER BY created_at DESC
LIMIT 10;

-- Query 2: Check receive addresses that might be reused
SELECT 
    receive_address_text,
    COUNT(*) as order_count,
    SUM(amount) as total_amount,
    SUM(amount_paid) as total_paid
FROM payment_orders
WHERE amount_paid > 0
GROUP BY receive_address_text
HAVING COUNT(*) > 1
ORDER BY order_count DESC;

-- Query 3: Get specific details for orders with big mismatches
SELECT 
    id,
    status,
    amount,
    amount_paid,
    tx_hash,
    from_address,
    return_address,
    receive_address_text,
    created_at
FROM payment_orders
WHERE (amount_paid > amount * 1.5 OR amount_paid = 0)
AND status IN ('pending', 'validated')
ORDER BY created_at DESC
LIMIT 10;
