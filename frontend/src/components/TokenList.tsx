'use client';

import { useState } from 'react';
import { Search, Star, TrendingUp, Clock } from 'lucide-react';
import { Token } from '@/store/useStore';

interface TokenListProps {
  onSelectToken: (token: Token) => void;
}

const ALL_TOKENS: Token[] = [
  // Top by market cap
  { address: '0x0000000000000000000000000000000000000000', symbol: 'ETH', name: 'Ethereum', decimals: 18, logoURI: 'https://assets.coingecko.com/coins/images/279/small/ethereum.png', chainId: 1, price: 3450.00 },
  { address: '0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48', symbol: 'USDC', name: 'USD Coin', decimals: 6, logoURI: 'https://assets.coingecko.com/coins/images/6319/small/usdc.png', chainId: 1, price: 1.00 },
  { address: '0xdAC17F958D2ee523a2206206994597C13D831ec7', symbol: 'USDT', name: 'Tether USD', decimals: 6, logoURI: 'https://assets.coingecko.com/coins/images/325/small/Tether.png', chainId: 1, price: 1.00 },
  { address: '0x2260FAC5E5542a773Aa44fBCfeDf7C193bc2C599', symbol: 'WBTC', name: 'Wrapped Bitcoin', decimals: 8, logoURI: 'https://assets.coingecko.com/coins/images/7598/small/wrapped_bitcoin_wbtc.png', chainId: 1, price: 67500.00 },
  { address: '0x514910771AF9Ca656af840dff83E8264EcF986CA', symbol: 'LINK', name: 'Chainlink', decimals: 18, logoURI: 'https://assets.coingecko.com/coins/images/877/small/chainlink-new-logo.png', chainId: 1, price: 18.50 },
  { address: '0x1f9840a85d5aF5bf1D1762F925BDADdC4201F984', symbol: 'UNI', name: 'Uniswap', decimals: 18, logoURI: 'https://assets.coingecko.com/coins/images/12504/small/uniswap-uni.png', chainId: 1, price: 12.80 },
  { address: '0x7Fc66500c84A76Ad7e9c93437bFc5Ac33E2DDaE9', symbol: 'AAVE', name: 'Aave', decimals: 18, logoURI: 'https://assets.coingecko.com/coins/images/12645/small/AAVE.png', chainId: 1, price: 285.00 },
  { address: '0x6B175474E89094C44Da98b954EedeAC495271d0F', symbol: 'DAI', name: 'Dai Stablecoin', decimals: 18, logoURI: 'https://assets.coingecko.com/coins/images/9956/small/4943.png', chainId: 1, price: 1.00 },
  { address: '0x7D1AfA7B718fb893dB30A3aBc0Cfc608AaCfeBB0', symbol: 'MATIC', name: 'Polygon', decimals: 18, logoURI: 'https://assets.coingecko.com/coins/images/4713/small/polygon.png', chainId: 1, price: 0.85 },
  { address: '0x4d224452801ACEd8B2F0aEBE155379bb55946036', symbol: 'AAVE', name: 'Aave', decimals: 18, logoURI: 'https://assets.coingecko.com/coins/images/12645/small/AAVE.png', chainId: 137, price: 285.00 },
  { address: '0x1BFD67037B42Cf73acF2047067bd4F2C47D9BfD6', symbol: 'WBTC', name: 'Wrapped Bitcoin', decimals: 8, logoURI: 'https://assets.coingecko.com/coins/images/7598/small/wrapped_bitcoin_wbtc.png', chainId: 137, price: 67500.00 },
  // More tokens
  { address: '0xbBbBBBBbbBBBBBbbbBbbBbbbbBBbBbbbbBBbB', symbol: 'BTC', name: 'Bitcoin', decimals: 8, logoURI: 'https://assets.coingecko.com/coins/images/1/small/bitcoin.png', chainId: 1, price: 68500.00 },
  { address: '0x8f3Cf7ad23Cd3CaDbD9735AFf958023239c6A063', symbol: 'DAI', name: 'Dai Stablecoin', decimals: 18, logoURI: 'https://assets.coingecko.com/coins/images/9956/small/4943.png', chainId: 137, price: 1.00 },
  // BNB Chain tokens
  { address: '0xbb4CdB9CBd36B01bD1cBaEBF2De08d9173bc095c', symbol: 'WBNB', name: 'Wrapped BNB', decimals: 18, logoURI: 'https://assets.coingecko.com/coins/images/825/small/bnb-icon2_2x.png', chainId: 56, price: 580.00 },
  { address: '0x55d398326f99059fF775485246999027B3197955', symbol: 'USDT', name: 'Tether USD', decimals: 18, logoURI: 'https://assets.coingecko.com/coins/images/325/small/Tether.png', chainId: 56, price: 1.00 },
  { address: '0xe9e7CEA3DedcA5984780Bafc599bD69ADd087D56', symbol: 'BUSD', name: 'Binance USD', decimals: 18, logoURI: 'https://assets.coingecko.com/coins/images/9576/small/BUSD.png', chainId: 56, price: 1.00 },
  // Solana tokens
  { address: 'So11111111111111111111111111111111111111112', symbol: 'SOL', name: 'Solana', decimals: 9, logoURI: 'https://assets.coingecko.com/coins/images/4128/small/solana.png', chainId: 101, price: 145.00 },
  { address: 'EPjFWdd5AufqSSFqM7BcAmfpR3T1nr9eTuec6G5ZCXNs', symbol: 'USDC', name: 'USD Coin', decimals: 6, logoURI: 'https://assets.coingecko.com/coins/images/6319/small/usdc.png', chainId: 101, price: 1.00 },
  // Pi Network (testnet placeholder)
  { address: '0x0000000000000000000000000000000000000000', symbol: 'PI', name: 'Pi Network', decimals: 18, logoURI: 'https://assets.coingecko.com/coins/images/，群内可看/piconix.jpeg', chainId: 314159, price: 0.00 },
  // More popular tokens
  { address: '0xD533a949740bb3306d119CC777fa900bA034cd52', symbol: 'CRV', name: 'Curve DAO', decimals: 18, logoURI: 'https://assets.coingecko.com/coins/images/12124/small/Curve.png', chainId: 1, price: 0.45 },
  { address: '0xba7435a4b4C747E0101780073eeda872a69Bdcd4', symbol: 'DOGE', name: 'Dogecoin', decimals: 8, logoURI: 'https://assets.coingecko.com/coins/images/5/small/dogecoin.png', chainId: 1, price: 0.15 },
  { address: '0xE4a2620edE3B6a13B2d7EaB42e54d4b4d6b5b5b5', symbol: 'TRX', name: 'TRON', decimals: 6, logoURI: 'https://assets.coingecko.com/coins/images/1094/small/tron-logo.png', chainId: 1, price: 0.12 },
  { address: '0x3506424F91fD33084466F402d5D97f05F8e3b4AF', symbol: 'PAXG', name: 'Pax Gold', decimals: 18, logoURI: 'https://assets.coingecko.com/coins/images/9519/small/paxg_.png', chainId: 1, price: 2350.00 },
];

export function TokenList({ onSelectToken }: TokenListProps) {
  const [searchQuery, setSearchQuery] = useState('');
  const [activeTab, setActiveTab] = useState<'all' | 'popular' | 'favorites'>('all');

  const filteredTokens = ALL_TOKENS.filter(token => 
    token.symbol.toLowerCase().includes(searchQuery.toLowerCase()) ||
    token.name.toLowerCase().includes(searchQuery.toLowerCase())
  );

  const popularTokens = ALL_TOKENS.slice(0, 10);

  return (
    <div className="w-full max-w-md">
      <div className="mb-4">
        <div className="relative">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-5 h-5 text-gray-400" />
          <input
            type="text"
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            placeholder="Search tokens..."
            className="w-full pl-10 pr-4 py-3 bg-white/5 border border-white/10 rounded-xl text-white placeholder:text-gray-500 focus:outline-none focus:border-tiger-orange/50"
          />
        </div>
      </div>

      <div className="flex space-x-2 mb-4">
        {[
          { id: 'all', label: 'All', icon: null },
          { id: 'popular', label: 'Popular', icon: TrendingUp },
          { id: 'favorites', label: 'Favorites', icon: Star },
        ].map(tab => (
          <button
            key={tab.id}
            onClick={() => setActiveTab(tab.id as any)}
            className={`flex-1 flex items-center justify-center space-x-2 px-4 py-2 rounded-lg transition-colors ${
              activeTab === tab.id
                ? 'bg-tiger-orange/20 text-tiger-orange border border-tiger-orange/30'
                : 'bg-white/5 text-gray-400 hover:text-white'
            }`}
          >
            {tab.icon && <tab.icon className="w-4 h-4" />}
            <span>{tab.label}</span>
          </button>
        ))}
      </div>

      <div className="space-y-1 max-h-80 overflow-y-auto">
        {(activeTab === 'popular' ? popularTokens : filteredTokens).map(token => (
          <button
            key={`${token.chainId}-${token.symbol}`}
            onClick={() => onSelectToken(token)}
            className="w-full flex items-center space-x-3 p-3 rounded-xl hover:bg-white/5 transition-colors"
          >
            <img src={token.logoURI} alt={token.symbol} className="w-10 h-10 rounded-full" />
            <div className="flex-1 text-left">
              <div className="text-white font-medium">{token.symbol}</div>
              <div className="text-gray-500 text-sm">{token.name}</div>
            </div>
            <div className="text-right">
              <div className="text-white font-medium">${token.price?.toLocaleString() || '0.00'}</div>
              <div className="text-green-400 text-xs">+2.5%</div>
            </div>
          </button>
        ))}
      </div>
    </div>
  );
}
