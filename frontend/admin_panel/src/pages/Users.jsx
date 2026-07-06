import React, { useState } from 'react';
import { Box, Typography, Table, TableBody, TableCell, TableContainer, TableHead, TableRow, Paper, TextField, InputAdornment, Button, Dialog, DialogTitle, DialogContent, DialogActions, Chip, Avatar, IconButton } from '@mui/material';
import { Search, Visibility, Edit, Delete, Add } from '@mui/icons-material';
const usersData = [
    { id: '1', address: '0x1234...abcd', email: 'user1@example.com', name: 'John Doe', createdAt: '2024-01-15', status: 'active', volume: '$125,000', trades: 342 },
    { id: '2', address: '0x5678...efgh', email: 'user2@example.com', name: 'Jane Smith', createdAt: '2024-01-18', status: 'active', volume: '$89,500', trades: 215 },
    { id: '3', address: '0x9abc...ijkl', email: 'user3@example.com', name: 'Bob Wilson', createdAt: '2024-01-20', status: 'suspended', volume: '$45,000', trades: 98 },
    { id: '4', address: '0xdef0...mnop', email: 'user4@example.com', name: 'Alice Brown', createdAt: '2024-01-22', status: 'active', volume: '$210,000', trades: 567 },
    { id: '5', address: '0x3456...qrst', email: 'user5@example.com', name: 'Charlie Davis', createdAt: '2024-01-25', status: 'active', volume: '$67,000', trades: 189 },
];
const Users = () => {
    const [searchTerm, setSearchTerm] = useState('');
    const [openDialog, setOpenDialog] = useState(false);
    const [selectedUser, setSelectedUser] = useState(null);
    const filteredUsers = usersData.filter(user => user.address.toLowerCase().includes(searchTerm.toLowerCase()) ||
        user.email.toLowerCase().includes(searchTerm.toLowerCase()) ||
        user.name.toLowerCase().includes(searchTerm.toLowerCase()));
    return (<Box>
      <Typography variant="h4" gutterBottom>User Management</Typography>
      <Box sx={{ display: 'flex', gap: 2, mb: 3 }}>
        <TextField placeholder="Search users..." value={searchTerm} onChange={(e) => setSearchTerm(e.target.value)} InputProps={{
            startAdornment: <InputAdornment position="start"><Search /></InputAdornment>,
        }} sx={{ width: 300 }}/>
        <Button variant="contained" startIcon={<Add />}>Add User</Button>
        <Button variant="outlined" color="error">Export</Button>
      </Box>

      <TableContainer component={Paper}>
        <Table>
          <TableHead>
            <TableRow>
              <TableCell>User</TableCell>
              <TableCell>Address</TableCell>
              <TableCell>Email</TableCell>
              <TableCell>Status</TableCell>
              <TableCell>Volume (30d)</TableCell>
              <TableCell>Trades</TableCell>
              <TableCell>Created</TableCell>
              <TableCell>Actions</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {filteredUsers.map((user) => (<TableRow key={user.id}>
                <TableCell>
                  <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                    <Avatar sx={{ width: 32, height: 32, bgcolor: 'primary.main' }}>{user.name[0]}</Avatar>
                    <Typography>{user.name}</Typography>
                  </Box>
                </TableCell>
                <TableCell><Chip label={user.address} size="small"/></TableCell>
                <TableCell>{user.email}</TableCell>
                <TableCell>
                  <Chip label={user.status} size="small" color={user.status === 'active' ? 'success' : 'error'}/>
                </TableCell>
                <TableCell>{user.volume}</TableCell>
                <TableCell>{user.trades}</TableCell>
                <TableCell>{user.createdAt}</TableCell>
                <TableCell>
                  <IconButton size="small"><Visibility /></IconButton>
                  <IconButton size="small"><Edit /></IconButton>
                  <IconButton size="small" color="error"><Delete /></IconButton>
                </TableCell>
              </TableRow>))}
          </TableBody>
        </Table>
      </TableContainer>

      <Dialog open={openDialog} onClose={() => setOpenDialog(false)}>
        <DialogTitle>User Details</DialogTitle>
        <DialogContent>
          {selectedUser && (<Box sx={{ minWidth: 400 }}>
              <Typography variant="body1"><strong>Address:</strong> {selectedUser.address}</Typography>
              <Typography variant="body1"><strong>Email:</strong> {selectedUser.email}</Typography>
              <Typography variant="body1"><strong>Name:</strong> {selectedUser.name}</Typography>
              <Typography variant="body1"><strong>Status:</strong> {selectedUser.status}</Typography>
              <Typography variant="body1"><strong>Volume:</strong> {selectedUser.volume}</Typography>
              <Typography variant="body1"><strong>Trades:</strong> {selectedUser.trades}</Typography>
            </Box>)}
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setOpenDialog(false)}>Close</Button>
        </DialogActions>
      </Dialog>
    </Box>);
};
export default Users;
//# sourceMappingURL=Users.jsx.map