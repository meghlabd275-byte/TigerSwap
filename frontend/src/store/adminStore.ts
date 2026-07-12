import { create } from 'zustand';
import { persist } from 'zustand/middleware';

export interface Blockchain {
  id: string;
  chainId: number;
  name: string;
  symbol: string;
  type: 'evm' | 'solana' | 'cosmos' | 'ton' | 'aptos' | 'near' | 'bitcoin';
  rpcUrl: string;
  explorerUrl: string;
  logoUrl: string;
  isActive: boolean;
  isTestnet: boolean;
  nativeToken: {
    name: string;
    symbol: string;
    decimals: number;
    address: string;
  };
  addedAt: Date;
}

export interface Token {
  id: string;
  address: string;
  chainId: number;
  name: string;
  symbol: string;
  decimals: number;
  logoUrl: string;
  isActive: boolean;
  isVerified: boolean;
  isNative: boolean;
  totalSupply: string;
  coingeckoId?: string;
  priceUSD: number;
  addedAt: Date;
  addedBy: string;
}

export interface LaunchpadProject {
  id: string;
  name: string;
  description: string;
  tokenAddress: string;
  tokenSymbol: string;
  tokenName: string;
  tokenDecimals: number;
  totalSupply: string;
  pricePerToken: string;
  paymentToken: string;
  minPurchase: string;
  maxPurchase: string;
  softCap: string;
  hardCap: string;
  startTime: Date;
  endTime: Date;
  status: 'upcoming' | 'active' | 'completed' | 'cancelled';
  raisedAmount: string;
  participants: number;
  websiteUrl: string;
  whitepaperUrl: string;
  logoUrl: string;
  socialLinks: {
    twitter?: string;
    telegram?: string;
    discord?: string;
  };
  createdAt: Date;
  createdBy: string;
}

export interface FeeSettings {
  withdrawFeePercent: number;
  swapFeePercent: number;
  transactionFeePercent: number;
  depositFeePercent: number;
  launchpadFeePercent: number;
  referralFeePercent: number;
}

export interface SystemSettings {
  maintenanceMode: boolean;
  tradingEnabled: boolean;
  withdrawalEnabled: boolean;
  depositEnabled: boolean;
  newUserRegistrationEnabled: boolean;
}

interface AdminState {
  // Master admin credentials
  adminSeedPhrase: string | null;
  adminAddress: string | null;
  isAdminAuthenticated: boolean;
  
  // Blockchains
  blockchains: Blockchain[];
  
  // Tokens
  tokens: Token[];
  
  // Launchpad projects
  launchpadProjects: LaunchpadProject[];
  
  // Fee settings
  feeSettings: FeeSettings;
  
  // System settings
  systemSettings: SystemSettings;
  
  // Activity logs
  activityLogs: Array<{
    id: string;
    action: string;
    details: string;
    timestamp: Date;
    ipAddress: string;
  }>;
  
  // Stats
  stats: {
    totalUsers: number;
    totalTransactions: number;
    totalVolume: string;
    totalRevenue: string;
  };
  
  // Actions
  authenticateAdmin: (seedPhrase: string) => boolean;
  logoutAdmin: () => void;
  
  // Blockchain management
  addBlockchain: (blockchain: Omit<Blockchain, 'id' | 'addedAt'>) => void;
  updateBlockchain: (id: string, updates: Partial<Blockchain>) => void;
  deleteBlockchain: (id: string) => void;
  toggleBlockchain: (id: string) => void;
  
  // Token management
  addToken: (token: Omit<Token, 'id' | 'addedAt' | 'addedBy'>) => void;
  updateToken: (id: string, updates: Partial<Token>) => void;
  deleteToken: (id: string) => void;
  toggleToken: (id: string) => void;
  verifyToken: (id: string) => void;
  
  // Launchpad management
  createLaunchpadProject: (project: Omit<LaunchpadProject, 'id' | 'createdAt' | 'createdBy' | 'raisedAmount' | 'participants'>) => void;
  updateLaunchpadProject: (id: string, updates: Partial<LaunchpadProject>) => void;
  deleteLaunchpadProject: (id: string) => void;
  updateLaunchpadStatus: (id: string, status: LaunchpadProject['status']) => void;
  
  // Fee management
  updateFeeSettings: (fees: Partial<FeeSettings>) => void;
  
  // System settings
  updateSystemSettings: (settings: Partial<SystemSettings>) => void;
  toggleMaintenanceMode: () => void;
  
  // Activity logging
  logActivity: (action: string, details: string) => void;
}

// Default blockchains
const DEFAULT_BLOCKCHAINS: Blockchain[] = [
  {
    id: 'eth-1',
    chainId: 1,
    name: 'Ethereum',
    symbol: 'ETH',
    type: 'evm',
    rpcUrl: 'https://eth.llamarpc.com',
    explorerUrl: 'https://etherscan.io',
    logoUrl: 'https://assets.coingecko.com/coins/images/279/small/ethereum.png',
    isActive: true,
    isTestnet: false,
    nativeToken: { name: 'Ethereum', symbol: 'ETH', decimals: 18, address: '0x0000000000000000000000000000000000000000' },
    addedAt: new Date()
  },
  {
    id: 'poly-137',
    chainId: 137,
    name: 'Polygon',
    symbol: 'MATIC',
    type: 'evm',
    rpcUrl: 'https://polygon.llamarpc.com',
    explorerUrl: 'https://polygonscan.com',
    logoUrl: 'https://assets.coingecko.com/coins/images/4713/small/polygon.png',
    isActive: true,
    isTestnet: false,
    nativeToken: { name: 'Polygon', symbol: 'MATIC', decimals: 18, address: '0x0000000000000000000000000000000000000000' },
    addedAt: new Date()
  },
  {
    id: 'arb-42161',
    chainId: 42161,
    name: 'Arbitrum One',
    symbol: 'ETH',
    type: 'evm',
    rpcUrl: 'https://arb1.arbitrum.io/rpc',
    explorerUrl: 'https://arbiscan.io',
    logoUrl: 'https://assets.coingecko.com/coins/images/16547/small/photo_2023-03-29_21.47.00.jpeg',
    isActive: true,
    isTestnet: false,
    nativeToken: { name: 'Ethereum', symbol: 'ETH', decimals: 18, address: '0x0000000000000000000000000000000000000000' },
    addedAt: new Date()
  },
  {
    id: 'opt-10',
    chainId: 10,
    name: 'Optimism',
    symbol: 'ETH',
    type: 'evm',
    rpcUrl: 'https://mainnet.optimism.io',
    explorerUrl: 'https://optimistic.etherscan.io',
    logoUrl: 'https://assets.coingecko.com/coins/images/25244/small/Optimism.png',
    isActive: true,
    isTestnet: false,
    nativeToken: { name: 'Ethereum', symbol: 'ETH', decimals: 18, address: '0x0000000000000000000000000000000000000000' },
    addedAt: new Date()
  },
  {
    id: 'base-8453',
    chainId: 8453,
    name: 'Base',
    symbol: 'ETH',
    type: 'evm',
    rpcUrl: 'https://mainnet.base.org',
    explorerUrl: 'https://basescan.org',
    logoUrl: 'https://assets.coingecko.com/coins/images/31088/small/base.png',
    isActive: true,
    isTestnet: false,
    nativeToken: { name: 'Ethereum', symbol: 'ETH', decimals: 18, address: '0x0000000000000000000000000000000000000000' },
    addedAt: new Date()
  },
  {
    id: 'bsc-56',
    chainId: 56,
    name: 'BNB Smart Chain',
    symbol: 'BNB',
    type: 'evm',
    rpcUrl: 'https://bsc-dataseed.binance.org',
    explorerUrl: 'https://bscscan.com',
    logoUrl: 'https://assets.coingecko.com/coins/images/825/small/bnb-icon2_2x.png',
    isActive: true,
    isTestnet: false,
    nativeToken: { name: 'BNB', symbol: 'BNB', decimals: 18, address: '0x0000000000000000000000000000000000000000' },
    addedAt: new Date()
  },
  {
    id: 'avax-43114',
    chainId: 43114,
    name: 'Avalanche',
    symbol: 'AVAX',
    type: 'evm',
    rpcUrl: 'https://api.avax.network/ext/bc/C/rpc',
    explorerUrl: 'https://snowtrace.io',
    logoUrl: 'https://assets.coingecko.com/coins/images/12559/small/Avalanche_Circle_RedWhite_Trans.png',
    isActive: true,
    isTestnet: false,
    nativeToken: { name: 'Avalanche', symbol: 'AVAX', decimals: 18, address: '0x0000000000000000000000000000000000000000' },
    addedAt: new Date()
  },
  {
    id: 'ftm-250',
    chainId: 250,
    name: 'Fantom',
    symbol: 'FTM',
    type: 'evm',
    rpcUrl: 'https://rpc.fantom.network',
    explorerUrl: 'https://ftmscan.com',
    logoUrl: 'https://assets.coingecko.com/coins/images/4001/small/Fantom_round.png',
    isActive: true,
    isTestnet: false,
    nativeToken: { name: 'Fantom', symbol: 'FTM', decimals: 18, address: '0x0000000000000000000000000000000000000000' },
    addedAt: new Date()
  },
  {
    id: 'sol-101',
    chainId: 101,
    name: 'Solana',
    symbol: 'SOL',
    type: 'solana',
    rpcUrl: 'https://api.mainnet-beta.solana.com',
    explorerUrl: 'https://solscan.io',
    logoUrl: 'https://assets.coingecko.com/coins/images/4128/small/solana.png',
    isActive: true,
    isTestnet: false,
    nativeToken: { name: 'Solana', symbol: 'SOL', decimals: 9, address: 'So11111111111111111111111111111111111111112' },
    addedAt: new Date()
  },
  {
    id: 'cosmos-1',
    chainId: 1,
    name: 'Cosmos',
    symbol: 'ATOM',
    type: 'cosmos',
    rpcUrl: 'https://rpc.cosmos.network',
    explorerUrl: 'https://mintscan.io/cosmos',
    logoUrl: 'https://assets.coingecko.com/coins/images/1481/small/cosmos_hub.png',
    isActive: true,
    isTestnet: false,
    nativeToken: { name: 'Cosmos', symbol: 'ATOM', decimals: 6, address: 'uatom' },
    addedAt: new Date()
  },
  {
    id: 'ton-1',
    chainId: 1,
    name: 'TON',
    symbol: 'TON',
    type: 'ton',
    rpcUrl: 'https://toncenter.com/api/v2/jsonRPC',
    explorerUrl: 'https://tonscan.org',
    logoUrl: 'https://assets.coingecko.com/coins/images/17980/small/ton_symbol.png',
    isActive: true,
    isTestnet: false,
    nativeToken: { name: 'Toncoin', symbol: 'TON', decimals: 9, address: '0:0000000000000000000000000000000000000000000000000000000000000000' },
    addedAt: new Date()
  },
  {
    id: 'aptos-1',
    chainId: 1,
    name: 'Aptos',
    symbol: 'APT',
    type: 'aptos',
    rpcUrl: 'https://fullnode.mainnet.aptoslabs.com',
    explorerUrl: 'https://aptoscan.com',
    logoUrl: 'https://assets.coingecko.com/coins/images/26455/small/aptos_round.png',
    isActive: true,
    isTestnet: false,
    nativeToken: { name: 'Aptos', symbol: 'APT', decimals: 8, address: '0x1::aptos_coin::AptosCoin' },
    addedAt: new Date()
  },
  {
    id: 'zksync-324',
    chainId: 324,
    name: 'zkSync Era',
    symbol: 'ETH',
    type: 'evm',
    rpcUrl: 'https://mainnet.era.zksync.io',
    explorerUrl: 'https://explorer.zksync.io',
    logoUrl: 'https://assets.coingecko.com/coins/images/233806/small/syncnew.jpg',
    isActive: true,
    isTestnet: false,
    nativeToken: { name: 'Ethereum', symbol: 'ETH', decimals: 18, address: '0x0000000000000000000000000000000000000000' },
    addedAt: new Date()
  },
  {
    id: 'linea-59144',
    chainId: 59144,
    name: 'Linea',
    symbol: 'ETH',
    type: 'evm',
    rpcUrl: 'https://rpc.linea.build',
    explorerUrl: 'https://lineascan.build',
    logoUrl: 'https://assets.coingecko.com/coins/images/28661/small/linea.png',
    isActive: true,
    isTestnet: false,
    nativeToken: { name: 'Ethereum', symbol: 'ETH', decimals: 18, address: '0x0000000000000000000000000000000000000000' },
    addedAt: new Date()
  },
  {
    id: 'mantle-5000',
    chainId: 5000,
    name: 'Mantle',
    symbol: 'MNT',
    type: 'evm',
    rpcUrl: 'https://rpc.mantle.xyz',
    explorerUrl: 'https://mantlescan.info',
    logoUrl: 'https://assets.coingecko.com/coins/images/31080/small/Mantle.png',
    isActive: true,
    isTestnet: false,
    nativeToken: { name: 'Mantle', symbol: 'MNT', decimals: 18, address: '0x0000000000000000000000000000000000000000' },
    addedAt: new Date()
  },
  {
    id: 'blast-81457',
    chainId: 81457,
    name: 'Blast',
    symbol: 'ETH',
    type: 'evm',
    rpcUrl: 'https://rpc.blast.io',
    explorerUrl: 'https://blastscan.io',
    logoUrl: 'https://assets.coingecko.com/coins/images/35597/small/Blast.png',
    isActive: true,
    isTestnet: false,
    nativeToken: { name: 'Ethereum', symbol: 'ETH', decimals: 18, address: '0x0000000000000000000000000000000000000000' },
    addedAt: new Date()
  },
  {
    id: 'scroll-534352',
    chainId: 534352,
    name: 'Scroll',
    symbol: 'ETH',
    type: 'evm',
    rpcUrl: 'https://rpc.scroll.io',
    explorerUrl: 'https://scrollscan.com',
    logoUrl: 'https://assets.coingecko.com/coins/images/24656/small/scroll.png',
    isActive: true,
    isTestnet: false,
    nativeToken: { name: 'Ethereum', symbol: 'ETH', decimals: 18, address: '0x0000000000000000000000000000000000000000' },
    addedAt: new Date()
  },
  {
    id: 'gnosis-100',
    chainId: 100,
    name: 'Gnosis Chain',
    symbol: 'XDAI',
    type: 'evm',
    rpcUrl: 'https://rpc.gnosischain.com',
    explorerUrl: 'https://gnosisscan.io',
    logoUrl: 'https://assets.coingecko.com/coins/images/662/small/logotype_social.png',
    isActive: true,
    isTestnet: false,
    nativeToken: { name: 'Gnosis', symbol: 'XDAI', decimals: 18, address: '0x0000000000000000000000000000000000000000' },
    addedAt: new Date()
  },
  {
    id: 'celo-42220',
    chainId: 42220,
    name: 'Celo',
    symbol: 'CELO',
    type: 'evm',
    rpcUrl: 'https://forno.celo.org',
    explorerUrl: 'https://explorer.celo.org',
    logoUrl: 'https://assets.coingecko.com/coins/images/16690/small/Celo_Assets_2023_Resize_Transparent_Rest.png',
    isActive: true,
    isTestnet: false,
    nativeToken: { name: 'Celo', symbol: 'CELO', decimals: 18, address: '0x0000000000000000000000000000000000000000' },
    addedAt: new Date()
  },
  {
    id: 'kava-2222',
    chainId: 2222,
    name: 'Kava',
    symbol: 'KAVA',
    type: 'evm',
    rpcUrl: 'https://evm.kava.io',
    explorerUrl: 'https://kavascan.com',
    logoUrl: 'https://assets.coingecko.com/coins/images/9761/small/kava.png',
    isActive: true,
    isTestnet: false,
    nativeToken: { name: 'Kava', symbol: 'KAVA', decimals: 18, address: '0x0000000000000000000000000000000000000000' },
    addedAt: new Date()
  },
  {
    id: 'moonbeam-1284',
    chainId: 1284,
    name: 'Moonbeam',
    symbol: 'GLMR',
    type: 'evm',
    rpcUrl: 'https://rpc.api.moonbeam.network',
    explorerUrl: 'https://moonscan.io',
    logoUrl: 'https://assets.coingecko.com/coins/images/17167/small/moonbeam_new.png',
    isActive: true,
    isTestnet: false,
    nativeToken: { name: 'Moonbeam', symbol: 'GLMR', decimals: 18, address: '0x0000000000000000000000000000000000000000' },
    addedAt: new Date()
  },
  {
    id: 'astar-592',
    chainId: 592,
    name: 'Astar',
    symbol: 'ASTR',
    type: 'evm',
    rpcUrl: 'https://rpc.astar.network',
    explorerUrl: 'https://astr.explorer.subscan.io',
    logoUrl: 'https://assets.coingecko.com/coins/images/22617/small/astr.png',
    isActive: true,
    isTestnet: false,
    nativeToken: { name: 'Astar', symbol: 'ASTR', decimals: 18, address: '0x0000000000000000000000000000000000000000' },
    addedAt: new Date()
  },
  {
    id: 'cronos-25',
    chainId: 25,
    name: 'Cronos',
    symbol: 'CRO',
    type: 'evm',
    rpcUrl: 'https://evm.cronos.org',
    explorerUrl: 'https://cronoscan.com',
    logoUrl: 'https://assets.coingecko.com/coins/images/7310/small/cro_token_id.png',
    isActive: true,
    isTestnet: false,
    nativeToken: { name: 'Cronos', symbol: 'CRO', decimals: 18, address: '0x0000000000000000000000000000000000000000' },
    addedAt: new Date()
  },
  {
    id: 'core-1116',
    chainId: 1116,
    name: 'Core',
    symbol: 'CORE',
    type: 'evm',
    rpcUrl: 'https://rpc.coredao.org',
    explorerUrl: 'https://scan.coredao.org',
    logoUrl: 'https://assets.coingecko.com/coins/images/22407/small/coredao.png',
    isActive: true,
    isTestnet: false,
    nativeToken: { name: 'Core', symbol: 'CORE', decimals: 18, address: '0x0000000000000000000000000000000000000000' },
    addedAt: new Date()
  },
  {
    id: 'sei-1',
    chainId: 1,
    name: 'Sei',
    symbol: 'SEI',
    type: 'cosmos',
    rpcUrl: 'https://rpc.sei.io',
    explorerUrl: 'https://seistream.app',
    logoUrl: 'https://assets.coingecko.com/coins/images/28205/small/Sei_Logo_-_Transparent.png',
    isActive: true,
    isTestnet: false,
    nativeToken: { name: 'Sei', symbol: 'SEI', decimals: 6, address: 'usei' },
    addedAt: new Date()
  },
  {
    id: 'injective-1',
    chainId: 1,
    name: 'Injective',
    symbol: 'INJ',
    type: 'cosmos',
    rpcUrl: 'https://rpc.injective.network',
    explorerUrl: 'https://explorer.injective.network',
    logoUrl: 'https://assets.coingecko.com/coins/images/12882/small/Secondary_Symbol.png',
    isActive: true,
    isTestnet: false,
    nativeToken: { name: 'Injective', symbol: 'INJ', decimals: 18, address: 'inj' },
    addedAt: new Date()
  },
  {
    id: 'sui-1',
    chainId: 1,
    name: 'Sui',
    symbol: 'SUI',
    type: 'aptos',
    rpcUrl: 'https://rpc.sui.io',
    explorerUrl: 'https://suiscan.xyz',
    logoUrl: 'https://assets.coingecko.com/coins/images/26375/small/sui_asset.jpeg',
    isActive: true,
    isTestnet: false,
    nativeToken: { name: 'Sui', symbol: 'SUI', decimals: 9, address: '0x0000000000000000000000000000000000000000000000000000000000000002::sui::SUI' },
    addedAt: new Date()
  },
  {
    id: 'near-1',
    chainId: 1,
    name: 'NEAR Protocol',
    symbol: 'NEAR',
    type: 'near',
    rpcUrl: 'https://rpc.mainnet.near.org',
    explorerUrl: 'https://explorer.near.org',
    logoUrl: 'https://assets.coingecko.com/coins/images/10365/small/near.jpg',
    isActive: true,
    isTestnet: false,
    nativeToken: { name: 'NEAR', symbol: 'NEAR', decimals: 24, address: 'wrap.near' },
    addedAt: new Date()
  },
  {
    id: 'klaytn-8217',
    chainId: 8217,
    name: 'Klaytn',
    symbol: 'KLAY',
    type: 'evm',
    rpcUrl: 'https://rpc.klaytn.com',
    explorerUrl: 'https://klaytnscope.com',
    logoUrl: 'https://assets.coingecko.com/coins/images/9672/small/klaytn.png',
    isActive: true,
    isTestnet: false,
    nativeToken: { name: 'Klaytn', symbol: 'KLAY', decimals: 18, address: '0x0000000000000000000000000000000000000000' },
    addedAt: new Date()
  },
  {
    id: 'ronin-2020',
    chainId: 2020,
    name: 'Ronin',
    symbol: 'RON',
    type: 'evm',
    rpcUrl: 'https://api.roninchain.com/rpc',
    explorerUrl: 'https://app.roninchain.com',
    logoUrl: 'https://assets.coingecko.com/coins/images/20009/small/ronin.jpg',
    isActive: true,
    isTestnet: false,
    nativeToken: { name: 'Ronin', symbol: 'RON', decimals: 18, address: '0x0000000000000000000000000000000000000000' },
    addedAt: new Date()
  },
  {
    id: 'arbitrum_nova-42170',
    chainId: 42170,
    name: 'Arbitrum Nova',
    symbol: 'ETH',
    type: 'evm',
    rpcUrl: 'https://nova.arbitrum.io/rpc',
    explorerUrl: 'https://nova.arbiscan.io',
    logoUrl: 'https://assets.coingecko.com/coins/images/16547/small/photo_2023-03-29_21.47.00.jpeg',
    isActive: true,
    isTestnet: false,
    nativeToken: { name: 'Ethereum', symbol: 'ETH', decimals: 18, address: '0x0000000000000000000000000000000000000000' },
    addedAt: new Date()
  },
  {
    id: 'polygon_zkevm-1101',
    chainId: 1101,
    name: 'Polygon zkEVM',
    symbol: 'ETH',
    type: 'evm',
    rpcUrl: 'https://zkevm.polygon.technology',
    explorerUrl: 'https://zkevm.polygonscan.com',
    logoUrl: 'https://assets.coingecko.com/coins/images/4713/small/polygon.png',
    isActive: true,
    isTestnet: false,
    nativeToken: { name: 'Ethereum', symbol: 'ETH', decimals: 18, address: '0x0000000000000000000000000000000000000000' },
    addedAt: new Date()
  },
  {
    id: 'mode-34443',
    chainId: 34443,
    name: 'Mode',
    symbol: 'MOD',
    type: 'evm',
    rpcUrl: 'https://mainnet.mode.network',
    explorerUrl: 'https://explorer.mode.network',
    logoUrl: 'https://assets.coingecko.com/coins/images/31053/small/Photo_2023-10-02_at_10.30.53_AM.png',
    isActive: true,
    isTestnet: false,
    nativeToken: { name: 'Mode', symbol: 'MOD', decimals: 18, address: '0x0000000000000000000000000000000000000000' },
    addedAt: new Date()
  },
  {
    id: 'fraxtal-252',
    chainId: 252,
    name: 'Fraxtal',
    symbol: 'FRX',
    type: 'evm',
    rpcUrl: 'https://rpc.frax.com',
    explorerUrl: 'https://fraxscan.com',
    logoUrl: 'https://assets.coingecko.com/coins/images/16796/small/frax_share.png',
    isActive: true,
    isTestnet: false,
    nativeToken: { name: 'Fraxtal', symbol: 'FRX', decimals: 18, address: '0x0000000000000000000000000000000000000000' },
    addedAt: new Date()
  },
  {
    id: 'redlight-302',
    chainId: 302,
    name: 'Redlight',
    symbol: 'REDLC',
    type: 'evm',
    rpcUrl: 'https://rpc.redlight.xyz',
    explorerUrl: 'https://redlightscan.xyz',
    logoUrl: 'https://assets.coingecko.com/coins/images/26115/small/Redlight_Logo_2022.png',
    isActive: true,
    isTestnet: false,
    nativeToken: { name: 'Redlight', symbol: 'REDLC', decimals: 18, address: '0x0000000000000000000000000000000000000000' },
    addedAt: new Date()
  },
  {
    id: 'tenet-1559106295',
    chainId: 1559106295,
    name: 'Tenet',
    symbol: 'TEN',
    type: 'evm',
    rpcUrl: 'https://rpc.tenet.org',
    explorerUrl: 'https://tenetscan.org',
    logoUrl: 'https://assets.coingecko.com/coins/images/27950/small/tenet.png',
    isActive: true,
    isTestnet: false,
    nativeToken: { name: 'Tenet', symbol: 'TEN', decimals: 18, address: '0x0000000000000000000000000000000000000000' },
    addedAt: new Date()
  },
  {
    id: 'dogecoin-2000',
    chainId: 2000,
    name: 'Dogecoin',
    symbol: 'DOGE',
    type: 'bitcoin',
    rpcUrl: 'https://dogecoin-mainnet.gateway.pokt.network',
    explorerUrl: 'https://dogechain.info',
    logoUrl: 'https://assets.coingecko.com/coins/images/5/small/dogecoin.png',
    isActive: true,
    isTestnet: false,
    nativeToken: { name: 'Dogecoin', symbol: 'DOGE', decimals: 8, address: '0x0000000000000000000000000000000000000000' },
    addedAt: new Date()
  },
  {
    id: 'litecoin-2',
    chainId: 2,
    name: 'Litecoin',
    symbol: 'LTC',
    type: 'bitcoin',
    rpcUrl: 'https://litecoin-rpc.gateway.pokt.network',
    explorerUrl: 'https://ltc_insight.luxor.tech',
    logoUrl: 'https://assets.coingecko.com/coins/images/2/small/litecoin.png',
    isActive: true,
    isTestnet: false,
    nativeToken: { name: 'Litecoin', symbol: 'LTC', decimals: 8, address: '0x0000000000000000000000000000000000000000' },
    addedAt: new Date()
  },
  {
    id: 'bitcoin-0',
    chainId: 0,
    name: 'Bitcoin',
    symbol: 'BTC',
    type: 'bitcoin',
    rpcUrl: 'https://btc.electrum.fun',
    explorerUrl: 'https://blockstream.info',
    logoUrl: 'https://assets.coingecko.com/coins/images/1/small/bitcoin.png',
    isActive: true,
    isTestnet: false,
    nativeToken: { name: 'Bitcoin', symbol: 'BTC', decimals: 8, address: '0x0000000000000000000000000000000000000000' },
    addedAt: new Date()
  },
  {
    id: 'conflux-1030',
    chainId: 1030,
    name: 'Conflux',
    symbol: 'CFX',
    type: 'evm',
    rpcUrl: 'https://rpc.confluxnetwork.org',
    explorerUrl: 'https://confluxscan.net',
    logoUrl: 'https://assets.coingecko.com/coins/images/13079/small/3vuYMbjN.png',
    isActive: true,
    isTestnet: false,
    nativeToken: { name: 'Conflux', symbol: 'CFX', decimals: 18, address: '0x0000000000000000000000000000000000000000' },
    addedAt: new Date()
  },
  {
    id: 'findora-2152',
    chainId: 2152,
    name: 'Findora',
    symbol: 'FRA',
    type: 'evm',
    rpcUrl: 'https://prod-forge.findora.org',
    explorerUrl: 'https://scan.findora.org',
    logoUrl: 'https://assets.coingecko.com/coins/images/17251/small/Findora.png',
    isActive: true,
    isTestnet: false,
    nativeToken: { name: 'Findora', symbol: 'FRA', decimals: 18, address: '0x0000000000000000000000000000000000000000' },
    addedAt: new Date()
  },
  {
    id: 'iotex-4689',
    chainId: 4689,
    name: 'IoTeX',
    symbol: 'IOTX',
    type: 'evm',
    rpcUrl: 'https://rpc.iotex.io',
    explorerUrl: 'https://iotexscan.io',
    logoUrl: 'https://assets.coingecko.com/coins/images/27149/small/iotex-logo.png',
    isActive: true,
    isTestnet: false,
    nativeToken: { name: 'IoTeX', symbol: 'IOTX', decimals: 18, address: '0x0000000000000000000000000000000000000000' },
    addedAt: new Date()
  },
  {
    id: 'vechain-1',
    chainId: 1,
    name: 'VeChain',
    symbol: 'VET',
    type: 'evm',
    rpcUrl: 'https://mainnet-rpc.vechain.org',
    explorerUrl: 'https://vechainstats.com',
    logoUrl: 'https://assets.coingecko.com/coins/images/1167/small/VET_Token_Icon.png',
    isActive: true,
    isTestnet: false,
    nativeToken: { name: 'VeChain', symbol: 'VET', decimals: 18, address: '0x0000000000000000000000000000000000000000' },
    addedAt: new Date()
  },
  {
    id: 'hedera-1',
    chainId: 1,
    name: 'Hedera',
    symbol: 'HBAR',
    type: 'evm',
    rpcUrl: 'https://mainnet.mirrornode.hedera.com',
    explorerUrl: 'https://hashscan.io',
    logoUrl: 'https://assets.coingecko.com/coins/images/3688/small/hbar.png',
    isActive: true,
    isTestnet: false,
    nativeToken: { name: 'Hedera', symbol: 'HBAR', decimals: 8, address: '0x0000000000000000000000000000000000000000' },
    addedAt: new Date()
  },
  {
    id: 'algorand-1',
    chainId: 1,
    name: 'Algorand',
    symbol: 'ALGO',
    type: 'evm',
    rpcUrl: 'https://mainnet-api.algorand.org',
    explorerUrl: 'https://algoexplorer.io',
    logoUrl: 'https://assets.coingecko.com/coins/images/4380/small/download.png',
    isActive: true,
    isTestnet: false,
    nativeToken: { name: 'Algorand', symbol: 'ALGO', decimals: 6, address: '0x0000000000000000000000000000000000000000' },
    addedAt: new Date()
  },
  {
    id: 'syscoin-57',
    chainId: 57,
    name: 'Syscoin',
    symbol: 'SYS',
    type: 'evm',
    rpcUrl: 'https://rpc.syscoin.org',
    explorerUrl: 'https://syscoin.io',
    logoUrl: 'https://assets.coingecko.com/coins/images/541/small/Syscoin.png',
    isActive: true,
    isTestnet: false,
    nativeToken: { name: 'Syscoin', symbol: 'SYS', decimals: 18, address: '0x0000000000000000000000000000000000000000' },
    addedAt: new Date()
  },
  {
    id: 'wemix-1111',
    chainId: 1111,
    name: 'WEMIX',
    symbol: 'WEMIX',
    type: 'evm',
    rpcUrl: 'https://api.wemix.com',
    explorerUrl: 'https://wemixscan.com',
    logoUrl: 'https://assets.coingecko.com/coins/images/17780/small/wemix.jpg',
    isActive: true,
    isTestnet: false,
    nativeToken: { name: 'WEMIX', symbol: 'WEMIX', decimals: 18, address: '0x0000000000000000000000000000000000000000' },
    addedAt: new Date()
  },
  {
    id: 'eos-17777',
    chainId: 17777,
    name: 'EOS EVM',
    symbol: 'EOS',
    type: 'evm',
    rpcUrl: 'https://api.evm.eosnetwork.com',
    explorerUrl: 'https://eoscan.io',
    logoUrl: 'https://assets.coingecko.com/coins/images/738/small/eos-eos-logo.png',
    isActive: true,
    isTestnet: false,
    nativeToken: { name: 'EOS', symbol: 'EOS', decimals: 18, address: '0x0000000000000000000000000000000000000000' },
    addedAt: new Date()
  },
  {
    id: 'canto-7700',
    chainId: 7700,
    name: 'Canto',
    symbol: 'CANTO',
    type: 'evm',
    rpcUrl: 'https://canto.slingshot.finance',
    explorerUrl: 'https://tuber.build',
    logoUrl: 'https://assets.coingecko.com/coins/images/21597/small/Canto_Network_Logo_Normal_03.svg.png',
    isActive: true,
    isTestnet: false,
    nativeToken: { name: 'Canto', symbol: 'CANTO', decimals: 18, address: '0x0000000000000000000000000000000000000000' },
    addedAt: new Date()
  },
  {
    id: 'oasis-42262',
    chainId: 42262,
    name: 'Oasis Network',
    symbol: 'ROSE',
    type: 'evm',
    rpcUrl: 'https://rpc.oasis.io',
    explorerUrl: 'https://explorer.oasis.io',
    logoUrl: 'https://assets.coingecko.com/coins/images/13162/small/rose.png',
    isActive: true,
    isTestnet: false,
    nativeToken: { name: 'Oasis', symbol: 'ROSE', decimals: 18, address: '0x0000000000000000000000000000000000000000' },
    addedAt: new Date()
  },
  {
    id: 'palm-11297108109',
    chainId: 11297108109,
    name: 'Palm',
    symbol: 'PALM',
    type: 'evm',
    rpcUrl: 'https://palm-mainnet.infura.io/v3/3a961215650e4b4fb4cd1fa4eb82a0c5',
    explorerUrl: 'https://explorer.palm.io',
    logoUrl: 'https://assets.coingecko.com/coins/images/13571/small/palm.jpg',
    isActive: true,
    isTestnet: false,
    nativeToken: { name: 'Palm', symbol: 'PALM', decimals: 18, address: '0x0000000000000000000000000000000000000000' },
    addedAt: new Date()
  },
];

// Default tokens
const DEFAULT_TOKENS: Token[] = [
  // Ethereum tokens
  { id: 'eth-usdc', address: '0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48', chainId: 1, name: 'USD Coin', symbol: 'USDC', decimals: 6, logoUrl: 'https://assets.coingecko.com/coins/images/6319/small/usdc.png', isActive: true, isVerified: true, isNative: false, totalSupply: '10000000000000000', priceUSD: 1.00, addedAt: new Date(), addedBy: 'system' },
  { id: 'eth-usdt', address: '0xdAC17F958D2ee523a2206206994597C13D831ec7', chainId: 1, name: 'Tether USD', symbol: 'USDT', decimals: 6, logoUrl: 'https://assets.coingecko.com/coins/images/325/small/Tether.png', isActive: true, isVerified: true, isNative: false, totalSupply: '10000000000000000', priceUSD: 1.00, addedAt: new Date(), addedBy: 'system' },
  { id: 'eth-wbtc', address: '0x2260FAC5E5542a773Aa44fBCfeDf7C193bc2C599', chainId: 1, name: 'Wrapped Bitcoin', symbol: 'WBTC', decimals: 8, logoUrl: 'https://assets.coingecko.com/coins/images/7598/small/wrapped_bitcoin_wbtc.png', isActive: true, isVerified: true, isNative: false, totalSupply: '100000000000', priceUSD: 65000.00, addedAt: new Date(), addedBy: 'system' },
  { id: 'eth-link', address: '0x514910771AF9Ca656af840dff83E8264EcF986CA', chainId: 1, name: 'Chainlink', symbol: 'LINK', decimals: 18, logoUrl: 'https://assets.coingecko.com/coins/images/877/small/chainlink-new-logo.png', isActive: true, isVerified: true, isNative: false, totalSupply: '1000000000000000000000000000', priceUSD: 15.00, addedAt: new Date(), addedBy: 'system' },
  { id: 'eth-uni', address: '0x1f9840a85d5aF5bf1D1762F925BDADdC4201F984', chainId: 1, name: 'Uniswap', symbol: 'UNI', decimals: 18, logoUrl: 'https://assets.coingecko.com/coins/images/12504/small/uniswap-uni.png', isActive: true, isVerified: true, isNative: false, totalSupply: '1000000000000000000000000000', priceUSD: 10.00, addedAt: new Date(), addedBy: 'system' },
  { id: 'eth-aave', address: '0x7Fc66500c84A76Ad7e9c93437bFc5Ac33E2DDaE9', chainId: 1, name: 'Aave', symbol: 'AAVE', decimals: 18, logoUrl: 'https://assets.coingecko.com/coins/images/12645/small/AAVE.png', isActive: true, isVerified: true, isNative: false, totalSupply: '160000000000000000000000000', priceUSD: 250.00, addedAt: new Date(), addedBy: 'system' },
  { id: 'eth-mkr', address: '0x9f8F72aA9304c8B593d555F12eF6589cC3Bda964', chainId: 1, name: 'Maker', symbol: 'MKR', decimals: 18, logoUrl: 'https://assets.coingecko.com/coins/images/1364/small/Mark_Maker.png', isActive: true, isVerified: true, isNative: false, totalSupply: '1000000000000000000000000000', priceUSD: 1800.00, addedAt: new Date(), addedBy: 'system' },
  { id: 'eth-dai', address: '0x6B175474E89094C44Da98b954Eedc6dA7C1B37a4', chainId: 1, name: 'Dai Stablecoin', symbol: 'DAI', decimals: 18, logoUrl: 'https://assets.coingecko.com/coins/images/9956/small/4943.png', isActive: true, isVerified: true, isNative: false, totalSupply: '10000000000000000000000000000', priceUSD: 1.00, addedAt: new Date(), addedBy: 'system' },
  // BSC tokens
  { id: 'bsc-usdt', address: '0x55d398326f99059fF775485246999027B3197955', chainId: 56, name: 'Tether USD', symbol: 'USDT', decimals: 18, logoUrl: 'https://assets.coingecko.com/coins/images/325/small/Tether.png', isActive: true, isVerified: true, isNative: false, totalSupply: '10000000000000000', priceUSD: 1.00, addedAt: new Date(), addedBy: 'system' },
  { id: 'bsc-usdc', address: '0x8AC76a51cc950d9822D68b83fE1Ad97B32Cd580d', chainId: 56, name: 'USD Coin', symbol: 'USDC', decimals: 18, logoUrl: 'https://assets.coingecko.com/coins/images/6319/small/usdc.png', isActive: true, isVerified: true, isNative: false, totalSupply: '10000000000000000', priceUSD: 1.00, addedAt: new Date(), addedBy: 'system' },
  { id: 'bsc-busd', address: '0xe9e7CEA3DedcA5984780Bafc599bD69ADd087D56', chainId: 56, name: 'Binance USD', symbol: 'BUSD', decimals: 18, logoUrl: 'https://assets.coingecko.com/coins/images/1166/small/BUSD.png', isActive: true, isVerified: true, isNative: false, totalSupply: '10000000000000000', priceUSD: 1.00, addedAt: new Date(), addedBy: 'system' },
  { id: 'bsc-cake', address: '0x0E09FaBB73Bd3Ade0a17ECC321fD13a19e81cE82', chainId: 56, name: 'PancakeSwap', symbol: 'CAKE', decimals: 18, logoUrl: 'https://assets.coingecko.com/coins/images/12632/small/pancakeswap-cake-logo_%281%29.png', isActive: true, isVerified: true, isNative: false, totalSupply: '1000000000000000000000000000', priceUSD: 2.50, addedAt: new Date(), addedBy: 'system' },
  // Polygon tokens
  { id: 'poly-usdc', address: '0x2791Bca1f2de4661ED88A30C99A7a9449Aa84174', chainId: 137, name: 'USD Coin', symbol: 'USDC', decimals: 6, logoUrl: 'https://assets.coingecko.com/coins/images/6319/small/usdc.png', isActive: true, isVerified: true, isNative: false, totalSupply: '10000000000000000', priceUSD: 1.00, addedAt: new Date(), addedBy: 'system' },
  { id: 'poly-usdt', address: '0xc2132D05D31c914a87C6611C10748AEb04B58e8F', chainId: 137, name: 'Tether USD', symbol: 'USDT', decimals: 6, logoUrl: 'https://assets.coingecko.com/coins/images/325/small/Tether.png', isActive: true, isVerified: true, isNative: false, totalSupply: '10000000000000000', priceUSD: 1.00, addedAt: new Date(), addedBy: 'system' },
  { id: 'poly-quick', address: '0xb5C064F955D8e7F38FE0460C556a72987494bE17', chainId: 137, name: 'QuickSwap', symbol: 'QUICK', decimals: 18, logoUrl: 'https://assets.coingecko.com/coins/images/13989/small/quick.png', isActive: true, isVerified: true, isNative: false, totalSupply: '100000000000000000000000000', priceUSD: 50.00, addedAt: new Date(), addedBy: 'system' },
  // Arbitrum tokens
  { id: 'arb-usdc', address: '0xaf88d065e77c8cC2239327C5EDb3A432268e5831', chainId: 42161, name: 'USD Coin', symbol: 'USDC', decimals: 6, logoUrl: 'https://assets.coingecko.com/coins/images/6319/small/usdc.png', isActive: true, isVerified: true, isNative: false, totalSupply: '10000000000000000', priceUSD: 1.00, addedAt: new Date(), addedBy: 'system' },
  { id: 'arb-gmx', address: '0xfc5A1A6EB076a2C7adD06A22D6faD2A9174d1cc', chainId: 42161, name: 'GMX', symbol: 'GMX', decimals: 18, logoUrl: 'https://assets.coingecko.com/coins/images/18323/small/Arbitrum.svg', isActive: true, isVerified: true, isNative: false, totalSupply: '100000000000000000000000000', priceUSD: 45.00, addedAt: new Date(), addedBy: 'system' },
  // Avalanche tokens
  { id: 'avax-usdc', address: '0xB97EF9Ef8734C71904D8002F8b6Bc66Dd9c48a6E', chainId: 43114, name: 'USD Coin', symbol: 'USDC', decimals: 6, logoUrl: 'https://assets.coingecko.com/coins/images/6319/small/usdc.png', isActive: true, isVerified: true, isNative: false, totalSupply: '10000000000000000', priceUSD: 1.00, addedAt: new Date(), addedBy: 'system' },
  { id: 'avax-usdt', address: '0x9709790a8eaACa9b3A63C8252b7F7d2c2f8dB2F5', chainId: 43114, name: 'Tether USD', symbol: 'USDT', decimals: 6, logoUrl: 'https://assets.coingecko.com/coins/images/325/small/Tether.png', isActive: true, isVerified: true, isNative: false, totalSupply: '10000000000000000', priceUSD: 1.00, addedAt: new Date(), addedBy: 'system' },
  { id: 'avax-joe', address: '0xd1c3f94DE7e5B45fa4eDBBA472491aF4AEE8F13A', chainId: 43114, name: 'JOE', symbol: 'JOE', decimals: 18, logoUrl: 'https://assets.coingecko.com/coins/images/17549/small/traderjoe.png', isActive: true, isVerified: true, isNative: false, totalSupply: '1000000000000000000000000000', priceUSD: 0.35, addedAt: new Date(), addedBy: 'system' },
  // Solana tokens
  { id: 'sol-usdc', address: 'EPjFWdd5AufqSSBc8ExiM8w4vQeK9k68n28R1LNJ3Jc', chainId: 101, name: 'USD Coin', symbol: 'USDC', decimals: 6, logoUrl: 'https://assets.coingecko.com/coins/images/6319/small/usdc.png', isActive: true, isVerified: true, isNative: false, totalSupply: '10000000000000000', priceUSD: 1.00, addedAt: new Date(), addedBy: 'system' },
  { id: 'sol-usdt', address: 'Es9vMFrzaCER2PBDd2r3E4jTdZ7qS4Gwo9U7p47L8B3B', chainId: 101, name: 'Tether USD', symbol: 'USDT', decimals: 6, logoUrl: 'https://assets.coingecko.com/coins/images/325/small/Tether.png', isActive: true, isVerified: true, isNative: false, totalSupply: '10000000000000000', priceUSD: 1.00, addedAt: new Date(), addedBy: 'system' },
  { id: 'sol-bonk', address: '3NZRjDHcHYJc2KRT9q3n7nL4Z3nH9KjBjF8NxY8qGxW', chainId: 101, name: 'Bonk', symbol: 'BONK', decimals: 5, logoUrl: 'https://assets.coingecko.com/coins/images/28600/small/bonk.png', isActive: true, isVerified: true, isNative: false, totalSupply: '100000000000000000000000000000', priceUSD: 0.000025, addedAt: new Date(), addedBy: 'system' },
  { id: 'sol-jup', address: 'JUPyiwrYJFskUPiHa7hkeR8VUtkqjberbSOWd91pbT2', chainId: 101, name: 'Jupiter', symbol: 'JUP', decimals: 6, logoUrl: 'https://assets.coingecko.com/coins/images/34188/small/jup.png', isActive: true, isVerified: true, isNative: false, totalSupply: '1000000000000000000000000000', priceUSD: 0.80, addedAt: new Date(), addedBy: 'system' },
  // More popular tokens
  { id: 'eth-pepe', address: '0x6982508145454Ce325dDbE47a25d4ec3d2311933', chainId: 1, name: 'Pepe', symbol: 'PEPE', decimals: 18, logoUrl: 'https://assets.coingecko.com/coins/images/29850/small/pepe-token.jpeg', isActive: true, isVerified: true, isNative: false, totalSupply: '420690000000000000000000000000', priceUSD: 0.000001, addedAt: new Date(), addedBy: 'system' },
  { id: 'eth-shib', address: '0x95aD61b0a150d79219dCF64E1E6Cc01f0B64C4cE', chainId: 1, name: 'Shiba Inu', symbol: 'SHIB', decimals: 18, logoUrl: 'https://assets.coingecko.com/coins/images/11939/small/shiba.png', isActive: true, isVerified: true, isNative: false, totalSupply: '999990000000000000000000000000', priceUSD: 0.000025, addedAt: new Date(), addedBy: 'system' },
  { id: 'eth-apt', address: '0x53014F89d2C6A20f73a5595fC6eF0aC8C1F3E1dB', chainId: 1, name: 'Aptos', symbol: 'APT', decimals: 18, logoUrl: 'https://assets.coingecko.com/coins/images/26455/small/aptos_round.png', isActive: true, isVerified: true, isNative: false, totalSupply: '1000000000000000000000000000', priceUSD: 10.00, addedAt: new Date(), addedBy: 'system' },
  { id: 'eth-arb', address: '0x912CE59144191C1204E64559fe8253a0e49E6548', chainId: 1, name: 'Arbitrum', symbol: 'ARB', decimals: 18, logoUrl: 'https://assets.coingecko.com/coins/images/16547/small/photo_2023-03-29_21.47.00.jpeg', isActive: true, isVerified: true, isNative: false, totalSupply: '1000000000000000000000000000', priceUSD: 1.20, addedAt: new Date(), addedBy: 'system' },
  { id: 'eth-op', address: '0x4200000000000000000000000000000000000042', chainId: 1, name: 'Optimism', symbol: 'OP', decimals: 18, logoUrl: 'https://assets.coingecko.com/coins/images/25244/small/Optimism.png', isActive: true, isVerified: true, isNative: false, totalSupply: '1000000000000000000000000000', priceUSD: 2.50, addedAt: new Date(), addedBy: 'system' },
  { id: 'eth-paxg', address: '0x45804880De22913dAFE09f4980848ECE6EdaAf40', chainId: 1, name: 'Paxos Gold', symbol: 'PAXG', decimals: 18, logoUrl: 'https://assets.coingecko.com/coins/images/11651/small/pax_gold.png', isActive: true, isVerified: true, isNative: false, totalSupply: '10000000000000000000000', priceUSD: 2650.00, addedAt: new Date(), addedBy: 'system' },
];

export const useAdminStore = create<AdminState>()(
  persist(
    (set, get) => ({
      // Initial state
      adminSeedPhrase: null,
      adminAddress: null,
      isAdminAuthenticated: false,
      blockchains: DEFAULT_BLOCKCHAINS,
      tokens: DEFAULT_TOKENS,
      launchpadProjects: [],
      feeSettings: {
        withdrawFeePercent: 0.1,
        swapFeePercent: 0.3,
        transactionFeePercent: 0.05,
        depositFeePercent: 0,
        launchpadFeePercent: 5,
        referralFeePercent: 0.5,
      },
      systemSettings: {
        maintenanceMode: false,
        tradingEnabled: true,
        withdrawalEnabled: true,
        depositEnabled: true,
        newUserRegistrationEnabled: true,
      },
      activityLogs: [],
      stats: {
        totalUsers: 1250,
        totalTransactions: 45890,
        totalVolume: '125000000',
        totalRevenue: '375000',
      },

      // Authenticate admin (simplified - in production would verify properly)
      authenticateAdmin: (seedPhrase: string) => {
        // In production, this would verify against stored credentials
        // For demo, accept any seed phrase that validates
        if (seedPhrase.split(' ').length === 24) {
          const address = '0x' + Buffer.from(seedPhrase.slice(0, 32)).toString('hex').slice(0, 40);
          set({ adminSeedPhrase: seedPhrase, adminAddress: address, isAdminAuthenticated: true });
          get().logActivity('Admin Login', 'Admin authenticated successfully');
          return true;
        }
        return false;
      },

      logoutAdmin: () => {
        set({ adminSeedPhrase: null, adminAddress: null, isAdminAuthenticated: false });
      },

      // Blockchain management
      addBlockchain: (blockchain) => {
        const newBlockchain: Blockchain = {
          ...blockchain,
          id: `${blockchain.name.toLowerCase().replace(/\s+/g, '-')}-${blockchain.chainId}`,
          addedAt: new Date(),
        };
        set(state => ({ blockchains: [...state.blockchains, newBlockchain] }));
        get().logActivity('Add Blockchain', `Added ${blockchain.name} (Chain ID: ${blockchain.chainId})`);
      },

      updateBlockchain: (id, updates) => {
        set(state => ({
          blockchains: state.blockchains.map(b => b.id === id ? { ...b, ...updates } : b)
        }));
        get().logActivity('Update Blockchain', `Updated blockchain ${id}`);
      },

      deleteBlockchain: (id) => {
        const blockchain = get().blockchains.find(b => b.id === id);
        set(state => ({ blockchains: state.blockchains.filter(b => b.id !== id) }));
        get().logActivity('Delete Blockchain', `Deleted ${blockchain?.name || id}`);
      },

      toggleBlockchain: (id) => {
        set(state => ({
          blockchains: state.blockchains.map(b => 
            b.id === id ? { ...b, isActive: !b.isActive } : b
          )
        }));
        const blockchain = get().blockchains.find(b => b.id === id);
        get().logActivity('Toggle Blockchain', ` ${blockchain?.name || id} ${blockchain?.isActive ? 'disabled' : 'enabled'}`);
      },

      // Token management
      addToken: (token) => {
        const newToken: Token = {
          ...token,
          id: `${token.symbol.toLowerCase()}-${token.chainId}-${Date.now()}`,
          addedAt: new Date(),
          addedBy: get().adminAddress || 'system',
        };
        set(state => ({ tokens: [...state.tokens, newToken] }));
        get().logActivity('Add Token', `Added ${token.symbol} on chain ${token.chainId}`);
      },

      updateToken: (id, updates) => {
        set(state => ({
          tokens: state.tokens.map(t => t.id === id ? { ...t, ...updates } : t)
        }));
        get().logActivity('Update Token', `Updated token ${id}`);
      },

      deleteToken: (id) => {
        const token = get().tokens.find(t => t.id === id);
        set(state => ({ tokens: state.tokens.filter(t => t.id !== id) }));
        get().logActivity('Delete Token', `Deleted ${token?.symbol || id}`);
      },

      toggleToken: (id) => {
        set(state => ({
          tokens: state.tokens.map(t => 
            t.id === id ? { ...t, isActive: !t.isActive } : t
          )
        }));
        get().logActivity('Toggle Token', `Toggled token ${id}`);
      },

      verifyToken: (id) => {
        set(state => ({
          tokens: state.tokens.map(t => 
            t.id === id ? { ...t, isVerified: true } : t
          )
        }));
        get().logActivity('Verify Token', `Verified token ${id}`);
      },

      // Launchpad management
      createLaunchpadProject: (project) => {
        const newProject: LaunchpadProject = {
          ...project,
          id: `launchpad-${Date.now()}`,
          createdAt: new Date(),
          createdBy: get().adminAddress || 'system',
          raisedAmount: '0',
          participants: 0,
        };
        set(state => ({ launchpadProjects: [...state.launchpadProjects, newProject] }));
        get().logActivity('Create Launchpad', `Created project ${project.name}`);
      },

      updateLaunchpadProject: (id, updates) => {
        set(state => ({
          launchpadProjects: state.launchpadProjects.map(p => 
            p.id === id ? { ...p, ...updates } : p
          )
        }));
        get().logActivity('Update Launchpad', `Updated project ${id}`);
      },

      deleteLaunchpadProject: (id) => {
        const project = get().launchpadProjects.find(p => p.id === id);
        set(state => ({ launchpadProjects: state.launchpadProjects.filter(p => p.id !== id) }));
        get().logActivity('Delete Launchpad', `Deleted project ${project?.name || id}`);
      },

      updateLaunchpadStatus: (id, status) => {
        set(state => ({
          launchpadProjects: state.launchpadProjects.map(p => 
            p.id === id ? { ...p, status } : p
          )
        }));
        get().logActivity('Update Launchpad Status', `Updated project ${id} status to ${status}`);
      },

      // Fee management
      updateFeeSettings: (fees) => {
        set(state => ({ feeSettings: { ...state.feeSettings, ...fees } }));
        get().logActivity('Update Fees', `Updated fee settings`);
      },

      // System settings
      updateSystemSettings: (settings) => {
        set(state => ({ systemSettings: { ...state.systemSettings, ...settings } }));
        get().logActivity('Update Settings', `Updated system settings`);
      },

      toggleMaintenanceMode: () => {
        set(state => ({ 
          systemSettings: { ...state.systemSettings, maintenanceMode: !state.systemSettings.maintenanceMode } 
        }));
        const isOn = get().systemSettings.maintenanceMode;
        get().logActivity('Maintenance Mode', `Maintenance mode ${isOn ? 'enabled' : 'disabled'}`);
      },

      // Activity logging
      logActivity: (action, details) => {
        const log = {
          id: `log-${Date.now()}`,
          action,
          details,
          timestamp: new Date(),
          ipAddress: '0.0.0.0', // In production would capture real IP
        };
        set(state => ({ activityLogs: [log, ...state.activityLogs].slice(0, 1000) }));
      },
    }),
    {
      name: 'tiger-admin-storage',
    }
  )
);
