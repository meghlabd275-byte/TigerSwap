//! Bot Module

use serde::{Deserialize, Serialize};

/// Bot type
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum BotType {
    Grid,
    MarketMaking,
    Arbitrage,
    Sniper,
}

impl BotType {
    pub fn to_string(&self) -> &'static str {
        match self {
            BotType::Grid => "grid",
            BotType::MarketMaking => "market_making",
            BotType::Arbitrage => "arbitrage",
            BotType::Sniper => "sniper",
        }
    }
}

/// Bot status
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum BotStatus {
    Stopped,
    Running,
    Paused,
    Error,
}

/// Bot info
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct BotInfo {
    pub bot_id: String,
    pub bot_type: BotType,
    pub status: BotStatus,
    pub pnl: f64,
}

/// Bot manager
pub struct BotManager {
    bots: std::collections::HashMap<String, BotInfo>,
}

impl BotManager {
    pub fn new() -> Self {
        Self {
            bots: std::collections::HashMap::new(),
        }
    }
    
    pub fn register(&mut self, info: BotInfo) {
        self.bots.insert(info.bot_id.clone(), info);
    }
    
    pub fn list(&self) -> Vec<BotInfo> {
        self.bots.values().cloned().collect()
    }
}

impl Default for BotManager {
    fn default() -> Self {
        Self::new()
    }
}