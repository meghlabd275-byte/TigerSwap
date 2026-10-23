'use client';

import { useState } from 'react';
import { Search, Star, TrendingUp, Clock } from 'lucide-react';
import { useTokens, usePrices } from '@/hooks/useApi';
import type { Token as ApiToken } from '@/lib/types';

interface TokenListProps {
  onSelectToken: (token: ApiToken) => void;
}


export function TokenList({ onSelectToken }: TokenListProps) {
  const [searchQuery, setSearchQuery] = useState('');
  const [activeTab, setActiveTab] = useState<'all' | 'popular' | 'favorites'>('all');

  const { data: tokens, error: tokensError, isPending } = useTokens();
  const { data: prices } = usePrices();

  const priceBySymbol = new Map((prices ?? []).map((entry) => [entry.symbol, entry]));

  const available = tokens ?? [];
  const filteredTokens = available.filter(
    (token) =>
      token.symbol.toLowerCase().includes(searchQuery.toLowerCase()) ||
      token.name.toLowerCase().includes(searchQuery.toLowerCase()),
  );

  const popularTokens = available.slice(0, 10);

  return (
    <div className="w-full max-w-md">
      <div className="mb-4">
        <div className="relative">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-5 h-5 text-gray-400" />
          <input
            type="text"
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            placeholder="Search tokens..."
            className="w-full pl-10 pr-4 py-3 bg-white/5 border border-white/10 rounded-xl text-white placeholder:text-gray-500 focus:outline-none focus:border-tiger-orange/50"
          />
        </div>
      </div>

      <div className="flex space-x-2 mb-4">
        {[
          { id: 'all', label: 'All', icon: null },
          { id: 'popular', label: 'Popular', icon: TrendingUp },
          { id: 'favorites', label: 'Favorites', icon: Star },
        ].map(tab => (
          <button
            key={tab.id}
            onClick={() => setActiveTab(tab.id as 'all' | 'popular' | 'favorites')}
            className={`flex-1 flex items-center justify-center space-x-2 px-4 py-2 rounded-lg transition-colors ${
              activeTab === tab.id
                ? 'bg-tiger-orange/20 text-tiger-orange border border-tiger-orange/30'
                : 'bg-white/5 text-gray-400 hover:text-white'
            }`}
          >
            {tab.icon && <tab.icon className="w-4 h-4" />}
            <span>{tab.label}</span>
          </button>
        ))}
      </div>

      {tokensError && (
        <p role="alert" className="text-red-400 text-sm mb-2">
          Token list unavailable: {tokensError.message}
        </p>
      )}
      {isPending && <p className="text-gray-400 text-sm mb-2">Loading tokens…</p>}

      <div className="space-y-1 max-h-80 overflow-y-auto">
        {(activeTab === 'popular' ? popularTokens : filteredTokens).map(token => (
          <button
            key={`${token.chainId}-${token.symbol}`}
            onClick={() => onSelectToken(token)}
            className="w-full flex items-center space-x-3 p-3 rounded-xl hover:bg-white/5 transition-colors"
          >
            {token.logoURI ? (
              <img src={token.logoURI} alt="" className="w-10 h-10 rounded-full" />
            ) : (
              <div className="w-10 h-10 rounded-full bg-white/10 flex items-center justify-center text-xs text-white">
                {token.symbol.slice(0, 2)}
              </div>
            )}
            <div className="flex-1 text-left">
              <div className="text-white font-medium">{token.symbol}</div>
              <div className="text-gray-500 text-sm">{token.name}</div>
            </div>
            <div className="text-right">
              <div className="text-white font-medium">
                {(() => {
                  const entry = priceBySymbol.get(token.symbol);
                  if (!entry) return '—';
                  return `$${entry.priceUSD.toLocaleString()}`;
                })()}
              </div>
              <div
                className={`text-xs ${
                  (priceBySymbol.get(token.symbol)?.change24h ?? 0) >= 0
                    ? 'text-green-400'
                    : 'text-red-400'
                }`}
              >
                {(() => {
                  const entry = priceBySymbol.get(token.symbol);
                  if (!entry) return '—';
                  return `${entry.change24h >= 0 ? '+' : ''}${entry.change24h.toFixed(2)}%`;
                })()}
              </div>
            </div>
          </button>
        ))}
      </div>
    </div>
  );
}
