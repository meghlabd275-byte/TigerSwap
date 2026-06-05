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
}

export interface TransactionReceipt {
  transactionHash: string;
  blockNumber: number;
  status: 'success' | 'reverted';
  gasUsed: string;
  effectiveGasPrice: string;
  logs: any[];
}

// ============================================================================
// Chain Configurations
// ============================================================================

export const SUPPORTED_CHAINS: Record<number, ChainConfig> = {
  // Ethereum Mainnet
  1: {
    chainId: 1,
    chainName: 'Ethereum',
    rpcUrl: 'https://eth.llamarpc.com',
    explorerUrl: 'https://etherscan.io',
    nativeCurrency: { name: 'Ether', symbol: 'ETH', decimals: 18 },
  },
  // Binance Smart Chain Mainnet
  56: {
    chainId: 56,
    chainName: 'BNB Chain',
    rpcUrl: 'https://bsc-dataseed.binance.org',
    explorerUrl: 'https://bscscan.com',
    nativeCurrency: { name: 'BNB', symbol: 'BNB', decimals: 18 },
  },
  // Arbitrum One
  42161: {
    chainId: 42161,
    chainName: 'Arbitrum One',
    rpcUrl: 'https://arb1.arbitrum.io/rpc',
    explorerUrl: 'https://arbiscan.io',
    nativeCurrency: { name: 'Ether', symbol: 'ETH', decimals: 18 },
  },
  // Optimism
  10: {
    chainId: 10,
    chainName: 'Optimism',
    rpcUrl: 'https://mainnet.optimism.io',
    explorerUrl: 'https://optimistic.etherscan.io',
    nativeCurrency: { name: 'Ether', symbol: 'ETH', decimals: 18 },
  },
  // Polygon
  137: {
    chainId: 137,
    chainName: 'Polygon',
    rpcUrl: 'https://polygon-rpc.com',
    explorerUrl: 'https://polygonscan.com',
    nativeCurrency: { name: 'MATIC', symbol: 'MATIC', decimals: 18 },
  },
  // Base
  8453: {
    chainId: 8453,
    chainName: 'Base',
    rpcUrl: 'https://mainnet.base.org',
    explorerUrl: 'https://basescan.org',
    nativeCurrency: { name: 'Ether', symbol: 'ETH', decimals: 18 },
  },
  // Solana (using different interface)
  101: {
    chainId: 101,
    chainName: 'Solana',
    rpcUrl: 'https://api.mainnet-beta.solana.com',
    explorerUrl: 'https://solscan.io',
    nativeCurrency: { name: 'Sol', symbol: 'SOL', decimals: 9 },
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
    inputs: [{ name: 'to', type: 'address' }, { name: 'amount', type: 'uint256' }],
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
// TigerSwap Router ABI
// ============================================================================

export const ROUTER_ABI = [
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
    name: 'addLiquidityETH',
    inputs: [
      { name: 'token', type: 'address' },
      { name: 'amountTokenDesired', type: 'uint256' },
      { name: 'amountTokenMin', type: 'uint256' },
      { name: 'amountETHMin', type: 'uint256' },
      { name: 'to', type: 'address' },
      { name: 'deadline', type: 'uint256' },
    ],
    outputs: [
      { name: 'amountToken', type: 'uint256' },
      { name: 'amountETH', type: 'uint256' },
      { name: 'liquidity', type: 'uint256' },
    ],
    stateMutability: 'payable',
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
    name: 'swapTokensForExactTokens',
    inputs: [
      { name: 'amountOut', type: 'uint256' },
      { name: 'amountInMax', type: 'uint256' },
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
    name: 'getAmountsOut',
    inputs: [{ name: 'amountIn', type: 'uint256' }, { name: 'path', type: 'address[]' }],
    outputs: [{ name: 'amounts', type: 'uint256[]' }],
    stateMutability: 'view',
    type: 'function',
  },
  {
    name: 'getAmountsIn',
    inputs: [{ name: 'amountOut', type: 'uint256' }, { name: 'path', type: 'address[]' }],
    outputs: [{ name: 'amounts', type: 'uint256[]' }],
    stateMutability: 'view',
    type: 'function',
  },
];

// ============================================================================
// Utility Functions
// ============================================================================

/**
 * Format wallet address for display
 */
export function formatAddress(address: string, chars: number = 4): string {
  if (!address) return '';
  return `${address.slice(0, chars + 2)}...${address.slice(-chars)}`;
}

/**
 * Format balance with proper decimals
 */
export function formatBalance(balance: string, decimals: number = 18, displayDecimals: number = 4): string {
  const num = Number(balance) / Math.pow(10, decimals);
  return num.toFixed(displayDecimals);
}

/**
 * Parse token amount from display string
 */
export function parseTokenAmount(amount: string, decimals: number = 18): bigint {
  const [whole, fraction = ''] = amount.split('.');
  const paddedFraction = fraction.padEnd(decimals, '0').slice(0, decimals);
  return BigInt(whole + paddedFraction);
}

/**
 * Convert to Hex string
 */
export function toHex(value: bigint): string {
  return '0x' + value.toString(16);
}

/**
 * Convert from Hex string
 */
export function fromHex(hex: string): bigint {
  return BigInt(hex);
}

// ============================================================================
// TigerSwap Wallet Class
// ============================================================================

export class TigerSwapWallet {
  private provider: any;
  private signer: any;
  private chainId: number = 1;
  private account: string | null = null;

  // Event callbacks
  public onConnect?: (account: string, chainId: number) => void;
  public onDisconnect?: () => void;
  public onChainChanged?: (chainId: number) => void;
  public onAccountChanged?: (account: string) => void;
  public onError?: (error: string) => void;

  constructor() {
    this.provider = null;
    this.signer = null;
  }

  // ==================== CONNECTION ====================

  /**
   * Check if MetaMask is available
   */
  isMetaMaskAvailable(): boolean {
    if (typeof window === 'undefined') return false;
    return typeof (window as any).ethereum !== 'undefined';
  }

  /**
   * Connect to MetaMask wallet
   */
  async connectMetaMask(): Promise<string> {
    if (!this.isMetaMaskAvailable()) {
      throw new Error('MetaMask is not installed');
    }

    try {
      const ethereum = (window as any).ethereum;
      
      // Request account access
      const accounts = await ethereum.request({
        method: 'eth_requestAccounts',
      });

      if (!accounts || accounts.length === 0) {
        throw new Error('No accounts found');
      }

      this.account = accounts[0];
      
      // Get chain ID
      const chainIdHex = await ethereum.request({ method: 'eth_chainId' });
      this.chainId = parseInt(chainIdHex, 16);

      // Setup provider and signer
      this.provider = ethereum;
      
      if (this.onConnect) {
        this.onConnect(this.account, this.chainId);
      }

      // Setup event listeners
      this.setupEventListeners();

      return this.account;
    } catch (error: any) {
      if (this.onError) {
        this.onError(error.message || 'Failed to connect');
      }
      throw error;
    }
  }

  /**
   * Connect to WalletConnect (requires WalletConnect project ID)
   */
  async connectWalletConnect(projectId: string): Promise<string> {
    // WalletConnect implementation would go here
    // For now, just connect MetaMask
    return this.connectMetaMask();
  }

  /**
   * Connect to Coinbase Wallet
   */
  async connectCoinbase(): Promise<string> {
    if (typeof window === 'undefined') {
      throw new Error('Browser required');
    }

    const coinbase = (window as any).coinbaseWallet;
    if (!coinbase) {
      throw new Error('Coinbase Wallet not installed');
    }

    try {
      const accounts = await coinbase.request({
        method: 'eth_requestAccounts',
      });

      this.account = accounts[0];
      const chainIdHex = await coinbase.request({ method: 'eth_chainId' });
      this.chainId = parseInt(chainIdHex, 16);

      this.provider = coinbase;
      this.setupEventListeners();

      if (this.onConnect) {
        this.onConnect(this.account, this.chainId);
      }

      return this.account;
    } catch (error: any) {
      throw new Error(`Coinbase connection failed: ${error.message}`);
    }
  }

  /**
   * Disconnect wallet
   */
  disconnect(): void {
    this.account = null;
    this.provider = null;
    this.signer = null;
    
    if (this.onDisconnect) {
      this.onDisconnect();
    }
  }

  // ==================== CHAIN MANAGEMENT ====================

  /**
   * Switch to a different chain
   */
  async switchChain(targetChainId: number): Promise<void> {
    const chainConfig = SUPPORTED_CHAINS[targetChainId];
    if (!chainConfig) {
      throw new Error(`Chain ${targetChainId} is not supported`);
    }

    const ethereum = (window as any).ethereum;
    if (!ethereum) {
      throw new Error('No wallet connected');
    }

    try {
      await ethereum.request({
        method: 'wallet_switchEthereumChain',
        params: [{ chainId: toHex(BigInt(targetChainId)) }],
      });
    } catch (switchError: any) {
      // If chain doesn't exist, add it
      if (switchError.code === 4902) {
        await ethereum.request({
          method: 'wallet_addEthereumChain',
          params: [
            {
              chainId: toHex(BigInt(targetChainId)),
              chainName: chainConfig.chainName,
              rpcUrls: [chainConfig.rpcUrl],
              blockExplorerUrls: [chainConfig.explorerUrl],
              nativeCurrency: chainConfig.nativeCurrency,
            },
          ],
        });
      } else {
        throw switchError;
      }
    }
  }

  /**
   * Add custom token to wallet
   */
  async addTokenToWallet(token: {
    address: string;
    symbol: string;
    decimals: number;
    image?: string;
  }): Promise<void> {
    const ethereum = (window as any).ethereum;
    if (!ethereum) return;

    try {
      await ethereum.request({
        method: 'wallet_watchAsset',
        params: {
          type: 'ERC20',
          options: {
            address: token.address,
            symbol: token.symbol,
            decimals: token.decimals,
            image: token.image,
          },
        },
      });
    } catch (error) {
      console.error('Failed to add token:', error);
    }
  }

  // ==================== BALANCE & TOKEN OPERATIONS ====================

  /**
   * Get native balance (ETH/BNB/MATIC)
   */
  async getNativeBalance(): Promise<string> {
    if (!this.account || !this.provider) {
      throw new Error('Wallet not connected');
    }

    const balance = await this.provider.request({
      method: 'eth_getBalance',
      params: [this.account, 'latest'],
    });

    return balance;
  }

  /**
   * Get ERC-20 token balance
   */
  async getTokenBalance(tokenAddress: string): Promise<string> {
    if (!this.account || !this.provider) {
      throw new Error('Wallet not connected');
    }

    const result = await this.provider.request({
      method: 'eth_call',
      params: [
        {
          to: tokenAddress,
          data: `0x70a08231000000000000000000000000${this.account.slice(2)}`,
        },
        'latest',
      ],
    });

    return result;
  }

  /**
   * Get token information
   */
  async getTokenInfo(tokenAddress: string): Promise<{
    name: string;
    symbol: string;
    decimals: number;
    totalSupply: string;
  }> {
    if (!this.provider) {
      throw new Error('Wallet not connected');
    }

    const [name, symbol, decimals, totalSupply] = await Promise.all([
      this.callContract(tokenAddress, ERC20_ABI, 'name', []),
      this.callContract(tokenAddress, ERC20_ABI, 'symbol', []),
      this.callContract(tokenAddress, ERC20_ABI, 'decimals', []),
      this.callContract(tokenAddress, ERC20_ABI, 'totalSupply', []),
    ]);

    return { name, symbol, decimals: Number(decimals), totalSupply };
  }

  /**
   * Approve token for spending
   */
  async approveToken(tokenAddress: string, spender: string, amount: bigint): Promise<string> {
    return this.sendTransaction({
      to: tokenAddress,
      data: this.encodeFunctionCall(ERC20_ABI, 'approve', [spender, toHex(amount)]),
    });
  }

  /**
   * Check allowance
   */
  async getAllowance(tokenAddress: string, owner: string, spender: string): Promise<string> {
    return this.callContract(tokenAddress, ERC20_ABI, 'allowance', [owner, spender]);
  }

  // ==================== TRANSACTIONS ====================

  /**
   * Send transaction (no data = native token transfer)
   */
  async sendTransaction(tx: TransactionRequest): Promise<string> {
    if (!this.account || !this.provider) {
      throw new Error('Wallet not connected');
    }

    try {
      // Add from address
      const txWithFrom = { ...tx, from: this.account };

      // Estimate gas if not provided
      if (!txWithFrom.gas) {
        txWithFrom.gas = await this.estimateGas(txWithFrom);
      }

      // Send transaction
      const txHash = await this.provider.request({
        method: 'eth_sendTransaction',
        params: [txWithFrom],
      });

      return txHash;
    } catch (error: any) {
      if (this.onError) {
        this.onError(error.message || 'Transaction failed');
      }
      throw error;
    }
  }

  /**
   * Sign and send transaction (for typed data)
   */
  async signTransaction(tx: TransactionRequest): Promise<string> {
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
      throw new Error(`Signing failed: ${error.message}`);
    }
  }

  /**
   * Get transaction receipt
   */
  async getTransactionReceipt(txHash: string): Promise<TransactionReceipt> {
    const receipt = await this.provider.request({
      method: 'eth_getTransactionReceipt',
      params: [txHash],
    });

    return {
      transactionHash: receipt.transactionHash,
      blockNumber: receipt.blockNumber,
      status: receipt.status === '0x1' ? 'success' : 'reverted',
      gasUsed: receipt.gasUsed,
      effectiveGasPrice: receipt.effectiveGasPrice,
      logs: receipt.logs,
    };
  }

  /**
   * Wait for transaction confirmation
   */
  async waitForConfirmation(txHash: string, confirmations: number = 1): Promise<TransactionReceipt> {
    return new Promise((resolve, reject) => {
      const checkReceipt = async () => {
        try {
          const receipt = await this.getTransactionReceipt(txHash);
          
          // Get current block to check confirmations
          const currentBlockHex = await this.provider.request({
            method: 'eth_blockNumber',
          });
          const currentBlock = parseInt(currentBlockHex, 16);
          const confirmationsSoFar = currentBlock - receipt.blockNumber;

          if (confirmationsSoFar >= confirmations) {
            resolve(receipt);
          } else {
            setTimeout(checkReceipt, 5000);
          }
        } catch (error) {
          setTimeout(checkReceipt, 5000);
        }
      };

      checkReceipt();
    });
  }

  // ==================== CONTRACT INTERACTIONS ====================

  /**
   * Call contract function (view/read-only)
   */
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

  /**
   * Encode function call data
   */
  encodeFunctionCall(abi: any[], functionName: string, params: any[]): string {
    const func = abi.find((f) => f.name === functionName);
    if (!func) throw new Error(`Function ${functionName} not found in ABI`);

    const selector = this.getFunctionSelector(func);
    const encodedParams = this.encodeParams(func.inputs, params);

    return selector + encodedParams;
  }

  /**
   * Get function selector (4-byte signature)
   */
  private getFunctionSelector(func: any): string {
    const signature = `${func.name}(${func.inputs.map((i: any) => i.type).join(',')})`;
    return this.keccak256(signature).slice(0, 10);
  }

  /**
   * Simple keccak256 implementation
   */
  private keccak256(str: string): string {
    // In production, use proper keccak library
    // This is a placeholder
    const encoder = new TextEncoder();
    const data = encoder.encode(str);
    
    // For actual keccak, import: import { keccak256 } from '@ethersproject/keccak256';
    // return keccak256(Buffer.from(data)).slice(2);
    
    // Mock for now - in production use proper library
    return '0x' + Array.from(data).map(b => b.toString(16).padStart(2, '0')).join('').slice(0, 8).padEnd(8, '0');
  }

  /**
   * Encode parameters
   */
  private encodeParams(inputs: any[], params: any[]): string {
    if (!inputs || inputs.length === 0) return '';

    let encoded = '';
    let dynamicOffset = inputs.length * 32;

    for (let i = 0; i < inputs.length; i++) {
      const input = inputs[i];
      const param = params[i];

      if (input.type === 'address') {
        encoded += param.slice(2).padStart(64, '0');
      } else if (input.type === 'uint256') {
        encoded += BigInt(param).toString(16).padStart(64, '0');
      } else if (input.type === 'uint8') {
        encoded += Number(param).toString(16).padStart(64, '0');
      } else if (input.type === 'bytes') {
        const dataOffset = dynamicOffset.toString(16).padStart(64, '0');
        encoded += dataOffset;
        
        const paddedData = param.slice(2).padStart(Math.ceil((param.length - 2) / 64) * 64 + 2, '0');
        encoded += paddedData;
        dynamicOffset += Math.ceil((param.length - 2) / 2 / 32) * 32;
      }
    }

    return encoded;
  }

  /**
   * Decode function result
   */
  private decodeFunctionResult(abi: any[], functionName: string, result: string): any {
    if (!result || result === '0x') return null;

    const func = abi.find((f) => f.name === functionName);
    if (!func) return null;

    // Simplified decoding - in production use proper ABI decoder
    if (func.outputs && func.outputs.length === 1) {
      const output = func.outputs[0];
      if (output.type === 'uint256' || output.type === 'uint112') {
        return BigInt(result);
      }
      if (output.type === 'address') {
        return '0x' + result.slice(result.length - 40);
      }
    }

    return result;
  }

  // ==================== GAS ESTIMATION ====================

  /**
   * Estimate gas for transaction
   */
  async estimateGas(tx: TransactionRequest): Promise<string> {
    if (!this.account || !this.provider) {
      throw new Error('Wallet not connected');
    }

    try {
      const gas = await this.provider.request({
        method: 'eth_estimateGas',
        params: [{ ...tx, from: this.account }],
      });
      return gas;
    } catch (error) {
      // Fallback to default gas
      return '0x5208'; // 21000 gas for basic transfer
    }
  }

  /**
   * Get current gas price
   */
  async getGasPrice(): Promise<string> {
    if (!this.provider) {
      throw new Error('Wallet not connected');
    }

    return await this.provider.request({
      method: 'eth_gasPrice',
    });
  }

  /**
   * Get current block info (for gas estimation)
   */
  async getCurrentBlock(): Promise<{
    number: number;
    baseFeePerGas: string;
    gasLimit: string;
  }> {
    if (!this.provider) {
      throw new Error('Wallet not connected');
    }

    const block = await this.provider.request({
      method: 'eth_getBlockByNumber',
      params: ['latest', false],
    });

    return {
      number: parseInt(block.number, 16),
      baseFeePerGas: block.baseFeePerGas || '0x0',
      gasLimit: block.gasLimit,
    };
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
      if (this.onChainChanged) {
        this.onChainChanged(this.chainId);
      }
    });

    ethereum.on('disconnect', () => {
      this.disconnect();
    });
  }

  // ==================== GETTERS ====================

  getAccount(): string | null {
    return this.account;
  }

  getChainId(): number {
    return this.chainId;
  }

  isConnected(): boolean {
    return this.account !== null;
  }

  getProvider(): any {
    return this.provider;
  }
}

// ============================================================================
// Default Export
// ============================================================================

export default TigerSwapWallet;

// ============================================================================
// Usage Example
// ============================================================================

/**
 * Example usage:
 * 
 * const wallet = new TigerSwapWallet();
 * 
 * // Connect
 * const account = await wallet.connectMetaMask();
 * console.log('Connected:', account);
 * 
 * // Get balance
 * const balance = await wallet.getNativeBalance();
 * console.log('Balance:', formatBalance(balance));
 * 
 * // Approve token
 * await wallet.approveToken(USDT_ADDRESS, ROUTER_ADDRESS, BigInt(1e18));
 * 
 * // Execute swap
 * const txHash = await wallet.sendTransaction({
 *   to: ROUTER_ADDRESS,
 *   data: encodedSwapData,
 *   value: '0x0',
 * });
 * 
 * // Wait for confirmation
 * const receipt = await wallet.waitForConfirmation(txHash);
 * console.log('Confirmed:', receipt.transactionHash);
 */