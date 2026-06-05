'use client';

import React, { useState, useEffect } from 'react';
import {
  Box, Typography, Card, CardContent, Button, Chip,
  CircularProgress, Snackbar, Alert, Divider, LinearProgress,
  Table, TableBody, TableCell, TableContainer, TableHead, TableRow,
  Tabs, Tab
} from '@mui/material';
import {
  AccountBalance, Wallet, ShowChart, TrendingUp, TrendingDown,
  SwapHoriz, Pool, History, Refresh
} from '@mui/icons-material';

// ============================================================================
// Types
// ============================================================================

interface Asset {
  symbol: string;
  name: string;
  balance: number;
  value: number;
  change24h: number;
  icon: string;
  address?: string;
}

interface Position {
  type: 'liquidity' | 'farming' | 'staking';
  protocol: string;
  pair: string;
  value: number;
  apr: number;
  pnl: number;
  icon: string;
}

interface Transaction {
  hash: string;
  type: string;
  tokenIn: string;
  tokenOut: string;
  amountIn: string;
  amountOut: string;
  value: number;
  status: 'success' | 'pending' | 'failed';
  timestamp: number;
}

// ============================================================================
// Mock Data
// ============================================================================

const ASSETS: Asset[] = [
  { symbol: 'ETH', name: 'Ethereum', balance: 2.5, value: 6125, change24h: 2.34, icon: '🔷' },
  { symbol: 'USDC', name: 'USD Coin', balance: 5000, value: 5000, change24h: 0.01, icon: '💵' },
  { symbol: 'USDT', name: 'Tether', balance: 2500, value: 2500, change24h: 0.02, icon: '💰' },
  { symbol: 'WBTC', name: 'Wrapped Bitcoin', balance: 0.1, value: 6250, change24h: 1.15, icon: '₿' },
  { symbol: 'LINK', name: 'Chainlink', balance: 150, value: 2775, change24h: -1.5, icon: '🔗' },
  { symbol: 'UNI', name: 'Uniswap', balance: 200, value: 2500, change24h: 3.2, icon: '🦄' },
];

const POSITIONS: Position[] = [
  { type: 'liquidity', protocol: 'TigerSwap', pair: 'ETH/USDC', value: 12500, apr: 24.5, pnl: 850, icon: '💎' },
  { type: 'farming', protocol: 'PancakeSwap', pair: 'CAKE/BNB', value: 8000, apr: 45.2, pnl: 1200, icon: '🥞' },
  { type: 'staking', protocol: 'Lido', pair: 'stETH', value: 5000, apr: 4.2, pnl: 150, icon: '🏦' },
];

const TRANSACTIONS: Transaction[] = [
  { hash: '0x1234567890abcdef', type: 'Swap', tokenIn: 'ETH', tokenOut: 'USDC', amountIn: '1.0', amountOut: '2450', value: 2450, status: 'success', timestamp: Date.now() - 3600000 },
  { hash: '0xabcdef1234567890', type: 'Add Liquidity', tokenIn: 'ETH', tokenOut: 'USDC', amountIn: '2.0', amountOut: '4900', value: 4900, status: 'success', timestamp: Date.now() - 86400000 },
  { hash: '0x567890abcdef1234', type: 'Swap', tokenIn: 'USDT', tokenOut: 'ETH', amountIn: '1000', amountOut: '0.4', value: 1000, status: 'success', timestamp: Date.now() - 172800000 },
];

// ============================================================================
// Utility Functions
// ============================================================================

function formatUSD(amount: number): string {
  return new Intl.NumberFormat('en-US', { style: 'currency', currency: 'USD' }).format(amount);
}

function formatNumber(num: number): string {
  if (num >= 1e6) return (num / 1e6).toFixed(2) + 'M';
  if (num >= 1e3) return (num / 1e3).toFixed(2) + 'K';
  return num.toFixed(2);
}

function timeAgo(timestamp: number): string {
  const diff = Date.now() - timestamp;
  const hours = Math.floor(diff / 3600000);
  if (hours < 24) return `${hours}h ago`;
  return `${Math.floor(hours / 24)}d ago`;
}

function formatAddress(hash: string): string {
  return `${hash.slice(0, 10)}...${hash.slice(-8)}`;
}

// ============================================================================
// Main Portfolio Page
// ============================================================================

export default function PortfolioPage() {
  const [assets] = useState<Asset[]>(ASSETS);
  const [positions] = useState<Position[]>(POSITIONS);
  const [transactions] = useState<Transaction[]>(TRANSACTIONS);
  const [activeTab, setActiveTab] = useState(0);
  const [loading] = useState(false);

  const totalValue = assets.reduce((sum, a) => sum + a.value, 0);
  const totalPnL = positions.reduce((sum, p) => sum + p.pnl, 0);

  return (
    <Box sx={{ minHeight: '100vh', bgcolor: '#0a0a14', p: 3 }}>
      <Box sx={{ maxWidth: 1400, mx: 'auto' }}>
        {/* Header */}
        <Box sx={{ mb: 4 }}>
          <Typography variant="h4" sx={{ color: 'white', fontWeight: 'bold' }}>
            💼 Portfolio
          </Typography>
          <Typography variant="body2" sx={{ color: '#9ca3af', mt: 1 }}>
            Track your assets, positions, and transaction history
          </Typography>
        </Box>

        {/* Overview Cards */}
        <Box sx={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: 2, mb: 4 }}>
          <Card sx={{ bgcolor: '#1a1a2e', borderRadius: 3 }}>
            <CardContent>
              <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 1 }}>
                <Wallet sx={{ color: '#00d4aa' }} />
                <Typography variant="caption" sx={{ color: '#9ca3af' }}>Total Value</Typography>
              </Box>
              <Typography variant="h5" sx={{ color: '#00d4aa', fontWeight: 'bold' }}>
                {formatUSD(totalValue)}
              </Typography>
            </CardContent>
          </Card>
          <Card sx={{ bgcolor: '#1a1a2e', borderRadius: 3 }}>
            <CardContent>
              <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 1 }}>
                <ShowChart sx={{ color: '#00d4aa' }} />
                <Typography variant="caption" sx={{ color: '#9ca3af' }}>24h Change</Typography>
              </Box>
              <Typography variant="h5" sx={{ color: '#00d4aa', fontWeight: 'bold' }}>
                +{formatUSD(totalValue * 0.023)}
              </Typography>
            </CardContent>
          </Card>
          <Card sx={{ bgcolor: '#1a1a2e', borderRadius: 3 }}>
            <CardContent>
              <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 1 }}>
                <Pool sx={{ color: '#ff9800' }} />
                <Typography variant="caption" sx={{ color: '#9ca3af' }}>Positions Value</Typography>
              </Box>
              <Typography variant="h5" sx={{ color: 'white', fontWeight: 'bold' }}>
                {formatUSD(positions.reduce((s, p) => s + p.value, 0))}
              </Typography>
            </CardContent>
          </Card>
          <Card sx={{ bgcolor: '#1a1a2e', borderRadius: 3 }}>
            <CardContent>
              <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 1 }}>
                <TrendingUp sx={{ color: '#00d4aa' }} />
                <Typography variant="caption" sx={{ color: '#9ca3af' }}>Total P&L</Typography>
              </Box>
              <Typography variant="h5" sx={{ color: totalPnL >= 0 ? '#00d4aa' : '#ff5722', fontWeight: 'bold' }}>
                {totalPnL >= 0 ? '+' : ''}{formatUSD(totalPnL)}
              </Typography>
            </CardContent>
          </Card>
        </Box>

        {/* Tabs */}
        <Card sx={{ bgcolor: '#1a1a2e', borderRadius: 3, mb: 3 }}>
          <Tabs
            value={activeTab}
            onChange={(_, v) => setActiveTab(v)}
            sx={{ borderBottom: '1px solid #2a2a3e', '& .MuiTab-root': { color: '#9ca3af' }, '& .Mui-selected': { color: '#00d4aa' } }}
          >
            <Tab label="Assets" />
            <Tab label="Positions" />
            <Tab label="History" />
          </Tabs>

          <CardContent sx={{ p: 3 }}>
            {activeTab === 0 && (
              <TableContainer>
                <Table>
                  <TableHead>
                    <TableRow>
                      <TableCell sx={{ color: '#9ca3af' }}>Asset</TableCell>
                      <TableCell sx={{ color: '#9ca3af' }} align="right">Balance</TableCell>
                      <TableCell sx={{ color: '#9ca3af' }} align="right">Value</TableCell>
                      <TableCell sx={{ color: '#9ca3af' }} align="right">24h Change</TableCell>
                    </TableRow>
                  </TableHead>
                  <TableBody>
                    {assets.map(asset => (
                      <TableRow key={asset.symbol} sx={{ '&:hover': { bgcolor: '#2a2a3e' } }}>
                        <TableCell>
                          <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
                            <Typography sx={{ fontSize: 24 }}>{asset.icon}</Typography>
                            <Box>
                              <Typography sx={{ color: 'white', fontWeight: 'bold' }}>{asset.symbol}</Typography>
                              <Typography variant="caption" sx={{ color: '#9ca3af' }}>{asset.name}</Typography>
                            </Box>
                          </Box>
                        </TableCell>
                        <TableCell align="right">
                          <Typography sx={{ color: 'white' }}>{asset.balance.toLocaleString()}</Typography>
                        </TableCell>
                        <TableCell align="right">
                          <Typography sx={{ color: '#00d4aa' }}>{formatUSD(asset.value)}</Typography>
                        </TableCell>
                        <TableCell align="right">
                          <Typography sx={{ color: asset.change24h >= 0 ? '#00d4aa' : '#ff5722' }}>
                            {asset.change24h >= 0 ? '+' : ''}{asset.change24h.toFixed(2)}%
                          </Typography>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </TableContainer>
            )}

            {activeTab === 1 && (
              <TableContainer>
                <Table>
                  <TableHead>
                    <TableRow>
                      <TableCell sx={{ color: '#9ca3af' }}>Position</TableCell>
                      <TableCell sx={{ color: '#9ca3af' }} align="right">Value</TableCell>
                      <TableCell sx={{ color: '#9ca3af' }} align="right">APR</TableCell>
                      <TableCell sx={{ color: '#9ca3af' }} align="right">P&L</TableCell>
                    </TableRow>
                  </TableHead>
                  <TableBody>
                    {positions.map((pos, i) => (
                      <TableRow key={i} sx={{ '&:hover': { bgcolor: '#2a2a3e' } }}>
                        <TableCell>
                          <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
                            <Typography sx={{ fontSize: 24 }}>{pos.icon}</Typography>
                            <Box>
                              <Typography sx={{ color: 'white', fontWeight: 'bold' }}>{pos.pair}</Typography>
                              <Typography variant="caption" sx={{ color: '#9ca3af' }}>{pos.protocol}</Typography>
                            </Box>
                          </Box>
                        </TableCell>
                        <TableCell align="right">
                          <Typography sx={{ color: '#00d4aa' }}>{formatUSD(pos.value)}</Typography>
                        </TableCell>
                        <TableCell align="right">
                          <Chip label={`${pos.apr}%`} size="small" sx={{ bgcolor: '#00d4aa20', color: '#00d4aa' }} />
                        </TableCell>
                        <TableCell align="right">
                          <Typography sx={{ color: pos.pnl >= 0 ? '#00d4aa' : '#ff5722' }}>
                            {pos.pnl >= 0 ? '+' : ''}{formatUSD(pos.pnl)}
                          </Typography>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </TableContainer>
            )}

            {activeTab === 2 && (
              <TableContainer>
                <Table>
                  <TableHead>
                    <TableRow>
                      <TableCell sx={{ color: '#9ca3af' }}>Type</TableCell>
                      <TableCell sx={{ color: '#9ca3af' }}>Details</TableCell>
                      <TableCell sx={{ color: '#9ca3af' }} align="right">Value</TableCell>
                      <TableCell sx={{ color: '#9ca3af' }} align="right">Status</TableCell>
                      <TableCell sx={{ color: '#9ca3af' }} align="right">Time</TableCell>
                    </TableRow>
                  </TableHead>
                  <TableBody>
                    {transactions.map(tx => (
                      <TableRow key={tx.hash} sx={{ '&:hover': { bgcolor: '#2a2a3e' } }}>
                        <TableCell>
                          <Chip label={tx.type} size="small" sx={{ bgcolor: '#2a2a3e' }} />
                        </TableCell>
                        <TableCell>
                          <Typography sx={{ color: 'white' }}>{tx.amountIn} {tx.tokenIn} → {tx.amountOut} {tx.tokenOut}</Typography>
                        </TableCell>
                        <TableCell align="right">
                          <Typography sx={{ color: '#00d4aa' }}>{formatUSD(tx.value)}</Typography>
                        </TableCell>
                        <TableCell align="right">
                          <Chip 
                            label={tx.status} 
                            size="small" 
                            sx={{ 
                              bgcolor: tx.status === 'success' ? '#00d4aa20' : tx.status === 'pending' ? '#ff980020' : '#ff572220',
                              color: tx.status === 'success' ? '#00d4aa' : tx.status === 'pending' ? '#ff9800' : '#ff5722'
                            }} 
                          />
                        </TableCell>
                        <TableCell align="right">
                          <Typography sx={{ color: '#9ca3af' }}>{timeAgo(tx.timestamp)}</Typography>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </TableContainer>
            )}
          </CardContent>
        </Card>
      </Box>
    </Box>
  );
}