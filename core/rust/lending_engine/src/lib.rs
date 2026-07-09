//! TigerSwap Lending Engine
//! Lending and borrowing protocol with collateral management

use std::collections::HashMap;
use parking_lot::RwLock;
use serde::{Deserialize, Serialize};
use chrono::{DateTime, Utc};
use uuid::Uuid;
use rust_decimal::Decimal;

// ============================================================================
// Types
// ============================================================================

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Market {
    pub id: String,
    pub token: String,
    
    // Supply
    pub supply_rate: Decimal,
    pub total_supply: Decimal,
    pub supply_index: Decimal,
    
    // Borrow
    pub borrow_rate: Decimal,
    pub total_borrow: Decimal,
    pub borrow_index: Decimal,
    
    // Collateral
    pub collateral_factor: Decimal,
    pub liquidation_threshold: Decimal,
    pub liquidation_penalty: Decimal,
    
    // Reserves
    pub reserve_factor: Decimal,
    pub total_reserves: Decimal,
    
    // Status
    pub is_active: bool,
    pub is_collateral_enabled: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SupplyPosition {
    pub id: String,
    pub user_id: String,
    pub market_id: String,
    
    pub balance: Decimal,
    pub accrual_start_timestamp: i64,
    pub supply_index: Decimal,
    
    pub entered_at: DateTime<Utc>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct BorrowPosition {
    pub id: String,
    pub user_id: String,
    pub market_id: String,
    
    pub borrow_balance: Decimal,
    pub accrual_start_timestamp: i64,
    pub borrow_index: Decimal,
    
    pub created_at: DateTime<Utc>,
    pub updated_at: DateTime<Utc>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Account {
    pub user_id: String,
    
    // Collateral values (in USD)
    pub total_collateral_usd: Decimal,
    pub total_borrow_usd: Decimal,
    
    // Health factor
    pub health_factor: Decimal,
    pub liquidation_threshold: Decimal,
    
    pub status: AccountStatus,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum AccountStatus {
    Healthy,
    BelowLiquidationThreshold,
    Liquidatable,
    Bankrupt,
}

// ============================================================================
// Lending Engine
// ============================================================================

pub struct LendingEngine {
    markets: RwLock<HashMap<String, Market>>,
    supply_positions: RwLock<HashMap<String, SupplyPosition>>,
    borrow_positions: RwLock<HashMap<String, BorrowPosition>>,
    accounts: RwLock<HashMap<String, Account>>,
    
    // Config
    reserve_factor: Decimal,
    close_factor: Decimal,
    liquidation_incentive: Decimal,
}

impl LendingEngine {
    pub fn new() -> Self {
        Self {
            markets: RwLock::new(HashMap::new()),
            supply_positions: RwLock::new(HashMap::new()),
            borrow_positions: RwLock::new(HashMap::new()),
            accounts: RwLock::new(HashMap::new()),
            reserve_factor: Decimal::from(1000), // 10%
            close_factor: Decimal::from(5000),   // 50%
            liquidation_incentive: Decimal::from(500), // 5%
        }
    }
    
    // ========================================================================
    // Market Management
    // ========================================================================
    
    pub fn add_market(&self, market: Market) {
        self.markets.write().insert(market.id.clone(), market);
    }
    
    pub fn get_market(&self, market_id: &str) -> Option<Market> {
        self.markets.read().get(market_id).cloned()
    }
    
    pub fn list_markets(&self) -> Vec<Market> {
        self.markets.read().values().cloned().collect()
    }
    
    // ========================================================================
    // Supply (Lending)
    // ========================================================================
    
    pub fn supply(&self, user_id: &str, market_id: &str, amount: Decimal) -> Result<SupplyPosition, &'static str> {
        // Validate market
        let market = self.markets.read()
            .get(market_id)
            .ok_or("Market not found")?;
        
        if !market.is_active {
            return Err("Market is not active");
        }
        
        let position_id = format!("{}_{}", user_id, market_id);
        
        let mut supply_positions = self.supply_positions.write();
        
        // Update or create position
        let position = if let Some(existing) = supply_positions.get_mut(&position_id) {
            existing.balance += amount;
            existing.updated_at = Utc::now();
            existing
        } else {
            let position = SupplyPosition {
                id: position_id.clone(),
                user_id: user_id.to_string(),
                market_id: market_id.to_string(),
                balance: amount,
                accrual_start_timestamp: Utc::now().timestamp(),
                supply_index: market.supply_index,
                entered_at: Utc::now(),
            };
            supply_positions.insert(position_id, position);
            supply_positions.get_mut(&position_id).unwrap()
        };
        
        // Update market total supply
        let mut markets = self.markets.write();
        if let Some(m) = markets.get_mut(market_id) {
            m.total_supply += amount;
        }
        
        // Update account
        self.update_account(user_id);
        
        Ok(position.clone())
    }
    
    pub fn withdraw(&self, user_id: &str, market_id: &str, amount: Decimal) -> Result<Decimal, &'static str> {
        let position_id = format!("{}_{}", user_id, market_id);
        
        let mut supply_positions = self.supply_positions.write();
        let position = supply_positions.get_mut(&position_id).ok_or("Position not found")?;
        
        if position.balance < amount {
            return Err("Insufficient balance");
        }
        
        // Check borrow collateral
        if !self.can_withdraw(user_id, amount) {
            return Err("Cannot withdraw - would cause undercollateralization");
        }
        
        position.balance -= amount;
        let withdrawn = amount;
        
        // Update market
        let mut markets = self.markets.write();
        if let Some(m) = markets.get_mut(market_id) {
            m.total_supply -= withdrawn;
        }
        
        // Update account
        drop(supply_positions);
        self.update_account(user_id);
        
        Ok(withdrawn)
    }
    
    // ========================================================================
    // Borrow
    // ========================================================================
    
    pub fn borrow(&self, user_id: &str, market_id: &str, amount: Decimal) -> Result<BorrowPosition, &'static str> {
        // Validate market
        let market = self.markets.read()
            .get(market_id)
            .ok_or("Market not found")?;
        
        if !market.is_active {
            return Err("Market is not active");
        }
        
        // Check collateral
        self.update_account(user_id);
        let account = self.accounts.read()
            .get(user_id)
            .ok_or("Account not found")?;
        
        let max_borrow = account.total_collateral_usd * account.liquidation_threshold / Decimal::from(10000) - account.total_borrow_usd;
        let borrow_usd = amount; // Simplified - would convert from token price
        
        if borrow_usd > max_borrow {
            return Err("Insufficient collateral");
        }
        
        let position_id = format!("{}_{}_borrow", user_id, market_id);
        
        let mut borrow_positions = self.borrow_positions.write();
        
        let position = if let Some(existing) = borrow_positions.get_mut(&position_id) {
            existing.borrow_balance += amount;
            existing.updated_at = Utc::now();
            existing
        } else {
            let position = BorrowPosition {
                id: position_id.clone(),
                user_id: user_id.to_string(),
                market_id: market_id.to_string(),
                borrow_balance: amount,
                accrual_start_timestamp: Utc::now().timestamp(),
                borrow_index: market.borrow_index,
                created_at: Utc::now(),
                updated_at: Utc::now(),
            };
            borrow_positions.insert(position_id, position);
            borrow_positions.get_mut(&position_id).unwrap()
        };
        
        // Update market total borrow
        let mut markets = self.markets.write();
        if let Some(m) = markets.get_mut(market_id) {
            m.total_borrow += amount;
        }
        
        // Update account
        drop(borrow_positions);
        self.update_account(user_id);
        
        Ok(position.clone())
    }
    
    pub fn repay(&self, user_id: &str, market_id: &str, amount: Decimal) -> Result<Decimal, &'static str> {
        let position_id = format!("{}_{}_borrow", user_id, market_id);
        
        let mut borrow_positions = self.borrow_positions.write();
        let position = borrow_positions.get_mut(&position_id).ok_or("Position not found")?;
        
        let repay_amount = amount.min(position.borrow_balance);
        position.borrow_balance -= repay_amount;
        
        // Update market
        let mut markets = self.markets.write();
        if let Some(m) = markets.get_mut(market_id) {
            m.total_borrow -= repay_amount;
        }
        
        // Update account
        drop(borrow_positions);
        self.update_account(user_id);
        
        Ok(repay_amount)
    }
    
    // ========================================================================
    // Liquidation
    // ========================================================================
    
    pub fn liquidate(
        &self,
        liquidator_id: &str,
        user_id: &str,
        repay_market_id: &str,
        collateral_market_id: &str,
        repay_amount: Decimal,
    ) -> Result<(Decimal, Decimal), &'static str> {
        // Check if user is liquidatable
        let account = self.accounts.read()
            .get(user_id)
            .ok_or("Account not found")?;
        
        if account.status != AccountStatus::Liquidatable {
            return Err("Account is not liquidatable");
        }
        
        let repay_position_id = format!("{}_{}_borrow", user_id, repay_market_id);
        let collateral_position_id = format!("{}_{}", user_id, collateral_market_id);
        
        // Get markets
        let markets = self.markets.read();
        let repay_market = markets.get(repay_market_id).ok_or("Repay market not found")?;
        let collateral_market = markets.get(collateral_market_id).ok_or("Collateral market not found")?;
        
        // Calculate liquidation amounts
        let max_repay = repay_amount * self.close_factor;
        let seize_amount = max_repay * (Decimal::ONE + self.liquidation_incentive);
        
        // Execute liquidation
        let mut borrow_positions = self.borrow_positions.write();
        if let Some(pos) = borrow_positions.get_mut(&repay_position_id) {
            pos.borrow_balance -= max_repay;
        }
        
        let mut supply_positions = self.supply_positions.write();
        if let Some(pos) = supply_positions.get_mut(&collateral_position_id) {
            pos.balance -= seize_amount;
        }
        
        // Update markets
        drop(borrow_positions);
        drop(supply_positions);
        drop(markets);
        
        let mut markets = self.markets.write();
        if let Some(m) = markets.get_mut(repay_market_id) {
            m.total_borrow -= max_repay;
        }
        if let Some(m) = markets.get_mut(collateral_market_id) {
            m.total_supply -= seize_amount;
            m.total_reserves += seize_amount * m.reserve_factor / Decimal::from(10000);
        }
        
        // Update accounts
        self.update_account(user_id);
        
        Ok((max_repay, seize_amount))
    }
    
    // ========================================================================
    // Account Management
    // ========================================================================
    
    fn update_account(&self, user_id: &str) {
        let markets = self.markets.read();
        
        // Calculate total collateral
        let supply_positions = self.supply_positions.read();
        let mut total_collateral = Decimal::ZERO;
        
        for pos in supply_positions.values() {
            if pos.user_id == user_id {
                if let Some(market) = markets.get(&pos.market_id) {
                    if market.is_collateral_enabled {
                        // Simplified - would use oracle price
                        total_collateral += pos.balance;
                    }
                }
            }
        }
        
        // Calculate total borrow
        let borrow_positions = self.borrow_positions.read();
        let mut total_borrow = Decimal::ZERO;
        
        for pos in borrow_positions.values() {
            if pos.user_id == user_id {
                if let Some(market) = markets.get(&pos.market_id) {
                    total_borrow += pos.borrow_balance;
                }
            }
        }
        
        // Calculate health factor
        let liquidation_threshold = Decimal::from(8000); // 80%
        let health_factor = if total_borrow > Decimal::ZERO {
            (total_collateral * liquidation_threshold / Decimal::from(10000)) / total_borrow * Decimal::from(10000)
        } else {
            Decimal::from(10000) // Max health
        };
        
        let status = if health_factor >= Decimal::from(10000) {
            AccountStatus::Healthy
        } else if health_factor >= Decimal::from(8000) {
            AccountStatus::BelowLiquidationThreshold
        } else if health_factor > Decimal::ZERO {
            AccountStatus::Liquidatable
        } else {
            AccountStatus::Bankrupt
        };
        
        let account = Account {
            user_id: user_id.to_string(),
            total_collateral_usd: total_collateral,
            total_borrow_usd: total_borrow,
            health_factor,
            liquidation_threshold,
            status,
        };
        
        self.accounts.write().insert(user_id.to_string(), account);
    }
    
    fn can_withdraw(&self, user_id: &str, amount: Decimal) -> bool {
        let account = match self.accounts.read().get(user_id) {
            Some(a) => a,
            None => return true,
        };
        
        // Simplified check
        account.health_factor >= Decimal::from(10000)
    }
    
    pub fn get_account(&self, user_id: &str) -> Option<Account> {
        self.update_account(user_id);
        self.accounts.read().get(user_id).cloned()
    }
}

impl Default for LendingEngine {
    fn default() -> Self {
        Self::new()
    }
}

// ============================================================================
// Tests
// ============================================================================

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_supply_borrow() {
        let engine = LendingEngine::new();
        
        // Add market
        engine.add_market(Market {
            id: "ETH".to_string(),
            token: "ETH".to_string(),
            supply_rate: Decimal::from(500),
            total_supply: Decimal::ZERO,
            supply_index: Decimal::ONE,
            borrow_rate: Decimal::from(1000),
            total_borrow: Decimal::ZERO,
            borrow_index: Decimal::ONE,
            collateral_factor: Decimal::from(8000), // 80%
            liquidation_threshold: Decimal::from(8500), // 85%
            liquidation_penalty: Decimal::from(500), // 5%
            reserve_factor: Decimal::from(1000), // 10%
            total_reserves: Decimal::ZERO,
            is_active: true,
            is_collateral_enabled: true,
        });
        
        // Supply
        let result = engine.supply("user1", "ETH", Decimal::from(10));
        assert!(result.is_ok());
        
        // Borrow
        let result = engine.borrow("user1", "USDT", Decimal::from(5));
        assert!(result.is_ok());
        
        // Check account
        let account = engine.get_account("user1");
        assert!(account.is_some());
    }
}
