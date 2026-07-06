'use client';
import React, { useState, useEffect, useCallback } from 'react';
import { Box, Typography, Card, CardContent, Button, TextField, Table, TableBody, TableCell, TableContainer, TableHead, TableRow, Chip, IconButton, Select, MenuItem, FormControl, InputLabel, Dialog, DialogTitle, DialogContent, DialogActions, Grid, Alert, Snackbar, CircularProgress, Divider, Switch, FormControlLabel } from '@mui/material';
import { Add, Edit, Delete, Refresh, Search, Info } from '@mui/icons-material';
import { chainRegistry } from '../../../../libs/chain_registry/universal_chain_registry';
// ============================================================================
// Constants
// ============================================================================
const CHAIN_CATEGORIES = [
    { value: 'evm', label: 'EVM (Ethereum Virtual Machine)' },
    { value: 'solana', label: 'Solana' },
    { value: 'aptos', label: 'Aptos' },
    { value: 'sui', label: 'Sui' },
    { value: 'ton', label: 'TON' },
    { value: 'tron', label: 'TRON' },
    { value: 'cosmos', label: 'Cosmos' },
    { value: 'near', label: 'NEAR' },
    { value: 'algorand', label: 'Algorand' },
    { value: 'polkadot', label: 'Polkadot' },
    { value: 'cardano', label: 'Cardano' },
    { value: 'other', label: 'Other' },
];
const CHAIN_STATUSES = [
    { value: 'active', label: 'Active', color: 'success' },
    { value: 'inactive', label: 'Inactive', color: 'default' },
    { value: 'deprecated', label: 'Deprecated', color: 'error' },
    { value: 'maintenance', label: 'Maintenance', color: 'warning' },
];
const CATEGORY_COLORS = {
    evm: '#627EEA',
    solana: '#9945FF',
    aptos: '#7A3EED',
    sui: '#6FBCF0',
    ton: '#0088CC',
    tron: '#EB0029',
    cosmos: '#2E3148',
    near: '#000000',
    algorand: '#000000',
    polkadot: '#E6007A',
    cardano: '#0033AD',
    other: '#808080',
};
// ============================================================================
// Utility Functions
// ============================================================================
function formatDate(timestamp) {
    if (!timestamp)
        return 'N/A';
    return new Date(timestamp).toLocaleDateString();
}
function getCategoryColor(category) {
    return CATEGORY_COLORS[category] || '#808080';
}
function getStatusColor(status) {
    switch (status) {
        case 'active': return 'success';
        case 'inactive': return 'default';
        case 'deprecated': return 'error';
        case 'maintenance': return 'warning';
        default: return 'default';
    }
}
function validateChainId(chainId) {
    return chainId >= 0 && chainId <= Number.MAX_SAFE_INTEGER;
}
// ============================================================================
// Main Component
// ============================================================================
export default function ChainManagementPage() {
    // State
    const [chains, setChains] = useState([]);
    const [filteredChains, setFilteredChains] = useState([]);
    const [loading, setLoading] = useState(true);
    const [searchQuery, setSearchQuery] = useState('');
    const [categoryFilter, setCategoryFilter] = useState('all');
    const [statusFilter, setStatusFilter] = useState('all');
    // Dialogs
    const [chainDialogOpen, setChainDialogOpen] = useState(false);
    const [rpcDialogOpen, setRpcDialogOpen] = useState(false);
    const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
    const [viewDialogOpen, setViewDialogOpen] = useState(false);
    const [selectedChain, setSelectedChain] = useState(null);
    const [selectedRpcChain, setSelectedRpcChain] = useState(null);
    // Form Data
    const [chainForm, setChainForm] = useState({
        id: '', name: '', symbol: '', category: 'evm', status: 'active',
        chainId: 0, rpcUrls: '', explorerUrls: '', nativeCurrencyName: '',
        nativeCurrencySymbol: '', nativeCurrencyDecimals: 18, blockTime: 3,
        gasLimit: 30000000, supportsEIP1559: true, supportsFlashbots: false,
        supportsMEV: false, supportsMulticall: true, notes: ''
    });
    const [rpcForm, setRpcForm] = useState({
        chainId: '', url: '', name: '', priority: 1, isWebSocket: false
    });
    // Stats
    const [stats, setStats] = useState({
        total: 0, evmCount: 0, nonEvmCount: 0,
        byCategory: {},
        byStatus: {}
    });
    // Snackbar
    const [snackbar, setSnackbar] = useState({ open: false, message: '', severity: 'info' });
    // ============================================================================
    // Load Data
    // ============================================================================
    const loadChains = useCallback(() => {
        setLoading(true);
        try {
            const allChains = chainRegistry.getAllChains();
            setChains(allChains);
            setFilteredChains(allChains);
            setStats(chainRegistry.getChainStats());
        }
        catch (error) {
            console.error('Failed to load chains:', error);
            showSnackbar('Failed to load chains', 'error');
        }
        setLoading(false);
    }, []);
    useEffect(() => {
        loadChains();
    }, [loadChains]);
    // ============================================================================
    // Filtering
    // ============================================================================
    useEffect(() => {
        let filtered = chains;
        if (searchQuery) {
            const query = searchQuery.toLowerCase();
            filtered = filtered.filter(c => c.name.toLowerCase().includes(query) ||
                c.symbol.toLowerCase().includes(query) ||
                c.id.toLowerCase().includes(query) ||
                c.chainId.toString().includes(query));
        }
        if (categoryFilter !== 'all') {
            filtered = filtered.filter(c => c.category === categoryFilter);
        }
        if (statusFilter !== 'all') {
            filtered = filtered.filter(c => c.status === statusFilter);
        }
        setFilteredChains(filtered);
    }, [chains, searchQuery, categoryFilter, statusFilter]);
    // ============================================================================
    // CRUD Operations
    // ============================================================================
    const handleAddChain = () => {
        setSelectedChain(null);
        setChainForm({
            id: '', name: '', symbol: '', category: 'evm', status: 'active',
            chainId: 0, rpcUrls: '', explorerUrls: '', nativeCurrencyName: '',
            nativeCurrencySymbol: '', nativeCurrencyDecimals: 18, blockTime: 3,
            gasLimit: 30000000, supportsEIP1559: true, supportsFlashbots: false,
            supportsMEV: false, supportsMulticall: true, notes: ''
        });
        setChainDialogOpen(true);
    };
    const handleEditChain = (chain) => {
        setSelectedChain(chain);
        setChainForm({
            id: chain.id,
            name: chain.name,
            symbol: chain.symbol,
            category: chain.category,
            status: chain.status,
            chainId: chain.chainId,
            rpcUrls: chain.rpcUrls.join('\n'),
            explorerUrls: chain.explorerUrls.join('\n'),
            nativeCurrencyName: chain.nativeCurrency.name,
            nativeCurrencySymbol: chain.nativeCurrency.symbol,
            nativeCurrencyDecimals: chain.nativeCurrency.decimals,
            blockTime: chain.blockTime,
            gasLimit: chain.gasLimit,
            supportsEIP1559: chain.supportsEIP1559 || false,
            supportsFlashbots: chain.supportsFlashbots || false,
            supportsMEV: chain.supportsMEV || false,
            supportsMulticall: chain.supportsMulticall || false,
            notes: chain.notes || ''
        });
        setChainDialogOpen(true);
    };
    const handleViewChain = (chain) => {
        setSelectedChain(chain);
        setViewDialogOpen(true);
    };
    const handleDeleteChain = (chain) => {
        setSelectedChain(chain);
        setDeleteDialogOpen(true);
    };
    const confirmDeleteChain = () => {
        if (!selectedChain)
            return;
        try {
            chainRegistry.removeChain(selectedChain.id);
            showSnackbar(`Chain ${selectedChain.name} deleted successfully`, 'success');
            loadChains();
        }
        catch (error) {
            showSnackbar(error.message || 'Failed to delete chain', 'error');
        }
        setDeleteDialogOpen(false);
        setSelectedChain(null);
    };
    const handleSaveChain = () => {
        try {
            // Validate
            if (!chainForm.id.trim())
                throw new Error('Chain ID is required');
            if (!chainForm.name.trim())
                throw new Error('Chain name is required');
            if (!chainForm.rpcUrls.trim())
                throw new Error('RPC URLs are required');
            if (!chainForm.nativeCurrencyName.trim())
                throw new Error('Native currency name is required');
            if (!chainForm.nativeCurrencySymbol.trim())
                throw new Error('Native currency symbol is required');
            const chainConfig = {
                id: chainForm.id.toLowerCase().replace(/\s+/g, '-'),
                name: chainForm.name,
                symbol: chainForm.symbol || chainForm.nativeCurrencySymbol,
                category: chainForm.category,
                status: chainForm.status,
                chainId: chainForm.chainId,
                rpcUrls: chainForm.rpcUrls.split('\n').map(url => url.trim()).filter(Boolean),
                explorerUrls: chainForm.explorerUrls.split('\n').map(url => url.trim()).filter(Boolean),
                nativeCurrency: {
                    name: chainForm.nativeCurrencyName,
                    symbol: chainForm.nativeCurrencySymbol,
                    decimals: chainForm.nativeCurrencyDecimals,
                },
                blockTime: chainForm.blockTime,
                gasLimit: chainForm.gasLimit,
                supportsEIP1559: chainForm.supportsEIP1559,
                supportsFlashbots: chainForm.supportsFlashbots,
                supportsMEV: chainForm.supportsMEV,
                supportsMulticall: chainForm.supportsMulticall,
                notes: chainForm.notes,
            };
            if (selectedChain) {
                chainRegistry.updateChain(selectedChain.id, chainConfig);
                showSnackbar(`Chain ${chainConfig.name} updated successfully`, 'success');
            }
            else {
                chainRegistry.addChain(chainConfig);
                showSnackbar(`Chain ${chainConfig.name} added successfully`, 'success');
            }
            setChainDialogOpen(false);
            loadChains();
        }
        catch (error) {
            showSnackbar(error.message || 'Failed to save chain', 'error');
        }
    };
    // ============================================================================
    // RPC Management
    // ============================================================================
    const handleAddRpc = (chainId) => {
        setSelectedRpcChain(chainId);
        setRpcForm({
            chainId, url: '', name: '', priority: 1, isWebSocket: false
        });
        setRpcDialogOpen(true);
    };
    const confirmAddRpc = () => {
        if (!selectedRpcChain || !rpcForm.url.trim()) {
            showSnackbar('RPC URL is required', 'error');
            return;
        }
        try {
            chainRegistry.addRPCEndpoint(selectedRpcChain, {
                url: rpcForm.url,
                name: rpcForm.name || `RPC ${Date.now()}`,
                priority: rpcForm.priority,
                isWebSocket: rpcForm.isWebSocket,
                isHealthy: true,
                latencyMs: 0,
                lastCheck: Date.now(),
            });
            showSnackbar('RPC endpoint added successfully', 'success');
            setRpcDialogOpen(false);
            loadChains();
        }
        catch (error) {
            showSnackbar(error.message || 'Failed to add RPC', 'error');
        }
    };
    // ============================================================================
    // Helpers
    // ============================================================================
    const showSnackbar = (message, severity) => {
        setSnackbar({ open: true, message, severity });
    };
    // ============================================================================
    // Render
    // ============================================================================
    if (loading) {
        return (<Box sx={{ display: 'flex', justifyContent: 'center', p: 4 }}>
        <CircularProgress />
      </Box>);
    }
    return (<Box sx={{ p: 3 }}>
      {/* Header */}
      <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 3 }}>
        <Box>
          <Typography variant="h4" sx={{ fontWeight: 'bold', color: '#00d4aa' }}>
            Chain Management
          </Typography>
          <Typography variant="body2" sx={{ color: 'gray' }}>
            Add, edit, update, and remove blockchain networks
          </Typography>
        </Box>
        <Button variant="contained" startIcon={<Add />} onClick={handleAddChain} sx={{ bgcolor: '#00d4aa', color: 'black', '&:hover': { bgcolor: '#00b894' } }}>
          Add Chain
        </Button>
      </Box>

      {/* Stats Cards */}
      <Grid container spacing={2} sx={{ mb: 3 }}>
        <Grid item xs={12} sm={6} md={3}>
          <Card sx={{ bgcolor: '#1a1a2e' }}>
            <CardContent>
              <Typography variant="h4" sx={{ color: '#00d4aa' }}>{stats.total}</Typography>
              <Typography variant="body2" sx={{ color: 'gray' }}>Total Chains</Typography>
            </CardContent>
          </Card>
        </Grid>
        <Grid item xs={12} sm={6} md={3}>
          <Card sx={{ bgcolor: '#1a1a2e' }}>
            <CardContent>
              <Typography variant="h4" sx={{ color: '#627EEA' }}>{stats.evmCount}</Typography>
              <Typography variant="body2" sx={{ color: 'gray' }}>EVM Chains</Typography>
            </CardContent>
          </Card>
        </Grid>
        <Grid item xs={12} sm={6} md={3}>
          <Card sx={{ bgcolor: '#1a1a2e' }}>
            <CardContent>
              <Typography variant="h4" sx={{ color: '#9945FF' }}>{stats.nonEvmCount}</Typography>
              <Typography variant="body2" sx={{ color: 'gray' }}>Non-EVM Chains</Typography>
            </CardContent>
          </Card>
        </Grid>
        <Grid item xs={12} sm={6} md={3}>
          <Card sx={{ bgcolor: '#1a1a2e' }}>
            <CardContent>
              <Typography variant="h4" sx={{ color: '#00d4aa' }}>
                {Object.keys(stats.byCategory || {}).length}
              </Typography>
              <Typography variant="body2" sx={{ color: 'gray' }}>Categories</Typography>
            </CardContent>
          </Card>
        </Grid>
      </Grid>

      {/* Filters */}
      <Card sx={{ mb: 3, bgcolor: '#1a1a2e' }}>
        <CardContent>
          <Grid container spacing={2} alignItems="center">
            <Grid item xs={12} md={4}>
              <TextField fullWidth size="small" placeholder="Search chains..." value={searchQuery} onChange={(e) => setSearchQuery(e.target.value)} InputProps={{
            startAdornment: <Search sx={{ color: 'gray', mr: 1 }}/>
        }} sx={{ '& input': { color: 'white' } }}/>
            </Grid>
            <Grid item xs={6} md={3}>
              <FormControl fullWidth size="small">
                <InputLabel sx={{ color: 'gray' }}>Category</InputLabel>
                <Select value={categoryFilter} label="Category" onChange={(e) => setCategoryFilter(e.target.value)} sx={{ color: 'white' }}>
                  <MenuItem value="all">All Categories</MenuItem>
                  {CHAIN_CATEGORIES.map(cat => (<MenuItem key={cat.value} value={cat.value}>{cat.label}</MenuItem>))}
                </Select>
              </FormControl>
            </Grid>
            <Grid item xs={6} md={3}>
              <FormControl fullWidth size="small">
                <InputLabel sx={{ color: 'gray' }}>Status</InputLabel>
                <Select value={statusFilter} label="Status" onChange={(e) => setStatusFilter(e.target.value)} sx={{ color: 'white' }}>
                  <MenuItem value="all">All Status</MenuItem>
                  {CHAIN_STATUSES.map(s => (<MenuItem key={s.value} value={s.value}>{s.label}</MenuItem>))}
                </Select>
              </FormControl>
            </Grid>
            <Grid item xs={12} md={2}>
              <Button fullWidth variant="outlined" startIcon={<Refresh />} onClick={loadChains} sx={{ borderColor: '#3a3a4e', color: 'white' }}>
                Refresh
              </Button>
            </Grid>
          </Grid>
        </CardContent>
      </Card>

      {/* Chains Table */}
      <Card sx={{ bgcolor: '#1a1a2e' }}>
        <TableContainer>
          <Table>
            <TableHead>
              <TableRow>
                <TableCell sx={{ color: 'gray' }}>Chain</TableCell>
                <TableCell sx={{ color: 'gray' }}>Category</TableCell>
                <TableCell sx={{ color: 'gray' }}>Chain ID</TableCell>
                <TableCell sx={{ color: 'gray' }}>Status</TableCell>
                <TableCell sx={{ color: 'gray' }}>RPCs</TableCell>
                <TableCell sx={{ color: 'gray' }}>Features</TableCell>
                <TableCell sx={{ color: 'gray' }}>Actions</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {filteredChains.map(chain => (<TableRow key={chain.id} hover>
                  <TableCell>
                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                      <Box sx={{
                width: 32, height: 32, borderRadius: '50%',
                bgcolor: getCategoryColor(chain.category),
                display: 'flex', alignItems: 'center', justifyContent: 'center',
                fontSize: '0.8rem', fontWeight: 'bold', color: 'white'
            }}>
                        {chain.symbol.slice(0, 2)}
                      </Box>
                      <Box>
                        <Typography sx={{ color: 'white', fontWeight: 'bold' }}>{chain.name}</Typography>
                        <Typography sx={{ color: 'gray', fontSize: '0.75rem' }}>{chain.id}</Typography>
                      </Box>
                    </Box>
                  </TableCell>
                  <TableCell>
                    <Chip label={chain.category.toUpperCase()} size="small" sx={{
                bgcolor: getCategoryColor(chain.category) + '33',
                color: getCategoryColor(chain.category),
                fontWeight: 'bold'
            }}/>
                  </TableCell>
                  <TableCell>
                    <Typography sx={{ color: 'white', fontFamily: 'monospace' }}>
                      {chain.chainId}
                    </Typography>
                  </TableCell>
                  <TableCell>
                    <Chip label={chain.status} size="small" color={getStatusColor(chain.status)}/>
                  </TableCell>
                  <TableCell>
                    <Typography sx={{ color: 'gray', fontSize: '0.85rem' }}>
                      {chain.rpcUrls.length} RPC{chain.rpcUrls.length !== 1 ? 's' : ''}
                    </Typography>
                  </TableCell>
                  <TableCell>
                    <Box sx={{ display: 'flex', gap: 0.5, flexWrap: 'wrap' }}>
                      {chain.supportsEIP1559 && <Chip label="EIP1559" size="small" sx={{ fontSize: '0.6rem', height: 20 }}/>}
                      {chain.supportsMulticall && <Chip label="Multicall" size="small" sx={{ fontSize: '0.6rem', height: 20 }}/>}
                      {chain.supportsMEV && <Chip label="MEV" size="small" sx={{ fontSize: '0.6rem', height: 20 }}/>}
                    </Box>
                  </TableCell>
                  <TableCell>
                    <IconButton size="small" onClick={() => handleViewChain(chain)} sx={{ color: '#00d4aa' }}>
                      <Info />
                    </IconButton>
                    <IconButton size="small" onClick={() => handleEditChain(chain)} sx={{ color: 'white' }}>
                      <Edit />
                    </IconButton>
                    <IconButton size="small" onClick={() => handleDeleteChain(chain)} sx={{ color: 'error.main' }}>
                      <Delete />
                    </IconButton>
                  </TableCell>
                </TableRow>))}
              {filteredChains.length === 0 && (<TableRow>
                  <TableCell colSpan={7} align="center" sx={{ py: 4, color: 'gray' }}>
                    No chains found
                  </TableCell>
                </TableRow>)}
            </TableBody>
          </Table>
        </TableContainer>
      </Card>

      {/* Add/Edit Chain Dialog */}
      <Dialog open={chainDialogOpen} onClose={() => setChainDialogOpen(false)} maxWidth="md" fullWidth>
        <DialogTitle sx={{ bgcolor: '#1a1a2e', color: 'white' }}>
          {selectedChain ? 'Edit Chain' : 'Add New Chain'}
        </DialogTitle>
        <DialogContent sx={{ bgcolor: '#1a1a2e', p: 2 }}>
          <Grid container spacing={2}>
            <Grid item xs={12} md={6}>
              <TextField fullWidth size="small" label="Chain ID" value={chainForm.id} onChange={(e) => setChainForm({ ...chainForm, id: e.target.value })} disabled={!!selectedChain} sx={{ '& input': { color: 'white' } }}/>
            </Grid>
            <Grid item xs={12} md={6}>
              <TextField fullWidth size="small" label="Name" value={chainForm.name} onChange={(e) => setChainForm({ ...chainForm, name: e.target.value })} sx={{ '& input': { color: 'white' } }}/>
            </Grid>
            <Grid item xs={12} md={4}>
              <TextField fullWidth size="small" label="Symbol" value={chainForm.symbol} onChange={(e) => setChainForm({ ...chainForm, symbol: e.target.value })} sx={{ '& input': { color: 'white' } }}/>
            </Grid>
            <Grid item xs={12} md={4}>
              <FormControl fullWidth size="small">
                <InputLabel sx={{ color: 'gray' }}>Category</InputLabel>
                <Select value={chainForm.category} label="Category" onChange={(e) => setChainForm({ ...chainForm, category: e.target.value })} sx={{ color: 'white' }}>
                  {CHAIN_CATEGORIES.map(cat => (<MenuItem key={cat.value} value={cat.value}>{cat.label}</MenuItem>))}
                </Select>
              </FormControl>
            </Grid>
            <Grid item xs={12} md={4}>
              <FormControl fullWidth size="small">
                <InputLabel sx={{ color: 'gray' }}>Status</InputLabel>
                <Select value={chainForm.status} label="Status" onChange={(e) => setChainForm({ ...chainForm, status: e.target.value })} sx={{ color: 'white' }}>
                  {CHAIN_STATUSES.map(s => (<MenuItem key={s.value} value={s.value}>{s.label}</MenuItem>))}
                </Select>
              </FormControl>
            </Grid>
            <Grid item xs={12}>
              <TextField fullWidth size="small" label="Chain ID (Number)" type="number" value={chainForm.chainId} onChange={(e) => setChainForm({ ...chainForm, chainId: parseInt(e.target.value) || 0 })} sx={{ '& input': { color: 'white' } }}/>
            </Grid>
            <Grid item xs={12}>
              <TextField fullWidth multiline rows={2} size="small" label="RPC URLs (one per line)" value={chainForm.rpcUrls} onChange={(e) => setChainForm({ ...chainForm, rpcUrls: e.target.value })} sx={{ '& input': { color: 'white' } }}/>
            </Grid>
            <Grid item xs={12}>
              <TextField fullWidth multiline rows={2} size="small" label="Explorer URLs (one per line)" value={chainForm.explorerUrls} onChange={(e) => setChainForm({ ...chainForm, explorerUrls: e.target.value })} sx={{ '& input': { color: 'white' } }}/>
            </Grid>
            <Grid item xs={12} md={4}>
              <TextField fullWidth size="small" label="Native Currency Name" value={chainForm.nativeCurrencyName} onChange={(e) => setChainForm({ ...chainForm, nativeCurrencyName: e.target.value })} sx={{ '& input': { color: 'white' } }}/>
            </Grid>
            <Grid item xs={12} md={4}>
              <TextField fullWidth size="small" label="Native Currency Symbol" value={chainForm.nativeCurrencySymbol} onChange={(e) => setChainForm({ ...chainForm, nativeCurrencySymbol: e.target.value })} sx={{ '& input': { color: 'white' } }}/>
            </Grid>
            <Grid item xs={12} md={4}>
              <TextField fullWidth size="small" label="Native Currency Decimals" type="number" value={chainForm.nativeCurrencyDecimals} onChange={(e) => setChainForm({ ...chainForm, nativeCurrencyDecimals: parseInt(e.target.value) || 18 })} sx={{ '& input': { color: 'white' } }}/>
            </Grid>
            <Grid item xs={6} md={3}>
              <TextField fullWidth size="small" label="Block Time (sec)" type="number" value={chainForm.blockTime || ''} onChange={(e) => setChainForm({ ...chainForm, blockTime: parseInt(e.target.value) || undefined })} sx={{ '& input': { color: 'white' } }}/>
            </Grid>
            <Grid item xs={6} md={3}>
              <TextField fullWidth size="small" label="Gas Limit" type="number" value={chainForm.gasLimit || ''} onChange={(e) => setChainForm({ ...chainForm, gasLimit: parseInt(e.target.value) || undefined })} sx={{ '& input': { color: 'white' } }}/>
            </Grid>
            <Grid item xs={12}>
              <Typography sx={{ color: 'gray', mb: 1 }}>Features</Typography>
              <Box sx={{ display: 'flex', gap: 2, flexWrap: 'wrap' }}>
                <FormControlLabel control={<Switch checked={chainForm.supportsEIP1559} onChange={(e) => setChainForm({ ...chainForm, supportsEIP1559: e.target.checked })}/>} label="EIP-1559" sx={{ color: 'white' }}/>
                <FormControlLabel control={<Switch checked={chainForm.supportsMulticall} onChange={(e) => setChainForm({ ...chainForm, supportsMulticall: e.target.checked })}/>} label="Multicall" sx={{ color: 'white' }}/>
                <FormControlLabel control={<Switch checked={chainForm.supportsFlashbots} onChange={(e) => setChainForm({ ...chainForm, supportsFlashbots: e.target.checked })}/>} label="Flashbots" sx={{ color: 'white' }}/>
                <FormControlLabel control={<Switch checked={chainForm.supportsMEV} onChange={(e) => setChainForm({ ...chainForm, supportsMEV: e.target.checked })}/>} label="MEV" sx={{ color: 'white' }}/>
              </Box>
            </Grid>
            <Grid item xs={12}>
              <TextField fullWidth multiline rows={2} size="small" label="Notes" value={chainForm.notes || ''} onChange={(e) => setChainForm({ ...chainForm, notes: e.target.value })} sx={{ '& input': { color: 'white' } }}/>
            </Grid>
          </Grid>
        </DialogContent>
        <DialogActions sx={{ bgcolor: '#1a1a2e' }}>
          <Button onClick={() => setChainDialogOpen(false)} sx={{ color: 'gray' }}>Cancel</Button>
          <Button onClick={handleSaveChain} variant="contained" sx={{ bgcolor: '#00d4aa', color: 'black' }}>
            {selectedChain ? 'Update' : 'Add'} Chain
          </Button>
        </DialogActions>
      </Dialog>

      {/* View Chain Dialog */}
      <Dialog open={viewDialogOpen} onClose={() => setViewDialogOpen(false)} maxWidth="md" fullWidth>
        <DialogTitle sx={{ bgcolor: '#1a1a2e', color: 'white' }}>
          Chain Details
        </DialogTitle>
        <DialogContent sx={{ bgcolor: '#1a1a2e', p: 2 }}>
          {selectedChain && (<Grid container spacing={2}>
              <Grid item xs={12}><Divider sx={{ borderColor: '#3a3a4e' }}/></Grid>
              <Grid item xs={6}><Typography sx={{ color: 'gray' }}>Name</Typography></Grid>
              <Grid item xs={6}><Typography sx={{ color: 'white' }}>{selectedChain.name}</Typography></Grid>
              <Grid item xs={6}><Typography sx={{ color: 'gray' }}>Chain ID</Typography></Grid>
              <Grid item xs={6}><Typography sx={{ color: 'white', fontFamily: 'monospace' }}>{selectedChain.chainId}</Typography></Grid>
              <Grid item xs={6}><Typography sx={{ color: 'gray' }}>Category</Typography></Grid>
              <Grid item xs={6}><Chip label={selectedChain.category} size="small" sx={{ bgcolor: getCategoryColor(selectedChain.category) + '33', color: getCategoryColor(selectedChain.category) }}/></Grid>
              <Grid item xs={6}><Typography sx={{ color: 'gray' }}>Status</Typography></Grid>
              <Grid item xs={6}><Chip label={selectedChain.status} size="small" color={getStatusColor(selectedChain.status)}/></Grid>
              <Grid item xs={6}><Typography sx={{ color: 'gray' }}>RPC URLs</Typography></Grid>
              <Grid item xs={6}>
                <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.5 }}>
                  {selectedChain.rpcUrls.map((url, i) => (<Typography key={i} sx={{ color: '#00d4aa', fontSize: '0.8rem', wordBreak: 'break-all' }}>{url}</Typography>))}
                </Box>
              </Grid>
              <Grid item xs={6}><Typography sx={{ color: 'gray' }}>Explorer URLs</Typography></Grid>
              <Grid item xs={6}>
                <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.5 }}>
                  {selectedChain.explorerUrls.map((url, i) => (<Typography key={i} sx={{ color: '#00d4aa', fontSize: '0.8rem', wordBreak: 'break-all' }}>{url}</Typography>))}
                </Box>
              </Grid>
              <Grid item xs={6}><Typography sx={{ color: 'gray' }}>Native Currency</Typography></Grid>
              <Grid item xs={6}><Typography sx={{ color: 'white' }}>{selectedChain.nativeCurrency.name} ({selectedChain.nativeCurrency.symbol})</Typography></Grid>
              <Grid item xs={6}><Typography sx={{ color: 'gray' }}>Added</Typography></Grid>
              <Grid item xs={6}><Typography sx={{ color: 'white' }}>{formatDate(selectedChain.addedAt)}</Typography></Grid>
            </Grid>)}
        </DialogContent>
        <DialogActions sx={{ bgcolor: '#1a1a2e' }}>
          <Button onClick={() => setViewDialogOpen(false)} sx={{ color: 'gray' }}>Close</Button>
        </DialogActions>
      </Dialog>

      {/* Delete Confirmation Dialog */}
      <Dialog open={deleteDialogOpen} onClose={() => setDeleteDialogOpen(false)}>
        <DialogTitle sx={{ bgcolor: '#1a1a2e', color: 'error.main' }}>
          Confirm Delete
        </DialogTitle>
        <DialogContent sx={{ bgcolor: '#1a1a2e' }}>
          <Typography sx={{ color: 'white' }}>
            Are you sure you want to delete chain "{selectedChain?.name}"? This action cannot be undone.
          </Typography>
        </DialogContent>
        <DialogActions sx={{ bgcolor: '#1a1a2e' }}>
          <Button onClick={() => setDeleteDialogOpen(false)} sx={{ color: 'gray' }}>Cancel</Button>
          <Button onClick={confirmDeleteChain} sx={{ bgcolor: 'error.main', color: 'white' }}>Delete</Button>
        </DialogActions>
      </Dialog>

      {/* Snackbar */}
      <Snackbar open={snackbar.open} autoHideDuration={5000} onClose={() => setSnackbar({ ...snackbar, open: false })} anchorOrigin={{ vertical: 'bottom', horizontal: 'center' }}>
        <Alert severity={snackbar.severity} sx={{ bgcolor: snackbar.severity === 'success' ? '#1b5e20' : snackbar.severity === 'error' ? '#b71c1c' : '#1a237e' }}>
          {snackbar.message}
        </Alert>
      </Snackbar>
    </Box>);
}
//# sourceMappingURL=page.jsx.map