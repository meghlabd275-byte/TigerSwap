'use client';
import React, { useState, useEffect, useCallback } from 'react';
import { Box, Typography, Card, CardContent, Button, TextField, Table, TableBody, TableCell, TableContainer, TableHead, TableRow, Chip, IconButton, Avatar, Snackbar, Alert, CircularProgress, Tabs, Tab, Dialog, DialogTitle, DialogContent, DialogActions, Select, MenuItem, FormControl } from '@mui/material';
import { Star, ContentCopy, Refresh, Verified, Whatshot } from '@mui/icons-material';
// ============================================================================
// Utility Functions
// ============================================================================
function formatAddress(address, chars = 4) {
    return `${address.slice(0, chars + 2)}...${address.slice(-chars)}`;
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
function formatUSD(amount) {
    if (amount >= 1e9)
        return `$${(amount / 1e9).toFixed(2)}B`;
    if (amount >= 1e6)
        return `$${(amount / 1e6).toFixed(2)}M`;
    if (amount >= 1e3)
        return `$${(amount / 1e3).toFixed(2)}K`;
    return `$${amount.toFixed(2)}`;
}
function formatPercent(value) {
    const sign = value >= 0 ? '+' : '';
    return `${sign}${value.toFixed(2)}%`;
}
function timeAgo(timestamp) {
    const diff = Date.now() - timestamp;
    const minutes = Math.floor(diff / 60000);
    if (minutes < 60)
        return `${minutes}m ago`;
    const hours = Math.floor(minutes / 60);
    if (hours < 24)
        return `${hours}h ago`;
    const days = Math.floor(hours / 24);
    return `${days}d ago`;
}
// ============================================================================
// Mock Data Generators
// ============================================================================
function generateTraders() {
    const names = [
        { address: '0x742d35Cc6634C0532925F6e1', ens: 'WhaleKing.eth', verified: true },
        { address: '0x8Ba1f109551bD432803012645Ac136ddd64DBA72', ens: 'AlphaTrader', verified: true },
        { address: '0x3d44D6C896D2346484d5E05b6e3cEb5c5c5d5F9e', ens: 'DeFiMaster', verified: false },
        { address: '0x5B2b5C9A8B4E3D5f6E8c1A2B3D4E5F6A7B8C9D0e', ens: 'YieldFarmer', verified: true },
        { address: '0x9C8B2E4c6D8F0A2B3C4D5E6F7A8B9C0D1E2F3A4b', ens: 'CryptoKing', verified: false },
        { address: '0xA1B2C3D4E5F6A7B8C9D0E1F2A3B4C5D6E7F8A9b', ens: 'ArbitrageBot', verified: true },
        { address: '0xB2C3D4E5F6A7B8C9D0E1F2A3B4C5D6E7F8A9b0C', ens: 'SwingTrader', verified: false },
        { address: '0xC3D4E5F6A7B8C9D0E1F2A3B4C5D6E7F8A9b0C1D', ens: 'TrendFollower', verified: true },
        { address: '0xD4E5F6A7B8C9D0E1F2A3B4C5D6E7F8A9b0C1D2E', ens: 'PnLPro', verified: false },
        { address: '0xE5F6A7B8C9D0E1F2A3B4C5D6E7F8A9b0C1D2E3F', ens: 'MaxProfit', verified: true },
    ];
    return names.map((n, i) => ({
        id: `trader_${i}`,
        address: n.address,
        ensName: n.ens,
        isVerified: n.verified,
        isCopiable: i < 8,
        totalPnL: (Math.random() - 0.2) * 500000,
        totalTrades: Math.floor(Math.random() * 5000) + 500,
        winRate: Math.random() * 30 + 50,
        avgHoldingTime: ['2h', '4h', '8h', '1d', '3d'][Math.floor(Math.random() * 5)],
        followers: Math.floor(Math.random() * 5000) + 100,
        following: Math.floor(Math.random() * 500) + 10,
        totalVolume: Math.random() * 10000000 + 100000,
        profitFactor: Math.random() * 2 + 1,
        sharpeRatio: Math.random() * 3 + 0.5,
        maxDrawdown: -(Math.random() * 30 + 5),
        tradingPair: ['ETH/USDC', 'BTC/USDT', 'ETH/BTC', 'LINK/ETH'][Math.floor(Math.random() * 4)],
        monthlyReturn: (Math.random() - 0.2) * 100,
        lastTradeTime: Date.now() - Math.random() * 3600000,
        isFollowing: Math.random() > 0.7,
    }));
}
function generateLeaderboard(traders) {
    return traders
        .map(trader => ({
        rank: 0,
        trader,
        monthlyReturn: trader.monthlyReturn,
        totalPnL: trader.totalPnL,
    }))
        .sort((a, b) => b.monthlyReturn - a.monthlyReturn)
        .map((entry, i) => ({ ...entry, rank: i + 1 }));
}
// ============================================================================
// Main Leaderboard Page Component
// ============================================================================
export default function LeaderboardPage() {
    // State
    const [traders, setTraders] = useState([]);
    const [leaderboard, setLeaderboard] = useState([]);
    const [loading, setLoading] = useState(true);
    const [activeTab, setActiveTab] = useState(0);
    const [selectedTrader, setSelectedTrader] = useState(null);
    const [showTraderDetail, setShowTraderDetail] = useState(false);
    const [copyAmount, setContentCopyAmount] = useState('');
    const [copying, setContentCopying] = useState(false);
    const [filterPair, setFilterPair] = useState('all');
    // Snackbar
    const [snackbar, setSnackbar] = useState({
        open: false,
        message: '',
        severity: 'success'
    });
    // ============================================================================
    // Data Loading
    // ============================================================================
    const loadData = useCallback(async () => {
        setLoading(true);
        try {
            await new Promise(resolve => setTimeout(resolve, 500));
            const mockTraders = generateTraders();
            setTraders(mockTraders);
            setLeaderboard(generateLeaderboard(mockTraders));
        }
        catch (error) {
            console.error('Failed to load leaderboard:', error);
        }
        finally {
            setLoading(false);
        }
    }, []);
    useEffect(() => {
        loadData();
    }, [loadData]);
    // ============================================================================
    // Trading Functions
    // ============================================================================
    const handleContentCopyTrader = async (trader) => {
        if (!copyAmount || parseFloat(copyAmount) <= 0) {
            setSnackbar({ open: true, message: 'Please enter a valid amount', severity: 'error' });
            return;
        }
        setContentCopying(true);
        try {
            await new Promise(resolve => setTimeout(resolve, 1500));
            setTraders(prev => prev.map(t => t.id === trader.id ? { ...t, followers: t.followers + 1 } : t));
            setShowTraderDetail(false);
            setContentCopyAmount('');
            setSnackbar({
                open: true,
                message: `Now copying ${trader.ensName || formatAddress(trader.address)} with ${copyAmount} USDC`,
                severity: 'success'
            });
        }
        catch (error) {
            setSnackbar({ open: true, message: 'Failed to start copying', severity: 'error' });
        }
        finally {
            setContentCopying(false);
        }
    };
    const handleFollowTrader = async (trader) => {
        try {
            await new Promise(resolve => setTimeout(resolve, 500));
            setTraders(prev => prev.map(t => t.id === trader.id ? { ...t, isFollowing: !t.isFollowing, followers: t.isFollowing ? t.followers - 1 : t.followers + 1 } : t));
            const updatedTrader = traders.find(t => t.id === trader.id);
            setSnackbar({
                open: true,
                message: updatedTrader?.isFollowing ? `Unfollowed ${trader.ensName || formatAddress(trader.address)}` : `Following ${trader.ensName || formatAddress(trader.address)}`,
                severity: 'success'
            });
        }
        catch (error) {
            setSnackbar({ open: true, message: 'Failed to update follow status', severity: 'error' });
        }
    };
    // ============================================================================
    // Filtering
    // ============================================================================
    const filteredLeaderboard = leaderboard
        .filter(entry => filterPair === 'all' || entry.trader.tradingPair === filterPair)
        .slice(0, 20);
    const topTraders = filteredLeaderboard.slice(0, 3);
    // ============================================================================
    // Render Helpers
    // ============================================================================
    const renderRankBadge = (rank) => {
        if (rank === 1)
            return <Whatshot sx={{ color: '#FFD700', fontSize: 28 }}/>;
        if (rank === 2)
            return <Whatshot sx={{ color: '#C0C0C0', fontSize: 24 }}/>;
        if (rank === 3)
            return <Whatshot sx={{ color: '#CD7F32', fontSize: 22 }}/>;
        return <Typography sx={{ color: '#9ca3af', fontWeight: 'bold' }}>{rank}</Typography>;
    };
    const renderPnL = (pnl) => (<Typography sx={{ color: pnl >= 0 ? '#00d4aa' : '#ff5722', fontWeight: 'bold' }}>
      {formatUSD(pnl)}
    </Typography>);
    const renderReturn = (ret) => (<Typography sx={{ color: ret >= 0 ? '#00d4aa' : '#ff5722', fontWeight: 'bold' }}>
      {formatPercent(ret)}
    </Typography>);
    // ============================================================================
    // Main Render
    // ============================================================================
    return (<Box sx={{ minHeight: '100vh', bgcolor: '#0a0a14', p: 3 }}>
      <Box sx={{ maxWidth: 1400, mx: 'auto' }}>
        {/* Header */}
        <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 4 }}>
          <Box>
            <Typography variant="h4" sx={{ color: 'white', fontWeight: 'bold' }}>
              🏆 ContentCopy Trading Leaderboard
            </Typography>
            <Typography variant="body2" sx={{ color: '#9ca3af', mt: 1 }}>
              Follow top traders and automatically copy their trades
            </Typography>
          </Box>
          <Button variant="outlined" startIcon={<Refresh />} onClick={loadData} sx={{ borderColor: '#3a3a4e', color: 'white' }}>
            Refresh
          </Button>
        </Box>

        {/* Top 3 Podium */}
        {topTraders.length >= 3 && (<Box sx={{ display: 'flex', justifyContent: 'center', alignItems: 'flex-end', gap: 2, mb: 4 }}>
            {/* 2nd Place */}
            <Card sx={{ bgcolor: '#1a1a2e', borderRadius: 3, width: 200, border: '2px solid #C0C0C0' }}>
              <CardContent sx={{ textAlign: 'center', pb: 2 }}>
                <Avatar sx={{ width: 60, height: 60, mx: 'auto', mb: 1, bgcolor: '#C0C0C0', fontSize: 24 }}>
                  {formatAddress(topTraders[1].trader.address, 2)}
                </Avatar>
                <Typography sx={{ color: '#9ca3af', fontSize: '0.85rem' }}>{topTraders[1].trader.ensName || formatAddress(topTraders[1].trader.address)}</Typography>
                <Typography variant="h5" sx={{ color: '#C0C0C0', fontWeight: 'bold', my: 1 }}>#2</Typography>
                {renderReturn(topTraders[1].monthlyReturn)}
                <Typography variant="caption" sx={{ color: '#9ca3af', display: 'block' }}>Monthly</Typography>
              </CardContent>
            </Card>

            {/* 1st Place */}
            <Card sx={{ bgcolor: '#1a1a2e', borderRadius: 3, width: 250, border: '2px solid #FFD700' }}>
              <CardContent sx={{ textAlign: 'center', pb: 2 }}>
                <Box sx={{ position: 'relative', mb: 1 }}>
                  <Avatar sx={{ width: 80, height: 80, mx: 'auto', bgcolor: '#FFD700', fontSize: 32 }}>
                    {formatAddress(topTraders[0].trader.address, 2)}
                  </Avatar>
                  <Star sx={{ position: 'absolute', top: -8, right: 'calc(50% - 50px)', color: '#FFD700' }}/>
                </Box>
                <Typography sx={{ color: '#FFD700', fontWeight: 'bold' }}>{topTraders[0].trader.ensName || formatAddress(topTraders[0].trader.address)}</Typography>
                {topTraders[0].trader.isVerified && <Verified sx={{ color: '#00d4aa', fontSize: 16, ml: 0.5 }}/>}
                <Typography variant="h4" sx={{ color: '#FFD700', fontWeight: 'bold', my: 1 }}>#1</Typography>
                {renderReturn(topTraders[0].monthlyReturn)}
                <Typography variant="caption" sx={{ color: '#9ca3af', display: 'block' }}>Monthly Return</Typography>
              </CardContent>
            </Card>

            {/* 3rd Place */}
            <Card sx={{ bgcolor: '#1a1a2e', borderRadius: 3, width: 200, border: '2px solid #CD7F32' }}>
              <CardContent sx={{ textAlign: 'center', pb: 2 }}>
                <Avatar sx={{ width: 60, height: 60, mx: 'auto', mb: 1, bgcolor: '#CD7F32', fontSize: 24 }}>
                  {formatAddress(topTraders[2].trader.address, 2)}
                </Avatar>
                <Typography sx={{ color: '#9ca3af', fontSize: '0.85rem' }}>{topTraders[2].trader.ensName || formatAddress(topTraders[2].trader.address)}</Typography>
                <Typography variant="h5" sx={{ color: '#CD7F32', fontWeight: 'bold', my: 1 }}>#3</Typography>
                {renderReturn(topTraders[2].monthlyReturn)}
                <Typography variant="caption" sx={{ color: '#9ca3af', display: 'block' }}>Monthly</Typography>
              </CardContent>
            </Card>
          </Box>)}

        {/* Tabs */}
        <Card sx={{ bgcolor: '#1a1a2e', borderRadius: 3, mb: 3 }}>
          <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', p: 2, borderBottom: '1px solid #2a2a3e' }}>
            <Tabs value={activeTab} onChange={(_, v) => setActiveTab(v)} sx={{
            '& .MuiTab-root': { color: '#9ca3af' },
            '& .Mui-selected': { color: '#00d4aa' },
        }}>
              <Tab label="All Traders"/>
              <Tab label="Following"/>
              <Tab label="ContentCopying"/>
            </Tabs>
            <FormControl size="small" sx={{ minWidth: 150 }}>
              <Select value={filterPair} onChange={(e) => setFilterPair(e.target.value)} sx={{ color: 'white', bgcolor: '#2a2a3e', '& .MuiOutlinedInput-notchedOutline': { borderColor: '#3a3a4e' } }}>
                <MenuItem value="all">All Pairs</MenuItem>
                <MenuItem value="ETH/USDC">ETH/USDC</MenuItem>
                <MenuItem value="BTC/USDT">BTC/USDT</MenuItem>
                <MenuItem value="ETH/BTC">ETH/BTC</MenuItem>
              </Select>
            </FormControl>
          </Box>

          <CardContent sx={{ p: 0 }}>
            {loading ? (<Box sx={{ display: 'flex', justifyContent: 'center', py: 5 }}>
                <CircularProgress sx={{ color: '#00d4aa' }}/>
              </Box>) : (<TableContainer>
                <Table>
                  <TableHead>
                    <TableRow>
                      <TableCell sx={{ color: '#9ca3af' }}>Rank</TableCell>
                      <TableCell sx={{ color: '#9ca3af' }}>Trader</TableCell>
                      <TableCell sx={{ color: '#9ca3af' }} align="right">Monthly</TableCell>
                      <TableCell sx={{ color: '#9ca3af' }} align="right">Total P&L</TableCell>
                      <TableCell sx={{ color: '#9ca3af' }} align="right">Win Rate</TableCell>
                      <TableCell sx={{ color: '#9ca3af' }} align="right">Followers</TableCell>
                      <TableCell sx={{ color: '#9ca3af' }} align="right">Avg Hold</TableCell>
                      <TableCell sx={{ color: '#9ca3af' }} align="right">Pair</TableCell>
                      <TableCell sx={{ color: '#9ca3af' }} align="right">Actions</TableCell>
                    </TableRow>
                  </TableHead>
                  <TableBody>
                    {filteredLeaderboard.map((entry) => (<TableRow key={entry.trader.id} sx={{ '&:hover': { bgcolor: '#2a2a3e', cursor: 'pointer' } }} onClick={() => { setSelectedTrader(entry.trader); setShowTraderDetail(true); }}>
                        <TableCell>
                          <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'center', width: 40 }}>
                            {renderRankBadge(entry.rank)}
                          </Box>
                        </TableCell>
                        <TableCell>
                          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5 }}>
                            <Avatar sx={{ bgcolor: '#2a2a3e', width: 36, height: 36 }}>
                              {formatAddress(entry.trader.address, 2)}
                            </Avatar>
                            <Box>
                              <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
                                <Typography sx={{ color: 'white', fontWeight: 'bold' }}>
                                  {entry.trader.ensName || formatAddress(entry.trader.address)}
                                </Typography>
                                {entry.trader.isVerified && <Verified sx={{ color: '#00d4aa', fontSize: 16 }}/>}
                              </Box>
                              <Typography variant="caption" sx={{ color: '#9ca3af' }}>
                                {formatAddress(entry.trader.address, 6)}
                              </Typography>
                            </Box>
                          </Box>
                        </TableCell>
                        <TableCell align="right">{renderReturn(entry.monthlyReturn)}</TableCell>
                        <TableCell align="right">{renderPnL(entry.totalPnL)}</TableCell>
                        <TableCell align="right">
                          <Typography sx={{ color: entry.trader.winRate > 60 ? '#00d4aa' : entry.trader.winRate > 50 ? '#ff9800' : '#ff5722', fontWeight: 'bold' }}>
                            {entry.trader.winRate.toFixed(1)}%
                          </Typography>
                        </TableCell>
                        <TableCell align="right">
                          <Typography sx={{ color: '#9ca3af' }}>{formatNumber(entry.trader.followers)}</Typography>
                        </TableCell>
                        <TableCell align="right">
                          <Typography sx={{ color: '#9ca3af' }}>{entry.trader.avgHoldingTime}</Typography>
                        </TableCell>
                        <TableCell align="right">
                          <Chip label={entry.trader.tradingPair} size="small" sx={{ bgcolor: '#2a2a3e' }}/>
                        </TableCell>
                        <TableCell align="right" onClick={(e) => e.stopPropagation()}>
                          <Box sx={{ display: 'flex', gap: 1, justifyContent: 'flex-end' }}>
                            <Button size="small" variant={entry.trader.isCopiable ? 'contained' : 'outlined'} startIcon={<ContentCopy />} onClick={() => { setSelectedTrader(entry.trader); setShowTraderDetail(true); }} sx={{
                    bgcolor: entry.trader.isCopiable ? '#00d4aa' : 'transparent',
                    color: entry.trader.isCopiable ? 'black' : 'white',
                    borderColor: '#3a3a4e',
                    minWidth: 0, px: 1
                }}>
                              ContentCopy
                            </Button>
                            <IconButton size="small" onClick={() => handleFollowTrader(entry.trader)} sx={{ color: entry.trader.isFollowing ? '#00d4aa' : '#9ca3af' }}>
                              {entry.trader.isFollowing ? <Star /> : <Star />}
                            </IconButton>
                          </Box>
                        </TableCell>
                      </TableRow>))}
                  </TableBody>
                </Table>
              </TableContainer>)}
          </CardContent>
        </Card>

        {/* Stats */}
        <Box sx={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: 2 }}>
          <Card sx={{ bgcolor: '#1a1a2e', borderRadius: 3 }}>
            <CardContent>
              <Typography variant="caption" sx={{ color: '#9ca3af' }}>Total Traders</Typography>
              <Typography variant="h5" sx={{ color: 'white', fontWeight: 'bold' }}>{traders.length}</Typography>
            </CardContent>
          </Card>
          <Card sx={{ bgcolor: '#1a1a2e', borderRadius: 3 }}>
            <CardContent>
              <Typography variant="caption" sx={{ color: '#9ca3af' }}>Active Copiers</Typography>
              <Typography variant="h5" sx={{ color: 'white', fontWeight: 'bold' }}>{formatNumber(traders.reduce((sum, t) => sum + t.followers, 0))}</Typography>
            </CardContent>
          </Card>
          <Card sx={{ bgcolor: '#1a1a2e', borderRadius: 3 }}>
            <CardContent>
              <Typography variant="caption" sx={{ color: '#9ca3af' }}>Avg Win Rate</Typography>
              <Typography variant="h5" sx={{ color: '#00d4aa', fontWeight: 'bold' }}>{(traders.reduce((sum, t) => sum + t.winRate, 0) / traders.length).toFixed(1)}%</Typography>
            </CardContent>
          </Card>
          <Card sx={{ bgcolor: '#1a1a2e', borderRadius: 3 }}>
            <CardContent>
              <Typography variant="caption" sx={{ color: '#9ca3af' }}>Total Volume</Typography>
              <Typography variant="h5" sx={{ color: 'white', fontWeight: 'bold' }}>{formatUSD(traders.reduce((sum, t) => sum + t.totalVolume, 0))}</Typography>
            </CardContent>
          </Card>
        </Box>
      </Box>

      {/* Trader Detail Dialog */}
      <Dialog open={showTraderDetail} onClose={() => setShowTraderDetail(false)} maxWidth="md" fullWidth PaperProps={{ sx: { bgcolor: '#1a1a2e', backgroundImage: 'none' } }}>
        {selectedTrader && (<>
            <DialogTitle sx={{ color: 'white', display: 'flex', justifyContent: 'space-between' }}>
              <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
                <Avatar sx={{ width: 56, height: 56, bgcolor: '#2a2a3e' }}>
                  {formatAddress(selectedTrader.address, 2)}
                </Avatar>
                <Box>
                  <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                    <Typography>{selectedTrader.ensName || formatAddress(selectedTrader.address)}</Typography>
                    {selectedTrader.isVerified && <Verified sx={{ color: '#00d4aa' }}/>}
                  </Box>
                  <Typography variant="caption" sx={{ color: '#9ca3af' }}>
                    {selectedTrader.tradingPair} Trader • Last trade {timeAgo(selectedTrader.lastTradeTime)}
                  </Typography>
                </Box>
              </Box>
              <IconButton onClick={() => setShowTraderDetail(false)} sx={{ color: 'white' }}>✕</IconButton>
            </DialogTitle>
            <DialogContent>
              <Box sx={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: 2, mb: 3 }}>
                <Card sx={{ bgcolor: '#2a2a3e', borderRadius: 2 }}>
                  <CardContent>
                    <Typography variant="caption" sx={{ color: '#9ca3af' }}>Total P&L</Typography>
                    <Typography variant="h6" sx={{ color: selectedTrader.totalPnL >= 0 ? '#00d4aa' : '#ff5722', fontWeight: 'bold' }}>
                      {formatUSD(selectedTrader.totalPnL)}
                    </Typography>
                  </CardContent>
                </Card>
                <Card sx={{ bgcolor: '#2a2a3e', borderRadius: 2 }}>
                  <CardContent>
                    <Typography variant="caption" sx={{ color: '#9ca3af' }}>Win Rate</Typography>
                    <Typography variant="h6" sx={{ color: '#00d4aa', fontWeight: 'bold' }}>
                      {selectedTrader.winRate.toFixed(1)}%
                    </Typography>
                  </CardContent>
                </Card>
                <Card sx={{ bgcolor: '#2a2a3e', borderRadius: 2 }}>
                  <CardContent>
                    <Typography variant="caption" sx={{ color: '#9ca3af' }}>Profit Factor</Typography>
                    <Typography variant="h6" sx={{ color: 'white', fontWeight: 'bold' }}>
                      {selectedTrader.profitFactor.toFixed(2)}
                    </Typography>
                  </CardContent>
                </Card>
                <Card sx={{ bgcolor: '#2a2a3e', borderRadius: 2 }}>
                  <CardContent>
                    <Typography variant="caption" sx={{ color: '#9ca3af' }}>Sharpe Ratio</Typography>
                    <Typography variant="h6" sx={{ color: selectedTrader.sharpeRatio > 1.5 ? '#00d4aa' : 'white', fontWeight: 'bold' }}>
                      {selectedTrader.sharpeRatio.toFixed(2)}
                    </Typography>
                  </CardContent>
                </Card>
              </Box>

              <Box sx={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: 2, mb: 3 }}>
                <Card sx={{ bgcolor: '#2a2a3e', borderRadius: 2 }}>
                  <CardContent>
                    <Typography variant="caption" sx={{ color: '#9ca3af' }}>Monthly Return</Typography>
                    <Typography variant="h6" sx={{ color: selectedTrader.monthlyReturn >= 0 ? '#00d4aa' : '#ff5722', fontWeight: 'bold' }}>
                      {formatPercent(selectedTrader.monthlyReturn)}
                    </Typography>
                  </CardContent>
                </Card>
                <Card sx={{ bgcolor: '#2a2a3e', borderRadius: 2 }}>
                  <CardContent>
                    <Typography variant="caption" sx={{ color: '#9ca3af' }}>Max Drawdown</Typography>
                    <Typography variant="h6" sx={{ color: '#ff5722', fontWeight: 'bold' }}>
                      {selectedTrader.maxDrawdown.toFixed(1)}%
                    </Typography>
                  </CardContent>
                </Card>
                <Card sx={{ bgcolor: '#2a2a3e', borderRadius: 2 }}>
                  <CardContent>
                    <Typography variant="caption" sx={{ color: '#9ca3af' }}>Total Trades</Typography>
                    <Typography variant="h6" sx={{ color: 'white', fontWeight: 'bold' }}>
                      {formatNumber(selectedTrader.totalTrades)}
                    </Typography>
                  </CardContent>
                </Card>
                <Card sx={{ bgcolor: '#2a2a3e', borderRadius: 2 }}>
                  <CardContent>
                    <Typography variant="caption" sx={{ color: '#9ca3af' }}>Followers</Typography>
                    <Typography variant="h6" sx={{ color: 'white', fontWeight: 'bold' }}>
                      {formatNumber(selectedTrader.followers)}
                    </Typography>
                  </CardContent>
                </Card>
              </Box>

              <Typography sx={{ color: '#9ca3af', mb: 2 }}>Start ContentCopying</Typography>
              <Box sx={{ display: 'flex', gap: 2 }}>
                <TextField fullWidth type="number" placeholder="Enter amount in USDC" value={copyAmount} onChange={(e) => setContentCopyAmount(e.target.value)} InputProps={{
                startAdornment: <Typography sx={{ color: '#9ca3af', mr: 1 }}>$</Typography>,
            }} sx={{
                '& .MuiInputBase-input': { color: 'white' },
                '& .MuiOutlinedInput-root': { '& fieldset': { borderColor: '#3a3a4e' } },
            }}/>
                <Button variant="contained" startIcon={<ContentCopy />} onClick={() => handleContentCopyTrader(selectedTrader)} disabled={copying || !selectedTrader.isCopiable} sx={{ bgcolor: '#00d4aa', color: 'black', px: 4 }}>
                  {copying ? <CircularProgress size={20} sx={{ color: 'black' }}/> : 'ContentCopy'}
                </Button>
              </Box>
              {!selectedTrader.isCopiable && (<Typography variant="caption" sx={{ color: '#ff9800', mt: 1, display: 'block' }}>
                  This trader is not currently accepting copy positions
                </Typography>)}
            </DialogContent>
            <DialogActions sx={{ p: 3 }}>
              <Button onClick={() => handleFollowTrader(selectedTrader)} sx={{ color: '#00d4aa' }}>
                {selectedTrader.isFollowing ? 'Unfollow' : 'Follow'}
              </Button>
              <Button onClick={() => setShowTraderDetail(false)} sx={{ color: '#9ca3af' }}>Close</Button>
            </DialogActions>
          </>)}
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