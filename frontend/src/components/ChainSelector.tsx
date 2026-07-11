'use client';

import { useState } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { ChevronDown, Search, Check, Globe } from 'lucide-react';
import { SUPPORTED_CHAINS, Chain } from '@/store/useStore';

interface ChainSelectorProps {
  selectedChain: string;
  onSelectChain: (chain: string) => void;
}

export function ChainSelector({ selectedChain, onSelectChain }: ChainSelectorProps) {
  const [isOpen, setIsOpen] = useState(false);
  const [searchQuery, setSearchQuery] = useState('');

  const chainKeys = Object.keys(SUPPORTED_CHAINS);
  const filteredChains = chainKeys.filter(key => 
    SUPPORTED_CHAINS[key].name.toLowerCase().includes(searchQuery.toLowerCase()) ||
    SUPPORTED_CHAINS[key].symbol.toLowerCase().includes(searchQuery.toLowerCase())
  );

  const currentChain = SUPPORTED_CHAINS[selectedChain];

  // Group chains by type
  const evmChains = filteredChains.filter(key => SUPPORTED_CHAINS[key].type === 'evm');
  const solanaChains = filteredChains.filter(key => SUPPORTED_CHAINS[key].type === 'solana');
  const cosmosChains = filteredChains.filter(key => SUPPORTED_CHAINS[key].type === 'cosmos');
  const otherChains = filteredChains.filter(key => 
    SUPPORTED_CHAINS[key].type !== 'evm' && 
    SUPPORTED_CHAINS[key].type !== 'solana' && 
    SUPPORTED_CHAINS[key].type !== 'cosmos'
  );

  return (
    <div className="relative">
      <button
        onClick={() => setIsOpen(!isOpen)}
        className="flex items-center space-x-3 px-4 py-2.5 bg-white/5 border border-white/10 rounded-xl hover:bg-white/10 transition-colors"
      >
        <div className="text-2xl">{currentChain?.icon || '🌐'}</div>
        <div className="text-left">
          <div className="text-white font-medium">{currentChain?.name || 'Select Chain'}</div>
          <div className="text-gray-500 text-xs">{currentChain?.type.toUpperCase()}</div>
        </div>
        <ChevronDown className={`w-4 h-4 text-gray-400 transition-transform ${isOpen ? 'rotate-180' : ''}`} />
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
                  onChange={(e) => setSearchQuery(e.target.value)}
                  placeholder="Search chains..."
                  className="w-full pl-9 pr-4 py-2 bg-white/5 border border-white/10 rounded-lg text-white text-sm placeholder:text-gray-500 focus:outline-none focus:border-tiger-orange/50"
                />
              </div>
            </div>
            
            <div className="overflow-y-auto max-h-72">
              {/* EVM Chains */}
              {evmChains.length > 0 && (
                <div className="p-2">
                  <div className="text-xs text-gray-500 px-2 py-1 uppercase tracking-wider">EVM Chains</div>
                  {evmChains.map(key => (
                    <ChainItem
                      key={key}
                      chain={SUPPORTED_CHAINS[key]}
                      isSelected={selectedChain === key}
                      onSelect={() => {
                        onSelectChain(key);
                        setIsOpen(false);
                        setSearchQuery('');
                      }}
                    />
                  ))}
                </div>
              )}

              {/* Solana */}
              {solanaChains.length > 0 && (
                <div className="p-2">
                  <div className="text-xs text-gray-500 px-2 py-1 uppercase tracking-wider">Solana</div>
                  {solanaChains.map(key => (
                    <ChainItem
                      key={key}
                      chain={SUPPORTED_CHAINS[key]}
                      isSelected={selectedChain === key}
                      onSelect={() => {
                        onSelectChain(key);
                        setIsOpen(false);
                        setSearchQuery('');
                      }}
                    />
                  ))}
                </div>
              )}

              {/* Cosmos */}
              {cosmosChains.length > 0 && (
                <div className="p-2">
                  <div className="text-xs text-gray-500 px-2 py-1 uppercase tracking-wider">Cosmos</div>
                  {cosmosChains.map(key => (
                    <ChainItem
                      key={key}
                      chain={SUPPORTED_CHAINS[key]}
                      isSelected={selectedChain === key}
                      onSelect={() => {
                        onSelectChain(key);
                        setIsOpen(false);
                        setSearchQuery('');
                      }}
                    />
                  ))}
                </div>
              )}

              {/* Other */}
              {otherChains.length > 0 && (
                <div className="p-2">
                  <div className="text-xs text-gray-500 px-2 py-1 uppercase tracking-wider">Other</div>
                  {otherChains.map(key => (
                    <ChainItem
                      key={key}
                      chain={SUPPORTED_CHAINS[key]}
                      isSelected={selectedChain === key}
                      onSelect={() => {
                        onSelectChain(key);
                        setIsOpen(false);
                        setSearchQuery('');
                      }}
                    />
                  ))}
                </div>
              )}
            </div>

            <div className="p-3 border-t border-white/10 text-center">
              <span className="text-gray-500 text-xs">100+ chains supported</span>
            </div>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
}

function ChainItem({ 
  chain, 
  isSelected, 
  onSelect 
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
      <span className="text-xl">{chain.icon}</span>
      <div className="flex-1 text-left">
        <div className={`font-medium ${isSelected ? 'text-tiger-orange' : 'text-white'}`}>
          {chain.name}
        </div>
        <div className="text-gray-500 text-xs">{chain.symbol}</div>
      </div>
      {isSelected && <Check className="w-4 h-4 text-tiger-orange" />}
    </button>
  );
}
