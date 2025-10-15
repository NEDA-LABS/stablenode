# LinkedAddress - Complete Explanation

## Overview

**LinkedAddress** is a powerful feature that enables **automatic payment order creation** when users send cryptocurrency to a specific smart contract address. It acts as a **persistent payment gateway** that's linked to a specific bank account or mobile money account.

Think of it as a **"crypto-to-fiat bridge address"** - a permanent address where users can send crypto anytime, and the system automatically creates a payment order to send fiat to their linked bank/mobile money account.

---

## Core Concept

### **Traditional Flow (Without LinkedAddress)**
```
1. User creates payment order via API
2. System generates temporary receive address
3. User sends crypto to receive address
4. Order is processed
5. Receive address expires
```

### **LinkedAddress Flow (Simplified)**
```
1. User creates a LinkedAddress once (links it to their bank account)
2. User saves this address
3. Anytime user sends crypto to this address → automatic order creation
4. No API calls needed for subsequent payments
```

---

## Database Schema

### **Fields**

```sql
CREATE TABLE linked_addresses (
    id                  BIGSERIAL PRIMARY KEY,
    created_at          TIMESTAMPTZ NOT NULL,
    updated_at          TIMESTAMPTZ NOT NULL,
    address             VARCHAR NOT NULL UNIQUE,        -- Smart contract address
    salt                BYTEA,                          -- Salt for address generation
    institution         VARCHAR NOT NULL,               -- Bank/MoMo code (e.g., "GTBANK")
    account_identifier  VARCHAR NOT NULL,               -- Account number
    account_name        VARCHAR NOT NULL,               -- Account holder name
    owner_address       VARCHAR NOT NULL UNIQUE,        -- User's wallet address
    last_indexed_block  BIGINT,                         -- Last processed block
    tx_hash             VARCHAR,                        -- Last transaction hash
    metadata            JSONB,                          -- Additional data
    sender_profile_linked_address UUID                  -- FK to sender_profile
);
```

### **Relationships**

- **Belongs to**: `SenderProfile` (one sender can have multiple linked addresses)
- **Has Many**: `PaymentOrder` (all orders created via this linked address)

---

## How It Works

### **1. Creation Process**

When a user creates a LinkedAddress:

```go
// User sends request with bank account details
POST /linked-address
{
    "institution": "GTBANK",
    "account_identifier": "0123456789",
    "account_name": "John Doe"
}

// System generates a smart contract address
address := receiveAddressService.CreateSmartAddress(ctx, "")

// System creates LinkedAddress record
linkedAddress := storage.Client.LinkedAddress.
    Create().
    SetAddress(address).                    // Smart contract address
    SetInstitution("GTBANK").               // Bank code
    SetAccountIdentifier("0123456789").     // Account number
    SetAccountName("John Doe").             // Account name
    SetOwnerAddress(userWalletAddress).     // User's wallet
    Save(ctx)
```

**Result**: User gets a permanent smart contract address like:
```
0x1234...5678
```

### **2. Automatic Order Creation**

The system continuously monitors blockchain for transfers to all linked addresses:

```go
// Blockchain indexer detects transfer
Transfer Event:
  From: 0xUser...Wallet
  To: 0x1234...5678 (LinkedAddress)
  Value: 100 USDC
  Block: 12345678

// System automatically:
1. Finds the LinkedAddress record
2. Gets institution and account details
3. Fetches current exchange rate
4. Creates PaymentOrder automatically
5. Creates PaymentOrderRecipient with bank details
6. Processes the order (assigns to provider)
```

**Key Code** (from `indexer.go` lines 89-270):

```go
func ProcessLinkedAddresses(ctx context.Context, ...) {
    // Find linked addresses that received transfers
    linkedAddresses := storage.Client.LinkedAddress.
        Query().
        Where(linkedaddress.AddressIn(unknownAddresses...)).
        All(ctx)
    
    for _, linkedAddress := range linkedAddresses {
        // Get transfer details
        transferEvent := addressToEvent[linkedAddress.Address]
        
        // Check if order already exists (prevent duplicates)
        paymentOrderExists := checkExistingOrder(...)
        if paymentOrderExists {
            return
        }
        
        // Get institution and currency
        institution := utils.GetInstitutionByCode(ctx, linkedAddress.Institution)
        
        // Get exchange rate
        rate := utils.GetTokenRateFromQueue(token.Symbol, amount, currency)
        
        // Create payment order automatically
        order := storage.Client.PaymentOrder.
            Create().
            SetAmount(transferEvent.Value).
            SetRate(rate).
            SetToken(token).
            SetLinkedAddress(linkedAddress).
            SetFromAddress(transferEvent.From).
            Save(ctx)
        
        // Create recipient with linked account details
        storage.Client.PaymentOrderRecipient.
            Create().
            SetInstitution(linkedAddress.Institution).
            SetAccountIdentifier(linkedAddress.AccountIdentifier).
            SetAccountName(linkedAddress.AccountName).
            SetPaymentOrder(order).
            Save(ctx)
        
        // Process the order (assign to provider)
        orderService.CreateOrder(ctx, order.ID)
    }
}
```

---

## Use Cases

### **1. Recurring Payments**
Users who frequently convert crypto to fiat can save their LinkedAddress and reuse it:
```
User sends 100 USDC → Automatic order → Fiat sent to bank
User sends 50 USDC  → Automatic order → Fiat sent to bank
User sends 200 USDC → Automatic order → Fiat sent to bank
```

### **2. Payment Widgets**
Integrate LinkedAddress into apps/websites:
```html
<!-- Show QR code with LinkedAddress -->
<img src="qr-code-for-0x1234...5678" />
<p>Send USDC to this address to receive NGN in your bank account</p>
```

### **3. Wallet Integration**
Save LinkedAddress in wallet address book:
```
Contacts:
- John's Bank Account: 0x1234...5678
- Savings Account: 0xABCD...EFGH
```

### **4. Automated Salary/Payments**
Employers can send crypto salaries to employee LinkedAddresses:
```
Employee provides LinkedAddress → Employer sends monthly crypto → Auto-converts to fiat
```

---

## Key Features

### **1. Persistent Address**
- Unlike receive addresses (which expire), LinkedAddress is permanent
- Can be reused indefinitely
- No need to create new orders via API

### **2. Automatic Processing**
- No API calls needed after initial setup
- System detects transfers automatically
- Orders are created and processed without user intervention

### **3. Account Binding**
- Linked to specific bank/mobile money account
- All transfers to this address go to the same account
- Reduces errors and simplifies user experience

### **4. Multi-Currency Support**
- Works with any supported token (USDC, DAI, USDT, etc.)
- Automatic rate conversion based on token and currency
- Supports all enabled fiat currencies

### **5. Transaction History**
- All orders created via LinkedAddress are tracked
- Users can query transaction history
- Blockchain-level traceability

---

## API Endpoints

### **1. Create LinkedAddress**
```http
POST /linked-address
Authorization: Bearer <wallet-signature>

Request:
{
    "institution": "GTBANK",
    "account_identifier": "0123456789",
    "account_name": "John Doe"
}

Response:
{
    "status": "success",
    "message": "Linked address created successfully",
    "data": {
        "linked_address": "0x1234...5678",
        "institution": "GTBANK",
        "account_identifier": "0123456789",
        "account_name": "John Doe",
        "created_at": "2024-01-01T00:00:00Z",
        "updated_at": "2024-01-01T00:00:00Z"
    }
}
```

### **2. Get LinkedAddress**
```http
GET /linked-address?owner_address=0xUser...Wallet

Response:
{
    "status": "success",
    "message": "Linked address fetched successfully",
    "data": {
        "linked_address": "0x1234...5678",
        "currency": "NGN"
    }
}

// If authenticated (with wallet signature):
{
    "status": "success",
    "data": {
        "linked_address": "0x1234...5678",
        "currency": "NGN",
        "account_identifier": "0123456789",
        "account_name": "John Doe",
        "institution": "Guaranty Trust Bank"
    }
}
```

### **3. Get LinkedAddress Transactions**
```http
GET /linked-address/:address/transactions?page=1&page_size=10

Response:
{
    "status": "success",
    "message": "Transactions fetched successfully",
    "data": {
        "orders": [
            {
                "id": "uuid",
                "amount": "100.00",
                "token": "USDC",
                "rate": "1658.10",
                "status": "settled",
                "tx_hash": "0xabc...def",
                "created_at": "2024-01-01T00:00:00Z"
            }
        ],
        "pagination": {
            "page": 1,
            "page_size": 10,
            "total": 25
        }
    }
}
```

---

## Security Considerations

### **1. Owner Verification**
- LinkedAddress is tied to `owner_address` (user's wallet)
- Only the owner can view sensitive details (account number, name)
- Public can only see the address and currency

### **2. Smart Contract Address**
- Generated using deterministic method with salt
- Cannot be predicted or spoofed
- Unique per user

### **3. Duplicate Prevention**
- System checks for existing orders before creating new ones
- Uses `from_address`, `amount`, and `block_number` to prevent duplicates
- Transaction idempotency guaranteed

### **4. Rate Protection**
- Uses current market rate at time of transfer
- Rate is fetched from priority queue
- Protects against stale rates

---

## Differences: LinkedAddress vs ReceiveAddress

| Feature | LinkedAddress | ReceiveAddress |
|---------|---------------|----------------|
| **Lifespan** | Permanent | Temporary (expires) |
| **Creation** | User-initiated | System-generated per order |
| **Purpose** | Reusable payment gateway | One-time order payment |
| **Order Creation** | Automatic on transfer | Manual via API first |
| **Account Binding** | Linked to specific account | Linked to specific order |
| **Use Case** | Recurring payments | Single transactions |
| **Status** | Always active | unused → used → expired |
| **API Required** | Only for initial setup | Required for each order |

---

## Implementation Flow

### **Complete User Journey**

```
Step 1: User Creates LinkedAddress
├── User provides bank account details
├── System generates smart contract address
├── System stores LinkedAddress record
└── User receives permanent address

Step 2: User Sends Crypto (First Time)
├── User sends 100 USDC to LinkedAddress
├── Blockchain indexer detects transfer
├── System finds LinkedAddress record
├── System creates PaymentOrder automatically
├── System creates PaymentOrderRecipient
├── Order is assigned to provider
├── Provider fulfills order
└── Fiat sent to user's bank account

Step 3: User Sends Crypto (Subsequent Times)
├── User sends 50 USDC to same LinkedAddress
├── Same automatic process
└── No additional setup needed

Step 4: User Checks History
├── User queries LinkedAddress transactions
└── Sees all orders created via this address
```

---

## Monitoring & Indexing

### **How System Detects Transfers**

The blockchain indexer continuously monitors for transfers:

```go
// Indexer runs every N seconds/blocks
1. Fetch recent blockchain events
2. Filter transfers to known addresses:
   - Receive addresses (temporary)
   - Linked addresses (permanent)
   - Provider addresses
3. For each transfer to LinkedAddress:
   - Check if order already exists
   - If not, create new order
   - Process order through normal flow
4. Update last_indexed_block
```

### **Indexing State**

```go
// LinkedAddress tracks indexing state
last_indexed_block: 12345678  // Last processed block
tx_hash: "0xabc...def"        // Last processed transaction

// Prevents:
- Duplicate order creation
- Re-processing old transactions
- Missing transactions during restarts
```

---

## Admin Operations

### **View LinkedAddresses**

```sql
-- Get all linked addresses with stats
SELECT 
    la.address,
    la.institution,
    la.account_identifier,
    la.owner_address,
    la.created_at,
    COUNT(po.id) as total_orders,
    SUM(po.amount) as total_volume
FROM linked_addresses la
LEFT JOIN payment_orders po ON la.id = po.linked_address_payment_orders
GROUP BY la.id;
```

### **Monitor Activity**

```sql
-- Get recent LinkedAddress activity
SELECT 
    la.address,
    la.institution,
    po.amount,
    po.status,
    po.created_at
FROM linked_addresses la
JOIN payment_orders po ON la.id = po.linked_address_payment_orders
WHERE po.created_at > NOW() - INTERVAL '24 hours'
ORDER BY po.created_at DESC;
```

### **Troubleshooting**

```sql
-- Find LinkedAddresses with failed orders
SELECT 
    la.address,
    la.institution,
    COUNT(CASE WHEN po.status = 'refunded' THEN 1 END) as failed_orders,
    COUNT(po.id) as total_orders
FROM linked_addresses la
JOIN payment_orders po ON la.id = po.linked_address_payment_orders
GROUP BY la.id
HAVING COUNT(CASE WHEN po.status = 'refunded' THEN 1 END) > 0;
```

---

## Best Practices

### **For Users**

1. **Save Your LinkedAddress**: Store it securely like any other important address
2. **Verify Institution**: Double-check bank/mobile money details before creating
3. **Test with Small Amount**: Send a small amount first to verify everything works
4. **Monitor Transactions**: Regularly check transaction history
5. **Keep Owner Address Secure**: Only you should have access to the owner wallet

### **For Developers**

1. **Display QR Codes**: Make it easy for users to send crypto
2. **Show Transaction History**: Provide clear visibility of all orders
3. **Handle Errors Gracefully**: Show clear messages if order creation fails
4. **Support Multiple Addresses**: Allow users to create multiple LinkedAddresses for different accounts
5. **Implement Webhooks**: Notify users when orders are created automatically

### **For System Administrators**

1. **Monitor Indexer Health**: Ensure blockchain indexer is running smoothly
2. **Check for Stuck Orders**: Monitor orders created via LinkedAddress
3. **Rate Accuracy**: Verify rates are current and accurate
4. **Duplicate Prevention**: Ensure duplicate detection is working
5. **Performance**: Monitor query performance for LinkedAddress lookups

---

## Common Issues & Solutions

### **Issue 1: Order Not Created Automatically**

**Possible Causes:**
- Indexer not running
- Transfer to wrong address
- Insufficient amount (below minimum)
- Token not supported

**Solution:**
```bash
# Check indexer status
# Verify transfer on blockchain explorer
# Check system logs for errors
```

### **Issue 2: Duplicate Orders**

**Possible Causes:**
- Indexer processed same block twice
- Duplicate detection failed

**Solution:**
```sql
-- System prevents this via:
WHERE 
    paymentorder.FromAddress(transferEvent.From),
    paymentorder.AmountEQ(orderAmount),
    paymentorder.HasLinkedAddressWith(
        linkedaddress.AddressEQ(linkedAddress.Address),
        linkedaddress.LastIndexedBlockEQ(int64(transferEvent.BlockNumber)),
    )
```

### **Issue 3: Wrong Rate Applied**

**Possible Causes:**
- Stale market rate
- Priority queue not updated

**Solution:**
- Ensure market rates are updated regularly
- Refresh priority queues periodically

---

## Future Enhancements

### **Potential Features**

1. **Multiple Accounts per Address**: Support routing to different accounts based on amount or memo
2. **Conditional Routing**: Route to different accounts based on time, amount, or other conditions
3. **Notification System**: SMS/Email notifications when orders are created
4. **Rate Limits**: Prevent abuse by limiting orders per time period
5. **Whitelisting**: Only accept transfers from specific addresses
6. **Metadata Support**: Allow users to pass additional data via transaction memo
7. **Multi-Signature**: Require multiple signatures for high-value transfers

---

## Conclusion

**LinkedAddress** is a powerful feature that bridges the gap between crypto and fiat by providing:

- ✅ **Permanent payment gateway** for recurring conversions
- ✅ **Automatic order creation** without API calls
- ✅ **Account binding** for simplified user experience
- ✅ **Blockchain-level traceability** for all transactions
- ✅ **Multi-currency support** for global reach

It's ideal for users who frequently convert crypto to fiat and want a seamless, automated experience without the complexity of API integration for each transaction.

**Key Takeaway**: LinkedAddress transforms the payment flow from "create order → get address → send crypto" to simply "send crypto" - making crypto-to-fiat conversion as simple as sending a regular crypto transfer.
