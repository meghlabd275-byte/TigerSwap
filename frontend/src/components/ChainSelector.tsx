'use client';

/**
 * Chain selector.
 *
 * The network list comes from the API. Selecting a network switches the user's
 * wallet to it, so the choice is real rather than a label that only changes
 * locally.
 */

import { useMemo, useState } from 'react';
import { AnimatePresence, motion } from 'framer-motion';
import { Check, ChevronDown, Search } from 'lucide-react';

import { useChains } from '@/hooks/useApi';
import type { Chain } from '@/lib/types';
import { useWalletStore } from '@/store/walletStore';

/** Groups chains by their family, in the order the UI shows them. */
const GROUP_ORDER = ['evm', 'solana', 'cosmos', 'other'] as const;
type Group = (typeof GROUP_ORDER)[number];

function groupOf(chain: Chain): Group {
  const type = chain.chainType.toLowerCase();
  if (type === 'evm') return 'evm';
  if (type === 'solana') return 'solana';
  if (type === 'cosmos') return 'cosmos';
  return 'other';
}

const GROUP_LABELS: Record<Group, string> = {
  evm: 'EVM chains',
  solana: 'Solana',
  cosmos: 'Cosmos',
  other: 'Other',
};

export function ChainSelector() {
  const { data: chains, error, isPending } = useChains();
  const { chainId, setChain } = useWalletStore();

  const [isOpen, setIsOpen] = useState(false);
  const [searchQuery, setSearchQuery] = useState('');

  const currentChain = chains?.find((chain) => chain.id === chainId) ?? null;

  const grouped = useMemo(() => {
    const query = searchQuery.trim().toLowerCase();
    const visible = (chains ?? []).filter(
      (chain) =>
        chain.name.toLowerCase().includes(query) ||
        chain.nativeSymbol.toLowerCase().includes(query),
    );

    const result: Record<Group, Chain[]> = { evm: [], solana: [], cosmos: [], other: [] };
    for (const chain of visible) {
      result[groupOf(chain)].push(chain);
    }
    return result;
  }, [chains, searchQuery]);

  return (
    <div className="relative">
      <button
        onClick={() => setIsOpen((value) => !value)}
        className="flex items-center space-x-3 px-4 py-2.5 bg-white/5 border border-white/10 rounded-xl hover:bg-white/10 transition-colors"
      >
        <div className="text-left">
          <div className="text-white font-medium">
            {isPending ? 'Loading networks…' : (currentChain?.name ?? 'Select network')}
          </div>
          <div className="text-gray-500 text-xs">
            {currentChain ? currentChain.chainType.toUpperCase() : '—'}
          </div>
        </div>
        <ChevronDown
          className={`w-4 h-4 text-gray-400 transition-transform ${isOpen ? 'rotate-180' : ''}`}
        />
      </button>

      <AnimatePresence>
        {isOpen && (
          <motion.div
            initial={{ opacity: 0, y: 10 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: 10 }}
            className="absolute top-full left-0 mt-2 w-80 max-h-96 overflow-hidden bg-tiger-dark border border-white/10 rounded-2xl shadow-2xl z-50"
          >
            <div className="p-3 border-b border-white/10">
              <div className="relative">
                <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400" />
                <input
                  type="text"
                  value={searchQuery}
                  onChange={(event) => setSearchQuery(event.target.value)}
                  placeholder="Search networks…"
                  className="w-full pl-9 pr-4 py-2 bg-white/5 border border-white/10 rounded-lg text-white text-sm placeholder:text-gray-500 focus:outline-none focus:border-tiger-orange/50"
                />
              </div>
            </div>

            <div className="overflow-y-auto max-h-72">
              {error && (
                <p role="alert" className="p-4 text-red-400 text-sm">
                  Network list unavailable: {error.message}
                </p>
              )}

              {GROUP_ORDER.map((group) =>
                grouped[group].length > 0 ? (
                  <div key={group} className="p-2">
                    <div className="text-xs text-gray-500 px-2 py-1 uppercase tracking-wider">
                      {GROUP_LABELS[group]}
                    </div>
                    {grouped[group].map((chain) => (
                      <ChainItem
                        key={chain.id}
                        chain={chain}
                        isSelected={chain.id === chainId}
                        onSelect={() => {
                          void setChain(chain.id);
                          setIsOpen(false);
                          setSearchQuery('');
                        }}
                      />
                    ))}
                  </div>
                ) : null,
              )}

              {!isPending && !error && (chains?.length ?? 0) === 0 && (
                <p className="p-4 text-gray-500 text-sm">No networks available.</p>
              )}
            </div>

            {chains && (
              <div className="p-3 border-t border-white/10 text-center">
                <span className="text-gray-500 text-xs">{chains.length} networks available</span>
              </div>
            )}
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
}

function ChainItem({
  chain,
  isSelected,
  onSelect,
}: {
  chain: Chain;
  isSelected: boolean;
  onSelect: () => void;
}) {
  return (
    <button
      onClick={onSelect}
      className={`w-full flex items-center space-x-3 px-3 py-2 rounded-lg transition-colors ${
        isSelected
          ? 'bg-tiger-orange/20 border border-tiger-orange/30'
          : 'hover:bg-white/5 border border-transparent'
      }`}
    >
      <div className="flex-1 text-left">
        <div className={`font-medium ${isSelected ? 'text-tiger-orange' : 'text-white'}`}>
          {chain.name}
        </div>
        <div className="text-gray-500 text-xs">{chain.nativeSymbol}</div>
      </div>
      {isSelected && <Check className="w-4 h-4 text-tiger-orange" />}
    </button>
  );
}
