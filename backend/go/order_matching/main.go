// Command order_matching serves TigerSwap's central limit order book over HTTP
// and WebSocket.
//
// All matching happens in the local orderbook engine; the service never calls
// an external venue. Listed pairs and their fee tiers come from the operator
// (ORDER_MATCHING_PAIRS), so a pair that is not listed cannot be traded.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"

	"tigerswap/backend/go/order_matching/orderbook"
)

// ============== CONFIGURATION ==============

type Config struct {
	Port           string
	RedisURL       string
	RateLimit      int
	AdminKey       string
	AllowedOrigins []string
}

func loadConfig() Config {
	return Config{
		Port:           getEnv("PORT", "8080"),
		RedisURL:       getEnv("REDIS_URL", "redis://localhost:6379"),
		RateLimit:      getEnvInt("RATE_LIMIT_PER_SECOND", 1000),
		AdminKey:       os.Getenv("ORDER_MATCHING_ADMIN_KEY"),
		AllowedOrigins: splitCSV(os.Getenv("ALLOWED_ORIGINS")),
	}
}

// ============== PAIR REGISTRY ==============

// pairSpec is the operator-supplied definition of a tradable pair. It is read
// from ORDER_MATCHING_PAIRS as a JSON array, so the listed set lives in
// configuration and not in the binary.
type pairSpec struct {
	Pair        string  `json:"pair"`
	MakerFeeBps int64   `json:"maker_fee_bps"`
	TakerFeeBps int64   `json:"taker_fee_bps"`
	MinQuantity float64 `json:"min_quantity"`
	MaxQuantity float64 `json:"max_quantity"`
}

// loadPairSpecs parses the operator's pair list. It fails closed: an unparsable
// list is an error rather than an empty registry, because silently listing
// nothing would look like a working venue.
func loadPairSpecs(raw string) ([]pairSpec, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, errors.New("ORDER_MATCHING_PAIRS is not set")
	}
	var specs []pairSpec
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&specs); err != nil {
		return nil, fmt.Errorf("ORDER_MATCHING_PAIRS is not valid JSON: %w", err)
	}
	if len(specs) == 0 {
		return nil, errors.New("ORDER_MATCHING_PAIRS lists no pairs")
	}
	seen := make(map[string]bool, len(specs))
	for i, s := range specs {
		if strings.TrimSpace(s.Pair) == "" {
			return nil, fmt.Errorf("ORDER_MATCHING_PAIRS[%d] has no pair name", i)
		}
		if seen[s.Pair] {
			return nil, fmt.Errorf("ORDER_MATCHING_PAIRS lists %q twice", s.Pair)
		}
		seen[s.Pair] = true
		if s.MakerFeeBps < 0 || s.TakerFeeBps < 0 {
			return nil, fmt.Errorf("ORDER_MATCHING_PAIRS[%d] has a negative fee", i)
		}
		if s.MinQuantity < 0 || (s.MaxQuantity > 0 && s.MaxQuantity < s.MinQuantity) {
			return nil, fmt.Errorf("ORDER_MATCHING_PAIRS[%d] has an invalid quantity range", i)
		}
	}
	return specs, nil
}

// ============== HTTP HANDLERS ==============

var engine = orderbook.NewEngine()

func submitOrderHandler(c *gin.Context) {
	var req struct {
		Trader   string  `json:"trader" binding:"required"`
		Pair     string  `json:"pair" binding:"required"`
		Side     string  `json:"side" binding:"required"`
		Type     string  `json:"type" binding:"required"`
		TIF      string  `json:"tif"`
		Price    float64 `json:"price"`
		Quantity float64 `json:"quantity" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid order request"})
		return
	}

	placed, trades, err := engine.Submit(orderbook.Order{
		Trader:   req.Trader,
		Pair:     req.Pair,
		Side:     req.Side,
		Type:     req.Type,
		TIF:      req.TIF,
		Price:    req.Price,
		Quantity: req.Quantity,
	})
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, orderbook.ErrUnknownPair) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error(), "order_id": placed.OrderID})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"order":   placed,
		"trades":  trades,
		"message": "order accepted",
	})
}

func cancelOrderHandler(c *gin.Context) {
	orderID, ok := parseOrderID(c.Param("id"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid order id"})
		return
	}

	var req struct {
		Trader string `json:"trader" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "trader is required to cancel an order"})
		return
	}

	cancelled, err := engine.Cancel(orderID, req.Trader)
	if err != nil {
		status := http.StatusBadRequest
		switch {
		case errors.Is(err, orderbook.ErrUnknownOrder):
			status = http.StatusNotFound
		case errors.Is(err, orderbook.ErrNotOwner):
			status = http.StatusForbidden
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "order": cancelled})
}

func getOrderHandler(c *gin.Context) {
	orderID, ok := parseOrderID(c.Param("id"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid order id"})
		return
	}

	order, err := engine.GetOrder(orderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, order)
}

func getTraderOrdersHandler(c *gin.Context) {
	trader := c.Param("address")
	orders := make([]orderbook.Order, 0)
	for _, pair := range engine.ListedPairs() {
		book, err := engine.Book(pair)
		if err != nil {
			continue
		}
		orders = append(orders, book.OrdersByTrader(trader)...)
	}
	c.JSON(http.StatusOK, gin.H{"orders": orders, "count": len(orders)})
}

func getMarketDataHandler(c *gin.Context) {
	pair, ok := requirePair(c)
	if !ok {
		return
	}
	book, err := engine.Book(pair)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	bids, asks := book.Depth(20)
	c.JSON(http.StatusOK, gin.H{
		"pair":        pair,
		"bids":        bids,
		"asks":        asks,
		"last_price":  book.LastPrice(),
		"best_bid":    book.BestBid(),
		"best_ask":    book.BestAsk(),
		"open_orders": book.OpenOrders(),
	})
}

func getDepthHandler(c *gin.Context) {
	pair, ok := requirePair(c)
	if !ok {
		return
	}
	limit := 20
	if l := c.Query("limit"); l != "" {
		fmt.Sscanf(l, "%d", &limit)
	}

	book, err := engine.Book(pair)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	bids, asks := book.Depth(limit)
	c.JSON(http.StatusOK, gin.H{"pair": pair, "bids": bids, "asks": asks})
}

func getTradesHandler(c *gin.Context) {
	pair, ok := requirePair(c)
	if !ok {
		return
	}
	limit := 100
	if l := c.Query("limit"); l != "" {
		fmt.Sscanf(l, "%d", &limit)
	}

	book, err := engine.Book(pair)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	trades := book.Trades(limit)
	c.JSON(http.StatusOK, gin.H{"trades": trades, "count": len(trades)})
}

func getStatsHandler(c *gin.Context) {
	c.JSON(http.StatusOK, engine.Stats())
}

func healthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"timestamp": time.Now().Unix(),
		"pairs":     len(engine.ListedPairs()),
	})
}

// ============== ADMIN PAIR MANAGEMENT ==============

// adminAuth requires the configured admin key. When no key is configured the
// admin surface is disabled entirely rather than left open.
func adminAuth(c *gin.Context) {
	if cfg.AdminKey == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "admin API is disabled: ORDER_MATCHING_ADMIN_KEY is not set"})
		c.Abort()
		return
	}
	provided := c.GetHeader("X-Admin-Key")
	if provided == "" {
		provided = strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
	}
	if !subtleCompare(provided, cfg.AdminKey) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid admin key"})
		c.Abort()
		return
	}
	c.Next()
}

func listPairsHandler(c *gin.Context) {
	pairs := engine.ListedPairs()
	c.JSON(http.StatusOK, gin.H{"pairs": pairs, "count": len(pairs)})
}

func registerPairHandler(c *gin.Context) {
	var spec pairSpec
	if err := c.ShouldBindJSON(&spec); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid pair spec"})
		return
	}
	if err := engine.RegisterPair(orderbook.PairConfig{
		Pair:        spec.Pair,
		MakerFeeBps: spec.MakerFeeBps,
		TakerFeeBps: spec.TakerFeeBps,
		MinQuantity: spec.MinQuantity,
		MaxQuantity: spec.MaxQuantity,
	}); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "pair": spec.Pair})
}

func unregisterPairHandler(c *gin.Context) {
	pair, ok := requirePair(c)
	if !ok {
		return
	}
	if !engine.UnregisterPair(pair) {
		c.JSON(http.StatusNotFound, gin.H{"error": "pair is not listed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "pair": pair})
}

// ============== WEBSOCKET ==============

func newUpgrader(allowed []string) websocket.Upgrader {
	allowAll := len(allowed) == 0
	set := make(map[string]bool, len(allowed))
	for _, o := range allowed {
		set[o] = true
	}
	return websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			if allowAll {
				// With no configured origins only same-host requests are
				// accepted, so a random page cannot open a socket to the venue.
				origin := r.Header.Get("Origin")
				if origin == "" {
					return true
				}
				return sameHost(origin, r.Host)
			}
			return set[r.Header.Get("Origin")]
		},
	}
}

func wsHandler(upgrader websocket.Upgrader) gin.HandlerFunc {
	return func(c *gin.Context) {
		pair, ok := requirePair(c)
		if !ok {
			return
		}
		book, err := engine.Book(pair)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}

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

		done := make(chan struct{})
		var once sync.Once
		closeDone := func() { once.Do(func() { close(done) }) }

		// Reader: detect the client going away.
		go func() {
			defer closeDone()
			for {
				if _, _, err := conn.ReadMessage(); err != nil {
					return
				}
			}
		}()

		write := func(v any) error {
			conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			return conn.WriteJSON(v)
		}

		bids, asks := book.Depth(20)
		if err := write(gin.H{"type": "snapshot", "data": gin.H{
			"pair": pair, "bids": bids, "asks": asks,
			"last_price": book.LastPrice(),
		}}); err != nil {
			return
		}

		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()

		var pubsubCh <-chan *redis.Message
		if pubsub != nil {
			pubsubCh = pubsub.Channel()
		}

		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				bids, asks := book.Depth(20)
				if err := write(gin.H{"type": "update", "data": gin.H{
					"pair": pair, "bids": bids, "asks": asks,
					"last_price": book.LastPrice(),
				}}); err != nil {
					return
				}
			case msg, ok := <-pubsubCh:
				if !ok {
					pubsubCh = nil
					continue
				}
				if err := write(gin.H{"type": "trade", "data": json.RawMessage(msg.Payload)}); err != nil {
					return
				}
			}
		}
	}
}

// ============== RATE LIMITING ==============

type RateLimiter struct {
	requests map[string][]time.Time
	mu       sync.Mutex
	limit    int
	window   time.Duration
	stop     chan struct{}
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		requests: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
		stop:     make(chan struct{}),
	}
	go rl.cleanupLoop()
	return rl
}

func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	valid := rl.requests[key][:0]
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

func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-rl.stop:
			return
		case <-ticker.C:
			rl.cleanup()
		}
	}
}

func (rl *RateLimiter) Stop() { close(rl.stop) }

func (rl *RateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	for key, requests := range rl.requests {
		valid := requests[:0]
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

// requirePair reads the pair from the query string. Pairs contain a slash
// ("BTC/RUSD"), so they cannot travel as a path parameter; the request is
// answered with 400 when the parameter is missing.
func requirePair(c *gin.Context) (string, bool) {
	pair := strings.TrimSpace(c.Query("pair"))
	if pair == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "pair query parameter is required"})
		return "", false
	}
	return pair, true
}

func parseOrderID(raw string) (uint64, bool) {
	var id uint64
	if _, err := fmt.Sscanf(raw, "%d", &id); err != nil || id == 0 {
		return 0, false
	}
	return id, true
}

func sameHost(origin, host string) bool {
	origin = strings.TrimPrefix(strings.TrimPrefix(origin, "http://"), "https://")
	origin = strings.TrimSuffix(origin, "/")
	return strings.EqualFold(origin, host)
}

// subtleCompare compares two secrets without leaking content through timing.
func subtleCompare(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var diff byte
	for i := 0; i < len(a); i++ {
		diff |= a[i] ^ b[i]
	}
	return diff == 0
}

func splitCSV(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		var parsed int
		if _, err := fmt.Sscanf(value, "%d", &parsed); err == nil && parsed > 0 {
			return parsed
		}
	}
	return defaultValue
}

// ============== MAIN ==============

var (
	cfg         Config
	redisClient *redis.Client
)

func main() {
	cfg = loadConfig()

	specs, err := loadPairSpecs(os.Getenv("ORDER_MATCHING_PAIRS"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "order matching: %v\n", err)
		os.Exit(1)
	}
	for _, s := range specs {
		if err := engine.RegisterPair(orderbook.PairConfig{
			Pair:        s.Pair,
			MakerFeeBps: s.MakerFeeBps,
			TakerFeeBps: s.TakerFeeBps,
			MinQuantity: s.MinQuantity,
			MaxQuantity: s.MaxQuantity,
		}); err != nil {
			fmt.Fprintf(os.Stderr, "order matching: register %s: %v\n", s.Pair, err)
			os.Exit(1)
		}
	}

	// Redis is used only to fan trades out to subscribers; the engine does not
	// depend on it to match.
	redisClient = redis.NewClient(&redis.Options{
		Addr:     redisAddr(cfg.RedisURL),
		Password: getEnv("REDIS_PASSWORD", ""),
		DB:       0,
	})
	pingCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := redisClient.Ping(pingCtx).Err(); err != nil {
		fmt.Printf("Warning: Redis not available, trade fan-out disabled: %v\n", err)
		redisClient.Close()
		redisClient = nil
	}
	cancel()

	if redisClient != nil {
		engine.OnTrade(func(t orderbook.Trade) {
			data, err := json.Marshal(t)
			if err != nil {
				return
			}
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			pipe := redisClient.Pipeline()
			pipe.Publish(ctx, "market:"+t.Pair, data)
			pipe.LPush(ctx, "trades:"+t.Pair, data)
			pipe.LTrim(ctx, "trades:"+t.Pair, 0, 999)
			_, _ = pipe.Exec(ctx)
		})
	}

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())

	rateLimiter := NewRateLimiter(cfg.RateLimit, time.Second)
	defer rateLimiter.Stop()
	r.Use(func(c *gin.Context) {
		if !rateLimiter.Allow(c.ClientIP()) {
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
			c.Abort()
			return
		}
		c.Next()
	})

	api := r.Group("/api/v1")
	{
		api.POST("/orders", submitOrderHandler)
		api.DELETE("/orders/:id", cancelOrderHandler)
		api.GET("/orders/:id", getOrderHandler)
		api.GET("/orders/trader/:address", getTraderOrdersHandler)
		api.GET("/market", getMarketDataHandler)
		api.GET("/market/depth", getDepthHandler)
		api.GET("/market/trades", getTradesHandler)
		api.GET("/pairs", listPairsHandler)
		api.GET("/stats", getStatsHandler)
		api.GET("/health", healthHandler)

		admin := api.Group("/admin", adminAuth)
		{
			admin.POST("/pairs", registerPairHandler)
			admin.DELETE("/pairs", unregisterPairHandler)
		}
	}

	r.GET("/ws", wsHandler(newUpgrader(cfg.AllowedOrigins)))

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		fmt.Printf("TigerSwap order matching engine listening on port %s with %d listed pairs\n", cfg.Port, len(engine.ListedPairs()))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Fprintf(os.Stderr, "order matching: server error: %v\n", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	fmt.Println("Shutting down...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		fmt.Fprintf(os.Stderr, "order matching: shutdown error: %v\n", err)
	}
	if redisClient != nil {
		redisClient.Close()
	}
}

// redisAddr extracts host:port from a redis:// URL, falling back to the raw
// value so a plain host:port is also accepted.
func redisAddr(raw string) string {
	if strings.HasPrefix(raw, "redis://") {
		return strings.TrimPrefix(raw, "redis://")
	}
	return raw
}
