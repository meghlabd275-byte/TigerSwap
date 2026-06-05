/**
 * TigerSwap Universal Blockchain Registry
 * Complete dynamic chain management for unlimited EVM and Non-EVM networks
 * Built from scratch - no dependencies on other protocols
 */

// ============================================================================
// Chain Types
// ============================================================================

export type ChainCategory = 'evm' | 'solana' | 'aptos' | 'sui' | 'ton' | 'tron' | 'cosmos' | 'near' | 'algorand' | 'polkadot' | 'cardano' | 'other';

export type ChainStatus = 'active' | 'inactive' | 'deprecated' | 'maintenance';

export type TokenStandard = 'ERC20' | 'SPL' | 'APT' | 'SUI' | 'TRC20' | 'NEP141' | 'other';

export interface ChainConfig {
  id: string;
  name: string;
  symbol: string;
  category: ChainCategory;
  status: ChainStatus;
  chainId: number;
  networkId?: number;
  chainTag?: string;
  rpcUrls: string[];
  explorerUrls: string[];
  iconUrl?: string;
  nativeCurrency: {
    name: string;
    symbol: string;
    decimals: number;
    logoUrl?: string;
  };
  blockTime?: number;
  maxBlockSize?: number;
  gasLimit?: number;
  minGasPrice?: string;
  gasPriceUpdateInterval?: number;
  supportsEIP1559?: boolean;
  supportsFlashbots?: boolean;
  supportsMEV?: boolean;
  supportsMulticall?: boolean;
  supportsBatchCalls?: boolean;
  contractAddressRegex?: string;
  addedBy?: string;
  addedAt?: number;
  updatedAt?: number;
  notes?: string;
  isHealthy?: boolean;
  lastHealthCheck?: number;
  latencyMs?: number;
  rpcPriority?: number;
}

export interface TokenConfig {
  address: string;
  chainId: string;
  symbol: string;
  name: string;
  decimals: number;
  logoURI?: string;
  priceSource?: 'chainlink' | 'uniswap' | 'coingecko' | 'manual';
  priceFeed?: string;
  isNative?: boolean;
  isStable?: boolean;
  isWrapped?: boolean;
  wrappedOf?: string;
  coingeckoId?: string;
  listedAt?: number;
  addedBy?: string;
}

export interface RPCEndpoint {
  url: string;
  chainId: string;
  name: string;
  priority: number;
  isHealthy: boolean;
  latencyMs: number;
  lastCheck: number;
  isWebSocket: boolean;
  rateLimit?: number;
  rateLimitRemaining?: number;
  isBackup: boolean;
}

// ============================================================================
// Universal Chain Registry
// ============================================================================

export class UniversalChainRegistry {
  private chains: Map<string, ChainConfig> = new Map();
  private tokens: Map<string, TokenConfig> = new Map();
  private rpcEndpoints: Map<string, RPCEndpoint[]> = new Map();
  private chainCategories: Map<ChainCategory, Set<string>> = new Map();
  
  constructor() {
    this.initializeDefaultChains();
  }

  // ============================================================================
  // Chain Management (CRUD)
  // ============================================================================

  addChain(chain: ChainConfig): void {
    if (this.chains.has(chain.id)) {
      throw new Error(`Chain ${chain.id} already exists`);
    }
    this.validateChainConfig(chain);
    chain.addedAt = Date.now();
    chain.updatedAt = Date.now();
    this.chains.set(chain.id, chain);
    
    if (!this.chainCategories.has(chain.category)) {
      this.chainCategories.set(chain.category, new Set());
    }
    this.chainCategories.get(chain.category)!.add(chain.id);
    
    if (chain.rpcUrls.length > 0) {
      this.rpcEndpoints.set(chain.id, chain.rpcUrls.map((url, i) => ({
        url,
        chainId: chain.id,
        name: `${chain.name} RPC ${i + 1}`,
        priority: i,
        isHealthy: true,
        latencyMs: 0,
        lastCheck: Date.now(),
        isWebSocket: url.startsWith('ws'),
        isBackup: i > 0,
      })));
    }
    console.log(`Chain ${chain.name} (${chain.id}) added successfully`);
  }

  updateChain(chainId: string, updates: Partial<ChainConfig>): void {
    const chain = this.chains.get(chainId);
    if (!chain) {
      throw new Error(`Chain ${chainId} not found`);
    }
    const updatedChain = { ...chain, ...updates, updatedAt: Date.now() };
    this.validateChainConfig(updatedChain);
    this.chains.set(chainId, updatedChain);
    console.log(`Chain ${chainId} updated successfully`);
  }

  removeChain(chainId: string): void {
    const chain = this.chains.get(chainId);
    if (!chain) {
      throw new Error(`Chain ${chainId} not found`);
    }
    const categorySet = this.chainCategories.get(chain.category);
    if (categorySet) {
      categorySet.delete(chainId);
    }
    this.chains.delete(chainId);
    this.rpcEndpoints.delete(chainId);
    console.log(`Chain ${chainId} removed successfully`);
  }

  getChain(chainId: string): ChainConfig | undefined {
    return this.chains.get(chainId);
  }

  getAllChains(): ChainConfig[] {
    return Array.from(this.chains.values());
  }

  getChainsByCategory(category: ChainCategory): ChainConfig[] {
    const chainIds = this.chainCategories.get(category);
    if (!chainIds) return [];
    return Array.from(chainIds).map(id => this.chains.get(id)).filter(Boolean) as ChainConfig[];
  }

  getChainsByStatus(status: ChainStatus): ChainConfig[] {
    return Array.from(this.chains.values()).filter(c => c.status === status);
  }

  searchChains(query: string): ChainConfig[] {
    const lowerQuery = query.toLowerCase();
    return Array.from(this.chains.values()).filter(
      c => c.name.toLowerCase().includes(lowerQuery) ||
           c.symbol.toLowerCase().includes(lowerQuery) ||
           c.id.toLowerCase().includes(lowerQuery)
    );
  }

  // ============================================================================
  // Token Management
  // ============================================================================

  addToken(token: TokenConfig): void {
    const key = `${token.chainId}:${token.address}`;
    if (this.tokens.has(key)) {
      throw new Error(`Token ${token.symbol} already exists on chain ${token.chainId}`);
    }
    this.tokens.set(key, token);
    console.log(`Token ${token.symbol} added to chain ${token.chainId}`);
  }

  updateToken(chainId: string, address: string, updates: Partial<TokenConfig>): void {
    const key = `${chainId}:${address}`;
    const token = this.tokens.get(key);
    if (!token) {
      throw new Error(`Token not found`);
    }
    this.tokens.set(key, { ...token, ...updates });
  }

  removeToken(chainId: string, address: string): void {
    const key = `${chainId}:${address}`;
    this.tokens.delete(key);
  }

  getToken(chainId: string, address: string): TokenConfig | undefined {
    return this.tokens.get(`${chainId}:${address}`);
  }

  getChainTokens(chainId: string): TokenConfig[] {
    return Array.from(this.tokens.values()).filter(t => t.chainId === chainId);
  }

  // ============================================================================
  // RPC Endpoint Management
  // ============================================================================

  addRPCEndpoint(chainId: string, endpoint: Omit<RPCEndpoint, 'chainId'>): void {
    const endpoints = this.rpcEndpoints.get(chainId) || [];
    if (endpoints.some(e => e.url === endpoint.url)) {
      throw new Error('RPC endpoint already exists');
    }
    endpoints.push({ ...endpoint, chainId });
    this.rpcEndpoints.set(chainId, endpoints);
  }

  removeRPCEndpoint(chainId: string, url: string): void {
    const endpoints = this.rpcEndpoints.get(chainId) || [];
    this.rpcEndpoints.set(chainId, endpoints.filter(e => e.url !== url));
  }

  getBestRPC(chainId: string): string | null {
    const endpoints = this.rpcEndpoints.get(chainId)?.sort((a, b) => {
      if (a.isHealthy !== b.isHealthy) return a.isHealthy ? -1 : 1;
      if (a.priority !== b.priority) return a.priority - b.priority;
      return a.latencyMs - b.latencyMs;
    });
    return endpoints?.[0]?.url || null;
  }

  // ============================================================================
  // Validation
  // ============================================================================

  private validateChainConfig(chain: ChainConfig): void {
    if (!chain.id || chain.id.trim() === '') {
      throw new Error('Chain ID is required');
    }
    if (!chain.name || chain.name.trim() === '') {
      throw new Error('Chain name is required');
    }
    if (!chain.category) {
      throw new Error('Chain category is required');
    }
    if (chain.rpcUrls.length === 0) {
      throw new Error('At least one RPC URL is required');
    }
    for (const url of chain.rpcUrls) {
      try {
        new URL(url);
      } catch {
        throw new Error(`Invalid RPC URL: ${url}`);
      }
    }
    for (const url of chain.explorerUrls) {
      try {
        new URL(url);
      } catch {
        throw new Error(`Invalid explorer URL: ${url}`);
      }
    }
    if (!chain.nativeCurrency.name || !chain.nativeCurrency.symbol) {
      throw new Error('Native currency name and symbol are required');
    }
  }

  // ============================================================================
  // Default Chains - 50+ Chains Supported
  // ============================================================================

  private initializeDefaultChains(): void {
    // =============================================================================
    // EVM CHAINS (30+ chains)
    // =============================================================================
    
    // Ethereum
    this.addChain({ id: 'ethereum', name: 'Ethereum', symbol: 'ETH', category: 'evm', status: 'active', chainId: 1, networkId: 1, rpcUrls: ['https://eth.llamarpc.com', 'https://rpc.ankr.com/eth', 'https://cloudflare-eth.com'], explorerUrls: ['https://etherscan.io'], nativeCurrency: { name: 'Ether', symbol: 'ETH', decimals: 18 }, blockTime: 12, gasLimit: 30000000, supportsEIP1559: true, supportsFlashbots: true, supportsMEV: true, supportsMulticall: true, supportsBatchCalls: true });
    this.addChain({ id: 'sepolia', name: 'Sepolia', symbol: 'ETH', category: 'evm', status: 'active', chainId: 11155111, rpcUrls: ['https://rpc.sepolia.org', 'https://rpc.sepolia.ethpandaops.io'], explorerUrls: ['https://sepolia.etherscan.io'], nativeCurrency: { name: 'Sepolia Ether', symbol: 'ETH', decimals: 18 }, blockTime: 12, supportsEIP1559: true });
    
    // BNB Chain
    this.addChain({ id: 'bnb-smart-chain', name: 'BNB Chain', symbol: 'BNB', category: 'evm', status: 'active', chainId: 56, networkId: 56, rpcUrls: ['https://bsc-dataseed.binance.org', 'https://rpc.ankr.com/bsc', 'https://binance.llamarpc.com'], explorerUrls: ['https://bscscan.com'], nativeCurrency: { name: 'BNB', symbol: 'BNB', decimals: 18 }, blockTime: 3, gasLimit: 30000000, supportsEIP1559: true, supportsMulticall: true });
    this.addChain({ id: 'bnb-testnet', name: 'BNB Chain Testnet', symbol: 'BNB', category: 'evm', status: 'active', chainId: 97, rpcUrls: ['https://data-seed-prebsc-1-s1.binance.org:8545'], explorerUrls: ['https://testnet.bscscan.com'], nativeCurrency: { name: 'Test BNB', symbol: 'BNB', decimals: 18 }, blockTime: 3 });
    
    // Polygon
    this.addChain({ id: 'polygon', name: 'Polygon', symbol: 'MATIC', category: 'evm', status: 'active', chainId: 137, networkId: 137, rpcUrls: ['https://polygon-rpc.com', 'https://rpc.ankr.com/polygon', 'https://polygon.llamarpc.com'], explorerUrls: ['https://polygonscan.com'], nativeCurrency: { name: 'MATIC', symbol: 'MATIC', decimals: 18 }, blockTime: 2, gasLimit: 30000000, supportsEIP1559: true, supportsMulticall: true });
    this.addChain({ id: 'polygon-mumbai', name: 'Mumbai', symbol: 'MATIC', category: 'evm', status: 'active', chainId: 80001, rpcUrls: ['https://rpc-mumbai.maticvigil.com'], explorerUrls: ['https://mumbai.polygonscan.com'], nativeCurrency: { name: 'Test MATIC', symbol: 'MATIC', decimals: 18 }, blockTime: 2 });
    
    // Arbitrum
    this.addChain({ id: 'arbitrum-one', name: 'Arbitrum One', symbol: 'ETH', category: 'evm', status: 'active', chainId: 42161, networkId: 42161, rpcUrls: ['https://arb1.arbitrum.io/rpc', 'https://rpc.ankr.com/arbitrum'], explorerUrls: ['https://arbiscan.io'], nativeCurrency: { name: 'Ether', symbol: 'ETH', decimals: 18 }, blockTime: 1, gasLimit: 32000000, supportsEIP1559: true, supportsMulticall: true });
    this.addChain({ id: 'arbitrum-sepolia', name: 'Arbitrum Sepolia', symbol: 'ETH', category: 'evm', status: 'active', chainId: 421614, rpcUrls: ['https://sepolia-rollup.arbitrum.io/rpc'], explorerUrls: ['https://sepolia.arbiscan.io'], nativeCurrency: { name: 'Sepolia Ether', symbol: 'ETH', decimals: 18 }, blockTime: 1 });
    
    // Optimism
    this.addChain({ id: 'optimism', name: 'Optimism', symbol: 'ETH', category: 'evm', status: 'active', chainId: 10, networkId: 10, rpcUrls: ['https://mainnet.optimism.io', 'https://rpc.ankr.com/optimism'], explorerUrls: ['https://optimistic.etherscan.io'], nativeCurrency: { name: 'Ether', symbol: 'ETH', decimals: 18 }, blockTime: 2, supportsEIP1559: true, supportsMulticall: true });
    this.addChain({ id: 'optimism-sepolia', name: 'Optimism Sepolia', symbol: 'ETH', category: 'evm', status: 'active', chainId: 11155420, rpcUrls: ['https://sepolia.optimism.io'], explorerUrls: ['https://sepolia-optimism.etherscan.io'], nativeCurrency: { name: 'Sepolia Ether', symbol: 'ETH', decimals: 18 }, blockTime: 2 });
    
    // Base
    this.addChain({ id: 'base', name: 'Base', symbol: 'ETH', category: 'evm', status: 'active', chainId: 8453, networkId: 8453, rpcUrls: ['https://mainnet.base.org', 'https://base.llamarpc.com'], explorerUrls: ['https://basescan.org'], nativeCurrency: { name: 'Ether', symbol: 'ETH', decimals: 18 }, blockTime: 2, supportsEIP1559: true, supportsMulticall: true });
    this.addChain({ id: 'base-sepolia', name: 'Base Sepolia', symbol: 'ETH', category: 'evm', status: 'active', chainId: 84532, rpcUrls: ['https://sepolia.base.org'], explorerUrls: ['https://sepolia.basescan.org'], nativeCurrency: { name: 'Sepolia Ether', symbol: 'ETH', decimals: 18 }, blockTime: 2 });
    
    // Avalanche
    this.addChain({ id: 'avalanche-c', name: 'Avalanche C-Chain', symbol: 'AVAX', category: 'evm', status: 'active', chainId: 43114, networkId: 43114, rpcUrls: ['https://api.avax.network/ext/bc/C/rpc', 'https://rpc.ankr.com/avalanche'], explorerUrls: ['https://snowtrace.io'], nativeCurrency: { name: 'Avalanche', symbol: 'AVAX', decimals: 18 }, blockTime: 2, supportsEIP1559: true });
    this.addChain({ id: 'avalanche-fuji', name: 'Avalanche Fuji', symbol: 'AVAX', category: 'evm', status: 'active', chainId: 43113, rpcUrls: ['https://api.avax-test.network/ext/bc/C/rpc'], explorerUrls: ['https://testnet.snowtrace.io'], nativeCurrency: { name: 'Test AVAX', symbol: 'AVAX', decimals: 18 }, blockTime: 2 });
    
    // Fantom
    this.addChain({ id: 'fantom', name: 'Fantom', symbol: 'FTM', category: 'evm', status: 'active', chainId: 250, networkId: 250, rpcUrls: ['https://rpc.fantom.network', 'https://fantom.llamarpc.com'], explorerUrls: ['https://ftmscan.com'], nativeCurrency: { name: 'Fantom', symbol: 'FTM', decimals: 18 }, blockTime: 1 });
    this.addChain({ id: 'fantom-testnet', name: 'Fantom Testnet', symbol: 'FTM', category: 'evm', status: 'active', chainId: 4002, rpcUrls: ['https://rpc.ankr.com/fantom_testnet'], explorerUrls: ['https://testnet.ftmscan.com'], nativeCurrency: { name: 'Test FTM', symbol: 'FTM', decimals: 18 }, blockTime: 1 });
    
    // Other EVM Chains
    this.addChain({ id: 'cronos', name: 'Cronos', symbol: 'CRO', category: 'evm', status: 'active', chainId: 25, rpcUrls: ['https://evm.cronos.org'], explorerUrls: ['https://cronoscan.com'], nativeCurrency: { name: 'Cronos', symbol: 'CRO', decimals: 18 }, blockTime: 6 });
    this.addChain({ id: 'celo', name: 'Celo', symbol: 'CELO', category: 'evm', status: 'active', chainId: 42220, rpcUrls: ['https://forno.celo.org'], explorerUrls: ['https://celoscan.io'], nativeCurrency: { name: 'Celo', symbol: 'CELO', decimals: 18 }, blockTime: 5 });
    this.addChain({ id: 'gnosis', name: 'Gnosis Chain', symbol: 'xDai', category: 'evm', status: 'active', chainId: 100, rpcUrls: ['https://rpc.gnosischain.com'], explorerUrls: ['https://gnosisscan.io'], nativeCurrency: { name: 'xDai', symbol: 'xDai', decimals: 18 }, blockTime: 5 });
    this.addChain({ id: 'moonbeam', name: 'Moonbeam', symbol: 'GLMR', category: 'evm', status: 'active', chainId: 1284, rpcUrls: ['https://rpc.api.moonbeam.network'], explorerUrls: ['https://moonscan.io'], nativeCurrency: { name: 'Glimmer', symbol: 'GLMR', decimals: 18 }, blockTime: 12 });
    this.addChain({ id: 'moonriver', name: 'Moonriver', symbol: 'MOVR', category: 'evm', status: 'active', chainId: 1285, rpcUrls: ['https://rpc.api.moonriver.moonbeam.network'], explorerUrls: ['https://moonriver.moonscan.io'], nativeCurrency: { name: 'Moonriver', symbol: 'MOVR', decimals: 18 }, blockTime: 12 });
    this.addChain({ id: 'kava', name: 'Kava', symbol: 'KAVA', category: 'evm', status: 'active', chainId: 2222, rpcUrls: ['https://evm.kava.io'], explorerUrls: ['https://kavascan.com'], nativeCurrency: { name: 'Kava', symbol: 'KAVA', decimals: 18 }, blockTime: 6 });
    this.addChain({ id: 'linea', name: 'Linea', symbol: 'ETH', category: 'evm', status: 'active', chainId: 59144, rpcUrls: ['https://rpc.linea.build'], explorerUrls: ['https://lineascan.build'], nativeCurrency: { name: 'Ether', symbol: 'ETH', decimals: 18 }, blockTime: 12, supportsEIP1559: true });
    this.addChain({ id: 'zkevm', name: 'zkEVM', symbol: 'ETH', category: 'evm', status: 'active', chainId: 1101, rpcUrls: ['https://zkevm-rpc.com'], explorerUrls: ['https://zkevm.polygonscan.com'], nativeCurrency: { name: 'Ether', symbol: 'ETH', decimals: 18 }, blockTime: 12 });
    this.addChain({ id: 'scroll', name: 'Scroll', symbol: 'ETH', category: 'evm', status: 'active', chainId: 534352, rpcUrls: ['https://rpc.scroll.io'], explorerUrls: ['https://scrollscan.com'], nativeCurrency: { name: 'Ether', symbol: 'ETH', decimals: 18 }, blockTime: 3 });
    this.addChain({ id: 'mantle', name: 'Mantle', symbol: 'MNT', category: 'evm', status: 'active', chainId: 5000, rpcUrls: ['https://rpc.mantle.xyz'], explorerUrls: ['https://mantlescan.xyz'], nativeCurrency: { name: 'Mantle', symbol: 'MNT', decimals: 18 }, blockTime: 2 });
    this.addChain({ id: 'opbnb', name: 'opBNB', symbol: 'BNB', category: 'evm', status: 'active', chainId: 204, rpcUrls: ['https://opbnb.publicnode.com'], explorerUrls: ['https://opbnbscan.com'], nativeCurrency: { name: 'BNB', symbol: 'BNB', decimals: 18 }, blockTime: 1 });
    this.addChain({ id: 'mode', name: 'Mode', symbol: 'ETH', category: 'evm', status: 'active', chainId: 34443, rpcUrls: ['https://mainnet.mode.network'], explorerUrls: ['https://explorer.mode.network'], nativeCurrency: { name: 'Ether', symbol: 'ETH', decimals: 18 }, blockTime: 2 });
    this.addChain({ id: 'zora', name: 'Zora', symbol: 'ETH', category: 'evm', status: 'active', chainId: 7777777, rpcUrls: ['https://rpc.zora.energy'], explorerUrls: ['https://explorer.zora.energy'], nativeCurrency: { name: 'Ether', symbol: 'ETH', decimals: 18 }, blockTime: 2 });
    this.addChain({ id: 'harmony', name: 'Harmony', symbol: 'ONE', category: 'evm', status: 'active', chainId: 1666600000, rpcUrls: ['https://api.harmony.one'], explorerUrls: ['https://explorer.harmony.one'], nativeCurrency: { name: 'Harmony', symbol: 'ONE', decimals: 18 }, blockTime: 2 });
    this.addChain({ id: 'metis', name: 'Metis', symbol: 'METIS', category: 'evm', status: 'active', chainId: 1088, rpcUrls: ['https://andromeda.metis.io'], explorerUrls: ['https://andromeda-explorer.metis.io'], nativeCurrency: { name: 'Metis', symbol: 'METIS', decimals: 18 }, blockTime: 1 });
    this.addChain({ id: 'shimmer', name: 'Shimmer', symbol: 'SMR', category: 'evm', status: 'active', chainId: 148, rpcUrls: ['https://json-rpc.evm.shimmer.network'], explorerUrls: ['https://explorer.shimmer.network'], nativeCurrency: { name: 'Shimmer', symbol: 'SMR', decimals: 18 }, blockTime: 5 });
    this.addChain({ id: 'core', name: 'Core', symbol: 'CORE', category: 'evm', status: 'active', chainId: 1116, rpcUrls: ['https://rpc.coredao.org'], explorerUrls: ['https://scan.coredao.org'], nativeCurrency: { name: 'Core', symbol: 'CORE', decimals: 18 }, blockTime: 1 });

    // =============================================================================
    // SOLANA (Non-EVM)
    // =============================================================================
    this.addChain({ id: 'solana', name: 'Solana', symbol: 'SOL', category: 'solana', status: 'active', chainId: 101, rpcUrls: ['https://api.mainnet-beta.solana.com', 'https://solana-api.projectserum.com', 'https://rpc.ankr.com/solana'], explorerUrls: ['https://solscan.io', 'https://solana.fm'], nativeCurrency: { name: 'Solana', symbol: 'SOL', decimals: 9 }, blockTime: 0.4, supportsMulticall: false });
    this.addChain({ id: 'solana-devnet', name: 'Solana Devnet', symbol: 'SOL', category: 'solana', status: 'active', chainId: 102, rpcUrls: ['https://api.devnet.solana.com'], explorerUrls: ['https://explorer.solana.com?cluster=devnet'], nativeCurrency: { name: 'Solana', symbol: 'SOL', decimals: 9 }, blockTime: 0.4 });
    this.addChain({ id: 'solana-testnet', name: 'Solana Testnet', symbol: 'SOL', category: 'solana', status: 'active', chainId: 103, rpcUrls: ['https://api.testnet.solana.com'], explorerUrls: ['https://explorer.solana.com?cluster=testnet'], nativeCurrency: { name: 'Solana', symbol: 'SOL', decimals: 9 }, blockTime: 0.4 });

    // =============================================================================
    // APTOS (Non-EVM)
    // =============================================================================
    this.addChain({ id: 'aptos', name: 'Aptos', symbol: 'APT', category: 'aptos', status: 'active', chainId: 1, rpcUrls: ['https://fullnode.mainnet.aptoslabs.com', 'https://aptos-mainnet.nodereal.io/v1'], explorerUrls: ['https://explorer.aptoslabs.com'], nativeCurrency: { name: 'Aptos', symbol: 'APT', decimals: 8 }, blockTime: 1 });
    this.addChain({ id: 'aptos-devnet', name: 'Aptos Devnet', symbol: 'APT', category: 'aptos', status: 'active', chainId: 2, rpcUrls: ['https://fullnode.devnet.aptoslabs.com'], explorerUrls: ['https://explorer.aptoslabs.com/?network=devnet'], nativeCurrency: { name: 'Aptos', symbol: 'APT', decimals: 8 }, blockTime: 1 });

    // =============================================================================
    // SUI (Non-EVM)
    // =============================================================================
    this.addChain({ id: 'sui', name: 'Sui', symbol: 'SUI', category: 'sui', status: 'active', chainId: 1, rpcUrls: ['https://fullnode.mainnet.sui.io', 'https://rpc.ankr.com/sui'], explorerUrls: ['https://suiscan.xyz', 'https://explorer.sui.io'], nativeCurrency: { name: 'Sui', symbol: 'SUI', decimals: 9 }, blockTime: 1 });
    this.addChain({ id: 'sui-devnet', name: 'Sui Devnet', symbol: 'SUI', category: 'sui', status: 'active', chainId: 2, rpcUrls: ['https://fullnode.devnet.sui.io'], explorerUrls: ['https://explorer.sui.io/?network=devnet'], nativeCurrency: { name: 'Sui', symbol: 'SUI', decimals: 9 }, blockTime: 1 });

    // =============================================================================
    // TON (Non-EVM)
    // =============================================================================
    this.addChain({ id: 'ton', name: 'TON', symbol: 'TON', category: 'ton', status: 'active', chainId: 0, rpcUrls: ['https://toncenter.com/api/v2/jsonRPC', 'https://tonapi.io/v2/jsonRPC'], explorerUrls: ['https://tonscan.org', 'https://explorer.toncoin.org'], nativeCurrency: { name: 'Toncoin', symbol: 'TON', decimals: 9 }, blockTime: 5 });

    // =============================================================================
    // TRON (Non-EVM)
    // =============================================================================
    this.addChain({ id: 'tron', name: 'TRON', symbol: 'TRX', category: 'tron', status: 'active', chainId: 728126428, rpcUrls: ['https://api.trongrid.io', 'https://rpc.ankr.com/tron'], explorerUrls: ['https://tronscan.org'], nativeCurrency: { name: 'TRON', symbol: 'TRX', decimals: 6 }, blockTime: 3 });
    this.addChain({ id: 'tron-nile', name: 'TRON Nile', symbol: 'TRX', category: 'tron', status: 'active', chainId: 5234, rpcUrls: ['https://api.nileex.io'], explorerUrls: ['https://nile.tronscan.org'], nativeCurrency: { name: 'TRON', symbol: 'TRX', decimals: 6 }, blockTime: 3 });

    // =============================================================================
    // COSMOS (Non-EVM)
    // =============================================================================
    this.addChain({ id: 'cosmos-hub', name: 'Cosmos Hub', symbol: 'ATOM', category: 'cosmos', status: 'active', chainId: 1, rpcUrls: ['https://rpc.cosmos.network'], explorerUrls: ['https://mintscan.io/cosmoshub'], nativeCurrency: { name: 'Atom', symbol: 'ATOM', decimals: 6 }, blockTime: 7 });
    this.addChain({ id: 'osmosis', name: 'Osmosis', symbol: 'OSMO', category: 'cosmos', status: 'active', chainId: 1, rpcUrls: ['https://rpc.osmosis.zone'], explorerUrls: ['https://mintscan.io/osmosis'], nativeCurrency: { name: 'Osmosis', symbol: 'OSMO', decimals: 6 }, blockTime: 6 });
    this.addChain({ id: 'injective', name: 'Injective', symbol: 'INJ', category: 'cosmos', status: 'active', chainId: 1, rpcUrls: ['https://public.injective.network'], explorerUrls: ['https://explorer.injective.network'], nativeCurrency: { name: 'Injective', symbol: 'INJ', decimals: 18 }, blockTime: 1 });

    // =============================================================================
    // NEAR (Non-EVM)
    // =============================================================================
    this.addChain({ id: 'near', name: 'NEAR Protocol', symbol: 'NEAR', category: 'near', status: 'active', chainId: 1, rpcUrls: ['https://rpc.ankr.com/near', 'https://rpc.mainnet.near.org'], explorerUrls: ['https://explorer.near.org'], nativeCurrency: { name: 'NEAR', symbol: 'NEAR', decimals: 24 }, blockTime: 1 });

    // =============================================================================
    // ALGORAND (Non-EVM)
    // =============================================================================
    this.addChain({ id: 'algorand', name: 'Algorand', symbol: 'ALGO', category: 'algorand', status: 'active', chainId: 416849760, rpcUrls: ['https://mainnet-api.algonode.org', 'https://algoexplorerapi.io'], explorerUrls: ['https://algoexplorer.io'], nativeCurrency: { name: 'Algorand', symbol: 'ALGO', decimals: 6 }, blockTime: 3 });

    // =============================================================================
    // POLKADOT (Non-EVM)
    // =============================================================================
    this.addChain({ id: 'polkadot', name: 'Polkadot', symbol: 'DOT', category: 'polkadot', status: 'active', chainId: 1, rpcUrls: ['https://rpc.polkadot.io'], explorerUrls: ['https://explorer.polkadot.io'], nativeCurrency: { name: 'Polkadot', symbol: 'DOT', decimals: 10 }, blockTime: 6 });
    this.addChain({ id: 'kusama', name: 'Kusama', symbol: 'KSM', category: 'polkadot', status: 'active', chainId: 2, rpcUrls: ['https://rpc.kusama.io'], explorerUrls: ['https://explorer.kusama.io'], nativeCurrency: { name: 'Kusama', symbol: 'KSM', decimals: 12 }, blockTime: 6 });

    // =============================================================================
    // CARDANO (Non-EVM)
    // =============================================================================
    this.addChain({ id: 'cardano', name: 'Cardano', symbol: 'ADA', category: 'cardano', status: 'active', chainId: 1, rpcUrls: ['https://cardano-mainnet.blockfrost.io/api/v0'], explorerUrls: ['https://cardanoscan.io'], nativeCurrency: { name: 'Ada', symbol: 'ADA', decimals: 6 }, blockTime: 20 });

    console.log(`Initialized ${this.chains.size} chains across ${this.chainCategories.size} categories`);
  }

  // ============================================================================
  // Utility Methods
  // ============================================================================

  getChainStats(): { total: number; byCategory: Record<string, number>; byStatus: Record<string, number>; evmCount: number; nonEvmCount: number } {
    const byCategory: Record<string, number> = {};
    const byStatus: Record<string, number> = {};
    let evmCount = 0;
    let nonEvmCount = 0;

    for (const [category, chains] of this.chainCategories) {
      byCategory[category] = chains.size;
    }

    for (const chain of this.chains.values()) {
      byStatus[chain.status] = (byStatus[chain.status] || 0) + 1;
      if (chain.category === 'evm') evmCount++;
      else nonEvmCount++;
    }

    return { total: this.chains.size, byCategory, byStatus, evmCount, nonEvmCount };
  }

  exportChains(): string {
    return JSON.stringify(Array.from(this.chains.values()), null, 2);
  }

  importChains(chainsJson: string): number {
    const chains = JSON.parse(chainsJson) as ChainConfig[];
    let imported = 0;
    for (const chain of chains) {
      if (!this.chains.has(chain.id)) {
        this.addChain(chain);
        imported++;
      }
    }
    return imported;
  }
}

export const chainRegistry = new UniversalChainRegistry();
export default UniversalChainRegistry;