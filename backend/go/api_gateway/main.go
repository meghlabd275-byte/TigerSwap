// Package api_gateway provides the main API gateway for TigerSwap
// High-performance, distributed API gateway with rate limiting, caching, and monitoring
package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
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
)

// Server represents the API Gateway server
type Server struct {
	httpServer *http.Server
	redis      *redis.Client
	config     *config.Config
	started    atomic.Bool
	ready      atomic.Bool
	// probeLeaf is the DER of the leaf certificate the listener loaded. The
	// readiness probe pins to it so the probe verifies the exact certificate
	// this process serves instead of disabling verification.
	probeLeaf []byte
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

	// Only a configured proxy may set the client IP through a forwarding header.
	// gin trusts every peer by default, so without this a caller could rotate
	// X-Forwarded-For and get a fresh rate-limit bucket per request.
	if err := r.SetTrustedProxies(s.config.TrustedProxies); err != nil {
		log.Printf("failed to set trusted proxies, trusting none: %v", err)
		if fallbackErr := r.SetTrustedProxies(nil); fallbackErr != nil {
			log.Printf("failed to disable trusted proxies: %v", fallbackErr)
		}
	}

	// Global middleware
	r.Use(gin.Recovery())
	r.Use(middleware.Logger())
	r.Use(middleware.RequestID())
	r.Use(middleware.Timeout(s.config.RequestTimeout))
	// Browser-facing hardening headers and a request-body cap, applied to
	// every route so no handler can be reached without them.
	r.Use(middleware.Secure())
	r.Use(middleware.BodyLimiter(1 << 20))

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

	// Handlers shared by the public and account-scoped route groups are built
	// once here so both groups can register their own routes against them.
	stakingHandler := handlers.NewStakingHandler(s.config)
	farmingHandler := handlers.NewFarmingHandler(s.config)
	launchpadHandler := handlers.NewLaunchpadHandler(s.config)
	bridgeHandler := handlers.NewBridgeHandler(s.config)

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

		// Chain routes
		chainHandler := handlers.NewChainHandler(s.config)
		v1.GET("/chains", chainHandler.GetChains)
		v1.GET("/chains/:chainId", chainHandler.GetChain)
		v1.GET("/chains/:chainId/fees", chainHandler.GetGasFees)

		// Public catalogues. Staking and farming pools, the launchpad listing and
		// the bridge chain list are read-only and need no identity.
		v1.GET("/staking/pools", stakingHandler.GetPools)

		v1.GET("/farming/pools", farmingHandler.GetPools)

		v1.GET("/launchpad/projects", launchpadHandler.GetProjects)
		v1.GET("/launchpad/projects/:id", launchpadHandler.GetProject)

		v1.GET("/bridge/chains", bridgeHandler.GetSupportedChains)
	}

	// Account-scoped routes. A valid bearer token is required so a handler can
	// resolve the caller from the session; the identity is never taken from the
	// path, the query string or the body.
	secured := v1.Group("")
	secured.Use(middleware.Auth(s.config))
	{
		// Wallet routes. Creating or importing a wallet is a per-account action;
		// balance and history are public chain reads but are kept behind the token
		// so the gateway never proxies an unauthenticated wallet call.
		walletHandler := handlers.NewWalletHandler(s.config)
		secured.POST("/wallet/create", walletHandler.CreateWallet)
		secured.POST("/wallet/import", walletHandler.ImportWallet)
		secured.GET("/wallet/:address/balance", walletHandler.GetBalance)
		secured.GET("/wallet/:address/transactions", walletHandler.GetTransactions)
		secured.POST("/wallet/transfer", walletHandler.Transfer)
		secured.POST("/wallet/approve", walletHandler.Approve)

		// Order routes
		orderHandler := handlers.NewOrderHandler(s.config)
		secured.POST("/orders", orderHandler.CreateOrder)
		secured.GET("/orders/:orderId", orderHandler.GetOrder)
		secured.PUT("/orders/:orderId", orderHandler.UpdateOrder)
		secured.DELETE("/orders/:orderId", orderHandler.CancelOrder)
		secured.GET("/orders/user/:userId", orderHandler.GetUserOrders)

		// Perpetual routes
		perpetualHandler := handlers.NewPerpetualHandler(s.config)
		secured.GET("/perpetual/positions", perpetualHandler.GetPositions)
		secured.POST("/perpetual/positions", perpetualHandler.OpenPosition)
		secured.PUT("/perpetual/positions/:positionId", perpetualHandler.UpdatePosition)
		secured.DELETE("/perpetual/positions/:positionId", perpetualHandler.ClosePosition)

		// Staking actions
		secured.POST("/staking/stake", stakingHandler.Stake)
		secured.POST("/staking/unstake", stakingHandler.Unstake)
		secured.GET("/staking/rewards", stakingHandler.GetRewards)

		// Bridge actions
		secured.POST("/bridge/transfer", bridgeHandler.Transfer)
		secured.GET("/bridge/transfer/:txHash", bridgeHandler.GetTransferStatus)

		// Farming actions
		secured.POST("/farming/deposit", farmingHandler.Deposit)
		secured.POST("/farming/withdraw", farmingHandler.Withdraw)
		secured.GET("/farming/rewards", farmingHandler.GetRewards)

		// Launchpad actions
		secured.POST("/launchpad/projects", launchpadHandler.CreateProject)
		secured.POST("/launchpad/contribute", launchpadHandler.Contribute)
	}

	// Admin routes (protected). The group requires both a valid token and an
	// admin role: a plain authenticated user must not reach these handlers.
	admin := r.Group("/api/v1/admin")
	admin.Use(middleware.Auth(s.config), middleware.AdminRequired())
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
			// Refuse anything below TLS 1.2 so a downgrade to an obsolete
			// protocol cannot be negotiated against the public API.
			MinVersion: tls.VersionTLS12,
		}
		// Record the DER of the leaf certificate so the readiness probe can pin to
		// the exact certificate this listener serves.
		if len(cert.Certificate) > 0 {
			s.probeLeaf = cert.Certificate[0]
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

	// Wait for server to be ready. The probe must speak the same scheme the
	// listener does, or a TLS-enabled server is reported as never ready.
	scheme := "http"
	probeClient := &http.Client{Timeout: 2 * time.Second}
	if s.config.TLS.Enabled {
		scheme = "https"
		// Pin to the leaf certificate this process just loaded. The probe talks
		// to the server over loopback, so pinning verifies the exact cert being
		// served without weakening verification or trusting the system roots.
		leaf := s.probeLeaf
		probeClient.Transport = &http.Transport{TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
			VerifyPeerCertificate: func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
				if len(rawCerts) == 0 {
					return fmt.Errorf("probe received no server certificate")
				}
				if len(leaf) == 0 || !bytes.Equal(rawCerts[0], leaf) {
					return fmt.Errorf("probe server certificate does not match the loaded certificate")
				}
				return nil
			},
		}}
	}
	for i := 0; i < 10; i++ {
		time.Sleep(100 * time.Millisecond)
		resp, err := probeClient.Get(fmt.Sprintf("%s://%s/health", scheme, addr))
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
