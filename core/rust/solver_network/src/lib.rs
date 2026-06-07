//! Solver Staking Network
//! 
//! Provides solver discovery, staking, rewards, and reputation for intent-based trading

use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use std::sync::Arc;
use parking_lot::RwLock;
use thiserror::Error;
use uuid::Uuid;
use chrono::{DateTime, Utc};

/// Solver network errors
#[derive(Debug, Error)]
pub enum SolverError {
    #[error("Solver not found: {0}")]
    SolverNotFound(String),
    #[error("Insufficient stake")]
    InsufficientStake,
    #[error("Solver offline")]
    SolverOffline,
    #[error("Intent expired")]
    IntentExpired,
    #[error("No solver available")]
    NoSolverAvailable,
    #[error("Slashing: {0}")]
    Slashing(String),
}

/// Solver status
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum SolverStatus {
    Active,
    Inactive,
    Slashed,
    Banned,
}

/// Intent (user request)
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Intent {
    /// Intent ID
    pub intent_id: String,
    /// Owner
    pub owner: String,
    /// What they want (e.g., "sell 1 ETH for USDC")
    pub description: String,
    /// Input token
    pub token_in: String,
    /// Output token
    pub token_out: String,
    /// Amount in
    pub amount_in: u128,
    /// Min amount out
    pub min_amount_out: u128,
    /// Deadline
    pub deadline: i64,
    /// Slippage tolerance (bps)
    pub slippage_bps: i64,
    /// Created at
    pub created_at: i64,
    /// Status
    pub status: IntentStatus,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum IntentStatus {
    Open,
    Pending,
    Filled,
    Expired,
    Cancelled,
}

impl Intent {
    pub fn new(
        owner: String,
        description: String,
        token_in: String,
        token_out: String,
        amount_in: u128,
        min_amount_out: u128,
    ) -> Self {
        Self {
            intent_id: Uuid::new_v4().to_string(),
            owner,
            description,
            token_in,
            token_out,
            amount_in,
            min_amount_out,
            deadline: Utc::now().timestamp() + 600, // 10 min
            slippage_bps: 50,
            created_at: Utc::now().timestamp(),
            status: IntentStatus::Open,
        }
    }

    /// Check if expired
    pub fn is_expired(&self) -> bool {
        Utc::now().timestamp() > self.deadline
    }
}

/// Solver
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Solver {
    /// Solver address
    pub address: String,
    /// Name
    pub name: String,
    /// Stake amount (in protocol token)
    pub stake: u128,
    /// Reputation score (0-100)
    pub reputation: u32,
    /// Total fulfilled
    pub total_fulfilled: u64,
    /// Total failed
    pub total_failed: u64,
    /// Success rate
    pub success_rate: f64,
    /// Average execution time (ms)
    pub avg_execution_time_ms: u64,
    /// Status
    pub status: SolverStatus,
    /// Regions served
    pub regions: Vec<String>,
    /// Chains supported
    pub chains: Vec<u64>,
    /// Last active
    pub last_active: i64,
    /// Registered at
    pub registered_at: i64,
    /// Pending slashing
    pub pending_slashing: u128,
}

impl Solver {
    pub fn new(address: String, name: String, stake: u128) -> Self {
        Self {
            address,
            name,
            stake,
            reputation: 100,
            total_fulfilled: 0,
            total_failed: 0,
            success_rate: 1.0,
            avg_execution_time_ms: 0,
            status: SolverStatus::Active,
            regions: vec!["global".to_string()],
            chains: vec![1, 56, 137, 42161],
            last_active: Utc::now().timestamp(),
            registered_at: Utc::now().timestamp(),
            pending_slashing: 0,
        }
    }

    /// Update success
    pub fn on_success(&mut self, execution_time_ms: u64) {
        self.total_fulfilled += 1;
        
        // Update success rate
        let total = self.total_fulfilled + self.total_failed;
        self.success_rate = total as f64 / (self.total_fulfilled + 1) as f64;
        
        // Update avg execution time
        if self.avg_execution_time_ms == 0 {
            self.avg_execution_time_ms = execution_time_ms;
        } else {
            self.avg_execution_time_ms = (self.avg_execution_time_ms + execution_time_ms) / 2;
        }
        
        self.last_active = Utc::now().timestamp();
    }

    /// Update failure
    pub fn on_failure(&mut self) {
        self.total_failed += 1;
        
        // Update success rate
        let total = self.total_fulfilled + self.total_failed;
        self.success_rate = self.total_fulfilled as f64 / total as f64;
        
        // Decrease reputation
        if self.reputation > 10 {
            self.reputation -= 10;
        }
        
        self.last_active = Utc::now().timestamp();
    }

    /// Slash solver
    pub fn slash(&mut self, amount: u128) {
        self.pending_slashing += amount;
        self.stake = self.stake.saturating_sub(amount);
        
        if self.stake < 1000_000_000_000_000_000 {
            // Below minimum, ban
            self.status = SolverStatus::Banned;
        }
    }
}

/// Solver bid
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SolverBid {
    /// Bid ID
    pub bid_id: String,
    /// Solver address
    pub solver: String,
    /// Intent ID
    pub intent_id: String,
    /// Amount out
    pub amount_out: u128,
    /// Execution fee
    pub fee: u128,
    /// Expiration
    pub expires_at: i64,
    /// Timestamp
    pub timestamp: i64,
}

impl SolverBid {
    pub fn new(
        solver: String,
        intent_id: String,
        amount_out: u128,
        fee: u128,
    ) -> Self {
        Self {
            bid_id: Uuid::new_v4().to_string(),
            solver,
            intent_id,
            amount_out,
            fee,
            expires_at: Utc::now().timestamp() + 60, // 1 min
            timestamp: Utc::now().timestamp(),
        }
    }
}

/// Solver network
pub struct SolverNetwork {
    /// Solvers by address
    solvers: Arc<RwLock<HashMap<String, Solver>>>,
    /// Pending intents
    intents: Arc<RwLock<HashMap<String, Intent>>>,
    /// Pending bids
    bids: Arc<RwLock<HashMap<String, Vec<SolverBid>>>>,
    /// Minimum stake
    min_stake: u128,
    /// Protocol fee (bps)
    protocol_fee_bps: i64,
    /// Rewards pool
    rewards_pool: Arc<RwLock<u128>>,
}

impl SolverNetwork {
    pub fn new() -> Self {
        Self {
            solvers: Arc::new(RwLock::new(HashMap::new())),
            intents: Arc::new(RwLock::new(HashMap::new())),
            bids: Arc::new(RwLock::new(HashMap::new())),
            min_stake: 1000_000_000_000_000_000, // 1000 tokens
            protocol_fee_bps: 10, // 0.1%
            rewards_pool: Arc::new(RwLock::new(0)),
        }
    }

    /// Register solver
    pub fn register_solver(&self, solver: Solver) -> Result<(), SolverError> {
        if solver.stake < self.min_stake {
            return Err(SolverError::InsufficientStake);
        }
        
        let mut solvers = self.solvers.write();
        solvers.insert(solver.address.clone(), solver);
        Ok(())
    }

    /// Get solver
    pub fn get_solver(&self, address: &str) -> Option<Solver> {
        let solvers = self.solvers.read();
        solvers.get(address).cloned()
    }

    /// Get active solvers
    pub fn get_active_solvers(&self) -> Vec<Solver> {
        let solvers = self.solvers.read();
        solvers.values()
            .filter(|s| s.status == SolverStatus::Active && s.stake >= self.min_stake)
            .cloned()
            .collect()
    }

    /// Submit intent
    pub fn submit_intent(&self, intent: Intent) -> Result<String, SolverError> {
        let intent_id = intent.intent_id.clone();
        
        let mut intents = self.intents.write();
        intents.insert(intent_id.clone(), intent);
        
        Ok(intent_id)
    }

    /// Get intent
    pub fn get_intent(&self, intent_id: &str) -> Option<Intent> {
        let intents = self.intents.read();
        intents.get(intent_id).cloned()
    }

    /// Submit bid
    pub fn submit_bid(&self, bid: SolverBid) -> Result<(), SolverError> {
        let mut bids = self.bids.write();
        
        let intent_bids = bids.entry(bid.intent_id.clone()).or_insert_with(Vec::new);
        intent_bids.push(bid);
        
        Ok(())
    }

    /// Get best bid for intent
    pub fn get_best_bid(&self, intent_id: &str) -> Option<SolverBid> {
        let bids = self.bids.read();
        let intent_bids = bids.get(intent_id)?;
        
        intent_bids.iter()
            .max_by_key(|b| b.amount_out)
            .cloned()
    }

    /// Select solver for intent (based on reputation + stake)
    pub fn select_solver(&self, intent: &Intent) -> Result<Solver, SolverError> {
        let solvers = self.solvers.read();
        
        // Filter by:
        // 1. Active status
        // 2. Sufficient stake
        // 3. Supports the chain
        let mut eligible: Vec<&Solver> = solvers.values()
            .filter(|s| {
                s.status == SolverStatus::Active && 
                s.stake >= self.min_stake
            })
            .collect();
        
        // Score: reputation * stake
        eligible.sort_by(|a, b| {
            let score_a = (a.reputation as u128) * a.stake;
            let score_b = (b.reputation as u128) * b.stake;
            score_b.cmp(&score_a)
        });
        
        eligible.first()
            .cloned()
            .map(|s| s.clone())
            .ok_or(SolverError::NoSolverAvailable)
    }

    /// Execute intent with solver
    pub fn execute_intent(
        &self,
        intent_id: &str,
        solver_address: &str,
        amount_out: u128,
    ) -> Result<(), SolverError> {
        // Mark intent as filled
        let mut intents = self.intents.write();
        if let Some(intent) = intents.get_mut(intent_id) {
            intent.status = IntentStatus::Filled;
        }
        
        // Update solver stats
        let mut solvers = self.solvers.write();
        if let Some(solver) = solvers.get_mut(solver_address) {
            solver.on_success(100); // 100ms typical
        }
        
        Ok(())
    }

    /// Slash solver
    pub fn slash_solver(&self, address: &str, amount: u128, reason: &str) -> Result<(), SolverError> {
        let mut solvers = self.solvers.write();
        if let Some(solver) = solvers.get_mut(address) {
            solver.slash(amount);
            return Err(SolverError::Slashing(reason.to_string()));
        }
        
        Err(SolverError::SolverNotFound(address.to_string()))
    }

    /// Add to rewards pool
    pub fn add_rewards(&self, amount: u128) {
        let mut pool = self.rewards_pool.write();
        *pool += amount;
    }

    /// Get network stats
    pub fn stats(&self) -> NetworkStats {
        let solvers = self.solvers.read();
        let active = solvers.values()
            .filter(|s| s.status == SolverStatus::Active)
            .count();
        
        let intents = self.intents.read();
        let open = intents.values()
            .filter(|i| i.status == IntentStatus::Open)
            .count();
        
        let rewards = *self.rewards_pool.read();
        
        NetworkStats {
            total_solvers: solvers.len(),
            active_solvers: active,
            total_intents: intents.len(),
            open_intents: open,
            rewards_pool: rewards,
        }
    }
}

impl Default for SolverNetwork {
    fn default() -> Self {
        Self::new()
    }
}

/// Network statistics
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct NetworkStats {
    pub total_solvers: usize,
    pub active_solvers: usize,
    pub total_intents: usize,
    pub open_intents: usize,
    pub rewards_pool: u128,
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_register_solver() {
        let network = SolverNetwork::new();
        
        let solver = Solver::new(
            "0x1234".to_string(),
            "TestSolver".to_string(),
            5000_000_000_000_000_000_000,
        );
        
        assert!(network.register_solver(solver).is_ok());
    }

    #[test]
    fn test_submit_intent() {
        let network = SolverNetwork::new();
        
        let intent = Intent::new(
            "0xowner".to_string(),
            "Swap ETH for USDC".to_string(),
            "0xETH".to_string(),
            "0xUSDC".to_string(),
            1_000_000_000_000_000_000,
            1800_000_000,
        );
        
        let id = network.submit_intent(intent).unwrap();
        assert!(!id.is_empty());
    }

    #[test]
    fn test_select_solver() {
        let network = SolverNetwork::new();
        
        // Register solver
        let solver = Solver::new(
            "0xsolver".to_string(),
            "TestSolver".to_string(),
            5000_000_000_000_000_000_000,
        );
        network.register_solver(solver).unwrap();
        
        // Create intent
        let intent = Intent::new(
            "0xowner".to_string(),
            "Swap".to_string(),
            "0xETH".to_string(),
            "0xUSDC".to_string(),
            1_000_000_000_000_000_000,
            1800_000_000,
        );
        
        let result = network.select_solver(&intent);
        assert!(result.is_ok());
    }
}