//! Position management for liquidity providers

use serde::{Deserialize, Serialize};
use num_bigint::BigUint;
use std::collections::HashMap;

/// Unique position identifier
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
pub struct PositionId(pub u64);

impl PositionId {
    pub fn new(id: u64) -> Self {
        Self(id)
    }

    pub fn as_u64(&self) -> u64 {
        self.0
    }
}

impl Default for PositionId {
    fn default() -> Self {
        Self(0)
    }
}

/// Liquidity position in a concentrated liquidity pool
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Position {
    pub id: PositionId,
    pub owner: String,
    pub token0: String,
    pub token1: String,
    pub tick_lower: i32,
    pub tick_upper: i32,
    pub liquidity: BigUint,
    pub fee_growth_inside_0: BigUint,
    pub fee_growth_inside_1: BigUint,
    pub tokens_owed_0: BigUint,
    pub tokens_owed_1: BigUint,
    pub deposited_token0: BigUint,
    pub deposited_token1: BigUint,
}

impl Position {
    pub fn new(
        id: PositionId,
        owner: String,
        token0: String,
        token1: String,
        tick_lower: i32,
        tick_upper: i32,
        liquidity: BigUint,
    ) -> Self {
        Self {
            id,
            owner,
            token0,
            token1,
            tick_lower,
            tick_upper,
            liquidity,
            fee_growth_inside_0: BigUint::from(0u64),
            fee_growth_inside_1: BigUint::from(0u64),
            tokens_owed_0: BigUint::from(0u64),
            tokens_owed_1: BigUint::from(0u64),
            deposited_token0: BigUint::from(0u64),
            deposited_token1: BigUint::from(0u64),
        }
    }

    /// Update position with new liquidity
    pub fn update_liquidity(&mut self, liquidity_delta: &BigUint) {
        if liquidity_delta > &BigUint::from(0u64) {
            self.liquidity += liquidity_delta.clone();
        } else {
            self.liquidity -= liquidity_delta.clone();
        }
    }

    /// Collect fees from position
    pub fn collect(&mut self) -> (BigUint, BigUint) {
        let owed_0 = self.tokens_owed_0.clone();
        let owed_1 = self.tokens_owed_1.clone();
        
        self.tokens_owed_0 = BigUint::from(0u64);
        self.tokens_owed_1 = BigUint::from(0u64);
        
        (owed_0, owed_1)
    }

    /// Update fee growth inside
    pub fn update_fee_growth(
        &mut self,
        fee_growth_0: &BigUint,
        fee_growth_1: &BigUint,
        tick_lower: i32,
        tick_upper: i32,
        current_tick: i32,
        fee_growth_outside_lower: &BigUint,
        fee_growth_outside_upper: &BigUint,
    ) {
        let (fee_growth_inside_0, fee_growth_inside_1) = Self::calculate_fee_growth_inside(
            fee_growth_0,
            fee_growth_1,
            tick_lower,
            tick_upper,
            current_tick,
            fee_growth_outside_lower,
            fee_growth_outside_upper,
        );
        
        // Calculate uncollected fees
        let uncollected_0 = if fee_growth_inside_0 > self.fee_growth_inside_0 {
            let diff = &fee_growth_inside_0 - &self.fee_growth_inside_0;
            (&diff * &self.liquidity) / BigUint::from(1u64) // Simplified
        } else {
            BigUint::from(0u64)
        };
        
        let uncollected_1 = if fee_growth_inside_1 > self.fee_growth_inside_1 {
            let diff = &fee_growth_inside_1 - &self.fee_growth_inside_1;
            (&diff * &self.liquidity) / BigUint::from(1u64)
        } else {
            BigUint::from(0u64)
        };
        
        self.tokens_owed_0 += uncollected_0;
        self.tokens_owed_1 += uncollected_1;
        self.fee_growth_inside_0 = fee_growth_inside_0;
        self.fee_growth_inside_1 = fee_growth_inside_1;
    }

    /// Calculate fee growth inside a tick range
    fn calculate_fee_growth_inside(
        fee_growth_global_0: &BigUint,
        fee_growth_global_1: &BigUint,
        tick_lower: i32,
        tick_upper: i32,
        current_tick: i32,
        fee_growth_outside_lower: &BigUint,
        fee_growth_outside_upper: &BigUint,
    ) -> (BigUint, BigUint) {
        let mut fee_growth_inside_0 = fee_growth_global_0.clone();
        let mut fee_growth_inside_1 = fee_growth_global_1.clone();
        
        // Adjust for lower tick
        if current_tick >= tick_lower {
            fee_growth_inside_0 -= fee_growth_outside_lower.clone();
            fee_growth_inside_1 -= fee_growth_outside_lower.clone();
        } else {
            fee_growth_inside_0 = fee_growth_outside_lower.clone();
            fee_growth_inside_1 = fee_growth_outside_lower.clone();
        }
        
        // Adjust for upper tick
        if current_tick >= tick_upper {
            fee_growth_inside_0 -= fee_growth_outside_upper.clone();
            fee_growth_inside_1 -= fee_growth_outside_upper.clone();
        }
        
        (fee_growth_inside_0, fee_growth_inside_1)
    }

    /// Check if position is still in range
    pub fn is_in_range(&self, current_tick: i32) -> bool {
        current_tick >= self.tick_lower && current_tick < self.tick_upper
    }

    /// Get position value in terms of token amounts
    pub fn get_value(&self, amount0: &BigUint, amount1: &BigUint) -> (BigUint, BigUint) {
        // In production, this would calculate the actual token amounts
        // based on liquidity and current price
        (amount0.clone(), amount1.clone())
    }
}

/// Position manager for tracking all positions
pub struct PositionManager {
    positions: HashMap<PositionId, Position>,
    next_id: u64,
    positions_by_owner: HashMap<String, Vec<PositionId>>,
}

impl PositionManager {
    pub fn new() -> Self {
        Self {
            positions: HashMap::new(),
            next_id: 1,
            positions_by_owner: HashMap::new(),
        }
    }

    /// Create a new position
    pub fn create_position(
        &mut self,
        owner: String,
        token0: String,
        token1: String,
        tick_lower: i32,
        tick_upper: i32,
        liquidity: BigUint,
    ) -> PositionId {
        let id = PositionId::new(self.next_id);
        self.next_id += 1;
        
        let position = Position::new(
            id,
            owner.clone(),
            token0,
            token1,
            tick_lower,
            tick_upper,
            liquidity,
        );
        
        self.positions.insert(id, position);
        
        // Track by owner
        self.positions_by_owner
            .entry(owner)
            .or_insert_with(Vec::new)
            .push(id);
        
        id
    }

    /// Get a position by ID
    pub fn get_position(&self, id: PositionId) -> Option<&Position> {
        self.positions.get(&id)
    }

    /// Get a mutable position by ID
    pub fn get_position_mut(&mut self, id: PositionId) -> Option<&mut Position> {
        self.positions.get_mut(&id)
    }

    /// Get all positions for an owner
    pub fn get_positions_by_owner(&self, owner: &str) -> Vec<&Position> {
        self.positions_by_owner
            .get(owner)
            .map(|ids| ids.iter().filter_map(|id| self.positions.get(id)).collect())
            .unwrap_or_default()
    }

    /// Remove a position
    pub fn remove_position(&mut self, id: PositionId) -> Option<Position> {
        if let Some(position) = self.positions.remove(&id) {
            // Remove from owner's list
            if let Some(owner_positions) = self.positions_by_owner.get_mut(&position.owner) {
                owner_positions.retain(|&p| p != id);
            }
            Some(position)
        } else {
            None
        }
    }

    /// Get total number of positions
    pub fn total_positions(&self) -> usize {
        self.positions.len()
    }

    /// Update all positions (for fee collection)
    pub fn update_all_positions(
        &mut self,
        fee_growth_0: &BigUint,
        fee_growth_1: &BigUint,
        current_tick: i32,
        tick_data: &HashMap<i32, (BigUint, BigUint)>,
    ) {
        for position in self.positions.values_mut() {
            let (fee_lower_0, fee_lower_1) = tick_data.get(&position.tick_lower)
                .cloned()
                .unwrap_or((BigUint::from(0u64), BigUint::from(0u64)));
            let (fee_upper_0, fee_upper_1) = tick_data.get(&position.tick_upper)
                .cloned()
                .unwrap_or((BigUint::from(0u64), BigUint::from(0u64)));
            
            position.update_fee_growth(
                fee_growth_0,
                fee_growth_1,
                position.tick_lower,
                position.tick_upper,
                current_tick,
                &fee_lower_0,
                &fee_upper_1,
            );
        }
    }
}

impl Default for PositionManager {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_position_creation() {
        let mut manager = PositionManager::new();
        
        let id = manager.create_position(
            "0xOwner".to_string(),
            "0xToken0".to_string(),
            "0xToken1".to_string(),
            -100,
            100,
            BigUint::from(1000u64),
        );
        
        assert_eq!(id.as_u64(), 1);
        assert_eq!(manager.total_positions(), 1);
    }

    #[test]
    fn test_position_in_range() {
        let position = Position::new(
            PositionId::new(1),
            "0xOwner".to_string(),
            "0xToken0".to_string(),
            "0xToken1".to_string(),
            -100,
            100,
            BigUint::from(1000u64),
        );
        
        assert!(position.is_in_range(0));
        assert!(position.is_in_range(-50));
        assert!(!position.is_in_range(200));
        assert!(!position.is_in_range(-200));
    }

    #[test]
    fn test_collect_fees() {
        let mut position = Position::new(
            PositionId::new(1),
            "0xOwner".to_string(),
            "0xToken0".to_string(),
            "0xToken1".to_string(),
            -100,
            100,
            BigUint::from(1000u64),
        );
        
        position.tokens_owed_0 = BigUint::from(100u64);
        position.tokens_owed_1 = BigUint::from(200u64);
        
        let (owed_0, owed_1) = position.collect();
        
        assert_eq!(owed_0, BigUint::from(100u64));
        assert_eq!(owed_1, BigUint::from(200u64));
        assert_eq!(position.tokens_owed_0, BigUint::from(0u64));
        assert_eq!(position.tokens_owed_1, BigUint::from(0u64));
    }
}
