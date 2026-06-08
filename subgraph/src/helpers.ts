import { Token, Pool, Factory, User, Transaction, TokenHourData, TokenDayData, PoolHourData, PoolDayData, DayData } from '../generated/schema'
import { BigInt, BigDecimal, ethereum } from '@graphprotocol/graph-ts'

// Factory address
export const FACTORY_ADDRESS = '0x0000000000000000000000000000000000000001'

// Get or create token
export function getOrCreateToken(id: string): Token {
  let token = Token.load(id)
  if (token === null) {
    token = new Token(id)
    token.symbol = ''
    token.name = ''
    token.decimals = 0
    token.totalSupply = BigInt.fromI32(0)
    token.tradeVolume = BigInt.fromI32(0)
    token.tradeVolumeUSD = BigDecimal.fromString('0')
    token.totalValueLockedUSD = BigDecimal.fromString('0')
    token.txCount = BigInt.fromI32(0)
    token.createdAt = BigInt.fromI32(0)
    token.updatedAt = BigInt.fromI32(0)
  }
  return token as Token
}

// Get or create pool
export function getOrCreatePool(id: string): Pool {
  let pool = Pool.load(id)
  if (pool === null) {
    pool = new Pool(id)
    pool.liquidity = BigInt.fromI32(0)
    pool.sqrtPrice = BigInt.fromI32(0)
    pool.token0Price = BigDecimal.fromString('0')
    pool.token1Price = BigDecimal.fromString('0')
    pool.observationCardinality = 0
    pool.observationCardinalityNext = BigInt.fromI32(0)
    pool.volumeToken0 = BigInt.fromI32(0)
    pool.volumeToken1 = BigInt.fromI32(0)
    pool.volumeUSD = BigDecimal.fromString('0')
    pool.txCount = BigInt.fromI32(0)
    pool.createdAtTimestamp = BigInt.fromI32(0)
    pool.createdAtBlock = BigInt.fromI32(0)
  }
  return pool as Pool
}

// Get or create factory
export function getOrCreateFactory(): Factory {
  let factory = Factory.load(FACTORY_ADDRESS)
  if (factory === null) {
    factory = new Factory(FACTORY_ADDRESS)
    factory.poolCount = BigInt.fromI32(0)
    factory.totalValueLockedUSD = BigDecimal.fromString('0')
    factory.totalVolumeUSD = BigDecimal.fromString('0')
    factory.totalFeesUSD = BigDecimal.fromString('0')
    factory.txCount = BigInt.fromI32(0)
  }
  return factory as Factory
}

// Get or create user
export function getOrCreateUser(id: string): User {
  let user = User.load(id)
  if (user === null) {
    user = new User(id)
    user.totalValueLockedUSD = BigDecimal.fromString('0')
  }
  return user as User
}

// Get or create transaction
export function getOrCreateTransaction(id: string): Transaction {
  let tx = Transaction.load(id)
  if (tx === null) {
    tx = new Transaction(id)
    tx.blockNumber = BigInt.fromI32(0)
    tx.timestamp = BigInt.fromI32(0)
  }
  return tx as Transaction
}

// Helper to get token hour data ID
export function getTokenHourDataId(tokenId: string, timestamp: BigInt): string {
  let hourStartUnix = timestamp.toI32() / 3600 * 3600
  return tokenId.concat('-').concat(hourStartUnix.toString())
}

// Helper to get token day data ID
export function getTokenDayDataId(tokenId: string, timestamp: BigInt): string {
  let date = timestamp.toI32() / 86400
  return tokenId.concat('-').concat(date.toString())
}

// Helper to get pool hour data ID
export function getPoolHourDataId(poolId: string, timestamp: BigInt): string {
  let hourStartUnix = timestamp.toI32() / 3600 * 3600
  return poolId.concat('-').concat(hourStartUnix.toString())
}

// Helper to get pool day data ID
export function getPoolDayDataId(poolId: string, timestamp: BigInt): string {
  let date = timestamp.toI32() / 86400
  return poolId.concat('-').concat(date.toString())
}

// Helper to get day data ID
export function getDayDataId(timestamp: BigInt): string {
  let date = timestamp.toI32() / 86400
  return date.toString()
}

// Update pool day data
export function updatePoolDayData(poolId: string, timestamp: BigInt, volumeToken0: BigDecimal, volumeToken1: BigDecimal): void {
  let pool = Pool.load(poolId)!
  let poolDayData = PoolDayData.load(getPoolDayDataId(poolId, timestamp))
  
  if (poolDayData === null) {
    poolDayData = new PoolDayData(getPoolDayDataId(poolId, timestamp))
    poolDayData.pool = pool.id
    poolDayData.date = timestamp.toI32() / 86400
    poolDayData.volumeToken0 = BigDecimal.fromString('0')
    poolDayData.volumeToken1 = BigDecimal.fromString('0')
    poolDayData.volumeUSD = BigDecimal.fromString('0')
    poolDayData.txCount = BigInt.fromI32(0)
    poolDayData.tvlUSD = BigDecimal.fromString('0')
  }
  
  poolDayData.volumeToken0 = poolDayData.volumeToken0.plus(volumeToken0)
  poolDayData.volumeToken1 = poolDayData.volumeToken1.plus(volumeToken1)
  poolDayData.volumeUSD = poolDayData.volumeUSD.plus(volumeToken0.plus(volumeToken1))
  poolDayData.txCount = poolDayData.txCount.plus(BigInt.fromI32(1))
  poolDayData.save()
}

// Update factory day data
export function updateFactoryDayData(timestamp: BigInt, volumeUSD: BigDecimal, tvlUSD: BigDecimal, feesUSD: BigDecimal): void {
  let dayData = DayData.load(getDayDataId(timestamp))
  
  if (dayData === null) {
    dayData = new DayData(getDayDataId(timestamp))
    dayData.date = timestamp.toI32() / 86400
    dayData.tvlUSD = BigDecimal.fromString('0')
    dayData.volumeUSD = BigDecimal.fromString('0')
    dayData.feesUSD = BigDecimal.fromString('0')
  }
  
  dayData.volumeUSD = dayData.volumeUSD.plus(volumeUSD)
  dayData.tvlUSD = dayData.tvlUSD.plus(tvlUSD)
  dayData.feesUSD = dayData.feesUSD.plus(feesUSD)
  dayData.save()
}