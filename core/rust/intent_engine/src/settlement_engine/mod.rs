//! Settlement Engine for Intent
//! 
//! Settles executed intents.

use std::collections::HashMap;
use std::sync::{Arc, RwLock};

#[derive(Debug, Clone)]
pub struct Settlement {
    pub settlement_id: String,
    pub intent_id: String,
    pub solver_id: String,
    pub amount: u64,
    pub status: SettlementStatus,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum SettlementStatus {
    Pending,
    Executed,
    Failed,
    Cancelled,
}

pub struct SettlementEngine {
    settlements: RwLock<HashMap<String, Settlement>>,
}

impl SettlementEngine {
    pub fn new() -> Self {
        Self {
            settlements: RwLock::new(HashMap::new()),
        }
    }
    
    pub fn create_settlement(
        &self,
        intent_id: &str,
        solver_id: &str,
        amount: u64,
    ) -> String {
        let id = format!("settlement-{}-{}", intent_id, current_timestamp());
        
        let settlement = Settlement {
            settlement_id: id.clone(),
            intent_id: intent_id.to_string(),
            solver_id: solver_id.to_string(),
            amount,
            status: SettlementStatus::Pending,
        };
        
        self.settlements.write().unwrap().insert(id.clone(), settlement);
        id
    }
    
    pub fn execute(&self, settlement_id: &str) -> Result<(), String> {
        let mut settlements = self.settlements.write().unwrap();
        let settlement = settlements.get_mut(settlement_id)
            .ok_or("settlement not found")?;
        
        settlement.status = SettlementStatus::Executed;
        
        Ok(())
    }
    
    pub fn fail(&self, settlement_id: &str) {
        if let Some(settlement) = self.settlements.write().unwrap().get_mut(settlement_id) {
            settlement.status = SettlementStatus::Failed;
        }
    }
    
    pub fn get_settlement(&self, settlement_id: &str) -> Option<Settlement> {
        self.settlements.read().unwrap().get(settlement_id).cloned()
    }
}

fn current_timestamp() -> u64 {
    std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .unwrap()
        .as_secs()
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_settlement() {
        let engine = SettlementEngine::new();
        let id = engine.create_settlement("intent-1", "solver-1", 1000);
        assert!(!id.is_empty());
    }
}