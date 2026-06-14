// TigerEX Integration - Rust Implementation
// High-Performance, Memory-Safe Core Library
//
// This module provides:
// - Thread-safe operations with Rust's ownership model
// - Ultra-low latency for trading operations
// - Memory safety without garbage collection
// - Zero-cost abstractions

// ============================================================================
// Cargo.toml
// ============================================================================
// [package]
// name = "tiger_ex"
// version = "1.0.0"
// edition = "2021"
//
// [dependencies]
// serde = { version = "1.0", features = ["derive"] }
// serde_json = "1.0"
// tokio = { version = "1.0", features = ["full"] }
// parking_lot = "0.12"
// ahash = "0.7"
// uuid = { version = "1.0", features = ["v4"] }
// chrono = { version = "0.4", features = ["serde"] }
// thiserror = "1.0"

use std::collections::HashMap;
use std::sync::RwLock;
use std::time::{SystemTime, UNIX_EPOCH};

// ============================================================================
// Types & Structures
// ============================================================================

#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
pub enum ChainCategory {
    Evm,
    Solana,
    Aptos,
    Sui,
    Ton,
    Cosmos,
    Near,
    Other,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
pub enum ChainStatus {
    Active,
    Inactive,
    Paused,
    Deprecated,
}

#[derive(Debug, Clone)]
pub struct ChainConfig {
    pub id: String,
    pub name: String,
    pub symbol: String,
    pub category: ChainCategory,
    pub status: ChainStatus,
    pub chain_id: i64,
    pub rpc_urls: Vec<String>,
    pub explorer_urls: Vec<String>,
    pub native_currency: NativeCurrency,
    pub block_time: f64,
    pub supports_eip1559: bool,
}

#[derive(Debug, Clone)]
pub struct NativeCurrency {
    pub name: String,
    pub symbol: String,
    pub decimals: u8,
}

#[derive(Debug, Clone)]
pub struct TokenConfig {
    pub address: String,
    pub chain_id: String,
    pub symbol: String,
    pub name: String,
    pub decimals: u8,
    pub is_native: bool,
    pub is_stable: bool,
    pub is_wrapped: bool,
    pub wrapped_of: Option<String>,
}

#[derive(Debug, Clone)]
pub struct LiquidityPool {
    pub token_a: String,
    pub token_b: String,
    pub reserve_a: i64,
    pub reserve_b: i64,
    pub fee: i32,          // basis points
    pub liquidity: i64,
    pub apy: i32,
}

#[derive(Debug, Clone)]
pub struct FarmInfo {
    pub pool_id: String,
    pub reward_token: String,
    pub staked_token: String,
    pub staked_amount: i64,
    pub reward_amount: i64,
    pub apy: i32,
    pub start_time: i64,
    pub end_time: i64,
}

#[derive(Debug, Clone)]
pub struct BridgeInfo {
    pub bridge_id: String,
    pub source_chain: String,
    pub target_chain: String,
    pub token: String,
    pub min_amount: i64,
    pub max_amount: i64,
    pub fee: i32,           // basis points
    pub estimated_time: i32, // milliseconds
    pub is_active: bool,
}

#[derive(Debug, Clone)]
pub struct FeeConfig {
    pub trading_fee: i32,     // basis points
    pub swap_fee: i32,       // basis points
    pub bridge_fee: i32,      // basis points
    pub wallet_tx_fee: i32,  // basis points
    pub withdrawal_fee: i32,
    pub deposit_fee: i32,
}

#[derive(Debug, Clone)]
pub struct Product {
    pub name: String,
    pub version: String,
    pub status: String,
    pub fee_config: FeeConfig,
    pub enabled: bool,
    pub created_at: i64,
    pub updated_at: i64,
}

#[derive(Debug, Clone)]
pub struct SwapResult {
    pub input_token: String,
    pub output_token: String,
    pub amount_in: i64,
    pub amount_out: i64,
    pub fee: i64,
    pub path: Vec<String>,
}

#[derive(Debug, Clone)]
pub struct BridgeResult {
    pub source_chain: String,
    pub target_chain: String,
    pub amount_sent: i64,
    pub amount_received: i64,
    pub fee: i64,
    pub estimated_time: i32,
}

#[derive(Debug, Clone)]
pub struct FeeRecord {
    pub amount: i64,
    pub source: String,
    pub timestamp: i64,
}

// ============================================================================
// TigerEX Core (Rust)
// ============================================================================

pub struct TigerEX {
    // Products
    pub tiger_wallet: Product,
    pub tiger_swap: Product,
    pub tiger_chain: Product,
    pub tiger_ex: Product,

    // Chains
    evm_chains: RwLock<HashMap<String, ChainConfig>>,
    non_evm_chains: RwLock<HashMap<String, ChainConfig>>,

    // Tokens
    tokens: RwLock<HashMap<String, TokenConfig>>,

    // DEX Pools
    pools: RwLock<HashMap<String, LiquidityPool>>,

    // Farms
    farms: RwLock<HashMap<String, FarmInfo>>,

    // Bridges
    bridges: RwLock<HashMap<String, BridgeInfo>>,

    // Fee collection
    total_fees_collected: RwLock<i64>,
    fee_history: RwLock<Vec<FeeRecord>>,

    // Stats
    initialized: RwLock<bool>,
}

impl Default for TigerEX {
    fn default() -> Self {
        Self::new()
    }
}

impl TigerEX {
    pub fn new() -> Self {
        let ex = Self {
            tiger_wallet: Product {
                name: "TigerWallet".to_string(),
                version: "1.0.0".to_string(),
                status: "active".to_string(),
                fee_config: FeeConfig {
                    trading_fee: 0,
                    swap_fee: 0,
                    bridge_fee: 0,
                    wallet_tx_fee: 10,  // 0.1%
                    withdrawal_fee: 5,
                    deposit_fee: 0,
                },
                enabled: true,
                created_at: current_time_millis(),
                updated_at: current_time_millis(),
            },
            tiger_swap: Product {
                name: "Tigerswap".to_string(),
                version: "1.0.0".to_string(),
                status: "active".to_string(),
                fee_config: FeeConfig {
                    trading_fee: 0,
                    swap_fee: 30,   // 0.3%
                    bridge_fee: 0,
                    wallet_tx_fee: 0,
                    withdrawal_fee: 0,
                    deposit_fee: 0,
                },
                enabled: true,
                created_at: current_time_millis(),
                updated_at: current_time_millis(),
            },
            tiger_chain: Product {
                name: "TigerSmartChain".to_string(),
                version: "1.0.0".to_string(),
                status: "active".to_string(),
                fee_config: FeeConfig {
                    trading_fee: 0,
                    swap_fee: 0,
                    bridge_fee: 10,  // 0.1%
                    wallet_tx_fee: 0,
                    withdrawal_fee: 5,
                    deposit_fee: 0,
                },
                enabled: true,
                created_at: current_time_millis(),
                updated_at: current_time_millis(),
            },
            tiger_ex: Product {
                name: "TigerEx".to_string(),
                version: "1.0.0".to_string(),
                status: "active".to_string(),
                fee_config: FeeConfig {
                    trading_fee: 10,   // 0.1%
                    swap_fee: 0,
                    bridge_fee: 0,
                    wallet_tx_fee: 0,
                    withdrawal_fee: 5,
                    deposit_fee: 0,
                },
                enabled: true,
                created_at: current_time_millis(),
                updated_at: current_time_millis(),
            },
            evm_chains: RwLock::new(HashMap::new()),
            non_evm_chains: RwLock::new(HashMap::new()),
            tokens: RwLock::new(HashMap::new()),
            pools: RwLock::new(HashMap::new()),
            farms: RwLock::new(HashMap::new()),
            bridges: RwLock::new(HashMap::new()),
            total_fees_collected: RwLock::new(0),
            fee_history: RwLock::new(Vec::new()),
            initialized: RwLock::new(false),
        };

        ex
    }

    pub fn initialize(&self) {
        self.initialize_evm_chains();
        self.initialize_non_evm_chains();
        self.initialize_tokens();
        self.initialize_dex_pools();
        self.initialize_farms();
        self.initialize_bridges();
        
        *self.initialized.write().unwrap() = true;
        println!("[TigerEX] Initialized successfully");
    }

    fn initialize_evm_chains(&self) {
        let mut chains = self.evm_chains.write().unwrap();
        
        // TigerSmartChain (Native)
        chains.insert("tigersmartchain".to_string(), ChainConfig {
            id: "tigersmartchain".to_string(),
            name: "TigerSmartChain".to_string(),
            symbol: "TGR".to_string(),
            category: ChainCategory::Evm,
            status: ChainStatus::Active,
            chain_id: 13000,
            rpc_urls: vec!["https://rpc.tigersmartchain.com".to_string()],
            explorer_urls: vec!["https://scan.tigersmartchain.com".to_string()],
            native_currency: NativeCurrency { name: "Tiger".to_string(), symbol: "TGR".to_string(), decimals: 18 },
            block_time: 2.0,
            supports_eip1559: true,
        });

        // Ethereum
        chains.insert("ethereum".to_string(), ChainConfig {
            id: "ethereum".to_string(),
            name: "Ethereum".to_string(),
            symbol: "ETH".to_string(),
            category: ChainCategory::Evm,
            status: ChainStatus::Active,
            chain_id: 1,
            rpc_urls: vec!["https://eth.llamarpc.com".to_string()],
            explorer_urls: vec!["https://etherscan.io".to_string()],
            native_currency: NativeCurrency { name: "Ethereum".to_string(), symbol: "ETH".to_string(), decimals: 18 },
            block_time: 12.0,
            supports_eip1559: true,
        });

        // BSC
        chains.insert("bsc".to_string(), ChainConfig {
            id: "bsc".to_string(),
            name: "BNB Smart Chain".to_string(),
            symbol: "BNB".to_string(),
            category: ChainCategory::Evm,
            status: ChainStatus::Active,
            chain_id: 56,
            rpc_urls: vec!["https://bsc-dataseed.binance.org".to_string()],
            explorer_urls: vec!["https://bscscan.com".to_string()],
            native_currency: NativeCurrency { name: "BNB".to_string(), symbol: "BNB".to_string(), decimals: 18 },
            block_time: 3.0,
            supports_eip1559: true,
        });

        // Polygon
        chains.insert("polygon".to_string(), ChainConfig {
            id: "polygon".to_string(),
            name: "Polygon".to_string(),
            symbol: "MATIC".to_string(),
            category: ChainCategory::Evm,
            status: ChainStatus::Active,
            chain_id: 137,
            rpc_urls: vec!["https://polygon-rpc.com".to_string()],
            explorer_urls: vec!["https://polygonscan.com".to_string()],
            native_currency: NativeCurrency { name: "MATIC".to_string(), symbol: "MATIC".to_string(), decimals: 18 },
            block_time: 2.0,
            supports_eip1559: true,
        });

        // Avalanche
        chains.insert("avalanche".to_string(), ChainConfig {
            id: "avalanche".to_string(),
            name: "Avalanche".to_string(),
            symbol: "AVAX".to_string(),
            category: ChainCategory::Evm,
            status: ChainStatus::Active,
            chain_id: 43114,
            rpc_urls: vec!["https://api.avax.network/ext/bc/C/rpc".to_string()],
            explorer_urls: vec!["https://snowtrace.io".to_string()],
            native_currency: NativeCurrency { name: "AVAX".to_string(), symbol: "AVAX".to_string(), decimals: 18 },
            block_time: 2.0,
            supports_eip1559: false,
        });

        // Arbitrum
        chains.insert("arbitrum".to_string(), ChainConfig {
            id: "arbitrum".to_string(),
            name: "Arbitrum One".to_string(),
            symbol: "ETH".to_string(),
            category: ChainCategory::Evm,
            status: ChainStatus::Active,
            chain_id: 42161,
            rpc_urls: vec!["https://arb1.arbitrum.io/rpc".to_string()],
            explorer_urls: vec!["https://arbiscan.io".to_string()],
            native_currency: NativeCurrency { name: "Ethereum".to_string(), symbol: "ETH".to_string(), decimals: 18 },
            block_time: 0.25,
            supports_eip1559: true,
        });

        // Optimism
        chains.insert("optimism".to_string(), ChainConfig {
            id: "optimism".to_string(),
            name: "Optimism".to_string(),
            symbol: "ETH".to_string(),
            category: ChainCategory::Evm,
            status: ChainStatus::Active,
            chain_id: 10,
            rpc_urls: vec!["https://mainnet.optimism.io".to_string()],
            explorer_urls: vec!["https://optimistic.etherscan.io".to_string()],
            native_currency: NativeCurrency { name: "Ethereum".to_string(), symbol: "ETH".to_string(), decimals: 18 },
            block_time: 2.0,
            supports_eip1559: true,
        });

        // Base
        chains.insert("base".to_string(), ChainConfig {
            id: "base".to_string(),
            name: "Base".to_string(),
            symbol: "ETH".to_string(),
            category: ChainCategory::Evm,
            status: ChainStatus::Active,
            chain_id: 8453,
            rpc_urls: vec!["https://mainnet.base.org".to_string()],
            explorer_urls: vec!["https://basescan.org".to_string()],
            native_currency: NativeCurrency { name: "Ethereum".to_string(), symbol: "ETH".to_string(), decimals: 18 },
            block_time: 2.0,
            supports_eip1559: true,
        });

        // Additional EVM chains
        let additional_chains = vec![
            ("fantom", "Fantom", "FTM", 250, "https://rpc.ftm.tools", "https://ftmscan.com", "FTM", 18, 1.0),
            ("celo", "Celo", "CELO", 42220, "https://forno.celo.org", "https://explorer.celo.org", "CELO", 18, 5.0),
            ("gnosis", "Gnosis Chain", "GNO", 100, "https://rpc.gnosischain.com", "https://gnosisscan.io", "GNO", 18, 5.0),
            ("moonbeam", "Moonbeam", "GLMR", 1284, "https://rpc.api.moonbeam.network", "https://moonbeam.moonscan.io", "GLMR", 18, 12.0),
            ("zkevm", "Polygon zkEVM", "ETH", 1101, "https://zkevm-rpc.com", "https://zkevm.polygonscan.com", "ETH", 18, 1.0),
            ("linea", "Linea", "ETH", 59144, "https://rpc.linea.build", "https://lineascan.build", "ETH", 18, 2.0),
            ("scroll", "Scroll", "ETH", 534352, "https://rpc.scroll.io", "https://scrollscan.com", "ETH", 18, 3.0),
            ("astar", "Astar", "ASTR", 432201, "https://rpc.astar.network", "https://blockscout.com/astar", "ASTR", 18, 12.0),
            ("klaytn", "Klaytn", "KLAY", 8217, "https://klaytn-mainnet-rpc.allthatnode.com", "https://scope.klaytn.com", "KLAY", 18, 1.0),
            ("cronos", "Cronos", "CRO", 25, "https://evm.cronos.org", "https://cronoscan.com", "CRO", 18, 5.0),
            ("core", "Core", "CORE", 1116, "https://rpc.coredao.org", "https://scan.coredao.org", "CORE", 18, 2.0),
            ("mantle", "Mantle", "MNT", 5000, "https://rpc.mantle.xyz", "https://explorer.mantle.xyz", "MNT", 18, 2.0),
            ("berachain", "Berachain", "BERA", 845321, "https://rpc.berachain.com", "https://berascan.com", "BERA", 18, 2.0),
            ("sonic", "Sonic", "S", 1460, "https://rpc.soniclabs.com", "https://sonicscan.org", "S", 18, 2.0),
            ("monad", "Monad", "MON", 10143, "https://rpc.monad.xyz", "https://monadscan.com", "MON", 18, 2.0),
            ("megaeth", "MegaETH", "MEGA", 1205398815, "https://rpc.megaeth.com", "https://megascan.io", "MEGA", 18, 0.1),
        ];

        for (id, name, symbol, chain_id, rpc, explorer, native, decimals, block_time) in additional_chains {
            chains.insert(id.to_string(), ChainConfig {
                id: id.to_string(),
                name: name.to_string(),
                symbol: symbol.to_string(),
                category: ChainCategory::Evm,
                status: ChainStatus::Active,
                chain_id,
                rpc_urls: vec![rpc.to_string()],
                explorer_urls: vec![explorer.to_string()],
                native_currency: NativeCurrency { name: native.to_string(), symbol: symbol.to_string(), decimals },
                block_time,
                supports_eip1559: true,
            });
        }
    }

    fn initialize_non_evm_chains(&self) {
        let mut chains = self.non_evm_chains.write().unwrap();
        
        let non_evm_chains = vec![
            ("solana", "Solana", "SOL", "solana", -1, "https://api.mainnet-beta.solana.com", "https://solscan.io", "SOL", 9, 0.4),
            ("aptos", "Aptos", "APT", "aptos", -1, "https://aptos-mainnet.nodereal.io/v1", "https://explorer.aptoslabs.com", "APT", 8, 1.0),
            ("sui", "Sui", "SUI", "sui", -1, "https://rpc.mainnet.sui.io", "https://suiscan.xyz", "SUI", 9, 1.0),
            ("ton", "TON", "TON", "ton", -1, "https://toncenter.com/api/v2", "https://tonscan.org", "TON", 9, 5.0),
            ("cosmos", "Cosmos", "ATOM", "cosmos", -1, "https://rpc-cosmoshub.keplr.app", "https://mintscan.io/cosmos", "ATOM", 6, 7.0),
            ("near", "NEAR Protocol", "NEAR", "near", -1, "https://rpc.mainnet.near.org", "https://explorer.near.org", "NEAR", 24, 1.0),
            ("algorand", "Algorand", "ALGO", "other", -1, "https://mainnet-api.algorand.network", "https://algoexplorer.cc", "ALGO", 6, 3.0),
            ("osmosis", "Osmosis", "OSMO", "cosmos", -1, "https://rpc-osmosis.keplr.app", "https://mintscan.io/osmosis", "OSMO", 6, 5.0),
            ("juno", "Juno", "JUNO", "cosmos", -1, "https://rpc.juno.kingnodes.com", "https://mintscan.io/juno", "JUNO", 6, 7.0),
            ("injective", "Injective", "INJ", "cosmos", -1, "https://public.injective.network", "https://explorer.injective.network", "INJ", 18, 2.0),
            ("sei", "Sei", "SEI", "cosmos", -1, "https://rpc.sei.io", "https://seistats.io", "SEI", 6, 0.4),
            ("radix", "Radix", "XRD", "other", -1, "https://mainnet.radixdlt.com", "https://explorer.radixdlt.com", "XRD", 10, 2.0),
            ("flow", "Flow", "FLOW", "other", -1, "https://flow-evm.g.alchemy.com/v2/demo", "https://flowdiver.io", "FLOW", 8, 2.0),
            ("hedera", "Hedera", "HBAR", "other", -1, "https://mainnet.mirror.hedera.com/api/v1/contracts/call", "https://hashscan.io", "HBAR", 8, 3.0),
            ("icon", "ICON", "ICX", "other", -1, "https://ctz.solidwallet.io", "https://tracker.icon.community", "ICX", 18, 2.0),
            ("vechain", "VeChain", "VET", "other", -1, "https://mainnet.vechain.org", "https://explore.vechain.org", "VET", 18, 6.0),
            ("theta", "Theta", "THETA", "other", -1, "https://eth-rpc-api.thetatoken.org/rest", "https://explorer.thetatoken.org", "THETA", 18, 10.0),
            ("multiversx", "MultiversX", "EGLD", "other", -1, "https://api.multiversx.com", "https://explorer.multiversx.com", "EGLD", 18, 6.0),
            ("polkadot", "Polkadot", "DOT", "other", -1, "https://rpc.polkadot.io", "https://polkadot.subscan.io", "DOT", 10, 12.0),
            ("kusama", "Kusama", "KSM", "other", -1, "https://kusama-rpc.polkadot.io", "https://kusama.subscan.io", "KSM", 12, 6.0),
            ("kadena", "Kadena", "KDA", "other", -1, "https://api.chainweb.com", "https://explorer.kadena.io", "KDA", 12, 1.0),
            ("casper", "Casper", "CSPR", "other", -1, "https://rpc.mainnet.casper.network", "https://cspr.live", "CSPR", 9, 60.0),
            ("fuel", "Fuel", "FUEL", "other", -1, "https://mainnet.fuel.network", "https://fuelscan.io", "FUEL", 18, 2.0),
            ("tron", "Tron", "TRX", "other", -1, "https://api.trongrid.io", "https://tronscan.org", "TRX", 6, 3.0),
            ("stellar", "Stellar", "XLM", "other", -1, "https://horizon.stellar.org", "https://stellar.expert", "XLM", 7, 5.0),
        ];

        for (id, name, symbol, category, chain_id, rpc, explorer, native, decimals, block_time) in non_evm_chains {
            let cat = match category {
                "solana" => ChainCategory::Solana,
                "aptos" => ChainCategory::Aptos,
                "sui" => ChainCategory::Sui,
                "ton" => ChainCategory::Ton,
                "cosmos" => ChainCategory::Cosmos,
                "near" => ChainCategory::Near,
                _ => ChainCategory::Other,
            };
            
            chains.insert(id.to_string(), ChainConfig {
                id: id.to_string(),
                name: name.to_string(),
                symbol: symbol.to_string(),
                category: cat,
                status: ChainStatus::Active,
                chain_id,
                rpc_urls: vec![rpc.to_string()],
                explorer_urls: vec![explorer.to_string()],
                native_currency: NativeCurrency { name: native.to_string(), symbol: symbol.to_string(), decimals },
                block_time,
                supports_eip1559: false,
            });
        }
    }

    fn initialize_tokens(&self) {
        let mut tokens = self.tokens.write().unwrap();
        
        // Tiger Ecosystem Tokens
        tokens.insert("TGR".to_string(), TokenConfig {
            address: "0x0000000000000000000000000000000000000000".to_string(),
            chain_id: "13000".to_string(),
            symbol: "TGR".to_string(),
            name: "Tiger Coin".to_string(),
            decimals: 18,
            is_native: true,
            is_stable: false,
            is_wrapped: false,
            wrapped_of: None,
        });

        tokens.insert("RUSD".to_string(), TokenConfig {
            address: "0x0000000000000000000000000000000000000001".to_string(),
            chain_id: "13000".to_string(),
            symbol: "RUSD".to_string(),
            name: "Royal Tiger United States Dollar".to_string(),
            decimals: 18,
            is_native: false,
            is_stable: true,
            is_wrapped: false,
            wrapped_of: None,
        });

        // Major tokens
        let major_tokens = vec![
            ("ETH", "Ethereum", 1, true, false),
            ("BNB", "BNB", 56, true, false),
            ("MATIC", "Polygon", 137, true, false),
            ("AVAX", "Avalanche", 43114, true, false),
            ("ARB", "Arbitrum", 42161, true, false),
            ("OP", "Optimism", 10, true, false),
            ("USDT", "Tether USD", 1, false, true),
            ("USDC", "USD Coin", 1, false, true),
            ("DAI", "Dai Stablecoin", 1, false, true),
            ("WBTC", "Wrapped Bitcoin", 1, false, false),
            ("LINK", "Chainlink", 1, false, false),
            ("UNI", "Uniswap", 1, false, false),
            ("AAVE", "Aave", 1, false, false),
            ("MKR", "Maker", 1, false, false),
            ("CRV", "Curve DAO", 1, false, false),
            ("LDO", "Lido DAO", 1, false, false),
            ("SNX", "Synthetix", 1, false, false),
            ("COMP", "Compound", 1, false, false),
            ("SUSHI", "SushiSwap", 1, false, false),
        ];

        for (symbol, name, chain_id, is_native, is_stable) in major_tokens {
            tokens.insert(symbol.to_string(), TokenConfig {
                address: if is_native { "0x0000000000000000000000000000000000000000".to_string() } else { format!("0x{:0>40}", "") },
                chain_id: chain_id.to_string(),
                symbol: symbol.to_string(),
                name: name.to_string(),
                decimals: 18,
                is_native,
                is_stable,
                is_wrapped: false,
                wrapped_of: None,
            });
        }
    }

    fn initialize_dex_pools(&self) {
        let mut pools = self.pools.write().unwrap();
        
        let dex_pools = vec![
            ("TGR", "USDT", 1000000000000000000i64, 500000000000i64, 30i32, 1000000000000000000i64, 25i32),
            ("TGR", "RUSD", 500000000000000000i64, 500000000000000000i64, 30i32, 500000000000000000i64, 30i32),
            ("TGR", "ETH", 1000000000000000000i64, 500000000000000000000i64, 30i32, 500000000000000000i64, 20i32),
            ("RUSD", "USDT", 1000000000000000000i64, 1000000000000000i64, 10i32, 1000000000000000000i64, 10i32),
            ("ETH", "USDT", 1000000000000000000i64, 3000000000000000i64, 30i32, 1000000000000000000i64, 15i32),
            ("BTC", "USDT", 10000000000i64, 5000000000000i64, 30i32, 10000000000i64, 12i32),
            ("ETH", "BTC", 500000000000000000i64, 1000000000i64, 30i32, 1000000000i64, 18i32),
        ];

        for (token_a, token_b, reserve_a, reserve_b, fee, liquidity, apy) in dex_pools {
            let key = format!("{}-{}", token_a, token_b);
            pools.insert(key, LiquidityPool {
                token_a: token_a.to_string(),
                token_b: token_b.to_string(),
                reserve_a,
                reserve_b,
                fee,
                liquidity,
                apy,
            });
        }
    }

    fn initialize_farms(&self) {
        let mut farms = self.farms.write().unwrap();
        
        let now = current_time_millis();
        
        let farm_data = vec![
            ("TGR-USDT", "TGR", "TGR-USDT", 25i32),
            ("TGR-ETH", "TGR", "TGR-ETH", 20i32),
            ("RUSD-USDT", "TGR", "RUSD-USDT", 15i32),
        ];

        for (pool_id, reward_token, staked_token, apy) in farm_data {
            farms.insert(pool_id.to_string(), FarmInfo {
                pool_id: pool_id.to_string(),
                reward_token: reward_token.to_string(),
                staked_token: staked_token.to_string(),
                staked_amount: 0,
                reward_amount: 0,
                apy,
                start_time: now,
                end_time: now + (365 * 24 * 60 * 60 * 1000),
            });
        }
    }

    fn initialize_bridges(&self) {
        let mut bridges = self.bridges.write().unwrap();
        
        let bridge_data = vec![
            ("eth-bsc", "ethereum", "bsc", "*", 10i32, 600000i32, true),
            ("eth-polygon", "ethereum", "polygon", "*", 10i32, 900000i32, true),
            ("eth-arbitrum", "ethereum", "arbitrum", "*", 15i32, 1200000i32, true),
            ("eth-optimism", "ethereum", "optimism", "*", 15i32, 900000i32, true),
            ("eth-avalanche", "ethereum", "avalanche", "*", 10i32, 600000i32, true),
            ("bsc-polygon", "bsc", "polygon", "*", 10i32, 600000i32, true),
            ("tgr-eth", "tigersmartchain", "ethereum", "TGR", 10i32, 1800000i32, true),
            ("tgr-bsc", "tigersmartchain", "bsc", "TGR", 10i32, 1200000i32, true),
            ("rusd-eth", "tigersmartchain", "ethereum", "RUSD", 10i32, 1800000i32, true),
        ];

        for (bridge_id, source, target, token, fee, estimated_time, is_active) in bridge_data {
            bridges.insert(bridge_id.to_string(), BridgeInfo {
                bridge_id: bridge_id.to_string(),
                source_chain: source.to_string(),
                target_chain: target.to_string(),
                token: token.to_string(),
                min_amount: 10000000000000000,
                max_amount: 1000000000000000000000,
                fee,
                estimated_time,
                is_active,
            });
        }
    }

    // ============================================================================
    // Public API Methods
    // ============================================================================

    pub fn get_evm_chains(&self) -> Vec<ChainConfig> {
        let chains = self.evm_chains.read().unwrap();
        chains.values().cloned().collect()
    }

    pub fn get_non_evm_chains(&self) -> Vec<ChainConfig> {
        let chains = self.non_evm_chains.read().unwrap();
        chains.values().cloned().collect()
    }

    pub fn get_tokens(&self) -> Vec<TokenConfig> {
        let tokens = self.tokens.read().unwrap();
        tokens.values().cloned().collect()
    }

    pub fn get_pools(&self) -> Vec<LiquidityPool> {
        let pools = self.pools.read().unwrap();
        pools.values().cloned().collect()
    }

    pub fn get_farms(&self) -> Vec<FarmInfo> {
        let farms = self.farms.read().unwrap();
        farms.values().cloned().collect()
    }

    pub fn get_bridges(&self) -> Vec<BridgeInfo> {
        let bridges = self.bridges.read().unwrap();
        bridges.values().cloned().collect()
    }

    pub fn get_product(&self, product: &str) -> Option<Product> {
        match product {
            "wallet" => Some(self.tiger_wallet.clone()),
            "swap" => Some(self.tiger_swap.clone()),
            "smartchain" => Some(self.tiger_chain.clone()),
            "ex" => Some(self.tiger_ex.clone()),
            _ => None,
        }
    }

    pub fn get_stats(&self) -> HashMap<String, i64> {
        let mut stats = HashMap::new();
        
        stats.insert("totalEvmChains".to_string(), self.evm_chains.read().unwrap().len() as i64);
        stats.insert("totalNonEvmChains".to_string(), self.non_evm_chains.read().unwrap().len() as i64);
        stats.insert("totalTokens".to_string(), self.tokens.read().unwrap().len() as i64);
        stats.insert("totalPools".to_string(), self.pools.read().unwrap().len() as i64);
        stats.insert("totalFarms".to_string(), self.farms.read().unwrap().len() as i64);
        stats.insert("totalBridges".to_string(), self.bridges.read().unwrap().len() as i64);
        stats.insert("initialized".to_string(), if *self.initialized.read().unwrap() { 1 } else { 0 });
        
        stats
    }

    pub fn calculate_swap(&self, input_token: &str, output_token: &str, amount_in: i64) -> Result<SwapResult, String> {
        let pools = self.pools.read().unwrap();
        
        let pool_key = format!("{}-{}", input_token, output_token);
        
        if let Some(pool) = pools.get(&pool_key) {
            let reserve_in = pool.reserve_a;
            let reserve_out = pool.reserve_b;
            
            let amount_out = (amount_in * reserve_out) / (reserve_in + amount_in);
            let fee = (pool.fee as i64 * amount_out) / 10000;
            
            return Ok(SwapResult {
                input_token: input_token.to_string(),
                output_token: output_token.to_string(),
                amount_in,
                amount_out: amount_out - fee,
                fee,
                path: vec![input_token.to_string(), output_token.to_string()],
            });
        }
        
        // Try reverse
        let reverse_key = format!("{}-{}", output_token, input_token);
        if let Some(pool) = pools.get(&reverse_key) {
            let reserve_in = pool.reserve_b;
            let reserve_out = pool.reserve_a;
            
            let amount_out = (amount_in * reserve_out) / (reserve_in + amount_in);
            let fee = (pool.fee as i64 * amount_out) / 10000;
            
            return Ok(SwapResult {
                input_token: input_token.to_string(),
                output_token: output_token.to_string(),
                amount_in,
                amount_out: amount_out - fee,
                fee,
                path: vec![input_token.to_string(), output_token.to_string()],
            });
        }
        
        // Multi-hop
        self.calculate_multi_hop_swap(input_token, output_token, amount_in)
    }

    fn calculate_multi_hop_swap(&self, input_token: &str, output_token: &str, amount_in: i64) -> Result<SwapResult, String> {
        let pools = self.pools.read().unwrap();
        
        let hop_token = "USDT";
        let pool1_key = format!("{}-{}", input_token, hop_token);
        let pool2_key = format!("{}-{}", hop_token, output_token);
        
        if let (Some(pool1), (Some(pool2)) = (pools.get(&pool1_key), pools.get(&pool2_key)) {
            let intermediate = (amount_in * pool1.reserve_b) / (pool1.reserve_a + amount_in);
            let fee1 = (pool1.fee as i64 * intermediate) / 10000;
            let after_fee1 = intermediate - fee1;
            
            let amount_out = (after_fee1 * pool2.reserve_b) / (pool2.reserve_a + after_fee1);
            let fee2 = (pool2.fee as i64 * amount_out) / 10000;
            
            return Ok(SwapResult {
                input_token: input_token.to_string(),
                output_token: output_token.to_string(),
                amount_in,
                amount_out: amount_out - fee2,
                fee: fee1 + fee2,
                path: vec![input_token.to_string(), hop_token.to_string(), output_token.to_string()],
            });
        }
        
        Err(format!("No pool found for {}-{}", input_token, output_token))
    }

    pub fn calculate_bridge(&self, source_chain: &str, target_chain: &str, _token: &str, amount: i64) -> Result<BridgeResult, String> {
        let bridges = self.bridges.read().unwrap();
        
        let bridge_key = format!("{}-{}", source_chain, target_chain);
        
        if let Some(bridge) = bridges.get(&bridge_key) {
            if !bridge.is_active {
                return Err(format!("Bridge {} is not active", bridge_key));
            }
            
            if amount < bridge.min_amount {
                return Err(format!("Amount too low. Minimum: {}", bridge.min_amount));
            }
            
            if amount > bridge.max_amount {
                return Err(format!("Amount too high. Maximum: {}", bridge.max_amount));
            }
            
            let fee = (bridge.fee as i64 * amount) / 10000;
            
            return Ok(BridgeResult {
                source_chain: source_chain.to_string(),
                target_chain: target_chain.to_string(),
                amount_sent: amount,
                amount_received: amount - fee,
                fee,
                estimated_time: bridge.estimated_time,
            });
        }
        
        Err(format!("No bridge found from {} to {}", source_chain, target_chain))
    }

    pub fn search_chains(&self, query: &str) -> Vec<ChainConfig> {
        let mut results = Vec::new();
        let query_lower = query.to_lowercase();
        
        let evm = self.evm_chains.read().unwrap();
        for chain in evm.values() {
            if chain.name.to_lowercase().contains(&query_lower) || 
               chain.symbol.to_lowercase().contains(&query_lower) ||
               chain.id.contains(&query_lower) {
                results.push(chain.clone());
            }
        }
        
        let non_evm = self.non_evm_chains.read().unwrap();
        for chain in non_evm.values() {
            if chain.name.to_lowercase().contains(&query_lower) || 
               chain.symbol.to_lowercase().contains(&query_lower) ||
               chain.id.contains(&query_lower) {
                results.push(chain.clone());
            }
        }
        
        results
    }

    pub fn add_evm_chain(&self, config: ChainConfig) -> Result<(), String> {
        let mut chains = self.evm_chains.write().unwrap();
        
        if chains.contains_key(&config.id) {
            return Err(format!("Chain {} already exists", config.id));
        }
        
        chains.insert(config.id.clone(), config);
        Ok(())
    }

    pub fn add_token(&self, config: TokenConfig) -> Result<(), String> {
        let mut tokens = self.tokens.write().unwrap();
        
        if tokens.contains_key(&config.symbol) {
            return Err(format!("Token {} already exists", config.symbol));
        }
        
        tokens.insert(config.symbol.clone(), config);
        Ok(())
    }

    pub fn create_pool(&self, token_a: &str, token_b: &str, fee: i32) -> Result<(), String> {
        let mut pools = self.pools.write().unwrap();
        
        let key = format!("{}-{}", token_a, token_b);
        
        if pools.contains_key(&key) {
            return Err(format!("Pool {} already exists", key));
        }
        
        pools.insert(key, LiquidityPool {
            token_a: token_a.to_string(),
            token_b: token_b.to_string(),
            reserve_a: 0,
            reserve_b: 0,
            fee,
            liquidity: 0,
            apy: 0,
        });
        
        Ok(())
    }

    pub fn create_farm(&self, pool_id: &str, reward_token: &str, apy: i32) -> Result<(), String> {
        let mut farms = self.farms.write().unwrap();
        
        if farms.contains_key(pool_id) {
            return Err(format!("Farm {} already exists", pool_id));
        }
        
        let now = current_time_millis();
        
        farms.insert(pool_id.to_string(), FarmInfo {
            pool_id: pool_id.to_string(),
            reward_token: reward_token.to_string(),
            staked_token: pool_id.to_string(),
            staked_amount: 0,
            reward_amount: 0,
            apy,
            start_time: now,
            end_time: now + (365 * 24 * 60 * 60 * 1000),
        });
        
        Ok(())
    }

    pub fn add_bridge(&self, source_chain: &str, target_chain: &str, token: &str, fee: i32, estimated_time: i32) -> Result<(), String> {
        let mut bridges = self.bridges.write().unwrap();
        
        let bridge_id = format!("{}-{}", source_chain, target_chain);
        
        if bridges.contains_key(&bridge_id) {
            return Err(format!("Bridge {} already exists", bridge_id));
        }
        
        bridges.insert(bridge_id, BridgeInfo {
            bridge_id,
            source_chain: source_chain.to_string(),
            target_chain: target_chain.to_string(),
            token: token.to_string(),
            min_amount: 10000000000000000,
            max_amount: 1000000000000000000000,
            fee,
            estimated_time,
            is_active: true,
        });
        
        Ok(())
    }

    pub fn collect_fee(&self, amount: i64, source: &str) {
        let mut total = self.total_fees_collected.write().unwrap();
        *total += amount;
        
        let mut history = self.fee_history.write().unwrap();
        history.push(FeeRecord {
            amount,
            source: source.to_string(),
            timestamp: current_time_millis(),
        });
    }
}

// ============================================================================
// Helper Functions
// ============================================================================

fn current_time_millis() -> i64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap()
        .as_millis() as i64
}

// ============================================================================
// Main Function (for testing)
// ============================================================================

fn main() {
    println!("=== TigerEX Rust Integration ===");
    
    let ex = TigerEX::new();
    ex.initialize();
    
    // Test stats
    let stats = ex.get_stats();
    println!("\nPlatform Statistics:");
    println!("  EVM Chains: {}", stats.get("totalEvmChains").unwrap_or(&0));
    println!("  Non-EVM Chains: {}", stats.get("totalNonEvmChains").unwrap_or(&0));
    println!("  Tokens: {}", stats.get("totalTokens").unwrap_or(&0));
    println!("  DEX Pools: {}", stats.get("totalPools").unwrap_or(&0));
    println!("  Farms: {}", stats.get("totalFarms").unwrap_or(&0));
    println!("  Bridges: {}", stats.get("totalBridges").unwrap_or(&0));
    println!("  Initialized: {}", stats.get("initialized").unwrap_or(&0));
    
    // Test products
    println!("\nProducts:");
    if let Some(wallet) = ex.get_product("wallet") {
        println!("  TigerWallet: {} v{} - {}", wallet.name, wallet.version, wallet.status);
    }
    if let Some(swap) = ex.get_product("swap") {
        println!("  Tigerswap: {} v{} - {}", swap.name, swap.version, swap.status);
    }
    if let Some(chain) = ex.get_product("smartchain") {
        println!("  TigerSmartChain: {} v{} - {}", chain.name, chain.version, chain.status);
    }
    if let Some(ex) = ex.get_product("ex") {
        println!("  TigerEx: {} v{} - {}", ex.name, ex.version, ex.status);
    }
    
    // Test swap
    println!("\nSwap Test (100 USDT to TGR):");
    match ex.calculate_swap("USDT", "TGR", 100000000) {
        Ok(result) => {
            println!("  Input: {} USDT", result.amount_in);
            println!("  Output: {} TGR", result.amount_out);
            println!("  Fee: {} TGR", result.fee);
            println!("  Path: {:?}", result.path);
        }
        Err(e) => println!("  Error: {}", e),
    }
    
    // Test bridge
    println!("\nBridge Test (ETH to BSC):");
    match ex.calculate_bridge("ethereum", "bsc", "ETH", 1000000000000000000) {
        Ok(result) => {
            println!("  Sent: {} ETH", result.amount_sent);
            println!("  Received: {} ETH", result.amount_received);
            println!("  Fee: {} ETH", result.fee);
            println!("  Time: {}ms", result.estimated_time);
        }
        Err(e) => println!("  Error: {}", e),
    }
    
    // Test chain search
    println!("\nChain Search (Polygon):");
    let chains = ex.search_chains("Polygon");
    println!("  Found: {} chains", chains.len());
    for chain in chains.iter().take(3) {
        println!("    - {} ({}) - Chain ID: {}", chain.name, chain.symbol, chain.chain_id);
    }
    
    println!("\n=== Test Complete ===");
}