package main

import (
	"context"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
	"tigerswap/backend/config"
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

// corsAllowedOrigins returns the browser origins permitted to call the API with
// credentials. CORS_ALLOW_ORIGINS is a comma-separated list; entries are
// trimmed and blank entries dropped. A wildcard is never returned, because
// AllowCredentials is enabled and the browser would reject the response.
func corsAllowedOrigins() []string {
	raw := strings.TrimSpace(os.Getenv("CORS_ALLOW_ORIGINS"))
	if raw == "" {
		return []string{"http://localhost:3000"}
	}

	origins := make([]string, 0, 4)
	for _, part := range strings.Split(raw, ",") {
		origin := strings.TrimSpace(part)
		if origin == "" || origin == "*" {
			continue
		}
		origins = append(origins, origin)
	}
	if len(origins) == 0 {
		return []string{"http://localhost:3000"}
	}
	return origins
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
		&models.MarginAccount{},
		&models.MarginCollateral{},
		&models.MarginBorrow{},
		&models.VeLock{},
		&models.Gauge{},
		&models.GaugeVote{},
		&models.Bribe{},
		&models.BribeClaim{},
		&models.BotInstance{},
		&models.BotOrder{},
		&models.BotPerformance{},
		&models.OtcOrder{},
		&models.TwammOrder{},
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
	governanceService := services.NewGovernanceService(db)

	// Real on-chain and trading services. These share one blockchain client and
	// one price aggregator; every handler reads live chain or market state.
	blockchainClient := services.NewBlockchainClient()
	priceAggregator := services.NewPriceAggregator(redis)
	swapExecutor := services.NewSwapExecutor(blockchainClient, priceAggregator, redis)
	services.SetSwapExecutor(swapExecutor)
	services.SetBlockchainClient(blockchainClient)

	liquidityService := services.NewLiquidityPoolService(blockchainClient, priceAggregator)
	marginEngine := services.NewMarginEngine(db, priceAggregator)
	perpetualEngine := services.NewPerpetualEngine(blockchainClient, priceAggregator)
	if err := perpetualEngine.SetDB(db); err != nil {
		log.Fatalf("Failed to load perpetual markets: %v", err)
	}
	// Reload the engine's market registry whenever an admin changes a listing,
	// so a newly listed or paused market takes effect without a restart.
	listingService.SetMarketsChangedHook(func() {
		if err := perpetualEngine.RefreshMarkets(); err != nil {
			log.Printf("failed to refresh perpetual markets: %v", err)
		}
	})
	copyTradingEngine := services.NewCopyTradingEngine(blockchainClient, priceAggregator)
	bridgeService := services.NewBridgeService(db, blockchainClient, priceAggregator)
	launchpadService := services.NewLaunchpadService(db, blockchainClient, priceAggregator)
	feeCollector := services.NewFeeCollectorService()
	feeCollector.SetBlockchain(blockchainClient)
	feeCollector.SetDB(db)

	// DEX-native order types and automation. OTC block trades, TWAMM
	// schedules and trading bots are each persisted so a restart resumes them.
	otcService := services.NewOtcService(db)
	twammService := services.NewTwammService(db)
	botsService := services.NewBotsService(db)
	dynamicIntegration := services.NewDynamicIntegrationService()

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
	router := setupRouter(apiDeps{
		redis:              redis,
		blockchainService:  blockchainService,
		tokenService:       tokenService,
		swapService:        swapService,
		authService:        authService,
		adminService:       adminService,
		listingService:     listingService,
		governanceService:  governanceService,
		blockchainClient:   blockchainClient,
		priceAggregator:    priceAggregator,
		swapExecutor:       swapExecutor,
		liquidityService:   liquidityService,
		marginEngine:       marginEngine,
		perpetualEngine:    perpetualEngine,
		copyTradingEngine:  copyTradingEngine,
		bridgeService:      bridgeService,
		launchpadService:   launchpadService,
		feeCollector:       feeCollector,
		walletService:      walletService,
		otcService:         otcService,
		twammService:       twammService,
		botsService:        botsService,
		dynamicIntegration: dynamicIntegration,
	})

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
