'use client';
import { useState } from 'react';
import { ThemeToggle } from './components/ThemeToggle';
import { useTheme } from './components/ThemeProvider';
export default function Home() {
    const [swapFrom, setSwapFrom] = useState({ token: 'ETH', amount: '' });
    const [swapTo, setSwapTo] = useState({ token: 'USDT', amount: '' });
    const [slippage, setSlippage] = useState(0.5);
    const { theme } = useTheme();
    const popularTokens = ['ETH', 'USDT', 'USDC', 'BNB', 'MATIC', 'ARB', 'WBTC', 'DAI'];
    const supportedChains = ['Ethereum', 'BNB Chain', 'Polygon', 'Arbitrum', 'Optimism', 'Base', 'Avalanche'];
    return (<div className="container">
      <header className="header">
        <div className="logo">🐯 TigerSwap</div>
        <nav className="nav">
          <a href="/swap" className="nav-link">Swap</a>
          <a href="/pool" className="nav-link">Pool</a>
          <a href="/bridge" className="nav-link">Bridge</a>
          <a href="/farming" className="nav-link">Farming</a>
        </nav>
        <div className="flex items-center gap-4">
          <ThemeToggle />
          <button className="btn-primary">Connect Wallet</button>
        </div>
      </header>

      <main className="flex-1 p-8 max-w-6xl mx-auto w-full">
        <div className="text-center py-12">
          <h1 className="text-5xl font-bold gradient-text mb-4">Multichain DEX Aggregator</h1>
          <p className="text-xl text-slate-400 dark:text-slate-500">Swap across 19 chains, 20+ DEXs, with the best rates</p>
        </div>

        <div className="swap-card">
          <div className="mb-4">
            <select className="form-select">
              {supportedChains.map(chain => (<option key={chain} value={chain}>{chain}</option>))}
            </select>
          </div>

          <div className="bg-slate-900/60 dark:bg-slate-900/60 bg-white/50 rounded-xl p-6 mb-4">
            <div className="flex items-center gap-4 flex-wrap">
              <span className="text-slate-400 dark:text-slate-500 min-w-[60px]">From</span>
              <input type="number" placeholder="0.0" value={swapFrom.amount} onChange={(e) => setSwapFrom({ ...swapFrom, amount: e.target.value })} className="flex-1 bg-transparent border-none text-slate-50 dark:text-slate-50 text-slate-900 text-xl outline-none min-w-[100px]"/>
              <div className="bg-orange-500/20 px-4 py-2 rounded-lg cursor-pointer">
                <span>{swapFrom.token}</span>
              </div>
            </div>

            <button className="block mx-auto my-4 bg-orange-500/20 border-none text-orange-500 text-xl px-4 py-2 rounded-full cursor-pointer">↓</button>

            <div className="flex items-center gap-4 flex-wrap">
              <span className="text-slate-400 dark:text-slate-500 min-w-[60px]">To</span>
              <input type="number" placeholder="0.0" value={swapTo.amount} onChange={(e) => setSwapTo({ ...swapTo, amount: e.target.value })} className="flex-1 bg-transparent border-none text-slate-50 dark:text-slate-50 text-slate-900 text-xl outline-none min-w-[100px]"/>
              <div className="bg-orange-500/20 px-4 py-2 rounded-lg cursor-pointer">
                <span>{swapTo.token}</span>
              </div>
            </div>
          </div>

          <div className="flex items-center gap-4 my-4">
            <label className="text-slate-400 dark:text-slate-500">Slippage Tolerance: {slippage}%</label>
            <input type="range" min="0.1" max="5" step="0.1" value={slippage} onChange={(e) => setSlippage(parseFloat(e.target.value))} className="flex-1"/>
          </div>

          <button className="w-full bg-gradient-to-r from-orange-500 to-orange-600 text-white border-none p-4 rounded-xl text-lg font-semibold cursor-pointer transition-transform hover:scale-[1.02]">
            Swap
          </button>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-4 gap-8 mt-12">
          {[
            { label: 'Total Value Locked', value: '$2.4B+' },
            { label: '24h Volume', value: '$890M+' },
            { label: 'Active Users', value: '125K+' },
            { label: 'Trading Pairs', value: '3200+' },
        ].map(stat => (<div key={stat.label} className="stat-card">
              <span className="stat-value">{stat.value}</span>
              <span className="stat-label">{stat.label}</span>
            </div>))}
        </div>

        <div className="grid grid-cols-1 md:grid-cols-3 gap-8 mt-12">
          {[
            { title: 'Best Rates', desc: 'Auto-routing across 20+ DEXs for optimal prices' },
            { title: 'Fast Swaps', desc: 'Sub-3-second transaction signing' },
            { title: 'Low Fees', desc: 'Dynamic fee optimization' },
        ].map(feature => (<div key={feature.title} className="feature-card">
              <h3 className="feature-title">{feature.title}</h3>
              <p className="feature-description">{feature.desc}</p>
            </div>))}
        </div>
      </main>

      <footer className="text-center p-8 border-t border-white/10 dark:border-white/10 border-black/10 text-slate-600 dark:text-slate-500">
        <p>© 2026 TigerSwap - Enterprise DEX</p>
      </footer>
    </div>);
}
//# sourceMappingURL=page.jsx.map