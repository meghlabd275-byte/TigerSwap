/**
 * TigerSwap Production Frontend
 * Next.js 14 + TypeScript + Tailwind CSS
 * 
 * @author TigerSwap
 * @version 1.0.0 Production
 */

import { configureChains, createConfig, WagmiConfig } from 'wagmi';
import { mainnet, polygon, arbitrum, optimism, base } from 'wagmi/chains';
import { publicProvider } from 'wagmi/providers/public';
import { WalletConnectConnector } from 'wagmi/connectors/walletConnect';
import { RainbowKitProvider, getDefaultWallets, darkTheme } from '@rainbow-me/rainbowkit';
import '@rainbow-me/rainbowkit/styles.css';
import type { Metadata } from 'next';
import { Inter } from 'next/font/google';
import './globals.css';

const inter = Inter({ subsets: ['latin'] });

// Configure chains
const { chains, publicClient } = configureChains(
  [mainnet, polygon, arbitrum, optimism, base],
  [publicProvider()]
);

// Configure wagmi
const config = createConfig({
  autoConnect: true,
  publicClient,
  chains,
});

// Get wallets
const { connectors } = getDefaultWallets({
  appName: 'TigerSwap',
  projectId: 'tigerswap-dex',
  chains,
});

export const metadata: Metadata = {
  title: 'TigerSwap - Decentralized Exchange',
  description: 'Multi-chain decentralized exchange with best rates and ultra-low latency',
  keywords: ['DEX', 'DeFi', 'Cryptocurrency', 'Swap', 'Trading'],
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="en">
      <body className={inter.className}>
        <WagmiConfig config={config}>
          <RainbowKitProvider 
            chains={chains}
            theme={darkTheme({
              accentColor: '#f97316',
              accentColorForeground: 'white',
              borderRadius: 'medium',
              fontStack: 'system',
              overlayBlur: 'small',
            })}
          >
            {children}
          </RainbowKitProvider>
        </WagmiConfig>
      </body>
    </html>
  );
}
