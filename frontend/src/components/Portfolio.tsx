'use client';

/**
 * Portfolio panel.
 *
 * Every figure comes from the API (`/user/portfolio` and `/user/transactions`)
 * or from the connected wallet. Nothing is hardcoded, and a failed request
 * renders an explicit error rather than plausible-looking numbers.
 */

import { useState } from 'react';
import { motion } from 'framer-motion';
import {
  TrendingUp,
  TrendingDown,
  ArrowUpRight,
  ArrowDownRight,
  RefreshCw,
  Eye,
  EyeOff,
  ExternalLink,
} from 'lucide-react';

import { usePortfolio, useTransactions } from '@/hooks/useApi';
import { formatAddress } from '@/lib/wallet';

interface PortfolioProps {
  /** Explorer base URL for the active chain, used to link transactions. */
  explorerURL?: string | null;
}

export function Portfolio({ explorerURL }: PortfolioProps) {
  const [showBalances, setShowBalances] = useState(true);
  const [activeTab, setActiveTab] = useState<'assets' | 'transactions'>('assets');

  const {
    data: portfolio,
    error: portfolioError,
    isPending: portfolioPending,
    refetch: refetchPortfolio,
  } = usePortfolio();

  const {
    data: transactions,
    error: transactionsError,
    isPending: transactionsPending,
  } = useTransactions();

  const formatValue = (value: number | null) => {
    if (!showBalances) return '••••••';
    if (value === null) return '—';
    return `$${value.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`;
  };

  if (portfolioError || transactionsError) {
    const message = (portfolioError ?? transactionsError)?.message ?? 'unknown error';
    return (
      <div className="glass-dark rounded-2xl p-6">
        <h2 className="text-xl font-display font-bold text-white mb-4">Portfolio</h2>
        <p role="alert" className="text-red-400 text-sm">
          Portfolio data is unavailable: {message}
        </p>
      </div>
    );
  }

  const total = portfolio?.totalValueUSD ?? null;

  return (
    <div className="space-y-6">
      <div className="glass-dark rounded-2xl p-6">
        <div className="flex items-center justify-between mb-6">
          <h2 className="text-xl font-display font-bold text-white">Portfolio</h2>
          <div className="flex items-center space-x-2">
            <button
              onClick={() => setShowBalances((value) => !value)}
              className="p-2 text-gray-400 hover:text-white transition-colors"
              title={showBalances ? 'Hide balances' : 'Show balances'}
            >
              {showBalances ? <Eye className="w-4 h-4" /> : <EyeOff className="w-4 h-4" />}
            </button>
            <button
              onClick={() => void refetchPortfolio()}
              className="p-2 text-gray-400 hover:text-white transition-colors"
              title="Refresh"
            >
              <RefreshCw className="w-4 h-4" />
            </button>
          </div>
        </div>

        <div className="mb-6">
          <div className="text-gray-400 text-sm mb-1">Total balance</div>
          <div className="flex items-baseline space-x-3">
            <span className="text-4xl font-display font-bold text-white">
              {portfolioPending ? '…' : formatValue(total)}
            </span>
          </div>
          {portfolio?.updatedAt && (
            <div className="text-gray-500 text-xs mt-2">
              Updated {new Date(portfolio.updatedAt).toLocaleString()}
            </div>
          )}
        </div>

        <div className="flex space-x-2 mb-4">
          <button
            onClick={() => setActiveTab('assets')}
            className={`px-4 py-2 rounded-lg text-sm font-medium transition-colors ${
              activeTab === 'assets'
                ? 'bg-tiger-orange/20 text-tiger-orange'
                : 'text-gray-400 hover:text-white'
            }`}
          >
            Assets
          </button>
          <button
            onClick={() => setActiveTab('transactions')}
            className={`px-4 py-2 rounded-lg text-sm font-medium transition-colors ${
              activeTab === 'transactions'
                ? 'bg-tiger-orange/20 text-tiger-orange'
                : 'text-gray-400 hover:text-white'
            }`}
          >
            Transactions
          </button>
        </div>

        {activeTab === 'assets' ? (
          portfolioPending ? (
            <p className="text-gray-400 text-sm">Loading assets…</p>
          ) : portfolio && portfolio.assets.length > 0 ? (
            <div className="space-y-3">
              {portfolio.assets.map((asset, index) => (
                <motion.div
                  key={`${asset.chainId}-${asset.tokenAddress}`}
                  initial={{ opacity: 0, x: -20 }}
                  animate={{ opacity: 1, x: 0 }}
                  transition={{ delay: Math.min(index, 10) * 0.05 }}
                  className="flex items-center justify-between p-3 bg-white/5 rounded-xl hover:bg-white/10 transition-colors"
                >
                  <div>
                    <div className="text-white font-medium">{asset.symbol}</div>
                    <div className="text-gray-500 text-xs font-mono">
                      {formatAddress(asset.tokenAddress)}
                    </div>
                  </div>
                  <div className="text-right">
                    <div className="text-white font-medium">
                      {showBalances ? asset.balance : '••••••'} {asset.symbol}
                    </div>
                    <div className="text-gray-500 text-xs">{formatValue(asset.valueUSD)}</div>
                  </div>
                </motion.div>
              ))}
            </div>
          ) : (
            <p className="text-gray-400 text-sm">No assets found for this account.</p>
          )
        ) : transactionsPending ? (
          <p className="text-gray-400 text-sm">Loading transactions…</p>
        ) : transactions && transactions.length > 0 ? (
          <div className="space-y-3">
            {transactions.map((transaction, index) => (
              <motion.div
                key={transaction.hash}
                initial={{ opacity: 0, x: -20 }}
                animate={{ opacity: 1, x: 0 }}
                transition={{ delay: Math.min(index, 10) * 0.05 }}
                className="flex items-center justify-between p-3 bg-white/5 rounded-xl"
              >
                <div className="flex items-center space-x-3">
                  <div
                    className={`w-8 h-8 rounded-full flex items-center justify-center ${
                      transaction.type === 'swap'
                        ? 'bg-blue-500/20 text-blue-400'
                        : transaction.type === 'receive'
                          ? 'bg-green-500/20 text-green-400'
                          : 'bg-red-500/20 text-red-400'
                    }`}
                  >
                    {transaction.type === 'swap' ? (
                      <RefreshCw className="w-4 h-4" />
                    ) : transaction.type === 'receive' ? (
                      <ArrowDownRight className="w-4 h-4" />
                    ) : (
                      <ArrowUpRight className="w-4 h-4" />
                    )}
                  </div>
                  <div>
                    <div className="text-white font-medium capitalize">{transaction.type}</div>
                    <div className="text-gray-500 text-xs">
                      {new Date(transaction.createdAt).toLocaleString()} · {transaction.status}
                    </div>
                  </div>
                </div>
                <div className="text-right">
                  <div className="text-white text-sm">
                    {transaction.amount} {transaction.tokenSymbol}
                  </div>
                  {explorerURL && (
                    <a
                      href={`${explorerURL}/tx/${transaction.hash}`}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="text-tiger-orange text-xs flex items-center justify-end space-x-1"
                    >
                      <span>Explorer</span>
                      <ExternalLink className="w-3 h-3" />
                    </a>
                  )}
                </div>
              </motion.div>
            ))}
          </div>
        ) : (
          <p className="text-gray-400 text-sm">No transactions yet.</p>
        )}
      </div>
    </div>
  );
}
