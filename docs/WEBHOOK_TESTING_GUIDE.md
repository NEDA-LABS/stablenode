# Webhook + Polling Testing Guide

## Overview

This guide walks you through testing the complete payment detection system with webhooks as primary and polling as fallback.

---

## Prerequisites

### 1. Configuration
```bash
# .env
ALCHEMY_API_KEY=your_api_key
ALCHEMY_AUTH_TOKEN=your_auth_token
USE_ALCHEMY_FOR_RECEIVE_ADDRESSES=true

# Polling fallback
ENABLE_POLLING_FALLBACK=true
POLLING_INTERVAL=1m
POLLING_MIN_AGE=5m
POLLING_CACHE_TTL=30s
```

### 2. Services Running
```bash
# Start application
docker-compose up -d

# Check logs
docker logs -f aggregator_app_1
```

---

## Test Scenarios

### **Scenario 1: Webhook Success (Happy Path)** ✅

**Goal**: Verify webhooks detect payments instantly

**Steps**:
1. Create test order
2. Send crypto to receive address
3. Webhook fires within 1 second
4. Order updated immediately

**Expected Behavior**:
- ✅ Webhook fires < 1 second after transaction
- ✅ `amount_paid` updated
- ✅ Order status changes to `validated`
- ✅ Polling service does NOT detect (order < 5 minutes old)

**Test**:
```bash
# 1. Create order
curl -X POST http://localhost:8000/v1/sender/orders \
  -H "Content-Type: application/json" \
  -H "X-API-Key: 11f93de0-d304-4498-8b7b-6cecbc5b2dd8" \
  -d '{
    "amount": 0.5,
    "token": "DAI",
    "rate": 1482.3,
    "network": "base-sepolia",
    "recipient": {
      "institution": "ABNGNGLA",
      "accountIdentifier": "0123456789",
      "accountName": "John Doe",
      "currency": "NGN"
    },
    "reference": "WEBHOOK-TEST-001",
    "returnAddress": "0x18000433c7cc39ebdAbB06262F88795960FE5Cf9"
  }'

# 2. Note the receive_address from response

# 3. Send 0.5 DAI to receive address using MetaMask/wallet

# 4. Watch logs for webhook
docker logs -f aggregator_app_1 | grep -i webhook

# Expected log:
# "POST /v1/alchemy/webhook" - Webhook received
# "Payment detected via webhook"
# "amount_paid updated: 0.5"

# 5. Verify order status
curl http://localhost:8000/v1/sender/orders/{order_id} \
  -H "X-API-Key: 11f93de0-d304-4498-8b7b-6cecbc5b2dd8"

# Expected response:
# {
#   "amount_paid": "0.5",
#   "status": "validated"
# }
```

---

### **Scenario 2: Webhook Failure → Polling Fallback** 🔄

**Goal**: Verify polling detects payment when webhook fails

**Steps**:
1. Create order
2. Temporarily disable webhook endpoint
3. Send crypto
4. Wait 5+ minutes
5. Polling detects payment

**Expected Behavior**:
- ❌ Webhook fails (endpoint down)
- ⏰ Order stays in `initiated` for 5 minutes
- ✅ Polling service detects payment after 5 minutes
- ✅ Order updated via polling

**Test**:
```bash
# 1. Create order (same as Scenario 1)

# 2. Simulate webhook failure by commenting out webhook route
# In routers/routes.go, comment:
# router.POST("/v1/alchemy/webhook", controller.AlchemyWebhook)

# 3. Restart app
docker-compose restart app

# 4. Send crypto to receive address

# 5. Wait 5 minutes (POLLING_MIN_AGE)

# 6. Watch polling logs
docker logs -f aggregator_app_1 | grep -i polling

# Expected logs after 5 minutes:
# "Polling pending orders (fallback mode)" - count: 1
# "💰 Payment detected via polling fallback"
# "✅ Payment sufficient, order ready for fulfillment"

# 7. Verify order updated
curl http://localhost:8000/v1/sender/orders/{order_id}

# Expected:
# {
#   "amount_paid": "0.5",
#   "status": "validated"
# }
```

---

### **Scenario 3: Webhook Delay → Polling Catches** ⚡

**Goal**: Verify polling catches payments if webhook is delayed

**Steps**:
1. Create order
2. Send crypto
3. Webhook delayed > 5 minutes (network issues)
4. Polling detects first

**Expected Behavior**:
- ⏰ Webhook delayed
- ✅ Polling detects after 5 minutes
- ✅ Order updated
- ℹ️ Webhook fires later but order already updated (idempotent)

**Test**:
```bash
# Simulate by:
# 1. Creating order
# 2. Sending crypto
# 3. Immediately blocking webhook endpoint for 6 minutes
# 4. Unblocking after polling has detected

# This tests idempotency - both webhook and polling try to update
```

---

### **Scenario 4: Multiple Orders** 📊

**Goal**: Verify system handles multiple orders correctly

**Steps**:
1. Create 5 orders
2. Send crypto to 3 of them
3. Verify correct orders updated

**Expected Behavior**:
- ✅ Each order tracked independently
- ✅ Only paid orders updated
- ✅ No cross-contamination

**Test**:
```bash
# Create 5 orders
for i in {1..5}; do
  curl -X POST http://localhost:8000/v1/sender/orders \
    -H "Content-Type: application/json" \
    -H "X-API-Key: 11f93de0-d304-4498-8b7b-6cecbc5b2dd8" \
    -d "{
      \"amount\": 0.5,
      \"token\": \"DAI\",
      \"network\": \"base-sepolia\",
      \"reference\": \"MULTI-TEST-$i\",
      ...
    }"
done

# Send crypto to orders 1, 3, 5 only

# Wait for detection (webhook or polling)

# Verify:
# - Orders 1, 3, 5: amount_paid = 0.5
# - Orders 2, 4: amount_paid = 0
```

---

### **Scenario 5: Partial Payment** 💸

**Goal**: Verify system handles partial payments correctly

**Steps**:
1. Create order for 1.0 DAI
2. Send 0.5 DAI
3. Verify order not fulfilled
4. Send another 0.5 DAI
5. Verify order fulfilled

**Expected Behavior**:
- ✅ First payment: amount_paid = 0.5, status = initiated
- ✅ Second payment: amount_paid = 1.0, status = validated

**Test**:
```bash
# 1. Create order for 1.0 DAI

# 2. Send 0.5 DAI
# Check: amount_paid = 0.5, status = initiated

# 3. Send another 0.5 DAI
# Check: amount_paid = 1.0, status = validated
```

---

### **Scenario 6: Overpayment** 💰

**Goal**: Verify system handles overpayment

**Steps**:
1. Create order for 0.5 DAI
2. Send 1.0 DAI
3. Verify order fulfilled

**Expected Behavior**:
- ✅ amount_paid = 1.0
- ✅ status = validated (payment sufficient)

---

### **Scenario 7: Concurrent Payments** ⚡⚡

**Goal**: Verify system handles concurrent payments

**Steps**:
1. Create 10 orders simultaneously
2. Send crypto to all 10 at once
3. Verify all detected

**Expected Behavior**:
- ✅ All 10 orders updated
- ✅ No race conditions
- ✅ No duplicate updates

---

## Monitoring

### **Logs to Watch**

```bash
# Webhook logs
docker logs -f aggregator_app_1 | grep -E "webhook|Webhook"

# Polling logs
docker logs -f aggregator_app_1 | grep -E "polling|Polling"

# Payment detection
docker logs -f aggregator_app_1 | grep -E "Payment detected|amount_paid"

# Errors
docker logs -f aggregator_app_1 | grep -E "ERROR|Error|error"
```

### **Metrics to Track**

```bash
# Polling metrics (logged every 5 minutes)
# Look for:
# - orders_checked: Number of orders polled
# - payments_detected: Payments found by polling
# - rpc_calls: RPC calls made
# - errors: Errors encountered
# - avg_check_time: Average time per check
```

---

## Verification Checklist

### **Before Testing**
- [ ] Application running
- [ ] Database connected
- [ ] Redis connected
- [ ] Alchemy API key configured
- [ ] Polling service enabled
- [ ] Webhook endpoint accessible (if testing webhooks)

### **Webhook Testing**
- [ ] Webhook fires on payment
- [ ] Detection time < 1 second
- [ ] Order updated correctly
- [ ] Status changes to validated
- [ ] Logs show webhook received

### **Polling Testing**
- [ ] Polling service starts
- [ ] Polls only old orders (> 5 minutes)
- [ ] Detects payments correctly
- [ ] Updates order
- [ ] Metrics logged every 5 minutes

### **Fallback Testing**
- [ ] Polling detects when webhook fails
- [ ] No duplicate updates
- [ ] Both methods can update same order (idempotent)
- [ ] Graceful degradation

---

## Troubleshooting

### **Webhook Not Firing**

```bash
# Check webhook is created
# Check Alchemy Dashboard → Notify → Webhooks

# Check endpoint is accessible
curl -X POST http://your-domain.com/v1/alchemy/webhook \
  -H "Content-Type: application/json" \
  -d '{"test": true}'

# Check logs for webhook errors
docker logs aggregator_app_1 | grep -i "webhook.*error"
```

### **Polling Not Detecting**

```bash
# Check polling is enabled
docker logs aggregator_app_1 | grep "Polling service started"

# Check order age
# Polling only checks orders > POLLING_MIN_AGE (default 5m)

# Check RPC connection
# Look for "Failed to get balance" errors

# Check metrics
docker logs aggregator_app_1 | grep "Polling service metrics"
```

### **Order Not Updating**

```bash
# Check database trigger
# The check_payment_order_amount() trigger validates payment

# Check amount_paid value
SELECT id, amount, amount_paid, sender_fee, network_fee, protocol_fee, status
FROM payment_orders
WHERE id = 'order_id';

# Check if payment is sufficient
# total_required = amount + sender_fee + network_fee + protocol_fee
# amount_paid must be >= total_required
```

---

## Performance Testing

### **Load Test**

```bash
# Create 100 orders
for i in {1..100}; do
  # Create order
  # Send crypto
done

# Monitor:
# - Webhook processing time
# - Polling cycle time
# - RPC call count
# - Database load
# - Memory usage
```

### **Expected Performance**

| Metric | Webhook | Polling |
|--------|---------|---------|
| Detection Time | < 1s | ~30s-5m |
| RPC Calls | 0 | 1 per order per interval |
| CPU Usage | Low | Low-Medium |
| Memory Usage | Low | Low |

---

## Success Criteria

✅ **Webhooks Working:**
- Payments detected < 1 second
- 99%+ success rate
- No errors in logs

✅ **Polling Working:**
- Detects payments for orders > 5 minutes old
- No RPC errors
- Metrics show payments_detected > 0

✅ **Fallback Working:**
- Polling catches webhook failures
- No payments missed
- Graceful degradation

✅ **System Stable:**
- No memory leaks
- No database deadlocks
- No race conditions
- Logs clean

---

## Next Steps After Testing

1. **If Webhooks Work**: Reduce polling frequency to 5-10 minutes
2. **If Webhooks Fail**: Increase polling frequency to 30 seconds
3. **If Both Work**: Keep hybrid mode (recommended)
4. **Monitor Production**: Track webhook vs polling detection ratio

---

## Production Checklist

- [ ] Webhooks configured in Alchemy Dashboard
- [ ] Webhook endpoint has SSL/HTTPS
- [ ] Signature verification enabled
- [ ] Polling enabled as fallback
- [ ] Monitoring/alerting set up
- [ ] Logs aggregated (e.g., Sentry, CloudWatch)
- [ ] Metrics tracked (e.g., Prometheus, Grafana)
- [ ] Load tested with expected volume
- [ ] Disaster recovery plan documented

---

**Last Updated**: 2025-10-09
**Status**: Ready for testing
