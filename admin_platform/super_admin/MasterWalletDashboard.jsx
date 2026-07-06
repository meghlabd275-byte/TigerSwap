// TigerSwap Admin Dashboard - Master Wallet Control Panel
// Comprehensive admin interface for wallet management
import React, { useState } from 'react';
import { Box, Typography, Grid, Card, CardContent, Button, Table, TableBody, TableCell, TableContainer, TableHead, TableRow, TextField, Select, MenuItem, Dialog, DialogTitle, DialogContent, DialogActions, Tabs, Tab, Chip, IconButton, Switch, FormControl, InputLabel } from '@mui/material';
import { Add, Edit, Delete, Visibility, AccountBalance, Token, Hub, TrendingUp, Wallet, Security, Backup } from '@mui/icons-material';
export const ChainManagement = ({ enabledChains, onAddChain, onRemoveChain, onToggleChain }) => {
    const [chains, setChains] = useState([
        { id: 1, name: 'Ethereum', type: 'EVM', isEnabled: true },
        { id: 56, name: 'BNB Chain', type: 'EVM', isEnabled: true },
        { id: 137, name: 'Polygon', type: 'EVM', isEnabled: true },
        { id: 42161, name: 'Arbitrum', type: 'EVM', isEnabled: true },
        { id: 10, name: 'Optimism', type: 'EVM', isEnabled: true },
        { id: 43114, name: 'Avalanche', type: 'EVM', isEnabled: true },
        { id: 43114, name: 'Solana', type: 'Solana', isEnabled: false },
        { id: 728126428, name: 'Tron', type: 'Tron', isEnabled: false },
    ]);
    return (<Box>
      <Box sx={{ display: 'flex', justifyContent: 'space-between', mb: 2 }}>
        <Typography variant="h6">Blockchain Networks</Typography>
        <Button variant="contained" startIcon={<Add />}>Add Network</Button>
      </Box>

      <TableContainer>
        <Table>
          <TableHead>
            <TableRow>
              <TableCell>Chain ID</TableCell>
              <TableCell>Name</TableCell>
              <TableCell>Type</TableCell>
              <TableCell>Status</TableCell>
              <TableCell>Actions</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {chains.map(chain => (<TableRow key={chain.id}>
                <TableCell>{chain.id}</TableCell>
                <TableCell>{chain.name}</TableCell>
                <TableCell><Chip label={chain.type} size="small"/></TableCell>
                <TableCell>
                  <Switch checked={chain.isEnabled} onChange={() => onToggleChain(chain.id)}/>
                </TableCell>
                <TableCell>
                  <IconButton size="small"><Edit /></IconButton>
                  <IconButton size="small" color="error" onClick={() => onRemoveChain(chain.id)}>
                    <Delete />
                  </IconButton>
                </TableCell>
              </TableRow>))}
          </TableBody>
        </Table>
      </TableContainer>
    </Box>);
};
export const TokenManagement = ({ tokens, onAddToken, onRemoveToken, onToggleToken }) => {
    const [showAddDialog, setShowAddDialog] = useState(false);
    const mockTokens = [
        { symbol: 'ETH', name: 'Ethereum', address: '0x...', chainId: 1, isEnabled: true, isStable: false },
        { symbol: 'USDT', name: 'Tether USD', address: '0xdAC17...', chainId: 1, isEnabled: true, isStable: true },
        { symbol: 'USDC', name: 'USD Coin', address: '0xA0b86...', chainId: 1, isEnabled: true, isStable: true },
        { symbol: 'BNB', name: 'BNB', address: '0x...', chainId: 56, isEnabled: true, isStable: false },
        { symbol: 'CAKE', name: 'PancakeSwap', address: '0x0E09...', chainId: 56, isEnabled: true, isStable: false },
        { symbol: 'MATIC', name: 'Polygon', address: '0x...', chainId: 137, isEnabled: true, isStable: false },
    ];
    return (<Box>
      <Box sx={{ display: 'flex', justifyContent: 'space-between', mb: 2 }}>
        <Typography variant="h6">Token Management</Typography>
        <Button variant="contained" startIcon={<Add />} onClick={() => setShowAddDialog(true)}>
          Add Token
        </Button>
      </Box>

      <TableContainer>
        <Table>
          <TableHead>
            <TableRow>
              <TableCell>Symbol</TableCell>
              <TableCell>Name</TableCell>
              <TableCell>Address</TableCell>
              <TableCell>Chain</TableCell>
              <TableCell>Type</TableCell>
              <TableCell>Status</TableCell>
              <TableCell>Actions</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {mockTokens.map((token, index) => (<TableRow key={index}>
                <TableCell><Chip label={token.symbol} color="primary" size="small"/></TableCell>
                <TableCell>{token.name}</TableCell>
                <TableCell><Typography variant="caption">{token.address}</Typography></TableCell>
                <TableCell>{token.chainId}</TableCell>
                <TableCell>
                  {token.isStable ? (<Chip label="Stable" color="success" size="small"/>) : (<Chip label="Crypto" size="small"/>)}
                </TableCell>
                <TableCell>
                  <Switch checked={token.isEnabled}/>
                </TableCell>
                <TableCell>
                  <IconButton size="small"><Edit /></IconButton>
                  <IconButton size="small" color="error"><Delete /></IconButton>
                </TableCell>
              </TableRow>))}
          </TableBody>
        </Table>
      </TableContainer>

      <Dialog open={showAddDialog} onClose={() => setShowAddDialog(false)}>
        <DialogTitle>Add New Token</DialogTitle>
        <DialogContent>
          <TextField label="Symbol" fullWidth margin="dense"/>
          <TextField label="Name" fullWidth margin="dense"/>
          <TextField label="Contract Address" fullWidth margin="dense"/>
          <TextField label="Decimals" type="number" fullWidth margin="dense"/>
          <FormControl fullWidth margin="dense">
            <InputLabel>Chain</InputLabel>
            <Select label="Chain">
              <MenuItem value={1}>Ethereum</MenuItem>
              <MenuItem value={56}>BNB Chain</MenuItem>
              <MenuItem value={137}>Polygon</MenuItem>
            </Select>
          </FormControl>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setShowAddDialog(false)}>Cancel</Button>
          <Button variant="contained" onClick={() => setShowAddDialog(false)}>Add Token</Button>
        </DialogActions>
      </Dialog>
    </Box>);
};
export const FeeManagement = ({ fees, onUpdateFees }) => {
    const [feeConfig, setFeeConfig] = useState(fees || {
        withdrawFee: '0.001',
        withdrawFeeType: 'fixed',
        swapFee: '0.3',
        swapFeeType: 'percentage',
        bridgeFee: '0.01',
        bridgeFeeType: 'percentage',
        minWithdrawAmount: '10',
        maxWithdrawAmount: '1000000',
    });
    return (<Box>
      <Typography variant="h6" gutterBottom>Fee Configuration</Typography>

      <Grid container spacing={3}>
        <Grid item xs={12} md={6}>
          <Card>
            <CardContent>
              <Typography variant="subtitle1" gutterBottom>Withdrawal Fees</Typography>
              <TextField label="Fee Amount" value={feeConfig.withdrawFee} onChange={(e) => setFeeConfig({ ...feeConfig, withdrawFee: e.target.value })} fullWidth margin="dense"/>
              <FormControl fullWidth margin="dense">
                <InputLabel>Fee Type</InputLabel>
                <Select value={feeConfig.withdrawFeeType} onChange={(e) => setFeeConfig({ ...feeConfig, withdrawFeeType: e.target.value })}>
                  <MenuItem value="fixed">Fixed</MenuItem>
                  <MenuItem value="percentage">Percentage</MenuItem>
                </Select>
              </FormControl>
              <TextField label="Min Withdraw Amount" value={feeConfig.minWithdrawAmount} onChange={(e) => setFeeConfig({ ...feeConfig, minWithdrawAmount: e.target.value })} fullWidth margin="dense"/>
              <TextField label="Max Withdraw Amount" value={feeConfig.maxWithdrawAmount} onChange={(e) => setFeeConfig({ ...feeConfig, maxWithdrawAmount: e.target.value })} fullWidth margin="dense"/>
            </CardContent>
          </Card>
        </Grid>

        <Grid item xs={12} md={6}>
          <Card>
            <CardContent>
              <Typography variant="subtitle1" gutterBottom>Swap & Bridge Fees</Typography>
              <TextField label="Swap Fee (%)" value={feeConfig.swapFee} onChange={(e) => setFeeConfig({ ...feeConfig, swapFee: e.target.value })} fullWidth margin="dense"/>
              <TextField label="Bridge Fee (%)" value={feeConfig.bridgeFee} onChange={(e) => setFeeConfig({ ...feeConfig, bridgeFee: e.target.value })} fullWidth margin="dense"/>
            </CardContent>
          </Card>
        </Grid>
      </Grid>

      <Box sx={{ mt: 3 }}>
        <Button variant="contained" onClick={() => onUpdateFees(feeConfig)}>Save Fee Configuration</Button>
      </Box>
    </Box>);
};
export const UserWalletManagement = ({ wallets, onViewWallet, onRecoverWallet, onFreezeWallet }) => {
    const mockWallets = [
        { id: '1', address: '0x1234...abcd', name: 'User Wallet 1', chainId: 1, volume: '$12,500', status: 'active', createdAt: '2024-01-15' },
        { id: '2', address: '0x5678...efgh', name: 'User Wallet 2', chainId: 56, volume: '$8,200', status: 'active', createdAt: '2024-01-18' },
        { id: '3', address: '0x9abc...ijkl', name: 'User Wallet 3', chainId: 137, volume: '$5,600', status: 'frozen', createdAt: '2024-01-20' },
        { id: '4', address: '0xdef0...mnop', name: 'User Wallet 4', chainId: 1, volume: '$25,000', status: 'active', createdAt: '2024-01-22' },
    ];
    return (<Box>
      <Box sx={{ display: 'flex', justifyContent: 'space-between', mb: 2 }}>
        <Typography variant="h6">User Wallets</Typography>
        <Button variant="outlined">Export All Wallets</Button>
      </Box>

      <TableContainer>
        <Table>
          <TableHead>
            <TableRow>
              <TableCell>Wallet ID</TableCell>
              <TableCell>Address</TableCell>
              <TableCell>Name</TableCell>
              <TableCell>Chain</TableCell>
              <TableCell>Volume (30d)</TableCell>
              <TableCell>Status</TableCell>
              <TableCell>Created</TableCell>
              <TableCell>Actions</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {mockWallets.map(wallet => (<TableRow key={wallet.id}>
                <TableCell>{wallet.id}</TableCell>
                <TableCell><Chip label={wallet.address} size="small"/></TableCell>
                <TableCell>{wallet.name}</TableCell>
                <TableCell>{wallet.chainId}</TableCell>
                <TableCell>{wallet.volume}</TableCell>
                <TableCell>
                  <Chip label={wallet.status} size="small" color={wallet.status === 'active' ? 'success' : 'error'}/>
                </TableCell>
                <TableCell>{wallet.createdAt}</TableCell>
                <TableCell>
                  <IconButton size="small" onClick={() => onViewWallet(wallet.id)}>
                    <Visibility />
                  </IconButton>
                  <IconButton size="small" onClick={() => onRecoverWallet(wallet.id)}>
                    <Backup />
                  </IconButton>
                  <IconButton size="small" color="error" onClick={() => onFreezeWallet(wallet.id)}>
                    <Security />
                  </IconButton>
                </TableCell>
              </TableRow>))}
          </TableBody>
        </Table>
      </TableContainer>
    </Box>);
};
export const MasterWalletDashboard = ({ masterWallet }) => {
    const [tabIndex, setTabIndex] = useState(0);
    return (<Box>
      <Card sx={{ mb: 3 }}>
        <CardContent>
          <Grid container spacing={3}>
            <Grid item xs={12} md={4}>
              <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
                <AccountBalance sx={{ fontSize: 40, color: 'primary.main' }}/>
                <Box>
                  <Typography variant="h6">Master Wallet</Typography>
                  <Typography variant="body2" color="textSecondary">
                    {masterWallet?.address || 'Not initialized'}
                  </Typography>
                </Box>
              </Box>
            </Grid>
            <Grid item xs={12} md={8}>
              <Grid container spacing={2}>
                <Grid item xs={3}>
                  <Typography variant="caption" color="textSecondary">Total Users</Typography>
                  <Typography variant="h5">154,289</Typography>
                </Grid>
                <Grid item xs={3}>
                  <Typography variant="caption" color="textSecondary">Total Wallets</Typography>
                  <Typography variant="h5">245,678</Typography>
                </Grid>
                <Grid item xs={3}>
                  <Typography variant="caption" color="textSecondary">Total Volume</Typography>
                  <Typography variant="h5">$2.1B</Typography>
                </Grid>
                <Grid item xs={3}>
                  <Typography variant="caption" color="textSecondary">Revenue</Typography>
                  <Typography variant="h5" color="success.main">$1.2M</Typography>
                </Grid>
              </Grid>
            </Grid>
          </Grid>
        </CardContent>
      </Card>

      <Tabs value={tabIndex} onChange={(_, v) => setTabIndex(v)} sx={{ mb: 2 }}>
        <Tab label="User Wallets" icon={<Wallet />}/>
        <Tab label="Chains" icon={<Hub />}/>
        <Tab label="Tokens" icon={<Token />}/>
        <Tab label="Fees" icon={<TrendingUp />}/>
        <Tab label="Backup" icon={<Backup />}/>
      </Tabs>

      {tabIndex === 0 && (<UserWalletManagement wallets={[]} onViewWallet={(id) => console.log('View', id)} onRecoverWallet={(id) => console.log('Recover', id)} onFreezeWallet={(id) => console.log('Freeze', id)}/>)}
      {tabIndex === 1 && (<ChainManagement enabledChains={[1, 56, 137]} onAddChain={(c) => console.log('Add chain', c)} onRemoveChain={(id) => console.log('Remove', id)} onToggleChain={(id) => console.log('Toggle', id)}/>)}
      {tabIndex === 2 && (<TokenManagement tokens={[]} onAddToken={(t) => console.log('Add token', t)} onRemoveToken={(a, c) => console.log('Remove', a, c)} onToggleToken={(a, c) => console.log('Toggle', a, c)}/>)}
      {tabIndex === 3 && (<FeeManagement fees={{}} onUpdateFees={(f) => console.log('Update fees', f)}/>)}
      {tabIndex === 4 && (<Box>
          <Typography variant="h6">System Backup</Typography>
          <Card sx={{ mt: 2 }}>
            <CardContent>
              <Typography variant="body2" color="textSecondary" gutterBottom>
                Backup codes are automatically saved to the admin dashboard
              </Typography>
              <Button variant="outlined" sx={{ mt: 2 }}>Generate New Backup</Button>
            </CardContent>
          </Card>
        </Box>)}
    </Box>);
};
export default MasterWalletDashboard;
//# sourceMappingURL=MasterWalletDashboard.jsx.map