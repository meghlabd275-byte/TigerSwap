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
  OtcOrder,
  PerpMarket,
  PerpOpenResult,
  PerpPosition,
  PerpsListing,
  PerpsListingInput,
  Pool,
  Portfolio,
  PriceEntry,
  SwapQuote,
  Token,
  Trade,
  TwammOrder,
  UserProfile,
  UserTransaction,
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
  otcOrderListValidator,
  otcOrderValidator,
  perpMarketListValidator,
  perpOpenResultValidator,
  perpPositionListValidator,
  perpsListingListValidator,
  perpsListingValidator,
  poolListValidator,
  portfolioValidator,
  priceListValidator,
  swapQuoteValidator,
  tokenListValidator,
  tradeListValidator,
  twammOrderListValidator,
  twammOrderValidator,
  userProfileValidator,
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
    creator: string;
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
};
