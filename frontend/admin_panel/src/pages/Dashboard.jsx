import React from 'react';
import { Box, Grid, Card, CardContent, Typography, Table, TableBody, TableCell, TableContainer, TableHead, TableRow, Chip } from '@mui/material';
import { LineChart, Line, PieChart, Pie, Cell, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, Legend } from 'recharts';
const volumeData = [
    { name: 'Mon', volume: 2400000 },
    { name: 'Tue', volume: 3200000 },
    { name: 'Wed', volume: 2800000 },
    { name: 'Thu', volume: 4100000 },
    { name: 'Fri', volume: 3800000 },
    { name: 'Sat', volume: 2900000 },
    { name: 'Sun', volume: 2600000 },
];
const chainDistribution = [
    { name: 'Ethereum', value: 35 },
    { name: 'BSC', value: 28 },
    { name: 'Polygon', value: 15 },
    { name: 'Arbitrum', value: 12 },
    { name: 'Other', value: 10 },
];
const recentSwaps = [
    { id: '0x1234...5678', user: '0xAbc...789', tokenIn: 'ETH', amountIn: '2.5', tokenOut: 'USDT', amountOut: '6125', chain: 'Ethereum', time: '2m ago' },
    { id: '0x2345...6789', user: '0xDef...012', tokenIn: 'USDC', amountIn: '10000', tokenOut: 'ETH', amountOut: '4.1', chain: 'BSC', time: '5m ago' },
    { id: '0x3456...7890', user: '0xGhi...345', tokenIn: 'BNB', amountIn: '15', tokenOut: 'CAKE', amountOut: '225', chain: 'BSC', time: '8m ago' },
    { id: '0x4567...8901', user: '0xJkl...678', tokenIn: 'MATIC', amountIn: '5000', tokenOut: 'ETH', amountOut: '2.8', chain: 'Polygon', time: '12m ago' },
    { id: '0x5678...9012', user: '0xMno...901', tokenIn: 'ETH', amountIn: '1.2', tokenOut: 'ARB', amountOut: '1200', chain: 'Arbitrum', time: '15m ago' },
];
const Dashboard = () => {
    return (<Box>
      <Typography variant="h4" gutterBottom>Dashboard</Typography>
      <Grid container spacing={3}>
        <Grid item xs={12} sm={6} md={3}>
          <Card><CardContent><Typography color="textSecondary" gutterBottom>Total Volume (24h)</Typography><Typography variant="h4">$21.8M</Typography><Typography variant="body2" color="success.main">+12.5%</Typography></CardContent></Card>
        </Grid>
        <Grid item xs={12} sm={6} md={3}>
          <Card><CardContent><Typography color="textSecondary" gutterBottom>Total Users</Typography><Typography variant="h4">154,289</Typography><Typography variant="body2" color="success.main">+8.2%</Typography></CardContent></Card>
        </Grid>
        <Grid item xs={12} sm={6} md={3}>
          <Card><CardContent><Typography color="textSecondary" gutterBottom>Total Transactions</Typography><Typography variant="h4">1.2M</Typography><Typography variant="body2" color="success.main">+15.3%</Typography></CardContent></Card>
        </Grid>
        <Grid item xs={12} sm={6} md={3}>
          <Card><CardContent><Typography color="textSecondary" gutterBottom>Active Pools</Typography><Typography variant="h4">3,456</Typography><Typography variant="body2" color="success.main">+5.1%</Typography></CardContent></Card>
        </Grid>
        <Grid item xs={12} md={8}>
          <Card sx={{ height: 400 }}><CardContent><Typography variant="h6" gutterBottom>Volume Trend</Typography><ResponsiveContainer width="100%" height="90%"><LineChart data={volumeData}><CartesianGrid strokeDasharray="3 3"/><XAxis dataKey="name"/><YAxis /><Tooltip /><Legend /><Line type="monotone" dataKey="volume" stroke="#f97316" strokeWidth={2}/></LineChart></ResponsiveContainer></CardContent></Card>
        </Grid>
        <Grid item xs={12} md={4}>
          <Card sx={{ height: 400 }}><CardContent><Typography variant="h6" gutterBottom>Chain Distribution</Typography><ResponsiveContainer width="100%" height="90%"><PieChart><Pie data={chainDistribution} dataKey="value" nameKey="name" cx="50%" cy="50%" outerRadius={80} label>{chainDistribution.map((_, index) => (<Cell key={`cell-${index}`} fill={['#f97316', '#3b82f6', '#10b981', '#8b5cf6', '#ec4899'][index % 5]}/>))}</Pie><Tooltip /><Legend /></PieChart></ResponsiveContainer></CardContent></Card>
        </Grid>
        <Grid item xs={12}>
          <Card><CardContent><Typography variant="h6" gutterBottom>Recent Swaps</Typography><TableContainer><Table><TableHead><TableRow><TableCell>Tx Hash</TableCell><TableCell>User</TableCell><TableCell>Token In</TableCell><TableCell>Amount In</TableCell><TableCell>Token Out</TableCell><TableCell>Amount Out</TableCell><TableCell>Chain</TableCell><TableCell>Time</TableCell></TableRow></TableHead><TableBody>{recentSwaps.map((swap) => (<TableRow key={swap.id}><TableCell><Chip label={swap.id} size="small"/></TableCell><TableCell>{swap.user}</TableCell><TableCell>{swap.tokenIn}</TableCell><TableCell>{swap.amountIn}</TableCell><TableCell>{swap.tokenOut}</TableCell><TableCell>{swap.amountOut}</TableCell><TableCell><Chip label={swap.chain} size="small" color="primary"/></TableCell><TableCell>{swap.time}</TableCell></TableRow>))}</TableBody></Table></TableContainer></CardContent></Card>
        </Grid>
      </Grid>
    </Box>);
};
export default Dashboard;
//# sourceMappingURL=Dashboard.jsx.map