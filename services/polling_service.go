package services

import (
	"context"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/shopspring/decimal"
	"github.com/spf13/viper"

	"github.com/NEDA-LABS/stablenode/ent"
	"github.com/NEDA-LABS/stablenode/ent/paymentorder"
	"github.com/NEDA-LABS/stablenode/storage"
	"github.com/NEDA-LABS/stablenode/utils"
	"github.com/NEDA-LABS/stablenode/utils/logger"
)

// PollingService handles periodic balance checking for receive addresses
// Acts as fallback when webhooks fail or are not available
type PollingService struct {
	interval       time.Duration
	minOrderAge    time.Duration // Only poll orders older than this
	stopChan       chan bool
	metrics        *PollingMetrics
	metricsMutex   sync.RWMutex
	balanceCache   *BalanceCache
}

// PollingMetrics tracks polling service performance
type PollingMetrics struct {
	OrdersChecked     int64
	PaymentsDetected  int64
	RPCCallsMade      int64
	ErrorsEncountered int64
	LastRunTime       time.Time
	AverageCheckTime  time.Duration
}

// BalanceCache caches balance results to reduce RPC calls
type BalanceCache struct {
	balances map[string]CachedBalance
	mutex    sync.RWMutex
	ttl      time.Duration
}

// CachedBalance represents a cached balance with timestamp
type CachedBalance struct {
	Amount    decimal.Decimal
	Timestamp time.Time
}

// NewPollingService creates a new polling service
func NewPollingService(interval time.Duration) *PollingService {
	minOrderAge := viper.GetDuration("POLLING_MIN_AGE")
	if minOrderAge == 0 {
		minOrderAge = 5 * time.Minute // Default: only poll orders > 5 minutes old
	}

	cacheTTL := viper.GetDuration("POLLING_CACHE_TTL")
	if cacheTTL == 0 {
		cacheTTL = 30 * time.Second // Default: cache for 30 seconds
	}

	return &PollingService{
		interval:    interval,
		minOrderAge: minOrderAge,
		stopChan:    make(chan bool),
		metrics: &PollingMetrics{
			LastRunTime: time.Now(),
		},
		balanceCache: &BalanceCache{
			balances: make(map[string]CachedBalance),
			ttl:      cacheTTL,
		},
	}
}

// Start begins the polling loop
func (s *PollingService) Start(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	// Start metrics reporting
	go s.reportMetrics()

	logger.WithFields(logger.Fields{
		"interval":    s.interval,
		"minOrderAge": s.minOrderAge,
	}).Infof("Starting polling service (fallback mode)")

	// Run immediately on start
	s.pollPendingOrders(ctx)

	for {
		select {
		case <-ticker.C:
			s.pollPendingOrders(ctx)
		case <-s.stopChan:
			logger.Infof("Stopping polling service")
			return
		case <-ctx.Done():
			logger.Infof("Context cancelled, stopping polling service")
			return
		}
	}
}

// Stop stops the polling service
func (s *PollingService) Stop() {
	close(s.stopChan)
}

// pollPendingOrders checks all pending orders for payments
func (s *PollingService) pollPendingOrders(ctx context.Context) {
	startTime := time.Now()

	// Only poll orders that:
	// 1. Are in 'initiated' status
	// 2. Are older than minOrderAge (webhook should have fired by then)
	// 3. Have a receive address
	cutoffTime := time.Now().Add(-s.minOrderAge)

	orders, err := storage.Client.PaymentOrder.
		Query().
		Where(
			paymentorder.StatusEQ(paymentorder.StatusInitiated),
			paymentorder.CreatedAtLT(cutoffTime),
			paymentorder.HasReceiveAddress(),
		).
		WithReceiveAddress().
		WithToken(func(q *ent.TokenQuery) {
			q.WithNetwork()
		}).
		All(ctx)

	if err != nil {
		logger.Errorf("Failed to fetch pending orders: %v", err)
		s.incrementErrors()
		return
	}

	if len(orders) == 0 {
		logger.Debugf("No pending orders to poll")
		return
	}

	logger.WithFields(logger.Fields{
		"count":      len(orders),
		"minAge":     s.minOrderAge,
		"cutoffTime": cutoffTime,
	}).Infof("Polling pending orders (fallback mode)")

	// Group orders by network for batch processing
	ordersByNetwork := s.groupOrdersByNetwork(orders)

	for _, networkOrders := range ordersByNetwork {
		s.pollNetworkOrders(ctx, networkOrders)
	}

	// Update metrics
	duration := time.Since(startTime)
	s.updateMetrics(len(orders), duration)

	logger.WithFields(logger.Fields{
		"ordersChecked": len(orders),
		"duration":      duration,
		"paymentsFound": s.metrics.PaymentsDetected,
	}).Infof("Polling cycle completed")
}

// groupOrdersByNetwork groups orders by network for efficient batch processing
func (s *PollingService) groupOrdersByNetwork(orders []*ent.PaymentOrder) map[int64][]*ent.PaymentOrder {
	grouped := make(map[int64][]*ent.PaymentOrder)

	for _, order := range orders {
		networkID := order.Edges.Token.Edges.Network.ChainID
		grouped[networkID] = append(grouped[networkID], order)
	}

	return grouped
}

// pollNetworkOrders polls all orders for a specific network
func (s *PollingService) pollNetworkOrders(ctx context.Context, orders []*ent.PaymentOrder) {
	if len(orders) == 0 {
		return
	}

	network := orders[0].Edges.Token.Edges.Network

	logger.WithFields(logger.Fields{
		"network": network.Identifier,
		"count":   len(orders),
	}).Debugf("Polling network orders")

	for _, order := range orders {
		s.checkOrderPayment(ctx, order)
	}
}

// checkOrderPayment checks if payment has been received for an order
func (s *PollingService) checkOrderPayment(ctx context.Context, order *ent.PaymentOrder) {
	receiveAddr := order.Edges.ReceiveAddress
	token := order.Edges.Token
	network := token.Edges.Network

	// Check if receive address is expired
	if time.Now().After(receiveAddr.ValidUntil) {
		logger.WithFields(logger.Fields{
			"OrderID": order.ID,
			"Address": receiveAddr.Address,
		}).Debugf("Receive address expired, skipping")
		return
	}

	// Check cache first
	cacheKey := fmt.Sprintf("%d:%s:%s", network.ChainID, token.ContractAddress, receiveAddr.Address)
	if cachedBalance, found := s.balanceCache.Get(cacheKey); found {
		s.processBalance(ctx, order, cachedBalance)
		return
	}

	// Get balance from blockchain
	balance, err := s.getTokenBalance(ctx, network.RPCEndpoint, receiveAddr.Address, token.ContractAddress, int(token.Decimals))
	if err != nil {
		logger.WithFields(logger.Fields{
			"OrderID": order.ID,
			"Address": receiveAddr.Address,
			"Error":   err,
		}).Errorf("Failed to get balance")
		s.incrementErrors()
		return
	}

	s.incrementRPCCalls()

	// Cache the result
	s.balanceCache.Set(cacheKey, balance)

	// Process the balance
	s.processBalance(ctx, order, balance)
}

// processBalance processes the balance and updates order if needed
func (s *PollingService) processBalance(ctx context.Context, order *ent.PaymentOrder, balance decimal.Decimal) {
	// Check if balance has changed
	if balance.GreaterThan(order.AmountPaid) {
		logger.WithFields(logger.Fields{
			"OrderID":     order.ID,
			"Address":     order.Edges.ReceiveAddress.Address,
			"OldBalance":  order.AmountPaid,
			"NewBalance":  balance,
			"Method":      "polling_fallback",
		}).Infof("💰 Payment detected via polling fallback")

		// Update order
		err := s.updateOrderPayment(ctx, order, balance)
		if err != nil {
			logger.Errorf("Failed to update order payment: %v", err)
			s.incrementErrors()
		} else {
			s.incrementPaymentsDetected()
		}
	}
}

// getTokenBalance gets the ERC-20 token balance for an address
func (s *PollingService) getTokenBalance(ctx context.Context, rpcURL, address, tokenContract string, decimals int) (decimal.Decimal, error) {
	// Build full RPC URL with API key from environment
	fullRPCURL := utils.BuildRPCURL(rpcURL)
	
	// Connect to RPC
	client, err := ethclient.Dial(fullRPCURL)
	if err != nil {
		return decimal.Zero, fmt.Errorf("failed to connect to RPC: %w", err)
	}
	defer client.Close()

	// ERC-20 balanceOf function signature: balanceOf(address) returns (uint256)
	// Function selector: 0x70a08231
	addressBytes := common.HexToAddress(address)
	data := common.Hex2Bytes("70a08231" + "000000000000000000000000" + addressBytes.Hex()[2:])

	// Prepare call message
	contractAddr := common.HexToAddress(tokenContract)
	msg := ethereum.CallMsg{
		To:   &contractAddr,
		Data: data,
	}

	// Call contract
	result, err := client.CallContract(ctx, msg, nil)
	if err != nil {
		return decimal.Zero, fmt.Errorf("failed to call contract: %w", err)
	}

	// Parse result
	balance := new(big.Int).SetBytes(result)
	balanceDecimal := decimal.NewFromBigInt(balance, 0)

	// Convert to human-readable amount
	divisor := decimal.NewFromInt(10).Pow(decimal.NewFromInt(int64(decimals)))
	humanBalance := balanceDecimal.Div(divisor)

	return humanBalance, nil
}

// updateOrderPayment updates the order with the new payment amount
func (s *PollingService) updateOrderPayment(ctx context.Context, order *ent.PaymentOrder, amount decimal.Decimal) error {
	// Update amount_paid
	_, err := order.Update().
		SetAmountPaid(amount).
		Save(ctx)

	if err != nil {
		return fmt.Errorf("failed to update order: %w", err)
	}

	// Check if payment is sufficient
	totalRequired := order.Amount.Add(order.SenderFee).Add(order.NetworkFee).Add(order.ProtocolFee)

	if amount.GreaterThanOrEqual(totalRequired) {
		logger.WithFields(logger.Fields{
			"OrderID":        order.ID,
			"AmountPaid":     amount,
			"AmountRequired": totalRequired,
			"Status":         "sufficient",
		}).Infof("✅ Payment sufficient, order ready for fulfillment")

		// TODO: Trigger order fulfillment
		// This should call the same logic as webhook handler
		// For now, the database trigger will handle status change
	} else {
		logger.WithFields(logger.Fields{
			"OrderID":        order.ID,
			"AmountPaid":     amount,
			"AmountRequired": totalRequired,
			"Shortfall":      totalRequired.Sub(amount),
		}).Warnf("⚠️  Payment insufficient, waiting for more")
	}

	return nil
}

// Balance cache methods

func (c *BalanceCache) Get(key string) (decimal.Decimal, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	cached, exists := c.balances[key]
	if !exists {
		return decimal.Zero, false
	}

	// Check if cache is still valid
	if time.Since(cached.Timestamp) > c.ttl {
		return decimal.Zero, false
	}

	return cached.Amount, true
}

func (c *BalanceCache) Set(key string, amount decimal.Decimal) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.balances[key] = CachedBalance{
		Amount:    amount,
		Timestamp: time.Now(),
	}
}

func (c *BalanceCache) Clear() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.balances = make(map[string]CachedBalance)
}

// Metrics methods

func (s *PollingService) incrementRPCCalls() {
	s.metricsMutex.Lock()
	defer s.metricsMutex.Unlock()
	s.metrics.RPCCallsMade++
}

func (s *PollingService) incrementPaymentsDetected() {
	s.metricsMutex.Lock()
	defer s.metricsMutex.Unlock()
	s.metrics.PaymentsDetected++
}

func (s *PollingService) incrementErrors() {
	s.metricsMutex.Lock()
	defer s.metricsMutex.Unlock()
	s.metrics.ErrorsEncountered++
}

func (s *PollingService) updateMetrics(ordersChecked int, duration time.Duration) {
	s.metricsMutex.Lock()
	defer s.metricsMutex.Unlock()

	s.metrics.OrdersChecked += int64(ordersChecked)
	s.metrics.LastRunTime = time.Now()
	s.metrics.AverageCheckTime = duration
}

func (s *PollingService) GetMetrics() PollingMetrics {
	s.metricsMutex.RLock()
	defer s.metricsMutex.RUnlock()
	return *s.metrics
}

// reportMetrics logs metrics periodically
func (s *PollingService) reportMetrics() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			metrics := s.GetMetrics()
			logger.WithFields(logger.Fields{
				"orders_checked":     metrics.OrdersChecked,
				"payments_detected":  metrics.PaymentsDetected,
				"rpc_calls":          metrics.RPCCallsMade,
				"errors":             metrics.ErrorsEncountered,
				"avg_check_time":     metrics.AverageCheckTime,
				"last_run":           metrics.LastRunTime,
			}).Infof("📊 Polling service metrics")
		case <-s.stopChan:
			return
		}
	}
}
