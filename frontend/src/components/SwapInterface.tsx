'use client';

/**
 * Swap panel.
 *
 * The quote comes from the API's routing engine; the swap itself is built by
 * the API and signed and broadcast by the user's wallet. Balances are read
 * from the chain. There is no local price arithmetic and no optimistic
 * "success" state: the panel reports what actually happened.
 */

import { useEffect, useMemo, useState } from 'react';
import { AnimatePresence, motion } from 'framer-motion';
import {
  AlertTriangle,
  ArrowDown,
  Check,
  ChevronDown,
  Loader2,
  Search,
  Settings,
  Zap,
} from 'lucide-react';

import { useExecuteSwap, useSwapQuote, useTokens } from '@/hooks/useApi';
import type { Token } from '@/lib/types';
import { formatAddress, getErc20Balance, getInjectedProvider, getNativeBalance } from '@/lib/wallet';
import { useWalletStore } from '@/store/walletStore';

const NATIVE_SENTINEL = '0x0000000000000000000000000000000000000000';
const SLIPPAGE_PRESETS = [0.1, 0.5, 1.0] as const;

export function SwapInterface() {
  const { account, chainId } = useWalletStore();
  const { data: tokens, error: tokensError } = useTokens(chainId);

  const [tokenIn, setTokenIn] = useState<Token | null>(null);
  const [tokenOut, setTokenOut] = useState<Token | null>(null);
  const [amountIn, setAmountIn] = useState('');
  const [slippage, setSlippage] = useState(0.5);
  const [showTokenSelector, setShowTokenSelector] = useState<'in' | 'out' | null>(null);
  const [showSettings, setShowSettings] = useState(false);
  const [searchQuery, setSearchQuery] = useState('');
  const [balanceIn, setBalanceIn] = useState<string | null>(null);
  const [balanceError, setBalanceError] = useState<string | null>(null);
  const [txHash, setTxHash] = useState<string | null>(null);
  const [swapError, setSwapError] = useState<string | null>(null);

  const quoteParams = useMemo(() => {
    if (!tokenIn || !tokenOut || !amountIn || Number(amountIn) <= 0) {
      return null;
    }
    return {
      tokenIn: tokenIn.address,
      tokenOut: tokenOut.address,
      amountIn,
      chainId,
    };
  }, [tokenIn, tokenOut, amountIn, chainId]);

  const { data: quote, error: quoteError, isFetching: quoteFetching } = useSwapQuote(
    quoteParams,
    quoteParams !== null,
  );

  const executeSwap = useExecuteSwap();

  // Read the input-token balance from the chain whenever the selection changes.
  useEffect(() => {
    let cancelled = false;
    setBalanceError(null);

    const loadBalance = async () => {
      if (!account || !tokenIn) {
        setBalanceIn(null);
        return;
      }
      const provider = getInjectedProvider();
      if (!provider) {
        setBalanceIn(null);
        return;
      }

      try {
        if (tokenIn.address.toLowerCase() === NATIVE_SENTINEL) {
          const raw = await getNativeBalance(account.address, provider);
          if (!cancelled) {
            setBalanceIn(formatBalance(raw, tokenIn.decimals));
          }
        } else {
          const raw = await getErc20Balance(tokenIn.address, account.address, provider);
          if (!cancelled) {
            setBalanceIn(formatBalance(raw, tokenIn.decimals));
          }
        }
      } catch (error) {
        if (!cancelled) {
          setBalanceIn(null);
          setBalanceError(error instanceof Error ? error.message : 'Could not read balance');
        }
      }
    };

    void loadBalance();
    return () => {
      cancelled = true;
    };
  }, [account, tokenIn]);

  const filteredTokens = (tokens ?? []).filter(
    (token) =>
      token.symbol.toLowerCase().includes(searchQuery.toLowerCase()) ||
      token.name.toLowerCase().includes(searchQuery.toLowerCase()),
  );

  const handleSelectToken = (token: Token) => {
    if (showTokenSelector === 'in') {
      setTokenIn(token);
    } else {
      setTokenOut(token);
    }
    setShowTokenSelector(null);
    setSearchQuery('');
  };

  const handleSwitchTokens = () => {
    setTokenIn(tokenOut);
    setTokenOut(tokenIn);
    setAmountIn(quote?.amountOut ?? '');
    setTxHash(null);
    setSwapError(null);
  };

  const handleSwap = async () => {
    if (!account || !tokenIn || !tokenOut || !quote) {
      return;
    }

    setSwapError(null);
    setTxHash(null);

    try {
      const hash = await executeSwap.mutateAsync({
        tokenIn: tokenIn.address,
        tokenOut: tokenOut.address,
        amountIn,
        chainId,
        recipient: account.address,
        slippageBps: Math.round(slippage * 100),
        send: async (transaction) => {
          const { sendTransaction } = await import('@/lib/wallet');
          return sendTransaction({
            to: transaction.to,
            data: transaction.data,
            value: transaction.value,
          });
        },
      });
      setTxHash(hash);
      setAmountIn('');
    } catch (error) {
      setSwapError(error instanceof Error ? error.message : 'Swap failed');
    }
  };

  const displayQuoteError = quoteError instanceof Error ? quoteError.message : null;

  return (
    <div className="glass-dark rounded-2xl p-6">
      <div className="flex items-center justify-between mb-6">
        <h2 className="text-xl font-display font-bold text-white">Swap</h2>
        <button
          onClick={() => setShowSettings((value) => !value)}
          className="p-2 text-gray-400 hover:text-white rounded-lg hover:bg-white/5 transition-colors"
          title="Swap settings"
        >
          <Settings className="w-4 h-4" />
        </button>
      </div>

      <AnimatePresence>
        {showSettings && (
          <motion.div
            initial={{ opacity: 0, height: 0 }}
            animate={{ opacity: 1, height: 'auto' }}
            exit={{ opacity: 0, height: 0 }}
            className="mb-6 p-4 bg-white/5 rounded-xl border border-white/10 overflow-hidden"
          >
            <label className="block text-sm text-gray-400 mb-2">Slippage tolerance</label>
            <div className="flex items-center space-x-2">
              {SLIPPAGE_PRESETS.map((value) => (
                <button
                  key={value}
                  onClick={() => setSlippage(value)}
                  className={`px-3 py-1.5 rounded-lg text-sm border ${
                    slippage === value
                      ? 'bg-tiger-orange/20 text-tiger-orange border-tiger-orange/30'
                      : 'bg-white/5 text-gray-400 border-white/10 hover:bg-white/10'
                  }`}
                >
                  {value}%
                </button>
              ))}
              <input
                type="number"
                value={slippage}
                min={0}
                max={50}
                step={0.1}
                onChange={(event) => {
                  const value = Number(event.target.value);
                  if (Number.isFinite(value) && value >= 0 && value <= 50) {
                    setSlippage(value);
                  }
                }}
                aria-label="Custom slippage tolerance"
                className="w-20 px-2 py-1.5 bg-white/5 border border-white/10 rounded-lg text-white text-sm"
              />
              <span className="text-gray-400 text-sm">%</span>
            </div>
          </motion.div>
        )}
      </AnimatePresence>

      <div className="space-y-4">
        {/* From token */}
        <div className="relative bg-white/5 rounded-xl border border-white/10 p-4">
          <div className="flex items-center justify-between mb-2">
            <span className="text-gray-400 text-sm">From</span>
            <span className="text-gray-400 text-sm">
              {balanceIn === null ? '' : `Balance: ${balanceIn}`}
            </span>
          </div>
          <div className="flex items-center space-x-4">
            <button
              onClick={() => setShowTokenSelector('in')}
              className="flex items-center space-x-2 px-3 py-2 bg-white/5 rounded-lg hover:bg-white/10 transition-colors"
            >
              {tokenIn ? (
                <>
                  <span className="text-white font-medium">{tokenIn.symbol}</span>
                  <ChevronDown className="w-4 h-4 text-gray-400" />
                </>
              ) : (
                <>
                  <span className="text-gray-400">Select token</span>
                  <ChevronDown className="w-4 h-4 text-gray-400" />
                </>
              )}
            </button>
            <input
              type="text"
              inputMode="decimal"
              value={amountIn}
              onChange={(event) => setAmountIn(event.target.value)}
              placeholder="0.0"
              className="flex-1 bg-transparent text-right text-2xl text-white placeholder-gray-600 focus:outline-none"
            />
          </div>
          {balanceError && <p className="text-red-400 text-xs mt-2">{balanceError}</p>}
        </div>

        <div className="flex justify-center -my-2 relative z-10">
          <button
            onClick={handleSwitchTokens}
            className="p-2 bg-tiger-dark border border-white/10 rounded-lg hover:bg-white/5 transition-colors"
            title="Switch tokens"
          >
            <ArrowDown className="w-4 h-4 text-gray-400" />
          </button>
        </div>

        {/* To token */}
        <div className="bg-white/5 rounded-xl border border-white/10 p-4">
          <div className="flex items-center justify-between mb-2">
            <span className="text-gray-400 text-sm">To</span>
            {quoteFetching && <Loader2 className="w-4 h-4 text-gray-400 animate-spin" />}
          </div>
          <div className="flex items-center space-x-4">
            <button
              onClick={() => setShowTokenSelector('out')}
              className="flex items-center space-x-2 px-3 py-2 bg-white/5 rounded-lg hover:bg-white/10 transition-colors"
            >
              {tokenOut ? (
                <>
                  <span className="text-white font-medium">{tokenOut.symbol}</span>
                  <ChevronDown className="w-4 h-4 text-gray-400" />
                </>
              ) : (
                <>
                  <span className="text-gray-400">Select token</span>
                  <ChevronDown className="w-4 h-4 text-gray-400" />
                </>
              )}
            </button>
            <div className="flex-1 text-right text-2xl text-white">
              {quote ? quote.amountOut : <span className="text-gray-600">0.0</span>}
            </div>
          </div>
        </div>

        {/* Quote detail */}
        {quote && (
          <div className="p-4 bg-white/5 rounded-xl space-y-2 text-sm">
            <div className="flex justify-between">
              <span className="text-gray-400">Minimum received</span>
              <span className="text-white">{quote.amountOutMin}</span>
            </div>
            <div className="flex justify-between">
              <span className="text-gray-400">Price impact</span>
              <span className={quote.priceImpact > 5 ? 'text-yellow-400' : 'text-white'}>
                {quote.priceImpact.toFixed(2)}%
              </span>
            </div>
            <div className="flex justify-between">
              <span className="text-gray-400">Route</span>
              <span className="text-white">{quote.route.join(' → ')}</span>
            </div>
            <div className="flex justify-between">
              <span className="text-gray-400">Estimated gas</span>
              <span className="text-white">{quote.gasEstimate.toLocaleString()}</span>
            </div>
          </div>
        )}

        {displayQuoteError && (
          <div role="alert" className="flex items-start space-x-2 p-3 bg-yellow-500/10 border border-yellow-500/20 rounded-xl">
            <AlertTriangle className="w-4 h-4 text-yellow-400 shrink-0 mt-0.5" />
            <p className="text-yellow-400 text-sm">{displayQuoteError}</p>
          </div>
        )}

        {swapError && (
          <div role="alert" className="p-3 bg-red-500/10 border border-red-500/20 rounded-xl text-red-400 text-sm">
            {swapError}
          </div>
        )}

        {txHash && (
          <div className="p-3 bg-green-500/10 border border-green-500/20 rounded-xl">
            <div className="flex items-center space-x-2 text-green-400 text-sm mb-1">
              <Check className="w-4 h-4" />
              <span>Swap submitted</span>
            </div>
            <p className="text-white font-mono text-xs break-all">{txHash}</p>
          </div>
        )}

        {tokensError && (
          <p role="alert" className="text-red-400 text-sm">
            Token list unavailable: {tokensError.message}
          </p>
        )}

        <button
          onClick={() => void handleSwap()}
          disabled={!account || !quote || executeSwap.isPending}
          className="w-full py-4 rounded-xl font-bold transition-colors bg-tiger-orange hover:bg-tiger-accent disabled:bg-gray-700 disabled:text-gray-500 disabled:cursor-not-allowed flex items-center justify-center space-x-2"
        >
          {executeSwap.isPending ? (
            <>
              <Loader2 className="w-5 h-5 animate-spin" />
              <span>Confirm in your wallet…</span>
            </>
          ) : !account ? (
            <span>Connect your wallet</span>
          ) : (
            <span>Swap</span>
          )}
        </button>
      </div>

      {/* Token selector */}
      <AnimatePresence>
        {showTokenSelector && (
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            className="fixed inset-0 bg-black/80 backdrop-blur-sm flex items-center justify-center z-50 p-4"
            onClick={() => setShowTokenSelector(null)}
          >
            <motion.div
              initial={{ scale: 0.95, opacity: 0 }}
              animate={{ scale: 1, opacity: 1 }}
              exit={{ scale: 0.95, opacity: 0 }}
              className="bg-[#1a1a2e] border border-white/10 rounded-2xl p-6 w-full max-w-md"
              onClick={(event) => event.stopPropagation()}
            >
              <div className="relative mb-4">
                <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-5 h-5 text-gray-400" />
                <input
                  type="text"
                  value={searchQuery}
                  onChange={(event) => setSearchQuery(event.target.value)}
                  placeholder="Search by name or symbol"
                  className="w-full pl-10 pr-4 py-3 bg-white/5 border border-white/10 rounded-xl text-white placeholder:text-gray-500 focus:outline-none focus:border-tiger-orange/50"
                />
              </div>

              <div className="space-y-1 max-h-80 overflow-y-auto">
                {filteredTokens.map((token) => (
                  <button
                    key={`${token.chainId}-${token.address}`}
                    onClick={() => handleSelectToken(token)}
                    className="w-full flex items-center justify-between p-3 rounded-xl hover:bg-white/5 transition-colors"
                  >
                    <div className="text-left">
                      <div className="text-white font-medium">{token.symbol}</div>
                      <div className="text-gray-500 text-sm">{token.name}</div>
                    </div>
                    <div className="text-right text-gray-400 text-xs font-mono">
                      {formatAddress(token.address)}
                    </div>
                  </button>
                ))}
                {filteredTokens.length === 0 && (
                  <p className="text-gray-500 text-sm text-center py-6">No tokens match your search.</p>
                )}
              </div>
            </motion.div>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
}

/** Formats a raw base-unit balance using the token's decimals. */
function formatBalance(raw: bigint, decimals: number): string {
  if (decimals === 0) return raw.toString();
  const divisor = 10n ** BigInt(decimals);
  const whole = raw / divisor;
  const fraction = raw % divisor;
  if (fraction === 0n) return whole.toString();
  const padded = fraction.toString().padStart(decimals, '0').replace(/0+$/, '');
  return `${whole}.${padded}`;
}
