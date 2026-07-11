'use client';

import { useState } from 'react';
import { motion } from 'framer-motion';
import { 
  TrendingUp, 
  TrendingDown, 
  Clock, 
  Settings, 
  AlertTriangle,
  Zap,
  Target,
  BarChart3,
  Layers
} from 'lucide-react';

const PERPETUAL_PAIRS = [
  { symbol: 'BTC-PERP', name: 'Bitcoin Perpetual', price: 68500, change: 2.5, volume: '1.2B', long: 65, short: 35, icon: '₿' },
  { symbol: 'ETH-PERP', name: 'Ethereum Perpetual', price: 3450, change: 1.8, volume: '890M', long: 58, short: 42, icon: 'Ξ' },
  { symbol: 'SOL-PERP', name: 'Solana Perpetual', price: 145, change: -1.2, volume: '450M', long: 45, short: 55, icon: '☀️' },
  { symbol: 'ARB-PERP', name: 'Arbitrum Perpetual', price: 1.85, change: 3.2, volume: '120M', long: 70, short: 30, icon: '🔵' },
  { symbol: 'AVAX-PERP', name: 'Avalanche Perpetual', price: 38.5, change: 0.8, volume: '95M', long: 52, short: 48, icon: '🔺' },
];

const LEVERAGE_OPTIONS = [1, 2, 3, 5, 10, 20, 50];

export function PerpetualTrading() {
  const [selectedPair, setSelectedPair] = useState(PERPETUAL_PAIRS[0]);
  const [leverage, setLeverage] = useState(10);
  const [orderType, setOrderType] = useState<'limit' | 'market'>('limit');
  const [positionSide, setPositionSide] = useState<'long' | 'short'>('long');
  const [amount, setAmount] = useState('');
  const [limitPrice, setLimitPrice] = useState('');

  const estimatedCollateral = amount ? (parseFloat(amount) * selectedPair.price / leverage).toFixed(2) : '0.00';
  const liquidationPrice = positionSide === 'long'
    ? (selectedPair.price * (1 - 0.5 / leverage)).toFixed(2)
    : (selectedPair.price * (1 + 0.5 / leverage)).toFixed(2);

  return (
    <div className="glass-dark rounded-2xl p-6">
      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <div className="flex items-center space-x-3">
          <h2 className="text-xl font-display font-bold text-white">Perpetual Trading</h2>
          <span className="px-2 py-0.5 bg-blue-500/20 text-blue-400 text-xs rounded-full">Up to 50x</span>
        </div>
        <button className="p-2 text-gray-400 hover:text-white transition-colors">
          <BarChart3 className="w-4 h-4" />
        </button>
      </div>

      {/* Market Ticker */}
      <div className="mb-6 overflow-x-auto">
        <div className="flex space-x-2">
          {PERPETUAL_PAIRS.map((pair) => (
            <button
              key={pair.symbol}
              onClick={() => setSelectedPair(pair)}
              className={`flex-shrink-0 flex items-center space-x-2 px-4 py-2 rounded-xl transition-colors ${
                selectedPair.symbol === pair.symbol
                  ? 'bg-tiger-orange/20 border border-tiger-orange/30'
                  : 'bg-white/5 hover:bg-white/10'
              }`}
            >
              <span className="text-lg">{pair.icon}</span>
              <div className="text-left">
                <div className="text-white text-sm font-medium">{pair.symbol}</div>
                <div className={`text-xs ${pair.change >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                  {pair.change >= 0 ? '+' : ''}{pair.change}%
                </div>
              </div>
            </button>
          ))}
        </div>
      </div>

      {/* Price Chart Placeholder */}
      <div className="h-48 bg-white/5 rounded-xl mb-6 flex items-center justify-center relative overflow-hidden">
        <div className="absolute inset-0 flex items-center justify-center">
          <BarChart3 className="w-16 h-16 text-gray-700" />
        </div>
        <div className="absolute bottom-4 left-4 flex space-x-4 text-xs">
          <span className="text-gray-500">1H</span>
          <span className="text-gray-400">4H</span>
          <span className="text-tiger-orange">1D</span>
          <span className="text-gray-400">1W</span>
        </div>
      </div>

      {/* Order Form */}
      <div className="space-y-4">
        {/* Position Side */}
        <div className="grid grid-cols-2 gap-2">
          <button
            onClick={() => setPositionSide('long')}
            className={`flex items-center justify-center space-x-2 py-3 rounded-xl font-medium transition-colors ${
              positionSide === 'long'
                ? 'bg-green-500/20 text-green-400 border border-green-500/30'
                : 'bg-white/5 text-gray-400 hover:text-white'
            }`}
          >
            <TrendingUp className="w-4 h-4" />
            <span>Long</span>
          </button>
          <button
            onClick={() => setPositionSide('short')}
            className={`flex items-center justify-center space-x-2 py-3 rounded-xl font-medium transition-colors ${
              positionSide === 'short'
                ? 'bg-red-500/20 text-red-400 border border-red-500/30'
                : 'bg-white/5 text-gray-400 hover:text-white'
            }`}
          >
            <TrendingDown className="w-4 h-4" />
            <span>Short</span>
          </button>
        </div>

        {/* Order Type */}
        <div className="grid grid-cols-2 gap-2">
          <button
            onClick={() => setOrderType('limit')}
            className={`py-2 rounded-lg text-sm font-medium transition-colors ${
              orderType === 'limit'
                ? 'bg-tiger-orange/20 text-tiger-orange'
                : 'text-gray-400 hover:text-white'
            }`}
          >
            Limit
          </button>
          <button
            onClick={() => setOrderType('market')}
            className={`py-2 rounded-lg text-sm font-medium transition-colors ${
              orderType === 'market'
                ? 'bg-tiger-orange/20 text-tiger-orange'
                : 'text-gray-400 hover:text-white'
            }`}
          >
            Market
          </button>
        </div>

        {/* Price Input (for limit orders) */}
        {orderType === 'limit' && (
          <div>
            <label className="block text-sm text-gray-400 mb-2">Limit Price</label>
            <div className="relative">
              <input
                type="number"
                value={limitPrice}
                onChange={(e) => setLimitPrice(e.target.value)}
                placeholder={selectedPair.price.toString()}
                className="w-full px-4 py-3 bg-white/5 border border-white/10 rounded-xl text-white placeholder:text-gray-500 focus:outline-none focus:border-tiger-orange/50"
              />
              <span className="absolute right-4 top-1/2 -translate-y-1/2 text-gray-500">USD</span>
            </div>
          </div>
        )}

        {/* Size Input */}
        <div>
          <label className="block text-sm text-gray-400 mb-2">Size (USD)</label>
          <input
            type="number"
            value={amount}
            onChange={(e) => setAmount(e.target.value)}
            placeholder="0.00"
            className="w-full px-4 py-3 bg-white/5 border border-white/10 rounded-xl text-white placeholder:text-gray-500 focus:outline-none focus:border-tiger-orange/50"
          />
        </div>

        {/* Leverage */}
        <div>
          <div className="flex items-center justify-between mb-2">
            <label className="text-sm text-gray-400">Leverage</label>
            <span className="text-tiger-orange font-medium">{leverage}x</span>
          </div>
          <div className="flex items-center space-x-1">
            {LEVERAGE_OPTIONS.map((lev) => (
              <button
                key={lev}
                onClick={() => setLeverage(lev)}
                className={`flex-1 py-1.5 rounded text-xs font-medium transition-colors ${
                  leverage === lev
                    ? 'bg-tiger-orange/20 text-tiger-orange'
                    : 'bg-white/5 text-gray-400 hover:text-white'
                }`}
              >
                {lev}x
              </button>
            ))}
          </div>
        </div>

        {/* Order Summary */}
        <div className="p-4 bg-white/5 rounded-xl space-y-2">
          <div className="flex items-center justify-between text-sm">
            <span className="text-gray-400">Entry Price</span>
            <span className="text-white">${selectedPair.price.toLocaleString()}</span>
          </div>
          <div className="flex items-center justify-between text-sm">
            <span className="text-gray-400">Est. Collateral</span>
            <span className="text-white">${estimatedCollateral}</span>
          </div>
          <div className="flex items-center justify-between text-sm">
            <span className="text-gray-400">Liquidation Price</span>
            <span className="text-red-400">${liquidationPrice}</span>
          </div>
        </div>

        {/* Warning */}
        <div className="flex items-start space-x-2 p-3 bg-yellow-500/10 rounded-xl border border-yellow-500/20">
          <AlertTriangle className="w-4 h-4 text-yellow-400 flex-shrink-0 mt-0.5" />
          <p className="text-yellow-400 text-xs">
            High leverage increases liquidation risk. Only trade with funds you can afford to lose.
          </p>
        </div>

        {/* Submit Button */}
        <button
          disabled={!amount}
          className={`w-full py-4 rounded-xl font-semibold text-lg transition-all ${
            !amount
              ? 'bg-gray-700 text-gray-500 cursor-not-allowed'
              : positionSide === 'long'
              ? 'bg-green-500 hover:bg-green-600 text-white'
              : 'bg-red-500 hover:bg-red-600 text-white'
          }`}
        >
          {positionSide === 'long' ? 'Open Long' : 'Open Short'} {selectedPair.symbol}
        </button>

        {/* Funding Info */}
        <div className="flex items-center justify-between text-xs text-gray-500">
          <span>Funding Rate</span>
          <span className="text-green-400">0.0100% / 8h</span>
        </div>
      </div>

      {/* Open Positions */}
      <div className="mt-6 pt-6 border-t border-white/10">
        <h3 className="text-sm font-semibold text-white mb-4">Open Positions</h3>
        <div className="text-center py-8 text-gray-500">
          <Layers className="w-8 h-8 mx-auto mb-2 opacity-50" />
          <p>No open positions</p>
        </div>
      </div>
    </div>
  );
}
