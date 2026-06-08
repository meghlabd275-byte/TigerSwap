import {
  Token,
  Pool,
  Swap,
  Mint,
  Burn,
  Transaction,
  User,
  LiquidityPosition,
  Factory,
  TokenHourData,
  TokenDayData,
  PoolHourData,
  PoolDayData,
  TokenPosition,
  DayData,
  PositionSnapshot
} from './generated/schema'

import {
  Transfer,
  Approval,
  Swap as SwapEvent,
  Mint as MintEvent,
  Burn as BurnEvent,
  Sync
} from './generated/templates/pool/TigerPool'

import { BigInt, BigDecimal, ethereum } from '@graphprotocol/graph-ts'

// Factory address
const FACTORY_ADDRESS = '0x0000000000000000000000000000000000000001'

// Global counters
let factory = Factory.load(FACTORY_ADDRESS)
if (factory === null) {
  factory = new Factory(FACTORY_ADDRESS)
  factory.poolCount = BigInt.fromI32(0)
  factory.totalValueLockedUSD = BigDecimal.fromString('0')
  factory.totalVolumeUSD = BigDecimal.fromString('0')
  factory.totalFeesUSD = BigDecimal.fromString('0')
  factory.txCount = BigInt.fromI32(0)
}

// Helper functions
function getOrCreateToken(id: string): Token {
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

function getOrCreatePool(id: string): Pool {
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
    
    // Update factory
    factory!.poolCount = factory!.poolCount.plus(BigInt.fromI32(1))
  }
  return pool as Pool
}

function getOrCreateUser(id: string): User {
  let user = User.load(id)
  if (user === null) {
    user = new User(id)
    user.totalValueLockedUSD = BigDecimal.fromString('0')
  }
  return user as User
}

function getOrCreateTransaction(id: string): Transaction {
  let tx = Transaction.load(id)
  if (tx === null) {
    tx = new Transaction(id)
    tx.blockNumber = BigInt.fromI32(0)
    tx.timestamp = BigInt.fromI32(0)
  }
  return tx as Transaction
}

export function handleSync(event: Sync): void {
  let pool = getOrCreatePool(event.address.toHex())
  
  // Update pool state
  pool.liquidity = event.params.liquidity
  pool.sqrtPrice = event.params.sqrtPriceX96
  
  // Update token prices
  if (pool.token0Price.notEqual(BigDecimal.fromString('0'))) {
    pool.token1Price = pool.token0Price.times(BigDecimal.fromString('1'))
  }
  
  pool.save()
}

export function handleSwap(event: SwapEvent): void {
  let pool = getOrCreatePool(event.address.toHex())
  let transaction = getOrCreateTransaction(event.transaction.hash.toHex())
  
  transaction.blockNumber = event.block.number
  transaction.timestamp = event.block.timestamp
  transaction.save()
  
  // Create swap record
  let swap = new Swap(event.transaction.hash.toHex().concat('-').concat(event.logIndex.toString()))
  swap.pool = pool.id
  swap.tokenIn = event.params.amount0Out.gt(BigInt.fromI32(0)) ? pool.token0 : pool.token1
  swap.tokenOut = event.params.amount1Out.gt(BigInt.fromI32(0)) ? pool.token1 : pool.token0
  swap.amountIn = event.params.amount0Out.gt(BigInt.fromI32(0)) 
    ? (event.params.amount0Out.toBigDecimal())
    : (event.params.amount1Out.toBigDecimal())
  swap.amountOut = event.params.amount1Out.gt(BigInt.fromI32(0))
    ? (event.params.amount1Out.toBigDecimal())
    : (event.params.amount0Out.toBigDecimal())
  swap.amountInUSD = swap.amountIn
  swap.amountOutUSD = swap.amountOut
  swap.sender = event.params.sender
  swap.recipient = event.params.to
  swap.origin = event.transaction.from
  swap.blockNumber = event.block.number
  swap.timestamp = event.block.timestamp
  swap.transaction = transaction.id
  
  swap.save()
  
  // Update pool stats
  pool.txCount = pool.txCount.plus(BigInt.fromI32(1))
  pool.save()
  
  // Update factory
  factory!.txCount = factory!.txCount.plus(BigInt.fromI32(1))
  factory!.save()
}

export function handleMint(event: MintEvent): void {
  let pool = getOrCreatePool(event.address.toHex())
  let transaction = getOrCreateTransaction(event.transaction.hash.toHex())
  
  transaction.blockNumber = event.block.number
  transaction.timestamp = event.block.timestamp
  transaction.save()
  
  // Create mint record
  let mint = new Mint(event.transaction.hash.toHex().concat('-').concat(event.logIndex.toString()))
  mint.pool = pool.id
  mint.token0Amount = event.params.amount0.toBigDecimal()
  mint.token1Amount = event.params.amount1.toBigDecimal()
  mint.amountUSD = mint.token0Amount.plus(mint.token1Amount)
  mint.sender = event.params.sender
  mint.origin = event.transaction.from
  mint.blockNumber = event.block.number
  mint.timestamp = event.block.timestamp
  mint.transaction = transaction.id
  
  mint.save()
}

export function handleBurn(event: BurnEvent): void {
  let pool = getOrCreatePool(event.address.toHex())
  let transaction = getOrCreateTransaction(event.transaction.hash.toHex())
  
  transaction.blockNumber = event.block.number
  transaction.timestamp = event.block.timestamp
  transaction.save()
  
  // Create burn record
  let burn = new Burn(event.transaction.hash.toHex().concat('-').concat(event.logIndex.toString()))
  burn.pool = pool.id
  burn.token0Amount = event.params.amount0.toBigDecimal()
  burn.token1Amount = event.params.amount1.toBigDecimal()
  burn.amountUSD = burn.token0Amount.plus(burn.token1Amount)
  burn.sender = event.params.sender
  burn.origin = event.transaction.from
  burn.blockNumber = event.block.number
  burn.timestamp = event.block.timestamp
  burn.transaction = transaction.id
  
  burn.save()
}

// Token transfer handler
export function handleTransfer(event: Transfer): void {
  let token = getOrCreateToken(event.address.toHex())
  
  // Update total supply on mint/burn
  if (event.params.to.toHex() == '0x0000000000000000000000000000000000000000') {
    // Burn
    token.totalSupply = token.totalSupply.minus(event.params.value)
  } else if (event.params.from.toHex() == '0x0000000000000000000000000000000000000000') {
    // Mint
    token.totalSupply = token.totalSupply.plus(event.params.value)
  }
  
  token.updatedAt = event.block.timestamp
  token.save()
}
