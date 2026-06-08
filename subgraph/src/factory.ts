import { PoolCreated } from '../generated/TigerFactory/TigerFactory'
import { TigerPool } from '../generated/templates'
import { Pool, Token, Factory, TokenDayData, PoolDayData, DayData } from '../generated/schema'
import { BigInt, BigDecimal, ethereum } from '@graphprotocol/graph-ts'
import {
  getOrCreateToken,
  getOrCreatePool,
  getOrCreateFactory,
  getTokenDayDataId,
  getPoolDayDataId,
  getDayDataId,
  updateFactoryDayData,
  updatePoolDayData
} from './helpers'

export function handlePoolCreated(event: PoolCreated): void {
  let factory = getOrCreateFactory()
  
  // Create the pool template
  let poolAddress = event.params.pool
  let token0Address = event.params.token0
  let token1Address = event.params.token1
  
  // Create token entities if they don't exist
  let token0 = getOrCreateToken(token0Address.toHex())
  let token1 = getOrCreateToken(token1Address.toHex())
  
  // Create pool entity
  let pool = getOrCreatePool(poolAddress.toHex())
  pool.token0 = token0.id
  pool.token1 = token1.id
  pool.feeTier = event.params.fee.toI32()
  pool.sqrtPrice = BigInt.fromI32(0)
  pool.liquidity = BigInt.fromI32(0)
  pool.token0Price = BigDecimal.fromString('0')
  pool.token1Price = BigDecimal.fromString('0')
  pool.observationCardinality = 0
  pool.observationCardinalityNext = BigInt.fromI32(0)
  pool.volumeToken0 = BigInt.fromI32(0)
  pool.volumeToken1 = BigInt.fromI32(0)
  pool.volumeUSD = BigDecimal.fromString('0')
  pool.txCount = BigInt.fromI32(0)
  pool.createdAtTimestamp = event.block.timestamp
  pool.createdAtBlock = event.block.number
  
  pool.save()
  
  // Update factory
  factory.poolCount = factory.poolCount.plus(BigInt.fromI32(1))
  factory.save()
  
  // Create pool day data
  let poolDayData = new PoolDayData(getPoolDayDataId(poolAddress.toHex(), event.block.timestamp))
  poolDayData.pool = pool.id
  poolDayData.date = event.block.timestamp.toI32() / 86400
  poolDayData.volumeToken0 = BigDecimal.fromString('0')
  poolDayData.volumeToken1 = BigDecimal.fromString('0')
  poolDayData.volumeUSD = BigDecimal.fromString('0')
  poolDayData.txCount = BigInt.fromI32(0)
  poolDayData.tvlUSD = BigDecimal.fromString('0')
  poolDayData.save()
  
  // Update global day data
  let dayData = DayData.load(getDayDataId(event.block.timestamp))
  if (dayData === null) {
    dayData = new DayData(getDayDataId(event.block.timestamp))
    dayData.date = event.block.timestamp.toI32() / 86400
    dayData.tvlUSD = BigDecimal.fromString('0')
    dayData.volumeUSD = BigDecimal.fromString('0')
    dayData.feesUSD = BigDecimal.fromString('0')
  }
  dayData.save()
  
  // Create pool template for indexing
  TigerPool.create(poolAddress)
}