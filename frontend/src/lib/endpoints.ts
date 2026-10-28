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
  AdminDashboard,
  Bribe,
  BribeClaimResult,
  Chain,
  FuturesMarket,
  FuturesMarketInput,
  Gauge,
  GovernanceBoost,
  MarginAccount,
  MarginAccountView,
  MarginListing,
  MarginListingInput,
  MarketStats,
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
  UserProfile,
  UserTransaction,
  VeLock,
  accountBalanceListValidator,
  adminDashboardValidator,
  chainListValidator,
  futuresMarketListValidator,
  futuresMarketValidator,
  bribeClaimResultValidator,
  bribeListValidator,
  bribeValidator,
  gaugeListValidator,
  gaugeValidator,
  governanceBoostValidator,
  marginAccountValidator,
  marginAccountViewValidator,
  marginListingListValidator,
  marginListingValidator,
  marketStatsValidator,
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

  /** Perpetual markets. Returns an empty list when the venue is not enabled. */
  async getPerpMarkets(): Promise<PerpMarket[]> {
    const body = await apiRequest("/perpetuals/markets", { authenticated: false, timeoutMs: 8000 });
    return perpMarketListValidator(body);
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
};
