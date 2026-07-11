'use client';

import { useState } from 'react';
import { motion } from 'framer-motion';
import { 
  Globe, 
  ArrowRight, 
  Clock, 
  Zap, 
  Shield,
  Settings,
  ExternalLink,
  ArrowDown
} from 'lucide-react';
import { SUPPORTED_CHAINS, Chain } from '@/store/useStore';

const BRIDGE_ROUTES = [
  { from: 'ethereum', to: 'arbitrum', time: '10-30 min', fee: '0.02%', icon: '🌉' },
  { from: 'ethereum', to: 'optimism', time: '10-30 min', fee: '0.02%', icon: '🌉' },
  { from: 'ethereum', to: 'polygon', time: '5-15 min', fee: '0.01%', icon: '🌉' },
  { from: 'ethereum', to: 'bsc', time: '15-45 min', fee: '0.03%', icon: '🌉' },
  { from: 'ethereum', to: 'avalanche', time: '15-45 min', fee: '0.03%', icon: '🌉' },
  { from: 'solana', to: 'ethereum', time: '20-60 min', fee: '0.05%', icon: '🌉' },
];

export function CrossChain() {
  const [fromChain, setFromChain] = useState<string>('ethereum');
  const [toChain, setToChain] = useState<string>('arbitrum');
  const [amount, setAmount] = useState('');
  const [isSwapping, setIsSwapping] = useState(false);
  const [showFromSelector, setShowFromSelector] = useState(false);
  const [showToSelector, setShowToSelector] = useState(false);

  const chainKeys = Object.keys(SUPPORTED_CHAINS);

  const handleSwitchChains = () => {
    const temp = fromChain;
    setFromChain(toChain);
    setToChain(temp);
  };

  return (
    <div className="glass-dark rounded-2xl p-6">
      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <div className="flex items-center space-x-3">
          <Globe className="w-5 h-5 text-tiger-orange" />
          <h2 className="text-xl font-display font-bold text-white">Cross-Chain Bridge</h2>
        </div>
        <button className="p-2 text-gray-400 hover:text-white transition-colors">
          <Settings className="w-4 h-4" />
        </button>
      </div>

      {/* Chain Selection */}
      <div className="space-y-4">
        {/* From Chain */}
        <div className="bg-white/5 rounded-xl p-4 border border-white/10">
          <label className="block text-sm text-gray-400 mb-3">From</label>
          <button
            onClick={() => setShowFromSelector(!showFromSelector)}
            className="w-full flex items-center justify-between p-3 bg-white/5 rounded-lg"
          >
            <div className="flex items-center space-x-3">
              <span className="text-2xl">{SUPPORTED_CHAINS[fromChain]?.icon}</span>
              <div className="text-left">
                <div className="text-white font-medium">{SUPPORTED_CHAINS[fromChain]?.name}</div>
                <div className="text-gray-500 text-xs">{SUPPORTED_CHAINS[fromChain]?.symbol}</div>
              </div>
            </div>
            <ArrowDown className="w-4 h-4 text-gray-400" />
          </button>

          {showFromSelector && (
            <div className="mt-3 max-h-48 overflow-y-auto space-y-1">
              {chainKeys.slice(0, 20).map(key => (
                <button
                  key={key}
                  onClick={() => {
                    setFromChain(key);
                    setShowFromSelector(false);
                  }}
                  className={`w-full flex items-center space-x-3 p-2 rounded-lg transition-colors ${
                    fromChain === key ? 'bg-tiger-orange/20' : 'hover:bg-white/5'
                  }`}
                >
                  <span className="text-xl">{SUPPORTED_CHAINS[key].icon}</span>
                  <span className={fromChain === key ? 'text-tiger-orange' : 'text-white'}>
                    {SUPPORTED_CHAINS[key].name}
                  </span>
                </button>
              ))}
            </div>
          )}
        </div>

        {/* Switch Button */}
        <div className="flex justify-center -my-2 relative z-10">
          <button
            onClick={handleSwitchChains}
            className="p-2 bg-tiger-orange rounded-full border-4 border-tiger-dark hover:bg-tiger-accent transition-colors"
          >
            <ArrowRight className="w-4 h-4 text-white rotate-180" />
          </button>
        </div>

        {/* To Chain */}
        <div className="bg-white/5 rounded-xl p-4 border border-white/10">
          <label className="block text-sm text-gray-400 mb-3">To</label>
          <button
            onClick={() => setShowToSelector(!showToSelector)}
            className="w-full flex items-center justify-between p-3 bg-white/5 rounded-lg"
          >
            <div className="flex items-center space-x-3">
              <span className="text-2xl">{SUPPORTED_CHAINS[toChain]?.icon}</span>
              <div className="text-left">
                <div className="text-white font-medium">{SUPPORTED_CHAINS[toChain]?.name}</div>
                <div className="text-gray-500 text-xs">{SUPPORTED_CHAINS[toChain]?.symbol}</div>
              </div>
            </div>
            <ArrowDown className="w-4 h-4 text-gray-400" />
          </button>

          {showToSelector && (
            <div className="mt-3 max-h-48 overflow-y-auto space-y-1">
              {chainKeys.slice(0, 20).map(key => (
                <button
                  key={key}
                  onClick={() => {
                    setToChain(key);
                    setShowToSelector(false);
                  }}
                  className={`w-full flex items-center space-x-3 p-2 rounded-lg transition-colors ${
                    toChain === key ? 'bg-tiger-orange/20' : 'hover:bg-white/5'
                  }`}
                >
                  <span className="text-xl">{SUPPORTED_CHAINS[key].icon}</span>
                  <span className={toChain === key ? 'text-tiger-orange' : 'text-white'}>
                    {SUPPORTED_CHAINS[key].name}
                  </span>
                </button>
              ))}
            </div>
          )}
        </div>

        {/* Amount Input */}
        <div>
          <label className="block text-sm text-gray-400 mb-2">Amount</label>
          <input
            type="number"
            value={amount}
            onChange={(e) => setAmount(e.target.value)}
            placeholder="0.00"
            className="w-full px-4 py-3 bg-white/5 border border-white/10 rounded-xl text-white placeholder:text-gray-500 focus:outline-none focus:border-tiger-orange/50"
          />
          <div className="flex justify-between mt-2 text-xs">
            <button className="text-gray-500 hover:text-white">Max</button>
            <span className="text-gray-500">Balance: 0.00</span>
          </div>
        </div>

        {/* Route Info */}
        <div className="p-4 bg-white/5 rounded-xl border border-white/10">
          <div className="flex items-center justify-between mb-2">
            <span className="text-gray-400 text-sm">Estimated Time</span>
            <span className="text-white text-sm flex items-center">
              <Clock className="w-3 h-3 mr-1" />10-30 min
            </span>
          </div>
          <div className="flex items-center justify-between mb-2">
            <span className="text-gray-400 text-sm">Bridge Fee</span>
            <span className="text-white text-sm flex items-center">
              <Zap className="w-3 h-3 mr-1" />0.02%
            </span>
          </div>
          <div className="flex items-center justify-between">
            <span className="text-gray-400 text-sm">You Receive</span>
            <span className="text-green-400 text-sm font-medium">
              {(parseFloat(amount || '0') * 0.9998).toFixed(4)} {SUPPORTED_CHAINS[toChain]?.symbol}
            </span>
          </div>
        </div>

        {/* Security Badge */}
        <div className="flex items-center space-x-2 text-sm text-gray-400">
          <Shield className="w-4 h-4 text-green-400" />
          <span>Secured by LayerZero + Stargate</span>
        </div>

        {/* Transfer Button */}
        <button
          disabled={!amount || isSwapping}
          className={`w-full py-4 rounded-xl font-semibold text-lg transition-all ${
            !amount
              ? 'bg-gray-700 text-gray-500 cursor-not-allowed'
              : 'bg-gradient-to-r from-tiger-orange to-tiger-accent text-white btn-glow hover:opacity-90'
          }`}
        >
          {isSwapping ? 'Bridging...' : 'Bridge'}
        </button>

        {/* Supported Bridges */}
        <div className="pt-4 border-t border-white/10">
          <h3 className="text-sm font-semibold text-white mb-3">Supported Bridges</h3>
          <div className="flex flex-wrap gap-2">
            {['LayerZero', 'Stargate', 'Axelar', 'Wormhole', 'Celer'].map(bridge => (
              <span 
                key={bridge}
                className="px-3 py-1 bg-white/5 rounded-full text-xs text-gray-400"
              >
                {bridge}
              </span>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}
