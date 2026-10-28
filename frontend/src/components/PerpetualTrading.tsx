'use client';

/**
 * Perpetuals trading panel.
 *
 * Markets, mark price, funding and open interest all come from the API. When
 * the perp venue is not enabled the panel says so instead of showing an
 * invented price or an invented funding rate.
 *
 * Opening a position posts to the API, which prices it from TigerSwap's own
 * oracle and persists it; nothing is executed client-side. The caller's open
 * positions are listed below the order form.
 */

import { useMemo, useState } from 'react';
import {
  AlertTriangle,
  BarChart3,
  Layers,
  Loader2,
  TrendingDown,
  TrendingUp,
  X,
} from 'lucide-react';

import {
  useClosePerpPosition,
  useOpenPerpPosition,
  usePerpMarkets,
  usePerpPositions,
} from '@/hooks/useApi';
import { isAuthenticated } from '@/lib/auth';
import type { PerpMarket, PerpPosition } from '@/lib/types';

type PositionSide = 'long' | 'short';

const LEVERAGE_OPTIONS = [1, 2, 3, 5, 10, 20, 50] as const;

export function PerpetualTrading() {
  const { data: markets, error, isPending } = usePerpMarkets();
  const positionsQuery = usePerpPositions();
  const openPosition = useOpenPerpPosition();
  const closePosition = useClosePerpPosition();

  const [selectedSymbol, setSelectedSymbol] = useState<string | null>(null);
  const [positionSide, setPositionSide] = useState<PositionSide>('long');
  const [leverage, setLeverage] = useState(10);
  const [amount, setAmount] = useState('');
  const [formError, setFormError] = useState<string | null>(null);

  const selectedMarket: PerpMarket | null = useMemo(() => {
    if (!markets || markets.length === 0) {
      return null;
    }
    return markets.find((market) => market.symbol === selectedSymbol) ?? markets[0];
  }, [markets, selectedSymbol]);

  const formatPrice = (value: number) => {
    if (value >= 1000) return value.toLocaleString('en-US', { maximumFractionDigits: 2 });
    if (value >= 1) return value.toFixed(2);
    return value.toFixed(6);
  };

  if (isPending) {
    return (
      <div className="glass-dark rounded-2xl p-8 text-center text-gray-400">
        <Loader2 className="w-8 h-8 mx-auto mb-3 animate-spin" />
        Loading perpetual markets…
      </div>
    );
  }

  if (error) {
    return (
      <div className="glass-dark rounded-2xl p-8 text-center">
        <AlertTriangle className="w-10 h-10 text-yellow-500 mx-auto mb-4" />
        <h3 className="text-white font-semibold mb-2">Perpetuals unavailable</h3>
        <p className="text-gray-400 text-sm" role="alert">
          {error.message}
        </p>
      </div>
    );
  }

  if (!selectedMarket) {
    return (
      <div className="glass-dark rounded-2xl p-8 text-center">
        <Layers className="w-10 h-10 text-gray-500 mx-auto mb-4" />
        <h3 className="text-white font-semibold mb-2">No perpetual markets</h3>
        <p className="text-gray-400 text-sm">
          No perpetual markets are currently listed. Markets appear here once they are enabled.
        </p>
      </div>
    );
  }

  const fundingPercent = selectedMarket.fundingRate * 100;
  // A market's listing caps leverage; the selected value is clamped so a market
  // switch can never submit more leverage than the listing allows.
  const effectiveLeverage = Math.min(leverage, selectedMarket.maxLeverage);
  const collateralValue = Number(amount);
  const collateralValid = Number.isFinite(collateralValue) && collateralValue > 0;
  const canSubmit =
    isAuthenticated() && selectedMarket.isActive && collateralValid && !openPosition.isPending;

  const handleOpen = () => {
    setFormError(null);

    if (!isAuthenticated()) {
      setFormError('Sign in to trade perpetuals.');
      return;
    }
    if (!selectedMarket.isActive) {
      setFormError('This market is not currently tradable.');
      return;
    }
    if (!collateralValid) {
      setFormError('Enter a collateral amount greater than zero.');
      return;
    }

    const notional = collateralValue * effectiveLeverage;
    openPosition.mutate(
      {
        pair: selectedMarket.symbol,
        side: positionSide,
        size: notional.toFixed(8),
        collateral: collateralValue.toFixed(8),
        leverage: effectiveLeverage,
      },
      {
        onSuccess: () => setAmount(''),
        onError: (mutationError) => setFormError(mutationError.message),
      },
    );
  };

  const handleClose = (position: PerpPosition) => {
    setFormError(null);
    closePosition.mutate(position.id, {
      onError: (mutationError) => setFormError(mutationError.message),
    });
  };

  const positions = positionsQuery.data ?? [];

  return (
    <div className="glass-dark rounded-2xl p-6">
      <div className="flex items-center justify-between mb-4">
        <h2 className="text-lg font-display font-bold text-white">Perpetuals</h2>
        <BarChart3 className="w-5 h-5 text-tiger-orange" />
      </div>

      {/* Market selector */}
      <div className="mb-4">
        <label htmlFor="perp-market" className="text-gray-400 text-sm mb-2 block">
          Market
        </label>
        <select
          id="perp-market"
          value={selectedMarket.symbol}
          onChange={(event) => setSelectedSymbol(event.target.value)}
          className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white focus:outline-none focus:border-tiger-orange"
        >
          {markets.map((market) => (
            <option key={market.marketId} value={market.symbol} className="bg-gray-900">
              {market.symbol} — up to {market.maxLeverage}x
            </option>
          ))}
        </select>
      </div>

      {/* Market data */}
      <div className="grid grid-cols-2 gap-3 mb-4">
        <div className="bg-white/5 rounded-xl p-3">
          <p className="text-gray-400 text-xs mb-1">Mark price</p>
          <p className="text-white font-semibold">${formatPrice(selectedMarket.markPrice)}</p>
        </div>
        <div className="bg-white/5 rounded-xl p-3">
          <p className="text-gray-400 text-xs mb-1">Index price</p>
          <p className="text-white font-semibold">${formatPrice(selectedMarket.indexPrice)}</p>
        </div>
        <div className="bg-white/5 rounded-xl p-3">
          <p className="text-gray-400 text-xs mb-1">Funding rate</p>
          <p className={`font-semibold ${fundingPercent >= 0 ? 'text-green-400' : 'text-red-400'}`}>
            {fundingPercent >= 0 ? '+' : ''}
            {fundingPercent.toFixed(4)}%
          </p>
        </div>
        <div className="bg-white/5 rounded-xl p-3">
          <p className="text-gray-400 text-xs mb-1">Open interest</p>
          <p className="text-white font-semibold">
            L ${selectedMarket.openInterestLong.toLocaleString()} / S{' '}
            {selectedMarket.openInterestShort.toLocaleString()}
          </p>
        </div>
      </div>

      {/* Side */}
      <div className="flex space-x-2 mb-4">
        <button
          onClick={() => setPositionSide('long')}
          className={`flex-1 flex items-center justify-center space-x-2 py-3 rounded-xl font-medium transition-colors ${
            positionSide === 'long'
              ? 'bg-green-500 text-white'
              : 'bg-white/5 text-gray-400 hover:text-white'
          }`}
        >
          <TrendingUp className="w-4 h-4" />
          <span>Long</span>
        </button>
        <button
          onClick={() => setPositionSide('short')}
          className={`flex-1 flex items-center justify-center space-x-2 py-3 rounded-xl font-medium transition-colors ${
            positionSide === 'short'
              ? 'bg-red-500 text-white'
              : 'bg-white/5 text-gray-400 hover:text-white'
          }`}
        >
          <TrendingDown className="w-4 h-4" />
          <span>Short</span>
        </button>
      </div>

      {/* Leverage */}
      <div className="mb-4">
        <label className="text-gray-400 text-sm mb-2 block">Leverage</label>
        <div className="flex flex-wrap gap-2">
          {LEVERAGE_OPTIONS.filter((option) => option <= selectedMarket.maxLeverage).map((option) => (
            <button
              key={option}
              onClick={() => setLeverage(option)}
              className={`px-3 py-2 rounded-lg text-sm font-medium transition-colors ${
                effectiveLeverage === option
                  ? 'bg-tiger-orange text-white'
                  : 'bg-white/5 text-gray-400 hover:text-white'
              }`}
            >
              {option}x
            </button>
          ))}
        </div>
      </div>

      {/* Collateral */}
      <div className="mb-4">
        <label htmlFor="perp-amount" className="text-gray-400 text-sm mb-2 block">
          Collateral
        </label>
        <input
          id="perp-amount"
          type="text"
          inputMode="decimal"
          value={amount}
          onChange={(event) => setAmount(event.target.value)}
          placeholder="0.00"
          className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white focus:outline-none focus:border-tiger-orange"
        />
        {collateralValid && (
          <p className="text-gray-500 text-xs mt-2">
            Position notional ≈ ${(collateralValue * effectiveLeverage).toLocaleString()}
          </p>
        )}
      </div>

      <div className="flex items-start space-x-2 p-3 bg-yellow-500/10 border border-yellow-500/20 rounded-xl mb-4">
        <AlertTriangle className="w-4 h-4 text-yellow-400 shrink-0 mt-0.5" />
        <p className="text-yellow-400 text-xs">
          High leverage increases liquidation risk. Only trade with funds you can afford to lose.
        </p>
      </div>

      {formError && (
        <p className="text-red-400 text-xs mb-3" role="alert">
          {formError}
        </p>
      )}

      <button
        onClick={handleOpen}
        disabled={!canSubmit}
        className={`w-full py-4 rounded-xl font-semibold transition-colors ${
          canSubmit
            ? positionSide === 'long'
              ? 'bg-green-500 hover:bg-green-600 text-white'
              : 'bg-red-500 hover:bg-red-600 text-white'
            : 'bg-gray-700 text-gray-500 cursor-not-allowed'
        }`}
      >
        {openPosition.isPending ? (
          <span className="inline-flex items-center space-x-2">
            <Loader2 className="w-4 h-4 animate-spin" />
            <span>Submitting…</span>
          </span>
        ) : (
          `${positionSide === 'long' ? 'Open long' : 'Open short'} ${selectedMarket.symbol}`
        )}
      </button>

      <div className="mt-6 pt-6 border-t border-white/10">
        <h3 className="text-sm font-semibold text-white mb-4">Open positions</h3>

        {positionsQuery.error && (
          <p className="text-red-400 text-xs mb-3" role="alert">
            {positionsQuery.error.message}
          </p>
        )}

        {positions.length === 0 ? (
          <div className="text-center py-8 text-gray-500">
            <Layers className="w-8 h-8 mx-auto mb-2 opacity-50" />
            <p className="text-sm">No open positions</p>
          </div>
        ) : (
          <ul className="space-y-3">
            {positions.map((position) => (
              <li key={position.id} className="bg-white/5 rounded-xl p-3">
                <div className="flex items-center justify-between mb-2">
                  <div className="flex items-center space-x-2">
                    <span className="text-white text-sm font-semibold">{position.pair}</span>
                    <span
                      className={`text-xs px-2 py-0.5 rounded-full ${
                        position.side === 'long'
                          ? 'bg-green-500/20 text-green-400'
                          : 'bg-red-500/20 text-red-400'
                      }`}
                    >
                      {position.side} {position.leverage}x
                    </span>
                  </div>
                  <button
                    onClick={() => handleClose(position)}
                    disabled={closePosition.isPending}
                    title="Close position"
                    className="text-gray-400 hover:text-white disabled:opacity-50"
                  >
                    <X className="w-4 h-4" />
                  </button>
                </div>
                <dl className="grid grid-cols-3 gap-2 text-xs">
                  <div>
                    <dt className="text-gray-500">Size</dt>
                    <dd className="text-white">{position.size}</dd>
                  </div>
                  <div>
                    <dt className="text-gray-500">Entry</dt>
                    <dd className="text-white">{position.entry_price}</dd>
                  </div>
                  <div>
                    <dt className="text-gray-500">PnL</dt>
                    <dd className={Number(position.pnl) >= 0 ? 'text-green-400' : 'text-red-400'}>
                      {position.pnl}
                    </dd>
                  </div>
                </dl>
              </li>
            ))}
          </ul>
        )}
      </div>
    </div>
  );
}
