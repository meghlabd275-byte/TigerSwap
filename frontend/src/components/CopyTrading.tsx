'use client';

import { useState } from 'react';
import { motion } from 'framer-motion';
import { 
  Users, 
  TrendingUp, 
  TrendingDown, 
  Copy, 
  Wallet, 
  BarChart3,
  Star,
  Clock,
  Zap,
  AlertTriangle,
  CheckCircle,
  RefreshCw,
  ArrowUpRight,
  ArrowDownRight,
  Eye,
  EyeOff,
  Play,
  Pause
} from 'lucide-react';

interface Trader {
  id: string;
  address: string;
  name: string;
  avatar: string;
  winRate: number;
  totalProfit: string;
  followers: number;
  weeklyProfit: number;
  monthlyProfit: number;
  isVerified: boolean;
  isPro: boolean;
  strategies: string[];
  trades: number;
  avgHoldingTime: string;
  riskScore: 'low' | 'medium' | 'high';
}

interface CopyTrade {
  id: string;
  traderId: string;
  traderName: string;
  type: 'buy' | 'sell';
  token: string;
  amount: string;
  profit: string;
  timestamp: Date;
  status: 'active' | 'closed';
}

const MOCK_TRADERS: Trader[] = [
  {
    id: '1',
    address: '0x742d35Cc6634C0532925a3b844Bc9e7595f0fEb1',
    name: 'CryptoWhale',
    avatar: '🐋',
    winRate: 78.5,
    totalProfit: '245.8 ETH',
    followers: 12543,
    weeklyProfit: 12.4,
    monthlyProfit: 45.2,
    isVerified: true,
    isPro: true,
    strategies: ['Momentum', 'Breakout'],
    trades: 1847,
    avgHoldingTime: '2.5h',
    riskScore: 'medium'
  },
  {
    id: '2',
    address: '0x1234567890abcdef1234567890abcdef12345678',
    name: 'DeFiMaster',
    avatar: '🎯',
    winRate: 82.3,
    totalProfit: '189.5 ETH',
    followers: 8934,
    weeklyProfit: 8.7,
    monthlyProfit: 32.1,
    isVerified: true,
    isPro: true,
    strategies: ['Grid Trading', 'Arbitrage'],
    trades: 2341,
    avgHoldingTime: '1.2h',
    riskScore: 'low'
  },
  {
    id: '3',
    address: '0xabcdef1234567890abcdef1234567890abcdef12',
    name: 'YieldHunter',
    avatar: '🦁',
    winRate: 71.2,
    totalProfit: '156.3 ETH',
    followers: 5621,
    weeklyProfit: 15.8,
    monthlyProfit: 52.3,
    isVerified: true,
    isPro: false,
    strategies: ['Yield Farming', 'Staking'],
    trades: 923,
    avgHoldingTime: '5d',
    riskScore: 'low'
  },
  {
    id: '4',
    address: '0x9876543210fedcba9876543210fedcba98765432',
    name: 'SniperPro',
    avatar: '🎯',
    winRate: 65.8,
    totalProfit: '312.1 ETH',
    followers: 15782,
    weeklyProfit: 22.4,
    monthlyProfit: 78.9,
    isVerified: false,
    isPro: true,
    strategies: ['Meme Coins', 'New Pairs'],
    trades: 456,
    avgHoldingTime: '15m',
    riskScore: 'high'
  },
  {
    id: '5',
    address: '0xfedcba9876543210fedcba9876543210fedcba98',
    name: 'StableYield',
    avatar: '🛡️',
    winRate: 95.2,
    totalProfit: '89.4 ETH',
    followers: 3421,
    weeklyProfit: 3.2,
    monthlyProfit: 12.8,
    isVerified: true,
    isPro: false,
    strategies: ['Stablecoin', 'Options'],
    trades: 5678,
    avgHoldingTime: '7d',
    riskScore: 'low'
  }
];

const RECENT_TRADES: CopyTrade[] = [
  { id: '1', traderId: '1', traderName: 'CryptoWhale', type: 'buy', token: 'PEPE', amount: '50,000,000', profit: '+234%', timestamp: new Date(), status: 'active' },
  { id: '2', traderId: '2', traderName: 'DeFiMaster', type: 'sell', token: 'ARB', amount: '1,250', profit: '+12.5%', timestamp: new Date(), status: 'closed' },
  { id: '3', traderId: '3', traderName: 'YieldHunter', type: 'buy', token: 'OP', amount: '850', profit: '+28.3%', timestamp: new Date(), status: 'active' },
  { id: '4', traderId: '4', traderName: 'SniperPro', type: 'buy', token: 'BONK', amount: '100,000,000', profit: '+156%', timestamp: new Date(), status: 'active' },
  { id: '5', traderId: '5', traderName: 'StableYield', type: 'sell', token: 'USDC', amount: '10,000', profit: '+0.8%', timestamp: new Date(), status: 'closed' },
];

interface CopyTradingProps {
  isAdmin?: boolean;
}

export function CopyTrading({ isAdmin = false }: CopyTradingProps) {
  const [activeTab, setActiveTab] = useState<'traders' | 'portfolio' | 'history'>('traders');
  const [following, setFollowing] = useState<string[]>([]);
  const [searchQuery, setSearchQuery] = useState('');
  const [sortBy, setSortBy] = useState<'followers' | 'profit' | 'winRate'>('followers');
  const [filterStrategy, setFilterStrategy] = useState<string | null>(null);
  const [copyAmount, setCopyAmount] = useState<Record<string, string>>({});
  const [isCopying, setIsCopying] = useState<Record<string, boolean>>({});

  const filteredTraders = MOCK_TRADERS
    .filter(t => 
      t.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
      t.address.toLowerCase().includes(searchQuery.toLowerCase())
    )
    .filter(t => !filterStrategy || t.strategies.includes(filterStrategy))
    .sort((a, b) => {
      if (sortBy === 'followers') return b.followers - a.followers;
      if (sortBy === 'profit') return parseFloat(b.totalProfit) - parseFloat(a.totalProfit);
      return b.winRate - a.winRate;
    });

  const handleFollow = (traderId: string) => {
    if (following.includes(traderId)) {
      setFollowing(following.filter(id => id !== traderId));
    } else {
      setFollowing([...following, traderId]);
    }
  };

  const handleStartCopy = (traderId: string) => {
    if (!copyAmount[traderId] || parseFloat(copyAmount[traderId]) <= 0) return;
    setIsCopying({ ...isCopying, [traderId]: true });
    setTimeout(() => {
      setIsCopying({ ...isCopying, [traderId]: false });
      if (!following.includes(traderId)) {
        setFollowing([...following, traderId]);
      }
    }, 2000);
  };

  const allStrategies = [...new Set(MOCK_TRADERS.flatMap(t => t.strategies))];

  return (
    <div className="space-y-6">
      {/* Header Stats */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        <div className="bg-white/5 rounded-xl p-4">
          <div className="flex items-center space-x-2 text-gray-400 mb-2">
            <Users className="w-4 h-4" />
            <span className="text-sm">Total Traders</span>
          </div>
          <p className="text-2xl font-bold text-white">{MOCK_TRADERS.length}</p>
        </div>
        <div className="bg-white/5 rounded-xl p-4">
          <div className="flex items-center space-x-2 text-gray-400 mb-2">
            <TrendingUp className="w-4 h-4" />
            <span className="text-sm">Avg. Win Rate</span>
          </div>
          <p className="text-2xl font-bold text-white">
            {(MOCK_TRADERS.reduce((a, b) => a + b.winRate, 0) / MOCK_TRADERS.length).toFixed(1)}%
          </p>
        </div>
        <div className="bg-white/5 rounded-xl p-4">
          <div className="flex items-center space-x-2 text-gray-400 mb-2">
            <Wallet className="w-4 h-4" />
            <span className="text-sm">Active Copiers</span>
          </div>
          <p className="text-2xl font-bold text-white">
            {MOCK_TRADERS.reduce((a, b) => a + b.followers, 0).toLocaleString()}
          </p>
        </div>
        <div className="bg-white/5 rounded-xl p-4">
          <div className="flex items-center space-x-2 text-gray-400 mb-2">
            <BarChart3 className="w-4 h-4" />
            <span className="text-sm">Total Profit</span>
          </div>
          <p className="text-2xl font-bold text-green-400">
            {MOCK_TRADERS.reduce((a, b) => a + parseFloat(b.totalProfit), 0).toFixed(1)} ETH
          </p>
        </div>
      </div>

      {/* Tabs */}
      <div className="flex items-center space-x-2 border-b border-white/10">
        {[
          { id: 'traders', label: 'Top Traders', icon: Users },
          { id: 'portfolio', label: 'My Portfolio', icon: Wallet },
          { id: 'history', label: 'Trade History', icon: Clock },
        ].map(tab => (
          <button
            key={tab.id}
            onClick={() => setActiveTab(tab.id as any)}
            className={`flex items-center space-x-2 px-4 py-3 border-b-2 transition-colors ${
              activeTab === tab.id
                ? 'border-tiger-orange text-tiger-orange'
                : 'border-transparent text-gray-400 hover:text-white'
            }`}
          >
            <tab.icon className="w-4 h-4" />
            <span>{tab.label}</span>
          </button>
        ))}
      </div>

      {/* Filters */}
      {activeTab === 'traders' && (
        <div className="flex flex-wrap items-center gap-4">
          <div className="relative flex-1 min-w-[200px]">
            <input
              type="text"
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              placeholder="Search traders..."
              className="w-full bg-white/10 border border-white/20 rounded-lg px-4 py-2 text-white placeholder-gray-500 focus:outline-none focus:border-tiger-orange"
            />
          </div>
          <select
            value={sortBy}
            onChange={(e) => setSortBy(e.target.value as any)}
            className="bg-white/10 border border-white/20 rounded-lg px-4 py-2 text-white focus:outline-none focus:border-tiger-orange"
          >
            <option value="followers">Sort by Followers</option>
            <option value="profit">Sort by Profit</option>
            <option value="winRate">Sort by Win Rate</option>
          </select>
          <select
            value={filterStrategy || ''}
            onChange={(e) => setFilterStrategy(e.target.value || null)}
            className="bg-white/10 border border-white/20 rounded-lg px-4 py-2 text-white focus:outline-none focus:border-tiger-orange"
          >
            <option value="">All Strategies</option>
            {allStrategies.map(s => (
              <option key={s} value={s}>{s}</option>
            ))}
          </select>
        </div>
      )}

      {/* Content */}
      {activeTab === 'traders' && (
        <div className="space-y-4">
          {filteredTraders.map(trader => (
            <motion.div
              key={trader.id}
              initial={{ opacity: 0, y: 10 }}
              animate={{ opacity: 1, y: 0 }}
              className="bg-white/5 rounded-xl p-6"
            >
              <div className="flex flex-wrap items-start justify-between gap-4">
                <div className="flex items-start space-x-4">
                  <div className="w-14 h-14 bg-gradient-to-br from-tiger-orange to-yellow-500 rounded-full flex items-center justify-center text-2xl">
                    {trader.avatar}
                  </div>
                  <div>
                    <div className="flex items-center space-x-2">
                      <h3 className="text-white font-semibold">{trader.name}</h3>
                      {trader.isVerified && <CheckCircle className="w-4 h-4 text-blue-400" />}
                      {trader.isPro && <Star className="w-4 h-4 text-yellow-400" />}
                    </div>
                    <p className="text-gray-400 text-sm font-mono">
                      {trader.address.slice(0, 6)}...{trader.address.slice(-4)}
                    </p>
                    <div className="flex items-center space-x-2 mt-2">
                      {trader.strategies.map(s => (
                        <span key={s} className="px-2 py-0.5 bg-white/10 rounded text-xs text-gray-300">
                          {s}
                        </span>
                      ))}
                    </div>
                  </div>
                </div>

                <div className="grid grid-cols-2 md:grid-cols-4 gap-4 md:gap-8">
                  <div className="text-center">
                    <p className="text-gray-400 text-sm">Win Rate</p>
                    <p className="text-green-400 font-bold">{trader.winRate}%</p>
                  </div>
                  <div className="text-center">
                    <p className="text-gray-400 text-sm">Total Profit</p>
                    <p className="text-green-400 font-bold">{trader.totalProfit}</p>
                  </div>
                  <div className="text-center">
                    <p className="text-gray-400 text-sm">Followers</p>
                    <p className="text-white font-bold">{trader.followers.toLocaleString()}</p>
                  </div>
                  <div className="text-center">
                    <p className="text-gray-400 text-sm">Risk</p>
                    <span className={`px-2 py-0.5 rounded text-xs font-medium ${
                      trader.riskScore === 'low' ? 'bg-green-500/20 text-green-400' :
                      trader.riskScore === 'medium' ? 'bg-yellow-500/20 text-yellow-400' :
                      'bg-red-500/20 text-red-400'
                    }`}>
                      {trader.riskScore.toUpperCase()}
                    </span>
                  </div>
                </div>
              </div>

              <div className="flex flex-wrap items-center justify-between gap-4 mt-4 pt-4 border-t border-white/10">
                <div className="flex items-center space-x-4 text-sm text-gray-400">
                  <span>{trader.trades.toLocaleString()} trades</span>
                  <span>Avg: {trader.avgHoldingTime}</span>
                  <span className="text-green-400">+{trader.weeklyProfit}% this week</span>
                </div>
                <div className="flex items-center space-x-3">
                  {following.includes(trader.id) ? (
                    <>
                      <div className="flex items-center space-x-2">
                        <input
                          type="number"
                          value={copyAmount[trader.id] || ''}
                          onChange={(e) => setCopyAmount({ ...copyAmount, [trader.id]: e.target.value })}
                          placeholder="Amount"
                          className="w-24 bg-white/10 border border-white/20 rounded-lg px-3 py-2 text-white text-sm"
                        />
                        <span className="text-gray-400 text-sm">ETH</span>
                      </div>
                      <button
                        onClick={() => handleStartCopy(trader.id)}
                        disabled={isCopying[trader.id]}
                        className="flex items-center space-x-2 px-4 py-2 bg-tiger-orange text-white rounded-lg hover:bg-tiger-accent transition-colors disabled:opacity-50"
                      >
                        {isCopying[trader.id] ? (
                          <RefreshCw className="w-4 h-4 animate-spin" />
                        ) : (
                          <Copy className="w-4 h-4" />
                        )}
                        <span>Copy Trade</span>
                      </button>
                    </>
                  ) : (
                    <button
                      onClick={() => handleFollow(trader.id)}
                      className="flex items-center space-x-2 px-4 py-2 bg-white/10 text-white rounded-lg hover:bg-white/20 transition-colors"
                    >
                      <Star className="w-4 h-4" />
                      <span>Follow</span>
                    </button>
                  )}
                </div>
              </div>
            </motion.div>
          ))}
        </div>
      )}

      {activeTab === 'portfolio' && (
        <div className="bg-white/5 rounded-xl p-8 text-center">
          <Wallet className="w-16 h-16 text-gray-400 mx-auto mb-4" />
          <h3 className="text-white font-semibold mb-2">No Active Copy Positions</h3>
          <p className="text-gray-400 mb-4">Start following traders to copy their trades automatically</p>
          <button
            onClick={() => setActiveTab('traders')}
            className="px-6 py-3 bg-tiger-orange text-white rounded-lg hover:bg-tiger-accent transition-colors"
          >
            Browse Traders
          </button>
        </div>
      )}

      {activeTab === 'history' && (
        <div className="bg-white/5 rounded-xl overflow-hidden">
          <table className="w-full">
            <thead className="bg-white/5">
              <tr>
                <th className="text-left px-6 py-4 text-gray-400 font-medium">Trader</th>
                <th className="text-left px-6 py-4 text-gray-400 font-medium">Type</th>
                <th className="text-left px-6 py-4 text-gray-400 font-medium">Token</th>
                <th className="text-left px-6 py-4 text-gray-400 font-medium">Amount</th>
                <th className="text-left px-6 py-4 text-gray-400 font-medium">Profit</th>
                <th className="text-left px-6 py-4 text-gray-400 font-medium">Status</th>
              </tr>
            </thead>
            <tbody>
              {RECENT_TRADES.map(trade => (
                <tr key={trade.id} className="border-t border-white/5">
                  <td className="px-6 py-4 text-white">{trade.traderName}</td>
                  <td className="px-6 py-4">
                    <span className={`flex items-center space-x-1 ${
                      trade.type === 'buy' ? 'text-green-400' : 'text-red-400'
                    }`}>
                      {trade.type === 'buy' ? <ArrowUpRight className="w-4 h-4" /> : <ArrowDownRight className="w-4 h-4" />}
                      <span className="capitalize">{trade.type}</span>
                    </span>
                  </td>
                  <td className="px-6 py-4 text-white">{trade.token}</td>
                  <td className="px-6 py-4 text-gray-300">{trade.amount}</td>
                  <td className="px-6 py-4 text-green-400">{trade.profit}</td>
                  <td className="px-6 py-4">
                    <span className={`px-2 py-1 rounded text-xs ${
                      trade.status === 'active' 
                        ? 'bg-blue-500/20 text-blue-400' 
                        : 'bg-gray-500/20 text-gray-400'
                    }`}>
                      {trade.status}
                    </span>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
