import { ethers } from 'ethers';
import * as bip39 from 'bip39';

// Types for multi-chain wallet
export interface WalletInfo {
  address: string;
  privateKey: string;
  publicKey: string;
  chainType: 'evm' | 'solana' | 'cosmos' | 'ton' | 'aptos';
  chainId: number;
  chainName: string;
  balance: string;
  balanceUSD: number;
}

export interface MasterWallet {
  seedPhrase: string;
  wallets: Record<string, WalletInfo>;
  createdAt: Date;
  backupCode: string;
}

export interface TransactionRequest {
  from: string;
  to: string;
  amount: string;
  token?: string;
  chainId: number;
  gasPrice?: string;
  gasLimit?: string;
  data?: string;
}

export interface TransactionResult {
  hash: string;
  status: 'pending' | 'confirmed' | 'failed';
  blockNumber?: number;
  gasUsed?: string;
}

export interface TokenInfo {
  address: string;
  symbol: string;
  name: string;
  decimals: number;
  logoURI: string;
  balance: string;
  balanceUSD: number;
  chainId: number;
  priceUSD: number;
}

export interface LaunchpadProject {
  id: string;
  name: string;
  description: string;
  tokenSymbol: string;
  tokenAddress: string;
  totalSupply: string;
  price: string;
  minPurchase: string;
  maxPurchase: string;
  startTime: Date;
  endTime: Date;
  status: 'upcoming' | 'active' | 'completed';
  raisedAmount: string;
  softCap: string;
  hardCap: string;
  website: string;
  whitepaper: string;
  logo: string;
}

export interface FeeConfig {
  withdrawFeePercent: number;
  swapFeePercent: number;
  transactionFeePercent: number;
  masterWalletAddress: string;
}

// EVM BIP44 path: m/44'/60'/0'/0/0
const EVM_DERIVATION_PATH = "m/44'/60'/0'/0/0";

// Generate wallet from seed phrase for EVM chains
export async function generateEVMMnemonicWallet(
  mnemonic: string,
  chainId: number
): Promise<WalletInfo> {
  try {
    const wallet = ethers.Wallet.fromMnemonic(mnemonic, EVM_DERIVATION_PATH);
    const provider = new ethers.providers.JsonRpcProvider(getRPCForChain(chainId));
    
    let balance = '0';
    try {
      const bal = await provider.getBalance(wallet.address);
      balance = ethers.utils.formatEther(bal);
    } catch (e) {
      console.log('Could not fetch balance');
    }

    const chainInfo = getChainInfo(chainId);
    
    return {
      address: wallet.address,
      privateKey: wallet.privateKey,
      publicKey: wallet.publicKey,
      chainType: 'evm',
      chainId,
      chainName: chainInfo?.name || 'Unknown',
      balance: ethers.utils.formatEther(balance),
      balanceUSD: 0 // Would need price feed
    };
  } catch (error) {
    throw new Error(`Failed to generate EVM wallet: ${error}`);
  }
}

// Generate wallet for Solana
export async function generateSolanaWallet(mnemonic: string): Promise<WalletInfo> {
  try {
    // For Solana, we'd use @solana/web3.js in a real implementation
    // This is a simplified version
    const seed = bip39.mnemonicToSeedSync(mnemonic);
    const keyPair = {
      publicKey: Buffer.from(seed.slice(0, 32)).toString('hex'),
      secretKey: Buffer.from(seed.slice(0, 64)).toString('hex')
    };
    
    return {
      address: keyPair.publicKey.slice(0, 44),
      privateKey: keyPair.secretKey,
      publicKey: keyPair.publicKey,
      chainType: 'solana',
      chainId: 101,
      chainName: 'Solana',
      balance: '0',
      balanceUSD: 0
    };
  } catch (error) {
    throw new Error(`Failed to generate Solana wallet: ${error}`);
  }
}

// Generate wallet for Cosmos chains
export async function generateCosmosWallet(mnemonic: string): Promise<WalletInfo> {
  try {
    const seed = bip39.mnemonicToSeedSync(mnemonic);
    // Simplified - in production would use cosmos-sdk
    const address = Buffer.from(seed.slice(0, 32)).toString('hex').slice(0, 44);
    
    return {
      address,
      privateKey: Buffer.from(seed.slice(0, 32)).toString('hex'),
      publicKey: Buffer.from(seed.slice(0, 32)).toString('hex'),
      chainType: 'cosmos',
      chainId: 1,
      chainName: 'Cosmos',
      balance: '0',
      balanceUSD: 0
    };
  } catch (error) {
    throw new Error(`Failed to generate Cosmos wallet: ${error}`);
  }
}

// Generate all wallets for a seed phrase
export async function generateAllWallets(
  mnemonic: string,
  chainIds: number[] = [1, 137, 42161, 10, 8453, 56, 43114, 250, 101, 1]
): Promise<Record<string, WalletInfo>> {
  const wallets: Record<string, WalletInfo> = {};
  
  // Generate EVM wallets for each chain
  for (const chainId of chainIds) {
    try {
      const wallet = await generateEVMMnemonicWallet(mnemonic, chainId);
      wallets[`evm_${chainId}`] = wallet;
    } catch (error) {
      console.error(`Failed to generate wallet for chain ${chainId}:`, error);
    }
  }

  // Generate Solana wallet
  try {
    const solWallet = await generateSolanaWallet(mnemonic);
    wallets['solana_101'] = solWallet;
  } catch (error) {
    console.error('Failed to generate Solana wallet:', error);
  }

  // Generate Cosmos wallet
  try {
    const cosmosWallet = await generateCosmosWallet(mnemonic);
    wallets['cosmos_1'] = cosmosWallet;
  } catch (error) {
    console.error('Failed to generate Cosmos wallet:', error);
  }

  return wallets;
}

// Get RPC URL for chain
function getRPCForChain(chainId: number): string {
  const rpcUrls: Record<number, string> = {
    1: 'https://eth.llamarpc.com',
    137: 'https://polygon.llamarpc.com',
    42161: 'https://arb1.arbitrum.io/rpc',
    10: 'https://mainnet.optimism.io',
    8453: 'https://mainnet.base.org',
    56: 'https://bsc-dataseed.binance.org',
    43114: 'https://api.avax.network/ext/bc/C/rpc',
    250: 'https://rpc.fantom.network',
    324: 'https://mainnet.era.zksync.io',
    59144: 'https://rpc.linea.build',
    5000: 'https://rpc.mantle.xyz',
    81457: 'https://rpc.blast.io',
    534352: 'https://rpc.scroll.io',
    100: 'https://rpc.gnosischain.com',
    42220: 'https://forno.celo.org',
    2222: 'https://evm.kava.io',
    1284: 'https://rpc.api.moonbeam.network',
    592: 'https://rpc.astar.network',
  };
  return rpcUrls[chainId] || 'https://eth.llamarpc.com';
}

// Get chain info
function getChainInfo(chainId: number): { name: string; symbol: string; type: string } | undefined {
  const chains: Record<number, { name: string; symbol: string; type: string }> = {
    1: { name: 'Ethereum', symbol: 'ETH', type: 'evm' },
    137: { name: 'Polygon', symbol: 'MATIC', type: 'evm' },
    42161: { name: 'Arbitrum One', symbol: 'ETH', type: 'evm' },
    10: { name: 'Optimism', symbol: 'ETH', type: 'evm' },
    8453: { name: 'Base', symbol: 'ETH', type: 'evm' },
    56: { name: 'BNB Smart Chain', symbol: 'BNB', type: 'evm' },
    43114: { name: 'Avalanche', symbol: 'AVAX', type: 'evm' },
    250: { name: 'Fantom', symbol: 'FTM', type: 'evm' },
    324: { name: 'zkSync Era', symbol: 'ETH', type: 'evm' },
    59144: { name: 'Linea', symbol: 'ETH', type: 'evm' },
    5000: { name: 'Mantle', symbol: 'MNT', type: 'evm' },
    81457: { name: 'Blast', symbol: 'ETH', type: 'evm' },
    101: { name: 'Solana', symbol: 'SOL', type: 'solana' },
  };
  return chains[chainId];
}

// Send transaction on EVM chain
export async function sendEVMMnemonicTransaction(
  mnemonic: string,
  to: string,
  amount: string,
  chainId: number,
  tokenAddress?: string
): Promise<TransactionResult> {
  try {
    const wallet = ethers.Wallet.fromMnemonic(mnemonic, EVM_DERIVATION_PATH);
    const provider = new ethers.providers.JsonRpcProvider(getRPCForChain(chainId));
    const signer = wallet.connect(provider);

    let tx;
    if (tokenAddress && tokenAddress !== '0x0000000000000000000000000000000000000000') {
      // Token transfer
      const token = new ethers.Contract(
        tokenAddress,
        ['function transfer(address to, uint256 amount) returns (bool)'],
        signer
      );
      const decimals = 18; // Would fetch from token contract
      const amountWei = ethers.utils.parseUnits(amount, decimals);
      tx = await token.transfer(to, amountWei);
    } else {
      // Native ETH transfer
      tx = await signer.sendTransaction({
        to,
        value: ethers.utils.parseEther(amount)
      });
    }

    const receipt = await tx.wait();
    
    return {
      hash: tx.hash,
      status: receipt.status === 1 ? 'confirmed' : 'failed',
      blockNumber: receipt.blockNumber,
      gasUsed: receipt.gasUsed.toString()
    };
  } catch (error) {
    throw new Error(`Transaction failed: ${error}`);
  }
}

// Get token balance for EVM chain
export async function getEVMMnemonicTokenBalance(
  mnemonic: string,
  tokenAddress: string,
  chainId: number
): Promise<string> {
  try {
    const wallet = ethers.Wallet.fromMnemonic(mnemonic, EVM_DERIVATION_PATH);
    const provider = new ethers.providers.JsonRpcProvider(getRPCForChain(chainId));

    if (tokenAddress === '0x0000000000000000000000000000000000000000') {
      // Native balance
      const balance = await provider.getBalance(wallet.address);
      return ethers.utils.formatEther(balance);
    } else {
      // Token balance
      const token = new ethers.Contract(
        tokenAddress,
        ['function balanceOf(address owner) view returns (uint256)'],
        provider
      );
      const balance = await token.balanceOf(wallet.address);
      return ethers.utils.formatUnits(balance, 18);
    }
  } catch (error) {
    return '0';
  }
}

// Swap tokens on EVM chain (simplified - would integrate with aggregators)
export async function swapEVMTokens(
  mnemonic: string,
  fromToken: string,
  toToken: string,
  amount: string,
  chainId: number,
  slippage: number = 0.5
): Promise<TransactionResult> {
  try {
    // In production, this would use 1inch, Uniswap, etc.
    // Simplified implementation
    const wallet = ethers.Wallet.fromMnemonic(mnemonic, EVM_DERIVATION_PATH);
    const provider = new ethers.providers.JsonRpcProvider(getRPCForChain(chainId));
    const signer = wallet.connect(provider);

    // This is a mock - real implementation would call DEX router
    const tx = await signer.sendTransaction({
      to: wallet.address, // Would be router address
      value: ethers.utils.parseEther(amount)
    });

    const receipt = await tx.wait();
    
    return {
      hash: tx.hash,
      status: receipt.status === 1 ? 'confirmed' : 'failed',
      blockNumber: receipt.blockNumber,
      gasUsed: receipt.gasUsed.toString()
    };
  } catch (error) {
    throw new Error(`Swap failed: ${error}`);
  }
}

// Generate backup code from seed
export function generateBackupCode(seedPhrase: string): string {
  const hash = ethers.utils.keccak256(ethers.utils.toUtf8Bytes(seedPhrase));
  return hash.slice(0, 16).toUpperCase();
}

// Validate seed phrase
export function validateMnemonic(mnemonic: string): boolean {
  return bip39.validateMnemonic(mnemonic);
}

// Generate new random seed phrase
export function generateMnemonic(): string {
  return bip39.generateMnemonic(128); // 12 words
}

// Format address for display
export function formatAddress(address: string): string {
  if (!address) return '';
  return `${address.slice(0, 6)}...${address.slice(-4)}`;
}

// Get explorer URL for transaction
export function getExplorerUrl(chainId: number, txHash: string): string {
  const explorers: Record<number, string> = {
    1: `https://etherscan.io/tx/${txHash}`,
    137: `https://polygonscan.com/tx/${txHash}`,
    42161: `https://arbiscan.io/tx/${txHash}`,
    10: `https://optimistic.etherscan.io/tx/${txHash}`,
    8453: `https://basescan.org/tx/${txHash}`,
    56: `https://bscscan.com/tx/${txHash}`,
    43114: `https://snowtrace.io/tx/${txHash}`,
    250: `https://ftmscan.com/tx/${txHash}`,
  };
  return explorers[chainId] || `https://etherscan.io/tx/${txHash}`;
}

// Get all supported chains
export function getSupportedChains(): Array<{id: number; name: string; symbol: string; type: string}> {
  return [
    { id: 1, name: 'Ethereum', symbol: 'ETH', type: 'evm' },
    { id: 137, name: 'Polygon', symbol: 'MATIC', type: 'evm' },
    { id: 42161, name: 'Arbitrum One', symbol: 'ETH', type: 'evm' },
    { id: 10, name: 'Optimism', symbol: 'ETH', type: 'evm' },
    { id: 8453, name: 'Base', symbol: 'ETH', type: 'evm' },
    { id: 56, name: 'BNB Smart Chain', symbol: 'BNB', type: 'evm' },
    { id: 43114, name: 'Avalanche', symbol: 'AVAX', type: 'evm' },
    { id: 250, name: 'Fantom', symbol: 'FTM', type: 'evm' },
    { id: 324, name: 'zkSync Era', symbol: 'ETH', type: 'evm' },
    { id: 59144, name: 'Linea', symbol: 'ETH', type: 'evm' },
    { id: 5000, name: 'Mantle', symbol: 'MNT', type: 'evm' },
    { id: 81457, name: 'Blast', symbol: 'ETH', type: 'evm' },
    { id: 534352, name: 'Scroll', symbol: 'ETH', type: 'evm' },
    { id: 100, name: 'Gnosis', symbol: 'XDAI', type: 'evm' },
    { id: 42220, name: 'Celo', symbol: 'CELO', type: 'evm' },
    { id: 2222, name: 'Kava', symbol: 'KAVA', type: 'evm' },
    { id: 1284, name: 'Moonbeam', symbol: 'GLMR', type: 'evm' },
    { id: 592, name: 'Astar', symbol: 'ASTR', type: 'evm' },
    { id: 101, name: 'Solana', symbol: 'SOL', type: 'solana' },
    { id: 1, name: 'Cosmos', symbol: 'ATOM', type: 'cosmos' },
    { id: 1, name: 'Aptos', symbol: 'APT', type: 'aptos' },
    { id: 1, name: 'TON', symbol: 'TON', type: 'ton' },
    { id: 1, name: 'Sei', symbol: 'SEI', type: 'cosmos' },
    { id: 1, name: 'Injective', symbol: 'INJ', type: 'cosmos' },
    { id: 1, name: 'Sui', symbol: 'SUI', type: 'aptos' },
    { id: 1, name: 'NEAR', symbol: 'NEAR', type: 'near' },
    { id: 25, name: 'Cronos', symbol: 'CRO', type: 'evm' },
    { id: 42170, name: 'Arbitrum Nova', symbol: 'ETH', type: 'evm' },
    { id: 1116, name: 'Core', symbol: 'CORE', type: 'evm' },
    { id: 7700, name: 'Canto', symbol: 'CANTO', type: 'evm' },
    { id: 8217, name: 'Klaytn', symbol: 'KLAY', type: 'evm' },
    { id: 2020, name: 'Ronin', symbol: 'RON', type: 'evm' },
    { id: 252, name: 'Fraxtal', symbol: 'FRX', type: 'evm' },
    { id: 34443, name: 'Mode', symbol: 'MOD', type: 'evm' },
  ];
}

// Get popular tokens
export function getPopularTokens(): Array<{
  symbol: string;
  name: string;
  address: string;
  decimals: number;
  chainId: number;
  logoURI: string;
}> {
  return [
    { symbol: 'ETH', name: 'Ethereum', address: '0x0000000000000000000000000000000000000000', decimals: 18, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/279/small/ethereum.png' },
    { symbol: 'BTC', name: 'Bitcoin', address: '0x2260FAC5E5542a773Aa44fBCfeDf7C193bc2C599', decimals: 8, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/7598/small/wrapped_bitcoin_wbtc.png' },
    { symbol: 'USDC', name: 'USD Coin', address: '0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48', decimals: 6, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/6319/small/usdc.png' },
    { symbol: 'USDT', name: 'Tether USD', address: '0xdAC17F958D2ee523a2206206994597C13D831ec7', decimals: 6, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/325/small/Tether.png' },
    { symbol: 'BNB', name: 'BNB', address: '0x0000000000000000000000000000000000000000', decimals: 18, chainId: 56, logoURI: 'https://assets.coingecko.com/coins/images/825/small/bnb-icon2_2x.png' },
    { symbol: 'MATIC', name: 'Polygon', address: '0x7D1AfA7B718fb893dB30A3aBc0Cfc608AaCfeBB0', decimals: 18, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/4713/small/polygon.png' },
    { symbol: 'AVAX', name: 'Avalanche', address: '0x0000000000000000000000000000000000000000', decimals: 18, chainId: 43114, logoURI: 'https://assets.coingecko.com/coins/images/12559/small/Avalanche_Circle_RedWhite_Trans.png' },
    { symbol: 'SOL', name: 'Solana', address: 'So11111111111111111111111111111111111111112', decimals: 9, chainId: 101, logoURI: 'https://assets.coingecko.com/coins/images/4128/small/solana.png' },
    { symbol: 'LINK', name: 'Chainlink', address: '0x514910771AF9Ca656af840dff83E8264EcF986CA', decimals: 18, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/877/small/chainlink-new-logo.png' },
    { symbol: 'UNI', name: 'Uniswap', address: '0x1f9840a85d5aF5bf1D1762F925BDADdC4201F984', decimals: 18, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/12504/small/uniswap-uni.png' },
    { symbol: 'AAVE', name: 'Aave', address: '0x7Fc66500c84A76Ad7e9c93437bFc5Ac33E2DDaE9', decimals: 18, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/12645/small/AAVE.png' },
    { symbol: 'DOT', name: 'Polkadot', address: '0x0000000000000000000000000000000000000000', decimals: 10, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/12171/small/polkadot.png' },
    { symbol: 'ATOM', name: 'Cosmos', address: 'uatom', decimals: 6, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/1481/small/cosmos_hub.png' },
    { symbol: 'LTC', name: 'Litecoin', address: '0x0000000000000000000000000000000000000000', decimals: 8, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/2/small/litecoin.png' },
    { symbol: 'XRP', name: 'XRP', address: '0x0000000000000000000000000000000000000000', decimals: 6, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/44/small/xrp-symbol-white-128.png' },
    { symbol: 'DOGE', name: 'Dogecoin', address: '0x0000000000000000000000000000000000000000', decimals: 8, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/5/small/dogecoin.png' },
    { symbol: 'TRX', name: 'TRON', address: '0x0000000000000000000000000000000000000000', decimals: 6, chainId: 728126428, logoURI: 'https://assets.coingecko.com/coins/images/1094/small/tron-logo.png' },
    { symbol: 'APT', name: 'Aptos', address: '0x1::aptos_coin::AptosCoin', decimals: 8, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/26455/small/aptos_round.png' },
    { symbol: 'ARB', name: 'Arbitrum', address: '0x912CE59144191C1204E64559fe8253a0e49E6548', decimals: 18, chainId: 42161, logoURI: 'https://assets.coingecko.com/coins/images/16547/small/photo_2023-03-29_21.47.00.jpeg' },
    { symbol: 'OP', name: 'Optimism', address: '0x4200000000000000000000000000000000000042', decimals: 18, chainId: 10, logoURI: 'https://assets.coingecko.com/coins/images/25244/small/Optimism.png' },
    { symbol: 'PAXG', name: 'Paxos Gold', address: '0xd4CA20A32032996f7c03B32A78c54B4899C30C9A', decimals: 18, chainId: 56, logoURI: 'https://assets.coingecko.com/coins/images/11651/small/pax_gold.png' },
    { symbol: 'TON', name: 'Toncoin', address: '0:0000000000000000000000000000000000000000000000000000000000000000', decimals: 9, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/17980/small/ton_symbol.png' },
  ];
}
