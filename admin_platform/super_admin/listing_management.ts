// TigerSwap - Token Listing & Trading Pair Management System
// Complete listing management for admins and token owners

export interface TradingPair {
  id: string
  pairName: string           // e.g., "ETH/USDT"
  baseToken: TokenInfo        // e.g., ETH
  quoteToken: TokenInfo       // e.g., USDT
  chainId: number
  dex: string
  poolAddress: string
  lpTokenAddress: string
  createdAt: number
  createdBy: string
  listingFee: string
  tradingFee: string
  status: PairStatus
  price: string
  priceChange24h: number
  volume24h: string
  liquidity: string
  isStablePair: boolean
  isFeatured: boolean
  tier: ListingTier
  metadata: PairMetadata
}

export interface TokenInfo {
  address: string
  symbol: string
  name: string
  decimals: number
  logo: string
  chainId: number
  isNative: boolean
  isStable: boolean
  totalSupply: string
  holders: number
  marketCap: string
  website?: string
  twitter?: string
  telegram?: string
  discord?: string
  description?: string
}

export interface PoolInfo {
  address: string
  pairId: string
  token0: TokenInfo
  token1: TokenInfo
  reserve0: string
  reserve1: string
  liquidity: string
  lpSupply: string
  volume24h: string
  fee: string
  createdAt: number
}

export enum PairStatus {
  PENDING = 'pending',
  APPROVED = 'approved',
  ACTIVE = 'active',
  DELISTED = 'delisted',
  SUSPENDED = 'suspended'
}

export enum ListingTier {
  TIER_1 = 'tier1',  // Major pairs (BTC, ETH, etc.)
  TIER_2 = 'tier2',  // Established tokens
  TIER_3 = 'tier3',  // New tokens
  TIER_4 = 'tier4'   // Community tokens
}

export interface PairMetadata {
  category: string
  tags: string[]
  warning?: string
  auditStatus?: 'passed' | 'pending' | 'failed'
  auditReportUrl?: string
  kycStatus?: 'verified' | 'pending' | 'none'
}

export interface ListingApplication {
  id: string
  applicantAddress: string
  token: TokenInfo
  baseToken: string           // Quote token for the pair (usually USDT/USDC)
  listingTier: ListingTier
  requestedDex: string[]
  listingFee: string
  tradingFee: string
  status: ApplicationStatus
  submittedAt: number
  reviewedAt?: number
  reviewedBy?: string
  notes?: string
  documents: string[]
}

export enum ApplicationStatus {
  SUBMITTED = 'submitted',
  DOCUMENT_PENDING = 'document_pending',
  AUDIT_PENDING = 'audit_pending',
  APPROVED = 'approved',
  REJECTED = 'rejected',
  FEE_PENDING = 'fee_pending'
}

export interface FeeConfig {
  listingFee: string
  listingFeeUsd: string
  tradingFee: string           // Percentage
  tradingFeeMaker: string     // Percentage
  tradingFeeTaker: string     // Percentage
  lpRewardFee: string         // Percentage
  withdrawalFee: string
  depositFee: string
  stablePairDiscount: number // Percentage discount
}

export interface ListingQueue {
  id: string
  position: number
  token: TokenInfo
  appliedAt: number
  estimatedListingTime: number
  status: 'queued' | 'processing' | 'completed'
}

// Token Listing Manager
export class TokenListingManager {
  private pairs: Map<string, TradingPair> = new Map()
  private pools: Map<string, PoolInfo> = new Map()
  private applications: Map<string, ListingApplication> = new Map()
  private feeConfig: FeeConfig

  constructor() {
    this.initializeDefaultPairs()
    this.initializeDefaultFees()
  }

  private initializeDefaultFees(): void {
    this.feeConfig = {
      listingFee: '1000',           // 1000 TIGER tokens
      listingFeeUsd: '500',          // ~$500 USD
      tradingFee: '0.25',           // 0.25%
      tradingFeeMaker: '0.20',
      tradingFeeTaker: '0.30',
      lpRewardFee: '0.02',
      withdrawalFee: '0.001',
      depositFee: '0',
      stablePairDiscount: 50       // 50% discount for stable pairs
    }
  }

  private initializeDefaultPairs(): void {
    const defaultPairs: TradingPair[] = [
      this.createPair('ETH/USDT', { address: '0x0000000000000000000000000000000000000000', symbol: 'ETH', name: 'Ethereum', decimals: 18, logo: 'eth.png', chainId: 1, isNative: true, isStable: false, totalSupply: '', holders: 0, marketCap: '' }, { address: '0xdAC17F958D2ee523a2206206994597C13D831ec7', symbol: 'USDT', name: 'Tether USD', decimals: 6, logo: 'usdt.png', chainId: 1, isNative: false, isStable: true, totalSupply: '', holders: 0, marketCap: '' }, 'uniswap', '0x1', '0.30', '0.25', PairStatus.ACTIVE, '2450.50', 2.5, '125000000', '50000000'),
      this.createPair('BTC/USDT', { address: '0x2260FAC5E5542a773Aa44fCF2df52aDCEb44661f', symbol: 'WBTC', name: 'Wrapped Bitcoin', decimals: 8, logo: 'wbtc.png', chainId: 1, isNative: false, isStable: false, totalSupply: '', holders: 0, marketCap: '' }, { address: '0xdAC17F958D2ee523a2206206994597C13D831ec7', symbol: 'USDT', name: 'Tether USD', decimals: 6, logo: 'usdt.png', chainId: 1, isNative: false, isStable: true, totalSupply: '', holders: 0, marketCap: '' }, 'uniswap', '0x2', '2000', '0.25', PairStatus.ACTIVE, '62500', 1.2, '250000000', '100000000'),
      this.createPair('BNB/USDT', { address: '0x0000000000000000000000000000000000000000', symbol: 'BNB', name: 'BNB', decimals: 18, logo: 'bnb.png', chainId: 56, isNative: true, isStable: false, totalSupply: '', holders: 0, marketCap: '' }, { address: '0x55d398326f99059fF775485246999027B3197955', symbol: 'USDT', name: 'Tether USD', decimals: 18, logo: 'usdt.png', chainId: 56, isNative: false, isStable: true, totalSupply: '', holders: 0, marketCap: '' }, 'pancakeswap', '0x3', '500', '0.25', PairStatus.ACTIVE, '310.25', 3.1, '80000000', '35000000'),
      this.createPair('MATIC/USDT', { address: '0x0000000000000000000000000000000000000000', symbol: 'MATIC', name: 'Polygon', decimals: 18, logo: 'matic.png', chainId: 137, isNative: true, isStable: false, totalSupply: '', holders: 0, marketCap: '' }, { address: '0xc2132D05D31c914a87C6611C10748AEb04B58e8', symbol: 'USDT', name: 'Tether USD', decimals: 18, logo: 'usdt.png', chainId: 137, isNative: false, isStable: true, totalSupply: '', holders: 0, marketCap: '' }, 'quickswap', '0x4', '200', '0.30', PairStatus.ACTIVE, '0.85', -1.5, '45000000', '18000000'),
    ]

    defaultPairs.forEach(pair => {
      this.pairs.set(pair.id, pair)
    })
  }

  private createPair(
    pairName: string,
    baseToken: TokenInfo,
    quoteToken: TokenInfo,
    dex: string,
    poolAddress: string,
    listingFee: string,
    tradingFee: string,
    status: PairStatus,
    price: string,
    priceChange24h: number,
    volume24h: string,
    liquidity: string
  ): TradingPair {
    return {
      id: `pair_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`,
      pairName,
      baseToken,
      quoteToken,
      chainId: baseToken.chainId,
      dex,
      poolAddress,
      lpTokenAddress: '0x' + Math.random().toString(16).slice(2, 42),
      createdAt: Date.now(),
      createdBy: 'system',
      listingFee,
      tradingFee,
      status,
      price,
      priceChange24h,
      volume24h,
      liquidity,
      isStablePair: quoteToken.isStable,
      isFeatured: ['ETH/USDT', 'BTC/USDT', 'BNB/USDT'].includes(pairName),
      tier: ListingTier.TIER_1,
      metadata: {
        category: 'Major',
        tags: ['blue-chip', 'high-liquidity'],
        auditStatus: 'passed',
        kycStatus: 'verified'
      }
    }
  }

  // ============== ADMIN FUNCTIONS ==============

  // Create new trading pair
  createTradingPair(
    baseToken: TokenInfo,
    quoteToken: TokenInfo,
    dex: string,
    initialLiquidity: string,
    createdBy: string
  ): TradingPair {
    const pairName = `${baseToken.symbol}/${quoteToken.symbol}`
    
    const pair: TradingPair = {
      id: `pair_${Date.now()}`,
      pairName,
      baseToken,
      quoteToken,
      chainId: baseToken.chainId,
      dex,
      poolAddress: this.generatePoolAddress(),
      lpTokenAddress: this.generateLPTokenAddress(),
      createdAt: Date.now(),
      createdBy,
      listingFee: this.feeConfig.listingFee,
      tradingFee: this.feeConfig.tradingFee,
      status: PairStatus.ACTIVE,
      price: '0',
      priceChange24h: 0,
      volume24h: '0',
      liquidity: initialLiquidity,
      isStablePair: quoteToken.isStable,
      isFeatured: false,
      tier: this.determineTier(baseToken),
      metadata: {
        category: 'New',
        tags: [],
        auditStatus: 'pending',
        kycStatus: 'none'
      }
    }

    this.pairs.set(pair.id, pair)
    
    // Create initial pool
    this.createPool(pair.id, baseToken, quoteToken, initialLiquidity)
    
    return pair
  }

  // Create liquidity pool
  createPool(
    pairId: string,
    token0: TokenInfo,
    token1: TokenInfo,
    initialLiquidity: string
  ): PoolInfo {
    const pool: PoolInfo = {
      address: this.generatePoolAddress(),
      pairId,
      token0,
      token1,
      reserve0: initialLiquidity,
      reserve1: (parseFloat(initialLiquidity) * 1000).toString(),
      liquidity: initialLiquidity,
      lpSupply: initialLiquidity,
      volume24h: '0',
      fee: '0.25',
      createdAt: Date.now()
    }

    this.pools.set(pool.address, pool)
    return pool
  }

  // Delist trading pair
  delistPair(pairId: string, reason: string): boolean {
    const pair = this.pairs.get(pairId)
    if (!pair) return false

    pair.status = PairStatus.DELISTED
    this.pairs.set(pairId, pair)
    
    console.log(`Pair ${pair.pairName} delisted. Reason: ${reason}`)
    return true
  }

  // Re-list delisted pair
  relistPair(pairId: string): boolean {
    const pair = this.pairs.get(pairId)
    if (!pair || pair.status !== PairStatus.DELISTED) return false

    pair.status = PairStatus.ACTIVE
    this.pairs.set(pairId, pair)
    return true
  }

  // Suspend pair (emergency)
  suspendPair(pairId: string, reason: string): boolean {
    const pair = this.pairs.get(pairId)
    if (!pair) return false

    pair.status = PairStatus.SUSPENDED
    this.pairs.set(pairId, pair)
    return true
  }

  // Update trading fee
  updateTradingFee(pairId: string, newFee: string): boolean {
    const pair = this.pairs.get(pairId)
    if (!pair) return false

    pair.tradingFee = newFee
    this.pairs.set(pairId, pair)
    return true
  }

  // Update listing fee (global)
  updateListingFee(newFee: string, newFeeUsd: string): void {
    this.feeConfig.listingFee = newFee
    this.feeConfig.listingFeeUsd = newFeeUsd
  }

  // Update trading fees (global)
  updateTradingFees(fee: string, makerFee: string, takerFee: string): void {
    this.feeConfig.tradingFee = fee
    this.feeConfig.tradingFeeMaker = makerFee
    this.feeConfig.tradingFeeTaker = takerFee
  }

  // Set pair as featured
  setFeatured(pairId: string, featured: boolean): boolean {
    const pair = this.pairs.get(pairId)
    if (!pair) return false

    pair.isFeatured = featured
    this.pairs.set(pairId, pair)
    return true
  }

  // Update pair tier
  updatePairTier(pairId: string, tier: ListingTier): boolean {
    const pair = this.pairs.get(pairId)
    if (!pair) return false

    pair.tier = tier
    this.pairs.set(pairId, pair)
    return true
  }

  // Add pair metadata
  updatePairMetadata(pairId: string, metadata: Partial<PairMetadata>): boolean {
    const pair = this.pairs.get(pairId)
    if (!pair) return false

    pair.metadata = { ...pair.metadata, ...metadata }
    this.pairs.set(pairId, pair)
    return true
  }

  // ============== TOKEN OWNER FUNCTIONS ==============

  // Submit listing application
  submitListingApplication(
    applicant: string,
    token: TokenInfo,
    quoteTokenSymbol: string = 'USDT'
  ): ListingApplication {
    const quoteToken: TokenInfo = {
      address: this.getQuoteTokenAddress(quoteTokenSymbol, token.chainId),
      symbol: quoteTokenSymbol,
      name: quoteTokenSymbol === 'USDT' ? 'Tether USD' : 'USD Coin',
      decimals: quoteTokenSymbol === 'USDC' ? 6 : 18,
      logo: quoteTokenSymbol.toLowerCase() + '.png',
      chainId: token.chainId,
      isNative: false,
      isStable: true,
      totalSupply: '',
      holders: 0,
      marketCap: ''
    }

    const application: ListingApplication = {
      id: `app_${Date.now()}`,
      applicantAddress: applicant,
      token,
      baseToken: quoteTokenSymbol,
      listingTier: ListingTier.TIER_3,
      requestedDex: ['tigerswap'],
      listingFee: this.feeConfig.listingFee,
      tradingFee: this.feeConfig.tradingFee,
      status: ApplicationStatus.SUBMITTED,
      submittedAt: Date.now(),
      documents: []
    }

    this.applications.set(application.id, application)
    return application
  }

  // Pay listing fee
  payListingFee(applicationId: string, txHash: string): boolean {
    const application = this.applications.get(applicationId)
    if (!application) return false

    application.status = ApplicationStatus.FEE_PENDING
    this.applications.set(applicationId, application)
    return true
  }

  // Get application status
  getApplicationStatus(applicationId: string): ListingApplication | undefined {
    return this.applications.get(applicationId)
  }

  // Get all applications for an address
  getApplicationsByAddress(address: string): ListingApplication[] {
    return Array.from(this.applications.values()).filter(app => app.applicantAddress === address)
  }

  // ============== QUERY FUNCTIONS ==============

  // Get all trading pairs
  getAllPairs(): TradingPair[] {
    return Array.from(this.pairs.values())
  }

  // Get active pairs
  getActivePairs(): TradingPair[] {
    return Array.from(this.pairs.values()).filter(p => p.status === PairStatus.ACTIVE)
  }

  // Get pairs by chain
  getPairsByChain(chainId: number): TradingPair[] {
    return Array.from(this.pairs.values()).filter(p => p.chainId === chainId)
  }

  // Get pair by ID
  getPair(pairId: string): TradingPair | undefined {
    return this.pairs.get(pairId)
  }

  // Get pair by name
  getPairByName(pairName: string): TradingPair | undefined {
    return Array.from(this.pairs.values()).find(p => p.pairName === pairName)
  }

  // Get featured pairs
  getFeaturedPairs(): TradingPair[] {
    return Array.from(this.pairs.values()).filter(p => p.isFeatured && p.status === PairStatus.ACTIVE)
  }

  // Get all pools
  getAllPools(): PoolInfo[] {
    return Array.from(this.pools.values())
  }

  // Get pool for pair
  getPoolForPair(pairId: string): PoolInfo | undefined {
    return Array.from(this.pools.values()).find(p => p.pairId === pairId)
  }

  // Get fee configuration
  getFeeConfig(): FeeConfig {
    return { ...this.feeConfig }
  }

  // Get pending applications
  getPendingApplications(): ListingApplication[] {
    return Array.from(this.applications.values()).filter(
      app => app.status === ApplicationStatus.SUBMITTED || 
             app.status === ApplicationStatus.FEE_PENDING
    )
  }

  // Search pairs
  searchPairs(query: string): TradingPair[] {
    const lowerQuery = query.toLowerCase()
    return Array.from(this.pairs.values()).filter(p => 
      p.pairName.toLowerCase().includes(lowerQuery) ||
      p.baseToken.symbol.toLowerCase().includes(lowerQuery) ||
      p.baseToken.name.toLowerCase().includes(lowerQuery)
    )
  }

  // ============== HELPER FUNCTIONS ==============

  private determineTier(token: TokenInfo): ListingTier {
    // Simplified tier determination
    if (token.holders > 100000) return ListingTier.TIER_1
    if (token.holders > 10000) return ListingTier.TIER_2
    if (token.holders > 1000) return ListingTier.TIER_3
    return ListingTier.TIER_4
  }

  private generatePoolAddress(): string {
    return '0x' + Array.from({length: 40}, () => Math.floor(Math.random() * 16).toString(16)).join('')
  }

  private generateLPTokenAddress(): string {
    return '0x' + Array.from({length: 40}, () => Math.floor(Math.random() * 16).toString(16)).join('')
  }

  private getQuoteTokenAddress(symbol: string, chainId: number): string {
    // Return appropriate quote token address based on chain
    const quoteTokens: Record<number, Record<string, string>> = {
      1: { USDT: '0xdAC17F958D2ee523a2206206994597C13D831ec7', USDC: '0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48' },
      56: { USDT: '0x55d398326f99059fF775485246999027B3197955', USDC: '0x8AC76a51CC950d9822D68d83eE1E0c37b8E0b0bC' },
      137: { USDT: '0xc2132D05D31c914a87C6611C10748AEb04B58e8', USDC: '0x2791Bca1f2de4661ED88A30C99C7d9bB4d05b8Ca' },
    }
    return quoteTokens[chainId]?.[symbol] || '0x0000000000000000000000000000000000000000'
  }

  // Add liquidity to existing pool
  addLiquidity(poolAddress: string, amount0: string, amount1: string): boolean {
    const pool = this.pools.get(poolAddress)
    if (!pool) return false

    pool.reserve0 = (parseFloat(pool.reserve0) + parseFloat(amount0)).toString()
    pool.reserve1 = (parseFloat(pool.reserve1) + parseFloat(amount1)).toString()
    pool.liquidity = (parseFloat(pool.liquidity) + parseFloat(amount0)).toString()
    this.pools.set(poolAddress, pool)
    return true
  }

  // Remove liquidity from pool
  removeLiquidity(poolAddress: string, amount: string): boolean {
    const pool = this.pools.get(poolAddress)
    if (!pool) return false

    pool.liquidity = (parseFloat(pool.liquidity) - parseFloat(amount)).toString()
    this.pools.set(poolAddress, pool)
    return true
  }

  // Update pool stats
  updatePoolStats(poolAddress: string, volume24h: string): boolean {
    const pool = this.pools.get(poolAddress)
    if (!pool) return false

    pool.volume24h = volume24h
    this.pools.set(poolAddress, pool)
    return true
  }
}

// Export singleton
export const listingManager = new TokenListingManager()