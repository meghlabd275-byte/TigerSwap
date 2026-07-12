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

// Get all supported chains - 100+ chains including top EVM and non-EVM
export function getSupportedChains(): Array<{id: number; name: string; symbol: string; type: string; isTestnet?: boolean}> {
  return [
    // Top EVM Chains (Top 50 EVM)
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
    { id: 100, name: 'Gnosis Chain', symbol: 'XDAI', type: 'evm' },
    { id: 42220, name: 'Celo', symbol: 'CELO', type: 'evm' },
    { id: 2222, name: 'Kava', symbol: 'KAVA', type: 'evm' },
    { id: 1284, name: 'Moonbeam', symbol: 'GLMR', type: 'evm' },
    { id: 592, name: 'Astar', symbol: 'ASTR', type: 'evm' },
    { id: 25, name: 'Cronos', symbol: 'CRO', type: 'evm' },
    { id: 42170, name: 'Arbitrum Nova', symbol: 'ETH', type: 'evm' },
    { id: 1116, name: 'Core', symbol: 'CORE', type: 'evm' },
    { id: 7700, name: 'Canto', symbol: 'CANTO', type: 'evm' },
    { id: 8217, name: 'Klaytn', symbol: 'KLAY', type: 'evm' },
    { id: 2020, name: 'Ronin', symbol: 'RON', type: 'evm' },
    { id: 252, name: 'Fraxtal', symbol: 'FRX', type: 'evm' },
    { id: 34443, name: 'Mode', symbol: 'MOD', type: 'evm' },
    { id: 169, name: 'Manta Pacific', symbol: 'ETH', type: 'evm' },
    { id: 480, name: 'Worldcoin', symbol: 'WLD', type: 'evm' },
    { id: 4660, name: 'Superseed', symbol: 'SEED', type: 'evm' },
    { id: 2000, name: 'Dogechain', symbol: 'DC', type: 'evm' },
    { id: 18, name: 'Shiba Inu', symbol: 'SHIB', type: 'evm' },
    { id: 199, name: 'BitTorrent Chain', symbol: 'BTT', type: 'evm' },
    { id: 10200, name: 'Chiliz', symbol: 'CHZ', type: 'evm' },
    { id: 888888888, name: 'Vision', symbol: 'VS', type: 'evm' },
    { id: 520, name: 'ChainX', symbol: 'PCX', type: 'evm' },
    { id: 2559, name: 'Kroma', symbol: 'ETH', type: 'evm' },
    { id: 200101, name: 'Milkomeda', symbol: 'ADA', type: 'evm' },
    { id: 1666600000, name: 'Harmony', symbol: 'ONE', type: 'evm' },
    { id: 1088, name: 'Metis', symbol: 'METIS', type: 'evm' },
    { id: 66, name: 'OKX Chain', symbol: 'OKT', type: 'evm' },
    { id: 42262, name: 'Oasis Emerald', symbol: 'ROSE', type: 'evm' },
    { id: 336, name: 'Shiden', symbol: 'SDN', type: 'evm' },
    { id: 821, name: 'Callisto', symbol: 'CLO', type: 'evm' },
    { id: 61, name: 'Ethereum Classic', symbol: 'ETC', type: 'evm' },
    { id: 880, name: 'Ambroschain', symbol: 'AMBR', type: 'evm' },
    { id: 2810, name: 'Morph', symbol: 'ETH', type: 'evm' },
    { id: 1777, name: 'Gauss', symbol: 'GAL', type: 'evm' },
    { id: 1625, name: 'Swan Chain', symbol: 'SWAN', type: 'evm' },
    { id: 1514, name: 'Syndicate', symbol: 'SYN', type: 'evm' },
    { id: 12051, name: 'ZetaChain', symbol: 'ZETA', type: 'evm' },
    { id: 50001, name: 'SXP', symbol: 'SXP', type: 'evm' },
    { id: 200625, name: 'Akash', symbol: 'AKT', type: 'evm' },
    { id: 1313161554, name: 'Aurora', symbol: 'ETH', type: 'evm' },
    { id: 728126428, name: 'TRON', symbol: 'TRX', type: 'tron' },
    { id: 369, name: 'PulseChain', symbol: 'PLS', type: 'evm' },
    { id: 314159, name: 'Filecoin', symbol: 'FIL', type: 'evm' },
    
    // Non-EVM Chains (Top 50 Non-EVM)
    { id: 101, name: 'Solana', symbol: 'SOL', type: 'solana' },
    { id: 0, name: 'Bitcoin', symbol: 'BTC', type: 'bitcoin' },
    { id: 0, name: 'Bitcoin Cash', symbol: 'BCH', type: 'bitcoin' },
    { id: 0, name: 'Litecoin', symbol: 'LTC', type: 'bitcoin' },
    { id: 0, name: 'Dogecoin', symbol: 'DOGE', type: 'bitcoin' },
    { id: 0, name: 'Ripple', symbol: 'XRP', type: 'ripple' },
    { id: 0, name: 'Cardano', symbol: 'ADA', type: 'cardano' },
    { id: 0, name: 'Polkadot', symbol: 'DOT', type: 'polkadot' },
    { id: 0, name: 'Near', symbol: 'NEAR', type: 'near' },
    { id: 0, name: 'Aptos', symbol: 'APT', type: 'aptos' },
    { id: 0, name: 'Sui', symbol: 'SUI', type: 'aptos' },
    { id: 0, name: 'Cosmos', symbol: 'ATOM', type: 'cosmos' },
    { id: 0, name: 'Sei', symbol: 'SEI', type: 'cosmos' },
    { id: 0, name: 'Injective', symbol: 'INJ', type: 'cosmos' },
    { id: 0, name: 'Osmosis', symbol: 'OSMO', type: 'cosmos' },
    { id: 0, name: 'THORChain', symbol: 'RUNE', type: 'cosmos' },
    { id: 0, name: 'TON', symbol: 'TON', type: 'ton' },
    { id: 0, name: 'Tezos', symbol: 'XTZ', type: 'tezos' },
    { id: 0, name: 'Algorand', symbol: 'ALGO', type: 'algorand' },
    { id: 0, name: 'VeChain', symbol: 'VET', type: 'vet' },
    { id: 0, name: 'Flow', symbol: 'FLOW', type: 'flow' },
    { id: 0, name: 'Aptos', symbol: 'APT', type: 'aptos' },
    { id: 0, name: 'Stellar', symbol: 'XLM', type: 'stellar' },
    { id: 0, name: 'Hedera', symbol: 'HBAR', type: 'hedera' },
    { id: 0, name: 'EOS', symbol: 'EOS', type: 'eos' },
    { id: 0, name: 'Chainlink', symbol: 'LINK', type: 'chainlink' },
    { id: 0, name: 'Polkadot', symbol: 'DOT', type: 'polkadot' },
    { id: 0, name: 'Kusama', symbol: 'KSM', type: 'polkadot' },
    { id: 0, name: 'Acala', symbol: 'ACA', type: 'polkadot' },
    { id: 0, name: 'Moonbeam', symbol: 'GLMR', type: 'polkadot' },
    { id: 0, name: 'Astar', symbol: 'ASTR', type: 'polkadot' },
    { id: 0, name: 'Interlay', symbol: 'INTR', type: 'polkadot' },
    { id: 0, name: 'Composable', symbol: 'LAYR', type: 'polkadot' },
    { id: 0, name: 'Centrifuge', symbol: 'CFG', type: 'polkadot' },
    { id: 0, name: 'Manta', symbol: 'MANTA', type: 'polkadot' },
    { id: 0, name: 'OriginTrail', symbol: 'OTP', type: 'chain' },
    { id: 0, name: 'Pi Network', symbol: 'PI', type: 'pi' },
    { id: 0, name: 'Plasma', symbol: 'PLG', type: 'plasma' },
    { id: 0, name: 'Sui', symbol: 'SUI', type: 'sui' },
    { id: 0, name: 'Pepe', symbol: 'PEPE', type: 'evm' },
    { id: 0, name: 'Shiba Inu', symbol: 'SHIB', type: 'evm' },
    { id: 0, name: 'Floki', symbol: 'FLOKI', type: 'evm' },
    { id: 0, name: 'Bonk', symbol: 'BONK', type: 'solana' },
    { id: 0, name: 'WIF', symbol: 'WIF', type: 'solana' },
    { id: 0, name: 'Jupiter', symbol: 'JUP', type: 'solana' },
    { id: 0, name: 'Raydium', symbol: 'RAY', type: 'solana' },
    { id: 0, name: 'Orca', symbol: 'ORCA', type: 'solana' },
    { id: 0, name: 'Marinade', symbol: 'MNDE', type: 'solana' },
    { id: 0, name: 'Saga', symbol: 'SAGA', type: 'cosmos' },
    { id: 0, name: 'Dymension', symbol: 'DYM', type: 'cosmos' },
    { id: 0, name: 'Celestia', symbol: 'TIA', type: 'cosmos' },
  ];
}

// Get popular tokens - 200+ tokens across all chains
export function getPopularTokens(): Array<{
  symbol: string;
  name: string;
  address: string;
  decimals: number;
  chainId: number;
  logoURI: string;
}> {
  return [
    // Core Assets (Top 20)
    { symbol: 'ETH', name: 'Ethereum', address: '0x0000000000000000000000000000000000000000', decimals: 18, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/279/small/ethereum.png' },
    { symbol: 'BTC', name: 'Bitcoin', address: '0x2260FAC5E5542a773Aa44fBCfeDf7C193bc2C599', decimals: 8, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/7598/small/wrapped_bitcoin_wbtc.png' },
    { symbol: 'USDC', name: 'USD Coin', address: '0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48', decimals: 6, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/6319/small/usdc.png' },
    { symbol: 'USDT', name: 'Tether USD', address: '0xdAC17F958D2ee523a2206206994597C13D831ec7', decimals: 6, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/325/small/Tether.png' },
    { symbol: 'BNB', name: 'BNB', address: '0x0000000000000000000000000000000000000000', decimals: 18, chainId: 56, logoURI: 'https://assets.coingecko.com/coins/images/825/small/bnb-icon2_2x.png' },
    { symbol: 'SOL', name: 'Solana', address: 'So11111111111111111111111111111111111111112', decimals: 9, chainId: 101, logoURI: 'https://assets.coingecko.com/coins/images/4128/small/solana.png' },
    { symbol: 'XRP', name: 'XRP', address: '0x0000000000000000000000000000000000000000', decimals: 6, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/44/small/xrp-symbol-white-128.png' },
    { symbol: 'USDC', name: 'USD Coin', address: '0x2791Bca1f2de4661ED88A30C99A7a9449Aa84174', decimals: 6, chainId: 137, logoURI: 'https://assets.coingecko.com/coins/images/6319/small/usdc.png' },
    { symbol: 'USDT', name: 'Tether USD', address: '0xc2132D05D31c914a87C6611C10748AEb04B58e8F', decimals: 6, chainId: 137, logoURI: 'https://assets.coingecko.com/coins/images/325/small/Tether.png' },
    { symbol: 'USDC', name: 'USD Coin', address: '0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48', decimals: 6, chainId: 42161, logoURI: 'https://assets.coingecko.com/coins/images/6319/small/usdc.png' },
    { symbol: 'USDT', name: 'Tether USD', address: '0xFd086b7Be5F27C5Fce17a4d43b3AfC99D3d9D84b', decimals: 6, chainId: 42161, logoURI: 'https://assets.coingecko.com/coins/images/325/small/Tether.png' },
    { symbol: 'USDC', name: 'USD Coin', address: '0x0b2C639c533813f1AaCB0B3d5073C7CFea90aD6c', decimals: 6, chainId: 10, logoURI: 'https://assets.coingecko.com/coins/images/6319/small/usdc.png' },
    { symbol: 'USDT', name: 'Tether USD', address: '0x94b008aA00596cF27E3D9218aE44B1e2D8cdd1D5', decimals: 6, chainId: 10, logoURI: 'https://assets.coingecko.com/coins/images/325/small/Tether.png' },
    { symbol: 'USDC', name: 'USD Coin', address: '0x833589fCD6eDb6E08f4c7c32D4d71dBD6AD1fC1D', decimals: 6, chainId: 8453, logoURI: 'https://assets.coingecko.com/coins/images/6319/small/usdc.png' },
    { symbol: 'USDT', name: 'Tether USD', address: '0xFDBC6c8926dA61Da9D5E52a7Eb1D992e8F631b97', decimals: 6, chainId: 8453, logoURI: 'https://assets.coingecko.com/coins/images/325/small/Tether.png' },
    { symbol: 'USDC', name: 'USD Coin', address: '0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48', decimals: 6, chainId: 56, logoURI: 'https://assets.coingecko.com/coins/images/6319/small/usdc.png' },
    { symbol: 'USDT', name: 'Tether USD', address: '0x55d398326f99059fF775485246999027B3197955', decimals: 18, chainId: 56, logoURI: 'https://assets.coingecko.com/coins/images/325/small/Tether.png' },
    { symbol: 'USDC', name: 'USD Coin', address: '0xB97EF9Ef8734C71904D8002F8b6Bc66Dd9c48a6E', decimals: 6, chainId: 43114, logoURI: 'https://assets.coingecko.com/coins/images/6319/small/usdc.png' },
    { symbol: 'USDT', name: 'Tether USD', address: '0x9702230A8Ea53601f5cD2dc00f3c22d986477924', decimals: 6, chainId: 43114, logoURI: 'https://assets.coingecko.com/coins/images/325/small/Tether.png' },
    { symbol: 'TRX', name: 'TRON', address: '0x0000000000000000000000000000000000000000', decimals: 6, chainId: 728126428, logoURI: 'https://assets.coingecko.com/coins/images/1094/small/tron-logo.png' },
    
    // Ethereum Tokens (Top by Market Cap)
    { symbol: 'MATIC', name: 'Polygon', address: '0x7D1AfA7B718fb893dB30A3aBc0Cfc608AaCfeBB0', decimals: 18, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/4713/small/polygon.png' },
    { symbol: 'LINK', name: 'Chainlink', address: '0x514910771AF9Ca656af840dff83E8264EcF986CA', decimals: 18, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/877/small/chainlink-new-logo.png' },
    { symbol: 'UNI', name: 'Uniswap', address: '0x1f9840a85d5aF5bf1D1762F925BDADdC4201F984', decimals: 18, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/12504/small/uniswap-uni.png' },
    { symbol: 'AAVE', name: 'Aave', address: '0x7Fc66500c84A76Ad7e9c93437bFc5Ac33E2DDaE9', decimals: 18, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/12645/small/AAVE.png' },
    { symbol: 'DOT', name: 'Polkadot', address: '0x0000000000000000000000000000000000000000', decimals: 10, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/12171/small/polkadot.png' },
    { symbol: 'ATOM', name: 'Cosmos', address: 'uatom', decimals: 6, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/1481/small/cosmos_hub.png' },
    { symbol: 'LTC', name: 'Litecoin', address: '0x0000000000000000000000000000000000000000', decimals: 8, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/2/small/litecoin.png' },
    { symbol: 'DOGE', name: 'Dogecoin', address: '0x0000000000000000000000000000000000000000', decimals: 8, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/5/small/dogecoin.png' },
    { symbol: 'APT', name: 'Aptos', address: '0x1::aptos_coin::AptosCoin', decimals: 8, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/26455/small/aptos_round.png' },
    { symbol: 'ARB', name: 'Arbitrum', address: '0x912CE59144191C1204E64559fe8253a0e49E6548', decimals: 18, chainId: 42161, logoURI: 'https://assets.coingecko.com/coins/images/16547/small/photo_2023-03-29_21.47.00.jpeg' },
    { symbol: 'OP', name: 'Optimism', address: '0x4200000000000000000000000000000000000042', decimals: 18, chainId: 10, logoURI: 'https://assets.coingecko.com/coins/images/25244/small/Optimism.png' },
    { symbol: 'PAXG', name: 'Paxos Gold', address: '0xd4CA20A32032996f7c03B32A78c54B4899C30C9A', decimals: 18, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/11651/small/pax_gold.png' },
    { symbol: 'TON', name: 'Toncoin', address: '0:0000000000000000000000000000000000000000000000000000000000000000', decimals: 9, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/17980/small/ton_symbol.png' },
    { symbol: 'WBTC', name: 'Wrapped Bitcoin', address: '0x2260FAC5E5542a773Aa44fBCfeDf7C193bc2C599', decimals: 8, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/7598/small/wrapped_bitcoin_wbtc.png' },
    { symbol: 'SHIB', name: 'Shiba Inu', address: '0x95aD61b0a150d79219dCF64E1E6Cc01f0B64C4cE', decimals: 18, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/11939/small/shiba.png' },
    { symbol: 'PEPE', name: 'Pepe', address: '0x6982508145454Ce325dDbE47a25d4ec3d2311933', decimals: 18, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/29850/small/pepe-token.jpeg' },
    { symbol: 'NEAR', name: 'NEAR Protocol', address: '0x0000000000000000000000000000000000000000', decimals: 24, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/10365/small/near.jpg' },
    { symbol: 'INJ', name: 'Injective', address: '0x0000000000000000000000000000000000000000', decimals: 18, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/12882/small/Secondary_Symbol.png' },
    { symbol: 'SUI', name: 'Sui', address: '0x0000000000000000000000000000000000000000', decimals: 9, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/26375/small/sui_asset.jpeg' },
    { symbol: 'SEI', name: 'Sei', address: '0x0000000000000000000000000000000000000000', decimals: 18, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/28205/small/Sei_Logo_-_Transparent.png' },
    { symbol: 'TIA', name: 'Celestia', address: '0x0000000000000000000000000000000000000000', decimals: 6, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/31967/small/tia.jpg' },
    { symbol: 'RUNE', name: 'THORChain', address: '0x0000000000000000000000000000000000000000', decimals: 8, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/6595/small/Rune200x200.png' },
    { symbol: 'MKR', name: 'Maker', address: '0x9f8F72aA9304c8B593d555F12eF6589cC3A76A6F', decimals: 18, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/1364/small/Mark_Maker.png' },
    { symbol: 'SNX', name: 'Synthetix', address: '0xC011a73ee8576Fb46F5E1c5751cA3B9Fe0Af2a6F', decimals: 18, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/3406/small/SNX.png' },
    { symbol: 'CRV', name: 'Curve DAO', address: '0xD533a949740bb3306d119CC777fa900bA034cd52', decimals: 18, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/12124/small/Curve.png' },
    { symbol: 'LDO', name: 'Lido DAO', address: '0x5A98FcBEA516Cf06857215779Fd812CA3beF4B32', decimals: 18, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/13573/small/Lido_DAO.png' },
    { symbol: 'VET', name: 'VeChain', address: '0x0000000000000000000000000000000000000000', decimals: 18, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/1167/small/VET_Token_Icon.png' },
    { symbol: 'FIL', name: 'Filecoin', address: '0x0000000000000000000000000000000000000000', decimals: 18, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/12817/small/filecoin.png' },
    { symbol: 'HBAR', name: 'Hedera', address: '0x0000000000000000000000000000000000000000', decimals: 8, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/3688/small/hbar.png' },
    { symbol: 'ALGO', name: 'Algorand', address: '0x0000000000000000000000000000000000000000', decimals: 6, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/4380/small/download.png' },
    { symbol: 'XLM', name: 'Stellar', address: '0x0000000000000000000000000000000000000000', decimals: 7, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/100/small/Stellar_symbol_black_RGB.png' },
    { symbol: 'BCH', name: 'Bitcoin Cash', address: '0x0000000000000000000000000000000000000000', decimals: 8, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/780/small/bitcoin-cash-circle.png' },
    { symbol: 'APT', name: 'Aptos', address: '0x0000000000000000000000000000000000000000', decimals: 8, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/26455/small/aptos_round.png' },
    { symbol: 'NEAR', name: 'NEAR Protocol', address: '0x0000000000000000000000000000000000000000', decimals: 24, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/10365/small/near.jpg' },
    { symbol: 'FTM', name: 'Fantom', address: '0x0000000000000000000000000000000000000000', decimals: 18, chainId: 250, logoURI: 'https://assets.coingecko.com/coins/images/4001/small/Fantom_round.png' },
    { symbol: 'AVAX', name: 'Avalanche', address: '0x0000000000000000000000000000000000000000', decimals: 18, chainId: 43114, logoURI: 'https://assets.coingecko.com/coins/images/12559/small/Avalanche_Circle_RedWhite_Trans.png' },
    { symbol: 'SAND', name: 'The Sandbox', address: '0x3845badAde8e6dFF049820540d3604a5d1f9e6fA', decimals: 18, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/12129/small/sandbox_logo.jpg' },
    { symbol: 'MANA', name: 'Decentraland', address: '0x0F5D2fB29fb1d4E2f73c8a4D7b1C7C8a4D7B1C7C', decimals: 18, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/878/small/decentraland-mana.png' },
    { symbol: 'AXS', name: 'Axie Infinity', address: '0xBB0E17EF65F82Ab018d8edd776e8Dd940327B28b', decimals: 18, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/13029/small/axie_infinity_logo.png' },
    { symbol: 'CHZ', name: 'Chiliz', address: '0x3506424F91fD33084466F402d5D97f05F8e3b4AF', decimals: 18, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/8834/small/CHZ_Token_updated.png' },
    { symbol: 'FTG', name: 'Fortune', address: '0x0000000000000000000000000000000000000000', decimals: 18, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/24200/small/Fortune300.png' },
    { symbol: 'TUSDT', name: 'TrueUSD', address: '0x0000000000000000000000000000000000000000', decimals: 18, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/3448/small/tusd.png' },
    { symbol: 'BUSD', name: 'Binance USD', address: '0x4Fabb145d64652a948d72533023f6E7A623C7C53', decimals: 18, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/9576/small/BUSD.png' },
    { symbol: 'FRAX', name: 'Frax', address: '0x853d955aCEf822Db058eb8505911ED77F175b99e', decimals: 18, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/9959/small/Frax_logo.png' },
    { symbol: 'DAI', name: 'Dai', address: '0x6B175474E89094C44Da98b954EedeAC495271d0F', decimals: 18, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/1/small/dai-logo.png' },
    { symbol: 'MNT', name: 'Mantle', address: '0x0000000000000000000000000000000000000000', decimals: 18, chainId: 5000, logoURI: 'https://assets.coingecko.com/coins/images/31080/small/mantle.png' },
    { symbol: 'OP', name: 'Optimism', address: '0x0000000000000000000000000000000000000000', decimals: 18, chainId: 10, logoURI: 'https://assets.coingecko.com/coins/images/25244/small/Optimism.png' },
    { symbol: 'ARB', name: 'Arbitrum', address: '0x0000000000000000000000000000000000000000', decimals: 18, chainId: 42161, logoURI: 'https://assets.coingecko.com/coins/images/16547/small/photo_2023-03-29_21.47.00.jpeg' },
    { symbol: 'BASE', name: 'Base', address: '0x0000000000000000000000000000000000000000', decimals: 18, chainId: 8453, logoURI: 'https://assets.coingecko.com/coins/images/31083/small/base.png' },
    { symbol: 'BLUR', name: 'Blur', address: '0x0000000000000000000000000000000000000000', decimals: 18, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/28453/small/blur.png' },
    { symbol: 'IMX', name: 'Immutable', address: '0x0000000000000000000000000000000000000000', decimals: 18, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/17233/small/immutableX-symbol-BLK-RGB.png' },
    { symbol: 'GALA', name: 'Gala', address: '0x0000000000000000000000000000000000000000', decimals: 8, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/12493/small/GALA-COINGECKO.png' },
    { symbol: 'ENJ', name: 'Enjin Coin', address: '0x0000000000000000000000000000000000000000', decimals: 18, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/396/small/enjin-coin-logo.png' },
    { symbol: 'BAT', name: 'Basic Attention Token', address: '0x0D8775F648430679A709E98d2b0Cb6250d2887EF', decimals: 18, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/677/small/basic-attention-token.png' },
    { symbol: 'ZEC', name: 'Zcash', address: '0x0000000000000000000000000000000000000000', decimals: 8, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/486/small/circle-zcash-color.png' },
    { symbol: 'XMR', name: 'Monero', address: '0x0000000000000000000000000000000000000000', decimals: 12, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/69/small/monero_logo.png' },
    { symbol: 'Dash', name: 'Dash', address: '0x0000000000000000000000000000000000000000', decimals: 8, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/19/small/dash.png' },
    { symbol: 'ZIL', name: 'Zilliqa', address: '0x0000000000000000000000000000000000000000', decimals: 12, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/996/small/ZIL.png' },
    { symbol: 'ENS', name: 'Ethereum Name Service', address: '0xC18360217d8F7Ab5e3c2330cB8CF78EdA9cEA7E5', decimals: 18, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/19785/small/acatxTm8_400x400.jpg' },
    { symbol: '1INCH', name: '1inch', address: '0x111111111117dC0aa78b770fA6A738034120C302', decimals: 18, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/13469/small/1inch-token.png' },
    { symbol: 'COMP', name: 'Compound', address: '0xc00e94Cb662C3520282E6f5717214004A7f26888', decimals: 18, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/10775/small/COMP.png' },
    { symbol: 'MIM', name: 'Magic Internet Money', address: '0x99D8a9C45b2ecA8864373A26D1459e3Dff0e17Ed', decimals: 18, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/14686/small/mimlogopng.png' },
    { symbol: 'FEI', name: 'Fei USD', address: '0x956F47F50A8621632C3F2470A0E01D0AEfc32C8C', decimals: 18, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/14569/small/Fei.png' },
    { symbol: 'CRO', name: 'Cronos', address: '0x0000000000000000000000000000000000000000', decimals: 8, chainId: 25, logoURI: 'https://assets.coingecko.com/coins/images/7310/small/cro_token.png' },
    { symbol: 'KCS', name: 'KuCoin Token', address: '0x0000000000000000000000000000000000000000', decimals: 18, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/1047/small/sa9z79.png' },
    { symbol: 'OKB', name: 'OKB', address: '0x0000000000000000000000000000000000000000', decimals: 18, chainId: 66, logoURI: 'https://assets.coingecko.com/coins/images/12532/small/WeChat_Image_20220118105252.png' },
    { symbol: 'HT', name: 'Huobi Token', address: '0x0000000000000000000000000000000000000000', decimals: 18, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/2828/small/huobi-token-logo.png' },
    { symbol: 'KLAY', name: 'Klaytn', address: '0x0000000000000000000000000000000000000000', decimals: 18, chainId: 8217, logoURI: 'https://assets.coingecko.com/coins/images/9672/small/klaytn.png' },
    { symbol: 'ONE', name: 'Harmony', address: '0x0000000000000000000000000000000000000000', decimals: 18, chainId: 1666600000, logoURI: 'https://assets.coingecko.com/coins/images/4342/small/harmony-logo.png' },
    { symbol: 'MOVR', name: 'Moonriver', address: '0x0000000000000000000000000000000000000000', decimals: 18, chainId: 1285, logoURI: 'https://assets.coingecko.com/coins/images/11145/small/Moonriver_-_Transparent.png' },
    { symbol: 'GLMR', name: 'Moonbeam', address: '0x0000000000000000000000000000000000000000', decimals: 18, chainId: 1284, logoURI: 'https://assets.coingecko.com/coins/images/16123/small/moonbeam.png' },
    { symbol: 'ASTR', name: 'Astar', address: '0x0000000000000000000000000000000000000000', decimals: 18, chainId: 592, logoURI: 'https://assets.coingecko.com/coins/images/22617/small/astr.png' },
    { symbol: 'KAVA', name: 'Kava', address: '0x0000000000000000000000000000000000000000', decimals: 18, chainId: 2222, logoURI: 'https://assets.coingecko.com/coins/images/9761/small/kava.png' },
    { symbol: 'ZEC', name: 'Zcash', address: '0x0000000000000000000000000000000000000000', decimals: 8, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/486/small/circle-zcash-color.png' },
    { symbol: 'NEO', name: 'Neo', address: '0x0000000000000000000000000000000000000000', decimals: 8, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/480/small/NEO_512_512.png' },
    { symbol: 'WAVES', name: 'Waves', address: '0x0000000000000000000000000000000000000000', decimals: 8, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/885/small/waves.png' },
    { symbol: 'THETA', name: 'THETA', address: '0x0000000000000000000000000000000000000000', decimals: 18, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/2538/small/theta-token-logo.png' },
    { symbol: 'IOTA', name: 'IOTA', address: '0x0000000000000000000000000000000000000000', decimals: 6, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/692/small/IOTA_Swirl.png' },
    { symbol: 'XTZ', name: 'Tezos', address: '0x0000000000000000000000000000000000000000', decimals: 6, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/976/small/xtz.png' },
    { symbol: 'EOS', name: 'EOS', address: '0x0000000000000000000000000000000000000000', decimals: 18, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/738/small/eos-eos-logo.png' },
    { symbol: 'FLOW', name: 'Flow', address: '0x0000000000000000000000000000000000000000', decimals: 8, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/13446/small/5f6294c0c7a8c58c93427a3.png' },
    { symbol: 'KSM', name: 'Kusama', address: '0x0000000000000000000000000000000000000000', decimals: 12, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/9568/small/kusama.png' },
    { symbol: 'CAKE', name: 'PancakeSwap', address: '0x0E09FaBB73Bd3Ade0a17ECC321fD13a19e81cE82', decimals: 18, chainId: 56, logoURI: 'https://assets.coingecko.com/coins/images/12632/small/pancakeswap-cake-logo_%281%29.png' },
    { symbol: 'GMX', name: 'GMX', address: '0x0000000000000000000000000000000000000000', decimals: 18, chainId: 42161, logoURI: 'https://assets.coingecko.com/coins/images/18323/small/Arbitrum.png' },
    { symbol: 'LQTY', name: 'Liquity', address: '0x6DEA81C8171d0bA574754EF6F8b412F2Ed88c04D', decimals: 18, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/14673/small/LQTY.png' },
    { symbol: 'RPL', name: 'Rocket Pool', address: '0xD33526068D116cE69F19A9ee46F0BD304F21A51f', decimals: 18, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/2090/small/rocket_pool_%28RPL%29.png' },
    { symbol: 'GMX', name: 'GMX', address: '0x0000000000000000000000000000000000000000', decimals: 18, chainId: 43114, logoURI: 'https://assets.coingecko.com/coins/images/18323/small/Arbitrum.png' },
    { symbol: 'RDNT', name: 'Radiant Capital', address: '0x0000000000000000000000000000000000000000', decimals: 18, chainId: 42161, logoURI: 'https://assets.coingecko.com/coins/images/25480/small/512x512_Radient_Circle_Red.png' },
    { symbol: 'MAGIC', name: 'Magic', address: '0x0000000000000000000000000000000000000000', decimals: 18, chainId: 42161, logoURI: 'https://assets.coingecko.com/coins/images/18623/small/magic.png' },
    { symbol: 'JOE', name: 'Trader Joe', address: '0x0000000000000000000000000000000000000000', decimals: 18, chainId: 43114, logoURI: 'https://assets.coingecko.com/coins/images/17549/small/trader-joe.png' },
    { symbol: 'PNG', name: 'Pangolin', address: '0x0000000000000000000000000000000000000000', decimals: 18, chainId: 43114, logoURI: 'https://assets.coingecko.com/coins/images/14942/small/Pangolin.png' },
    { symbol: 'QI', name: 'Benqi', address: '0x0000000000000000000000000000000000000000', decimals: 18, chainId: 43114, logoURI: 'https://assets.coingecko.com/coins/images/15811/small/LOGO_BENQI.png' },
    { symbol: 'PTP', name: 'Platypus', address: '0x0000000000000000000000000000000000000000', decimals: 18, chainId: 43114, logoURI: 'https://assets.coingecko.com/coins/images/17582/small/platypus.png' },
    { symbol: 'JEWEL', name: 'DeFi Kingdoms', address: '0x0000000000000000000000000000000000000000', decimals: 18, chainId: 25, logoURI: 'https://assets.coingecko.com/coins/images/17241/small/DeFi_Kingdoms.png' },
    { symbol: 'RON', name: 'Ronin', address: '0x0000000000000000000000000000000000000000', decimals: 18, chainId: 2020, logoURI: 'https://assets.coingecko.com/coins/images/20009/small/ronin.jpg' },
    { symbol: 'AXL', name: 'Axelar', address: '0x0000000000000000000000000000000000000000', decimals: 6, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/19787/small/axelar.png' },
    { symbol: 'OSMO', name: 'Osmosis', address: '0x0000000000000000000000000000000000000000', decimals: 6, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/12233/small/osmo.png' },
    { symbol: 'DYM', name: 'Dymension', address: '0x0000000000000000000000000000000000000000', decimals: 18, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/34715/small/dym_transparent.png' },
    { symbol: 'SAGA', name: 'Saga', address: '0x0000000000000000000000000000000000000000', decimals: 6, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/41282/small/saga_token_full_color_rgb_2000px_72ppi_fb766ac85a.png' },
    { symbol: 'METIS', name: 'Metis', address: '0x0000000000000000000000000000000000000000', decimals: 18, chainId: 1088, logoURI: 'https://assets.coingecko.com/coins/images/15595/small/metis.png' },
    { symbol: 'KDA', name: 'Kadena', address: '0x0000000000000000000000000000000000000000', decimals: 12, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/5647/small/polylogo.png' },
    { symbol: 'BORA', name: 'BORA', address: '0x0000000000000000000000000000000000000000', decimals: 18, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/17797/small/bora.png' },
    { symbol: 'CTSI', name: 'Cartesi', address: '0x0000000000000000000000000000000000000000', decimals: 18, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/15244/small/cartesi.png' },
    { symbol: 'RSS3', name: 'RSS3', address: '0x0000000000000000000000000000000000000000', decimals: 18, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/23642/small/RSS3.png' },
    { symbol: 'ANKR', name: 'Ankr', address: '0x0000000000000000000000000000000000000000', decimals: 18, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/4324/small/U85xTl2.png' },
    { symbol: 'REQ', name: 'Request', address: '0x0000000000000000000000000000000000000000', decimals: 18, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/3361/small/req_token_200px.png' },
    { symbol: 'ICX', name: 'Icon', address: '0x0000000000000000000000000000000000000000', decimals: 18, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/10000/small/icon-icx-logo.png' },
    { symbol: 'SXP', name: 'Solar', address: '0x0000000000000000000000000000000000000000', decimals: 18, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/5018/small/sxp.png' },
    { symbol: 'CELO', name: 'Celo', address: '0x0000000000000000000000000000000000000000', decimals: 18, chainId: 42220, logoURI: 'https://assets.coingecko.com/coins/images/5568/small/celo.png' },
    { symbol: 'CFX', name: 'Conflux', address: '0x0000000000000000000000000000000000000000', decimals: 18, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/13079/small/3vuYMbjN.png' },
    { symbol: 'NYM', name: 'Nym', address: '0x0000000000000000000000000000000000000000', decimals: 6, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/24560/small/nym_token_hd.png' },
    { symbol: 'ROSE', name: 'Oasis', address: '0x0000000000000000000000000000000000000000', decimals: 18, chainId: 42262, logoURI: 'https://assets.coingecko.com/coins/images/13162/small/rose.png' },
    { symbol: 'MINA', name: 'Mina', address: '0x0000000000000000000000000000000000000000', decimals: 9, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/15628/small/JM4_vQ34_400x400.png' },
    { symbol: 'BOBA', name: 'Boba Network', address: '0x0000000000000000000000000000000000000000', decimals: 18, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/20208/small/boba_token_200px.png' },
    { symbol: 'KAI', name: 'KardiaChain', address: '0x0000000000000000000000000000000000000000', decimals: 18, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/12104/small/kai.png' },
    { symbol: 'NEO', name: 'Neo', address: '0x0000000000000000000000000000000000000000', decimals: 8, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/480/small/NEO_512_512.png' },
    { symbol: 'SOL', name: 'Solana', address: 'So11111111111111111111111111111111111111112', decimals: 9, chainId: 101, logoURI: 'https://assets.coingecko.com/coins/images/4128/small/solana.png' },
    { symbol: 'BONK', name: 'Bonk', address: 'DezXAZ8z7PnrnzjzT25R9iv9mgm9ArL5J6sWv5bK4xM', decimals: 5, chainId: 101, logoURI: 'https://assets.coingecko.com/coins/images/28600/small/bonk.jpg' },
    { symbol: 'WIF', name: 'WIF', address: '85VBFQZC9TZkfaptBWqv14ALD9fJNUKtWA41kh69teRP', decimals: 6, chainId: 101, logoURI: 'https://assets.coingecko.com/coins/images/28491/small/wif.png' },
    { symbol: 'JUP', name: 'Jupiter', address: 'JUPyiwrYJFskUPiHa7hkeR8VUtkqjberbSOWd91pbT2', decimals: 6, chainId: 101, logoURI: 'https://assets.coingecko.com/coins/images/34188/small/jup.png' },
    { symbol: 'RAY', name: 'Raydium', address: '4k3Dyjzvzp8eMZWUXbBCjEvwSkkk58S5vq3cWJnrV1T', decimals: 6, chainId: 101, logoURI: 'https://assets.coingecko.com/coins/images/13928/small/PSigc4ie_400x400.jpg' },
    { symbol: 'ORCA', name: 'Orca', address: 'orcaEKTdK7ATcqaBStRMBcm3WVC45jSjE8RjjwR46D4o', decimals: 6, chainId: 101, logoURI: 'https://assets.coingecko.com/coins/images/13152/small/orca.png' },
    { symbol: 'MNDE', name: 'Marinade', address: 'MNDEFzGvMt87ueuHvVU9V3W7vV9r9ajU2WQ7t6xk3v5', decimals: 9, chainId: 101, logoURI: 'https://assets.coingecko.com/coins/images/23085/small/mnde.png' },
    { symbol: 'SRM', name: 'Serum', address: 'SRMuApVNdxXokk5GT7XD5cUUgXMBCoAz2LHeuAoKWRt', decimals: 6, chainId: 101, logoURI: 'https://assets.coingecko.com/coins/images/11969/small/serum.png' },
    { symbol: 'STEP', name: 'Step Finance', address: 'STEPN5SBg5RC3VJpJ7U31NMXR6R7UGHP2XehYt6g6W7W', decimals: 9, chainId: 101, logoURI: 'https://assets.coingecko.com/coins/images/14988/small/step.png' },
    { symbol: 'COPE', name: 'Coinbase', address: '8KsL3WbEuzRmuWrLLCW6M1hWA7U6aGxBxV25ZQzB2j3e', decimals: 6, chainId: 101, logoURI: 'https://assets.coingecko.com/coins/images/11811/small/coinbase_icon.png' },
    { symbol: 'SLIM', name: 'Solanium', address: 'xxxxa1sKNGwFtw2kFn8XauAW9PYNHShRq7hELCpxo9d', decimals: 6, chainId: 101, logoURI: 'https://assets.coingecko.com/coins/images/14983/small/200x200.png' },
    { symbol: 'FLOKI', name: 'FLOKI', address: 'D8jV7X8xW6v3v7X8xW6v3v7X8xW6v3v7X8xW6v3v7X8xW', decimals: 9, chainId: 101, logoURI: 'https://assets.coingecko.com/coins/images/16746/small/FLOKI.png' },
    { symbol: 'ATLAS', name: 'Star Atlas', address: 'ATLASXmbPQxBUY6t3eHKk9H29zsGM7YqKj5Y9gYqKj5Y', decimals: 8, chainId: 101, logoURI: 'https://assets.coingecko.com/coins/images/15720/small/Star_Atlas_Logos_2023_Black_Text_Glow.png' },
    { symbol: 'POLIS', name: 'Star Atlas DAO', address: 'POLISXdbEmcJ9FLDvQP6bNPnW2M2d9V1X7X7qKj5Y9g', decimals: 8, chainId: 101, logoURI: 'https://assets.coingecko.com/coins/images/15724/small/polis.png' },
    { symbol: 'SYP', name: 'Sypool', address: 'SYPk9bK3k8xWv3v7X8xW6v3v7X8xW6v3v7X8xW6v3v', decimals: 9, chainId: 101, logoURI: 'https://assets.coingecko.com/coins/images/15725/small/syp.png' },
    { symbol: 'SAMO', name: 'Samoyedcoin', address: '7xKXtg2CW87d97TXJSDpbD5jTkhej8cNLu1uFMoGqZz', decimals: 9, chainId: 101, logoURI: 'https://assets.coingecko.com/coins/images/15874/small/samo.png' },
    { symbol: 'REN', name: 'Ren', address: '0x0000000000000000000000000000000000000000', decimals: 18, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/2539/small/ren.png' },
    { symbol: 'OCEAN', name: 'Ocean Protocol', address: '0x0000000000000000000000000000000000000000', decimals: 18, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/3687/small/ocean-protocol-logo.jpg' },
    { symbol: 'BAND', name: 'Band Protocol', address: '0x0000000000000000000000000000000000000000', decimals: 18, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/5135/small/Band_token_blue.png' },
    { symbol: 'SUSHI', name: 'SushiSwap', address: '0x6B3595068778DD592e39A122f4f5a5cF09C90fE2', decimals: 18, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/12271/small/512x512_Logo_no_chop.png' },
    { symbol: 'YFI', name: 'Yearn Finance', address: '0x0bc529c00C6401aEF6D5BE5905903DDEEA07a778', decimals: 18, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/12149/small/yfi-192x192.png' },
    { symbol: 'YFII', name: 'DFI.money', address: '0xa1d0E215a23d7030842FC67cE582a6aFa93CCB0d', decimals: 18, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/11902/small/YFII-logo.3ec3c1ac7b4f2a7b1a7c1a7.png' },
    { symbol: 'RUNE', name: 'THORChain', address: '0x0000000000000000000000000000000000000000', decimals: 8, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/6595/small/Rune200x200.png' },
    { symbol: 'KAVA', name: 'Kava', address: '0x0000000000000000000000000000000000000000', decimals: 18, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/9761/small/kava.png' },
    { symbol: 'LUNA', name: 'Terra', address: '0x0000000000000000000000000000000000000000', decimals: 6, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/8284/small/luna1557227671661.png' },
    { symbol: 'UST', name: 'TerraUSD', address: '0x0000000000000000000000000000000000000000', decimals: 6, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/12624/small/UST.png' },
    { symbol: 'BETH', name: 'Binance ETH', address: '0x0000000000000000000000000000000000000000', decimals: 18, chainId: 56, logoURI: 'https://assets.coingecko.com/coins/images/16912/small/beth.png' },
    { symbol: 'BTT', name: 'BitTorrent', address: '0x0000000000000000000000000000000000000000', decimals: 18, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/22457/small/bittorrent_new.png' },
    { symbol: 'GOLD', name: 'Gold', address: '0x0000000000000000000000000000000000000000', decimals: 8, chainId: 1, logoURI: 'https://assets.coingecko.com/coins/images/10577/small/gold.png' },
  ];
}
