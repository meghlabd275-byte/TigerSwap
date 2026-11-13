'use client';

/**
 * On-chain perpetual order book panel.
 *
 * This is the Hyperliquid/dYdX-style venue: the deployed
 * `TigerSwapPerpOrderBook` contract holds every resting order, matches a taker
 * against them in one transaction, and keeps each trader's signed position as
 * the net of its fills. Funding, margin and liquidation all settle on-chain.
 *
 * The server is only a calldata builder here. It reads chain state for the
 * ladder and the trader's own rows, and returns unsigned transactions the
 * caller's wallet signs, so a user can verify every fill with an RPC endpoint.
 */

import { useMemo, useState } from 'react';
import { AlertTriangle, Loader2 } from 'lucide-react';

import {
  useBuildPerpBookCancel,
  useBuildPerpBookDeposit,
  useBuildPerpBookInsurance,
  useBuildPerpBookLiquidity,
  useBuildPerpBookOrder,
  useBuildPerpBookWithdraw,
  usePerpBookDepth,
  usePerpBookMarkets,
  usePerpBookFills,
  usePerpBookOrders,
  usePerpBookPositions,
} from '@/hooks/useApi';
import { useSession } from '@/hooks/useSession';
import type { OnChainCall, PerpBookOrder, PerpBookPosition } from '@/lib/types';

const WAD = 10n ** 18n;
const DECIMALS = 18;

type Side = 'buy' | 'sell';
type TimeInForce = 'gtc' | 'ioc' | 'fok' | 'post_only';

/** Renders a 1e18-scaled integer string as a human number. */
function formatScaled(value: string, decimals = DECIMALS): string {
  try {
    const raw = BigInt(value);
    const negative = raw < 0n;
    const magnitude = negative ? -raw : raw;
    const divisor = 10n ** BigInt(decimals);
    const whole = magnitude / divisor;
    const fraction = (magnitude % divisor)
      .toString()
      .padStart(decimals, '0')
      .slice(0, 4)
      .replace(/0+$/, '');
    const rendered = fraction ? `${whole}.${fraction}` : whole.toString();
    return negative ? `-${rendered}` : rendered;
  } catch {
    return value;
  }
}

/** Converts a human amount like "1.5" into an 18-decimal integer string. */
function toBaseUnits(amount: string): string | null {
  const trimmed = amount.trim();
  if (!/^\d+(\.\d+)?$/.test(trimmed)) {
    return null;
  }
  const [whole, fraction = ''] = trimmed.split('.');
  if (fraction.length > DECIMALS) {
    return null;
  }
  return `${whole}${fraction.padEnd(DECIMALS, '0')}`.replace(/^0+(?=\d)/, '');
}

async function sendCall(call: OnChainCall): Promise<string> {
  const { sendTransaction, waitForTransactionReceipt } = await import('@/lib/wallet');
  const hash = await sendTransaction({ to: call.to, data: call.data, value: call.value });
  await waitForTransactionReceipt(hash);
  return hash;
}

/**
 * Approves `amount` of `token` to the order book and waits for the approval to
 * be mined, so the pull that follows cannot revert for a missing allowance.
 */
async function approveToken(token: string, spender: string, amount: bigint): Promise<void> {
  const { approveErc20, waitForTransactionReceipt } = await import('@/lib/wallet');
  const hash = await approveErc20(token, spender, amount);
  await waitForTransactionReceipt(hash);
}

export function PerpOrderBook() {
  const { user } = useSession();
  const marketsQuery = usePerpBookMarkets();

  const [selectedKey, setSelectedKey] = useState<string | null>(null);
  const [side, setSide] = useState<Side>('buy');
  const [price, setPrice] = useState('');
  const [quantity, setQuantity] = useState('');
  const [tif, setTif] = useState<TimeInForce>('gtc');
  const [reduceOnly, setReduceOnly] = useState(false);
  const [marginAmount, setMarginAmount] = useState('');
  const [formError, setFormError] = useState<string | null>(null);
  const [txHash, setTxHash] = useState<string | null>(null);

  const markets = useMemo(() => marketsQuery.data ?? [], [marketsQuery.data]);
  const selected = useMemo(
    () => markets.find((m) => `${m.chain_id}:${m.market_id}` === selectedKey) ?? markets[0] ?? null,
    [markets, selectedKey],
  );
  const chainId = selected?.chain_id ?? null;
  const marketId = selected?.market_id ?? null;

  const depthQuery = usePerpBookDepth(chainId, marketId);
  const positionsQuery = usePerpBookPositions(chainId);
  const ordersQuery = usePerpBookOrders(chainId);
  const fillsQuery = usePerpBookFills(chainId);

  const deposit = useBuildPerpBookDeposit();
  const withdraw = useBuildPerpBookWithdraw();
  const placeOrder = useBuildPerpBookOrder();
  const cancelOrder = useBuildPerpBookCancel();
  const liquidity = useBuildPerpBookLiquidity();
  const insurance = useBuildPerpBookInsurance();

  const run = async (action: () => Promise<string>) => {
    setFormError(null);
    setTxHash(null);
    try {
      setTxHash(await action());
    } catch (error) {
      setFormError(error instanceof Error ? error.message : 'Transaction failed');
    }
  };

  const requireMarket = (): { chainId: number; marketId: string } | null => {
    if (!selected || chainId === null || marketId === null) {
      setFormError('Select a market first');
      return null;
    }
    return { chainId, marketId };
  };

  const handleDeposit = () => {
    const target = requireMarket();
    const amount = toBaseUnits(marginAmount);
    if (!target) return;
    if (!amount) {
      setFormError('Enter a positive amount');
      return;
    }
    void run(() =>
      deposit.mutateAsync({
        chain_id: target.chainId,
        market_id: target.marketId,
        amount,
        token: selected?.token,
        approve: approveToken,
        send: sendCall,
      }),
    );
  };

  const handleWithdraw = () => {
    const target = requireMarket();
    const amount = toBaseUnits(marginAmount);
    if (!target) return;
    if (!amount) {
      setFormError('Enter a positive amount');
      return;
    }
    void run(() =>
      withdraw.mutateAsync({
        chain_id: target.chainId,
        market_id: target.marketId,
        amount,
        send: sendCall,
      }),
    );
  };

  const handlePlace = () => {
    const target = requireMarket();
    const quantityWei = toBaseUnits(quantity);
    const priceWei = toBaseUnits(price);
    if (!target) return;
    if (!quantityWei || !priceWei) {
      setFormError('Enter a positive quantity and price');
      return;
    }
    void run(() =>
      placeOrder.mutateAsync({
        chain_id: target.chainId,
        market_id: target.marketId,
        quantity: quantityWei,
        price: priceWei,
        side,
        time_in_force: tif,
        reduce_only: reduceOnly,
        send: sendCall,
      }),
    );
  };

  const handleCancel = (order: PerpBookOrder) => {
    if (!selected) return;
    void run(() =>
      cancelOrder.mutateAsync({
        order_id: order.order_id,
        chain_id: selected.chain_id,
        send: sendCall,
      }),
    );
  };

  const handleLiquidity = (remove: boolean) => {
    const target = requireMarket();
    const amount = toBaseUnits(marginAmount);
    if (!target) return;
    if (!amount) {
      setFormError('Enter a positive amount');
      return;
    }
    void run(() =>
      liquidity.mutateAsync({
        chain_id: target.chainId,
        market_id: target.marketId,
        amount,
        token: selected?.token,
        approve: approveToken,
        remove,
        send: sendCall,
      }),
    );
  };

  const handleInsurance = () => {
    const target = requireMarket();
    const amount = toBaseUnits(marginAmount);
    if (!target) return;
    if (!amount) {
      setFormError('Enter a positive amount');
      return;
    }
    void run(() =>
      insurance.mutateAsync({
        chain_id: target.chainId,
        market_id: target.marketId,
        amount,
        token: selected?.token,
        approve: approveToken,
        send: sendCall,
      }),
    );
  };

  const busy =
    deposit.isPending ||
    withdraw.isPending ||
    placeOrder.isPending ||
    cancelOrder.isPending ||
    liquidity.isPending ||
    insurance.isPending;

  if (marketsQuery.error) {
    return (
      <div className="glass-dark rounded-2xl p-6">
        <div className="flex items-start space-x-3 text-yellow-400">
          <AlertTriangle className="w-5 h-5 mt-0.5" />
          <div>
            <h2 className="text-lg font-semibold">Order book unavailable</h2>
            <p className="text-sm text-gray-400 mt-1">
              {marketsQuery.error instanceof Error
                ? marketsQuery.error.message
                : 'The on-chain order book venue is not reachable.'}
            </p>
          </div>
        </div>
      </div>
    );
  }

  if (marketsQuery.isPending) {
    return (
      <div className="glass-dark rounded-2xl p-6 flex items-center space-x-3 text-gray-400">
        <Loader2 className="w-5 h-5 animate-spin" />
        <span>Loading on-chain order book markets…</span>
      </div>
    );
  }

  if (markets.length === 0) {
    return (
      <div className="glass-dark rounded-2xl p-6">
        <h2 className="text-lg font-semibold text-white">On-chain order book</h2>
        <p className="text-sm text-gray-400 mt-2">
          No on-chain order book market is configured. An operator creates a market on the
          deployed contract; the chain is the source of truth, so nothing is shown until one
          exists.
        </p>
      </div>
    );
  }

  const depth = depthQuery.data ?? null;
  const positions = positionsQuery.data ?? [];
  const orders = ordersQuery.data ?? [];
  const fills = fillsQuery.data ?? [];
  const bidLevels = depth ? depth.bid_prices.map((p, i) => ({ price: p, qty: depth.bid_qty[i] })) : [];
  const askLevels = depth ? depth.ask_prices.map((p, i) => ({ price: p, qty: depth.ask_qty[i] })) : [];

  return (
    <div className="space-y-6">
      <div className="glass-dark rounded-2xl p-6">
        <div className="flex items-center justify-between mb-4">
          <div>
            <h2 className="text-xl font-display font-bold text-white">On-chain Order Book</h2>
            <p className="text-xs text-gray-500 mt-1">
              Non-custodial CLOB: every order rests and matches in the contract; your wallet
              signs each transaction.
            </p>
          </div>
          {selected && (
            <span className="text-xs px-2 py-1 rounded-lg bg-green-500/10 text-green-400 border border-green-500/20">
              chain {selected.chain_id}
            </span>
          )}
        </div>

        <label className="block text-xs text-gray-400 mb-1">Market</label>
        <select
          value={selected ? `${selected.chain_id}:${selected.market_id}` : ''}
          onChange={(e) => setSelectedKey(e.target.value)}
          className="w-full bg-black/30 border border-white/10 rounded-lg px-3 py-2 text-sm text-white mb-4"
        >
          {markets.map((m) => (
            <option key={`${m.chain_id}:${m.market_id}`} value={`${m.chain_id}:${m.market_id}`}>
              {m.symbol} · {m.chain_id} · {m.is_active ? 'active' : 'paused'} · max{' '}
              {formatScaled(m.max_leverage, 0)}x
            </option>
          ))}
        </select>

        {selected && (
          <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 text-xs">
            <div className="bg-black/20 rounded-lg p-3">
              <div className="text-gray-500">Mark</div>
              <div className="text-white font-medium">
                {depth ? formatScaled(depth.mark_price) : '—'}
              </div>
            </div>
            <div className="bg-black/20 rounded-lg p-3">
              <div className="text-gray-500">Index</div>
              <div className="text-white font-medium">
                {depth ? formatScaled(depth.index_price) : '—'}
              </div>
            </div>
            <div className="bg-black/20 rounded-lg p-3">
              <div className="text-gray-500">Best bid</div>
              <div className="text-green-400 font-medium">
                {depth ? formatScaled(depth.best_bid) : '—'}
              </div>
            </div>
            <div className="bg-black/20 rounded-lg p-3">
              <div className="text-gray-500">Best ask</div>
              <div className="text-red-400 font-medium">
                {depth ? formatScaled(depth.best_ask) : '—'}
              </div>
            </div>
          </div>
        )}
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Order book ladder */}
        <div className="glass-dark rounded-2xl p-6">
          <h3 className="text-sm font-semibold text-white mb-3">Book</h3>
          <div className="grid grid-cols-2 gap-4 text-xs font-mono">
            <div>
              <div className="flex justify-between text-gray-500 mb-2">
                <span>Bid size</span>
                <span>Bid</span>
              </div>
              {bidLevels.length === 0 && <div className="text-gray-600">no bids</div>}
              {bidLevels.map((l, i) => (
                <div key={i} className="flex justify-between text-green-400">
                  <span>{formatScaled(l.qty)}</span>
                  <span>{formatScaled(l.price)}</span>
                </div>
              ))}
            </div>
            <div>
              <div className="flex justify-between text-gray-500 mb-2">
                <span>Ask</span>
                <span>Ask size</span>
              </div>
              {askLevels.length === 0 && <div className="text-gray-600">no asks</div>}
              {askLevels.map((l, i) => (
                <div key={i} className="flex justify-between text-red-400">
                  <span>{formatScaled(l.price)}</span>
                  <span>{formatScaled(l.qty)}</span>
                </div>
              ))}
            </div>
          </div>
        </div>

        {/* Order ticket */}
        <div className="glass-dark rounded-2xl p-6">
          <h3 className="text-sm font-semibold text-white mb-3">Place limit order</h3>
          <div className="flex space-x-2 mb-3">
            <button
              onClick={() => setSide('buy')}
              className={`flex-1 py-2 rounded-lg text-sm font-medium ${
                side === 'buy' ? 'bg-green-500/20 text-green-400' : 'bg-white/5 text-gray-400'
              }`}
            >
              Buy / Long
            </button>
            <button
              onClick={() => setSide('sell')}
              className={`flex-1 py-2 rounded-lg text-sm font-medium ${
                side === 'sell' ? 'bg-red-500/20 text-red-400' : 'bg-white/5 text-gray-400'
              }`}
            >
              Sell / Short
            </button>
          </div>

          <label className="block text-xs text-gray-400 mb-1">Price</label>
          <input
            value={price}
            onChange={(e) => setPrice(e.target.value)}
            placeholder="0.0"
            inputMode="decimal"
            className="w-full bg-black/30 border border-white/10 rounded-lg px-3 py-2 text-sm text-white mb-3"
          />

          <label className="block text-xs text-gray-400 mb-1">Quantity</label>
          <input
            value={quantity}
            onChange={(e) => setQuantity(e.target.value)}
            placeholder="0.0"
            inputMode="decimal"
            className="w-full bg-black/30 border border-white/10 rounded-lg px-3 py-2 text-sm text-white mb-3"
          />

          <div className="flex items-center justify-between mb-3">
            <div className="flex space-x-1">
              {(['gtc', 'ioc', 'fok', 'post_only'] as TimeInForce[]).map((t) => (
                <button
                  key={t}
                  onClick={() => setTif(t)}
                  className={`px-2 py-1 rounded text-xs ${
                    tif === t ? 'bg-tiger-orange/20 text-tiger-orange' : 'bg-white/5 text-gray-400'
                  }`}
                >
                  {t.toUpperCase()}
                </button>
              ))}
            </div>
            <label className="flex items-center space-x-2 text-xs text-gray-400">
              <input
                type="checkbox"
                checked={reduceOnly}
                onChange={(e) => setReduceOnly(e.target.checked)}
              />
              <span>Reduce only</span>
            </label>
          </div>

          <button
            onClick={handlePlace}
            disabled={busy || !user}
            className="w-full py-2 rounded-lg bg-tiger-orange text-black text-sm font-semibold disabled:opacity-50"
          >
            {busy ? 'Submitting…' : user ? `Place ${side} order` : 'Sign in to trade'}
          </button>

          {formError && <p className="text-xs text-red-400 mt-2">{formError}</p>}
          {txHash && <p className="text-xs text-green-400 mt-2 break-all">tx: {txHash}</p>}
        </div>
      </div>

      {/* Margin, liquidity, insurance */}
      <div className="glass-dark rounded-2xl p-6">
        <h3 className="text-sm font-semibold text-white mb-3">Margin & liquidity</h3>
        <div className="flex flex-col sm:flex-row sm:items-end gap-3">
          <div className="flex-1">
            <label className="block text-xs text-gray-400 mb-1">Amount</label>
            <input
              value={marginAmount}
              onChange={(e) => setMarginAmount(e.target.value)}
              placeholder="0.0"
              inputMode="decimal"
              className="w-full bg-black/30 border border-white/10 rounded-lg px-3 py-2 text-sm text-white"
            />
          </div>
          <div className="flex flex-wrap gap-2">
            <button
              onClick={handleDeposit}
              disabled={busy}
              className="px-3 py-2 rounded-lg bg-white/5 text-sm text-white hover:bg-white/10 disabled:opacity-50"
            >
              Deposit margin
            </button>
            <button
              onClick={handleWithdraw}
              disabled={busy}
              className="px-3 py-2 rounded-lg bg-white/5 text-sm text-white hover:bg-white/10 disabled:opacity-50"
            >
              Withdraw
            </button>
            <button
              onClick={() => handleLiquidity(false)}
              disabled={busy}
              className="px-3 py-2 rounded-lg bg-white/5 text-sm text-white hover:bg-white/10 disabled:opacity-50"
            >
              Add LP
            </button>
            <button
              onClick={() => handleLiquidity(true)}
              disabled={busy}
              className="px-3 py-2 rounded-lg bg-white/5 text-sm text-white hover:bg-white/10 disabled:opacity-50"
            >
              Remove LP
            </button>
            <button
              onClick={handleInsurance}
              disabled={busy}
              className="px-3 py-2 rounded-lg bg-white/5 text-sm text-white hover:bg-white/10 disabled:opacity-50"
            >
              Fund insurance
            </button>
          </div>
        </div>
      </div>

      {/* Positions */}
      <div className="glass-dark rounded-2xl p-6">
        <h3 className="text-sm font-semibold text-white mb-3">Your positions</h3>
        {positions.length === 0 ? (
          <p className="text-xs text-gray-500">No open positions on this chain.</p>
        ) : (
          <div className="space-y-2">
            {positions.map((p) => (
              <div
                key={`${p.market_id}:${p.trader}`}
                className="flex items-center justify-between text-xs bg-black/20 rounded-lg p-3"
              >
                <span className="text-white font-medium">{p.symbol}</span>
                <span
                  className={p.side === 'long' ? 'text-green-400' : p.side === 'short' ? 'text-red-400' : 'text-gray-400'}
                >
                  {p.side} {formatScaled(p.size)}
                </span>
                <span className="text-gray-400">entry {formatScaled(p.entry_price)}</span>
                <span className="text-gray-400">margin {formatScaled(p.margin)}</span>
                <span className={BigInt(p.unrealized_pnl) >= 0n ? 'text-green-400' : 'text-red-400'}>
                  PnL {formatScaled(p.unrealized_pnl)}
                </span>
                <span className="text-gray-400">equity {formatScaled(p.equity)}</span>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Open orders */}
      <div className="glass-dark rounded-2xl p-6">
        <h3 className="text-sm font-semibold text-white mb-3">Your open orders</h3>
        {orders.length === 0 ? (
          <p className="text-xs text-gray-500">No resting orders on this chain.</p>
        ) : (
          <div className="space-y-2">
            {orders.map((o) => (
              <div
                key={o.order_id}
                className="flex items-center justify-between text-xs bg-black/20 rounded-lg p-3"
              >
                <span className="text-white font-medium">{o.symbol || o.market_id.slice(0, 10)}</span>
                <span className={o.is_long ? 'text-green-400' : 'text-red-400'}>
                  {o.is_long ? 'long' : 'short'}
                </span>
                <span className="text-gray-400">price {formatScaled(o.price)}</span>
                <span className="text-gray-400">
                  {formatScaled(o.filled)} / {formatScaled(o.quantity)}
                </span>
                <span className="text-gray-500">
                  {o.post_only ? 'post-only' : o.reduce_only ? 'reduce-only' : 'limit'}
                </span>
                <button
                  onClick={() => handleCancel(o)}
                  disabled={busy}
                  className="px-2 py-1 rounded bg-white/5 text-gray-300 hover:bg-white/10 disabled:opacity-50"
                >
                  Cancel
                </button>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Recent fills */}
      <div className="glass-dark rounded-2xl p-6">
        <h3 className="text-sm font-semibold text-white mb-3">Your recent fills</h3>
        {fills.length === 0 ? (
          <p className="text-xs text-gray-500">No executed fills indexed on this chain yet.</p>
        ) : (
          <div className="space-y-2">
            {fills.map((f) => (
              <div
                key={`${f.tx_hash}:${f.log_index}`}
                className="flex items-center justify-between text-xs bg-black/20 rounded-lg p-3"
              >
                <span className="text-white font-medium">
                  {f.symbol || f.market_id.slice(0, 10)}
                </span>
                <span className={f.is_maker ? 'text-blue-400' : 'text-amber-400'}>
                  {f.is_maker ? 'maker' : 'taker'}
                </span>
                <span className="text-gray-400">price {formatScaled(f.price)}</span>
                <span className="text-gray-400">qty {formatScaled(f.quantity)}</span>
                <span className="text-gray-500">fee {formatScaled(f.fee)}</span>
                <span className="text-gray-600 font-mono">
                  {f.tx_hash.slice(0, 10)}…#{f.log_index}
                </span>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
