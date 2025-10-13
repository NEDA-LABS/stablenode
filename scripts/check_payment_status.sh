#!/bin/bash
# Check payment detection status for receive addresses

echo "🔍 Checking Payment Detection Status..."
echo ""

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Receive addresses from logs
ADDRESSES=(
    "0x013542D234dE04f442a832F475872Acd88Cf0bE4"
    "0xF59EFa9b93db835D7db22D6D6Dfe32c9417104A0"
    "0x15A4fF16425e81D46f3F2a74004AEA47D3Bb23ED"
)

# Load .env for database credentials
if [ -f .env ]; then
    source .env
else
    echo "❌ .env file not found"
    exit 1
fi

echo "📊 Checking orders in database..."
echo "=================================="
echo ""

for addr in "${ADDRESSES[@]}"; do
    echo -e "${YELLOW}Address: $addr${NC}"
    
    # Query database
    result=$(sudo docker exec -i nedapay_db psql -U "$DB_USER" -d "$DB_NAME" -t -c \
        "SELECT 
            po.id, 
            po.status, 
            po.amount, 
            po.amount_paid,
            CASE WHEN po.validated_at IS NOT NULL THEN 'Yes' ELSE 'No' END as validated
        FROM payment_orders po
        JOIN receive_addresses ra ON po.receive_address_payment_order = ra.id
        WHERE ra.address = '$addr'
        ORDER BY po.created_at DESC
        LIMIT 1;" 2>/dev/null)
    
    if [ -z "$result" ]; then
        echo -e "  ${RED}✗ No order found${NC}"
    else
        # Parse result
        order_id=$(echo "$result" | awk '{print $1}')
        status=$(echo "$result" | awk '{print $3}')
        amount=$(echo "$result" | awk '{print $5}')
        amount_paid=$(echo "$result" | awk '{print $7}')
        validated=$(echo "$result" | awk '{print $9}')
        
        echo "  Order ID: $order_id"
        
        if [ "$status" == "validated" ]; then
            echo -e "  ${GREEN}✓ Status: $status${NC}"
        else
            echo -e "  ${YELLOW}⚠ Status: $status${NC}"
        fi
        
        echo "  Amount Required: $amount USDC"
        echo "  Amount Paid: $amount_paid USDC"
        echo "  Validated: $validated"
        
        # Check if amount matches
        if [ "$amount" == "$amount_paid" ]; then
            echo -e "  ${GREEN}✓ Payment complete${NC}"
        else
            echo -e "  ${YELLOW}⚠ Payment incomplete${NC}"
        fi
    fi
    
    echo ""
done

echo "=================================="
echo ""
echo "📝 Recent logs for these addresses:"
echo ""

for addr in "${ADDRESSES[@]}"; do
    echo -e "${YELLOW}Last activity for $addr:${NC}"
    sudo docker logs nedapay_aggregator 2>&1 | grep "$addr" | tail -3
    echo ""
done

echo "=================================="
echo ""
echo "📊 Polling Service Status:"
sudo docker logs nedapay_aggregator 2>&1 | grep "Polling service metrics" | tail -1
echo ""

echo "✅ Check complete!"
