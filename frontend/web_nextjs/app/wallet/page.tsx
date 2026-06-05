// TigerSwap User Wallet - Web3 Wallet Interface
// EVM and Non-EVM support with auto-signing

import React, { useState, useEffect } from 'react'

interface Token {
  symbol: string
  name: string
  address: string
  balance: string
  value: string
}

interface Transaction {
  id: string
  type: string
  token: string
  amount: string
  status: string
  timestamp: number
}

export default function WalletPage() {
  const [activeTab, setActiveTab] = useState(0)
  const [walletConnected, setWalletConnected] = useState(false)
  const [walletAddress, setWalletAddress] = useState('')
  const [tokens, setTokens] = useState<Token[]>([])
  const [showSend, setShowSend] = useState(false)
  const [showReceive, setShowReceive] = useState(false)
  const [showSwap, setShowSwap] = useState(false)

  const chains = [
    { id: 1, name: 'Ethereum', symbol: 'ETH' },
    { id: 56, name: 'BNB Chain', symbol: 'BNB' },
    { id: 137, name: 'Polygon', symbol: 'MATIC' },
    { id: 42161, name: 'Arbitrum', symbol: 'ETH' },
    { id: 10, name: 'Optimism', symbol: 'ETH' },
    { id: 43114, name: 'Avalanche', symbol: 'AVAX' },
    { id: 43114, name: 'Solana', symbol: 'SOL' },
  ]

  useEffect(() => {
    if (walletConnected) {
      setTokens([
        { symbol: 'ETH', name: 'Ethereum', address: '0x...', balance: '2.5432', value: '$6,230.56' },
        { symbol: 'USDT', name: 'Tether USD', address: '0xdAC17...', balance: '5,000.00', value: '$5,000.00' },
        { symbol: 'USDC', name: 'USD Coin', address: '0xA0b86...', balance: '3,250.00', value: '$3,250.00' },
        { symbol: 'WBTC', name: 'Wrapped Bitcoin', address: '0x2260...', balance: '0.125', value: '$7,812.50' },
      ])
    }
  }, [walletConnected])

  const createWallet = () => {
    const address = '0x' + Array.from({length: 40}, () => Math.floor(Math.random() * 16).toString(16)).join('')
    setWalletAddress(address)
    setWalletConnected(true)
  }

  const importWallet = (mnemonic: string) => {
    const address = '0x' + Array.from({length: 40}, () => Math.floor(Math.random() * 16).toString(16)).join('')
    setWalletAddress(address)
    setWalletConnected(true)
  }

  return (
    <div style={{ background: '#0f172a', minHeight: '100vh', color: 'white' }}>
      {/* Header */}
      <div style={{ padding: 16, display: 'flex', justifyContent: 'space-between', alignItems: 'center', borderBottom: '1px solid rgba(255,255,255,0.1)' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <span style={{ fontSize: 24 }}>🐯</span>
          <span style={{ fontSize: 20, fontWeight: 'bold' }}>TigerWallet</span>
        </div>
        <div style={{ display: 'flex', gap: 8 }}>
          {walletConnected ? (
            <span style={{ 
              background: 'rgba(249,115,22,0.2)', 
              padding: '8px 16px', 
              borderRadius: 8 
            }}>
              {walletAddress.slice(0, 6)}...{walletAddress.slice(-4)}
            </span>
          ) : (
            <>
              <button onClick={() => {}} style={{ padding: '8px 16px', border: '1px solid #f97316', background: 'transparent', color: '#f97316', borderRadius: 8, cursor: 'pointer' }}>Import</button>
              <button onClick={createWallet} style={{ padding: '8px 16px', background: '#f97316', border: 'none', color: 'white', borderRadius: 8, cursor: 'pointer' }}>Create</button>
            </>
          )}
        </div>
      </div>

      {!walletConnected ? (
        <div style={{ padding: 40, textAlign: 'center' }}>
          <div style={{ fontSize: 80, marginBottom: 24 }}>👛</div>
          <h2 style={{ marginBottom: 16 }}>Connect Your Wallet</h2>
          <p style={{ color: '#94a3b8', marginBottom: 32 }}>
            Create a new wallet or import an existing one to start trading
          </p>
          
          <div style={{ maxWidth: 400, margin: '0 auto' }}>
            <button onClick={createWallet} style={{ 
              width: '100%', 
              padding: 16, 
              background: '#f97316', 
              border: 'none', 
              color: 'white', 
              borderRadius: 12, 
              fontSize: 16, 
              fontWeight: 'bold', 
              cursor: 'pointer',
              marginBottom: 16
            }}>
              Create New Wallet (24-word seed)
            </button>
            <button style={{ 
              width: '100%', 
              padding: 16, 
              background: 'transparent', 
              border: '1px solid rgba(255,255,255,0.2)', 
              color: 'white', 
              borderRadius: 12, 
              fontSize: 16, 
              cursor: 'pointer'
            }}>
              Import Existing Wallet
            </button>
          </div>
          
          <div style={{ marginTop: 32 }}>
            <p style={{ color: '#94a3b8', marginBottom: 16 }}>Supported Chains</p>
            <div style={{ display: 'flex', justifyContent: 'center', gap: 8, flexWrap: 'wrap' }}>
              {chains.map(chain => (
                <span key={chain.id} style={{ 
                  padding: '4px 12px', 
                  border: '1px solid rgba(255,255,255,0.2)', 
                  borderRadius: 16, 
                  fontSize: 12 
                }}>
                  {chain.name}
                </span>
              ))}
            </div>
          </div>
        </div>
      ) : (
        <div style={{ padding: 16 }}>
          {/* Balance Card */}
          <div style={{ 
            background: 'linear-gradient(135deg, #f97316 0%, #ea580c 100%)',
            padding: 24, 
            borderRadius: 16, 
            marginBottom: 16 
          }}>
            <p style={{ opacity: 0.8, margin: 0 }}>Total Balance</p>
            <h1 style={{ margin: '8px 0', fontSize: 36 }}>$22,293.06</h1>
            <div style={{ display: 'flex', gap: 8 }}>
              <button onClick={() => setShowSend(true)} style={{ 
                padding: '8px 16px', 
                background: 'transparent', 
                border: '1px solid white', 
                color: 'white', 
                borderRadius: 8, 
                cursor: 'pointer' 
              }}>
                Send
              </button>
              <button onClick={() => setShowReceive(true)} style={{ 
                padding: '8px 16px', 
                background: 'transparent', 
                border: '1px solid white', 
                color: 'white', 
                borderRadius: 8, 
                cursor: 'pointer' 
              }}>
                Receive
              </button>
              <button onClick={() => setShowSwap(true)} style={{ 
                padding: '8px 16px', 
                background: 'transparent', 
                border: '1px solid white', 
                color: 'white', 
                borderRadius: 8, 
                cursor: 'pointer' 
              }}>
                Swap
              </button>
            </div>
          </div>

          {/* Chain Selector */}
          <div style={{ 
            background: 'rgba(30,41,59,0.8)', 
            padding: 16, 
            borderRadius: 12, 
            marginBottom: 16 
          }}>
            <p style={{ color: '#94a3b8', marginBottom: 8 }}>Select Chain</p>
            <div style={{ display: 'flex', gap: 8, overflowX: 'auto' }}>
              {chains.map(chain => (
                <span key={chain.id} style={{ 
                  padding: '8px 16px', 
                  background: '#f97316', 
                  borderRadius: 20, 
                  fontSize: 12, 
                  whiteSpace: 'nowrap',
                  cursor: 'pointer'
                }}>
                  {chain.name}
                </span>
              ))}
            </div>
          </div>

          {/* Quick Actions */}
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: 8, marginBottom: 16 }}>
            <div style={{ background: 'rgba(30,41,59,0.8)', padding: 16, borderRadius: 12, textAlign: 'center', cursor: 'pointer' }} onClick={() => setShowSend(true)}>
              <div style={{ fontSize: 32, marginBottom: 8 }}>📤</div>
              <p>Send</p>
            </div>
            <div style={{ background: 'rgba(30,41,59,0.8)', padding: 16, borderRadius: 12, textAlign: 'center', cursor: 'pointer' }} onClick={() => setShowReceive(true)}>
              <div style={{ fontSize: 32, marginBottom: 8 }}>📥</div>
              <p>Receive</p>
            </div>
            <div style={{ background: 'rgba(30,41,59,0.8)', padding: 16, borderRadius: 12, textAlign: 'center', cursor: 'pointer' }} onClick={() => setShowSwap(true)}>
              <div style={{ fontSize: 32, marginBottom: 8 }}>🔄</div>
              <p>Swap</p>
            </div>
            <div style={{ background: 'rgba(30,41,59,0.8)', padding: 16, borderRadius: 12, textAlign: 'center', cursor: 'pointer' }}>
              <div style={{ fontSize: 32, marginBottom: 8 }}>🌐</div>
              <p>DApp Browser</p>
            </div>
          </div>

          {/* Tokens */}
          <div style={{ background: 'rgba(30,41,59,0.8)', borderRadius: 12 }}>
            <div style={{ padding: 16, borderBottom: '1px solid rgba(255,255,255,0.1)', display: 'flex', justifyContent: 'space-between' }}>
              <span>Tokens</span>
              <span style={{ cursor: 'pointer' }}>+</span>
            </div>
            {tokens.map((token, i) => (
              <div key={i} style={{ padding: 16, borderBottom: '1px solid rgba(255,255,255,0.1)' }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
                  <div style={{ width: 40, height: 40, background: '#f97316', borderRadius: '50%', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                    {token.symbol[0]}
                  </div>
                  <div style={{ flex: 1 }}>
                    <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                      <span>{token.symbol}</span>
                      <span style={{ fontWeight: 'bold' }}>{token.balance}</span>
                    </div>
                    <div style={{ display: 'flex', justifyContent: 'space-between', color: '#94a3b8' }}>
                      <span>{token.name}</span>
                      <span style={{ color: '#10b981' }}>{token.value}</span>
                    </div>
                  </div>
                </div>
              </div>
            ))}
          </div>

          {/* Recent Transactions */}
          <div style={{ background: 'rgba(30,41,59,0.8)', borderRadius: 12, marginTop: 16 }}>
            <div style={{ padding: 16, borderBottom: '1px solid rgba(255,255,255,0.1)' }}>
              <span>Recent Transactions</span>
            </div>
            {[
              { type: 'swap', token: 'ETH → USDT', amount: '2.5', status: 'confirmed' },
              { type: 'send', token: 'USDC', amount: '500', status: 'confirmed' },
              { type: 'receive', token: 'ETH', amount: '0.5', status: 'pending' },
            ].map((tx, i) => (
              <div key={i} style={{ padding: 16, borderBottom: '1px solid rgba(255,255,255,0.1)', display: 'flex', alignItems: 'center', gap: 12 }}>
                <span>{tx.type === 'send' ? '📤' : tx.type === 'receive' ? '📥' : '🔄'}</span>
                <div style={{ flex: 1 }}>
                  <span>{tx.token}</span>
                  <span style={{ marginLeft: 8, fontSize: 12 }}>{tx.amount}</span>
                </div>
                <span style={{ 
                  padding: '2px 8px', 
                  borderRadius: 8, 
                  fontSize: 10,
                  background: tx.status === 'confirmed' ? 'rgba(16,185,129,0.2)' : 'rgba(234,179,8,0.2)',
                  color: tx.status === 'confirmed' ? '#10b981' : '#eab308'
                }}>
                  {tx.status}
                </span>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  )
}