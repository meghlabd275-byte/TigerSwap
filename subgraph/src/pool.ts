import { 
  Swap as SwapEvent, 
  Mint as MintEvent, 
  Burn as BurnEvent, 
  Sync, 
  Collect,
  SetFeeProtocol
} from '../generated/templates/TigerPool/TigerPool'
import { 
  Pool, 
  Swap, 
  Mint, 
  Burn, 
  Transaction,
  TokenPosition,
  PositionSnapshot,
  LiquidityPosition
} from '../generated/schema'
import { BigInt, BigDecimal, ethereum } from '@graphprotocol/graph-ts'
import {
  getOrCreatePool,
  getOrCreateToken,
  getOrCreateUser,
  getOrCreateTransaction,
  getTokenDayDataId,
  getPoolDayDataId,
  getDayDataId,
  updatePoolDayData
} from './helpers'

// Constants
const FEE_DENOMINATOR = BigInt.fromI32(1000000)

// Handle swap events
export function handleSwap(event: SwapEvent): void {
  let pool = Pool.load(event.address.toHex())!
  
  // Get or create transaction
  let transaction = getOrCreateTransaction(event.transaction.hash.toHex())
  transaction.blockNumber = event.block.number
  transaction.timestamp = event.block.timestamp
  transaction.save()
  
  // Determine token in/out
  let tokenIn = event.params.amount0Out.gt(BigInt.fromI32(0)) ? pool.token0 : pool.token1
  let tokenOut = event.params.amount1Out.gt(BigInt.fromI32(0)) ? pool.token1 : pool.token0
  
  let amountIn = event.params.amount0Out.gt(BigInt.fromI32(0)) 
    ? event.params.amount0Out.toBigDecimal()
    : event.params.amount1Out.toBigDecimal()
  
  let amountOut = event.params.amount1Out.gt(BigInt.fromI32(0))
    ? event.params.amount1Out.toBigDecimal()
    : event.params.amount0Out.toBigDecimal()
  
  // Create swap entity
  let swap = new Swap(event.transaction.hash.toHex().concat('-').concat(event.logIndex.toString()))
  swap.pool = pool.id
  swap.tokenIn = tokenIn
  swap.tokenOut = tokenOut
  swap.amountIn = amountIn
  swap.amountOut = amountOut
  swap.amountInUSD = amountIn
  swap.amountOutUSD = amountOut
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
  
  // Update pool day data
  updatePoolDayData(pool.id, event.block.timestamp, amountIn, amountOut)
}

// Handle mint events
export function handleMint(event: MintEvent): void {
  let pool = Pool.load(event.address.toHex())!
  let transaction = getOrCreateTransaction(event.transaction.hash.toHex())
  
  transaction.blockNumber = event.block.number
  transaction.timestamp = event.block.timestamp
  transaction.save()
  
  let token0Amount = event.params.amount0.toBigDecimal()
  let token1Amount = event.params.amount1.toBigDecimal()
  
  // Create mint entity
  let mint = new Mint(event.transaction.hash.toHex().concat('-').concat(event.logIndex.toString()))
  mint.pool = pool.id
  mint.token0Amount = token0Amount
  mint.token1Amount = token1Amount
  mint.amountUSD = token0Amount.plus(token1Amount)
  mint.sender = event.params.sender
  mint.origin = event.transaction.from
  mint.blockNumber = event.block.number
  mint.timestamp = event.block.timestamp
  mint.transaction = transaction.id
  mint.save()
  
  // Update pool liquidity
  pool.liquidity = pool.liquidity.plus(event.params.liquidity)
  pool.save()
}

// Handle burn events
export function handleBurn(event: BurnEvent): void {
  let pool = Pool.load(event.address.toHex())!
  let transaction = getOrCreateTransaction(event.transaction.hash.toHex())
  
  transaction.blockNumber = event.block.number
  transaction.timestamp = event.block.timestamp
  transaction.save()
  
  let token0Amount = event.params.amount0.toBigDecimal()
  let token1Amount = event.params.amount1.toBigDecimal()
  
  // Create burn entity
  let burn = new Burn(event.transaction.hash.toHex().concat('-').concat(event.logIndex.toString()))
  burn.pool = pool.id
  burn.token0Amount = token0Amount
  burn.token1Amount = token1Amount
  burn.amountUSD = token0Amount.plus(token1Amount)
  burn.sender = event.params.sender
  burn.origin = event.transaction.from
  burn.blockNumber = event.block.number
  burn.timestamp = event.block.timestamp
  burn.transaction = transaction.id
  burn.save()
  
  // Update pool liquidity
  pool.liquidity = pool.liquidity.minus(event.params.liquidity)
  pool.save()
}

// Handle sync events - update pool prices
export function handleSync(event: Sync): void {
  let pool = Pool.load(event.address.toHex())!
  
  pool.sqrtPrice = event.params.sqrtPriceX96
  pool.liquidity = event.params.liquidity
  
  pool.save()
}

// Handle collect events
export function handleCollect(event: Collect): void {
  // Collect events track fee collection - could update position data here
}

// Handle fee protocol updates
export function handleSetFeeProtocol(event: SetFeeProtocol): void {
  let pool = Pool.load(event.address.toHex())!
  
  // Update pool fee tier if needed
  pool.save()
}