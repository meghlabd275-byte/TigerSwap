'use client';

/**
 * Copy trading panel.
 *
 * The leaderboard comes from `GET /copy-trading/traders`, which the Rust copy
 * engine fills from real trader activity. Following a trader calls
 * `POST /copy-trading/follow` and is bound to the session token, so a client
 * cannot follow on another account's behalf. When the copy-trading service has
 * no traders the panel says so plainly rather than inventing win rates or
 * profits.
 */

import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { AlertTriangle, Check, Copy, Loader2, Users } from 'lucide-react';

import { endpoints } from '@/lib/endpoints';
import type { CopyPosition, CopyTrader } from '@/lib/types';
import { isAuthenticated } from '@/lib/auth';

export function CopyTrading() {
  const [selected, setSelected] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const queryClient = useQueryClient();

  const { data: traders, error, isPending } = useQuery<CopyTrader[], Error>({
    queryKey: ['copy-trading', 'traders'],
    queryFn: () => endpoints.listCopyTraders(20),
    retry: false,
  });

  const { data: positions } = useQuery<CopyPosition[], Error>({
    queryKey: ['copy-trading', 'positions'],
    queryFn: () => endpoints.getCopyPositions(),
    enabled: isAuthenticated(),
    retry: false,
  });

  const follow = useMutation({
    mutationFn: (address: string) =>
      endpoints.followTrader({ trader_address: address, copy_ratio: 1 }),
    onSuccess: (_result, address) => {
      setSelected(address);
      setNotice('Now following this trader.');
      void queryClient.invalidateQueries({ queryKey: ['copy-trading'] });
    },
    onError: (mutationError: Error) => setNotice(mutationError.message),
  });

  if (isPending) {
    return (
      <div className="glass-dark rounded-2xl p-8 text-center text-gray-400">
        <Loader2 className="w-8 h-8 mx-auto mb-3 animate-spin" />
        Loading traders…
      </div>
    );
  }

  if (error) {
    return (
      <div className="glass-dark rounded-2xl p-8 text-center">
        <AlertTriangle className="w-10 h-10 text-yellow-500 mx-auto mb-4" />
        <h3 className="text-white font-semibold mb-2">Copy trading unavailable</h3>
        <p className="text-gray-400 text-sm" role="alert">
          {error.message}
        </p>
      </div>
    );
  }

  if (!traders || traders.length === 0) {
    return (
      <div className="glass-dark rounded-2xl p-8 text-center">
        <Users className="w-10 h-10 text-gray-500 mx-auto mb-4" />
        <h3 className="text-white font-semibold mb-2">No traders to copy yet</h3>
        <p className="text-gray-400 text-sm">
          Traders appear here once the copy-trading service has recorded activity.
        </p>
      </div>
    );
  }

  return (
    <div className="glass-dark rounded-2xl p-6">
      <div className="flex items-center justify-between mb-6">
        <h2 className="text-lg font-display font-bold text-white">Copy trading</h2>
        <Users className="w-5 h-5 text-tiger-orange" />
      </div>

      {notice && (
        <p className="mb-4 text-sm text-tiger-orange" role="status">
          {notice}
        </p>
      )}

      {positions && positions.length > 0 && (
        <div className="mb-6">
          <h3 className="text-sm text-gray-400 mb-2">Your copied positions</h3>
          <div className="space-y-2">
            {positions.map((position) => (
              <div
                key={position.id}
                className="flex items-center justify-between p-3 bg-white/5 rounded-xl text-sm"
              >
                <div>
                  <p className="text-white">
                    {position.pair} · {position.side} · {position.leverage}x
                  </p>
                  <p className="text-gray-500 text-xs">{position.status}</p>
                </div>
                <p className={Number(position.pnl) >= 0 ? 'text-green-400' : 'text-red-400'}>
                  {position.pnl}
                </p>
              </div>
            ))}
          </div>
        </div>
      )}

      <div className="space-y-3">
        {traders.map((trader) => (
          <div
            key={trader.address}
            className={`p-4 rounded-xl border transition-colors ${
              selected === trader.address
                ? 'border-tiger-orange bg-tiger-orange/10'
                : 'border-white/10 bg-white/5'
            }`}
          >
            <div className="flex items-center justify-between mb-2">
              <div>
                <p className="text-white font-medium">
                  {trader.name}
                  {trader.is_verified && (
                    <span className="ml-2 text-tiger-orange text-xs">verified</span>
                  )}
                </p>
                <p className="text-gray-500 text-xs font-mono">{trader.address}</p>
              </div>
              <button
                type="button"
                disabled={!isAuthenticated() || follow.isPending}
                onClick={() => follow.mutate(trader.address)}
                title={isAuthenticated() ? undefined : 'Sign in to copy a trader'}
                className="flex items-center space-x-1 px-3 py-2 rounded-lg bg-tiger-orange/20 text-tiger-orange text-sm hover:bg-tiger-orange/30 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
              >
                {selected === trader.address ? (
                  <Check className="w-4 h-4" />
                ) : (
                  <Copy className="w-4 h-4" />
                )}
                <span>{selected === trader.address ? 'Following' : 'Copy'}</span>
              </button>
            </div>
            <div className="grid grid-cols-3 gap-2 text-sm">
              <div>
                <p className="text-gray-400 text-xs">Win rate</p>
                <p className="text-white">{(trader.win_rate * 100).toFixed(1)}%</p>
              </div>
              <div>
                <p className="text-gray-400 text-xs">Weekly profit</p>
                <p className={trader.weekly_profit >= 0 ? 'text-green-400' : 'text-red-400'}>
                  ${trader.weekly_profit.toLocaleString()}
                </p>
              </div>
              <div>
                <p className="text-gray-400 text-xs">Followers</p>
                <p className="text-white">{trader.followers.toLocaleString()}</p>
              </div>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
