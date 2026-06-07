//! Network Module - High-level Intent Settlement Network

use std::sync::Arc;

use crate::{SettlementEngine, IntentAuction, Intent, Solver, Fill, IntentError, Auction, IntentSettlementNetwork};

impl IntentSettlementNetwork {
    pub fn new() -> Self {
        let engine = Arc::new(SettlementEngine::new());
        let auction = Arc::new(IntentAuction::new());
        
        Self { engine, auction }
    }
    
    pub fn engine(&self) -> &Arc<SettlementEngine> {
        &self.engine
    }
    
    pub fn auction(&self) -> &Arc<IntentAuction> {
        &self.auction
    }
    
    /// Submit new intent
    pub async fn submit_intent(&self, intent: Intent) -> String {
        self.engine.register_intent(intent).await
    }
    
    /// Register solver with stake
    pub async fn register_solver(&self, address: String, stake_amount: u128) -> Result<String, IntentError> {
        let solver = Solver::new(address, stake_amount);
        self.engine.register_solver(solver).await
    }
    
    /// Fill intent
    pub async fn fill_intent(
        &self,
        intent_id: &str,
        solver_id: &str,
        fill_amount: u128,
    ) -> Result<Fill, IntentError> {
        self.engine.submit_fill(intent_id, solver_id, fill_amount).await
    }
    
    /// Get intent by ID
    pub async fn get_intent(&self, intent_id: &str) -> Option<Intent> {
        self.engine.get_intent(intent_id).await
    }
    
    /// Get solver by ID
    pub async fn get_solver(&self, solver_id: &str) -> Option<Solver> {
        self.engine.get_solver(solver_id).await
    }
    
    /// Get active solvers
    pub async fn get_active_solvers(&self) -> Vec<Solver> {
        self.engine.get_active_solvers().await
    }
    
    /// Create auction for intent
    pub async fn create_auction(&self, intent_id: &str) -> Auction {
        self.auction.create_auction(intent_id).await
    }
    
    /// Get pending fills
    pub async fn get_pending_fills(&self) -> Vec<Fill> {
        self.engine.get_pending_fills().await
    }
}

impl Default for IntentSettlementNetwork {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[tokio::test]
    async fn test_network() {
        let network = IntentSettlementNetwork::new();
        
        // Register solver
        let solver_id = network
            .register_solver("0xsolver".to_string(), 20_000 * 10u128.pow(18))
            .await
            .unwrap();
        
        // Submit intent
        let intent = Intent::new(
            "0xowner".to_string(),
            "ETH".to_string(),
            "USDC".to_string(),
            1000,
            900,
        );
        let intent_id = network.submit_intent(intent).await;
        
        // Fill intent
        let fill = network.fill_intent(&intent_id, &solver_id, 1000).await.unwrap();
        
        assert_eq!(fill.fill_amount, 1000);
    }
}