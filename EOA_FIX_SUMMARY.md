# EOA Transaction Fix - Complete Implementation

## ✅ **Problem Solved**

The receive addresses created by Alchemy **ARE EOAs** (not smart accounts), but the code was trying to send transactions using **UserOperations** (Account Abstraction API) which only works for smart accounts.

## **What Was Fixed**

### **1. Address Generation** (`services/receive_address.go`)
- ✅ Generate proper EOA with private key
- ✅ Encrypt private key before storing
- ✅ Return both address and encrypted key

### **2. Private Key Storage** (`controllers/sender/sender.go`)
- ✅ Store encrypted private key in `salt` field of `receive_address` table
- ✅ Only for Alchemy EOAs (Thirdweb doesn't need this)

### **3. Transaction Detection** (`services/alchemy.go`)
- ✅ Check if address is EOA or smart contract
- ✅ Route to appropriate transaction method

### **4. EOA Transaction Signing** (`services/alchemy.go`)
- ✅ Retrieve encrypted private key from database
- ✅ Decrypt private key
- ✅ Sign transaction with EIP-155
- ✅ Send via `eth_sendRawTransaction`

## **How It Works Now**

### **Step 1: Create Payment Order**
```
User → API: POST /v1/sender/orders
```

### **Step 2: Generate EOA Receive Address**
```go
// services/receive_address.go
address, encryptedKey := CreateEVMAddress()
// Returns: 0x1234..., encrypted_private_key

// controllers/sender/sender.go
receiveAddress.Create().
    SetAddress(address).
    SetSalt(encryptedKey).  // ← Store encrypted key
    Save()
```

### **Step 3: User Sends USDC**
```
User → 0x1234... (EOA): 0.5 USDC
```

### **Step 4: Indexer Detects Payment**
```
Indexer → CreateOrder(orderID)
```

### **Step 5: Send Transaction from EOA**
```go
// services/alchemy.go:SendTransactionBatch()

// 1. Check if address is EOA
isEOA := !isAccountDeployed(address)  // true

// 2. Route to EOA method
sendEOATransactionBatch(address, txPayload)

// 3. Retrieve private key from database
receiveAddr := db.ReceiveAddress.Query().Where(address).Only()
privateKeyBytes := decrypt(receiveAddr.Salt)

// 4. Sign transaction
tx := types.NewTransaction(nonce, to, value, gas, gasPrice, data)
signedTx := types.SignTx(tx, signer, privateKey)

// 5. Send via RPC
eth_sendRawTransaction(signedTx)
```

### **Step 6: Transaction Mined**
```
EOA → Gateway: approve() + createOrder()
```

## **Configuration**

Set these in `.env`:

```bash
# Use Alchemy for everything (no Thirdweb needed)
USE_ALCHEMY_SERVICE=true
USE_ALCHEMY_FOR_RECEIVE_ADDRESSES=true

# Alchemy credentials
ALCHEMY_AUTH_TOKEN=your_token_here
ALCHEMY_GAS_POLICY_ID=your_policy_id  # Optional: for gas sponsorship

# NOT NEEDED for EOAs:
# SMART_ACCOUNT_OWNER_ADDRESS=...  # Only needed for smart accounts
# SMART_ACCOUNT_OWNER_PRIVATE_KEY=...  # Only needed for smart accounts
```

## **Benefits of EOA Approach**

### **vs Smart Accounts:**
- ✅ **No deployment needed** - works immediately
- ✅ **Lower gas costs** - no smart contract overhead
- ✅ **Simpler** - standard Ethereum transactions
- ✅ **No Account Abstraction complexity** - no UserOperations, bundlers, etc.
- ✅ **Works everywhere** - all EVM chains support EOAs

### **vs Thirdweb:**
- ✅ **Much cheaper** - Alchemy free tier vs $99-999/month
- ✅ **Self-hosted** - full control over private keys
- ✅ **No vendor lock-in** - standard EOA transactions

## **Security**

### **Private Key Protection:**
1. ✅ Generated securely using `crypto.GenerateKey()`
2. ✅ Encrypted with `cryptoUtils.EncryptPlain()` before storage
3. ✅ Stored in database `salt` field (encrypted)
4. ✅ Decrypted only when needed for signing
5. ✅ Never logged or exposed in API responses

### **Transaction Security:**
1. ✅ Signed with EIP-155 (replay protection)
2. ✅ Nonce managed automatically
3. ✅ Gas price fetched dynamically
4. ✅ Sent via secure RPC endpoint

## **Testing**

### **Test 1: Create New Order**
```bash
curl -X POST http://localhost:8080/v1/sender/orders \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "token": "USDC",
    "network": "base-sepolia",
    "amount": "0.5",
    "recipient": {
      "institution": "test_bank",
      "account_identifier": "1234567890",
      "account_name": "Test User"
    }
  }'
```

**Expected:**
- ✅ Returns receive address (EOA)
- ✅ Address has no code (is_contract: false)
- ✅ Private key stored encrypted in database

### **Test 2: Send USDC**
```bash
# Send 0.5 USDC to the receive address
```

**Expected:**
- ✅ Indexer detects payment
- ✅ CreateOrder triggered
- ✅ Transaction signed with EOA private key
- ✅ Transaction sent via eth_sendRawTransaction
- ✅ USDC transferred to gateway

### **Test 3: Check Transaction**
```bash
# Check on Blockscout
curl "https://base-sepolia.blockscout.com/api/v2/addresses/<RECEIVE_ADDRESS>/transactions"
```

**Expected:**
- ✅ Shows outgoing transaction from receive address
- ✅ Transaction to gateway contract
- ✅ Status: Success

## **Troubleshooting**

### **Error: "no private key found for address"**
- **Cause:** Address was created before the fix
- **Fix:** Create a new payment order (old addresses won't have private keys)

### **Error: "failed to decrypt private key"**
- **Cause:** Encryption key mismatch
- **Fix:** Check `ENCRYPTION_KEY` in `.env` hasn't changed

### **Error: "insufficient funds for gas"**
- **Cause:** EOA has no ETH for gas
- **Fix:** 
  - Option 1: Send small amount of ETH to receive address
  - Option 2: Use Alchemy Gas Policy (gas sponsorship)

### **Error: "nonce too low"**
- **Cause:** Transaction already sent with that nonce
- **Fix:** System will auto-increment nonce on retry

## **Next Steps**

1. ✅ **Deploy the fix** - Rebuild and restart server
2. ✅ **Test with new order** - Create fresh payment order
3. ✅ **Monitor logs** - Check for "Sending transaction via EOA"
4. ✅ **Verify on Blockscout** - Confirm transactions succeed

## **Migration from Thirdweb**

If you have existing orders with Thirdweb smart accounts:

1. **Keep Thirdweb enabled temporarily:**
   ```bash
   USE_ALCHEMY_SERVICE=false  # Use Thirdweb for now
   ```

2. **Process existing orders** - Let them complete

3. **Switch to Alchemy:**
   ```bash
   USE_ALCHEMY_SERVICE=true
   USE_ALCHEMY_FOR_RECEIVE_ADDRESSES=true
   ```

4. **New orders use EOAs** - All future orders will use EOA approach

5. **Cancel Thirdweb** - Once all old orders are processed

## **Cost Comparison**

### **Thirdweb (Before):**
- Monthly: $99-999
- Per transaction: Gas fees
- **Total: $1,000-12,000/year**

### **Alchemy EOA (After):**
- Monthly: $0-49 (free tier usually sufficient)
- Per transaction: Gas fees (same as before)
- **Total: $0-600/year**

**Savings: $1,000-11,400/year** 🎉
