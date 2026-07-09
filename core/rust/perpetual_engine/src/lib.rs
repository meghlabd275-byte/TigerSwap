//! TigerSwap Perpetual Trading Engine
//! High-performance decentralized perpetual exchange

use std::collections::HashMap;
use std::sync::Arc;
use parking_lot::RwLock;
use tokio::sync::broadcast;
use serde::{Deserialize, Serialize};
use rust_decimal::Decimal;
use uuid::Uuid;
use chrono::{DateTime, Utc};

// ============================================================================
// Types
// ============================================================================

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum OrderSide {
    Buy,
    Sell,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum OrderType {
    Market,
    Limit,
    StopLoss,
    StopLimit,
    TakeProfit,
    TakeProfitLimit,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum PositionSide {
    Long,
    Short,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum OrderStatus {
    Open,
    PartiallyFilled,
    Filled,
    Cancelled,
    Liquidated,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum TimeInForce {
    GTC,  // Good Till Cancel
    IOC,  // Immediate or Cancel
    FOK,  // Fill or Kill
}

// ============================================================================
// Core Structures
// ============================================================================

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Market {
    pub id: String,
    pub symbol: String,
    pub base_asset: String,
    pub quote_asset: String,
    
    // Price
    pub mark_price: Decimal,
    pub index_price: Decimal,
    pub last_price: Decimal,
    
    // Funding
    pub funding_rate: Decimal,
    pub funding_timestamp: i64,
    pub next_funding_time: i64,
    
    // Limits
    pub max_leverage: u32,
    pub max_position_size: Decimal,
    pub maintenance_margin_rate: Decimal,
    pub initial_margin_rate: Decimal,
    
    // Status
    pub is_active: bool,
    pub is_paused: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Order {
    pub id: String,
    pub market_id: String,
    pub user_id: String,
    
    pub side: OrderSide,
    pub order_type: OrderType,
    pub time_in_force: TimeInForce,
    
    pub price: Decimal,
    pub quantity: Decimal,
    pub filled_quantity: Decimal,
    
    pub leverage: u32,
    
    pub stop_price: Option<Decimal>,
    
    pub status: OrderStatus,
    pub created_at: DateTime<Utc>,
    pub updated_at: DateTime<Utc>,
    
    // For matching
    pub avg_fill_price: Option<Decimal>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Position {
    pub id: String,
    pub market_id: String,
    pub user_id: String,
    
    pub side: PositionSide,
    pub size: Decimal,
    pub entry_price: Decimal,
    pub mark_price: Decimal,
    
    pub margin: Decimal,
    pub leverage: u32,
    
    pub unrealized_pnl: Decimal,
    pub realized_pnl: Decimal,
    
    pub liquidation_price: Option<Decimal>,
    pub bankruptcy_price: Option<Decimal>,
    
    pub created_at: DateTime<Utc>,
    pub updated_at: DateTime<Utc>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Trade {
    pub id: String,
    pub market_id: String,
    pub order_id: String,
    pub counter_order_id: Option<String>,
    
    pub side: OrderSide,
    pub price: Decimal,
    pub quantity: Decimal,
    
    pub maker_fee: Decimal,
    pub taker_fee: Decimal,
    pub funding_payment: Decimal,
    
    pub maker_user_id: String,
    pub taker_user_id: String,
    
    pub timestamp: DateTime<Utc>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Account {
    pub user_id: String,
    pub total_collateral: Decimal,
    pub available_collateral: Decimal,
    pub total_position_value: Decimal,
    pub total_margin_used: Decimal,
    
    pub unrealized_pnl: Decimal,
    pub realized_pnl: Decimal,
    
    pub margin_ratio: Decimal,
    pub account_status: AccountStatus,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum AccountStatus {
    Healthy,
    AtRisk,
    PartialLiquidation,
    Liquidated,
    Bankrupt,
}

// ============================================================================
// Engine Errors
// ============================================================================

#[derive(Debug, thiserror::Error)]
pub enum EngineError {
    #[error("Market not found: {0}")]
    MarketNotFound(String),
    
    #[error("Insufficient margin: available {available}, required {required}")]
    InsufficientMargin { available: Decimal, required: Decimal },
    
    #[error("Insufficient balance")]
    InsufficientBalance,
    
    #[error("Position not found: {0}")]
    PositionNotFound(String),
    
    #[error("Order not found: {0}")]
    OrderNotFound(String),
    
    #[error("Invalid leverage: {0}")]
    InvalidLeverage(u32),
    
    #[error("Price too far from index")]
    PriceTooFarFromIndex,
    
    #[error("Position would be liquidated")]
    WouldBeLiquidated,
    
    #[error("Max position size exceeded")]
    MaxPositionSizeExceeded,
    
    #[error("Market paused")]
    MarketPaused,
}

// ============================================================================
// Perpetual Engine
// ============================================================================

pub struct PerpetualEngine {
    markets: RwLock<HashMap<String, Market>>,
    orders: RwLock<HashMap<String, Order>>,
    positions: RwLock<HashMap<String, Position>>,
    accounts: RwLock<HashMap<String, Account>>,
    
    // Price feeds
    price_feeds: RwLock<HashMap<String, Decimal>>,
    
    // Event channels
    trade_sender: broadcast::Sender<Trade>,
    liquidation_sender: broadcast::Sender<LiquidationEvent>,
    
    // Config
    max_leverage: u32,
    default_leverage: u32,
    maker_fee: Decimal,
    taker_fee: Decimal,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct LiquidationEvent {
    pub position_id: String,
    pub user_id: String,
    pub market_id: String,
    pub side: PositionSide,
    pub size: Decimal,
    pub liquidation_price: Decimal,
    pub bankruptcy_price: Decimal,
    pub margin_remaining: Decimal,
    pub timestamp: DateTime<Utc>,
}

impl PerpetualEngine {
    pub fn new() -> Self {
        let (trade_sender, _) = broadcast::channel(10000);
        let (liquidation_sender, _) = broadcast::channel(1000);
        
        Self {
            markets: RwLock::new(HashMap::new()),
            orders: RwLock::new(HashMap::new()),
            positions: RwLock::new(HashMap::new()),
            accounts: RwLock::new(HashMap::new()),
            price_feeds: RwLock::new(HashMap::new()),
            trade_sender,
            liquidation_sender,
            max_leverage: 100,
            default_leverage: 10,
            maker_fee: Decimal::from(10),   // 0.001%
            taker_fee: Decimal::from(30),   // 0.003%
        }
    }
    
    // ========================================================================
    // Market Management
    // ========================================================================
    
    pub fn add_market(&self, market: Market) {
        self.markets.write().insert(market.id.clone(), market);
    }
    
    pub fn get_market(&self, market_id: &str) -> Option<Market> {
        self.markets.read().get(market_id).cloned()
    }
    
    pub fn list_markets(&self) -> Vec<Market> {
        self.markets.read().values().cloned().collect()
    }
    
    // ========================================================================
    // Price Feed
    // ========================================================================
    
    pub fn update_price(&self, market_id: &str, price: Decimal) {
        self.price_feeds.write().insert(market_id.to_string(), price);
        
        // Update market mark price
        if let Some(market) = self.markets.write().get_mut(market_id) {
            market.mark_price = price;
        }
        
        // Check liquidations
        self.check_liquidations(market_id);
    }
    
    // ========================================================================
    // Order Management
    // ========================================================================
    
    pub fn create_order(
        &self,
        market_id: &str,
        user_id: &str,
        side: OrderSide,
        order_type: OrderType,
        price: Decimal,
        quantity: Decimal,
        leverage: u32,
        time_in_force: TimeInForce,
        stop_price: Option<Decimal>,
    ) -> Result<Order, EngineError> {
        // Validate market
        let market = self.markets.read()
            .get(market_id)
            .ok_or_else(|| EngineError::MarketNotFound(market_id.to_string()))?
            .clone();
        
        if !market.is_active || market.is_paused {
            return Err(EngineError::MarketPaused);
        }
        
        // Validate leverage
        if leverage > self.max_leverage || leverage == 0 {
            return Err(EngineError::InvalidLeverage(leverage));
        }
        
        if leverage > market.max_leverage {
            return Err(EngineError::InvalidLeverage(leverage));
        }
        
        // Calculate required margin
        let notional_value = price * quantity;
        let required_margin = notional_value / Decimal::from(leverage);
        
        // Check balance
        let account = self.accounts.read()
            .get(user_id)
            .cloned();
        
        if let Some(acc) = account {
            if required_margin > acc.available_collateral {
                return Err(EngineError::InsufficientMargin {
                    available: acc.available_collateral,
                    required: required_margin,
                });
            }
        } else {
            return Err(EngineError::InsufficientBalance);
        }
        
        // Validate price (for limit orders)
        if order_type == OrderType::Limit {
            let index_price = market.index_price;
            let max_diff = index_price * Decimal::from(5) / Decimal::from(1000);  // 0.5%
            
            if (price - index_price).abs() > max_diff {
                return Err(EngineError::PriceTooFarFromIndex);
            }
        }
        
        // Create order
        let order = Order {
            id: Uuid::new_v4().to_string(),
            market_id: market_id.to_string(),
            user_id: user_id.to_string(),
            side,
            order_type,
            time_in_force,
            price,
            quantity,
            filled_quantity: Decimal::ZERO,
            leverage,
            stop_price,
            status: OrderStatus::Open,
            created_at: Utc::now(),
            updated_at: Utc::now(),
            avg_fill_price: None,
        };
        
        // Store order
        self.orders.write().insert(order.id.clone(), order.clone());
        
        // Execute if market order
        if order_type == OrderType::Market {
            return self.execute_order(&order.id);
        }
        
        Ok(order)
    }
    
    pub fn execute_order(&self, order_id: &str) -> Result<Order, EngineError> {
        let mut order = self.orders.write()
            .get_mut(order_id)
            .ok_or_else(|| EngineError::OrderNotFound(order_id.to_string()))?
            .clone();
        
        // Get market and price
        let market = self.markets.read()
            .get(&order.market_id)
            .ok_or_else(|| EngineError::MarketNotFound(order.market_id.clone()))?
            .clone();
        
        // Use market price for market orders
        let fill_price = if order.order_type == OrderType::Market {
            market.mark_price
        } else {
            order.price
        };
        
        // Check and update position
        let position_id = format!("{}_{}", order.user_id, order.market_id);
        
        let position = self.positions.write()
            .entry(position_id.clone())
            .or_insert_with(|| Position {
                id: position_id,
                market_id: order.market_id.clone(),
                user_id: order.user_id.clone(),
                side: if order.side == OrderSide::Buy { PositionSide::Long } else { PositionSide::Short },
                size: Decimal::ZERO,
                entry_price: Decimal::ZERO,
                mark_price: market.mark_price,
                margin: Decimal::ZERO,
                leverage: order.leverage,
                unrealized_pnl: Decimal::ZERO,
                realized_pnl: Decimal::ZERO,
                liquidation_price: None,
                bankruptcy_price: None,
                created_at: Utc::now(),
                updated_at: Utc::now(),
            });
        
        // Calculate new position
        let new_side = if order.side == OrderSide::Buy { PositionSide::Long } else { PositionSide::Short };
        
        if position.size == Decimal::ZERO {
            position.side = new_side;
            position.size = order.quantity;
            position.entry_price = fill_price;
            position.leverage = order.leverage;
        } else if position.side == new_side {
            // Same direction - increase
            let total_size = position.size + order.quantity;
            position.entry_price = (position.entry_price * position.size + fill_price * order.quantity) / total_size;
            position.size = total_size;
        } else {
            // Opposite direction - reduce or flip
            if order.quantity >= position.size {
                // Flip position
                position.size = order.quantity - position.size;
                position.side = new_side;
                position.entry_price = fill_price;
            } else {
                // Reduce position
                position.size = position.size - order.quantity;
            }
        }
        
        // Update position margin
        let notional_value = fill_price * position.size;
        position.margin = notional_value / Decimal::from(position.leverage);
        position.updated_at = Utc::now();
        
        // Update order
        order.filled_quantity = order.quantity;
        order.avg_fill_price = Some(fill_price);
        order.status = OrderStatus::Filled;
        order.updated_at = Utc::now();
        
        self.orders.write().insert(order.id.clone(), order.clone());
        
        // Create trade event
        let trade = Trade {
            id: Uuid::new_v4().to_string(),
            market_id: order.market_id.clone(),
            order_id: order.id.clone(),
            counter_order_id: None,
            side: order.side,
            price: fill_price,
            quantity: order.quantity,
            maker_fee: self.maker_fee * fill_price * order.quantity / Decimal::from(1000000),
            taker_fee: self.taker_fee * fill_price * order.quantity / Decimal::from(1000000),
            funding_payment: Decimal::ZERO,
            maker_user_id: order.user_id.clone(),
            taker_user_id: order.user_id.clone(),
            timestamp: Utc::now(),
        };
        
        let _ = self.trade_sender.send(trade);
        
        // Update account
        self.update_account(&order.user_id);
        
        Ok(order)
    }
    
    pub fn cancel_order(&self, order_id: &str) -> Result<Order, EngineError> {
        let mut order = self.orders.write()
            .get_mut(order_id)
            .ok_or_else(|| EngineError::OrderNotFound(order_id.to_string()))?
            .clone();
        
        if order.status != OrderStatus::Open && order.status != OrderStatus::PartiallyFilled {
            return Err(EngineError::OrderNotFound(order_id.to_string()));
        }
        
        order.status = OrderStatus::Cancelled;
        order.updated_at = Utc::now();
        
        self.orders.write().insert(order.id.clone(), order.clone());
        
        Ok(order)
    }
    
    // ========================================================================
    // Position Management
    // ========================================================================
    
    pub fn get_position(&self, user_id: &str, market_id: &str) -> Option<Position> {
        let position_id = format!("{}_{}", user_id, market_id);
        self.positions.read().get(&position_id).cloned()
    }
    
    pub fn get_user_positions(&self, user_id: &str) -> Vec<Position> {
        self.positions.read()
            .values()
            .filter(|p| p.user_id == user_id && p.size > Decimal::ZERO)
            .cloned()
            .collect()
    }
    
    pub fn close_position(&self, user_id: &str, market_id: &str, quantity: Option<Decimal>) -> Result<Position, EngineError> {
        let position_id = format!("{}_{}", user_id, market_id);
        
        let mut position = self.positions.write()
            .get_mut(&position_id)
            .ok_or_else(|| EngineError::PositionNotFound(position_id))?
            .clone();
        
        let close_qty = quantity.unwrap_or(position.size);
        
        if close_qty > position.size {
            return Err(EngineError::InsufficientBalance);
        }
        
        let market = self.markets.read()
            .get(market_id)
            .ok_or_else(|| EngineError::MarketNotFound(market_id.to_string()))?
            .clone();
        
        // Calculate PnL
        let pnl = if position.side == PositionSide::Long {
            (market.mark_price - position.entry_price) * close_qty
        } else {
            (position.entry_price - market.mark_price) * close_qty
        };
        
        position.realized_pnl += pnl;
        
        if close_qty == position.size {
            position.size = Decimal::ZERO;
        } else {
            position.size = position.size - close_qty;
        }
        
        position.updated_at = Utc::now();
        
        self.positions.write().insert(position_id.clone(), position.clone());
        
        // Update account
        self.update_account(user_id);
        
        Ok(position)
    }
    
    // ========================================================================
    // Account Management
    // ========================================================================
    
    pub fn create_account(&self, user_id: &str, initial_balance: Decimal) {
        let account = Account {
            user_id: user_id.to_string(),
            total_collateral: initial_balance,
            available_collateral: initial_balance,
            total_position_value: Decimal::ZERO,
            total_margin_used: Decimal::ZERO,
            unrealized_pnl: Decimal::ZERO,
            realized_pnl: Decimal::ZERO,
            margin_ratio: Decimal::ZERO,
            account_status: AccountStatus::Healthy,
        };
        
        self.accounts.write().insert(user_id.to_string(), account);
    }
    
    pub fn get_account(&self, user_id: &str) -> Option<Account> {
        self.accounts.read().get(user_id).cloned()
    }
    
    fn update_account(&self, user_id: &str) {
        let mut account = match self.accounts.write().get_mut(user_id) {
            Some(a) => a,
            None => return,
        };
        
        // Calculate position values
        let user_positions: Vec<_> = self.positions.read()
            .values()
            .filter(|p| p.user_id == user_id && p.size > Decimal::ZERO)
            .collect();
        
        let mut total_unrealized_pnl = Decimal::ZERO;
        let mut total_margin_used = Decimal::ZERO;
        let mut total_position_value = Decimal::ZERO;
        
        for position in &user_positions {
            let market = match self.markets.read().get(&position.market_id) {
                Some(m) => m,
                None => continue,
            };
            
            let pnl = if position.side == PositionSide::Long {
                (market.mark_price - position.entry_price) * position.size
            } else {
                (position.entry_price - market.mark_price) * position.size
            };
            
            total_unrealized_pnl += pnl;
            total_margin_used += position.margin;
            total_position_value += market.mark_price * position.size;
        }
        
        account.unrealized_pnl = total_unrealized_pnl;
        account.total_margin_used = total_margin_used;
        account.total_position_value = total_position_value;
        
        account.total_collateral = account.total_collateral + total_unrealized_pnl;
        account.available_collateral = account.total_collateral - total_margin_used;
        
        // Calculate margin ratio
        if total_position_value > Decimal::ZERO {
            account.margin_ratio = account.total_collateral * Decimal::from(1000000) / total_position_value;
        }
        
        // Update status
        account.account_status = if account.margin_ratio >= Decimal::from(100000) {  // 10%
            AccountStatus::Healthy
        } else if account.margin_ratio >= Decimal::from(50000) {  // 5%
            AccountStatus::AtRisk
        } else if account.margin_ratio > Decimal::ZERO {
            AccountStatus::PartialLiquidation
        } else {
            AccountStatus::Bankrupt
        };
    }
    
    // ========================================================================
    // Liquidation
    // ========================================================================
    
    fn check_liquidations(&self, market_id: &str) {
        let market = match self.markets.read().get(market_id) {
            Some(m) => m.clone(),
            None => return,
        };
        
        let positions_to_liquidate: Vec<_> = self.positions.read()
            .values()
            .filter(|p| p.market_id == market_id && p.size > Decimal::ZERO)
            .filter(|p| {
                if let Some(liq_price) = p.liquidation_price {
                    return market.mark_price <= liq_price;
                }
                false
            })
            .cloned()
            .collect();
        
        for position in positions_to_liquidate {
            self.liquidate_position(&position);
        }
    }
    
    fn liquidate_position(&self, position: &Position) {
        let market = match self.markers.read().get(&position.market_id) {
            Some(m) => m.clone(),
            None => return,
        };
        
        // Calculate liquidation
        let bankruptcy_price = if position.side == PositionSide::Long {
            position.entry_price * (Decimal::ONE - Decimal::from(1) / Decimal::from(position.leverage))
        } else {
            position.entry_price * (Decimal::ONE + Decimal::ONE / Decimal::from(position.leverage))
        };
        
        let margin_remaining = if market.mark_price >= bankruptcy_price {
            position.margin - (market.mark_price - position.entry_price) * position.size
        } else {
            Decimal::ZERO
        };
        
        // Create liquidation event
        let event = LiquidationEvent {
            position_id: position.id.clone(),
            user_id: position.user_id.clone(),
            market_id: position.market_id.clone(),
            side: position.side,
            size: position.size,
            liquidation_price: position.liquidation_price.unwrap_or(market.mark_price),
            bankruptcy_price,
            margin_remaining,
            timestamp: Utc::now(),
        };
        
        let _ = self.liquidation_sender.send(event);
        
        // Update position
        let mut positions = self.positions.write();
        if let Some(p) = positions.get_mut(&position.id) {
            p.size = Decimal::ZERO;
            p.status = OrderStatus::Liquidated;
            p.updated_at = Utc::now();
        }
        
        // Update account
        self.update_account(&position.user_id);
    }
    
    // ========================================================================
    // Funding
    // ========================================================================
    
    pub fn calculate_funding(&self, market_id: &str) -> Result<Decimal, EngineError> {
        let market = self.markets.read()
            .get(market_id)
            .ok_or_else(|| EngineError::MarketNotFound(market_id.to_string()))?
            .clone();
        
        // Premium = (Mark Price - Index Price) / Index Price
        let premium = (market.mark_price - market.index_price) / market.index_price;
        
        // Funding rate = premium, clamped to [-0.1%, 0.1%]
        let funding_rate = premium.max(Decimal::from(-1000)).min(Decimal::from(1000));
        
        Ok(funding_rate)
    }
    
    // ========================================================================
    // Subscriptions
    // ========================================================================
    
    pub fn subscribe_trades(&self) -> broadcast::Receiver<Trade> {
        self.trade_sender.subscribe()
    }
    
    pub fn subscribe_liquidations(&self) -> broadcast::Receiver<LiquidationEvent> {
        self.liquidation_sender.subscribe()
    }
}

impl Default for PerpetualEngine {
    fn default() -> Self {
        Self::new()
    }
}

// ============================================================================
// Tests
// ============================================================================

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_create_order() {
        let engine = PerpetualEngine::new();
        
        // Add market
        engine.add_market(Market {
            id: "ETH-PERP".to_string(),
            symbol: "ETH-PERP".to_string(),
            base_asset: "ETH".to_string(),
            quote_asset: "USD".to_string(),
            mark_price: Decimal::from(3250000000000000000i64),
            index_price: Decimal::from(3250000000000000000i64),
            last_price: Decimal::from(3250000000000000000i64),
            funding_rate: Decimal::ZERO,
            funding_timestamp: 0,
            next_funding_time: 0,
            max_leverage: 50,
            max_position_size: Decimal::from(1000000),
            maintenance_margin_rate: Decimal::from(50000),  // 5%
            initial_margin_rate: Decimal::from(100000),     // 10%
            is_active: true,
            is_paused: false,
        });
        
        // Create account
        engine.create_account("user1", Decimal::from(100000));
        
        // Create order
        let result = engine.create_order(
            "ETH-PERP",
            "user1",
            OrderSide::Buy,
            OrderType::Limit,
            Decimal::from(3250000000000000000i64),
            Decimal::from(1),
            10,
            TimeInForce::GTC,
            None,
        );
        
        assert!(result.is_ok());
    }
}
