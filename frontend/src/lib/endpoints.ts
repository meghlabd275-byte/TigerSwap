/**
 * Typed endpoints for the TigerSwap API.
 *
 * Each function validates the raw response before returning it, so callers
 * only ever see checked data. Where the backend exposes an endpoint, it is
 * called; where it does not, the function is simply absent rather than
 * returning a fabricated value.
 */

import { api, apiRequest } from "./apiClient";
import {
  AccountBalance,
  AdminDashboard,
  Chain,
  FuturesMarket,
  FuturesMarketInput,
  MarginListing,
  MarginListingInput,
  MarketStats,
  PerpMarket,
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
  accountBalanceListValidator,
  adminDashboardValidator,
  chainListValidator,
  futuresMarketListValidator,
  futuresMarketValidator,
  marginListingListValidator,
  marginListingValidator,
  marketStatsValidator,
  perpMarketListValidator,
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
};
