//! TigerSwap Concentrated Liquidity Pool - Security Critical
//! 
//! Security validation for CL pool operations

#![deny(unsafe_code)]

use std::collections::HashMap;
use std::sync::{Arc, RwLock};
use thiserror::Error;

#[derive(Error, Debug, Clone)]
pub enum CLError {
    #[error("Invalid tick range: {0}")]
    InvalidTickRange(String),
    
    #[error("Invalid liquidity: {0}")]
    InvalidLiquidity(String),
    
    #[error("Insufficient liquidity")]
    InsufficientLiquidity,
    
    #[error("Position not found")]
    PositionNotFound,
    
    #[error("Unauthorized")]
    Unauthorized,
    
    #[error("Invalid price")]
    InvalidPrice,
    
    #[error("Slippage exceeded")]
    SlippageExceeded,
}

// ============ Constants ============
pub const MIN_TICK: i32 = -887272;
pub const MAX_TICK: i32 = 887272;
pub const MIN_SQRT_RATIO: u128 = 4295128739;
pub const MAX_SQRT_RATIO: u128 = 79228162514264337593543950335;
pub const MAX_LIQUIDITY: u64 = 2^64 - 1;

// ============ Data Structures ============

#[derive(Debug, Clone)]
pub struct TickRange {
    pub tick_lower: i32,
    pub tick_upper: i32,
}

#[derive(Debug, Clone)]
pub struct CLPositionRequest {
    pub owner: String,
    pub tick_lower: i32,
    pub tick_upper: i32,
    pub liquidity: u64,
}

#[derive(Debug, Clone)]
pub struct SwapRequest {
    pub recipient: String,
    pub zero_for_one: bool,
    pub amount_in: u128,
    pub amount_out_min: u128,
    pub sqrt_price_limit: u128,
}

// ============ CL Pool Validator ============

pub struct CLPoolValidator {
    min_tick: i32,
    max_tick: i32,
    max_liquidity_per_position: u64,
    max_total_liquidity: u64,
    allowed_fees: Vec<u16>,
}

impl CLPoolValidator {
    pub fn new() -> Self {
        Self {
            min_tick: MIN_TICK,
            max_tick: MAX_TICK,
            max_liquidity_per_position: MAX_LIQUIDITY,
            max_total_liquidity: MAX_LIQUIDITY * 10,
            allowed_fees: vec![100, 500, 1000, 3000, 10000],
        }
    }
    
    /// Validate tick range
    pub fn validate_tick_range(&self, range: &TickRange) -> Result<(), CLError> {
        if range.tick_lower >= range.tick_upper {
            return Err(CLError::InvalidTickRange(
                "Lower tick must be less than upper tick".to_string()
            ));
        }
        
        if range.tick_lower < self.min_tick || range.tick_lower > self.max_tick {
            return Err(CLError::InvalidTickRange(
                format!("Lower tick out of bounds: {}", range.tick_lower)
            ));
        }
        
        if range.tick_upper < self.min_tick || range.tick_upper > self.max_tick {
            return Err(CLError::InvalidTickRange(
                format!("Upper tick out of bounds: {}", range.tick_upper)
            ));
        }
        
        // Check tick spacing
        let tick_spacing = (range.tick_upper - range.tick_lower).abs();
        if tick_spacing < 60 || tick_spacing % 60 != 0 {
            return Err(CLError::InvalidTickRange(
                "Tick spacing must be at least 60 and divisible by 60".to_string()
            ));
        }
        
        Ok(())
    }
    
    /// Validate liquidity addition
    pub fn validate_liquidity_add(
        &self,
        current_liquidity: u64,
        new_liquidity: u64,
        total_liquidity: u64
    ) -> Result<(), CLError> {
        if new_liquidity == 0 {
            return Err(CLError::InvalidLiquidity("Liquidity must be > 0".to_string()));
        }
        
        if new_liquidity > self.max_liquidity_per_position {
            return Err(CLError::InvalidLiquidity(
                format!("Liquidity exceeds max per position: {}", self.max_liquidity_per_position)
            ));
        }
        
        let new_total = current_liquidity + new_liquidity;
        if new_total > self.max_total_liquidity {
            return Err(CLError::InvalidLiquidity(
                "Total liquidity would exceed maximum".to_string()
            ));
        }
        
        if new_total > total_liquidity + new_liquidity {
            return Err(CLError::InvalidLiquidity("Liquidity overflow".to_string()));
        }
        
        Ok(())
    }
    
    /// Validate swap request
    pub fn validate_swap(&self, request: &SwapRequest) -> Result<(), CLError> {
        if request.amount_in == 0 {
            return Err(CLError::InvalidPrice("Amount in must be > 0".to_string()));
        }
        
        if request.amount_out_min == 0 {
            return Err(CLError::SlippageExceeded("Min amount out is 0".to_string()));
        }
        
        // Validate sqrt price limit
        if request.zero_for_one {
            if request.sqrt_price_limit >= MIN_SQRT_RATIO {
                return Err(CLError::InvalidPrice(
                    "sqrt_price_limit must be less than MIN_SQRT_RATIO for zero_for_one".to_string()
                ));
            }
        } else {
            if request.sqrt_price_limit <= MAX_SQRT_RATIO {
                return Err(CLError::InvalidPrice(
                    "sqrt_price_limit must be greater than MAX_SQRT_RATIO for !zero_for_one".to_string()
                ));
            }
        }
        
        Ok(())
    }
    
    /// Validate fee
    pub fn validate_fee(&self, fee: u16) -> Result<(), CLError> {
        if !self.allowed_fees.contains(&fee) {
            return Err(CLError::InvalidPrice(
                format!("Fee not allowed: {}", fee)
            ));
        }
        Ok(())
    }
    
    /// Validate price impact
    pub fn validate_price_impact(
        &self,
        price_before: u128,
        price_after: u128,
        max_impact_bps: u64
    ) -> Result<(), CLError> {
        if price_before == 0 {
            return Err(CLError::InvalidPrice("Price before is 0".to_string()));
        }
        
        let impact = ((price_before - price_after) * 10000) / price_before;
        
        if impact > max_impact_bps as u128 {
            return Err(CLError::SlippageExceeded(
                format!("Price impact {} bps exceeds max {} bps", impact, max_impact_bps)
            ));
        }
        
        Ok(())
    }
    
    /// Set max liquidity
    pub fn set_max_liquidity(&mut self, max: u64) {
        self.max_total_liquidity = max;
    }
    
    /// Add allowed fee tier
    pub fn add_allowed_fee(&mut self, fee: u16) {
        if !self.allowed_fees.contains(&fee) {
            self.allowed_fees.push(fee);
            self.allowed_fees.sort();
        }
    }
}

// ============ CL Pool Access Control ============

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum CLRole {
    None,
    Operator,
    Admin,
    Guardian,
}

pub struct CLAccessControl {
    roles: RwLock<HashMap<String, CLRole>>,
    admin: String,
}

impl CLAccessControl {
    pub fn new(admin: String) -> Self {
        let mut roles = HashMap::new();
        roles.insert(admin.clone(), CLRole::Admin);
        
        Self {
            roles: RwLock::new(roles),
            admin,
        }
    }
    
    /// Grant role
    pub fn grant_role(&self, address: &str, role: CLRole) -> Result<(), CLError> {
        let mut roles = self.roles.write().unwrap();
        
        // Check caller is admin
        let caller = std::thread::current().name().unwrap_or("");
        if roles.get(caller) != Some(&CLRole::Admin) {
            return Err(CLError::Unauthorized);
        }
        
        roles.insert(address.to_string(), role);
        Ok(())
    }
    
    /// Revoke role
    pub fn revoke_role(&self, address: &str) -> Result<(), CLError> {
        let mut roles = self.roles.write().unwrap();
        
        // Check caller is admin
        let caller = std::thread::current().name().unwrap_or("");
        if roles.get(caller) != Some(&CLRole::Admin) {
            return Err(CLError::Unauthorized);
        }
        
        // Cannot remove admin
        if address == self.admin {
            return Err(CLError::Unauthorized);
        }
        
        roles.remove(address);
        Ok(())
    }
    
    /// Check role
    pub fn has_role(&self, address: &str, role: CLRole) -> bool {
        let roles = self.roles.read().unwrap();
        roles.get(address) == Some(&role)
    }
    
    /// Only admin modifier
    pub fn only_admin(&self) -> Result<(), CLError> {
        let caller = std::thread::current().name().unwrap_or("");
        if !self.has_role(caller, CLRole::Admin) {
            return Err(CLError::Unauthorized);
        }
        Ok(())
    }
}

// ============ Emergency Controller ============

pub struct CLEmergencyController {
    paused: RwLock<bool>,
    emergency: RwLock<bool>,
    admin: String,
    guardians: RwLock<Vec<String>>,
}

impl CLEmergencyController {
    pub fn new(admin: String) -> Self {
        Self {
            paused: RwLock::new(false),
            emergency: RwLock::new(false),
            admin,
            guardians: RwLock::new(Vec::new()),
        }
    }
    
    /// Pause pool
    pub fn pause(&self) -> Result<(), CLError> {
        let mut paused = self.paused.write().unwrap();
        *paused = true;
        Ok(())
    }
    
    /// Unpause pool
    pub fn unpause(&self) -> Result<(), CLError> {
        let caller = std::thread::current().name().unwrap_or("");
        let paused = self.paused.read().unwrap();
        
        if !*paused {
            return Err(CLError::Unauthorized);
        }
        
        // Only admin or guardian can unpause
        drop(paused);
        
        let guardians = self.guardians.read().unwrap();
        let is_authorized = caller == self.admin || guardians.contains(&caller.to_string());
        
        if !is_authorized {
            return Err(CLError::Unauthorized);
        }
        
        let mut paused = self.paused.write().unwrap();
        *paused = false;
        
        Ok(())
    }
    
    /// Emergency stop
    pub fn emergency_stop(&self) -> Result<(), CLError> {
        let mut emergency = self.emergency.write().unwrap();
        *emergency = true;
        Ok(())
    }
    
    /// Check if paused
    pub fn is_paused(&self) -> bool {
        *self.paused.read().unwrap()
    }
    
    /// Check if emergency
    pub fn is_emergency(&self) -> bool {
        *self.emergency.read().unwrap()
    }
    
    /// Require not paused
    pub fn require_not_paused(&self) -> Result<(), CLError> {
        if self.is_paused() {
            return Err(CLError::Unauthorized);
        }
        Ok(())
    }
    
    /// Require not emergency
    pub fn require_not_emergency(&self) -> Result<(), CLError> {
        if self.is_emergency() {
            return Err(CLError::Unauthorized);
        }
        Ok(())
    }
}

// ============ Position Tracker ============

pub struct CLPositionTracker {
    positions: RwLock<HashMap<u64, CLPositionState>>,
    positions_by_owner: RwLock<HashMap<String, Vec<u64>>>,
}

#[derive(Debug, Clone)]
pub struct CLPositionState {
    pub id: u64,
    pub owner: String,
    pub tick_lower: i32,
    pub tick_upper: i32,
    pub liquidity: u64,
    pub tokens_owed_0: u128,
    pub tokens_owed_1: u128,
    pub created_at: u64,
    pub updated_at: u64,
}

impl CLPositionTracker {
    pub fn new() -> Self {
        Self {
            positions: RwLock::new(HashMap::new()),
            positions_by_owner: RwLock::new(HashMap::new()),
        }
    }
    
    /// Create position
    pub fn create(&self, id: u64, owner: &str, tick_lower: i32, tick_upper: i32, liquidity: u64) {
        let state = CLPositionState {
            id,
            owner: owner.to_string(),
            tick_lower,
            tick_upper,
            liquidity,
            tokens_owed_0: 0,
            tokens_owed_1: 0,
            created_at: current_timestamp(),
            updated_at: current_timestamp(),
        };
        
        // Add to positions
        self.positions.write().unwrap().insert(id, state.clone());
        
        // Add to owner index
        let mut by_owner = self.positions_by_owner.write().unwrap();
        by_owner.entry(owner.to_string()).or_insert_with(Vec::new).push(id);
    }
    
    /// Update position
    pub fn update(&self, id: u64, liquidity_delta: i64) -> Result<(), CLError> {
        let mut positions = self.positions.write().unwrap();
        
        let position = positions.get_mut(&id).ok_or(CLError::PositionNotFound)?;
        
        if liquidity_delta > 0 {
            position.liquidity += liquidity_delta as u64;
        } else {
            position.liquidity = position.liquidity.saturating_sub((-liquidity_delta) as u64);
        }
        
        position.updated_at = current_timestamp();
        
        Ok(())
    }
    
    /// Get position
    pub fn get(&self, id: u64) -> Option<CLPositionState> {
        self.positions.read().unwrap().get(&id).cloned()
    }
    
    /// Get positions by owner
    pub fn get_by_owner(&self, owner: &str) -> Vec<CLPositionState> {
        let by_owner = self.positions_by_owner.read().unwrap();
        let positions = self.positions.read().unwrap();
        
        by_owner.get(owner)
            .map(|ids| ids.iter().filter_map(|id| positions.get(id).cloned()).collect())
            .unwrap_or_default()
    }
    
    /// Get total liquidity by owner
    pub fn total_liquidity_by_owner(&self, owner: &str) -> u64 {
        self.get_by_owner(owner).iter().map(|p| p.liquidity).sum()
    }
}

fn current_timestamp() -> u64 {
    std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .unwrap()
        .as_secs()
}

// ============ Tests ============

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_tick_range_validation() {
        let validator = CLPoolValidator::new();
        
        // Valid range
        assert!(validator.validate_tick_range(&TickRange {
            tick_lower: -1000,
            tick_upper: 1000
        }).is_ok());
        
        // Invalid: lower >= upper
        assert!(validator.validate_tick_range(&TickRange {
            tick_lower: 1000,
            tick_upper: -1000
        }).is_err());
        
        // Invalid: out of bounds
        assert!(validator.validate_tick_range(&TickRange {
            tick_lower: MIN_TICK - 1,
            tick_upper: 1000
        }).is_err());
    }
    
    #[test]
    fn test_swap_validation() {
        let validator = CLPoolValidator::new();
        
        // Valid swap
        assert!(validator.validate_swap(&SwapRequest {
            recipient: "0x123".to_string(),
            zero_for_one: true,
            amount_in: 1000,
            amount_out_min: 1,
            sqrt_price_limit: 1000
        }).is_ok());
        
        // Invalid: amount_in = 0
        assert!(validator.validate_swap(&SwapRequest {
            recipient: "0x123".to_string(),
            zero_for_one: true,
            amount_in: 0,
            amount_out_min: 1,
            sqrt_price_limit: 1000
        }).is_err());
    }
}