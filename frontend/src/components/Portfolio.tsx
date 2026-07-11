'use client';

import { useState } from 'react';
import { motion } from 'framer-motion';
import { 
  Wallet, 
  TrendingUp, 
  TrendingDown, 
  ArrowUpRight, 
  ArrowDownRight,
  RefreshCw,
  Download,
  Eye,
  EyeOff,
  Copy,
  ExternalLink
} from 'lucide-react';

const PORTFOLIO_DATA = {
  totalValue: 24580.50,
  change24h: 1250.30,
  changePercent: 5.35,
  assets: [
    { symbol: 'ETH', name: 'Ethereum', balance: 5.2, value: 17940, change: 3.2, icon: '🦄' },
    { symbol: 'USDC', name: 'USD Coin', balance: 3000, value: 3000, change: 0, icon: '💵' },
    { symbol: 'WBTC', name: 'Wrapped Bitcoin', balance: 0.08, value: 5400, change: 4.1, icon: '₿' },
    { symbol: 'UNI', name: 'Uniswap', balance: 45, value: 576, change: -2.3, icon: '🦄' },
    { symbol: 'LINK', name: 'Chainlink', balance: 36, value: 666, change: 1.8, icon: '🔗' },
  ],
  transactions: [
    { type: 'swap', from: 'ETH', to: 'USDC', amount: '1.5 ETH', value: '$5,175', time: '2 hours ago', status: 'completed' },
    { type: 'receive', from: 'External', to: 'ETH', amount: '2.0 ETH', value: '$6,900', time: '5 hours ago', status: 'completed' },
    { type: 'swap', from: 'USDC', to: 'WBTC', amount: '$3,000', value: '$3,000', time: '1 day ago', status: 'completed' },
    { type: 'send', from: 'ETH', to: 'External', amount: '0.5 ETH', value: '$1,725', time: '2 days ago', status: 'completed' },
  ]
};

export function Portfolio() {
  const [showBalances, setShowBalances] = useState(true);
  const [activeTab, setActiveTab] = useState<'assets' | 'transactions'>('assets');

  const formatValue = (value: number) => {
    if (!showBalances) return '******';
    return `$${value.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`;
  };

  const formatBalance = (balance: number, symbol: string) => {
    if (!showBalances) return '******';
    return `${balance.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 6 })} ${symbol}`;
  };

  return (
    <div className="space-y-6">
      {/* Portfolio Summary */}
      <div className="glass-dark rounded-2xl p-6">
        <div className="flex items-center justify-between mb-6">
          <h2 className="text-xl font-display font-bold text-white">Portfolio</h2>
          <div className="flex items-center space-x-2">
            <button 
              onClick={() => setShowBalances(!showBalances)}
              className="p-2 text-gray-400 hover:text-white transition-colors"
            >
              {showBalances ? <Eye className="w-4 h-4" /> : <EyeOff className="w-4 h-4" />}
            </button>
            <button className="p-2 text-gray-400 hover:text-white transition-colors">
              <RefreshCw className="w-4 h-4" />
            </button>
            <button className="p-2 text-gray-400 hover:text-white transition-colors">
              <Download className="w-4 h-4" />
            </button>
          </div>
        </div>

        {/* Total Value */}
        <div className="mb-6">
          <div className="text-gray-400 text-sm mb-1">Total Balance</div>
          <div className="flex items-baseline space-x-3">
            <span className="text-4xl font-display font-bold text-white">
              {formatValue(PORTFOLIO_DATA.totalValue)}
            </span>
            <div className={`flex items-center ${PORTFOLIO_DATA.changePercent >= 0 ? 'text-green-400' : 'text-red-400'}`}>
              {PORTFOLIO_DATA.changePercent >= 0 ? <TrendingUp className="w-4 h-4" /> : <TrendingDown className="w-4 h-4" />}
              <span className="font-medium">+{PORTFOLIO_DATA.changePercent}%</span>
              <span className="text-gray-500 ml-1">24h</span>
            </div>
          </div>
          <div className="text-gray-400 text-sm mt-1">
            {PORTFOLIO_DATA.change24h >= 0 ? '+' : ''}{formatValue(PORTFOLIO_DATA.change24h)}
          </div>
        </div>

        {/* Tabs */}
        <div className="flex space-x-2 mb-4">
          <button
            onClick={() => setActiveTab('assets')}
            className={`px-4 py-2 rounded-lg text-sm font-medium transition-colors ${
              activeTab === 'assets'
                ? 'bg-tiger-orange/20 text-tiger-orange'
                : 'text-gray-400 hover:text-white'
            }`}
          >
            Assets
          </button>
          <button
            onClick={() => setActiveTab('transactions')}
            className={`px-4 py-2 rounded-lg text-sm font-medium transition-colors ${
              activeTab === 'transactions'
                ? 'bg-tiger-orange/20 text-tiger-orange'
                : 'text-gray-400 hover:text-white'
            }`}
          >
            Transactions
          </button>
        </div>

        {/* Content */}
        {activeTab === 'assets' ? (
          <div className="space-y-3">
            {PORTFOLIO_DATA.assets.map((asset, i) => (
              <motion.div
                key={asset.symbol}
                initial={{ opacity: 0, x: -20 }}
                animate={{ opacity: 1, x: 0 }}
                transition={{ delay: i * 0.1 }}
                className="flex items-center justify-between p-3 bg-white/5 rounded-xl hover:bg-white/10 transition-colors"
              >
                <div className="flex items-center space-x-3">
                  <div className="w-10 h-10 rounded-full bg-white/10 flex items-center justify-center text-xl">
                    {asset.icon}
                  </div>
                  <div>
                    <div className="text-white font-medium">{asset.symbol}</div>
                    <div className="text-gray-500 text-xs">{asset.name}</div>
                  </div>
                </div>
                <div className="text-right">
                  <div className="text-white font-medium">{formatBalance(asset.balance, asset.symbol)}</div>
                  <div className={`text-xs ${asset.change >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                    {asset.change >= 0 ? '+' : ''}{asset.change}%
                  </div>
                </div>
                <div className="text-right ml-4">
                  <div className="text-white font-medium">{formatValue(asset.value)}</div>
                  <div className="text-gray-500 text-xs">{(asset.value / PORTFOLIO_DATA.totalValue * 100).toFixed(1)}%</div>
                </div>
              </motion.div>
            ))}
          </div>
        ) : (
          <div className="space-y-3">
            {PORTFOLIO_DATA.transactions.map((tx, i) => (
              <motion.div
                key={i}
                initial={{ opacity: 0, x: -20 }}
                animate={{ opacity: 1, x: 0 }}
                transition={{ delay: i * 0.1 }}
                className="flex items-center justify-between p-3 bg-white/5 rounded-xl"
              >
                <div className="flex items-center space-x-3">
                  <div className={`w-8 h-8 rounded-full flex items-center justify-center ${
                    tx.type === 'swap' ? 'bg-blue-500/20 text-blue-400' :
                    tx.type === 'receive' ? 'bg-green-500/20 text-green-400' :
                    'bg-red-500/20 text-red-400'
                  }`}>
                    {tx.type === 'swap' ? <RefreshCw className="w-4 h-4" /> :
                     tx.type === 'receive' ? <ArrowDownRight className="w-4 h-4" /> :
                     <ArrowUpRight className="w-4 h-4" />}
                  </div>
                  <div>
                    <div className="text-white font-medium capitalize">{tx.type}</div>
                    <div className="text-gray-500 text-xs">{tx.time}</div>
                  </div>
                </div>
                <div className="text-right">
                  <div className="text-white text-sm">
                    {tx.type === 'swap' ? `${tx.from} → ${tx.to}` :
                     tx.type === 'receive' ? `From ${tx.from}` :
                     `To ${tx.to}`}
                  </div>
                  <div className="text-gray-500 text-xs">{tx.value}</div>
                </div>
              </motion.div>
            ))}
          </div>
        )}
      </div>

      {/* Quick Actions */}
      <div className="grid grid-cols-3 gap-4">
        <button className="flex flex-col items-center justify-center p-4 bg-white/5 rounded-xl hover:bg-white/10 transition-colors">
          <div className="w-10 h-10 rounded-full bg-tiger-orange/20 flex items-center justify-center mb-2">
            <ArrowUpRight className="w-5 h-5 text-tiger-orange" />
          </div>
          <span className="text-white text-sm">Send</span>
        </button>
        <button className="flex flex-col items-center justify-center p-4 bg-white/5 rounded-xl hover:bg-white/10 transition-colors">
          <div className="w-10 h-10 rounded-full bg-green-500/20 flex items-center justify-center mb-2">
            <ArrowDownRight className="w-5 h-5 text-green-400" />
          </div>
          <span className="text-white text-sm">Receive</span>
        </button>
        <button className="flex flex-col items-center justify-center p-4 bg-white/5 rounded-xl hover:bg-white/10 transition-colors">
          <div className="w-10 h-10 rounded-full bg-blue-500/20 flex items-center justify-center mb-2">
            <RefreshCw className="w-5 h-5 text-blue-400" />
          </div>
          <span className="text-white text-sm">Swap</span>
        </button>
      </div>
    </div>
  );
}
