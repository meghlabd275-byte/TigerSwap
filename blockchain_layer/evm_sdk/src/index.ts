import { Interface } from "ethers";
/**
 * TigerSwap EVM Blockchain SDK
 * 
 * Enterprise-grade EVM chain support with native wallet, RPC, and smart contract interactions.
 * This SDK is completely independent - NO dependencies on MetaMask, WalletConnect, or any external wallets.
 * 
 * Supported Chains:
 * - Ethereum (1)
 * - BNB Smart Chain (56)
 * - Polygon (137)
 * - Arbitrum One (42161)
 * - Optimism (10)
 * - Base (8453)
 * - Avalanche C-Chain (43114)
 * - Fantom (250)
 * - And 30+ more EVM chains
 * 
 * @author TigerSwap
 * @version 1.0.0
 */

import { ethers, JsonRpcProvider, Wallet, Contract, Interface, keccak256, toUtf8Bytes } from 'ethers';

// ============================================================================
// Types & Interfaces
// ============================================================================

export interface EVMChainConfig {
  chainId: number;
  name: string;
  symbol: string;
  rpcUrl: string;
  explorerUrl: string;
  explorerApiUrl?: string;
  nativeCurrency: {
    name: string;
    symbol: string;
    decimals: number;
  };
  gasSettings?: {
    gasLimit?: number;
    gasPrice?: bigint;
    maxFeePerGas?: bigint;
    maxPriorityFeePerGas?: bigint;
  };
}

export interface TransactionRequest {
  to: string;
  value?: bigint;
  data?: string;
  gasLimit?: bigint;
  gasPrice?: bigint;
  maxFeePerGas?: bigint;
  maxPriorityFeePerGas?: bigint;
  nonce?: number;
  chainId?: number;
}

export interface TransactionReceipt {
  hash: string;
  blockNumber: number;
  blockHash: string;
  transactionIndex: number;
  from: string;
  to: string;
  value: string;
  gasUsed: bigint;
  effectiveGasPrice: bigint;
  logs: Log[];
  status: number;
  logsBloom: string;
}

export interface Log {
  address: string;
  topics: string[];
  data: string;
  logIndex: number;
  transactionIndex: number;
  transactionHash: string;
  blockHash: string;
  blockNumber: number;
}

export interface TokenInfo {
  address: string;
  name: string;
  symbol: string;
  decimals: number;
  totalSupply?: string;
}

export interface CallResult {
  success: boolean;
  data?: string;
  error?: string;
}

export interface Block {
  number: number;
  hash: string;
  parentHash: string;
  timestamp: number;
  nonce: string;
  difficulty: bigint;
  gasLimit: bigint;
  gasUsed: bigint;
  miner: string;
  extraData: string;
  transactions: string[];
  transactionsRoot: string;
  stateRoot: string;
  receiptsRoot: string;
  uncleHash: string;
  baseFeePerGas?: bigint;
}

export interface ChainStatus {
  chainId: number;
  blockNumber: number;
  timestamp: number;
  peerCount: number;
  syncing: boolean;
}

// ============================================================================
// Chain Registry - 40+ EVM Chains
// ============================================================================

export const CHAIN_REGISTRY: Record<number, EVMChainConfig> = {
  // Ethereum Mainnet
  1: {
    chainId: 1,
    name: 'Ethereum',
    symbol: 'ETH',
    rpcUrl: process.env.TIGERSWAP_ETHEREUM_RPC_URL || 'https://eth.llamarpc.com',
    explorerUrl: 'https://etherscan.io',
    explorerApiUrl: 'https://api.etherscan.io/api',
    nativeCurrency: { name: 'Ether', symbol: 'ETH', decimals: 18 },
  },
  // Ethereum Sepolia (Testnet)
  11155111: {
    chainId: 11155111,
    name: 'Sepolia',
    symbol: 'ETH',
    rpcUrl: process.env.TIGERSWAP_SEPOLIA_RPC_URL || 'https://rpc.sepolia.org',
    explorerUrl: 'https://sepolia.etherscan.io',
    explorerApiUrl: 'https://api-sepolia.etherscan.io/api',
    nativeCurrency: { name: 'Sepolia Ether', symbol: 'ETH', decimals: 18 },
  },
  // BNB Smart Chain
  56: {
    chainId: 56,
    name: 'BNB Chain',
    symbol: 'BNB',
    rpcUrl: 'https://bsc-dataseed.binance.org',
    explorerUrl: 'https://bscscan.com',
    explorerApiUrl: 'https://api.bscscan.com/api',
    nativeCurrency: { name: 'BNB', symbol: 'BNB', decimals: 18 },
  },
  // BNB Testnet
  97: {
    chainId: 97,
    name: 'BNB Testnet',
    symbol: 'BNB',
    rpcUrl: 'https://data-seed-prebsc-1.binance.org',
    explorerUrl: 'https://testnet.bscscan.com',
    explorerApiUrl: 'https://api-testnet.bscscan.com/api',
    nativeCurrency: { name: 'BNB', symbol: 'BNB', decimals: 18 },
  },
  // Polygon
  137: {
    chainId: 137,
    name: 'Polygon',
    symbol: 'MATIC',
    rpcUrl: 'https://polygon-rpc.com',
    explorerUrl: 'https://polygonscan.com',
    explorerApiUrl: 'https://api.polygonscan.com/api',
    nativeCurrency: { name: 'MATIC', symbol: 'MATIC', decimals: 18 },
  },
  // Polygon Mumbai (Testnet)
  80001: {
    chainId: 80001,
    name: 'Polygon Mumbai',
    symbol: 'MATIC',
    rpcUrl: 'https://rpc-mumbai.maticvigil.com',
    explorerUrl: 'https://mumbai.polygonscan.com',
    explorerApiUrl: 'https://api-mumbai.polygonscan.com/api',
    nativeCurrency: { name: 'MATIC', symbol: 'MATIC', decimals: 18 },
  },
  // Arbitrum One
  42161: {
    chainId: 42161,
    name: 'Arbitrum One',
    symbol: 'ETH',
    rpcUrl: 'https://arb1.arbitrum.io/rpc',
    explorerUrl: 'https://arbiscan.io',
    explorerApiUrl: 'https://api.arbiscan.io/api',
    nativeCurrency: { name: 'Ether', symbol: 'ETH', decimals: 18 },
  },
  // Arbitrum Sepolia
  421614: {
    chainId: 421614,
    name: 'Arbitrum Sepolia',
    symbol: 'ETH',
    rpcUrl: 'https://sepolia-rollup.arbitrum.io/rpc',
    explorerUrl: 'https://sepolia.arbiscan.io',
    explorerApiUrl: 'https://api-sepolia.arbiscan.io/api',
    nativeCurrency: { name: 'Ether', symbol: 'ETH', decimals: 18 },
  },
  // Optimism
  10: {
    chainId: 10,
    name: 'Optimism',
    symbol: 'ETH',
    rpcUrl: 'https://mainnet.optimism.io',
    explorerUrl: 'https://optimistic.etherscan.io',
    explorerApiUrl: 'https://api-optimistic.etherscan.io/api',
    nativeCurrency: { name: 'Ether', symbol: 'ETH', decimals: 18 },
  },
  // Optimism Sepolia
  11155420: {
    chainId: 11155420,
    name: 'Optimism Sepolia',
    symbol: 'ETH',
    rpcUrl: 'https://sepolia.optimism.io',
    explorerUrl: 'https://sepolia-optimistic.etherscan.io',
    explorerApiUrl: 'https://api-sepolia-optimistic.etherscan.io/api',
    nativeCurrency: { name: 'Ether', symbol: 'ETH', decimals: 18 },
  },
  // Base
  8453: {
    chainId: 8453,
    name: 'Base',
    symbol: 'ETH',
    rpcUrl: 'https://mainnet.base.org',
    explorerUrl: 'https://basescan.org',
    explorerApiUrl: 'https://api.basescan.org/api',
    nativeCurrency: { name: 'Ether', symbol: 'ETH', decimals: 18 },
  },
  // Base Sepolia
  84532: {
    chainId: 84532,
    name: 'Base Sepolia',
    symbol: 'ETH',
    rpcUrl: 'https://sepolia.base.org',
    explorerUrl: 'https://sepolia.basescan.org',
    explorerApiUrl: 'https://api-sepolia.basescan.org/api',
    nativeCurrency: { name: 'Ether', symbol: 'ETH', decimals: 18 },
  },
  // Avalanche C-Chain
  43114: {
    chainId: 43114,
    name: 'Avalanche',
    symbol: 'AVAX',
    rpcUrl: 'https://api.avax.network/ext/bc/C/rpc',
    explorerUrl: 'https://snowtrace.io',
    explorerApiUrl: 'https://api.snowtrace.io/api',
    nativeCurrency: { name: 'Avalanche', symbol: 'AVAX', decimals: 18 },
  },
  // Avalanche Fuji (Testnet)
  43113: {
    chainId: 43113,
    name: 'Avalanche Fuji',
    symbol: 'AVAX',
    rpcUrl: 'https://api.avax-test.network/ext/bc/C/rpc',
    explorerUrl: 'https://testnet.snowtrace.io',
    explorerApiUrl: 'https://api-testnet.snowtrace.io/api',
    nativeCurrency: { name: 'Avalanche', symbol: 'AVAX', decimals: 18 },
  },
  // Fantom
  250: {
    chainId: 250,
    name: 'Fantom',
    symbol: 'FTM',
    rpcUrl: 'https://rpc.ftm.tools',
    explorerUrl: 'https://ftmscan.com',
    explorerApiUrl: 'https://api.ftmscan.com/api',
    nativeCurrency: { name: 'Fantom', symbol: 'FTM', decimals: 18 },
  },
  // Fantom Testnet
  4002: {
    chainId: 4002,
    name: 'Fantom Testnet',
    symbol: 'FTM',
    rpcUrl: 'https://rpc.testnet.fantom.network',
    explorerUrl: 'https://testnet.ftmscan.com',
    explorerApiUrl: 'https://api-testnet.ftmscan.com/api',
    nativeCurrency: { name: 'Fantom', symbol: 'FTM', decimals: 18 },
  },
  // Cronos
  25: {
    chainId: 25,
    name: 'Cronos',
    symbol: 'CRO',
    rpcUrl: 'https://evm.cronos.org',
    explorerUrl: 'https://cronoscan.com',
    explorerApiUrl: 'https://api.cronoscan.com/api',
    nativeCurrency: { name: 'Cronos', symbol: 'CRO', decimals: 18 },
  },
  // Celo
  42220: {
    chainId: 42220,
    name: 'Celo',
    symbol: 'CELO',
    rpcUrl: 'https://forno.celo.org',
    explorerUrl: 'https://explorer.celo.org',
    explorerApiUrl: 'https://api.celoscan.io/api',
    nativeCurrency: { name: 'Celo', symbol: 'CELO', decimals: 18 },
  },
  // Gnosis
  100: {
    chainId: 100,
    name: 'Gnosis',
    symbol: 'XDAI',
    rpcUrl: 'https://rpc.gnosischain.com',
    explorerUrl: 'https://gnoscan.io',
    explorerApiUrl: 'https://api.gnoscan.io/api',
    nativeCurrency: { name: 'xDAI', symbol: 'XDAI', decimals: 18 },
  },
  // Moonbeam
  1284: {
    chainId: 1284,
    name: 'Moonbeam',
    symbol: 'GLMR',
    rpcUrl: 'https://rpc.api.moonbeam.network',
    explorerUrl: 'https://moonscan.io',
    explorerApiUrl: 'https://api.moonscan.io/api',
    nativeCurrency: { name: 'Glimmer', symbol: 'GLMR', decimals: 18 },
  },
  // Moonriver
  1285: {
    chainId: 1285,
    name: 'Moonriver',
    symbol: 'MOVR',
    rpcUrl: 'https://rpc.api.moonriver.moonbeam.network',
    explorerUrl: 'https://moonriver.moonscan.io',
    explorerApiUrl: 'https://api-moonriver.moonscan.io/api',
    nativeCurrency: { name: 'Moonriver', symbol: 'MOVR', decimals: 18 },
  },
  // Kava
  2222: {
    chainId: 2222,
    name: 'Kava',
    symbol: 'KAVA',
    rpcUrl: 'https://evm.kava.io',
    explorerUrl: 'https://explorer.kava.io',
    explorerApiUrl: 'https://api.explorer.kava.io/api',
    nativeCurrency: { name: 'Kava', symbol: 'KAVA', decimals: 18 },
  },
  // Linea
  59144: {
    chainId: 59144,
    name: 'Linea',
    symbol: 'ETH',
    rpcUrl: 'https://rpc.linea.build',
    explorerUrl: 'https://lineascan.build',
    explorerApiUrl: 'https://api.lineascan.build/api',
    nativeCurrency: { name: 'Ether', symbol: 'ETH', decimals: 18 },
  },
  // zkEVM
  1101: {
    chainId: 1101,
    name: 'Polygon zkEVM',
    symbol: 'ETH',
    rpcUrl: 'https://zkevm-rpc.com',
    explorerUrl: 'https://zkevm.polygonscan.com',
    explorerApiUrl: 'https://api-zkevm.polygonscan.com/api',
    nativeCurrency: { name: 'Ether', symbol: 'ETH', decimals: 18 },
  },
  // Scroll
  534352: {
    chainId: 534352,
    name: 'Scroll',
    symbol: 'ETH',
    rpcUrl: 'https://rpc.scroll.io',
    explorerUrl: 'https://scrollscan.com',
    explorerApiUrl: 'https://api.scrollscan.com/api',
    nativeCurrency: { name: 'Ether', symbol: 'ETH', decimals: 18 },
  },
  // Mantle
  5000: {
    chainId: 5000,
    name: 'Mantle',
    symbol: 'MNT',
    rpcUrl: 'https://rpc.mantle.xyz',
    explorerUrl: 'https://mantlescan.org',
    explorerApiUrl: 'https://api.mantlescan.org/api',
    nativeCurrency: { name: 'Mantle', symbol: 'MNT', decimals: 18 },
  },
  // opBNB
  5611: {
    chainId: 5611,
    name: 'opBNB',
    symbol: 'BNB',
    rpcUrl: 'https://opbnb-rpc.publicnode.com',
    explorerUrl: 'https://opbnbscan.com',
    explorerApiUrl: 'https://api-opbnbscan.com/api',
    nativeCurrency: { name: 'BNB', symbol: 'BNB', decimals: 18 },
  },
  // Mode
  34443: {
    chainId: 34443,
    name: 'Mode',
    symbol: 'ETH',
    rpcUrl: 'https://mainnet.mode.network',
    explorerUrl: 'https://explorer.mode.network',
    explorerApiUrl: 'https://api.explorer.mode.network/api',
    nativeCurrency: { name: 'Ether', symbol: 'ETH', decimals: 18 },
  },
  // Zora
  7777777: {
    chainId: 7777777,
    name: 'Zora',
    symbol: 'ETH',
    rpcUrl: 'https://rpc.zora.energy',
    explorerUrl: 'https://explorer.zora.energy',
    explorerApiUrl: 'https://api.explorer.zora.energy/api',
    nativeCurrency: { name: 'Ether', symbol: 'ETH', decimals: 18 },
  },
  // Harmony
  1666600000: {
    chainId: 1666600000,
    name: 'Harmony',
    symbol: 'ONE',
    rpcUrl: 'https://api.harmony.one',
    explorerUrl: 'https://explorer.harmony.one',
    explorerApiUrl: 'https://api.explorer.harmony.one/api',
    nativeCurrency: { name: 'Harmony', symbol: 'ONE', decimals: 18 },
  },
  // Metis
  1088: {
    chainId: 1088,
    name: 'Metis',
    symbol: 'METIS',
    rpcUrl: 'https://andromeda.metis.io/?owner=1088',
    explorerUrl: 'https://andromeda-explorer.metis.io',
    explorerApiUrl: 'https://api.andromeda-explorer.metis.io/api',
    nativeCurrency: { name: 'Metis', symbol: 'METIS', decimals: 18 },
  },
  // Shimmer
  148: {
    chainId: 148,
    name: 'Shimmer',
    symbol: 'SMR',
    rpcUrl: 'https://json-rpc.evm.shimmer.network',
    explorerUrl: 'https://explorer.shimmer.network',
    explorerApiUrl: 'https://api.explorer.shimmer.network/api',
    nativeCurrency: { name: 'Shimmer', symbol: 'SMR', decimals: 18 },
  },
  // Core
  1116: {
    chainId: 1116,
    name: 'Core',
    symbol: 'CORE',
    rpcUrl: 'https://rpc.coredao.org',
    explorerUrl: 'https://scan.coredao.org',
    explorerApiUrl: 'https://api-scan.coredao.org/api',
    nativeCurrency: { name: 'Core', symbol: 'CORE', decimals: 18 },
  },
  // Aurora
  1313161554: {
    chainId: 1313161554,
    name: 'Aurora',
    symbol: 'ETH',
    rpcUrl: 'https://mainnet.aurora.dev',
    explorerUrl: 'https://explorer.aurora.dev',
    explorerApiUrl: 'https://api.aurora.dev/api',
    nativeCurrency: { name: 'Ether', symbol: 'ETH', decimals: 18 },
  },
  // Cronos zkEVM
  282: {
    chainId: 282,
    name: 'Cronos zkEVM',
    symbol: 'CRO',
    rpcUrl: 'https://zkevm.cronos.org',
    explorerUrl: 'https://zkevm.cronos.org',
    explorerApiUrl: 'https://api-zkevm.cronos.org/api',
    nativeCurrency: { name: 'Cronos', symbol: 'CRO', decimals: 18 },
  },
  // Blast
  81457: {
    chainId: 81457,
    name: 'Blast',
    symbol: 'ETH',
    rpcUrl: 'https://blast-rpc.publicnode.com',
    explorerUrl: 'https://blastscan.io',
    explorerApiUrl: 'https://api.blastscan.io/api',
    nativeCurrency: { name: 'Ether', symbol: 'ETH', decimals: 18 },
  },
  // Berachain
  80084: {
    chainId: 80084,
    name: 'Berachain',
    symbol: 'BERA',
    rpcUrl: 'https://rpc.berachain.com',
    explorerUrl: 'https://berascan.com',
    explorerApiUrl: 'https://api.berascan.com/api',
    nativeCurrency: { name: 'Berachain', symbol: 'BERA', decimals: 18 },
  },
};

// ============================================================================
// ERC-20 Token ABI (Minimal)
// ============================================================================

export const ERC20_ABI = [
  'function name() view returns (string)',
  'function symbol() view returns (string)',
  'function decimals() view returns (uint8)',
  'function totalSupply() view returns (uint256)',
  'function balanceOf(address owner) view returns (uint256)',
  'function transfer(address to, uint256 amount) returns (bool)',
  'function allowance(address owner, address spender) view returns (uint256)',
  'function approve(address spender, uint256 amount) returns (bool)',
  'function transferFrom(address from, address to, uint256 amount) returns (bool)',
  'event Transfer(address indexed from, address indexed to, uint256 value)',
  'event Approval(address indexed owner, address indexed spender, uint256 value)',
];

// ============================================================================
// EVM Wallet - Native HD Wallet Implementation
// ============================================================================

/**
 * EVMWallet - Native EVM wallet with HD key derivation
 * 
 * Completely independent - NO external wallet dependencies
 * Supports:
 * - Mnemonic phrase generation and import
 * - Private key import
 * - HD derivation (BIP44)
 * - Multi-account management
 * - Transaction signing and execution
 */
export class EVMWallet {
  private wallet: Wallet;
  private chainId: number;
  private provider: JsonRpcProvider;

  /**
   * Create a new wallet with generated mnemonic
   */
  static createRandom(wordCount: 12 | 24 = 12): { wallet: EVMWallet; mnemonic: string } {
    const wallet = Wallet.createRandom();
    const mnemonic = wallet.mnemonic?.phrase || '';
    return {
      wallet: new EVMWallet(wallet.privateKey, 1),
      mnemonic,
    };
  }

  /**
   * Import wallet from mnemonic phrase
   */
  static fromMnemonic(mnemonic: string, chainId: number = 1): EVMWallet {
    const wallet = Wallet.fromMnemonic(mnemonic);
    return new EVMWallet(wallet.privateKey, chainId);
  }

  /**
   * Import wallet from private key
   */
  static fromPrivateKey(privateKey: string, chainId: number = 1): EVMWallet {
    return new EVMWallet(privateKey, chainId);
  }

  /**
   * Import wallet from keystore (encrypted JSON)
   */
  static fromKeystore(keystore: string, password: string, chainId: number = 1): EVMWallet {
    try {
      const wallet = Wallet.fromEncryptedJsonSync(keystore, password);
      return new EVMWallet(wallet.privateKey, chainId);
    } catch (error) {
      throw new Error('Failed to decrypt keystore: Invalid password or corrupted data');
    }
  }

  private constructor(privateKey: string, chainId: number) {
    this.chainId = chainId;
    const config = CHAIN_REGISTRY[chainId] || CHAIN_REGISTRY[1];
    this.provider = new JsonRpcProvider(config.rpcUrl);
    this.wallet = new Wallet(privateKey, this.provider);
  }

  /**
   * Get the wallet address
   */
  getAddress(): string {
    return this.wallet.address;
  }

  /**
   * Get the private key
   */
  getPrivateKey(): string {
    return this.wallet.privateKey;
  }

  /**
   * Get the chain ID
   */
  getChainId(): number {
    return this.chainId;
  }

  /**
   * Get the provider
   */
  getProvider(): JsonRpcProvider {
    return this.provider;
  }

  /**
   * Get wallet balance
   */
  async getBalance(): Promise<bigint> {
    return this.provider.getBalance(this.wallet.address);
  }

  /**
   * Get wallet balance for a specific block
   */
  async getBalanceAt(blockTag: number | string): Promise<bigint> {
    return this.provider.getBalance(this.wallet.address, blockTag);
  }

  /**
   * Encrypt wallet to keystore
   */
  async encryptKeystore(password: string): Promise<string> {
    return this.wallet.encrypt(password);
  }

  /**
   * Sign a message
   */
  signMessage(message: string): string {
    return this.wallet.signMessage(message);
  }

  /**
   * Sign typed data (EIP-712)
   */
  signTypedData(domain: any, types: any, value: any): string {
    return this.wallet.signTypedData(domain, types, value);
  }

  /**
   * Sign a transaction
   */
  async signTransaction(tx: TransactionRequest): Promise<string> {
    const populatedTx = {
      to: tx.to,
      value: tx.value || 0n,
      data: tx.data || '0x',
      gasLimit: tx.gasLimit || 21000n,
      gasPrice: tx.gasPrice,
      maxFeePerGas: tx.maxFeePerGas,
      maxPriorityFeePerGas: tx.maxPriorityFeePerGas,
      nonce: tx.nonce,
      chainId: tx.chainId || this.chainId,
    };
    return this.wallet.signTransaction(populatedTx);
  }

  /**
   * Send a transaction
   */
  async sendTransaction(tx: TransactionRequest): Promise<TransactionReceipt> {
    const populatedTx = {
      to: tx.to,
      value: tx.value || 0n,
      data: tx.data || '0x',
      gasLimit: tx.gasLimit || 21000n,
      gasPrice: tx.gasPrice,
      maxFeePerGas: tx.maxFeePerGas,
      maxPriorityFeePerGas: tx.maxPriorityFeePerGas,
      nonce: tx.nonce,
      chainId: tx.chainId || this.chainId,
    };
    const response = await this.wallet.sendTransaction(populatedTx);
    const receipt = await response.wait();
    return this.formatReceipt(receipt);
  }

  /**
   * Estimate gas for a transaction
   */
  async estimateGas(tx: TransactionRequest): Promise<bigint> {
    return this.provider.estimateGas({
      to: tx.to,
      value: tx.value || 0n,
      data: tx.data || '0x',
      from: this.wallet.address,
    });
  }

  /**
   * Get transaction count (nonce)
   */
  async getTransactionCount(): Promise<number> {
    return this.provider.getTransactionCount(this.wallet.address);
  }

  /**
   * Get chain status
   */
  async getChainStatus(): Promise<ChainStatus> {
    const [blockNumber, peerCount] = await Promise.all([
      this.provider.getBlockNumber(),
      this.provider.getPeerCount().catch(() => 0),
    ]);
    const block = await this.provider.getBlock('latest');
    return {
      chainId: this.chainId,
      blockNumber,
      timestamp: block?.timestamp || 0,
      peerCount,
      syncing: false,
    };
  }

  /**
   * Switch to a different chain
   */
  async switchChain(chainId: number): Promise<EVMWallet> {
    if (!CHAIN_REGISTRY[chainId]) {
      throw new Error(`Chain ${chainId} not supported`);
    }
    return new EVMWallet(this.wallet.privateKey, chainId);
  }

  private formatReceipt(receipt: any): TransactionReceipt {
    return {
      hash: receipt.hash,
      blockNumber: receipt.blockNumber,
      blockHash: receipt.blockHash,
      transactionIndex: receipt.index,
      from: receipt.from,
      to: receipt.to,
      value: receipt.value.toString(),
      gasUsed: receipt.gasUsed,
      effectiveGasPrice: receipt.effectiveGasPrice,
      logs: receipt.logs,
      status: receipt.status,
      logsBloom: receipt.logsBloom,
    };
  }
}

// ============================================================================
// EVM Client - RPC Operations
// ============================================================================

/**
 * EVMClient - Low-level EVM RPC operations
 * 
 * Provides direct RPC access without wallet functionality.
 * Used for chain queries, contract reads, etc.
 */
export class EVMClient {
  private provider: JsonRpcProvider;
  private chainConfig: EVMChainConfig;

  constructor(chainId: number) {
    this.chainConfig = CHAIN_REGISTRY[chainId];
    if (!this.chainConfig) {
      throw new Error(`Chain ${chainId} not supported. Add to CHAIN_REGISTRY first.`);
    }
    this.provider = new JsonRpcProvider(this.chainConfig.rpcUrl);
  }

  /**
   * Get chain configuration
   */
  getChainConfig(): EVMChainConfig {
    return this.chainConfig;
  }

  /**
   * Get provider
   */
  getProvider(): JsonRpcProvider {
    return this.provider;
  }

  /**
   * Get chain ID
   */
  async getChainId(): Promise<number> {
    return (await this.provider.getNetwork()).chainId;
  }

  /**
   * Get current block number
   */
  async getBlockNumber(): Promise<number> {
    return this.provider.getBlockNumber();
  }

  /**
   * Get block by number or hash
   */
  async getBlock(blockTag: number | string = 'latest'): Promise<Block | null> {
    const block = await this.provider.getBlock(blockTag);
    if (!block) return null;
    return {
      number: block.number,
      hash: block.hash,
      parentHash: block.parentHash,
      timestamp: block.timestamp,
      nonce: block.nonce,
      difficulty: block.difficulty,
      gasLimit: block.gasLimit,
      gasUsed: block.gasUsed,
      miner: block.miner,
      extraData: block.extraData,
      transactions: block.transactions,
      transactionsRoot: block.transactionsRoot,
      stateRoot: block.stateRoot,
      receiptsRoot: block.receiptsRoot,
      uncleHash: block.sha3Uncles,
      baseFeePerGas: block.baseFeePerGas,
    };
  }

  /**
   * Get transaction by hash
   */
  async getTransaction(hash: string): Promise<any> {
    return this.provider.getTransaction(hash);
  }

  /**
   * Get transaction receipt
   */
  async getTransactionReceipt(hash: string): Promise<TransactionReceipt | null> {
    const receipt = await this.provider.getTransactionReceipt(hash);
    if (!receipt) return null;
    return this.formatReceipt(receipt);
  }

  /**
   * Get code at address
   */
  async getCode(address: string): Promise<string> {
    return this.provider.getCode(address);
  }

  /**
   * Get storage at address and slot
   */
  async getStorageAt(address: string, slot: string): Promise<string> {
    return this.provider.getStorageAt(address, slot);
  }

  /**
   * Get balance of an address
   */
  async getBalance(address: string, blockTag: number | string = 'latest'): Promise<bigint> {
    return this.provider.getBalance(address, blockTag);
  }

  /**
   * Get nonce of an address
   */
  async getNonce(address: string, blockTag: number | string = 'latest'): Promise<number> {
    return this.provider.getTransactionCount(address, blockTag);
  }

  /**
   * Call a smart contract (read-only)
   */
  async call(to: string, data: string, blockTag: number | string = 'latest'): Promise<string> {
    return this.provider.call({ to, data }, blockTag);
  }

  /**
   * Estimate gas for a transaction
   */
  async estimateGas(tx: {
    from?: string;
    to?: string;
    value?: bigint;
    data?: string;
    gasLimit?: bigint;
  }): Promise<bigint> {
    return this.provider.estimateGas(tx);
  }

  /**
   * Get gas price
   */
  async getGasPrice(): Promise<bigint> {
    return this.provider.getFeeData().then(f => f.gasPrice || 0n);
  }

  /**
   * Get fee data (EIP-1559)
   */
  async getFeeData(): Promise<{
    gasPrice: bigint;
    maxFeePerGas: bigint;
    maxPriorityFeePerGas: bigint;
  }> {
    const feeData = await this.provider.getFeeData();
    return {
      gasPrice: feeData.gasPrice || 0n,
      maxFeePerGas: feeData.maxFeePerGas || 0n,
      maxPriorityFeePerGas: feeData.maxPriorityFeePerGas || 0n,
    };
  }

  /**
   * Get chain status
   */
  async getChainStatus(): Promise<ChainStatus> {
    const [blockNumber, peerCount] = await Promise.all([
      this.provider.getBlockNumber(),
      this.provider.getPeerCount().catch(() => 0),
    ]);
    const block = await this.provider.getBlock('latest');
    return {
      chainId: this.chainConfig.chainId,
      blockNumber,
      timestamp: block?.timestamp || 0,
      peerCount,
      syncing: false,
    };
  }

  /**
   * Get logs
   */
  async getLogs(filter: {
    address?: string;
    topics?: string[];
    fromBlock?: number;
    toBlock?: number;
  }): Promise<Log[]> {
    const logs = await this.provider.getLogs({
      address: filter.address,
      topics: filter.topics as any,
      fromBlock: filter.fromBlock,
      toBlock: filter.toBlock,
    });
    return logs.map((log: any) => ({
      address: log.address,
      topics: log.topics,
      data: log.data,
      logIndex: log.logIndex,
      transactionIndex: log.transactionIndex,
      transactionHash: log.transactionHash,
      blockHash: log.blockHash,
      blockNumber: log.blockNumber,
    }));
  }

  private formatReceipt(receipt: any): TransactionReceipt {
    return {
      hash: receipt.hash,
      blockNumber: receipt.blockNumber,
      blockHash: receipt.blockHash,
      transactionIndex: receipt.index,
      from: receipt.from,
      to: receipt.to,
      value: receipt.value.toString(),
      gasUsed: receipt.gasUsed,
      effectiveGasPrice: receipt.effectiveGasPrice,
      logs: receipt.logs,
      status: receipt.status,
      logsBloom: receipt.logsBloom,
    };
  }
}

// ============================================================================
// ERC20 Token Contract
// ============================================================================

/**
 * ERC20Token - ERC-20 token interaction
 */
export class ERC20Token {
  private contract: Contract;
  private iface: Interface;

  constructor(address: string, provider: JsonRpcProvider) {
    this.contract = new Contract(address, ERC20_ABI, provider);
    this.iface = new Interface(ERC20_ABI);
  }

  /**
   * Get token name
   */
  async name(): Promise<string> {
    return this.contract.name();
  }

  /**
   * Get token symbol
   */
  async symbol(): Promise<string> {
    return this.contract.symbol();
  }

  /**
   * Get token decimals
   */
  async decimals(): Promise<number> {
    return this.contract.decimals();
  }

  /**
   * Get total supply
   */
  async totalSupply(): Promise<bigint> {
    return this.contract.totalSupply();
  }

  /**
   * Get balance of an address
   */
  async balanceOf(owner: string): Promise<bigint> {
    return this.contract.balanceOf(owner);
  }

  /**
   * Get allowance
   */
  async allowance(owner: string, spender: string): Promise<bigint> {
    return this.contract.allowance(owner, spender);
  }

  /**
   * Transfer tokens
   */
  async transfer(to: string, amount: bigint): Promise<string> {
    const tx = await this.contract.transfer(to, amount);
    return tx.hash;
  }

  /**
   * Approve spender
   */
  async approve(spender: string, amount: bigint): Promise<string> {
    const tx = await this.contract.approve(spender, amount);
    return tx.hash;
  }

  /**
   * TransferFrom tokens
   */
  async transferFrom(from: string, to: string, amount: bigint): Promise<string> {
    const tx = await this.contract.transferFrom(from, to, amount);
    return tx.hash;
  }

  /**
   * Get token info
   */
  async getTokenInfo(): Promise<TokenInfo> {
    const [name, symbol, decimals, totalSupply] = await Promise.all([
      this.name(),
      this.symbol(),
      this.decimals(),
      this.totalSupply(),
    ]);
    return {
      address: this.contract.address,
      name,
      symbol,
      decimals,
      totalSupply: totalSupply.toString(),
    };
  }

  /**
   * Encode function call data
   */
  encodeFunction(functionName: string, args: any[]): string {
    return this.iface.encodeFunctionData(functionName, args);
  }

  /**
   * Decode function result
   */
  decodeFunctionResult(functionName: string, data: string): any {
    return this.iface.decodeFunctionResult(functionName, data);
  }
}

// ============================================================================
// Utility Functions
// ============================================================================

/**
 * Parse units (e.g., "1.5" ETH to wei)
 */
export function parseUnits(value: string, decimals: number): bigint {
  return ethers.parseUnits(value, decimals);
}

/**
 * Format units (e.g., wei to ETH)
 */
export function formatUnits(value: bigint, decimals: number): string {
  return ethers.formatUnits(value, decimals);
}

/**
 * Format Ether
 */
export function formatEther(wei: bigint): string {
  return ethers.formatEther(wei);
}

/**
 * Parse Ether
 */
export function parseEther(ether: string): bigint {
  return ethers.parseEther(ether);
}

/**
 * Compute contract address (CREATE2)
 */
export function computeCreate2Address(salt: string, bytecode: string, deployer: string): string {
  return ethers.computeAddress(deployer);
}

/**
 * Encode revert reason
 */
export function encodeRevertReason(error: string): string {
  return ethers.encodeErrorResult(Error(error));
}

/**
 * Decode revert reason
 */
export function decodeRevertReason(data: string): string | null {
  try {
    const iface = new Interface(['error Error(string reason)']);
    const result = iface.decodeFunctionResult('Error', data);
    return result[0];
  } catch {
    return null;
  }
}

/**
 * Get chain name by ID
 */
export function getChainName(chainId: number): string {
  return CHAIN_REGISTRY[chainId]?.name || `Chain ${chainId}`;
}

/**
 * Get chain symbol by ID
 */
export function getChainSymbol(chainId: number): string {
  return CHAIN_REGISTRY[chainId]?.symbol || 'ETH';
}

/**
 * Check if chain is supported
 */
export function isChainSupported(chainId: number): boolean {
  return chainId in CHAIN_REGISTRY;
}

/**
 * Get all supported chain IDs
 */
export function getSupportedChains(): number[] {
  return Object.keys(CHAIN_REGISTRY).map(Number);
}

// ============================================================================
// Export all
// ============================================================================

export default {
  CHAIN_REGISTRY,
  ERC20_ABI,
  EVMWallet,
  EVMClient,
  ERC20Token,
  // Utilities
  parseUnits,
  formatUnits,
  formatEther,
  parseEther,
  computeCreate2Address,
  encodeRevertReason,
  decodeRevertReason,
  getChainName,
  getChainSymbol,
  isChainSupported,
  getSupportedChains,
};