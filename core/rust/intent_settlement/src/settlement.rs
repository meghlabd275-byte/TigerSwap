//! Settlement Module

use std::collections::HashMap;
use std::sync::Arc;
use tokio::sync::RwLock;

use serde::{Deserialize, Serialize};
use uuid::Uuid;

use crate::{Intent, IntentError, Solver, SolverStatus, IntentStatus};

const MIN_STAKE_AMOUNT: u128 = 10_000 * 10u128.pow(18);

/// Settlement Engine
pub struct SettlementEngine {
    intents: RwLock<HashMap<String, Intent>>,
    solvers: RwLock<HashMap<String, Solver>>,
    fills: RwLock<HashMap<String, Fill>>,
}

impl SettlementEngine {
    pub fn new() -> Self {
        Self {
            intents: RwLock::new(HashMap::new()),
            solvers: RwLock::new(HashMap::new()),
            fills: RwLock::new(HashMap::new()),
        }
    }
    
    /// Register intent
    pub async fn register_intent(&self, intent: Intent) -> String {
        let intent_id = intent.intent_id.clone();
        let mut intents = self.intents.write().await;
        intents.insert(intent_id.clone(), intent);
        intent_id
    }
    
    /// Get intent
    pub async fn get_intent(&self, intent_id: &str) -> Option<Intent> {
        let intents = self.intents.read().await;
        intents.get(intent_id).cloned()
    }
    
    /// Register solver
    pub async fn register_solver(&self, solver: Solver) -> Result<String, IntentError> {
        if solver.stake_amount < MIN_STAKE_AMOUNT {
            return Err(IntentError::InsufficientStake);
        }
        
        let solver_id = solver.solver_id.clone();
        let mut solvers = self.solvers.write().await;
        solvers.insert(solver_id.clone(), solver);
        Ok(solver_id)
    }
    
    /// Get solver
    pub async fn get_solver(&self, solver_id: &str) -> Option<Solver> {
        let solvers = self.solvers.read().await;
        solvers.get(solver_id).cloned()
    }
    
    /// Get active solvers
    pub async fn get_active_solvers(&self) -> Vec<Solver> {
        let solvers = self.solvers.read().await;
        solvers
            .values()
            .filter(|s| s.status == crate::SolverStatus::Active)
            .cloned()
            .collect()
    }
    
    /// Submit fill
    pub async fn submit_fill(
        &self,
        intent_id: &str,
        solver_id: &str,
        fill_amount: u128,
    ) -> Result<Fill, IntentError> {
        // Get and validate intent
        let mut intents = self.intents.write().await;
        let intent = intents
            .get_mut(intent_id)
            .ok_or(IntentError::IntentNotFound)?;
        
        if intent.status == crate::IntentStatus::Filled {
            return Err(IntentError::IntentFilled);
        }
        
        if intent.is_expired() {
            intent.status = crate::IntentStatus::Expired;
            return Err(IntentError::IntentExpired);
        }
        
        drop(intents);
        
        // Get and validate solver
        let mut solvers = self.solvers.write().await;
        let solver = solvers
            .get_mut(solver_id)
            .ok_or(IntentError::SolverNotFound)?;
        
        if solver.status != crate::SolverStatus::Active {
            return Err(IntentError::InvalidSolver);
        }
        
        // Update intent
        let mut intents = self.intents.write().await;
        let intent = intents.get_mut(intent_id).unwrap();
        intent.fill(fill_amount);
        
        // Calculate reward
        let reward = fill_amount * 1000 / 1000000;
        solver.record_fill(true, reward);
        
        // Create fill
        let fill = Fill {
            fill_id: Uuid::new_v4().to_string(),
            intent_id: intent_id.to_string(),
            solver_id: solver_id.to_string(),
            fill_amount,
            created_at: chrono::Utc::now().timestamp(),
            status: FillStatus::Pending,
        };
        
        let fill_id = fill.fill_id.clone();
        let mut fill_store = self.fills.write().await;
        fill_store.insert(fill_id, fill.clone());
        
        Ok(fill)
    }
    
    /// Get fill
    pub async fn get_fill(&self, fill_id: &str) -> Option<Fill> {
        let fills = self.fills.read().await;
        fills.get(fill_id).cloned()
    }
    
    /// Get pending fills
    pub async fn get_pending_fills(&self) -> Vec<Fill> {
        let fills = self.fills.read().await;
        fills
            .values()
            .filter(|f| f.status == FillStatus::Pending)
            .cloned()
            .collect()
    }
    
    /// Confirm fill
    pub async fn confirm_fill(&self, fill_id: &str) -> Result<(), IntentError> {
        let mut fills = self.fills.write().await;
        let fill = fills
            .get_mut(fill_id)
            .ok_or(IntentError::IntentNotFound)?;
        
        fill.status = FillStatus::Confirmed;
        
        Ok(())
    }
    
    /// Cancel fill
    pub async fn cancel_fill(&self, fill_id: &str) -> Result<(), IntentError> {
        let mut fills = self.fills.write().await;
        let fill = fills
            .get_mut(fill_id)
            .ok_or(IntentError::IntentNotFound)?;
        
        fill.status = FillStatus::Failed;
        
        // Get intent and revert
        let mut intents = self.intents.write().await;
        if let Some(intent) = intents.get_mut(&fill.intent_id) {
            intent.filled_amount = intent.filled_amount.saturating_sub(fill.fill_amount);
            if intent.filled_amount == 0 {
                intent.status = crate::IntentStatus::Open;
            }
        }
        
        Ok(())
    }
}

impl Default for SettlementEngine {
    fn default() -> Self {
        Self::new()
    }
}

/// Fill
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Fill {
    pub fill_id: String,
    pub intent_id: String,
    pub solver_id: String,
    pub fill_amount: u128,
    pub created_at: i64,
    pub status: FillStatus,
}

/// Fill status
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum FillStatus {
    Pending,
    Confirmed,
    Failed,
}

/// Pending fill
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PendingFill {
    pub fill_id: String,
    pub intent_id: String,
    pub solver_id: String,
    pub fill_amount: u128,
    pub created_at: i64,
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[tokio::test]
    async fn test_settlement_engine() {
        let engine = SettlementEngine::new();
        
        // Register intent
        let intent = Intent::new(
            "0xowner".to_string(),
            "ETH".to_string(),
            "USDC".to_string(),
            1000,
            900,
        );
        let intent_id = engine.register_intent(intent).await;
        
        // Register solver
        let solver = Solver::new("0xsolver".to_string(), 20_000 * 10u128.pow(18));
        let solver_id = engine.register_solver(solver).await.unwrap();
        
        // Submit fill
        let fill = engine.submit_fill(&intent_id, &solver_id, 1000).await.unwrap();
        
        assert_eq!(fill.fill_amount, 1000);
    }
}