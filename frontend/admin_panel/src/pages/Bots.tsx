import React, { useState, useEffect } from 'react';
import {
  Box, Typography, Grid, Card, CardContent, Button, Table, TableBody,
  TableCell, TableContainer, TableHead, TableRow, Chip, IconButton,
  Dialog, DialogTitle, DialogContent, DialogActions, TextField, Select,
  MenuItem, FormControl, InputLabel, Switch, FormControlLabel, Tabs, Tab,
  LinearProgress, Avatar, Badge, Tooltip, Paper, Divider, Alert
} from '@mui/material';
import {
  PlayArrow, Stop, Settings, Delete, Add, Refresh, ShowChart,
  TrendingUp, TrendingDown, Speed, AccountBalanceWallet, Build,
  Flag, CheckCircle, Error, Warning
} from '@mui/icons-material';

// ============================================================================
// Bot Types Enum (matching Rust backend)
// ============================================================================
const BOT_TYPES = [
  { value: 'MarketMaker', label: 'Market Maker', fee: 5000, desc: 'Earn spread from liquidity provision' },
  { value: 'Arbitrage', label: 'Arbitrage', fee: 3000, desc: 'Cross-exchange price differences' },
  { value: 'Sniper', label: 'Sniper', fee: 2500, desc: 'Ultra-fast trade execution' },
  { value: 'Liquidity', label: 'Liquidity Provider', fee: 2500, desc: 'Deep order books' },
  { value: 'MevBot', label: 'MEV Bot', fee: 2500, desc: 'Mempool extraction' },
  { value: 'Sandwich', label: 'Sandwich Bot', fee: 2500, desc: 'Trade wrapping' },
  { value: 'FlashLoan', label: 'Flash Loan', fee: 2500, desc: 'Risk-free leverage' },
  { value: 'CrossChain', label: 'Cross-Chain', fee: 3000, desc: 'Bridge arbitrage' },
  { value: 'PerpHedge', label: 'Perp Hedge', fee: 2500, desc: 'Perpetual hedging' },
  { value: 'FrontRun', label: 'Front Run', fee: 2500, desc: 'Order anticipation' },
];

// Top 20 DEXs (matching Rust backend)
const TOP_DEXS = [
  'uniswap_v4', 'uniswap_v3', 'pancakeswap_v4', 'curve_finance', 'sushiswap',
  'hyperliquid', 'dydx_v4', 'jupiter', 'raydium', 'orca', 'balancer_v2',
  '1inch', 'odos', 'maverick', 'velodrome_v3', 'aerodrome', 'woofi',
  'spirit_swap', 'spookyswap'
];

// ============================================================================
// Bot Interface
// ============================================================================
interface Bot {
  id: string;
  name: string;
  type: string;
  status: 'running' | 'stopped' | 'error';
  pnl: number;
  volume: number;
  orders: number;
  avgLatency: number;
  connectedDexs: number;
  connectedCexs: number;
  fee: number;
  createdAt: string;
  lastTrade: string;
}

// ============================================================================
// Mock Data
// ============================================================================
const initialBots: Bot[] = [
  {
    id: 'bot_001',
    name: 'ETH Market Maker',
    type: 'MarketMaker',
    status: 'running',
    pnl: 5420.50,
    volume: 1250000,
    orders: 1247,
    avgLatency: 850,
    connectedDexs: 20,
    connectedCexs: 200,
    fee: 5000,
    createdAt: '2024-01-15',
    lastTrade: '2 min ago',
  },
  {
    id: 'bot_002',
    name: 'BTC Arbitrage',
    type: 'Arbitrage',
    status: 'running',
    pnl: 3210.75,
    volume: 890000,
    orders: 456,
    avgLatency: 1200,
    connectedDexs: 20,
    connectedCexs: 200,
    fee: 3000,
    createdAt: '2024-01-16',
    lastTrade: '5 min ago',
  },
  {
    id: 'bot_003',
    name: 'SOL Sniper',
    type: 'Sniper',
    status: 'stopped',
    pnl: -120.30,
    volume: 45000,
    orders: 89,
    avgLatency: 320,
    connectedDexs: 20,
    connectedCexs: 200,
    fee: 2500,
    createdAt: '2024-01-17',
    lastTrade: '1 hour ago',
  },
];

// ============================================================================
// Main Component
// ============================================================================
const Bots: React.FC = () => {
  const [bots, setBots] = useState<Bot[]>(initialBots);
  const [tab, setTab] = useState(0);
  const [createDialogOpen, setCreateDialogOpen] = useState(false);
  const [settingsDialogOpen, setSettingsDialogOpen] = useState(false);
  const [selectedBot, setSelectedBot] = useState<Bot | null>(null);

  // Form state
  const [newBotName, setNewBotName] = useState('');
  const [newBotType, setNewBotType] = useState('MarketMaker');
  const [selectedDexs, setSelectedDexs] = useState<string[]>(TOP_DEXS);
  const [selectedCexs, setSelectedCexs] = useState<string[]>([]);
  const [maxPosition, setMaxPosition] = useState(100000);
  const [maxDailyLoss, setMaxDailyLoss] = useState(5000);
  const [latencyTarget, setLatencyTarget] = useState(5000);

  const handleTabChange = (_event: React.SyntheticEvent, newValue: number) => {
    setTab(newValue);
  };

  const handleStartBot = (botId: string) => {
    setBots(prev => prev.map(b => 
      b.id === botId ? { ...b, status: 'running' as const } : b
    ));
  };

  const handleStopBot = (botId: string) => {
    setBots(prev => prev.map(b => 
      b.id === botId ? { ...b, status: 'stopped' as const } : b
    ));
  };

  const handleCreateBot = () => {
    const botType = BOT_TYPES.find(t => t.value === newBotType);
    const newBot: Bot = {
      id: `bot_${Date.now()}`,
      name: newBotName || `${botType?.label} Bot`,
      type: newBotType,
      status: 'stopped',
      pnl: 0,
      volume: 0,
      orders: 0,
      avgLatency: 0,
      connectedDexs: selectedDexs.length,
      connectedCexs: selectedCexs.length > 0 ? selectedCexs.length : 200,
      fee: botType?.fee || 2500,
      createdAt: new Date().toISOString().split('T')[0],
      lastTrade: 'Never',
    };
    setBots(prev => [...prev, newBot]);
    setCreateDialogOpen(false);
    resetForm();
  };

  const handleDeleteBot = (botId: string) => {
    if (confirm('Are you sure you want to delete this bot?')) {
      setBots(prev => prev.filter(b => b.id !== botId));
    }
  };

  const handleOpenSettings = (bot: Bot) => {
    setSelectedBot(bot);
    setSettingsDialogOpen(true);
  };

  const resetForm = () => {
    setNewBotName('');
    setNewBotType('MarketMaker');
    setSelectedDexs(TOP_DEXS);
    setSelectedCexs([]);
    setMaxPosition(100000);
    setMaxDailyLoss(5000);
    setLatencyTarget(5000);
  };

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'running': return 'success';
      case 'stopped': return 'default';
      case 'error': return 'error';
      default: return 'default';
    }
  };

  const getLatencyColor = (latency: number) => {
    if (latency < 500) return 'success';
    if (latency < 2000) return 'warning';
    return 'error';
  };

  // Calculate totals
  const totalPnl = bots.reduce((sum, b) => sum + b.pnl, 0);
  const totalVolume = bots.reduce((sum, b) => sum + b.volume, 0);
  const totalOrders = bots.reduce((sum, b) => sum + b.orders, 0);
  const avgLatency = bots.length > 0 ? bots.reduce((sum, b) => sum + b.avgLatency, 0) / bots.length : 0;
  const runningBots = bots.filter(b => b.status === 'running').length;

  return (
    <Box sx={{ p: 3 }}>
      {/* Header */}
      <Box sx={{ display: 'flex', justifyContent: 'space-between', mb: 3 }}>
        <Box>
          <Typography variant="h4" sx={{ fontWeight: 'bold' }}>Bot Management</Typography>
          <Typography variant="body2" color="text.secondary">
            Manage all trading bots, strategies, and monitor performance
          </Typography>
        </Box>
        <Box sx={{ display: 'flex', gap: 2 }}>
          <Button variant="outlined" startIcon={<Refresh />}>Sync</Button>
          <Button 
            variant="contained" 
            startIcon={<Add />}
            onClick={() => setCreateDialogOpen(true)}
          >
            Create Bot
          </Button>
        </Box>
      </Box>

      {/* Stats Cards */}
      <Grid container spacing={3} sx={{ mb: 3 }}>
        <Grid item xs={12} sm={6} md={3}>
          <Card>
            <CardContent>
              <Box sx={{ display: 'flex', alignItems: 'center' }}>
                <Avatar sx={{ bgcolor: 'primary.main', mr: 2 }}>
                  <Speed />
                </Avatar>
                <Box>
                  <Typography variant="body2" color="text.secondary">Running Bots</Typography>
                  <Typography variant="h4">{runningBots}/{bots.length}</Typography>
                </Box>
              </Box>
            </CardContent>
          </Card>
        </Grid>
        <Grid item xs={12} sm={6} md={3}>
          <Card>
            <CardContent>
              <Box sx={{ display: 'flex', alignItems: 'center' }}>
                <Avatar sx={{ bgcolor: totalPnl >= 0 ? 'success.main' : 'error.main', mr: 2 }}>
                  {totalPnl >= 0 ? <TrendingUp /> : <TrendingDown />}
                </Avatar>
                <Box>
                  <Typography variant="body2" color="text.secondary">Total PnL</Typography>
                  <Typography variant="h4" sx={{ color: totalPnl >= 0 ? 'success.main' : 'error.main' }}>
                    ${totalPnl.toLocaleString()}
                  </Typography>
                </Box>
              </Box>
            </CardContent>
          </Card>
        </Grid>
        <Grid item xs={12} sm={6} md={3}>
          <Card>
            <CardContent>
              <Box sx={{ display: 'flex', alignItems: 'center' }}>
                <Avatar sx={{ bgcolor: 'info.main', mr: 2 }}>
                  <AccountBalanceWallet />
                </Avatar>
                <Box>
                  <Typography variant="body2" color="text.secondary">Total Volume</Typography>
                  <Typography variant="h4">${totalVolume.toLocaleString()}</Typography>
                </Box>
              </Box>
            </CardContent>
          </Card>
        </Grid>
        <Grid item xs={12} sm={6} md={3}>
          <Card>
            <CardContent>
              <Box sx={{ display: 'flex', alignItems: 'center' }}>
                <Avatar sx={{ bgcolor: 'warning.main', mr: 2 }}>
                  <ShowChart />
                </Avatar>
                <Box>
                  <Typography variant="body2" color="text.secondary">Avg Latency</Typography>
                  <Typography variant="h4">{Math.round(avgLatency)}μs</Typography>
                </Box>
              </Box>
            </CardContent>
          </Card>
        </Grid>
      </Grid>

      {/* Fee Structure Alert */}
      <Alert severity="info" sx={{ mb: 3 }}>
        <Typography variant="body2">
          <strong>Fee Structure:</strong> Market Maker Bot: $5,000/month + $1,000 per exchange | 
          Other Bots: $2,500/month + $500 per exchange. All bots include full features and connectivity.
        </Typography>
      </Alert>

      {/* Tabs */}
      <Tabs value={tab} onChange={handleTabChange} sx={{ mb: 2 }}>
        <Tab label={`All Bots (${bots.length})`} />
        <Tab label={`Running (${runningBots})`} />
        <Tab label="Stopped" />
        <Tab label="Performance" />
      </Tabs>

      {/* Bot Table */}
      <TableContainer component={Paper}>
        <Table>
          <TableHead>
            <TableRow sx={{ bgcolor: 'grey[100]' }}>
              <TableCell><strong>Bot</strong></TableCell>
              <TableCell><strong>Type</strong></TableCell>
              <TableCell><strong>Status</strong></TableCell>
              <TableCell><strong>PnL</strong></TableCell>
              <TableCell><strong>Volume</strong></TableCell>
              <TableCell><strong>Orders</strong></TableCell>
              <TableCell><strong>Latency</strong></TableCell>
              <TableCell><strong>Exchanges</strong></TableCell>
              <TableCell><strong>Fee/Month</strong></TableCell>
              <TableCell><strong>Actions</strong></TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {bots
              .filter(b => {
                if (tab === 1) return b.status === 'running';
                if (tab === 2) return b.status === 'stopped';
                return true;
              })
              .map((bot) => (
                <TableRow key={bot.id} hover>
                  <TableCell>
                    <Box>
                      <Typography variant="subtitle1" sx={{ fontWeight: 'bold' }}>
                        {bot.name}
                      </Typography>
                      <Typography variant="caption" color="text.secondary">
                        ID: {bot.id} | Created: {bot.createdAt}
                      </Typography>
                    </Box>
                  </TableCell>
                  <TableCell>
                    <Chip 
                      label={BOT_TYPES.find(t => t.value === bot.type)?.label || bot.type}
                      size="small"
                      variant="outlined"
                    />
                  </TableCell>
                  <TableCell>
                    <Chip 
                      icon={bot.status === 'running' ? <CheckCircle /> : 
                            bot.status === 'error' ? <Error /> : <Warning />}
                      label={bot.status.toUpperCase()}
                      color={getStatusColor(bot.status)}
                      size="small"
                    />
                  </TableCell>
                  <TableCell>
                    <Typography 
                      variant="body2" 
                      sx={{ 
                        color: bot.pnl >= 0 ? 'success.main' : 'error.main',
                        fontWeight: 'bold'
                      }}
                    >
                      ${bot.pnl.toLocaleString()}
                    </Typography>
                  </TableCell>
                  <TableCell>
                    ${bot.volume.toLocaleString()}
                  </TableCell>
                  <TableCell>
                    {bot.orders.toLocaleString()}
                  </TableCell>
                  <TableCell>
                    <Chip 
                      label={`${bot.avgLatency}μs`}
                      color={getLatencyColor(bot.avgLatency)}
                      size="small"
                    />
                  </TableCell>
                  <TableCell>
                    <Tooltip title={`${bot.connectedDexs} DEXs, ${bot.connectedCexs} CEXs`}>
                      <Chip 
                        label={`${bot.connectedDexs}/${bot.connectedCexs}`}
                        size="small"
                        variant="outlined"
                      />
                    </Tooltip>
                  </TableCell>
                  <TableCell>
                    <Typography variant="body2" sx={{ fontWeight: 'bold' }}>
                      ${bot.fee}
                    </Typography>
                  </TableCell>
                  <TableCell>
                    <Box sx={{ display: 'flex', gap: 0.5 }}>
                      {bot.status === 'running' ? (
                        <Tooltip title="Stop Bot">
                          <IconButton 
                            color="error" 
                            size="small"
                            onClick={() => handleStopBot(bot.id)}
                          >
                            <Stop />
                          </IconButton>
                        </Tooltip>
                      ) : (
                        <Tooltip title="Start Bot">
                          <IconButton 
                            color="success" 
                            size="small"
                            onClick={() => handleStartBot(bot.id)}
                          >
                            <PlayArrow />
                          </IconButton>
                        </Tooltip>
                      )}
                      <Tooltip title="Settings">
                        <IconButton 
                          color="info" 
                          size="small"
                          onClick={() => handleOpenSettings(bot)}
                        >
                          <Settings />
                        </IconButton>
                      </Tooltip>
                      <Tooltip title="Delete">
                        <IconButton 
                          color="error" 
                          size="small"
                          onClick={() => handleDeleteBot(bot.id)}
                        >
                          <Delete />
                        </IconButton>
                      </Tooltip>
                    </Box>
                  </TableCell>
                </TableRow>
              ))}
          </TableBody>
        </Table>
      </TableContainer>

      {/* Create Bot Dialog */}
      <Dialog open={createDialogOpen} onClose={() => setCreateDialogOpen(false)} maxWidth="md" fullWidth>
        <DialogTitle>Create New Trading Bot</DialogTitle>
        <DialogContent>
          <Grid container spacing={2} sx={{ mt: 1 }}>
            <Grid item xs={12}>
              <TextField
                fullWidth
                label="Bot Name"
                value={newBotName}
                onChange={(e) => setNewBotName(e.target.value)}
                placeholder="e.g., ETH Market Maker"
              />
            </Grid>
            <Grid item xs={12} md={6}>
              <FormControl fullWidth>
                <InputLabel>Bot Type</InputLabel>
                <Select
                  value={newBotType}
                  label="Bot Type"
                  onChange={(e) => setNewBotType(e.target.value)}
                >
                  {BOT_TYPES.map((type) => (
                    <MenuItem key={type.value} value={type.value}>
                      <Box>
                        <Typography variant="body2">{type.label}</Typography>
                        <Typography variant="caption" color="text.secondary">
                          ${type.fee}/month - {type.desc}
                        </Typography>
                      </Box>
                    </MenuItem>
                  ))}
                </Select>
              </FormControl>
            </Grid>
            <Grid item xs={12} md={6}>
              <FormControl fullWidth>
                <InputLabel>Latency Target (μs)</InputLabel>
                <Select
                  value={latencyTarget}
                  label="Latency Target (μs)"
                  onChange={(e) => setLatencyTarget(Number(e.target.value))}
                >
                  <MenuItem value={500}>Ultra Low: 500μs</MenuItem>
                  <MenuItem value={1000}>Low: 1ms</MenuItem>
                  <MenuItem value={5000}>Standard: 5ms</MenuItem>
                  <MenuItem value={10000}>High: 10ms</MenuItem>
                </Select>
              </FormControl>
            </Grid>
            <Grid item xs={12} md={6}>
              <TextField
                fullWidth
                type="number"
                label="Max Position (USD)"
                value={maxPosition}
                onChange={(e) => setMaxPosition(Number(e.target.value))}
              />
            </Grid>
            <Grid item xs={12} md={6}>
              <TextField
                fullWidth
                type="number"
                label="Max Daily Loss (USD)"
                value={maxDailyLoss}
                onChange={(e) => setMaxDailyLoss(Number(e.target.value))}
              />
            </Grid>
            <Grid item xs={12}>
              <FormControlLabel
                control={<Switch checked={selectedDexs.length === TOP_DEXS.length} />}
                label="Connect to All 20 Top DEXs"
                onChange={(_, checked) => {
                  if (checked) setSelectedDexs(TOP_DEXS);
                  else setSelectedDexs([]);
                }}
              />
            </Grid>
            <Grid item xs={12}>
              <FormControlLabel
                control={<Switch checked={selectedCexs.length === 200} />}
                label="Connect to All 200 CEXs"
                onChange={(_, checked) => {
                  if (checked) setSelectedCexs(Array.from({length: 200}, (_, i) => `cex_${i}`));
                  else setSelectedCexs([]);
                }}
              />
            </Grid>
            <Grid item xs={12}>
              <Alert severity="info">
                Fee: ${BOT_TYPES.find(t => t.value === newBotType)?.fee || 2500}/month + 
                $1000 per DEX + $1000 per CEX (for MM bot) or $500 per exchange (other bots)
              </Alert>
            </Grid>
          </Grid>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setCreateDialogOpen(false)}>Cancel</Button>
          <Button variant="contained" onClick={handleCreateBot}>Create Bot</Button>
        </DialogActions>
      </Dialog>

      {/* Settings Dialog */}
      <Dialog open={settingsDialogOpen} onClose={() => setSettingsDialogOpen(false)} maxWidth="sm" fullWidth>
        <DialogTitle>Bot Settings: {selectedBot?.name}</DialogTitle>
        <DialogContent>
          {selectedBot && (
            <Grid container spacing={2} sx={{ mt: 1 }}>
              <Grid item xs={12}>
                <Typography variant="subtitle2" color="text.secondary">
                  Bot ID: {selectedBot.id}
                </Typography>
              </Grid>
              <Grid item xs={12} md={6}>
                <TextField
                  fullWidth
                  type="number"
                  label="Max Position (USD)"
                  defaultValue={maxPosition}
                />
              </Grid>
              <Grid item xs={12} md={6}>
                <TextField
                  fullWidth
                  type="number"
                  label="Max Daily Loss (USD)"
                  defaultValue={maxDailyLoss}
                />
              </Grid>
              <Grid item xs={12}>
                <Typography variant="body2" sx={{ mb: 1 }}>
                  Connected Exchanges:
                </Typography>
                <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 0.5 }}>
                  <Chip label={`${selectedBot.connectedDexs} DEXs`} size="small" />
                  <Chip label={`${selectedBot.connectedCexs} CEXs`} size="small" />
                </Box>
              </Grid>
            </Grid>
          )}
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setSettingsDialogOpen(false)}>Cancel</Button>
          <Button variant="contained">Save Settings</Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
};

export default Bots;
