package main

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/joho/godotenv"
	"tigerswap/backend/config"
	orderbookpkg "tigerswap/backend/go/orderbook"
	orderspkg "tigerswap/backend/go/orders"
	perpspkg "tigerswap/backend/go/perps"
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

// walletSigner returns a signer for approved withdrawal releases. It resolves the
// platform key through the wallet service so the queue and the direct path use
// one key source; when custody is disabled the signer reports unavailable and an
// approval is refused rather than sent unsigned.
func walletSigner(wallet *services.WalletService) func() (*ecdsa.PrivateKey, error) {
	return func() (*ecdsa.PrivateKey, error) {
		if wallet == nil {
			return nil, fmt.Errorf("wallet custody is not configured")
		}
		return wallet.SignWithdrawalKey()
	}
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
		&models.OnChainPerpPosition{},
		&models.OnChainPerpCursor{},
		&models.OnChainPerpLiquidation{},
		&models.OnChainBookPosition{},
		&models.OnChainBookCursor{},
		&models.OnChainBookLiquidation{},
		&models.OnChainBookFill{},
		&models.WalletLoginNonce{},
		&models.RefreshToken{},
		&models.AccountToken{},
		&models.WithdrawalAddress{},
		&models.WithdrawalLimit{},
		&models.WithdrawalRequest{},
		&models.KycProfile{},
		&models.SanctionsEntry{},
		&models.ComplianceSettings{},
		&models.DeveloperAPIKey{},
		&models.Webhook{},
		&models.WebhookDelivery{},
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
	authService.SetMailer(services.NewMailer())
	adminService := services.NewAdminService(db)
	adminService.SetAuthService(authService)
	platformGate := services.NewPlatformGate(db)
	adminService.SetPlatformGate(platformGate)
	authService.SetPlatformGate(platformGate)
	// Compliance: identity verification, sanctions screening and geographic
	// restriction. The service is wired so an unverified account cannot trade or
	// withdraw and a screened destination or jurisdiction is refused.
	complianceService := services.NewComplianceService(db)
	apiKeyService := services.NewAPIKeyService(db)
	webhookService := services.NewWebhookService(db)
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
	// On-chain perpetuals: the deployed TigerSwapPerpetuals contract is the
	// settlement layer, so a derivative position is verifiable on-chain rather
	// than only in the platform ledger. The service dials only TigerSwap's own
	// nodes and needs no user key — order building returns unsigned calldata.
	onChainPerps := services.NewOnChainPerpsService(db, priceAggregator)
	// Set the keeper key before loading deployments: a client is built with the
	// signer it sends with, so the keeper that liquidates and the oracle writer
	// that publishes marks can actually submit a transaction.
	if signerKey, ok := perpspkg.LoadSigner(); ok {
		if key, err := crypto.HexToECDSA(signerKey); err == nil {
			onChainPerps.SetKeeperKey(key)
		} else {
			log.Printf("warning: TIGERSWAP_PERPS_KEEPER_KEY is not a valid key: %v", err)
		}
	}
	// The owner key signs owner-only on-chain calls (createMarket). It is separate
	// from the keeper key because the contract owner is not the keeper.
	if ownerKey, ok := perpspkg.LoadOwnerKey(); ok {
		if key, err := crypto.HexToECDSA(ownerKey); err == nil {
			onChainPerps.SetOwnerKey(key)
		} else {
			log.Printf("warning: TIGERSWAP_PERPS_OWNER_KEY is not a valid key: %v", err)
		}
	}
	if err := onChainPerps.LoadDeployments(); err != nil {
		log.Printf("warning: on-chain perps deployments not loaded: %v", err)
	}
	if err := onChainPerps.ReloadSymbols(); err != nil {
		log.Printf("warning: could not map perp market symbols: %v", err)
	}
	// Mirror a new perps listing onto the deployed contract, so an on-chain
	// position can be opened for a listing the operator can already see.
	listingService.SetPerpsMarketCreatedHook(func(market *models.PerpsMarket) error {
		return onChainPerps.MirrorPerpsMarket(context.Background(), market)
	})
	// On-chain advanced orders and TWAMM: the deployed TigerSwapAdvancedOrders
	// and TigerSwapTWAMM contracts escrow and fill orders on-chain, so a limit,
	// stop, TWAP or long-term order is verifiable on-chain rather than only in
	// the platform ledger. Order building returns unsigned calldata; the
	// executor key only fills resting orders the platform may submit itself.
	onChainOrders := services.NewOnChainOrdersService(db)
	if execKey, ok := orderspkg.LoadSigner(); ok {
		if key, err := crypto.HexToECDSA(execKey); err == nil {
			onChainOrders.SetExecutorKey(key)
		} else {
			log.Printf("warning: TIGERSWAP_ORDERS_EXECUTOR_KEY is not a valid key: %v", err)
		}
	}
	if err := onChainOrders.LoadDeployments(); err != nil {
		log.Printf("warning: on-chain orders deployments not loaded: %v", err)
	}

	// On-chain perpetual order book: the deployed TigerSwapPerpOrderBook
	// contract is the derivatives settlement layer — orders, matching, margin,
	// funding and liquidation all live on-chain (the Hyperliquid/dYdX model).
	// Order building returns unsigned calldata the caller's own wallet signs.
	onChainPerpBook := services.NewOnChainPerpBookService(db, priceAggregator)
	if key, ok := orderbookpkg.LoadSigner(); ok {
		if parsed, err := crypto.HexToECDSA(key); err == nil {
			onChainPerpBook.SetKeeperKey(parsed)
		} else {
			log.Printf("warning: TIGERSWAP_ORDERBOOK_KEEPER_KEY is not a valid key: %v", err)
		}
	}
	if key, ok := orderbookpkg.LoadOwnerKey(); ok {
		if parsed, err := crypto.HexToECDSA(key); err == nil {
			onChainPerpBook.SetOwnerKey(parsed)
		} else {
			log.Printf("warning: TIGERSWAP_ORDERBOOK_OWNER_KEY is not a valid key: %v", err)
		}
	}
	if err := onChainPerpBook.LoadDeployments(); err != nil {
		log.Printf("warning: on-chain perp order book deployments not loaded: %v", err)
	}
	if err := onChainPerpBook.ReloadSymbols(); err != nil {
		log.Printf("warning: could not map order book symbols: %v", err)
	}

	copyTradingEngine := services.NewCopyTradingEngine(blockchainClient, priceAggregator).SetDB(db)
	bridgeService := services.NewBridgeService(db, blockchainClient, priceAggregator)
	launchpadService := services.NewLaunchpadService(db, blockchainClient, priceAggregator)
	feeCollector := services.NewFeeCollectorService()
	feeCollector.SetBlockchain(blockchainClient)
	feeCollector.SetDB(db)

	// DEX-native order types and automation. OTC block trades and trading bots
	// are each persisted so a restart resumes them.
	otcService := services.NewOtcService(db)
	botsService := services.NewBotsService(db)
	dynamicIntegration := services.NewDynamicIntegrationService(db)

	ledgerService := services.NewLedgerService(db)

	// The perpetual engine escrows margin from and settles payouts to the ledger,
	// denominated in each listing's quote asset. Wiring it here means a position
	// can only be opened against a real ledger balance.
	perpetualEngine.SetLedger(ledgerService, services.PerpSettlementChainID())
	marginEngine.SetLedger(ledgerService, services.PerpSettlementChainID())
	otcService.SetLedger(ledgerService, services.PerpSettlementChainID())
	copyTradingEngine.SetLedger(ledgerService, services.PerpSettlementChainID())
	launchpadService.SetLedger(ledgerService, services.PerpSettlementChainID())
	// Governance bribes escrow a real ledger balance: funding debits the funder
	// and a claim credits the voter, so a bribe only pays out what was funded.
	governanceService.SetLedger(ledgerService, services.PerpSettlementChainID())

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
	if config.Secret("WALLET_ENCRYPTION_KEY") != "" {
		walletService = services.NewWalletService(db)
		walletService.SetDependencies(blockchainClient, swapExecutor, perpetualEngine, copyTradingEngine)
		walletService.SetLedger(ledgerService)
		// Outbound bridge locks are signed with the holder's own custody key, so
		// the relayer needs the wallet service to resolve that key.
		bridgeService.SetWallet(walletService)
	} else {
		log.Println("WALLET_ENCRYPTION_KEY not set: wallet custody endpoints are disabled")
	}

	// The withdrawal policy is attached to the wallet service before the router
	// is built, so a withdrawal cannot reach the signer without passing the
	// allow-list and limit checks. It signs approved queue releases with the same
	// platform key the direct path uses.
	withdrawalService := services.NewWithdrawalService(db, ledgerService, blockchainClient, walletSigner(walletService))
	if walletService != nil {
		walletService.SetWithdrawalPolicy(withdrawalService)
		// Screen a withdrawal destination against the sanctions denylist and the
		// geographic policy before the policy or the signer is reached.
		walletService.SetCompliance(complianceService)
		// Charge the configured withdrawal fee to the ledger and record it.
		walletService.SetFeeCollector(feeCollector)
	}

	// Credit user balances from confirmed on-chain deposits. The watcher owns
	// the only path that creates platform credit.
	depositWatcher := services.NewDepositWatcher(db, blockchainClient, ledgerService)
	depositWatcher.SetWebhooks(webhookService)
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
	// Reconcile pending withdrawals against their on-chain receipt so a reverted
	// transaction reverses the debit instead of leaving the user charged forever.
	services.StartWithdrawalReconciler(realtimeCtx, db, blockchainClient, ledgerService, webhookService, services.DefaultWithdrawalReconcileInterval)
	// Index on-chain perp events and liquidate unhealthy positions. Started only
	// when a deployment is configured; otherwise it is a no-op.
	onChainPerps.Start(realtimeCtx)
	// Fill resting advanced orders and trigger stop/take-profit orders whose
	// on-chain condition is met. Started only when an executor key and an orders
	// deployment are configured; otherwise it is a no-op.
	onChainOrders.Start(realtimeCtx)
	onChainPerpBook.Start(realtimeCtx)
	services.StartPriceSampler(realtimeCtx, priceAggregator, services.DefaultPriceSampleInterval)
	// Deliver queued webhook events to subscribers with bounded retries.
	services.StartWebhookDispatcher(realtimeCtx, webhookService, services.DefaultWebhookDispatchInterval)

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

	// Metrics registry. Probes report the real state of a dependency, so
	// /readyz drains a replica whose database or chain client is unusable.
	metrics := services.NewMetrics()
	metrics.RegisterProbe("database", func() error {
		sqlDB, err := db.DB()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		return sqlDB.PingContext(ctx)
	})
	if redis != nil {
		metrics.RegisterProbe("redis", func() error {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			return redis.Ping(ctx).Err()
		})
	}
	if blockchainClient != nil {
		metrics.RegisterProbe("blockchain_client", func() error {
			if blockchainClient.ActiveChainCount() == 0 {
				return fmt.Errorf("no active chain endpoint is registered")
			}
			return nil
		})
	}

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
		onChainPerps:       onChainPerps,
		onChainPerpBook:    onChainPerpBook,
		onChainOrders:      onChainOrders,
		copyTradingEngine:  copyTradingEngine,
		bridgeService:      bridgeService,
		launchpadService:   launchpadService,
		feeCollector:       feeCollector,
		walletService:      walletService,
		withdrawalService:  withdrawalService,
		otcService:         otcService,
		botsService:        botsService,
		dynamicIntegration: dynamicIntegration,
		farmService:        farmService,
		airdropService:     airdropService,
		optionService:      optionService,
		nftService:         nftService,
		analyticsService:   analyticsService,
		portfolioService:   portfolioService,
		platformGate:       platformGate,
		complianceService:  complianceService,
		apiKeyService:      apiKeyService,
		webhookService:     webhookService,
		metrics:            metrics,
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

	// Serve in the background so a signal can trigger a graceful shutdown: the
	// listener stops accepting, in-flight requests are given a bounded window to
	// finish, and then the process exits. Without this an orchestrator's SIGTERM
	// would kill in-flight withdrawals mid-flight.
	serveErr := make(chan error, 1)
	go func() {
		log.Printf("TigerSwap API Server starting on port %s", port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
			return
		}
		serveErr <- nil
	}()

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serveErr:
		if err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}
	case sig := <-shutdown:
		log.Printf("received %s, shutting down gracefully", sig)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			log.Printf("graceful shutdown did not complete: %v", err)
			_ = server.Close()
		}
		// Wait for the serve goroutine to observe the close so the process does
		// not exit while a handler is still running.
		if err := <-serveErr; err != nil {
			log.Printf("server stopped with error: %v", err)
		}
		log.Printf("TigerSwap API Server stopped")
	}
}
