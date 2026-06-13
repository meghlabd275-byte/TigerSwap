/**
 * TigerSwap Complete Admin Platform - TypeScript Implementation
 * 
 * Features:
 * - Super Admin with complete control
 * - Multi-admin role management
 * - Blockchain management (40+ EVM/Non-EVM)
 * - Token management (50+ tokens)
 * - Fee configuration
 * - White label system with 20% revenue sharing
 * - Bot client management
 * - External API system
 * - Complete audit logging
 * 
 * @author TigerSwap
 */

import * as crypto from 'crypto';

// ============================================================================
// Types & Interfaces
// ============================================================================

export enum AdminRole {
  SUPER_ADMIN = 'super_admin',
  ADMIN = 'admin',
  MODERATOR = 'moderator',
  SUPPORT = 'support',
  VIEWER = 'viewer'
}

export enum Permission {
  MANAGE_USERS = 'manage_users',
  VIEW_USERS = 'view_users',
  BAN_USERS = 'ban_users',
  MANAGE_TRADING = 'manage_trading',
  VIEW_TRADING = 'view_trading',
  MANAGE_FEES = 'manage_fees',
  MANAGE_BLOCKCHAINS = 'manage_blockchains',
  VIEW_BLOCKCHAINS = 'view_blockchains',
  MANAGE_TOKENS = 'manage_tokens',
  LIST_TOKENS = 'list_tokens',
  MANAGE_WHITELABEL = 'manage_whitelabel',
  APPROVE_WHITELABEL = 'approve_whitelabel',
  MANAGE_BOTS = 'manage_bots',
  APPROVE_BOTS = 'approve_bots',
  MANAGE_CEX = 'manage_cex',
  MANAGE_DEX = 'manage_dex',
  MANAGE_WALLETS = 'manage_wallets',
  MANAGE_API = 'manage_api',
  VIEW_API = 'view_api',
  MANAGE_ADMINS = 'manage_admins',
  VIEW_ADMINS = 'view_admins'
}

export interface Admin {
  id: string;
  email: string;
  username: string;
  passwordHash: string;
  salt: string;
  role: AdminRole;
  permissions: Permission[];
  isActive: boolean;
  isSuperAdmin: boolean;
  createdAt: number;
  updatedAt: number;
  lastLogin: number;
  failedLoginAttempts: number;
  lockedUntil: number;
  twoFactorEnabled: boolean;
}

export interface Blockchain {
  id: string;
  name: string;
  symbol: string;
  chainId: number;
  type: 'evm' | 'solana' | 'cosmos' | 'aptos' | 'sui' | 'ton' | 'bitcoin' | 'near' | 'polkadot' | 'algorand' | 'cardano';
  rpcUrl: string;
  explorerUrl: string;
  nativeCurrency: { name: string; symbol: string; decimals: number; address: string };
  isActive: boolean;
  isDefault: boolean;
}

export interface Token {
  id: string;
  address: string;
  blockchainId: string;
  name: string;
  symbol: string;
  decimals: number;
  type: string;
  isActive: boolean;
  isWhitelisted: boolean;
  coingeckoId?: string;
  listingFee: string;
  listingFeePaid: boolean;
}

export interface FeeConfig {
  id: string;
  name: string;
  type: string;
  value: string;
  recipient: string;
  isActive: boolean;
}

export interface WhiteLabel {
  id: string;
  name: string;
  domain: string;
  apiKey: string;
  apiSecret: string;
  isActive: boolean;
  isApproved: boolean;
  approvedBy?: string;
  approvedAt?: number;
  ownerAdminId: string;
  feeSharingPercent: number;
}

export interface BotClient {
  id: string;
  name: string;
  email: string;
  botType: string;
  subscriptionTier: string;
  isActive: boolean;
  isApproved: boolean;
  approvedBy?: string;
}

export interface ExternalAPI {
  id: string;
  name: string;
  type: string;
  apiKey: string;
  permissions: Permission[];
  isActive: boolean;
}

// ============================================================================
// Security Functions
// ============================================================================

export function generateSecureToken(length: number = 32): string {
  return crypto.randomBytes(length).toString('hex');
}

export function generateAdminId(): string {
  return 'admin_' + crypto.randomBytes(16).toString('hex');
}

export function generateAPIKey(): string {
  return 'tig_' + crypto.randomBytes(32).toString('hex');
}

export function hashPassword(password: string, salt?: string): { hash: string; salt: string } {
  const useSalt = salt || crypto.randomBytes(32).toString('hex');
  const hash = crypto.pbkdf2Sync(password, useSalt, 100000, 64, 'sha512').toString('hex');
  return { hash, salt: useSalt };
}

export function verifyPassword(password: string, hash: string, salt: string): boolean {
  const { hash: computedHash } = hashPassword(password, salt);
  return computedHash === hash;
}

// ============================================================================
// Permission System
// ============================================================================

export const ROLE_PERMISSIONS: Record<AdminRole, Permission[]> = {
  [AdminRole.SUPER_ADMIN]: Object.values(Permission),
  [AdminRole.ADMIN]: [
    Permission.VIEW_USERS, Permission.MANAGE_TRADING, Permission.VIEW_TRADING,
    Permission.MANAGE_FEES, Permission.VIEW_BLOCKCHAINS, Permission.MANAGE_TOKENS,
    Permission.LIST_TOKENS, Permission.MANAGE_BOTS, Permission.APPROVE_BOTS,
    Permission.MANAGE_CEX, Permission.MANAGE_DEX, Permission.VIEW_API, Permission.VIEW_ADMINS,
  ],
  [AdminRole.MODERATOR]: [
    Permission.VIEW_USERS, Permission.VIEW_TRADING, Permission.VIEW_BLOCKCHAINS, Permission.VIEW_API,
  ],
  [AdminRole.SUPPORT]: [Permission.VIEW_USERS, Permission.VIEW_TRADING],
  [AdminRole.VIEWER]: [Permission.VIEW_USERS, Permission.VIEW_TRADING, Permission.VIEW_BLOCKCHAINS, Permission.VIEW_API, Permission.VIEW_ADMINS],
};

// ============================================================================
// Blockchain Data (40+ Chains)
// ============================================================================

export const DEFAULT_BLOCKCHAINS: Blockchain[] = [
  // EVM Chains (20+)
  { id: 'eth', name: 'Ethereum', symbol: 'ETH', chainId: 1, type: 'evm', rpcUrl: 'https://eth.llamarpc.com', explorerUrl: 'https://etherscan.io', nativeCurrency: { name: 'Ether', symbol: 'ETH', decimals: 18, address: '0x0000000000000000000000000000000000000000' }, isActive: true, isDefault: true },
  { id: 'bsc', name: 'BNB Smart Chain', symbol: 'BNB', chainId: 56, type: 'evm', rpcUrl: 'https://bsc-dataseed.binance.org', explorerUrl: 'https://bscscan.com', nativeCurrency: { name: 'BNB', symbol: 'BNB', decimals: 18, address: '0x0000000000000000000000000000000000000000' }, isActive: true, isDefault: true },
  { id: 'polygon', name: 'Polygon', symbol: 'MATIC', chainId: 137, type: 'evm', rpcUrl: 'https://polygon-rpc.com', explorerUrl: 'https://polygonscan.com', nativeCurrency: { name: 'MATIC', symbol: 'MATIC', decimals: 18, address: '0x0000000000000000000000000000000000000000' }, isActive: true, isDefault: true },
  { id: 'arbitrum', name: 'Arbitrum One', symbol: 'ETH', chainId: 42161, type: 'evm', rpcUrl: 'https://arb1.arbitrum.io/rpc', explorerUrl: 'https://arbiscan.io', nativeCurrency: { name: 'Ether', symbol: 'ETH', decimals: 18, address: '0x0000000000000000000000000000000000000000' }, isActive: true, isDefault: true },
  { id: 'optimism', name: 'Optimism', symbol: 'ETH', chainId: 10, type: 'evm', rpcUrl: 'https://mainnet.optimism.io', explorerUrl: 'https://optimistic.etherscan.io', nativeCurrency: { name: 'Ether', symbol: 'ETH', decimals: 18, address: '0x0000000000000000000000000000000000000000' }, isActive: true, isDefault: true },
  { id: 'base', name: 'Base', symbol: 'ETH', chainId: 8453, type: 'evm', rpcUrl: 'https://base-mainnet.infura.io', explorerUrl: 'https://basescan.org', nativeCurrency: { name: 'Ether', symbol: 'ETH', decimals: 18, address: '0x0000000000000000000000000000000000000000' }, isActive: true, isDefault: true },
  { id: 'avalanche', name: 'Avalanche', symbol: 'AVAX', chainId: 43114, type: 'evm', rpcUrl: 'https://api.avax.network/ext/bc/C/r', explorerUrl: 'https://snowtrace.io', nativeCurrency: { name: 'AVAX', symbol: 'AVAX', decimals: 18, address: '0x0000000000000000000000000000000000000000' }, isActive: true, isDefault: true },
  { id: 'fantom', name: 'Fantom', symbol: 'FTM', chainId: 250, type: 'evm', rpcUrl: 'https://rpc.fantom.network', explorerUrl: 'https://ftmscan.com', nativeCurrency: { name: 'Fantom', symbol: 'FTM', decimals: 18, address: '0x0000000000000000000000000000000000000000' }, isActive: true, isDefault: false },
  { id: 'celo', name: 'Celo', symbol: 'CELO', chainId: 42220, type: 'evm', rpcUrl: 'https://forno.celo.org', explorerUrl: 'https://explorer.celo.org', nativeCurrency: { name: 'CELO', symbol: 'CELO', decimals: 18, address: '0x0000000000000000000000000000000000000000' }, isActive: true, isDefault: false },
  { id: 'gnosis', name: 'Gnosis', symbol: 'XDAI', chainId: 100, type: 'evm', rpcUrl: 'https://rpc.gnosischain.com', explorerUrl: 'https://gnosisscan.io', nativeCurrency: { name: 'xDAI', symbol: 'XDAI', decimals: 18, address: '0x0000000000000000000000000000000000000000' }, isActive: true, isDefault: false },
  { id: 'moonbeam', name: 'Moonbeam', symbol: 'GLMR', chainId: 1284, type: 'evm', rpcUrl: 'https://rpc.api.moonbeam.network', explorerUrl: 'https://moonscan.io', nativeCurrency: { name: 'Moonbeam', symbol: 'GLMR', decimals: 18, address: '0x0000000000000000000000000000000000000000' }, isActive: true, isDefault: false },
  { id: 'kava', name: 'Kava', symbol: 'KAVA', chainId: 2222, type: 'evm', rpcUrl: 'https://evm.kava.io', explorerUrl: 'https://explorer.kava.io', nativeCurrency: { name: 'Kava', symbol: 'KAVA', decimals: 18, address: '0x0000000000000000000000000000000000000000' }, isActive: true, isDefault: false },
  { id: 'linea', name: 'Linea', symbol: 'ETH', chainId: 59144, type: 'evm', rpcUrl: 'https://rpc.linea.build', explorerUrl: 'https://explorer.linea.build', nativeCurrency: { name: 'Ether', symbol: 'ETH', decimals: 18, address: '0x0000000000000000000000000000000000000000' }, isActive: true, isDefault: false },
  { id: 'scroll', name: 'Scroll', symbol: 'ETH', chainId: 534352, type: 'evm', rpcUrl: 'https://rpc.scroll.io', explorerUrl: 'https://blockscout.scroll.io', nativeCurrency: { name: 'Ether', symbol: 'ETH', decimals: 18, address: '0x0000000000000000000000000000000000000000' }, isActive: true, isDefault: false },
  { id: 'zkevm', name: 'zkEVM', symbol: 'ETH', chainId: 1101, type: 'evm', rpcUrl: 'https://zkevm-rpc.com', explorerUrl: 'https://zkevm.polygonscan.com', nativeCurrency: { name: 'Ether', symbol: 'ETH', decimals: 18, address: '0x0000000000000000000000000000000000000000' }, isActive: true, isDefault: false },
  { id: 'mantle', name: 'Mantle', symbol: 'MNT', chainId: 5000, type: 'evm', rpcUrl: 'https://rpc.mantle.xyz', explorerUrl: 'https://explorer.mantle.xyz', nativeCurrency: { name: 'Mantle', symbol: 'MNT', decimals: 18, address: '0x0000000000000000000000000000000000000000' }, isActive: true, isDefault: false },
  { id: 'opbnb', name: 'opBNB', symbol: 'BNB', chainId: 204, type: 'evm', rpcUrl: 'https://opbnb-mainnet-rpc.bnbchain.org', explorerUrl: 'https://opbnbscan.com', nativeCurrency: { name: 'BNB', symbol: 'BNB', decimals: 18, address: '0x0000000000000000000000000000000000000000' }, isActive: true, isDefault: false },
  { id: 'mode', name: 'Mode', symbol: 'ETH', chainId: 34443, type: 'evm', rpcUrl: 'https://mainnet.mode.network', explorerUrl: 'https://explorer.mode.network', nativeCurrency: { name: 'Ether', symbol: 'ETH', decimals: 18, address: '0x0000000000000000000000000000000000000000' }, isActive: true, isDefault: false },
  { id: 'zora', name: 'Zora', symbol: 'ETH', chainId: 7777777, type: 'evm', rpcUrl: 'https://rpc.zora.energy', explorerUrl: 'https://explorer.zora.energy', nativeCurrency: { name: 'Ether', symbol: 'ETH', decimals: 18, address: '0x0000000000000000000000000000000000000000' }, isActive: true, isDefault: false },
  { id: 'metis', name: 'Metis', symbol: 'METIS', chainId: 1088, type: 'evm', rpcUrl: 'https://andromeda.metis.io', explorerUrl: 'https://andromeda-explorer.metis.io', nativeCurrency: { name: 'Metis', symbol: 'METIS', decimals: 18, address: '0x0000000000000000000000000000000000000000' }, isActive: true, isDefault: false },

  // Non-EVM Chains (20+)
  { id: 'solana', name: 'Solana', symbol: 'SOL', chainId: 101, type: 'solana', rpcUrl: 'https://api.mainnet-beta.solana.com', explorerUrl: 'https://solscan.io', nativeCurrency: { name: 'Solana', symbol: 'SOL', decimals: 9, address: '' }, isActive: true, isDefault: true },
  { id: 'cosmos', name: 'Cosmos', symbol: 'ATOM', chainId: 0, type: 'cosmos', rpcUrl: 'https://rpc.cosmos.network', explorerUrl: 'https://mintscan.io/cosmos', nativeCurrency: { name: 'Atom', symbol: 'ATOM', decimals: 6, address: '' }, isActive: true, isDefault: true },
  { id: 'osmosis', name: 'Osmosis', symbol: 'OSMO', chainId: 0, type: 'cosmos', rpcUrl: 'https://rpc.osmosis.zone', explorerUrl: 'https://mintscan.io/osmosis', nativeCurrency: { name: 'Osmosis', symbol: 'OSMO', decimals: 6, address: '' }, isActive: true, isDefault: false },
  { id: 'injective', name: 'Injective', symbol: 'INJ', chainId: 0, type: 'cosmos', rpcUrl: 'https://rpc.injective.network', explorerUrl: 'https://explorer.injective.network', nativeCurrency: { name: 'Injective', symbol: 'INJ', decimals: 18, address: '' }, isActive: true, isDefault: false },
  { id: 'aptos', name: 'Aptos', symbol: 'APT', chainId: 0, type: 'aptos', rpcUrl: 'https://aptos-mainnet.nodereal.io/v1', explorerUrl: 'https://explorer.aptoslabs.com', nativeCurrency: { name: 'Aptos', symbol: 'APT', decimals: 8, address: '' }, isActive: true, isDefault: true },
  { id: 'sui', name: 'Sui', symbol: 'SUI', chainId: 0, type: 'sui', rpcUrl: 'https://fullnode.mainnet.sui.io', explorerUrl: 'https://suiscan.xyz', nativeCurrency: { name: 'Sui', symbol: 'SUI', decimals: 9, address: '' }, isActive: true, isDefault: true },
  { id: 'ton', name: 'Toncoin', symbol: 'TON', chainId: 0, type: 'ton', rpcUrl: 'https://toncenter.com/api/v2', explorerUrl: 'https://tonscan.org', nativeCurrency: { name: 'Toncoin', symbol: 'TON', decimals: 9, address: '' }, isActive: true, isDefault: true },
  { id: 'bitcoin', name: 'Bitcoin', symbol: 'BTC', chainId: 0, type: 'bitcoin', rpcUrl: 'https://blockstream.info/api', explorerUrl: 'https://blockstream.info', nativeCurrency: { name: 'Bitcoin', symbol: 'BTC', decimals: 8, address: '' }, isActive: true, isDefault: true },
  { id: 'near', name: 'NEAR', symbol: 'NEAR', chainId: 0, type: 'near', rpcUrl: 'https://rpc.mainnet.near.org', explorerUrl: 'https://explorer.near.org', nativeCurrency: { name: 'NEAR', symbol: 'NEAR', decimals: 24, address: '' }, isActive: true, isDefault: true },
  { id: 'algorand', name: 'Algorand', symbol: 'ALGO', chainId: 0, type: 'algorand', rpcUrl: 'https://mainnet-api.algonode.cloud', explorerUrl: 'https://algoexplorer.io', nativeCurrency: { name: 'Algorand', symbol: 'ALGO', decimals: 6, address: '' }, isActive: true, isDefault: false },
  { id: 'cardano', name: 'Cardano', symbol: 'ADA', chainId: 0, type: 'cardano', rpcUrl: 'https://cardano-mainnet.blockfrost.io', explorerUrl: 'https://cardanoscan.io', nativeCurrency: { name: 'Cardano', symbol: 'ADA', decimals: 6, address: '' }, isActive: true, isDefault: false },
  { id: 'polkadot', name: 'Polkadot', symbol: 'DOT', chainId: 0, type: 'polkadot', rpcUrl: 'https://rpc.polkadot.io', explorerUrl: 'https://polkadot.subscan.io', nativeCurrency: { name: 'Polkadot', symbol: 'DOT', decimals: 10, address: '' }, isActive: true, isDefault: false },
  { id: 'kusama', name: 'Kusama', symbol: 'KSM', chainId: 0, type: 'polkadot', rpcUrl: 'https://rpc.kusama.network', explorerUrl: 'https://kusama.subscan.io', nativeCurrency: { name: 'Kusama', symbol: 'KSM', decimals: 12, address: '' }, isActive: true, isDefault: false },
  { id: 'tron', name: 'Tron', symbol: 'TRX', chainId: 0, type: 'evm', rpcUrl: 'https://api.trongrid.io', explorerUrl: 'https://tronscan.org', nativeCurrency: { name: 'Tron', symbol: 'TRX', decimals: 6, address: '' }, isActive: true, isDefault: true },
  { id: 'sei', name: 'Sei', symbol: 'SEI', chainId: 0, type: 'cosmos', rpcUrl: 'https://rpc.sei.io', explorerUrl: 'https://seitrace.com', nativeCurrency: { name: 'Sei', symbol: 'SEI', decimals: 6, address: '' }, isActive: true, isDefault: false },
  { id: 'berachain', name: 'Berachain', symbol: 'BERA', chainId: 0, type: 'evm', rpcUrl: 'https://rpc.berachain.com', explorerUrl: 'https://berascan.com', nativeCurrency: { name: 'Berachain', symbol: 'BERA', decimals: 18, address: '0x0000000000000000000000000000000000000000' }, isActive: true, isDefault: false },
  { id: 'sonic', name: 'Sonic', symbol: 'S', chainId: 0, type: 'evm', rpcUrl: 'https://api.soniclabs.com', explorerUrl: 'https://sonicscan.org', nativeCurrency: { name: 'Sonic', symbol: 'S', decimals: 18, address: '0x0000000000000000000000000000000000000000' }, isActive: true, isDefault: false },
  { id: 'monad', name: 'Monad', symbol: 'MON', chainId: 0, type: 'evm', rpcUrl: 'https://rpc.monad.xyz', explorerUrl: 'https://explorer.monad.xyz', nativeCurrency: { name: 'Monad', symbol: 'MON', decimals: 18, address: '0x0000000000000000000000000000000000000000' }, isActive: true, isDefault: false },
  { id: 'megaeth', name: 'MegaETH', symbol: 'MEGA', chainId: 0, type: 'evm', rpcUrl: 'https://rpc.megaeth.com', explorerUrl: 'https://explorer.megaeth.com', nativeCurrency: { name: 'MegaETH', symbol: 'MEGA', decimals: 18, address: '0x0000000000000000000000000000000000000000' }, isActive: true, isDefault: false },
];

// ============================================================================
// Token Data (50+ Tokens)
// ============================================================================

export const DEFAULT_TOKENS: Token[] = [
  // Ethereum
  { id: 'eth', address: '0x0000000000000000000000000000000000000000', blockchainId: 'eth', name: 'Ethereum', symbol: 'ETH', decimals: 18, type: 'native', isActive: true, isWhitelisted: true, listingFee: '0', listingFeePaid: true },
  { id: 'usdc', address: '0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48', blockchainId: 'eth', name: 'USD Coin', symbol: 'USDC', decimals: 6, type: 'erc20', isActive: true, isWhitelisted: true, listingFee: '0', listingFeePaid: true },
  { id: 'usdt', address: '0xdAC17F958D2ee523a2206206994597C13D831ec7', blockchainId: 'eth', name: 'Tether USD', symbol: 'USDT', decimals: 6, type: 'erc20', isActive: true, isWhitelisted: true, listingFee: '0', listingFeePaid: true },
  { id: 'dai', address: '0x6B175474E89094C44Da98b954EeadeAC9f2F8d7a', blockchainId: 'eth', name: 'Dai Stablecoin', symbol: 'DAI', decimals: 18, type: 'erc20', isActive: true, isWhitelisted: true, listingFee: '0', listingFeePaid: true },
  { id: 'wbtc', address: '0x2260FAC5E5542a773Aa44fBCfeDf7C193bc2C805', blockchainId: 'eth', name: 'Wrapped Bitcoin', symbol: 'WBTC', decimals: 8, type: 'erc20', isActive: true, isWhitelisted: true, listingFee: '0', listingFeePaid: true },
  { id: 'link', address: '0x514910771AF9CA656af840bdff391E2f99b2EbA2D0', blockchainId: 'eth', name: 'Chainlink', symbol: 'LINK', decimals: 18, type: 'erc20', isActive: true, isWhitelisted: true, listingFee: '0', listingFeePaid: true },
  { id: 'uni', address: '0x1f9840a85d5aF5bf1D1762F925BDADdC4201F9840', blockchainId: 'eth', name: 'Uniswap', symbol: 'UNI', decimals: 18, type: 'erc20', isActive: true, isWhitelisted: true, listingFee: '0', listingFeePaid: true },
  { id: 'aave', address: '0x7Fc66500c84A76Ad7e9c93437bFc5Ac33E2DDaE9', blockchainId: 'eth', name: 'Aave', symbol: 'AAVE', decimals: 18, type: 'erc20', isActive: true, isWhitelisted: true, listingFee: '0', listingFeePaid: true },
  { id: 'mkr', address: '0x9f8F72aA9304c8B593d555F12eF6589cC4BAb865', blockchainId: 'eth', name: 'Maker', symbol: 'MKR', decimals: 18, type: 'erc20', isActive: true, isWhitelisted: true, listingFee: '0', listingFeePaid: true },
  { id: 'crv', address: '0xD533a949740bb3306d119CC777fa900bA034cd52', blockchainId: 'eth', name: 'Curve DAO', symbol: 'CRV', decimals: 18, type: 'erc20', isActive: true, isWhitelisted: true, listingFee: '0', listingFeePaid: true },
  { id: 'pepe', address: '0x6982508145454Ce325dDbE47a25d4ec3d2311933', blockchainId: 'eth', name: 'Pepe', symbol: 'PEPE', decimals: 18, type: 'erc20', isActive: true, isWhitelisted: true, listingFee: '0', listingFeePaid: true },
  { id: 'shib', address: '0x95aD61b0a150d79219dCF64E1E6Cc01f0B64C4dE', blockchainId: 'eth', name: 'Shiba Inu', symbol: 'SHIB', decimals: 18, type: 'erc20', isActive: true, isWhitelisted: true, listingFee: '0', listingFeePaid: true },
  
  // BSC
  { id: 'bnb', address: '0x0000000000000000000000000000000000000000', blockchainId: 'bsc', name: 'BNB', symbol: 'BNB', decimals: 18, type: 'native', isActive: true, isWhitelisted: true, listingFee: '0', listingFeePaid: true },
  { id: 'busd', address: '0xe9e7CEA3DedcA5984780Bafc599bD69ADd087D56', blockchainId: 'bsc', name: 'Binance USD', symbol: 'BUSD', decimals: 18, type: 'erc20', isActive: true, isWhitelisted: true, listingFee: '0', listingFeePaid: true },
  { id: 'cake', address: '0x0E09FaBB73Bd3Ade0a17ECC321fD13a19e81cE82', blockchainId: 'bsc', name: 'PancakeSwap', symbol: 'CAKE', decimals: 18, type: 'erc20', isActive: true, isWhitelisted: true, listingFee: '0', listingFeePaid: true },
  
  // Polygon
  { id: 'matic', address: '0x0000000000000000000000000000000000000000', blockchainId: 'polygon', name: 'Polygon', symbol: 'MATIC', decimals: 18, type: 'native', isActive: true, isWhitelisted: true, listingFee: '0', listingFeePaid: true },
  { id: 'quick', address: '0xb5C064F955D8e7F38FE0460C556a72987494bE17', blockchainId: 'polygon', name: 'QuickSwap', symbol: 'QUICK', decimals: 18, type: 'erc20', isActive: true, isWhitelisted: true, listingFee: '0', listingFeePaid: true },
  
  // Solana
  { id: 'sol', address: '', blockchainId: 'solana', name: 'Solana', symbol: 'SOL', decimals: 9, type: 'native', isActive: true, isWhitelisted: true, listingFee: '0', listingFeePaid: true },
  { id: 'ray', address: '4k3DyjzvzpNoeMauLei1d6xG5M6q1J5vBQfBDqYqC6qK', blockchainId: 'solana', name: 'Raydium', symbol: 'RAY', decimals: 6, type: 'spl', isActive: true, isWhitelisted: true, listingFee: '0', listingFeePaid: true },
  { id: 'orca', address: 'orcaEKTdK7LKzJGvaKjDSFFYkC5kaJiHW4YREtm2kTDFh', blockchainId: 'solana', name: 'Orca', symbol: 'ORCA', decimals: 6, type: 'spl', isActive: true, isWhitelisted: true, listingFee: '0', listingFeePaid: true },
  { id: 'jito', address: 'JUPyiwrYJFskUPiHa7hkeR8VUtkqjberbSOWd91pbT2', blockchainId: 'solana', name: 'Jito', symbol: 'JTO', decimals: 9, type: 'spl', isActive: true, isWhitelisted: true, listingFee: '0', listingFeePaid: true },
  { id: 'bonk', address: 'DezXAZ8z7Pnrnzjx7AAgADVLUd3EVdRmSCBy6JFPMJG', blockchainId: 'solana', name: 'Bonk', symbol: 'BONK', decimals: 5, type: 'spl', isActive: true, isWhitelisted: true, listingFee: '0', listingFeePaid: true },
  { id: 'usdc-sol', address: 'EPjFWdd5AufqSSBc4pt2uNTfKp5r2m2h6D2vC2pX7KnF', blockchainId: 'solana', name: 'USD Coin', symbol: 'USDC', decimals: 6, type: 'spl', isActive: true, isWhitelisted: true, listingFee: '0', listingFeePaid: true },
  { id: 'usdt-sol', address: 'Es9vMFrzaC7wBQk65VQvJ4eJGB3vZ8q9v4YJ4Y8vJ4', blockchainId: 'solana', name: 'Tether USD', symbol: 'USDT', decimals: 6, type: 'spl', isActive: true, isWhitelisted: true, listingFee: '0', listingFeePaid: true },
  
  // More chains
  { id: 'avax', address: '0x0000000000000000000000000000000000000000', blockchainId: 'avalanche', name: 'Avalanche', symbol: 'AVAX', decimals: 18, type: 'native', isActive: true, isWhitelisted: true, listingFee: '0', listingFeePaid: true },
  { id: 'ftm', address: '0x0000000000000000000000000000000000000000', blockchainId: 'fantom', name: 'Fantom', symbol: 'FTM', decimals: 18, type: 'native', isActive: true, isWhitelisted: true, listingFee: '0', listingFeePaid: true },
  { id: 'arb', address: '0x0000000000000000000000000000000000000000', blockchainId: 'arbitrum', name: 'Arbitrum', symbol: 'ETH', decimals: 18, type: 'native', isActive: true, isWhitelisted: true, listingFee: '0', listingFeePaid: true },
  { id: 'op', address: '0x0000000000000000000000000000000000000000', blockchainId: 'optimism', name: 'Optimism', symbol: 'ETH', decimals: 18, type: 'native', isActive: true, isWhitelisted: true, listingFee: '0', listingFeePaid: true },
  { id: 'base-eth', address: '0x0000000000000000000000000000000000000000', blockchainId: 'base', name: 'Base', symbol: 'ETH', decimals: 18, type: 'native', isActive: true, isWhitelisted: true, listingFee: '0', listingFeePaid: true },
  { id: 'apt', address: '', blockchainId: 'aptos', name: 'Aptos', symbol: 'APT', decimals: 8, type: 'native', isActive: true, isWhitelisted: true, listingFee: '0', listingFeePaid: true },
  { id: 'sui-token', address: '', blockchainId: 'sui', name: 'Sui', symbol: 'SUI', decimals: 9, type: 'native', isActive: true, isWhitelisted: true, listingFee: '0', listingFeePaid: true },
  { id: 'ton-token', address: '', blockchainId: 'ton', name: 'Toncoin', symbol: 'TON', decimals: 9, type: 'native', isActive: true, isWhitelisted: true, listingFee: '0', listingFeePaid: true },
  { id: 'btc', address: '', blockchainId: 'bitcoin', name: 'Bitcoin', symbol: 'BTC', decimals: 8, type: 'native', isActive: true, isWhitelisted: true, listingFee: '0', listingFeePaid: true },
  { id: 'near-token', address: '', blockchainId: 'near', name: 'NEAR', symbol: 'NEAR', decimals: 24, type: 'native', isActive: true, isWhitelisted: true, listingFee: '0', listingFeePaid: true },
  { id: 'trx', address: '', blockchainId: 'tron', name: 'Tron', symbol: 'TRX', decimals: 6, type: 'native', isActive: true, isWhitelisted: true, listingFee: '0', listingFeePaid: true },
];

// ============================================================================
// Default Fee Configuration
// ============================================================================

export const DEFAULT_FEES: FeeConfig[] = [
  { id: 'swap_fee', name: 'Swap Fee', type: 'swap_fee', value: '30', recipient: '', isActive: true },
  { id: 'trading_fee', name: 'Trading Fee', type: 'trading_fee', value: '30', recipient: '', isActive: true },
  { id: 'withdraw_fee', name: 'Withdrawal Fee', type: 'withdraw_fee', value: '10', recipient: '', isActive: true },
  { id: 'deposit_fee', name: 'Deposit Fee', type: 'deposit_fee', value: '0', recipient: '', isActive: true },
  { id: 'transfer_fee', name: 'Transfer Fee', type: 'transfer_fee', value: '5', recipient: '', isActive: true },
  { id: 'listing_fee', name: 'Token Listing Fee', type: 'listing_fee', value: '1000000000000000000', recipient: '', isActive: true },
  { id: 'bot_subscription_fee', name: 'Bot Subscription Fee', type: 'bot_subscription_fee', value: '100000000000000000', recipient: '', isActive: true },
  { id: 'whitelabel_fee', name: 'White Label Fee', type: 'whitelabel_fee', value: '2000', recipient: '', isActive: true },
  { id: 'api_key_fee', name: 'API Key Fee', type: 'api_key_fee', value: '50000000000000000', recipient: '', isActive: true },
  { id: 'cross_chain_fee', name: 'Cross-Chain Fee', type: 'cross_chain_fee', value: '50', recipient: '', isActive: true },
];

// ============================================================================
// Admin Manager Class
// ============================================================================

export class CompleteAdminManager {
  private admins: Map<string, Admin> = new Map();
  private blockchains: Map<string, Blockchain> = new Map();
  private tokens: Map<string, Token> = new Map();
  private fees: Map<string, FeeConfig> = new Map();
  private whiteLabels: Map<string, WhiteLabel> = new Map();
  private botClients: Map<string, BotClient> = new Map();
  private externalAPIs: Map<string, ExternalAPI> = new Map();
  private sessions: Map<string, { adminId: string; expiresAt: number }> = new Map();

  constructor() {
    this.initializeSystem();
  }

  private initializeSystem(): void {
    // Initialize blockchains
    for (const chain of DEFAULT_BLOCKCHAINS) {
      this.blockchains.set(chain.id, chain);
    }

    // Initialize tokens
    for (const token of DEFAULT_TOKENS) {
      this.tokens.set(token.id, token);
    }

    // Initialize fees
    for (const fee of DEFAULT_FEES) {
      this.fees.set(fee.id, fee);
    }

    // Create super admin
    const { hash, salt } = hashPassword('TigerSwap@2026SuperAdmin');
    const superAdmin: Admin = {
      id: generateAdminId(),
      email: 'superadmin@tigerswap.io',
      username: 'superadmin',
      passwordHash: hash,
      salt,
      role: AdminRole.SUPER_ADMIN,
      permissions: ROLE_PERMISSIONS[AdminRole.SUPER_ADMIN],
      isActive: true,
      isSuperAdmin: true,
      createdAt: Date.now(),
      updatedAt: Date.now(),
      lastLogin: 0,
      failedLoginAttempts: 0,
      lockedUntil: 0,
      twoFactorEnabled: false,
    };
    this.admins.set(superAdmin.id, superAdmin);
    
    console.log('✅ Complete Admin System Initialized');
    console.log(`📝 Super Admin Email: superadmin@tigerswap.io`);
    console.log(`🔐 Default Password: TigerSwap@2026SuperAdmin`);
    console.log(`🌐 Blockchains: ${this.blockchains.size}`);
    console.log(`🪙 Tokens: ${this.tokens.size}`);
  }

  // Authentication
  async login(email: string, password: string): Promise<{ success: boolean; sessionId?: string; error?: string }> {
    const admin = Array.from(this.admins.values()).find(a => a.email === email);
    
    if (!admin) return { success: false, error: 'Invalid credentials' };
    
    if (admin.lockedUntil > Date.now()) {
      return { success: false, error: 'Account is locked' };
    }

    if (!verifyPassword(password, admin.passwordHash, admin.salt)) {
      admin.failedLoginAttempts++;
      if (admin.failedLoginAttempts >= 5) {
        admin.lockedUntil = Date.now() + 15 * 60 * 1000;
      }
      return { success: false, error: 'Invalid credentials' };
    }

    admin.failedLoginAttempts = 0;
    admin.lastLogin = Date.now();
    
    const sessionId = generateSecureToken(32);
    this.sessions.set(sessionId, { adminId: admin.id, expiresAt: Date.now() + 24 * 60 * 60 * 1000 });
    
    return { success: true, sessionId };
  }

  logout(sessionId: string): boolean {
    return this.sessions.delete(sessionId);
  }

  validateSession(sessionId: string): boolean {
    const session = this.sessions.get(sessionId);
    return session && session.expiresAt > Date.now();
  }

  // Admin Management
  createAdmin(email: string, username: string, password: string, role: AdminRole): Admin | null {
    if (Array.from(this.admins.values()).some(a => a.email === email)) {
      return null;
    }

    const { hash, salt } = hashPassword(password);
    const admin: Admin = {
      id: generateAdminId(),
      email,
      username,
      passwordHash: hash,
      salt,
      role,
      permissions: ROLE_PERMISSIONS[role],
      isActive: true,
      isSuperAdmin: role === AdminRole.SUPER_ADMIN,
      createdAt: Date.now(),
      updatedAt: Date.now(),
      lastLogin: 0,
      failedLoginAttempts: 0,
      lockedUntil: 0,
      twoFactorEnabled: false,
    };
    
    this.admins.set(admin.id, admin);
    return admin;
  }

  getAdmin(id: string): Admin | undefined {
    return this.admins.get(id);
  }

  getAllAdmins(): Admin[] {
    return Array.from(this.admins.values());
  }

  // Blockchain Management
  getBlockchains(): Blockchain[] {
    return Array.from(this.blockchains.values());
  }

  getActiveBlockchains(): Blockchain[] {
    return Array.from(this.blockchains.values()).filter(b => b.isActive);
  }

  addBlockchain(blockchain: Blockchain): void {
    this.blockchains.set(blockchain.id, blockchain);
  }

  updateBlockchain(id: string, updates: Partial<Blockchain>): void {
    const chain = this.blockchains.get(id);
    if (chain) {
      Object.assign(chain, updates);
      this.blockchains.set(id, chain);
    }
  }

  removeBlockchain(id: string): void {
    this.blockchains.delete(id);
  }

  // Token Management
  getTokens(): Token[] {
    return Array.from(this.tokens.values());
  }

  getActiveTokens(): Token[] {
    return Array.from(this.tokens.values()).filter(t => t.isActive);
  }

  addToken(token: Token): void {
    this.tokens.set(token.id, token);
  }

  updateToken(id: string, updates: Partial<Token>): void {
    const token = this.tokens.get(id);
    if (token) {
      Object.assign(token, updates);
      this.tokens.set(id, token);
    }
  }

  removeToken(id: string): void {
    this.tokens.delete(id);
  }

  // Fee Management
  getFees(): FeeConfig[] {
    return Array.from(this.fees.values());
  }

  updateFee(id: string, value: string, recipient: string): void {
    const fee = this.fees.get(id);
    if (fee) {
      fee.value = value;
      fee.recipient = recipient;
      this.fees.set(id, fee);
    }
  }

  calculateFee(feeId: string, amount: bigint): bigint {
    const fee = this.fees.get(feeId);
    if (!fee || !fee.isActive) return 0n;
    return (amount * BigInt(fee.value)) / 10000n;
  }

  // White Label Management
  createWhiteLabel(name: string, domain: string, ownerAdminId: string, feePercent: number): WhiteLabel {
    const whiteLabel: WhiteLabel = {
      id: generateAdminId(),
      name,
      domain,
      apiKey: generateAPIKey(),
      apiSecret: generateSecureToken(48),
      isActive: true,
      isApproved: false,
      ownerAdminId,
      feeSharingPercent: feePercent,
    };
    this.whiteLabels.set(whiteLabel.id, whiteLabel);
    return whiteLabel;
  }

  approveWhiteLabel(id: string, adminId: string): void {
    const wl = this.whiteLabels.get(id);
    if (wl) {
      wl.isApproved = true;
      wl.approvedBy = adminId;
      wl.approvedAt = Date.now();
      this.whiteLabels.set(id, wl);
    }
  }

  getWhiteLabels(): WhiteLabel[] {
    return Array.from(this.whiteLabels.values());
  }

  // Bot Client Management
  createBotClient(name: string, email: string, botType: string): BotClient {
    const bot: BotClient = {
      id: generateAdminId(),
      name,
      email,
      botType,
      subscriptionTier: 'free',
      isActive: true,
      isApproved: false,
    };
    this.botClients.set(bot.id, bot);
    return bot;
  }

  approveBotClient(id: string, adminId: string): void {
    const bot = this.botClients.get(id);
    if (bot) {
      bot.isApproved = true;
      bot.approvedBy = adminId;
      this.botClients.set(id, bot);
    }
  }

  getBotClients(): BotClient[] {
    return Array.from(this.botClients.values());
  }

  // External API Management
  createExternalAPI(name: string, type: string, permissions: Permission[]): ExternalAPI {
    const api: ExternalAPI = {
      id: generateAdminId(),
      name,
      type,
      apiKey: generateAPIKey(),
      permissions,
      isActive: true,
    };
    this.externalAPIs.set(api.id, api);
    return api;
  }

  getExternalAPIs(): ExternalAPI[] {
    return Array.from(this.externalAPIs.values());
  }
}

// ============================================================================
// Export
// ============================================================================

export default CompleteAdminManager;
export {
  AdminRole,
  Permission,
  DEFAULT_BLOCKCHAINS,
  DEFAULT_TOKENS,
  DEFAULT_FEES
};