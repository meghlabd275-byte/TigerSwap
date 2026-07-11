import { create } from 'zustand';
import { persist } from 'zustand/middleware';

export interface Chain {
  id: number;
  name: string;
  symbol: string;
  icon: string;
  rpc: string;
  explorer: string;
  type: 'evm' | 'solana' | 'aptos' | 'cosmos' | 'ton';
}

export interface Token {
  address: string;
  symbol: string;
  name: string;
  decimals: number;
  logoURI: string;
  chainId: number;
  price?: number;
  balance?: string;
}

export interface SupportedChains {
  [key: string]: Chain;
}

export const SUPPORTED_CHAINS: SupportedChains = {
  // EVM Chains
  ethereum: {
    id: 1,
    name: 'Ethereum',
    symbol: 'ETH',
    icon: '🦄',
    rpc: 'https://eth.llamarpc.com',
    explorer: 'https://etherscan.io',
    type: 'evm'
  },
  polygon: {
    id: 137,
    name: 'Polygon',
    symbol: 'MATIC',
    icon: '🔷',
    rpc: 'https://polygon.llamarpc.com',
    explorer: 'https://polygonscan.com',
    type: 'evm'
  },
  arbitrum: {
    id: 42161,
    name: 'Arbitrum One',
    symbol: 'ETH',
    icon: '🔵',
    rpc: 'https://arb1.arbitrum.io/rpc',
    explorer: 'https://arbiscan.io',
    type: 'evm'
  },
  optimism: {
    id: 10,
    name: 'Optimism',
    symbol: 'ETH',
    icon: '🔴',
    rpc: 'https://mainnet.optimism.io',
    explorer: 'https://optimistic.etherscan.io',
    type: 'evm'
  },
  base: {
    id: 8453,
    name: 'Base',
    symbol: 'ETH',
    icon: '🔵',
    rpc: 'https://mainnet.base.org',
    explorer: 'https://basescan.org',
    type: 'evm'
  },
  bsc: {
    id: 56,
    name: 'BNB Smart Chain',
    symbol: 'BNB',
    icon: '🟡',
    rpc: 'https://bsc-dataseed.binance.org',
    explorer: 'https://bscscan.com',
    type: 'evm'
  },
  avalanche: {
    id: 43114,
    name: 'Avalanche',
    symbol: 'AVAX',
    icon: '🔺',
    rpc: 'https://api.avax.network/ext/bc/C/rpc',
    explorer: 'https://snowtrace.io',
    type: 'evm'
  },
  fantom: {
    id: 250,
    name: 'Fantom',
    symbol: 'FTM',
    icon: '👻',
    rpc: 'https://rpc.fantom.network',
    explorer: 'https://ftmscan.com',
    type: 'evm'
  },
  cronos: {
    id: 25,
    name: 'Cronos',
    symbol: 'CRO',
    icon: '🌟',
    rpc: 'https://evm.cronos.org',
    explorer: 'https://cronoscan.com',
    type: 'evm'
  },
  aurora: {
    id: 1313161554,
    name: 'Aurora',
    symbol: 'ETH',
    icon: '🌅',
    rpc: 'https://mainnet.aurora.dev',
    explorer: 'https://explorer.aurora.dev',
    type: 'evm'
  },
  // Solana
  solana: {
    id: 101,
    name: 'Solana',
    symbol: 'SOL',
    icon: '☀️',
    rpc: 'https://api.mainnet-beta.solana.com',
    explorer: 'https://solscan.io',
    type: 'solana'
  },
  // Aptos
  aptos: {
    id: 1,
    name: 'Aptos',
    symbol: 'APT',
    icon: '🔷',
    rpc: 'https://fullnode.mainnet.aptoslabs.com',
    explorer: 'https://aptoscan.com',
    type: 'aptos'
  },
  // Cosmos
  cosmos: {
    id: 1,
    name: 'Cosmos',
    symbol: 'ATOM',
    icon: '🌌',
    rpc: 'https://rpc.cosmos.network',
    explorer: 'https://mintscan.io/cosmos',
    type: 'cosmos'
  },
  // TON
  ton: {
    id: 1,
    name: 'TON',
    symbol: 'TON',
    icon: '📱',
    rpc: 'https://toncenter.com/api/v2/jsonRPC',
    explorer: 'https://tonscan.org',
    type: 'ton'
  },
  // Arbitrum Nova
  arbitrum_nova: {
    id: 42170,
    name: 'Arbitrum Nova',
    symbol: 'ETH',
    icon: '🔵',
    rpc: 'https://nova.arbitrum.io/rpc',
    explorer: 'https://nova.arbiscan.io',
    type: 'evm'
  },
  // zkSync Era
  zksync: {
    id: 324,
    name: 'zkSync Era',
    symbol: 'ETH',
    icon: '⚡',
    rpc: 'https://mainnet.era.zksync.io',
    explorer: 'https://explorer.zksync.io',
    type: 'evm'
  },
  // Linea
  linea: {
    id: 59144,
    name: 'Linea',
    symbol: 'ETH',
    icon: '📐',
    rpc: 'https://rpc.linea.build',
    explorer: 'https://lineascan.build',
    type: 'evm'
  },
  // Monad Testnet (for upcoming support)
  monad: {
    id: 10143,
    name: 'Monad',
    symbol: 'MON',
    icon: '🐲',
    rpc: 'https://rpc.monad.xyz',
    explorer: 'https://explorer.monad.xyz',
    type: 'evm'
  },
  // Berachain
  berachain: {
    id: 80084,
    name: 'Berachain',
    symbol: 'BERA',
    icon: '🐻',
    rpc: 'https://rpc.berachain.com',
    explorer: 'https://berascan.com',
    type: 'evm'
  },
  // Sonic
  sonic: {
    id: 146,
    name: 'Sonic',
    symbol: 'S',
    icon: '🎵',
    rpc: 'https://rpc.soniclabs.com',
    explorer: 'https://sonicscan.org',
    type: 'evm'
  },
  // Pi Network (Testnet)
  pi: {
    id: 314159,
    name: 'Pi Network',
    symbol: 'PI',
    icon: 'π',
    rpc: 'https://rpc.πnetwork.io',
    explorer: 'https://explorer.πnetwork.io',
    type: 'evm'
  },
  // PulseChain
  pulse: {
    id: 369,
    name: 'PulseChain',
    symbol: 'PLS',
    icon: '💚',
    rpc: 'https://rpc.pulsechain.com',
    explorer: 'https://scan.pulsechain.com',
    type: 'evm'
  },
  // Sei
  sei: {
    id: 1,
    name: 'Sei',
    symbol: 'SEI',
    icon: '🐟',
    rpc: 'https://rpc.sei.io',
    explorer: 'https://seistream.app',
    type: 'cosmos'
  },
  // Injective
  injective: {
    id: 1,
    name: 'Injective',
    symbol: 'INJ',
    icon: '🦎',
    rpc: 'https://rpc.injective.network',
    explorer: 'https://explorer.injective.network',
    type: 'cosmos'
  },
  // Sui
  sui: {
    id: 1,
    name: 'Sui',
    symbol: 'SUI',
    icon: '💧',
    rpc: 'https://rpc.sui.io',
    explorer: 'https://suiscan.xyz',
    type: 'aptos'
  },
  // NEAR
  near: {
    id: 1,
    name: 'NEAR',
    symbol: 'NEAR',
    icon: '🌀',
    rpc: 'https://rpc.mainnet.near.org',
    explorer: 'https://explorer.near.org',
    type: 'evm'
  },
  // Algorand
  algorand: {
    id: 1,
    name: 'Algorand',
    symbol: 'ALGO',
    icon: '🔷',
    rpc: 'https://mainnet-api.algorand.org',
    explorer: 'https://algoexplorer.io',
    type: 'evm'
  },
  // Radix
  radix: {
    id: 1,
    name: 'Radix',
    symbol: 'XRD',
    icon: '🔴',
    rpc: 'https://radix-rpc.radixdlt.com',
    explorer: 'https://dashboard.radixdlt.com',
    type: 'evm'
  },
  // VeChain
  vechain: {
    id: 1,
    name: 'VeChain',
    symbol: 'VET',
    icon: '🔗',
    rpc: 'https://mainnet-rpc.vechain.org',
    explorer: 'https://vechainstats.com',
    type: 'evm'
  },
  // Hedera
  hedera: {
    id: 1,
    name: 'Hedera',
    symbol: 'HBAR',
    icon: '💎',
    rpc: 'https://mainnet.mirrornode.hedera.com',
    explorer: 'https://hashscan.io',
    type: 'evm'
  },
  // IOTA
  iota: {
    id: 1,
    name: 'IOTA',
    symbol: 'IOTA',
    icon: '🪶',
    rpc: 'https://api.iota.org',
    explorer: 'https://explorer.iota.org',
    type: 'evm'
  },
  // Aptos already added above
  // Core
  core: {
    id: 1116,
    name: 'Core',
    symbol: 'CORE',
    icon: '🔶',
    rpc: 'https://rpc.coredao.org',
    explorer: 'https://scan.coredao.org',
    type: 'evm'
  },
  // Canto
  canto: {
    id: 7700,
    name: 'Canto',
    symbol: 'CANTO',
    icon: '🎭',
    rpc: 'https://canto.slingshot.finance',
    explorer: 'https://tuber.build',
    type: 'evm'
  },
  // Kava
  kava: {
    id: 2222,
    name: 'Kava',
    symbol: 'KAVA',
    icon: '⚓',
    rpc: 'https://evm.kava.io',
    explorer: 'https://kavascan.com',
    type: 'evm'
  },
  // Celo
  celo: {
    id: 42220,
    name: 'Celo',
    symbol: 'CELO',
    icon: '🌕',
    rpc: 'https://forno.celo.org',
    explorer: 'https://explorer.celo.org',
    type: 'evm'
  },
  // Moonbeam
  moonbeam: {
    id: 1284,
    name: 'Moonbeam',
    symbol: 'GLMR',
    icon: '🌙',
    rpc: 'https://rpc.api.moonbeam.network',
    explorer: 'https://moonscan.io',
    type: 'evm'
  },
  // Moonriver
  moonriver: {
    id: 1285,
    name: 'Moonriver',
    symbol: 'MOVR',
    icon: '🌊',
    rpc: 'https://rpc.api.moonriver.moonbeam.network',
    explorer: 'https://moonriver.moonscan.io',
    type: 'evm'
  },
  // Astar
  astar: {
    id: 592,
    name: 'Astar',
    symbol: 'ASTR',
    icon: '⭐',
    rpc: 'https://rpc.astar.network',
    explorer: 'https://astar.explorer.io',
    type: 'evm'
  },
  // Shiden
  shiden: {
    id: 336,
    name: 'Shiden',
    symbol: 'SDN',
    icon: '⚡',
    rpc: 'https://rpc.shiden.astar.org',
    explorer: 'https://shiden.explorer.io',
    type: 'evm'
  },
  // Fuse
  fuse: {
    id: 122,
    name: 'Fuse',
    symbol: 'FUSE',
    icon: '🔥',
    rpc: 'https://rpc.fuse.io',
    explorer: 'https://explorer.fuse.io',
    type: 'evm'
  },
  // Evmos
  evmos: {
    id: 9001,
    name: 'Evmos',
    symbol: 'EVMOS',
    icon: '🌌',
    rpc: 'https://evmos-rpc.thewayscan.io',
    explorer: 'https://evmos.thewayscan.io',
    type: 'evm'
  },
  // Cronos
  // Already added above
  // KardiaChain
  kardia: {
    id: 24,
    name: 'KardiaChain',
    symbol: 'KAI',
    icon: '🔮',
    rpc: 'https://rpc.kardiachain.io',
    explorer: 'https://explorer.kardiachain.io',
    type: 'evm'
  },
  // Next Smart Chain
  next: {
    id: 1088,
    name: 'Next Smart Chain',
    symbol: 'NEXT',
    icon: '➡️',
    rpc: 'https://rpc.nextsmartchain.com',
    explorer: 'https://explorer.nextsmartchain.com',
    type: 'evm'
  },
  // opBNB
  opbnb: {
    id: 204,
    name: 'opBNB',
    symbol: 'BNB',
    icon: '🟠',
    rpc: 'https://opbnb-mainnet-rpc.bnbchain.org',
    explorer: 'https://opbnb.bscscan.com',
    type: 'evm'
  },
  // Scroll
  scroll: {
    id: 534352,
    name: 'Scroll',
    symbol: 'ETH',
    icon: '📜',
    rpc: 'https://rpc.scroll.io',
    explorer: 'https://scrollscan.com',
    type: 'evm'
  },
  // zkEVM
  zkevm: {
    id: 1101,
    name: 'Polygon zkEVM',
    symbol: 'ETH',
    icon: '🔶',
    rpc: 'https://zkevm-rpc.polygon.technology',
    explorer: 'https://zkevm.polygonscan.com',
    type: 'evm'
  },
  // Manta
  manta: {
    id: 1699,
    name: 'Manta Pacific',
    symbol: 'ETH',
    icon: '🐋',
    rpc: 'https://pacific-rpc.manta.network/http',
    explorer: 'https://pacific-explorer.manta.network',
    type: 'evm'
  },
  // Mantle
  mantle: {
    id: 5000,
    name: 'Mantle',
    symbol: 'MNT',
    icon: '🪙',
    rpc: 'https://rpc.mantle.xyz',
    explorer: 'https://mantlescan.info',
    type: 'evm'
  },
  // Fraxtal
  fraxtal: {
    id: 252,
    name: 'Fraxtal',
    symbol: 'FRX',
    icon: '🏛️',
    rpc: 'https://rpc.frax.com',
    explorer: 'https://fraxscan.com',
    type: 'evm'
  },
  // Blast
  blast: {
    id: 81457,
    name: 'Blast',
    symbol: 'ETH',
    icon: '💥',
    rpc: 'https://rpc.blast.io',
    explorer: 'https://blastscan.io',
    type: 'evm'
  },
  // MegaETH
  megaeth: {
    id: 1338,
    name: 'MegaETH',
    symbol: 'MEGA',
    icon: '🚀',
    rpc: 'https://rpc.megaeth.com',
    explorer: 'https://megaexplorer.io',
    type: 'evm'
  },
  // Mode
  mode: {
    id: 34443,
    name: 'Mode',
    symbol: 'MOD',
    icon: '🎯',
    rpc: 'https://mainnet.mode.network',
    explorer: 'https://explorer.mode.network',
    type: 'evm'
  },
  // Orderly
  orderly: {
    id: 291,
    name: 'Orderly',
    symbol: 'ORD',
    icon: '📋',
    rpc: 'https://rpc.orderly.network',
    explorer: 'https://explorer.orderly.network',
    type: 'evm'
  },
  // Tenet
  tenet: {
    id: 1559106295,
    name: 'Tenet',
    symbol: 'TEN',
    icon: '🔱',
    rpc: 'https://rpc.tenet.org',
    explorer: 'https://tenetscan.org',
    type: 'evm'
  },
  // Redlight
  redlight: {
    id: 302,
    name: 'Redlight',
    symbol: 'REDLC',
    icon: '🔴',
    rpc: 'https://rpc.redlight.xyz',
    explorer: 'https://redlightscan.xyz',
    type: 'evm'
  },
  // Fuse already added
  // Dogecoin
  dogecoin: {
    id: 2000,
    name: 'Dogecoin',
    symbol: 'DOGE',
    icon: '🐕',
    rpc: 'https://dogecoin-mainnet.gateway.pokt.network',
    explorer: 'https://dogechain.info',
    type: 'evm'
  },
  // Litecoin
  litecoin: {
    id: 2,
    name: 'Litecoin',
    symbol: 'LTC',
    icon: '🪙',
    rpc: 'https://litecoin-rpc.gateway.pokt.network',
    explorer: 'https://ltc_insight.luxor.tech',
    type: 'evm'
  },
  // Bitcoin (via EVM)
  bitcoin: {
    id: 0,
    name: 'Bitcoin',
    symbol: 'BTC',
    icon: '₿',
    rpc: 'https://btc.electrum.fun',
    explorer: 'https://blockstream.info',
    type: 'evm'
  },
  // Part of major EVM chains
  // Gnosis
  gnosis: {
    id: 100,
    name: 'Gnosis Chain',
    symbol: 'XDAI',
    icon: '🦉',
    rpc: 'https://rpc.gnosischain.com',
    explorer: 'https://gnosisscan.io',
    type: 'evm'
  },
  // HECO
  heco: {
    id: 128,
    name: 'HECO',
    symbol: 'HT',
    icon: '🟠',
    rpc: 'https://http-mainnet.hecochain.com',
    explorer: 'https://www.hecohunter.com',
    type: 'evm'
  },
  // OKC
  okc: {
    id: 66,
    name: 'OKC',
    symbol: 'OKT',
    icon: '🟢',
    rpc: 'https://exchainrpc.okex.org',
    explorer: 'https://www.oklink.com/okc',
    type: 'evm'
  },
  // Ronin
  ronin: {
    id: 2020,
    name: 'Ronin',
    symbol: 'RON',
    icon: '🪙',
    rpc: 'https://api.roninchain.com/rpc',
    explorer: 'https://app.roninchain.com',
    type: 'evm'
  },
  // Palm
  palm: {
    id: 11297108109,
    name: 'Palm',
    symbol: 'PALM',
    icon: '🌴',
    rpc: 'https://palm-mainnet.infura.io/v3/3a961215650e4b4fb4cd1fa4eb82a0c5',
    explorer: 'https://explorer.palm.io',
    type: 'evm'
  },
  // Syscoin
  syscoin: {
    id: 57,
    name: 'Syscoin',
    symbol: 'SYS',
    icon: '⚙️',
    rpc: 'https://rpc.syscoin.org',
    explorer: 'https://syscoin.io',
    type: 'evm'
  },
  // Moon Alpha
  moon_alpha: {
    id: 507,
    name: 'Moon Alpha',
    symbol: 'MOON',
    icon: '🌙',
    rpc: 'https://rpc.api.moonalpha.io',
    explorer: 'https://explorer.moonalpha.io',
    type: 'evm'
  },
  // PlatON
  platon: {
    id: 210309,
    name: 'PlatON',
    symbol: 'LAT',
    icon: '🔷',
    rpc: 'https://openapi.platon.network/rpc',
    explorer: 'https://scan.platon.network',
    type: 'evm'
  },
  // Conflux
  conflux: {
    id: 1030,
    name: 'Conflux',
    symbol: 'CFX',
    icon: '🔷',
    rpc: 'https://rpc.confluxnetwork.org',
    explorer: 'https://confluxscan.net',
    type: 'evm'
  },
  // Findora
  findora: {
    id: 2152,
    name: 'Findora',
    symbol: 'FRA',
    icon: '🔍',
    rpc: 'https://prod-forge.findora.org',
    explorer: 'https://scan.findora.org',
    type: 'evm'
  },
  // Ankr
  ankr: {
    id: 96,
    name: 'Ankr Chain',
    symbol: 'ANKR',
    icon: '🪶',
    rpc: 'https://rpc.ankr.com/ankr',
    explorer: 'https://ankrscan.io',
    type: 'evm'
  },
  // Klaytn
  klaytn: {
    id: 8217,
    name: 'Klaytn',
    symbol: 'KLAY',
    icon: '🧊',
    rpc: 'https://rpc.klaytn.com',
    explorer: 'https://klaytnscope.com',
    type: 'evm'
  },
  // Ontology
  ontology: {
    id: 58,
    name: 'Ontology',
    symbol: 'ONT',
    icon: '🔗',
    rpc: 'https://dappnode1.ont.io:10339',
    explorer: 'https://explorer.ont.io',
    type: 'evm'
  },
  // IoTeX
  iotex: {
    id: 4689,
    name: 'IoTeX',
    symbol: 'IOTX',
    icon: '📡',
    rpc: 'https://rpc.iotex.io',
    explorer: 'https://iotexscan.io',
    type: 'evm'
  },
  // Bitgert
  bitgert: {
    id: 32520,
    name: 'Bitgert',
    symbol: 'BRISE',
    icon: '💎',
    rpc: 'https://rpc.icecreamswap.com',
    explorer: 'https://scan.bitgert.io',
    type: 'evm'
  },
  // Zilliqa
  zilliqa: {
    id: 1,
    name: 'Zilliqa',
    symbol: 'ZIL',
    icon: '💠',
    rpc: 'https://api.zilliqa.com',
    explorer: 'https://viewblock.io/zilliqa',
    type: 'evm'
  },
  // Elastos
  elastos: {
    id: 20,
    name: 'Elastos',
    symbol: 'ELA',
    icon: '🟢',
    rpc: 'https://api.elastos.io/eth',
    explorer: 'https://elastos.info',
    type: 'evm'
  },
  // EOS EVM
  eos: {
    id: 17777,
    name: 'EOS EVM',
    symbol: 'EOS',
    icon: '🪨',
    rpc: 'https://api.evm.eosnetwork.com',
    explorer: 'https://eoscan.io',
    type: 'evm'
  },
  // WEMIX
  wemix: {
    id: 1111,
    name: 'WEMIX',
    symbol: 'WEMIX',
    icon: '🌐',
    rpc: 'https://api.wemix.com',
    explorer: 'https://wemixscan.com',
    type: 'evm'
  },
  // OEC
  oec: {
    id: 66,
    name: 'OKC',
    symbol: 'OKT',
    icon: '🟢',
    rpc: 'https://exchainrpc.okex.org',
    explorer: 'https://www.oklink.com/okc',
    type: 'evm'
  },
};

interface StoreState {
  connected: boolean;
  address: string | null;
  selectedChain: string;
  tokens: Token[];
  selectedTokenIn: Token | null;
  selectedTokenOut: Token | null;
  connectWallet: () => void;
  disconnectWallet: () => void;
  setSelectedChain: (chain: string) => void;
  setTokens: (tokens: Token[]) => void;
  setSelectedTokenIn: (token: Token | null) => void;
  setSelectedTokenOut: (token: Token | null) => void;
}

export const useStore = create<StoreState>()(
  persist(
    (set) => ({
      connected: false,
      address: null,
      selectedChain: 'ethereum',
      tokens: [],
      selectedTokenIn: null,
      selectedTokenOut: null,
      connectWallet: () => {
        // Mock wallet connection
        set({ connected: true, address: '0x742d35Cc6634C0532925a3b844Bc9e7595f0fEb1' });
      },
      disconnectWallet: () => {
        set({ connected: false, address: null });
      },
      setSelectedChain: (chain) => set({ selectedChain: chain }),
      setTokens: (tokens) => set({ tokens }),
      setSelectedTokenIn: (token) => set({ selectedTokenIn: token }),
      setSelectedTokenOut: (token) => set({ selectedTokenOut: token }),
    }),
    {
      name: 'tigerswap-storage',
      partialize: (state) => ({ 
        selectedChain: state.selectedChain,
        connected: state.connected,
        address: state.address 
      }),
    }
  )
);
