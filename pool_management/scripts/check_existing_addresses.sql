-- Check existing receive addresses before pool deployment
-- Run: psql $DATABASE_URL -f pool_management/scripts/check_existing_addresses.sql

\echo '========================================='
\echo 'Existing Receive Addresses Analysis'
\echo '========================================='
\echo ''

-- Count by status
\echo '1. Count by Status:'
\echo '-------------------'
SELECT 
    status,
    COUNT(*) as count
FROM receive_addresses
GROUP BY status
ORDER BY count DESC;

\echo ''
\echo '2. Addresses with Payment Orders:'
\echo '----------------------------------'
SELECT COUNT(*) as addresses_with_orders
FROM receive_addresses ra
WHERE EXISTS (
    SELECT 1 FROM payment_orders po 
    WHERE po.payment_order_receive_address = ra.id
);

\echo ''
\echo '3. Recent Addresses (last 7 days):'
\echo '-----------------------------------'
SELECT 
    COUNT(*) as recent_addresses,
    MIN(created_at) as oldest,
    MAX(created_at) as newest
FROM receive_addresses
WHERE created_at > NOW() - INTERVAL '7 days';

\echo ''
\echo '4. Addresses by Network (if chain_id exists):'
\echo '----------------------------------------------'
SELECT 
    COALESCE(chain_id::text, 'unknown') as chain_id,
    COUNT(*) as count
FROM receive_addresses
GROUP BY chain_id
ORDER BY count DESC;

\echo ''
\echo '5. Sample of Existing Addresses:'
\echo '---------------------------------'
SELECT 
    id,
    address,
    status,
    created_at,
    CASE 
        WHEN EXISTS (
            SELECT 1 FROM payment_orders po 
            WHERE po.payment_order_receive_address = id
        ) THEN 'Yes' 
        ELSE 'No' 
    END as has_order
FROM receive_addresses
ORDER BY created_at DESC
LIMIT 5;

\echo ''
\echo '========================================='
\echo 'Recommendation:'
\echo '========================================='
\echo 'If addresses have payment orders linked,'
\echo 'DO NOT delete them. They can coexist with'
\echo 'the new pool addresses.'
\echo ''
