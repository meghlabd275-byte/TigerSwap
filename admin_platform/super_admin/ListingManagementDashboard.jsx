// TigerSwap - Listing Management Admin Dashboard
// Complete UI for trading pairs, pools, and token listings
import React, { useState, useEffect } from 'react';
import { Box, Typography, Card, CardContent, Button, Table, TableBody, TableCell, TableContainer, TableHead, TableRow, TextField, Select, MenuItem, Dialog, DialogTitle, DialogContent, DialogActions, Tabs, Tab, Chip, IconButton, Switch, FormControl, InputLabel, Grid, Avatar } from '@mui/material';
import { Add, Edit, Delete, TrendingUp, TrendingDown, Refresh, Search, Pool, Pending, ShowChart, AttachMoney } from '@mui/icons-material';
export const ListingManagementDashboard = () => {
    const [tabIndex, setTabIndex] = useState(0);
    const [pairs, setPairs] = useState([]);
    const [applications, setApplications] = useState([]);
    const [searchTerm, setSearchTerm] = useState('');
    const [filterStatus, setFilterStatus] = useState('all');
    const [showCreatePair, setShowCreatePair] = useState(false);
    const [showEditFees, setShowEditFees] = useState(false);
    const [selectedPair, setSelectedPair] = useState(null);
    const [loading, setLoading] = useState(false);
    useEffect(() => {
        loadData();
    }, []);
    const loadData = () => {
        // Mock data
        setPairs([
            { id: '1', pairName: 'ETH/USDT', baseToken: { symbol: 'ETH', name: 'Ethereum', logo: 'eth.png' }, quoteToken: { symbol: 'USDT', name: 'Tether USD', logo: 'usdt.png' }, chainId: 1, status: 'active', price: '2450.50', priceChange24h: 2.5, volume24h: '$125M', liquidity: '$50M', tradingFee: '0.25', listingFee: '1000', isFeatured: true, tier: 'tier1' },
            { id: '2', pairName: 'BTC/USDT', baseToken: { symbol: 'WBTC', name: 'Wrapped Bitcoin', logo: 'wbtc.png' }, quoteToken: { symbol: 'USDT', name: 'Tether USD', logo: 'usdt.png' }, chainId: 1, status: 'active', price: '62500', priceChange24h: 1.2, volume24h: '$250M', liquidity: '$100M', tradingFee: '0.25', listingFee: '2000', isFeatured: true, tier: 'tier1' },
            { id: '3', pairName: 'BNB/USDT', baseToken: { symbol: 'BNB', name: 'BNB', logo: 'bnb.png' }, quoteToken: { symbol: 'USDT', name: 'Tether USD', logo: 'usdt.png' }, chainId: 56, status: 'active', price: '310.25', priceChange24h: 3.1, volume24h: '$80M', liquidity: '$35M', tradingFee: '0.25', listingFee: '800', isFeatured: true, tier: 'tier1' },
            { id: '4', pairName: 'MATIC/USDT', baseToken: { symbol: 'MATIC', name: 'Polygon', logo: 'matic.png' }, quoteToken: { symbol: 'USDT', name: 'Tether USD', logo: 'usdt.png' }, chainId: 137, status: 'active', price: '0.85', priceChange24h: -1.5, volume24h: '$45M', liquidity: '$18M', tradingFee: '0.30', listingFee: '500', isFeatured: false, tier: 'tier2' },
            { id: '5', pairName: 'ARB/USDT', baseToken: { symbol: 'ARB', name: 'Arbitrum', logo: 'arb.png' }, quoteToken: { symbol: 'USDT', name: 'Tether USD', logo: 'usdt.png' }, chainId: 42161, status: 'active', price: '1.20', priceChange24h: 5.2, volume24h: '$35M', liquidity: '$12M', tradingFee: '0.30', listingFee: '600', isFeatured: true, tier: 'tier2' },
            { id: '6', pairName: 'TIGER/USDT', baseToken: { symbol: 'TIGER', name: 'TigerSwap', logo: 'tiger.png' }, quoteToken: { symbol: 'USDT', name: 'Tether USD', logo: 'usdt.png' }, chainId: 1, status: 'active', price: '2.50', priceChange24h: 10.5, volume24h: '$15M', liquidity: '$5M', tradingFee: '0.25', listingFee: '1000', isFeatured: true, tier: 'tier1' },
            { id: '7', pairName: 'DOGE/USDT', baseToken: { symbol: 'DOGE', name: 'Dogecoin', logo: 'doge.png' }, quoteToken: { symbol: 'USDT', name: 'Tether USD', logo: 'usdt.png' }, chainId: 1, status: 'pending', price: '0.12', priceChange24h: 0, volume24h: '$0', liquidity: '$0', tradingFee: '0.30', listingFee: '700', isFeatured: false, tier: 'tier3' },
            { id: '8', pairName: 'SHIB/USDT', baseToken: { symbol: 'SHIB', name: 'Shiba Inu', logo: 'shib.png' }, quoteToken: { symbol: 'USDT', name: 'Tether USD', logo: 'usdt.png' }, chainId: 1, status: 'delisted', price: '0.0000089', priceChange24h: -2.3, volume24h: '$20M', liquidity: '$8M', tradingFee: '0.30', listingFee: '600', isFeatured: false, tier: 'tier2' },
        ]);
        setApplications([
            { id: 'a1', token: { symbol: 'NEW1', name: 'New Token 1', address: '0x1234...' }, applicant: '0x5678...', status: 'submitted', tier: 'tier3', listingFee: '500', submittedAt: Date.now() - 86400000 },
            { id: 'a2', token: { symbol: 'NEW2', name: 'New Token 2', address: '0xabcd...' }, applicant: '0xdef0...', status: 'fee_paid', tier: 'tier3', listingFee: '500', submittedAt: Date.now() - 43200000 },
            { id: 'a3', token: { symbol: 'PEPE', name: 'Pepe Token', address: '0x9876...' }, applicant: '0x1234...', status: 'approved', tier: 'tier3', listingFee: '600', submittedAt: Date.now() - 172800000 },
        ]);
    };
    const getStatusColor = (status) => {
        switch (status) {
            case 'active': return 'success';
            case 'pending': return 'warning';
            case 'delisted': return 'error';
            case 'suspended': return 'error';
            default: return 'default';
        }
    };
    const getTierColor = (tier) => {
        switch (tier) {
            case 'tier1': return '#f97316';
            case 'tier2': return '#3b82f6';
            case 'tier3': return '#8b5cf6';
            case 'tier4': return '#6b7280';
            default: return '#6b7280';
        }
    };
    const filteredPairs = pairs.filter(pair => {
        const matchesSearch = pair.pairName.toLowerCase().includes(searchTerm.toLowerCase()) ||
            pair.baseToken.symbol.toLowerCase().includes(searchTerm.toLowerCase());
        const matchesStatus = filterStatus === 'all' || pair.status === filterStatus;
        return matchesSearch && matchesStatus;
    });
    const handleDelist = (pairId) => {
        if (confirm('Are you sure you want to delist this trading pair?')) {
            setPairs(pairs.map(p => p.id === pairId ? { ...p, status: 'delisted' } : p));
        }
    };
    const handleRelist = (pairId) => {
        setPairs(pairs.map(p => p.id === pairId ? { ...p, status: 'active' } : p));
    };
    const handleToggleFeatured = (pairId) => {
        setPairs(pairs.map(p => p.id === pairId ? { ...p, isFeatured: !p.isFeatured } : p));
    };
    return (<Box sx={{ p: 3 }}>
      {/* Header */}
      <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 4 }}>
        <Box>
          <Typography variant="h4" gutterBottom>Trading Pair Management</Typography>
          <Typography variant="body2" color="text.secondary">
            Manage listings, pools, fees, and token applications
          </Typography>
        </Box>
        <Button variant="contained" startIcon={<Add />} onClick={() => setShowCreatePair(true)}>
          Create New Pair
        </Button>
      </Box>

      {/* Stats Cards */}
      <Grid container spacing={3} sx={{ mb: 4 }}>
        <Grid item xs={12} sm={6} md={3}>
          <Card>
            <CardContent sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
              <ShowChart sx={{ fontSize: 40, color: '#f97316' }}/>
              <Box>
                <Typography variant="h4">{pairs.filter(p => p.status === 'active').length}</Typography>
                <Typography variant="body2" color="text.secondary">Active Pairs</Typography>
              </Box>
            </CardContent>
          </Card>
        </Grid>
        <Grid item xs={12} sm={6} md={3}>
          <Card>
            <CardContent sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
              <Pool sx={{ fontSize: 40, color: '#3b82f6' }}/>
              <Box>
                <Typography variant="h4">{pairs.length}</Typography>
                <Typography variant="body2" color="text.secondary">Total Pools</Typography>
              </Box>
            </CardContent>
          </Card>
        </Grid>
        <Grid item xs={12} sm={6} md={3}>
          <Card>
            <CardContent sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
              <Pending sx={{ fontSize: 40, color: '#eab308' }}/>
              <Box>
                <Typography variant="h4">{applications.filter(a => a.status === 'submitted').length}</Typography>
                <Typography variant="body2" color="text.secondary">Pending Applications</Typography>
              </Box>
            </CardContent>
          </Card>
        </Grid>
        <Grid item xs={12} sm={6} md={3}>
          <Card>
            <CardContent sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
              <AttachMoney sx={{ fontSize: 40, color: '#10b981' }}/>
              <Box>
                <Typography variant="h4">$1.2M</Typography>
                <Typography variant="body2" color="text.secondary">Total Fees Collected</Typography>
              </Box>
            </CardContent>
          </Card>
        </Grid>
      </Grid>

      {/* Tabs */}
      <Tabs value={tabIndex} onChange={(_, v) => setTabIndex(v)} sx={{ mb: 3 }}>
        <Tab label="Trading Pairs"/>
        <Tab label={`Applications (${applications.length})`}/>
        <Tab label="Fee Configuration"/>
        <Tab label="Pool Management"/>
      </Tabs>

      {/* Trading Pairs Tab */}
      {tabIndex === 0 && (<>
          <Card sx={{ mb: 3 }}>
            <CardContent>
              <Grid container spacing={2}>
                <Grid item xs={12} md={6}>
                  <TextField fullWidth placeholder="Search pairs..." value={searchTerm} onChange={(e) => setSearchTerm(e.target.value)} InputProps={{ startAdornment: <Search /> }} size="small"/>
                </Grid>
                <Grid item xs={12} md={3}>
                  <FormControl fullWidth size="small">
                    <InputLabel>Status</InputLabel>
                    <Select value={filterStatus} onChange={(e) => setFilterStatus(e.target.value)} label="Status">
                      <MenuItem value="all">All</MenuItem>
                      <MenuItem value="active">Active</MenuItem>
                      <MenuItem value="pending">Pending</MenuItem>
                      <MenuItem value="delisted">Delisted</MenuItem>
                    </Select>
                  </FormControl>
                </Grid>
                <Grid item xs={12} md={3}>
                  <Button variant="outlined" fullWidth onClick={() => setShowEditFees(true)}>
                    Edit Fees
                  </Button>
                </Grid>
              </Grid>
            </CardContent>
          </Card>

          <TableContainer>
            <Table>
              <TableHead>
                <TableRow>
                  <TableCell>Pair</TableCell>
                  <TableCell>Price</TableCell>
                  <TableCell>24h Change</TableCell>
                  <TableCell>Volume</TableCell>
                  <TableCell>Liquidity</TableCell>
                  <TableCell>Fee</TableCell>
                  <TableCell>Tier</TableCell>
                  <TableCell>Status</TableCell>
                  <TableCell>Featured</TableCell>
                  <TableCell>Actions</TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {filteredPairs.map(pair => (<TableRow key={pair.id} hover>
                    <TableCell>
                      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                        <Avatar src={pair.baseToken.logo} sx={{ width: 32, height: 32 }}/>
                        <Box>
                          <Typography fontWeight="bold">{pair.pairName}</Typography>
                          <Typography variant="caption" color="text.secondary">
                            Chain: {pair.chainId}
                          </Typography>
                        </Box>
                      </Box>
                    </TableCell>
                    <TableCell>${pair.price}</TableCell>
                    <TableCell>
                      <Chip icon={pair.priceChange24h >= 0 ? <TrendingUp /> : <TrendingDown />} label={`${pair.priceChange24h >= 0 ? '+' : ''}${pair.priceChange24h}%`} size="small" color={pair.priceChange24h >= 0 ? 'success' : 'error'}/>
                    </TableCell>
                    <TableCell>{pair.volume24h}</TableCell>
                    <TableCell>{pair.liquidity}</TableCell>
                    <TableCell>{pair.tradingFee}%</TableCell>
                    <TableCell>
                      <Chip label={pair.tier.toUpperCase()} size="small" sx={{ bgcolor: getTierColor(pair.tier), color: 'white' }}/>
                    </TableCell>
                    <TableCell>
                      <Chip label={pair.status} size="small" color={getStatusColor(pair.status)}/>
                    </TableCell>
                    <TableCell>
                      <Switch checked={pair.isFeatured} onChange={() => handleToggleFeatured(pair.id)}/>
                    </TableCell>
                    <TableCell>
                      <IconButton size="small"><Edit /></IconButton>
                      {pair.status === 'active' ? (<IconButton size="small" color="error" onClick={() => handleDelist(pair.id)}><Delete /></IconButton>) : (<IconButton size="small" color="success" onClick={() => handleRelist(pair.id)}><Refresh /></IconButton>)}
                    </TableCell>
                  </TableRow>))}
              </TableBody>
            </Table>
          </TableContainer>
        </>)}

      {/* Applications Tab */}
      {tabIndex === 1 && (<TableContainer>
          <Table>
            <TableHead>
              <TableRow>
                <TableCell>Token</TableCell>
                <TableCell>Applicant</TableCell>
                <TableCell>Tier</TableCell>
                <TableCell>Listing Fee</TableCell>
                <TableCell>Submitted</TableCell>
                <TableCell>Status</TableCell>
                <TableCell>Actions</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {applications.map(app => (<TableRow key={app.id} hover>
                  <TableCell>
                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                      <Avatar sx={{ width: 32, height: 32, bgcolor: '#f97316' }}>{app.token.symbol[0]}</Avatar>
                      <Box>
                        <Typography fontWeight="bold">{app.token.symbol}</Typography>
                        <Typography variant="caption" color="text.secondary">{app.token.name}</Typography>
                      </Box>
                    </Box>
                  </TableCell>
                  <TableCell>
                    <Chip label={app.applicant.slice(0, 6) + '...' + app.applicant.slice(-4)} size="small"/>
                  </TableCell>
                  <TableCell>
                    <Chip label={app.tier.toUpperCase()} size="small" sx={{ bgcolor: getTierColor(app.tier), color: 'white' }}/>
                  </TableCell>
                  <TableCell>{app.listingFee} TIGER</TableCell>
                  <TableCell>{new Date(app.submittedAt).toLocaleDateString()}</TableCell>
                  <TableCell>
                    <Chip label={app.status.replace('_', ' ')} size="small" color={app.status === 'approved' ? 'success' : app.status === 'rejected' ? 'error' : 'warning'}/>
                  </TableCell>
                  <TableCell>
                    <Button size="small" variant="outlined">Review</Button>
                  </TableCell>
                </TableRow>))}
            </TableBody>
          </Table>
        </TableContainer>)}

      {/* Fee Configuration Tab */}
      {tabIndex === 2 && (<Grid container spacing={3}>
          <Grid item xs={12} md={6}>
            <Card>
              <CardContent>
                <Typography variant="h6" gutterBottom>Listing Fees</Typography>
                <TextField label="Listing Fee (TIGER)" defaultValue="1000" fullWidth sx={{ mb: 2 }}/>
                <TextField label="Listing Fee (USD)" defaultValue="500" fullWidth sx={{ mb: 2 }}/>
                <TextField label="Stable Pair Discount" defaultValue="50" fullWidth sx={{ mb: 2 }}/>
                <Button variant="contained">Save Changes</Button>
              </CardContent>
            </Card>
          </Grid>
          <Grid item xs={12} md={6}>
            <Card>
              <CardContent>
                <Typography variant="h6" gutterBottom>Trading Fees</Typography>
                <TextField label="Trading Fee (%)" defaultValue="0.25" fullWidth sx={{ mb: 2 }}/>
                <TextField label="Maker Fee (%)" defaultValue="0.20" fullWidth sx={{ mb: 2 }}/>
                <TextField label="Taker Fee (%)" defaultValue="0.30" fullWidth sx={{ mb: 2 }}/>
                <TextField label="LP Reward Fee (%)" defaultValue="0.02" fullWidth sx={{ mb: 2 }}/>
                <Button variant="contained">Save Changes</Button>
              </CardContent>
            </Card>
          </Grid>
        </Grid>)}

      {/* Pool Management Tab */}
      {tabIndex === 3 && (<Box>
          <Typography variant="h6" gutterBottom>Active Pools</Typography>
          <Grid container spacing={3}>
            {filteredPairs.filter(p => p.status === 'active').map(pair => (<Grid item xs={12} md={4} key={pair.id}>
                <Card>
                  <CardContent>
                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 2 }}>
                      <Avatar>{pair.baseToken.symbol[0]}</Avatar>
                      <Typography variant="h6">{pair.pairName}</Typography>
                    </Box>
                    <Box sx={{ display: 'flex', justifyContent: 'space-between', mb: 1 }}>
                      <Typography variant="body2">Liquidity</Typography>
                      <Typography variant="body2" fontWeight="bold">{pair.liquidity}</Typography>
                    </Box>
                    <Box sx={{ display: 'flex', justifyContent: 'space-between', mb: 1 }}>
                      <Typography variant="body2">Volume 24h</Typography>
                      <Typography variant="body2" fontWeight="bold">{pair.volume24h}</Typography>
                    </Box>
                    <Box sx={{ display: 'flex', justifyContent: 'space-between', mb: 2 }}>
                      <Typography variant="body2">Fee</Typography>
                      <Typography variant="body2" fontWeight="bold">{pair.tradingFee}%</Typography>
                    </Box>
                    <Box sx={{ display: 'flex', gap: 1 }}>
                      <Button size="small" variant="outlined">Add Liquidity</Button>
                      <Button size="small" variant="outlined">Remove Liquidity</Button>
                    </Box>
                  </CardContent>
                </Card>
              </Grid>))}
          </Grid>
        </Box>)}

      {/* Create Pair Dialog */}
      <Dialog open={showCreatePair} onClose={() => setShowCreatePair(false)} maxWidth="md" fullWidth>
        <DialogTitle>Create New Trading Pair</DialogTitle>
        <DialogContent dividers>
          <Grid container spacing={3}>
            <Grid item xs={12} md={6}>
              <Typography variant="subtitle2" gutterBottom>Base Token</Typography>
              <TextField label="Token Address" fullWidth sx={{ mb: 2 }}/>
              <TextField label="Token Symbol" fullWidth sx={{ mb: 2 }}/>
              <TextField label="Token Name" fullWidth/>
            </Grid>
            <Grid item xs={12} md={6}>
              <Typography variant="subtitle2" gutterBottom>Quote Token</Typography>
              <FormControl fullWidth sx={{ mb: 2 }}>
                <InputLabel>Quote Token</InputLabel>
                <Select label="Quote Token" defaultValue="USDT">
                  <MenuItem value="USDT">USDT</MenuItem>
                  <MenuItem value="USDC">USDC</MenuItem>
                  <MenuItem value="ETH">ETH</MenuItem>
                  <MenuItem value="BNB">BNB</MenuItem>
                </Select>
              </FormControl>
              <TextField label="Initial Liquidity" fullWidth sx={{ mb: 2 }}/>
              <FormControl fullWidth>
                <InputLabel>DEX</InputLabel>
                <Select label="DEX" defaultValue="tigerswap">
                  <MenuItem value="tigerswap">TigerSwap</MenuItem>
                  <MenuItem value="uniswap">Uniswap</MenuItem>
                  <MenuItem value="pancakeswap">PancakeSwap</MenuItem>
                </Select>
              </FormControl>
            </Grid>
            <Grid item xs={12}>
              <Typography variant="subtitle2" gutterBottom>Pair Tier</Typography>
              <Grid container spacing={2}>
                {['tier1', 'tier2', 'tier3', 'tier4'].map(tier => (<Grid item xs={3} key={tier}>
                    <Card sx={{ cursor: 'pointer', border: '2px solid', borderColor: tier === 'tier2' ? '#f97316' : 'transparent' }}>
                      <CardContent sx={{ textAlign: 'center', py: 2 }}>
                        <Typography fontWeight="bold">{tier.toUpperCase()}</Typography>
                        <Typography variant="caption">{tier === 'tier1' ? 'Major Pairs' : tier === 'tier2' ? 'Established' : tier === 'tier3' ? 'New' : 'Community'}</Typography>
                      </CardContent>
                    </Card>
                  </Grid>))}
              </Grid>
            </Grid>
          </Grid>
        </DialogContent>
        <DialogActions sx={{ p: 2 }}>
          <Button onClick={() => setShowCreatePair(false)}>Cancel</Button>
          <Button variant="contained">Create Pair</Button>
        </DialogActions>
      </Dialog>

      {/* Edit Fees Dialog */}
      <Dialog open={showEditFees} onClose={() => setShowEditFees(false)} maxWidth="sm" fullWidth>
        <DialogTitle>Edit Fee Configuration</DialogTitle>
        <DialogContent>
          <TextField label="Trading Fee (%)" defaultValue="0.25" fullWidth sx={{ my: 2 }}/>
          <TextField label="Maker Fee (%)" defaultValue="0.20" fullWidth sx={{ mb: 2 }}/>
          <TextField label="Taker Fee (%)" defaultValue="0.30" fullWidth sx={{ mb: 2 }}/>
          <TextField label="LP Reward Fee (%)" defaultValue="0.02" fullWidth/>
        </DialogContent>
        <DialogActions sx={{ p: 2 }}>
          <Button onClick={() => setShowEditFees(false)}>Cancel</Button>
          <Button variant="contained">Save</Button>
        </DialogActions>
      </Dialog>
    </Box>);
};
export default ListingManagementDashboard;
//# sourceMappingURL=ListingManagementDashboard.jsx.map