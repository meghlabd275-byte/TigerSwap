//! TigerSwap Limit Order Engine
//! 
//! On-chain limit orders with support for:
//! - Standard limit orders (fill at or better than limit price)
//! - Stop-loss orders (trigger when price crosses threshold)
//! - Take-profit orders (exit when profit target reached)
//! - Trailing stop orders (dynamic stop that follows price)
//! - OCO (One-Cancels-Other) orders
//! - Good-Till-Date (GTD) orders
//! - IOC (Immediate-or-Cancel) orders
//! - FOK (Fill-or-Kill) orders
//!
//! Implementation: Pure Rust with no external dependencies

use serde::{Deserialize, Serialize};
use std::sync::Arc;
use parking_lot::RwLock;
use rust_decimal::Decimal;
use thiserror::Error;
use uuid::Uuid;
use chrono::{Utc, DateTime};
use std::collections::HashMap;
use std::cmp::Ordering;

/// Chain ID constants
pub const CHAIN_ETH: u64 = 1;
pub const CHAIN_BSC: u64 = 56;
pub const CHAIN_POLYGON: u64 = 137;
pub const CHAIN_ARBITRUM: u64 = 42161;
pub const CHAIN_OPTIMISM: u64 = 10;
pub const CHAIN_BASE: u64 = 8453;
pub const CHAIN_AVALANCHE: u64 = 43114;
pub const CHAIN_SOLANA: u64 = 0;
pub const CHAIN_APTOS: u64 = 43120;
pub const CHAIN_SUI: u64 = 784;

#[derive(Debug, Error)]
pub enum LimitOrderError {
    #[error("Order not found: {0}")]
    OrderNotFound(String),
    #[error("Order expired: {0}")]
    OrderExpired(String),
    #[error("Order cancelled: {0}")]
    OrderCancelled(String),
    #[error("Insufficient balance: {0}")]
    InsufficientBalance(String),
    #[error("Price condition not met: expected {expected}, got {actual}")]
    PriceConditionNotMet { expected: Decimal, actual: Decimal },
    #[error("Invalid order parameters: {0}")]
    InvalidParameters(String),
    #[error("Chain not supported: {0}")]
    ChainNotSupported(u64),
    #[error("Insufficient liquidity: {0}")]
    InsufficientLiquidity(String),
    #[error("Partial fill not allowed")]
    PartialFillNotAllowed,
    #[error("Stop trigger not reached: {0}")]
    StopTriggerNotReached(String),
}

/// Order side
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum OrderSide {
    Buy,
    Sell,
}

impl OrderSide {
    pub fn is_buy(&self) -> bool { matches!(self, OrderSide::Buy) }
    pub fn is_sell(&self) -> bool { matches!(self, OrderSide::Sell) }
}

/// Order type
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum OrderType {
    Limit,
    StopLoss,
    TakeProfit,
    TrailingStop,
    OCO,
    GTD,
    IOC,
    FOK,
}

impl Default for OrderType {
    fn default() -> Self { OrderType::Limit }
}

impl OrderType {
    pub fn requires_trigger(&self) -> bool {
        matches!(self, OrderType::StopLoss | OrderType::TakeProfit | OrderType::TrailingStop)
    }
}

/// Order status
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum OrderStatus {
    Pending,
    Active,
    PartiallyFilled,
    Filled,
    Cancelled,
    Expired,
    Triggered,
}

impl Default for OrderStatus {
    fn default() -> Self { OrderStatus::Pending }
}

/// Time in force
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum TimeInForce {
    GTC,  // Good-Till-Cancel
    GTD,   // Good-Till-Date
    IOC,   // Immediate-or-Cancel
    FOK,   // Fill-or-Kill
}

impl Default for TimeInForce {
    fn default() -> Self { TimeInForce::GTC }
}

/// Trailing stop configuration
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct TrailingStopConfig {
    pub trail_type: TrailType,
    pub trail_value_bps: i64,  // Percentage to trail (e.g., 100 = 1%)
    pub activation_price: Decimal,  // Price to activate trailing stop
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum TrailType {
    Percentage,  // Trail by percentage
    Absolute,    // Trail by absolute amount
}

impl Default for TrailingStopConfig {
    fn default() -> Self {
        Self {
            trail_type: TrailType::Percentage,
            trail_value_bps: 100,
            activation_price: Decimal::ZERO,
        }
    }
}

/// OCO (One-Cancels-Other) order configuration
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct OCOConfig {
    pub order_a: String,  // First order ID
    pub order_b: String,  // Second order ID
    pub trigger_first: bool,  // If true, first triggered order cancels the other
}

impl Default for OCOConfig {
    fn default() -> Self {
        Self {
            order_a: String::new(),
            order_b: String::new(),
            trigger_first: true,
        }
    }
}

/// Price condition for stop/take-profit orders
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PriceCondition {
    pub trigger_price: Decimal,
    pub condition: TriggerCondition,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum TriggerCondition {
    GreaterThanOrEqual,  // Trigger when price >= trigger
    LessThanOrEqual,     // Trigger when price <= trigger
    CrossingAbove,       // Trigger when price crosses above
    CrossingBelow,       // Trigger when price crosses below
}

/// Limit order definition
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct LimitOrder {
    pub order_id: String,
    pub user: String,
    pub chain_id: u64,
    pub token_in: String,
    pub token_out: String,
    pub side: OrderSide,
    pub order_type: OrderType,
    pub amount_in: u128,
    pub amount_filled: u128,
    pub limit_price: Decimal,
    pub stop_price: Option<Decimal>,
    pub take_profit_price: Option<Decimal>,
    pub price_condition: Option<PriceCondition>,
    pub trailing_stop: Option<TrailingStopConfig>,
    pub oco: Option<OCOConfig>,
    pub time_in_force: TimeInForce,
    pub expire_at: Option<i64>,
    pub status: OrderStatus,
    pub created_at: i64,
    pub updated_at: i64,
    pub filled_at: Option<i64>,
    pub referrer: Option<String>,
    pub fee_bps: i64,  // Fee in basis points
}

impl LimitOrder {
    /// Create a new limit order
    pub fn new(
        user: String,
        chain_id: u64,
        token_in: String,
        token_out: String,
        side: OrderSide,
        amount_in: u128,
        limit_price: Decimal,
    ) -> Self {
        let now = Utc::now().timestamp();
        Self {
            order_id: Uuid::new_v4().to_string(),
            user,
            chain_id,
            token_in,
            token_out,
            side,
            order_type: OrderType::Limit,
            amount_in,
            amount_filled: 0,
            limit_price,
            stop_price: None,
            take_profit_price: None,
            price_condition: None,
            trailing_stop: None,
            oco: None,
            time_in_force: TimeInForce::GTC,
            expire_at: None,
            status: OrderStatus::Pending,
            created_at: now,
            updated_at: now,
            filled_at: None,
            referrer: None,
            fee_bps: 0,
        }
    }

    /// Create a stop-loss order
    pub fn new_stop_loss(
        user: String,
        chain_id: u64,
        token_in: String,
        token_out: String,
        side: OrderSide,
        amount_in: u128,
        stop_price: Decimal,
    ) -> Self {
        let mut order = Self::new(user, chain_id, token_in, token_out, side, amount_in, Decimal::ZERO);
        order.order_type = OrderType::StopLoss;
        order.stop_price = Some(stop_price);
        order.price_condition = Some(PriceCondition {
            trigger_price: stop_price,
            condition: if side.is_sell() { TriggerCondition::LessThanOrEqual } else { TriggerCondition::GreaterThanOrEqual },
        });
        order
    }

    /// Create a take-profit order
    pub fn new_take_profit(
        user: String,
        chain_id: u64,
        token_in: String,
        token_out: String,
        side: OrderSide,
        amount_in: u128,
        take_profit_price: Decimal,
    ) -> Self {
        let mut order = Self::new(user, chain_id, token_in, token_out, side, amount_in, Decimal::ZERO);
        order.order_type = OrderType::TakeProfit;
        order.take_profit_price = Some(take_profit_price);
        order.price_condition = Some(PriceCondition {
            trigger_price: take_profit_price,
            condition: if side.is_sell() { TriggerCondition::GreaterThanOrEqual } else { TriggerCondition::LessThanOrEqual },
        });
        order
    }

    /// Create a trailing stop order
    pub fn new_trailing_stop(
        user: String,
        chain_id: u64,
        token_in: String,
        token_out: String,
        side: OrderSide,
        amount_in: u128,
        trail_value_bps: i64,
        activation_price: Decimal,
    ) -> Self {
        let mut order = Self::new(user, chain_id, token_in, token_out, side, amount_in, Decimal::ZERO);
        order.order_type = OrderType::TrailingStop;
        order.trailing_stop = Some(TrailingStopConfig {
            trail_type: TrailType::Percentage,
            trail_value_bps,
            activation_price,
        });
        order
    }

    /// Create a GTD (Good-Till-Date) order
    pub fn new_gtd(
        user: String,
        chain_id: u64,
        token_in: String,
        token_out: String,
        side: OrderSide,
        amount_in: u128,
        limit_price: Decimal,
        expire_at: i64,
    ) -> Self {
        let mut order = Self::new(user, chain_id, token_in, token_out, side, amount_in, limit_price);
        order.order_type = OrderType::GTD;
        order.time_in_force = TimeInForce::GTD;
        order.expire_at = Some(expire_at);
        order
    }

    /// Create an IOC (Immediate-or-Cancel) order
    pub fn new_ioc(
        user: String,
        chain_id: u64,
        token_in: String,
        token_out: String,
        side: OrderSide,
        amount_in: u128,
        limit_price: Decimal,
    ) -> Self {
        let mut order = Self::new(user, chain_id, token_in, token_out, side, amount_in, limit_price);
        order.order_type = OrderType::IOC;
        order.time_in_force = TimeInForce::IOC;
        order
    }

    /// Create an FOK (Fill-or-Kill) order
    pub fn new_fok(
        user: String,
        chain_id: u64,
        token_in: String,
        token_out: String,
        side: OrderSide,
        amount_in: u128,
        limit_price: Decimal,
    ) -> Self {
        let mut order = Self::new(user, chain_id, token_in, token_out, side, amount_in, limit_price);
        order.order_type = OrderType::FOK;
        order.time_in_force = TimeInForce::FOK;
        order
    }

    /// Validate order parameters
    pub fn validate(&self) -> Result<(), LimitOrderError> {
        if self.amount_in == 0 {
            return Err(LimitOrderError::InvalidParameters("Amount must be greater than 0".to_string()));
        }
        if self.token_in == self.token_out {
            return Err(LimitOrderError::InvalidParameters("Token pair must be different".to_string()));
        }
        if self.limit_price <= Decimal::ZERO && !self.order_type.requires_trigger() {
            return Err(LimitOrderError::InvalidParameters("Limit price must be positive".to_string()));
        }
        if let Some(expire_at) = self.expire_at {
            if expire_at <= Utc::now().timestamp() {
                return Err(LimitOrderError::InvalidParameters("Expire time must be in the future".to_string()));
            }
        }
        Ok(())
    }

    /// Check if order can be filled at given price
    pub fn can_fill(&self, current_price: Decimal) -> bool {
        match self.side {
            OrderSide::Buy => current_price <= self.limit_price,
            OrderSide::Sell => current_price >= self.limit_price,
        }
    }

    /// Check if stop trigger is reached
    pub fn check_stop_trigger(&self, current_price: Decimal) -> bool {
        if let Some(ref condition) = self.price_condition {
            match condition.condition {
                TriggerCondition::GreaterThanOrEqual => current_price >= condition.trigger_price,
                TriggerCondition::LessThanOrEqual => current_price <= condition.trigger_price,
                TriggerCondition::CrossingAbove => current_price >= condition.trigger_price,
                TriggerCondition::CrossingBelow => current_price <= condition.trigger_price,
            }
        } else {
            false
        }
    }

    /// Calculate trailing stop price
    pub fn calculate_trailing_stop(&self, highest_price: Decimal, current_price: Decimal) -> Decimal {
        if let Some(ref config) = self.trailing_stop {
            if current_price < config.activation_price {
                return Decimal::ZERO;
            }
            match config.trail_type {
                TrailType::Percentage => {
                    let trail_amount = highest_price * Decimal::from(config.trail_value_bps) / Decimal::from(10000);
                    highest_price - trail_amount
                }
                TrailType::Absolute => highest_price - Decimal::from(config.trail_value_bps),
            }
        } else {
            Decimal::ZERO
        }
    }

    /// Fill the order
    pub fn fill(&mut self, amount: u128, current_price: Decimal) -> Result<(), LimitOrderError> {
        let remaining = self.amount_in - self.amount_filled;
        if amount > remaining {
            return Err(LimitOrderError::InvalidParameters("Fill amount exceeds remaining".to_string()));
        }
        
        self.amount_filled += amount;
        self.updated_at = Utc::now().timestamp();
        
        if self.amount_filled >= self.amount_in {
            self.status = OrderStatus::Filled;
            self.filled_at = Some(Utc::now().timestamp());
        } else {
            self.status = OrderStatus::PartiallyFilled;
        }
        
        Ok(())
    }

    /// Activate the order
    pub fn activate(&mut self) {
        self.status = OrderStatus::Active;
        self.updated_at = Utc::now().timestamp();
    }

    /// Cancel the order
    pub fn cancel(&mut self) {
        self.status = OrderStatus::Cancelled;
        self.updated_at = Utc::now().timestamp();
    }

    /// Expire the order
    pub fn expire(&mut self) {
        self.status = OrderStatus::Expired;
        self.updated_at = Utc::now().timestamp();
    }

    /// Get remaining amount
    pub fn remaining(&self) -> u128 {
        self.amount_in - self.amount_filled
    }

    /// Get effective price
    pub fn effective_price(&self) -> Decimal {
        if self.amount_filled > 0 {
            self.limit_price
        } else {
            self.limit_price
        }
    }

    /// Calculate fees
    pub fn calculate_fees(&self) -> u128 {
        let filled_value = self.amount_filled;
        (filled_value * self.fee_bps as u128) / 10000
    }
}

/// Order fill result
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct OrderFill {
    pub order_id: String,
    pub user: String,
    pub amount_in: u128,
    pub amount_out: u128,
    pub price: Decimal,
    pub fee: u128,
    pub filled_at: i64,
    pub tx_hash: String,
}

/// Order execution result
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ExecuteResult {
    pub success: bool,
    pub order_id: String,
    pub fills: Vec<OrderFill>,
    pub total_amount_in: u128,
    pub total_amount_out: u128,
    pub average_price: Decimal,
    pub total_fees: u128,
    pub error: Option<String>,
}

impl ExecuteResult {
    pub fn success(order_id: String, fills: Vec<OrderFill>) -> Self {
        let total_amount_in: u128 = fills.iter().map(|f| f.amount_in).sum();
        let total_amount_out: u128 = fills.iter().map(|f| f.amount_out).sum();
        let total_fees: u128 = fills.iter().map(|f| f.fee).sum();
        let average_price = if total_amount_in > 0 {
            Decimal::from(total_amount_out) / Decimal::from(total_amount_in)
        } else {
            Decimal::ZERO
        };

        Self {
            success: true,
            order_id,
            fills,
            total_amount_in,
            total_amount_out,
            average_price,
            total_fees,
            error: None,
        }
    }

    pub fn failure(order_id: String, error: String) -> Self {
        Self {
            success: false,
            order_id,
            fills: vec![],
            total_amount_in: 0,
            total_amount_out: 0,
            average_price: Decimal::ZERO,
            total_fees: 0,
            error: Some(error),
        }
    }
}

/// Price oracle interface
pub trait PriceOracle: Send + Sync {
    fn get_price(&self, chain_id: u64, token: &str) -> Option<Decimal>;
    fn get_prices(&self, chain_id: u64, tokens: &[&str]) -> HashMap<String, Decimal>;
}

/// Limit order engine
pub struct LimitOrderEngine {
    orders: Arc<RwLock<HashMap<String, LimitOrder>>>,
    filled_orders: Arc<RwLock<HashMap<String, Vec<OrderFill>>>>,
    price_oracle: Arc<RwLock<Option<Box<dyn PriceOracle>>>>,
    supported_chains: Arc<RwLock<std::collections::HashSet<u64>>>,
    max_slippage_bps: i64,
}

impl LimitOrderEngine {
    /// Create a new limit order engine
    pub fn new() -> Self {
        let chains: std::collections::HashSet<u64> = [
            CHAIN_ETH, CHAIN_BSC, CHAIN_POLYGON, CHAIN_ARBITRUM, 
            CHAIN_OPTIMISM, CHAIN_BASE, CHAIN_AVALANCHE, CHAIN_SOLANA,
            CHAIN_APTOS, CHAIN_SUI,
        ].into_iter().collect();
        
        Self {
            orders: Arc::new(RwLock::new(HashMap::new())),
            filled_orders: Arc::new(RwLock::new(HashMap::new())),
            price_oracle: Arc::new(RwLock::new(None)),
            supported_chains: Arc::new(RwLock::new(chains)),
            max_slippage_bps: 100,  // 1% default max slippage
        }
    }

    /// Set price oracle
    pub fn set_price_oracle(&self, oracle: Box<dyn PriceOracle>) {
        *self.price_oracle.write() = Some(oracle);
    }

    /// Get current price
    pub fn get_price(&self, chain_id: u64, token: &str) -> Option<Decimal> {
        self.price_oracle.read().as_ref()?.get_price(chain_id, token)
    }

    /// Check if chain is supported
    pub fn is_chain_supported(&self, chain_id: u64) -> bool {
        self.supported_chains.read().contains(&chain_id)
    }

    /// Create a new limit order
    pub fn create_order(&self, order: LimitOrder) -> Result<String, LimitOrderError> {
        order.validate()?;
        
        if !self.is_chain_supported(order.chain_id) {
            return Err(LimitOrderError::ChainNotSupported(order.chain_id));
        }
        
        let order_id = order.order_id.clone();
        self.orders.write().insert(order_id.clone(), order);
        Ok(order_id)
    }

    /// Get order by ID
    pub fn get_order(&self, order_id: &str) -> Option<LimitOrder> {
        self.orders.read().get(order_id).cloned()
    }

    /// Get orders for a user
    pub fn get_user_orders(&self, user: &str) -> Vec<LimitOrder> {
        self.orders.read()
            .values()
            .filter(|o| o.user == user)
            .cloned()
            .collect()
    }

    /// Get active orders for a token pair
    pub fn get_active_orders(&self, chain_id: u64, token_in: &str, token_out: &str) -> Vec<LimitOrder> {
        self.orders.read()
            .values()
            .filter(|o| {
                o.chain_id == chain_id &&
                o.token_in == token_in &&
                o.token_out == token_out &&
                matches!(o.status, OrderStatus::Active | OrderStatus::Pending)
            })
            .cloned()
            .collect()
    }

    /// Get buy orders sorted by price (highest first)
    pub fn get_buy_orders(&self, chain_id: u64, token_in: &str, token_out: &str) -> Vec<LimitOrder> {
        let mut orders: Vec<LimitOrder> = self.get_active_orders(chain_id, token_in, token_out)
            .into_iter()
            .filter(|o| o.side.is_buy())
            .collect();
        
        orders.sort_by(|a, b| b.limit_price.cmp(&a.limit_price));
        orders
    }

    /// Get sell orders sorted by price (lowest first)
    pub fn get_sell_orders(&self, chain_id: u64, token_in: &str, token_out: &str) -> Vec<LimitOrder> {
        let mut orders: Vec<LimitOrder> = self.get_active_orders(chain_id, token_in, token_out)
            .into_iter()
            .filter(|o| o.side.is_sell())
            .collect();
        
        orders.sort_by(|a, b| a.limit_price.cmp(&b.limit_price));
        orders
    }

    /// Cancel an order
    pub fn cancel_order(&self, order_id: &str, user: &str) -> Result<(), LimitOrderError> {
        let mut orders = self.orders.write();
        let order = orders.get_mut(order_id)
            .ok_or_else(|| LimitOrderError::OrderNotFound(order_id.to_string()))?;
        
        if order.user != user {
            return Err(LimitOrderError::OrderNotFound(order_id.to_string()));
        }
        
        if matches!(order.status, OrderStatus::Filled | OrderStatus::Cancelled) {
            return Err(LimitOrderError::OrderCancelled(order_id.to_string()));
        }
        
        order.cancel();
        Ok(())
    }

    /// Cancel all orders for a user
    pub fn cancel_all_orders(&self, user: &str) -> usize {
        let mut count = 0;
        let mut orders = self.orders.write();
        for order in orders.values_mut() {
            if order.user == user && !matches!(order.status, OrderStatus::Filled | OrderStatus::Cancelled) {
                order.cancel();
                count += 1;
            }
        }
        count
    }

    /// Execute a limit order
    pub fn execute_order(&self, order_id: &str, current_price: Decimal, allow_partial: bool) -> Result<ExecuteResult, LimitOrderError> {
        let mut orders = self.orders.write();
        let order = orders.get_mut(order_id)
            .ok_or_else(|| LimitOrderError::OrderNotFound(order_id.to_string()))?;
        
        // Check order status
        match order.status {
            OrderStatus::Filled => return Err(LimitOrderError::OrderNotFound(order_id.to_string())),
            OrderStatus::Cancelled => return Err(LimitOrderError::OrderCancelled(order_id.to_string())),
            OrderStatus::Expired => return Err(LimitOrderError::OrderExpired(order_id.to_string())),
            _ => {}
        }
        
        // Check GTD expiration
        if let Some(expire_at) = order.expire_at {
            if Utc::now().timestamp() > expire_at {
                order.expire();
                return Err(LimitOrderError::OrderExpired(order_id.to_string()));
            }
        }
        
        // Check stop/take-profit trigger
        if order.order_type.requires_trigger() {
            if !order.check_stop_trigger(current_price) {
                return Err(LimitOrderError::StopTriggerNotReached(order_id.to_string()));
            }
            order.activate();
        }
        
        // Check if order can be filled
        if !order.can_fill(current_price) {
            return Err(LimitOrderError::PriceConditionNotMet {
                expected: order.limit_price,
                actual: current_price,
            });
        }
        
        // Calculate fill amount
        let fill_amount = if allow_partial {
            order.remaining()
        } else {
            order.amount_in
        };
        
        // Fill the order
        order.fill(fill_amount, current_price)?;
        
        // Create fill record
        let fill = OrderFill {
            order_id: order_id.to_string(),
            user: order.user.clone(),
            amount_in: fill_amount,
            amount_out: fill_amount * current_price.as_u128(),
            price: current_price,
            fee: order.calculate_fees(),
            filled_at: Utc::now().timestamp(),
            tx_hash: Uuid::new_v4().to_string(),
        };
        
        // Store fill
        let fills = vec![fill];
        self.filled_orders.write().insert(order_id.to_string(), fills.clone());
        
        Ok(ExecuteResult::success(order_id.to_string(), fills))
    }

    /// Execute all matching limit orders (market order style)
    pub fn execute_market(
        &self,
        chain_id: u64,
        token_in: &str,
        token_out: &str,
        side: OrderSide,
        amount_in: u128,
        current_price: Decimal,
    ) -> Result<ExecuteResult, LimitOrderError> {
        let orders = if side.is_buy() {
            self.get_buy_orders(chain_id, token_in, token_out)
        } else {
            self.get_sell_orders(chain_id, token_in, token_out)
        };
        
        let mut fills: Vec<OrderFill> = vec![];
        let mut remaining = amount_in;
        
        for order in orders {
            if remaining == 0 { break; }
            
            let fill_amount = std::cmp::min(remaining, order.remaining());
            let amount_out = fill_amount * current_price.as_u128();
            
            let fill = OrderFill {
                order_id: order.order_id.clone(),
                user: order.user.clone(),
                amount_in: fill_amount,
                amount_out,
                price: current_price,
                fee: (amount_out * order.fee_bps as u128) / 10000,
                filled_at: Utc::now().timestamp(),
                tx_hash: Uuid::new_v4().to_string(),
            };
            
            fills.push(fill);
            remaining -= fill_amount;
        }
        
        if fills.is_empty() {
            return Err(LimitOrderError::InsufficientLiquidity("No matching orders".to_string()));
        }
        
        // Update orders
        let mut orders_lock = self.orders.write();
        for fill in &fills {
            if let Some(order) = orders_lock.get_mut(&fill.order_id) {
                order.fill(fill.amount_in, current_price).ok();
            }
        }
        
        // Store fills
        for fill in &fills {
            self.filled_orders.write()
                .entry(fill.order_id.clone())
                .or_insert_with(Vec::new)
                .push(fill.clone());
        }
        
        let total_in: u128 = fills.iter().map(|f| f.amount_in).sum();
        Ok(ExecuteResult::success(String::new(), fills))
    }

    /// Process stop-loss triggers
    pub fn process_stop_triggers(&self, chain_id: u64, token_in: &str, token_out: &str, current_price: Decimal) -> Vec<String> {
        let mut triggered = vec![];
        let mut orders = self.orders.write();
        
        for order in orders.values_mut() {
            if order.chain_id == chain_id &&
               order.token_in == token_in &&
               order.token_out == token_out &&
               order.order_type == OrderType::StopLoss &&
               matches!(order.status, OrderStatus::Active | OrderStatus::Pending) &&
               order.check_stop_trigger(current_price) {
                order.activate();
                triggered.push(order.order_id.clone());
            }
        }
        
        triggered
    }

    /// Process take-profit triggers
    pub fn process_take_profit_triggers(&self, chain_id: u64, token_in: &str, token_out: &str, current_price: Decimal) -> Vec<String> {
        let mut triggered = vec![];
        let mut orders = self.orders.write();
        
        for order in orders.values_mut() {
            if order.chain_id == chain_id &&
               order.token_in == token_in &&
               order.token_out == token_out &&
               order.order_type == OrderType::TakeProfit &&
               matches!(order.status, OrderStatus::Active | OrderStatus::Pending) &&
               order.check_stop_trigger(current_price) {
                order.activate();
                triggered.push(order.order_id.clone());
            }
        }
        
        triggered
    }

    /// Process trailing stops
    pub fn process_trailing_stops(&self, chain_id: u64, token_in: &str, token_out: &str, current_price: Decimal) -> Vec<String> {
        let mut triggered = vec![];
        let mut orders = self.orders.write();
        
        for order in orders.values_mut() {
            if order.chain_id == chain_id &&
               order.token_in == token_in &&
               order.token_out == token_out &&
               order.order_type == OrderType::TrailingStop &&
               matches!(order.status, OrderStatus::Active | OrderStatus::Pending) {
                
                // Calculate trailing stop
                let stop_price = order.calculate_trailing_stop(current_price, current_price);
                
                // Check if trailing stop is triggered
                if stop_price > Decimal::ZERO {
                    match order.side {
                        OrderSide::Sell if current_price <= stop_price => {
                            order.activate();
                            triggered.push(order.order_id.clone());
                        }
                        OrderSide::Buy if current_price >= stop_price => {
                            order.activate();
                            triggered.push(order.order_id.clone());
                        }
                        _ => {}
                    }
                }
            }
        }
        
        triggered
    }

    /// Process OCO orders (cancel other if one is filled)
    pub fn process_oco(&self, order_id: &str) -> Result<(), LimitOrderError> {
        let orders = self.orders.read();
        let order = orders.get(order_id)
            .ok_or_else(|| LimitOrderError::OrderNotFound(order_id.to_string()))?;
        
        if let Some(ref oco) = order.oco {
            // Cancel the other order in the pair
            let other_id = if oco.order_a == order_id { &oco.order_b } else { &oco.order_a };
            drop(orders);
            
            let mut orders = self.orders.write();
            if let Some(other) = orders.get_mut(other_id) {
                other.cancel();
            }
        }
        
        Ok(())
    }

    /// Get order history for a user
    pub fn get_order_history(&self, user: &str, limit: usize) -> Vec<LimitOrder> {
        let mut orders: Vec<LimitOrder> = self.orders.read()
            .values()
            .filter(|o| o.user == user)
            .cloned()
            .collect();
        
        orders.sort_by(|a, b| b.updated_at.cmp(&a.updated_at));
        orders.truncate(limit);
        orders
    }

    /// Get filled orders
    pub fn get_filled_orders(&self, order_id: &str) -> Vec<OrderFill> {
        self.filled_orders.read().get(order_id).cloned().unwrap_or_default()
    }

    /// Get market depth
    pub fn get_market_depth(&self, chain_id: u64, token_in: &str, token_out: &str, levels: usize) -> (Vec<(Decimal, u128)>, Vec<(Decimal, u128)>) {
        let buy_orders = self.get_buy_orders(chain_id, token_in, token_out);
        let sell_orders = self.get_sell_orders(chain_id, token_in, token_out);
        
        let mut bids: Vec<(Decimal, u128)> = vec![];
        let mut asks: Vec<(Decimal, u128)> = vec![];
        
        for order in buy_orders.iter().take(levels) {
            bids.push((order.limit_price, order.remaining()));
        }
        
        for order in sell_orders.iter().take(levels) {
            asks.push((order.limit_price, order.remaining()));
        }
        
        (bids, asks)
    }

    /// Calculate average fill price
    pub fn calculate_vwap(&self, chain_id: u64, token_in: &str, token_out: &str) -> Decimal {
        let orders = self.get_active_orders(chain_id, token_in, token_out);
        
        let mut total_value: u128 = 0;
        let mut total_amount: u128 = 0;
        
        for order in orders {
            let value = order.remaining() * order.limit_price.as_u128();
            total_value += value;
            total_amount += order.remaining();
        }
        
        if total_amount > 0 {
            Decimal::from(total_value) / Decimal::from(total_amount)
        } else {
            Decimal::ZERO
        }
    }

    /// Clean up expired orders
    pub fn cleanup_expired(&self) -> usize {
        let now = Utc::now().timestamp();
        let mut count = 0;
        let mut orders = self.orders.write();
        
        for order in orders.values_mut() {
            if let Some(expire_at) = order.expire_at {
                if now > expire_at && !matches!(order.status, OrderStatus::Filled | OrderStatus::Cancelled) {
                    order.expire();
                    count += 1;
                }
            }
            
            // Clean up old filled/cancelled orders (older than 30 days)
            if let Some(filled_at) = order.filled_at {
                if now - filled_at > 30 * 24 * 60 * 60 {
                    // Could archive here
                }
            }
        }
        
        count
    }

    /// Get statistics
    pub fn get_stats(&self) -> OrderStats {
        let orders = self.orders.read();
        
        let mut pending = 0;
        let mut active = 0;
        let mut filled = 0;
        let mut cancelled = 0;
        let mut expired = 0;
        let mut total_volume: u128 = 0;
        
        for order in orders.values() {
            match order.status {
                OrderStatus::Pending => pending += 1,
                OrderStatus::Active => active += 1,
                OrderStatus::Filled => {
                    filled += 1;
                    total_volume += order.amount_in;
                }
                OrderStatus::Cancelled => cancelled += 1,
                OrderStatus::Expired => expired += 1,
                _ => {}
            }
        }
        
        OrderStats {
            pending,
            active,
            filled,
            cancelled,
            expired,
            total_orders: orders.len(),
            total_volume,
        }
    }

    /// Add supported chain
    pub fn add_chain(&self, chain_id: u64) {
        self.supported_chains.write().insert(chain_id);
    }

    /// Remove supported chain
    pub fn remove_chain(&self, chain_id: u64) {
        self.supported_chains.write().remove(&chain_id);
    }

    /// Get supported chains
    pub fn supported_chains(&self) -> Vec<u64> {
        self.supported_chains.read().iter().cloned().collect()
    }
}

impl Default for LimitOrderEngine {
    fn default() -> Self { Self::new() }
}

/// Order statistics
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct OrderStats {
    pub pending: usize,
    pub active: usize,
    pub filled: usize,
    pub cancelled: usize,
    pub expired: usize,
    pub total_orders: usize,
    pub total_volume: u128,
}

#[cfg(test)]
mod tests {
    use super::*;
    use rust_decimal_macros::dec;

    #[test]
    fn test_limit_order_creation() {
        let order = LimitOrder::new(
            "user1".to_string(),
            CHAIN_ETH,
            "0xA0b86a33E6441C4A1E3C1E1F1C1E1F1C1E1F1C1E1".to_string(),
            "0xB0b86a33E6441C4A1E3C1E1F1C1E1F1C1E1F1C1E1".to_string(),
            OrderSide::Buy,
            1000,
            dec!(0.05),
        );
        
        assert!(order.validate().is_ok());
        assert_eq!(order.side, OrderSide::Buy);
    }

    #[test]
    fn test_stop_loss_order() {
        let order = LimitOrder::new_stop_loss(
            "user1".to_string(),
            CHAIN_ETH,
            "WETH".to_string(),
            "USDC".to_string(),
            OrderSide::Sell,
            1000,
            dec!(0.045),
        );
        
        assert_eq!(order.order_type, OrderType::StopLoss);
        assert!(order.check_stop_trigger(dec!(0.044)));
    }

    #[test]
    fn test_order_can_fill() {
        let order = LimitOrder::new(
            "user1".to_string(),
            CHAIN_ETH,
            "WETH".to_string(),
            "USDC".to_string(),
            OrderSide::Buy,
            1000,
            dec!(0.05),
        );
        
        assert!(order.can_fill(dec!(0.04)));  // Lower price = can buy
        assert!(!order.can_fill(dec!(0.06))); // Higher price = can't buy
    }

    #[test]
    fn test_trailing_stop() {
        let order = LimitOrder::new_trailing_stop(
            "user1".to_string(),
            CHAIN_ETH,
            "WETH".to_string(),
            "USDC".to_string(),
            OrderSide::Sell,
            1000,
            100,  // 1% trail
            dec!(0.05),
        );
        
        let stop_price = order.calculate_trailing_stop(dec!(0.06), dec!(0.055));
        assert!(stop_price > Decimal::ZERO);
    }
}