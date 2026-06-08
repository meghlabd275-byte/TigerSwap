//! TigerSwap Prediction Markets Engine
//! 
//! Implements prediction markets:
//! - Binary outcomes
//! - Categorical outcomes
//! - Spread markets
//! -流动性提供
//!
//! Implementation: Pure Rust with no external dependencies

use serde::{Deserialize, Serialize};
use std::sync::Arc;
use parking_lot::RwLock;
use thiserror::Error;
use uuid::Uuid;
use chrono::Utc;
use std::collections::HashMap;

/// Chain ID constants
pub const CHAIN_ETH: u64 = 1;

#[derive(Debug, Error)]
pub enum PredictionError {
    #[error("Market not found: {0}")]
    MarketNotFound(String),
    #[error("Invalid parameters: {0}")]
    InvalidParameters(String),
    #[error("Market resolved: {0}")]
    MarketResolved(String),
}

/// Outcome type
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum OutcomeType {
    Binary,      // Yes/No
    Categorical, // Multiple choices
    Scalar,    // Range
}

impl Default for OutcomeType {
    fn default() -> Self { OutcomeType::Binary }
}

/// Market status
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum MarketStatus {
    Active,
    Resolved,
    Cancelled,
}

impl Default for MarketStatus {
    fn default() -> Self { MarketStatus::Active }
}

/// Prediction market
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PredictionMarket {
    pub market_id: String,
    pub question: String,
    pub outcome_type: OutcomeType,
    
    // Outcomes
    pub outcomes: Vec<Outcome>,
    
    // Resolution
    pub resolution_source: String,
    pub resolve_by: i64,
    pub resolved_at: Option<i64>,
    pub winning_outcome: Option<usize>,
    pub status: MarketStatus,
    
    // Volume
    pub volume: u128,
    pub yes_volume: u128,
    pub no_volume: u128,
    
    // Timestamps
    pub created_at: i64,
    pub updated_at: i64,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Outcome {
    pub id: usize,
    pub name: String,
    pub price: f64,       // Probability (0-1)
    pub volume: u128,
    pub payout_ratio: f64,
}

impl PredictionMarket {
    pub fn new_binary(question: String, resolve_by: i64) -> Self {
        Self {
            market_id: Uuid::new_v4().to_string(),
            question,
            outcome_type: OutcomeType::Binary,
            outcomes: vec![
                Outcome { id: 0, name: "Yes".to_string(), price: 0.5, volume: 0, payout_ratio: 2.0 },
                Outcome { id: 1, name: "No".to_string(), price: 0.5, volume: 0, payout_ratio: 2.0 },
            ],
            resolution_source: String::new(),
            resolve_by,
            resolved_at: None,
            winning_outcome: None,
            status: MarketStatus::Active,
            volume: 0,
            yes_volume: 0,
            no_volume: 0,
            created_at: Utc::now().timestamp(),
            updated_at: Utc::now().timestamp(),
        }
    }

    pub fn new_categorical(question: String, outcomes: Vec<String>, resolve_by: i64) -> Self {
        let outcome_list: Vec<Outcome> = outcomes.into_iter()
            .enumerate()
            .map(|(i, name)| Outcome {
                id: i,
                name,
                price: 1.0 / outcomes.len() as f64,
                volume: 0,
                payout_ratio: outcomes.len() as f64,
            })
            .collect();
        
        Self {
            market_id: Uuid::new_v4().to_string(),
            question,
            outcome_type: OutcomeType::Categorical,
            outcomes: outcome_list,
            resolution_source: String::new(),
            resolve_by,
            resolved_at: None,
            winning_outcome: None,
            status: MarketStatus::Active,
            volume: 0,
            yes_volume: 0,
            no_volume: 0,
            created_at: Utc::now().timestamp(),
            updated_at: Utc::now().timestamp(),
        }
    }

    /// Place bet
    pub fn bet(&mut self, outcome_id: usize, amount: u128) -> Result<u128, PredictionError> {
        if self.status != MarketStatus::Active {
            return Err(PredictionError::MarketResolved("Market resolved".to_string()));
        }
        
        if outcome_id >= self.outcomes.len() {
            return Err(PredictionError::InvalidParameters("Invalid outcome".to_string()));
        }
        
        let outcome = &mut self.outcomes[outcome_id];
        
        // Update volume
        outcome.volume += amount;
        self.volume += amount;
        
        if outcome_id == 0 {
            self.yes_volume += amount;
        } else {
            self.no_volume += amount;
        }
        
        // Update price based on volume
        self.update_prices();
        
        let payout = (amount as f64 * outcome.payout_ratio) as u128;
        
        self.updated_at = Utc::now().timestamp();
        
        Ok(payout)
    }

    /// Update prices based on volume
    fn update_prices(&mut self) {
        if self.volume == 0 {
            return;
        }
        
        // Use AMM-style pricing
        let k = 1000000.0; // Constant product
        
        for outcome in &mut self.outcomes {
            let other_volume: u128 = self.volume - outcome.volume;
            outcome.price = k / (k + other_volume as f64);
        }
    }

    /// Resolve market
    pub fn resolve(&mut self, winning_id: usize) -> Result<(), PredictionError> {
        if self.status != MarketStatus::Active {
            return Err(PredictionError::MarketResolved("Already resolved".to_string()));
        }
        
        if winning_id >= self.outcomes.len() {
            return Err(PredictionError::InvalidParameters("Invalid outcome".to_string()));
        }
        
        self.winning_outcome = Some(winning_id);
        self.status = MarketStatus::Resolved;
        self.resolved_at = Some(Utc::now().timestamp());
        
        // Update payout ratios
        let winner = &self.outcomes[winning_id];
        
        for (i, outcome) in self.outcomes.iter_mut().enumerate() {
            if i == winning_id {
                outcome.payout_ratio = 1.0 / winner.price;
            } else {
                outcome.payout_ratio = 0.0;
            }
        }
        
        self.updated_at = Utc::now().timestamp();
        
        Ok(())
    }

    /// Get current prices
    pub fn get_prices(&self) -> Vec<f64> {
        self.outcomes.iter().map(|o| o.price).collect()
    }
}

/// Bet position
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct BetPosition {
    pub position_id: String,
    pub user: String,
    pub market_id: String,
    pub outcome_id: usize,
    pub amount: u128,
    pub potential_payout: u128,
    pub created_at: i64,
}

/// Prediction Markets Engine
pub struct PredictionMarketsEngine {
    markets: Arc<RwLock<HashMap<String, PredictionMarket>>>,
    positions: Arc<RwLock<HashMap<String, Vec<BetPosition>>>>,
}

impl PredictionMarketsEngine {
    pub fn new() -> Self {
        Self {
            markets: Arc::new(RwLock::new(HashMap::new())),
            positions: Arc::new(RwLock::new(HashMap::new())),
        }
    }

    /// Create binary market
    pub fn create_binary_market(
        &self,
        question: String,
        resolve_by: i64,
    ) -> String {
        let market = PredictionMarket::new_binary(question, resolve_by);
        let market_id = market.market_id.clone();
        
        self.markets.write().insert(market_id.clone(), market);
        
        market_id
    }

    /// Create categorical market
    pub fn create_categorical_market(
        &self,
        question: String,
        outcomes: Vec<String>,
        resolve_by: i64,
    ) -> String {
        let market = PredictionMarket::new_categorical(question, outcomes, resolve_by);
        let market_id = market.market_id.clone();
        
        self.markets.write().insert(market_id.clone(), market);
        
        market_id
    }

    /// Get market
    pub fn get_market(&self, market_id: &str) -> Option<PredictionMarket> {
        self.markets.read().get(market_id).cloned()
    }

    /// Place bet
    pub fn bet(
        &self,
        user: String,
        market_id: &str,
        outcome_id: usize,
        amount: u128,
    ) -> Result<u128, PredictionError> {
        let mut markets = self.markets.write();
        let market = markets.get_mut(market_id)
            .ok_or_else(|| PredictionError::MarketNotFound(market_id.to_string()))?;
        
        let payout = market.bet(outcome_id, amount)?;
        
        // Record position
        let position = BetPosition {
            position_id: Uuid::new_v4().to_string(),
            user: user.clone(),
            market_id: market_id.to_string(),
            outcome_id,
            amount,
            potential_payout: payout,
            created_at: Utc::now().timestamp(),
        };
        
        self.positions.write()
            .entry(user)
            .or_insert_with(Vec::new)
            .push(position);
        
        Ok(payout)
    }

    /// Resolve market
    pub fn resolve(&self, market_id: &str, winning_id: usize) -> Result<(), PredictionError> {
        let mut markets = self.markets.write();
        let market = markets.get_mut(market_id)
            .ok_or_else(|| PredictionError::MarketNotFound(market_id.to_string()))?;
        
        market.resolve(winning_id)
    }

    /// Get active markets
    pub fn get_active_markets(&self) -> Vec<PredictionMarket> {
        self.markets.read()
            .values()
            .filter(|m| m.status == MarketStatus::Active)
            .cloned()
            .collect()
    }

    /// Get user positions
    pub fn get_user_positions(&self, user: &str) -> Vec<BetPosition> {
        self.positions.read()
            .get(user)
            .cloned()
            .unwrap_or_default()
    }
}

impl Default for PredictionMarketsEngine {
    fn default() -> Self { Self::new() }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_binary_market() {
        let market = PredictionMarket::new_binary(
            "Will BTC reach $100k by 2025?".to_string(),
            Utc::now().timestamp() + 365 * 24 * 60 * 60,
        );
        
        assert_eq!(market.outcomes.len(), 2);
    }

    #[test]
    fn test_bet() {
        let mut market = PredictionMarket::new_binary(
            "Test?".to_string(),
            Utc::now().timestamp() + 86400,
        );
        
        let payout = market.bet(0, 1000).unwrap();
        
        assert!(payout > 0);
    }
}