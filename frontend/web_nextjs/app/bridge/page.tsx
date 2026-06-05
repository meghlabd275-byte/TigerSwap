'use client';

import React, { useState, useEffect, useCallback } from 'react';
import {
  Box, Typography, Card, CardContent, Button, TextField,
  Select, MenuItem, FormControl, InputLabel, Chip,
  CircularProgress, Snackbar, Alert, IconButton,
  Stepper, Step, StepLabel, Divider, InputAdornment
} from '@mui/material';
import {
  SwapHoriz, ArrowForward, AccountBalance, CheckCircle,
  Error, AccessTime, Speed, Security, Warning,
  ContentCopy, Refresh, ArrowDropDown
} from '@mui/icons-material';

// ============================================================================
// Types & Interfaces
// ============================================================================

interface Chain {
  id: number;
  name: string;
  icon: string;
  color: string;
  rpcUrl: string;
  explorerUrl: string;
  nativeCurrency: string;
  avgBridgeTime: string;
  gasCost: string;
}

interface BridgeQuote {
  fromChain: number;
  toChain: number;
  token: string;
  amount: string;
  bridgeFee: number;
  networkFee: number;
  estimatedTime: string;
  receivedAmount: string;
  rate: number;
  availableRoutes: BridgeRoute[];
}

interface BridgeRoute {
  id: string;
  name: string;
  logo: string;
  fee: number;
  time: string;
  reliability: number;
  minAmount: number;
  maxAmount: number;
}

interface BridgeTransfer {
  id: string;
  fromChain: number;
  toChain: number;
  token: string;
  amount: string;
  status: 'pending' | 'submitted' | 'confirming' | 'completed' | 'failed';
  sourceTxHash?: string;
  destTxHash?: string;
  confirmations: number;
  requiredConfirmations: number;
  timestamp: number;
}

// ============================================================================
// Constants
// ============================================================================

const CHAINS: Chain[] = [
  { id: 1, name: 'Ethereum', icon: '🔷', color: '#627EEA', rpcUrl: 'https://eth.llamarpc.com', explorerUrl: 'https://etherscan.io', nativeCurrency: 'ETH', avgBridgeTime: '10-15 min', gasCost: '$5-15' },
  { id: 56, name: 'BNB Chain', icon: '🟡', color: '#F3BA2F', rpcUrl: 'https://bsc-dataseed.binance.org', explorerUrl: 'https://bscscan.com', nativeCurrency: 'BNB', avgBridgeTime: '5-10 min', gasCost: '$1-3' },
  { id: 137, name: 'Polygon', icon: '🟣', color: '#8247E5', rpcUrl: 'https://polygon-rpc.com', explorerUrl: 'https://polygonscan.com', nativeCurrency: 'MATIC', avgBridgeTime: '7-12 min', gasCost: '$0.5-2' },
  { id: 42161, name: 'Arbitrum', icon: '🔵', color: '#28A0F0', rpcUrl: 'https://arb1.arbitrum.io/rpc', explorerUrl: 'https://arbiscan.io', nativeCurrency: 'ETH', avgBridgeTime: '10-15 min', gasCost: '$2-5' },
  { id: 10, name: 'Optimism', icon: '🔴', color: '#FF0420', rpcUrl: 'https://mainnet.optimism.io', explorerUrl: 'https://optimistic.etherscan.io', nativeCurrency: 'ETH', avgBridgeTime: '10-15 min', gasCost: '$1-3' },
  { id: 8453, name: 'Base', icon: '🔵', color: '#0052FF', rpcUrl: 'https://mainnet.base.org', explorerUrl: 'https://basescan.org', nativeCurrency: 'ETH', avgBridgeTime: '5-8 min', gasCost: '$0.5-2' },
  { id: 43114, name: 'Avalanche', icon: '🔶', color: '#E84142', rpcUrl: 'https://api.avax.network/ext/bc/C/rpc', explorerUrl: 'https://snowtrace.io', nativeCurrency: 'AVAX', avgBridgeTime: '3-5 min', gasCost: '$1-3' },
];

const TOKENS = [
  { symbol: 'ETH', name: 'Ethereum', decimals: 18, icon: '🔷', isNative: true },
  { symbol: 'USDC', name: 'USD Coin', decimals: 6, icon: '💵', isNative: false },
  { symbol: 'USDT', name: 'Tether', decimals: 6, icon: '💰', isNative: false },
  { symbol: 'DAI', name: 'Dai', decimals: 18, icon: '📎', isNative: false },
  { symbol: 'WBTC', name: 'Wrapped Bitcoin', decimals: 8, icon: '₿', isNative: false },
  { symbol: 'MATIC', name: 'Polygon', decimals: 18, icon: '🟣', isNative: false },
];

const BRIDGE_ROUTES: BridgeRoute[] = [
  { id: 'across', name: 'Across', logo: '🔄', fee: 0.09, time: '1-3 min', reliability: 99.2, minAmount: 10, maxAmount: 1000000 },
  { id: 'stargate', name: 'Stargate', logo: '🌉', fee: 0.06, time: '3-5 min', reliability: 98.8, minAmount: 50, maxAmount: 500000 },
  { id: 'hop', name: 'Hop Exchange', logo: '⚡', fee: 0.04, time: '5-10 min', reliability: 98.5, minAmount: 100, maxAmount: 250000 },
  { id: 'cbridge', name: 'Celer Bridge', logo: '🌐', fee: 0.03, time: '10-20 min', reliability: 97.9, minAmount: 100, maxAmount: 1000000 },
  { id: 'synapse', name: 'Synapse', logo: '🔗', fee: 0.05, time: '5-15 min', reliability: 97.5, minAmount: 50, maxAmount: 250000 },
];

// ============================================================================
// Utility Functions
// ============================================================================

function formatAddress(address: string, chars: number = 4): string {
  return `${address.slice(0, chars + 2)}...${address.slice(-chars)}`;
}

function formatAmount(amount: string, decimals: number = 18): string {
  if (!amount || amount === '0') return '0';
  try {
    const num = Number(amount) / Math.pow(10, decimals);
    if (num < 0.0001) return '<0.0001';
    return num.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 6 });
  } catch {
    return '0';
  }
}

function formatUSD(amount: number): string {
  return new Intl.NumberFormat('en-US', { style: 'currency', currency: 'USD' }).format(amount);
}

function timeAgo(timestamp: number): string {
  const diff = Date.now() - timestamp;
  const minutes = Math.floor(diff / 60000);
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  return `${Math.floor(hours / 24)}d ago`;
}

// ============================================================================
// Mock Data Generator
// ============================================================================

function generateBridgeQuote(fromChain: number, toChain: number, token: string, amount: string): BridgeQuote {
  const amountNum = parseFloat(amount) || 0;
  const bridgeFee = amountNum * 0.001; // 0.1% bridge fee
  const networkFee = fromChain === 1 ? 0.005 * amountNum : 0.001 * amountNum;
  const totalFees = bridgeFee + networkFee;
  const receivedAmount = amountNum - totalFees;

  return {
    fromChain,
    toChain,
    token,
    amount,
    bridgeFee,
    networkFee,
    estimatedTime: '5-10 min',
    receivedAmount: receivedAmount.toString(),
    rate: 0.999,
    availableRoutes: BRIDGE_ROUTES.map(route => ({
      ...route,
      fee: route.fee + 0.001, // Add our fee
    })),
  };
}

function generateTransferHistory(): BridgeTransfer[] {
  return [
    {
      id: 'tx_1',
      fromChain: 1,
      toChain: 137,
      token: 'ETH',
      amount: '0.5',
      status: 'completed',
      sourceTxHash: '0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef',
      destTxHash: '0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890',
      confirmations: 12,
      requiredConfirmations: 12,
      timestamp: Date.now() - 3600000,
    },
    {
      id: 'tx_2',
      fromChain: 56,
      toChain: 42161,
      token: 'USDC',
      amount: '1000',
      status: 'confirming',
      sourceTxHash: '0x9876543210fedcba9876543210fedcba9876543210fedcba9876543210fedcba',
      confirmations: 10,
      requiredConfirmations: 12,
      timestamp: Date.now() - 600000,
    },
    {
      id: 'tx_3',
      fromChain: 137,
      toChain: 1,
      token: 'MATIC',
      amount: '500',
      status: 'pending',
      confirmations: 0,
      requiredConfirmations: 12,
      timestamp: Date.now() - 120000,
    },
  ];
}

// ============================================================================
// Main Bridge Page Component
// ============================================================================

export default function BridgePage() {
  // State
  const [fromChain, setFromChain] = useState(1);
  const [toChain, setToChain] = useState(137);
  const [token, setToken] = useState('ETH');
  const [amount, setAmount] = useState('');
  const [quote, setQuote] = useState<BridgeQuote | null>(null);
  const [selectedRoute, setSelectedRoute] = useState('across');
  const [loading, setLoading] = useState(false);
  const [transferHistory, setTransferHistory] = useState<BridgeTransfer[]>([]);
  const [activeTab, setActiveTab] = useState(0);
  const [activeStep, setActiveStep] = useState(0);
  const [walletAddress] = useState('0x742d35Cc6634C0532...5F6e1');

  // Snackbar
  const [snackbar, setSnackbar] = useState({
    open: false,
    message: '',
    severity: 'success' as 'success' | 'error' | 'info'
  });

  // ============================================================================
  // Effects
  // ============================================================================

  useEffect(() => {
    setTransferHistory(generateTransferHistory());
  }, []);

  useEffect(() => {
    if (amount && parseFloat(amount) > 0) {
      const mockQuote = generateBridgeQuote(fromChain, toChain, token, amount);
      setQuote(mockQuote);
    } else {
      setQuote(null);
    }
  }, [fromChain, toChain, token, amount]);

  // ============================================================================
  // Handlers
  // ============================================================================

  const handleSwitchChains = () => {
    const temp = fromChain;
    setFromChain(toChain);
    setToChain(temp);
  };

  const handleMaxAmount = () => {
    // In production, this would query actual wallet balance
    setAmount('1.0');
  };

  const handleBridge = async () => {
    if (!quote) return;

    setLoading(true);
    setActiveStep(1);

    try {
      // Simulate bridge transaction
      await new Promise(resolve => setTimeout(resolve, 2000));

      const newTransfer: BridgeTransfer = {
        id: `tx_${Date.now()}`,
        fromChain,
        toChain,
        token,
        amount,
        status: 'submitted',
        confirmations: 0,
        requiredConfirmations: 12,
        timestamp: Date.now(),
      };

      setTransferHistory(prev => [newTransfer, ...prev]);
      setActiveStep(2);

      // Simulate confirmation progress
      for (let i = 1; i <= 12; i++) {
        await new Promise(resolve => setTimeout(resolve, 500));
        setTransferHistory(prev => prev.map(tx =>
          tx.id === newTransfer.id
            ? { ...tx, confirmations: i, status: i >= 12 ? 'completed' : 'confirming' }
            : tx
        ));
      }

      setActiveStep(3);
      setSnackbar({ open: true, message: `Successfully bridged ${amount} ${token} to ${CHAINS.find(c => c.id === toChain)?.name}`, severity: 'success' });

      // Reset form after success
      setTimeout(() => {
        setActiveStep(0);
        setAmount('');
        setQuote(null);
      }, 2000);

    } catch (error) {
      setSnackbar({ open: true, message: 'Bridge transaction failed', severity: 'error' });
    } finally {
      setLoading(false);
    }
  };

  const getStatusChip = (status: BridgeTransfer['status']) => {
    switch (status) {
      case 'pending':
        return <Chip label="Pending" size="small" sx={{ bgcolor: '#ff980020', color: '#ff9800' }} />;
      case 'submitted':
        return <Chip label="Submitted" size="small" sx={{ bgcolor: '#00d4ff20', color: '#00d4ff' }} />;
      case 'confirming':
        return <Chip label="Confirming" size="small" sx={{ bgcolor: '#ff980020', color: '#ff9800' }} />;
      case 'completed':
        return <Chip label="Completed" size="small" sx={{ bgcolor: '#00d4aa20', color: '#00d4aa' }} />;
      case 'failed':
        return <Chip label="Failed" size="small" sx={{ bgcolor: '#ff572220', color: '#ff5722' }} />;
    }
  };

  // ============================================================================
  // Main Render
  // ============================================================================

  return (
    <Box sx={{ minHeight: '100vh', bgcolor: '#0a0a14', p: 3 }}>
      <Box sx={{ maxWidth: 1200, mx: 'auto' }}>
        {/* Header */}
        <Box sx={{ mb: 4 }}>
          <Typography variant="h4" sx={{ color: 'white', fontWeight: 'bold' }}>
            🌉 Cross-Chain Bridge
          </Typography>
          <Typography variant="body2" sx={{ color: '#9ca3af', mt: 1 }}>
            Transfer assets across 7+ chains with the best rates and fastest routes
          </Typography>
        </Box>

        {/* Features Bar */}
        <Box sx={{ display: 'flex', gap: 3, mb: 4, flexWrap: 'wrap' }}>
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
            <Speed sx={{ color: '#00d4aa', fontSize: 20 }} />
            <Typography variant="caption" sx={{ color: '#9ca3af' }}>Fast Transfers</Typography>
          </Box>
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
            <Security sx={{ color: '#00d4aa', fontSize: 20 }} />
            <Typography variant="caption" sx={{ color: '#9ca3af' }}>Secure</Typography>
          </Box>
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
            <SwapHoriz sx={{ color: '#00d4aa', fontSize: 20 }} />
            <Typography variant="caption" sx={{ color: '#9ca3af' }}>7+ Chains</Typography>
          </Box>
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
            <CheckCircle sx={{ color: '#00d4aa', fontSize: 20 }} />
            <Typography variant="caption" sx={{ color: '#9ca3af' }}>Best Routes</Typography>
          </Box>
        </Box>

        {/* Tabs */}
        <Box sx={{ display: 'flex', gap: 2, mb: 3 }}>
          <Button
            variant={activeTab === 0 ? 'contained' : 'outlined'}
            onClick={() => setActiveTab(0)}
            sx={{ bgcolor: activeTab === 0 ? '#00d4aa' : 'transparent', color: activeTab === 0 ? 'black' : 'white', borderColor: '#3a3a4e' }}
          >
            Bridge
          </Button>
          <Button
            variant={activeTab === 1 ? 'contained' : 'outlined'}
            onClick={() => setActiveTab(1)}
            sx={{ bgcolor: activeTab === 1 ? '#00d4aa' : 'transparent', color: activeTab === 1 ? 'black' : 'white', borderColor: '#3a3a4e' }}
          >
            History
          </Button>
        </Box>

        {activeTab === 0 ? (
          /* Bridge Tab */
          <Box sx={{ display: 'grid', gridTemplateColumns: '2fr 1fr', gap: 3 }}>
            {/* Main Bridge Card */}
            <Card sx={{ bgcolor: '#1a1a2e', borderRadius: 3 }}>
              <CardContent sx={{ p: 4 }}>
                {/* Stepper */}
                <Stepper activeStep={activeStep} sx={{ mb: 4 }}>
                  <Step><StepLabel>Configure</StepLabel></Step>
                  <Step><StepLabel>Transfer</StepLabel></Step>
                  <Step><StepLabel>Complete</StepLabel></Step>
                </Stepper>

                {/* From Chain */}
                <Box sx={{ mb: 3 }}>
                  <Typography sx={{ color: '#9ca3af', mb: 1 }}>From</Typography>
                  <Box sx={{ display: 'flex', gap: 2, alignItems: 'center' }}>
                    <FormControl fullWidth size="small">
                      <Select
                        value={fromChain}
                        onChange={(e) => setFromChain(e.target.value as number)}
                        sx={{ color: 'white', bgcolor: '#2a2a3e', '& .MuiOutlinedInput-notchedOutline': { borderColor: '#3a3a4e' } }}
                      >
                        {CHAINS.map(chain => (
                          <MenuItem key={chain.id} value={chain.id}>
                            <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                              <span>{chain.icon}</span>
                              <span>{chain.name}</span>
                            </Box>
                          </MenuItem>
                        ))}
                      </Select>
                    </FormControl>
                    <TextField
                      size="small"
                      type="number"
                      placeholder="0.0"
                      value={amount}
                      onChange={(e) => setAmount(e.target.value)}
                      InputProps={{
                        endAdornment: (
                          <InputAdornment>
                            <Button size="small" onClick={handleMaxAmount} sx={{ color: '#00d4aa', minWidth: 0 }}>
                              MAX
                            </Button>
                          </InputAdornment>
                        ),
                      }}
                      sx={{ flex: 1, '& input': { color: 'white' }, '& .MuiOutlinedInput-root': { '& fieldset': { borderColor: '#3a3a4e' } } }}
                    />
                  </Box>
                </Box>

                {/* Switch Button */}
                <Box sx={{ display: 'flex', justifyContent: 'center', my: 2 }}>
                  <IconButton
                    onClick={handleSwitchChains}
                    sx={{
                      bgcolor: '#2a2a3e',
                      border: '4px solid #1a1a2e',
                      '&:hover': { bgcolor: '#3a3a4e' },
                    }}
                  >
                    <SwapHoriz sx={{ color: '#00d4aa' }} />
                  </IconButton>
                </Box>

                {/* To Chain */}
                <Box sx={{ mb: 3 }}>
                  <Typography sx={{ color: '#9ca3af', mb: 1 }}>To</Typography>
                  <FormControl fullWidth size="small">
                    <Select
                      value={toChain}
                      onChange={(e) => setToChain(e.target.value as number)}
                      sx={{ color: 'white', bgcolor: '#2a2a3e', '& .MuiOutlinedInput-notchedOutline': { borderColor: '#3a3a4e' } }}
                    >
                      {CHAINS.filter(c => c.id !== fromChain).map(chain => (
                        <MenuItem key={chain.id} value={chain.id}>
                          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                            <span>{chain.icon}</span>
                            <span>{chain.name}</span>
                          </Box>
                        </MenuItem>
                      ))}
                    </Select>
                  </FormControl>
                </Box>

                {/* Token Selection */}
                <Box sx={{ mb: 3 }}>
                  <Typography sx={{ color: '#9ca3af', mb: 1 }}>Token</Typography>
                  <Box sx={{ display: 'flex', gap: 1, flexWrap: 'wrap' }}>
                    {TOKENS.map(t => (
                      <Chip
                        key={t.symbol}
                        label={
                          <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
                            <span>{t.icon}</span>
                            <span>{t.symbol}</span>
                          </Box>
                        }
                        onClick={() => setToken(t.symbol)}
                        sx={{
                          bgcolor: token === t.symbol ? '#00d4aa20' : '#2a2a3e',
                          color: token === t.symbol ? '#00d4aa' : 'white',
                          cursor: 'pointer',
                        }}
                      />
                    ))}
                  </Box>
                </Box>

                <Divider sx={{ borderColor: '#3a3a4e', my: 3 }} />

                {/* Route Selection */}
                <Typography sx={{ color: '#9ca3af', mb: 2 }}>Select Route</Typography>
                <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1, mb: 3 }}>
                  {quote?.availableRoutes.slice(0, 3).map(route => (
                    <Box
                      key={route.id}
                      onClick={() => setSelectedRoute(route.id)}
                      sx={{
                        p: 2,
                        borderRadius: 2,
                        bgcolor: selectedRoute === route.id ? '#00d4aa10' : '#2a2a3e',
                        border: `1px solid ${selectedRoute === route.id ? '#00d4aa' : 'transparent'}`,
                        cursor: 'pointer',
                        display: 'flex',
                        justifyContent: 'space-between',
                        alignItems: 'center',
                        '&:hover': { bgcolor: '#3a3a4e' },
                      }}
                    >
                      <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
                        <Typography sx={{ fontSize: 20 }}>{route.logo}</Typography>
                        <Box>
                          <Typography sx={{ color: 'white', fontWeight: 'bold' }}>{route.name}</Typography>
                          <Typography variant="caption" sx={{ color: '#9ca3af' }}>{route.time}</Typography>
                        </Box>
                      </Box>
                      <Box sx={{ textAlign: 'right' }}>
                        <Typography sx={{ color: '#00d4aa' }}>{(route.fee * 100).toFixed(2)}% fee</Typography>
                        <Typography variant="caption" sx={{ color: '#9ca3af' }}>{route.reliability}% success</Typography>
                      </Box>
                    </Box>
                  ))}
                </Box>

                {/* Bridge Button */}
                <Button
                  fullWidth
                  variant="contained"
                  size="large"
                  onClick={handleBridge}
                  disabled={!quote || loading || activeStep > 0}
                  startIcon={loading ? <CircularProgress size={20} sx={{ color: 'black' }} /> : <ArrowForward />}
                  sx={{
                    bgcolor: '#00d4aa',
                    color: 'black',
                    py: 1.5,
                    '&:hover': { bgcolor: '#00b894' },
                    '&:disabled': { bgcolor: '#3a3a4e', color: '#666' },
                  }}
                >
                  {loading ? 'Processing...' : amount && quote ? `Bridge ${amount} ${token}` : 'Enter Amount'}
                </Button>
              </CardContent>
            </Card>

            {/* Quote Card */}
            <Card sx={{ bgcolor: '#1a1a2e', borderRadius: 3, height: 'fit-content' }}>
              <CardContent sx={{ p: 3 }}>
                <Typography variant="h6" sx={{ color: 'white', mb: 3 }}>Transfer Details</Typography>

                {quote ? (
                  <Box>
                    <Box sx={{ display: 'flex', justifyContent: 'space-between', mb: 2 }}>
                      <Typography sx={{ color: '#9ca3af' }}>You Send</Typography>
                      <Typography sx={{ color: 'white' }}>{formatAmount(amount)} {token}</Typography>
                    </Box>
                    <Box sx={{ display: 'flex', justifyContent: 'space-between', mb: 2 }}>
                      <Typography sx={{ color: '#9ca3af' }}>Bridge Fee</Typography>
                      <Typography sx={{ color: '#ff9800' }}>{formatUSD(quote.bridgeFee)}</Typography>
                    </Box>
                    <Box sx={{ display: 'flex', justifyContent: 'space-between', mb: 2 }}>
                      <Typography sx={{ color: '#9ca3af' }}>Network Fee</Typography>
                      <Typography sx={{ color: '#ff9800' }}>{formatUSD(quote.networkFee)}</Typography>
                    </Box>
                    <Box sx={{ display: 'flex', justifyContent: 'space-between', mb: 2 }}>
                      <Typography sx={{ color: '#9ca3af' }}>Estimated Time</Typography>
                      <Typography sx={{ color: 'white' }}>{quote.estimatedTime}</Typography>
                    </Box>
                    <Divider sx={{ borderColor: '#3a3a4e', my: 2 }} />
                    <Box sx={{ display: 'flex', justifyContent: 'space-between', mb: 2 }}>
                      <Typography sx={{ color: '#9ca3af' }}>You Receive</Typography>
                      <Typography sx={{ color: '#00d4aa', fontWeight: 'bold', fontSize: 18 }}>
                        {formatAmount(quote.receivedAmount)} {token}
                      </Typography>
                    </Box>

                    <Box sx={{ bgcolor: '#2a2a3e', p: 2, borderRadius: 2, mt: 3 }}>
                      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 1 }}>
                        <Speed sx={{ color: '#00d4aa', fontSize: 16 }} />
                        <Typography variant="caption" sx={{ color: '#9ca3af' }}>Fastest Route</Typography>
                      </Box>
                      <Typography sx={{ color: 'white' }}>
                        {CHAINS.find(c => c.id === fromChain)?.icon} {CHAINS.find(c => c.id === fromChain)?.name}
                        {' → '}
                        {CHAINS.find(c => c.id === toChain)?.icon} {CHAINS.find(c => c.id === toChain)?.name}
                      </Typography>
                    </Box>
                  </Box>
                ) : (
                  <Box sx={{ textAlign: 'center', py: 4 }}>
                    <Typography sx={{ color: '#9ca3af' }}>Enter an amount to see quote</Typography>
                  </Box>
                )}
              </CardContent>
            </Card>
          </Box>
        ) : (
          /* History Tab */
          <Card sx={{ bgcolor: '#1a1a2e', borderRadius: 3 }}>
            <CardContent sx={{ p: 3 }}>
              <Typography variant="h6" sx={{ color: 'white', mb: 3 }}>Transfer History</Typography>
              <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
                {transferHistory.map(tx => (
                  <Box
                    key={tx.id}
                    sx={{
                      p: 3,
                      bgcolor: '#2a2a3e',
                      borderRadius: 2,
                      display: 'flex',
                      justifyContent: 'space-between',
                      alignItems: 'center',
                    }}
                  >
                    <Box>
                      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 1 }}>
                        {CHAINS.find(c => c.id === tx.fromChain)?.icon}
                        <ArrowForward sx={{ color: '#9ca3af', fontSize: 16 }} />
                        {CHAINS.find(c => c.id === tx.toChain)?.icon}
                        <Typography sx={{ color: 'white', ml: 1 }}>
                          {tx.amount} {tx.token}
                        </Typography>
                        {getStatusChip(tx.status)}
                      </Box>
                      <Typography variant="caption" sx={{ color: '#9ca3af' }}>
                        {timeAgo(tx.timestamp)}
                        {tx.sourceTxHash && (
                          <> • Source: <a href={`${CHAINS.find(c => c.id === tx.fromChain)?.explorerUrl}/tx/${tx.sourceTxHash}`} target="_blank" rel="noopener" style={{ color: '#00d4aa' }}>{formatAddress(tx.sourceTxHash)}</a></>
                        )}
                      </Typography>
                      {tx.status === 'confirming' && (
                        <Box sx={{ mt: 1 }}>
                          <Typography variant="caption" sx={{ color: '#ff9800' }}>
                            Confirming: {tx.confirmations}/{tx.requiredConfirmations} blocks
                          </Typography>
                        </Box>
                      )}
                    </Box>
                    {tx.destTxHash && (
                      <Button
                        size="small"
                        href={`${CHAINS.find(c => c.id === tx.toChain)?.explorerUrl}/tx/${tx.destTxHash}`}
                        target="_blank"
                        sx={{ color: '#00d4aa' }}
                      >
                        View on Dest
                      </Button>
                    )}
                  </Box>
                ))}
              </Box>
            </CardContent>
          </Card>
        )}

        {/* Supported Chains */}
        <Box sx={{ mt: 4 }}>
          <Typography variant="h6" sx={{ color: 'white', mb: 2 }}>Supported Chains</Typography>
          <Box sx={{ display: 'flex', gap: 2, flexWrap: 'wrap' }}>
            {CHAINS.map(chain => (
              <Card key={chain.id} sx={{ bgcolor: '#1a1a2e', borderRadius: 2, minWidth: 150 }}>
                <CardContent sx={{ p: 2, '&:last-child': { pb: 2 } }}>
                  <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 1 }}>
                    <Typography sx={{ fontSize: 24 }}>{chain.icon}</Typography>
                    <Typography sx={{ color: 'white', fontWeight: 'bold' }}>{chain.name}</Typography>
                  </Box>
                  <Typography variant="caption" sx={{ color: '#9ca3af', display: 'block' }}>
                    Avg Time: {chain.avgBridgeTime}
                  </Typography>
                  <Typography variant="caption" sx={{ color: '#9ca3af' }}>
                    Gas: {chain.gasCost}
                  </Typography>
                </CardContent>
              </Card>
            ))}
          </Box>
        </Box>
      </Box>

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