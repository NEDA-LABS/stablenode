# Receive Address Pool Architecture

## System Flow Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                    INITIALIZATION PHASE                          │
│                    (One-time setup)                              │
└─────────────────────────────────────────────────────────────────┘

    Admin/Script                 Pool Service              Blockchain
         │                            │                          │
         │  InitializePool(size=10)   │                          │
         ├───────────────────────────>│                          │
         │                            │                          │
         │                            │ For each address:        │
         │                            │  1. Generate address     │
         │                            │  2. Create UserOp        │
         │                            │  3. Get paymaster data   │
         │                            │  4. Send deployment      │
         │                            ├─────────────────────────>│
         │                            │    Deploy SmartAccount   │
         │                            │<─────────────────────────┤
         │                            │  TxHash + Receipt        │
         │                            │                          │
         │                   Save to Database:                   │
         │                   ┌────────────────────┐             │
         │                   │ ReceiveAddress     │             │
         │                   ├────────────────────┤             │
         │                   │ address: 0x123...  │             │
         │                   │ is_deployed: true  │             │
         │                   │ status: pool_ready │             │
         │                   │ chain_id: 84532    │             │
         │                   │ times_used: 0      │             │
         │                   └────────────────────┘             │
         │<───────────────────────────│                          │
         │     Pool Ready (10 addrs)  │                          │


┌─────────────────────────────────────────────────────────────────┐
│                    ORDER CREATION PHASE                          │
│                    (Per order)                                   │
└─────────────────────────────────────────────────────────────────┘

    User/API                 Controller              Pool Service      Database
         │                        │                       │                │
         │  POST /payment-order   │                       │                │
         ├───────────────────────>│                       │                │
         │                        │  GetAvailableAddress()│                │
         │                        ├──────────────────────>│                │
         │                        │                       │                │
         │                        │             Query: WHERE status='pool_ready'
         │                        │                       │    AND is_deployed=true
         │                        │                       ├───────────────>│
         │                        │                       │<───────────────┤
         │                        │                       │  [Addr1, Addr2, ...]
         │                        │                       │                │
         │                        │              Pick Random + Lock        │
         │                        │                       │  FOR UPDATE    │
         │                        │                       ├───────────────>│
         │                        │                       │                │
         │                        │              Update Status:            │
         │                        │                SET status='pool_assigned'
         │                        │                SET assigned_at=NOW()   │
         │                        │                SET times_used += 1     │
         │                        │                       ├───────────────>│
         │                        │<──────────────────────┤                │
         │                        │  ReceiveAddress       │                │
         │                        │                       │                │
         │        Create PaymentOrder with ReceiveAddress │                │
         │                        ├──────────────────────────────────────>│
         │<───────────────────────┤                       │                │
         │  Order + receiveAddress│                       │                │


┌─────────────────────────────────────────────────────────────────┐
│                    PAYMENT DETECTION PHASE                       │
│                    (Indexer/Polling)                             │
└─────────────────────────────────────────────────────────────────┘

    Blockchain              Indexer/Polling          Database          Order Processor
         │                        │                       │                    │
         │  Transfer detected     │                       │                    │
         │  to 0x123... (pooled)  │                       │                    │
         ├───────────────────────>│                       │                    │
         │                        │                       │                    │
         │              Find ReceiveAddress + PaymentOrder│                    │
         │                        ├──────────────────────>│                    │
         │                        │<──────────────────────┤                    │
         │                        │  Order details        │                    │
         │                        │                       │                    │
         │                        │        Update status: pool_processing     │
         │                        ├──────────────────────>│                    │
         │                        │                       │                    │
         │                        │  Process Order        │                    │
         │                        ├───────────────────────────────────────────>│
         │                        │                       │                    │
         │                        │                       │   Execute payment  │
         │                        │                       │   to recipient     │


┌─────────────────────────────────────────────────────────────────┐
│                    ORDER COMPLETION PHASE                        │
│                    (Recycling)                                   │
└─────────────────────────────────────────────────────────────────┘

    Order Processor         Pool Service              Database
         │                        │                       │
         │  Order Completed       │                       │
         │  RecycleAddress(id)    │                       │
         ├───────────────────────>│                       │
         │                        │                       │
         │              Update ReceiveAddress:            │
         │                SET status='pool_ready'         │
         │                SET recycled_at=NOW()           │
         │                        ├──────────────────────>│
         │                        │                       │
         │<───────────────────────┤                       │
         │  Address back in pool  │                       │
         │                        │                       │
         │              Address now available for reuse   │


┌─────────────────────────────────────────────────────────────────┐
│                    POOL MAINTENANCE PHASE                        │
│                    (Background task - every 10 min)              │
└─────────────────────────────────────────────────────────────────┘

    Scheduler               Pool Service              Database       Blockchain
         │                        │                       │                │
         │  MaintainPoolSize()    │                       │                │
         ├───────────────────────>│                       │                │
         │                        │                       │                │
         │              Count available addresses         │                │
         │                        ├──────────────────────>│                │
         │                        │<──────────────────────┤                │
         │                        │  count = 3            │                │
         │                        │                       │                │
         │              if count < 5: Create more         │                │
         │                        │                       │                │
         │                        │  CreateAndDeployAddress()              │
         │                        ├───────────────────────────────────────>│
         │                        │<───────────────────────────────────────┤
         │                        │                       │                │
         │                        │  Save to pool         │                │
         │                        ├──────────────────────>│                │
         │<───────────────────────┤                       │                │
         │  Pool replenished      │                       │                │
```

## Database State Transitions

```
┌─────────────────────────────────────────────────────────────────┐
│                  ReceiveAddress Status Flow                      │
└─────────────────────────────────────────────────────────────────┘

    [NEW]
      │
      │ Deploy & Add to Database
      ↓
  pool_ready ←──────────────┐
      │                     │
      │ Assigned to Order   │ Recycled
      ↓                     │
  pool_assigned             │
      │                     │
      │ Payment Detected    │
      ↓                     │
  pool_processing           │
      │                     │
      │ Order Completed     │
      ↓                     │
  pool_completed ───────────┘
      
```

## Address Lifecycle

```
┌────────────────────────────────────────────────────────────┐
│  Single Address Lifecycle (Reused 100+ times)             │
└────────────────────────────────────────────────────────────┘

Deployment:
  ┌─────────────────┐
  │ Create Account  │  ← One-time cost, Alchemy sponsored
  │ Deploy On-Chain │  ← Nonce = 0, with initCode
  └─────────────────┘
          │
          ↓
  ┌─────────────────┐
  │ Add to Pool     │  ← status = pool_ready
  │ times_used = 0  │
  └─────────────────┘
          │
          ↓
  ┌─────────────────────────────────────────┐
  │         Reuse Cycle (100x)              │
  │                                         │
  │  1. Assign to Order                     │
  │     └─> status = pool_assigned          │
  │                                         │
  │  2. Receive Payment                     │
  │     └─> status = pool_processing        │
  │     └─> Process order                   │
  │     └─> Sweep balance                   │
  │                                         │
  │  3. Complete & Recycle                  │
  │     └─> status = pool_ready             │
  │     └─> times_used++                    │
  │                                         │
  │  Repeat 100x                            │
  └─────────────────────────────────────────┘
          │
          ↓
  ┌─────────────────┐
  │ Retire Address  │  ← times_used >= 100
  │ Remove from Pool│
  └─────────────────┘
```

## Pool Size Management

```
┌────────────────────────────────────────────────────────────┐
│              Pool Size Over Time                            │
└────────────────────────────────────────────────────────────┘

Initial State:
  Pool: [A1, A2, A3, A4, A5]  (5 ready)

After 2 Orders Created:
  Pool: [A3, A4, A5]          (3 ready)
  In Use: [A1, A2]            (2 assigned)

After 1 Order Completed:
  Pool: [A3, A4, A5, A1]      (4 ready)  ← A1 recycled
  In Use: [A2]                (1 assigned)

Background Task Detects Low Pool (<5):
  Pool: [A3, A4, A5, A1, A6, A7, A8]  (7 ready)  ← Added A6, A7, A8
  In Use: [A2]                         (1 assigned)

System maintains: 5-10 addresses ready at all times
```

## Concurrency Safety

```
┌────────────────────────────────────────────────────────────┐
│         Concurrent Order Creation (Race Condition)          │
└────────────────────────────────────────────────────────────┘

WITHOUT Database Locking (BAD):
  Order1                      Order2
    │                           │
    ├─ Query pool_ready         ├─ Query pool_ready
    │  [A1, A2, A3]             │  [A1, A2, A3]
    │                           │
    ├─ Pick random: A1          ├─ Pick random: A1  ⚠️ CONFLICT
    │                           │
    ├─ Update A1 = assigned     ├─ Update A1 = assigned
    │                           │
    └─ Use A1                   └─ Use A1  ⚠️ BOTH ORDERS USE SAME ADDRESS


WITH Database Locking (GOOD):
  Order1                      Order2
    │                           │
    ├─ BEGIN TRANSACTION        ├─ BEGIN TRANSACTION
    │                           │
    ├─ Query FOR UPDATE         ├─ Query FOR UPDATE
    │  Lock: [A1, A2, A3]       │  ⏳ WAIT for lock...
    │                           │
    ├─ Pick: A1                 │
    │                           │
    ├─ Update A1 = assigned     │
    │                           │
    ├─ COMMIT                   │
    │  Release lock              │  Lock acquired: [A2, A3]
    │                           │
    └─ Use A1                   ├─ Pick: A2  ✅ Different address
                                │
                                ├─ Update A2 = assigned
                                │
                                ├─ COMMIT
                                │
                                └─ Use A2  ✅ No conflict
```

## Resource Efficiency

```
┌────────────────────────────────────────────────────────────┐
│         Resource Usage: Current vs Pool Approach            │
└────────────────────────────────────────────────────────────┘

Current Approach (Create per order):
  Orders:     1000 orders
  Addresses:  1000 unique addresses
  Deployments: 1000 deployments
  Gas Cost:   1000 × deployment_cost
  Errors:     High (AA20, AA23)

Pool Approach (Reuse):
  Orders:     1000 orders
  Addresses:  10 addresses (reused 100x each)
  Deployments: 10 deployments (one-time)
  Gas Cost:   10 × deployment_cost
  Errors:     None (pre-deployed)

Savings:     99% reduction in deployments
             99% reduction in gas costs
             100% reduction in deployment errors
```

## System Benefits

```
┌────────────────────────────────────────────────────────────┐
│                 Key Benefits Summary                        │
└────────────────────────────────────────────────────────────┘

1. Performance
   ┌─────────────────────────────────────────┐
   │ Order Creation: 10s → 1s                │
   │ No deployment wait time                  │
   │ Instant address assignment               │
   └─────────────────────────────────────────┘

2. Reliability
   ┌─────────────────────────────────────────┐
   │ No AA20 errors (account exists)         │
   │ No AA23 errors (already deployed)       │
   │ No initCode complexity                   │
   └─────────────────────────────────────────┘

3. Cost Efficiency
   ┌─────────────────────────────────────────┐
   │ 99% reduction in deployments            │
   │ One-time deployment cost per address    │
   │ Each address serves 100+ orders         │
   └─────────────────────────────────────────┘

4. Scalability
   ┌─────────────────────────────────────────┐
   │ Auto-replenishment of pool              │
   │ Handles concurrent orders safely        │
   │ Graceful degradation if pool exhausted  │
   └─────────────────────────────────────────┘
```
