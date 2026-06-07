'use client'

import React, { useState, useEffect, useCallback } from 'react'
import { useTheme } from '../components/ThemeProvider'

// Types
interface Token {
  id: string
  symbol: string
  name: string
  address: string
  chainId: number
  decimals: number
  balance: string
  balanceUsd: number
  logo?: string
  isNative: boolean
}

interface Chain {
  id: number
  chainId: number
  name: string
  symbol: string
  type: 'evm' | 'solana' | 'cosmos' | 'aptos' | 'sui' | 'tron' | 'bitcoin'
  rpcUrl: string
  explorerUrl: string
  nativeCurrency: { name: string; symbol: string; decimals: number }
  isActive: boolean
}

interface Transaction {
  id: string
  type: 'send' | 'receive' | 'swap' | 'approve'
  token: string
  amount: string
  hash: string
  status: 'pending' | 'confirmed' | 'failed'
  timestamp: number
  from: string
  to: string
  gasUsed?: string
}

interface WalletState {
  address: string
  privateKey?: string
  seedPhrase?: string
  derivedAddresses: Record<number, string>
}

// Pre-configured chains (20 EVM + 20 Non-EVM)
const SUPPORTED_CHAINS: Chain[] = [
  // EVM Chains
  { id: 1, chainId: 1, name: 'Ethereum', symbol: 'ETH', type: 'evm', rpcUrl: 'https://eth.llamarpc.com', explorerUrl: 'https://etherscan.io', nativeCurrency: { name: 'Ether', symbol: 'ETH', decimals: 18 }, isActive: true },
  { id: 2, chainId: 56, name: 'BNB Chain', symbol: 'BNB', type: 'evm', rpcUrl: 'https://bsc-dataseed.binance.org', explorerUrl: 'https://bscscan.com', nativeCurrency: { name: 'BNB', symbol: 'BNB', decimals: 18 }, isActive: true },
  { id: 3, chainId: 137, name: 'Polygon', symbol: 'MATIC', type: 'evm', rpcUrl: 'https://polygon-rpc.com', explorerUrl: 'https://polygonscan.com', nativeCurrency: { name: 'MATIC', symbol: 'MATIC', decimals: 18 }, isActive: true },
  { id: 4, chainId: 42161, name: 'Arbitrum', symbol: 'ETH', type: 'evm', rpcUrl: 'https://arb1.arbitrum.io/rpc', explorerUrl: 'https://arbiscan.io', nativeCurrency: { name: 'Ether', symbol: 'ETH', decimals: 18 }, isActive: true },
  { id: 5, chainId: 10, name: 'Optimism', symbol: 'ETH', type: 'evm', rpcUrl: 'https://mainnet.optimism.io', explorerUrl: 'https://optimistic.etherscan.io', nativeCurrency: { name: 'Ether', symbol: 'ETH', decimals: 18 }, isActive: true },
  { id: 6, chainId: 8453, name: 'Base', symbol: 'ETH', type: 'evm', rpcUrl: 'https://mainnet.base.org', explorerUrl: 'https://basescan.org', nativeCurrency: { name: 'Ether', symbol: 'ETH', decimals: 18 }, isActive: true },
  { id: 7, chainId: 43114, name: 'Avalanche', symbol: 'AVAX', type: 'evm', rpcUrl: 'https://api.avax.network/ext/bc/C/rpc', explorerUrl: 'https://snowtrace.io', nativeCurrency: { name: 'Avalanche', symbol: 'AVAX', decimals: 18 }, isActive: true },
  { id: 8, chainId: 25, name: 'Cronos', symbol: 'CRO', type: 'evm', rpcUrl: 'https://evm.cronos.org', explorerUrl: 'https://cronoscan.com', nativeCurrency: { name: 'Cronos', symbol: 'CRO', decimals: 18 }, isActive: true },
  { id: 9, chainId: 42220, name: 'Celo', symbol: 'CELO', type: 'evm', rpcUrl: 'https://forno.celo.org', explorerUrl: 'https://explorer.celo.org', nativeCurrency: { name: 'Celo', symbol: 'CELO', decimals: 18 }, isActive: true },
  { id: 10, chainId: 1666600000, name: 'Harmony', symbol: 'ONE', type: 'evm', rpcUrl: 'https://api.harmony.one', explorerUrl: 'https://explorer.harmony.one', nativeCurrency: { name: 'ONE', symbol: 'ONE', decimals: 18 }, isActive: true },
  { id: 11, chainId: 128, name: 'HECO', symbol: 'HT', type: 'evm', rpcUrl: 'https://http-mainnet.hecochain.com', explorerUrl: 'https://hecoinfo.com', nativeCurrency: { name: 'Huobi Token', symbol: 'HT', decimals: 18 }, isActive: true },
  { id: 12, chainId: 8217, name: 'Klaytn', symbol: 'KLAY', type: 'evm', rpcUrl: 'https://klaytn.fandom.finance', explorerUrl: 'https://scope.klaytn.com', nativeCurrency: { name: 'Klaytn', symbol: 'KLAY', decimals: 18 }, isActive: true },
  { id: 13, chainId: 42262, name: 'Oasis', symbol: 'ROSE', type: 'evm', rpcUrl: 'https://emerald.oasis.dev', explorerUrl: 'https://explorer.oasis.updev.si', nativeCurrency: { name: 'Oasis', symbol: 'ROSE', decimals: 18 }, isActive: true },
  { id: 14, chainId: 4689, name: 'IOTA', symbol: 'IOTA', type: 'evm', rpcUrl: 'https://evm.fiota.io', explorerUrl: 'https://evm.fiota.io/explorer', nativeCurrency: { name: 'IOTA', symbol: 'IOTA', decimals: 18 }, isActive: true },
  { id: 15, chainId: 1313161554, name: 'Aurora', symbol: 'ETH', type: 'evm', rpcUrl: 'https://mainnet.aurora.dev', explorerUrl: 'https://explorer.aurora.dev', nativeCurrency: { name: 'Ether', symbol: 'ETH', decimals: 18 }, isActive: true },
  { id: 16, chainId: 1088, name: 'Metis', symbol: 'METIS', type: 'evm', rpcUrl: 'https://andromeda.metis.io', explorerUrl: 'https://andromeda-explorer.metis.io', nativeCurrency: { name: 'Metis', symbol: 'METIS', decimals: 18 }, isActive: true },
  { id: 17, chainId: 288, name: 'Boba', symbol: 'ETH', type: 'evm', rpcUrl: 'https://mainnet.boba.network', explorerUrl: 'https://bobascan.com', nativeCurrency: { name: 'Ether', symbol: 'ETH', decimals: 18 }, isActive: true },
  { id: 18, chainId: 106, name: 'Velas', symbol: 'VLX', type: 'evm', rpcUrl: 'https://evm.velas.com/rpc', explorerUrl: 'https://evm.velas.com', nativeCurrency: { name: 'Velas', symbol: 'VLX', decimals: 18 }, isActive: true },
  { id: 19, chainId: 5700, name: 'Meter', symbol: 'MTR', type: 'evm', rpcUrl: 'https://rpc.meter.io', explorerUrl: 'https://scan.meter.io', nativeCurrency: { name: 'Meter', symbol: 'MTR', decimals: 18 }, isActive: true },
  { id: 20, chainId: 2001, name: 'Milkomeda', symbol: 'ADA', type: 'evm', rpcUrl: 'https://rpc.milkomeda.com', explorerUrl: 'https://explorer.milkomeda.com', nativeCurrency: { name: 'Milk-ADA', symbol: 'mADA', decimals: 18 }, isActive: true },
  // Non-EVM Chains
  { id: 21, chainId: 0, name: 'Solana', symbol: 'SOL', type: 'solana', rpcUrl: 'https://api.mainnet-beta.solana.com', explorerUrl: 'https://solscan.io', nativeCurrency: { name: 'Solana', symbol: 'SOL', decimals: 9 }, isActive: true },
  { id: 22, chainId: 100, name: 'Cosmos', symbol: 'ATOM', type: 'cosmos', rpcUrl: 'https://cosmos-rpc.polkachu.com', explorerUrl: 'https://mintscan.io/cosmos', nativeCurrency: { name: 'Atom', symbol: 'ATOM', decimals: 6 }, isActive: true },
  { id: 23, chainId: 1100, name: 'Sui', symbol: 'SUI', type: 'sui', rpcUrl: 'https://fullnode.mainnet.sui.io', explorerUrl: 'https://suiexplorer.com', nativeCurrency: { name: 'Sui', symbol: 'SUI', decimals: 9 }, isActive: true },
  { id: 24, chainId: 1101, name: 'Aptos', symbol: 'APT', type: 'aptos', rpcUrl: 'https://fullnode.mainnet.aptoslabs.com', explorerUrl: 'https://aptoscan.com', nativeCurrency: { name: 'Aptos', symbol: 'APT', decimals: 8 }, isActive: true },
  { id: 25, chainId: -13, name: 'Tron', symbol: 'TRX', type: 'tron', rpcUrl: 'https://api.trongrid.io', explorerUrl: 'https://tronscan.org', nativeCurrency: { name: 'Tron', symbol: 'TRX', decimals: 6 }, isActive: true },
  { id: 26, chainId: -3, name: 'Bitcoin', symbol: 'BTC', type: 'bitcoin', rpcUrl: 'https://blockstream.info/api', explorerUrl: 'https://mempool.space', nativeCurrency: { name: 'Bitcoin', symbol: 'BTC', decimals: 8 }, isActive: true },
  { id: 27, chainId: -5, name: 'Litecoin', symbol: 'LTC', type: 'bitcoin', rpcUrl: 'https://litecoin.blockbook.api.vault/graphql', explorerUrl: 'https://blockchair.com/litecoin', nativeCurrency: { name: 'Litecoin', symbol: 'LTC', decimals: 8 }, isActive: true },
  { id: 28, chainId: -14, name: 'Dogecoin', symbol: 'DOGE', type: 'bitcoin', rpcUrl: 'https://dogecoin.phantom.org/api', explorerUrl: 'https://dogechain.info', nativeCurrency: { name: 'Dogecoin', symbol: 'DOGE', decimals: 8 }, isActive: true },
  { id: 29, chainId: -12, name: 'XRP', symbol: 'XRP', type: 'tron', rpcUrl: 'https://xrplcluster.com', explorerUrl: 'https://xrpcharts.ripple.com', nativeCurrency: { name: 'XRP', symbol: 'XRP', decimals: 6 }, isActive: true },
  { id: 30, chainId: 2000, name: 'Near', symbol: 'NEAR', type: 'solana', rpcUrl: 'https://rpc.mainnet.near.org', explorerUrl: 'https://nearblocks.io', nativeCurrency: { name: 'NEAR', symbol: 'NEAR', decimals: 24 }, isActive: true },
  { id: 31, chainId: 3000, name: 'Hedera', symbol: 'HBAR', type: 'solana', rpcUrl: 'https://mainnet-public.mirrornode.hedera.com', explorerUrl: 'https://app.dragonglass.me', nativeCurrency: { name: 'Hedera', symbol: 'HBAR', decimals: 8 }, isActive: true },
  { id: 32, chainId: 4000, name: 'Algorand', symbol: 'ALGO', type: 'solana', rpcUrl: 'https://mainnet-api.algonode.cloud', explorerUrl: 'https://algoexplorer.io', nativeCurrency: { name: 'Algorand', symbol: 'ALGO', decimals: 6 }, isActive: true },
  { id: 33, chainId: 5000, name: 'MultiversX', symbol: 'EGLD', type: 'solana', rpcUrl: 'https://gateway.multiversx.com', explorerUrl: 'https://multiversx.com/explorer', nativeCurrency: { name: 'MultiversX', symbol: 'EGLD', decimals: 18 }, isActive: true },
  { id: 34, chainId: 6000, name: 'Ton', symbol: 'TON', type: 'ton', rpcUrl: 'https://toncenter.com/api/v2/', explorerUrl: 'https://tonscan.org', nativeCurrency: { name: 'Toncoin', symbol: 'TON', decimals: 9 }, isActive: true },
  { id: 35, chainId: 7000, name: 'Kava', symbol: 'KAVA', type: 'cosmos', rpcUrl: 'https://kava-rpc.publicnode.com', explorerUrl: 'https://www.mintscan.io/kava', nativeCurrency: { name: 'Kava', symbol: 'KAVA', decimals: 6 }, isActive: true },
  { id: 36, chainId: 8000, name: 'Celestia', symbol: 'TIA', type: 'cosmos', rpcUrl: 'https://celestia-rpc.publicnode.com', explorerUrl: 'https://mintscan.io/celestia', nativeCurrency: { name: 'Celestia', symbol: 'TIA', decimals: 6 }, isActive: true },
  { id: 37, chainId: 9000, name: 'Injective', symbol: 'INJ', type: 'cosmos', rpcUrl: 'https://injective-rpc.publicnode.com', explorerUrl: 'https://explorer.injective.network', nativeCurrency: { name: 'Injective', symbol: 'INJ', decimals: 18 }, isActive: true },
  { id: 38, chainId: 10000, name: 'Sei', symbol: 'SEI', type: 'cosmos', rpcUrl: 'https://sei-rpc.publicnode.com', explorerUrl: 'https://www.seiscan.app', nativeCurrency: { name: 'Sei', symbol: 'SEI', decimals: 6 }, isActive: true },
  { id: 39, chainId: 11000, name: 'Osmosis', symbol: 'OSMO', type: 'cosmos', rpcUrl: 'https://osmosis-rpc.polkachu.com', explorerUrl: 'https://www.mintscan.io/osmosis', nativeCurrency: { name: 'Osmosis', symbol: 'OSMO', decimals: 6 }, isActive: true },
  { id: 40, chainId: 12000, name: 'Dymension', symbol: 'DYM', type: 'cosmos', rpcUrl: 'https://dymension-rpc.polkachu.com', explorerUrl: 'https://mintscan.io/dymension', nativeCurrency: { name: 'Dymension', symbol: 'DYM', decimals: 18 }, isActive: true },
]

// Top 50 tokens across chains
const TOP_TOKENS: Token[] = [
  { id: '1-eth', symbol: 'ETH', name: 'Ether', address: '', chainId: 1, decimals: 18, balance: '0', balanceUsd: 0, isNative: true },
  { id: '1-usdc', symbol: 'USDC', name: 'USD Coin', address: '0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48', chainId: 1, decimals: 6, balance: '0', balanceUsd: 0, isNative: false },
  { id: '1-usdt', symbol: 'USDT', name: 'Tether USD', address: '0xdac17f958d2ee523a2206206994597c13d831ec7', chainId: 1, decimals: 6, balance: '0', balanceUsd: 0, isNative: false },
  { id: '1-wbtc', symbol: 'WBTC', name: 'Wrapped BTC', address: '0x2260fac5e5542a773aa44fbcfedf7c193bc2c599', chainId: 1, decimals: 8, balance: '0', balanceUsd: 0, isNative: false },
  { id: '1-dai', symbol: 'DAI', name: 'Dai Stablecoin', address: '0x6b175474e89094c44da98b954eedeac495271d0f', chainId: 1, decimals: 18, balance: '0', balanceUsd: 0, isNative: false },
  { id: '1-link', symbol: 'LINK', name: 'Chainlink', address: '0x514910771af9ca656af840dff83e8264ecf986ca', chainId: 1, decimals: 18, balance: '0', balanceUsd: 0, isNative: false },
  { id: '1-uni', symbol: 'UNI', name: 'Uniswap', address: '0x1f9840a85d5af5bf1d1762f925bdaddc4201f984', chainId: 1, decimals: 18, balance: '0', balanceUsd: 0, isNative: false },
  { id: '1-aave', symbol: 'AAVE', name: 'Aave', address: '0x7fc66500c84a76ad7e9c93437bfc5ac33e2ddae9', chainId: 1, decimals: 18, balance: '0', balanceUsd: 0, isNative: false },
  { id: '56-bnb', symbol: 'BNB', name: 'BNB', address: '', chainId: 56, decimals: 18, balance: '0', balanceUsd: 0, isNative: true },
  { id: '56-busd', symbol: 'BUSD', name: 'Binance USD', address: '0xe9e7cea3dedca5984780bafc599bd69add087d56', chainId: 56, decimals: 18, balance: '0', balanceUsd: 0, isNative: false },
  { id: '137-matic', symbol: 'MATIC', name: 'Polygon', address: '', chainId: 137, decimals: 18, balance: '0', balanceUsd: 0, isNative: true },
  { id: '137-usdc', symbol: 'USDC', name: 'USD Coin', address: '0x2791bca1f2de4661ed88a30c99a7a944daa1b79e', chainId: 137, decimals: 6, balance: '0', balanceUsd: 0, isNative: false },
  { id: '42161-arb', symbol: 'ARB', name: 'Arbitrum', address: '', chainId: 42161, decimals: 18, balance: '0', balanceUsd: 0, isNative: true },
  { id: '10-op', symbol: 'OP', name: 'Optimism', address: '', chainId: 10, decimals: 18, balance: '0', balanceUsd: 0, isNative: true },
  { id: '8453-base', symbol: 'ETH', name: 'Ether', address: '', chainId: 8453, decimals: 18, balance: '0', balanceUsd: 0, isNative: true },
  { id: '43114-avax', symbol: 'AVAX', name: 'Avalanche', address: '', chainId: 43114, decimals: 18, balance: '0', balanceUsd: 0, isNative: true },
  { id: '0-sol', symbol: 'SOL', name: 'Solana', address: '', chainId: 0, decimals: 9, balance: '0', balanceUsd: 0, isNative: true },
  { id: '100-atom', symbol: 'ATOM', name: 'Cosmos', address: '', chainId: 100, decimals: 6, balance: '0', balanceUsd: 0, isNative: true },
  { id: '1100-sui', symbol: 'SUI', name: 'Sui', address: '', chainId: 1100, decimals: 9, balance: '0', balanceUsd: 0, isNative: true },
  { id: '1101-apt', symbol: 'APT', name: 'Aptos', address: '', chainId: 1101, decimals: 8, balance: '0', balanceUsd: 0, isNative: true },
  { id: '-13-trx', symbol: 'TRX', name: 'Tron', address: '', chainId: -13, decimals: 6, balance: '0', balanceUsd: 0, isNative: true },
  { id: '-3-btc', symbol: 'BTC', name: 'Bitcoin', address: '', chainId: -3, decimals: 8, balance: '0', balanceUsd: 0, isNative: true },
  { id: '-5-ltc', symbol: 'LTC', name: 'Litecoin', address: '', chainId: -5, decimals: 8, balance: '0', balanceUsd: 0, isNative: true },
  { id: '-14-doge', symbol: 'DOGE', name: 'Dogecoin', address: '', chainId: -14, decimals: 8, balance: '0', balanceUsd: 0, isNative: true },
  { id: '2000-near', symbol: 'NEAR', name: 'NEAR Protocol', address: '', chainId: 2000, decimals: 24, balance: '0', balanceUsd: 0, isNative: true },
  { id: '3000-hbar', symbol: 'HBAR', name: 'Hedera', address: '', chainId: 3000, decimals: 8, balance: '0', balanceUsd: 0, isNative: true },
  { id: '6000-ton', symbol: 'TON', name: 'Toncoin', address: '', chainId: 6000, decimals: 9, balance: '0', balanceUsd: 0, isNative: true },
  { id: '9000-inj', symbol: 'INJ', name: 'Injective', address: '', chainId: 9000, decimals: 18, balance: '0', balanceUsd: 0, isNative: true },
  { id: '10000-sei', symbol: 'SEI', name: 'Sei', address: '', chainId: 10000, decimals: 6, balance: '0', balanceUsd: 0, isNative: true },
]

// Utility functions
function generateMnemonic(): string {
  const words = ['abandon', 'ability', 'able', 'about', 'above', 'absent', 'absorb', 'abstract', 'absurd', 'abuse', 'access', 'accident', 'account', 'accuse', 'achieve', 'acid', 'acoustic', 'acquire', 'across', 'act', 'action', 'actor', 'actress', 'actual', 'adapt', 'add', 'addict', 'address', 'adjust', 'admit', 'adult', 'advance', 'advice', 'aerobic', 'affair', 'afford', 'afraid', 'again', 'age', 'agent', 'agree', 'ahead', 'aim', 'air', 'airport', 'aisle', 'alarm', 'album']
  const randomWords: string[] = []
  for (let i = 0; i < 24; i++) {
    randomWords.push(words[Math.floor(Math.random() * words.length)])
  }
  return randomWords.join(' ')
}

function deriveAddressFromSeed(seed: string, chainId: number, index: number = 0): string {
  // Simplified derivation - in production, use proper HD wallet derivation
  const seedHash = seed.split('').reduce((acc, char, i) => acc + char.charCodeAt(0) * (i + 1), 0)
  const chainOffset = chainId * 1000 + index
  
  if (chainId === -3 || chainId === -5 || chainId === -14) {
    // Bitcoin-like: start with 1 or 3
    return '1' + ((seedHash + chainOffset) % Math.pow(10, 25)).toString(16).padStart(25, '0')
  } else if (chainId === 0) {
    // Solana: base58-ish
    return ((seedHash + chainOffset) % Math.pow(10, 43)).toString(36).padStart(44, 'x')
  } else {
    // EVM: 0x + 40 hex chars
    return '0x' + ((seedHash + chainOffset) % Math.pow(16, 40)).toString(16).padStart(40, '0')
  }
}

function formatAddress(address: string, chars: number = 6): string {
  if (!address || address.length < chars * 2) return address
  return `${address.slice(0, chars)}...${address.slice(-chars)}`
}

function formatBalance(balance: string, decimals: number = 18): string {
  const num = parseFloat(balance)
  if (isNaN(num)) return '0'
  if (num < 0.0001) return '<0.0001'
  if (num < 1) return num.toFixed(4)
  if (num < 1000) return num.toFixed(2)
  return num.toLocaleString('en-US', { maximumFractionDigits: 2 })
}

// Component: Send Modal
function SendModal({ token, chain, onClose, onSend }: { token: Token; chain: Chain; onClose: () => void; onSend: (to: string, amount: string) => void }) {
  const [to, setTo] = useState('')
  const [amount, setAmount] = useState('')
  const [isSending, setIsSending] = useState(false)
  
  const handleSend = async () => {
    if (!to || !amount) return
    setIsSending(true)
    await new Promise(resolve => setTimeout(resolve, 2000))
    onSend(to, amount)
    setIsSending(false)
    onClose()
  }
  
  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50" onClick={onClose}>
      <div className="light:bg-white dark:bg-slate-800 rounded-2xl p-6 w-full max-w-md" onClick={e => e.stopPropagation()}>
        <div className="flex justify-between items-center mb-6">
          <h3 className="text-xl font-bold">Send {token.symbol}</h3>
          <button onClick={onClose} className="text-slate-400 hover:text-white">✕</button>
        </div>
        
        <div className="mb-4">
          <label className="block text-sm text-slate-400 mb-2">Recipient Address</label>
          <input
            type="text"
            value={to}
            onChange={e => setTo(e.target.value)}
            placeholder="0x..."
            className="w-full p-3 rounded-lg bg-slate-900 border border-white/10 focus:border-orange-500 outline-none"
          />
        </div>
        
        <div className="mb-6">
          <label className="block text-sm text-slate-400 mb-2">Amount ({token.symbol})</label>
          <input
            type="number"
            value={amount}
            onChange={e => setAmount(e.target.value)}
            placeholder="0.0"
            className="w-full p-3 rounded-lg bg-slate-900 border border-white/10 focus:border-orange-500 outline-none"
          />
          <p className="text-sm text-slate-400 mt-2">Available: {token.balance} {token.symbol}</p>
        </div>
        
        <button
          onClick={handleSend}
          disabled={!to || !amount || isSending}
          className="w-full py-3 bg-orange-500 text-white rounded-lg font-semibold disabled:opacity-50"
        >
          {isSending ? 'Sending...' : 'Send'}
        </button>
      </div>
    </div>
  )
}

// Component: Receive Modal
function ReceiveModal({ address, chain, onClose }: { address: string; chain: Chain; onClose: () => void }) {
  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50" onClick={onClose}>
      <div className="light:bg-white dark:bg-slate-800 rounded-2xl p-6 w-full max-w-md" onClick={e => e.stopPropagation()}>
        <div className="flex justify-between items-center mb-6">
          <h3 className="text-xl font-bold">Receive {chain.symbol}</h3>
          <button onClick={onClose} className="text-slate-400 hover:text-white">✕</button>
        </div>
        
        <div className="bg-slate-100 dark:bg-slate-900 p-4 rounded-lg mb-4">
          <div className="w-48 h-48 mx-auto bg-white p-2 rounded-lg">
            <div className="w-full h-full bg-slate-200 flex items-center justify-center text-6xl">
              📋
            </div>
          </div>
        </div>
        
        <div className="text-center">
          <p className="text-sm text-slate-400 mb-2">{chain.name} Address</p>
          <p className="font-mono text-sm break-all bg-slate-100 dark:bg-slate-900 p-2 rounded">{address}</p>
        </div>
        
        <button
          onClick={() => navigator.clipboard.writeText(address)}
          className="w-full mt-4 py-3 bg-orange-500 text-white rounded-lg font-semibold"
        >
          Copy Address
        </button>
      </div>
    </div>
  )
}

// Main Wallet Page Component
export default function WalletPage() {
  const { theme } = useTheme()
  const [activeTab, setActiveTab] = useState(0)
  const [wallet, setWallet] = useState<WalletState | null>(null)
  const [selectedChain, setSelectedChain] = useState<Chain>(SUPPORTED_CHAINS[0])
  const [tokens, setTokens] = useState<Token[]>([])
  const [transactions, setTransactions] = useState<Transaction[]>([])
  const [showCreate, setShowCreate] = useState(false)
  const [showImport, setShowImport] = useState(false)
  const [showSend, setShowSend] = useState(false)
  const [showReceive, setShowReceive] = useState(false)
  const [showSwap, setShowSwap] = useState(false)
  const [selectedToken, setSelectedToken] = useState<Token | null>(null)
  const [importPhrase, setImportPhrase] = useState('')
  const [seedPhrase, setSeedPhrase] = useState<string | null>(null)
  
  // Calculate total balance
  const totalBalanceUsd = tokens.reduce((sum, t) => sum + t.balanceUsd, 0)
  
  // Filter tokens by selected chain
  const chainTokens = tokens.filter(t => t.chainId === selectedChain.chainId)
  
  // Create new wallet
  const createWallet = () => {
    const mnemonic = generateMnemonic()
    setSeedPhrase(mnemonic)
    setShowCreate(true)
  }
  
  const confirmCreateWallet = () => {
    if (!seedPhrase) return
    
    const addresses: Record<number, string> = {}
    SUPPORTED_CHAINS.forEach(chain => {
      addresses[chain.chainId] = deriveAddressFromSeed(seedPhrase, chain.chainId)
    })
    
    const primaryAddress = addresses[selectedChain.chainId]
    
    setWallet({
      address: primaryAddress,
      seedPhrase,
      derivedAddresses: addresses
    })
    
    // Initialize tokens with balances
    const updatedTokens = TOP_TOKENS.map(t => ({
      ...t,
      balance: (Math.random() * 10).toFixed(4),
      balanceUsd: Math.random() * 1000
    }))
    setTokens(updatedTokens)
    
    // Sample transactions
    setTransactions([
      { id: '1', type: 'swap', token: 'ETH → USDT', amount: '2.5', hash: '0x123...', status: 'confirmed', timestamp: Date.now() - 3600000, from: '0xabc...', to: '0xdef...' },
      { id: '2', type: 'send', token: 'USDC', amount: '500', hash: '0x456...', status: 'confirmed', timestamp: Date.now() - 7200000, from: '0xabc...', to: '0xghi...' },
      { id: '3', type: 'receive', token: 'ETH', amount: '0.5', hash: '0x789...', status: 'pending', timestamp: Date.now(), from: '0xjkl...', to: primaryAddress },
    ])
    
    setShowCreate(false)
  }
  
  // Import wallet
  const importWallet = () => {
    if (!importPhrase.trim()) return
    
    const addresses: Record<number, string> = {}
    SUPPORTED_CHAINS.forEach(chain => {
      addresses[chain.chainId] = deriveAddressFromSeed(importPhrase, chain.chainId)
    })
    
    const primaryAddress = addresses[selectedChain.chainId]
    
    setWallet({
      address: primaryAddress,
      seedPhrase: importPhrase,
      derivedAddresses: addresses
    })
    
    // Initialize tokens with balances
    const updatedTokens = TOP_TOKENS.map(t => ({
      ...t,
      balance: (Math.random() * 10).toFixed(4),
      balanceUsd: Math.random() * 1000
    }))
    setTokens(updatedTokens)
    
    setShowImport(false)
  }
  
  // Handle send
  const handleSend = (to: string, amount: string) => {
    if (!selectedToken || !wallet) return
    
    const newTx: Transaction = {
      id: Date.now().toString(),
      type: 'send',
      token: selectedToken.symbol,
      amount,
      hash: '0x' + Math.random().toString(16).slice(2, 66),
      status: 'pending',
      timestamp: Date.now(),
      from: wallet.address,
      to
    }
    
    setTransactions(prev => [newTx, ...prev])
    
    // Update token balance
    setTokens(prev => prev.map(t => {
      if (t.id === selectedToken.id) {
        const newBalance = parseFloat(t.balance) - parseFloat(amount)
        return { ...t, balance: newBalance.toString(), balanceUsd: newBalance * (t.balanceUsd / parseFloat(t.balance) || 0) }
      }
      return t
    }))
  }
  
  // Get current address for selected chain
  const currentAddress = wallet?.derivedAddresses[selectedChain.chainId] || wallet?.address || ''
  
  return (
    <div className={`min-h-screen ${theme === 'dark' ? 'bg-slate-900 text-white' : 'bg-slate-100 text-slate-900'}`}>
      {/* Header */}
      <header className={`sticky top-0 z-40 ${theme === 'dark' ? 'bg-slate-900/95 border-b border-white/10' : 'bg-white/95 border-b border-black/10'}`}>
        <div className="max-w-6xl mx-auto px-4 py-4 flex justify-between items-center">
          <div className="flex items-center gap-3">
            <span className="text-3xl">🐯</span>
            <span className="text-xl font-bold gradient-text">TigerWallet</span>
          </div>
          
          <div className="flex items-center gap-3">
            {wallet ? (
              <div className="flex items-center gap-2">
                <span className="text-sm text-slate-400">{formatAddress(currentAddress, 8)}</span>
                <button
                  onClick={() => {
                    setWallet(null)
                    setTokens([])
                    setTransactions([])
                  }}
                  className="text-sm text-orange-500 hover:text-orange-600"
                >
                  Disconnect
                </button>
              </div>
            ) : (
              <div className="flex gap-2">
                <button
                  onClick={() => setShowImport(true)}
                  className={`px-4 py-2 rounded-lg border ${theme === 'dark' ? 'border-white/20 text-white' : 'border-black/20 text-slate-900'} hover:border-orange-500`}
                >
                  Import
                </button>
                <button
                  onClick={createWallet}
                  className="px-4 py-2 bg-orange-500 text-white rounded-lg hover:bg-orange-600"
                >
                  Create
                </button>
              </div>
            )}
          </div>
        </div>
      </header>
      
      {!wallet ? (
        // Not connected state
        <div className="max-w-lg mx-auto px-4 py-20 text-center">
          <div className="text-8xl mb-6">👛</div>
          <h1 className="text-3xl font-bold mb-4">TigerWallet</h1>
          <p className={`mb-8 ${theme === 'dark' ? 'text-slate-400' : 'text-slate-600'}`}>
            Multi-chain HD wallet supporting 40+ blockchains.<br />
            Create or import with your 24-word seed phrase.
          </p>
          
          <div className="space-y-4">
            <button
              onClick={createWallet}
              className="w-full py-4 bg-orange-500 text-white rounded-xl font-semibold text-lg hover:bg-orange-600 transition"
            >
              Create New Wallet
            </button>
            <button
              onClick={() => setShowImport(true)}
              className={`w-full py-4 rounded-xl font-semibold text-lg border ${theme === 'dark' ? 'border-white/20 text-white hover:border-orange-500' : 'border-black/20 text-slate-900 hover:border-orange-500'}`}
            >
              Import with Seed Phrase
            </button>
          </div>
          
          {/* Supported Chains */}
          <div className="mt-12">
            <p className={`text-sm mb-4 ${theme === 'dark' ? 'text-slate-500' : 'text-slate-500'}`}>
              Supported Chains: {SUPPORTED_CHAINS.length}
            </p>
            <div className="flex flex-wrap justify-center gap-2">
              {SUPPORTED_CHAINS.slice(0, 12).map(chain => (
                <span
                  key={chain.id}
                  className={`px-3 py-1 rounded-full text-xs ${theme === 'dark' ? 'bg-slate-800 text-slate-400' : 'bg-slate-200 text-slate-600'}`}
                >
                  {chain.symbol}
                </span>
              ))}
              <span className={`px-3 py-1 rounded-full text-xs ${theme === 'dark' ? 'bg-slate-800 text-slate-400' : 'bg-slate-200 text-slate-600'}`}>
                +{SUPPORTED_CHAINS.length - 12} more
              </span>
            </div>
          </div>
        </div>
      ) : (
        // Wallet connected state
        <div className="max-w-6xl mx-auto px-4 py-6">
          {/* Balance Card */}
          <div className="bg-gradient-to-r from-orange-500 to-orange-600 rounded-2xl p-6 mb-6">
            <p className="text-white/80 text-sm">Total Balance</p>
            <h1 className="text-4xl font-bold text-white my-2">
              ${totalBalanceUsd.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}
            </h1>
            <div className="flex gap-3 mt-4">
              <button
                onClick={() => { setSelectedToken(chainTokens[0] || tokens[0]); setShowSend(true) }}
                className="px-4 py-2 bg-white/20 text-white rounded-lg text-sm hover:bg-white/30"
              >
                Send
              </button>
              <button
                onClick={() => setShowReceive(true)}
                className="px-4 py-2 bg-white/20 text-white rounded-lg text-sm hover:bg-white/30"
              >
                Receive
              </button>
              <button
                onClick={() => setShowSwap(true)}
                className="px-4 py-2 bg-white/20 text-white rounded-lg text-sm hover:bg-white/30"
              >
                Swap
              </button>
            </div>
          </div>
          
          {/* Chain Selector */}
          <div className={`rounded-xl p-4 mb-6 ${theme === 'dark' ? 'bg-slate-800/50' : 'bg-white'}`}>
            <div className="flex gap-2 overflow-x-auto pb-2">
              {SUPPORTED_CHAINS.slice(0, 15).map(chain => (
                <button
                  key={chain.id}
                  onClick={() => setSelectedChain(chain)}
                  className={`px-4 py-2 rounded-full text-sm whitespace-nowrap transition ${
                    selectedChain.id === chain.id
                      ? 'bg-orange-500 text-white'
                      : theme === 'dark' ? 'bg-slate-700 text-slate-300 hover:bg-slate-600' : 'bg-slate-100 text-slate-600 hover:bg-slate-200'
                  }`}
                >
                  {chain.symbol}
                </button>
              ))}
            </div>
          </div>
          
          {/* Quick Actions */}
          <div className="grid grid-cols-4 gap-4 mb-6">
            {[
              { icon: '📤', label: 'Send', action: () => { setSelectedToken(chainTokens[0] || tokens[0]); setShowSend(true) } },
              { icon: '📥', label: 'Receive', action: () => setShowReceive(true) },
              { icon: '🔄', label: 'Swap', action: () => setShowSwap(true) },
              { icon: '🌐', label: 'DApps', action: () => {} },
            ].map((item, i) => (
              <button
                key={i}
                onClick={item.action}
                className={`p-4 rounded-xl text-center ${theme === 'dark' ? 'bg-slate-800/50 hover:bg-slate-700/50' : 'bg-white hover:bg-slate-50'}`}
              >
                <div className="text-2xl mb-1">{item.icon}</div>
                <span className="text-sm">{item.label}</span>
              </button>
            ))}
          </div>
          
          {/* Tabs */}
          <div className={`flex border-b mb-4 ${theme === 'dark' ? 'border-white/10' : 'border-black/10'}`}>
            {['Tokens', 'Activity', 'Swap', 'Earn'].map((tab, i) => (
              <button
                key={tab}
                onClick={() => setActiveTab(i)}
                className={`px-4 py-3 text-sm font-medium ${activeTab === i ? 'text-orange-500 border-b-2 border-orange-500' : theme === 'dark' ? 'text-slate-400' : 'text-slate-600'}`}
              >
                {tab}
              </button>
            ))}
          </div>
          
          {/* Tokens Tab */}
          {activeTab === 0 && (
            <div className={`rounded-xl overflow-hidden ${theme === 'dark' ? 'bg-slate-800/50' : 'bg-white'}`}>
              <div className="p-4 border-b flex justify-between items-center">
                <span className="font-medium">Tokens</span>
                <button className="text-orange-500 text-sm">+ Add Token</button>
              </div>
              {chainTokens.map(token => (
                <div
                  key={token.id}
                  className={`p-4 flex items-center gap-4 ${theme === 'dark' ? 'border-b border-white/5 hover:bg-slate-700/30' : 'border-b border-black/5 hover:bg-slate-50'}`}
                >
                  <div className="w-10 h-10 bg-orange-500 rounded-full flex items-center justify-center text-white font-bold">
                    {token.symbol[0]}
                  </div>
                  <div className="flex-1">
                    <div className="font-medium">{token.symbol}</div>
                    <div className={`text-sm ${theme === 'dark' ? 'text-slate-400' : 'text-slate-500'}`}>{token.name}</div>
                  </div>
                  <div className="text-right">
                    <div className="font-medium">{formatBalance(token.balance)}</div>
                    <div className={`text-sm ${theme === 'dark' ? 'text-slate-400' : 'text-slate-500'}`}>
                      ${token.balanceUsd.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}
                    </div>
                  </div>
                </div>
              ))}
              {chainTokens.length === 0 && (
                <div className="p-8 text-center text-slate-400">
                  No tokens on {selectedChain.name}
                </div>
              )}
            </div>
          )}
          
          {/* Activity Tab */}
          {activeTab === 1 && (
            <div className={`rounded-xl overflow-hidden ${theme === 'dark' ? 'bg-slate-800/50' : 'bg-white'}`}>
              <div className="p-4 border-b">
                <span className="font-medium">Recent Transactions</span>
              </div>
              {transactions.map(tx => (
                <div
                  key={tx.id}
                  className={`p-4 flex items-center gap-4 ${theme === 'dark' ? 'border-b border-white/5' : 'border-b border-black/5'}`}
                >
                  <div className="text-2xl">
                    {tx.type === 'send' ? '📤' : tx.type === 'receive' ? '📥' : '🔄'}
                  </div>
                  <div className="flex-1">
                    <div className="font-medium">{tx.token}</div>
                    <div className={`text-sm ${theme === 'dark' ? 'text-slate-400' : 'text-slate-500'}`}>
                      {new Date(tx.timestamp).toLocaleString()}
                    </div>
                  </div>
                  <div className="text-right">
                    <div className="font-medium">{tx.type === 'send' ? '-' : '+'}{tx.amount}</div>
                    <span className={`text-xs px-2 py-1 rounded ${
                      tx.status === 'confirmed' ? 'bg-green-500/20 text-green-500' :
                      tx.status === 'pending' ? 'bg-yellow-500/20 text-yellow-500' :
                      'bg-red-500/20 text-red-500'
                    }`}>
                      {tx.status}
                    </span>
                  </div>
                </div>
              ))}
              {transactions.length === 0 && (
                <div className="p-8 text-center text-slate-400">
                  No transactions yet
                </div>
              )}
            </div>
          )}
          
          {/* Swap Tab */}
          {activeTab === 2 && (
            <div className={`rounded-xl p-6 ${theme === 'dark' ? 'bg-slate-800/50' : 'bg-white'}`}>
              <h3 className="font-bold mb-4">Swap</h3>
              <div className="space-y-4">
                <div className={`p-4 rounded-lg ${theme === 'dark' ? 'bg-slate-900' : 'bg-slate-50'}`}>
                  <label className="text-sm text-slate-400">You Pay</label>
                  <div className="flex gap-2 mt-2">
                    <input type="number" placeholder="0.0" className="flex-1 bg-transparent outline-none text-xl" />
                    <select className="bg-transparent outline-none">
                      <option>ETH</option>
                      <option>USDC</option>
                      <option>USDT</option>
                    </select>
                  </div>
                </div>
                <div className="text-center text-2xl">↓</div>
                <div className={`p-4 rounded-lg ${theme === 'dark' ? 'bg-slate-900' : 'bg-slate-50'}`}>
                  <label className="text-sm text-slate-400">You Receive</label>
                  <div className="flex gap-2 mt-2">
                    <input type="number" placeholder="0.0" className="flex-1 bg-transparent outline-none text-xl" readOnly />
                    <select className="bg-transparent outline-none">
                      <option>USDC</option>
                      <option>ETH</option>
                      <option>USDT</option>
                    </select>
                  </div>
                </div>
                <button className="w-full py-3 bg-orange-500 text-white rounded-lg font-semibold">
                  Swap
                </button>
              </div>
            </div>
          )}
          
          {/* Earn Tab */}
          {activeTab === 3 && (
            <div className={`rounded-xl p-6 ${theme === 'dark' ? 'bg-slate-800/50' : 'bg-white'}`}>
              <h3 className="font-bold mb-4">Earn</h3>
              <div className="space-y-4">
                {['Staking', 'Liquidity Pools', 'Yield Farming'].map((item, i) => (
                  <div key={i} className={`p-4 rounded-lg ${theme === 'dark' ? 'bg-slate-900' : 'bg-slate-50'}`}>
                    <div className="font-medium">{item}</div>
                    <div className="text-sm text-slate-400 mt-1">APY: {((Math.random() * 10) + 1).toFixed(1)}%</div>
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>
      )}
      
      {/* Create Wallet Modal */}
      {showCreate && seedPhrase && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4">
          <div className={`max-w-md w-full rounded-2xl p-6 ${theme === 'dark' ? 'bg-slate-800' : 'bg-white'}`}>
            <h3 className="text-xl font-bold mb-4">Your Seed Phrase</h3>
            <p className={`text-sm mb-4 ${theme === 'dark' ? 'text-yellow-400' : 'text-yellow-600'}`}>
              ⚠️ Write down these 24 words and keep them safe. This is the ONLY way to recover your wallet.
            </p>
            <div className={`p-4 rounded-lg font-mono text-sm mb-4 ${theme === 'dark' ? 'bg-slate-900' : 'bg-slate-100'}`}>
              {seedPhrase}
            </div>
            <label className="flex items-center gap-2 mb-4">
              <input type="checkbox" className="w-4 h-4" />
              <span className="text-sm">I have saved my seed phrase securely</span>
            </label>
            <button
              onClick={confirmCreateWallet}
              className="w-full py-3 bg-orange-500 text-white rounded-lg font-semibold"
            >
              Continue to Wallet
            </button>
          </div>
        </div>
      )}
      
      {/* Import Wallet Modal */}
      {showImport && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4" onClick={() => setShowImport(false)}>
          <div className={`max-w-md w-full rounded-2xl p-6 ${theme === 'dark' ? 'bg-slate-800' : 'bg-white'}`} onClick={e => e.stopPropagation()}>
            <h3 className="text-xl font-bold mb-4">Import Wallet</h3>
            <p className={`text-sm mb-4 ${theme === 'dark' ? 'text-slate-400' : 'text-slate-600'}`}>
              Enter your 24-word seed phrase to restore your wallet.
            </p>
            <textarea
              value={importPhrase}
              onChange={e => setImportPhrase(e.target.value)}
              placeholder="word1 word2 word3 ... word24"
              rows={4}
              className={`w-full p-3 rounded-lg border mb-4 ${theme === 'dark' ? 'bg-slate-900 border-white/10' : 'bg-slate-50 border-black/10'}`}
            />
            <div className="flex gap-3">
              <button
                onClick={() => setShowImport(false)}
                className="flex-1 py-3 rounded-lg border hover:bg-slate-100 dark:hover:bg-slate-700"
              >
                Cancel
              </button>
              <button
                onClick={importWallet}
                disabled={!importPhrase.trim()}
                className="flex-1 py-3 bg-orange-500 text-white rounded-lg font-semibold disabled:opacity-50"
              >
                Import
              </button>
            </div>
          </div>
        </div>
      )}
      
      {/* Send Modal */}
      {showSend && selectedToken && (
        <SendModal
          token={selectedToken}
          chain={selectedChain}
          onClose={() => { setShowSend(false); setSelectedToken(null) }}
          onSend={handleSend}
        />
      )}
      
      {/* Receive Modal */}
      {showReceive && (
        <ReceiveModal
          address={currentAddress}
          chain={selectedChain}
          onClose={() => setShowReceive(false)}
        />
      )}
    </div>
  )
}