'use client';

import React, { useState, useEffect, useCallback } from 'react';
import {
  Box, Typography, Card, CardContent, Button, TextField, Tabs, Tab, Chip,
  Table, TableBody, TableCell, TableContainer, TableHead, TableRow,
  Paper, IconButton, Dialog, DialogTitle, DialogContent, DialogActions,
  Alert, Select, MenuItem, FormControl, InputLabel, Switch,
  FormControlLabel, Slider, Grid, Avatar, Divider, List, ListItem,
  ListItemText, ListItemIcon, LinearProgress, Tooltip, Badge, Accordion,
  AccordionSummary, AccordionDetails, SelectChangeEvent
} from '@mui/material';
import {
  AccountTree, Lan, Storage, Cloud, Security, Settings, Add, PlayArrow,
  Stop, Pause, Delete, Refresh, ExpandMore, Link, Language,
  AccountBalance, Speed, ShowChart, TrendingUp, Warning, CheckCircle,
  Error as ErrorIcon, CloudQueue, Dns, Router, SwapHoriz
} from '@mui/icons-material';
import { useTheme } from '../components/ThemeProvider';
import { ThemeToggle } from '../components/ThemeToggle';

// ============================================================================
// Types & Interfaces
// ============================================================================

interface Chain {
  id: string;
  name: string;
  type: 'evm' | 'solana' | 'aptos' | 'sui' | 'ton' | 'cosmos';
  chainId: number;
  rpcUrl: string;
  explorerUrl: string;
  status: 'active' | 'paused' | 'inactive' | 'upgrading';
  isTestnet: boolean;
  nativeCurrency: string;
  decimals: number;
  minGasPrice: number;
  maxGasPrice: number;
  blockTime: number;
  tps: number;
  tvl: number;
  addedAt: number;
}

interface Validator {
  id: string;
  address: string;
  name: string;
  endpoint: string;
  stake: number;
  status: 'active' | 'inactive';
  performance: number;
  chains: string[];
}

interface Bridge {
  id: string;
  name: string;
  sourceChain: string;
  destChain: string;
  status: 'active' | 'inactive';
  minAmount: number;
  maxAmount: number;
  fee: number;
  volume24h: number;
}

interface TokenDeployment {
  id: string;
  chainId: string;
  tokenAddress: string;
  name: string;
  symbol: string;
  decimals: number;
  totalSupply: number;
  deployer: string;
  deployedAt: number;
  status: 'deployed' | 'pending' | 'failed';
}

interface ChainMetrics {
  chainId: string;
  tps: number;
  avgGasUsed: number;
  avgBlockTime: number;
  uptime: number;
  totalTx: number;
  totalVolume: number;
  activeUsers: number;
}

// ============================================================================
// Constants
// ============================================================================

const CHAIN_TYPES = [
  { value: 'evm', label: 'EVM', icon: <AccountTree />, description: 'Ethereum, BSC, Polygon, etc.' },
  { value: 'solana', label: 'Solana', icon: <Speed />, description: 'Solana blockchain' },
  { value: 'aptos', label: 'Aptos', icon: <Cloud />, description: 'Aptos Move' },
  { value: 'sui', label: 'Sui', icon: <SwapHoriz />, description: 'Sui Move' },
  { value: 'ton', label: 'TON', icon: <CloudQueue />, description: 'Telegram Open Network' },
  { value: 'cosmos', label: 'Cosmos', icon: <Lan />, description: 'Cosmos SDK chains' },
];

const DEFAULT_EVM_CHAINS = [
  { name: 'Ethereum', chainId: 1, symbol: 'ETH', decimals: 18 },
  { name: 'BNB Smart Chain', chainId: 56, symbol: 'BNB', decimals: 18 },
  { name: 'Polygon', chainId: 137, symbol: 'MATIC', decimals: 18 },
  { name: 'Arbitrum One', chainId: 42161, symbol: 'ETH', decimals: 18 },
  { name: 'Optimism', chainId: 10, symbol: 'ETH', decimals: 18 },
  { name: 'Base', chainId: 8453, symbol: 'ETH', decimals: 18 },
  { name: 'Avalanche', chainId: 43114, symbol: 'AVAX', decimals: 18 },
  { name: 'Linea', chainId: 59144, symbol: 'ETH', decimals: 18 },
  { name: 'Scroll', chainId: 534352, symbol: 'ETH', decimals: 18 },
  { name: 'zkSync Era', chainId: 324, symbol: 'ETH', decimals: 18 },
];

// ============================================================================
// Component
// ============================================================================

export default function ChainAdminPanel() {
  // State
  const [activeTab, setActiveTab] = useState(0);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);
  
  // Data
  const [chains, setChains] = useState<Chain[]>([]);
  const [validators, setValidators] = useState<Validator[]>([]);
  const [bridges, setBridges] = useState<Bridge[]>([]);
  const [tokenDeployments, setTokenDeployments] = useState<TokenDeployment[]>([]);
  const [metrics, setMetrics] = useState<Record<string, ChainMetrics>>({});
  
  // Dialogs
  const [addChainDialog, setAddChainDialog] = useState(false);
  const [addValidatorDialog, setAddValidatorDialog] = useState(false);
  const [addBridgeDialog, setAddBridgeDialog] = useState(false);
  const [deployTokenDialog, setDeployTokenDialog] = useState(false);
  
  // Forms
  const [newChain, setNewChain] = useState({
    name: '',
    type: 'evm' as Chain['type'],
    chainId: 1,
    rpcUrl: '',
    explorerUrl: '',
    nativeCurrency: 'ETH',
    decimals: 18,
    isTestnet: false,
  });
  
  const [newValidator, setNewValidator] = useState({
    name: '',
    address: '',
    endpoint: '',
    stake: 10000,
  });
  
  const [newBridge, setNewBridge] = useState({
    name: '',
    sourceChain: '',
    destChain: '',
    minAmount: 100,
    maxAmount: 1000000,
    fee: 0.1,
  });
  
  // ============================================================================
  // Effects
  // ============================================================================
  
  useEffect(() => {
    loadData();
  }, []);
  
  // ============================================================================
  // Data Loading
  // ============================================================================
  
  const loadData = useCallback(async () => {
    setLoading(true);
    try {
      await new Promise(resolve => setTimeout(resolve, 500));
      
      // Mock chains
      setChains([
        {
          id: 'eth-mainnet',
          name: 'Ethereum',
          type: 'evm',
          chainId: 1,
          rpcUrl: 'https://eth.llamarpc.com',
          explorerUrl: 'https://etherscan.io',
          status: 'active',
          isTestnet: false,
          nativeCurrency: 'ETH',
          decimals: 18,
          minGasPrice: 1000000000,
          maxGasPrice: 100000000000,
          blockTime: 12,
          tps: 15,
          tvl: 50000000000,
          addedAt: Date.now() - 86400000 * 90,
        },
        {
          id: 'bsc-mainnet',
          name: 'BNB Smart Chain',
          type: 'evm',
          chainId: 56,
          rpcUrl: 'https://bsc-dataseed.binance.org',
          explorerUrl: 'https://bscscan.com',
          status: 'active',
          isTestnet: false,
          nativeCurrency: 'BNB',
          decimals: 18,
          minGasPrice: 5000000000,
          maxGasPrice: 100000000000,
          blockTime: 3,
          tps: 150,
          tvl: 8000000000,
          addedAt: Date.now() - 86400000 * 60,
        },
        {
          id: 'polygon-mainnet',
          name: 'Polygon',
          type: 'evm',
          chainId: 137,
          rpcUrl: 'https://polygon-rpc.com',
          explorerUrl: 'https://polygonscan.com',
          status: 'active',
          isTestnet: false,
          nativeCurrency: 'MATIC',
          decimals: 18,
          minGasPrice: 1000000000,
          maxGasPrice: 100000000000,
          blockTime: 2,
          tps: 350,
          tvl: 2000000000,
          addedAt: Date.now() - 86400000 * 45,
        },
        {
          id: 'arbitrum-mainnet',
          name: 'Arbitrum One',
          type: 'evm',
          chainId: 42161,
          rpcUrl: 'https://arb1.arbitrum.io/rpc',
          explorerUrl: 'https://arbiscan.io',
          status: 'active',
          isTestnet: false,
          nativeCurrency: 'ETH',
          decimals: 18,
          minGasPrice: 100000000,
          maxGasPrice: 10000000000,
          blockTime: 1,
          tps: 500,
          tvl: 3000000000,
          addedAt: Date.now() - 86400000 * 30,
        },
        {
          id: 'solana-mainnet',
          name: 'Solana',
          type: 'solana',
          chainId: 101,
          rpcUrl: 'https://api.mainnet-beta.solana.com',
          explorerUrl: 'https://solscan.io',
          status: 'active',
          isTestnet: false,
          nativeCurrency: 'SOL',
          decimals: 9,
          minGasPrice: 0,
          maxGasPrice: 0,
          blockTime: 0.4,
          tps: 6500,
          tvl: 1500000000,
          addedAt: Date.now() - 86400000 * 20,
        },
        {
          id: 'aptos-mainnet',
          name: 'Aptos',
          type: 'aptos',
          chainId: 1,
          rpcUrl: 'https://aptos-mainnet.nodereal.io/v1',
          explorerUrl: 'https://aptoscan.com',
          status: 'active',
          isTestnet: false,
          nativeCurrency: 'APT',
          decimals: 8,
          minGasPrice: 0,
          maxGasPrice: 0,
          blockTime: 1,
          tps: 2000,
          tvl: 500000000,
          addedAt: Date.now() - 86400000 * 10,
        },
      ]);
      
      // Mock validators
      setValidators([
        {
          id: 'val1',
          address: '0x1234...abcd',
          name: 'Primary Validator 1',
          endpoint: 'https://validator1.tigerswap.io',
          stake: 50000,
          status: 'active',
          performance: 99.9,
          chains: ['eth-mainnet', 'bsc-mainnet'],
        },
        {
          id: 'val2',
          address: '0x5678...efgh',
          name: 'Primary Validator 2',
          endpoint: 'https://validator2.tigerswap.io',
          stake: 45000,
          status: 'active',
          performance: 99.8,
          chains: ['eth-mainnet', 'polygon-mainnet'],
        },
        {
          id: 'val3',
          address: '0xabcd...1234',
          name: 'Backup Validator',
          endpoint: 'https://validator3.tigerswap.io',
          stake: 25000,
          status: 'inactive',
          performance: 98.5,
          chains: ['solana-mainnet'],
        },
      ]);
      
      // Mock bridges
      setBridges([
        {
          id: 'bridge1',
          name: 'Ethereum Bridge',
          sourceChain: 'eth-mainnet',
          destChain: 'polygon-mainnet',
          status: 'active',
          minAmount: 100,
          maxAmount: 1000000,
          fee: 0.05,
          volume24h: 5000000,
        },
        {
          id: 'bridge2',
          name: 'Solana Bridge',
          sourceChain: 'solana-mainnet',
          destChain: 'eth-mainnet',
          status: 'active',
          minAmount: 10,
          maxAmount: 500000,
          fee: 0.1,
          volume24h: 2500000,
        },
        {
          id: 'bridge3',
          name: 'Aptos Bridge',
          sourceChain: 'aptos-mainnet',
          destChain: 'eth-mainnet',
          status: 'active',
          minAmount: 50,
          maxAmount: 250000,
          fee: 0.08,
          volume24h: 1000000,
        },
      ]);
      
      // Mock token deployments
      setTokenDeployments([
        {
          id: 'token1',
          chainId: 'eth-mainnet',
          tokenAddress: '0x1234...abcd',
          name: 'Tiger Token',
          symbol: 'TIGER',
          decimals: 18,
          totalSupply: 1000000000,
          deployer: '0xadmin...xyz',
          deployedAt: Date.now() - 86400000 * 30,
          status: 'deployed',
        },
      ]);
      
      // Mock metrics
      setMetrics({
        'eth-mainnet': {
          chainId: 'eth-mainnet',
          tps: 15,
          avgGasUsed: 21000,
          avgBlockTime: 12,
          uptime: 99.99,
          totalTx: 2500000000,
          totalVolume: 1500000000000,
          activeUsers: 500000,
        },
        'bsc-mainnet': {
          chainId: 'bsc-mainnet',
          tps: 150,
          avgGasUsed: 5000,
          avgBlockTime: 3,
          uptime: 99.95,
          totalTx: 5000000000,
          totalVolume: 800000000000,
          activeUsers: 2000000,
        },
      });
      
      setSuccess('Chain data loaded successfully');
    } catch (err: any) {
      setError(err.message || 'Failed to load data');
    } finally {
      setLoading(false);
    }
  }, []);
  
  // ============================================================================
  // Actions
  // ============================================================================
  
  const handleAddChain = useCallback(async () => {
    if (!newChain.name || !newChain.rpcUrl) {
      setError('Please fill all required fields');
      return;
    }
    
    setLoading(true);
    try {
      await new Promise(resolve => setTimeout(resolve, 500));
      
      setChains([...chains, {
        id: `chain-${Date.now()}`,
        ...newChain,
        status: 'active',
        minGasPrice: newChain.type === 'evm' ? 1000000000 : 0,
        maxGasPrice: newChain.type === 'evm' ? 100000000000 : 0,
        blockTime: 3,
        tps: 100,
        tvl: 0,
        addedAt: Date.now(),
      }]);
      
      setSuccess(`Chain ${newChain.name} added successfully`);
      setAddChainDialog(false);
      setNewChain({
        name: '',
        type: 'evm',
        chainId: 1,
        rpcUrl: '',
        explorerUrl: '',
        nativeCurrency: 'ETH',
        decimals: 18,
        isTestnet: false,
      });
    } catch (err: any) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  }, [newChain, chains]);
  
  const handleAddValidator = useCallback(async () => {
    if (!newValidator.name || !newValidator.address) {
      setError('Please fill all required fields');
      return;
    }
    
    setLoading(true);
    try {
      await new Promise(resolve => setTimeout(resolve, 500));
      
      setValidators([...validators, {
        id: `val-${Date.now()}`,
        ...newValidator,
        status: 'active',
        performance: 100,
        chains: [],
      }]);
      
      setSuccess('Validator added successfully');
      setAddValidatorDialog(false);
      setNewValidator({ name: '', address: '', endpoint: '', stake: 10000 });
    } catch (err: any) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  }, [newValidator, validators]);
  
  const handleAddBridge = useCallback(async () => {
    if (!newBridge.name || !newBridge.sourceChain || !newBridge.destChain) {
      setError('Please fill all required fields');
      return;
    }
    
    setLoading(true);
    try {
      await new Promise(resolve => setTimeout(resolve, 500));
      
      setBridges([...bridges, {
        id: `bridge-${Date.now()}`,
        ...newBridge,
        status: 'active',
        volume24h: 0,
      }]);
      
      setSuccess('Bridge added successfully');
      setAddBridgeDialog(false);
      setNewBridge({ name: '', sourceChain: '', destChain: '', minAmount: 100, maxAmount: 1000000, fee: 0.1 });
    } catch (err: any) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  }, [newBridge, bridges]);
  
  const handleUpdateChainStatus = useCallback(async (chainId: string, status: Chain['status']) => {
    setLoading(true);
    try {
      await new Promise(resolve => setTimeout(resolve, 300));
      setChains(chains.map(c => c.id === chainId ? { ...c, status } : c));
      setSuccess('Chain status updated');
    } catch (err: any) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  }, [chains]);
  
  // ============================================================================
  // Helper Functions
  // ============================================================================
  
  const formatCurrency = (amount: number) => {
    if (amount >= 1e9) return `$${(amount / 1e9).toFixed(2)}B`;
    if (amount >= 1e6) return `$${(amount / 1e6).toFixed(2)}M`;
    if (amount >= 1e3) return `$${(amount / 1e3).toFixed(2)}K`;
    return `$${amount.toFixed(2)}`;
  };
  
  const formatDate = (timestamp: number) => {
    return new Date(timestamp).toLocaleDateString('en-US', {
      month: 'short',
      day: 'numeric',
      year: 'numeric',
    });
  };
  
  // ============================================================================
  // Render
  // ============================================================================
  
  return (
    <Box sx={{ p: 3 }}>
      {/* Header */}
      <Box sx={{ mb: 3, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <Box>
          <Typography variant="h5" fontWeight="bold">Multi-Chain Management</Typography>
          <Typography variant="body2" color="text.secondary">
            EVM & Non-EVM Blockchain Administration
          </Typography>
        </Box>
        <Box sx={{ display: 'flex', gap: 2 }}>
          <Button variant="outlined" startIcon={<Refresh />} onClick={loadData}>Refresh</Button>
          <Button variant="contained" startIcon={<Add />} onClick={() => setAddChainDialog(true)}>
            Add Chain
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
      
      {/* Stats */}
      <Grid container spacing={3} sx={{ mb: 3 }}>
        <Grid item xs={12} sm={6} md={3}>
          <Card>
            <CardContent>
              <Typography color="text.secondary" gutterBottom>Total Chains</Typography>
              <Typography variant="h4">{chains.length}</Typography>
              <Typography variant="caption" color="text.secondary">
                EVM: {chains.filter(c => c.type === 'evm').length} | Non-EVM: {chains.filter(c => c.type !== 'evm').length}
              </Typography>
            </CardContent>
          </Card>
        </Grid>
        <Grid item xs={12} sm={6} md={3}>
          <Card>
            <CardContent>
              <Typography color="text.secondary" gutterBottom>Active Validators</Typography>
              <Typography variant="h4">{validators.filter(v => v.status === 'active').length}</Typography>
              <Typography variant="caption" color="text.secondary">
                Total Staked: {formatCurrency(validators.reduce((a, v) => a + v.stake, 0))}
              </Typography>
            </CardContent>
          </Card>
        </Grid>
        <Grid item xs={12} sm={6} md={3}>
          <Card>
            <CardContent>
              <Typography color="text.secondary" gutterBottom>Active Bridges</Typography>
              <Typography variant="h4">{bridges.filter(b => b.status === 'active').length}</Typography>
              <Typography variant="caption" color="text.secondary">
                24h Volume: {formatCurrency(bridges.reduce((a, b) => a + b.volume24h, 0))}
              </Typography>
            </CardContent>
          </Card>
        </Grid>
        <Grid item xs={12} sm={6} md={3}>
          <Card>
            <CardContent>
              <Typography color="text.secondary" gutterBottom>Total TVL</Typography>
              <Typography variant="h4">{formatCurrency(chains.reduce((a, c) => a + c.tvl, 0))}</Typography>
              <Typography variant="caption" color="text.secondary">
                Across all chains
              </Typography>
            </CardContent>
          </Card>
        </Grid>
      </Grid>
      
      {/* Tabs */}
      <Tabs value={activeTab} onChange={(_, v) => setActiveTab(v)} sx={{ mb: 3 }}>
        <Tab label="Chains" icon={<AccountTree />} iconPosition="start" />
        <Tab label="Validators" icon={<Dns />} iconPosition="start" />
        <Tab label="Bridges" icon={<SwapHoriz />} iconPosition="start" />
        <Tab label="Token Deployments" icon={<Storage />} iconPosition="start" />
        <Tab label="Metrics" icon={<ShowChart />} iconPosition="start" />
      </Tabs>
      
      {/* Chains Tab */}
      {activeTab === 0 && (
        <Grid container spacing={3}>
          {chains.map(chain => (
            <Grid item xs={12} md={6} key={chain.id}>
              <Card>
                <CardContent>
                  <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', mb: 2 }}>
                    <Box>
                      <Typography variant="h6">{chain.name}</Typography>
                      <Typography variant="body2" color="text.secondary">
                        ID: {chain.chainId} | Type: {chain.type.toUpperCase()}
                      </Typography>
                    </Box>
                    <Chip 
                      label={chain.status}
                      size="small"
                      color={chain.status === 'active' ? 'success' : chain.status === 'paused' ? 'warning' : 'default'}
                    />
                  </Box>
                  
                  <Divider sx={{ my: 2 }} />
                  
                  <Grid container spacing={2}>
                    <Grid item xs={6}>
                      <Typography variant="caption" color="text.secondary">TPS</Typography>
                      <Typography variant="h6">{chain.tps.toLocaleString()}</Typography>
                    </Grid>
                    <Grid item xs={6}>
                      <Typography variant="caption" color="text.secondary">TVL</Typography>
                      <Typography variant="h6">{formatCurrency(chain.tvl)}</Typography>
                    </Grid>
                    <Grid item xs={6}>
                      <Typography variant="caption" color="text.secondary">Block Time</Typography>
                      <Typography variant="body1">{chain.blockTime}s</Typography>
                    </Grid>
                    <Grid item xs={6}>
                      <Typography variant="caption" color="text.secondary">Min Gas</Typography>
                      <Typography variant="body1">{chain.minGasPrice > 0 ? `${(chain.minGasPrice / 1e9).toFixed(2)} gwei` : 'N/A'}</Typography>
                    </Grid>
                  </Grid>
                  
                  <Box sx={{ display: 'flex', gap: 1, mt: 2 }}>
                    {chain.status === 'active' ? (
                      <Button size="small" variant="outlined" color="warning" onClick={() => handleUpdateChainStatus(chain.id, 'paused')}>
                        Pause
                      </Button>
                    ) : (
                      <Button size="small" variant="outlined" color="success" onClick={() => handleUpdateChainStatus(chain.id, 'active')}>
                        Activate
                      </Button>
                    )}
                    <Button size="small" variant="outlined" onClick={() => window.open(chain.explorerUrl, '_blank')}>
                      Explorer
                    </Button>
                  </Box>
                </CardContent>
              </Card>
            </Grid>
          ))}
        </Grid>
      )}
      
      {/* Validators Tab */}
      {activeTab === 1 && (
        <Box>
          <Box sx={{ mb: 2, display: 'flex', justifyContent: 'flex-end' }}>
            <Button variant="contained" startIcon={<Add />} onClick={() => setAddValidatorDialog(true)}>
              Add Validator
            </Button>
          </Box>
          <TableContainer component={Paper}>
            <Table>
              <TableHead>
                <TableRow>
                  <TableCell>Name</TableCell>
                  <TableCell>Address</TableCell>
                  <TableCell>Endpoint</TableCell>
                  <TableCell align="right">Stake</TableCell>
                  <TableCell>Status</TableCell>
                  <TableCell align="right">Performance</TableCell>
                  <TableCell>Chains</TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {validators.map(validator => (
                  <TableRow key={validator.id}>
                    <TableCell>{validator.name}</TableCell>
                    <TableCell>{validator.address}</TableCell>
                    <TableCell>
                      <Tooltip title={validator.endpoint}>
                        <Typography variant="body2" sx={{ maxWidth: 200, overflow: 'hidden', textOverflow: 'ellipsis' }}>
                          {validator.endpoint}
                        </Typography>
                      </Tooltip>
                    </TableCell>
                    <TableCell align="right">{formatCurrency(validator.stake)}</TableCell>
                    <TableCell>
                      <Chip 
                        label={validator.status}
                        size="small"
                        color={validator.status === 'active' ? 'success' : 'default'}
                      />
                    </TableCell>
                    <TableCell align="right">{validator.performance.toFixed(2)}%</TableCell>
                    <TableCell>
                      {validator.chains.map(c => (
                        <Chip key={c} label={c} size="small" sx={{ mr: 0.5 }} />
                      ))}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </TableContainer>
        </Box>
      )}
      
      {/* Bridges Tab */}
      {activeTab === 2 && (
        <Box>
          <Box sx={{ mb: 2, display: 'flex', justifyContent: 'flex-end' }}>
            <Button variant="contained" startIcon={<Add />} onClick={() => setAddBridgeDialog(true)}>
              Add Bridge
            </Button>
          </Box>
          <TableContainer component={Paper}>
            <Table>
              <TableHead>
                <TableRow>
                  <TableCell>Name</TableCell>
                  <TableCell>Source</TableCell>
                  <TableCell>Destination</TableCell>
                  <TableCell>Status</TableCell>
                  <TableCell align="right">Min/Max</TableCell>
                  <TableCell align="right">Fee</TableCell>
                  <TableCell align="right">24h Volume</TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {bridges.map(bridge => (
                  <TableRow key={bridge.id}>
                    <TableCell>{bridge.name}</TableCell>
                    <TableCell>{bridge.sourceChain}</TableCell>
                    <TableCell>{bridge.destChain}</TableCell>
                    <TableCell>
                      <Chip 
                        label={bridge.status}
                        size="small"
                        color={bridge.status === 'active' ? 'success' : 'default'}
                      />
                    </TableCell>
                    <TableCell align="right">{formatCurrency(bridge.minAmount)} / {formatCurrency(bridge.maxAmount)}</TableCell>
                    <TableCell align="right">{bridge.fee}%</TableCell>
                    <TableCell align="right">{formatCurrency(bridge.volume24h)}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </TableContainer>
        </Box>
      )}
      
      {/* Token Deployments Tab */}
      {activeTab === 3 && (
        <TableContainer component={Paper}>
          <Table>
            <TableHead>
              <TableRow>
                <TableCell>Token</TableCell>
                <TableCell>Chain</TableCell>
                <TableCell>Address</TableCell>
                <TableCell align="right">Supply</TableCell>
                <TableCell>Deployer</TableCell>
                <TableCell>Deployed</TableCell>
                <TableCell>Status</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {tokenDeployments.map(token => (
                <TableRow key={token.id}>
                  <TableCell>{token.name} ({token.symbol})</TableCell>
                  <TableCell>{token.chainId}</TableCell>
                  <TableCell>{token.tokenAddress}</TableCell>
                  <TableCell align="right">{token.totalSupply.toLocaleString()}</TableCell>
                  <TableCell>{token.deployer}</TableCell>
                  <TableCell>{formatDate(token.deployedAt)}</TableCell>
                  <TableCell>
                    <Chip 
                      label={token.status}
                      size="small"
                      color={token.status === 'deployed' ? 'success' : token.status === 'pending' ? 'warning' : 'error'}
                    />
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </TableContainer>
      )}
      
      {/* Metrics Tab */}
      {activeTab === 4 && (
        <Grid container spacing={3}>
          {Object.entries(metrics).map(([chainId, m]) => (
            <Grid item xs={12} md={6} key={chainId}>
              <Card>
                <CardContent>
                  <Typography variant="h6" gutterBottom>{chainId}</Typography>
                  <Grid container spacing={2}>
                    <Grid item xs={6}>
                      <Typography variant="caption" color="text.secondary">TPS</Typography>
                      <Typography variant="h5">{m.tps.toLocaleString()}</Typography>
                    </Grid>
                    <Grid item xs={6}>
                      <Typography variant="caption" color="text.secondary">Uptime</Typography>
                      <Typography variant="h5">{m.uptime.toFixed(2)}%</Typography>
                    </Grid>
                    <Grid item xs={6}>
                      <Typography variant="caption" color="text.secondary">Total Transactions</Typography>
                      <Typography variant="body1">{m.totalTx.toLocaleString()}</Typography>
                    </Grid>
                    <Grid item xs={6}>
                      <Typography variant="caption" color="text.secondary">Active Users</Typography>
                      <Typography variant="body1">{m.activeUsers.toLocaleString()}</Typography>
                    </Grid>
                    <Grid item xs={6}>
                      <Typography variant="caption" color="text.secondary">Avg Block Time</Typography>
                      <Typography variant="body1">{m.avgBlockTime}s</Typography>
                    </Grid>
                    <Grid item xs={6}>
                      <Typography variant="caption" color="text.secondary">Total Volume</Typography>
                      <Typography variant="body1">{formatCurrency(m.totalVolume)}</Typography>
                    </Grid>
                  </Grid>
                </CardContent>
              </Card>
            </Grid>
          ))}
        </Grid>
      )}
      
      {/* Add Chain Dialog */}
      <Dialog open={addChainDialog} onClose={() => setAddChainDialog(false)} maxWidth="md" fullWidth>
        <DialogTitle>Add New Chain</DialogTitle>
        <DialogContent>
          <Box sx={{ pt: 2, display: 'flex', flexDirection: 'column', gap: 2 }}>
            <TextField
              fullWidth
              label="Chain Name"
              value={newChain.name}
              onChange={(e) => setNewChain({ ...newChain, name: e.target.value })}
            />
            <FormControl fullWidth>
              <InputLabel>Chain Type</InputLabel>
              <Select
                value={newChain.type}
                label="Chain Type"
                onChange={(e) => setNewChain({ ...newChain, type: e.target.value as Chain['type'] })}
              >
                {CHAIN_TYPES.map(ct => (
                  <MenuItem key={ct.value} value={ct.value}>
                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                      {ct.icon} {ct.label}
                    </Box>
                  </MenuItem>
                ))}
              </Select>
            </FormControl>
            <TextField
              fullWidth
              label="Chain ID"
              type="number"
              value={newChain.chainId}
              onChange={(e) => setNewChain({ ...newChain, chainId: parseInt(e.target.value) })}
            />
            <TextField
              fullWidth
              label="RPC URL"
              value={newChain.rpcUrl}
              onChange={(e) => setNewChain({ ...newChain, rpcUrl: e.target.value })}
            />
            <TextField
              fullWidth
              label="Explorer URL"
              value={newChain.explorerUrl}
              onChange={(e) => setNewChain({ ...newChain, explorerUrl: e.target.value })}
            />
            <FormControlLabel
              control={
                <Switch
                  checked={newChain.isTestnet}
                  onChange={(e) => setNewChain({ ...newChain, isTestnet: e.target.checked })}
                />
              }
              label="Testnet"
            />
          </Box>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setAddChainDialog(false)}>Cancel</Button>
          <Button variant="contained" onClick={handleAddChain} disabled={loading}>
            Add Chain
          </Button>
        </DialogActions>
      </Dialog>
      
      {/* Add Validator Dialog */}
      <Dialog open={addValidatorDialog} onClose={() => setAddValidatorDialog(false)} maxWidth="sm" fullWidth>
        <DialogTitle>Add Validator</DialogTitle>
        <DialogContent>
          <Box sx={{ pt: 2, display: 'flex', flexDirection: 'column', gap: 2 }}>
            <TextField
              fullWidth
              label="Validator Name"
              value={newValidator.name}
              onChange={(e) => setNewValidator({ ...newValidator, name: e.target.value })}
            />
            <TextField
              fullWidth
              label="Validator Address"
              value={newValidator.address}
              onChange={(e) => setNewValidator({ ...newValidator, address: e.target.value })}
            />
            <TextField
              fullWidth
              label="Endpoint URL"
              value={newValidator.endpoint}
              onChange={(e) => setNewValidator({ ...newValidator, endpoint: e.target.value })}
            />
            <TextField
              fullWidth
              label="Stake Amount"
              type="number"
              value={newValidator.stake}
              onChange={(e) => setNewValidator({ ...newValidator, stake: parseInt(e.target.value) })}
            />
          </Box>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setAddValidatorDialog(false)}>Cancel</Button>
          <Button variant="contained" onClick={handleAddValidator} disabled={loading}>
            Add Validator
          </Button>
        </DialogActions>
      </Dialog>
      
      {/* Add Bridge Dialog */}
      <Dialog open={addBridgeDialog} onClose={() => setAddBridgeDialog(false)} maxWidth="sm" fullWidth>
        <DialogTitle>Add Bridge</DialogTitle>
        <DialogContent>
          <Box sx={{ pt: 2, display: 'flex', flexDirection: 'column', gap: 2 }}>
            <TextField
              fullWidth
              label="Bridge Name"
              value={newBridge.name}
              onChange={(e) => setNewBridge({ ...newBridge, name: e.target.value })}
            />
            <FormControl fullWidth>
              <InputLabel>Source Chain</InputLabel>
              <Select
                value={newBridge.sourceChain}
                label="Source Chain"
                onChange={(e) => setNewBridge({ ...newBridge, sourceChain: e.target.value })}
              >
                {chains.map(c => (
                  <MenuItem key={c.id} value={c.id}>{c.name}</MenuItem>
                ))}
              </Select>
            </FormControl>
            <FormControl fullWidth>
              <InputLabel>Destination Chain</InputLabel>
              <Select
                value={newBridge.destChain}
                label="Destination Chain"
                onChange={(e) => setNewBridge({ ...newBridge, destChain: e.target.value })}
              >
                {chains.map(c => (
                  <MenuItem key={c.id} value={c.id}>{c.name}</MenuItem>
                ))}
              </Select>
            </FormControl>
            <TextField
              fullWidth
              label="Min Amount"
              type="number"
              value={newBridge.minAmount}
              onChange={(e) => setNewBridge({ ...newBridge, minAmount: parseInt(e.target.value) })}
            />
            <TextField
              fullWidth
              label="Max Amount"
              type="number"
              value={newBridge.maxAmount}
              onChange={(e) => setNewBridge({ ...newBridge, maxAmount: parseInt(e.target.value) })}
            />
            <TextField
              fullWidth
              label="Fee (%)"
              type="number"
              value={newBridge.fee}
              onChange={(e) => setNewBridge({ ...newBridge, fee: parseFloat(e.target.value) })}
            />
          </Box>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setAddBridgeDialog(false)}>Cancel</Button>
          <Button variant="contained" onClick={handleAddBridge} disabled={loading}>
            Add Bridge
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
}