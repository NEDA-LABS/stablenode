# Pre-Deployed Receive Address Pool - Implementation Plan

## Overview

Transform the receive address system from **"create on demand"** to **"pick from pre-deployed pool"**.

### Benefits
✅ **No deployment at transaction time** - addresses already deployed  
✅ **Solves AA20/AA23 errors** - accounts exist on-chain  
✅ **Easier sponsorship** - no initCode needed in UserOps  
✅ **Faster order creation** - just pick from pool  
✅ **Reusable addresses** - recycle after order completion  

---

## Phase 1: Database Schema Changes

### 1.1 Update ReceiveAddress Schema

**File:** `ent/schema/receiveaddress.go`

Add new fields to track pool status and deployment:

```go
func (ReceiveAddress) Fields() []ent.Field {
    return []ent.Field{
        field.String("address").Unique(),
        field.Bytes("salt").Optional(),
        
        // UPDATED: New status values for pool management
        field.Enum("status").
            Values(
                "pool_ready",      // Deployed and available in pool
                "pool_assigned",   // Assigned to an order (in use)
                "pool_processing", // Order is being processed
                "pool_completed",  // Order completed, ready for recycling
                "unused",          // Legacy: Not deployed
                "used",            // Legacy: Was used for an order
                "expired",         // Legacy: Expired
            ).
            Default("unused"),
        
        // NEW: Track deployment status
        field.Bool("is_deployed").Default(false),
        field.Int64("deployment_block").Optional(),
        field.String("deployment_tx_hash").Optional().MaxLen(70),
        field.Time("deployed_at").Optional(),
        
        // NEW: Pool management
        field.String("network_identifier").Optional(), // e.g., "base-sepolia"
        field.Int64("chain_id").Optional(),
        field.Time("assigned_at").Optional(), // When assigned to an order
        field.Time("recycled_at").Optional(), // When returned to pool
        field.Int("times_used").Default(0),   // Track reuse count
        
        // Existing fields
        field.Int64("last_indexed_block").Optional(),
        field.Time("last_used").Optional(),
        field.String("tx_hash").MaxLen(70).Optional(),
        field.Time("valid_until").Optional(),
    }
}
```

### 1.2 Add Index for Pool Queries

```go
// Indexes of the ReceiveAddress.
func (ReceiveAddress) Indexes() []ent.Index {
    return []ent.Index{
        // Fast lookup for available addresses in pool
        index.Fields("status", "is_deployed", "network_identifier"),
        
        // Fast lookup by chain
        index.Fields("chain_id", "status"),
    }
}
```

### 1.3 Migration Command

```bash
# Generate migration
go run -mod=mod entgo.io/ent/cmd/ent generate ./ent/schema

# Create Atlas migration
atlas migrate diff receive_address_pool \
  --dir "file://migrations" \
  --to "ent://ent/schema" \
  --dev-database "docker://postgres/15/test?search_path=public"
```

---

## Phase 2: Pool Management Service

### 2.1 Create Pool Service

**File:** `services/receive_address_pool.go`

```go
package services

import (
    "context"
    "crypto/rand"
    "fmt"
    "math/big"
    "time"

    "github.com/NEDA-LABS/stablenode/ent"
    "github.com/NEDA-LABS/stablenode/ent/receiveaddress"
    "github.com/NEDA-LABS/stablenode/storage"
    "github.com/NEDA-LABS/stablenode/utils/logger"
)

const (
    // Pool configuration
    MinPoolSize           = 5  // Minimum addresses in pool
    TargetPoolSize        = 10 // Target pool size
    MaxPoolSize           = 20 // Maximum pool size
    MaxAddressReuseCount  = 100 // Max times an address can be reused
)

type ReceiveAddressPoolService struct {
    alchemyService *AlchemyService
}

func NewReceiveAddressPoolService() *ReceiveAddressPoolService {
    return &ReceiveAddressPoolService{
        alchemyService: NewAlchemyService(),
    }
}

// GetAvailableAddress gets a random available address from the pool
// Uses database-level locking to prevent race conditions
func (s *ReceiveAddressPoolService) GetAvailableAddress(ctx context.Context, chainID int64, networkIdentifier string) (*ent.ReceiveAddress, error) {
    // Start transaction for atomic operation
    tx, err := storage.Client.Tx(ctx)
    if err != nil {
        return nil, fmt.Errorf("failed to start transaction: %w", err)
    }
    defer tx.Rollback()

    // Find available addresses with FOR UPDATE lock
    addresses, err := tx.ReceiveAddress.
        Query().
        Where(
            receiveaddress.StatusEQ(receiveaddress.StatusPoolReady),
            receiveaddress.IsDeployedEQ(true),
            receiveaddress.ChainIDEQ(chainID),
            receiveaddress.NetworkIdentifierEQ(networkIdentifier),
            receiveaddress.TimesUsedLT(MaxAddressReuseCount),
        ).
        ForUpdate().
        All(ctx)

    if err != nil {
        return nil, fmt.Errorf("failed to query available addresses: %w", err)
    }

    if len(addresses) == 0 {
        return nil, fmt.Errorf("no available addresses in pool for chain %d", chainID)
    }

    // Pick random address from available pool
    randomIndex, err := rand.Int(rand.Reader, big.NewInt(int64(len(addresses))))
    if err != nil {
        return nil, fmt.Errorf("failed to generate random index: %w", err)
    }
    selectedAddress := addresses[randomIndex.Int64()]

    // Mark as assigned
    selectedAddress, err = selectedAddress.Update().
        SetStatus(receiveaddress.StatusPoolAssigned).
        SetAssignedAt(time.Now()).
        SetTimesUsed(selectedAddress.TimesUsed + 1).
        Save(ctx)
    if err != nil {
        return nil, fmt.Errorf("failed to mark address as assigned: %w", err)
    }

    // Commit transaction
    if err := tx.Commit(); err != nil {
        return nil, fmt.Errorf("failed to commit transaction: %w", err)
    }

    logger.WithFields(logger.Fields{
        "Address":     selectedAddress.Address,
        "ChainID":     chainID,
        "TimesUsed":   selectedAddress.TimesUsed,
        "AvailableInPool": len(addresses) - 1,
    }).Info("Assigned receive address from pool")

    // Check if pool needs replenishment (async)
    go s.MaintainPoolSize(context.Background(), chainID, networkIdentifier)

    return selectedAddress, nil
}

// RecycleAddress returns an address to the pool after order completion
func (s *ReceiveAddressPoolService) RecycleAddress(ctx context.Context, addressID int) error {
    address, err := storage.Client.ReceiveAddress.Get(ctx, addressID)
    if err != nil {
        return fmt.Errorf("failed to get address: %w", err)
    }

    // Update status to ready for reuse
    _, err = address.Update().
        SetStatus(receiveaddress.StatusPoolReady).
        SetRecycledAt(time.Now()).
        Save(ctx)
    if err != nil {
        return fmt.Errorf("failed to recycle address: %w", err)
    }

    logger.WithFields(logger.Fields{
        "Address":   address.Address,
        "TimesUsed": address.TimesUsed,
    }).Info("Recycled receive address back to pool")

    return nil
}

// InitializePool creates and deploys initial pool of addresses
func (s *ReceiveAddressPoolService) InitializePool(ctx context.Context, chainID int64, networkIdentifier string, size int) error {
    logger.WithFields(logger.Fields{
        "ChainID":           chainID,
        "NetworkIdentifier": networkIdentifier,
        "TargetSize":        size,
    }).Info("Initializing receive address pool")

    for i := 0; i < size; i++ {
        if err := s.CreateAndDeployAddress(ctx, chainID, networkIdentifier); err != nil {
            logger.WithFields(logger.Fields{
                "Error":  err.Error(),
                "Index":  i,
                "ChainID": chainID,
            }).Error("Failed to create address in pool")
            continue
        }

        logger.WithFields(logger.Fields{
            "Progress": fmt.Sprintf("%d/%d", i+1, size),
        }).Info("Pool initialization progress")
    }

    return nil
}

// CreateAndDeployAddress creates a new address and deploys it on-chain
func (s *ReceiveAddressPoolService) CreateAndDeployAddress(ctx context.Context, chainID int64, networkIdentifier string) error {
    // Create smart account address
    ownerAddress := "0xFb84E5503bD20526f2579193411Dd0993d08077519b6f7" // System owner
    address, encryptedSalt, err := s.alchemyService.CreateSmartAccount(ctx, chainID, ownerAddress)
    if err != nil {
        return fmt.Errorf("failed to create smart account: %w", err)
    }

    // Deploy the account on-chain using paymaster
    deploymentTxHash, deploymentBlock, err := s.deploySmartAccount(ctx, chainID, address, encryptedSalt, ownerAddress)
    if err != nil {
        return fmt.Errorf("failed to deploy smart account: %w", err)
    }

    // Save to database as pool-ready
    _, err = storage.Client.ReceiveAddress.
        Create().
        SetAddress(address).
        SetSalt(encryptedSalt).
        SetStatus(receiveaddress.StatusPoolReady).
        SetIsDeployed(true).
        SetDeploymentTxHash(deploymentTxHash).
        SetDeploymentBlock(deploymentBlock).
        SetDeployedAt(time.Now()).
        SetChainID(chainID).
        SetNetworkIdentifier(networkIdentifier).
        SetTimesUsed(0).
        Save(ctx)
    if err != nil {
        return fmt.Errorf("failed to save address to pool: %w", err)
    }

    logger.WithFields(logger.Fields{
        "Address":       address,
        "ChainID":       chainID,
        "DeploymentTx":  deploymentTxHash,
        "DeploymentBlock": deploymentBlock,
    }).Info("Created and deployed address to pool")

    return nil
}

// deploySmartAccount deploys the smart account with paymaster sponsorship
func (s *ReceiveAddressPoolService) deploySmartAccount(ctx context.Context, chainID int64, address string, encryptedSalt []byte, ownerAddress string) (string, int64, error) {
    // This will use the existing deployment logic from AlchemyService
    // Returns: txHash, blockNumber, error
    
    // Implementation: Use SendUserOperationWithDeployment from alchemy.go
    txHash, err := s.alchemyService.SendUserOperationWithDeployment(ctx, chainID, address, encryptedSalt, ownerAddress, "0x") // Empty callData
    if err != nil {
        return "", 0, err
    }

    // Wait for deployment confirmation
    receipt, err := s.alchemyService.GetUserOperationReceipt(ctx, chainID, txHash)
    if err != nil {
        return "", 0, err
    }

    blockNumber := int64(0)
    if bn, ok := receipt["blockNumber"].(string); ok {
        fmt.Sscanf(bn, "0x%x", &blockNumber)
    }

    return txHash, blockNumber, nil
}

// MaintainPoolSize ensures the pool has enough addresses
func (s *ReceiveAddressPoolService) MaintainPoolSize(ctx context.Context, chainID int64, networkIdentifier string) {
    count, err := storage.Client.ReceiveAddress.
        Query().
        Where(
            receiveaddress.StatusEQ(receiveaddress.StatusPoolReady),
            receiveaddress.ChainIDEQ(chainID),
            receiveaddress.NetworkIdentifierEQ(networkIdentifier),
        ).
        Count(ctx)
    if err != nil {
        logger.WithFields(logger.Fields{
            "Error":   err.Error(),
            "ChainID": chainID,
        }).Error("Failed to check pool size")
        return
    }

    if count < MinPoolSize {
        toCreate := TargetPoolSize - count
        logger.WithFields(logger.Fields{
            "CurrentSize": count,
            "TargetSize":  TargetPoolSize,
            "ToCreate":    toCreate,
            "ChainID":     chainID,
        }).Info("Pool size below minimum, replenishing")

        for i := 0; i < toCreate; i++ {
            if err := s.CreateAndDeployAddress(ctx, chainID, networkIdentifier); err != nil {
                logger.WithFields(logger.Fields{
                    "Error": err.Error(),
                }).Error("Failed to create address during pool maintenance")
            }
        }
    }
}

// GetPoolStats returns statistics about the address pool
func (s *ReceiveAddressPoolService) GetPoolStats(ctx context.Context, chainID int64) (map[string]int, error) {
    stats := make(map[string]int)

    // Count by status
    statuses := []receiveaddress.Status{
        receiveaddress.StatusPoolReady,
        receiveaddress.StatusPoolAssigned,
        receiveaddress.StatusPoolProcessing,
        receiveaddress.StatusPoolCompleted,
    }

    for _, status := range statuses {
        count, err := storage.Client.ReceiveAddress.
            Query().
            Where(
                receiveaddress.StatusEQ(status),
                receiveaddress.ChainIDEQ(chainID),
            ).
            Count(ctx)
        if err != nil {
            return nil, err
        }
        stats[string(status)] = count
    }

    return stats, nil
}
```

---

## Phase 3: Update Order Creation

### 3.1 Modify Sender Controller

**File:** `controllers/sender/sender.go`

Replace the `CreateSmartAddress` call with pool lookup:

```go
// OLD CODE (remove):
// receiveAddress, err := ctrl.receiveAddressService.CreateSmartAddress(ctx, order.TokenID.String())

// NEW CODE:
poolService := services.NewReceiveAddressPoolService()
receiveAddress, err := poolService.GetAvailableAddress(ctx, token.Edges.Network.ChainID, token.Edges.Network.Identifier)
if err != nil {
    logger.Errorf("Error: Failed to get receive address from pool: %v", err)
    u.APIResponse(ctx, http.StatusInternalServerError, "error", "Failed to create payment order", nil)
    return
}
```

---

## Phase 4: Update Indexer/Polling

### 4.1 No Changes Needed!

The indexer already monitors receive addresses by their address, so it will work the same way:

**File:** `services/common/indexer.go` (no changes)
```go
// This code already works with the pool approach
receiveAddress, err := storage.Client.ReceiveAddress.
    Query().
    Where(receiveaddress.AddressEQ(toAddress)).
    WithPaymentOrder(...).
    Only(ctx)
```

### 4.2 Add Recycling After Order Completion

**File:** `services/order/evm.go` or wherever orders are marked as settled

```go
// After order is settled/completed
if order.Edges.ReceiveAddress != nil {
    poolService := services.NewReceiveAddressPoolService()
    if err := poolService.RecycleAddress(ctx, order.Edges.ReceiveAddress.ID); err != nil {
        logger.WithFields(logger.Fields{
            "Error": err.Error(),
            "OrderID": order.ID,
            "ReceiveAddress": order.Edges.ReceiveAddress.Address,
        }).Warn("Failed to recycle receive address")
    }
}
```

---

## Phase 5: Initialization Script

### 5.1 Create Pool Init Command

**File:** `cmd/init_receive_pool/main.go`

```go
package main

import (
    "context"
    "flag"
    "log"

    "github.com/NEDA-LABS/stablenode/config"
    "github.com/NEDA-LABS/stablenode/services"
    "github.com/NEDA-LABS/stablenode/storage"
)

func main() {
    chainID := flag.Int64("chain-id", 84532, "Chain ID (default: Base Sepolia)")
    network := flag.String("network", "base-sepolia", "Network identifier")
    size := flag.Int("size", 10, "Number of addresses to create")
    flag.Parse()

    // Initialize storage
    if err := storage.Init(); err != nil {
        log.Fatalf("Failed to initialize storage: %v", err)
    }
    defer storage.Client.Close()

    // Initialize pool
    poolService := services.NewReceiveAddressPoolService()
    ctx := context.Background()

    log.Printf("Initializing pool with %d addresses for chain %d (%s)", *size, *chainID, *network)

    if err := poolService.InitializePool(ctx, *chainID, *network, *size); err != nil {
        log.Fatalf("Failed to initialize pool: %v", err)
    }

    log.Println("Pool initialization complete!")

    // Show stats
    stats, err := poolService.GetPoolStats(ctx, *chainID)
    if err != nil {
        log.Fatalf("Failed to get stats: %v", err)
    }

    log.Printf("Pool stats: %+v", stats)
}
```

### 5.2 Run Initialization

```bash
# Build
go build -o bin/init_receive_pool ./cmd/init_receive_pool

# Run for Base Sepolia
./bin/init_receive_pool --chain-id=84532 --network=base-sepolia --size=10

# Run for other networks as needed
```

---

## Phase 6: Background Maintenance Task

### 6.1 Add to Tasks Service

**File:** `tasks/tasks.go`

```go
// Add to task scheduler
func (t *Tasks) MaintainReceiveAddressPool(ctx context.Context) {
    logger.Info("MaintainReceiveAddressPool started")

    poolService := services.NewReceiveAddressPoolService()

    // Get all active networks
    networks, err := storage.Client.Network.Query().All(ctx)
    if err != nil {
        logger.Errorf("Failed to get networks: %v", err)
        return
    }

    for _, network := range networks {
        // Check and maintain pool for each network
        poolService.MaintainPoolSize(ctx, network.ChainID, network.Identifier)

        // Show stats
        stats, err := poolService.GetPoolStats(ctx, network.ChainID)
        if err != nil {
            logger.Errorf("Failed to get pool stats: %v", err)
            continue
        }

        logger.WithFields(logger.Fields{
            "Network": network.Identifier,
            "ChainID": network.ChainID,
            "Stats":   stats,
        }).Info("Receive address pool status")
    }

    logger.Info("MaintainReceiveAddressPool completed")
}

// Add to scheduler (runs every 10 minutes)
func (t *Tasks) StartScheduler(ctx context.Context) {
    // ... existing tasks ...

    // Pool maintenance
    c.AddFunc("*/10 * * * *", func() {
        t.MaintainReceiveAddressPool(ctx)
    })
}
```

---

## Phase 7: Monitoring & Admin Endpoints

### 7.1 Add Pool Status Endpoint

**File:** `controllers/index.go`

```go
// GetReceiveAddressPoolStats returns pool statistics
func (ctrl *Controller) GetReceiveAddressPoolStats(ctx *gin.Context) {
    chainIDStr := ctx.Query("chain_id")
    if chainIDStr == "" {
        u.APIResponse(ctx, http.StatusBadRequest, "error", "chain_id required", nil)
        return
    }

    chainID, err := strconv.ParseInt(chainIDStr, 10, 64)
    if err != nil {
        u.APIResponse(ctx, http.StatusBadRequest, "error", "Invalid chain_id", nil)
        return
    }

    poolService := svc.NewReceiveAddressPoolService()
    stats, err := poolService.GetPoolStats(context.Background(), chainID)
    if err != nil {
        u.APIResponse(ctx, http.StatusInternalServerError, "error", "Failed to get pool stats", nil)
        return
    }

    u.APIResponse(ctx, http.StatusOK, "success", "Pool stats retrieved", stats)
}
```

### 7.2 Add Route

**File:** `routers/index.go`

```go
// Admin routes
admin := v1.Group("admin")
{
    admin.GET("receive-pool/stats", ctrl.GetReceiveAddressPoolStats)
}
```

---

## Implementation Checklist

### Database
- [ ] Update `ent/schema/receiveaddress.go` with new fields
- [ ] Generate ent code: `go generate ./ent`
- [ ] Create migration: `atlas migrate diff`
- [ ] Run migration: `atlas migrate apply`

### Services
- [ ] Create `services/receive_address_pool.go`
- [ ] Update `controllers/sender/sender.go` to use pool
- [ ] Add recycling logic to order completion
- [ ] Add pool maintenance to `tasks/tasks.go`

### Initialization
- [ ] Create `cmd/init_receive_pool/main.go`
- [ ] Run initialization for each network
- [ ] Verify deployments on-chain

### Testing
- [ ] Test order creation with pool address
- [ ] Test payment to pool address
- [ ] Test address recycling
- [ ] Test concurrent order creation (no conflicts)
- [ ] Monitor pool size maintenance

### Monitoring
- [ ] Add pool stats endpoint
- [ ] Add logging for pool operations
- [ ] Set up alerts for low pool size

---

## Migration Strategy

### Step 1: Deploy Schema Changes (Zero Downtime)
```bash
# Run migration - adds new fields with defaults
atlas migrate apply --url "postgres://..."
```

### Step 2: Initialize Pool
```bash
# Create and deploy 10 addresses per network
./bin/init_receive_pool --chain-id=84532 --network=base-sepolia --size=10
```

### Step 3: Deploy New Code (Feature Flag)
```go
// Use environment variable for gradual rollout
if os.Getenv("USE_RECEIVE_ADDRESS_POOL") == "true" {
    // Use pool
} else {
    // Use old method
}
```

### Step 4: Monitor & Validate
- Watch for pool depletion
- Verify addresses work correctly
- Check gas sponsorship working

### Step 5: Full Rollout
- Remove feature flag
- Remove old `CreateSmartAddress` code

---

## Cost Analysis

### Deployment Costs
- **10 addresses × 5 networks = 50 deployments**
- **Estimated gas per deployment:** ~100k-300k gas
- **With paymaster sponsorship:** $0 (Alchemy sponsors)
- **One-time cost:** Already paid

### Ongoing Costs
- **Address reuse:** ~100 times per address
- **Maintenance:** Auto-replenish when pool < 5
- **Net savings:** No deployment gas at transaction time

---

## Security Considerations

1. **Concurrent Access:** Database `FOR UPDATE` lock prevents race conditions
2. **Address Reuse Limit:** Max 100 uses per address prevents correlation attacks
3. **Pool Exhaustion:** Auto-maintenance ensures availability
4. **Orphaned Addresses:** Background task recycles completed orders

---

## Troubleshooting

### Pool Depleted
```bash
# Manually add more addresses
./bin/init_receive_pool --chain-id=84532 --network=base-sepolia --size=5
```

### Address Stuck in "Assigned"
```sql
-- Find stuck addresses (assigned > 1 hour ago)
SELECT * FROM receive_addresses 
WHERE status = 'pool_assigned' 
AND assigned_at < NOW() - INTERVAL '1 hour';

-- Manually recycle
UPDATE receive_addresses 
SET status = 'pool_ready', recycled_at = NOW() 
WHERE id = <id>;
```

### Check Pool Health
```bash
curl "http://localhost:8080/api/v1/admin/receive-pool/stats?chain_id=84532"
```
