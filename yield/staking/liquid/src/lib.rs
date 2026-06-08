//! TigerSwap Liquid Staking
//! 
//! Liquid staking with derivative tokens:
//! - stTiger (staked Tiger)
//! - Rewards distribution
//! - Unstaking queue
//!
//! Uses Rust for high performance

use std::collections::HashMap;
use std::sync::Arc;
use tokio::sync::RwLock;
use serde::{Deserialize, Serialize};
use uint::construct_uint;

construct_uint! {
    pub struct U256(4);
}

// ==================== STAKING TYPES ====================

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Validator {
    pub id: [u8; 32],
    pub stake: U256,
    pub active: bool,
    pub delegators_count: u32,
    pub commission: u64,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Delegator {
    pub user: [u8; 20],
    pub staked_amount: U256,
    pub accumulated_rewards: U256,
    pub last_checkpoint: u64,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct UnstakeRequest {
    pub id: [u8; 32],
    pub user: [u8; 20],
    pub amount: U256,
    pub shares: U256,
    pub request_time: u64,
    pub available_time: u64,
    pub claimed: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Epoch {
    pub id: u64,
    pub start_time: u64,
    pub end_time: u64,
    pub total_staked: U256,
    pub total_rewards: U256,
    pub reward_rate: U256,
}

// ==================== LIQUID STAKING ====================

pub struct LiquidStaking {
    // Token state
    total_staked: Arc<RwLock<U256>>,
    total_rewards: Arc<RwLock<U256>>,
    exchange_rate: Arc<RwLock<U256>>, // shares per staked token
    
    // Delegators
    delegators: Arc<RwLock<HashMap<[u8; 20], Delegator>>>,
    
    // Unstaking queue
    unstake_requests: Arc<RwLock<HashMap<[u8; 32], UnstakeRequest>>>,
    user_unstakes: Arc<RwLock<HashMap<[u8; 20], Vec<[u8; 32]>>>,
    
    // Epoch tracking
    epochs: Arc<RwLock<HashMap<u64, Epoch>>>,
    current_epoch: Arc<RwLock<u64>>,
    
    // Configuration
    cooldown_period: u64, // seconds before unstake is available
    min_stake: U256,
    max_stake: U256,
}

impl LiquidStaking {
    pub fn new(cooldown_period: u64) -> Self {
        Self {
            total_staked: Arc::new(RwLock::new(U256::zero())),
            total_rewards: Arc::new(RwLock::new(U256::zero())),
            exchange_rate: Arc::new(RwLock::new(U256::from(10.pow(18)))), // 1:1 initially
            
            delegators: Arc::new(RwLock::new(HashMap::new())),
            
            unstake_requests: Arc::new(RwLock::new(HashMap::new())),
            user_unstakes: Arc::new(RwLock::new(HashMap::new())),
            
            epochs: Arc::new(RwLock::new(HashMap::new())),
            current_epoch: Arc::new(RwLock::new(0)),
            
            cooldown_period,
            min_stake: U256::from(10).pow(18)), // 10 tokens min
            max_stake: U256::from(10).pow(26)), // 10M tokens max
        }
    }
    
    // ==================== STAKE ====================
    
    pub async fn stake(&self, user: [u8; 20], amount: U256) -> Result<U256, StakingError> {
        if amount < self.min_stake {
            return Err(StakingError::BelowMinimum);
        }
        
        if amount > self.max_stake {
            return Err(StakingError::AboveMaximum);
        }
        
        // Calculate shares to mint
        let exchange_rate = *self.exchange_rate.read().await;
        let shares = (amount * U256::from(10).pow(18)) / exchange_rate;
        
        // Update delegator
        let mut delegators = self.delegators.write().await;
        let delegator = delegators.entry(user).or_insert_with(|| Delegator {
            user,
            staked_amount: U256::zero(),
            accumulated_rewards: U256::zero(),
            last_checkpoint: current_timestamp(),
        });
        
        delegator.staked_amount = delegator.staked_amount + amount;
        delegator.last_checkpoint = current_timestamp();
        
        // Update total
        let mut total = self.total_staked.write().await;
        *total = *total + amount;
        
        Ok(shares)
    }
    
    // ==================== UNSTAKE ====================
    
    pub async fn unstake(&self, user: [u8; 20], shares: U256) -> Result<[u8; 32], StakingError> {
        let delegators = self.delegators.read().await;
        
        // Check delegator exists
        if let Some(delegator) = delegators.get(&user) {
            if delegator.staked_amount == U256::zero() {
                return Err(StakingError::NothingStaked);
            }
        } else {
            return Err(StakingError::NothingStaked);
        }
        
        drop(delegators);
        
        // Calculate unstake amount
        let exchange_rate = *self.exchange_rate.read().await;
        let amount = (shares * exchange_rate) / U256::from(10).pow(18);
        
        // Update delegator
        let mut delegators = self.delegators.write().await;
        if let Some(delegator) = delegators.get_mut(&user) {
            delegator.staked_amount = delegator.staked_amount - amount;
            delegator.last_checkpoint = current_timestamp();
        }
        
        // Update total
        let mut total = self.total_staked.write().await;
        *total = *total - amount;
        
        // Create unstake request
        let mut request_id = [0u8; 32];
        request_id[..20].copy_from_slice(&user);
        request_id[20..].copy_from_slice(&current_timestamp().to_le_bytes()[..12]);
        
        let now = current_timestamp();
        let request = UnstakeRequest {
            id: request_id,
            user,
            amount,
            shares,
            request_time: now,
            available_time: now + self.cooldown_period,
            claimed: false,
        };
        
        self.unstake_requests.write().await.insert(request_id, request);
        
        // Index by user
        self.user_unstakes.write().await
            .entry(user)
            .or_insert_with(Vec::new)
            .push(request_id);
        
        Ok(request_id)
    }
    
    pub async fn claim_unstake(&self, request_id: &[u8; 32]) -> Result<U256, StakingError> {
        let mut requests = self.unstake_requests.write().await;
        
        if let Some(request) = requests.get_mut(request_id) {
            if request.claimed {
                return Err(StakingError::AlreadyClaimed);
            }
            
            if current_timestamp() < request.available_time {
                return Err(StakingError::StillCooldown);
            }
            
            request.claimed = true;
            
            Ok(request.amount)
        } else {
            Err(StakingError::RequestNotFound)
        }
    }
    
    // ==================== REWARDS ====================
    
    pub async fn notify_rewards(&self, amount: U256) -> Result<(), StakingError> {
        let total = *self.total_staked.read().await;
        
        if total == U256::zero() {
            return Ok(());
        }
        
        // Update total rewards
        let mut rewards = self.total_rewards.write().await;
        *rewards = *rewards + amount;
        
        // Update exchange rate
        let mut rate = self.exchange_rate.write().await;
        
        // New rate = old rate + (rewards / total staked)
        let new_rate_increase = (amount * U256::from(10).pow(18)) / total;
        *rate = *rate + new_rate_increase;
        
        // Create new epoch
        let mut epoch_id = self.current_epoch.write().await;
        *epoch_id += 1;
        
        let epoch = Epoch {
            id: *epoch_id,
            start_time: current_timestamp(),
            end_time: current_timestamp() + 86400, // Daily
            total_staked: total,
            total_rewards: amount,
            reward_rate: new_rate_increase,
        };
        
        self.epochs.write().await.insert(*epoch_id, epoch);
        
        Ok(())
    }
    
    pub async fn claim_rewards(&self, user: [u8; 20]) -> Result<U256, StakingError> {
        let mut delegators = self.delegators.write().await;
        
        if let Some(delegator) = delegators.get_mut(&user) {
            let rewards = delegator.accumulated_rewards;
            delegator.accumulated_rewards = U256::zero();
            
            Ok(rewards)
        } else {
            Err(StakingError::NothingStaked)
        }
    }
    
    // ==================== QUERIES ====================
    
    pub async fn get_delegator(&self, user: &[u8; 20]) -> Option<Delegator> {
        let delegators = self.delegators.read().await;
        delegators.get(user).cloned()
    }
    
    pub async fn get_exchange_rate(&self) -> U256 {
        *self.exchange_rate.read().await
    }
    
    pub async fn get_total_staked(&self) -> U256 {
        *self.total_staked.read().await
    }
    
    pub async fn get_pending_unstakes(&self, user: &[u8; 20]) -> Vec<UnstakeRequest> {
        let user_unstakes = self.user_unstakes.read().await;
        let requests = self.unstake_requests.read().await;
        
        if let Some(request_ids) = user_unstakes.get(user) {
            request_ids.iter()
                .filter_map(|id| {
                    if let Some(req) = requests.get(id) {
                        if !req.claimed {
                            return Some(req.clone());
                        }
                    }
                    None
                })
                .collect()
        } else {
            Vec::new()
        }
    }
}

// ==================== ERRORS ====================

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum StakingError {
    BelowMinimum,
    AboveMaximum,
    NothingStaked,
    RequestNotFound,
    AlreadyClaimed,
    StillCooldown,
    InvalidAmount,
}

impl std::fmt::Display for StakingError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            StakingError::BelowMinimum => write!(f, "Below minimum stake"),
            StakingError::AboveMaximum => write!(f, "Above maximum stake"),
            StakingError::NothingStaked => write!(f, "Nothing staked"),
            StakingError::RequestNotFound => write!(f, "Request not found"),
            StakingError::AlreadyClaimed => write!(f, "Already claimed"),
            StakingError::StillCooldown => write!(f, "Still in cooldown"),
            StakingError::InvalidAmount => write!(f, "Invalid amount"),
        }
    }
}

// ==================== HELPER ====================

fn current_timestamp() -> u64 {
    std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .unwrap()
        .as_secs()
}

trait U256Ext {
    fn zero() -> Self;
    fn from(n: u64) -> Self;
    fn pow(n: u32) -> Self;
}

impl U256Ext for U256 {
    fn zero() -> Self { U256::from(0) }
    fn from(n: u64) -> Self { U256::from(n) }
    fn pow(n: u32) -> Self {
        let mut result = U256::from(1);
        let base = U256::from(n);
        for _ in 0..n {
            result = result * base;
        }
        result
    }
}

// ==================== PUBLIC API ====================

pub mod api {
    use super::*;
    
    pub type LiquidStakingHandle = Arc<LiquidStaking>;
    
    pub fn create_staking(cooldown_seconds: u64) -> LiquidStakingHandle {
        Arc::new(LiquidStaking::new(cooldown_seconds))
    }
}

// ==================== TESTS ====================

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_staking_creation() {
        let staking = LiquidStaking::new(86400); // 24 hour cooldown
        
        // This would need async context in real tests
    }
}