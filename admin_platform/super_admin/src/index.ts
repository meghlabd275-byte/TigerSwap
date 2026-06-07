/**
 * TigerSwap Admin Platform - Super Admin Module
 * 
 * Enterprise-grade admin panel for platform management.
 * Zero external dependencies - fully native implementation.
 * 
 * Features:
 * - System configuration
 * - User management
 * - Fee management
 * - Chain management
 * - Emergency controls
 * 
 * @author TigerSwap
 * @version 1.0.0
 */

// ============================================================================
// Types & Interfaces
// ============================================================================

export interface SystemConfig {
  name: string;
  version: string;
  maintenanceMode: boolean;
  paused: boolean;
}

export interface UserRecord {
  id: string;
  address: string;
  email: string;
  status: 'active' | 'suspended' | 'banned';
  kycLevel: number;
  createdAt: number;
  lastActive: number;
}

export interface FeeConfig {
  protocolFee: number;
  gasFee: number;
  referralFee: number;
}

export interface ChainConfig {
  chainId: number;
  name: string;
  enabled: boolean;
  rpcUrl: string;
  explorerUrl: string;
}

export interface EmergencyAction {
  type: 'pause_all' | 'unpause_all' | 'pause_bridge' | 'pause_swap' | 'freeze_funds';
  reason: string;
  timestamp: number;
  executedBy: string;
}

// ============================================================================
// Super Admin
// ============================================================================

export class SuperAdmin {
  private systemConfig: SystemConfig;
  private users: Map<string, UserRecord>;
  private fees: Map<string, FeeConfig>;
  private chains: Map<number, ChainConfig>;
  private emergencyActions: EmergencyAction[];
  private adminKey: string;

  constructor(adminKey: string) {
    this.adminKey = adminKey;
    this.systemConfig = {
      name: 'TigerSwap',
      version: '1.0.0',
      maintenanceMode: false,
      paused: false,
    };
    this.users = new Map();
    this.fees = new Map();
    this.chains = new Map();
    this.emergencyActions = [];
  }

  /**
   * Get system status
   */
  getSystemStatus(): SystemConfig {
    return { ...this.systemConfig };
  }

  /**
   * Update system config
   */
  updateSystemConfig(updates: Partial<SystemConfig>): void {
    Object.assign(this.systemConfig, updates);
  }

  /**
   * Set maintenance mode
   */
  setMaintenanceMode(enabled: boolean): void {
    this.systemConfig.maintenanceMode = enabled;
  }

  /**
   * Pause/Unpause all trading
   */
  setPaused(paused: boolean): void {
    this.systemConfig.paused = paused;
  }

  // ============================================================================
  // User Management
  // ============================================================================

  /**
   * Create user
   */
  createUser(address: string, email: string): UserRecord {
    const user: UserRecord = {
      id: `user_${Date.now()}`,
      address,
      email,
      status: 'active',
      kycLevel: 0,
      createdAt: Date.now(),
      lastActive: Date.now(),
    };
    this.users.set(user.id, user);
    return user;
  }

  /**
   * Get user
   */
  getUser(userId: string): UserRecord | null {
    return this.users.get(userId) || null;
  }

  /**
   * Get user by address
   */
  getUserByAddress(address: string): UserRecord | null {
    for (const user of this.users.values()) {
      if (user.address.toLowerCase() === address.toLowerCase()) {
        return user;
      }
    }
    return null;
  }

  /**
   * Update user
   */
  updateUser(userId: string, updates: Partial<UserRecord>): void {
    const user = this.users.get(userId);
    if (user) {
      Object.assign(user, updates);
    }
  }

  /**
   * Suspend user
   */
  suspendUser(userId: string): void {
    const user = this.users.get(userId);
    if (user) {
      user.status = 'suspended';
    }
  }

  /**
   * Ban user
   */
  banUser(userId: string): void {
    const user = this.users.get(userId);
    if (user) {
      user.status = 'banned';
    }
  }

  /**
   * Approve KYC
   */
  approveKYC(userId: string, level: number): void {
    const user = this.users.get(userId);
    if (user) {
      user.kycLevel = level;
    }
  }

  /**
   * Get all users
   */
  getAllUsers(): UserRecord[] {
    return Array.from(this.users.values());
  }

  // ============================================================================
  // Fee Management
  // ============================================================================

  /**
   * Set protocol fee
   */
  setProtocolFee(token: string, fee: number): void {
    const config = this.fees.get(token) || { protocolFee: 0, gasFee: 0, referralFee: 0 };
    config.protocolFee = fee;
    this.fees.set(token, config);
  }

  /**
   * Set gas fee
   */
  setGasFee(token: string, fee: number): void {
    const config = this.fees.get(token) || { protocolFee: 0, gasFee: 0, referralFee: 0 };
    config.gasFee = fee;
    this.fees.set(token, config);
  }

  /**
   * Get fee config
   */
  getFeeConfig(token: string): FeeConfig {
    return this.fees.get(token) || { protocolFee: 30, gasFee: 10, referralFee: 20 };
  }

  /**
   * Get all fee configs
   */
  getAllFeeConfigs(): FeeConfig[] {
    return Array.from(this.fees.values());
  }

  // ============================================================================
  // Chain Management
  // ============================================================================

  /**
   * Add chain
   */
  addChain(config: ChainConfig): void {
    this.chains.set(config.chainId, config);
  }

  /**
   * Get chain
   */
  getChain(chainId: number): ChainConfig | null {
    return this.chains.get(chainId) || null;
  }

  /**
   * Enable chain
   */
  enableChain(chainId: number): void {
    const chain = this.chains.get(chainId);
    if (chain) {
      chain.enabled = true;
    }
  }

  /**
   * Disable chain
   */
  disableChain(chainId: number): void {
    const chain = this.chains.get(chainId);
    if (chain) {
      chain.enabled = false;
    }
  }

  /**
   * Update chain RPC
   */
  updateChainRPC(chainId: number, rpcUrl: string): void {
    const chain = this.chains.get(chainId);
    if (chain) {
      chain.rpcUrl = rpcUrl;
    }
  }

  /**
   * Get all chains
   */
  getAllChains(): ChainConfig[] {
    return Array.from(this.chains.values());
  }

  // ============================================================================
  // Emergency Controls
  // ============================================================================

  /**
   * Execute emergency action
   */
  executeEmergencyAction(type: EmergencyAction['type'], reason: string, adminAddress: string): void {
    const action: EmergencyAction = {
      type,
      reason,
      timestamp: Date.now(),
      executedBy: adminAddress,
    };

    this.emergencyActions.push(action);

    switch (type) {
      case 'pause_all':
        this.systemConfig.paused = true;
        break;
      case 'unpause_all':
        this.systemConfig.paused = false;
        break;
      case 'pause_bridge':
      case 'pause_swap':
      case 'freeze_funds':
        // Handle specific emergency actions
        break;
    }
  }

  /**
   * Get emergency actions
   */
  getEmergencyActions(limit: number = 100): EmergencyAction[] {
    return this.emergencyActions.slice(-limit);
  }

  // ============================================================================
  // Analytics
  // ============================================================================

  /**
   * Get dashboard data
   */
  getDashboard(): {
    totalUsers: number;
    activeUsers: number;
    totalChains: number;
    enabledChains: number;
    paused: boolean;
    maintenance: boolean;
    emergencyActions: number;
  } {
    let activeUsers = 0;
    for (const user of this.users.values()) {
      if (user.status === 'active') activeUsers++;
    }

    let enabledChains = 0;
    for (const chain of this.chains.values()) {
      if (chain.enabled) enabledChains++;
    }

    return {
      totalUsers: this.users.size,
      activeUsers,
      totalChains: this.chains.size,
      enabledChains,
      paused: this.systemConfig.paused,
      maintenance: this.systemConfig.maintenanceMode,
      emergencyActions: this.emergencyActions.length,
    };
  }
}

// ============================================================================
// Admin Roles
// ============================================================================

export type AdminRole = 
  | 'super_admin'
  | 'operations_admin'
  | 'treasury_admin'
  | 'compliance_admin'
  | 'support_admin'
  | 'partner_admin'
  | 'market_maker_admin'
  | 'bot_admin'
  | 'chain_admin'
  | 'dex_admin'
  | 'bridge_admin'
  | 'liquidity_admin'
  | 'fee_admin'
  | 'user_admin'
  | 'analytics_admin'
  | 'audit_admin'
  | 'security_admin'
  | 'emergency_admin';

export interface AdminPermissions {
  role: AdminRole;
  canPause: boolean;
  canSuspend: boolean;
  canModifyFees: boolean;
  canManageChains: boolean;
  canViewAudit: boolean;
  canExecuteEmergency: boolean;
}

export const ROLE_PERMISSIONS: Record<AdminRole, AdminPermissions> = {
  super_admin: {
    role: 'super_admin',
    canPause: true,
    canSuspend: true,
    canModifyFees: true,
    canManageChains: true,
    canViewAudit: true,
    canExecuteEmergency: true,
  },
  operations_admin: {
    role: 'operations_admin',
    canPause: true,
    canSuspend: false,
    canModifyFees: false,
    canManageChains: true,
    canViewAudit: true,
    canExecuteEmergency: false,
  },
  treasury_admin: {
    role: 'treasury_admin',
    canPause: false,
    canSuspend: false,
    canModifyFees: true,
    canManageChains: false,
    canViewAudit: true,
    canExecuteEmergency: false,
  },
  compliance_admin: {
    role: 'compliance_admin',
    canPause: false,
    canSuspend: true,
    canModifyFees: false,
    canManageChains: false,
    canViewAudit: true,
    canExecuteEmergency: false,
  },
  support_admin: {
    role: 'support_admin',
    canPause: false,
    canSuspend: false,
    canModifyFees: false,
    canManageChains: false,
    canViewAudit: false,
    canExecuteEmergency: false,
  },
  partner_admin: {
    role: 'partner_admin',
    canPause: false,
    canSuspend: false,
    canModifyFees: false,
    canManageChains: false,
    canViewAudit: false,
    canExecuteEmergency: false,
  },
  market_maker_admin: {
    role: 'market_maker_admin',
    canPause: false,
    canSuspend: false,
    canModifyFees: false,
    canManageChains: false,
    canViewAudit: true,
    canExecuteEmergency: false,
  },
  bot_admin: {
    role: 'bot_admin',
    canPause: false,
    canSuspend: false,
    canModifyFees: false,
    canManageChains: false,
    canViewAudit: false,
    canExecuteEmergency: false,
  },
  chain_admin: {
    role: 'chain_admin',
    canPause: false,
    canSuspend: false,
    canModifyFees: false,
    canManageChains: true,
    canViewAudit: false,
    canExecuteEmergency: false,
  },
  dex_admin: {
    role: 'dex_admin',
    canPause: false,
    canSuspend: false,
    canModifyFees: false,
    canManageChains: false,
    canViewAudit: false,
    canExecuteEmergency: false,
  },
  bridge_admin: {
    role: 'bridge_admin',
    canPause: false,
    canSuspend: false,
    canModifyFees: false,
    canManageChains: false,
    canViewAudit: false,
    canExecuteEmergency: false,
  },
  liquidity_admin: {
    role: 'liquidity_admin',
    canPause: false,
    canSuspend: false,
    canModifyFees: false,
    canManageChains: false,
    canViewAudit: false,
    canExecuteEmergency: false,
  },
  fee_admin: {
    role: 'fee_admin',
    canPause: false,
    canSuspend: false,
    canModifyFees: true,
    canManageChains: false,
    canViewAudit: false,
    canExecuteEmergency: false,
  },
  user_admin: {
    role: 'user_admin',
    canPause: false,
    canSuspend: true,
    canModifyFees: false,
    canManageChains: false,
    canViewAudit: false,
    canExecuteEmergency: false,
  },
  analytics_admin: {
    role: 'analytics_admin',
    canPause: false,
    canSuspend: false,
    canModifyFees: false,
    canManageChains: false,
    canViewAudit: false,
    canExecuteEmergency: false,
  },
  audit_admin: {
    role: 'audit_admin',
    canPause: false,
    canSuspend: false,
    canModifyFees: false,
    canManageChains: false,
    canViewAudit: true,
    canExecuteEmergency: false,
  },
  security_admin: {
    role: 'security_admin',
    canPause: false,
    canSuspend: false,
    canModifyFees: false,
    canManageChains: false,
    canViewAudit: true,
    canExecuteEmergency: false,
  },
  emergency_admin: {
    role: 'emergency_admin',
    canPause: true,
    canSuspend: false,
    canModifyFees: false,
    canManageChains: false,
    canViewAudit: false,
    canExecuteEmergency: true,
  },
};

// ============================================================================
// Export
// ============================================================================

export default {
  SuperAdmin,
  ROLE_PERMISSIONS,
};