'use client';

/**
 * On-chain advanced orders and TWAMM panel.
 *
 * Every order lives in the deployed `TigerSwapAdvancedOrders` contract, and a
 * long-term order lives in `TigerSwapTWAMM`. The order's condition (limit,
 * stop, TWAP slice) is evaluated on-chain against the price oracle, so the
 * server can never assert that a condition was met. The panel reads the
 * caller's orders straight from chain state and builds unsigned calldata that
 * the caller's own wallet signs; the server never holds a user key.
 *
 * Amounts are entered as human decimals and converted to the token's smallest
 * unit with exact integer arithmetic; no float round-trip happens anywhere.
 */

import { useMemo, useState } from 'react';
import { Loader2, SlidersHorizontal } from 'lucide-react';

import {
  useBuildOnChainCancelOrder,
  useBuildOnChainOrder,
  useBuildOnChainTwammOrder,
  useChains,
  useOnChainOrders,
  useOnChainTwammOrders,
  useTokens,
} from '@/hooks/useApi';
import { useSession } from '@/hooks/useSession';
import type { OnChainCall, OnChainOrder, OnChainTwammOrder, Token } from '@/lib/types';

const ORDER_TYPES = ['limit', 'stop_loss', 'take_profit', 'twap', 'gtc', 'ioc', 'post_only'] as const;
type OrderType = (typeof ORDER_TYPES)[number];

const PRICE_ORDER_TYPES: readonly OrderType[] = ['limit', 'gtc', 'ioc', 'post_only', 'take_profit'];
const STOP_ORDER_TYPES: readonly OrderType[] = ['stop_loss'];
const TWAP_ORDER_TYPES: readonly OrderType[] = ['twap'];
const NO_EXPIRY_ORDER_TYPES: readonly OrderType[] = ['gtc', 'ioc'];

const MAX_DECIMALS = 18;

async function sendCall(call: OnChainCall): Promise<string> {
  const { sendTransaction, waitForTransactionReceipt } = await import('@/lib/wallet');
  const hash = await sendTransaction({ to: call.to, data: call.data, value: call.value });
  await waitForTransactionReceipt(hash);
  return hash;
}

/**
 * Approves `amount` of `token` to the orders contract and waits for the
 * approval to be mined, so the escrow pull that follows cannot revert.
 */
async function approveToken(token: string, spender: string, amount: bigint): Promise<void> {
  const { approveErc20, waitForTransactionReceipt } = await import('@/lib/wallet');
  const hash = await approveErc20(token, spender, amount);
  await waitForTransactionReceipt(hash);
}

/** Converts a human decimal amount into an exact integer string of base units. */
function toBaseUnits(amount: string, decimals: number): string | null {
  const trimmed = amount.trim();
  if (!/^\d+(\.\d+)?$/.test(trimmed)) {
    return null;
  }
  const [whole, fraction = ''] = trimmed.split('.');
  if (fraction.length > decimals) {
    return null;
  }
  const padded = fraction.padEnd(decimals, '0');
  return `${whole}${padded}`.replace(/^0+(?=\d)/, '');
}

/** Renders an integer string of base units as a compact human number. */
function formatBaseUnits(value: string, decimals: number): string {
  try {
    const raw = BigInt(value);
    const negative = raw < 0n;
    const magnitude = negative ? -raw : raw;
    const divisor = 10n ** BigInt(decimals);
    const whole = magnitude / divisor;
    const fraction = (magnitude % divisor).toString().padStart(decimals, '0').slice(0, 4).replace(/0+$/, '');
    const rendered = fraction ? `${whole}.${fraction}` : whole.toString();
    return negative ? `-${rendered}` : rendered;
  } catch {
    return value;
  }
}

export function OnChainOrders() {
  const { user } = useSession();
  const chainsQuery = useChains();

  const chains = useMemo(
    () => (chainsQuery.data ?? []).filter((chain) => chain.isActive),
    [chainsQuery.data],
  );
  const [chainId, setChainId] = useState<number | null>(null);
  const activeChainId = chainId ?? chains[0]?.id ?? null;

  const tokensQuery = useTokens(activeChainId ?? undefined);
  const tokens = useMemo(() => tokensQuery.data ?? [], [tokensQuery.data]);
  const decimalsByAddress = useMemo(() => {
    const map = new Map<string, number>();
    for (const token of tokens) {
      map.set(token.address.toLowerCase(), token.decimals);
    }
    return map;
  }, [tokens]);

  const [orderType, setOrderType] = useState<OrderType>('limit');
  const [side, setSide] = useState<'buy' | 'sell'>('buy');
  const [tokenIn, setTokenIn] = useState('');
  const [tokenOut, setTokenOut] = useState('');
  const [amountIn, setAmountIn] = useState('');
  const [amountOutMin, setAmountOutMin] = useState('');
  const [price, setPrice] = useState('');
  const [stopPrice, setStopPrice] = useState('');
  const [expiresInHours, setExpiresInHours] = useState('24');
  const [twapParts, setTwapParts] = useState('4');
  const [twapInterval, setTwapInterval] = useState('3600');

  const [twammIsToken0, setTwammIsToken0] = useState(true);
  const [twammSellRate, setTwammSellRate] = useState('');
  const [twammDurationHours, setTwammDurationHours] = useState('24');

  const [formError, setFormError] = useState<string | null>(null);
  const [txHash, setTxHash] = useState<string | null>(null);

  const ordersQuery = useOnChainOrders(activeChainId);
  const twammQuery = useOnChainTwammOrders(activeChainId);
  const buildOrder = useBuildOnChainOrder();
  const cancelOrder = useBuildOnChainCancelOrder();
  const buildTwamm = useBuildOnChainTwammOrder();

  const selectedIn = tokens.find((token) => token.address === tokenIn) ?? null;
  const selectedOut = tokens.find((token) => token.address === tokenOut) ?? null;

  const run = async (action: () => Promise<string>) => {
    setFormError(null);
    setTxHash(null);
    try {
      setTxHash(await action());
    } catch (error) {
      setFormError(error instanceof Error ? error.message : 'Transaction failed');
    }
  };

  const handleCreateOrder = () => {
    if (!activeChainId || !selectedIn || !selectedOut) {
      setFormError('Select a chain and both tokens.');
      return;
    }
    const amountWei = toBaseUnits(amountIn, selectedIn.decimals);
    if (!amountWei) {
      setFormError(`Amount in must be a positive number with at most ${selectedIn.decimals} decimals.`);
      return;
    }
    const payload: Parameters<typeof buildOrder.mutate>[0] = {
      chain_id: activeChainId,
      order_type: orderType,
      side,
      token_in: selectedIn.address,
      token_out: selectedOut.address,
      amount_in: amountWei,
      approve: approveToken,
      send: sendCall,
    };
    if (amountOutMin.trim()) {
      const minWei = toBaseUnits(amountOutMin, selectedOut.decimals);
      if (!minWei) {
        setFormError('Minimum amount out is not a valid amount.');
        return;
      }
      payload.amount_out_min = minWei;
    }
    if (PRICE_ORDER_TYPES.includes(orderType)) {
      const priceWei = toBaseUnits(price, selectedOut.decimals);
      if (!priceWei) {
        setFormError('This order type needs a positive limit price.');
        return;
      }
      payload.price = priceWei;
    }
    if (STOP_ORDER_TYPES.includes(orderType)) {
      const stopWei = toBaseUnits(stopPrice, selectedOut.decimals);
      if (!stopWei) {
        setFormError('A stop order needs a positive stop price.');
        return;
      }
      payload.stop_price = stopWei;
    }
    if (TWAP_ORDER_TYPES.includes(orderType)) {
      if (!/^\d+$/.test(twapParts.trim()) || !/^\d+$/.test(twapInterval.trim())) {
        setFormError('TWAP needs an exact part count and interval in seconds.');
        return;
      }
      payload.twap_parts = Number(twapParts);
      payload.twap_interval = Number(twapInterval);
    }
    if (!NO_EXPIRY_ORDER_TYPES.includes(orderType)) {
      const hours = Number(expiresInHours);
      if (!Number.isFinite(hours) || hours <= 0) {
        setFormError('Expiry must be a positive number of hours.');
        return;
      }
      payload.expires_in = Math.round(hours * 3600);
    }
    buildOrder.mutate(payload);
  };

  const handleCreateTwamm = () => {
    if (!activeChainId) return;
    const sellToken = twammIsToken0 ? selectedIn : selectedOut;
    const sellRateWei = toBaseUnits(twammSellRate, sellToken?.decimals ?? MAX_DECIMALS);
    if (!sellRateWei) {
      setFormError('Sell rate must be a positive number in the sold token.');
      return;
    }
    const hours = Number(twammDurationHours);
    if (!Number.isFinite(hours) || hours <= 0) {
      setFormError('TWAMM duration must be a positive number of hours.');
      return;
    }
    // The whole sellRate * duration is escrowed up front, so the approval has
    // to cover the full escrow, not just the first interval.
    const durationSeconds = BigInt(Math.round(hours) * 3600);
    const escrow = (BigInt(sellRateWei) * durationSeconds).toString();
    buildTwamm.mutate({
      chain_id: activeChainId,
      is_token0: twammIsToken0,
      sell_rate: sellRateWei,
      duration_hours: Math.round(hours),
      sell_token: sellToken?.address ?? '',
      escrow,
      approve: approveToken,
      send: sendCall,
    });
  };

  const showPrice = PRICE_ORDER_TYPES.includes(orderType);
  const showStop = STOP_ORDER_TYPES.includes(orderType);
  const showTwap = TWAP_ORDER_TYPES.includes(orderType);
  const showExpiry = !NO_EXPIRY_ORDER_TYPES.includes(orderType);

  const decimalsFor = (addressValue: string): number =>
    decimalsByAddress.get(addressValue.toLowerCase()) ?? MAX_DECIMALS;

  const inputClass =
    'bg-white/10 border border-white/20 rounded-xl px-4 py-2 text-white focus:outline-none focus:border-tiger-orange';

  return (
    <div className="glass-dark rounded-2xl p-6 space-y-6">
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-display font-bold text-white">On-chain Orders</h2>
        <SlidersHorizontal className="w-5 h-5 text-tiger-orange" />
      </div>

      {!user && (
        <p className="text-gray-500 text-xs">Sign in and connect a wallet to place an on-chain order.</p>
      )}

      {formError && (
        <p className="text-red-400 text-xs" role="alert">
          {formError}
        </p>
      )}
      {txHash && (
        <p className="text-green-400 text-xs" role="status">
          Submitted: {txHash}
        </p>
      )}

      <section className="space-y-3">
        <h3 className="text-sm font-semibold text-white">Advanced order</h3>
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
          <select
            aria-label="Chain"
            value={activeChainId ?? ''}
            onChange={(event) => setChainId(Number(event.target.value))}
            className={inputClass}
          >
            {chains.map((chain) => (
              <option key={chain.id} value={chain.id} className="bg-gray-900">
                {chain.name}
              </option>
            ))}
          </select>
          <select
            aria-label="Order type"
            value={orderType}
            onChange={(event) => setOrderType(event.target.value as OrderType)}
            className={inputClass}
          >
            {ORDER_TYPES.map((type) => (
              <option key={type} value={type} className="bg-gray-900">
                {type}
              </option>
            ))}
          </select>
          <select
            aria-label="Side"
            value={side}
            onChange={(event) => setSide(event.target.value as 'buy' | 'sell')}
            className={inputClass}
          >
            <option value="buy" className="bg-gray-900">
              buy
            </option>
            <option value="sell" className="bg-gray-900">
              sell
            </option>
          </select>
          <select
            aria-label="Token in"
            value={tokenIn}
            onChange={(event) => setTokenIn(event.target.value)}
            className={inputClass}
          >
            <option value="" className="bg-gray-900">
              Token in…
            </option>
            {tokens.map((token: Token) => (
              <option key={`in-${token.address}`} value={token.address} className="bg-gray-900">
                {token.symbol}
              </option>
            ))}
          </select>
          <select
            aria-label="Token out"
            value={tokenOut}
            onChange={(event) => setTokenOut(event.target.value)}
            className={inputClass}
          >
            <option value="" className="bg-gray-900">
              Token out…
            </option>
            {tokens.map((token: Token) => (
              <option key={`out-${token.address}`} value={token.address} className="bg-gray-900">
                {token.symbol}
              </option>
            ))}
          </select>
          <input
            aria-label="Amount in"
            type="text"
            inputMode="decimal"
            placeholder="Amount in"
            value={amountIn}
            onChange={(event) => setAmountIn(event.target.value)}
            className={inputClass}
          />
          <input
            aria-label="Minimum amount out"
            type="text"
            inputMode="decimal"
            placeholder="Min amount out (optional)"
            value={amountOutMin}
            onChange={(event) => setAmountOutMin(event.target.value)}
            className={inputClass}
          />
          {showPrice && (
            <input
              aria-label="Limit price"
              type="text"
              inputMode="decimal"
              placeholder="Limit price"
              value={price}
              onChange={(event) => setPrice(event.target.value)}
              className={inputClass}
            />
          )}
          {showStop && (
            <input
              aria-label="Stop price"
              type="text"
              inputMode="decimal"
              placeholder="Stop price"
              value={stopPrice}
              onChange={(event) => setStopPrice(event.target.value)}
              className={inputClass}
            />
          )}
          {showTwap && (
            <>
              <input
                aria-label="TWAP parts"
                type="number"
                min={1}
                value={twapParts}
                onChange={(event) => setTwapParts(event.target.value)}
                className={inputClass}
              />
              <input
                aria-label="TWAP interval in seconds"
                type="number"
                min={1}
                value={twapInterval}
                onChange={(event) => setTwapInterval(event.target.value)}
                className={inputClass}
              />
            </>
          )}
          {showExpiry && (
            <input
              aria-label="Expiry in hours"
              type="number"
              min={1}
              value={expiresInHours}
              onChange={(event) => setExpiresInHours(event.target.value)}
              className={inputClass}
            />
          )}
        </div>
        <button
          onClick={handleCreateOrder}
          disabled={buildOrder.isPending || !activeChainId}
          className="px-4 py-2 rounded-xl font-medium bg-tiger-orange text-white disabled:bg-gray-700 disabled:text-gray-500"
        >
          {buildOrder.isPending ? <Loader2 className="w-4 h-4 animate-spin" /> : 'Place on-chain order'}
        </button>
      </section>

      <section>
        <h3 className="text-sm font-semibold text-white mb-2">My on-chain orders</h3>
        {ordersQuery.error ? (
          <p className="text-red-400 text-xs" role="alert">
            {ordersQuery.error.message}
          </p>
        ) : (ordersQuery.data ?? []).length === 0 ? (
          <p className="text-gray-500 text-xs">{ordersQuery.isPending ? 'Loading…' : 'None yet.'}</p>
        ) : (
          <ul className="space-y-2">
            {(ordersQuery.data ?? []).map((order: OnChainOrder) => (
              <li
                key={order.order_id}
                className="bg-white/5 rounded-xl p-3 flex items-center justify-between"
              >
                <span className="text-xs text-gray-300">
                  {order.order_type} {order.side}{' '}
                  {formatBaseUnits(order.amount_in, decimalsFor(order.token_in))} · filled{' '}
                  {formatBaseUnits(order.filled_amount, decimalsFor(order.token_in))} ·{' '}
                  {order.status}
                </span>
                {order.status !== 'filled' && order.status !== 'cancelled' && activeChainId && (
                  <button
                    onClick={() =>
                      run(() =>
                        cancelOrder.mutateAsync({
                          order_id: order.order_id,
                          chain_id: activeChainId,
                          send: sendCall,
                        }),
                      )
                    }
                    disabled={cancelOrder.isPending}
                    className="px-3 py-1 rounded-lg text-xs bg-white/10 text-gray-200 hover:bg-white/20"
                  >
                    Cancel
                  </button>
                )}
              </li>
            ))}
          </ul>
        )}
      </section>

      <section className="border-t border-white/10 pt-4 space-y-3">
        <h3 className="text-sm font-semibold text-white">TWAMM schedule</h3>
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
          <select
            aria-label="TWAMM sell token side"
            value={twammIsToken0 ? 'token0' : 'token1'}
            onChange={(event) => setTwammIsToken0(event.target.value === 'token0')}
            className={inputClass}
          >
            <option value="token0" className="bg-gray-900">
              sell token in
            </option>
            <option value="token1" className="bg-gray-900">
              sell token out
            </option>
          </select>
          <input
            aria-label="TWAMM sell rate"
            type="text"
            inputMode="decimal"
            placeholder="Sell rate (per second)"
            value={twammSellRate}
            onChange={(event) => setTwammSellRate(event.target.value)}
            className={inputClass}
          />
          <input
            aria-label="TWAMM duration in hours"
            type="number"
            min={1}
            value={twammDurationHours}
            onChange={(event) => setTwammDurationHours(event.target.value)}
            className={inputClass}
          />
        </div>
        <button
          onClick={handleCreateTwamm}
          disabled={buildTwamm.isPending || !activeChainId}
          className="px-4 py-2 rounded-xl font-medium bg-tiger-orange text-white disabled:bg-gray-700 disabled:text-gray-500"
        >
          {buildTwamm.isPending ? <Loader2 className="w-4 h-4 animate-spin" /> : 'Schedule TWAMM'}
        </button>

        {twammQuery.error ? (
          <p className="text-red-400 text-xs" role="alert">
            {twammQuery.error.message}
          </p>
        ) : (twammQuery.data ?? []).length === 0 ? (
          <p className="text-gray-500 text-xs">{twammQuery.isPending ? 'Loading…' : 'No schedules yet.'}</p>
        ) : (
          <ul className="space-y-2">
            {(twammQuery.data ?? []).map((order: OnChainTwammOrder) => (
              <li key={order.order_id} className="bg-white/5 rounded-xl p-3">
                <span className="text-xs text-gray-300">
                  {order.is_token0 ? 'token0' : 'token1'} · rate{' '}
                  {formatBaseUnits(order.sell_rate, MAX_DECIMALS)} · escrow{' '}
                  {formatBaseUnits(order.escrow, MAX_DECIMALS)} ·{' '}
                  {order.active ? 'active' : 'inactive'}
                </span>
              </li>
            ))}
          </ul>
        )}
      </section>
    </div>
  );
}
