import React from 'react';
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { CssBaseline, ThemeProvider, createTheme } from '@mui/material';
import Layout from './components/Layout';
import Dashboard from './pages/Dashboard';
import Users from './pages/Users';
import Pools from './pages/Pools';
import Bridges from './pages/Bridges';
import Transactions from './pages/Transactions';
import MarketMaker from './pages/MarketMaker';
import Bots from './pages/Bots';
import Chains from './pages/Chains';
import DEXs from './pages/DEXs';
import Fees from './pages/Fees';
import Treasury from './pages/Treasury';
import Security from './pages/Security';
import Analytics from './pages/Analytics';
import Settings from './pages/Settings';
const queryClient = new QueryClient({
    defaultOptions: {
        queries: {
            refetchOnWindowFocus: false,
            retry: 1,
        },
    },
});
const theme = createTheme({
    palette: {
        primary: { main: '#f97316' },
        secondary: { main: '#1e293b' },
        background: { default: '#0f172a', paper: '#1e293b' },
        text: { primary: '#f8fafc', secondary: '#94a3b8' },
    },
    typography: {
        fontFamily: '"Inter", "Roboto", "Helvetica", "Arial", sans-serif',
    },
});
function App() {
    return (<QueryClientProvider client={queryClient}>
      <ThemeProvider theme={theme}>
        <CssBaseline />
        <BrowserRouter>
          <Routes>
            <Route path="/" element={<Layout />}>
              <Route index element={<Navigate to="/dashboard" replace/>}/>
              <Route path="dashboard" element={<Dashboard />}/>
              <Route path="users" element={<Users />}/>
              <Route path="pools" element={<Pools />}/>
              <Route path="bridges" element={<Bridges />}/>
              <Route path="transactions" element={<Transactions />}/>
              <Route path="market-maker" element={<MarketMaker />}/>
              <Route path="bots" element={<Bots />}/>
              <Route path="chains" element={<Chains />}/>
              <Route path="dexs" element={<DEXs />}/>
              <Route path="fees" element={<Fees />}/>
              <Route path="treasury" element={<Treasury />}/>
              <Route path="security" element={<Security />}/>
              <Route path="analytics" element={<Analytics />}/>
              <Route path="settings" element={<Settings />}/>
            </Route>
          </Routes>
        </BrowserRouter>
      </ThemeProvider>
    </QueryClientProvider>);
}
export default App;
//# sourceMappingURL=App.jsx.map