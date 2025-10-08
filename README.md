# NEDAPay "Stablenode" Aggregator Order Lifecycle Documentation by team NEDA ---oct 5, 2025 


**for development setup check (`readme.md`)**

## Overview

This document provides a comprehensive technical overview of the order lifecycle in the NEDA "Stablenode" aggregator system adapted from PAYCREST PROTOCOL, from initial order creation through final settlement or refund. The system implements a sophisticated EVM payment processing pipeline with ERC-4337 Account Abstraction integration and support for multiple blockchain service providers (Alchemy recommended, Thirdweb Engine legacy) for wallet management.

## Order Lifecycle Diagram

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         COMPLETE ORDER LIFECYCLE                             │
└─────────────────────────────────────────────────────────────────────────────┘

1. USER CREATES ORDER (via API)
   │
   ├─→ Aggregator validates request
   ├─→ Creates PaymentOrder in database (status: order_initiated)
   │
   ↓
2. AGGREGATOR GENERATES RECEIVE ADDRESS
   │
   ├─→ Calls Blockchain Service Provider (Alchemy recommended, Thirdweb legacy)
   ├─→ Creates ERC-4337 smart account: 0xRECEIVE_ADDRESS_123
   ├─→ Stores ReceiveAddress in database
   └─→ Returns address to user
   │
   ↓
3. USER SENDS CRYPTO
   │
   └─→ User transfers tokens to: 0xRECEIVE_ADDRESS_123
   │
   ↓
4. AGGREGATOR DETECTS DEPOSIT (via Blockchain Webhooks)
   │
   ├─→ Webhook receives Transfer event (Alchemy Notify or Thirdweb Insight)
   ├─→ Validates: correct token, amount, receive address
   ├─→ Updates order status: crypto_deposited
   │
   ↓
5. AGGREGATOR CREATES ORDER ON GATEWAY CONTRACT
   │
   ├─→ Prepares transaction:
   │   • FROM: AGGREGATOR_SMART_ACCOUNT (0x03Ff...)
   │   • TO: Gateway Contract
   │   • FUNCTION: createOrder(token, amount, rate, recipient, refundAddress)
   │
   ├─→ Sends via Blockchain Service Provider:
   │   • Signs with AGGREGATOR_PRIVATE_KEY (via Alchemy or Thirdweb)
   │   • Transfers funds: 0xRECEIVE_ADDRESS_123 → Gateway Contract
   │
   ├─→ Gateway Contract emits: OrderCreated event
   └─→ Updates database: order_created, records gateway_id
   │
   ↓
6. PROVIDER MATCHING
   │
   ├─→ Creates LockPaymentOrder (status: pending)
   ├─→ Notifies available providers
   └─→ Provider claims order
   │
   ↓
7. PROVIDER FULFILLS ORDER (Off-chain)
   │
   ├─→ Provider sends fiat to recipient
   ├─→ Provider submits proof of payment
   └─→ Aggregator validates fulfillment
   │
   ↓
8. AGGREGATOR SETTLES ORDER ON GATEWAY CONTRACT
   │
   ├─→ Prepares transaction:
   │   • FROM: AGGREGATOR_SMART_ACCOUNT (0x03Ff...)
   │   • TO: Gateway Contract
   │   • FUNCTION: settle(orderId, provider, settlePercent)
   │
   ├─→ Sends via Blockchain Service Provider:
   │   • Signs with AGGREGATOR_PRIVATE_KEY (via Alchemy or Thirdweb)
   │
   ├─→ Gateway Contract:
   │   • Releases funds to provider
   │   • Deducts protocol fees
   │   • Emits: OrderSettled event
   │
   └─→ Updates database: order_fulfilled
   │
   ↓
9. ORDER COMPLETE ✓

┌─────────────────────────────────────────────────────────────────────────────┐
│                         ALTERNATIVE: REFUND PATH                             │
└─────────────────────────────────────────────────────────────────────────────┘

REFUND TRIGGERS:
• Order timeout (no provider claims within ORDER_REFUND_TIMEOUT)
• Provider cancellation (exceeds REFUND_CANCELLATION_COUNT)
• Manual admin refund
│
↓
AGGREGATOR REFUNDS ORDER
│
├─→ Prepares transaction:
│   • FROM: AGGREGATOR_SMART_ACCOUNT (0x03Ff...)
│   • TO: Gateway Contract
│   • FUNCTION: refund(fee, orderId)
│
├─→ Sends via Blockchain Service Provider (Alchemy or Thirdweb)
│
├─→ Gateway Contract:
│   • Returns funds to user's refundAddress
│   • Emits: OrderRefunded event
│
└─→ Updates database: order_refunded

```

## Order Initiation Flow (Detailed)

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                      ORDER INITIATION ARCHITECTURE                           │
└─────────────────────────────────────────────────────────────────────────────┘

CLIENT                  API                 DATABASE            ALCHEMY/THIRDWEB
  │                      │                      │                      │
  │  POST /v1/sender/    │                      │                      │
  │  orders              │                      │                      │
  ├─────────────────────>│                      │                      │
  │                      │                      │                      │
  │                      │  Validate sender     │                      │
  │                      │  & token config      │                      │
  │                      ├─────────────────────>│                      │
  │                      │<─────────────────────┤                      │
  │                      │  Sender & Token OK   │                      │
  │                      │                      │                      │
  │                      │ Calculate Fees:      │                      │
  │                      │ • sender_fee = %     │                      │
  │                      │ • network_fee        │                      │
  │                      │ • protocol_fee = 0   │                      │
  │                      │ • total = amount+fees│                      │
  │                      │                      │                      │
  │                      │                      │                      │
  │                      │ ┌──────────────────────────────────────────┐
  │                      │ │ IF USE_ALCHEMY_FOR_RECEIVE_ADDRESSES     │
  │                      │ └──────────────────────────────────────────┘
  │                      │                      │                      │
  │                      │  CreateSmartAccount(owner, chainID, salt)   │
  │                      ├─────────────────────────────────────────────>│
  │                      │                      │   • Generate unique  │
  │                      │                      │     salt (timestamp) │
  │                      │                      │   • Compute CREATE2  │
  │                      │                      │     address          │
  │                      │<─────────────────────────────────────────────┤
  │                      │  Smart Account Addr  │                      │
  │                      │  (0x9876737E...)     │                      │
  │                      │                      │                      │
  │                      │ ⚠️ Webhook creation  │                      │
  │                      │    SKIPPED           │                      │
  │                      │                      │                      │
  │                      │ ┌──────────────────────────────────────────┐
  │                      │ │ ELSE (Using Thirdweb Engine)             │
  │                      │ └──────────────────────────────────────────┘
  │                      │                      │                      │
  │                      │  CreateServerWallet()│                      │
  │                      ├─────────────────────────────────────────────>│
  │                      │<─────────────────────────────────────────────┤
  │                      │  Wallet Address      │                      │
  │                      │                      │                      │
  │                      │  CreateTransferWebhook(address, token)      │
  │                      ├─────────────────────────────────────────────>│
  │                      │<─────────────────────────────────────────────┤
  │                      │  Webhook ID & Secret │                      │
  │                      │                      │                      │
  │                      │                      │                      │
  │                      │  BEGIN TRANSACTION   │                      │
  │                      ├─────────────────────>│                      │
  │                      │                      │                      │
  │                      │  Create TransactionLog                      │
  │                      │  (status: initiated) │                      │
  │                      ├─────────────────────>│                      │
  │                      │                      │                      │
  │                      │  Create PaymentOrder │                      │
  │                      │  • amount            │                      │
  │                      │  • amount_paid = 0   │                      │
  │                      │  • sender_fee        │                      │
  │                      │  • network_fee       │                      │
  │                      │  • protocol_fee = 0  │                      │
  │                      │  • receive_address   │                      │
  │                      │  • status = initiated│                      │
  │                      ├─────────────────────>│                      │
  │                      │                      │                      │
  │                      │  Create ReceiveAddress                      │
  │                      │  • address           │                      │
  │                      │  • valid_until       │                      │
  │                      │  • label             │                      │
  │                      ├─────────────────────>│                      │
  │                      │                      │                      │
  │                      │  Create PaymentOrderRecipient               │
  │                      │  • institution       │                      │
  │                      │  • account_id        │                      │
  │                      │  • account_name      │                      │
  │                      │  • currency          │                      │
  │                      ├─────────────────────>│                      │
  │                      │                      │                      │
  │                      │  IF Thirdweb:        │                      │
  │                      │  Create PaymentWebhook                      │
  │                      │  • webhook_id        │                      │
  │                      │  • webhook_secret    │                      │
  │                      │  • callback_url      │                      │
  │                      ├─────────────────────>│                      │
  │                      │                      │                      │
  │                      │  COMMIT TRANSACTION  │                      │
  │                      ├─────────────────────>│                      │
  │                      │<─────────────────────┤                      │
  │                      │  Transaction OK      │                      │
  │                      │                      │                      │
  │  201 Created         │                      │                      │
  │  {                   │                      │                      │
  │    order_id,         │                      │                      │
  │    receive_address,  │                      │                      │
  │    amount + fees,    │                      │                      │
  │    valid_until       │                      │                      │
  │  }                   │                      │                      │
  │<─────────────────────┤                      │                      │
  │                      │                      │                      │
  │                      │                      │                      │
┌─────────────────────────────────────────────────────────────────────────────┐
│                    PAYMENT DETECTION (POST-CREATION)                         │
└─────────────────────────────────────────────────────────────────────────────┘
  │                      │                      │                      │
  │  User sends crypto   │                      │                      │
  │  to receive_address  │                      │                      │
  │                      │                      │                      │
  │                      │ ┌──────────────────────────────────────────┐
  │                      │ │ IF Thirdweb Webhook Active               │
  │                      │ └──────────────────────────────────────────┘
  │                      │                      │                      │
  │                      │  POST /v1/insight/webhook                   │
  │                      │  (Transfer event)    │                      │
  │                      │<─────────────────────────────────────────────┤
  │                      │                      │                      │
  │                      │  Update amount_paid  │                      │
  │                      ├─────────────────────>│                      │
  │                      │                      │                      │
  │                      │  Check:              │                      │
  │                      │  amount_paid >=      │                      │
  │                      │  total_amount?       │                      │
  │                      ├─────────────────────>│                      │
  │                      │                      │ DB Trigger:          │
  │                      │                      │ check_payment_       │
  │                      │                      │ order_amount()       │
  │                      │                      │ validates payment    │
  │                      │<─────────────────────┤                      │
  │                      │  Status: validated   │                      │
  │                      │                      │                      │
  │                      │ ┌──────────────────────────────────────────┐
  │                      │ │ ELSE (Alchemy - No Webhook Yet)          │
  │                      │ └──────────────────────────────────────────┘
  │                      │                      │                      │
  │                      │  ⚠️ PAYMENT DETECTION NOT IMPLEMENTED       │
  │                      │                      │                      │
  │                      │  Options:            │                      │
  │                      │  1. Alchemy Notify webhooks (recommended)   │
  │                      │  2. Polling mechanism                       │
  │                      │  3. Blockchain indexer extension            │
  │                      │                      │                      │
```

### **Key Components:**

1. **Fee Calculation**
   - `sender_fee` = Percentage of order amount (e.g., 1%)
   - `network_fee` = Blockchain gas fee estimate
   - `protocol_fee` = Platform fee (currently 0)
   - `total_amount` = amount + sender_fee + network_fee + protocol_fee

2. **Receive Address Generation**
   - **Alchemy**: Deterministic CREATE2 address with unique salt (timestamp-based)
   - **Thirdweb**: Server-managed wallet creation via Engine API

3. **Webhook Management**
   - **Thirdweb**: Automatic webhook creation for transfer monitoring
   - **Alchemy**: Webhook creation skipped (requires separate Alchemy Notify setup)

4. **Database Trigger**
   - `check_payment_order_amount()` validates that `amount_paid >= total_amount`
   - Prevents order fulfillment with insufficient payment
   - Runs automatically on order status updates

5. **Payment Detection Gap (Alchemy)**
   - ⚠️ When using Alchemy receive addresses, payment detection is not yet implemented
   - Orders will be created but won't automatically update when crypto is deposited
   - **Critical**: Requires implementation before production use
   - **Options**:
     - **A. Alchemy Notify** (recommended) - Set up webhooks in Alchemy Dashboard
     - **B. Polling** - Background job to check address balances
     - **C. Indexer** - Extend existing blockchain indexer to monitor Alchemy addresses

---



**Note**: The system currently focuses exclusively on EVM-compatible chains (Ethereum, Base, Arbitrum, Polygon, etc.). Tron support has been removed.

## Architecture Components

### Core Services
- **Order Service**: Handles order creation and smart contract interactions (`services/order/`)
- **Indexer Service**: Monitors blockchain events and updates database state (`services/indexer/`)
- **Service Manager**: Routes operations between Alchemy and Thirdweb services (`services/manager.go`)
- **Alchemy Service**: Manages smart accounts via Alchemy Account Abstraction APIs (`services/alchemy.go`)
- **Engine Service**: Manages wallet operations via Thirdweb Engine API (`services/engine.go`)
- **Receive Address Service**: Generates temporary deposit addresses for orders (`services/receive_address.go`)
- **Priority Queue Service**: Manages order processing queues (`services/priority_queue.go`)

### Blockchain Service Provider Integration

**Alchemy (Recommended)**:
- **Wallet Management**: Creates and manages ERC-4337 smart accounts using deterministic deployment
- **Transaction Signing**: Direct cryptographic signing with self-managed keys
- **Event Monitoring**: Alchemy Notify API for webhook events
- **Key Storage**: Self-managed in environment variables
- **Cost**: $0-49/month (free tier sufficient)

**Thirdweb Engine (Legacy)**:
- **Wallet Management**: Creates and manages ERC-4337 smart accounts via Engine API
- **Transaction Signing**: Signs all transactions using Engine vault
- **Webhook System**: Thirdweb Insight for blockchain events (Transfer, OrderCreated, OrderSettled, OrderRefunded)
- **Key Storage**: Securely stores keys in Thirdweb Engine vault
- **Cost**: $99-999/month subscription

### Database Layer
- **Ent ORM**: Database schema and operations (`ent/`)
- **PostgreSQL**: Primary data store
- **Redis**: Caching and session management

### Smart Contracts
- **Gateway Contract**: Main order processing contract
- **EntryPoint Contract**: ERC-4337 Account Abstraction entry point
- **SimpleAccount**: Smart wallet implementation
- **ERC20 Tokens**: Supported payment tokens

## Order Lifecycle Phases

### Phase 1: Order Initiation

#### 1.1 API Request Processing
**File**: `controllers/index.go`
**Function**: Order creation endpoints

```go
// Entry point for order creation requests
func (ctrl *Controller) CreateOrder(ctx *gin.Context) {
    // Validates request payload
    // Authenticates user
    // Creates initial order record
}
```

**Database Operations**:
- Creates `PaymentOrder` entity with status `order_initiated`
- Links to `Recipient`, `Token`, and `Network` entities
- Generates unique order ID and receive address

#### 1.2 Receive Address Generation
**File**: `services/receive_address.go`

```go
// Creates ERC-4337 smart accounts via Alchemy or Thirdweb
func (s *ReceiveAddressService) CreateSmartAddress(ctx context.Context, label string) (string, error) {
    // If USE_ALCHEMY_FOR_RECEIVE_ADDRESSES=true
    if useAlchemy {
        return s.alchemyService.CreateSmartAccount(ctx, owner, chainID, salt)
        // Generates deterministic CREATE2 address with unique salt
    }
    // Legacy: Thirdweb Engine
    return s.engineService.CreateServerWallet(ctx, label)
    // Calls Thirdweb Engine API to create a new smart account
}
```

**Database Operations**:
- Creates `ReceiveAddress` entity
- Sets expiration time based on `RECEIVE_ADDRESS_VALIDITY`
- Links to payment order

### Phase 2: Crypto Deposit Detection

#### 2.1 Blockchain Monitoring
**File**: `services/indexer/evm.go`

```go
// Monitors blockchain for incoming transfers on EVM chains
func (s *IndexerEVM) IndexReceiveAddress(ctx context.Context, token *ent.Token, address string, fromBlock int64, toBlock int64, txHash string) (*types.EventCounts, error) {
    // Scans for ERC-20 Transfer events to receive address
    // Validates transfer amount and token
    // Triggers order processing
}
```

**Event Processing**:
- Listens for ERC-20 `Transfer` events
- Validates transfer amount meets order requirements
- Updates order status to `crypto_deposited`

#### 2.2 Transfer Event Handling
**File**: `controllers/index.go`
**Function**: `handleTransferEvent`

```go
func (ctrl *Controller) handleTransferEvent(ctx *gin.Context, event types.ThirdwebWebhookEvent) error {
    // Processes incoming transfer events
    // Validates against pending orders
    // Triggers order creation on blockchain
}
```

**Database Operations**:
- Updates `PaymentOrder` status
- Creates `TransactionLog` entries
- Records transfer transaction hash

### Phase 3: Smart Contract Order Creation

#### 3.1 Order Preparation
**File**: `services/order/evm.go`

```go
// Prepares order for blockchain submission on EVM chains
func (s *OrderEVM) CreateOrder(order *ent.PaymentOrder) error {
    // Encrypts recipient details
    // Prepares smart contract call data
    // Submits via Account Abstraction (ERC-4337)
}
```

**Smart Contract Interaction**:
- Calls `Gateway.createOrder()` function
- Passes encrypted recipient data
- Uses ERC-4337 UserOperation for gas-less execution

#### 3.2 Transaction Execution via Blockchain Service Provider
**Files**: 
- `services/alchemy.go` - Alchemy service (recommended)
- `services/engine.go` - Thirdweb Engine (legacy)

```go
// Alchemy: Direct transaction signing
func (s *AlchemyService) SendUserOperation(ctx context.Context, userOp UserOperation) (string, error) {
    // Signs with self-managed AGGREGATOR_PRIVATE_KEY
    // Submits via Alchemy Account Abstraction API
    // Returns operation hash for tracking
}

// Thirdweb Engine: Vault-managed signing
func (s *EngineService) SendTransactionBatch(ctx context.Context, chainID int64, address string, txPayload []map[string]interface{}) (queueID string, err error) {
    // Calls Thirdweb Engine API
    // Engine signs transaction with AGGREGATOR_PRIVATE_KEY (stored in vault)
    // Returns queue ID for tracking
}
```

**Process Flow**:
1. Aggregator prepares transaction payload (createOrder call data)
2. Routes to appropriate service (Alchemy or Thirdweb) via Service Manager
3. Service signs with `AGGREGATOR_PRIVATE_KEY`
4. Transaction submitted to blockchain
5. Funds transferred: receive address → Gateway contract
6. Gateway contract validates and executes order creation

#### 3.3 Gateway Contract Execution
**File**: `services/contracts/Gateway.go`
**Generated from**: Gateway.sol

```solidity
// Gateway contract createOrder function
function createOrder(
    address _token,
    uint256 _amount,
    uint96 _rate,
    address _senderFeeRecipient,
    uint256 _senderFee,
    address _refundAddress,
    string memory messageHash
) external returns (bytes32 orderId)
```

**Contract Operations**:
- Validates token and amount
- Calculates protocol fees
- Emits `OrderCreated` event
- Returns unique order ID

### Phase 4: Event Processing and Database Updates

#### 4.1 OrderCreated Event Handling
**File**: `controllers/index.go`
**Function**: `handleOrderCreatedEvent`

```go
func (ctrl *Controller) handleOrderCreatedEvent(ctx *gin.Context, event types.ThirdwebWebhookEvent) error {
    // Processes OrderCreated events from Gateway contract
    // Updates order status in database
    // Triggers provider notification
}
```

**Event Structure**:
```go
type OrderCreatedEvent struct {
    BlockNumber int64
    TxHash      string
    Token       string
    Amount      decimal.Decimal
    OrderId     string
    Rate        decimal.Decimal
    MessageHash string
}
```

#### 4.2 Database State Updates
**Database Operations**:
- Updates `PaymentOrder` status to `order_created`
- Records blockchain transaction hash
- Creates `LockPaymentOrder` for provider matching
- Updates `TransactionLog` with event details

### Phase 5: Provider Matching and Settlement

#### 5.1 Lock Order Creation
**File**: `services/common/order.go`

```go
// Creates lock orders for provider matching
func CreateLockOrder(order *ent.PaymentOrder) error {
    // Splits order into provider-sized chunks
    // Creates LockPaymentOrder entities
    // Notifies available providers
}
```

**Provider Matching Logic**:
- Queries available providers by token and amount
- Considers provider rates and availability
- Creates lock orders with expiration times

#### 5.2 Provider Settlement
**Files**:
- `controllers/provider/provider.go` - Provider endpoints
- `services/order/evm.go` - Settlement execution

```go
// Processes provider settlement
func (s *OrderEVM) SettleOrder(lockOrder *ent.LockPaymentOrder, provider *ent.ProviderProfile) error {
    // Validates provider settlement
    // Calls Gateway.settle() function
    // Updates order status
}
```

**Settlement Process**:
1. Provider claims lock order
2. Provides off-chain payment proof
3. System validates settlement
4. Calls `Gateway.settle()` with settlement details
5. Emits `OrderSettled` event

### Phase 6: Order Completion

#### 6.1 OrderSettled Event Processing
**File**: `controllers/index.go`
**Function**: `handleOrderSettledEvent`

```go
func (ctrl *Controller) handleOrderSettledEvent(ctx *gin.Context, event types.ThirdwebWebhookEvent) error {
    // Processes settlement events
    // Updates order status to fulfilled
    // Releases provider funds
}
```

#### 6.2 Final Status Updates
**Database Operations**:
- Updates `PaymentOrder` status to `order_fulfilled`
- Updates `LockPaymentOrder` status to `settled`
- Records final settlement transaction
- Calculates and records fees

### Phase 7: Refund Handling (Alternative Path)

#### 7.1 Refund Triggers
**Conditions for Refund**:
- Order timeout (no provider settlement)
- Provider cancellation
- System error conditions
- Manual admin refund

#### 7.2 Refund Execution
**File**: `services/order/evm.go`

```go
// Processes order refunds
func (s *OrderEVM) RefundOrder(order *ent.PaymentOrder) error {
    // Validates refund conditions
    // Calls Gateway.refund() function
    // Returns funds to user
}
```

**Refund Process**:
1. System detects refund condition
2. Calls `Gateway.refund()` with order ID
3. Contract validates and processes refund
4. Emits `OrderRefunded` event
5. Updates database status

## File Structure and Responsibilities

### Controllers Layer
```
controllers/
├── index.go              # Main API endpoints, webhook handlers
├── provider/provider.go  # Provider-specific endpoints
└── sender/sender.go      # Sender/user endpoints
```

### Services Layer
```
services/
├── order/
│   └── evm.go           # EVM-based order processing
├── indexer/
│   └── evm.go           # EVM event indexing
├── common/
│   ├── order.go         # Shared order logic
│   └── indexer.go       # Shared indexing logic
├── contracts/           # Generated contract bindings
├── alchemy.go           # Alchemy service (recommended)
├── engine.go            # Thirdweb Engine service (legacy)
├── manager.go           # Service manager (routes between providers)
├── receive_address.go   # Receive address generation
└── priority_queue.go    # Order queue management
```

### Database Layer
```
ent/
├── paymentorder/        # Main order entities
├── lockpaymentorder/    # Provider lock orders
├── transactionlog/      # Transaction history
├── receiveaddress/      # Generated addresses
└── network/             # Blockchain networks
```

### Utilities
```
utils/
├── userop.go           # Account Abstraction utilities
├── rpc_events.go       # Event decoding utilities
└── crypto/             # Cryptographic utilities
```

## Configuration and Environment

### Key Environment Variables
```bash
# ============================================
# ALCHEMY SERVICE (Recommended)
# ============================================
ALCHEMY_API_KEY=your_alchemy_api_key
ALCHEMY_BASE_URL=https://api.g.alchemy.com/v2
ALCHEMY_GAS_POLICY_ID=your_gas_policy_id  # Optional

# ============================================
# THIRDWEB ENGINE (Legacy)
# ============================================
ENGINE_BASE_URL=https://your-engine-instance.com
ENGINE_ACCESS_TOKEN=your-vault-access-token
THIRDWEB_SECRET_KEY=your-thirdweb-secret-key

# ============================================
# SERVICE SELECTION
# ============================================
USE_ALCHEMY_SERVICE=false  # Set to true to use Alchemy
USE_ALCHEMY_FOR_RECEIVE_ADDRESSES=true  # Use Alchemy for receive addresses

# ============================================
# AGGREGATOR ACCOUNT - Operational Wallet
# ============================================
# The main smart account that executes all order operations
AGGREGATOR_SMART_ACCOUNT=0x03Ff9504c7067980c1637BF9400E7b7e3655782c

# Keys controlling the aggregator account
AGGREGATOR_PRIVATE_KEY="-----BEGIN RSA PRIVATE KEY-----..."
AGGREGATOR_PUBLIC_KEY="-----BEGIN RSA PUBLIC KEY-----..."

# ============================================
# SMART CONTRACT ADDRESSES
# ============================================
ENTRY_POINT_CONTRACT_ADDRESS=0x5FF137D4b0FDCD49DcA30c7CF57E578a026d2789

# ============================================
# ORDER CONFIGURATION
# ============================================
ORDER_FULFILLMENT_VALIDITY=1    # minutes
ORDER_REFUND_TIMEOUT=5          # minutes
RECEIVE_ADDRESS_VALIDITY=30     # minutes
REFUND_CANCELLATION_COUNT=3     # max provider cancellations before refund
```

### Blockchain Service Provider Setup

#### Alchemy Setup (Recommended)
1. **Create Alchemy Account**: Sign up at https://alchemy.com
2. **Get API Key**: Create app and copy API key
3. **Configure Environment**: Set `ALCHEMY_API_KEY` and `USE_ALCHEMY_FOR_RECEIVE_ADDRESSES=true`
4. **Set Up Webhooks** (for payment detection):
   - Go to Alchemy Dashboard → Notify
   - Create Address Activity webhook
   - Point to: `https://your-domain.com/v1/alchemy/webhook`
5. **Optional Gas Manager**: Configure gas sponsorship policies

**Advantages:**
- Free tier sufficient for most use cases ($0-49/month)
- Self-managed keys (no third-party vault)
- Direct API access
- Comprehensive documentation

#### Thirdweb Engine Setup (Legacy)
1. **Deploy Engine**: Self-hosted or cloud instance
2. **Configure**: Set `ENGINE_BASE_URL` and `ENGINE_ACCESS_TOKEN`
3. **Import Keys**: Add `AGGREGATOR_PRIVATE_KEY` to Engine vault
4. **Webhooks**: Automatic via Thirdweb Insight

**Note**: Thirdweb Engine costs $99-999/month. Migration to Alchemy recommended.

### Network Configuration
Each supported EVM network requires:
- RPC endpoint configuration
- Gateway contract address
- Supported token contracts
- Gas price and fee settings
- Webhook configuration (Alchemy Notify or Thirdweb Insight)

## Gateway Contract Deployment Strategy

### **🏗️ Pre-Deployed Contract Approach**

The Gateway contracts are **already deployed** on each supported network and their addresses are stored in the database. The system uses pre-deployed contracts rather than deploying them during runtime.

#### **Current Deployed Gateway Contracts (EVM Testnets):**
```sql
-- From scripts/db_data/dump.sql
INSERT INTO "public"."networks" (..., "gateway_contract_address", ...) VALUES
-- Ethereum Sepolia Testnet
('0xCAD53Ff499155Cc2fAA2082A85716322906886c2'),
-- Arbitrum Sepolia Testnet  
('0x87B321fc77A0fDD0ca1fEe7Ab791131157B9841A'),
-- Base Sepolia Testnet
('0x...')  -- Add your deployed contract address
```

### **📋 How Gateway Addresses Are Managed**

#### **1. Database Storage**
Each network entity stores its Gateway contract address:
```go
type Network struct {
    ChainID                int64
    Identifier            string
    RPCEndpoint           string
    GatewayContractAddress string  // Pre-deployed contract address
    BundlerURL            string
    PaymasterURL          string
}
```

#### **2. Runtime Usage**
Orders are created using the pre-deployed Gateway address from the database:
```go
func (s *OrderEVM) CreateOrder(order *ent.PaymentOrder) error {
    gatewayAddress := order.Edges.Token.Edges.Network.GatewayContractAddress
    // Calls createOrder() on the existing contract
}
```

### **🚀 Deployment Process (Done Separately)**

The Gateway contracts are deployed **outside** of the aggregator application:

1. **Contract Deployment** - Gateway contracts deployed manually/via scripts per network
2. **Database Configuration** - Contract addresses added to database via `scripts/db_data/dump.sql`
3. **Code Generation** - Go bindings generated in `services/contracts/Gateway.go`

### **⚙️ Why This Approach?**

**Advantages:**
- **Stability**: Contract addresses don't change between deployments
- **Gas Efficiency**: No deployment costs during runtime
- **Security**: Contracts can be audited and verified before use
- **Multi-Network**: Each network has its optimized Gateway instance
- **Upgradability**: Can deploy new versions and update database references

### **🔄 Adding New Networks**

To support a new blockchain network:
1. **Deploy Gateway Contract** on the new network
2. **Update Database** with new network record including gateway address
3. **Configure RPC/Bundler** endpoints for the network
4. **Test Integration** with the aggregator

**Related Files:**
```
services/contracts/Gateway.go     # Generated contract bindings
services/order/evm.go            # EVM Gateway interactions
scripts/db_data/dump.sql         # Network/Gateway configuration
ent/network/                     # Database schema for networks
```

## Error Handling and Recovery

### Automatic Recovery
- Failed transactions are retried with exponential backoff
- Stuck orders are automatically refunded after timeout
- Provider failures trigger alternative provider selection

### Manual Intervention
- Admin endpoints for order status override
- Manual refund processing capabilities
- Provider performance monitoring and adjustment

## Monitoring and Observability

### Logging
- Structured logging with correlation IDs
- Transaction-level tracing
- Performance metrics collection

### Event Tracking
- Real-time order status updates
- Provider performance metrics
- System health monitoring

## Security Considerations

### Private Key Management
- Aggregator private key controls all operations
- Hardware security module (HSM) recommended for production
- Key rotation procedures documented

### Smart Contract Security
- All contracts are audited implementations
- Multi-signature controls for critical functions
- Emergency pause mechanisms available

### Data Protection
- Recipient information encrypted at rest
- PII handling compliant with regulations
- Secure communication channels required

## Performance Optimization

### Database Optimization
- Indexed queries for order lookups
- Connection pooling for high throughput
- Read replicas for analytics queries

### Blockchain Optimization
- Batch processing for multiple orders
- Gas price optimization strategies
- RPC endpoint failover mechanisms

## Deployment Considerations

### Infrastructure Requirements
- PostgreSQL database with replication
- Redis for caching and sessions
- Load balancers for API endpoints
- Monitoring and alerting systems

### Scaling Strategies
- Horizontal scaling of API services
- Database sharding by network/region
- Separate indexing services per blockchain
- Service provider horizontal scaling (Alchemy or Thirdweb)

## Key Architectural Points

### Wallet Architecture
The system uses **three distinct wallet types** (EVM-only):

1. **Receive Addresses** (Temporary, Many)
   - Created via Alchemy (recommended) or Thirdweb Engine for each order
   - ERC-4337 smart accounts with deterministic CREATE2 deployment
   - **Alchemy**: Self-managed keys, deterministic address generation
   - **Thirdweb**: Keys managed by Engine vault
   - Purpose: Receive user deposits

2. **Aggregator Smart Account** (Permanent, One)
   - Your operational identity: `AGGREGATOR_SMART_ACCOUNT`
   - Controlled by `AGGREGATOR_PRIVATE_KEY`
   - Executes all business logic transactions
   - Purpose: Create, settle, and refund orders

3. **Gateway Contract** (Escrow)
   - Pre-deployed on each EVM network
   - Holds funds during order processing
   - Releases funds on settlement or refund

### Transaction Flow
```
User Deposit → Receive Address (Alchemy/Thirdweb-managed)
             ↓
Aggregator detects deposit (Webhook: Alchemy Notify or Thirdweb Insight)
             ↓
Aggregator creates order → Gateway Contract (via Service Provider)
             ↓
Funds: Receive Address → Gateway Contract
             ↓
Provider fulfills order
             ↓
Aggregator settles → Gateway releases funds to Provider
```

### Blockchain Service Provider Role
**Alchemy (Recommended)**:
- Deterministic smart account creation via CREATE2
- Direct transaction signing with self-managed keys
- Alchemy Notify for webhook events
- Gas Manager for sponsored transactions (optional)
- Cost-effective ($0-49/month)

**Thirdweb Engine (Legacy)**:
- Vault-managed wallet creation and signing
- Thirdweb Insight for webhook events
- Automatic gas management
- Higher cost ($99-999/month)

### Security Model
- **Separation of Concerns**: Receive addresses isolated from operational account
- **Key Management**: 
  - Alchemy: Self-managed in environment variables
  - Thirdweb: Stored in Engine vault
- **Transaction Control**: Only `AGGREGATOR_SMART_ACCOUNT` can execute order operations
- **Escrow Protection**: User funds held in Gateway contract until settlement/refund
- **EVM-Only**: Focused security model for EVM chains

---

This documentation provides a complete technical overview of the order lifecycle in the NEDA aggregator system. Each phase involves multiple components working together to provide a seamless EVM payment processing experience while maintaining security, reliability, and scalability through modern blockchain service providers (Alchemy recommended).
