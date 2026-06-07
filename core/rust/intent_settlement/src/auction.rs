//! Auction Module

use std::sync::Arc;
use tokio::sync::RwLock;
use std::collections::HashMap;

use serde::{Deserialize, Serialize};
use uuid::Uuid;

use crate::{Intent, IntentError, Solver};

const MIN_STAKE_AMOUNT: u128 = 10_000 * 10u128.pow(18);

/// Intent Auction
pub struct IntentAuction {
    solvers: RwLock<HashMap<String, Solver>>,
}

impl IntentAuction {
    pub fn new() -> Self {
        Self {
            solvers: RwLock::new(HashMap::new()),
        }
    }
    
    /// Register solver for auction
    pub async fn register_solver(&self, solver: Solver) {
        let mut solvers = self.solvers.write().await;
        solvers.insert(solver.solver_id.clone(), solver);
    }
    
    /// Find best solver for intent
    pub async fn find_best_solver(&self, intent: &Intent) -> Option<Solver> {
        let solvers = self.solvers.read().await;
        
        let mut eligible: Vec<&Solver> = solvers
            .values()
            .filter(|s| s.can_fill(intent))
            .collect();
        
        // Sort by score (reputation * stake)
        eligible.sort_by(|a, b| {
            let score_a = a.reputation_score * (a.stake_amount as f64 / MIN_STAKE_AMOUNT as f64);
            let score_b = b.reputation_score * (b.stake_amount as f64 / MIN_STAKE_AMOUNT as f64);
            score_b.partial_cmp(&score_a).unwrap()
        });
        
        eligible.into_iter().next().cloned()
    }
    
    /// Create auction for intent
    pub async fn create_auction(&self, intent_id: &str) -> Auction {
        Auction {
            auction_id: Uuid::new_v4().to_string(),
            intent_id: intent_id.to_string(),
            solvers: Vec::new(),
            status: AuctionStatus::Open,
            created_at: chrono::Utc::now().timestamp(),
        }
    }
}

impl Default for IntentAuction {
    fn default() -> Self {
        Self::new()
    }
}

/// Auction
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Auction {
    pub auction_id: String,
    pub intent_id: String,
    pub solvers: Vec<String>,
    pub status: AuctionStatus,
    pub created_at: i64,
}

impl Auction {
    pub fn add_solver(&mut self, solver_id: &str) {
        if !self.solvers.contains(&solver_id.to_string()) {
            self.solvers.push(solver_id.to_string());
        }
    }
    
    pub fn award(&mut self, solver_id: &str) -> bool {
        if self.solvers.contains(&solver_id.to_string()) {
            self.status = AuctionStatus::Awarded;
            true
        } else {
            false
        }
    }
    
    pub fn is_expired(&self) -> bool {
        chrono::Utc::now().timestamp() - self.created_at > 60 // 1 minute
    }
}

/// Auction status
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum AuctionStatus {
    Open,
    Awarded,
    Expired,
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[tokio::test]
    async fn test_auction() {
        let auction = IntentAuction::new();
        
        let intent = Intent::new(
            "0xowner".to_string(),
            "ETH".to_string(),
            "USDC".to_string(),
            1000,
            900,
        );
        
        let solver = auction.find_best_solver(&intent).await;
        assert!(solver.is_none()); // No solvers registered
    }
}