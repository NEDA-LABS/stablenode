#!/bin/bash

# Direct curl test based on Alchemy documentation

echo "🧪 Testing Alchemy Webhook API with curl"
echo "=========================================="
echo ""

# Get token from .env
TOKEN=$(grep "ALCHEMY_AUTH_TOKEN=" .env | cut -d'=' -f2 | tr -d '"' | tr -d ' ')

if [ -z "$TOKEN" ]; then
    echo "❌ No ALCHEMY_AUTH_TOKEN found in .env"
    exit 1
fi

echo "Using token: ${TOKEN:0:10}..."
echo ""

# Test 1: ADDRESS_ACTIVITY type
echo "Test 1: ADDRESS_ACTIVITY webhook type"
echo "--------------------------------------"
curl -X POST https://dashboard.alchemy.com/api/create-webhook \
  -H "X-Alchemy-Token: $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "network": "BASE_SEPOLIA",
    "webhook_type": "ADDRESS_ACTIVITY",
    "webhook_url": "https://test.example.com/webhook",
    "addresses": []
  }' | jq '.' 2>/dev/null || cat

echo ""
echo ""

# Test 2: GRAPHQL type (from docs)
echo "Test 2: GRAPHQL webhook type"
echo "-----------------------------"
curl -X POST https://dashboard.alchemy.com/api/create-webhook \
  -H "X-Alchemy-Token: $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "network": "BASE_SEPOLIA",
    "webhook_type": "GRAPHQL",
    "webhook_url": "https://test.example.com/webhook",
    "graphql_query": "{ block { number } }",
    "addresses": []
  }' | jq '.' 2>/dev/null || cat

echo ""
echo ""

# Test 3: Try with ALCHEMY_API_KEY
echo "Test 3: Using ALCHEMY_API_KEY instead"
echo "--------------------------------------"
API_KEY=$(grep "ALCHEMY_API_KEY=" .env | cut -d'=' -f2 | tr -d '"' | tr -d ' ')

if [ ! -z "$API_KEY" ]; then
    echo "Using API key: ${API_KEY:0:10}..."
    curl -X POST https://dashboard.alchemy.com/api/create-webhook \
      -H "X-Alchemy-Token: $API_KEY" \
      -H "Content-Type: application/json" \
      -d '{
        "network": "BASE_SEPOLIA",
        "webhook_type": "ADDRESS_ACTIVITY",
        "webhook_url": "https://test.example.com/webhook",
        "addresses": []
      }' | jq '.' 2>/dev/null || cat
else
    echo "No ALCHEMY_API_KEY found"
fi

echo ""
echo ""
echo "=========================================="
echo "Analysis:"
echo "- If all return 401: Need correct auth token from webhooks dashboard"
echo "- If one succeeds: Use that webhook type and token"
echo "- Check response for error details"
