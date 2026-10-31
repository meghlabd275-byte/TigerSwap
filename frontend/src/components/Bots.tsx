'use client';

/**
 * Trading bots panel.
 *
 * A bot is a persisted strategy configuration. The engine that runs it lives in
 * the Rust `tigerswap-bots` crate; this panel only creates, lists and controls
 * the configuration, and shows the orders and realised performance the engine
 * recorded. Nothing here computes a price or a PnL in the browser.
 */

import { useState } from 'react';
import { Bot as BotIcon, Loader2, Pause, Play, Trash2 } from 'lucide-react';

import {
  useBotOrders,
  useBotPerformance,
  useBotStrategies,
  useBots,
  useCreateBot,
  useDeleteBot,
  useSetBotStatus,
} from '@/hooks/useApi';
import { isAuthenticated } from '@/lib/auth';

const STATUS_COLOURS: Record<string, string> = {
  active: 'text-green-400',
  paused: 'text-yellow-400',
  stopped: 'text-gray-400',
};

export function Bots() {
  const [name, setName] = useState('');
  const [strategy, setStrategy] = useState('');
  const [symbol, setSymbol] = useState('BTC/RUSD');
  const [configText, setConfigText] = useState('{}');
  const [maxOrderSize, setMaxOrderSize] = useState('');
  const [maxDailyLoss, setMaxDailyLoss] = useState('0');
  const [maxOpenOrders, setMaxOpenOrders] = useState('10');
  const [selectedBotId, setSelectedBotId] = useState<string | null>(null);
  const [formError, setFormError] = useState<string | null>(null);

  const authed = isAuthenticated();
  const strategiesQuery = useBotStrategies();
  const botsQuery = useBots();
  const ordersQuery = useBotOrders(selectedBotId);
  const performanceQuery = useBotPerformance(selectedBotId);
  const createBot = useCreateBot();
  const setStatus = useSetBotStatus();
  const deleteBot = useDeleteBot();

  const strategies = strategiesQuery.data ?? [];
  const bots = botsQuery.data ?? [];
  const activeStrategy = strategy || strategies[0]?.strategy || '';

  const handleCreate = () => {
    setFormError(null);
    if (!authed) {
      setFormError('Sign in to configure a bot.');
      return;
    }
    if (!name.trim()) {
      setFormError('A bot name is required.');
      return;
    }
    if (!activeStrategy) {
      setFormError('No strategy is available.');
      return;
    }
    let parsedConfig: Record<string, unknown>;
    try {
      const parsed = JSON.parse(configText) as unknown;
      if (parsed === null || typeof parsed !== 'object' || Array.isArray(parsed)) {
        throw new Error('configuration must be a JSON object');
      }
      parsedConfig = parsed as Record<string, unknown>;
    } catch (error) {
      setFormError(`Configuration is not valid JSON: ${(error as Error).message}`);
      return;
    }
    createBot.mutate(
      {
        name: name.trim(),
        strategy: activeStrategy,
        symbol: symbol.trim(),
        config: parsedConfig,
        max_order_size: maxOrderSize.trim(),
        max_daily_loss: maxDailyLoss.trim() || '0',
        max_open_orders: Number(maxOpenOrders),
      },
      {
        onSuccess: (bot) => {
          setName('');
          setMaxOrderSize('');
          setSelectedBotId(bot.id);
        },
        onError: (error) => setFormError(error.message),
      },
    );
  };

  return (
    <div className="glass-dark rounded-2xl p-6">
      <div className="flex items-center justify-between mb-4">
        <h2 className="text-lg font-display font-bold text-white">Trading bots</h2>
        <BotIcon className="w-5 h-5 text-tiger-orange" />
      </div>

      {/* Create */}
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 mb-4">
        <div>
          <label htmlFor="bot-name" className="text-gray-400 text-sm mb-1 block">
            Name
          </label>
          <input
            id="bot-name"
            type="text"
            value={name}
            onChange={(event) => setName(event.target.value)}
            className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-2 text-white focus:outline-none focus:border-tiger-orange"
          />
        </div>
        <div>
          <label htmlFor="bot-strategy" className="text-gray-400 text-sm mb-1 block">
            Strategy
          </label>
          <select
            id="bot-strategy"
            value={activeStrategy}
            onChange={(event) => setStrategy(event.target.value)}
            disabled={strategies.length === 0}
            className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-2 text-white focus:outline-none focus:border-tiger-orange disabled:text-gray-500"
          >
            {strategies.length === 0 ? (
              <option value="" className="bg-gray-900">
                {strategiesQuery.isPending ? 'Loading strategies…' : 'No strategies available'}
              </option>
            ) : (
              strategies.map((entry) => (
                <option key={entry.strategy} value={entry.strategy} className="bg-gray-900">
                  {entry.strategy}
                </option>
              ))
            )}
          </select>
        </div>
        <div>
          <label htmlFor="bot-symbol" className="text-gray-400 text-sm mb-1 block">
            Market
          </label>
          <input
            id="bot-symbol"
            type="text"
            value={symbol}
            onChange={(event) => setSymbol(event.target.value)}
            className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-2 text-white focus:outline-none focus:border-tiger-orange"
          />
        </div>
        <div>
          <label htmlFor="bot-max-order" className="text-gray-400 text-sm mb-1 block">
            Max order size
          </label>
          <input
            id="bot-max-order"
            type="text"
            inputMode="decimal"
            value={maxOrderSize}
            onChange={(event) => setMaxOrderSize(event.target.value)}
            className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-2 text-white focus:outline-none focus:border-tiger-orange"
          />
        </div>
        <div>
          <label htmlFor="bot-max-loss" className="text-gray-400 text-sm mb-1 block">
            Max daily loss
          </label>
          <input
            id="bot-max-loss"
            type="text"
            inputMode="decimal"
            value={maxDailyLoss}
            onChange={(event) => setMaxDailyLoss(event.target.value)}
            className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-2 text-white focus:outline-none focus:border-tiger-orange"
          />
        </div>
        <div>
          <label htmlFor="bot-max-orders" className="text-gray-400 text-sm mb-1 block">
            Max open orders
          </label>
          <input
            id="bot-max-orders"
            type="number"
            min={1}
            max={1000}
            value={maxOpenOrders}
            onChange={(event) => setMaxOpenOrders(event.target.value)}
            className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-2 text-white focus:outline-none focus:border-tiger-orange"
          />
        </div>
        <div className="sm:col-span-2">
          <label htmlFor="bot-config" className="text-gray-400 text-sm mb-1 block">
            Strategy configuration (JSON)
          </label>
          <textarea
            id="bot-config"
            rows={3}
            value={configText}
            onChange={(event) => setConfigText(event.target.value)}
            spellCheck={false}
            className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-2 text-white font-mono text-sm focus:outline-none focus:border-tiger-orange"
          />
          {strategies.find((entry) => entry.strategy === activeStrategy) && (
            <p className="text-gray-500 text-xs mt-1">
              Accepted keys:{' '}
              {Object.entries(
                strategies.find((entry) => entry.strategy === activeStrategy)!.config_keys,
              )
                .map(([key, kind]) => `${key}:${kind}`)
                .join(', ') || 'none'}
            </p>
          )}
        </div>
      </div>

      <button
        onClick={handleCreate}
        disabled={createBot.isPending}
        className="w-full mb-4 px-4 py-3 rounded-xl font-medium bg-tiger-orange text-white disabled:bg-gray-700 disabled:text-gray-500"
      >
        {createBot.isPending ? <Loader2 className="w-4 h-4 animate-spin mx-auto" /> : 'Create bot'}
      </button>

      {formError && (
        <p className="text-red-400 text-xs mb-3" role="alert">
          {formError}
        </p>
      )}
      {botsQuery.error && (
        <p className="text-red-400 text-xs mb-3" role="alert">
          {botsQuery.error.message}
        </p>
      )}

      {/* List */}
      {bots.length === 0 ? (
        <p className="text-gray-500 text-sm">
          {botsQuery.isPending ? 'Loading bots…' : 'No bots configured yet.'}
        </p>
      ) : (
        <ul className="space-y-2">
          {bots.map((bot) => (
            <li
              key={bot.id}
              className="bg-white/5 rounded-xl p-3 flex items-center justify-between"
            >
              <button
                onClick={() => setSelectedBotId(bot.id)}
                className="text-left flex-1"
              >
                <span className="text-white font-medium">{bot.name}</span>
                <span className="text-gray-500 text-xs ml-2">
                  {bot.strategy} · {bot.symbol}
                </span>
                <span className={`text-xs ml-2 ${STATUS_COLOURS[bot.status] ?? 'text-gray-400'}`}>
                  {bot.status}
                </span>
                <span className="block text-gray-400 text-xs mt-1">
                  PnL {bot.realised_pnl} · {bot.filled_orders} filled · volume {bot.traded_volume}
                </span>
              </button>
              <div className="flex items-center space-x-1">
                {bot.status === 'active' ? (
                  <button
                    aria-label="Pause bot"
                    onClick={() => setStatus.mutate({ id: bot.id, status: 'paused' })}
                    className="p-2 text-yellow-400 hover:bg-white/10 rounded-lg"
                  >
                    <Pause className="w-4 h-4" />
                  </button>
                ) : bot.status === 'paused' ? (
                  <button
                    aria-label="Resume bot"
                    onClick={() => setStatus.mutate({ id: bot.id, status: 'active' })}
                    className="p-2 text-green-400 hover:bg-white/10 rounded-lg"
                  >
                    <Play className="w-4 h-4" />
                  </button>
                ) : null}
                {bot.status !== 'active' && (
                  <button
                    aria-label="Delete bot"
                    onClick={() => deleteBot.mutate(bot.id)}
                    className="p-2 text-red-400 hover:bg-white/10 rounded-lg"
                  >
                    <Trash2 className="w-4 h-4" />
                  </button>
                )}
              </div>
            </li>
          ))}
        </ul>
      )}

      {/* Detail */}
      {selectedBotId && (
        <div className="mt-4 border-t border-white/10 pt-4">
          <h3 className="text-sm font-semibold text-white mb-2">Recent orders</h3>
          {ordersQuery.isPending ? (
            <p className="text-gray-400 text-sm flex items-center space-x-2">
              <Loader2 className="w-4 h-4 animate-spin" />
              <span>Loading orders…</span>
            </p>
          ) : ordersQuery.error ? (
            <p className="text-red-400 text-xs" role="alert">
              {ordersQuery.error.message}
            </p>
          ) : (ordersQuery.data ?? []).length === 0 ? (
            <p className="text-gray-500 text-xs">No orders recorded.</p>
          ) : (
            <ul className="space-y-1 mb-4">
              {(ordersQuery.data ?? []).map((order) => (
                <li key={order.id} className="text-xs text-gray-300">
                  {order.symbol} {order.side} {order.quantity} @ {order.price} · {order.status}
                </li>
              ))}
            </ul>
          )}

          <h3 className="text-sm font-semibold text-white mb-2">Daily performance</h3>
          {performanceQuery.isPending ? (
            <p className="text-gray-400 text-sm flex items-center space-x-2">
              <Loader2 className="w-4 h-4 animate-spin" />
              <span>Loading performance…</span>
            </p>
          ) : performanceQuery.error ? (
            <p className="text-red-400 text-xs" role="alert">
              {performanceQuery.error.message}
            </p>
          ) : (performanceQuery.data ?? []).length === 0 ? (
            <p className="text-gray-500 text-xs">No performance recorded.</p>
          ) : (
            <ul className="space-y-1">
              {(performanceQuery.data ?? []).map((day) => (
                <li key={day.id} className="text-xs text-gray-300">
                  {day.day.slice(0, 10)} · PnL {day.realised_pnl} · volume {day.volume} ·{' '}
                  {day.orders_filled}/{day.orders_placed} filled
                </li>
              ))}
            </ul>
          )}
        </div>
      )}
    </div>
  );
}
