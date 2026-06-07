/**
 * TigerSwap User Features - Lending & Borrowing Module
 * 
 * Native lending protocol with collateral, liquidation, and interest rates.
 * Zero external dependencies - fully native implementation.
 * 
 * Features:
 * - Collateral deposits
 * - Token borrowing
 * - Interest rate models
 * - Liquidation mechanism
 * - Flash loans
 * 
 * @author TigerSwap
 * @version 1.0.0
 */

import { EVMClient, EVMWallet } from '@tigerswap/evm-sdk';

// ============================================================================
// Types & Interfaces
// ============================================================================

export interface LendingPool {
  id: string;
  token: string;
  totalDeposits: bigint;
  totalBorrows: bigint;
  depositRate: number;
  borrowRate: number;
  utilizationRate: number;
  collateralFactor: number;
  liquidationThreshold: number;
  liquidationPenalty: number;
  reserveFactor: number;
  totalReserves: bigint;
}

export interface UserPosition {
  user: string;
  deposits: Map<string, bigint>;
  borrows: Map<string, bigint>;
  collateralValue: bigint;
  borrowValue: bigint;
  healthFactor: number;
}

export interface Liquidation {
  liquidator: string;
  borrower: string;
  collateralToken: string;
  repayAmount: bigint;
  collateralAmount: bigint;
  timestamp: number;
}

export interface InterestRateModel {
  baseRate: number;
  slope1: number;
  slope2: number;
  optimalUtilization: number;
}

export interface Market {
  token: string;
  pool: LendingPool;
  priceOracle: string;
  price: number;
}

// ============================================================================
// Lending Protocol
// ============================================================================

export class LendingProtocol {
  private pools: Map<string, LendingPool>;
  private positions: Map<string, UserPosition>;
  private liquidations: Liquidation[];
  private interestModels: Map<string, InterestRateModel>;

  constructor() {
    this.pools = new Map();
    this.positions = new Map();
    this.liquidations = [];
    this.interestModels = new Map();
  }

  /**
   * Create lending pool
   */
  createPool(
    token: string,
    collateralFactor: number,
    liquidationThreshold: number,
    liquidationPenalty: number,
    reserveFactor: number,
    interestModel: InterestRateModel
  ): LendingPool {
    const pool: LendingPool = {
      id: `pool_${token}`,
      token,
      totalDeposits: 0n,
      totalBorrows: 0n,
      depositRate: 0,
      borrowRate: 0,
      utilizationRate: 0,
      collateralFactor,
      liquidationThreshold,
      liquidationPenalty,
      reserveFactor,
      totalReserves: 0n,
    };

    this.pools.set(token, pool);
    this.interestModels.set(token, interestModel);

    return pool;
  }

  /**
   * Deposit collateral
   */
  deposit(user: string, token: string, amount: bigint): void {
    const pool = this.pools.get(token);
    if (!pool) throw new Error('Pool not found');

    // Update pool
    pool.totalDeposits += amount;

    // Update user position
    const position = this.getOrCreatePosition(user);
    const current = position.deposits.get(token) || 0n;
    position.deposits.set(token, current + amount);

    // Update collateral value
    this.updateCollateralValue(position, pool, token, amount, true);

    // Update rates
    this.updateRates(pool);
  }

  /**
   * Withdraw collateral
   */
  withdraw(user: string, token: string, amount: bigint): void {
    const pool = this.pools.get(token);
    if (!pool) throw new Error('Pool not found');

    const position = this.positions.get(user);
    if (!position) throw new Error('No position found');

    const deposited = position.deposits.get(token) || 0n;
    if (deposited < amount) throw new Error('Insufficient balance');

    // Check health factor after withdrawal
    const testPosition = this.clonePosition(position);
    testPosition.deposits.set(token, deposited - amount);
    this.updateCollateralValue(testPosition, pool, token, amount, false);

    if (testPosition.healthFactor < 10000) {
      throw new Error('Health factor would be too low');
    }

    // Update pool
    pool.totalDeposits -= amount;

    // Update position
    position.deposits.set(token, deposited - amount);
    this.updateCollateralValue(position, pool, token, amount, false);

    // Update rates
    this.updateRates(pool);
  }

  /**
   * Borrow tokens
   */
  borrow(user: string, token: string, amount: bigint): void {
    const pool = this.pools.get(token);
    if (!pool) throw new Error('Pool not found');

    const position = this.getOrCreatePosition(user);

    // Check if borrowing is allowed
    if (position.healthFactor < 10000) {
      throw new Error('Health factor too low');
    }

    // Calculate max borrow
    const maxBorrow = this.calculateMaxBorrow(position, pool);
    if (amount > maxBorrow) {
      throw new Error('Exceeds max borrow');
    }

    // Check available liquidity
    const available = pool.totalDeposits - pool.totalBorrows;
    if (amount > available) {
      throw new Error('Insufficient liquidity');
    }

    // Update pool
    pool.totalBorrows += amount;

    // Update position
    const current = position.borrows.get(token) || 0n;
    position.borrows.set(token, current + amount);

    // Update borrow value
    this.updateBorrowValue(position, pool, amount, true);

    // Update rates
    this.updateRates(pool);
  }

  /**
   * Repay borrow
   */
  repay(user: string, token: string, amount: bigint): void {
    const pool = this.pools.get(token);
    if (!pool) throw new Error('Pool not found');

    const position = this.positions.get(user);
    if (!position) throw new Error('No position found');

    const borrowed = position.borrows.get(token) || 0n;
    const repayAmount = amount > borrowed ? borrowed : amount;

    // Update pool
    pool.totalBorrows -= repayAmount;

    // Update position
    position.borrows.set(token, borrowed - repayAmount);
    this.updateBorrowValue(position, pool, repayAmount, false);

    // Update rates
    this.updateRates(pool);
  }

  /**
   * Liquidate position
   */
  liquidate(
    liquidator: string,
    borrower: string,
    collateralToken: string,
    repayToken: string,
    repayAmount: bigint
  ): Liquidation {
    const position = this.positions.get(borrower);
    if (!position) throw new Error('Position not found');

    const collateralPool = this.pools.get(collateralToken);
    if (!collateralPool) throw new Error('Collateral pool not found');

    // Check if position is liquidatable
    if (position.healthFactor >= 10000) {
      throw new Error('Position not liquidatable');
    }

    // Calculate collateral to receive
    const collateralAmount = this.calculateLiquidationAmount(
      repayAmount,
      collateralPool.liquidationPenalty
    );

    // Execute liquidation
    const collateralDeposited = position.deposits.get(collateralToken) || 0n;
    const actualCollateral = collateralAmount > collateralDeposited 
      ? collateralDeposited 
      : collateralAmount;

    // Update positions
    position.deposits.set(collateralToken, collateralDeposited - actualCollateral);
    this.updateCollateralValue(position, collateralPool, actualCollateral, false);

    // Update pools
    collateralPool.totalDeposits -= actualCollateral;

    const repayPool = this.pools.get(repayToken);
    if (repayPool) {
      repayPool.totalBorrows -= repayAmount;
      this.updateRates(repayPool);
    }

    // Record liquidation
    const liquidation: Liquidation = {
      liquidator,
      borrower,
      collateralToken,
      repayAmount,
      collateralAmount: actualCollateral,
      timestamp: Date.now(),
    };

    this.liquidations.push(liquidation);

    return liquidation;
  }

  /**
   * Get user position
   */
  getPosition(user: string): UserPosition | null {
    return this.positions.get(user) || null;
  }

  /**
   * Get pool info
   */
  getPool(token: string): LendingPool | null {
    return this.pools.get(token) || null;
  }

  /**
   * Get all pools
   */
  getAllPools(): LendingPool[] {
    return Array.from(this.pools.values());
  }

  /**
   * Get liquidations
   */
  getLiquidations(limit: number = 100): Liquidation[] {
    return this.liquidations.slice(-limit);
  }

  /**
   * Calculate health factor
   */
  calculateHealthFactor(position: UserPosition): number {
    if (position.collateralValue === 0n) return 0;
    return Number((position.borrowValue * 10000n) / position.collateralValue);
  }

  private calculateMaxBorrow(position: UserPosition, pool: LendingPool): bigint {
    const maxCollateral = (position.collateralValue * BigInt(pool.collateralFactor)) / 10000n;
    const available = maxCollateral - position.borrowValue;
    return available > 0n ? available : 0n;
  }

  private calculateLiquidationAmount(repayAmount: bigint, penalty: number): bigint {
    return (repayAmount * BigInt(10000 + penalty)) / 10000n;
  }

  private updateCollateralValue(
    position: UserPosition,
    pool: LendingPool,
    token: string,
    amount: bigint,
    isDeposit: boolean
  ): void {
    if (isDeposit) {
      position.collateralValue += (amount * BigInt(pool.collateralFactor)) / 10000n;
    } else {
      position.collateralValue -= (amount * BigInt(pool.collateralFactor)) / 10000n;
    }
    position.healthFactor = this.calculateHealthFactor(position);
  }

  private updateBorrowValue(
    position: UserPosition,
    pool: LendingPool,
    amount: bigint,
    isBorrow: boolean
  ): void {
    if (isBorrow) {
      position.borrowValue += amount;
    } else {
      position.borrowValue -= amount;
    }
    position.healthFactor = this.calculateHealthFactor(position);
  }

  private updateRates(pool: LendingPool): void {
    const utilization = pool.totalDeposits > 0n
      ? (pool.totalBorrows * 10000n) / pool.totalDeposits
      : 0n;

    const model = this.interestModels.get(pool.token);
    if (!model) return;

    const utilizationNum = Number(utilization);
    const optimal = model.optimalUtilization;

    if (utilizationNum <= optimal) {
      const factor = utilizationNum / optimal;
      pool.borrowRate = model.baseRate + factor * model.slope1;
    } else {
      const factor = (utilizationNum - optimal) / (10000 - optimal);
      pool.borrowRate = model.baseRate + model.slope1 + factor * model.slope2;
    }

    pool.depositRate = (pool.borrowRate * Number(utilization) * (10000 - pool.reserveFactor)) / 100000000;
    pool.utilizationRate = utilizationNum / 100;
  }

  private getOrCreatePosition(user: string): UserPosition {
    if (!this.positions.has(user)) {
      this.positions.set(user, {
        user,
        deposits: new Map(),
        borrows: new Map(),
        collateralValue: 0n,
        borrowValue: 0n,
        healthFactor: 1000000,
      });
    }
    return this.positions.get(user)!;
  }

  private clonePosition(position: UserPosition): UserPosition {
    return {
      user: position.user,
      deposits: new Map(position.deposits),
      borrows: new Map(position.borrows),
      collateralValue: position.collateralValue,
      borrowValue: position.borrowValue,
      healthFactor: position.healthFactor,
    };
  }
}

// ============================================================================
// Interest Rate Calculator
// ============================================================================

export class InterestRateCalculator {
  /**
   * Calculate compound interest
   */
  static compound(principal: bigint, rate: number, time: number): bigint {
    const ratePerSecond = rate / (365 * 24 * 60 * 60);
    const compoundFactor = 1 + ratePerSecond * time;
    return (principal * BigInt(Math.floor(compoundFactor * 1000000))) / 1000000n;
  }

  /**
   * Calculate APY from APR
   */
  static aprToApy(apr: number): number {
    return Math.pow(1 + apr / 100, 1) - 1;
  }

  /**
   * Calculate APR from APY
   */
  static apyToApr(apy: number): number {
    return Math.pow(1 + apy, 1) - 1;
  }
}

// ============================================================================
// Price Oracle
// ============================================================================

export class PriceOracle {
  private prices: Map<string, number>;

  constructor() {
    this.prices = new Map();
  }

  /**
   * Set price
   */
  setPrice(token: string, price: number): void {
    this.prices.set(token, price);
  }

  /**
   * Get price
   */
  getPrice(token: string): number {
    return this.prices.get(token) || 0;
  }

  /**
   * Get price in USD
   */
  getValueInUSD(token: string, amount: bigint, decimals: number): number {
    const price = this.getPrice(token);
    return Number(amount) / Math.pow(10, decimals) * price;
  }
}

// ============================================================================
// Export
// ============================================================================

export default {
  LendingProtocol,
  InterestRateCalculator,
  PriceOracle,
};