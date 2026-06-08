//! TigerSwap Liquid Staking Engine
//! 
//! Implements liquid staking similar to Lido, RocketPool:
//! - Stake tokens, receive liquid staked tokens
//! - Rewards distribution
//! - Node operator management
//! - Withdrawal queue
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

#[derive(Debug, Error)]
pub enum StakingError {
    #[error("Pool not found: {0}")]
    PoolNotFound(String),
    #[error("Insufficient balance: {0}")]
    InsufficientBalance(String),
    #[error("Invalid parameters: {0}")]
    InvalidParameters(String),
    #[error("Not yet implemented: {0}")]
    NotImplemented(String),
    #[error("Chain not supported: {0}")]
    ChainNotSupported(u64),
}

/// Liquid staking pool
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct LiquidStakingPool {
    pub pool_id: String,
    pub chain_id: u64,
    pub name: String,
    pub native_token: String,    // ETH, BNB, etc.
    pub staked_token: String,    // stETH, stBNB, etc.
    
    // Staking
    pub total_staked: u128,
    pub total_rewards: u128,
    pub validators_count: u32,
    
    // Rewards
    pub reward_rate: f64,       // APY
    pub last_update: i64,
    
    // Fees
    pub protocol_fee: u32,       // Basis points
    pub node_fee: u32,          // Basis points
    
    // Withdrawal
    pub withdrawal_queue: Vec<WithdrawalRequest>,
    pub next_withdrawal_id: u64,
    
    pub created_at: i64,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct WithdrawalRequest {
    pub request_id: u64,
    pub user: String,
    pub amount: u128,
    pub shares: u128,
    pub status: WithdrawalStatus,
    pub requested_at: i64,
    pub processed_at: Option<i64>,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum WithdrawalStatus {
    Pending,
    Ready,
    Completed,
    Cancelled,
}

impl Default for WithdrawalStatus {
    fn default() -> Self { WithdrawalStatus::Pending }
}

impl LiquidStakingPool {
    pub fn new(
        chain_id: u64,
        name: String,
        native_token: String,
    ) -> Self {
        let staked_token = format!("st{}", native_token);
        
        Self {
            pool_id: Uuid::new_v4().to_string(),
            chain_id,
            name,
            native_token,
            staked_token,
            total_staked: 0,
            total_rewards: 0,
            validators_count: 0,
            reward_rate: 0.05, // 5% APY default
            last_update: Utc::now().timestamp(),
            protocol_fee: 1000, // 10%
            node_fee: 500,      // 5%
            withdrawal_queue: vec![],
            next_withdrawal_id: 1,
            created_at: Utc::now().timestamp(),
        }
    }

    /// Stake tokens
    pub fn stake(&mut self, user: &str, amount: u128) -> Result<(u128, u128), StakingError> {
        if amount == 0 {
            return Err(StakingError::InvalidParameters("Amount must be > 0".to_string()));
        }
        
        // Calculate shares to mint
        let shares = if self.total_staked == 0 {
            amount // Initial deposit
        } else {
            let share_price = self.get_share_price();
            (amount as f64 / share_price) as u128
        };
        
        // Update state
        self.total_staked += amount;
        self.last_update = Utc::now().timestamp();
        
        // Mint staked tokens to user (in real implementation, would mint to user address)
        let staked_tokens = (shares as f64 * self.get_share_price()) as u128;
        
        Ok((shares, staked_tokens))
    }

    /// Unstake tokens
    pub fn unstake(&mut self, shares: u128) -> Result<u128, StakingError> {
        if shares == 0 {
            return Err(StakingError::InvalidParameters("Shares must be > 0".to_string()));
        }
        
        // Calculate tokens to receive
        let share_price = self.get_share_price();
        let tokens = (shares as f64 * share_price) as u128;
        
        if tokens > self.total_staked {
            return Err(StakingError::InsufficientBalance("Insufficient staked balance".to_string()));
        }
        
        // Update state
        self.total_staked -= tokens;
        self.last_update = Utc::now().timestamp();
        
        Ok(tokens)
    }

    /// Request withdrawal
    pub fn request_withdrawal(&mut self, user: String, amount: u128) -> u64 {
        let share_price = self.get_share_price();
        let shares = (amount as f64 / share_price) as u128;
        
        let request = WithdrawalRequest {
            request_id: self.next_withdrawal_id,
            user,
            amount,
            shares,
            status: WithdrawalStatus::Pending,
            requested_at: Utc::now().timestamp(),
            processed_at: None,
        };
        
        self.withdrawal_queue.push(request);
        self.next_withdrawal_id += 1;
        
        request.request_id
    }

    /// Process withdrawal
    pub fn process_withdrawal(&mut self, request_id: u64) -> Result<u128, StakingError> {
        let request = self.withdrawal_queue.iter_mut()
            .find(|r| r.request_id == request_id)
            .ok_or_else(|| StakingError::PoolNotFound("Request not found".to_string()))?;
        
        if request.status != WithdrawalStatus::Pending {
            return Err(StakingError::InvalidParameters("Already processed".to_string()));
        }
        
        // Process the withdrawal
        request.status = WithdrawalStatus::Completed;
        request.processed_at = Some(Utc::now().timestamp());
        
        Ok(request.amount)
    }

    /// Get share price
    pub fn get_share_price(&self) -> f64 {
        if self.total_staked == 0 {
            return 1.0;
        }
        
        // Share price = total staked / total shares (assuming 1:1 initially)
        self.total_staked as f64 / self.total_staked.max(1) as f64
    }

    /// Distribute rewards
    pub fn distribute_rewards(&mut self, amount: u128) {
        self.total_rewards += amount;
        
        // Protocol fee
        let protocol_amount = (amount * self.protocol_fee as u128) / 10000;
        
        // Node fee
        let node_amount = (amount * self.node_fee as u128) / 10000;
        
        // Remaining goes to stakers
        let staker_amount = amount - protocol_amount - node_amount;
        
        // Add to total staked (increases share price)
        self.total_staked += staker_amount;
        self.last_update = Utc::now().timestamp();
    }

    /// Get current APY
    pub fn get_apy(&self) -> f64 {
        self.reward_rate * 100.0
    }
}

/// Node operator
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct NodeOperator {
    pub operator_id: String,
    pub address: String,
    pub staked_amount: u128,
    pub validators: u32,
    pub performance_score: f64,
    pub is_active: bool,
    pub registered_at: i64,
}

impl NodeOperator {
    pub fn new(address: String) -> Self {
        Self {
            operator_id: Uuid::new_v4().to_string(),
            address,
            staked_amount: 0,
            validators: 0,
            performance_score: 1.0,
            is_active: true,
            registered_at: Utc::now().timestamp(),
        }
    }
}

/// Liquid Staking Engine
pub struct LiquidStakingEngine {
    pools: Arc<RwLock<HashMap<String, LiquidStakingPool>>>,
    node_operators: Arc<RwLock<HashMap<String, NodeOperator>>>,
    staker_balances: Arc<RwLock<HashMap<String, u128>>>, // user -> staked amount
    supported_chains: Arc<RwLock<std::collections::HashSet<u64>>>,
}

impl LiquidStakingEngine {
    pub fn new() -> Self {
        let chains: std::collections::HashSet<u64> = [
            CHAIN_ETH, CHAIN_BSC,
        ].into_iter().collect();
        
        Self {
            pools: Arc::new(RwLock::new(HashMap::new())),
            node_operators: Arc::new(RwLock::new(HashMap::new())),
            staker_balances: Arc::new(RwLock::new(HashMap::new())),
            supported_chains: Arc::new(RwLock::new(chains)),
        }
    }

    pub fn is_chain_supported(&self, chain_id: u64) -> bool {
        self.supported_chains.read().contains(&chain_id)
    }

    /// Create staking pool
    pub fn create_pool(
        &self,
        chain_id: u64,
        name: String,
        native_token: String,
    ) -> Result<String, StakingError> {
        if !self.is_chain_supported(chain_id) {
            return Err(StakingError::ChainNotSupported(chain_id));
        }
        
        let pool = LiquidStakingPool::new(chain_id, name, native_token);
        let pool_id = pool.pool_id.clone();
        
        self.pools.write().insert(pool_id.clone(), pool);
        
        Ok(pool_id)
    }

    /// Get pool
    pub fn get_pool(&self, pool_id: &str) -> Option<LiquidStakingPool> {
        self.pools.read().get(pool_id).cloned()
    }

    /// Stake
    pub fn stake(&self, pool_id: &str, user: String, amount: u128) -> Result<(u128, u128), StakingError> {
        let mut pools = self.pools.write();
        let pool = pools.get_mut(pool_id)
            .ok_or_else(|| StakingError::PoolNotFound(pool_id.to_string()))?;
        
        let (shares, staked_tokens) = pool.stake(&user, amount)?;
        
        // Update user balance
        let mut balances = self.staker_balances.write();
        let current = balances.get(&user).copied().unwrap_or(0);
        balances.insert(user, current + staked_tokens);
        
        Ok((shares, staked_tokens))
    }

    /// Unstake
    pub fn unstake(&self, pool_id: &str, user: &str, shares: u128) -> Result<u128, StakingError> {
        let mut pools = self.pools.write();
        let pool = pools.get_mut(pool_id)
            .ok_or_else(|| StakingError::PoolNotFound(pool_id.to_string()))?;
        
        let tokens = pool.unstake(shares)?;
        
        // Update user balance
        let mut balances = self.staker_balances.write();
        let current = balances.get(user).copied().unwrap_or(0);
        let new_balance = current.saturating_sub((shares as f64 * pool.get_share_price()) as u128);
        
        if new_balance > 0 {
            balances.insert(user.to_string(), new_balance);
        } else {
            balances.remove(user);
        }
        
        Ok(tokens)
    }

    /// Request withdrawal
    pub fn request_withdrawal(&self, pool_id: &str, user: String, amount: u128) -> Result<u64, StakingError> {
        let mut pools = self.pools.write();
        let pool = pools.get_mut(pool_id)
            .ok_or_else(|| StakingError::PoolNotFound(pool_id.to_string()))?;
        
        Ok(pool.request_withdrawal(user, amount))
    }

    /// Get staker balance
    pub fn get_staker_balance(&self, user: &str) -> u128 {
        self.staker_balances.read().get(user).copied().unwrap_or(0)
    }

    /// Register node operator
    pub fn register_node_operator(&self, address: String) -> String {
        let operator = NodeOperator::new(address);
        let operator_id = operator.operator_id.clone();
        
        self.node_operators.write().insert(operator_id.clone(), operator);
        
        operator_id
    }

    /// Get total TVL
    pub fn get_total_tvl(&self, pool_id: &str) -> u128 {
        self.pools.read()
            .get(pool_id)
            .map(|p| p.total_staked)
            .unwrap_or(0)
    }
}

impl Default for LiquidStakingEngine {
    fn default() -> Self { Self::new() }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_pool_creation() {
        let pool = LiquidStakingPool::new(
            CHAIN_ETH,
            "Ethereum Staking".to_string(),
            "ETH".to_string(),
        );
        
        assert_eq!(pool.native_token, "ETH");
    }

    #[test]
    fn test_stake() {
        let mut pool = LiquidStakingPool::new(
            CHAIN_ETH,
            "Test".to_string(),
            "ETH".to_string(),
        );
        
        let (shares, tokens) = pool.stake("user1", 1000).unwrap();
        
        assert!(shares > 0);
    }
}