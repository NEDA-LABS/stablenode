# Testing Quick Reference

## 🚀 Quick Commands

```bash
# Run all tests
./run_tests.sh all

# Run unit tests only (fastest)
./run_tests.sh unit

# Run with coverage
./run_tests.sh coverage

# Run specific test
./run_tests.sh specific TestAlchemyWebhookHandler
```

## 📁 Test Organization

```
tests/
├── unit/           # Fast, no dependencies (10 tests)
├── integration/    # Mocked integration (8 tests)
└── e2e/           # Full system (6 tests)

services/
└── alchemy_webhook_test.go  # Service tests (8 tests)
```

## ✅ Test Checklist

Before committing code:
- [ ] Run `./run_tests.sh quick` (< 5s)
- [ ] All tests pass
- [ ] Coverage > 80%
- [ ] No lint errors

Before deploying:
- [ ] Run `./run_tests.sh all`
- [ ] Run `./run_tests.sh e2e` (requires running app)
- [ ] Check `coverage.html`

## 🔧 Common Test Commands

```bash
# Unit tests
go test ./tests/unit/... -v

# Service tests
go test ./services/alchemy_webhook_test.go -v

# Integration tests
go test ./tests/integration/... -v

# E2E tests (requires app running)
go test ./tests/e2e/... -v

# Skip slow tests
go test ./tests/... -v -short

# Run with race detector
go test ./tests/... -race

# Verbose output
go test ./tests/... -v -count=1

# Coverage
go test ./tests/... -cover -coverprofile=coverage.out
go tool cover -html=coverage.out
```

## 🐛 Debugging Tests

```bash
# Run single test with logs
go test ./tests/unit/... -v -run TestAlchemyWebhookHandler

# Print debug info
t.Logf("Debug: %v", variable)

# Use debugger
dlv test ./tests/unit/... -- -test.run TestName
```

## 📊 Test Metrics

| Type | Count | Speed | Coverage |
|------|-------|-------|----------|
| Unit | 10 | < 1s | 90% |
| Service | 8 | < 2s | 85% |
| Integration | 8 | < 5s | 80% |
| E2E | 6 | < 30s | 75% |
| **Total** | **32** | **< 40s** | **85%** |

## 🎯 Test Scenarios

### Unit Tests
✅ Webhook handler logic
✅ Payload parsing
✅ Amount calculations
✅ Address matching
✅ Payment validation

### Integration Tests
✅ Webhook flow
✅ Signature verification
✅ Concurrent processing
✅ Retry mechanism

### E2E Tests
✅ Order creation
✅ Webhook callback
✅ Database updates
✅ Idempotency

## 🔑 Environment Variables

```bash
# For E2E tests
export ALCHEMY_API_KEY=test_key
export ALCHEMY_AUTH_TOKEN=test_token
export DB_NAME=test_db
```

## 📝 Writing New Tests

### Template
```go
func TestFeature(t *testing.T) {
    // Arrange
    input := "test"
    
    // Act
    result := function(input)
    
    // Assert
    assert.Equal(t, expected, result)
}
```

### Table-Driven
```go
tests := []struct {
    name     string
    input    string
    expected string
}{
    {"case 1", "in1", "out1"},
    {"case 2", "in2", "out2"},
}

for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        result := function(tt.input)
        assert.Equal(t, tt.expected, result)
    })
}
```

## 🚨 Troubleshooting

| Error | Solution |
|-------|----------|
| Connection refused | Start app: `docker-compose up -d` |
| Database not found | Run migrations |
| Test timeout | Increase timeout or check for deadlocks |
| Signature failed | Check signing key |

## 📚 Documentation

- **Full Guide**: `tests/README.md`
- **Test Summary**: `TEST_IMPLEMENTATION_SUMMARY.md`
- **Webhook Setup**: `ALCHEMY_WEBHOOK_SETUP.md`

## 🎬 CI/CD

Tests run automatically on:
- Every commit (unit + integration)
- Pull requests (all tests)
- Before deployment (all + E2E)

---

**Quick Help**: `./run_tests.sh help`
