# Alchemy Webhook Setup Guide

## Overview

This guide explains how to set up Alchemy Address Activity webhooks for automatic payment detection when using Alchemy receive addresses.

## Why Webhooks Are Needed

When `USE_ALCHEMY_FOR_RECEIVE_ADDRESSES=true`, orders are created with unique Alchemy smart account addresses. However, **payment detection is broken** without webhooks because:

- Thirdweb webhook creation is skipped (line 498 in `sender.go`)
- No automatic monitoring of receive addresses
- Orders stay in `initiated` status even after user deposits crypto

## Two Approaches

### **Approach 1: Manual Dashboard Setup (Quick Start)**

1. **Go to Alchemy Dashboard**
   - Navigate to: https://dashboard.alchemy.com/notify
   - Click "Create Webhook"

2. **Select Webhook Type**
   - Choose: **"Address Activity"**
   - This monitors all transactions for specific addresses

3. **Configure Webhook**
   - **Network**: Select your chain (e.g., Base Sepolia, Ethereum Sepolia)
   - **Webhook URL**: `https://your-domain.com/v1/alchemy/webhook`
   - **Addresses**: Leave empty initially (add programmatically)

4. **Save and Get Credentials**
   - Copy the **Webhook ID**
   - Copy the **Signing Key** (for signature verification)
   - Store in database for each network

5. **Add Addresses Dynamically**
   - Use Alchemy Notify API to add receive addresses as orders are created
   - See Approach 2 for implementation

**Limitations:**
- Manual setup per network
- Still need API integration to add addresses dynamically
- Max 100K addresses per webhook

---

### **Approach 2: Programmatic Setup (Recommended)**

Full automation using Alchemy Notify API.

#### **Step 1: Get Alchemy Auth Token**

1. Go to: https://dashboard.alchemy.com/settings
2. Navigate to "Auth Tokens"
3. Create new token with permissions:
   - `notify:read`
   - `notify:write`
4. Copy the token

#### **Step 2: Configure Environment**

Add to `.env`:
```bash
ALCHEMY_AUTH_TOKEN=""
USE_ALCHEMY_FOR_RECEIVE_ADDRESSES=true
```

#### **Step 3: Implementation Options**

**Option A: Create One Webhook Per Network (Recommended)**

Create a webhook for each supported network and dynamically add/remove addresses:

```go
// Initialize webhooks on startup (main.go or init function)
func initializeAlchemyWebhooks() error {
    alchemyService := svc.NewAlchemyService()
    serverConf := config.ServerConfig()
    
    // Get all supported networks
    networks, err := storage.Client.Network.Query().All(context.Background())
    if err != nil {
        return err
    }
    
    for _, network := range networks {
        // Check if webhook already exists for this network
        existingWebhook, _ := storage.Client.AlchemyWebhook.
            Query().
            Where(alchemywebhook.NetworkIDEQ(network.ID)).
            Only(context.Background())
        
        if existingWebhook != nil {
            continue // Webhook already exists
        }
        
        // Create webhook
        webhookURL := fmt.Sprintf("%s/v1/alchemy/webhook", serverConf.ServerURL)
        webhookID, signingKey, err := alchemyService.CreateAddressActivityWebhook(
            context.Background(),
            network.ChainID,
            []string{}, // Start with empty addresses
            webhookURL,
        )
        
        if err != nil {
            logger.Errorf("Failed to create webhook for network %s: %v", network.Identifier, err)
            continue
        }
        
        // Store webhook in database
        _, err = storage.Client.AlchemyWebhook.
            Create().
            SetWebhookID(webhookID).
            SetSigningKey(signingKey).
            SetNetwork(network).
            SetCallbackURL(webhookURL).
            Save(context.Background())
        
        if err != nil {
            logger.Errorf("Failed to save webhook: %v", err)
        }
    }
    
    return nil
}
```

**Option B: Add Address to Webhook During Order Creation**

Modify `sender.go` to add the receive address to the webhook:

```go
// After creating receive address (around line 536 in sender.go)
if useAlchemy {
    // Get webhook for this network
    webhook, err := tx.AlchemyWebhook.
        Query().
        Where(alchemywebhook.HasNetworkWith(networkent.IDEQ(token.Edges.Network.ID))).
        Only(ctx)
    
    if err != nil {
        logger.Errorf("No Alchemy webhook found for network %s", token.Edges.Network.Identifier)
        // Continue without webhook (fallback to polling/indexer)
    } else {
        // Add receive address to webhook
        alchemyService := svc.NewAlchemyService()
        err = alchemyService.AddAddressesToWebhook(
            ctx,
            webhook.WebhookID,
            []string{receiveAddress.Address},
        )
        
        if err != nil {
            logger.Errorf("Failed to add address to webhook: %v", err)
            // Continue - address can be added later or use fallback
        } else {
            logger.Infof("Added address %s to Alchemy webhook %s", 
                receiveAddress.Address, webhook.WebhookID)
        }
    }
}
```

#### **Step 4: Create Webhook Handler**

Create endpoint to receive Alchemy webhooks:

```go
// In controllers/index.go
func (ctrl *Controller) AlchemyWebhook(ctx *gin.Context) {
    // Get raw body for signature verification
    rawBody, err := ctx.GetRawData()
    if err != nil {
        logger.Errorf("Failed to read Alchemy webhook payload: %v", err)
        ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload"})
        return
    }
    
    // Get signature from header
    signature := ctx.GetHeader("X-Alchemy-Signature")
    
    // TODO: Verify signature using webhook signing key
    // (Similar to Thirdweb signature verification)
    
    // Parse webhook payload
    var payload types.AlchemyWebhookPayload
    if err := json.Unmarshal(rawBody, &payload); err != nil {
        logger.Errorf("Failed to parse Alchemy webhook: %v", err)
        ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload"})
        return
    }
    
    // Process activity events
    for _, activity := range payload.Event.Activity {
        // Check if this is an incoming transfer
        if activity.Category == "token" && activity.ToAddress != "" {
            // Find order with this receive address
            order, err := storage.Client.PaymentOrder.
                Query().
                Where(
                    paymentorder.HasReceiveAddressWith(
                        receiveaddress.AddressEqualFold(activity.ToAddress),
                    ),
                    paymentorder.StatusEQ(paymentorder.StatusInitiated),
                ).
                WithToken().
                Only(ctx)
            
            if err != nil {
                continue // Not our address or order already processed
            }
            
            // Parse transfer amount
            value, _ := decimal.NewFromString(activity.Value)
            amountReceived := value.Div(decimal.NewFromInt(10).Pow(
                decimal.NewFromInt(int64(order.Edges.Token.Decimals)),
            ))
            
            // Update order amount_paid
            _, err = order.Update().
                SetAmountPaid(amountReceived).
                Save(ctx)
            
            if err != nil {
                logger.Errorf("Failed to update order amount_paid: %v", err)
                continue
            }
            
            // Check if payment is sufficient
            totalRequired := order.Amount.Add(order.SenderFee).Add(order.NetworkFee).Add(order.ProtocolFee)
            if amountReceived.GreaterThanOrEqual(totalRequired) {
                // Payment sufficient - trigger order processing
                logger.Infof("Payment received for order %s: %s", order.ID, amountReceived)
                // TODO: Trigger order creation on Gateway contract
            }
        }
    }
    
    ctx.JSON(http.StatusOK, gin.H{"status": "success"})
}
```

#### **Step 5: Add Route**

In `main.go` or routes file:
```go
router.POST("/v1/alchemy/webhook", controller.AlchemyWebhook)
```

#### **Step 6: Database Schema**

Create `AlchemyWebhook` entity to store webhook details:

```go
// In ent/schema/alchemywebhook.go
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

---

## Testing

### **1. Create Test Order**
```bash
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
    "reference": "TEST-WEBHOOK-001",
    "returnAddress": "0x18000433c7cc39ebdAbB06262F88795960FE5Cf9"
  }'
```

### **2. Send Test Crypto**
Send DAI to the receive address returned in the response.

### **3. Monitor Webhook**
Check logs for webhook callback:
```bash
docker logs -f aggregator_app_1 | grep "Alchemy webhook"
```

### **4. Verify Order Update**
```bash
curl http://localhost:8000/v1/sender/orders/ORDER_ID \
  -H "X-API-Key: 11f93de0-d304-4498-8b7b-6cecbc5b2dd8"
```

Check that `amount_paid` is updated and status changes to `validated`.

---

## Webhook Payload Structure

Alchemy Address Activity webhook sends:

```json
{
  "webhookId": "wh_xxx",
  "id": "whevt_xxx",
  "createdAt": "2025-10-09T10:00:00.000Z",
  "type": "ADDRESS_ACTIVITY",
  "event": {
    "network": "BASE_SEPOLIA",
    "activity": [
      {
        "fromAddress": "0xUSER_ADDRESS",
        "toAddress": "0xRECEIVE_ADDRESS",
        "blockNum": "0x123456",
        "hash": "0xTX_HASH",
        "value": 500000000000000000,
        "asset": "DAI",
        "category": "token",
        "rawContract": {
          "address": "0xDAI_CONTRACT",
          "decimals": 18
        }
      }
    ]
  }
}
```

---

## Cleanup (Optional)

Remove addresses from webhook when order is completed:

```go
func removeAddressFromWebhook(ctx context.Context, receiveAddress string, networkID uuid.UUID) error {
    webhook, err := storage.Client.AlchemyWebhook.
        Query().
        Where(alchemywebhook.HasNetworkWith(networkent.IDEQ(networkID))).
        Only(ctx)
    
    if err != nil {
        return err
    }
    
    alchemyService := svc.NewAlchemyService()
    return alchemyService.RemoveAddressesFromWebhook(
        ctx,
        webhook.WebhookID,
        []string{receiveAddress},
    )
}
```

---

## Troubleshooting

### **Webhook Not Firing**
- Check webhook is active in Alchemy Dashboard
- Verify callback URL is publicly accessible
- Check firewall/security group allows incoming HTTPS
- Test with ngrok for local development

### **Signature Verification Failing**
- Ensure using correct signing key from webhook creation
- Verify signature algorithm matches Alchemy's implementation
- Check header name is `X-Alchemy-Signature`

### **Address Not Monitored**
- Verify address was added to webhook successfully
- Check webhook has capacity (max 100K addresses)
- Ensure address is checksummed (mixed case)

### **Duplicate Events**
- Alchemy may send duplicate webhooks
- Use `event.id` to deduplicate
- Check `amount_paid` before updating

---

## Cost Considerations

- **Free Tier**: 100 webhooks, 100K addresses per webhook
- **Growth Tier**: Unlimited webhooks and addresses
- **Webhook calls**: No additional cost
- **Recommended**: Start with 1 webhook per network

---

## Security Best Practices

1. **Always verify webhook signatures**
2. **Use HTTPS for callback URL**
3. **Store signing keys securely** (database, not code)
4. **Rate limit webhook endpoint**
5. **Validate payload structure**
6. **Log all webhook events** for debugging

---

## Next Steps

1. ✅ Implement Alchemy webhook methods (Done)
2. ⏳ Create `AlchemyWebhook` database schema
3. ⏳ Implement webhook handler endpoint
4. ⏳ Add signature verification
5. ⏳ Test with testnet orders
6. ⏳ Deploy to production

---

**Status**: Implementation complete, testing pending
**Last Updated**: 2025-10-09
