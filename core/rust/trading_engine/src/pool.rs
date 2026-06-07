use std::sync::Arc;
use parking_lot::RwLock;
use std::collections::HashMap;

#[derive(Debug, Clone, Default)]
pub struct PoolManager {
    pools: Arc<RwLock<HashMap<String, PoolState>>>,
}

impl PoolManager {
    pub fn new() -> Self { Self { pools: Arc::new(RwLock::new(HashMap::new())) } }
    pub fn register(&self, address: String, token0: String, token1: String) { self.pools.write().insert(address, PoolState { token0, token1, ..Default::default() }); }
    pub fn get(&self, address: &str) -> Option<PoolState> { self.pools.read().get(address).cloned() }
    pub fn count(&self) -> usize { self.pools.read().len() }
}

#[derive(Debug, Clone, Default)]
pub struct PoolState {
    pub token0: String,
    pub token1: String,
    pub reserve0: u128,
    pub reserve1: u128,
}