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
  AdminChain,
  AdminDashboard,
  AdminSettings,
  AdminToken,
  Chain,
  FeeConfig,
  LaunchpadProject,
  MarketStats,
  MarginAccountView,
  MarginListing,
  PerpMarket,
  PerpPosition,
  Pool,
  Portfolio,
  PriceEntry,
  SwapQuote,
  Token,
  Trade,
  UserTransaction,
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
