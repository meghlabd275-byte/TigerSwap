'use client'

import { useState, useEffect } from 'react'
import { ThemeToggle } from '../components/ThemeToggle'
import { useTheme } from '../components/ThemeProvider'

interface WhiteLabelClient {
  client_id: string
  client_name: string
  brand_name: string
  status: string
  tier: string
  swap_fee_share_bps: number
  trading_fee_share_bps: number
  bot_subscription_fee_share_bps: number
  total_volume_usd: number
  total_fees_paid: number
  total_users: number
  can_use_swap: boolean
  can_use_trading: boolean
  can_use_bots: boolean
  can_use_listings: boolean
  can_use_bridge: boolean
  can_use_farming: boolean
}

export default function WhiteLabelDashboard() {
  const { theme } = useTheme()
  const [clients, setClients] = useState<WhiteLabelClient[]>([])
  const [loading, setLoading] = useState(true)
  const [selectedClient, setSelectedClient] = useState<WhiteLabelClient | null>(null)
  const [showModal, setShowModal] = useState(false)
  const [newClient, setNewClient] = useState({
    client_name: '',
    brand_name: '',
    contact_email: '',
    tier: 'basic',
  })

  useEffect(() => {
    fetchClients()
  }, [])

  const fetchClients = async () => {
    try {
      const response = await fetch('/api/admin/white-label/clients')
      const data = await response.json()
      setClients(data.clients || [])
    } catch (error) {
      console.error('Failed to fetch clients:', error)
    } finally {
      setLoading(false)
    }
  }

  const approveClient = async (clientId: string) => {
    try {
      await fetch('/api/admin/white-label/approve', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ client_id: clientId }),
      })
      fetchClients()
    } catch (error) {
      console.error('Failed to approve client:', error)
    }
  }

  const suspendClient = async (clientId: string, reason: string) => {
    try {
      await fetch('/api/admin/white-label/suspend', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ client_id: clientId, reason }),
      })
      fetchClients()
    } catch (error) {
      console.error('Failed to suspend client:', error)
    }
  }

  const toggleFeature = async (clientId: string, feature: string, enabled: boolean) => {
    try {
      await fetch('/api/admin/white-label/feature', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ client_id: clientId, feature, enabled }),
      })
      fetchClients()
    } catch (error) {
      console.error('Failed to toggle feature:', error)
    }
  }

  const updateFees = async (clientId: string, feeType: string, bps: number) => {
    try {
      await fetch('/api/admin/white-label/fees', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ client_id: clientId, [feeType]: bps }),
      })
      fetchClients()
    } catch (error) {
      console.error('Failed to update fees:', error)
    }
  }

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'approved':
        return 'bg-green-500'
      case 'pending':
        return 'bg-yellow-500'
      case 'suspended':
        return 'bg-red-500'
      default:
        return 'bg-gray-500'
    }
  }

  const formatBPS = (bps: number) => `${(bps / 100).toFixed(1)}%`

  return (
    <div className="container">
      <header className="header">
        <div className="logo">🐯 TigerSwap Admin</div>
        <nav className="nav">
          <a href="/admin_fees" className="nav-link">Fees</a>
          <a href="/admin_listing" className="nav-link">Listing</a>
          <a href="/white_label" className="nav-link">White Label</a>
        </nav>
        <div className="flex items-center gap-4">
          <ThemeToggle />
        </div>
      </header>

      <main className="flex-1 p-8 max-w-7xl mx-auto w-full">
        <div className="flex justify-between items-center mb-8">
          <h1 className="text-3xl font-bold">White Label Clients</h1>
          <button
            onClick={() => setShowModal(true)}
            className="btn-primary"
          >
            + New Client
          </button>
        </div>

        {loading ? (
          <div className="text-center py-12">Loading...</div>
        ) : (
          <div className="grid gap-6">
            {clients.map((client) => (
              <div key={client.client_id} className="card">
                <div className="flex justify-between items-start mb-4">
                  <div>
                    <h3 className="text-xl font-bold">{client.client_name}</h3>
                    <p className="text-slate-400">{client.brand_name}</p>
                  </div>
                  <div className="flex items-center gap-4">
                    <span className={`px-3 py-1 rounded-full text-white text-sm ${getStatusColor(client.status)}`}>
                      {client.status}
                    </span>
                    <span className="text-slate-400">{client.tier}</span>
                  </div>
                </div>

                <div className="grid grid-cols-4 gap-4 mb-4">
                  <div>
                    <span className="text-slate-400">Volume</span>
                    <p className="text-xl font-bold">${client.total_volume_usd.toLocaleString()}</p>
                  </div>
                  <div>
                    <span className="text-slate-400">Fees Paid</span>
                    <p className="text-xl font-bold text-orange-500">${client.total_fees_paid.toLocaleString()}</p>
                  </div>
                  <div>
                    <span className="text-slate-400">Users</span>
                    <p className="text-xl font-bold">{client.total_users}</p>
                  </div>
                  <div>
                    <span className="text-slate-400">Swap Fee</span>
                    <p className="text-xl font-bold">{formatBPS(client.swap_fee_share_bps)}</p>
                  </div>
                </div>

                <div className="border-t border-white/10 pt-4">
                  <h4 className="font-semibold mb-3">Features</h4>
                  <div className="flex flex-wrap gap-2">
                    {[
                      { key: 'swap', label: 'Swap', enabled: client.can_use_swap },
                      { key: 'trading', label: 'Trading', enabled: client.can_use_trading },
                      { key: 'bots', label: 'Bots', enabled: client.can_use_bots },
                      { key: 'listings', label: 'Listings', enabled: client.can_use_listings },
                      { key: 'bridge', label: 'Bridge', enabled: client.can_use_bridge },
                      { key: 'farming', label: 'Farming', enabled: client.can_use_farming },
                    ].map((feature) => (
                      <button
                        key={feature.key}
                        onClick={() => toggleFeature(client.client_id, feature.key, !feature.enabled)}
                        className={`px-3 py-1 rounded text-sm ${
                          feature.enabled
                            ? 'bg-green-500/20 text-green-400'
                            : 'bg-red-500/20 text-red-400'
                        }`}
                      >
                        {feature.label}: {feature.enabled ? 'ON' : 'OFF'}
                      </button>
                    ))}
                  </div>
                </div>

                <div className="border-t border-white/10 pt-4 mt-4 flex gap-4">
                  {client.status === 'pending' && (
                    <button
                      onClick={() => approveClient(client.client_id)}
                      className="btn-primary"
                    >
                      Approve
                    </button>
                  )}
                  {client.status === 'approved' && (
                    <button
                      onClick={() => suspendClient(client.client_id, 'Admin suspended')}
                      className="btn-secondary bg-red-500/20 text-red-400"
                    >
                      Suspend/Halt
                    </button>
                  )}
                  <button
                    onClick={() => setSelectedClient(client)}
                    className="btn-secondary"
                  >
                    Edit Fees
                  </button>
                </div>
              </div>
            ))}
          </div>
        )}

        {clients.length === 0 && (
          <div className="text-center py-12 text-slate-400">
            No white label clients yet
          </div>
        )}
      </main>

      {showModal && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="card max-w-lg w-full">
            <h2 className="text-2xl font-bold mb-4">Create White Label Client</h2>
            <div className="space-y-4">
              <div>
                <label className="block text-slate-400 mb-2">Client Name</label>
                <input
                  type="text"
                  value={newClient.client_name}
                  onChange={(e) => setNewClient({ ...newClient, client_name: e.target.value })}
                  className="input-field"
                />
              </div>
              <div>
                <label className="block text-slate-400 mb-2">Brand Name</label>
                <input
                  type="text"
                  value={newClient.brand_name}
                  onChange={(e) => setNewClient({ ...newClient, brand_name: e.target.value })}
                  className="input-field"
                />
              </div>
              <div>
                <label className="block text-slate-400 mb-2">Contact Email</label>
                <input
                  type="email"
                  value={newClient.contact_email}
                  onChange={(e) => setNewClient({ ...newClient, contact_email: e.target.value })}
                  className="input-field"
                />
              </div>
              <div>
                <label className="block text-slate-400 mb-2">Tier</label>
                <select
                  value={newClient.tier}
                  onChange={(e) => setNewClient({ ...newClient, tier: e.target.value })}
                  className="form-select"
                >
                  <option value="basic">Basic</option>
                  <option value="pro">Pro</option>
                  <option value="enterprise">Enterprise</option>
                </select>
              </div>
            </div>
            <div className="flex gap-4 mt-6">
              <button onClick={() => setShowModal(false)} className="btn-secondary flex-1">
                Cancel
              </button>
              <button className="btn-primary flex-1">Create</button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}