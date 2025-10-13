#!/bin/bash

# Check Smart Account Deployment Status
# Usage: ./check_smart_account.sh <address>

ADDRESS=$1

if [ -z "$ADDRESS" ]; then
    echo "Usage: ./check_smart_account.sh <address>"
    exit 1
fi

echo "=== Checking Smart Account: $ADDRESS ==="
echo ""

# Check if it's a contract
echo "1. Checking if deployed..."
IS_CONTRACT=$(curl -s "https://base-sepolia.blockscout.com/api/v2/addresses/$ADDRESS" | jq -r '.is_contract')

if [ "$IS_CONTRACT" = "true" ]; then
    echo "   ✅ DEPLOYED (is_contract: true)"
else
    echo "   ❌ NOT DEPLOYED (is_contract: false)"
fi
echo ""

# Check USDC balance
echo "2. Checking USDC balance..."
USDC_BALANCE=$(curl -s "https://base-sepolia.blockscout.com/api/v2/addresses/$ADDRESS/token-balances" | jq -r '.[] | select(.token.address == "0x036CbD53842c5426634e7929541eC2318f3dCF7e") | .value')

if [ -z "$USDC_BALANCE" ] || [ "$USDC_BALANCE" = "null" ]; then
    echo "   ❌ No USDC balance"
else
    # Convert from wei (6 decimals for USDC)
    USDC_HUMAN=$(echo "scale=6; $USDC_BALANCE / 1000000" | bc)
    echo "   ✅ USDC Balance: $USDC_HUMAN USDC"
fi
echo ""

# Check ETH balance
echo "3. Checking ETH balance..."
ETH_BALANCE=$(curl -s "https://base-sepolia.blockscout.com/api/v2/addresses/$ADDRESS" | jq -r '.coin_balance')

if [ -z "$ETH_BALANCE" ] || [ "$ETH_BALANCE" = "null" ] || [ "$ETH_BALANCE" = "0" ]; then
    echo "   ✅ ETH Balance: 0 (gas will be sponsored by Alchemy)"
else
    ETH_HUMAN=$(echo "scale=6; $ETH_BALANCE / 1000000000000000000" | bc)
    echo "   ℹ️  ETH Balance: $ETH_HUMAN ETH"
fi
echo ""

# Check transactions
echo "4. Checking transactions..."
TX_COUNT=$(curl -s "https://base-sepolia.blockscout.com/api/v2/addresses/$ADDRESS/transactions" | jq -r '.items | length')

if [ -z "$TX_COUNT" ] || [ "$TX_COUNT" = "null" ] || [ "$TX_COUNT" = "0" ]; then
    echo "   ❌ No transactions yet"
else
    echo "   ✅ Transactions: $TX_COUNT"
fi
echo ""

# Summary
echo "=== Summary ==="
if [ "$IS_CONTRACT" = "true" ]; then
    echo "✅ Smart account is DEPLOYED"
    echo "   View on Blockscout: https://base-sepolia.blockscout.com/address/$ADDRESS"
else
    echo "❌ Smart account is NOT deployed yet"
    echo ""
    echo "To deploy it:"
    echo "1. Send 0.1 USDC to: $ADDRESS"
    echo "2. Wait for indexer to detect payment"
    echo "3. System will send transaction FROM this address"
    echo "4. Transaction will include initCode to deploy the account"
    echo "5. Alchemy will sponsor the gas"
    echo ""
    echo "Or manually send USDC:"
    echo "cast send 0x036CbD53842c5426634e7929541eC2318f3dCF7e \"transfer(address,uint256)\" $ADDRESS 100000 --rpc-url https://sepolia.base.org --private-key YOUR_KEY"
fi
echo ""
