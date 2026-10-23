import { create } from 'zustand';
import { persist } from 'zustand/middleware';

export interface Token {
  address: string;
  symbol: string;
  name: string;
  decimals: number;
  logoURI: string;
  chainId: number;
  price?: number;
  balance?: string;
}



interface StoreState {
  connected: boolean;
  address: string | null;
  selectedChain: string;
  tokens: Token[];
  selectedTokenIn: Token | null;
  selectedTokenOut: Token | null;
  connectWallet: () => Promise<void>;
  disconnectWallet: () => void;
  setSelectedChain: (chain: string) => void;
  setTokens: (tokens: Token[]) => void;
  setSelectedTokenIn: (token: Token | null) => void;
  setSelectedTokenOut: (token: Token | null) => void;
}

export const useStore = create<StoreState>()(
  persist(
    (set) => ({
      connected: false,
      address: null,
      selectedChain: 'ethereum',
      tokens: [],
      selectedTokenIn: null,
      selectedTokenOut: null,
      connectWallet: async () => {
        // Delegates to the user's own wallet. This store never sees a key.
        const { connectWallet } = await import('@/lib/wallet');
        const account = await connectWallet();
        set({ connected: true, address: account.address });
      },
      disconnectWallet: () => {
        set({ connected: false, address: null });
      },
      setSelectedChain: (chain) => set({ selectedChain: chain }),
      setTokens: (tokens) => set({ tokens }),
      setSelectedTokenIn: (token) => set({ selectedTokenIn: token }),
      setSelectedTokenOut: (token) => set({ selectedTokenOut: token }),
    }),
    {
      name: 'tigerswap-storage',
      partialize: (state) => ({ 
        selectedChain: state.selectedChain,
        connected: state.connected,
        address: state.address 
      }),
    }
  )
);
