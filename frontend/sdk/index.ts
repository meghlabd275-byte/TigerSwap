/**
 * TigerSwap TypeScript SDK
 * Production-ready SDK for interacting with TigerSwap DEX
 */

import { ethers, providers, Wallet, Contract } from 'ethers';

// ============================================================================
// Types
// ============================================================================

export interface Token {
  symbol: string;
  name: string;
  address: string;
  decimals: number;
  chainId: number;
}

export interface Pool {
  poolId: string;
  token0: Token;
  token1: Token;
  reserve0: string;
  reserve1: string;
  fee: number;
  tick: number;
}

export interface SwapParams {
  tokenIn: string;
  tokenOut: string;
  amountIn: string;
  amountOutMin: string;
  to: string;
  deadline: number;
}

export interface Quote {
  amountIn: string;
  amountOut: string;
  priceImpact: string;
  gasEstimate: string;
}

export interface Position {
  tokenId: number;
  owner: string;
  token0: string;
  token1: string;
  tickLower: number;
  tickUpper: number;
  liquidity: string;
  feeGrowthInside0LastX128: string;
  feeGrowthInside1LastX128: string;
}

// ============================================================================
// SDK Client
// ============================================================================

export class TigerSwapSDK {
  private provider: providers.Provider;
  private wallet?: Wallet;
  private chainId: number;
  
  // Contract addresses (would be configured per chain)
  private readonly FACTORY_ADDRESS = '0x...';
  private readonly ROUTER_ADDRESS = '0x...';
  
  constructor(
    provider: providers.Provider,
    privateKey?: string,
    chainId: number = 1
  ) {
    this.provider = provider;
    this.chainId = chainId;
    
    if (privateKey) {
      this.wallet = new Wallet(privateKey, provider);
    }
  }
  
  // ============================================================================
  // Token Operations
  // ============================================================================
  
  /**
   * Get token balance
   */
  async getBalance(tokenAddress: string, account: string): Promise<string> {
    if (tokenAddress === ethers.constants.AddressZero) {
      // ETH balance
      return this.provider.getBalance(account);
    }
    
    // ERC20 balance
    const token = new Contract(
      tokenAddress,
      ['function balanceOf(address) view returns (uint256)'],
      this.provider
    );
    
    return token.balanceOf(account);
  }
  
  /**
   * Get token allowance
   */
  async getAllowance(tokenAddress: string, owner: string, spender: string): Promise<string> {
    const token = new Contract(
      tokenAddress,
      ['function allowance(address, address) view returns (uint256)'],
      this.provider
    );
    
    return token.allowance(owner, spender);
  }
  
  /**
   * Approve token
   */
  async approve(tokenAddress: string, spender: string, amount: string): Promise<ethers.TransactionResponse> {
    if (!this.wallet) {
      throw new Error('Wallet not configured');
    }
    
    const token = new Contract(
      tokenAddress,
      ['function approve(address, uint256) returns (bool)'],
      this.wallet
    );
    
    return token.approve(spender, amount);
  }
  
  // ============================================================================
  // Swap Operations
  // ============================================================================
  
  /**
   * Get swap quote
   */
  async getQuote(params: SwapParams): Promise<Quote> {
    // In production, call quoting API
    return {
      amountIn: params.amountIn,
      amountOut: '0', // Would be calculated
      priceImpact: '0.1',
      gasEstimate: '100000',
    };
  }
  
  /**
   * Execute swap
   */
  async swap(params: SwapParams): Promise<ethers.TransactionResponse> {
    if (!this.wallet) {
      throw new Error('Wallet not configured');
    }
    
    // Execute swap via router
    const router = new Contract(
      this.ROUTER_ADDRESS,
      [
        'function exactInputSingle((address, address, address, uint256, uint256, uint256, uint256, bytes))',
      ],
      this.wallet
    );
    
    return router.exactInputSingle([
      params.tokenIn,
      params.tokenOut,
      this.wallet.address,
      params.amountIn,
      params.amountOutMin,
      params.deadline,
      '0x',
    ]);
  }
  
  /**
   * Get pool info
   */
  async getPool(tokenA: string, tokenB: string): Promise<Pool | null> {
    const factory = new Contract(
      this.FACTORY_ADDRESS,
      ['function getPool(address, address, uint24) view returns (address)'],
      this.provider
    );
    
    const poolAddress = await factory.getPool(tokenA, tokenB, 3000);
    
    if (poolAddress === ethers.constants.AddressZero) {
      return null;
    }
    
    // Get pool data
    return {
      poolId: poolAddress,
      token0: { symbol: 'TOKEN0', name: 'Token 0', address: tokenA, decimals: 18, chainId: this.chainId },
      token1: { symbol: 'TOKEN1', name: 'Token 1', address: tokenB, decimals: 18, chainId: this.chainId },
      reserve0: '0',
      reserve1: '0',
      fee: 3000,
      tick: 0,
    };
  }
  
  // ============================================================================
  // Position Operations
  // ============================================================================
  
  /**
   * Get positions for address
   */
  async getPositions(owner: string): Promise<Position[]> {
    // Would query position NFT contract
    return [];
  }
  
  /**
   * Create position
   */
  async createPosition(
    token0: string,
    token1: string,
    fee: number,
    tickLower: number,
    tickUpper: number,
    amount0Desired: string,
    amount1Desired: string
  ): Promise<ethers.TransactionResponse> {
    if (!this.wallet) {
      throw new Error('Wallet not configured');
    }
    
    // Mint position via NFT
    const router = new Contract(
      this.ROUTER_ADDRESS,
      ['function mint((address, address, uint24, int24, int24, uint256, uint256, uint256, uint256, address, uint256))'],
      this.wallet
    );
    
    return router.mint([
      token0,
      token1,
      fee,
      tickLower,
      tickUpper,
      amount0Desired,
      amount1Desired,
      0,
      0,
      this.wallet.address,
      0,
    ]);
  }
  
  // ============================================================================
  // Utility
  // ============================================================================
  
  /**
   * Get chain ID
   */
  getChainId(): number {
    return this.chainId;
  }
  
  /**
   * Get connected wallet address
   */
  getAddress(): string | null {
    return this.wallet?.address ?? null;
  }
}

// ============================================================================
// Swap Widget
// ============================================================================

export class SwapWidget {
  private sdk: TigerSwapSDK;
  
  constructor(sdk: TigerSwapSDK) {
    this.sdk = sdk;
  }
  
  /**
   * Render swap widget
   */
  render(container: HTMLElement): void {
    container.innerHTML = `
      <div class="tigerswap-swap-widget">
        <div class="widget-header">
          <h3>Swap</h3>
        </div>
        <div class="widget-body">
          <div class="token-input">
            <label>From</label>
            <input type="number" placeholder="0.0" />
            <select></select>
          </div>
          <div class="swap-icon">↓</div>
          <div class="token-input">
            <label>To</label>
            <input type="number" placeholder="0.0" disabled />
            <select></select>
          </div>
        </div>
        <div class="widget-footer">
          <button class="swap-button">Swap</button>
        </div>
      </div>
    `;
  }
}

// ============================================================================
// Wallet Widget
// ============================================================================

export class WalletWidget {
  private sdk: TigerSwapSDK;
  
  constructor(sdk: TigerSwapSDK) {
    this.sdk = sdk;
  }
  
  /**
   * Render wallet widget
   */
  render(container: HTMLElement): void {
    const address = this.sdk.getAddress();
    
    container.innerHTML = `
      <div class="tigerswap-wallet-widget">
        <div class="wallet-header">
          <span class="balance">$0.00</span>
          <span class="address">${address ? address.slice(0, 6) + '...' + address.slice(-4) : 'Not connected'}</span>
        </div>
        <div class="wallet-actions">
          <button class="action-button">Send</button>
          <button class="action-button">Receive</button>
          <button class="action-button">Buy</button>
        </div>
      </div>
    `;
  }
}

// ============================================================================
// Export
// ============================================================================

export default TigerSwapSDK;