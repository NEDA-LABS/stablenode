# Quick Start: Alchemy Webhooks

## 🚀 5-Minute Setup (Manual)

### 1. Get Auth Token
```
https://dashboard.alchemy.com/settings → Auth Tokens → Create
Permissions: notify:read, notify:write
```

### 2. Add to .env
```bash
ALCHEMY_AUTH_TOKEN=alchemy_token_xxxxxxxxxxxxx
```

### 3. Create Webhook in Dashboard
```
https://dashboard.alchemy.com/notify → Create Webhook
Type: Address Activity
Network: Base Sepolia
Webhook URL: https://your-domain.com/v1/alchemy/webhook
```

### 4. Test
```bash
# Create order
curl -X POST http://localhost:8000/v1/sender/orders \
  -H "Content-Type: application/json" \
  -H "X-API-Key: 11f93de0-d304-4498-8b7b-6cecbc5b2dd8" \
  -d '{"amount": 0.5, "token": "DAI", "network": "base-sepolia", ...}'

# Send crypto to receive_address from response

# Check webhook fired
docker logs -f aggregator_app_1 | grep webhook
```

---

## 📋 Implementation Checklist

### Phase 1: Basic Setup
- [ ] Get Alchemy auth token
- [ ] Add `ALCHEMY_AUTH_TOKEN` to `.env`
- [ ] Restart application

### Phase 2: Database
- [ ] Create `ent/schema/alchemywebhook.go`
- [ ] Run `go generate ./ent/schema`
- [ ] Run database migration

### Phase 3: Webhook Handler
- [ ] Create `AlchemyWebhook()` in `controllers/index.go`
- [ ] Add route: `router.POST("/v1/alchemy/webhook", ...)`
- [ ] Implement signature verification

### Phase 4: Integration
- [ ] Initialize webhooks on startup (`main.go`)
- [ ] Add address to webhook during order creation (`sender.go`)
- [ ] Test end-to-end flow

---

## 🔧 Code Snippets

### Create Webhook (Programmatic)
```go
alchemyService := svc.NewAlchemyService()
webhookID, signingKey, err := alchemyService.CreateAddressActivityWebhook(
    ctx,
    84532, // Base Sepolia
    []string{},
    "https://your-domain.com/v1/alchemy/webhook",
)
```

### Add Address to Webhook
```go
err = alchemyService.AddAddressesToWebhook(
    ctx,
    webhookID,
    []string{"0x9876737E..."},
)
```

### Webhook Handler (Minimal)
```go
func (ctrl *Controller) AlchemyWebhook(ctx *gin.Context) {
    var payload types.AlchemyWebhookPayload
    ctx.BindJSON(&payload)
    
    for _, activity := range payload.Event.Activity {
        // Find order by receive address
        // Update amount_paid
        // Trigger fulfillment
    }
    
    ctx.JSON(200, gin.H{"status": "success"})
}
```

---

## 🐛 Common Issues

| Issue | Solution |
|-------|----------|
| Webhook not firing | Check URL is publicly accessible, use ngrok for local testing |
| 401 Unauthorized | Verify `ALCHEMY_AUTH_TOKEN` is correct |
| Address not monitored | Ensure address was added to webhook successfully |
| Duplicate events | Use `event.id` to deduplicate |

---

## 📚 Documentation

- **Full Guide**: `ALCHEMY_WEBHOOK_SETUP.md`
- **Summary**: `WEBHOOK_IMPLEMENTATION_SUMMARY.md`
- **Alchemy Docs**: https://docs.alchemy.com/reference/notify-api

---

## ⚡ Quick Commands

```bash
# Check if webhook exists
curl https://dashboard.alchemy.com/api/team-webhooks \
  -H "X-Alchemy-Token: $ALCHEMY_AUTH_TOKEN"

# Test webhook endpoint
curl -X POST http://localhost:8000/v1/alchemy/webhook \
  -H "Content-Type: application/json" \
  -d '{"test": true}'

# View logs
docker logs -f aggregator_app_1 | grep -i webhook
```

---

**Time to Complete**: 30 minutes - 2 hours
**Difficulty**: Medium
**Priority**: Critical (blocks payment detection)
