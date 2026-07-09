'use client';

import { useState } from 'react';
import { useAccount, useBalance, useReadContract, useWriteContract } from 'wagmi';
import { ConnectButton } from '@rainbow-me/rainbowkit';
import { motion } from 'framer-motion';
import { 
  RefreshCw, 
  ArrowDown, 
  Settings, 
  Zap,
  Wallet,
  ExternalLink,
  ChevronDown,
  Info,
  TrendingUp,
  Clock,
  ArrowRight
} from 'lucide-react';
import { Address, parseEther, parseUnits, formatEther } from 'viem';
import Link from 'next/link';

const POPULAR_TOKENS = [
  { symbol: 'ETH', name: 'Ethereum', decimals: 18, logo: 'https://cryptologos.cc/logos/ethereum-eth-logo.png' },
  { symbol: 'USDC', name: 'USD Coin', decimals: 6, logo: 'https://cryptologos.cc/logos/usd-coin-usdc-logo.png' },
  { symbol: 'USDT', name: 'Tether', decimals: 6, logo: 'https://cryptologos.cc/logos/tether-usdt-logo.png' },
  { symbol: 'WBTC', name: 'Wrapped Bitcoin', decimals: 8, logo: 'https://cryptologos.cc/logos/wrapped-bitcoin-wbtc-logo.png' },
  { symbol: 'DAI', name: 'Dai', decimals: 18, logo: 'https://cryptologos.cc/logos/dai-dai-logo.png' },
];

const DEXS = [
  { name: 'Uniswap', logo: 'https://cryptologos.cc/logos/uniswap-uni-logo.png', fee: '0.3%' },
  { name: 'SushiSwap', logo: 'https://cryptologos.cc/logos/sushi-sushi-logo.png', fee: '0.3%' },
  { name: 'Curve', logo: 'https://cryptologos.cc/logos/curve-dao-token-crv-logo.png', fee: '0.04%' },
];

export default function SwapPage() {
  const { address, isConnected } = useAccount();
  const { data: balance } = useBalance({ address });
  
  const [fromToken, setFromToken] = useState(POPULAR_TOKENS[0]);
  const [toToken, setToToken] = useState(POPULAR_TOKENS[1]);
  const [fromAmount, setFromAmount] = useState('');
  const [toAmount, setToAmount] = useState('');
  const [slippage, setSlippage] = useState(0.5);
  const [isLoading, setIsLoading] = useState(false);
  const [showSettings, setShowSettings] = useState(false);

  const handleSwap = async () => {
    if (!fromAmount || !toAmount) return;
    setIsLoading(true);
    
    try {
      // Simulate swap
      await new Promise(resolve => setTimeout(resolve, 2000));
      setFromAmount('');
      setToAmount('');
    } catch (error) {
      console.error('Swap failed:', error);
    } finally {
      setIsLoading(false);
    }
  };

  const handleSwitchTokens = () => {
    setFromToken(toToken);
    setToToken(fromToken);
    setFromAmount(toAmount);
    setToAmount(fromAmount);
  };

  const mockToAmount = fromAmount ? (parseFloat(fromAmount) * 2500 * 0.998).toFixed(6) : '';

  if (!isConnected) {
    return (
      <div className="min-h-screen bg-tiger-dark flex items-center justify-center p-4">
        <div className="text-center">
          <RefreshCw className="w-20 h-20 text-tiger-orange mx-auto mb-6" />
          <h1 className="text-3xl font-bold text-white mb-4">Connect to Swap</h1>
          <p className="text-gray-400 mb-8 max-w-md">
            Connect your wallet to swap tokens at the best rates across multiple DEXs.
          </p>
          <ConnectButton />
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-tiger-dark">
      {/* Header */}
      <header className="sticky top-0 z-50 glass-dark border-b border-white/5">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
          <div className="flex items-center justify-between h-16">
            <div className="flex items-center space-x-3">
              <Link href="/" className="flex items-center space-x-3">
                <div className="w-8 h-8 rounded-lg bg-gradient-to-br from-tiger-orange to-tiger-yellow flex items-center justify-center">
                  <RefreshCw className="w-5 h-5 text-black" />
                </div>
                <span className="text-lg font-bold text-white">TigerWallet</span>
              </Link>
            </div>
            <ConnectButton showBalance={false} />
          </div>
        </div>
      </header>

      <div className="max-w-2xl mx-auto px-4 py-12">
        {/* Title */}
        <div className="text-center mb-8">
          <h1 className="text-3xl font-bold text-white mb-2">Swap</h1>
          <p className="text-gray-400">Exchange tokens at the best rates</p>
        </div>

        {/* Swap Card */}
        <motion.div 
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          className="relative"
        >
          {/* From Token */}
          <div className="p-6 rounded-2xl bg-white/5 border border-white/10 mb-2">
            <div className="flex items-center justify-between mb-4">
              <span className="text-sm text-gray-400">From</span>
              <span className="text-sm text-gray-400">
                Balance: {balance ? parseFloat(balance.formatted).toFixed(4) : '0'} {fromToken.symbol}
              </span>
            </div>
            
            <div className="flex items-center space-x-4">
              <div className="flex-1">
                <input
                  type="number"
                  placeholder="0.00"
                  value={fromAmount}
                  onChange={(e) => {
                    setFromAmount(e.target.value);
                    setToAmount(mockToAmount);
                  }}
                  className="w-full bg-transparent text-3xl font-bold text-white placeholder-gray-600 focus:outline-none"
                />
              </div>
              
              <button className="flex items-center space-x-2 px-4 py-2 rounded-xl bg-white/10 hover:bg-white/20 transition-colors">
                <img src={fromToken.logo} alt={fromToken.symbol} className="w-6 h-6 rounded-full" />
                <span className="text-white font-medium">{fromToken.symbol}</span>
                <ChevronDown className="w-4 h-4 text-gray-400" />
              </button>
            </div>
          </div>

          {/* Swap Button */}
          <div className="flex justify-center -my-4 relative z-10">
            <button
              onClick={handleSwitchTokens}
              className="p-3 rounded-full bg-tiger-orange border-4 border-tiger-dark hover:bg-tiger-orange/80 transition-colors"
            >
              <ArrowDown className="w-5 h-5 text-white" />
            </button>
          </div>

          {/* To Token */}
          <div className="p-6 rounded-2xl bg-white/5 border border-white/10">
            <div className="flex items-center justify-between mb-4">
              <span className="text-sm text-gray-400">To</span>
              <span className="text-sm text-gray-400">
                Balance: 0.0000 {toToken.symbol}
              </span>
            </div>
            
            <div className="flex items-center space-x-4">
              <div className="flex-1">
                <input
                  type="number"
                  placeholder="0.00"
                  value={toAmount}
                  readOnly
                  className="w-full bg-transparent text-3xl font-bold text-white placeholder-gray-600 focus:outline-none"
                />
              </div>
              
              <button className="flex items-center space-x-2 px-4 py-2 rounded-xl bg-white/10 hover:bg-white/20 transition-colors">
                <img src={toToken.logo} alt={toToken.symbol} className="w-6 h-6 rounded-full" />
                <span className="text-white font-medium">{toToken.symbol}</span>
                <ChevronDown className="w-4 h-4 text-gray-400" />
              </button>
            </div>
          </div>

          {/* Swap Details */}
          <div className="mt-4 p-4 rounded-2xl bg-white/5 border border-white/10 space-y-3">
            <div className="flex items-center justify-between text-sm">
              <span className="text-gray-400">Rate</span>
              <span className="text-white">
                1 {fromToken.symbol} = 2,499.5 {toToken.symbol}
              </span>
            </div>
            <div className="flex items-center justify-between text-sm">
              <span className="text-gray-400">Price Impact</span>
              <span className="text-green-400">0.02%</span>
            </div>
            <div className="flex items-center justify-between text-sm">
              <span className="text-gray-400">Liquidity Provider Fee</span>
              <span className="text-white">0.003 ETH</span>
            </div>
            <div className="flex items-center justify-between text-sm">
              <span className="text-gray-400">Slippage Tolerance</span>
              <button 
                onClick={() => setShowSettings(!showSettings)}
                className="text-tiger-orange"
              >
                {slippage}%
              </button>
            </div>
            
            {showSettings && (
              <div className="pt-3 border-t border-white/10">
                <div className="flex items-center justify-between text-sm mb-2">
                  <span className="text-gray-400">Custom Slippage</span>
                  <div className="flex space-x-2">
                    {[0.1, 0.5, 1.0].map((v) => (
                      <button
                        key={v}
                        onClick={() => setSlippage(v)}
                        className={`px-3 py-1 rounded-lg text-xs ${
                          slippage === v 
                            ? 'bg-tiger-orange text-white' 
                            : 'bg-white/10 text-gray-400'
                        }`}
                      >
                        {v}%
                      </button>
                    ))}
                  </div>
                </div>
              </div>
            )}
          </div>

          {/* Swap Button */}
          <button
            onClick={handleSwap}
            disabled={!fromAmount || !toAmount || isLoading}
            className="w-full mt-4 py-4 rounded-xl bg-tiger-orange hover:bg-tiger-orange/90 disabled:opacity-50 disabled:cursor-not-allowed text-white font-semibold flex items-center justify-center transition-colors"
          >
            {isLoading ? (
              <RefreshCw className="w-5 h-5 animate-spin" />
            ) : (
              <>
                <Zap className="w-5 h-5 mr-2" />
                Swap
              </>
            )}
          </button>
        </motion.div>

        {/* Best Rates */}
        <div className="mt-8">
          <h3 className="text-lg font-semibold text-white mb-4">Best Rates From</h3>
          <div className="space-y-2">
            {DEXS.map((dex, index) => (
              <div 
                key={index}
                className="flex items-center justify-between p-4 rounded-xl bg-white/5 border border-white/5 hover:border-white/10 transition-colors"
              >
                <div className="flex items-center space-x-3">
                  <img src={dex.logo} alt={dex.name} className="w-8 h-8 rounded-full" />
                  <span className="text-white font-medium">{dex.name}</span>
                </div>
                <div className="text-right">
                  <p className="text-white font-medium">2,499.5 USDC</p>
                  <p className="text-sm text-gray-400">Fee: {dex.fee}</p>
                </div>
              </div>
            ))}
          </div>
        </div>

        {/* Recent Swaps */}
        <div className="mt-8">
          <h3 className="text-lg font-semibold text-white mb-4">Recent Activity</h3>
          <div className="text-center py-8 text-gray-500">
            <Clock className="w-12 h-12 mx-auto mb-4 opacity-50" />
            <p>No recent swaps</p>
          </div>
        </div>
      </div>
    </div>
  );
}
