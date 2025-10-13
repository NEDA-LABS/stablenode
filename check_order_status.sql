-- Check order status for the receive addresses mentioned in logs

-- Order 1: 0x013542D234dE04f442a832F475872Acd88Cf0bE4
SELECT 
    po.id as order_id,
    po.status,
    po.amount,
    po.amount_paid,
    po.created_at,
    po.validated_at,
    ra.address as receive_address,
    n.identifier as network
FROM payment_orders po
JOIN receive_addresses ra ON po.receive_address_payment_order = ra.id
JOIN networks n ON po.network = n.id
WHERE ra.address = '0x013542D234dE04f442a832F475872Acd88Cf0bE4'
ORDER BY po.created_at DESC
LIMIT 1;

-- Order 2: 0xF59EFa9b93db835D7db22D6D6Dfe32c9417104A0
SELECT 
    po.id as order_id,
    po.status,
    po.amount,
    po.amount_paid,
    po.created_at,
    po.validated_at,
    ra.address as receive_address,
    n.identifier as network
FROM payment_orders po
JOIN receive_addresses ra ON po.receive_address_payment_order = ra.id
JOIN networks n ON po.network = n.id
WHERE ra.address = '0xF59EFa9b93db835D7db22D6D6Dfe32c9417104A0'
ORDER BY po.created_at DESC
LIMIT 1;

-- Order 3: 0x15A4fF16425e81D46f3F2a74004AEA47D3Bb23ED
SELECT 
    po.id as order_id,
    po.status,
    po.amount,
    po.amount_paid,
    po.created_at,
    po.validated_at,
    ra.address as receive_address,
    n.identifier as network
FROM payment_orders po
JOIN receive_addresses ra ON po.receive_address_payment_order = ra.id
JOIN networks n ON po.network = n.id
WHERE ra.address = '0x15A4fF16425e81D46f3F2a74004AEA47D3Bb23ED'
ORDER BY po.created_at DESC
LIMIT 1;

-- Check all pending orders on base-sepolia
SELECT 
    po.id as order_id,
    po.status,
    po.amount,
    po.amount_paid,
    ra.address as receive_address,
    po.created_at
FROM payment_orders po
JOIN receive_addresses ra ON po.receive_address_payment_order = ra.id
JOIN networks n ON po.network = n.id
WHERE n.identifier = 'base-sepolia'
AND po.status = 'pending'
ORDER BY po.created_at DESC;
