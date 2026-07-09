package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"

	"tigerswap/api/internal/handlers"
	"tigerswap/api/internal/middleware"
	"tigerswap/api/internal/services"
)

// @title TigerSwap API
// @version 1.0
// @description High-performance DEX API Gateway
// @termsOfService http://swagger.io/terms/

// @contact.name TigerSwap Support
// @contact.url https://tigerswap.io
// @contact.email support@tigerswap.io

// @license.name MIT
// @license.url https://github.com/tigerswap/tigerswap/blob/main/LICENSE

func main() {
	// Load configuration
	loadConfig()

	// Initialize services
	redisClient := services.NewRedisClient()
	defer redisClient.Close()

	db := services.NewDatabase()
	defer db.Close()

	// Initialize handlers
	authHandler := handlers.NewAuthHandler()
	walletHandler := handlers.NewWalletHandler()
	marketHandler := handlers.NewMarketHandler()
	orderHandler := handlers.NewOrderHandler()
	swapHandler := handlers.NewSwapHandler()

	// Setup router
	router := setupRouter(authHandler, walletHandler, marketHandler, orderHandler, swapHandler)

	// Create server
	srv := &http.Server{
		Addr:         ":" + viper.GetString("server.port"),
		Handler:      router,
		ReadTimeout:  time.Duration(viper.GetInt("server.read_timeout")) * time.Second,
		WriteTimeout: time.Duration(viper.GetInt("server.write_timeout")) * time.Second,
		IdleTimeout:  time.Duration(viper.GetInt("server.idle_timeout")) * time.Second,
	}

	// Start server in goroutine
	go func() {
		log.Printf("TigerSwap API Server starting on port %s", viper.GetString("server.port"))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}

func loadConfig() {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("/etc/tigerswap/")

	// Set defaults
	viper.SetDefault("server.port", "8080")
	viper.SetDefault("server.read_timeout", 30)
	viper.SetDefault("server.write_timeout", 30)
	viper.SetDefault("server.idle_timeout", 60)

	viper.SetDefault("database.host", "localhost")
	viper.SetDefault("database.port", 5432)
	viper.SetDefault("database.name", "tigerswap")

	viper.SetDefault("redis.host", "localhost")
	viper.SetDefault("redis.port", 6379)

	viper.SetDefault("jwt.secret", "tigerswap-secret-key")
	viper.SetDefault("jwt.expiry", 24)

	if err := viper.ReadInConfig(); err != nil {
		log.Printf("Warning: Config file not found, using defaults: %v", err)
	}

	viper.AutomaticEnv()
}

func setupRouter(
	authHandler *handlers.AuthHandler,
	walletHandler *handlers.WalletHandler,
	marketHandler *handlers.MarketHandler,
	orderHandler *handlers.OrderHandler,
	swapHandler *handlers.SwapHandler,
) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()
	router.Use(gin.Logger())
	router.Use(gin.Recovery())
	router.Use(middleware.CORS())
	router.Use(middleware.RateLimiter())
	router.Use(middleware.RequestID())

	// Health check
	router.GET("/health", handlers.HealthCheck)

	// API v1
	v1 := router.Group("/api/v1")
	{
		// Public endpoints
		public := v1.Group("")
		{
			public.GET("/markets", marketHandler.ListMarkets)
			public.GET("/markets/:id", marketHandler.GetMarket)
			public.GET("/tickers", marketHandler.GetTickers)
			public.GET("/orderbook/:market", marketHandler.GetOrderBook)
			public.GET("/trades/:market", marketHandler.GetRecentTrades)
			public.GET("/klines", marketHandler.GetKLines)
			public.GET("/tokens", marketHandler.ListTokens)
			public.GET("/fee", marketHandler.GetFeeInfo)
		}

		// Auth endpoints
		auth := v1.Group("/auth")
		{
			auth.POST("/register", authHandler.Register)
			auth.POST("/login", authHandler.Login)
			auth.POST("/refresh", authHandler.RefreshToken)
			auth.POST("/logout", authHandler.Logout)
		}

		// Protected endpoints
		protected := v1.Group("")
		protected.Use(middleware.AuthRequired())
		{
			// Wallet endpoints
			wallet := protected.Group("/wallet")
			{
				wallet.GET("/balance", walletHandler.GetBalance)
				wallet.GET("/balances", walletHandler.GetAllBalances)
				wallet.POST("/transfer", walletHandler.Transfer)
				wallet.POST("/approve", walletHandler.Approve)
				wallet.GET("/allowances", walletHandler.GetAllowances)
			}

			// Order endpoints
			orders := protected.Group("/orders")
			{
				orders.POST("", orderHandler.CreateOrder)
				orders.GET("", orderHandler.ListOrders)
				orders.GET("/:id", orderHandler.GetOrder)
				orders.DELETE("/:id", orderHandler.CancelOrder)
				orders.DELETE("", orderHandler.CancelAllOrders)
				orders.POST("/:id/modify", orderHandler.ModifyOrder)
			}

			// Swap endpoints
			swap := protected.Group("/swap")
			{
				swap.POST("", swapHandler.Swap)
				swap.GET("/quote", swapHandler.GetQuote)
				swap.GET("/routes", swapHandler.GetRoutes)
				swap.POST("/approve", swapHandler.ApproveToken)
			}

			// Portfolio
			protected.GET("/portfolio", handlers.GetPortfolio)
			protected.GET("/history", handlers.GetTransactionHistory)
		}
	}

	// WebSocket
	router.GET("/ws", handlers.HandleWebSocket)

	// 404 handler
	router.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "Endpoint not found",
		})
	})

	return router
}
