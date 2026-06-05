'use client'

import { useState } from 'react'
import styles from './page.module.css'

export default function Home() {
  const [swapFrom, setSwapFrom] = useState({ token: 'ETH', amount: '' })
  const [swapTo, setSwapTo] = useState({ token: 'USDT', amount: '' })
  const [slippage, setSlippage] = useState(0.5)

  const popularTokens = ['ETH', 'USDT', 'USDC', 'BNB', 'MATIC', 'ARB', 'WBTC', 'DAI']
  const supportedChains = ['Ethereum', 'BNB Chain', 'Polygon', 'Arbitrum', 'Optimism', 'Base', 'Avalanche']

  return (
    <div className={styles.container}>
      <header className={styles.header}>
        <div className={styles.logo}>🐯 TigerSwap</div>
        <nav className={styles.nav}>
          <a href="/swap">Swap</a>
          <a href="/pool">Pool</a>
          <a href="/bridge">Bridge</a>
          <a href="/farming">Farming</a>
        </nav>
        <button className={styles.connectBtn}>Connect Wallet</button>
      </header>

      <main className={styles.main}>
        <div className={styles.hero}>
          <h1>Multichain DEX Aggregator</h1>
          <p>Swap across 19 chains, 20+ DEXs, with the best rates</p>
        </div>

        <div className={styles.swapCard}>
          <div className={styles.chainSelector}>
            <select className={styles.chainSelect}>
              {supportedChains.map(chain => (
                <option key={chain} value={chain}>{chain}</option>
              ))}
            </select>
          </div>

          <div className={styles.swapBox}>
            <div className={styles.swapInput}>
              <span className={styles.label}>From</span>
              <input 
                type="number" 
                placeholder="0.0" 
                value={swapFrom.amount}
                onChange={(e) => setSwapFrom({...swapFrom, amount: e.target.value})}
              />
              <div className={styles.tokenSelect}>
                <span>{swapFrom.token}</span>
              </div>
            </div>

            <button className={styles.swapArrow}>↓</button>

            <div className={styles.swapInput}>
              <span className={styles.label}>To</span>
              <input 
                type="number" 
                placeholder="0.0" 
                value={swapTo.amount}
                onChange={(e) => setSwapTo({...swapTo, amount: e.target.value})}
              />
              <div className={styles.tokenSelect}>
                <span>{swapTo.token}</span>
              </div>
            </div>
          </div>

          <div className={styles.swapSettings}>
            <label>Slippage Tolerance: {slippage}%</label>
            <input 
              type="range" 
              min="0.1" 
              max="5" 
              step="0.1" 
              value={slippage}
              onChange={(e) => setSlippage(parseFloat(e.target.value))}
            />
          </div>

          <button className={styles.swapBtn}>Swap</button>
        </div>

        <div className={styles.features}>
          <div className={styles.feature}>
            <h3>🔄 Cross-Chain Swaps</h3>
            <p>Seamlessly swap tokens across 19+ blockchains</p>
          </div>
          <div className={styles.feature}>
            <h3>📊 Best Rates</h3>
            <p>Aggregates 20+ DEXs for optimal pricing</p>
          </div>
          <div className={styles.feature}>
            <h3>⚡ Fast Execution</h3>
            <p>Lightning-fast transaction execution</p>
          </div>
          <div className={styles.feature}>
            <h3>🔒 Secure</h3>
            <p>Advanced security with MEV protection</p>
          </div>
        </div>

        <div className={styles.stats}>
          <div className={styles.stat}>
            <span className={styles.statValue}>$2.1B+</span>
            <span className={styles.statLabel}>Total Volume</span>
          </div>
          <div className={styles.stat}>
            <span className={styles.statValue}>154K+</span>
            <span className={styles.statLabel}>Users</span>
          </div>
          <div className={styles.stat}>
            <span className={styles.statValue}>19+</span>
            <span className={styles.statLabel}>Chains</span>
          </div>
          <div className={styles.stat}>
            <span className={styles.statValue}>20+</span>
            <span className={styles.statLabel}>DEXs</span>
          </div>
        </div>
      </main>

      <footer className={styles.footer}>
        <p>TigerSwap © 2024 - Enterprise-grade Multichain DEX</p>
      </footer>
    </div>
  )
}