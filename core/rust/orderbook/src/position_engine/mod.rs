//! Position Engine
//! 
//! Manages trader positions across all markets.

use std::collections::HashMap;
use std::sync::{Arc, RwLock};
use thiserror::Error;

#[derive(Error, Debug)]
pub enum PositionError {
    #[error("Position not found: {0}")]
    PositionNotFound(String),
    #[error("Invalid size: {0}")]
    InvalidSize(String),
}

// ============================================================================
// Types
// ============================================================================

#[derive(Debug, Clone)]
pub struct Position {
    pub position_id: String,
    pub user: String,
    pub market: String,
    pub size: i64,
    pub entry_price: u64,
    pub opens: u64,
}

#[derive(Debug, Clone)]
pub struct PositionUpdate {
    pub position_id: String,
    pub size_delta: i64,
    pub price: u64,
    pub realized_pnl: i64,
}

// ============================================================================
// Position Engine
// ============================================================================

pub struct PositionEngine {
    positions: RwLock<HashMap<String, Position>>,
}

impl PositionEngine {
    pub fn new() -> Self {
        Self {
            positions: RwLock::new(HashMap::new()),
        }
    }
    
    pub fn open_position(&self, user: &str, market: &str, size: i64, price: u64) -> String {
        let id = format!("{}-{}-{}", user, market, current_timestamp());
        let position = Position {
            position_id: id.clone(),
            user: user.to_string(),
            market: market.to_string(),
            size,
            entry_price: price,
            opens: 1,
        };
        self.positions.write().unwrap().insert(id.clone(), position);
        id
    }
    
    pub fn add_to_position(&self, id: &str, size: i64, price: u64) -> Result<PositionUpdate, PositionError> {
        let mut positions = self.positions.write().unwrap();
        let pos = positions.get_mut(id).ok_or_else(|| PositionError::PositionNotFound(id.to_string()))?;
        
        let old_size = pos.size;
        let old_price = pos.entry_price;
        pos.size += size;
        pos.entry_price = price;
        pos.opens += 1;
        
        let realized = ((old_price as i64 - price as i64) * old_size.min(0).abs()) as i64;
        
        Ok(PositionUpdate {
            position_id: id.to_string(),
            size_delta: size,
            price,
            realized_pnl: realized,
        })
    }
    
    pub fn close_position(&self, id: &str, price: u64) -> Result<i64, PositionError> {
        let positions = self.positions.write().unwrap();
        let pos = positions.get(id).ok_or_else(|| PositionError::PositionNotFound(id.to_string()))?;
        
        let pnl = if pos.size > 0 {
            (price as i64 - pos.entry_price as i64) * pos.size
        } else {
            (pos.entry_price as i64 - price as i64) * pos.size.abs()
        };
        
        drop(positions);
        self.positions.write().unwrap().remove(id);
        
        Ok(pnl)
    }
    
    pub fn get_position(&self, id: &str) -> Option<Position> {
        self.positions.read().unwrap().get(id).cloned()
    }
    
    pub fn get_user_positions(&self, user: &str) -> Vec<Position> {
        self.positions.read().unwrap().values().filter(|p| p.user == user).cloned().collect()
    }
}

fn current_timestamp() -> u64 {
    std::time::SystemTime::now().duration_since(std::time::UNIX_EPOCH).unwrap().as_secs()
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_open() {
        let engine = PositionEngine::new();
        let id = engine.open_position("user1", "ETH-USD", 1000, 3000_000000);
        assert!(!id.is_empty());
    }
    
    #[test]
    fn test_close() {
        let engine = PositionEngine::new();
        let id = engine.open_position("user1", "ETH-USD", 1000, 3000_000000);
        let pnl = engine.close_position(&id, 3100_000000).unwrap();
        assert!(pnl > 0);
    }
}