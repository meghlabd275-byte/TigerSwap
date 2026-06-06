'use client';

import React, { useState, useEffect, useCallback } from 'react';
import {
  Box, Typography, Card, CardContent, Button, TextField, Tabs, Tab, Chip,
  Table, TableBody, TableCell, TableContainer, TableHead, TableRow,
  Paper, IconButton, Dialog, DialogTitle, DialogContent, DialogActions,
  Alert, Select, MenuItem, FormControl, InputLabel, Switch,
  FormControlLabel, Slider, Grid, Avatar, Divider, List, ListItem,
  ListItemText, ListItemIcon, LinearProgress, Tooltip, Badge
} from '@mui/material';
import {
  AccountBalance, Speed, ShowChart, TrendingUp, TrendingDown,
  Settings, Add, PlayArrow, Stop, Pause, Delete, Refresh,
  Build, Security, MonetizationOn, Warning, CheckCircle, Error as ErrorIcon,
  Timeline, People, Store, Dashboard, Notifications, History
} from '@mui/icons-material';

// ============================================================================
// Types & Interfaces
// ============================================================================

interface User {
  id: string;
  address: string;
  name: string;
  role: 'admin' | 'operator' | 'client';
  email?: string;
  createdAt: number;
  isActive: boolean;
}

interface Bot {
  id: string;
  name: string;
  type: 'market_maker' | 'arbitrage' | 'sniper' | 'liquidity' | 'front_run' | 'mev' | 'flash_loan' | 'cross_chain' | 'perp_hedge';
  status: 'running' | 'paused' | 'stopped';
  owner: string;
  profit: number;
  volume: number;
  trades: number;
  winRate: number;
  createdAt: number;
  lastActive: number;
  config: BotConfig;
}

interface BotConfig {
  minInvestment: number;
  maxInvestment: number;
  targetApy: number;
  riskLevel: number;
  maxDailyLoss: number;
  stopLoss: number;
}

interface BotStats {
  totalVolume: number;
  totalProfit: number;
  totalTrades: number;
  successfulTrades: number;
  failedTrades: number;
  winRate: number;
  uptime: number;
}

interface Exchange {
  id: string;
  name: string;
  isActive: boolean;
  minTrade: number;
  maxTrade: number;
  fee: number;
}

interface Transaction {
  id: string;
  botId: string;
  exchange: string;
  type: 'buy' | 'sell';
  amount: number;
  price: number;
  profit: number;
  status: 'success' | 'failed';
  timestamp: number;
}

// ============================================================================
// Constants
// ============================================================================

const BOT_TYPES = [
  { value: 'market_maker', label: 'Market Maker', description: 'Provide liquidity and earn spread', icon: <MonetizationOn /> },
  { value: 'arbitrage', label: 'Arbitrage', description: 'Profit from price differences', icon: <ShowChart /> },
  { value: 'sniper', label: 'Sniper', description: 'Fast trade execution', icon: <Speed /> },
  { value: 'liquidity', label: 'Liquidity', description: 'Deepen order books', icon: <Build /> },
  { value: 'mev', label: 'MEV', description: 'Extract MEV from mempool', icon: <TrendingUp /> },
  { value: 'flash_loan', label: 'Flash Loan', description: 'Risk-free flash loan strategies', icon: <TrendingDown /> },
];

const EXCHANGES = [
  { id: 'uniswap', name: 'Uniswap', isActive: true, minTrade: 100, maxTrade: 1000000, fee: 0.3 },
  { id: 'pancakeswap', name: 'PancakeSwap', isActive: true, minTrade: 50, maxTrade: 500000, fee: 0.25 },
  { id: 'sushiswap', name: 'SushiSwap', isActive: true, minTrade: 100, maxTrade: 500000, fee: 0.3 },
  { id: 'curve', name: 'Curve', isActive: true, minTrade: 1000, maxTrade: 10000000, fee: 0.04 },
  { id: 'dydx', name: 'dYdX', isActive: true, minTrade: 100, maxTrade: 1000000, fee: 0.2 },
];

// ============================================================================
// Component
// ============================================================================

export default function BotDashboard() {
  // Auth State
  const [currentUser, setCurrentUser] = useState<User | null>(null);
  const [isConnected, setIsConnected] = useState(false);
  
  // UI State
  const [activeTab, setActiveTab] = useState(0);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);
  
  // Data State
  const [users, setUsers] = useState<User[]>([]);
  const [bots, setBots] = useState<Bot[]>([]);
  const [transactions, setTransactions] = useState<Transaction[]>([]);
  const [exchanges, setExchanges] = useState<Exchange[]>(EXCHANGES);
  
  // Dialogs
  const [createBotDialog, setCreateBotDialog] = useState(false);
  const [createUserDialog, setCreateUserDialog] = useState(false);
  const [confirmDialog, setConfirmDialog] = useState({ open: false, title: '', message: '', action: '' });
  
  // New Bot Form
  const [newBot, setNewBot] = useState({
    name: '',
    type: 'market_maker' as Bot['type'],
    minInvestment: 1000,
    maxInvestment: 10000,
    targetApy: 20,
    riskLevel: 5,
  });
  
  // New User Form
  const [newUser, setNewUser] = useState({
    name: '',
    email: '',
    role: 'client' as User['role'],
    address: '',
  });
  
  // ============================================================================
  // Effects
  // ============================================================================
  
  useEffect(() => {
    if (isConnected) {
      loadData();
    }
  }, [isConnected]);
  
  // ============================================================================
  // Data Loading
  // ============================================================================
  
  const loadData = useCallback(async () => {
    setLoading(true);
    try {
      // Simulate API calls
      await new Promise(resolve => setTimeout(resolve, 500));
      
      // Mock users
      setUsers([
        {
          id: '1',
          address: '0x1234...abcd',
          name: 'Admin',
          role: 'admin',
          email: 'admin@tigerswap.io',
          createdAt: Date.now() - 86400000 * 30,
          isActive: true,
        },
        {
          id: '2',
          address: '0x5678...efgh',
          name: 'Bot Operator',
          role: 'operator',
          email: 'operator@tigerswap.io',
          createdAt: Date.now() - 86400000 * 20,
          isActive: true,
        },
        {
          id: '3',
          address: '0xabcd...1234',
          name: 'Client User',
          role: 'client',
          email: 'client@example.com',
          createdAt: Date.now() - 86400000 * 10,
          isActive: true,
        },
      ]);
      
      // Mock bots
      setBots([
        {
          id: 'bot1',
          name: 'ETH-USDC Market Maker',
          type: 'market_maker',
          status: 'running',
          owner: '0x1234...abcd',
          profit: 15420.5,
          volume: 2500000,
          trades: 1520,
          winRate: 92.5,
          createdAt: Date.now() - 86400000 * 15,
          lastActive: Date.now() - 300000,
          config: { minInvestment: 5000, maxInvestment: 50000, targetApy: 25, riskLevel: 3, maxDailyLoss: 1000, stopLoss: 5 },
        },
        {
          id: 'bot2',
          name: 'BTC Arbitrage',
          type: 'arbitrage',
          status: 'running',
          owner: '0x5678...efgh',
          profit: 8750.2,
          volume: 1800000,
          trades: 850,
          winRate: 88.2,
          createdAt: Date.now() - 86400000 * 10,
          lastActive: Date.now() - 600000,
          config: { minInvestment: 10000, maxInvestment: 100000, targetApy: 40, riskLevel: 6, maxDailyLoss: 2000, stopLoss: 8 },
        },
        {
          id: 'bot3',
          name: 'SOL Sniper',
          type: 'sniper',
          status: 'paused',
          owner: '0xabcd...1234',
          profit: 3200.0,
          volume: 450000,
          trades: 120,
          winRate: 75.0,
          createdAt: Date.now() - 86400000 * 5,
          lastActive: Date.now() - 3600000,
          config: { minInvestment: 1000, maxInvestment: 10000, targetApy: 50, riskLevel: 8, maxDailyLoss: 500, stopLoss: 10 },
        },
      ]);
      
      // Mock transactions
      setTransactions([
        { id: 'tx1', botId: 'bot1', exchange: 'Uniswap', type: 'buy', amount: 50000, price: 3500, profit: 150, status: 'success', timestamp: Date.now() - 60000 },
        { id: 'tx2', botId: 'bot1', exchange: 'SushiSwap', type: 'sell', amount: 48000, price: 3510, profit: 480, status: 'success', timestamp: Date.now() - 120000 },
        { id: 'tx3', botId: 'bot2', exchange: 'Curve', type: 'buy', amount: 100000, price: 1.0, profit: -50, status: 'failed', timestamp: Date.now() - 180000 },
        { id: 'tx4', botId: 'bot2', exchange: 'Uniswap', type: 'sell', amount: 75000, price: 3505, profit: 375, status: 'success', timestamp: Date.now() - 240000 },
      ]);
      
      setSuccess('Data loaded successfully');
    } catch (err: any) {
      setError(err.message || 'Failed to load data');
    } finally {
      setLoading(false);
    }
  }, []);
  
  // ============================================================================
  // User Management
  // ============================================================================
  
  const handleConnect = useCallback(async () => {
    // Simulate wallet connection
    setIsConnected(true);
    setCurrentUser({
      id: '1',
      address: '0x1234...abcd',
      name: 'Admin',
      role: 'admin',
      email: 'admin@tigerswap.io',
      createdAt: Date.now(),
      isActive: true,
    });
  }, []);
  
  const handleCreateUser = useCallback(async () => {
    if (!newUser.name || !newUser.address) {
      setError('Please fill all fields');
      return;
    }
    
    setLoading(true);
    try {
      await new Promise(resolve => setTimeout(resolve, 500));
      
      setUsers([...users, {
        id: String(users.length + 1),
        ...newUser,
        createdAt: Date.now(),
        isActive: true,
      }]);
      
      setSuccess(`User created: ${newUser.name}`);
      setCreateUserDialog(false);
      setNewUser({ name: '', email: '', role: 'client', address: '' });
    } catch (err: any) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  }, [newUser, users]);
  
  const handleDeleteUser = useCallback(async (userId: string) => {
    setLoading(true);
    try {
      await new Promise(resolve => setTimeout(resolve, 300));
      setUsers(users.filter(u => u.id !== userId));
      setSuccess('User deleted');
    } catch (err: any) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  }, [users]);
  
  // ============================================================================
  // Bot Management
  // ============================================================================
  
  const handleCreateBot = useCallback(async () => {
    if (!newBot.name) {
      setError('Please enter bot name');
      return;
    }
    
    setLoading(true);
    try {
      await new Promise(resolve => setTimeout(resolve, 500));
      
      const bot: Bot = {
        id: `bot${bots.length + 1}`,
        name: newBot.name,
        type: newBot.type,
        status: 'running',
        owner: currentUser?.address || '0x0000...0000',
        profit: 0,
        volume: 0,
        trades: 0,
        winRate: 0,
        createdAt: Date.now(),
        lastActive: Date.now(),
        config: {
          minInvestment: newBot.minInvestment,
          maxInvestment: newBot.maxInvestment,
          targetApy: newBot.targetApy,
          riskLevel: newBot.riskLevel,
          maxDailyLoss: newBot.maxInvestment * 0.1,
          stopLoss: 5,
        },
      };
      
      setBots([...bots, bot]);
      setSuccess(`Bot created: ${newBot.name}`);
      setCreateBotDialog(false);
      setNewBot({ name: '', type: 'market_maker', minInvestment: 1000, maxInvestment: 10000, targetApy: 20, riskLevel: 5 });
    } catch (err: any) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  }, [newBot, bots, currentUser]);
  
  const handleStartBot = useCallback(async (botId: string) => {
    setLoading(true);
    try {
      await new Promise(resolve => setTimeout(resolve, 300));
      setBots(bots.map(b => b.id === botId ? { ...b, status: 'running' as const } : b));
      setSuccess('Bot started');
    } catch (err: any) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  }, [bots]);
  
  const handleStopBot = useCallback(async (botId: string) => {
    setLoading(true);
    try {
      await new Promise(resolve => setTimeout(resolve, 300));
      setBots(bots.map(b => b.id === botId ? { ...b, status: 'stopped' as const } : b));
      setSuccess('Bot stopped');
    } catch (err: any) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  }, [bots]);
  
  const handlePauseBot = useCallback(async (botId: string) => {
    setLoading(true);
    try {
      await new Promise(resolve => setTimeout(resolve, 300));
      setBots(bots.map(b => b.id === botId ? { ...b, status: 'paused' as const } : b));
      setSuccess('Bot paused');
    } catch (err: any) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  }, [bots]);
  
  const handleDeleteBot = useCallback(async (botId: string) => {
    setConfirmDialog({
      open: true,
      title: 'Delete Bot',
      message: 'Are you sure you want to delete this bot? This action cannot be undone.',
      action: 'delete',
    });
  }, []);
  
  const confirmAction = useCallback(async (action: string) => {
    if (action === 'delete') {
      setLoading(true);
      try {
        await new Promise(resolve => setTimeout(resolve, 300));
        // Delete logic would go here
        setSuccess('Bot deleted');
      } catch (err: any) {
        setError(err.message);
      } finally {
        setLoading(false);
        setConfirmDialog({ open: false, title: '', message: '', action: '' });
      }
    }
  }, []);
  
  // ============================================================================
  // Helper Functions
  // ============================================================================
  
  const formatCurrency = (amount: number) => {
    return amount.toLocaleString('en-US', { style: 'currency', currency: 'USD' });
  };
  
  const formatNumber = (num: number) => {
    return num.toLocaleString('en-US');
  };
  
  const formatPercent = (percent: number) => {
    return `${percent.toFixed(1)}%`;
  };
  
  const formatDate = (timestamp: number) => {
    return new Date(timestamp).toLocaleDateString('en-US', {
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    });
  };
  
  // ============================================================================
  // Render
  // ============================================================================
  
  if (!isConnected) {
    return (
      <Box sx={{ p: 4, textAlign: 'center' }}>
        <Typography variant="h4" gutterBottom>Bot Platform Dashboard</Typography>
        <Typography color="text.secondary" sx={{ mb: 4 }}>
          Connect your wallet to manage trading bots
        </Typography>
        <Button variant="contained" size="large" onClick={handleConnect}>
          Connect Wallet
        </Button>
      </Box>
    );
  }
  
  return (
    <Box sx={{ p: 3 }}>
      {/* Header */}
      <Box sx={{ mb: 3, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <Box>
          <Typography variant="h5" fontWeight="bold">Bot Platform</Typography>
          <Typography variant="body2" color="text.secondary">
            Role: {currentUser?.role.toUpperCase()} | Address: {currentUser?.address}
          </Typography>
        </Box>
        <Box sx={{ display: 'flex', gap: 2 }}>
          <Button variant="outlined" startIcon={<Refresh />} onClick={loadData}>Refresh</Button>
          {currentUser?.role === 'admin' && (
            <Button variant="contained" startIcon={<People />} onClick={() => setCreateUserDialog(true)}>
              Add User
            </Button>
          )}
          <Button variant="contained" startIcon={<Add />} onClick={() => setCreateBotDialog(true)}>
            Create Bot
          </Button>
        </Box>
      </Box>
      
      {/* Alerts */}
      {error && (
        <Alert severity="error" sx={{ mb: 2 }} onClose={() => setError(null)}>
          {error}
        </Alert>
      )}
      {success && (
        <Alert severity="success" sx={{ mb: 2 }} onClose={() => setSuccess(null)}>
          {success}
        </Alert>
      )}
      
      {/* Loading */}
      {loading && <LinearProgress sx={{ mb: 2 }} />}
      
      {/* Stats Cards */}
      <Grid container spacing={3} sx={{ mb: 3 }}>
        <Grid item xs={12} sm={6} md={3}>
          <Card>
            <CardContent>
              <Typography color="text.secondary" gutterBottom>Total Bots</Typography>
              <Typography variant="h4">{bots.length}</Typography>
            </CardContent>
          </Card>
        </Grid>
        <Grid item xs={12} sm={6} md={3}>
          <Card>
            <CardContent>
              <Typography color="text.secondary" gutterBottom>Total Profit</Typography>
              <Typography variant="h4" color="success.main">
                {formatCurrency(bots.reduce((a, b) => a + b.profit, 0))}
              </Typography>
            </CardContent>
          </Card>
        </Grid>
        <Grid item xs={12} sm={6} md={3}>
          <Card>
            <CardContent>
              <Typography color="text.secondary" gutterBottom>Total Volume</Typography>
              <Typography variant="h4">
                {formatCurrency(bots.reduce((a, b) => a + b.volume, 0))}
              </Typography>
            </CardContent>
          </Card>
        </Grid>
        <Grid item xs={12} sm={6} md={3}>
          <Card>
            <CardContent>
              <Typography color="text.secondary" gutterBottom>Active Bots</Typography>
              <Typography variant="h4">
                {bots.filter(b => b.status === 'running').length}
              </Typography>
            </CardContent>
          </Card>
        </Grid>
      </Grid>
      
      {/* Tabs */}
      <Tabs value={activeTab} onChange={(_, v) => setActiveTab(v)} sx={{ mb: 3 }}>
        <Tab label="Bots" icon={<Dashboard />} iconPosition="start" />
        <Tab label="Transactions" icon={<History />} iconPosition="start" />
        <Tab label="Exchanges" icon={<Store />} iconPosition="start" />
        {currentUser?.role === 'admin' && (
          <Tab label="Users" icon={<People />} iconPosition="start" />
        )}
        <Tab label="Settings" icon={<Settings />} iconPosition="start" />
      </Tabs>
      
      {/* Bots Tab */}
      {activeTab === 0 && (
        <TableContainer component={Paper}>
          <Table>
            <TableHead>
              <TableRow>
                <TableCell>Name</TableCell>
                <TableCell>Type</TableCell>
                <TableCell>Status</TableCell>
                <TableCell align="right">Profit</TableCell>
                <TableCell align="right">Volume</TableCell>
                <TableCell align="right">Trades</TableCell>
                <TableCell align="right">Win Rate</TableCell>
                <TableCell>Actions</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {bots.map(bot => (
                <TableRow key={bot.id}>
                  <TableCell>{bot.name}</TableCell>
                  <TableCell>
                    <Chip 
                      label={bot.type.replace('_', ' ')} 
                      size="small" 
                      icon={BOT_TYPES.find(t => t.value === bot.type)?.icon}
                    />
                  </TableCell>
                  <TableCell>
                    <Chip 
                      label={bot.status}
                      size="small"
                      color={bot.status === 'running' ? 'success' : bot.status === 'paused' ? 'warning' : 'default'}
                    />
                  </TableCell>
                  <TableCell align="right" sx={{ color: bot.profit >= 0 ? 'success.main' : 'error.main' }}>
                    {formatCurrency(bot.profit)}
                  </TableCell>
                  <TableCell align="right">{formatCurrency(bot.volume)}</TableCell>
                  <TableCell align="right">{bot.trades}</TableCell>
                  <TableCell align="right">{formatPercent(bot.winRate)}</TableCell>
                  <TableCell>
                    <Box sx={{ display: 'flex', gap: 1 }}>
                      {bot.status === 'stopped' ? (
                        <IconButton size="small" color="success" onClick={() => handleStartBot(bot.id)}>
                          <PlayArrow />
                        </IconButton>
                      ) : (
                        <IconButton size="small" color="warning" onClick={() => handlePauseBot(bot.id)}>
                          <Pause />
                        </IconButton>
                      )}
                      <IconButton size="small" color="error" onClick={() => handleStopBot(bot.id)}>
                        <Stop />
                      </IconButton>
                      <IconButton size="small" onClick={() => handleDeleteBot(bot.id)}>
                        <Delete />
                      </IconButton>
                    </Box>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </TableContainer>
      )}
      
      {/* Transactions Tab */}
      {activeTab === 1 && (
        <TableContainer component={Paper}>
          <Table>
            <TableHead>
              <TableRow>
                <TableCell>Time</TableCell>
                <TableCell>Bot</TableCell>
                <TableCell>Exchange</TableCell>
                <TableCell>Type</TableCell>
                <TableCell align="right">Amount</TableCell>
                <TableCell align="right">Price</TableCell>
                <TableCell align="right">Profit</TableCell>
                <TableCell>Status</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {transactions.map(tx => (
                <TableRow key={tx.id}>
                  <TableCell>{formatDate(tx.timestamp)}</TableCell>
                  <TableCell>{bots.find(b => b.id === tx.botId)?.name}</TableCell>
                  <TableCell>{tx.exchange}</TableCell>
                  <TableCell sx={{ color: tx.type === 'buy' ? 'success.main' : 'error.main' }}>
                    {tx.type.toUpperCase()}
                  </TableCell>
                  <TableCell align="right">{formatCurrency(tx.amount)}</TableCell>
                  <TableCell align="right">{formatCurrency(tx.price)}</TableCell>
                  <TableCell align="right" sx={{ color: tx.profit >= 0 ? 'success.main' : 'error.main' }}>
                    {formatCurrency(tx.profit)}
                  </TableCell>
                  <TableCell>
                    <Chip 
                      label={tx.status}
                      size="small"
                      color={tx.status === 'success' ? 'success' : 'error'}
                      icon={tx.status === 'success' ? <CheckCircle /> : <ErrorIcon />}
                    />
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </TableContainer>
      )}
      
      {/* Exchanges Tab */}
      {activeTab === 2 && (
        <TableContainer component={Paper}>
          <Table>
            <TableHead>
              <TableRow>
                <TableCell>Name</TableCell>
                <TableCell>Status</TableCell>
                <TableCell align="right">Min Trade</TableCell>
                <TableCell align="right">Max Trade</TableCell>
                <TableCell align="right">Fee</TableCell>
                <TableCell>Actions</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {exchanges.map(ex => (
                <TableRow key={ex.id}>
                  <TableCell>{ex.name}</TableCell>
                  <TableCell>
                    <Chip 
                      label={ex.isActive ? 'Active' : 'Inactive'}
                      size="small"
                      color={ex.isActive ? 'success' : 'default'}
                    />
                  </TableCell>
                  <TableCell align="right">{formatCurrency(ex.minTrade)}</TableCell>
                  <TableCell align="right">{formatCurrency(ex.maxTrade)}</TableCell>
                  <TableCell align="right">{ex.fee}%</TableCell>
                  <TableCell>
                    <FormControlLabel
                      control={
                        <Switch 
                          checked={ex.isActive}
                          onChange={() => {
                            setExchanges(exchanges.map(e => 
                              e.id === ex.id ? { ...e, isActive: !e.isActive } : e
                            ));
                          }}
                        />
                      }
                      label={ex.isActive ? 'Enabled' : 'Disabled'}
                    />
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </TableContainer>
      )}
      
      {/* Users Tab (Admin Only) */}
      {activeTab === 3 && currentUser?.role === 'admin' && (
        <TableContainer component={Paper}>
          <Table>
            <TableHead>
              <TableRow>
                <TableCell>Name</TableCell>
                <TableCell>Address</TableCell>
                <TableCell>Role</TableCell>
                <TableCell>Email</TableCell>
                <TableCell>Created</TableCell>
                <TableCell>Status</TableCell>
                <TableCell>Actions</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {users.map(user => (
                <TableRow key={user.id}>
                  <TableCell>{user.name}</TableCell>
                  <TableCell>{user.address}</TableCell>
                  <TableCell>
                    <Chip 
                      label={user.role}
                      size="small"
                      color={user.role === 'admin' ? 'error' : user.role === 'operator' ? 'warning' : 'primary'}
                    />
                  </TableCell>
                  <TableCell>{user.email}</TableCell>
                  <TableCell>{formatDate(user.createdAt)}</TableCell>
                  <TableCell>
                    <Chip 
                      label={user.isActive ? 'Active' : 'Inactive'}
                      size="small"
                      color={user.isActive ? 'success' : 'error'}
                    />
                  </TableCell>
                  <TableCell>
                    {user.role !== 'admin' && (
                      <IconButton size="small" onClick={() => handleDeleteUser(user.id)}>
                        <Delete />
                      </IconButton>
                    )}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </TableContainer>
      )}
      
      {/* Settings Tab */}
      {activeTab === 4 && (
        <Card>
          <CardContent>
            <Typography variant="h6" gutterBottom>Platform Settings</Typography>
            <Divider sx={{ my: 2 }} />
            
            <Grid container spacing={3}>
              <Grid item xs={12} md={6}>
                <Typography variant="subtitle1" gutterBottom>Protocol Fee</Typography>
                <TextField fullWidth defaultValue="0.5" type="number" size="small" />%
              </Grid>
              <Grid item xs={12} md={6}>
                <Typography variant="subtitle1" gutterBottom>Verification Fee</Typography>
                <TextField fullWidth defaultValue="0.05" type="number" size="small" /> ETH
              </Grid>
              <Grid item xs={12}>
                <FormControlLabel
                  control={<Switch defaultChecked />}
                  label="Require Token Verification"
                />
              </Grid>
              <Grid item xs={12}>
                <FormControlLabel
                  control={<Switch defaultChecked />}
                  label="Require KYC"
                />
              </Grid>
            </Grid>
          </CardContent>
        </Card>
      )}
      
      {/* Create Bot Dialog */}
      <Dialog open={createBotDialog} onClose={() => setCreateBotDialog(false)} maxWidth="sm" fullWidth>
        <DialogTitle>Create New Bot</DialogTitle>
        <DialogContent>
          <Box sx={{ pt: 2, display: 'flex', flexDirection: 'column', gap: 2 }}>
            <TextField
              fullWidth
              label="Bot Name"
              value={newBot.name}
              onChange={(e) => setNewBot({ ...newBot, name: e.target.value })}
            />
            <FormControl fullWidth>
              <InputLabel>Bot Type</InputLabel>
              <Select
                value={newBot.type}
                label="Bot Type"
                onChange={(e) => setNewBot({ ...newBot, type: e.target.value as Bot['type'] })}
              >
                {BOT_TYPES.map(bt => (
                  <MenuItem key={bt.value} value={bt.value}>
                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                      {bt.icon} {bt.label}
                    </Box>
                  </MenuItem>
                ))}
              </Select>
            </FormControl>
            <TextField
              fullWidth
              label="Min Investment (USD)"
              type="number"
              value={newBot.minInvestment}
              onChange={(e) => setNewBot({ ...newBot, minInvestment: parseInt(e.target.value) })}
            />
            <TextField
              fullWidth
              label="Max Investment (USD)"
              type="number"
              value={newBot.maxInvestment}
              onChange={(e) => setNewBot({ ...newBot, maxInvestment: parseInt(e.target.value) })}
            />
            <TextField
              fullWidth
              label="Target APY (%)"
              type="number"
              value={newBot.targetApy}
              onChange={(e) => setNewBot({ ...newBot, targetApy: parseInt(e.target.value) })}
            />
            <Box>
              <Typography variant="subtitle2" gutterBottom>Risk Level: {newBot.riskLevel}</Typography>
              <Slider
                value={newBot.riskLevel}
                onChange={(_, v) => setNewBot({ ...newBot, riskLevel: v as number })}
                min={1}
                max={10}
                marks={[
                  { value: 1, label: 'Low' },
                  { value: 5, label: 'Medium' },
                  { value: 10, label: 'High' },
                ]}
              />
            </Box>
          </Box>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setCreateBotDialog(false)}>Cancel</Button>
          <Button variant="contained" onClick={handleCreateBot} disabled={loading}>
            Create Bot
          </Button>
        </DialogActions>
      </Dialog>
      
      {/* Create User Dialog */}
      <Dialog open={createUserDialog} onClose={() => setCreateUserDialog(false)} maxWidth="sm" fullWidth>
        <DialogTitle>Add New User</DialogTitle>
        <DialogContent>
          <Box sx={{ pt: 2, display: 'flex', flexDirection: 'column', gap: 2 }}>
            <TextField
              fullWidth
              label="Name"
              value={newUser.name}
              onChange={(e) => setNewUser({ ...newUser, name: e.target.value })}
            />
            <TextField
              fullWidth
              label="Email"
              type="email"
              value={newUser.email}
              onChange={(e) => setNewUser({ ...newUser, email: e.target.value })}
            />
            <TextField
              fullWidth
              label="Wallet Address"
              value={newUser.address}
              onChange={(e) => setNewUser({ ...newUser, address: e.target.value })}
            />
            <FormControl fullWidth>
              <InputLabel>Role</InputLabel>
              <Select
                value={newUser.role}
                label="Role"
                onChange={(e) => setNewUser({ ...newUser, role: e.target.value as User['role'] })}
              >
                <MenuItem value="admin">Admin</MenuItem>
                <MenuItem value="operator">Bot Operator</MenuItem>
                <MenuItem value="client">Client</MenuItem>
              </Select>
            </FormControl>
          </Box>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setCreateUserDialog(false)}>Cancel</Button>
          <Button variant="contained" onClick={handleCreateUser} disabled={loading}>
            Add User
          </Button>
        </DialogActions>
      </Dialog>
      
      {/* Confirm Dialog */}
      <Dialog open={confirmDialog.open} onClose={() => setConfirmDialog({ open: false, title: '', message: '', action: '' })}>
        <DialogTitle>{confirmDialog.title}</DialogTitle>
        <DialogContent>
          <Typography>{confirmDialog.message}</Typography>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setConfirmDialog({ open: false, title: '', message: '', action: '' })}>Cancel</Button>
          <Button variant="contained" color="error" onClick={() => confirmAction(confirmDialog.action)}>
            Confirm
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
}