// TigerSwap - Token Listing Application Page
import React, { useState } from 'react'

export default function TokenListingPage() {
  const [step, setStep] = useState(1)
  const [token, setToken] = useState({ symbol: '', name: '', address: '', chainId: 1 })
  const [quoteToken, setQuoteToken] = useState('USDT')
  const [selectedTier, setSelectedTier] = useState('tier3')
  const [agreed, setAgreed] = useState(false)

  const tiers = [
    { id: 'tier1', name: 'Tier 1 - Major Pairs', fee: '5000', feeUsd: '2500' },
    { id: 'tier2', name: 'Tier 2 - Established', fee: '2000', feeUsd: '1000' },
    { id: 'tier3', name: 'Tier 3 - New Tokens', fee: '1000', feeUsd: '500' },
    { id: 'tier4', name: 'Tier 4 - Community', fee: '500', feeUsd: '250' },
  ]

  const chains = [
    { id: 1, name: 'Ethereum' }, { id: 56, name: 'BNB Chain' }, { id: 137, name: 'Polygon' },
    { id: 42161, name: 'Arbitrum' }, { id: 10, name: 'Optimism' }, { id: 43114, name: 'Avalanche' },
  ]

  const quoteTokens = ['USDT', 'USDC', 'ETH', 'BNB']

  return (
    <div style={{ background: '#0f172a', minHeight: '100vh', color: 'white' }}>
      <div style={{ padding: 24, borderBottom: '1px solid rgba(255,255,255,0.1)' }}>
        <div style={{ maxWidth: 800, margin: '0 auto' }}>
          <h1 style={{ fontSize: 28, margin: 0 }}>📋 Token Listing Application</h1>
          <p style={{ color: '#94a3b8', margin: '8px 0 0' }}>Apply to list your token on TigerSwap</p>
        </div>
      </div>

      <div style={{ maxWidth: 800, margin: '0 auto', padding: '24px 0' }}>
        {/* Progress */}
        <div style={{ display: 'flex', justifyContent: 'center', gap: 16, marginBottom: 32 }}>
          {[1, 2, 3].map(s => (
            <div key={s} style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
              <div style={{ width: 40, height: 40, borderRadius: '50%', background: step >= s ? '#f97316' : 'rgba(255,255,255,0.1)', display: 'flex', alignItems: 'center', justifyContent: 'center', fontWeight: 'bold' }}>
                {step > s ? '✓' : s}
              </div>
              <span style={{ color: step >= s ? 'white' : '#94a3b8' }}>{s === 1 ? 'Token' : s === 2 ? 'Tier' : 'Review'}</span>
            </div>
          ))}
        </div>

        {/* Step 1: Token Info */}
        {step === 1 && (
          <div style={{ background: 'rgba(30,41,59,0.8)', borderRadius: 16, padding: 32 }}>
            <h2>Token Information</h2>
            
            <div style={{ marginBottom: 24 }}>
              <label style={{ display: 'block', marginBottom: 8, color: '#94a3b8' }}>Chain</label>
              <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
                {chains.map(chain => (
                  <div key={chain.id} onClick={() => setToken({...token, chainId: chain.id})} style={{ padding: '12px 24px', borderRadius: 8, border: `1px solid ${token.chainId === chain.id ? '#f97316' : 'rgba(255,255,255,0.2)'}`, background: token.chainId === chain.id ? 'rgba(249,115,22,0.2)' : 'transparent', cursor: 'pointer' }}>
                    {chain.name}
                  </div>
                ))}
              </div>
            </div>

            <div style={{ marginBottom: 24 }}>
              <label style={{ display: 'block', marginBottom: 8, color: '#94a3b8' }}>Token Contract Address</label>
              <input type="text" placeholder="0x..." value={token.address} onChange={(e) => setToken({...token, address: e.target.value})} style={{ width: '100%', padding: 12, borderRadius: 8, background: 'rgba(0,0,0,0.3)', border: '1px solid rgba(255,255,255,0.2)', color: 'white', fontSize: 16 }} />
            </div>

            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 16, marginBottom: 24 }}>
              <div>
                <label style={{ display: 'block', marginBottom: 8, color: '#94a3b8' }}>Symbol</label>
                <input type="text" placeholder="e.g., BTC" value={token.symbol} onChange={(e) => setToken({...token, symbol: e.target.value})} style={{ width: '100%', padding: 12, borderRadius: 8, background: 'rgba(0,0,0,0.3)', border: '1px solid rgba(255,255,255,0.2)', color: 'white', fontSize: 16 }} />
              </div>
              <div>
                <label style={{ display: 'block', marginBottom: 8, color: '#94a3b8' }}>Name</label>
                <input type="text" placeholder="e.g., Bitcoin" value={token.name} onChange={(e) => setToken({...token, name: e.target.value})} style={{ width: '100%', padding: 12, borderRadius: 8, background: 'rgba(0,0,0,0.3)', border: '1px solid rgba(255,255,255,0.2)', color: 'white', fontSize: 16 }} />
              </div>
            </div>

            <div style={{ marginBottom: 24 }}>
              <label style={{ display: 'block', marginBottom: 8, color: '#94a3b8' }}>Quote Token (Pair)</label>
              <div style={{ display: 'flex', gap: 8 }}>
                {quoteTokens.map(qt => (
                  <div key={qt} onClick={() => setQuoteToken(qt)} style={{ padding: '12px 24px', borderRadius: 8, border: `1px solid ${quoteToken === qt ? '#f97316' : 'rgba(255,255,255,0.2)'}`, background: quoteToken === qt ? 'rgba(249,115,22,0.2)' : 'transparent', cursor: 'pointer' }}>
                    {qt}
                  </div>
                ))}
              </div>
            </div>

            <button onClick={() => setStep(2)} disabled={!token.address || !token.symbol || !token.name} style={{ padding: '16px 32px', background: '#f97316', border: 'none', borderRadius: 8, color: 'white', fontSize: 16, fontWeight: 'bold', cursor: 'pointer', opacity: (!token.address || !token.symbol || !token.name) ? 0.5 : 1 }}>
              Continue to Tier Selection
            </button>
          </div>
        )}

        {/* Step 2: Tier Selection */}
        {step === 2 && (
          <div style={{ background: 'rgba(30,41,59,0.8)', borderRadius: 16, padding: 32 }}>
            <h2>Select Listing Tier</h2>
            <p style={{ color: '#94a3b8', marginBottom: 24 }}>Choose the tier that best fits your token.</p>

            <div style={{ display: 'grid', gap: 16 }}>
              {tiers.map(tier => (
                <div key={tier.id} onClick={() => setSelectedTier(tier.id)} style={{ padding: 24, borderRadius: 12, border: `2px solid ${selectedTier === tier.id ? '#f97316' : 'rgba(255,255,255,0.1)'}`, background: selectedTier === tier.id ? 'rgba(249,115,22,0.1)' : 'transparent', cursor: 'pointer' }}>
                  <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                    <div>
                      <h3 style={{ margin: 0 }}>{tier.name}</h3>
                    </div>
                    <div style={{ textAlign: 'right' }}>
                      <div style={{ fontSize: 24, fontWeight: 'bold', color: '#f97316' }}>{tier.fee}</div>
                      <div style={{ color: '#94a3b8' }}>TIGER ≈ ${tier.feeUsd}</div>
                    </div>
                  </div>
                </div>
              ))}
            </div>

            <div style={{ display: 'flex', gap: 16, marginTop: 24 }}>
              <button onClick={() => setStep(1)} style={{ padding: '16px 32px', background: 'transparent', border: '1px solid rgba(255,255,255,0.2)', borderRadius: 8, color: 'white', cursor: 'pointer' }}>Back</button>
              <button onClick={() => setStep(3)} style={{ padding: '16px 32px', background: '#f97316', border: 'none', borderRadius: 8, color: 'white', fontSize: 16, fontWeight: 'bold', cursor: 'pointer' }}>Continue to Review</button>
            </div>
          </div>
        )}

        {/* Step 3: Review */}
        {step === 3 && (
          <div style={{ background: 'rgba(30,41,59,0.8)', borderRadius: 16, padding: 32 }}>
            <h2>Review & Submit</h2>

            <div style={{ background: 'rgba(0,0,0,0.3)', borderRadius: 12, padding: 24, marginBottom: 24 }}>
              <h3>Token Details</h3>
              <p><strong>Token:</strong> {token.symbol} ({token.name})</p>
              <p><strong>Contract:</strong> {token.address}</p>
              <p><strong>Chain:</strong> {chains.find(c => c.id === token.chainId)?.name}</p>
              <p><strong>Pair:</strong> {token.symbol}/{quoteToken}</p>
            </div>

            <div style={{ background: 'rgba(0,0,0,0.3)', borderRadius: 12, padding: 24, marginBottom: 24 }}>
              <h3>Listing Tier</h3>
              <p><strong>{tiers.find(t => t.id === selectedTier)?.name}</strong></p>
              <p style={{ fontSize: 24, fontWeight: 'bold', color: '#f97316' }}>{tiers.find(t => t.id === selectedTier)?.fee} TIGER ≈ ${tiers.find(t => t.id === selectedTier)?.feeUsd}</p>
            </div>

            <div style={{ marginBottom: 24 }}>
              <label style={{ display: 'flex', alignItems: 'center', gap: 12, cursor: 'pointer' }}>
                <input type="checkbox" checked={agreed} onChange={(e) => setAgreed(e.target.checked)} style={{ width: 20, height: 20 }} />
                <span>I agree to the <a href="#" style={{ color: '#f97316' }}>Token Listing Terms</a></span>
              </label>
            </div>

            <div style={{ display: 'flex', gap: 16 }}>
              <button onClick={() => setStep(2)} style={{ padding: '16px 32px', background: 'transparent', border: '1px solid rgba(255,255,255,0.2)', borderRadius: 8, color: 'white', cursor: 'pointer' }}>Back</button>
              <button onClick={() => alert('Application submitted!')} disabled={!agreed} style={{ padding: '16px 32px', background: '#f97316', border: 'none', borderRadius: 8, color: 'white', fontSize: 16, fontWeight: 'bold', cursor: 'pointer', opacity: !agreed ? 0.5 : 1 }}>Submit Application</button>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}