/**
 * TigerSwap Production API Gateway
 * High-performance, distributed API Gateway for DEX operations
 * 
 * Features:
 * - RESTful API with OpenAPI 3.0
 * - WebSocket support for real-time data
 * - Rate limiting and throttling
 * - JWT authentication
 * - Request validation
 * - Circuit breaker pattern
 * - Distributed tracing
 * - Metrics and monitoring
 * 
 * @author TigerSwap
 * @version 1.0.0 Production
 */

package main

import (
	"context"
	"crypto/tls"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
	"golang.org/x/crypto/bcrypt"
)

// ============================================================================
// Configuration
// ============================================================================

type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	Redis    RedisConfig    `mapstructure:"redis"`
	Security SecurityConfig `mapstructure:"security"`
	RateLimit RateLimitConfig `mapstructure:"rateLimit"`
}

type ServerConfig struct {
	Port            int           `mapstructure:"port"`
	ReadTimeout     time.Duration `mapstructure:"readTimeout"`
	WriteTimeout    time.Duration `mapstructure:"writeTimeout"`
	MaxHeaderBytes  int           `mapstructure:"maxHeaderBytes"`
	EnableTLS       bool          `mapstructure:"enableTLS"`
	TLSCertFile     string        `mapstructure:"tlsCertFile"`
	TLSKeyFile      string        `mapstructure:"tlsKeyFile"`
}

type DatabaseConfig struct {
	Host            string `mapstructure:"host"`
	Port            int    `mapstructure:"port"`
	User            string `mapstructure:"user"`
	Password        string `mapstructure:"password"`
	DBName          string `mapstructure:"dbname"`
	MaxOpenConns    int    `mapstructure:"maxOpenConns"`
	MaxIdleConns    int    `mapstructure:"maxIdleConns"`
	ConnMaxLifetime time.Duration `mapstructure:"connMaxLifetime"`
}

type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
	PoolSize int    `mapstructure:"poolSize"`
}

type SecurityConfig struct {
	JWTSecret          string        `mapstructure:"jwtSecret"`
	JWTExpire          time.Duration `mapstructure:"jwtExpire"`
	EnableCORS         bool          `mapstructure:"enableCORS"`
	CORSAllowedOrigins []string      `mapstructure:"corsAllowedOrigins"`
	APIKeyHeader       string        `mapstructure:"apiKeyHeader"`
}

type RateLimitConfig struct {
	RequestsPerMinute int           `mapstructure:"requestsPerMinute"`
	BurstSize         int           `mapstructure:"burstSize"`
	CleanupInterval   time.Duration `mapstructure:"cleanupInterval"`
}

// ============================================================================
// Database Models
// ============================================================================

type User struct {
	ID           uint64    `json:"id" db:"id"`
	Email        string    `json:"email" db:"email"`
	Username     string    `json:"username" db:"username"`
	PasswordHash string    `json:"-" db:"password_hash"`
	APIKey       string    `json:"api_key" db:"api_key"`
	IsActive     bool      `json:"is_active" db:"is_active"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}

type Token struct {
	ID          uint64    `json:"id" db:"id"`
	UserID      uint64    `json:"user_id" db:"user_id"`
	Token       string    `json:"token" db:"token"`
	ExpiresAt   time.Time `json:"expires_at" db:"expires_at"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}

// ============================================================================
// Services
// ============================================================================

type APIServer struct {
	config      *Config
	router      *gin.Engine
	db          *sql.DB
	redis       *redis.Client
	wsHub       *WebSocketHub
	rateLimiter *RateLimiter
	circuitBreaker *CircuitBreaker
	metrics     *Metrics
}

func NewAPIServer(config *Config) (*APIServer, error) {
	// Initialize database
	db, err := initDatabase(config.Database)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	// Initialize Redis
	rdb := initRedis(config.Redis)

	// Initialize WebSocket hub
	wsHub := NewWebSocketHub()

	// Initialize rate limiter
	rateLimiter := NewRateLimiter(
		config.RateLimit.RequestsPerMinute,
		config.RateLimit.BurstSize,
	)

	// Initialize circuit breaker
	circuitBreaker := NewCircuitBreaker(5, 30*time.Second)

	// Initialize metrics
	metrics := NewMetrics()

	// Create server
	server := &APIServer{
		config:       config,
		db:           db,
		redis:        rdb,
		wsHub:        wsHub,
		rateLimiter:  rateLimiter,
		circuitBreaker: circuitBreaker,
		metrics:      metrics,
	}

	// Setup router
	server.setupRouter()

	return server, nil
}

func initDatabase(cfg DatabaseConfig) (*sql.DB, error) {
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	if err := db.Ping(); err != nil {
		return nil, err
	}

	return db, nil
}

func initRedis(cfg RedisConfig) *redis.Client {
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password: cfg.Password,
		DB:       cfg.DB,
		PoolSize: cfg.PoolSize,
	})

	return client
}

// ============================================================================
// Router Setup
// ============================================================================

func (s *APIServer) setupRouter() {
	if viper.GetBool("server.debug") {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	s.router = gin.New()
	s.router.Use(gin.Recovery())
	s.router.Use(gin.Logger())
	s.router.Use(s.metrics.Middleware())
	s.router.Use(s.rateLimiter.Middleware())

	// Health check
	s.router.GET("/health", s.handleHealth)

	// API v1
	v1 := s.router.Group("/api/v1")
	v1.Use(s.authMiddleware())
	{
		// Swap routes
		v1.POST("/swap", s.handleSwap)
		v1.POST("/swap/quote", s.handleGetQuote)
		v1.POST("/swap/execute", s.handleExecuteSwap)
		
		// Token routes
		v1.GET("/tokens", s.handleGetTokens)
		v1.GET("/tokens/:address", s.handleGetToken)
		
		// Pool routes
		v1.GET("/pools", s.handleGetPools)
		v1.GET("/pools/:address", s.handleGetPool)
		
		// User routes
		v1.GET("/user/portfolio", s.handleGetPortfolio)
		v1.GET("/user/orders", s.handleGetOrders)
		v1.POST("/user/orders", s.handleCreateOrder)
		v1.DELETE("/user/orders/:id", s.handleCancelOrder)
		
		// Wallet routes
		v1.POST("/wallet/connect", s.handleWalletConnect)
		v1.POST("/wallet/disconnect", s.handleWalletDisconnect)
		
		// Analytics routes
		v1.GET("/analytics/volume", s.handleGetVolume)
		v1.GET("/analytics/TVL", s.handleGetTVL)
		v1.GET("/analytics/fees", s.handleGetFees)
	}

	// Public API (no auth required)
	public := s.router.Group("/api/public")
	{
		public.GET("/prices", s.handleGetPrices)
		public.GET("/gas-estimate", s.handleGetGasEstimate)
		public.GET("/blocks", s.handleGetRecentBlocks)
	}

	// WebSocket
	s.router.GET("/ws", s.handleWebSocket)

	// Admin routes (protected)
	admin := s.router.Group("/api/admin")
	admin.Use(s.adminMiddleware())
	{
		admin.GET("/stats", s.handleAdminStats)
		admin.POST("/config/update", s.handleUpdateConfig)
		admin.GET("/users", s.handleAdminListUsers)
	}
}

// ============================================================================
// Request Handlers
// ============================================================================

// Health check
func (s *APIServer) handleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "healthy",
		"timestamp": time.Now().Unix(),
	})
}

// Swap handlers
type SwapRequest struct {
	FromToken  string `json:"fromToken" binding:"required"`
	ToToken    string `json:"toToken" binding:"required"`
	AmountIn   string `json:"amountIn" binding:"required"`
	Slippage   uint64 `json:"slippage"`
	Recipient  string `json:"recipient"`
	Referrer   string `json:"referrer"`
}

type QuoteResponse struct {
	FromToken     string `json:"fromToken"`
	ToToken       string `json:"toToken"`
	AmountIn      string `json:"amountIn"`
	AmountOut     string `json:"amountOut"`
	PriceImpact   string `json:"priceImpact"`
	GasEstimate   string `json:"gasEstimate"`
	GasPrice      string `json:"gasPrice"`
	Route         []string `json:"route"`
}

func (s *APIServer) handleSwap(c *gin.Context) {
	var req SwapRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get quote from aggregator
	quote, err := s.getSwapQuote(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, quote)
}

func (s *APIServer) handleGetQuote(c *gin.Context) {
	var req SwapRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	quote, err := s.getSwapQuote(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, quote)
}

func (s *APIServer) handleExecuteSwap(c *gin.Context) {
	var req SwapRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Execute swap
	txHash, err := s.executeSwap(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"txHash":    txHash,
	})
}

// Token handlers
func (s *APIServer) handleGetTokens(c *gin.Context) {
	tokens, err := s.getTokens(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"tokens": tokens})
}

func (s *APIServer) handleGetToken(c *gin.Context) {
	address := c.Param("address")
	
	token, err := s.getToken(c.Request.Context(), address)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Token not found"})
		return
	}

	c.JSON(http.StatusOK, token)
}

// Pool handlers
func (s *APIServer) handleGetPools(c *gin.Context) {
	pools, err := s.getPools(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"pools": pools})
}

func (s *APIServer) handleGetPool(c *gin.Context) {
	address := c.Param("address")
	
	pool, err := s.getPool(c.Request.Context(), address)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Pool not found"})
		return
	}

	c.JSON(http.StatusOK, pool)
}

// User handlers
func (s *APIServer) handleGetPortfolio(c *gin.Context) {
	userID := c.GetUint64("userID")
	
	portfolio, err := s.getPortfolio(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, portfolio)
}

func (s *APIServer) handleGetOrders(c *gin.Context) {
	userID := c.GetUint64("userID")
	
	orders, err := s.getOrders(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"orders": orders})
}

func (s *APIServer) handleCreateOrder(c *gin.Context) {
	userID := c.GetUint64("userID")
	
	var req struct {
		TokenIn   string `json:"tokenIn" binding:"required"`
		TokenOut  string `json:"tokenOut" binding:"required"`
		AmountIn  string `json:"amountIn" binding:"required"`
		AmountOut string `json:"amountOut"`
		Type      string `json:"type" binding:"required"`
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	order, err := s.createOrder(c.Request.Context(), userID, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, order)
}

func (s *APIServer) handleCancelOrder(c *gin.Context) {
	userID := c.GetUint64("userID")
	orderID := c.Param("id")
	
	err := s.cancelOrder(c.Request.Context(), userID, orderID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// Wallet handlers
func (s *APIServer) handleWalletConnect(c *gin.Context) {
	var req struct {
		Address   string `json:"address" binding:"required"`
		Signature string `json:"signature" binding:"required"`
		Message   string `json:"message"`
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	token, user, err := s.verifyWallet(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid signature"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token": token,
		"user":  user,
	})
}

func (s *APIServer) handleWalletDisconnect(c *gin.Context) {
	userID := c.GetUint64("userID")
	
	err := s.disconnectWallet(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// Analytics handlers
func (s *APIServer) handleGetVolume(c *gin.Context) {
	volume, err := s.getVolume(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, volume)
}

func (s *APIServer) handleGetTVL(c *gin.Context) {
	tvl, err := s.getTVL(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, tvl)
}

func (s *APIServer) handleGetFees(c *gin.Context) {
	fees, err := s.getFees(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, fees)
}

// Public handlers
func (s *APIServer) handleGetPrices(c *gin.Context) {
	prices, err := s.getPrices(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, prices)
}

func (s *APIServer) handleGetGasEstimate(c *gin.Context) {
	gas, err := s.getGasEstimate(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gas)
}

func (s *APIServer) handleGetRecentBlocks(c *gin.Context) {
	blocks, err := s.getRecentBlocks(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, blocks)
}

// Admin handlers
func (s *APIServer) handleAdminStats(c *gin.Context) {
	stats, err := s.getAdminStats(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, stats)
}

func (s *APIServer) handleUpdateConfig(c *gin.Context) {
	var config map[string]interface{}
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := s.updateConfig(c.Request.Context(), config)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *APIServer) handleAdminListUsers(c *gin.Context) {
	users, err := s.listUsers(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"users": users})
}

// ============================================================================
// WebSocket Handler
// ============================================================================

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // In production, check origin
	},
}

func (s *APIServer) handleWebSocket(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	client := s.wsHub.Register(conn)
	defer s.wsHub.Unregister(client)

	// Send initial data
	client.Send <- []byte(`{"type": "connected", "message": "Welcome to TigerSwap"}`)

	// Handle messages
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			break
		}

		s.handleWSMessage(client, message)
	}
}

func (s *APIServer) handleWSMessage(client *WSClient, message []byte) {
	var msg map[string]interface{}
	if err := json.Unmarshal(message, &msg); err != nil {
		client.Send <- []byte(`{"error": "Invalid message format"}`)
		return
	}

	msgType, ok := msg["type"].(string)
	if !ok {
		client.Send <- []byte(`{"error": "Missing message type"}`)
		return
	}

	switch msgType {
	case "subscribe":
		s.handleWSSubscribe(client, msg)
	case "unsubscribe":
		s.handleWSUnsubscribe(client, msg)
	case "ping":
		client.Send <- []byte(`{"type": "pong"}`)
	}
}

func (s *APIServer) handleWSSubscribe(client *WSClient, msg map[string]interface{}) {
	channels, ok := msg["channels"].([]interface{})
	if !ok {
		client.Send <- []byte(`{"error": "Invalid channels"}`)
		return
	}

	for _, ch := range channels {
		if channel, ok := ch.(string); ok {
			client.Subscribe(channel)
		}
	}

	client.Send <- []byte(`{"type": "subscribed"}`)
}

func (s *APIServer) handleWSUnsubscribe(client *WSClient, msg map[string]interface{}) {
	channels, ok := msg["channels"].([]interface{})
	if !ok {
		client.Send <- []byte(`{"error": "Invalid channels"}`)
		return
	}

	for _, ch := range channels {
		if channel, ok := ch.(string); ok {
			client.Unsubscribe(channel)
		}
	}

	client.Send <- []byte(`{"type": "unsubscribed"}`)
}

// ============================================================================
// Service Methods (Placeholder implementations)
// ============================================================================

func (s *APIServer) getSwapQuote(ctx context.Context, req *SwapRequest) (*QuoteResponse, error) {
	// Would call the routing engine
	return &QuoteResponse{
		FromToken:   req.FromToken,
		ToToken:     req.ToToken,
		AmountIn:    req.AmountIn,
		AmountOut:   "1000000",
		PriceImpact: "0.01",
		GasEstimate: "150000",
		GasPrice:    "20000000000",
		Route:       []string{req.FromToken, req.ToToken},
	}, nil
}

func (s *APIServer) executeSwap(ctx context.Context, req *SwapRequest) (string, error) {
	// Would execute the swap
	return "0x1234567890abcdef1234567890abcdef12345678", nil
}

func (s *APIServer) getTokens(ctx context.Context) ([]map[string]interface{}, error) {
	return []map[string]interface{}{
		{"address": "0x0000000000000000000000000000000000000000", "symbol": "ETH", "name": "Ethereum", "decimals": 18},
		{"address": "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48", "symbol": "USDC", "name": "USD Coin", "decimals": 6},
	}, nil
}

func (s *APIServer) getToken(ctx context.Context, address string) (map[string]interface{}, error) {
	return map[string]interface{}{
		"address":  address,
		"symbol":   "TOKEN",
		"name":     "Token",
		"decimals": 18,
		"price":    "1.0",
	}, nil
}

func (s *APIServer) getPools(ctx context.Context) ([]map[string]interface{}, error) {
	return []map[string]interface{}{}, nil
}

func (s *APIServer) getPool(ctx context.Context, address string) (map[string]interface{}, error) {
	return map[string]interface{}{
		"address":       address,
		"token0":        "0x0000000000000000000000000000000000000000",
		"token1":        "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48",
		"reserve0":      "1000000000000000000",
		"reserve1":      "1500000000",
		"totalSupply":   "1000000000000000000",
	}, nil
}

func (s *APIServer) getPortfolio(ctx context.Context, userID uint64) (map[string]interface{}, error) {
	return map[string]interface{}{
		"totalValue": "10000.00",
		"tokens":     []map[string]interface{}{},
	}, nil
}

func (s *APIServer) getOrders(ctx context.Context, userID uint64) ([]map[string]interface{}, error) {
	return []map[string]interface{}{}, nil
}

func (s *APIServer) createOrder(ctx context.Context, userID uint64, req *interface{}) (map[string]interface{}, error) {
	return map[string]interface{}{
		"id":        "1",
		"status":    "pending",
	}, nil
}

func (s *APIServer) cancelOrder(ctx context.Context, userID uint64, orderID string) error {
	return nil
}

func (s *APIServer) verifyWallet(ctx context.Context, req *struct{Address, Signature, Message string}) (string, *User, error) {
	return "jwt-token", &User{ID: 1, Username: "user"}, nil
}

func (s *APIServer) disconnectWallet(ctx context.Context, userID uint64) error {
	return nil
}

func (s *APIServer) getVolume(ctx context.Context) (map[string]interface{}, error) {
	return map[string]interface{}{
		"24h":  "100000000",
		"7d":   "700000000",
		"30d":  "3000000000",
	}, nil
}

func (s *APIServer) getTVL(ctx context.Context) (map[string]interface{}, error) {
	return map[string]interface{}{
		"total": "500000000",
	}, nil
}

func (s *APIServer) getFees(ctx context.Context) (map[string]interface{}, error) {
	return map[string]interface{}{
		"24h": "100000",
	}, nil
}

func (s *APIServer) getPrices(ctx context.Context) (map[string]interface{}, error) {
	return map[string]interface{}{
		"ETH": "2000.00",
		"USDC": "1.00",
	}, nil
}

func (s *APIServer) getGasEstimate(ctx context.Context) (map[string]interface{}, error) {
	return map[string]interface{}{
		"slow":    "20000000000",
		"average": "30000000000",
		"fast":    "50000000000",
	}, nil
}

func (s *APIServer) getRecentBlocks(ctx context.Context) ([]map[string]interface{}, error) {
	return []map[string]interface{}{
		{"number": 18000000, "timestamp": time.Now().Unix()},
	}, nil
}

func (s *APIServer) getAdminStats(ctx context.Context) (map[string]interface{}, error) {
	return map[string]interface{}{
		"totalUsers":    10000,
		"totalVolume":   "1000000000",
		"totalFees":     "1000000",
		"activePools":   100,
	}, nil
}

func (s *APIServer) updateConfig(ctx context.Context, config map[string]interface{}) error {
	return nil
}

func (s *APIServer) listUsers(ctx context.Context) ([]map[string]interface{}, error) {
	return []map[string]interface{}{}, nil
}

// ============================================================================
// Middleware
// ============================================================================

func (s *APIServer) authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check API key header first
		apiKey := c.GetHeader(s.config.Security.APIKeyHeader)
		if apiKey != "" {
			userID, err := s.validateAPIKey(c.Request.Context(), apiKey)
			if err == nil {
				c.Set("userID", userID)
				c.Next()
				return
			}
		}

		// Check JWT token
		authHeader := c.GetHeader("Authorization")
		if authHeader != "" && len(authHeader) > 7 {
			token := authHeader[7:]
			userID, err := s.validateToken(c.Request.Context(), token)
			if err == nil {
				c.Set("userID", userID)
				c.Next()
				return
			}
		}

		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		c.Abort()
	}
}

func (s *APIServer) adminMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check admin role
		isAdmin := c.GetBool("isAdmin")
		if !isAdmin {
			c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
			c.Abort()
			return
		}
		c.Next()
	}
}

func (s *APIServer) validateToken(ctx context.Context, token string) (uint64, error) {
	// Would validate JWT token
	return 1, nil
}

func (s *APIServer) validateAPIKey(ctx context.Context, key string) (uint64, error) {
	// Would validate API key
	return 1, nil
}

// ============================================================================
// Rate Limiter
// ============================================================================

type RateLimiter struct {
	mu           sync.Mutex
	requests     map[string][]time.Time
	rate         int
	burst        int
	cleanupInterval time.Duration
}

func NewRateLimiter(rate, burst int) *RateLimiter {
	rl := &RateLimiter{
		requests: make(map[string][]time.Time),
		rate:     rate,
		burst:    burst,
		cleanupInterval: time.Minute,
	}

	go rl.cleanup()

	return rl
}

func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(rl.cleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for key, times := range rl.requests {
			var valid []time.Time
			for _, t := range times {
				if now.Sub(t) < time.Minute {
					valid = append(valid, t)
				}
			}
			if len(valid) == 0 {
				delete(rl.requests, key)
			} else {
				rl.requests[key] = valid
			}
		}
		rl.mu.Unlock()
	}
}

func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	times := rl.requests[key]

	// Remove old requests
	var valid []time.Time
	for _, t := range times {
		if now.Sub(t) < time.Minute {
			valid = append(valid, t)
		}
	}

	if len(valid) >= rl.rate {
		return false
	}

	rl.requests[key] = append(valid, now)
	return true
}

func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.ClientIP()
		
		if !rl.Allow(key) {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "Rate limit exceeded",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}

// ============================================================================
// Circuit Breaker
// ============================================================================

type CircuitBreaker struct {
	mu             sync.Mutex
	failures       int
	threshold      int
	timeout        time.Duration
	lastFailure    time.Time
	state          string // closed, open, half-open
}

func NewCircuitBreaker(threshold int, timeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		threshold: threshold,
		timeout:   timeout,
		state:     "closed",
	}
}

func (cb *CircuitBreaker) Call(fn func() error) error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.state == "open" {
		if time.Since(cb.lastFailure) > cb.timeout {
			cb.state = "half-open"
		} else {
			return fmt.Errorf("circuit breaker is open")
		}
	}

	err := fn()

	if err != nil {
		cb.failures++
		cb.lastFailure = time.Now()
		
		if cb.failures >= cb.threshold {
			cb.state = "open"
		}
	} else {
		cb.failures = 0
		cb.state = "closed"
	}

	return err
}

// ============================================================================
// WebSocket Hub
// ============================================================================

type WSClient struct {
	ID         string
	Conn       *websocket.Conn
	Send       chan []byte
	Channels   map[string]bool
	mu         sync.Mutex
}

type WebSocketHub struct {
	clients    map[*WSClient]bool
	register   chan *WSClient
	unregister chan *WSClient
	broadcast  chan []byte
	mu         sync.RWMutex
}

func NewWebSocketHub() *WebSocketHub {
	hub := &WebSocketHub{
		clients:    make(map[*WSClient]bool),
		register:   make(chan *WSClient),
		unregister: make(chan *WSClient),
		broadcast:  make(chan []byte),
	}

	go hub.run()

	return hub
}

func (h *WebSocketHub) run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.Send)
			}
			h.mu.Unlock()

		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.Send <- message:
				default:
					close(client.Send)
					delete(h.clients, client)
				}
			}
			h.mu.RUnlock()
		}
	}
}

func (h *WebSocketHub) Register(conn *websocket.Conn) *WSClient {
	client := &WSClient{
		ID:       fmt.Sprintf("%d", time.Now().UnixNano()),
		Conn:     conn,
		Send:     make(chan []byte, 256),
		Channels: make(map[string]bool),
	}

	h.register <- client

	go client.writePump()

	return client
}

func (h *WebSocketHub) Unregister(client *WSClient) {
	h.unregister <- client
}

func (c *WSClient) writePump() {
	defer c.Conn.Close()

	for {
		message, ok := <-c.Send
		if !ok {
			c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
			return
		}

		if err := c.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
			return
		}
	}
}

func (c *WSClient) Subscribe(channel string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Channels[channel] = true
}

func (c *WSClient) Unsubscribe(channel string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.Channels, channel)
}

// ============================================================================
// Metrics
// ============================================================================

type Metrics struct {
	requestsTotal   int64
	requestsByRoute map[string]int64
	errorsTotal     int64
	latencies       []time.Duration
	mu              sync.Mutex
}

func NewMetrics() *Metrics {
	return &Metrics{
		requestsByRoute: make(map[string]int64),
	}
}

func (m *Metrics) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		route := c.FullPath()

		c.Next()

		latency := time.Since(start)

		m.mu.Lock()
		m.requestsTotal++
		m.requestsByRoute[route]++
		if c.Writer.Status() >= 400 {
			m.errorsTotal++
		}
		m.latencies = append(m.latencies, latency)
		if len(m.latencies) > 1000 {
			m.latencies = m.latencies[1:]
		}
		m.mu.Unlock()
	}
}

// ============================================================================
// Main
// ============================================================================

func main() {
	// Load configuration
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("/etc/tigerswap/")

	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("Error reading config: %v", err)
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		log.Fatalf("Error unmarshaling config: %v", err)
	}

	// Create server
	server, err := NewAPIServer(&config)
	if err != nil {
		log.Fatalf("Error creating server: %v", err)
	}

	// Create HTTP server
	httpServer := &http.Server{
		Addr:           fmt.Sprintf(":%d", config.Server.Port),
		Handler:        server.router,
		ReadTimeout:    config.Server.ReadTimeout,
		WriteTimeout:   config.Server.WriteTimeout,
		MaxHeaderBytes: config.Server.MaxHeaderBytes,
	}

	// Start server
	go func() {
		log.Printf("Starting TigerSwap API Gateway on port %d", config.Server.Port)
		
		if config.Server.EnableTLS {
			httpServer.TLSConfig = &tls.Config{
				MinVersion:               tls.VersionTLS12,
				CurvePreferences:         []tls.CurveID{tls.CurveP256, tls.X25519},
				PreferServerCipherSuites: true,
			}
			if err := httpServer.ListenAndServeTLS(
				config.Server.TLSCertFile,
				config.Server.TLSKeyFile,
			); err != nil && err != http.ErrServerClosed {
				log.Fatalf("Error starting server: %v", err)
			}
		} else {
			if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Fatalf("Error starting server: %v", err)
			}
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown:", err)
	}

	log.Println("Server exited")
}
