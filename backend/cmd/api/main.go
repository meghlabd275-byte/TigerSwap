package main

import (
	"context"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"tigerswap/backend/config"
	"tigerswap/backend/middleware"
	"tigerswap/backend/models"
	"tigerswap/backend/services"
)

// depositWatcherChains returns the chains the deposit watcher should scan.
// DEPOSIT_WATCH_CHAINS holds a comma-separated list of chain ids; the default
// covers the chains the platform enables out of the box.
func depositWatcherChains() []int64 {
	raw := strings.TrimSpace(os.Getenv("DEPOSIT_WATCH_CHAINS"))
	if raw == "" {
		return []int64{1, 56, 137, 42161, 10, 8453, 43114}
	}

	var chains []int64
	for _, part := range strings.Split(raw, ",") {
		chainID, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64)
		if err != nil {
			log.Printf("ignoring invalid DEPOSIT_WATCH_CHAINS entry %q: %v", part, err)
			continue
		}
		chains = append(chains, chainID)
	}
	return chains
}

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
		&models.PerpetualPosition{},
		&models.CopyTrade{},
		&models.LaunchpadProject{},
		&models.MasterWallet{},
		&models.FeeSettings{},
		&models.SystemSettings{},
		&models.BridgeTransfer{},
		&models.RevokedToken{},
		&models.AccountBalance{},
		&models.DepositAddress{},
		&models.ProcessedDeposit{},
		&models.DepositCursor{},
		&models.FuturesMarket{},
		&models.PerpsMarket{},
		&models.MarginMarket{},
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
	listingService := services.NewListingService(db)

	// Real on-chain and trading services. These share one blockchain client and
	// one price aggregator; every handler reads live chain or market state.
	blockchainClient := services.NewBlockchainClient()
	priceAggregator := services.NewPriceAggregator(redis)
	swapExecutor := services.NewSwapExecutor(blockchainClient, priceAggregator, redis)
	services.SetSwapExecutor(swapExecutor)
	services.SetBlockchainClient(blockchainClient)

	liquidityService := services.NewLiquidityPoolService(blockchainClient, priceAggregator)
	perpetualEngine := services.NewPerpetualEngine(blockchainClient, priceAggregator)
	copyTradingEngine := services.NewCopyTradingEngine(blockchainClient, priceAggregator)
	bridgeService := services.NewBridgeService(db, blockchainClient, priceAggregator)
	launchpadService := services.NewLaunchpadService(blockchainClient, priceAggregator)
	feeCollector := services.NewFeeCollectorService()
	feeCollector.SetBlockchain(blockchainClient)

	ledgerService := services.NewLedgerService(db)

	var walletService *services.WalletService
	if os.Getenv("WALLET_ENCRYPTION_KEY") != "" {
		walletService = services.NewWalletService(db)
		walletService.SetDependencies(blockchainClient, swapExecutor, perpetualEngine, copyTradingEngine)
		walletService.SetLedger(ledgerService)
	} else {
		log.Println("WALLET_ENCRYPTION_KEY not set: wallet custody endpoints are disabled")
	}

	// Credit user balances from confirmed on-chain deposits. The watcher owns
	// the only path that creates platform credit.
	depositWatcher := services.NewDepositWatcher(db, blockchainClient, ledgerService)
	depositCtx, stopDepositWatcher := context.WithCancel(context.Background())
	defer stopDepositWatcher()
	if err := depositWatcher.Start(depositCtx, depositWatcherChains()); err != nil {
		log.Printf("deposit watcher failed to start: %v", err)
	}

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
	router.Use(middleware.RedisRateLimiter(redis))
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

			// Copy trading routes (follower is the authenticated user)
			protected.POST("/copy-trading/traders", copyTradingEngine.RegisterTraderHandler)
			protected.POST("/copy-trading/follow", copyTradingEngine.StartFollowingHandler)
			protected.DELETE("/copy-trading/follow", copyTradingEngine.StopFollowingHandler)
			protected.GET("/copy-trading/positions", copyTradingEngine.GetPositionsHandler)
			protected.DELETE("/copy-trading/positions/:id", copyTradingEngine.ClosePositionHandler)

			// Perpetual engine routes
			protected.POST("/perpetual/open", perpetualEngine.OpenPositionHandler)
			protected.DELETE("/perpetual/positions/:id", perpetualEngine.ClosePositionHandler)
			protected.GET("/perpetual/positions", perpetualEngine.GetPositionsHandler)
			protected.POST("/perpetual/orders", perpetualEngine.CreateOrderHandler)

			// Liquidity pool read routes
			protected.GET("/liquidity/pools", liquidityService.GetPoolsHandler)
			protected.GET("/liquidity/pools/:address", liquidityService.GetPoolHandler)

			// Bridge routes
			protected.POST("/bridge/quote", bridgeService.GetQuoteHandler)
			protected.POST("/bridge/transfer", bridgeService.InitiateTransferHandler)
			protected.GET("/bridge/transfers", bridgeService.GetUserTransfersHandler)

			// Launchpad routes
			protected.POST("/launchpad/projects", launchpadService.CreateProjectHandler)
			protected.POST("/launchpad/projects/:id/participate", launchpadService.ParticipateHandler)
			protected.POST("/launchpad/projects/:id/claim", launchpadService.ClaimTokensHandler)

			// Wallet custody routes (only when a wallet encryption key is set)
			if walletService != nil {
				protected.POST("/wallet/master/create", walletService.CreateMasterWallet)
				protected.GET("/wallet/master", walletService.GetMasterWallet)
				protected.GET("/wallet/balances", walletService.GetBalances)
				protected.POST("/wallet/autosign", walletService.AutoSignTransaction)
				protected.POST("/wallet/deposit", walletService.Deposit)
				protected.POST("/wallet/withdraw", walletService.Withdraw)
				protected.POST("/wallet/swap", walletService.ExecuteSwap)
				protected.POST("/wallet/swap/quote", walletService.GetSwapQuote)
				protected.POST("/wallet/perpetual/open", walletService.CreatePerpetualPosition)
				protected.POST("/wallet/perpetual/close", walletService.ClosePerpetualPosition)
				protected.GET("/wallet/perpetual/positions", walletService.GetPerpetualPositions)
				protected.POST("/wallet/copy-trade", walletService.CreateCopyTrade)
				protected.GET("/wallet/copy-trades", walletService.GetCopyTrades)
			}
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

			// Futures market listings (admin managed)
			admin.POST("/futures/markets", listingService.CreateFuturesMarket)
			admin.GET("/futures/markets", listingService.GetFuturesMarkets)
			admin.PUT("/futures/markets/:id", listingService.UpdateFuturesMarket)
			admin.DELETE("/futures/markets/:id", listingService.DeleteFuturesMarket)

			// Perpetual listings (admin managed)
			admin.POST("/perps/markets", listingService.CreatePerpsMarket)
			admin.GET("/perps/markets", listingService.GetPerpsMarkets)
			admin.PUT("/perps/markets/:id", listingService.UpdatePerpsMarket)
			admin.DELETE("/perps/markets/:id", listingService.DeletePerpsMarket)

			// Margin listings (admin managed)
			admin.POST("/margin/markets", listingService.CreateMarginMarket)
			admin.GET("/margin/markets", listingService.GetMarginMarkets)
			admin.PUT("/margin/markets/:id", listingService.UpdateMarginMarket)
			admin.DELETE("/margin/markets/:id", listingService.DeleteMarginMarket)

			// Fee management (admin only)
			admin.POST("/fees/config", feeCollector.UpdateConfigHandler)
			admin.POST("/fees/master-wallet", feeCollector.SetMasterWalletHandler)
			admin.POST("/fees/distribute", feeCollector.DistributeFeesHandler)
		}

		// Price aggregation (public)
		public.GET("/prices/aggregate", priceAggregator.GetPricesHandler)
		public.POST("/prices/aggregate/quote", priceAggregator.GetQuoteHandler)
		public.GET("/prices/aggregate/:symbol/history", priceAggregator.GetHistoricalPricesHandler)

		// Perpetual markets (public read)
		public.GET("/perpetuals/markets", perpetualEngine.GetMarketsHandler)
		public.GET("/perpetuals/pairs", perpetualEngine.GetSupportedPairsHandler)

		// TigerSwap router (public read, protected broadcast)
		public.GET("/tigerswap/supported", swapExecutor.GetSupportedDEXHandler)
		public.POST("/tigerswap/quote", swapExecutor.GetSwapQuoteHandler)
		protected.POST("/tigerswap/broadcast", swapExecutor.ExecuteSwapHandler)
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
