/**
 * TigerSwap Complete Admin Dashboard
 * Full-featured admin panel with all management capabilities
 * 
 * Features:
 * - Platform-wide bot management (all users' bots)
 * - Fee configuration (all fees to admin addresses)
 * - External CEX/DEX connection management
 * - Blockchain management (EVM + Non-EVM)
 * - Token listing management with fees
 * - API key management for external users
 * - Complete analytics
 * - User management
 */

import React, { useState, useEffect } from 'react';
import { LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, Legend, ResponsiveContainer, BarChart, Bar, PieChart, Pie, Cell } from 'recharts';

// ============================================================================
// TYPES
// ============================================================================

type UserRole = 'super_admin' | 'dex_admin' | 'cex_admin' | 'finance_admin' | 'client';

interface AdminUser {
  id: string;
  username: string;
  email: string;
  role: UserRole;
  isActive: boolean;
  permissions: string[];
  lastLogin: Date;
}

interface BotInstance {
  id: string;
  userId: string;
  userEmail: string;
  botType: string;
  name: string;
  status: 'running' | 'stopped' | 'error' | 'paused';
  connectedDEXs: number;
  connectedCEXs: number;
  totalPnL: number;
  totalVolume: number;
  totalOrders: number;
  avgLatencyUs: number;
  createdAt: Date;
  lastTradeAt: Date;
}

interface BotTier {
  id: string;
  name: string;
  displayName: string;
  monthlyFeeUsd: number;
  perDexFeeUsd: number;
  perCexFeeUsd: number;
  maxBots: number;
  maxDEXs: number;
  maxCEXs: number;
  maxPositionUsd: number;
  maxDailyVolume: number;
  latencyTargetMs: number;
  isActive: boolean;
}

interface FeeConfig {
  id: string;
  feeType: string;
  chainId?: number;
  tokenSymbol?: string;
  feeAmountUsd: number;
  feePercentage: number;
  minFeeUsd: number;
  maxFeeUsd?: number;
  isActive: boolean;
}

interface AdminFeeAddress {
  id: string;
  feeType: string;
  chainId: number;
  walletAddress: string;
  tokenSymbol?: string;
  isActive: boolean;
  priority: number;
}

interface ExternalCEXConnection {
  id: string;
  userId: string;
  userEmail: string;
  exchangeName: string;
  accountId: string;
  isActive: boolean;
  canTrade: boolean;
  canWithdraw: boolean;
  canDeposit: boolean;
  lastSyncAt: Date;
  syncStatus: 'idle' | 'syncing' | 'error';
}

interface ExternalDEXConnection {
  id: string;
  userId: string;
  userEmail: string;
  dexName: string;
  chainId: number;
  chainName: string;
  walletAddress: string;
  isActive: boolean;
  maxSlippageBps: number;
  lastTxHash?: string;
  lastTxAt: Date;
}

interface Blockchain {
  id: string;
  name: string;
  symbol: string;
  chainId: number;
  chainIdHex?: string;
  isEVM: boolean;
  isActive: boolean;
  explorerUrl?: string;
  rpcUrl?: string;
  nativeTokenSymbol: string;
  avgGasPriceGwei: number;
}

interface TokenListing {
  id: string;
  tokenSymbol: string;
  tokenName: string;
  contractAddress: string;
  chainId: number;
  tier: 'basic' | 'standard' | 'premium' | 'premium_plus';
  status: 'pending' | 'approved' | 'rejected';
  requesterAddress: string;
  requesterEmail: string;
  oneTimeFee: number;
  monthlyFee: number;
  requestedAt: Date;
}

interface APIKey {
  id: string;
  userId: string;
  userEmail: string;
  keyName: string;
  tier: 'free' | 'basic' | 'pro' | 'enterprise';
  permissions: { trading: boolean; reading: boolean; withdrawal: boolean };
  rateLimitPerMinute: number;
  rateLimitPerDay: number;
  isActive: boolean;
  lastUsedAt?: Date;
  expiresAt: Date;
  createdAt: Date;
}

interface PlatformStats {
  totalUsers: number;
  totalBots: number;
  activeBots: number;
  totalVolume: number;
  totalPnL: number;
  totalFeesCollected: number;
  activeCEXConnections: number;
  activeDEXConnections: number;
}

// ============================================================================
// COLORS
// ============================================================================

const COLORS = ['#0088FE', '#00C49F', '#FFBB28', '#FF8042', '#8884D8', '#82CA9D'];

// ============================================================================
// MAIN COMPONENT
// ============================================================================

export default function CompleteAdminDashboard() {
  const [currentTab, setCurrentTab] = useState('overview');
  const [isLoading, setIsLoading] = useState(false);
  const [adminUser, setAdminUser] = useState<AdminUser>({
    id: 'admin_1',
    username: 'super_admin',
    email: 'admin@tigerswap.io',
    role: 'super_admin',
    isActive: true,
    permissions: ['all'],
    lastLogin: new Date(),
  });

  // State for all data
  const [platformStats, setPlatformStats] = useState<PlatformStats>({
    totalUsers: 1250,
    totalBots: 3420,
    activeBots: 890,
    totalVolume: 125000000,
    totalPnL: 5200000,
    totalFeesCollected: 850000,
    activeCEXConnections: 2100,
    activeDEXConnections: 450,
  });

  const [botTiers, setBotTiers] = useState<BotTier[]>([
    { id: 'tier_1', name: 'tier_1', displayName: 'Basic', monthlyFeeUsd: 2500, perDexFeeUsd: 500, perCexFeeUsd: 50, maxBots: 1, maxDEXs: 5, maxCEXs: 20, maxPositionUsd: 100000, maxDailyVolume: 1000000, latencyTargetMs: 100, isActive: true },
    { id: 'tier_2', name: 'tier_2', displayName: 'Pro', monthlyFeeUsd: 5000, perDexFeeUsd: 750, perCexFeeUsd: 75, maxBots: 3, maxDEXs: 10, maxCEXs: 50, maxPositionUsd: 500000, maxDailyVolume: 5000000, latencyTargetMs: 50, isActive: true },
    { id: 'tier_3', name: 'tier_3', displayName: 'Enterprise', monthlyFeeUsd: 10000, perDexFeeUsd: 1000, perCexFeeUsd: 100, maxBots: 10, maxDEXs: 20, maxCEXs: 200, maxPositionUsd: 5000000, maxDailyVolume: 50000000, latencyTargetMs: 10, isActive: true },
  ]);

  const [feeConfigs, setFeeConfigs] = useState<FeeConfig[]>([
    { id: 'swap', feeType: 'swap', feeAmountUsd: 0.3, feePercentage: 0, minFeeUsd: 0, isActive: true },
    { id: 'liquidity', feeType: 'liquidity', feeAmountUsd: 0.25, feePercentage: 0, minFeeUsd: 0, isActive: true },
    { id: 'withdrawal', feeType: 'withdrawal', feeAmountUsd: 5, feePercentage: 0, minFeeUsd: 5, maxFeeUsd: 50, isActive: true },
    { id: 'bot_subscription', feeType: 'bot_subscription', feeAmountUsd: 0, feePercentage: 0, isActive: true },
    { id: 'api_key', feeType: 'api_key', feeAmountUsd: 99, feePercentage: 0, isActive: true },
    { id: 'listing_basic', feeType: 'listing', tokenSymbol: 'BASIC', feeAmountUsd: 5000, feePercentage: 0, isActive: true },
    { id: 'listing_premium', feeType: 'listing', tokenSymbol: 'PREMIUM', feeAmountUsd: 15000, feePercentage: 0, isActive: true },
  ]);

  const [adminFeeAddresses, setAdminFeeAddresses] = useState<AdminFeeAddress[]>([
    { id: 'afa_1', feeType: 'swap', chainId: 1, walletAddress: '0xAdminFeeAddress1...', tokenSymbol: 'ETH', isActive: true, priority: 1 },
    { id: 'afa_2', feeType: 'swap', chainId: 56, walletAddress: '0xAdminFeeAddress2...', tokenSymbol: 'BNB', isActive: true, priority: 1 },
    { id: 'afa_3', feeType: 'bot_subscription', chainId: 1, walletAddress: '0xBotFeeAddress...', tokenSymbol: 'USDT', isActive: true, priority: 1 },
  ]);

  const [blockchains, setBlockchains] = useState<Blockchain[]>([
    { id: 'eth_1', name: 'Ethereum', symbol: 'ETH', chainId: 1, chainIdHex: '0x1', isEVM: true, isActive: true, explorerUrl: 'https://etherscan.io', rpcUrl: 'https://eth-mainnet.alchemyapi.io', nativeTokenSymbol: 'ETH', avgGasPriceGwei: 20 },
    { id: 'bsc_56', name: 'BNB Smart Chain', symbol: 'BNB', chainId: 56, chainIdHex: '0x38', isEVM: true, isActive: true, explorerUrl: 'https://bscscan.com', nativeTokenSymbol: 'BNB', avgGasPriceGwei: 3 },
    { id: 'arb_42161', name: 'Arbitrum One', symbol: 'ETH', chainId: 42161, chainIdHex: '0xa4b1', isEVM: true, isActive: true, explorerUrl: 'https://arbiscan.io', nativeTokenSymbol: 'ETH', avgGasPriceGwei: 0.1 },
    { id: 'sol_101', name: 'Solana', symbol: 'SOL', chainId: 101, isEVM: false, isActive: true, explorerUrl: 'https://solscan.io', nativeTokenSymbol: 'SOL', avgGasPriceGwei: 0 },
    { id: 'aptos_101', name: 'Aptos', symbol: 'APT', chainId: 101, isEVM: false, isActive: true, explorerUrl: 'https://explorer.aptoslabs.com', nativeTokenSymbol: 'APT', avgGasPriceGwei: 0 },
  ]);

  // Sample data for display
  const [recentBots, setRecentBots] = useState<BotInstance[]>([
    { id: 'bot_1', userId: 'user_1', userEmail: 'user1@example.com', botType: 'MarketMaker', name: 'ETH Market Maker', status: 'running', connectedDEXS: 20, connectedCEXs: 200, totalPnL: 15200, totalVolume: 5200000, totalOrders: 15420, avgLatencyUs: 5, createdAt: new Date(), lastTradeAt: new Date() },
    { id: 'bot_2', userId: 'user_2', userEmail: 'user2@example.com', botType: 'Arbitrage', name: 'BTC Arbitrage', status: 'running', connectedDEXS: 20, connectedCEXs: 200, totalPnL: 28500, totalVolume: 8900000, totalOrders: 42300, avgLatencyUs: 8, createdAt: new Date(), lastTradeAt: new Date() },
    { id: 'bot_3', userId: 'user_3', userEmail: 'user3@example.com', botType: 'Sniper', name: 'SOL Sniper', status: 'stopped', connectedDEXS: 10, connectedCEXs: 50, totalPnL: 5200, totalVolume: 1200000, totalOrders: 3200, avgLatencyUs: 3, createdAt: new Date(), lastTradeAt: new Date() },
  ]);

  const [cexConnections, setCexConnections] = useState<ExternalCEXConnection[]>([
    { id: 'cex_1', userId: 'user_1', userEmail: 'user1@example.com', exchangeName: 'Binance', accountId: 'binance_123', isActive: true, canTrade: true, canWithdraw: false, canDeposit: true, lastSyncAt: new Date(), syncStatus: 'idle' },
    { id: 'cex_2', userId: 'user_2', userEmail: 'user2@example.com', exchangeName: 'Coinbase', accountId: 'cb_456', isActive: true, canTrade: true, canWithdraw: true, canDeposit: true, lastSyncAt: new Date(), syncStatus: 'idle' },
  ]);

  const [dexConnections, setDexConnections] = useState<ExternalDEXConnection[]>([
    { id: 'dex_1', userId: 'user_1', userEmail: 'user1@example.com', dexName: 'Uniswap', chainId: 1, chainName: 'Ethereum', walletAddress: '0xWallet1...', isActive: true, maxSlippageBps: 300, lastTxAt: new Date() },
    { id: 'dex_2', userId: 'user_2', userEmail: 'user2@example.com', dexName: 'PancakeSwap', chainId: 56, chainName: 'BNB Chain', walletAddress: '0xWallet2...', isActive: true, maxSlippageBps: 500, lastTxAt: new Date() },
  ]);

  const [tokenListings, setTokenListings] = useState<TokenListing[]>([
    { id: 'listing_1', tokenSymbol: 'TIGER', tokenName: 'Tiger Token', contractAddress: '0x123...', chainId: 1, tier: 'premium', status: 'pending', requesterAddress: '0xReq1...', requesterEmail: 'requester1@example.com', oneTimeFee: 15000, monthlyFee: 5000, requestedAt: new Date() },
  ]);

  const [apiKeys, setApiKeys] = useState<APIKey[]>([
    { id: 'key_1', userId: 'user_1', userEmail: 'user1@example.com', keyName: 'Production API', tier: 'pro', permissions: { trading: true, reading: true, withdrawal: false }, rateLimitPerMinute: 600, rateLimitPerDay: 100000, isActive: true, expiresAt: new Date(), createdAt: new Date() },
  ]);

  // Chart data
  const volumeData = [
    { date: '2026-05-01', volume: 4200000 },
    { date: '2026-05-02', volume: 4800000 },
    { date: '2026-05-03', volume: 3900000 },
    { date: '2026-05-04', volume: 5100000 },
    { date: '2026-05-05', volume: 5500000 },
    { date: '2026-05-06', volume: 6200000 },
  ];

  const feeBreakdown = [
    { name: 'Swap Fees', value: 450000 },
    { name: 'Bot Subscriptions', value: 280000 },
    { name: 'API Keys', value: 85000 },
    { name: 'Listings', value: 35000 },
  ];

  // ============================================================================
  // RENDER HELPERS
  // ============================================================================

  const formatCurrency = (value: number): string => {
    return new Intl.NumberFormat('en-US', { style: 'currency', currency: 'USD' }).format(value);
  };

  const formatNumber = (value: number): string => {
    return new Intl.NumberFormat('en-US').format(value);
  };

  // ============================================================================
  // TAB RENDERERS
  // ============================================================================

  const renderOverview = () => (
    <div className="space-y-6">
      {/* Stats Cards */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        <div className="bg-gray-800 rounded-lg p-6 border border-gray-700">
          <div className="text-sm text-gray-400">Total Users</div>
          <div className="text-3xl font-bold text-white mt-2">{formatNumber(platformStats.totalUsers)}</div>
        </div>
        <div className="bg-gray-800 rounded-lg p-6 border border-gray-700">
          <div className="text-sm text-gray-400">Total Bots</div>
          <div className="text-3xl font-bold text-white mt-2">{formatNumber(platformStats.totalBots)}</div>
        </div>
        <div className="bg-gray-800 rounded-lg p-6 border border-gray-700">
          <div className="text-sm text-gray-400">Active Bots</div>
          <div className="text-3xl font-bold text-green-400 mt-2">{formatNumber(platformStats.activeBots)}</div>
        </div>
        <div className="bg-gray-800 rounded-lg p-6 border border-gray-700">
          <div className="text-sm text-gray-400">Total Volume</div>
          <div className="text-3xl font-bold text-blue-400 mt-2">{formatCurrency(platformStats.totalVolume)}</div>
        </div>
      </div>

      {/* Charts */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <div className="bg-gray-800 rounded-lg p-6 border border-gray-700">
          <h3 className="text-lg font-semibold text-white mb-4">Volume Trend</h3>
          <ResponsiveContainer width="100%" height={300}>
            <LineChart data={volumeData}>
              <CartesianGrid strokeDasharray="3 3" stroke="#374151" />
              <XAxis dataKey="date" stroke="#9CA3AF" />
              <YAxis stroke="#9CA3AF" />
              <Tooltip contentStyle={{ backgroundColor: '#1F2937', border: 'none' }} />
              <Line type="monotone" dataKey="volume" stroke="#3B82F6" strokeWidth={2} />
            </LineChart>
          </ResponsiveContainer>
        </div>

        <div className="bg-gray-800 rounded-lg p-6 border border-gray-700">
          <h3 className="text-lg font-semibold text-white mb-4">Fee Breakdown</h3>
          <ResponsiveContainer width="100%" height={300}>
            <PieChart>
              <Pie data={feeBreakdown} dataKey="value" nameKey="name" cx="50%" cy="50%" outerRadius={100} label>
                {feeBreakdown.map((_, index) => (
                  <Cell key={`cell-${index}`} fill={COLORS[index % COLORS.length]} />
                ))}
              </Pie>
              <Tooltip contentStyle={{ backgroundColor: '#1F2937', border: 'none' }} />
              <Legend />
            </PieChart>
          </ResponsiveContainer>
        </div>
      </div>

      {/* Platform Health */}
      <div className="bg-gray-800 rounded-lg p-6 border border-gray-700">
        <h3 className="text-lg font-semibold text-white mb-4">Platform Health</h3>
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
          <div className="text-center">
            <div className="text-2xl font-bold text-green-400">{platformStats.activeCEXConnections}</div>
            <div className="text-sm text-gray-400">Active CEX Connections</div>
          </div>
          <div className="text-center">
            <div className="text-2xl font-bold text-green-400">{platformStats.activeDEXConnections}</div>
            <div className="text-sm text-gray-400">Active DEX Connections</div>
          </div>
          <div className="text-center">
            <div className="text-2xl font-bold text-blue-400">{formatCurrency(platformStats.totalFeesCollected)}</div>
            <div className="text-sm text-gray-400">Total Fees Collected</div>
          </div>
          <div className="text-center">
            <div className="text-2xl font-bold text-yellow-400">{formatCurrency(platformStats.totalPnL)}</div>
            <div className="text-sm text-gray-400">Total Bot PnL</div>
          </div>
        </div>
      </div>
    </div>
  );

  const renderBotManagement = () => (
    <div className="space-y-6">
      <div className="bg-gray-800 rounded-lg p-6 border border-gray-700">
        <h3 className="text-lg font-semibold text-white mb-4">All Platform Bots (Admin Can Manage All)</h3>
        <div className="overflow-x-auto">
          <table className="w-full">
            <thead>
              <tr className="border-b border-gray-700">
                <th className="text-left py-3 px-4 text-gray-400">Bot</th>
                <th className="text-left py-3 px-4 text-gray-400">User</th>
                <th className="text-left py-3 px-4 text-gray-400">Type</th>
                <th className="text-left py-3 px-4 text-gray-400">Status</th>
                <th className="text-left py-3 px-4 text-gray-400">Connections</th>
                <th className="text-left py-3 px-4 text-gray-400">PnL</th>
                <th className="text-left py-3 px-4 text-gray-400">Volume</th>
                <th className="text-left py-3 px-4 text-gray-400">Actions</th>
              </tr>
            </thead>
            <tbody>
              {recentBots.map((bot) => (
                <tr key={bot.id} className="border-b border-gray-700">
                  <td className="py-3 px-4 text-white">{bot.name}</td>
                  <td className="py-3 px-4 text-gray-300">{bot.userEmail}</td>
                  <td className="py-3 px-4 text-gray-300">{bot.botType}</td>
                  <td className="py-3 px-4">
                    <span className={`px-2 py-1 rounded text-sm ${bot.status === 'running' ? 'bg-green-900 text-green-300' : 'bg-gray-700 text-gray-300'}`}>
                      {bot.status}
                    </span>
                  </td>
                  <td className="py-3 px-4 text-gray-300">DEX: {bot.connectedDEXs} / CEX: {bot.connectedCEXs}</td>
                  <td className="py-3 px-4 text-green-400">{formatCurrency(bot.totalPnL)}</td>
                  <td className="py-3 px-4 text-gray-300">{formatCurrency(bot.totalVolume)}</td>
                  <td className="py-3 px-4">
                    <button className="text-blue-400 hover:text-blue-300 mr-3">Manage</button>
                    <button className="text-red-400 hover:text-red-300">Stop</button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      {/* Bot Tiers */}
      <div className="bg-gray-800 rounded-lg p-6 border border-gray-700">
        <h3 className="text-lg font-semibold text-white mb-4">Bot Subscription Tiers</h3>
        <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
          {botTiers.map((tier) => (
            <div key={tier.id} className="bg-gray-700 rounded-lg p-4">
              <div className="flex justify-between items-center mb-2">
                <span className="text-lg font-semibold text-white">{tier.displayName}</span>
                <span className="text-green-400 font-bold">{formatCurrency(tier.monthlyFeeUsd)}/mo</span>
              </div>
              <div className="space-y-1 text-sm text-gray-400">
                <div>Max Bots: {tier.maxBots}</div>
                <div>Max DEXs: {tier.maxDEXs}</div>
                <div>Max CEXs: {tier.maxCEXs}</div>
                <div>Max Position: {formatCurrency(tier.maxPositionUsd)}</div>
                <div>Latency Target: {tier.latencyTargetMs}ms</div>
                <div>Per DEX: {formatCurrency(tier.perDexFeeUsd)}</div>
                <div>Per CEX: {formatCurrency(tier.perCexFeeUsd)}</div>
              </div>
            </div>
          ))}
        </div>
      </div>
    </div>
  );

  const renderFeeManagement = () => (
    <div className="space-y-6">
      {/* Admin Fee Addresses - ALL FEES GO HERE */}
      <div className="bg-gray-800 rounded-lg p-6 border border-gray-700">
        <h3 className="text-lg font-semibold text-white mb-4">Admin Fee Addresses (All Fees Collected Here)</h3>
        <div className="overflow-x-auto">
          <table className="w-full">
            <thead>
              <tr className="border-b border-gray-700">
                <th className="text-left py-3 px-4 text-gray-400">Fee Type</th>
                <th className="text-left py-3 px-4 text-gray-400">Chain</th>
                <th className="text-left py-3 px-4 text-gray-400">Wallet Address</th>
                <th className="text-left py-3 px-4 text-gray-400">Token</th>
                <th className="text-left py-3 px-4 text-gray-400">Priority</th>
                <th className="text-left py-3 px-4 text-gray-400">Status</th>
                <th className="text-left py-3 px-4 text-gray-400">Actions</th>
              </tr>
            </thead>
            <tbody>
              {adminFeeAddresses.map((addr) => (
                <tr key={addr.id} className="border-b border-gray-700">
                  <td className="py-3 px-4 text-white">{addr.feeType}</td>
                  <td className="py-3 px-4 text-gray-300">{addr.chainId}</td>
                  <td className="py-3 px-4 text-gray-300 font-mono">{addr.walletAddress}</td>
                  <td className="py-3 px-4 text-gray-300">{addr.tokenSymbol}</td>
                  <td className="py-3 px-4 text-gray-300">{addr.priority}</td>
                  <td className="py-3 px-4">
                    <span className="px-2 py-1 rounded text-sm bg-green-900 text-green-300">Active</span>
                  </td>
                  <td className="py-3 px-4">
                    <button className="text-blue-400 hover:text-blue-300 mr-3">Edit</button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      {/* Fee Configurations */}
      <div className="bg-gray-800 rounded-lg p-6 border border-gray-700">
        <h3 className="text-lg font-semibold text-white mb-4">Fee Configurations</h3>
        <div className="overflow-x-auto">
          <table className="w-full">
            <thead>
              <tr className="border-b border-gray-700">
                <th className="text-left py-3 px-4 text-gray-400">Fee Type</th>
                <th className="text-left py-3 px-4 text-gray-400">Amount (USD)</th>
                <th className="text-left py-3 px-4 text-gray-400">Percentage</th>
                <th className="text-left py-3 px-4 text-gray-400">Min Fee</th>
                <th className="text-left py-3 px-4 text-gray-400">Max Fee</th>
                <th className="text-left py-3 px-4 text-gray-400">Status</th>
                <th className="text-left py-3 px-4 text-gray-400">Actions</th>
              </tr>
            </thead>
            <tbody>
              {feeConfigs.map((config) => (
                <tr key={config.id} className="border-b border-gray-700">
                  <td className="py-3 px-4 text-white">{config.feeType}</td>
                  <td className="py-3 px-4 text-gray-300">{formatCurrency(config.feeAmountUsd)}</td>
                  <td className="py-3 px-4 text-gray-300">{config.feePercentage}%</td>
                  <td className="py-3 px-4 text-gray-300">{formatCurrency(config.minFeeUsd)}</td>
                  <td className="py-3 px-4 text-gray-300">{config.maxFeeUsd ? formatCurrency(config.maxFeeUsd) : '-'}</td>
                  <td className="py-3 px-4">
                    <span className={`px-2 py-1 rounded text-sm ${config.isActive ? 'bg-green-900 text-green-300' : 'bg-gray-700 text-gray-300'}`}>
                      {config.isActive ? 'Active' : 'Inactive'}
                    </span>
                  </td>
                  <td className="py-3 px-4">
                    <button className="text-blue-400 hover:text-blue-300">Edit</button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      {/* Total Fees Collected */}
      <div className="bg-green-900 rounded-lg p-6 border border-green-700">
        <div className="text-center">
          <div className="text-4xl font-bold text-white">{formatCurrency(platformStats.totalFeesCollected)}</div>
          <div className="text-green-300 mt-2">Total Fees Collected to Admin Addresses</div>
        </div>
      </div>
    </div>
  );

  const renderExternalConnections = () => (
    <div className="space-y-6">
      {/* CEX Connections */}
      <div className="bg-gray-800 rounded-lg p-6 border border-gray-700">
        <h3 className="text-lg font-semibold text-white mb-4">User CEX Connections (Connect Their Own Exchanges)</h3>
        <div className="overflow-x-auto">
          <table className="w-full">
            <thead>
              <tr className="border-b border-gray-700">
                <th className="text-left py-3 px-4 text-gray-400">User</th>
                <th className="text-left py-3 px-4 text-gray-400">Exchange</th>
                <th className="text-left py-3 px-4 text-gray-400">Account ID</th>
                <th className="text-left py-3 px-4 text-gray-400">Can Trade</th>
                <th className="text-left py-3 px-4 text-gray-400">Can Withdraw</th>
                <th className="text-left py-3 px-4 text-gray-400">Status</th>
                <th className="text-left py-3 px-4 text-gray-400">Actions</th>
              </tr>
            </thead>
            <tbody>
              {cexConnections.map((conn) => (
                <tr key={conn.id} className="border-b border-gray-700">
                  <td className="py-3 px-4 text-gray-300">{conn.userEmail}</td>
                  <td className="py-3 px-4 text-white">{conn.exchangeName}</td>
                  <td className="py-3 px-4 text-gray-300">{conn.accountId}</td>
                  <td className="py-3 px-4 text-gray-300">{conn.canTrade ? '✓' : '✗'}</td>
                  <td className="py-3 px-4 text-gray-300">{conn.canWithdraw ? '✓' : '✗'}</td>
                  <td className="py-3 px-4">
                    <span className={`px-2 py-1 rounded text-sm ${conn.syncStatus === 'idle' ? 'bg-green-900 text-green-300' : 'bg-yellow-900 text-yellow-300'}`}>
                      {conn.syncStatus}
                    </span>
                  </td>
                  <td className="py-3 px-4">
                    <button className="text-blue-400 hover:text-blue-300 mr-3">Manage</button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      {/* DEX Connections */}
      <div className="bg-gray-800 rounded-lg p-6 border border-gray-700">
        <h3 className="text-lg font-semibold text-white mb-4">User DEX Connections (Connect Their Own Wallets)</h3>
        <div className="overflow-x-auto">
          <table className="w-full">
            <thead>
              <tr className="border-b border-gray-700">
                <th className="text-left py-3 px-4 text-gray-400">User</th>
                <th className="text-left py-3 px-4 text-gray-400">DEX</th>
                <th className="text-left py-3 px-4 text-gray-400">Chain</th>
                <th className="text-left py-3 px-4 text-gray-400">Wallet</th>
                <th className="text-left py-3 px-4 text-gray-400">Max Slippage</th>
                <th className="text-left py-3 px-4 text-gray-400">Last TX</th>
                <th className="text-left py-3 px-4 text-gray-400">Actions</th>
              </tr>
            </thead>
            <tbody>
              {dexConnections.map((conn) => (
                <tr key={conn.id} className="border-b border-gray-700">
                  <td className="py-3 px-4 text-gray-300">{conn.userEmail}</td>
                  <td className="py-3 px-4 text-white">{conn.dexName}</td>
                  <td className="py-3 px-4 text-gray-300">{conn.chainName}</td>
                  <td className="py-3 px-4 text-gray-300 font-mono">{conn.walletAddress}</td>
                  <td className="py-3 px-4 text-gray-300">{conn.maxSlippageBps} bps</td>
                  <td className="py-3 px-4 text-gray-300">{conn.lastTxAt.toLocaleDateString()}</td>
                  <td className="py-3 px-4">
                    <button className="text-blue-400 hover:text-blue-300 mr-3">Manage</button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );

  const renderBlockchainManagement = () => (
    <div className="space-y-6">
      <div className="bg-gray-800 rounded-lg p-6 border border-gray-700">
        <h3 className="text-lg font-semibold text-white mb-4">Blockchain Management (EVM + Non-EVM)</h3>
        <div className="overflow-x-auto">
          <table className="w-full">
            <thead>
              <tr className="border-b border-gray-700">
                <th className="text-left py-3 px-4 text-gray-400">Chain</th>
                <th className="text-left py-3 px-4 text-gray-400">Chain ID</th>
                <th className="text-left py-3 px-4 text-gray-400">Type</th>
                <th className="text-left py-3 px-4 text-gray-400">Native Token</th>
                <th className="text-left py-3 px-4 text-gray-400">Avg Gas (Gwei)</th>
                <th className="text-left py-3 px-4 text-gray-400">Explorer</th>
                <th className="text-left py-3 px-4 text-gray-400">Status</th>
                <th className="text-left py-3 px-4 text-gray-400">Actions</th>
              </tr>
            </thead>
            <tbody>
              {blockchains.map((chain) => (
                <tr key={chain.id} className="border-b border-gray-700">
                  <td className="py-3 px-4 text-white">{chain.name}</td>
                  <td className="py-3 px-4 text-gray-300">{chain.chainId} ({chain.chainIdHex})</td>
                  <td className="py-3 px-4 text-gray-300">{chain.isEVM ? 'EVM' : 'Non-EVM'}</td>
                  <td className="py-3 px-4 text-gray-300">{chain.nativeTokenSymbol}</td>
                  <td className="py-3 px-4 text-gray-300">{chain.avgGasPriceGwei}</td>
                  <td className="py-3 px-4 text-blue-400">{chain.explorerUrl}</td>
                  <td className="py-3 px-4">
                    <span className={`px-2 py-1 rounded text-sm ${chain.isActive ? 'bg-green-900 text-green-300' : 'bg-gray-700 text-gray-300'}`}>
                      {chain.isActive ? 'Active' : 'Inactive'}
                    </span>
                  </td>
                  <td className="py-3 px-4">
                    <button className="text-blue-400 hover:text-blue-300 mr-3">Edit</button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );

  const renderTokenListings = () => (
    <div className="space-y-6">
      <div className="bg-gray-800 rounded-lg p-6 border border-gray-700">
        <h3 className="text-lg font-semibold text-white mb-4">Token Listing Requests</h3>
        <div className="overflow-x-auto">
          <table className="w-full">
            <thead>
              <tr className="border-b border-gray-700">
                <th className="text-left py-3 px-4 text-gray-400">Token</th>
                <th className="text-left py-3 px-4 text-gray-400">Requester</th>
                <th className="text-left py-3 px-4 text-gray-400">Tier</th>
                <th className="text-left py-3 px-4 text-gray-400">One-Time Fee</th>
                <th className="text-left py-3 px-4 text-gray-400">Monthly Fee</th>
                <th className="text-left py-3 px-4 text-gray-400">Status</th>
                <th className="text-left py-3 px-4 text-gray-400">Actions</th>
              </tr>
            </thead>
            <tbody>
              {tokenListings.map((listing) => (
                <tr key={listing.id} className="border-b border-gray-700">
                  <td className="py-3 px-4 text-white">{listing.tokenName} ({listing.tokenSymbol})</td>
                  <td className="py-3 px-4 text-gray-300">{listing.requesterEmail}</td>
                  <td className="py-3 px-4 text-gray-300">{listing.tier}</td>
                  <td className="py-3 px-4 text-gray-300">{formatCurrency(listing.oneTimeFee)}</td>
                  <td className="py-3 px-4 text-gray-300">{formatCurrency(listing.monthlyFee)}</td>
                  <td className="py-3 px-4">
                    <span className={`px-2 py-1 rounded text-sm ${listing.status === 'approved' ? 'bg-green-900 text-green-300' : listing.status === 'rejected' ? 'bg-red-900 text-red-300' : 'bg-yellow-900 text-yellow-300'}`}>
                      {listing.status}
                    </span>
                  </td>
                  <td className="py-3 px-4">
                    {listing.status === 'pending' && (
                      <>
                        <button className="text-green-400 hover:text-green-300 mr-3">Approve</button>
                        <button className="text-red-400 hover:text-red-300">Reject</button>
                      </>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );

  const renderAPIManagement = () => (
    <div className="space-y-6">
      <div className="bg-gray-800 rounded-lg p-6 border border-gray-700">
        <h3 className="text-lg font-semibold text-white mb-4">External User API Keys</h3>
        <div className="overflow-x-auto">
          <table className="w-full">
            <thead>
              <tr className="border-b border-gray-700">
                <th className="text-left py-3 px-4 text-gray-400">User</th>
                <th className="text-left py-3 px-4 text-gray-400">Key Name</th>
                <th className="text-left py-3 px-4 text-gray-400">Tier</th>
                <th className="text-left py-3 px-4 text-gray-400">Permissions</th>
                <th className="text-left py-3 px-4 text-gray-400">Rate Limits</th>
                <th className="text-left py-3 px-4 text-gray-400">Status</th>
                <th className="text-left py-3 px-4 text-gray-400">Actions</th>
              </tr>
            </thead>
            <tbody>
              {apiKeys.map((key) => (
                <tr key={key.id} className="border-b border-gray-700">
                  <td className="py-3 px-4 text-gray-300">{key.userEmail}</td>
                  <td className="py-3 px-4 text-white">{key.keyName}</td>
                  <td className="py-3 px-4 text-gray-300">{key.tier}</td>
                  <td className="py-3 px-4 text-gray-300">
                    Trading: {key.permissions.trading ? '✓' : '✗'}, 
                    Reading: {key.permissions.reading ? '✓' : '✗'}, 
                    Withdrawal: {key.permissions.withdrawal ? '✓' : '✗'}
                  </td>
                  <td className="py-3 px-4 text-gray-300">{key.rateLimitPerMinute}/min, {key.rateLimitPerDay}/day</td>
                  <td className="py-3 px-4">
                    <span className={`px-2 py-1 rounded text-sm ${key.isActive ? 'bg-green-900 text-green-300' : 'bg-gray-700 text-gray-300'}`}>
                      {key.isActive ? 'Active' : 'Inactive'}
                    </span>
                  </td>
                  <td className="py-3 px-4">
                    <button className="text-blue-400 hover:text-blue-300 mr-3">Revoke</button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );

  // ============================================================================
  // MAIN RENDER
  // ============================================================================

  return (
    <div className="min-h-screen bg-gray-900 text-white">
      {/* Header */}
      <header className="bg-gray-800 border-b border-gray-700">
        <div className="max-w-7xl mx-auto px-4 py-4">
          <div className="flex justify-between items-center">
            <div className="flex items-center space-x-4">
              <h1 className="text-2xl font-bold text-orange-500">TigerSwap Admin</h1>
              <span className="px-2 py-1 bg-orange-900 text-orange-300 text-sm rounded">Super Admin</span>
            </div>
            <div className="flex items-center space-x-4">
              <span className="text-gray-400">{adminUser.email}</span>
              <button className="text-gray-400 hover:text-white">Logout</button>
            </div>
          </div>
        </div>
      </header>

      {/* Navigation */}
      <nav className="bg-gray-800 border-b border-gray-700">
        <div className="max-w-7xl mx-auto px-4">
          <div className="flex space-x-8">
            {[
              { id: 'overview', label: 'Overview' },
              { id: 'bots', label: 'Bot Management' },
              { id: 'fees', label: 'Fee Management' },
              { id: 'connections', label: 'External Connections' },
              { id: 'blockchains', label: 'Blockchains' },
              { id: 'listings', label: 'Token Listings' },
              { id: 'api', label: 'API Keys' },
            ].map((tab) => (
              <button
                key={tab.id}
                onClick={() => setCurrentTab(tab.id)}
                className={`py-4 px-2 border-b-2 ${
                  currentTab === tab.id
                    ? 'border-orange-500 text-orange-500'
                    : 'border-transparent text-gray-400 hover:text-white'
                }`}
              >
                {tab.label}
              </button>
            ))}
          </div>
        </div>
      </nav>

      {/* Content */}
      <main className="max-w-7xl mx-auto px-4 py-8">
        {currentTab === 'overview' && renderOverview()}
        {currentTab === 'bots' && renderBotManagement()}
        {currentTab === 'fees' && renderFeeManagement()}
        {currentTab === 'connections' && renderExternalConnections()}
        {currentTab === 'blockchains' && renderBlockchainManagement()}
        {currentTab === 'listings' && renderTokenListings()}
        {currentTab === 'api' && renderAPIManagement()}
      </main>
    </div>
  );
}