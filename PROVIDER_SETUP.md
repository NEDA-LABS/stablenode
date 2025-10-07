# Provider Setup Guide

This guide explains how to configure a provider to work with the aggregator system.

## Prerequisites

- Aggregator running and connected to database
- Redis running
- Provider node (e.g., riverpod) container ready to deploy

## Database Configuration Requirements

### 1. Provider Profile

The provider must exist in the `provider_profiles` table with the following settings:

```sql
-- Check provider profile
SELECT id, trading_name, visibility_mode, is_active, is_kyb_verified, host_identifier
FROM provider_profiles 
WHERE id = 'YOUR_PROVIDER_ID';
```

**Required values:**
- `visibility_mode` = `'public'` (NOT `'private'`)
- `is_active` = `true`
- `is_kyb_verified` = `true`
- `host_identifier` = Provider node URL (e.g., `'http://host.docker.internal:8105'`)

### 2. User KYB Status

The user associated with the provider MUST have KYB approved:

```sql
-- Check user KYB status
SELECT u.id, u.email, u.kyb_verification_status
FROM users u
JOIN provider_profiles pp ON u.id = pp.user_provider_profile
WHERE pp.id = 'YOUR_PROVIDER_ID';
```

**Required value:**
- `kyb_verification_status` = `'approved'` (NOT `'not_started'` or `'pending'`)

**To approve:**
```sql
UPDATE users 
SET kyb_verification_status = 'approved' 
WHERE id = 'USER_ID';
```

### 3. Provider Currencies

The provider must have at least one currency configured with available balance:

```sql
-- Check provider currencies
SELECT pc.id, fc.code, pc.is_available, pc.available_balance, pc.total_balance
FROM provider_currencies pc
JOIN fiat_currencies fc ON pc.fiat_currency_provider_currencies = fc.id
WHERE pc.provider_profile_provider_currencies = 'YOUR_PROVIDER_ID';
```

**Required values:**
- `is_available` = `true`
- `available_balance` > 0 (should be greater than minimum order amount)

**To add a currency (e.g., NGN):**
```sql
INSERT INTO provider_currencies (
    id, 
    available_balance, 
    total_balance, 
    reserved_balance, 
    is_available, 
    updated_at, 
    fiat_currency_provider_currencies, 
    provider_profile_provider_currencies
) VALUES (
    gen_random_uuid(), 
    10000,  -- Available balance
    10000,  -- Total balance
    0,      -- Reserved balance
    true,   -- Is available
    NOW(), 
    'CURRENCY_UUID',  -- Get from fiat_currencies table
    'YOUR_PROVIDER_ID'
);
```

### 4. Provision Buckets

Provision buckets define amount ranges for order matching. The provider must be linked to buckets:

```sql
-- Check provision buckets
SELECT pb.id, pb.min_amount, pb.max_amount, fc.code
FROM provision_buckets pb
JOIN fiat_currencies fc ON pb.fiat_currency_provision_buckets = fc.id
WHERE fc.code = 'NGN';
```

**To create buckets (example for NGN):**
```sql
INSERT INTO provision_buckets (id, min_amount, max_amount, created_at, fiat_currency_provision_buckets) VALUES
(42949672960, 0.5, 1000, NOW(), 'CURRENCY_UUID'),
(42949672961, 1001, 5000, NOW(), 'CURRENCY_UUID'),
(42949672962, 5001, 50000, NOW(), 'CURRENCY_UUID')
ON CONFLICT (id) DO NOTHING;
```

**Link provider to buckets:**
```sql
INSERT INTO provision_bucket_provider_profiles (provision_bucket_id, provider_profile_id) VALUES
(42949672960, 'YOUR_PROVIDER_ID'),
(42949672961, 'YOUR_PROVIDER_ID'),
(42949672962, 'YOUR_PROVIDER_ID')
ON CONFLICT DO NOTHING;
```

### 5. Provider Order Tokens

The provider must support at least one token for the currency:

```sql
-- Check provider order tokens
SELECT pot.id, t.symbol, pot.network, pot.min_order_amount, pot.max_order_amount
FROM provider_order_tokens pot
JOIN tokens t ON pot.token_provider_order_tokens = t.id
WHERE pot.provider_profile_order_tokens = 'YOUR_PROVIDER_ID';
```

**To add a token (e.g., DAI on base-sepolia):**
```sql
-- First, get the token ID
SELECT id, symbol, network_tokens FROM tokens WHERE symbol = 'DAI';

-- Then insert the provider order token
INSERT INTO provider_order_tokens (
    id, 
    created_at, 
    updated_at, 
    fixed_conversion_rate, 
    floating_conversion_rate, 
    conversion_rate_type, 
    max_order_amount, 
    min_order_amount, 
    provider_profile_order_tokens, 
    address, 
    network, 
    fiat_currency_provider_order_tokens, 
    token_provider_order_tokens
) VALUES (
    32,  -- Unique ID
    NOW(), 
    NOW(), 
    0,              -- Fixed rate (0 if using floating)
    0,              -- Floating rate percentage
    'floating',     -- 'fixed' or 'floating'
    900,            -- Max order amount
    0.5,            -- Min order amount
    'YOUR_PROVIDER_ID', 
    '0x409689E3008d43a9eb439e7B275749D4a71D8E2D',  -- Provider wallet address
    'base-sepolia', -- Network identifier
    'CURRENCY_UUID',  -- NGN currency UUID
    55834574852     -- Token ID from tokens table
) ON CONFLICT (id) DO UPDATE SET
    min_order_amount = 0.5,
    max_order_amount = 900;
```

## Provider Node Docker Configuration

### Network Connectivity

For the provider node to communicate with the aggregator on Linux, use one of these approaches:

#### Option 1: Using `--add-host` (Recommended)

```bash
sudo docker run -d \
  --add-host=host.docker.internal:host-gateway \
  -p 8105:8000 \
  -e AGGREGATOR_BASE_URL=http://host.docker.internal:8000 \
  -e AGGREGATOR_CLIENT_ID=YOUR_CLIENT_ID \
  -e AGGREGATOR_SECRET_KEY=YOUR_SECRET_KEY \
  -e ALLOWED_HOSTS=0.0.0.0 \
  -e CURRENCIES=NGN \
  -e DEBUG=False \
  -e SECRET=YOUR_SECRET \
  -e SERVER_HOST=0.0.0.0 \
  -e SERVER_PORT=8000 \
  docker.io/safaria/riverpod:pilot-main-amd64
```

#### Option 2: Using Docker Bridge IP

```bash
# Find Docker bridge IP
DOCKER_BRIDGE_IP=$(ip addr show docker0 | grep 'inet ' | awk '{print $2}' | cut -d/ -f1)

# Update database host_identifier
UPDATE provider_profiles 
SET host_identifier = 'http://172.17.0.1:8105' 
WHERE id = 'YOUR_PROVIDER_ID';

# Run provider
sudo docker run -d \
  -p 8105:8000 \
  -e AGGREGATOR_BASE_URL=http://172.17.0.1:8000 \
  -e AGGREGATOR_CLIENT_ID=YOUR_CLIENT_ID \
  -e AGGREGATOR_SECRET_KEY=YOUR_SECRET_KEY \
  # ... other env vars
  docker.io/safaria/riverpod:pilot-main-amd64
```

#### Option 3: Using Host Network (Simplest)

```bash
sudo docker run -d \
  --network host \
  -e AGGREGATOR_BASE_URL=http://localhost:8000 \
  -e AGGREGATOR_CLIENT_ID=YOUR_CLIENT_ID \
  -e AGGREGATOR_SECRET_KEY=YOUR_SECRET_KEY \
  # ... other env vars
  docker.io/safaria/riverpod:pilot-main-amd64

# Update database
UPDATE provider_profiles 
SET host_identifier = 'http://localhost:8000' 
WHERE id = 'YOUR_PROVIDER_ID';
```

### Important Notes

- **AGGREGATOR_BASE_URL**: Should be just the base URL (e.g., `http://host.docker.internal:8000`), NOT including `/v1/provider`
- **Port Mapping**: `-p 8105:8000` maps host port 8105 to container port 8000
- **Network Direction**:
  - Provider → Aggregator: Uses `AGGREGATOR_BASE_URL`
  - Aggregator → Provider: Uses `host_identifier` from database

## Verification Steps

### 1. Check Redis Buckets

After restarting the aggregator, verify that Redis buckets are populated:

```bash
sudo docker exec nedapay_redis redis-cli KEYS "bucket_NGN_*"
```

Expected output:
```
1) "bucket_NGN_0.5_1000"
2) "bucket_NGN_1001_5000"
3) "bucket_NGN_5001_50000"
```

### 2. Check Provider in Queue

```bash
sudo docker exec nedapay_redis redis-cli LRANGE "bucket_NGN_0.5_1000" 0 -1
```

Expected output should include your provider:
```
AtGaDPqT:DAI:1472.31:0.5:900
```

### 3. Test Rate Endpoint

```bash
curl "http://localhost:8000/v1/rates/DAI/0.5/NGN?network=base-sepolia"
```

Expected response:
```json
{
  "status": "success",
  "message": "Rate fetched successfully",
  "data": "1472.31"
}
```

### 4. Test Provider Health

```bash
curl "http://localhost:8105/health"
```

Expected response:
```json
{
  "data": {
    "currencies": ["NGN"]
  },
  "message": "Node is live",
  "status": "success"
}
```

## Troubleshooting

### Issue: "no provider available" error

**Possible causes:**
1. Provider `visibility_mode` is `'private'` instead of `'public'`
2. User `kyb_verification_status` is not `'approved'`
3. Provider has no currencies configured
4. Provider currency `is_available` is `false`
5. Provider currency `available_balance` is 0 or too low
6. Provider has no order tokens configured
7. Provider is not linked to provision buckets
8. Redis buckets are empty (not built yet)

**Solution:** Check all requirements above and restart aggregator:
```bash
sudo docker restart nedapay_aggregator
```

### Issue: Provider node cannot reach aggregator

**Possible causes:**
1. Using `localhost` instead of `host.docker.internal` in Docker environment
2. Missing `--add-host` flag on Linux
3. Network isolation between containers

**Solution:** Use one of the network configuration options above.

### Issue: Aggregator cannot reach provider node

**Possible causes:**
1. `host_identifier` in database points to wrong URL
2. Provider container not running
3. Port mapping incorrect

**Solution:** 
```sql
-- Update host_identifier
UPDATE provider_profiles 
SET host_identifier = 'http://host.docker.internal:8105' 
WHERE id = 'YOUR_PROVIDER_ID';
```

### Issue: Redis buckets are empty

**Possible causes:**
1. Aggregator hasn't rebuilt queues yet (waits 10 minutes)
2. Provider doesn't meet all requirements

**Solution:** Restart aggregator to trigger immediate rebuild:
```bash
sudo docker restart nedapay_aggregator
sleep 15
sudo docker exec nedapay_redis redis-cli KEYS "bucket_*"
```

## Complete Setup Checklist

- [ ] Provider profile exists with `visibility_mode='public'`, `is_active=true`
- [ ] User KYB status is `'approved'`
- [ ] Provider has at least one currency with `is_available=true` and `available_balance > 0`
- [ ] Provision buckets exist for the currency
- [ ] Provider is linked to provision buckets
- [ ] Provider has at least one order token configured
- [ ] Provider node Docker container is running with correct network configuration
- [ ] Aggregator can reach provider node (test `/info` endpoint)
- [ ] Provider node can reach aggregator (test `/health` endpoint)
- [ ] Redis buckets are populated with provider data
- [ ] Rate endpoint returns successful response

## API Authentication

Provider nodes authenticate with the aggregator using HMAC signatures:

```bash
CLIENT_ID="your-client-id"
SECRET_KEY="your-secret-key"
TIMESTAMP=$(date +%s)

# For GET requests, add timestamp as query parameter
curl "http://localhost:8000/v1/provider/node-info?timestamp=$TIMESTAMP" \
  -H "Authorization: HMAC $CLIENT_ID:$SIGNATURE"
```

The signature must be generated using HMAC-SHA256 with the secret key and payload containing the timestamp.

## Environment Variables Reference

### Aggregator (.env)
```bash
DB_HOST=your-database-host
DB_NAME=postgres
DB_USER=your-db-user
DB_PASSWORD=your-db-password
DB_PORT=5432
REDIS_HOST=redis
REDIS_PORT=6379
SERVER_HOST=0.0.0.0
SERVER_PORT=8000
ENVIRONMENT=local  # Must NOT be 'production' for initial queue build
BUCKET_QUEUE_REBUILD_INTERVAL=10  # Minutes
```

### Provider Node
```bash
AGGREGATOR_BASE_URL=http://host.docker.internal:8000
AGGREGATOR_CLIENT_ID=your-client-id
AGGREGATOR_SECRET_KEY=your-secret-key
CURRENCIES=NGN
SERVER_HOST=0.0.0.0
SERVER_PORT=8000
```

## Queue Rebuild Schedule

The aggregator rebuilds provider queues:
- **On startup** (if ENVIRONMENT != 'production')
- **Every 10 minutes** (configurable via BUCKET_QUEUE_REBUILD_INTERVAL)

To force an immediate rebuild, restart the aggregator.
