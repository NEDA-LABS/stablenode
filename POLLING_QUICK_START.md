# Polling Fallback - Quick Start

## 🚀 5-Minute Setup

### 1. Add Configuration
```bash
# .env
ENABLE_POLLING_FALLBACK=true
POLLING_INTERVAL=1m  # Check every 1 minute
```

### 2. Create Polling Service
```bash
# Create file
touch services/polling_service.go
```

Copy implementation from `POLLING_FALLBACK_GUIDE.md`

### 3. Start Service in main.go
```go
// In main.go, after app initialization
if viper.GetBool("ENABLE_POLLING_FALLBACK") {
    pollingService := services.NewPollingService(
        viper.GetDuration("POLLING_INTERVAL"),
    )
    go pollingService.Start(context.Background())
}
```

### 4. Test
```bash
# Create test order
curl -X POST http://localhost:8000/v1/sender/orders ...

# Send crypto to receive address

# Wait for polling interval (1 minute)

# Check order status
curl http://localhost:8000/v1/sender/orders/{order_id}
```

---

## 📊 Comparison

| Method | Detection Time | RPC Calls | Setup |
|--------|---------------|-----------|-------|
| **Webhook** | < 1 second | 0 | Medium |
| **Polling (1m)** | ~30 seconds avg | High | Easy |
| **Polling (30s)** | ~15 seconds avg | Very High | Easy |

---

## 💡 Recommendations

### Development
```bash
ENABLE_POLLING_FALLBACK=true
POLLING_INTERVAL=30s
```

### Production (with webhooks)
```bash
ENABLE_POLLING_FALLBACK=true  # Fallback only
POLLING_INTERVAL=5m           # Check old orders
```

### Production (without webhooks)
```bash
ENABLE_POLLING_FALLBACK=true
POLLING_INTERVAL=1m           # Balance speed vs cost
```

---

## 🎯 Key Features

✅ **No Public URL Required** - Works locally
✅ **Simple Setup** - Just enable in .env
✅ **Reliable** - Direct blockchain queries
✅ **Automatic** - Background service
✅ **Configurable** - Adjust interval as needed

---

## 🔧 Advanced Options

### Batch Processing
Poll multiple orders in one RPC call (saves costs)

### Smart Intervals
- Recent orders: Every 10s
- Medium age: Every 1m
- Old orders: Every 5m

### Caching
Cache balances for 30s to reduce RPC calls

---

## 📝 Full Guide

See `POLLING_FALLBACK_GUIDE.md` for:
- Complete implementation
- Performance optimization
- Cost analysis
- Monitoring setup
- Testing strategies

---

**Time to Implement**: 30 minutes
**Difficulty**: Easy
**Cost**: Moderate (RPC calls)
