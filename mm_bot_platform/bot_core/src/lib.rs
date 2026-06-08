//! TigerSwap Complete Bot Platform
//! 
//! All bot types with full features and role-based access

use std::collections::HashMap;
use std::sync::Arc;
use tokio::sync::RwLock;
use serde::{Deserialize, Serialize};

// ==================== BOT TYPES ====================

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum BotType {
    // Trading Bots
    MarketMaker,
    GridTrading,
    DCA,
    Arbitrage,
    Sniper,
    Momentum,
    MeanReversion,
    TrendFollowing,
    Scalping,
    
    // Social Trading
    CopyTrading,
    SignalTrading,
    
    // Advanced
    AITrading,
    OptionsBot,
    FuturesBot,
    LiquidityBot,
    
    // Utility
    DustCollector,
    Rebalancer,
    AutoCompounder,
    LiquidityManager,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum BotStatus {
    Draft,
    Active,
    Paused,
    Stopped,
    Error,
    Archived,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum BotPricing {
    Free,
    OneTime(u64),
    Subscription { monthly: u64 },
    Performance { fee_percentage: u64 },
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum TradingMode {
    Testnet,
    Mainnet,
}

// ==================== BOT CONFIGURATION ====================

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct BotConfig {
    pub id: String,
    pub bot_type: BotType,
    pub name: String,
    pub description: String,
    pub version: String,
    pub creator: String,
    pub pricing: BotPricing,
    
    // Trading pairs
    pub trading_pairs: Vec<String>,
    pub chains: Vec<String>,
    
    // Risk management
    pub max_position_size: f64,
    pub max_daily_loss: f64,
    pub stop_loss: f64,
    pub take_profit: f64,
    
    // Execution
    pub order_size: f64,
    pub slippage_tolerance: f64,
    pub gas_priority_fee: f64,
    
    // Advanced
    pub use_smart_routing: bool,
    pub use_flash_swap: bool,
    pub enable_mev_protection: bool,
    
    // Status
    pub status: BotStatus,
    pub created_at: u64,
    pub updated_at: u64,
    pub download_count: u32,
    pub rating: f64,
    pub review_count: u32,
}

impl BotConfig {
    pub fn new(bot_type: BotType, name: String, creator: String) -> Self {
        Self {
            id: Self::generate_id(),
            bot_type,
            name,
            description: String::new(),
            version: "1.0.0".to_string(),
            creator,
            pricing: BotPricing::Free,
            trading_pairs: Vec::new(),
            chains: Vec::new(),
            max_position_size: 0.0,
            max_daily_loss: 0.0,
            stop_loss: 0.0,
            take_profit: 0.0,
            order_size: 0.0,
            slippage_tolerance: 0.5,
            gas_priority_fee: 0.0,
            use_smart_routing: true,
            use_flash_swap: false,
            enable_mev_protection: true,
            status: BotStatus::Draft,
            created_at: current_timestamp(),
            updated_at: current_timestamp(),
            download_count: 0,
            rating: 0.0,
            review_count: 0,
        }
    }
    
    fn generate_id() -> String {
        use std::time::{SystemTime, UNIX_EPOCH};
        let ts = SystemTime::now().duration_since(UNIX_EPOCH).unwrap().as_nanos();
        format!("bot_{}", ts)
    }
}

// ==================== MARKET MAKER BOT ====================

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct MarketMakerConfig {
    pub base_spread: f64,
    pub spread_volatility: f64,
    pub order_refresh_time: u64,
    pub min_order_size: f64,
    pub max_order_size: f64,
    pub inventory_target: f64,
    pub inventory_tolerance: f64,
    pub aggressive_pricing: bool,
    pub pricing_calculation: PricingCalculation,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum PricingCalculation {
    Linear,
    Exponential,
    VolatilityBased,
    TWAP,
    VWAP,
}

impl Default for MarketMakerConfig {
    fn default() -> Self {
        Self {
            base_spread: 0.3,
            spread_volatility: 0.1,
            order_refresh_time: 5,
            min_order_size: 10.0,
            max_order_size: 10000.0,
            inventory_target: 0.5,
            inventory_tolerance: 0.1,
            aggressive_pricing: false,
            pricing_calculation: PricingCalculation::Linear,
        }
    }
}

// ==================== GRID TRADING BOT ====================

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct GridTradingConfig {
    pub grid_levels: u32,
    pub grid_size: f64,
    pub upper_bound: f64,
    pub lower_bound: f64,
    pub auto_rebalance: bool,
    pub rebalance_threshold: f64,
    pub grid_type: GridType,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum GridType {
    Arithmetic,
    Geometric,
    Fibonacci,
    Custom,
}

impl Default for GridTradingConfig {
    fn default() -> Self {
        Self {
            grid_levels: 10,
            grid_size: 100.0,
            upper_bound: 0.0,
            lower_bound: 0.0,
            auto_rebalance: true,
            rebalance_threshold: 0.2,
            grid_type: GridType::Arithmetic,
        }
    }
}

// ==================== DCA BOT ====================

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct DCAConfig {
    pub order_size: f64,
    pub interval_seconds: u64,
    pub total_orders: u32,
    pub execution_type: DCAExecution,
    pub base_order_increase: f64,
    pub max_slippage: f64,
    pub take_profit_target: f64,
    pub stop_loss: f64,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum DCAExecution {
    Fixed,
    Geometric,
    Arithmetic,
}

impl Default for DCAConfig {
    fn default() -> Self {
        Self {
            order_size: 10.0,
            interval_seconds: 3600,
            total_orders: 0,
            execution_type: DCAExecution::Fixed,
            base_order_increase: 0.0,
            max_slippage: 1.0,
            take_profit_target: 0.0,
            stop_loss: 0.0,
        }
    }
}

// ==================== ARBITRAGE BOT ====================

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ArbitrageConfig {
    pub min_profit_threshold: f64,
    pub max_investment: f64,
    pub max_slippage: f64,
    pub execution_delay_ms: u64,
    pub route_types: Vec<ArbitrageRoute>,
    pub flash_loan_enabled: bool,
    pub multi_hop_enabled: bool,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum ArbitrageRoute {
    Triangle,
    CrossExchange,
    FlashLoan,
    MultiHop,
}

impl Default for ArbitrageConfig {
    fn default() -> Self {
        Self {
            min_profit_threshold: 0.5,
            max_investment: 50000.0,
            max_slippage: 0.3,
            execution_delay_ms: 100,
            route_types: vec![ArbitrageRoute::Triangle],
            flash_loan_enabled: true,
            multi_hop_enabled: true,
        }
    }
}

// ==================== COPY TRADING BOT ====================

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CopyTradingConfig {
    pub leader_address: String,
    pub copy_ratio: f64,
    pub max_copy_size: f64,
    pub follow_new_trades: bool,
    pub close_on_leader_exit: bool,
    pub auto_compound: bool,
    pub trailing_stop: f64,
}

impl Default for CopyTradingConfig {
    fn default() -> Self {
        Self {
            leader_address: String::new(),
            copy_ratio: 1.0,
            max_copy_size: 10000.0,
            follow_new_trades: true,
            close_on_leader_exit: false,
            auto_compound: false,
            trailing_stop: 0.0,
        }
    }
}

// ==================== SNIPER BOT ====================

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SniperConfig {
    pub target_tokens: Vec<String>,
    pub buy_on_launch: bool,
    pub sell_strategy: SniperStrategy,
    pub max_gas_price: f64,
    pub front_run: bool,
    pub anti_rug: bool,
    pub min_liquidity: f64,
    pub honeypot_check: bool,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum SniperStrategy {
    ImmediateSell,
    TakeProfit,
    Hold,
    Custom,
}

impl Default for SniperConfig {
    fn default() -> Self {
        Self {
            target_tokens: Vec::new(),
            buy_on_launch: true,
            sell_strategy: SniperStrategy::TakeProfit,
            max_gas_price: 100.0,
            front_run: false,
            anti_rug: true,
            min_liquidity: 10000.0,
            honeypot_check: true,
        }
    }
}

// ==================== AI TRADING BOT ====================

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct AITradingConfig {
    pub model_type: AIModel,
    pub prediction_interval: u64,
    pub confidence_threshold: f64,
    pub max_positions: u32,
    pub risk_level: RiskLevel,
    pub training_data_source: String,
    pub rebalance_frequency: u64,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum AIModel {
    LSTM,
    Transformer,
    RandomForest,
    XGBoost,
    Ensemble,
    ReinforcementLearning,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum RiskLevel {
    Conservative,
    Moderate,
    Aggressive,
}

impl Default for AITradingConfig {
    fn default() -> Self {
        Self {
            model_type: AIModel::Ensemble,
            prediction_interval: 3600,
            confidence_threshold: 0.7,
            max_positions: 5,
            risk_level: RiskLevel::Moderate,
            training_data_source: String::new(),
            rebalance_frequency: 86400,
        }
    }
}

// ==================== BOT SUBSCRIPTION ====================

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct BotSubscription {
    pub id: String,
    pub bot_id: String,
    pub user_id: String,
    pub subscription_type: SubscriptionType,
    pub status: SubscriptionStatus,
    pub start_date: u64,
    pub end_date: u64,
    pub price_paid: f64,
    pub auto_renew: bool,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum SubscriptionType {
    Free,
    Monthly,
    Quarterly,
    Yearly,
    Lifetime,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum SubscriptionStatus {
    Active,
    Expired,
    Cancelled,
    Pending,
}

// ==================== BOT STATS ====================

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct BotStats {
    pub bot_id: String,
    pub total_pnl: f64,
    pub daily_pnl: f64,
    pub weekly_pnl: f64,
    pub monthly_pnl: f64,
    pub win_rate: f64,
    pub total_trades: u64,
    pub profitable_trades: u64,
    pub losing_trades: u64,
    pub avg_trade_size: f64,
    pub avg_trade_duration: u64,
    pub max_drawdown: f64,
    pub sharpe_ratio: f64,
    pub last_updated: u64,
}

impl BotStats {
    pub fn new(bot_id: String) -> Self {
        Self {
            bot_id,
            total_pnl: 0.0,
            daily_pnl: 0.0,
            weekly_pnl: 0.0,
            monthly_pnl: 0.0,
            win_rate: 0.0,
            total_trades: 0,
            profitable_trades: 0,
            losing_trades: 0,
            avg_trade_size: 0.0,
            avg_trade_duration: 0,
            max_drawdown: 0.0,
            sharpe_ratio: 0.0,
            last_updated: current_timestamp(),
        }
    }
    
    pub fn calculate_win_rate(&mut self) {
        if self.total_trades > 0 {
            self.win_rate = (self.profitable_trades as f64 / self.total_trades as f64) * 100.0;
        }
    }
}

// ==================== BOT MANAGER ====================

pub struct BotManager {
    // Bot registry
    bots: Arc<RwLock<HashMap<String, BotConfig>>>,
    
    // Bot configurations
    market_maker_configs: Arc<RwLock<HashMap<String, MarketMakerConfig>>>,
    grid_configs: Arc<RwLock<HashMap<String, GridTradingConfig>>>,
    dca_configs: Arc<RwLock<HashMap<String, DCAConfig>>>,
    arbitrage_configs: Arc<RwLock<HashMap<String, ArbitrageConfig>>>,
    copy_configs: Arc<RwLock<HashMap<String, CopyTradingConfig>>>,
    sniper_configs: Arc<RwLock<HashMap<String, SniperConfig>>>,
    ai_configs: Arc<RwLock<HashMap<String, AITradingConfig>>>,
    
    // Subscriptions
    subscriptions: Arc<RwLock<HashMap<String, BotSubscription>>>,
    
    // Stats
    stats: Arc<RwLock<HashMap<String, BotStats>>>,
    
    // User bots
    user_bots: Arc<RwLock<HashMap<String, Vec<String>>>>,
}

impl BotManager {
    pub fn new() -> Self {
        Self {
            bots: Arc::new(RwLock::new(HashMap::new())),
            market_maker_configs: Arc::new(RwLock::new(HashMap::new())),
            grid_configs: Arc::new(RwLock::new(HashMap::new())),
            dca_configs: Arc::new(RwLock::new(HashMap::new())),
            arbitrage_configs: Arc::new(RwLock::new(HashMap::new())),
            copy_configs: Arc::new(RwLock::new(HashMap::new())),
            sniper_configs: Arc::new(RwLock::new(HashMap::new())),
            ai_configs: Arc::new(RwLock::new(HashMap::new())),
            subscriptions: Arc::new(RwLock::new(HashMap::new())),
            stats: Arc::new(RwLock::new(HashMap::new())),
            user_bots: Arc::new(RwLock::new(HashMap::new())),
        }
    }

    // ==================== BOT REGISTRATION ====================
    
    pub async fn register_bot(&self, bot: BotConfig) -> Result<String, BotError> {
        let bot_id = bot.id.clone();
        
        self.bots.write().await.insert(bot_id.clone(), bot);
        
        Ok(bot_id)
    }
    
    pub async fn get_bot(&self, bot_id: &str) -> Option<BotConfig> {
        self.bots.read().await.get(bot_id).cloned()
    }
    
    pub async fn get_all_bots(&self) -> Vec<BotConfig> {
        self.bots.read().await.values().cloned().collect()
    }
    
    pub async fn get_bots_by_type(&self, bot_type: BotType) -> Vec<BotConfig> {
        self.bots.read().await
            .values()
            .filter(|b| b.bot_type == bot_type)
            .cloned()
            .collect()
    }
    
    // ==================== BOT CONFIGURATIONS ====================
    
    pub async fn set_market_maker_config(&self, bot_id: &str, config: MarketMakerConfig) {
        self.market_maker_configs.write().await.insert(bot_id.to_string(), config);
    }
    
    pub async fn set_grid_config(&self, bot_id: &str, config: GridTradingConfig) {
        self.grid_configs.write().await.insert(bot_id.to_string(), config);
    }
    
    pub async fn set_dca_config(&self, bot_id: &str, config: DCAConfig) {
        self.dca_configs.write().await.insert(bot_id.to_string(), config);
    }
    
    pub async fn set_arbitrage_config(&self, bot_id: &str, config: ArbitrageConfig) {
        self.arbitrage_configs.write().await.insert(bot_id.to_string(), config);
    }
    
    pub async fn set_copy_config(&self, bot_id: &str, config: CopyTradingConfig) {
        self.copy_configs.write().await.insert(bot_id.to_string(), config);
    }
    
    pub async fn set_sniper_config(&self, bot_id: &str, config: SniperConfig) {
        self.sniper_configs.write().await.insert(bot_id.to_string(), config);
    }
    
    pub async fn set_ai_config(&self, bot_id: &str, config: AITradingConfig) {
        self.ai_configs.write().await.insert(bot_id.to_string(), config);
    }

    // ==================== SUBSCRIPTIONS ====================
    
    pub async fn create_subscription(
        &self,
        bot_id: String,
        user_id: String,
        subscription_type: SubscriptionType,
        duration_days: u32,
    ) -> Result<BotSubscription, BotError> {
        let subscription = BotSubscription {
            id: Self::generate_subscription_id(),
            bot_id,
            user_id,
            subscription_type,
            status: SubscriptionStatus::Active,
            start_date: current_timestamp(),
            end_date: current_timestamp() + (duration_days as u64 * 86400),
            price_paid: 0.0,
            auto_renew: false,
        };
        
        self.subscriptions.write().await.insert(subscription.id.clone(), subscription.clone());
        
        Ok(subscription)
    }
    
    pub async fn get_user_subscriptions(&self, user_id: &str) -> Vec<BotSubscription> {
        self.subscriptions.read().await
            .values()
            .filter(|s| s.user_id == user_id)
            .cloned()
            .collect()
    }

    // ==================== STATS ====================
    
    pub async fn update_stats(&self, bot_id: &str, stats: BotStats) {
        self.stats.write().await.insert(bot_id.to_string(), stats);
    }
    
    pub async fn get_stats(&self, bot_id: &str) -> Option<BotStats> {
        self.stats.read().await.get(bot_id).cloned()
    }

    // ==================== USER BOTS ====================
    
    pub async fn start_user_bot(&self, user_id: &str, bot_id: &str) -> Result<(), BotError> {
        let mut user_bots = self.user_bots.write().await;
        
        user_bots.entry(user_id.to_string())
            .or_insert_with(Vec::new)
            .push(bot_id.to_string());
        
        Ok(())
    }
    
    pub async fn stop_user_bot(&self, user_id: &str, bot_id: &str) -> Result<(), BotError> {
        let mut user_bots = self.user_bots.write().await;
        
        if let Some(bots) = user_bots.get_mut(user_id) {
            bots.retain(|b| b != bot_id);
        }
        
        Ok(())
    }
    
    pub async fn get_user_bots(&self, user_id: &str) -> Vec<String> {
        self.user_bots.read().await
            .get(user_id)
            .cloned()
            .unwrap_or_default()
    }

    // ==================== HELPERS ====================
    
    fn generate_subscription_id() -> String {
        use std::time::{SystemTime, UNIX_EPOCH};
        let ts = SystemTime::now().duration_since(UNIX_EPOCH).unwrap().as_nanos();
        format!("sub_{}", ts)
    }
}

// ==================== ERRORS ====================

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum BotError {
    BotNotFound,
    InvalidConfig,
    SubscriptionExpired,
    InsufficientFunds,
    TradingError,
    NetworkError,
}

impl std::fmt::Display for BotError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            BotError::BotNotFound => write!(f, "Bot not found"),
            BotError::InvalidConfig => write!(f, "Invalid configuration"),
            BotError::SubscriptionExpired => write!(f, "Subscription expired"),
            BotError::InsufficientFunds => write!(f, "Insufficient funds"),
            BotError::TradingError => write!(f, "Trading error"),
            BotError::NetworkError => write!(f, "Network error"),
        }
    }
}

fn current_timestamp() -> u64 {
    std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .unwrap()
        .as_secs()
}