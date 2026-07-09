// TigerSwap - Admin Chain Management Dashboard
// Complete UI for managing unlimited EVM and Non-EVM chains
import React, { useState, useEffect } from 'react';
import { Box, Typography, Card, CardContent, Button, Table, TableBody, TableCell, TableContainer, TableHead, TableRow, TextField, Select, MenuItem, Dialog, DialogTitle, DialogContent, DialogActions, Tabs, Tab, Chip, IconButton, Switch, FormControl, InputLabel, Grid, Avatar, LinearProgress, Divider, Tooltip } from '@mui/material';
import { Add, Edit, Delete, Visibility, Hub, CheckCircle, Search, Memory } from '@mui/icons-material';
const AddChainDialog = ({ open, onClose, onAdd }) => {
    const [chainType, setChainType] = useState('evm');
    const [chainData, setChainData] = useState({
        name: '',
        symbol: '',
        decimals: 18,
        chainId: '',
        networkId: '',
        rpc: '',
        wsRpc: '',
        explorer: '',
        explorerApi: '',
        wrappedToken: '',
        currencyName: '',
        description: '',
    });
    const nonEVMTypes = [
        { value: 'solana', label: 'Solana', color: '#9945FF' },
        { value: 'tron', label: 'Tron', color: '#EF0027' },
        { value: 'sui', label: 'Sui', color: '#6F BCEF' },
        { value: 'aptos', label: 'Aptos', color: '#3D2847' },
        { value: 'near', label: 'NEAR', color: '#000000' },
        { value: 'cosmos', label: 'Cosmos', color: '#2E3148' },
        { value: 'osmosis', label: 'Osmosis', color: '#E6007A' },
        { value: 'injective', label: 'Injective', color: '#00B9C6' },
        { value: 'ton', label: 'TON', color: '#0098EA' },
        { value: 'cardano', label: 'Cardano', color: '#0033AD' },
        { value: 'polkadot', label: 'Polkadot', color: '#E6007A' },
        { value: 'algorand', label: 'Algorand', color: '#000000' },
        { value: 'flow', label: 'Flow', color: '#00EF8B' },
        { value: 'hedera', label: 'Hedera', color: '#00EEC4' },
    ];
    const handleSubmit = () => {
        onAdd({
            ...chainData,
            type: chainType === 'evm' ? 'evm' : nonEVMTypes.find(t => t.value)?.value || 'solana',
            isEnabled: true,
            isTestnet: false,
        });
        onClose();
    };
    return (<Dialog open={open} onClose={onClose} maxWidth="md" fullWidth>
      <DialogTitle>
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
          <Add /> Add New Blockchain
        </Box>
      </DialogTitle>
      <DialogContent dividers>
        <Grid container spacing={3}>
          <Grid item xs={12}>
            <Tabs value={chainType} onChange={(_, v) => setChainType(v)}>
              <Tab label="EVM Chain" value="evm"/>
              <Tab label="Non-EVM Chain" value="non-evm"/>
            </Tabs>
          </Grid>

          <Grid item xs={12} md={6}>
            <TextField label="Chain Name" value={chainData.name} onChange={(e) => setChainData({ ...chainData, name: e.target.value })} fullWidth required helperText="e.g., Ethereum, Polygon, Avalanche"/>
          </Grid>

          <Grid item xs={12} md={6}>
            <TextField label="Native Token Symbol" value={chainData.symbol} onChange={(e) => setChainData({ ...chainData, symbol: e.target.value })} fullWidth required helperText="e.g., ETH, MATIC, AVAX"/>
          </Grid>

          {chainType === 'evm' ? (<>
              <Grid item xs={12} md={6}>
                <TextField label="Chain ID (EVM)" type="number" value={chainData.chainId} onChange={(e) => setChainData({ ...chainData, chainId: e.target.value })} fullWidth required helperText="e.g., 1 for Ethereum, 56 for BSC"/>
              </Grid>
              <Grid item xs={12} md={6}>
                <TextField label="Network ID" type="number" value={chainData.networkId} onChange={(e) => setChainData({ ...chainData, networkId: e.target.value })} fullWidth helperText="Chain network ID"/>
              </Grid>
            </>) : (<Grid item xs={12} md={6}>
              <FormControl fullWidth>
                <InputLabel>Chain Type</InputLabel>
                <Select value={chainData.type || 'solana'} onChange={(e) => setChainData({ ...chainData, type: e.target.value })} label="Chain Type">
                  {nonEVMTypes.map(type => (<MenuItem key={type.value} value={type.value}>
                      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                        <Box sx={{ width: 12, height: 12, borderRadius: '50%', bgcolor: type.color }}/>
                        {type.label}
                      </Box>
                    </MenuItem>))}
                </Select>
              </FormControl>
            </Grid>)}

          <Grid item xs={12} md={6}>
            <TextField label="RPC URL" value={chainData.rpc} onChange={(e) => setChainData({ ...chainData, rpc: e.target.value })} fullWidth required placeholder="https://..." helperText="Main RPC endpoint"/>
          </Grid>

          <Grid item xs={12} md={6}>
            <TextField label="WebSocket RPC (Optional)" value={chainData.wsRpc} onChange={(e) => setChainData({ ...chainData, wsRpc: e.target.value })} fullWidth placeholder="wss://..."/>
          </Grid>

          <Grid item xs={12} md={6}>
            <TextField label="Block Explorer URL" value={chainData.explorer} onChange={(e) => setChainData({ ...chainData, explorer: e.target.value })} fullWidth required placeholder="https://..."/>
          </Grid>

          <Grid item xs={12} md={6}>
            <TextField label="Explorer API URL (Optional)" value={chainData.explorerApi} onChange={(e) => setChainData({ ...chainData, explorerApi: e.target.value })} fullWidth placeholder="https://api..."/>
          </Grid>

          <Grid item xs={12} md={6}>
            <TextField label="Wrapped Token Address (for EVM)" value={chainData.wrappedToken} onChange={(e) => setChainData({ ...chainData, wrappedToken: e.target.value })} fullWidth placeholder="0x..."/>
          </Grid>

          <Grid item xs={12} md={6}>
            <TextField label="Token Decimals" type="number" value={chainData.decimals} onChange={(e) => setChainData({ ...chainData, decimals: parseInt(e.target.value) || 18 })} fullWidth helperText="Usually 18 for EVM chains"/>
          </Grid>

          <Grid item xs={12}>
            <TextField label="Description (Optional)" value={chainData.description} onChange={(e) => setChainData({ ...chainData, description: e.target.value })} fullWidth multiline rows={2}/>
          </Grid>
        </Grid>
      </DialogContent>
      <DialogActions sx={{ p: 2 }}>
        <Button onClick={onClose}>Cancel</Button>
        <Button variant="contained" onClick={handleSubmit}>Add Chain</Button>
      </DialogActions>
    </Dialog>);
};
// Chain Cards Component
const ChainCards = ({ chains, onToggle, onEdit, onDelete }) => {
    return (<Grid container spacing={3}>
      {chains.map(chain => (<Grid item xs={12} sm={6} md={4} key={chain.id}>
          <Card sx={{ height: '100%', position: 'relative' }}>
            <CardContent>
              <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', mb: 2 }}>
                <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
                  <Avatar sx={{ bgcolor: chain.type === 'evm' ? '#627EEA' : '#9945FF', width: 48, height: 48 }}>
                    {chain.name[0]}
                  </Avatar>
                  <Box>
                    <Typography variant="h6">{chain.name}</Typography>
                    <Typography variant="body2" color="text.secondary">
                      {chain.type.toUpperCase()} • {chain.symbol}
                    </Typography>
                  </Box>
                </Box>
                <Chip label={chain.isEnabled ? 'Active' : 'Disabled'} size="small" color={chain.isEnabled ? 'success' : 'default'}/>
              </Box>

              <Box sx={{ mb: 2 }}>
                <Typography variant="caption" color="text.secondary">
                  Chain ID: {chain.chainId}
                </Typography>
              </Box>

              {/* Capabilities */}
              <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 0.5, mb: 2 }}>
                {chain.capabilities?.swap && <Chip label="Swap" size="small" sx={{ fontSize: 10 }}/>}
                {chain.capabilities?.bridge && <Chip label="Bridge" size="small" sx={{ fontSize: 10 }}/>}
                {chain.capabilities?.staking && <Chip label="Staking" size="small" sx={{ fontSize: 10 }}/>}
                {chain.capabilities?.nft && <Chip label="NFT" size="small" sx={{ fontSize: 10 }}/>}
              </Box>

              <Divider sx={{ my: 2 }}/>

              <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                <Switch checked={chain.isEnabled} onChange={(e) => onToggle(chain.id, e.target.checked)} color="success"/>
                <Box>
                  <IconButton size="small" onClick={() => onEdit(chain)}>
                    <Edit fontSize="small"/>
                  </IconButton>
                  <IconButton size="small" color="error" onClick={() => onDelete(chain.id)}>
                    <Delete fontSize="small"/>
                  </IconButton>
                </Box>
              </Box>
            </CardContent>
          </Card>
        </Grid>))}
    </Grid>);
};
// Chain Table Component
const ChainTable = ({ chains, onToggle, onEdit, onDelete }) => {
    return (<TableContainer>
      <Table>
        <TableHead>
          <TableRow>
            <TableCell>Chain</TableCell>
            <TableCell>Type</TableCell>
            <TableCell>Chain ID</TableCell>
            <TableCell>Symbol</TableCell>
            <TableCell>RPC Status</TableCell>
            <TableCell>Capabilities</TableCell>
            <TableCell>Status</TableCell>
            <TableCell>Actions</TableCell>
          </TableRow>
        </TableHead>
        <TableBody>
          {chains.map(chain => (<TableRow key={chain.id} hover>
              <TableCell>
                <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
                  <Avatar sx={{ bgcolor: chain.type === 'evm' ? '#627EEA' : '#9945FF' }}>
                    {chain.name[0]}
                  </Avatar>
                  <Box>
                    <Typography fontWeight="bold">{chain.name}</Typography>
                    <Typography variant="caption" color="text.secondary">
                      {chain.explorer ? chain.explorer.slice(0, 30) + '...' : 'No explorer'}
                    </Typography>
                  </Box>
                </Box>
              </TableCell>
              <TableCell>
                <Chip label={chain.type.toUpperCase()} size="small" variant="outlined"/>
              </TableCell>
              <TableCell>{chain.chainId}</TableCell>
              <TableCell>{chain.symbol}</TableCell>
              <TableCell>
                <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                  <CheckCircle color="success" fontSize="small"/>
                  <Typography variant="body2">Connected</Typography>
                </Box>
              </TableCell>
              <TableCell>
                <Box sx={{ display: 'flex', gap: 0.5 }}>
                  {chain.capabilities?.swap && <Tooltip title="Swap"><Chip label="S" size="small"/></Tooltip>}
                  {chain.capabilities?.bridge && <Tooltip title="Bridge"><Chip label="B" size="small"/></Tooltip>}
                  {chain.capabilities?.staking && <Tooltip title="Staking"><Chip label="ST" size="small"/></Tooltip>}
                </Box>
              </TableCell>
              <TableCell>
                <Switch checked={chain.isEnabled} onChange={(e) => onToggle(chain.id, e.target.checked)}/>
              </TableCell>
              <TableCell>
                <IconButton size="small" onClick={() => onEdit(chain)}><Visibility fontSize="small"/></IconButton>
                <IconButton size="small" onClick={() => onEdit(chain)}><Edit fontSize="small"/></IconButton>
                <IconButton size="small" color="error" onClick={() => onDelete(chain.id)}><Delete fontSize="small"/></IconButton>
              </TableCell>
            </TableRow>))}
        </TableBody>
      </Table>
    </TableContainer>);
};
// Main Chain Management Component
export const ChainManagementDashboard = () => {
    const [chains, setChains] = useState([]);
    const [viewMode, setViewMode] = useState('cards');
    const [searchTerm, setSearchTerm] = useState('');
    const [filterType, setFilterType] = useState('all');
    const [showAddDialog, setShowAddDialog] = useState(false);
    const [selectedChain, setSelectedChain] = useState(null);
    const [loading, setLoading] = useState(false);
    // Load chains
    useEffect(() => {
        // Mock data - in production would load from blockchain manager
        setChains([
            // EVM Chains
            { id: '1', chainId: 1, name: 'Ethereum', type: 'evm', symbol: 'ETH', rpc: 'https://eth.llamarpc.com', explorer: 'https://etherscan.io', isEnabled: true, isTestnet: false, capabilities: { swap: true, bridge: true, staking: true, farming: true, nft: true, dappBrowser: true } },
            { id: '56', chainId: 56, name: 'BNB Chain', type: 'evm', symbol: 'BNB', rpc: 'https://bsc.llamarpc.com', explorer: 'https://bscscan.com', isEnabled: true, isTestnet: false, capabilities: { swap: true, bridge: true, staking: true, farming: true, nft: true, dappBrowser: true } },
            { id: '137', chainId: 137, name: 'Polygon', type: 'evm', symbol: 'MATIC', rpc: 'https://polygon.llamarpc.com', explorer: 'https://polygonscan.com', isEnabled: true, isTestnet: false, capabilities: { swap: true, bridge: true, staking: true, farming: true, nft: true, dappBrowser: true } },
            { id: '42161', chainId: 42161, name: 'Arbitrum One', type: 'evm', symbol: 'ETH', rpc: 'https://arbitrum.llamarpc.com', explorer: 'https://arbiscan.io', isEnabled: true, isTestnet: false, capabilities: { swap: true, bridge: true, staking: true, farming: true, nft: true, dappBrowser: true } },
            { id: '10', chainId: 10, name: 'Optimism', type: 'evm', symbol: 'ETH', rpc: 'https://optimism.llamarpc.com', explorer: 'https://optimistic.etherscan.io', isEnabled: true, isTestnet: false, capabilities: { swap: true, bridge: true, staking: true, farming: true, nft: true, dappBrowser: true } },
            { id: '43114', chainId: 43114, name: 'Avalanche C-Chain', type: 'evm', symbol: 'AVAX', rpc: 'https://avax.llamarpc.com', explorer: 'https://snowtrace.io', isEnabled: true, isTestnet: false, capabilities: { swap: true, bridge: true, staking: true, farming: true, nft: true, dappBrowser: true } },
            { id: '8453', chainId: 8453, name: 'Base', type: 'evm', symbol: 'ETH', rpc: 'https://base.llamarpc.com', explorer: 'https://basescan.org', isEnabled: true, isTestnet: false, capabilities: { swap: true, bridge: true, staking: true, farming: true, nft: true, dappBrowser: true } },
            { id: '250', chainId: 250, name: 'Fantom', type: 'evm', symbol: 'FTM', rpc: 'https://fantom.llamarpc.com', explorer: 'https://ftmscan.com', isEnabled: true, isTestnet: false, capabilities: { swap: true, bridge: true, staking: true, farming: true, nft: true, dappBrowser: true } },
            // Non-EVM Chains
            { id: 'solana', chainId: -1, name: 'Solana', type: 'solana', symbol: 'SOL', rpc: 'https://api.mainnet-beta.solana.com', explorer: 'https://solscan.io', isEnabled: true, isTestnet: false, capabilities: { swap: true, bridge: true, staking: true, farming: false, nft: true, dappBrowser: true } },
            { id: 'tron', chainId: -2, name: 'Tron', type: 'tron', symbol: 'TRX', rpc: 'https://api.trongrid.io', explorer: 'https://tronscan.org', isEnabled: true, isTestnet: false, capabilities: { swap: true, bridge: true, staking: true, farming: true, nft: true, dappBrowser: true } },
            { id: 'sui', chainId: -3, name: 'Sui', type: 'sui', symbol: 'SUI', rpc: 'https://fullnode.mainnet.sui.io', explorer: 'https://suiscan.xyz', isEnabled: true, isTestnet: false, capabilities: { swap: true, bridge: true, staking: true, farming: false, nft: true, dappBrowser: true } },
            { id: 'aptos', chainId: -4, name: 'Aptos', type: 'aptos', symbol: 'APT', rpc: 'https://fullnode.aptoslabs.com', explorer: 'https://aptoscan.com', isEnabled: true, isTestnet: false, capabilities: { swap: true, bridge: true, staking: true, farming: false, nft: true, dappBrowser: true } },
            { id: 'near', chainId: -5, name: 'NEAR Protocol', type: 'near', symbol: 'NEAR', rpc: 'https://rpc.mainnet.near.org', explorer: 'https://nearblocks.io', isEnabled: false, isTestnet: false, capabilities: { swap: true, bridge: true, staking: true, farming: false, nft: true, dappBrowser: true } },
            { id: 'cosmos', chainId: -6, name: 'Cosmos Hub', type: 'cosmos', symbol: 'ATOM', rpc: 'https://rpc.cosmos.network', explorer: 'https://mintscan.io/cosmos', isEnabled: false, isTestnet: false, capabilities: { swap: true, bridge: true, staking: true, farming: false, nft: false, dappBrowser: true } },
        ]);
    }, []);
    const filteredChains = chains.filter(chain => {
        const matchesSearch = chain.name.toLowerCase().includes(searchTerm.toLowerCase()) ||
            chain.symbol.toLowerCase().includes(searchTerm.toLowerCase());
        const matchesType = filterType === 'all' || chain.type === filterType;
        return matchesSearch && matchesType;
    });
    const handleToggle = (id, enabled) => {
        setChains(chains.map(c => c.id === id ? { ...c, isEnabled: enabled } : c));
    };
    const handleAddChain = (chainData) => {
        const newChain = {
            id: `chain_${Date.now()}`,
            chainId: chainData.chainId || -Date.now(),
            name: chainData.name,
            type: chainData.type,
            symbol: chainData.symbol,
            rpc: chainData.rpc,
            explorer: chainData.explorer,
            isEnabled: true,
            isTestnet: false,
            capabilities: { swap: true, bridge: true, staking: true, farming: true, nft: true, dappBrowser: true },
        };
        setChains([...chains, newChain]);
    };
    const handleDeleteChain = (id) => {
        if (confirm('Are you sure you want to remove this chain?')) {
            setChains(chains.filter(c => c.id !== id));
        }
    };
    const enabledCount = chains.filter(c => c.isEnabled).length;
    const evmCount = chains.filter(c => c.type === 'evm').length;
    const nonEvmCount = chains.filter(c => c.type !== 'evm').length;
    return (<Box sx={{ p: 3 }}>
      {/* Header */}
      <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 4 }}>
        <Box>
          <Typography variant="h4" gutterBottom>Blockchain Network Management</Typography>
          <Typography variant="body2" color="text.secondary">
            Manage all supported EVM and Non-EVM blockchain networks
          </Typography>
        </Box>
        <Button variant="contained" startIcon={<Add />} onClick={() => setShowAddDialog(true)}>
          Add New Chain
        </Button>
      </Box>

      {/* Stats Cards */}
      <Grid container spacing={3} sx={{ mb: 4 }}>
        <Grid item xs={12} sm={4}>
          <Card>
            <CardContent sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
              <Hub sx={{ fontSize: 40, color: '#f97316' }}/>
              <Box>
                <Typography variant="h4">{chains.length}</Typography>
                <Typography variant="body2" color="text.secondary">Total Chains</Typography>
              </Box>
            </CardContent>
          </Card>
        </Grid>
        <Grid item xs={12} sm={4}>
          <Card>
            <CardContent sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
              <CheckCircle sx={{ fontSize: 40, color: '#10b981' }}/>
              <Box>
                <Typography variant="h4">{enabledCount}</Typography>
                <Typography variant="body2" color="text.secondary">Active Chains</Typography>
              </Box>
            </CardContent>
          </Card>
        </Grid>
        <Grid item xs={12} sm={4}>
          <Card>
            <CardContent sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
                <Memory sx={{ fontSize: 40, color: '#8b5cf6' }}/>
              <Box>
                <Typography variant="h4">{evmCount} / {nonEvmCount}</Typography>
                <Typography variant="body2" color="text.secondary">EVM / Non-EVM</Typography>
              </Box>
            </CardContent>
          </Card>
        </Grid>
      </Grid>

      {/* Filters and Search */}
      <Card sx={{ mb: 3 }}>
        <CardContent>
          <Grid container spacing={2} alignItems="center">
            <Grid item xs={12} md={4}>
              <TextField fullWidth placeholder="Search chains..." value={searchTerm} onChange={(e) => setSearchTerm(e.target.value)} InputProps={{ startAdornment: <Search sx={{ mr: 1 }}/> }} size="small"/>
            </Grid>
            <Grid item xs={12} md={4}>
              <FormControl fullWidth size="small">
                <InputLabel>Filter by Type</InputLabel>
                <Select value={filterType} onChange={(e) => setFilterType(e.target.value)} label="Filter by Type">
                  <MenuItem value="all">All Types</MenuItem>
                  <MenuItem value="evm">EVM Chains</MenuItem>
                  <MenuItem value="solana">Solana</MenuItem>
                  <MenuItem value="tron">Tron</MenuItem>
                  <MenuItem value="sui">Sui</MenuItem>
                  <MenuItem value="aptos">Aptos</MenuItem>
                  <MenuItem value="near">NEAR</MenuItem>
                  <MenuItem value="cosmos">Cosmos</MenuItem>
                </Select>
              </FormControl>
            </Grid>
            <Grid item xs={12} md={4}>
              <Box sx={{ display: 'flex', gap: 1 }}>
                <Chip label="Cards" onClick={() => setViewMode('cards')} variant={viewMode === 'cards' ? 'filled' : 'outlined'}/>
                <Chip label="Table" onClick={() => setViewMode('table')} variant={viewMode === 'table' ? 'filled' : 'outlined'}/>
              </Box>
            </Grid>
          </Grid>
        </CardContent>
      </Card>

      {/* Chain List */}
      {loading ? (<LinearProgress />) : viewMode === 'cards' ? (<ChainCards chains={filteredChains} onToggle={handleToggle} onEdit={setSelectedChain} onDelete={handleDeleteChain}/>) : (<ChainTable chains={filteredChains} onToggle={handleToggle} onEdit={setSelectedChain} onDelete={handleDeleteChain}/>)}

      {/* Add Chain Dialog */}
      <AddChainDialog open={showAddDialog} onClose={() => setShowAddDialog(false)} onAdd={handleAddChain}/>

      {/* Chain Details Dialog */}
      <Dialog open={!!selectedChain} onClose={() => setSelectedChain(null)} maxWidth="md" fullWidth>
        <DialogTitle>
          {selectedChain?.name} - Chain Details
        </DialogTitle>
        <DialogContent>
          {selectedChain && (<Grid container spacing={2}>
              <Grid item xs={12} md={6}>
                <Typography variant="subtitle2">Chain ID</Typography>
                <Typography variant="body1" sx={{ mb: 2 }}>{selectedChain.chainId}</Typography>
              </Grid>
              <Grid item xs={12} md={6}>
                <Typography variant="subtitle2">Type</Typography>
                <Typography variant="body1" sx={{ mb: 2 }}>{selectedChain.type.toUpperCase()}</Typography>
              </Grid>
              <Grid item xs={12} md={6}>
                <Typography variant="subtitle2">Symbol</Typography>
                <Typography variant="body1" sx={{ mb: 2 }}>{selectedChain.symbol}</Typography>
              </Grid>
              <Grid item xs={12} md={6}>
                <Typography variant="subtitle2">Status</Typography>
                <Chip label={selectedChain.isEnabled ? 'Active' : 'Disabled'} color={selectedChain.isEnabled ? 'success' : 'default'}/>
              </Grid>
              <Grid item xs={12}>
                <Typography variant="subtitle2">RPC URL</Typography>
                <Typography variant="body2" sx={{ mb: 2, wordBreak: 'break-all' }}>{selectedChain.rpc}</Typography>
              </Grid>
              <Grid item xs={12}>
                <Typography variant="subtitle2">Explorer URL</Typography>
                <Typography variant="body2" sx={{ wordBreak: 'break-all' }}>{selectedChain.explorer}</Typography>
              </Grid>
            </Grid>)}
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setSelectedChain(null)}>Close</Button>
          <Button variant="contained">Edit Chain</Button>
        </DialogActions>
      </Dialog>
    </Box>);
};
export default ChainManagementDashboard;
//# sourceMappingURL=ChainManagementDashboard.jsx.map