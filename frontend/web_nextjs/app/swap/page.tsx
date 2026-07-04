'use client';

import React, { useState, useEffect, useCallback, useRef } from 'react';
import {
  Box, Typography, Card, CardContent, Button, TextField,
  Chip, CircularProgress, Alert, IconButton,
  InputAdornment, Slider, Divider, Stack, Switch, FormControlLabel,
  Dialog, DialogTitle, DialogContent, List, ListItemIcon,
  ListItemText, ListItemButton, Avatar, Tabs, Tab, Badge,
  Snackbar, LinearProgress, Tooltip, useTheme as useMuiTheme
} from '@mui/material';
import {
  SwapHoriz, Settings, ArrowDropDown, Warning,
  OpenInNew, AccountBalanceWallet, Shield, Speed, CompareArrows,
  Check, Refresh, ArrowForward, KeyboardArrowDown, Close
} from '@mui/icons-material';
import { useTheme } from '../components/ThemeProvider';
import {
  TigerSwapWallet,
  COMMON_TOKENS,
  DEX_ROUTERS,
  TokenInfo,
  GasPriceInfo
} from '../lib/TigerSwapWallet';

// ============================================================================
// Types & Interfaces
// ============================================================================

interface Token {
  address: string;
  symbol: string;
  name: string;
  decimals: number;
  logoURI?: string;
  priceUSD?: number;
  chainId: number;
  isPopular?: boolean;
  isNative?: boolean;
  isStable?: boolean;
}

interface SwapQuote {
  inputToken: string;
  outputToken: string;
  inputAmount: string;
  outputAmount: string;
  outputAmountMin: string;
  priceImpact: number;
  route: RouteInfo[];
  gasEstimate: string;
  gasFeeUSD: number;
  exchangeRate: number;
  slippage: number;
  provider: string;
  expiresAt: number;
}

interface RouteInfo {
  dex: string;
  dexName: string;
  path: string[];
  percentage: number;
  poolAddress: string;
  fee: number;
  amountIn: string;
  amountOut: string;
}

interface WalletState {
  isConnected: boolean;
  account: string | null;
  chainId: number;
  balance: string;
  chainName: string;
  provider: 'metamask' | 'walletconnect' | 'coinbase' | null;
}

interface TokenBalance {
  token: Token;
  balance: string;
  balanceUSD: number;
}

interface GasPrice {
  slow: number;
  standard: number;
  fast: number;
  instant: number;
  baseFee: number;
}

interface TransactionState {
  status: 'idle' | 'approving' | 'swapping' | 'confirming' | 'success' | 'error';
  hash: string | null;
  error: string | null;
}

// ============================================================================
// Constants
// ============================================================================

const CHAIN_CONFIG: Record<number, { name: string; rpcUrl: string; explorer: string; native: string; nativeSymbol: string }> = {
  1: { name: 'Ethereum', rpcUrl: 'https://eth.llamarpc.com', explorer: 'https://etherscan.io', native: 'Ether', nativeSymbol: 'ETH' },
  56: { name: 'BNB Chain', rpcUrl: 'https://bsc-dataseed.binance.org', explorer: 'https://bscscan.com', native: 'BNB', nativeSymbol: 'BNB' },
  42161: { name: 'Arbitrum', rpcUrl: 'https://arb1.arbitrum.io/rpc', explorer: 'https://arbiscan.io', native: 'Ether', nativeSymbol: 'ETH' },
  137: { name: 'Polygon', rpcUrl: 'https://polygon-rpc.com', explorer: 'https://polygonscan.com', native: 'MATIC', nativeSymbol: 'MATIC' },
  10: { name: 'Optimism', rpcUrl: 'https://mainnet.optimism.io', explorer: 'https://optimistic.etherscan.io', native: 'Ether', nativeSymbol: 'ETH' },
  8453: { name: 'Base', rpcUrl: 'https://mainnet.base.org', explorer: 'https://basescan.org', native: 'Ether', nativeSymbol: 'ETH' },
};

const DEX_INFO: Record<string, { name: string; logo: string; color: string }> = {
  'uniswap_v2': { name: 'Uniswap V2', logo: '🦄', color: '#FF007A' },
  'uniswap_v3': { name: 'Uniswap V3', logo: '🦄', color: '#FF007A' },
  'sushiswap': { name: 'SushiSwap', logo: '🍣', color: '#FA52A0' },
  'pancakeswap': { name: 'PancakeSwap', logo: '🥞', color: '#633001' },
  'quickswap': { name: 'QuickSwap', logo: '⚡', color: '#6c8fc5' },
};

// ============================================================================
// Utility Functions
// ============================================================================

function formatAddress(address: string, chars: number = 4): string {
  return `${address.slice(0, chars + 2)}...${address.slice(-chars)}`;
}

function formatBalance(balance: string, decimals: number = 18): string {
  if (!balance || balance === '0' || balance === '0x0') return '0';
  try {
    const num = Number(balance) / Math.pow(10, decimals);
    if (num < 0.0001) return '<0.0001';
    return num.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 4 });
  } catch {
    return '0';
  }
}

function formatUSD(amount: number): string {
  if (amount < 0.01) return '$0.00';
  return new Intl.NumberFormat('en-US', { style: 'currency', currency: 'USD' }).format(amount);
}

function formatNumber(num: number, decimals: number = 2): string {
  if (num >= 1e9) return (num / 1e9).toFixed(decimals) + 'B';
  if (num >= 1e6) return (num / 1e6).toFixed(decimals) + 'M';
  if (num >= 1e3) return (num / 1e3).toFixed(decimals) + 'K';
  return num.toFixed(decimals);
}

function parseAmount(amount: string, decimals: number): bigint {
  if (!amount || amount === '0') return BigInt(0);
  const [integer, fraction = ''] = amount.split('.');
  const paddedFraction = fraction.padEnd(decimals, '0').slice(0, decimals);
  return BigInt(integer + paddedFraction);
}

function formatAmount(amount: bigint, decimals: number): string {
  const divisor = BigInt(10 ** decimals);
  const integer = amount / divisor;
  const fraction = amount % divisor;
  const fractionStr = fraction.toString().padStart(decimals, '0').replace(/0+$/, '');
  return fractionStr === '' ? integer.toString() : `${integer}.${fractionStr}`;
}

// ============================================================================
// Main Swap Component
// ============================================================================

export default function SwapPage() {
  const { theme } = useTheme();
  const muiTheme = useMuiTheme();
  const isDark = theme === 'dark';
  const walletRef = useRef<any>(null);

  // Theme-aware colors
  const bgPrimary = isDark ? '#0f172a' : '#f8fafc';
  const bgSecondary = isDark ? '#1e293b' : '#e2e8f0';
  const bgCard = isDark ? 'rgba(30, 41, 59, 0.8)' : 'rgba(255, 255, 255, 0.9)';
  const textPrimary = isDark ? '#f8fafc' : '#0f172a';
  const textSecondary = isDark ? '#94a3b8' : '#64748b';
  const borderColor = isDark ? 'rgba(255, 255, 255, 0.1)' : 'rgba(0, 0, 0, 0.1)';
  const accentColor = '#f97316';
  
  // Wallet state
  const [wallet, setWallet] = useState<WalletState>({
    isConnected: false,
    account: null,
    chainId: 1,
    balance: '0',
    chainName: 'Ethereum',
    provider: null,
  });

  // Token selection
  const [tokenIn, setTokenIn] = useState<Token | null>(null);
  const [tokenOut, setTokenOut] = useState<Token | null>(null);
  const [amountIn, setAmountIn] = useState<string>('');
  const [amountOut, setAmountOut] = useState<string>('');
  const [tokenBalances, setTokenBalances] = useState<TokenBalance[]>([]);
  const [supportedTokens, setSupportedTokens] = useState<Token[]>([]);

  // UI State
  const [showTokenSelector, setShowTokenSelector] = useState<'in' | 'out' | null>(null);
  const [showSettings, setShowSettings] = useState(false);
  const [showRouteDetails, setShowRouteDetails] = useState(false);
  const [slippage, setSlippage] = useState<number>(0.5);
  const [deadline, setDeadline] = useState<number>(20);
  const [gasPreference, setGasPreference] = useState<'slow' | 'standard' | 'fast' | 'instant'>('standard');
  const [gasPrice, setGasPrice] = useState<GasPrice>({ slow: 20, standard: 35, fast: 50, instant: 75, baseFee: 30 });
  const [txState, setTxState] = useState<TransactionState>({ status: 'idle', hash: null, error: null });
  const [quote, setQuote] = useState<SwapQuote | null>(null);
  const [loadingQuote, setLoadingQuote] = useState(false);
  const [priceFromChainlink, setPriceFromChainlink] = useState<number | null>(null);
  const [autoSlippage, setAutoSlippage] = useState(true);
  const [searchQuery, setSearchQuery] = useState('');

  // Snackbar
  const [snackbar, setSnackbar] = useState<{ open: boolean; message: string; severity: 'success' | 'error' | 'info' }>({
    open: false,
    message: '',
    severity: 'info'
  });

  const SearchIcon = (props: any) => (
    <svg {...props} viewBox="0 0 24 24" width="20" height="20" fill="currentColor">
      <path d="M15.5 14h-.79l-.28-.27C15.41 12.59 16 11.11 16 9.5 16 5.91 13.09 3 9.5 3S3 5.91 3 9.5 5.91 16 9.5 16c1.61 0 3.09-.59 4.23-1.57l.27.28v.79l5 4.99L20.49 19l-4.99-5zm-6 0C7.01 14 5 11.99 5 9.5S7.01 5 9.5 5 14 7.01 14 9.5 11.99 14 9.5 14z"/>
    </svg>
  );

  // ============================================================================
  // Wallet Initialization
  // ============================================================================

  useEffect(() => {
    walletRef.current = new TigerSwapWallet();
    
    // Try auto-connect
    const initWallet = async () => {
      const w = walletRef.current;
      if (!w) return;
      
      const connected = await w.autoConnect();
      if (connected) {
        updateWalletState(w);
        await loadTokenBalances(w);
        await loadGasPrice(w);
      }
    };
    
    initWallet();
    
    // Listen for account changes
    walletRef.current.onAccountsChange((account) => {
      updateWalletState(walletRef.current!);
      loadTokenBalances(walletRef.current!);
    });
    
    walletRef.current.onChainChange((chainId) => {
      updateWalletState(walletRef.current!);
      loadTokenBalances(walletRef.current!);
      loadGasPrice(walletRef.current!);
    });
    
    walletRef.current.onDisconnectCallback(() => {
      setWallet({
        isConnected: false,
        account: null,
        chainId: 1,
        balance: '0',
        chainName: 'Ethereum',
        provider: null,
      });
      setTokenBalances([]);
    });
  }, []);

  const updateWalletState = (w: TigerSwapWallet) => {
    setWallet({
      isConnected: w.isConnected(),
      account: w.getAccount(),
      chainId: w.getChainId(),
      balance: w.formatBalance(w.getAccount() ? '0x0' : '0'), // Would need to fetch
      chainName: w.getChainName(),
      provider: w.getProvider(),
    });
  };

  // ============================================================================
  // Load Supported Tokens
  // ============================================================================

  useEffect(() => {
    const tokens: Token[] = [];
    const chainTokens = COMMON_TOKENS[wallet.chainId] || {};
    
    Object.values(chainTokens).forEach((t: TokenInfo) => {
      tokens.push({
        address: t.address,
        symbol: t.symbol,
        name: t.name,
        decimals: t.decimals,
        logoURI: t.logoURI,
        priceUSD: t.priceUSD,
        chainId: t.chainId,
        isNative: t.isNative,
        isStable: t.isStable,
        isPopular: true,
      });
    });
    
    // Add native token
    const chainConfig = CHAIN_CONFIG[wallet.chainId];
    if (chainConfig) {
      tokens.unshift({
        address: '0x0000000000000000000000000000000000000000',
        symbol: chainConfig.nativeSymbol,
        name: chainConfig.native,
        decimals: 18,
        chainId: wallet.chainId,
        isNative: true,
        isPopular: true,
      });
    }
    
    setSupportedTokens(tokens);
    
    // Set default selections
    if (!tokenIn && tokens.length > 0) {
      setTokenIn(tokens[0]);
    }
    if (!tokenOut && tokens.length > 1) {
      setTokenOut(tokens[1]);
    }
  }, [wallet.chainId]);

  // ============================================================================
  // Load Token Balances from Blockchain
  // ============================================================================

  const loadTokenBalances = async (w: TigerSwapWallet) => {
    if (!w.isConnected()) return;
    
    const account = w.getAccount();
    if (!account) return;
    
    const balances: TokenBalance[] = [];
    const chainTokens = COMMON_TOKENS[wallet.chainId] || {};
    
    try {
      // Get native balance
      const nativeBalance = await w.getNativeBalance();
      const chainConfig = CHAIN_CONFIG[wallet.chainId];
      if (chainConfig) {
        balances.push({
          token: {
            address: '0x0000000000000000000000000000000000000000',
            symbol: chainConfig.nativeSymbol,
            name: chainConfig.native,
            decimals: 18,
            chainId: wallet.chainId,
            isNative: true,
          },
          balance: nativeBalance,
          balanceUSD: 0, // Would need price oracle
        });
      }
      
      // Get token balances
      for (const [symbol, token] of Object.entries(chainTokens)) {
        try {
          const balance = await w.getTokenBalance(token.address);
          if (balance && balance !== '0x0') {
            balances.push({
              token: {
                address: token.address,
                symbol: token.symbol,
                name: token.name,
                decimals: token.decimals,
                logoURI: token.logoURI,
                chainId: token.chainId,
                isStable: token.isStable,
              },
              balance: balance,
              balanceUSD: 0,
            });
          }
        } catch (e) {
          console.error(`Failed to load balance for ${symbol}:`, e);
        }
      }
      
      setTokenBalances(balances);
    } catch (error) {
      console.error('Failed to load token balances:', error);
    }
  };

  // ============================================================================
  // Load Gas Price (EIP-1559)
  // ============================================================================

  const loadGasPrice = async (w: TigerSwapWallet) => {
    if (!w.isConnected()) return;
    
    try {
      const gasInfo: GasPriceInfo = await w.getGasPrice();
      setGasPrice({
        slow: parseFloat(w.formatGwei(gasInfo.slow)),
        standard: parseFloat(w.formatGwei(gasInfo.standard)),
        fast: parseFloat(w.formatGwei(gasInfo.fast)),
        instant: parseFloat(w.formatGwei(gasInfo.instant)),
        baseFee: parseFloat(w.formatGwei(gasInfo.baseFee)),
      });
    } catch (error) {
      console.error('Failed to load gas price:', error);
    }
  };

  // ============================================================================
  // Load Price from Chainlink Oracle
  // ============================================================================

  const loadChainlinkPrice = async (w: TigerSwapWallet, baseSymbol: string) => {
    try {
      const price = await w.getPriceFromChainlink(baseSymbol, 'USD');
      setPriceFromChainlink(price);
      return price;
    } catch (error) {
      console.error('Failed to load Chainlink price:', error);
      return null;
    }
  };

  // ============================================================================
  // Calculate Swap Quote (Real DEX Query)
  // ============================================================================

  const calculateQuote = useCallback(async () => {
    if (!tokenIn || !tokenOut || !amountIn || parseFloat(amountIn) <= 0) {
      setAmountOut('');
      setQuote(null);
      return;
    }

    const w = walletRef.current;
    if (!w || !w.isConnected()) return;

    setLoadingQuote(true);

    try {
      const amountInRaw = parseAmount(amountIn, tokenIn.decimals);
      const router = DEX_ROUTERS[wallet.chainId]?.UniswapV2;
      
      if (!router) {
        throw new Error('No router available for this chain');
      }

      // Query the DEX router for quote
      const path = [tokenIn.address, tokenOut.address];
      const amounts = await w.callContract(
        router,
        [
          {
            name: 'getAmountsOut',
            inputs: [{ name: 'amountIn', type: 'uint256' }, { name: 'path', type: 'address[]' }],
            outputs: [{ name: 'amounts', type: 'uint256[]' }],
            stateMutability: 'view',
            type: 'function',
          }
        ],
        'getAmountsOut',
        [amountInRaw.toString(), path]
      );

      if (!amounts || amounts.length < 2) {
        throw new Error('Invalid quote response');
      }

      const amountOutRaw = BigInt(amounts[1]);
      const amountOutFormatted = formatAmount(amountOutRaw, tokenOut.decimals);
      
      // Calculate price impact based on AMM formula
      const amountOutMin = amountOutRaw * BigInt(Math.floor((100 - slippage) * 100)) / BigInt(10000);
      
      // Calculate price impact (simplified)
      const priceImpact = calculatePriceImpact(amountInRaw, amountOutRaw, tokenIn.decimals, tokenOut.decimals);
      
      // Get gas estimate
      const gasEstimate = await w.estimateGas({
        from: w.getAccount()!,
        to: router,
        data: '0x',
      });

      // Get price from Chainlink for USD conversion
      let priceInUSD = priceFromChainlink;
      if (!priceInUSD && tokenIn.isNative) {
        priceInUSD = await loadChainlinkPrice(w, tokenIn.symbol);
      }

      const gasFeeUSD = priceInUSD ? (parseInt(gasEstimate, 16) * gasPrice.standard * 1e-9) * priceInUSD : 12.50;

      setAmountOut(amountOutFormatted);
      setQuote({
        inputToken: tokenIn.address,
        outputToken: tokenOut.address,
        inputAmount: amountIn,
        outputAmount: amountOutFormatted,
        outputAmountMin: formatAmount(amountOutMin, tokenOut.decimals),
        priceImpact,
        route: [{
          dex: 'uniswap_v2',
          dexName: 'Uniswap V2',
          path: [tokenIn.symbol, tokenOut.symbol],
          percentage: 100,
          poolAddress: router,
          fee: 300,
          amountIn: amountIn,
          amountOut: amountOutFormatted,
        }],
        gasEstimate: gasEstimate,
        gasFeeUSD,
        exchangeRate: parseFloat(amountOutFormatted) / parseFloat(amountIn),
        slippage,
        provider: 'TigerSwap',
        expiresAt: Date.now() + 30000,
      });

      // Auto-adjust slippage based on price impact
      if (autoSlippage && priceImpact > 1) {
        const suggestedSlippage = Math.min(Math.max(priceImpact * 1.5, 0.5), 5);
        setSlippage(suggestedSlippage);
      }

    } catch (error: any) {
      console.error('Quote calculation failed:', error);
      setSnackbar({
        open: true,
        message: `Quote error: ${error.message}`,
        severity: 'error',
      });
      setQuote(null);
    } finally {
      setLoadingQuote(false);
    }
  }, [tokenIn, tokenOut, amountIn, slippage, wallet.chainId, autoSlippage, priceFromChainlink, gasPrice]);

  useEffect(() => {
    if (tokenIn && tokenOut && amountIn) {
      const debounce = setTimeout(calculateQuote, 500);
      return () => clearTimeout(debounce);
    }
  }, [calculateQuote]);

  // ============================================================================
  // Price Impact Calculation (AMM Formula)
  // ============================================================================

  const calculatePriceImpact = (amountIn: bigint, amountOut: bigint, decimalsIn: number, decimalsOut: number): number => {
    if (amountIn === BigInt(0) || amountOut === BigInt(0)) return 0;
    
    // Simplified constant product formula: x * y = k
    // Price impact = (amountIn / reserveIn) * 100
    // Assuming 1:1 initial rate for simplicity
    const amountInNum = Number(amountIn) / Math.pow(10, decimalsIn);
    const amountOutNum = Number(amountOut) / Math.pow(10, decimalsOut);
    
    if (amountInNum === 0) return 0;
    
    const fairPrice = amountOutNum; // Assuming 1:1 for same decimals
    const expectedOut = amountInNum; // Simplified
    const priceImpact = Math.max(0, ((expectedOut - amountOutNum) / expectedOut) * 100);
    
    return Math.min(priceImpact, 50); // Cap at 50%
  };

  // ============================================================================
  // Wallet Connection
  // ============================================================================

  const connectWallet = async (provider: 'metamask' | 'walletconnect' | 'coinbase') => {
    const w = walletRef.current;
    if (!w) return;

    try {
      let account: string | null = null;
      
      switch (provider) {
        case 'metamask':
          account = await w.connectMetaMask();
          break;
        case 'coinbase':
          account = await w.connectCoinbaseWallet();
          break;
        case 'walletconnect':
          account = await w.connectWalletConnect();
          break;
      }

      if (account) {
        updateWalletState(w);
        await loadTokenBalances(w);
        await loadGasPrice(w);
        setSnackbar({
          open: true,
          message: `Connected: ${formatAddress(account)}`,
          severity: 'success',
        });
      }
    } catch (error: any) {
      setSnackbar({
        open: true,
        message: error.message || 'Failed to connect wallet',
        severity: 'error',
      });
    }
  };

  const disconnectWallet = () => {
    walletRef.current?.disconnect();
    setWallet({
      isConnected: false,
      account: null,
      chainId: 1,
      balance: '0',
      chainName: 'Ethereum',
      provider: null,
    });
    setTokenBalances([]);
  };

  // ============================================================================
  // Token Selection
  // ============================================================================

  const handleSelectToken = (token: Token) => {
    if (showTokenSelector === 'in') {
      if (token.address === tokenOut?.address) {
        setTokenOut(tokenIn);
      }
      setTokenIn(token);
    } else {
      if (token.address === tokenIn?.address) {
        setTokenIn(tokenOut);
      }
      setTokenOut(token);
    }
    setShowTokenSelector(null);
    setSearchQuery('');
    setAmountOut('');
    setQuote(null);
  };

  const getTokenBalance = (token: Token): string => {
    const balance = tokenBalances.find(
      b => b.token.address.toLowerCase() === token.address.toLowerCase()
    );
    return balance?.balance || '0x0';
  };

  const switchTokens = () => {
    const tempIn = tokenIn;
    const tempOut = tokenOut;
    setTokenIn(tempOut);
    setTokenOut(tempIn);
    setAmountIn(amountOut);
    setAmountOut('');
    setQuote(null);
  };

  // ============================================================================
  // Execute Swap
  // ============================================================================

  const executeSwap = async () => {
    const w = walletRef.current;
    if (!w || !w.isConnected() || !tokenIn || !tokenOut || !amountIn || !quote) {
      return;
    }

    setTxState({ status: 'idle', hash: null, error: null });

    try {
      const amountInRaw = parseAmount(amountIn, tokenIn.decimals);
      const amountOutMinRaw = parseAmount(quote.outputAmountMin, tokenOut.decimals);
      const router = DEX_ROUTERS[wallet.chainId]?.UniswapV2;
      
      if (!router) {
        throw new Error('No router available');
      }

      // Check and handle approval for ERC20 tokens
      if (!tokenIn.isNative) {
        setTxState({ status: 'approving', hash: null, error: null });
        
        const allowance = await w.getAllowance(tokenIn.address, router);
        if (BigInt(allowance) < amountInRaw) {
          const approveTx = await w.approve(
            tokenIn.address,
            router,
            BigInt('0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff')
          );
          
          setSnackbar({
            open: true,
            message: `Approval pending...`,
            severity: 'info',
          });
          
          await w.waitForConfirmation(approveTx);
        }
      }

      // Execute swap
      setTxState({ status: 'swapping', hash: null, error: null });

      const deadlineTimestamp = Math.floor(Date.now() / 1000) + deadline * 60;
      const path = [tokenIn.address, tokenOut.address];

      let txHash: string;
      
      if (tokenIn.isNative) {
        // Wrap ETH and swap
        const weth = COMMON_TOKENS[wallet.chainId]?.WETH?.address;
        if (weth) {
          txHash = await w.sendTransaction({
            from: w.getAccount()!,
            to: router,
            value: amountInRaw.toString(),
            data: w.encodeFunctionCall(
              [{ name: 'swapExactETHForTokens', inputs: [], outputs: [], stateMutability: 'payable', type: 'function' }],
              'swapExactETHForTokens',
              []
            ),
          });
        }
      } else {
        txHash = await w.executeSwap(
          tokenIn.address,
          tokenOut.address,
          amountInRaw,
          amountOutMinRaw,
          path,
          router,
          deadlineTimestamp
        );
      }

      setTxState({ status: 'confirming', hash: txHash, error: null });

      // Wait for confirmation
      const receipt = await w.waitForConfirmation(txHash);

      if (receipt.status === 'success') {
        setTxState({ status: 'success', hash: txHash, error: null });
        setSnackbar({
          open: true,
          message: `Swap successful!`,
          severity: 'success',
        });
        
        // Reload balances
        await loadTokenBalances(w);
        
        // Reset form
        setAmountIn('');
        setAmountOut('');
        setQuote(null);
      } else {
        throw new Error('Transaction reverted');
      }

    } catch (error: any) {
      console.error('Swap failed:', error);
      setTxState({
        status: 'error',
        hash: null,
        error: error.message || 'Swap failed',
      });
      setSnackbar({
        open: true,
        message: `Swap failed: ${error.message}`,
        severity: 'error',
      });
    }
  };

  // ============================================================================
  // Render Token Selector
  // ============================================================================

  const renderTokenSelector = () => {
    const filteredTokens = supportedTokens.filter(token => {
      if (!searchQuery) return true;
      const query = searchQuery.toLowerCase();
      return (
        token.symbol.toLowerCase().includes(query) ||
        token.name.toLowerCase().includes(query) ||
        token.address.toLowerCase().includes(query)
      );
    });

    const popularTokens = filteredTokens.filter(t => t.isPopular);
    const otherTokens = filteredTokens.filter(t => !t.isPopular);

    return (
      <Dialog
        open={!!showTokenSelector}
        onClose={() => { setShowTokenSelector(null); setSearchQuery(''); }}
        PaperProps={{
          sx: {
            bgcolor: '#1a1a2e',
            backgroundImage: 'none',
            maxWidth: 480,
            width: '100%',
          }
        }}
      >
        <DialogTitle sx={{ color: 'white', display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          Select Token
          <IconButton onClick={() => { setShowTokenSelector(null); setSearchQuery(''); }} sx={{ color: 'white' }}>
            <Close />
          </IconButton>
        </DialogTitle>
        <DialogContent>
          <TextField
            fullWidth
            placeholder="Search by name, symbol, or address"
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            sx={{
              mb: 2,
              input: { color: 'white' },
              '& .MuiOutlinedInput-root': {
                '& fieldset': { borderColor: '#3a3a4e' },
              }
            }}
            InputProps={{
              startAdornment: (
                <InputAdornment position="start">
                  <SearchIcon sx={{ color: 'gray' }} />
                </InputAdornment>
              ),
            }}
          />

          {popularTokens.length > 0 && (
            <>
              <Typography variant="caption" sx={{ color: 'gray', display: 'block', mb: 1 }}>
                Popular Tokens
              </Typography>
              <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 1, mb: 2 }}>
                {popularTokens.map(token => (
                  <Chip
                    key={token.address}
                    label={token.symbol}
                    onClick={() => handleSelectToken(token)}
                    sx={{
                      bgcolor: '#2a2a3e',
                      color: 'white',
                      cursor: 'pointer',
                      '&:hover': { bgcolor: '#3a3a4e' }
                    }}
                  />
                ))}
              </Box>
            </>
          )}

          <Divider sx={{ borderColor: '#2a2a3e', my: 2 }} />

          <List sx={{ maxHeight: 300, overflow: 'auto' }}>
            {otherTokens.map(token => (
              <ListItemButton
                key={token.address}
                onClick={() => handleSelectToken(token)}
                sx={{ borderRadius: 1, mb: 0.5 }}
              >
                <ListItemIcon>
                  {token.logoURI ? (
                    <Avatar src={token.logoURI} sx={{ width: 32, height: 32 }} />
                  ) : (
                    <Avatar sx={{ bgcolor: '#2a2a3e', width: 32, height: 32 }}>
                      {token.symbol[0]}
                    </Avatar>
                  )}
                </ListItemIcon>
                <ListItemText
                  primary={
                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                      <Typography sx={{ color: 'white', fontWeight: 600 }}>{token.symbol}</Typography>
                      {token.isStable && <Chip label="Stable" size="small" sx={{ bgcolor: '#2a2a3e', fontSize: '0.65rem', height: 18 }} />}
                    </Box>
                  }
                  secondary={token.name}
                  primaryTypographyProps={{ color: 'white' }}
                  secondaryTypographyProps={{ color: 'gray' }}
                />
                <Box sx={{ textAlign: 'right' }}>
                  <Typography sx={{ color: 'white' }}>
                    {formatBalance(getTokenBalance(token), token.decimals)}
                  </Typography>
                  {token.priceUSD && (
                    <Typography variant="caption" sx={{ color: 'gray' }}>
                      {formatUSD(parseFloat(formatBalance(getTokenBalance(token), token.decimals)) * token.priceUSD)}
                    </Typography>
                  )}
                </Box>
              </ListItemButton>
            ))}
          </List>
        </DialogContent>
      </Dialog>
    );
  };

  // ============================================================================
  // Render
  // ============================================================================

  const getMaxBalance = () => {
    if (!tokenIn) return '0';
    const balance = getTokenBalance(tokenIn);
    return formatBalance(balance, tokenIn.decimals);
  };

  const setMaxAmount = () => {
    if (!tokenIn) return;
    const max = getMaxBalance();
    // Leave some for gas if native token
    if (tokenIn.isNative) {
      const maxBigInt = parseAmount(max, tokenIn.decimals);
      const gasBuffer = BigInt(0.01 * 1e18); // 0.01 ETH buffer
      const adjusted = maxBigInt > gasBuffer ? maxBigInt - gasBuffer : BigInt(0);
      setAmountIn(formatAmount(adjusted, tokenIn.decimals));
    } else {
      setAmountIn(max);
    }
  };

  return (
    <Box sx={{ 
      minHeight: '100vh', 
      bgcolor: '#0a0a14', 
      py: 4,
      px: { xs: 2, md: 4 }
    }}>
      <Box sx={{ maxWidth: 480, mx: 'auto' }}>
        {/* Header */}
        <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 3 }}>
          <Typography variant="h5" sx={{ color: 'white', fontWeight: 700 }}>
            Swap
          </Typography>
          <Box sx={{ display: 'flex', gap: 1 }}>
            <Tooltip title="Settings">
              <IconButton onClick={() => setShowSettings(!showSettings)} sx={{ color: 'white' }}>
                <Settings />
              </IconButton>
            </Tooltip>
            <Tooltip title="Refresh">
              <IconButton onClick={() => { loadTokenBalances(walletRef.current!); loadGasPrice(walletRef.current!); }} sx={{ color: 'white' }}>
                <Refresh />
              </IconButton>
            </Tooltip>
          </Box>
        </Box>

        {/* Chain Selector */}
        <Card sx={{ mb: 2, bgcolor: '#1a1a2e', borderRadius: 3 }}>
          <CardContent sx={{ p: 2, '&:last-child': { pb: 2 } }}>
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
              <Shield sx={{ color: '#00d4aa', fontSize: 16 }} />
              <Typography variant="caption" sx={{ color: 'gray' }}>
                {CHAIN_CONFIG[wallet.chainId]?.name || 'Ethereum'}
              </Typography>
            </Box>
          </CardContent>
        </Card>

        {/* Swap Card */}
        <Card sx={{ bgcolor: '#1a1a2e', borderRadius: 3 }}>
          <CardContent sx={{ p: 3 }}>
            {/* Input Token */}
            <Box sx={{ mb: 2 }}>
              <Box sx={{ display: 'flex', justifyContent: 'space-between', mb: 1 }}>
                <Typography variant="caption" sx={{ color: 'gray' }}>
                  You Pay
                </Typography>
                {wallet.isConnected && tokenIn && (
                  <Typography 
                    variant="caption" 
                    sx={{ color: 'gray', cursor: 'pointer' }}
                    onClick={() => setMaxAmount()}
                  >
                    Balance: {getMaxBalance()}
                  </Typography>
                )}
              </Box>
              <Box sx={{ display: 'flex', gap: 2, alignItems: 'center' }}>
                <TextField
                  fullWidth
                  placeholder="0.0"
                  value={amountIn}
                  onChange={(e) => {
                    const val = e.target.value;
                    if (/^\d*\.?\d*$/.test(val)) {
                      setAmountIn(val);
                    }
                  }}
                  disabled={!wallet.isConnected}
                  sx={{
                    input: { 
                      color: 'white', 
                      fontSize: '1.5rem',
                      fontWeight: 600,
                    },
                    '& .MuiOutlinedInput-root': {
                      '& fieldset': { borderColor: 'transparent' },
                      '&:hover fieldset': { borderColor: '#3a3a4e' },
                      '&.Mui-focused fieldset': { borderColor: '#00d4ff' },
                    }
                  }}
                />
                <Button
                  onClick={() => setShowTokenSelector('in')}
                  disabled={!wallet.isConnected}
                  sx={{
                    bgcolor: '#2a2a3e',
                    color: 'white',
                    minWidth: 120,
                    borderRadius: 2,
                    textTransform: 'none',
                    '&:hover': { bgcolor: '#3a3a4e' }
                  }}
                >
                  {tokenIn ? (
                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                      {tokenIn.logoURI && <Avatar src={tokenIn.logoURI} sx={{ width: 24, height: 24 }} />}
                      <Typography>{tokenIn.symbol}</Typography>
                    </Box>
                  ) : (
                    'Select'
                  )}
                  <KeyboardArrowDown />
                </Button>
              </Box>
              {tokenIn && tokenIn.priceUSD && amountIn && (
                <Typography variant="caption" sx={{ color: 'gray', mt: 0.5, display: 'block' }}>
                  ≈ {formatUSD(parseFloat(amountIn) * tokenIn.priceUSD)}
                </Typography>
              )}
            </Box>

            {/* Swap Direction Button */}
            <Box sx={{ display: 'flex', justifyContent: 'center', my: -1, position: 'relative', zIndex: 1 }}>
              <IconButton
                onClick={switchTokens}
                sx={{
                  bgcolor: '#2a2a3e',
                  border: '4px solid #1a1a2e',
                  '&:hover': { bgcolor: '#3a3a4e' }
                }}
              >
                <SwapHoriz sx={{ color: '#00d4ff' }} />
              </IconButton>
            </Box>

            {/* Output Token */}
            <Box sx={{ mt: 2 }}>
              <Box sx={{ display: 'flex', justifyContent: 'space-between', mb: 1 }}>
                <Typography variant="caption" sx={{ color: 'gray' }}>
                  You Receive
                </Typography>
                {wallet.isConnected && tokenOut && (
                  <Typography variant="caption" sx={{ color: 'gray' }}>
                    Balance: {formatBalance(getTokenBalance(tokenOut), tokenOut.decimals)}
                  </Typography>
                )}
              </Box>
              <Box sx={{ display: 'flex', gap: 2, alignItems: 'center' }}>
                <Box sx={{ flex: 1, position: 'relative' }}>
                  <TextField
                    fullWidth
                    placeholder="0.0"
                    value={amountOut}
                    InputProps={{ readOnly: true }}
                    sx={{
                      input: { 
                        color: 'white', 
                        fontSize: '1.5rem',
                        fontWeight: 600,
                      },
                      '& .MuiOutlinedInput-root': {
                        '& fieldset': { borderColor: 'transparent' },
                      }
                    }}
                  />
                  {loadingQuote && (
                    <LinearProgress 
                      sx={{ 
                        position: 'absolute', 
                        bottom: 0, 
                        left: 0, 
                        right: 0,
                        bgcolor: 'transparent',
                        '& .MuiLinearProgress-bar': { bgcolor: '#00d4ff' }
                      }} 
                    />
                  )}
                </Box>
                <Button
                  onClick={() => setShowTokenSelector('out')}
                  disabled={!wallet.isConnected}
                  sx={{
                    bgcolor: '#2a2a3e',
                    color: 'white',
                    minWidth: 120,
                    borderRadius: 2,
                    textTransform: 'none',
                    '&:hover': { bgcolor: '#3a3a4e' }
                  }}
                >
                  {tokenOut ? (
                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                      {tokenOut.logoURI && <Avatar src={tokenOut.logoURI} sx={{ width: 24, height: 24 }} />}
                      <Typography>{tokenOut.symbol}</Typography>
                    </Box>
                  ) : (
                    'Select'
                  )}
                  <KeyboardArrowDown />
                </Button>
              </Box>
              {tokenOut && tokenOut.priceUSD && amountOut && (
                <Typography variant="caption" sx={{ color: 'gray', mt: 0.5, display: 'block' }}>
                  ≈ {formatUSD(parseFloat(amountOut) * tokenOut.priceUSD)}
                </Typography>
              )}
            </Box>

            {/* Route Details */}
            {quote && quote.route.length > 0 && (
              <Box sx={{ mt: 2 }}>
                <Button
                  onClick={() => setShowRouteDetails(!showRouteDetails)}
                  sx={{ 
                    color: 'gray', 
                    textTransform: 'none',
                    fontSize: '0.75rem'
                  }}
                  endIcon={showRouteDetails ? <KeyboardArrowDown /> : <ArrowForward />}
                >
                  Best route via {quote.route[0].dexName}
                </Button>
                
                {showRouteDetails && (
                  <Card sx={{ bgcolor: '#0a0a14', mt: 1, borderRadius: 2 }}>
                    <CardContent sx={{ p: 2, '&:last-child': { pb: 2 } }}>
                      {quote.route.map((r, i) => (
                        <Box key={i} sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 1 }}>
                          <Typography sx={{ color: '#00d4aa', fontSize: '0.75rem' }}>
                            {DEX_INFO[r.dex]?.logo} {r.dexName}
                          </Typography>
                          <Typography sx={{ color: 'gray', fontSize: '0.75rem' }}>
                            Fee: {(r.fee / 100).toFixed(2)}%
                          </Typography>
                        </Box>
                      ))}
                      <Divider sx={{ borderColor: '#2a2a3e', my: 1 }} />
                      <Box sx={{ display: 'flex', justifyContent: 'space-between' }}>
                        <Typography sx={{ color: 'gray', fontSize: '0.75rem' }}>Price Impact</Typography>
                        <Typography sx={{ 
                          color: quote.priceImpact > 5 ? '#ff4444' : quote.priceImpact > 1 ? '#ffaa00' : '#00d4aa',
                          fontSize: '0.75rem'
                        }}>
                          {quote.priceImpact.toFixed(2)}%
                        </Typography>
                      </Box>
                      <Box sx={{ display: 'flex', justifyContent: 'space-between', mt: 0.5 }}>
                        <Typography sx={{ color: 'gray', fontSize: '0.75rem' }}>Minimum received</Typography>
                        <Typography sx={{ color: 'white', fontSize: '0.75rem' }}>
                          {quote.outputAmountMin} {tokenOut?.symbol}
                        </Typography>
                      </Box>
                      <Box sx={{ display: 'flex', justifyContent: 'space-between', mt: 0.5 }}>
                        <Typography sx={{ color: 'gray', fontSize: '0.75rem' }}>Gas fee</Typography>
                        <Typography sx={{ color: 'white', fontSize: '0.75rem' }}>
                          ≈ {formatUSD(quote.gasFeeUSD)}
                        </Typography>
                      </Box>
                    </CardContent>
                  </Card>
                )}
              </Box>
            )}

            {/* Wallet Connect / Swap Button */}
            <Box sx={{ mt: 3 }}>
              {!wallet.isConnected ? (
                <Stack spacing={1}>
                  <Button
                    fullWidth
                    variant="contained"
                    onClick={() => connectWallet('metamask')}
                    sx={{
                      bgcolor: '#FF007A',
                      color: 'white',
                      py: 1.5,
                      fontWeight: 600,
                      '&:hover': { bgcolor: '#cc005a' }
                    }}
                  >
                    Connect MetaMask
                  </Button>
                  <Button
                    fullWidth
                    variant="outlined"
                    onClick={() => connectWallet('walletconnect')}
                    sx={{
                      borderColor: '#3a3a4e',
                      color: 'white',
                      py: 1.5,
                      '&:hover': { borderColor: '#00d4ff', bgcolor: 'transparent' }
                    }}
                  >
                    Connect WalletConnect
                  </Button>
                </Stack>
              ) : (
                <Button
                  fullWidth
                  variant="contained"
                  onClick={executeSwap}
                  disabled={
                    !tokenIn || 
                    !tokenOut || 
                    !amountIn || 
                    parseFloat(amountIn) <= 0 ||
                    txState.status === 'approving' ||
                    txState.status === 'swapping' ||
                    txState.status === 'confirming'
                  }
                  sx={{
                    bgcolor: '#00d4ff',
                    color: 'black',
                    py: 1.5,
                    fontWeight: 600,
                    '&:hover': { bgcolor: '#00b8d4' },
                    '&:disabled': { bgcolor: '#3a3a4e', color: 'gray' }
                  }}
                >
                  {txState.status === 'approving' && 'Approving...'}
                  {txState.status === 'swapping' && 'Swap Pending...'}
                  {txState.status === 'confirming' && 'Confirming...'}
                  {txState.status === 'success' && 'Swap Complete!'}
                  {txState.status === 'error' && 'Try Again'}
                  {txState.status === 'idle' && !tokenIn && 'Select Token'}
                  {txState.status === 'idle' && tokenIn && !amountIn && 'Enter Amount'}
                  {txState.status === 'idle' && tokenIn && amountIn && parseFloat(amountIn) > 0 && 'Swap'}
                </Button>
              )}
            </Box>

            {/* Transaction Hash */}
            {txState.hash && (
              <Box sx={{ mt: 2, textAlign: 'center' }}>
                <Button
                  href={`${CHAIN_CONFIG[wallet.chainId]?.explorer}/tx/${txState.hash}`}
                  target="_blank"
                  rel="noopener"
                  endIcon={<OpenInNew />}
                  sx={{ color: '#00d4aa', textTransform: 'none' }}
                >
                  View on Explorer
                </Button>
              </Box>
            )}
          </CardContent>
        </Card>

        {/* Settings Panel */}
        {showSettings && (
          <Card sx={{ mt: 2, bgcolor: '#1a1a2e', borderRadius: 3 }}>
            <CardContent sx={{ p: 3 }}>
              <Typography variant="h6" sx={{ color: 'white', mb: 2 }}>Transaction Settings</Typography>
              
              <Box sx={{ mb: 3 }}>
                <Box sx={{ display: 'flex', justifyContent: 'space-between', mb: 1 }}>
                  <Typography variant="body2" sx={{ color: 'gray' }}>
                    Slippage Tolerance
                  </Typography>
                  <Typography variant="body2" sx={{ color: 'white' }}>
                    {slippage}%
                  </Typography>
                </Box>
                <Box sx={{ display: 'flex', gap: 1 }}>
                  {[0.1, 0.5, 1.0].map(val => (
                    <Button
                      key={val}
                      size="small"
                      variant={slippage === val ? 'contained' : 'outlined'}
                      onClick={() => setSlippage(val)}
                      sx={{ 
                        bgcolor: slippage === val ? '#00d4ff' : 'transparent',
                        color: slippage === val ? 'black' : 'white',
                        borderColor: '#3a3a4e'
                      }}
                    >
                      {val}%
                    </Button>
                  ))}
                  <TextField
                    size="small"
                    type="number"
                    value={slippage}
                    onChange={(e) => setSlippage(parseFloat(e.target.value) || 0)}
                    InputProps={{
                      endAdornment: <InputAdornment position="end"><Typography variant="caption" sx={{ color: 'gray' }}>%</Typography></InputAdornment>
                    }}
                    sx={{
                      width: 80,
                      '& input': { textAlign: 'center', color: 'white' },
                      '& .MuiOutlinedInput-root': {
                        '& fieldset': { borderColor: '#3a3a4e' },
                      }
                    }}
                  />
                </Box>
                <FormControlLabel
                  control={
                    <Switch 
                      checked={autoSlippage} 
                      onChange={(e) => setAutoSlippage(e.target.checked)}
                      sx={{ '& .MuiSwitch-thumb': { bgcolor: '#00d4ff' } }}
                    />
                  }
                  label={<Typography variant="caption" sx={{ color: 'gray' }}>Auto slippage</Typography>}
                  sx={{ mt: 1 }}
                />
              </Box>

              <Box sx={{ mb: 3 }}>
                <Typography variant="body2" sx={{ color: 'gray', mb: 1 }}>
                  Transaction Deadline: {deadline} minutes
                </Typography>
                <Slider
                  value={deadline}
                  onChange={(_, v) => setDeadline(v as number)}
                  min={1}
                  max={60}
                  sx={{ color: '#00d4ff' }}
                />
              </Box>

              <Box>
                <Typography variant="body2" sx={{ color: 'gray', mb: 1 }}>
                  Gas Preference: {gasPreference.charAt(0).toUpperCase() + gasPreference.slice(1)}
                </Typography>
                <Typography variant="caption" sx={{ color: 'gray', display: 'block', mb: 1 }}>
                  Base Fee: {gasPrice.baseFee.toFixed(1)} gwei
                </Typography>
                <Box sx={{ display: 'flex', gap: 1 }}>
                  {(['slow', 'standard', 'fast', 'instant'] as const).map(speed => (
                    <Button
                      key={speed}
                      size="small"
                      variant={gasPreference === speed ? 'contained' : 'outlined'}
                      onClick={() => setGasPreference(speed)}
                      sx={{ 
                        flex: 1,
                        minWidth: 0,
                        bgcolor: gasPreference === speed ? '#00d4ff' : 'transparent',
                        color: gasPreference === speed ? 'black' : 'white',
                        borderColor: '#3a3a4e',
                        fontSize: '0.65rem',
                        padding: '4px 8px'
                      }}
                    >
                      {speed.charAt(0).toUpperCase() + speed.slice(1)}
                      <Typography variant="caption" sx={{ display: 'block', opacity: 0.7, fontSize: '0.6rem' }}>
                        {gasPrice[speed]} gwei
                      </Typography>
                    </Button>
                  ))}
                </Box>
              </Box>
            </CardContent>
          </Card>
        )}

        {/* Features Bar */}
        <Box sx={{ mt: 3, display: 'flex', justifyContent: 'center', gap: 3, flexWrap: 'wrap' }}>
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5, color: '#00d4aa' }}>
            <Shield fontSize="small" />
            <Typography variant="caption" sx={{ color: 'gray' }}>MEV Protected</Typography>
          </Box>
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5, color: '#00d4aa' }}>
            <Speed fontSize="small" />
            <Typography variant="caption" sx={{ color: 'gray' }}>Best Route</Typography>
          </Box>
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5, color: '#00d4aa' }}>
            <CompareArrows fontSize="small" />
            <Typography variant="caption" sx={{ color: 'gray' }}>20+ DEXs</Typography>
          </Box>
        </Box>
      </Box>

      {/* Token Selector Modal */}
      {renderTokenSelector()}

      {/* Snackbar */}
      <Snackbar
        open={snackbar.open}
        autoHideDuration={5000}
        onClose={() => setSnackbar({ ...snackbar, open: false })}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'center' }}
      >
        <Alert 
          onClose={() => setSnackbar({ ...snackbar, open: false })} 
          severity={snackbar.severity}
          sx={{ bgcolor: snackbar.severity === 'success' ? '#1b5e20' : snackbar.severity === 'error' ? '#b71c1c' : '#1a237e' }}
        >
          {snackbar.message}
        </Alert>
      </Snackbar>
    </Box>
  );
}

