//! TigerSwap Intent Engine
//! 
//! Intent-based trading system with solver networks, auctions, and intent matching.

mod solver_network;
mod auction_engine;
mod intent_matching;
mod settlement_engine;

pub use solver_network::*;
pub use auction_engine::*;
pub use intent_matching::*;
pub use settlement_engine::*;

use std::collections::HashMap;
use std::sync::{Arc, RwLock};

/// Intent type
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum IntentType {
    Swap,
    LimitOrder,
    CrossChain,
    Arbitrage,
}

/// Intent
#[derive(Debug, Clone)]
pub struct Intent {
    pub id: String,
    pub intent_type: IntentType,
    pub sender: String,
    pub data: Vec<u8>,
    pub hash: [u8; 32],
    pub expiry: u64,
}

/// Intent Engine
pub struct IntentEngine {
    intents: RwLock<HashMap<String, Intent>>,
    solver: Arc<SolverNetwork>,
    auction: Arc<AuctionEngine>,
    matcher: Arc<IntentMatcher>,
    settlement: Arc<SettlementEngine>,
}

impl IntentEngine {
    pub fn new() -> Self {
        Self {
            intents: RwLock::new(HashMap::new()),
            solver: Arc::new(SolverNetwork::new()),
            auction: Arc::new(AuctionEngine::new()),
            matcher: Arc::new(IntentMatcher::new()),
            settlement: Arc::new(SettlementEngine::new()),
        }
    }
    
    pub fn submit_intent(&self, intent: Intent) {
        self.intents.write().unwrap().insert(intent.id.clone(), intent);
    }
    
    pub fn process_intents(&self) {
        let intents: Vec<Intent> = self.intents.read().unwrap().values().cloned().collect();
        
        for intent in intents {
            // Run auction
            if let Ok(bids) = self.auction.run_auction(&intent) {
                // Select solver
                if let Some(winner) = self.solver.select_winner(&bids) {
                    // Execute
                    let _ = self.solver.execute_solution(&intent, &winner);
                }
            }
        }
    }
}

impl Default for IntentEngine {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_intent_engine() {
        let engine = IntentEngine::new();
        assert!(engine.intents.read().unwrap().is_empty());
    }
}