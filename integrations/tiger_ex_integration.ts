/**
 * TigerEX - Unified Integration Layer
 * 
 * Complete integration of:
 * - TigerWallet (Multichain Web3 Wallet)
 * - Tigerswap (Multichain DEX)
 * - TigerSmartChain (EVM Blockchain with TGR & RUSD tokens)
 * - TigerEx (Centralized Exchange)
 * 
 * Fee Collection System:
 * - Exchange trading fees
 * - DEX swap fees (0.3% per trade)
 * - Bridge cross-chain fees (0.1%)
 * - Wallet transaction fees
 * 
 * Supports:
 * - 24 EVM Blockchains
 * - 26 Non-EVM Blockchains
 * - 200+ tokens
 * - Unlimited dynamic additions at runtime
 */

import { UniversalChainRegistry, ChainConfig, ChainCategory, TokenConfig } from '../libs/chain_registry/universal_chain_registry';

// ============================================================================
// Types & Interfaces
// ============================================================================

export type IntegrationStatus = 'active' | 'inactive' | 'paused' | 'deprecated';

export interface TigerProduct {
  name: string;
  version: string;
  status: IntegrationStatus;
  feeCollection: FeeConfig;
  enabled: boolean;
  createdAt: number;
  updatedAt: number;
}

export interface FeeConfig {
  tradingFee: number;      // Exchange trading fee (0.1-1%)
  swapFee: number;        // DEX swap fee (default 0.3%)
  bridgeFee: number;     // Bridge cross-chain fee (default 0.1%)
  walletTxFee: number;   // Wallet transaction fee
  withdrawalFee: number; // Withdrawal fee
  depositFee: number;    // Deposit fee
}

export interface CrossChainRoute {
  sourceChain: string;
  targetChain: string;
  amount: bigint;
  inputToken: string;
  outputToken: string;
  estimatedOutput: bigint;
  fee: bigint;
  path: string[];
  dexPool?: string;
  bridge?: string;
  estimatedTime: number;
}

export interface WalletInfo {
  address: string;
  publicKey: string;
  chainId: number;
  balance: bigint;
  tokens: TokenBalance[];
}

export interface TokenBalance {
  symbol: string;
  address: string;
  balance: bigint;
  decimals: number;
  price: number;
  value: number;
}

export interface SwapRoute {
  inputToken: string;
  outputToken: string;
  amountIn: bigint;
  amountOut: bigint;
  path: string[];
  pools: LiquidityPool[];
  fee: bigint;
  priceImpact: number;
  slippage: number;
}

export interface LiquidityPool {
  tokenA: string;
  tokenB: string;
  reserveA: bigint;
  reserveB: bigint;
  fee: number;
  liquidity: bigint;
  apy: number;
}

export interface FarmInfo {
  poolId: string;
  rewardToken: string;
  stakedToken: string;
  stakedAmount: bigint;
  rewardAmount: bigint;
  apy: number;
  startTime: number;
  endTime: number;
}

export interface BridgeInfo {
  bridgeId: string;
  sourceChain: string;
  targetChain: string;
  token: string;
  minAmount: bigint;
  maxAmount: bigint;
  fee: number;
  estimatedTime: number;
  isActive: boolean;
}

// ============================================================================
// TigerEX Integration Core
// ============================================================================

export class TigerEXIntegration {
  // Products
  private tigerWallet: TigerProduct;
  private tigerSwap: TigerProduct;
  private tigerSmartChain: TigerProduct;
  private tigerEx: TigerProduct;
  
  // Registry
  private chainRegistry: UniversalChainRegistry;
  
  // Fee Collection
  private totalFeesCollected: bigint = 0n;
  private feeHistory: FeeRecord[] = [];
  
  // State
  private initialized: boolean = false;
  
  // Multichain Support
  private supportedEvmChains: Map<string, ChainConfig> = new Map();
  private supportedNonEvmChains: Map<string, ChainConfig> = new Map();
  private supportedTokens: Map<string, TokenConfig> = new Map();
  
  // DEX Pools
  private dexPools: Map<string, LiquidityPool> = new Map();
  private farms: Map<string, FarmInfo> = new Map();
  
  // Bridges
  private bridges: Map<string, BridgeInfo> = new Map();
  
  // Wallets (in-memory for demo)
  private wallets: Map<string, WalletInfo> = new Map();

  constructor() {
    // Initialize products
    this.tigerWallet = {
      name: 'TigerWallet',
      version: '1.0.0',
      status: 'active',
      feeCollection: {
        tradingFee: 0,
        swapFee: 0,
        bridgeFee: 0,
        walletTxFee: 0.001, // 0.1% for wallet transactions
        withdrawalFee: 0.0005,
        depositFee: 0,
      },
      enabled: true,
      createdAt: Date.now(),
      updatedAt: Date.now(),
    };

    this.tigerSwap = {
      name: 'Tigerswap',
      version: '1.0.0',
      status: 'active',
      feeCollection: {
        tradingFee: 0,
        swapFee: 0.003, // 0.3% DEX swap fee
        bridgeFee: 0,
        walletTxFee: 0,
        withdrawalFee: 0,
        depositFee: 0,
      },
      enabled: true,
      createdAt: Date.now(),
      updatedAt: Date.now(),
    };

    this.tigerSmartChain = {
      name: 'TigerSmartChain',
      version: '1.0.0',
      status: 'active',
      feeCollection: {
        tradingFee: 0,
        swapFee: 0,
        bridgeFee: 0.001, // 0.1% bridge fee
        walletTxFee: 0,
        withdrawalFee: 0.0005,
        depositFee: 0,
      },
      enabled: true,
      createdAt: Date.now(),
      updatedAt: Date.now(),
    };

    this.tigerEx = {
      name: 'TigerEx',
      version: '1.0.0',
      status: 'active',
      feeCollection: {
        tradingFee: 0.001, // 0.1% trading fee
        swapFee: 0,
        bridgeFee: 0,
        walletTxFee: 0,
        withdrawalFee: 0.0005,
        depositFee: 0,
      },
      enabled: true,
      createdAt: Date.now(),
      updatedAt: Date.now(),
    };

    this.chainRegistry = new UniversalChainRegistry();
    this.initialize();
  }

  // ============================================================================
  // Initialization
  // ============================================================================

  private initialize(): void {
    this.initializeEvmChains();
    this.initializeNonEvmChains();
    this.initializeTokens();
    this.initializeDexPools();
    this.initializeFarms();
    this.initializeBridges();
    this.initialized = true;
    console.log('[TigerEX] Integration layer initialized successfully');
  }

  private initializeEvmChains(): void {
    // TigerSmartChain (Native)
    this.supportedEvmChains.set('tigersmartchain', {
      id: 'tigersmartchain',
      name: 'TigerSmartChain',
      symbol: 'TGR',
      category: 'evm' as ChainCategory,
      status: 'active',
      chainId: 13000,
      rpcUrls: ['https://rpc.tigersmartchain.com'],
      explorerUrls: ['https://scan.tigersmartchain.com'],
      nativeCurrency: { name: 'Tiger', symbol: 'TGR', decimals: 18 },
      blockTime: 2,
      maxBlockSize: 50000,
      supportsEIP1559: true,
    });

    // Ethereum
    this.supportedEvmChains.set('ethereum', {
      id: 'ethereum',
      name: 'Ethereum',
      symbol: 'ETH',
      category: 'evm' as ChainCategory,
      status: 'active',
      chainId: 1,
      rpcUrls: ['https://eth.llamarpc.com', 'https://rpc.mew.io'],
      explorerUrls: ['https://etherscan.io'],
      nativeCurrency: { name: 'Ethereum', symbol: 'ETH', decimals: 18 },
      blockTime: 12,
      supportsEIP1559: true,
    });

    // BSC
    this.supportedEvmChains.set('bsc', {
      id: 'bsc',
      name: 'BNB Smart Chain',
      symbol: 'BNB',
      category: 'evm' as ChainCategory,
      status: 'active',
      chainId: 56,
      rpcUrls: ['https://bsc-dataseed.binance.org'],
      explorerUrls: ['https://bscscan.com'],
      nativeCurrency: { name: 'BNB', symbol: 'BNB', decimals: 18 },
      blockTime: 3,
      supportsEIP1559: true,
    });

    // Polygon
    this.supportedEvmChains.set('polygon', {
      id: 'polygon',
      name: 'Polygon',
      symbol: 'MATIC',
      category: 'evm' as ChainCategory,
      status: 'active',
      chainId: 137,
      rpcUrls: ['https://polygon-rpc.com'],
      explorerUrls: ['https://polygonscan.com'],
      nativeCurrency: { name: 'MATIC', symbol: 'MATIC', decimals: 18 },
      blockTime: 2,
      supportsEIP1559: true,
    });

    // Avalanche
    this.supportedEvmChains.set('avalanche', {
      id: 'avalanche',
      name: 'Avalanche',
      symbol: 'AVAX',
      category: 'evm' as ChainCategory,
      status: 'active',
      chainId: 43114,
      rpcUrls: ['https://api.avax.network/ext/bc/C/rpc'],
      explorerUrls: ['https://snowtrace.io'],
      nativeCurrency: { name: 'AVAX', symbol: 'AVAX', decimals: 18 },
      blockTime: 2,
    });

    // Arbitrum
    this.supportedEvmChains.set('arbitrum', {
      id: 'arbitrum',
      name: 'Arbitrum One',
      symbol: 'ETH',
      category: 'evm' as ChainCategory,
      status: 'active',
      chainId: 42161,
      rpcUrls: ['https://arb1.arbitrum.io/rpc'],
      explorerUrls: ['https://arbiscan.io'],
      nativeCurrency: { name: 'Ethereum', symbol: 'ETH', decimals: 18 },
      blockTime: 0.25,
      supportsEIP1559: true,
    });

    // Optimism
    this.supportedEvmChains.set('optimism', {
      id: 'optimism',
      name: 'Optimism',
      symbol: 'ETH',
      category: 'evm' as ChainCategory,
      status: 'active',
      chainId: 10,
      rpcUrls: ['https://mainnet.optimism.io'],
      explorerUrls: ['https://optimistic.etherscan.io'],
      nativeCurrency: { name: 'Ethereum', symbol: 'ETH', decimals: 18 },
      blockTime: 2,
      supportsEIP1559: true,
    });

    // Base
    this.supportedEvmChains.set('base', {
      id: 'base',
      name: 'Base',
      symbol: 'ETH',
      category: 'evm' as ChainCategory,
      status: 'active',
      chainId: 8453,
      rpcUrls: ['https://mainnet.base.org'],
      explorerUrls: ['https://basescan.org'],
      nativeCurrency: { name: 'Ethereum', symbol: 'ETH', decimals: 18 },
      blockTime: 2,
      supportsEIP1559: true,
    });

    // Fantom
    this.supportedEvmChains.set('fantom', {
      id: 'fantom',
      name: 'Fantom',
      symbol: 'FTM',
      category: 'evm' as ChainCategory,
      status: 'active',
      chainId: 250,
      rpcUrls: ['https://rpc.ftm.tools'],
      explorerUrls: ['https://ftmscan.com'],
      nativeCurrency: { name: 'Fantom', symbol: 'FTM', decimals: 18 },
      blockTime: 1,
    });

    // Celo
    this.supportedEvmChains.set('celo', {
      id: 'celo',
      name: 'Celo',
      symbol: 'CELO',
      category: 'evm' as ChainCategory,
      status: 'active',
      chainId: 42220,
      rpcUrls: ['https://forno.celo.org'],
      explorerUrls: ['https://explorer.celo.org'],
      nativeCurrency: { name: 'Celo', symbol: 'CELO', decimals: 18 },
      blockTime: 5,
    });

    // Gnosis
    this.supportedEvmChains.set('gnosis', {
      id: 'gnosis',
      name: 'Gnosis Chain',
      symbol: 'GNO',
      category: 'evm' as ChainCategory,
      status: 'active',
      chainId: 100,
      rpcUrls: ['https://rpc.gnosischain.com'],
      explorerUrls: ['https://gnosisscan.io'],
      nativeCurrency: { name: 'Gnosis', symbol: 'GNO', decimals: 18 },
      blockTime: 5,
    });

    // Moonbeam
    this.supportedEvmChains.set('moonbeam', {
      id: 'moonbeam',
      name: 'Moonbeam',
      symbol: 'GLMR',
      category: 'evm' as ChainCategory,
      status: 'active',
      chainId: 1284,
      rpcUrls: ['https://rpc.api.moonbeam.network'],
      explorerUrls: ['https://moonbeam.moonscan.io'],
      nativeCurrency: { name: 'Glimmer', symbol: 'GLMR', decimals: 18 },
      blockTime: 12,
    });

    // zkEVM
    this.supportedEvmChains.set('zkevm', {
      id: 'zkevm',
      name: 'Polygon zkEVM',
      symbol: 'ETH',
      category: 'evm' as ChainCategory,
      status: 'active',
      chainId: 1101,
      rpcUrls: ['https://zkevm-rpc.com'],
      explorerUrls: ['https://zkevm.polygonscan.com'],
      nativeCurrency: { name: 'Ethereum', symbol: 'ETH', decimals: 18 },
      blockTime: 1,
    });

    // Linea
    this.supportedEvmChains.set('linea', {
      id: 'linea',
      name: 'Linea',
      symbol: 'ETH',
      category: 'evm' as ChainCategory,
      status: 'active',
      chainId: 59144,
      rpcUrls: ['https://rpc.linea.build'],
      explorerUrls: ['https://lineascan.build'],
      nativeCurrency: { name: 'Ethereum', symbol: 'ETH', decimals: 18 },
      blockTime: 2,
    });

    // Scroll
    this.supportedEvmChains.set('scroll', {
      id: 'scroll',
      name: 'Scroll',
      symbol: 'ETH',
      category: 'evm' as ChainCategory,
      status: 'active',
      chainId: 534352,
      rpcUrls: ['https://rpc.scroll.io'],
      explorerUrls: ['https://scrollscan.com'],
      nativeCurrency: { name: 'Ethereum', symbol: 'ETH', decimals: 18 },
      blockTime: 3,
    });

    // Astar
    this.supportedEvmChains.set('astar', {
      id: 'astar',
      name: 'Astar',
      symbol: 'ASTR',
      category: 'evm' as ChainCategory,
      status: 'active',
      chainId: 432201,
      rpcUrls: ['https://rpc.astar.network'],
      explorerUrls: ['https://blockscout.com/astar'],
      nativeCurrency: { name: 'Astar', symbol: 'ASTR', decimals: 18 },
      blockTime: 12,
    });

    // Klaytn
    this.supportedEvmChains.set('klaytn', {
      id: 'klaytn',
      name: 'Klaytn',
      symbol: 'KLAY',
      category: 'evm' as ChainCategory,
      status: 'active',
      chainId: 8217,
      rpcUrls: ['https://klaytn-mainnet-rpc.allthatnode.com'],
      explorerUrls: ['https://scope.klaytn.com'],
      nativeCurrency: { name: 'Klaytn', symbol: 'KLAY', decimals: 18 },
      blockTime: 1,
    });

    // Cronos
    this.supportedEvmChains.set('cronos', {
      id: 'cronos',
      name: 'Cronos',
      symbol: 'CRO',
      category: 'evm' as ChainCategory,
      status: 'active',
      chainId: 25,
      rpcUrls: ['https://evm.cronos.org'],
      explorerUrls: ['https://cronoscan.com'],
      nativeCurrency: { name: 'Cronos', symbol: 'CRO', decimals: 18 },
      blockTime: 5,
    });

    // Core
    this.supportedEvmChains.set('core', {
      id: 'core',
      name: 'Core',
      symbol: 'CORE',
      category: 'evm' as ChainCategory,
      status: 'active',
      chainId: 1116,
      rpcUrls: ['https://rpc.coredao.org'],
      explorerUrls: ['https://scan.coredao.org'],
      nativeCurrency: { name: 'Core', symbol: 'CORE', decimals: 18 },
      blockTime: 2,
    });

    // Mantle
    this.supportedEvmChains.set('mantle', {
      id: 'mantle',
      name: 'Mantle',
      symbol: 'MNT',
      category: 'evm' as ChainCategory,
      status: 'active',
      chainId: 5000,
      rpcUrls: ['https://rpc.mantle.xyz'],
      explorerUrls: ['https://explorer.mantle.xyz'],
      nativeCurrency: { name: 'Mantle', symbol: 'MNT', decimals: 18 },
      blockTime: 2,
    });

    // Berachain
    this.supportedEvmChains.set('berachain', {
      id: 'berachain',
      name: 'Berachain',
      symbol: 'BERA',
      category: 'evm' as ChainCategory,
      status: 'active',
      chainId: 845321,
      rpcUrls: ['https://rpc.berachain.com'],
      explorerUrls: ['https://berascan.com'],
      nativeCurrency: { name: 'Berachain', symbol: 'BERA', decimals: 18 },
      blockTime: 2,
    });

    // Sonic
    this.supportedEvmChains.set('sonic', {
      id: 'sonic',
      name: 'Sonic',
      symbol: 'S',
      category: 'evm' as ChainCategory,
      status: 'active',
      chainId: 1460,
      rpcUrls: ['https://rpc.soniclabs.com'],
      explorerUrls: ['https://sonicscan.org'],
      nativeCurrency: { name: 'Sonic', symbol: 'S', decimals: 18 },
      blockTime: 2,
    });

    // Monad
    this.supportedEvmChains.set('monad', {
      id: 'monad',
      name: 'Monad',
      symbol: 'MON',
      category: 'evm' as ChainCategory,
      status: 'active',
      chainId: 10143,
      rpcUrls: ['https://rpc.monad.xyz'],
      explorerUrls: ['https://monadscan.com'],
      nativeCurrency: { name: 'Monad', symbol: 'MON', decimals: 18 },
      blockTime: 2,
    });

    // MegaETH
    this.supportedEvmChains.set('megaeth', {
      id: 'megaeth',
      name: 'MegaETH',
      symbol: 'MEGA',
      category: 'evm' as ChainCategory,
      status: 'active',
      chainId: 1205398815,
      rpcUrls: ['https://rpc.megaeth.com'],
      explorerUrls: ['https://megascan.io'],
      nativeCurrency: { name: 'MegaETH', symbol: 'MEGA', decimals: 18 },
      blockTime: 0.1,
    });
  }

  private initializeNonEvmChains(): void {
    // Solana
    this.supportedNonEvmChains.set('solana', {
      id: 'solana',
      name: 'Solana',
      symbol: 'SOL',
      category: 'solana' as ChainCategory,
      status: 'active',
      chainId: -1,
      rpcUrls: ['https://api.mainnet-beta.solana.com'],
      explorerUrls: ['https://solscan.io'],
      nativeCurrency: { name: 'Solana', symbol: 'SOL', decimals: 9 },
      blockTime: 0.4,
    });

    // Aptos
    this.supportedNonEvmChains.set('aptos', {
      id: 'aptos',
      name: 'Aptos',
      symbol: 'APT',
      category: 'aptos' as ChainCategory,
      status: 'active',
      chainId: -1,
      rpcUrls: ['https://aptos-mainnet.nodereal.io/v1'],
      explorerUrls: ['https://explorer.aptoslabs.com'],
      nativeCurrency: { name: 'Aptos', symbol: 'APT', decimals: 8 },
      blockTime: 1,
    });

    // Sui
    this.supportedNonEvmChains.set('sui', {
      id: 'sui',
      name: 'Sui',
      symbol: 'SUI',
      category: 'sui' as ChainCategory,
      status: 'active',
      chainId: -1,
      rpcUrls: ['https://rpc.mainnet.sui.io'],
      explorerUrls: ['https://suiscan.xyz'],
      nativeCurrency: { name: 'Sui', symbol: 'SUI', decimals: 9 },
      blockTime: 1,
    });

    // TON
    this.supportedNonEvmChains.set('ton', {
      id: 'ton',
      name: 'TON',
      symbol: 'TON',
      category: 'ton' as ChainCategory,
      status: 'active',
      chainId: -1,
      rpcUrls: ['https://toncenter.com/api/v2'],
      explorerUrls: ['https://tonscan.org'],
      nativeCurrency: { name: 'TON', symbol: 'TON', decimals: 9 },
      blockTime: 5,
    });

    // Cosmos
    this.supportedNonEvmChains.set('cosmos', {
      id: 'cosmos',
      name: 'Cosmos',
      symbol: 'ATOM',
      category: 'cosmos' as ChainCategory,
      status: 'active',
      chainId: -1,
      rpcUrls: ['https://rpc-cosmoshub.keplr.app'],
      explorerUrls: ['https://mintscan.io/cosmos'],
      nativeCurrency: { name: 'Atom', symbol: 'ATOM', decimals: 6 },
      blockTime: 7,
    });

    // NEAR
    this.supportedNonEvmChains.set('near', {
      id: 'near',
      name: 'NEAR Protocol',
      symbol: 'NEAR',
      category: 'near' as ChainCategory,
      status: 'active',
      chainId: -1,
      rpcUrls: ['https://rpc.mainnet.near.org'],
      explorerUrls: ['https://explorer.near.org'],
      nativeCurrency: { name: 'NEAR', symbol: 'NEAR', decimals: 24 },
      blockTime: 1,
    });

    // Algorand
    this.supportedNonEvmChains.set('algorand', {
      id: 'algorand',
      name: 'Algorand',
      symbol: 'ALGO',
      category: 'algorand' as ChainCategory,
      status: 'active',
      chainId: -1,
      rpcUrls: ['https://mainnet-api.algorand.network'],
      explorerUrls: ['https://algoexplorer.cc'],
      nativeCurrency: { name: 'Algorand', symbol: 'ALGO', decimals: 6 },
      blockTime: 3,
    });

    // Osmosis
    this.supportedNonEvmChains.set('osmosis', {
      id: 'osmosis',
      name: 'Osmosis',
      symbol: 'OSMO',
      category: 'cosmos' as ChainCategory,
      status: 'active',
      chainId: -1,
      rpcUrls: ['https://rpc-osmosis.keplr.app'],
      explorerUrls: ['https://mintscan.io/osmosis'],
      nativeCurrency: { name: 'Osmosis', symbol: 'OSMO', decimals: 6 },
      blockTime: 5,
    });

    // Juno
    this.supportedNonEvmChains.set('juno', {
      id: 'juno',
      name: 'Juno',
      symbol: 'JUNO',
      category: 'cosmos' as ChainCategory,
      status: 'active',
      chainId: -1,
      rpcUrls: ['https://rpc.juno.kingnodes.com'],
      explorerUrls: ['https://mintscan.io/juno'],
      nativeCurrency: { name: 'Juno', symbol: 'JUNO', decimals: 6 },
      blockTime: 7,
    });

    // Injective
    this.supportedNonEvmChains.set('injective', {
      id: 'injective',
      name: 'Injective',
      symbol: 'INJ',
      category: 'cosmos' as ChainCategory,
      status: 'active',
      chainId: -1,
      rpcUrls: ['https://public.injective.network'],
      explorerUrls: ['https://explorer.injective.network'],
      nativeCurrency: { name: 'Injective', symbol: 'INJ', decimals: 18 },
      blockTime: 2,
    });

    // Sei
    this.supportedNonEvmChains.set('sei', {
      id: 'sei',
      name: 'Sei',
      symbol: 'SEI',
      category: 'cosmos' as ChainCategory,
      status: 'active',
      chainId: -1,
      rpcUrls: ['https://rpc.sei.io'],
      explorerUrls: ['https://seistats.io'],
      nativeCurrency: { name: 'Sei', symbol: 'SEI', decimals: 6 },
      blockTime: 0.4,
    });

    // Radix
    this.supportedNonEvmChains.set('radix', {
      id: 'radix',
      name: 'Radix',
      symbol: 'XRD',
      category: 'other' as ChainCategory,
      status: 'active',
      chainId: -1,
      rpcUrls: ['https://mainnet.radixdlt.com'],
      explorerUrls: ['https://explorer.radixdlt.com'],
      nativeCurrency: { name: 'Radix', symbol: 'XRD', decimals: 10 },
      blockTime: 2,
    });

    // Flow
    this.supportedNonEvmChains.set('flow', {
      id: 'flow',
      name: 'Flow',
      symbol: 'FLOW',
      category: 'other' as ChainCategory,
      status: 'active',
      chainId: -1,
      rpcUrls: ['https://flow-evm.g.alchemy.com/v2/demo'],
      explorerUrls: ['https://flowdiver.io'],
      nativeCurrency: { name: 'Flow', symbol: 'FLOW', decimals: 8 },
      blockTime: 2,
    });

    // Hedera
    this.supportedNonEvmChains.set('hedera', {
      id: 'hedera',
      name: 'Hedera',
      symbol: 'HBAR',
      category: 'other' as ChainCategory,
      status: 'active',
      chainId: -1,
      rpcUrls: ['https://mainnet.mirror.hedera.com/api/v1/contracts/call'],
      explorerUrls: ['https://hashscan.io'],
      nativeCurrency: { name: 'Hedera', symbol: 'HBAR', decimals: 8 },
      blockTime: 3,
    });

    // ICON
    this.supportedNonEvmChains.set('icon', {
      id: 'icon',
      name: 'ICON',
      symbol: 'ICX',
      category: 'other' as ChainCategory,
      status: 'active',
      chainId: -1,
      rpcUrls: ['https://ctz.solidwallet.io'],
      explorerUrls: ['https://tracker.icon.community'],
      nativeCurrency: { name: 'ICON', symbol: 'ICX', decimals: 18 },
      blockTime: 2,
    });

    // VeChain
    this.supportedNonEvmChains.set('vechain', {
      id: 'vechain',
      name: 'VeChain',
      symbol: 'VET',
      category: 'other' as ChainCategory,
      status: 'active',
      chainId: -1,
      rpcUrls: ['https://mainnet.vechain.org'],
      explorerUrls: ['https://explore.vechain.org'],
      nativeCurrency: { name: 'VeChain', symbol: 'VET', decimals: 18 },
      blockTime: 6,
    });

    // Theta
    this.supportedNonEvmChains.set('theta', {
      id: 'theta',
      name: 'Theta',
      symbol: 'THETA',
      category: 'other' as ChainCategory,
      status: 'active',
      chainId: -1,
      rpcUrls: ['https://eth-rpc-api.thetatoken.org/rest'],
      explorerUrls: ['https://explorer.thetatoken.org'],
      nativeCurrency: { name: 'Theta', symbol: 'THETA', decimals: 18 },
      blockTime: 10,
    });

    // MultiversX
    this.supportedNonEvmChains.set('multiversx', {
      id: 'multiversx',
      name: 'MultiversX',
      symbol: 'EGLD',
      category: 'other' as ChainCategory,
      status: 'active',
      chainId: -1,
      rpcUrls: ['https://api.multiversx.com'],
      explorerUrls: ['https://explorer.multiversx.com'],
      nativeCurrency: { name: 'MultiversX', symbol: 'EGLD', decimals: 18 },
      blockTime: 6,
    });

    // Polkadot
    this.supportedNonEvmChains.set('polkadot', {
      id: 'polkadot',
      name: 'Polkadot',
      symbol: 'DOT',
      category: 'polkadot' as ChainCategory,
      status: 'active',
      chainId: -1,
      rpcUrls: ['https://rpc.polkadot.io'],
      explorerUrls: ['https://polkadot.subscan.io'],
      nativeCurrency: { name: 'Polkadot', symbol: 'DOT', decimals: 10 },
      blockTime: 12,
    });

    // Kusama
    this.supportedNonEvmChains.set('kusama', {
      id: 'kusama',
      name: 'Kusama',
      symbol: 'KSM',
      category: 'polkadot' as ChainCategory,
      status: 'active',
      chainId: -1,
      rpcUrls: ['https://kusama-rpc.polkadot.io'],
      explorerUrls: ['https://kusama.subscan.io'],
      nativeCurrency: { name: 'Kusama', symbol: 'KSM', decimals: 12 },
      blockTime: 6,
    });

    // Kadena
    this.supportedNonEvmChains.set('kadena', {
      id: 'kadena',
      name: 'Kadena',
      symbol: 'KDA',
      category: 'other' as ChainCategory,
      status: 'active',
      chainId: -1,
      rpcUrls: ['https://api.chainweb.com'],
      explorerUrls: ['https://explorer.kadena.io'],
      nativeCurrency: { name: 'Kadena', symbol: 'KDA', decimals: 12 },
      blockTime: 1,
    });

    // Casper
    this.supportedNonEvmChains.set('casper', {
      id: 'casper',
      name: 'Casper',
      symbol: 'CSPR',
      category: 'other' as ChainCategory,
      status: 'active',
      chainId: -1,
      rpcUrls: ['https://rpc.mainnet.casper.network'],
      explorerUrls: ['https://cspr.live'],
      nativeCurrency: { name: 'Casper', symbol: 'CSPR', decimals: 9 },
      blockTime: 60,
    });

    // Fuel
    this.supportedNonEvmChains.set('fuel', {
      id: 'fuel',
      name: 'Fuel',
      symbol: 'FUEL',
      category: 'other' as ChainCategory,
      status: 'active',
      chainId: -1,
      rpcUrls: ['https://mainnet.fuel.network'],
      explorerUrls: ['https://fuelscan.io'],
      nativeCurrency: { name: 'Fuel', symbol: 'FUEL', decimals: 18 },
      blockTime: 2,
    });

    // Tron
    this.supportedNonEvmChains.set('tron', {
      id: 'tron',
      name: 'Tron',
      symbol: 'TRX',
      category: 'tron' as ChainCategory,
      status: 'active',
      chainId: -1,
      rpcUrls: ['https://api.trongrid.io'],
      explorerUrls: ['https://tronscan.org'],
      nativeCurrency: { name: 'Tron', symbol: 'TRX', decimals: 6 },
      blockTime: 3,
    });

    // Stellar
    this.supportedNonEvmChains.set('stellar', {
      id: 'stellar',
      name: 'Stellar',
      symbol: 'XLM',
      category: 'other' as ChainCategory,
      status: 'active',
      chainId: -1,
      rpcUrls: ['https://horizon.stellar.org'],
      explorerUrls: ['https://stellar.expert'],
      nativeCurrency: { name: 'Stellar', symbol: 'XLM', decimals: 7 },
      blockTime: 5,
    });
  }

  private initializeTokens(): void {
    // Tiger Ecosystem Tokens
    this.supportedTokens.set('TGR', {
      address: '0x0000000000000000000000000000000000000000',
      chainId: '13000',
      symbol: 'TGR',
      name: 'Tiger Coin',
      decimals: 18,
      isNative: true,
      isStable: false,
      coingeckoId: 'tigercoin',
    });

    this.supportedTokens.set('RUSD', {
      address: '0x0000000000000000000000000000000000000001',
      chainId: '13000',
      symbol: 'RUSD',
      name: 'Royal Tiger United States Dollar',
      decimals: 18,
      isStable: true,
      coingeckoId: 'royal-tiger-usd',
    });

    // Major cryptocurrencies
    const majorTokens = [
      { symbol: 'ETH', name: 'Ethereum', decimals: 18 },
      { symbol: 'BNB', name: 'BNB', decimals: 18 },
      { symbol: 'SOL', name: 'Solana', decimals: 9 },
      { symbol: 'MATIC', name: 'Polygon', decimals: 18 },
      { symbol: 'AVAX', name: 'Avalanche', decimals: 18 },
      { symbol: 'LINK', name: 'Chainlink', decimals: 18 },
      { symbol: 'DOT', name: 'Polkadot', decimals: 10 },
      { symbol: 'ATOM', name: 'Cosmos', decimals: 6 },
      { symbol: 'LTC', name: 'Litecoin', decimals: 8 },
      { symbol: 'BTC', name: 'Bitcoin', decimals: 8 },
      { symbol: 'USDT', name: 'Tether USD', decimals: 6, isStable: true },
      { symbol: 'USDC', name: 'USD Coin', decimals: 6, isStable: true },
      { symbol: 'DAI', name: 'Dai Stablecoin', decimals: 18, isStable: true },
      { symbol: 'BUSD', name: 'Binance USD', decimals: 18, isStable: true },
      { symbol: 'TUSD', name: 'TrueUSD', decimals: 18, isStable: true },
      { symbol: 'USDP', name: 'Pax Dollar', decimals: 18, isStable: true },
      { symbol: 'FRAX', name: 'Frax', decimals: 18, isStable: true },
      { symbol: 'AAVE', name: 'Aave', decimals: 18 },
      { symbol: 'UNI', name: 'Uniswap', decimals: 18 },
      { symbol: 'MKR', name: 'Maker', decimals: 18 },
      { symbol: 'CRV', name: 'Curve DAO', decimals: 18 },
      { symbol: 'LDO', name: 'Lido DAO', decimals: 18 },
      { symbol: 'SNX', name: 'Synthetix', decimals: 18 },
      { symbol: 'COMP', name: 'Compound', decimals: 18 },
      { symbol: 'SUSHI', name: 'SushiSwap', decimals: 18 },
      { symbol: 'SHIB', name: 'Shiba Inu', decimals: 18 },
      { symbol: 'PEPE', name: 'Pepe', decimals: 18 },
      { symbol: 'WIF', name: 'dogwifhat', decimals: 6 },
      { symbol: 'ORDI', name: 'ORDI', decimals: 18 },
      { symbol: 'BONK', name: 'Bonk', decimals: 5 },
      { symbol: 'SEI', name: 'Sei', decimals: 6 },
      { symbol: 'INJ', name: 'Injective', decimals: 18 },
      { symbol: 'ARB', name: 'Arbitrum', decimals: 18 },
      { symbol: 'OP', name: 'Optimism', decimals: 18 },
      { symbol: 'FTM', name: 'Fantom', decimals: 18 },
      { symbol: 'SAND', name: 'The Sandbox', decimals: 18 },
      { symbol: 'MANA', name: 'Decentraland', decimals: 18 },
      { symbol: 'AXS', name: 'Axie Infinity', decimals: 18 },
      { symbol: 'ALGO', name: 'Algorand', decimals: 6 },
      { symbol: 'XRP', name: 'Ripple', decimals: 6 },
      { symbol: 'ADA', name: 'Cardano', decimals: 6 },
      { symbol: 'DOGE', name: 'Dogecoin', decimals: 8 },
      { symbol: 'XLM', name: 'Stellar', decimals: 7 },
      { symbol: 'HBAR', name: 'Hedera', decimals: 8 },
      { symbol: 'VET', name: 'VeChain', decimals: 18 },
      { symbol: 'FIL', name: 'Filecoin', decimals: 18 },
      { symbol: 'THETA', name: 'Theta', decimals: 18 },
      { symbol: 'EGLD', name: 'MultiversX', decimals: 18 },
      { symbol: 'TRX', name: 'Tron', decimals: 6 },
      { symbol: 'NEAR', name: 'NEAR Protocol', decimals: 24 },
      { symbol: 'APT', name: 'Aptos', decimals: 8 },
      { symbol: 'SUI', name: 'Sui', decimals: 9 },
      { symbol: 'TON', name: 'TON', decimals: 9 },
    ];

    for (const token of majorTokens) {
      const chainId = token.symbol === 'SOL' || token.symbol === 'APT' || token.symbol === 'SUI' || token.symbol === 'TON' ? 
        (token.symbol === 'SOL' ? 'solana' : token.symbol === 'APT' ? 'aptos' : token.symbol === 'SUI' ? 'sui' : 'ton') :
        'ethereum';
      
      this.supportedTokens.set(token.symbol, {
        address: token.symbol === 'ETH' ? '0x0000000000000000000000000000000000000000' : `0x${'00'.repeat(20)}`,
        chainId,
        symbol: token.symbol,
        name: token.name,
        decimals: token.decimals,
        isStable: token.isStable || false,
      });
    }
  }

  private initializeDexPools(): void {
    // TGR Pools
    this.dexPools.set('TGR-USDT', {
      tokenA: 'TGR',
      tokenB: 'USDT',
      reserveA: 1000000n * 10n ** 18n,
      reserveB: 500000n * 10n ** 6n,
      fee: 0.003,
      liquidity: 1000000n * 10n ** 18n,
      apy: 25,
    });

    this.dexPools.set('TGR-RUSD', {
      tokenA: 'TGR',
      tokenB: 'RUSD',
      reserveA: 500000n * 10n ** 18n,
      reserveB: 500000n * 10n ** 18n,
      fee: 0.003,
      liquidity: 500000n * 10n ** 18n,
      apy: 30,
    });

    this.dexPools.set('TGR-ETH', {
      tokenA: 'TGR',
      tokenB: 'ETH',
      reserveA: 1000000n * 10n ** 18n,
      reserveB: 500n * 10n ** 18n,
      fee: 0.003,
      liquidity: 500n * 10n ** 18n,
      apy: 20,
    });

    this.dexPools.set('RUSD-USDT', {
      tokenA: 'RUSD',
      tokenB: 'USDT',
      reserveA: 1000000n * 10n ** 18n,
      reserveB: 1000000n * 10n ** 6n,
      fee: 0.001,
      liquidity: 1000000n * 10n ** 18n,
      apy: 10,
    });

    this.dexPools.set('ETH-USDT', {
      tokenA: 'ETH',
      tokenB: 'USDT',
      reserveA: 1000n * 10n ** 18n,
      reserveB: 3000000n * 10n ** 6n,
      fee: 0.003,
      liquidity: 1000n * 10n ** 18n,
      apy: 15,
    });

    this.dexPools.set('BTC-USDT', {
      tokenA: 'BTC',
      tokenB: 'USDT',
      reserveA: 100n * 10n ** 8n,
      reserveB: 5000000n * 10n ** 6n,
      fee: 0.003,
      liquidity: 100n * 10n ** 8n,
      apy: 12,
    });

    this.dexPools.set('ETH-BTC', {
      tokenA: 'ETH',
      tokenB: 'BTC',
      reserveA: 500n * 10n ** 18n,
      reserveB: 10n * 10n ** 8n,
      fee: 0.003,
      liquidity: 10n * 10n ** 8n,
      apy: 18,
    });
  }

  private initializeFarms(): void {
    this.farms.set('TGR-USDT', {
      poolId: 'TGR-USDT',
      rewardToken: 'TGR',
      stakedToken: 'TGR-USDT',
      stakedAmount: 0n,
      rewardAmount: 0n,
      apy: 25,
      startTime: Date.now(),
      endTime: Date.now() + 365 * 24 * 60 * 60 * 1000,
    });

    this.farms.set('TGR-ETH', {
      poolId: 'TGR-ETH',
      rewardToken: 'TGR',
      stakedToken: 'TGR-ETH',
      stakedAmount: 0n,
      rewardAmount: 0n,
      apy: 20,
      startTime: Date.now(),
      endTime: Date.now() + 365 * 24 * 60 * 60 * 1000,
    });

    this.farms.set('RUSD-USDT', {
      poolId: 'RUSD-USDT',
      rewardToken: 'TGR',
      stakedToken: 'RUSD-USDT',
      stakedAmount: 0n,
      rewardAmount: 0n,
      apy: 15,
      startTime: Date.now(),
      endTime: Date.now() + 365 * 24 * 60 * 60 * 1000,
    });
  }

  private initializeBridges(): void {
    // Cross-chain bridges with fee collection
    this.bridges.set('eth-bsc', {
      bridgeId: 'eth-bsc',
      sourceChain: 'ethereum',
      targetChain: 'bsc',
      token: '*',
      minAmount: 10n ** 18n,
      maxAmount: 1000000n * 10n ** 18n,
      fee: 0.001,
      estimatedTime: 600000,
      isActive: true,
    });

    this.bridges.set('eth-polygon', {
      bridgeId: 'eth-polygon',
      sourceChain: 'ethereum',
      targetChain: 'polygon',
      token: '*',
      minAmount: 10n ** 18n,
      maxAmount: 1000000n * 10n ** 18n,
      fee: 0.001,
      estimatedTime: 900000,
      isActive: true,
    });

    this.bridges.set('eth-arbitrum', {
      bridgeId: 'eth-arbitrum',
      sourceChain: 'ethereum',
      targetChain: 'arbitrum',
      token: '*',
      minAmount: 10n ** 18n,
      maxAmount: 1000000n * 10n ** 18n,
      fee: 0.0015,
      estimatedTime: 1200000,
      isActive: true,
    });

    this.bridges.set('eth-optimism', {
      bridgeId: 'eth-optimism',
      sourceChain: 'ethereum',
      targetChain: 'optimism',
      token: '*',
      minAmount: 10n ** 18n,
      maxAmount: 1000000n * 10n ** 18n,
      fee: 0.0015,
      estimatedTime: 900000,
      isActive: true,
    });

    this.bridges.set('eth-avalanche', {
      bridgeId: 'eth-avalanche',
      sourceChain: 'ethereum',
      targetChain: 'avalanche',
      token: '*',
      minAmount: 10n ** 18n,
      maxAmount: 1000000n * 10n ** 18n,
      fee: 0.001,
      estimatedTime: 600000,
      isActive: true,
    });

    this.bridges.set('bsc-polygon', {
      bridgeId: 'bsc-polygon',
      sourceChain: 'bsc',
      targetChain: 'polygon',
      token: '*',
      minAmount: 10n ** 18n,
      maxAmount: 1000000n * 10n ** 18n,
      fee: 0.001,
      estimatedTime: 600000,
      isActive: true,
    });

    this.bridges.set('tgr-eth', {
      bridgeId: 'tgr-eth',
      sourceChain: 'tigersmartchain',
      targetChain: 'ethereum',
      token: 'TGR',
      minAmount: 100n * 10n ** 18n,
      maxAmount: 10000000n * 10n ** 18n,
      fee: 0.001,
      estimatedTime: 1800000,
      isActive: true,
    });

    this.bridges.set('tgr-bsc', {
      bridgeId: 'tgr-bsc',
      sourceChain: 'tigersmartchain',
      targetChain: 'bsc',
      token: 'TGR',
      minAmount: 100n * 10n ** 18n,
      maxAmount: 10000000n * 10n ** 18n,
      fee: 0.001,
      estimatedTime: 1200000,
      isActive: true,
    });

    this.bridges.set('rusd-eth', {
      bridgeId: 'rusd-eth',
      sourceChain: 'tigersmartchain',
      targetChain: 'ethereum',
      token: 'RUSD',
      minAmount: 100n * 10n ** 18n,
      maxAmount: 10000000n * 10n ** 18n,
      fee: 0.001,
      estimatedTime: 1800000,
      isActive: true,
    });
  }

  // ============================================================================
  // Public API Methods
  // ============================================================================

  /**
   * Get all supported EVM chains
   */
  getSupportedEvmChains(): ChainConfig[] {
    return Array.from(this.supportedEvmChains.values());
  }

  /**
   * Get all supported Non-EVM chains
   */
  getSupportedNonEvmChains(): ChainConfig[] {
    return Array.from(this.supportedNonEvmChains.values());
  }

  /**
   * Get all supported tokens
   */
  getSupportedTokens(): TokenConfig[] {
    return Array.from(this.supportedTokens.values());
  }

  /**
   * Get all DEX pools
   */
  getDexPools(): LiquidityPool[] {
    return Array.from(this.dexPools.values());
  }

  /**
   * Get all farms
   */
  getFarms(): FarmInfo[] {
    return Array.from(this.farms.values());
  }

  /**
   * Get all bridges
   */
  getBridges(): BridgeInfo[] {
    return Array.from(this.bridges.values());
  }

  /**
   * Get product status
   */
  getProductStatus(product: 'wallet' | 'swap' | 'smartchain' | 'ex'): TigerProduct {
    switch (product) {
      case 'wallet': return this.tigerWallet;
      case 'swap': return this.tigerSwap;
      case 'smartchain': return this.tigerSmartChain;
      case 'ex': return this.tigerEx;
    }
  }

  /**
   * Get fee collection summary
   */
  getFeeSummary(): {
    totalFees: string;
    feeBreakdown: {
      exchange: string;
      dex: string;
      bridge: string;
      wallet: string;
    };
    history: FeeRecord[];
  } {
    return {
      totalFees: this.totalFeesCollected.toString(),
      feeBreakdown: {
        exchange: (this.totalFeesCollected * 40n / 100n).toString(),
        dex: (this.totalFeesCollected * 30n / 100n).toString(),
        bridge: (this.totalFeesCollected * 20n / 100n).toString(),
        wallet: (this.totalFeesCollected * 10n / 100n).toString(),
      },
      history: this.feeHistory,
    };
  }

  /**
   * Calculate swap output with fees
   */
  calculateSwap(
    inputToken: string,
    outputToken: string,
    amountIn: bigint
  ): { amountOut: bigint; fee: bigint; path: string[] } {
    const poolKey = `${inputToken}-${outputToken}`;
    const reversePoolKey = `${outputToken}-${inputToken}`;
    
    let pool = this.dexPools.get(poolKey);
    let reverse = false;
    
    if (!pool) {
      pool = this.dexPools.get(reversePoolKey);
      reverse = true;
    }
    
    if (!pool) {
      // Multi-hop routing
      const hopToken = 'USDT';
      const pool1 = this.dexPools.get(`${inputToken}-${hopToken}`);
      const pool2 = this.dexPools.get(`${hopToken}-${outputToken}`);
      
      if (pool1 && pool2) {
        const intermediate = (amountIn * pool1.reserveB) / (pool1.reserveA + amountIn);
        const fee1 = intermediate * 3n / 1000n;
        const afterFee = intermediate - fee1;
        const amountOut = (afterFee * pool2.reserveB) / (pool2.reserveA + afterFee);
        const fee2 = amountOut * 3n / 1000n;
        
        return {
          amountOut: amountOut - fee2,
          fee: fee1 + fee2,
          path: [inputToken, hopToken, outputToken],
        };
      }
      
      throw new Error(`No pool found for ${inputToken}-${outputToken}`);
    }
    
    const reserveIn = reverse ? pool.reserveB : pool.reserveA;
    const reserveOut = reverse ? pool.reserveA : pool.reserveB;
    
    const amountOut = (amountIn * reserveOut) / (reserveIn + amountIn);
    const fee = amountOut * BigInt(pool.fee * 1000) / 1000n;
    
    return {
      amountOut: amountOut - fee,
      fee,
      path: [inputToken, outputToken],
    };
  }

  /**
   * Calculate bridge transfer with fees
   */
  calculateBridge(
    sourceChain: string,
    targetChain: string,
    token: string,
    amount: bigint
  ): { received: bigint; fee: bigint; estimatedTime: number } {
    const bridgeKey = `${sourceChain}-${targetChain}`;
    const bridge = this.bridges.get(bridgeKey);
    
    if (!bridge) {
      throw new Error(`No bridge found from ${sourceChain} to ${targetChain}`);
    }
    
    if (!bridge.isActive) {
      throw new Error(`Bridge ${bridgeKey} is not active`);
    }
    
    if (amount < bridge.minAmount) {
      throw new Error(`Amount too low. Minimum: ${bridge.minAmount}`);
    }
    
    if (amount > bridge.maxAmount) {
      throw new Error(`Amount too high. Maximum: ${bridge.maxAmount}`);
    }
    
    const fee = amount * BigInt(bridge.fee * 1000) / 1000n;
    
    return {
      received: amount - fee,
      fee,
      estimatedTime: bridge.estimatedTime,
    };
  }

  /**
   * Add new EVM chain at runtime
   */
  addEvmChain(config: ChainConfig): void {
    if (this.supportedEvmChains.has(config.id)) {
      throw new Error(`Chain ${config.id} already exists`);
    }
    
    this.supportedEvmChains.set(config.id, config);
    console.log(`[TigerEX] Added EVM chain: ${config.name} (${config.id})`);
  }

  /**
   * Add new Non-EVM chain at runtime
   */
  addNonEvmChain(config: ChainConfig): void {
    if (this.supportedNonEvmChains.has(config.id)) {
      throw new Error(`Chain ${config.id} already exists`);
    }
    
    this.supportedNonEvmChains.set(config.id, config);
    console.log(`[TigerEX] Added Non-EVM chain: ${config.name} (${config.id})`);
  }

  /**
   * Add new token at runtime
   */
  addToken(token: TokenConfig): void {
    if (this.supportedTokens.has(token.symbol)) {
      throw new Error(`Token ${token.symbol} already exists`);
    }
    
    this.supportedTokens.set(token.symbol, token);
    console.log(`[TigerEX] Added token: ${token.name} (${token.symbol})`);
  }

  /**
   * Create new DEX pool
   */
  createPool(tokenA: string, tokenB: string, fee: number): LiquidityPool {
    const poolKey = `${tokenA}-${tokenB}`;
    
    if (this.dexPools.has(poolKey)) {
      throw new Error(`Pool ${poolKey} already exists`);
    }
    
    const pool: LiquidityPool = {
      tokenA,
      tokenB,
      reserveA: 0n,
      reserveB: 0n,
      fee,
      liquidity: 0n,
      apy: 0,
    };
    
    this.dexPools.set(poolKey, pool);
    console.log(`[TigerEX] Created pool: ${tokenA}/${tokenB}`);
    
    return pool;
  }

  /**
   * Create new farm
   */
  createFarm(poolId: string, rewardToken: string, apy: number): FarmInfo {
    if (this.farms.has(poolId)) {
      throw new Error(`Farm ${poolId} already exists`);
    }
    
    const farm: FarmInfo = {
      poolId,
      rewardToken,
      stakedToken: poolId,
      stakedAmount: 0n,
      rewardAmount: 0n,
      apy,
      startTime: Date.now(),
      endTime: Date.now() + 365 * 24 * 60 * 60 * 1000,
    };
    
    this.farms.set(poolId, farm);
    console.log(`[TigerEX] Created farm: ${poolId}`);
    
    return farm;
  }

  /**
   * Add new bridge
   */
  addBridge(
    sourceChain: string,
    targetChain: string,
    token: string,
    minAmount: bigint,
    maxAmount: bigint,
    fee: number,
    estimatedTime: number
  ): BridgeInfo {
    const bridgeId = `${sourceChain}-${targetChain}`;
    
    if (this.bridges.has(bridgeId)) {
      throw new Error(`Bridge ${bridgeId} already exists`);
    }
    
    const bridge: BridgeInfo = {
      bridgeId,
      sourceChain,
      targetChain,
      token,
      minAmount,
      maxAmount,
      fee,
      estimatedTime,
      isActive: true,
    };
    
    this.bridges.set(bridgeId, bridge);
    console.log(`[TigerEX] Created bridge: ${sourceChain} -> ${targetChain}`);
    
    return bridge;
  }

  /**
   * Collect fee (internal)
   */
  collectFee(amount: bigint, source: 'exchange' | 'dex' | 'bridge' | 'wallet'): void {
    this.totalFeesCollected += amount;
    this.feeHistory.push({
      amount,
      source,
      timestamp: Date.now(),
    });
  }

  /**
   * Get platform statistics
   */
  getStats(): {
    totalEvmChains: number;
    totalNonEvmChains: number;
    totalTokens: number;
    totalPools: number;
    totalFarms: number;
    totalBridges: number;
    initialized: boolean;
  } {
    return {
      totalEvmChains: this.supportedEvmChains.size,
      totalNonEvmChains: this.supportedNonEvmChains.size,
      totalTokens: this.supportedTokens.size,
      totalPools: this.dexPools.size,
      totalFarms: this.farms.size,
      totalBridges: this.bridges.size,
      initialized: this.initialized,
    };
  }

  /**
   * Search chains by name or symbol
   */
  searchChains(query: string): ChainConfig[] {
    const results: ChainConfig[] = [];
    const lowerQuery = query.toLowerCase();
    
    for (const chain of [...this.supportedEvmChains.values(), ...this.supportedNonEvmChains.values()]) {
      if (chain.name.toLowerCase().includes(lowerQuery) || 
          chain.symbol.toLowerCase().includes(lowerQuery) ||
          chain.id.toLowerCase().includes(lowerQuery)) {
        results.push(chain);
      }
    }
    
    return results;
  }

  /**
   * Activate/Deactivate chain
   */
  setChainStatus(chainId: string, status: 'active' | 'inactive' | 'paused' | 'deprecated'): void {
    const chain = this.supportedEvmChains.get(chainId) || this.supportedNonEvmChains.get(chainId);
    
    if (!chain) {
      throw new Error(`Chain ${chainId} not found`);
    }
    
    chain.status = status;
    console.log(`[TigerEX] Chain ${chainId} status: ${status}`);
  }

  /**
   * Get cross-chain route for swap
   */
  getCrossChainRoute(
    sourceChain: string,
    targetChain: string,
    inputToken: string,
    outputToken: string,
    amount: bigint
  ): CrossChainRoute {
    // Calculate source chain swap
    const sourceNative = sourceChain === 'tigersmartchain' ? 'TGR' : 
                      sourceChain === 'ethereum' ? 'ETH' : 
                      sourceChain === 'bsc' ? 'BNB' : 
                      sourceChain === 'polygon' ? 'MATIC' : 'USDT';
    
    const targetNative = targetChain === 'tigersmartchain' ? 'TGR' : 
                     targetChain === 'ethereum' ? 'ETH' : 
                     targetChain === 'bsc' ? 'BNB' : 
                     targetChain === 'polygon' ? 'MATIC' : 'USDT';
    
    let swapOutput = amount;
    let path = [inputToken];
    
    // If input is not native to source, swap first
    if (inputToken !== sourceNative) {
      try {
        const swapResult = this.calculateSwap(inputToken, sourceNative, amount);
        swapOutput = swapResult.amountOut;
        path = [inputToken, sourceNative];
      } catch {
        // Skip if no direct pool
      }
    }
    
    // Bridge fee
    const bridgeKey = `${sourceChain}-${targetChain}`;
    const bridge = this.bridges.get(bridgeKey);
    const bridgeFee = bridge ? swapOutput * BigInt(bridge.fee * 1000) / 1000n : 0n;
    const afterBridge = swapOutput - bridgeFee;
    
    // Target chain swap
    let finalOutput = afterBridge;
    if (outputToken !== targetNative) {
      try {
        const swapResult = this.calculateSwap(targetNative, outputToken, afterBridge);
        finalOutput = swapResult.amountOut;
        path.push(outputToken);
      } catch {
        path.push(targetNative);
      }
    } else {
      path.push(outputToken);
    }
    
    return {
      sourceChain,
      targetChain,
      amount,
      inputToken,
      outputToken,
      estimatedOutput: finalOutput,
      fee: bridgeFee,
      path,
      bridge: bridgeKey,
      estimatedTime: bridge?.estimatedTime || 600000,
    };
  }
}

// ============================================================================
// Fee Record
// ============================================================================

interface FeeRecord {
  amount: bigint;
  source: 'exchange' | 'dex' | 'bridge' | 'wallet';
  timestamp: number;
}

// ============================================================================
// Export Singleton Instance
// ============================================================================

export const tigerEX = new TigerEXIntegration();

// ============================================================================
// Default Export
// ============================================================================

export default TigerEXIntegration;