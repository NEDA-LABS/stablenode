# RPC API Key Migration Guide

## 🎯 Overview

**Changed:** RPC URLs now load API keys from environment variables instead of storing them in the database.

**Why:** 
- ✅ Better security (API keys not in database)
- ✅ Easier key rotation
- ✅ Centralized configuration
- ✅ No database migration needed

---

## 🔄 How It Works

### **Before:**
```
Database: rpc_endpoint = "https://base-sepolia.g.alchemy.com/v2/YOUR_API_KEY"
                                                                    ↑
                                                            Hardcoded in DB
```

### **After:**
```
Database: rpc_endpoint = "https://base-sepolia.g.alchemy.com/v2"
                                                                ↑
                                                        Base URL only

Environment: ALCHEMY_API_KEY=abc123xyz...
                            ↑
                    Loaded at runtime

Runtime: https://base-sepolia.g.alchemy.com/v2/abc123xyz...
                                                    ↑
                                            Appended automatically
```

---

## ⚙️ Configuration

### **1. Update `.env` file**

Ensure you have:
```bash
# Alchemy Configuration
ALCHEMY_API_KEY=your_actual_alchemy_api_key_here

# Optional: Infura (if you use Infura)
INFURA_API_KEY=your_infura_api_key_here
```

### **2. Update Database RPC URLs**

Run this SQL in Supabase Dashboard → SQL Editor:

```sql
-- Remove API keys from RPC endpoints (keep base URLs only)

-- Base Sepolia
UPDATE networks 
SET rpc_endpoint = 'https://base-sepolia.g.alchemy.com/v2' 
WHERE chain_id = 84532;

-- Ethereum Sepolia
UPDATE networks 
SET rpc_endpoint = 'https://eth-sepolia.g.alchemy.com/v2' 
WHERE chain_id = 11155111;

-- Arbitrum Sepolia
UPDATE networks 
SET rpc_endpoint = 'https://arb-sepolia.g.alchemy.com/v2' 
WHERE chain_id = 421614;

-- Verify
SELECT chain_id, identifier, rpc_endpoint 
FROM networks 
WHERE chain_id IN (84532, 11155111, 421614);
```

**Expected output:**
```
chain_id | identifier         | rpc_endpoint
---------|-------------------|------------------------------------------
84532    | base-sepolia      | https://base-sepolia.g.alchemy.com/v2
11155111 | ethereum-sepolia  | https://eth-sepolia.g.alchemy.com/v2
421614   | arbitrum-sepolia  | https://arb-sepolia.g.alchemy.com/v2
```

### **3. Rebuild and Restart**

```bash
sudo docker-compose down
sudo docker-compose up --build -d
```

---

## 🔍 How API Keys Are Appended

The new `utils.BuildRPCURL()` function automatically:

1. **Detects Alchemy URLs** (contains `alchemy.com`)
2. **Loads API key** from `ALCHEMY_API_KEY` environment variable
3. **Appends key** to base URL
4. **Returns full URL** for RPC client

### **Example:**

```go
// Database has:
rpcEndpoint := "https://base-sepolia.g.alchemy.com/v2"

// Code calls:
fullURL := utils.BuildRPCURL(rpcEndpoint)

// Returns:
// "https://base-sepolia.g.alchemy.com/v2/YOUR_ALCHEMY_API_KEY"
```

---

## 📝 Code Changes Made

### **1. New Utility Function** (`utils/rpc.go`)

```go
// BuildRPCURL constructs full RPC URL with API key from environment
func BuildRPCURL(baseURL string) string {
    // Detects Alchemy/Infura URLs
    // Loads API key from environment
    // Returns full URL with key appended
}
```

### **2. Updated Files**

- ✅ `services/polling_service.go` - Balance checking
- ✅ `services/engine.go` - RPC client creation
- ✅ `services/alchemy.go` - Alchemy service
- ✅ `services/common/order.go` - AML compliance checks

All RPC connections now use `utils.BuildRPCURL()`.

---

## ✅ Verification

### **1. Check Environment Variable**

```bash
# Verify ALCHEMY_API_KEY is set
grep ALCHEMY_API_KEY .env
```

Expected output:
```
ALCHEMY_API_KEY=your_actual_key_here
```

### **2. Check Database URLs**

```sql
SELECT chain_id, identifier, rpc_endpoint 
FROM networks 
WHERE rpc_endpoint LIKE '%alchemy%';
```

**Should NOT contain API keys** (no long random strings after `/v2`)

### **3. Test RPC Connection**

```bash
# Watch logs for successful RPC calls
sudo docker logs -f nedapay_aggregator | grep -i "rpc\|balance\|polling"
```

Expected:
```
INFO: Polling pending orders | count=1
INFO: Checking balance for address: 0x...
INFO: 💰 Payment detected via polling fallback
```

**Should NOT see:**
```
ERROR: 401 Unauthorized
ERROR: Must be authenticated
```

### **4. Test Balance Check**

Create a test order and send payment. Within 1-5 minutes, you should see:

```
INFO: 💰 Payment detected via polling fallback | NewBalance=0.5
INFO: Order status updated: pending → validated
```

---

## 🚨 Troubleshooting

### **Issue 1: Still getting 401 Unauthorized**

**Cause:** API key not loaded or incorrect

**Fix:**
```bash
# 1. Check .env has correct key
cat .env | grep ALCHEMY_API_KEY

# 2. Restart aggregator to reload environment
sudo docker-compose restart server

# 3. Verify key is loaded
sudo docker exec nedapay_aggregator env | grep ALCHEMY_API_KEY
```

---

### **Issue 2: Database still has API keys in URLs**

**Cause:** SQL update not run

**Fix:**
```sql
-- Check current URLs
SELECT chain_id, rpc_endpoint FROM networks WHERE rpc_endpoint LIKE '%alchemy%';

-- If they contain long keys after /v2/, update them:
UPDATE networks 
SET rpc_endpoint = REGEXP_REPLACE(rpc_endpoint, '/v2/[a-zA-Z0-9_-]+$', '/v2')
WHERE rpc_endpoint LIKE '%alchemy.com/v2/%';
```

---

### **Issue 3: API key not being appended**

**Cause:** URL format not recognized

**Check:**
```bash
# Database URL should be:
https://base-sepolia.g.alchemy.com/v2

# NOT:
https://base-sepolia.g.alchemy.com/v2/
https://base-sepolia.g.alchemy.com/v2/abc123
wss://base-sepolia.g.alchemy.com/ws/v2
```

**Fix:** Update database URL to exact format above (no trailing slash, no key)

---

## 🔐 Security Benefits

### **Before (API keys in database):**
- ❌ Keys visible in database dumps
- ❌ Keys in version control (if schema exported)
- ❌ Hard to rotate keys (requires database update)
- ❌ Same key for all environments

### **After (API keys in environment):**
- ✅ Keys not in database
- ✅ Keys not in version control
- ✅ Easy key rotation (just update .env)
- ✅ Different keys per environment (dev/staging/prod)

---

## 🔄 Key Rotation

To rotate your Alchemy API key:

```bash
# 1. Get new API key from Alchemy dashboard

# 2. Update .env
nano .env
# Change: ALCHEMY_API_KEY=old_key
# To:     ALCHEMY_API_KEY=new_key

# 3. Restart aggregator
sudo docker-compose restart server

# 4. Verify
sudo docker logs nedapay_aggregator | grep -i "polling\|balance"
```

**No database changes needed!** ✨

---

## 📊 Supported RPC Providers

### **Alchemy** (Auto-detected)
```bash
ALCHEMY_API_KEY=your_key
```
URLs: `*.alchemy.com/v2`

### **Infura** (Auto-detected)
```bash
INFURA_API_KEY=your_key
```
URLs: `*.infura.io/*`

### **Other Providers** (No changes needed)
- Public RPCs (no API key)
- Custom RPCs with embedded keys
- WebSocket URLs

---

## 📝 Summary

**What Changed:**
- ✅ Created `utils.BuildRPCURL()` function
- ✅ Updated all RPC connection points
- ✅ API keys now loaded from environment

**What You Need to Do:**
1. ✅ Ensure `ALCHEMY_API_KEY` is in `.env`
2. ✅ Update database RPC URLs (remove API keys)
3. ✅ Rebuild and restart aggregator
4. ✅ Verify RPC connections work

**Expected Result:**
- 🚀 RPC calls succeed with 200 OK
- 🚀 No more 401 Unauthorized errors
- 🚀 Payments detected successfully
- 🚀 Better security and easier key management

---

## 🆘 Need Help?

1. **Check logs:**
   ```bash
   sudo docker logs nedapay_aggregator --tail 100 | grep -i "error\|401"
   ```

2. **Verify environment:**
   ```bash
   sudo docker exec nedapay_aggregator env | grep ALCHEMY
   ```

3. **Test RPC manually:**
   ```bash
   curl https://base-sepolia.g.alchemy.com/v2/$ALCHEMY_API_KEY \
     -X POST \
     -H "Content-Type: application/json" \
     -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}'
   ```

Expected: `{"jsonrpc":"2.0","id":1,"result":"0x..."}`
