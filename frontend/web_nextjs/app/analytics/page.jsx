'use client';
import React, { useState, useEffect, useCallback } from 'react';
import { Box, Typography, Card, CardContent, Button, Table, TableBody, TableCell, TableContainer, TableHead, TableRow, Tabs, Tab, Tooltip, CircularProgress, LinearProgress, ToggleButton, ToggleButtonGroup } from '@mui/material';
import { TrendingUp, TrendingDown, Refresh, Download, AttachMoney, Speed, Pool, SwapHoriz, AccountBalanceWallet } from '@mui/icons-material';
// ============================================================================
// Constants
// ============================================================================
const CHAIN_CONFIG = {
    1: { name: 'Ethereum', color: '#627EEA' },
    56: { name: 'BNB Chain', color: '#F3BA2F' },
    137: { name: 'Polygon', color: '#8247E5' },
    42161: { name: 'Arbitrum', color: '#28A0F0' },
    10: { name: 'Optimism', color: '#FF0420' },
    8453: { name: 'Base', color: '#0052FF' },
    43114: { name: 'Avalanche', color: '#E84142' },
};
const TOP_TOKENS = [
    { symbol: 'ETH', name: 'Ethereum', price: 2450, change24h: 2.5, volume24h: 1250000000, marketCap: 294000000000 },
    { symbol: 'WBTC', name: 'Wrapped Bitcoin', price: 62500, change24h: 1.2, volume24h: 890000000, marketCap: 1210000000000 },
    { symbol: 'USDC', name: 'USD Coin', price: 1.00, change24h: 0.01, volume24h: 520000000, marketCap: 32000000000 },
    { symbol: 'USDT', name: 'Tether', price: 1.00, change24h: 0.02, volume24h: 480000000, marketCap: 95000000000 },
    { symbol: 'LINK', name: 'Chainlink', price: 18.5, change24h: -1.5, volume24h: 125000000, marketCap: 10500000000 },
    { symbol: 'UNI', name: 'Uniswap', price: 12.5, change24h: 3.2, volume24h: 89000000, marketCap: 7500000000 },
    { symbol: 'AAVE', name: 'Aave', price: 285, change24h: 4.5, volume24h: 67000000, marketCap: 4200000000 },
    { symbol: 'MATIC', name: 'Polygon', price: 0.85, change24h: -2.1, volume24h: 45000000, marketCap: 8500000000 },
];
// ============================================================================
// Utility Functions
// ============================================================================
function formatUSD(amount) {
    if (amount >= 1e12)
        return `$${(amount / 1e12).toFixed(2)}T`;
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
function formatPercent(value) {
    return `${value >= 0 ? '+' : ''}${value.toFixed(2)}%`;
}
function formatDate(timestamp) {
    return new Date(timestamp).toLocaleDateString('en-US', { month: 'short', day: 'numeric' });
}
// ============================================================================
// Mock Data Generators
// ============================================================================
function generateProtocolStats() {
    return {
        totalVolume24h: Math.random() * 500000000 + 100000000,
        totalVolume7d: Math.random() * 3000000000 + 500000000,
        totalVolume30d: Math.random() * 10000000000 + 2000000000,
        totalFees24h: Math.random() * 1500000 + 500000,
        totalFees7d: Math.random() * 10000000 + 3000000,
        totalFees30d: Math.random() * 40000000 + 10000000,
        totalTVL: Math.random() * 500000000 + 200000000,
        totalUsers: Math.floor(Math.random() * 500000 + 100000),
        totalTransactions: Math.floor(Math.random() * 1000000 + 200000),
        avgSwapSize: Math.random() * 5000 + 500,
        priceChange24h: (Math.random() - 0.5) * 10,
    };
}
function generateVolumeData(days = 30) {
    const data = [];
    let baseVolume = 200000000;
    for (let i = days; i >= 0; i--) {
        const timestamp = Date.now() - i * 24 * 60 * 60 * 1000;
        const variation = 0.8 + Math.random() * 0.4;
        const volume = baseVolume * variation;
        const trades = Math.floor(volume / 2000);
        const fees = volume * 0.003;
        data.push({ timestamp, volume, trades, fees });
        baseVolume = volume * (0.95 + Math.random() * 0.1);
    }
    return data;
}
function generatePoolAnalytics() {
    const pools = [];
    const poolNames = [
        'ETH/USDC', 'ETH/USDT', 'WBTC/ETH', 'ETH/DAI', 'LINK/ETH',
        'UNI/ETH', 'AAVE/ETH', 'MATIC/ETH', 'CRV/ETH', 'SNX/ETH'
    ];
    for (let i = 0; i < poolNames.length; i++) {
        const tvl = Math.random() * 50000000 + 1000000;
        const volume24h = Math.random() * 5000000 + 100000;
        pools.push({
            id: `pool-${i}`,
            name: poolNames[i],
            tvl,
            volume24h,
            volume7d: volume24h * 7 * (0.8 + Math.random() * 0.4),
            fees24h: volume24h * 0.003,
            apr: Math.random() * 150 + 10,
            utilization: Math.random() * 80 + 10,
        });
    }
    return pools.sort((a, b) => b.volume24h - a.volume24h);
}
function generateChainAnalytics() {
    return Object.entries(CHAIN_CONFIG).map(([id, config], index) => {
        const share = [45, 25, 12, 8, 5, 3, 2][index] || 2;
        const volume24h = Math.random() * 200000000 + 50000000;
        return {
            chainId: parseInt(id),
            chainName: config.name,
            volume24h,
            tvl: Math.random() * 100000000 + 10000000,
            transactions: Math.floor(Math.random() * 200000 + 10000),
            avgGasPrice: Math.random() * 100 + 10,
            sharePercent: share,
        };
    }).sort((a, b) => b.volume24h - a.volume24h);
}
// ============================================================================
// Main Analytics Page Component
// ============================================================================
export default function AnalyticsPage() {
    // State
    const [timeRange, setTimeRange] = useState('7d');
    const [activeTab, setActiveTab] = useState(0);
    const [loading, setLoading] = useState(true);
    const [protocolStats, setProtocolStats] = useState(null);
    const [volumeData, setVolumeData] = useState([]);
    const [poolAnalytics, setPoolAnalytics] = useState([]);
    const [chainAnalytics, setChainAnalytics] = useState([]);
    const [tokenAnalytics] = useState(TOP_TOKENS);
    // ============================================================================
    // Data Loading
    // ============================================================================
    const loadAnalytics = useCallback(async () => {
        setLoading(true);
        try {
            await new Promise(resolve => setTimeout(resolve, 800));
            setProtocolStats(generateProtocolStats());
            setVolumeData(generateVolumeData(30));
            setPoolAnalytics(generatePoolAnalytics());
            setChainAnalytics(generateChainAnalytics());
        }
        catch (error) {
            console.error('Failed to load analytics:', error);
        }
        finally {
            setLoading(false);
        }
    }, []);
    useEffect(() => {
        loadAnalytics();
    }, [loadAnalytics]);
    // ============================================================================
    // Render Helpers
    // ============================================================================
    const renderChangeIndicator = (value) => {
        const isPositive = value >= 0;
        return (<Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5, color: isPositive ? '#00d4aa' : '#ff4757' }}>
        {isPositive ? <TrendingUp fontSize="small"/> : <TrendingDown fontSize="small"/>}
        <Typography variant="body2" sx={{ fontWeight: 'bold' }}>
          {formatPercent(value)}
        </Typography>
      </Box>);
    };
    const renderStatCard = (title, value, change, icon, subtitle) => (<Card sx={{ bgcolor: '#1a1a2e', borderRadius: 3 }}>
      <CardContent>
        <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', mb: 1 }}>
          <Typography variant="caption" sx={{ color: '#9ca3af' }}>{title}</Typography>
          {icon && <Box sx={{ color: '#9ca3af' }}>{icon}</Box>}
        </Box>
        <Typography variant="h5" sx={{ color: 'white', fontWeight: 'bold', mb: 0.5 }}>
          {value}
        </Typography>
        {change !== undefined && renderChangeIndicator(change)}
        {subtitle && (<Typography variant="caption" sx={{ color: '#9ca3af', display: 'block', mt: 0.5 }}>
            {subtitle}
          </Typography>)}
      </CardContent>
    </Card>);
    // ============================================================================
    // Main Render
    // ============================================================================
    return (<Box sx={{ minHeight: '100vh', bgcolor: '#0a0a14', p: 3 }}>
      <Box sx={{ maxWidth: 1600, mx: 'auto' }}>
        {/* Header */}
        <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 4 }}>
          <Box>
            <Typography variant="h4" sx={{ color: 'white', fontWeight: 'bold' }}>
              📊 Analytics Dashboard
            </Typography>
            <Typography variant="body2" sx={{ color: '#9ca3af', mt: 1 }}>
              Real-time protocol metrics and performance insights
            </Typography>
          </Box>
          <Box sx={{ display: 'flex', gap: 2 }}>
            <ToggleButtonGroup value={timeRange} exclusive onChange={(_, v) => v && setTimeRange(v)} size="small" sx={{
            '& .MuiToggleButton-root': {
                color: '#9ca3af',
                borderColor: '#3a3a4e',
                '&.Mui-selected': { bgcolor: '#00d4aa', color: 'black' },
            },
        }}>
              <ToggleButton value="24h">24H</ToggleButton>
              <ToggleButton value="7d">7D</ToggleButton>
              <ToggleButton value="30d">30D</ToggleButton>
              <ToggleButton value="all">ALL</ToggleButton>
            </ToggleButtonGroup>
            <Button variant="outlined" startIcon={<Download />} sx={{ borderColor: '#3a3a4e', color: 'white' }}>
              Export
            </Button>
            <Button variant="outlined" startIcon={<Refresh />} onClick={loadAnalytics} sx={{ borderColor: '#3a3a4e', color: 'white' }}>
              Refresh
            </Button>
          </Box>
        </Box>

        {/* Time Range Stats */}
        {protocolStats && (<>
            <Box sx={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: 2, mb: 4 }}>
              {timeRange === '24h' && (<>
                  {renderStatCard('24h Volume', formatUSD(protocolStats.totalVolume24h), undefined, <SwapHoriz />)}
                  {renderStatCard('24h Fees', formatUSD(protocolStats.totalFees24h), undefined, <AttachMoney />)}
                  {renderStatCard('Transactions', formatNumber(protocolStats.totalTransactions), undefined, <Speed />)}
                  {renderStatCard('Avg Swap Size', formatUSD(protocolStats.avgSwapSize), undefined, <AccountBalanceWallet />)}
                </>)}
              {timeRange === '7d' && (<>
                  {renderStatCard('7d Volume', formatUSD(protocolStats.totalVolume7d), undefined, <SwapHoriz />)}
                  {renderStatCard('7d Fees', formatUSD(protocolStats.totalFees7d), undefined, <AttachMoney />)}
                  {renderStatCard('Total Users', formatNumber(protocolStats.totalUsers), undefined, <AccountBalanceWallet />)}
                  {renderStatCard('TVL', formatUSD(protocolStats.totalTVL), undefined, <Pool />)}
                </>)}
              {timeRange === '30d' && (<>
                  {renderStatCard('30d Volume', formatUSD(protocolStats.totalVolume30d), undefined, <SwapHoriz />)}
                  {renderStatCard('30d Fees', formatUSD(protocolStats.totalFees30d), undefined, <AttachMoney />)}
                  {renderStatCard('Total Users', formatNumber(protocolStats.totalUsers), undefined, <AccountBalanceWallet />)}
                  {renderStatCard('TVL', formatUSD(protocolStats.totalTVL), undefined, <Pool />)}
                </>)}
              {timeRange === 'all' && (<>
                  {renderStatCard('All-Time Volume', formatUSD(protocolStats.totalVolume30d * 12), undefined, <SwapHoriz />)}
                  {renderStatCard('All-Time Fees', formatUSD(protocolStats.totalFees30d * 12), undefined, <AttachMoney />)}
                  {renderStatCard('Total Users', formatNumber(protocolStats.totalUsers), undefined, <AccountBalanceWallet />)}
                  {renderStatCard('TVL', formatUSD(protocolStats.totalTVL), undefined, <Pool />)}
                </>)}
            </Box>
          </>)}

        {/* Tabs */}
        <Card sx={{ bgcolor: '#1a1a2e', borderRadius: 3, mb: 3 }}>
          <Tabs value={activeTab} onChange={(_, v) => setActiveTab(v)} sx={{
            borderBottom: '1px solid #2a2a3e',
            '& .MuiTab-root': { color: '#9ca3af' },
            '& .Mui-selected': { color: '#00d4aa' },
        }}>
            <Tab label="Overview"/>
            <Tab label="Pools"/>
            <Tab label="Tokens"/>
            <Tab label="Chains"/>
          </Tabs>

          <CardContent sx={{ p: 3 }}>
            {loading ? (<Box sx={{ display: 'flex', justifyContent: 'center', py: 5 }}>
                <CircularProgress sx={{ color: '#00d4aa' }}/>
              </Box>) : activeTab === 0 ? (
        /* Overview Tab */
        <Box>
                {/* Protocol Stats Grid */}
                <Box sx={{ display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: 2, mb: 4 }}>
                  {protocolStats && (<>
                      {renderStatCard('Total Value Locked', formatUSD(protocolStats.totalTVL), protocolStats.priceChange24h, <Pool />)}
                      {renderStatCard('24h Trading Volume', formatUSD(protocolStats.totalVolume24h), undefined, <SwapHoriz />)}
                      {renderStatCard('Active Users (24h)', formatNumber(protocolStats.totalUsers * 0.15), undefined, <AccountBalanceWallet />, 'Based on unique wallets')}
                    </>)}
                </Box>

                {/* Volume Chart */}
                <Card sx={{ bgcolor: '#2a2a3e', borderRadius: 3, mb: 3 }}>
                  <CardContent>
                    <Typography variant="h6" sx={{ color: 'white', mb: 2 }}>Volume History</Typography>
                    <Box sx={{ height: 300, display: 'flex', alignItems: 'flex-end', gap: 1, px: 2 }}>
                      {volumeData.slice(-14).map((data, idx) => {
                const maxVolume = Math.max(...volumeData.slice(-14).map(d => d.volume));
                const height = (data.volume / maxVolume) * 250;
                return (<Tooltip key={idx} title={`${formatDate(data.timestamp)}: ${formatUSD(data.volume)}`}>
                            <Box sx={{
                        flex: 1,
                        height,
                        bgcolor: 'linear-gradient(to top, #00d4aa, #00d4aa80)',
                        background: 'linear-gradient(to top, #00d4aa, #00d4aa60)',
                        borderRadius: '4px 4px 0 0',
                        minWidth: 20,
                        cursor: 'pointer',
                        '&:hover': { background: '#00d4aa' },
                    }}/>
                          </Tooltip>);
            })}
                    </Box>
                    <Box sx={{ display: 'flex', justifyContent: 'space-between', mt: 2, px: 2 }}>
                      <Typography variant="caption" sx={{ color: '#9ca3af' }}>
                        {formatDate(volumeData[volumeData.length - 14]?.timestamp || Date.now())}
                      </Typography>
                      <Typography variant="caption" sx={{ color: '#9ca3af' }}>
                        {formatDate(volumeData[volumeData.length - 1]?.timestamp || Date.now())}
                      </Typography>
                    </Box>
                  </CardContent>
                </Card>

                {/* Key Metrics */}
                <Box sx={{ display: 'grid', gridTemplateColumns: 'repeat(2, 1fr)', gap: 3 }}>
                  <Card sx={{ bgcolor: '#2a2a3e', borderRadius: 3 }}>
                    <CardContent>
                      <Typography variant="h6" sx={{ color: 'white', mb: 2 }}>Fee Distribution</Typography>
                      <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
                        {[
                { label: 'Swap Fees', value: 65, color: '#00d4aa' },
                { label: 'Protocol Fees', value: 20, color: '#00d4ff' },
                { label: 'Gas Refunds', value: 10, color: '#ff9800' },
                { label: 'Other', value: 5, color: '#9ca3af' },
            ].map(item => (<Box key={item.label}>
                            <Box sx={{ display: 'flex', justifyContent: 'space-between', mb: 0.5 }}>
                              <Typography variant="body2" sx={{ color: '#9ca3af' }}>{item.label}</Typography>
                              <Typography variant="body2" sx={{ color: 'white' }}>{item.value}%</Typography>
                            </Box>
                            <LinearProgress variant="determinate" value={item.value} sx={{
                    height: 8,
                    borderRadius: 4,
                    bgcolor: '#1a1a2e',
                    '& .MuiLinearProgress-bar': { bgcolor: item.color, borderRadius: 4 },
                }}/>
                          </Box>))}
                      </Box>
                    </CardContent>
                  </Card>

                  <Card sx={{ bgcolor: '#2a2a3e', borderRadius: 3 }}>
                    <CardContent>
                      <Typography variant="h6" sx={{ color: 'white', mb: 2 }}>Top Performing Pools</Typography>
                      {poolAnalytics.slice(0, 5).map((pool, idx) => (<Box key={pool.id} sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 1.5 }}>
                          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                            <Typography variant="body2" sx={{ color: '#9ca3af', width: 20 }}>{idx + 1}</Typography>
                            <Typography variant="body2" sx={{ color: 'white' }}>{pool.name}</Typography>
                          </Box>
                          <Box sx={{ textAlign: 'right' }}>
                            <Typography variant="body2" sx={{ color: '#00d4aa' }}>{formatPercent(pool.apr)}</Typography>
                            <Typography variant="caption" sx={{ color: '#9ca3af' }}>APR</Typography>
                          </Box>
                        </Box>))}
                    </CardContent>
                  </Card>
                </Box>
              </Box>) : activeTab === 1 ? (
        /* Pools Tab */
        <TableContainer>
                <Table>
                  <TableHead>
                    <TableRow>
                      <TableCell sx={{ color: '#9ca3af' }}>Pool</TableCell>
                      <TableCell sx={{ color: '#9ca3af' }} align="right">TVL</TableCell>
                      <TableCell sx={{ color: '#9ca3af' }} align="right">Volume (24h)</TableCell>
                      <TableCell sx={{ color: '#9ca3af' }} align="right">Fees (24h)</TableCell>
                      <TableCell sx={{ color: '#9ca3af' }} align="right">APR</TableCell>
                      <TableCell sx={{ color: '#9ca3af' }} align="right">Utilization</TableCell>
                    </TableRow>
                  </TableHead>
                  <TableBody>
                    {poolAnalytics.map((pool) => (<TableRow key={pool.id} sx={{ '&:hover': { bgcolor: '#2a2a3e' } }}>
                        <TableCell>
                          <Typography sx={{ color: '#00d4aa', fontWeight: 'bold' }}>{pool.name}</Typography>
                        </TableCell>
                        <TableCell align="right">
                          <Typography sx={{ color: 'white' }}>{formatUSD(pool.tvl)}</Typography>
                        </TableCell>
                        <TableCell align="right">
                          <Typography sx={{ color: 'white' }}>{formatUSD(pool.volume24h)}</Typography>
                        </TableCell>
                        <TableCell align="right">
                          <Typography sx={{ color: '#ff9800' }}>{formatUSD(pool.fees24h)}</Typography>
                        </TableCell>
                        <TableCell align="right">
                          <Typography sx={{ color: pool.apr > 50 ? '#00d4aa' : pool.apr > 20 ? '#ff9800' : '#ff4757', fontWeight: 'bold' }}>
                            {formatPercent(pool.apr)}
                          </Typography>
                        </TableCell>
                        <TableCell align="right">
                          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, justifyContent: 'flex-end' }}>
                            <Box sx={{ width: 60, bgcolor: '#1a1a2e', borderRadius: 1, height: 6 }}>
                              <Box sx={{ width: `${pool.utilization}%`, bgcolor: pool.utilization > 70 ? '#00d4aa' : pool.utilization > 40 ? '#ff9800' : '#ff4757', height: '100%', borderRadius: 1 }}/>
                            </Box>
                            <Typography sx={{ color: 'white' }}>{pool.utilization.toFixed(0)}%</Typography>
                          </Box>
                        </TableCell>
                      </TableRow>))}
                  </TableBody>
                </Table>
              </TableContainer>) : activeTab === 2 ? (
        /* Tokens Tab */
        <TableContainer>
                <Table>
                  <TableHead>
                    <TableRow>
                      <TableCell sx={{ color: '#9ca3af' }}>Token</TableCell>
                      <TableCell sx={{ color: '#9ca3af' }} align="right">Price</TableCell>
                      <TableCell sx={{ color: '#9ca3af' }} align="right">24h Change</TableCell>
                      <TableCell sx={{ color: '#9ca3af' }} align="right">24h Volume</TableCell>
                      <TableCell sx={{ color: '#9ca3af' }} align="right">Market Cap</TableCell>
                    </TableRow>
                  </TableHead>
                  <TableBody>
                    {tokenAnalytics.map((token) => (<TableRow key={token.symbol} sx={{ '&:hover': { bgcolor: '#2a2a3e' } }}>
                        <TableCell>
                          <Box>
                            <Typography sx={{ color: '#00d4aa', fontWeight: 'bold' }}>{token.symbol}</Typography>
                            <Typography variant="caption" sx={{ color: '#9ca3af' }}>{token.name}</Typography>
                          </Box>
                        </TableCell>
                        <TableCell align="right">
                          <Typography sx={{ color: 'white' }}>
                            {token.price < 1 ? `$${token.price.toFixed(4)}` : `$${formatNumber(token.price)}`}
                          </Typography>
                        </TableCell>
                        <TableCell align="right">
                          {renderChangeIndicator(token.change24h)}
                        </TableCell>
                        <TableCell align="right">
                          <Typography sx={{ color: 'white' }}>{formatUSD(token.volume24h)}</Typography>
                        </TableCell>
                        <TableCell align="right">
                          <Typography sx={{ color: 'white' }}>{formatUSD(token.marketCap)}</Typography>
                        </TableCell>
                      </TableRow>))}
                  </TableBody>
                </Table>
              </TableContainer>) : (
        /* Chains Tab */
        <Box>
                <Box sx={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: 2, mb: 4 }}>
                  {chainAnalytics.slice(0, 4).map((chain) => (<Card key={chain.chainId} sx={{ bgcolor: '#2a2a3e', borderRadius: 3 }}>
                      <CardContent>
                        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 1 }}>
                          <Box sx={{ width: 12, height: 12, borderRadius: '50%', bgcolor: CHAIN_CONFIG[chain.chainId]?.color || '#666' }}/>
                          <Typography sx={{ color: 'white', fontWeight: 'bold' }}>{chain.chainName}</Typography>
                        </Box>
                        <Typography variant="h6" sx={{ color: '#00d4aa' }}>{formatUSD(chain.volume24h)}</Typography>
                        <Typography variant="caption" sx={{ color: '#9ca3af' }}>24h Volume</Typography>
                        <Box sx={{ mt: 1 }}>
                          <Typography variant="caption" sx={{ color: '#9ca3af' }}>
                            Share: {chain.sharePercent}%
                          </Typography>
                        </Box>
                      </CardContent>
                    </Card>))}
                </Box>

                <TableContainer>
                  <Table>
                    <TableHead>
                      <TableRow>
                        <TableCell sx={{ color: '#9ca3af' }}>Chain</TableCell>
                        <TableCell sx={{ color: '#9ca3af' }} align="right">Volume (24h)</TableCell>
                        <TableCell sx={{ color: '#9ca3af' }} align="right">TVL</TableCell>
                        <TableCell sx={{ color: '#9ca3af' }} align="right">Transactions</TableCell>
                        <TableCell sx={{ color: '#9ca3af' }} align="right">Avg Gas</TableCell>
                        <TableCell sx={{ color: '#9ca3af' }} align="right">Market Share</TableCell>
                      </TableRow>
                    </TableHead>
                    <TableBody>
                      {chainAnalytics.map((chain) => (<TableRow key={chain.chainId} sx={{ '&:hover': { bgcolor: '#2a2a3e' } }}>
                          <TableCell>
                            <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                              <Box sx={{ width: 10, height: 10, borderRadius: '50%', bgcolor: CHAIN_CONFIG[chain.chainId]?.color || '#666' }}/>
                              <Typography sx={{ color: 'white' }}>{chain.chainName}</Typography>
                            </Box>
                          </TableCell>
                          <TableCell align="right">
                            <Typography sx={{ color: 'white' }}>{formatUSD(chain.volume24h)}</Typography>
                          </TableCell>
                          <TableCell align="right">
                            <Typography sx={{ color: '#00d4aa' }}>{formatUSD(chain.tvl)}</Typography>
                          </TableCell>
                          <TableCell align="right">
                            <Typography sx={{ color: 'white' }}>{formatNumber(chain.transactions)}</Typography>
                          </TableCell>
                          <TableCell align="right">
                            <Typography sx={{ color: '#9ca3af' }}>{chain.avgGasPrice.toFixed(1)} gwei</Typography>
                          </TableCell>
                          <TableCell align="right">
                            <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, justifyContent: 'flex-end' }}>
                              <Box sx={{ width: 60, bgcolor: '#1a1a2e', borderRadius: 1, height: 6 }}>
                                <Box sx={{ width: `${chain.sharePercent * 3}%`, bgcolor: '#00d4aa', height: '100%', borderRadius: 1 }}/>
                              </Box>
                              <Typography sx={{ color: 'white' }}>{chain.sharePercent}%</Typography>
                            </Box>
                          </TableCell>
                        </TableRow>))}
                    </TableBody>
                  </Table>
                </TableContainer>
              </Box>)}
          </CardContent>
        </Card>
      </Box>
    </Box>);
}
//# sourceMappingURL=page.jsx.map