'use client';

/**
 * Protocol analytics panel.
 *
 * Every figure on this screen is aggregated by the Go analytics service from
 * rows TigerSwap itself wrote: confirmed swaps, the derivatives book, the margin
 * book, farms, listings and the marketplace. Nothing is fetched from an external
 * indexer and nothing is estimated in the browser, so a number shown here can be
 * reconciled against the ledger. When the API is unreachable the panel says so
 * instead of drawing a chart from invented data.
 */

import { useState } from 'react';
import {
  Activity,
  AlertTriangle,
  BarChart3,
  Layers,
  LineChart,
  Loader2,
  RefreshCw,
  Trophy,
} from 'lucide-react';
import {
  Bar,
  BarChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts';

import {
  useAnalyticsOverview,
  useAnalyticsPairs,
  useAnalyticsTraders,
  useAnalyticsVolume,
} from '@/hooks/useApi';
import type { PairAnalytics, TopTrader } from '@/lib/types';

/** Formats a USD amount with a compact suffix. */
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

/** Formats a signed USD amount, keeping the sign visible for PnL. */
function formatSignedUSD(value: number): string {
  const formatted = formatUSD(Math.abs(value));
  return value < 0 ? `-${formatted}` : formatted;
}

function formatPercent(fraction: number): string {
  if (!Number.isFinite(fraction)) {
    return '—';
  }
  return `${(fraction * 100).toFixed(2)}%`;
}

/** Shortens a user id or address so a leaderboard row stays one line. */
function shorten(value: string): string {
  if (value.length <= 14) {
    return value;
  }
  return `${value.slice(0, 6)}…${value.slice(-4)}`;
}

const WINDOW_OPTIONS = [
  { label: '24h', seconds: 86_400, interval: 3_600 },
  { label: '7d', seconds: 604_800, interval: 21_600 },
  { label: '30d', seconds: 2_592_000, interval: 86_400 },
] as const;

export function Analytics() {
  const [windowIndex, setWindowIndex] = useState(0);
  const activeWindow = WINDOW_OPTIONS[windowIndex];

  const overviewQuery = useAnalyticsOverview();
  const volumeQuery = useAnalyticsVolume({
    window: activeWindow.seconds,
    interval: activeWindow.interval,
  });
  const pairsQuery = useAnalyticsPairs({ window: activeWindow.seconds, limit: 10 });
  const tradersQuery = useAnalyticsTraders({ limit: 10 });

  if (overviewQuery.isPending) {
    return (
      <div className="glass-dark rounded-2xl p-8 text-center text-gray-400">
        <Loader2 className="w-8 h-8 mx-auto mb-3 animate-spin" />
        Loading protocol analytics…
      </div>
    );
  }

  if (overviewQuery.error) {
    return (
      <div className="glass-dark rounded-2xl p-8 text-center">
        <AlertTriangle className="w-10 h-10 text-yellow-500 mx-auto mb-4" />
        <h3 className="text-white font-semibold mb-2">Analytics unavailable</h3>
        <p className="text-gray-400 text-sm" role="alert">
          {overviewQuery.error.message}
        </p>
      </div>
    );
  }

  const overview = overviewQuery.data;
  const series = volumeQuery.data?.series ?? [];
  const pairs = pairsQuery.data ?? [];
  const traders = tradersQuery.data ?? [];

  const chartData = series.map((point) => ({
    label: new Date(point.timestamp).toLocaleDateString(undefined, {
      month: 'short',
      day: 'numeric',
      hour: activeWindow.interval < 86_400 ? '2-digit' : undefined,
    }),
    volume: Number(point.volume_usd.toFixed(2)),
    fees: Number(point.fees_usd.toFixed(2)),
  }));

  const stats: Array<{ label: string; value: string; hint?: string }> = [
    { label: 'Total value locked', value: formatUSD(overview.total_tvl_usd), hint: `${overview.active_pools} active pools` },
    { label: 'Volume (24h)', value: formatUSD(overview.volume_24h_usd), hint: `${overview.total_trades_24h} trades` },
    { label: 'Fees (24h)', value: formatUSD(overview.fees_24h_usd) },
    { label: 'Volume (7d)', value: formatUSD(overview.volume_7d_usd) },
    { label: 'Open interest', value: formatUSD(overview.open_interest_usd) },
    { label: 'Margin borrowed', value: formatUSD(overview.margin_borrowed_usd) },
    { label: 'Pools', value: String(overview.total_pools), hint: `${overview.active_pools} active` },
    { label: 'Farms', value: String(overview.total_farms), hint: `${overview.active_farms} active` },
    { label: 'Tokens', value: String(overview.total_tokens) },
    { label: 'Chains', value: String(overview.total_chains), hint: `${overview.active_chains} active` },
    { label: 'Traders', value: String(overview.total_users) },
    { label: 'NFT listings', value: String(overview.nft_listings_active) },
  ];

  return (
    <div className="space-y-6">
      {/* Headline stats */}
      <div className="glass-dark rounded-2xl p-6">
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-lg font-display font-bold text-white">Protocol overview</h2>
          <BarChart3 className="w-5 h-5 text-tiger-orange" />
        </div>
        <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-4">
          {stats.map((stat) => (
            <div key={stat.label} className="rounded-xl bg-white/5 p-4">
              <p className="text-xs uppercase tracking-wide text-gray-400">{stat.label}</p>
              <p className="mt-1 text-lg font-semibold text-white">{stat.value}</p>
              {stat.hint && <p className="text-xs text-gray-500 mt-0.5">{stat.hint}</p>}
            </div>
          ))}
        </div>
      </div>

      {/* Volume series */}
      <div className="glass-dark rounded-2xl p-6">
        <div className="flex items-center justify-between mb-4">
          <div className="flex items-center gap-2">
            <LineChart className="w-5 h-5 text-tiger-orange" />
            <h2 className="text-lg font-display font-bold text-white">Volume and fees</h2>
          </div>
          <div className="flex gap-1">
            {WINDOW_OPTIONS.map((option, index) => (
              <button
                key={option.label}
                onClick={() => setWindowIndex(index)}
                className={`px-3 py-1 text-xs rounded-lg transition-colors ${
                  index === windowIndex
                    ? 'bg-tiger-orange text-white'
                    : 'bg-white/5 text-gray-400 hover:text-white'
                }`}
              >
                {option.label}
              </button>
            ))}
          </div>
        </div>
        {volumeQuery.error ? (
          <p className="text-sm text-gray-400" role="alert">
            {volumeQuery.error.message}
          </p>
        ) : chartData.length === 0 ? (
          <p className="text-sm text-gray-400">
            No rolled-up volume in this window yet. An operator can rebuild the rollup from the
            admin panel.
          </p>
        ) : (
          <div className="h-64">
            <ResponsiveContainer width="100%" height="100%">
              <BarChart data={chartData}>
                <CartesianGrid strokeDasharray="3 3" stroke="rgba(255,255,255,0.05)" />
                <XAxis dataKey="label" stroke="#6b7280" fontSize={11} />
                <YAxis stroke="#6b7280" fontSize={11} tickFormatter={(value) => formatUSD(Number(value))} />
                <Tooltip
                  contentStyle={{ background: '#12121f', border: '1px solid rgba(255,255,255,0.1)', borderRadius: 12 }}
                  labelStyle={{ color: '#9ca3af' }}
                  formatter={(value: number, name: string) => [formatUSD(value), name === 'volume' ? 'Volume' : 'Fees']}
                />
                <Bar dataKey="volume" fill="#f97316" radius={[4, 4, 0, 0]} />
                <Bar dataKey="fees" fill="#38bdf8" radius={[4, 4, 0, 0]} />
              </BarChart>
            </ResponsiveContainer>
          </div>
        )}
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Market leaderboard */}
        <div className="glass-dark rounded-2xl p-6">
          <div className="flex items-center gap-2 mb-4">
            <Layers className="w-5 h-5 text-tiger-orange" />
            <h2 className="text-lg font-display font-bold text-white">Top markets</h2>
          </div>
          {pairsQuery.error ? (
            <p className="text-sm text-gray-400" role="alert">
              {pairsQuery.error.message}
            </p>
          ) : pairs.length === 0 ? (
            <p className="text-sm text-gray-400">No market volume recorded in this window.</p>
          ) : (
            <MarketTable pairs={pairs} />
          )}
        </div>

        {/* Trader leaderboard */}
        <div className="glass-dark rounded-2xl p-6">
          <div className="flex items-center gap-2 mb-4">
            <Trophy className="w-5 h-5 text-tiger-orange" />
            <h2 className="text-lg font-display font-bold text-white">Top traders</h2>
          </div>
          {tradersQuery.error ? (
            <p className="text-sm text-gray-400" role="alert">
              {tradersQuery.error.message}
            </p>
          ) : traders.length === 0 ? (
            <p className="text-sm text-gray-400">No closed positions to rank yet.</p>
          ) : (
            <TraderTable traders={traders} />
          )}
        </div>
      </div>

      <p className="flex items-center gap-2 text-xs text-gray-500">
        <Activity className="w-3.5 h-3.5" />
        Updated {overview.updated_at ? new Date(overview.updated_at).toLocaleString() : '—'} · aggregated
        from TigerSwap&apos;s own ledger, derivatives and margin books
        <RefreshCw className="w-3.5 h-3.5" />
      </p>
    </div>
  );
}

function MarketTable({ pairs }: { pairs: PairAnalytics[] }) {
  return (
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
  );
}

function TraderTable({ traders }: { traders: TopTrader[] }) {
  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm">
        <thead>
          <tr className="text-left text-xs uppercase tracking-wide text-gray-400">
            <th className="py-2">Trader</th>
            <th className="py-2 text-right">Realized PnL</th>
            <th className="py-2 text-right">Win rate</th>
            <th className="py-2 text-right">Trades</th>
          </tr>
        </thead>
        <tbody>
          {traders.map((trader) => (
            <tr key={trader.user_id} className="border-t border-white/5">
              <td className="py-2 font-mono text-white" title={trader.user_id}>
                {shorten(trader.user_id)}
              </td>
              <td
                className={`py-2 text-right ${trader.realized_pnl >= 0 ? 'text-green-400' : 'text-red-400'}`}
              >
                {formatSignedUSD(trader.realized_pnl)}
              </td>
              <td className="py-2 text-right text-gray-300">{formatPercent(trader.win_rate)}</td>
              <td className="py-2 text-right text-gray-300">{trader.closed_trades}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
