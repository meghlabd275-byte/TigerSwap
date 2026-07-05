/**
 * TigerSwap Wallet SDK - Browser Implementation
 */

export interface GasPriceInfo {
  slow: string;
  standard: string;
  fast: string;
  instant: string;
  baseFee: string;
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

export class TigerSwapWallet {
  private connected: boolean = false;
  private account: string | null = null;
  private chainId: number = 1;
  private provider: 'metamask' | 'walletconnect' | 'coinbase' | null = null;

  async autoConnect(): Promise<boolean> {
    // In production, this would check browser storage/provider state
    return false;
  }

  async connectMetaMask(): Promise<string | null> {
    this.connected = true;
    this.account = '0x1234567890123456789012345678901234567890';
    this.provider = 'metamask';
    return this.account;
  }

  async connectCoinbaseWallet(): Promise<string | null> {
    this.connected = true;
    this.account = '0x1234567890123456789012345678901234567890';
    this.provider = 'coinbase';
    return this.account;
  }

  async connectWalletConnect(): Promise<string | null> {
    this.connected = true;
    this.account = '0x1234567890123456789012345678901234567890';
    this.provider = 'walletconnect';
    return this.account;
  }

  disconnect(): void {
    this.connected = false;
    this.account = null;
    this.provider = null;
  }

  isConnected(): boolean {
    return this.connected;
  }

  getAccount(): string | null {
    return this.account;
  }

  getChainId(): number {
    return this.chainId;
  }

  getChainName(): string {
    const names: Record<number, string> = {
      1: 'Ethereum',
      56: 'BNB Chain',
      137: 'Polygon',
      42161: 'Arbitrum',
      10: 'Optimism',
      8453: 'Base'
    };
    return names[this.chainId] || 'Unknown';
  }

  getProvider(): 'metamask' | 'walletconnect' | 'coinbase' | null {
    return this.provider;
  }

  async getNativeBalance(): Promise<string> {
    return '0x0';
  }

  async getTokenBalance(tokenAddress: string): Promise<string> {
    return '0x0';
  }

  async getGasPrice(): Promise<GasPriceInfo> {
    return {
      slow: '20000000000',
      standard: '35000000000',
      fast: '50000000000',
      instant: '75000000000',
      baseFee: '30000000000'
    };
  }

  async getPriceFromChainlink(base: string, quote: string): Promise<number> {
    return 2450.0;
  }

  formatBalance(hex: string): string {
    return '0';
  }

  formatGwei(wei: string): string {
    return (parseInt(wei) / 1e9).toString();
  }

  async callContract(address: string, abi: any[], method: string, args: any[]): Promise<any> {
    if (method === 'getAmountsOut') {
        // Mock implementation of getAmountsOut
        return ['0', args[0]];
    }
    return null;
  }

  async estimateGas(tx: any): Promise<string> {
    return '0x5208';
  }

  encodeFunctionCall(abi: any[], method: string, args: any[]): string {
    return '0x';
  }

  async getAllowance(token: string, spender: string): Promise<string> {
    return '0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff';
  }

  async approve(token: string, spender: string, amount: bigint): Promise<string> {
    return '0x' + '0'.repeat(64);
  }

  async sendTransaction(tx: any): Promise<string> {
    return '0x' + '0'.repeat(64);
  }

  async executeSwap(tokenIn: string, tokenOut: string, amountIn: bigint, minAmountOut: bigint, path: string[], router: string, deadline: number): Promise<string> {
    return '0x' + '0'.repeat(64);
  }

  async waitForConfirmation(hash: string): Promise<any> {
    return { status: 'success' };
  }

  onAccountsChange(callback: (accounts: string[]) => void): void {}
  onChainChange(callback: (chainId: number) => void): void {}
  onDisconnectCallback(callback: () => void): void {}
}

export const COMMON_TOKENS: Record<number, Record<string, TokenInfo>> = {
  1: {
    'ETH': { address: '0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2', symbol: 'ETH', name: 'Ethereum', decimals: 18, priceUSD: 2450, chainId: 1, isNative: true },
    'USDC': { address: '0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48', symbol: 'USDC', name: 'USD Coin', decimals: 6, priceUSD: 1, chainId: 1, isStable: true },
  }
};

export const DEX_ROUTERS: Record<number, { UniswapV2?: string }> = {
  1: {
    UniswapV2: '0x7a250d5630B4cF539739dF2C5dAcb4c659F2488D'
  }
};
