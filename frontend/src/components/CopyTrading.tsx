'use client';

/**
 * Copy trading panel.
 *
 * The leaderboard and trade history come from the API. When the copy-trading
 * service is not available the panel reports that plainly rather than showing
 * invented traders, win rates or profits.
 */

import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { AlertTriangle, Copy, Loader2, Users } from 'lucide-react';

import { apiRequest } from '@/lib/apiClient';
import { arrayLenient, nonNegative, number, object, string } from '@/lib/validation';

interface Leader {
  address: string;
  displayName: string;
  winRate: number;
  totalProfitUSD: number;
  followers: number;
  trades: number;
}

const leaderValidator = object<Leader>({
  address: string,
  displayName: string,
  winRate: number,
  totalProfitUSD: number,
  followers: nonNegative,
  trades: nonNegative,
});

export function CopyTrading() {
  const [selected, setSelected] = useState<string | null>(null);

  const { data: leaders, error, isPending } = useQuery<Leader[], Error>({
    queryKey: ['copy-trading', 'leaders'],
    queryFn: async () => {
      const body = await apiRequest('/copy-trading/leaders', {
        authenticated: false,
        timeoutMs: 8000,
      });
      return arrayLenient(leaderValidator)(body, 'leaders');
    },
    retry: false,
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

  if (!leaders || leaders.length === 0) {
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

      <div className="space-y-3">
        {leaders.map((leader) => (
          <div
            key={leader.address}
            className={`p-4 rounded-xl border transition-colors ${
              selected === leader.address
                ? 'border-tiger-orange bg-tiger-orange/10'
                : 'border-white/10 bg-white/5'
            }`}
          >
            <div className="flex items-center justify-between mb-2">
              <div>
                <p className="text-white font-medium">{leader.displayName}</p>
                <p className="text-gray-500 text-xs font-mono">{leader.address}</p>
              </div>
              <button
                onClick={() => setSelected(leader.address)}
                className="flex items-center space-x-1 px-3 py-2 rounded-lg bg-tiger-orange/20 text-tiger-orange text-sm hover:bg-tiger-orange/30 transition-colors"
              >
                <Copy className="w-4 h-4" />
                <span>Copy</span>
              </button>
            </div>
            <div className="grid grid-cols-3 gap-2 text-sm">
              <div>
                <p className="text-gray-400 text-xs">Win rate</p>
                <p className="text-white">{(leader.winRate * 100).toFixed(1)}%</p>
              </div>
              <div>
                <p className="text-gray-400 text-xs">Profit</p>
                <p className={leader.totalProfitUSD >= 0 ? 'text-green-400' : 'text-red-400'}>
                  ${leader.totalProfitUSD.toLocaleString()}
                </p>
              </div>
              <div>
                <p className="text-gray-400 text-xs">Followers</p>
                <p className="text-white">{leader.followers.toLocaleString()}</p>
              </div>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
