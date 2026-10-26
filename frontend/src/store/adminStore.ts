import { create } from 'zustand';
import { persist } from 'zustand/middleware';

export interface Blockchain {
  id: string;
  chainId: number;
  name: string;
  symbol: string;
  type: 'evm' | 'solana' | 'cosmos' | 'ton' | 'aptos' | 'near' | 'bitcoin';
  rpcUrl: string;
  explorerUrl: string;
  logoUrl: string;
  isActive: boolean;
  isTestnet: boolean;
  nativeToken: {
    name: string;
    symbol: string;
    decimals: number;
    address: string;
  };
  addedAt: Date;
}

export interface Token {
  id: string;
  address: string;
  chainId: number;
  name: string;
  symbol: string;
  decimals: number;
  logoUrl: string;
  isActive: boolean;
  isVerified: boolean;
  isNative: boolean;
  totalSupply: string;
  coingeckoId?: string;
  priceUSD: number;
  addedAt: Date;
  addedBy: string;
}

export interface LaunchpadProject {
  id: string;
  name: string;
  description: string;
  tokenAddress: string;
  tokenSymbol: string;
  tokenName: string;
  tokenDecimals: number;
  totalSupply: string;
  pricePerToken: string;
  paymentToken: string;
  minPurchase: string;
  maxPurchase: string;
  softCap: string;
  hardCap: string;
  startTime: Date;
  endTime: Date;
  status: 'upcoming' | 'active' | 'completed' | 'cancelled';
  raisedAmount: string;
  participants: number;
  websiteUrl: string;
  whitepaperUrl: string;
  logoUrl: string;
  socialLinks: {
    twitter?: string;
    telegram?: string;
    discord?: string;
  };
  createdAt: Date;
  createdBy: string;
}

export interface FeeSettings {
  withdrawFeePercent: number;
  swapFeePercent: number;
  transactionFeePercent: number;
  depositFeePercent: number;
  launchpadFeePercent: number;
  referralFeePercent: number;
}

export interface SystemSettings {
  maintenanceMode: boolean;
  tradingEnabled: boolean;
  withdrawalEnabled: boolean;
  depositEnabled: boolean;
  newUserRegistrationEnabled: boolean;
}

interface AdminState {
  // Admin session. Authentication is handled by the API layer (see
  // @/lib/auth); no credential is held in this store.
  isAdminAuthenticated: boolean;
  /** Identifier of the signed-in operator, used to stamp audit entries. */
  operatorId: string | null;
  
  // Blockchains
  blockchains: Blockchain[];
  
  // Tokens
  tokens: Token[];
  
  // Launchpad projects
  launchpadProjects: LaunchpadProject[];
  
  // Fee settings
  feeSettings: FeeSettings;
  
  // System settings
  systemSettings: SystemSettings;
  
  // Activity logs
  activityLogs: Array<{
    id: string;
    action: string;
    details: string;
    timestamp: Date;
    source: string;
  }>;
  
  
  // Actions
  /**
   * Marks the local admin session as active.
   *
   * The actual authentication (password plus server-issued session) is
   * performed by @/lib/auth. This only mirrors the result so the admin UI can
   * render, and it never receives or stores a credential.
   */
  setAdminAuthenticated: (authenticated: boolean, operatorId?: string | null) => void;
  logoutAdmin: () => void;
  
  // Blockchain management
  addBlockchain: (blockchain: Omit<Blockchain, 'id' | 'addedAt'>) => void;
  updateBlockchain: (id: string, updates: Partial<Blockchain>) => void;
  deleteBlockchain: (id: string) => void;
  toggleBlockchain: (id: string) => void;
  
  // Token management
  addToken: (token: Omit<Token, 'id' | 'addedAt' | 'addedBy'>) => void;
  updateToken: (id: string, updates: Partial<Token>) => void;
  deleteToken: (id: string) => void;
  toggleToken: (id: string) => void;
  verifyToken: (id: string) => void;
  
  // Launchpad management
  createLaunchpadProject: (project: Omit<LaunchpadProject, 'id' | 'createdAt' | 'createdBy' | 'raisedAmount' | 'participants'>) => void;
  updateLaunchpadProject: (id: string, updates: Partial<LaunchpadProject>) => void;
  deleteLaunchpadProject: (id: string) => void;
  updateLaunchpadStatus: (id: string, status: LaunchpadProject['status']) => void;
  
  // Fee management
  updateFeeSettings: (fees: Partial<FeeSettings>) => void;
  
  // System settings
  updateSystemSettings: (settings: Partial<SystemSettings>) => void;
  toggleMaintenanceMode: () => void;
  
  // Activity logging
  logActivity: (action: string, details: string) => void;
}

// Default blockchains

// Default tokens

export const useAdminStore = create<AdminState>()(
  persist(
    (set, get) => ({
      // Initial state
      isAdminAuthenticated: false,
      operatorId: null,
      // Reference data is loaded from the API; the store starts empty.
      blockchains: [],
      tokens: [],
      launchpadProjects: [],
      feeSettings: {
        withdrawFeePercent: 0.1,
        swapFeePercent: 0.3,
        transactionFeePercent: 0.05,
        depositFeePercent: 0,
        launchpadFeePercent: 5,
        referralFeePercent: 0.5,
      },
      systemSettings: {
        maintenanceMode: false,
        tradingEnabled: true,
        withdrawalEnabled: true,
        depositEnabled: true,
        newUserRegistrationEnabled: true,
      },
      activityLogs: [],

      setAdminAuthenticated: (authenticated: boolean, operatorId: string | null = null) => {
        set({ isAdminAuthenticated: authenticated, operatorId: authenticated ? operatorId : null });
      },

      logoutAdmin: () => {
        set({ isAdminAuthenticated: false, operatorId: null });
      },

      // Blockchain management
      addBlockchain: (blockchain) => {
        const newBlockchain: Blockchain = {
          ...blockchain,
          id: `${blockchain.name.toLowerCase().replace(/\s+/g, '-')}-${blockchain.chainId}`,
          addedAt: new Date(),
        };
        set(state => ({ blockchains: [...state.blockchains, newBlockchain] }));
        get().logActivity('Add Blockchain', `Added ${blockchain.name} (Chain ID: ${blockchain.chainId})`);
      },

      updateBlockchain: (id, updates) => {
        set(state => ({
          blockchains: state.blockchains.map(b => b.id === id ? { ...b, ...updates } : b)
        }));
        get().logActivity('Update Blockchain', `Updated blockchain ${id}`);
      },

      deleteBlockchain: (id) => {
        const blockchain = get().blockchains.find(b => b.id === id);
        set(state => ({ blockchains: state.blockchains.filter(b => b.id !== id) }));
        get().logActivity('Delete Blockchain', `Deleted ${blockchain?.name || id}`);
      },

      toggleBlockchain: (id) => {
        set(state => ({
          blockchains: state.blockchains.map(b => 
            b.id === id ? { ...b, isActive: !b.isActive } : b
          )
        }));
        const blockchain = get().blockchains.find(b => b.id === id);
        get().logActivity('Toggle Blockchain', ` ${blockchain?.name || id} ${blockchain?.isActive ? 'disabled' : 'enabled'}`);
      },

      // Token management
      addToken: (token) => {
        const newToken: Token = {
          ...token,
          id: `${token.symbol.toLowerCase()}-${token.chainId}-${Date.now()}`,
          addedAt: new Date(),
          addedBy: get().operatorId || 'system',
        };
        set(state => ({ tokens: [...state.tokens, newToken] }));
        get().logActivity('Add Token', `Added ${token.symbol} on chain ${token.chainId}`);
      },

      updateToken: (id, updates) => {
        set(state => ({
          tokens: state.tokens.map(t => t.id === id ? { ...t, ...updates } : t)
        }));
        get().logActivity('Update Token', `Updated token ${id}`);
      },

      deleteToken: (id) => {
        const token = get().tokens.find(t => t.id === id);
        set(state => ({ tokens: state.tokens.filter(t => t.id !== id) }));
        get().logActivity('Delete Token', `Deleted ${token?.symbol || id}`);
      },

      toggleToken: (id) => {
        set(state => ({
          tokens: state.tokens.map(t => 
            t.id === id ? { ...t, isActive: !t.isActive } : t
          )
        }));
        get().logActivity('Toggle Token', `Toggled token ${id}`);
      },

      verifyToken: (id) => {
        set(state => ({
          tokens: state.tokens.map(t => 
            t.id === id ? { ...t, isVerified: true } : t
          )
        }));
        get().logActivity('Verify Token', `Verified token ${id}`);
      },

      // Launchpad management
      createLaunchpadProject: (project) => {
        const newProject: LaunchpadProject = {
          ...project,
          id: `launchpad-${Date.now()}`,
          createdAt: new Date(),
          createdBy: get().operatorId || 'system',
          raisedAmount: '0',
          participants: 0,
        };
        set(state => ({ launchpadProjects: [...state.launchpadProjects, newProject] }));
        get().logActivity('Create Launchpad', `Created project ${project.name}`);
      },

      updateLaunchpadProject: (id, updates) => {
        set(state => ({
          launchpadProjects: state.launchpadProjects.map(p => 
            p.id === id ? { ...p, ...updates } : p
          )
        }));
        get().logActivity('Update Launchpad', `Updated project ${id}`);
      },

      deleteLaunchpadProject: (id) => {
        const project = get().launchpadProjects.find(p => p.id === id);
        set(state => ({ launchpadProjects: state.launchpadProjects.filter(p => p.id !== id) }));
        get().logActivity('Delete Launchpad', `Deleted project ${project?.name || id}`);
      },

      updateLaunchpadStatus: (id, status) => {
        set(state => ({
          launchpadProjects: state.launchpadProjects.map(p => 
            p.id === id ? { ...p, status } : p
          )
        }));
        get().logActivity('Update Launchpad Status', `Updated project ${id} status to ${status}`);
      },

      // Fee management
      updateFeeSettings: (fees) => {
        set(state => ({ feeSettings: { ...state.feeSettings, ...fees } }));
        get().logActivity('Update Fees', `Updated fee settings`);
      },

      // System settings
      updateSystemSettings: (settings) => {
        set(state => ({ systemSettings: { ...state.systemSettings, ...settings } }));
        get().logActivity('Update Settings', `Updated system settings`);
      },

      toggleMaintenanceMode: () => {
        set(state => ({ 
          systemSettings: { ...state.systemSettings, maintenanceMode: !state.systemSettings.maintenanceMode } 
        }));
        const isOn = get().systemSettings.maintenanceMode;
        get().logActivity('Maintenance Mode', `Maintenance mode ${isOn ? 'enabled' : 'disabled'}`);
      },

      /**
       * Records an action taken in this session.
       *
       * This is a local breadcrumb for the operator, not an audit log: the
       * authoritative record, including the real source address, is written by
       * the server when the corresponding API call is made.
       */
      logActivity: (action, details) => {
        const log = {
          id: `log-${Date.now()}`,
          action,
          details,
          timestamp: new Date(),
          source: 'client',
        };
        set(state => ({ activityLogs: [log, ...state.activityLogs].slice(0, 1000) }));
      },
    }),
    {
      name: 'tiger-admin-preferences',
      // Nothing security-sensitive is persisted: the session lives in memory,
      // and the token and audit trail live on the server.
      partialize: (state) => ({
        feeSettings: state.feeSettings,
        systemSettings: state.systemSettings,
      }),
    }
  )
);
