'use client';

import React, { useState, useEffect, useCallback } from 'react';
import {
  Box, Typography, Card, CardContent, Button, TextField,
  Table, TableBody, TableCell, TableContainer, TableHead, TableRow,
  Chip, IconButton, Select, MenuItem, FormControl, InputLabel,
  Dialog, DialogTitle, DialogContent, DialogActions, Tabs, Tab,
  LinearProgress, Snackbar, Alert, CircularProgress,
  Slider, InputAdornment, Divider, ToggleButton, ToggleButtonGroup
} from '@mui/material';
import {
  Schedule, TrendingUp, TrendingDown, Add, Remove, PlayArrow,
  Pause, Delete, Refresh, ShowChart, AccessTime, CheckCircle
} from '@mui/icons-material';
import { useTheme } from '../components/ThemeProvider';

// ============================================================================
// Types & Interfaces
// ============================================================================

interface TWAPOrder {
  id: string;
  userAddress: string;
  tokenIn: string;
  tokenOut: string;
  totalAmount: string;
  filledAmount: string;
  numOrders: number;
  completedOrders: number;
  intervalMinutes: number;
  startTime: number;
  endTime: number;
  nextExecutionTime: number;
  priceType: 'market' | 'limit';
  limitPrice?: number;
  slippageBps: number;
  status: 'active' | 'paused' | 'completed' | 'cancelled';
  createdAt: number;
  lastExecutionTime?: number;
  lastExecutionPrice?: number;
}

interface DCAOrder {
  id: string;
  userAddress: string;
  tokenIn: string;
  tokenOut: string;
  amountPerOrder: string;
  totalAmount: string;
  filledAmount: string;
  frequency: 'daily' | 'weekly' | 'monthly';
  dayOfWeek?: number;
  dayOfMonth?: number;
  hourOfDay: number;
  nextExecutionTime: number;
  status: 'active' | 'paused' | 'completed' | 'cancelled';
  createdAt: number;
  totalOrders: number;
  completedOrders: number;
}

interface OrderExecution {
  id: string;
  orderId: string;
  orderType: 'twap' | 'dca';
  tokenIn: string;
  tokenOut: string;
  amountIn: string;
  amountOut: string;
  price: number;
  timestamp: number;
  txHash: string;
  status: 'success' | 'failed' | 'pending';
}

// ============================================================================
// Constants
// ============================================================================

const TOKENS = [
  { symbol: 'ETH', name: 'Ethereum', icon: '🔷', price: 2450 },
  { symbol: 'BTC', name: 'Bitcoin', icon: '₿', price: 62500 },
  { symbol: 'USDC', name: 'USD Coin', icon: '💵', price: 1 },
  { symbol: 'USDT', name: 'Tether', icon: '💰', price: 1 },
  { symbol: 'LINK', name: 'Chainlink', icon: '🔗', price: 18.5 },
  { symbol: 'UNI', name: 'Uniswap', icon: '🦄', price: 12.5 },
  { symbol: 'AAVE', name: 'Aave', icon: '👻', price: 285 },
  { symbol: 'MATIC', name: 'Polygon', icon: '🟣', price: 0.85 },
];

const FREQUENCIES = [
  { value: 'daily', label: 'Daily' },
  { value: 'weekly', label: 'Weekly' },
  { value: 'monthly', label: 'Monthly' },
];

const INTERVALS = [
  { value: 15, label: '15 minutes' },
  { value: 30, label: '30 minutes' },
  { value: 60, label: '1 hour' },
  { value: 240, label: '4 hours' },
  { value: 1440, label: 'Daily' },
];

const DAYS_OF_WEEK = ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday'];

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

function formatDateTime(timestamp: number): string {
  return new Date(timestamp).toLocaleString('en-US', {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit'
  });
}

function timeUntil(timestamp: number): string {
  const diff = timestamp - Date.now();
  if (diff <= 0) return 'Now';
  const hours = Math.floor(diff / 3600000);
  const minutes = Math.floor((diff % 3600000) / 60000);
  if (hours > 24) return `${Math.floor(hours / 24)}d ${hours % 24}h`;
  if (hours > 0) return `${hours}h ${minutes}m`;
  return `${minutes}m`;
}

// ============================================================================
// Mock Data Generators
// ============================================================================

function generateTWAPOrders(): TWAPOrder[] {
  return [
    {
      id: 'twap_1',
      userAddress: '0x742d35Cc6634C0532',
      tokenIn: 'ETH',
      tokenOut: 'USDC',
      totalAmount: '10000',
      filledAmount: '3500',
      numOrders: 10,
      completedOrders: 3,
      intervalMinutes: 60,
      startTime: Date.now() - 3600000 * 5,
      endTime: Date.now() + 3600000 * 19,
      nextExecutionTime: Date.now() + 1800000,
      priceType: 'market',
      slippageBps: 50,
      status: 'active',
      createdAt: Date.now() - 3600000 * 5,
      lastExecutionTime: Date.now() - 1800000,
      lastExecutionPrice: 2448,
    },
    {
      id: 'twap_2',
      userAddress: '0x742d35Cc6634C0532',
      tokenIn: 'BTC',
      tokenOut: 'USDC',
      totalAmount: '50000',
      filledAmount: '0',
      numOrders: 20,
      completedOrders: 0,
      intervalMinutes: 240,
      startTime: Date.now() + 3600000,
      endTime: Date.now() + 3600000 * 24 * 5,
      nextExecutionTime: Date.now() + 3600000,
      priceType: 'limit',
      limitPrice: 60000,
      slippageBps: 100,
      status: 'paused',
      createdAt: Date.now() - 3600000,
    },
  ];
}

function generateDCAOrders(): DCAOrder[] {
  return [
    {
      id: 'dca_1',
      userAddress: '0x742d35Cc6634C0532',
      tokenIn: 'USDC',
      tokenOut: 'ETH',
      amountPerOrder: '500',
      totalAmount: '10000',
      filledAmount: '3000',
      frequency: 'weekly',
      dayOfWeek: 1,
      hourOfDay: 9,
      nextExecutionTime: Date.now() + 3600000 * 24 * 2,
      status: 'active',
      createdAt: Date.now() - 3600000 * 24 * 14,
      totalOrders: 14,
      completedOrders: 3,
    },
    {
      id: 'dca_2',
      userAddress: '0x742d35Cc6634C0532',
      tokenIn: 'USDC',
      tokenOut: 'BTC',
      amountPerOrder: '1000',
      totalAmount: '50000',
      filledAmount: '14000',
      frequency: 'monthly',
      dayOfMonth: 1,
      hourOfDay: 12,
      nextExecutionTime: Date.now() + 3600000 * 24 * 20,
      status: 'active',
      createdAt: Date.now() - 3600000 * 24 * 60,
      totalOrders: 50,
      completedOrders: 14,
    },
  ];
}

function generateExecutionHistory(): OrderExecution[] {
  return [
    {
      id: 'exec_1',
      orderId: 'twap_1',
      orderType: 'twap',
      tokenIn: 'ETH',
      tokenOut: 'USDC',
      amountIn: '1',
      amountOut: '2448',
      price: 2448,
      timestamp: Date.now() - 1800000,
      txHash: '0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef',
      status: 'success',
    },
    {
      id: 'exec_2',
      orderId: 'twap_1',
      orderType: 'twap',
      tokenIn: 'ETH',
      tokenOut: 'USDC',
      amountIn: '1',
      amountOut: '2452',
      price: 2452,
      timestamp: Date.now() - 3600000 * 2,
      txHash: '0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890',
      status: 'success',
    },
    {
      id: 'exec_3',
      orderId: 'dca_1',
      orderType: 'dca',
      tokenIn: 'USDC',
      tokenOut: 'ETH',
      amountIn: '500',
      amountOut: '0.204',
      price: 2450,
      timestamp: Date.now() - 3600000 * 24 * 7,
      txHash: '0x9876543210fedcba9876543210fedcba9876543210fedcba9876543210fedcba',
      status: 'success',
    },
  ];
}

// ============================================================================
// Main TWAP/DCA Page Component
// ============================================================================

export default function TWAPPage() {
  // State
  const [twapOrders, setTwapOrders] = useState<TWAPOrder[]>([]);
  const [dcaOrders, setDcaOrders] = useState<DCAOrder[]>([]);
  const [executionHistory, setExecutionHistory] = useState<OrderExecution[]>([]);
  const [loading, setLoading] = useState(true);
  const [activeTab, setActiveTab] = useState(0);
  const [showCreateDialog, setShowCreateDialog] = useState(false);
  const [orderType, setOrderType] = useState<'twap' | 'dca'>('twap');

  // Create form state
  const [tokenIn, setTokenIn] = useState('USDC');
  const [tokenOut, setTokenOut] = useState('ETH');
  const [totalAmount, setTotalAmount] = useState('');
  const [numOrders, setNumOrders] = useState(10);
  const [intervalMinutes, setIntervalMinutes] = useState(60);
  const [frequency, setFrequency] = useState<'daily' | 'weekly' | 'monthly'>('weekly');
  const [dayOfWeek, setDayOfWeek] = useState(1);
  const [dayOfMonth, setDayOfMonth] = useState(1);
  const [hourOfDay, setHourOfDay] = useState(9);
  const [priceType, setPriceType] = useState<'market' | 'limit'>('market');
  const [limitPrice, setLimitPrice] = useState('');
  const [slippageBps, setSlippageBps] = useState(50);
  const [creating, setCreating] = useState(false);

  // Snackbar
  const [snackbar, setSnackbar] = useState({
    open: false,
    message: '',
    severity: 'success' as 'success' | 'error' | 'info'
  });

  // ============================================================================
  // Data Loading
  // ============================================================================

  const loadData = useCallback(async () => {
    setLoading(true);
    try {
      await new Promise(resolve => setTimeout(resolve, 500));
      setTwapOrders(generateTWAPOrders());
      setDcaOrders(generateDCAOrders());
      setExecutionHistory(generateExecutionHistory());
    } catch (error) {
      console.error('Failed to load data:', error);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    loadData();
  }, [loadData]);

  // ============================================================================
  // Order Management
  // ============================================================================

  const handleCreateOrder = async () => {
    if (!totalAmount || parseFloat(totalAmount) <= 0) {
      setSnackbar({ open: true, message: 'Please enter a valid amount', severity: 'error' });
      return;
    }

    setCreating(true);
    try {
      await new Promise(resolve => setTimeout(resolve, 1500));

      if (orderType === 'twap') {
        const newOrder: TWAPOrder = {
          id: `twap_${Date.now()}`,
          userAddress: '0x742d35Cc6634C0532',
          tokenIn,
          tokenOut,
          totalAmount,
          filledAmount: '0',
          numOrders,
          completedOrders: 0,
          intervalMinutes,
          startTime: Date.now(),
          endTime: Date.now() + intervalMinutes * numOrders * 60000,
          nextExecutionTime: Date.now() + intervalMinutes * 60000,
          priceType,
          limitPrice: priceType === 'limit' ? parseFloat(limitPrice) : undefined,
          slippageBps,
          status: 'active',
          createdAt: Date.now(),
        };
        setTwapOrders(prev => [...prev, newOrder]);
      } else {
        const newOrder: DCAOrder = {
          id: `dca_${Date.now()}`,
          userAddress: '0x742d35Cc6634C0532',
          tokenIn,
          tokenOut,
          amountPerOrder: (parseFloat(totalAmount) / numOrders).toString(),
          totalAmount,
          filledAmount: '0',
          frequency,
          dayOfWeek: frequency === 'weekly' ? dayOfWeek : undefined,
          dayOfMonth: frequency === 'monthly' ? dayOfMonth : undefined,
          hourOfDay,
          nextExecutionTime: Date.now() + 3600000 * 24 * 7,
          status: 'active',
          createdAt: Date.now(),
          totalOrders: numOrders,
          completedOrders: 0,
        };
        setDcaOrders(prev => [...prev, newOrder]);
      }

      setShowCreateDialog(false);
      setTotalAmount('');
      setSnackbar({ open: true, message: `${orderType.toUpperCase()} order created successfully!`, severity: 'success' });
    } catch (error) {
      setSnackbar({ open: true, message: 'Failed to create order', severity: 'error' });
    } finally {
      setCreating(false);
    }
  };

  const handlePauseResume = async (orderId: string, type: 'twap' | 'dca', currentStatus: string) => {
    const newStatus = currentStatus === 'active' ? 'paused' : 'active';
    
    try {
      if (type === 'twap') {
        setTwapOrders(prev => prev.map(o =>
          o.id === orderId ? { ...o, status: newStatus as TWAPOrder['status'] } : o
        ));
      } else {
        setDcaOrders(prev => prev.map(o =>
          o.id === orderId ? { ...o, status: newStatus as DCAOrder['status'] } : o
        ));
      }
      
      setSnackbar({ open: true, message: `Order ${newStatus}`, severity: 'success' });
    } catch (error) {
      setSnackbar({ open: true, message: 'Failed to update order', severity: 'error' });
    }
  };

  const handleCancel = async (orderId: string, type: 'twap' | 'dca') => {
    try {
      if (type === 'twap') {
        setTwapOrders(prev => prev.map(o =>
          o.id === orderId ? { ...o, status: 'cancelled' as TWAPOrder['status'] } : o
        ));
      } else {
        setDcaOrders(prev => prev.map(o =>
          o.id === orderId ? { ...o, status: 'cancelled' as DCAOrder['status'] } : o
        ));
      }
      
      setSnackbar({ open: true, message: 'Order cancelled', severity: 'success' });
    } catch (error) {
      setSnackbar({ open: true, message: 'Failed to cancel order', severity: 'error' });
    }
  };

  // ============================================================================
  // Statistics
  // ============================================================================

  const totalTwapVolume = twapOrders.reduce((sum, o) => sum + parseFloat(o.totalAmount), 0);
  const totalDCAVolume = dcaOrders.reduce((sum, o) => sum + parseFloat(o.totalAmount), 0);
  const totalFilledVolume = [...twapOrders, ...dcaOrders].reduce((sum, o) => sum + parseFloat(o.filledAmount), 0);
  const activeOrders = twapOrders.filter(o => o.status === 'active').length + dcaOrders.filter(o => o.status === 'active').length;

  // ============================================================================
  // Main Render
  // ============================================================================

  return (
    <Box sx={{ minHeight: '100vh', bgcolor: '#0a0a14', p: 3 }}>
      <Box sx={{ maxWidth: 1400, mx: 'auto' }}>
        {/* Header */}
        <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 4 }}>
          <Box>
            <Typography variant="h4" sx={{ color: 'white', fontWeight: 'bold' }}>
              📅 TWAP & DCA Orders
            </Typography>
            <Typography variant="body2" sx={{ color: '#9ca3af', mt: 1 }}>
              Schedule recurring orders and split large orders over time
            </Typography>
          </Box>
          <Box sx={{ display: 'flex', gap: 2 }}>
            <Button
              variant="outlined"
              startIcon={<Refresh />}
              onClick={loadData}
              sx={{ borderColor: '#3a3a4e', color: 'white' }}
            >
              Refresh
            </Button>
            <Button
              variant="contained"
              startIcon={<Add />}
              onClick={() => setShowCreateDialog(true)}
              sx={{ bgcolor: '#00d4aa', color: 'black' }}
            >
              Create Order
            </Button>
          </Box>
        </Box>

        {/* Stats */}
        <Box sx={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: 2, mb: 4 }}>
          <Card sx={{ bgcolor: '#1a1a2e', borderRadius: 3 }}>
            <CardContent>
              <Typography variant="caption" sx={{ color: '#9ca3af' }}>Total TWAP Volume</Typography>
              <Typography variant="h5" sx={{ color: '#00d4aa', fontWeight: 'bold' }}>
                {formatUSD(totalTwapVolume)}
              </Typography>
            </CardContent>
          </Card>
          <Card sx={{ bgcolor: '#1a1a2e', borderRadius: 3 }}>
            <CardContent>
              <Typography variant="caption" sx={{ color: '#9ca3af' }}>Total DCA Volume</Typography>
              <Typography variant="h5" sx={{ color: '#00d4aa', fontWeight: 'bold' }}>
                {formatUSD(totalDCAVolume)}
              </Typography>
            </CardContent>
          </Card>
          <Card sx={{ bgcolor: '#1a1a2e', borderRadius: 3 }}>
            <CardContent>
              <Typography variant="caption" sx={{ color: '#9ca3af' }}>Filled Volume</Typography>
              <Typography variant="h5" sx={{ color: '#ff9800', fontWeight: 'bold' }}>
                {formatUSD(totalFilledVolume)}
              </Typography>
            </CardContent>
          </Card>
          <Card sx={{ bgcolor: '#1a1a2e', borderRadius: 3 }}>
            <CardContent>
              <Typography variant="caption" sx={{ color: '#9ca3af' }}>Active Orders</Typography>
              <Typography variant="h5" sx={{ color: 'white', fontWeight: 'bold' }}>
                {activeOrders}
              </Typography>
            </CardContent>
          </Card>
        </Box>

        {/* Tabs */}
        <Card sx={{ bgcolor: '#1a1a2e', borderRadius: 3, mb: 3 }}>
          <Tabs
            value={activeTab}
            onChange={(_, v) => setActiveTab(v)}
            sx={{
              borderBottom: '1px solid #2a2a3e',
              '& .MuiTab-root': { color: '#9ca3af' },
              '& .Mui-selected': { color: '#00d4aa' },
            }}
          >
            <Tab label={`TWAP Orders (${twapOrders.length})`} />
            <Tab label={`DCA Orders (${dcaOrders.length})`} />
            <Tab label="Execution History" />
          </Tabs>

          <CardContent sx={{ p: 3 }}>
            {loading ? (
              <Box sx={{ display: 'flex', justifyContent: 'center', py: 5 }}>
                <CircularProgress sx={{ color: '#00d4aa' }} />
              </Box>
            ) : activeTab === 0 ? (
              /* TWAP Orders */
              <Box>
                {twapOrders.length === 0 ? (
                  <Box sx={{ textAlign: 'center', py: 5 }}>
                    <Typography sx={{ color: '#9ca3af', mb: 2 }}>No TWAP orders yet</Typography>
                    <Button
                      variant="contained"
                      startIcon={<Add />}
                      onClick={() => { setOrderType('twap'); setShowCreateDialog(true); }}
                      sx={{ bgcolor: '#00d4aa', color: 'black' }}
                    >
                      Create Your First TWAP
                    </Button>
                  </Box>
                ) : (
                  <TableContainer>
                    <Table>
                      <TableHead>
                        <TableRow>
                          <TableCell sx={{ color: '#9ca3af' }}>Pair</TableCell>
                          <TableCell sx={{ color: '#9ca3af' }}>Progress</TableCell>
                          <TableCell sx={{ color: '#9ca3af' }} align="right">Total Amount</TableCell>
                          <TableCell sx={{ color: '#9ca3af' }} align="right">Filled</TableCell>
                          <TableCell sx={{ color: '#9ca3af' }}>Interval</TableCell>
                          <TableCell sx={{ color: '#9ca3af' }}>Next Execution</TableCell>
                          <TableCell sx={{ color: '#9ca3af' }}>Status</TableCell>
                          <TableCell sx={{ color: '#9ca3af' }} align="right">Actions</TableCell>
                        </TableRow>
                      </TableHead>
                      <TableBody>
                        {twapOrders.map(order => (
                          <TableRow key={order.id} sx={{ '&:hover': { bgcolor: '#2a2a3e' } }}>
                            <TableCell>
                              <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                                <Typography sx={{ color: '#00d4aa', fontWeight: 'bold' }}>{order.tokenIn}</Typography>
                                <TrendingDown sx={{ color: '#9ca3af', fontSize: 16 }} />
                                <Typography sx={{ color: '#ff9800', fontWeight: 'bold' }}>{order.tokenOut}</Typography>
                              </Box>
                            </TableCell>
                            <TableCell>
                              <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                                <LinearProgress
                                  variant="determinate"
                                  value={(parseFloat(order.filledAmount) / parseFloat(order.totalAmount)) * 100}
                                  sx={{ width: 80, height: 6, borderRadius: 3, bgcolor: '#2a2a3e', '& .MuiLinearProgress-bar': { bgcolor: '#00d4aa' } }}
                                />
                                <Typography variant="caption" sx={{ color: '#9ca3af' }}>
                                  {order.completedOrders}/{order.numOrders}
                                </Typography>
                              </Box>
                            </TableCell>
                            <TableCell align="right">
                              <Typography sx={{ color: 'white' }}>{formatUSD(parseFloat(order.totalAmount))}</Typography>
                            </TableCell>
                            <TableCell align="right">
                              <Typography sx={{ color: '#00d4aa' }}>{formatUSD(parseFloat(order.filledAmount))}</Typography>
                            </TableCell>
                            <TableCell>
                              <Chip label={`Every ${INTERVALS.find(i => i.value === order.intervalMinutes)?.label}`} size="small" sx={{ bgcolor: '#2a2a3e' }} />
                            </TableCell>
                            <TableCell>
                              <Typography sx={{ color: order.status === 'active' ? '#00d4aa' : '#9ca3af' }}>
                                {timeUntil(order.nextExecutionTime)}
                              </Typography>
                            </TableCell>
                            <TableCell>
                              <Chip
                                label={order.status}
                                size="small"
                                sx={{
                                  bgcolor: order.status === 'active' ? '#00d4aa20' : '#ff980020',
                                  color: order.status === 'active' ? '#00d4aa' : '#ff9800',
                                }}
                              />
                            </TableCell>
                            <TableCell align="right">
                              <Box sx={{ display: 'flex', gap: 1, justifyContent: 'flex-end' }}>
                                <IconButton
                                  size="small"
                                  onClick={() => handlePauseResume(order.id, 'twap', order.status)}
                                  sx={{ color: order.status === 'active' ? '#ff9800' : '#00d4aa' }}
                                >
                                  {order.status === 'active' ? <Pause fontSize="small" /> : <PlayArrow fontSize="small" />}
                                </IconButton>
                                <IconButton
                                  size="small"
                                  onClick={() => handleCancel(order.id, 'twap')}
                                  sx={{ color: '#ff5722' }}
                                >
                                  <Delete fontSize="small" />
                                </IconButton>
                              </Box>
                            </TableCell>
                          </TableRow>
                        ))}
                      </TableBody>
                    </Table>
                  </TableContainer>
                )}
              </Box>
            ) : activeTab === 1 ? (
              /* DCA Orders */
              <Box>
                {dcaOrders.length === 0 ? (
                  <Box sx={{ textAlign: 'center', py: 5 }}>
                    <Typography sx={{ color: '#9ca3af', mb: 2 }}>No DCA orders yet</Typography>
                    <Button
                      variant="contained"
                      startIcon={<Add />}
                      onClick={() => { setOrderType('dca'); setShowCreateDialog(true); }}
                      sx={{ bgcolor: '#00d4aa', color: 'black' }}
                    >
                      Create Your First DCA
                    </Button>
                  </Box>
                ) : (
                  <TableContainer>
                    <Table>
                      <TableHead>
                        <TableRow>
                          <TableCell sx={{ color: '#9ca3af' }}>Pair</TableCell>
                          <TableCell sx={{ color: '#9ca3af' }}>Frequency</TableCell>
                          <TableCell sx={{ color: '#9ca3af' }} align="right">Per Order</TableCell>
                          <TableCell sx={{ color: '#9ca3af' }} align="right">Total</TableCell>
                          <TableCell sx={{ color: '#9ca3af' }} align="right">Filled</TableCell>
                          <TableCell sx={{ color: '#9ca3af' }}>Next Execution</TableCell>
                          <TableCell sx={{ color: '#9ca3af' }}>Status</TableCell>
                          <TableCell sx={{ color: '#9ca3af' }} align="right">Actions</TableCell>
                        </TableRow>
                      </TableHead>
                      <TableBody>
                        {dcaOrders.map(order => (
                          <TableRow key={order.id} sx={{ '&:hover': { bgcolor: '#2a2a3e' } }}>
                            <TableCell>
                              <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                                <Typography sx={{ color: '#00d4aa', fontWeight: 'bold' }}>{order.tokenIn}</Typography>
                                <TrendingDown sx={{ color: '#9ca3af', fontSize: 16 }} />
                                <Typography sx={{ color: '#ff9800', fontWeight: 'bold' }}>{order.tokenOut}</Typography>
                              </Box>
                            </TableCell>
                            <TableCell>
                              <Chip label={FREQUENCIES.find(f => f.value === order.frequency)?.label || order.frequency} size="small" sx={{ bgcolor: '#2a2a3e' }} />
                            </TableCell>
                            <TableCell align="right">
                              <Typography sx={{ color: 'white' }}>{formatUSD(parseFloat(order.amountPerOrder))}</Typography>
                            </TableCell>
                            <TableCell align="right">
                              <Typography sx={{ color: 'white' }}>{formatUSD(parseFloat(order.totalAmount))}</Typography>
                            </TableCell>
                            <TableCell align="right">
                              <Typography sx={{ color: '#00d4aa' }}>{formatUSD(parseFloat(order.filledAmount))}</Typography>
                            </TableCell>
                            <TableCell>
                              <Typography sx={{ color: order.status === 'active' ? '#00d4aa' : '#9ca3af' }}>
                                {formatDateTime(order.nextExecutionTime)}
                              </Typography>
                            </TableCell>
                            <TableCell>
                              <Chip
                                label={order.status}
                                size="small"
                                sx={{
                                  bgcolor: order.status === 'active' ? '#00d4aa20' : '#ff980020',
                                  color: order.status === 'active' ? '#00d4aa' : '#ff9800',
                                }}
                              />
                            </TableCell>
                            <TableCell align="right">
                              <Box sx={{ display: 'flex', gap: 1, justifyContent: 'flex-end' }}>
                                <IconButton
                                  size="small"
                                  onClick={() => handlePauseResume(order.id, 'dca', order.status)}
                                  sx={{ color: order.status === 'active' ? '#ff9800' : '#00d4aa' }}
                                >
                                  {order.status === 'active' ? <Pause fontSize="small" /> : <PlayArrow fontSize="small" />}
                                </IconButton>
                                <IconButton
                                  size="small"
                                  onClick={() => handleCancel(order.id, 'dca')}
                                  sx={{ color: '#ff5722' }}
                                >
                                  <Delete fontSize="small" />
                                </IconButton>
                              </Box>
                            </TableCell>
                          </TableRow>
                        ))}
                      </TableBody>
                    </Table>
                  </TableContainer>
                )}
              </Box>
            ) : (
              /* Execution History */
              <TableContainer>
                <Table>
                  <TableHead>
                    <TableRow>
                      <TableCell sx={{ color: '#9ca3af' }}>Time</TableCell>
                      <TableCell sx={{ color: '#9ca3af' }}>Type</TableCell>
                      <TableCell sx={{ color: '#9ca3af' }}>Pair</TableCell>
                      <TableCell sx={{ color: '#9ca3af' }} align="right">Amount In</TableCell>
                      <TableCell sx={{ color: '#9ca3af' }} align="right">Amount Out</TableCell>
                      <TableCell sx={{ color: '#9ca3af' }} align="right">Price</TableCell>
                      <TableCell sx={{ color: '#9ca3af' }}>Status</TableCell>
                    </TableRow>
                  </TableHead>
                  <TableBody>
                    {executionHistory.map(exec => (
                      <TableRow key={exec.id} sx={{ '&:hover': { bgcolor: '#2a2a3e' } }}>
                        <TableCell>
                          <Typography variant="caption" sx={{ color: '#9ca3af' }}>
                            {formatDateTime(exec.timestamp)}
                          </Typography>
                        </TableCell>
                        <TableCell>
                          <Chip label={exec.orderType.toUpperCase()} size="small" sx={{ bgcolor: '#2a2a3e', textTransform: 'uppercase', fontSize: '0.65rem' }} />
                        </TableCell>
                        <TableCell>
                          <Typography sx={{ color: '#00d4aa' }}>{exec.tokenIn}</Typography>
                          <Typography sx={{ color: '#9ca3af', fontSize: '0.75rem' }}>→ {exec.tokenOut}</Typography>
                        </TableCell>
                        <TableCell align="right">
                          <Typography sx={{ color: 'white' }}>{exec.amountIn}</Typography>
                        </TableCell>
                        <TableCell align="right">
                          <Typography sx={{ color: '#00d4aa' }}>{exec.amountOut}</Typography>
                        </TableCell>
                        <TableCell align="right">
                          <Typography sx={{ color: 'white' }}>${exec.price.toLocaleString()}</Typography>
                        </TableCell>
                        <TableCell>
                          <Chip
                            icon={exec.status === 'success' ? <CheckCircle sx={{ fontSize: 14 }} /> : undefined}
                            label={exec.status}
                            size="small"
                            sx={{
                              bgcolor: exec.status === 'success' ? '#00d4aa20' : exec.status === 'pending' ? '#ff980020' : '#ff572220',
                              color: exec.status === 'success' ? '#00d4aa' : exec.status === 'pending' ? '#ff9800' : '#ff5722',
                            }}
                          />
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </TableContainer>
            )}
          </CardContent>
        </Card>
      </Box>

      {/* Create Order Dialog */}
      <Dialog
        open={showCreateDialog}
        onClose={() => setShowCreateDialog(false)}
        maxWidth="md"
        fullWidth
        PaperProps={{ sx: { bgcolor: '#1a1a2e', backgroundImage: 'none' } }}
      >
        <DialogTitle sx={{ color: 'white', display: 'flex', justifyContent: 'space-between' }}>
          Create {orderType.toUpperCase()} Order
          <IconButton onClick={() => setShowCreateDialog(false)} sx={{ color: 'white' }}>✕</IconButton>
        </DialogTitle>
        <DialogContent>
          <Box sx={{ mt: 2 }}>
            {/* Order Type Toggle */}
            <Box sx={{ mb: 3 }}>
              <ToggleButtonGroup
                value={orderType}
                exclusive
                onChange={(_, v) => v && setOrderType(v)}
                fullWidth
                sx={{ '& .MuiToggleButton-root': { color: '#9ca3af', borderColor: '#3a3a4e' }, '& .Mui-selected': { bgcolor: '#00d4aa', color: 'black' } }}
              >
                <ToggleButton value="twap">TWAP (Time-Weighted)</ToggleButton>
                <ToggleButton value="dca">DCA (Dollar-Cost Avg)</ToggleButton>
              </ToggleButtonGroup>
            </Box>

            {/* Token Selection */}
            <Box sx={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 2, mb: 3 }}>
              <FormControl fullWidth size="small">
                <InputLabel sx={{ color: '#9ca3af' }}>From Token</InputLabel>
                <Select
                  value={tokenIn}
                  onChange={(e) => setTokenIn(e.target.value)}
                  label="From Token"
                  sx={{ color: 'white', '& .MuiOutlinedInput-notchedOutline': { borderColor: '#3a3a4e' } }}
                >
                  {TOKENS.map(t => (
                    <MenuItem key={t.symbol} value={t.symbol}>
                      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                        <span>{t.icon}</span>
                        <span>{t.symbol}</span>
                      </Box>
                    </MenuItem>
                  ))}
                </Select>
              </FormControl>
              <FormControl fullWidth size="small">
                <InputLabel sx={{ color: '#9ca3af' }}>To Token</InputLabel>
                <Select
                  value={tokenOut}
                  onChange={(e) => setTokenOut(e.target.value)}
                  label="To Token"
                  sx={{ color: 'white', '& .MuiOutlinedInput-notchedOutline': { borderColor: '#3a3a4e' } }}
                >
                  {TOKENS.map(t => (
                    <MenuItem key={t.symbol} value={t.symbol}>
                      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                        <span>{t.icon}</span>
                        <span>{t.symbol}</span>
                      </Box>
                    </MenuItem>
                  ))}
                </Select>
              </FormControl>
            </Box>

            {/* Amount */}
            <TextField
              fullWidth
              type="number"
              label="Total Amount"
              value={totalAmount}
              onChange={(e) => setTotalAmount(e.target.value)}
              InputProps={{
                startAdornment: <InputAdornment sx={{ color: '#9ca3af' }}>$</InputAdornment>,
              }}
              sx={{ mb: 3, '& .MuiInputLabel-root': { color: '#9ca3af' }, '& input': { color: 'white' }, '& .MuiOutlinedInput-root': { '& fieldset': { borderColor: '#3a3a4e' } } }}
            />

            {/* TWAP Settings */}
            {orderType === 'twap' ? (
              <Box>
                <Box sx={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 2, mb: 3 }}>
                  <FormControl fullWidth size="small">
                    <InputLabel sx={{ color: '#9ca3af' }}>Number of Orders</InputLabel>
                    <Select
                      value={numOrders}
                      onChange={(e) => setNumOrders(e.target.value as number)}
                      label="Number of Orders"
                      sx={{ color: 'white', '& .MuiOutlinedInput-notchedOutline': { borderColor: '#3a3a4e' } }}
                    >
                      {[5, 10, 20, 50, 100].map(n => (
                        <MenuItem key={n} value={n}>{n} orders</MenuItem>
                      ))}
                    </Select>
                  </FormControl>
                  <FormControl fullWidth size="small">
                    <InputLabel sx={{ color: '#9ca3af' }}>Interval</InputLabel>
                    <Select
                      value={intervalMinutes}
                      onChange={(e) => setIntervalMinutes(e.target.value as number)}
                      label="Interval"
                      sx={{ color: 'white', '& .MuiOutlinedInput-notchedOutline': { borderColor: '#3a3a4e' } }}
                    >
                      {INTERVALS.map(int => (
                        <MenuItem key={int.value} value={int.value}>{int.label}</MenuItem>
                      ))}
                    </Select>
                  </FormControl>
                </Box>

                <FormControl fullWidth size="small" sx={{ mb: 3 }}>
                  <InputLabel sx={{ color: '#9ca3af' }}>Price Type</InputLabel>
                  <Select
                    value={priceType}
                    onChange={(e) => setPriceType(e.target.value as 'market' | 'limit')}
                    label="Price Type"
                    sx={{ color: 'white', '& .MuiOutlinedInput-notchedOutline': { borderColor: '#3a3a4e' } }}
                  >
                    <MenuItem value="market">Market Price</MenuItem>
                    <MenuItem value="limit">Limit Price</MenuItem>
                  </Select>
                </FormControl>

                {priceType === 'limit' && (
                  <TextField
                    fullWidth
                    type="number"
                    label="Limit Price"
                    value={limitPrice}
                    onChange={(e) => setLimitPrice(e.target.value)}
                    sx={{ mb: 3, '& .MuiInputLabel-root': { color: '#9ca3af' }, '& input': { color: 'white' }, '& .MuiOutlinedInput-root': { '& fieldset': { borderColor: '#3a3a4e' } } }}
                  />
                )}
              </Box>
            ) : (
              /* DCA Settings */
              <Box>
                <FormControl fullWidth size="small" sx={{ mb: 3 }}>
                  <InputLabel sx={{ color: '#9ca3af' }}>Frequency</InputLabel>
                  <Select
                    value={frequency}
                    onChange={(e) => setFrequency(e.target.value as 'daily' | 'weekly' | 'monthly')}
                    label="Frequency"
                    sx={{ color: 'white', '& .MuiOutlinedInput-notchedOutline': { borderColor: '#3a3a4e' } }}
                  >
                    {FREQUENCIES.map(f => (
                      <MenuItem key={f.value} value={f.value}>{f.label}</MenuItem>
                    ))}
                  </Select>
                </FormControl>

                <Box sx={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 2, mb: 3 }}>
                  {frequency === 'weekly' && (
                    <FormControl fullWidth size="small">
                      <InputLabel sx={{ color: '#9ca3af' }}>Day of Week</InputLabel>
                      <Select
                        value={dayOfWeek}
                        onChange={(e) => setDayOfWeek(e.target.value as number)}
                        label="Day of Week"
                        sx={{ color: 'white', '& .MuiOutlinedInput-notchedOutline': { borderColor: '#3a3a4e' } }}
                      >
                        {DAYS_OF_WEEK.map((day, i) => (
                          <MenuItem key={i} value={i}>{day}</MenuItem>
                        ))}
                      </Select>
                    </FormControl>
                  )}
                  {frequency === 'monthly' && (
                    <FormControl fullWidth size="small">
                      <InputLabel sx={{ color: '#9ca3af' }}>Day of Month</InputLabel>
                      <Select
                        value={dayOfMonth}
                        onChange={(e) => setDayOfMonth(e.target.value as number)}
                        label="Day of Month"
                        sx={{ color: 'white', '& .MuiOutlinedInput-notchedOutline': { borderColor: '#3a3a4e' } }}
                      >
                        {Array.from({ length: 28 }, (_, i) => (
                          <MenuItem key={i + 1} value={i + 1}>{i + 1}</MenuItem>
                        ))}
                      </Select>
                    </FormControl>
                  )}
                  <FormControl fullWidth size="small">
                    <InputLabel sx={{ color: '#9ca3af' }}>Time of Day</InputLabel>
                    <Select
                      value={hourOfDay}
                      onChange={(e) => setHourOfDay(e.target.value as number)}
                      label="Time of Day"
                      sx={{ color: 'white', '& .MuiOutlinedInput-notchedOutline': { borderColor: '#3a3a4e' } }}
                    >
                      {Array.from({ length: 24 }, (_, i) => (
                        <MenuItem key={i} value={i}>{i.toString().padStart(2, '0')}:00</MenuItem>
                      ))}
                    </Select>
                  </FormControl>
                </Box>
              </Box>
            )}

            {/* Slippage */}
            <Box sx={{ mb: 3 }}>
              <Typography variant="caption" sx={{ color: '#9ca3af', mb: 1, display: 'block' }}>
                Slippage Tolerance: {slippageBps / 100}%
              </Typography>
              <Slider
                value={slippageBps}
                onChange={(_, v) => setSlippageBps(v as number)}
                min={10}
                max={500}
                step={10}
                marks={[{ value: 50, label: '0.5%' }, { value: 100, label: '1%' }, { value: 500, label: '5%' }]}
                sx={{ color: '#00d4aa' }}
              />
            </Box>

            {/* Summary */}
            <Box sx={{ bgcolor: '#2a2a3e', p: 2, borderRadius: 2 }}>
              <Typography variant="caption" sx={{ color: '#9ca3af' }}>
                {orderType === 'twap' ? (
                  <>Your order will be split into {numOrders} orders over {INTERVALS.find(i => i.value === intervalMinutes)?.label?.toLowerCase()}.</>
                ) : (
                  <>You will buy approximately {(parseFloat(totalAmount) / (orderType === 'dca' ? numOrders : 1)).toFixed(2)} {tokenOut} per order on a {frequency} basis.</>
                )}
              </Typography>
            </Box>
          </Box>
        </DialogContent>
        <DialogActions sx={{ p: 3 }}>
          <Button onClick={() => setShowCreateDialog(false)} sx={{ color: '#9ca3af' }}>Cancel</Button>
          <Button
            variant="contained"
            onClick={handleCreateOrder}
            disabled={creating || !totalAmount}
            sx={{ bgcolor: '#00d4aa', color: 'black' }}
          >
            {creating ? <CircularProgress size={20} sx={{ color: 'black' }} /> : `Create ${orderType.toUpperCase()}`}
          </Button>
        </DialogActions>
      </Dialog>

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