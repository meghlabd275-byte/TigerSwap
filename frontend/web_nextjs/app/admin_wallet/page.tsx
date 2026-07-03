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
  AccountBalanceWallet, Add, Edit, Delete, Refresh, Visibility,
  VisibilityOff, ContentCopy, Send, MoveToInbox, SwapHoriz, Key,
  Security, Warning, CheckCircle, Error as ErrorIcon, Lock,
  AccountTree, Hexagon
} from '@mui/icons-material';
import { useTheme } from '../components/ThemeProvider';
import { ThemeToggle } from '../components/ThemeToggle';

// ============================================================================
// Types & Interfaces
// ============================================================================

interface Wallet {
  id: string;
  name: string;
  type: 'hot' | 'cold' | 'multi_sig' | 'treasury' | 'operations';
  address: string;
  chainId: number;
  chainName: string;
  balance: number;
  balanceUSD: number;
  tokenCount: number;
  status: 'active' | 'frozen' | 'compromised';
  signers?: string[];
  threshold?: number;
  createdAt: number;
  lastActivity: number;
}

interface Transaction {
  id: string;
  walletId: string;
  type: 'send' | 'receive' | 'swap' | 'approve';
  token: string;
  amount: number;
  amountUSD: number;
  status: 'pending' | 'confirmed' | 'failed';
  from: string;
  to: string;
  txHash: string;
  timestamp: number;
  confirmations: number;
}

interface WalletStats {
  totalWallets: number;
  totalBalance: number;
  totalValue: number;
  hotWallets: number;
  coldWallets: number;
  multiSigWallets: number;
  pendingTransactions: number;
}

// ============================================================================
// Component
// ============================================================================

export default function AdminWalletPage() {
  // State
  const [activeTab, setActiveTab] = useState(0);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);
  
  // Data
  const [wallets, setWallets] = useState<Wallet[]>([]);
  const [transactions, setTransactions] = useState<Transaction[]>([]);
  const [stats, setStats] = useState<WalletStats | null>(null);
  
  // Dialogs
  const [createDialogOpen, setCreateDialogOpen] = useState(false);
  const [detailDialogOpen, setDetailDialogOpen] = useState(false);
  const [selectedWallet, setSelectedWallet] = useState<Wallet | null>(null);
  
  // Form
  const [formData, setFormData] = useState({
    name: '',
    type: 'hot' as Wallet['type'],
    chainId: 1,
    signers: [''],
    threshold: 2,
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
      
      // Mock wallets
      setWallets([
        {
          id: 'w1',
          name: 'Operations Hot Wallet',
          type: 'hot',
          address: '0x1234abcd1234abcd1234abcd1234abcd1234abcd',
          chainId: 1,
          chainName: 'Ethereum',
          balance: 50.5,
          balanceUSD: 175000,
          tokenCount: 12,
          status: 'active',
          createdAt: Date.now() - 86400000 * 90,
          lastActivity: Date.now() - 3600000,
        },
        {
          id: 'w2',
          name: 'Treasury Cold Wallet',
          type: 'cold',
          address: '0xabcd1234abcd1234abcd1234abcd1234abcd1234',
          chainId: 1,
          chainName: 'Ethereum',
          balance: 2500,
          balanceUSD: 8750000,
          tokenCount: 25,
          status: 'active',
          createdAt: Date.now() - 86400000 * 180,
          lastActivity: Date.now() - 86400000 * 7,
        },
        {
          id: 'w3',
          name: 'Multi-Sig Treasury',
          type: 'multi_sig',
          address: '0x5678567856785678567856785678567856785678',
          chainId: 1,
          chainName: 'Ethereum',
          balance: 500,
          balanceUSD: 1750000,
          tokenCount: 8,
          status: 'active',
          signers: ['0xsig1...', '0xsig2...', '0xsig3...'],
          threshold: 2,
          createdAt: Date.now() - 86400000 * 365,
          lastActivity: Date.now() - 86400000 * 2,
        },
        {
          id: 'w4',
          name: 'Fee Collector',
          type: 'operations',
          address: '0x9999999999999999999999999999999999999999',
          chainId: 56,
          chainName: 'BNB Chain',
          balance: 10,
          balanceUSD: 3000,
          tokenCount: 5,
          status: 'active',
          createdAt: Date.now() - 86400000 * 30,
          lastActivity: Date.now() - 7200000,
        },
      ]);
      
      // Mock transactions
      setTransactions([
        {
          id: 'tx1',
          walletId: 'w1',
          type: 'send',
          token: 'USDC',
          amount: 50000,
          amountUSD: 50000,
          status: 'confirmed',
          from: '0x1234...abcd',
          to: '0x5678...efgh',
          txHash: '0xabc123...',
          timestamp: Date.now() - 60000,
          confirmations: 12,
        },
        {
          id: 'tx2',
          walletId: 'w2',
          type: 'receive',
          token: 'ETH',
          amount: 100,
          amountUSD: 350000,
          status: 'confirmed',
          from: '0x1111...2222',
          to: '0xabcd...1234',
          txHash: '0xdef456...',
          timestamp: Date.now() - 3600000,
          confirmations: 15,
        },
        {
          id: 'tx3',
          walletId: 'w3',
          type: 'swap',
          token: 'ETH→USDC',
          amount: 50,
          amountUSD: 175000,
          status: 'confirmed',
          from: '0xabcd...1234',
          to: '0x5678...efgh',
          txHash: '0xghi789...',
          timestamp: Date.now() - 86400000,
          confirmations: 18,
        },
        {
          id: 'tx4',
          walletId: 'w1',
          type: 'send',
          token: 'USDT',
          amount: 25000,
          amountUSD: 25000,
          status: 'pending',
          from: '0x1234...abcd',
          to: '0x9999...ffff',
          txHash: '0xjkl012...',
          timestamp: Date.now() - 300000,
          confirmations: 2,
        },
      ]);
      
      // Mock stats
      setStats({
        totalWallets: 4,
        totalBalance: 3060.5,
        totalValue: 10677000,
        hotWallets: 1,
        coldWallets: 1,
        multiSigWallets: 1,
        pendingTransactions: 1,
      });
      
      setSuccess('Wallet data loaded successfully');
    } catch (err: any) {
      setError(err.message || 'Failed to load data');
    } finally {
      setLoading(false);
    }
  }, []);
  
  // ============================================================================
  // Actions
  // ============================================================================
  
  const handleCreateWallet = useCallback(async () => {
    if (!formData.name) {
      setError('Please enter wallet name');
      return;
    }
    
    setLoading(true);
    try {
      await new Promise(resolve => setTimeout(resolve, 500));
      
      const newWallet: Wallet = {
        id: `w${Date.now()}`,
        name: formData.name,
        type: formData.type,
        address: `0x${Math.random().toString(16).slice(2)}${Math.random().toString(16).slice(2)}`.slice(0, 42),
        chainId: formData.chainId,
        chainName: formData.chainId === 1 ? 'Ethereum' : formData.chainId === 56 ? 'BNB Chain' : 'Polygon',
        balance: 0,
        balanceUSD: 0,
        tokenCount: 0,
        status: 'active',
        signers: formData.type === 'multi_sig' ? formData.signers : undefined,
        threshold: formData.type === 'multi_sig' ? formData.threshold : undefined,
        createdAt: Date.now(),
        lastActivity: Date.now(),
      };
      
      setWallets([...wallets, newWallet]);
      setSuccess(`Wallet "${formData.name}" created successfully`);
      setCreateDialogOpen(false);
      setFormData({ name: '', type: 'hot', chainId: 1, signers: [''], threshold: 2 });
    } catch (err: any) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  }, [formData, wallets]);
  
  const handleDeleteWallet = useCallback(async (walletId: string) => {
    setLoading(true);
    try {
      await new Promise(resolve => setTimeout(resolve, 300));
      setWallets(wallets.filter(w => w.id !== walletId));
      setSuccess('Wallet deleted successfully');
    } catch (err: any) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  }, [wallets]);
  
  const handleFreezeWallet = useCallback(async (walletId: string) => {
    setWallets(wallets.map(w => 
      w.id === walletId ? { ...w, status: w.status === 'active' ? 'frozen' as const : 'active' as const } : w
    ));
  }, [wallets]);
  
  const handleViewWallet = useCallback((wallet: Wallet) => {
    setSelectedWallet(wallet);
    setDetailDialogOpen(true);
  }, []);
  
  const copyAddress = useCallback((address: string) => {
    navigator.clipboard.writeText(address);
    setSuccess('Address copied to clipboard');
  }, []);
  
  // ============================================================================
  // Helper Functions
  // ============================================================================
  
  const formatCurrency = (amount: number) => {
    if (amount >= 1e6) return `$${(amount / 1e6).toFixed(2)}M`;
    if (amount >= 1e3) return `$${(amount / 1e3).toFixed(2)}K`;
    return `$${amount.toFixed(2)}`;
  };
  
  const formatDate = (timestamp: number) => {
    return new Date(timestamp).toLocaleDateString('en-US', {
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    });
  };
  
  const getWalletTypeIcon = (type: Wallet['type']) => {
    switch (type) {
      case 'hot': return <Warning color="error" />;
      case 'cold': return <Lock color="primary" />;
      case 'multi_sig': return <AccountTree color="secondary" />;
      case 'treasury': return <Hexagon color="warning" />;
      case 'operations': return <SwapHoriz />;
    }
  };
  
  // ============================================================================
  // Render
  // ============================================================================
  
  return (
    <Box sx={{ p: 3 }}>
      {/* Header */}
      <Box sx={{ mb: 3, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <Box>
          <Typography variant="h5" fontWeight="bold">Wallet Management</Typography>
          <Typography variant="body2" color="text.secondary">
            Manage protocol wallets, multi-sig, and treasury
          </Typography>
        </Box>
        <Box sx={{ display: 'flex', gap: 2 }}>
          <Button variant="outlined" startIcon={<Refresh />} onClick={loadData}>Refresh</Button>
          <Button variant="contained" startIcon={<Add />} onClick={() => setCreateDialogOpen(true)}>
            Create Wallet
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
      
      {loading && <LinearProgress sx={{ mb: 2 }} />}
      
      {/* Stats */}
      {stats && (
        <Grid container spacing={3} sx={{ mb: 3 }}>
          <Grid item xs={12} sm={6} md={3}>
            <Card>
              <CardContent>
                <Typography color="text.secondary" gutterBottom>Total Wallets</Typography>
                <Typography variant="h4">{stats.totalWallets}</Typography>
              </CardContent>
            </Card>
          </Grid>
          <Grid item xs={12} sm={6} md={3}>
            <Card>
              <CardContent>
                <Typography color="text.secondary" gutterBottom>Total Balance</Typography>
                <Typography variant="h4">{stats.totalBalance.toFixed(2)} ETH</Typography>
              </CardContent>
            </Card>
          </Grid>
          <Grid item xs={12} sm={6} md={3}>
            <Card>
              <CardContent>
                <Typography color="text.secondary" gutterBottom>Total Value</Typography>
                <Typography variant="h4">{formatCurrency(stats.totalValue)}</Typography>
              </CardContent>
            </Card>
          </Grid>
          <Grid item xs={12} sm={6} md={3}>
            <Card>
              <CardContent>
                <Typography color="text.secondary" gutterBottom>Pending Tx</Typography>
                <Typography variant="h4" color="warning.main">{stats.pendingTransactions}</Typography>
              </CardContent>
            </Card>
          </Grid>
        </Grid>
      )}
      
      {/* Tabs */}
      <Tabs value={activeTab} onChange={(_, v) => setActiveTab(v)} sx={{ mb: 3 }}>
        <Tab label="Wallets" icon={<AccountBalanceWallet />} iconPosition="start" />
        <Tab label="Transactions" icon={<SwapHoriz />} iconPosition="start" />
        <Tab label="Multi-Sig" icon={<AccountTree />} iconPosition="start" />
        <Tab label="Security" icon={<Security />} iconPosition="start" />
      </Tabs>
      
      {/* Wallets Tab */}
      {activeTab === 0 && (
        <Grid container spacing={3}>
          {wallets.map(wallet => (
            <Grid item xs={12} md={6} key={wallet.id}>
              <Card>
                <CardContent>
                  <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', mb: 2 }}>
                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
                      {getWalletTypeIcon(wallet.type)}
                      <Box>
                        <Typography variant="h6">{wallet.name}</Typography>
                        <Typography variant="caption" color="text.secondary">
                          {wallet.type.toUpperCase()} | {wallet.chainName}
                        </Typography>
                      </Box>
                    </Box>
                    <Chip 
                      label={wallet.status.toUpperCase()}
                      size="small"
                      color={wallet.status === 'active' ? 'success' : 'error'}
                    />
                  </Box>
                  
                  <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 2, p: 1, bgcolor: 'grey.100', borderRadius: 1 }}>
                    <Typography variant="body2" sx={{ fontFamily: 'monospace', flex: 1 }}>
                      {wallet.address.slice(0, 10)}...{wallet.address.slice(-8)}
                    </Typography>
                    <IconButton size="small" onClick={() => copyAddress(wallet.address)}>
                      <ContentCopy fontSize="small" />
                    </IconButton>
                  </Box>
                  
                  <Grid container spacing={2}>
                    <Grid item xs={6}>
                      <Typography variant="caption" color="text.secondary">Balance</Typography>
                      <Typography variant="body1" fontWeight="bold">{wallet.balance.toFixed(4)} ETH</Typography>
                    </Grid>
                    <Grid item xs={6}>
                      <Typography variant="caption" color="text.secondary">Value</Typography>
                      <Typography variant="body1" fontWeight="bold">{formatCurrency(wallet.balanceUSD)}</Typography>
                    </Grid>
                    <Grid item xs={6}>
                      <Typography variant="caption" color="text.secondary">Tokens</Typography>
                      <Typography variant="body1">{wallet.tokenCount}</Typography>
                    </Grid>
                    <Grid item xs={6}>
                      <Typography variant="caption" color="text.secondary">Last Activity</Typography>
                      <Typography variant="body1">{formatDate(wallet.lastActivity)}</Typography>
                    </Grid>
                  </Grid>
                  
                  <Box sx={{ display: 'flex', gap: 1, mt: 2 }}>
                    <Button size="small" startIcon={<Visibility />} onClick={() => handleViewWallet(wallet)}>
                      Details
                    </Button>
                    <Button 
                      size="small" 
                      color={wallet.status === 'active' ? 'warning' : 'success'}
                      onClick={() => handleFreezeWallet(wallet.id)}
                    >
                      {wallet.status === 'active' ? 'Freeze' : 'Unfreeze'}
                    </Button>
                    <Button size="small" color="error" startIcon={<Delete />} onClick={() => handleDeleteWallet(wallet.id)}>
                      Delete
                    </Button>
                  </Box>
                </CardContent>
              </Card>
            </Grid>
          ))}
        </Grid>
      )}
      
      {/* Transactions Tab */}
      {activeTab === 1 && (
        <TableContainer component={Paper}>
          <Table>
            <TableHead>
              <TableRow>
                <TableCell>Type</TableCell>
                <TableCell>Wallet</TableCell>
                <TableCell>Token</TableCell>
                <TableCell align="right">Amount</TableCell>
                <TableCell>From/To</TableCell>
                <TableCell>Tx Hash</TableCell>
                <TableCell>Time</TableCell>
                <TableCell>Status</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {transactions.map(tx => {
                const wallet = wallets.find(w => w.id === tx.walletId);
                return (
                  <TableRow key={tx.id}>
                    <TableCell>
                      <Chip 
                        label={tx.type.toUpperCase()} 
                        size="small"
                        color={tx.type === 'send' ? 'error' : tx.type === 'receive' ? 'success' : 'primary'}
                        icon={tx.type === 'send' ? <Send /> : tx.type === 'receive' ? <MoveToInbox /> : <SwapHoriz />}
                      />
                    </TableCell>
                    <TableCell>{wallet?.name}</TableCell>
                    <TableCell>{tx.token}</TableCell>
                    <TableCell align="right" fontWeight="bold">{formatCurrency(tx.amountUSD)}</TableCell>
                    <TableCell sx={{ fontFamily: 'monospace', fontSize: '0.75rem' }}>
                      {tx.type === 'send' ? `→ ${tx.to.slice(0, 8)}...` : `← ${tx.from.slice(0, 8)}...`}
                    </TableCell>
                    <TableCell sx={{ fontFamily: 'monospace', fontSize: '0.75rem' }}>
                      {tx.txHash.slice(0, 10)}...
                    </TableCell>
                    <TableCell>{formatDate(tx.timestamp)}</TableCell>
                    <TableCell>
                      <Chip 
                        label={tx.status.toUpperCase()} 
                        size="small"
                        color={tx.status === 'confirmed' ? 'success' : tx.status === 'pending' ? 'warning' : 'error'}
                      />
                    </TableCell>
                  </TableRow>
                );
              })}
            </TableBody>
          </Table>
        </TableContainer>
      )}
      
      {/* Multi-Sig Tab */}
      {activeTab === 2 && (
        <Grid container spacing={3}>
          {wallets.filter(w => w.type === 'multi_sig').map(wallet => (
            <Grid item xs={12} md={6} key={wallet.id}>
              <Card>
                <CardContent>
                  <Typography variant="h6" gutterBottom>{wallet.name}</Typography>
                  <Divider sx={{ my: 2 }} />
                  <Typography variant="subtitle2" gutterBottom>Signers ({wallet.threshold}/{wallet.signers?.length})</Typography>
                  {wallet.signers?.map((signer, i) => (
                    <Box key={i} sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 1 }}>
                      <Chip label={`Signer ${i + 1}`} size="small" />
                      <Typography variant="body2" sx={{ fontFamily: 'monospace' }}>{signer}</Typography>
                    </Box>
                  ))}
                  <Divider sx={{ my: 2 }} />
                  <Grid container spacing={2}>
                    <Grid item xs={6}>
                      <Typography variant="caption" color="text.secondary">Balance</Typography>
                      <Typography variant="h6">{wallet.balance.toFixed(2)} ETH</Typography>
                    </Grid>
                    <Grid item xs={6}>
                      <Typography variant="caption" color="text.secondary">Value</Typography>
                      <Typography variant="h6">{formatCurrency(wallet.balanceUSD)}</Typography>
                    </Grid>
                  </Grid>
                </CardContent>
              </Card>
            </Grid>
          ))}
        </Grid>
      )}
      
      {/* Security Tab */}
      {activeTab === 3 && (
        <Grid container spacing={3}>
          <Grid item xs={12} md={6}>
            <Card>
              <CardContent>
                <Typography variant="h6" gutterBottom>Security Status</Typography>
                <List>
                  <ListItem>
                    <ListItemIcon><CheckCircle color="success" /></ListItemIcon>
                    <ListItemText primary="Multi-Sig Enabled" secondary="All treasury wallets require multiple signatures" />
                  </ListItem>
                  <ListItem>
                    <ListItemIcon><CheckCircle color="success" /></ListItemIcon>
                    <ListItemText primary="Cold Storage" secondary="95% of funds in cold wallets" />
                  </ListItem>
                  <ListItem>
                    <ListItemIcon><Warning color="warning" /></ListItemIcon>
                    <ListItemText primary="Hot Wallet Monitoring" secondary="Real-time monitoring active" />
                  </ListItem>
                  <ListItem>
                    <ListItemIcon><CheckCircle color="success" /></ListItemIcon>
                    <ListItemText primary="Hardware Security" secondary="All signers use hardware wallets" />
                  </ListItem>
                </List>
              </CardContent>
            </Card>
          </Grid>
          <Grid item xs={12} md={6}>
            <Card>
              <CardContent>
                <Typography variant="h6" gutterBottom>Access Keys</Typography>
                <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
                  Manage API keys and access credentials
                </Typography>
                <Button variant="outlined" startIcon={<Key />} fullWidth>
                  Generate New API Key
                </Button>
              </CardContent>
            </Card>
          </Grid>
        </Grid>
      )}
      
      {/* Create Wallet Dialog */}
      <Dialog open={createDialogOpen} onClose={() => setCreateDialogOpen(false)} maxWidth="sm" fullWidth>
        <DialogTitle>Create New Wallet</DialogTitle>
        <DialogContent>
          <Box sx={{ pt: 2, display: 'flex', flexDirection: 'column', gap: 2 }}>
            <TextField
              fullWidth
              label="Wallet Name"
              value={formData.name}
              onChange={(e) => setFormData({ ...formData, name: e.target.value })}
            />
            <FormControl fullWidth>
              <InputLabel>Wallet Type</InputLabel>
              <Select
                value={formData.type}
                label="Wallet Type"
                onChange={(e) => setFormData({ ...formData, type: e.target.value as Wallet['type'] })}
              >
                <MenuItem value="hot">Hot Wallet</MenuItem>
                <MenuItem value="cold">Cold Wallet</MenuItem>
                <MenuItem value="multi_sig">Multi-Sig</MenuItem>
                <MenuItem value="treasury">Treasury</MenuItem>
                <MenuItem value="operations">Operations</MenuItem>
              </Select>
            </FormControl>
            <FormControl fullWidth>
              <InputLabel>Chain</InputLabel>
              <Select
                value={formData.chainId}
                label="Chain"
                onChange={(e) => setFormData({ ...formData, chainId: e.target.value as number })}
              >
                <MenuItem value={1}>Ethereum</MenuItem>
                <MenuItem value={56}>BNB Chain</MenuItem>
                <MenuItem value={137}>Polygon</MenuItem>
                <MenuItem value={42161}>Arbitrum</MenuItem>
              </Select>
            </FormControl>
            {formData.type === 'multi_sig' && (
              <>
                <Typography variant="subtitle2">Signers</Typography>
                {formData.signers.map((signer, i) => (
                  <TextField
                    key={i}
                    fullWidth
                    label={`Signer ${i + 1} Address`}
                    value={signer}
                    onChange={(e) => {
                      const newSigners = [...formData.signers];
                      newSigners[i] = e.target.value;
                      setFormData({ ...formData, signers: newSigners });
                    }}
                  />
                ))}
                <Button variant="outlined" onClick={() => setFormData({ ...formData, signers: [...formData.signers, ''] })}>
                  Add Signer
                </Button>
                <TextField
                  fullWidth
                  label="Threshold"
                  type="number"
                  value={formData.threshold}
                  onChange={(e) => setFormData({ ...formData, threshold: parseInt(e.target.value) })}
                />
              </>
            )}
          </Box>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setCreateDialogOpen(false)}>Cancel</Button>
          <Button variant="contained" onClick={handleCreateWallet} disabled={loading}>
            Create
          </Button>
        </DialogActions>
      </Dialog>
      
      {/* Wallet Detail Dialog */}
      <Dialog open={detailDialogOpen} onClose={() => setDetailDialogOpen(false)} maxWidth="md" fullWidth>
        <DialogTitle>Wallet Details</DialogTitle>
        <DialogContent>
          {selectedWallet && (
            <Box sx={{ pt: 2 }}>
              <Typography variant="h5" gutterBottom>{selectedWallet.name}</Typography>
              <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 3, p: 2, bgcolor: 'grey.100', borderRadius: 1 }}>
                <Typography variant="body1" sx={{ fontFamily: 'monospace', flex: 1 }}>
                  {selectedWallet.address}
                </Typography>
                <IconButton onClick={() => copyAddress(selectedWallet.address)}>
                  <ContentCopy />
                </IconButton>
              </Box>
              <Grid container spacing={2}>
                <Grid item xs={6}>
                  <Typography variant="caption" color="text.secondary">Type</Typography>
                  <Typography variant="body1">{selectedWallet.type.toUpperCase()}</Typography>
                </Grid>
                <Grid item xs={6}>
                  <Typography variant="caption" color="text.secondary">Chain</Typography>
                  <Typography variant="body1">{selectedWallet.chainName}</Typography>
                </Grid>
                <Grid item xs={6}>
                  <Typography variant="caption" color="text.secondary">Balance</Typography>
                  <Typography variant="h6">{selectedWallet.balance.toFixed(4)} ETH</Typography>
                </Grid>
                <Grid item xs={6}>
                  <Typography variant="caption" color="text.secondary">USD Value</Typography>
                  <Typography variant="h6">{formatCurrency(selectedWallet.balanceUSD)}</Typography>
                </Grid>
                <Grid item xs={6}>
                  <Typography variant="caption" color="text.secondary">Status</Typography>
                  <Chip label={selectedWallet.status.toUpperCase()} size="small" color={selectedWallet.status === 'active' ? 'success' : 'error'} />
                </Grid>
                <Grid item xs={6}>
                  <Typography variant="caption" color="text.secondary">Created</Typography>
                  <Typography variant="body1">{formatDate(selectedWallet.createdAt)}</Typography>
                </Grid>
              </Grid>
            </Box>
          )}
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setDetailDialogOpen(false)}>Close</Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
}