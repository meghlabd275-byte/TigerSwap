'use client';

import React, { useState, useEffect, useCallback } from 'react';
import {
  Box, Typography, Card, CardContent, Button, TextField, Tabs, Tab, Chip,
  Table, TableBody, TableCell, TableContainer, TableHead, TableRow,
  Paper, IconButton, Dialog, DialogTitle, DialogContent, DialogActions,
  Alert, Select, MenuItem, FormControl, InputLabel, Switch,
  FormControlLabel, Slider, Grid, Avatar, Divider, List, ListItem,
  ListItemText, ListItemIcon, LinearProgress, Tooltip, Badge, AvatarGroup
} from '@mui/material';
import {
  ListAlt, Add, Edit, Delete, Refresh, CheckCircle, Error as ErrorIcon,
  Warning, Schedule, Verified, TrendingUp, TrendingDown, Visibility,
  Star, Comment, ThumbUp, ThumbDown
} from '@mui/icons-material';

// ============================================================================
// Types & Interfaces
// ============================================================================

interface ListingRequest {
  id: string;
  tokenName: string;
  tokenSymbol: string;
  tokenAddress: string;
  chain: string;
  requester: string;
  description: string;
  website: string;
  logo: string;
  status: 'pending' | 'approved' | 'rejected' | 'featured';
  votesFor: number;
  votesAgainst: number;
  kycStatus: 'none' | 'basic' | 'full';
  auditStatus: 'none' | 'pending' | 'approved' | 'failed';
  listingFee: number;
  requestedAt: number;
  resolvedAt?: number;
  socialLinks: {
    twitter?: string;
    telegram?: string;
    discord?: string;
  };
}

interface ListedToken {
  id: string;
  name: string;
  symbol: string;
  address: string;
  chain: string;
  logo: string;
  price: number;
  priceChange24h: number;
  volume24h: number;
  marketCap: number;
  liquidity: number;
  holders: number;
  verified: boolean;
  featured: boolean;
  listedAt: number;
  category: string;
}

interface ListingStats {
  totalListed: number;
  pendingRequests: number;
  approvedThisMonth: number;
  rejectedThisMonth: number;
  totalFees: number;
}

// ============================================================================
// Component
// ============================================================================

export default function AdminListingPage() {
  // State
  const [activeTab, setActiveTab] = useState(0);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);
  
  // Data
  const [requests, setRequests] = useState<ListingRequest[]>([]);
  const [tokens, setTokens] = useState<ListedToken[]>([]);
  const [stats, setStats] = useState<ListingStats | null>(null);
  
  // Dialog
  const [detailDialogOpen, setDetailDialogOpen] = useState(false);
  const [selectedRequest, setSelectedRequest] = useState<ListingRequest | null>(null);
  
  // ============================================================================
  // Effects
  // ============================================================================
  
  useEffect(() => {
    loadData();
  }, []);
  
  // ============================================================================
  // Data Loading
  // ============================================================================
  
  const loadData = useCallback(async () => {
    setLoading(true);
    try {
      await new Promise(resolve => setTimeout(resolve, 500));
      
      // Mock listing requests
      setRequests([
        {
          id: 'req1',
          tokenName: 'Tiger Token',
          tokenSymbol: 'TIGER',
          tokenAddress: '0x1234...abcd',
          chain: 'Ethereum',
          requester: '0xreq...1234',
          description: 'The native token of TigerSwap DEX',
          website: 'https://tigerswap.io',
          logo: 'https://tigerswap.io/logo.png',
          status: 'pending',
          votesFor: 150,
          votesAgainst: 20,
          kycStatus: 'full',
          auditStatus: 'approved',
          listingFee: 10000,
          requestedAt: Date.now() - 86400000,
          socialLinks: { twitter: '@tigerswap', telegram: '@tigerswap' },
        },
        {
          id: 'req2',
          tokenName: 'DeFi Protocol Token',
          tokenSymbol: 'DEF',
          tokenAddress: '0x5678...efgh',
          chain: 'BNB Chain',
          requester: '0xreq...5678',
          description: 'DeFi yield farming protocol',
          website: 'https://defiprotocol.io',
          logo: 'https://defiprotocol.io/logo.png',
          status: 'pending',
          votesFor: 80,
          votesAgainst: 45,
          kycStatus: 'basic',
          auditStatus: 'pending',
          listingFee: 5000,
          requestedAt: Date.now() - 172800000,
          socialLinks: { twitter: '@defiprotocol' },
        },
        {
          id: 'req3',
          tokenName: 'GameFi Token',
          tokenSymbol: 'GAME',
          tokenAddress: '0xabcd...1234',
          chain: 'Polygon',
          requester: '0xreq...abcd',
          description: 'Play-to-earn gaming platform',
          website: 'https://gamefi.io',
          logo: 'https://gamefi.io/logo.png',
          status: 'approved',
          votesFor: 200,
          votesAgainst: 10,
          kycStatus: 'full',
          auditStatus: 'approved',
          listingFee: 8000,
          requestedAt: Date.now() - 604800000,
          resolvedAt: Date.now() - 259200000,
          socialLinks: { twitter: '@gamefi', discord: 'gamefi' },
        },
      ]);
      
      // Mock listed tokens
      setTokens([
        {
          id: 'tok1',
          name: 'Wrapped Ethereum',
          symbol: 'WETH',
          address: '0xc02a...',
          chain: 'Ethereum',
          logo: '',
          price: 3500.00,
          priceChange24h: 2.5,
          volume24h: 1500000000,
          marketCap: 45000000000,
          liquidity: 250000000,
          holders: 150000,
          verified: true,
          featured: true,
          listedAt: Date.now() - 31536000000,
          category: 'Wrapper',
        },
        {
          id: 'tok2',
          name: 'USD Coin',
          symbol: 'USDC',
          address: '0xa0b8...',
          chain: 'Ethereum',
          logo: '',
          price: 1.00,
          priceChange24h: 0.01,
          volume24h: 8000000000,
          marketCap: 45000000000,
          liquidity: 500000000,
          holders: 500000,
          verified: true,
          featured: true,
          listedAt: Date.now() - 63072000000,
          category: 'Stablecoin',
        },
        {
          id: 'tok3',
          name: 'Tiger Swap',
          symbol: 'TIGER',
          address: '0xtiger...',
          chain: 'Ethereum',
          logo: '',
          price: 2.50,
          priceChange24h: 5.2,
          volume24h: 50000000,
          marketCap: 250000000,
          liquidity: 10000000,
          holders: 25000,
          verified: true,
          featured: true,
          listedAt: Date.now() - 2592000000,
          category: 'DEX',
        },
      ]);
      
      // Mock stats
      setStats({
        totalListed: 156,
        pendingRequests: 12,
        approvedThisMonth: 8,
        rejectedThisMonth: 3,
        totalFees: 250000,
      });
      
      setSuccess('Listing data loaded successfully');
    } catch (err: any) {
      setError(err.message || 'Failed to load data');
    } finally {
      setLoading(false);
    }
  }, []);
  
  // ============================================================================
  // Actions
  // ============================================================================
  
  const handleApprove = useCallback(async (requestId: string) => {
    setLoading(true);
    try {
      await new Promise(resolve => setTimeout(resolve, 500));
      setRequests(requests.map(r => 
        r.id === requestId ? { ...r, status: 'approved' as const, resolvedAt: Date.now() } : r
      ));
      setSuccess('Listing request approved');
    } catch (err: any) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  }, [requests]);
  
  const handleReject = useCallback(async (requestId: string) => {
    setLoading(true);
    try {
      await new Promise(resolve => setTimeout(resolve, 500));
      setRequests(requests.map(r => 
        r.id === requestId ? { ...r, status: 'rejected' as const, resolvedAt: Date.now() } : r
      ));
      setSuccess('Listing request rejected');
    } catch (err: any) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  }, [requests]);
  
  const handleFeature = useCallback(async (tokenId: string, featured: boolean) => {
    setTokens(tokens.map(t => 
      t.id === tokenId ? { ...t, featured } : t
    ));
  }, [tokens]);
  
  const handleVerify = useCallback(async (tokenId: string, verified: boolean) => {
    setTokens(tokens.map(t => 
      t.id === tokenId ? { ...t, verified } : t
    ));
  }, [tokens]);
  
  const handleViewDetail = useCallback((request: ListingRequest) => {
    setSelectedRequest(request);
    setDetailDialogOpen(true);
  }, []);
  
  // ============================================================================
  // Helper Functions
  // ============================================================================
  
  const formatCurrency = (amount: number) => {
    if (amount >= 1e9) return `$${(amount / 1e9).toFixed(2)}B`;
    if (amount >= 1e6) return `$${(amount / 1e6).toFixed(2)}M`;
    if (amount >= 1e3) return `$${(amount / 1e3).toFixed(2)}K`;
    return `$${amount.toFixed(2)}`;
  };
  
  const formatDate = (timestamp: number) => {
    return new Date(timestamp).toLocaleDateString('en-US', {
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    });
  };
  
  // ============================================================================
  // Render
  // ============================================================================
  
  return (
    <Box sx={{ p: 3 }}>
      {/* Header */}
      <Box sx={{ mb: 3, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <Box>
          <Typography variant="h5" fontWeight="bold">Listing Management</Typography>
          <Typography variant="body2" color="text.secondary">
            Manage token listings, requests, and fees
          </Typography>
        </Box>
        <Button variant="outlined" startIcon={<Refresh />} onClick={loadData}>
          Refresh
        </Button>
      </Box>
      
      {/* Alerts */}
      {error && (
        <Alert severity="error" sx={{ mb: 2 }} onClose={() => setError(null)}>
          {error}
        </Alert>
      )}
      {success && (
        <Alert severity="success" sx={{ mb: 2 }} onClose={() => setSuccess(null)}>
          {success}
        </Alert>
      )}
      
      {loading && <LinearProgress sx={{ mb: 2 }} />}
      
      {/* Stats */}
      {stats && (
        <Grid container spacing={3} sx={{ mb: 3 }}>
          <Grid item xs={12} sm={6} md={3}>
            <Card>
              <CardContent>
                <Typography color="text.secondary" gutterBottom>Total Listed</Typography>
                <Typography variant="h4">{stats.totalListed}</Typography>
              </CardContent>
            </Card>
          </Grid>
          <Grid item xs={12} sm={6} md={3}>
            <Card>
              <CardContent>
                <Typography color="text.secondary" gutterBottom>Pending Requests</Typography>
                <Typography variant="h4" color="warning.main">{stats.pendingRequests}</Typography>
              </CardContent>
            </Card>
          </Grid>
          <Grid item xs={12} sm={6} md={3}>
            <Card>
              <CardContent>
                <Typography color="text.secondary" gutterBottom>Approved This Month</Typography>
                <Typography variant="h4" color="success.main">{stats.approvedThisMonth}</Typography>
              </CardContent>
            </Card>
          </Grid>
          <Grid item xs={12} sm={6} md={3}>
            <Card>
              <CardContent>
                <Typography color="text.secondary" gutterBottom>Total Fees</Typography>
                <Typography variant="h4">{formatCurrency(stats.totalFees)}</Typography>
              </CardContent>
            </Card>
          </Grid>
        </Grid>
      )}
      
      {/* Tabs */}
      <Tabs value={activeTab} onChange={(_, v) => setActiveTab(v)} sx={{ mb: 3 }}>
        <Tab label="Pending Requests" icon={<Schedule />} iconPosition="start" />
        <Tab label="Listed Tokens" icon={<ListAlt />} iconPosition="start" />
        <Tab label="Featured" icon={<Star />} iconPosition="start" />
      </Tabs>
      
      {/* Pending Requests Tab */}
      {activeTab === 0 && (
        <TableContainer component={Paper}>
          <Table>
            <TableHead>
              <TableRow>
                <TableCell>Token</TableCell>
                <TableCell>Chain</TableCell>
                <TableCell>Requester</TableCell>
                <TableCell align="center">Votes</TableCell>
                <TableCell>KYC</TableCell>
                <TableCell>Audit</TableCell>
                <TableCell align="right">Fee</TableCell>
                <TableCell>Status</TableCell>
                <TableCell>Actions</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {requests.filter(r => r.status === 'pending').map(request => (
                <TableRow key={request.id}>
                  <TableCell>
                    <Box>
                      <Typography variant="body2" fontWeight="bold">{request.tokenName}</Typography>
                      <Typography variant="caption" color="text.secondary">{request.tokenSymbol}</Typography>
                    </Box>
                  </TableCell>
                  <TableCell>{request.chain}</TableCell>
                  <TableCell sx={{ fontFamily: 'monospace', fontSize: '0.8rem' }}>
                    {request.requester}
                  </TableCell>
                  <TableCell align="center">
                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                      <Chip icon={<ThumbUp />} label={request.votesFor} size="small" color="success" variant="outlined" />
                      <Chip icon={<ThumbDown />} label={request.votesAgainst} size="small" color="error" variant="outlined" />
                    </Box>
                  </TableCell>
                  <TableCell>
                    <Chip 
                      label={request.kycStatus.toUpperCase()} 
                      size="small" 
                      color={request.kycStatus === 'full' ? 'success' : request.kycStatus === 'basic' ? 'warning' : 'default'} 
                    />
                  </TableCell>
                  <TableCell>
                    <Chip 
                      label={request.auditStatus.toUpperCase()} 
                      size="small" 
                      color={request.auditStatus === 'approved' ? 'success' : request.auditStatus === 'pending' ? 'warning' : request.auditStatus === 'failed' ? 'error' : 'default'} 
                    />
                  </TableCell>
                  <TableCell align="right">{formatCurrency(request.listingFee)}</TableCell>
                  <TableCell>
                    <Chip label="Pending" size="small" color="warning" />
                  </TableCell>
                  <TableCell>
                    <Box sx={{ display: 'flex', gap: 1 }}>
                      <Tooltip title="View Details">
                        <IconButton size="small" onClick={() => handleViewDetail(request)}>
                          <Visibility />
                        </IconButton>
                      </Tooltip>
                      <Tooltip title="Approve">
                        <IconButton size="small" color="success" onClick={() => handleApprove(request.id)}>
                          <CheckCircle />
                        </IconButton>
                      </Tooltip>
                      <Tooltip title="Reject">
                        <IconButton size="small" color="error" onClick={() => handleReject(request.id)}>
                          <ErrorIcon />
                        </IconButton>
                      </Tooltip>
                    </Box>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </TableContainer>
      )}
      
      {/* Listed Tokens Tab */}
      {activeTab === 1 && (
        <TableContainer component={Paper}>
          <Table>
            <TableHead>
              <TableRow>
                <TableCell>Token</TableCell>
                <TableCell>Chain</TableCell>
                <TableCell align="right">Price</TableCell>
                <TableCell align="right">24h Change</TableCell>
                <TableCell align="right">Volume</TableCell>
                <TableCell align="right">Market Cap</TableCell>
                <TableCell align="right">Holders</TableCell>
                <TableCell>Verified</TableCell>
                <TableCell>Actions</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {tokens.map(token => (
                <TableRow key={token.id}>
                  <TableCell>
                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                      <Avatar src={token.logo} sx={{ width: 32, height: 32 }}>
                        {token.symbol[0]}
                      </Avatar>
                      <Box>
                        <Typography variant="body2" fontWeight="bold">{token.name}</Typography>
                        <Typography variant="caption" color="text.secondary">{token.symbol}</Typography>
                      </Box>
                    </Box>
                  </TableCell>
                  <TableCell>{token.chain}</TableCell>
                  <TableCell align="right">{formatCurrency(token.price)}</TableCell>
                  <TableCell align="right" sx={{ color: token.priceChange24h >= 0 ? 'success.main' : 'error.main' }}>
                    {token.priceChange24h >= 0 ? <TrendingUp /> : <TrendingDown />}
                    {token.priceChange24h.toFixed(2)}%
                  </TableCell>
                  <TableCell align="right">{formatCurrency(token.volume24h)}</TableCell>
                  <TableCell align="right">{formatCurrency(token.marketCap)}</TableCell>
                  <TableCell align="right">{token.holders.toLocaleString()}</TableCell>
                  <TableCell>
                    <Switch 
                      checked={token.verified} 
                      onChange={(e) => handleVerify(token.id, e.target.checked)}
                      size="small"
                    />
                  </TableCell>
                  <TableCell>
                    <Switch 
                      checked={token.featured} 
                      onChange={(e) => handleFeature(token.id, e.target.checked)}
                      size="small"
                    />
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </TableContainer>
      )}
      
      {/* Featured Tab */}
      {activeTab === 2 && (
        <Grid container spacing={3}>
          {tokens.filter(t => t.featured).map(token => (
            <Grid item xs={12} md={4} key={token.id}>
              <Card>
                <CardContent>
                  <Box sx={{ display: 'flex', alignItems: 'center', gap: 2, mb: 2 }}>
                    <Avatar src={token.logo} sx={{ width: 48, height: 48 }}>
                      {token.symbol[0]}
                    </Avatar>
                    <Box>
                      <Typography variant="h6">{token.name}</Typography>
                      <Typography variant="caption" color="text.secondary">{token.symbol}</Typography>
                    </Box>
                    {token.verified && (
                      <Chip icon={<Verified />} label="Verified" size="small" color="primary" />
                    )}
                  </Box>
                  <Divider sx={{ my: 2 }} />
                  <Grid container spacing={2}>
                    <Grid item xs={6}>
                      <Typography variant="caption" color="text.secondary">Price</Typography>
                      <Typography variant="body1" fontWeight="bold">{formatCurrency(token.price)}</Typography>
                    </Grid>
                    <Grid item xs={6}>
                      <Typography variant="caption" color="text.secondary">24h Volume</Typography>
                      <Typography variant="body1" fontWeight="bold">{formatCurrency(token.volume24h)}</Typography>
                    </Grid>
                    <Grid item xs={6}>
                      <Typography variant="caption" color="text.secondary">Market Cap</Typography>
                      <Typography variant="body1" fontWeight="bold">{formatCurrency(token.marketCap)}</Typography>
                    </Grid>
                    <Grid item xs={6}>
                      <Typography variant="caption" color="text.secondary">Holders</Typography>
                      <Typography variant="body1" fontWeight="bold">{token.holders.toLocaleString()}</Typography>
                    </Grid>
                  </Grid>
                </CardContent>
              </Card>
            </Grid>
          ))}
        </Grid>
      )}
      
      {/* Detail Dialog */}
      <Dialog open={detailDialogOpen} onClose={() => setDetailDialogOpen(false)} maxWidth="md" fullWidth>
        <DialogTitle>Listing Request Details</DialogTitle>
        <DialogContent>
          {selectedRequest && (
            <Box sx={{ pt: 2 }}>
              <Grid container spacing={3}>
                <Grid item xs={12}>
                  <Typography variant="h6">{selectedRequest.tokenName} ({selectedRequest.tokenSymbol})</Typography>
                  <Typography variant="body2" color="text.secondary">{selectedRequest.description}</Typography>
                </Grid>
                <Grid item xs={6}>
                  <Typography variant="caption" color="text.secondary">Token Address</Typography>
                  <Typography variant="body2" sx={{ fontFamily: 'monospace' }}>{selectedRequest.tokenAddress}</Typography>
                </Grid>
                <Grid item xs={6}>
                  <Typography variant="caption" color="text.secondary">Chain</Typography>
                  <Typography variant="body2">{selectedRequest.chain}</Typography>
                </Grid>
                <Grid item xs={6}>
                  <Typography variant="caption" color="text.secondary">Requester</Typography>
                  <Typography variant="body2" sx={{ fontFamily: 'monospace' }}>{selectedRequest.requester}</Typography>
                </Grid>
                <Grid item xs={6}>
                  <Typography variant="caption" color="text.secondary">Requested At</Typography>
                  <Typography variant="body2">{formatDate(selectedRequest.requestedAt)}</Typography>
                </Grid>
                <Grid item xs={6}>
                  <Typography variant="caption" color="text.secondary">KYC Status</Typography>
                  <Chip label={selectedRequest.kycStatus.toUpperCase()} size="small" />
                </Grid>
                <Grid item xs={6}>
                  <Typography variant="caption" color="text.secondary">Audit Status</Typography>
                  <Chip label={selectedRequest.auditStatus.toUpperCase()} size="small" />
                </Grid>
                <Grid item xs={6}>
                  <Typography variant="caption" color="text.secondary">Votes For</Typography>
                  <Typography variant="h6" color="success.main">{selectedRequest.votesFor}</Typography>
                </Grid>
                <Grid item xs={6}>
                  <Typography variant="caption" color="text.secondary">Votes Against</Typography>
                  <Typography variant="h6" color="error.main">{selectedRequest.votesAgainst}</Typography>
                </Grid>
              </Grid>
            </Box>
          )}
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setDetailDialogOpen(false)}>Close</Button>
          {selectedRequest?.status === 'pending' && (
            <>
              <Button color="error" onClick={() => {
                handleReject(selectedRequest.id);
                setDetailDialogOpen(false);
              }}>Reject</Button>
              <Button variant="contained" color="success" onClick={() => {
                handleApprove(selectedRequest.id);
                setDetailDialogOpen(false);
              }}>Approve</Button>
            </>
          )}
        </DialogActions>
      </Dialog>
    </Box>
  );
}