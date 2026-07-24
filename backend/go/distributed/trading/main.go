package main

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
	"golang.org/x/time/rate"
)

// ============== CONFIGURATION ==============

type Config struct {
	Port             string
	RedisHost        string
	RedisPort        int
	MaxConnections   int
	RequestsPerSecond int
	BurstSize        int
	WorkerPoolSize   int
}

var cfg = Config{
	Port:             getEnv("PORT", "9090"),
	RedisHost:        getEnv("REDIS_HOST", "localhost"),
	RedisPort:        6379,
	MaxConnections:    100000,
	RequestsPerSecond: 50000,
	BurstSize:        100000,
	WorkerPoolSize:    runtime.NumCPU() * 4,
}

// ============== HIGH-PERFORMANCE ORDER BOOK ==============

type Order struct {
	ID           uint64    `json:"id"`
	TraderID     uint64    `json:"trader_id"`
	PairID       uint64    `json:"pair_id"`
	Price        float64   `json:"price"`
	Quantity     float64   `json:"quantity"`
	Filled       float64   `json:"filled"`
	Side         string    `json:"side"`
	Type         string    `json:"type"`
	Status       string    `json:"status"`
	CreatedAt    int64     `json:"created_at"`
	ExpiresAt    int64     `json:"expires_at"`
}

type Trade struct {
	ID            uint64  `json:"id"`
	MakerOrderID  uint64  `json:"maker_order_id"`
	TakerOrderID  uint64  `json:"taker_order_id"`
	PairID        uint64  `json:"pair_id"`
	Price         float64 `json:"price"`
	Quantity      float64 `json:"quantity"`
	Fee           float64 `json:"fee"`
	Timestamp     int64   `json:"timestamp"`
}

type OrderBook struct {
	mu         sync.RWMutex
	orders     map[uint64]Order
	traderIdx  map[uint64][]uint64
	pairIdx    map[uint64][]uint64
	orderID    uint64
	tradeID    uint64
	stats      Stats
}

type Stats struct {
	TotalOrders     uint64    `json:"total_orders"`
	TotalTrades     uint64    `json:"total_trades"`
	TotalVolume     float64   `json:"total_volume"`
	MinLatencyNs    uint64    `json:"min_latency_ns"`
	MaxLatencyNs    uint64    `json:"max_latency_ns"`
	AvgLatencyNs    uint64    `json:"avg_latency_ns"`
	RequestsPerSec  float64   `json:"requests_per_sec"`
	LastUpdate      time.Time `json:"last_update"`
}

func NewOrderBook() *OrderBook {
	return &OrderBook{
		orders:    make(map[uint64]Order),
		traderIdx: make(map[uint64][]uint64),
		pairIdx:   make(map[uint64][]uint64),
		stats:     Stats{MinLatencyNs: math.MaxUint64},
	}
}

// SubmitOrder - Optimized for high throughput
func (ob *OrderBook) SubmitOrder(order Order) (uint64, error) {
	start := time.Now()
	
	ob.mu.Lock()
	
	ob.orderID++
	order.ID = ob.orderID
	order.Status = "open"
	order.CreatedAt = time.Now().UnixMilli()
	order.ExpiresAt = order.CreatedAt + 86400000
	order.Filled = 0
	
	ob.orders[order.ID] = order
	ob.traderIdx[order.TraderID] = append(ob.traderIdx[order.TraderID], order.ID)
	ob.pairIdx[order.PairID] = append(ob.pairIdx[order.PairID], order.ID)
	ob.stats.TotalOrders++
	
	ob.mu.Unlock()
	
	latency := time.Since(start).Nanoseconds()
	ob.updateLatency(latency)
	publishOrder(order)
	
	return order.ID, nil
}

func (ob *OrderBook) CancelOrder(orderID, traderID uint64) error {
	ob.mu.Lock()
	defer ob.mu.Unlock()
	
	order, exists := ob.orders[orderID]
	if !exists {
		return fmt.Errorf("order not found")
	}
	
	if order.TraderID != traderID {
		return fmt.Errorf("unauthorized")
	}
	
	order.Status = "cancelled"
	order.Filled = order.Quantity
	ob.orders[orderID] = order
	
	return nil
}

func (ob *OrderBook) GetOrder(orderID uint64) (Order, bool) {
	ob.mu.RLock()
	defer ob.mu.RUnlock()
	
	order, exists := ob.orders[orderID]
	return order, exists
}

func (ob *OrderBook) GetTraderOrders(traderID uint64) []Order {
	ob.mu.RLock()
	defer ob.mu.RUnlock()
	
	orderIDs, exists := ob.traderIdx[traderID]
	if !exists {
		return nil
	}
	
	orders := make([]Order, 0, len(orderIDs))
	for _, id := range orderIDs {
		if order, ok := ob.orders[id]; ok {
			orders = append(orders, order)
		}
	}
	
	return orders
}

func (ob *OrderBook) GetMarketData(pairID uint64) MarketData {
	ob.mu.RLock()
	defer ob.mu.RUnlock()
	
	orderIDs, exists := ob.pairIdx[pairID]
	if !exists {
		return MarketData{PairID: pairID}
	}
	
	var bestBid, bestAsk float64
	var bidQty, askQty float64
	
	for _, id := range orderIDs {
		order := ob.orders[id]
		if order.Status == "cancelled" || order.Status == "filled" {
			continue
		}
		
		if order.Side == "buy" && order.Price > bestBid {
			bestBid = order.Price
			bidQty = order.Quantity - order.Filled
		}
		if order.Side == "sell" && (bestAsk == 0 || order.Price < bestAsk) {
			bestAsk = order.Price
			askQty = order.Quantity - order.Filled
		}
	}
	
	spread := 0.0
	if bestBid > 0 && bestAsk > 0 {
		spread = bestAsk - bestBid
	}
	
	return MarketData{
		PairID:    pairID,
		BestBid:   bestBid,
		BestAsk:   bestAsk,
		BidQty:    bidQty,
		AskQty:    askQty,
		Spread:    spread,
		Timestamp: time.Now().UnixMilli(),
	}
}

func (ob *OrderBook) updateLatency(latencyNs int64) {
	atomic.AddUint64(&ob.stats.TotalOrders, 1)
	
	current := atomic.LoadUint64(&ob.stats.MinLatencyNs)
	for uint64(latencyNs) < current {
		atomic.CompareAndSwapUint64(&ob.stats.MinLatencyNs, current, uint64(latencyNs))
		current = atomic.LoadUint64(&ob.stats.MinLatencyNs)
		break
	}
	
	current = atomic.LoadUint64(&ob.stats.MaxLatencyNs)
	for uint64(latencyNs) > current {
		atomic.CompareAndSwapUint64(&ob.stats.MaxLatencyNs, current, uint64(latencyNs))
		current = atomic.LoadUint64(&ob.stats.MaxLatencyNs)
		break
	}
	
	orders := atomic.LoadUint64(&ob.stats.TotalOrders)
	avg := atomic.LoadUint64(&ob.stats.AvgLatencyNs)
	newAvg := (avg*(orders-1) + uint64(latencyNs)) / orders
	atomic.StoreUint64(&ob.stats.AvgLatencyNs, newAvg)
}

type MarketData struct {
	PairID    uint64  `json:"pair_id"`
	BestBid   float64 `json:"best_bid"`
	BestAsk   float64 `json:"best_ask"`
	BidQty    float64 `json:"bid_qty"`
	AskQty    float64 `json:"ask_qty"`
	Spread    float64 `json:"spread"`
	Trades    []Trade `json:"trades"`
	Timestamp int64   `json:"timestamp"`
}

var redisClient *redis.Client
var orderBook *OrderBook
var rateLimiter *rate.Limiter

func publishOrder(order Order) {
	if redisClient == nil {
		return
	}
	
	data, _ := json.Marshal(order)
	ctx := context.Background()
	redisClient.Publish(ctx, fmt.Sprintf("orders:%d", order.PairID), data)
	redisClient.ZAdd(ctx, fmt.Sprintf("orders:trader:%d", order.TraderID), redis.Z{
		Score:  float64(order.CreatedAt),
		Member: string(data),
	})
}

func setupRoutes(r *gin.Engine) {
	api := r.Group("/api/v2")
	{
		api.POST("/orders", submitOrderHandler)
		api.DELETE("/orders/:id", cancelOrderHandler)
		api.GET("/orders/:id", getOrderHandler)
		api.GET("/orders/trader/:trader_id", getTraderOrdersHandler)
		api.GET("/market/:pair_id", getMarketDataHandler)
		api.GET("/stats", getStatsHandler)
		api.GET("/health", healthHandler)
	}
	r.GET("/ws/:pair_id", wsHandler)
}

func submitOrderHandler(c *gin.Context) {
	if !rateLimiter.Allow() {
		c.JSON(429, gin.H{"error": "Rate limit exceeded"})
		return
	}
	
	var order Order
	if err := c.ShouldBindJSON(&order); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	
	orderID, err := orderBook.SubmitOrder(order)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(201, gin.H{"success": true, "order_id": orderID})
}

func cancelOrderHandler(c *gin.Context) {
	var req struct {
		TraderID uint64 `json:"trader_id" binding:"required"`
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	
	var orderID uint64
	fmt.Sscanf(c.Param("id"), "%d", &orderID)
	
	err := orderBook.CancelOrder(orderID, req.TraderID)
	if err != nil {
		c.JSON(404, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(200, gin.H{"success": true})
}

func getOrderHandler(c *gin.Context) {
	var orderID uint64
	fmt.Sscanf(c.Param("id"), "%d", &orderID)
	
	order, exists := orderBook.GetOrder(orderID)
	if !exists {
		c.JSON(404, gin.H{"error": "Order not found"})
		return
	}
	
	c.JSON(200, order)
}

func getTraderOrdersHandler(c *gin.Context) {
	var traderID uint64
	fmt.Sscanf(c.Param("trader_id"), "%d", &traderID)
	
	orders := orderBook.GetTraderOrders(traderID)
	c.JSON(200, gin.H{"orders": orders, "count": len(orders)})
}

func getMarketDataHandler(c *gin.Context) {
	var pairID uint64
	fmt.Sscanf(c.Param("pair_id"), "%d", &pairID)
	
	data := orderBook.GetMarketData(pairID)
	c.JSON(200, data)
}

func getStatsHandler(c *gin.Context) {
	stats := orderBook.stats
	stats.LastUpdate = time.Now()
	
	uptime := time.Since(stats.LastUpdate).Seconds()
	if uptime > 0 {
		stats.RequestsPerSec = float64(stats.TotalOrders) / uptime
	}
	
	c.JSON(200, stats)
}

func healthHandler(c *gin.Context) {
	c.JSON(200, gin.H{"status": "healthy", "timestamp": time.Now().UnixMilli()})
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  512,
	WriteBufferSize: 512,
	CheckOrigin: func(r *http.Request) bool { return true },
}

func wsHandler(c *gin.Context) {
	pairID := c.Param("pair_id")
	
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	
	var pair uint64
	fmt.Sscanf(pairID, "%d", &pair)
	
	pubsub := redisClient.Subscribe(context.Background(), fmt.Sprintf("orders:%d", pair))
	defer pubsub.Close()
	
	data := orderBook.GetMarketData(pair)
	conn.WriteJSON(gin.H{"type": "snapshot", "data": data})
	
	for {
		select {
		case msg := <-pubsub.Channel():
			if msg != nil {
				conn.WriteJSON(gin.H{"type": "update", "data": msg.Payload})
			}
		case <-time.After(50 * time.Millisecond):
			data := orderBook.GetMarketData(pair)
			conn.WriteJSON(gin.H{"type": "heartbeat", "data": data})
		}
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

func encodeOrder(order Order) []byte {
	buf := make([]byte, 64)
	binary.BigEndian.PutUint64(buf[0:8], order.ID)
	binary.BigEndian.PutUint64(buf[8:16], order.TraderID)
	binary.BigEndian.PutUint64(buf[16:24], order.PairID)
	binary.BigEndian.PutUint64(buf[24:32], math.Float64bits(order.Price))
	binary.BigEndian.PutUint64(buf[32:40], math.Float64bits(order.Quantity))
	binary.BigEndian.PutUint64(buf[40:48], math.Float64bits(order.Filled))
	binary.BigEndian.PutUint64(buf[48:56], uint64(order.CreatedAt))
	binary.BigEndian.PutUint64(buf[56:64], uint64(order.ExpiresAt))
	return buf
}

func main() {
	redisClient = redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.RedisHost, cfg.RedisPort),
		Password: getEnv("REDIS_PASSWORD", ""),
		DB:       0,
		PoolSize: 100,
	})
	defer redisClient.Close()
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := redisClient.Ping(ctx).Err(); err != nil {
		fmt.Printf("Warning: Redis not available: %v\n", err)
		redisClient = nil
	}
	cancel()
	
	orderBook = NewOrderBook()
	rateLimiter = rate.NewLimiter(rate.Limit(cfg.RequestsPerSecond), cfg.BurstSize)
	
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(gin.Logger())
	
	setupRoutes(r)
	
	fmt.Printf("=== TigerSwap Distributed Engine ===\n")
	fmt.Printf("Port: %s\n", cfg.Port)
	fmt.Printf("Workers: %d\n", cfg.WorkerPoolSize)
	fmt.Printf("Rate Limit: %d req/s\n", cfg.RequestsPerSecond)
	
	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	
	go func() {
		<-quit
		fmt.Println("\nShutting down...")
		srv.Shutdown(context.Background())
		os.Exit(0)
	}()
	
	fmt.Printf("Server starting on port %s\n", cfg.Port)
	srv.ListenAndServe()
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
