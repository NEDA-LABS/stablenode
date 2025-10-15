# Pre-Flight Testing - Test Before Running Aggregator

## Overview

Test webhook functionality **BEFORE** starting the full aggregator to catch configuration errors early and avoid rebuild cycles.

---

## Why Test First?

✅ **Catch errors early** - Find configuration issues before deployment
✅ **Faster debugging** - No need to rebuild Docker containers
✅ **Verify credentials** - Confirm Alchemy auth token works
✅ **Test API calls** - Ensure webhook API is accessible
✅ **Save time** - 2 minutes of testing saves hours of debugging

---

## Quick Test (2 Minutes)

### **Step 1: Get Alchemy Auth Token**

```bash
# 1. Go to Alchemy Dashboard
https://dashboard.alchemy.com/settings

# 2. Navigate to "Auth Tokens" section

# 3. Click "Create Auth Token"

# 4. Set permissions:
   ☑ notify:read
   ☑ notify:write

# 5. Copy the token (starts with "alchemy_...")

# 6. Add to .env
echo 'ALCHEMY_AUTH_TOKEN=alchemy_your_token_here' >> .env
```

### **Step 2: Run Webhook Test**

```bash
# Run the test script
./test_webhook.sh
```

**Expected Output:**
```
================================
🧪 Alchemy Webhook Test
================================

✅ Configuration found

Building webhook test tool...
✅ Build successful

Running webhook tests...

🧪 Alchemy Webhook Testing Tool
================================

✅ Auth token found: alchemy_abc...

📋 Test 1: Network ID Mapping
  Chain ID 1 → ✅ ETH_MAINNET
  Chain ID 11155111 → ✅ ETH_SEPOLIA
  Chain ID 8453 → ✅ BASE_MAINNET
  Chain ID 84532 → ✅ BASE_SEPOLIA
  Chain ID 42161 → ✅ ARB_MAINNET
  Chain ID 421614 → ✅ ARB_SEPOLIA

📋 Test 2: Create Webhook
  Creating webhook for Base Sepolia (chain 84532)...
  Webhook URL: https://your-domain.com/v1/alchemy/webhook
  ✅ Webhook created successfully!
  Webhook ID: wh_abc123xyz
  Signing Key: whsec_abc123xyz...

📋 Test 3: Add Addresses to Webhook
  Adding 2 test addresses to webhook...
  ✅ Addresses added successfully!
    1. 0x1111111111111111111111111111111111111111
    2. 0x2222222222222222222222222222222222222222

📋 Test 4: Remove Addresses from Webhook
  Removing 1 address from webhook...
  ✅ Address removed successfully!
    0x1111111111111111111111111111111111111111

📋 Test 5: Delete Webhook (cleanup)
  Deleting webhook wh_abc123xyz...
  ✅ Webhook deleted successfully!
  (Check Alchemy Dashboard to confirm)

================================
✅ All webhook tests completed!

Next steps:
1. Check Alchemy Dashboard: https://dashboard.alchemy.com/notify
2. Verify webhook was created and deleted
3. If all tests passed, you're ready to start the aggregator

Webhook ID: wh_abc123xyz
Signing Key: whsec_abc123xyz...

================================
✅ Testing complete!
================================
```

### **Step 3: Verify in Alchemy Dashboard**

```bash
# 1. Go to Alchemy Dashboard
https://dashboard.alchemy.com/notify

# 2. Check "Webhooks" tab

# 3. Verify:
   - Webhook was created (during test)
   - Webhook was deleted (cleanup)
   - No errors in activity log
```

---

## Manual Testing (Alternative)

If the script doesn't work, test manually:

### **Test 1: Build Test Tool**

```bash
# Build the test tool
go build -o test_webhook_tool ./cmd/test_webhook/main.go

# Run it
./test_webhook_tool

# Clean up
rm test_webhook_tool
```

### **Test 2: Test with curl**

```bash
# Load environment
source .env

# Test create webhook
curl -X POST https://dashboard.alchemy.com/api/create-webhook \
  -H "X-Alchemy-Token: $ALCHEMY_AUTH_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "network": "BASE_SEPOLIA",
    "webhook_type": "ADDRESS_ACTIVITY",
    "webhook_url": "https://your-domain.com/v1/alchemy/webhook",
    "addresses": []
  }'

# Expected response:
# {
#   "data": {
#     "id": "wh_...",
#     "signing_key": "whsec_...",
#     ...
#   }
# }
```

---

## Troubleshooting

### **Error: "ALCHEMY_AUTH_TOKEN not set"**

```bash
# Check .env file
cat .env | grep ALCHEMY_AUTH_TOKEN

# Should show:
# ALCHEMY_AUTH_TOKEN=alchemy_...

# If empty, add it:
echo 'ALCHEMY_AUTH_TOKEN=your_token_here' >> .env
```

### **Error: "401 Unauthorized"**

```bash
# Token is invalid or missing permissions

# Fix:
# 1. Go to https://dashboard.alchemy.com/settings
# 2. Delete old token
# 3. Create new token with notify:read, notify:write
# 4. Update .env with new token
```

### **Error: "400 Bad Request"**

```bash
# Invalid network or webhook URL

# Check:
# 1. Network ID is correct (BASE_SEPOLIA, ETH_MAINNET, etc.)
# 2. Webhook URL is valid HTTPS URL
# 3. Addresses are valid Ethereum addresses (if provided)
```

### **Error: "Build failed"**

```bash
# Missing dependencies

# Fix:
go mod download
go mod tidy

# Then retry:
./test_webhook.sh
```

### **Error: "Cannot connect to Alchemy"**

```bash
# Network/firewall issue

# Check:
# 1. Internet connection
# 2. Firewall allows HTTPS to dashboard.alchemy.com
# 3. Proxy settings (if behind corporate proxy)

# Test connection:
curl -I https://dashboard.alchemy.com
```

---

## What Gets Tested

### ✅ **Configuration**
- Auth token is set
- Auth token is valid
- Environment variables loaded

### ✅ **Network Mapping**
- Chain IDs map to Alchemy network names
- All supported networks recognized

### ✅ **Webhook Creation**
- Can create webhook via API
- Webhook ID returned
- Signing key returned

### ✅ **Address Management**
- Can add addresses to webhook
- Can remove addresses from webhook
- Addresses properly formatted

### ✅ **Webhook Deletion**
- Can delete webhook
- Cleanup successful

---

## After Testing

### **If All Tests Pass** ✅

```bash
# You're ready to start the aggregator!

# 1. Start application
docker-compose up -d

# 2. Watch logs
docker logs -f aggregator_app_1

# 3. Create test order
# Follow START_TESTING_NOW.md
```

### **If Tests Fail** ❌

```bash
# Fix the issues before starting aggregator

# Common fixes:
# 1. Check ALCHEMY_AUTH_TOKEN in .env
# 2. Verify token has correct permissions
# 3. Check internet connection
# 4. Review error messages

# Re-run test after fixes:
./test_webhook.sh
```

---

## Integration with Aggregator

Once tests pass, the aggregator will use the same webhook methods:

```go
// In your code (sender.go, etc.)
alchemyService := services.NewAlchemyService()

// Create webhook for network
webhookID, signingKey, err := alchemyService.CreateAddressActivityWebhook(
    ctx,
    chainID,
    []string{receiveAddress},
    webhookURL,
)

// Store in database
// ... save webhookID and signingKey
```

---

## Test Checklist

Before starting aggregator:

- [ ] Alchemy auth token obtained
- [ ] Token added to .env
- [ ] `./test_webhook.sh` runs successfully
- [ ] All 5 tests pass
- [ ] Webhook visible in Alchemy Dashboard
- [ ] Webhook deleted successfully (cleanup)
- [ ] No errors in test output

After checklist complete:

- [ ] Start aggregator: `docker-compose up -d`
- [ ] Follow `START_TESTING_NOW.md`

---

## Advanced Testing

### **Test Multiple Networks**

Modify `cmd/test_webhook/main.go` to test multiple networks:

```go
// Test all networks
networks := []int64{
    84532,  // Base Sepolia
    11155111, // Ethereum Sepolia
    421614, // Arbitrum Sepolia
}

for _, chainID := range networks {
    webhookID, _ := testCreateWebhook(service, chainID)
    testDeleteWebhook(service, webhookID)
}
```

### **Test with Real Addresses**

```go
// Use real receive addresses from your system
realAddresses := []string{
    "0xYourRealAddress1",
    "0xYourRealAddress2",
}

testAddAddresses(service, webhookID, realAddresses)
```

---

## Files

```
cmd/test_webhook/
└── main.go              # Standalone test tool

test_webhook.sh          # Test runner script
PRE_FLIGHT_TESTING.md    # This file
```

---

## Quick Commands

```bash
# Run full test
./test_webhook.sh

# Build and run manually
go build -o test_webhook_tool ./cmd/test_webhook/main.go
./test_webhook_tool
rm test_webhook_tool

# Check configuration
cat .env | grep ALCHEMY

# Verify Alchemy Dashboard
open https://dashboard.alchemy.com/notify
```

---

## Summary

**Time**: 2 minutes
**Purpose**: Verify webhook setup before starting aggregator
**Benefit**: Catch errors early, save debugging time

**Process**:
1. Get auth token (1 minute)
2. Run test script (30 seconds)
3. Verify results (30 seconds)
4. Start aggregator (if tests pass)

**Result**: Confidence that webhooks will work in production!

---

**Last Updated**: 2025-10-09
**Status**: Ready to use
**Next**: Run `./test_webhook.sh`
