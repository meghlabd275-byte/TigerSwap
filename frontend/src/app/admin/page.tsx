'use client';

import { useState } from 'react';
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
  Edit,
  Trash2,
  ToggleLeft,
  ToggleRight,
  Search,
  X,
  Check,
  AlertTriangle,
  BarChart3,
  RefreshCw,
  Lock,
  Unlock,
  Link,
  Copy,
  ExternalLink
} from 'lucide-react';
import { useAdminStore, Blockchain, Token, LaunchpadProject } from '@/store/adminStore';

type Tab = 'dashboard' | 'blockchains' | 'tokens' | 'launchpad' | 'fees' | 'settings' | 'activity';

export default function AdminPage() {
  const {
    isAdminAuthenticated,
    blockchains,
    tokens,
    launchpadProjects,
    feeSettings,
    systemSettings,
    activityLogs,
    stats,
    authenticateAdmin,
    logoutAdmin,
    addBlockchain,
    updateBlockchain,
    deleteBlockchain,
    toggleBlockchain,
    addToken,
    updateToken,
    deleteToken,
    toggleToken,
    verifyToken,
    createLaunchpadProject,
    updateLaunchpadProject,
    deleteLaunchpadProject,
    updateLaunchpadStatus,
    updateFeeSettings,
    updateSystemSettings,
    toggleMaintenanceMode,
  } = useAdminStore();

  const [activeTab, setActiveTab] = useState<Tab>('dashboard');
  const [adminSeed, setAdminSeed] = useState('');
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
    type: 'evm' as const,
    rpcUrl: '',
    explorerUrl: '',
    logoUrl: '',
    isActive: true,
    isTestnet: false,
    nativeToken: { name: '', symbol: '', decimals: 18, address: '' }
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
    isNative: false,
    totalSupply: '',
    priceUSD: 0
  });

  const [newProject, setNewProject] = useState({
    name: '',
    description: '',
    tokenAddress: '',
    tokenSymbol: '',
    tokenName: '',
    tokenDecimals: 18,
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
    socialLinks: { twitter: '', telegram: '', discord: '' }
  });

  const handleLogin = () => {
    if (authenticateAdmin(adminSeed)) {
      setAdminSeed('');
    }
  };

  const handleAddBlockchain = () => {
    if (!newBlockchain.name || !newBlockchain.symbol) return;
    addBlockchain(newBlockchain);
    setShowAddBlockchain(false);
    setNewBlockchain({
      chainId: 0, name: '', symbol: '', type: 'evm', rpcUrl: '', explorerUrl: '', logoUrl: '', isActive: true, isTestnet: false,
      nativeToken: { name: '', symbol: '', decimals: 18, address: '' }
    });
  };

  const handleAddToken = () => {
    if (!newToken.name || !newToken.symbol) return;
    addToken(newToken);
    setShowAddToken(false);
    setNewToken({ address: '', chainId: 1, name: '', symbol: '', decimals: 18, logoUrl: '', isActive: true, isVerified: false, isNative: false, totalSupply: '', priceUSD: 0 });
  };

  const handleAddProject = () => {
    if (!newProject.name || !newProject.tokenSymbol) return;
    createLaunchpadProject({
      ...newProject,
      status: 'upcoming',
      startTime: new Date(newProject.startTime),
      endTime: new Date(newProject.endTime)
    });
    setShowAddProject(false);
  };

  const filteredBlockchains = blockchains.filter(b => 
    b.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
    b.symbol.toLowerCase().includes(searchQuery.toLowerCase())
  );

  const filteredTokens = tokens.filter(t => 
    t.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
    t.symbol.toLowerCase().includes(searchQuery.toLowerCase()) ||
    (filterChain ? t.chainId === filterChain : true)
  );

  if (!isAdminAuthenticated) {
    return (
      <div className="min-h-screen bg-gradient-to-b from-tiger-dark via-[#0f0f1a] to-[#0a0a12] flex items-center justify-center p-4">
        <div className="bg-white/5 border border-white/10 rounded-2xl p-8 w-full max-w-md">
          <div className="text-center mb-8">
            <div className="w-20 h-20 bg-gradient-to-br from-tiger-orange to-yellow-500 rounded-full flex items-center justify-center mx-auto mb-4">
              <Shield className="w-10 h-10 text-white" />
            </div>
            <h1 className="text-3xl font-bold text-white mb-2">Admin Login</h1>
            <p className="text-gray-400">Enter your master wallet seed phrase</p>
          </div>
          <textarea
            value={adminSeed}
            onChange={(e) => setAdminSeed(e.target.value)}
            placeholder="Enter 24-word seed phrase..."
            rows={4}
            className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white placeholder-gray-500 focus:outline-none focus:border-tiger-orange mb-4 font-mono"
          />
          <button
            onClick={handleLogin}
            disabled={adminSeed.split(' ').length !== 24}
            className="w-full bg-tiger-orange hover:bg-tiger-accent disabled:bg-gray-600 disabled:cursor-not-allowed text-white py-4 rounded-xl font-bold transition-colors"
          >
            Login
          </button>
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
                onClick={toggleMaintenanceMode}
                className={`px-4 py-2 rounded-lg font-medium transition-colors ${
                  systemSettings.maintenanceMode 
                    ? 'bg-red-500/20 text-red-400' 
                    : 'bg-green-500/20 text-green-400'
                }`}
              >
                {systemSettings.maintenanceMode ? (
                  <span className="flex items-center space-x-2"><Lock className="w-4 h-4" /><span>Maintenance ON</span></span>
                ) : (
                  <span className="flex items-center space-x-2"><Unlock className="w-4 h-4" /><span>Live</span></span>
                )}
              </button>
              <button
                onClick={logoutAdmin}
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
        <AnimatePresence mode="wait">
          {activeTab === 'dashboard' && (
            <motion.div key="dashboard" initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: -10 }}>
              <h2 className="text-2xl font-bold text-white mb-6">Dashboard</h2>
              <div className="grid md:grid-cols-4 gap-6 mb-8">
                <div className="bg-white/5 rounded-2xl p-6">
                  <div className="flex items-center justify-between mb-4">
                    <Users className="w-8 h-8 text-tiger-orange" />
                    <span className="text-green-400 text-sm">+12%</span>
                  </div>
                  <p className="text-3xl font-bold text-white">{stats.totalUsers.toLocaleString()}</p>
                  <p className="text-gray-400 text-sm">Total Users</p>
                </div>
                <div className="bg-white/5 rounded-2xl p-6">
                  <div className="flex items-center justify-between mb-4">
                    <Activity className="w-8 h-8 text-tiger-orange" />
                    <span className="text-green-400 text-sm">+8%</span>
                  </div>
                  <p className="text-3xl font-bold text-white">{stats.totalTransactions.toLocaleString()}</p>
                  <p className="text-gray-400 text-sm">Total Transactions</p>
                </div>
                <div className="bg-white/5 rounded-2xl p-6">
                  <div className="flex items-center justify-between mb-4">
                    <DollarSign className="w-8 h-8 text-tiger-orange" />
                    <span className="text-green-400 text-sm">+15%</span>
                  </div>
                  <p className="text-3xl font-bold text-white">${parseFloat(stats.totalVolume).toLocaleString()}</p>
                  <p className="text-gray-400 text-sm">Total Volume</p>
                </div>
                <div className="bg-white/5 rounded-2xl p-6">
                  <div className="flex items-center justify-between mb-4">
                    <Wallet className="w-8 h-8 text-tiger-orange" />
                  </div>
                  <p className="text-3xl font-bold text-white">${parseFloat(stats.totalRevenue).toLocaleString()}</p>
                  <p className="text-gray-400 text-sm">Total Revenue</p>
                </div>
              </div>

              <div className="grid md:grid-cols-3 gap-6">
                <div className="bg-white/5 rounded-2xl p-6">
                  <h3 className="text-white font-semibold mb-4">System Status</h3>
                  <div className="space-y-3">
                    <div className="flex justify-between items-center">
                      <span className="text-gray-400">Trading</span>
                      <span className={`px-2 py-1 rounded text-xs ${systemSettings.tradingEnabled ? 'bg-green-500/20 text-green-400' : 'bg-red-500/20 text-red-400'}`}>
                        {systemSettings.tradingEnabled ? 'Enabled' : 'Disabled'}
                      </span>
                    </div>
                    <div className="flex justify-between items-center">
                      <span className="text-gray-400">Withdrawals</span>
                      <span className={`px-2 py-1 rounded text-xs ${systemSettings.withdrawalEnabled ? 'bg-green-500/20 text-green-400' : 'bg-red-500/20 text-red-400'}`}>
                        {systemSettings.withdrawalEnabled ? 'Enabled' : 'Disabled'}
                      </span>
                    </div>
                    <div className="flex justify-between items-center">
                      <span className="text-gray-400">Deposits</span>
                      <span className={`px-2 py-1 rounded text-xs ${systemSettings.depositEnabled ? 'bg-green-500/20 text-green-400' : 'bg-red-500/20 text-red-400'}`}>
                        {systemSettings.depositEnabled ? 'Enabled' : 'Disabled'}
                      </span>
                    </div>
                    <div className="flex justify-between items-center">
                      <span className="text-gray-400">Registration</span>
                      <span className={`px-2 py-1 rounded text-xs ${systemSettings.newUserRegistrationEnabled ? 'bg-green-500/20 text-green-400' : 'bg-red-500/20 text-red-400'}`}>
                        {systemSettings.newUserRegistrationEnabled ? 'Enabled' : 'Disabled'}
                      </span>
                    </div>
                  </div>
                </div>

                <div className="bg-white/5 rounded-2xl p-6">
                  <h3 className="text-white font-semibold mb-4">Quick Stats</h3>
                  <div className="space-y-3">
                    <div className="flex justify-between items-center">
                      <span className="text-gray-400">Blockchains</span>
                      <span className="text-white font-medium">{blockchains.length}</span>
                    </div>
                    <div className="flex justify-between items-center">
                      <span className="text-gray-400">Active Chains</span>
                      <span className="text-white font-medium">{blockchains.filter(b => b.isActive).length}</span>
                    </div>
                    <div className="flex justify-between items-center">
                      <span className="text-gray-400">Tokens</span>
                      <span className="text-white font-medium">{tokens.length}</span>
                    </div>
                    <div className="flex justify-between items-center">
                      <span className="text-gray-400">Verified Tokens</span>
                      <span className="text-white font-medium">{tokens.filter(t => t.isVerified).length}</span>
                    </div>
                  </div>
                </div>

                <div className="bg-white/5 rounded-2xl p-6">
                  <h3 className="text-white font-semibold mb-4">Recent Activity</h3>
                  <div className="space-y-3">
                    {activityLogs.slice(0, 5).map(log => (
                      <div key={log.id} className="text-sm">
                        <p className="text-white">{log.action}</p>
                        <p className="text-gray-500 text-xs">{new Date(log.timestamp).toLocaleString()}</p>
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
                <h2 className="text-2xl font-bold text-white">Blockchains ({blockchains.length})</h2>
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
                            <img src={chain.logoUrl} alt={chain.name} className="w-8 h-8 rounded-full" />
                            <div>
                              <p className="text-white font-medium">{chain.name}</p>
                              <p className="text-gray-400 text-sm">{chain.symbol}</p>
                            </div>
                          </div>
                        </td>
                        <td className="px-6 py-4 text-white font-mono">{chain.chainId}</td>
                        <td className="px-6 py-4">
                          <span className="px-2 py-1 bg-white/10 rounded text-xs text-white">{chain.type.toUpperCase()}</span>
                        </td>
                        <td className="px-6 py-4 text-gray-400 text-sm truncate max-w-xs">{chain.rpcUrl}</td>
                        <td className="px-6 py-4">
                          <button
                            onClick={() => toggleBlockchain(chain.id)}
                            className={chain.isActive ? 'text-green-400' : 'text-red-400'}
                          >
                            {chain.isActive ? <ToggleRight className="w-6 h-6" /> : <ToggleLeft className="w-6 h-6" />}
                          </button>
                        </td>
                        <td className="px-6 py-4">
                          <div className="flex items-center space-x-2">
                            <button className="p-2 text-gray-400 hover:text-white"><Edit className="w-4 h-4" /></button>
                            <button onClick={() => deleteBlockchain(chain.id)} className="p-2 text-gray-400 hover:text-red-400"><Trash2 className="w-4 h-4" /></button>
                          </div>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </motion.div>
          )}

          {activeTab === 'tokens' && (
            <motion.div key="tokens" initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: -10 }}>
              <div className="flex items-center justify-between mb-6">
                <h2 className="text-2xl font-bold text-white">Tokens ({tokens.length})</h2>
                <div className="flex items-center space-x-3">
                  <select
                    value={filterChain || ''}
                    onChange={(e) => setFilterChain(e.target.value ? parseInt(e.target.value) : null)}
                    className="px-4 py-2 bg-white/10 border border-white/20 rounded-lg text-white focus:outline-none focus:border-tiger-orange"
                  >
                    <option value="">All Chains</option>
                    {blockchains.map(b => (
                      <option key={b.id} value={b.chainId}>{b.name}</option>
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
                      <th className="text-left px-6 py-4 text-gray-400 font-medium">Price</th>
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
                            <img src={token.logoUrl} alt={token.symbol} className="w-8 h-8 rounded-full" />
                            <div>
                              <p className="text-white font-medium">{token.name}</p>
                              <p className="text-gray-400 text-sm">{token.symbol}</p>
                            </div>
                          </div>
                        </td>
                        <td className="px-6 py-4 text-white">{blockchains.find(b => b.chainId === token.chainId)?.name || token.chainId}</td>
                        <td className="px-6 py-4 text-gray-400 text-sm font-mono truncate max-w-xs">{token.address.slice(0, 10)}...</td>
                        <td className="px-6 py-4 text-white">${token.priceUSD.toFixed(2)}</td>
                        <td className="px-6 py-4">
                          {token.isVerified ? (
                            <Check className="w-5 h-5 text-green-400" />
                          ) : (
                            <button onClick={() => verifyToken(token.id)} className="text-yellow-400 text-sm hover:underline">Verify</button>
                          )}
                        </td>
                        <td className="px-6 py-4">
                          <button onClick={() => toggleToken(token.id)} className={token.isActive ? 'text-green-400' : 'text-red-400'}>
                            {token.isActive ? <ToggleRight className="w-6 h-6" /> : <ToggleLeft className="w-6 h-6" />}
                          </button>
                        </td>
                        <td className="px-6 py-4">
                          <div className="flex items-center space-x-2">
                            <button className="p-2 text-gray-400 hover:text-white"><Edit className="w-4 h-4" /></button>
                            <button onClick={() => deleteToken(token.id)} className="p-2 text-gray-400 hover:text-red-400"><Trash2 className="w-4 h-4" /></button>
                          </div>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </motion.div>
          )}

          {activeTab === 'launchpad' && (
            <motion.div key="launchpad" initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: -10 }}>
              <div className="flex items-center justify-between mb-6">
                <h2 className="text-2xl font-bold text-white">Launchpad Projects ({launchpadProjects.length})</h2>
                <button
                  onClick={() => setShowAddProject(true)}
                  className="flex items-center space-x-2 px-4 py-2 bg-tiger-orange text-white rounded-lg hover:bg-tiger-accent transition-colors"
                >
                  <Plus className="w-4 h-4" />
                  <span>Create Project</span>
                </button>
              </div>

              <div className="grid md:grid-cols-2 lg:grid-cols-3 gap-6">
                {launchpadProjects.length === 0 ? (
                  <div className="md:col-span-2 lg:col-span-3 bg-white/5 rounded-2xl p-12 text-center">
                    <Rocket className="w-16 h-16 text-gray-400 mx-auto mb-4" />
                    <p className="text-gray-400 text-lg">No launchpad projects yet</p>
                    <button onClick={() => setShowAddProject(true)} className="mt-4 text-tiger-orange hover:underline">Create your first project</button>
                  </div>
                ) : (
                  launchpadProjects.map(project => (
                    <div key={project.id} className="bg-white/5 rounded-2xl p-6">
                      <div className="flex items-start justify-between mb-4">
                        <div className="flex items-center space-x-3">
                          <img src={project.logoUrl} alt={project.name} className="w-12 h-12 rounded-xl" />
                          <div>
                            <h3 className="text-white font-semibold">{project.name}</h3>
                            <p className="text-tiger-orange">{project.tokenSymbol}</p>
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
                        <div><p className="text-gray-400">Raised</p><p className="text-white">${project.raisedAmount} / ${project.hardCap}</p></div>
                        <div><p className="text-gray-400">Price</p><p className="text-white">${project.pricePerToken}</p></div>
                      </div>
                      <div className="flex items-center space-x-2">
                        {project.status === 'upcoming' && (
                          <button onClick={() => updateLaunchpadStatus(project.id, 'active')} className="flex-1 bg-green-500/20 text-green-400 py-2 rounded-lg text-sm hover:bg-green-500/30">Activate</button>
                        )}
                        {project.status === 'active' && (
                          <button onClick={() => updateLaunchpadStatus(project.id, 'completed')} className="flex-1 bg-blue-500/20 text-blue-400 py-2 rounded-lg text-sm hover:bg-blue-500/30">Complete</button>
                        )}
                        <button onClick={() => deleteLaunchpadProject(project.id)} className="p-2 text-red-400 hover:bg-red-500/20 rounded-lg"><Trash2 className="w-4 h-4" /></button>
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
              <div className="grid md:grid-cols-2 gap-6">
                <div className="bg-white/5 rounded-2xl p-6">
                  <h3 className="text-white font-semibold mb-6">Transaction Fees</h3>
                  <div className="space-y-4">
                    <div>
                      <label className="text-gray-400 text-sm mb-2 block">Withdraw Fee (%)</label>
                      <input
                        type="number"
                        step="0.01"
                        value={feeSettings.withdrawFeePercent}
                        onChange={(e) => updateFeeSettings({ withdrawFeePercent: parseFloat(e.target.value) })}
                        className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white focus:outline-none focus:border-tiger-orange"
                      />
                    </div>
                    <div>
                      <label className="text-gray-400 text-sm mb-2 block">Swap Fee (%)</label>
                      <input
                        type="number"
                        step="0.01"
                        value={feeSettings.swapFeePercent}
                        onChange={(e) => updateFeeSettings({ swapFeePercent: parseFloat(e.target.value) })}
                        className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white focus:outline-none focus:border-tiger-orange"
                      />
                    </div>
                    <div>
                      <label className="text-gray-400 text-sm mb-2 block">Deposit Fee (%)</label>
                      <input
                        type="number"
                        step="0.01"
                        value={feeSettings.depositFeePercent}
                        onChange={(e) => updateFeeSettings({ depositFeePercent: parseFloat(e.target.value) })}
                        className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white focus:outline-none focus:border-tiger-orange"
                      />
                    </div>
                  </div>
                </div>
                <div className="bg-white/5 rounded-2xl p-6">
                  <h3 className="text-white font-semibold mb-6">Other Fees</h3>
                  <div className="space-y-4">
                    <div>
                      <label className="text-gray-400 text-sm mb-2 block">Launchpad Fee (%)</label>
                      <input
                        type="number"
                        step="0.01"
                        value={feeSettings.launchpadFeePercent}
                        onChange={(e) => updateFeeSettings({ launchpadFeePercent: parseFloat(e.target.value) })}
                        className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white focus:outline-none focus:border-tiger-orange"
                      />
                    </div>
                    <div>
                      <label className="text-gray-400 text-sm mb-2 block">Referral Fee (%)</label>
                      <input
                        type="number"
                        step="0.01"
                        value={feeSettings.referralFeePercent}
                        onChange={(e) => updateFeeSettings({ referralFeePercent: parseFloat(e.target.value) })}
                        className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white focus:outline-none focus:border-tiger-orange"
                      />
                    </div>
                    <div>
                      <label className="text-gray-400 text-sm mb-2 block">Transaction Fee (%)</label>
                      <input
                        type="number"
                        step="0.01"
                        value={feeSettings.transactionFeePercent}
                        onChange={(e) => updateFeeSettings({ transactionFeePercent: parseFloat(e.target.value) })}
                        className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white focus:outline-none focus:border-tiger-orange"
                      />
                    </div>
                  </div>
                </div>
              </div>
            </motion.div>
          )}

          {activeTab === 'settings' && (
            <motion.div key="settings" initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: -10 }}>
              <h2 className="text-2xl font-bold text-white mb-6">System Settings</h2>
              <div className="bg-white/5 rounded-2xl p-6">
                <div className="space-y-6">
                  <div className="flex items-center justify-between">
                    <div>
                      <h3 className="text-white font-medium">Trading</h3>
                      <p className="text-gray-400 text-sm">Enable or disable trading functionality</p>
                    </div>
                    <button
                      onClick={() => updateSystemSettings({ tradingEnabled: !systemSettings.tradingEnabled })}
                      className={systemSettings.tradingEnabled ? 'text-green-400' : 'text-red-400'}
                    >
                      {systemSettings.tradingEnabled ? <ToggleRight className="w-8 h-8" /> : <ToggleLeft className="w-8 h-8" />}
                    </button>
                  </div>
                  <div className="flex items-center justify-between">
                    <div>
                      <h3 className="text-white font-medium">Withdrawals</h3>
                      <p className="text-gray-400 text-sm">Enable or disable withdrawal functionality</p>
                    </div>
                    <button
                      onClick={() => updateSystemSettings({ withdrawalEnabled: !systemSettings.withdrawalEnabled })}
                      className={systemSettings.withdrawalEnabled ? 'text-green-400' : 'text-red-400'}
                    >
                      {systemSettings.withdrawalEnabled ? <ToggleRight className="w-8 h-8" /> : <ToggleLeft className="w-8 h-8" />}
                    </button>
                  </div>
                  <div className="flex items-center justify-between">
                    <div>
                      <h3 className="text-white font-medium">Deposits</h3>
                      <p className="text-gray-400 text-sm">Enable or disable deposit functionality</p>
                    </div>
                    <button
                      onClick={() => updateSystemSettings({ depositEnabled: !systemSettings.depositEnabled })}
                      className={systemSettings.depositEnabled ? 'text-green-400' : 'text-red-400'}
                    >
                      {systemSettings.depositEnabled ? <ToggleRight className="w-8 h-8" /> : <ToggleLeft className="w-8 h-8" />}
                    </button>
                  </div>
                  <div className="flex items-center justify-between">
                    <div>
                      <h3 className="text-white font-medium">User Registration</h3>
                      <p className="text-gray-400 text-sm">Enable or disable new user registrations</p>
                    </div>
                    <button
                      onClick={() => updateSystemSettings({ newUserRegistrationEnabled: !systemSettings.newUserRegistrationEnabled })}
                      className={systemSettings.newUserRegistrationEnabled ? 'text-green-400' : 'text-red-400'}
                    >
                      {systemSettings.newUserRegistrationEnabled ? <ToggleRight className="w-8 h-8" /> : <ToggleLeft className="w-8 h-8" />}
                    </button>
                  </div>
                </div>
              </div>
            </motion.div>
          )}

          {activeTab === 'activity' && (
            <motion.div key="activity" initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: -10 }}>
              <h2 className="text-2xl font-bold text-white mb-6">Activity Log</h2>
              <div className="bg-white/5 rounded-2xl overflow-hidden">
                {activityLogs.length === 0 ? (
                  <div className="p-12 text-center text-gray-400">No activity yet</div>
                ) : (
                  <table className="w-full">
                    <thead className="bg-white/5">
                      <tr>
                        <th className="text-left px-6 py-4 text-gray-400 font-medium">Action</th>
                        <th className="text-left px-6 py-4 text-gray-400 font-medium">Details</th>
                        <th className="text-left px-6 py-4 text-gray-400 font-medium">Time</th>
                        <th className="text-left px-6 py-4 text-gray-400 font-medium">IP</th>
                      </tr>
                    </thead>
                    <tbody>
                      {activityLogs.map(log => (
                        <tr key={log.id} className="border-t border-white/5">
                          <td className="px-6 py-4 text-white">{log.action}</td>
                          <td className="px-6 py-4 text-gray-400">{log.details}</td>
                          <td className="px-6 py-4 text-gray-400 text-sm">{new Date(log.timestamp).toLocaleString()}</td>
                          <td className="px-6 py-4 text-gray-400 font-mono text-sm">{log.ipAddress}</td>
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
                  <div><label className="text-gray-400 text-sm mb-2 block">Type</label><select value={newBlockchain.type} onChange={e => setNewBlockchain({...newBlockchain, type: e.target.value as any})} className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white"><option value="evm">EVM</option><option value="solana">Solana</option><option value="cosmos">Cosmos</option><option value="ton">TON</option><option value="aptos">Aptos</option></select></div>
                </div>
                <div><label className="text-gray-400 text-sm mb-2 block">RPC URL</label><input type="text" value={newBlockchain.rpcUrl} onChange={e => setNewBlockchain({...newBlockchain, rpcUrl: e.target.value})} className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white" /></div>
                <div><label className="text-gray-400 text-sm mb-2 block">Explorer URL</label><input type="text" value={newBlockchain.explorerUrl} onChange={e => setNewBlockchain({...newBlockchain, explorerUrl: e.target.value})} className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white" /></div>
                <div><label className="text-gray-400 text-sm mb-2 block">Logo URL</label><input type="text" value={newBlockchain.logoUrl} onChange={e => setNewBlockchain({...newBlockchain, logoUrl: e.target.value})} className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white" /></div>
                <div className="grid grid-cols-3 gap-4">
                  <div><label className="text-gray-400 text-sm mb-2 block">Native Name</label><input type="text" value={newBlockchain.nativeToken.name} onChange={e => setNewBlockchain({...newBlockchain, nativeToken: {...newBlockchain.nativeToken, name: e.target.value}})} className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white" /></div>
                  <div><label className="text-gray-400 text-sm mb-2 block">Native Symbol</label><input type="text" value={newBlockchain.nativeToken.symbol} onChange={e => setNewBlockchain({...newBlockchain, nativeToken: {...newBlockchain.nativeToken, symbol: e.target.value}})} className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white" /></div>
                  <div><label className="text-gray-400 text-sm mb-2 block">Decimals</label><input type="number" value={newBlockchain.nativeToken.decimals} onChange={e => setNewBlockchain({...newBlockchain, nativeToken: {...newBlockchain.nativeToken, decimals: parseInt(e.target.value)}})} className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white" /></div>
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
                  <div><label className="text-gray-400 text-sm mb-2 block">Chain</label><select value={newToken.chainId} onChange={e => setNewToken({...newToken, chainId: parseInt(e.target.value)})} className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white">{blockchains.map(b => (<option key={b.id} value={b.chainId}>{b.name}</option>))}</select></div>
                  <div><label className="text-gray-400 text-sm mb-2 block">Decimals</label><input type="number" value={newToken.decimals} onChange={e => setNewToken({...newToken, decimals: parseInt(e.target.value)})} className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white" /></div>
                </div>
                <div><label className="text-gray-400 text-sm mb-2 block">Contract Address</label><input type="text" value={newToken.address} onChange={e => setNewToken({...newToken, address: e.target.value})} className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white" /></div>
                <div><label className="text-gray-400 text-sm mb-2 block">Logo URL</label><input type="text" value={newToken.logoUrl} onChange={e => setNewToken({...newToken, logoUrl: e.target.value})} className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white" /></div>
                <div><label className="text-gray-400 text-sm mb-2 block">Price (USD)</label><input type="number" step="0.01" value={newToken.priceUSD} onChange={e => setNewToken({...newToken, priceUSD: parseFloat(e.target.value)})} className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white" /></div>
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
