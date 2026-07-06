'use client';
import React, { useState, useEffect, useCallback } from 'react';
import { Box, Typography, Card, CardContent, Button, TextField, Table, TableBody, TableCell, TableContainer, TableHead, TableRow, Chip, IconButton, Select, MenuItem, FormControl, Dialog, DialogTitle, DialogContent, DialogActions, Tabs, Tab, InputAdornment, Tooltip, CircularProgress, Snackbar, Alert, Divider } from '@mui/material';
import { Add, Visibility, Close, Refresh, ContentCopy } from '@mui/icons-material';
// ============================================================================
// Constants
// ============================================================================
const CHAIN_CONFIG = {
    1: { name: 'Ethereum', explorer: 'https://etherscan.io' },
    56: { name: 'BNB Chain', explorer: 'https://bscscan.com' },
    137: { name: 'Polygon', explorer: 'https://polygonscan.com' },
    42161: { name: 'Arbitrum', explorer: 'https://arbiscan.io' },
    10: { name: 'Optimism', explorer: 'https://optimistic.etherscan.io' },
    8453: { name: 'Base', explorer: 'https://basescan.org' },
};
const DEX_INFO = {
    'uniswap_v2': { name: 'Uniswap V2', logo: '🦄', color: '#FF007A' },
    'uniswap_v3': { name: 'Uniswap V3', logo: '🦄', color: '#FF007A' },
    'sushiswap': { name: 'SushiSwap', logo: '🍣', color: '#FA52A0' },
    'pancakeswap': { name: 'PancakeSwap', logo: '🥞', color: '#633001' },
    'quickswap': { name: 'QuickSwap', logo: '⚡', color: '#6c8fc5' },
};
const FEE_TIERS = [
    { value: 0.01, label: '0.01%', description: 'Stable pairs' },
    { value: 0.05, label: '0.05%', description: 'Stable pairs' },
    { value: 0.3, label: '0.30%', description: 'Traditional' },
    { value: 1, label: '1.00%', description: 'Exotic pairs' },
];
// ============================================================================
// Utility Functions
// ============================================================================
function formatAddress(address, chars = 4) {
    return `${address.slice(0, chars + 2)}...${address.slice(-chars)}`;
}
function formatUSD(amount) {
    if (amount >= 1e9)
        return `$${(amount / 1e9).toFixed(2)}B`;
    if (amount >= 1e6)
        return `$${(amount / 1e6).toFixed(2)}M`;
    if (amount >= 1e3)
        return `$${(amount / 1e3).toFixed(2)}K`;
    return new Intl.NumberFormat('en-US', { style: 'currency', currency: 'USD' }).format(amount);
}
function formatNumber(num, decimals = 2) {
    if (num >= 1e9)
        return (num / 1e9).toFixed(decimals) + 'B';
    if (num >= 1e6)
        return (num / 1e6).toFixed(decimals) + 'M';
    if (num >= 1e3)
        return (num / 1e3).toFixed(decimals) + 'K';
    return num.toFixed(decimals);
}
function formatTokenAmount(amount, decimals = 18) {
    if (!amount || amount === '0')
        return '0';
    try {
        const num = Number(amount) / Math.pow(10, decimals);
        if (num < 0.0001)
            return '<0.0001';
        return num.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 6 });
    }
    catch {
        return '0';
    }
}
function formatPercent(value) {
    return `${value >= 0 ? '+' : ''}${value.toFixed(2)}%`;
}
// ============================================================================
// Mock Data Generators
// ============================================================================
const COMMON_TOKENS = {
    1: {
        'ETH': { address: '0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2', symbol: 'ETH', name: 'Ethereum', decimals: 18, priceUSD: 2450, chainId: 1 },
        'WETH': { address: '0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2', symbol: 'WETH', name: 'Wrapped Ether', decimals: 18, priceUSD: 2450, chainId: 1 },
        'USDC': { address: '0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48', symbol: 'USDC', name: 'USD Coin', decimals: 6, priceUSD: 1, chainId: 1 },
        'USDT': { address: '0xdAC17F958D2ee523a2206206994597C13D831ec7', symbol: 'USDT', name: 'Tether', decimals: 6, priceUSD: 1, chainId: 1 },
        'DAI': { address: '0x6B175474E89094C44Da98b954EedeAC495271d0F', symbol: 'DAI', name: 'Dai', decimals: 18, priceUSD: 1, chainId: 1 },
        'WBTC': { address: '0x2260FAC5E5542a773Aa44fBCfeDf7C193bc2C599', symbol: 'WBTC', name: 'Wrapped Bitcoin', decimals: 8, priceUSD: 62500, chainId: 1 },
        'LINK': { address: '0x514910771AF9Ca656af840dff83E8264EcF986CA', symbol: 'LINK', name: 'Chainlink', decimals: 18, priceUSD: 18.5, chainId: 1 },
        'UNI': { address: '0x1f9840a85d5aF5bf1D1762F925BDADdC4201F984', symbol: 'UNI', name: 'Uniswap', decimals: 18, priceUSD: 12.5, chainId: 1 },
    },
    56: {
        'BNB': { address: '0xbb4CdB9CBd36B01bD1cBaEBF2De08d9173bc095c', symbol: 'BNB', name: 'BNB', decimals: 18, priceUSD: 350, chainId: 56 },
        'USDT': { address: '0x55d398326f99059fF775485246999027B3197955', symbol: 'USDT', name: 'Tether', decimals: 18, priceUSD: 1, chainId: 56 },
        'USDC': { address: '0x8AC76a51cc950d9822D68Db83eEAdE4d2B2FC23b', symbol: 'USDC', name: 'USD Coin', decimals: 18, priceUSD: 1, chainId: 56 },
        'CAKE': { address: '0x0E09FaBB73Bd3Ade0a17ECC321fD13a19e81cE82', symbol: 'CAKE', name: 'PancakeSwap', decimals: 18, priceUSD: 2.5, chainId: 56 },
    },
    137: {
        'MATIC': { address: '0x0d500B1d8E8eF31E21C99d1Db9A6444d3ADf1270', symbol: 'MATIC', name: 'Polygon', decimals: 18, priceUSD: 0.85, chainId: 137 },
        'USDC': { address: '0x2791Bca1f2de4661ED88A30C99A7a9449Aa84174', symbol: 'USDC', name: 'USD Coin', decimals: 6, priceUSD: 1, chainId: 137 },
        'USDT': { address: '0xc2132D05D31c914a87C6611C10748AEb04B58e8F', symbol: 'USDT', name: 'Tether', decimals: 18, priceUSD: 1, chainId: 137 },
        'QUICK': { address: '0x831753DD7087CaC61aB5644b308642cc1c33Dc13', symbol: 'QUICK', name: 'QuickSwap', decimals: 18, priceUSD: 0.5, chainId: 137 },
    },
};
function generateMockPools(chainId, count = 20) {
    const pools = [];
    const dexes = Object.entries(DEX_INFO);
    const chainTokens = COMMON_TOKENS[chainId] || COMMON_TOKENS[1];
    const tokenList = Object.values(chainTokens);
    for (let i = 0; i < count; i++) {
        const dexEntry = dexes[Math.floor(Math.random() * dexes.length)];
        const token0Idx = Math.floor(Math.random() * tokenList.length);
        let token1Idx = Math.floor(Math.random() * tokenList.length);
        while (token1Idx === token0Idx) {
            token1Idx = Math.floor(Math.random() * tokenList.length);
        }
        const token0 = tokenList[token0Idx];
        const token1 = tokenList[token1Idx];
        const tvl = Math.random() * 50000000 + 100000;
        const volume24h = Math.random() * 5000000 + 10000;
        pools.push({
            id: `pool-${i}-${chainId}`,
            address: '0x' + Array.from({ length: 40 }, () => '0123456789abcdef'[Math.floor(Math.random() * 16)]).join(''),
            token0,
            token1,
            dex: dexEntry[0],
            dexName: dexEntry[1].name,
            feeTier: FEE_TIERS[Math.floor(Math.random() * FEE_TIERS.length)].value,
            tvlUSD: tvl,
            volume24h,
            volume7d: volume24h * 7 * (0.8 + Math.random() * 0.4),
            apr: Math.random() * 100 + 5,
            token0Reserve: (Math.random() * 10000).toFixed(2),
            token1Reserve: (Math.random() * 10000).toFixed(2),
            liquidity: tvl / (token0.priceUSD + token1.priceUSD),
            price0: token0.priceUSD / token1.priceUSD,
            price1: token1.priceUSD / token0.priceUSD,
        });
    }
    return pools.sort((a, b) => b.tvlUSD - a.tvlUSD);
}
function generateMockPositions(account, pools) {
    const positions = [];
    for (let i = 0; i < Math.min(5, pools.length); i++) {
        const pool = pools[i];
        const token0Amt = Math.random() * 10 + 1;
        const token1Amt = Math.random() * 10 + 1;
        const totalLiquidity = token0Amt * pool.token0.priceUSD + token1Amt * pool.token1.priceUSD;
        positions.push({
            id: `position-${i}`,
            pool,
            token0Amount: token0Amt.toFixed(6),
            token1Amount: token1Amt.toFixed(6),
            liquidityTokenBalance: (totalLiquidity / 100).toFixed(2),
            totalLiquidity,
            poolShare: Math.random() * 5,
            feesEarned0: (Math.random() * 0.5).toFixed(4),
            feesEarned1: (Math.random() * 0.5).toFixed(4),
            rangeLow: pool.feeTier > 0.1 ? 0.8 : undefined,
            rangeHigh: pool.feeTier > 0.1 ? 1.2 : undefined,
            isActive: Math.random() > 0.2,
        });
    }
    return positions;
}
// ============================================================================
// Main Pool Page Component
// ============================================================================
export default function PoolPage() {
    // State
    const [chainId, setChainId] = useState(1);
    const [pools, setPools] = useState([]);
    const [positions, setPositions] = useState([]);
    const [stats, setStats] = useState(null);
    const [loading, setLoading] = useState(true);
    const [selectedPool, setSelectedPool] = useState(null);
    const [walletAddress] = useState(null);
    // UI State
    const [activeTab, setActiveTab] = useState(0);
    const [showCreatePool, setShowCreatePool] = useState(false);
    const [showPoolDetails, setShowPoolDetails] = useState(false);
    const [filterDEX, setFilterDEX] = useState('all');
    const [sortBy, setSortBy] = useState('tvl');
    const [searchQuery, setSearchQuery] = useState('');
    // Create pool form
    const [token0, setToken0] = useState(null);
    const [token1, setToken1] = useState(null);
    const [amount0, setAmount0] = useState('');
    const [amount1, setAmount1] = useState('');
    const [feeTier, setFeeTier] = useState(0.3);
    const [priceRangeLow, setPriceRangeLow] = useState(0);
    const [priceRangeHigh, setPriceRangeHigh] = useState(0);
    const [creating, setCreating] = useState(false);
    // Snackbar
    const [snackbar, setSnackbar] = useState({ open: false, message: '', severity: 'info' });
    // ============================================================================
    // Data Loading
    // ============================================================================
    const loadPools = useCallback(async () => {
        setLoading(true);
        try {
            const mockPools = generateMockPools(chainId, 25);
            setPools(mockPools);
            if (walletAddress) {
                setPositions(generateMockPositions(walletAddress, mockPools));
            }
            const totalTVL = mockPools.reduce((sum, p) => sum + p.tvlUSD, 0);
            const totalVolume24h = mockPools.reduce((sum, p) => sum + p.volume24h, 0);
            const totalVolume7d = mockPools.reduce((sum, p) => sum + p.volume7d, 0);
            const totalFees24h = totalVolume24h * 0.003;
            setStats({
                totalTVL,
                totalVolume24h,
                totalVolume7d,
                totalFees24h,
                totalPools: mockPools.length,
                topPools: mockPools.slice(0, 5),
            });
        }
        catch (error) {
            console.error('Failed to load pools:', error);
        }
        finally {
            setLoading(false);
        }
    }, [chainId, walletAddress]);
    useEffect(() => {
        loadPools();
    }, [loadPools]);
    // ============================================================================
    // Pool Creation
    // ============================================================================
    const handleCreatePool = async () => {
        if (!token0 || !token1 || !amount0 || !amount1) {
            setSnackbar({ open: true, message: 'Please fill in all fields', severity: 'error' });
            return;
        }
        setCreating(true);
        try {
            await new Promise(resolve => setTimeout(resolve, 2000));
            setSnackbar({ open: true, message: 'Pool created successfully!', severity: 'success' });
            setShowCreatePool(false);
            setToken0(null);
            setToken1(null);
            setAmount0('');
            setAmount1('');
            loadPools();
        }
        catch (error) {
            setSnackbar({ open: true, message: 'Failed to create pool', severity: 'error' });
        }
        finally {
            setCreating(false);
        }
    };
    const handleAddLiquidity = async () => {
        if (!selectedPool || !amount0 || !amount1)
            return;
        setCreating(true);
        try {
            await new Promise(resolve => setTimeout(resolve, 2000));
            setSnackbar({ open: true, message: 'Liquidity added successfully!', severity: 'success' });
            setShowPoolDetails(false);
            loadPools();
        }
        catch (error) {
            setSnackbar({ open: true, message: 'Failed to add liquidity', severity: 'error' });
        }
        finally {
            setCreating(false);
        }
    };
    // ============================================================================
    // Filtering & Sorting
    // ============================================================================
    const filteredPools = pools
        .filter(pool => {
        if (filterDEX !== 'all' && pool.dex !== filterDEX)
            return false;
        if (searchQuery) {
            const query = searchQuery.toLowerCase();
            return (pool.token0.symbol.toLowerCase().includes(query) ||
                pool.token1.symbol.toLowerCase().includes(query) ||
                pool.dexName.toLowerCase().includes(query));
        }
        return true;
    })
        .sort((a, b) => {
        switch (sortBy) {
            case 'volume': return b.volume24h - a.volume24h;
            case 'apr': return b.apr - a.apr;
            default: return b.tvlUSD - a.tvlUSD;
        }
    });
    // ============================================================================
    // Render Helpers
    // ============================================================================
    const copyToClipboard = (text) => {
        navigator.clipboard.writeText(text);
        setSnackbar({ open: true, message: 'Copied to clipboard!', severity: 'success' });
    };
    const renderPoolChip = (pool) => (<Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
      <Typography sx={{ color: '#00d4aa', fontWeight: 'bold' }}>
        {pool.token0.symbol}
      </Typography>
      <Typography sx={{ color: '#9ca3af' }}>/</Typography>
      <Typography sx={{ color: '#00d4aa', fontWeight: 'bold' }}>
        {pool.token1.symbol}
      </Typography>
    </Box>);
    const renderDEXChip = (dex) => {
        const info = DEX_INFO[dex] || { name: dex, logo: '📦', color: '#666' };
        return (<Chip label={`${info.logo} ${info.name}`} size="small" sx={{
                bgcolor: `${info.color}20`,
                color: info.color,
                fontSize: '0.7rem',
            }}/>);
    };
    const renderFeeChip = (fee) => (<Chip label={`${fee}%`} size="small" sx={{
            bgcolor: fee <= 0.05 ? '#00d4aa20' : fee <= 0.3 ? '#ff980020' : '#ff572220',
            color: fee <= 0.05 ? '#00d4aa' : fee <= 0.3 ? '#ff9800' : '#ff5722',
            fontSize: '0.7rem',
        }}/>);
    // ============================================================================
    // Main Render
    // ============================================================================
    return (<Box sx={{ minHeight: '100vh', bgcolor: '#0a0a14', p: 3 }}>
      <Box sx={{ maxWidth: 1400, mx: 'auto' }}>
        {/* Header */}
        <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 4 }}>
          <Box>
            <Typography variant="h4" sx={{ color: 'white', fontWeight: 'bold' }}>
              🏊 Liquidity Pools
            </Typography>
            <Typography variant="body2" sx={{ color: '#9ca3af', mt: 1 }}>
              Provide liquidity and earn fees from trades
            </Typography>
          </Box>
          <Box sx={{ display: 'flex', gap: 2 }}>
            <FormControl size="small" sx={{ minWidth: 150 }}>
              <Select value={chainId} onChange={(e) => setChainId(e.target.value)} sx={{ color: 'white', bgcolor: '#1a1a2e', '& .MuiOutlinedInput-notchedOutline': { borderColor: '#3a3a4e' } }}>
                {Object.entries(CHAIN_CONFIG).map(([id, config]) => (<MenuItem key={id} value={parseInt(id)}>{config.name}</MenuItem>))}
              </Select>
            </FormControl>
            <Button variant="outlined" startIcon={<Refresh />} onClick={loadPools} sx={{ borderColor: '#3a3a4e', color: 'white' }}>
              Refresh
            </Button>
            <Button variant="contained" startIcon={<Add />} onClick={() => setShowCreatePool(true)} sx={{ bgcolor: '#00d4aa', color: 'black', '&:hover': { bgcolor: '#00b894' } }}>
              Create Pool
            </Button>
          </Box>
        </Box>

        {/* Stats Cards */}
        {stats && (<Box sx={{ display: 'grid', gridTemplateColumns: 'repeat(5, 1fr)', gap: 2, mb: 4 }}>
            <Card sx={{ bgcolor: '#1a1a2e', borderRadius: 3 }}>
              <CardContent>
                <Typography variant="caption" sx={{ color: '#9ca3af' }}>Total TVL</Typography>
                <Typography variant="h5" sx={{ color: '#00d4aa', fontWeight: 'bold' }}>
                  {formatUSD(stats.totalTVL)}
                </Typography>
              </CardContent>
            </Card>
            <Card sx={{ bgcolor: '#1a1a2e', borderRadius: 3 }}>
              <CardContent>
                <Typography variant="caption" sx={{ color: '#9ca3af' }}>24h Volume</Typography>
                <Typography variant="h5" sx={{ color: 'white', fontWeight: 'bold' }}>
                  {formatUSD(stats.totalVolume24h)}
                </Typography>
              </CardContent>
            </Card>
            <Card sx={{ bgcolor: '#1a1a2e', borderRadius: 3 }}>
              <CardContent>
                <Typography variant="caption" sx={{ color: '#9ca3af' }}>7d Volume</Typography>
                <Typography variant="h5" sx={{ color: 'white', fontWeight: 'bold' }}>
                  {formatUSD(stats.totalVolume7d)}
                </Typography>
              </CardContent>
            </Card>
            <Card sx={{ bgcolor: '#1a1a2e', borderRadius: 3 }}>
              <CardContent>
                <Typography variant="caption" sx={{ color: '#9ca3af' }}>24h Fees</Typography>
                <Typography variant="h5" sx={{ color: '#ff9800', fontWeight: 'bold' }}>
                  {formatUSD(stats.totalFees24h)}
                </Typography>
              </CardContent>
            </Card>
            <Card sx={{ bgcolor: '#1a1a2e', borderRadius: 3 }}>
              <CardContent>
                <Typography variant="caption" sx={{ color: '#9ca3af' }}>Total Pools</Typography>
                <Typography variant="h5" sx={{ color: 'white', fontWeight: 'bold' }}>
                  {stats.totalPools}
                </Typography>
              </CardContent>
            </Card>
          </Box>)}

        {/* Tabs */}
        <Card sx={{ bgcolor: '#1a1a2e', borderRadius: 3, mb: 3 }}>
          <Tabs value={activeTab} onChange={(_, v) => setActiveTab(v)} sx={{
            borderBottom: '1px solid #2a2a3e',
            '& .MuiTab-root': { color: '#9ca3af' },
            '& .Mui-selected': { color: '#00d4aa' },
        }}>
            <Tab label="All Pools"/>
            <Tab label="My Positions"/>
            <Tab label="Analytics"/>
          </Tabs>

          <CardContent sx={{ p: 3 }}>
            {/* Filters */}
            <Box sx={{ display: 'flex', gap: 2, mb: 3, flexWrap: 'wrap' }}>
              <TextField size="small" placeholder="Search tokens or DEX..." value={searchQuery} onChange={(e) => setSearchQuery(e.target.value)} sx={{
            minWidth: 250,
            '& input': { color: 'white' },
            '& .MuiOutlinedInput-root': {
                '& fieldset': { borderColor: '#3a3a4e' },
            },
        }}/>
              <FormControl size="small" sx={{ minWidth: 150 }}>
                <Select value={filterDEX} onChange={(e) => setFilterDEX(e.target.value)} sx={{ color: 'white', '& .MuiOutlinedInput-notchedOutline': { borderColor: '#3a3a4e' } }}>
                  <MenuItem value="all">All DEXs</MenuItem>
                  {Object.entries(DEX_INFO).map(([key, info]) => (<MenuItem key={key} value={key}>{info.logo} {info.name}</MenuItem>))}
                </Select>
              </FormControl>
              <FormControl size="small" sx={{ minWidth: 120 }}>
                <Select value={sortBy} onChange={(e) => setSortBy(e.target.value)} sx={{ color: 'white', '& .MuiOutlinedInput-notchedOutline': { borderColor: '#3a3a4e' } }}>
                  <MenuItem value="tvl">Sort by TVL</MenuItem>
                  <MenuItem value="volume">Sort by Volume</MenuItem>
                  <MenuItem value="apr">Sort by APR</MenuItem>
                </Select>
              </FormControl>
            </Box>

            {/* Pool List */}
            {loading ? (<Box sx={{ display: 'flex', justifyContent: 'center', py: 5 }}>
                <CircularProgress sx={{ color: '#00d4aa' }}/>
              </Box>) : activeTab === 0 ? (<TableContainer>
                <Table>
                  <TableHead>
                    <TableRow>
                      <TableCell sx={{ color: '#9ca3af' }}>Pool</TableCell>
                      <TableCell sx={{ color: '#9ca3af' }}>DEX</TableCell>
                      <TableCell sx={{ color: '#9ca3af' }}>Fee</TableCell>
                      <TableCell sx={{ color: '#9ca3af' }} align="right">TVL</TableCell>
                      <TableCell sx={{ color: '#9ca3af' }} align="right">Volume (24h)</TableCell>
                      <TableCell sx={{ color: '#9ca3af' }} align="right">APR</TableCell>
                      <TableCell sx={{ color: '#9ca3af' }} align="right">Actions</TableCell>
                    </TableRow>
                  </TableHead>
                  <TableBody>
                    {filteredPools.map((pool) => (<TableRow key={pool.id} sx={{ '&:hover': { bgcolor: '#2a2a3e' } }}>
                        <TableCell>
                          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                            {renderPoolChip(pool)}
                          </Box>
                        </TableCell>
                        <TableCell>{renderDEXChip(pool.dex)}</TableCell>
                        <TableCell>{renderFeeChip(pool.feeTier)}</TableCell>
                        <TableCell align="right">
                          <Typography sx={{ color: '#00d4aa' }}>{formatUSD(pool.tvlUSD)}</Typography>
                        </TableCell>
                        <TableCell align="right">
                          <Typography sx={{ color: 'white' }}>{formatUSD(pool.volume24h)}</Typography>
                        </TableCell>
                        <TableCell align="right">
                          <Typography sx={{ color: pool.apr > 20 ? '#00d4aa' : pool.apr > 5 ? '#ff9800' : '#ff5722' }}>
                            {formatPercent(pool.apr)}
                          </Typography>
                        </TableCell>
                        <TableCell align="right">
                          <Box sx={{ display: 'flex', gap: 1, justifyContent: 'flex-end' }}>
                            <Tooltip title="View Details">
                              <IconButton size="small" onClick={() => { setSelectedPool(pool); setShowPoolDetails(true); }}>
                                <Visibility sx={{ color: '#9ca3af', fontSize: 18 }}/>
                              </IconButton>
                            </Tooltip>
                            <Tooltip title="Add Liquidity">
                              <IconButton size="small" onClick={() => { setSelectedPool(pool); setShowPoolDetails(true); }}>
                                <Add sx={{ color: '#00d4aa', fontSize: 18 }}/>
                              </IconButton>
                            </Tooltip>
                          </Box>
                        </TableCell>
                      </TableRow>))}
                  </TableBody>
                </Table>
              </TableContainer>) : activeTab === 1 ? (<Box>
                {positions.length === 0 ? (<Box sx={{ textAlign: 'center', py: 5 }}>
                    <Typography sx={{ color: '#9ca3af', mb: 2 }}>No liquidity positions</Typography>
                    <Button variant="contained" startIcon={<Add />} onClick={() => setShowCreatePool(true)} sx={{ bgcolor: '#00d4aa', color: 'black' }}>
                      Create Your First Position
                    </Button>
                  </Box>) : (<TableContainer>
                    <Table>
                      <TableHead>
                        <TableRow>
                          <TableCell sx={{ color: '#9ca3af' }}>Pool</TableCell>
                          <TableCell sx={{ color: '#9ca3af' }} align="right">Token 0 Amount</TableCell>
                          <TableCell sx={{ color: '#9ca3af' }} align="right">Token 1 Amount</TableCell>
                          <TableCell sx={{ color: '#9ca3af' }} align="right">Total Value</TableCell>
                          <TableCell sx={{ color: '#9ca3af' }} align="right">Share</TableCell>
                          <TableCell sx={{ color: '#9ca3af' }} align="right">Fees Earned</TableCell>
                          <TableCell sx={{ color: '#9ca3af' }} align="right">Status</TableCell>
                        </TableRow>
                      </TableHead>
                      <TableBody>
                        {positions.map((pos) => (<TableRow key={pos.id} sx={{ '&:hover': { bgcolor: '#2a2a3e' } }}>
                            <TableCell>{renderPoolChip(pos.pool)}</TableCell>
                            <TableCell align="right">
                              <Typography sx={{ color: 'white' }}>
                                {parseFloat(pos.token0Amount).toFixed(4)} {pos.pool.token0.symbol}
                              </Typography>
                            </TableCell>
                            <TableCell align="right">
                              <Typography sx={{ color: 'white' }}>
                                {parseFloat(pos.token1Amount).toFixed(4)} {pos.pool.token1.symbol}
                              </Typography>
                            </TableCell>
                            <TableCell align="right">
                              <Typography sx={{ color: '#00d4aa' }}>{formatUSD(pos.totalLiquidity)}</Typography>
                            </TableCell>
                            <TableCell align="right">
                              <Typography sx={{ color: 'white' }}>{pos.poolShare.toFixed(3)}%</Typography>
                            </TableCell>
                            <TableCell align="right">
                              <Typography sx={{ color: '#ff9800' }}>
                                {pos.feesEarned0} {pos.pool.token0.symbol} + {pos.feesEarned1} {pos.pool.token1.symbol}
                              </Typography>
                            </TableCell>
                            <TableCell align="right">
                              <Chip label={pos.isActive ? 'Active' : 'Inactive'} size="small" sx={{
                        bgcolor: pos.isActive ? '#00d4aa20' : '#ff572220',
                        color: pos.isActive ? '#00d4aa' : '#ff5722',
                    }}/>
                            </TableCell>
                          </TableRow>))}
                      </TableBody>
                    </Table>
                  </TableContainer>)}
              </Box>) : (<Box sx={{ py: 3 }}>
                <Typography sx={{ color: '#9ca3af', textAlign: 'center' }}>
                  📊 Analytics Dashboard - Coming Soon
                </Typography>
              </Box>)}
          </CardContent>
        </Card>
      </Box>

      {/* Create Pool Dialog */}
      <Dialog open={showCreatePool} onClose={() => setShowCreatePool(false)} maxWidth="md" fullWidth PaperProps={{ sx: { bgcolor: '#1a1a2e', backgroundImage: 'none' } }}>
        <DialogTitle sx={{ color: 'white', display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          Create New Pool
          <IconButton onClick={() => setShowCreatePool(false)} sx={{ color: 'white' }}>
            <Close />
          </IconButton>
        </DialogTitle>
        <DialogContent>
          <Box sx={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 3, mt: 2 }}>
            <Box>
              <Typography variant="body2" sx={{ color: '#9ca3af', mb: 1 }}>Token 0</Typography>
              <TextField fullWidth size="small" placeholder="Enter token symbol (e.g., ETH)" value={token0?.symbol || ''} onChange={(e) => {
            const symbol = e.target.value.toUpperCase();
            const found = COMMON_TOKENS[chainId]?.[symbol];
            if (found)
                setToken0({ ...found, chainId });
        }} sx={{
            '& input': { color: 'white' },
            '& .MuiOutlinedInput-root': { '& fieldset': { borderColor: '#3a3a4e' } },
        }}/>
            </Box>
            <Box>
              <Typography variant="body2" sx={{ color: '#9ca3af', mb: 1 }}>Token 1</Typography>
              <TextField fullWidth size="small" placeholder="Enter token symbol (e.g., USDC)" value={token1?.symbol || ''} onChange={(e) => {
            const symbol = e.target.value.toUpperCase();
            const found = COMMON_TOKENS[chainId]?.[symbol];
            if (found)
                setToken1({ ...found, chainId });
        }} sx={{
            '& input': { color: 'white' },
            '& .MuiOutlinedInput-root': { '& fieldset': { borderColor: '#3a3a4e' } },
        }}/>
            </Box>
            <Box>
              <Typography variant="body2" sx={{ color: '#9ca3af', mb: 1 }}>Amount Token 0</Typography>
              <TextField fullWidth size="small" type="number" placeholder="0.0" value={amount0} onChange={(e) => setAmount0(e.target.value)} InputProps={{
            endAdornment: token0 ? <InputAdornment sx={{ color: '#9ca3af' }}>{token0.symbol}</InputAdornment> : null,
        }} sx={{
            '& input': { color: 'white' },
            '& .MuiOutlinedInput-root': { '& fieldset': { borderColor: '#3a3a4e' } },
        }}/>
            </Box>
            <Box>
              <Typography variant="body2" sx={{ color: '#9ca3af', mb: 1 }}>Amount Token 1</Typography>
              <TextField fullWidth size="small" type="number" placeholder="0.0" value={amount1} onChange={(e) => setAmount1(e.target.value)} InputProps={{
            endAdornment: token1 ? <InputAdornment sx={{ color: '#9ca3af' }}>{token1.symbol}</InputAdornment> : null,
        }} sx={{
            '& input': { color: 'white' },
            '& .MuiOutlinedInput-root': { '& fieldset': { borderColor: '#3a3a4e' } },
        }}/>
            </Box>
          </Box>

          <Box sx={{ mt: 3 }}>
            <Typography variant="body2" sx={{ color: '#9ca3af', mb: 2 }}>Fee Tier</Typography>
            <Box sx={{ display: 'flex', gap: 2 }}>
              {FEE_TIERS.map((tier) => (<Card key={tier.value} sx={{
                flex: 1,
                cursor: 'pointer',
                bgcolor: feeTier === tier.value ? '#00d4aa20' : '#2a2a3e',
                borderColor: feeTier === tier.value ? '#00d4aa' : 'transparent',
                border: '1px solid',
                '&:hover': { bgcolor: '#3a3a4e' },
            }} onClick={() => setFeeTier(tier.value)}>
                  <CardContent sx={{ textAlign: 'center', py: 2 }}>
                    <Typography sx={{ color: 'white', fontWeight: 'bold' }}>{tier.label}</Typography>
                    <Typography variant="caption" sx={{ color: '#9ca3af' }}>{tier.description}</Typography>
                  </CardContent>
                </Card>))}
            </Box>
          </Box>

          {feeTier > 0.1 && (<Box sx={{ mt: 3 }}>
              <Typography variant="body2" sx={{ color: '#9ca3af', mb: 2 }}>Price Range (Concentrated Liquidity)</Typography>
              <Box sx={{ display: 'flex', gap: 2 }}>
                <TextField fullWidth size="small" type="number" label="Min Price" value={priceRangeLow} onChange={(e) => setPriceRangeLow(parseFloat(e.target.value) || 0)} sx={{
                '& input': { color: 'white' },
                '& .MuiOutlinedInput-root': { '& fieldset': { borderColor: '#3a3a4e' } },
                '& .MuiInputLabel-root': { color: '#9ca3af' },
            }}/>
                <TextField fullWidth size="small" type="number" label="Max Price" value={priceRangeHigh} onChange={(e) => setPriceRangeHigh(parseFloat(e.target.value) || 0)} sx={{
                '& input': { color: 'white' },
                '& .MuiOutlinedInput-root': { '& fieldset': { borderColor: '#3a3a4e' } },
                '& .MuiInputLabel-root': { color: '#9ca3af' },
            }}/>
              </Box>
            </Box>)}
        </DialogContent>
        <DialogActions sx={{ p: 3 }}>
          <Button onClick={() => setShowCreatePool(false)} sx={{ color: '#9ca3af' }}>
            Cancel
          </Button>
          <Button variant="contained" onClick={handleCreatePool} disabled={creating || !token0 || !token1 || !amount0 || !amount1} sx={{ bgcolor: '#00d4aa', color: 'black' }}>
            {creating ? <CircularProgress size={20} sx={{ color: 'black' }}/> : 'Create Pool'}
          </Button>
        </DialogActions>
      </Dialog>

      {/* Pool Details Dialog */}
      <Dialog open={showPoolDetails} onClose={() => setShowPoolDetails(false)} maxWidth="sm" fullWidth PaperProps={{ sx: { bgcolor: '#1a1a2e', backgroundImage: 'none' } }}>
        <DialogTitle sx={{ color: 'white', display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          Pool Details
          <IconButton onClick={() => setShowPoolDetails(false)} sx={{ color: 'white' }}>
            <Close />
          </IconButton>
        </DialogTitle>
        <DialogContent>
          {selectedPool && (<Box>
              <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 3 }}>
                {renderPoolChip(selectedPool)}
                {renderDEXChip(selectedPool.dex)}
              </Box>

              <Box sx={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 2, mb: 3 }}>
                <Box sx={{ bgcolor: '#2a2a3e', p: 2, borderRadius: 2 }}>
                  <Typography variant="caption" sx={{ color: '#9ca3af' }}>TVL</Typography>
                  <Typography sx={{ color: '#00d4aa', fontWeight: 'bold' }}>{formatUSD(selectedPool.tvlUSD)}</Typography>
                </Box>
                <Box sx={{ bgcolor: '#2a2a3e', p: 2, borderRadius: 2 }}>
                  <Typography variant="caption" sx={{ color: '#9ca3af' }}>24h Volume</Typography>
                  <Typography sx={{ color: 'white', fontWeight: 'bold' }}>{formatUSD(selectedPool.volume24h)}</Typography>
                </Box>
                <Box sx={{ bgcolor: '#2a2a3e', p: 2, borderRadius: 2 }}>
                  <Typography variant="caption" sx={{ color: '#9ca3af' }}>APR</Typography>
                  <Typography sx={{ color: '#00d4aa', fontWeight: 'bold' }}>{formatPercent(selectedPool.apr)}</Typography>
                </Box>
                <Box sx={{ bgcolor: '#2a2a3e', p: 2, borderRadius: 2 }}>
                  <Typography variant="caption" sx={{ color: '#9ca3af' }}>Fee Tier</Typography>
                  <Typography sx={{ color: 'white', fontWeight: 'bold' }}>{selectedPool.feeTier}%</Typography>
                </Box>
              </Box>

              <Box sx={{ mb: 2 }}>
                <Typography variant="caption" sx={{ color: '#9ca3af' }}>Pool Address</Typography>
                <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                  <Typography sx={{ color: 'white', wordBreak: 'break-all', fontSize: '0.85rem' }}>
                    {selectedPool.address}
                  </Typography>
                  <IconButton size="small" onClick={() => copyToClipboard(selectedPool.address)}>
                    <ContentCopy sx={{ color: '#9ca3af', fontSize: 16 }}/>
                  </IconButton>
                </Box>
              </Box>

              <Divider sx={{ borderColor: '#3a3a4e', my: 2 }}/>

              <Typography variant="body2" sx={{ color: '#9ca3af', mb: 2 }}>Add Liquidity</Typography>
              <TextField fullWidth size="small" type="number" placeholder={`Amount ${selectedPool.token0.symbol}`} value={amount0} onChange={(e) => setAmount0(e.target.value)} sx={{
                mb: 2,
                '& input': { color: 'white' },
                '& .MuiOutlinedInput-root': { '& fieldset': { borderColor: '#3a3a4e' } },
            }}/>
              <TextField fullWidth size="small" type="number" placeholder={`Amount ${selectedPool.token1.symbol}`} value={amount1} onChange={(e) => setAmount1(e.target.value)} sx={{
                mb: 2,
                '& input': { color: 'white' },
                '& .MuiOutlinedInput-root': { '& fieldset': { borderColor: '#3a3a4e' } },
            }}/>
            </Box>)}
        </DialogContent>
        <DialogActions sx={{ p: 3 }}>
          <Button onClick={() => setShowPoolDetails(false)} sx={{ color: '#9ca3af' }}>
            Cancel
          </Button>
          <Button variant="contained" onClick={handleAddLiquidity} disabled={creating || !amount0 || !amount1} sx={{ bgcolor: '#00d4aa', color: 'black' }}>
            {creating ? <CircularProgress size={20} sx={{ color: 'black' }}/> : 'Add Liquidity'}
          </Button>
        </DialogActions>
      </Dialog>

      {/* Snackbar */}
      <Snackbar open={snackbar.open} autoHideDuration={5000} onClose={() => setSnackbar({ ...snackbar, open: false })} anchorOrigin={{ vertical: 'bottom', horizontal: 'center' }}>
        <Alert onClose={() => setSnackbar({ ...snackbar, open: false })} severity={snackbar.severity} sx={{ bgcolor: snackbar.severity === 'success' ? '#1b5e20' : snackbar.severity === 'error' ? '#b71c1c' : '#1a237e' }}>
          {snackbar.message}
        </Alert>
      </Snackbar>
    </Box>);
}
//# sourceMappingURL=page.jsx.map