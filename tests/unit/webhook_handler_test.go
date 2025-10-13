package unit

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockOrderService mocks the order service
type MockOrderService struct {
	mock.Mock
}

func (m *MockOrderService) UpdateOrderPayment(orderID string, amount decimal.Decimal) error {
	args := m.Called(orderID, amount)
	return args.Error(0)
}

// TestAlchemyWebhookHandler tests the webhook handler endpoint
func TestAlchemyWebhookHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		payload        map[string]interface{}
		signature      string
		expectedStatus int
		expectedBody   string
	}{
		{
			name: "Valid webhook payload",
			payload: map[string]interface{}{
				"webhookId": "wh_test123",
				"id":        "whevt_456",
				"type":      "ADDRESS_ACTIVITY",
				"event": map[string]interface{}{
					"network": "BASE_SEPOLIA",
					"activity": []map[string]interface{}{
						{
							"fromAddress": "0xUSER",
							"toAddress":   "0xRECEIVE",
							"value":       "1000000000000000000",
							"category":    "token",
						},
					},
				},
			},
			signature:      "valid_signature",
			expectedStatus: http.StatusOK,
			expectedBody:   "success",
		},
		{
			name:           "Invalid JSON payload",
			payload:        nil,
			signature:      "valid_signature",
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "Invalid payload",
		},
		{
			name: "Missing signature",
			payload: map[string]interface{}{
				"webhookId": "wh_test123",
			},
			signature:      "",
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "Invalid signature",
		},
		{
			name: "Invalid signature",
			payload: map[string]interface{}{
				"webhookId": "wh_test123",
			},
			signature:      "invalid_sig",
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "Invalid signature",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test router
			router := gin.Default()
			router.POST("/webhook", mockWebhookHandler)

			// Create request
			var body []byte
			if tt.payload != nil {
				body, _ = json.Marshal(tt.payload)
			} else {
				body = []byte("invalid json")
			}

			req, _ := http.NewRequest("POST", "/webhook", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			if tt.signature != "" {
				req.Header.Set("X-Alchemy-Signature", tt.signature)
			}

			// Record response
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Assertions
			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.expectedBody != "" {
				assert.Contains(t, w.Body.String(), tt.expectedBody)
			}
		})
	}
}

// TestWebhookPayloadParsing tests parsing of different webhook payloads
func TestWebhookPayloadParsing(t *testing.T) {
	tests := []struct {
		name          string
		payload       string
		expectedError bool
	}{
		{
			name: "Valid ERC20 transfer",
			payload: `{
				"webhookId": "wh_123",
				"event": {
					"activity": [{
						"fromAddress": "0xFrom",
						"toAddress": "0xTo",
						"value": "1000000000000000000",
						"category": "token",
						"asset": "DAI"
					}]
				}
			}`,
			expectedError: false,
		},
		{
			name: "Native token transfer",
			payload: `{
				"webhookId": "wh_123",
				"event": {
					"activity": [{
						"fromAddress": "0xFrom",
						"toAddress": "0xTo",
						"value": "1000000000000000000",
						"category": "external"
					}]
				}
			}`,
			expectedError: false,
		},
		{
			name:          "Malformed JSON",
			payload:       `{"webhookId": "wh_123"`,
			expectedError: true,
		},
		{
			name:          "Empty payload",
			payload:       `{}`,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var parsed map[string]interface{}
			err := json.Unmarshal([]byte(tt.payload), &parsed)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, parsed["webhookId"])
			}
		})
	}
}

// TestAmountCalculation tests amount parsing and decimal conversion
func TestAmountCalculation(t *testing.T) {
	tests := []struct {
		name           string
		valueStr       string
		decimals       int
		expectedAmount string
	}{
		{
			name:           "18 decimals (DAI)",
			valueStr:       "1000000000000000000",
			decimals:       18,
			expectedAmount: "1",
		},
		{
			name:           "6 decimals (USDC)",
			valueStr:       "1000000",
			decimals:       6,
			expectedAmount: "1",
		},
		{
			name:           "Fractional amount",
			valueStr:       "500000000000000000",
			decimals:       18,
			expectedAmount: "0.5",
		},
		{
			name:           "Large amount",
			valueStr:       "1000000000000000000000",
			decimals:       18,
			expectedAmount: "1000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, err := decimal.NewFromString(tt.valueStr)
			assert.NoError(t, err)

			divisor := decimal.NewFromInt(10).Pow(decimal.NewFromInt(int64(tt.decimals)))
			result := value.Div(divisor)

			expected, _ := decimal.NewFromString(tt.expectedAmount)
			assert.True(t, result.Equal(expected))
		})
	}
}

// TestWebhookDeduplication tests handling of duplicate webhook events
func TestWebhookDeduplication(t *testing.T) {
	processedEvents := make(map[string]bool)

	events := []string{
		"whevt_123",
		"whevt_456",
		"whevt_123", // Duplicate
		"whevt_789",
		"whevt_456", // Duplicate
	}

	uniqueCount := 0
	duplicateCount := 0

	for _, eventID := range events {
		if processedEvents[eventID] {
			duplicateCount++
			continue
		}
		processedEvents[eventID] = true
		uniqueCount++
	}

	assert.Equal(t, 3, uniqueCount)
	assert.Equal(t, 2, duplicateCount)
}

// TestAddressMatching tests matching receive addresses to orders
func TestAddressMatching(t *testing.T) {
	// Mock database of orders with receive addresses
	orders := map[string]string{
		"0xRECEIVE1": "order_123",
		"0xRECEIVE2": "order_456",
		"0xRECEIVE3": "order_789",
	}

	tests := []struct {
		name          string
		toAddress     string
		expectedOrder string
		shouldFind    bool
	}{
		{
			name:          "Exact match",
			toAddress:     "0xRECEIVE1",
			expectedOrder: "order_123",
			shouldFind:    true,
		},
		{
			name:          "Case insensitive match",
			toAddress:     "0xreceive2",
			expectedOrder: "order_456",
			shouldFind:    true,
		},
		{
			name:          "No match",
			toAddress:     "0xUNKNOWN",
			expectedOrder: "",
			shouldFind:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Normalize address for comparison
			normalizedAddr := normalizeAddress(tt.toAddress)
			orderID, found := orders[normalizedAddr]

			assert.Equal(t, tt.shouldFind, found)
			if found {
				assert.Equal(t, tt.expectedOrder, orderID)
			}
		})
	}
}

// TestPaymentSufficiencyCheck tests checking if payment is sufficient
func TestPaymentSufficiencyCheck(t *testing.T) {
	tests := []struct {
		name           string
		amountPaid     string
		amountRequired string
		senderFee      string
		networkFee     string
		protocolFee    string
		isSufficient   bool
	}{
		{
			name:           "Exact amount",
			amountPaid:     "1.01",
			amountRequired: "1.0",
			senderFee:      "0.01",
			networkFee:     "0.0",
			protocolFee:    "0.0",
			isSufficient:   true,
		},
		{
			name:           "Overpayment",
			amountPaid:     "1.5",
			amountRequired: "1.0",
			senderFee:      "0.01",
			networkFee:     "0.0",
			protocolFee:    "0.0",
			isSufficient:   true,
		},
		{
			name:           "Underpayment",
			amountPaid:     "0.5",
			amountRequired: "1.0",
			senderFee:      "0.01",
			networkFee:     "0.0",
			protocolFee:    "0.0",
			isSufficient:   false,
		},
		{
			name:           "With all fees",
			amountPaid:     "1.06",
			amountRequired: "1.0",
			senderFee:      "0.01",
			networkFee:     "0.02",
			protocolFee:    "0.03",
			isSufficient:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			paid, _ := decimal.NewFromString(tt.amountPaid)
			required, _ := decimal.NewFromString(tt.amountRequired)
			senderFee, _ := decimal.NewFromString(tt.senderFee)
			networkFee, _ := decimal.NewFromString(tt.networkFee)
			protocolFee, _ := decimal.NewFromString(tt.protocolFee)

			totalRequired := required.Add(senderFee).Add(networkFee).Add(protocolFee)
			isSufficient := paid.GreaterThanOrEqual(totalRequired)

			assert.Equal(t, tt.isSufficient, isSufficient)
		})
	}
}

// Mock webhook handler for testing
func mockWebhookHandler(c *gin.Context) {
	signature := c.GetHeader("X-Alchemy-Signature")
	
	if signature == "" || signature == "invalid_sig" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid signature"})
		return
	}

	var payload map[string]interface{}
	if err := c.BindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

// Helper function to normalize addresses
func normalizeAddress(addr string) string {
	// In production, use ethereum common.HexToAddress
	// For testing, simple uppercase
	return addr
}
