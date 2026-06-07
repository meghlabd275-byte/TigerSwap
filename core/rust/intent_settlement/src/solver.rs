//! Solver Module

use serde::{Deserialize, Serialize};
use uuid::Uuid;

pub const MIN_STAKE_AMOUNT: u128 = 10_000 * 10u128.pow(18);

/// Solver
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Solver {
    pub solver_id: String,
    pub address: String,
    pub stake_amount: u128,
    pub reputation_score: f64,
    pub total_fills: u64,
    pub successful_fills: u64,
    pub failed_fills: u64,
    pub total_rewards: u128,
    pub total_slashed: u128,
    pub registered_at: i64,
    pub last_active: i64,
    pub status: SolverStatus,
    pub capabilities: Vec<String>,
}

impl Solver {
    pub fn new(address: String, stake_amount: u128) -> Self {
        Self {
            solver_id: Uuid::new_v4().to_string(),
            address,
            stake_amount,
            reputation_score: 100.0,
            total_fills: 0,
            successful_fills: 0,
            failed_fills: 0,
            total_rewards: 0,
            total_slashed: 0,
            registered_at: chrono::Utc::now().timestamp(),
            last_active: chrono::Utc::now().timestamp(),
            status: SolverStatus::Active,
            capabilities: vec!["swap".to_string()],
        }
    }
    
    pub fn success_rate(&self) -> f64 {
        if self.total_fills == 0 {
            return 100.0;
        }
        (self.successful_fills as f64 / self.total_fills as f64) * 100.0
    }
    
    pub fn record_fill(&mut self, success: bool, reward: u128) {
        self.total_fills += 1;
        self.last_active = chrono::Utc::now().timestamp();
        
        if success {
            self.successful_fills += 1;
            self.total_rewards += reward;
        } else {
            self.failed_fills += 1;
        }
        
        self.reputation_score = (self.successful_fills as f64 / self.total_fills as f64) * 100.0;
    }
    
    pub fn slash(&mut self, amount: u128) {
        self.total_slashed += amount;
        self.stake_amount = self.stake_amount.saturating_sub(amount);
        
        if self.stake_amount < MIN_STAKE_AMOUNT {
            self.status = SolverStatus::Slashed;
        }
    }
    
    pub fn can_fill(&self, intent: &super::Intent) -> bool {
        self.status == SolverStatus::Active && 
        self.stake_amount >= MIN_STAKE_AMOUNT &&
        self.capabilities.contains(&"swap".to_string())
    }
}

/// Solver status
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum SolverStatus {
    Active,
    Inactive,
    Slashed,
    Jailed,
}

/// Solver statistics
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SolverStats {
    pub solver_id: String,
    pub total_fills: u64,
    pub successful_fills: u64,
    pub failed_fills: u64,
    pub success_rate: f64,
    pub total_rewards: u128,
    pub total_slashed: u128,
    pub reputation_score: f64,
}

impl From<&Solver> for SolverStats {
    fn from(solver: &Solver) -> Self {
        Self {
            solver_id: solver.solver_id.clone(),
            total_fills: solver.total_fills,
            successful_fills: solver.successful_fills,
            failed_fills: solver.failed_fills,
            success_rate: solver.success_rate(),
            total_rewards: solver.total_rewards,
            total_slashed: solver.total_slashed,
            reputation_score: solver.reputation_score,
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_solver() {
        let solver = Solver::new("0xsolver".to_string(), 20_000 * 10u128.pow(18));
        
        assert_eq!(solver.status, SolverStatus::Active);
        assert!(solver.can_fill(&Intent::new("0xowner".to_string(), "ETH".to_string(), "USDC".to_string(), 1000, 900)));
    }
}