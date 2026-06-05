// TigerSwap - Complete Blockchain Network Management System
// Supports unlimited EVM and Non-EVM chains with full admin control

export interface Blockchain {
  id: string
  chainId: number
  name: string
  type: 'evm' | 'solana' | 'tron' | 'bitcoin' | 'sui' | 'aptos' | 'near' | 'cosmos' | 'osmosis' | 'injective' | 'ton' | 'cardano' | 'polkadot' | 'avalanche' | 'algorand' | 'flow' | 'hedera'
  symbol: string
  decimals: number
  rpc: string
  wsRpc?: string
  explorer: string
  explorerApi?: string
  explorerName?: string
  logo: string
  isNative: boolean
  wrappedToken?: string
  isEnabled: boolean
  isTestnet: boolean
  addedAt: number
  addedBy: string
  config: ChainConfig
  capabilities: ChainCapabilities
  gasSettings: GasSettings
  tokens: string[] // token contract addresses
  metadata: ChainMetadata
}

export interface ChainConfig {
  chainType: string
  networkId: number
  currency: {
    name: string
    symbol: string
    decimals: number
  }
  httpHeaders?: Record<string, string>
  retryConfig?: {
    maxRetries: number
    retryDelay: number
    timeout: number
  }
  rateLimit?: {
    requestsPerSecond: number
    burstLimit: number
  }
}

export interface ChainCapabilities {
  swap: boolean
  bridge: boolean
  staking: boolean
  farming: boolean
  nft: boolean
  dappBrowser: boolean
  multiSig: boolean
  hardwareWallet: boolean
  delegation: boolean
  governance: boolean
}

export interface GasSettings {
  gasToken: string
  minGasPrice: string
  maxGasPrice: string
  avgGasPrice: string
  gasLimitMultiplier: number
  EIP1559: boolean
  gasStationUrl?: string
}

export interface ChainMetadata {
  color: string
  bgColor: string
  description?: string
  website?: string
  documentation?: string
  socialLinks?: {
    twitter?: string
    discord?: string
    telegram?: string
    github?: string
  }
  supportedWalletTypes?: ('metamask' | 'walletconnect' | 'coinbase' | 'phantom' | 'solflare' | 'keplr' | 'ledger' | 'trezor')[]
  maxTransactionSize?: string
  averageBlockTime?: number
  blockExplorerUrls?: string[]
}

export interface Token {
  address: string
  chainId: number
  symbol: string
  name: string
  decimals: number
  logo: string
  isNative: boolean
  isStable: boolean
  isWhitelisted: boolean
  coingeckoId?: string
  price?: string
  change24h?: number
  volume24h?: string
  liquidity?: string
  addedAt: number
  addedBy: string
}

export interface UserWallet {
  id: string
  address: string
  chainType: string
  chainId: number
  createdAt: number
  name?: string
  balances: Map<string, TokenBalance>
}

export interface TokenBalance {
  symbol: string
  amount: string
  value: string
  locked?: string
}

// Blockchain Manager - Core Chain Management System
export class BlockchainManager {
  private chains: Map<string, Blockchain> = new Map()
  private tokens: Map<string, Token[]> = new Map()
  private userWallets: Map<string, UserWallet[]> = new Map()
  private masterWallet: string = ''

  constructor() {
    this.initializeDefaultChains()
  }

  // Initialize with default supported chains
  private initializeDefaultChains(): void {
    const defaultChains: Blockchain[] = [
      // EVM Chains
      this.createChain({
        chainId: 1,
        name: 'Ethereum',
        type: 'evm',
        symbol: 'ETH',
        decimals: 18,
        rpc: 'https://eth.llamarpc.com',
        explorer: 'https://etherscan.io',
        isEnabled: true,
        isNative: true,
        capabilities: { swap: true, bridge: true, staking: true, farming: true, nft: true, dappBrowser: true, multiSig: true, hardwareWallet: true, delegation: true, governance: true },
        gasSettings: { gasToken: 'ETH', minGasPrice: '20', maxGasPrice: '500', avgGasPrice: '30', gasLimitMultiplier: 1.2, EIP1559: true },
        metadata: { color: '#627EEA', bgColor: '#627EEA20', description: 'Ethereum Mainnet' }
      }),
      this.createChain({
        chainId: 56,
        name: 'BNB Chain',
        type: 'evm',
        symbol: 'BNB',
        decimals: 18,
        rpc: 'https://bsc.llamarpc.com',
        explorer: 'https://bscscan.com',
        isEnabled: true,
        isNative: true,
        capabilities: { swap: true, bridge: true, staking: true, farming: true, nft: true, dappBrowser: true, multiSig: true, hardwareWallet: true, delegation: true, governance: true },
        gasSettings: { gasToken: 'BNB', minGasPrice: '3', maxGasPrice: '100', avgGasPrice: '5', gasLimitMultiplier: 1.1, EIP1559: false },
        metadata: { color: '#F3BA2F', bgColor: '#F3BA2F20' }
      }),
      this.createChain({
        chainId: 137,
        name: 'Polygon',
        type: 'evm',
        symbol: 'MATIC',
        decimals: 18,
        rpc: 'https://polygon.llamarpc.com',
        explorer: 'https://polygonscan.com',
        isEnabled: true,
        isNative: true,
        capabilities: { swap: true, bridge: true, staking: true, farming: true, nft: true, dappBrowser: true, multiSig: true, hardwareWallet: true, delegation: true, governance: true },
        gasSettings: { gasToken: 'MATIC', minGasPrice: '0.1', maxGasPrice: '100', avgGasPrice: '1', gasLimitMultiplier: 1.2, EIP1559: false },
        metadata: { color: '#8247E5', bgColor: '#8247E520' }
      }),
      this.createChain({
        chainId: 42161,
        name: 'Arbitrum One',
        type: 'evm',
        symbol: 'ETH',
        decimals: 18,
        rpc: 'https://arbitrum.llamarpc.com',
        explorer: 'https://arbiscan.io',
        isEnabled: true,
        isNative: true,
        capabilities: { swap: true, bridge: true, staking: true, farming: true, nft: true, dappBrowser: true, multiSig: true, hardwareWallet: true, delegation: true, governance: false },
        gasSettings: { gasToken: 'ETH', minGasPrice: '0.1', maxGasPrice: '50', avgGasPrice: '0.2', gasLimitMultiplier: 1.3, EIP1559: true },
        metadata: { color: '#28A0F0', bgColor: '#28A0F020' }
      }),
      this.createChain({
        chainId: 10,
        name: 'Optimism',
        type: 'evm',
        symbol: 'ETH',
        decimals: 18,
        rpc: 'https://optimism.llamarpc.com',
        explorer: 'https://optimistic.etherscan.io',
        isEnabled: true,
        isNative: true,
        capabilities: { swap: true, bridge: true, staking: true, farming: true, nft: true, dappBrowser: true, multiSig: true, hardwareWallet: true, delegation: true, governance: false },
        gasSettings: { gasToken: 'ETH', minGasPrice: '0.001', maxGasPrice: '10', avgGasPrice: '0.005', gasLimitMultiplier: 1.2, EIP1559: true },
        metadata: { color: '#FF0420', bgColor: '#FF042020' }
      }),
      this.createChain({
        chainId: 43114,
        name: 'Avalanche C-Chain',
        type: 'evm',
        symbol: 'AVAX',
        decimals: 18,
        rpc: 'https://avax.llamarpc.com',
        explorer: 'https://snowtrace.io',
        isEnabled: true,
        isNative: true,
        capabilities: { swap: true, bridge: true, staking: true, farming: true, nft: true, dappBrowser: true, multiSig: true, hardwareWallet: true, delegation: true, governance: true },
        gasSettings: { gasToken: 'AVAX', minGasPrice: '25', maxGasPrice: '500', avgGasPrice: '30', gasLimitMultiplier: 1.1, EIP1559: false },
        metadata: { color: '#E84142', bgColor: '#E8414220' }
      }),
      this.createChain({
        chainId: 8453,
        name: 'Base',
        type: 'evm',
        symbol: 'ETH',
        decimals: 18,
        rpc: 'https://base.llamarpc.com',
        explorer: 'https://basescan.org',
        isEnabled: true,
        isNative: true,
        capabilities: { swap: true, bridge: true, staking: true, farming: true, nft: true, dappBrowser: true, multiSig: true, hardwareWallet: true, delegation: true, governance: false },
        gasSettings: { gasToken: 'ETH', minGasPrice: '0.01', maxGasPrice: '100', avgGasPrice: '0.1', gasLimitMultiplier: 1.2, EIP1559: true },
        metadata: { color: '#0052FF', bgColor: '#0052FF20' }
      }),
      this.createChain({
        chainId: 250,
        name: 'Fantom',
        type: 'evm',
        symbol: 'FTM',
        decimals: 18,
        rpc: 'https://fantom.llamarpc.com',
        explorer: 'https://ftmscan.com',
        isEnabled: true,
        isNative: true,
        capabilities: { swap: true, bridge: true, staking: true, farming: true, nft: true, dappBrowser: true, multiSig: true, hardwareWallet: true, delegation: true, governance: true },
        gasSettings: { gasToken: 'FTM', minGasPrice: '0.001', maxGasPrice: '100', avgGasPrice: '0.01', gasLimitMultiplier: 1.1, EIP1559: false },
        metadata: { color: '#13B5EC', bgColor: '#13B5EC20' }
      }),
      // Non-EVM Chains
      this.createChain({
        chainId: -1,
        name: 'Solana',
        type: 'solana',
        symbol: 'SOL',
        decimals: 9,
        rpc: 'https://api.mainnet-beta.solana.com',
        explorer: 'https://solscan.io',
        isEnabled: true,
        isNative: true,
        capabilities: { swap: true, bridge: true, staking: true, farming: false, nft: true, dappBrowser: true, multiSig: false, hardwareWallet: true, delegation: true, governance: true },
        gasSettings: { gasToken: 'SOL', minGasPrice: '0.00005', maxGasPrice: '0.01', avgGasPrice: '0.00025', gasLimitMultiplier: 1, EIP1559: false },
        metadata: { color: '#9945FF', bgColor: '#9945FF20', description: 'Solana Mainnet' }
      }),
      this.createChain({
        chainId: -2,
        name: 'Tron',
        type: 'tron',
        symbol: 'TRX',
        decimals: 6,
        rpc: 'https://api.trongrid.io',
        explorer: 'https://tronscan.org',
        isEnabled: true,
        isNative: true,
        capabilities: { swap: true, bridge: true, staking: true, farming: true, nft: true, dappBrowser: true, multiSig: true, hardwareWallet: true, delegation: false, governance: false },
        gasSettings: { gasToken: 'TRX', minGasPrice: '1', maxGasPrice: '100', avgGasPrice: '10', gasLimitMultiplier: 1, EIP1559: false },
        metadata: { color: '#EF0027', bgColor: '#EF002720' }
      }),
      this.createChain({
        chainId: -3,
        name: 'Sui',
        type: 'sui',
        symbol: 'SUI',
        decimals: 9,
        rpc: 'https://fullnode.mainnet.sui.io',
        explorer: 'https://suiscan.xyz',
        isEnabled: true,
        isNative: true,
        capabilities: { swap: true, bridge: true, staking: true, farming: false, nft: true, dappBrowser: true, multiSig: true, hardwareWallet: true, delegation: true, governance: false },
        gasSettings: { gasToken: 'SUI', minGasPrice: '0.00001', maxGasPrice: '0.001', avgGasPrice: '0.0001', gasLimitMultiplier: 1, EIP1559: false },
        metadata: { color: '#6F BCEF', bgColor: '#6F BCEF20' }
      }),
      this.createChain({
        chainId: -4,
        name: 'Aptos',
        type: 'aptos',
        symbol: 'APT',
        decimals: 8,
        rpc: 'https://fullnode.aptoslabs.com',
        explorer: 'https://aptoscan.com',
        isEnabled: true,
        isNative: true,
        capabilities: { swap: true, bridge: true, staking: true, farming: false, nft: true, dappBrowser: true, multiSig: true, hardwareWallet: true, delegation: true, governance: true },
        gasSettings: { gasToken: 'APT', minGasPrice: '0.0001', maxGasPrice: '1', avgGasPrice: '0.001', gasLimitMultiplier: 1, EIP1559: false },
        metadata: { color: '#3D2847', bgColor: '#3D284720' }
      }),
    ]

    defaultChains.forEach(chain => {
      this.chains.set(chain.id, chain)
    })
  }

  private createChain(config: Partial<Blockchain>): Blockchain {
    return {
      id: config.id || `chain_${config.chainId}`,
      chainId: config.chainId || 1,
      name: config.name || 'Unknown',
      type: config.type || 'evm',
      symbol: config.symbol || 'ETH',
      decimals: config.decimals || 18,
      rpc: config.rpc || '',
      explorer: config.explorer || '',
      logo: config.logo || '',
      isNative: config.isNative ?? true,
      wrappedToken: config.wrappedToken,
      isEnabled: config.isEnabled ?? true,
      isTestnet: config.isTestnet ?? false,
      addedAt: config.addedAt || Date.now(),
      addedBy: config.addedBy || 'system',
      config: config.config || { chainType: 'mainnet', networkId: 1, currency: { name: config.name || 'Native', symbol: config.symbol || 'TOKEN', decimals: config.decimals || 18 } },
      capabilities: config.capabilities || { swap: true, bridge: true, staking: true, farming: false, nft: true, dappBrowser: true, multiSig: false, hardwareWallet: true, delegation: true, governance: false },
      gasSettings: config.gasSettings || { gasToken: config.symbol || 'ETH', minGasPrice: '0', maxGasPrice: '0', avgGasPrice: '0', gasLimitMultiplier: 1, EIP1559: false },
      tokens: config.tokens || [],
      metadata: config.metadata || { color: '#888888', bgColor: '#88888820' }
    }
  }

  // Add new EVM chain
  addEVMChain(chainData: {
    chainId: number
    name: string
    symbol: string
    decimals: number
    rpc: string
    explorer: string
    wrappedToken?: string
    currencyName?: string
  }, addedBy: string = 'admin'): Blockchain {
    const chain = this.createChain({
      chainId: chainData.chainId,
      name: chainData.name,
      type: 'evm',
      symbol: chainData.symbol,
      decimals: chainData.decimals,
      rpc: chainData.rpc,
      explorer: chainData.explorer,
      wrappedToken: chainData.wrappedToken,
      isEnabled: true,
      isNative: true,
      addedBy,
      config: {
        chainType: 'mainnet',
        networkId: chainData.chainId,
        currency: { name: chainData.currencyName || chainData.name, symbol: chainData.symbol, decimals: chainData.decimals }
      },
      capabilities: { swap: true, bridge: true, staking: true, farming: true, nft: true, dappBrowser: true, multiSig: true, hardwareWallet: true, delegation: true, governance: true },
      gasSettings: { gasToken: chainData.symbol, minGasPrice: '1', maxGasPrice: '1000', avgGasPrice: '10', gasLimitMultiplier: 1.2, EIP1559: false },
      metadata: { color: '#888888', bgColor: '#88888820' }
    })
    
    this.chains.set(chain.id, chain)
    console.log(`Added EVM chain: ${chain.name} (Chain ID: ${chain.chainId})`)
    return chain
  }

  // Add new Non-EVM chain (Solana, Tron, Sui, Aptos, etc.)
  addNonEVMChain(chainData: {
    name: string
    type: 'solana' | 'tron' | 'sui' | 'aptos' | 'near' | 'cosmos' | 'osmosis' | 'injective' | 'ton' | 'cardano' | 'polkadot' | 'avalanche' | 'algorand' | 'flow' | 'hedera'
    symbol: string
    decimals: number
    rpc: string
    explorer: string
  }, addedBy: string = 'admin'): Blockchain {
    const chainId = this.getNextNegativeChainId()
    
    const chain = this.createChain({
      chainId,
      name: chainData.name,
      type: chainData.type,
      symbol: chainData.symbol,
      decimals: chainData.decimals,
      rpc: chainData.rpc,
      explorer: chainData.explorer,
      isEnabled: true,
      isNative: true,
      addedBy,
      config: {
        chainType: 'mainnet',
        networkId: Math.abs(chainId),
        currency: { name: chainData.name, symbol: chainData.symbol, decimals: chainData.decimals }
      },
      capabilities: this.getDefaultCapabilities(chainData.type),
      gasSettings: this.getDefaultGasSettings(chainData.type, chainData.symbol),
      metadata: { color: '#888888', bgColor: '#88888820' }
    })
    
    this.chains.set(chain.id, chain)
    console.log(`Added Non-EVM chain: ${chain.name} (Type: ${chain.type})`)
    return chain
  }

  private getNextNegativeChainId(): number {
    const existingIds = Array.from(this.chains.values()).map(c => c.chainId)
    const negativeIds = existingIds.filter(id => id < 0)
    if (negativeIds.length === 0) return -1
    return Math.min(...negativeIds) - 1
  }

  private getDefaultCapabilities(type: string): ChainCapabilities {
    const base: ChainCapabilities = { swap: true, bridge: true, staking: true, farming: false, nft: true, dappBrowser: true, multiSig: false, hardwareWallet: true, delegation: true, governance: false }
    
    switch (type) {
      case 'solana':
        return { ...base, multiSig: false, governance: true }
      case 'tron':
        return { ...base, farming: true, multiSig: true }
      case 'sui':
        return { ...base, governance: false }
      case 'aptos':
        return { ...base, governance: true }
      case 'near':
        return { ...base, multiSig: true, governance: true }
      case 'cosmos':
        return { ...base, governance: true }
      default:
        return base
    }
  }

  private getDefaultGasSettings(type: string, symbol: string): GasSettings {
    const defaults: Record<string, GasSettings> = {
      solana: { gasToken: symbol, minGasPrice: '0.00005', maxGasPrice: '0.01', avgGasPrice: '0.00025', gasLimitMultiplier: 1, EIP1559: false },
      tron: { gasToken: symbol, minGasPrice: '1', maxGasPrice: '100', avgGasPrice: '10', gasLimitMultiplier: 1, EIP1559: false },
      sui: { gasToken: symbol, minGasPrice: '0.00001', maxGasPrice: '0.001', avgGasPrice: '0.0001', gasLimitMultiplier: 1, EIP1559: false },
      aptos: { gasToken: symbol, minGasPrice: '0.0001', maxGasPrice: '1', avgGasPrice: '0.001', gasLimitMultiplier: 1, EIP1559: false },
    }
    return defaults[type] || { gasToken: symbol, minGasPrice: '0.01', maxGasPrice: '100', avgGasPrice: '1', gasLimitMultiplier: 1, EIP1559: false }
  }

  // Update chain
  updateChain(chainId: string, updates: Partial<Blockchain>): boolean {
    const chain = this.chains.get(chainId)
    if (!chain) return false
    
    const updated = { ...chain, ...updates, id: chainId }
    this.chains.set(chainId, updated)
    console.log(`Updated chain: ${chain.name}`)
    return true
  }

  // Remove chain
  removeChain(chainId: string): boolean {
    const chain = this.chains.get(chainId)
    if (!chain) return false
    
    this.chains.delete(chainId)
    console.log(`Removed chain: ${chain.name}`)
    return true
  }

  // Enable/Disable chain
  toggleChain(chainId: string, enabled: boolean): boolean {
    const chain = this.chains.get(chainId)
    if (!chain) return false
    
    chain.isEnabled = enabled
    this.chains.set(chainId, chain)
    console.log(`Chain ${chain.name} ${enabled ? 'enabled' : 'disabled'}`)
    return true
  }

  // Get all chains
  getAllChains(): Blockchain[] {
    return Array.from(this.chains.values())
  }

  // Get enabled chains
  getEnabledChains(): Blockchain[] {
    return Array.from(this.chains.values()).filter(c => c.isEnabled)
  }

  // Get EVM chains
  getEVMChains(): Blockchain[] {
    return Array.from(this.chains.values()).filter(c => c.type === 'evm')
  }

  // Get Non-EVM chains
  getNonEVMChains(): Blockchain[] {
    return Array.from(this.chains.values()).filter(c => c.type !== 'evm')
  }

  // Get chain by ID
  getChain(chainId: string): Blockchain | undefined {
    return this.chains.get(chainId)
  }

  // Add token to chain
  addToken(chainId: string, token: Token): void {
    const chain = this.chains.get(chainId)
    if (!chain) return
    
    const tokens = this.tokens.get(chainId) || []
    const existing = tokens.findIndex(t => t.address === token.address)
    if (existing >= 0) {
      tokens[existing] = token
    } else {
      tokens.push(token)
    }
    this.tokens.set(chainId, tokens)
    
    if (!chain.tokens.includes(token.address)) {
      chain.tokens.push(token.address)
      this.chains.set(chainId, chain)
    }
  }

  // Remove token from chain
  removeToken(chainId: string, tokenAddress: string): void {
    const chain = this.chains.get(chainId)
    if (!chain) return
    
    const tokens = this.tokens.get(chainId) || []
    this.tokens.set(chainId, tokens.filter(t => t.address !== tokenAddress))
    chain.tokens = chain.tokens.filter(t => t !== tokenAddress)
    this.chains.set(chainId, chain)
  }

  // Get tokens for chain
  getChainTokens(chainId: string): Token[] {
    return this.tokens.get(chainId) || []
  }

  // Get supported chain types
  getSupportedChainTypes(): string[] {
    return ['evm', 'solana', 'tron', 'sui', 'aptos', 'near', 'cosmos', 'osmosis', 'injective', 'ton', 'cardano', 'polkadot', 'avalanche', 'algorand', 'flow', 'hedera']
  }

  // Validate RPC connection
  async validateRPC(rpcUrl: string): Promise<boolean> {
    try {
      // In production, would make actual RPC call to validate
      console.log(`Validating RPC: ${rpcUrl}`)
      return true
    } catch {
      return false
    }
  }
}

// Export singleton
export const blockchainManager = new BlockchainManager()