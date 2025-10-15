# Polling Fallback Implementation Guide

## Overview

When Alchemy webhooks are not available or fail, a **polling mechanism** can detect payments by periodically checking receive address balances.

## Why Polling?

**Use Cases:**
- Webhook setup not yet complete
- Webhook endpoint temporarily down
- Network issues preventing webhook delivery
- Testing/development without public URL
- Backup for webhook failures

**Trade-offs:**
- ✅ Simple to implement
- ✅ No external dependencies
- ✅ Works without public URL
- ❌ Delayed detection (polling interval)
- ❌ More RPC calls (costs)
- ❌ Higher server load

---

## Implementation Options

### **Option 1: Background Job (Recommended)**

Poll receive addresses periodically using a background worker.

### **Option 2: On-Demand Polling**

Check balance when user requests order status.

### **Option 3: Hybrid**

Use webhooks as primary, polling as fallback.

---

## Option 1: Background Job Implementation

### **Step 1: Create Polling Service**

**File**: `services/polling_service.go`

```go
package services

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/shopspring/decimal"
	"github.com/NEDA-LABS/stablenode/ent"
	"github.com/NEDA-LABS/stablenode/ent/paymentorder"
	"github.com/NEDA-LABS/stablenode/ent/receiveaddress"
	"github.com/NEDA-LABS/stablenode/storage"
	"github.com/NEDA-LABS/stablenode/utils/logger"
)

// PollingService handles periodic balance checking for receive addresses
type PollingService struct {
	alchemyService *AlchemyService
	interval       time.Duration
	stopChan       chan bool
}

// NewPollingService creates a new polling service
func NewPollingService(interval time.Duration) *PollingService {
	return &PollingService{
		alchemyService: NewAlchemyService(),
		interval:       interval,
		stopChan:       make(chan bool),
	}
}

// Start begins the polling loop
func (s *PollingService) Start(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	logger.Infof("Starting polling service with interval: %v", s.interval)

	for {
		select {
		case <-ticker.C:
			s.pollPendingOrders(ctx)
		case <-s.stopChan:
			logger.Infof("Stopping polling service")
			return
		case <-ctx.Done():
			logger.Infof("Context cancelled, stopping polling service")
			return
		}
	}
}

// Stop stops the polling service
func (s *PollingService) Stop() {
	close(s.stopChan)
}

// pollPendingOrders checks all pending orders for payments
func (s *PollingService) pollPendingOrders(ctx context.Context) {
	// Get all orders in 'initiated' status
	orders, err := storage.Client.PaymentOrder.
		Query().
		Where(
			paymentorder.StatusEQ(paymentorder.StatusInitiated),
			paymentorder.HasReceiveAddress(),
		).
		WithReceiveAddress().
		WithToken(func(q *ent.TokenQuery) {
			q.WithNetwork()
		}).
		All(ctx)

	if err != nil {
		logger.Errorf("Failed to fetch pending orders: %v", err)
		return
	}

	logger.Infof("Polling %d pending orders", len(orders))

	for _, order := range orders {
		s.checkOrderPayment(ctx, order)
	}
}

// checkOrderPayment checks if payment has been received for an order
func (s *PollingService) checkOrderPayment(ctx context.Context, order *ent.PaymentOrder) {
	receiveAddr := order.Edges.ReceiveAddress
	token := order.Edges.Token
	network := token.Edges.Network

	// Check if receive address is expired
	if time.Now().After(receiveAddr.ValidUntil) {
		logger.WithFields(logger.Fields{
			"OrderID": order.ID,
			"Address": receiveAddr.Address,
		}).Warnf("Receive address expired, skipping")
		return
	}

	// Get balance from blockchain
	balance, err := s.getTokenBalance(ctx, network.RPCEndpoint, receiveAddr.Address, token.ContractAddress, token.Decimals)
	if err != nil {
		logger.WithFields(logger.Fields{
			"OrderID": order.ID,
			"Address": receiveAddr.Address,
			"Error":   err,
		}).Errorf("Failed to get balance")
		return
	}

	// Check if balance has changed
	if balance.GreaterThan(order.AmountPaid) {
		logger.WithFields(logger.Fields{
			"OrderID":     order.ID,
			"Address":     receiveAddr.Address,
			"OldBalance":  order.AmountPaid,
			"NewBalance":  balance,
		}).Infof("Payment detected via polling")

		// Update order
		err = s.updateOrderPayment(ctx, order, balance)
		if err != nil {
			logger.Errorf("Failed to update order payment: %v", err)
		}
	}
}

// getTokenBalance gets the ERC-20 token balance for an address
func (s *PollingService) getTokenBalance(ctx context.Context, rpcURL, address, tokenContract string, decimals int) (decimal.Decimal, error) {
	// Connect to RPC
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return decimal.Zero, fmt.Errorf("failed to connect to RPC: %w", err)
	}
	defer client.Close()

	// ERC-20 balanceOf function signature
	// balanceOf(address) returns (uint256)
	data := common.Hex2Bytes("70a08231" + "000000000000000000000000" + address[2:])

	// Call contract
	msg := ethereum.CallMsg{
		To:   &common.Address{},
		Data: data,
	}
	
	// Set contract address
	contractAddr := common.HexToAddress(tokenContract)
	msg.To = &contractAddr

	result, err := client.CallContract(ctx, msg, nil)
	if err != nil {
		return decimal.Zero, fmt.Errorf("failed to call contract: %w", err)
	}

	// Parse result
	balance := new(big.Int).SetBytes(result)
	balanceDecimal := decimal.NewFromBigInt(balance, 0)

	// Convert to human-readable amount
	divisor := decimal.NewFromInt(10).Pow(decimal.NewFromInt(int64(decimals)))
	humanBalance := balanceDecimal.Div(divisor)

	return humanBalance, nil
}

// updateOrderPayment updates the order with the new payment amount
func (s *PollingService) updateOrderPayment(ctx context.Context, order *ent.PaymentOrder, amount decimal.Decimal) error {
	// Update amount_paid
	_, err := order.Update().
		SetAmountPaid(amount).
		Save(ctx)

	if err != nil {
		return fmt.Errorf("failed to update order: %w", err)
	}

	// Check if payment is sufficient
	totalRequired := order.Amount.Add(order.SenderFee).Add(order.NetworkFee).Add(order.ProtocolFee)
	
	if amount.GreaterThanOrEqual(totalRequired) {
		logger.WithFields(logger.Fields{
			"OrderID":       order.ID,
			"AmountPaid":    amount,
			"AmountRequired": totalRequired,
		}).Infof("Payment sufficient, order ready for fulfillment")

		// TODO: Trigger order fulfillment
		// This would call the same logic as webhook handler
	}

	return nil
}
```

### **Step 2: Start Polling Service**

**File**: `main.go`

```go
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/NEDA-LABS/stablenode/services"
	"github.com/NEDA-LABS/stablenode/utils/logger"
	"github.com/spf13/viper"
)

func main() {
	// ... existing setup code ...

	// Start polling service if enabled
	if viper.GetBool("ENABLE_POLLING_FALLBACK") {
		pollingInterval := viper.GetDuration("POLLING_INTERVAL")
		if pollingInterval == 0 {
			pollingInterval = 30 * time.Second // Default: 30 seconds
		}

		pollingService := services.NewPollingService(pollingInterval)
		
		// Start in background
		go pollingService.Start(context.Background())

		// Graceful shutdown
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		
		go func() {
			<-sigChan
			logger.Infof("Shutting down polling service...")
			pollingService.Stop()
		}()
	}

	// ... rest of main code ...
}
```

### **Step 3: Configuration**

**File**: `.env`

```bash
# Polling Configuration
ENABLE_POLLING_FALLBACK=true
POLLING_INTERVAL=30s  # Check every 30 seconds

# Alternative intervals:
# POLLING_INTERVAL=1m   # Every 1 minute (recommended)
# POLLING_INTERVAL=5m   # Every 5 minutes (low frequency)
# POLLING_INTERVAL=10s  # Every 10 seconds (high frequency, more RPC calls)
```

---

## Option 2: On-Demand Polling

Check balance when user requests order status.

### **Implementation**

**File**: `controllers/sender/sender.go`

```go
// GetOrderStatus returns the current order status with balance check
func (ctrl *SenderController) GetOrderStatus(ctx *gin.Context) {
	orderID := ctx.Param("id")

	// Get order from database
	order, err := storage.Client.PaymentOrder.
		Query().
		Where(paymentorder.IDEQ(uuid.MustParse(orderID))).
		WithReceiveAddress().
		WithToken(func(q *ent.TokenQuery) {
			q.WithNetwork()
		}).
		Only(ctx)

	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
		return
	}

	// If order is pending, check balance
	if order.Status == paymentorder.StatusInitiated {
		pollingService := svc.NewPollingService(0)
		pollingService.checkOrderPayment(ctx, order)

		// Refresh order from database
		order, _ = storage.Client.PaymentOrder.Get(ctx, order.ID)
	}

	ctx.JSON(http.StatusOK, gin.H{
		"data": order,
	})
}
```

---

## Option 3: Hybrid Approach (Recommended)

Use webhooks as primary, polling as fallback.

### **Strategy**

1. **Primary**: Alchemy webhooks (instant, efficient)
2. **Fallback**: Polling for orders older than 5 minutes without payment
3. **Cleanup**: Stop polling once webhook is confirmed working

### **Implementation**

```go
func (s *PollingService) pollPendingOrders(ctx context.Context) {
	// Only poll orders that:
	// 1. Are in 'initiated' status
	// 2. Created more than 5 minutes ago (webhook should have fired by then)
	// 3. Don't have a recent webhook event

	fiveMinutesAgo := time.Now().Add(-5 * time.Minute)

	orders, err := storage.Client.PaymentOrder.
		Query().
		Where(
			paymentorder.StatusEQ(paymentorder.StatusInitiated),
			paymentorder.CreatedAtLT(fiveMinutesAgo),
			paymentorder.HasReceiveAddress(),
		).
		WithReceiveAddress().
		WithToken(func(q *ent.TokenQuery) {
			q.WithNetwork()
		}).
		All(ctx)

	// ... rest of polling logic
}
```

---

## Performance Optimization

### **1. Batch RPC Calls**

Instead of calling RPC for each order individually, batch them:

```go
// Use Alchemy's batch JSON-RPC
func (s *PollingService) batchGetBalances(ctx context.Context, addresses []string, tokenContract string, rpcURL string) (map[string]decimal.Decimal, error) {
	// Prepare batch request
	requests := make([]map[string]interface{}, len(addresses))
	
	for i, addr := range addresses {
		data := "0x70a08231" + "000000000000000000000000" + addr[2:]
		requests[i] = map[string]interface{}{
			"jsonrpc": "2.0",
			"method":  "eth_call",
			"params": []interface{}{
				map[string]string{
					"to":   tokenContract,
					"data": data,
				},
				"latest",
			},
			"id": i + 1,
		}
	}

	// Send batch request
	// ... HTTP POST to RPC with batch requests
	// ... Parse batch response
	
	return balances, nil
}
```

### **2. Cache Results**

Cache balances to reduce RPC calls:

```go
type BalanceCache struct {
	balances map[string]CachedBalance
	mutex    sync.RWMutex
	ttl      time.Duration
}

type CachedBalance struct {
	Amount    decimal.Decimal
	Timestamp time.Time
}

func (c *BalanceCache) Get(address string) (decimal.Decimal, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	cached, exists := c.balances[address]
	if !exists {
		return decimal.Zero, false
	}

	// Check if cache is still valid
	if time.Since(cached.Timestamp) > c.ttl {
		return decimal.Zero, false
	}

	return cached.Amount, true
}
```

### **3. Prioritize Recent Orders**

Poll recent orders more frequently:

```go
func (s *PollingService) getPollInterval(order *ent.PaymentOrder) time.Duration {
	age := time.Since(order.CreatedAt)

	switch {
	case age < 5*time.Minute:
		return 10 * time.Second // Recent orders: every 10s
	case age < 30*time.Minute:
		return 1 * time.Minute  // Medium age: every 1m
	default:
		return 5 * time.Minute  // Old orders: every 5m
	}
}
```

---

## Cost Analysis

### **RPC Call Costs**

Assuming 100 pending orders:

| Interval | Calls/Hour | Calls/Day | Alchemy Free Tier |
|----------|------------|-----------|-------------------|
| 10s | 36,000 | 864,000 | ❌ Exceeds limit |
| 30s | 12,000 | 288,000 | ⚠️ Close to limit |
| 1m | 6,000 | 144,000 | ✅ Within limit |
| 5m | 1,200 | 28,800 | ✅ Well within |

**Recommendation**: Use 1-minute interval with batching.

---

## Monitoring & Alerts

### **Metrics to Track**

```go
type PollingMetrics struct {
	OrdersChecked     int64
	PaymentsDetected  int64
	RPCCallsMade      int64
	ErrorsEncountered int64
	AverageCheckTime  time.Duration
}

func (s *PollingService) recordMetrics() {
	// Log metrics every 5 minutes
	ticker := time.NewTicker(5 * time.Minute)
	
	for range ticker.C {
		logger.WithFields(logger.Fields{
			"orders_checked":     s.metrics.OrdersChecked,
			"payments_detected":  s.metrics.PaymentsDetected,
			"rpc_calls":          s.metrics.RPCCallsMade,
			"errors":             s.metrics.ErrorsEncountered,
			"avg_check_time":     s.metrics.AverageCheckTime,
		}).Infof("Polling service metrics")
	}
}
```

### **Alerts**

Set up alerts for:
- High error rate (> 10%)
- No payments detected for > 1 hour
- RPC call failures
- Slow response times

---

## Testing

### **Unit Test**

```go
func TestPollingService_CheckOrderPayment(t *testing.T) {
	// Mock RPC server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return mock balance
		response := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"result":  "0x0de0b6b3a7640000", // 1 token in hex
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer mockServer.Close()

	// Test polling
	service := NewPollingService(1 * time.Minute)
	// ... test logic
}
```

### **Integration Test**

```bash
# Test with real testnet
ENABLE_POLLING_FALLBACK=true \
POLLING_INTERVAL=10s \
go run main.go
```

---

## Migration Strategy

### **Phase 1: Enable Polling (Week 1)**
- Deploy polling service
- Monitor metrics
- Verify payment detection works

### **Phase 2: Webhook Setup (Week 2)**
- Set up Alchemy webhooks
- Run both webhook + polling
- Compare detection times

### **Phase 3: Optimize (Week 3)**
- Reduce polling frequency
- Use polling only as fallback
- Monitor cost savings

### **Phase 4: Production (Week 4)**
- Webhooks primary
- Polling for orders > 5 minutes old
- Full monitoring in place

---

## Comparison: Webhook vs Polling

| Feature | Webhook | Polling |
|---------|---------|---------|
| **Detection Speed** | Instant (< 1s) | Delayed (interval) |
| **RPC Calls** | None | High |
| **Cost** | Free | Moderate |
| **Reliability** | Depends on network | High |
| **Setup Complexity** | Medium | Low |
| **Public URL Required** | Yes | No |
| **Best For** | Production | Development/Fallback |

---

## Recommendation

**Use Hybrid Approach:**

1. **Primary**: Alchemy webhooks for instant detection
2. **Fallback**: Polling every 1-2 minutes for orders > 5 minutes old
3. **Optimization**: Batch RPC calls, cache results, prioritize recent orders

This gives you:
- ✅ Fast detection (webhooks)
- ✅ Reliability (polling fallback)
- ✅ Cost efficiency (minimal polling)
- ✅ Works during webhook outages

---

**Next Steps:**
1. Implement `PollingService` in `services/polling_service.go`
2. Add configuration to `.env`
3. Start service in `main.go`
4. Test with testnet orders
5. Monitor metrics and adjust interval

---

**Last Updated**: 2025-10-09
**Status**: Implementation guide complete
