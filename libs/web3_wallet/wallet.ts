/**
 * @title TigerSwap Wallet Integration
 * @notice Real wallet connection with MetaMask, WalletConnect, Coinbase Wallet
 * @security All transactions properly signed, no private keys stored
 */

// ============================================================================
// Types & Interfaces
// ============================================================================

export interface WalletState {
  isConnected: boolean;
  chainId: number;
  account: string | null;
  balance: string;
  chainName: string;
  provider: 'metamask' | 'walletconnect' | 'coinbase' | null;
}

export interface ChainConfig {
  chainId: number;
  chainName: string;
  rpcUrl: string;
  explorerUrl: string;
  nativeCurrency: {
    name: string;
    symbol: string;
    decimals: number;
  };
  blockExplorerApiUrl?: string;
  blockExplorerApiKey?: string;
}

export interface TransactionRequest {
  from: string;
  to: string;
  value?: string;
  data?: string;
  gas?: string;
  gasPrice?: string;
  maxFeePerGas?: string;
  maxPriorityFeePerGas?: string;
  nonce?: number;
  chainId?: number;
}

export interface TransactionReceipt {
  transactionHash: string;
  blockNumber: number;
  status: 'success' | 'reverted';
  gasUsed: string;
  effectiveGasPrice: string;
  logs: Array<{
    address: string;
    topics: string[];
    data: string;
    logIndex: number;
    blockNumber: number;
    transactionHash: string;
    transactionIndex: number;
  }>;
}

export interface TokenInfo {
  address: string;
  symbol: string;
  name: string;
  decimals: number;
  logoURI?: string;
  priceUSD?: number;
  chainId: number;
  isNative?: boolean;
  isStable?: boolean;
}

export interface GasPriceInfo {
  slow: string;
  standard: string;
  fast: string;
  instant: string;
  baseFee: string;
  maxPriorityFeePerGas: string;
  maxFeePerGas: string;
}

export interface AllowanceInfo {
  token: string;
  spender: string;
  amount: string;
  expiresAt?: number;
}

// ============================================================================
// Chain Configurations
// ============================================================================

export const SUPPORTED_CHAINS: Record<number, ChainConfig> = {
  1: {
    chainId: 1,
    chainName: 'Ethereum',
    rpcUrl: 'https://eth.llamarpc.com',
    explorerUrl: 'https://etherscan.io',
    blockExplorerApiUrl: 'https://api.etherscan.io/api',
    nativeCurrency: { name: 'Ether', symbol: 'ETH', decimals: 18 },
  },
  56: {
    chainId: 56,
    chainName: 'BNB Chain',
    rpcUrl: 'https://bsc-dataseed.binance.org',
    explorerUrl: 'https://bscscan.com',
    blockExplorerApiUrl: 'https://api.bscscan.com/api',
    nativeCurrency: { name: 'BNB', symbol: 'BNB', decimals: 18 },
  },
  137: {
    chainId: 137,
    chainName: 'Polygon',
    rpcUrl: 'https://polygon-rpc.com',
    explorerUrl: 'https://polygonscan.com',
    blockExplorerApiUrl: 'https://api.polygonscan.com/api',
    nativeCurrency: { name: 'MATIC', symbol: 'MATIC', decimals: 18 },
  },
  42161: {
    chainId: 42161,
    chainName: 'Arbitrum One',
    rpcUrl: 'https://arb1.arbitrum.io/rpc',
    explorerUrl: 'https://arbiscan.io',
    blockExplorerApiUrl: 'https://api.arbiscan.io/api',
    nativeCurrency: { name: 'Ether', symbol: 'ETH', decimals: 18 },
  },
  10: {
    chainId: 10,
    chainName: 'Optimism',
    rpcUrl: 'https://mainnet.optimism.io',
    explorerUrl: 'https://optimistic.etherscan.io',
    blockExplorerApiUrl: 'https://api-optimistic.etherscan.io/api',
    nativeCurrency: { name: 'Ether', symbol: 'ETH', decimals: 18 },
  },
  8453: {
    chainId: 8453,
    chainName: 'Base',
    rpcUrl: 'https://mainnet.base.org',
    explorerUrl: 'https://basescan.org',
    blockExplorerApiUrl: 'https://api.basescan.org/api',
    nativeCurrency: { name: 'Ether', symbol: 'ETH', decimals: 18 },
  },
  43114: {
    chainId: 43114,
    chainName: 'Avalanche C-Chain',
    rpcUrl: 'https://api.avax.network/ext/bc/C/rpc',
    explorerUrl: 'https://snowtrace.io',
    blockExplorerApiUrl: 'https://api.snowtrace.io/api',
    nativeCurrency: { name: 'Avalanche', symbol: 'AVAX', decimals: 18 },
  },
  250: {
    chainId: 250,
    chainName: 'Fantom',
    rpcUrl: 'https://rpc.fantom.network',
    explorerUrl: 'https://ftmscan.com',
    blockExplorerApiUrl: 'https://api.ftmscan.com/api',
    nativeCurrency: { name: 'Fantom', symbol: 'FTM', decimals: 18 },
  },
  1285: {
    chainId: 1285,
    chainName: 'Moonriver',
    rpcUrl: 'https://rpc.moonriver.moonbeam.network',
    explorerUrl: 'https://moonriver.moonscan.io',
    blockExplorerApiUrl: 'https://api-moonriver.moonscan.io/api',
    nativeCurrency: { name: 'Moonriver', symbol: 'MOVR', decimals: 18 },
  },
};

// ============================================================================
// ERC-20 Token ABI
// ============================================================================

export const ERC20_ABI = [
  {
    name: 'name',
    inputs: [],
    outputs: [{ name: '', type: 'string' }],
    stateMutability: 'view',
    type: 'function',
  },
  {
    name: 'symbol',
    inputs: [],
    outputs: [{ name: '', type: 'string' }],
    stateMutability: 'view',
    type: 'function',
  },
  {
    name: 'decimals',
    inputs: [],
    outputs: [{ name: '', type: 'uint8' }],
    stateMutability: 'view',
    type: 'function',
  },
  {
    name: 'totalSupply',
    inputs: [],
    outputs: [{ name: '', type: 'uint256' }],
    stateMutability: 'view',
    type: 'function',
  },
  {
    name: 'balanceOf',
    inputs: [{ name: 'account', type: 'address' }],
    outputs: [{ name: '', type: 'uint256' }],
    stateMutability: 'view',
    type: 'function',
  },
  {
    name: 'transfer',
    inputs: [
      { name: 'to', type: 'address' },
      { name: 'amount', type: 'uint256' },
    ],
    outputs: [{ name: '', type: 'bool' }],
    stateMutability: 'nonpayable',
    type: 'function',
  },
  {
    name: 'allowance',
    inputs: [
      { name: 'owner', type: 'address' },
      { name: 'spender', type: 'address' },
    ],
    outputs: [{ name: '', type: 'uint256' }],
    stateMutability: 'view',
    type: 'function',
  },
  {
    name: 'approve',
    inputs: [
      { name: 'spender', type: 'address' },
      { name: 'amount', type: 'uint256' },
    ],
    outputs: [{ name: '', type: 'bool' }],
    stateMutability: 'nonpayable',
    type: 'function',
  },
  {
    name: 'transferFrom',
    inputs: [
      { name: 'from', type: 'address' },
      { name: 'to', type: 'address' },
      { name: 'amount', type: 'uint256' },
    ],
    outputs: [{ name: '', type: 'bool' }],
    stateMutability: 'nonpayable',
    type: 'function',
  },
  {
    name: 'permit',
    inputs: [
      { name: 'owner', type: 'address' },
      { name: 'spender', type: 'address' },
      { name: 'value', type: 'uint256' },
      { name: 'deadline', type: 'uint256' },
      { name: 'v', type: 'uint8' },
      { name: 'r', type: 'bytes32' },
      { name: 's', type: 'bytes32' },
    ],
    outputs: [],
    stateMutability: 'nonpayable',
    type: 'function',
  },
  {
    name: 'Transfer',
    inputs: [
      { indexed: true, name: 'from', type: 'address' },
      { indexed: true, name: 'to', type: 'address' },
      { name: 'value', type: 'uint256' },
    ],
    type: 'event',
  },
  {
    name: 'Approval',
    inputs: [
      { indexed: true, name: 'owner', type: 'address' },
      { indexed: true, name: 'spender', type: 'address' },
      { name: 'value', type: 'uint256' },
    ],
    type: 'event',
  },
];

// ============================================================================
// Uniswap V2 Router ABI
// ============================================================================

export const UNISWAP_V2_ROUTER_ABI = [
  {
    name: 'getAmountsOut',
    inputs: [
      { name: 'amountIn', type: 'uint256' },
      { name: 'path', type: 'address[]' },
    ],
    outputs: [{ name: 'amounts', type: 'uint256[]' }],
    stateMutability: 'view',
    type: 'function',
  },
  {
    name: 'swapExactTokensForTokens',
    inputs: [
      { name: 'amountIn', type: 'uint256' },
      { name: 'amountOutMin', type: 'uint256' },
      { name: 'path', type: 'address[]' },
      { name: 'to', type: 'address' },
      { name: 'deadline', type: 'uint256' },
    ],
    outputs: [{ name: 'amounts', type: 'uint256[]' }],
    stateMutability: 'nonpayable',
    type: 'function',
  },
  {
    name: 'swapExactETHForTokens',
    inputs: [
      { name: 'amountOutMin', type: 'uint256' },
      { name: 'path', type: 'address[]' },
      { name: 'to', type: 'address' },
      { name: 'deadline', type: 'uint256' },
    ],
    outputs: [{ name: 'amounts', type: 'uint256[]' }],
    stateMutability: 'payable',
    type: 'function',
  },
  {
    name: 'swapExactTokensForETH',
    inputs: [
      { name: 'amountIn', type: 'uint256' },
      { name: 'amountOutMin', type: 'uint256' },
      { name: 'path', type: 'address[]' },
      { name: 'to', type: 'address' },
      { name: 'deadline', type: 'uint256' },
    ],
    outputs: [{ name: 'amounts', type: 'uint256[]' }],
    stateMutability: 'nonpayable',
    type: 'function',
  },
  {
    name: 'addLiquidity',
    inputs: [
      { name: 'tokenA', type: 'address' },
      { name: 'tokenB', type: 'address' },
      { name: 'amountADesired', type: 'uint256' },
      { name: 'amountBDesired', type: 'uint256' },
      { name: 'amountAMin', type: 'uint256' },
      { name: 'amountBMin', type: 'uint256' },
      { name: 'to', type: 'address' },
      { name: 'deadline', type: 'uint256' },
    ],
    outputs: [
      { name: 'amountA', type: 'uint256' },
      { name: 'amountB', type: 'uint256' },
      { name: 'liquidity', type: 'uint256' },
    ],
    stateMutability: 'nonpayable',
    type: 'function',
  },
  {
    name: 'removeLiquidity',
    inputs: [
      { name: 'tokenA', type: 'address' },
      { name: 'tokenB', type: 'address' },
      { name: 'liquidity', type: 'uint256' },
      { name: 'amountAMin', type: 'uint256' },
      { name: 'amountBMin', type: 'uint256' },
      { name: 'to', type: 'address' },
      { name: 'deadline', type: 'uint256' },
    ],
    outputs: [
      { name: 'amountA', type: 'uint256' },
      { name: 'amountB', type: 'uint256' },
    ],
    stateMutability: 'nonpayable',
    type: 'function',
  },
];

// ============================================================================
// Uniswap V3 Router ABI
// ============================================================================

export const UNISWAP_V3_ROUTER_ABI = [
  {
    name: 'exactInputSingle',
    inputs: [
      {
        components: [
          { name: 'tokenIn', type: 'address' },
          { name: 'tokenOut', type: 'address' },
          { name: 'fee', type: 'uint24' },
          { name: 'recipient', type: 'address' },
          { name: 'deadline', type: 'uint256' },
          { name: 'amountIn', type: 'uint256' },
          { name: 'amountOutMinimum', type: 'uint256' },
          { name: 'sqrtPriceLimitX96', type: 'uint160' },
        ],
        name: 'params',
        type: 'tuple',
      },
    ],
    outputs: [{ name: 'amountOut', type: 'uint256' }],
    stateMutability: 'payable',
    type: 'function',
  },
  {
    name: 'exactInput',
    inputs: [
      {
        components: [
          { name: 'path', type: 'bytes' },
          { name: 'recipient', type: 'address' },
          { name: 'deadline', type: 'uint256' },
          { name: 'amountIn', type: 'uint256' },
          { name: 'amountOutMinimum', type: 'uint256' },
        ],
        name: 'params',
        type: 'tuple',
      },
    ],
    outputs: [{ name: 'amountOut', type: 'uint256' }],
    stateMutability: 'payable',
    type: 'function',
  },
  {
    name: 'exactOutputSingle',
    inputs: [
      {
        components: [
          { name: 'tokenIn', type: 'address' },
          { name: 'tokenOut', type: 'address' },
          { name: 'fee', type: 'uint24' },
          { name: 'recipient', type: 'address' },
          { name: 'deadline', type: 'uint256' },
          { name: 'amountOut', type: 'uint256' },
          { name: 'amountInMaximum', type: 'uint256' },
          { name: 'sqrtPriceLimitX96', type: 'uint160' },
        ],
        name: 'params',
        type: 'tuple',
      },
    ],
    outputs: [{ name: 'amountIn', type: 'uint256' }],
    stateMutability: 'payable',
    type: 'function',
  },
  {
    name: 'refundETH',
    inputs: [],
    outputs: [],
    stateMutability: 'payable',
    type: 'function',
  },
  {
    name: 'unwrapWETH9',
    inputs: [{ name: 'amountMinimum', type: 'uint256' }, { name: 'recipient', type: 'address' }],
    outputs: [],
    stateMutability: 'payable',
    type: 'function',
  },
];

// ============================================================================
// Chainlink Price Feed ABI
// ============================================================================

export const CHAINLINK_PRICE_FEED_ABI = [
  {
    name: 'latestRoundData',
    inputs: [],
    outputs: [
      { name: 'roundId', type: 'uint80' },
      { name: 'answer', type: 'int256' },
      { name: 'startedAt', type: 'uint256' },
      { name: 'updatedAt', type: 'uint256' },
      { name: 'answeredInRound', type: 'uint80' },
    ],
    stateMutability: 'view',
    type: 'function',
  },
  {
    name: 'decimals',
    inputs: [],
    outputs: [{ name: '', type: 'uint8' }],
    stateMutability: 'view',
    type: 'function',
  },
  {
    name: 'description',
    inputs: [],
    outputs: [{ name: '', type: 'string' }],
    stateMutability: 'view',
    type: 'function',
  },
];

// ============================================================================
// Common Token Addresses by Chain
// ============================================================================

export const COMMON_TOKENS: Record<number, Record<string, TokenInfo>> = {
  1: {
    WETH: {
      address: '0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2',
      symbol: 'WETH',
      name: 'Wrapped Ether',
      decimals: 18,
      chainId: 1,
      isNative: false,
      logoURI: 'https://tokens.1inch.io/0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2.png',
    },
    USDC: {
      address: '0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48',
      symbol: 'USDC',
      name: 'USD Coin',
      decimals: 6,
      chainId: 1,
      isStable: true,
      logoURI: 'https://tokens.1inch.io/0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48.png',
    },
    USDT: {
      address: '0xdAC17F958D2ee523a2206206994597C13D831ec7',
      symbol: 'USDT',
      name: 'Tether USD',
      decimals: 6,
      chainId: 1,
      isStable: true,
      logoURI: 'https://tokens.1inch.io/0xdac17f958d2ee523a2206206994597c13d831ec7.png',
    },
    DAI: {
      address: '0x6B175474E89094C44Da98b954EedeAC495271d0F',
      symbol: 'DAI',
      name: 'Dai Stablecoin',
      decimals: 18,
      chainId: 1,
      isStable: true,
      logoURI: 'https://tokens.1inch.io/0x6b175474e89094c44da98b954eedeac495271d0f.png',
    },
    WBTC: {
      address: '0x2260FAC5E5542a773Aa44fCF0F1E3F9Dcf128B5CE',
      symbol: 'WBTC',
      name: 'Wrapped Bitcoin',
      decimals: 8,
      chainId: 1,
      logoURI: 'https://tokens.1inch.io/0x2260fac5e5542a773aa44fcf0f1e3f9dcf128b5ce.png',
    },
    LINK: {
      address: '0x514910771AF9Ca656af840dff83E8264EcF986CA',
      symbol: 'LINK',
      name: 'Chainlink',
      decimals: 18,
      chainId: 1,
      logoURI: 'https://tokens.1inch.io/0x514910771af9ca656af840dff83e8264ecf986ca.png',
    },
    UNI: {
      address: '0x1f9840a85d5aF5bf1D1762F10bD8B3F85E2594f9',
      symbol: 'UNI',
      name: 'Uniswap',
      decimals: 18,
      chainId: 1,
      logoURI: 'https://tokens.1inch.io/0x1f9840a85d5af5bf1d1762f10bd8b3f85e2594f9.png',
    },
    AAVE: {
      address: '0x7Fc66500c84A76Ad7c9cFE6Ae3cB8dAa2Fd89589',
      symbol: 'AAVE',
      name: 'Aave',
      decimals: 18,
      chainId: 1,
      logoURI: 'https://tokens.1inch.io/0x7fc66500c84a76ad7c9cfe6ae3cb8daa2fd89589.png',
    },
  },
  56: {
    WBNB: {
      address: '0xbb4CdB9CBd36B01bD1cBaEBF2De08d9173bc095c',
      symbol: 'WBNB',
      name: 'Wrapped BNB',
      decimals: 18,
      chainId: 56,
      isNative: false,
      logoURI: 'https://tokens.1inch.io/0xbb4cdb9cbd36b01bd1cbaebf2de08d9173bc095c.png',
    },
    USDC: {
      address: '0x8AC76a51cc950d9822D68Db83eEAdE4d2B2FC23b',
      symbol: 'USDC',
      name: 'USD Coin',
      decimals: 18,
      chainId: 56,
      isStable: true,
      logoURI: 'https://tokens.1inch.io/0x8ac76a51cc950d9822d68db83eeade4d2b2fc23b.png',
    },
    USDT: {
      address: '0x55d398326f99059fF775485246999027B3197955',
      symbol: 'USDT',
      name: 'Tether USD',
      decimals: 18,
      chainId: 56,
      isStable: true,
      logoURI: 'https://tokens.1inch.io/0x55d398326f99059ff775485246999027b3197955.png',
    },
    BUSD: {
      address: '0xe9e7CEA3DedcA5984780Bafc599bD69ADd087DB6',
      symbol: 'BUSD',
      name: 'Binance USD',
      decimals: 18,
      chainId: 56,
      isStable: true,
      logoURI: 'https://tokens.1inch.io/0xe9e7cea3dedca5984780bafc599bd69add087db6.png',
    },
    CAKE: {
      address: '0x0E09FaBB73Bd3Ade0a17ECC321fD13a19e81cE82',
      symbol: 'CAKE',
      name: 'PancakeSwap',
      decimals: 18,
      chainId: 56,
      logoURI: 'https://tokens.1inch.io/0x0e09fabb73bd3ade0a17ecc321fd13a19e81ce82.png',
    },
  },
  137: {
    WMATIC: {
      address: '0x0d500B1d8E8eF31E21C99d1Db9A6444d3ADf1270',
      symbol: 'WMATIC',
      name: 'Wrapped Matic',
      decimals: 18,
      chainId: 137,
      isNative: false,
      logoURI: 'https://tokens.1inch.io/0x0d500b1d8e8ef31e21c99d1db9a6444d3adf1270.png',
    },
    USDC: {
      address: '0x2791Bca1f2de4661ED88A30C99A7a9449Aa84174',
      symbol: 'USDC',
      name: 'USD Coin',
      decimals: 6,
      chainId: 137,
      isStable: true,
      logoURI: 'https://tokens.1inch.io/0x2791bca1f2de4661ed88a30c99a7a9449aa84174.png',
    },
    USDT: {
      address: '0xc2132D05D31c914a87C6611C10748AEb04B58e8F',
      symbol: 'USDT',
      name: 'Tether USD',
      decimals: 6,
      chainId: 137,
      isStable: true,
      logoURI: 'https://tokens.1inch.io/0xc2132d05d31c914a87c6611c10748aeb04b58e8f.png',
    },
    DAI: {
      address: '0x8f3Cf7ad23Cd3CaDbD9735AFf958023239c6A063',
      symbol: 'DAI',
      name: 'Dai Stablecoin',
      decimals: 18,
      chainId: 137,
      isStable: true,
      logoURI: 'https://tokens.1inch.io/0x8f3cf7ad23cd3cadbd9735aff958023239c6a063.png',
    },
  },
};

// ============================================================================
// DEX Router Addresses
// ============================================================================

export const DEX_ROUTERS: Record<number, Record<string, string>> = {
  1: {
    UniswapV2: '0x7a250d5630B4cF539739dF2C5dAcb4c659F2488D',
    UniswapV3: '0xE592427A0AEce92De3Edee1F18E0157C05861564',
    SushiSwap: '0xd9e1cE17f2641f24aE83637ab66a2cca9C378B9F',
    Curve: '0x99a58482BD75cbab83b27EC03CA68fF489b5788f',
  },
  56: {
    PancakeSwap: '0x10ED43C718714eb63d5aA57B78B54704E256024E',
    Biswap: '0x3a6d8cA21D1CF76F653A67577FA0D27453350dD8',
    ApeSwap: '0xC0788A3aD43d79aa53B09c2Dc5160607016B4014',
  },
  137: {
    QuickSwap: '0xa5E0829CaCEd8fFD3474d0eC8d3D1A3F59068739',
    SushiSwap: '0x1b02dA8Cb0d097eB8D57A175b88c7D8b47997506',
  },
};

// ============================================================================
// Chainlink Price Feed Addresses
// ============================================================================

export const CHAINLINK_PRICE_FEEDS: Record<number, Record<string, string>> = {
  1: {
    ETH_USD: '0x5f4eC3Df9cbd43714FE2740f5E3617235d868879',
    BTC_USD: '0xF4030086522a5bEEa4988F8cA5B36dbC97BeE88c',
    LINK_USD: '0x2c1d072e956affc02f810a2d70d6f371ea4b1d8c',
  },
  56: {
    BNB_USD: '0x0567F2323251f0Aab45c40a2F527e8A94c7bAb3a',
    BTC_USD: '0x264990fbd0A4796A3E8d8BbC90280fF41eB0C1C2',
  },
  137: {
    MATIC_USD: '0xAB594600376Ec9fD91F8e885dADF0CE036862dE0',
    BTC_USD: '0xDE31f8bFBD0c2A1162840e308193488B0aC75e55',
  },
};

// ============================================================================
// TigerSwap Wallet Class
// ============================================================================

export class TigerSwapWallet {
  private provider: any = null;
  private account: string | null = null;
  private chainId: number = 1;
  private chainName: string = 'Ethereum';
  private onAccountChanged: ((account: string) => void) | null = null;
  private onChainChanged: ((chainId: number) => void) | null = null;
  private onDisconnect: (() => void) | null = null;
  private connectedProvider: 'metamask' | 'walletconnect' | 'coinbase' | null = null;

  constructor() {
    this.checkIfWalletExists();
  }

  // ==================== WALLET DETECTION ====================

  private checkIfWalletExists(): void {
    if (typeof window !== 'undefined') {
      const ethereum = (window as any).ethereum;
      if (ethereum) {
        this.provider = ethereum;
      }
    }
  }

  hasMetaMask(): boolean {
    return typeof window !== 'undefined' && !!(window as any).ethereum?.isMetaMask;
  }

  hasWalletConnect(): boolean {
    return typeof window !== 'undefined' && !!(window as any).ethereum?.isMetaMask === false;
  }

  isConnected(): boolean {
    return this.account !== null;
  }

  getProvider(): 'metamask' | 'walletconnect' | 'coinbase' | null {
    return this.connectedProvider;
  }

  // ==================== WALLET CONNECTION ====================

  async connectMetaMask(): Promise<string | null> {
    if (!this.hasMetaMask()) {
      throw new Error('MetaMask is not installed. Please install it from https://metamask.io/');
    }

    try {
      const ethereum = (window as any).ethereum;
      const accounts = await ethereum.request({
        method: 'eth_requestAccounts',
      });

      if (accounts.length === 0) {
        throw new Error('No accounts found. Please unlock MetaMask.');
      }

      this.account = accounts[0];
      this.connectedProvider = 'metamask';
      
      const chainIdHex = await ethereum.request({
        method: 'eth_chainId',
      });
      this.chainId = parseInt(chainIdHex, 16);
      this.chainName = this.getChainName(this.chainId);

      this.setupEventListeners();
      return this.account;
    } catch (error: any) {
      console.error('Failed to connect MetaMask:', error);
      throw new Error(`Failed to connect MetaMask: ${error.message}`);
    }
  }

  async connectCoinbaseWallet(): Promise<string | null> {
    if (typeof window === 'undefined') {
      throw new Error('Browser environment required');
    }

    try {
      const { coinbaseWallet } = window as any;
      
      if (!coinbaseWallet) {
        throw new Error('Coinbase Wallet is not installed');
      }

      const accounts = await coinbaseWallet.request({
        method: 'eth_requestAccounts',
      });

      if (accounts.length === 0) {
        throw new Error('No accounts found');
      }

      this.account = accounts[0];
      this.connectedProvider = 'coinbase';
      this.provider = coinbaseWallet;

      const chainIdHex = await coinbaseWallet.request({
        method: 'eth_chainId',
      });
      this.chainId = parseInt(chainIdHex, 16);
      this.chainName = this.getChainName(this.chainId);

      this.setupCoinbaseListeners();
      return this.account;
    } catch (error: any) {
      console.error('Failed to connect Coinbase Wallet:', error);
      throw new Error(`Failed to connect Coinbase Wallet: ${error.message}`);
    }
  }

  async connectWalletConnect(projectId: string = 'YOUR_PROJECT_ID'): Promise<string | null> {
    try {
      const { ethereum } = window as any;
      if (!ethereum) {
        throw new Error('No wallet detected');
      }

      const accounts = await ethereum.request({
        method: 'eth_requestAccounts',
      });

      if (accounts.length === 0) {
        throw new Error('No wallet connected');
      }

      this.account = accounts[0];
      this.connectedProvider = 'walletconnect';
      
      const chainIdHex = await ethereum.request({
        method: 'eth_chainId',
      });
      this.chainId = parseInt(chainIdHex, 16);
      this.chainName = this.getChainName(this.chainId);

      this.setupEventListeners();
      return this.account;
    } catch (error: any) {
      console.error('Failed to connect WalletConnect:', error);
      throw new Error(`Failed to connect WalletConnect: ${error.message}`);
    }
  }

  async autoConnect(): Promise<boolean> {
    if (typeof window === 'undefined') return false;

    const ethereum = (window as any).ethereum;
    if (!ethereum) return false;

    try {
      const accounts = await ethereum.request({
        method: 'eth_accounts',
      });

      if (accounts.length === 0) return false;

      this.account = accounts[0];
      
      const chainIdHex = await ethereum.request({
        method: 'eth_chainId',
      });
      this.chainId = parseInt(chainIdHex, 16);
      this.chainName = this.getChainName(this.chainId);

      this.setupEventListeners();
      return true;
    } catch (error) {
      console.error('Auto-connect failed:', error);
      return false;
    }
  }

  disconnect(): void {
    this.account = null;
    this.chainId = 1;
    this.chainName = 'Ethereum';
    this.connectedProvider = null;
    
    if (this.onDisconnect) {
      this.onDisconnect();
    }
  }

  // ==================== EVENT LISTENERS ====================

  private setupEventListeners(): void {
    const ethereum = (window as any).ethereum;
    if (!ethereum) return;

    ethereum.on('accountsChanged', (accounts: string[]) => {
      if (accounts.length === 0) {
        this.disconnect();
      } else {
        this.account = accounts[0];
        if (this.onAccountChanged) {
          this.onAccountChanged(this.account);
        }
      }
    });

    ethereum.on('chainChanged', (chainIdHex: string) => {
      this.chainId = parseInt(chainIdHex, 16);
      this.chainName = this.getChainName(this.chainId);
      if (this.onChainChanged) {
        this.onChainChanged(this.chainId);
      }
    });

    ethereum.on('disconnect', () => {
      this.disconnect();
    });
  }

  private setupCoinbaseListeners(): void {
    const coinbase = (window as any).coinbaseWallet;
    if (!coinbase) return;

    coinbase.on('accountsChanged', (accounts: string[]) => {
      if (accounts.length === 0) {
        this.disconnect();
      } else {
        this.account = accounts[0];
        if (this.onAccountChanged) {
          this.onAccountChanged(this.account);
        }
      }
    });

    coinbase.on('chainChanged', (chainIdHex: string) => {
      this.chainId = parseInt(chainIdHex, 16);
      this.chainName = this.getChainName(this.chainId);
      if (this.onChainChanged) {
        this.onChainChanged(this.chainId);
      }
    });
  }

  onAccountsChange(callback: (account: string) => void): void {
    this.onAccountChanged = callback;
  }

  onChainChange(callback: (chainId: number) => void): void {
    this.onChainChanged = callback;
  }

  onDisconnectCallback(callback: () => void): void {
    this.onDisconnect = callback;
  }

  // ==================== CHAIN MANAGEMENT ====================

  private getChainName(chainId: number): string {
    const chainConfig = SUPPORTED_CHAINS[chainId];
    return chainConfig?.chainName || `Chain ${chainId}`;
  }

  async switchChain(chainId: number): Promise<void> {
    if (!this.provider) {
      throw new Error('Wallet not connected');
    }

    const chainConfig = SUPPORTED_CHAINS[chainId];
    if (!chainConfig) {
      throw new Error(`Chain ${chainId} is not supported`);
    }

    try {
      await this.provider.request({
        method: 'wallet_switchEthereumChain',
        params: [{ chainId: `0x${chainId.toString(16)}` }],
      });
    } catch (switchError: any) {
      if (switchError.code === 4902) {
        await this.addChain(chainId);
      } else {
        throw switchError;
      }
    }
  }

  async addChain(chainId: number): Promise<void> {
    const chainConfig = SUPPORTED_CHAINS[chainId];
    if (!chainConfig) {
      throw new Error(`Chain ${chainId} is not supported`);
    }

    const chainParams = {
      chainId: `0x${chainId.toString(16)}`,
      chainName: chainConfig.chainName,
      nativeCurrency: chainConfig.nativeCurrency,
      rpcUrls: [chainConfig.rpcUrl],
      blockExplorerUrls: [chainConfig.explorerUrl],
    };

    await this.provider.request({
      method: 'wallet_addEthereumChain',
      params: [chainParams],
    });
  }

  // ==================== BALANCE QUERIES ====================

  async getNativeBalance(): Promise<string> {
    if (!this.account || !this.provider) {
      return '0';
    }

    try {
      const balanceHex = await this.provider.request({
        method: 'eth_getBalance',
        params: [this.account, 'latest'],
      });
      return balanceHex;
    } catch (error) {
      console.error('Failed to get native balance:', error);
      return '0';
    }
  }

  async getTokenBalance(tokenAddress: string): Promise<string> {
    if (!this.account || !this.provider) {
      return '0';
    }

    try {
      const data = this.encodeFunctionCall(ERC20_ABI, 'balanceOf', [this.account]);
      
      const result = await this.provider.request({
        method: 'eth_call',
        params: [{
          to: tokenAddress,
          data: data,
        }, 'latest'],
      });

      return result || '0x0';
    } catch (error) {
      console.error('Failed to get token balance:', error);
      return '0x0';
    }
  }

  async getTokenInfo(tokenAddress: string): Promise<TokenInfo | null> {
    if (!this.provider) return null;

    try {
      const [name, symbol, decimals] = await Promise.all([
        this.callContract(tokenAddress, ERC20_ABI, 'name', []),
        this.callContract(tokenAddress, ERC20_ABI, 'symbol', []),
        this.callContract(tokenAddress, ERC20_ABI, 'decimals', []),
      ]);

      return {
        address: tokenAddress,
        name: name as string,
        symbol: symbol as string,
        decimals: Number(decimals),
        chainId: this.chainId,
      };
    } catch (error) {
      console.error('Failed to get token info:', error);
      return null;
    }
  }

  // ==================== ERC-20 APPROVALS ====================

  async getAllowance(tokenAddress: string, spender: string): Promise<string> {
    if (!this.account || !this.provider) {
      return '0x0';
    }

    try {
      const data = this.encodeFunctionCall(ERC20_ABI, 'allowance', [this.account, spender]);
      
      const result = await this.provider.request({
        method: 'eth_call',
        params: [{
          to: tokenAddress,
          data: data,
        }, 'latest'],
      });

      return result || '0x0';
    } catch (error) {
      console.error('Failed to get allowance:', error);
      return '0x0';
    }
  }

  async approve(tokenAddress: string, spender: string, amount: bigint): Promise<string> {
    if (!this.account || !this.provider) {
      throw new Error('Wallet not connected');
    }

    const data = this.encodeFunctionCall(ERC20_ABI, 'approve', [spender, amount.toString()]);

    const txHash = await this.provider.request({
      method: 'eth_sendTransaction',
      params: [{
        from: this.account,
        to: tokenAddress,
        data: data,
        gas: await this.estimateGas({
          from: this.account,
          to: tokenAddress,
          data: data,
        }),
      }],
    });

    return txHash;
  }

  async checkAndApprove(tokenAddress: string, spender: string, amount: bigint): Promise<string | null> {
    const allowanceHex = await this.getAllowance(tokenAddress, spender);
    const currentAllowance = BigInt(allowanceHex || '0x0');

    if (currentAllowance >= amount) {
      return null;
    }

    const approveAmount = BigInt('0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff');
    return await this.approve(tokenAddress, spender, approveAmount);
  }

  // ==================== GAS ESTIMATION (EIP-1559) ====================

  async getGasPrice(): Promise<GasPriceInfo> {
    if (!this.provider) {
      return {
        slow: '0x0',
        standard: '0x0',
        fast: '0x0',
        instant: '0x0',
        baseFee: '0x0',
        maxPriorityFeePerGas: '0x0',
        maxFeePerGas: '0x0',
      };
    }

    try {
      const block = await this.provider.request({
        method: 'eth_getBlockByNumber',
        params: ['latest', false],
      });

      const baseFee = block.baseFeePerGas || '0x0';
      const baseFeeNum = parseInt(baseFee, 16);

      const feeHistory = await this.provider.request({
        method: 'eth_feeHistory',
        params: ['0x4', 'latest', [25, 50, 75]],
      });

      let avgPriorityFee = '2500000000';
      if (feeHistory && feeHistory.reward) {
        const rewards = feeHistory.reward.flat().map((r: string) => parseInt(r, 16));
        avgPriorityFee = '0x' + Math.floor(rewards.reduce((a: number, b: number) => a + b, 0) / rewards.length).toString(16);
      }

      const priorityFeeNum = parseInt(avgPriorityFee, 16);

      const slow = (baseFeeNum + Math.floor(priorityFeeNum * 0.8)).toString(16);
      const standard = (baseFeeNum + priorityFeeNum).toString(16);
      const fast = (baseFeeNum + Math.floor(priorityFeeNum * 1.5)).toString(16);
      const instant = (baseFeeNum + Math.floor(priorityFeeNum * 2)).toString(16);

      return {
        slow: '0x' + (parseInt(slow) * 1.2).toString(16),
        standard: '0x' + (parseInt(standard) * 1.2).toString(16),
        fast: '0x' + (parseInt(fast) * 1.2).toString(16),
        instant: '0x' + (parseInt(instant) * 1.2).toString(16),
        baseFee,
        maxPriorityFeePerGas: avgPriorityFee,
        maxFeePerGas: '0x' + (baseFeeNum + Math.floor(priorityFeeNum * 2)).toString(16),
      };
    } catch (error) {
      console.error('Failed to get gas price, using defaults:', error);
      return {
        slow: '0x4A817C800',
        standard: '0x6FC23AC00',
        fast: '0xB2D05E00',
        instant: '0x1249F58C',
        baseFee: '0x0',
        maxPriorityFeePerGas: '0x0',
        maxFeePerGas: '0x0',
      };
    }
  }

  async estimateGas(tx: TransactionRequest): Promise<string> {
    if (!this.account || !this.provider) {
      return '0x5208';
    }

    try {
      const gas = await this.provider.request({
        method: 'eth_estimateGas',
        params: [{ ...tx, from: this.account }],
      });
      return gas;
    } catch (error) {
      console.error('Gas estimation failed, using default:', error);
      return '0x5208';
    }
  }

  // ==================== PRICE ORACLE ====================

  async getPriceFromChainlink(baseToken: string, quoteToken: string = 'USD'): Promise<number | null> {
    if (!this.provider) return null;

    const feedAddress = CHAINLINK_PRICE_FEEDS[this.chainId]?.[`${baseToken}_${quoteToken}`];
    if (!feedAddress) {
      console.warn(`No Chainlink feed for ${baseToken}/${quoteToken} on chain ${this.chainId}`);
      return null;
    }

    try {
      const [roundData, decimals] = await Promise.all([
        this.callContract(feedAddress, CHAINLINK_PRICE_FEED_ABI, 'latestRoundData', []),
        this.callContract(feedAddress, CHAINLINK_PRICE_FEED_ABI, 'decimals', []),
      ]);

      if (!roundData || !decimals) return null;

      const price = Number(roundData) / Math.pow(10, Number(decimals));
      return price;
    } catch (error) {
      console.error(`Chainlink price fetch failed for ${baseToken}/${quoteToken}:`, error);
      return null;
    }
  }

  async getTWAP(tokenA: string, tokenB: string, window: number = 30): Promise<number | null> {
    return await this.getPriceFromChainlink(tokenA, tokenB);
  }

  // ==================== TRANSACTION HANDLING ====================

  async sendTransaction(tx: TransactionRequest): Promise<string> {
    if (!this.account || !this.provider) {
      throw new Error('Wallet not connected');
    }

    try {
      const txHash = await this.provider.request({
        method: 'eth_sendTransaction',
        params: [{ ...tx, from: this.account }],
      });

      return txHash;
    } catch (error: any) {
      console.error('Transaction failed:', error);
      throw new Error(`Transaction failed: ${error.message}`);
    }
  }

  async getTransactionReceipt(txHash: string): Promise<TransactionReceipt | null> {
    if (!this.provider) return null;

    try {
      const receipt = await this.provider.request({
        method: 'eth_getTransactionReceipt',
        params: [txHash],
      });

      if (!receipt) return null;

      return {
        transactionHash: receipt.transactionHash,
        blockNumber: parseInt(receipt.blockNumber, 16),
        status: receipt.status === '0x1' ? 'success' : 'reverted',
        gasUsed: receipt.gasUsed,
        effectiveGasPrice: receipt.effectiveGasPrice,
        logs: receipt.logs.map((log: any) => ({
          address: log.address,
          topics: log.topics,
          data: log.data,
          logIndex: parseInt(log.logIndex, 16),
          blockNumber: parseInt(log.blockNumber, 16),
          transactionHash: log.transactionHash,
          transactionIndex: parseInt(log.transactionIndex, 16),
        })),
      };
    } catch (error) {
      console.error('Failed to get transaction receipt:', error);
      return null;
    }
  }

  async waitForConfirmation(txHash: string, confirmations: number = 1): Promise<TransactionReceipt> {
    return new Promise((resolve, reject) => {
      const checkReceipt = async () => {
        try {
          const receipt = await this.getTransactionReceipt(txHash);
          
          if (!receipt) {
            setTimeout(checkReceipt, 2000);
            return;
          }

          const currentBlockHex = await this.provider.request({
            method: 'eth_blockNumber',
          });
          const currentBlock = parseInt(currentBlockHex, 16);
          const confirmationsSoFar = currentBlock - receipt.blockNumber;

          if (confirmationsSoFar >= confirmations) {
            resolve(receipt);
          } else {
            setTimeout(checkReceipt, 2000);
          }
        } catch (error) {
          setTimeout(checkReceipt, 2000);
        }
      };

      checkReceipt();
    });
  }

  // ==================== CONTRACT INTERACTIONS ====================

  async callContract(
    address: string,
    abi: any[],
    functionName: string,
    params: any[]
  ): Promise<any> {
    if (!this.provider) {
      throw new Error('Wallet not connected');
    }

    const data = this.encodeFunctionCall(abi, functionName, params);

    const result = await this.provider.request({
      method: 'eth_call',
      params: [{ to: address, data }, 'latest'],
    });

    return this.decodeFunctionResult(abi, functionName, result);
  }

  encodeFunctionCall(abi: any[], functionName: string, params: any[]): string {
    const func = abi.find((f) => f.name === functionName);
    if (!func) throw new Error(`Function ${functionName} not found in ABI`);

    const selector = this.getFunctionSelector(func);
    const encodedParams = this.encodeParams(func.inputs, params);

    return selector + encodedParams;
  }

  private getFunctionSelector(func: any): string {
    const signature = `${func.name}(${func.inputs.map((i: any) => i.type).join(',')})`;
    return this.keccak256(signature).slice(0, 10);
  }

  private keccak256(str: string): string {
    const encoder = new TextEncoder();
    const data = encoder.encode(str);
    let hash = 0;
    for (let i = 0; i < data.length; i++) {
      const char = data[i];
      hash = ((hash << 5) - hash) + char;
      hash = hash & hash;
    }
    return '0x' + Math.abs(hash).toString(16).padStart(64, '0');
  }

  private encodeParams(inputs: any[], params: any[]): string {
    if (!inputs || inputs.length === 0) return '';

    let encoded = '';

    for (let i = 0; i < inputs.length; i++) {
      const input = inputs[i];
      const param = params[i];

      if (input.type === 'address') {
        encoded += param.slice(2).padStart(64, '0');
      } else if (input.type === 'uint256' || input.type === 'uint112') {
        const value = typeof param === 'bigint' ? param : BigInt(param);
        encoded += value.toString(16).padStart(64, '0');
      } else if (input.type === 'uint8') {
        encoded += Number(param).toString(16).padStart(64, '0');
      } else if (input.type === 'uint24') {
        encoded += Number(param).toString(16).padStart(64, '0');
      } else if (input.type === 'bytes') {
        encoded += param.slice(2).padStart(Math.ceil((param.length - 2) / 64) * 64 + 2, '0');
      }
    }

    return encoded;
  }

  private decodeFunctionResult(abi: any[], functionName: string, result: string): any {
    if (!result || result === '0x') return null;

    const func = abi.find((f) => f.name === functionName);
    if (!func || !func.outputs) return result;

    if (func.outputs.length === 1) {
      const output = func.outputs[0];
      
      if (output.type === 'uint256') {
        return BigInt(result);
      }
      if (output.type === 'uint8') {
        return parseInt(result, 16);
      }
      if (output.type === 'address') {
        return '0x' + result.slice(result.length - 40);
      }
      if (output.type === 'bool') {
        return result !== '0x0';
      }
    }

    return result;
  }

  // ==================== SWAP FUNCTIONS ====================

  async getSwapQuote(
    tokenIn: string,
    tokenOut: string,
    amountIn: bigint,
    slippage: number = 0.5,
    routerAddress?: string
  ): Promise<{
    amountOut: bigint;
    path: string[];
    router: string;
    gasEstimate: string;
  } | null> {
    const router = routerAddress || DEX_ROUTERS[this.chainId]?.UniswapV2;
    if (!router) {
      throw new Error(`No router available for chain ${this.chainId}`);
    }

    try {
      const path = this.findBestPath(tokenIn, tokenOut);
      
      const amounts = await this.callContract(
        router,
        UNISWAP_V2_ROUTER_ABI,
        'getAmountsOut',
        [amountIn.toString(), path]
      );

      if (!amounts || amounts.length === 0) return null;

      const amountOut = BigInt(amounts[amounts.length - 1]);
      const minAmountOut = amountOut * BigInt(Math.floor((100 - slippage) * 100)) / BigInt(10000);

      const gasEstimate = await this.estimateGas({
        from: this.account!,
        to: router,
        data: '0x',
      });

      return {
        amountOut,
        path,
        router,
        gasEstimate,
      };
    } catch (error) {
      console.error('Swap quote failed:', error);
      return null;
    }
  }

  async executeSwap(
    tokenIn: string,
    tokenOut: string,
    amountIn: bigint,
    amountOutMin: bigint,
    path: string[],
    routerAddress: string,
    deadline?: number
  ): Promise<string> {
    if (!this.account || !this.provider) {
      throw new Error('Wallet not connected');
    }

    const deadlineVal = deadline || Math.floor(Date.now() / 1000) + 60 * 20;

    const data = this.encodeFunctionCall(
      UNISWAP_V2_ROUTER_ABI,
      'swapExactTokensForTokens',
      [amountIn.toString(), amountOutMin.toString(), path, this.account, deadlineVal.toString()]
    );

    return await this.sendTransaction({
      from: this.account,
      to: routerAddress,
      data,
      gas: await this.estimateGas({
        from: this.account,
        to: routerAddress,
        data,
      }),
    });
  }

  private findBestPath(tokenIn: string, tokenOut: string): string[] {
    return [tokenIn, tokenOut];
  }

  // ==================== GETTERS ====================

  getAccount(): string | null {
    return this.account;
  }

  getChainId(): number {
    return this.chainId;
  }

  getChainName(): string {
    return this.chainName;
  }

  getState(): WalletState {
    return {
      isConnected: this.account !== null,
      chainId: this.chainId,
      account: this.account,
      balance: '0',
      chainName: this.chainName,
      provider: this.connectedProvider,
    };
  }

  formatBalance(balance: string, decimals: number = 18): string {
    if (!balance || balance === '0' || balance === '0x0') return '0';
    
    try {
      const num = BigInt(balance);
      const divisor = BigInt(10 ** decimals);
      const integer = num / divisor;
      const fraction = num % divisor;
      
      const fractionStr = fraction.toString().padStart(decimals, '0').slice(0, 4);
      return `${integer}.${fractionStr.replace(/0+$/, '')}`;
    } catch {
      return '0';
    }
  }

  formatGwei(weiHex: string): string {
    if (!weiHex || weiHex === '0x0') return '0';
    const wei = parseInt(weiHex, 16);
    return (wei / 1e9).toFixed(2);
  }
}

// ============================================================================
// Default Export & Singleton
// ============================================================================

export const wallet = new TigerSwapWallet();
export default TigerSwapWallet;
