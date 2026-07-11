/**
 * TigerSwap Swap Component
 * Production-ready swap interface with real-time quotes
 * 
 * @author TigerSwap
 * @version 1.0.0 Production
 */

'use client';

import { useState, useEffect, useCallback } from 'react';
import { useAccount, useBalance, useWriteContract, useWaitForTransactionReceipt } from 'wagmi';
import { parseEther, formatEther } from 'viem';
import { Token, Quote, SwapRoute, TokenBalance } from './types';
import { useSwap, useTokenList, usePrice } from './hooks';
import { ArrowUpDown, Settings, ChevronDown, Zap, Shield, Activity } from 'lucide-react';
import clsx from 'clsx';

interface SwapProps {
  className?: string;
}

export default function Swap({ className }: SwapProps) {
  const { address, isConnected } = useAccount();
  
  // State
  const [fromToken, setFromToken] = useState<Token | null>(null);
  const [toToken, setToToken] = useState<Token | null>(null);
  const [fromAmount, setFromAmount] = useState('');
  const [toAmount, setToAmount] = useState('');
  const [slippage, setSlippage] = useState(0.5);
  const [isLoading, setIsLoading] = useState(false);
  const [isApproving, setIsApproving] = useState(false);
  const [showTokenSelector, setShowTokenSelector] = useState(false);
  const [selectorType, setSelectorType] = useState<'from' | 'to'>('from');
  const [quote, setQuote] = useState<Quote | null>(null);
  const [priceImpact, setPriceImpact] = useState(0);
  const [route, setRoute] = useState<SwapRoute | null>(null);

  // Hooks
  const { tokens, isLoading: tokensLoading } = useTokenList();
  const { getQuote, executeSwap, isLoading: swapLoading } = useSwap();
  const { data: priceData } = usePrice(fromToken?.address, toToken?.address);

  // Balance
  const { data: balance } = useBalance({
    address: address,
    token: fromToken?.address as `0x${string}`,
    enabled: !!fromToken && isConnected,
  });

  // Contract write
  const { writeContractAsync } = useWriteContract();
  const { isLoading: isConfirming, isSuccess } = useWaitForTransactionReceipt({
    hash: quote?.txHash as `0x${string}`,
  });

  // Set default tokens
  useEffect(() => {
    if (tokens.length > 0 && !fromToken) {
      setFromToken(tokens.find(t => t.symbol === 'ETH') || tokens[0]);
      setToToken(tokens.find(t => t.symbol === 'USDC') || tokens[1]);
    }
  }, [tokens, fromToken]);

  // Get quote when amount changes
  useEffect(() => {
    const fetchQuote = async () => {
      if (!fromToken || !toToken || !fromAmount || parseFloat(fromAmount) <= 0) {
        setToAmount('');
        setQuote(null);
        return;
      }

      try {
        setIsLoading(true);
        const result = await getQuote({
          fromToken: fromToken.address,
          toToken: toToken.address,
          amountIn: parseEther(fromAmount).toString(),
          slippage: slippage,
        });

        if (result) {
          setToAmount(formatEther(result.amountOut));
          setQuote(result);
          setPriceImpact(result.priceImpact);
          setRoute(result.route);
        }
      } catch (error) {
        console.error('Failed to get quote:', error);
      } finally {
        setIsLoading(false);
      }
    };

    const debounce = setTimeout(fetchQuote, 300);
    return () => clearTimeout(debounce);
  }, [fromToken, toToken, fromAmount, slippage, getQuote]);

  // Handle swap direction
  const handleSwitchTokens = useCallback(() => {
    setFromToken(toToken);
    setToToken(fromToken);
    setFromAmount(toAmount);
    setToAmount(fromAmount);
  }, [fromToken, toToken, fromAmount, toAmount]);

  // Handle token selection
  const handleSelectToken = (token: Token, type: 'from' | 'to') => {
    if (type === 'from') {
      setFromToken(token);
      if (toToken?.address === token.address) {
        setToToken(fromToken);
      }
    } else {
      setToToken(token);
      if (fromToken?.address === token.address) {
        setToToken(fromToken);
      }
    }
    setShowTokenSelector(false);
  };

  // Handle max button
  const handleSetMax = () => {
    if (balance) {
      setFromAmount(formatEther(balance.value));
    }
  };

  // Handle swap execution
  const handleSwap = async () => {
    if (!fromToken || !toToken || !fromAmount || !quote) return;

    try {
      setIsLoading(true);
      const txHash = await executeSwap({
        fromToken: fromToken.address,
        toToken: toToken.address,
        amountIn: parseEther(fromAmount).toString(),
        amountOutMinimum: quote.amountOut.toString(),
        slippage: slippage,
      });

      if (txHash) {
        console.log('Transaction submitted:', txHash);
      }
    } catch (error) {
      console.error('Swap failed:', error);
    } finally {
      setIsLoading(false);
    }
  };

  // Calculate minimum received
  const minReceived = quote 
    ? parseFloat(formatEther(quote.amountOut)) * (1 - slippage / 100)
    : 0;

  return (
    <div className={clsx('glass-card p-6 max-w-md mx-auto', className)}>
      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <h2 className="text-2xl font-bold gradient-text">Swap</h2>
        <div className="flex items-center gap-2">
          <button className="p-2 hover:bg-slate-800 rounded-lg transition-colors" title="Settings">
            <Settings className="w-5 h-5 text-slate-400" />
          </button>
          <button className="p-2 hover:bg-slate-800 rounded-lg transition-colors" title="Price Protection">
            <Shield className="w-5 h-5 text-slate-400" />
          </button>
        </div>
      </div>

      {/* From Token */}
      <div className="space-y-2">
        <div className="flex items-center justify-between text-sm text-slate-400">
          <span>You pay</span>
          {isConnected && balance && fromToken && (
            <button 
              onClick={handleSetMax}
              className="text-orange-400 hover:text-orange-300 transition-colors"
            >
              Balance: {parseFloat(formatEther(balance.value)).toFixed(4)} {fromToken.symbol}
            </button>
          )}
        </div>
        
        <div className="relative">
          <button
            onClick={() => { setSelectorType('from'); setShowTokenSelector(true); }}
            className="absolute left-3 top-1/2 -translate-y-1/2 flex items-center gap-2 px-3 py-2 bg-slate-800 rounded-lg hover:bg-slate-700 transition-colors"
          >
            {fromToken && (
              <>
                <div className="w-6 h-6 rounded-full bg-gradient-to-br from-orange-400 to-amber-500 flex items-center justify-center text-xs font-bold">
                  {fromToken.symbol[0]}
                </div>
                <span className="font-semibold">{fromToken.symbol}</span>
              </>
            )}
            <ChevronDown className="w-4 h-4 text-slate-400" />
          </button>
          
          <input
            type="number"
            value={fromAmount}
            onChange={(e) => setFromAmount(e.target.value)}
            placeholder="0.00"
            className="w-full bg-transparent text-right text-3xl font-bold py-4 pr-4 pl-36 focus:outline-none placeholder:text-slate-600"
          />
        </div>
      </div>

      {/* Switch Button */}
      <div className="flex justify-center -my-3 relative z-10">
        <button
          onClick={handleSwitchTokens}
          className="p-2 bg-slate-800 border-4 border-slate-900 rounded-full hover:bg-slate-700 transition-all hover:scale-110"
        >
          <ArrowUpDown className="w-5 h-5 text-orange-400" />
        </button>
      </div>

      {/* To Token */}
      <div className="space-y-2">
        <div className="flex items-center justify-between text-sm text-slate-400">
          <span>You receive</span>
        </div>
        
        <div className="relative">
          <button
            onClick={() => { setSelectorType('to'); setShowTokenSelector(true); }}
            className="absolute left-3 top-1/2 -translate-y-1/2 flex items-center gap-2 px-3 py-2 bg-slate-800 rounded-lg hover:bg-slate-700 transition-colors"
          >
            {toToken && (
              <>
                <div className="w-6 h-6 rounded-full bg-gradient-to-br from-blue-400 to-purple-500 flex items-center justify-center text-xs font-bold">
                  {toToken.symbol[0]}
                </div>
                <span className="font-semibold">{toToken.symbol}</span>
              </>
            )}
            <ChevronDown className="w-4 h-4 text-slate-400" />
          </button>
          
          <div className="w-full text-right text-3xl font-bold py-4 pr-4 pl-36 text-slate-300">
            {isLoading ? (
              <span className="animate-pulse">...</span>
            ) : toAmount ? (
              toAmount
            ) : (
              '0.00'
            )}
          </div>
        </div>
      </div>

      {/* Rate & Price Impact */}
      {quote && fromToken && toToken && (
        <div className="mt-4 space-y-2 text-sm">
          <div className="flex items-center justify-between text-slate-400">
            <span>Rate</span>
            <span>1 {fromToken.symbol} = {parseFloat(toAmount) / parseFloat(fromAmount)} {toToken.symbol}</span>
          </div>
          <div className="flex items-center justify-between text-slate-400">
            <span>Price Impact</span>
            <span className={clsx(
              priceImpact > 5 ? 'text-red-400' : priceImpact > 1 ? 'text-yellow-400' : 'text-green-400'
            )}>
              {priceImpact.toFixed(2)}%
            </span>
          </div>
          <div className="flex items-center justify-between text-slate-400">
            <span>Min. Received</span>
            <span>{minReceived.toFixed(6)} {toToken.symbol}</span>
          </div>
          <div className="flex items-center justify-between text-slate-400">
            <span>Gas Fee</span>
            <span>~${quote.gasFeeUSD.toFixed(2)}</span>
          </div>
        </div>
      )}

      {/* Swap Button */}
      <button
        onClick={handleSwap}
        disabled={!fromAmount || !toAmount || isLoading || isConfirming || !isConnected}
        className="w-full mt-6 btn-primary flex items-center justify-center gap-2"
      >
        {isLoading || isConfirming ? (
          <Activity className="w-5 h-5 animate-spin" />
        ) : !isConnected ? (
          'Connect Wallet'
        ) : !fromAmount || !toAmount ? (
          'Enter Amount'
        ) : (
          <>
            <Zap className="w-5 h-5" />
            Swap
          </>
        )}
      </button>

      {/* Transaction Status */}
      {isSuccess && (
        <div className="mt-4 p-4 bg-green-900/30 border border-green-500/30 rounded-xl">
          <p className="text-green-400 text-center">Transaction successful!</p>
        </div>
      )}

      {/* Token Selector Modal */}
      {showTokenSelector && (
        <TokenSelector
          tokens={tokens}
          selectedToken={selectorType === 'from' ? fromToken : toToken}
          onSelect={(token) => handleSelectToken(token, selectorType)}
          onClose={() => setShowTokenSelector(false)}
        />
      )}
    </div>
  );
}

// Token Selector Component
function TokenSelector({
  tokens,
  selectedToken,
  onSelect,
  onClose,
}: {
  tokens: Token[];
  selectedToken: Token | null;
  onSelect: (token: Token) => void;
  onClose: () => void;
}) {
  const [search, setSearch] = useState('');

  const filteredTokens = tokens.filter(
    (token) =>
      token.symbol.toLowerCase().includes(search.toLowerCase()) ||
      token.name.toLowerCase().includes(search.toLowerCase())
  );

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/50 backdrop-blur-sm">
      <div className="glass-card w-full max-w-md max-h-[80vh] overflow-hidden">
        <div className="p-4 border-b border-slate-700">
          <div className="flex items-center justify-between mb-4">
            <h3 className="text-lg font-semibold">Select Token</h3>
            <button onClick={onClose} className="text-slate-400 hover:text-white">
              ✕
            </button>
          </div>
          <input
            type="text"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Search tokens..."
            className="w-full px-4 py-3 bg-slate-800 border border-slate-700 rounded-xl focus:outline-none focus:border-orange-500"
          />
        </div>
        
        <div className="overflow-y-auto max-h-96">
          {filteredTokens.map((token) => (
            <button
              key={token.address}
              onClick={() => onSelect(token)}
              className={clsx(
                'w-full flex items-center gap-3 px-4 py-3 hover:bg-slate-800 transition-colors',
                selectedToken?.address === token.address && 'bg-slate-800'
              )}
            >
              <div className="w-10 h-10 rounded-full bg-gradient-to-br from-orange-400 to-amber-500 flex items-center justify-center text-lg font-bold">
                {token.symbol[0]}
              </div>
              <div className="text-left">
                <div className="font-semibold">{token.symbol}</div>
                <div className="text-sm text-slate-400">{token.name}</div>
              </div>
              <div className="ml-auto text-right">
                <div className="font-medium">${token.price.toFixed(2)}</div>
              </div>
            </button>
          ))}
        </div>
      </div>
    </div>
  );
}
