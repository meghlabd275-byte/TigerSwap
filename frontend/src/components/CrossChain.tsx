'use client';

/**
 * Cross-chain transfer panel.
 *
 * TigerSwap uses its own TigerBridge relayer, not a third-party bridge. A
 * quote is requested from `POST /bridge/quote`, which prices the bridged token
 * from the source chain's registry entry. When the relayer has no route for
 * the pair, the panel shows the relayer's own error rather than an invented
 * time or fee.
 *
 * The transfer itself is submitted with `POST /bridge/transfer`; the sender is
 * the caller's custody wallet, resolved server-side from the session, so this
 * component never asserts who is sending.
 */

import { useState } from 'react';
import { useMutation, useQuery } from '@tanstack/react-query';
import { AlertTriangle, Globe, Loader2, Shield } from 'lucide-react';

import { endpoints } from '@/lib/endpoints';
import type { BridgeQuote } from '@/lib/types';
import { useChains, useTokens } from '@/hooks/useApi';
import { isAuthenticated } from '@/lib/auth';
import { parseUnits } from '@/lib/wallet';

export function CrossChain() {
  const { data: chains } = useChains();
  const { data: tokens } = useTokens();
  const [fromChainId, setFromChainId] = useState<number | null>(null);
  const [toChainId, setToChainId] = useState<number | null>(null);
  const [tokenAddress, setTokenAddress] = useState<string>('');
  const [recipient, setRecipient] = useState('');
  const [amount, setAmount] = useState('');
  const [notice, setNotice] = useState<string | null>(null);

  const from = fromChainId ?? chains?.[0]?.id ?? null;
  const to = toChainId ?? chains?.[1]?.id ?? null;

  const sourceTokens = (tokens ?? []).filter((token) => token.chainId === from);
  const selectedToken =
    sourceTokens.find((token) => token.address === tokenAddress) ?? sourceTokens[0] ?? null;

  const quoteEnabled =
    from !== null &&
    to !== null &&
    from !== to &&
    selectedToken !== null &&
    recipient.length > 0 &&
    amount.length > 0;

  const {
    data: quote,
    error: quoteError,
    isPending: quotePending,
  } = useQuery<BridgeQuote, Error>({
    queryKey: ['bridge', 'quote', from, to, selectedToken?.address, amount],
    queryFn: () =>
      endpoints.getBridgeQuote({
        src_chain: from as number,
        dst_chain: to as number,
        token: selectedToken!.address,
        amount: normalizedAmount(),
      }),
    enabled: quoteEnabled,
    retry: false,
  });

  function normalizedAmount(): string {
    if (!selectedToken) return '0';
    try {
      return parseUnits(amount, selectedToken.decimals).toString();
    } catch {
      return '0';
    }
  }

  const transfer = useMutation({
    mutationFn: () =>
      endpoints.initiateBridgeTransfer({
        src_chain: from as number,
        dst_chain: to as number,
        token: selectedToken!.address,
        amount: normalizedAmount(),
        recipient,
      }),
    onSuccess: (created) => {
      setNotice(`Transfer ${created.id} submitted (${created.status}).`);
    },
    onError: (mutationError: Error) => setNotice(mutationError.message),
  });

  const formatDuration = (estimated: string) => (estimated.length > 0 ? estimated : 'pending');

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
            onChange={(event) => {
              setFromChainId(Number(event.target.value));
              setTokenAddress('');
            }}
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
          <label htmlFor="bridge-token" className="text-gray-400 text-sm mb-2 block">
            Token
          </label>
          <select
            id="bridge-token"
            value={selectedToken?.address ?? ''}
            onChange={(event) => setTokenAddress(event.target.value)}
            className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white focus:outline-none focus:border-tiger-orange"
          >
            {sourceTokens.map((token) => (
              <option key={token.address} value={token.address} className="bg-gray-900">
                {token.symbol}
              </option>
            ))}
          </select>
        </div>

        <div>
          <label htmlFor="bridge-recipient" className="text-gray-400 text-sm mb-2 block">
            Recipient
          </label>
          <input
            id="bridge-recipient"
            type="text"
            value={recipient}
            onChange={(event) => setRecipient(event.target.value)}
            placeholder="0x…"
            className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white font-mono focus:outline-none focus:border-tiger-orange"
          />
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

        {notice && (
          <p className="text-sm text-tiger-orange" role="status">
            {notice}
          </p>
        )}

        {quote && (
          <div className="space-y-2 text-sm">
            <div className="flex justify-between">
              <span className="text-gray-400">You receive</span>
              <span className="text-white font-medium">{quote.amount_out}</span>
            </div>
            <div className="flex justify-between">
              <span className="text-gray-400">Bridge fee</span>
              <span className="text-white font-medium">{quote.fee}</span>
            </div>
            <div className="flex justify-between">
              <span className="text-gray-400">Estimated time</span>
              <span className="text-white font-medium">{formatDuration(quote.estimated_time)}</span>
            </div>
          </div>
        )}

        <button
          type="button"
          disabled={!quote || !isAuthenticated() || transfer.isPending}
          onClick={() => transfer.mutate()}
          className={`w-full py-4 rounded-xl font-semibold transition-all ${
            !quote || !isAuthenticated()
              ? 'bg-gray-700 text-gray-500 cursor-not-allowed'
              : 'bg-gradient-to-r from-tiger-orange to-tiger-accent text-white btn-glow hover:opacity-90'
          }`}
        >
          {transfer.isPending ? 'Submitting…' : 'Bridge'}
        </button>

        <div className="flex items-center space-x-2 text-sm text-gray-400 pt-4 border-t border-white/10">
          <Shield className="w-4 h-4 text-green-400" />
          <span>Settled by the TigerBridge relayer</span>
        </div>
      </div>
    </div>
  );
}
