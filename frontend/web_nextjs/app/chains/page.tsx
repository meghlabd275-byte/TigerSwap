// TigerSwap - Supported Chains Page
// Display all EVM and Non-EVM blockchain networks

import React, { useState, useEffect } from 'react'
import { useTheme } from '../components/ThemeProvider'
import { ThemeToggle } from '../components/ThemeToggle'

interface Chain {
  id: string
  chainId: number
  name: string
  type: string
  symbol: string
  isEnabled: boolean
  description?: string
}

export default function ChainsPage() {
  const { theme } = useTheme()
  const [chains, setChains] = useState<Chain[]>([])
  const [selectedChain, setSelectedChain] = useState<Chain | null>(null)
  
  const isDark = theme === 'dark'

  useEffect(() => {
    setChains([
      { id: '1', chainId: 1, name: 'Ethereum', type: 'evm', symbol: 'ETH', isEnabled: true, description: 'The leading smart contract platform' },
      { id: '56', chainId: 56, name: 'BNB Chain', type: 'evm', symbol: 'BNB', isEnabled: true, description: 'Fast and low-cost transactions' },
      { id: '137', chainId: 137, name: 'Polygon', type: 'evm', symbol: 'MATIC', isEnabled: true, description: 'Ethereum scaling solution' },
      { id: '42161', chainId: 42161, name: 'Arbitrum', type: 'evm', symbol: 'ETH', isEnabled: true, description: 'Layer 2 scaling for Ethereum' },
      { id: '10', chainId: 10, name: 'Optimism', type: 'evm', symbol: 'ETH', isEnabled: true, description: 'Fast, cheap Ethereum transactions' },
      { id: '43114', chainId: 43114, name: 'Avalanche', type: 'evm', symbol: 'AVAX', isEnabled: true, description: 'High performance blockchain platform' },
      { id: '8453', chainId: 8453, name: 'Base', type: 'evm', symbol: 'ETH', isEnabled: true, description: "Coinbase's Layer 2 network" },
      { id: '250', chainId: 250, name: 'Fantom', type: 'evm', symbol: 'FTM', isEnabled: true, description: 'Fast and scalable blockchain' },
      { id: 'solana', chainId: -1, name: 'Solana', type: 'solana', symbol: 'SOL', isEnabled: true, description: 'High speed blockchain' },
      { id: 'tron', chainId: -2, name: 'Tron', type: 'tron', symbol: 'TRX', isEnabled: true, description: 'Decentralized entertainment platform' },
      { id: 'sui', chainId: -3, name: 'Sui', type: 'sui', symbol: 'SUI', isEnabled: true, description: 'Next-gen blockchain by Mysten Labs' },
      { id: 'aptos', chainId: -4, name: 'Aptos', type: 'aptos', symbol: 'APT', isEnabled: true, description: 'Safe and scalable Layer 1' },
      { id: 'near', chainId: -5, name: 'NEAR', type: 'near', symbol: 'NEAR', isEnabled: false, description: 'User-friendly blockchain' },
      { id: 'cosmos', chainId: -6, name: 'Cosmos', type: 'cosmos', symbol: 'ATOM', isEnabled: false, description: 'Internet of Blockchains' },
    ])
  }, [])

  const chainColors: Record<string, string> = {
    ethereum: '#627EEA', bsc: '#F3BA2F', polygon: '#8247E5', arbitrum: '#28A0F0',
    optimism: '#FF0420', avalanche: '#E84142', base: '#0052FF', fantom: '#13B5EC',
    solana: '#9945FF', tron: '#EF0027', sui: '#6F BCEF', aptos: '#3D2847',
  }

  // Theme-aware styles
  const bgPrimary = isDark ? '#0f172a' : '#f8fafc'
  const bgSecondary = isDark ? 'rgba(30,41,59,0.8)' : '#e2e8f0'
  const textPrimary = isDark ? '#f8fafc' : '#0f172a'
  const textSecondary = isDark ? '#94a3b8' : '#64748b'
  const borderColor = isDark ? 'rgba(255,255,255,0.1)' : 'rgba(0,0,0,0.1)'

  return (
    <div style={{ background: bgPrimary, minHeight: '100vh', color: textPrimary }}>
      <div style={{ padding: 24, borderBottom: `1px solid ${borderColor}` }}>
        <div style={{ maxWidth: 1200, margin: '0 auto' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
            <h1 style={{ fontSize: 28, margin: 0, display: 'flex', alignItems: 'center', gap: 12 }}>
              <span>🔗</span> Supported Chains
            </h1>
            <ThemeToggle />
          </div>
          <p style={{ color: textSecondary, margin: '8px 0 0' }}>Connect to any blockchain network</p>
        </div>
      </div>

      <div style={{ maxWidth: 1200, margin: '0 auto', padding: 24 }}>
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: 16, marginBottom: 32 }}>
          <div style={{ background: bgSecondary, padding: 20, borderRadius: 12 }}>
            <p style={{ color: textSecondary, margin: 0 }}>Total Chains</p>
            <h2 style={{ margin: '8px 0', fontSize: 32 }}>{chains.length}</h2>
          </div>
          <div style={{ background: bgSecondary, padding: 20, borderRadius: 12 }}>
            <p style={{ color: textSecondary, margin: 0 }}>Active</p>
            <h2 style={{ margin: '8px 0', fontSize: 32, color: '#10b981' }}>{chains.filter(c => c.isEnabled).length}</h2>
          </div>
          <div style={{ background: bgSecondary, padding: 20, borderRadius: 12 }}>
            <p style={{ color: textSecondary, margin: 0 }}>EVM / Non-EVM</p>
            <h2 style={{ margin: '8px 0', fontSize: 32 }}>{chains.filter(c => c.type === 'evm').length} / {chains.filter(c => c.type !== 'evm').length}</h2>
          </div>
        </div>

        <div style={{ marginBottom: 40 }}>
          <h2 style={{ display: 'flex', alignItems: 'center', gap: 12, marginBottom: 16 }}>
            <span>⚛️</span> EVM Chains <span style={{ background: '#f97316', padding: '2px 12px', borderRadius: 12, fontSize: 12 }}>{chains.filter(c => c.type === 'evm' && c.isEnabled).length}</span>
          </h2>
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(280px, 1fr))', gap: 16 }}>
            {chains.filter(c => c.type === 'evm').map(chain => (
              <div key={chain.id} onClick={() => setSelectedChain(chain)} style={{ 
                background: bgSecondary, borderRadius: 12, padding: 20, cursor: 'pointer',
                border: `1px solid ${chainColors[chain.name.toLowerCase()] || '#f97316'}`, opacity: chain.isEnabled ? 1 : 0.5
              }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: 12, marginBottom: 12 }}>
                  <div style={{ width: 48, height: 48, borderRadius: '50%', background: chainColors[chain.name.toLowerCase()] || '#f97316', display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: 20, fontWeight: 'bold' }}>{chain.name[0]}</div>
                  <div>
                    <h3 style={{ margin: 0 }}>{chain.name}</h3>
                    <p style={{ margin: 0, color: textSecondary, fontSize: 14 }}>Chain ID: {chain.chainId}</p>
                  </div>
                </div>
                <p style={{ color: textSecondary, fontSize: 14, marginBottom: 12 }}>{chain.description}</p>
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                  <span style={{ background: 'rgba(16,185,129,0.2)', padding: '4px 12px', borderRadius: 8, fontSize: 12, color: '#10b981' }}>{chain.symbol}</span>
                  <span style={{ padding: '4px 12px', borderRadius: 8, fontSize: 12, background: chain.isEnabled ? 'rgba(16,185,129,0.2)' : 'rgba(239,68,68,0.2)', color: chain.isEnabled ? '#10b981' : '#ef4444' }}>{chain.isEnabled ? '✓ Active' : '✗ Disabled'}</span>
                </div>
              </div>
            ))}
          </div>
        </div>

        <div>
          <h2 style={{ display: 'flex', alignItems: 'center', gap: 12, marginBottom: 16 }}>
            <span>🌐</span> Non-EVM Chains <span style={{ background: '#8b5cf6', padding: '2px 12px', borderRadius: 12, fontSize: 12 }}>{chains.filter(c => c.type !== 'evm' && c.isEnabled).length}</span>
          </h2>
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(280px, 1fr))', gap: 16 }}>
            {chains.filter(c => c.type !== 'evm').map(chain => (
              <div key={chain.id} onClick={() => setSelectedChain(chain)} style={{ 
                background: bgSecondary, borderRadius: 12, padding: 20, cursor: 'pointer',
                border: `1px solid ${chainColors[chain.name.toLowerCase()] || '#8b5cf6'}`, opacity: chain.isEnabled ? 1 : 0.5
              }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: 12, marginBottom: 12 }}>
                  <div style={{ width: 48, height: 48, borderRadius: '50%', background: chainColors[chain.name.toLowerCase()] || '#8b5cf6', display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: 20, fontWeight: 'bold' }}>{chain.name[0]}</div>
                  <div>
                    <h3 style={{ margin: 0 }}>{chain.name}</h3>
                    <p style={{ margin: 0, color: textSecondary, fontSize: 14 }}>{chain.type.toUpperCase()}</p>
                  </div>
                </div>
                <p style={{ color: textSecondary, fontSize: 14, marginBottom: 12 }}>{chain.description}</p>
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                  <span style={{ background: 'rgba(139,92,246,0.2)', padding: '4px 12px', borderRadius: 8, fontSize: 12, color: '#8b5cf6' }}>{chain.symbol}</span>
                  <span style={{ padding: '4px 12px', borderRadius: 8, fontSize: 12, background: chain.isEnabled ? 'rgba(16,185,129,0.2)' : 'rgba(239,68,68,0.2)', color: chain.isEnabled ? '#10b981' : '#ef4444' }}>{chain.isEnabled ? '✓ Active' : '✗ Disabled'}</span>
                </div>
              </div>
            ))}
          </div>
        </div>
      </div>

      {selectedChain && (
        <div style={{ position: 'fixed', top: 0, left: 0, right: 0, bottom: 0, background: 'rgba(0,0,0,0.8)', display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 1000 }} onClick={() => setSelectedChain(null)}>
          <div style={{ background: '#1e293b', borderRadius: 16, padding: 32, maxWidth: 500, width: '90%', border: `2px solid ${chainColors[selectedChain.name.toLowerCase()] || '#f97316'}` }} onClick={e => e.stopPropagation()}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 16, marginBottom: 24 }}>
              <div style={{ width: 64, height: 64, borderRadius: '50%', background: chainColors[selectedChain.name.toLowerCase()] || '#f97316', display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: 28, fontWeight: 'bold' }}>{selectedChain.name[0]}</div>
              <div>
                <h2 style={{ margin: 0 }}>{selectedChain.name}</h2>
                <p style={{ margin: 0, color: '#94a3b8' }}>{selectedChain.type.toUpperCase()} • {selectedChain.symbol}</p>
              </div>
            </div>
            <p style={{ color: '#94a3b8', margin: '0 0 4px' }}>Chain ID</p>
            <p style={{ margin: '0 0 16px', fontSize: 18 }}>{selectedChain.chainId}</p>
            <p style={{ color: '#94a3b8', margin: '0 0 4px' }}>Description</p>
            <p style={{ margin: '0 0 24px' }}>{selectedChain.description}</p>
            <div style={{ display: 'flex', gap: 12 }}>
              <button style={{ flex: 1, padding: 12, borderRadius: 8, border: 'none', background: 'rgba(255,255,255,0.1)', color: 'white', cursor: 'pointer' }} onClick={() => setSelectedChain(null)}>Close</button>
              <button style={{ flex: 1, padding: 12, borderRadius: 8, border: 'none', background: '#f97316', color: 'white', cursor: 'pointer', fontWeight: 'bold' }}>Connect</button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}