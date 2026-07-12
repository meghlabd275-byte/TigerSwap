'use client';

import { useState } from 'react';
import { motion } from 'framer-motion';
import { 
  Wallet, 
  ArrowDownLeft, 
  ArrowUpRight, 
  Copy, 
  Check,
  QrCode,
  ExternalLink,
  Clock,
  RefreshCw,
  AlertCircle,
  Loader2,
  CreditCard,
  Building,
  Globe,
  Key,
  Shield
} from 'lucide-react';

interface DepositWithdrawProps {
  currentWallet: {
    address: string;
    chainId: number;
    chainName: string;
  } | null;
  tokens: Array<{
    symbol: string;
    name: string;
    address: string;
    logoURI: string;
  }>;
  chainId: number;
}

export function DepositWithdraw({ currentWallet, tokens, chainId }: DepositWithdrawProps) {
  const [activeTab, setActiveTab] = useState<'deposit' | 'withdraw'>('deposit');
  const [selectedToken, setSelectedToken] = useState(tokens[0] || null);
  const [depositAmount, setDepositAmount] = useState('');
  const [withdrawAddress, setWithdrawAddress] = useState('');
  const [withdrawAmount, setWithdrawAmount] = useState('');
  const [showQR, setShowQR] = useState(false);
  const [copied, setCopied] = useState(false);
  const [isProcessing, setIsProcessing] = useState(false);

  // Mock deposit addresses for different chains
  const getDepositAddress = () => {
    if (!currentWallet) return '';
    return currentWallet.address;
  };

  // Mock bank transfer details
  const bankDetails = {
    bankName: 'TigerSwap Bank',
    accountName: 'TigerSwap Ltd',
    accountNumber: '1234567890',
    routingNumber: '987654321',
    iban: 'GB82 WEST 1234 5678 9012 34',
    swift: 'WESTGB2LXXX',
    reference: `TIGER${Date.now().toString().slice(-8)}`
  };

  const copyAddress = () => {
    if (currentWallet?.address) {
      navigator.clipboard.writeText(currentWallet.address);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    }
  };

  const handleDeposit = async () => {
    if (!depositAmount || parseFloat(depositAmount) <= 0) return;
    setIsProcessing(true);
    // Simulate deposit processing
    await new Promise(resolve => setTimeout(resolve, 2000));
    setIsProcessing(false);
    setDepositAmount('');
  };

  const handleWithdraw = async () => {
    if (!withdrawAddress || !withdrawAmount || parseFloat(withdrawAmount) <= 0) return;
    setIsProcessing(true);
    // Simulate withdrawal processing
    await new Promise(resolve => setTimeout(resolve, 2000));
    setIsProcessing(false);
    setWithdrawAddress('');
    setWithdrawAmount('');
  };

  if (!currentWallet) {
    return (
      <div className="bg-white/5 rounded-xl p-8 text-center">
        <Wallet className="w-16 h-16 text-gray-400 mx-auto mb-4" />
        <h3 className="text-white font-semibold mb-2">Connect Wallet</h3>
        <p className="text-gray-400">Connect your wallet to deposit or withdraw</p>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Tab Switcher */}
      <div className="flex items-center space-x-2 bg-white/5 rounded-xl p-1">
        <button
          onClick={() => setActiveTab('deposit')}
          className={`flex-1 flex items-center justify-center space-x-2 py-3 rounded-lg transition-colors ${
            activeTab === 'deposit'
              ? 'bg-tiger-orange text-white'
              : 'text-gray-400 hover:text-white'
          }`}
        >
          <ArrowDownLeft className="w-5 h-5" />
          <span>Deposit</span>
        </button>
        <button
          onClick={() => setActiveTab('withdraw')}
          className={`flex-1 flex items-center justify-center space-x-2 py-3 rounded-lg transition-colors ${
            activeTab === 'withdraw'
              ? 'bg-tiger-orange text-white'
              : 'text-gray-400 hover:text-white'
          }`}
        >
          <ArrowUpRight className="w-5 h-5" />
          <span>Withdraw</span>
        </button>
      </div>

      {/* Content */}
      {activeTab === 'deposit' && (
        <motion.div
          initial={{ opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          className="space-y-6"
        >
          {/* Crypto Deposit */}
          <div className="bg-white/5 rounded-xl p-6">
            <h3 className="text-white font-semibold mb-4">Cryptocurrency Deposit</h3>
            
            {/* Token Selector */}
            <div className="mb-4">
              <label className="text-gray-400 text-sm mb-2 block">Select Token</label>
              <select
                value={selectedToken?.address || ''}
                onChange={(e) => setSelectedToken(tokens.find(t => t.address === e.target.value) || tokens[0])}
                className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white focus:outline-none focus:border-tiger-orange"
              >
                {tokens.map((token, i) => (
                  <option key={i} value={token.address} className="bg-gray-900">
                    {token.symbol} - {token.name}
                  </option>
                ))}
              </select>
            </div>

            {/* Deposit Address */}
            <div className="mb-4">
              <label className="text-gray-400 text-sm mb-2 block">Your Deposit Address</label>
              <div className="flex items-center space-x-2">
                <div className="flex-1 bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white font-mono text-sm truncate">
                  {getDepositAddress()}
                </div>
                <button
                  onClick={copyAddress}
                  className="p-3 bg-white/10 border border-white/20 rounded-xl text-gray-400 hover:text-white transition-colors"
                >
                  {copied ? <Check className="w-5 h-5 text-green-400" /> : <Copy className="w-5 h-5" />}
                </button>
                <button
                  onClick={() => setShowQR(!showQR)}
                  className="p-3 bg-white/10 border border-white/20 rounded-xl text-gray-400 hover:text-white transition-colors"
                >
                  <QrCode className="w-5 h-5" />
                </button>
              </div>
            </div>

            {/* QR Code */}
            {showQR && (
              <div className="mb-4 p-4 bg-white rounded-xl flex justify-center">
                <div className="w-48 h-48 bg-gray-200 flex items-center justify-center text-gray-500">
                  QR Code
                </div>
              </div>
            )}

            {/* Warning */}
            <div className="flex items-start space-x-3 p-4 bg-yellow-500/10 border border-yellow-500/20 rounded-xl">
              <AlertCircle className="w-5 h-5 text-yellow-400 flex-shrink-0 mt-0.5" />
              <div className="text-sm text-yellow-400">
                <p className="font-medium mb-1">Important</p>
                <p>Only send {selectedToken?.symbol || 'tokens'} to this address. Sending other tokens may result in permanent loss.</p>
              </div>
            </div>
          </div>

          {/* Bank Transfer */}
          <div className="bg-white/5 rounded-xl p-6">
            <div className="flex items-center space-x-2 mb-4">
              <Building className="w-5 h-5 text-tiger-orange" />
              <h3 className="text-white font-semibold">Bank Transfer (Fiat)</h3>
            </div>
            
            <div className="space-y-3 text-sm">
              <div className="flex justify-between py-2 border-b border-white/10">
                <span className="text-gray-400">Bank Name</span>
                <span className="text-white">{bankDetails.bankName}</span>
              </div>
              <div className="flex justify-between py-2 border-b border-white/10">
                <span className="text-gray-400">Account Name</span>
                <span className="text-white">{bankDetails.accountName}</span>
              </div>
              <div className="flex justify-between py-2 border-b border-white/10">
                <span className="text-gray-400">Account Number</span>
                <span className="text-white font-mono">{bankDetails.accountNumber}</span>
              </div>
              <div className="flex justify-between py-2 border-b border-white/10">
                <span className="text-gray-400">Routing Number</span>
                <span className="text-white font-mono">{bankDetails.routingNumber}</span>
              </div>
              <div className="flex justify-between py-2 border-b border-white/10">
                <span className="text-gray-400">IBAN</span>
                <span className="text-white font-mono">{bankDetails.iban}</span>
              </div>
              <div className="flex justify-between py-2 border-b border-white/10">
                <span className="text-gray-400">SWIFT</span>
                <span className="text-white font-mono">{bankDetails.swift}</span>
              </div>
              <div className="flex justify-between py-2">
                <span className="text-gray-400">Reference</span>
                <span className="text-tiger-orange font-mono">{bankDetails.reference}</span>
              </div>
            </div>

            <p className="text-gray-400 text-xs mt-4">
              Please include your reference number in the payment description. Deposits are processed within 1-3 business days.
            </p>
          </div>
        </motion.div>
      )}

      {activeTab === 'withdraw' && (
        <motion.div
          initial={{ opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          className="space-y-6"
        >
          <div className="bg-white/5 rounded-xl p-6">
            <h3 className="text-white font-semibold mb-4">Withdraw Cryptocurrency</h3>
            
            {/* Token Selector */}
            <div className="mb-4">
              <label className="text-gray-400 text-sm mb-2 block">Select Token</label>
              <select
                value={selectedToken?.address || ''}
                onChange={(e) => setSelectedToken(tokens.find(t => t.address === e.target.value) || tokens[0])}
                className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white focus:outline-none focus:border-tiger-orange"
              >
                {tokens.map((token, i) => (
                  <option key={i} value={token.address} className="bg-gray-900">
                    {token.symbol} - {token.name}
                  </option>
                ))}
              </select>
            </div>

            {/* Recipient Address */}
            <div className="mb-4">
              <label className="text-gray-400 text-sm mb-2 block">Recipient Address</label>
              <input
                type="text"
                value={withdrawAddress}
                onChange={(e) => setWithdrawAddress(e.target.value)}
                placeholder="0x..."
                className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white placeholder-gray-500 focus:outline-none focus:border-tiger-orange font-mono"
              />
            </div>

            {/* Amount */}
            <div className="mb-4">
              <label className="text-gray-400 text-sm mb-2 block">Amount</label>
              <div className="relative">
                <input
                  type="number"
                  value={withdrawAmount}
                  onChange={(e) => setWithdrawAmount(e.target.value)}
                  placeholder="0.00"
                  className="w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white placeholder-gray-500 focus:outline-none focus:border-tiger-orange"
                />
                <span className="absolute right-4 top-1/2 -translate-y-1/2 text-gray-400">
                  {selectedToken?.symbol || 'TOKEN'}
                </span>
              </div>
            </div>

            {/* Fee Info */}
            <div className="bg-white/5 rounded-xl p-4 mb-4">
              <div className="flex justify-between text-sm mb-2">
                <span className="text-gray-400">Network Fee</span>
                <span className="text-white">~0.001 {selectedToken?.symbol || ''}</span>
              </div>
              <div className="flex justify-between text-sm">
                <span className="text-gray-400">Withdrawal Fee</span>
                <span className="text-tiger-orange">0.1%</span>
              </div>
            </div>

            {/* Submit */}
            <button
              onClick={handleWithdraw}
              disabled={isProcessing || !withdrawAddress || !withdrawAmount}
              className="w-full bg-tiger-orange hover:bg-tiger-accent disabled:bg-gray-600 disabled:cursor-not-allowed text-white py-4 rounded-xl font-bold transition-colors flex items-center justify-center space-x-2"
            >
              {isProcessing ? (
                <>
                  <Loader2 className="w-5 h-5 animate-spin" />
                  <span>Processing...</span>
                </>
              ) : (
                <>
                  <ArrowUpRight className="w-5 h-5" />
                  <span>Withdraw {selectedToken?.symbol || ''}</span>
                </>
              )}
            </button>
          </div>

          {/* Fiat Withdrawal */}
          <div className="bg-white/5 rounded-xl p-6">
            <div className="flex items-center space-x-2 mb-4">
              <CreditCard className="w-5 h-5 text-tiger-orange" />
              <h3 className="text-white font-semibold">Fiat Withdrawal</h3>
            </div>
            <p className="text-gray-400 text-sm mb-4">
              Request a bank transfer to your registered bank account. Fiat withdrawals are processed within 1-5 business days.
            </p>
            <button className="w-full bg-white/10 hover:bg-white/20 text-white py-3 rounded-xl font-medium transition-colors border border-white/20">
              Request Fiat Withdrawal
            </button>
          </div>
        </motion.div>
      )}

      {/* Recent Transactions */}
      <div className="bg-white/5 rounded-xl p-6">
        <div className="flex items-center justify-between mb-4">
          <h3 className="text-white font-semibold">Recent Transactions</h3>
          <button className="text-tiger-orange text-sm hover:underline">View All</button>
        </div>
        <div className="space-y-3">
          {[
            { type: 'deposit', token: 'USDC', amount: '1,000.00', time: '2 hours ago', status: 'confirmed' },
            { type: 'withdraw', token: 'ETH', amount: '0.5', time: '5 hours ago', status: 'confirmed' },
            { type: 'deposit', token: 'BTC', amount: '0.1', time: '1 day ago', status: 'confirmed' },
          ].map((tx, i) => (
            <div key={i} className="flex items-center justify-between p-3 bg-white/5 rounded-lg">
              <div className="flex items-center space-x-3">
                <div className={`p-2 rounded-lg ${tx.type === 'deposit' ? 'bg-green-500/20 text-green-400' : 'bg-red-500/20 text-red-400'}`}>
                  {tx.type === 'deposit' ? <ArrowDownLeft className="w-4 h-4" /> : <ArrowUpRight className="w-4 h-4" />}
                </div>
                <div>
                  <p className="text-white font-medium capitalize">{tx.type}</p>
                  <p className="text-gray-400 text-xs">{tx.time}</p>
                </div>
              </div>
              <div className="text-right">
                <p className="text-white font-medium">{tx.amount} {tx.token}</p>
                <span className="text-green-400 text-xs">{tx.status}</span>
              </div>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
