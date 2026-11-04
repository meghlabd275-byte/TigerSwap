'use client';

/**
 * Options trading panel.
 *
 * A series is a fully collateralised pool: buying pays a Black-Scholes premium
 * out of the caller's real ledger balance, an LP deposits the settlement asset
 * to underwrite it, and a settled position pays its intrinsic value. Every
 * number here comes from the API, including the greeks; when a value is
 * unavailable the panel shows a dash rather than a fabricated figure.
 */

import { useMemo, useState } from 'react';
import { AlertTriangle, LineChart, Loader2, TrendingDown, TrendingUp, Wallet } from 'lucide-react';

import {
  useAddOptionCollateral,
  useBuyOption,
  useExerciseOption,
  useOptionPositions,
  useOptionSeries,
  useRemoveOptionCollateral,
  useWithdrawOptionLiquidity,
} from '@/hooks/useApi';
import { isAuthenticated } from '@/lib/auth';

/** Formats a 1e18 fixed-point decimal string for display. */
function formatWad(raw: string, places = 4): string {
  if (raw === '' || raw === undefined || raw === null) {
    return '—';
  }
  const value = Number(raw);
  if (!Number.isFinite(value)) {
    return '—';
  }
  return value.toLocaleString(undefined, {
    minimumFractionDigits: 0,
    maximumFractionDigits: places,
  });
}

function formatDate(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return '—';
  }
  return date.toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' });
}

/** Formats a 1e18 decimal string as a percentage. */
function formatPercent(raw: string, places = 2): string {
  const value = Number(raw);
  if (!Number.isFinite(value)) {
    return '—';
  }
  return `${(value * 100).toFixed(places)}%`;
}

export function OptionsTrading() {
  const authed = isAuthenticated();
  const seriesQuery = useOptionSeries();
  const positionsQuery = useOptionPositions();

  const buy = useBuyOption();
  const exercise = useExerciseOption();
  const addCollateral = useAddOptionCollateral();
  const removeCollateral = useRemoveOptionCollateral();
  const withdrawLiquidity = useWithdrawOptionLiquidity();

  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [size, setSize] = useState('1');
  const [maxPremium, setMaxPremium] = useState('');
  const [collateralAmount, setCollateralAmount] = useState('');
  const [message, setMessage] = useState<{ kind: 'ok' | 'error'; text: string } | null>(null);

  const series = seriesQuery.data ?? [];
  const selected = useMemo(
    () => series.find((view) => view.series.id === selectedId) ?? series[0] ?? null,
    [series, selectedId],
  );

  /** The premium for the entered size, from the series' unit premium. */
  const estimatedPremium = useMemo(() => {
    if (!selected) {
      return '—';
    }
    const unit = Number(selected.premium_per_unit);
    const qty = Number(size);
    if (!Number.isFinite(unit) || !Number.isFinite(qty)) {
      return '—';
    }
    return formatWad((unit * qty).toFixed(18), 4);
  }, [selected, size]);

  if (seriesQuery.isPending) {
    return (
      <div className="glass-dark rounded-2xl p-10 text-center">
        <Loader2 className="w-8 h-8 mx-auto mb-3 animate-spin text-tiger-orange" />
        <p className="text-gray-400">Loading option series…</p>
      </div>
    );
  }

  if (seriesQuery.error) {
    return (
      <div className="glass-dark rounded-2xl p-6">
        <div className="flex items-center gap-3 text-red-400" role="alert">
          <AlertTriangle className="w-5 h-5" />
          <p>{seriesQuery.error.message}</p>
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {message && (
        <div
          role="alert"
          className={`rounded-xl p-3 text-sm ${
            message.kind === 'ok'
              ? 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/20'
              : 'bg-red-500/10 text-red-400 border border-red-500/20'
          }`}
        >
          {message.text}
        </div>
      )}

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Series list */}
        <div className="glass-dark rounded-2xl p-6 lg:col-span-2">
          <div className="flex items-center justify-between mb-4">
            <h2 className="text-lg font-display font-bold text-white">Option series</h2>
            <LineChart className="w-5 h-5 text-tiger-orange" />
          </div>
          {series.length === 0 ? (
            <p className="text-gray-400 text-sm">
              No option series are available. An operator lists them from the admin panel.
            </p>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="text-gray-500 text-xs uppercase">
                    <th className="text-left py-2">Series</th>
                    <th className="text-right py-2">Strike</th>
                    <th className="text-right py-2">Mark</th>
                    <th className="text-right py-2">IV</th>
                    <th className="text-right py-2">Premium</th>
                    <th className="text-right py-2">Delta</th>
                    <th className="text-right py-2">Expiry</th>
                  </tr>
                </thead>
                <tbody>
                  {series.map((view) => (
                    <tr
                      key={view.series.id}
                      onClick={() => setSelectedId(view.series.id)}
                      className={`cursor-pointer border-t border-white/5 hover:bg-white/5 ${
                        selected?.series.id === view.series.id ? 'bg-tiger-orange/10' : ''
                      }`}
                    >
                      <td className="py-2">
                        <div className="flex items-center gap-2">
                          {view.series.is_call ? (
                            <TrendingUp className="w-3.5 h-3.5 text-emerald-400" />
                          ) : (
                            <TrendingDown className="w-3.5 h-3.5 text-red-400" />
                          )}
                          <span className="text-white">
                            {view.series.underlying_symbol} {view.series.is_call ? 'Call' : 'Put'}
                          </span>
                          <span className="text-gray-500 text-xs">{view.series.collateral_token_symbol}</span>
                        </div>
                      </td>
                      <td className="text-right text-gray-300 font-mono">{formatWad(view.series.strike, 2)}</td>
                      <td className="text-right text-gray-300 font-mono">{formatWad(view.series.mark_price, 2)}</td>
                      <td className="text-right text-gray-300 font-mono">{formatPercent(view.implied_volatility)}</td>
                      <td className="text-right text-white font-mono">{formatWad(view.premium_per_unit, 2)}</td>
                      <td className="text-right text-gray-300 font-mono">{formatWad(view.delta, 3)}</td>
                      <td className="text-right text-gray-400 text-xs">{formatDate(view.series.expiry)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>

        {/* Trade ticket */}
        <div className="glass-dark rounded-2xl p-6">
          <h2 className="text-lg font-display font-bold text-white mb-4">Buy option</h2>
          {!selected ? (
            <p className="text-gray-400 text-sm">Select a series to trade.</p>
          ) : !authed ? (
            <p className="text-gray-400 text-sm">Sign in to buy an option.</p>
          ) : (
            <div className="space-y-4">
              <div>
                <label className="text-gray-400 text-xs block mb-1" htmlFor="option-size">
                  Size (contracts)
                </label>
                <input
                  id="option-size"
                  value={size}
                  onChange={(event) => setSize(event.target.value)}
                  inputMode="decimal"
                  className="w-full bg-white/5 border border-white/10 rounded-lg px-3 py-2 text-white font-mono focus:outline-none focus:border-tiger-orange"
                />
              </div>
              <div>
                <label className="text-gray-400 text-xs block mb-1" htmlFor="option-max-premium">
                  Max premium ({selected.series.collateral_token_symbol})
                </label>
                <input
                  id="option-max-premium"
                  value={maxPremium}
                  onChange={(event) => setMaxPremium(event.target.value)}
                  inputMode="numeric"
                  placeholder="integer in smallest unit"
                  className="w-full bg-white/5 border border-white/10 rounded-lg px-3 py-2 text-white font-mono focus:outline-none focus:border-tiger-orange"
                />
              </div>
              <div className="text-sm space-y-1">
                <div className="flex justify-between">
                  <span className="text-gray-400">Estimated premium</span>
                  <span className="text-white font-mono">{estimatedPremium}</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-gray-400">Greeks (Δ Γ ν Θ)</span>
                  <span className="text-gray-300 font-mono text-xs">
                    {formatWad(selected.delta, 2)} / {formatWad(selected.gamma, 4)} /{' '}
                    {formatWad(selected.vega, 2)} / {formatWad(selected.theta, 2)}
                  </span>
                </div>
              </div>
              <button
                onClick={() => {
                  setMessage(null);
                  buy.mutate(
                    { id: selected.series.id, size, maxPremium },
                    {
                      onSuccess: (result) =>
                        setMessage({ kind: 'ok', text: `Bought. Premium paid: ${formatWad(result.premium, 4)}` }),
                      onError: (error) => setMessage({ kind: 'error', text: error.message }),
                    },
                  );
                }}
                disabled={buy.isPending || maxPremium.trim() === ''}
                className="w-full py-2.5 rounded-lg bg-tiger-orange text-black font-semibold hover:opacity-90 disabled:opacity-40 disabled:cursor-not-allowed transition"
              >
                {buy.isPending ? 'Buying…' : 'Buy'}
              </button>
            </div>
          )}

          {/* Liquidity */}
          {selected && authed && !selected.expired && selected.series.status === 'active' && (
            <div className="mt-6 pt-4 border-t border-white/10 space-y-3">
              <p className="text-gray-400 text-xs">Provide liquidity ({selected.series.collateral_token_symbol})</p>
              <input
                value={collateralAmount}
                onChange={(event) => setCollateralAmount(event.target.value)}
                inputMode="numeric"
                placeholder="integer in smallest unit"
                className="w-full bg-white/5 border border-white/10 rounded-lg px-3 py-2 text-white font-mono text-sm focus:outline-none focus:border-tiger-orange"
              />
              <div className="flex gap-2">
                <button
                  onClick={() => {
                    setMessage(null);
                    addCollateral.mutate(
                      { id: selected.series.id, amount: collateralAmount },
                      {
                        onSuccess: () => setMessage({ kind: 'ok', text: 'Collateral deposited.' }),
                        onError: (error) => setMessage({ kind: 'error', text: error.message }),
                      },
                    );
                  }}
                  disabled={addCollateral.isPending || collateralAmount.trim() === ''}
                  className="flex-1 py-2 rounded-lg bg-white/10 text-white text-sm hover:bg-white/20 disabled:opacity-40 transition"
                >
                  Deposit
                </button>
                <button
                  onClick={() => {
                    setMessage(null);
                    removeCollateral.mutate(
                      { id: selected.series.id, amount: collateralAmount },
                      {
                        onSuccess: () => setMessage({ kind: 'ok', text: 'Collateral withdrawn.' }),
                        onError: (error) => setMessage({ kind: 'error', text: error.message }),
                      },
                    );
                  }}
                  disabled={removeCollateral.isPending || collateralAmount.trim() === ''}
                  className="flex-1 py-2 rounded-lg bg-white/10 text-white text-sm hover:bg-white/20 disabled:opacity-40 transition"
                >
                  Withdraw
                </button>
              </div>
            </div>
          )}

          {selected && authed && selected.series.status === 'settled' && (
            <div className="mt-6 pt-4 border-t border-white/10 space-y-3">
              <p className="text-gray-400 text-xs">Series settled — exercise or withdraw liquidity.</p>
              <div className="flex gap-2">
                <button
                  onClick={() => {
                    setMessage(null);
                    exercise.mutate(selected.series.id, {
                      onSuccess: (result) =>
                        setMessage({ kind: 'ok', text: `Exercised. Payout: ${formatWad(result.payout, 4)}` }),
                      onError: (error) => setMessage({ kind: 'error', text: error.message }),
                    });
                  }}
                  disabled={exercise.isPending}
                  className="flex-1 py-2 rounded-lg bg-tiger-orange text-black text-sm font-semibold hover:opacity-90 disabled:opacity-40 transition"
                >
                  Exercise
                </button>
                <button
                  onClick={() => {
                    setMessage(null);
                    withdrawLiquidity.mutate(
                      { id: selected.series.id, amount: collateralAmount },
                      {
                        onSuccess: () => setMessage({ kind: 'ok', text: 'Liquidity withdrawn.' }),
                        onError: (error) => setMessage({ kind: 'error', text: error.message }),
                      },
                    );
                  }}
                  disabled={withdrawLiquidity.isPending || collateralAmount.trim() === ''}
                  className="flex-1 py-2 rounded-lg bg-white/10 text-white text-sm hover:bg-white/20 disabled:opacity-40 transition"
                >
                  Withdraw LP
                </button>
              </div>
            </div>
          )}
        </div>
      </div>

      {/* Positions */}
      <div className="glass-dark rounded-2xl p-6">
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-lg font-display font-bold text-white">Your positions</h2>
          <Wallet className="w-5 h-5 text-tiger-orange" />
        </div>
        {!authed ? (
          <p className="text-gray-400 text-sm">Sign in to view your option positions.</p>
        ) : positionsQuery.isPending ? (
          <p className="text-gray-400 text-sm">Loading positions…</p>
        ) : positionsQuery.error ? (
          <p className="text-red-400 text-sm" role="alert">
            {positionsQuery.error.message}
          </p>
        ) : (positionsQuery.data ?? []).length === 0 ? (
          <p className="text-gray-400 text-sm">You have no open option positions.</p>
        ) : (
          <div className="space-y-3">
            {(positionsQuery.data ?? []).map((view) => (
              <div key={view.position.id} className="p-4 rounded-xl border border-white/10 bg-white/5">
                <div className="flex items-center justify-between">
                  <div>
                    <p className="text-white font-medium">
                      {view.series.underlying_symbol} {view.series.is_call ? 'Call' : 'Put'}{' '}
                      {formatWad(view.series.strike, 2)}
                    </p>
                    <p className="text-gray-500 text-xs">
                      {view.series.collateral_token_symbol} · expires {formatDate(view.series.expiry)} ·{' '}
                      {view.series.status}
                    </p>
                  </div>
                  <div className="text-right">
                    <p className="text-gray-400 text-xs">Size</p>
                    <p className="text-white font-mono">{formatWad(view.position.size, 4)}</p>
                  </div>
                </div>
                <div className="grid grid-cols-3 gap-2 mt-3 pt-2 border-t border-white/10 text-xs">
                  <div>
                    <p className="text-gray-500">Premium paid</p>
                    <p className="text-white font-mono">{formatWad(view.position.premium_paid, 2)}</p>
                  </div>
                  <div>
                    <p className="text-gray-500">Current value</p>
                    <p className="text-white font-mono">{formatWad(view.current_premium, 2)}</p>
                  </div>
                  <div>
                    <p className="text-gray-500">Payout estimate</p>
                    <p className="text-white font-mono">{formatWad(view.payout_estimate, 2)}</p>
                  </div>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
