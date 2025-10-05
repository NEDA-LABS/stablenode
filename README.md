# NEDAPay "Stablenode" Aggregator Order Lifecycle Documentation ---oct 5, 2025

**for development setup check readme.md**

## Overview

This document provides a comprehensive technical overview of the order lifecycle in the NEDA aggregator system, from initial order creation through final settlement or refund. The system implements a sophisticated multi-chain payment processing pipeline with ERC-4337 Account Abstraction integration.

## Architecture Components

### Core Services
- **Order Service**: Handles order creation and smart contract interactions (`services/order/`)
- **Indexer Service**: Monitors blockchain events and updates database state (`services/indexer/`)
- **Engine Service**: Manages RPC connections and blockchain interactions (`services/engine.go`)
- **Priority Queue Service**: Manages order processing queues (`services/priority_queue.go`)

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
// Generates unique receive addresses for orders
func GenerateReceiveAddress(order *ent.PaymentOrder) (string, error) {
    // Uses HD wallet derivation
    // Creates time-limited receive address
    // Links to payment order
}
```

**Database Operations**:
- Creates `ReceiveAddress` entity
- Sets expiration time based on `RECEIVE_ADDRESS_VALIDITY`
- Links to payment order

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

#### 3.2 Account Abstraction Integration
**File**: `utils/userop.go`

```go
// Creates and submits UserOperations
func CreateUserOperation(order *ent.PaymentOrder) (*userop.UserOperation, error) {
    // Builds UserOperation structure
    // Signs with aggregator private key
    // Submits to bundler service (Biconomy/Alchemy)
}
```

**Process Flow**:
1. Creates UserOperation with order data
2. Signs with `AGGREGATOR_PRIVATE_KEY`
3. Submits to AA bundler service
4. Bundler includes in batch transaction
5. EntryPoint contract validates and executes

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
# Smart Contract Addresses
ENTRY_POINT_CONTRACT_ADDRESS=0x5FF137D4b0FDCD49DcA30c7CF57E578a026d2789
AGGREGATOR_SMART_ACCOUNT=0x03Ff9504c7067980c1637BF9400E7b7e3655782c

# Order Configuration
ORDER_FULFILLMENT_VALIDITY=1    # minutes
ORDER_REFUND_TIMEOUT=5          # minutes
RECEIVE_ADDRESS_VALIDITY=30     # minutes

# Account Abstraction
AGGREGATOR_PRIVATE_KEY="..."    # Signs UserOperations
AGGREGATOR_PUBLIC_KEY="..."     # Verification key
```

### Network Configuration
Each supported blockchain network requires:
- RPC endpoint configuration
- Gateway contract address
- Supported token contracts
- Gas price and fee settings

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

This documentation provides a complete technical overview of the order lifecycle in the NEDA aggregator system. Each phase involves multiple components working together to provide a seamless payment processing experience while maintaining security, reliability, and scalability.
