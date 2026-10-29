'use client';

import { useEffect, useState } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import {
  Settings,
  Shield,
  Wallet,
  Coins,
  Rocket,
  Globe,
  Activity,
  Users,
  DollarSign,
  Plus,
  Trash2,
  ToggleLeft,
  ToggleRight,
  Search,
  X,
  Check,
  BarChart3,
  Lock,
  Unlock,
} from 'lucide-react';
import { getCurrentUser, login, logout, restoreSession, subscribe } from '@/lib/auth';
import { endpoints } from '@/lib/endpoints';
import type { AdminChain, AdminSettings, AdminToken, FeeConfig, LaunchpadProject } from '@/lib/types';
import {
  useAdminChains,
  useAdminDashboard,
  useAdminLogs,
  useAdminSettings,
  useAdminTokens,
  useFeeConfig,
  useLaunchpadProjects,
} from '@/hooks/useApi';
import { AdminListingsPanel } from '@/components/AdminListingsPanel';

type Tab = 'dashboard' | 'blockchains' | 'tokens' | 'listings' | 'launchpad' | 'fees' | 'settings' | 'activity';

export default function AdminPage() {
  const [activeTab, setActiveTab] = useState<Tab>('dashboard');
  const [adminEmail, setAdminEmail] = useState('');
  const [adminPassword, setAdminPassword] = useState('');
  const [loginError, setLoginError] = useState<string | null>(null);
  const [isLoggingIn, setIsLoggingIn] = useState(false);
  const [isAdminAuthenticated, setAdminAuthenticated] = useState(false);
  const [operatorId, setOperatorId] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [isSaving, setIsSaving] = useState(false);

  const { data: dashboard, error: dashboardQueryError } = useAdminDashboard();
  const dashboardError = dashboardQueryError instanceof Error ? dashboardQueryError.message : null;
  const { data: chains, error: chainsQueryError } = useAdminChains();
  const { data: tokens, error: tokensQueryError } = useAdminTokens();
  const { data: settings, error: settingsQueryError } = useAdminSettings();
  const { data: feeConfig, error: feeQueryError } = useFeeConfig();
  const { data: activityLogs, error: logsQueryError } = useAdminLogs();
  const { data: launchpadProjects, error: projectsQueryError, refetch: refetchProjects } = useLaunchpadProjects();

  const [showAddBlockchain, setShowAddBlockchain] = useState(false);
  const [showAddToken, setShowAddToken] = useState(false);
  const [showAddProject, setShowAddProject] = useState(false);
  const [searchQuery, setSearchQuery] = useState('');
  const [filterChain, setFilterChain] = useState<number | null>(null);

  // Form states
  const [newBlockchain, setNewBlockchain] = useState({
    chainId: 0,
    name: '',
    symbol: '',
    type: 'evm',
    rpcUrl: '',
    explorerUrl: '',
    logoUrl: '',
    isActive: true,
    isTestnet: false,
  });

  const [newToken, setNewToken] = useState({
    address: '',
    chainId: 1,
    name: '',
    symbol: '',
    decimals: 18,
    logoUrl: '',
    isActive: true,
    isVerified: false,
    totalSupply: '',
  });

  const [newProject, setNewProject] = useState({
    name: '',
    description: '',
    tokenAddress: '',
    tokenSymbol: '',
    tokenName: '',
    totalSupply: '',
    pricePerToken: '',
    paymentToken: 'USDC',
    minPurchase: '',
    maxPurchase: '',
    softCap: '',
    hardCap: '',
    startTime: '',
    endTime: '',
    websiteUrl: '',
    whitepaperUrl: '',
    logoUrl: '',
  });

  // The session is in-memory, so a reload must be re-validated against the API
  // before the admin console unlocks.
  useEffect(() => {
    let cancelled = false;
    const applyUser = () => {
      const user = getCurrentUser();
      const allowed = user?.role === 'admin' || user?.role === 'superadmin';
      setAdminAuthenticated(allowed);
      setOperatorId(allowed ? user!.id : null);
    };
    const unsubscribe = subscribe(applyUser);
    void restoreSession().then(() => {
      if (!cancelled) applyUser();
    });
    return () => {
      cancelled = true;
      unsubscribe();
    };
  }, []);

  const chainRows = chains ?? [];
  const tokenRows = tokens ?? [];
  const projectRows = launchpadProjects ?? [];
  const logRows = activityLogs ?? [];
  const settingsRow = settings ?? null;
  const feeRow = feeConfig ?? null;

  /**
   * Signs the operator in with email and password.
   *
   * Authentication is performed by the API; the resulting session token is
   * held in memory by the auth layer. A mnemonic is never requested or sent.
   */
  const handleLogin = async () => {
    setLoginError(null);
    setIsLoggingIn(true);
    try {
      const user = await login(adminEmail, adminPassword);
      if (user.role !== 'admin' && user.role !== 'superadmin') {
        await logout();
        setLoginError('This account does not have admin access');
        return;
      }
      setAdminAuthenticated(true);
      setOperatorId(user.id);
      setAdminPassword('');
    } catch (error) {
      setLoginError(error instanceof Error ? error.message : 'Sign-in failed');
    } finally {
      setIsLoggingIn(false);
    }
  };

  const handleLogout = async () => {
    await logout();
    setAdminAuthenticated(false);
    setOperatorId(null);
  };

  const handleAddBlockchain = async () => {
    if (!newBlockchain.name || !newBlockchain.symbol) return;
    setActionError(null);
    setIsSaving(true);
    try {
      await endpoints.createAdminChain({
        chain_id: newBlockchain.chainId,
        name: newBlockchain.name,
        symbol: newBlockchain.symbol,
        type: newBlockchain.type,
        rpc_url: newBlockchain.rpcUrl,
        explorer_url: newBlockchain.explorerUrl,
        icon: newBlockchain.logoUrl,
        is_active: newBlockchain.isActive,
        is_testnet: newBlockchain.isTestnet,
      });
      setShowAddBlockchain(false);
      setNewBlockchain({
        chainId: 0, name: '', symbol: '', type: 'evm', rpcUrl: '', explorerUrl: '', logoUrl: '', isActive: true, isTestnet: false,
      });
    } catch (error) {
      setActionError(error instanceof Error ? error.message : 'Failed to create chain');
    } finally {
      setIsSaving(false);
    }
  };

  const handleToggleBlockchain = async (chain: AdminChain) => {
    setActionError(null);
    try {
      await endpoints.updateAdminChain(chain.id, { is_active: !chain.is_active });
    } catch (error) {
      setActionError(error instanceof Error ? error.message : 'Failed to update chain');
    }
  };

  const handleDeleteBlockchain = async (chain: AdminChain) => {
    setActionError(null);
    try {
      await endpoints.deleteAdminChain(chain.id);
    } catch (error) {
      setActionError(error instanceof Error ? error.message : 'Failed to delete chain');
    }
  };

  const handleAddToken = async () => {
    if (!newToken.name || !newToken.symbol) return;
    setActionError(null);
    setIsSaving(true);
    try {
      await endpoints.createAdminToken({
        address: newToken.address,
        chain_id: newToken.chainId,
        name: newToken.name,
        symbol: newToken.symbol,
        decimals: newToken.decimals,
        logo_uri: newToken.logoUrl,
        total_supply: newToken.totalSupply || '0',
        is_active: newToken.isActive,
        is_verified: newToken.isVerified,
      });
      setShowAddToken(false);
      setNewToken({ address: '', chainId: 1, name: '', symbol: '', decimals: 18, logoUrl: '', isActive: true, isVerified: false, totalSupply: '' });
    } catch (error) {
      setActionError(error instanceof Error ? error.message : 'Failed to create token');
    } finally {
      setIsSaving(false);
    }
  };

  const handleToggleToken = async (token: AdminToken) => {
    setActionError(null);
    try {
      await endpoints.updateAdminToken(token.id, { is_active: !token.is_active });
    } catch (error) {
      setActionError(error instanceof Error ? error.message : 'Failed to update token');
    }
  };

  const handleVerifyToken = async (token: AdminToken) => {
    setActionError(null);
    try {
      await endpoints.updateAdminToken(token.id, { is_verified: true });
    } catch (error) {
      setActionError(error instanceof Error ? error.message : 'Failed to verify token');
    }
  };

  const handleDeleteToken = async (token: AdminToken) => {
    setActionError(null);
    try {
      await endpoints.deleteAdminToken(token.id);
    } catch (error) {
      setActionError(error instanceof Error ? error.message : 'Failed to delete token');
    }
  };

  const handleAddProject = async () => {
    if (!newProject.name || !newProject.tokenSymbol) return;
    setActionError(null);
    setIsSaving(true);
    try {
      await endpoints.createLaunchpadProject({
        creator: operatorId ?? '',
        name: newProject.name,
        description: newProject.description,
        token_address: newProject.tokenAddress,
        token_symbol: newProject.tokenSymbol,
        token_name: newProject.tokenName || newProject.name,
        total_supply: newProject.totalSupply,
        token_price: newProject.pricePerToken,
        payment_token: newProject.paymentToken,
        min_purchase: newProject.minPurchase,
        max_purchase: newProject.maxPurchase,
        soft_cap: newProject.softCap,
        hard_cap: newProject.hardCap,
        start_time: new Date(newProject.startTime).toISOString(),
        end_time: newProject.endTime ? new Date(newProject.endTime).toISOString() : new Date(newProject.startTime).toISOString(),
        website_url: newProject.websiteUrl,
        whitepaper_url: newProject.whitepaperUrl,
        logo_url: newProject.logoUrl,
      });
      setShowAddProject(false);
      await refetchProjects();
    } catch (error) {
      setActionError(error instanceof Error ? error.message : 'Failed to create project');
    } finally {
      setIsSaving(false);
    }
  };

  const handleUpdateProjectStatus = async (project: LaunchpadProject, status: string) => {
    setActionError(null);
    try {
      await endpoints.updateLaunchpadStatus(project.id, status);
      await refetchProjects();
    } catch (error) {
      setActionError(error instanceof Error ? error.message : 'Failed to update project');
    }
  };

  const handleUpdateFee = async (key: keyof FeeConfig, value: number) => {
    setActionError(null);
    setIsSaving(true);
    try {
      await endpoints.updateFeeConfig({ ...(feeRow ?? {}), [key]: value });
    } catch (error) {
      setActionError(error instanceof Error ? error.message : 'Failed to update fee');
    } finally {
      setIsSaving(false);
    }
  };

  const handleUpdateSettings = async (patch: Partial<AdminSettings>) => {
    setActionError(null);
    setIsSaving(true);
    try {
      await endpoints.updateAdminSettings(patch);
    } catch (error) {
      setActionError(error instanceof Error ? error.message : 'Failed to update settings');
    } finally {
      setIsSaving(false);
    }
  };

  const handleToggleMaintenance = async () => {
    setActionError(null);
    setIsSaving(true);
    try {
      await endpoints.setMaintenance(!(settingsRow?.maintenance_mode ?? false));
    } catch (error) {
      setActionError(error instanceof Error ? error.message : 'Failed to change maintenance mode');
    } finally {
      setIsSaving(false);
    }
  };

  const filteredBlockchains = chainRows.filter(b =>
    b.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
    b.symbol.toLowerCase().includes(searchQuery.toLowerCase())
  );

  const filteredTokens = tokenRows.filter(t =>
    t.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
    t.symbol.toLowerCase().includes(searchQuery.toLowerCase()) ||
    (filterChain ? t.chain_id === filterChain : true)
  );

  const maintenanceOn = settingsRow?.maintenance_mode ?? false;

  if (!isAdminAuthenticated) {
    return (
      <div className="min-h-screen bg-gradient-to-b from-tiger-dark via-[#0f0f1a] to-[#0a0a12] flex items-center justify-center p-4">
        <div className="bg-white/5 border border-white/10 rounded-2xl p-8 w-full max-w-md">
          <div className="text-center mb-8">
            <div className="w-20 h-20 bg-gradient-to-br from-tiger-orange to-yellow-500 rounded-full flex items-center justify-center mx-auto mb-4">
              <Shield className="w-10 h-10 text-white" />
            </div>
            <h1 className="text-3xl font-bold text-white mb-2">Admin Login</h1>
            <p className="text-gray-400">Sign in with your operator account</p>
          </div>
          <form
            onSubmit={(event) => {
              event.preventDefault();
              void handleLogin();
            }}
            className="space-y-4"
          >
            <input
              type="email"
              value={adminEmail}
              onChange={(e) => setAdminEmail(e.target.value)}
              placeholder="operator@example.com"
              autoComplete="username"
              required
              className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white placeholder-gray-500 focus:outline-none focus:border-tiger-orange"
            />
            <input
              type="password"
              value={adminPassword}
              onChange={(e) => setAdminPassword(e.target.value)}
              placeholder="Password"
              autoComplete="current-password"
              required
              className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white placeholder-gray-500 focus:outline-none focus:border-tiger-orange"
            />
            {loginError && (
              <p role="alert" className="text-red-400 text-sm">{loginError}</p>
            )}
            <button
              type="submit"
              disabled={isLoggingIn || !adminEmail || !adminPassword}
              className="w-full bg-tiger-orange hover:bg-tiger-accent disabled:bg-gray-600 disabled:cursor-not-allowed text-white py-4 rounded-xl font-bold transition-colors"
            >
              {isLoggingIn ? 'Signing in...' : 'Login'}
            </button>
          </form>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-gradient-to-b from-tiger-dark via-[#0f0f1a] to-[#0a0a12]">
      {/* Header */}
      <header className="border-b border-white/5 bg-tiger-dark/80 backdrop-blur-xl sticky top-0 z-50">
        <div className="max-w-7xl mx-auto px-4 py-4">
          <div className="flex items-center justify-between">
            <div className="flex items-center space-x-4">
              <div className="flex items-center space-x-2">
                <div className="w-10 h-10 bg-gradient-to-br from-tiger-orange to-yellow-500 rounded-xl flex items-center justify-center">
                  <span className="text-white font-bold text-xl">T</span>
                </div>
                <span className="text-white font-bold text-xl">TigerSwap Admin</span>
              </div>
              <span className="px-3 py-1 bg-tiger-orange/20 text-tiger-orange rounded-full text-sm font-medium">
                Super Admin
              </span>
            </div>
            <div className="flex items-center space-x-4">
              <button
                onClick={handleToggleMaintenance}
                disabled={isSaving}
                className={`px-4 py-2 rounded-lg font-medium transition-colors disabled:opacity-50 ${
                  maintenanceOn
                    ? 'bg-red-500/20 text-red-400'
                    : 'bg-green-500/20 text-green-400'
                }`}
              >
                {maintenanceOn ? (
                  <span className="flex items-center space-x-2"><Lock className="w-4 h-4" /><span>Maintenance ON</span></span>
                ) : (
                  <span className="flex items-center space-x-2"><Unlock className="w-4 h-4" /><span>Live</span></span>
                )}
              </button>
              <button
                onClick={handleLogout}
                className="px-4 py-2 bg-white/10 text-white rounded-lg hover:bg-white/20 transition-colors"
              >
                Logout
              </button>
            </div>
          </div>
        </div>
      </header>

      {/* Navigation */}
      <nav className="border-b border-white/5 bg-tiger-dark/50">
        <div className="max-w-7xl mx-auto px-4">
          <div className="flex items-center space-x-1 overflow-x-auto">
            {[
              { id: 'dashboard', label: 'Dashboard', icon: BarChart3 },
              { id: 'blockchains', label: 'Blockchains', icon: Globe },
              { id: 'tokens', label: 'Tokens', icon: Coins },
              { id: 'listings', label: 'Listings', icon: BarChart3 },
              { id: 'launchpad', label: 'Launchpad', icon: Rocket },
              { id: 'fees', label: 'Fees', icon: DollarSign },
              { id: 'settings', label: 'Settings', icon: Settings },
              { id: 'activity', label: 'Activity', icon: Activity },
            ].map(tab => (
              <button
                key={tab.id}
                onClick={() => setActiveTab(tab.id as Tab)}
                className={`flex items-center space-x-2 px-4 py-3 text-sm font-medium border-b-2 transition-colors whitespace-nowrap ${
                  activeTab === tab.id 
                    ? 'border-tiger-orange text-tiger-orange' 
                    : 'border-transparent text-gray-400 hover:text-white'
                }`}
              >
                <tab.icon className="w-4 h-4" />
                <span>{tab.label}</span>
              </button>
            ))}
          </div>
        </div>
      </nav>

      {/* Content */}
      <main className="max-w-7xl mx-auto px-4 py-8">
        {actionError && (
          <p role="alert" className="text-red-400 text-sm mb-6">{actionError}</p>
        )}
        <AnimatePresence mode="wait">
          {activeTab === 'dashboard' && (
            <motion.div key="dashboard" initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: -10 }}>
              <h2 className="text-2xl font-bold text-white mb-6">Dashboard</h2>
              <div className="grid md:grid-cols-4 gap-6 mb-8">
                <div className="bg-white/5 rounded-2xl p-6">
                  <div className="flex items-center justify-between mb-4">
                    <Users className="w-8 h-8 text-tiger-orange" />
                  </div>
                  <p className="text-3xl font-bold text-white">
                    {dashboard ? dashboard.totalUsers.toLocaleString() : '—'}
                  </p>
                  <p className="text-gray-400 text-sm">Total Users</p>
                </div>
                <div className="bg-white/5 rounded-2xl p-6">
                  <div className="flex items-center justify-between mb-4">
                    <Activity className="w-8 h-8 text-tiger-orange" />
                  </div>
                  <p className="text-3xl font-bold text-white">
                    {dashboard ? dashboard.totalTransactions.toLocaleString() : '—'}
                  </p>
                  <p className="text-gray-400 text-sm">Total Transactions</p>
                </div>
                <div className="bg-white/5 rounded-2xl p-6">
                  <div className="flex items-center justify-between mb-4">
                    <DollarSign className="w-8 h-8 text-tiger-orange" />
                  </div>
                  <p className="text-3xl font-bold text-white">
                    {dashboard ? `$${dashboard.totalVolume.toLocaleString()}` : '—'}
                  </p>
                  <p className="text-gray-400 text-sm">Total Volume</p>
                </div>
                <div className="bg-white/5 rounded-2xl p-6">
                  <div className="flex items-center justify-between mb-4">
                    <Wallet className="w-8 h-8 text-tiger-orange" />
                  </div>
                  <p className="text-3xl font-bold text-white">
                    {dashboard ? `$${dashboard.totalRevenue.toLocaleString()}` : '—'}
                  </p>
                  <p className="text-gray-400 text-sm">Total Revenue</p>
                </div>
              </div>
              {dashboardError && (
                <p role="alert" className="text-red-400 text-sm mb-6">
                  Dashboard data is unavailable: {dashboardError}
                </p>
              )}

              <div className="grid md:grid-cols-3 gap-6">
                <div className="bg-white/5 rounded-2xl p-6">
                  <h3 className="text-white font-semibold mb-4">System Status</h3>
                  <div className="space-y-3">
                    <div className="flex justify-between items-center">
                      <span className="text-gray-400">Trading</span>
                      <span className={`px-2 py-1 rounded text-xs ${settingsRow?.trading_enabled ? 'bg-green-500/20 text-green-400' : 'bg-red-500/20 text-red-400'}`}>
                        {settingsRow?.trading_enabled ? 'Enabled' : 'Disabled'}
                      </span>
                    </div>
                    <div className="flex justify-between items-center">
                      <span className="text-gray-400">Withdrawals</span>
                      <span className={`px-2 py-1 rounded text-xs ${settingsRow?.withdrawal_enabled ? 'bg-green-500/20 text-green-400' : 'bg-red-500/20 text-red-400'}`}>
                        {settingsRow?.withdrawal_enabled ? 'Enabled' : 'Disabled'}
                      </span>
                    </div>
                    <div className="flex justify-between items-center">
                      <span className="text-gray-400">Deposits</span>
                      <span className={`px-2 py-1 rounded text-xs ${settingsRow?.deposit_enabled ? 'bg-green-500/20 text-green-400' : 'bg-red-500/20 text-red-400'}`}>
                        {settingsRow?.deposit_enabled ? 'Enabled' : 'Disabled'}
                      </span>
                    </div>
                    <div className="flex justify-between items-center">
                      <span className="text-gray-400">Registration</span>
                      <span className={`px-2 py-1 rounded text-xs ${settingsRow?.new_user_registration_enabled ? 'bg-green-500/20 text-green-400' : 'bg-red-500/20 text-red-400'}`}>
                        {settingsRow?.new_user_registration_enabled ? 'Enabled' : 'Disabled'}
                      </span>
                    </div>
                  </div>
                </div>

                <div className="bg-white/5 rounded-2xl p-6">
                  <h3 className="text-white font-semibold mb-4">Quick Stats</h3>
                  <div className="space-y-3">
                    <div className="flex justify-between items-center">
                      <span className="text-gray-400">Blockchains</span>
                      <span className="text-white font-medium">{chainRows.length}</span>
                    </div>
                    <div className="flex justify-between items-center">
                      <span className="text-gray-400">Active Chains</span>
                      <span className="text-white font-medium">{chainRows.filter(b => b.is_active).length}</span>
                    </div>
                    <div className="flex justify-between items-center">
                      <span className="text-gray-400">Tokens</span>
                      <span className="text-white font-medium">{tokenRows.length}</span>
                    </div>
                    <div className="flex justify-between items-center">
                      <span className="text-gray-400">Verified Tokens</span>
                      <span className="text-white font-medium">{tokenRows.filter(t => t.is_verified).length}</span>
                    </div>
                  </div>
                </div>

                <div className="bg-white/5 rounded-2xl p-6">
                  <h3 className="text-white font-semibold mb-4">Recent Activity</h3>
                  <div className="space-y-3">
                    {logRows.slice(0, 5).map(log => (
                      <div key={log.id} className="text-sm">
                        <p className="text-white">{log.action}</p>
                        <p className="text-gray-500 text-xs">{new Date(log.created_at).toLocaleString()}</p>
                      </div>
                    ))}
                  </div>
                </div>
              </div>
            </motion.div>
          )}

          {activeTab === 'blockchains' && (
            <motion.div key="blockchains" initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: -10 }}>
              <div className="flex items-center justify-between mb-6">
                <h2 className="text-2xl font-bold text-white">Blockchains ({chainRows.length})</h2>
                <div className="flex items-center space-x-3">
                  <div className="relative">
                    <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400" />
                    <input
                      type="text"
                      value={searchQuery}
                      onChange={(e) => setSearchQuery(e.target.value)}
                      placeholder="Search..."
                      className="pl-10 pr-4 py-2 bg-white/10 border border-white/20 rounded-lg text-white placeholder-gray-500 focus:outline-none focus:border-tiger-orange"
                    />
                  </div>
                  <button
                    onClick={() => setShowAddBlockchain(true)}
                    className="flex items-center space-x-2 px-4 py-2 bg-tiger-orange text-white rounded-lg hover:bg-tiger-accent transition-colors"
                  >
                    <Plus className="w-4 h-4" />
                    <span>Add Blockchain</span>
                  </button>
                </div>
              </div>

              <div className="bg-white/5 rounded-2xl overflow-hidden">
                <table className="w-full">
                  <thead className="bg-white/5">
                    <tr>
                      <th className="text-left px-6 py-4 text-gray-400 font-medium">Chain</th>
                      <th className="text-left px-6 py-4 text-gray-400 font-medium">Chain ID</th>
                      <th className="text-left px-6 py-4 text-gray-400 font-medium">Type</th>
                      <th className="text-left px-6 py-4 text-gray-400 font-medium">RPC</th>
                      <th className="text-left px-6 py-4 text-gray-400 font-medium">Status</th>
                      <th className="text-left px-6 py-4 text-gray-400 font-medium">Actions</th>
                    </tr>
                  </thead>
                  <tbody>
                    {filteredBlockchains.map(chain => (
                      <tr key={chain.id} className="border-t border-white/5">
                        <td className="px-6 py-4">
                          <div className="flex items-center space-x-3">
                            <div className="w-8 h-8 rounded-full bg-tiger-orange/20 flex items-center justify-center text-tiger-orange text-xs font-bold">
                              {chain.symbol.slice(0, 3).toUpperCase()}
                            </div>
                            <div>
                              <p className="text-white font-medium">{chain.name}</p>
                              <p className="text-gray-400 text-sm">{chain.symbol}</p>
                            </div>
                          </div>
                        </td>
                        <td className="px-6 py-4 text-white font-mono">{chain.chain_id}</td>
                        <td className="px-6 py-4">
                          <span className="px-2 py-1 bg-white/10 rounded text-xs text-white">{(chain.type || 'evm').toUpperCase()}</span>
                        </td>
                        <td className="px-6 py-4 text-gray-400 text-sm truncate max-w-xs">{chain.rpc_url || '—'}</td>
                        <td className="px-6 py-4">
                          <button
                            onClick={() => handleToggleBlockchain(chain)}
                            className={chain.is_active ? 'text-green-400' : 'text-red-400'}
                          >
                            {chain.is_active ? <ToggleRight className="w-6 h-6" /> : <ToggleLeft className="w-6 h-6" />}
                          </button>
                        </td>
                        <td className="px-6 py-4">
                          <div className="flex items-center space-x-2">
                            <button onClick={() => handleDeleteBlockchain(chain)} className="p-2 text-gray-400 hover:text-red-400"><Trash2 className="w-4 h-4" /></button>
                          </div>
                        </td>
                      </tr>
                    ))}
                    {filteredBlockchains.length === 0 && (
                      <tr>
                        <td colSpan={6} className="px-6 py-12 text-center text-gray-400">
                          {chainsQueryError ? `Unable to load chains: ${chainsQueryError.message}` : 'No chains configured'}
                        </td>
                      </tr>
                    )}
                  </tbody>
                </table>
              </div>
            </motion.div>
          )}

          {activeTab === 'tokens' && (
            <motion.div key="tokens" initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: -10 }}>
              <div className="flex items-center justify-between mb-6">
                <h2 className="text-2xl font-bold text-white">Tokens ({tokenRows.length})</h2>
                <div className="flex items-center space-x-3">
                  <div className="relative">
                    <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400" />
                    <input
                      type="text"
                      value={searchQuery}
                      onChange={(e) => setSearchQuery(e.target.value)}
                      placeholder="Search..."
                      className="pl-10 pr-4 py-2 bg-white/10 border border-white/20 rounded-lg text-white placeholder-gray-500 focus:outline-none focus:border-tiger-orange"
                    />
                  </div>
                  <select
                    value={filterChain || ''}
                    onChange={(e) => setFilterChain(e.target.value ? parseInt(e.target.value) : null)}
                    className="px-4 py-2 bg-white/10 border border-white/20 rounded-lg text-white focus:outline-none focus:border-tiger-orange"
                  >
                    <option value="">All Chains</option>
                    {chainRows.map(b => (
                      <option key={b.id} value={b.chain_id}>{b.name}</option>
                    ))}
                  </select>
                  <button
                    onClick={() => setShowAddToken(true)}
                    className="flex items-center space-x-2 px-4 py-2 bg-tiger-orange text-white rounded-lg hover:bg-tiger-accent transition-colors"
                  >
                    <Plus className="w-4 h-4" />
                    <span>Add Token</span>
                  </button>
                </div>
              </div>

              <div className="bg-white/5 rounded-2xl overflow-hidden">
                <table className="w-full">
                  <thead className="bg-white/5">
                    <tr>
                      <th className="text-left px-6 py-4 text-gray-400 font-medium">Token</th>
                      <th className="text-left px-6 py-4 text-gray-400 font-medium">Chain</th>
                      <th className="text-left px-6 py-4 text-gray-400 font-medium">Address</th>
                      <th className="text-left px-6 py-4 text-gray-400 font-medium">Total Supply</th>
                      <th className="text-left px-6 py-4 text-gray-400 font-medium">Verified</th>
                      <th className="text-left px-6 py-4 text-gray-400 font-medium">Status</th>
                      <th className="text-left px-6 py-4 text-gray-400 font-medium">Actions</th>
                    </tr>
                  </thead>
                  <tbody>
                    {filteredTokens.map(token => (
                      <tr key={token.id} className="border-t border-white/5">
                        <td className="px-6 py-4">
                          <div className="flex items-center space-x-3">
                            <div className="w-8 h-8 rounded-full bg-tiger-orange/20 flex items-center justify-center text-tiger-orange text-xs font-bold">
                              {token.symbol.slice(0, 3).toUpperCase()}
                            </div>
                            <div>
                              <p className="text-white font-medium">{token.name}</p>
                              <p className="text-gray-400 text-sm">{token.symbol}</p>
                            </div>
                          </div>
                        </td>
                        <td className="px-6 py-4 text-white">{chainRows.find(b => b.chain_id === token.chain_id)?.name || token.chain_id}</td>
                        <td className="px-6 py-4 text-gray-400 text-sm font-mono truncate max-w-xs">{token.address.slice(0, 10)}…</td>
                        <td className="px-6 py-4 text-white font-mono text-sm">{token.total_supply}</td>
                        <td className="px-6 py-4">
                          {token.is_verified ? (
                            <Check className="w-5 h-5 text-green-400" />
                          ) : (
                            <button onClick={() => handleVerifyToken(token)} className="text-yellow-400 text-sm hover:underline">Verify</button>
                          )}
                        </td>
                        <td className="px-6 py-4">
                          <button onClick={() => handleToggleToken(token)} className={token.is_active ? 'text-green-400' : 'text-red-400'}>
                            {token.is_active ? <ToggleRight className="w-6 h-6" /> : <ToggleLeft className="w-6 h-6" />}
                          </button>
                        </td>
                        <td className="px-6 py-4">
                          <div className="flex items-center space-x-2">
                            <button onClick={() => handleDeleteToken(token)} className="p-2 text-gray-400 hover:text-red-400"><Trash2 className="w-4 h-4" /></button>
                          </div>
                        </td>
                      </tr>
                    ))}
                    {filteredTokens.length === 0 && (
                      <tr>
                        <td colSpan={7} className="px-6 py-12 text-center text-gray-400">
                          {tokensQueryError ? `Unable to load tokens: ${tokensQueryError.message}` : 'No tokens configured'}
                        </td>
                      </tr>
                    )}
                  </tbody>
                </table>
              </div>
            </motion.div>
          )}

          {activeTab === 'listings' && (
            <motion.div key="listings" initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: -10 }}>
              <AdminListingsPanel />
            </motion.div>
          )}

          {activeTab === 'launchpad' && (
            <motion.div key="launchpad" initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: -10 }}>
              <div className="flex items-center justify-between mb-6">
                <h2 className="text-2xl font-bold text-white">Launchpad Projects ({projectRows.length})</h2>
                <button
                  onClick={() => setShowAddProject(true)}
                  className="flex items-center space-x-2 px-4 py-2 bg-tiger-orange text-white rounded-lg hover:bg-tiger-accent transition-colors"
                >
                  <Plus className="w-4 h-4" />
                  <span>Create Project</span>
                </button>
              </div>

              <div className="grid md:grid-cols-2 lg:grid-cols-3 gap-6">
                {projectRows.length === 0 ? (
                  <div className="md:col-span-2 lg:col-span-3 bg-white/5 rounded-2xl p-12 text-center">
                    <Rocket className="w-16 h-16 text-gray-400 mx-auto mb-4" />
                    <p className="text-gray-400 text-lg">
                      {projectsQueryError ? `Unable to load projects: ${projectsQueryError.message}` : 'No launchpad projects yet'}
                    </p>
                    <button onClick={() => setShowAddProject(true)} className="mt-4 text-tiger-orange hover:underline">Create your first project</button>
                  </div>
                ) : (
                  projectRows.map(project => (
                    <div key={project.id} className="bg-white/5 rounded-2xl p-6">
                      <div className="flex items-start justify-between mb-4">
                        <div className="flex items-center space-x-3">
                          <div className="w-12 h-12 rounded-xl bg-tiger-orange/20 flex items-center justify-center text-tiger-orange font-bold">
                            {project.token_symbol.slice(0, 3).toUpperCase()}
                          </div>
                          <div>
                            <h3 className="text-white font-semibold">{project.name}</h3>
                            <p className="text-tiger-orange">{project.token_symbol}</p>
                          </div>
                        </div>
                        <span className={`px-3 py-1 rounded-full text-xs font-medium ${
                          project.status === 'active' ? 'bg-green-500/20 text-green-400' :
                          project.status === 'upcoming' ? 'bg-yellow-500/20 text-yellow-400' :
                          project.status === 'completed' ? 'bg-blue-500/20 text-blue-400' :
                          'bg-red-500/20 text-red-400'
                        }`}>
                          {project.status.toUpperCase()}
                        </span>
                      </div>
                      <p className="text-gray-400 text-sm mb-4 line-clamp-2">{project.description}</p>
                      <div className="grid grid-cols-2 gap-4 text-sm mb-4">
                        <div><p className="text-gray-400">Raised</p><p className="text-white">${project.raised_amount} / ${project.hard_cap}</p></div>
                        <div><p className="text-gray-400">Price</p><p className="text-white">${project.price_per_token}</p></div>
                      </div>
                      <div className="flex items-center space-x-2">
                        {project.status === 'upcoming' && (
                          <button onClick={() => handleUpdateProjectStatus(project, 'active')} className="flex-1 bg-green-500/20 text-green-400 py-2 rounded-lg text-sm hover:bg-green-500/30">Activate</button>
                        )}
                        {project.status === 'active' && (
                          <button onClick={() => handleUpdateProjectStatus(project, 'completed')} className="flex-1 bg-blue-500/20 text-blue-400 py-2 rounded-lg text-sm hover:bg-blue-500/30">Complete</button>
                        )}
                        {project.status !== 'cancelled' && project.status !== 'completed' && (
                          <button onClick={() => handleUpdateProjectStatus(project, 'cancelled')} className="flex-1 bg-red-500/20 text-red-400 py-2 rounded-lg text-sm hover:bg-red-500/30">Cancel</button>
                        )}
                      </div>
                    </div>
                  ))
                )}
              </div>
            </motion.div>
          )}

          {activeTab === 'fees' && (
            <motion.div key="fees" initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: -10 }}>
              <h2 className="text-2xl font-bold text-white mb-6">Fee Configuration</h2>
              {feeQueryError && (
                <p role="alert" className="text-red-400 text-sm mb-6">
                  Fee configuration is unavailable: {feeQueryError.message}
                </p>
              )}
              {!feeRow ? (
                <div className="bg-white/5 rounded-2xl p-12 text-center text-gray-400">Loading fee configuration…</div>
              ) : (
                <div className="grid md:grid-cols-2 gap-6">
                  <div className="bg-white/5 rounded-2xl p-6">
                    <h3 className="text-white font-semibold mb-6">Transaction Fees</h3>
                    <div className="space-y-4">
                      {([
                        ['withdraw_fee_percent', 'Withdraw Fee (%)'],
                        ['swap_fee_percent', 'Swap Fee (%)'],
                        ['deposit_fee_percent', 'Deposit Fee (%)'],
                        ['bridge_fee_percent', 'Bridge Fee (%)'],
                        ['wallet_tx_fee_percent', 'Wallet Tx Fee (%)'],
                      ] as [keyof FeeConfig, string][]).map(([key, label]) => (
                        <div key={key}>
                          <label className="text-gray-400 text-sm mb-2 block">{label}</label>
                          <input
                            type="number"
                            step="0.01"
                            value={feeRow[key]}
                            onChange={(e) => handleUpdateFee(key, parseFloat(e.target.value) || 0)}
                            className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white focus:outline-none focus:border-tiger-orange"
                          />
                        </div>
                      ))}
                    </div>
                  </div>
                  <div className="bg-white/5 rounded-2xl p-6">
                    <h3 className="text-white font-semibold mb-6">Other Fees</h3>
                    <div className="space-y-4">
                      {([
                        ['launchpad_fee_percent', 'Launchpad Fee (%)'],
                        ['referral_fee_percent', 'Referral Fee (%)'],
                      ] as [keyof FeeConfig, string][]).map(([key, label]) => (
                        <div key={key}>
                          <label className="text-gray-400 text-sm mb-2 block">{label}</label>
                          <input
                            type="number"
                            step="0.01"
                            value={feeRow[key]}
                            onChange={(e) => handleUpdateFee(key, parseFloat(e.target.value) || 0)}
                            className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white focus:outline-none focus:border-tiger-orange"
                          />
                        </div>
                      ))}
                    </div>
                  </div>
                </div>
              )}
            </motion.div>
          )}

          {activeTab === 'settings' && (
            <motion.div key="settings" initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: -10 }}>
              <h2 className="text-2xl font-bold text-white mb-6">System Settings</h2>
              {settingsQueryError && (
                <p role="alert" className="text-red-400 text-sm mb-6">
                  Settings are unavailable: {settingsQueryError.message}
                </p>
              )}
              <div className="bg-white/5 rounded-2xl p-6">
                <div className="space-y-6">
                  {([
                    ['trading_enabled', 'Trading', 'Enable or disable trading functionality'],
                    ['withdrawal_enabled', 'Withdrawals', 'Enable or disable withdrawal functionality'],
                    ['deposit_enabled', 'Deposits', 'Enable or disable deposit functionality'],
                    ['new_user_registration_enabled', 'User Registration', 'Enable or disable new user registrations'],
                  ] as [keyof AdminSettings, string, string][]).map(([key, label, hint]) => {
                    const on = Boolean(settingsRow?.[key]);
                    return (
                      <div key={key} className="flex items-center justify-between">
                        <div>
                          <h3 className="text-white font-medium">{label}</h3>
                          <p className="text-gray-400 text-sm">{hint}</p>
                        </div>
                        <button
                          disabled={isSaving}
                          onClick={() => handleUpdateSettings({ [key]: !on } as Partial<AdminSettings>)}
                          className={on ? 'text-green-400' : 'text-red-400'}
                        >
                          {on ? <ToggleRight className="w-8 h-8" /> : <ToggleLeft className="w-8 h-8" />}
                        </button>
                      </div>
                    );
                  })}
                </div>
              </div>
            </motion.div>
          )}

          {activeTab === 'activity' && (
            <motion.div key="activity" initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: -10 }}>
              <h2 className="text-2xl font-bold text-white mb-6">Activity Log</h2>
              <div className="bg-white/5 rounded-2xl overflow-hidden">
                {logRows.length === 0 ? (
                  <div className="p-12 text-center text-gray-400">
                    {logsQueryError ? `Unable to load activity: ${logsQueryError.message}` : 'No activity yet'}
                  </div>
                ) : (
                  <table className="w-full">
                    <thead className="bg-white/5">
                      <tr>
                        <th className="text-left px-6 py-4 text-gray-400 font-medium">Action</th>
                        <th className="text-left px-6 py-4 text-gray-400 font-medium">Entity</th>
                        <th className="text-left px-6 py-4 text-gray-400 font-medium">Details</th>
                        <th className="text-left px-6 py-4 text-gray-400 font-medium">Time</th>
                      </tr>
                    </thead>
                    <tbody>
                      {logRows.map(log => (
                        <tr key={log.id} className="border-t border-white/5">
                          <td className="px-6 py-4 text-white">{log.action}</td>
                          <td className="px-6 py-4 text-gray-400">{log.entity_type}{log.entity_id ? ` · ${log.entity_id}` : ''}</td>
                          <td className="px-6 py-4 text-gray-400">{log.details}</td>
                          <td className="px-6 py-4 text-gray-400 text-sm">{new Date(log.created_at).toLocaleString()}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                )}
              </div>
            </motion.div>
          )}
        </AnimatePresence>
      </main>

      {/* Add Blockchain Modal */}
      <AnimatePresence>
        {showAddBlockchain && (
          <motion.div initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }} className="fixed inset-0 bg-black/80 backdrop-blur-sm flex items-center justify-center z-50 p-4" onClick={() => setShowAddBlockchain(false)}>
            <motion.div initial={{ scale: 0.95 }} animate={{ scale: 1 }} exit={{ scale: 0.95 }} className="bg-[#1a1a2e] border border-white/10 rounded-2xl p-6 w-full max-w-lg max-h-[90vh] overflow-y-auto" onClick={e => e.stopPropagation()}>
              <div className="flex items-center justify-between mb-6">
                <h3 className="text-xl font-bold text-white">Add Blockchain</h3>
                <button onClick={() => setShowAddBlockchain(false)} className="text-gray-400 hover:text-white"><X className="w-6 h-6" /></button>
              </div>
              <div className="space-y-4">
                <div className="grid grid-cols-2 gap-4">
                  <div><label className="text-gray-400 text-sm mb-2 block">Name</label><input type="text" value={newBlockchain.name} onChange={e => setNewBlockchain({...newBlockchain, name: e.target.value})} className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white" /></div>
                  <div><label className="text-gray-400 text-sm mb-2 block">Symbol</label><input type="text" value={newBlockchain.symbol} onChange={e => setNewBlockchain({...newBlockchain, symbol: e.target.value})} className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white" /></div>
                </div>
                <div className="grid grid-cols-2 gap-4">
                  <div><label className="text-gray-400 text-sm mb-2 block">Chain ID</label><input type="number" value={newBlockchain.chainId || ''} onChange={e => setNewBlockchain({...newBlockchain, chainId: parseInt(e.target.value)})} className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white" /></div>
                  <div><label className="text-gray-400 text-sm mb-2 block">Type</label><select value={newBlockchain.type} onChange={e => setNewBlockchain({...newBlockchain, type: e.target.value})} className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white"><option value="evm">EVM</option><option value="solana">Solana</option><option value="cosmos">Cosmos</option><option value="ton">TON</option><option value="aptos">Aptos</option></select></div>
                </div>
                <div><label className="text-gray-400 text-sm mb-2 block">RPC URL</label><input type="text" value={newBlockchain.rpcUrl} onChange={e => setNewBlockchain({...newBlockchain, rpcUrl: e.target.value})} className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white" /></div>
                <div><label className="text-gray-400 text-sm mb-2 block">Explorer URL</label><input type="text" value={newBlockchain.explorerUrl} onChange={e => setNewBlockchain({...newBlockchain, explorerUrl: e.target.value})} className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white" /></div>
                <div><label className="text-gray-400 text-sm mb-2 block">Logo URL</label><input type="text" value={newBlockchain.logoUrl} onChange={e => setNewBlockchain({...newBlockchain, logoUrl: e.target.value})} className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white" /></div>
                <div className="flex items-center justify-between">
                  <div>
                    <p className="text-white font-medium">Testnet</p>
                    <p className="text-gray-400 text-sm">Mark this chain as a test network</p>
                  </div>
                  <button
                    type="button"
                    onClick={() => setNewBlockchain({ ...newBlockchain, isTestnet: !newBlockchain.isTestnet })}
                    className={newBlockchain.isTestnet ? 'text-green-400' : 'text-gray-500'}
                  >
                    {newBlockchain.isTestnet ? <ToggleRight className="w-8 h-8" /> : <ToggleLeft className="w-8 h-8" />}
                  </button>
                </div>
                <button onClick={handleAddBlockchain} className="w-full bg-tiger-orange hover:bg-tiger-accent text-white py-4 rounded-xl font-bold">Add Blockchain</button>
              </div>
            </motion.div>
          </motion.div>
        )}
      </AnimatePresence>

      {/* Add Token Modal */}
      <AnimatePresence>
        {showAddToken && (
          <motion.div initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }} className="fixed inset-0 bg-black/80 backdrop-blur-sm flex items-center justify-center z-50 p-4" onClick={() => setShowAddToken(false)}>
            <motion.div initial={{ scale: 0.95 }} animate={{ scale: 1 }} exit={{ scale: 0.95 }} className="bg-[#1a1a2e] border border-white/10 rounded-2xl p-6 w-full max-w-lg" onClick={e => e.stopPropagation()}>
              <div className="flex items-center justify-between mb-6">
                <h3 className="text-xl font-bold text-white">Add Token</h3>
                <button onClick={() => setShowAddToken(false)} className="text-gray-400 hover:text-white"><X className="w-6 h-6" /></button>
              </div>
              <div className="space-y-4">
                <div className="grid grid-cols-2 gap-4">
                  <div><label className="text-gray-400 text-sm mb-2 block">Name</label><input type="text" value={newToken.name} onChange={e => setNewToken({...newToken, name: e.target.value})} className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white" /></div>
                  <div><label className="text-gray-400 text-sm mb-2 block">Symbol</label><input type="text" value={newToken.symbol} onChange={e => setNewToken({...newToken, symbol: e.target.value})} className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white" /></div>
                </div>
                <div className="grid grid-cols-2 gap-4">
                  <div><label className="text-gray-400 text-sm mb-2 block">Chain</label><select value={newToken.chainId} onChange={e => setNewToken({...newToken, chainId: parseInt(e.target.value)})} className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white">{chainRows.map(b => (<option key={b.id} value={b.chain_id}>{b.name}</option>))}</select></div>
                  <div><label className="text-gray-400 text-sm mb-2 block">Decimals</label><input type="number" value={newToken.decimals} onChange={e => setNewToken({...newToken, decimals: parseInt(e.target.value)})} className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white" /></div>
                </div>
                <div><label className="text-gray-400 text-sm mb-2 block">Contract Address</label><input type="text" value={newToken.address} onChange={e => setNewToken({...newToken, address: e.target.value})} className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white" /></div>
                <div><label className="text-gray-400 text-sm mb-2 block">Logo URL</label><input type="text" value={newToken.logoUrl} onChange={e => setNewToken({...newToken, logoUrl: e.target.value})} className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white" /></div>
                <div><label className="text-gray-400 text-sm mb-2 block">Total Supply</label><input type="text" value={newToken.totalSupply} onChange={e => setNewToken({...newToken, totalSupply: e.target.value})} placeholder="1000000000000000000000000" className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white placeholder-gray-500" /></div>
                <button onClick={handleAddToken} className="w-full bg-tiger-orange hover:bg-tiger-accent text-white py-4 rounded-xl font-bold">Add Token</button>
              </div>
            </motion.div>
          </motion.div>
        )}
      </AnimatePresence>

      {/* Add Project Modal */}
      <AnimatePresence>
        {showAddProject && (
          <motion.div initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }} className="fixed inset-0 bg-black/80 backdrop-blur-sm flex items-center justify-center z-50 p-4" onClick={() => setShowAddProject(false)}>
            <motion.div initial={{ scale: 0.95 }} animate={{ scale: 1 }} exit={{ scale: 0.95 }} className="bg-[#1a1a2e] border border-white/10 rounded-2xl p-6 w-full max-w-2xl max-h-[90vh] overflow-y-auto" onClick={e => e.stopPropagation()}>
              <div className="flex items-center justify-between mb-6">
                <h3 className="text-xl font-bold text-white">Create Launchpad Project</h3>
                <button onClick={() => setShowAddProject(false)} className="text-gray-400 hover:text-white"><X className="w-6 h-6" /></button>
              </div>
              <div className="space-y-4">
                <div className="grid grid-cols-2 gap-4">
                  <div><label className="text-gray-400 text-sm mb-2 block">Project Name</label><input type="text" value={newProject.name} onChange={e => setNewProject({...newProject, name: e.target.value})} className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white" /></div>
                  <div><label className="text-gray-400 text-sm mb-2 block">Token Symbol</label><input type="text" value={newProject.tokenSymbol} onChange={e => setNewProject({...newProject, tokenSymbol: e.target.value})} className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white" /></div>
                </div>
                <div><label className="text-gray-400 text-sm mb-2 block">Description</label><textarea value={newProject.description} onChange={e => setNewProject({...newProject, description: e.target.value})} rows={3} className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white" /></div>
                <div className="grid grid-cols-2 gap-4">
                  <div><label className="text-gray-400 text-sm mb-2 block">Token Address</label><input type="text" value={newProject.tokenAddress} onChange={e => setNewProject({...newProject, tokenAddress: e.target.value})} className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white" /></div>
                  <div><label className="text-gray-400 text-sm mb-2 block">Price Per Token</label><input type="text" value={newProject.pricePerToken} onChange={e => setNewProject({...newProject, pricePerToken: e.target.value})} className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white" /></div>
                </div>
                <div><label className="text-gray-400 text-sm mb-2 block">Total Supply</label><input type="text" value={newProject.totalSupply} onChange={e => setNewProject({...newProject, totalSupply: e.target.value})} className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white" /></div>
                <div className="grid grid-cols-2 gap-4">
                  <div><label className="text-gray-400 text-sm mb-2 block">Soft Cap (USD)</label><input type="text" value={newProject.softCap} onChange={e => setNewProject({...newProject, softCap: e.target.value})} className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white" /></div>
                  <div><label className="text-gray-400 text-sm mb-2 block">Hard Cap (USD)</label><input type="text" value={newProject.hardCap} onChange={e => setNewProject({...newProject, hardCap: e.target.value})} className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white" /></div>
                </div>
                <div className="grid grid-cols-2 gap-4">
                  <div><label className="text-gray-400 text-sm mb-2 block">Start Time</label><input type="datetime-local" value={newProject.startTime} onChange={e => setNewProject({...newProject, startTime: e.target.value})} className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white" /></div>
                  <div><label className="text-gray-400 text-sm mb-2 block">End Time</label><input type="datetime-local" value={newProject.endTime} onChange={e => setNewProject({...newProject, endTime: e.target.value})} className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white" /></div>
                </div>
                <div><label className="text-gray-400 text-sm mb-2 block">Website URL</label><input type="text" value={newProject.websiteUrl} onChange={e => setNewProject({...newProject, websiteUrl: e.target.value})} className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white" /></div>
                <div><label className="text-gray-400 text-sm mb-2 block">Logo URL</label><input type="text" value={newProject.logoUrl} onChange={e => setNewProject({...newProject, logoUrl: e.target.value})} className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white" /></div>
                <button onClick={handleAddProject} className="w-full bg-tiger-orange hover:bg-tiger-accent text-white py-4 rounded-xl font-bold">Create Project</button>
              </div>
            </motion.div>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
}
