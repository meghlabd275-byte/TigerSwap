'use client';

import { useState, useEffect } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { 
  ArrowRight, 
  Settings, 
  Zap, 
  Clock, 
  AlertTriangle, 
  ArrowDown,
  RefreshCw,
  Copy,
  Check,
  ChevronDown,
  Search,
  Sliders,
  History
} from 'lucide-react';
import { useStore, Token, SUPPORTED_CHAINS } from '@/store/useStore';

const POPULAR_TOKENS: Token[] = [
  { address: '0x0000000000000000000000000000000000000000', symbol: 'ETH', name: 'Ethereum', decimals: 18, logoURI: 'https://assets.coingecko.com/coins/images/279/small/ethereum.png', chainId: 1 },
  { address: '0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48', symbol: 'USDC', name: 'USD Coin', decimals: 6, logoURI: 'https://assets.coingecko.com/coins/images/6319/small/usdc.png', chainId: 1 },
  { address: '0xdAC17F958D2ee523a2206206994597C13D831ec7', symbol: 'USDT', name: 'Tether USD', decimals: 6, logoURI: 'https://assets.coingecko.com/coins/images/325/small/Tether.png', chainId: 1 },
  { address: '0x2260FAC5E5542a773Aa44fBCfeDf7C193bc2C599', symbol: 'WBTC', name: 'Wrapped Bitcoin', decimals: 8, logoURI: 'https://assets.coingecko.com/coins/images/7598/small/wrapped_bitcoin_wbtc.png', chainId: 1 },
  { address: '0x7Fc66500c84A76Ad7e9c93437bFc5Ac33E2DDaE9', symbol: 'AAVE', name: 'Aave', decimals: 18, logoURI: 'https://assets.coingecko.com/coins/images/12645/small/AAVE.png', chainId: 1 },
  { address: '0x1f9840a85d5aF5bf1D1762F925BDADdC4201F984', symbol: 'UNI', name: 'Uniswap', decimals: 18, logoURI: 'https://assets.coingecko.com/coins/images/12504/small/uniswap-uni.png', chainId: 1 },
  { address: '0x514910771AF9Ca656af840dff83E8264EcF986CA', symbol: 'LINK', name: 'Chainlink', decimals: 18, logoURI: 'https://assets.coingecko.com/coins/images/877/small/chainlink-new-logo.png', chainId: 1 },
  { address: '0x7D1AfA7B718fb893dB30A3aBc0Cfc608AaCfeBB0', symbol: 'MATIC', name: 'Polygon', decimals: 18, logoURI: 'https://assets.coingecko.com/coins/images/4713/small/polygon.png', chainId: 1 },
  { address: '0x4d224452801ACEd8B2F0aEBE155379bb55946036', symbol: 'AAVE', name: 'Aave (Polygon)', decimals: 18, logoURI: 'https://assets.coingecko.com/coins/images/12645/small/AAVE.png', chainId: 137 },
  { address: '0x1BFD67037B42Cf73acF2047067bd4F2C47D9BfD6', symbol: 'WBTC', name: 'Wrapped Bitcoin (Polygon)', decimals: 8, logoURI: 'https://assets.coingecko.com/coins/images/7598/small/wrapped_bitcoin_wbtc.png', chainId: 137 },
];

export function SwapInterface() {
  const { selectedChain } = useStore();
  const [tokenIn, setTokenIn] = useState<Token | null>(null);
  const [tokenOut, setTokenOut] = useState<Token | null>(null);
  const [amountIn, setAmountIn] = useState('');
  const [amountOut, setAmountOut] = useState('');
  const [slippage, setSlippage] = useState(0.5);
  const [deadline, setDeadline] = useState(20);
  const [showTokenSelector, setShowTokenSelector] = useState<'in' | 'out' | null>(null);
  const [showSettings, setShowSettings] = useState(false);
  const [isSwapping, setIsSwapping] = useState(false);
  const [swapSuccess, setSwapSuccess] = useState(false);
  const [searchQuery, setSearchQuery] = useState('');
  const [showExpertMode, setShowExpertMode] = useState(false);

  const filteredTokens = POPULAR_TOKENS.filter(
    token => token.symbol.toLowerCase().includes(searchQuery.toLowerCase()) ||
             token.name.toLowerCase().includes(searchQuery.toLowerCase())
  );

  useEffect(() => {
    // Simulate price calculation
    if (amountIn && tokenIn && tokenOut) {
      // Mock conversion rate
      const rate = Math.random() * 0.1 + 0.9;
      setAmountOut((parseFloat(amountIn) * rate).toFixed(6));
    }
  }, [amountIn, tokenIn, tokenOut]);

  const handleSwap = async () => {
    if (!tokenIn || !tokenOut || !amountIn) return;
    
    setIsSwapping(true);
    
    // Simulate swap execution
    await new Promise(resolve => setTimeout(resolve, 2000));
    
    setIsSwapping(false);
    setSwapSuccess(true);
    
    setTimeout(() => {
      setSwapSuccess(false);
      setAmountIn('');
      setAmountOut('');
    }, 3000);
  };

  const handleSelectToken = (token: Token) => {
    if (showTokenSelector === 'in') {
      setTokenIn(token);
    } else {
      setTokenOut(token);
    }
    setShowTokenSelector(null);
    setSearchQuery('');
  };

  const handleSwitchTokens = () => {
    const temp = tokenIn;
    setTokenIn(tokenOut);
    setTokenOut(temp);
    setAmountIn(amountOut);
    setAmountOut(amountIn);
  };

  return (
    <div className="glass-dark rounded-2xl p-6">
      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <h2 className="text-xl font-display font-bold text-white">Swap</h2>
        <div className="flex items-center space-x-2">
          <button
            onClick={() => setShowExpertMode(!showExpertMode)}
            className={`p-2 rounded-lg transition-colors ${showExpertMode ? 'bg-tiger-orange/20 text-tiger-orange' : 'text-gray-400 hover:text-white'}`}
          >
            <Zap className="w-4 h-4" />
          </button>
          <button
            onClick={() => setShowSettings(!showSettings)}
            className="p-2 text-gray-400 hover:text-white rounded-lg hover:bg-white/5 transition-colors"
          >
            <Settings className="w-4 h-4" />
          </button>
          <button className="p-2 text-gray-400 hover:text-white rounded-lg hover:bg-white/5 transition-colors">
            <History className="w-4 h-4" />
          </button>
        </div>
      </div>

      {/* Settings Panel */}
      <AnimatePresence>
        {showSettings && (
          <motion.div
            initial={{ opacity: 0, height: 0 }}
            animate={{ opacity: 1, height: 'auto' }}
            exit={{ opacity: 0, height: 0 }}
            className="mb-6 p-4 bg-white/5 rounded-xl border border-white/10"
          >
            <div className="grid grid-cols-2 gap-4">
              <div>
                <label className="block text-sm text-gray-400 mb-2">Slippage Tolerance</label>
                <div className="flex items-center space-x-2">
                  {[0.1, 0.5, 1.0].map((value) => (
                    <button
                      key={value}
                      onClick={() => setSlippage(value)}
                      className={`px-3 py-1.5 rounded-lg text-sm ${
                        slippage === value
                          ? 'bg-tiger-orange/20 text-tiger-orange border border-tiger-orange/30'
                          : 'bg-white/5 text-gray-400 border border-white/10 hover:bg-white/10'
                      }`}
                    >
                      {value}%
                    </button>
                  ))}
                  <input
                    type="number"
                    value={slippage}
                    onChange={(e) => setSlippage(parseFloat(e.target.value))}
                    className="w-16 px-2 py-1.5 bg-white/5 border border-white/10 rounded-lg text-white text-sm"
                    step="0.1"
                  />
                </div>
              </div>
              <div>
                <label className="block text-sm text-gray-400 mb-2">Transaction Deadline</label>
                <div className="flex items-center space-x-2">
                  <input
                    type="number"
                    value={deadline}
                    onChange={(e) => setDeadline(parseInt(e.target.value))}
                    className="w-20 px-2 py-1.5 bg-white/5 border border-white/10 rounded-lg text-white text-sm"
                  />
                  <span className="text-gray-400 text-sm">minutes</span>
                </div>
              </div>
            </div>
          </motion.div>
        )}
      </AnimatePresence>

      {/* Token Input */}
      <div className="space-y-4">
        {/* From Token */}
        <div className="relative bg-white/5 rounded-xl border border-white/10 p-4">
          <div className="flex items-center justify-between mb-2">
            <span className="text-gray-400 text-sm">From</span>
            <button className="flex items-center space-x-1 text-gray-400 hover:text-white text-sm transition-colors">
              <span>Balance: 0.00</span>
            </button>
          </div>
          <div className="flex items-center space-x-4">
            <button
              onClick={() => setShowTokenSelector('in')}
              className="flex items-center space-x-2 px-3 py-2 bg-white/5 rounded-lg hover:bg-white/10 transition-colors"
            >
              {tokenIn ? (
                <>
                  <img src={tokenIn.logoURI} alt={tokenIn.symbol} className="w-6 h-6 rounded-full" />
                  <span className="text-white font-medium">{tokenIn.symbol}</span>
                </>
              ) : (
                <span className="text-gray-400">Select token</span>
              )}
              <ChevronDown className="w-4 h-4 text-gray-400" />
            </button>
            <input
              type="number"
              value={amountIn}
              onChange={(e) => setAmountIn(e.target.value)}
              placeholder="0.00"
              className="flex-1 bg-transparent text-2xl font-semibold text-white outline-none placeholder:text-gray-600"
            />
          </div>
        </div>

        {/* Switch Button */}
        <div className="flex justify-center -my-3 relative z-10">
          <button
            onClick={handleSwitchTokens}
            className="p-2 bg-tiger-orange rounded-xl border-4 border-tiger-dark hover:bg-tiger-accent transition-colors"
          >
            <ArrowDown className="w-5 h-5 text-white" />
          </button>
        </div>

        {/* To Token */}
        <div className="relative bg-white/5 rounded-xl border border-white/10 p-4">
          <div className="flex items-center justify-between mb-2">
            <span className="text-gray-400 text-sm">To</span>
            <button className="flex items-center space-x-1 text-gray-400 hover:text-white text-sm transition-colors">
              <span>Balance: 0.00</span>
            </button>
          </div>
          <div className="flex items-center space-x-4">
            <button
              onClick={() => setShowTokenSelector('out')}
              className="flex items-center space-x-2 px-3 py-2 bg-white/5 rounded-lg hover:bg-white/10 transition-colors"
            >
              {tokenOut ? (
                <>
                  <img src={tokenOut.logoURI} alt={tokenOut.symbol} className="w-6 h-6 rounded-full" />
                  <span className="text-white font-medium">{tokenOut.symbol}</span>
                </>
              ) : (
                <span className="text-gray-400">Select token</span>
              )}
              <ChevronDown className="w-4 h-4 text-gray-400" />
            </button>
            <input
              type="number"
              value={amountOut}
              onChange={(e) => setAmountOut(e.target.value)}
              placeholder="0.00"
              className="flex-1 bg-transparent text-2xl font-semibold text-white outline-none placeholder:text-gray-600"
              readOnly
            />
          </div>
        </div>

        {/* Exchange Rate & Price Impact */}
        {amountIn && amountOut && (
          <div className="flex items-center justify-between text-sm">
            <div className="flex items-center space-x-2 text-gray-400">
              <span>Rate</span>
              <span className="text-white">1 {tokenIn?.symbol || ''} = {(parseFloat(amountOut) / parseFloat(amountIn)).toFixed(6)} {tokenOut?.symbol || ''}</span>
            </div>
            <div className="flex items-center space-x-2 text-gray-400">
              <span>Impact</span>
              <span className="text-green-400">0.15%</span>
            </div>
          </div>
        )}

        {/* Swap Button */}
        <button
          onClick={handleSwap}
          disabled={!tokenIn || !tokenOut || !amountIn || isSwapping}
          className={`w-full py-4 rounded-xl font-semibold text-lg transition-all ${
            swapSuccess
              ? 'bg-green-500 text-white'
              : !tokenIn || !tokenOut || !amountIn
              ? 'bg-gray-700 text-gray-500 cursor-not-allowed'
              : 'bg-gradient-to-r from-tiger-orange to-tiger-accent text-white btn-glow hover:opacity-90'
          }`}
        >
          {isSwapping ? (
            <span className="flex items-center justify-center space-x-2">
              <RefreshCw className="w-5 h-5 animate-spin" />
              <span>Swapping...</span>
            </span>
          ) : swapSuccess ? (
            <span className="flex items-center justify-center space-x-2">
              <Check className="w-5 h-5" />
              <span>Swap Complete!</span>
            </span>
          ) : (
            'Swap'
          )}
        </button>

        {/* Warning */}
        {slippage > 5 && (
          <div className="flex items-center space-x-2 text-yellow-400 text-sm">
            <AlertTriangle className="w-4 h-4" />
            <span>High slippage setting. Your trade may fail.</span>
          </div>
        )}
      </div>

      {/* Token Selector Modal */}
      <AnimatePresence>
        {showTokenSelector && (
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            className="fixed inset-0 bg-black/60 backdrop-blur-sm flex items-center justify-center z-50 p-4"
            onClick={() => setShowTokenSelector(null)}
          >
            <motion.div
              initial={{ scale: 0.95, opacity: 0 }}
              animate={{ scale: 1, opacity: 1 }}
              exit={{ scale: 0.95, opacity: 0 }}
              className="w-full max-w-md bg-tiger-dark border border-white/10 rounded-2xl overflow-hidden"
              onClick={(e) => e.stopPropagation()}
            >
              <div className="p-4 border-b border-white/10">
                <h3 className="text-lg font-semibold text-white mb-4">Select a token</h3>
                <div className="relative">
                  <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-5 h-5 text-gray-400" />
                  <input
                    type="text"
                    value={searchQuery}
                    onChange={(e) => setSearchQuery(e.target.value)}
                    placeholder="Search tokens..."
                    className="w-full pl-10 pr-4 py-3 bg-white/5 border border-white/10 rounded-xl text-white placeholder:text-gray-500 focus:outline-none focus:border-tiger-orange/50"
                    autoFocus
                  />
                </div>
              </div>
              <div className="max-h-80 overflow-y-auto p-2">
                {filteredTokens.map((token) => (
                  <button
                    key={`${token.chainId}-${token.address}`}
                    onClick={() => handleSelectToken(token)}
                    className="w-full flex items-center space-x-3 p-3 rounded-xl hover:bg-white/5 transition-colors"
                  >
                    <img src={token.logoURI} alt={token.symbol} className="w-10 h-10 rounded-full" />
                    <div className="flex-1 text-left">
                      <div className="text-white font-medium">{token.symbol}</div>
                      <div className="text-gray-500 text-sm">{token.name}</div>
                    </div>
                    <div className="text-gray-400 text-sm">
                      {SUPPORTED_CHAINS[Object.keys(SUPPORTED_CHAINS).find(key => SUPPORTED_CHAINS[key as keyof typeof SUPPORTED_CHAINS].id === token.chainId) || '']?.symbol || ''}
                    </div>
                  </button>
                ))}
              </div>
            </motion.div>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
}
