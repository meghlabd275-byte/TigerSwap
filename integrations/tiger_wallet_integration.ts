/**
 * TigerWallet Integration
 * 
 * Multichain Web3 Wallet functionality
 * Integrated into TigerEX platform
 * 
 * Features:
 * - Create non-custodial wallets
 * - Multi-chain support (24 EVM + 26 Non-EVM chains)
 * - Transaction history
 * - EIP-712 message signing
 * - Wallet transaction fees
 */

import { tigerEX } from './tiger_ex_integration';
import { ethers } from 'ethers';
import { createHash, randomBytes } from 'crypto';

// ============================================================================
// Types & Interfaces
// ============================================================================

export interface WalletConfig {
  id: string;
  address: string;
  publicKey: string;
  privateKeyHash: string;
  chainType: 'evm' | 'solana' | 'aptos' | 'sui' | 'ton' | 'cosmos';
  createdAt: number;
  lastActive: number;
  nonce: number;
}

export interface WalletBalance {
  symbol: string;
  address: string;
  balance: string;
  decimals: number;
  usdValue: number;
  chainId: string;
}

export interface Transaction {
  hash: string;
  from: string;
  to: string;
  value: string;
  token: string;
  chainId: string;
  status: 'pending' | 'confirmed' | 'failed';
  timestamp: number;
  gasUsed?: string;
  gasFee?: string;
  blockNumber?: number;
}

export interface WalletStats {
  totalTransactions: number;
  totalVolume: string;
  gasSpent: string;
  chainsUsed: string[];
}

// ============================================================================
// TigerWallet Core
// ============================================================================

export class TigerWallet {
  private wallets: Map<string, WalletConfig> = new Map();
  private transactions: Map<string, Transaction[]> = new Map();
  private balances: Map<string, Map<string, WalletBalance>> = new Map();
  
  constructor() {
    this.initialize();
  }

  private initialize(): void {
    console.log('[TigerWallet] Initialized successfully');
  }

  // ============================================================================
  // Wallet Creation
  // ============================================================================

  /**
   * Create new wallet
   */
  createWallet(chainType: 'evm' | 'solana' | 'aptos' | 'sui' | 'ton' | 'cosmos' = 'evm'): WalletConfig {
    const wallet = ethers.Wallet.createRandom();
    const id = this.generateWalletId();
    
    const config: WalletConfig = {
      id,
      address: wallet.address,
      publicKey: wallet.publicKey,
      privateKeyHash: this.hashPrivateKey(wallet.privateKey),
      chainType,
      createdAt: Date.now(),
      lastActive: Date.now(),
      nonce: 0,
    };
    
    this.wallets.set(id, config);
    this.transactions.set(id, []);
    this.balances.set(id, new Map());
    
    console.log(`[TigerWallet] Created wallet ${id} at ${wallet.address}`);
    
    return config;
  }

  /**
   * Import wallet from private key
   */
  importWallet(privateKey: string, chainType: 'evm' | 'solana' | 'aptos' | 'sui' | 'ton' | 'cosmos' = 'evm'): WalletConfig {
    const wallet = new ethers.Wallet(privateKey);
    const id = this.generateWalletId();
    
    const config: WalletConfig = {
      id,
      address: wallet.address,
      publicKey: wallet.publicKey,
      privateKeyHash: this.hashPrivateKey(privateKey),
      chainType,
      createdAt: Date.now(),
      lastActive: Date.now(),
      nonce: 0,
    };
    
    this.wallets.set(id, config);
    this.transactions.set(id, []);
    this.balances.set(id, new Map());
    
    console.log(`[TigerWallet] Imported wallet ${id}`);
    
    return config;
  }

  /**
   * Get wallet by ID
   */
  getWallet(id: string): WalletConfig | undefined {
    return this.wallets.get(id);
  }

  /**
   * Get wallet by address
   */
  getWalletByAddress(address: string): WalletConfig | undefined {
    for (const wallet of this.wallets.values()) {
      if (wallet.address.toLowerCase() === address.toLowerCase()) {
        return wallet;
      }
    }
    return undefined;
  }

  /**
   * Get all wallets
   */
  getAllWallets(): WalletConfig[] {
    return Array.from(this.wallets.values());
  }

  // ============================================================================
  // Transaction Management
  // ============================================================================

  /**
   * Send transaction
   */
  async sendTransaction(
    walletId: string,
    to: string,
    value: bigint,
    token: string = 'ETH',
    chainId: string = 'ethereum'
  ): Promise<Transaction> {
    const wallet = this.wallets.get(walletId);
    
    if (!wallet) {
      throw new Error('Wallet not found');
    }
    
    const tx: Transaction = {
      hash: this.generateTxHash(),
      from: wallet.address,
      to,
      value: value.toString(),
      token,
      chainId,
      status: 'pending',
      timestamp: Date.now(),
    };
    
    const txs = this.transactions.get(walletId) || [];
    txs.push(tx);
    this.transactions.set(walletId, txs);
    
    // Update wallet nonce
    wallet.nonce++;
    wallet.lastActive = Date.now();
    
    // Calculate and collect fee
    const gasFee = value * 1n / 1000n; // 0.1% wallet fee
    tigerEX.collectFee(gasFee, 'wallet');
    
    return tx;
  }

  /**
   * Sign message (EIP-712)
   */
  signMessage(walletId: string, message: string): string {
    const wallet = this.wallets.get(walletId);
    
    if (!wallet) {
      throw new Error('Wallet not found');
    }
    
    // In real implementation, sign with actual private key
    // For demo, return mock signature
    return '0x' + randomBytes(64).toString('hex');
  }

  /**
   * Get transaction history
   */
  getTransactionHistory(walletId: string): Transaction[] {
    return this.transactions.get(walletId) || [];
  }

  /**
   * Get pending transactions
   */
  getPendingTransactions(walletId: string): Transaction[] {
    const txs = this.transactions.get(walletId) || [];
    return txs.filter(tx => tx.status === 'pending');
  }

  // ============================================================================
  // Balance Management
  // ============================================================================

  /**
   * Update balance
   */
  updateBalance(walletId: string, symbol: string, balance: string, decimals: number, usdValue: number, chainId: string): void {
    const walletBalances = this.balances.get(walletId) || new Map();
    
    walletBalances.set(symbol, {
      symbol,
      address: this.wallets.get(walletId)?.address || '',
      balance,
      decimals,
      usdValue,
      chainId,
    });
    
    this.balances.set(walletId, walletBalances);
  }

  /**
   * Get balance
   */
  getBalance(walletId: string, symbol: string): WalletBalance | undefined {
    const walletBalances = this.balances.get(walletId);
    return walletBalances?.get(symbol);
  }

  /**
   * Get all balances
   */
  getAllBalances(walletId: string): WalletBalance[] {
    const walletBalances = this.balances.get(walletId);
    return walletBalances ? Array.from(walletBalances.values()) : [];
  }

  // ============================================================================
  // Wallet Stats
  // ============================================================================

  /**
   * Get wallet stats
   */
  getWalletStats(walletId: string): WalletStats {
    const txs = this.transactions.get(walletId) || [];
    const chainsUsed = new Set(txs.map(tx => tx.chainId));
    
    let totalVolume = 0n;
    let gasSpent = 0n;
    
    for (const tx of txs) {
      totalVolume += BigInt(tx.value || '0');
      gasSpent += BigInt(tx.gasFee || '0');
    }
    
    return {
      totalTransactions: txs.length,
      totalVolume: totalVolume.toString(),
      gasSpent: gasSpent.toString(),
      chainsUsed: Array.from(chainsUsed),
    };
  }

  // ============================================================================
  // Cross-Chain Support
  // ============================================================================

  /**
   * Get supported chains for wallet
   */
  getSupportedChains(): string[] {
    const evmChains = tigerEX.getSupportedEvmChains().map(c => c.id);
    const nonEvmChains = tigerEX.getSupportedNonEvmChains().map(c => c.id);
    return [...evmChains, ...nonEvmChains];
  }

  /**
   * Switch chain
   */
  async switchChain(walletId: string, chainId: string): Promise<void> {
    const wallet = this.wallets.get(walletId);
    
    if (!wallet) {
      throw new Error('Wallet not found');
    }
    
    const supportedChains = this.getSupportedChains();
    
    if (!supportedChains.includes(chainId)) {
      throw new Error(`Chain ${chainId} not supported`);
    }
    
    wallet.lastActive = Date.now();
    console.log(`[TigerWallet] Wallet ${walletId} switched to ${chainId}`);
  }

  // ============================================================================
  // Utility Methods
  // ============================================================================

  private generateWalletId(): string {
    return 'wallet_' + randomBytes(8).toString('hex');
  }

  private generateTxHash(): string {
    return '0x' + randomBytes(32).toString('hex');
  }

  private hashPrivateKey(privateKey: string): string {
    const hash = createHash('sha256');
    hash.update(privateKey);
    return hash.digest('hex');
  }
}

// ============================================================================
// Export Singleton
// ============================================================================

export const tigerWallet = new TigerWallet();

export default TigerWallet;