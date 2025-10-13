-- Check if users sent multiple payments to same receive address

-- Check the specific problematic addresses
SELECT 
    receive_address_text,
    COUNT(*) as payment_count,
    array_agg(DISTINCT tx_hash) as transaction_hashes,
    array_agg(amount_paid) as amounts_paid,
    SUM(amount_paid) as total_paid
FROM payment_orders
WHERE receive_address_text IN (
    '0x15A4fF16425e81D46f3F2a74004AEA47D3Bb23ED',
    '0xF59EFa9b93db835D7db22D6D6Dfe32c9417104A0',
    '0x013542D234dE04f442a832F475872Acd88Cf0bE4'
)
GROUP BY receive_address_text;

-- Check all payment orders for these addresses
SELECT 
    id,
    status,
    amount,
    amount_paid,
    tx_hash,
    from_address,
    receive_address_text,
    created_at
FROM payment_orders
WHERE receive_address_text IN (
    '0x15A4fF16425e81D46f3F2a74004AEA47D3Bb23ED',
    '0xF59EFa9b93db835D7db22D6D6Dfe32c9417104A0',
    '0x013542D234dE04f442a832F475872Acd88Cf0bE4'
)
ORDER BY receive_address_text, created_at;
