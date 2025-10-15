# Webhook Implementation Summary

## What Was Implemented

### **1. Alchemy Webhook Management API** ✅

Added complete webhook management to `services/alchemy.go`:

- **`CreateAddressActivityWebhook()`** - Create new webhook for a network
- **`AddAddressesToWebhook()`** - Add receive addresses dynamically
- **`RemoveAddressesFromWebhook()`** - Remove addresses after order completion
- **`DeleteWebhook()`** - Delete webhook
- **`getAlchemyNetworkID()`** - Map chain IDs to Alchemy network identifiers

**Supported Networks:**
- Ethereum (Mainnet & Sepolia)
- Polygon (Mainnet & Amoy)
- Arbitrum (Mainnet & Sepolia)
- Optimism (Mainnet & Sepolia)
- Base (Mainnet & Sepolia)
- BNB Chain (Mainnet & Testnet)

### **2. Configuration Updates** ✅

**`.env.example`:**
```bash
ALCHEMY_AUTH_TOKEN=your_alchemy_auth_token_here  # For webhook management API
```

**`config/alchemy.go`:**
```go
type AlchemyConfiguration struct {
    APIKey      string  // For RPC calls
    BaseURL     string
    GasPolicyID string
    AuthToken   string  // For webhook management
}
```

### **3. HTTP Client Implementation** ✅

Uses `fastshot` HTTP client with:
- `X-Alchemy-Token` header for authentication
- JSON request/response handling
- Error handling and logging

---

## How It Works

### **Current Flow (Without Webhooks)**
```
User creates order → Alchemy address generated → Order saved
                                                      ↓
                                              ❌ NO PAYMENT DETECTION
```

### **New Flow (With Webhooks)**
```
1. Startup: Create webhook for each network
   └─→ Store webhook_id and signing_key in database

2. Order Creation:
   └─→ Generate Alchemy receive address
   └─→ Add address to network's webhook
   └─→ Save order

3. User Deposits Crypto:
   └─→ Alchemy detects transfer
   └─→ POST /v1/alchemy/webhook
   └─→ Verify signature
   └─→ Update amount_paid
   └─→ Trigger order fulfillment

4. Order Complete:
   └─→ Remove address from webhook (cleanup)
```

---

## What You Need to Do

### **Step 1: Get Alchemy Auth Token** 🔑

1. Go to https://dashboard.alchemy.com/settings
2. Navigate to "Auth Tokens"
3. Create token with permissions:
   - `notify:read`
   - `notify:write`
4. Add to `.env`:
   ```bash
   ALCHEMY_AUTH_TOKEN=alchemy_token_xxxxx
   ```

### **Step 2: Choose Implementation Strategy** 📋

**Option A: Manual Dashboard Setup (Quick)**
- Create webhook manually in Alchemy Dashboard
- Use API to add/remove addresses dynamically
- Good for testing

**Option B: Fully Programmatic (Recommended)**
- Create webhooks on app startup
- Automatically add addresses during order creation
- Production-ready

See `ALCHEMY_WEBHOOK_SETUP.md` for detailed implementation.

### **Step 3: Create Database Schema** 💾

Create `AlchemyWebhook` entity:
```go
// ent/schema/alchemywebhook.go
type AlchemyWebhook struct {
    ent.Schema
}

func (AlchemyWebhook) Fields() []ent.Field {
    return []ent.Field{
        field.String("webhook_id").Unique(),
        field.String("signing_key"),
        field.String("callback_url"),
        field.Time("created_at").Default(time.Now),
    }
}

func (AlchemyWebhook) Edges() []ent.Edge {
    return []ent.Edge{
        edge.From("network", Network.Type).
            Ref("alchemy_webhooks").
            Unique().
            Required(),
    }
}
```

Run migration:
```bash
go run -mod=mod entgo.io/ent/cmd/ent generate ./ent/schema
```

### **Step 4: Implement Webhook Handler** 🎯

Create endpoint in `controllers/index.go`:
```go
func (ctrl *Controller) AlchemyWebhook(ctx *gin.Context) {
    // 1. Read raw body
    // 2. Verify signature
    // 3. Parse payload
    // 4. Find order by receive address
    // 5. Update amount_paid
    // 6. Trigger fulfillment if sufficient
}
```

Add route in `main.go`:
```go
router.POST("/v1/alchemy/webhook", controller.AlchemyWebhook)
```

### **Step 5: Initialize Webhooks** 🚀

Add to `main.go` or init function:
```go
func initializeAlchemyWebhooks() error {
    // For each network:
    // 1. Check if webhook exists
    // 2. Create if not exists
    // 3. Store in database
}
```

### **Step 6: Update Order Creation** 📝

Modify `sender.go` around line 536:
```go
if useAlchemy {
    // Get webhook for network
    webhook, _ := getWebhookForNetwork(token.Edges.Network.ID)
    
    // Add receive address to webhook
    alchemyService.AddAddressesToWebhook(
        ctx,
        webhook.WebhookID,
        []string{receiveAddress.Address},
    )
}
```

---

## Testing Checklist

- [ ] Get Alchemy auth token
- [ ] Add `ALCHEMY_AUTH_TOKEN` to `.env`
- [ ] Create `AlchemyWebhook` schema
- [ ] Run database migration
- [ ] Implement webhook handler
- [ ] Add webhook route
- [ ] Initialize webhooks on startup
- [ ] Update order creation logic
- [ ] Create test order
- [ ] Send test crypto
- [ ] Verify webhook fires
- [ ] Check `amount_paid` updates
- [ ] Verify order fulfillment triggers

---

## API Methods Available

```go
alchemyService := svc.NewAlchemyService()

// Create webhook
webhookID, signingKey, err := alchemyService.CreateAddressActivityWebhook(
    ctx,
    chainID,      // 84532 for Base Sepolia
    []string{},   // Start with empty addresses
    webhookURL,   // https://your-domain.com/v1/alchemy/webhook
)

// Add addresses
err = alchemyService.AddAddressesToWebhook(
    ctx,
    webhookID,
    []string{"0xAddress1", "0xAddress2"},
)

// Remove addresses
err = alchemyService.RemoveAddressesFromWebhook(
    ctx,
    webhookID,
    []string{"0xAddress1"},
)

// Delete webhook
err = alchemyService.DeleteWebhook(ctx, webhookID)
```

---

## Key Files Modified

1. **`services/alchemy.go`** - Added webhook management methods
2. **`config/alchemy.go`** - Added `AuthToken` field
3. **`.env.example`** - Added `ALCHEMY_AUTH_TOKEN`

## Key Files to Create

1. **`ent/schema/alchemywebhook.go`** - Database schema
2. **Webhook handler** in `controllers/index.go`
3. **Initialization logic** in `main.go`

---

## Documentation Created

1. **`ALCHEMY_WEBHOOK_SETUP.md`** - Complete setup guide
2. **`WEBHOOK_IMPLEMENTATION_SUMMARY.md`** - This file
3. **Updated `ALCHEMY_MIGRATION.md`** - Progress tracking

---

## Next Steps Priority

1. **Critical**: Get Alchemy auth token
2. **Critical**: Create database schema
3. **Critical**: Implement webhook handler
4. **High**: Initialize webhooks on startup
5. **High**: Update order creation logic
6. **Medium**: Add signature verification
7. **Medium**: Implement cleanup logic
8. **Low**: Add monitoring/alerting

---

## Support

- **Alchemy Docs**: https://docs.alchemy.com/reference/notify-api
- **Webhook Types**: https://docs.alchemy.com/reference/webhook-types
- **Dashboard**: https://dashboard.alchemy.com/notify

---

**Status**: Implementation ready for testing
**Estimated Time**: 2-4 hours for full integration
**Last Updated**: 2025-10-09 10:38
