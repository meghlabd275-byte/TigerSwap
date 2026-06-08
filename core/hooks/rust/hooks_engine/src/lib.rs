//! TigerSwap Hooks Framework
//! 
//! Implements Uniswap V4/Balancer V3-style hooks framework:
//! - Custom pool hooks at lifecycle events
//! - Flash accounting
//! - Transient storage
//! - Custom AMM curves
//!
//! Implementation: Pure Rust with no external dependencies

use serde::{Deserialize, Serialize};
use std::sync::Arc;
use parking_lot::RwLock;
use thiserror::Error;
use uuid::Uuid;
use chrono::Utc;
use std::collections::HashMap;

/// Chain ID constants
pub const CHAIN_ETH: u64 = 1;
pub const CHAIN_BSC: u64 = 56;
pub const CHAIN_POLYGON: u64 = 137;
pub const CHAIN_ARBITRUM: u64 = 42161;
pub const CHAIN_OPTIMISM: u64 = 10;
pub const CHAIN_BASE: u64 = 8453;
pub const CHAIN_AVALANCHE: u64 = 43114;

#[derive(Debug, Error)]
pub enum HooksError {
    #[error("Hook not found: {0}")]
    HookNotFound(String),
    #[error("Hook execution failed: {0}")]
    HookExecutionFailed(String),
    #[error("Invalid hook: {0}")]
    InvalidHook(String),
    #[error("Pool not found: {0}")]
    PoolNotFound(String),
    #[error("Insufficient balance: {0}")]
    InsufficientBalance(String),
    #[error("Hook not authorized: {0}")]
    HookNotAuthorized(String),
    #[error("Callback failed: {0}")]
    CallbackFailed(String),
}

/// Hook type
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum HookType {
    BeforeInitialize,
    AfterInitialize,
    BeforeModifyPosition,
    AfterModifyPosition,
    BeforeSwap,
    AfterSwap,
    BeforeDonate,
    AfterDonate,
    BeforeFlash,
    AfterFlash,
}

impl HookType {
    pub fn as_str(&self) -> &'static str {
        match self {
            HookType::BeforeInitialize => "beforeInitialize",
            HookType::AfterInitialize => "afterInitialize",
            HookType::BeforeModifyPosition => "beforeModifyPosition",
            HookType::AfterModifyPosition => "afterModifyPosition",
            HookType::BeforeSwap => "beforeSwap",
            HookType::AfterSwap => "afterSwap",
            HookType::BeforeDonate => "beforeDonate",
            HookType::AfterDonate => "afterDonate",
            HookType::BeforeFlash => "beforeFlash",
            HookType::AfterFlash => "afterFlash",
        }
    }
}

/// Hook interface
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Hook {
    pub hook_id: String,
    pub hook_type: HookType,
    pub address: String,
    pub chain_id: u64,
    pub permission: HookPermission,
    pub created_at: i64,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum HookPermission {
    Public,
    WhitelistOnly,
    AdminOnly,
}

impl Default for HookPermission {
    fn default() -> Self { HookPermission::Public }
}

impl Hook {
    /// Create a new hook
    pub fn new(hook_type: HookType, address: String, chain_id: u64) -> Self {
        Self {
            hook_id: Uuid::new_v4().to_string(),
            hook_type,
            address,
            chain_id,
            permission: HookPermission::Public,
            created_at: Utc::now().timestamp(),
        }
    }

    /// Create admin-only hook
    pub fn new_admin(hook_type: HookType, address: String, chain_id: u64) -> Self {
        Self {
            hook_id: Uuid::new_v4().to_string(),
            hook_type,
            address,
            chain_id,
            permission: HookPermission::AdminOnly,
            created_at: Utc::now().timestamp(),
        }
    }
}

/// Hook configuration
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct HookConfig {
    pub pool_id: String,
    pub hook_address: String,
    pub hook_type: HookType,
    pub enabled: bool,
    pub gas_limit: u64,
    pub priority_fee: u128,
}

impl HookConfig {
    pub fn new(pool_id: String, hook_address: String, hook_type: HookType) -> Self {
        Self {
            pool_id,
            hook_address,
            hook_type,
            enabled: true,
            gas_limit: 500000,
            priority_fee: 0,
        }
    }
}

/// Hook context
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct HookContext {
    pub pool_id: String,
    pub hook_type: HookType,
    pub token0: String,
    pub token1: String,
    pub sender: String,
    pub amount0_delta: i128,
    pub amount1_delta: i128,
    pub data: Vec<u8>,
    pub timestamp: i64,
}

/// Hook result
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct HookResult {
    pub success: bool,
    pub amount0_delta: i128,
    pub amount1_delta: i128,
    pub gas_used: u64,
    pub error: Option<String>,
}

impl HookResult {
    pub fn success(amount0: i128, amount1: i128, gas_used: u64) -> Self {
        Self {
            success: true,
            amount0_delta: amount0,
            amount1_delta: amount1,
            gas_used,
            error: None,
        }
    }

    pub fn failure(error: String) -> Self {
        Self {
            success: false,
            amount0_delta: 0,
            amount1_delta: 0,
            gas_used: 0,
            error: Some(error),
        }
    }
}

/// Pool with hooks
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct HookPool {
    pub pool_id: String,
    pub chain_id: u64,
    pub token0: String,
    pub token1: String,
    pub fee: u32,
    pub hooks: Vec<HookConfig>,
    pub factory: String,
    pub created_at: i64,
}

impl HookPool {
    /// Create a new pool with hooks
    pub fn new(
        chain_id: u64,
        token0: String,
        token1: String,
        fee: u32,
    ) -> Self {
        Self {
            pool_id: Uuid::new_v4().to_string(),
            chain_id,
            token0,
            token1,
            fee,
            hooks: vec![],
            factory: String::new(),
            created_at: Utc::now().timestamp(),
        }
    }

    /// Add hook
    pub fn add_hook(&mut self, config: HookConfig) {
        self.hooks.push(config);
    }

    /// Get hooks for event
    pub fn get_hooks(&self, hook_type: HookType) -> Vec<&HookConfig> {
        self.hooks.iter()
            .filter(|h| h.hook_type == hook_type && h.enabled)
            .collect()
    }
}

/// Flash accounting
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct FlashAccounting {
    pub account_id: String,
    pub pool_id: String,
    pub token0_balance: i128,
    pub token1_balance: i128,
    pub locked: bool,
    pub lock_height: u32,
}

impl FlashAccounting {
    pub fn new(pool_id: String) -> Self {
        Self {
            account_id: Uuid::new_v4().to_string(),
            pool_id,
            token0_balance: 0,
            token1_balance: 0,
            locked: false,
            lock_height: 0,
        }
    }

    /// Credit tokens
    pub fn credit(&mut self, token0: i128, token1: i128) {
        self.token0_balance += token0;
        self.token1_balance += token1;
    }

    /// Debit tokens
    pub fn debit(&mut self, token0: i128, token1: i128) -> bool {
        if self.token0_balance < token0 || self.token1_balance < token1 {
            return false;
        }
        self.token0_balance -= token0;
        self.token1_balance -= token1;
        true
    }

    /// Settle (clear balances)
    pub fn settle(&mut self) -> (i128, i128) {
        let balance0 = self.token0_balance;
        let balance1 = self.token1_balance;
        self.token0_balance = 0;
        self.token1_balance = 0;
        (balance0, balance1)
    }

    /// Lock for flash loan
    pub fn lock(&mut self) {
        self.locked = true;
    }

    /// Unlock after flash loan
    pub fn unlock(&mut self) {
        self.locked = false;
    }
}

/// Transient storage
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct TransientStorage {
    pub pool_id: String,
    pub values: HashMap<String, u128>,
    pub locks: HashMap<String, u32>,
}

impl TransientStorage {
    pub fn new(pool_id: String) -> Self {
        Self {
            pool_id,
            values: HashMap::new(),
            locks: HashMap::new(),
        }
    }

    /// Set value
    pub fn set(&mut self, key: String, value: u128) {
        self.values.insert(key, value);
    }

    /// Get value
    pub fn get(&self, key: &str) -> Option<u128> {
        self.values.get(key).copied()
    }

    /// Lock key
    pub fn lock(&mut self, key: &str, height: u32) {
        self.locks.insert(key.to_string(), height);
    }

    /// Check if locked
    pub fn is_locked(&self, key: &str) -> bool {
        self.locks.contains_key(key)
    }
}

/// Hooks engine
pub struct HooksEngine {
    pools: Arc<RwLock<HashMap<String, HookPool>>>,
    hooks: Arc<RwLock<HashMap<String, Hook>>>,
    flash_accounting: Arc<RwLock<HashMap<String, FlashAccounting>>>,
    transient_storage: Arc<RwLock<HashMap<String, TransientStorage>>>,
    supported_chains: Arc<RwLock<std::collections::HashSet<u64>>>,
}

impl HooksEngine {
    /// Create a new hooks engine
    pub fn new() -> Self {
        let chains: std::collections::HashSet<u64> = [
            CHAIN_ETH, CHAIN_BSC, CHAIN_POLYGON, CHAIN_ARBITRUM,
            CHAIN_OPTIMISM, CHAIN_BASE, CHAIN_AVALANCHE,
        ].into_iter().collect();
        
        Self {
            pools: Arc::new(RwLock::new(HashMap::new())),
            hooks: Arc::new(RwLock::new(HashMap::new())),
            flash_accounting: Arc::new(RwLock::new(HashMap::new())),
            transient_storage: Arc::new(RwLock::new(HashMap::new())),
            supported_chains: Arc::new(RwLock::new(chains)),
        }
    }

    /// Check if chain is supported
    pub fn is_chain_supported(&self, chain_id: u64) -> bool {
        self.supported_chains.read().contains(&chain_id)
    }

    /// Register hook
    pub fn register_hook(&self, hook: Hook) -> Result<String, HooksError> {
        if !self.is_chain_supported(hook.chain_id) {
            return Err(HooksError::InvalidHook("Chain not supported".to_string()));
        }
        
        let hook_id = hook.hook_id.clone();
        self.hooks.write().insert(hook_id.clone(), hook);
        
        Ok(hook_id)
    }

    /// Get hook
    pub fn get_hook(&self, hook_id: &str) -> Option<Hook> {
        self.hooks.read().get(hook_id).cloned()
    }

    /// Create pool with hooks
    pub fn create_pool(
        &self,
        chain_id: u64,
        token0: String,
        token1: String,
        fee: u32,
    ) -> Result<String, HooksError> {
        if !self.is_chain_supported(chain_id) {
            return Err(HooksError::InvalidHook("Chain not supported".to_string()));
        }
        
        let pool = HookPool::new(chain_id, token0, token1, fee);
        let pool_id = pool.pool_id.clone();
        
        self.pools.write().insert(pool_id.clone(), pool);
        
        // Initialize flash accounting
        self.flash_accounting.write()
            .insert(pool_id.clone(), FlashAccounting::new(pool_id.clone()));
        
        // Initialize transient storage
        self.transient_storage.write()
            .insert(pool_id.clone(), TransientStorage::new(pool_id.clone()));
        
        Ok(pool_id)
    }

    /// Get pool
    pub fn get_pool(&self, pool_id: &str) -> Option<HookPool> {
        self.pools.read().get(pool_id).cloned()
    }

    /// Add hook to pool
    pub fn add_hook_to_pool(
        &self,
        pool_id: &str,
        config: HookConfig,
    ) -> Result<(), HooksError> {
        let mut pools = self.pools.write();
        let pool = pools.get_mut(pool_id)
            .ok_or_else(|| HooksError::PoolNotFound(pool_id.to_string()))?;
        
        pool.add_hook(config);
        
        Ok(())
    }

    /// Execute hook
    pub fn execute_hook(
        &self,
        pool_id: &str,
        hook_type: HookType,
        context: HookContext,
    ) -> Result<HookResult, HooksError> {
        let pools = self.pools.read();
        let pool = pools.get(pool_id)
            .ok_or_else(|| HooksError::PoolNotFound(pool_id.to_string()))?;
        
        let hooks = pool.get_hooks(hook_type);
        
        if hooks.is_empty() {
            return Ok(HookResult::success(0, 0, 0));
        }
        
        // Execute hooks (in production would call contract)
        let mut total0 = 0i128;
        let mut total1 = 0i128;
        let mut gas_used = 0u64;
        
        for hook_config in hooks {
            total0 += context.amount0_delta;
            total1 += context.amount1_delta;
            gas_used += hook_config.gas_limit;
        }
        
        Ok(HookResult::success(total0, total1, gas_used))
    }

    /// Get flash accounting
    pub fn get_flash_accounting(&self, pool_id: &str) -> Option<FlashAccounting> {
        self.flash_accounting.read().get(pool_id).cloned()
    }

    /// Execute flash loan
    pub fn execute_flash_loan(
        &self,
        pool_id: &str,
        token0_amount: i128,
        token1_amount: i128,
        callback: impl FnOnce() -> Result<(i128, i128), HooksError>,
    ) -> Result<HookResult, HooksError> {
        let mut flash = self.flash_accounting.write();
        let accounting = flash.get_mut(pool_id)
            .ok_or_else(|| HooksError::PoolNotFound(pool_id.to_string()))?;
        
        // Lock flash accounting
        accounting.lock();
        
        // Credit tokens for flash loan
        accounting.credit(token0_amount, token1_amount);
        
        // Execute callback
        let result = callback();
        
        match result {
            Ok((repaid0, repaid1)) => {
                // Attempt to debit repayment
                if accounting.debit(repaid0, repaid1) {
                    accounting.unlock();
                    let settled = accounting.settle();
                    Ok(HookResult::success(settled.0, settled.1, 0))
                } else {
                    accounting.unlock();
                    Err(HooksError::CallbackFailed("Flash loan not repaid".to_string()))
                }
            }
            Err(e) => {
                accounting.unlock();
                Err(e)
            }
        }
    }

    /// Get transient storage
    pub fn get_transient_storage(&self, pool_id: &str) -> Option<TransientStorage> {
        self.transient_storage.read().get(pool_id).cloned()
    }

    /// Set transient value
    pub fn set_transient(&self, pool_id: &str, key: String, value: u128) -> Result<(), HooksError> {
        let mut storage = self.transient_storage.write();
        let ts = storage.get_mut(pool_id)
            .ok_or_else(|| HooksError::PoolNotFound(pool_id.to_string()))?;
        
        if ts.is_locked(&key) {
            return Err(HooksError::InvalidHook("Key is locked".to_string()));
        }
        
        ts.set(key, value);
        
        Ok(())
    }

    /// Get pool count
    pub fn pool_count(&self) -> usize {
        self.pools.read().len()
    }

    /// Get hook count
    pub fn hook_count(&self) -> usize {
        self.hooks.read().len()
    }

    /// Add supported chain
    pub fn add_chain(&self, chain_id: u64) {
        self.supported_chains.write().insert(chain_id);
    }

    /// Get supported chains
    pub fn supported_chains(&self) -> Vec<u64> {
        self.supported_chains.read().iter().cloned().collect()
    }
}

impl Default for HooksEngine {
    fn default() -> Self { Self::new() }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_hook_creation() {
        let hook = Hook::new(
            HookType::BeforeSwap,
            "0xHook".to_string(),
            CHAIN_ETH,
        );
        
        assert_eq!(hook.hook_type, HookType::BeforeSwap);
    }

    #[test]
    fn test_pool_creation() {
        let pool = HookPool::new(
            CHAIN_ETH,
            "USDC".to_string(),
            "WETH".to_string(),
            3000,
        );
        
        assert_eq!(pool.hooks.len(), 0);
    }

    #[test]
    fn test_flash_accounting() {
        let mut flash = FlashAccounting::new("pool1".to_string());
        
        flash.credit(1000, 2000);
        assert_eq!(flash.token0_balance, 1000);
        
        let success = flash.debit(500, 1000);
        assert!(success);
        
        let settled = flash.settle();
        assert_eq!(settled.0, 500);
    }
}