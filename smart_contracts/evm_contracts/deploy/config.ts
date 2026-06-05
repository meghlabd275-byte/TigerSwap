/**
 * TigerSwap Deployment Configuration
 * Environment variables and network configurations
 */

// Network RPC URLs
export const RPC_URLS = {
  mainnet: process.env.MAINNET_RPC_URL || "https://eth.llamarpc.com",
  sepolia: process.env.SEPOLIA_RPC_URL || "https://rpc.sepolia.org",
  arbitrum: process.env.ARBITRUM_RPC_URL || "https://arb1.arbitrum.io/rpc",
  arbitrumSepolia: process.env.ARB_SEPOLIA_RPC_URL || "https://sepolia-rollup.arbitrum.io/rpc",
  polygon: process.env.POLYGON_RPC_URL || "https://polygon-rpc.com",
  bsc: process.env.BSC_RPC_URL || "https://bsc-dataseed.binance.org",
  optimism: process.env.OPTIMISM_RPC_URL || "https://mainnet.optimism.io",
  base: process.env.BASE_RPC_URL || "https://mainnet.base.org",
  baseSepolia: process.env.BASE_SEPOLIA_RPC_URL || "https://sepolia.base.org",
  avalanche: process.env.AVALANCHE_RPC_URL || "https://api.avax.network/ext/bc/C/rpc",
  fantom: process.env.FANTOM_RPC_URL || "https://rpc.fantom.network",
};

// Chain IDs
export const CHAIN_IDS = {
  mainnet: 1,
  sepolia: 11155111,
  arbitrum: 42161,
  arbitrumSepolia: 421614,
  polygon: 137,
  bsc: 56,
  optimism: 10,
  base: 8453,
  baseSepolia: 84532,
  avalanche: 43114,
  fantom: 250,
};

// Block Explorers
export const BLOCK_EXPLORERS = {
  mainnet: "https://etherscan.io",
  sepolia: "https://sepolia.etherscan.io",
  arbitrum: "https://arbiscan.io",
  arbitrumSepolia: "https://sepolia.arbiscan.io",
  polygon: "https://polygonscan.com",
  bsc: "https://bscscan.com",
  optimism: "https://optimistic.etherscan.io",
  base: "https://basescan.org",
  baseSepolia: "https://sepolia.basescan.org",
  avalanche: "https://snowtrace.io",
  fantom: "https://ftmscan.com",
};

// Contract Addresses by Network
export const CONTRACT_ADDRESSES: Record<number, {
  factory: string;
  router: string;
  WETH: string;
  tigerToken: string;
  masterChef: string;
  staking: string;
  governance: string;
  bridge: string;
  vault: string;
  treasury: string;
}> = {
  [CHAIN_IDS.mainnet]: {
    factory: "",
    router: "",
    WETH: "0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2",
    tigerToken: "",
    masterChef: "",
    staking: "",
    governance: "",
    bridge: "",
    vault: "",
    treasury: "",
  },
  [CHAIN_IDS.sepolia]: {
    factory: "",
    router: "",
    WETH: "0xfFf9976782d46CC05630D1f6eB18b0f4982AcB37",
    tigerToken: "",
    masterChef: "",
    staking: "",
    governance: "",
    bridge: "",
    vault: "",
    treasury: "",
  },
  [CHAIN_IDS.arbitrum]: {
    factory: "",
    router: "",
    WETH: "0x82aF49447D8a07e3bd95BD0d56f35241523fBab1",
    tigerToken: "",
    masterChef: "",
    staking: "",
    governance: "",
    bridge: "",
    vault: "",
    treasury: "",
  },
  [CHAIN_IDS.polygon]: {
    factory: "",
    router: "",
    WETH: "0x0d500B1d8E8eF31E21C99d1Db9A6444d3ADf1270",
    tigerToken: "",
    masterChef: "",
    staking: "",
    governance: "",
    bridge: "",
    vault: "",
    treasury: "",
  },
  [CHAIN_IDS.bsc]: {
    factory: "",
    router: "",
    WETH: "0xbb4CdB9CBd36B01bD1cBaEBF2De08d9173bc095c",
    tigerToken: "",
    masterChef: "",
    staking: "",
    governance: "",
    bridge: "",
    vault: "",
    treasury: "",
  },
  [CHAIN_IDS.base]: {
    factory: "",
    router: "",
    WETH: "0x4200000000000000000000000000000000000006",
    tigerToken: "",
    masterChef: "",
    staking: "",
    governance: "",
    bridge: "",
    vault: "",
    treasury: "",
  },
};

// Fee Configuration
export const FEE_CONFIG = {
  protocolFeeBps: 25,
  lpRewardsBps: 5,
  defaultSwapFee: 300,
  stableSwapFee: 100,
  highVolatilityFee: 1000,
};

// MasterChef Configuration
export const MASTERCHEF_CONFIG = {
  rewardPerSecond: "1000000000000000000",
  startTime: 0,
  devPercent: 1000,
  treasuryPercent: 1000,
  investorPercent: 500,
  liquidityPercent: 3000,
};

// Governance Configuration  
export const GOVERNANCE_CONFIG = {
  votingPeriod: 3 * 24 * 60 * 60,
  proposalThreshold: "1000000000000000000000",
  quorumThreshold: 400,
  proposalTimelockDelay: 2 * 24 * 60 * 60,
};

// Staking Configuration
export const STAKING_CONFIG = {
  minStakeAmount: "100000000000000000000",
  maxStakeAmount: "1000000000000000000000000",
  minStakeDuration: 1 * 24 * 60 * 60,
  earlyWithdrawPenalty: 500,
  lockPeriods: [
    { days: 0, multiplier: 1000 },
    { days: 7, multiplier: 1500 },
    { days: 30, multiplier: 2000 },
    { days: 90, multiplier: 3000 },
    { days: 180, multiplier: 4000 },
    { days: 365, multiplier: 5000 },
  ],
};

// Token Configuration
export const TOKENS = {
  mainnet: {
    WETH: "0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2",
    USDC: "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48",
    USDT: "0xdAC17F958D2ee523a2206206994597C13D831ec7",
    DAI: "0x6B175474E89094C44Da98b954EedeAC495271d0F",
    WBTC: "0x2260FAC5E5542a773Aa44fBCfeDf7C193bc2C599",
    LINK: "0x514910771AF9Ca656af840dff83E8264EcF986CA",
    UNI: "0x1f9840a85d5aF5bf1D1762F925BDADdC4201F984",
    AAVE: "0x7Fc66500c84A76Ad7e9c93437bFc5Ac33E2DDaE9",
  },
  arbitrum: {
    WETH: "0x82aF49447D8a07e3bd95BD0d56f35241523fBab1",
    USDC: "0xaf88d065e77c8cC2239327C5EDb3A432268e5831",
    USDT: "0xFd086bC7CD5C481DCC9C85ebE478A1C0b69FCbb9",
    ARB: "0x912CE59144191C1204E64559FE8253a0e49E6548",
  },
  polygon: {
    WETH: "0x0d500B1d8E8eF31E21C99d1Db9A6444d3ADf1270",
    USDC: "0x2791Bca1f2de4661ED88A30C99A7a9449Aa84174",
    USDT: "0xc2132D05D31c914a87C6611C10748AEb04B58e8F",
    MATIC: "0x0000000000000000000000000000000000001010",
  },
  bsc: {
    WBNB: "0xbb4CdB9CBd36B01bD1cBaEBF2De08d9173bc095c",
    BUSD: "0xe9e7CEA3DedcA5984780Bafc599bD69ADd087D56",
    USDT: "0x55d398326f99059fF775485246999027B3197955",
    CAKE: "0x0E09FaBB73Bd3Ade0a17ECC321fD13a19e81cE82",
  },
  base: {
    WETH: "0x4200000000000000000000000000000000000006",
    USDC: "0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913",
  },
};

export default {
  RPC_URLS,
  CHAIN_IDS,
  BLOCK_EXPLORERS,
  CONTRACT_ADDRESSES,
  FEE_CONFIG,
  MASTERCHEF_CONFIG,
  GOVERNANCE_CONFIG,
  STAKING_CONFIG,
  TOKENS,
};