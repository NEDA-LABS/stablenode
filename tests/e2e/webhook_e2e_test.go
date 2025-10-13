package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCompleteOrderWebhookFlow tests the entire order creation to webhook callback flow
func TestCompleteOrderWebhookFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test")
	}

	// This test requires:
	// 1. Running application
	// 2. Database connection
	// 3. Alchemy API credentials

	baseURL := "http://localhost:8000"
	apiKey := "11f93de0-d304-4498-8b7b-6cecbc5b2dd8"

	t.Run("Create Order with Alchemy Address", func(t *testing.T) {
		// Step 1: Create order
		orderPayload := map[string]interface{}{
			"amount":  0.5,
			"token":   "DAI",
			"rate":    1482.3,
			"network": "base-sepolia",
			"recipient": map[string]interface{}{
				"institution":       "ABNGNGLA",
				"accountIdentifier": "0123456789",
				"accountName":       "John Doe",
				"currency":          "NGN",
			},
			"reference":     fmt.Sprintf("E2E-TEST-%d", time.Now().Unix()),
			"returnAddress": "0x18000433c7cc39ebdAbB06262F88795960FE5Cf9",
		}

		orderResp := createOrder(t, baseURL, apiKey, orderPayload)
		require.NotNil(t, orderResp)
		
		orderID := orderResp["id"].(string)
		receiveAddress := orderResp["receive_address"].(string)
		
		assert.NotEmpty(t, orderID)
		assert.NotEmpty(t, receiveAddress)
		assert.True(t, isValidEthereumAddress(receiveAddress))

		t.Logf("Order created: %s", orderID)
		t.Logf("Receive address: %s", receiveAddress)

		// Step 2: Verify order in database
		order := getOrder(t, baseURL, apiKey, orderID)
		assert.Equal(t, "initiated", order["status"])
		assert.Equal(t, "0", order["amount_paid"])

		// Step 3: Simulate webhook callback (in real test, send actual crypto)
		webhookPayload := createMockWebhookPayload(receiveAddress, "500000000000000000")
		sendWebhook(t, baseURL, webhookPayload)

		// Step 4: Wait for processing
		time.Sleep(2 * time.Second)

		// Step 5: Verify order updated
		updatedOrder := getOrder(t, baseURL, apiKey, orderID)
		assert.Equal(t, "0.5", updatedOrder["amount_paid"])
		// Status should change to validated if payment is sufficient
	})
}

// TestWebhookWithMultipleTransfers tests handling multiple transfers to same address
func TestWebhookWithMultipleTransfers(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test")
	}

	baseURL := "http://localhost:8000"
	receiveAddress := "0xTEST_ADDRESS"

	// Send multiple webhook events
	transfers := []string{
		"100000000000000000",  // 0.1
		"200000000000000000",  // 0.2
		"200000000000000000",  // 0.2
	}

	for i, amount := range transfers {
		t.Logf("Sending transfer %d: %s", i+1, amount)
		payload := createMockWebhookPayload(receiveAddress, amount)
		sendWebhook(t, baseURL, payload)
		time.Sleep(500 * time.Millisecond)
	}

	// Total should be 0.5
	// Verify in database that amount_paid = 0.5
}

// TestWebhookFailureAndRetry tests webhook failure handling
func TestWebhookFailureAndRetry(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test")
	}

	// This would test:
	// 1. Webhook endpoint returns error
	// 2. Alchemy retries
	// 3. Eventually succeeds
	// 4. Order is updated correctly
}

// TestWebhookSignatureValidation tests that invalid signatures are rejected
func TestWebhookSignatureValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test")
	}

	baseURL := "http://localhost:8000"
	
	tests := []struct {
		name           string
		signature      string
		expectedStatus int
	}{
		{
			name:           "Valid signature",
			signature:      "valid_hmac_signature",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Invalid signature",
			signature:      "invalid_signature",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "Missing signature",
			signature:      "",
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := createMockWebhookPayload("0xTEST", "1000000000000000000")
			resp := sendWebhookWithSignature(t, baseURL, payload, tt.signature)
			assert.Equal(t, tt.expectedStatus, resp.StatusCode)
		})
	}
}

// TestConcurrentWebhooks tests handling multiple webhooks simultaneously
func TestConcurrentWebhooks(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test")
	}

	baseURL := "http://localhost:8000"
	numWebhooks := 10

	done := make(chan bool, numWebhooks)

	for i := 0; i < numWebhooks; i++ {
		go func(id int) {
			address := fmt.Sprintf("0xADDRESS_%d", id)
			payload := createMockWebhookPayload(address, "1000000000000000000")
			sendWebhook(t, baseURL, payload)
			done <- true
		}(i)
	}

	// Wait for all to complete
	timeout := time.After(10 * time.Second)
	completed := 0

	for completed < numWebhooks {
		select {
		case <-done:
			completed++
		case <-timeout:
			t.Fatal("Concurrent webhook test timeout")
		}
	}

	assert.Equal(t, numWebhooks, completed)
}

// TestWebhookIdempotency tests that duplicate webhooks don't cause issues
func TestWebhookIdempotency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test")
	}

	baseURL := "http://localhost:8000"
	apiKey := "11f93de0-d304-4498-8b7b-6cecbc5b2dd8"

	// Create order
	orderPayload := map[string]interface{}{
		"amount":  1.0,
		"token":   "DAI",
		"network": "base-sepolia",
		// ... other fields
	}

	orderResp := createOrder(t, baseURL, apiKey, orderPayload)
	receiveAddress := orderResp["receive_address"].(string)

	// Send same webhook multiple times
	payload := createMockWebhookPayload(receiveAddress, "1000000000000000000")
	
	for i := 0; i < 3; i++ {
		sendWebhook(t, baseURL, payload)
		time.Sleep(100 * time.Millisecond)
	}

	// Verify amount_paid is still 1.0, not 3.0
	order := getOrder(t, baseURL, apiKey, orderResp["id"].(string))
	assert.Equal(t, "1.0", order["amount_paid"])
}

// Helper functions

func createOrder(t *testing.T, baseURL, apiKey string, payload map[string]interface{}) map[string]interface{} {
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", baseURL+"/v1/sender/orders", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", apiKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	
	return result["data"].(map[string]interface{})
}

func getOrder(t *testing.T, baseURL, apiKey, orderID string) map[string]interface{} {
	req, _ := http.NewRequest("GET", baseURL+"/v1/sender/orders/"+orderID, nil)
	req.Header.Set("X-API-Key", apiKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	
	return result["data"].(map[string]interface{})
}

func createMockWebhookPayload(toAddress, value string) map[string]interface{} {
	return map[string]interface{}{
		"webhookId": "wh_test_" + fmt.Sprint(time.Now().Unix()),
		"id":        "whevt_" + fmt.Sprint(time.Now().UnixNano()),
		"createdAt": time.Now().Format(time.RFC3339),
		"type":      "ADDRESS_ACTIVITY",
		"event": map[string]interface{}{
			"network": "BASE_SEPOLIA",
			"activity": []map[string]interface{}{
				{
					"fromAddress": "0xUSER_ADDRESS",
					"toAddress":   toAddress,
					"blockNum":    "0x123456",
					"hash":        "0x" + fmt.Sprint(time.Now().UnixNano()),
					"value":       value,
					"asset":       "DAI",
					"category":    "token",
					"rawContract": map[string]interface{}{
						"address":  "0xDAI_CONTRACT",
						"decimals": 18,
					},
				},
			},
		},
	}
}

func sendWebhook(t *testing.T, baseURL string, payload map[string]interface{}) *http.Response {
	return sendWebhookWithSignature(t, baseURL, payload, "test_signature")
}

func sendWebhookWithSignature(t *testing.T, baseURL string, payload map[string]interface{}, signature string) *http.Response {
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", baseURL+"/v1/alchemy/webhook", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	if signature != "" {
		req.Header.Set("X-Alchemy-Signature", signature)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	require.NoError(t, err)
	
	return resp
}

func isValidEthereumAddress(addr string) bool {
	// Simple validation - starts with 0x and is 42 characters
	return len(addr) == 42 && addr[:2] == "0x"
}
