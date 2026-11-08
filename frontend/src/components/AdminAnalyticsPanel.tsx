'use client';

/**
 * Admin panel for protocol analytics.
 *
 * The one operation here is rebuilding the volume rollup. The rollup folds
 * confirmed swaps into fixed-width buckets and is idempotent, so an operator can
 * safely re-run a window after a late confirmation or a reorg. The dashboard
 * itself is read-only and comes from the public analytics endpoints.
 */

import { useCallback, useEffect, useState } from 'react';
import { AlertTriangle, BarChart3, CheckCircle2, Loader2, RefreshCw } from 'lucide-react';

import { endpoints } from '@/lib/endpoints';
import type { PairAnalytics, ProtocolOverview, TopTrader } from '@/lib/types';
import { ValidationError } from '@/lib/validation';

function errorMessage(error: unknown): string {
  if (error instanceof ValidationError) {
    return error.message;
  }
  if (error instanceof Error) {
    return error.message;
  }
  return 'Unexpected error';
}

const WINDOWS = [
  { label: '1h', seconds: 3_600 },
  { label: '24h', seconds: 86_400 },
  { label: '7d', seconds: 604_800 },
] as const;

function formatUSD(value: number): string {
  if (!Number.isFinite(value)) {
    return '—';
  }
  const abs = Math.abs(value);
  if (abs >= 1_000_000_000) return `$${(value / 1_000_000_000).toFixed(2)}B`;
  if (abs >= 1_000_000) return `$${(value / 1_000_000).toFixed(2)}M`;
  if (abs >= 1_000) return `$${(value / 1_000).toFixed(2)}K`;
  return `$${value.toFixed(2)}`;
}

export function AdminAnalyticsPanel() {
  const [overview, setOverview] = useState<ProtocolOverview | null>(null);
  const [pairs, setPairs] = useState<PairAnalytics[]>([]);
  const [traders, setTraders] = useState<TopTrader[]>([]);
  const [loading, setLoading] = useState(false);
  const [rollupWindow, setRollupWindow] = useState<number>(3_600);
  const [rollingUp, setRollingUp] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [ok, setOk] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [nextOverview, nextPairs, nextTraders] = await Promise.all([
        endpoints.getAnalyticsOverview(),
        endpoints.getAnalyticsPairs({ window: 86_400, limit: 20 }),
        endpoints.getAnalyticsTraders({ limit: 20 }),
      ]);
      setOverview(nextOverview);
      setPairs(nextPairs);
      setTraders(nextTraders);
    } catch (caught) {
      setError(errorMessage(caught));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const handleRollup = async () => {
    setRollingUp(true);
    setError(null);
    setOk(null);
    try {
      const result = await endpoints.runAnalyticsRollup(rollupWindow);
      setOk(`Rollup rebuilt ${result.buckets_written} bucket(s).`);
      await load();
    } catch (caught) {
      setError(errorMessage(caught));
    } finally {
      setRollingUp(false);
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div className="flex items-center space-x-2">
          <BarChart3 className="w-5 h-5 text-tiger-orange" />
          <h2 className="text-lg font-display font-bold text-white">Protocol analytics</h2>
        </div>
        <button
          onClick={() => void load()}
          disabled={loading}
          className="flex items-center space-x-2 px-4 py-2 bg-white/10 text-white rounded-lg hover:bg-white/20 transition-colors disabled:opacity-50"
        >
          <RefreshCw className={`w-4 h-4 ${loading ? 'animate-spin' : ''}`} />
          <span>Refresh</span>
        </button>
      </div>

      {error && (
        <p className="flex items-center gap-2 text-red-400 text-sm" role="alert">
          <AlertTriangle className="w-4 h-4" />
          {error}
        </p>
      )}
      {ok && (
        <p className="flex items-center gap-2 text-green-400 text-sm" role="status">
          <CheckCircle2 className="w-4 h-4" />
          {ok}
        </p>
      )}

      {/* Rollup control */}
      <div className="glass-dark rounded-2xl p-6">
        <h3 className="text-white font-semibold mb-2">Rebuild volume rollup</h3>
        <p className="text-sm text-gray-400 mb-4">
          The rollup rebuilds each bucket from the confirmed swaps in its window, so re-running it is
          safe: a window is replaced, never appended to. Use it after a late confirmation or a reorg.
        </p>
        <div className="flex flex-wrap items-center gap-3">
          {WINDOWS.map((option) => (
            <button
              key={option.label}
              onClick={() => setRollupWindow(option.seconds)}
              className={`px-3 py-1 text-xs rounded-lg transition-colors ${
                rollupWindow === option.seconds
                  ? 'bg-tiger-orange text-white'
                  : 'bg-white/5 text-gray-400 hover:text-white'
              }`}
            >
              Last {option.label}
            </button>
          ))}
          <button
            onClick={() => void handleRollup()}
            disabled={rollingUp}
            className="flex items-center space-x-2 px-4 py-2 bg-tiger-orange text-white rounded-lg hover:bg-tiger-orange/90 transition-colors disabled:opacity-50"
          >
            {rollingUp ? <Loader2 className="w-4 h-4 animate-spin" /> : <RefreshCw className="w-4 h-4" />}
            <span>Run rollup</span>
          </button>
        </div>
      </div>

      {/* Overview snapshot */}
      <div className="glass-dark rounded-2xl p-6">
        <h3 className="text-white font-semibold mb-4">Dashboard snapshot</h3>
        {loading && !overview ? (
          <div className="text-center text-gray-400 py-6">
            <Loader2 className="w-6 h-6 mx-auto mb-2 animate-spin" />
            Loading…
          </div>
        ) : !overview ? (
          <p className="text-sm text-gray-400">No snapshot available.</p>
        ) : (
          <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
            {[
              { label: 'TVL', value: formatUSD(overview.total_tvl_usd) },
              { label: 'Volume 24h', value: formatUSD(overview.volume_24h_usd) },
              { label: 'Fees 24h', value: formatUSD(overview.fees_24h_usd) },
              { label: 'Volume 7d', value: formatUSD(overview.volume_7d_usd) },
              { label: 'Fees 7d', value: formatUSD(overview.fees_7d_usd) },
              { label: 'Open interest', value: formatUSD(overview.open_interest_usd) },
              { label: 'Margin borrowed', value: formatUSD(overview.margin_borrowed_usd) },
              { label: 'Trades 24h', value: String(overview.total_trades_24h) },
              { label: 'Pools', value: `${overview.active_pools}/${overview.total_pools}` },
              { label: 'Farms', value: `${overview.active_farms}/${overview.total_farms}` },
              { label: 'Chains', value: `${overview.active_chains}/${overview.total_chains}` },
              { label: 'NFT listings', value: String(overview.nft_listings_active) },
            ].map((stat) => (
              <div key={stat.label} className="rounded-xl bg-white/5 p-4">
                <p className="text-xs uppercase tracking-wide text-gray-400">{stat.label}</p>
                <p className="mt-1 text-lg font-semibold text-white">{stat.value}</p>
              </div>
            ))}
          </div>
        )}
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Per-market volume */}
        <div className="glass-dark rounded-2xl p-6">
          <h3 className="text-white font-semibold mb-4">Markets by volume (24h)</h3>
          {pairs.length === 0 ? (
            <p className="text-sm text-gray-400">No market volume recorded.</p>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="text-left text-xs uppercase tracking-wide text-gray-400">
                    <th className="py-2">Market</th>
                    <th className="py-2 text-right">Volume</th>
                    <th className="py-2 text-right">Fees</th>
                    <th className="py-2 text-right">Trades</th>
                  </tr>
                </thead>
                <tbody>
                  {pairs.map((pair) => (
                    <tr key={`${pair.pair}-${pair.chain_id}`} className="border-t border-white/5">
                      <td className="py-2 text-white">{pair.pair}</td>
                      <td className="py-2 text-right text-gray-300">{formatUSD(pair.volume_usd)}</td>
                      <td className="py-2 text-right text-gray-300">{formatUSD(pair.fees_usd)}</td>
                      <td className="py-2 text-right text-gray-300">{pair.trade_count}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>

        {/* Trader leaderboard */}
        <div className="glass-dark rounded-2xl p-6">
          <h3 className="text-white font-semibold mb-4">Top traders by realized PnL</h3>
          {traders.length === 0 ? (
            <p className="text-sm text-gray-400">No closed positions to rank.</p>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="text-left text-xs uppercase tracking-wide text-gray-400">
                    <th className="py-2">Trader</th>
                    <th className="py-2 text-right">Realized PnL</th>
                    <th className="py-2 text-right">Win rate</th>
                    <th className="py-2 text-right">Liquidations</th>
                  </tr>
                </thead>
                <tbody>
                  {traders.map((trader) => (
                    <tr key={trader.user_id} className="border-t border-white/5">
                      <td className="py-2 font-mono text-white" title={trader.user_id}>
                        {trader.user_id.length > 14
                          ? `${trader.user_id.slice(0, 6)}…${trader.user_id.slice(-4)}`
                          : trader.user_id}
                      </td>
                      <td
                        className={`py-2 text-right ${trader.realized_pnl >= 0 ? 'text-green-400' : 'text-red-400'}`}
                      >
                        {trader.realized_pnl < 0 ? '-' : ''}
                        {formatUSD(Math.abs(trader.realized_pnl))}
                      </td>
                      <td className="py-2 text-right text-gray-300">{(trader.win_rate * 100).toFixed(2)}%</td>
                      <td className="py-2 text-right text-gray-300">{trader.liquidations}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
