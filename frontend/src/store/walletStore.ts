/**
 * Wallet state for TigerSwap.
 *
 * Security model:
 *
 *  - No seed phrase or private key is ever held by this store, written to
 *    `localStorage`, or sent to a server. Keys stay inside the user's wallet.
 *  - Signing and broadcasting are delegated to the injected provider via
 *    `@/lib/wallet`.
 *  - The only persisted state is non-sensitive preference data (the selected
 *    chain and the fee display configuration).
 *
 * Balances are read from the chain, so the store never invents an amount.
 */

import { create } from "zustand";
import { persist } from "zustand/middleware";

import { ApiError } from "@/lib/apiClient";
import { endpoints } from "@/lib/endpoints";
import type { Chain } from "@/lib/types";
import {
  ConnectedAccount,
  WalletError,
  connectWallet as connectProvider,
  formatUnits,
  getChainId,
  getInjectedProvider,
  getNativeBalance,
  sendTransaction as sendProviderTransaction,
  switchChain as switchProviderChain,
} from "@/lib/wallet";

export interface TokenBalance {
  address: string;
  symbol: string;
  decimals: number;
  /** Raw balance in base units. */
  raw: bigint;
  /** Human-readable balance. */
  formatted: string;
  /** Price in USD, or null when no price feed is available. */
  priceUSD: number | null;
}

export interface FeeConfig {
  /** Protocol fee applied to a swap, in basis points. */
  swapFeeBps: number;
  /** Withdrawal fee, in basis points. */
  withdrawalFeeBps: number;
}

const DEFAULT_FEE_CONFIG: FeeConfig = {
  swapFeeBps: 30,
  withdrawalFeeBps: 10,
};

interface WalletState {
  account: ConnectedAccount | null;
  chainId: number;
  chains: Chain[];
  nativeBalance: TokenBalance | null;
  tokenBalances: TokenBalance[];
  isConnecting: boolean;
  isSending: boolean;
  error: string | null;
  feeConfig: FeeConfig;

  connect: () => Promise<ConnectedAccount>;
  disconnect: () => void;
  loadChains: () => Promise<void>;
  setChain: (chainId: number) => Promise<void>;
  refreshBalances: () => Promise<void>;
  sendNative: (to: string, amount: string) => Promise<string>;
  clearError: () => void;
}

/** Converts a thrown error into a message safe to show the user. */
function describeError(error: unknown): string {
  if (error instanceof WalletError) {
    return error.message;
  }
  if (error instanceof ApiError) {
    return error.message;
  }
  if (error instanceof Error) {
    return error.message;
  }
  return "Something went wrong";
}

export const useWalletStore = create<WalletState>()(
  persist(
    (set, get) => ({
      account: null,
      chainId: 1,
      chains: [],
      nativeBalance: null,
      tokenBalances: [],
      isConnecting: false,
      isSending: false,
      error: null,
      feeConfig: DEFAULT_FEE_CONFIG,

      connect: async () => {
        set({ isConnecting: true, error: null });
        try {
          const account = await connectProvider();
          set({ account, chainId: account.chainId, isConnecting: false });
          // Read balances immediately so the UI never shows a stale zero.
          void get().refreshBalances();
          return account;
        } catch (error) {
          set({ isConnecting: false, error: describeError(error) });
          throw error;
        }
      },

      disconnect: () => {
        // Only the local reference is dropped; the wallet itself is untouched.
        set({ account: null, nativeBalance: null, tokenBalances: [], error: null });
      },

      loadChains: async () => {
        try {
          const chains = await endpoints.getChains();
          set({ chains });
        } catch (error) {
          set({ error: describeError(error) });
        }
      },

      setChain: async (chainId: number) => {
        set({ error: null });
        try {
          const chain = get().chains.find((entry) => entry.id === chainId);
          await switchProviderChain(chainId, chain
            ? {
                chainName: chain.name,
                nativeCurrency: { name: chain.nativeSymbol, symbol: chain.nativeSymbol, decimals: 18 },
                rpcUrls: [],
                blockExplorerUrls: chain.explorerURL ? [chain.explorerURL] : undefined,
              }
            : undefined);
          set({ chainId });
          void get().refreshBalances();
        } catch (error) {
          set({ error: describeError(error) });
          throw error;
        }
      },

      refreshBalances: async () => {
        const { account } = get();
        if (!account) {
          set({ nativeBalance: null, tokenBalances: [] });
          return;
        }

        const provider = getInjectedProvider();
        if (!provider) {
          set({ error: "No wallet detected" });
          return;
        }

        try {
          const currentChain = await getChainId(provider);
          const chain = get().chains.find((entry) => entry.id === currentChain);

          const raw = await getNativeBalance(account.address, provider);
          const decimals = 18;

          set({
            chainId: currentChain,
            nativeBalance: {
              address: "native",
              symbol: chain?.nativeSymbol ?? "ETH",
              decimals,
              raw,
              formatted: formatUnits(raw, decimals),
              priceUSD: null,
            },
            error: null,
          });
        } catch (error) {
          set({ error: describeError(error) });
        }
      },

      sendNative: async (to: string, amount: string) => {
        set({ isSending: true, error: null });
        try {
          // The fee is configured, but it is the protocol that collects it on
          // chain; this client only builds the transfer the user asked for.
          const hash = await sendProviderTransaction({
            to,
            value: toHexValue(amount),
          });
          void get().refreshBalances();
          return hash;
        } catch (error) {
          set({ error: describeError(error) });
          throw error;
        } finally {
          set({ isSending: false });
        }
      },

      clearError: () => set({ error: null }),
    }),
    {
      name: "tiger-wallet-preferences",
      // Only non-sensitive preferences are persisted.
      partialize: (state) => ({
        chainId: state.chainId,
        feeConfig: state.feeConfig,
      }),
    },
  ),
);

/**
 * Converts a decimal amount to a hex wei value.
 *
 * Kept local to avoid pulling a formatting library into the store; the wallet
 * lib owns the stricter parser used for token amounts.
 */
function toHexValue(amount: string): string {
  if (!/^\d+(\.\d+)?$/.test(amount)) {
    throw new WalletError("Amount must be a positive decimal number", "invalid_amount");
  }

  const [whole, fraction = ""] = amount.split(".");
  const paddedFraction = fraction.padEnd(18, "0").slice(0, 18);
  const wei = BigInt(whole) * 10n ** 18n + BigInt(paddedFraction || "0");

  if (wei <= 0n) {
    throw new WalletError("Amount must be greater than zero", "invalid_amount");
  }

  return `0x${wei.toString(16)}`;
}
