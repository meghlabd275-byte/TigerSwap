// Package api_gateway provides the main API gateway for TigerSwap
// High-performance, distributed API gateway with rate limiting, caching, and monitoring
package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"tigerswap/backend/go/api_gateway/config"
	"tigerswap/backend/go/api_gateway/handlers"
	"tigerswap/backend/go/api_gateway/middleware"
	"tigerswap/backend/go/api_gateway/router"
)

// Server represents the API Gateway server
type Server struct {
	httpServer *http.Server
	redis      *redis.Client
	config     *config.Config
	started    atomic.Bool
	ready      atomic.Bool
}

// NewServer creates a new API Gateway server
func NewServer(cfg *config.Config) *Server {
	return &Server{
		config: cfg,
	}
}

// Initialize initializes the server components
func (s *Server) Initialize() error {
	log.Println("Initializing API Gateway...")

	// Initialize Redis client
	s.redis = redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%d", s.config.Redis.Host, s.config.Redis.Port),
		Password:     s.config.Redis.Password,
		DB:           s.config.Redis.DB,
		PoolSize:     s.config.Redis.PoolSize,
		MinIdleConns: s.config.Redis.MinIdleConns,
	})

	// Test Redis connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.redis.Ping(ctx).Err(); err != nil {
		log.Printf("Warning: Redis connection failed: %v", err)
	}

	// Set up Gin
	if s.config.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	return nil
}

// SetupRouter sets up the main router
func (s *Server) SetupRouter() *gin.Engine {
	r := gin.New()

	// Global middleware
	r.Use(gin.Recovery())
	r.Use(middleware.Logger())
	r.Use(middleware.RequestID())
	r.Use(middleware.Timeout(s.config.RequestTimeout))

	// CORS configuration
	r.Use(cors.New(cors.Config{
		AllowOrigins:     s.config.CORS.AllowOrigins,
		AllowMethods:     s.config.CORS.AllowMethods,
		AllowHeaders:     s.config.CORS.AllowHeaders,
		ExposeHeaders:    s.config.CORS.ExposeHeaders,
		AllowCredentials: s.config.CORS.AllowCredentials,
		MaxAge:           s.config.CORS.MaxAge,
	}))

	// Health check endpoints
	r.GET("/health", handlers.HealthCheck())
	r.GET("/ready", handlers.ReadyCheck(s.redis))

	// Rate limiting
	rateLimiter := middleware.NewRateLimiter(s.redis, s.config.RateLimit)

	// API v1 routes
	v1 := r.Group("/api/v1")
	v1.Use(rateLimiter.Middleware())
	{
		// Swap routes
		swapHandler := handlers.NewSwapHandler(s.config)
		v1.POST("/swap/quote", swapHandler.GetQuote)
		v1.POST("/swap/execute", swapHandler.ExecuteSwap)
		v1.GET("/swap/routes", swapHandler.GetRoutes)
		v1.POST("/swap/multihop", swapHandler.MultiHopSwap)

		// Price routes
		priceHandler := handlers.NewPriceHandler(s.config, s.redis)
		v1.GET("/prices", priceHandler.GetPrices)
		v1.GET("/prices/:symbol", priceHandler.GetPrice)
		v1.GET("/prices/:symbol/history", priceHandler.GetPriceHistory)

		// Token routes
		tokenHandler := handlers.NewTokenHandler(s.config)
		v1.GET("/tokens", tokenHandler.GetTokens)
		v1.GET("/tokens/:address", tokenHandler.GetToken)
		v1.GET("/tokens/search", tokenHandler.SearchTokens)

		// Pool routes
		poolHandler := handlers.NewPoolHandler(s.config)
		v1.GET("/pools", poolHandler.GetPools)
		v1.GET("/pools/:tokenA/:tokenB", poolHandler.GetPool)
		v1.GET("/pools/:tokenA/:tokenB/volume", poolHandler.GetPoolVolume)

		// Wallet routes
		walletHandler := handlers.NewWalletHandler(s.config)
		v1.POST("/wallet/create", walletHandler.CreateWallet)
		v1.POST("/wallet/import", walletHandler.ImportWallet)
		v1.GET("/wallet/:address/balance", walletHandler.GetBalance)
		v1.GET("/wallet/:address/transactions", walletHandler.GetTransactions)
		v1.POST("/wallet/transfer", walletHandler.Transfer)
		v1.POST("/wallet/approve", walletHandler.Approve)

		// Chain routes
		chainHandler := handlers.NewChainHandler(s.config)
		v1.GET("/chains", chainHandler.GetChains)
		v1.GET("/chains/:chainId", chainHandler.GetChain)
		v1.GET("/chains/:chainId/fees", chainHandler.GetGasFees)

		// Order routes
		orderHandler := handlers.NewOrderHandler(s.config)
		v1.POST("/orders", orderHandler.CreateOrder)
		v1.GET("/orders/:orderId", orderHandler.GetOrder)
		v1.PUT("/orders/:orderId", orderHandler.UpdateOrder)
		v1.DELETE("/orders/:orderId", orderHandler.CancelOrder)
		v1.GET("/orders/user/:userId", orderHandler.GetUserOrders)

		// Perpetual routes
		perpetualHandler := handlers.NewPerpetualHandler(s.config)
		v1.GET("/perpetual/positions", perpetualHandler.GetPositions)
		v1.POST("/perpetual/positions", perpetualHandler.OpenPosition)
		v1.PUT("/perpetual/positions/:positionId", perpetualHandler.UpdatePosition)
		v1.DELETE("/perpetual/positions/:positionId", perpetualHandler.ClosePosition)

		// Staking routes
		stakingHandler := handlers.NewStakingHandler(s.config)
		v1.GET("/staking/pools", stakingHandler.GetPools)
		v1.POST("/staking/stake", stakingHandler.Stake)
		v1.POST("/staking/unstake", stakingHandler.Unstake)
		v1.GET("/staking/rewards", stakingHandler.GetRewards)

		// Bridge routes
		bridgeHandler := handlers.NewBridgeHandler(s.config)
		v1.POST("/bridge/transfer", bridgeHandler.Transfer)
		v1.GET("/bridge/transfer/:txHash", bridgeHandler.GetTransferStatus)
		v1.GET("/bridge/chains", bridgeHandler.GetSupportedChains)

		// Farming routes
		farmingHandler := handlers.NewFarmingHandler(s.config)
		v1.GET("/farming/pools", farmingHandler.GetPools)
		v1.POST("/farming/deposit", farmingHandler.Deposit)
		v1.POST("/farming/withdraw", farmingHandler.Withdraw)
		v1.GET("/farming/rewards", farmingHandler.GetRewards)

		// Launchpad routes
		launchpadHandler := handlers.NewLaunchpadHandler(s.config)
		v1.GET("/launchpad/projects", launchpadHandler.GetProjects)
		v1.GET("/launchpad/projects/:id", launchpadHandler.GetProject)
		v1.POST("/launchpad/projects", launchpadHandler.CreateProject)
		v1.POST("/launchpad/contribute", launchpadHandler.Contribute)
	}

	// Admin routes (protected)
	admin := r.Group("/api/v1/admin")
	admin.Use(middleware.Auth(s.config))
	{
		adminHandler := handlers.NewAdminHandler(s.config)
		admin.GET("/stats", adminHandler.GetStats)
		admin.GET("/users", adminHandler.GetUsers)
		admin.GET("/users/:id", adminHandler.GetUser)
		admin.PUT("/users/:id", adminHandler.UpdateUser)
		admin.DELETE("/users/:id", adminHandler.DeleteUser)
		admin.GET("/fees", adminHandler.GetFeeSettings)
		admin.PUT("/fees", adminHandler.UpdateFeeSettings)
		admin.GET("/whitelist", adminHandler.GetWhitelist)
		admin.POST("/whitelist", adminHandler.AddToWhitelist)
		admin.DELETE("/whitelist/:address", adminHandler.RemoveFromWhitelist)
	}

	return r
}

// Start starts the API Gateway server
func (s *Server) Start() error {
	if !s.started.CompareAndSwap(false, true) {
		return fmt.Errorf("server already started")
	}

	router := s.SetupRouter()

	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  s.config.ReadTimeout,
		WriteTimeout: s.config.WriteTimeout,
		IdleTimeout:  s.config.IdleTimeout,
	}

	// TLS configuration
	if s.config.TLS.Enabled {
		cert, err := tls.LoadX509KeyPair(s.config.TLS.CertFile, s.config.TLS.KeyFile)
		if err != nil {
			return fmt.Errorf("failed to load TLS cert: %w", err)
		}
		s.httpServer.TLSConfig = &tls.Config{
			Certificates: []tls.Certificate{cert},
		}
	}

	// Start server
	go func() {
		log.Printf("Starting API Gateway on %s", addr)
		var err error
		if s.config.TLS.Enabled {
			err = s.httpServer.ListenAndServeTLS("", "")
		} else {
			err = s.httpServer.ListenAndServe()
		}
		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for server to be ready
	for i := 0; i < 10; i++ {
		time.Sleep(100 * time.Millisecond)
		resp, err := http.Get(fmt.Sprintf("http://%s/health", addr))
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				s.ready.Store(true)
				log.Println("API Gateway is ready")
				return nil
			}
		}
	}

	return fmt.Errorf("server failed to become ready")
}

// Stop gracefully stops the server
func (s *Server) Stop() error {
	if !s.started.Load() {
		return nil
	}

	log.Println("Shutting down API Gateway...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := s.httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shutdown server: %w", err)
	}

	if s.redis != nil {
		s.redis.Close()
	}

	log.Println("API Gateway stopped")
	return nil
}

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Create and start server
	server := NewServer(cfg)
	if err := server.Initialize(); err != nil {
		log.Fatalf("Failed to initialize server: %v", err)
	}

	if err := server.Start(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	// Graceful shutdown
	if err := server.Stop(); err != nil {
		log.Printf("Error during shutdown: %v", err)
	}
}
