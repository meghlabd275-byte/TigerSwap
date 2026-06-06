//! Funding Engine
//! 
//! Handles funding rate calculations and payments for perpetual contracts.
//! Supports fixed rate, pegged index, and premium/discount models.

use std::collections::HashMap;
use std::sync::{Arc, RwLock};
use std::time::{SystemTime, UNIX_EPOCH};
use thiserror::Error;

#[derive(Error, Debug)]
pub enum FundingError {
    #[error("Market not found: {0}")]
    MarketNotFound(String),
    #[error("Calculation failed: {0}")]
    CalculationFailed(String),
    #[error("Invalid rate: {0}")]
    InvalidRate(String),
}

// ============================================================================
// Types
// ============================================================================

/// Funding rate
#[derive(Debug, Clone)]
pub struct FundingRate {
    pub market: String,
    pub rate: i64,          // Rate in 1e8 (0.00000001)
    pub next_funding_time: u64,
    pub period_hours: u64,
}

/// Funding payment
#[derive(Debug, Clone)]
pub struct FundingPayment {
    pub user: String,
    pub market: String,
    pub side: bool,         // true = long pays, false = short receives
    pub payment: i64,
    pub rate: i64,
    pub position_size: i64,
    pub index_price: u64,
    pub mark_price: u64,
}

/// Funding history entry
#[derive(Debug, Clone)]
pub struct FundingHistoryEntry {
    pub timestamp: u64,
    pub market: String,
    pub rate: i64,
    pub index_price: u64,
    pub mark_price: u64,
}

/// Funding model
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum FundingModel {
    Fixed,          // Fixed rate
    PeggedIndex,    // Pegged to external index
    Premium,        // Premium/discount based on mark - index
    Hybrid,         // Combination of above
}

// ============================================================================
// Funding Engine
// ============================================================================

pub struct FundingEngine {
    markets: RwLock<HashMap<String, MarketFunding>>,
    history: RwLock<HashMap<String, Vec<FundingHistoryEntry>>>,
    config: FundingConfig,
}

#[derive(Debug, Clone)]
pub struct MarketFunding {
    pub market: String,
    pub model: FundingModel,
    pub base_rate: i64,        // Fixed rate in 1e8
    pub rate_cap: i64,         // Maximum rate
    pub rate_floor: i64,      // Minimum rate
    pub period_hours: u64,    // Funding period (usually 8 hours)
    pub next_funding_time: u64,
    pub index_price: u64,
    pub mark_price: u64,
}

#[derive(Debug, Clone)]
pub struct FundingConfig {
    pub default_period_hours: u64,
    pub default_base_rate: i64,
    pub rate_cap: i64,
    pub rate_floor: i64,
    pub clamp_rate: i64,
}

impl Default for FundingConfig {
    fn default() -> Self {
        Self {
            default_period_hours: 8,
            default_base_rate: 0, // 0% default
            rate_cap: 1_00000000, // 100% cap (high for volatile markets)
            rate_floor: -1_00000000, // -100% floor
            clamp_rate: 1_0000000, // 1% max change per period
        }
    }
}

impl FundingEngine {
    pub fn new() -> Self {
        Self {
            markets: RwLock::new(HashMap::new()),
            history: RwLock::new(HashMap::new()),
            config: FundingConfig::default(),
        }
    }
    
    /// Register market for funding
    pub fn register_market(
        &self,
        market: String,
        model: FundingModel,
        base_rate: Option<i64>,
    ) {
        let funding = MarketFunding {
            market: market.clone(),
            model,
            base_rate: base_rate.unwrap_or(self.config.default_base_rate),
            rate_cap: self.config.rate_cap,
            rate_floor: self.config.rate_floor,
            period_hours: self.config.default_period_hours,
            next_funding_time: current_timestamp() + self.config.default_period_hours * 3600,
            index_price: 0,
            mark_price: 0,
        };
        
        self.markets.write().unwrap().insert(market, funding);
    }
    
    /// Update prices for funding calculation
    pub fn update_prices(&self, market: &str, index_price: u64, mark_price: u64) {
        let mut markets = self.markets.write().unwrap();
        if let Some(funding) = markets.get_mut(market) {
            funding.index_price = index_price;
            funding.mark_price = mark_price;
        }
    }
    
    /// Calculate funding rate
    pub fn calculate_rate(&self, market: &str) -> Result<FundingRate, FundingError> {
        let markets = self.markets.read().unwrap();
        let funding = markets.get(market)
            .ok_or_else(|| FundingError::MarketNotFound(market.to_string()))?;
        
        let rate = match funding.model {
            FundingModel::Fixed => funding.base_rate,
            FundingModel::PeggedIndex => funding.base_rate,
            FundingModel::Premium => {
                // Premium = mark - index
                if funding.index_price == 0 {
                    return Err(FundingError::CalculationFailed(
                        "index price not set".to_string()
                    ));
                }
                
                let premium = ((funding.mark_price as i64 - funding.index_price as i64) 
                    * 1_00000000) / funding.index_price as i64;
                
                // Clamp premium
                premium.clamp(funding.rate_floor, funding.rate_cap)
            },
            FundingModel::Hybrid => {
                // Base + premium component
                let base = funding.base_rate;
                let premium = if funding.index_price > 0 {
                    ((funding.mark_price as i64 - funding.index_price as i64) 
                        * 1_00000000) / funding.index_price as i64
                } else {
                    0
                };
                
                (base + premium / 2).clamp(funding.rate_floor, funding.rate_cap)
            },
        };
        
        Ok(FundingRate {
            market: market.to_string(),
            rate,
            next_funding_time: funding.next_funding_time,
            period_hours: funding.period_hours,
        })
    }
    
    /// Calculate funding payment for user
    pub fn calculate_payment(
        &self,
        user: &str,
        market: &str,
        position_size: i64,
        side: bool, // true = long
    ) -> Result<FundingPayment, FundingError> {
        let funding_rate = self.calculate_rate(market)?;
        
        let markets = self.markets.read().unwrap();
        let market_funding = markets.get(market)
            .ok_or_else(|| FundingError::MarketNotFound(market.to_string()))?;
        
        // Calculate payment: rate * position * price
        let position_value = position_size.unsigned_abs() * market_funding.index_price;
        let payment = (funding_rate.rate as i128 * position_value as i128 / 1_00000000_i128) as i64;
        
        Ok(FundingPayment {
            user: user.to_string(),
            market: market.to_string(),
            side,
            payment,
            rate: funding_rate.rate,
            position_size,
            index_price: market_funding.index_price,
            mark_price: market_funding.mark_price,
        })
    }
    
    /// Process funding payments
    pub fn process_funding(&self, market: &str) -> Result<Vec<FundingPayment>, FundingError> {
        // In production: get all positions for market
        // Simplified: return empty
        Ok(Vec::new())
    }
    
    /// Advance to next funding period
    pub fn advance_period(&self, market: &str) -> Result<(), FundingError> {
        let mut markets = self.markets.write().unwrap();
        let funding = markets.get_mut(market)
            .ok_or_else(|| FundingError::MarketNotFound(market.to_string()))?;
        
        // Record history
        let history_entry = FundingHistoryEntry {
            timestamp: current_timestamp(),
            market: market.to_string(),
            rate: 0, // Would be calculated rate
            index_price: funding.index_price,
            mark_price: funding.mark_price,
        };
        
        self.history.write().unwrap()
            .entry(market.to_string())
            .or_insert_with(Vec::new)
            .push(history_entry);
        
        // Advance time
        funding.next_funding_time += funding.period_hours * 3600;
        
        Ok(())
    }
    
    /// Get funding history
    pub fn get_history(&self, market: &str, limit: usize) -> Vec<FundingHistoryEntry> {
        self.history.read().unwrap()
            .get(market)
            .map(|h| h.iter().rev().take(limit).cloned().collect())
            .unwrap_or_default()
    }
    
    /// Set funding rate manually
    pub fn set_rate(&self, market: &str, rate: i64) -> Result<(), FundingError> {
        if rate < self.config.rate_floor || rate > self.config.rate_cap {
            return Err(FundingError::InvalidRate(
                format!("rate {} outside bounds", rate)
            ));
        }
        
        let mut markets = self.markets.write().unwrap();
        let funding = markets.get_mut(market)
            .ok_or_else(|| FundingError::MarketNotFound(market.to_string()))?;
        
        funding.base_rate = rate;
        Ok(())
    }
}

// ============================================================================
// Helper Functions
// ============================================================================

fn current_timestamp() -> u64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap()
        .as_secs()
}

// ============================================================================
// Tests
// ============================================================================

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_funding_engine_creation() {
        let engine = FundingEngine::new();
        assert!(engine.markets.read().unwrap().is_empty());
    }

    #[test]
    fn test_register_market() {
        let engine = FundingEngine::new();
        engine.register_market("ETH-USD".to_string(), FundingModel::Fixed, Some(1000000));
        
        assert!(engine.markets.read().unwrap().contains_key("ETH-USD"));
    }

    #[test]
    fn test_update_prices() {
        let engine = FundingEngine::new();
        engine.register_market("ETH-USD".to_string(), FundingModel::Premium, None);
        engine.update_prices("ETH-USD", 3000_000000, 3010_000000);
        
        let markets = engine.markets.read().unwrap();
        let funding = markets.get("ETH-USD").unwrap();
        assert_eq!(funding.index_price, 3000_000000);
    }

    #[test]
    fn test_calculate_fixed_rate() {
        let engine = FundingEngine::new();
        engine.register_market("ETH-USD".to_string(), FundingModel::Fixed, Some(1000000)); // 0.01%
        
        let rate = engine.calculate_rate("ETH-USD").unwrap();
        assert_eq!(rate.rate, 1000000);
    }

    #[test]
    fn test_calculate_premium_rate() {
        let engine = FundingEngine::new();
        engine.register_market("ETH-USD".to_string(), FundingModel::Premium, None);
        engine.update_prices("ETH-USD", 3000_000000, 3010_000000); // Mark > Index = positive rate
        
        let rate = engine.calculate_rate("ETH-USD").unwrap();
        assert!(rate.rate > 0);
    }

    #[test]
    fn test_calculate_payment() {
        let engine = FundingEngine::new();
        engine.register_market("ETH-USD".to_string(), FundingModel::Fixed, Some(1000000));
        engine.update_prices("ETH-USD", 3000_000000, 3000_000000);
        
        let payment = engine.calculate_payment("user-1", "ETH-USD", 1000, true).unwrap();
        assert!(payment.payment != 0);
    }

    #[test]
    fn test_funding_history() {
        let engine = FundingEngine::new();
        engine.register_market("ETH-USD".to_string(), FundingModel::Fixed, Some(1000000));
        
        engine.advance_period("ETH-USD").unwrap();
        
        let history = engine.get_history("ETH-USD", 10);
        assert_eq!(history.len(), 1);
    }

    #[test]
    fn test_invalid_rate() {
        let engine = FundingEngine::new();
        engine.register_market("ETH-USD".to_string(), FundingModel::Fixed, None);
        
        let result = engine.set_rate("ETH-USD", 2_00000000); // 200% - exceeds cap
        assert!(result.is_err());
    }
}