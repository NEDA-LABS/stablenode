#!/bin/bash

# Test Alchemy Smart Account Creation
# This script allows you to test Alchemy without rebuilding Docker
#
# Usage:
#   ./test_alchemy.sh
#
# What it tests:
#   - Database connection
#   - Network configuration (Base Sepolia)
#   - Smart account address generation
#   - Unique address creation (run multiple times to verify)
#
# Requirements:
#   - .env file configured with Alchemy credentials
#   - Database running (docker-compose up -d nedapay_db)
#   - SMART_ACCOUNT_OWNER_ADDRESS set in .env

echo "🧪 Testing Alchemy Service..."
echo "==============================="
echo ""

# Check if .env exists
if [ ! -f .env ]; then
    echo "❌ .env file not found!"
    exit 1
fi

# Run the test
go run cmd/test_alchemy/main.go

echo ""
echo "==============================="
echo "✅ Test completed!"
echo ""
echo "💡 Tip: Run this multiple times to verify unique addresses are generated"
