'use client';

import React, { useState, useEffect, useCallback } from 'react';
import {
  Box, Typography, Card, CardContent, Button, TextField, Tabs, Tab, Chip,
  Table, TableBody, TableCell, TableContainer, TableHead, TableRow,
  Paper, IconButton, Dialog, DialogTitle, DialogContent, DialogActions,
  Alert, Select, MenuItem, FormControl, InputLabel, Switch,
  FormControlLabel, Slider, Grid, Avatar, Divider, List, ListItem,
  ListItemText, ListItemIcon, LinearProgress, Tooltip, Badge, Accordion,
  AccordionSummary, AccordionDetails, SelectChangeEvent, TableSortLabel
} from '@mui/material';
import {
  AccountBalance, MonetizationOn, AttachMoney, Receipt, TrendingUp,
  Settings, Add, Edit, Delete, Refresh, ExpandMore, Warning,
  CheckCircle, Error as ErrorIcon, Money, CreditCard, Wallet
} from '@mui/icons-material';

// ============================================================================
// Types & Interfaces
// ============================================================================

interface FeeConfig {
  id: string;
  name: string;
  type: 'swap' | 'mint' | 'burn' | 'bot' | 'withdrawal' | 'listing' | 'subscription';
  chainId: number;
  chainName: string;
  feeType: 'flat' | 'percentage' | 'tiered';
  value: number;
  minValue?: number;
  maxValue?: number;
  tier?: number;
  isActive: boolean;
  updatedAt: number;
}

interface FeeTransaction {
  id: string;
  type: string;
  amount: number;
  token: string;
  feeRecipient: string;
  txHash: string;
  timestamp: number;
  status: 'pending' | 'collected' | 'distributed';
}

interface RevenueStats {
  totalRevenue: number;
  swapFees: number;
  mintFees: number;
  burnFees: number;
  botFees: number;
  listingFees: number;
  subscriptionFees: number;
  withdrawalFees: number;
  monthlyGrowth: number;
}

// ============================================================================
// Constants
// ============================================================================

const FEE_TYPES = [
  { value: 'swap', label: 'Swap Fee', description: 'Fee on token swaps' },
  { value: 'mint', label: 'Mint Fee', description: 'Fee on liquidity provision' },
  { value: 'burn', label: 'Burn Fee', description: 'Fee on liquidity removal' },
  { value: 'bot', label: 'Bot Fee', description: 'Trading bot fees' },
  { value: 'listing', label: 'Listing Fee', description: 'Token listing fees' },
  { value: 'subscription', label: 'Subscription Fee', description: 'Bot subscription fees' },
  { value: 'withdrawal', label: 'Withdrawal Fee', description: 'Network withdrawal fees' },
];

const CHAINS = [
  { id: 1, name: 'Ethereum', symbol: 'ETH' },
  { id: 56, name: 'BNB Chain', symbol: 'BNB' },
  { id: 137, name: 'Polygon', symbol: 'MATIC' },
  { id: 42161, name: 'Arbitrum', symbol: 'ETH' },
  { id: 10, name: 'Optimism', symbol: 'ETH' },
  { id: 8453, name: 'Base', symbol: 'ETH' },
];

// ============================================================================
// Component
// ============================================================================

export default function AdminFeesPage() {
  // State
  const [activeTab, setActiveTab] = useState(0);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);
  
  // Data
  const [feeConfigs, setFeeConfigs] = useState<FeeConfig[]>([]);
  const [transactions, setTransactions] = useState<FeeTransaction[]>([]);
  const [revenueStats, setRevenueStats] = useState<RevenueStats | null>(null);
  
  // Dialogs
  const [editDialogOpen, setEditDialogOpen] = useState(false);
  const [selectedFee, setSelectedFee] = useState<FeeConfig | null>(null);
  
  // Form
  const [formData, setFormData] = useState({
    name: '',
    type: 'swap' as FeeConfig['type'],
    chainId: 1,
    feeType: 'percentage' as FeeConfig['feeType'],
    value: 0,
    minValue: 0,
    maxValue: 0,
    isActive: true,
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
      
      // Mock fee configs
      setFeeConfigs([
        { id: '1', name: 'ETH/USDC Swap', type: 'swap', chainId: 1, chainName: 'Ethereum', feeType: 'percentage', value: 0.3, isActive: true, updatedAt: Date.now() - 86400000 },
        { id: '2', name: 'BTC/USDC Swap', type: 'swap', chainId: 1, chainName: 'Ethereum', feeType: 'percentage', value: 0.3, isActive: true, updatedAt: Date.now() - 86400000 },
        { id: '3', name: 'BNB/USDT Swap', type: 'swap', chainId: 56, chainName: 'BNB Chain', feeType: 'percentage', value: 0.25, isActive: true, updatedAt: Date.now() - 86400000 },
        { id: '4', name: 'MATIC/USDC Swap', type: 'swap', chainId: 137, chainName: 'Polygon', feeType: 'percentage', value: 0.3, isActive: true, updatedAt: Date.now() - 86400000 },
        { id: '5', name: 'Liquidity Mint', type: 'mint', chainId: 1, chainName: 'Ethereum', feeType: 'percentage', value: 0.5, isActive: true, updatedAt: Date.now() - 86400000 },
        { id: '6', name: 'Liquidity Burn', type: 'burn', chainId: 1, chainName: 'Ethereum', feeType: 'percentage', value: 0.5, isActive: true, updatedAt: Date.now() - 86400000 },
        { id: '7', name: 'Market Maker Bot', type: 'bot', chainId: 1, chainName: 'All Chains', feeType: 'flat', value: 5000, isActive: true, updatedAt: Date.now() - 86400000 },
        { id: '8', name: 'Arbitrage Bot', type: 'bot', chainId: 1, chainName: 'All Chains', feeType: 'flat', value: 3000, isActive: true, updatedAt: Date.now() - 86400000 },
        { id: '9', name: 'Token Listing', type: 'listing', chainId: 1, chainName: 'Ethereum', feeType: 'flat', value: 10000, isActive: true, updatedAt: Date.now() - 86400000 },
        { id: '10', name: 'Bot Subscription', type: 'subscription', chainId: 1, chainName: 'All Chains', feeType: 'flat', value: 500, isActive: true, updatedAt: Date.now() - 86400000 },
        { id: '11', name: 'Network Withdrawal', type: 'withdrawal', chainId: 1, chainName: 'Ethereum', feeType: 'tiered', value: 10, minValue: 0, maxValue: 100, tier: 1, isActive: true, updatedAt: Date.now() - 86400000 },
        { id: '12', name: 'Network Withdrawal', type: 'withdrawal', chainId: 56, chainName: 'BNB Chain', feeType: 'tiered', value: 1, minValue: 0, maxValue: 1000, tier: 1, isActive: true, updatedAt: Date.now() - 86400000 },
      ]);
      
      // Mock transactions
      setTransactions([
        { id: 'tx1', type: 'swap', amount: 1500, token: 'USDC', feeRecipient: '0xfee...', txHash: '0x123...', timestamp: Date.now() - 60000, status: 'collected' },
        { id: 'tx2', type: 'bot', amount: 5000, token: 'USDT', feeRecipient: '0xfee...', txHash: '0x456...', timestamp: Date.now() - 120000, status: 'distributed' },
        { id: 'tx3', type: 'listing', amount: 10000, token: 'USDC', feeRecipient: '0xfee...', txHash: '0x789...', timestamp: Date.now() - 300000, status: 'collected' },
        { id: 'tx4', type: 'subscription', amount: 500, token: 'USDC', feeRecipient: '0xfee...', txHash: '0xabc...', timestamp: Date.now() - 600000, status: 'collected' },
        { id: 'tx5', type: 'swap', amount: 800, token: 'USDC', feeRecipient: '0xfee...', txHash: '0xdef...', timestamp: Date.now() - 900000, status: 'distributed' },
      ]);
      
      // Mock revenue stats
      setRevenueStats({
        totalRevenue: 1250000,
        swapFees: 450000,
        mintFees: 120000,
        burnFees: 80000,
        botFees: 350000,
        listingFees: 150000,
        subscriptionFees: 50000,
        withdrawalFees: 50000,
        monthlyGrowth: 15.5,
      });
      
      setSuccess('Fee data loaded successfully');
    } catch (err: any) {
      setError(err.message || 'Failed to load data');
    } finally {
      setLoading(false);
    }
  }, []);
  
  // ============================================================================
  // Actions
  // ============================================================================
  
  const handleEditFee = useCallback((fee: FeeConfig) => {
    setSelectedFee(fee);
    setFormData({
      name: fee.name,
      type: fee.type,
      chainId: fee.chainId,
      feeType: fee.feeType,
      value: fee.value,
      minValue: fee.minValue || 0,
      maxValue: fee.maxValue || 0,
      isActive: fee.isActive,
    });
    setEditDialogOpen(true);
  }, []);
  
  const handleSaveFee = useCallback(async () => {
    setLoading(true);
    try {
      await new Promise(resolve => setTimeout(resolve, 500));
      
      if (selectedFee) {
        setFeeConfigs(feeConfigs.map(f => 
          f.id === selectedFee.id ? { ...f, ...formData, updatedAt: Date.now() } : f
        ));
        setSuccess(`Fee "${formData.name}" updated successfully`);
      } else {
        const newFee: FeeConfig = {
          id: `fee-${Date.now()}`,
          ...formData,
          chainName: CHAINS.find(c => c.id === formData.chainId)?.name || 'Unknown',
          updatedAt: Date.now(),
        };
        setFeeConfigs([...feeConfigs, newFee]);
        setSuccess(`Fee "${formData.name}" created successfully`);
      }
      
      setEditDialogOpen(false);
    } catch (err: any) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  }, [selectedFee, formData, feeConfigs]);
  
  const handleDeleteFee = useCallback(async (feeId: string) => {
    setLoading(true);
    try {
      await new Promise(resolve => setTimeout(resolve, 300));
      setFeeConfigs(feeConfigs.filter(f => f.id !== feeId));
      setSuccess('Fee deleted successfully');
    } catch (err: any) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  }, [feeConfigs]);
  
  const handleToggleFee = useCallback(async (feeId: string) => {
    setFeeConfigs(feeConfigs.map(f => 
      f.id === feeId ? { ...f, isActive: !f.isActive } : f
    ));
  }, [feeConfigs]);
  
  // ============================================================================
  // Helper Functions
  // ============================================================================
  
  const formatCurrency = (amount: number) => {
    return amount.toLocaleString('en-US', { style: 'currency', currency: 'USD' });
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
  
  return (
    <Box sx={{ p: 3 }}>
      {/* Header */}
      <Box sx={{ mb: 3, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <Box>
          <Typography variant="h5" fontWeight="bold">Fee Management</Typography>
          <Typography variant="body2" color="text.secondary">
            Configure all protocol fees, tiers, and revenue tracking
          </Typography>
        </Box>
        <Box sx={{ display: 'flex', gap: 2 }}>
          <Button variant="outlined" startIcon={<Refresh />} onClick={loadData}>Refresh</Button>
          <Button variant="contained" startIcon={<Add />} onClick={() => {
            setSelectedFee(null);
            setFormData({
              name: '',
              type: 'swap',
              chainId: 1,
              feeType: 'percentage',
              value: 0.3,
              minValue: 0,
              maxValue: 0,
              isActive: true,
            });
            setEditDialogOpen(true);
          }}>
            Add Fee
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
      
      {/* Revenue Stats */}
      {revenueStats && (
        <Grid container spacing={3} sx={{ mb: 3 }}>
          <Grid item xs={12} sm={6} md={3}>
            <Card>
              <CardContent>
                <Typography color="text.secondary" gutterBottom>Total Revenue</Typography>
                <Typography variant="h4" color="primary">{formatCurrency(revenueStats.totalRevenue)}</Typography>
                <Typography variant="caption" color="success.main">+{revenueStats.monthlyGrowth}%</Typography>
              </CardContent>
            </Card>
          </Grid>
          <Grid item xs={12} sm={6} md={3}>
            <Card>
              <CardContent>
                <Typography color="text.secondary" gutterBottom>Swap Fees</Typography>
                <Typography variant="h5">{formatCurrency(revenueStats.swapFees)}</Typography>
              </CardContent>
            </Card>
          </Grid>
          <Grid item xs={12} sm={6} md={3}>
            <Card>
              <CardContent>
                <Typography color="text.secondary" gutterBottom>Bot Fees</Typography>
                <Typography variant="h5">{formatCurrency(revenueStats.botFees)}</Typography>
              </CardContent>
            </Card>
          </Grid>
          <Grid item xs={12} sm={6} md={3}>
            <Card>
              <CardContent>
                <Typography color="text.secondary" gutterBottom>Listing Fees</Typography>
                <Typography variant="h5">{formatCurrency(revenueStats.listingFees)}</Typography>
              </CardContent>
            </Card>
          </Grid>
        </Grid>
      )}
      
      {/* Tabs */}
      <Tabs value={activeTab} onChange={(_, v) => setActiveTab(v)} sx={{ mb: 3 }}>
        <Tab label="Fee Configuration" icon={<MonetizationOn />} iconPosition="start" />
        <Tab label="Fee Transactions" icon={<Receipt />} iconPosition="start" />
        <Tab label="Revenue Analytics" icon={<TrendingUp />} iconPosition="start" />
      </Tabs>
      
      {/* Fee Configuration Tab */}
      {activeTab === 0 && (
        <TableContainer component={Paper}>
          <Table>
            <TableHead>
              <TableRow>
                <TableCell>Name</TableCell>
                <TableCell>Type</TableCell>
                <TableCell>Chain</TableCell>
                <TableCell>Fee Type</TableCell>
                <TableCell align="right">Value</TableCell>
                <TableCell>Status</TableCell>
                <TableCell>Actions</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {feeConfigs.map(fee => (
                <TableRow key={fee.id}>
                  <TableCell>{fee.name}</TableCell>
                  <TableCell>
                    <Chip 
                      label={fee.type} 
                      size="small"
                      color={fee.type === 'swap' ? 'primary' : fee.type === 'bot' ? 'secondary' : 'default'}
                    />
                  </TableCell>
                  <TableCell>{fee.chainName}</TableCell>
                  <TableCell sx={{ textTransform: 'capitalize' }}>{fee.feeType}</TableCell>
                  <TableCell align="right">
                    {fee.feeType === 'percentage' ? `${fee.value}%` : formatCurrency(fee.value)}
                    {fee.tier && ` (Tier ${fee.tier})`}
                  </TableCell>
                  <TableCell>
                    <Chip 
                      label={fee.isActive ? 'Active' : 'Inactive'}
                      size="small"
                      color={fee.isActive ? 'success' : 'default'}
                    />
                  </TableCell>
                  <TableCell>
                    <Box sx={{ display: 'flex', gap: 1 }}>
                      <IconButton size="small" onClick={() => handleEditFee(fee)}>
                        <Edit fontSize="small" />
                      </IconButton>
                      <IconButton size="small" onClick={() => handleToggleFee(fee.id)}>
                        {fee.isActive ? <ErrorIcon fontSize="small" /> : <CheckCircle fontSize="small" />}
                      </IconButton>
                      <IconButton size="small" color="error" onClick={() => handleDeleteFee(fee.id)}>
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
      
      {/* Transactions Tab */}
      {activeTab === 1 && (
        <TableContainer component={Paper}>
          <Table>
            <TableHead>
              <TableRow>
                <TableCell>Type</TableCell>
                <TableCell align="right">Amount</TableCell>
                <TableCell>Token</TableCell>
                <TableCell>Fee Recipient</TableCell>
                <TableCell>Tx Hash</TableCell>
                <TableCell>Time</TableCell>
                <TableCell>Status</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {transactions.map(tx => (
                <TableRow key={tx.id}>
                  <TableCell>
                    <Chip 
                      label={tx.type} 
                      size="small"
                      color={tx.type === 'swap' ? 'primary' : tx.type === 'bot' ? 'secondary' : 'default'}
                    />
                  </TableCell>
                  <TableCell align="right" sx={{ fontWeight: 'bold' }}>{formatCurrency(tx.amount)}</TableCell>
                  <TableCell>{tx.token}</TableCell>
                  <TableCell sx={{ fontFamily: 'monospace', fontSize: '0.8rem' }}>{tx.feeRecipient}</TableCell>
                  <TableCell sx={{ fontFamily: 'monospace', fontSize: '0.8rem' }}>{tx.txHash}</TableCell>
                  <TableCell>{formatDate(tx.timestamp)}</TableCell>
                  <TableCell>
                    <Chip 
                      label={tx.status}
                      size="small"
                      color={tx.status === 'collected' ? 'success' : tx.status === 'distributed' ? 'info' : 'warning'}
                    />
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </TableContainer>
      )}
      
      {/* Analytics Tab */}
      {activeTab === 2 && (
        <Grid container spacing={3}>
          <Grid item xs={12} md={6}>
            <Card>
              <CardContent>
                <Typography variant="h6" gutterBottom>Revenue by Fee Type</Typography>
                {revenueStats && (
                  <Box sx={{ mt: 2 }}>
                    {Object.entries({
                      'Swap Fees': revenueStats.swapFees,
                      'Mint Fees': revenueStats.mintFees,
                      'Burn Fees': revenueStats.burnFees,
                      'Bot Fees': revenueStats.botFees,
                      'Listing Fees': revenueStats.listingFees,
                      'Subscription Fees': revenueStats.subscriptionFees,
                      'Withdrawal Fees': revenueStats.withdrawalFees,
                    }).map(([name, value]) => (
                      <Box key={name} sx={{ display: 'flex', justifyContent: 'space-between', mb: 1 }}>
                        <Typography>{name}</Typography>
                        <Typography fontWeight="bold">{formatCurrency(value)}</Typography>
                      </Box>
                    ))}
                  </Box>
                )}
              </CardContent>
            </Card>
          </Grid>
          <Grid item xs={12} md={6}>
            <Card>
              <CardContent>
                <Typography variant="h6" gutterBottom>Fee Distribution</Typography>
                <Typography variant="body2" color="text.secondary">
                  Fee breakdown across different categories
                </Typography>
                {revenueStats && (
                  <Box sx={{ mt: 2 }}>
                    {[
                      { label: 'Swap', value: revenueStats.swapFees / revenueStats.totalRevenue * 100 },
                      { label: 'Bot', value: revenueStats.botFees / revenueStats.totalRevenue * 100 },
                      { label: 'Listing', value: revenueStats.listingFees / revenueStats.totalRevenue * 100 },
                      { label: 'Mint/Burn', value: (revenueStats.mintFees + revenueStats.burnFees) / revenueStats.totalRevenue * 100 },
                      { label: 'Other', value: (revenueStats.subscriptionFees + revenueStats.withdrawalFees) / revenueStats.totalRevenue * 100 },
                    ].map(item => (
                      <Box key={item.label} sx={{ mb: 2 }}>
                        <Box sx={{ display: 'flex', justifyContent: 'space-between', mb: 0.5 }}>
                          <Typography variant="body2">{item.label}</Typography>
                          <Typography variant="body2" fontWeight="bold">{item.value.toFixed(1)}%</Typography>
                        </Box>
                        <LinearProgress variant="determinate" value={item.value} sx={{ height: 8, borderRadius: 4 }} />
                      </Box>
                    ))}
                  </Box>
                )}
              </CardContent>
            </Card>
          </Grid>
        </Grid>
      )}
      
      {/* Edit Dialog */}
      <Dialog open={editDialogOpen} onClose={() => setEditDialogOpen(false)} maxWidth="sm" fullWidth>
        <DialogTitle>{selectedFee ? 'Edit Fee' : 'Create New Fee'}</DialogTitle>
        <DialogContent>
          <Box sx={{ pt: 2, display: 'flex', flexDirection: 'column', gap: 2 }}>
            <TextField
              fullWidth
              label="Fee Name"
              value={formData.name}
              onChange={(e) => setFormData({ ...formData, name: e.target.value })}
            />
            <FormControl fullWidth>
              <InputLabel>Fee Type</InputLabel>
              <Select
                value={formData.type}
                label="Fee Type"
                onChange={(e) => setFormData({ ...formData, type: e.target.value as FeeConfig['type'] })}
              >
                {FEE_TYPES.map(ft => (
                  <MenuItem key={ft.value} value={ft.value}>{ft.label}</MenuItem>
                ))}
              </Select>
            </FormControl>
            <FormControl fullWidth>
              <InputLabel>Chain</InputLabel>
              <Select
                value={formData.chainId}
                label="Chain"
                onChange={(e) => setFormData({ ...formData, chainId: e.target.value as number })}
              >
                {CHAINS.map(c => (
                  <MenuItem key={c.id} value={c.id}>{c.name}</MenuItem>
                ))}
              </Select>
            </FormControl>
            <FormControl fullWidth>
              <InputLabel>Calculation Type</InputLabel>
              <Select
                value={formData.feeType}
                label="Calculation Type"
                onChange={(e) => setFormData({ ...formData, feeType: e.target.value as FeeConfig['feeType'] })}
              >
                <MenuItem value="percentage">Percentage (%)</MenuItem>
                <MenuItem value="flat">Flat Fee ($)</MenuItem>
                <MenuItem value="tiered">Tiered</MenuItem>
              </Select>
            </FormControl>
            <TextField
              fullWidth
              label={formData.feeType === 'percentage' ? 'Fee (%)' : 'Fee (USD)'}
              type="number"
              value={formData.value}
              onChange={(e) => setFormData({ ...formData, value: parseFloat(e.target.value) })}
            />
            {formData.feeType === 'tiered' && (
              <>
                <TextField
                  fullWidth
                  label="Min Value"
                  type="number"
                  value={formData.minValue}
                  onChange={(e) => setFormData({ ...formData, minValue: parseFloat(e.target.value) })}
                />
                <TextField
                  fullWidth
                  label="Max Value"
                  type="number"
                  value={formData.maxValue}
                  onChange={(e) => setFormData({ ...formData, maxValue: parseFloat(e.target.value) })}
                />
              </>
            )}
            <FormControlLabel
              control={
                <Switch
                  checked={formData.isActive}
                  onChange={(e) => setFormData({ ...formData, isActive: e.target.checked })}
                />
              }
              label="Active"
            />
          </Box>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setEditDialogOpen(false)}>Cancel</Button>
          <Button variant="contained" onClick={handleSaveFee} disabled={loading}>
            {selectedFee ? 'Update' : 'Create'}
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
}