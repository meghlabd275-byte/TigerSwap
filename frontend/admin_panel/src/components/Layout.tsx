import React, { useState } from 'react'
import { Outlet } from 'react-router-dom'
import {
  AppBar, Toolbar, Typography, Drawer, List, ListItem, ListItemIcon, ListItemText,
  Box, IconButton, Avatar, Menu, MenuItem, Divider, useTheme, Badge, BadgeProps
} from '@mui/material'
import {
  Dashboard, People, Pool, Hub, Receipt, ShowChart, SmartToy,
  AccountTree, HubOutlined, AttachMoney, Security, Analytics, Settings,
  Menu as MenuIcon, Notifications, ChevronLeft, Brightness7, Brightness4
} from '@mui/icons-material'

const drawerWidth = 260

const menuItems = [
  { text: 'Dashboard', icon: <Dashboard />, path: '/dashboard' },
  { text: 'Users', icon: <People />, path: '/users' },
  { text: 'Pools', icon: <Pool />, path: '/pools' },
  { text: 'Bridges', icon: <Hub />, path: '/bridges' },
  { text: 'Transactions', icon: <Receipt />, path: '/transactions' },
  { text: 'Market Maker', icon: <ShowChart />, path: '/market-maker' },
  { text: 'Bots', icon: <SmartToy />, path: '/bots' },
  { text: 'Chains', icon: <AccountTree />, path: '/chains' },
  { text: 'DEXs', icon: <HubOutlined />, path: '/dexs' },
  { text: 'Fees', icon: <AttachMoney />, path: '/fees' },
  { text: 'Treasury', icon: <AttachMoney />, path: '/treasury' },
  { text: 'Security', icon: <Security />, path: '/security' },
  { text: 'Analytics', icon: <Analytics />, path: '/analytics' },
  { text: 'Settings', icon: <Settings />, path: '/settings' },
]

const Layout: React.FC = () => {
  const [mobileOpen, setMobileOpen] = useState(false)
  const [anchorEl, setAnchorEl] = useState<null | HTMLElement>(null)
  const theme = useTheme()

  const handleDrawerToggle = () => setMobileOpen(!mobileOpen)
  const handleProfileMenuOpen = (event: React.MouseEvent<HTMLElement>) => setAnchorEl(event.currentTarget)
  const handleMenuClose = () => setAnchorEl(null)

  const drawer = (
    <div>
      <Toolbar sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
        <Box component="span" sx={{ fontSize: 28, fontWeight: 'bold', color: 'primary.main' }}>🐯</Box>
        <Typography variant="h6" noWrap>TigerSwap</Typography>
      </Toolbar>
      <Divider />
      <List>
        {menuItems.map((item) => (
          <ListItem button key={item.text} component="a" href={item.path} sx={{ '&:hover': { bgcolor: 'action.hover' } }}>
            <ListItemIcon sx={{ minWidth: 40 }}>{item.icon}</ListItemIcon>
            <ListItemText primary={item.text} />
          </ListItem>
        ))}
      </List>
    </div>
  )

  return (
    <Box sx={{ display: 'flex' }}>
      <AppBar position="fixed" sx={{ zIndex: (theme) => theme.zIndex.drawer + 1 }}>
        <Toolbar>
          <IconButton color="inherit" edge="start" onClick={handleDrawerToggle} sx={{ mr: 2 }}>
            <MenuIcon />
          </IconButton>
          <Typography variant="h6" noWrap sx={{ flexGrow: 1 }}>Admin Panel</Typography>
          <IconButton color="inherit"><Badge badgeContent={4} color="error"><Notifications /></Badge></IconButton>
          <IconButton color="inherit" onClick={handleProfileMenuOpen}>
            <Avatar sx={{ width: 32, height: 32, bgcolor: 'primary.main' }}>A</Avatar>
          </IconButton>
        </Toolbar>
      </AppBar>
      <Box component="nav" sx={{ width: { md: drawerWidth }, flexShrink: { md: 0 } }}>
        <Drawer variant="temporary" open={mobileOpen} onClose={handleDrawerToggle} sx={{ '& .MuiDrawer-paper': { boxSizing: 'border-box', width: drawerWidth } }}>
          {drawer}
        </Drawer>
        <Drawer variant="permanent" sx={{ display: { xs: 'none', md: 'block' }, '& .MuiDrawer-paper': { boxSizing: 'border-box', width: drawerWidth } }} open>
          {drawer}
        </Drawer>
      </Box>
      <Box component="main" sx={{ flexGrow: 1, p: 3, width: { md: `calc(100% - ${drawerWidth}px)` }, mt: 8 }}>
        <Outlet />
      </Box>
      <Menu anchorEl={anchorEl} open={Boolean(anchorEl)} onClose={handleMenuClose}>
        <MenuItem onClick={handleMenuClose}>Profile</MenuItem>
        <MenuItem onClick={handleMenuClose}>Logout</MenuItem>
      </Menu>
    </Box>
  )
}

export default Layout