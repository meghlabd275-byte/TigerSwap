'use client';
import React, { useState, useEffect, useCallback } from 'react';
import { Box, Typography, Card, CardContent, Button, TextField, Tabs, Tab, Chip, Slider, Divider, List, ListItem, ListItemText, Alert, Select, MenuItem, FormControl, InputLabel, ToggleButton, ToggleButtonGroup, Dialog, DialogTitle, DialogContent, DialogActions, Table, TableBody, TableCell, TableContainer, TableHead, TableRow, Paper } from '@mui/material';
import { ShowChart, TrendingUp, TrendingDown, Speed, AccountBalance, SwapHoriz, Timer, Warning } from '@mui/icons-material';
// ============================================================================
// Constants
// ============================================================================
const ORDER_TYPES = [
    { type: 'market', label: 'Market', icon: <SwapHoriz /> },
    { type: 'limit', label: 'Limit', icon: <Speed /> },
    { type: 'stop_loss', label: 'Stop Loss', icon: <Warning /> },
    { type: 'take_profit', label: 'Take Profit', icon: <TrendingUp /> },
    { type: 'twap', label: 'TWAP', icon: <Timer /> },
    { type: 'trailing', label: 'Trailing', icon: <ShowChart /> },
];
const POPULAR_PAIRS = [
    { symbol: 'ETH/USDC', base: 'ETH', quote: 'USDC' },
    { symbol: 'BTC/USDC', base: 'BTC', quote: 'USDC' },
    { symbol: 'SOL/USDC', base: 'SOL', quote: 'USDC' },
    { symbol: 'WIF/USDC', base: 'WIF', quote: 'USDC' },
    { symbol: 'PEPE/USDC', base: 'PEPE', quote: 'USDC' },
];
// ============================================================================
// Component
// ============================================================================
export default function AdvancedTradingInterface() {
    // State
    const [activeTab, setActiveTab] = useState(0);
    const [selectedPair, setSelectedPair] = useState('ETH/USDC');
    const [orderType, setOrderType] = useState('limit');
    const [side, setSide] = useState('buy');
    const [price, setPrice] = useState('');
    const [amount, setAmount] = useState('');
    const [stopPrice, setStopPrice] = useState('');
    const [limitPrice, setLimitPrice] = useState('');
    const [twapIntervals, setTwapIntervals] = useState(10);
    const [twapIntervalMinutes, setTwapIntervalMinutes] = useState(60);
    const [trailingDistance, setTrailingDistance] = useState('2');
    const [slippage, setSlippage] = useState('0.5');
    const [deadline, setDeadline] = useState(30);
    // Order book
    const [bids, setBids] = useState([]);
    const [asks, setAsks] = useState([]);
    // Positions & Orders
    const [positions, setPositions] = useState([]);
    const [openOrders, setOpenOrders] = useState([]);
    const [orderHistory, setOrderHistory] = useState([]);
    // Concentrated liquidity
    const [concentratedPositions, setConcentratedPositions] = useState([]);
    const [selectedRange, setSelectedRange] = useState({ lower: 0, upper: 0 });
    // UI State
    const [orderDialogOpen, setOrderDialogOpen] = useState(false);
    const [confirmDialogOpen, setConfirmDialogOpen] = useState(false);
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState(null);
    const [success, setSuccess] = useState(null);
    // ============================================================================
    // Effects
    // ============================================================================
    useEffect(() => {
        // Load order book
        loadOrderBook();
        // Load positions and orders
        loadPositions();
        loadOrders();
        // Load concentrated positions
        loadConcentratedPositions();
    }, [selectedPair]);
    // ============================================================================
    // Data Loading
    // ============================================================================
    const loadOrderBook = useCallback(async () => {
        // Mock order book data
        const mockBids = [];
        const mockAsks = [];
        const basePrice = 3500;
        for (let i = 0; i < 15; i++) {
            mockBids.push({
                price: basePrice - i * 0.5,
                amount: Math.random() * 10 + 1,
                total: 0,
            });
            mockAsks.push({
                price: basePrice + (i + 1) * 0.5,
                amount: Math.random() * 10 + 1,
                total: 0,
            });
        }
        // Calculate totals
        let bidTotal = 0;
        let askTotal = 0;
        mockBids.forEach(b => { bidTotal += b.amount; b.total = bidTotal; });
        mockAsks.forEach(a => { askTotal += a.amount; a.total = askTotal; });
        setBids(mockBids);
        setAsks(mockAsks);
    }, [selectedPair]);
    const loadPositions = useCallback(async () => {
        // Mock positions
        setPositions([
            {
                id: '1',
                pair: 'ETH/USDC',
                side: 'long',
                size: 1.5,
                entryPrice: 3200,
                currentPrice: 3500,
                pnl: 450,
                pnlPercent: 9.4,
                margin: 1000,
                leverage: 5,
                liquidationPrice: 2800,
            },
        ]);
    }, []);
    const loadOrders = useCallback(async () => {
        // Mock open orders
        setOpenOrders([
            {
                id: '1',
                type: 'limit',
                pair: 'ETH/USDC',
                side: 'buy',
                price: 3400,
                amount: 2,
                filled: 0,
                status: 'pending',
                createdAt: Date.now() - 3600000,
                expiresAt: Date.now() + 86400000,
            },
            {
                id: '2',
                type: 'stop_loss',
                pair: 'ETH/USDC',
                side: 'sell',
                price: 3300,
                amount: 1,
                filled: 0,
                status: 'pending',
                createdAt: Date.now() - 1800000,
                expiresAt: Date.now() + 172800000,
                triggerPrice: 3300,
            },
        ]);
        // Mock order history
        setOrderHistory([
            {
                id: '3',
                type: 'twap',
                pair: 'BTC/USDC',
                side: 'buy',
                price: 65000,
                amount: 5,
                filled: 5,
                status: 'filled',
                createdAt: Date.now() - 86400000,
                expiresAt: Date.now() - 3600000,
                interval: 3600,
                numIntervals: 10,
            },
        ]);
    }, []);
    const loadConcentratedPositions = useCallback(async () => {
        // Mock concentrated positions
        setConcentratedPositions([
            {
                id: 1,
                token0: 'ETH',
                token1: 'USDC',
                tickLower: 20000,
                tickUpper: 30000,
                liquidity: 50000,
                feesEarned: 125.5,
                apr: 24.5,
            },
        ]);
    }, []);
    // ============================================================================
    // Order Actions
    // ============================================================================
    const handleSubmitOrder = useCallback(async () => {
        if (!price || !amount) {
            setError('Please enter price and amount');
            return;
        }
        setConfirmDialogOpen(true);
    }, [price, amount, orderType]);
    const confirmOrder = useCallback(async () => {
        setLoading(true);
        setError(null);
        setSuccess(null);
        try {
            // Submit order based on type
            let orderData = {
                pair: selectedPair,
                side,
                amount: parseFloat(amount),
            };
            switch (orderType) {
                case 'limit':
                    orderData.price = parseFloat(price);
                    orderData.type = 'limit';
                    break;
                case 'stop_loss':
                    orderData.stopPrice = parseFloat(stopPrice);
                    orderData.type = 'stop_loss';
                    break;
                case 'take_profit':
                    orderData.targetPrice = parseFloat(price);
                    orderData.type = 'take_profit';
                    break;
                case 'twap':
                    orderData.intervals = twapIntervals;
                    orderData.intervalMinutes = twapIntervalMinutes;
                    orderData.type = 'twap';
                    break;
                case 'trailing':
                    orderData.trailingDistance = parseFloat(trailingDistance);
                    orderData.type = 'trailing_stop';
                    break;
                default:
                    orderData.type = 'market';
            }
            // Simulate API call
            await new Promise(resolve => setTimeout(resolve, 1000));
            setSuccess(`Order placed successfully: ${side.toUpperCase()} ${amount} ${selectedPair} @ ${orderType}`);
            setConfirmDialogOpen(false);
            // Reload orders
            loadOrders();
        }
        catch (err) {
            setError(err.message || 'Order failed');
        }
        finally {
            setLoading(false);
        }
    }, [selectedPair, side, amount, price, orderType, stopPrice, twapIntervals, twapIntervalMinutes, trailingDistance, loadOrders]);
    const handleCancelOrder = useCallback(async (orderId) => {
        setLoading(true);
        try {
            await new Promise(resolve => setTimeout(resolve, 500));
            setSuccess('Order cancelled');
            loadOrders();
        }
        catch (err) {
            setError(err.message);
        }
        finally {
            setLoading(false);
        }
    }, [loadOrders]);
    const handleClosePosition = useCallback(async (positionId) => {
        setLoading(true);
        try {
            await new Promise(resolve => setTimeout(resolve, 1000));
            setSuccess('Position closed with profit');
            loadPositions();
        }
        catch (err) {
            setError(err.message);
        }
        finally {
            setLoading(false);
        }
    }, [loadPositions]);
    // ============================================================================
    // Render Helpers
    // ============================================================================
    const formatPrice = (price) => {
        return price.toLocaleString('en-US', {
            minimumFractionDigits: 2,
            maximumFractionDigits: 2,
        });
    };
    const formatPercent = (percent) => {
        const sign = percent >= 0 ? '+' : '';
        return `${sign}${percent.toFixed(2)}%`;
    };
    // ============================================================================
    // Render
    // ============================================================================
    return (<Box sx={{ p: 3 }}>
      {/* Header */}
      <Box sx={{ mb: 3, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <Typography variant="h5" fontWeight="bold">
          Advanced Trading
        </Typography>

        <FormControl size="small" sx={{ minWidth: 150 }}>
          <InputLabel>Trading Pair</InputLabel>
          <Select value={selectedPair} label="Trading Pair" onChange={(e) => setSelectedPair(e.target.value)}>
            {POPULAR_PAIRS.map(pair => (<MenuItem key={pair.symbol} value={pair.symbol}>
                {pair.symbol}
              </MenuItem>))}
          </Select>
        </FormControl>
      </Box>

      {/* Alerts */}
      {error && (<Alert severity="error" sx={{ mb: 2 }} onClose={() => setError(null)}>
          {error}
        </Alert>)}
      {success && (<Alert severity="success" sx={{ mb: 2 }} onClose={() => setSuccess(null)}>
          {success}
        </Alert>)}

      {/* Tabs */}
      <Tabs value={activeTab} onChange={(_, v) => setActiveTab(v)} sx={{ mb: 3 }}>
        <Tab label="Trade" icon={<SwapHoriz />} iconPosition="start"/>
        <Tab label="Order Book" icon={<ShowChart />} iconPosition="start"/>
        <Tab label="Positions" icon={<AccountBalance />} iconPosition="start"/>
        <Tab label="Orders" icon={<Speed />} iconPosition="start"/>
        <Tab label="Concentrated" icon={<TrendingUp />} iconPosition="start"/>
      </Tabs>

      {/* Trade Tab */}
      {activeTab === 0 && (<Box sx={{ display: 'flex', gap: 3 }}>
          {/* Order Form */}
          <Card sx={{ flex: 1 }}>
            <CardContent>
              <Typography variant="h6" gutterBottom>Place Order</Typography>

              {/* Buy/Sell Toggle */}
              <ToggleButtonGroup value={side} exclusive onChange={(_, v) => v && setSide(v)} fullWidth sx={{ mb: 3 }}>
                <ToggleButton value="buy" sx={{
                bgcolor: side === 'buy' ? 'success.main' : 'transparent',
                color: side === 'buy' ? 'white' : 'text.primary',
                '&:hover': { bgcolor: side === 'buy' ? 'success.dark' : 'action.hover' }
            }}>
                  <TrendingUp sx={{ mr: 1 }}/> BUY
                </ToggleButton>
                <ToggleButton value="sell" sx={{
                bgcolor: side === 'sell' ? 'error.main' : 'transparent',
                color: side === 'sell' ? 'white' : 'text.primary',
                '&:hover': { bgcolor: side === 'sell' ? 'error.dark' : 'action.hover' }
            }}>
                  <TrendingDown sx={{ mr: 1 }}/> SELL
                </ToggleButton>
              </ToggleButtonGroup>

              {/* Order Type */}
              <FormControl fullWidth sx={{ mb: 2 }}>
                <InputLabel>Order Type</InputLabel>
                <Select value={orderType} label="Order Type" onChange={(e) => setOrderType(e.target.value)}>
                  {ORDER_TYPES.map(ot => (<MenuItem key={ot.type} value={ot.type}>
                      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                        {ot.icon} {ot.label}
                      </Box>
                    </MenuItem>))}
                </Select>
              </FormControl>

              {/* Price Input */}
              {(orderType === 'limit' || orderType === 'take_profit') && (<TextField fullWidth label="Price" type="number" value={price} onChange={(e) => setPrice(e.target.value)} sx={{ mb: 2 }} InputProps={{
                    endAdornment: <Chip label={selectedPair.split('/')[1]} size="small"/>,
                }}/>)}

              {/* Stop Price */}
              {(orderType === 'stop_loss' || orderType === 'stop_loss') && (<TextField fullWidth label="Stop Price" type="number" value={stopPrice} onChange={(e) => setStopPrice(e.target.value)} sx={{ mb: 2 }} InputProps={{
                    endAdornment: <Chip label={selectedPair.split('/')[1]} size="small"/>,
                }}/>)}

              {/* Limit Price for Stop-Limit */}
              {orderType === 'stop_loss' && (<TextField fullWidth label="Limit Price" type="number" value={limitPrice} onChange={(e) => setLimitPrice(e.target.value)} sx={{ mb: 2 }} InputProps={{
                    endAdornment: <Chip label={selectedPair.split('/')[1]} size="small"/>,
                }}/>)}

              {/* TWAP Settings */}
              {orderType === 'twap' && (<Box sx={{ mb: 2 }}>
                  <Typography variant="subtitle2" gutterBottom>TWAP Settings</Typography>
                  <Box sx={{ display: 'flex', gap: 2 }}>
                    <TextField fullWidth label="Intervals" type="number" value={twapIntervals} onChange={(e) => setTwapIntervals(parseInt(e.target.value))}/>
                    <TextField fullWidth label="Minutes Between" type="number" value={twapIntervalMinutes} onChange={(e) => setTwapIntervalMinutes(parseInt(e.target.value))}/>
                  </Box>
                </Box>)}

              {/* Trailing Stop Settings */}
              {orderType === 'trailing' && (<TextField fullWidth label="Trailing Distance (%)" type="number" value={trailingDistance} onChange={(e) => setTrailingDistance(e.target.value)} sx={{ mb: 2 }}/>)}

              {/* Amount */}
              <TextField fullWidth label="Amount" type="number" value={amount} onChange={(e) => setAmount(e.target.value)} sx={{ mb: 2 }} InputProps={{
                endAdornment: <Chip label={selectedPair.split('/')[0]} size="small"/>,
            }}/>

              {/* Slippage */}
              <Box sx={{ mb: 2 }}>
                <Typography variant="subtitle2" gutterBottom>Slippage Tolerance</Typography>
                <Slider value={parseFloat(slippage)} onChange={(_, v) => setSlippage(v.toString())} min={0.1} max={10} step={0.1} marks={[
                { value: 0.1, label: '0.1%' },
                { value: 0.5, label: '0.5%' },
                { value: 1, label: '1%' },
                { value: 5, label: '5%' },
            ]}/>
              </Box>

              {/* Submit Button */}
              <Button fullWidth variant="contained" size="large" onClick={handleSubmitOrder} disabled={loading || !price || !amount} sx={{
                bgcolor: side === 'buy' ? 'success.main' : 'error.main',
                '&:hover': {
                    bgcolor: side === 'buy' ? 'success.dark' : 'error.dark',
                },
            }}>
                {side === 'buy' ? 'Buy' : 'Sell'} {selectedPair}
              </Button>
            </CardContent>
          </Card>

          {/* Order Summary */}
          <Card sx={{ width: 350 }}>
            <CardContent>
              <Typography variant="h6" gutterBottom>Order Summary</Typography>

              <Divider sx={{ my: 2 }}/>

              <List dense>
                <ListItem>
                  <ListItemText primary="Order Type" secondary={orderType}/>
                </ListItem>
                <ListItem>
                  <ListItemText primary="Price" secondary={price || 'Market'}/>
                </ListItem>
                <ListItem>
                  <ListItemText primary="Amount" secondary={amount || '-'}/>
                </ListItem>
                <ListItem>
                  <ListItemText primary="Subtotal" secondary={price && amount ? `$${(parseFloat(price) * parseFloat(amount)).toLocaleString()}` : '-'}/>
                </ListItem>
                <ListItem>
                  <ListItemText primary="Fee (0.3%)" secondary={price && amount ? `$${(parseFloat(price) * parseFloat(amount) * 0.003).toFixed(2)}` : '-'}/>
                </ListItem>
                <ListItem>
                  <ListItemText primary="Total" secondary={price && amount ? `$${(parseFloat(price) * parseFloat(amount) * 1.003).toFixed(2)}` : '-'}/>
                </ListItem>
              </List>

              <Divider sx={{ my: 2 }}/>

              <Typography variant="subtitle2" gutterBottom>Advanced Options</Typography>

              <TextField fullWidth label="Deadline (minutes)" type="number" value={deadline} onChange={(e) => setDeadline(parseInt(e.target.value))} size="small"/>
            </CardContent>
          </Card>
        </Box>)}

      {/* Order Book Tab */}
      {activeTab === 1 && (<Box sx={{ display: 'flex', gap: 3 }}>
          <Card sx={{ flex: 1 }}>
            <CardContent>
              <Typography variant="h6" gutterBottom>Order Book</Typography>

              {/* Asks */}
              <Typography variant="subtitle2" color="error" sx={{ mb: 1 }}>Asks ({asks.length})</Typography>
              <TableContainer component={Paper} variant="outlined" sx={{ mb: 2, maxHeight: 300 }}>
                <Table size="small" stickyHeader>
                  <TableHead>
                    <TableRow>
                      <TableCell>Price</TableCell>
                      <TableCell align="right">Amount</TableCell>
                      <TableCell align="right">Total</TableCell>
                    </TableRow>
                  </TableHead>
                  <TableBody>
                    {asks.slice(0, 10).map((ask, i) => (<TableRow key={i} sx={{ '&:last-child td, &:last-child th': { border: 0 } }}>
                        <TableCell sx={{ color: 'error.main' }}>{formatPrice(ask.price)}</TableCell>
                        <TableCell align="right">{ask.amount.toFixed(4)}</TableCell>
                        <TableCell align="right">{ask.total.toFixed(4)}</TableCell>
                      </TableRow>))}
                  </TableBody>
                </Table>
              </TableContainer>

              {/* Spread */}
              <Box sx={{ textAlign: 'center', py: 2, bgcolor: 'grey.100', borderRadius: 1, mb: 2 }}>
                <Typography variant="h6">
                  {asks.length > 0 && bids.length > 0
                ? formatPrice(asks[0].price - bids[0].price)
                : '-'}
                </Typography>
                <Typography variant="caption">Spread</Typography>
              </Box>

              {/* Bids */}
              <Typography variant="subtitle2" color="success" sx={{ mb: 1 }}>Bids ({bids.length})</Typography>
              <TableContainer component={Paper} variant="outlined" sx={{ maxHeight: 300 }}>
                <Table size="small" stickyHeader>
                  <TableHead>
                    <TableRow>
                      <TableCell>Price</TableCell>
                      <TableCell align="right">Amount</TableCell>
                      <TableCell align="right">Total</TableCell>
                    </TableRow>
                  </TableHead>
                  <TableBody>
                    {bids.slice(0, 10).map((bid, i) => (<TableRow key={i} sx={{ '&:last-child td, &:last-child th': { border: 0 } }}>
                        <TableCell sx={{ color: 'success.main' }}>{formatPrice(bid.price)}</TableCell>
                        <TableCell align="right">{bid.amount.toFixed(4)}</TableCell>
                        <TableCell align="right">{bid.total.toFixed(4)}</TableCell>
                      </TableRow>))}
                  </TableBody>
                </Table>
              </TableContainer>
            </CardContent>
          </Card>
        </Box>)}

      {/* Positions Tab */}
      {activeTab === 2 && (<Card>
          <CardContent>
            <Typography variant="h6" gutterBottom>Open Positions</Typography>

            {positions.length === 0 ? (<Alert severity="info">No open positions</Alert>) : (<TableContainer>
                <Table>
                  <TableHead>
                    <TableRow>
                      <TableCell>Pair</TableCell>
                      <TableCell>Side</TableCell>
                      <TableCell align="right">Size</TableCell>
                      <TableCell align="right">Entry</TableCell>
                      <TableCell align="right">Current</TableCell>
                      <TableCell align="right">PnL</TableCell>
                      <TableCell align="right">Margin</TableCell>
                      <TableCell align="right">Liq. Price</TableCell>
                      <TableCell>Actions</TableCell>
                    </TableRow>
                  </TableHead>
                  <TableBody>
                    {positions.map(pos => (<TableRow key={pos.id}>
                        <TableCell>{pos.pair}</TableCell>
                        <TableCell sx={{
                        color: pos.side === 'long' ? 'success.main' : 'error.main',
                        fontWeight: 'bold'
                    }}>
                          {pos.side.toUpperCase()}
                        </TableCell>
                        <TableCell align="right">{pos.size}</TableCell>
                        <TableCell align="right">${formatPrice(pos.entryPrice)}</TableCell>
                        <TableCell align="right">${formatPrice(pos.currentPrice)}</TableCell>
                        <TableCell align="right" sx={{
                        color: pos.pnl >= 0 ? 'success.main' : 'error.main'
                    }}>
                          ${pos.pnl.toFixed(2)} ({formatPercent(pos.pnlPercent)})
                        </TableCell>
                        <TableCell align="right">${pos.margin}</TableCell>
                        <TableCell align="right" sx={{
                        color: pos.liquidationPrice < pos.currentPrice ? 'error.main' : 'text.primary'
                    }}>
                          ${formatPrice(pos.liquidationPrice)}
                        </TableCell>
                        <TableCell>
                          <Button size="small" variant="outlined" onClick={() => handleClosePosition(pos.id)}>
                            Close
                          </Button>
                        </TableCell>
                      </TableRow>))}
                  </TableBody>
                </Table>
              </TableContainer>)}
          </CardContent>
        </Card>)}

      {/* Orders Tab */}
      {activeTab === 3 && (<Box sx={{ display: 'flex', gap: 3 }}>
          {/* Open Orders */}
          <Card sx={{ flex: 1 }}>
            <CardContent>
              <Typography variant="h6" gutterBottom>Open Orders</Typography>

              {openOrders.length === 0 ? (<Alert severity="info">No open orders</Alert>) : (<TableContainer>
                  <Table size="small">
                    <TableHead>
                      <TableRow>
                        <TableCell>Type</TableCell>
                        <TableCell>Pair</TableCell>
                        <TableCell>Side</TableCell>
                        <TableCell align="right">Price</TableCell>
                        <TableCell align="right">Amount</TableCell>
                        <TableCell>Status</TableCell>
                        <TableCell>Actions</TableCell>
                      </TableRow>
                    </TableHead>
                    <TableBody>
                      {openOrders.map(order => (<TableRow key={order.id}>
                          <TableCell sx={{ textTransform: 'capitalize' }}>{order.type}</TableCell>
                          <TableCell>{order.pair}</TableCell>
                          <TableCell sx={{
                        color: order.side === 'buy' ? 'success.main' : 'error.main'
                    }}>
                            {order.side.toUpperCase()}
                          </TableCell>
                          <TableCell align="right">
                            {order.price ? `$${formatPrice(order.price)}` : 'Market'}
                            {order.triggerPrice && ` ($${formatPrice(order.triggerPrice)})`}
                          </TableCell>
                          <TableCell align="right">{order.amount}</TableCell>
                          <TableCell>
                            <Chip label={order.status} size="small" color={order.status === 'pending' ? 'primary' : 'default'}/>
                          </TableCell>
                          <TableCell>
                            <Button size="small" onClick={() => handleCancelOrder(order.id)}>
                              Cancel
                            </Button>
                          </TableCell>
                        </TableRow>))}
                    </TableBody>
                  </Table>
                </TableContainer>)}
            </CardContent>
          </Card>

          {/* Order History */}
          <Card sx={{ flex: 1 }}>
            <CardContent>
              <Typography variant="h6" gutterBottom>Order History</Typography>

              {orderHistory.length === 0 ? (<Alert severity="info">No order history</Alert>) : (<TableContainer>
                  <Table size="small">
                    <TableHead>
                      <TableRow>
                        <TableCell>Type</TableCell>
                        <TableCell>Pair</TableCell>
                        <TableCell>Side</TableCell>
                        <TableCell align="right">Price</TableCell>
                        <TableCell align="right">Filled</TableCell>
                        <TableCell>Status</TableCell>
                      </TableRow>
                    </TableHead>
                    <TableBody>
                      {orderHistory.map(order => (<TableRow key={order.id}>
                          <TableCell sx={{ textTransform: 'capitalize' }}>{order.type}</TableCell>
                          <TableCell>{order.pair}</TableCell>
                          <TableCell>{order.side.toUpperCase()}</TableCell>
                          <TableCell align="right">${formatPrice(order.price)}</TableCell>
                          <TableCell align="right">{order.filled}/{order.amount}</TableCell>
                          <TableCell>
                            <Chip label={order.status} size="small" color={order.status === 'filled' ? 'success' : 'default'}/>
                          </TableCell>
                        </TableRow>))}
                    </TableBody>
                  </Table>
                </TableContainer>)}
            </CardContent>
          </Card>
        </Box>)}

      {/* Concentrated Liquidity Tab */}
      {activeTab === 4 && (<Card>
          <CardContent>
            <Typography variant="h6" gutterBottom>Concentrated Liquidity Positions</Typography>

            <Alert severity="info" sx={{ mb: 2 }}>
              Concentrated liquidity provides up to 4000x capital efficiency vs standard AMM
            </Alert>

            {concentratedPositions.length === 0 ? (<Box sx={{ textAlign: 'center', py: 4 }}>
                <Typography color="text.secondary" gutterBottom>No concentrated positions</Typography>
                <Button variant="contained">Create Position</Button>
              </Box>) : (<TableContainer>
                <Table>
                  <TableHead>
                    <TableRow>
                      <TableCell>Token Pair</TableCell>
                      <TableCell align="right">Tick Range</TableCell>
                      <TableCell align="right">Liquidity</TableCell>
                      <TableCell align="right">Fees Earned</TableCell>
                      <TableCell align="right">APR</TableCell>
                      <TableCell>Actions</TableCell>
                    </TableRow>
                  </TableHead>
                  <TableBody>
                    {concentratedPositions.map(pos => (<TableRow key={pos.id}>
                        <TableCell>{pos.token0}/{pos.token1}</TableCell>
                        <TableCell align="right">
                          {pos.tickLower} - {pos.tickUpper}
                        </TableCell>
                        <TableCell align="right">${pos.liquidity.toLocaleString()}</TableCell>
                        <TableCell align="right">${pos.feesEarned.toFixed(2)}</TableCell>
                        <TableCell align="right" sx={{ color: 'success.main', fontWeight: 'bold' }}>
                          {pos.apr.toFixed(1)}%
                        </TableCell>
                        <TableCell>
                          <Button size="small">Collect</Button>
                          <Button size="small">Modify</Button>
                        </TableCell>
                      </TableRow>))}
                  </TableBody>
                </Table>
              </TableContainer>)}
          </CardContent>
        </Card>)}

      {/* Confirm Dialog */}
      <Dialog open={confirmDialogOpen} onClose={() => setConfirmDialogOpen(false)}>
        <DialogTitle>Confirm Order</DialogTitle>
        <DialogContent>
          <Typography>
            {side === 'buy' ? 'Buy' : 'Sell'} {amount} {selectedPair} @ {orderType}
            {price && ` at $${price}`}
          </Typography>
          <Divider sx={{ my: 2 }}/>
          <Typography>
            Estimated total: ${(parseFloat(price || '0') * parseFloat(amount || '0') * 1.003).toFixed(2)}
          </Typography>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setConfirmDialogOpen(false)}>Cancel</Button>
          <Button onClick={confirmOrder} variant="contained" disabled={loading}>
            {loading ? 'Confirming...' : 'Confirm'}
          </Button>
        </DialogActions>
      </Dialog>
    </Box>);
}
//# sourceMappingURL=page.jsx.map