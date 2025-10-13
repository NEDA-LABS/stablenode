# Provision Node Implementation Specification

This document provides a complete specification for building a provision node that integrates with the aggregator system.

## Overview

A provision node is a service that:
1. Receives order assignments from the aggregator
2. Processes fiat disbursements to recipients via payment service providers (PSPs)
3. Reports order status and balance updates back to the aggregator
4. Operates in either manual or automatic mode

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     PROVISION NODE                          │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌──────────────┐    ┌──────────────┐    ┌─────────────┐  │
│  │   REST API   │───▶│   Business   │───▶│  PSP Client │  │
│  │   Server     │    │    Logic     │    │  (Lenco)    │  │
│  └──────────────┘    └──────────────┘    └─────────────┘  │
│         │                    │                    │         │
│         ▼                    ▼                    ▼         │
│  ┌──────────────┐    ┌──────────────┐    ┌─────────────┐  │
│  │ HMAC Auth    │    │   Database   │    │  Webhook    │  │
│  │ Middleware   │    │   (SQLite)   │    │  Handler    │  │
│  └──────────────┘    └──────────────┘    └─────────────┘  │
│                                                             │
└─────────────────────────────────────────────────────────────┘
                           │
                           ▼
                  ┌────────────────┐
                  │  AGGREGATOR    │
                  └────────────────┘
```

## Required Endpoints

### 1. Health Check
**Endpoint:** `GET /health`

**Response:**
```json
{
  "status": "success",
  "message": "Node is live",
  "data": {
    "currencies": ["NGN", "KES"]
  }
}
```

### 2. Node Info
**Endpoint:** `GET /info`

**Response:**
```json
{
  "status": "success",
  "message": "Node info retrieved",
  "data": {
    "serviceInfo": {
      "currencies": ["NGN", "KES"],
      "version": "1.0.0",
      "mode": "auto"
    }
  }
}
```

### 3. Get Orders (Called by Aggregator)
**Endpoint:** `POST /orders`

**Request from Aggregator:**
```json
{
  "orderId": "uuid",
  "amount": 736.155,
  "currency": "NGN",
  "token": "DAI",
  "tokenAmount": 0.5,
  "network": "base-sepolia",
  "recipient": {
    "institution": "ABNGNGLA",
    "accountIdentifier": "0123456789",
    "accountName": "John Doe",
    "memo": "Payment for services"
  },
  "reference": "ORDER-2025-001",
  "expiresAt": "2025-10-06T14:00:00Z"
}
```

**Response:**
```json
{
  "status": "success",
  "message": "Order received",
  "data": {
    "orderId": "uuid",
    "status": "pending"
  }
}
```

### 4. Order Status Webhook (Called by PSP)
**Endpoint:** `POST /webhooks/psp`

**Request from PSP:**
```json
{
  "transactionId": "psp-tx-id",
  "status": "success",
  "reference": "ORDER-2025-001",
  "amount": 736.155,
  "currency": "NGN"
}
```

## Communication with Aggregator

### 1. Accept Order
**Endpoint:** `POST {AGGREGATOR_BASE_URL}/v1/provider/orders/{orderId}/accept`

**Headers:**
```
Authorization: HMAC {CLIENT_ID}:{SIGNATURE}
Content-Type: application/json
```

**Request Body:**
```json
{
  "timestamp": 1728201600
}
```

### 2. Fulfill Order
**Endpoint:** `POST {AGGREGATOR_BASE_URL}/v1/provider/orders/{orderId}/fulfill`

**Headers:**
```
Authorization: HMAC {CLIENT_ID}:{SIGNATURE}
Content-Type: application/json
```

**Request Body:**
```json
{
  "transactionHash": "psp-transaction-id",
  "timestamp": 1728201600
}
```

### 3. Decline Order
**Endpoint:** `POST {AGGREGATOR_BASE_URL}/v1/provider/orders/{orderId}/decline`

**Headers:**
```
Authorization: HMAC {CLIENT_ID}:{SIGNATURE}
Content-Type: application/json
```

**Request Body:**
```json
{
  "reason": "Insufficient balance",
  "timestamp": 1728201600
}
```

### 4. Cancel Order
**Endpoint:** `POST {AGGREGATOR_BASE_URL}/v1/provider/orders/{orderId}/cancel`

**Headers:**
```
Authorization: HMAC {CLIENT_ID}:{SIGNATURE}
Content-Type: application/json
```

**Request Body:**
```json
{
  "reason": "PSP timeout",
  "timestamp": 1728201600
}
```

### 5. Update Balance
**Endpoint:** `POST {AGGREGATOR_BASE_URL}/v1/provider/balances`

**Headers:**
```
Authorization: HMAC {CLIENT_ID}:{SIGNATURE}
Content-Type: application/json
```

**Request Body:**
```json
{
  "balances": [
    {
      "currency": "NGN",
      "available": 50000.00,
      "total": 100000.00,
      "reserved": 50000.00
    }
  ],
  "timestamp": 1728201600
}
```

## HMAC Authentication

### Signature Generation

```python
import hmac
import hashlib
import json
import time

def generate_hmac_signature(payload, secret_key):
    """
    Generate HMAC-SHA256 signature for aggregator authentication
    
    Args:
        payload: Dictionary containing request data
        secret_key: Your AGGREGATOR_SECRET_KEY
    
    Returns:
        Hex-encoded signature string
    """
    # Add timestamp to payload
    payload['timestamp'] = int(time.time())
    
    # Convert to JSON string (sorted keys for consistency)
    payload_json = json.dumps(payload, separators=(',', ':'), sort_keys=True)
    
    # Generate HMAC signature
    signature = hmac.new(
        secret_key.encode('utf-8'),
        payload_json.encode('utf-8'),
        hashlib.sha256
    ).hexdigest()
    
    return signature

# Example usage
payload = {"transactionHash": "psp-tx-123"}
secret_key = "97F5-4ZyPoKynLCelFc1H8sE6Rv_I4HaCOhf4ha14sA="
signature = generate_hmac_signature(payload, secret_key)

# Build authorization header
client_id = "0c73884d-4438-41a8-9624-d6aec679f868"
auth_header = f"HMAC {client_id}:{signature}"
```

### Signature Verification (for incoming requests)

```python
def verify_hmac_signature(payload, signature, secret_key):
    """
    Verify HMAC signature from aggregator
    
    Args:
        payload: Dictionary containing request data
        signature: Signature from Authorization header
        secret_key: Your AGGREGATOR_SECRET_KEY
    
    Returns:
        Boolean indicating if signature is valid
    """
    # Generate expected signature
    expected_signature = generate_hmac_signature(payload, secret_key)
    
    # Compare signatures (timing-safe comparison)
    return hmac.compare_digest(signature, expected_signature)
```

## Database Schema

### Orders Table
```sql
CREATE TABLE orders (
    id TEXT PRIMARY KEY,
    aggregator_order_id TEXT UNIQUE NOT NULL,
    amount REAL NOT NULL,
    currency TEXT NOT NULL,
    token TEXT NOT NULL,
    token_amount REAL NOT NULL,
    network TEXT NOT NULL,
    recipient_institution TEXT NOT NULL,
    recipient_account TEXT NOT NULL,
    recipient_name TEXT NOT NULL,
    recipient_memo TEXT,
    reference TEXT,
    status TEXT NOT NULL, -- pending, accepted, processing, fulfilled, declined, cancelled, failed
    psp_transaction_id TEXT,
    error_message TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP
);

CREATE INDEX idx_orders_status ON orders(status);
CREATE INDEX idx_orders_aggregator_id ON orders(aggregator_order_id);
```

### Balances Table
```sql
CREATE TABLE balances (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    currency TEXT UNIQUE NOT NULL,
    available REAL NOT NULL DEFAULT 0,
    total REAL NOT NULL DEFAULT 0,
    reserved REAL NOT NULL DEFAULT 0,
    last_synced_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

### Transactions Table
```sql
CREATE TABLE transactions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    order_id TEXT NOT NULL,
    psp_transaction_id TEXT,
    amount REAL NOT NULL,
    currency TEXT NOT NULL,
    status TEXT NOT NULL, -- initiated, pending, success, failed
    psp_response TEXT, -- JSON response from PSP
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (order_id) REFERENCES orders(id)
);
```

## PSP Integration (Lenco Example)

### Initialize Payment

```python
import requests

def initiate_lenco_payment(order):
    """
    Initiate payment via Lenco API
    
    Args:
        order: Order object from database
    
    Returns:
        Transaction ID from Lenco
    """
    url = f"{LENCO_BASE_URL}/transfers"
    
    headers = {
        "Authorization": f"Bearer {LENCO_API_KEY}",
        "Content-Type": "application/json"
    }
    
    payload = {
        "account_id": LENCO_ACCOUNT_ID,
        "amount": order['amount'],
        "currency": order['currency'],
        "recipient": {
            "bank_code": order['recipient_institution'],
            "account_number": order['recipient_account'],
            "account_name": order['recipient_name']
        },
        "narration": order['recipient_memo'],
        "reference": order['reference']
    }
    
    response = requests.post(url, json=payload, headers=headers)
    response.raise_for_status()
    
    data = response.json()
    return data['data']['transaction_id']
```

### Check Payment Status

```python
def check_lenco_payment_status(transaction_id):
    """
    Check payment status from Lenco
    
    Args:
        transaction_id: Transaction ID from Lenco
    
    Returns:
        Status string: 'pending', 'success', 'failed'
    """
    url = f"{LENCO_BASE_URL}/transfers/{transaction_id}"
    
    headers = {
        "Authorization": f"Bearer {LENCO_API_KEY}"
    }
    
    response = requests.get(url, headers=headers)
    response.raise_for_status()
    
    data = response.json()
    return data['data']['status']
```

## Order Processing Flow

### Automatic Mode

```
1. Aggregator assigns order → POST /orders
2. Node validates order (balance, limits, etc.)
3. Node accepts order → POST /v1/provider/orders/{id}/accept
4. Node initiates PSP payment
5. Node polls PSP for status OR receives webhook
6. On success: Node fulfills order → POST /v1/provider/orders/{id}/fulfill
7. On failure: Node cancels order → POST /v1/provider/orders/{id}/cancel
8. Node updates balance → POST /v1/provider/balances
```

### Manual Mode

```
1. Aggregator assigns order → POST /orders
2. Node stores order in database with status 'pending'
3. Admin reviews order via dashboard
4. Admin approves → Node accepts order → POST /v1/provider/orders/{id}/accept
5. Admin initiates payment manually
6. Admin marks as fulfilled → Node fulfills order → POST /v1/provider/orders/{id}/fulfill
   OR Admin declines → Node declines order → POST /v1/provider/orders/{id}/decline
```

## Configuration

### Environment Variables

```bash
# Server Configuration
SERVER_HOST=0.0.0.0
SERVER_PORT=8000
DEBUG=False
SECRET=your-app-secret-key

# Aggregator Configuration
AGGREGATOR_BASE_URL=http://host.docker.internal:8000
AGGREGATOR_CLIENT_ID=0c73884d-4438-41a8-9624-d6aec679f868
AGGREGATOR_SECRET_KEY=97F5-4ZyPoKynLCelFc1H8sE6Rv_I4HaCOhf4ha14sA=

# Supported Currencies (comma-separated)
CURRENCIES=NGN,KES

# Operation Mode
PROVISION_MODE=auto  # 'auto' or 'manual'

# PSP Configuration (Lenco)
LENCO_BASE_URL=https://sandbox.lenco.co/access/v1
LENCO_API_KEY=your-lenco-api-key
LENCO_ACCOUNT_ID=your-lenco-account-id

# Database
DATABASE_URL=sqlite:///provision_node.db

# Polling Configuration (for auto mode)
ORDER_POLL_INTERVAL=30  # seconds
PSP_STATUS_CHECK_INTERVAL=10  # seconds
BALANCE_SYNC_INTERVAL=300  # seconds (5 minutes)

# Limits
MAX_ORDER_AMOUNT=50000
MIN_ORDER_AMOUNT=0.5
CRITICAL_BALANCE_THRESHOLD=10000
```

## Error Handling

### Error Codes

```python
ERROR_CODES = {
    "INSUFFICIENT_BALANCE": "Provider balance too low",
    "AMOUNT_EXCEEDS_LIMIT": "Order amount exceeds maximum limit",
    "AMOUNT_BELOW_MINIMUM": "Order amount below minimum limit",
    "UNSUPPORTED_CURRENCY": "Currency not supported",
    "UNSUPPORTED_INSTITUTION": "Bank/institution not supported",
    "PSP_TIMEOUT": "Payment service provider timeout",
    "PSP_ERROR": "Payment service provider error",
    "INVALID_ACCOUNT": "Invalid recipient account",
    "DUPLICATE_ORDER": "Order already exists",
    "ORDER_EXPIRED": "Order has expired",
    "INVALID_SIGNATURE": "Invalid HMAC signature"
}
```

### Retry Logic

```python
def retry_with_backoff(func, max_retries=3, backoff_factor=2):
    """
    Retry function with exponential backoff
    
    Args:
        func: Function to retry
        max_retries: Maximum number of retry attempts
        backoff_factor: Multiplier for backoff delay
    
    Returns:
        Function result or raises last exception
    """
    for attempt in range(max_retries):
        try:
            return func()
        except Exception as e:
            if attempt == max_retries - 1:
                raise
            
            delay = backoff_factor ** attempt
            time.sleep(delay)
```

## Logging

### Log Levels

- **INFO**: Order received, order accepted, order fulfilled, balance updated
- **WARNING**: Order declined, insufficient balance, approaching critical balance
- **ERROR**: PSP error, order cancellation, failed to update aggregator
- **DEBUG**: API requests/responses, signature verification

### Log Format

```json
{
  "timestamp": "2025-10-06T13:00:00Z",
  "level": "INFO",
  "message": "Order fulfilled successfully",
  "context": {
    "orderId": "uuid",
    "amount": 736.155,
    "currency": "NGN",
    "pspTransactionId": "psp-tx-123"
  }
}
```

## Monitoring & Health Checks

### Metrics to Track

1. **Order Metrics**
   - Total orders received
   - Orders accepted/declined/cancelled
   - Orders fulfilled/failed
   - Average fulfillment time
   - Success rate

2. **Balance Metrics**
   - Current available balance per currency
   - Reserved balance per currency
   - Balance utilization percentage
   - Critical balance alerts

3. **PSP Metrics**
   - PSP API response time
   - PSP success rate
   - PSP error rate
   - Webhook delivery rate

4. **System Metrics**
   - API response time
   - Error rate
   - Uptime
   - Active orders count

### Health Check Response

```json
{
  "status": "healthy",
  "timestamp": "2025-10-06T13:00:00Z",
  "checks": {
    "database": "ok",
    "psp_connection": "ok",
    "aggregator_connection": "ok",
    "balance_status": "ok"
  },
  "metrics": {
    "active_orders": 5,
    "pending_orders": 2,
    "available_balance_ngn": 50000.00
  }
}
```

## Security Considerations

### 1. HMAC Signature Validation
- Always verify HMAC signatures on incoming requests from aggregator
- Use timing-safe comparison to prevent timing attacks
- Reject requests with expired timestamps (> 5 minutes old)

### 2. Rate Limiting
- Implement rate limiting on all endpoints
- Suggested limits:
  - `/orders`: 10 requests/minute
  - `/info`: 60 requests/minute
  - `/health`: 120 requests/minute

### 3. Input Validation
- Validate all input data (amounts, currencies, account numbers)
- Sanitize recipient information
- Validate order expiration times

### 4. Secrets Management
- Never hardcode API keys or secrets
- Use environment variables or secret management services
- Rotate secrets regularly

### 5. Audit Logging
- Log all order state changes
- Log all API requests/responses
- Store logs securely with retention policy

## Testing

### Unit Tests

```python
def test_hmac_signature_generation():
    """Test HMAC signature generation"""
    payload = {"test": "data"}
    secret = "test-secret"
    signature = generate_hmac_signature(payload, secret)
    assert len(signature) == 64  # SHA256 hex length

def test_order_validation():
    """Test order validation logic"""
    order = {
        "amount": 1000,
        "currency": "NGN",
        "recipient": {...}
    }
    assert validate_order(order) == True

def test_balance_check():
    """Test balance checking"""
    assert check_balance("NGN", 1000) == True
    assert check_balance("NGN", 1000000) == False
```

### Integration Tests

```python
def test_order_flow_end_to_end():
    """Test complete order flow"""
    # 1. Receive order
    response = client.post('/orders', json=order_payload)
    assert response.status_code == 200
    
    # 2. Check order accepted
    order_id = response.json['data']['orderId']
    # Mock aggregator accept endpoint
    
    # 3. Mock PSP payment
    # Mock PSP success response
    
    # 4. Check order fulfilled
    # Verify aggregator fulfill endpoint called

def test_aggregator_communication():
    """Test communication with aggregator"""
    # Mock aggregator endpoints
    # Test accept, fulfill, decline, cancel, balance update
```

## Deployment

### Docker Deployment

```dockerfile
FROM python:3.11-slim

WORKDIR /app

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY . .

EXPOSE 8000

CMD ["python", "app.py"]
```

### Docker Compose

```yaml
version: '3.8'

services:
  provision-node:
    build: .
    ports:
      - "8105:8000"
    environment:
      - AGGREGATOR_BASE_URL=http://host.docker.internal:8000
      - AGGREGATOR_CLIENT_ID=${AGGREGATOR_CLIENT_ID}
      - AGGREGATOR_SECRET_KEY=${AGGREGATOR_SECRET_KEY}
      - LENCO_API_KEY=${LENCO_API_KEY}
      - LENCO_ACCOUNT_ID=${LENCO_ACCOUNT_ID}
      - CURRENCIES=NGN
      - PROVISION_MODE=auto
    volumes:
      - ./data:/app/data
    restart: unless-stopped
```

## Maintenance

### Database Backups

```bash
# Backup SQLite database
sqlite3 provision_node.db ".backup provision_node_backup.db"

# Schedule daily backups
0 0 * * * sqlite3 /app/data/provision_node.db ".backup /app/backups/provision_node_$(date +\%Y\%m\%d).db"
```

### Log Rotation

```bash
# Rotate logs daily, keep 30 days
/var/log/provision-node/*.log {
    daily
    rotate 30
    compress
    delaycompress
    notifempty
    create 0640 root root
}
```

## Support & Troubleshooting

### Common Issues

1. **Orders not being received**
   - Check aggregator can reach provider node
   - Verify `host_identifier` in database
   - Check firewall/network configuration

2. **Authentication failures**
   - Verify HMAC signature generation
   - Check timestamp is within 5 minutes
   - Verify CLIENT_ID and SECRET_KEY match database

3. **PSP payment failures**
   - Check PSP API credentials
   - Verify recipient account details
   - Check PSP API status

4. **Balance sync issues**
   - Check balance update endpoint
   - Verify balance calculation logic
   - Check reserved balance tracking

### Debug Mode

Enable debug logging:
```bash
DEBUG=True
LOG_LEVEL=DEBUG
```

This will log:
- All API requests/responses
- HMAC signature details
- PSP API calls
- Database queries
