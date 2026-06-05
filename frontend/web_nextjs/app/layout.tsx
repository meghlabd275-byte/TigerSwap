import type { Metadata } from 'next'
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
    <html lang="en">
      <body>{children}</body>
    </html>
  )
}