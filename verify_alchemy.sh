#!/bin/bash

echo "🔍 Verifying Alchemy Configuration..."
echo ""

# Check if services are running
echo "1. Checking Docker containers..."
docker ps --filter "name=nedapay" --format "table {{.Names}}\t{{.Status}}" || sudo docker ps --filter "name=nedapay" --format "table {{.Names}}\t{{.Status}}"
echo ""

# Check environment variables
echo "2. Checking Alchemy configuration..."
if grep -q "USE_ALCHEMY_SERVICE=true" .env 2>/dev/null; then
    echo "✅ USE_ALCHEMY_SERVICE=true"
else
    echo "❌ USE_ALCHEMY_SERVICE not enabled"
fi

if grep -q "USE_ALCHEMY_FOR_RECEIVE_ADDRESSES=true" .env 2>/dev/null; then
    echo "✅ USE_ALCHEMY_FOR_RECEIVE_ADDRESSES=true"
else
    echo "❌ USE_ALCHEMY_FOR_RECEIVE_ADDRESSES not enabled"
fi

if grep -q "ALCHEMY_API_KEY=" .env 2>/dev/null && ! grep -q "ALCHEMY_API_KEY=your" .env 2>/dev/null; then
    echo "✅ ALCHEMY_API_KEY configured"
else
    echo "❌ ALCHEMY_API_KEY not configured"
fi

if grep -q "SMART_ACCOUNT_OWNER_ADDRESS=" .env 2>/dev/null && ! grep -q "SMART_ACCOUNT_OWNER_ADDRESS=your" .env 2>/dev/null; then
    echo "✅ SMART_ACCOUNT_OWNER_ADDRESS configured"
else
    echo "❌ SMART_ACCOUNT_OWNER_ADDRESS not configured"
fi

echo ""
echo "3. Watching logs for Alchemy activity..."
echo "   (Press Ctrl+C to stop)"
echo ""

# Watch logs for Alchemy-related messages
docker logs -f nedapay_aggregator 2>&1 | grep --line-buffered -i "alchemy\|smart account\|receive address" || \
sudo docker logs -f nedapay_aggregator 2>&1 | grep --line-buffered -i "alchemy\|smart account\|receive address"
