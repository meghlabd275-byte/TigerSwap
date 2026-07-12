'use client';

import { useState } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { 
  Wallet, 
  Send,
  RefreshCw, 
  Copy, 
  ChevronDown,
  Settings,
  Layers,
  Rocket,
  Coins,
  Key,
  Eye,
  EyeOff,
  Plus,
  ExternalLink,
  AlertTriangle,
  CheckCircle,
  Loader2,
  Menu,
  X,
  Download
} from 'lucide-react';
import { useWalletStore } from '@/store/walletStore';
import { 
  getSupportedChains, 
  getPopularTokens, 
  formatAddress, 
  getExplorerUrl,
  validateMnemonic,
  generateMnemonic
} from '@/services/walletService';

type Tab = 'wallet' | 'send' | 'swap' | 'bridge' | 'launchpad' | 'settings';
type SendTab = 'send' | 'receive';
type LaunchpadTab = 'projects' | 'create';

export default function WalletPage() {
  const {
    masterWallet,
    isMasterWalletSet,
    userWallet,
    isUserWalletSet,
    currentChainId,
    currentWallet,
    isAdminMode,
    isLoading,
    isGenerating,
    isSending,
    feeConfig,
    launchpadProjects,
    popularTokens,
    createMasterWallet,
    importMasterWallet,
    createUserWallet,
    importUserWallet,
    setCurrentChain,
    sendTransaction,
    swapTokens,
    addLaunchpadProject,
    setFeeConfig,
    setAdminMode,
    logoutUser,
    logoutMaster,
    refreshBalances
  } = useWalletStore();

  const [activeTab, setActiveTab] = useState<Tab>('wallet');
  const [sendTab, setSendTab] = useState<SendTab>('send');
  const [launchpadTab, setLaunchpadTab] = useState<LaunchpadTab>('projects');
  
  // Wallet modal
  const [showWalletModal, setShowWalletModal] = useState(false);
  const [walletMode, setWalletMode] = useState<'create' | 'import'>('create');
  const [walletType, setWalletType] = useState<'master' | 'user'>('user');
  const [seedPhrase, setSeedPhrase] = useState('');
  const [showSeedPhrase, setShowSeedPhrase] = useState(false);
  const [walletError, setWalletError] = useState('');
  const [walletSuccess, setWalletSuccess] = useState('');
  
  // Send form
  const [sendTo, setSendTo] = useState('');
  const [sendAmount, setSendAmount] = useState('');
  const [sendToken, setSendToken] = useState('0x0000000000000000000000000000000000000000');
  const [sendTxHash, setSendTxHash] = useState('');
  const [sendStatus, setSendStatus] = useState<'idle' | 'pending' | 'success' | 'error'>('idle');
  
  // Swap form
  const [swapFromToken, setSwapFromToken] = useState('');
  const [swapToToken, setSwapToToken] = useState('');
  const [swapAmount, setSwapAmount] = useState('');
  const [swapSlippage, setSwapSlippage] = useState(0.5);
  
  // Bridge form
  const [bridgeFromChain, setBridgeFromChain] = useState(1);
  const [bridgeToChain, setBridgeToChain] = useState(137);
  const [bridgeAmount, setBridgeAmount] = useState('');
  
  // Launchpad form
  const [newProject, setNewProject] = useState({
    name: '',
    description: '',
    tokenSymbol: '',
    tokenAddress: '',
    price: '',
    softCap: '',
    hardCap: '',
    startTime: '',
    endTime: ''
  });
  
  // Settings
  const [newFeeConfig, setNewFeeConfig] = useState({...feeConfig});
  
  // Mobile menu
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false);

  const chains = getSupportedChains();
  const tokens = getPopularTokens();
  const hasWallet = isMasterWalletSet || isUserWalletSet;

  // Handle create wallet
  const handleCreateWallet = async () => {
    setWalletError('');
    setWalletSuccess('');
    
    try {
      if (walletType === 'master') {
        await createMasterWallet();
      } else {
        await createUserWallet();
      }
      setWalletSuccess('Wallet created successfully!');
      setTimeout(() => {
        setShowWalletModal(false);
        setSeedPhrase('');
        setWalletSuccess('');
      }, 3000);
    } catch (error: any) {
      setWalletError(error.message || 'Failed to create wallet');
    }
  };

  // Handle import wallet
  const handleImportWallet = async () => {
    setWalletError('');
    setWalletSuccess('');
    
    if (!seedPhrase.trim()) {
      setWalletError('Please enter your seed phrase');
      return;
    }
    
    if (!validateMnemonic(seedPhrase.trim())) {
      setWalletError('Invalid seed phrase');
      return;
    }
    
    try {
      if (walletType === 'master') {
        await importMasterWallet(seedPhrase.trim());
      } else {
        await importUserWallet(seedPhrase.trim());
      }
      setWalletSuccess('Wallet imported successfully!');
      setTimeout(() => {
        setShowWalletModal(false);
        setSeedPhrase('');
        setWalletSuccess('');
      }, 3000);
    } catch (error: any) {
      setWalletError(error.message || 'Failed to import wallet');
    }
  };

  // Handle send
  const handleSend = async () => {
    if (!sendTo || !sendAmount) return;
    setSendStatus('pending');
    try {
      const hash = await sendTransaction(sendTo, sendAmount, sendToken);
      setSendTxHash(hash);
      setSendStatus('success');
      setSendTo('');
      setSendAmount('');
    } catch (error: any) {
      setSendStatus('error');
    }
  };

  // Handle swap
  const handleSwap = async () => {
    if (!swapFromToken || !swapToToken || !swapAmount) return;
    try {
      await swapTokens(swapFromToken, swapToToken, swapAmount);
    } catch (error) {
      console.error(error);
    }
  };

  // Handle create project
  const handleCreateProject = () => {
    if (!newProject.name || !newProject.tokenSymbol) return;
    addLaunchpadProject({
      ...newProject,
      totalSupply: '1000000000',
      raisedAmount: '0',
      status: 'upcoming',
      logo: 'https://via.placeholder.com/150',
      startTime: new Date(newProject.startTime),
      endTime: new Date(newProject.endTime),
      minPurchase: '100',
      maxPurchase: '10000',
      website: '',
      whitepaper: ''
    });
    setNewProject({ name: '', description: '', tokenSymbol: '', tokenAddress: '', price: '', softCap: '', hardCap: '', startTime: '', endTime: '' });
  };

  const handleLogout = () => {
    if (isAdminMode) logoutMaster();
    else logoutUser();
  };

  const toggleAdminMode = () => setAdminMode(!isAdminMode);

  const currentWalletAddress = currentWallet?.address || '';
  const currentWalletBalance = currentWallet?.balance || '0';

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
                <span className="text-white font-bold text-xl">TigerWallet</span>
              </div>
              {isMasterWalletSet && (
                <button
                  onClick={toggleAdminMode}
                  className={`px-3 py-1.5 rounded-lg text-sm font-medium transition-all ${isAdminMode ? 'bg-tiger-orange text-white' : 'bg-white/10 text-gray-400 hover:text-white'}`}
                >
                  {isAdminMode ? '👑 Master' : '👤 User'}
                </button>
              )}
            </div>

            <div className="flex items-center space-x-4">
              <select
                value={currentChainId}
                onChange={(e) => setCurrentChain(parseInt(e.target.value))}
                className="appearance-none bg-white/10 border border-white/20 rounded-lg px-4 py-2 pr-10 text-white focus:outline-none focus:border-tiger-orange"
              >
                {chains.filter(c => c.type === 'evm').slice(0, 20).map(chain => (
                  <option key={chain.id} value={chain.id} className="bg-gray-900">{chain.name}</option>
                ))}
              </select>

              {!hasWallet ? (
                <button
                  onClick={() => { setWalletType('user'); setWalletMode('create'); setShowWalletModal(true); }}
                  className="bg-tiger-orange hover:bg-tiger-accent text-white px-4 py-2 rounded-lg font-medium transition-colors"
                >
                  Create Wallet
                </button>
              ) : (
                <div className="flex items-center space-x-2">
                  <button onClick={handleLogout} className="p-2 text-gray-400 hover:text-white transition-colors" title="Logout">
                    <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M15.75 9V5.25A2.25 2.25 0 0013.5 3h-6a2.25 2.25 0 00-2.25 2.25v13.5A2.25 2.25 0 007.5 21h6a2.25 2.25 0 002.25-2.25V15M12 9l-3 3m0 0l3 3m-3-3h12.75" /></svg>
                  </button>
                  <button onClick={() => setActiveTab('settings')} className="p-2 text-gray-400 hover:text-white transition-colors">
                    <Settings className="w-5 h-5" />
                  </button>
                </div>
              )}
              
              <button onClick={() => setMobileMenuOpen(!mobileMenuOpen)} className="lg:hidden p-2 text-gray-400 hover:text-white">
                {mobileMenuOpen ? <X className="w-6 h-6" /> : <Menu className="w-6 h-6" />}
              </button>
            </div>
          </div>
        </div>
      </header>

      {/* Mobile menu */}
      {mobileMenuOpen && (
        <div className="lg:hidden bg-tiger-dark border-b border-white/5 p-4">
          <div className="flex flex-col space-y-2">
            {[{ id: 'wallet', label: 'Wallet', icon: Wallet }, { id: 'send', label: 'Send', icon: Send }, { id: 'swap', label: 'Swap', icon: RefreshCw }, { id: 'bridge', label: 'Bridge', icon: Layers }, { id: 'launchpad', label: 'Launchpad', icon: Rocket }, { id: 'settings', label: 'Settings', icon: Settings }].map(tab => (
              <button
                key={tab.id}
                onClick={() => { setActiveTab(tab.id as Tab); setMobileMenuOpen(false); }}
                className={`flex items-center space-x-3 px-4 py-3 rounded-lg transition-colors ${activeTab === tab.id ? 'bg-tiger-orange/20 text-tiger-orange' : 'text-gray-400 hover:text-white hover:bg-white/5'}`}
              >
                <tab.icon className="w-5 h-5" /><span>{tab.label}</span>
              </button>
            ))}
          </div>
        </div>
      )}

      {/* Desktop Navigation */}
      <nav className="hidden lg:flex border-b border-white/5 bg-tiger-dark/50">
        <div className="max-w-7xl mx-auto px-4">
          <div className="flex items-center space-x-1">
            {[{ id: 'wallet', label: 'Wallet', icon: Wallet }, { id: 'send', label: 'Send', icon: Send }, { id: 'swap', label: 'Swap', icon: RefreshCw }, { id: 'bridge', label: 'Bridge', icon: Layers }, { id: 'launchpad', label: 'Launchpad', icon: Rocket }, { id: 'settings', label: 'Settings', icon: Settings }].map(tab => (
              <button
                key={tab.id}
                onClick={() => setActiveTab(tab.id as Tab)}
                className={`flex items-center space-x-2 px-4 py-3 text-sm font-medium border-b-2 transition-colors ${activeTab === tab.id ? 'border-tiger-orange text-tiger-orange' : 'border-transparent text-gray-400 hover:text-white'}`}
              >
                <tab.icon className="w-4 h-4" /><span>{tab.label}</span>
              </button>
            ))}
          </div>
        </div>
      </nav>

      {/* Main Content */}
      <main className="max-w-7xl mx-auto px-4 py-8">
        {!hasWallet ? (
          <div className="text-center py-20">
            <div className="w-24 h-24 bg-gradient-to-br from-tiger-orange to-yellow-500 rounded-full flex items-center justify-center mx-auto mb-8">
              <Wallet className="w-12 h-12 text-white" />
            </div>
            <h1 className="text-4xl font-bold text-white mb-4">Welcome to TigerWallet</h1>
            <p className="text-gray-400 mb-8 max-w-2xl mx-auto">The most advanced multi-chain DeFi wallet. Manage all your crypto assets across 100+ blockchains, swap, bridge, and participate in launchpads.</p>
            <div className="flex flex-col sm:flex-row items-center justify-center space-y-4 sm:space-y-0 sm:space-x-4">
              <button onClick={() => { setWalletType('user'); setWalletMode('create'); setShowWalletModal(true); }} className="bg-tiger-orange hover:bg-tiger-accent text-white px-8 py-4 rounded-xl font-bold text-lg transition-colors">Create New Wallet</button>
              <button onClick={() => { setWalletType('user'); setWalletMode('import'); setShowWalletModal(true); }} className="bg-white/10 hover:bg-white/20 text-white px-8 py-4 rounded-xl font-bold text-lg transition-colors border border-white/20">Import Wallet</button>
            </div>
            <div className="grid md:grid-cols-3 gap-8 mt-20">
              {[{ icon: Layers, title: '100+ Chains', desc: 'Support for all major blockchains' }, { icon: RefreshCw, title: 'Instant Swap', desc: 'Trade across any chain instantly' }, { icon: Rocket, title: 'Launchpad', desc: 'Join IDOs and token sales' }].map((feature, i) => (
                <div key={i} className="bg-white/5 rounded-2xl p-6">
                  <feature.icon className="w-10 h-10 text-tiger-orange mx-auto mb-4" />
                  <h3 className="text-white font-semibold mb-2">{feature.title}</h3>
                  <p className="text-gray-400 text-sm">{feature.desc}</p>
                </div>
              ))}
            </div>
          </div>
        ) : (
          <div className="grid lg:grid-cols-4 gap-6">
            <div className="lg:col-span-3">
              <AnimatePresence mode="wait">
                {activeTab === 'wallet' && (
                  <motion.div key="wallet" initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: -10 }}>
                    <div className="bg-gradient-to-br from-tiger-orange/20 to-yellow-500/10 rounded-2xl p-6 mb-6">
                      <div className="flex items-center justify-between mb-6">
                        <div>
                          <p className="text-gray-400 text-sm mb-1">{isAdminMode ? 'Master Wallet' : 'User Wallet'} • {chains.find(c => c.id === currentChainId)?.name}</p>
                          <div className="flex items-center space-x-2">
                            <h2 className="text-2xl font-bold text-white">{formatAddress(currentWalletAddress)}</h2>
                            <button onClick={() => navigator.clipboard.writeText(currentWalletAddress)} className="text-gray-400 hover:text-white"><Copy className="w-4 h-4" /></button>
                          </div>
                        </div>
                        <button onClick={refreshBalances} className="p-2 bg-white/10 rounded-lg text-gray-400 hover:text-white"><RefreshCw className="w-5 h-5" /></button>
                      </div>
                      <div className="mb-6">
                        <p className="text-gray-400 text-sm">Balance</p>
                        <p className="text-4xl font-bold text-white">{parseFloat(currentWalletBalance).toFixed(6)} {chains.find(c => c.id === currentChainId)?.symbol}</p>
                      </div>
                      <div className="flex space-x-3">
                        <button onClick={() => setSendTab('send')} className="flex-1 bg-tiger-orange hover:bg-tiger-accent text-white py-3 rounded-xl font-medium transition-colors flex items-center justify-center space-x-2"><Send className="w-4 h-4" /><span>Send</span></button>
                        <button onClick={() => setSendTab('receive')} className="flex-1 bg-white/10 hover:bg-white/20 text-white py-3 rounded-xl font-medium transition-colors flex items-center justify-center space-x-2 border border-white/20"><Download className="w-4 h-4" /><span>Receive</span></button>
                      </div>
                    </div>
                    {sendTab === 'receive' && (
                      <div className="bg-white/5 rounded-2xl p-6">
                        <h3 className="text-white font-semibold mb-4">Receive {chains.find(c => c.id === currentChainId)?.symbol}</h3>
                        <div className="bg-white p-4 rounded-xl w-48 h-48 mx-auto mb-4 flex items-center justify-center text-gray-400 text-sm">QR Code</div>
                        <p className="text-center text-gray-400 text-sm mb-2">Scan to receive</p>
                        <p className="text-center text-white font-mono text-sm break-all">{currentWalletAddress}</p>
                      </div>
                    )}
                    <div className="bg-white/5 rounded-2xl p-6 mt-6">
                      <h3 className="text-white font-semibold mb-4">Assets</h3>
                      <div className="space-y-3">
                        {tokens.slice(0, 10).map((token, i) => (
                          <div key={i} className="flex items-center justify-between p-3 bg-white/5 rounded-xl">
                            <div className="flex items-center space-x-3">
                              <div className="w-10 h-10 bg-white/10 rounded-full flex items-center justify-center"><span className="text-xs text-white">{token.symbol.slice(0,2)}</span></div>
                              <div><p className="text-white font-medium">{token.symbol}</p><p className="text-gray-400 text-sm">{token.name}</p></div>
                            </div>
                            <div className="text-right"><p className="text-white font-medium">0.00</p><p className="text-gray-400 text-sm">$0.00</p></div>
                          </div>
                        ))}
                      </div>
                    </div>
                  </motion.div>
                )}
                {activeTab === 'send' && (
                  <motion.div key="send" initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: -10 }} className="bg-white/5 rounded-2xl p-6">
                    <h3 className="text-white font-semibold mb-6">Send Crypto</h3>
                    <div className="space-y-4">
                      <div><label className="text-gray-400 text-sm mb-2 block">Recipient Address</label><input type="text" value={sendTo} onChange={(e) => setSendTo(e.target.value)} placeholder="0x..." className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white placeholder-gray-500 focus:outline-none focus:border-tiger-orange" /></div>
                      <div><label className="text-gray-400 text-sm mb-2 block">Token</label><select value={sendToken} onChange={(e) => setSendToken(e.target.value)} className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white focus:outline-none focus:border-tiger-orange">{tokens.map((token, i) => (<option key={i} value={token.address} className="bg-gray-900">{token.symbol} - {token.name}</option>))}</select></div>
                      <div><label className="text-gray-400 text-sm mb-2 block">Amount</label><input type="number" value={sendAmount} onChange={(e) => setSendAmount(e.target.value)} placeholder="0.00" className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white placeholder-gray-500 focus:outline-none focus:border-tiger-orange" /></div>
                      <div className="bg-white/5 rounded-xl p-4"><div className="flex justify-between text-sm mb-2"><span className="text-gray-400">Transaction Fee</span><span className="text-white">{feeConfig.transactionFeePercent}%</span></div>{isAdminMode && <div className="flex justify-between text-sm"><span className="text-gray-400">Network Fee</span><span className="text-tiger-orange">Free (Master Wallet)</span></div>}</div>
                      {sendStatus === 'pending' && <div className="bg-yellow-500/10 border border-yellow-500/20 rounded-xl p-4 flex items-center space-x-3"><Loader2 className="w-5 h-5 text-yellow-500 animate-spin" /><span className="text-yellow-500">Processing...</span></div>}
                      {sendStatus === 'success' && <div className="bg-green-500/10 border border-green-500/20 rounded-xl p-4"><div className="flex items-center space-x-2 mb-2"><CheckCircle className="w-5 h-5 text-green-500" /><span className="text-green-500">Success!</span></div><a href={getExplorerUrl(currentChainId, sendTxHash)} target="_blank" rel="noopener noreferrer" className="text-tiger-orange text-sm flex items-center space-x-1"><span>View on Explorer</span><ExternalLink className="w-3 h-3" /></a></div>}
                      <button onClick={handleSend} disabled={isSending || !sendTo || !sendAmount} className="w-full bg-tiger-orange hover:bg-tiger-accent disabled:bg-gray-600 disabled:cursor-not-allowed text-white py-4 rounded-xl font-bold transition-colors flex items-center justify-center space-x-2">{isSending ? <><Loader2 className="w-5 h-5 animate-spin" /><span>Sending...</span></> : <><Send className="w-5 h-5" /><span>Send {chains.find(c => c.id === currentChainId)?.symbol}</span></>}</button>
                    </div>
                  </motion.div>
                )}
                {activeTab === 'swap' && (
                  <motion.div key="swap" initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: -10 }} className="bg-white/5 rounded-2xl p-6">
                    <h3 className="text-white font-semibold mb-6">Swap Tokens</h3>
                    <div className="space-y-4">
                      <div><label className="text-gray-400 text-sm mb-2 block">From</label><div className="flex space-x-3"><select value={swapFromToken} onChange={(e) => setSwapFromToken(e.target.value)} className="flex-1 bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white focus:outline-none focus:border-tiger-orange"><option value="">Select Token</option>{tokens.map((token, i) => (<option key={i} value={token.address} className="bg-gray-900">{token.symbol}</option>))}</select><input type="number" value={swapAmount} onChange={(e) => setSwapAmount(e.target.value)} placeholder="0.00" className="w-32 bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white placeholder-gray-500 focus:outline-none focus:border-tiger-orange text-right" /></div></div>
                      <div className="flex justify-center"><div className="bg-white/10 p-2 rounded-full"><RefreshCw className="w-5 h-5 text-gray-400" /></div></div>
                      <div><label className="text-gray-400 text-sm mb-2 block">To</label><select value={swapToToken} onChange={(e) => setSwapToToken(e.target.value)} className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white focus:outline-none focus:border-tiger-orange"><option value="">Select Token</option>{tokens.map((token, i) => (<option key={i} value={token.address} className="bg-gray-900">{token.symbol}</option>))}</select></div>
                      <div><label className="text-gray-400 text-sm mb-2 block">Slippage Tolerance</label><div className="flex space-x-2">{[0.1, 0.5, 1, 3].map(slip => (<button key={slip} onClick={() => setSwapSlippage(slip)} className={`flex-1 py-2 rounded-lg text-sm font-medium transition-colors ${swapSlippage === slip ? 'bg-tiger-orange text-white' : 'bg-white/10 text-gray-400 hover:text-white'}`}>{slip}%</button>))}</div></div>
                      <button onClick={handleSwap} disabled={!swapFromToken || !swapToToken || !swapAmount || isSending} className="w-full bg-tiger-orange hover:bg-tiger-accent disabled:bg-gray-600 disabled:cursor-not-allowed text-white py-4 rounded-xl font-bold transition-colors">{isSending ? 'Swapping...' : 'Swap Tokens'}</button>
                    </div>
                  </motion.div>
                )}
                {activeTab === 'bridge' && (
                  <motion.div key="bridge" initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: -10 }} className="bg-white/5 rounded-2xl p-6">
                    <h3 className="text-white font-semibold mb-6">Bridge Crypto</h3>
                    <div className="space-y-4">
                      <div><label className="text-gray-400 text-sm mb-2 block">From Chain</label><select value={bridgeFromChain} onChange={(e) => setBridgeFromChain(parseInt(e.target.value))} className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white focus:outline-none focus:border-tiger-orange">{chains.filter(c => c.type === 'evm').slice(0, 15).map(chain => (<option key={chain.id} value={chain.id} className="bg-gray-900">{chain.name}</option>))}</select></div>
                      <div className="flex justify-center"><div className="bg-white/10 p-2 rounded-full"><Layers className="w-5 h-5 text-gray-400" /></div></div>
                      <div><label className="text-gray-400 text-sm mb-2 block">To Chain</label><select value={bridgeToChain} onChange={(e) => setBridgeToChain(parseInt(e.target.value))} className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white focus:outline-none focus:border-tiger-orange">{chains.filter(c => c.type === 'evm').slice(0, 15).map(chain => (<option key={chain.id} value={chain.id} className="bg-gray-900">{chain.name}</option>))}</select></div>
                      <div><label className="text-gray-400 text-sm mb-2 block">Amount</label><input type="number" value={bridgeAmount} onChange={(e) => setBridgeAmount(e.target.value)} placeholder="0.00" className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white placeholder-gray-500 focus:outline-none focus:border-tiger-orange" /></div>
                      <button disabled={!bridgeAmount || bridgeFromChain === bridgeToChain} className="w-full bg-tiger-orange hover:bg-tiger-accent disabled:bg-gray-600 disabled:cursor-not-allowed text-white py-4 rounded-xl font-bold transition-colors">Bridge {chains.find(c => c.id === bridgeFromChain)?.symbol} to {chains.find(c => c.id === bridgeToChain)?.name}</button>
                    </div>
                  </motion.div>
                )}
                {activeTab === 'launchpad' && (
                  <motion.div key="launchpad" initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: -10 }}>
                    {isAdminMode && (
                      <div className="flex space-x-2 mb-6">
                        <button onClick={() => setLaunchpadTab('projects')} className={`px-4 py-2 rounded-lg font-medium transition-colors ${launchpadTab === 'projects' ? 'bg-tiger-orange text-white' : 'bg-white/10 text-gray-400 hover:text-white'}`}>All Projects</button>
                        <button onClick={() => setLaunchpadTab('create')} className={`px-4 py-2 rounded-lg font-medium transition-colors ${launchpadTab === 'create' ? 'bg-tiger-orange text-white' : 'bg-white/10 text-gray-400 hover:text-white'}`}>Create Project</button>
                      </div>
                    )}
                    {launchpadTab === 'create' && isAdminMode && (
                      <div className="bg-white/5 rounded-2xl p-6 mb-6">
                        <h3 className="text-white font-semibold mb-6">Create Launchpad Project</h3>
                        <div className="grid md:grid-cols-2 gap-4">
                          <div><label className="text-gray-400 text-sm mb-2 block">Project Name</label><input type="text" value={newProject.name} onChange={(e) => setNewProject({...newProject, name: e.target.value})} className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white focus:outline-none focus:border-tiger-orange" /></div>
                          <div><label className="text-gray-400 text-sm mb-2 block">Token Symbol</label><input type="text" value={newProject.tokenSymbol} onChange={(e) => setNewProject({...newProject, tokenSymbol: e.target.value})} className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white focus:outline-none focus:border-tiger-orange" /></div>
                          <div><label className="text-gray-400 text-sm mb-2 block">Price (USD)</label><input type="text" value={newProject.price} onChange={(e) => setNewProject({...newProject, price: e.target.value})} className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white focus:outline-none focus:border-tiger-orange" /></div>
                          <div><label className="text-gray-400 text-sm mb-2 block">Hard Cap (USD)</label><input type="text" value={newProject.hardCap} onChange={(e) => setNewProject({...newProject, hardCap: e.target.value})} className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white focus:outline-none focus:border-tiger-orange" /></div>
                          <div><label className="text-gray-400 text-sm mb-2 block">Start Time</label><input type="datetime-local" value={newProject.startTime} onChange={(e) => setNewProject({...newProject, startTime: e.target.value})} className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white focus:outline-none focus:border-tiger-orange" /></div>
                          <div><label className="text-gray-400 text-sm mb-2 block">End Time</label><input type="datetime-local" value={newProject.endTime} onChange={(e) => setNewProject({...newProject, endTime: e.target.value})} className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white focus:outline-none focus:border-tiger-orange" /></div>
                        </div>
                        <button onClick={handleCreateProject} className="w-full bg-tiger-orange hover:bg-tiger-accent text-white py-4 rounded-xl font-bold transition-colors mt-6">Create Project</button>
                      </div>
                    )}
                    <div className="grid md:grid-cols-2 gap-4">
                      {launchpadProjects.length === 0 ? (
                        <div className="md:col-span-2 bg-white/5 rounded-2xl p-8 text-center"><Rocket className="w-12 h-12 text-gray-400 mx-auto mb-4" /><p className="text-gray-400">No launchpad projects yet</p>{isAdminMode && <button onClick={() => setLaunchpadTab('create')} className="text-tiger-orange mt-2">Create your first project</button>}</div>
                      ) : (
                        launchpadProjects.map(project => (
                          <div key={project.id} className="bg-white/5 rounded-2xl p-6">
                            <div className="flex items-start justify-between mb-4">
                              <div><h4 className="text-white font-semibold">{project.name}</h4><p className="text-tiger-orange font-medium">{project.tokenSymbol}</p></div>
                              <span className={`px-3 py-1 rounded-full text-xs font-medium ${project.status === 'active' ? 'bg-green-500/20 text-green-400' : project.status === 'upcoming' ? 'bg-yellow-500/20 text-yellow-400' : 'bg-gray-500/20 text-gray-400'}`}>{project.status.toUpperCase()}</span>
                            </div>
                            <p className="text-gray-400 text-sm mb-4 line-clamp-2">{project.description}</p>
                            <div className="space-y-2 text-sm"><div className="flex justify-between"><span className="text-gray-400">Raised</span><span className="text-white">${project.raisedAmount} / ${project.hardCap}</span></div><div className="flex justify-between"><span className="text-gray-400">Price</span><span className="text-white">${project.price}</span></div></div>
                            <button disabled={project.status !== 'active'} className="w-full bg-tiger-orange hover:bg-tiger-accent disabled:bg-gray-600 disabled:cursor-not-allowed text-white py-3 rounded-xl font-medium transition-colors mt-4">{project.status === 'active' ? 'Participate' : 'Coming Soon'}</button>
                          </div>
                        ))
                      )}
                    </div>
                  </motion.div>
                )}
                {activeTab === 'settings' && (
                  <motion.div key="settings" initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: -10 }} className="space-y-6">
                    <div className="bg-white/5 rounded-2xl p-6">
                      <h3 className="text-white font-semibold mb-4">Wallet Information</h3>
                      <div className="space-y-4">
                        <div className="flex justify-between items-center py-3 border-b border-white/10"><span className="text-gray-400">Address</span><span className="text-white font-mono">{formatAddress(currentWalletAddress)}</span></div>
                        <div className="flex justify-between items-center py-3 border-b border-white/10"><span className="text-gray-400">Network</span><span className="text-white">{chains.find(c => c.id === currentChainId)?.name}</span></div>
                        <div className="flex justify-between items-center py-3"><span className="text-gray-400">Backup Code</span><span className="text-tiger-orange">{isAdminMode && masterWallet ? masterWallet.backupCode : '••••••••••••'}</span></div>
                      </div>
                    </div>
                    {isAdminMode && (
                      <div className="bg-white/5 rounded-2xl p-6">
                        <h3 className="text-white font-semibold mb-4">Fee Configuration</h3>
                        <div className="space-y-4">
                          <div><label className="text-gray-400 text-sm mb-2 block">Withdraw Fee (%)</label><input type="number" step="0.01" value={newFeeConfig.withdrawFeePercent} onChange={(e) => setNewFeeConfig({...newFeeConfig, withdrawFeePercent: parseFloat(e.target.value)})} className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white focus:outline-none focus:border-tiger-orange" /></div>
                          <div><label className="text-gray-400 text-sm mb-2 block">Swap Fee (%)</label><input type="number" step="0.01" value={newFeeConfig.swapFeePercent} onChange={(e) => setNewFeeConfig({...newFeeConfig, swapFeePercent: parseFloat(e.target.value)})} className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white focus:outline-none focus:border-tiger-orange" /></div>
                          <button onClick={() => setFeeConfig(newFeeConfig)} className="w-full bg-tiger-orange hover:bg-tiger-accent text-white py-3 rounded-xl font-medium transition-colors">Save Fee Settings</button>
                        </div>
                      </div>
                    )}
                    {!isMasterWalletSet && (
                      <div className="bg-white/5 rounded-2xl p-6">
                        <h3 className="text-white font-semibold mb-4">Add Master Wallet</h3>
                        <p className="text-gray-400 text-sm mb-4">Create or import a master wallet to access admin features.</p>
                        <div className="flex space-x-3">
                          <button onClick={() => { setWalletType('master'); setWalletMode('create'); setShowWalletModal(true); }} className="flex-1 bg-tiger-orange hover:bg-tiger-accent text-white py-3 rounded-xl font-medium transition-colors">Create Master</button>
                          <button onClick={() => { setWalletType('master'); setWalletMode('import'); setShowWalletModal(true); }} className="flex-1 bg-white/10 hover:bg-white/20 text-white py-3 rounded-xl font-medium transition-colors border border-white/20">Import Master</button>
                        </div>
                      </div>
                    )}
                    <button onClick={handleLogout} className="w-full bg-red-500/10 hover:bg-red-500/20 text-red-400 py-4 rounded-xl font-medium transition-colors border border-red-500/20">Logout</button>
                  </motion.div>
                )}
              </AnimatePresence>
            </div>
            
            {/* Sidebar */}
            <div className="space-y-6">
              <div className="bg-white/5 rounded-2xl p-4">
                <h4 className="text-white font-semibold mb-4">Quick Stats</h4>
                <div className="space-y-3">
                  <div className="flex justify-between"><span className="text-gray-400 text-sm">Total Balance</span><span className="text-white font-medium">$0.00</span></div>
                  <div className="flex justify-between"><span className="text-gray-400 text-sm">Networks</span><span className="text-white font-medium">{chains.length}+</span></div>
                  <div className="flex justify-between"><span className="text-gray-400 text-sm">Tokens</span><span className="text-white font-medium">{tokens.length}+</span></div>
                </div>
              </div>
              <div className="bg-white/5 rounded-2xl p-4">
                <h4 className="text-white font-semibold mb-4">Supported Chains</h4>
                <div className="grid grid-cols-4 gap-2">
                  {chains.filter(c => c.type === 'evm').slice(0, 16).map(chain => (
                    <button key={chain.id} onClick={() => setCurrentChain(chain.id)} className={`aspect-square rounded-lg flex items-center justify-center text-xs font-medium transition-colors ${currentChainId === chain.id ? 'bg-tiger-orange text-white' : 'bg-white/5 text-gray-400 hover:bg-white/10 hover:text-white'}`} title={chain.name}>{chain.symbol.slice(0, 3)}</button>
                  ))}
                </div>
                <p className="text-gray-400 text-xs mt-3 text-center">+{chains.length - 16} more chains</p>
              </div>
            </div>
          </div>
        )}
      </main>

      {/* Wallet Modal */}
      <AnimatePresence>
        {showWalletModal && (
          <motion.div initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }} className="fixed inset-0 bg-black/80 backdrop-blur-sm flex items-center justify-center z-50 p-4" onClick={() => setShowWalletModal(false)}>
            <motion.div initial={{ scale: 0.95, opacity: 0 }} animate={{ scale: 1, opacity: 1 }} exit={{ scale: 0.95, opacity: 0 }} className="bg-[#1a1a2e] border border-white/10 rounded-2xl p-6 w-full max-w-md" onClick={(e) => e.stopPropagation()}>
              <div className="flex items-center justify-between mb-6">
                <h3 className="text-xl font-bold text-white">{walletMode === 'create' ? 'Create' : 'Import'} {walletType === 'master' ? 'Master' : ''} Wallet</h3>
                <button onClick={() => setShowWalletModal(false)} className="text-gray-400 hover:text-white"><X className="w-6 h-6" /></button>
              </div>
              {walletSuccess && <div className="bg-green-500/10 border border-green-500/20 rounded-xl p-4 mb-4"><div className="flex items-center space-x-2"><CheckCircle className="w-5 h-5 text-green-500" /><span className="text-green-400">{walletSuccess}</span></div></div>}
              {walletError && <div className="bg-red-500/10 border border-red-500/20 rounded-xl p-4 mb-4"><div className="flex items-center space-x-2"><AlertTriangle className="w-5 h-5 text-red-500" /><span className="text-red-400">{walletError}</span></div></div>}
              {walletMode === 'create' ? (
                <div className="space-y-4">
                  <p className="text-gray-400 text-sm">Your 24-word recovery phrase is the only way to recover your wallet.</p>
                  {seedPhrase ? (
                    <div className="bg-white/5 rounded-xl p-4">
                      <div className="flex items-center justify-between mb-2"><span className="text-gray-400 text-sm">Recovery Phrase</span><button onClick={() => setShowSeedPhrase(!showSeedPhrase)} className="text-gray-400 hover:text-white">{showSeedPhrase ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}</button></div>
                      <p className={`text-white font-mono text-sm ${showSeedPhrase ? '' : 'blur-sm select-none'}`}>{seedPhrase}</p>
                    </div>
                  ) : (
                    <button onClick={() => setSeedPhrase(generateMnemonic())} className="w-full bg-white/10 hover:bg-white/20 text-white py-4 rounded-xl font-medium transition-colors border border-white/20">Generate Phrase</button>
                  )}
                  <button onClick={handleCreateWallet} disabled={!seedPhrase || isGenerating} className="w-full bg-tiger-orange hover:bg-tiger-accent disabled:bg-gray-600 disabled:cursor-not-allowed text-white py-4 rounded-xl font-bold transition-colors">{isGenerating ? <span className="flex items-center justify-center space-x-2"><Loader2 className="w-5 h-5 animate-spin" /><span>Creating...</span></span> : 'Create Wallet'}</button>
                </div>
              ) : (
                <div className="space-y-4">
                  <div><label className="text-gray-400 text-sm mb-2 block">Enter 24-word Recovery Phrase</label><textarea value={seedPhrase} onChange={(e) => setSeedPhrase(e.target.value)} placeholder="word1 word2 word3 ..." rows={4} className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white placeholder-gray-500 focus:outline-none focus:border-tiger-orange font-mono" /></div>
                  <button onClick={handleImportWallet} disabled={!seedPhrase || isGenerating} className="w-full bg-tiger-orange hover:bg-tiger-accent disabled:bg-gray-600 disabled:cursor-not-allowed text-white py-4 rounded-xl font-bold transition-colors">{isGenerating ? <span className="flex items-center justify-center space-x-2"><Loader2 className="w-5 h-5 animate-spin" /><span>Importing...</span></span> : 'Import Wallet'}</button>
                </div>
              )}
            </motion.div>
          </motion.div>
        )}
      </AnimatePresence>

      <footer className="border-t border-white/5 bg-tiger-dark/50 mt-12">
        <div className="max-w-7xl mx-auto px-4 py-8"><div className="text-center text-gray-500 text-sm">© 2024 TigerWallet. All rights reserved.</div></div>
      </footer>
    </div>
  );
}
