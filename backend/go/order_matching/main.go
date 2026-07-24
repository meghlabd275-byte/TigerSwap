package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
)

// ============== CONFIGURATION ==============

type Config struct {
	Port           string
	RedisURL        string
	PostgresURL     string
	MaxConnections  int
	RateLimit       int
}

var cfg = Config{
	Port:          getEnv("PORT", "8080"),
	RedisURL:      getEnv("REDIS_URL", "redis://localhost:6379"),
	PostgresURL:   getEnv("DATABASE_URL", ""),
	MaxConnections: 10000,
	RateLimit:      1000,
}

// ============== ORDER BOOK SERVICE ==============

type OrderBookService struct {
	mu         sync.RWMutex
	orderMap   map[uint64]Order
	traderMap  map[string][]uint64
	tradeCh    chan Trade
	stats      OrderBookStats
}

type Order struct {
	OrderID    uint64  `json:"order_id"`
	Trader     string  `json:"trader"`
	Pair       string  `json:"pair"`
	Side       string  `json:"side"`
	Type       string  `json:"type"`
	Price      float64 `json:"price"`
	Quantity   float64 `json:"quantity"`
	Filled     float64 `json:"filled"`
	Leaves     float64 `json:"leaves"`
	Status     string  `json:"status"`
	CreatedAt  int64   `json:"created_at"`
	ExpiresAt  int64   `json:"expires_at"`
}

type Trade struct {
	TradeID   uint64  `json:"trade_id"`
	OrderID   uint64  `json:"order_id"`
	Pair      string  `json:"pair"`
	Side      string  `json:"side"`
	Price     float64 `json:"price"`
	Quantity  float64 `json:"quantity"`
	Fee       float64 `json:"fee"`
	Timestamp int64   `json:"timestamp"`
}

type OrderBookStats struct {
	TotalOrders     uint64    `json:"total_orders"`
	TotalTrades     uint64    `json:"total_trades"`
	OrdersPerSecond float64   `json:"orders_per_second"`
	AvgLatencyUs    float64   `json:"avg_latency_us"`
	LastUpdate      time.Time `json:"last_update"`
}

type MarketData struct {
	Pair      string        `json:"pair"`
	Bids      []PriceLevel  `json:"bids"`
	Asks      []PriceLevel  `json:"asks"`
	Trades    []Trade       `json:"trades"`
	LastPrice float64       `json:"last_price"`
	Spread    float64       `json:"spread"`
	Volume24h float64       `json:"volume_24h"`
	High24h   float64       `json:"high_24h"`
	Low24h    float64       `json:"low_24h"`
}

type PriceLevel struct {
	Price    float64 `json:"price"`
	Quantity float64 `json:"quantity"`
	Orders   int     `json:"orders"`
}

var orderBook *OrderBookService
var redisClient *redis.Client

// ============== ORDER BOOK IMPLEMENTATION ==============

func NewOrderBookService() *OrderBookService {
	return &OrderBookService{
		orderMap: make(map[uint64]Order),
		traderMap: make(map[string][]uint64),
		tradeCh:   make(chan Trade, 10000),
		stats:     OrderBookStats{LastUpdate: time.Now()},
	}
}

// SubmitOrder - Ultra low latency order submission
func (s *OrderBookService) SubmitOrder(order Order) (uint64, error) {
	start := time.Now()
	
	// Generate order ID (in production, use distributed ID generator)
	orderID := generateOrderID()
	
	// Store in memory for O(1) access
	order.OrderID = orderID
	order.Status = "open"
	order.CreatedAt = time.Now().UnixMilli()
	order.ExpiresAt = order.CreatedAt + 86400000
	order.Leaves = order.Quantity
	
	s.mu.Lock()
	s.orderMap[orderID] = order
	s.traderMap[order.Trader] = append(s.traderMap[order.Trader], orderID)
	s.stats.TotalOrders++
	s.stats.LastUpdate = time.Now()
	s.mu.Unlock()
	
	// Match against existing orders (in production, use C++ order book)
	s.matchOrder(order)
	
	// Publish to Redis for distribution
	s.publishOrder(order)
	
	// Calculate latency
	latency := time.Since(start).Microseconds()
	s.mu.Lock()
	s.stats.AvgLatencyUs = (s.stats.AvgLatencyUs*float64(s.stats.TotalOrders-1) + float64(latency)) / float64(s.stats.TotalOrders)
	s.mu.Unlock()
	
	return orderID, nil
}

// matchOrder - Match incoming order against the book
func (s *OrderBookService) matchOrder(incoming Order) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// Get opposite side orders
	var oppositeOrders []Order
	for _, order := range s.orderMap {
		if order.Pair == incoming.Pair && order.Side != incoming.Side && order.Status == "open" {
			oppositeOrders = append(oppositeOrders, order)
		}
	}
	
	// Sort by price (best first)
	if incoming.Side == "buy" {
		// For buy orders, sort by descending price
		for i := 0; i < len(oppositeOrders)-1; i++ {
			for j := i + 1; j < len(oppositeOrders); j++ {
				if oppositeOrders[i].Price < oppositeOrders[j].Price {
					oppositeOrders[i], oppositeOrders[j] = oppositeOrders[j], oppositeOrders[i]
				}
			}
		}
	} else {
		// For sell orders, sort by ascending price
		for i := 0; i < len(oppositeOrders)-1; i++ {
			for j := i + 1; j < len(oppositeOrders); j++ {
				if oppositeOrders[i].Price > oppositeOrders[j].Price {
					oppositeOrders[i], oppositeOrders[j] = oppositeOrders[j], oppositeOrders[i]
				}
			}
		}
	}
	
	// Match
	for _, maker := range oppositeOrders {
		if incoming.Leaves <= 0 {
			break
		}
		
		// Check price
		if incoming.Side == "buy" && maker.Price > incoming.Price {
			break
		}
		if incoming.Side == "sell" && maker.Price < incoming.Price {
			break
		}
		
		// Execute trade
		tradeQty := min(incoming.Leaves, maker.Leaves)
		tradePrice := maker.Price
		
		// Update orders
		incoming.Filled += tradeQty
		incoming.Leaves -= tradeQty
		maker.Filled += tradeQty
		maker.Leaves -= tradeQty
		
		if maker.Leaves <= 0 {
			maker.Status = "filled"
		} else {
			maker.Status = "partially_filled"
		}
		
		s.orderMap[maker.OrderID] = maker
		
		// Create trade record
		trade := Trade{
			TradeID:   generateTradeID(),
			OrderID:   incoming.OrderID,
			Pair:      incoming.Pair,
			Side:      incoming.Side,
			Price:     tradePrice,
			Quantity:  tradeQty,
			Fee:       tradeQty * tradePrice * 0.003, // 0.3% fee
			Timestamp: time.Now().UnixMilli(),
		}
		
		s.stats.TotalTrades++
		
		// Publish trade
		s.publishTrade(trade)
	}
	
	// Update incoming order
	if incoming.Leaves > 0 {
		incoming.Status = "partially_filled"
	} else {
		incoming.Status = "filled"
	}
	s.orderMap[incoming.OrderID] = incoming
}

func (s *OrderBookService) CancelOrder(orderID uint64, trader string) error {
	s.mu.RLock()
	order, exists := s.orderMap[orderID]
	s.mu.RUnlock()
	
	if !exists {
		return fmt.Errorf("order not found")
	}
	
	if order.Trader != trader {
		return fmt.Errorf("unauthorized")
	}
	
	s.mu.Lock()
	order.Status = "cancelled"
	order.Leaves = 0
	s.orderMap[orderID] = order
	s.mu.Unlock()
	
	return nil
}

func (s *OrderBookService) GetOrder(orderID uint64) (Order, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	order, exists := s.orderMap[orderID]
	if !exists {
		return Order{}, fmt.Errorf("order not found")
	}
	
	return order, nil
}

func (s *OrderBookService) GetTraderOrders(trader string) []Order {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	orderIDs, exists := s.traderMap[trader]
	if !exists {
		return []Order{}
	}
	
	orders := make([]Order, 0, len(orderIDs))
	for _, id := range orderIDs {
		if order, ok := s.orderMap[id]; ok {
			orders = append(orders, order)
		}
	}
	
	return orders
}

func (s *OrderBookService) GetMarketData(pair string, limit int) MarketData {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	// Find best bid/ask
	var bestBid, bestAsk float64
	var bidQty, askQty float64
	
	for _, order := range s.orderMap {
		if order.Pair != pair || order.Status == "cancelled" || order.Status == "filled" {
			continue
		}
		
		if order.Side == "buy" && (bestBid == 0 || order.Price > bestBid) {
			bestBid = order.Price
			bidQty += order.Leaves
		}
		if order.Side == "sell" && (bestAsk == 0 || order.Price < bestAsk) {
			bestAsk = order.Price
			askQty += order.Leaves
		}
	}
	
	spread := 0.0
	if bestBid > 0 && bestAsk > 0 {
		spread = bestAsk - bestBid
	}
	
	return MarketData{
		Pair:      pair,
		Bids:      []PriceLevel{{Price: bestBid, Quantity: bidQty, Orders: 1}},
		Asks:      []PriceLevel{{Price: bestAsk, Quantity: askQty, Orders: 1}},
		LastPrice: bestBid,
		Spread:    spread,
	}
}

func (s *OrderBookService) GetStats() OrderBookStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	stats := s.stats
	stats.LastUpdate = time.Now()
	
	uptime := time.Since(stats.LastUpdate).Seconds()
	if uptime > 0 {
		stats.OrdersPerSecond = float64(stats.TotalOrders) / uptime
	}
	
	return stats
}

func (s *OrderBookService) publishOrder(order Order) {
	if redisClient == nil {
		return
	}
	
	data, _ := json.Marshal(order)
	redisClient.Publish(context.Background(), "orders:"+order.Pair, data)
}

func (s *OrderBookService) publishTrade(trade Trade) {
	if redisClient == nil {
		return
	}
	
	data, _ := json.Marshal(trade)
	redisClient.Publish(context.Background(), "trades:"+trade.Pair, data)
	
	// Also add to trade list
	redisClient.LPush(context.Background(), "trades:"+trade.Pair, data)
	redisClient.LTrim(context.Background(), "trades:"+trade.Pair, 0, 999) // Keep last 1000
}

// ============== HTTP HANDLERS ==============

func setupRoutes(r *gin.Engine) {
	api := r.Group("/api/v1")
	{
		api.POST("/orders", submitOrderHandler)
		api.DELETE("/orders/:id", cancelOrderHandler)
		api.GET("/orders/:id", getOrderHandler)
		api.GET("/orders/trader/:address", getTraderOrdersHandler)
		api.GET("/market/:pair", getMarketDataHandler)
		api.GET("/market/:pair/depth", getDepthHandler)
		api.GET("/market/:pair/trades", getTradesHandler)
		api.GET("/stats", getStatsHandler)
		api.GET("/health", healthHandler)
	}
	
	r.GET("/ws/:pair", wsHandler)
}

func submitOrderHandler(c *gin.Context) {
	var req struct {
		Trader   string  `json:"trader" binding:"required"`
		Pair     string  `json:"pair" binding:"required"`
		Side     string  `json:"side" binding:"required"`
		Type     string  `json:"type" binding:"required"`
		Price    float64 `json:"price"`
		Quantity float64 `json:"quantity" binding:"required"`
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	order := Order{
		Trader:   req.Trader,
		Pair:     req.Pair,
		Side:     req.Side,
		Type:     req.Type,
		Price:    req.Price,
		Quantity: req.Quantity,
		Filled:   0,
		Leaves:   req.Quantity,
	}
	
	orderID, err := orderBook.SubmitOrder(order)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusCreated, gin.H{
		"success":   true,
		"order_id":  orderID,
		"message":   "Order submitted successfully",
	})
}

func cancelOrderHandler(c *gin.Context) {
	orderID := c.Param("id")
	var req struct {
		Trader string `json:"trader" binding:"required"`
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	var id uint64
	fmt.Sscanf(orderID, "%d", &id)
	
	err := orderBook.CancelOrder(id, req.Trader)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Order cancelled"})
}

func getOrderHandler(c *gin.Context) {
	orderID := c.Param("id")
	
	var id uint64
	fmt.Sscanf(orderID, "%d", &id)
	
	order, err := orderBook.GetOrder(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, order)
}

func getTraderOrdersHandler(c *gin.Context) {
	trader := c.Param("address")
	
	orders := orderBook.GetTraderOrders(trader)
	c.JSON(http.StatusOK, gin.H{"orders": orders, "count": len(orders)})
}

func getMarketDataHandler(c *gin.Context) {
	pair := c.Param("pair")
	
	data := orderBook.GetMarketData(pair, 10)
	c.JSON(http.StatusOK, data)
}

func getDepthHandler(c *gin.Context) {
	pair := c.Param("pair")
	
	data := orderBook.GetMarketData(pair, 10)
	c.JSON(http.StatusOK, gin.H{
		"bids": data.Bids,
		"asks": data.Asks,
	})
}

func getTradesHandler(c *gin.Context) {
	pair := c.Param("pair")
	limit := 100
	
	if l := c.Query("limit"); l != "" {
		fmt.Sscanf(l, "%d", &limit)
	}
	
	if redisClient != nil {
		trades, _ := redisClient.LRange(context.Background(), "trades:"+pair, 0, int64(limit-1)).Result()
		c.JSON(http.StatusOK, gin.H{"trades": trades, "count": len(trades)})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"trades": []string{}, "count": 0})
}

func getStatsHandler(c *gin.Context) {
	stats := orderBook.GetStats()
	c.JSON(http.StatusOK, stats)
}

func healthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"timestamp": time.Now().Unix(),
	})
}

// ============== WEBSOCKET ==============

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func wsHandler(c *gin.Context) {
	pair := c.Param("pair")
	
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	
	var pubsub *redis.PubSub
	if redisClient != nil {
		pubsub = redisClient.Subscribe(context.Background(), "market:"+pair)
		defer pubsub.Close()
	}
	
	// Send initial data
	data := orderBook.GetMarketData(pair, 10)
	conn.WriteJSON(gin.H{"type": "snapshot", "data": data})
	
	// Handle updates
	for {
		select {
		case <-time.After(100 * time.Millisecond):
			data := orderBook.GetMarketData(pair, 10)
			conn.WriteJSON(gin.H{"type": "update", "data": data})
			
		case msg := <-pubsub.Channel():
			if msg != nil {
				conn.WriteJSON(gin.H{"type": "trade", "data": msg.Payload})
			}
		}
		
		// Read client message
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

// ============== RATE LIMITING ==============

type RateLimiter struct {
	requests map[string][]time.Time
	mu       sync.Mutex
	limit    int
	window   time.Duration
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		requests: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
	}
	
	go func() {
		for {
			time.Sleep(time.Minute)
			rl.cleanup()
		}
	}()
	
	return rl
}

func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	now := time.Now()
	
	var valid []time.Time
	for _, t := range rl.requests[key] {
		if now.Sub(t) < rl.window {
			valid = append(valid, t)
		}
	}
	
	if len(valid) >= rl.limit {
		rl.requests[key] = valid
		return false
	}
	
	rl.requests[key] = append(valid, now)
	return true
}

func (rl *RateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	now := time.Now()
	for key, requests := range rl.requests {
		var valid []time.Time
		for _, t := range requests {
			if now.Sub(t) < rl.window {
				valid = append(valid, t)
			}
		}
		
		if len(valid) == 0 {
			delete(rl.requests, key)
		} else {
			rl.requests[key] = valid
		}
	}
}

// ============== UTILITIES ==============

var orderIDCounter uint64
var tradeIDCounter uint64
var idMutex sync.Mutex

func generateOrderID() uint64 {
	idMutex.Lock()
	defer idMutex.Unlock()
	orderIDCounter++
	return uint64(time.Now().UnixMilli())<<32 | orderIDCounter
}

func generateTradeID() uint64 {
	idMutex.Lock()
	defer idMutex.Unlock()
	tradeIDCounter++
	return uint64(time.Now().UnixMilli())<<32 | tradeIDCounter
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// ============== MAIN ==============

func main() {
	// Initialize Redis
	redisClient = redis.NewClient(&redis.Options{
		Addr:     getEnv("REDIS_HOST", "localhost:6379"),
		Password: getEnv("REDIS_PASSWORD", ""),
		DB:       0,
	})
	defer redisClient.Close()
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	if err := redisClient.Ping(ctx).Err(); err != nil {
		fmt.Printf("Warning: Redis not available: %v\n", err)
		redisClient = nil
	}
	
	// Initialize order book service
	orderBook = NewOrderBookService()
	
	// Setup Gin
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(gin.Logger())
	
	// Rate limiting
	rateLimiter := NewRateLimit(cfg.RateLimit, time.Second)
	r.Use(func(c *gin.Context) {
		if !rateLimiter.Allow(c.ClientIP()) {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "Rate limit exceeded",
			})
			c.Abort()
			return
		}
		c.Next()
	})
	
	// Setup routes
	setupRoutes(r)
	
	// Start server
	fmt.Printf("TigerSwap Order Matching Engine starting on port %s\n", cfg.Port)
	fmt.Printf("Redis: %s\n", cfg.RedisURL)
	
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	
	go func() {
		<-quit
		fmt.Println("\nShutting down...")
		os.Exit(0)
	}()
	
	r.Run(":" + cfg.Port)
}
