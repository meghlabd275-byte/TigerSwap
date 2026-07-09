'use client';

import { useState } from 'react';
import { useAccount } from 'wagmi';
import { ConnectButton } from '@rainbow-me/rainbowkit';
import { motion } from 'framer-motion';
import { 
  Store, 
  Search, 
  Globe,
  TrendingUp,
  Star,
  ExternalLink,
  Filter,
  Zap
} from 'lucide-react';
import Link from 'next/link';

const CATEGORIES = [
  { id: 'all', name: 'All', icon: Store },
  { id: 'defi', name: 'DeFi', icon: Zap },
  { id: 'nft', name: 'NFT', icon: Star },
  { id: 'games', name: 'Games', icon: TrendingUp },
  { id: 'social', name: 'Social', icon: Globe },
  { id: 'bridge', name: 'Bridge', icon: Store },
  { id: 'staking', name: 'Staking', icon: Star },
];

const DAPPS = [
  {
    id: '1',
    name: 'Uniswap',
    description: ' Decentralized trading protocol for automated trading',
    category: 'defi',
    url: 'https://uniswap.org',
    logo: 'https://cryptologos.cc/logos/uniswap-uni-logo.png',
    chains: [1, 10, 42161, 8453],
    verified: true,
    rating: 4.8,
    visits: 1250000,
  },
  {
    id: '2',
    name: 'OpenSea',
    description: 'NFT marketplace for buying and selling digital items',
    category: 'nft',
    url: 'https://opensea.io',
    logo: 'https://cryptologos.cc/logos/opensea-logo.png',
    chains: [1, 137, 10],
    verified: true,
    rating: 4.6,
    visits: 2500000,
  },
  {
    id: '3',
    name: 'Aave',
    description: 'Non-custodial liquidity protocol for earning interest',
    category: 'defi',
    url: 'https://aave.com',
    logo: 'https://cryptologos.cc/logos/aave-aave-logo.png',
    chains: [1, 137, 10],
    verified: true,
    rating: 4.7,
    visits: 800000,
  },
  {
    id: '4',
    name: 'Magic Eden',
    description: 'NFT marketplace on Solana and Ethereum',
    category: 'nft',
    url: 'https://magiceden.io',
    logo: 'https://cryptologos.cc/logos/magic-eden-logo.png',
    chains: [1, 1399811149], // Ethereum, Solana
    verified: true,
    rating: 4.5,
    visits: 1500000,
  },
  {
    id: '5',
    name: 'Curve',
    description: 'Stablecoin exchange for low-slippage trades',
    category: 'defi',
    url: 'https://curve.fi',
    logo: 'https://cryptologos.cc/logos/curve-dao-token-crv-logo.png',
    chains: [1, 10, 137, 43114],
    verified: true,
    rating: 4.6,
    visits: 600000,
  },
  {
    id: '6',
    name: 'Stargate',
    description: 'Cross-chain liquidity protocol',
    category: 'bridge',
    url: 'https://stargate.finance',
    logo: 'https://cryptologos.cc/logos/stargate-stg-logo.png',
    chains: [1, 10, 42161, 43114, 56],
    verified: true,
    rating: 4.4,
    visits: 400000,
  },
];

export default function DAppsPage() {
  const { isConnected } = useAccount();
  const [activeCategory, setActiveCategory] = useState('all');
  const [searchQuery, setSearchQuery] = useState('');

  const filteredDApps = DAPPS.filter(dapp => {
    const matchesCategory = activeCategory === 'all' || dapp.category === activeCategory;
    const matchesSearch = dapp.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
                          dapp.description.toLowerCase().includes(searchQuery.toLowerCase());
    return matchesCategory && matchesSearch;
  });

  return (
    <div className="min-h-screen bg-tiger-dark">
      {/* Header */}
      <header className="sticky top-0 z-50 glass-dark border-b border-white/5">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
          <div className="flex items-center justify-between h-16">
            <div className="flex items-center space-x-3">
              <Link href="/" className="flex items-center space-x-3">
                <div className="w-8 h-8 rounded-lg bg-gradient-to-br from-tiger-orange to-tiger-yellow flex items-center justify-center">
                  <Store className="w-5 h-5 text-black" />
                </div>
                <span className="text-lg font-bold text-white">TigerWallet</span>
              </Link>
            </div>
            <ConnectButton showBalance={false} />
          </div>
        </div>
      </header>

      <div className="max-w-7xl mx-auto px-4 py-8">
        {/* Title */}
        <div className="text-center mb-8">
          <h1 className="text-3xl font-bold text-white mb-2">DApp Store</h1>
          <p className="text-gray-400">Discover and use decentralized applications</p>
        </div>

        {/* Search */}
        <div className="relative max-w-xl mx-auto mb-8">
          <Search className="absolute left-4 top-1/2 -translate-y-1/2 w-5 h-5 text-gray-400" />
          <input
            type="text"
            placeholder="Search DApps..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            className="w-full pl-12 pr-4 py-3 rounded-xl bg-white/5 border border-white/10 text-white placeholder-gray-500 focus:outline-none focus:border-tiger-orange"
          />
        </div>

        {/* Categories */}
        <div className="flex flex-wrap items-center justify-center gap-2 mb-8">
          {CATEGORIES.map((category) => (
            <button
              key={category.id}
              onClick={() => setActiveCategory(category.id)}
              className={`flex items-center space-x-2 px-4 py-2 rounded-xl transition-colors ${
                activeCategory === category.id
                  ? 'bg-tiger-orange text-white'
                  : 'bg-white/5 border border-white/10 text-gray-400 hover:text-white hover:border-white/20'
              }`}
            >
              <category.icon className="w-4 h-4" />
              <span className="text-sm font-medium">{category.name}</span>
            </button>
          ))}
        </div>

        {/* Featured */}
        {activeCategory === 'all' && !searchQuery && (
          <div className="mb-8">
            <h2 className="text-xl font-semibold text-white mb-4">Featured</h2>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div className="p-6 rounded-2xl bg-gradient-to-r from-tiger-orange/20 to-tiger-yellow/20 border border-tiger-orange/20">
                <div className="flex items-start justify-between">
                  <div>
                    <h3 className="text-lg font-semibold text-white mb-1">Uniswap</h3>
                    <p className="text-sm text-gray-400">Swap tokens instantly</p>
                  </div>
                  <Zap className="w-6 h-6 text-tiger-orange" />
                </div>
                <button className="mt-4 px-4 py-2 rounded-lg bg-tiger-orange text-white text-sm font-medium">
                  Launch
                </button>
              </div>
              <div className="p-6 rounded-2xl bg-gradient-to-r from-purple-500/20 to-pink-500/20 border border-purple-500/20">
                <div className="flex items-start justify-between">
                  <div>
                    <h3 className="text-lg font-semibold text-white mb-1">OpenSea</h3>
                    <p className="text-sm text-gray-400">Trade NFTs</p>
                  </div>
                  <Star className="w-6 h-6 text-purple-400" />
                </div>
                <button className="mt-4 px-4 py-2 rounded-lg bg-purple-500 text-white text-sm font-medium">
                  Launch
                </button>
              </div>
            </div>
          </div>
        )}

        {/* DApps Grid */}
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {filteredDApps.map((dapp) => (
            <motion.div
              key={dapp.id}
              initial={{ opacity: 0, y: 20 }}
              animate={{ opacity: 1, y: 0 }}
              className="group p-4 rounded-2xl bg-white/5 border border-white/10 hover:border-tiger-orange/50 transition-colors"
            >
              <div className="flex items-start space-x-4">
                <img
                  src={dapp.logo}
                  alt={dapp.name}
                  className="w-14 h-14 rounded-xl"
                />
                <div className="flex-1">
                  <div className="flex items-center space-x-2">
                    <h3 className="font-semibold text-white">{dapp.name}</h3>
                    {dapp.verified && (
                      <span className="px-1.5 py-0.5 rounded bg-green-500/20 text-green-400 text-xs">
                        ✓
                      </span>
                    )}
                  </div>
                  <p className="text-sm text-gray-400 line-clamp-2 mt-1">{dapp.description}</p>
                </div>
              </div>

              <div className="mt-4 flex items-center justify-between">
                <div className="flex items-center space-x-4 text-sm">
                  <div className="flex items-center space-x-1 text-gray-400">
                    <Star className="w-4 h-4 text-yellow-400" />
                    <span>{dapp.rating}</span>
                  </div>
                  <div className="flex items-center space-x-1 text-gray-400">
                    <TrendingUp className="w-4 h-4" />
                    <span>{(dapp.visits / 1000).toFixed(0)}K</span>
                  </div>
                </div>
                
                <a
                  href={dapp.url}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="p-2 rounded-lg bg-white/10 hover:bg-white/20 transition-colors"
                >
                  <ExternalLink className="w-4 h-4 text-gray-400" />
                </a>
              </div>

              <div className="mt-4 flex flex-wrap gap-1">
                {dapp.chains.map((chain) => (
                  <span
                    key={chain}
                    className="px-2 py-1 rounded bg-white/5 text-xs text-gray-400"
                  >
                    {getChainName(chain)}
                  </span>
                ))}
              </div>
            </motion.div>
          ))}
        </div>

        {filteredDApps.length === 0 && (
          <div className="text-center py-12">
            <Store className="w-16 h-16 text-gray-400 mx-auto mb-4" />
            <h3 className="text-xl font-semibold text-white mb-2">No DApps Found</h3>
            <p className="text-gray-400">Try adjusting your search or filters</p>
          </div>
        )}
      </div>
    </div>
  );
}

function getChainName(chainId: number): string {
  const chains: Record<number, string> = {
    1: 'Ethereum',
    10: 'Optimism',
    56: 'BNB',
    137: 'Polygon',
    42161: 'Arbitrum',
    8453: 'Base',
    43114: 'Avalanche',
    1399811149: 'Solana',
  };
  return chains[chainId] || `Chain ${chainId}`;
}
