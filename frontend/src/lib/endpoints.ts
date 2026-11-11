/**
 * Typed endpoints for the TigerSwap API.
 *
 * Each function validates the raw response before returning it, so callers
 * only ever see checked data. Where the backend exposes an endpoint, it is
 * called; where it does not, the function is simply absent rather than
 * returning a fabricated value.
 */

import { api, apiRequest } from "./apiClient";
import { integer, object, string } from "./validation";
import {
  AccountBalance,
  AdminAuditLog,
  AirdropAllocationView,
  AirdropCampaign,
  AirdropClaimResult,
  AdminChain,
  AdminDashboard,
  AdminSettings,
  AdminToken,
  Bribe,
  BribeClaimResult,
  BotInstance,
  BotOrder,
  BotPerformance,
  BotsSummary,
  BotStrategy,
  BridgeQuote,
  BridgeTransfer,
  Chain,
  CopyPosition,
  CopyTrader,
  FarmClaimResult,
  FarmStakeView,
  FarmView,
  FeeConfig,
  FuturesMarket,
  FuturesMarketInput,
  Gauge,
  GovernanceBoost,
  LaunchpadProject,
  MarginAccount,
  MarginAccountView,
  MarginListing,
  MarginListingInput,
  MarketStats,
  NFTCollection,
  NFTListing,
  NFTListingView,
  NFTMarketplaceConfig,
  NFTOffer,
  NFTOfferView,
  NFTToken,
  OtcOrder,
  OptionBuyResult,
  OptionExerciseResult,
  OptionPositionView,
  OptionSeries,
  OptionSeriesView,
  PerpMarket,
  PerpOpenResult,
  PerpOrder,
  PerpPosition,
  PerpsListing,
  PerpsListingInput,
  Pool,
  AnalyticsRollupResult,
  PairAnalytics,
  Portfolio,
  ProtocolOverview,
  PriceEntry,
  SwapQuote,
  Token,
  Trade,
  TopTrader,
  TwammOrder,
  UserProfile,
  UserTransaction,
  VolumeSeriesResponse,
  VeLock,
  accountBalanceListValidator,
  adminAuditLogListValidator,
  adminChainListValidator,
  adminChainValidator,
  adminDashboardValidator,
  adminSettingsValidator,
  adminTokenListValidator,
  adminTokenValidator,
  bridgeChainsValidator,
  bridgeQuoteValidator,
  bridgeTransferListValidator,
  bridgeTransferValidator,
  chainListValidator,
  copyPositionListValidator,
  copyTraderListValidator,
  copyTraderValidator,
  airdropAllocationViewListValidator,
  airdropCampaignListValidator,
  airdropCampaignValidator,
  airdropClaimResultValidator,
  farmClaimResultValidator,
  farmPoolValidator,
  farmStakeViewListValidator,
  farmViewListValidator,
  farmViewValidator,
  optionBuyResultValidator,
  optionExerciseResultValidator,
  optionPositionViewListValidator,
  optionSeriesEnvelopeValidator,
  optionSeriesViewListValidator,
  optionSeriesViewValidator,
  feeConfigValidator,
  futuresMarketListValidator,
  futuresMarketValidator,
  bribeClaimResultValidator,
  bribeListValidator,
  bribeValidator,
  botListValidator,
  botOrderListValidator,
  botPerformanceListValidator,
  botStrategyListValidator,
  botsSummaryValidator,
  botValidator,
  gaugeListValidator,
  gaugeValidator,
  governanceBoostValidator,
  launchpadProjectListValidator,
  launchpadProjectValidator,
  marginAccountValidator,
  marginAccountViewValidator,
  marginListingListValidator,
  marginListingValidator,
  marketStatsValidator,
  nftCollectionEnvelopeValidator,
  nftCollectionListValidator,
  nftListingEnvelopeValidator,
  nftListingListValidator,
  nftListingViewListValidator,
  nftMarketplaceConfigValidator,
  nftOfferEnvelopeValidator,
  nftOfferListValidator,
  nftOfferViewListValidator,
  nftTokenEnvelopeValidator,
  nftTokenListValidator,
  analyticsRollupValidator,
  pairAnalyticsListValidator,
  otcOrderListValidator,
  otcOrderValidator,
  perpMarketListValidator,
  perpOpenResultValidator,
  perpOrderListValidator,
  perpOrderValidator,
  protocolOverviewValidator,
  perpPositionListValidator,
  perpsListingListValidator,
  perpsListingValidator,
  poolListValidator,
  portfolioValidator,
  priceListValidator,
  swapQuoteValidator,
  tokenListValidator,
  tradeListValidator,
  topTraderListValidator,
  twammOrderListValidator,
  twammOrderValidator,
  userProfileValidator,
  volumeSeriesValidator,
  userTransactionListValidator,
  veLockListValidator,
  veLockValidator,
} from "./types";

export const endpoints = {
  /** Public market data. */
  async getTokens(chainId?: number): Promise<Token[]> {
    const body = await api.get("/tokens", { authenticated: false, query: { chainId } });
    return tokenListValidator(body);
  },

  async getChains(): Promise<Chain[]> {
    const body = await api.get("/chains", { authenticated: false });
    return chainListValidator(body);
  },

  async getMarketStats(): Promise<MarketStats> {
    const body = await api.get("/market/stats", { authenticated: false });
    return marketStatsValidator(body);
  },

  async getRecentTrades(limit = 50): Promise<Trade[]> {
    const body = await api.get("/market/trades", { authenticated: false, query: { limit } });
    return tradeListValidator(body);
  },

  async getPairs(): Promise<Pool[]> {
    const body = await api.get("/swap/pairs", { authenticated: false });
    return poolListValidator(body);
  },

  async getPrices(): Promise<PriceEntry[]> {
    const body = await api.get("/prices", { authenticated: false });
    return priceListValidator(body);
  },

  async getPrice(symbol: string): Promise<PriceEntry> {
    const body = await api.get(`/prices/${encodeURIComponent(symbol)}`, { authenticated: false });
    return priceListValidator([body])[0];
  },

  /** Quotes a swap through the TigerSwap router on the server side. */
  async quoteSwap(params: {
    tokenIn: string;
    tokenOut: string;
    amountIn: string;
    chainId: number;
    slippageBps?: number;
  }): Promise<SwapQuote> {
    const body = await api.post("/swap/quote", {
      token_in: params.tokenIn,
      token_out: params.tokenOut,
      amount_in: params.amountIn,
      chain_id: params.chainId,
      slippage_bps: params.slippageBps ?? 50,
    });
    return swapQuoteValidator(body);
  },

  /**
   * Builds an unsigned swap transaction.
   *
   * The server returns the calldata and target; the wallet signs it locally.
   * No key ever leaves the browser.
   */
  async buildSwap(params: {
    tokenIn: string;
    tokenOut: string;
    amountIn: string;
    chainId: number;
    recipient: string;
    slippageBps?: number;
  }): Promise<{ to: string; data: string; value: string; gasLimit: string | null }> {
    const body = await api.post("/swap/build", {
      token_in: params.tokenIn,
      token_out: params.tokenOut,
      amount_in: params.amountIn,
      chain_id: params.chainId,
      recipient: params.recipient,
      slippage_bps: params.slippageBps ?? 50,
    });

    const record = body as Record<string, unknown>;
    return {
      to: String(record.to ?? ""),
      data: String(record.data ?? "0x"),
      value: String(record.value ?? "0"),
      gasLimit: record.gas_limit ? String(record.gas_limit) : null,
    };
  },

  /** Authenticated user data. */
  async getProfile(): Promise<UserProfile> {
    const body = await api.get("/user/profile");
    return userProfileValidator(body);
  },

  async getPortfolio(): Promise<Portfolio> {
    const body = await api.get("/user/portfolio");
    return portfolioValidator(body);
  },

  async getTransactions(): Promise<UserTransaction[]> {
    const body = await api.get("/user/transactions");
    return userTransactionListValidator(body);
  },

  /** Admin dashboard. Requires an admin session. */
  async getAdminDashboard(): Promise<AdminDashboard> {
    const body = await api.get("/admin/dashboard");
    return adminDashboardValidator(body);
  },

  /** Admin: every configured chain, including inactive ones. */
  async getAdminChains(): Promise<AdminChain[]> {
    const body = await api.get("/admin/chains");
    return adminChainListValidator(body);
  },

  async createAdminChain(payload: Partial<AdminChain>): Promise<AdminChain> {
    const body = await api.post("/admin/chains", payload);
    return adminChainValidator(body);
  },

  async updateAdminChain(id: number, payload: Partial<AdminChain>): Promise<AdminChain> {
    const body = await api.put(`/admin/chains/${encodeURIComponent(id)}`, payload);
    return adminChainValidator(body);
  },

  async deleteAdminChain(id: number): Promise<void> {
    await apiRequest(`/admin/chains/${encodeURIComponent(id)}`, { method: "DELETE" });
  },

  /** Admin: every configured token, including inactive ones. */
  async getAdminTokens(): Promise<AdminToken[]> {
    const body = await api.get("/admin/tokens");
    return adminTokenListValidator(body);
  },

  async createAdminToken(payload: Partial<AdminToken>): Promise<AdminToken> {
    const body = await api.post("/admin/tokens", payload);
    return adminTokenValidator(body);
  },

  async updateAdminToken(id: number, payload: Partial<AdminToken>): Promise<AdminToken> {
    const body = await api.put(`/admin/tokens/${encodeURIComponent(id)}`, payload);
    return adminTokenValidator(body);
  },

  async deleteAdminToken(id: number): Promise<void> {
    await apiRequest(`/admin/tokens/${encodeURIComponent(id)}`, { method: "DELETE" });
  },

  /** Admin: persisted platform runtime switches. */
  async getAdminSettings(): Promise<AdminSettings> {
    const body = await api.get("/admin/settings");
    return adminSettingsValidator(body);
  },

  /** Admin: current fee configuration. */
  async getFeeConfig(): Promise<FeeConfig> {
    const body = await api.get("/admin/fees/config");
    return feeConfigValidator(body);
  },

  async updateFeeConfig(payload: Partial<FeeConfig>): Promise<void> {
    await api.post("/admin/fees/config", payload);
  },

  /** Admin: persist platform runtime switches. */
  async updateAdminSettings(payload: Partial<AdminSettings>): Promise<AdminSettings> {
    const body = await api.post("/admin/config/update", {
      trading_enabled: payload.trading_enabled,
      withdrawal_enabled: payload.withdrawal_enabled,
      deposit_enabled: payload.deposit_enabled,
      new_user_registration_enabled: payload.new_user_registration_enabled,
    });
    return adminSettingsValidator(body);
  },

  /** Admin: enter or leave maintenance mode. */
  async setMaintenance(enabled: boolean): Promise<void> {
    await api.post(enabled ? "/admin/maintenance/enable" : "/admin/maintenance/disable");
  },

  /** Admin: immutable audit log of operator actions. */
  async getAdminLogs(): Promise<AdminAuditLog[]> {
    const body = await api.get("/admin/logs");
    return adminAuditLogListValidator(body);
  },

  /** Public launchpad projects. */
  async getLaunchpadProjects(status?: string): Promise<LaunchpadProject[]> {
    const query = status ? `?status=${encodeURIComponent(status)}` : "";
    const body = await apiRequest(`/launchpad/projects${query}`, {
      authenticated: false,
      timeoutMs: 8000,
    });
    return launchpadProjectListValidator(body);
  },

  async createLaunchpadProject(payload: {
    name: string;
    token_address: string;
    token_symbol: string;
    token_name: string;
    total_supply: string;
    token_price: string;
    payment_token: string;
    start_time: string;
    end_time: string;
    description?: string;
    min_purchase?: string;
    max_purchase?: string;
    soft_cap?: string;
    hard_cap?: string;
    website_url?: string;
    whitepaper_url?: string;
    logo_url?: string;
  }): Promise<LaunchpadProject> {
    const body = await api.post("/launchpad/projects", payload);
    return launchpadProjectValidator(body);
  },

  /** Admin: move a launchpad project between lifecycle states. */
  async updateLaunchpadStatus(id: string, status: string): Promise<void> {
    await api.put(`/admin/launchpad/projects/${encodeURIComponent(id)}/status`, { status });
  },

  /** Perpetual markets. Returns an empty list when the venue is not enabled. */
  async getPerpMarkets(): Promise<PerpMarket[]> {
    const body = await apiRequest("/perpetuals/markets", { authenticated: false, timeoutMs: 8000 });
    return perpMarketListValidator(body);
  },

  /**
   * Active spot-margin markets. Only listings the operator has activated are
   * returned, so the margin panel offers exactly the pairs that can trade.
   */
  async getMarginMarkets(): Promise<MarginListing[]> {
    const body = await apiRequest("/margin/markets", { authenticated: false, timeoutMs: 8000 });
    return marginListingListValidator(body);
  },

  /** The authenticated caller's open and historical perpetual positions. */
  async getPerpPositions(): Promise<PerpPosition[]> {
    const body = await api.get("/wallet/perpetual/positions");
    const envelope = object<{ positions: PerpPosition[] }>({ positions: perpPositionListValidator })(body);
    return envelope.positions;
  },

  /**
   * Opens a perpetual position for the authenticated caller.
   *
   * The engine prices the position from its own oracle and persists it; the
   * response is the stored position.
   */
  async openPerpPosition(payload: {
    pair: string;
    side: 'long' | 'short';
    size: string;
    collateral: string;
    leverage: number;
  }): Promise<PerpOpenResult> {
    const body = await api.post("/wallet/perpetual/open", payload);
    return perpOpenResultValidator(body);
  },

  /** Closes a perpetual position owned by the authenticated caller. */
  async closePerpPosition(positionId: string): Promise<void> {
    await api.post("/wallet/perpetual/close", { position_id: positionId });
  },

  /** The authenticated caller's resting, filled and cancelled perpetual orders. */
  async getPerpOrders(): Promise<PerpOrder[]> {
    const body = await api.get("/perpetual/orders");
    const envelope = object<{ orders: PerpOrder[] }>({ orders: perpOrderListValidator })(body);
    return envelope.orders;
  },

  /** Places a resting limit or stop order for the authenticated caller. */
  async createPerpOrder(payload: {
    pair: string;
    side: 'long' | 'short';
    order_type: 'limit' | 'stop';
    size: string;
    price?: number;
    trigger_price?: number;
    expires_in: number;
  }): Promise<PerpOrder> {
    const body = await api.post("/perpetual/orders", payload);
    return perpOrderValidator(body);
  },

  /** Cancels one of the authenticated caller's own pending perpetual orders. */
  async cancelPerpOrder(orderId: string): Promise<void> {
    await api.delete(`/perpetual/orders/${encodeURIComponent(orderId)}`);
  },

  /** Opens a spot margin account for the authenticated caller. */
  async openMarginAccount(payload: { pair: string; margin_mode?: string }): Promise<MarginAccount> {
    const body = await api.post("/margin/accounts", payload);
    return marginAccountValidator(body);
  },

  /** Returns the risk view of a margin account the caller owns. */
  async getMarginAccount(accountId: string): Promise<MarginAccountView> {
    const body = await api.get(`/margin/accounts/${encodeURIComponent(accountId)}`);
    return marginAccountViewValidator(body);
  },

  /** Adds collateral to a margin account the caller owns. */
  async depositMarginCollateral(payload: {
    account_id: string;
    asset: string;
    amount: string;
  }): Promise<void> {
    await api.post("/margin/collateral", payload);
  },

  /** Borrows against a margin account the caller owns. */
  async borrowMargin(payload: { account_id: string; asset: string; amount: string }): Promise<void> {
    await api.post("/margin/borrow", payload);
  },

  /** Repays debt on a margin account the caller owns. */
  async repayMargin(payload: { account_id: string; asset: string; amount: string }): Promise<void> {
    await api.post("/margin/repay", payload);
  },

  /**
   * Admin listing management.
   *
   * These call the admin listing endpoints and validate the response. The
   * status of a new listing is decided by the server (new listings start
   * paused); the client never assumes otherwise.
   */
  /** The authenticated caller's platform ledger balances. */
  async getAccountBalances(): Promise<AccountBalance[]> {
    const body = await api.get("/wallet/balances");
    return accountBalanceListValidator(body);
  },

  async getFuturesMarkets(): Promise<FuturesMarket[]> {
    const body = await api.get("/admin/futures/markets");
    return futuresMarketListValidator(body);
  },

  async createFuturesMarket(payload: FuturesMarketInput): Promise<FuturesMarket> {
    const body = await api.post("/admin/futures/markets", payload);
    return futuresMarketValidator(body);
  },

  async updateFuturesMarket(id: string, payload: Partial<FuturesMarketInput>): Promise<FuturesMarket> {
    const body = await api.put(`/admin/futures/markets/${encodeURIComponent(id)}`, payload);
    return futuresMarketValidator(body);
  },

  async deleteFuturesMarket(id: string): Promise<void> {
    await apiRequest(`/admin/futures/markets/${encodeURIComponent(id)}`, { method: "DELETE" });
  },

  async getPerpsListings(): Promise<PerpsListing[]> {
    const body = await api.get("/admin/perps/markets");
    return perpsListingListValidator(body);
  },

  async createPerpsListing(payload: PerpsListingInput): Promise<PerpsListing> {
    const body = await api.post("/admin/perps/markets", payload);
    return perpsListingValidator(body);
  },

  async updatePerpsListing(id: string, payload: Partial<PerpsListingInput>): Promise<PerpsListing> {
    const body = await api.put(`/admin/perps/markets/${encodeURIComponent(id)}`, payload);
    return perpsListingValidator(body);
  },

  async deletePerpsListing(id: string): Promise<void> {
    await apiRequest(`/admin/perps/markets/${encodeURIComponent(id)}`, { method: "DELETE" });
  },

  async getMarginListings(): Promise<MarginListing[]> {
    const body = await api.get("/admin/margin/markets");
    return marginListingListValidator(body);
  },

  async createMarginListing(payload: MarginListingInput): Promise<MarginListing> {
    const body = await api.post("/admin/margin/markets", payload);
    return marginListingValidator(body);
  },

  async updateMarginListing(id: string, payload: Partial<MarginListingInput>): Promise<MarginListing> {
    const body = await api.put(`/admin/margin/markets/${encodeURIComponent(id)}`, payload);
    return marginListingValidator(body);
  },

  async deleteMarginListing(id: string): Promise<void> {
    await apiRequest(`/admin/margin/markets/${encodeURIComponent(id)}`, { method: "DELETE" });
  },

  /** Governance: veTGR locks, gauge voting and the bribe market. */

  async getLocks(): Promise<VeLock[]> {
    const body = await api.get("/governance/locks");
    return veLockListValidator(body);
  },

  async createLock(payload: { amount: string; duration_seconds: number }): Promise<VeLock> {
    const body = await api.post("/governance/locks", payload);
    return veLockValidator(body);
  },

  async withdrawLock(id: string): Promise<void> {
    await api.post(`/governance/locks/${encodeURIComponent(id)}/withdraw`, {});
  },

  async getGauges(activeOnly = false): Promise<Gauge[]> {
    const body = await api.get("/governance/gauges", { query: activeOnly ? { active: "true" } : undefined });
    return gaugeListValidator(body);
  },

  async vote(gaugeId: string, weight: string): Promise<{ gauge_id: string; weight: string; epoch: number }> {
    const body = await api.post("/governance/vote", { gauge_id: gaugeId, weight });
    return object<{ gauge_id: string; weight: string; epoch: number }>({
      gauge_id: string,
      weight: string,
      epoch: integer,
    })(body);
  },

  async getBribes(gaugeId: string): Promise<Bribe[]> {
    const body = await api.get("/governance/bribes", { query: { gauge_id: gaugeId } });
    return bribeListValidator(body);
  },

  async addBribe(payload: { gauge_id: string; token: string; amount: string }): Promise<Bribe> {
    const body = await api.post("/governance/bribes", payload);
    return bribeValidator(body);
  },

  async claimBribe(id: string): Promise<BribeClaimResult> {
    const body = await api.post(`/governance/bribes/${encodeURIComponent(id)}/claim`, {});
    return bribeClaimResultValidator(body);
  },

  async getBoost(stakedAmount: string): Promise<GovernanceBoost> {
    const body = await api.get("/governance/boost", { query: { staked_amount: stakedAmount } });
    return governanceBoostValidator(body);
  },

  /** Admin: gauge creation and emission weights. */

  async getAdminGauges(): Promise<Gauge[]> {
    const body = await api.get("/admin/gauges");
    return gaugeListValidator(body);
  },

  async createGauge(payload: { name: string; gauge_type: string; reference?: string }): Promise<Gauge> {
    const body = await api.post("/admin/gauges", payload);
    const envelope = object<{ gauge: Gauge }>({ gauge: gaugeValidator })(body);
    return envelope.gauge;
  },

  async setGaugeWeight(id: string, payload: { weight: string; active?: boolean }): Promise<void> {
    await api.put(`/admin/gauges/${encodeURIComponent(id)}/weight`, payload);
  },

  /** Trading bots: each strategy is an engine the Rust bots crate runs. */

  async listBots(params: { strategy?: string; status?: string } = {}): Promise<BotInstance[]> {
    const query: Record<string, string> = {};
    if (params.strategy) query.strategy = params.strategy;
    if (params.status) query.status = params.status;
    const body = await api.get("/bots", { query: Object.keys(query).length ? query : undefined });
    return botListValidator(body);
  },

  async getBot(id: string): Promise<BotInstance> {
    const body = await api.get(`/bots/${encodeURIComponent(id)}`);
    return botValidator(body);
  },

  async createBot(payload: {
    name: string;
    strategy: string;
    symbol: string;
    config: Record<string, unknown>;
    max_order_size: string;
    max_daily_loss: string;
    max_open_orders: number;
  }): Promise<BotInstance> {
    const body = await api.post("/bots", payload);
    return botValidator(body);
  },

  async setBotStatus(id: string, status: "active" | "paused" | "stopped"): Promise<BotInstance> {
    const body = await api.put(`/bots/${encodeURIComponent(id)}/status`, { status });
    return botValidator(body);
  },

  async deleteBot(id: string): Promise<void> {
    await apiRequest(`/bots/${encodeURIComponent(id)}`, { method: "DELETE" });
  },

  async getBotOrders(id: string, limit = 100): Promise<BotOrder[]> {
    const body = await api.get(`/bots/${encodeURIComponent(id)}/orders`, { query: { limit } });
    return botOrderListValidator(body);
  },

  async getBotPerformance(id: string): Promise<BotPerformance[]> {
    const body = await api.get(`/bots/${encodeURIComponent(id)}/performance`);
    return botPerformanceListValidator(body);
  },

  async getBotStrategies(): Promise<BotStrategy[]> {
    const body = await api.get("/bots/strategies");
    return botStrategyListValidator(body);
  },

  async getBotsSummary(): Promise<BotsSummary> {
    const body = await api.get("/bots/summary");
    return botsSummaryValidator(body);
  },

  /** OTC private block trades. Identity comes from the session token. */

  async listOtcOrders(status?: string): Promise<OtcOrder[]> {
    const body = await api.get("/otc/orders", {
      query: status ? { status } : undefined,
    });
    return otcOrderListValidator(body);
  },

  async listOpenOtcOrders(symbol?: string): Promise<OtcOrder[]> {
    const body = await api.get("/otc/orders/open", {
      query: symbol ? { symbol } : undefined,
    });
    return otcOrderListValidator(body);
  },

  async createOtcOrder(payload: {
    symbol: string;
    side: "buy" | "sell";
    quantity: string;
    price: string;
    min_fill?: string;
    expires_in_seconds: number;
  }): Promise<OtcOrder> {
    const body = await api.post("/otc/orders", payload);
    return otcOrderValidator(body);
  },

  async takeOtcOrder(id: string, quantity: string): Promise<OtcOrder> {
    const body = await api.post(`/otc/orders/${encodeURIComponent(id)}/take`, { quantity });
    return otcOrderValidator(body);
  },

  async cancelOtcOrder(id: string): Promise<void> {
    await apiRequest(`/otc/orders/${encodeURIComponent(id)}`, { method: "DELETE" });
  },

  /** TWAMM schedules: large orders sliced over their lifetime. */

  async listTwammOrders(): Promise<TwammOrder[]> {
    const body = await api.get("/twamm/orders");
    return twammOrderListValidator(body);
  },

  async createTwammOrder(payload: {
    symbol: string;
    side: "buy" | "sell";
    total_quantity: string;
    limit_price: string;
    duration_hours: number;
    start_delay_seconds: number;
  }): Promise<TwammOrder> {
    const body = await api.post("/twamm/orders", payload);
    return twammOrderValidator(body);
  },

  /** Copy trading: public leaderboard plus the caller's own follow state. */
  async listCopyTraders(limit = 20): Promise<CopyTrader[]> {
    const body = await api.get("/copy-trading/traders", { query: { limit } });
    return copyTraderListValidator(body);
  },

  async getCopyTrader(address: string): Promise<CopyTrader> {
    const body = await api.get(`/copy-trading/traders/${encodeURIComponent(address)}`);
    return copyTraderValidator(body);
  },

  /** TigerBridge: a quote is priced per token and per source chain. */
  async getBridgeQuote(payload: {
    src_chain: number;
    dst_chain: number;
    token: string;
    amount: string;
  }): Promise<BridgeQuote> {
    const body = await api.post("/bridge/quote", payload);
    return bridgeQuoteValidator(body);
  },

  async getBridgeChains(): Promise<number[]> {
    const body = await api.get("/bridge/chains");
    return bridgeChainsValidator(body);
  },

  /**
   * Locks the caller's tokens on the source chain via the TigerBridge
   * relayer. The sender is the caller's custody wallet, resolved from the
   * session, so it is not part of the request.
   */
  async initiateBridgeTransfer(payload: {
    src_chain: number;
    dst_chain: number;
    token: string;
    amount: string;
    recipient: string;
  }): Promise<BridgeTransfer> {
    const body = await api.post("/bridge/transfer", payload);
    return bridgeTransferValidator(body);
  },

  /** The caller's own bridge transfers. */
  async listBridgeTransfers(): Promise<BridgeTransfer[]> {
    const body = await api.get("/bridge/transfers");
    return bridgeTransferListValidator(body);
  },

  /** The caller's copied positions and their live PnL. */
  async getCopyPositions(): Promise<CopyPosition[]> {
    const body = await api.get("/copy-trading/positions");
    return copyPositionListValidator(body);
  },

  /** Starts copying a trader for the authenticated caller. */
  async followTrader(payload: {
    trader_address: string;
    copy_ratio?: number;
    stop_loss?: number;
    take_profit?: number;
  }): Promise<void> {
    await api.post("/copy-trading/follow", payload);
  },

  /** Stops copying the trader the caller currently follows. */
  async unfollowTrader(): Promise<void> {
    await api.delete("/copy-trading/follow");
  },

  async cancelTwammOrder(id: string): Promise<void> {
    await apiRequest(`/twamm/orders/${encodeURIComponent(id)}`, { method: "DELETE" });
  },

  /** Farms: liquidity mining with one or more reward streams. */

  /** Active farms a user can stake in, with their live APR. */
  async getFarms(): Promise<FarmView[]> {
    const body = await api.get("/farms", { authenticated: false });
    return farmViewListValidator(body);
  },

  /** Every farm, including paused and closed ones. */
  async getAllFarms(): Promise<FarmView[]> {
    const body = await api.get("/farms/all", { authenticated: false });
    return farmViewListValidator(body);
  },

  async getFarm(id: string): Promise<FarmView> {
    const body = await api.get(`/farms/${encodeURIComponent(id)}`, { authenticated: false });
    return farmViewValidator(body);
  },

  /** The caller's stakes with their pending rewards. */
  async getFarmStakes(): Promise<FarmStakeView[]> {
    const body = await api.get("/farms/stakes");
    return farmStakeViewListValidator(body);
  },

  /** Escrows `amount` of the farm's staking token. */
  async stakeFarm(id: string, amount: string): Promise<void> {
    await api.post(`/farms/${encodeURIComponent(id)}/stake`, { amount });
  },

  /** Returns staked tokens to the caller's balance. */
  async unstakeFarm(id: string, amount: string): Promise<void> {
    await api.post(`/farms/${encodeURIComponent(id)}/unstake`, { amount });
  },

  /** Settles accrued reward. An empty `tokenAddress` claims every stream. */
  async claimFarmRewards(id: string, tokenAddress?: string): Promise<FarmClaimResult> {
    const body = await api.post(
      `/farms/${encodeURIComponent(id)}/claim`,
      tokenAddress ? { token_address: tokenAddress } : {},
    );
    return farmClaimResultValidator(body);
  },

  /** Admin: create a farm. It starts paused until activated. */
  async createFarm(payload: {
    key: string;
    name: string;
    farm_type?: string;
    chain_id: number;
    staking_token_address: string;
    staking_token_symbol: string;
    staking_token_decimals: number;
    start_time: string;
    end_time: string;
    rewards: Array<{
      token_address: string;
      token_symbol: string;
      token_decimals: number;
      rate_per_second: string;
    }>;
  }): Promise<void> {
    await api.post("/admin/farms", payload);
  },

  /** Admin: edit a farm's parameters. */
  async updateFarm(
    id: string,
    payload: {
      name?: string;
      farm_type?: string;
      start_time?: string;
      end_time?: string;
      status?: string;
    },
  ): Promise<void> {
    await api.put(`/admin/farms/${encodeURIComponent(id)}`, payload);
  },

  /** Admin: close an empty farm. */
  async deleteFarm(id: string): Promise<void> {
    await apiRequest(`/admin/farms/${encodeURIComponent(id)}`, { method: "DELETE" });
  },

  /** Admin: move reward tokens from the caller's balance into the farm. */
  async fundFarm(id: string, tokenAddress: string, amount: string): Promise<void> {
    await api.post(`/admin/farms/${encodeURIComponent(id)}/fund`, {
      token_address: tokenAddress,
      amount,
    });
  },

  /** Airdrops: an admin allocates and funds, the user claims. */

  /** Campaigns currently open for claims. */
  async getAirdrops(): Promise<AirdropCampaign[]> {
    const body = await api.get("/airdrops", { authenticated: false });
    return airdropCampaignListValidator(body);
  },

  /** Every campaign, including paused and closed ones. */
  async getAllAirdrops(): Promise<AirdropCampaign[]> {
    const body = await api.get("/airdrops/all", { authenticated: false });
    return airdropCampaignListValidator(body);
  },

  /** The caller's airdrop entitlements. */
  async getMyAirdrops(): Promise<AirdropAllocationView[]> {
    const body = await api.get("/airdrops/allocations");
    return airdropAllocationViewListValidator(body);
  },

  /** Claims the caller's allocation in a campaign. */
  async claimAirdrop(id: string): Promise<AirdropClaimResult> {
    const body = await api.post(`/airdrops/${encodeURIComponent(id)}/claim`, {});
    return airdropClaimResultValidator(body);
  },

  /** Admin: open a campaign. It starts paused until funded and activated. */
  async createAirdrop(payload: {
    key: string;
    name: string;
    chain_id: number;
    token_address: string;
    token_symbol: string;
    token_decimals: number;
    start_time: string;
    end_time: string;
  }): Promise<AirdropCampaign> {
    const body = await api.post("/admin/airdrops", payload);
    return airdropCampaignValidator(body);
  },

  /** Admin: edit a campaign. */
  async updateAirdrop(
    id: string,
    payload: { name?: string; start_time?: string; end_time?: string; status?: string },
  ): Promise<void> {
    await api.put(`/admin/airdrops/${encodeURIComponent(id)}`, payload);
  },

  /** Admin: close or delete a campaign. */
  async deleteAirdrop(id: string): Promise<void> {
    await apiRequest(`/admin/airdrops/${encodeURIComponent(id)}`, { method: "DELETE" });
  },

  /** Admin: replace the campaign's allocation set. */
  async setAirdropAllocations(
    id: string,
    allocations: Array<{ user_id: string; amount: string }>,
  ): Promise<void> {
    await api.post(`/admin/airdrops/${encodeURIComponent(id)}/allocations`, { allocations });
  },

  /** Admin: move tokens from the caller's balance into the campaign escrow. */
  async fundAirdrop(id: string, amount: string): Promise<void> {
    await api.post(`/admin/airdrops/${encodeURIComponent(id)}/fund`, { amount });
  },

  /** Options: a self-built European option AMM settled in RUSD, USDT or USDC. */

  /** Tradable series with their live greeks and unit premium. */
  async getOptionSeries(): Promise<OptionSeriesView[]> {
    const body = await api.get("/options/series", { authenticated: false });
    return optionSeriesViewListValidator(body);
  },

  /** Every series, including paused and settled ones. */
  async getAllOptionSeries(): Promise<OptionSeriesView[]> {
    const body = await api.get("/options/series/all", { authenticated: false });
    return optionSeriesViewListValidator(body);
  },

  async getOptionSeriesById(id: string): Promise<OptionSeriesView> {
    const body = await api.get(`/options/series/${encodeURIComponent(id)}`, {
      authenticated: false,
    });
    return optionSeriesEnvelopeValidator(body);
  },

  /** The caller's open option positions. */
  async getOptionPositions(): Promise<OptionPositionView[]> {
    const body = await api.get("/options/positions");
    return optionPositionViewListValidator(body);
  },

  /** Buys `size` of a series, refusing a premium above `maxPremium`. */
  async buyOption(id: string, size: string, maxPremium: string): Promise<OptionBuyResult> {
    const body = await api.post(`/options/series/${encodeURIComponent(id)}/buy`, {
      size,
      max_premium: maxPremium,
    });
    return optionBuyResultValidator(body);
  },

  /** Deposits settlement collateral to underwrite a series. */
  async addOptionCollateral(id: string, amount: string): Promise<void> {
    await api.post(`/options/series/${encodeURIComponent(id)}/collateral`, { amount });
  },

  /** Withdraws collateral that is not backing an open position. */
  async removeOptionCollateral(id: string, amount: string): Promise<void> {
    await api.post(`/options/series/${encodeURIComponent(id)}/collateral/remove`, { amount });
  },

  /** Exercises a settled position, paying its intrinsic value. */
  async exerciseOption(id: string): Promise<OptionExerciseResult> {
    const body = await api.post(`/options/series/${encodeURIComponent(id)}/exercise`, {});
    return optionExerciseResultValidator(body);
  },

  /** Withdraws an LP's share of a settled series' remaining collateral. */
  async withdrawOptionLiquidity(id: string, amount: string): Promise<void> {
    await api.post(`/options/series/${encodeURIComponent(id)}/withdraw`, { amount });
  },

  /** Admin: create an option series. It starts paused until activated. */
  async createOptionSeries(payload: {
    key: string;
    name: string;
    chain_id: number;
    underlying_symbol: string;
    collateral_token_address: string;
    collateral_token_symbol: string;
    collateral_token_decimals: number;
    strike: string;
    base_iv: string;
    expiry: string;
    is_call: boolean;
    mark_price: string;
    status?: string;
  }): Promise<OptionSeriesView> {
    const body = await api.post("/admin/options/series", payload);
    return optionSeriesEnvelopeValidator(body);
  },

  /** Admin: edit a series' name, implied volatility, expiry or status. */
  async updateOptionSeries(
    id: string,
    payload: { name?: string; base_iv?: string; mark_price?: string; expiry?: string; status?: string },
  ): Promise<OptionSeriesView> {
    const body = await api.put(`/admin/options/series/${encodeURIComponent(id)}`, payload);
    return optionSeriesEnvelopeValidator(body);
  },

  /** Admin: close an empty series. */
  async deleteOptionSeries(id: string): Promise<void> {
    await apiRequest(`/admin/options/series/${encodeURIComponent(id)}`, { method: "DELETE" });
  },

  /** Admin: settle an expired series at a settlement price. */
  async settleOptionSeries(id: string, settlementPrice: string): Promise<OptionSeriesView> {
    const body = await api.post(`/admin/options/series/${encodeURIComponent(id)}/settle`, {
      settlement_price: settlementPrice,
    });
    return optionSeriesEnvelopeValidator(body);
  },

  /** Admin: refresh a series' mark price from the oracle. */
  async refreshOptionMarkPrice(id: string): Promise<OptionSeriesView> {
    const body = await api.post(`/admin/options/series/${encodeURIComponent(id)}/refresh-price`, {});
    return optionSeriesEnvelopeValidator(body);
  },

  /**
   * NFT marketplace: a self-built marketplace where a listing escrows the
   * token and an offer escrows the bid, so settlement cannot fail for a missing
   * asset or balance. Prices are exact integers in the payment token's smallest
   * unit.
   */

  /** Live listings, each joined with its token and collection. */
  async getNFTListings(): Promise<NFTListingView[]> {
    const body = await api.get("/nft/listings", { authenticated: false });
    return nftListingViewListValidator(body);
  },

  /** Active collections. */
  async getNFTCollections(): Promise<NFTCollection[]> {
    const body = await api.get("/nft/collections", { authenticated: false });
    return nftCollectionListValidator(body);
  },

  /** Live offers, optionally filtered to one token. */
  async getNFTOffers(collectionId?: string, tokenId?: number): Promise<NFTOfferView[]> {
    const params = new URLSearchParams();
    if (collectionId) {
      params.set("collection_id", collectionId);
    }
    if (tokenId !== undefined) {
      params.set("token_id", String(tokenId));
    }
    const query = params.toString();
    const body = await api.get(`/nft/offers${query ? `?${query}` : ""}`, {
      authenticated: false,
    });
    return nftOfferViewListValidator(body);
  },

  /** The marketplace fee configuration. */
  async getNFTConfig(): Promise<NFTMarketplaceConfig> {
    const body = await api.get("/nft/config", { authenticated: false });
    return nftMarketplaceConfigValidator(body);
  },

  /** The caller's own tokens. */
  async getMyNFTs(): Promise<NFTToken[]> {
    const body = await api.get("/nft/tokens/mine");
    return nftTokenListValidator(body);
  },

  /** The caller's own listings. */
  async getMyNFTListings(): Promise<NFTListing[]> {
    const body = await api.get("/nft/listings/mine");
    return nftListingListValidator(body);
  },

  /** The caller's own offers. */
  async getMyNFTOffers(): Promise<NFTOffer[]> {
    const body = await api.get("/nft/offers/mine");
    return nftOfferListValidator(body);
  },

  /** Lists an owned token for a fixed price, escrowing it. */
  async listNFT(payload: {
    collection_id: string;
    token_id: number;
    payment_token_address: string;
    payment_token_symbol: string;
    price: string;
    deadline: string;
  }): Promise<NFTListing> {
    const body = await api.post("/nft/listings", payload);
    return nftListingEnvelopeValidator(body);
  },

  /** Buys a live listing, paying the seller net of the frozen fee. */
  async buyNFT(id: string): Promise<NFTListing> {
    const body = await api.post(`/nft/listings/${encodeURIComponent(id)}/buy`, {});
    return nftListingEnvelopeValidator(body);
  },

  /** Cancels a listing, returning the escrowed token. */
  async cancelNFTListing(id: string): Promise<void> {
    await api.post(`/nft/listings/${encodeURIComponent(id)}/cancel`, {});
  },

  /** Re-prices a listing the caller owns. */
  async updateNFTListingPrice(id: string, price: string): Promise<NFTListing> {
    const body = await api.put(`/nft/listings/${encodeURIComponent(id)}`, { price });
    return nftListingEnvelopeValidator(body);
  },

  /** Opens an escrowed bid on a token. */
  async makeNFTOffer(payload: {
    collection_id: string;
    token_id: number;
    payment_token_address: string;
    payment_token_symbol: string;
    amount: string;
    deadline: string;
  }): Promise<NFTOffer> {
    const body = await api.post("/nft/offers", payload);
    return nftOfferEnvelopeValidator(body);
  },

  /** Accepts an escrowed bid, paying the seller net of the frozen fee. */
  async acceptNFTOffer(id: string, collectionId: string, tokenId: number): Promise<NFTOffer> {
    const params = new URLSearchParams({
      collection_id: collectionId,
      token_id: String(tokenId),
    });
    const body = await api.post(
      `/nft/offers/${encodeURIComponent(id)}/accept?${params.toString()}`,
      {},
    );
    return nftOfferEnvelopeValidator(body);
  },

  /** Cancels an offer, refunding the escrowed bid. */
  async cancelNFTOffer(id: string): Promise<void> {
    await api.post(`/nft/offers/${encodeURIComponent(id)}/cancel`, {});
  },

  /** Admin: create a collection. It starts paused until activated. */
  async createNFTCollection(payload: {
    key: string;
    name: string;
    symbol: string;
    chain_id: number;
    contract_address?: string;
    base_uri?: string;
    status?: string;
  }): Promise<NFTCollection> {
    const body = await api.post("/admin/nft/collections", payload);
    return nftCollectionEnvelopeValidator(body);
  },

  /** Admin: every collection, including paused ones. */
  async getAllNFTCollections(): Promise<NFTCollection[]> {
    const body = await api.get("/admin/nft/collections");
    return nftCollectionListValidator(body);
  },

  /** Admin: edit a collection. */
  async updateNFTCollection(
    id: string,
    payload: { name?: string; symbol?: string; contract_address?: string; base_uri?: string; status?: string },
  ): Promise<NFTCollection> {
    const body = await api.put(`/admin/nft/collections/${encodeURIComponent(id)}`, payload);
    return nftCollectionEnvelopeValidator(body);
  },

  /** Admin: delete an empty collection. */
  async deleteNFTCollection(id: string): Promise<void> {
    await apiRequest(`/admin/nft/collections/${encodeURIComponent(id)}`, { method: "DELETE" });
  },

  /** Admin: mint a token into an account. */
  async mintNFT(
    collectionId: string,
    payload: { uri: string; owner?: string; token_id?: number },
  ): Promise<NFTToken> {
    const body = await api.post(
      `/admin/nft/collections/${encodeURIComponent(collectionId)}/mint`,
      payload,
    );
    return nftTokenEnvelopeValidator(body);
  },

  /** Admin: every listing, including settled and cancelled ones. */
  async getAllNFTListings(): Promise<NFTListing[]> {
    const body = await api.get("/admin/nft/listings");
    return nftListingListValidator(body);
  },

  /** Admin: set the marketplace fee in basis points. */
  async updateNFTConfig(payload: { fee_bps?: number; fee_account?: string }): Promise<NFTMarketplaceConfig> {
    const body = await api.put("/admin/nft/config", payload);
    return nftMarketplaceConfigValidator(body);
  },

  /* ------------------------------------------------------------ analytics */

  /** The protocol dashboard snapshot: TVL, volume, fees, counts and open interest. */
  async getAnalyticsOverview(): Promise<ProtocolOverview> {
    const body = await api.get("/analytics/overview", { authenticated: false });
    return protocolOverviewValidator(body);
  },

  /**
   * A volume series for one market (or every market when `pair` is omitted).
   * `interval` and `window` are in seconds; the server requires `interval` to be
   * a whole multiple of its rollup bucket.
   */
  async getAnalyticsVolume(params: {
    pair?: string;
    chainId?: number;
    interval?: number;
    window?: number;
  } = {}): Promise<VolumeSeriesResponse> {
    const body = await api.get("/analytics/volume", {
      authenticated: false,
      query: {
        pair: params.pair,
        chain_id: params.chainId,
        interval: params.interval,
        window: params.window,
      },
    });
    return volumeSeriesValidator(body);
  },

  /** The per-market volume leaderboard over a window. */
  async getAnalyticsPairs(params: { chainId?: number; window?: number; limit?: number } = {}): Promise<
    PairAnalytics[]
  > {
    const body = await api.get("/analytics/pairs", {
      authenticated: false,
      query: { chain_id: params.chainId, window: params.window, limit: params.limit },
    });
    return pairAnalyticsListValidator(body);
  },

  /** The realized-PnL leaderboard. `window` in seconds; 0 means all time. */
  async getAnalyticsTraders(params: { window?: number; limit?: number } = {}): Promise<TopTrader[]> {
    const body = await api.get("/analytics/traders", {
      authenticated: false,
      query: { window: params.window, limit: params.limit },
    });
    return topTraderListValidator(body);
  },

  /** Admin: rebuild the volume rollup for a window. Idempotent. */
  async runAnalyticsRollup(windowSeconds = 3600): Promise<AnalyticsRollupResult> {
    const body = await api.post("/admin/analytics/rollup", undefined, {
      query: { window: windowSeconds },
    });
    return analyticsRollupValidator(body);
  },
};
