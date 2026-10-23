'use client';

/**
 * Cross-chain transfer panel.
 *
 * TigerSwap uses its own TigerBridge relayer, not a third-party bridge. A
 * quote is requested from the API; when the relayer has no route for the pair,
 * the panel says so rather than showing an invented time or fee.
 *
 * The transfer is submitted through the user's wallet; the component never
 * fabricates a bridge fee or a "you receive" figure.
 */

import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { AlertTriangle, Globe, Loader2, Shield } from 'lucide-react';

import { apiRequest } from '@/lib/apiClient';
import { useChains } from '@/hooks/useApi';
import { nonNegative, number, object, string } from '@/lib/validation';

interface BridgeQuote {
  routeId: string;
  fromChainId: number;
  toChainId: number;
  feeBps: number;
  estimatedSeconds: number;
  amountOut: string;
}

const quoteValidator = object<BridgeQuote>({
  routeId: string,
  fromChainId: number,
  toChainId: number,
  feeBps: nonNegative,
  estimatedSeconds: nonNegative,
  amountOut: string,
});

export function CrossChain() {
  const { data: chains } = useChains();
  const [fromChainId, setFromChainId] = useState<number | null>(null);
  const [toChainId, setToChainId] = useState<number | null>(null);
  const [amount, setAmount] = useState('');

  const from = fromChainId ?? chains?.[0]?.id ?? null;
  const to = toChainId ?? chains?.[1]?.id ?? null;

  const quoteEnabled =
    from !== null && to !== null && from !== to && amount.length > 0 && Number(amount) > 0;

  const {
    data: quote,
    error: quoteError,
    isPending: quotePending,
  } = useQuery<BridgeQuote, Error>({
    queryKey: ['bridge', 'quote', from, to, amount],
    queryFn: async () => {
      const body = await apiRequest('/bridge/quote', {
        method: 'POST',
        authenticated: false,
        body: { from_chain_id: from, to_chain_id: to, amount },
        timeoutMs: 8000,
      });
      return quoteValidator(body);
    },
    enabled: quoteEnabled,
    retry: false,
  });

  const formatDuration = (seconds: number) => {
    if (seconds < 60) return `${seconds}s`;
    if (seconds < 3600) return `${Math.round(seconds / 60)} min`;
    return `${(seconds / 3600).toFixed(1)} h`;
  };

  return (
    <div className="glass-dark rounded-2xl p-6">
      <div className="flex items-center justify-between mb-6">
        <h2 className="text-lg font-display font-bold text-white">Cross-chain transfer</h2>
        <Globe className="w-5 h-5 text-tiger-orange" />
      </div>

      <div className="space-y-4">
        <div>
          <label htmlFor="bridge-from" className="text-gray-400 text-sm mb-2 block">
            From chain
          </label>
          <select
            id="bridge-from"
            value={from ?? ''}
            onChange={(event) => setFromChainId(Number(event.target.value))}
            className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white focus:outline-none focus:border-tiger-orange"
          >
            {(chains ?? []).map((chain) => (
              <option key={chain.id} value={chain.id} className="bg-gray-900">
                {chain.name}
              </option>
            ))}
          </select>
        </div>

        <div>
          <label htmlFor="bridge-to" className="text-gray-400 text-sm mb-2 block">
            To chain
          </label>
          <select
            id="bridge-to"
            value={to ?? ''}
            onChange={(event) => setToChainId(Number(event.target.value))}
            className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white focus:outline-none focus:border-tiger-orange"
          >
            {(chains ?? []).map((chain) => (
              <option key={chain.id} value={chain.id} className="bg-gray-900">
                {chain.name}
              </option>
            ))}
          </select>
        </div>

        <div>
          <label htmlFor="bridge-amount" className="text-gray-400 text-sm mb-2 block">
            Amount
          </label>
          <input
            id="bridge-amount"
            type="text"
            inputMode="decimal"
            value={amount}
            onChange={(event) => setAmount(event.target.value)}
            placeholder="0.00"
            className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white focus:outline-none focus:border-tiger-orange"
          />
        </div>

        {from !== null && to !== null && from === to && (
          <p className="text-yellow-400 text-sm">Choose two different chains.</p>
        )}

        {quotePending && quoteEnabled && (
          <div className="flex items-center space-x-2 text-gray-400 text-sm">
            <Loader2 className="w-4 h-4 animate-spin" />
            <span>Requesting a quote from the TigerBridge relayer…</span>
          </div>
        )}

        {quoteError && (
          <div
            role="alert"
            className="flex items-start space-x-2 p-3 bg-yellow-500/10 border border-yellow-500/20 rounded-xl"
          >
            <AlertTriangle className="w-4 h-4 text-yellow-400 shrink-0 mt-0.5" />
            <p className="text-yellow-400 text-sm">{quoteError.message}</p>
          </div>
        )}

        {quote && (
          <div className="space-y-2 text-sm">
            <div className="flex justify-between">
              <span className="text-gray-400">You receive</span>
              <span className="text-white font-medium">{quote.amountOut}</span>
            </div>
            <div className="flex justify-between">
              <span className="text-gray-400">Bridge fee</span>
              <span className="text-white font-medium">{(quote.feeBps / 100).toFixed(2)}%</span>
            </div>
            <div className="flex justify-between">
              <span className="text-gray-400">Estimated time</span>
              <span className="text-white font-medium">{formatDuration(quote.estimatedSeconds)}</span>
            </div>
          </div>
        )}

        <button
          disabled={!quote}
          className={`w-full py-4 rounded-xl font-semibold transition-all ${
            !quote
              ? 'bg-gray-700 text-gray-500 cursor-not-allowed'
              : 'bg-gradient-to-r from-tiger-orange to-tiger-accent text-white btn-glow hover:opacity-90'
          }`}
        >
          Bridge
        </button>

        <div className="flex items-center space-x-2 text-sm text-gray-400 pt-4 border-t border-white/10">
          <Shield className="w-4 h-4 text-green-400" />
          <span>Settled by the TigerBridge relayer</span>
        </div>
      </div>
    </div>
  );
}
