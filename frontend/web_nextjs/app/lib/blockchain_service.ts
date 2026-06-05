/**
 * TigerSwap Blockchain Service
 * Real blockchain integration layer for all DEX operations
 * Built from scratch - no dependencies on other DEX protocols
 */

import { ethers, JsonRpcProvider, Contract, BrowserProvider, formatUnits, parseUnits } from 'ethers';
import { CHAIN_CONFIG, ERC20_ABI, DEX_ROUTERS, COMMON_TOKENS, TokenInfo, GasPriceInfo } from '../../libs/web3_wallet/wallet';

// ============================================================================
// Types & Interfaces
// ============================================================================

export interface PoolInfo {
  address: string;
  token0: TokenInfo;
  token1: TokenInfo;
  reserve0: bigint;
  reserve1: bigint;
  fee: number;
  liquidity: bigint;
  volume24h: number;
  apr: number;
}

export interface LiquidityPosition {
  poolAddress: string;
  token0: TokenInfo;
  token1: TokenInfo;
  liquidity: bigint;
  amount0: bigint;
  amount1: bigint;
  feeEarned0: bigint;
  feeEarned1: bigint;
  poolShare: number;
}

export interface FarmPosition {
  pid: number;
  poolAddress: string;
  lpToken: string;
  amount: bigint;
  pendingReward: bigint;
  poolShare: number;
  apr: number;
}

export interface SwapResult {
  success: boolean;
  txHash?: string;
  amountOut?: bigint;
  priceImpact?: number;
  error?: string;
}

// ============================================================================
// Contract Addresses by Network (Deploy these after running deployment scripts)
// ============================================================================

export const CONTRACT_ADDRESSES: Record<number, {
  factory: string;
  router: string;
  masterChef?: string;
  staking?: string;
  governance?: string;
  tigerToken?: string;
}> = {
  1: { factory: '', router: '', masterChef: '', staking: '', governance: '', tigerToken: '' },
  11155111: { factory: '', router: '', masterChef: '', staking: '', governance: '', tigerToken: '' },
  42161: { factory: '', router: '' },
  137: { factory: '', router: '' },
  56: { factory: '', router: '' },
  8453: { factory: '', router: '' },
};

// ============================================================================
// ABI Definitions
// ============================================================================

const FACTORY_ABI = [
  'function getPair(address tokenA, address tokenB) external view returns (address pair)',
  'function allPairs(uint256) external view returns (address)',
  'function allPairsLength() external view returns (uint256)',
  'function feeTo() external view returns (address)',
  'function feeToSetter() external view returns (address)',
  'event PairCreated(address indexed token0, address indexed token1, address pair, uint256)',
];

const PAIR_ABI = [
  'function getReserves() external view returns (uint112 reserve0, uint112 reserve1, uint32 blockTimestampLast)',
  'function token0() external view returns (address)',
  'function token1() external view returns (address)',
  'function price0CumulativeLast() external view returns (uint256)',
  'function price1CumulativeLast() external view returns (uint256)',
  'function sync() external',
  'function mint(address to) external returns (uint256 liquidity)',
  'function burn(address to) external returns (uint256 amount0, uint256 amount1)',
  'function swap(uint256 amount0Out, uint256 amount1Out, address to) external',
  'event Mint(address indexed sender, uint256 amount0, uint256 amount1)',
  'event Burn(address indexed sender, uint256 amount0, uint256 amount1, address indexed to)',
  'event Swap(address indexed sender, uint256 amount0Out, uint256 amount1Out, uint256 amount1In, address indexed to)',
  'event Sync(uint112 reserve0, uint112 reserve1)',
];

const ROUTER_ABI = [
  'function getAmountsOut(uint256 amountIn, address[] memory path) external view returns (uint256[] memory amounts)',
  'function getAmountsIn(uint256 amountOut, address[] memory path) external view returns (uint256[] memory amounts)',
  'function getAmountOut(uint256 amountIn, uint256 reserveIn, uint256 reserveOut, uint256 fee) external pure returns (uint256 amountOut)',
  'function quote(uint256 amountA, uint256 reserveA, uint256 reserveB) external pure returns (uint256 amountB)',
  'function addLiquidity(address tokenA, address tokenB, uint256 amountADesired, uint256 amountBDesired, uint256 amountAMin, uint256 amountBMin, address to, uint256 deadline) external returns (uint256 amountA, uint256 amountB, uint256 liquidity)',
  'function addLiquidityETH(address token, uint256 amountTokenDesired, uint256 amountTokenMin, uint256 amountETHMin, address to, uint256 deadline) external payable returns (uint256 amountToken, uint256 amountETH, uint256 liquidity)',
  'function removeLiquidity(address tokenA, address tokenB, uint256 liquidity, uint256 amountAMin, uint256 amountBMin, address to, uint256 deadline) external returns (uint256 amountA, uint256 amountB)',
  'function removeLiquidityETH(address token, uint256 liquidity, uint256 amountTokenMin, uint256 amountETHMin, address to, uint256 deadline) external returns (uint256 amountToken, uint256 amountETH)',
  'function swapExactTokensForTokens(uint256 amountIn, uint256 amountOutMin, address[] calldata path, address to, uint256 deadline) external returns (uint256[] memory amounts)',
  'function swapExactETHForTokens(uint256 amountOutMin, address[] calldata path, address to, uint256 deadline) external payable returns (uint256[] memory amounts)',
  'function swapExactTokensForETH(uint256 amountIn, uint256 amountOutMin, address[] calldata path, address to, uint256 deadline) external returns (uint256[] memory amounts)',
];

const MASTERCHEF_ABI = [
  'function poolLength() external view returns (uint256)',
  'function poolInfo(uint256) external view returns (address lpToken, uint256 allocPoint, uint256 lastRewardTime, uint256 accTigerPerShare)',
  'function userInfo(uint256, address) external view returns (uint256 amount, uint256 rewardDebt)',
  'function pendingTiger(uint256 _pid, address _user) external view returns (uint256 pending)',
  'function deposit(uint256 _pid, uint256 _amount) external',
  'function withdraw(uint256 _pid, uint256 _amount) external',
  'function claim(uint256 _pid) external',
  'function emergencyWithdraw(uint256 _pid) external',
];

// ============================================================================
// Blockchain Service Class
// ============================================================================

export class BlockchainService {
  private provider: JsonRpcProvider | BrowserProvider | null = null;
  private signer: ethers.Signer | null = null;
  private chainId: number = 1;
  private contracts: Map<string, Contract> = new Map();

  constructor() {}

  // ============================================================================
  // Provider Management
  // ============================================================================

  async initialize(provider: JsonRpcProvider | BrowserProvider, chainId: number): Promise<void> {
    this.provider = provider;
    this.chainId = chainId;
    
    if ('getSigner' in provider) {
      this.signer = await provider.getSigner();
    } else {
      this.signer = null;
    }

    await this.initializeContracts();
  }

  private async initializeContracts(): Promise<void> {
    if (!this.provider) return;

    const addresses = CONTRACT_ADDRESSES[this.chainId];
    if (!addresses) {
      console.warn(`No contract addresses configured for chain ${this.chainId}`);
      return;
    }

    if (addresses.factory) {
      this.contracts.set('factory', new Contract(addresses.factory, FACTORY_ABI, this.provider));
    }
    if (addresses.router) {
      this.contracts.set('router', new Contract(addresses.router, ROUTER_ABI, this.provider));
    }
    if (addresses.masterChef) {
      this.contracts.set('masterChef', new Contract(addresses.masterChef, MASTERCHEF_ABI, this.provider));
    }
  }

  getProvider(): JsonRpcProvider | BrowserProvider | null {
    return this.provider;
  }

  getSigner(): ethers.Signer | null {
    return this.signer;
  }

  getChainId(): number {
    return this.chainId;
  }

  // ============================================================================
  // Token Operations
  // ============================================================================

  async getTokenBalance(tokenAddress: string, walletAddress: string): Promise<bigint> {
    if (!this.provider) throw new Error('Provider not initialized');
    
    const token = new Contract(tokenAddress, ERC20_ABI, this.provider);
    return await token.balanceOf(walletAddress);
  }

  async getTokenAllowance(tokenAddress: string, owner: string, spender: string): Promise<bigint> {
    if (!this.provider) throw new Error('Provider not initialized');
    
    const token = new Contract(tokenAddress, ERC20_ABI, this.provider);
    return await token.allowance(owner, spender);
  }

  async approveToken(tokenAddress: string, spender: string, amount: bigint): Promise<string> {
    if (!this.signer) throw new Error('Signer not available');
    
    const token = new Contract(tokenAddress, ERC20_ABI, this.signer);
    const tx = await token.approve(spender, amount);
    const receipt = await tx.wait();
    return receipt.hash;
  }

  async getTokenInfo(tokenAddress: string): Promise<TokenInfo | null> {
    if (!this.provider) throw new Error('Provider not initialized');
    
    try {
      const token = new Contract(tokenAddress, ERC20_ABI, this.provider);
      const [name, symbol, decimals] = await Promise.all([
        token.name(),
        token.symbol(),
        token.decimals(),
      ]);
      
      return {
        address: tokenAddress,
        name,
        symbol,
        decimals,
        chainId: this.chainId,
      };
    } catch (error) {
      console.error('Failed to get token info:', error);
      return null;
    }
  }

  // ============================================================================
  // Pool Operations
  // ============================================================================

  async getAllPools(): Promise<string[]> {
    const factory = this.contracts.get('factory');
    if (!factory) throw new Error('Factory contract not available');

    const poolCount = await factory.allPairsLength();
    const pools: string[] = [];
    
    for (let i = 0; i < poolCount; i++) {
      const pool = await factory.allPairs(i);
      pools.push(pool);
    }
    
    return pools;
  }

  async getPoolInfo(poolAddress: string): Promise<PoolInfo | null> {
    if (!this.provider) throw new Error('Provider not initialized');

    try {
      const pair = new Contract(poolAddress, PAIR_ABI, this.provider);
      
      const [token0Address, token1Address, reserves] = await Promise.all([
        pair.token0(),
        pair.token1(),
        pair.getReserves(),
      ]);

      const token0 = await this.getTokenInfo(token0Address);
      const token1 = await this.getTokenInfo(token1Address);
      
      if (!token0 || !token1) return null;

      return {
        address: poolAddress,
        token0,
        token1,
        reserve0: reserves[0],
        reserve1: reserves[1],
        fee: 300,
        liquidity: reserves[0] + reserves[1],
        volume24h: 0,
        apr: 0,
      };
    } catch (error) {
      console.error('Failed to get pool info:', error);
      return null;
    }
  }

  async getPoolFromFactory(token0: string, token1: string): Promise<string | null> {
    const factory = this.contracts.get('factory');
    if (!factory) throw new Error('Factory contract not available');

    try {
      const pool = await factory.getPair(token0, token1);
      return pool === '0x0000000000000000000000000000000000000000' ? null : pool;
    } catch {
      return null;
    }
  }

  // ============================================================================
  // Swap Operations
  // ============================================================================

  async getSwapQuote(
    tokenIn: string,
    tokenOut: string,
    amountIn: bigint,
    slippage: number = 0.5
  ): Promise<{ amountOut: bigint; path: string[]; priceImpact: number } | null> {
    const router = this.contracts.get('router');
    if (!router) throw new Error('Router contract not available');

    try {
      const path = [tokenIn, tokenOut];
      const amounts = await router.getAmountsOut(amountIn, path);
      const amountOut = amounts[amounts.length - 1];
      
      const reserves = await this.getPoolReserves(tokenIn, tokenOut);
      if (!reserves) return null;

      const priceImpact = this.calculatePriceImpact(
        amountIn,
        amountOut,
        reserves.reserveIn,
        reserves.reserveOut
      );

      return { amountOut, path, priceImpact };
    } catch (error) {
      console.error('Failed to get swap quote:', error);
      return null;
    }
  }

  async executeSwap(
    tokenIn: string,
    tokenOut: string,
    amountIn: bigint,
    amountOutMin: bigint,
    deadline: number = 20 * 60
  ): Promise<SwapResult> {
    if (!this.signer) return { success: false, error: 'Signer not available' };

    const router = this.contracts.get('router');
    if (!router) return { success: false, error: 'Router contract not available' };

    try {
      const path = [tokenIn, tokenOut];
      const signerAddr = await this.signer.getAddress();
      
      const allowance = await this.getTokenAllowance(tokenIn, signerAddr, await router.getAddress());
      if (allowance < amountIn) {
        await this.approveToken(tokenIn, await router.getAddress(), ethers.MaxUint256);
      }

      const routerWithSigner = router.connect(this.signer);
      const tx = await routerWithSigner.swapExactTokensForTokens(
        amountIn,
        amountOutMin,
        path,
        signerAddr,
        Math.floor(Date.now() / 1000) + deadline
      );

      const receipt = await tx.wait();
      
      return { success: true, txHash: receipt.hash };
    } catch (error: any) {
      console.error('Swap failed:', error);
      return { success: false, error: error.message || 'Swap failed' };
    }
  }

  // ============================================================================
  // Liquidity Operations
  // ============================================================================

  async addLiquidity(
    token0: string,
    token1: string,
    amount0: bigint,
    amount1: bigint,
    amount0Min: bigint = 0n,
    amount1Min: bigint = 0n,
    deadline: number = 20 * 60
  ): Promise<SwapResult> {
    if (!this.signer) return { success: false, error: 'Signer not available' };

    const router = this.contracts.get('router');
    if (!router) return { success: false, error: 'Router contract not available' };

    try {
      const routerWithSigner = router.connect(this.signer);
      const signerAddr = await this.signer.getAddress();
      
      const allowance0 = await this.getTokenAllowance(token0, signerAddr, await router.getAddress());
      const allowance1 = await this.getTokenAllowance(token1, signerAddr, await router.getAddress());

      if (allowance0 < amount0) {
        await this.approveToken(token0, await router.getAddress(), ethers.MaxUint256);
      }
      if (allowance1 < amount1) {
        await this.approveToken(token1, await router.getAddress(), ethers.MaxUint256);
      }

      const tx = await routerWithSigner.addLiquidity(
        token0, token1, amount0, amount1,
        amount0Min, amount1Min,
        signerAddr,
        Math.floor(Date.now() / 1000) + deadline
      );

      const receipt = await tx.wait();
      return { success: true, txHash: receipt.hash };
    } catch (error: any) {
      console.error('Add liquidity failed:', error);
      return { success: false, error: error.message || 'Add liquidity failed' };
    }
  }

  async removeLiquidity(
    token0: string,
    token1: string,
    liquidity: bigint,
    amount0Min: bigint = 0n,
    amount1Min: bigint = 0n,
    deadline: number = 20 * 60
  ): Promise<SwapResult> {
    if (!this.signer) return { success: false, error: 'Signer not available' };

    const router = this.contracts.get('router');
    if (!router) return { success: false, error: 'Router contract not available' };

    try {
      const routerWithSigner = router.connect(this.signer);
      const signerAddr = await this.signer.getAddress();

      const tx = await routerWithSigner.removeLiquidity(
        token0, token1, liquidity,
        amount0Min, amount1Min,
        signerAddr,
        Math.floor(Date.now() / 1000) + deadline
      );

      const receipt = await tx.wait();
      return { success: true, txHash: receipt.hash };
    } catch (error: any) {
      console.error('Remove liquidity failed:', error);
      return { success: false, error: error.message || 'Remove liquidity failed' };
    }
  }

  // ============================================================================
  // Farming Operations
  // ============================================================================

  async getFarmingPositions(walletAddress: string): Promise<FarmPosition[]> {
    const masterChef = this.contracts.get('masterChef');
    if (!masterChef) return [];

    try {
      const poolCount = await masterChef.poolLength();
      const positions: FarmPosition[] = [];

      for (let i = 0; i < poolCount; i++) {
        const [poolInfo, userInfo] = await Promise.all([
          masterChef.poolInfo(i),
          masterChef.userInfo(i, walletAddress),
        ]);

        if (userInfo.amount > 0n) {
          const pendingReward = await masterChef.pendingTiger(i, walletAddress);
          
          positions.push({
            pid: i,
            poolAddress: poolInfo.lpToken,
            lpToken: poolInfo.lpToken,
            amount: userInfo.amount,
            pendingReward,
            poolShare: 0,
            apr: 0,
          });
        }
      }

      return positions;
    } catch (error) {
      console.error('Failed to get farming positions:', error);
      return [];
    }
  }

  async depositToFarm(pid: number, amount: bigint): Promise<SwapResult> {
    if (!this.signer) return { success: false, error: 'Signer not available' };

    const masterChef = this.contracts.get('masterChef');
    if (!masterChef) return { success: false, error: 'MasterChef not available' };

    try {
      const masterChefWithSigner = masterChef.connect(this.signer);
      const tx = await masterChefWithSigner.deposit(pid, amount);
      const receipt = await tx.wait();
      return { success: true, txHash: receipt.hash };
    } catch (error: any) {
      return { success: false, error: error.message || 'Deposit failed' };
    }
  }

  async withdrawFromFarm(pid: number, amount: bigint): Promise<SwapResult> {
    if (!this.signer) return { success: false, error: 'Signer not available' };

    const masterChef = this.contracts.get('masterChef');
    if (!masterChef) return { success: false, error: 'MasterChef not available' };

    try {
      const masterChefWithSigner = masterChef.connect(this.signer);
      const tx = await masterChefWithSigner.withdraw(pid, amount);
      const receipt = await tx.wait();
      return { success: true, txHash: receipt.hash };
    } catch (error: any) {
      return { success: false, error: error.message || 'Withdraw failed' };
    }
  }

  async claimFarmRewards(pid: number): Promise<SwapResult> {
    if (!this.signer) return { success: false, error: 'Signer not available' };

    const masterChef = this.contracts.get('masterChef');
    if (!masterChef) return { success: false, error: 'MasterChef not available' };

    try {
      const masterChefWithSigner = masterChef.connect(this.signer);
      const tx = await masterChefWithSigner.claim(pid);
      const receipt = await tx.wait();
      return { success: true, txHash: receipt.hash };
    } catch (error: any) {
      return { success: false, error: error.message || 'Claim failed' };
    }
  }

  // ============================================================================
  // Helper Functions
  // ============================================================================

  private async getPoolReserves(tokenA: string, tokenB: string): Promise<{ reserveIn: bigint; reserveOut: bigint } | null> {
    const factory = this.contracts.get('factory');
    if (!factory) return null;

    try {
      const pool = await factory.getPair(tokenA, tokenB);
      if (pool === '0x0000000000000000000000000000000000000000') return null;

      const pair = new Contract(pool, PAIR_ABI, this.provider);
      const [token0, reserves] = await Promise.all([
        pair.token0(),
        pair.getReserves(),
      ]);

      const isReversed = tokenA.toLowerCase() < tokenB.toLowerCase();
      return {
        reserveIn: isReversed ? reserves[1] : reserves[0],
        reserveOut: isReversed ? reserves[0] : reserves[1],
      };
    } catch {
      return null;
    }
  }

  private calculatePriceImpact(amountIn: bigint, amountOut: bigint, reserveIn: bigint, reserveOut: bigint): number {
    if (amountIn === 0n || reserveIn === 0n) return 0;
    
    const spotPrice = Number(reserveOut) / Number(reserveIn);
    const executionPrice = Number(amountOut) / Number(amountIn);
    
    return Math.max(0, ((spotPrice - executionPrice) / spotPrice) * 100);
  }

  async estimateGas(transaction: any): Promise<bigint> {
    if (!this.provider || !this.signer) throw new Error('Provider not initialized');
    
    const gasEstimate = await this.provider.estimateGas(transaction);
    return (gasEstimate * 120n) / 100n;
  }

  async getGasPrice(): Promise<{ baseFee: bigint; priorityFee: bigint; maxFee: bigint }> {
    if (!this.provider) throw new Error('Provider not initialized');

    const feeData = await this.provider.getFeeData();
    
    return {
      baseFee: feeData.baseFeePerGas || 0n,
      priorityFee: feeData.maxPriorityFeePerGas || 0n,
      maxFee: feeData.maxFeePerGas || 0n,
    };
  }
}

export const blockchainService = new BlockchainService();
export default BlockchainService;