package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"tigerswap/backend/config"
	"tigerswap/backend/models"
	"tigerswap/backend/services"
	"tigerswap/backend/services/botengine"
)

// depositWatcherChains returns the chains the deposit watcher should scan.
// DEPOSIT_WATCH_CHAINS holds a comma-separated list of chain ids; the default
// covers the chains the platform enables out of the box.
func depositWatcherChains() []int64 {
	raw := strings.TrimSpace(os.Getenv("DEPOSIT_WATCH_CHAINS"))
	if raw == "" {
		// TigerSmartChain (6666) is the platform's own chain, so it is watched by
		// default alongside the other enabled networks. A chain with no
		// TIGERSWAP_RPC_<id> endpoint is skipped at scan time rather than failing.
		return []int64{1, 56, 137, 42161, 10, 8453, 43114, 6666}
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
		&models.PerpetualOrder{},
		&models.CopyTrade{},
		&models.CopyTrader{},
		&models.CopyFollow{},
		&models.CopyPositionRecord{},
		&models.FeeRecordModel{},
		&models.LaunchpadProject{},
		&models.LaunchpadAllocation{},
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
		&models.FarmPool{},
		&models.FarmReward{},
		&models.FarmStake{},
		&models.FarmStakeReward{},
		&models.AirdropCampaign{},
		&models.AirdropAllocation{},
		&models.OptionSeries{},
		&models.OptionPosition{},
		&models.OptionLiquidity{},
		&models.NFTCollection{},
		&models.NFTToken{},
		&models.NFTListing{},
		&models.NFTOffer{},
		&models.VolumeBucket{},
		&models.IntegrationChain{},
		&models.IntegrationToken{},
		&models.IntegrationBridge{},
		&models.IntegrationFarm{},
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
	adminService.SetAuthService(authService)
	listingService := services.NewListingService(db)
	governanceService := services.NewGovernanceService(db)

	// Real on-chain and trading services. These share one blockchain client and
	// one price aggregator; every handler reads live chain or market state.
	blockchainClient := services.NewBlockchainClient()
	// Dial the endpoints the operator stored in the admin panel, so an
	// admin-entered node is used instead of leaving the chain unreachable.
	if err := blockchainClient.LoadChainRPCs(db); err != nil {
		log.Printf("warning: could not load stored chain RPC endpoints: %v", err)
	}
	priceAggregator := services.NewPriceAggregator(redis)
	swapExecutor := services.NewSwapExecutor(blockchainClient, priceAggregator, redis)
	services.SetSwapExecutor(swapExecutor)
	services.SetBlockchainClient(blockchainClient)
	adminService.SetBlockchainClient(blockchainClient)
	// The swap quote and price endpoints read the aggregator through this
	// package-level handle; without it they can only answer 503.
	services.SetPriceAggregator(priceAggregator)

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
	copyTradingEngine := services.NewCopyTradingEngine(blockchainClient, priceAggregator).SetDB(db)
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
	dynamicIntegration := services.NewDynamicIntegrationService(db)

	ledgerService := services.NewLedgerService(db)

	// Liquidity mining and token distribution. Both escrow a real ledger
	// balance, so a farm reward or an airdrop claim can only pay out what was
	// funded. The farm boost reads veTGR voting power from governance.
	farmService := services.NewFarmService(db, ledgerService)
	farmService.SetBoostSource(governanceService)
	farmService.SetPriceSource(priceAggregator)
	airdropService := services.NewAirdropService(db, ledgerService)

	// European option AMM. A series is a fully collateralised pool; the premium
	// is priced with the exact port of the on-chain Black-Scholes library and the
	// payout is bounded by the collateral the pool actually holds.
	optionService := services.NewOptionService(db, ledgerService)
	optionService.SetPriceSource(priceAggregator)

	// Self-built NFT marketplace. A listing escrows the token and an offer
	// escrows the buyer's payment, so neither a sale nor an accepted offer can
	// fail for a missing asset or balance. The marketplace fee is frozen onto
	// each listing and offer at creation.
	nftService := services.NewNFTMarketplaceService(db, ledgerService)
	if feeAccount := os.Getenv("NFT_FEE_ACCOUNT"); feeAccount != "" {
		nftService.SetFeeAccount(feeAccount)
	}
	if feeBpsRaw := os.Getenv("NFT_FEE_BPS"); feeBpsRaw != "" {
		if feeBps, err := strconv.Atoi(feeBpsRaw); err == nil {
			if err := nftService.SetFeeBps(feeBps); err != nil {
				log.Printf("NFT_FEE_BPS is invalid: %v", err)
			}
		} else {
			log.Printf("NFT_FEE_BPS is not a number: %v", err)
		}
	}

	// Protocol and market analytics. Every figure is aggregated from rows
	// TigerSwap wrote, so the dashboard reads the platform's own ledger and
	// books rather than an external indexer. The rollup is idempotent.
	analyticsService := services.NewAnalyticsService(db, priceAggregator)

	// Account portfolio and transaction history. Holdings come from the
	// caller's own ledger rows, valued at the oracle price; nothing is
	// derived from a client-supplied identity or an external indexer. The
	// derivatives engine is attached so unrealized PnL is the live, re-marked
	// value rather than the row the engine last persisted.
	portfolioService := services.NewPortfolioService(db, priceAggregator)
	portfolioService.SetPositionSource(perpetualEngine)

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
	// The WebSocket hub and the market feed share one context so both stop when
	// the process shuts down.
	realtimeCtx, stopRealtime := context.WithCancel(context.Background())
	defer stopRealtime()
	services.StartWebSocketHub(realtimeCtx)
	services.StartMarketFeed(realtimeCtx, priceAggregator, services.DefaultMarketFeedInterval)
	services.StartPositionMaintenance(realtimeCtx, perpetualEngine, copyTradingEngine, services.DefaultPositionRefreshInterval)
	services.StartBridgeRelayer(realtimeCtx, bridgeService, services.DefaultBridgeSweepInterval)
	services.StartPriceSampler(realtimeCtx, priceAggregator, services.DefaultPriceSampleInterval)

	// Drive every active bot against live market data. The Rust engine is the
	// strategy layer; when the server is built without it the runner reports
	// that instead of pretending bots trade.
	botEngine, err := botengine.New()
	if err != nil {
		log.Printf("bot engine unavailable: %v", err)
	}
	botRunner := services.NewBotRunner(db, botsService, perpetualEngine, botEngine)
	botRunner.SetVenueSource(services.NewPoolVenueSource(priceAggregator))
	botRunner.SetListingSource(services.NewPerpListingSource(db, perpetualEngine))
	botRunner.SetMarginEngine(marginEngine)
	botRunner.SetPortfolioService(portfolioService)
	botRunner.SetCopySource(services.NewPerpCopySource(perpetualEngine))
	go botRunner.Run(realtimeCtx)

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
		farmService:        farmService,
		airdropService:     airdropService,
		optionService:      optionService,
		nftService:         nftService,
		analyticsService:   analyticsService,
		portfolioService:   portfolioService,
	})

	// Get port from environment or default
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Explicit server timeouts. A bare http.Server has no read deadline, so a
	// client that opens a connection and dribbles bytes can hold a worker
	// indefinitely. ReadHeaderTimeout bounds the header phase, ReadTimeout the
	// whole request, IdleTimeout a kept-alive connection, and MaxHeaderBytes
	// caps the header block before it is parsed.
	server := &http.Server{
		Addr:              ":" + port,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	log.Printf("TigerSwap API Server starting on port %s", port)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("Failed to start server: %v", err)
	}
}
