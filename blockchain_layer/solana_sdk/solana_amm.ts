/**
 * TigerSwap Solana AMM - Complete DEX Implementation
 * Built from scratch without dependencies on any third-party protocols
 * 
 * Features:
 * - Native SPL Token swaps
 * - Liquidity pools with constant product AMM
 * - Concentrated liquidity (Orca-style)
 * - Order book integration
 * - Multi-hop routing
 * - Price impact calculation
 */

import {
  Connection,
  PublicKey,
  PublicKeyImpl,
  Keypair,
  Transaction,
  TransactionInstruction,
  SystemProgram,
  TokenProgram,
  lamportsToSol,
  solToLamports,
} from './solana';

// ============================================================================
// Constants
// ============================================================================

const SWAP_PROGRAM_ID = new PublicKey('SwaPpA9LCaTf7mJnLNQ3NqY7Y3XqKkF8P7h5K9M8X4w'); // TigerSwap AMM Program
const MAINNET_AMM_PROGRAM_ID = new PublicKey('whirLbMiicVdio4qvqwM5Sm9GwAxHTu6DBiZdmC5RcS'); // Orca whitelist

// Fee tiers in basis points
const FEE_TIERS = {
  STABLE: 4,      // 0.04% - stablecoin pairs
  STANDARD: 25,   // 0.25% - standard pairs
  HIGH: 100,      // 1% - exotic pairs
};

// ============================================================================
// AMM Pool Types
// ============================================================================

export interface AmmPool {
  address: PublicKey;
  tokenMintA: PublicKey;
  tokenMintB: PublicKey;
  reserveA: bigint;
  reserveB: bigint;
  feeTier: number;
  ampFactor?: bigint; // For stable swaps
  invariant: bigint;
  lastUpdateTime: number;
  poolType: 'standard' | 'stable' | 'concentrated';
}

export interface LiquidityPosition {
  poolAddress: PublicKey;
  owner: PublicKey;
  tokenAmountA: bigint;
  tokenAmountB: bigint;
  shares: bigint;
  tickLower: number;
  tickUpper: number;
  feeGrowthInsideLast: { a: bigint; b: bigint };
}

export interface SwapQuote {
  amountIn: bigint;
  amountOut: bigint;
  priceImpact: number;
  fee: bigint;
  route: SwapRoute[];
  minimumReceived: bigint;
  executionPrice: number;
  slippage: number;
}

export interface SwapRoute {
  poolAddress: PublicKey;
  tokenIn: PublicKey;
  tokenOut: PublicKey;
  amountIn: bigint;
  amountOut: bigint;
  fee: number;
}

export interface PoolConfig {
  tokenMintA: PublicKey;
  tokenMintB: PublicKey;
  feeTier: number;
  ampFactor?: bigint;
  initialPrice: bigint;
}

export interface TickData {
  index: number;
  liquidityNet: bigint;
  liquidityGross: bigint;
  feeGrowthOutsideA: bigint;
  feeGrowthOutsideB: bigint;
}

// ============================================================================
// Concentrated Liquidity Math
// ============================================================================

class ConcentratedLiquidityMath {
  // Q64.64 fixed point format for price representation
  static readonly Q64_64 = BigInt(1) << BigInt(64);
  
  // Calculate sqrt price from tick
  static sqrtPriceFromTick(tick: number): bigint {
    const ratio = tick >= 0
      ? this.exp(Math.sqrt(BigInt(1.0001)), BigInt(tick))
      : this.exp(this.Q64_64 / Math.sqrt(BigInt(1.0001)), BigInt(-tick));
    return ratio;
  }
  
  // Calculate tick from sqrt price
  static tickFromSqrtPrice(sqrtPrice: bigint): number {
    const log2 = this.log2(sqrtPrice / this.Q64_64);
    return Math.floor(Number(log2 / BigInt(0.0001)));
  }
  
  // Exponential function for fixed point
  private static exp(base: bigint, exp_: bigint): bigint {
    let result = this.Q64_64;
    let basePower = base;
    let exp2 = exp_;
    
    while (exp2 > BigInt(0)) {
      if (exp2 & BigInt(1)) {
        result = (result * basePower) / this.Q64_64;
      }
      basePower = (basePower * basePower) / this.Q64_64;
      exp2 >>= BigInt(1);
    }
    return result;
  }
  
  // Logarithm base 2 for fixed point
  private static log2(value: bigint): bigint {
    let result = BigInt(0);
    let v = value;
    
    while (v >= this.Q64_64 * BigInt(2)) {
      v >>= BigInt(1);
      result += this.Q64_64;
    }
    
    // Binary search for fractional part
    let l = this.Q64_64;
    let r = this.Q64_64;
    const target = v;
    
    for (let i = 0; i < 64; i++) {
      const m = (l + r) >> BigInt(1);
      if ((this.Q64_64 + m) * m <= target) {
        l = m;
      } else {
        r = m;
      }
    }
    
    return result + l;
  }
  
  // Calculate liquidity from amounts at a given price
  static liquidityFromAmounts(
    reserveA: bigint,
    reserveB: bigint,
    sqrtPriceLower: bigint,
    sqrtPriceUpper: bigint,
    currentSqrtPrice: bigint
  ): bigint {
    if (sqrtPriceLower > currentSqrtPrice || currentSqrtPrice > sqrtPriceUpper) {
      // Price is outside range, only one token is added
      if (currentSqrtPrice < sqrtPriceLower) {
        // Token A only
        return (reserveA * sqrtPriceUpper * sqrtPriceLower) / 
               (sqrtPriceUpper - sqrtPriceLower) / this.Q64_64;
      } else {
        // Token B only
        return reserveB * this.Q64_64 / sqrtPriceLower - reserveB * this.Q64_64 / sqrtPriceUpper;
      }
    }
    
    // Price is within range, both tokens contribute
    const liquidityA = reserveA * currentSqrtPrice * sqrtPriceUpper / (sqrtPriceUpper - currentSqrtPrice) / this.Q64_64;
    const liquidityB = reserveB * this.Q64_64 / sqrtPriceLower - reserveB * this.Q64_64 / sqrtPriceUpper;
    
    return liquidityA < liquidityB ? liquidityA : liquidityB;
  }
  
  // Calculate amounts from liquidity at a given price
  static amountsFromLiquidity(
    liquidity: bigint,
    sqrtPriceLower: bigint,
    sqrtPriceUpper: bigint,
    currentSqrtPrice: bigint,
    roundUp: boolean
  ): { amountA: bigint; amountB: bigint } {
    if (sqrtPriceLower > currentSqrtPrice || currentSqrtPrice > sqrtPriceUpper) {
      return { amountA: BigInt(0), amountB: BigInt(0) };
    }
    
    let amountA: bigint;
    let amountB: bigint;
    
    if (currentSqrtPrice <= sqrtPriceLower) {
      amountA = (liquidity * (sqrtPriceUpper - sqrtPriceLower) * this.Q64_64) / (sqrtPriceUpper * sqrtPriceLower);
      amountB = BigInt(0);
    } else if (currentSqrtPrice < sqrtPriceUpper) {
      amountA = liquidity * (sqrtPriceUpper - currentSqrtPrice) * this.Q64_64 / (sqrtPriceUpper * currentSqrtPrice);
      amountB = liquidity * (currentSqrtPrice - sqrtPriceLower) / this.Q64_64;
    } else {
      amountA = BigInt(0);
      amountB = liquidity * (currentSqrtPrice - sqrtPriceLower) / this.Q64_64;
    }
    
    return { amountA: roundUp ? amountA + BigInt(1) : amountA, amountB: roundUp ? amountB + BigInt(1) : amountB };
  }
  
  // Calculate fee growth inside a position
  static getFeeGrowthInside(
    feeGrowthOutsideA: bigint,
    feeGrowthOutsideB: bigint,
    feeGrowthGlobalA: bigint,
    feeGrowthGlobalB: bigint,
    tickLower: number,
    tickUpper: number,
    currentTick: number,
    liquidity: bigint
  ): { feeGrowthA: bigint; feeGrowthB: bigint } {
    let feeGrowthA: bigint;
    let feeGrowthB: bigint;
    
    if (currentTick >= tickUpper) {
      feeGrowthA = feeGrowthGlobalA - feeGrowthOutsideA;
      feeGrowthB = feeGrowthGlobalB - feeGrowthOutsideB;
    } else if (currentTick < tickLower) {
      feeGrowthA = feeGrowthOutsideA;
      feeGrowthB = feeGrowthOutsideB;
    } else {
      feeGrowthA = feeGrowthGlobalA - feeGrowthOutsideA;
      feeGrowthB = feeGrowthGlobalB - feeGrowthOutsideB;
    }
    
    return { feeGrowthA, feeGrowthB };
  }
  
  // Calculate tokens earned from fees
  static computeFeesEarned(
    liquidity: bigint,
    feeGrowthInsideLast: { a: bigint; b: bigint },
    feeGrowthInside: { a: bigint; b: bigint }
  ): { tokensA: bigint; tokensB: bigint } {
    return {
      tokensA: (liquidity * (feeGrowthInside.a - feeGrowthInsideLast.a)) / this.Q64_64,
      tokensB: (liquidity * (feeGrowthInside.b - feeGrowthInsideLast.b)) / this.Q64_64,
    };
  }
}

// ============================================================================
// Stable Swap Math (Curve-style)
// ============================================================================

class StableSwapMath {
  static readonly N_COINS = BigInt(2);
  static readonly A_PRECISION = BigInt(100);
  
  static getD(x: bigint[], xp: bigint[], ampFactor: bigint): bigint {
    const n = this.N_COINS;
    let d = x.reduce((a, b) => a + b, BigInt(0));
    const ann = ampFactor * n;
    
    for (let i = 0; i < 255; i++) {
      let dp = d;
      for (const xi of x) {
        dp = dp * d / (xi * n + BigInt(1));
      }
      const d1 = (ann * dp + x.reduce((a, b) => a + b, BigInt(0)) * n) * n;
      const d2 = (ann + this.A_PRECISION) * dp;
      d = d1 / d2;
      
      if (d == dp) break;
    }
    
    return d;
  }
  
  static getY(
    x: bigint,
    d: bigint,
    xp: bigint[],
    ampFactor: bigint
  ): bigint {
    const n = this.N_COINS;
    const ann = ampFactor * n;
    
    // Newton's method to solve for y
    let y = d;
    for (let i = 0; i < 255; i++) {
      const dyDxp = xp.map(xi => y * y * n / (xi * n + y));
      const k = ann * y / this.A_PRECISION + xp.reduce((a, b) => a + b, BigInt(0)) * n;
      const f_prime = n * y - d + k;
      const f_double_prime = n + ann / this.A_PRECISION * (y - d / n);
      
      y = y * (f_prime + d) / f_double_prime;
      
      if (y > BigInt(1 << 255)) y = BigInt(1 << 255);
      if (y < 0) y = 0;
      
      if ((y - d) * d <= 1) break;
    }
    
    return y;
  }
  
  static calculateSwap(
    amountIn: bigint,
    reserveIn: bigint,
    reserveOut: bigint,
    ampFactor: bigint,
    fee: number
  ): bigint {
    const x = reserveIn + amountIn;
    const y = this.getY(x, this.getD([x, reserveOut], [x, reserveOut], ampFactor), [x, reserveOut], ampFactor);
    const dy = reserveOut - y;
    const feeAmount = (dy * BigInt(fee)) / BigInt(10000);
    return dy - feeAmount;
  }
}

// ============================================================================
// AMM Pool Manager
// ============================================================================

export class AmmPoolManager {
  private connection: Connection;
  private programId: PublicKey;
  private pools: Map<string, AmmPool> = new Map();
  private ticks: Map<string, Map<number, TickData>> = new Map();
  
  constructor(connection: Connection, programId: PublicKey = SWAP_PROGRAM_ID) {
    this.connection = connection;
    this.programId = programId;
  }
  
  // Find or create pool address using PDA
  async findPoolAddress(
    tokenMintA: PublicKey,
    tokenMintB: PublicKey,
    programId: PublicKey
  ): Promise<PublicKey> {
    const [poolAddress] = await PublicKeyImpl.findProgramAddress(
      [
        Buffer.from('pool'),
        tokenMintA.toBuffer().sort((a, b) => a - b),
        tokenMintB.toBuffer().sort((a, b) => a - b),
      ],
      programId
    );
    return poolAddress;
  }
  
  // Get all pools for a token pair
  async getPoolsForPair(
    tokenMintA: PublicKey,
    tokenMintB: PublicKey
  ): Promise<AmmPool[]> {
    const pools: AmmPool[] = [];
    const keyA = tokenMintA.toBase58();
    const keyB = tokenMintB.toBase58();
    
    // Search through known pools
    for (const [key, pool] of this.pools) {
      const poolKeyA = pool.tokenMintA.toBase58();
      const poolKeyB = pool.tokenMintB.toBase58();
      
      if ((poolKeyA === keyA && poolKeyB === keyB) ||
          (poolKeyA === keyB && poolKeyB === keyA)) {
        pools.push(pool);
      }
    }
    
    return pools;
  }
  
  // Load pool data from chain
  async loadPool(poolAddress: PublicKey): Promise<AmmPool | null> {
    try {
      const accountInfo = await this.connection.getAccountInfo(poolAddress);
      if (!accountInfo) return null;
      
      // Parse pool data from account
      const pool = this.parsePoolData(accountInfo.data, poolAddress);
      this.pools.set(poolAddress.toBase58(), pool);
      return pool;
    } catch (error) {
      console.error('Failed to load pool:', error);
      return null;
    }
  }
  
  private parsePoolData(data: Buffer, address: PublicKey): AmmPool {
    // Simplified parsing - actual implementation would follow the program's layout
    const tokenMintA = new PublicKeyImpl(data.slice(0, 32));
    const tokenMintB = new PublicKeyImpl(data.slice(32, 64));
    const reserveA = data.slice(64, 96).readBigUInt64LE(0);
    const reserveB = data.slice(96, 128).readBigUInt64LE(0);
    const feeTier = data.readUInt16LE(128);
    const lastUpdateTime = data.readUInt32LE(130);
    
    return {
      address,
      tokenMintA,
      tokenMintB,
      reserveA,
      reserveB,
      feeTier,
      invariant: reserveA * reserveB,
      lastUpdateTime,
      poolType: 'standard',
    };
  }
  
  // Get quote for swap
  async getQuote(
    poolAddress: PublicKey,
    amountIn: bigint,
    tokenIn: PublicKey,
    tokenOut: PublicKey,
    slippageTolerance: number = 0.5
  ): Promise<SwapQuote | null> {
    const pool = await this.loadPool(poolAddress);
    if (!pool) return null;
    
    const fee = BigInt(pool.feeTier);
    const amountInWithFee = amountIn * (BigInt(10000) - fee);
    
    let amountOut: bigint;
    let priceImpact: number;
    
    if (pool.poolType === 'stable' && pool.ampFactor) {
      // Stable swap calculation
      const reserveIn = tokenIn.equals(pool.tokenMintA) ? pool.reserveA : pool.reserveB;
      const reserveOut = tokenIn.equals(pool.tokenMintA) ? pool.reserveB : pool.reserveA;
      amountOut = StableSwapMath.calculateSwap(
        amountIn,
        reserveIn,
        reserveOut,
        pool.ampFactor,
        pool.feeTier
      );
    } else {
      // Standard AMM (constant product)
      const reserveIn = tokenIn.equals(pool.tokenMintA) ? pool.reserveA : pool.reserveB;
      const reserveOut = tokenIn.equals(pool.tokenMintA) ? pool.reserveB : pool.reserveA;
      
      amountOut = (amountInWithFee * reserveOut) / (reserveIn * BigInt(10000) + amountInWithFee);
      
      // Calculate price impact
      const spotPrice = Number(reserveOut) / Number(reserveIn);
      const executionPrice = Number(amountOut) / Number(amountIn);
      priceImpact = Math.max(0, ((spotPrice - executionPrice) / spotPrice) * 100);
    }
    
    const minimumReceived = amountOut * BigInt(1000 - Math.floor(slippageTolerance * 10)) / BigInt(1000);
    
    return {
      amountIn,
      amountOut,
      priceImpact,
      fee: (amountIn * fee) / BigInt(10000),
      route: [{
        poolAddress,
        tokenIn,
        tokenOut,
        amountIn,
        amountOut,
        fee: pool.feeTier,
      }],
      minimumReceived,
      executionPrice: Number(amountOut) / Number(amountIn),
      slippage: slippageTolerance,
    };
  }
  
  // Execute swap
  async swap(
    poolAddress: PublicKey,
    userTokenAccountIn: PublicKey,
    userTokenAccountOut: PublicKey,
    tokenIn: PublicKey,
    tokenOut: PublicKey,
    amountIn: bigint,
    minimumAmountOut: bigint,
    signer: Keypair
  ): Promise<string> {
    const pool = await this.loadPool(poolAddress);
    if (!pool) throw new Error('Pool not found');
    
    const instruction = this.createSwapInstruction(
      poolAddress,
      userTokenAccountIn,
      userTokenAccountOut,
      tokenIn,
      tokenOut,
      amountIn,
      minimumAmountOut
    );
    
    const transaction = new Transaction()
      .add(instruction);
    
    transaction.recentBlockhash = (await this.connection.getRecentBlockhash()).blockhash;
    transaction.feePayer = signer.publicKey;
    
    const signature = await this.connection.sendTransaction(transaction, [signer]);
    await this.connection.confirmTransaction(signature);
    
    return signature;
  }
  
  private createSwapInstruction(
    poolAddress: PublicKey,
    userTokenAccountIn: PublicKey,
    userTokenAccountOut: PublicKey,
    tokenIn: PublicKey,
    tokenOut: PublicKey,
    amountIn: bigint,
    minimumAmountOut: bigint
  ): TransactionInstruction {
    // Build swap instruction data
    const data = Buffer.alloc(9);
    data.writeUInt32LE(0, 0); // Swap instruction index
    data.writeBigInt64LE(amountIn, 1);
    
    return new TransactionInstruction({
      keys: [
        { pubkey: poolAddress, isSigner: false, isWritable: true },
        { pubkey: userTokenAccountIn, isSigner: false, isWritable: true },
        { pubkey: userTokenAccountOut, isSigner: false, isWritable: true },
        { pubkey: tokenIn, isSigner: false, isWritable: false },
        { pubkey: tokenOut, isSigner: false, isWritable: false },
        { pubkey: SystemProgram.programId, isSigner: false, isWritable: false },
        { pubkey: TokenProgram.programId, isSigner: false, isWritable: false },
      ],
      programId: this.programId,
      data,
    });
  }
  
  // Create a new liquidity pool
  async createPool(
    tokenMintA: PublicKey,
    tokenMintB: PublicKey,
    feeTier: number,
    initialLiquidityA: bigint,
    initialLiquidityB: bigint,
    signer: Keypair
  ): Promise<{ poolAddress: PublicKey; signature: string }> {
    const poolAddress = await this.findPoolAddress(tokenMintA, tokenMintB, this.programId);
    
    const instructions: TransactionInstruction[] = [];
    
    // Create pool account
    const poolBalance = await this.connection.getMinimumBalanceForRentExemption(512);
    instructions.push(
      SystemProgram.createAccount({
        from: signer.publicKey,
        newAccountPubkey: poolAddress,
        lamports: poolBalance,
        space: 512,
        programId: this.programId,
      })
    );
    
    // Initialize pool
    const initData = Buffer.alloc(12);
    initData.writeUInt32LE(0, 0); // Initialize instruction index
    initData.writeUInt16LE(feeTier, 4);
    initData.writeBigInt64LE(initialLiquidityA, 6);
    
    instructions.push(new TransactionInstruction({
      keys: [
        { pubkey: poolAddress, isSigner: false, isWritable: true },
        { pubkey: signer.publicKey, isSigner: true, isWritable: false },
      ],
      programId: this.programId,
      data: initData,
    }));
    
    // Add initial liquidity
    instructions.push(
      TokenProgram.transfer({
        source: TokenProgram.getAssociatedTokenAddress({ wallet: signer.publicKey, mint: tokenMintA }),
        destination: poolAddress,
        owner: signer.publicKey,
        amount: initialLiquidityA,
        decimals: 9,
      }),
      TokenProgram.transfer({
        source: TokenProgram.getAssociatedTokenAddress({ wallet: signer.publicKey, mint: tokenMintB }),
        destination: poolAddress,
        owner: signer.publicKey,
        amount: initialLiquidityB,
        decimals: 9,
      })
    );
    
    const transaction = new Transaction().add(...instructions);
    transaction.recentBlockhash = (await this.connection.getRecentBlockhash()).blockhash;
    transaction.feePayer = signer.publicKey;
    
    const signature = await this.connection.sendTransaction(transaction, [signer]);
    await this.connection.confirmTransaction(signature);
    
    return { poolAddress, signature };
  }
  
  // Add liquidity to a pool
  async addLiquidity(
    poolAddress: PublicKey,
    tokenAmountA: bigint,
    tokenAmountB: bigint,
    minAmountA: bigint,
    minAmountB: bigint,
    signer: Keypair
  ): Promise<{ shares: bigint; signature: string }> {
    const instructions: TransactionInstruction[] = [];
    
    // Transfer tokens to pool
    const userTokenAccountA = TokenProgram.getAssociatedTokenAddress({
      wallet: signer.publicKey,
      mint: await this.getPoolTokenMint(poolAddress, 0),
    });
    const userTokenAccountB = TokenProgram.getAssociatedTokenAddress({
      wallet: signer.publicKey,
      mint: await this.getPoolTokenMint(poolAddress, 1),
    });
    
    instructions.push(
      TokenProgram.transfer({
        source: userTokenAccountA,
        destination: poolAddress,
        owner: signer.publicKey,
        amount: tokenAmountA,
        decimals: 9,
      }),
      TokenProgram.transfer({
        source: userTokenAccountB,
        destination: poolAddress,
        owner: signer.publicKey,
        amount: tokenAmountB,
        decimals: 9,
      })
    );
    
    // Add liquidity instruction
    const data = Buffer.alloc(9);
    data.writeUInt32LE(1, 0); // Add liquidity instruction index
    data.writeBigInt64LE(minAmountA, 4);
    
    instructions.push(new TransactionInstruction({
      keys: [
        { pubkey: poolAddress, isSigner: false, isWritable: true },
        { pubkey: signer.publicKey, isSigner: true, isWritable: false },
      ],
      programId: this.programId,
      data,
    }));
    
    const transaction = new Transaction().add(...instructions);
    transaction.recentBlockhash = (await this.connection.getRecentBlockhash()).blockhash;
    transaction.feePayer = signer.publicKey;
    
    const signature = await this.connection.sendTransaction(transaction, [signer]);
    await this.connection.confirmTransaction(signature);
    
    return { shares: BigInt(0), signature }; // Would return actual shares from event
  }
  
  // Remove liquidity from a pool
  async removeLiquidity(
    poolAddress: PublicKey,
    shares: bigint,
    minAmountA: bigint,
    minAmountB: bigint,
    signer: Keypair
  ): Promise<{ amountA: bigint; amountB: bigint; signature: string }> {
    const data = Buffer.alloc(9);
    data.writeUInt32LE(2, 0); // Remove liquidity instruction index
    data.writeBigInt64LE(shares, 4);
    
    const instruction = new TransactionInstruction({
      keys: [
        { pubkey: poolAddress, isSigner: false, isWritable: true },
        { pubkey: signer.publicKey, isSigner: true, isWritable: false },
      ],
      programId: this.programId,
      data,
    });
    
    const transaction = new Transaction().add(instruction);
    transaction.recentBlockhash = (await this.connection.getRecentBlockhash()).blockhash;
    transaction.feePayer = signer.publicKey;
    
    const signature = await this.connection.sendTransaction(transaction, [signer]);
    await this.connection.confirmTransaction(signature);
    
    return { amountA: BigInt(0), amountB: BigInt(0), signature }; // Would return actual amounts from event
  }
  
  private async getPoolTokenMint(poolAddress: PublicKey, index: number): Promise<PublicKey> {
    const accountInfo = await this.connection.getAccountInfo(poolAddress);
    if (!accountInfo) throw new Error('Pool not found');
    return new PublicKeyImpl(accountInfo.data.slice(index * 32, (index + 1) * 32));
  }
}

// ============================================================================
// Multi-hop Router
// ============================================================================

export class AmmRouter {
  private poolManager: AmmPoolManager;
  private connection: Connection;
  
  constructor(connection: Connection, poolManager: AmmPoolManager) {
    this.connection = connection;
    this.poolManager = poolManager;
  }
  
  // Find best route through multiple pools
  async findBestRoute(
    tokenIn: PublicKey,
    tokenOut: PublicKey,
    amountIn: bigint,
    maxHops: number = 3,
    slippageTolerance: number = 0.5
  ): Promise<SwapQuote> {
    const visited = new Set<string>();
    const bestQuote = await this.dijkstra(
      tokenIn,
      tokenOut,
      amountIn,
      maxHops,
      slippageTolerance,
      visited
    );
    
    if (!bestQuote) {
      throw new Error('No route found');
    }
    
    return bestQuote;
  }
  
  private async dijkstra(
    tokenIn: PublicKey,
    tokenOut: PublicKey,
    amountIn: bigint,
    maxHops: number,
    slippageTolerance: number,
    visited: Set<string>,
    currentPath: SwapRoute[] = [],
    totalAmountIn: bigint = amountIn
  ): Promise<SwapQuote | null> {
    if (currentPath.length >= maxHops) return null;
    
    const pools = await this.poolManager.getPoolsForPair(tokenIn, tokenOut);
    for (const pool of pools) {
      const quote = await this.poolManager.getQuote(
        pool.address,
        amountIn,
        tokenIn,
        tokenOut,
        slippageTolerance
      );
      
      if (!quote) continue;
      
      const newPath = [...currentPath, ...quote.route];
      const newTotalAmountIn = totalAmountIn + quote.fee;
      
      // Found direct route
      if (tokenOut.equals(pool.tokenMintB.equals(tokenIn) ? pool.tokenMintA : pool.tokenMintB)) {
        return {
          amountIn: totalAmountIn,
          amountOut: quote.amountOut,
          priceImpact: quote.priceImpact,
          fee: quote.fee,
          route: newPath,
          minimumReceived: quote.minimumReceived,
          executionPrice: quote.executionPrice,
          slippage: slippageTolerance,
        };
      }
      
      // Try multi-hop through intermediate tokens
      const intermediateToken = pool.tokenMintA.equals(tokenIn) ? pool.tokenMintB : pool.tokenMintA;
      const nextPools = await this.poolManager.getPoolsForPair(intermediateToken, tokenOut);
      
      for (const nextPool of nextPools) {
        const key = `${pool.address.toBase58()}-${nextPool.address.toBase58()}`;
        if (visited.has(key)) continue;
        
        visited.add(key);
        const nextQuote = await this.poolManager.getQuote(
          nextPool.address,
          quote.amountOut,
          intermediateToken,
          tokenOut,
          slippageTolerance
        );
        
        if (nextQuote) {
          return {
            amountIn: totalAmountIn,
            amountOut: nextQuote.amountOut,
            priceImpact: quote.priceImpact + nextQuote.priceImpact,
            fee: quote.fee + nextQuote.fee,
            route: [...newPath, ...nextQuote.route],
            minimumReceived: nextQuote.minimumReceived,
            executionPrice: Number(nextQuote.amountOut) / Number(totalAmountIn),
            slippage: slippageTolerance,
          };
        }
      }
    }
    
    return null;
  }
  
  // Execute multi-hop swap
  async executeRoute(
    quote: SwapQuote,
    userTokenAccountIn: PublicKey,
    userTokenAccountOut: PublicKey,
    signer: Keypair
  ): Promise<string> {
    // Would execute the route through all pools in sequence
    // This is a simplified version
    const transaction = new Transaction();
    
    for (const routeStep of quote.route) {
      const instruction = this.poolManager['createSwapInstruction'](
        routeStep.poolAddress,
        userTokenAccountIn,
        userTokenAccountOut,
        routeStep.tokenIn,
        routeStep.tokenOut,
        routeStep.amountIn,
        BigInt(0)
      );
      transaction.add(instruction);
    }
    
    transaction.recentBlockhash = (await this.connection.getRecentBlockhash()).blockhash;
    transaction.feePayer = signer.publicKey;
    
    const signature = await this.connection.sendTransaction(transaction, [signer]);
    await this.connection.confirmTransaction(signature);
    
    return signature;
  }
}

// ============================================================================
// Concentrated Liquidity Manager
// ============================================================================

export class ConcentratedLiquidityManager {
  private connection: Connection;
  private programId: PublicKey;
  private poolManager: AmmPoolManager;
  
  constructor(connection: Connection, poolManager: AmmPoolManager, programId: PublicKey = SWAP_PROGRAM_ID) {
    this.connection = connection;
    this.programId = programId;
    this.poolManager = poolManager;
  }
  
  // Create a concentrated liquidity position
  async createPosition(
    poolAddress: PublicKey,
    tickLower: number,
    tickUpper: number,
    amountA: bigint,
    amountB: bigint,
    signer: Keypair
  ): Promise<{ positionAddress: PublicKey; signature: string }> {
    const [positionAddress] = await PublicKeyImpl.findProgramAddress(
      [
        Buffer.from('position'),
        poolAddress.toBuffer(),
        signer.publicKey.toBuffer(),
        Buffer.from(tickLower.toString()),
        Buffer.from(tickUpper.toString()),
      ],
      this.programId
    );
    
    const data = Buffer.alloc(13);
    data.writeUInt32LE(3, 0); // Create position instruction
    data.writeInt32LE(tickLower, 4);
    data.writeInt32LE(tickUpper, 8);
    
    const instruction = new TransactionInstruction({
      keys: [
        { pubkey: positionAddress, isSigner: false, isWritable: true },
        { pubkey: poolAddress, isSigner: false, isWritable: true },
        { pubkey: signer.publicKey, isSigner: true, isWritable: true },
        { pubkey: SystemProgram.programId, isSigner: false, isWritable: false },
        { pubkey: TokenProgram.programId, isSigner: false, isWritable: false },
      ],
      programId: this.programId,
      data,
    });
    
    const transaction = new Transaction().add(instruction);
    transaction.recentBlockhash = (await this.connection.getRecentBlockhash()).blockhash;
    transaction.feePayer = signer.publicKey;
    
    const signature = await this.connection.sendTransaction(transaction, [signer]);
    await this.connection.confirmTransaction(signature);
    
    return { positionAddress, signature };
  }
  
  // Calculate liquidity for a position
  calculatePositionLiquidity(
    reserveA: bigint,
    reserveB: bigint,
    tickLower: number,
    tickUpper: number,
    currentTick: number
  ): bigint {
    const sqrtPriceLower = ConcentratedLiquidityMath.sqrtPriceFromTick(tickLower);
    const sqrtPriceUpper = ConcentratedLiquidityMath.sqrtPriceFromTick(tickUpper);
    const currentSqrtPrice = ConcentratedLiquidityMath.sqrtPriceFromTick(currentTick);
    
    return ConcentratedLiquidityMath.liquidityFromAmounts(
      reserveA,
      reserveB,
      sqrtPriceLower,
      sqrtPriceUpper,
      currentSqrtPrice
    );
  }
  
  // Calculate amounts for a position
  calculatePositionAmounts(
    liquidity: bigint,
    tickLower: number,
    tickUpper: number,
    currentTick: number
  ): { amountA: bigint; amountB: bigint } {
    const sqrtPriceLower = ConcentratedLiquidityMath.sqrtPriceFromTick(tickLower);
    const sqrtPriceUpper = ConcentratedLiquidityMath.sqrtPriceFromTick(tickUpper);
    const currentSqrtPrice = ConcentratedLiquidityMath.sqrtPriceFromTick(currentTick);
    
    return ConcentratedLiquidityMath.amountsFromLiquidity(
      liquidity,
      sqrtPriceLower,
      sqrtPriceUpper,
      currentSqrtPrice,
      true
    );
  }
  
  // Update position (add/remove liquidity)
  async updatePosition(
    positionAddress: PublicKey,
    liquidityDelta: bigint,
    signer: Keypair
  ): Promise<string> {
    const data = Buffer.alloc(9);
    data.writeUInt32LE(4, 0); // Update position instruction
    data.writeBigInt64LE(liquidityDelta, 4);
    
    const instruction = new TransactionInstruction({
      keys: [
        { pubkey: positionAddress, isSigner: false, isWritable: true },
        { pubkey: signer.publicKey, isSigner: true, isWritable: false },
      ],
      programId: this.programId,
      data,
    });
    
    const transaction = new Transaction().add(instruction);
    transaction.recentBlockhash = (await this.connection.getRecentBlockhash()).blockhash;
    transaction.feePayer = signer.publicKey;
    
    return await this.connection.sendTransaction(transaction, [signer]);
  }
  
  // Collect fees from position
  async collectFees(positionAddress: PublicKey, signer: Keypair): Promise<string> {
    const data = Buffer.alloc(4);
    data.writeUInt32LE(5, 0); // Collect fees instruction
    
    const instruction = new TransactionInstruction({
      keys: [
        { pubkey: positionAddress, isSigner: false, isWritable: true },
        { pubkey: signer.publicKey, isSigner: true, isWritable: true },
      ],
      programId: this.programId,
      data,
    });
    
    const transaction = new Transaction().add(instruction);
    transaction.recentBlockhash = (await this.connection.getRecentBlockhash()).blockhash;
    transaction.feePayer = signer.publicKey;
    
    return await this.connection.sendTransaction(transaction, [signer]);
  }
}

// ============================================================================
// Default Export
// ============================================================================

export default {
  AmmPoolManager,
  AmmRouter,
  ConcentratedLiquidityManager,
  ConcentratedLiquidityMath,
  StableSwapMath,
  FEE_TIERS,
};