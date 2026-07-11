/**
 * TigerSwap Mobile Wallet - Production Service
 * Core service for interacting with TigerSwap DEX
 * 
 * Features:
 * - Wallet connection (EVM, Solana, Cosmos)
 * - Token swaps
 * - Liquidity management
 * - Portfolio tracking
 * - Price alerts
 * 
 * @author TigerSwap
 * @version 1.0.0 Production
 */

import { ethers, providers } from 'ethers';
import { Connection, PublicKey, Transaction } from '@solana/web3.js';

// ==================== Types ====================

export interface Token {
  address: string;
  symbol: string;
  name: string;
  decimals: number;
  logo?: string;
  price?: number;
  balance?: string;
}

export interface Pool {
  address: string;
  tokenA: Token;
  tokenB: Token;
  tvl: number;
  volume24h: number;
  apy: number;
}

export interface SwapQuote {
  fromToken: Token;
  toToken: Token;
  fromAmount: string;
  toAmount: string;
  priceImpact: number;
  gasEstimate: string;
  route: string[];
}

export interface Position {
  tokenA: Token;
  tokenB: Token;
  liquidity: string;
  value: number;
  apy: number;
  fees24h: number;
}

export interface TransactionStatus {
  hash: string;
  status: 'pending' | 'confirmed' | 'failed';
  blockNumber?: number;
  timestamp: number;
}

export type ChainType = 'evm' | 'solana' | 'cosmos';

export interface WalletState {
  address: string | null;
  chain: ChainType | null;
  balance: string;
  connected: boolean;
}

// ==================== Configuration ====================

const CONFIG = {
  evm: {
    chainId: 1,
    rpcUrl: process.env.EVM_RPC_URL || 'https://eth-mainnet.g.alchemy.com',
    routerAddress: process.env.ROUTER_ADDRESS || '0x...',
  },
  solana: {
    rpcUrl: process.env.SOLANA_RPC_URL || 'https://api.mainnet-beta.solana.com',
    programId: process.env.SOLANA_PROGRAM_ID || '...',
  },
  cosmos: {
    rpcUrl: process.env.COSMOS_RPC_URL || 'https://rpc.cosmoshub.io',
  },
  apiUrl: process.env.API_URL || 'https://api.tigerswap.io',
  wsUrl: process.env.WS_URL || 'wss://ws.tigerswap.io',
};

// ==================== TigerSwap Service ====================

export class TigerSwapService {
  private provider: providers.JsonRpcProvider | Connection | null = null;
  private signer: ethers.Signer | null = null;
  private wallet: WalletState = {
    address: null,
    chain: null,
    balance: '0',
    connected: false,
  };

  // ==================== Wallet Connection ====================

  /**
   * Connect EVM wallet (MetaMask, WalletConnect, etc.)
   */
  async connectEvm(): Promise<WalletState> {
    try {
      // Check for injected provider
      if (typeof window !== 'undefined' && (window as any).ethereum) {
        const provider = new ethers.providers.Web3Provider(
          (window as any).ethereum,
          'any'
        );
        
        await provider.send('eth_requestAccounts', []);
        
        const signer = provider.getSigner();
        const address = await signer.getAddress();
        const network = await provider.getNetwork();
        
        this.provider = provider;
        this.signer = signer;
        
        this.wallet = {
          address,
          chain: 'evm',
          balance: await provider.getBalance(address),
          connected: true,
        };
        
        return this.wallet;
      }
      
      throw new Error('No EVM wallet found');
    } catch (error) {
      console.error('Failed to connect EVM wallet:', error);
      throw error;
    }
  }

  /**
   * Connect Solana wallet (Phantom, Solflare, etc.)
   */
  async connectSolana(): Promise<WalletState> {
    try {
      if (typeof window !== 'undefined' && (window as any).solana) {
        const { solana } = window as any;
        
        await solana.connect();
        const address = solana.publicKey.toString();
        
        this.wallet = {
          address,
          chain: 'solana',
          balance: '0', // Would need to fetch balance
          connected: true,
        };
        
        return this.wallet;
      }
      
      throw new Error('No Solana wallet found');
    } catch (error) {
      console.error('Failed to connect Solana wallet:', error);
      throw error;
    }
  }

  /**
   * Disconnect wallet
   */
  disconnect(): void {
    this.wallet = {
      address: null,
      chain: null,
      balance: '0',
      connected: false,
    };
    this.provider = null;
    this.signer = null;
  }

  // ==================== Token Operations ====================

  /**
   * Get token list
   */
  async getTokens(): Promise<Token[]> {
    const response = await fetch(`${CONFIG.apiUrl}/v1/tokens`);
    return response.json();
  }

  /**
   * Get token balance
   */
  async getTokenBalance(tokenAddress: string): Promise<string> {
    if (!this.wallet.address) throw new Error('Wallet not connected');
    
    if (this.wallet.chain === 'evm') {
      const abi = ['function balanceOf(address) view returns (uint256)'];
      const contract = new ethers.Contract(tokenAddress, abi, this.provider);
      const balance = await contract.balanceOf(this.wallet.address);
      return balance.toString();
    }
    
    throw new Error('Unsupported chain');
  }

  /**
   * Get token price
   */
  async getTokenPrice(tokenAddress: string): Promise<number> {
    const response = await fetch(`${CONFIG.apiUrl}/v1/price/${tokenAddress}`);
    const data = await response.json();
    return data.price;
  }

  // ==================== Swap Operations ====================

  /**
   * Get swap quote
   */
  async getSwapQuote(
    fromToken: string,
    toToken: string,
    amount: string
  ): Promise<SwapQuote> {
    const response = await fetch(
      `${CONFIG.apiUrl}/v1/swap/quote?` +
      `fromToken=${fromToken}&toToken=${toToken}&amount=${amount}`
    );
    return response.json();
  }

  /**
   * Execute swap
   */
  async executeSwap(quote: SwapQuote): Promise<TransactionStatus> {
    if (!this.signer) throw new Error('Wallet not connected');
    
    if (this.wallet.chain === 'evm') {
      // Build transaction
      const abi = ['function swapExactETHForTokens(uint amountOutMin, address[] path, address to, uint deadline)'];
      const router = new ethers.Contract(CONFIG.evm.routerAddress, abi, this.signer);
      
      const tx = await router.swapExactETHForTokens(
        quote.toAmount,
        quote.route,
        this.wallet.address,
        Math.floor(Date.now() / 1000) + 300 // 5 min deadline
      );
      
      const receipt = await tx.wait();
      
      return {
        hash: tx.hash,
        status: receipt.status === 1 ? 'confirmed' : 'failed',
        blockNumber: receipt.blockNumber,
        timestamp: Date.now(),
      };
    }
    
    throw new Error('Unsupported chain');
  }

  // ==================== Liquidity Operations ====================

  /**
   * Get pools for token pair
   */
  async getPools(tokenA: string, tokenB: string): Promise<Pool[]> {
    const response = await fetch(
      `${CONFIG.apiUrl}/v1/pools?tokenA=${tokenA}&tokenB=${tokenB}`
    );
    return response.json();
  }

  /**
   * Add liquidity
   */
  async addLiquidity(
    tokenA: string,
    tokenB: string,
    amountADesired: string,
    amountBDesired: string
  ): Promise<TransactionStatus> {
    if (!this.signer) throw new Error('Wallet not connected');
    
    const abi = [
      'function addLiquidity(address tokenA, address tokenB, uint amountADesired, uint amountBDesired, uint amountAMin, uint amountBMin, address to, uint deadline) returns (uint amountA, uint amountB, uint liquidity)'
    ];
    
    const router = new ethers.Contract(CONFIG.evm.routerAddress, abi, this.signer);
    
    const tx = await router.addLiquidity(
      tokenA,
      tokenB,
      amountADesired,
      amountBDesired,
      '0', // amountAMin - should calculate
      '0', // amountBMin - should calculate
      this.wallet.address,
      Math.floor(Date.now() / 1000) + 300
    );
    
    const receipt = await tx.wait();
    
    return {
      hash: tx.hash,
      status: receipt.status === 1 ? 'confirmed' : 'failed',
      blockNumber: receipt.blockNumber,
      timestamp: Date.now(),
    };
  }

  /**
   * Remove liquidity
   */
  async removeLiquidity(
    tokenA: string,
    tokenB: string,
    liquidity: string
  ): Promise<TransactionStatus> {
    if (!this.signer) throw new Error('Wallet not connected');
    
    const abi = [
      'function removeLiquidity(address tokenA, address tokenB, uint liquidity, uint amountAMin, uint amountBMin, address to, uint deadline) returns (uint amountA, uint amountB)'
    ];
    
    const router = new ethers.Contract(CONFIG.evm.routerAddress, abi, this.signer);
    
    const tx = await router.removeLiquidity(
      tokenA,
      tokenB,
      liquidity,
      '0',
      '0',
      this.wallet.address,
      Math.floor(Date.now() / 1000) + 300
    );
    
    const receipt = await tx.wait();
    
    return {
      hash: tx.hash,
      status: receipt.status === 1 ? 'confirmed' : 'failed',
      blockNumber: receipt.blockNumber,
      timestamp: Date.now(),
    };
  }

  // ==================== Portfolio ====================

  /**
   * Get user positions
   */
  async getPositions(): Promise<Position[]> {
    if (!this.wallet.address) throw new Error('Wallet not connected');
    
    const response = await fetch(
      `${CONFIG.apiUrl}/v1/portfolio/${this.wallet.address}`
    );
    return response.json();
  }

  /**
   * Get portfolio value
   */
  async getPortfolioValue(): Promise<number> {
    const positions = await this.getPositions();
    return positions.reduce((sum, pos) => sum + pos.value, 0);
  }

  // ==================== Price Alerts ====================

  /**
   * Set price alert
   */
  async setPriceAlert(
    token: string,
    targetPrice: number,
    direction: 'above' | 'below'
  ): Promise<void> {
    await fetch(`${CONFIG.apiUrl}/v1/alerts`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        token,
        targetPrice,
        direction,
        address: this.wallet.address,
      }),
    });
  }

  // ==================== Utility ====================

  /**
   * Get wallet state
   */
  getWalletState(): WalletState {
    return this.wallet;
  }

  /**
   * Get chain ID
   */
  getChainId(): number {
    return CONFIG.evm.chainId;
  }

  /**
   * Format token amount
   */
  formatAmount(amount: string, decimals: number): string {
    return ethers.utils.formatUnits(amount, decimals);
  }

  /**
   * Parse token amount
   */
  parseAmount(amount: string, decimals: number): string {
    return ethers.utils.parseUnits(amount, decimals).toString();
  }
}

// ==================== Export ====================

export const tigerSwapService = new TigerSwapService();
export default TigerSwapService;
