//! TigerSwap veToken Governance System
//! 
//! Implements Curve-style veToken governance (veCRV, veVELO, veAERO):
//! - veToken: Lock tokens for voting power
//! - Gauge voting: Distribute emissions to liquidity pools
//! - Bribe system: Incentivize gauge votes
//! - Fee distribution: Share protocol fees with voters
//! - Emissions scheduling: Configure token emissions
//!
//! Implementation: Pure Rust with no external dependencies

use serde::{Deserialize, Serialize};
use std::sync::Arc;
use parking_lot::RwLock;
use rust_decimal::Decimal;
use thiserror::Error;
use uuid::Uuid;
use chrono::{Utc, Duration};
use std::collections::{HashMap, HashSet};

/// Chain ID constants
pub const CHAIN_ETH: u64 = 1;
pub const CHAIN_BSC: u64 = 56;
pub const CHAIN_POLYGON: u64 = 137;
pub const CHAIN_ARBITRUM: u64 = 42161;
pub const CHAIN_OPTIMISM: u64 = 10;
pub const CHAIN_BASE: u64 = 8453;
pub const CHAIN_AVALANCHE: u64 = 43114;

#[derive(Debug, Error)]
pub enum GovernanceError {
    #[error("Lock not found: {0}")]
    LockNotFound(String),
    #[error("Gauge not found: {0}")]
    GaugeNotFound(String),
    #[error("Insufficient balance: {0}")]
    InsufficientBalance(String),
    #[error("Insufficient voting power: {0}")]
    InsufficientVotingPower(String),
    #[error("Lock period too short: minimum {0} days")]
    LockPeriodTooShort(i64),
    #[error("Lock period too long: maximum {0} years")]
    LockPeriodTooLong(i64),
    #[error("Invalid voting weight: {0}")]
    InvalidVotingWeight(String),
    #[error("Already voted: {0}")]
    AlreadyVoted(String),
    #[error("Vote not found: {0}")]
    VoteNotFound(String),
    #[error("Bribe not found: {0}")]
    BribeNotFound(String),
    #[error("Claim not available yet")]
    ClaimNotAvailable,
    #[error("Chain not supported: {0}")]
    ChainNotSupported(u64),
    #[error("Epoch not ended")]
    EpochNotEnded,
    #[error("Invalid parameters: {0}")]
    InvalidParameters(String),
}

/// Minimum lock period (days)
pub const MIN_LOCK_PERIOD: i64 = 7;
/// Maximum lock period (years)
pub const MAX_LOCK_YEARS: i64 = 4;

/// Voting power decay period (1 week)
pub const VOTE_DECAY_PERIOD: i64 = 7 * 24 * 60 * 60;

/// Gauge status
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum GaugeStatus {
    Active,
    Killed,
    Pending,
}

impl Default for GaugeStatus {
    fn default() -> Self { GaugeStatus::Pending }
}

/// veToken lock
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct VELock {
    pub lock_id: String,
    pub user: String,
    pub chain_id: u64,
    pub token: String,
    pub amount: u128,
    pub lock_start: i64,
    pub lock_end: i64,
    pub voting_power: Decimal,
    pub boosted_power: Decimal,  // With boost factor
    pub slope: Decimal,         // For linear decay
    pub bias: Decimal,           // Initial voting power
    pub is NFT: bool,
    pub NFT_id: Option<String>,
    pub created_at: i64,
    pub updated_at: i64,
}

impl VELock {
    /// Create a new lock
    pub fn new(
        user: String,
        chain_id: u64,
        token: String,
        amount: u128,
        lock_duration_days: i64,
    ) -> Result<Self, GovernanceError> {
        if lock_duration_days < MIN_LOCK_PERIOD {
            return Err(GovernanceError::LockPeriodTooShort(MIN_LOCK_PERIOD));
        }
        if lock_duration_days > MAX_LOCK_YEARS * 365 {
            return Err(GovernanceError::LockPeriodTooLong(MAX_LOCK_YEARS));
        }
        
        let now = Utc::now().timestamp();
        let lock_end = now + (lock_duration_days * 24 * 60 * 60);
        
        // Calculate voting power: amount * time_factor
        // Time factor increases linearly with lock duration (1-4 years)
        let time_factor = Decimal::from(lock_duration_days) / Decimal::from(365);
        let voting_power = Decimal::from(amount) * time_factor;
        
        // Calculate bias (initial voting power)
        let bias = voting_power;
        
        // Calculate slope (decay per second)
        let duration_seconds = (lock_duration_days * 24 * 60 * 60) as f64;
        let slope = voting_power / Decimal::from(duration_seconds);
        
        // Boost factor: longer locks get up to 2.5x boost
        let boost_factor = if lock_duration_days >= 365 * 4 {
            Decimal::from(250) / Decimal::from(100)  // 2.5x
        } else if lock_duration_days >= 365 * 2 {
            Decimal::from(200) / Decimal::from(100)  // 2x
        } else if lock_duration_days >= 365 {
            Decimal::from(150) / Decimal::from(100)  // 1.5x
        } else {
            Decimal::ONE
        };
        
        let boosted_power = voting_power * boost_factor;
        
        Ok(Self {
            lock_id: Uuid::new_v4().to_string(),
            user,
            chain_id,
            token,
            amount,
            lock_start: now,
            lock_end,
            voting_power,
            boosted_power,
            slope,
            bias,
            is NFT: false,
            NFT_id: None,
            created_at: now,
            updated_at: now,
        })
    }

    /// Get current voting power (with decay)
    pub fn get_current_voting_power(&self) -> Decimal {
        let now = Utc::now().timestamp();
        
        if now >= self.lock_end {
            return Decimal::ZERO;
        }
        
        let time_remaining = self.lock_end - now;
        let remaining = Decimal::from(time_remaining) * self.slope;
        
        if remaining > self.bias {
            self.bias
        } else {
            remaining
        }
    }

    /// Check if lock is expired
    pub fn is_expired(&self) -> bool {
        Utc::now().timestamp() >= self.lock_end
    }

    /// Get remaining time in seconds
    pub fn remaining_time(&self) -> i64 {
        let remaining = self.lock_end - Utc::now().timestamp();
        if remaining > 0 { remaining } else { 0 }
    }

    /// Extend lock duration
    pub fn extend(&mut self, additional_days: i64) -> Result<(), GovernanceError> {
        let new_end = self.lock_end + (additional_days * 24 * 60 * 60);
        let max_end = self.lock_start + (MAX_LOCK_YEARS * 365 * 24 * 60 * 60);
        
        if new_end > max_end {
            return Err(GovernanceError::LockPeriodTooLong(MAX_LOCK_YEARS));
        }
        
        self.lock_end = new_end;
        
        // Recalculate voting power
        let time_factor = Decimal::from(new_end - self.lock_start) / Decimal::from(365 * 24 * 60 * 60);
        self.voting_power = Decimal::from(self.amount) * time_factor;
        self.updated_at = Utc::now().timestamp();
        
        Ok(())
    }

    /// Withdraw after lock expires
    pub fn withdraw(&mut self) -> Result<u128, GovernanceError> {
        if !self.is_expired() {
            return Err(GovernanceError::LockPeriodTooShort(MIN_LOCK_PERIOD));
        }
        
        let amount = self.amount;
        self.amount = 0;
        self.voting_power = Decimal::ZERO;
        self.updated_at = Utc::now().timestamp();
        
        Ok(amount)
    }
}

/// Liquidity gauge
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Gauge {
    pub gauge_id: String,
    pub name: String,
    pub chain_id: u64,
    pub pool_address: String,
    pub token0: String,
    pub token1: String,
    
    // Voting
    pub weight: u64,
    pub votes_count: u32,
    pub total_votes: u128,
    
    // Emissions
    pub emission_rate: u128,
    pub emission_token: String,
    pub total_supply: u128,
    
    // Status
    pub status: GaugeStatus,
    pub created_at: i64,
    pub updated_at: i64,
}

impl Gauge {
    /// Create a new gauge
    pub fn new(
        chain_id: u64,
        pool_address: String,
        token0: String,
        token1: String,
    ) -> Self {
        let now = Utc::now().timestamp();
        
        Self {
            gauge_id: Uuid::new_v4().to_string(),
            name: format!("{}/{} Pool", token0, token1),
            chain_id,
            pool_address,
            token0,
            token1,
            weight: 0,
            vote_count: 0,
            total_votes: 0,
            emission_rate: 0,
            emission_token: String::new(),
            total_supply: 0,
            status: GaugeStatus::Pending,
            created_at: now,
            updated_at: now,
        }
    }

    /// Update gauge weight
    pub fn update_weight(&mut self, weight: u64) {
        self.weight = weight;
        self.updated_at = Utc::now().timestamp();
    }

    /// Add vote
    pub fn add_vote(&mut self, voting_power: u128) {
        self.total_votes += voting_power;
        self.vote_count += 1;
        self.updated_at = Utc::now().timestamp();
    }

    /// Remove vote
    pub fn remove_vote(&mut self, voting_power: u128) {
        self.total_votes = self.total_votes.saturating_sub(voting_power);
        self.vote_count = self.vote_count.saturating_sub(1);
        self.updated_at = Utc::now().timestamp();
    }

    /// Kill gauge
    pub fn kill(&mut self) {
        self.status = GaugeStatus::Killed;
        self.updated_at = Utc::now().timestamp();
    }

    /// Activate gauge
    pub fn activate(&mut self) {
        self.status = GaugeStatus::Active;
        self.updated_at = Utc::now().timestamp();
    }
}

/// Gauge vote
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct GaugeVote {
    pub vote_id: String,
    pub user: String,
    pub gauge_id: String,
    pub voting_power: u128,
    pub timestamp: i64,
}

/// Bribe
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Bribe {
    pub bribe_id: String,
    pub gauge_id: String,
    pub reward_token: String,
    pub total_rewards: u128,
    pub claimed_rewards: u128,
    pub start_time: i64,
    pub end_time: i64,
    pub epoch: u32,
    pub created_at: i64,
}

impl Bribe {
    /// Create a new bribe
    pub fn new(
        gauge_id: String,
        reward_token: String,
        total_rewards: u128,
        duration_days: i64,
    ) -> Self {
        let now = Utc::now().timestamp();
        
        Self {
            bribe_id: Uuid::new_v4().to_string(),
            gauge_id,
            reward_token,
            total_rewards,
            claimed_rewards: 0,
            start_time: now,
            end_time: now + (duration_days * 24 * 60 * 60),
            epoch: (now / (7 * 24 * 60 * 60)) as u32,
            created_at: now,
        }
    }

    /// Check if bribe is active
    pub fn is_active(&self) -> bool {
        let now = Utc::now().timestamp();
        now >= self.start_time && now <= self.end_time
    }

    /// Claim rewards
    pub fn claim(&mut self, amount: u128) -> Result<(), GovernanceError> {
        if self.claimed_rewards + amount > self.total_rewards {
            return Err(GovernanceError::InvalidParameters("Claim exceeds available".to_string()));
        }
        
        self.claimed_rewards += amount;
        Ok(())
    }

    /// Get remaining rewards
    pub fn remaining(&self) -> u128 {
        self.total_rewards - self.claimed_rewards
    }
}

/// Fee distribution
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct FeeDistribution {
    pub distribution_id: String,
    pub epoch: u32,
    pub token: String,
    pub total_fees: u128,
    pub distributed: u128,
    pub start_time: i64,
    pub end_time: i64,
}

impl FeeDistribution {
    /// Create a new distribution
    pub fn new(epoch: u32, token: String, total_fees: u128) -> Self {
        let now = Utc::now().timestamp();
        
        Self {
            distribution_id: Uuid::new_v4().to_string(),
            epoch,
            token,
            total_fees,
            distributed: 0,
            start_time: now,
            end_time: now + (7 * 24 * 60 * 60),  // 1 week
        }
    }

    /// Calculate user share
    pub fn calculate_share(&self, voting_power: u128, total_voting_power: u128) -> u128 {
        if total_voting_power == 0 || self.total_fees == 0 {
            return 0;
        }
        
        (self.total_fees * voting_power) / total_voting_power
    }

    /// Distribute fees
    pub fn distribute(&mut self, amount: u128) -> Result<(), GovernanceError> {
        if self.distributed + amount > self.total_fees {
            return Err(GovernanceError::InvalidParameters("Exceeds total fees".to_string()));
        }
        
        self.distributed += amount;
        Ok(())
    }
}

/// User claim
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct UserClaim {
    pub claim_id: String,
    pub user: String,
    pub distribution_id: String,
    pub amount: u128,
    pub claimed_at: Option<i64>,
    pub created_at: i64,
}

/// Governance engine
pub struct GovernanceEngine {
    locks: Arc<RwLock<HashMap<String, VELock>>>,
    gauges: Arc<RwLock<HashMap<String, Gauge>>>,
    votes: Arc<RwLock<HashMap<String, Vec<GaugeVote>>>>,
    bribes: Arc<RwLock<HashMap<String, Bribe>>>>,
    distributions: Arc<RwLock<HashMap<String, FeeDistribution>>>>,
    claims: Arc<RwLock<HashMap<String, Vec<UserClaim>>>>,
    supported_chains: Arc<RwLock<std::collections::HashSet<u64>>>,
    total_voting_power: Arc<RwLock<Decimal>>,
    protocol_token: Arc<RwLock<String>>>,
    emission_rate: Arc<RwLock<u128>>,
}

impl GovernanceEngine {
    /// Create a new governance engine
    pub fn new() -> Self {
        let chains: std::collections::HashSet<u64> = [
            CHAIN_ETH, CHAIN_BSC, CHAIN_POLYGON, CHAIN_ARBITRUM,
            CHAIN_OPTIMISM, CHAIN_BASE, CHAIN_AVALANCHE,
        ].into_iter().collect();
        
        Self {
            locks: Arc::new(RwLock::new(HashMap::new())),
            gauges: Arc::new(RwLock::new(HashMap::new())),
            votes: Arc::new(RwLock::new(HashMap::new())),
            bribes: Arc::new(RwLock::new(HashMap::new())),
            distributions: Arc::new(RwLock::new(HashMap::new())),
            claims: Arc::new(RwLock::new(HashMap::new())),
            supported_chains: Arc::new(RwLock::new(chains)),
            total_voting_power: Arc::new(RwLock::new(Decimal::ZERO)),
            protocol_token: Arc::new(RwLock::new(String::new())),
            emission_rate: Arc::new(RwLock::new(0)),
        }
    }

    /// Initialize governance token
    pub fn initialize(&self, token: String, emission_rate: u128) {
        *self.protocol_token.write() = token;
        *self.emission_rate.write() = emission_rate;
    }

    /// Check if chain is supported
    pub fn is_chain_supported(&self, chain_id: u64) -> bool {
        self.supported_chains.read().contains(&chain_id)
    }

    /// Create a lock
    pub fn create_lock(
        &self,
        user: String,
        chain_id: u64,
        token: String,
        amount: u128,
        lock_duration_days: i64,
    ) -> Result<String, GovernanceError> {
        if !self.is_chain_supported(chain_id) {
            return Err(GovernanceError::ChainNotSupported(chain_id));
        }
        
        let lock = VELock::new(user, chain_id, token, amount, lock_duration_days)?;
        let lock_id = lock.lock_id.clone();
        
        self.locks.write().insert(lock_id.clone(), lock);
        
        // Update total voting power
        *self.total_voting_power.write() += Decimal::from(amount);
        
        Ok(lock_id)
    }

    /// Get lock
    pub fn get_lock(&self, lock_id: &str) -> Option<VELock> {
        self.locks.read().get(lock_id).cloned()
    }

    /// Get user locks
    pub fn get_user_locks(&self, user: &str) -> Vec<VELock> {
        self.locks.read()
            .values()
            .filter(|l| l.user == user && l.amount > 0)
            .cloned()
            .collect()
    }

    /// Get user voting power
    pub fn get_user_voting_power(&self, user: &str) -> Decimal {
        self.locks.read()
            .values()
            .filter(|l| l.user == user)
            .map(|l| l.get_current_voting_power())
            .sum()
    }

    /// Extend lock
    pub fn extend_lock(&self, lock_id: &str, additional_days: i64) -> Result<(), GovernanceError> {
        let mut locks = self.locks.write();
        let lock = locks.get_mut(lock_id)
            .ok_or_else(|| GovernanceError::LockNotFound(lock_id.to_string()))?;
        
        lock.extend(additional_days)?;
        
        Ok(())
    }

    /// Withdraw from expired lock
    pub fn withdraw(&self, lock_id: &str) -> Result<u128, GovernanceError> {
        let mut locks = self.locks.write();
        let lock = locks.get_mut(lock_id)
            .ok_or_else(|| GovernanceError::LockNotFound(lock_id.to_string()))?;
        
        let amount = lock.withdraw()?;
        
        // Update total voting power
        *self.total_voting_power.write() -= Decimal::from(amount);
        
        Ok(amount)
    }

    /// Create a gauge
    pub fn create_gauge(
        &self,
        chain_id: u64,
        pool_address: String,
        token0: String,
        token1: String,
    ) -> Result<String, GovernanceError> {
        if !self.is_chain_supported(chain_id) {
            return Err(GovernanceError::ChainNotSupported(chain_id));
        }
        
        let gauge = Gauge::new(chain_id, pool_address, token0, token1);
        let gauge_id = gauge.gauge_id.clone();
        
        self.gauges.write().insert(gauge_id.clone(), gauge);
        
        Ok(gauge_id)
    }

    /// Get gauge
    pub fn get_gauge(&self, gauge_id: &str) -> Option<Gauge> {
        self.gauges.read().get(gauge_id).cloned()
    }

    /// Get all gauges
    pub fn get_gauges(&self) -> Vec<Gauge> {
        self.gauges.read().values().cloned().collect()
    }

    /// Vote for gauges
    pub fn vote(
        &self,
        user: &str,
        gauge_weights: Vec<(String, u128)>,
    ) -> Result<(), GovernanceError> {
        let voting_power = self.get_user_voting_power(user);
        
        if voting_power == Decimal::ZERO {
            return Err(GovernanceError::InsufficientVotingPower(user.to_string()));
        }
        
        let total_weight: u128 = gauge_weights.iter().map(|(_, w)| w).sum();
        
        // Check voting power
        let vp = voting_power.as_u128();
        if total_weight > vp {
            return Err(GovernanceError::InsufficientVotingPower(format!(
                "Required: {}, Available: {}",
                total_weight, vp
            )));
        }
        
        let mut user_votes = self.votes.write();
        
        // Remove old votes
        if let Some(old_votes) = user_votes.get(user) {
            for vote in old_votes {
                if let Some(gauge) = self.gauges.write().get_mut(&vote.gauge_id) {
                    gauge.remove_vote(vote.voting_power);
                }
            }
        }
        
        // Add new votes
        let mut new_votes = vec![];
        for (gauge_id, weight) in gauge_weights {
            let gauge_vp = (voting_power.as_u128() * weight) / 10000;
            
            let vote = GaugeVote {
                vote_id: Uuid::new_v4().to_string(),
                user: user.to_string(),
                gauge_id: gauge_id.clone(),
                voting_power: gauge_vp,
                timestamp: Utc::now().timestamp(),
            };
            
            if let Some(gauge) = self.gauges.write().get_mut(&gauge_id) {
                gauge.add_vote(gauge_vp);
            }
            
            new_votes.push(vote);
        }
        
        user_votes.insert(user.to_string(), new_votes);
        
        Ok(())
    }

    /// Get user votes
    pub fn get_user_votes(&self, user: &str) -> Vec<GaugeVote> {
        self.votes.read()
            .get(user)
            .cloned()
            .unwrap_or_default()
    }

    /// Get gauge weight
    pub fn get_gauge_weight(&self, gauge_id: &str) -> u128 {
        self.gauges.read()
            .get(gauge_id)
            .map(|g| g.total_votes)
            .unwrap_or(0)
    }

    /// Get total voting power
    pub fn get_total_voting_power(&self) -> Decimal {
        *self.total_voting_power.read()
    }

    /// Create a bribe
    pub fn create_bribe(
        &self,
        gauge_id: &str,
        reward_token: String,
        total_rewards: u128,
        duration_days: i64,
    ) -> Result<String, GovernanceError> {
        // Verify gauge exists
        if !self.gauges.read().contains_key(gauge_id) {
            return Err(GovernanceError::GaugeNotFound(gauge_id.to_string()));
        }
        
        let bribe = Bribe::new(gauge_id.to_string(), reward_token, total_rewards, duration_days);
        let bribe_id = bribe.bribe_id.clone();
        
        self.bribes.write().insert(bribe_id.clone(), bribe);
        
        Ok(bribe_id)
    }

    /// Get bribe
    pub fn get_bribe(&self, bribe_id: &str) -> Option<Bribe> {
        self.bribes.read().get(bribe_id).cloned()
    }

    /// Claim bribe rewards
    pub fn claim_bribe(&self, bribe_id: &str, user: &str) -> Result<u128, GovernanceError> {
        let mut bribes = self.bribes.write();
        let bribe = bribes.get_mut(bribe_id)
            .ok_or_else(|| GovernanceError::BribeNotFound(bribe_id.to_string()))?;
        
        if !bribe.is_active() {
            return Err(GovernanceError::ClaimNotAvailable);
        }
        
        // Calculate user share based on voting power
        let user_vp = self.get_user_voting_power(user);
        let total_vp = self.get_total_voting_power();
        
        if total_vp == Decimal::ZERO {
            return Ok(0);
        }
        
        let share = (bribe.remaining() * user_vp.as_u128()) / total_vp.as_u128();
        
        if share == 0 {
            return Ok(0);
        }
        
        bribe.claim(share)?;
        
        Ok(share)
    }

    /// Create fee distribution
    pub fn create_distribution(
        &self,
        token: String,
        total_fees: u128,
    ) -> Result<String, GovernanceError> {
        let epoch = (Utc::now().timestamp() / (7 * 24 * 60 * 60)) as u32;
        
        let distribution = FeeDistribution::new(epoch, token, total_fees);
        let distribution_id = distribution.distribution_id.clone();
        
        self.distributions.write().insert(distribution_id.clone(), distribution);
        
        Ok(dribution_id)
    }

    /// Claim fees
    pub fn claim_fees(&self, distribution_id: &str, user: &str) -> Result<u128, GovernanceError> {
        let mut distributions = self.distributions.write();
        let distribution = distributions.get_mut(distribution_id)
            .ok_or_else(|| GovernanceError::LockNotFound(distribution_id.to_string()))?;
        
        let user_vp = self.get_user_voting_power(user);
        let total_vp = self.get_total_voting_power();
        
        if total_vp == Decimal::ZERO {
            return Ok(0);
        }
        
        let available = distribution.total_fees - distribution.distributed;
        let share = (available * user_vp.as_u128()) / total_vp.as_u128();
        
        if share == 0 {
            return Ok(0);
        }
        
        distribution.distribute(share)?;
        
        // Record claim
        let claim = UserClaim {
            claim_id: Uuid::new_v4().to_string(),
            user: user.to_string(),
            distribution_id: distribution_id.to_string(),
            amount: share,
            claimed_at: None,
            created_at: Utc::now().timestamp(),
        };
        
        self.claims.write()
            .entry(user.to_string())
            .or_insert_with(Vec::new)
            .push(claim);
        
        Ok(share)
    }

    /// Get pending emissions
    pub fn get_pending_emissions(&self) -> u128 {
        *self.emission_rate.read()
    }

    /// Calculate emissions for gauge
    pub fn calculate_gauge_emissions(&self, gauge_id: &str) -> u128 {
        let gauges = self.gauges.read();
        let gauge = match gauges.get(gauge_id) {
            Some(g) => g,
            None => return 0,
        };
        
        let total_weight: u128 = gauges.values().map(|g| g.total_votes).sum();
        
        if total_weight == 0 {
            return 0;
        }
        
        let emission_rate = *self.emission_rate.read();
        (emission_rate * gauge.total_votes) / total_weight
    }

    /// Kill gauge
    pub fn kill_gauge(&self, gauge_id: &str) -> Result<(), GovernanceError> {
        let mut gauges = self.gauges.write();
        let gauge = gauges.get_mut(gauge_id)
            .ok_or_else(|| GovernanceError::GaugeNotFound(gauge_id.to_string()))?;
        
        gauge.kill();
        
        Ok(())
    }

    /// Get governance statistics
    pub fn get_stats(&self) -> GovernanceStats {
        let locks = self.locks.read();
        let gauges = self.gauges.read();
        
        let total_locked: u128 = locks.values().map(|l| l.amount).sum();
        let total_voting_power = *self.total_voting_power.read();
        
        let active_gauges = gauges.values()
            .filter(|g| matches!(g.status, GaugeStatus::Active))
            .count();
        
        GovernanceStats {
            total_locks: locks.len(),
            total_locked,
            total_voting_power: total_voting_power.as_u128(),
            total_gauges: gauges.len(),
            active_gauges,
        }
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

impl Default for GovernanceEngine {
    fn default() -> Self { Self::new() }
}

/// Governance statistics
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct GovernanceStats {
    pub total_locks: usize,
    pub total_locked: u128,
    pub total_voting_power: u128,
    pub total_gauges: usize,
    pub active_gauges: usize,
}

#[cfg(test)]
mod tests {
    use super::*;
    use rust_decimal_macros::dec;

    #[test]
    fn test_lock_creation() {
        let lock = VELock::new(
            "user1".to_string(),
            CHAIN_ETH,
            "TIGER".to_string(),
            1000,
            365,
        ).unwrap();
        
        assert!(lock.voting_power > dec!(0));
    }

    #[test]
    fn test_voting_power_decay() {
        let mut lock = VELock::new(
            "user1".to_string(),
            CHAIN_ETH,
            "TIGER".to_string(),
            1000,
            365,
        ).unwrap();
        
        let vp = lock.get_current_voting_power();
        assert!(vp > Decimal::ZERO);
    }

    #[test]
    fn test_gauge_creation() {
        let gauge = Gauge::new(
            CHAIN_ETH,
            "0xPool".to_string(),
            "USDC".to_string(),
            "WETH".to_string(),
        );
        
        assert_eq!(gauge.status, GaugeStatus::Pending);
    }

    #[test]
    fn test_bribe() {
        let bribe = Bribe::new(
            "gauge1".to_string(),
            "USDC".to_string(),
            10000,
            7,
        );
        
        assert!(bribe.is_active());
    }
}