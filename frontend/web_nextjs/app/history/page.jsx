'use client';
import React, { useState, useEffect, useCallback } from 'react';
import { Box, Typography, Card, CardContent, Button, TextField, Table, TableBody, TableCell, TableContainer, TableHead, TableRow, TablePagination, Chip, IconButton, Select, MenuItem, FormControl, InputLabel, ToggleButton, ToggleButtonGroup, Tooltip, Dialog, DialogTitle, DialogContent, DialogActions, LinearProgress } from '@mui/material';
import { FilterList, Download, Refresh, Visibility, OpenInNew, CheckCircle, Schedule, Cancel, ContentCopy } from '@mui/icons-material';
// ============================================================================
// Constants
// ============================================================================
const CHAIN_CONFIG = {
    1: { name: 'Ethereum', explorer: 'https://etherscan.io', color: '#627EEA' },
    56: { name: 'BNB Chain', explorer: 'https://bscscan.com', color: '#F3BA2F' },
    137: { name: 'Polygon', explorer: 'https://polygonscan.com', color: '#8247E5' },
    42161: { name: 'Arbitrum', explorer: 'https://arbiscan.io', color: '#28A0F0' },
    10: { name: 'Optimism', explorer: 'https://optimistic.etherscan.io', color: '#FF0420' },
    8453: { name: 'Base', explorer: 'https://basescan.org', color: '#0052FF' },
};
const TX_TYPES = [
    { value: 'swap', label: 'Swap', color: '#00d4aa' },
    { value: 'approve', label: 'Approve', color: '#FF007A' },
    { value: 'transfer', label: 'Transfer', color: '#6366f1' },
    { value: 'addLiquidity', label: 'Add Liquidity', color: '#22c55e' },
    { value: 'removeLiquidity', label: 'Remove Liquidity', color: '#f59e0b' },
    { value: 'stake', label: 'Stake', color: '#8b5cf6' },
    { value: 'unstake', label: 'Unstake', color: '#ec4899' },
    { value: 'claim', label: 'Claim', color: '#14b8a6' },
];
// ============================================================================
// Utility Functions
// ============================================================================
function formatAddress(address, chars = 4) {
    return `${address.slice(0, chars + 2)}...${address.slice(-chars)}`;
}
function formatTimestamp(timestamp, format = 'readable') {
    const date = new Date(timestamp);
    switch (format) {
        case 'iso':
            return date.toISOString();
        case 'unix':
            return Math.floor(timestamp / 1000).toString();
        case 'readable':
        default:
            const now = Date.now();
            const diff = now - timestamp;
            if (diff < 60000)
                return 'Just now';
            if (diff < 3600000)
                return `${Math.floor(diff / 60000)}m ago`;
            if (diff < 86400000)
                return `${Math.floor(diff / 3600000)}h ago`;
            if (diff < 604800000)
                return `${Math.floor(diff / 86400000)}d ago`;
            return date.toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' });
    }
}
function formatAmount(amount, decimals = 18) {
    if (!amount || amount === '0')
        return '0';
    try {
        const num = Number(amount) / Math.pow(10, decimals);
        if (num < 0.0001)
            return '<0.0001';
        return num.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 4 });
    }
    catch {
        return '0';
    }
}
function formatUSD(amount) {
    if (amount < 0.01)
        return '$0.00';
    return new Intl.NumberFormat('en-US', { style: 'currency', currency: 'USD' }).format(amount);
}
function formatGwei(weiHex) {
    if (!weiHex || weiHex === '0x0')
        return '0';
    const wei = parseInt(weiHex, 16);
    return (wei / 1e9).toFixed(2);
}
// ============================================================================
// Mock Data Generator
// ============================================================================
function generateMockTransactions(account, count = 50) {
    const transactions = [];
    const tokens = ['ETH', 'USDC', 'USDT', 'DAI', 'WBTC', 'LINK', 'UNI', 'AAVE'];
    const statuses = ['confirmed', 'confirmed', 'confirmed', 'pending', 'failed'];
    const types = ['swap', 'approve', 'transfer', 'addLiquidity', 'removeLiquidity'];
    for (let i = 0; i < count; i++) {
        const type = types[Math.floor(Math.random() * types.length)];
        const status = statuses[Math.floor(Math.random() * statuses.length)];
        const tokenInSymbol = tokens[Math.floor(Math.random() * tokens.length)];
        const tokenOutSymbol = tokens[Math.floor(Math.random() * tokens.length)];
        const amount = (Math.random() * 10 + 0.1).toFixed(4);
        transactions.push({
            hash: '0x' + Array.from({ length: 64 }, () => '0123456789abcdef'[Math.floor(Math.random() * 16)]).join(''),
            type,
            status,
            tokenIn: {
                symbol: tokenInSymbol,
                amount,
                amountUSD: Math.random() * 1000,
                address: '0x...',
            },
            tokenOut: type !== 'approve' && type !== 'transfer' ? {
                symbol: tokenOutSymbol,
                amount: (parseFloat(amount) * (0.8 + Math.random() * 0.2)).toFixed(4),
                amountUSD: Math.random() * 1000,
                address: '0x...',
            } : undefined,
            gasUsed: (21000 + Math.floor(Math.random() * 100000)).toString(),
            gasPrice: '0x' + (Math.floor(30 * 1e9)).toString(16),
            gasFee: (21000 * 30 * 1e9 / 1e18).toFixed(6),
            timestamp: Date.now() - Math.floor(Math.random() * 7 * 24 * 60 * 60 * 1000),
            blockNumber: 18000000 + Math.floor(Math.random() * 100000),
            from: account,
            to: '0x...' + Math.random().toString(16).slice(2, 6),
            chainId: [1, 56, 137, 42161][Math.floor(Math.random() * 4)],
            priceImpact: type === 'swap' ? Math.random() * 2 : undefined,
            route: type === 'swap' ? [tokenInSymbol, tokenOutSymbol] : undefined,
        });
    }
    return transactions.sort((a, b) => b.timestamp - a.timestamp);
}
// ============================================================================
// Main Transaction History Component
// ============================================================================
export default function TransactionHistoryPage() {
    // State
    const [transactions, setTransactions] = useState([]);
    const [filteredTransactions, setFilteredTransactions] = useState([]);
    const [loading, setLoading] = useState(true);
    const [selectedTx, setSelectedTx] = useState(null);
    const [showTxDetails, setShowTxDetails] = useState(false);
    const [page, setPage] = useState(0);
    const [rowsPerPage, setRowsPerPage] = useState(20);
    const [viewMode, setViewMode] = useState('table');
    const [exportDialog, setExportDialog] = useState(false);
    const [exportFormat, setExportFormat] = useState({
        type: 'csv',
        includeFailed: true,
        dateFormat: 'readable',
    });
    const [walletConnected, setWalletConnected] = useState(false);
    const [account, setAccount] = useState(null);
    // Filters
    const [filters, setFilters] = useState({
        type: [],
        status: [],
        chainId: 'all',
        dateFrom: '',
        dateTo: '',
        token: '',
        minAmount: '',
        searchQuery: '',
    });
    // ============================================================================
    // Load Transactions
    // ============================================================================
    const loadTransactions = useCallback(async () => {
        setLoading(true);
        // Simulate loading delay
        await new Promise(resolve => setTimeout(resolve, 500));
        // Generate or fetch transactions
        const mockAccount = account || '0x1234567890abcdef1234567890abcdef12345678';
        const txs = generateMockTransactions(mockAccount, 100);
        setTransactions(txs);
        setFilteredTransactions(txs);
        setLoading(false);
    }, [account]);
    useEffect(() => {
        loadTransactions();
    }, [loadTransactions]);
    // ============================================================================
    // Apply Filters
    // ============================================================================
    useEffect(() => {
        let result = [...transactions];
        // Type filter
        if (filters.type.length > 0) {
            result = result.filter(tx => filters.type.includes(tx.type));
        }
        // Status filter
        if (filters.status.length > 0) {
            result = result.filter(tx => filters.status.includes(tx.status));
        }
        // Chain filter
        if (filters.chainId !== 'all') {
            result = result.filter(tx => tx.chainId === filters.chainId);
        }
        // Date from
        if (filters.dateFrom) {
            const fromDate = new Date(filters.dateFrom).getTime();
            result = result.filter(tx => tx.timestamp >= fromDate);
        }
        // Date to
        if (filters.dateTo) {
            const toDate = new Date(filters.dateTo).getTime() + 86400000; // Include full day
            result = result.filter(tx => tx.timestamp <= toDate);
        }
        // Token filter
        if (filters.token) {
            const tokenLower = filters.token.toLowerCase();
            result = result.filter(tx => tx.tokenIn?.symbol.toLowerCase().includes(tokenLower) ||
                tx.tokenOut?.symbol.toLowerCase().includes(tokenLower) ||
                tx.tokenIn?.address.toLowerCase().includes(tokenLower));
        }
        // Min amount filter
        if (filters.minAmount) {
            const min = parseFloat(filters.minAmount);
            result = result.filter(tx => {
                const amount = parseFloat(tx.tokenIn?.amount || '0');
                return amount >= min;
            });
        }
        // Search query
        if (filters.searchQuery) {
            const query = filters.searchQuery.toLowerCase();
            result = result.filter(tx => tx.hash.toLowerCase().includes(query) ||
                tx.from.toLowerCase().includes(query) ||
                tx.to.toLowerCase().includes(query) ||
                tx.tokenIn?.symbol.toLowerCase().includes(query) ||
                tx.tokenOut?.symbol.toLowerCase().includes(query));
        }
        setFilteredTransactions(result);
        setPage(0);
    }, [transactions, filters]);
    // ============================================================================
    // Filter Handlers
    // ============================================================================
    const handleTypeFilterChange = (event) => {
        const value = event.target.value;
        setFilters(prev => ({
            ...prev,
            type: typeof value === 'string' ? value.split(',') : value,
        }));
    };
    const handleStatusFilterChange = (event) => {
        const value = event.target.value;
        setFilters(prev => ({
            ...prev,
            status: typeof value === 'string' ? value.split(',') : value,
        }));
    };
    const handleChainFilterChange = (event) => {
        const value = event.target.value;
        setFilters(prev => ({
            ...prev,
            chainId: value === 'all' ? 'all' : Number(value),
        }));
    };
    const clearFilters = () => {
        setFilters({
            type: [],
            status: [],
            chainId: 'all',
            dateFrom: '',
            dateTo: '',
            token: '',
            minAmount: '',
            searchQuery: '',
        });
    };
    // ============================================================================
    // Export Functions
    // ============================================================================
    const exportTransactions = () => {
        let dataToExport = filteredTransactions;
        if (!exportFormat.includeFailed) {
            dataToExport = dataToExport.filter(tx => tx.status !== 'failed');
        }
        switch (exportFormat.type) {
            case 'csv':
                exportCSV(dataToExport);
                break;
            case 'json':
                exportJSON(dataToExport);
                break;
            case 'pdf':
                exportPDF(dataToExport);
                break;
        }
        setExportDialog(false);
    };
    const exportCSV = (data) => {
        const headers = ['Hash', 'Type', 'Status', 'Token In', 'Amount In', 'Token Out', 'Amount Out', 'Gas Fee', 'Timestamp', 'Chain', 'Block'];
        const rows = data.map(tx => [
            tx.hash,
            tx.type,
            tx.status,
            tx.tokenIn?.symbol || '',
            tx.tokenIn?.amount || '',
            tx.tokenOut?.symbol || '',
            tx.tokenOut?.amount || '',
            tx.gasFee || '',
            formatTimestamp(tx.timestamp, exportFormat.dateFormat),
            CHAIN_CONFIG[tx.chainId]?.name || tx.chainId.toString(),
            tx.blockNumber?.toString() || '',
        ]);
        const csv = [headers, ...rows].map(row => row.map(cell => `"${cell}"`).join(',')).join('\n');
        downloadFile(csv, 'transactions.csv', 'text/csv');
    };
    const exportJSON = (data) => {
        const json = JSON.stringify(data, null, 2);
        downloadFile(json, 'transactions.json', 'application/json');
    };
    const exportPDF = (data) => {
        // For PDF, we create a simple HTML-based export
        const html = `
      <html>
        <head><title>Transaction History</title></head>
        <body>
          <h1>TigerSwap Transaction History</h1>
          <p>Generated: ${new Date().toLocaleString()}</p>
          <table border="1" style="border-collapse: collapse; width: 100%">
            <tr>
              ${['Hash', 'Type', 'Status', 'Token In', 'Amount', 'Token Out', 'Amount', 'Gas Fee', 'Date'].map(h => `<th>${h}</th>`).join('')}
            </tr>
            ${data.map(tx => `
              <tr>
                <td>${tx.hash.slice(0, 10)}...</td>
                <td>${tx.type}</td>
                <td>${tx.status}</td>
                <td>${tx.tokenIn?.symbol || ''}</td>
                <td>${tx.tokenIn?.amount || ''}</td>
                <td>${tx.tokenOut?.symbol || ''}</td>
                <td>${tx.tokenOut?.amount || ''}</td>
                <td>${tx.gasFee || ''}</td>
                <td>${formatTimestamp(tx.timestamp, 'readable')}</td>
              </tr>
            `).join('')}
          </table>
        </body>
      </html>
    `;
        downloadFile(html, 'transactions.html', 'text/html');
    };
    const downloadFile = (content, filename, mimeType) => {
        const blob = new Blob([content], { type: mimeType });
        const url = URL.createObjectURL(blob);
        const link = document.createElement('a');
        link.href = url;
        link.download = filename;
        document.body.appendChild(link);
        link.click();
        document.body.removeChild(link);
        URL.revokeObjectURL(url);
    };
    // ============================================================================
    // ContentCopy to Clipboard
    // ============================================================================
    const copyToClipboard = async (text) => {
        try {
            await navigator.clipboard.writeText(text);
            // Could show a snackbar here
        }
        catch (err) {
            console.error('Failed to copy:', err);
        }
    };
    // ============================================================================
    // Render Status Chip
    // ============================================================================
    const renderStatusChip = (status) => {
        const configs = {
            confirmed: { label: 'Confirmed', color: '#00d4aa', icon: <CheckCircle sx={{ fontSize: 14 }}/> },
            pending: { label: 'Pending', color: '#ffaa00', icon: <Schedule sx={{ fontSize: 14 }}/> },
            failed: { label: 'Failed', color: '#ff4757', icon: <Cancel sx={{ fontSize: 14 }}/> },
        };
        const config = configs[status];
        return (<Chip size="small" label={config.label} icon={config.icon} sx={{
                bgcolor: `${config.color}20`,
                color: config.color,
                borderColor: config.color,
                '& .MuiChip-icon': { color: config.color },
            }} variant="outlined"/>);
    };
    // ============================================================================
    // Render Type Chip
    // ============================================================================
    const renderTypeChip = (type) => {
        const config = TX_TYPES.find(t => t.value === type);
        const color = config?.color || '#666';
        return (<Chip size="small" label={config?.label || type} sx={{
                bgcolor: `${color}20`,
                color: color,
            }}/>);
    };
    // ============================================================================
    // Pagination
    // ============================================================================
    const handleChangePage = (_, newPage) => {
        setPage(newPage);
    };
    const handleChangeRowsPerPage = (event) => {
        setRowsPerPage(parseInt(event.target.value, 10));
        setPage(0);
    };
    // ============================================================================
    // Render
    // ============================================================================
    return (<Box sx={{ minHeight: '100vh', bgcolor: '#0a0a14', py: 4, px: { xs: 2, md: 4 } }}>
      <Box sx={{ maxWidth: 1400, mx: 'auto' }}>
        {/* Header */}
        <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 3, flexWrap: 'wrap', gap: 2 }}>
          <Typography variant="h5" sx={{ color: 'white', fontWeight: 700 }}>
            Transaction History
          </Typography>

          <Box sx={{ display: 'flex', gap: 2, alignItems: 'center' }}>
            <ToggleButtonGroup value={viewMode} exclusive onChange={(_, v) => v && setViewMode(v)} size="small" sx={{
            '& .MuiToggleButton-root': {
                color: '#9ca3af',
                borderColor: '#2a2a3e',
                '&.Mui-selected': { bgcolor: '#00d4ff', color: 'black' },
            },
        }}>
              <ToggleButton value="table">Table</ToggleButton>
              <ToggleButton value="card">Cards</ToggleButton>
            </ToggleButtonGroup>

            <Button variant="outlined" startIcon={<Download />} onClick={() => setExportDialog(true)} sx={{ borderColor: '#2a2a3e', color: 'white' }}>
              Export
            </Button>

            <Button variant="contained" startIcon={<Refresh />} onClick={loadTransactions} sx={{ bgcolor: '#00d4ff', color: 'black' }}>
              Refresh
            </Button>
          </Box>
        </Box>

        {/* Filters */}
        <Card sx={{ bgcolor: '#1a1a2e', borderRadius: 3, mb: 3 }}>
          <CardContent>
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 2, mb: 2 }}>
              <FilterList sx={{ color: '#00d4aa' }}/>
              <Typography variant="subtitle1" sx={{ color: 'white' }}>
                Filters
              </Typography>
              <Button size="small" onClick={clearFilters} sx={{ color: '#9ca3af', ml: 'auto' }}>
                Clear All
              </Button>
            </Box>

            <Box sx={{ display: 'flex', gap: 2, flexWrap: 'wrap' }}>
              {/* Search */}
              <TextField placeholder="Search by hash, address, token..." value={filters.searchQuery} onChange={(e) => setFilters(prev => ({ ...prev, searchQuery: e.target.value }))} size="small" sx={{
            minWidth: 250,
            '& .MuiOutlinedInput-root': {
                '& fieldset': { borderColor: '#3a3a4e' },
                '& input': { color: 'white' },
            },
        }}/>

              {/* Type Filter */}
              <FormControl size="small" sx={{ minWidth: 150 }}>
                <InputLabel sx={{ color: '#9ca3af' }}>Type</InputLabel>
                <Select multiple value={filters.type} onChange={handleTypeFilterChange} label="Type" sx={{
            color: 'white',
            '& .MuiOutlinedInput-notchedOutline': { borderColor: '#3a3a4e' },
            '& .MuiSelect-select': { py: 1 },
        }} renderValue={(selected) => (<Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 0.5 }}>
                      {selected.map((value) => (<Chip key={value} label={TX_TYPES.find(t => t.value === value)?.label || value} size="small"/>))}
                    </Box>)}>
                  {TX_TYPES.map((type) => (<MenuItem key={type.value} value={type.value}>
                      {type.label}
                    </MenuItem>))}
                </Select>
              </FormControl>

              {/* Status Filter */}
              <FormControl size="small" sx={{ minWidth: 130 }}>
                <InputLabel sx={{ color: '#9ca3af' }}>Status</InputLabel>
                <Select multiple value={filters.status} onChange={handleStatusFilterChange} label="Status" sx={{
            color: 'white',
            '& .MuiOutlinedInput-notchedOutline': { borderColor: '#3a3a4e' },
        }} renderValue={(selected) => (<Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 0.5 }}>
                      {selected.map((value) => (<Chip key={value} label={value} size="small"/>))}
                    </Box>)}>
                  <MenuItem value="confirmed">Confirmed</MenuItem>
                  <MenuItem value="pending">Pending</MenuItem>
                  <MenuItem value="failed">Failed</MenuItem>
                </Select>
              </FormControl>

              {/* Chain Filter */}
              <FormControl size="small" sx={{ minWidth: 130 }}>
                <InputLabel sx={{ color: '#9ca3af' }}>Chain</InputLabel>
                <Select value={filters.chainId} onChange={handleChainFilterChange} label="Chain" sx={{
            color: 'white',
            '& .MuiOutlinedInput-notchedOutline': { borderColor: '#3a3a4e' },
        }}>
                  <MenuItem value="all">All Chains</MenuItem>
                  {Object.entries(CHAIN_CONFIG).map(([id, config]) => (<MenuItem key={id} value={id}>{config.name}</MenuItem>))}
                </Select>
              </FormControl>

              {/* Date From */}
              <TextField type="date" label="From" value={filters.dateFrom} onChange={(e) => setFilters(prev => ({ ...prev, dateFrom: e.target.value }))} size="small" InputLabelProps={{ shrink: true }} sx={{
            '& .MuiOutlinedInput-root': {
                '& fieldset': { borderColor: '#3a3a4e' },
                '& input': { color: 'white' },
            },
        }}/>

              {/* Date To */}
              <TextField type="date" label="To" value={filters.dateTo} onChange={(e) => setFilters(prev => ({ ...prev, dateTo: e.target.value }))} size="small" InputLabelProps={{ shrink: true }} sx={{
            '& .MuiOutlinedInput-root': {
                '& fieldset': { borderColor: '#3a3a4e' },
                '& input': { color: 'white' },
            },
        }}/>

              {/* Min Amount */}
              <TextField type="number" placeholder="Min amount" value={filters.minAmount} onChange={(e) => setFilters(prev => ({ ...prev, minAmount: e.target.value }))} size="small" sx={{
            width: 120,
            '& .MuiOutlinedInput-root': {
                '& fieldset': { borderColor: '#3a3a4e' },
                '& input': { color: 'white' },
            },
        }}/>
            </Box>
          </CardContent>
        </Card>

        {/* Results Count */}
        <Box sx={{ mb: 2 }}>
          <Typography variant="body2" sx={{ color: '#9ca3af' }}>
            Showing {filteredTransactions.length} of {transactions.length} transactions
          </Typography>
        </Box>

        {/* Loading */}
        {loading && <LinearProgress sx={{ mb: 2, '& .MuiLinearProgress-bar': { bgcolor: '#00d4ff' } }}/>}

        {/* Table View */}
        {viewMode === 'table' && (<Card sx={{ bgcolor: '#1a1a2e', borderRadius: 3 }}>
            <TableContainer>
              <Table>
                <TableHead>
                  <TableRow>
                    <TableCell sx={{ color: '#9ca3af' }}>Type</TableCell>
                    <TableCell sx={{ color: '#9ca3af' }}>Status</TableCell>
                    <TableCell sx={{ color: '#9ca3af' }}>Token In</TableCell>
                    <TableCell sx={{ color: '#9ca3af' }}>Token Out</TableCell>
                    <TableCell sx={{ color: '#9ca3af' }}>Amount (USD)</TableCell>
                    <TableCell sx={{ color: '#9ca3af' }}>Price Impact</TableCell>
                    <TableCell sx={{ color: '#9ca3af' }}>Gas Fee</TableCell>
                    <TableCell sx={{ color: '#9ca3af' }}>Date</TableCell>
                    <TableCell sx={{ color: '#9ca3af' }}>Actions</TableCell>
                  </TableRow>
                </TableHead>
                <TableBody>
                  {filteredTransactions
                .slice(page * rowsPerPage, page * rowsPerPage + rowsPerPage)
                .map((tx) => (<TableRow key={tx.hash} sx={{
                    '&:hover': { bgcolor: '#2a2a3e' },
                    cursor: 'pointer',
                }} onClick={() => {
                    setSelectedTx(tx);
                    setShowTxDetails(true);
                }}>
                        <TableCell sx={{ color: 'white' }}>
                          {renderTypeChip(tx.type)}
                        </TableCell>
                        <TableCell>{renderStatusChip(tx.status)}</TableCell>
                        <TableCell>
                          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                            {tx.tokenIn?.symbol}
                            <Typography sx={{ color: '#9ca3af', fontSize: '0.8rem' }}>
                              {formatAmount(tx.tokenIn?.amount || '0')}
                            </Typography>
                          </Box>
                        </TableCell>
                        <TableCell>
                          {tx.tokenOut ? (<Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                              {tx.tokenOut.symbol}
                              <Typography sx={{ color: '#9ca3af', fontSize: '0.8rem' }}>
                                {formatAmount(tx.tokenOut.amount || '0')}
                              </Typography>
                            </Box>) : (<Typography sx={{ color: '#666' }}>-</Typography>)}
                        </TableCell>
                        <TableCell>
                          {tx.tokenIn?.amountUSD && (<Typography sx={{ color: '#00d4aa' }}>
                              {formatUSD(tx.tokenIn.amountUSD)}
                            </Typography>)}
                        </TableCell>
                        <TableCell>
                          {tx.priceImpact !== undefined && (<Typography sx={{
                        color: tx.priceImpact > 5 ? '#ff4757' : '#00d4aa',
                    }}>
                              {tx.priceImpact.toFixed(2)}%
                            </Typography>)}
                        </TableCell>
                        <TableCell>
                          <Typography sx={{ color: '#9ca3af', fontSize: '0.85rem' }}>
                            {tx.gasFee ? `${tx.gasFee} ETH` : '-'}
                          </Typography>
                        </TableCell>
                        <TableCell>
                          <Typography sx={{ color: '#9ca3af', fontSize: '0.85rem' }}>
                            {formatTimestamp(tx.timestamp)}
                          </Typography>
                        </TableCell>
                        <TableCell>
                          <Box sx={{ display: 'flex', gap: 0.5 }}>
                            <Tooltip title="View Details">
                              <IconButton size="small" onClick={(e) => {
                    e.stopPropagation();
                    setSelectedTx(tx);
                    setShowTxDetails(true);
                }} sx={{ color: '#9ca3af' }}>
                                <Visibility fontSize="small"/>
                              </IconButton>
                            </Tooltip>
                            <Tooltip title="ContentCopy Hash">
                              <IconButton size="small" onClick={(e) => {
                    e.stopPropagation();
                    copyToClipboard(tx.hash);
                }} sx={{ color: '#9ca3af' }}>
                                <ContentCopy fontSize="small"/>
                              </IconButton>
                            </Tooltip>
                            <Tooltip title="View on Explorer">
                              <IconButton size="small" onClick={(e) => {
                    e.stopPropagation();
                    window.open(`${CHAIN_CONFIG[tx.chainId]?.explorer}/tx/${tx.hash}`, '_blank');
                }} sx={{ color: '#9ca3af' }}>
                                <OpenInNew fontSize="small"/>
                              </IconButton>
                            </Tooltip>
                          </Box>
                        </TableCell>
                      </TableRow>))}
                </TableBody>
              </Table>
            </TableContainer>
            <TablePagination component="div" count={filteredTransactions.length} page={page} onPageChange={handleChangePage} rowsPerPage={rowsPerPage} onRowsPerPageChange={handleChangeRowsPerPage} rowsPerPageOptions={[10, 20, 50, 100]} sx={{
                color: '#9ca3af',
                '& .MuiTablePagination-select': { color: 'white' },
            }}/>
          </Card>)}

        {/* Card View */}
        {viewMode === 'card' && (<Box sx={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(350px, 1fr))', gap: 2 }}>
            {filteredTransactions
                .slice(page * rowsPerPage, page * rowsPerPage + rowsPerPage)
                .map((tx) => (<Card key={tx.hash} sx={{
                    bgcolor: '#1a1a2e',
                    borderRadius: 3,
                    cursor: 'pointer',
                    '&:hover': { bgcolor: '#2a2a3e' },
                }} onClick={() => {
                    setSelectedTx(tx);
                    setShowTxDetails(true);
                }}>
                  <CardContent>
                    <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', mb: 2 }}>
                      <Box>
                        {renderTypeChip(tx.type)}
                        <Box sx={{ mt: 1 }}>{renderStatusChip(tx.status)}</Box>
                      </Box>
                      <Typography variant="caption" sx={{ color: '#9ca3af' }}>
                        {formatTimestamp(tx.timestamp)}
                      </Typography>
                    </Box>

                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 2, mb: 2 }}>
                      <Box sx={{ flex: 1 }}>
                        <Typography sx={{ color: '#9ca3af', fontSize: '0.75rem' }}>From</Typography>
                        <Typography sx={{ color: 'white' }}>
                          {tx.tokenIn?.symbol} {formatAmount(tx.tokenIn?.amount || '0')}
                        </Typography>
                      </Box>
                      <ArrowForward sx={{ color: '#00d4aa' }}/>
                      <Box sx={{ flex: 1 }}>
                        <Typography sx={{ color: '#9ca3af', fontSize: '0.75rem' }}>To</Typography>
                        <Typography sx={{ color: 'white' }}>
                          {tx.tokenOut?.symbol || '-'} {tx.tokenOut ? formatAmount(tx.tokenOut.amount) : ''}
                        </Typography>
                      </Box>
                    </Box>

                    <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                      <Typography variant="caption" sx={{ color: '#9ca3af' }}>
                        {formatAddress(tx.hash)}
                      </Typography>
                      <Chip size="small" label={CHAIN_CONFIG[tx.chainId]?.name || `Chain ${tx.chainId}`} sx={{ bgcolor: CHAIN_CONFIG[tx.chainId]?.color, color: 'white', fontSize: '0.7rem' }}/>
                    </Box>
                  </CardContent>
                </Card>))}
          </Box>)}

        {/* Pagination for card view */}
        {viewMode === 'card' && (<Box sx={{ mt: 2, display: 'flex', justifyContent: 'center' }}>
            <TablePagination component="div" count={filteredTransactions.length} page={page} onPageChange={handleChangePage} rowsPerPage={rowsPerPage} onRowsPerPageChange={handleChangeRowsPerPage} rowsPerPageOptions={[12, 24, 48]} sx={{ color: '#9ca3af' }}/>
          </Box>)}
      </Box>

      {/* Transaction Details Dialog */}
      <Dialog open={showTxDetails} onClose={() => setShowTxDetails(false)} PaperProps={{ sx: { bgcolor: '#1a1a2e', backgroundImage: 'none', maxWidth: 600 } }}>
        {selectedTx && (<>
            <DialogTitle sx={{ color: 'white', display: 'flex', justifyContent: 'space-between' }}>
              Transaction Details
              <IconButton onClick={() => setShowTxDetails(false)} sx={{ color: 'white' }}>
                <Cancel />
              </IconButton>
            </DialogTitle>
            <DialogContent>
              <Box sx={{ mb: 3 }}>
                <Typography variant="caption" sx={{ color: '#9ca3af' }}>Hash</Typography>
                <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                  <Typography sx={{ color: 'white', wordBreak: 'break-all' }}>
                    {selectedTx.hash}
                  </Typography>
                  <IconButton size="small" onClick={() => copyToClipboard(selectedTx.hash)} sx={{ color: '#9ca3af' }}>
                    <ContentCopy fontSize="small"/>
                  </IconButton>
                </Box>
              </Box>

              <Box sx={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 2, mb: 3 }}>
                <Box>
                  <Typography variant="caption" sx={{ color: '#9ca3af' }}>Type</Typography>
                  <Box>{renderTypeChip(selectedTx.type)}</Box>
                </Box>
                <Box>
                  <Typography variant="caption" sx={{ color: '#9ca3af' }}>Status</Typography>
                  <Box sx={{ mt: 0.5 }}>{renderStatusChip(selectedTx.status)}</Box>
                </Box>
              </Box>

              <Box sx={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 2, mb: 3 }}>
                <Box>
                  <Typography variant="caption" sx={{ color: '#9ca3af' }}>Token In</Typography>
                  <Typography sx={{ color: 'white' }}>
                    {selectedTx.tokenIn?.symbol} {formatAmount(selectedTx.tokenIn?.amount || '0')}
                  </Typography>
                </Box>
                <Box>
                  <Typography variant="caption" sx={{ color: '#9ca3af' }}>Token Out</Typography>
                  <Typography sx={{ color: 'white' }}>
                    {selectedTx.tokenOut?.symbol || '-'} {selectedTx.tokenOut ? formatAmount(selectedTx.tokenOut.amount) : ''}
                  </Typography>
                </Box>
              </Box>

              <Box sx={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 2, mb: 3 }}>
                <Box>
                  <Typography variant="caption" sx={{ color: '#9ca3af' }}>Amount USD</Typography>
                  <Typography sx={{ color: '#00d4aa' }}>
                    {selectedTx.tokenIn?.amountUSD ? formatUSD(selectedTx.tokenIn.amountUSD) : '-'}
                  </Typography>
                </Box>
                <Box>
                  <Typography variant="caption" sx={{ color: '#9ca3af' }}>Price Impact</Typography>
                  <Typography sx={{ color: selectedTx.priceImpact && selectedTx.priceImpact > 5 ? '#ff4757' : '#00d4aa' }}>
                    {selectedTx.priceImpact !== undefined ? `${selectedTx.priceImpact.toFixed(2)}%` : '-'}
                  </Typography>
                </Box>
              </Box>

              <Box sx={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 2, mb: 3 }}>
                <Box>
                  <Typography variant="caption" sx={{ color: '#9ca3af' }}>Gas Used</Typography>
                  <Typography sx={{ color: 'white' }}>{selectedTx.gasUsed || '-'}</Typography>
                </Box>
                <Box>
                  <Typography variant="caption" sx={{ color: '#9ca3af' }}>Gas Fee</Typography>
                  <Typography sx={{ color: 'white' }}>{selectedTx.gasFee ? `${selectedTx.gasFee} ETH` : '-'}</Typography>
                </Box>
              </Box>

              <Box sx={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 2, mb: 3 }}>
                <Box>
                  <Typography variant="caption" sx={{ color: '#9ca3af' }}>Block</Typography>
                  <Typography sx={{ color: 'white' }}>{selectedTx.blockNumber || '-'}</Typography>
                </Box>
                <Box>
                  <Typography variant="caption" sx={{ color: '#9ca3af' }}>Chain</Typography>
                  <Typography sx={{ color: 'white' }}>
                    {CHAIN_CONFIG[selectedTx.chainId]?.name || `Chain ${selectedTx.chainId}`}
                  </Typography>
                </Box>
              </Box>

              <Box sx={{ mb: 2 }}>
                <Typography variant="caption" sx={{ color: '#9ca3af' }}>From</Typography>
                <Typography sx={{ color: 'white' }}>{formatAddress(selectedTx.from, 8)}</Typography>
              </Box>

              <Box sx={{ mb: 2 }}>
                <Typography variant="caption" sx={{ color: '#9ca3af' }}>To</Typography>
                <Typography sx={{ color: 'white' }}>{formatAddress(selectedTx.to, 8)}</Typography>
              </Box>

              {selectedTx.route && selectedTx.route.length > 0 && (<Box>
                  <Typography variant="caption" sx={{ color: '#9ca3af' }}>Route</Typography>
                  <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, flexWrap: 'wrap', mt: 0.5 }}>
                    {selectedTx.route.map((token, i) => (<React.Fragment key={i}>
                        <Chip label={token} size="small" sx={{ bgcolor: '#2a2a3e', color: 'white' }}/>
                        {i < selectedTx.route.length - 1 && <ArrowForward sx={{ color: '#9ca3af', fontSize: 16 }}/>}
                      </React.Fragment>))}
                  </Box>
                </Box>)}
            </DialogContent>
            <DialogActions>
              <Button href={`${CHAIN_CONFIG[selectedTx.chainId]?.explorer}/tx/${selectedTx.hash}`} target="_blank" endIcon={<OpenInNew />} sx={{ color: '#00d4aa' }}>
                View on Explorer
              </Button>
            </DialogActions>
          </>)}
      </Dialog>

      {/* Export Dialog */}
      <Dialog open={exportDialog} onClose={() => setExportDialog(false)} PaperProps={{ sx: { bgcolor: '#1a1a2e', backgroundImage: 'none' } }}>
        <DialogTitle sx={{ color: 'white' }}>Export Transactions</DialogTitle>
        <DialogContent>
          <FormControl fullWidth sx={{ mt: 2 }}>
            <InputLabel sx={{ color: '#9ca3af' }}>Format</InputLabel>
            <Select value={exportFormat.type} onChange={(e) => setExportFormat(prev => ({ ...prev, type: e.target.value }))} sx={{ color: 'white', '& .MuiOutlinedInput-notchedOutline': { borderColor: '#3a3a4e' } }}>
              <MenuItem value="csv">CSV</MenuItem>
              <MenuItem value="json">JSON</MenuItem>
              <MenuItem value="pdf">PDF (HTML)</MenuItem>
            </Select>
          </FormControl>

          <FormControl fullWidth sx={{ mt: 2 }}>
            <InputLabel sx={{ color: '#9ca3af' }}>Date Format</InputLabel>
            <Select value={exportFormat.dateFormat} onChange={(e) => setExportFormat(prev => ({ ...prev, dateFormat: e.target.value }))} sx={{ color: 'white', '& .MuiOutlinedInput-notchedOutline': { borderColor: '#3a3a4e' } }}>
              <MenuItem value="readable">Readable (Dec 25, 2024)</MenuItem>
              <MenuItem value="iso">ISO (2024-12-25T12:00:00Z)</MenuItem>
              <MenuItem value="unix">Unix Timestamp</MenuItem>
            </Select>
          </FormControl>

          <Box sx={{ mt: 2 }}>
            <Typography variant="body2" sx={{ color: '#9ca3af', mb: 1 }}>
              Include {filteredTransactions.length} transactions
            </Typography>
          </Box>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setExportDialog(false)} sx={{ color: '#9ca3af' }}>Cancel</Button>
          <Button onClick={exportTransactions} variant="contained" sx={{ bgcolor: '#00d4ff', color: 'black' }}>
            Export
          </Button>
        </DialogActions>
      </Dialog>
    </Box>);
}
//# sourceMappingURL=page.jsx.map