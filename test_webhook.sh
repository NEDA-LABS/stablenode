#!/bin/bash

# Webhook Testing Script
# Run this BEFORE starting the aggregator to verify webhook setup

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}================================${NC}"
echo -e "${BLUE}🧪 Alchemy Webhook Test${NC}"
echo -e "${BLUE}================================${NC}"
echo ""

# Check .env exists
if [ ! -f .env ]; then
    echo -e "${RED}❌ .env file not found${NC}"
    echo "Please create .env from .env.example"
    exit 1
fi

# Check ALCHEMY_AUTH_TOKEN is set
if ! grep -q "ALCHEMY_AUTH_TOKEN=" .env || grep -q "ALCHEMY_AUTH_TOKEN=\"\"" .env; then
    echo -e "${RED}❌ ALCHEMY_AUTH_TOKEN not set in .env${NC}"
    echo ""
    echo "Please add your Alchemy auth token:"
    echo "1. Go to https://dashboard.alchemy.com/settings"
    echo "2. Create auth token with notify:read, notify:write permissions"
    echo "3. Add to .env: ALCHEMY_AUTH_TOKEN=your_token_here"
    exit 1
fi

echo -e "${GREEN}✅ Configuration found${NC}"
echo ""

# Build test tool
echo -e "${YELLOW}Building webhook test tool...${NC}"
go build -o test_webhook_tool ./cmd/test_webhook/main.go

if [ $? -ne 0 ]; then
    echo -e "${RED}❌ Build failed${NC}"
    exit 1
fi

echo -e "${GREEN}✅ Build successful${NC}"
echo ""

# Run test tool
echo -e "${YELLOW}Running webhook tests...${NC}"
echo ""
./test_webhook_tool

# Cleanup
rm -f test_webhook_tool

echo ""
echo -e "${GREEN}================================${NC}"
echo -e "${GREEN}✅ Testing complete!${NC}"
echo -e "${GREEN}================================${NC}"
echo ""
echo "If all tests passed, you can now:"
echo "1. Start the aggregator: docker-compose up -d"
echo "2. Create real orders and test payment detection"
echo ""
