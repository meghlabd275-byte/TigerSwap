/**
 * TigerSwap DApp Browser - Permissions Manager
 * 
 * Native permissions management for DApp access.
 * Zero external dependencies - fully native implementation.
 * 
 * @author TigerSwap
 * @version 1.0.0
 */

export interface Permission {
  id: string;
  origin: string;
  type: PermissionType;
  granted: boolean;
  expiresAt?: number;
  createdAt: number;
}

export type PermissionType = 
  | 'eth_accounts'
  | 'eth_chainId'
  | 'eth_requestAccounts'
  | 'eth_sendTransaction'
  | 'personal_sign'
  | 'signTypedData';

export interface PermissionRequest {
  origin: string;
  permissions: PermissionType[];
}

export interface PermissionResult {
  granted: PermissionType[];
  rejected: PermissionType[];
}

// Default permissions
const DEFAULT_PERMISSIONS: PermissionType[] = [
  'eth_accounts',
  'eth_chainId',
];

// Sensitive permissions
const SENSITIVE_PERMISSIONS: PermissionType[] = [
  'eth_sendTransaction',
  'personal_sign',
  'signTypedData',
];

export class PermissionsManager {
  private permissions: Map<string, Permission[]>;
  private autoApproveOrigins: Set<string>;

  constructor() {
    this.permissions = new Map();
    this.autoApproveOrigins = new Set();
  }

  /**
   * Request permissions
   */
  async requestPermissions(request: PermissionRequest): Promise<PermissionResult> {
    const granted: PermissionType[] = [];
    const rejected: PermissionType[] = [];

    for (const perm of request.permissions) {
      const existing = this.getPermission(request.origin, perm);
      
      if (existing?.granted) {
        granted.push(perm);
      } else if (this.autoApproveOrigins.has(request.origin)) {
        granted.push(perm);
        this.grantPermission(request.origin, perm);
      } else if (!SENSITIVE_PERMISSIONS.includes(perm)) {
        granted.push(perm);
        this.grantPermission(request.origin, perm);
      } else {
        rejected.push(perm);
      }
    }

    return { granted, rejected };
  }

  /**
   * Grant permission
   */
  grantPermission(origin: string, type: PermissionType, expiresAt?: number): void {
    const perms = this.permissions.get(origin) || [];
    
    // Remove existing
    const filtered = perms.filter(p => p.type !== type);
    
    // Add new
    filtered.push({
      id: this.generateId(),
      origin,
      type,
      granted: true,
      expiresAt,
      createdAt: Date.now(),
    });
    
    this.permissions.set(origin, filtered);
  }

  /**
   * Revoke permission
   */
  revokePermission(origin: string, type: PermissionType): void {
    const perms = this.permissions.get(origin) || [];
    const filtered = perms.filter(p => p.type !== type);
    this.permissions.set(origin, filtered);
  }

  /**
   * Get permission
   */
  getPermission(origin: string, type: PermissionType): Permission | undefined {
    const perms = this.permissions.get(origin) || [];
    return perms.find(p => p.type === type);
  }

  /**
   * Check if permission granted
   */
  isGranted(origin: string, type: PermissionType): boolean {
    const perm = this.getPermission(origin, type);
    if (!perm || !perm.granted) return false;
    
    if (perm.expiresAt && perm.expiresAt < Date.now()) {
      this.revokePermission(origin, type);
      return false;
    }
    
    return true;
  }

  /**
   * Get all permissions for origin
   */
  getPermissions(origin: string): Permission[] {
    return this.permissions.get(origin) || [];
  }

  /**
   * Revoke all permissions for origin
   */
  revokeAll(origin: string): void {
    this.permissions.delete(origin);
  }

  /**
   * Add to auto-approve list
   */
  addToAutoApprove(origin: string): void {
    this.autoApproveOrigins.add(origin);
  }

  /**
   * Remove from auto-approve list
   */
  removeFromAutoApprove(origin: string): void {
    this.autoApproveOrigins.delete(origin);
  }

  /**
   * Check if origin is auto-approved
   */
  isAutoApproved(origin: string): boolean {
    return this.autoApproveOrigins.has(origin);
  }

  /**
   * Get expired permissions
   */
  getExpiredPermissions(): { origin: string; type: PermissionType }[] {
    const expired: { origin: string; type: PermissionType }[] = [];
    const now = Date.now();

    for (const [origin, perms] of this.permissions.entries()) {
      for (const perm of perms) {
        if (perm.expiresAt && perm.expiresAt < now) {
          expired.push({ origin, type: perm.type });
        }
      }
    }

    return expired;
  }

  /**
   * Clean expired permissions
   */
  cleanExpired(): void {
    const expired = this.getExpiredPermissions();
    for (const { origin, type } of expired) {
      this.revokePermission(origin, type);
    }
  }

  /**
   * Get permission requests
   */
  getPermissionRequests(origin: string): PermissionType[] {
    const perms = this.permissions.get(origin) || [];
    return perms.filter(p => !p.granted).map(p => p.type);
  }

  private generateId(): string {
    return 'perm_' + Math.random().toString(36).substr(2, 9);
  }
}

export default PermissionsManager;