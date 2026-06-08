//! TigerSwap IL Protection Engine
//! 
//! Implements impermanent loss protection for liquidity providers:
//! - IL Insurance
//! - Buffered IL protection
//! - Delta-neutral strategies
//! - Hedging mechanisms
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
pub const CHAIN_BSC: u64 = 56;
pub const CHAIN_POLYGON: u64 = 137;
pub const CHAIN_ARBITRUM: u64 = 42161;

#[derive(Debug, Error)]
pub enum ILError {
    #[error("Position not found: {0}")]
    PositionNotFound(String),
    #[error("Insufficient balance: {0}")]
    InsufficientBalance(String),
    #[error("Invalid parameters: {0}")]
    InvalidParameters(String),
    #[error("Protection not active: {0}")]
    ProtectionNotActive(String),
    #[error("Claim too early: {0}")]
    ClaimTooEarly(String),
    #[error("Chain not supported: {0}")]
    ChainNotSupported(u64),
}

/// Protection type
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum ProtectionType {
    Buffered,       // Buffer pool to cover IL
    Insurance,      // Insurance fund
    DeltaNeutral,  // Delta-neutral LP
    Hedging,        // External hedging
}

impl Default for ProtectionType {
    fn default() -> Self { ProtectionType::Buffered }
}

/// IL Protection position
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ILProtection {
    pub position_id: String,
    pub user: String,
    pub chain_id: u64,
    pub pool_id: String,
    pub protection_type: ProtectionType,
    
    // LP Position
    pub lp_token_amount: u128,
    pub token0_amount: u128,
    pub token1_amount: u128,
    
    // Entry state
    pub entry_price0: Decimal,
    pub entry_price1: Decimal,
    pub entry_value: u128,
    
    // Current state
    pub current_price0: Decimal,
    pub current_price1: Decimal,
    pub current_value: u128,
    
    // IL calculation
    pub il_loss: i128,        // Calculated IL
    pub protection_covered: u128,  // Amount covered
    pub claimable: u128,      // Amount claimable
    
    // Insurance
    pub premium_paid: u128,
    pub coverage_start: i64,
    pub coverage_end: i64,
    
    // Status
    pub is_active: bool,
    pub created_at: i64,
    pub updated_at: i64,
}

impl ILProtection {
    /// Create new protection position
    pub fn new(
        user: String,
        chain_id: u64,
        pool_id: String,
        protection_type: ProtectionType,
        lp_amount: u128,
        token0_amount: u128,
        token1_amount: u128,
        entry_price0: Decimal,
        entry_price1: Decimal,
        premium: u128,
        duration_days: i64,
    ) -> Self {
        let entry_value = token0_amount * entry_price0.as_u128() + token1_amount * entry_price1.as_u128();
        
        Self {
            position_id: Uuid::new_v4().to_string(),
            user,
            chain_id,
            pool_id,
            protection_type,
            lp_token_amount: lp_amount,
            token0_amount,
            token1_amount,
            entry_price0,
            entry_price1,
            entry_value,
            current_price0: entry_price0,
            current_price1: entry_price1,
            current_value: entry_value,
            il_loss: 0,
            protection_covered: 0,
            claimable: 0,
            premium_paid: premium,
            coverage_start: Utc::now().timestamp(),
            coverage_end: Utc::now().timestamp() + (duration_days * 24 * 60 * 60),
            is_active: true,
            created_at: Utc::now().timestamp(),
            updated_at: Utc::now().timestamp(),
        }
    }

    /// Calculate impermanent loss
    pub fn calculate_il(&mut self, current_price0: Decimal, current_price1: Decimal) -> i128 {
        self.current_price0 = current_price0;
        self.current_price1 = current_price1;
        
        // Current value with new prices
        let current_value = 
            self.token0_amount * current_price0.as_u128() +
            self.token1_amount * current_price1.as_u128();
        
        self.current_value = current_value;
        
        // Calculate what value would be without IL (simple hold)
        let hold0 = self.token0_amount * self.entry_price0.as_u128();
        let hold1 = self.token1_amount * self.entry_price1.as_u128();
        let hold_value = hold0 + hold1;
        
        // IL = (LP value) - (Hold value)
        let il = current_value as i128 - hold_value as i128;
        
        self.il_loss = il;
        
        il
    }

    /// Calculate protection coverage
    pub fn calculate_coverage(&self, coverage_ratio: u256) -> u128 {
        if self.il_loss >= 0 {
            return 0; // No loss, no claim
        }
        
        // Coverage based on ratio
        let loss = (-self.il_loss) as u256;
        let coverage = (loss * coverage_ratio) / 10000;
        
        coverage as u128
    }

    /// Claim protection
    pub fn claim(&mut self, current_price0: Decimal, current_price1: Decimal) -> Result<u128, ILError> {
        if !self.is_active {
            return Err(ILError::ProtectionNotActive(self.position_id.clone()));
        }
        
        // Calculate current IL
        self.calculate_il(current_price0, current_price1);
        
        if self.il_loss >= 0 {
            return Ok(0); // Profitable, no IL
        }
        
        // Calculate claimable (capped at premium paid)
        let claim = (-self.il_loss) as u128;
        self.claimable = claim.min(self.premium_paid);
        
        if self.claimable == 0 {
            return Ok(0);
        }
        
        self.updated_at = Utc::now().timestamp();
        
        Ok(self.claimable)
    }

    /// Extend coverage
    pub fn extend_coverage(&mut self, additional_days: i64, additional_premium: u128) -> Result<(), ILError> {
        if additional_premium > 0 {
            self.premium_paid += additional_premium;
        }
        
        self.coverage_end += additional_days * 24 * 60 * 60;
        self.updated_at = Utc::now().timestamp();
        
        Ok(())
    }

    /// Check if coverage is active
    pub fn is_coverage_active(&self) -> bool {
        let now = Utc::now().timestamp();
        self.is_active && now < self.coverage_end
    }

    /// Get remaining coverage time
    pub fn remaining_time(&self) -> i64 {
        let remaining = self.coverage_end - Utc::now().timestamp();
        if remaining > 0 { remaining } else { 0 }
    }
}

/// IL Buffer pool
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ILBufferPool {
    pub pool_id: String,
    pub chain_id: u64,
    pub token: String,
    pub total_deposits: u128,
    pub total_claims: u128,
    pub buffer_balance: u128,
    pub min_coverage_ratio: u256,
    pub apy: u256,
    pub created_at: i64,
}

impl ILBufferPool {
    pub fn new(chain_id: u64, token: String) -> Self {
        Self {
            pool_id: Uuid::new_v4().to_string(),
            chain_id,
            token,
            total_deposits: 0,
            total_claims: 0,
            buffer_balance: 0,
            min_coverage_ratio: 7500, // 75% coverage
            apy: 500, // 5% APY
            created_at: Utc::now().timestamp(),
        }
    }

    pub fn deposit(&mut self, amount: u128) {
        self.buffer_balance += amount;
        self.total_deposits += amount;
    }

    pub fn claim(&mut self, amount: u128) -> bool {
        if self.buffer_balance >= amount {
            self.buffer_balance -= amount;
            self.total_claims += amount;
            true
        } else {
            false
        }
    }

    pub fn get_utilization(&self) -> u256 {
        if self.total_deposits == 0 {
            0
        } else {
            (self.total_claims as u256 * 10000) / self.total_deposits as u256
        }
    }
}

/// Insurance fund
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct InsuranceFund {
    pub fund_id: String,
    pub chain_id: u64,
    pub token: String,
    pub total_premiums: u128,
    pub total_payouts: u128,
    pub fund_balance: u128,
    pub risk_reserve: u128,
    pub created_at: i64,
}

impl InsuranceFund {
    pub fn new(chain_id: u64, token: String) -> Self {
        Self {
            fund_id: Uuid::new_v4().to_string(),
            chain_id,
            token,
            total_premiums: 0,
            total_payouts: 0,
            fund_balance: 0,
            risk_reserve: 0,
            created_at: Utc::now().timestamp(),
        }
    }

    pub fn collect_premium(&mut self, amount: u128) {
        self.total_premiums += amount;
        self.fund_balance += amount;
    }

    pub fn pay_out(&mut self, amount: u128) -> Result<u128, ILError> {
        let available = self.fund_balance.saturating_sub(self.risk_reserve);
        let payout = amount.min(available);
        
        if payout == 0 {
            return Err(ILError::InsufficientBalance("Fund exhausted".to_string()));
        }
        
        self.fund_balance -= payout;
        self.total_payouts += payout;
        
        Ok(payout)
    }

    pub fn set_risk_reserve(&mut self, amount: u128) {
        self.risk_reserve = amount;
    }

    pub fn get_solvency(&self) -> u256 {
        if self.fund_balance == 0 {
            0
        } else {
            (self.fund_balance.saturating_sub(self.risk_reserve) * 10000) / self.fund_balance
        }
    }
}

/// IL Protection Engine
pub struct ILProtectionEngine {
    positions: Arc<RwLock<HashMap<String, ILProtection>>>,
    buffer_pools: Arc<RwLock<HashMap<String, ILBufferPool>>>,
    insurance_funds: Arc<RwLock<HashMap<String, InsuranceFund>>>,
    supported_chains: Arc<RwLock<std::collections::HashSet<u64>>>,
}

impl ILProtectionEngine {
    pub fn new() -> Self {
        let chains: std::collections::HashSet<u64> = [
            CHAIN_ETH, CHAIN_BSC, CHAIN_POLYGON, CHAIN_ARBITRUM,
        ].into_iter().collect();
        
        Self {
            positions: Arc::new(RwLock::new(HashMap::new())),
            buffer_pools: Arc::new(RwLock::new(HashMap::new())),
            insurance_funds: Arc::new(RwLock::new(HashMap::new())),
            supported_chains: Arc::new(RwLock::new(chains)),
        }
    }

    pub fn is_chain_supported(&self, chain_id: u64) -> bool {
        self.supported_chains.read().contains(&chain_id)
    }

    /// Create protection position
    pub fn create_protection(
        &self,
        user: String,
        chain_id: u64,
        pool_id: String,
        protection_type: ProtectionType,
        lp_amount: u128,
        token0_amount: u128,
        token1_amount: u128,
        entry_price0: Decimal,
        entry_price1: Decimal,
        premium: u128,
        duration_days: i64,
    ) -> Result<String, ILError> {
        if !self.is_chain_supported(chain_id) {
            return Err(ILError::ChainNotSupported(chain_id));
        }
        
        let protection = ILProtection::new(
            user,
            chain_id,
            pool_id,
            protection_type,
            lp_amount,
            token0_amount,
            token1_amount,
            entry_price0,
            entry_price1,
            premium,
            duration_days,
        );
        
        let position_id = protection.position_id.clone();
        
        // Collect premium to fund
        match protection_type {
            ProtectionType::Insurance => {
                let mut funds = self.insurance_funds.write();
                let fund = funds.entry(pool_id.clone()).or_insert_with(|| InsuranceFund::new(chain_id, "USDC".to_string()));
                fund.collect_premium(premium);
            }
            ProtectionType::Buffered => {
                let mut pools = self.buffer_pools.write();
                let pool = pools.entry(pool_id.clone()).or_insert_with(|| ILBufferPool::new(chain_id, "USDC".to_string()));
                pool.deposit(premium);
            }
            _ => {}
        }
        
        self.positions.write().insert(position_id.clone(), protection);
        
        Ok(position_id)
    }

    /// Get position
    pub fn get_position(&self, position_id: &str) -> Option<ILProtection> {
        self.positions.read().get(position_id).cloned()
    }

    /// Update prices and calculate IL
    pub fn update_prices(
        &self,
        position_id: &str,
        current_price0: Decimal,
        current_price1: Decimal,
    ) -> Result<i128, ILError> {
        let mut positions = self.positions.write();
        let position = positions.get_mut(position_id)
            .ok_or_else(|| ILError::PositionNotFound(position_id.to_string()))?;
        
        Ok(position.calculate_il(current_price0, current_price1))
    }

    /// Claim protection
    pub fn claim(
        &self,
        position_id: &str,
        current_price0: Decimal,
        current_price1: Decimal,
    ) -> Result<u128, ILError> {
        let mut positions = self.positions.write();
        let position = positions.get_mut(position_id)
            .ok_or_else(|| ILError::PositionNotFound(position_id.to_string()))?;
        
        // Calculate claim
        let claim_amount = position.claim(current_price0, current_price1)?;
        
        if claim_amount == 0 {
            return Ok(0);
        }
        
        // Pay from appropriate source
        match position.protection_type {
            ProtectionType::Insurance => {
                let mut funds = self.insurance_funds.write();
                if let Some(fund) = funds.get_mut(&position.pool_id) {
                    fund.pay_out(claim_amount)?;
                }
            }
            ProtectionType::Buffered => {
                let mut pools = self.buffer_pools.write();
                if let Some(pool) = pools.get_mut(&position.pool_id) {
                    if !pool.claim(claim_amount) {
                        return Err(ILError::InsufficientBalance("Buffer exhausted".to_string()));
                    }
                }
            }
            _ => {}
        }
        
        Ok(claim_amount)
    }

    /// Extend coverage
    pub fn extend_coverage(
        &self,
        position_id: &str,
        additional_days: i64,
        additional_premium: u128,
    ) -> Result<(), ILError> {
        let mut positions = self.positions.write();
        let position = positions.get_mut(position_id)
            .ok_or_else(|| ILError::PositionNotFound(position_id.to_string()))?;
        
        position.extend_coverage(additional_days, additional_premium)
    }

    /// Get user positions
    pub fn get_user_positions(&self, user: &str) -> Vec<ILProtection> {
        self.positions.read()
            .values()
            .filter(|p| p.user == user)
            .cloned()
            .collect()
    }

    /// Get buffer pool
    pub fn get_buffer_pool(&self, pool_id: &str) -> Option<ILBufferPool> {
        self.buffer_pools.read().get(pool_id).cloned()
    }

    /// Get insurance fund
    pub fn get_insurance_fund(&self, pool_id: &str) -> Option<InsuranceFund> {
        self.insurance_funds.read().get(pool_id).cloned()
    }

    /// Get statistics
    pub fn get_stats(&self) -> ILStats {
        let positions = self.positions.read();
        let mut active = 0;
        let mut total_il_covered = 0u128;
        
        for position in positions.values() {
            if position.is_active {
                active += 1;
                total_il_covered += position.protection_covered;
            }
        }
        
        ILStats {
            active_positions: active,
            total_il_covered,
            total_buffer: self.buffer_pools.read().values().map(|p| p.buffer_balance).sum(),
            total_fund: self.insurance_funds.read().values().map(|f| f.fund_balance).sum(),
        }
    }

    /// Add supported chain
    pub fn add_chain(&self, chain_id: u64) {
        self.supported_chains.write().insert(chain_id);
    }

    pub fn supported_chains(&self) -> Vec<u64> {
        self.supported_chains.read().iter().cloned().collect()
    }
}

impl Default for ILProtectionEngine {
    fn default() -> Self { Self::new() }
}

/// IL Statistics
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ILStats {
    pub active_positions: usize,
    pub total_il_covered: u128,
    pub total_buffer: u128,
    pub total_fund: u128,
}

#[cfg(test)]
mod tests {
    use super::*;
    use rust_decimal_macros::dec;

    #[test]
    fn test_protection_creation() {
        let protection = ILProtection::new(
            "user1".to_string(),
            CHAIN_ETH,
            "pool1".to_string(),
            ProtectionType::Buffered,
            1000,
            500,
            500,
            dec!(1.0),
            dec!(2000.0),
            100,
            30,
        );
        
        assert_eq!(protection.protection_type, ProtectionType::Buffered);
    }

    #[test]
    fn test_il_calculation() {
        let mut protection = ILProtection::new(
            "user1".to_string(),
            CHAIN_ETH,
            "pool1".to_string(),
            ProtectionType::Buffered,
            1000,
            500,
            500,
            dec!(1.0),
            dec!(2000.0),
            100,
            30,
        );
        
        // ETH price drops from 2000 to 1500
        let il = protection.calculate_il(dec!(1.0), dec!(1500.0));
        
        assert!(il < 0); // Should be negative (loss)
    }

    #[test]
    fn test_claim() {
        let mut protection = ILProtection::new(
            "user1".to_string(),
            CHAIN_ETH,
            "pool1".to_string(),
            ProtectionType::Insurance,
            1000,
            500,
            500,
            dec!(1.0),
            dec!(2000.0),
            100,
            30,
        );
        
        // ETH price drops
        let claim = protection.claim(dec!(1.0), dec!(1500.0)).unwrap();
        
        assert!(claim > 0);
    }

    #[test]
    fn test_buffer_pool() {
        let mut pool = ILBufferPool::new(CHAIN_ETH, "USDC".to_string());
        
        pool.deposit(10000);
        
        assert_eq!(pool.buffer_balance, 10000);
    }
}