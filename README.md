# NEDAPay "Stablenode" Aggregator Order Lifecycle Documentation by team NEDA ---oct 5, 2025 


**for development setup check (`readme.md`)**

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
   ├─→ EVM Chains: Calls Blockchain Service Provider (Alchemy or Thirdweb)
   │   └─→ Creates ERC-4337 smart account: 0xRECEIVE_ADDRESS_123
   │
   ├─→ Tron: Generates address from HD wallet
   │   └─→ Creates Tron address: TReceiveAddress123
   │
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

## Overview

This document provides a comprehensive technical overview of the order lifecycle in the NEDA "Stablenode" aggregator system adapted from PAYCREST PROTOCOL, from initial order creation through final settlement or refund. The system implements a sophisticated multi-chain payment processing pipeline with ERC-4337 Account Abstraction integration and support for multiple blockchain service providers (Alchemy and Thirdweb Engine) for wallet management.

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
// For EVM chains: Creates ERC-4337 smart accounts via Thirdweb Engine
func (s *ReceiveAddressService) CreateSmartAddress(ctx context.Context, label string) (string, error) {
    return s.engineService.CreateServerWallet(ctx, label)
    // Calls Thirdweb Engine API to create a new smart account
    // Engine manages the private keys for these accounts
}

// For Tron network: Generates addresses from wallet
func (s *ReceiveAddressService) CreateTronAddress(ctx context.Context) (string, []byte, error) {
    wallet := tronWallet.GenerateTronWallet(nodeUrl)
    // Generates new Tron address with encrypted private key
}
```

**Database Operations**:
- Creates `ReceiveAddress` entity
- Sets expiration time based on `RECEIVE_ADDRESS_VALIDITY`
- Links to payment order
- Stores encrypted private key (Tron only)

### Phase 2: Crypto Deposit Detection

#### 2.1 Blockchain Monitoring
**Files**: 
- `services/indexer/evm.go` - Ethereum-based chains
- `services/indexer/tron.go` - Tron network

```go
// Monitors blockchain for incoming transfers
func (s *IndexerEVM) IndexReceiveAddress(ctx context.Context, token *ent.Token, address string, fromBlock int64, toBlock int64, txHash string) (*types.EventCounts, error) {
    // Scans for Transfer events to receive address
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
**Files**:
- `services/order/evm.go` - Ethereum chains
- `services/order/tron.go` - Tron network

```go
// Prepares order for blockchain submission
func (s *OrderEVM) CreateOrder(order *ent.PaymentOrder) error {
    // Encrypts recipient details
    // Prepares smart contract call data
    // Submits via Account Abstraction
}
```

**Smart Contract Interaction**:
- Calls `Gateway.createOrder()` function
- Passes encrypted recipient data
- Uses ERC-4337 UserOperation for gas-less execution

#### 3.2 Transaction Execution via Thirdweb Engine
**File**: `services/engine.go`

```go
// Sends transactions via Thirdweb Engine
func (s *EngineService) SendTransactionBatch(ctx context.Context, chainID int64, address string, txPayload []map[string]interface{}) (queueID string, err error) {
    // Calls Thirdweb Engine API
    // Engine signs transaction with AGGREGATOR_PRIVATE_KEY
    // Returns queue ID for tracking
}
```

**Process Flow**:
1. Aggregator prepares transaction payload (createOrder call data)
2. Sends to Thirdweb Engine via `SendTransactionBatch`
3. Engine signs with `AGGREGATOR_PRIVATE_KEY` (stored in Engine vault)
4. Engine submits transaction to blockchain
5. Transaction transfers funds from receive address to Gateway contract
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
│   ├── evm.go           # Ethereum-based order processing
│   └── tron.go          # Tron network order processing
├── indexer/
│   ├── evm.go           # Ethereum event indexing
│   └── tron.go          # Tron event indexing
├── common/
│   ├── order.go         # Shared order logic
│   └── indexer.go       # Shared indexing logic
├── contracts/           # Generated contract bindings
├── engine.go            # RPC client management
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
# THIRDWEB ENGINE - Wallet Management
# ============================================
ENGINE_BASE_URL=https://your-engine-instance.com
ENGINE_ACCESS_TOKEN=your-vault-access-token
THIRDWEB_SECRET_KEY=your-thirdweb-secret-key

# ============================================
# AGGREGATOR ACCOUNT - Operational Wallet
# ============================================
# The main smart account that executes all order operations
AGGREGATOR_SMART_ACCOUNT=0x03Ff9504c7067980c1637BF9400E7b7e3655782c

# Keys controlling the aggregator account (stored in Thirdweb Engine vault)
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

# ============================================
# HD WALLET - Tron & Tests Only
# ============================================
HD_WALLET_MNEMONIC="twelve word mnemonic phrase..."
```

### Thirdweb Engine Setup
The aggregator requires a Thirdweb Engine instance for:
1. **Wallet Creation**: Generates ERC-4337 smart accounts for receive addresses
2. **Transaction Execution**: Signs and submits transactions using stored keys
3. **Event Monitoring**: Webhooks for Transfer, OrderCreated, OrderSettled, OrderRefunded events
4. **Key Management**: Securely stores and manages private keys

**Setup Steps:**
1. Deploy Thirdweb Engine instance (self-hosted or cloud)
2. Configure `ENGINE_BASE_URL` and `ENGINE_ACCESS_TOKEN`
3. Import `AGGREGATOR_PRIVATE_KEY` into Engine vault
4. Set up webhooks for supported networks

### Network Configuration
Each supported blockchain network requires:
- RPC endpoint configuration
- Gateway contract address
- Supported token contracts
- Gas price and fee settings
- Thirdweb Engine webhook configuration

## Gateway Contract Deployment Strategy

### **🏗️ Pre-Deployed Contract Approach**

The Gateway contracts are **already deployed** on each supported network and their addresses are stored in the database. The system uses pre-deployed contracts rather than deploying them during runtime.

#### **Current Deployed Gateway Contracts:**
```sql
-- From scripts/db_data/dump.sql
INSERT INTO "public"."networks" (..., "gateway_contract_address", ...) VALUES
-- Ethereum Sepolia Testnet
('0xCAD53Ff499155Cc2fAA2082A85716322906886c2'),
-- Arbitrum Sepolia Testnet  
('0x87B321fc77A0fDD0ca1fEe7Ab791131157B9841A'),
-- Tron Shasta Testnet
('TYA8urq7nkN2yU7rJqAgwDShCusDZrrsxZ')
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
services/order/tron.go           # Tron Gateway interactions
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
- Thirdweb Engine horizontal scaling for high transaction volume

## Key Architectural Points

### Wallet Architecture
The system uses **three distinct wallet types**:

1. **Receive Addresses** (Temporary, Many)
   - Created via Thirdweb Engine for each order
   - ERC-4337 smart accounts (EVM chains)
   - Generated wallets (Tron network)
   - Keys managed by Thirdweb Engine
   - Purpose: Receive user deposits

2. **Aggregator Smart Account** (Permanent, One)
   - Your operational identity: `AGGREGATOR_SMART_ACCOUNT`
   - Controlled by `AGGREGATOR_PRIVATE_KEY`
   - Executes all business logic transactions
   - Purpose: Create, settle, and refund orders

3. **Gateway Contract** (Escrow)
   - Pre-deployed on each network
   - Holds funds during order processing
   - Releases funds on settlement or refund

### Transaction Flow
```
User Deposit → Receive Address (Engine-managed)
             ↓
Aggregator detects deposit (Webhook)
             ↓
Aggregator creates order → Gateway Contract (via Engine)
             ↓
Funds: Receive Address → Gateway Contract
             ↓
Provider fulfills order
             ↓
Aggregator settles → Gateway releases funds to Provider
```

### Thirdweb Engine Role
- **Central wallet infrastructure provider**
- Manages all wallet creation and transaction signing
- Stores `AGGREGATOR_PRIVATE_KEY` securely in vault
- Provides webhook system for event monitoring
- Handles gas management and transaction retries

### Security Model
- **Separation of Concerns**: Receive addresses isolated from operational account
- **Key Management**: All private keys stored in Thirdweb Engine vault
- **Transaction Control**: Only `AGGREGATOR_SMART_ACCOUNT` can execute order operations
- **Escrow Protection**: User funds held in Gateway contract until settlement/refund

---

This documentation provides a complete technical overview of the order lifecycle in the NEDA aggregator system. Each phase involves multiple components working together to provide a seamless payment processing experience while maintaining security, reliability, and scalability through Thirdweb Engine integration.
