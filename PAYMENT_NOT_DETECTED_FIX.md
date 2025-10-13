# Payment Not Detected - Root Cause Analysis & Fix

## 🔴 Critical Issues Found

### 1. **RPC URL Missing Alchemy API Key (CRITICAL)**
**Error:**
```
ERROR: 401 Unauthorized: "Must be authenticated!"
failed to get transaction receipt
failed to call contract
```

**Root Cause:**
- Database has incomplete RPC URLs: `https://base-sepolia.g.alchemy.com/v2` (missing API key)
- Should be: `https://base-sepolia.g.alchemy.com/v2/YOUR_ALCHEMY_API_KEY`

**Impact:**
- ❌ Polling service cannot check balances
- ❌ Indexer cannot fetch transaction receipts
- ❌ Payments are NOT detected

---

### 2. **Database Connection Timeouts**
**Error:**
```
ERROR: read tcp connection reset by peer (Supabase)
ERROR: connection timed out
```

**Root Cause:**
- Supabase connection pool exhaustion
- Network instability to Supabase (3.65.151.229:5432)

**Impact:**
- ❌ Orders cannot be updated
- ❌ Payment status stuck at "pending"

---

### 3. **Etherscan API Timeouts**
**Warning:**
```
WARNING: Etherscan failed for chain 84532, falling back to Engine: context deadline exceeded
```

**Root Cause:**
- Etherscan API slow/rate limited
- System correctly falls back to ThirdWeb Engine, but Engine also times out

**Impact:**
- ⚠️ Slower transaction indexing
- ⚠️ Missed events if both fail

---

## ✅ Fixes

### **Fix 1: Update RPC URLs with Alchemy API Key**

#### Option A: Using the Script (Recommended)
```bash
cd /home/commendatore/Desktop/NEDA/rails/aggregator
chmod +x scripts/fix_rpc_urls.sh
./scripts/fix_rpc_urls.sh
```

#### Option B: Manual SQL (via Supabase Dashboard)
1. Go to https://supabase.com/dashboard
2. Select your project → SQL Editor
3. Run this SQL (replace `YOUR_ALCHEMY_API_KEY`):

```sql
-- Get your ALCHEMY_API_KEY from .env first!

UPDATE networks 
SET rpc_endpoint = 'https://base-sepolia.g.alchemy.com/v2/YOUR_ALCHEMY_API_KEY' 
WHERE chain_id = 84532;

UPDATE networks 
SET rpc_endpoint = 'https://eth-sepolia.g.alchemy.com/v2/YOUR_ALCHEMY_API_KEY' 
WHERE chain_id = 11155111;

UPDATE networks 
SET rpc_endpoint = 'https://arb-sepolia.g.alchemy.com/v2/YOUR_ALCHEMY_API_KEY' 
WHERE chain_id = 421614;

-- Verify
SELECT chain_id, identifier, rpc_endpoint FROM networks WHERE chain_id IN (84532, 11155111, 421614);
```

---

### **Fix 2: Restart Aggregator**
After updating RPC URLs:
```bash
sudo docker-compose restart server
```

---

### **Fix 3: Monitor Logs**
```bash
# Watch for successful payment detection
sudo docker logs -f nedapay_aggregator | grep -i "payment\|balance\|detected"
```

---

## 🔍 Verification Steps

### 1. Check RPC URL is correct:
```sql
SELECT chain_id, identifier, rpc_endpoint 
FROM networks 
WHERE chain_id = 84532;
```

Expected output:
```
chain_id | identifier    | rpc_endpoint
---------|---------------|--------------------------------------------------
84532    | base-sepolia  | https://base-sepolia.g.alchemy.com/v2/YOUR_KEY
```

### 2. Test RPC connection:
```bash
# Should return a block number
curl https://base-sepolia.g.alchemy.com/v2/YOUR_ALCHEMY_API_KEY \
  -X POST \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}'
```

### 3. Check order status:
```sql
SELECT id, status, amount_paid, created_at 
FROM payment_orders 
WHERE id = 'YOUR_ORDER_ID';
```

### 4. Check receive address balance:
```sql
SELECT ra.address, po.amount, po.amount_paid, po.status
FROM receive_addresses ra
JOIN payment_orders po ON po.receive_address_payment_order = ra.id
WHERE ra.address = '0x013542D234dE04f442a832F475872Acd88Cf0bE4';
```

---

## 📊 Expected Behavior After Fix

### Logs should show:
```
INFO: Polling pending orders | count=1
INFO: Checking balance for address: 0x013542D234dE04f442a832F475872Acd88Cf0bE4
INFO: 💰 Payment detected via polling fallback | NewBalance=0.5
INFO: Order status updated: pending → validated
```

### Database should update:
- `payment_orders.amount_paid` = 0.5
- `payment_orders.status` = 'validated'
- `payment_orders.validated_at` = current timestamp

---

## 🚨 Why Your Payment Wasn't Detected

Your transaction:
- **Address:** `0x013542D234dE04f442a832F475872Acd88Cf0bE4`
- **Amount:** 0.5 USDC
- **Network:** Base Sepolia (84532)

**What happened:**
1. ✅ You sent 0.5 USDC to the receive address
2. ❌ Polling service tried to check balance using RPC
3. ❌ RPC returned 401 Unauthorized (missing API key)
4. ❌ Polling service logged error and skipped the order
5. ❌ Order remained in "pending" status

**What will happen after fix:**
1. ✅ Polling service checks balance using correct RPC URL
2. ✅ Detects 0.5 USDC balance
3. ✅ Updates order status to "validated"
4. ✅ Provider can now accept/fulfill the order

---

## 🔧 Additional Recommendations

### 1. **Increase Database Connection Pool**
Add to your `.env`:
```bash
DB_MAX_CONNECTIONS=20
DB_MAX_IDLE_CONNECTIONS=5
```

### 2. **Add RPC Timeout Configuration**
```bash
RPC_TIMEOUT=30s
RPC_RETRY_ATTEMPTS=3
```

### 3. **Monitor Polling Service**
```bash
# Check polling metrics every 5 minutes
sudo docker logs nedapay_aggregator 2>&1 | grep "Polling service metrics"
```

### 4. **Set up Alchemy Webhook (Recommended)**
Once RPC is working, set up webhook for real-time detection:
```bash
# Use the webhook setup endpoint
curl -X POST http://localhost:8000/v1/admin/setup-webhook \
  -H "Content-Type: application/json" \
  -d '{"network": "base-sepolia"}'
```

---

## 📝 Summary

**Primary Issue:** RPC URLs in database are missing Alchemy API key
**Fix:** Run `scripts/fix_rpc_urls.sh` or manually update via SQL
**Time to Fix:** 2 minutes
**Expected Result:** Payments will be detected within next polling cycle (5 minutes)

---

## 🆘 If Still Not Working

1. Check Alchemy API key is valid:
   ```bash
   echo $ALCHEMY_API_KEY
   ```

2. Verify Alchemy dashboard shows API calls

3. Check aggregator logs for new errors:
   ```bash
   sudo docker logs nedapay_aggregator --tail 100 | grep ERROR
   ```

4. Manually trigger reindex:
   ```bash
   curl "http://localhost:8000/v1/reindex/base-sepolia/0x013542D234dE04f442a832F475872Acd88Cf0bE4"
   ```
