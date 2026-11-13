'use client';

/**
 * On-chain perpetuals panel.
 *
 * This is the non-custodial derivatives venue: positions, margin, funding and
 * liquidation all live in the deployed `TigerSwapPerpetuals` contract, and the
 * chain is the source of truth. Markets and positions are read straight from the
 * contract; opening, closing and margin changes are built server-side as
 * unsigned calldata and signed by the caller's own wallet. The server never
 * holds a user key, so a user can verify every position with an RPC endpoint.
 *
 * Amounts entered here are human-readable units of the settlement token (e.g.
 * USDC). The API scales them to 18-decimal base units before encoding the call.
 */

import { useMemo, useState } from 'react';
import { AlertTriangle, Loader2, TrendingDown, TrendingUp } from 'lucide-react';

import {
  useBuildOnChainClose,
  useBuildOnChainLiquidity,
  useBuildOnChainMargin,
  useBuildOnChainOpen,
  useOnChainPerpMarkets,
  useOnChainPerpPositions,
} from '@/hooks/useApi';
import { useSession } from '@/hooks/useSession';
import type { OnChainCall, OnChainPerpMarket, OnChainPerpPosition } from '@/lib/types';

const RATIO_PRECISION = 10n ** 18n;
const COLLATERAL_DECIMALS = 18;

type PositionSide = 'long' | 'short';

/** Renders a 1e18-scaled integer string as a compact human number. */
function formatScaled(value: string, decimals = 18): string {
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

/** Converts a 1e18-scaled ratio to a percentage string. */
function formatRatioPercent(value: string): string {
  try {
    const scaled = (BigInt(value) * 10000n) / RATIO_PRECISION;
    return `${Number(scaled) / 100}%`;
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
  if (fraction.length > COLLATERAL_DECIMALS) {
    return null;
  }
  const padded = fraction.padEnd(COLLATERAL_DECIMALS, '0');
  return `${whole}${padded}`.replace(/^0+(?=\d)/, '');
}

async function sendCall(call: OnChainCall): Promise<string> {
  const { sendTransaction } = await import('@/lib/wallet');
  return sendTransaction({ to: call.to, data: call.data, value: call.value });
}

export function OnChainPerpetuals() {
  const { user } = useSession();
  const marketsQuery = useOnChainPerpMarkets();

  const [selectedKey, setSelectedKey] = useState<string | null>(null);
  const [side, setSide] = useState<PositionSide>('long');
  const [collateral, setCollateral] = useState('');
  const [size, setSize] = useState('');
  const [formError, setFormError] = useState<string | null>(null);
  const [txHash, setTxHash] = useState<string | null>(null);

  const markets = useMemo(() => marketsQuery.data ?? [], [marketsQuery.data]);
  const selected = useMemo(
    () => markets.find((m) => `${m.chain_id}:${m.symbol}` === selectedKey) ?? markets[0] ?? null,
    [markets, selectedKey],
  );
  const chainId = selected?.chain_id ?? null;

  const positionsQuery = useOnChainPerpPositions(chainId);
  const openPosition = useBuildOnChainOpen();
  const closePosition = useBuildOnChainClose();
  const changeMargin = useBuildOnChainMargin();
  const addLiquidity = useBuildOnChainLiquidity();

  const run = async (action: () => Promise<string>) => {
    setFormError(null);
    setTxHash(null);
    try {
      const hash = await action();
      setTxHash(hash);
    } catch (error) {
      setFormError(error instanceof Error ? error.message : 'Transaction failed');
    }
  };

  const handleOpen = () => {
    if (!selected) return;
    const collateralWei = toBaseUnits(collateral);
    const sizeWei = toBaseUnits(size);
    if (!collateralWei || !sizeWei) {
      setFormError('Enter collateral and size as positive decimal amounts');
      return;
    }
    void run(async () =>
      openPosition.mutateAsync({
        chain_id: selected.chain_id,
        symbol: selected.symbol,
        size: sizeWei,
        collateral: collateralWei,
        side,
        send: sendCall,
      }),
    );
  };

  const handleClose = (position: OnChainPerpPosition) => {
    void run(async () =>
      closePosition.mutateAsync({
        position_id: position.position_id,
        chain_id: position.chain_id,
        send: sendCall,
      }),
    );
  };

  const handleMargin = (position: OnChainPerpPosition, add: boolean) => {
    const amountWei = toBaseUnits(collateral);
    if (!amountWei) {
      setFormError('Enter a positive margin amount first');
      return;
    }
    void run(async () =>
      changeMargin.mutateAsync({
        chain_id: position.chain_id,
        position_id: position.position_id,
        amount: amountWei,
        add,
        send: sendCall,
      }),
    );
  };

  const handleLiquidity = () => {
    if (!selected) return;
    const amountWei = toBaseUnits(collateral);
    if (!amountWei) {
      setFormError('Enter a positive liquidity amount first');
      return;
    }
    void run(async () =>
      addLiquidity.mutateAsync({
        chain_id: selected.chain_id,
        symbol: selected.symbol,
        amount: amountWei,
        send: sendCall,
      }),
    );
  };

  const busy =
    openPosition.isPending ||
    closePosition.isPending ||
    changeMargin.isPending ||
    addLiquidity.isPending;

  if (marketsQuery.error) {
    return (
      <div className="glass-dark rounded-2xl p-6">
        <div className="flex items-start space-x-3 text-yellow-400">
          <AlertTriangle className="w-5 h-5 mt-0.5" />
          <div>
            <h2 className="text-lg font-semibold">On-chain perpetuals unavailable</h2>
            <p className="text-sm text-gray-400 mt-1">
              {marketsQuery.error instanceof Error
                ? marketsQuery.error.message
                : 'The on-chain perps venue is not reachable.'}
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
        <span>Loading on-chain markets…</span>
      </div>
    );
  }

  if (markets.length === 0) {
    return (
      <div className="glass-dark rounded-2xl p-6">
        <h2 className="text-lg font-semibold text-white">On-chain perpetuals</h2>
        <p className="text-sm text-gray-400 mt-2">
          No on-chain perpetual market is configured. An operator creates a market on the
          deployed contract and lists it; the chain is the source of truth, so nothing is
          shown until one exists.
        </p>
      </div>
    );
  }

  const positions = positionsQuery.data ?? [];

  return (
    <div className="space-y-6">
      <div className="glass-dark rounded-2xl p-6">
        <div className="flex items-center justify-between mb-4">
          <div>
            <h2 className="text-xl font-display font-bold text-white">On-chain Perpetuals</h2>
            <p className="text-xs text-gray-500 mt-1">
              Non-custodial: positions live in the contract, your wallet signs every order.
            </p>
          </div>
          {selected && (
            <span className="text-xs px-2 py-1 rounded-lg bg-green-500/10 text-green-400 border border-green-500/20">
              chain {selected.chain_id}
            </span>
          )}
        </div>

        {/* Market selector */}
        <div className="flex flex-wrap gap-2 mb-6">
          {markets.map((market) => {
            const key = `${market.chain_id}:${market.symbol}`;
            const active = selected ? `${selected.chain_id}:${selected.symbol}` === key : false;
            return (
              <button
                key={key}
                onClick={() => setSelectedKey(key)}
                className={`px-3 py-2 rounded-lg text-sm font-medium transition-colors ${
                  active
                    ? 'bg-tiger-orange/20 text-tiger-orange border border-tiger-orange/30'
                    : 'bg-white/5 text-gray-400 border border-white/10 hover:text-white'
                }`}
              >
                {market.symbol}
                {!market.is_active && <span className="ml-2 text-xs text-yellow-400">paused</span>}
              </button>
            );
          })}
        </div>

        {selected && <MarketStats market={selected} />}

        {/* Order form */}
        <div className="mt-6 grid grid-cols-2 gap-3">
          <button
            onClick={() => setSide('long')}
            className={`flex items-center justify-center space-x-2 py-3 rounded-xl font-semibold transition-colors ${
              side === 'long'
                ? 'bg-green-500/20 text-green-400 border border-green-500/30'
                : 'bg-white/5 text-gray-400 border border-white/10'
            }`}
          >
            <TrendingUp className="w-4 h-4" />
            <span>Long</span>
          </button>
          <button
            onClick={() => setSide('short')}
            className={`flex items-center justify-center space-x-2 py-3 rounded-xl font-semibold transition-colors ${
              side === 'short'
                ? 'bg-red-500/20 text-red-400 border border-red-500/30'
                : 'bg-white/5 text-gray-400 border border-white/10'
            }`}
          >
            <TrendingDown className="w-4 h-4" />
            <span>Short</span>
          </button>
        </div>

        <div className="mt-4 grid grid-cols-1 sm:grid-cols-2 gap-4">
          <label className="block">
            <span className="text-gray-400 text-sm mb-1 block">Collateral</span>
            <input
              value={collateral}
              onChange={(e) => setCollateral(e.target.value)}
              inputMode="decimal"
              placeholder="0.0"
              className="w-full bg-white/5 border border-white/10 rounded-xl px-4 py-3 text-white focus:outline-none focus:border-tiger-orange"
            />
          </label>
          <label className="block">
            <span className="text-gray-400 text-sm mb-1 block">Size (notional units)</span>
            <input
              value={size}
              onChange={(e) => setSize(e.target.value)}
              inputMode="decimal"
              placeholder="0.0"
              className="w-full bg-white/5 border border-white/10 rounded-xl px-4 py-3 text-white focus:outline-none focus:border-tiger-orange"
            />
          </label>
        </div>

        {formError && <p className="text-red-400 text-sm mt-3">{formError}</p>}
        {txHash && <p className="text-green-400 text-sm mt-3 break-all">Submitted: {txHash}</p>}

        {!user ? (
          <p className="text-yellow-400 text-sm mt-4">Sign in to trade on-chain perpetuals.</p>
        ) : (
          <div className="mt-4 flex flex-col sm:flex-row gap-3">
            <button
              onClick={handleOpen}
              disabled={busy || !selected?.is_active}
              className="flex-1 flex items-center justify-center space-x-2 px-4 py-3 bg-gradient-to-r from-tiger-orange to-tiger-accent rounded-xl text-white font-semibold hover:opacity-90 transition-opacity disabled:opacity-50"
            >
              {busy && <Loader2 className="w-4 h-4 animate-spin" />}
              <span>Open {side}</span>
            </button>
            <button
              onClick={handleLiquidity}
              disabled={busy || !selected?.is_active}
              className="px-4 py-3 bg-white/5 border border-white/10 rounded-xl text-white font-medium hover:bg-white/10 transition-colors disabled:opacity-50"
            >
              Add liquidity
            </button>
          </div>
        )}
      </div>

      {/* Positions */}
      <div className="glass-dark rounded-2xl p-6">
        <h3 className="text-lg font-semibold text-white mb-4">Your on-chain positions</h3>
        {!user ? (
          <p className="text-gray-400 text-sm">Sign in to see your on-chain positions.</p>
        ) : positionsQuery.error ? (
          <p className="text-red-400 text-sm">
            {positionsQuery.error instanceof Error
              ? positionsQuery.error.message
              : 'Could not load positions.'}
          </p>
        ) : positions.length === 0 ? (
          <p className="text-gray-400 text-sm">No open on-chain positions on this chain.</p>
        ) : (
          <div className="space-y-3">
            {positions.map((position) => (
              <PositionRow
                key={position.position_id}
                position={position}
                busy={busy}
                onClose={() => handleClose(position)}
                onAddMargin={() => handleMargin(position, true)}
                onRemoveMargin={() => handleMargin(position, false)}
              />
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

function MarketStats({ market }: { market: OnChainPerpMarket }) {
  const maxLeverage = (() => {
    try {
      return `${Number((BigInt(market.max_leverage) * 100n) / RATIO_PRECISION) / 100}x`;
    } catch {
      return market.max_leverage;
    }
  })();

  return (
    <div className="grid grid-cols-2 sm:grid-cols-4 gap-4 p-4 bg-white/5 rounded-xl">
      <Stat label="Max leverage" value={maxLeverage} />
      <Stat label="Open interest" value={formatScaled(market.open_interest)} />
      <Stat label="Max OI" value={formatScaled(market.max_open_interest)} />
      <Stat label="Maintenance" value={formatRatioPercent(market.maintenance_margin_rate)} />
      <Stat label="Open fee" value={formatRatioPercent(market.open_fee)} />
      <Stat label="Liquidation fee" value={formatRatioPercent(market.liquidation_fee)} />
    </div>
  );
}

function Stat({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <div className="text-xs text-gray-500">{label}</div>
      <div className="text-white font-medium">{value}</div>
    </div>
  );
}

function PositionRow({
  position,
  busy,
  onClose,
  onAddMargin,
  onRemoveMargin,
}: {
  position: OnChainPerpPosition;
  busy: boolean;
  onClose: () => void;
  onAddMargin: () => void;
  onRemoveMargin: () => void;
}) {
  const isLong = position.side === 'long';
  const pnl = BigInt(position.unrealized_pnl);
  const pnlPositive = pnl >= 0n;

  return (
    <div className="p-4 bg-white/5 rounded-xl border border-white/10">
      <div className="flex items-center justify-between">
        <div className="flex items-center space-x-3">
          <span
            className={`px-2 py-1 rounded-lg text-xs font-semibold ${
              isLong ? 'bg-green-500/20 text-green-400' : 'bg-red-500/20 text-red-400'
            }`}
          >
            {position.side.toUpperCase()}
          </span>
          <span className="text-white font-medium">{position.symbol}</span>
        </div>
        <span className={`font-semibold ${pnlPositive ? 'text-green-400' : 'text-red-400'}`}>
          {formatScaled(position.unrealized_pnl)}
        </span>
      </div>

      <div className="mt-3 grid grid-cols-2 sm:grid-cols-4 gap-3 text-sm">
        <Stat label="Size" value={formatScaled(position.size)} />
        <Stat label="Collateral" value={formatScaled(position.collateral)} />
        <Stat label="Entry" value={formatScaled(position.entry_price)} />
        <Stat label="Mark" value={formatScaled(position.mark_price)} />
        <Stat label="Accrued funding" value={formatScaled(position.accrued_funding)} />
        <Stat label="Funded collateral" value={formatScaled(position.funded_collateral)} />
      </div>

      <div className="mt-3 flex gap-2">
        <button
          onClick={onClose}
          disabled={busy}
          className="px-3 py-2 bg-red-500/10 border border-red-500/20 rounded-lg text-red-400 text-sm hover:bg-red-500/20 transition-colors disabled:opacity-50"
        >
          Close
        </button>
        <button
          onClick={onAddMargin}
          disabled={busy}
          className="px-3 py-2 bg-white/5 border border-white/10 rounded-lg text-gray-300 text-sm hover:bg-white/10 transition-colors disabled:opacity-50"
        >
          Add margin
        </button>
        <button
          onClick={onRemoveMargin}
          disabled={busy}
          className="px-3 py-2 bg-white/5 border border-white/10 rounded-lg text-gray-300 text-sm hover:bg-white/10 transition-colors disabled:opacity-50"
        >
          Remove margin
        </button>
      </div>
    </div>
  );
}
