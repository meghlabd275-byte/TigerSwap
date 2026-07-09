'use client';

import { useState, useEffect } from 'react';
import { Search, ArrowDown, Settings, RefreshCw, Wallet, TrendingUp, Activity, X, Loader2 } from 'lucide-react';
import { LineChart, Line, XAxis, YAxis, Tooltip, ResponsiveContainer } from 'recharts';
import { calculateSwapOutput, getTokensWithPrices, PriceData } from '../lib/services/priceService';

// Token interface
interface Token {
  symbol: string;
  name: string;
  address: string;
  logo: string;
}

// Token list with real contract addresses
const TOKENS: Token[] = [
  { symbol: 'ETH', name: 'Ethereum', address: '0x0000000000000000000000000000000000000000', logo: 'https://assets.coingecko.com/coins/images/279/small/ethereum.png' },
  { symbol: 'USDT', name: 'Tether USD', address: '0xdAC17F958D2ee523a2206206994597C13D831ec7', logo: 'https://assets.coingecko.com/coins/images/325/small/Tether.png' },
  { symbol: 'USDC', name: 'USD Coin', address: '0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48', logo: 'https://assets.coingecko.com/coins/images/6319/small/USD_Coin_icon.png' },
  { symbol: 'WBTC', name: 'Wrapped Bitcoin', address: '0x2260FAC5E5542a773Aa44fBCfeDf7C193bc2C599', logo: 'https://assets.coingecko.com/coins/images/7598/small/wrapped_bitcoin_wbtc.png' },
  { symbol: 'BNB', name: 'BNB', address: '0xB8c77482e45F1F44dE1745F52C74426C631bDD52', logo: 'https://assets.coingecko.com/coins/images/825/small/bnb-icon2_2x.png' },
  { symbol: 'SOL', name: 'Solana', address: '0x0000000000000000000000000000000000000000', logo: 'https://assets.coingecko.com/coins/images/4128/small/solana.png' },
  { symbol: 'MATIC', name: 'Polygon', address: '0x0000000000000000000000000000000000000000', logo: 'https://assets.coingecko.com/coins/images/4713/small/polygon.png' },
  { symbol: 'DAI', name: 'Dai Stablecoin', address: '0x6B175474E89094C44Da98b954EesadcdEF9ce6CC', logo: 'https://assets.coingecko.com/coins/images/9956/small/DAI.png' },
];

// Mock market data
const MARKET_DATA = {
  'ETH-USDT': { price: '3,250.00', change: '+2.50%', high: '3,300.00', low: '3,200.00', volume: '1.2B' },
  'BTC-USDT': { price: '67,500.00', change: '+1.80%', high: '68,000.00', low: '66,500.00', volume: '5.8B' },
  'SOL-USDT': { price: '145.00', change: '+5.20%', high: '150.00', low: '138.00', volume: '800M' },
};

// Chart data
const CHART_DATA = [
  { time: '00:00', price: 3200 },
  { time: '04:00', price: 3220 },
  { time: '08:00', price: 3210 },
  { time: '12:00', price: 3240 },
  { time: '16:00', price: 3230 },
  { time: '20:00', price: 3250 },
];

export default function SwapPage() {
  const [fromToken, setFromToken] = useState<Token>(TOKENS[0]);
  const [toToken, setToToken] = useState<Token>(TOKENS[1]);
  const [fromAmount, setFromAmount] = useState<string>('');
  const [toAmount, setToAmount] = useState<string>('');
  const [slippage, setSlippage] = useState<string>('0.5');
  const [showTokenSelect, setShowTokenSelect] = useState<boolean>(false);
  const [selectingToken, setSelectingToken] = useState<'from' | 'to'>('from');
  const [isSwapping, setIsSwapping] = useState<boolean>(false);
  const [currentMarket, setCurrentMarket] = useState<string>('ETH-USDT');
  const [isLoadingPrice, setIsLoadingPrice] = useState<boolean>(false);
  const [priceData, setPriceData] = useState<Map<string, PriceData>>(new Map());

  // Fetch prices on mount
  useEffect(() => {
    const loadPrices = async () => {
      const prices = await getTokensWithPrices();
      const priceMap = new Map<string, PriceData>();
      prices.forEach(p => priceMap.set(p.symbol, p));
      setPriceData(priceMap);
    };
    loadPrices();
  }, []);

  // Calculate swap output using real prices
  useEffect(() => {
    const calculateOutput = async () => {
      if (fromAmount && fromToken && toToken && parseFloat(fromAmount) > 0) {
        setIsLoadingPrice(true);
        try {
          const quote = await calculateSwapOutput(fromToken.symbol, toToken.symbol, fromAmount);
          if (quote) {
            setToAmount(quote.toAmount);
          } else {
            // Fallback calculation if API fails
            const fromPrice = priceData.get(fromToken.symbol)?.price || 1;
            const toPrice = priceData.get(toToken.symbol)?.price || 1;
            const rate = fromPrice / toPrice;
            setToAmount((parseFloat(fromAmount) * rate).toFixed(toPrice < 1 ? 8 : 2));
          }
        } catch (error) {
          console.error('Price calculation error:', error);
        } finally {
          setIsLoadingPrice(false);
        }
      } else {
        setToAmount('');
      }
    };

    const debounceTimer = setTimeout(calculateOutput, 300);
    return () => clearTimeout(debounceTimer);
  }, [fromAmount, fromToken, toToken, priceData]);

  const handleSwap = async () => {
    if (!fromAmount || !toAmount) return;
    
    setIsSwapping(true);
    // Simulate swap
    await new Promise(resolve => setTimeout(resolve, 2000));
    setIsSwapping(false);
    setFromAmount('');
    setToAmount('');
    alert('Swap completed successfully!');
  };

  const handleTokenSelect = (token: Token) => {
    if (selectingToken === 'from') {
      setFromToken(token);
    } else {
      setToToken(token);
    }
    setShowTokenSelect(false);
  };

  const switchTokens = () => {
    setFromToken(toToken);
    setToToken(fromToken);
  };

  const market = MARKET_DATA[currentMarket as keyof typeof MARKET_DATA] || MARKET_DATA['ETH-USDT'];

  return (
    <div className="min-h-screen bg-gray-900 text-white">
      {/* Header */}
      <header className="bg-gray-800 border-b border-gray-700">
        <div className="max-w-7xl mx-auto px-4 py-4 flex items-center justify-between">
          <div className="flex items-center space-x-8">
            <h1 className="text-2xl font-bold text-orange-500">TigerSwap</h1>
            <nav className="hidden md:flex space-x-6">
              <a href="#" className="text-white hover:text-orange-500 transition">Swap</a>
              <a href="#" className="text-gray-400 hover:text-white transition">Pool</a>
              <a href="#" className="text-gray-400 hover:text-white transition">Farms</a>
              <a href="#" className="text-gray-400 hover:text-white transition">Bridge</a>
              <a href="#" className="text-gray-400 hover:text-white transition">Launchpad</a>
            </nav>
          </div>
          <div className="flex items-center space-x-4">
            <button className="p-2 hover:bg-gray-700 rounded-lg transition">
              <Settings className="w-5 h-5 text-gray-400" />
            </button>
            <button className="flex items-center space-x-2 bg-orange-500 hover:bg-orange-600 px-4 py-2 rounded-lg transition">
              <Wallet className="w-5 h-5" />
              <span>Connect Wallet</span>
            </button>
          </div>
        </div>
      </header>

      <main className="max-w-7xl mx-auto px-4 py-8">
        <div className="grid grid-cols-1 lg:grid-cols-3 gap-8">
          {/* Swap Interface */}
          <div className="lg:col-span-1">
            <div className="bg-gray-800 rounded-2xl p-6 border border-gray-700">
              <div className="flex items-center justify-between mb-6">
                <h2 className="text-xl font-semibold">Swap</h2>
                <div className="flex items-center space-x-2">
                  <span className="text-gray-400 text-sm">Slippage:</span>
                  <select 
                    value={slippage}
                    onChange={(e) => setSlippage(e.target.value)}
                    className="bg-gray-700 border border-gray-600 rounded px-2 py-1 text-sm"
                  >
                    <option value="0.1">0.1%</option>
                    <option value="0.5">0.5%</option>
                    <option value="1">1%</option>
                    <option value="3">3%</option>
                  </select>
                </div>
              </div>

              {/* From Token */}
              <div className="bg-gray-700 rounded-xl p-4 mb-4">
                <div className="flex justify-between mb-2">
                  <span className="text-gray-400 text-sm">From</span>
                  <span className="text-gray-400 text-sm">Balance: 10.5</span>
                </div>
                <div className="flex items-center justify-between">
                  <button 
                    onClick={() => { setSelectingToken('from'); setShowTokenSelect(true); }}
                    className="flex items-center space-x-2 bg-gray-600 hover:bg-gray-500 px-3 py-2 rounded-lg transition"
                  >
                    <img src={fromToken.logo} alt={fromToken.symbol} className="w-6 h-6 rounded-full" />
                    <span className="font-semibold">{fromToken.symbol}</span>
                    <ArrowDown className="w-4 h-4" />
                  </button>
                  <input
                    type="number"
                    value={fromAmount}
                    onChange={(e) => setFromAmount(e.target.value)}
                    placeholder="0.00"
                    className="bg-transparent text-2xl font-bold text-right w-40 outline-none"
                  />
                </div>
              </div>

              {/* Swap Button */}
              <div className="flex justify-center -my-2 relative z-10">
                <button 
                  onClick={switchTokens}
                  className="bg-gray-800 p-2 rounded-full border-4 border-gray-900 hover:bg-orange-500 transition"
                >
                  <ArrowDown className="w-5 h-5" />
                </button>
              </div>

              {/* To Token */}
              <div className="bg-gray-700 rounded-xl p-4 mb-6">
                <div className="flex justify-between mb-2">
                  <span className="text-gray-400 text-sm">To</span>
                  <span className="text-gray-400 text-sm">Balance: 50,000</span>
                </div>
                <div className="flex items-center justify-between">
                  <button 
                    onClick={() => { setSelectingToken('to'); setShowTokenSelect(true); }}
                    className="flex items-center space-x-2 bg-gray-600 hover:bg-gray-500 px-3 py-2 rounded-lg transition"
                  >
                    <img src={toToken.logo} alt={toToken.symbol} className="w-6 h-6 rounded-full" />
                    <span className="font-semibold">{toToken.symbol}</span>
                    <ArrowDown className="w-4 h-4" />
                  </button>
                  <input
                    type="number"
                    value={toAmount}
                    readOnly
                    placeholder="0.00"
                    className="bg-transparent text-2xl font-bold text-right w-40 outline-none"
                  />
                </div>
              </div>

              {/* Swap Details */}
              {fromAmount && toAmount && (
                <div className="bg-gray-700/50 rounded-lg p-4 mb-6 space-y-2">
                  <div className="flex justify-between text-sm">
                    <span className="text-gray-400">Rate</span>
                    <span>1 {fromToken.symbol} = {(parseFloat(toAmount) / parseFloat(fromAmount)).toFixed(4)} {toToken.symbol}</span>
                  </div>
                  <div className="flex justify-between text-sm">
                    <span className="text-gray-400">Price Impact</span>
                    <span className="text-green-500">0.1%</span>
                  </div>
                  <div className="flex justify-between text-sm">
                    <span className="text-gray-400">Max Slippage</span>
                    <span>{slippage}%</span>
                  </div>
                  <div className="flex justify-between text-sm">
                    <span className="text-gray-400">Network Fee</span>
                    <span>~$2.50</span>
                  </div>
                </div>
              )}

              {/* Swap Button */}
              <button
                onClick={handleSwap}
                disabled={!fromAmount || !toAmount || isSwapping}
                className={`w-full py-4 rounded-xl font-bold text-lg transition ${
                  !fromAmount || !toAmount || isSwapping
                    ? 'bg-gray-600 text-gray-400 cursor-not-allowed'
                    : 'bg-orange-500 hover:bg-orange-600 text-white'
                }`}
              >
                {isSwapping ? (
                  <span className="flex items-center justify-center">
                    <RefreshCw className="w-5 h-5 mr-2 animate-spin" />
                    Swapping...
                  </span>
                ) : !fromAmount ? 'Enter an amount' : 'Swap'}
              </button>
            </div>
          </div>

          {/* Market Info & Chart */}
          <div className="lg:col-span-2 space-y-6">
            {/* Market Ticker */}
            <div className="bg-gray-800 rounded-2xl p-6 border border-gray-700">
              <div className="flex items-center justify-between mb-4">
                <div className="flex items-center space-x-4">
                  <select 
                    value={currentMarket}
                    onChange={(e) => setCurrentMarket(e.target.value)}
                    className="bg-gray-700 border border-gray-600 rounded-lg px-4 py-2"
                  >
                    <option value="ETH-USDT">ETH/USDT</option>
                    <option value="BTC-USDT">BTC/USDT</option>
                    <option value="SOL-USDT">SOL/USDT</option>
                  </select>
                  <div>
                    <h3 className="text-2xl font-bold">{market.price}</h3>
                    <span className="text-green-500">{market.change}</span>
                  </div>
                </div>
                <div className="flex space-x-4 text-sm text-gray-400">
                  <div>
                    <span className="block">24h High</span>
                    <span className="text-white">{market.high}</span>
                  </div>
                  <div>
                    <span className="block">24h Low</span>
                    <span className="text-white">{market.low}</span>
                  </div>
                  <div>
                    <span className="block">24h Volume</span>
                    <span className="text-white">{market.volume}</span>
                  </div>
                </div>
              </div>

              {/* Chart */}
              <div className="h-64">
                <ResponsiveContainer width="100%" height="100%">
                  <LineChart data={CHART_DATA}>
                    <XAxis dataKey="time" stroke="#6B7280" fontSize={12} />
                    <YAxis stroke="#6B7280" fontSize={12} domain={['dataMin - 50', 'dataMax + 50']} />
                    <Tooltip 
                      contentStyle={{ backgroundColor: '#1F2937', border: 'none', borderRadius: '8px' }}
                      labelStyle={{ color: '#9CA3AF' }}
                    />
                    <Line 
                      type="monotone" 
                      dataKey="price" 
                      stroke="#F97316" 
                      strokeWidth={2} 
                      dot={false}
                    />
                  </LineChart>
                </ResponsiveContainer>
              </div>
            </div>

            {/* Order Book Preview */}
            <div className="bg-gray-800 rounded-2xl p-6 border border-gray-700">
              <h3 className="text-lg font-semibold mb-4">Order Book</h3>
              <div className="grid grid-cols-2 gap-4">
                {/* Asks */}
                <div>
                  <div className="grid grid-cols-3 text-sm text-gray-400 mb-2">
                    <span>Price</span>
                    <span className="text-right">Amount</span>
                    <span className="text-right">Total</span>
                  </div>
                  {[
                    { price: '3,251.00', amount: '80.5', total: '261,831' },
                    { price: '3,251.50', amount: '120.0', total: '390,180' },
                    { price: '3,252.00', amount: '200.0', total: '650,400' },
                    { price: '3,252.50', amount: '150.0', total: '487,875' },
                    { price: '3,253.00', amount: '180.0', total: '585,540' },
                  ].map((ask, i) => (
                    <div key={i} className="grid grid-cols-3 text-sm py-1">
                      <span className="text-red-400">{ask.price}</span>
                      <span className="text-right">{ask.amount}</span>
                      <span className="text-right text-gray-400">{ask.total}</span>
                    </div>
                  ))}
                </div>
                {/* Bids */}
                <div>
                  <div className="grid grid-cols-3 text-sm text-gray-400 mb-2">
                    <span>Price</span>
                    <span className="text-right">Amount</span>
                    <span className="text-right">Total</span>
                  </div>
                  {[
                    { price: '3,250.00', amount: '100.0', total: '325,000' },
                    { price: '3,249.50', amount: '150.0', total: '487,425' },
                    { price: '3,249.00', amount: '200.0', total: '649,800' },
                    { price: '3,248.50', amount: '180.0', total: '584,730' },
                    { price: '3,248.00', amount: '220.0', total: '714,560' },
                  ].map((bid, i) => (
                    <div key={i} className="grid grid-cols-3 text-sm py-1">
                      <span className="text-green-400">{bid.price}</span>
                      <span className="text-right">{bid.amount}</span>
                      <span className="text-right text-gray-400">{bid.total}</span>
                    </div>
                  ))}
                </div>
              </div>
            </div>
          </div>
        </div>
      </main>

      {/* Token Select Modal */}
      {showTokenSelect && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="bg-gray-800 rounded-2xl p-6 w-96 border border-gray-700">
            <div className="flex justify-between items-center mb-4">
              <h3 className="text-lg font-semibold">Select Token</h3>
              <button onClick={() => setShowTokenSelect(false)}>
                <X className="w-5 h-5" />
              </button>
            </div>
            <div className="relative mb-4">
              <Search className="absolute left-3 top-3 w-5 h-5 text-gray-400" />
              <input
                type="text"
                placeholder="Search token..."
                className="w-full bg-gray-700 border border-gray-600 rounded-lg pl-10 pr-4 py-2"
              />
            </div>
            <div className="max-h-64 overflow-y-auto space-y-2">
              {TOKENS.map((token) => (
                <button
                  key={token.address}
                  onClick={() => handleTokenSelect(token)}
                  className="w-full flex items-center space-x-3 p-2 hover:bg-gray-700 rounded-lg transition"
                >
                  <img src={token.logo} alt={token.symbol} className="w-8 h-8 rounded-full" />
                  <div className="text-left">
                    <div className="font-semibold">{token.symbol}</div>
                    <div className="text-xs text-gray-400">{token.name}</div>
                  </div>
                </button>
              ))}
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
