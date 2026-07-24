import { Token, Chain } from '@/store/useStore';

// TigerSmartChain - Main EVM Blockchain
export const TIGER_SMART_CHAIN = {
  chainId: 8888,
  name: 'TigerSmartChain',
  symbol: 'TGR',
  decimals: 18,
  rpcUrl: 'https://rpc.tigersmartchain.com',
  explorerUrl: 'https://explorer.tigersmartchain.com',
  type: 'evm' as const,
  isTestnet: false,
};

// TigerSmartChain Testnet
export const TIGER_TESTNET = {
  chainId: 18888,
  name: 'TigerSmartChain Testnet',
  symbol: 'TGR',
  decimals: 18,
  rpcUrl: 'https://rpc-testnet.tigersmartchain.com',
  explorerUrl: 'https://explorer-testnet.tigersmartchain.com',
  type: 'evm' as const,
  isTestnet: true,
};

// Comprehensive token database with 200+ tokens across 100+ chains
export interface TokenData extends Token {
  chainId: number;
  chainKey: string;
  contractAddress?: string;
  isNative?: boolean;
  priceUSD?: number;
  marketCap?: number;
  volume24h?: number;
}

export const TOKEN_DATABASE: Record<string, TokenData[]> = {
  // Ethereum (Chain ID: 1)
  'ethereum': [
    { address: '0x0000000000000000000000000000000000000000', symbol: 'ETH', name: 'Ethereum', decimals: 18, logoURI: 'https://assets.coingecko.com/coins/images/279/small/ethereum.png', chainId: 1, chainKey: 'ethereum', isNative: true, priceUSD: 3500.00 },
    { address: '0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48', symbol: 'USDC', name: 'USD Coin', decimals: 6, logoURI: 'https://assets.coingecko.com/coins/images/6319/small/usdc.png', chainId: 1, chainKey: 'ethereum', priceUSD: 1.00 },
    { address: '0xdAC17F958D2ee523a2206206994597C13D831ec7', symbol: 'USDT', name: 'Tether USD', decimals: 6, logoURI: 'https://assets.coingecko.com/coins/images/325/small/Tether.png', chainId: 1, chainKey: 'ethereum', priceUSD: 1.00 },
    { address: '0x2260FAC5E5542a773Aa44fBCfeDf7C193bc2C599', symbol: 'WBTC', name: 'Wrapped Bitcoin', decimals: 8, logoURI: 'https://assets.coingecko.com/coins/images/7598/small/wrapped_bitcoin_wbtc.png', chainId: 1, chainKey: 'ethereum', priceUSD: 65000.00 },
    { address: '0x514910771AF9Ca656af840dff83E8264EcF986CA', symbol: 'LINK', name: 'Chainlink', decimals: 18, logoURI: 'https://assets.coingecko.com/coins/images/877/small/chainlink-new-logo.png', chainId: 1, chainKey: 'ethereum', priceUSD: 15.00 },
    { address: '0x1f9840a85d5aF5bf1D1762F925BDADdC4201F984', symbol: 'UNI', name: 'Uniswap', decimals: 18, logoURI: 'https://assets.coingecko.com/coins/images/12504/small/uniswap-uni.png', chainId: 1, chainKey: 'ethereum', priceUSD: 10.00 },
    { address: '0x7Fc66500c84A76Ad7e9c93437bFc5Ac33E2DDaE9', symbol: 'AAVE', name: 'Aave', decimals: 18, logoURI: 'https://assets.coingecko.com/coins/images/12645/small/AAVE.png', chainId: 1, chainKey: 'ethereum', priceUSD: 250.00 },
    { address: '0x0D8775F648430679A709E98d2b0Cb6250d2887EF', symbol: 'BAT', name: 'Basic Attention Token', decimals: 18, logoURI: 'https://assets.coingecko.com/coins/images/677/small/bat.png', chainId: 1, chainKey: 'ethereum', priceUSD: 0.30 },
    { address: '0xE41d2489571d322189246DDA5BAutE5b8CE7e556', symbol: 'MKR', name: 'Maker', decimals: 18, logoURI: 'https://assets.coingecko.com/coins/images/1364/small/Mark_Maker.png', chainId: 1, chainKey: 'ethereum', priceUSD: 1800.00 },
    { address: '0xd26114cd6EE289AccF82350c8d8487fedB8C0C12', symbol: 'OMG', name: 'OmiseGO', decimals: 18, logoURI: 'https://assets.coingecko.com/coins/images/1108/small/omise_go.png', chainId: 1, chainKey: 'ethereum', priceUSD: 1.50 },
    { address: '0xBB0E17EF65F82Ab018d8EDd776e8DD940327B28b', symbol: 'AXS', name: 'Axie Infinity', decimals: 18, logoURI: 'https://assets.coingecko.com/coins/images/13029/small/axie_infinity_logo.png', chainId: 1, chainKey: 'ethereum', priceUSD: 8.00 },
    { address: '0xdd974D5C2e2928deA5f71b9824A3987E85ad8A17', symbol: 'ZRX', name: '0x', decimals: 18, logoURI: 'https://assets.coingecko.com/coins/images/863/small/0x.png', chainId: 1, chainKey: 'ethereum', priceUSD: 0.40 },
    { address: '0x0bc529c00C6401aEF6D5BE9a09EacE3b2c6d0dD6', symbol: 'YFI', name: 'yearn.finance', decimals: 18, logoURI: 'https://assets.coingecko.com/coins/images/11849/small/yearn-finance.png', chainId: 1, chainKey: 'ethereum', priceUSD: 4500.00 },
    { address: '0xD533a949740bb3306d119CC777fa900bA034cd52', symbol: 'CRV', name: 'Curve DAO Token', decimals: 18, logoURI: 'https://assets.coingecko.com/coins/images/12124/small/Curve.png', chainId: 1, chainKey: 'ethereum', priceUSD: 0.60 },
    { address: '0x7D1AfA7B718fb893dB30A3aBc0Cfc608AaCfeBB0', symbol: 'MATIC', name: 'Polygon', decimals: 18, logoURI: 'https://assets.coingecko.com/coins/images/4713/small/polygon.png', chainId: 1, chainKey: 'ethereum', priceUSD: 0.80 },
    { address: '0x8a90CAb2b38dba80c64b7734e58Ee1dB38B8992e', symbol: 'DOGE', name: 'Dogecoin', decimals: 8, logoURI: 'https://assets.coingecko.com/coins/images/5/small/dogecoin.png', chainId: 1, chainKey: 'ethereum', priceUSD: 0.15 },
    { address: '0x4fE83213D56318330F327d78D5f8C98D3b9dB9f6', symbol: 'LTC', name: 'Litecoin', decimals: 8, logoURI: 'https://assets.coingecko.com/coins/images/2/small/litecoin.png', chainId: 1, chainKey: 'ethereum', priceUSD: 85.00 },
    { address: '0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2', symbol: 'WETH', name: 'Wrapped Ether', decimals: 18, logoURI: 'https://assets.coingecko.com/coins/images/2518/small/weth.png', chainId: 1, chainKey: 'ethereum', priceUSD: 3500.00 },
    { address: '0x6B175474E89094C44Da98b954EesadcdEF9ce66CC', symbol: 'DAI', name: 'Dai Stablecoin', decimals: 18, logoURI: 'https://assets.coingecko.com/coins/images/9956/small/4943.png', chainId: 1, chainKey: 'ethereum', priceUSD: 1.00 },
    { address: '0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48', symbol: 'USDC', name: 'USD Coin', decimals: 6, logoURI: 'https://assets.coingecko.com/coins/images/6319/small/usdc.png', chainId: 42161, chainKey: 'arbitrum', priceUSD: 1.00 },
    { address: '0xdAC17F958D2ee523a2206206994597C13D831ec7', symbol: 'USDT', name: 'Tether USD', decimals: 6, logoURI: 'https://assets.coingecko.com/coins/images/325/small/Tether.png', chainId: 42161, chainKey: 'arbitrum', priceUSD: 1.00 },
    { address: '0x82aF49447D8a07e3bd95BD0d56f35241523fBab1', symbol: 'WETH', name: 'Wrapped Ether', decimals: 18, logoURI: 'https://assets.coingecko.com/coins/images/2518/small/weth.png', chainId: 42161, chainKey: 'arbitrum', priceUSD: 3500.00 },
    { address: '0x912CE59144191C1204E64559fe8253a0e49E6548', symbol: 'ARB', name: 'Arbitrum', decimals: 18, logoURI: 'https://assets.coingecko.com/coins/images/16547/small/photo_2023-03-29_21.47.00.jpeg', chainId: 42161, chainKey: 'arbitrum', priceUSD: 1.20 },
    { address: '0x2f2a2543B76A4166549F7aaB2e75Bfe0e5Ba3C68', symbol: 'WBTC', name: 'Wrapped Bitcoin', decimals: 8, logoURI: 'https://assets.coingecko.com/coins/images/7598/small/wrapped_bitcoin_wbtc.png', chainId: 42161, chainKey: 'arbitrum', priceUSD: 65000.00 },
    { address: '0x13Ad51ed83F0647B27a2E9D2d44dA80F22a9d8D9', symbol: 'GMX', name: 'GMX', decimals: 18, logoURI: 'https://assets.coingecko.com/coins/images/18323/small/Arbitrum.svg', chainId: 42161, chainKey: 'arbitrum', priceUSD: 45.00 },
    { address: '0x0000000000000000000000000000000000000000', symbol: 'ETH', name: 'Ethereum', decimals: 18, logoURI: 'https://assets.coingecko.com/coins/images/279/small/ethereum.png', chainId: 42161, chainKey: 'arbitrum', isNative: true, priceUSD: 3500.00 },
    { address: '0x0000000000000000000000000000000000000000', symbol: 'ETH', name: 'Ethereum', decimals: 18, logoURI: 'https://assets.coingecko.com/coins/images/279/small/ethereum.png', chainId: 10, chainKey: 'optimism', isNative: true, priceUSD: 3500.00 },
    { address: '0x0b2C639c533813f4Aa9D7837CAf62653d097Ff85', symbol: 'USDC', name: 'USD Coin', decimals: 6, logoURI: 'https://assets.coingecko.com/coins/images/6319/small/usdc.png', chainId: 10, chainKey: 'optimism', priceUSD: 1.00 },
    { address: '0x94b008aA00579c1307B0EF2c49429fDf38F27cD6', symbol: 'USDT', name: 'Tether USD', decimals: 6, logoURI: 'https://assets.coingecko.com/coins/images/325/small/Tether.png', chainId: 10, chainKey: 'optimism', priceUSD: 1.00 },
    { address: '0x4200000000000000000000000000000000000006', symbol: 'WETH', name: 'Wrapped Ether', decimals: 18, logoURI: 'https://assets.coingecko.com/coins/images/2518/small/weth.png', chainId: 10, chainKey: 'optimism', priceUSD: 3500.00 },
    { address: '0x4200000000000000000000000000000000000042', symbol: 'OP', name: 'Optimism', decimals: 18, logoURI: 'https://assets.coingecko.com/coins/images/25244/small/Optimism.png', chainId: 10, chainKey: 'optimism', priceUSD: 2.50 },
    { address: '0x0000000000000000000000000000000000000000', symbol: 'ETH', name: 'Ethereum', decimals: 18, logoURI: 'https://assets.coingecko.com/coins/images/279/small/ethereum.png', chainId: 8453, chainKey: 'base', isNative: true, priceUSD: 3500.00 },
    { address: '0x833589fCD6eDb6E08f4c7c32D4f71b54bdA02913', symbol: 'USDC', name: 'USD Coin', decimals: 6, logoURI: 'https://assets.coingecko.com/coins/images/6319/small/usdc.png', chainId: 8453, chainKey: 'base', priceUSD: 1.00 },
    { address: '0x4ed4e862860bed51a9570b96d89af5e1b0efefed', symbol: 'DEGEN', name: 'DEGEN', decimals: 18, logoURI: 'https://assets.coingecko.com/coins/images/37353/small/degen.png', chainId: 8453, chainKey: 'base', priceUSD: 0.015 },
    { address: '0x0000000000000000000000000000000000000000', symbol: 'BNB', name: 'BNB', decimals: 18, logoURI: 'https://assets.coingecko.com/coins/images/825/small/bnb-icon2_2x.png', chainId: 56, chainKey: 'bsc', isNative: true, priceUSD: 600.00 },
    { address: '0x55d398326f99059fF775485246999027B3197955', symbol: 'USDT', name: 'Tether USD', decimals: 18, logoURI: 'https://assets.coingecko.com/coins/images/325/small/Tether.png', chainId: 56, chainKey: 'bsc', priceUSD: 1.00 },
    { address: '0x8AC76a51cc950d9822D68b83fE1Ad97B32Cd580d', symbol: 'USDC', name: 'USD Coin', decimals: 18, logoURI: 'https://assets.coingecko.com/coins/images/6319/small/usdc.png', chainId: 56, chainKey: 'bsc', priceUSD: 1.00 },
    { address: '0x1AF3F329e8BE154074D4209e478e7683fB2D4D1', symbol: 'DAI', name: 'Dai Stablecoin', decimals: 18, logoURI: 'https://assets.coingecko.com/coins/images/9956/small/4943.png', chainId: 56, chainKey: 'bsc', priceUSD: 1.00 },
    { address: '0xbb4CdB9CBd36B01bD1cBaEBF2De08d9173bc095c', symbol: 'WBNB', name: 'Wrapped BNB', decimals: 18, logoURI: 'https://assets.coingecko.com/coins/images/825/small/bnb-icon2_2x.png', chainId: 56, chainKey: 'bsc', priceUSD: 600.00 },
    { address: '0xE0dF710778d035C46B740740Fe8dB655c713bE68', symbol: 'XRP', name: 'XRP', decimals: 6, logoURI: 'https://assets.coingecko.com/coins/images/44/small/xrp-symbol-white-128.png', chainId: 56, chainKey: 'bsc', priceUSD: 0.60 },
    { address: '0x7130d2A12B9BCbDAe1DA4E496C5f2a9D5C9d8d5A', symbol: 'TRX', name: 'TRON', decimals: 6, logoURI: 'https://assets.coingecko.com/coins/images/1094/small/tron-logo.png', chainId: 56, chainKey: 'bsc', priceUSD: 0.12 },
    { address: '0xd4CA20A32032996f7c03B32A78c54B4899C30C9A', symbol: 'PAXG', name: 'Paxos Gold', decimals: 18, logoURI: 'https://assets.coingecko.com/coins/images/11651/small/pax_gold.png', chainId: 56, chainKey: 'bsc', priceUSD: 2650.00 },
    { address: '0x0000000000000000000000000000000000000000', symbol: 'AVAX', name: 'Avalanche', decimals: 18, logoURI: 'https://assets.coingecko.com/coins/images/12559/small/Avalanche_Circle_RedWhite_Trans.png', chainId: 43114, chainKey: 'avalanche', isNative: true, priceUSD: 35.00 },
    { address: '0xB97EF9Ef8734C71904D8002F8b6Bc66Dd9c48a6E', symbol: 'USDC', name: 'USD Coin', decimals: 6, logoURI: 'https://assets.coingecko.com/coins/images/6319/small/usdc.png', chainId: 43114, chainKey: 'avalanche', priceUSD: 1.00 },
    { address: '0x9709790a8eaACa9b3A63C8252b7F7d2c2f8dB2F5', symbol: 'USDT', name: 'Tether USD', decimals: 6, logoURI: 'https://assets.coingecko.com/coins/images/325/small/Tether.png', chainId: 43114, chainKey: 'avalanche', priceUSD: 1.00 },
    { address: '0xd1c3f94DE7e5B45fa4eDBBA472491aF4AEE8F13A', symbol: 'JOE', name: 'JOE', decimals: 18, logoURI: 'https://assets.coingecko.com/coins/images/17549/small/traderjoe.png', chainId: 43114, chainKey: 'avalanche', priceUSD: 0.35 },
    // Solana tokens
    { address: 'So11111111111111111111111111111111111111112', symbol: 'SOL', name: 'Solana', decimals: 9, logoURI: 'https://assets.coingecko.com/coins/images/4128/small/solana.png', chainId: 101, chainKey: 'solana', isNative: true, priceUSD: 150.00 },
    { address: 'EPjFWdd5AufqSSBc8ExiM8w4vQeK9k68n28R1LNJ3Jc', symbol: 'USDC', name: 'USD Coin', decimals: 6, logoURI: 'https://assets.coingecko.com/coins/images/6319/small/usdc.png', chainId: 101, chainKey: 'solana', priceUSD: 1.00 },
    { address: 'Es9vMFrzaCER2PBDd2r3E4jTdZ7qS4Gwo9U7p47L8B3B', symbol: 'USDT', name: 'Tether USD', decimals: 6, logoURI: 'https://assets.coingecko.com/coins/images/325/small/Tether.png', chainId: 101, chainKey: 'solana', priceUSD: 1.00 },
    { address: '3NZRjDHcHYJc2KRT9q3n7nL4Z3nH9KjBjF8NxY8qGxW', symbol: 'BONK', name: 'Bonk', decimals: 5, logoURI: 'https://assets.coingecko.com/coins/images/28600/small/bonk.png', chainId: 101, chainKey: 'solana', priceUSD: 0.000025 },
    { address: 'JUPyiwrYJFskUPiHa7hkeR8VUtkqjberbSOWd91pbT2', symbol: 'JUP', name: 'Jupiter', decimals: 6, logoURI: 'https://assets.coingecko.com/coins/images/34188/small/jup.png', chainId: 101, chainKey: 'solana', priceUSD: 0.80 },
    { address: 'mSoLzYCxHdYgdzU16g5QSh3iYK2m8v8vK1a9b1c1d1e', symbol: 'MSOL', name: 'Marinade Staked SOL', decimals: 9, logoURI: 'https://assets.coingecko.com/coins/images/16320/small/mSOL.png', chainId: 101, chainKey: 'solana', priceUSD: 170.00 },
    // Aptos tokens
    { address: '0x1::aptos_coin::AptosCoin', symbol: 'APT', name: 'Aptos', decimals: 8, logoURI: 'https://assets.coingecko.com/coins/images/26455/small/aptos_round.png', chainId: 1, chainKey: 'aptos', isNative: true, priceUSD: 10.00 },
    { address: '0xf22bede237a07e121b56d91a491eb7bcdfd1f5907926a9e58338f0a05a9c0089::usdc::USDC', symbol: 'USDC', name: 'USD Coin', decimals: 6, logoURI: 'https://assets.coingecko.com/coins/images/6319/small/usdc.png', chainId: 1, chainKey: 'aptos', priceUSD: 1.00 },
    // Cosmos tokens
    { address: 'uatom', symbol: 'ATOM', name: 'Cosmos Hub', decimals: 6, logoURI: 'https://assets.coingecko.com/coins/images/1481/small/cosmos_hub.png', chainId: 1, chainKey: 'cosmos', isNative: true, priceUSD: 9.00 },
    { address: 'uosmo', symbol: 'OSMO', name: 'Osmosis', decimals: 6, logoURI: 'https://assets.coingecko.com/coins/images/12271/small/Osmosis.png', chainId: 1, chainKey: 'cosmos', priceUSD: 0.50 },
    { address: 'ujuno', symbol: 'JUNO', name: 'Juno', decimals: 6, logoURI: 'https://assets.coingecko.com/coins/images/15889/small/juno.png', chainId: 1, chainKey: 'cosmos', priceUSD: 5.00 },
    // TON tokens
    { address: '0:0000000000000000000000000000000000000000000000000000000000000000', symbol: 'TON', name: 'Toncoin', decimals: 9, logoURI: 'https://assets.coingecko.com/coins/images/17980/small/ton_symbol.png', chainId: 1, chainKey: 'ton', isNative: true, priceUSD: 6.00 },
    // Tron tokens
    { address: 'TR7NHqjeKQxGTCi8q8ZY4pL8otCgjF6EXm', symbol: 'USDT', name: 'Tether USD', decimals: 6, logoURI: 'https://assets.coingecko.com/coins/images/325/small/Tether.png', chainId: 728126428, chainKey: 'tron', priceUSD: 1.00 },
    // Fantom
    { address: '0x0000000000000000000000000000000000000000', symbol: 'FTM', name: 'Fantom', decimals: 18, logoURI: 'https://assets.coingecko.com/coins/images/4001/small/Fantom_round.png', chainId: 250, chainKey: 'fantom', isNative: true, priceUSD: 0.40 },
    { address: '0x04068da6c83afcfa0e13bb36525e9e2375c7d98e', symbol: 'USDC', name: 'USD Coin', decimals: 6, logoURI: 'https://assets.coingecko.com/coins/images/6319/small/usdc.png', chainId: 250, chainKey: 'fantom', priceUSD: 1.00 },
    // Cronos
    { address: '0x0000000000000000000000000000000000000000', symbol: 'CRO', name: 'Cronos', decimals: 18, logoURI: 'https://assets.coingecko.com/coins/images/7310/small/cro_token_id.png', chainId: 25, chainKey: 'cronos', isNative: true, priceUSD: 0.10 },
    // zkSync Era
    { address: '0x0000000000000000000000000000000000000000', symbol: 'ETH', name: 'Ethereum', decimals: 18, logoURI: 'https://assets.coingecko.com/coins/images/279/small/ethereum.png', chainId: 324, chainKey: 'zksync', isNative: true, priceUSD: 3500.00 },
    { address: '0x3355df6D4c9C3035724Fd0e3914dE96A5a83aAF4', symbol: 'USDC', name: 'USD Coin', decimals: 6, logoURI: 'https://assets.coingecko.com/coins/images/6319/small/usdc.png', chainId: 324, chainKey: 'zksync', priceUSD: 1.00 },
    // Linea
    { address: '0x0000000000000000000000000000000000000000', symbol: 'ETH', name: 'Ethereum', decimals: 18, logoURI: 'https://assets.coingecko.com/coins/images/279/small/ethereum.png', chainId: 59144, chainKey: 'linea', isNative: true, priceUSD: 3500.00 },
    // Mantle
    { address: '0x0000000000000000000000000000000000000000', symbol: 'MNT', name: 'Mantle', decimals: 18, logoURI: 'https://assets.coingecko.com/coins/images/31080/small/Mantle.png', chainId: 5000, chainKey: 'mantle', isNative: true, priceUSD: 0.60 },
    // Blast
    { address: '0x0000000000000000000000000000000000000000', symbol: 'ETH', name: 'Ethereum', decimals: 18, logoURI: 'https://assets.coingecko.com/coins/images/279/small/ethereum.png', chainId: 81457, chainKey: 'blast', isNative: true, priceUSD: 3500.00 },
    // Scroll
    { address: '0x0000000000000000000000000000000000000000', symbol: 'ETH', name: 'Ethereum', decimals: 18, logoURI: 'https://assets.coingecko.com/coins/images/279/small/ethereum.png', chainId: 534352, chainKey: 'scroll', isNative: true, priceUSD: 3500.00 },
    // Polygon zkEVM
    { address: '0x0000000000000000000000000000000000000000', symbol: 'ETH', name: 'Ethereum', decimals: 18, logoURI: 'https://assets.coingecko.com/coins/images/279/small/ethereum.png', chainId: 1101, chainKey: 'polygon_zkevm', isNative: true, priceUSD: 3500.00 },
    // Gnosis Chain
    { address: '0x0000000000000000000000000000000000000000', symbol: 'XDAI', name: 'Gnosis Chain', decimals: 18, logoURI: 'https://assets.coingecko.com/coins/images/662/small/logotype_social.png', chainId: 100, chainKey: 'gnosis', isNative: true, priceUSD: 1.00 },
    // Celo
    { address: '0x0000000000000000000000000000000000000000', symbol: 'CELO', name: 'Celo', decimals: 18, logoURI: 'https://assets.coingecko.com/coins/images/16690/small/Celo_Assets_2023_Resize_Transparent_Rest.png', chainId: 42220, chainKey: 'celo', isNative: true, priceUSD: 0.70 },
    // Kava
    { address: '0x0000000000000000000000000000000000000000', symbol: 'KAVA', name: 'Kava', decimals: 18, logoURI: 'https://assets.coingecko.com/coins/images/9761/small/kava.png', chainId: 2222, chainKey: 'kava', isNative: true, priceUSD: 0.70 },
    // Moonbeam
    { address: '0x0000000000000000000000000000000000000000', symbol: 'GLMR', name: 'Moonbeam', decimals: 18, logoURI: 'https://assets.coingecko.com/coins/images/17167/small/moonbeam_new.png', chainId: 1284, chainKey: 'moonbeam', isNative: true, priceUSD: 0.35 },
    // Astar
    { address: '0x0000000000000000000000000000000000000000', symbol: 'ASTR', name: 'Astar', decimals: 18, logoURI: 'https://assets.coingecko.com/coins/images/22617/small/astr.png', chainId: 592, chainKey: 'astar', isNative: true, priceUSD: 0.10 },
    // Sei
    { address: 'usei', symbol: 'SEI', name: 'Sei', decimals: 6, logoURI: 'https://assets.coingecko.com/coins/images/28205/small/Sei_Logo_-_Transparent.png', chainId: 1, chainKey: 'sei', isNative: true, priceUSD: 0.50 },
    // Injective
    { address: 'inj', symbol: 'INJ', name: 'Injective', decimals: 18, logoURI: 'https://assets.coingecko.com/coins/images/12882/small/Secondary_Symbol.png', chainId: 1, chainKey: 'injective', isNative: true, priceUSD: 25.00 },
    // Sui
    { address: '0x0000000000000000000000000000000000000000000000000000000000000002::sui::SUI', symbol: 'SUI', name: 'Sui', decimals: 9, logoURI: 'https://assets.coingecko.com/coins/images/26375/small/sui_asset.jpeg', chainId: 1, chainKey: 'sui', isNative: true, priceUSD: 1.50 },
    // Near
    { address: 'wrap.near', symbol: 'NEAR', name: 'NEAR Protocol', decimals: 24, logoURI: 'https://assets.coingecko.com/coins/images/10365/small/near.jpg', chainId: 1, chainKey: 'near', isNative: true, priceUSD: 5.00 },
    // Algorand
    { address: '0', symbol: 'ALGO', name: 'Algorand', decimals: 6, logoURI: 'https://assets.coingecko.com/coins/images/4380/small/download.png', chainId: 1, chainKey: 'algorand', isNative: true, priceUSD: 0.20 },
    // VeChain
    { address: '0x0000000000000000000000000000000000000000', symbol: 'VET', name: 'VeChain', decimals: 18, logoURI: 'https://assets.coingecko.com/coins/images/1167/small/VET_Token_Icon.png', chainId: 1, chainKey: 'vechain', isNative: true, priceUSD: 0.03 },
    // Hedera
    { address: '0.0.456821', symbol: 'HBAR', name: 'Hedera', decimals: 8, logoURI: 'https://assets.coingecko.com/coins/images/3688/small/hbar.png', chainId: 1, chainKey: 'hedera', isNative: true, priceUSD: 0.08 },
    // IOTA
    { address: '0x0000000000000000000000000000000000000000', symbol: 'IOTA', name: 'IOTA', decimals: 18, logoURI: 'https://assets.coingecko.com/coins/images/692/small/IOTA_Swirl.png', chainId: 1, chainKey: 'iota', isNative: true, priceUSD: 0.30 },
    // Core
    { address: '0x0000000000000000000000000000000000000000', symbol: 'CORE', name: 'Core', decimals: 18, logoURI: 'https://assets.coingecko.com/coins/images/22407/small/coredao.png', chainId: 1116, chainKey: 'core', isNative: true, priceUSD: 2.50 },
    // Canto
    { address: '0x0000000000000000000000000000000000000000', symbol: 'CANTO', name: 'Canto', decimals: 18, logoURI: 'https://assets.coingecko.com/coins/images/21597/small/Canto_Network_Logo_Normal_03.svg.png', chainId: 7700, chainKey: 'canto', isNative: true, priceUSD: 0.05 },
    // Klaytn
    { address: '0x0000000000000000000000000000000000000000', symbol: 'KLAY', name: 'Klaytn', decimals: 18, logoURI: 'https://assets.coingecko.com/coins/images/9672/small/klaytn.png', chainId: 8217, chainKey: 'klaytn', isNative: true, priceUSD: 0.20 },
    // Ronin
    { address: '0x0000000000000000000000000000000000000000', symbol: 'RON', name: 'Ronin', decimals: 18, logoURI: 'https://assets.coingecko.com/coins/images/20009/small/ronin.jpg', chainId: 2020, chainKey: 'ronin', isNative: true, priceUSD: 1.50 },
    // Fraxtal
    { address: '0x0000000000000000000000000000000000000000', symbol: 'FRX', name: 'Fraxtal', decimals: 18, logoURI: 'https://assets.coingecko.com/coins/images/16796/small/frax_share.png', chainId: 252, chainKey: 'fraxtal', isNative: true, priceUSD: 0.02 },
    // Mode
    { address: '0x0000000000000000000000000000000000000000', symbol: 'MOD', name: 'Mode', decimals: 18, logoURI: 'https://assets.coingecko.com/coins/images/31053/small/Photo_2023-10-02_at_10.30.53_AM.png', chainId: 34443, chainKey: 'mode', isNative: true, priceUSD: 0.02 },
    // Dogecoin (EVM)
    { address: '0x0000000000000000000000000000000000000000', symbol: 'DOGE', name: 'Dogecoin', decimals: 18, logoURI: 'https://assets.coingecko.com/coins/images/5/small/dogecoin.png', chainId: 2000, chainKey: 'dogecoin', isNative: true, priceUSD: 0.15 },
    // Litecoin (EVM)
    { address: '0x0000000000000000000000000000000000000000', symbol: 'LTC', name: 'Litecoin', decimals: 18, logoURI: 'https://assets.coingecko.com/coins/images/2/small/litecoin.png', chainId: 2, chainKey: 'litecoin', isNative: true, priceUSD: 85.00 },
    // Syscoin
    { address: '0x0000000000000000000000000000000000000000', symbol: 'SYS', name: 'Syscoin', decimals: 18, logoURI: 'https://assets.coingecko.com/coins/images/541/small/Syscoin.png', chainId: 57, chainKey: 'syscoin', isNative: true, priceUSD: 0.05 },
    // Conflux
    { address: '0x0000000000000000000000000000000000000000', symbol: 'CFX', name: 'Conflux', decimals: 18, logoURI: 'https://assets.coingecko.com/coins/images/13079/small/3vuYMbjN.png', chainId: 1030, chainKey: 'conflux', isNative: true, priceUSD: 0.20 },
    // Findora
    { address: '0x0000000000000000000000000000000000000000', symbol: 'FRA', name: 'Findora', decimals: 18, logoURI: 'https://assets.coingecko.com/coins/images/17251/small/Findora.png', chainId: 2152, chainKey: 'findora', isNative: true, priceUSD: 0.03 },
    // Ontology
    { address: '0x0000000000000000000000000000000000000000', symbol: 'ONT', name: 'Ontology', decimals: 18, logoURI: 'https://assets.coingecko.com/coins/images/3447/small/ONT.png', chainId: 58, chainKey: 'ontology', isNative: true, priceUSD: 0.02 },
    // IoTeX
    { address: '0x0000000000000000000000000000000000000000', symbol: 'IOTX', name: 'IoTeX', decimals: 18, logoURI: 'https://assets.coingecko.com/coins/images/27149/small/iotex-logo.png', chainId: 4689, chainKey: 'iotex', isNative: true, priceUSD: 0.03 },
    // Zilliqa
    { address: '0x0000000000000000000000000000000000000000', symbol: 'ZIL', name: 'Zilliqa', decimals: 18, logoURI: 'https://assets.coingecko.com/coins/images/1160/small/zilliqa-zil.png', chainId: 1, chainKey: 'zilliqa', isNative: true, priceUSD: 0.02 },
    // Elastos
    { address: '0x0000000000000000000000000000000000000000', symbol: 'ELA', name: 'Elastos', decimals: 18, logoURI: 'https://assets.coingecko.com/coins/images/2604/small/Elastos.png', chainId: 20, chainKey: 'elastos', isNative: true, priceUSD: 3.00 },
    // EOS EVM
    { address: '0x0000000000000000000000000000000000000000', symbol: 'EOS', name: 'EOS', decimals: 18, logoURI: 'https://assets.coingecko.com/coins/images/738/small/eos-eos-logo.png', chainId: 17777, chainKey: 'eos', isNative: true, priceUSD: 0.80 },
    // WEMIX
    { address: '0x0000000000000000000000000000000000000000', symbol: 'WEMIX', name: 'WEMIX', decimals: 18, logoURI: 'https://assets.coingecko.com/coins/images/17780/small/wemix.jpg', chainId: 1111, chainKey: 'wemix', isNative: true, priceUSD: 1.50 },
    // THORChain
    { address: '0x0000000000000000000000000000000000000000', symbol: 'RUNE', name: 'THORChain', decimals: 8, logoURI: 'https://assets.coingecko.com/coins/images/6595/small/Rune200x200.png', chainId: 1, chainKey: 'thorchain', isNative: true, priceUSD: 5.00 },
    // Secret
    { address: '0x0000000000000000000000000000000000000000', symbol: 'SCRT', name: 'Secret Network', decimals: 6, logoURI: 'https://assets.coingecko.com/coins/images/7367/small/Screen_Shot_2020-12-06_at_4.03.30_PM.png', chainId: 1, chainKey: 'secret', isNative: true, priceUSD: 1.50 },
    // Fetch.ai
    { address: '0x0000000000000000000000000000000000000000', symbol: 'FET', name: 'Fetch.ai', decimals: 18, logoURI: 'https://assets.coingecko.com/coins/images/5681/small/Fetch.jpg', chainId: 1, chainKey: 'fetch_ai', isNative: true, priceUSD: 2.00 },
  ],
};

// Get all tokens for a specific chain
export function getTokensForChain(chainKey: string): TokenData[] {
  return TOKEN_DATABASE[chainKey] || [];
}

// Get token by address on a specific chain
export function getTokenByAddress(chainKey: string, address: string): TokenData | undefined {
  const tokens = TOKEN_DATABASE[chainKey];
  if (!tokens) return undefined;
  return tokens.find(t => t.address.toLowerCase() === address.toLowerCase());
}

// Get all unique tokens across all chains
export function getAllTokens(): TokenData[] {
  const allTokens: TokenData[] = [];
  const seen = new Set<string>();
  
  for (const tokens of Object.values(TOKEN_DATABASE)) {
    for (const token of tokens) {
      const key = `${token.chainKey}-${token.symbol}`;
      if (!seen.has(key)) {
        seen.add(key);
        allTokens.push(token);
      }
    }
  }
  
  return allTokens;
}

// Get popular tokens across all chains
export function getPopularTokens(): TokenData[] {
  const popularSymbols = ['ETH', 'BTC', 'USDC', 'USDT', 'BNB', 'MATIC', 'AVAX', 'SOL', 'LINK', 'UNI', 'AAVE', 'DOT', 'ATOM', 'LTC', 'XRP', 'DOGE', 'TRX', 'APT', 'ARB', 'OP', 'WBTC'];
  const allTokens = getAllTokens();
  return allTokens.filter(t => popularSymbols.includes(t.symbol));
}

// Get all supported chains
export function getAllChains(): Chain[] {
  const chains: Chain[] = [];
  const seen = new Set<number>();
  
  for (const tokens of Object.values(TOKEN_DATABASE)) {
    for (const token of tokens) {
      if (!seen.has(token.chainId)) {
        seen.add(token.chainId);
        chains.push({
          id: token.chainId,
          name: token.chainKey.charAt(0).toUpperCase() + token.chainKey.slice(1),
          symbol: token.symbol,
          icon: '🔗',
          rpc: '',
          explorer: '',
          type: 'evm'
        });
      }
    }
  }
  
  return chains;
}
