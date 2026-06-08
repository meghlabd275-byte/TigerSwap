//! TigerSwap Options Trading Engine
//! 
//! Implements options trading for DeFi:
//! - Call/Put options
//! - European/American style
//! - Settlement (cash or physical)
//! - Greeks calculation
//! - Pricing models
//!
//! Implementation: Pure Rust with no external dependencies

use serde::{Deserialize, Serialize};
use std::sync::Arc;
use parking_lot::RwLock;
use rust_decimal::Decimal;
use thiserror::Error;
use uuid::Uuid;
use chrono::Utc;
use std::collections::HashMap;

/// Chain ID constants
pub const CHAIN_ETH: u64 = 1;
pub const CHAIN_ARBITRUM: u64 = 42161;
pub const CHAIN_AVALANCHE: u64 = 43114;

#[derive(Debug, Error)]
pub enum OptionsError {
    #[error("Option not found: {0}")]
    OptionNotFound(String),
    #[error("Position not found: {0}")]
    PositionNotFound(String),
    #[error("Insufficient balance: {0}")]
    InsufficientBalance(String),
    #[error("Expired: {0}")]
    Expired(String),
    #[error("Strike exceeded: {0}")]
    StrikeExceeded(String),
    #[error("Invalid parameters: {0}")]
    InvalidParameters(String),
    #[error("Chain not supported: {0}")]
    ChainNotSupported(u64),
}

/// Option type
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum OptionType {
    Call,
    Put,
}

/// Option style
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum OptionStyle {
    European,   // Exercise only at expiry
    American,   // Exercise any time before expiry
}

impl Default for OptionStyle {
    fn default() -> Self { OptionStyle::European }
}

/// Option settlement
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum SettlementType {
    Cash,       // Cash settlement
    Physical,   // Physical delivery
}

impl Default for SettlementType {
    fn default() -> Self { SettlementType::Cash }
}

/// Option status
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum OptionStatus {
    Available,
    Exercised,
    Expired,
    Cancelled,
}

impl Default for OptionStatus {
    fn default() -> Self { OptionStatus::Available }
}

/// Option definition
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct OptionContract {
    pub option_id: String,
    pub writer: String,           // Option seller
    pub chain_id: u64,
    pub underlying: String,      // e.g., ETH
    pub strike_token: String,     // e.g., USDC
    pub option_type: OptionType,
    pub style: OptionStyle,
    pub strike_price: Decimal,
    pub expiry: i64,
    pub size: u128,           // Amount of underlying
    pub premium: u128,        // Premium paid
    pub settlement: SettlementType,
    pub status: OptionStatus,
    pub exercised_at: Option<i64>,
    pub created_at: i64,
}

impl OptionContract {
    /// Create a call option
    pub fn create_call(
        writer: String,
        chain_id: u64,
        underlying: String,
        strike_token: String,
        strike_price: Decimal,
        expiry: i64,
        size: u128,
        premium: u128,
    ) -> Self {
        Self {
            option_id: Uuid::new_v4().to_string(),
            writer,
            chain_id,
            underlying,
            strike_token,
            option_type: OptionType::Call,
            style: OptionStyle::European,
            strike_price,
            expiry,
            size,
            premium,
            settlement: SettlementType::Cash,
            status: OptionStatus::Available,
            exercised_at: None,
            created_at: Utc::now().timestamp(),
        }
    }

    /// Create a put option
    pub fn create_put(
        writer: String,
        chain_id: u64,
        underlying: String,
        strike_token: String,
        strike_price: Decimal,
        expiry: i64,
        size: u128,
        premium: u128,
    ) -> Self {
        let mut option = Self::create_call(
            writer,
            chain_id,
            underlying,
            strike_token,
            strike_price,
            expiry,
            size,
            premium,
        );
        option.option_type = OptionType::Put;
        option
    }

    /// Check if expired
    pub fn is_expired(&self) -> bool {
        Utc::now().timestamp() > self.expiry
    }

    /// Check if can exercise (for buyer)
    pub fn can_exercise(&self, current_price: Decimal) -> bool {
        if self.status != OptionStatus::Available {
            return false;
        }
        
        if self.is_expired() {
            return false;
        }
        
        match self.option_type {
            OptionType::Call => current_price > self.strike_price,
            OptionType::Put => current_price < self.strike_price,
        }
    }

    /// Exercise option
    pub fn exercise(&mut self, current_price: Decimal) -> Result<u128, OptionsError> {
        if self.status != OptionStatus::Available {
            return Err(OptionsError::OptionNotFound(self.option_id.clone()));
        }
        
        if self.is_expired() {
            self.status = OptionStatus::Expired;
            return Err(OptionsError::Expired("Option expired".to_string()));
        }
        
        // Calculate payout
        let payout = match self.option_type {
            OptionType::Call => {
                if current_price > self.strike_price {
                    (current_price - self.strike_price) * self.size as Decimal
                } else {
                    Decimal::ZERO
                }
            }
            OptionType::Put => {
                if current_price < self.strike_price {
                    (self.strike_price - current_price) * self.size as Decimal
                } else {
                    Decimal::ZERO
                }
            }
        };
        
        let payout_u128 = payout.as_u128();
        
        if payout_u128 == 0 {
            self.status = OptionStatus::Expired;
            return Err(OptionsError::StrikeExceeded("Out of the money".to_string()));
        }
        
        self.status = OptionStatus::Exercised;
        self.exercised_at = Some(Utc::now().timestamp());
        
        Ok(payout_u128)
    }

    /// Calculate intrinsic value
    pub fn intrinsic_value(&self, current_price: Decimal) -> u128 {
        match self.option_type {
            OptionType::Call => {
                if current_price > self.strike_price {
                    ((current_price - self.strike_price) * self.size as Decimal).as_u128()
                } else {
                    0
                }
            }
            OptionType::Put => {
                if current_price < self.strike_price {
                    ((self.strike_price - current_price) * self.size as Decimal).as_u128()
                } else {
                    0
                }
            }
        }
    }
}

/// Option position (for buyer)
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct OptionPosition {
    pub position_id: String,
    pub buyer: String,
    pub option_id: String,
    pub quantity: u128,
    pub premium_paid: u128,
    pub created_at: i64,
}

/// Option writer position
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct WriterPosition {
    pub position_id: String,
    pub writer: String,
    pub option_id: String,
    pub collateral: u128,
    pub obligations: u128,
    pub created_at: i64,
}

/// Greeks calculation
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Greeks {
    pub delta: f64,    // Change in option price per change in underlying
    pub gamma: f64,    // Change in delta per change in underlying
    pub theta: f64,    // Time decay per day
    pub vega: f64,    // Sensitivity to volatility
    pub rho: f64,    // Sensitivity to interest rate
}

impl Default for Greeks {
    fn default() -> Self {
        Self {
            delta: 0.0,
            gamma: 0.0,
            theta: 0.0,
            vega: 0.0,
            rho: 0.0,
        }
    }
}

impl Greeks {
    /// Calculate Black-Scholes Greeks (simplified)
    pub fn calculate_black_scholes(
        spot: f64,
        strike: f64,
        time_to_expiry: f64,
        volatility: f64,
        rate: f64,
        is_call: bool,
    ) -> Self {
        if time_to_expiry <= 0.0 || volatility <= 0.0 {
            return Self::default();
        }
        
        let sqrt_t = time_to_expiry.sqrt();
        let sqrt_vol = volatility * sqrt_t;
        
        let d1 = (spot / strike + (rate + volatility * volatility / 2.0) * time_to_expiry) / sqrt_vol;
        let d2 = d1 - sqrt_vol;
        
        // Standard normal CDF approximation
        let norm_cdf = |x: f64| -> f64 {
            let a1 = 0.254829592;
            let a2 = -0.284496736;
            let a3 = 1.421413741;
            let a4 = -1.453152027;
            let a5 = 1.061405429;
            let p = 0.3275911;
            
            let sign = if x < 0.0 { -1.0 } else { 1.0 };
            let x = x.abs() / (1.0 + p * x).abs();
            let y = 1.0 - (((((a5 * x + a4) * x + a3) * x + a2) * x + a1) * x * (-x).exp() / (1.0 + p * x).exp();
            
            (1.0 - sign) * y / 2.0 + sign * y / 2.0 + 0.5
        };
        
        let norm_pdf = |x: f64| -> f64 {
            (-x * x / 2.0).exp() / (2.0 * std::f64::consts::PI).sqrt()
        };
        
        let cdf_d1 = norm_cdf(d1);
        let cdf_d2 = norm_cdf(d2);
        let pdf_d1 = norm_pdf(d1);
        
        let e_rt = (-rate * time_to_expiry).exp();
        
        let delta = if is_call {
            cdf_d1
        } else {
            cdf_d1 - 1.0
        };
        
        let gamma = pdf_d1 / (spot * sqrt_vol);
        
        let theta = if is_call {
            -(spot * pdf_d1 * volatility / (2.0 * sqrt_t)) - rate * strike * e_rt * cdf_d2
        } else {
            -(spot * pdf_d1 * volatility / (2.0 * sqrt_t)) + rate * strike * e_rt * (1.0 - cdf_d2)
        };
        theta /= 365.0; // Per day
        
        let vega = spot * pdf_d1 * sqrt_t / 100.0; // Per 1% vol change
        
        let rho = if is_call {
            strike * time_to_expiry * e_rt * cdf_d2 / 100.0
        } else {
            -strike * time_to_expiry * e_rt * (1.0 - cdf_d2) / 100.0
        };
        
        Self {
            delta,
            gamma,
            theta,
            vega,
            rho,
        }
    }
}

/// Option order
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct OptionOrder {
    pub order_id: String,
    pub user: String,
    pub chain_id: u64,
    pub side: OrderSide,
    pub option_type: OptionType,
    pub underlying: String,
    pub strike_price: Decimal,
    pub expiry: i64,
    pub quantity: u128,
    pub premium_limit: u128,
    pub status: OrderStatus,
    pub created_at: i64,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum OrderSide {
    Buy,
    Sell,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum OrderStatus {
    Pending,
    Filled,
    Cancelled,
    Expired,
}

impl Default for OrderStatus {
    fn default() -> Self { OrderStatus::Pending }
}

impl OptionOrder {
    pub fn new_buy(
        user: String,
        chain_id: u64,
        option_type: OptionType,
        underlying: String,
        strike_price: Decimal,
        expiry: i64,
        quantity: u128,
        premium_limit: u128,
    ) -> Self {
        Self {
            order_id: Uuid::new_v4().to_string(),
            user,
            chain_id,
            side: OrderSide::Buy,
            option_type,
            underlying,
            strike_price,
            expiry,
            quantity,
            premium_limit,
            status: OrderStatus::Pending,
            created_at: Utc::now().timestamp(),
        }
    }
}

/// Options Engine
pub struct OptionsEngine {
    options: Arc<RwLock<HashMap<String, OptionContract>>>,
    buyer_positions: Arc<RwLock<HashMap<String, Vec<OptionPosition>>>>,
    writer_positions: Arc<RwLock<HashMap<String, Vec<WriterPosition>>>>,
    orders: Arc<RwLock<HashMap<String, OptionOrder>>>,
    supported_chains: Arc<RwLock<std::collections::HashSet<u64>>>,
}

impl OptionsEngine {
    pub fn new() -> Self {
        let chains: std::collections::HashSet<u64> = [
            CHAIN_ETH, CHAIN_ARBITRUM, CHAIN_AVALANCHE,
        ].into_iter().collect();
        
        Self {
            options: Arc::new(RwLock::new(HashMap::new())),
            buyer_positions: Arc::new(RwLock::new(HashMap::new())),
            writer_positions: Arc::new(RwLock::new(HashMap::new())),
            orders: Arc::new(RwLock::new(HashMap::new())),
            supported_chains: Arc::new(RwLock::new(chains)),
        }
    }

    pub fn is_chain_supported(&self, chain_id: u64) -> bool {
        self.supported_chains.read().contains(&chain_id)
    }

    /// Write (sell) an option
    pub fn write_option(
        &self,
        writer: String,
        chain_id: u64,
        option_type: OptionType,
        underlying: String,
        strike_token: String,
        strike_price: Decimal,
        expiry: i64,
        size: u128,
        premium: u128,
    ) -> Result<String, OptionsError> {
        if !self.is_chain_supported(chain_id) {
            return Err(OptionsError::ChainNotSupported(chain_id));
        }
        
        let option = match option_type {
            OptionType::Call => OptionContract::create_call(
                writer, chain_id, underlying, strike_token, strike_price, expiry, size, premium,
            ),
            OptionType::Put => OptionContract::create_put(
                writer, chain_id, underlying, strike_token, strike_price, expiry, size, premium,
            ),
        };
        
        let option_id = option.option_id.clone();
        
        // Create writer position
        let writer_pos = WriterPosition {
            position_id: Uuid::new_v4().to_string(),
            writer: writer.clone(),
            option_id: option_id.clone(),
            collateral: size * strike_price.as_u128(), // Lock collateral
            obligations: size,
            created_at: Utc::now().timestamp(),
        };
        
        self.writer_positions.write()
            .entry(writer.clone())
            .or_insert_with(Vec::new)
            .push(writer_pos);
        
        self.options.write().insert(option_id.clone(), option);
        
        Ok(option_id)
    }

    /// Buy an option
    pub fn buy_option(
        &self,
        buyer: String,
        option_id: &str,
        quantity: u128,
    ) -> Result<String, OptionsError> {
        let mut options = self.options.write();
        let option = options.get_mut(option_id)
            .ok_or_else(|| OptionsError::OptionNotFound(option_id.to_string()))?;
        
        if option.status != OptionStatus::Available {
            return Err(OptionsError::OptionNotFound("Option not available".to_string()));
        }
        
        if option.is_expired() {
            return Err(OptionsError::Expired("Option expired".to_string()));
        }
        
        let total_premium = option.premium * quantity;
        
        // Create buyer position
        let position = OptionPosition {
            position_id: Uuid::new_v4().to_string(),
            buyer: buyer.clone(),
            option_id: option_id.to_string(),
            quantity,
            premium_paid: total_premium,
            created_at: Utc::now().timestamp(),
        };
        
        let position_id = position.position_id.clone();
        
        self.buyer_positions.write()
            .entry(buyer.clone())
            .or_insert_with(Vec::new)
            .push(position);
        
        Ok(position_id)
    }

    /// Exercise option
    pub fn exercise(
        &self,
        buyer: &str,
        position_id: &str,
        current_price: Decimal,
    ) -> Result<u128, OptionsError> {
        let positions = self.buyer_positions.read();
        
        let buyer_positions = positions.get(buyer)
            .ok_or_else(|| OptionsError::PositionNotFound(position_id.to_string()))?;
        
        let position = buyer_positions.iter()
            .find(|p| p.position_id == position_id)
            .ok_or_else(|| OptionsError::PositionNotFound(position_id.to_string()))?;
        
        let option_id = position.option_id.clone();
        drop(positions);
        
        let mut options = self.options.write();
        let option = options.get_mut(&option_id)
            .ok_or_else(|| OptionsError::OptionNotFound(option_id.clone()))?;
        
        option.exercise(current_price)
    }

    /// Get option
    pub fn get_option(&self, option_id: &str) -> Option<OptionContract> {
        self.options.read().get(option_id).cloned()
    }

    /// Get buyer positions
    pub fn get_buyer_positions(&self, buyer: &str) -> Vec<OptionPosition> {
        self.buyer_positions.read()
            .get(buyer)
            .cloned()
            .unwrap_or_default()
    }

    /// Get writer positions
    pub fn get_writer_positions(&self, writer: &str) -> Vec<WriterPosition> {
        self.writer_positions.read()
            .get(writer)
            .cloned()
            .unwrap_or_default()
    }

    /// Calculate Greeks
    pub fn calculate_greeks(
        &self,
        spot_price: Decimal,
        strike_price: Decimal,
        time_to_expiry_days: i64,
        volatility: f64,
        risk_free_rate: f64,
        is_call: bool,
    ) -> Greeks {
        let time_years = time_to_expiry_days as f64 / 365.0;
        
        Greeks::calculate_black_scholes(
            spot_price.as_f64(),
            strike_price.as_f64(),
            time_years,
            volatility,
            risk_free_rate,
            is_call,
        )
    }

    /// Get available options
    pub fn get_available_options(&self, underlying: &str) -> Vec<OptionContract> {
        self.options.read()
            .values()
            .filter(|o| o.underlying == underlying && o.status == OptionStatus::Available)
            .cloned()
            .collect()
    }

    /// Add supported chain
    pub fn add_chain(&self, chain_id: u64) {
        self.supported_chains.write().insert(chain_id);
    }

    pub fn supported_chains(&self) -> Vec<u64> {
        self.supported_chains.read().iter().cloned().collect()
    }
}

impl Default for OptionsEngine {
    fn default() -> Self { Self::new() }
}

#[cfg(test)]
mod tests {
    use super::*;
    use rust_decimal_macros::dec;

    #[test]
    fn test_call_option() {
        let option = OptionContract::create_call(
            "writer1".to_string(),
            CHAIN_ETH,
            "ETH".to_string(),
            "USDC".to_string(),
            dec!(2000.0),
            Utc::now().timestamp() + 30 days,
            1,
            100,
        );
        
        assert_eq!(option.option_type, OptionType::Call);
    }

    #[test]
    fn test_exercise_call() {
        let mut option = OptionContract::create_call(
            "writer1".to_string(),
            CHAIN_ETH,
            "ETH".to_string(),
            "USDC".to_string(),
            dec!(2000.0),
            Utc::now().timestamp() + 30 days,
            1,
            100,
        );
        
        let payout = option.exercise(dec!(2500.0)).unwrap();
        
        assert!(payout > 0);
    }

    #[test]
    fn test_intrinsic_value() {
        let option = OptionContract::create_call(
            "writer1".to_string(),
            CHAIN_ETH,
            "ETH".to_string(),
            "USDC".to_string(),
            dec!(2000.0),
            Utc::now().timestamp() + 30 days,
            1,
            100,
        );
        
        let intrinsic = option.intrinsic_value(dec!(2500.0));
        
        assert!(intrinsic > 0);
    }

    #[test]
    fn test_greeks_calculation() {
        let greeks = Greeks::calculate_black_scholes(
            2000.0,
            2000.0,
            30.0 / 365.0,
            0.5,
            0.05,
            true,
        );
        
        assert!(greeks.delta > 0.0);
    }
}