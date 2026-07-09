'use client';

import { useState } from 'react';
import { useAccount, useConnect, useDisconnect, useBalance } from 'wagmi';
import { ConnectButton } from '@rainbow-me/rainbowkit';
import { motion } from 'framer-motion';
import { 
  Wallet, 
  TrendingUp, 
  Shield, 
  Zap, 
  Globe, 
  Layers,
  ChevronRight,
  WalletCards,
  ArrowRightLeft,
  Crown,
  Smartphone,
  KeyRound,
  Fingerprint,
  Users,
  Bell,
  Settings,
  CreditCard,
  Store,
  Image,
  Lock,
  Bot,
  Vote,
  Activity,
  SmartphoneNfc,
  AlertTriangle,
  Scan,
  RefreshCw,
  LayoutGrid,
  Coins,
  Hexagon
} from 'lucide-react';
import Link from 'next/link';

const features = [
  {
    icon: WalletCards,
    title: 'Smart Accounts (ERC-4337)',
    description: 'Social login, gasless transactions, session keys, and key rotation',
    href: '/wallet/smart-account',
    color: 'from-orange-500 to-yellow-500',
  },
  {
    icon: KeyRound,
    title: 'MPC Wallet',
    description: 'Multi-party computation with distributed key shares',
    href: '/wallet/mpc',
    color: 'from-purple-500 to-pink-500',
  },
  {
    icon: Users,
    title: 'Social Recovery',
    description: 'Guardian-based recovery with time-lock protection',
    href: '/wallet/social-recovery',
    color: 'from-green-500 to-emerald-500',
  },
  {
    icon: Smartphone,
    title: 'Mobile App',
    description: 'iOS and Android with biometric authentication',
    href: '/mobile',
    color: 'from-blue-500 to-cyan-500',
  },
  {
    icon: CreditCard,
    title: 'Fiat On-Ramp',
    description: 'Apple Pay, Google Pay, and 50+ payment methods',
    href: '/fiat',
    color: 'from-indigo-500 to-purple-500',
  },
  {
    icon: Bell,
    title: 'Push Notifications',
    description: 'Price alerts, transactions, and airdrop alerts',
    href: '/notifications',
    color: 'from-red-500 to-orange-500',
  },
  {
    icon: Layers,
    title: 'Multi-Chain',
    description: '100+ chains including EVM, Solana, Bitcoin, and more',
    href: '/chains',
    color: 'from-cyan-500 to-blue-500',
  },
  {
    icon: ArrowRightLeft,
    title: 'Cross-Chain Bridge',
    description: 'Intent-based routing with best price execution',
    href: '/bridge',
    color: 'from-yellow-500 to-green-500',
  },
  {
    icon: Store,
    title: 'DApp Store',
    description: 'Curated DApps with revenue share for users',
    href: '/dapps',
    color: 'from-pink-500 to-rose-500',
  },
  {
    icon: Image,
    title: 'NFT Marketplace',
    description: 'Trade NFTs across OpenSea, Magic Eden, and more',
    href: '/nft',
    color: 'from-violet-500 to-purple-500',
  },
  {
    icon: Lock,
    title: 'Privacy Mode',
    description: 'Privacy pools, stealth addresses, and mixers',
    href: '/privacy',
    color: 'from-slate-500 to-zinc-500',
  },
  {
    icon: Zap,
    title: 'MEV Protection',
    description: 'Flashbots Protect, sandwich detection, and private pools',
    href: '/mev',
    color: 'from-amber-500 to-yellow-500',
  },
  {
    icon: Bot,
    title: 'AI Features',
    description: 'Smart money tracking, gas prediction, and trading signals',
    href: '/ai',
    color: 'from-emerald-500 to-teal-500',
  },
  {
    icon: Vote,
    title: 'Governance',
    description: 'DAO proposals, voting, and treasury management',
    href: '/governance',
    color: 'from-blue-500 to-indigo-500',
  },
  {
    icon: SmartphoneNfc,
    title: 'Hardware Wallet',
    description: 'Ledger, Trezor, Keystone, and air-gapped signing',
    href: '/hardware',
    color: 'from-zinc-500 to-neutral-500',
  },
  {
    icon: Scan,
    title: 'Security Scanner',
    description: 'Honeypot detection, vulnerability scanning, and simulation',
    href: '/security',
    color: 'from-red-500 to-pink-500',
  },
  {
    icon: RefreshCw,
    title: 'Multi-Device Sync',
    description: 'Real-time encrypted sync across all your devices',
    href: '/sync',
    color: 'from-teal-500 to-cyan-500',
  },
  {
    icon: LayoutGrid,
    title: 'Embedded Wallet',
    description: 'React SDK for dApps, games, and platforms',
    href: '/embedded',
    color: 'from-orange-500 to-red-500',
  },
];

const stats = [
  { label: 'Supported Chains', value: '100+' },
  { label: 'Total Value Secured', value: '$2.5B+' },
  { label: 'Active Users', value: '500K+' },
  { label: 'Daily Volume', value: '$150M+' },
];

export default function Home() {
  const { isConnected, address } = useAccount();
  const { connect, connectors } = useConnect();
  const { disconnect } = useDisconnect();
  const { data: balance } = useBalance({
    address,
  });

  return (
    <div className="min-h-screen bg-tiger-dark">
      {/* Header */}
      <header className="fixed top-0 left-0 right-0 z-50 glass-dark border-b border-white/5">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
          <div className="flex items-center justify-between h-16">
            <div className="flex items-center space-x-3">
              <div className="w-10 h-10 rounded-xl bg-gradient-to-br from-tiger-orange to-tiger-yellow flex items-center justify-center">
                <Hexagon className="w-6 h-6 text-black" />
              </div>
              <span className="text-xl font-bold text-white">TigerWallet</span>
            </div>
            
            <nav className="hidden md:flex items-center space-x-1">
              <Link href="/wallet" className="px-4 py-2 text-sm text-gray-300 hover:text-white transition-colors">
                Wallet
              </Link>
              <Link href="/swap" className="px-4 py-2 text-sm text-gray-300 hover:text-white transition-colors">
                Swap
              </Link>
              <Link href="/bridge" className="px-4 py-2 text-sm text-gray-300 hover:text-white transition-colors">
                Bridge
              </Link>
              <Link href="/nft" className="px-4 py-2 text-sm text-gray-300 hover:text-white transition-colors">
                NFT
              </Link>
              <Link href="/dapps" className="px-4 py-2 text-sm text-gray-300 hover:text-white transition-colors">
                DApps
              </Link>
            </nav>

            <div className="flex items-center space-x-3">
              <ConnectButton showBalance={false} />
            </div>
          </div>
        </div>
      </header>

      {/* Hero Section */}
      <section className="pt-32 pb-20 px-4 sm:px-6 lg:px-8">
        <div className="max-w-7xl mx-auto">
          <div className="text-center">
            <motion.div
              initial={{ opacity: 0, y: 20 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.5 }}
            >
              <div className="inline-flex items-center px-4 py-2 rounded-full bg-orange-500/10 border border-orange-500/20 text-orange-400 text-sm mb-8">
                <Zap className="w-4 h-4 mr-2" />
                Next Generation Web3 Wallet
              </div>
            </motion.div>
            
            <motion.h1 
              initial={{ opacity: 0, y: 20 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.5, delay: 0.1 }}
              className="text-5xl md:text-7xl font-bold text-white mb-6"
            >
              One Wallet.
              <span className="bg-gradient-to-r from-tiger-orange to-tiger-yellow bg-clip-text text-transparent">
                {' '}Every Chain.
              </span>
            </motion.h1>
            
            <motion.p 
              initial={{ opacity: 0, y: 20 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.5, delay: 0.2 }}
              className="text-xl text-gray-400 max-w-2xl mx-auto mb-10"
            >
              The most advanced decentralized wallet with ERC-4337 smart accounts, MPC security, 
              social recovery, and seamless multi-chain experience.
            </motion.p>

            <motion.div
              initial={{ opacity: 0, y: 20 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.5, delay: 0.3 }}
              className="flex flex-col sm:flex-row items-center justify-center gap-4"
            >
              <ConnectButton />
              <Link 
                href="/docs" 
                className="px-6 py-3 rounded-xl border border-white/10 text-white hover:bg-white/5 transition-all flex items-center"
              >
                View Documentation
                <ChevronRight className="w-4 h-4 ml-2" />
              </Link>
            </motion.div>
          </div>

          {/* Stats */}
          <motion.div 
            initial={{ opacity: 0, y: 40 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.5, delay: 0.4 }}
            className="mt-20 grid grid-cols-2 md:grid-cols-4 gap-8"
          >
            {stats.map((stat, index) => (
              <div key={index} className="text-center">
                <div className="text-3xl md:text-4xl font-bold text-white mb-2">{stat.value}</div>
                <div className="text-sm text-gray-500">{stat.label}</div>
              </div>
            ))}
          </motion.div>
        </div>
      </section>

      {/* Features Grid */}
      <section className="py-20 px-4 sm:px-6 lg:px-8">
        <div className="max-w-7xl mx-auto">
          <div className="text-center mb-16">
            <h2 className="text-3xl md:text-4xl font-bold text-white mb-4">
              Everything You Need
            </h2>
            <p className="text-gray-400 max-w-2xl mx-auto">
              A complete Web3 ecosystem with enterprise-grade security and unmatched user experience
            </p>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
            {features.map((feature, index) => (
              <motion.div
                key={index}
                initial={{ opacity: 0, y: 20 }}
                animate={{ opacity: 1, y: 0 }}
                transition={{ duration: 0.3, delay: index * 0.05 }}
              >
                <Link href={feature.href}>
                  <div className="group relative p-6 rounded-2xl bg-white/5 border border-white/5 hover:border-white/10 transition-all hover:bg-white/10">
                    <div className={`absolute inset-0 rounded-2xl bg-gradient-to-br ${feature.color} opacity-0 group-hover:opacity-10 transition-opacity`} />
                    <div className="relative">
                      <div className={`w-12 h-12 rounded-xl bg-gradient-to-br ${feature.color} flex items-center justify-center mb-4`}>
                        <feature.icon className="w-6 h-6 text-white" />
                      </div>
                      <h3 className="text-lg font-semibold text-white mb-2 group-hover:text-tiger-orange transition-colors">
                        {feature.title}
                      </h3>
                      <p className="text-sm text-gray-400">
                        {feature.description}
                      </p>
                    </div>
                  </div>
                </Link>
              </motion.div>
            ))}
          </div>
        </div>
      </section>

      {/* CTA Section */}
      <section className="py-20 px-4 sm:px-6 lg:px-8">
        <div className="max-w-4xl mx-auto">
          <div className="relative rounded-3xl overflow-hidden">
            <div className="absolute inset-0 bg-gradient-to-r from-tiger-orange to-tiger-yellow opacity-20" />
            <div className="absolute inset-0 bg-[url('/grid.svg')] opacity-10" />
            <div className="relative p-12 text-center">
              <Crown className="w-16 h-16 text-tiger-orange mx-auto mb-6" />
              <h2 className="text-3xl md:text-4xl font-bold text-white mb-4">
                Ready to experience the future?
              </h2>
              <p className="text-gray-300 mb-8 max-w-xl mx-auto">
                Join 500,000+ users who trust TigerWallet with their digital assets. 
                Start with a new wallet or import your existing one.
              </p>
              <div className="flex flex-col sm:flex-row items-center justify-center gap-4">
                <ConnectButton />
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* Footer */}
      <footer className="py-12 px-4 sm:px-6 lg:px-8 border-t border-white/5">
        <div className="max-w-7xl mx-auto">
          <div className="flex flex-col md:flex-row items-center justify-between">
            <div className="flex items-center space-x-3 mb-4 md:mb-0">
              <div className="w-8 h-8 rounded-lg bg-gradient-to-br from-tiger-orange to-tiger-yellow flex items-center justify-center">
                <Hexagon className="w-5 h-5 text-black" />
              </div>
              <span className="text-lg font-bold text-white">TigerWallet</span>
            </div>
            <div className="flex items-center space-x-6 text-sm text-gray-500">
              <Link href="/docs" className="hover:text-white transition-colors">Documentation</Link>
              <Link href="/terms" className="hover:text-white transition-colors">Terms</Link>
              <Link href="/privacy" className="hover:text-white transition-colors">Privacy</Link>
              <a href="https://github.com" className="hover:text-white transition-colors">GitHub</a>
            </div>
          </div>
          <div className="mt-8 text-center text-sm text-gray-600">
            © 2026 TigerWallet. All rights reserved.
          </div>
        </div>
      </footer>
    </div>
  );
}
