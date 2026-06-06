import type { Metadata } from 'next'
import { ThemeProvider } from './components/ThemeProvider'
import './globals.css'

export const metadata: Metadata = {
  title: 'TigerSwap - Multichain DEX',
  description: 'Enterprise-grade multichain decentralized exchange with cross-chain swaps',
}

export default function RootLayout({
  children,
}: {
  children: React.ReactNode
}) {
  return (
    <html lang="en" suppressHydrationWarning>
      <head>
        <script
          dangerouslySetInnerHTML={{
            __html: `
              (function() {
                var theme = localStorage.getItem('tigerswap-theme');
                if (!theme) {
                  theme = window.matchMedia('(prefers-color-scheme: light)').matches ? 'light' : 'dark';
                }
                document.documentElement.classList.add(theme);
              })();
            `,
          }}
        />
      </head>
      <body>
        <ThemeProvider>{children}</ThemeProvider>
      </body>
    </html>
  )
}