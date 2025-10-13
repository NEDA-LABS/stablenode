# Provision Node - Go Implementation Guide

## Quick Start

### Prerequisites
```bash
# Install Go 1.21+
wget https://go.dev/dl/go1.21.6.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.21.6.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin

# Verify installation
go version
```

### Initialize Project
```bash
mkdir provision-node
cd provision-node
go mod init github.com/yourusername/provision-node

# Install dependencies
go get github.com/gin-gonic/gin
go get gorm.io/gorm
go get gorm.io/driver/sqlite
go get gorm.io/driver/postgres
go get github.com/joho/godotenv
go get github.com/go-co-op/gocron
go get github.com/sirupsen/logrus
go get github.com/stretchr/testify
```

## Core Implementation Examples

### 1. Main Application (main.go)

```go
package main

import (
    "log"
    "os"

    "github.com/gin-gonic/gin"
    "github.com/joho/godotenv"
    "github.com/yourusername/provision-node/config"
    "github.com/yourusername/provision-node/handlers"
    "github.com/yourusername/provision-node/middleware"
    "github.com/yourusername/provision-node/models"
    "github.com/yourusername/provision-node/tasks"
)

func main() {
    // Load environment variables
    if err := godotenv.Load(); err != nil {
        log.Println("No .env file found")
    }

    // Initialize configuration
    cfg := config.Load()

    // Initialize database
    db, err := models.InitDB(cfg.DatabaseURL)
    if err != nil {
        log.Fatalf("Failed to connect to database: %v", err)
    }

    // Run migrations
    if err := models.Migrate(db); err != nil {
        log.Fatalf("Failed to migrate database: %v", err)
    }

    // Initialize Gin
    if !cfg.Debug {
        gin.SetMode(gin.ReleaseMode)
    }
    router := gin.Default()

    // Middleware
    router.Use(middleware.Logger())
    router.Use(middleware.Recovery())

    // Public routes
    router.GET("/health", handlers.HealthCheck(cfg))
    router.GET("/info", handlers.NodeInfo(cfg))

    // Protected routes (HMAC authentication)
    protected := router.Group("/")
    protected.Use(middleware.HMACAuth(cfg.AggregatorSecretKey))
    {
        protected.POST("/orders", handlers.ReceiveOrder(db, cfg))
    }

    // PSP webhook routes
    router.POST("/webhooks/psp", handlers.PSPWebhook(db, cfg))

    // Start background tasks
    scheduler := tasks.NewScheduler(db, cfg)
    scheduler.Start()

    // Start server
    addr := cfg.ServerHost + ":" + cfg.ServerPort
    log.Printf("Server running at %s", addr)
    if err := router.Run(addr); err != nil {
        log.Fatalf("Failed to start server: %v", err)
    }
}
```

### 2. Configuration (config/config.go)

```go
package config

import (
    "os"
    "strings"
)

type Config struct {
    // Server
    ServerHost string
    ServerPort string
    ServerURL  string
    Debug      bool
    Secret     string

    // Aggregator
    AggregatorBaseURL   string
    AggregatorClientID  string
    AggregatorSecretKey string

    // Database
    DatabaseURL string

    // Currencies
    Currencies []string

    // PSP Configuration
    LencoAPIKey    string
    LencoAccountID string
    LencoBaseURL   string

    // Limits
    MaxOrderAmount float64
    MinOrderAmount float64
}

func Load() *Config {
    return &Config{
        ServerHost: getEnv("SERVER_HOST", "0.0.0.0"),
        ServerPort: getEnv("SERVER_PORT", "8000"),
        ServerURL:  getEnv("SERVER_URL", "http://localhost:8000"),
        Debug:      getEnv("DEBUG", "false") == "true",
        Secret:     getEnv("SECRET", ""),

        AggregatorBaseURL:   getEnv("AGGREGATOR_BASE_URL", ""),
        AggregatorClientID:  getEnv("AGGREGATOR_CLIENT_ID", ""),
        AggregatorSecretKey: getEnv("AGGREGATOR_SECRET_KEY", ""),

        DatabaseURL: getEnv("DATABASE_URL", "provision_node.db"),

        Currencies: strings.Split(getEnv("CURRENCIES", "NGN"), ","),

        LencoAPIKey:    getEnv("LENCO_API_KEY", ""),
        LencoAccountID: getEnv("LENCO_ACCOUNT_ID", ""),
        LencoBaseURL:   getEnv("LENCO_BASE_URL", "https://api.lenco.ng/access/v1"),

        MaxOrderAmount: 50000,
        MinOrderAmount: 0.5,
    }
}

func getEnv(key, defaultValue string) string {
    if value := os.Getenv(key); value != "" {
        return value
    }
    return defaultValue
}
```

### 3. Database Models (models/order.go)

```go
package models

import (
    "time"

    "gorm.io/gorm"
)

type OrderStatus string

const (
    OrderStatusPending    OrderStatus = "pending"
    OrderStatusAccepted   OrderStatus = "accepted"
    OrderStatusProcessing OrderStatus = "processing"
    OrderStatusFulfilled  OrderStatus = "fulfilled"
    OrderStatusDeclined   OrderStatus = "declined"
    OrderStatusCancelled  OrderStatus = "cancelled"
    OrderStatusFailed     OrderStatus = "failed"
)

type Order struct {
    ID                  string      `gorm:"primaryKey"`
    AggregatorOrderID   string      `gorm:"uniqueIndex;not null"`
    Amount              float64     `gorm:"not null"`
    Currency            string      `gorm:"not null"`
    Token               string      `gorm:"not null"`
    TokenAmount         float64     `gorm:"not null"`
    Network             string      `gorm:"not null"`
    RecipientInstitution string     `gorm:"not null"`
    RecipientAccount    string      `gorm:"not null"`
    RecipientName       string      `gorm:"not null"`
    RecipientMemo       string
    Reference           string
    Status              OrderStatus `gorm:"not null;index"`
    PSPTransactionID    string
    ErrorMessage        string
    CreatedAt           time.Time
    UpdatedAt           time.Time
    ExpiresAt           *time.Time
}

type Balance struct {
    ID           uint      `gorm:"primaryKey"`
    Currency     string    `gorm:"uniqueIndex;not null"`
    Available    float64   `gorm:"not null;default:0"`
    Total        float64   `gorm:"not null;default:0"`
    Reserved     float64   `gorm:"not null;default:0"`
    LastSyncedAt time.Time
}

type Transaction struct {
    ID               uint      `gorm:"primaryKey"`
    OrderID          string    `gorm:"not null;index"`
    PSPTransactionID string
    Amount           float64   `gorm:"not null"`
    Currency         string    `gorm:"not null"`
    Status           string    `gorm:"not null"`
    PSPResponse      string    `gorm:"type:text"`
    CreatedAt        time.Time
}

func InitDB(dbURL string) (*gorm.DB, error) {
    // Use SQLite for development
    db, err := gorm.Open(sqlite.Open(dbURL), &gorm.Config{})
    if err != nil {
        return nil, err
    }
    return db, nil
}

func Migrate(db *gorm.DB) error {
    return db.AutoMigrate(&Order{}, &Balance{}, &Transaction{})
}
```

### 4. HMAC Authentication (middleware/auth.go)

```go
package middleware

import (
    "crypto/hmac"
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "net/http"
    "strings"
    "time"

    "github.com/gin-gonic/gin"
)

func HMACAuth(secretKey string) gin.HandlerFunc {
    return func(c *gin.Context) {
        // Get Authorization header
        authHeader := c.GetHeader("Authorization")
        if authHeader == "" {
            c.JSON(http.StatusUnauthorized, gin.H{
                "status":  "error",
                "message": "Missing Authorization header",
            })
            c.Abort()
            return
        }

        // Parse "HMAC client_id:signature"
        parts := strings.SplitN(authHeader, " ", 2)
        if len(parts) != 2 || parts[0] != "HMAC" {
            c.JSON(http.StatusUnauthorized, gin.H{
                "status":  "error",
                "message": "Invalid Authorization header format",
            })
            c.Abort()
            return
        }

        authParts := strings.SplitN(parts[1], ":", 2)
        if len(authParts) != 2 {
            c.JSON(http.StatusUnauthorized, gin.H{
                "status":  "error",
                "message": "Invalid Authorization header format",
            })
            c.Abort()
            return
        }

        clientID, signature := authParts[0], authParts[1]

        // Get request body
        var payload map[string]interface{}
        if err := c.ShouldBindJSON(&payload); err != nil {
            c.JSON(http.StatusBadRequest, gin.H{
                "status":  "error",
                "message": "Invalid JSON payload",
            })
            c.Abort()
            return
        }

        // Verify timestamp
        timestamp, ok := payload["timestamp"].(float64)
        if !ok {
            c.JSON(http.StatusUnauthorized, gin.H{
                "status":  "error",
                "message": "Missing or invalid timestamp",
            })
            c.Abort()
            return
        }

        // Check timestamp is within 5 minutes
        now := time.Now().Unix()
        if now-int64(timestamp) > 300 {
            c.JSON(http.StatusUnauthorized, gin.H{
                "status":  "error",
                "message": "Timestamp expired",
            })
            c.Abort()
            return
        }

        // Verify signature
        if !verifySignature(payload, signature, secretKey) {
            c.JSON(http.StatusUnauthorized, gin.H{
                "status":  "error",
                "message": "Invalid signature",
            })
            c.Abort()
            return
        }

        // Store client ID in context
        c.Set("client_id", clientID)

        // Remove timestamp from payload for handlers
        delete(payload, "timestamp")
        c.Set("payload", payload)

        c.Next()
    }
}

func verifySignature(payload map[string]interface{}, signature, secretKey string) bool {
    // Convert payload to JSON
    payloadJSON, err := json.Marshal(payload)
    if err != nil {
        return false
    }

    // Generate expected signature
    mac := hmac.New(sha256.New, []byte(secretKey))
    mac.Write(payloadJSON)
    expectedSignature := hex.EncodeToString(mac.Sum(nil))

    // Compare signatures
    return hmac.Equal([]byte(signature), []byte(expectedSignature))
}
```

### 5. Aggregator Client (services/aggregator.go)

```go
package services

import (
    "bytes"
    "crypto/hmac"
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "time"

    log "github.com/sirupsen/logrus"
)

type AggregatorClient struct {
    BaseURL   string
    ClientID  string
    SecretKey string
    client    *http.Client
}

func NewAggregatorClient(baseURL, clientID, secretKey string) *AggregatorClient {
    return &AggregatorClient{
        BaseURL:   baseURL,
        ClientID:  clientID,
        SecretKey: secretKey,
        client: &http.Client{
            Timeout: 30 * time.Second,
        },
    }
}

func (a *AggregatorClient) AcceptOrder(orderID string) error {
    endpoint := fmt.Sprintf("%s/v1/provider/orders/%s/accept", a.BaseURL, orderID)
    payload := map[string]interface{}{
        "timestamp": time.Now().Unix(),
    }
    return a.makeRequest("POST", endpoint, payload)
}

func (a *AggregatorClient) FulfillOrder(orderID, txHash string) error {
    endpoint := fmt.Sprintf("%s/v1/provider/orders/%s/fulfill", a.BaseURL, orderID)
    payload := map[string]interface{}{
        "transactionHash": txHash,
        "timestamp":       time.Now().Unix(),
    }
    return a.makeRequest("POST", endpoint, payload)
}

func (a *AggregatorClient) DeclineOrder(orderID, reason string) error {
    endpoint := fmt.Sprintf("%s/v1/provider/orders/%s/decline", a.BaseURL, orderID)
    payload := map[string]interface{}{
        "reason":    reason,
        "timestamp": time.Now().Unix(),
    }
    return a.makeRequest("POST", endpoint, payload)
}

func (a *AggregatorClient) CancelOrder(orderID, reason string) error {
    endpoint := fmt.Sprintf("%s/v1/provider/orders/%s/cancel", a.BaseURL, orderID)
    payload := map[string]interface{}{
        "reason":    reason,
        "timestamp": time.Now().Unix(),
    }
    return a.makeRequest("POST", endpoint, payload)
}

func (a *AggregatorClient) UpdateBalance(balances []map[string]interface{}) error {
    endpoint := fmt.Sprintf("%s/v1/provider/balances", a.BaseURL)
    payload := map[string]interface{}{
        "balances":  balances,
        "timestamp": time.Now().Unix(),
    }
    return a.makeRequest("POST", endpoint, payload)
}

func (a *AggregatorClient) makeRequest(method, url string, payload map[string]interface{}) error {
    // Generate HMAC signature
    signature := a.generateSignature(payload)

    // Marshal payload
    jsonData, err := json.Marshal(payload)
    if err != nil {
        return fmt.Errorf("failed to marshal payload: %w", err)
    }

    // Create request
    req, err := http.NewRequest(method, url, bytes.NewBuffer(jsonData))
    if err != nil {
        return fmt.Errorf("failed to create request: %w", err)
    }

    // Set headers
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Authorization", fmt.Sprintf("HMAC %s:%s", a.ClientID, signature))

    // Make request with retries
    var resp *http.Response
    for i := 0; i < 3; i++ {
        resp, err = a.client.Do(req)
        if err == nil && resp.StatusCode < 500 {
            break
        }
        time.Sleep(time.Duration(i+1) * time.Second)
    }

    if err != nil {
        return fmt.Errorf("request failed: %w", err)
    }
    defer resp.Body.Close()

    // Check response
    if resp.StatusCode >= 400 {
        body, _ := io.ReadAll(resp.Body)
        return fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
    }

    log.WithFields(log.Fields{
        "method": method,
        "url":    url,
        "status": resp.StatusCode,
    }).Info("Aggregator request successful")

    return nil
}

func (a *AggregatorClient) generateSignature(payload map[string]interface{}) string {
    // Convert to JSON
    jsonData, _ := json.Marshal(payload)

    // Generate HMAC-SHA256
    mac := hmac.New(sha256.New, []byte(a.SecretKey))
    mac.Write(jsonData)

    return hex.EncodeToString(mac.Sum(nil))
}
```

### 6. Order Handler (handlers/orders.go)

```go
package handlers

import (
    "net/http"

    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    "github.com/yourusername/provision-node/config"
    "github.com/yourusername/provision-node/models"
    "gorm.io/gorm"
)

func ReceiveOrder(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
    return func(c *gin.Context) {
        var req struct {
            OrderID   string  `json:"orderId" binding:"required"`
            Amount    float64 `json:"amount" binding:"required"`
            Currency  string  `json:"currency" binding:"required"`
            Token     string  `json:"token" binding:"required"`
            TokenAmount float64 `json:"tokenAmount" binding:"required"`
            Network   string  `json:"network" binding:"required"`
            Recipient struct {
                Institution       string `json:"institution" binding:"required"`
                AccountIdentifier string `json:"accountIdentifier" binding:"required"`
                AccountName       string `json:"accountName" binding:"required"`
                Memo              string `json:"memo"`
            } `json:"recipient" binding:"required"`
            Reference string `json:"reference"`
        }

        if err := c.ShouldBindJSON(&req); err != nil {
            c.JSON(http.StatusBadRequest, gin.H{
                "status":  "error",
                "message": "Invalid request payload",
                "data":    nil,
            })
            return
        }

        // Create order
        order := models.Order{
            ID:                  uuid.New().String(),
            AggregatorOrderID:   req.OrderID,
            Amount:              req.Amount,
            Currency:            req.Currency,
            Token:               req.Token,
            TokenAmount:         req.TokenAmount,
            Network:             req.Network,
            RecipientInstitution: req.Recipient.Institution,
            RecipientAccount:    req.Recipient.AccountIdentifier,
            RecipientName:       req.Recipient.AccountName,
            RecipientMemo:       req.Recipient.Memo,
            Reference:           req.Reference,
            Status:              models.OrderStatusPending,
        }

        // Save to database
        if err := db.Create(&order).Error; err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{
                "status":  "error",
                "message": "Failed to save order",
                "data":    nil,
            })
            return
        }

        c.JSON(http.StatusOK, gin.H{
            "status":  "success",
            "message": "Order received",
            "data": gin.H{
                "orderId": order.ID,
                "status":  order.Status,
            },
        })
    }
}
```

## AI Prompt for Go Implementation

```
I'm building a provision node in Go that integrates with a payment aggregator. 

Tech Stack:
- Go 1.21+
- Gin web framework
- GORM for database
- SQLite (development) / PostgreSQL (production)
- gocron for background jobs

I need you to help me implement [specific milestone from PROVISION_NODE_MILESTONES.md].

Please follow Go best practices:
- Use proper error handling
- Include context for cancellation
- Use interfaces for testability
- Add comprehensive logging
- Include unit tests

Start with [specific component].
```

## Next Steps

1. ✅ **Use the Go guide above** as your starting point
2. ✅ **Follow PROVISION_NODE_MILESTONES.md** for development sequence
3. ✅ **Use PROVISION_NODE_PROMPT.md** but adapt prompts for Go
4. ✅ **Reference the code examples** in this guide
5. ✅ **Test incrementally** as you build each component

Go is the right choice for this project! 🚀
