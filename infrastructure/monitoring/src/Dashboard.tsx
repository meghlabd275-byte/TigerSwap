/**
 * TigerSwap Monitoring Dashboard
 * Real-time monitoring and alerting for DEX operations
 * 
 * @author TigerSwap
 * @version 1.0.0 Production
 */

import React, { useState, useEffect } from 'react';
import { LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, AreaChart, Area } from 'recharts';
import { Activity, Zap, Shield, Clock, AlertTriangle, CheckCircle, XCircle, TrendingUp, TrendingDown, Wallet, Globe, Database, Server } from 'lucide-react';

// Types
interface Metric {
  name: string;
  value: number;
  change: number;
  unit: string;
}

interface Alert {
  id: string;
  severity: 'critical' | 'warning' | 'info';
  message: string;
  timestamp: number;
  resolved: boolean;
}

interface Node {
  id: string;
  name: string;
  status: 'online' | 'offline' | 'degraded';
  cpu: number;
  memory: number;
  requestsPerSec: number;
  latency: number;
}

// Sample data
const volumeData = [
  { time: '00:00', volume: 4500000 },
  { time: '04:00', volume: 3200000 },
  { time: '08:00', volume: 5800000 },
  { time: '12:00', volume: 12000000 },
  { time: '16:00', volume: 9800000 },
  { time: '20:00', volume: 8200000 },
  { time: '24:00', volume: 5400000 },
];

const tpsData = [
  { time: '00:00', tps: 45 },
  { time: '04:00', tps: 32 },
  { time: '08:00', tps: 58 },
  { time: '12:00', tps: 120 },
  { time: '16:00', tps: 98 },
  { time: '20:00', tps: 82 },
  { time: '24:00', tps: 54 },
];

const metrics: Metric[] = [
  { name: 'TVL', value: 234567890, change: 2.34, unit: 'USD' },
  { name: '24h Volume', value: 123456789, change: 5.67, unit: 'USD' },
  { name: 'Active Pools', value: 1250, change: 12, unit: '' },
  { name: '24h Trades', value: 45678, change: -3.2, unit: '' },
];

const alerts: Alert[] = [
  { id: '1', severity: 'warning', message: 'High latency detected on Arbitrum node', timestamp: Date.now() - 300000, resolved: false },
  { id: '2', severity: 'info', message: 'New pool created: WBTC/USDC', timestamp: Date.now() - 600000, resolved: true },
  { id: '3', severity: 'critical', message: 'RPC connection failed on Polygon', timestamp: Date.now() - 120000, resolved: false },
];

const nodes: Node[] = [
  { id: '1', name: 'API Gateway US-East', status: 'online', cpu: 45, memory: 62, requestsPerSec: 12000, latency: 12 },
  { id: '2', name: 'API Gateway EU-West', status: 'online', cpu: 38, memory: 55, requestsPerSec: 8500, latency: 18 },
  { id: '3', name: 'Matching Engine Primary', status: 'online', cpu: 72, memory: 68, requestsPerSec: 45000, latency: 2 },
  { id: '4', name: 'Matching Engine Backup', status: 'degraded', cpu: 85, memory: 82, requestsPerSec: 12000, latency: 45 },
  { id: '5', name: 'Indexer Ethereum', status: 'online', cpu: 55, memory: 70, requestsPerSec: 5000, latency: 25 },
  { id: '6', name: 'Indexer Polygon', status: 'offline', cpu: 0, memory: 0, requestsPerSec: 0, latency: 0 },
];

export default function Dashboard() {
  const [activeTab, setActiveTab] = useState('overview');

  return (
    <div className="min-h-screen bg-slate-950 text-white">
      {/* Header */}
      <header className="border-b border-slate-800 bg-slate-900/50 backdrop-blur-lg sticky top-0 z-50">
        <div className="max-w-7xl mx-auto px-6 py-4">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-3">
              <div className="w-10 h-10 rounded-xl bg-gradient-to-br from-orange-500 to-amber-600 flex items-center justify-center">
                <span className="text-white font-bold text-xl">T</span>
              </div>
              <span className="text-2xl font-bold gradient-text">TigerSwap</span>
              <span className="text-slate-500 ml-2">Monitor</span>
            </div>
            <div className="flex items-center gap-4">
              <div className="flex items-center gap-2 px-3 py-1 bg-green-500/20 rounded-full">
                <div className="w-2 h-2 bg-green-500 rounded-full animate-pulse" />
                <span className="text-sm text-green-400">System Online</span>
              </div>
              <div className="text-sm text-slate-400">
                Last updated: {new Date().toLocaleTimeString()}
              </div>
            </div>
          </div>
        </div>
      </header>

      <div className="max-w-7xl mx-auto px-6 py-8">
        {/* Navigation Tabs */}
        <div className="flex gap-4 mb-8 border-b border-slate-800 pb-4">
          {['overview', 'nodes', 'alerts', 'analytics'].map((tab) => (
            <button
              key={tab}
              onClick={() => setActiveTab(tab)}
              className={`px-4 py-2 rounded-lg font-medium transition-colors ${
                activeTab === tab
                  ? 'bg-orange-500/20 text-orange-400'
                  : 'text-slate-400 hover:text-white hover:bg-slate-800'
              }`}
            >
              {tab.charAt(0).toUpperCase() + tab.slice(1)}
            </button>
          ))}
        </div>

        {activeTab === 'overview' && (
          <>
            {/* Metrics Grid */}
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6 mb-8">
              {metrics.map((metric) => (
                <div key={metric.name} className="glass-card p-6">
                  <div className="flex items-center justify-between mb-4">
                    <span className="text-slate-400 text-sm">{metric.name}</span>
                    {metric.change > 0 ? (
                      <TrendingUp className="w-4 h-4 text-green-400" />
                    ) : (
                      <TrendingDown className="w-4 h-4 text-red-400" />
                    )}
                  </div>
                  <div className="text-3xl font-bold mb-1">
                    {metric.value.toLocaleString()}{metric.unit && ` ${metric.unit}`}
                  </div>
                  <div className={`text-sm ${metric.change > 0 ? 'text-green-400' : 'text-red-400'}`}>
                    {metric.change > 0 ? '+' : ''}{metric.change}%
                  </div>
                </div>
              ))}
            </div>

            {/* Charts */}
            <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 mb-8">
              <div className="glass-card p-6">
                <h3 className="text-lg font-semibold mb-4 flex items-center gap-2">
                  <Activity className="w-5 h-5 text-orange-400" />
                  Volume (24h)
                </h3>
                <ResponsiveContainer width="100%" height={300}>
                  <AreaChart data={volumeData}>
                    <defs>
                      <linearGradient id="volumeGradient" x1="0" y1="0" x2="0" y2="1">
                        <stop offset="5%" stopColor="#f97316" stopOpacity={0.3} />
                        <stop offset="95%" stopColor="#f97316" stopOpacity={0} />
                      </linearGradient>
                    </defs>
                    <CartesianGrid strokeDasharray="3 3" stroke="#334155" />
                    <XAxis dataKey="time" stroke="#64748b" />
                    <YAxis stroke="#64748b" tickFormatter={(v) => `$${(v/1000000).toFixed(1)}M`} />
                    <Tooltip
                      contentStyle={{ backgroundColor: '#1e293b', border: '1px solid #334155' }}
                      formatter={(value: number) => [`$${value.toLocaleString()}`, 'Volume']}
                    />
                    <Area type="monotone" dataKey="volume" stroke="#f97316" fill="url(#volumeGradient)" />
                  </AreaChart>
                </ResponsiveContainer>
              </div>

              <div className="glass-card p-6">
                <h3 className="text-lg font-semibold mb-4 flex items-center gap-2">
                  <Zap className="w-5 h-5 text-yellow-400" />
                  Transactions Per Second
                </h3>
                <ResponsiveContainer width="100%" height={300}>
                  <LineChart data={tpsData}>
                    <CartesianGrid strokeDasharray="3 3" stroke="#334155" />
                    <XAxis dataKey="time" stroke="#64748b" />
                    <YAxis stroke="#64748b" />
                    <Tooltip
                      contentStyle={{ backgroundColor: '#1e293b', border: '1px solid #334155' }}
                      formatter={(value: number) => [value, 'TPS']}
                    />
                    <Line type="monotone" dataKey="tps" stroke="#eab308" dot={{ fill: '#eab308' }} />
                  </LineChart>
                </ResponsiveContainer>
              </div>
            </div>

            {/* Recent Alerts */}
            <div className="glass-card p-6">
              <h3 className="text-lg font-semibold mb-4 flex items-center gap-2">
                <AlertTriangle className="w-5 h-5 text-yellow-400" />
                Active Alerts
              </h3>
              <div className="space-y-3">
                {alerts.map((alert) => (
                  <div
                    key={alert.id}
                    className={`flex items-center justify-between p-4 rounded-lg ${
                      alert.severity === 'critical' ? 'bg-red-500/10 border border-red-500/30' :
                      alert.severity === 'warning' ? 'bg-yellow-500/10 border border-yellow-500/30' :
                      'bg-blue-500/10 border border-blue-500/30'
                    }`}
                  >
                    <div className="flex items-center gap-3">
                      {alert.severity === 'critical' ? (
                        <XCircle className="w-5 h-5 text-red-400" />
                      ) : alert.severity === 'warning' ? (
                        <AlertTriangle className="w-5 h-5 text-yellow-400" />
                      ) : (
                        <CheckCircle className="w-5 h-5 text-blue-400" />
                      )}
                      <div>
                        <p className="font-medium">{alert.message}</p>
                        <p className="text-sm text-slate-400">
                          {new Date(alert.timestamp).toLocaleString()}
                        </p>
                      </div>
                    </div>
                    <span className={`text-xs px-2 py-1 rounded ${
                      alert.severity === 'critical' ? 'bg-red-500/20 text-red-400' :
                      alert.severity === 'warning' ? 'bg-yellow-500/20 text-yellow-400' :
                      'bg-blue-500/20 text-blue-400'
                    }`}>
                      {alert.severity.toUpperCase()}
                    </span>
                  </div>
                ))}
              </div>
            </div>
          </>
        )}

        {activeTab === 'nodes' && (
          <div className="glass-card p-6">
            <h3 className="text-lg font-semibold mb-4 flex items-center gap-2">
              <Server className="w-5 h-5 text-orange-400" />
              Node Status
            </h3>
            <div className="overflow-x-auto">
              <table className="w-full">
                <thead>
                  <tr className="border-b border-slate-700">
                    <th className="text-left py-3 px-4 text-slate-400">Node</th>
                    <th className="text-left py-3 px-4 text-slate-400">Status</th>
                    <th className="text-right py-3 px-4 text-slate-400">CPU</th>
                    <th className="text-right py-3 px-4 text-slate-400">Memory</th>
                    <th className="text-right py-3 px-4 text-slate-400">Req/s</th>
                    <th className="text-right py-3 px-4 text-slate-400">Latency</th>
                  </tr>
                </thead>
                <tbody>
                  {nodes.map((node) => (
                    <tr key={node.id} className="border-b border-slate-800 hover:bg-slate-800/50">
                      <td className="py-3 px-4 font-medium">{node.name}</td>
                      <td className="py-3 px-4">
                        <span className={`inline-flex items-center gap-2 px-2 py-1 rounded text-xs ${
                          node.status === 'online' ? 'bg-green-500/20 text-green-400' :
                          node.status === 'degraded' ? 'bg-yellow-500/20 text-yellow-400' :
                          'bg-red-500/20 text-red-400'
                        }`}>
                          <div className={`w-2 h-2 rounded-full ${
                            node.status === 'online' ? 'bg-green-400' :
                            node.status === 'degraded' ? 'bg-yellow-400' :
                            'bg-red-400'
                          }`} />
                          {node.status.toUpperCase()}
                        </span>
                      </td>
                      <td className="text-right py-3 px-4">{node.cpu}%</td>
                      <td className="text-right py-3 px-4">{node.memory}%</td>
                      <td className="text-right py-3 px-4">{node.requestsPerSec.toLocaleString()}</td>
                      <td className="text-right py-3 px-4">{node.latency}ms</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
