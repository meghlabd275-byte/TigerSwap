//! Tick management for concentrated liquidity

use serde::{Deserialize, Serialize};
use std::collections::{BTreeMap, HashMap};
use num_bigint::BigUint;

/// Tick data structure
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Tick {
    pub index: i32,
    pub liquidity_net: BigUint,
    pub liquidity_gross: BigUint,
    pub fee_growth_outside_0: BigUint,
    pub fee_growth_outside_1: BigUint,
    pub tick_cumulative_outside: i64,
    pub seconds_per_liquidity_outside: BigUint,
    pub seconds_outside: u64,
    pub initialized: bool,
}

impl Tick {
    pub fn new(index: i32) -> Self {
        Self {
            index,
            liquidity_net: BigUint::from(0u64),
            liquidity_gross: BigUint::from(0u64),
            fee_growth_outside_0: BigUint::from(0u64),
            fee_growth_outside_1: BigUint::from(0u64),
            tick_cumulative_outside: 0,
            seconds_per_liquidity_outside: BigUint::from(0u64),
            seconds_outside: 0,
            initialized: false,
        }
    }

    /// Update tick with new liquidity
    pub fn update(
        &mut self,
        liquidity_delta: &BigUint,
        fee_growth_0: &BigUint,
        fee_growth_1: &BigUint,
        current_tick: i32,
        seconds_per_liquidity: &BigUint,
        seconds: u64,
    ) -> bool {
        let mut liquidity_net_before = self.liquidity_net.clone();
        
        // Update fee growth
        if self.index < current_tick {
            self.fee_growth_outside_0 += fee_growth_0.clone();
            self.fee_growth_outside_1 += fee_growth_1.clone();
        }
        
        // Update liquidity net
        if liquidity_delta > &BigUint::from(0u64) {
            self.liquidity_net += liquidity_delta.clone();
        } else {
            self.liquidity_net -= liquidity_delta.clone();
        }
        
        // Update gross liquidity
        if liquidity_delta > &BigUint::from(0u64) {
            self.liquidity_gross += liquidity_delta.clone();
        } else {
            self.liquidity_gross -= liquidity_delta.clone();
        }
        
        // Update time-based values
        self.seconds_per_liquidity_outside += seconds_per_liquidity.clone();
        self.seconds_outside = seconds;
        
        // Mark as initialized if it has liquidity
        if self.liquidity_gross > BigUint::from(0u64) && !self.initialized {
            self.initialized = true;
        }
        
        // Return true if liquidity became zero (for crossing)
        self.liquidity_gross == BigUint::from(0u64)
    }

    /// Cross tick (when price moves through this tick)
    pub fn cross(
        &mut self,
        fee_growth_0: &BigUint,
        fee_growth_1: &BigUint,
        seconds_per_liquidity: &BigUint,
        seconds: u64,
    ) {
        self.fee_growth_outside_0 = fee_growth_0.clone() - self.fee_growth_outside_0.clone();
        self.fee_growth_outside_1 = fee_growth_1.clone() - self.fee_growth_outside_1.clone();
        self.seconds_per_liquidity_outside = seconds_per_liquidity.clone() - self.seconds_per_liquidity_outside;
        self.seconds_outside = seconds - self.seconds_outside;
        self.tick_cumulative_outside = -self.tick_cumulative_outside;
    }
}

/// Tick map for efficient tick lookups
pub struct TickMap {
    ticks: BTreeMap<i32, Tick>,
    tick_data: HashMap<i32, Tick>,
}

impl TickMap {
    pub fn new() -> Self {
        Self {
            ticks: BTreeMap::new(),
            tick_data: HashMap::new(),
        }
    }

    /// Insert or update a tick
    pub fn insert(&mut self, tick: Tick) {
        let index = tick.index;
        self.ticks.insert(index, tick.clone());
        self.tick_data.insert(index, tick);
    }

    /// Get a tick by index
    pub fn get(&self, index: i32) -> Option<&Tick> {
        self.tick_data.get(&index)
    }

    /// Get a mutable tick by index
    pub fn get_mut(&mut self, index: i32) -> Option<&mut Tick> {
        self.tick_data.get_mut(&index)
    }

    /// Check if a tick is initialized
    pub fn is_initialized(&self, index: i32) -> bool {
        self.tick_data.get(&index)
            .map(|t| t.initialized)
            .unwrap_or(false)
    }

    /// Get all ticks in a range
    pub fn get_ticks_in_range(&self, tick_low: i32, tick_high: i32) -> Vec<&Tick> {
        self.ticks.range(tick_low..=tick_high)
            .filter(|(_, t)| t.initialized)
            .map(|(_, t)| t)
            .collect()
    }

    /// Get the next initialized tick at or above a value
    pub fn next_initialized_tick(&self, tick: i32) -> Option<i32> {
        self.ticks.range(tick..)
            .find(|(_, t)| t.initialized)
            .map(|(idx, _)| *idx)
    }

    /// Get the previous initialized tick at or below a value
    pub fn prev_initialized_tick(&self, tick: i32) -> Option<i32> {
        self.ticks.range(..=tick)
            .filter(|(_, t)| t.initialized)
            .last()
            .map(|(idx, _)| *idx)
    }

    /// Calculate gross liquidity between two ticks
    pub fn get_gross_liquidity(&self, tick_low: i32, tick_high: i32) -> BigUint {
        let mut total = BigUint::from(0u64);
        for tick in self.ticks.range(tick_low..=tick_high) {
            total += tick.1.liquidity_gross.clone();
        }
        total
    }

    /// Get all initialized ticks
    pub fn get_all_initialized_ticks(&self) -> Vec<i32> {
        self.ticks.iter()
            .filter(|(_, t)| t.initialized)
            .map(|(idx, _)| *idx)
            .collect()
    }

    /// Remove a tick
    pub fn remove(&mut self, index: i32) {
        self.ticks.remove(&index);
        self.tick_data.remove(&index);
    }

    /// Get number of ticks
    pub fn len(&self) -> usize {
        self.ticks.len()
    }

    /// Check if empty
    pub fn is_empty(&self) -> bool {
        self.ticks.is_empty()
    }
}

impl Default for TickMap {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_tick_creation() {
        let tick = Tick::new(100);
        assert_eq!(tick.index, 100);
        assert!(!tick.initialized);
    }

    #[test]
    fn test_tick_update() {
        let mut tick = Tick::new(100);
        let liquidity_delta = BigUint::from(1000u64);
        let fee_growth = BigUint::from(100u64);
        let seconds_per_liq = BigUint::from(1000u64);
        
        let became_zero = tick.update(
            &liquidity_delta,
            &fee_growth,
            &fee_growth,
            50, // Current tick below
            &seconds_per_liq,
            1000,
        );
        
        assert!(tick.initialized);
        assert!(!became_zero);
    }

    #[test]
    fn test_tick_map() {
        let mut map = TickMap::new();
        
        let tick = Tick::new(100);
        map.insert(tick);
        
        assert!(map.is_initialized(100));
        assert!(!map.is_initialized(200));
    }

    #[test]
    fn test_next_tick() {
        let mut map = TickMap::new();
        
        map.insert(Tick::new(100));
        map.insert(Tick::new(200));
        map.insert(Tick::new(300));
        
        assert_eq!(map.next_initialized_tick(150), Some(200));
        assert_eq!(map.next_initialized_tick(100), Some(100));
        assert_eq!(map.next_initialized_tick(350), None);
    }

    #[test]
    fn test_prev_tick() {
        let mut map = TickMap::new();
        
        map.insert(Tick::new(100));
        map.insert(Tick::new(200));
        map.insert(Tick::new(300));
        
        assert_eq!(map.prev_initialized_tick(250), Some(200));
        assert_eq!(map.prev_initialized_tick(100), Some(100));
        assert_eq!(map.prev_initialized_tick(50), None);
    }
}
