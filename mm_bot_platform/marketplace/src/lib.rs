//! TigerSwap Bot Marketplace
//! 
//! Trading bot marketplace with:
//! - Bot registration and discovery
//! - Strategy templates
//! - Grid trading
//! - DCA (Dollar Cost Averaging)
//! - Arbitrage detection
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

// ==================== BOT TYPES ====================

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum BotType {
    Grid,
    DCA,
    Arbitrage,
    Sniper,
    Momentum,
    MeanReversion,
    Custom,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum BotStatus {
    Draft,
    Active,
    Paused,
    Stopped,
    Completed,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum BotPricing {
    Free,
    OneTime(u64),  // Price in USD
    Subscription(u64),  // Monthly price
    Performance(u64), // % of profits
}

// ==================== BOT STRUCTURE ====================

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Bot {
    pub id: [u8; 32],
    pub creator: [u8; 20],
    pub name: String,
    pub description: String,
    pub bot_type: BotType,
    pub version: String,
    pub parameters: Vec<BotParameter>,
    pub pricing: BotPricing,
    pub chain_ids: Vec<u64>,
    pub status: BotStatus,
    pub downloads: u32,
    pub rating: u8,
    pub review_count: u32,
    pub created_at: u64,
    pub updated_at: u64,
    pub code_hash: [u8; 32],
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct BotParameter {
    pub name: String,
    pub param_type: String,
    pub default_value: String,
    pub min_value: Option<String>,
    pub max_value: Option<String>,
    pub required: bool,
    pub description: String,
}

impl Bot {
    pub fn new(
        creator: [u8; 20],
        name: String,
        description: String,
        bot_type: BotType,
        version: String,
    ) -> Self {
        let mut id = [0u8; 32];
        id[..20].copy_from_slice(&creator);
        id[20..].copy_from_slice(&current_timestamp().to_le_bytes()[..12]);
        
        Self {
            id,
            creator,
            name,
            description,
            bot_type,
            version,
            parameters: Vec::new(),
            pricing: BotPricing::Free,
            chain_ids: vec![1], // Ethereum
            status: BotStatus::Draft,
            downloads: 0,
            rating: 0,
            review_count: 0,
            created_at: current_timestamp(),
            updated_at: current_timestamp(),
            code_hash: [0u8; 32],
        }
    }
    
    pub fn add_parameter(&mut self, param: BotParameter) {
        self.parameters.push(param);
        self.updated_at = current_timestamp();
    }
    
    pub fn set_pricing(&mut self, pricing: BotPricing) {
        self.pricing = pricing;
        self.updated_at = current_timestamp();
    }
    
    pub fn publish(&mut self) {
        self.status = BotStatus::Active;
        self.updated_at = current_timestamp();
    }
    
    pub fn increment_downloads(&mut self) {
        self.downloads += 1;
    }
    
    pub fn rate(&mut self, rating: u8) {
        if rating > 5 {
            return;
        }
        
        let total = self.rating as u32 * self.review_count;
        self.review_count += 1;
        self.rating = ((total + rating as u32) / self.review_count) as u8;
    }
}

// ==================== GRID BOT ====================

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct GridBot {
    pub bot_id: [u8; 32],
    pub user: [u8; 20],
    pub pair: [u8; 20],
    pub grid_levels: u32,
    pub grid_size: U256,
    pub upper_bound: U256,
    pub lower_bound: U256,
    pub active_orders: u32,
    pub total_profit: U256,
    pub status: BotStatus,
}

impl GridBot {
    pub fn new(
        user: [u8; 20],
        pair: [u8; 20],
        grid_levels: u32,
        grid_size: U256,
        upper_bound: U256,
        lower_bound: U256,
    ) -> Result<Self, BotError> {
        if upper_bound <= lower_bound {
            return Err(BotError::InvalidParameters);
        }
        
        if grid_levels == 0 || grid_levels > 100 {
            return Err(BotError::InvalidParameters);
        }
        
        let mut id = [0u8; 32];
        id[..20].copy_from_slice(&user);
        
        Ok(Self {
            bot_id: id,
            user,
            pair,
            grid_levels,
            grid_size,
            upper_bound,
            lower_bound,
            active_orders: 0,
            total_profit: U256::zero(),
            status: BotStatus::Active,
        })
    }
    
    pub fn calculate_grid_levels(&self) -> Vec<U256> {
        let mut levels = Vec::new();
        let step = (self.upper_bound - self.lower_bound) / U256::from(self.grid_levels);
        
        for i in 0..self.grid_levels {
            levels.push(self.lower_bound + (step * U256::from(i)));
        }
        
        levels
    }
    
    pub fn execute_grid_trade(&mut self, current_price: U256) -> Option<(U256, U256)> {
        // Find which grid level we're at
        let levels = self.calculate_grid_levels();
        
        for (i, level) in levels.iter().enumerate() {
            if current_price <= *level {
                // Execute buy at this level
                self.active_orders += 1;
                self.total_profit = self.total_profit + self.grid_size;
                
                return Some((self.grid_size, *level));
            }
        }
        
        None
    }
    
    pub fn stop(&mut self) {
        self.status = BotStatus::Stopped;
    }
}

// ==================== DCA BOT ====================

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct DCABot {
    pub bot_id: [u8; 32],
    pub user: [u8; 20],
    pub pair: [u8; 20],
    pub order_size: U256,
    pub interval_seconds: u64,
    pub total_orders: u32,
    pub executed_orders: u32,
    pub next_execution: u64,
    pub total_invested: U256,
    pub average_price: U256,
    pub status: BotStatus,
}

impl DCABot {
    pub fn new(
        user: [u8; 20],
        pair: [u8; 20],
        order_size: U256,
        interval_seconds: u64,
    ) -> Result<Self, BotError> {
        if order_size == U256::zero() {
            return Err(BotError::InvalidParameters);
        }
        
        if interval_seconds < 60 {
            return Err(BotError::InvalidParameters);
        }
        
        let mut id = [0u8; 32];
        id[..20].copy_from_slice(&user);
        
        Ok(Self {
            bot_id: id,
            user,
            pair,
            order_size,
            interval_seconds,
            total_orders: 0,
            executed_orders: 0,
            next_execution: current_timestamp(),
            total_invested: U256::zero(),
            average_price: U256::zero(),
            status: BotStatus::Active,
        })
    }
    
    pub fn execute_order(&mut self, price: U256) -> Result<(), BotError> {
        if self.status != BotStatus::Active {
            return Err(BotError::BotNotActive);
        }
        
        if current_timestamp() < self.next_execution {
            return Err(BotError::TooEarly);
        }
        
        self.executed_orders += 1;
        self.total_invested = self.total_invested + self.order_size;
        
        // Update average price
        let total_bought = self.order_size / price;
        let old_total = self.average_price * U256::from(self.executed_orders - 1);
        self.average_price = (old_total + total_bought) / U256::from(self.executed_orders);
        
        // Schedule next execution
        self.next_execution = current_timestamp() + self.interval_seconds;
        
        Ok(())
    }
    
    pub fn pause(&mut self) {
        self.status = BotStatus::Paused;
    }
    
    pub fn resume(&mut self) {
        if self.status == BotStatus::Paused {
            self.status = BotStatus::Active;
            self.next_execution = current_timestamp();
        }
    }
    
    pub fn stop(&mut self) {
        self.status = BotStatus::Stopped;
    }
}

// ==================== ARBITRAGE BOT ====================

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ArbitrageOpportunity {
    pub id: [u8; 32],
    pub pool_a: [u8; 20],
    pub pool_b: [u8; 20],
    pub token_in: [u8; 20],
    pub amount_in: U256,
    pub expected_profit: U256,
    pub profit_margin_bps: u64,
    pub detected_at: u64,
    pub expires_at: u64,
    pub executed: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ArbitrageBot {
    pub bot_id: [u8; 32],
    pub user: [u8; 20],
    pub min_profit_bps: u64,
    pub max_investment: U256,
    pub min_liquidity: U256,
    pub opportunities_found: u32,
    pub total_profit: U256,
    pub status: BotStatus,
}

impl ArbitrageBot {
    pub fn new(
        user: [u8; 20],
        min_profit_bps: u64,
        max_investment: U256,
        min_liquidity: U256,
    ) -> Self {
        let mut id = [0u8; 32];
        id[..20].copy_from_slice(&user);
        
        Self {
            bot_id: id,
            user,
            min_profit_bps,
            max_investment,
            min_liquidity,
            opportunities_found: 0,
            total_profit: U256::zero(),
            status: BotStatus::Active,
        }
    }
    
    pub fn detect_arbitrage(
        &self,
        price_a: U256,
        price_b: U256,
        pool_a_liquidity: U256,
        pool_b_liquidity: U256,
    ) -> Option<ArbitrageOpportunity> {
        // Check liquidity
        if pool_a_liquidity < self.min_liquidity || pool_b_liquidity < self.min_liquidity {
            return None;
        }
        
        // Calculate profit margin
        let diff = if price_a > price_b { price_a - price_b } else { price_b - price_a };
        let profit_bps = (diff * U256::from(10000)) / price_a;
        
        // Check minimum profit
        if profit_bps < self.min_profit_bps {
            return None;
        }
        
        // Calculate potential profit
        let investment = self.max_investment.min(pool_a_liquidity).min(pool_b_liquidity);
        let profit = (diff * investment) / price_a;
        
        if profit == U256::zero() {
            return None;
        }
        
        let mut id = [0u8; 32];
        let ts = current_timestamp();
        id[..8].copy_from_slice(&ts.to_le_bytes());
        
        Some(ArbitrageOpportunity {
            id,
            pool_a: [0u8; 20],
            pool_b: [0u8; 20],
            token_in: [0u8; 20],
            amount_in: investment,
            expected_profit: profit,
            profit_margin_bps: profit_bps.as_u64() as u64,
            detected_at: ts,
            expires_at: ts + 30, // 30 seconds expiry
            executed: false,
        })
    }
    
    pub fn stop(&mut self) {
        self.status = BotStatus::Stopped;
    }
}

// ==================== MARKETPLACE ====================

pub struct BotMarketplace {
    bots: Arc<RwLock<HashMap<[u8; 32], Bot>>>,
    user_bots: Arc<RwLock<HashMap<[u8; 20], Vec<[u8; 32]>>>,
    grid_bots: Arc<RwLock<HashMap<[u8; 32], GridBot>>>,
    dca_bots: Arc<RwLock<HashMap<[u8; 32], DCABot>>>,
    arbitrage_bots: Arc<RwLock<HashMap<[u8; 32], ArbitrageBot>>>,
}

impl BotMarketplace {
    pub fn new() -> Self {
        Self {
            bots: Arc::new(RwLock::new(HashMap::new())),
            user_bots: Arc::new(RwLock::new(HashMap::new())),
            grid_bots: Arc::new(RwLock::new(HashMap::new())),
            dca_bots: Arc::new(RwLock::new(HashMap::new())),
            arbitrage_bots: Arc::new(RwLock::new(HashMap::new())),
        }
    }
    
    // ==================== BOT REGISTRATION ====================
    
    pub async fn register_bot(&self, bot: Bot) -> Result<[u8; 32], BotError> {
        let bot_id = bot.id;
        
        self.bots.write().await.insert(bot_id, bot.clone());
        
        // Index by creator
        self.user_bots.write().await
            .entry(bot.creator)
            .or_insert_with(Vec::new)
            .push(bot_id);
        
        Ok(bot_id)
    }
    
    pub async fn update_bot(&self, bot_id: &[u8; 32], bot: Bot) -> Result<(), BotError> {
        let mut bots = self.bots.write().await;
        
        if bots.contains_key(bot_id) {
            bots.insert(*bot_id, bot);
            Ok(())
        } else {
            Err(BotError::BotNotFound)
        }
    }
    
    // ==================== GRID BOT MANAGEMENT ====================
    
    pub async fn create_grid_bot(&self, grid: GridBot) -> Result<[u8; 32], BotError> {
        let bot_id = grid.bot_id;
        
        self.grid_bots.write().await.insert(bot_id, grid);
        
        Ok(bot_id)
    }
    
    pub async fn get_grid_bot(&self, bot_id: &[u8; 32]) -> Option<GridBot> {
        let grid_bots = self.grid_bots.read().await;
        grid_bots.get(bot_id).cloned()
    }
    
    // ==================== DCA BOT MANAGEMENT ====================
    
    pub async fn create_dca_bot(&self, dca: DCABot) -> Result<[u8; 32], BotError> {
        let bot_id = dca.bot_id;
        
        self.dca_bots.write().await.insert(bot_id, dca);
        
        Ok(bot_id)
    }
    
    pub async fn get_dca_bot(&self, bot_id: &[u8; 32]) -> Option<DCABot> {
        let dca_bots = self.dca_bots.read().await;
        dca_bots.get(bot_id).cloned()
    }
    
    // ==================== QUERIES ====================
    
    pub async fn get_all_bots(&self) -> Vec<Bot> {
        let bots = self.bots.read().await;
        bots.values().cloned().collect()
    }
    
    pub async fn get_bots_by_type(&self, bot_type: BotType) -> Vec<Bot> {
        let bots = self.bots.read().await;
        
        bots.values()
            .filter(|b| b.bot_type == bot_type && b.status == BotStatus::Active)
            .cloned()
            .collect()
    }
    
    pub async fn get_bots_by_creator(&self, creator: &[u8; 20]) -> Vec<Bot> {
        let user_bots = self.user_bots.read().await;
        let bots = self.bots.read().await;
        
        if let Some(bot_ids) = user_bots.get(creator) {
            bot_ids.iter()
                .filter_map(|id| bots.get(id).cloned())
                .collect()
        } else {
            Vec::new()
        }
    }
    
    pub async fn get_user_grid_bots(&self, user: &[u8; 20]) -> Vec<GridBot> {
        let grid_bots = self.grid_bots.read().await;
        
        grid_bots.values()
            .filter(|b| b.user == *user)
            .cloned()
            .collect()
    }
    
    pub async fn get_user_dca_bots(&self, user: &[u8; 20]) -> Vec<DCABot> {
        let dca_bots = self.dca_bots.read().await;
        
        dca_bots.values()
            .filter(|b| b.user == *user)
            .cloned()
            .collect()
    }
}

// ==================== ERRORS ====================

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum BotError {
    BotNotFound,
    InvalidParameters,
    BotNotActive,
    TooEarly,
    InsufficientFunds,
}

impl std::fmt::Display for BotError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            BotError::BotNotFound => write!(f, "Bot not found"),
            BotError::InvalidParameters => write!(f, "Invalid parameters"),
            BotError::BotNotActive => write!(f, "Bot not active"),
            BotError::TooEarly => write!(f, "Too early to execute"),
            BotError::InsufficientFunds => write!(f, "Insufficient funds"),
        }
    }
}

// ==================== HELPER ====================

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
    
    pub type MarketplaceHandle = Arc<BotMarketplace>;
    
    pub fn create_marketplace() -> MarketplaceHandle {
        Arc::new(BotMarketplace::new())
    }
}

// ==================== TESTS ====================

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_grid_bot_creation() {
        let user = [0u8; 20];
        let pair = [0u8; 20];
        
        let grid = GridBot::new(
            user,
            pair,
            10,
            U256::from(1000),
            U256::from(2000),
            U256::from(1000),
        );
        
        assert!(grid.is_ok());
    }
    
    #[test]
    fn test_dca_bot_creation() {
        let user = [0u8; 20];
        let pair = [0u8; 20];
        
        let dca = DCABot::new(
            user,
            pair,
            U256::from(100),
            86400, // Daily
        );
        
        assert!(dca.is_ok());
    }
}