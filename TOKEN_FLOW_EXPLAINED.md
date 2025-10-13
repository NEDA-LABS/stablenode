# Token Flow for Receive Address Payment Orders

## 🔄 **Complete Token Journey**

### **Step 1: User Sends Tokens to Receive Address**
```
User Wallet → 0x013542D234dE04f442a832F475872Acd88Cf0bE4 (Receive Address)
Amount: 0.5 USDC
```

**What happens:**
- User sends USDC directly to the receive address
- Receive address is a **temporary holding address** controlled by your system
- Tokens sit there temporarily

---

### **Step 2: System Detects Payment**
```
Indexer/Polling → Detects Transfer event → Updates order status to 'validated'
```

**What happens:**
- Indexer detects the USDC transfer
- Calls `UpdateReceiveAddressStatus()`
- Updates payment order status: `pending` → `validated`
- Marks receive address as `used`

---

### **Step 3: CreateOrder is Called**
```
UpdateReceiveAddressStatus() → createOrder(ctx, paymentOrder.ID)
```

**This is the KEY step!** Let me explain what `CreateOrder` does:

---

## 🎯 **What CreateOrder Does**

Looking at the code in `services/order/evm.go`:

### **Step 3a: Prepare Transaction Data**
```go
// Line 134-146: Encrypt recipient information
encryptedOrderRecipient := encrypt(order.Recipient)
order.SetMessageHash(encryptedOrderRecipient)
order.SetStatus("initiated")
```

### **Step 3b: Create Approval Transaction**
```go
// Line 161-167: Approve gateway to spend tokens
approveGatewayData = approve(
    gatewayContract,
    amount + senderFee
)
```

### **Step 3c: Create Order Transaction**
```go
// Line 155-158: Create order on gateway contract
createOrderData = gatewayContract.createOrder(
    token,
    amount,
    rate,
    senderFeeRecipient,
    senderFee,
    refundAddress,
    encryptedRecipient
)
```

### **Step 3d: Send Batch Transaction**
```go
// Line 170-186: Execute transactions from receive address
txPayload = [
    {
        to: USDC_CONTRACT,
        data: approve(gateway, amount),
        from: RECEIVE_ADDRESS  // 0x013542...
    },
    {
        to: GATEWAY_CONTRACT,
        data: createOrder(...),
        from: RECEIVE_ADDRESS  // 0x013542...
    }
]

engineService.SendTransactionBatch(receiveAddress, txPayload)
```

---

## 💰 **Where Do Tokens Go?**

### **Final Destination: Gateway Contract**

```
Receive Address (0x013542...) → Gateway Contract (0x2135439...)
```

**The flow:**
1. ✅ User sends 0.5 USDC to receive address
2. ✅ System detects payment
3. ✅ System calls `CreateOrder()`
4. 🔄 **Receive address approves gateway to spend USDC**
5. 🔄 **Receive address calls `createOrder()` on gateway**
6. 💰 **Gateway pulls USDC from receive address into itself**
7. 🔒 **Tokens are now locked in gateway contract**

---

## 🏦 **Gateway Contract Role**

The gateway contract acts as an **escrow**:

```
Gateway Contract (0x2135439098B4a1880181f22cf9d4b25b8967f7B2)
├── Holds USDC in escrow
├── Emits OrderCreated event
├── Waits for provider to fulfill order
└── Releases USDC to provider when settled
```

---

## 📊 **Complete Flow Diagram**

```
┌─────────────┐
│  User Wallet│
└──────┬──────┘
       │ 1. Send 0.5 USDC
       ▼
┌─────────────────────────────┐
│  Receive Address            │
│  0x013542...                │ ← Temporary holding
└──────┬──────────────────────┘
       │ 2. System detects payment
       │ 3. CreateOrder() called
       │ 4. Approve gateway
       │ 5. Call createOrder()
       ▼
┌─────────────────────────────┐
│  Gateway Contract           │
│  0x2135439...               │ ← Escrow
│  - Holds 0.5 USDC           │
│  - Emits OrderCreated       │
└──────┬──────────────────────┘
       │ 6. Provider accepts order
       │ 7. Provider sends fiat
       │ 8. System calls settleOrder()
       ▼
┌─────────────────────────────┐
│  Provider Address           │
│  (Gets USDC)                │
└─────────────────────────────┘
```

---

## 🔍 **Why "No transactions found for gateway contract"?**

This message appears because the system checks **two types** of orders:

### **Type 1: Lock Payment Orders**
- User sends USDC **directly to gateway**
- Gateway emits `OrderCreated` immediately
- System looks for these events

### **Type 2: Receive Address Orders (Your Case)**
- User sends USDC **to receive address first**
- Receive address **then** sends to gateway
- Two-step process

The log message:
```
INFO No transactions found for gateway contract: 0x2135439...
```

Means: "No **direct** deposits to gateway found"

But that's OK because you're using **receive addresses**, which is a different flow!

---

## ⚠️ **Current Issue**

Based on your logs, here's what's happening:

```
✅ Step 1: User sent 0.5 USDC to receive address
✅ Step 2: System detected the transfer
✅ Step 3: UpdateReceiveAddressStatus() called
❓ Step 4: CreateOrder() called but...
```

**The question is:** Did `CreateOrder()` successfully send the tokens to the gateway?

Let me check your logs for errors:

```
ERROR 2025-10-10T15:14:59Z Failed to update receive address status when indexing ERC20 transfers for base-sepolia | 
Function=func2, 
Error=UpdateReceiveAddressStatus.CreateOrder: 0120cc93 - CreateOrder.sendTransactionBatch: failed to parse JSON response: 405
```

**Found it!** ❌

---

## 🚨 **The Problem**

```
CreateOrder.sendTransactionBatch: failed to parse JSON response: 405
```

**HTTP 405 = Method Not Allowed**

This means:
- ✅ Payment detected
- ✅ Order validated
- ❌ **Failed to send tokens from receive address to gateway**
- ❌ **Tokens are stuck in receive address**

---

## 🔧 **Why This Happens**

The `SendTransactionBatch()` function uses **ThirdWeb Engine** to send transactions from the receive address.

**Possible causes:**
1. **ThirdWeb Engine API endpoint wrong** - 405 suggests wrong HTTP method
2. **ThirdWeb Engine not configured** - Missing or invalid credentials
3. **Alchemy service should be used instead** - Based on your `.env` settings

---

## 🎯 **Solution**

Check your `.env` configuration:

```bash
# Which service are you using?
USE_ALCHEMY_SERVICE=false  # or true?
USE_ALCHEMY_FOR_RECEIVE_ADDRESSES=true  # Should be true!

# ThirdWeb Engine Config
ENGINE_BASE_URL=
ENGINE_ACCESS_TOKEN=

# Alchemy Config
ALCHEMY_API_KEY=5ukL-3qOfD-XufI4Pkf2z  # ✅ You have this
```

**If you want to use Alchemy for receive addresses:**

1. Set `USE_ALCHEMY_FOR_RECEIVE_ADDRESSES=true`
2. Ensure Alchemy service is properly configured
3. Restart aggregator

**OR if you want to use ThirdWeb Engine:**

1. Configure `ENGINE_BASE_URL` and `ENGINE_ACCESS_TOKEN`
2. Ensure ThirdWeb Engine is running
3. Restart aggregator

---

## 📝 **Summary**

### **Where tokens go:**
```
User → Receive Address → Gateway Contract → Provider
```

### **Current status:**
```
User → Receive Address ❌ (stuck here)
                       ↓
                   Gateway Contract (not reached)
```

### **Why stuck:**
```
CreateOrder.sendTransactionBatch: failed to parse JSON response: 405
```

### **Next steps:**
1. Check which service you want to use (Alchemy vs ThirdWeb)
2. Configure the appropriate service in `.env`
3. Restart aggregator
4. Tokens will be moved from receive address to gateway
5. Provider can then fulfill the order

---

## 🔍 **Check Current Token Balance**

To verify tokens are still in receive address:

```bash
# Check on BaseScan
https://sepolia.basescan.org/address/0x013542D234dE04f442a832F475872Acd88Cf0bE4

# Should show: 0.5 USDC balance
```

Once `CreateOrder()` succeeds, this balance will go to 0 (moved to gateway).
