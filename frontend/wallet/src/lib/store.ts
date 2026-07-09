import { create } from 'zustand';
import { persist } from 'zustand/middleware';
import { Address, Hash } from 'viem';
import type { 
  Wallet, 
  TokenBalance, 
  Notification, 
  Transaction,
  GasPrice 
} from '@/types/wallet';

interface WalletState {
  // Wallet
  currentWallet: Wallet | null;
  wallets: Wallet[];
  isConnected: boolean;
  
  // Balances
  tokenBalances: TokenBalance[];
  nativeBalance: bigint;
  totalUSDValue: number;
  
  // Network
  chainId: number;
  chainName: string;
  
  // Gas
  gasPrice: GasPrice | null;
  
  // Transactions
  transactions: Transaction[];
  pendingTransactions: Transaction[];
  
  // Notifications
  notifications: Notification[];
  unreadCount: number;
  
  // Settings
  theme: 'light' | 'dark' | 'system';
  currency: string;
  language: string;
  
  // Actions
  setCurrentWallet: (wallet: Wallet | null) => void;
  addWallet: (wallet: Wallet) => void;
  removeWallet: (address: Address) => void;
  setTokenBalances: (balances: TokenBalance[]) => void;
  setNativeBalance: (balance: bigint) => void;
  setTotalUSDValue: (value: number) => void;
  setChain: (chainId: number, chainName: string) => void;
  setGasPrice: (gasPrice: GasPrice) => void;
  addTransaction: (tx: Transaction) => void;
  updateTransaction: (hash: Hash, updates: Partial<Transaction>) => void;
  addNotification: (notification: Notification) => void;
  markNotificationRead: (id: string) => void;
  clearNotifications: () => void;
  setTheme: (theme: 'light' | 'dark' | 'system') => void;
  setCurrency: (currency: string) => void;
  setLanguage: (language: string) => void;
  reset: () => void;
}

const initialState = {
  currentWallet: null,
  wallets: [],
  isConnected: false,
  tokenBalances: [],
  nativeBalance: BigInt(0),
  totalUSDValue: 0,
  chainId: 1,
  chainName: 'Ethereum',
  gasPrice: null,
  transactions: [],
  pendingTransactions: [],
  notifications: [],
  unreadCount: 0,
  theme: 'dark' as const,
  currency: 'USD',
  language: 'en',
};

export const useWalletStore = create<WalletState>()(
  persist(
    (set, get) => ({
      ...initialState,
      
      setCurrentWallet: (wallet) => set({ 
        currentWallet: wallet,
        isConnected: wallet !== null,
      }),
      
      addWallet: (wallet) => set((state) => ({
        wallets: [...state.wallets, wallet],
      })),
      
      removeWallet: (address) => set((state) => ({
        wallets: state.wallets.filter(w => w.address !== address),
        currentWallet: state.currentWallet?.address === address 
          ? null 
          : state.currentWallet,
      })),
      
      setTokenBalances: (balances) => set({ tokenBalances: balances }),
      
      setNativeBalance: (balance) => set({ nativeBalance: balance }),
      
      setTotalUSDValue: (value) => set({ totalUSDValue: value }),
      
      setChain: (chainId, chainName) => set({ chainId, chainName }),
      
      setGasPrice: (gasPrice) => set({ gasPrice }),
      
      addTransaction: (tx) => set((state) => ({
        transactions: [tx, ...state.transactions],
        pendingTransactions: tx.status === 'pending' 
          ? [...state.pendingTransactions, tx]
          : state.pendingTransactions,
      })),
      
      updateTransaction: (hash, updates) => set((state) => ({
        transactions: state.transactions.map(tx => 
          tx.hash === hash ? { ...tx, ...updates } : tx
        ),
        pendingTransactions: updates.status === 'confirmed' || updates.status === 'failed'
          ? state.pendingTransactions.filter(tx => tx.hash !== hash)
          : state.pendingTransactions.map(tx => 
              tx.hash === hash ? { ...tx, ...updates } : tx
            ),
      })),
      
      addNotification: (notification) => set((state) => ({
        notifications: [notification, ...state.notifications],
        unreadCount: state.unreadCount + 1,
      })),
      
      markNotificationRead: (id) => set((state) => ({
        notifications: state.notifications.map(n => 
          n.id === id ? { ...n, read: true } : n
        ),
        unreadCount: Math.max(0, state.unreadCount - 1),
      })),
      
      clearNotifications: () => set({ 
        notifications: [], 
        unreadCount: 0,
      }),
      
      setTheme: (theme) => set({ theme }),
      
      setCurrency: (currency) => set({ currency }),
      
      setLanguage: (language) => set({ language }),
      
      reset: () => set(initialState),
    }),
    {
      name: 'tigerwallet-storage',
      partialize: (state) => ({
        wallets: state.wallets,
        theme: state.theme,
        currency: state.currency,
        language: state.language,
      }),
    }
  )
);
