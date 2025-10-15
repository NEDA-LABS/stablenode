# Receive Address Pool - Quick Start Guide

## What This Solves

✅ **No more AA20/AA23 errors** - addresses are already deployed  
✅ **No deployment gas at transaction time** - Alchemy sponsors upfront  
✅ **Faster order creation** - just pick from pool  
✅ **Address reuse** - efficient resource utilization  

---

## Step-by-Step Implementation

### Step 1: Update Schema (5 minutes)

```bash
# Update the ReceiveAddress schema
cd /home/commendatore/Desktop/NEDA/rails/aggregator

# Edit ent/schema/receiveaddress.go with the new fields from RECEIVE_ADDRESS_POOL_IMPLEMENTATION.md
# Then regenerate ent code:
go generate ./ent
```

Or use the SQL migration directly:
```bash
# Run the migration
psql $DATABASE_URL -f migrations/add_receive_address_pool.sql
```

### Step 2: Create Pool Service (10 minutes)

```bash
# Create the new service file
touch services/receive_address_pool.go

# Copy the code from RECEIVE_ADDRESS_POOL_IMPLEMENTATION.md Phase 2
# Or write a simpler version first
```

### Step 3: Initialize Pool (30 minutes - mostly deployment time)

```bash
# Option A: Manual SQL (for testing)
# Create addresses without deployment first, then deploy manually

# Option B: Build init script
mkdir -p cmd/init_receive_pool
touch cmd/init_receive_pool/main.go

# Copy code from implementation plan
# Build and run:
go build -o bin/init_receive_pool ./cmd/init_receive_pool
./bin/init_receive_pool --chain-id=84532 --network=base-sepolia --size=5
```

### Step 4: Update Order Creation (5 minutes)

**File:** `controllers/sender/sender.go`

Find this line (around line 434):
```go
receiveAddress, err := ctrl.receiveAddressService.CreateSmartAddress(ctx, order.TokenID.String())
```

Replace with:
```go
// Get from pool instead of creating new
poolService := svc.NewReceiveAddressPoolService()
receiveAddress, err := poolService.GetAvailableAddress(ctx, token.Edges.Network.ChainID, token.Edges.Network.Identifier)
```

### Step 5: Add Recycling (5 minutes)

Find where orders are marked as settled/completed and add:

```go
// After order completion
if order.Edges.ReceiveAddress != nil {
    poolService := svc.NewReceiveAddressPoolService()
    go func() {
        if err := poolService.RecycleAddress(context.Background(), order.Edges.ReceiveAddress.ID); err != nil {
            logger.Warnf("Failed to recycle address: %v", err)
        }
    }()
}
```

### Step 6: Test (10 minutes)

```bash
# 1. Restart application
make restart  # or however you restart

# 2. Create a test order
curl -X POST http://localhost:8080/api/v1/sender/payment-order \
  -H "Content-Type: application/json" \
  -d '{
    "amount": "10",
    "token": "USDC",
    "network": "base-sepolia",
    "recipient": {...}
  }'

# 3. Check logs
tail -f tmp/logs.txt | grep "pool"

# 4. Verify address is from pool
# The receiveAddress in response should match one from your pool
```

---

## Simplified MVP (1 hour implementation)

If you want to start simple:

### Minimal Schema Changes

```sql
-- Just add the essential fields
ALTER TABLE receive_addresses 
ADD COLUMN is_deployed BOOLEAN DEFAULT FALSE,
ADD COLUMN chain_id BIGINT,
ADD COLUMN times_used INTEGER DEFAULT 0;

-- Add pool_ready status
-- (Manually add rows with this status after deployment)
```

### Minimal Pool Service

```go
// services/receive_address_pool.go - Minimal version

package services

import (
    "context"
    "fmt"
    "math/rand"
    "time"

    "github.com/NEDA-LABS/stablenode/ent"
    "github.com/NEDA-LABS/stablenode/ent/receiveaddress"
    "github.com/NEDA-LABS/stablenode/storage"
)

type ReceiveAddressPoolService struct{}

func NewReceiveAddressPoolService() *ReceiveAddressPoolService {
    return &ReceiveAddressPoolService{}
}

func (s *ReceiveAddressPoolService) GetAvailableAddress(ctx context.Context, chainID int64) (*ent.ReceiveAddress, error) {
    // Get all available addresses
    addresses, err := storage.Client.ReceiveAddress.
        Query().
        Where(
            receiveaddress.StatusEQ("pool_ready"),
            receiveaddress.IsDeployedEQ(true),
            receiveaddress.ChainIDEQ(chainID),
        ).
        All(ctx)
    
    if err != nil || len(addresses) == 0 {
        return nil, fmt.Errorf("no available addresses in pool")
    }

    // Pick random
    rand.Seed(time.Now().UnixNano())
    selected := addresses[rand.Intn(len(addresses))]

    // Mark as assigned (simple status update)
    selected, err = selected.Update().
        SetStatus("pool_assigned").
        SetTimesUsed(selected.TimesUsed + 1).
        Save(ctx)
    
    return selected, err
}

func (s *ReceiveAddressPoolService) RecycleAddress(ctx context.Context, addressID int) error {
    address, err := storage.Client.ReceiveAddress.Get(ctx, addressID)
    if err != nil {
        return err
    }

    _, err = address.Update().
        SetStatus("pool_ready").
        Save(ctx)
    
    return err
}
```

### Manual Pool Creation

Instead of automated deployment, manually create pool:

```bash
# 1. Generate 5 addresses using existing CreateSmartAccount
# 2. Deploy each one using Alchemy dashboard or Tenderly
# 3. Insert into database:

INSERT INTO receive_addresses (
    address, 
    is_deployed, 
    chain_id, 
    status, 
    times_used,
    created_at,
    updated_at
) VALUES 
('0xAddress1...', true, 84532, 'pool_ready', 0, NOW(), NOW()),
('0xAddress2...', true, 84532, 'pool_ready', 0, NOW(), NOW()),
('0xAddress3...', true, 84532, 'pool_ready', 0, NOW(), NOW()),
('0xAddress4...', true, 84532, 'pool_ready', 0, NOW(), NOW()),
('0xAddress5...', true, 84532, 'pool_ready', 0, NOW(), NOW());
```

---

## Testing Checklist

- [ ] Schema updated (run migration)
- [ ] Pool service created
- [ ] 5 addresses deployed and added to database
- [ ] Order creation uses pool
- [ ] Test order created successfully
- [ ] Payment detected by indexer
- [ ] Order completed successfully
- [ ] Address recycled back to pool
- [ ] Can create another order with recycled address

---

## Monitoring

### Check Pool Status

```sql
-- Available addresses
SELECT COUNT(*) FROM receive_addresses 
WHERE status = 'pool_ready' AND is_deployed = true;

-- Assigned addresses
SELECT COUNT(*) FROM receive_addresses 
WHERE status = 'pool_assigned';

-- Address usage stats
SELECT 
    address, 
    times_used, 
    status 
FROM receive_addresses 
WHERE is_deployed = true 
ORDER BY times_used DESC;
```

### Alerts to Set Up

1. **Pool Low:** < 2 available addresses
2. **Pool Exhausted:** 0 available addresses  
3. **Address Overuse:** Any address used > 50 times
4. **Stuck Assignment:** Address assigned > 1 hour

---

## Rollback Plan

If something goes wrong:

### 1. Switch Back to Old Method

```go
// In sender controller, comment out pool code:
// poolService := svc.NewReceiveAddressPoolService()
// receiveAddress, err := poolService.GetAvailableAddress(...)

// Uncomment old code:
receiveAddress, err := ctrl.receiveAddressService.CreateSmartAddress(ctx, order.TokenID.String())
```

### 2. Database Rollback

```sql
-- If needed, revert schema changes
ALTER TABLE receive_addresses 
DROP COLUMN IF EXISTS is_deployed,
DROP COLUMN IF EXISTS chain_id,
DROP COLUMN IF EXISTS times_used;
```

---

## Next Steps After MVP

1. **Add automated deployment** - Build the init script
2. **Add auto-replenishment** - Background task to maintain pool size
3. **Add monitoring endpoint** - `/api/v1/admin/receive-pool/stats`
4. **Add multi-network support** - Pools for each network
5. **Add address rotation** - Retire overused addresses
6. **Add metrics** - Track pool efficiency

---

## Estimated Timeline

- **Schema changes:** 5 minutes
- **Minimal pool service:** 30 minutes  
- **Manual address creation:** 20 minutes (5 addresses)
- **Deploy addresses:** 30 minutes (wait for confirmations)
- **Update order creation:** 10 minutes
- **Testing:** 30 minutes
- **Total:** ~2 hours for MVP

---

## Benefits vs. Current Approach

| Metric | Current | With Pool |
|--------|---------|-----------|
| Order creation time | 5-10s | 1-2s |
| Deployment errors | Common (AA20/AA23) | None |
| Gas costs per order | High (deployment) | None (pre-deployed) |
| Sponsorship complexity | High (initCode) | Low (simple transfer) |
| Address reusability | No | Yes (100x) |

---

## Questions?

- **Q: What if pool runs out?**  
  A: Start with 10 addresses. Each can be reused 100x = 1000 orders before needing more.

- **Q: Is it safe to reuse addresses?**  
  A: Yes, after order completion the balance is swept. Each order gets a clean slate.

- **Q: What about privacy?**  
  A: Privacy is actually better - can't correlate orders by address creation time.

- **Q: Deployment costs?**  
  A: One-time cost, Alchemy paymaster sponsors. ~$0 with current policy.
