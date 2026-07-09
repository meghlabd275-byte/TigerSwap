'use client';

import { useState, useEffect } from 'react';
import { useAccount, useConnect, useDisconnect, useBalance } from 'wagmi';
import { ConnectButton } from '@rainbow-me/rainbowkit';
import { motion } from 'framer-motion';
import { 
  Wallet, 
  Copy,
  ExternalLink,
  Settings,
  KeyRound,
  Users,
  Shield,
  Zap,
  RefreshCw,
  Token,
  Coins,
  History,
  Send,
  Receive,
  WalletCards,
  CheckCircle,
  Loader2,
  ChevronRight
} from 'lucide-react';
import { Address } from 'viem';
import { useWalletStore } from '@/lib/store';
import { SUPPORTED_CHAINS } from '../config';
import Link from 'next/link';

export default function WalletPage() {
  const { address, isConnected, chainId } = useAccount();
  const { data: balance } = useBalance({ address });
  
  const [activeTab, setActiveTab] = useState<'assets' | 'activity' | 'swap'>('assets');
  const [loading, setLoading] = useState(false);
  const [showReceive, setShowReceive] = useState(false);
  const [showSend, setShowSend] = useState(false);
  const [copied, setCopied] = useState(false);

  const { 
    totalUSDValue,
    setNativeBalance,
    setTotalUSDValue,
  } = useWalletStore();

  useEffect(() => {
    if (balance) {
      setNativeBalance(balance.value);
      // Mock USD value calculation
      setTotalUSDValue(parseFloat(balance.formatted) * 2500);
    }
  }, [balance, setNativeBalance, setTotalUSDValue]);

  const copyAddress = () => {
    if (address) {
      navigator.clipboard.writeText(address);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    }
  };

  const currentChain = SUPPORTED_CHAINS.find(c => c.id === chainId) || SUPPORTED_CHAINS[0];

  if (!isConnected) {
    return (
      <div className="min-h-screen bg-tiger-dark flex items-center justify-center p-4">
        <div className="text-center">
          <WalletCards className="w-20 h-20 text-tiger-orange mx-auto mb-6" />
          <h1 className="text-3xl font-bold text-white mb-4">Connect Your Wallet</h1>
          <p className="text-gray-400 mb-8 max-w-md">
            Connect your wallet to view your assets, make transactions, and access all features of TigerWallet.
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
                  <Wallet className="w-5 h-5 text-black" />
                </div>
                <span className="text-lg font-bold text-white">TigerWallet</span>
              </Link>
            </div>
            
            <div className="flex items-center space-x-4">
              <button className="p-2 text-gray-400 hover:text-white transition-colors">
                <RefreshCw className="w-5 h-5" />
              </button>
              <Link href="/settings" className="p-2 text-gray-400 hover:text-white transition-colors">
                <Settings className="w-5 h-5" />
              </Link>
              <ConnectButton showBalance={false} />
            </div>
          </div>
        </div>
      </header>

      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
        {/* Wallet Card */}
        <motion.div 
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          className="relative rounded-3xl overflow-hidden mb-8"
        >
          <div className="absolute inset-0 bg-gradient-to-br from-tiger-orange/20 via-tiger-dark to-tiger-dark" />
          <div className="absolute inset-0 bg-[url('data:image/svg+xml;base64,PHN2ZyB3aWR0aD0iNjAiIGhlaWdodD0iNjAiIHZpZXdCb3g9IjAgMCA2MCA2MCIgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvc3ZnIj48ZyBmaWxsPSJub25lIiBmaWxsLXJ1bGU9ImV2ZW5vZGQiPjxwYXRoIGQ9Ik0zNiAxOGMtOS45NDEgMC0xOCA4LjA1OS0xOCAxOHM4LjA1OSAxOCAxOCAxOCAxOC04LjA1OSAxOC0xOC04LjA1OS0xOC0xOC0xOHptMCAzMmMtNy43MzIgMC0xNC02LjI2OC0xNC0xNHM2LjI2OC0xNCAxNC0xNCAxNCA2LjI2OCAxNCAxNC02LjI2OCAxNC0xNCAxNHoiIGZpbGw9IiNmZmYiIGZpbGwtb3BhY2l0eT0iMC4wNSIvPjwvZz48L3N2Zz4=')] opacity-30" />
          
          <div className="relative p-8">
            <div className="flex items-start justify-between mb-8">
              <div>
                <div className="flex items-center space-x-2 mb-2">
                  <Shield className="w-5 h-5 text-tiger-orange" />
                  <span className="text-sm text-tiger-orange font-medium">Smart Account</span>
                </div>
                <div className="flex items-center space-x-3">
                  <h1 className="text-2xl font-bold text-white font-mono">
                    {address ? `${address.slice(0, 6)}...${address.slice(-4)}` : ''}
                  </h1>
                  <button 
                    onClick={copyAddress}
                    className="p-1.5 rounded-lg bg-white/10 hover:bg-white/20 transition-colors"
                  >
                    {copied ? (
                      <CheckCircle className="w-4 h-4 text-green-400" />
                    ) : (
                      <Copy className="w-4 h-4 text-gray-400" />
                    )}
                  </button>
                  <a 
                    href={`${currentChain.blockExplorers?.default?.url}/address/${address}`}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="p-1.5 rounded-lg bg-white/10 hover:bg-white/20 transition-colors"
                  >
                    <ExternalLink className="w-4 h-4 text-gray-400" />
                  </a>
                </div>
              </div>
              
              <div className="flex items-center space-x-2">
                <select className="bg-white/10 border border-white/10 rounded-lg px-3 py-2 text-white text-sm">
                  {SUPPORTED_CHAINS.map(chain => (
                    <option key={chain.id} value={chain.id}>{chain.name}</option>
                  ))}
                </select>
              </div>
            </div>

            <div className="mb-6">
              <p className="text-sm text-gray-400 mb-1">Total Balance</p>
              <p className="text-4xl font-bold text-white">
                ${totalUSDValue.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}
              </p>
            </div>

            <div className="flex items-center space-x-3">
              <button 
                onClick={() => setShowSend(true)}
                className="flex-1 py-3 px-6 rounded-xl bg-tiger-orange hover:bg-tiger-orange/90 text-white font-semibold flex items-center justify-center transition-colors"
              >
                <Send className="w-5 h-5 mr-2" />
                Send
              </button>
              <button 
                onClick={() => setShowReceive(true)}
                className="flex-1 py-3 px-6 rounded-xl bg-white/10 hover:bg-white/20 text-white font-semibold flex items-center justify-center transition-colors"
              >
                <Receive className="w-5 h-5 mr-2" />
                Receive
              </button>
              <Link 
                href="/swap"
                className="flex-1 py-3 px-6 rounded-xl bg-white/10 hover:bg-white/20 text-white font-semibold flex items-center justify-center transition-colors"
              >
                <RefreshCw className="w-5 h-5 mr-2" />
                Swap
              </Link>
            </div>
          </div>
        </motion.div>

        {/* Quick Actions */}
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-8">
          <Link href="/wallet/smart-account" className="p-4 rounded-2xl bg-white/5 border border-white/5 hover:border-tiger-orange/50 transition-colors">
            <KeyRound className="w-8 h-8 text-tiger-orange mb-3" />
            <h3 className="font-semibold text-white">Smart Account</h3>
            <p className="text-sm text-gray-400">ERC-4337</p>
          </Link>
          <Link href="/wallet/mpc" className="p-4 rounded-2xl bg-white/5 border border-white/5 hover:border-purple-500/50 transition-colors">
            <Users className="w-8 h-8 text-purple-500 mb-3" />
            <h3 className="font-semibold text-white">MPC Wallet</h3>
            <p className="text-sm text-gray-400">Key Sharding</p>
          </Link>
          <Link href="/bridge" className="p-4 rounded-2xl bg-white/5 border border-white/5 hover:border-cyan-500/50 transition-colors">
            <Zap className="w-8 h-8 text-cyan-500 mb-3" />
            <h3 className="font-semibold text-white">Bridge</h3>
            <p className="text-sm text-gray-400">Cross-Chain</p>
          </Link>
          <Link href="/governance" className="p-4 rounded-2xl bg-white/5 border border-white/5 hover:border-blue-500/50 transition-colors">
            <Shield className="w-8 h-8 text-blue-500 mb-3" />
            <h3 className="font-semibold text-white">Governance</h3>
            <p className="text-sm text-gray-400">DAO</p>
          </Link>
        </div>

        {/* Tabs */}
        <div className="flex items-center space-x-1 mb-6 bg-white/5 rounded-xl p-1">
          {(['assets', 'activity', 'swap'] as const).map((tab) => (
            <button
              key={tab}
              onClick={() => setActiveTab(tab)}
              className={`flex-1 py-2.5 px-4 rounded-lg text-sm font-medium transition-colors ${
                activeTab === tab 
                  ? 'bg-white/10 text-white' 
                  : 'text-gray-400 hover:text-white'
              }`}
            >
              {tab.charAt(0).toUpperCase() + tab.slice(1)}
            </button>
          ))}
        </div>

        {/* Content */}
        {activeTab === 'assets' && (
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            className="space-y-3"
          >
            {/* Native Token */}
            <div className="p-4 rounded-2xl bg-white/5 border border-white/5 hover:border-white/10 transition-colors">
              <div className="flex items-center justify-between">
                <div className="flex items-center space-x-3">
                  <div className="w-12 h-12 rounded-full bg-gradient-to-br from-blue-500 to-cyan-500 flex items-center justify-center">
                    <Coins className="w-6 h-6 text-white" />
                  </div>
                  <div>
                    <h3 className="font-semibold text-white">{currentChain.nativeCurrency?.symbol || 'ETH'}</h3>
                    <p className="text-sm text-gray-400">{currentChain.name}</p>
                  </div>
                </div>
                <div className="text-right">
                  <p className="font-semibold text-white">
                    {balance ? parseFloat(balance.formatted).toFixed(4) : '0.0000'}
                  </p>
                  <p className="text-sm text-gray-400">
                    ${(parseFloat(balance?.formatted || '0') * 2500).toFixed(2)}
                  </p>
                </div>
              </div>
            </div>

            <div className="p-8 text-center text-gray-500">
              <Token className="w-12 h-12 mx-auto mb-4 opacity-50" />
              <p>No other tokens found</p>
              <Link href="/swap" className="text-tiger-orange hover:underline mt-2 inline-block">
                Get tokens by swapping
              </Link>
            </div>
          </motion.div>
        )}

        {activeTab === 'activity' && (
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            className="space-y-3"
          >
            <div className="p-8 text-center text-gray-500">
              <History className="w-12 h-12 mx-auto mb-4 opacity-50" />
              <p>No transaction history yet</p>
            </div>
          </motion.div>
        )}

        {activeTab === 'swap' && (
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
          >
            <Link href="/swap" className="block p-6 rounded-2xl bg-gradient-to-r from-tiger-orange/20 to-tiger-yellow/20 border border-tiger-orange/20 hover:border-tiger-orange/40 transition-colors">
              <div className="flex items-center justify-between">
                <div>
                  <h3 className="text-lg font-semibold text-white mb-1">Swap Tokens</h3>
                  <p className="text-sm text-gray-400">Exchange tokens at the best rates</p>
                </div>
                <ChevronRight className="w-6 h-6 text-tiger-orange" />
              </div>
            </Link>
          </motion.div>
        )}
      </div>

      {/* Send Modal */}
      {showSend && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
          <div className="absolute inset-0 bg-black/60 backdrop-blur-sm" onClick={() => setShowSend(false)} />
          <motion.div 
            initial={{ opacity: 0, scale: 0.95 }}
            animate={{ opacity: 1, scale: 1 }}
            className="relative w-full max-w-md p-6 rounded-2xl bg-tiger-dark border border-white/10"
          >
            <h2 className="text-xl font-bold text-white mb-6">Send Tokens</h2>
            <div className="space-y-4">
              <div>
                <label className="block text-sm text-gray-400 mb-2">Recipient Address</label>
                <input 
                  type="text" 
                  placeholder="0x..."
                  className="w-full px-4 py-3 rounded-xl bg-white/5 border border-white/10 text-white placeholder-gray-500 focus:outline-none focus:border-tiger-orange"
                />
              </div>
              <div>
                <label className="block text-sm text-gray-400 mb-2">Amount</label>
                <input 
                  type="number" 
                  placeholder="0.00"
                  className="w-full px-4 py-3 rounded-xl bg-white/5 border border-white/10 text-white placeholder-gray-500 focus:outline-none focus:border-tiger-orange"
                />
              </div>
              <button className="w-full py-3 px-6 rounded-xl bg-tiger-orange hover:bg-tiger-orange/90 text-white font-semibold">
                Send
              </button>
            </div>
          </motion.div>
        </div>
      )}

      {/* Receive Modal */}
      {showReceive && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
          <div className="absolute inset-0 bg-black/60 backdrop-blur-sm" onClick={() => setShowReceive(false)} />
          <motion.div 
            initial={{ opacity: 0, scale: 0.95 }}
            animate={{ opacity: 1, scale: 1 }}
            className="relative w-full max-w-md p-6 rounded-2xl bg-tiger-dark border border-white/10"
          >
            <h2 className="text-xl font-bold text-white mb-6">Receive Tokens</h2>
            <div className="text-center">
              <div className="w-48 h-48 mx-auto bg-white rounded-2xl p-4 mb-6">
                <div className="w-full h-full bg-black rounded-xl flex items-center justify-center">
                  <Wallet className="w-16 h-16 text-white" />
                </div>
              </div>
              <p className="text-sm text-gray-400 mb-2">Scan to receive</p>
              <p className="font-mono text-white bg-white/5 px-4 py-2 rounded-lg break-all">
                {address}
              </p>
              <button 
                onClick={copyAddress}
                className="mt-4 text-tiger-orange hover:underline"
              >
                Copy Address
              </button>
            </div>
          </motion.div>
        </div>
      )}
    </div>
  );
}
