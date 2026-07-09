'use client';

import { useState } from 'react';
import { useAccount } from 'wagmi';
import { ConnectButton } from '@rainbow-me/rainbowkit';
import { motion } from 'framer-motion';
import { 
  Vote, 
  TrendingUp,
  Clock,
  CheckCircle,
  XCircle,
  ArrowRight,
  Shield,
  Users,
  Wallet,
  Lock
} from 'lucide-react';
import Link from 'next/link';

const MOCK_PROPOSALS = [
  {
    id: '1',
    title: 'Add Support for New Chains',
    description: 'Proposal to add support for Monad and Berachain networks',
    status: 'active',
    votesFor: 2500000,
    votesAgainst: 500000,
    votesAbstain: 100000,
    endBlock: 18500000,
    proposer: '0x1234...5678',
  },
  {
    id: '2',
    title: 'Reduce Trading Fees',
    description: 'Reduce protocol fees from 0.3% to 0.2% for all swaps',
    status: 'pending',
    votesFor: 0,
    votesAgainst: 0,
    votesAbstain: 0,
    endBlock: 18600000,
    proposer: '0xabcd...efgh',
  },
  {
    id: '3',
    title: 'Add New Reward Pool',
    description: 'Allocate 100,000 TIGER tokens for liquidity mining program',
    status: 'executed',
    votesFor: 3200000,
    votesAgainst: 200000,
    votesAbstain: 50000,
    endBlock: 18200000,
    proposer: '0x9876...5432',
  },
];

const DELEGATES = [
  { name: 'Validator1', address: '0x1234...5678', votes: 500000, proposals: 10 },
  { name: 'DeFi_Dad', address: '0xabcd...efgh', votes: 350000, proposals: 8 },
  { name: 'WhaleWatcher', address: '0x9876...5432', votes: 250000, proposals: 5 },
];

export default function GovernancePage() {
  const { address, isConnected } = useAccount();
  const [activeTab, setActiveTab] = useState<'proposals' | 'delegates' | 'treasury'>('proposals');

  if (!isConnected) {
    return (
      <div className="min-h-screen bg-tiger-dark flex items-center justify-center p-4">
        <div className="text-center">
          <Vote className="w-20 h-20 text-tiger-orange mx-auto mb-6" />
          <h1 className="text-3xl font-bold text-white mb-4">Connect for Governance</h1>
          <p className="text-gray-400 mb-8 max-w-md">
            Connect your wallet to participate in governance proposals and vote on the future of TigerWallet.
          </p>
          <ConnectButton />
        </div>
      </div>
    );
  }

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'active': return 'text-green-400 bg-green-400/10';
      case 'pending': return 'text-yellow-400 bg-yellow-400/10';
      case 'executed': return 'text-blue-400 bg-blue-400/10';
      case 'defeated': return 'text-red-400 bg-red-400/10';
      default: return 'text-gray-400 bg-gray-400/10';
    }
  };

  const getStatusIcon = (status: string) => {
    switch (status) {
      case 'active': return <Clock className="w-4 h-4" />;
      case 'executed': return <CheckCircle className="w-4 h-4" />;
      case 'defeated': return <XCircle className="w-4 h-4" />;
      default: return <Clock className="w-4 h-4" />;
    }
  };

  return (
    <div className="min-h-screen bg-tiger-dark">
      {/* Header */}
      <header className="sticky top-0 z-50 glass-dark border-b border-white/5">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
          <div className="flex items-center justify-between h-16">
            <div className="flex items-center space-x-3">
              <Link href="/" className="flex items-center space-x-3">
                <div className="w-8 h-8 rounded-lg bg-gradient-to-br from-tiger-orange to-tiger-yellow flex items-center justify-center">
                  <Vote className="w-5 h-5 text-black" />
                </div>
                <span className="text-lg font-bold text-white">TigerWallet</span>
              </Link>
            </div>
            <ConnectButton showBalance={false} />
          </div>
        </div>
      </header>

      <div className="max-w-4xl mx-auto px-4 py-12">
        {/* Title */}
        <div className="text-center mb-8">
          <h1 className="text-3xl font-bold text-white mb-2">Governance</h1>
          <p className="text-gray-400">Shape the future of TigerWallet</p>
        </div>

        {/* Stats */}
        <div className="grid grid-cols-3 gap-4 mb-8">
          <div className="p-4 rounded-2xl bg-white/5 border border-white/10 text-center">
            <p className="text-2xl font-bold text-white">3.2M</p>
            <p className="text-sm text-gray-400">TIGER Token</p>
          </div>
          <div className="p-4 rounded-2xl bg-white/5 border border-white/10 text-center">
            <p className="text-2xl font-bold text-white">1,250</p>
            <p className="text-sm text-gray-400">Delegates</p>
          </div>
          <div className="p-4 rounded-2xl bg-white/5 border border-white/10 text-center">
            <p className="text-2xl font-bold text-white">12</p>
            <p className="text-sm text-gray-400">Active Proposals</p>
          </div>
        </div>

        {/* Tabs */}
        <div className="flex items-center space-x-1 mb-6 bg-white/5 rounded-xl p-1">
          {(['proposals', 'delegates', 'treasury'] as const).map((tab) => (
            <button
              key={tab}
              onClick={() => setActiveTab(tab)}
              className={`flex-1 py-2.5 px-4 rounded-lg text-sm font-medium transition-colors ${
                activeTab === tab 
                  ? 'bg-white/10 text-white' 
                  : 'text-gray-400 hover:text-white'
              }`}
            >
              {tab.charAt(0).toUpperCase() + tab.slice(1)}
            </button>
          ))}
        </div>

        {/* Proposals Tab */}
        {activeTab === 'proposals' && (
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            className="space-y-4"
          >
            <div className="flex justify-end mb-4">
              <button className="px-4 py-2 rounded-xl bg-tiger-orange text-white font-medium">
                Create Proposal
              </button>
            </div>

            {MOCK_PROPOSALS.map((proposal) => {
              const totalVotes = proposal.votesFor + proposal.votesAgainst + proposal.votesAbstain;
              const forPercent = totalVotes > 0 ? (proposal.votesFor / totalVotes * 100) : 0;

              return (
                <div 
                  key={proposal.id}
                  className="p-6 rounded-2xl bg-white/5 border border-white/10 hover:border-white/20 transition-colors"
                >
                  <div className="flex items-start justify-between mb-4">
                    <div>
                      <div className="flex items-center space-x-2 mb-2">
                        <span className={`px-2 py-1 rounded-lg text-xs font-medium flex items-center space-x-1 ${getStatusColor(proposal.status)}`}>
                          {getStatusIcon(proposal.status)}
                          <span>{proposal.status}</span>
                        </span>
                      </div>
                      <h3 className="text-lg font-semibold text-white">{proposal.title}</h3>
                      <p className="text-sm text-gray-400 mt-1">{proposal.description}</p>
                    </div>
                    <ArrowRight className="w-5 h-5 text-gray-400" />
                  </div>

                  <div className="space-y-3">
                    <div className="flex items-center justify-between text-sm">
                      <span className="text-gray-400">Votes For</span>
                      <span className="text-green-400 font-medium">
                        {(proposal.votesFor / 1000000).toFixed(2)}M ({forPercent.toFixed(1)}%)
                      </span>
                    </div>
                    <div className="h-2 bg-gray-700 rounded-full overflow-hidden">
                      <div 
                        className="h-full bg-green-500 rounded-full"
                        style={{ width: `${forPercent}%` }}
                      />
                    </div>
                    <div className="flex items-center justify-between text-sm">
                      <span className="text-gray-400">Against</span>
                      <span className="text-red-400 font-medium">
                        {(proposal.votesAgainst / 1000000).toFixed(2)}M ({(100 - forPercent).toFixed(1)}%)
                      </span>
                    </div>
                  </div>

                  <div className="mt-4 pt-4 border-t border-white/10 flex items-center justify-between">
                    <div className="flex items-center space-x-2 text-sm text-gray-400">
                      <Users className="w-4 h-4" />
                      <span>Proposer: {proposal.proposer}</span>
                    </div>
                    <div className="flex items-center space-x-2 text-sm text-gray-400">
                      <Clock className="w-4 h-4" />
                      <span>Ends: Block {proposal.endBlock.toLocaleString()}</span>
                    </div>
                  </div>
                </div>
              );
            })}
          </motion.div>
        )}

        {/* Delegates Tab */}
        {activeTab === 'delegates' && (
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            className="space-y-4"
          >
            {DELEGATES.map((delegate, index) => (
              <div 
                key={index}
                className="p-4 rounded-2xl bg-white/5 border border-white/10 hover:border-white/20 transition-colors flex items-center justify-between"
              >
                <div className="flex items-center space-x-4">
                  <div className="w-12 h-12 rounded-full bg-gradient-to-br from-tiger-orange to-tiger-yellow flex items-center justify-center text-black font-bold">
                    {index + 1}
                  </div>
                  <div>
                    <h3 className="font-semibold text-white">{delegate.name}</h3>
                    <p className="text-sm text-gray-400">{delegate.address}</p>
                  </div>
                </div>
                <div className="text-right">
                  <p className="font-semibold text-white">{(delegate.votes / 1000).toFixed(0)}K votes</p>
                  <p className="text-sm text-gray-400">{delegate.proposals} proposals</p>
                </div>
              </div>
            ))}
          </motion.div>
        )}

        {/* Treasury Tab */}
        {activeTab === 'treasury' && (
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
          >
            <div className="p-6 rounded-2xl bg-white/5 border border-white/10">
              <div className="flex items-center justify-between mb-6">
                <h3 className="text-lg font-semibold text-white">Treasury</h3>
                <Lock className="w-5 h-5 text-gray-400" />
              </div>

              <div className="grid grid-cols-2 gap-4 mb-6">
                <div className="p-4 rounded-xl bg-white/5">
                  <p className="text-sm text-gray-400 mb-1">Total Balance</p>
                  <p className="text-2xl font-bold text-white">$2.5M</p>
                </div>
                <div className="p-4 rounded-xl bg-white/5">
                  <p className="text-sm text-gray-400 mb-1">Monthly Income</p>
                  <p className="text-2xl font-bold text-green-400">$150K</p>
                </div>
              </div>

              <div className="space-y-2">
                <h4 className="text-sm font-medium text-gray-400 mb-3">Assets</h4>
                <div className="flex items-center justify-between p-3 rounded-xl bg-white/5">
                  <div className="flex items-center space-x-3">
                    <div className="w-8 h-8 rounded-full bg-blue-500" />
                    <span className="text-white">TIGER Token</span>
                  </div>
                  <span className="text-white font-medium">$1.5M</span>
                </div>
                <div className="flex items-center justify-between p-3 rounded-xl bg-white/5">
                  <div className="flex items-center space-x-3">
                    <div className="w-8 h-8 rounded-full bg-purple-500" />
                    <span className="text-white">ETH</span>
                  </div>
                  <span className="text-white font-medium">$500K</span>
                </div>
                <div className="flex items-center justify-between p-3 rounded-xl bg-white/5">
                  <div className="flex items-center space-x-3">
                    <div className="w-8 h-8 rounded-full bg-green-500" />
                    <span className="text-white">USDC</span>
                  </div>
                  <span className="text-white font-medium">$500K</span>
                </div>
              </div>
            </div>
          </motion.div>
        )}
      </div>
    </div>
  );
}
