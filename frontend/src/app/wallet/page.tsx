'use client';

/**
 * Wallet page.
 *
 * The page connects to the user's own wallet and reads balances from the
 * chain. It never generates, imports, displays, or stores a seed phrase or a
 * private key: those belong to the wallet, not to a web page.
 *
 * Where the API is required (chains, tokens, portfolio), the page uses the
 * validated endpoints in `@/hooks/useApi` and renders an explicit error when a
 * request fails.
 */

import { useEffect, useState } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import {
  Wallet,
  Send,
  RefreshCw,
  Copy,
  Settings,
  AlertTriangle,
  CheckCircle,
  Loader2,
  Menu,
  X,
  ExternalLink,
} from 'lucide-react';

import { useWalletStore } from '@/store/walletStore';
import { useChains, usePortfolio, useTokens } from '@/hooks/useApi';
import { formatAddress, parseUnits } from '@/lib/wallet';

type Tab = 'wallet' | 'send' | 'settings';

export default function WalletPage() {
  const {
    account,
    chainId,
    nativeBalance,
    isConnecting,
    isSending,
    error: walletError,
    connect,
    disconnect,
    setChain,
    refreshBalances,
    sendNative,
    clearError,
  } = useWalletStore();

  const { data: chains, error: chainsError } = useChains();
  const { data: tokens, error: tokensError } = useTokens();
  const { data: portfolio, error: portfolioError } = usePortfolio(Boolean(account));

  const [activeTab, setActiveTab] = useState<Tab>('wallet');
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false);

  const [sendTo, setSendTo] = useState('');
  const [sendAmount, setSendAmount] = useState('');
  const [sendTxHash, setSendTxHash] = useState('');
  const [sendStatus, setSendStatus] = useState<'idle' | 'pending' | 'success' | 'error'>('idle');
  const [sendError, setSendError] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);

  const currentChain = chains?.find((entry) => entry.id === chainId);

  // Balances must be re-read whenever the chain changes.
  useEffect(() => {
    if (account) {
      void refreshBalances();
    }
  }, [account, chainId, refreshBalances]);

  const handleConnect = async () => {
    clearError();
    try {
      await connect();
    } catch {
      // The store already recorded a user-facing message.
    }
  };

  const handleSend = async () => {
    setSendError(null);

    let amount: bigint;
    try {
      amount = parseUnits(sendAmount, 18);
    } catch (validationError) {
      setSendStatus('error');
      setSendError(validationError instanceof Error ? validationError.message : 'Invalid amount');
      return;
    }

    if (nativeBalance && amount > nativeBalance.raw) {
      setSendStatus('error');
      setSendError('Amount exceeds your available balance');
      return;
    }

    setSendStatus('pending');
    try {
      const hash = await sendNative(sendTo, sendAmount);
      setSendTxHash(hash);
      setSendStatus('success');
      setSendTo('');
      setSendAmount('');
    } catch (error) {
      setSendStatus('error');
      setSendError(error instanceof Error ? error.message : 'Transaction failed');
    }
  };

  const handleCopy = async (value: string) => {
    try {
      await navigator.clipboard.writeText(value);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 2000);
    } catch {
      setSendError('Could not copy to the clipboard');
    }
  };

  const explorerURL = currentChain?.explorerURL ?? null;

  if (!account) {
    return (
      <div className="min-h-screen bg-gradient-to-b from-tiger-dark via-[#0f0f1a] to-[#0a0a12]">
        <main className="max-w-3xl mx-auto px-4 py-20 text-center">
          <div className="w-24 h-24 bg-gradient-to-br from-tiger-orange to-yellow-500 rounded-full flex items-center justify-center mx-auto mb-8">
            <Wallet className="w-12 h-12 text-white" />
          </div>
          <h1 className="text-4xl font-bold text-white mb-4">Connect your wallet</h1>
          <p className="text-gray-400 mb-8">
            TigerSwap reads balances directly from your wallet and your wallet signs every
            transaction. Your keys never leave your device, and TigerSwap never asks for a
            recovery phrase.
          </p>
          <button
            onClick={() => void handleConnect()}
            disabled={isConnecting}
            className="bg-tiger-orange hover:bg-tiger-accent disabled:bg-gray-600 text-white px-8 py-4 rounded-xl font-bold text-lg transition-colors"
          >
            {isConnecting ? 'Connecting…' : 'Connect Wallet'}
          </button>
          {walletError && (
            <p role="alert" className="text-red-400 mt-6">{walletError}</p>
          )}
          {chainsError && (
            <p role="alert" className="text-red-400 mt-2">
              Chain list unavailable: {chainsError.message}
            </p>
          )}
        </main>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-gradient-to-b from-tiger-dark via-[#0f0f1a] to-[#0a0a12]">
      <header className="border-b border-white/5 bg-tiger-dark/80 backdrop-blur-xl sticky top-0 z-50">
        <div className="max-w-7xl mx-auto px-4 py-4 flex items-center justify-between">
          <div className="flex items-center space-x-3">
            <div className="w-10 h-10 bg-gradient-to-br from-tiger-orange to-yellow-500 rounded-xl flex items-center justify-center">
              <span className="text-white font-bold text-xl">T</span>
            </div>
            <span className="text-white font-bold text-xl">TigerWallet</span>
          </div>

          <div className="flex items-center space-x-4">
            <select
              value={chainId}
              onChange={(event) => void setChain(Number(event.target.value))}
              aria-label="Select network"
              className="appearance-none bg-white/10 border border-white/20 rounded-lg px-4 py-2 text-white focus:outline-none focus:border-tiger-orange"
            >
              {(chains ?? []).map((chain) => (
                <option key={chain.id} value={chain.id} className="bg-gray-900">
                  {chain.name}
                </option>
              ))}
            </select>
            <button
              onClick={() => void handleCopy(account.address)}
              className="text-gray-400 hover:text-white"
              title="Copy address"
            >
              <Copy className="w-5 h-5" />
            </button>
            <button
              onClick={disconnect}
              className="text-gray-400 hover:text-white"
              title="Disconnect"
            >
              <X className="w-5 h-5" />
            </button>
            <button
              onClick={() => setMobileMenuOpen(!mobileMenuOpen)}
              className="lg:hidden text-gray-400 hover:text-white"
            >
              {mobileMenuOpen ? <X className="w-6 h-6" /> : <Menu className="w-6 h-6" />}
            </button>
          </div>
        </div>
      </header>

      {mobileMenuOpen && (
        <div className="lg:hidden bg-tiger-dark border-b border-white/5 p-4">
          {([['wallet', 'Wallet'], ['send', 'Send'], ['settings', 'Settings']] as const).map(
            ([id, label]) => (
              <button
                key={id}
                onClick={() => {
                  setActiveTab(id);
                  setMobileMenuOpen(false);
                }}
                className={`block w-full text-left px-4 py-3 rounded-lg ${
                  activeTab === id ? 'bg-tiger-orange/20 text-tiger-orange' : 'text-gray-400'
                }`}
              >
                {label}
              </button>
            ),
          )}
        </div>
      )}

      <nav className="hidden lg:flex border-b border-white/5 bg-tiger-dark/50">
        <div className="max-w-7xl mx-auto px-4 flex items-center space-x-1">
          {([['wallet', 'Wallet'], ['send', 'Send'], ['settings', 'Settings']] as const).map(
            ([id, label]) => (
              <button
                key={id}
                onClick={() => setActiveTab(id)}
                className={`px-4 py-3 text-sm font-medium border-b-2 transition-colors ${
                  activeTab === id
                    ? 'border-tiger-orange text-tiger-orange'
                    : 'border-transparent text-gray-400 hover:text-white'
                }`}
              >
                {label}
              </button>
            ),
          )}
        </div>
      </nav>

      <main className="max-w-7xl mx-auto px-4 py-8">
        {copied && (
          <div className="mb-4 text-green-400 text-sm">Address copied to the clipboard</div>
        )}
        {walletError && (
          <div role="alert" className="mb-4 bg-red-500/10 border border-red-500/20 rounded-xl p-4 text-red-400">
            {walletError}
          </div>
        )}

        <AnimatePresence mode="wait">
          {activeTab === 'wallet' && (
            <motion.div
              key="wallet"
              initial={{ opacity: 0, y: 10 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: -10 }}
            >
              <div className="bg-gradient-to-br from-tiger-orange/20 to-yellow-500/10 rounded-2xl p-6 mb-6">
                <div className="flex items-center justify-between mb-6">
                  <div>
                    <p className="text-gray-400 text-sm mb-1">{currentChain?.name ?? 'Unknown network'}</p>
                    <h2 className="text-2xl font-bold text-white font-mono">
                      {formatAddress(account.address)}
                    </h2>
                  </div>
                  <button
                    onClick={() => void refreshBalances()}
                    className="p-2 bg-white/10 rounded-lg text-gray-400 hover:text-white"
                    title="Refresh balances"
                  >
                    <RefreshCw className="w-5 h-5" />
                  </button>
                </div>
                <div>
                  <p className="text-gray-400 text-sm">Balance</p>
                  <p className="text-4xl font-bold text-white">
                    {nativeBalance ? `${nativeBalance.formatted} ${nativeBalance.symbol}` : '—'}
                  </p>
                </div>
              </div>

              <div className="bg-white/5 rounded-2xl p-6">
                <h3 className="text-white font-semibold mb-4">Portfolio</h3>
                {portfolioError && (
                  <p role="alert" className="text-red-400 text-sm mb-4">
                    Portfolio unavailable: {portfolioError.message}
                  </p>
                )}
                {portfolio ? (
                  portfolio.assets.length === 0 ? (
                    <p className="text-gray-400 text-sm">No assets found for this wallet.</p>
                  ) : (
                    <div className="space-y-3">
                      {portfolio.assets.map((asset) => (
                        <div
                          key={`${asset.chainId}-${asset.tokenAddress}`}
                          className="flex items-center justify-between p-3 bg-white/5 rounded-xl"
                        >
                          <div>
                            <p className="text-white font-medium">{asset.symbol}</p>
                            <p className="text-gray-400 text-sm font-mono">
                              {formatAddress(asset.tokenAddress)}
                            </p>
                          </div>
                          <div className="text-right">
                            <p className="text-white font-medium">{asset.balance}</p>
                            <p className="text-gray-400 text-sm">
                              {asset.valueUSD === null ? '—' : `$${asset.valueUSD.toLocaleString()}`}
                            </p>
                          </div>
                        </div>
                      ))}
                    </div>
                  )
                ) : (
                  <p className="text-gray-400 text-sm">Loading portfolio…</p>
                )}
              </div>

              {tokensError && (
                <p role="alert" className="text-red-400 text-sm mt-4">
                  Token list unavailable: {tokensError.message}
                </p>
              )}
            </motion.div>
          )}

          {activeTab === 'send' && (
            <motion.div
              key="send"
              initial={{ opacity: 0, y: 10 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: -10 }}
              className="bg-white/5 rounded-2xl p-6 max-w-xl"
            >
              <h3 className="text-white font-semibold mb-6">Send {nativeBalance?.symbol ?? 'native token'}</h3>
              <div className="space-y-4">
                <div>
                  <label htmlFor="send-to" className="text-gray-400 text-sm mb-2 block">
                    Recipient address
                  </label>
                  <input
                    id="send-to"
                    type="text"
                    value={sendTo}
                    onChange={(event) => setSendTo(event.target.value)}
                    placeholder="0x…"
                    className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white font-mono focus:outline-none focus:border-tiger-orange"
                  />
                </div>
                <div>
                  <label htmlFor="send-amount" className="text-gray-400 text-sm mb-2 block">
                    Amount
                  </label>
                  <input
                    id="send-amount"
                    type="text"
                    inputMode="decimal"
                    value={sendAmount}
                    onChange={(event) => setSendAmount(event.target.value)}
                    placeholder="0.00"
                    className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white focus:outline-none focus:border-tiger-orange"
                  />
                  <p className="text-gray-500 text-xs mt-2">
                    Available: {nativeBalance ? `${nativeBalance.formatted} ${nativeBalance.symbol}` : '—'}
                  </p>
                </div>

                {sendStatus === 'pending' && (
                  <div className="bg-yellow-500/10 border border-yellow-500/20 rounded-xl p-4 flex items-center space-x-3">
                    <Loader2 className="w-5 h-5 text-yellow-500 animate-spin" />
                    <span className="text-yellow-500">Confirm the transaction in your wallet…</span>
                  </div>
                )}
                {sendStatus === 'error' && sendError && (
                  <div role="alert" className="bg-red-500/10 border border-red-500/20 rounded-xl p-4 text-red-400">
                    {sendError}
                  </div>
                )}
                {sendStatus === 'success' && (
                  <div className="bg-green-500/10 border border-green-500/20 rounded-xl p-4">
                    <div className="flex items-center space-x-2 mb-2">
                      <CheckCircle className="w-5 h-5 text-green-500" />
                      <span className="text-green-400">Transaction submitted</span>
                    </div>
                    <p className="text-white font-mono text-xs break-all mb-2">{sendTxHash}</p>
                    {explorerURL && (
                      <a
                        href={`${explorerURL}/tx/${sendTxHash}`}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="text-tiger-orange text-sm flex items-center space-x-1"
                      >
                        <span>View on explorer</span>
                        <ExternalLink className="w-3 h-3" />
                      </a>
                    )}
                  </div>
                )}

                <button
                  onClick={() => void handleSend()}
                  disabled={isSending || !sendTo || !sendAmount}
                  className="w-full bg-tiger-orange hover:bg-tiger-accent disabled:bg-gray-600 disabled:cursor-not-allowed text-white py-4 rounded-xl font-bold transition-colors flex items-center justify-center space-x-2"
                >
                  {isSending ? (
                    <>
                      <Loader2 className="w-5 h-5 animate-spin" />
                      <span>Sending…</span>
                    </>
                  ) : (
                    <>
                      <Send className="w-5 h-5" />
                      <span>Send</span>
                    </>
                  )}
                </button>
              </div>
            </motion.div>
          )}

          {activeTab === 'settings' && (
            <motion.div
              key="settings"
              initial={{ opacity: 0, y: 10 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: -10 }}
              className="space-y-6 max-w-2xl"
            >
              <div className="bg-white/5 rounded-2xl p-6">
                <h3 className="text-white font-semibold mb-4">Wallet information</h3>
                <div className="space-y-4">
                  <div className="flex justify-between items-center py-3 border-b border-white/10">
                    <span className="text-gray-400">Address</span>
                    <span className="text-white font-mono">{account.address}</span>
                  </div>
                  <div className="flex justify-between items-center py-3 border-b border-white/10">
                    <span className="text-gray-400">Network</span>
                    <span className="text-white">{currentChain?.name ?? chainId}</span>
                  </div>
                  <div className="flex justify-between items-center py-3">
                    <span className="text-gray-400">Chain ID</span>
                    <span className="text-white">{chainId}</span>
                  </div>
                </div>
              </div>

              <div className="bg-white/5 rounded-2xl p-6">
                <h3 className="text-white font-semibold mb-2">Key management</h3>
                <p className="text-gray-400 text-sm flex items-start space-x-2">
                  <AlertTriangle className="w-4 h-4 text-yellow-500 mt-0.5 shrink-0" />
                  <span>
                    TigerSwap does not hold your keys and will never ask for your recovery
                    phrase. Manage your keys in your wallet application.
                  </span>
                </p>
              </div>

              <button
                onClick={disconnect}
                className="w-full bg-red-500/10 hover:bg-red-500/20 text-red-400 py-4 rounded-xl font-medium transition-colors border border-red-500/20"
              >
                Disconnect wallet
              </button>
            </motion.div>
          )}
        </AnimatePresence>
      </main>

      <footer className="border-t border-white/5 bg-tiger-dark/50 mt-12">
        <div className="max-w-7xl mx-auto px-4 py-8 text-center text-gray-500 text-sm">
          © {new Date().getFullYear()} TigerSwap. All rights reserved.
        </div>
      </footer>
    </div>
  );
}
