//! TigerSwap Lending Protocol
//! 
//! Decentralized lending/borrowing with:
//! - Supply markets
//! - Borrow markets  
//! - Collateral management
//! - Liquidation
//! - Interest rate model
//!
//! Uses Rust for high performance

use std::collections::HashMap;
use std::sync::Arc;
use tokio::sync::RwLock;
use serde::{Deserialize, Serialize};
use uint::construct_uint;

construct_uint! {
    pub struct U256(4);
}

// ==================== LENDING TYPES ====================

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum MarketState {
    Active,
    Paused,
    Deprecated,
}

// ==================== MARKET ====================

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Market {
    pub token: [u8; 20],
    pub total_supply: U256,
    pub total_borrow: U256,
    pub supply_rate: U256,
    pub borrow_rate: U256,
    pub utilization: U256,
    pub collateral_factor: U256,  // How much can be borrowed against this
    pub liquidation_threshold: U256,
    pub reserve_factor: U256,
    pub state: MarketState,
}

impl Market {
    pub fn new(token: [u8; 20]) -> Self {
        Self {
            token,
            total_supply: U256::zero(),
            total_borrow: U256::zero(),
            supply_rate: U256::zero(),
            borrow_rate: U256::zero(),
            utilization: U256::zero(),
            collateral_factor: U256::from(75), // 75% collateral factor
            liquidation_threshold: U256::from(80), // 80% liquidation threshold
            reserve_factor: U256::from(10), // 10% to reserves
            state: MarketState::Active,
        }
    }
    
    pub fn update_rates(&mut self) {
        if self.total_supply == U256::zero() {
            self.supply_rate = U256::zero();
            self.borrow_rate = U256::zero();
            return;
        }
        
        // Calculate utilization
        self.utilization = (self.total_borrow * U256::from(10000)) / self.total_supply;
        let util_bps = self.utilization.as_u64() as u64;
        
        // Base rates
        let base_borrow_rate: u64 = 500; // 5% base
        let borrow_rate = base_borrow_rate + (util_bps * 3 / 2); // Slope
        self.borrow_rate = U256::from(borrow_rate);
        
        // Supply rate = borrow rate * utilization * (1 - reserve factor)
        let supply_rate = (self.borrow_rate * self.utilization * (U256::from(1000) - self.reserve_factor)) 
            / (U256::from(1000) * U256::from(10000));
        self.supply_rate = supply_rate;
    }
    
    pub fn accrue_interest(&mut self, seconds: u64) {
        // Accrue supply interest
        let supply_interest = self.total_supply * self.supply_rate * U256::from(seconds) 
            / (U256::from(365 * 24 * 3600) * U256::from(10000));
        self.total_supply = self.total_supply + supply_interest;
        
        // Accrue borrow interest
        let borrow_interest = self.total_borrow * self.borrow_rate * U256::from(seconds)
            / (U256::from(365 * 24 * 3600) * U256::from(10000));
        self.total_borrow = self.total_borrow + borrow_interest;
    }
}

// ==================== USER ACCOUNT ====================

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct UserAccount {
    pub user: [u8; 20],
    pub supplies: HashMap<[u8; 20],  // token -> amount
    pub borrows: HashMap<[u8; 20],  // token -> amount
    pub collateral_value: U256,
    pub borrow_value: U256,
    pub health_factor: U256,
}

impl UserAccount {
    pub fn new(user: [u8; 20]) -> Self {
        Self {
            user,
            supplies: HashMap::new(),
            borrows: HashMap::new(),
            collateral_value: U256::zero(),
            borrow_value: U256::zero(),
            health_factor: U256::from(10000), // 1.0 = healthy
        }
    }
    
    pub fn add_supply(&mut self, token: [u8; 20], amount: U256, price: U256, market: &Market) {
        let current = *self.supplies.get(&token).unwrap_or(&U256::zero());
        self.supplies.insert(token, current + amount);
        
        // Update collateral value
        self.collateral_value = self.collateral_value + (amount * price * market.collateral_factor / U256::from(100));
        self.recalculate_health();
    }
    
    pub fn remove_supply(&mut self, token: [u8; 20], amount: U256, price: U256, market: &Market) -> Result<(), LendingError> {
        let current = *self.supplies.get(&token).unwrap_or(&U256::zero());
        
        if current < amount {
            return Err(LendingError::InsufficientSupply);
        }
        
        self.supplies.insert(token, current - amount);
        
        // Update collateral value
        self.collateral_value = self.collateral_value - (amount * price * market.collateral_factor / U256::from(100));
        self.recalculate_health();
        
        Ok(())
    }
    
    pub fn borrow(&mut self, token: [u8; 20], amount: U256, price: U256) -> Result<(), LendingError> {
        // Check health factor before borrowing
        let new_borrow_value = self.borrow_value + (amount * price);
        
        if new_borrow_value > self.collateral_value {
            return Err(LendingError::InsufficientCollateral);
        }
        
        let current = *self.borrows.get(&token).unwrap_or(&U256::zero());
        self.borrows.insert(token, current + amount);
        self.borrow_value = new_borrow_value;
        self.recalculate_health();
        
        Ok(())
    }
    
    pub fn repay(&mut self, token: [u8; 20], amount: U256) -> Result<(), LendingError> {
        let current = *self.borrows.get(&token).unwrap_or(&U256::zero());
        
        if current < amount {
            return Err(LendingError::InsufficientBorrow);
        }
        
        self.borrows.insert(token, current - amount);
        self.recalculate_health();
        
        Ok(())
    }
    
    fn recalculate_health(&mut self) {
        if self.collateral_value == U256::zero() {
            self.health_factor = U256::from(10000);
            return;
        }
        
        // Health factor = collateral / borrows * 10000
        self.health_factor = (self.collateral_value * U256::from(10000)) / self.borrow_value;
    }
    
    pub fn is_healthy(&self) -> bool {
        self.health_factor >= U256::from(10000) // 1.0 health factor
    }
    
    pub fn get_max_borrow(&self, price: U256) -> U256 {
        // Max borrow = (collateral * 10000 / health_factor) - current borrows
        if !self.is_healthy() {
            return U256::zero();
        }
        
        let max_collateral = (self.collateral_value * U256::from(10000)) / self.health_factor;
        let max_borrow = max_collateral - self.borrow_value;
        
        if max_borrow > U256::zero() {
            max_borrow / price
        } else {
            U256::zero()
        }
    }
}

// ==================== RESERVES ====================

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ProtocolReserves {
    pub cash: HashMap<[u8; 20],  // token -> amount
    pub fees_accrued: HashMap<[u8; 20],  // token -> amount
}

impl ProtocolReserves {
    pub fn new() -> Self {
        Self {
            cash: HashMap::new(),
            fees_accrued: HashMap::new(),
        }
    }
    
    pub fn add_cash(&mut self, token: [u8; 20], amount: U256) {
        let current = *self.cash.get(&token).unwrap_or(&U256::zero());
        self.cash.insert(token, current + amount);
    }
    
    pub fn add_fee(&mut self, token: [u8; 20], amount: U256) {
        let current = *self.fees_accrued.get(&token).unwrap_or(&U256::zero());
        self.fees_accrued.insert(token, current + amount);
    }
}

// ==================== LIQUIDATION ====================

pub struct LiquidationEngine {
    close_factor: U256,  // How much of position can be liquidated
}

impl LiquidationEngine {
    pub fn new() -> Self {
        Self {
            close_factor: U256::from(50), // 50% max
        }
    }
    
    pub fn liquidate(
        &self,
        borrower: &mut UserAccount,
        markets: &HashMap<[u8; 20], Market>,
        liquidator: &mut UserAccount,
    ) -> Result<HashMap<[u8; 20], LendingError> {
        if borrower.is_healthy() {
            return Err(LendingError::AccountHealthy);
        }
        
        let mut seized = HashMap::new();
        
        // Liquidate borrows
        for (token, borrow_amount) in &borrower.borrows {
            if let Some(market) = markets.get(token) {
                let liquidate_amount = *borrow_amount * self.close_factor / U256::from(100);
                let value = liquidate_amount * U256::from(1); // Using price = 1 for simplicity
                
                // Seize collateral
                let seizure_bonus = value * U256::from(105) / U256::from(100); // 5% bonus
                
                let current_supply = *borrower.supplies.get(token).unwrap_or(&U256::zero());
                if current_supply >= seizure_bonus {
                    borrower.supplies.insert(*token, current_supply - seizure_bonus);
                    seized.insert(*token, seizure_bonus);
                    
                    // Give to liquidator
                    let current = *liquidator.supplies.get(token).unwrap_or(&U256::zero());
                    liquidator.supplies.insert(*token, current + seizure_bonus);
                }
            }
        }
        
        borrower.recalculate_health();
        
        Ok(seized)
    }
}

// ==================== LENDING ERROR ====================

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum LendingError {
    InsufficientSupply,
    InsufficientBorrow,
    InsufficientCollateral,
    MarketNotFound,
    AccountHealthy,
    InvalidAmount,
    Paused,
}

impl std::fmt::Display for LendingError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            LendingError::InsufficientSupply => write!(f, "Insufficient supply"),
            LendingError::InsufficientBorrow => write!(f, "Insufficient borrow balance"),
            LendingError::InsufficientCollateral => write!(f, "Insufficient collateral"),
            LendingError::MarketNotFound => write!(f, "Market not found"),
            LendingError::AccountHealthy => write!(f, "Account is healthy, cannot liquidate"),
            LendingError::InvalidAmount => write!(f, "Invalid amount"),
            LendingError::Paused => write!(f, "Market is paused"),
        }
    }
}

// ==================== LENDING PROTOCOL ====================

pub struct LendingProtocol {
    markets: Arc<RwLock<HashMap<[u8; 20], Market>>>,
    accounts: Arc<RwLock<HashMap<[u8; 20], UserAccount>>>,
    reserves: Arc<RwLock<ProtocolReserves>>,
    liquidation_engine: LiquidationEngine,
}

impl LendingProtocol {
    pub fn new() -> Self {
        Self {
            markets: Arc::new(RwLock::new(HashMap::new())),
            accounts: Arc::new(RwLock::new(HashMap::new())),
            reserves: Arc::new(RwLock::new(ProtocolReserves::new())),
            liquidation_engine: LiquidationEngine::new(),
        }
    }
    
    pub async fn create_market(&self, token: [u8; 20]) {
        let market = Market::new(token);
        self.markets.write().await.insert(token, market);
    }
    
    pub async fn supply(
        &self,
        user: [u8; 20],
        token: [u8; 20],
        amount: U256,
    ) -> Result<(), LendingError> {
        if amount == U256::zero() {
            return Err(LendingError::InvalidAmount);
        }
        
        let markets = self.markets.read().await;
        let market = markets.get(&token).ok_or(LendingError::MarketNotFound)?;
        
        if market.state != MarketState::Active {
            return Err(LendingError::Paused);
        }
        
        drop(markets);
        
        // Update market
        let mut markets = self.markets.write().await;
        if let Some(market) = markets.get_mut(&token) {
            market.total_supply = market.total_supply + amount;
            market.update_rates();
        }
        
        // Update account
        let mut accounts = self.accounts.write().await;
        let account = accounts.entry(user).or_insert_with(|| UserAccount::new(user));
        account.add_supply(token, amount, U256::from(1), &Market::new(token));
        
        // Add to reserves
        let mut reserves = self.reserves.write().await;
        reserves.add_cash(token, amount);
        
        Ok(())
    }
    
    pub async fn borrow(
        &self,
        user: [u8; 20],
        token: [u8; 20],
        amount: U256,
    ) -> Result<(), LendingError> {
        if amount == U256::zero() {
            return Err(LendingError::InvalidAmount);
        }
        
        let markets = self.markets.read().await;
        let market = markets.get(&token).ok_or(LendingError::MarketNotFound)?;
        
        // Check liquidity
        let available = market.total_supply - market.total_borrow;
        if amount > available {
            return Err(LendingError::InsufficientSupply);
        }
        
        drop(markets);
        
        // Update account
        let mut accounts = self.accounts.write().await;
        let account = accounts.entry(user).or_insert_with(|| UserAccount::new(user));
        account.borrow(token, amount, U256::from(1))?;
        
        // Update market
        let mut markets = self.markets.write().await;
        if let Some(market) = markets.get_mut(&token) {
            market.total_borrow = market.total_borrow + amount;
            market.update_rates();
        }
        
        Ok(())
    }
    
    pub async fn liquidate(
        &self,
        borrower: [u8; 20],
        liquidator: [u8; 20],
    ) -> Result<(), LendingError> {
        let mut accounts = self.accounts.write().await;
        
        let borrower_account = accounts.get_mut(&borrower).ok_or(LendingError::MarketNotFound)?;
        let liquidator_account = accounts.get_mut(&liquidator).ok_or(LendingError::MarketNotFound)?;
        
        let markets = self.markets.read().await;
        
        self.liquidation_engine.liquidate(
            borrower_account,
            &markets,
            liquidator_account,
        )
    }
    
    pub async fn get_account(&self, user: &[u8; 20]) -> Option<UserAccount> {
        let accounts = self.accounts.read().await;
        accounts.get(user).cloned()
    }
    
    pub async fn get_market(&self, token: &[u8; 20]) -> Option<Market> {
        let markets = self.markets.read().await;
        markets.get(token).cloned()
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
    
    pub type LendingProtocolHandle = Arc<LendingProtocol>;
    
    pub fn create_protocol() -> LendingProtocolHandle {
        Arc::new(LendingProtocol::new())
    }
}

// ==================== TESTS ====================

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_user_account_health() {
        let user = [0u8; 20];
        let mut account = UserAccount::new(user);
        
        // Add supply
        account.add_supply([0u8; 20], U256::from(1000), U256::from(1), &Market::new([0u8; 20]));
        
        // Should be healthy
        assert!(account.is_healthy());
    }
    
    #[test]
    fn test_borrow_limit() {
        let user = [0u8; 20];
        let mut account = UserAccount::new(user);
        
        // Add supply worth $1000
        account.add_supply([0u8; 20], U256::from(1000), U256::from(1), &Market::new([0u8; 20]));
        
        // Can borrow up to collateral value
        let max_borrow = account.get_max_borrow(U256::from(1));
        assert!(max_borrow > U256::zero());
    }
}