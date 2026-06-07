use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use std::sync::Arc;
use parking_lot::RwLock;

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum ChainType { EVM, Solana, Cosmos, Aptos, Sui }

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ChainConfig {
    pub chain_id: u64,
    pub name: String,
    pub symbol: String,
    pub chain_type: ChainType,
    pub rpc_url: String,
    pub explorer_url: String,
    pub confirmations: u64,
}

impl ChainConfig {
    pub fn new(chain_id: u64, name: &str, symbol: &str, chain_type: ChainType) -> Self {
        Self { chain_id, name: name.to_string(), symbol: symbol.to_string(), chain_type, rpc_url: String::new(), explorer_url: String::new(), confirmations: 12 }
    }
}

pub struct ChainRegistry {
    chains: Arc<RwLock<HashMap<u64, ChainConfig>>>,
}

impl ChainRegistry {
    pub fn new() -> Self {
        let registry = Self { chains: Arc::new(RwLock::new(HashMap::new())) };
        let mut chains = registry.chains.write();
        chains.insert(1, ChainConfig::new(1, "Ethereum", "ETH", ChainType::EVM));
        chains.insert(56, ChainConfig::new(56, "BNB Chain", "BNB", ChainType::EVM));
        chains.insert(137, ChainConfig::new(137, "Polygon", "MATIC", ChainType::EVM));
        chains.insert(42161, ChainConfig::new(42161, "Arbitrum", "ETH", ChainType::EVM));
        chains.insert(10, ChainConfig::new(10, "Optimism", "ETH", ChainType::EVM));
        chains.insert(8453, ChainConfig::new(8453, "Base", "ETH", ChainType::EVM));
        chains.insert(43114, ChainConfig::new(43114, "Avalanche", "AVAX", ChainType::EVM));
        chains.insert(0, ChainConfig::new(0, "Solana", "SOL", ChainType::Solana));
        drop(chains);
        registry
    }

    pub fn get(&self, chain_id: u64) -> Option<ChainConfig> { self.chains.read().get(&chain_id).cloned() }
    pub fn all(&self) -> Vec<ChainConfig> { self.chains.read().values().cloned().collect() }
    pub fn is_supported(&self, chain_id: u64) -> bool { self.chains.read().contains_key(&chain_id) }
    pub fn count(&self) -> usize { self.chains.read().len() }
}

impl Default for ChainRegistry { fn default() -> Self { Self::new() } }