'use client';

import { useState } from 'react';
import { useAccount } from 'wagmi';
import { ConnectButton } from '@rainbow-me/rainbowkit';
import { motion } from 'framer-motion';
import { 
  Image, 
  Search, 
  Filter,
  Grid,
  List,
  TrendingUp,
  Clock,
  Heart,
  ExternalLink,
  Wallet
} from 'lucide-react';
import Link from 'next/link';

const COLLECTIONS = [
  {
    id: '1',
    name: 'Bored Ape Yacht Club',
    symbol: 'BAYC',
    floorPrice: 18.5,
    floorChange: 5.2,
    totalVolume: 250000,
    image: 'https://cryptologos.cc/logos/bored-ape-yacht-club-bayc-logo.png',
  },
  {
    id: '2',
    name: 'CryptoPunks',
    symbol: 'PUNK',
    floorPrice: 45.2,
    floorChange: -2.1,
    totalVolume: 180000,
    image: 'https://cryptologos.cc/logos/cryptopunks-crypto-punks-logo.png',
  },
  {
    id: '3',
    name: 'Azuki',
    symbol: 'AZUKI',
    floorPrice: 12.8,
    floorChange: 8.5,
    totalVolume: 95000,
    image: 'https://cryptologos.cc/logos/azuki-azuki-logo.png',
  },
];

const NFTS = [
  {
    id: '1',
    tokenId: '7804',
    name: 'Bored Ape #7804',
    collection: 'Bored Ape Yacht Club',
    price: 22.5,
    image: 'https://cryptologos.cc/logos/bored-ape-yacht-club-bayc-logo.png',
    likes: 128,
  },
  {
    id: '2',
    tokenId: '3142',
    name: 'CryptoPunk #3142',
    collection: 'CryptoPunks',
    price: 55.0,
    image: 'https://cryptologos.cc/logos/cryptopunks-crypto-punks-logo.png',
    likes: 89,
  },
  {
    id: '3',
    tokenId: '9821',
    name: 'Azuki #9821',
    collection: 'Azuki',
    price: 15.2,
    image: 'https://cryptologos.cc/logos/azuki-azuki-logo.png',
    likes: 64,
  },
];

export default function NFTPage() {
  const { address, isConnected } = useAccount();
  const [activeTab, setActiveTab] = useState<'marketplace' | 'collections' | 'owned'>('marketplace');
  const [viewMode, setViewMode] = useState<'grid' | 'list'>('grid');
  const [searchQuery, setSearchQuery] = useState('');

  if (!isConnected) {
    return (
      <div className="min-h-screen bg-tiger-dark flex items-center justify-center p-4">
        <div className="text-center">
          <Image className="w-20 h-20 text-tiger-orange mx-auto mb-6" />
          <h1 className="text-3xl font-bold text-white mb-4">Connect for NFTs</h1>
          <p className="text-gray-400 mb-8 max-w-md">
            Connect your wallet to view, buy, and sell NFTs across multiple marketplaces.
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
                  <Image className="w-5 h-5 text-black" />
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
          <h1 className="text-3xl font-bold text-white mb-2">NFT Marketplace</h1>
          <p className="text-gray-400">Trade NFTs across OpenSea, Magic Eden, and more</p>
        </div>

        {/* Search & Filters */}
        <div className="flex flex-col md:flex-row items-center justify-between gap-4 mb-8">
          <div className="relative flex-1 max-w-xl">
            <Search className="absolute left-4 top-1/2 -translate-y-1/2 w-5 h-5 text-gray-400" />
            <input
              type="text"
              placeholder="Search NFTs, collections..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="w-full pl-12 pr-4 py-3 rounded-xl bg-white/5 border border-white/10 text-white placeholder-gray-500 focus:outline-none focus:border-tiger-orange"
            />
          </div>
          
          <div className="flex items-center space-x-2">
            <button className="p-3 rounded-xl bg-white/5 border border-white/10 hover:border-white/20 transition-colors">
              <Filter className="w-5 h-5 text-gray-400" />
            </button>
            <button
              onClick={() => setViewMode('grid')}
              className={`p-3 rounded-xl transition-colors ${viewMode === 'grid' ? 'bg-tiger-orange text-white' : 'bg-white/5 border border-white/10 text-gray-400'}`}
            >
              <Grid className="w-5 h-5" />
            </button>
            <button
              onClick={() => setViewMode('list')}
              className={`p-3 rounded-xl transition-colors ${viewMode === 'list' ? 'bg-tiger-orange text-white' : 'bg-white/5 border border-white/10 text-gray-400'}`}
            >
              <List className="w-5 h-5" />
            </button>
          </div>
        </div>

        {/* Tabs */}
        <div className="flex items-center space-x-1 mb-6 bg-white/5 rounded-xl p-1 w-fit">
          {(['marketplace', 'collections', 'owned'] as const).map((tab) => (
            <button
              key={tab}
              onClick={() => setActiveTab(tab)}
              className={`py-2.5 px-6 rounded-lg text-sm font-medium transition-colors ${
                activeTab === tab 
                  ? 'bg-white/10 text-white' 
                  : 'text-gray-400 hover:text-white'
              }`}
            >
              {tab.charAt(0).toUpperCase() + tab.slice(1)}
            </button>
          ))}
        </div>

        {/* Marketplace Tab */}
        {activeTab === 'marketplace' && (
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            className={viewMode === 'grid' ? 'grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6' : 'space-y-4'}
          >
            {NFTS.map((nft) => (
              <div 
                key={nft.id}
                className="group p-4 rounded-2xl bg-white/5 border border-white/10 hover:border-tiger-orange/50 transition-colors"
              >
                <div className="relative mb-4 aspect-square rounded-xl overflow-hidden bg-white/10">
                  <img 
                    src={nft.image} 
                    alt={nft.name}
                    className="w-full h-full object-cover group-hover:scale-110 transition-transform duration-300"
                  />
                  <button className="absolute top-3 right-3 p-2 rounded-full bg-black/50 hover:bg-black/70 transition-colors">
                    <Heart className="w-4 h-4 text-white" />
                  </button>
                </div>
                
                <div className="space-y-2">
                  <div>
                    <p className="text-sm text-gray-400">{nft.collection}</p>
                    <h3 className="font-semibold text-white">{nft.name}</h3>
                  </div>
                  
                  <div className="flex items-center justify-between">
                    <div>
                      <p className="text-xs text-gray-400">Price</p>
                      <p className="font-semibold text-white">{nft.price} ETH</p>
                    </div>
                    <button className="px-4 py-2 rounded-lg bg-tiger-orange text-white text-sm font-medium hover:bg-tiger-orange/90 transition-colors">
                      Buy Now
                    </button>
                  </div>
                </div>
              </div>
            ))}
          </motion.div>
        )}

        {/* Collections Tab */}
        {activeTab === 'collections' && (
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            className="space-y-4"
          >
            {COLLECTIONS.map((collection) => (
              <div 
                key={collection.id}
                className="p-4 rounded-2xl bg-white/5 border border-white/10 hover:border-white/20 transition-colors flex items-center justify-between"
              >
                <div className="flex items-center space-x-4">
                  <img 
                    src={collection.image} 
                    alt={collection.name}
                    className="w-16 h-16 rounded-xl"
                  />
                  <div>
                    <h3 className="font-semibold text-white">{collection.name}</h3>
                    <p className="text-sm text-gray-400">{collection.symbol}</p>
                  </div>
                </div>
                
                <div className="flex items-center space-x-8">
                  <div className="text-right">
                    <p className="text-xs text-gray-400">Floor Price</p>
                    <p className="font-semibold text-white">{collection.floorPrice} ETH</p>
                    <p className={`text-xs ${collection.floorChange >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                      {collection.floorChange >= 0 ? '+' : ''}{collection.floorChange}%
                    </p>
                  </div>
                  <div className="text-right">
                    <p className="text-xs text-gray-400">Volume</p>
                    <p className="font-semibold text-white">{(collection.totalVolume / 1000).toFixed(0)}K ETH</p>
                  </div>
                  <TrendingUp className="w-8 h-8 text-tiger-orange" />
                </div>
              </div>
            ))}
          </motion.div>
        )}

        {/* Owned Tab */}
        {activeTab === 'owned' && (
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
          >
            <div className="text-center py-12">
              <Wallet className="w-16 h-16 text-gray-400 mx-auto mb-4" />
              <h3 className="text-xl font-semibold text-white mb-2">No NFTs Yet</h3>
              <p className="text-gray-400 mb-6">Start collecting NFTs from the marketplace</p>
              <button className="px-6 py-3 rounded-xl bg-tiger-orange text-white font-medium">
                Browse Marketplace
              </button>
            </div>
          </motion.div>
        )}
      </div>
    </div>
  );
}
