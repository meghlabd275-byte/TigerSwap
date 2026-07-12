'use client';

import { useState } from 'react';
import { motion } from 'framer-motion';
import { Wallet, Menu, X, ChevronDown, Rocket, Zap, Shield, Globe } from 'lucide-react';
import { useStore } from '@/store/useStore';
import Link from 'next/link';

export function Header() {
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false);
  const [walletDropdownOpen, setWalletDropdownOpen] = useState(false);
  const { connected, address, connectWallet, disconnectWallet } = useStore();

  const formatAddress = (addr: string) => {
    return `${addr.slice(0, 6)}...${addr.slice(-4)}`;
  };

  return (
    <header className="fixed top-0 left-0 right-0 z-50 bg-tiger-dark/80 backdrop-blur-xl border-b border-white/5">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
        <div className="flex items-center justify-between h-16">
          {/* Logo */}
          <div className="flex items-center space-x-3">
            <div className="w-10 h-10 rounded-xl bg-gradient-to-br from-tiger-orange to-tiger-accent flex items-center justify-center">
              <span className="text-2xl">🐯</span>
            </div>
            <div>
              <h1 className="text-xl font-display font-bold text-white">
                Tiger<span className="text-tiger-orange">Swap</span>
              </h1>
              <p className="text-xs text-gray-500">Multi-Chain DEX</p>
            </div>
          </div>

          {/* Desktop Navigation */}
          <nav className="hidden md:flex items-center space-x-1">
            <NavLink href="#swap" active>Swap</NavLink>
            <NavLink href="#liquidity">Liquidity</NavLink>
            <NavLink href="#perpetuals">Perpetuals</NavLink>
            <NavLink href="#bridge">Bridge</NavLink>
            <NavLink href="#staking">Staking</NavLink>
            <Link href="/wallet" className="px-3 py-2 text-sm font-medium rounded-lg transition-colors text-gray-400 hover:text-white hover:bg-white/5">
              Wallet
            </Link>
            <Link href="/admin" className="px-3 py-2 text-sm font-medium rounded-lg transition-colors text-gray-400 hover:text-white hover:bg-white/5">
              Admin
            </Link>
            
            {/* Features Dropdown */}
            <div className="relative group">
              <button className="flex items-center space-x-1 px-3 py-2 text-gray-400 hover:text-white transition-colors">
                <span>Features</span>
                <ChevronDown className="w-4 h-4" />
              </button>
              <div className="absolute top-full left-0 mt-2 w-64 py-2 bg-tiger-dark border border-white/10 rounded-xl opacity-0 invisible group-hover:opacity-100 group-hover:visible transition-all shadow-xl">
                <DropdownItem icon={Zap} title="DCA Bot" desc="Dollar-cost averaging" />
                <DropdownItem icon={Globe} title="Cross-Chain" desc="Bridge across 100+ chains" />
                <DropdownItem icon={Shield} title="Copy Trading" desc="Follow top traders" />
                <DropdownItem icon={Rocket} title="Launchpad" desc="New token launches" />
              </div>
            </div>
          </nav>

          {/* Right Side */}
          <div className="flex items-center space-x-3">
            {/* Gas Optimizer */}
            <button className="hidden lg:flex items-center space-x-2 px-3 py-1.5 bg-green-500/10 border border-green-500/20 rounded-lg">
              <Zap className="w-4 h-4 text-green-400" />
              <span className="text-green-400 text-sm font-medium">Gas: 2.5 Gwei</span>
            </button>

            {/* Settings */}
            <button className="p-2 text-gray-400 hover:text-white transition-colors">
              <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
              </svg>
            </button>

            {/* Network Selector */}
            <button className="hidden sm:flex items-center space-x-2 px-3 py-1.5 bg-white/5 border border-white/10 rounded-lg hover:bg-white/10 transition-colors">
              <Globe className="w-4 h-4 text-tiger-orange" />
              <span className="text-white text-sm">100+ Chains</span>
            </button>

            {/* Wallet Button */}
            {connected ? (
              <div className="relative">
                <button
                  onClick={() => setWalletDropdownOpen(!walletDropdownOpen)}
                  className="flex items-center space-x-2 px-4 py-2 bg-tiger-orange/20 border border-tiger-orange/30 rounded-xl hover:bg-tiger-orange/30 transition-colors"
                >
                  <div className="w-2 h-2 bg-green-400 rounded-full animate-pulse" />
                  <span className="text-white font-medium">{formatAddress(address!)}</span>
                  <ChevronDown className="w-4 h-4 text-gray-400" />
                </button>
                
                {walletDropdownOpen && (
                  <div className="absolute right-0 mt-2 w-48 py-2 bg-tiger-dark border border-white/10 rounded-xl shadow-xl">
                    <button className="w-full px-4 py-2 text-left text-gray-400 hover:text-white hover:bg-white/5 transition-colors">
                      My Portfolio
                    </button>
                    <button className="w-full px-4 py-2 text-left text-gray-400 hover:text-white hover:bg-white/5 transition-colors">
                      Transaction History
                    </button>
                    <button className="w-full px-4 py-2 text-left text-gray-400 hover:text-white hover:bg-white/5 transition-colors">
                      Settings
                    </button>
                    <hr className="my-2 border-white/10" />
                    <button 
                      onClick={disconnectWallet}
                      className="w-full px-4 py-2 text-left text-red-400 hover:text-red-300 hover:bg-red-500/10 transition-colors"
                    >
                      Disconnect
                    </button>
                  </div>
                )}
              </div>
            ) : (
              <button
                onClick={connectWallet}
                className="flex items-center space-x-2 px-4 py-2 bg-gradient-to-r from-tiger-orange to-tiger-accent rounded-xl text-white font-semibold btn-glow hover:opacity-90 transition-opacity"
              >
                <Wallet className="w-4 h-4" />
                <span>Connect</span>
              </button>
            )}

            {/* Mobile Menu Toggle */}
            <button
              onClick={() => setMobileMenuOpen(!mobileMenuOpen)}
              className="md:hidden p-2 text-gray-400 hover:text-white"
            >
              {mobileMenuOpen ? <X className="w-6 h-6" /> : <Menu className="w-6 h-6" />}
            </button>
          </div>
        </div>
      </div>

      {/* Mobile Menu */}
      {mobileMenuOpen && (
        <motion.div
          initial={{ opacity: 0, height: 0 }}
          animate={{ opacity: 1, height: 'auto' }}
          className="md:hidden bg-tiger-dark border-b border-white/5"
        >
          <div className="px-4 py-4 space-y-2">
            <MobileNavLink href="#swap">Swap</MobileNavLink>
            <MobileNavLink href="#liquidity">Liquidity</MobileNavLink>
            <MobileNavLink href="#perpetuals">Perpetuals</MobileNavLink>
            <MobileNavLink href="#bridge">Bridge</MobileNavLink>
            <MobileNavLink href="#staking">Staking</MobileNavLink>
            <MobileNavLink href="#dca">DCA Bot</MobileNavLink>
            <MobileNavLink href="#copy">Copy Trading</MobileNavLink>
          </div>
        </motion.div>
      )}
    </header>
  );
}

function NavLink({ href, children, active }: { href: string; children: React.ReactNode; active?: boolean }) {
  return (
    <a
      href={href}
      className={`px-3 py-2 text-sm font-medium rounded-lg transition-colors ${
        active
          ? 'text-tiger-orange bg-tiger-orange/10'
          : 'text-gray-400 hover:text-white hover:bg-white/5'
      }`}
    >
      {children}
    </a>
  );
}

function MobileNavLink({ href, children }: { href: string; children: React.ReactNode }) {
  return (
    <a
      href={href}
      className="block px-4 py-2 text-gray-400 hover:text-white hover:bg-white/5 rounded-lg transition-colors"
    >
      {children}
    </a>
  );
}

function DropdownItem({ icon: Icon, title, desc }: { icon: any; title: string; desc: string }) {
  return (
    <button className="w-full flex items-center space-x-3 px-4 py-2 text-left hover:bg-white/5 transition-colors">
      <div className="w-8 h-8 rounded-lg bg-tiger-orange/20 flex items-center justify-center">
        <Icon className="w-4 h-4 text-tiger-orange" />
      </div>
      <div>
        <div className="text-white font-medium text-sm">{title}</div>
        <div className="text-gray-500 text-xs">{desc}</div>
      </div>
    </button>
  );
}
