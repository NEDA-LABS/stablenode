#!/bin/bash

# Quick test to see if current API key works for webhooks
# Sometimes Alchemy uses the same token for both APIs

echo "🧪 Testing if current API key works for webhooks..."
echo ""

# Get current token
TOKEN=$(grep "ALCHEMY_AUTH_TOKEN=" .env | cut -d'=' -f2 | tr -d '"')

if [ -z "$TOKEN" ]; then
    echo "❌ No ALCHEMY_AUTH_TOKEN found in .env"
    exit 1
fi

echo "Token: ${TOKEN:0:10}..."
echo ""

# Test webhook creation with current token
echo "Testing webhook creation..."

RESPONSE=$(curl -s -w "\nHTTP_CODE:%{http_code}" -X POST https://dashboard.alchemy.com/api/create-webhook \
  -H "X-Alchemy-Token: $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "network": "BASE_SEPOLIA",
    "webhook_type": "ADDRESS_ACTIVITY",
    "webhook_url": "https://test.example.com/webhook",
    "addresses": []
  }')

# Extract HTTP code
HTTP_CODE=$(echo "$RESPONSE" | grep "HTTP_CODE:" | cut -d':' -f2)
BODY=$(echo "$RESPONSE" | sed '/HTTP_CODE:/d')

echo "HTTP Status: $HTTP_CODE"
echo "Response: $BODY"
echo ""

case $HTTP_CODE in
    200)
        echo "✅ SUCCESS! Your current API key works for webhooks!"
        echo "You can proceed with testing."
        
        # Extract webhook ID and delete it
        WEBHOOK_ID=$(echo "$BODY" | grep -o '"id":"[^"]*"' | cut -d'"' -f4)
        if [ ! -z "$WEBHOOK_ID" ]; then
            echo "Cleaning up test webhook..."
            curl -s -X DELETE "https://dashboard.alchemy.com/api/delete-webhook?webhook_id=$WEBHOOK_ID" \
              -H "X-Alchemy-Token: $TOKEN"
            echo "✅ Test webhook deleted"
        fi
        ;;
    401)
        echo "❌ UNAUTHORIZED: Your API key doesn't work for webhooks"
        echo "You need to get the Auth Token from the webhooks dashboard"
        echo "See GET_ALCHEMY_AUTH_TOKEN.md for instructions"
        ;;
    400)
        echo "⚠️  BAD REQUEST: API key might work, but request format is wrong"
        echo "This suggests the token is valid but there's another issue"
        ;;
    *)
        echo "❓ Unexpected response: $HTTP_CODE"
        echo "Check Alchemy status or try again later"
        ;;
esac

echo ""
echo "Next steps:"
echo "1. If ✅ SUCCESS: Run ./test_webhook.sh"
echo "2. If ❌ UNAUTHORIZED: Follow GET_ALCHEMY_AUTH_TOKEN.md"
echo "3. If ⚠️ BAD REQUEST: Check webhook URL format"
