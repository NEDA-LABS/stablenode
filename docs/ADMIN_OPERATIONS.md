# Admin Operations Guide - NEDA Stablenode Aggregator

## Overview
This document outlines administrative operations required to manage the NEDA Stablenode Aggregator system. These operations are typically performed by system administrators and should be exposed through an admin portal.

---

## Table of Contents
1. [Provision Bucket Management](#provision-bucket-management)
2. [Fiat Currency Management](#fiat-currency-management)
3. [Network & Token Management](#network--token-management)
4. [Provider Management](#provider-management)
5. [Institution Management](#institution-management)
6. [KYB Verification](#kyb-verification)
7. [System Monitoring](#system-monitoring)

---

## Provision Bucket Management

### **What Are Provision Buckets?**
Provision buckets are amount ranges that organize orders and providers by:
- **Amount ranges**: Define min/max order amounts (e.g., 0.5-100, 101-500, 501-1000)
- **Fiat currency**: Each bucket is tied to a specific currency (NGN, GHS, KES, etc.)
- **Provider assignment**: Multiple providers can participate in the same bucket

### **Why They Matter**
- Enable efficient order routing and provider matching
- Allow fair distribution of orders through circular queue system
- Support order splitting across multiple providers
- Facilitate liquidity management by amount tiers

### **Current Bucket Structure (Example)**
```
NGN (Nigerian Naira):
├── Bucket 1: 0.5 - 100 NGN
├── Bucket 2: 101 - 500 NGN
├── Bucket 3: 501 - 1,000 NGN
├── Bucket 4: 1,001 - 5,000 NGN
└── Bucket 5: 5,001 - 50,000 NGN

GHS (Ghanaian Cedi):
├── Bucket 1: 0.5 - 100 GHS
├── Bucket 2: 101 - 500 GHS
└── Bucket 3: 501 - 5,000 GHS
```

### **Admin Portal Requirements**

#### **1. View Provision Buckets**
Display all buckets with:
- Currency code and name
- Min/Max amount range
- Number of assigned providers
- Total available liquidity in bucket
- Creation date
- Status (active/inactive)

**SQL Query:**
```sql
SELECT 
    pb.id,
    pb.min_amount,
    pb.max_amount,
    pb.created_at,
    fc.code as currency_code,
    fc.name as currency_name,
    fc.symbol as currency_symbol,
    COUNT(DISTINCT pbpp.provider_profile_id) as provider_count
FROM provision_buckets pb
JOIN fiat_currencies fc ON pb.fiat_currency_provision_buckets = fc.id
LEFT JOIN provision_bucket_provider_profiles pbpp ON pb.id = pbpp.provision_bucket_id
GROUP BY pb.id, fc.code, fc.name, fc.symbol
ORDER BY fc.code, pb.min_amount;
```

#### **2. Create New Provision Bucket**
**Form Fields:**
- Currency (dropdown - from enabled fiat currencies)
- Min Amount (decimal)
- Max Amount (decimal)
- Auto-assign providers (checkbox - assign all active providers)

**Validation Rules:**
- Min amount must be >= 0
- Max amount must be > min amount
- Ranges should not overlap with existing buckets for the same currency
- Consider leaving small gaps or overlaps based on business logic

**SQL Example:**
```sql
-- Create new bucket
INSERT INTO provision_buckets (min_amount, max_amount, created_at, fiat_currency_provision_buckets)
VALUES (10001, 50000, NOW(), 'currency-uuid-here')
RETURNING id;

-- Optionally auto-assign all active providers
INSERT INTO provision_bucket_provider_profiles (provision_bucket_id, provider_profile_id)
SELECT 
    'new-bucket-id',
    pp.id
FROM provider_profiles pp
WHERE pp.is_active = true 
  AND pp.is_kyb_verified = true
  AND EXISTS (
    SELECT 1 FROM provider_currencies pc
    WHERE pc.provider_profile_provider_currencies = pp.id
      AND pc.fiat_currency_provider_currencies = 'currency-uuid-here'
      AND pc.available_balance >= 10001  -- Min amount of new bucket
  );
```

#### **3. Edit Provision Bucket**
**Editable Fields:**
- Min Amount (with validation)
- Max Amount (with validation)

**Warning:** Changing bucket ranges may affect in-flight orders. Consider:
- Checking for pending orders in this bucket
- Notifying affected providers
- Updating Redis queue keys

#### **4. Delete Provision Bucket**
**Pre-deletion Checks:**
- No pending/processing orders assigned to this bucket
- All providers have been unassigned
- Redis queue has been cleared

**SQL Example:**
```sql
-- Check for active orders
SELECT COUNT(*) FROM lock_payment_orders 
WHERE provision_bucket_lock_payment_orders = 'bucket-id'
  AND status IN ('pending', 'processing');

-- If count = 0, proceed with deletion
DELETE FROM provision_bucket_provider_profiles WHERE provision_bucket_id = 'bucket-id';
DELETE FROM provision_buckets WHERE id = 'bucket-id';
```

#### **5. Assign/Unassign Providers to Bucket**
**UI Component:** Multi-select or drag-and-drop interface

**Display:**
- Available providers (not in bucket)
- Assigned providers (currently in bucket)
- Provider details: name, available balance, supported tokens

**SQL Operations:**
```sql
-- Assign provider to bucket
INSERT INTO provision_bucket_provider_profiles (provision_bucket_id, provider_profile_id)
VALUES ('bucket-id', 'provider-id')
ON CONFLICT DO NOTHING;

-- Unassign provider from bucket
DELETE FROM provision_bucket_provider_profiles 
WHERE provision_bucket_id = 'bucket-id' 
  AND provider_profile_id = 'provider-id';

-- Get providers for bucket
SELECT 
    pp.id,
    pp.trading_name,
    pp.is_active,
    pp.is_available,
    pc.available_balance,
    pc.total_balance,
    fc.code as currency_code
FROM provider_profiles pp
JOIN provision_bucket_provider_profiles pbpp ON pp.id = pbpp.provider_profile_id
JOIN provider_currencies pc ON pp.id = pc.provider_profile_provider_currencies
JOIN fiat_currencies fc ON pc.fiat_currency_provider_currencies = fc.id
WHERE pbpp.provision_bucket_id = 'bucket-id';
```

#### **6. View Bucket Analytics**
Display metrics:
- Total orders processed through bucket (last 24h, 7d, 30d)
- Average order amount
- Success rate
- Provider utilization (orders per provider)
- Average fulfillment time

---

## Fiat Currency Management

### **Admin Portal Requirements**

#### **1. View Fiat Currencies**
Display all currencies with:
- Code (ISO 4217)
- Name and short name
- Symbol
- Current market rate
- Decimals
- Enabled status
- Number of provision buckets
- Number of supporting providers
- Last rate update timestamp

#### **2. Add New Fiat Currency**
**Form Fields:**
- Code (3-letter ISO code, e.g., KES, UGX)
- Short Name (e.g., "Shilling")
- Full Name (e.g., "Kenyan Shilling")
- Symbol (e.g., "KSh")
- Decimals (default: 2)
- Market Rate (current exchange rate to USD)
- Enabled (checkbox)

**SQL Example:**
```sql
INSERT INTO fiat_currencies (
    id, created_at, updated_at, code, short_name, 
    name, symbol, decimals, market_rate, is_enabled
)
VALUES (
    gen_random_uuid(),
    NOW(),
    NOW(),
    'KES',
    'Shilling',
    'Kenyan Shilling',
    'KSh',
    2,
    151.45,
    true
);
```

**Post-Creation Tasks:**
- Create default provision buckets for the currency
- Add institutions (banks/mobile money) for the currency
- Notify providers to add support for the new currency

#### **3. Update Market Rate**
**Options:**
- Manual update (form input)
- Automatic update (integrate with exchange rate API)
- Scheduled updates (cron job)

**Important:** Rate updates affect:
- Provider rate calculations (floating rates)
- Order matching algorithm
- Priority queue generation

**SQL Example:**
```sql
UPDATE fiat_currencies 
SET market_rate = 1658.1, 
    updated_at = NOW()
WHERE code = 'NGN';
```

#### **4. Enable/Disable Currency**
**Impact:**
- Disabled currencies won't appear in order creation
- Existing orders continue to process
- Providers can't add new token configurations

**SQL Example:**
```sql
UPDATE fiat_currencies 
SET is_enabled = false, 
    updated_at = NOW()
WHERE code = 'KES';
```

---

## Network & Token Management

### **Admin Portal Requirements**

#### **1. View Networks**
Display all blockchain networks:
- Chain ID (decimal and hex)
- Identifier (e.g., "base-sepolia", "ethereum")
- RPC endpoint
- Gateway contract address
- Is testnet
- Network fee
- Block time
- Bundler/Paymaster URLs (for account abstraction)
- Number of enabled tokens

#### **2. Add New Network**
**Form Fields:**
- Chain ID (integer)
- Identifier (unique string, lowercase-hyphenated)
- RPC Endpoint (WebSocket URL preferred)
- Gateway Contract Address
- Is Testnet (checkbox)
- Network Fee (decimal)
- Block Time (seconds)
- Bundler URL (optional)
- Paymaster URL (optional)

**SQL Example:**
```sql
INSERT INTO networks (
    created_at, updated_at, chain_id, identifier, 
    rpc_endpoint, gateway_contract_address, is_testnet, 
    fee, block_time, bundler_url, paymaster_url
)
VALUES (
    NOW(),
    NOW(),
    84532,
    'base-sepolia',
    'wss://base-sepolia.infura.io/ws/v3/YOUR_KEY',
    '0x847dfdAa218F9137229CF8424378871A1DA8f625',
    true,
    0.03,
    2.0,
    'https://bundler.biconomy.io/api/v2/84532/KEY',
    'https://paymaster.biconomy.io/api/v1/84532/KEY'
);
```

#### **3. View Tokens**
Display all tokens:
- Symbol
- Contract address
- Network name
- Decimals
- Enabled status
- Base currency
- Number of payment orders
- Number of providers supporting token

#### **4. Add New Token**
**Form Fields:**
- Symbol (e.g., "USDC", "DAI")
- Contract Address (blockchain address)
- Network (dropdown)
- Decimals (integer, typically 6 or 18)
- Base Currency (default: "USD")
- Enabled (checkbox)

**SQL Example:**
```sql
INSERT INTO tokens (
    created_at, updated_at, symbol, contract_address, 
    decimals, is_enabled, network_tokens, base_currency
)
VALUES (
    NOW(),
    NOW(),
    'USDC',
    '0x036CbD53842c5426634e7929541eC2318f3dCF7e',
    6,
    true,
    17179869187,  -- Base Sepolia network ID
    'USD'
);
```

**Post-Creation Tasks:**
- Notify providers to add support for the new token
- Create provider order token configurations
- Update priority queues

#### **5. Enable/Disable Token**
**Impact:**
- Disabled tokens won't appear in order creation
- Existing orders continue to process
- Priority queues are updated

---

## Provider Management

### **Admin Portal Requirements**

#### **1. View Providers**
Display all providers with:
- Provider ID
- Trading name
- User email
- Is active
- Is available (real-time)
- Is KYB verified
- Visibility mode (public/private)
- Provision mode (auto/manual)
- Number of assigned buckets
- Total balance across all currencies
- Success rate
- Average fulfillment time

**Advanced Filters:**
- Status (active/inactive)
- KYB status
- Visibility mode
- Currency support
- Token support

#### **2. Provider Details View**
**Tabs:**

**Tab 1: Basic Information**
- Trading name
- Business name
- Host identifier
- Contact details (mobile, address)
- Date of birth (for individuals)
- Provision mode
- Visibility mode

**Tab 2: KYB Documents**
- Identity document type
- Identity document URL (view/download)
- Business document URL (view/download)
- Certificate of incorporation
- Articles of incorporation
- Business license
- Proof of business address
- AML/KYC policies
- Beneficial owners list

**Tab 3: Currency & Balance**
Display for each supported currency:
- Available balance
- Reserved balance
- Total balance
- Is available flag
- Last updated timestamp

**Actions:**
- Adjust balance (add/subtract)
- Enable/disable currency
- View transaction history

**Tab 4: Token Configuration**
Display provider order tokens:
- Token symbol
- Network
- Fiat currency
- Conversion rate type (fixed/floating)
- Fixed rate / Floating rate
- Rate slippage tolerance
- Min/Max order amount
- Provider address

**Actions:**
- Add new token configuration
- Edit existing configuration
- Remove token configuration

**Tab 5: Provision Buckets**
- List of assigned buckets
- Assign/unassign buckets

**Tab 6: Order History**
- Recent orders assigned to provider
- Success/failure rates
- Average fulfillment time
- Cancellation reasons

#### **3. Approve/Reject KYB**
**Workflow:**
1. Review submitted documents
2. Verify beneficial owners
3. Check compliance requirements
4. Approve or reject with comments

**SQL Example:**
```sql
-- Approve KYB
UPDATE users 
SET kyb_verification_status = 'approved',
    updated_at = NOW()
WHERE id = 'user-id';

UPDATE provider_profiles
SET is_kyb_verified = true,
    updated_at = NOW()
WHERE user_provider_profile = 'user-id';

-- Reject KYB
UPDATE users 
SET kyb_verification_status = 'rejected',
    updated_at = NOW()
WHERE id = 'user-id';

UPDATE kyb_profiles
SET kyb_rejection_comment = 'Reason for rejection',
    updated_at = NOW()
WHERE user_kyb_profile = 'user-id';
```

#### **4. Activate/Deactivate Provider**
**Impact:**
- Deactivated providers are removed from priority queues
- No new orders assigned
- Existing orders continue to process

**SQL Example:**
```sql
UPDATE provider_profiles 
SET is_active = false,
    is_available = false,
    updated_at = NOW()
WHERE id = 'provider-id';
```

#### **5. Set Provider Availability**
Real-time control for order assignment:

**SQL Example:**
```sql
UPDATE provider_profiles 
SET is_available = true,
    updated_at = NOW()
WHERE id = 'provider-id';
```

#### **6. Manage Provider Balance**
**Operations:**
- View balance history
- Add balance (top-up)
- Subtract balance (withdrawal)
- View reserved balance (locked for pending orders)

**SQL Example:**
```sql
-- Add balance
UPDATE provider_currencies
SET total_balance = total_balance + 10000,
    available_balance = available_balance + 10000,
    updated_at = NOW()
WHERE provider_profile_provider_currencies = 'provider-id'
  AND fiat_currency_provider_currencies = 'currency-id';

-- View balance breakdown
SELECT 
    fc.code,
    fc.symbol,
    pc.total_balance,
    pc.available_balance,
    pc.reserved_balance,
    pc.is_available,
    pc.updated_at
FROM provider_currencies pc
JOIN fiat_currencies fc ON pc.fiat_currency_provider_currencies = fc.id
WHERE pc.provider_profile_provider_currencies = 'provider-id';
```

---

## Institution Management

### **What Are Institutions?**
Institutions are banks and mobile money providers that facilitate fiat off-ramping. They are linked to specific fiat currencies.

### **Admin Portal Requirements**

#### **1. View Institutions**
Display all institutions:
- Code (unique identifier)
- Name
- Type (bank/mobile_money)
- Currency
- Creation date

#### **2. Add New Institution**
**Form Fields:**
- Code (unique, e.g., "GTBANK", "MPESA")
- Name (e.g., "Guaranty Trust Bank", "M-Pesa")
- Type (dropdown: bank, mobile_money)
- Currency (dropdown)

**SQL Example:**
```sql
INSERT INTO institutions (
    created_at, updated_at, code, name, type, fiat_currency_institutions
)
VALUES (
    NOW(),
    NOW(),
    'ECOBANK',
    'Ecobank Nigeria',
    'bank',
    'ngn-currency-uuid'
);
```

#### **3. Edit/Delete Institution**
**Warning:** Check for:
- Existing payment orders using this institution
- Provider configurations referencing this institution
- Linked addresses using this institution

---

## KYB Verification

### **Verification Workflow**

#### **1. Pending Verifications Dashboard**
Display all pending KYB requests:
- User name and email
- Company name
- Submission date
- Document completeness status
- Priority/urgency

#### **2. KYB Review Interface**
**Document Viewer:**
- Certificate of incorporation
- Articles of incorporation
- Business license
- Proof of business address
- AML policy
- KYC policy

**Beneficial Owners Section:**
For each beneficial owner:
- Full name
- Residential address
- Date of birth
- Ownership percentage
- Government ID type
- Government ID document (view/download)
- Proof of residential address (view/download)

**Actions:**
- Approve (sets `kyb_verification_status = 'approved'`)
- Reject (requires rejection comment)
- Request more information

#### **3. Verification History**
Track all verification actions:
- Timestamp
- Admin user who performed action
- Action type (approved/rejected/requested info)
- Comments

---

## System Monitoring

### **Admin Dashboard Metrics**

#### **1. Order Metrics**
- Total orders (today, 7d, 30d)
- Orders by status (initiated, processing, settled, refunded)
- Average order amount
- Average fulfillment time
- Success rate
- Failed orders (with reasons)

#### **2. Provider Metrics**
- Total active providers
- Providers by currency
- Average provider balance
- Provider utilization rate
- Top performing providers
- Providers with low balance alerts

#### **3. Liquidity Metrics**
- Total liquidity by currency
- Available vs reserved balance
- Liquidity by bucket
- Low liquidity alerts

#### **4. Network Metrics**
- Orders by network
- Orders by token
- Network fees collected
- Protocol fees collected

#### **5. System Health**
- Redis queue status
- Database connection status
- RPC endpoint status
- Webhook delivery success rate
- Failed webhook retry queue

---

## Priority Queue Management

### **Understanding the Queue System**

The system uses Redis-based circular queues for each provision bucket:

**Queue Key Format:**
```
bucket_{currency_code}_{min_amount}_{max_amount}
Example: bucket_NGN_0.5_100
```

**Queue Data Format:**
```
providerID:tokenSymbol:rate:minOrderAmount:maxOrderAmount
Example: AtGaDPqT:USDC:1658.1:0.5:900
```

### **Admin Operations**

#### **1. View Queue Status**
Display for each bucket:
- Queue key
- Number of providers in queue
- Last updated timestamp
- Queue health status

#### **2. Manually Refresh Queue**
Trigger priority queue regeneration:
- Updates provider rates
- Removes inactive providers
- Reorders based on availability

**Note:** This is typically automated via cron job but can be manually triggered.

#### **3. View Queue Contents**
Display providers in queue order:
- Position in queue
- Provider ID
- Token
- Rate
- Min/Max amounts

---

## Best Practices

### **1. Bucket Management**
- Create overlapping buckets for smooth transitions (e.g., 0-100, 90-500)
- Monitor bucket utilization and adjust ranges based on order patterns
- Ensure adequate provider coverage for each bucket
- Consider time-of-day patterns when setting ranges

### **2. Provider Onboarding**
- Complete KYB verification before activation
- Start with limited bucket assignment
- Monitor initial performance closely
- Gradually increase bucket assignments based on performance

### **3. Currency Addition**
- Research local banking infrastructure
- Identify major institutions (banks and mobile money)
- Set competitive market rates
- Create appropriate bucket structure for market size

### **4. Rate Management**
- Update market rates regularly (at least daily)
- Monitor for significant rate deviations
- Alert providers when their rates are stale
- Consider implementing automatic rate updates via API

### **5. Balance Monitoring**
- Set up low balance alerts for providers
- Monitor reserved vs available balance ratios
- Investigate providers with consistently high reserved balances
- Implement automated balance reporting

### **6. Security**
- Audit all admin actions
- Implement role-based access control
- Require multi-factor authentication for sensitive operations
- Regular security reviews of KYB documents

---

## API Endpoints for Admin Portal

### **Recommended Admin API Structure**

```
POST   /admin/provision-buckets              # Create bucket
GET    /admin/provision-buckets              # List buckets
GET    /admin/provision-buckets/:id          # Get bucket details
PUT    /admin/provision-buckets/:id          # Update bucket
DELETE /admin/provision-buckets/:id          # Delete bucket
POST   /admin/provision-buckets/:id/providers # Assign provider
DELETE /admin/provision-buckets/:id/providers/:provider_id # Unassign

POST   /admin/fiat-currencies                # Add currency
GET    /admin/fiat-currencies                # List currencies
PUT    /admin/fiat-currencies/:id            # Update currency
PUT    /admin/fiat-currencies/:id/rate       # Update market rate

POST   /admin/networks                       # Add network
GET    /admin/networks                       # List networks
PUT    /admin/networks/:id                   # Update network

POST   /admin/tokens                         # Add token
GET    /admin/tokens                         # List tokens
PUT    /admin/tokens/:id                     # Update token

GET    /admin/providers                      # List providers
GET    /admin/providers/:id                  # Get provider details
PUT    /admin/providers/:id/activate         # Activate provider
PUT    /admin/providers/:id/deactivate       # Deactivate provider
PUT    /admin/providers/:id/availability     # Set availability
POST   /admin/providers/:id/balance          # Adjust balance

GET    /admin/kyb/pending                    # Pending KYB requests
GET    /admin/kyb/:id                        # KYB details
POST   /admin/kyb/:id/approve                # Approve KYB
POST   /admin/kyb/:id/reject                 # Reject KYB

POST   /admin/institutions                   # Add institution
GET    /admin/institutions                   # List institutions
PUT    /admin/institutions/:id               # Update institution

GET    /admin/dashboard/metrics              # Dashboard metrics
GET    /admin/queues/status                  # Queue status
POST   /admin/queues/refresh                 # Refresh queues
```

---

## Database Backup & Maintenance

### **Critical Tables to Backup**
- `provision_buckets`
- `provision_bucket_provider_profiles`
- `fiat_currencies`
- `networks`
- `tokens`
- `institutions`
- `provider_profiles`
- `provider_currencies`
- `provider_order_tokens`

### **Maintenance Tasks**
- Regular database backups (daily)
- Archive old orders (>90 days)
- Clean up expired verification tokens
- Monitor database size and performance
- Index optimization for frequently queried tables

---

## Troubleshooting

### **Common Issues**

#### **Orders Not Being Assigned**
Check:
1. Provider is active and available
2. Provider has sufficient balance
3. Provider is assigned to correct bucket
4. Provider has token configuration for order token
5. Priority queue is up to date
6. Redis connection is healthy

#### **Provider Not Appearing in Queue**
Check:
1. Provider is KYB verified
2. Provider visibility mode is public (or order has provider_id)
3. Provider has currency configuration
4. Provider balance > bucket minimum
5. Provider rate is within acceptable deviation

#### **Rate Mismatch Issues**
Check:
1. Market rate is up to date
2. Provider rate slippage tolerance
3. Order rate vs provider rate deviation
4. Queue was refreshed recently

---

## Conclusion

This guide provides the foundation for building a comprehensive admin portal for the NEDA Stablenode Aggregator. Implement these features progressively, starting with the most critical operations (bucket management, provider activation, KYB verification) and expanding to advanced features (analytics, monitoring, automated operations) as the system matures.
