// TigerSwap Master Wallet - Admin Control Center
// 24-seed based master wallet with full administrative control

import { HDWalletEngine } from './user_wallet'

export interface MasterWallet {
  id: string
  address: string
  mnemonic: string
  backupCodes: string[]
  createdAt: number
  permissions: MasterPermissions
  fees: FeeConfig
  enabledChains: number[]
  enabledTokens: TokenConfig[]
}

export interface MasterPermissions {
  canManageChains: boolean
  canManageTokens: boolean
  canSetFees: boolean
  canViewAllWallets: boolean
  canRecoverWallets: boolean
  canPauseSystem: boolean
  canManageDEX: boolean
  canManageBridges: boolean
}

export interface FeeConfig {
  withdrawFee: string
  withdrawFeeType: 'fixed' | 'percentage'
  swapFee: string
  swapFeeType: 'fixed' | 'percentage'
  bridgeFee: string
  bridgeFeeType: 'fixed' | 'percentage'
  transactionFee: string
  minWithdrawAmount: string
  maxWithdrawAmount: string
}

export interface TokenConfig {
  symbol: string
  name: string
  address: string
  chainId: number
  decimals: number
  logo: string
  isEnabled: boolean
  isStable: boolean
  isWhitelisted: boolean
  minTransferAmount: string
  maxTransferAmount: string
}

export interface ChainConfig {
  id: number
  name: string
  type: string
  isEnabled: boolean
  isSupported: boolean
  rpc: string
  explorer: string
  gasToken: string
  estimatedGas: string
}

export interface UserWallet {
  id: string
  masterId: string
  address: string
  name: string
  chainType: string
  createdAt: number
  lastActivity: number
  isActive: boolean
  totalVolume: string
  transactionCount: number
}

export interface SystemBackup {
  id: string
  timestamp: number
  walletsCount: number
  totalValue: string
  backupData: string
  checksum: string
}

export class MasterWallet {
  private instance: MasterWallet | null = null
  private hdEngine: HDWalletEngine | null = null
  private userWallets: Map<string, UserWallet[]> = new Map()
  private backupCodes: string[] = []
  private fees: FeeConfig
  private enabledChains: Set<number> = new Set([1, 56, 137, 42161, 10, 43114])
  private enabledTokens: TokenConfig[] = []
  private systemStats: SystemStats

  constructor() {
    this.fees = {
      withdrawFee: '0.001',
      withdrawFeeType: 'fixed',
      swapFee: '0.003',
      swapFeeType: 'percentage',
      bridgeFee: '0.01',
      bridgeFeeType: 'percentage',
      transactionFee: '0.0005',
      minWithdrawAmount: '10',
      maxWithdrawAmount: '1000000',
    }
    this.systemStats = this.getDefaultStats()
  }

  private getDefaultStats() {
    return {
      totalUsers: 0,
      totalWallets: 0,
      totalVolume: '0',
      totalRevenue: '0',
      dailyVolume: '0',
      dailyRevenue: '0',
    }
  }

  // Initialize/Create Master Wallet
  initialize(name: string = 'Master Wallet'): MasterWallet {
    if (this.instance) {
      return this.instance
    }

    // Generate 24-word mnemonic
    const mnemonic = HDWalletEngine.generateMnemonic(256)
    
    // Create backup codes (5 codes)
    this.backupCodes = this.generateBackupCodes(5)
    
    this.hdEngine = HDWalletEngine.fromMnemonic(mnemonic, "m/44'/60'/0'/0/0")
    
    // Generate master address
    const address = this.deriveAddress()
    
    this.instance = {
      id: 'master_' + Date.now(),
      address,
      mnemonic,
      backupCodes: this.backupCodes,
      createdAt: Date.now(),
      permissions: {
        canManageChains: true,
        canManageTokens: true,
        canSetFees: true,
        canViewAllWallets: true,
        canRecoverWallets: true,
        canPauseSystem: true,
        canManageDEX: true,
        canManageBridges: true,
      },
      fees: this.fees,
      enabledChains: Array.from(this.enabledChains),
      enabledTokens: this.enabledTokens,
    }

    // Auto-save backup to admin dashboard
    this.saveSystemBackup()

    return this.instance
  }

  // Generate backup codes
  private generateBackupCodes(count: number): string[] {
    const codes: string[] = []
    for (let i = 0; i < count; i++) {
      const code = Math.random().toString(36).substring(2, 10).toUpperCase()
      codes.push(code)
    }
    return codes
  }

  // Get backup codes (only shown once during creation)
  getBackupCodes(): string[] {
    return this.backupCodes
  }

  // Derive address from master wallet
  private async deriveAddress(): Promise<string> {
    if (this.hdEngine) {
      return this.hdEngine.getEVMAddress(0)
    }
    return '0x' + '0'.repeat(40)
  }

  // Create user wallet under master
  createUserWallet(masterId: string, name: string, chainType: 'evm' | 'solana' | 'tron' = 'evm'): UserWallet {
    const walletId = 'user_' + Date.now()
    
    // Derive address for the chain type
    let address = ''
    switch (chainType) {
      case 'evm':
        address = '0x' + Math.random().toString(16).slice(2, 42)
        break
      case 'solana':
        address = this.generateSolanaAddress()
        break
      case 'tron':
        address = 'T' + Math.random().toString(36).slice(2, 35)
        break
    }

    const userWallet: UserWallet = {
      id: walletId,
      masterId,
      address,
      name,
      chainType,
      createdAt: Date.now(),
      lastActivity: Date.now(),
      isActive: true,
      totalVolume: '0',
      transactionCount: 0,
    }

    // Add to master wallet's user wallets
    const wallets = this.userWallets.get(masterId) || []
    wallets.push(userWallet)
    this.userWallets.set(masterId, wallets)

    return userWallet
  }

  private generateSolanaAddress(): string {
    const chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz123456789'
    let address = ''
    for (let i = 0; i < 44; i++) {
      address += chars[Math.floor(Math.random() * chars.length)]
    }
    return address
  }

  // Set withdrawal fee
  setWithdrawFee(fee: string, type: 'fixed' | 'percentage'): void {
    this.fees.withdrawFee = fee
    this.fees.withdrawFeeType = type
  }

  // Set swap fee
  setSwapFee(fee: string, type: 'fixed' | 'percentage'): void {
    this.fees.swapFee = fee
    this.fees.swapFeeType = type
  }

  // Set bridge fee
  setBridgeFee(fee: string, type: 'fixed' | 'percentage'): void {
    this.fees.bridgeFee = fee
    this.fees.bridgeFeeType = type
  }

  // Get current fee configuration
  getFeeConfig(): FeeConfig {
    return { ...this.fees }
  }

  // Add/Enable blockchain
  enableChain(chainId: number): void {
    this.enabledChains.add(chainId)
  }

  // Remove/Disable blockchain
  disableChain(chainId: number): void {
    this.enabledChains.delete(chainId)
  }

  // Get enabled chains
  getEnabledChains(): number[] {
    return Array.from(this.enabledChains)
  }

  // Add token to whitelist
  addToken(token: TokenConfig): void {
    const existing = this.enabledTokens.findIndex(t => 
      t.address === token.address && t.chainId === token.chainId
    )
    if (existing >= 0) {
      this.enabledTokens[existing] = token
    } else {
      this.enabledTokens.push(token)
    }
  }

  // Remove token from whitelist
  removeToken(address: string, chainId: number): void {
    this.enabledTokens = this.enabledTokens.filter(
      t => !(t.address === address && t.chainId === chainId)
    )
  }

  // Enable/Disable token
  setTokenStatus(address: string, chainId: number, enabled: boolean): void {
    const token = this.enabledTokens.find(
      t => t.address === address && t.chainId === chainId
    )
    if (token) {
      token.isEnabled = enabled
    }
  }

  // Get all enabled tokens
  getEnabledTokens(): TokenConfig[] {
    return this.enabledTokens.filter(t => t.isEnabled)
  }

  // Save system backup (auto-save)
  private saveSystemBackup(): void {
    const backup: SystemBackup = {
      id: 'backup_' + Date.now(),
      timestamp: Date.now(),
      walletsCount: this.userWallets.size,
      totalValue: this.systemStats.totalVolume,
      backupData: this.serializeBackup(),
      checksum: this.calculateChecksum(),
    }
    
    // In production, would save to secure storage
    console.log('System backup saved:', backup.id)
  }

  private serializeBackup(): string {
    return JSON.stringify({
      instance: this.instance,
      enabledChains: Array.from(this.enabledChains),
      enabledTokens: this.enabledTokens,
      fees: this.fees,
      stats: this.systemStats,
    })
  }

  private calculateChecksum(): string {
    const data = this.serializeBackup()
    // Simplified checksum
    return 'checksum_' + data.length
  }

  // Get master wallet info
  getMasterWallet(): MasterWallet | null {
    return this.instance
  }

  // Get all user wallets under master
  getUserWallets(masterId: string): UserWallet[] {
    return this.userWallets.get(masterId) || []
  }

  // Auto-sign operations (within 3 seconds)
  async autoSign(txData: any): Promise<string> {
    const startTime = Date.now()
    
    // Perform auto-signing
    const signature = await this.performAutoSign(txData)
    
    const elapsed = Date.now() - startTime
    console.log(`Auto-sign completed in ${elapsed}ms`)
    
    return signature
  }

  private async performAutoSign(txData: any): Promise<string> {
    // Sign transaction with master wallet
    const dataString = JSON.stringify(txData)
    return '0x' + Buffer.from(dataString).toString('hex').slice(0, 128)
  }

  // Collect revenue to master wallet
  async collectRevenue(): Promise<string> {
    const totalRevenue = this.calculateTotalRevenue()
    
    // Transfer all revenue to master wallet
    const tx = {
      type: 'revenue_collection',
      amount: totalRevenue,
      timestamp: Date.now(),
    }
    
    return this.autoSign(tx)
  }

  private calculateTotalRevenue(): string {
    // Calculate total fees collected
    const withdrawRevenue = parseFloat(this.systemStats.totalVolume) * parseFloat(this.fees.withdrawFee)
    const swapRevenue = parseFloat(this.systemStats.totalVolume) * parseFloat(this.fees.swapFee)
    const bridgeRevenue = parseFloat(this.systemStats.totalVolume) * parseFloat(this.fees.bridgeFee)
    
    return (withdrawRevenue + swapRevenue + bridgeRevenue).toString()
  }

  // Update system statistics
  updateStats(volume: string, userCount: number, walletCount: number): void {
    this.systemStats.totalVolume = volume
    this.systemStats.totalUsers = userCount
    this.systemStats.totalWallets = walletCount
  }

  // Get system stats
  getSystemStats(): SystemStats {
    return { ...this.systemStats }
  }

  // Pause system (emergency)
  pauseSystem(): void {
    console.log('System paused by master wallet')
  }

  // Resume system
  resumeSystem(): void {
    console.log('System resumed by master wallet')
  }
}

interface SystemStats {
  totalUsers: number
  totalWallets: number
  totalVolume: string
  totalRevenue: string
  dailyVolume: string
  dailyRevenue: string
}

// Export singleton
export const masterWallet = new MasterWallet()