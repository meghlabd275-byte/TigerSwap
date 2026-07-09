import { getDefaultConfig } from '@rainbow-me/rainbowkit';
import { mainnet, arbitrum, optimism, polygon, base, avalanche, bsc } from 'wagmi/chains';

export const config = getDefaultConfig({
  appName: 'TigerWallet',
  projectId: process.env.NEXT_PUBLIC_WALLETCONNECT_PROJECT_ID || 'demo-project-id',
  chains: [
    mainnet,
    arbitrum,
    optimism,
    polygon,
    base,
    avalanche,
    bsc,
  ],
  ssr: true,
});

export const SUPPORTED_CHAINS = [
  {
    id: 1,
    name: 'Ethereum',
    nativeCurrency: { name: 'Ether', symbol: 'ETH', decimals: 18 },
    rpcUrls: { default: { http: ['https://eth.llamarpc.com'] } },
    blockExplorers: { default: { name: 'Etherscan', url: 'https://etherscan.io' } },
  },
  {
    id: 10,
    name: 'Optimism',
    nativeCurrency: { name: 'Ether', symbol: 'ETH', decimals: 18 },
    rpcUrls: { default: { http: ['https://mainnet.optimism.io'] } },
    blockExplorers: { default: { name: 'Optimism Explorer', url: 'https://explorer.optimism.io' } },
  },
  {
    id: 42161,
    name: 'Arbitrum One',
    nativeCurrency: { name: 'Ether', symbol: 'ETH', decimals: 18 },
    rpcUrls: { default: { http: ['https://arb1.arbitrum.io/rpc'] } },
    blockExplorers: { default: { name: 'Arbiscan', url: 'https://arbiscan.io' } },
  },
  {
    id: 137,
    name: 'Polygon',
    nativeCurrency: { name: 'MATIC', symbol: 'MATIC', decimals: 18 },
    rpcUrls: { default: { http: ['https://polygon-rpc.com'] } },
    blockExplorers: { default: { name: 'Polygonscan', url: 'https://polygonscan.com' } },
  },
  {
    id: 8453,
    name: 'Base',
    nativeCurrency: { name: 'Ether', symbol: 'ETH', decimals: 18 },
    rpcUrls: { default: { http: ['https://mainnet.base.org'] } },
    blockExplorers: { default: { name: 'Base Explorer', url: 'https://basescan.org' } },
  },
  {
    id: 43114,
    name: 'Avalanche',
    nativeCurrency: { name: 'Avalanche', symbol: 'AVAX', decimals: 18 },
    rpcUrls: { default: { http: ['https://api.avax.network/ext/bc/C/rpc'] } },
    blockExplorers: { default: { name: 'Snowtrace', url: 'https://snowtrace.io' } },
  },
  {
    id: 56,
    name: 'BNB Smart Chain',
    nativeCurrency: { name: 'BNB', symbol: 'BNB', decimals: 18 },
    rpcUrls: { default: { http: ['https://bsc-dataseed.binance.org'] } },
    blockExplorers: { default: { name: 'BscScan', url: 'https://bscscan.com' } },
  },
  {
    id: 11155111,
    name: 'Sepolia',
    nativeCurrency: { name: 'Sepolia Ether', symbol: 'SEP', decimals: 18 },
    rpcUrls: { default: { http: ['https://rpc.sepolia.org'] } },
    blockExplorers: { default: { name: 'Sepolia Explorer', url: 'https://sepolia.etherscan.io' } },
  },
];

export const ENTRY_POINT_ADDRESS = '0x5FF137D4b0FDCD49DcA30c7CF57E578a026d2789';
export const ACCOUNT_FACTORY_ADDRESS = '0x9406Cc6185a346906296840746125a0E44976454';
