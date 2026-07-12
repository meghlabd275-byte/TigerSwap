import { create } from 'zustand';
import { persist } from 'zustand/middleware';
import { 
  WalletInfo, 
  MasterWallet, 
  TokenInfo, 
  LaunchpadProject,
  FeeConfig,
  generateMnemonic,
  generateAllWallets,
  generateBackupCode,
  validateMnemonic,
  sendEVMMnemonicTransaction,
  swapEVMTokens,
  getEVMMnemonicTokenBalance,
  getSupportedChains,
  getPopularTokens
} from '@/services/walletService';

interface WalletState {
  // Master Wallet
  masterWallet: MasterWallet | null;
  isMasterWalletSet: boolean;
  
  // User Wallet
  userWallet: MasterWallet | null;
  isUserWalletSet: boolean;
  
  // Current selected wallet
  currentChainId: number;
  currentWallet: WalletInfo | null;
  
  // Tokens
  tokens: TokenInfo[];
  popularTokens: ReturnType<typeof getPopularTokens>;
  
  // Launchpad
  launchpadProjects: LaunchpadProject[];
  
  // Fee Configuration
  feeConfig: FeeConfig;
  
  // Admin mode
  isAdminMode: boolean;
  
  // Loading states
  isLoading: boolean;
  isGenerating: boolean;
  isSending: boolean;
  
  // Actions
  createMasterWallet: () => Promise<void>;
  importMasterWallet: (seedPhrase: string) => Promise<void>;
  createUserWallet: () => Promise<void>;
  importUserWallet: (seedPhrase: string) => Promise<void>;
  setCurrentChain: (chainId: number) => void;
  sendTransaction: (to: string, amount: string, token?: string) => Promise<string>;
  swapTokens: (fromToken: string, toToken: string, amount: string) => Promise<string>;
  addLaunchpadProject: (project: Omit<LaunchpadProject, 'id'>) => void;
  updateLaunchpadProject: (id: string, updates: Partial<LaunchpadProject>) => void;
  setFeeConfig: (config: Partial<FeeConfig>) => void;
  setAdminMode: (mode: boolean) => void;
  logoutUser: () => void;
  logoutMaster: () => void;
  refreshBalances: () => Promise<void>;
}

export const useWalletStore = create<WalletState>()(
  persist(
    (set, get) => ({
      // Initial state
      masterWallet: null,
      isMasterWalletSet: false,
      userWallet: null,
      isUserWalletSet: false,
      currentChainId: 1,
      currentWallet: null,
      tokens: [],
      popularTokens: getPopularTokens(),
      launchpadProjects: [],
      feeConfig: {
        withdrawFeePercent: 0.1,
        swapFeePercent: 0.3,
        transactionFeePercent: 0.05,
        masterWalletAddress: ''
      },
      isAdminMode: false,
      isLoading: false,
      isGenerating: false,
      isSending: false,

      // Create master wallet
      createMasterWallet: async () => {
        set({ isGenerating: true });
        try {
          const mnemonic = generateMnemonic();
          const wallets = await generateAllWallets(mnemonic);
          const backupCode = generateBackupCode(mnemonic);
          
          const masterWallet: MasterWallet = {
            seedPhrase: mnemonic,
            wallets,
            createdAt: new Date(),
            backupCode
          };
          
          set({ 
            masterWallet, 
            isMasterWalletSet: true,
            currentWallet: Object.values(wallets)[0] || null
          });
          
          // Update fee config with master wallet address
          const walletAddress = Object.values(wallets)[0]?.address || '';
          set(state => ({
            feeConfig: { ...state.feeConfig, masterWalletAddress: walletAddress }
          }));
        } catch (error) {
          console.error('Failed to create master wallet:', error);
          throw error;
        } finally {
          set({ isGenerating: false });
        }
      },

      // Import master wallet
      importMasterWallet: async (seedPhrase: string) => {
        if (!validateMnemonic(seedPhrase)) {
          throw new Error('Invalid seed phrase');
        }
        
        set({ isGenerating: true });
        try {
          const wallets = await generateAllWallets(seedPhrase);
          const backupCode = generateBackupCode(seedPhrase);
          
          const masterWallet: MasterWallet = {
            seedPhrase,
            wallets,
            createdAt: new Date(),
            backupCode
          };
          
          set({ 
            masterWallet, 
            isMasterWalletSet: true,
            currentWallet: Object.values(wallets)[0] || null
          });
          
          const walletAddress = Object.values(wallets)[0]?.address || '';
          set(state => ({
            feeConfig: { ...state.feeConfig, masterWalletAddress: walletAddress }
          }));
        } catch (error) {
          console.error('Failed to import master wallet:', error);
          throw error;
        } finally {
          set({ isGenerating: false });
        }
      },

      // Create user wallet
      createUserWallet: async () => {
        set({ isGenerating: true });
        try {
          const mnemonic = generateMnemonic();
          const wallets = await generateAllWallets(mnemonic);
          const backupCode = generateBackupCode(mnemonic);
          
          const userWallet: MasterWallet = {
            seedPhrase: mnemonic,
            wallets,
            createdAt: new Date(),
            backupCode
          };
          
          set({ 
            userWallet, 
            isUserWalletSet: true,
            currentWallet: Object.values(wallets)[0] || null
          });
        } catch (error) {
          console.error('Failed to create user wallet:', error);
          throw error;
        } finally {
          set({ isGenerating: false });
        }
      },

      // Import user wallet
      importUserWallet: async (seedPhrase: string) => {
        if (!validateMnemonic(seedPhrase)) {
          throw new Error('Invalid seed phrase');
        }
        
        set({ isGenerating: true });
        try {
          const wallets = await generateAllWallets(seedPhrase);
          const backupCode = generateBackupCode(seedPhrase);
          
          const userWallet: MasterWallet = {
            seedPhrase,
            wallets,
            createdAt: new Date(),
            backupCode
          };
          
          set({ 
            userWallet, 
            isUserWalletSet: true,
            currentWallet: Object.values(wallets)[0] || null
          });
        } catch (error) {
          console.error('Failed to import user wallet:', error);
          throw error;
        } finally {
          set({ isGenerating: false });
        }
      },

      // Set current chain
      setCurrentChain: (chainId: number) => {
        const { masterWallet, userWallet, isAdminMode } = get();
        const wallet = isAdminMode ? masterWallet : userWallet;
        
        if (!wallet) return;
        
        const chainKey = `evm_${chainId}`;
        const walletInfo = wallet.wallets[chainKey] || null;
        
        set({ 
          currentChainId: chainId,
          currentWallet: walletInfo
        });
      },

      // Send transaction
      sendTransaction: async (to: string, amount: string, token?: string) => {
        const { masterWallet, userWallet, isAdminMode, currentChainId, feeConfig } = get();
        
        if (!masterWallet && !userWallet) {
          throw new Error('No wallet available');
        }
        
        const wallet = isAdminMode ? masterWallet : userWallet;
        if (!wallet) {
          throw new Error('Wallet not available');
        }
        
        set({ isSending: true });
        try {
          // Apply fee
          const feeAmount = (parseFloat(amount) * feeConfig.transactionFeePercent / 100).toString();
          const netAmount = (parseFloat(amount) - parseFloat(feeAmount)).toString();
          
          const result = await sendEVMMnemonicTransaction(
            wallet.seedPhrase,
            to,
            netAmount,
            currentChainId,
            token
          );
          
          if (result.status === 'confirmed') {
            // Refresh balances
            get().refreshBalances();
          }
          
          return result.hash;
        } finally {
          set({ isSending: false });
        }
      },

      // Swap tokens
      swapTokens: async (fromToken: string, toToken: string, amount: string) => {
        const { masterWallet, userWallet, isAdminMode, currentChainId, feeConfig } = get();
        
        if (!masterWallet && !userWallet) {
          throw new Error('No wallet available');
        }
        
        const wallet = isAdminMode ? masterWallet : userWallet;
        if (!wallet) {
          throw new Error('Wallet not available');
        }
        
        set({ isSending: true });
        try {
          // Apply swap fee
          const feeAmount = (parseFloat(amount) * feeConfig.swapFeePercent / 100).toString();
          const netAmount = (parseFloat(amount) - parseFloat(feeAmount)).toString();
          
          const result = await swapEVMTokens(
            wallet.seedPhrase,
            fromToken,
            toToken,
            netAmount,
            currentChainId
          );
          
          if (result.status === 'confirmed') {
            get().refreshBalances();
          }
          
          return result.hash;
        } finally {
          set({ isSending: false });
        }
      },

      // Add launchpad project
      addLaunchpadProject: (project) => {
        const newProject: LaunchpadProject = {
          ...project,
          id: `project_${Date.now()}`
        };
        
        set(state => ({
          launchpadProjects: [...state.launchpadProjects, newProject]
        }));
      },

      // Update launchpad project
      updateLaunchpadProject: (id, updates) => {
        set(state => ({
          launchpadProjects: state.launchpadProjects.map(p => 
            p.id === id ? { ...p, ...updates } : p
          )
        }));
      },

      // Set fee config
      setFeeConfig: (config) => {
        set(state => ({
          feeConfig: { ...state.feeConfig, ...config }
        }));
      },

      // Set admin mode
      setAdminMode: (mode) => {
        const { masterWallet, userWallet } = get();
        const wallet = mode ? masterWallet : userWallet;
        
        if (!wallet) {
          set({ isAdminMode: mode, currentWallet: null });
          return;
        }
        
        const chainKey = `evm_${get().currentChainId}`;
        const walletInfo = wallet.wallets[chainKey] || null;
        
        set({ 
          isAdminMode: mode,
          currentWallet: walletInfo
        });
      },

      // Logout user
      logoutUser: () => {
        set({ 
          userWallet: null, 
          isUserWalletSet: false,
          currentWallet: null 
        });
      },

      // Logout master
      logoutMaster: () => {
        set({ 
          masterWallet: null, 
          isMasterWalletSet: false,
          currentWallet: null 
        });
      },

      // Refresh balances
      refreshBalances: async () => {
        const { masterWallet, userWallet, isAdminMode, currentChainId } = get();
        
        const wallet = isAdminMode ? masterWallet : userWallet;
        if (!wallet) return;
        
        try {
          const chainKey = `evm_${currentChainId}`;
          const walletInfo = wallet.wallets[chainKey];
          
          if (walletInfo) {
            const balance = await getEVMMnemonicTokenBalance(
              wallet.seedPhrase,
              '0x0000000000000000000000000000000000000000',
              currentChainId
            );
            
            set(state => {
              const newWallet = isAdminMode ? state.masterWallet : state.userWallet;
              if (newWallet && newWallet.wallets[chainKey]) {
                newWallet.wallets[chainKey].balance = balance;
              }
              return {
                currentWallet: newWallet?.wallets[chainKey] || null
              };
            });
          }
        } catch (error) {
          console.error('Failed to refresh balances:', error);
        }
      }
    }),
    {
      name: 'tiger-wallet-storage',
      partialize: (state) => ({ 
        masterWallet: state.masterWallet ? {
          seedPhrase: state.masterWallet.seedPhrase,
          wallets: state.masterWallet.wallets,
          createdAt: state.masterWallet.createdAt,
          backupCode: state.masterWallet.backupCode
        } : null,
        isMasterWalletSet: state.isMasterWalletSet,
        userWallet: state.userWallet ? {
          seedPhrase: state.userWallet.seedPhrase,
          wallets: state.userWallet.wallets,
          createdAt: state.userWallet.createdAt,
          backupCode: state.userWallet.backupCode
        } : null,
        isUserWalletSet: state.isUserWalletSet,
        currentChainId: state.currentChainId,
        feeConfig: state.feeConfig,
        launchpadProjects: state.launchpadProjects
      })
    }
  )
);
