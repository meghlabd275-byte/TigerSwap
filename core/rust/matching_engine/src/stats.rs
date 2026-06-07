//! Stats Module

use serde::{Deserialize, Serialize};

/// Matching stats
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct MatchingStats {
    pub total_orders: u64,
    pub total_trades: u64,
    pub volume_24h: f64,
    pub last_updated: i64,
}

impl MatchingStats {
    pub fn new() -> Self {
        Self {
            total_orders: 0,
            total_trades: 0,
            volume_24h: 0.0,
            last_updated: chrono::Utc::now().timestamp(),
        }
    }
}

impl Default for MatchingStats {
    fn default() -> Self {
        Self::new()
    }
}