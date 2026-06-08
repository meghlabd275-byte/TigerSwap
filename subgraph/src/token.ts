import { Transfer, Approval } from '../generated/TigerToken/TigerToken'
import { Token, TokenHourData, TokenDayData } from '../generated/schema'
import { BigInt, BigDecimal, ethereum } from '@graphprotocol/graph-ts'
import { getOrCreateToken, getTokenHourDataId, getTokenDayDataId } from './helpers'

// Handle token transfers
export function handleTransfer(event: Transfer): void {
  let token = getOrCreateToken(event.address.toHex())
  
  // Update total supply on mint/burn
  if (event.params.to.toHex() == '0x0000000000000000000000000000000000000000') {
    // Burn - reduce supply
    token.totalSupply = token.totalSupply.minus(event.params.value)
  } else if (event.params.from.toHex() == '0x0000000000000000000000000000000000000000') {
    // Mint - increase supply
    token.totalSupply = token.totalSupply.plus(event.params.value)
  }
  
  // Update token transaction count
  token.txCount = token.txCount.plus(BigInt.fromI32(1))
  token.updatedAt = event.block.timestamp
  token.save()
  
  // Create or update token day data
  let tokenDayData = TokenDayData.load(getTokenDayDataId(token.id, event.block.timestamp))
  if (tokenDayData === null) {
    tokenDayData = new TokenDayData(getTokenDayDataId(token.id, event.block.timestamp))
    tokenDayData.token = token.id
    tokenDayData.date = event.block.timestamp.toI32() / 86400
    tokenDayData.volume = BigDecimal.fromString('0')
    tokenDayData.volumeUSD = BigDecimal.fromString('0')
    tokenDayData.untrackedVolumeUSD = BigDecimal.fromString('0')
    tokenDayData.txCount = BigInt.fromI32(0)
    tokenDayData.totalValueLockedUSD = BigDecimal.fromString('0')
  }
  tokenDayData.txCount = tokenDayData.txCount.plus(BigInt.fromI32(1))
  tokenDayData.save()
}

// Handle token approvals
export function handleApproval(event: Approval): void {
  // Approval events don't need special handling in the subgraph
  // They're useful for tracking allowances but not essential for pool data
}