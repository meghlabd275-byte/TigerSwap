package main

import (
	"log"
	"os"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"tigerswap/backend/config"
	"tigerswap/backend/middleware"
	"tigerswap/backend/models"
	"tigerswap/backend/services"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	// Initialize database
	db, err := config.InitDB()
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// Auto migrate models
	if err := db.AutoMigrate(
		&models.User{},
		&models.Token{},
		&models.Blockchain{},
		&models.Pool{},
		&models.Transaction{},
		&models.Order{},
		&models.AdminLog{},
	); err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}

	// Initialize Redis
	redis := config.InitRedis()

	// Initialize services
	blockchainService := services.NewBlockchainService(db)
	tokenService := services.NewTokenService(db)
	swapService := services.NewSwapService(db, redis)
	authService := services.NewAuthService(db)
	adminService := services.NewAdminService(db)

	// Setup router
	router := gin.Default()

	// CORS configuration
	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Request-ID"},
		ExposeHeaders:    []string{"Content-Length", "X-Request-ID"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// Global middleware
	router.Use(middleware.RequestLogger())
	router.Use(middleware.RateLimiter(redis))
	router.Use(middleware.SecurityHeaders())

	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":    "healthy",
			"timestamp": time.Now().Unix(),
			"version":   "1.0.0",
		})
	})

	// API v1
	v1 := router.Group("/api/v1")
	{
		// Public routes
		public := v1.Group("")
		{
			// Blockchain routes
			public.GET("/chains", blockchainService.GetSupportedChains)
			public.GET("/chains/:id", blockchainService.GetChain)

			// Token routes
			public.GET("/tokens", tokenService.GetTokens)
			public.GET("/tokens/:address", tokenService.GetToken)
			public.GET("/tokens/search", tokenService.SearchTokens)

			// Swap routes
			public.POST("/swap/quote", swapService.GetQuote)
			public.POST("/swap/build", swapService.BuildTransaction)
			public.GET("/swap/pairs", swapService.GetPairs)
			public.GET("/swap/pool/:tokenA/:tokenB", swapService.GetPool)

			// Price routes
			public.GET("/prices", swapService.GetPrices)
			public.GET("/prices/:symbol", swapService.GetPrice)

			// Market routes
			public.GET("/market/stats", swapService.GetMarketStats)
			public.GET("/market/trades", swapService.GetRecentTrades)
		}

		// Auth routes
		auth := v1.Group("/auth")
		{
			auth.POST("/register", authService.Register)
			auth.POST("/login", authService.Login)
			auth.POST("/refresh", authService.RefreshToken)
			auth.POST("/logout", authService.Logout)
		}

		// Protected routes
		protected := v1.Group("")
		protected.Use(middleware.AuthRequired(authService))
		{
			// User routes
			protected.GET("/user/profile", authService.GetProfile)
			protected.PUT("/user/profile", authService.UpdateProfile)
			protected.GET("/user/portfolio", authService.GetPortfolio)
			protected.GET("/user/transactions", authService.GetTransactions)

			// Wallet routes
			protected.POST("/wallet/connect", authService.ConnectWallet)
			protected.POST("/wallet/disconnect", authService.DisconnectWallet)

			// Swap routes
			protected.POST("/swap/execute", swapService.ExecuteSwap)
			protected.POST("/swap/approve", swapService.ApproveToken)

			// Order routes
			protected.POST("/orders/create", swapService.CreateOrder)
			protected.GET("/orders", swapService.GetOrders)
			protected.DELETE("/orders/:id", swapService.CancelOrder)

			// Liquidity routes
			protected.POST("/liquidity/add", swapService.AddLiquidity)
			protected.POST("/liquidity/remove", swapService.RemoveLiquidity)
			protected.GET("/liquidity/positions", swapService.GetPositions)
		}

		// Admin routes (protected by admin middleware)
		admin := v1.Group("/admin")
		admin.Use(middleware.AdminRequired(authService))
		{
			admin.GET("/dashboard", adminService.GetDashboard)
			admin.GET("/users", adminService.GetUsers)
			admin.GET("/users/:id", adminService.GetUser)
			admin.PUT("/users/:id", adminService.UpdateUser)
			admin.DELETE("/users/:id", adminService.DeleteUser)

			// Blockchain management
			admin.POST("/chains", adminService.CreateChain)
			admin.PUT("/chains/:id", adminService.UpdateChain)
			admin.DELETE("/chains/:id", adminService.DeleteChain)

			// Token management
			admin.POST("/tokens", adminService.CreateToken)
			admin.PUT("/tokens/:id", adminService.UpdateToken)
			admin.DELETE("/tokens/:id", adminService.DeleteToken)

			// Pool management
			admin.POST("/pools", adminService.CreatePool)
			admin.PUT("/pools/:id", adminService.UpdatePool)
			admin.DELETE("/pools/:id", adminService.DeletePool)

			// System management
			admin.GET("/logs", adminService.GetLogs)
			admin.POST("/config/update", adminService.UpdateConfig)
			admin.POST("/maintenance/enable", adminService.EnableMaintenance)
			admin.POST("/maintenance/disable", adminService.DisableMaintenance)
		}
	}

	// WebSocket for real-time updates
	router.GET("/ws", services.HandleWebSocket)

	// Get port from environment or default
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("TigerSwap API Server starting on port %s", port)
	if err := router.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
