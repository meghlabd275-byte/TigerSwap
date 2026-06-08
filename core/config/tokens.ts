// Pre-installed Tokens Configuration
// Top 50 tokens across all blockchains

export interface TokenConfig {
  id: string;
  name: string;
  symbol: string;
  decimals: number;
  contractAddress: string | null;
  blockchainId: string;
  logoUrl: string;
  priceUsd: number;
  isStablecoin: boolean;
  isActive: boolean;
  category: 'native' | 'stablecoin' | 'utility' | 'governance' | 'memecoin';
}

// Top 50 Tokens
export const topTokens: TokenConfig[] = [
  // Native Tokens
  { id: 'eth', name: 'Ethereum', symbol: 'ETH', decimals: 18, contractAddress: null, blockchainId: 'ethereum', logoUrl: 'https://assets.coingecko.com/coins/images/279/small/ethereum.png', priceUsd: 3500, isStablecoin: false, isActive: true, category: 'native' },
  { id: 'btc', name: 'Bitcoin', symbol: 'BTC', decimals: 8, contractAddress: null, blockchainId: 'bitcoin', logoUrl: 'https://assets.coingecko.com/coins/images/1/small/bitcoin.png', priceUsd: 65000, isStablecoin: false, isActive: true, category: 'native' },
  { id: 'bnb', name: 'BNB', symbol: 'BNB', decimals: 18, contractAddress: null, blockchainId: 'bsc', logoUrl: 'https://assets.coingecko.com/coins/images/825/small/bnb-icon2_2x.png', priceUsd: 600, isStablecoin: false, isActive: true, category: 'native' },
  { id: 'sol', name: 'Solana', symbol: 'SOL', decimals: 9, contractAddress: null, blockchainId: 'solana', logoUrl: 'https://assets.coingecko.com/coins/images/4128/small/solana.png', priceUsd: 145, isStablecoin: false, isActive: true, category: 'native' },
  { id: 'matic', name: 'Polygon', symbol: 'MATIC', decimals: 18, contractAddress: null, blockchainId: 'polygon', logoUrl: 'https://assets.coingecko.com/coins/images/4713/small/matic-token-icon.png', priceUsd: 0.85, isStablecoin: false, isActive: true, category: 'native' },
  { id: 'avax', name: 'Avalanche', symbol: 'AVAX', decimals: 18, contractAddress: null, blockchainId: 'avalanche', logoUrl: 'https://assets.coingecko.com/coins/images/12559/small/Avalanche_Circle_RedWhite_Trans.png', priceUsd: 35, isStablecoin: false, isActive: true, category: 'native' },
  { id: 'op', name: 'Optimism', symbol: 'OP', decimals: 18, contractAddress: null, blockchainId: 'optimism', logoUrl: 'https://assets.coingecko.com/coins/images/25244/small/Optimism.png', priceUsd: 2.5, isStablecoin: false, isActive: true, category: 'native' },
  { id: 'arb', name: 'Arbitrum', symbol: 'ETH', decimals: 18, contractAddress: null, blockchainId: 'arbitrum', logoUrl: 'https://assets.coingecko.com/coins/images/16547/small/photo_2023-03-29_21.47.00.jpeg', priceUsd: 1.1, isStablecoin: false, isActive: true, category: 'native' },
  { id: 'ftm', name: 'Fantom', symbol: 'FTM', decimals: 18, contractAddress: null, blockchainId: 'fantom', logoUrl: 'https://assets.coingecko.com/coins/images/4001/small/Fantom_round.png', priceUsd: 0.8, isStablecoin: false, isActive: true, category: 'native' },
  { id: 'cro', name: 'Cronos', symbol: 'CRO', decimals: 18, contractAddress: null, blockchainId: 'cronos', logoUrl: 'https://assets.coingecko.com/coins/images/7310/small/cro_token_logo.png', priceUsd: 0.15, isStablecoin: false, isActive: true, category: 'native' },
  { id: 'atom', name: 'Cosmos', symbol: 'ATOM', decimals: 6, contractAddress: null, blockchainId: 'cosmos', logoUrl: 'https://assets.coingecko.com/coins/images/1481/small/cosmos_hub.png', priceUsd: 9, isStablecoin: false, isActive: true, category: 'native' },
  { id: 'near', name: 'Near', symbol: 'NEAR', decimals: 24, contractAddress: null, blockchainId: 'near', logoUrl: 'https://assets.coingecko.com/coins/images/10365/small/near.jpg', priceUsd: 5, isStablecoin: false, isActive: true, category: 'native' },
  { id: 'apt', name: 'Aptos', symbol: 'APT', decimals: 8, contractAddress: null, blockchainId: 'aptos', logoUrl: 'https://assets.coingecko.com/coins/images/26455/small/aptos_round.png', priceUsd: 10, isStablecoin: false, isActive: true, category: 'native' },
  { id: 'sui', name: 'Sui', symbol: 'SUI', decimals: 9, contractAddress: null, blockchainId: 'sui', logoUrl: 'https://assets.coingecko.com/coins/images/26375/small/sui_asset.jpeg', priceUsd: 1.8, isStablecoin: false, isActive: true, category: 'native' },
  { id: 'dot', name: 'Polkadot', symbol: 'DOT', decimals: 10, contractAddress: null, blockchainId: 'polkadot', logoUrl: 'https://assets.coingecko.com/coins/images/12171/small/polkadot.png', priceUsd: 7, isStablecoin: false, isActive: true, category: 'native' },

  // Stablecoins
  { id: 'usdt', name: 'Tether', symbol: 'USDT', decimals: 6, contractAddress: '0xdAC17F958D2ee523a2206206994597C13D831ec7', blockchainId: 'ethereum', logoUrl: 'https://assets.coingecko.com/coins/images/325/small/Tether.png', priceUsd: 1.0, isStablecoin: true, isActive: true, category: 'stablecoin' },
  { id: 'usdc', name: 'USD Coin', symbol: 'USDC', decimals: 6, contractAddress: '0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48', blockchainId: 'ethereum', logoUrl: 'https://assets.coingecko.com/coins/images/6319/small/usdc.png', priceUsd: 1.0, isStablecoin: true, isActive: true, category: 'stablecoin' },
  { id: 'dai', name: 'Dai', symbol: 'DAI', decimals: 18, contractAddress: '0x6B175474E89094C44Da98b954EdeACF495a8eEA9', blockchainId: 'ethereum', logoUrl: 'https://assets.coingecko.com/coins/images/9956/small/SAI_dai_circle.png', priceUsd: 1.0, isStablecoin: true, isActive: true, category: 'stablecoin' },
  { id: 'busd', name: 'Binance USD', symbol: 'BUSD', decimals: 18, contractAddress: '0x4Fabb145d64652a948D7288886B8Fd7f7F5ab538', blockchainId: 'bsc', logoUrl: 'https://assets.coingecko.com/coins/images/9576/small/BUSD.png', priceUsd: 1.0, isStablecoin: true, isActive: true, category: 'stablecoin' },
  { id: 'tusd', name: 'TrueUSD', symbol: 'TUSD', decimals: 18, contractAddress: '0x0000000000085CeD6087B3d6b4d2F2bd3CEf59A4C7', blockchainId: 'ethereum', logoUrl: 'https://assets.coingecko.com/coins/images/3449/small/tusd.png', priceUsd: 1.0, isStablecoin: true, isActive: true, category: 'stablecoin' },
  { id: 'usdp', name: 'Pax Dollar', symbol: 'USDP', decimals: 18, contractAddress: '0x8E870D67F660D95d5CC5461B4C8eC5a7f50aE7dC', blockchainId: 'ethereum', logoUrl: 'https://assets.coingecko.com/coins/images/6013/small/paxos.png', priceUsd: 1.0, isStablecoin: true, isActive: true, category: 'stablecoin' },
  { id: 'frax', name: 'Frax', symbol: 'FRAX', decimals: 18, contractAddress: '0x853d955aCEf822Db058eb8505911ED77F175b99e', blockchainId: 'ethereum', logoUrl: 'https://assets.coingecko.com/coins/images/9952/small/frax_logo.png', priceUsd: 1.0, isStablecoin: true, isActive: true, category: 'stablecoin' },
  { id: 'gusd', name: 'Gemini Dollar', symbol: 'GUSD', decimals: 2, contractAddress: '0x056Fd409E1d539aDe8ad09d9AC2B80eCcC98e7A2', blockchainId: 'ethereum', logoUrl: 'https://assets.coingecko.com/coins/images/5012/small/gemini-dollar-gusd.png', priceUsd: 1.0, isStablecoin: true, isActive: true, category: 'stablecoin' },
  { id: 'mim', name: 'Magic Internet Money', symbol: 'MIM', decimals: 18, contractAddress: '0x99D8a9C45b2A86c008234F1C2d0a49f8d1c2Fa8b', blockchainId: 'ethereum', logoUrl: 'https://assets.coingecko.com/coins/images/15057/small/mimlogopng.png', priceUsd: 1.0, isStablecoin: true, isActive: true, category: 'stablecoin' },
  { id: 'usdd', name: 'USDD', symbol: 'USDD', decimals: 18, contractAddress: '0x0C10bF6FD80A42410412E5D52A6d4D8A02B2F97b', blockchainId: 'ethereum', logoUrl: 'https://assets.coingecko.com/coins/images/25380/small/photo_2022-05-03_18.53.39.jpg', priceUsd: 1.0, isStablecoin: true, isActive: true, category: 'stablecoin' },

  // Utility Tokens
  { id: 'link', name: 'Chainlink', symbol: 'LINK', decimals: 18, contractAddress: '0x514910771AF9C656259879C36Dde3CeFB1979AC8', blockchainId: 'ethereum', logoUrl: 'https://assets.coingecko.com/coins/images/877/small/chainlink-new-logo.png', priceUsd: 15, isStablecoin: false, isActive: true, category: 'utility' },
  { id: 'uni', name: 'Uniswap', symbol: 'UNI', decimals: 18, contractAddress: '0x1f9840a85d5aF5bf1D1762F925BDADdC4201F984', blockchainId: 'ethereum', logoUrl: 'https://assets.coingecko.com/coins/images/12504/small/uni.jpg', priceUsd: 10, isStablecoin: false, isActive: true, category: 'utility' },
  { id: 'aave', name: 'Aave', symbol: 'AAVE', decimals: 18, contractAddress: '0x7fc66500c84a76Ad7e9e934DCbC9835B6B1D0eB2', blockchainId: 'ethereum', logoUrl: 'https://assets.coingecko.com/coins/images/12645/small/AAVE.png', priceUsd: 250, isStablecoin: false, isActive: true, category: 'utility' },
  { id: 'mkr', name: 'Maker', symbol: 'MKR', decimals: 18, contractAddress: '0x9f8F72aA9304c8B593d555F12eF6589c4C2e5d5', blockchainId: 'ethereum', logoUrl: 'https://assets.coingecko.com/coins/images/1364/small/Mark_Maker.png', priceUsd: 2500, isStablecoin: false, isActive: true, category: 'governance' },
  { id: 'crv', name: 'Curve DAO', symbol: 'CRV', decimals: 18, contractAddress: '0xD533a949740bb3306d119CC777fa900bA034cd51', blockchainId: 'ethereum', logoUrl: 'https://assets.coingecko.com/coins/images/24/small/Curve.png', priceUsd: 0.5, isStablecoin: false, isActive: true, category: 'governance' },
  { id: 'comp', name: 'Compound', symbol: 'COMP', decimals: 18, contractAddress: '0xc00e94Cb662C3520282E6f5717214004A3f5cB76', blockchainId: 'ethereum', logoUrl: 'https://assets.coingecko.com/coins/images/10775/small/COMP.png', priceUsd: 50, isStablecoin: false, isActive: true, category: 'utility' },
  { id: 'snx', name: 'Synthetix', symbol: 'SNX', decimals: 18, contractAddress: '0xC011a73ee8576Fb46F5E1c25FcE17eCEaeeEF6b5', blockchainId: 'ethereum', logoUrl: 'https://assets.coingecko.com/coins/images/3406/small/SNX.png', priceUsd: 3, isStablecoin: false, isActive: true, category: 'utility' },
  { id: 'sushi', name: 'SushiSwap', symbol: 'SUSHI', decimals: 18, contractAddress: '0x6B3595068778DD592e39A122f4f5a5cF09d90fE8', blockchainId: 'ethereum', logoUrl: 'https://assets.coingecko.com/coins/images/12271/small/512x512_Logo_no_chikara_avatar.png', priceUsd: 1.2, isStablecoin: false, isActive: true, category: 'utility' },
  { id: 'cake', name: 'PancakeSwap', symbol: 'CAKE', decimals: 18, contractAddress: '0x0E09FaBBFAdB95B79BC23Da066f5B27c2d9F72Be', blockchainId: 'bsc', logoUrl: 'https://assets.coingecko.com/coins/images/12632/small/pancakeswap-cake-logo_%281%29.png', priceUsd: 2.5, isStablecoin: false, isActive: true, category: 'utility' },
  { id: 'gmx', name: 'GMX', symbol: 'GMX', decimals: 18, contractAddress: '0xfc5A1A6EB076a2c7ad05eA2cdD660bc7E7da606', blockchainId: 'arbitrum', logoUrl: 'https://assets.coingecko.com/coins/images/15899/small/photo_2022-12-05_18.23.32.jpg', priceUsd: 50, isStablecoin: false, isActive: true, category: 'utility' },
  { id: 'imx', name: 'Immutable X', symbol: 'IMX', decimals: 18, contractAddress: '0x3B3B31E7Ae5FAD9b6b3D9c6D71e02807D4C1eeb3', blockchainId: 'ethereum', logoUrl: 'https://assets.coingecko.com/coins/images/17233/small/immutableX-symbol-BLK-RGB.png', priceUsd: 2, isStablecoin: false, isActive: true, category: 'utility' },
  { id: 'blur', name: 'Blur', symbol: 'BLUR', decimals: 18, contractAddress: '0x5283D6D17D74a11F5F20B1A93C8E9b4C57A5aE0b', blockchainId: 'ethereum', logoUrl: 'https://assets.coingecko.com/coins/images/28453/small/blur.png', priceUsd: 0.5, isStablecoin: false, isActive: true, category: 'utility' },
  { id: 'pepe', name: 'Pepe', symbol: 'PEPE', decimals: 18, contractAddress: '0x6982508145454Ce6d95762c659D3A2B6Ac2eB7C', blockchainId: 'ethereum', logoUrl: 'https://assets.coingecko.com/coins/images/29850/small/pepe-token.jpeg', priceUsd: 0.000001, isStablecoin: false, isActive: true, category: 'memecoin' },
  { id: 'shib', name: 'Shiba Inu', symbol: 'SHIB', decimals: 18, contractAddress: '0x95aD61b0a150d79219dCF64E1E6C01fBFb1dAdF4', blockchainId: 'ethereum', logoUrl: 'https://assets.coingecko.com/coins/images/11939/small/shiba.png', priceUsd: 0.00002, isStablecoin: false, isActive: true, category: 'memecoin' },
  { id: 'doge', name: 'Dogecoin', symbol: 'DOGE', decimals: 8, contractAddress: null, blockchainId: 'dogecoin', logoUrl: 'https://assets.coingecko.com/coins/images/5/small/dogecoin.png', priceUsd: 0.15, isStablecoin: false, isActive: true, category: 'memecoin' },
  { id: 'xrp', name: 'XRP', symbol: 'XRP', decimals: 6, contractAddress: null, blockchainId: 'ripple', logoUrl: 'https://assets.coingecko.com/coins/images/44/small/xrp-symbol-white-128.png', priceUsd: 0.6, isStablecoin: false, isActive: true, category: 'native' },
  { id: 'ada', name: 'Cardano', symbol: 'ADA', decimals: 6, contractAddress: null, blockchainId: 'cardano', logoUrl: 'https://assets.coingecko.com/coins/images/975/small/cardano.png', priceUsd: 0.45, isStablecoin: false, isActive: true, category: 'native' },
  { id: 'dot', name: 'Polkadot', symbol: 'DOT', decimals: 10, contractAddress: null, blockchainId: 'polkadot', logoUrl: 'https://assets.coingecko.com/coins/images/12171/small/polkadot.png', priceUsd: 7, isStablecoin: false, isActive: true, category: 'native' },
  { id: 'ltc', name: 'Litecoin', symbol: 'LTC', decimals: 8, contractAddress: null, blockchainId: 'litecoin', logoUrl: 'https://assets.coingecko.com/coins/images/2/small/litecoin.png', priceUsd: 85, isStablecoin: false, isActive: true, category: 'native' },
  { id: 'trx', name: 'TRON', symbol: 'TRX', decimals: 6, contractAddress: null, blockchainId: 'tron', logoUrl: 'https://assets.coingecko.com/coins/images/1094/small/tron-logo.png', priceUsd: 0.12, isStablecoin: false, isActive: true, category: 'native' },
];

export default topTokens;