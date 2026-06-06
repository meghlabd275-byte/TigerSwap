//! Solver Network
//! 
//! Manages solver network for intent execution.

use std::collections::HashMap;
use std::sync::{Arc, RwLock};

#[derive(Debug, Clone)]
pub struct Solver {
    pub id: String,
    pub reputation: f64,
    pub success_rate: f64,
    pub bids: Vec<Bid>,
}

#[derive(Debug, Clone)]
pub struct Bid {
    pub solver_id: String,
    pub intent_id: String,
    pub amount: u64,
    pub gas_price: u64,
}

#[derive(Debug, Clone)]
pub struct Solution {
    pub solver_id: String,
    pub intent_id: String,
    pub tx_hash: String,
}

pub struct SolverNetwork {
    solvers: RwLock<HashMap<String, Solver>>,
    solutions: RwLock<HashMap<String, Solution>>,
}

impl SolverNetwork {
    pub fn new() -> Self {
        Self {
            solvers: RwLock::new(HashMap::new()),
            solutions: RwLock::new(HashMap::new()),
        }
    }
    
    pub fn register_solver(&self, id: String) {
        let solver = Solver {
            id: id.clone(),
            reputation: 100.0,
            success_rate: 0.99,
            bids: Vec::new(),
        };
        self.solvers.write().unwrap().insert(id, solver);
    }
    
    pub fn submit_bid(&self, bid: Bid) {
        if let Some(solver) = self.solvers.write().unwrap().get_mut(&bid.solver_id) {
            solver.bids.push(bid);
        }
    }
    
    pub fn select_winner(&self, bids: &[Bid]) -> Option<Bid> {
        if bids.is_empty() {
            return None;
        }
        
        // Select lowest gas price
        bids.iter().min_by_key(|b| b.gas_price).cloned()
    }
    
    pub fn execute_solution(&self, intent: &Intent, bid: &Bid) -> Result<Solution, String> {
        // Simplified execution
        let solution = Solution {
            solver_id: bid.solver_id.clone(),
            intent_id: intent.id.clone(),
            tx_hash: format!("0x{:x}", bid.intent_id.len()),
        };
        
        self.solutions.write().unwrap()
            .insert(bid.intent_id.clone(), solution.clone());
        
        Ok(solution)
    }
    
    pub fn update_reputation(&self, solver_id: &str, success: bool) {
        if let Some(solver) = self.solvers.write().unwrap().get_mut(solver_id) {
            if success {
                solver.reputation = (solver.reputation * 0.99 + 1.0) / 1.01;
            } else {
                solver.reputation *= 0.9;
            }
        }
    }
}

impl Default for SolverNetwork {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_solver_network() {
        let network = SolverNetwork::new();
        network.register_solver("solver-1".to_string());
        assert!(network.solvers.read().unwrap().contains_key("solver-1"));
    }
}