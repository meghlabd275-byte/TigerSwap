'use client';

import { useState } from 'react';
import { useAccount, useBalance } from 'wagmi';
import { ConnectButton } from '@rainbow-me/rainbowkit';
import { motion } from 'framer-motion';
import { 
  Layers, 
  ArrowRight, 
  Clock, 
  Zap,
  Shield,
  Wallet,
  ChevronDown,
  ExternalLink,
  RefreshCw
} from 'lucide-react';
import Link from 'next/link';

const CHAINS = [
  { id: 1, name: 'Ethereum', symbol: 'ETH', logo: 'https://cryptologos.cc/logos/ethereum-eth-logo.png', color: '#627EEA' },
  { id: 10, name: 'Optimism', symbol: 'ETH', logo: 'https://cryptologos.cc/logos/optimism-optimism-logo.png', color: '#FF0420' },
  { id: 42161, name: 'Arbitrum', symbol: 'ETH', logo: 'https://cryptologos.cc/logos/arbitrum-arb-logo.png', color: '#28A0F0' },
  { id: 8453, name: 'Base', symbol: 'ETH', logo: 'https://cryptologos.cc/logos/base-logo.png', color: '#0052FF' },
  { id: 137, name: 'Polygon', symbol: 'MATIC', logo: 'https://cryptologos.cc/logos/polygon-matic-logo.png', color: '#8247E5' },
  { id: 43114, name: 'Avalanche', symbol: 'AVAX', logo: 'https://cryptologos.cc/logos/avalanche-avax-logo.png', color: '#E84142' },
  { id: 56, name: 'BNB Chain', symbol: 'BNB', logo: 'https://cryptologos.cc/logos/bnb-bnb-logo.png', color: '#F3BA2F' },
];

const BRIDGES = [
  { name: 'Stargate', logo: 'https://cryptologos.cc/logos/stargate-stg-logo.png', fee: '0.06%', time: '3-5 min' },
  { name: 'Across', logo: 'https://cryptologos.cc/logos/across-logo.png', fee: '0.04%', time: '2-5 min' },
  { name: 'Hop', logo: 'https://cryptologos.cc/logos/hop-protocol-logo.png', fee: '0.05%', time: '5-30 min' },
];

export default function BridgePage() {
  const { address, isConnected } = useAccount();
  const { data: balance } = useBalance({ address });
  
  const [fromChain, setFromChain] = useState(CHAINS[0]);
  const [toChain, setToChain] = useState(CHAINS[2]);
  const [amount, setAmount] = useState('');
  const [isLoading, setIsLoading] = useState(false);

  const handleBridge = async () => {
    if (!amount) return;
    setIsLoading(true);
    
    try {
      await new Promise(resolve => setTimeout(resolve, 2000));
      setAmount('');
    } catch (error) {
      console.error('Bridge failed:', error);
    } finally {
      setIsLoading(false);
    }
  };

  if (!isConnected) {
    return (
      <div className="min-h-screen bg-tiger-dark flex items-center justify-center p-4">
        <div className="text-center">
          <Layers className="w-20 h-20 text-tiger-orange mx-auto mb-6" />
          <h1 className="text-3xl font-bold text-white mb-4">Connect to Bridge</h1>
          <p className="text-gray-400 mb-8 max-w-md">
            Connect your wallet to bridge assets across multiple chains with the best rates.
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
                  <Layers className="w-5 h-5 text-black" />
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
          <h1 className="text-3xl font-bold text-white mb-2">Bridge</h1>
          <p className="text-gray-400">Transfer assets across chains</p>
        </div>

        {/* Bridge Card */}
        <motion.div 
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          className="relative"
        >
          {/* From Chain */}
          <div className="p-6 rounded-2xl bg-white/5 border border-white/10 mb-2">
            <div className="flex items-center justify-between mb-4">
              <span className="text-sm text-gray-400">From</span>
              <span className="text-sm text-gray-400">
                Balance: {balance ? parseFloat(balance.formatted).toFixed(4) : '0'} {fromChain.symbol}
              </span>
            </div>
            
            <div className="flex items-center space-x-4">
              <div className="flex-1">
                <input
                  type="number"
                  placeholder="0.00"
                  value={amount}
                  onChange={(e) => setAmount(e.target.value)}
                  className="w-full bg-transparent text-3xl font-bold text-white placeholder-gray-600 focus:outline-none"
                />
              </div>
              
              <button className="flex items-center space-x-2 px-4 py-2 rounded-xl bg-white/10 hover:bg-white/20 transition-colors">
                <img src={fromChain.logo} alt={fromChain.name} className="w-6 h-6 rounded-full" />
                <span className="text-white font-medium">{fromChain.name}</span>
                <ChevronDown className="w-4 h-4 text-gray-400" />
              </button>
            </div>
          </div>

          {/* Switch Button */}
          <div className="flex justify-center -my-4 relative z-10">
            <button
              onClick={() => {
                const temp = fromChain;
                setFromChain(toChain);
                setToChain(temp);
              }}
              className="p-3 rounded-full bg-tiger-orange border-4 border-tiger-dark hover:bg-tiger-orange/80 transition-colors"
            >
              <ArrowRight className="w-5 h-5 text-white" />
            </button>
          </div>

          {/* To Chain */}
          <div className="p-6 rounded-2xl bg-white/5 border border-white/10">
            <div className="flex items-center justify-between mb-4">
              <span className="text-sm text-gray-400">To</span>
            </div>
            
            <div className="flex items-center space-x-4">
              <div className="flex-1">
                <input
                  type="number"
                  placeholder="0.00"
                  value={amount}
                  readOnly
                  className="w-full bg-transparent text-3xl font-bold text-white placeholder-gray-600 focus:outline-none"
                />
              </div>
              
              <button className="flex items-center space-x-2 px-4 py-2 rounded-xl bg-white/10 hover:bg-white/20 transition-colors">
                <img src={toChain.logo} alt={toChain.name} className="w-6 h-6 rounded-full" />
                <span className="text-white font-medium">{toChain.name}</span>
                <ChevronDown className="w-4 h-4 text-gray-400" />
              </button>
            </div>
          </div>

          {/* Bridge Details */}
          <div className="mt-4 p-4 rounded-2xl bg-white/5 border border-white/10 space-y-3">
            <div className="flex items-center justify-between text-sm">
              <span className="text-gray-400">Estimated Time</span>
              <span className="text-white flex items-center">
                <Clock className="w-4 h-4 mr-1" />
                3-5 minutes
              </span>
            </div>
            <div className="flex items-center justify-between text-sm">
              <span className="text-gray-400">Bridge Fee</span>
              <span className="text-white">0.06%</span>
            </div>
            <div className="flex items-center justify-between text-sm">
              <span className="text-gray-400">You Receive</span>
              <span className="text-white">{amount || '0.00'} {toChain.symbol}</span>
            </div>
          </div>

          {/* Bridge Button */}
          <button
            onClick={handleBridge}
            disabled={!amount || isLoading}
            className="w-full mt-4 py-4 rounded-xl bg-tiger-orange hover:bg-tiger-orange/90 disabled:opacity-50 disabled:cursor-not-allowed text-white font-semibold flex items-center justify-center transition-colors"
          >
            {isLoading ? (
              <RefreshCw className="w-5 h-5 animate-spin" />
            ) : (
              <>
                <Zap className="w-5 h-5 mr-2" />
                Bridge Now
              </>
            )}
          </button>
        </motion.div>

        {/* Best Bridges */}
        <div className="mt-8">
          <h3 className="text-lg font-semibold text-white mb-4">Best Bridges</h3>
          <div className="space-y-2">
            {BRIDGES.map((bridge, index) => (
              <div 
                key={index}
                className="flex items-center justify-between p-4 rounded-xl bg-white/5 border border-white/5 hover:border-white/10 transition-colors"
              >
                <div className="flex items-center space-x-3">
                  <img src={bridge.logo} alt={bridge.name} className="w-8 h-8 rounded-full" />
                  <span className="text-white font-medium">{bridge.name}</span>
                </div>
                <div className="text-right">
                  <p className="text-white font-medium">{amount || '0'} {toChain.symbol}</p>
                  <p className="text-sm text-gray-400">Fee: {bridge.fee} • {bridge.time}</p>
                </div>
              </div>
            ))}
          </div>
        </div>

        {/* Supported Chains */}
        <div className="mt-8">
          <h3 className="text-lg font-semibold text-white mb-4">Supported Chains</h3>
          <div className="flex flex-wrap gap-2">
            {CHAINS.map((chain) => (
              <div 
                key={chain.id}
                className="flex items-center space-x-2 px-3 py-2 rounded-xl bg-white/5 border border-white/5"
              >
                <img src={chain.logo} alt={chain.name} className="w-5 h-5 rounded-full" />
                <span className="text-white text-sm">{chain.name}</span>
              </div>
            ))}
          </div>
        </div>

        {/* Security Note */}
        <div className="mt-8 p-4 rounded-xl bg-green-500/10 border border-green-500/20">
          <div className="flex items-start space-x-3">
            <Shield className="w-5 h-5 text-green-400 flex-shrink-0 mt-0.5" />
            <div>
              <p className="text-green-400 font-medium text-sm">Security Verified</p>
              <p className="text-gray-400 text-sm">All bridges are audited and verified for security</p>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
