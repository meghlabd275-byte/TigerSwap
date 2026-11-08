/**
 * Data hooks backed by react-query.
 *
 * Every hook fetches from a real endpoint through `endpoints.ts`, so the UI
 * shows live data or an explicit error state. There are no fallbacks to
 * fabricated values: when a request fails the hook reports the failure and the
 * component renders an error.
 */

"use client";

import { useMutation, useQuery, useQueryClient, type UseQueryResult } from "@tanstack/react-query";

import { ApiError } from "@/lib/apiClient";
import { endpoints } from "@/lib/endpoints";
import { isAdmin, isAuthenticated } from "@/lib/auth";
import type {
  AdminAuditLog,
  AirdropAllocationView,
  AirdropCampaign,
  AirdropClaimResult,
  AdminChain,
  AdminDashboard,
  AdminSettings,
  AdminToken,
  BotInstance,
  BotOrder,
  BotPerformance,
  BotsSummary,
  BotStrategy,
  Chain,
  FarmClaimResult,
  FarmStakeView,
  FarmView,
  FeeConfig,
  LaunchpadProject,
  MarketStats,
  MarginAccountView,
  MarginListing,
  NFTCollection,
  NFTListing,
  NFTListingView,
  NFTMarketplaceConfig,
  NFTOffer,
  NFTOfferView,
  NFTToken,
  OptionBuyResult,
  OptionExerciseResult,
  OptionPositionView,
  OptionSeriesView,
  OtcOrder,
  PerpMarket,
  PerpPosition,
  Pool,
  PairAnalytics,
  ProtocolOverview,
  Portfolio,
  PriceEntry,
  SwapQuote,
  Token,
  TopTrader,
  Trade,
  TwammOrder,
  UserTransaction,
  VolumeSeriesResponse,
} from "@/lib/types";

/** Refetch cadences, in milliseconds. */
export const REFRESH = {
  fast: 5_000,
  normal: 15_000,
  slow: 60_000,
  static: 10 * 60_000,
} as const;

/**
 * A query is retried only for transient failures.
 *
 * Retrying a 401, 403 or 404 is pointless and delays the error state the user
 * needs to see.
 */
function shouldRetry(failureCount: number, error: unknown): boolean {
  if (error instanceof ApiError && error.status >= 400 && error.status < 500) {
    return false;
  }
  return failureCount < 2;
}

export function useTokens(chainId?: number): UseQueryResult<Token[], Error> {
  return useQuery<Token[], Error>({
    queryKey: ["tokens", chainId ?? "all"],
    queryFn: () => endpoints.getTokens(chainId),
    staleTime: REFRESH.slow,
    retry: shouldRetry,
  });
}

export function useChains(): UseQueryResult<Chain[], Error> {
  return useQuery<Chain[], Error>({
    queryKey: ["chains"],
    queryFn: () => endpoints.getChains(),
    staleTime: REFRESH.static,
    retry: shouldRetry,
  });
}

export function useMarketStats(): UseQueryResult<MarketStats, Error> {
  return useQuery<MarketStats, Error>({
    queryKey: ["market", "stats"],
    queryFn: () => endpoints.getMarketStats(),
    refetchInterval: REFRESH.normal,
    retry: shouldRetry,
  });
}

export function useRecentTrades(limit = 50): UseQueryResult<Trade[], Error> {
  return useQuery<Trade[], Error>({
    queryKey: ["market", "trades", limit],
    queryFn: () => endpoints.getRecentTrades(limit),
    refetchInterval: REFRESH.fast,
    retry: shouldRetry,
  });
}

export function usePools(): UseQueryResult<Pool[], Error> {
  return useQuery<Pool[], Error>({
    queryKey: ["pools"],
    queryFn: () => endpoints.getPairs(),
    refetchInterval: REFRESH.normal,
    retry: shouldRetry,
  });
}

export function usePrices(): UseQueryResult<PriceEntry[], Error> {
  return useQuery<PriceEntry[], Error>({
    queryKey: ["prices"],
    queryFn: () => endpoints.getPrices(),
    refetchInterval: REFRESH.normal,
    retry: shouldRetry,
  });
}

export function usePortfolio(enabled = true): UseQueryResult<Portfolio, Error> {
  return useQuery<Portfolio, Error>({
    queryKey: ["portfolio"],
    queryFn: () => endpoints.getPortfolio(),
    enabled: enabled && isAuthenticated(),
    refetchInterval: REFRESH.normal,
    retry: shouldRetry,
  });
}

export function useTransactions(): UseQueryResult<UserTransaction[], Error> {
  return useQuery<UserTransaction[], Error>({
    queryKey: ["transactions"],
    queryFn: () => endpoints.getTransactions(),
    enabled: isAuthenticated(),
    retry: shouldRetry,
  });
}

export function useAdminDashboard(): UseQueryResult<AdminDashboard, Error> {
  return useQuery<AdminDashboard, Error>({
    queryKey: ["admin", "dashboard"],
    queryFn: () => endpoints.getAdminDashboard(),
    enabled: isAdmin(),
    refetchInterval: REFRESH.normal,
    retry: shouldRetry,
  });
}

/** Admin: every configured chain, including inactive ones. */
export function useAdminChains(): UseQueryResult<AdminChain[], Error> {
  return useQuery<AdminChain[], Error>({
    queryKey: ["admin", "chains"],
    queryFn: () => endpoints.getAdminChains(),
    enabled: isAdmin(),
    refetchInterval: REFRESH.slow,
    retry: shouldRetry,
  });
}

/** Admin: every configured token, including inactive ones. */
export function useAdminTokens(): UseQueryResult<AdminToken[], Error> {
  return useQuery<AdminToken[], Error>({
    queryKey: ["admin", "tokens"],
    queryFn: () => endpoints.getAdminTokens(),
    enabled: isAdmin(),
    refetchInterval: REFRESH.slow,
    retry: shouldRetry,
  });
}

/** Admin: persisted platform runtime switches. */
export function useAdminSettings(): UseQueryResult<AdminSettings, Error> {
  return useQuery<AdminSettings, Error>({
    queryKey: ["admin", "settings"],
    queryFn: () => endpoints.getAdminSettings(),
    enabled: isAdmin(),
    refetchInterval: REFRESH.normal,
    retry: shouldRetry,
  });
}

/** Admin: current fee configuration. */
export function useFeeConfig(): UseQueryResult<FeeConfig, Error> {
  return useQuery<FeeConfig, Error>({
    queryKey: ["admin", "fees"],
    queryFn: () => endpoints.getFeeConfig(),
    enabled: isAdmin(),
    refetchInterval: REFRESH.slow,
    retry: shouldRetry,
  });
}

/** Admin: immutable audit log of operator actions. */
export function useAdminLogs(): UseQueryResult<AdminAuditLog[], Error> {
  return useQuery<AdminAuditLog[], Error>({
    queryKey: ["admin", "logs"],
    queryFn: () => endpoints.getAdminLogs(),
    enabled: isAdmin(),
    refetchInterval: REFRESH.normal,
    retry: shouldRetry,
  });
}

/** Public launchpad projects. */
export function useLaunchpadProjects(status?: string): UseQueryResult<LaunchpadProject[], Error> {
  return useQuery<LaunchpadProject[], Error>({
    queryKey: ["launchpad", "projects", status ?? "all"],
    queryFn: () => endpoints.getLaunchpadProjects(status),
    refetchInterval: REFRESH.slow,
    retry: shouldRetry,
  });
}

export function usePerpMarkets(): UseQueryResult<PerpMarket[], Error> {
  return useQuery<PerpMarket[], Error>({
    queryKey: ["perps", "markets"],
    queryFn: () => endpoints.getPerpMarkets(),
    refetchInterval: REFRESH.fast,
    retry: shouldRetry,
  });
}

/** Active spot-margin markets offered by the venue. */
export function useMarginMarkets(): UseQueryResult<MarginListing[], Error> {
  return useQuery<MarginListing[], Error>({
    queryKey: ["margin", "markets"],
    queryFn: () => endpoints.getMarginMarkets(),
    refetchInterval: REFRESH.normal,
    retry: shouldRetry,
  });
}

/** The authenticated caller's perpetual positions. */
export function usePerpPositions(enabled = true): UseQueryResult<PerpPosition[], Error> {
  return useQuery<PerpPosition[], Error>({
    queryKey: ["perps", "positions"],
    queryFn: () => endpoints.getPerpPositions(),
    enabled: enabled && isAuthenticated(),
    refetchInterval: REFRESH.fast,
    retry: shouldRetry,
  });
}

/**
 * Opens a perpetual position.
 *
 * On success the positions list and the portfolio are refetched, because both
 * change when a position opens.
 */
export function useOpenPerpPosition() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (params: {
      pair: string;
      side: 'long' | 'short';
      size: string;
      collateral: string;
      leverage: number;
    }) => endpoints.openPerpPosition(params),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["perps", "positions"] });
      void queryClient.invalidateQueries({ queryKey: ["portfolio"] });
    },
  });
}

/** Closes a perpetual position and refreshes the positions list. */
export function useClosePerpPosition() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (positionId: string) => endpoints.closePerpPosition(positionId),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["perps", "positions"] });
      void queryClient.invalidateQueries({ queryKey: ["portfolio"] });
    },
  });
}

/** The risk view of one margin account, refreshed while it is open. */
export function useMarginAccount(accountId: string | null): UseQueryResult<MarginAccountView, Error> {
  return useQuery<MarginAccountView, Error>({
    queryKey: ["margin", "account", accountId],
    queryFn: () => {
      if (!accountId) {
        throw new Error("an account id is required");
      }
      return endpoints.getMarginAccount(accountId);
    },
    enabled: isAuthenticated() && accountId !== null,
    refetchInterval: REFRESH.fast,
    retry: shouldRetry,
  });
}

/** Opens a margin account. */
export function useOpenMarginAccount() {
  return useMutation({
    mutationFn: (payload: { pair: string; margin_mode?: string }) =>
      endpoints.openMarginAccount(payload),
  });
}

/** Deposits collateral into a margin account. */
export function useDepositMarginCollateral() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (payload: { account_id: string; asset: string; amount: string }) =>
      endpoints.depositMarginCollateral(payload),
    onSuccess: (_data, variables) => {
      void queryClient.invalidateQueries({ queryKey: ["margin", "account", variables.account_id] });
    },
  });
}

/** Borrows against a margin account's collateral. */
export function useBorrowMargin() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (payload: { account_id: string; asset: string; amount: string }) =>
      endpoints.borrowMargin(payload),
    onSuccess: (_data, variables) => {
      void queryClient.invalidateQueries({ queryKey: ["margin", "account", variables.account_id] });
    },
  });
}

/** Repays debt on a margin account. */
export function useRepayMargin() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (payload: { account_id: string; asset: string; amount: string }) =>
      endpoints.repayMargin(payload),
    onSuccess: (_data, variables) => {
      void queryClient.invalidateQueries({ queryKey: ["margin", "account", variables.account_id] });
    },
  });
}

/**
 * Requests a swap quote.
 *
 * `enabled` lets the caller hold the request until the inputs are valid, so an
 * incomplete form never fires a request.
 */
export function useSwapQuote(
  params: { tokenIn: string; tokenOut: string; amountIn: string; chainId: number } | null,
  enabled: boolean,
): UseQueryResult<SwapQuote, Error> {
  return useQuery<SwapQuote, Error>({
    queryKey: ["swap", "quote", params],
    queryFn: () => {
      if (!params) {
        throw new Error("quote parameters are required");
      }
      return endpoints.quoteSwap(params);
    },
    enabled: enabled && params !== null,
    staleTime: REFRESH.fast,
    retry: shouldRetry,
  });
}

/**
 * Builds and sends a swap.
 *
 * The transaction is built server-side but signed and broadcast by the user's
 * wallet. The mutation returns the transaction hash on success.
 */
export function useExecuteSwap() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (params: {
      tokenIn: string;
      tokenOut: string;
      amountIn: string;
      chainId: number;
      recipient: string;
      slippageBps?: number;
      send: (transaction: { to: string; data: string; value: string }) => Promise<string>;
    }) => {
      const transaction = await endpoints.buildSwap({
        tokenIn: params.tokenIn,
        tokenOut: params.tokenOut,
        amountIn: params.amountIn,
        chainId: params.chainId,
        recipient: params.recipient,
        slippageBps: params.slippageBps,
      });

      // Signing happens in the wallet, never on the server.
      return params.send({
        to: transaction.to,
        data: transaction.data,
        value: transaction.value,
      });
    },
    onSuccess: () => {
      // Balances and history have changed.
      void queryClient.invalidateQueries({ queryKey: ["portfolio"] });
      void queryClient.invalidateQueries({ queryKey: ["transactions"] });
    },
  });
}

/** The authenticated caller's configured trading bots. */
export function useBots(
  filters: { strategy?: string; status?: string } = {},
): UseQueryResult<BotInstance[], Error> {
  return useQuery<BotInstance[], Error>({
    queryKey: ["bots", filters.strategy ?? "all", filters.status ?? "all"],
    queryFn: () => endpoints.listBots(filters),
    enabled: isAuthenticated(),
    refetchInterval: REFRESH.normal,
    retry: shouldRetry,
  });
}

/** Counts of the caller's bots, grouped by strategy and status. */
export function useBotsSummary(): UseQueryResult<BotsSummary, Error> {
  return useQuery<BotsSummary, Error>({
    queryKey: ["bots", "summary"],
    queryFn: () => endpoints.getBotsSummary(),
    enabled: isAuthenticated(),
    refetchInterval: REFRESH.normal,
    retry: shouldRetry,
  });
}

/** The strategies a client may configure, with their accepted config keys. */
export function useBotStrategies(): UseQueryResult<BotStrategy[], Error> {
  return useQuery<BotStrategy[], Error>({
    queryKey: ["bots", "strategies"],
    queryFn: () => endpoints.getBotStrategies(),
    enabled: isAuthenticated(),
    staleTime: REFRESH.static,
    retry: shouldRetry,
  });
}

/** Creates a bot and refreshes the list and the summary. */
export function useCreateBot() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (payload: {
      name: string;
      strategy: string;
      symbol: string;
      config: Record<string, unknown>;
      max_order_size: string;
      max_daily_loss: string;
      max_open_orders: number;
    }) => endpoints.createBot(payload),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["bots"] });
    },
  });
}

/** Changes a bot's lifecycle status. */
export function useSetBotStatus() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (params: { id: string; status: "active" | "paused" | "stopped" }) =>
      endpoints.setBotStatus(params.id, params.status),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["bots"] });
    },
  });
}

/** Deletes a paused or stopped bot. */
export function useDeleteBot() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => endpoints.deleteBot(id),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["bots"] });
    },
  });
}

/** The orders a bot decided to place. */
export function useBotOrders(botId: string | null): UseQueryResult<BotOrder[], Error> {
  return useQuery<BotOrder[], Error>({
    queryKey: ["bots", botId, "orders"],
    queryFn: () => {
      if (!botId) throw new Error("a bot id is required");
      return endpoints.getBotOrders(botId);
    },
    enabled: isAuthenticated() && botId !== null,
    refetchInterval: REFRESH.normal,
    retry: shouldRetry,
  });
}

/** The daily realised performance of a bot. */
export function useBotPerformance(botId: string | null): UseQueryResult<BotPerformance[], Error> {
  return useQuery<BotPerformance[], Error>({
    queryKey: ["bots", botId, "performance"],
    queryFn: () => {
      if (!botId) throw new Error("a bot id is required");
      return endpoints.getBotPerformance(botId);
    },
    enabled: isAuthenticated() && botId !== null,
    refetchInterval: REFRESH.slow,
    retry: shouldRetry,
  });
}

/** The caller's own OTC block trades. */
export function useOtcOrders(status?: string): UseQueryResult<OtcOrder[], Error> {
  return useQuery<OtcOrder[], Error>({
    queryKey: ["otc", "orders", status ?? "all"],
    queryFn: () => endpoints.listOtcOrders(status),
    enabled: isAuthenticated(),
    refetchInterval: REFRESH.normal,
    retry: shouldRetry,
  });
}

/** Open OTC block trades the caller could take. */
export function useOpenOtcOrders(symbol?: string): UseQueryResult<OtcOrder[], Error> {
  return useQuery<OtcOrder[], Error>({
    queryKey: ["otc", "open", symbol ?? "all"],
    queryFn: () => endpoints.listOpenOtcOrders(symbol),
    enabled: isAuthenticated(),
    refetchInterval: REFRESH.normal,
    retry: shouldRetry,
  });
}

/** Publishes a private OTC block trade. */
export function useCreateOtcOrder() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (payload: {
      symbol: string;
      side: "buy" | "sell";
      quantity: string;
      price: string;
      min_fill?: string;
      expires_in_seconds: number;
    }) => endpoints.createOtcOrder(payload),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["otc"] });
    },
  });
}

/** Fills an open OTC block trade. */
export function useTakeOtcOrder() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (params: { id: string; quantity: string }) =>
      endpoints.takeOtcOrder(params.id, params.quantity),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["otc"] });
    },
  });
}

/** Cancels one of the caller's open OTC block trades. */
export function useCancelOtcOrder() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => endpoints.cancelOtcOrder(id),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["otc"] });
    },
  });
}

/** The caller's TWAMM schedules. */
export function useTwammOrders(): UseQueryResult<TwammOrder[], Error> {
  return useQuery<TwammOrder[], Error>({
    queryKey: ["twamm", "orders"],
    queryFn: () => endpoints.listTwammOrders(),
    enabled: isAuthenticated(),
    refetchInterval: REFRESH.normal,
    retry: shouldRetry,
  });
}

/** Schedules a TWAMM order. */
export function useCreateTwammOrder() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (payload: {
      symbol: string;
      side: "buy" | "sell";
      total_quantity: string;
      limit_price: string;
      duration_hours: number;
      start_delay_seconds: number;
    }) => endpoints.createTwammOrder(payload),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["twamm"] });
    },
  });
}

/** Cancels a TWAMM order that has not finished. */
export function useCancelTwammOrder() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => endpoints.cancelTwammOrder(id),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["twamm"] });
    },
  });
}

/* ------------------------------------------------------------------ farms */

/** Active farms a user can stake in, with their live APR. */
export function useFarms(): UseQueryResult<FarmView[], Error> {
  return useQuery<FarmView[], Error>({
    queryKey: ["farms", "active"],
    queryFn: () => endpoints.getFarms(),
    refetchInterval: REFRESH.normal,
    retry: shouldRetry,
  });
}

/** The authenticated caller's stakes and their pending rewards. */
export function useFarmStakes(): UseQueryResult<FarmStakeView[], Error> {
  return useQuery<FarmStakeView[], Error>({
    queryKey: ["farms", "stakes"],
    queryFn: () => endpoints.getFarmStakes(),
    enabled: isAuthenticated(),
    refetchInterval: REFRESH.normal,
    retry: shouldRetry,
  });
}

/** Escrows staking tokens into a farm. */
export function useStakeFarm() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (params: { id: string; amount: string }) => endpoints.stakeFarm(params.id, params.amount),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["farms"] });
      void queryClient.invalidateQueries({ queryKey: ["balances"] });
    },
  });
}

/** Returns staked tokens to the caller's balance. */
export function useUnstakeFarm() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (params: { id: string; amount: string }) => endpoints.unstakeFarm(params.id, params.amount),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["farms"] });
      void queryClient.invalidateQueries({ queryKey: ["balances"] });
    },
  });
}

/** Settles accrued farm reward into the caller's balance. */
export function useClaimFarmRewards() {
  const queryClient = useQueryClient();
  return useMutation<FarmClaimResult, Error, { id: string; tokenAddress?: string }>({
    mutationFn: (params) => endpoints.claimFarmRewards(params.id, params.tokenAddress),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["farms"] });
      void queryClient.invalidateQueries({ queryKey: ["balances"] });
    },
  });
}

/* --------------------------------------------------------------- airdrops */

/** Airdrop campaigns currently open for claims. */
export function useAirdrops(): UseQueryResult<AirdropCampaign[], Error> {
  return useQuery<AirdropCampaign[], Error>({
    queryKey: ["airdrops", "active"],
    queryFn: () => endpoints.getAirdrops(),
    refetchInterval: REFRESH.normal,
    retry: shouldRetry,
  });
}

/** The authenticated caller's airdrop entitlements. */
export function useMyAirdrops(): UseQueryResult<AirdropAllocationView[], Error> {
  return useQuery<AirdropAllocationView[], Error>({
    queryKey: ["airdrops", "allocations"],
    queryFn: () => endpoints.getMyAirdrops(),
    enabled: isAuthenticated(),
    refetchInterval: REFRESH.normal,
    retry: shouldRetry,
  });
}

/** Claims the caller's allocation in a campaign. */
export function useClaimAirdrop() {
  const queryClient = useQueryClient();
  return useMutation<AirdropClaimResult, Error, string>({
    mutationFn: (id: string) => endpoints.claimAirdrop(id),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["airdrops"] });
      void queryClient.invalidateQueries({ queryKey: ["balances"] });
    },
  });
}

/* --------------------------------------------------------------- options */

/** Tradable option series with their live greeks. */
export function useOptionSeries(): UseQueryResult<OptionSeriesView[], Error> {
  return useQuery<OptionSeriesView[], Error>({
    queryKey: ["options", "series"],
    queryFn: () => endpoints.getOptionSeries(),
    refetchInterval: REFRESH.normal,
    retry: shouldRetry,
  });
}

/** Every option series, including paused and settled ones (admin view). */
export function useAllOptionSeries(enabled = true): UseQueryResult<OptionSeriesView[], Error> {
  return useQuery<OptionSeriesView[], Error>({
    queryKey: ["options", "series", "all"],
    queryFn: () => endpoints.getAllOptionSeries(),
    enabled,
    refetchInterval: REFRESH.normal,
    retry: shouldRetry,
  });
}

/** The authenticated caller's open option positions. */
export function useOptionPositions(): UseQueryResult<OptionPositionView[], Error> {
  return useQuery<OptionPositionView[], Error>({
    queryKey: ["options", "positions"],
    queryFn: () => endpoints.getOptionPositions(),
    enabled: isAuthenticated(),
    refetchInterval: REFRESH.normal,
    retry: shouldRetry,
  });
}

/** Buys a long option position, capping the premium paid. */
export function useBuyOption() {
  const queryClient = useQueryClient();
  return useMutation<OptionBuyResult, Error, { id: string; size: string; maxPremium: string }>({
    mutationFn: (params) => endpoints.buyOption(params.id, params.size, params.maxPremium),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["options"] });
      void queryClient.invalidateQueries({ queryKey: ["balances"] });
    },
  });
}

/** Deposits settlement collateral into a series. */
export function useAddOptionCollateral() {
  const queryClient = useQueryClient();
  return useMutation<void, Error, { id: string; amount: string }>({
    mutationFn: (params) => endpoints.addOptionCollateral(params.id, params.amount),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["options"] });
      void queryClient.invalidateQueries({ queryKey: ["balances"] });
    },
  });
}

/** Withdraws collateral that is not backing an open position. */
export function useRemoveOptionCollateral() {
  const queryClient = useQueryClient();
  return useMutation<void, Error, { id: string; amount: string }>({
    mutationFn: (params) => endpoints.removeOptionCollateral(params.id, params.amount),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["options"] });
      void queryClient.invalidateQueries({ queryKey: ["balances"] });
    },
  });
}

/** Exercises a settled position, paying its intrinsic value. */
export function useExerciseOption() {
  const queryClient = useQueryClient();
  return useMutation<OptionExerciseResult, Error, string>({
    mutationFn: (id: string) => endpoints.exerciseOption(id),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["options"] });
      void queryClient.invalidateQueries({ queryKey: ["balances"] });
    },
  });
}

/** Withdraws an LP's share of a settled series' remaining collateral. */
export function useWithdrawOptionLiquidity() {
  const queryClient = useQueryClient();
  return useMutation<void, Error, { id: string; amount: string }>({
    mutationFn: (params) => endpoints.withdrawOptionLiquidity(params.id, params.amount),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["options"] });
      void queryClient.invalidateQueries({ queryKey: ["balances"] });
    },
  });
}

/** Admin: creates an option series. */
export function useCreateOptionSeries() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (payload: Parameters<typeof endpoints.createOptionSeries>[0]) =>
      endpoints.createOptionSeries(payload),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["options"] });
    },
  });
}

/** Admin: edits a series. */
export function useUpdateOptionSeries() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (params: { id: string; payload: Parameters<typeof endpoints.updateOptionSeries>[1] }) =>
      endpoints.updateOptionSeries(params.id, params.payload),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["options"] });
    },
  });
}

/** Admin: closes an empty series. */
export function useDeleteOptionSeries() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => endpoints.deleteOptionSeries(id),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["options"] });
    },
  });
}

/** Admin: settles an expired series at a settlement price. */
export function useSettleOptionSeries() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (params: { id: string; settlementPrice: string }) =>
      endpoints.settleOptionSeries(params.id, params.settlementPrice),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["options"] });
    },
  });
}

/** Admin: refreshes a series' mark price from the oracle. */
export function useRefreshOptionMarkPrice() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => endpoints.refreshOptionMarkPrice(id),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["options"] });
    },
  });
}

/* ------------------------------------------------------------------- nft */

/** Live NFT listings, each joined with its token and collection. */
export function useNFTListings(): UseQueryResult<NFTListingView[], Error> {
  return useQuery<NFTListingView[], Error>({
    queryKey: ["nft", "listings"],
    queryFn: () => endpoints.getNFTListings(),
    refetchInterval: REFRESH.normal,
    retry: shouldRetry,
  });
}

/** Active NFT collections. */
export function useNFTCollections(): UseQueryResult<NFTCollection[], Error> {
  return useQuery<NFTCollection[], Error>({
    queryKey: ["nft", "collections"],
    queryFn: () => endpoints.getNFTCollections(),
    staleTime: REFRESH.slow,
    retry: shouldRetry,
  });
}

/** Live offers, optionally filtered to one token. */
export function useNFTOffers(
  collectionId?: string,
  tokenId?: number,
): UseQueryResult<NFTOfferView[], Error> {
  return useQuery<NFTOfferView[], Error>({
    queryKey: ["nft", "offers", collectionId ?? "all", tokenId ?? "all"],
    queryFn: () => endpoints.getNFTOffers(collectionId, tokenId),
    refetchInterval: REFRESH.normal,
    retry: shouldRetry,
  });
}

/** The marketplace fee configuration. */
export function useNFTConfig(): UseQueryResult<NFTMarketplaceConfig, Error> {
  return useQuery<NFTMarketplaceConfig, Error>({
    queryKey: ["nft", "config"],
    queryFn: () => endpoints.getNFTConfig(),
    staleTime: REFRESH.slow,
    retry: shouldRetry,
  });
}

/** The caller's own tokens. */
export function useMyNFTs(): UseQueryResult<NFTToken[], Error> {
  return useQuery<NFTToken[], Error>({
    queryKey: ["nft", "tokens", "mine"],
    queryFn: () => endpoints.getMyNFTs(),
    enabled: isAuthenticated(),
    refetchInterval: REFRESH.normal,
    retry: shouldRetry,
  });
}

/** The caller's own listings. */
export function useMyNFTListings(): UseQueryResult<NFTListing[], Error> {
  return useQuery<NFTListing[], Error>({
    queryKey: ["nft", "listings", "mine"],
    queryFn: () => endpoints.getMyNFTListings(),
    enabled: isAuthenticated(),
    refetchInterval: REFRESH.normal,
    retry: shouldRetry,
  });
}

/** The caller's own offers. */
export function useMyNFTOffers(): UseQueryResult<NFTOffer[], Error> {
  return useQuery<NFTOffer[], Error>({
    queryKey: ["nft", "offers", "mine"],
    queryFn: () => endpoints.getMyNFTOffers(),
    enabled: isAuthenticated(),
    refetchInterval: REFRESH.normal,
    retry: shouldRetry,
  });
}

/** Lists an owned token for a fixed price. */
export function useListNFT() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (payload: Parameters<typeof endpoints.listNFT>[0]) => endpoints.listNFT(payload),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["nft"] });
    },
  });
}

/** Buys a live listing. */
export function useBuyNFT() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => endpoints.buyNFT(id),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["nft"] });
      void queryClient.invalidateQueries({ queryKey: ["balances"] });
    },
  });
}

/** Cancels a listing, returning the escrowed token. */
export function useCancelNFTListing() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => endpoints.cancelNFTListing(id),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["nft"] });
    },
  });
}

/** Re-prices a listing the caller owns. */
export function useUpdateNFTListingPrice() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (params: { id: string; price: string }) =>
      endpoints.updateNFTListingPrice(params.id, params.price),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["nft"] });
    },
  });
}

/** Opens an escrowed bid on a token. */
export function useMakeNFTOffer() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (payload: Parameters<typeof endpoints.makeNFTOffer>[0]) =>
      endpoints.makeNFTOffer(payload),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["nft"] });
      void queryClient.invalidateQueries({ queryKey: ["balances"] });
    },
  });
}

/** Accepts an escrowed bid. */
export function useAcceptNFTOffer() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (params: { id: string; collectionId: string; tokenId: number }) =>
      endpoints.acceptNFTOffer(params.id, params.collectionId, params.tokenId),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["nft"] });
      void queryClient.invalidateQueries({ queryKey: ["balances"] });
    },
  });
}

/** Cancels an offer, refunding the escrowed bid. */
export function useCancelNFTOffer() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => endpoints.cancelNFTOffer(id),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["nft"] });
      void queryClient.invalidateQueries({ queryKey: ["balances"] });
    },
  });
}

/** Admin: every collection, including paused ones. */
export function useAllNFTCollections(enabled = true): UseQueryResult<NFTCollection[], Error> {
  return useQuery<NFTCollection[], Error>({
    queryKey: ["nft", "collections", "all"],
    queryFn: () => endpoints.getAllNFTCollections(),
    enabled,
    refetchInterval: REFRESH.normal,
    retry: shouldRetry,
  });
}

/** Admin: every listing, including settled and cancelled ones. */
export function useAllNFTListings(enabled = true): UseQueryResult<NFTListing[], Error> {
  return useQuery<NFTListing[], Error>({
    queryKey: ["nft", "listings", "all"],
    queryFn: () => endpoints.getAllNFTListings(),
    enabled,
    refetchInterval: REFRESH.normal,
    retry: shouldRetry,
  });
}

/** Admin: creates a collection. */
export function useCreateNFTCollection() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (payload: Parameters<typeof endpoints.createNFTCollection>[0]) =>
      endpoints.createNFTCollection(payload),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["nft"] });
    },
  });
}

/** Admin: edits a collection. */
export function useUpdateNFTCollection() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (params: { id: string; payload: Parameters<typeof endpoints.updateNFTCollection>[1] }) =>
      endpoints.updateNFTCollection(params.id, params.payload),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["nft"] });
    },
  });
}

/** Admin: deletes an empty collection. */
export function useDeleteNFTCollection() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => endpoints.deleteNFTCollection(id),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["nft"] });
    },
  });
}

/** Admin: mints a token into an account. */
export function useMintNFT() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (params: { collectionId: string; payload: Parameters<typeof endpoints.mintNFT>[1] }) =>
      endpoints.mintNFT(params.collectionId, params.payload),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["nft"] });
    },
  });
}

/** Admin: sets the marketplace fee. */
export function useUpdateNFTConfig() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (payload: Parameters<typeof endpoints.updateNFTConfig>[0]) =>
      endpoints.updateNFTConfig(payload),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["nft"] });
    },
  });
}

/* ------------------------------------------------------------- analytics */

/**
 * The protocol dashboard snapshot. This is the one screen that answers "how big
 * is the protocol right now" from real rows, so it refreshes on the slow
 * cadence rather than the fast one.
 */
export function useAnalyticsOverview(): UseQueryResult<ProtocolOverview, Error> {
  return useQuery<ProtocolOverview, Error>({
    queryKey: ["analytics", "overview"],
    queryFn: () => endpoints.getAnalyticsOverview(),
    refetchInterval: REFRESH.slow,
    retry: shouldRetry,
  });
}

/** A volume series for one market, or every market when `pair` is omitted. */
export function useAnalyticsVolume(params: {
  pair?: string;
  chainId?: number;
  interval?: number;
  window?: number;
} = {}): UseQueryResult<VolumeSeriesResponse, Error> {
  return useQuery<VolumeSeriesResponse, Error>({
    queryKey: ["analytics", "volume", params.pair ?? "", params.chainId ?? 0, params.interval ?? 0, params.window ?? 0],
    queryFn: () => endpoints.getAnalyticsVolume(params),
    refetchInterval: REFRESH.slow,
    retry: shouldRetry,
  });
}

/** The per-market volume leaderboard. */
export function useAnalyticsPairs(params: { chainId?: number; window?: number; limit?: number } = {}): UseQueryResult<
  PairAnalytics[],
  Error
> {
  return useQuery<PairAnalytics[], Error>({
    queryKey: ["analytics", "pairs", params.chainId ?? 0, params.window ?? 0, params.limit ?? 0],
    queryFn: () => endpoints.getAnalyticsPairs(params),
    refetchInterval: REFRESH.slow,
    retry: shouldRetry,
  });
}

/** The realized-PnL trader leaderboard. */
export function useAnalyticsTraders(params: { window?: number; limit?: number } = {}): UseQueryResult<
  TopTrader[],
  Error
> {
  return useQuery<TopTrader[], Error>({
    queryKey: ["analytics", "traders", params.window ?? 0, params.limit ?? 0],
    queryFn: () => endpoints.getAnalyticsTraders(params),
    refetchInterval: REFRESH.slow,
    retry: shouldRetry,
  });
}

/** Admin: rebuild the volume rollup for a window. */
export function useRunAnalyticsRollup() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (windowSeconds?: number) => endpoints.runAnalyticsRollup(windowSeconds),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["analytics"] });
    },
  });
}
