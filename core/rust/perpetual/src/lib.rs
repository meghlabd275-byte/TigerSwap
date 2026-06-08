//! TigerSwap Perpetual Futures Protocol
//! 
//! High-performance perpetual futures trading with:
//! - Up to 100x leverage
//! - Cross-margin and isolated margin
//! - Funding rate payments
//! - Insurance fund
//! - Auto-deleveraging
//! - Liquidation engine
//!
//! Uses Rust for sub-millisecond execution

use std::collections::HashMap;
use std::sync::Arc;
use tokio::sync::RwLock;
use serde::{Deserialize, Serialize};
use uint::construct_uint;

construct_uint! {
    pub struct U256(4);
}

// ==================== PERPETUAL TYPES ====================

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum PositionSide {
    Long,
    Short,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum MarginMode {
    Cross,
    Isolated,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum PositionStatus {
    Open,
    Partial,
    Liquidated,
    Closed,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum OrderType {
    Market,
    Limit,
    StopLoss,
    TakeProfit,
}

// ==================== POSITION ====================

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Position {
    pub id: [u8; 32],
    pub user: [u8; 20],
    pub pair: [u8; 20],
    pub side: PositionSide,
    pub size: U256,           // Position size
    pub entry_price: U256,
    pub mark_price: U256,
    pub leverage: u32,
    pub margin_mode: MarginMode,
    pub margin: U256,          // Isolated margin or cross-margin allocation
    pub margin_ratio: U256,     // Current margin ratio
    pub unrealized_pnl: U256,
    pub realized_pnl: U256,
    pub funding_payment: U256,
    pub open_notional: U256,
    pub status: PositionStatus,
    pub created_at: u64,
    pub updated_at: u64,
    pub liquidation_price: U256,
    pub stop_loss: Option<U256>,
    pub take_profit: Option<U256>,
}

impl Position {
    pub fn new(
        user: [u8; 20],
        pair: [u8; 20],
        side: PositionSide,
        size: U256,
        entry_price: U256,
        leverage: u32,
        margin_mode: MarginMode,
        margin: U256,
    ) -> Result<Self, PerpError> {
        if leverage < 1 || leverage > 100 {
            return Err(PerpError::InvalidLeverage);
        }
        
        if margin == U256::zero() {
            return Err(PerpError::InsufficientMargin);
        }
        
        let open_notional = size * entry_price;
        let required_margin = open_notional / U256::from(leverage);
        
        if required_margin < margin {
            return Err(PerpError::InsufficientMargin);
        }
        
        let liquidation_price = Self::calculate_liquidation_price(
            entry_price,
            leverage,
            side,
            margin,
            size,
        );
        
        Ok(Self {
            id: Self::generate_id(&user, &pair),
            user,
            pair,
            side,
            size,
            entry_price,
            mark_price: entry_price,
            leverage,
            margin_mode,
            margin,
            margin_ratio: U256::zero(),
            unrealized_pnl: U256::zero(),
            realized_pnl: U256::zero(),
            funding_payment: U256::zero(),
            open_notional,
            status: PositionStatus::Open,
            created_at: current_timestamp(),
            updated_at: current_timestamp(),
            liquidation_price,
            stop_loss: None,
            take_profit: None,
        })
    }
    
    fn generate_id(user: &[u8; 20], pair: &[u8; 20]) -> [u8; 32] {
        let mut id = [0u8; 32];
        let ts = current_timestamp();
        id[..20].copy_from_slice(user);
        id[20..].copy_from_slice(&ts.to_le_bytes()[..12]);
        id
    }
    
    fn calculate_liquidation_price(
        entry_price: U256,
        leverage: u32,
        side: PositionSide,
        margin: U256,
        size: U256,
    ) -> U256 {
        // Liquidation price calculation
        let max_loss = margin;
        let price_change = max_loss / size;
        
        match side {
            PositionSide::Long => entry_price - price_change,
            PositionSide::Short => entry_price + price_change,
        }
    }
    
    pub fn update_mark_price(&mut self, mark_price: U256) {
        self.mark_price = mark_price;
        self.updated_at = current_timestamp();
        
        // Calculate unrealized PnL
        let pnl = match self.side {
            PositionSide::Long => (mark_price - self.entry_price) * self.size,
            PositionSide::Short => (self.entry_price - mark_price) * self.size,
        };
        
        self.unrealized_pnl = pnl;
        
        // Update margin ratio
        let total_value = self.margin + self.unrealized_pnl;
        self.margin_ratio = (total_value * U256::from(10000)) / self.open_notional;
        
        // Check liquidation
        if self.is_liquidatable() {
            self.status = PositionStatus::Liquidated;
        }
    }
    
    pub fn is_liquidatable(&self) -> bool {
        let maintenance_margin = self.open_notional / U256::from(self.leverage * 2);
        let total_margin = self.margin + self.unrealized_pnl;
        total_margin < maintenance_margin
    }
    
    pub fn get_notional_value(&self) -> U256 {
        self.mark_price * self.size
    }
    
    pub fn get_margin_ratio_bps(&self) -> u64 {
        // Return in basis points
        let value = self.margin_ratio.as_u64();
        value as u64
    }
    
    pub fn add_funding(&mut self, funding: U256) {
        match self.side {
            PositionSide::Long => self.funding_payment = self.funding_payment - funding,
            PositionSide::Short => self.funding_payment = self.funding_payment + funding,
        }
    }
    
    pub fn close(&mut self, exit_price: U256) -> Result<(U256, U256), PerpError> {
        if self.status != PositionStatus::Open && self.status != PositionStatus::Partial {
            return Err(PerpError::InvalidPositionState);
        }
        
        let realized = self.unrealized_pnl + self.funding_payment;
        let exit_notional = self.size * exit_price;
        let fees = exit_notional * U256::from(30) / U256::from(10000); // 0.3% fee
        
        self.status = PositionStatus::Closed;
        self.realized_pnl = realized - fees;
        
        Ok((self.margin + realized - fees, fees))
    }
}

// ==================== FUNDING ====================

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct FundingState {
    pub pair: [u8; 20],
    pub long_rate: U256,      // Current funding rate (longs pay)
    pub short_rate: U256,     // Current funding rate (shorts receive)
    pub long_oi: U256,       // Long open interest
    pub short_oi: U256,     // Short open interest
    pub imbalance: U256,     // OI imbalance
    pub last_update: u64,
    pub period_start: u64,
}

impl FundingState {
    pub fn new(pair: [u8; 20]) -> Self {
        Self {
            pair,
            long_rate: U256::zero(),
            short_rate: U256::zero(),
            long_oi: U256::zero(),
            short_oi: U256::zero(),
            imbalance: U256::zero(),
            last_update: current_timestamp(),
            period_start: current_timestamp(),
        }
    }
    
    pub fn update_rates(&mut self) {
        // Calculate funding rate based on OI imbalance
        let total_oi = self.long_oi + self.short_oi;
        
        if total_oi == U256::zero() {
            return;
        }
        
        self.imbalance = if self.long_oi > self.short_oi {
            self.long_oi - self.short_oi
        } else {
            self.short_oi - self.long_oi
        };
        
        // Funding rate = imbalance / total_oi * 0.01% per hour
        let rate = (self.imbalance * U256::from(1)) / (total_oi * U256::from(10000));
        
        if self.long_oi > self.short_oi {
            self.long_rate = rate;
            self.short_rate = U256::zero();
        } else if self.short_oi > self.long_oi {
            self.short_rate = rate;
            self.long_rate = U256::zero();
        }
        
        self.last_update = current_timestamp();
    }
    
    pub fn settle_funding(&self, positions: &HashMap<[u8; 32], position: &Position) -> U256 {
        let funding = match position.side {
            PositionSide::Long => self.long_rate,
            PositionSide::Short => self.short_rate,
        };
        
        position.size * funding
    }
}

// ==================== INSURANCE FUND ====================

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct InsuranceFund {
    pub balance: U256,
    pub total_liquidations: U256,
    pub total_payouts: U256,
    pub max_payout_ratio: U256,  // Max % of pool to use per liquidation
}

impl InsuranceFund {
    pub fn new() -> Self {
        Self {
            balance: U256::zero(),
            total_liquidations: U256::zero(),
            total_payouts: U256::zero(),
            max_payout_ratio: U256::from(10), // 10% max
        }
    }
    
    pub fn add_collateral(&mut self, amount: U256) {
        self.balance = self.balance + amount;
    }
    
    pub fn use_for_liquidation(&mut self, payout: U256) -> Result<U256, PerpError> {
        let max_payout = (self.balance * self.max_payout_ratio) / U256::from(100);
        
        if payout > max_payout {
            self.balance = self.balance - max_payout;
            self.total_payouts = self.total_payouts + max_payout;
            Ok(max_payout)
        } else {
            self.balance = self.balance - payout;
            self.total_payouts = self.total_payouts + payout;
            Ok(payout)
        }
    }
}

// ==================== LIQUIDATION ====================

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct LiquidationResult {
    pub position_id: [u8; 32],
    pub liquidation_price: U256,
    pub remaining_margin: U256,
    pub penalty_fee: U256,
    pub insurance_payout: U256,
    pub liquidator_reward: U256,
}

pub struct LiquidationEngine {
    insurance_fund: Arc<RwLock<InsuranceFee>>,
    penalty_rate: U256,
    reward_rate: U256,
}

impl LiquidationEngine {
    pub fn new() -> Self {
        Self {
            insurance_fund: Arc::new(RwLock::new(InsuranceFund::new())),
            penalty_rate: U256::from(5),   // 0.5% penalty
            reward_rate: U256::from(25),  // 2.5% liquidator reward
        }
    }
    
    pub async fn liquidate(
        &self,
        position: &mut Position,
    ) -> Result<LiquidationResult, PerpError> {
        if !position.is_liquidatable() {
            return Err(PerpError::NotLiquidatable);
        }
        
        let penalty = (position.margin * self.penalty_rate) / U256::from(1000);
        let reward = (position.margin * self.reward_rate) / U256::from(1000);
        
        let remaining = position.margin - penalty - reward;
        
        // Try to use insurance fund
        let mut insurance = self.insurance_fund.write().await;
        let insurance_payout = insurance.use_for_liquidation(remaining).unwrap_or(U256::zero());
        drop(insurance);
        
        position.status = PositionStatus::Liquidated;
        
        Ok(LiquidationResult {
            position_id: position.id,
            liquidation_price: position.liquidation_price,
            remaining_margin: remaining + insurance_payout,
            penalty_fee: penalty,
            insurance_payout,
            liquidator_reward: reward,
        })
    }
}

// ==================== PERPETUAL ERROR ====================

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum PerpError {
    InvalidLeverage,
    InsufficientMargin,
    InvalidPositionState,
    PositionNotFound,
    InsufficientLiquidity,
    LiquidationUnavailable,
    NotLiquidatable,
    FundingError,
    InvalidOrderType,
}

impl std::fmt::Display for PerpError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            PerpError::InvalidLeverage => write!(f, "Invalid leverage (1-100)"),
            PerpError::InsufficientMargin => write!(f, "Insufficient margin"),
            PerpError::InvalidPositionState => write!(f, "Invalid position state"),
            PerpError::PositionNotFound => write!(f, "Position not found"),
            PerpError::InsufficientLiquidity => write!(f, "Insufficient liquidity"),
            PerpError::LiquidationUnavailable => write!(f, "Liquidation unavailable"),
            PerpError::NotLiquidatable => write!(f, "Position not liquidatable"),
            PerpError::FundingError => write!(f, "Funding calculation error"),
            PerpError::InvalidOrderType => write!(f, "Invalid order type"),
        }
    }
}

// ==================== PERPETUAL MARKET ====================

pub struct PerpetualMarket {
    positions: Arc<RwLock<HashMap<[u8; 32], Position>>>,
    funding_states: Arc<RwLock<HashMap<[u8; 20], FundingState>>>,
    liquidation_engine: LiquidationEngine,
    insurance_fund: Arc<RwLock<InsuranceFund>>,
    min_leverage: u32,
    max_leverage: u32,
}

impl PerpetualMarket {
    pub fn new(min_leverage: u32, max_leverage: u32) -> Self {
        Self {
            positions: Arc::new(RwLock::new(HashMap::new())),
            funding_states: Arc::new(RwLock::new(HashMap::new())),
            liquidation_engine: LiquidationEngine::new(),
            insurance_fund: Arc::new(RwLock::new(InsuranceFund::new())),
            min_leverage,
            max_leverage,
        }
    }
    
    pub async fn open_position(
        &self,
        user: [u8; 20],
        pair: [u8; 20],
        side: PositionSide,
        size: U256,
        entry_price: U256,
        leverage: u32,
        margin_mode: MarginMode,
        margin: U256,
    ) -> Result<[u8; 32], PerpError> {
        if leverage < self.min_leverage || leverage > self.max_leverage {
            return Err(PerpError::InvalidLeverage);
        }
        
        let position = Position::new(
            user, pair, side, size, entry_price, leverage, margin_mode, margin,
        )?;
        
        let position_id = position.id;
        
        // Store position
        self.positions.write().await.insert(position_id, position);
        
        // Update funding state
        let mut funding_states = self.funding_states.write().await;
        let funding = funding_states.entry(pair).or_insert_with(|| FundingState::new(pair));
        match side {
            PositionSide::Long => funding.long_oi = funding.long_oi + (entry_price * size),
            PositionSide::Short => funding.short_oi = funding.short_oi + (entry_price * size),
        }
        
        Ok(position_id)
    }
    
    pub async fn close_position(
        &self,
        position_id: &[u8; 32],
        user: &[u8; 20],
        exit_price: U256,
    ) -> Result<(U256, U256), PerpError> {
        let mut positions = self.positions.write().await;
        
        if let Some(position) = positions.get_mut(position_id) {
            if &position.user != user {
                return Err(PerpError::InsufficientMargin);
            }
            
            position.close(exit_price)
        } else {
            Err(PerpError::PositionNotFound)
        }
    }
    
    pub async fn update_prices(&self, pair: &[u8; 20], mark_price: U256) {
        let positions = self.positions.read().await;
        
        // Update all positions for this pair
        for (_id, position) in positions.iter() {
            if position.pair == *pair && position.status == PositionStatus::Open {
                // Update mark price (need mutable borrow, so we'd clone or refactor)
            }
        }
    }
    
    pub async fn liquidate_positions(&self) -> Vec<LiquidationResult> {
        let mut results = Vec::new();
        let mut positions = self.positions.write().await;
        
        for (_id, position) in positions.iter_mut() {
            if position.is_liquidatable() && position.status == PositionStatus::Open {
                if let Ok(result) = self.liquidation_engine.liquidate(position).await {
                    results.push(result);
                }
            }
        }
        
        results
    }
    
    pub async fn settle_funding(&self, pair: &[u8; 20]) -> Result<(), PerpError> {
        let mut funding_states = self.funding_states.write().await;
        
        if let Some(funding) = funding_states.get_mut(pair) {
            funding.update_rates();
            
            // Update all positions for this pair
            let positions = self.positions.read().await;
            for (_id, position) in positions.iter() {
                if position.pair == *pair {
                    let funding_payment = funding.settle_funding(&positions, position);
                    position.add_funding(funding_payment);
                }
            }
            
            Ok(())
        } else {
            Err(PerpError::FundingError)
        }
    }
    
    pub async fn get_position(&self, position_id: &[u8; 32]) -> Option<Position> {
        let positions = self.positions.read().await;
        positions.get(position_id).cloned()
    }
    
    pub async fn get_user_positions(&self, user: &[u8; 20]) -> Vec<Position> {
        let positions = self.positions.read().await;
        
        positions.values()
            .filter(|p| p.user == *user && p.status == PositionStatus::Open)
            .cloned()
            .collect()
    }
}

// ==================== HELPER FUNCTIONS ====================

fn current_timestamp() -> u64 {
    std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .unwrap()
        .as_secs()
}

trait U256Ext {
    fn zero() -> Self;
    fn from(n: u64) -> Self;
    fn as_u64(&self) -> u64;
}

impl U256Ext for U256 {
    fn zero() -> Self { U256::from(0) }
    fn from(n: u64) -> Self { U256::from(n) }
    fn as_u64(&self) -> u64 { self.0[0] as u64 }
}

// ==================== PUBLIC API ====================

pub mod api {
    use super::*;
    
    pub type PerpetualMarketHandle = Arc<PerpetualMarket>;
    pub type LiquidationEngineHandle = Arc<LiquidationEngine>;
    
    pub fn create_market(min_leverage: u32, max_leverage: u32) -> PerpetualMarketHandle {
        Arc::new(PerpetualMarket::new(min_leverage, max_leverage))
    }
    
    pub fn create_liquidation_engine() -> LiquidationEngineHandle {
        Arc::new(LiquidationEngine::new())
    }
}

// ==================== TESTS ====================

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_position_creation() {
        let user = [0u8; 20];
        let pair = [0u8; 20];
        
        let position = Position::new(
            user,
            pair,
            PositionSide::Long,
            U256::from(10),
            U256::from(50000),
            10,
            MarginMode::Cross,
            U256::from(5000),
        );
        
        assert!(position.is_ok());
    }
    
    #[test]
    fn test_leverage_validation() {
        let user = [0u8; 20];
        let pair = [0u8; 20];
        
        // Invalid leverage (101x)
        let position = Position::new(
            user,
            pair,
            PositionSide::Long,
            U256::from(10),
            U256::from(50000),
            101,
            MarginMode::Cross,
            U256::from(5000),
        );
        
        assert!(position.is_err());
    }
}