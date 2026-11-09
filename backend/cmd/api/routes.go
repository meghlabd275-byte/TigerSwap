package main

import (
	"log"
	"os"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"tigerswap/backend/middleware"
	"tigerswap/backend/services"
)

// apiDeps is every service the HTTP router needs. Keeping them in one struct
// lets the router be built and tested without starting the server.
type apiDeps struct {
	redis              *redis.Client
	blockchainService  *services.BlockchainService
	tokenService       *services.TokenService
	swapService        *services.SwapService
	authService        *services.AuthService
	adminService       *services.AdminService
	listingService     *services.ListingService
	governanceService  *services.GovernanceService
	blockchainClient   *services.BlockchainClient
	priceAggregator    *services.PriceAggregator
	swapExecutor       *services.SwapExecutor
	liquidityService   *services.LiquidityPoolService
	marginEngine       *services.MarginEngine
	perpetualEngine    *services.PerpetualEngine
	copyTradingEngine  *services.CopyTradingEngine
	bridgeService      *services.BridgeService
	launchpadService   *services.LaunchpadService
	feeCollector       *services.FeeCollectorService
	walletService      *services.WalletService
	otcService         *services.OtcService
	twammService       *services.TwammService
	botsService        *services.BotsService
	dynamicIntegration *services.DynamicIntegrationService
	farmService        *services.FarmService
	airdropService     *services.AirdropService
	optionService      *services.OptionService
	nftService         *services.NFTMarketplaceService
	analyticsService   *services.AnalyticsService
	portfolioService   *services.PortfolioService
}

// setupRouter registers every route on a fresh engine.
func setupRouter(deps apiDeps) *gin.Engine {
	router := gin.Default()

	// Trust a forwarding header only from configured proxies. gin's default is
	// to trust every peer, which would let a client set its own client IP and
	// rotate the rate-limit key at will.
	if err := middleware.ConfigureTrustedProxies(router, os.Getenv("TRUSTED_PROXIES")); err != nil {
		log.Printf("TRUSTED_PROXIES is invalid, refusing to trust any forwarding header: %v", err)
		if setErr := router.SetTrustedProxies(nil); setErr != nil {
			log.Printf("failed to disable trusted proxies: %v", setErr)
		}
	}

	// CORS configuration. A wildcard origin cannot be combined with
	// credentials: browsers reject `Access-Control-Allow-Origin: *` when
	// `Access-Control-Allow-Credentials: true`, and echoing credentials back to
	// any origin would let an arbitrary site drive the authenticated API. The
	// allowed origins are an explicit, configured list.
	router.Use(cors.New(cors.Config{
		AllowOrigins:     corsAllowedOrigins(),
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Request-ID"},
		ExposeHeaders:    []string{"Content-Length", "X-Request-ID"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// Global middleware
	router.Use(middleware.RequestLogger())
	router.Use(middleware.RedisRateLimiter(deps.redis))
	router.Use(middleware.SecurityHeaders())
	router.Use(middleware.BodySizeLimit(1 << 20)) // 1 MiB request bodies

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
			public.GET("/chains", deps.blockchainService.GetSupportedChains)
			public.GET("/chains/:id", deps.blockchainService.GetChain)

			// Token routes
			public.GET("/tokens", deps.tokenService.GetTokens)
			public.GET("/tokens/:address", deps.tokenService.GetToken)
			public.GET("/tokens/search", deps.tokenService.SearchTokens)

			// Swap routes
			public.POST("/swap/quote", deps.swapService.GetQuote)
			public.POST("/swap/build", deps.swapService.BuildTransaction)
			public.GET("/swap/pairs", deps.swapService.GetPairs)
			public.GET("/swap/pool/:tokenA/:tokenB", deps.swapService.GetPool)

			// Price routes
			public.GET("/prices", deps.swapService.GetPrices)
			public.GET("/prices/:symbol", deps.swapService.GetPrice)

			// Market routes
			public.GET("/market/stats", deps.swapService.GetMarketStats)
			public.GET("/market/trades", deps.swapService.GetRecentTrades)
		}

		// Auth routes
		auth := v1.Group("/auth")
		{
			auth.POST("/register", deps.authService.Register)
			auth.POST("/login", deps.authService.Login)
			auth.POST("/refresh", deps.authService.RefreshToken)
			auth.POST("/logout", deps.authService.Logout)
		}

		// Protected routes
		protected := v1.Group("")
		protected.Use(middleware.AuthRequired(deps.authService))
		{
			// User routes
			protected.GET("/user/profile", deps.authService.GetProfile)
			protected.PUT("/user/profile", deps.authService.UpdateProfile)
			// Portfolio and transaction history. Both are served as camelCase
			// DTOs from the caller's own ledger rows, so the frontend validators
			// pass against the real server rather than a hand-written shape.
			protected.GET("/user/portfolio", deps.portfolioService.GetPortfolioHandler)
			protected.GET("/user/transactions", deps.portfolioService.GetTransactionsHandler)

			// Wallet routes
			protected.POST("/wallet/connect", deps.authService.ConnectWallet)
			protected.POST("/wallet/disconnect", deps.authService.DisconnectWallet)

			// Swap routes
			protected.POST("/swap/execute", deps.swapService.ExecuteSwap)
			protected.POST("/swap/approve", deps.swapService.ApproveToken)

			// Order routes
			protected.POST("/orders/create", deps.swapService.CreateOrder)
			protected.GET("/orders", deps.swapService.GetOrders)
			protected.DELETE("/orders/:id", deps.swapService.CancelOrder)

			// Liquidity routes
			protected.POST("/liquidity/add", deps.swapService.AddLiquidity)
			protected.POST("/liquidity/remove", deps.swapService.RemoveLiquidity)
			protected.GET("/liquidity/positions", deps.swapService.GetPositions)

			// Copy trading routes (follower is the authenticated user)
			protected.POST("/copy-trading/traders", deps.copyTradingEngine.RegisterTraderHandler)
			protected.POST("/copy-trading/follow", deps.copyTradingEngine.StartFollowingHandler)
			protected.DELETE("/copy-trading/follow", deps.copyTradingEngine.StopFollowingHandler)
			protected.GET("/copy-trading/positions", deps.copyTradingEngine.GetPositionsHandler)
			protected.DELETE("/copy-trading/positions/:id", deps.copyTradingEngine.ClosePositionHandler)

			// Perpetual engine routes
			protected.POST("/perpetual/open", deps.perpetualEngine.OpenPositionHandler)
			protected.DELETE("/perpetual/positions/:id", deps.perpetualEngine.ClosePositionHandler)
			protected.GET("/perpetual/positions", deps.perpetualEngine.GetPositionsHandler)

			// Spot margin: accounts, collateral, borrow/repay.
			protected.POST("/margin/accounts", deps.marginEngine.OpenMarginAccountHandler)
			protected.GET("/margin/accounts/:id", deps.marginEngine.GetMarginAccountHandler)
			protected.POST("/margin/collateral", deps.marginEngine.DepositMarginCollateralHandler)
			protected.POST("/margin/borrow", deps.marginEngine.BorrowMarginHandler)
			protected.POST("/margin/repay", deps.marginEngine.RepayMarginHandler)
			protected.POST("/perpetual/orders", deps.perpetualEngine.CreateOrderHandler)
			protected.DELETE("/perpetual/orders/:id", deps.perpetualEngine.CancelOrderHandler)

			// Liquidity pool read routes
			protected.GET("/liquidity/pools", deps.liquidityService.GetPoolsHandler)
			protected.GET("/liquidity/pools/:address", deps.liquidityService.GetPoolHandler)

			// Bridge routes
			protected.POST("/bridge/quote", deps.bridgeService.GetQuoteHandler)
			protected.POST("/bridge/transfer", deps.bridgeService.InitiateTransferHandler)
			protected.GET("/bridge/transfers", deps.bridgeService.GetUserTransfersHandler)
			protected.GET("/bridge/transfers/:id", deps.bridgeService.GetTransferHandler)

			// Launchpad routes
			protected.POST("/launchpad/projects", deps.launchpadService.CreateProjectHandler)
			protected.POST("/launchpad/projects/:id/participate", deps.launchpadService.ParticipateHandler)
			protected.POST("/launchpad/projects/:id/claim", deps.launchpadService.ClaimTokensHandler)

			// Wallet custody routes (only when a wallet encryption key is set)
			if deps.walletService != nil {
				protected.POST("/wallet/master/create", deps.walletService.CreateMasterWallet)
				protected.GET("/wallet/master", deps.walletService.GetMasterWallet)
				protected.GET("/wallet/balances", deps.walletService.GetBalances)
				protected.POST("/wallet/autosign", deps.walletService.AutoSignTransaction)
				protected.POST("/wallet/deposit", deps.walletService.Deposit)
				protected.POST("/wallet/withdraw", deps.walletService.Withdraw)
				protected.POST("/wallet/swap", deps.walletService.ExecuteSwap)
				protected.POST("/wallet/swap/quote", deps.walletService.GetSwapQuote)
				protected.POST("/wallet/perpetual/open", deps.walletService.CreatePerpetualPosition)
				protected.POST("/wallet/perpetual/close", deps.walletService.ClosePerpetualPosition)
				protected.GET("/wallet/perpetual/positions", deps.walletService.GetPerpetualPositions)
				protected.POST("/wallet/copy-trade", deps.walletService.CreateCopyTrade)
				protected.GET("/wallet/copy-trades", deps.walletService.GetCopyTrades)
			}

			// Governance routes: veTGR locks, gauge voting and the bribe market
			protected.POST("/governance/locks", deps.governanceService.CreateLock)
			protected.GET("/governance/locks", deps.governanceService.GetLocks)
			protected.POST("/governance/locks/:id/withdraw", deps.governanceService.WithdrawLock)
			protected.GET("/governance/gauges", deps.governanceService.GetGauges)
			protected.POST("/governance/vote", deps.governanceService.Vote)
			protected.POST("/governance/bribes", deps.governanceService.AddBribe)
			protected.GET("/governance/bribes", deps.governanceService.GetBribes)
			protected.POST("/governance/bribes/:id/claim", deps.governanceService.ClaimBribe)
			protected.GET("/governance/boost", deps.governanceService.GetBoost)

			// OTC private block trades. The caller is always the maker or the
			// taker; identity comes from the token, never the body.
			protected.POST("/otc/orders", deps.otcService.CreateOrder)
			protected.GET("/otc/orders", deps.otcService.ListOrders)
			protected.GET("/otc/orders/open", deps.otcService.ListOpenOrders)
			protected.POST("/otc/orders/:id/take", deps.otcService.TakeOrder)
			protected.DELETE("/otc/orders/:id", deps.otcService.CancelOrder)

			// TWAMM schedules: large orders sliced over time to remove the
			// single transaction a sandwich attack needs.
			protected.POST("/twamm/orders", deps.twammService.CreateOrder)
			protected.GET("/twamm/orders", deps.twammService.ListOrders)
			protected.DELETE("/twamm/orders/:id", deps.twammService.CancelOrder)

			// Trading bots: each strategy is an engine in the Rust bots crate.
			protected.POST("/bots", deps.botsService.CreateBot)
			protected.GET("/bots", deps.botsService.ListBots)
			protected.GET("/bots/summary", deps.botsService.BotsSummary)
			protected.GET("/bots/strategies", deps.botsService.AvailableStrategies)
			protected.GET("/bots/:id", deps.botsService.GetBot)
			protected.PUT("/bots/:id/status", deps.botsService.UpdateBotStatus)
			protected.DELETE("/bots/:id", deps.botsService.DeleteBot)
			protected.GET("/bots/:id/orders", deps.botsService.ListBotOrders)
			protected.GET("/bots/:id/performance", deps.botsService.BotPerformanceSummary)

			// Farms: stake a token and earn one or more reward streams. A stake
			// escrows a real ledger balance and rewards are funded before they can
			// be claimed.
			protected.POST("/farms/:id/stake", deps.farmService.Stake)
			protected.POST("/farms/:id/unstake", deps.farmService.Unstake)
			protected.POST("/farms/:id/claim", deps.farmService.Claim)
			protected.POST("/farms/:id/fund", deps.farmService.FundFarmReward)
			protected.GET("/farms/stakes", deps.farmService.GetStakes)

			// Airdrops: an admin allocates and funds, the user claims.
			protected.POST("/airdrops/:id/claim", deps.airdropService.ClaimAirdrop)
			protected.GET("/airdrops/allocations", deps.airdropService.GetMyAllocations)

			// Options: buy a long position, provide or withdraw series liquidity,
			// and exercise a settled position. Every leg moves a real ledger
			// balance, and a series can only pay out what it holds.
			protected.POST("/options/series/:id/buy", deps.optionService.Buy)
			protected.POST("/options/series/:id/collateral", deps.optionService.AddCollateral)
			protected.POST("/options/series/:id/collateral/remove", deps.optionService.RemoveCollateral)
			protected.POST("/options/series/:id/exercise", deps.optionService.Exercise)
			protected.POST("/options/series/:id/withdraw", deps.optionService.WithdrawLiquidity)
			protected.GET("/options/positions", deps.optionService.GetMyPositions)

			// NFT marketplace: list an escrowed token, buy, cancel, and the
			// offer book. Every leg moves a real ledger balance and a real
			// token; the token is escrowed for the life of a listing.
			protected.POST("/nft/listings", deps.nftService.ListNFT)
			protected.POST("/nft/listings/:id/buy", deps.nftService.BuyNFT)
			protected.POST("/nft/listings/:id/cancel", deps.nftService.CancelListing)
			protected.PUT("/nft/listings/:id", deps.nftService.UpdateListingPrice)
			protected.GET("/nft/listings/mine", deps.nftService.GetMyListings)
			protected.POST("/nft/offers", deps.nftService.MakeOffer)
			protected.POST("/nft/offers/:id/accept", deps.nftService.AcceptOffer)
			protected.POST("/nft/offers/:id/cancel", deps.nftService.CancelOffer)
			protected.GET("/nft/offers/mine", deps.nftService.GetMyOffers)
			protected.GET("/nft/tokens/mine", deps.nftService.GetMyNFTs)
		}

		// Admin routes (protected by admin middleware)
		admin := v1.Group("/admin")
		admin.Use(middleware.AdminRequired(deps.authService))
		{
			admin.GET("/dashboard", deps.adminService.GetDashboard)
			admin.GET("/users", deps.adminService.GetUsers)
			admin.GET("/users/:id", deps.adminService.GetUser)
			admin.PUT("/users/:id", deps.adminService.UpdateUser)
			admin.DELETE("/users/:id", deps.adminService.DeleteUser)

			// Blockchain management
			admin.POST("/chains", deps.adminService.CreateChain)
			admin.GET("/chains", deps.adminService.GetChains)
			admin.PUT("/chains/:id", deps.adminService.UpdateChain)
			admin.DELETE("/chains/:id", deps.adminService.DeleteChain)

			// Token management
			admin.POST("/tokens", deps.adminService.CreateToken)
			admin.GET("/tokens", deps.adminService.GetTokens)
			admin.PUT("/tokens/:id", deps.adminService.UpdateToken)
			admin.DELETE("/tokens/:id", deps.adminService.DeleteToken)

			// Pool management
			admin.POST("/pools", deps.adminService.CreatePool)
			admin.PUT("/pools/:id", deps.adminService.UpdatePool)
			admin.DELETE("/pools/:id", deps.adminService.DeletePool)

			// System management
			admin.GET("/settings", deps.adminService.GetSettings)
			admin.GET("/logs", deps.adminService.GetLogs)
			admin.POST("/config/update", deps.adminService.UpdateConfig)
			admin.POST("/maintenance/enable", deps.adminService.EnableMaintenance)
			admin.POST("/maintenance/disable", deps.adminService.DisableMaintenance)

			// Launchpad administration
			admin.PUT("/launchpad/projects/:id/status", deps.launchpadService.UpdateProjectStatusHandler)

			// Futures market listings (admin managed)
			admin.POST("/futures/markets", deps.listingService.CreateFuturesMarket)
			admin.GET("/futures/markets", deps.listingService.GetFuturesMarkets)
			admin.PUT("/futures/markets/:id", deps.listingService.UpdateFuturesMarket)
			admin.DELETE("/futures/markets/:id", deps.listingService.DeleteFuturesMarket)

			// Perpetual listings (admin managed)
			admin.POST("/perps/markets", deps.listingService.CreatePerpsMarket)
			admin.GET("/perps/markets", deps.listingService.GetPerpsMarkets)
			admin.PUT("/perps/markets/:id", deps.listingService.UpdatePerpsMarket)
			admin.DELETE("/perps/markets/:id", deps.listingService.DeletePerpsMarket)

			// Margin listings (admin managed)
			admin.POST("/margin/markets", deps.listingService.CreateMarginMarket)
			admin.GET("/margin/markets", deps.listingService.GetMarginMarkets)
			admin.PUT("/margin/markets/:id", deps.listingService.UpdateMarginMarket)
			admin.DELETE("/margin/markets/:id", deps.listingService.DeleteMarginMarket)

			// Margin risk operations (admin only)
			admin.GET("/margin/liquidatable", deps.marginEngine.ListLiquidatableHandler)
			admin.POST("/margin/accounts/:id/liquidate", deps.marginEngine.LiquidateAccountHandler)

			// Perpetual risk operations (admin only)
			admin.GET("/perps/liquidatable", deps.perpetualEngine.ListLiquidatablePositionsHandler)
			admin.POST("/perps/positions/:id/liquidate", deps.perpetualEngine.LiquidatePositionHandler)

			// OTC block trades: expire open orders whose window has passed.
			admin.POST("/otc/expire", deps.otcService.ExpireStaleOrdersHandler)

			// Fee management (admin only)
			admin.GET("/fees/config", deps.feeCollector.GetConfigHandler)
			admin.POST("/fees/config", deps.feeCollector.UpdateConfigHandler)
			admin.POST("/fees/master-wallet", deps.feeCollector.SetMasterWalletHandler)
			admin.POST("/fees/distribute", deps.feeCollector.DistributeFeesHandler)

			// Governance: gauges and emission weights (admin only)
			admin.POST("/gauges", deps.governanceService.CreateGauge)
			admin.GET("/gauges", deps.governanceService.GetGauges)
			admin.PUT("/gauges/:id/weight", deps.governanceService.SetGaugeWeight)
			admin.GET("/bribes", deps.governanceService.GetBribes)

			// Dynamic integration registry: chains, tokens, farms and
			// bridges an operator adds without a redeploy.
			admin.POST("/integration/chains", deps.dynamicIntegration.AddEVMChainHandler)
			admin.POST("/integration/chains/non-evm", deps.dynamicIntegration.AddNonEVMChainHandler)
			admin.GET("/integration/chains", deps.dynamicIntegration.GetChainsHandler)
			admin.POST("/integration/chains/:key/active", deps.dynamicIntegration.ActivateChainHandler)
			admin.DELETE("/integration/chains/:key", deps.dynamicIntegration.DeleteChainHandler)
			admin.POST("/integration/tokens", deps.dynamicIntegration.AddTokenHandler)
			admin.GET("/integration/tokens", deps.dynamicIntegration.GetTokensHandler)
			admin.POST("/integration/tokens/:key/active", deps.dynamicIntegration.ActivateTokenHandler)
			admin.DELETE("/integration/tokens/:key", deps.dynamicIntegration.DeleteTokenHandler)
			admin.GET("/integration/farms", deps.dynamicIntegration.GetFarmsHandler)
			admin.POST("/integration/pools", deps.dynamicIntegration.CreatePoolHandler)
			admin.POST("/integration/farms", deps.dynamicIntegration.CreateFarmHandler)
			admin.POST("/integration/bridges", deps.dynamicIntegration.AddBridgeHandler)
			admin.GET("/integration/bridges", deps.dynamicIntegration.GetBridgesHandler)
			admin.GET("/integration/stats", deps.dynamicIntegration.GetStatsHandler)
			admin.GET("/integration/search", deps.dynamicIntegration.SearchHandler)

			// Fee revenue reporting (admin only).
			admin.GET("/fees/revenue", deps.feeCollector.GetRevenueReportHandler)
			admin.GET("/fees/revenue/total", deps.feeCollector.GetTotalRevenueHandler)
			admin.GET("/fees/records", deps.feeCollector.GetFeeRecordsHandler)

			// Farm administration: create, edit, close and fund farms.
			admin.POST("/farms", deps.farmService.CreateFarm)
			admin.GET("/farms", deps.farmService.GetFarms)
			admin.GET("/farms/:id", deps.farmService.GetFarm)
			admin.PUT("/farms/:id", deps.farmService.UpdateFarm)
			admin.DELETE("/farms/:id", deps.farmService.DeleteFarm)
			admin.POST("/farms/:id/fund", deps.farmService.FundFarmReward)

			// Airdrop administration: campaigns, allocations and funding.
			admin.POST("/airdrops", deps.airdropService.CreateCampaign)
			admin.GET("/airdrops", deps.airdropService.GetCampaigns)
			admin.PUT("/airdrops/:id", deps.airdropService.UpdateCampaign)
			admin.DELETE("/airdrops/:id", deps.airdropService.DeleteCampaign)
			admin.POST("/airdrops/:id/allocations", deps.airdropService.SetAllocations)
			admin.GET("/airdrops/:id/allocations", deps.airdropService.GetCampaignAllocations)
			admin.POST("/airdrops/:id/fund", deps.airdropService.FundCampaign)

			// Option series administration: create, edit, close, settle and
			// refresh the mark price from the oracle.
			admin.POST("/options/series", deps.optionService.CreateSeries)
			admin.GET("/options/series", deps.optionService.GetSeries)
			admin.GET("/options/series/:id", deps.optionService.GetSeriesDetail)
			admin.PUT("/options/series/:id", deps.optionService.UpdateSeries)
			admin.DELETE("/options/series/:id", deps.optionService.DeleteSeries)
			admin.POST("/options/series/:id/settle", deps.optionService.SettleSeries)
			admin.POST("/options/series/:id/refresh-price", deps.optionService.RefreshMarkPrice)

			// NFT marketplace administration: collections, minting, and the
			// marketplace fee.
			admin.POST("/nft/collections", deps.nftService.CreateCollection)
			admin.GET("/nft/collections", deps.nftService.GetCollections)
			admin.PUT("/nft/collections/:id", deps.nftService.UpdateCollection)
			admin.DELETE("/nft/collections/:id", deps.nftService.DeleteCollection)
			admin.POST("/nft/collections/:id/mint", deps.nftService.Mint)
			admin.GET("/nft/listings", deps.nftService.GetListings)
			admin.PUT("/nft/config", deps.nftService.UpdateMarketplaceConfig)

			// Analytics rollup. The rollup is idempotent, so an operator can
			// re-run a window after a late confirmation or a reorg without
			// inflating volume.
			admin.POST("/analytics/rollup", deps.analyticsService.RollupHandler)
		}

		// Price aggregation (public)
		public.GET("/prices/aggregate", deps.priceAggregator.GetPricesHandler)
		public.GET("/prices/aggregate/:symbol", deps.priceAggregator.GetPriceHandler)
		public.POST("/prices/aggregate/quote", deps.priceAggregator.GetQuoteHandler)
		public.GET("/prices/aggregate/:symbol/history", deps.priceAggregator.GetHistoricalPricesHandler)

		// Perpetual markets (public read)
		public.GET("/perpetuals/markets", deps.perpetualEngine.GetMarketsHandler)
		public.GET("/perpetuals/pairs", deps.perpetualEngine.GetSupportedPairsHandler)

		// Spot-margin markets (public read): the tradable pairs shown in the
		// margin panel. Only active listings are returned.
		public.GET("/margin/markets", deps.listingService.GetActiveMarginMarkets)

		// Launchpad (public read): projects and allocation tiers.
		public.GET("/launchpad/projects", deps.launchpadService.GetProjectsHandler)
		public.GET("/launchpad/projects/:id", deps.launchpadService.GetProjectHandler)
		public.GET("/launchpad/tiers", deps.launchpadService.GetTiersHandler)

		// Farms and airdrops (public read): the live listings a user can stake
		// in or claim from.
		public.GET("/farms", deps.farmService.GetActiveFarms)
		public.GET("/farms/all", deps.farmService.GetFarms)
		public.GET("/farms/:id", deps.farmService.GetFarm)
		public.GET("/airdrops", deps.airdropService.GetActiveCampaigns)
		public.GET("/airdrops/all", deps.airdropService.GetCampaigns)

		// Options (public read): the tradable series with live greeks.
		public.GET("/options/series", deps.optionService.GetActiveSeries)
		public.GET("/options/series/all", deps.optionService.GetSeries)
		public.GET("/options/series/:id", deps.optionService.GetSeriesDetail)

		// NFT marketplace (public read): live listings, the offer book for a
		// token, active collections and the fee configuration.
		public.GET("/nft/listings", deps.nftService.GetActiveListings)
		public.GET("/nft/collections", deps.nftService.GetActiveCollections)
		public.GET("/nft/offers", deps.nftService.GetOffers)
		public.GET("/nft/config", deps.nftService.GetMarketplaceConfig)

		// Analytics (public read). Every figure is aggregated from rows
		// TigerSwap wrote: confirmed swaps, the derivatives book, the
		// margin book and the marketplace. No external indexer is called.
		public.GET("/analytics/overview", deps.analyticsService.GetOverviewHandler)
		public.GET("/analytics/volume", deps.analyticsService.GetVolumeHandler)
		public.GET("/analytics/pairs", deps.analyticsService.GetPairsHandler)
		public.GET("/analytics/traders", deps.analyticsService.GetTopTradersHandler)

		// TigerSwap router (public read, protected broadcast)
		public.GET("/tigerswap/supported", deps.swapExecutor.GetSupportedDEXHandler)
		public.POST("/tigerswap/quote", deps.swapExecutor.GetSwapQuoteHandler)
		protected.POST("/tigerswap/broadcast", deps.swapExecutor.ExecuteSwapHandler)

		// Chain data (public read). Balances, gas and blocks are read from
		// the configured RPC endpoint for the chain, never fabricated.
		public.GET("/chain/:chain_id/balance", deps.blockchainClient.GetBalanceHandler)
		public.GET("/chain/:chain_id/token-balance", deps.blockchainClient.GetTokenBalanceHandler)
		public.GET("/chain/:chain_id/gas-price", deps.blockchainClient.GetGasPriceHandler)
		public.GET("/chain/:chain_id/block", deps.blockchainClient.GetCurrentBlockHandler)
		public.GET("/chain/:chain_id/tx/:tx_hash", deps.blockchainClient.GetTransactionReceiptHandler)
		protected.POST("/chain/broadcast", deps.blockchainClient.SendTransactionHandler)

		// Bridge route metadata. This is static relayer configuration (which
		// chains and routers TigerBridge supports), not per-user data, so it
		// is public. A transfer itself is visible only to a party of it and
		// lives in `protected` above.
		public.GET("/bridge/chains", deps.bridgeService.GetSupportedChainsHandler)
		public.GET("/bridge/chains/:chain_id/supported", deps.bridgeService.GetSupportedBridgesHandler)

		// Copy-trading leaderboard. A trader is keyed by address.
		public.GET("/copy-trading/traders", deps.copyTradingEngine.GetTopTradersHandler)
		public.GET("/copy-trading/traders/:address", deps.copyTradingEngine.GetTraderHandler)

		// Perpetual funding rate for a market.
		public.GET("/perpetuals/funding/:pair", deps.perpetualEngine.GetFundingRateHandler)

		// Liquidity: add to a pool (protected, reads live pool state).
		protected.POST("/liquidity/pool/add", deps.liquidityService.AddLiquidityHandler)
	}

	// WebSocket for real-time updates
	router.GET("/ws", services.HandleWebSocket)

	return router
}
