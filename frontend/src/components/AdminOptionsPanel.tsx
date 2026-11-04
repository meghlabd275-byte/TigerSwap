'use client';

/**
 * Admin panel for option series.
 *
 * An operator lists a series, sets its implied volatility and expiry, pulls the
 * mark price from the oracle, and settles an expired series. Every action goes
 * through a real admin endpoint; a failed request is shown rather than being
 * swallowed.
 */

import { useState } from 'react';
import { AlertTriangle, Loader2, Plus, RefreshCw, Trash2 } from 'lucide-react';

import {
  useAllOptionSeries,
  useCreateOptionSeries,
  useDeleteOptionSeries,
  useRefreshOptionMarkPrice,
  useSettleOptionSeries,
  useUpdateOptionSeries,
} from '@/hooks/useApi';

function formatWad(raw: string, places = 4): string {
  const value = Number(raw);
  if (!Number.isFinite(value)) {
    return '—';
  }
  return value.toLocaleString(undefined, { minimumFractionDigits: 0, maximumFractionDigits: places });
}

function formatDate(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return '—';
  }
  return date.toLocaleString(undefined, { year: 'numeric', month: 'short', day: 'numeric' });
}

export function AdminOptionsPanel() {
  const seriesQuery = useAllOptionSeries();
  const createSeries = useCreateOptionSeries();
  const updateSeries = useUpdateOptionSeries();
  const deleteSeries = useDeleteOptionSeries();
  const settleSeries = useSettleOptionSeries();
  const refreshPrice = useRefreshOptionMarkPrice();

  const [error, setError] = useState<string | null>(null);
  const [ok, setOk] = useState<string | null>(null);
  const [settleId, setSettleId] = useState<string | null>(null);
  const [settlePrice, setSettlePrice] = useState('');

  const [form, setForm] = useState({
    key: '',
    name: '',
    chain_id: '1',
    underlying_symbol: '',
    collateral_token_address: '',
    collateral_token_symbol: 'RUSD',
    collateral_token_decimals: '18',
    strike: '',
    base_iv: '0.8',
    expiry: '',
    is_call: true,
    mark_price: '',
    status: 'paused',
  });

  const series = seriesQuery.data ?? [];

  return (
    <div className="space-y-6">
      {error && (
        <div className="rounded-xl p-3 text-sm bg-red-500/10 text-red-400 border border-red-500/20" role="alert">
          {error}
        </div>
      )}
      {ok && (
        <div className="rounded-xl p-3 text-sm bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">
          {ok}
        </div>
      )}

      {/* Create form */}
      <div className="glass-dark rounded-2xl p-6">
        <h3 className="text-lg font-bold text-white mb-4 flex items-center gap-2">
          <Plus className="w-5 h-5 text-tiger-orange" /> List an option series
        </h3>
        <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
          <Field label="Key" value={form.key} onChange={(v) => setForm({ ...form, key: v })} />
          <Field label="Name" value={form.name} onChange={(v) => setForm({ ...form, name: v })} />
          <Field label="Chain ID" value={form.chain_id} onChange={(v) => setForm({ ...form, chain_id: v })} />
          <Field
            label="Underlying symbol"
            value={form.underlying_symbol}
            onChange={(v) => setForm({ ...form, underlying_symbol: v.toUpperCase() })}
          />
          <Field
            label="Collateral address"
            value={form.collateral_token_address}
            onChange={(v) => setForm({ ...form, collateral_token_address: v })}
          />
          <div>
            <label className="text-gray-400 text-xs block mb-1">Collateral symbol</label>
            <select
              value={form.collateral_token_symbol}
              onChange={(e) => setForm({ ...form, collateral_token_symbol: e.target.value })}
              className="w-full bg-white/5 border border-white/10 rounded-lg px-3 py-2 text-white text-sm focus:outline-none focus:border-tiger-orange"
            >
              {['RUSD', 'USDT', 'USDC', 'RYUSD'].map((symbol) => (
                <option key={symbol} value={symbol} className="bg-tiger-dark">
                  {symbol}
                </option>
              ))}
            </select>
          </div>
          <Field
            label="Collateral decimals"
            value={form.collateral_token_decimals}
            onChange={(v) => setForm({ ...form, collateral_token_decimals: v })}
          />
          <Field label="Strike" value={form.strike} onChange={(v) => setForm({ ...form, strike: v })} />
          <Field label="Base IV (e.g. 0.8)" value={form.base_iv} onChange={(v) => setForm({ ...form, base_iv: v })} />
          <Field label="Mark price" value={form.mark_price} onChange={(v) => setForm({ ...form, mark_price: v })} />
          <div>
            <label className="text-gray-400 text-xs block mb-1">Expiry (RFC3339)</label>
            <input
              value={form.expiry}
              onChange={(e) => setForm({ ...form, expiry: e.target.value })}
              placeholder="2026-12-31T08:00:00Z"
              className="w-full bg-white/5 border border-white/10 rounded-lg px-3 py-2 text-white text-sm focus:outline-none focus:border-tiger-orange"
            />
          </div>
          <div>
            <label className="text-gray-400 text-xs block mb-1">Type</label>
            <select
              value={form.is_call ? 'call' : 'put'}
              onChange={(e) => setForm({ ...form, is_call: e.target.value === 'call' })}
              className="w-full bg-white/5 border border-white/10 rounded-lg px-3 py-2 text-white text-sm focus:outline-none focus:border-tiger-orange"
            >
              <option value="call" className="bg-tiger-dark">Call</option>
              <option value="put" className="bg-tiger-dark">Put</option>
            </select>
          </div>
          <div>
            <label className="text-gray-400 text-xs block mb-1">Status</label>
            <select
              value={form.status}
              onChange={(e) => setForm({ ...form, status: e.target.value })}
              className="w-full bg-white/5 border border-white/10 rounded-lg px-3 py-2 text-white text-sm focus:outline-none focus:border-tiger-orange"
            >
              {['paused', 'active'].map((status) => (
                <option key={status} value={status} className="bg-tiger-dark">
                  {status}
                </option>
              ))}
            </select>
          </div>
        </div>
        <button
          onClick={() => {
            setError(null);
            setOk(null);
            createSeries.mutate(
              {
                key: form.key,
                name: form.name,
                chain_id: Number(form.chain_id),
                underlying_symbol: form.underlying_symbol,
                collateral_token_address: form.collateral_token_address,
                collateral_token_symbol: form.collateral_token_symbol,
                collateral_token_decimals: Number(form.collateral_token_decimals),
                strike: form.strike,
                base_iv: form.base_iv,
                expiry: form.expiry,
                is_call: form.is_call,
                mark_price: form.mark_price,
                status: form.status,
              },
              {
                onSuccess: () => setOk('Series created.'),
                onError: (e) => setError(e.message),
              },
            );
          }}
          disabled={createSeries.isPending}
          className="mt-4 px-4 py-2 rounded-lg bg-tiger-orange text-black font-semibold hover:opacity-90 disabled:opacity-40 transition"
        >
          {createSeries.isPending ? 'Creating…' : 'Create series'}
        </button>
      </div>

      {/* Series table */}
      <div className="glass-dark rounded-2xl p-6">
        <h3 className="text-lg font-bold text-white mb-4">Option series</h3>
        {seriesQuery.isPending ? (
          <div className="text-center py-6">
            <Loader2 className="w-6 h-6 mx-auto animate-spin text-tiger-orange" />
          </div>
        ) : seriesQuery.error ? (
          <div className="flex items-center gap-2 text-red-400 text-sm" role="alert">
            <AlertTriangle className="w-4 h-4" /> {seriesQuery.error.message}
          </div>
        ) : series.length === 0 ? (
          <p className="text-gray-400 text-sm">No option series yet.</p>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="text-gray-500 text-xs uppercase">
                  <th className="text-left py-2">Series</th>
                  <th className="text-right py-2">Strike</th>
                  <th className="text-right py-2">Mark</th>
                  <th className="text-right py-2">Collateral</th>
                  <th className="text-right py-2">Locked</th>
                  <th className="text-right py-2">Expiry</th>
                  <th className="text-center py-2">Status</th>
                  <th className="text-right py-2">Actions</th>
                </tr>
              </thead>
              <tbody>
                {series.map((view) => (
                  <tr key={view.series.id} className="border-t border-white/5">
                    <td className="py-2 text-white">
                      {view.series.underlying_symbol} {view.series.is_call ? 'Call' : 'Put'}
                      <span className="text-gray-500 text-xs ml-2">{view.series.key}</span>
                    </td>
                    <td className="text-right text-gray-300 font-mono">{formatWad(view.series.strike, 2)}</td>
                    <td className="text-right text-gray-300 font-mono">{formatWad(view.series.mark_price, 2)}</td>
                    <td className="text-right text-gray-300 font-mono">{formatWad(view.series.collateral, 2)}</td>
                    <td className="text-right text-gray-300 font-mono">{formatWad(view.series.locked_collateral, 2)}</td>
                    <td className="text-right text-gray-400 text-xs">{formatDate(view.series.expiry)}</td>
                    <td className="text-center">
                      <span className="text-xs px-2 py-0.5 rounded bg-white/10 text-gray-300">
                        {view.series.status}
                      </span>
                    </td>
                    <td className="text-right">
                      <div className="flex items-center justify-end gap-2">
                        <button
                          title="Refresh mark price from oracle"
                          onClick={() => {
                            setError(null);
                            setOk(null);
                            refreshPrice.mutate(view.series.id, {
                              onSuccess: () => setOk('Mark price refreshed.'),
                              onError: (e) => setError(e.message),
                            });
                          }}
                          className="p-1.5 rounded hover:bg-white/10 text-gray-400"
                        >
                          <RefreshCw className="w-4 h-4" />
                        </button>
                        <button
                          title={view.series.status === 'active' ? 'Pause' : 'Activate'}
                          onClick={() => {
                            setError(null);
                            setOk(null);
                            updateSeries.mutate(
                              {
                                id: view.series.id,
                                payload: { status: view.series.status === 'active' ? 'paused' : 'active' },
                              },
                              {
                                onSuccess: () => setOk('Status updated.'),
                                onError: (e) => setError(e.message),
                              },
                            );
                          }}
                          className="p-1.5 rounded hover:bg-white/10 text-gray-400"
                        >
                          {view.series.status === 'active' ? 'Pause' : 'Activate'}
                        </button>
                        {!view.expired && view.series.status !== 'settled' && (
                          <button
                            title="Set expiry to now to allow settlement"
                            onClick={() => {
                              setError(null);
                              setOk(null);
                              const now = new Date(Date.now() + 1000).toISOString();
                              updateSeries.mutate(
                                { id: view.series.id, payload: { expiry: now } },
                                { onError: (e) => setError(e.message) },
                              );
                            }}
                            className="p-1.5 rounded hover:bg-white/10 text-gray-400 text-xs"
                          >
                            Expire
                          </button>
                        )}
                        {view.series.status !== 'settled' && (
                          <button
                            title="Settle series"
                            onClick={() => {
                              setSettleId(view.series.id);
                              setSettlePrice(view.series.mark_price);
                            }}
                            className="px-2 py-1 rounded bg-white/10 text-xs text-white hover:bg-white/20"
                          >
                            Settle
                          </button>
                        )}
                        <button
                          title="Close series"
                          onClick={() => {
                            setError(null);
                            setOk(null);
                            deleteSeries.mutate(view.series.id, {
                              onSuccess: () => setOk('Series closed.'),
                              onError: (e) => setError(e.message),
                            });
                          }}
                          className="p-1.5 rounded hover:bg-red-500/20 text-red-400"
                        >
                          <Trash2 className="w-4 h-4" />
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* Settle dialog */}
      {settleId && (
        <div className="glass-dark rounded-2xl p-6">
          <h3 className="text-lg font-bold text-white mb-4">Settle series</h3>
          <div className="flex items-end gap-3">
            <div className="flex-1">
              <label className="text-gray-400 text-xs block mb-1">Settlement price</label>
              <input
                value={settlePrice}
                onChange={(e) => setSettlePrice(e.target.value)}
                className="w-full bg-white/5 border border-white/10 rounded-lg px-3 py-2 text-white font-mono focus:outline-none focus:border-tiger-orange"
              />
            </div>
            <button
              onClick={() => {
                setError(null);
                setOk(null);
                settleSeries.mutate(
                  { id: settleId, settlementPrice: settlePrice },
                  {
                    onSuccess: () => {
                      setOk('Series settled.');
                      setSettleId(null);
                    },
                    onError: (e) => setError(e.message),
                  },
                );
              }}
              disabled={settleSeries.isPending || settlePrice.trim() === ''}
              className="px-4 py-2 rounded-lg bg-tiger-orange text-black font-semibold hover:opacity-90 disabled:opacity-40 transition"
            >
              {settleSeries.isPending ? 'Settling…' : 'Settle'}
            </button>
            <button
              onClick={() => setSettleId(null)}
              className="px-4 py-2 rounded-lg bg-white/10 text-white hover:bg-white/20 transition"
            >
              Cancel
            </button>
          </div>
        </div>
      )}
    </div>
  );
}

function Field({
  label,
  value,
  onChange,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
}) {
  return (
    <div>
      <label className="text-gray-400 text-xs block mb-1">{label}</label>
      <input
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="w-full bg-white/5 border border-white/10 rounded-lg px-3 py-2 text-white text-sm focus:outline-none focus:border-tiger-orange"
      />
    </div>
  );
}
