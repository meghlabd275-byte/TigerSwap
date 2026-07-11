'use client';

import { useState, useEffect } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { 
  ArrowRightLeft, 
  TrendingUp, 
  Wallet, 
  Globe, 
  Layers,
  Settings,
  ChevronDown,
  Copy,
  RefreshCw,
  Zap,
  Shield,
  Coins,
  Crosshair,
  History,
  Bell,
  Search,
  Menu,
  X,
  ChevronRight,
  Gauge,
  LineChart,
  Clock
} from 'lucide-react';
import { useStore } from '@/store/useStore';
import { Header } from '@/components/Header';
import { SwapInterface } from '@/components/SwapInterface';
import { TokenList } from '@/components/TokenList';
import { ChainSelector } from '@/components/ChainSelector';
import { Portfolio } from '@/components/Portfolio';
import { PerpetualTrading } from '@/components/PerpetualTrading';
import { CrossChain } from '@/components/CrossChain';

type Tab = 'swap' | 'perpetuals' | 'cross-chain' | 'portfolio';

export default function Home() {
  const [activeTab, setActiveTab] = useState<Tab>('swap');
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false);
  const { 
    connected, 
    address, 
    connectWallet, 
    disconnectWallet,
    selectedChain,
    setSelectedChain 
  } = useStore();

  const tabs = [
    { id: 'swap' as const, label: 'Swap', icon: ArrowRightLeft },
    { id: 'perpetuals' as const, label: 'Perpetuals', icon: TrendingUp },
    { id: 'cross-chain' as const, label: 'Cross-Chain', icon: Globe },
    { id: 'portfolio' as const, label: 'Portfolio', icon: Wallet },
  ];

  return (
    <div className="min-h-screen bg-gradient-to-b from-tiger-dark via-[#0f0f1a] to-[#0a0a12]">
      <Header />
      
      {/* Navigation */}
      <nav className="border-b border-white/5 bg-tiger-dark/50 backdrop-blur-xl sticky top-16 z-40">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
          <div className="flex items-center justify-between h-14">
            <div className="flex items-center space-x-1">
              {tabs.map((tab) => (
                <button
                  key={tab.id}
                  onClick={() => setActiveTab(tab.id)}
                  className={`flex items-center space-x-2 px-4 py-2 rounded-lg text-sm font-medium transition-all ${
                    activeTab === tab.id
                      ? 'bg-tiger-orange/20 text-tiger-orange'
                      : 'text-gray-400 hover:text-white hover:bg-white/5'
                  }`}
                >
                  <tab.icon className="w-4 h-4" />
                  <span className="hidden sm:inline">{tab.label}</span>
                </button>
              ))}
            </div>
            
            <div className="flex items-center space-x-3">
              <button className="p-2 text-gray-400 hover:text-white transition-colors">
                <Bell className="w-5 h-5" />
              </button>
              <button className="p-2 text-gray-400 hover:text-white transition-colors">
                <Settings className="w-5 h-5" />
              </button>
              <button 
                onClick={() => setMobileMenuOpen(!mobileMenuOpen)}
                className="p-2 text-gray-400 hover:text-white transition-colors lg:hidden"
              >
                {mobileMenuOpen ? <X className="w-5 h-5" /> : <Menu className="w-5 h-5" />}
              </button>
            </div>
          </div>
        </div>
      </nav>

      {/* Main Content */}
      <main className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
        <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
          {/* Left Panel - Trading Interface */}
          <div className="lg:col-span-2 space-y-6">
            {/* Chain Selector */}
            <ChainSelector 
              selectedChain={selectedChain} 
              onSelectChain={setSelectedChain} 
            />
            
            {/* Active Trading Interface */}
            <AnimatePresence mode="wait">
              <motion.div
                key={activeTab}
                initial={{ opacity: 0, y: 10 }}
                animate={{ opacity: 1, y: 0 }}
                exit={{ opacity: 0, y: -10 }}
                transition={{ duration: 0.2 }}
              >
                {activeTab === 'swap' && <SwapInterface />}
                {activeTab === 'perpetuals' && <PerpetualTrading />}
                {activeTab === 'cross-chain' && <CrossChain />}
                {activeTab === 'portfolio' && <Portfolio />}
              </motion.div>
            </AnimatePresence>
          </div>

          {/* Right Panel - Info & Stats */}
          <div className="space-y-6">
            {/* Gas & Network Status */}
            <div className="glass-dark rounded-2xl p-4">
              <div className="flex items-center justify-between mb-4">
                <h3 className="text-sm font-semibold text-white">Network Status</h3>
                <span className="flex items-center text-green-400 text-sm">
                  <span className="w-2 h-2 bg-green-400 rounded-full mr-2 animate-pulse"></span>
                  Connected
                </span>
              </div>
              <div className="space-y-3">
                <div className="flex items-center justify-between">
                  <span className="text-gray-400 text-sm">Gas Price</span>
                  <span className="text-white font-medium">2.5 Gwei</span>
                </div>
                <div className="flex items-center justify-between">
                  <span className="text-gray-400 text-sm">Block</span>
                  <span className="text-white font-medium">#18,542,321</span>
                </div>
                <div className="flex items-center justify-between">
                  <span className="text-gray-400 text-sm">Avg. Confirm</span>
                  <span className="text-white font-medium">12s</span>
                </div>
              </div>
            </div>

            {/* Market Stats */}
            <div className="glass-dark rounded-2xl p-4">
              <div className="flex items-center justify-between mb-4">
                <h3 className="text-sm font-semibold text-white">Market Stats</h3>
                <TrendingUp className="w-4 h-4 text-tiger-orange" />
              </div>
              <div className="space-y-3">
                <div className="flex items-center justify-between">
                  <span className="text-gray-400 text-sm">TVL</span>
                  <span className="text-white font-medium">$2.4B</span>
                </div>
                <div className="flex items-center justify-between">
                  <span className="text-gray-400 text-sm">24h Volume</span>
                  <span className="text-white font-medium">$892M</span>
                </div>
                <div className="flex items-center justify-between">
                  <span className="text-gray-400 text-sm">Pairs</span>
                  <span className="text-white font-medium">12,450</span>
                </div>
              </div>
            </div>

            {/* Supported Chains */}
            <div className="glass-dark rounded-2xl p-4">
              <div className="flex items-center justify-between mb-4">
                <h3 className="text-sm font-semibold text-white">Supported Chains</h3>
                <Layers className="w-4 h-4 text-tiger-orange" />
              </div>
              <div className="grid grid-cols-4 gap-2">
                {['ETH', 'BSC', 'MATIC', 'ARB', 'OPT', 'AVAX', 'SOL', 'APT'].map((chain) => (
                  <div 
                    key={chain}
                    className="aspect-square rounded-lg bg-white/5 flex items-center justify-center text-xs font-medium text-gray-400 hover:bg-white/10 hover:text-white transition-colors cursor-pointer"
                  >
                    {chain}
                  </div>
                ))}
              </div>
              <button className="w-full mt-4 py-2 text-sm text-tiger-orange hover:text-tiger-accent transition-colors">
                View all 100+ chains →
              </button>
            </div>

            {/* Recent Transactions */}
            <div className="glass-dark rounded-2xl p-4">
              <div className="flex items-center justify-between mb-4">
                <h3 className="text-sm font-semibold text-white">Recent Activity</h3>
                <History className="w-4 h-4 text-tiger-orange" />
              </div>
              <div className="space-y-3">
                {[
                  { type: 'Swap', from: 'ETH', to: 'USDT', amount: '1.5 ETH', time: '2s ago' },
                  { type: 'Bridge', from: 'ETH', to: 'ARB', amount: '1000 USDC', time: '15s ago' },
                  { type: 'Swap', from: 'BNB', to: 'BUSD', amount: '5 BNB', time: '32s ago' },
                ].map((tx, i) => (
                  <div key={i} className="flex items-center justify-between text-sm">
                    <div className="flex items-center space-x-2">
                      <span className="text-gray-400">{tx.type}</span>
                      <span className="text-white">{tx.from}</span>
                      <ArrowRightLeft className="w-3 h-3 text-gray-500" />
                      <span className="text-white">{tx.to}</span>
                    </div>
                    <div className="text-right">
                      <div className="text-white font-medium">{tx.amount}</div>
                      <div className="text-gray-500 text-xs">{tx.time}</div>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          </div>
        </div>
      </main>

      {/* Footer */}
      <footer className="border-t border-white/5 bg-tiger-dark/50 mt-12">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
          <div className="grid grid-cols-2 md:grid-cols-4 gap-8">
            <div>
              <h4 className="text-white font-semibold mb-4">Protocol</h4>
              <ul className="space-y-2 text-sm text-gray-400">
                <li><a href="#" className="hover:text-tiger-orange transition-colors">Documentation</a></li>
                <li><a href="#" className="hover:text-tiger-orange transition-colors">Governance</a></li>
                <li><a href="#" className="hover:text-tiger-orange transition-colors">Stats</a></li>
              </ul>
            </div>
            <div>
              <h4 className="text-white font-semibold mb-4">Developers</h4>
              <ul className="space-y-2 text-sm text-gray-400">
                <li><a href="#" className="hover:text-tiger-orange transition-colors">SDK</a></li>
                <li><a href="#" className="hover:text-tiger-orange transition-colors">API</a></li>
                <li><a href="#" className="hover:text-tiger-orange transition-colors">Smart Contracts</a></li>
              </ul>
            </div>
            <div>
              <h4 className="text-white font-semibold mb-4">Support</h4>
              <ul className="space-y-2 text-sm text-gray-400">
                <li><a href="#" className="hover:text-tiger-orange transition-colors">Help Center</a></li>
                <li><a href="#" className="hover:text-tiger-orange transition-colors">Discord</a></li>
                <li><a href="#" className="hover:text-tiger-orange transition-colors">Twitter</a></li>
              </ul>
            </div>
            <div>
              <h4 className="text-white font-semibold mb-4">Legal</h4>
              <ul className="space-y-2 text-sm text-gray-400">
                <li><a href="#" className="hover:text-tiger-orange transition-colors">Privacy Policy</a></li>
                <li><a href="#" className="hover:text-tiger-orange transition-colors">Terms of Service</a></li>
                <li><a href="#" className="hover:text-tiger-orange transition-colors">Bug Bounty</a></li>
              </ul>
            </div>
          </div>
          <div className="mt-8 pt-8 border-t border-white/5 text-center text-sm text-gray-500">
            © 2024 TigerSwap. All rights reserved.
          </div>
        </div>
      </footer>
    </div>
  );
}
