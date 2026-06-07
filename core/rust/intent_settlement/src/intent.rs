//! Intent Module

use serde::{Deserialize, Serialize};
use uuid::Uuid;

/// Intent
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Intent {
    pub intent_id: String,
    pub owner: String,
    pub sell_token: String,
    pub buy_token: String,
    pub sell_amount: u128,
    pub min_buy_amount: u128,
    pub deadline: i64,
    pub created_at: i64,
    pub status: IntentStatus,
    pub filled_amount: u128,
    pub intent_data: Vec<u8>,
}

impl Intent {
    pub fn new(
        owner: String,
        sell_token: String,
        buy_token: String,
        sell_amount: u128,
        min_buy_amount: u128,
    ) -> Self {
        Self {
            intent_id: Uuid::new_v4().to_string(),
            owner,
            sell_token,
            buy_token,
            sell_amount,
            min_buy_amount,
            deadline: chrono::Utc::now().timestamp() + 3600,
            created_at: chrono::Utc::now().timestamp(),
            status: IntentStatus::Open,
            filled_amount: 0,
            intent_data: Vec::new(),
        }
    }
    
    pub fn is_expired(&self) -> bool {
        chrono::Utc::now().timestamp() > self.deadline
    }
    
    pub fn is_filled(&self) -> bool {
        self.filled_amount >= self.sell_amount
    }
    
    pub fn remaining(&self) -> u128 {
        self.sell_amount.saturating_sub(self.filled_amount)
    }
    
    pub fn fill(&mut self, amount: u128) {
        self.filled_amount += amount;
        if self.is_filled() {
            self.status = IntentStatus::Filled;
        } else {
            self.status = IntentStatus::Partial;
        }
    }
    
    pub fn price(&self) -> f64 {
        self.min_buy_amount as f64 / self.sell_amount as f64
    }
}

/// Intent status
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum IntentStatus {
    Open,
    Partial,
    Filled,
    Expired,
    Cancelled,
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_intent() {
        let intent = Intent::new(
            "owner".to_string(),
            "ETH".to_string(),
            "USDC".to_string(),
            1000,
            900,
        );
        
        assert!(!intent.is_filled());
        assert!(intent.remaining() > 0);
    }
}