//! Settlement Engine
//! 
//! Handles settlement of trades, funding payments, and liquidations.

use std::collections::HashMap;
use std::sync::{Arc, RwLock};
use std::time::{SystemTime, UNIX_EPOCH};
use thiserror::Error;

#[derive(Error, Debug)]
pub enum SettlementError {
    #[error("Settlement failed: {0}")]
    SettlementFailed(String),
    #[error("Insufficient balance: {0}")]
    InsufficientBalance(String),
}

// ============================================================================
// Types
// ============================================================================

#[derive(Debug, Clone)]
pub struct Settlement {
    pub settlement_id: String,
    pub user: String,
    pub market: String,
    pub amount: i64,  // Positive = receive, negative = pay
    pub type_: SettlementType,
    pub timestamp: u64,
    pub status: SettlementStatus,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum SettlementType {
    Trade,
    Funding,
    Liquidation,
    Withdrawal,
    Deposit,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum SettlementStatus {
    Pending,
    Completed,
    Failed,
}

// ============================================================================
// Settlement Engine
// ============================================================================

pub struct SettlementEngine {
    pending: RwLock<Vec<Settlement>>,
    completed: RwLock<HashMap<String, Settlement>>,
    balances: RwLock<HashMap<String, i64>>,
}

impl SettlementEngine {
    pub fn new() -> Self {
        Self {
            pending: RwLock::new(Vec::new()),
            completed: RwLock::new(HashMap::new()),
            balances: RwLock::new(HashMap::new()),
        }
    }
    
    pub fn credit(&self, user: &str, amount: i64) {
        let mut balances = self.balances.write().unwrap();
        *balances.entry(user.to_string()).or_insert(0) += amount;
    }
    
    pub fn debit(&self, user: &str, amount: i64) -> Result<(), SettlementError> {
        let mut balances = self.balances.write().unwrap();
        let balance = balances.entry(user.to_string()).or_insert(0);
        
        if *balance < amount {
            return Err(SettlementError::InsufficientBalance(format!(
                "need {}, have {}", amount, balance
            )));
        }
        
        *balance -= amount;
        Ok(())
    }
    
    pub fn get_balance(&self, user: &str) -> i64 {
        self.balances.read().unwrap().get(user).copied().unwrap_or(0)
    }
    
    pub fn create_settlement(
        &self,
        user: &str,
        market: &str,
        amount: i64,
        type_: SettlementType,
    ) -> String {
        let id = format!("{}-{}-{}", user, market, current_timestamp());
        let settlement = Settlement {
            settlement_id: id.clone(),
            user: user.to_string(),
            market: market.to_string(),
            amount,
            type_,
            timestamp: current_timestamp(),
            status: SettlementStatus::Pending,
        };
        
        self.pending.write().unwrap().push(settlement);
        id
    }
    
    pub fn process_settlement(&self, id: &str) -> Result<(), SettlementError> {
        let mut pending = self.pending.write().unwrap();
        let idx = pending.iter().position(|s| s.settlement_id == id)
            .ok_or_else(|| SettlementError::SettlementFailed("not found".to_string()))?;
        
        let settlement = pending.remove(idx);
        
        if settlement.amount > 0 {
            self.credit(&settlement.user, settlement.amount);
        } else {
            self.debit(&settlement.user, settlement.amount.abs())?;
        }
        
        let mut completed = self.completed.write().unwrap();
        let mut settled = settlement;
        settled.status = SettlementStatus::Completed;
        completed.insert(id.to_string(), settled);
        
        Ok(())
    }
    
    pub fn get_pending(&self) -> Vec<Settlement> {
        self.pending.read().unwrap().clone()
    }
    
    pub fn get_completed(&self, user: &str) -> Vec<Settlement> {
        self.completed.read().unwrap()
            .values()
            .filter(|s| s.user == user)
            .cloned()
            .collect()
    }
}

fn current_timestamp() -> u64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap()
        .as_secs()
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_credit_debit() {
        let engine = SettlementEngine::new();
        engine.credit("user1", 10000);
        engine.debit("user1", 5000).unwrap();
        assert_eq!(engine.get_balance("user1"), 5000);
    }
    
    #[test]
    fn test_settlement() {
        let engine = SettlementEngine::new();
        engine.credit("user1", 10000);
        
        let id = engine.create_settlement("user1", "ETH-USD", 1000, SettlementType::Trade);
        engine.process_settlement(&id).unwrap();
        
        assert_eq!(engine.get_balance("user1"), 11000);
    }
}