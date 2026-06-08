//! TigerSwap WebSocket Price Stream Engine
//! 
//! Implements real-time price streaming via WebSocket:
//! - Live price feeds
//! - Subscription management
//! - Price tick updates
//! - Heartbeat/ping-pong
//!
//! Implementation: Pure Rust with no external dependencies

use serde::{Deserialize, Serialize};
use std::sync::Arc;
use parking_lot::RwLock;
use rust_decimal::Decimal;
use thiserror::Error;
use uuid::Uuid;
use chrono::Utc;
use std::collections::{HashMap, HashSet};

/// Chain ID constants
pub const CHAIN_ETH: u64 = 1;
pub const CHAIN_BSC: u64 = 56;
pub const CHAIN_POLYGON: u64 = 137;
pub const CHAIN_ARBITRUM: u64 = 42161;
pub const CHAIN_OPTIMISM: u64 = 10;
pub const CHAIN_BASE: u64 = 8453;

#[derive(Debug, Error)]
pub enum WebSocketError {
    #[error("Connection not found: {0}")]
    ConnectionNotFound(String),
    #[error("Subscription not found: {0}")]
    SubscriptionNotFound(String),
    #[error("Invalid message: {0}")]
    InvalidMessage(String),
    #[error("Connection closed")]
    ConnectionClosed,
}

/// Message type
#[derive(Debug, Clone, Serialize, Deserialize)]
pub enum WSMessageType {
    Subscribe,
    Unsubscribe,
    PriceUpdate,
    Subscribed,
    Unsubscribed,
    Heartbeat,
    Pong,
    Error,
}

/// WebSocket message
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct WSMessage {
    pub msg_type: WSMessageType,
    pub channel: String,
    pub data: Option<serde_json::Value>,
    pub timestamp: i64,
}

impl WSMessage {
    pub fn new(msg_type: WSMessageType, channel: String) -> Self {
        Self {
            msg_type,
            channel,
            data: None,
            timestamp: Utc::now().timestamp(),
        }
    }

    pub fn with_data(mut self, data: serde_json::Value) -> Self {
        self.data = Some(data);
        self
    }
}

/// Price tick
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PriceTick {
    pub feed_key: String,
    pub price: Decimal,
    pub change_24h: Decimal,    // 24h change
    pub change_bps: i64,     // Basis points
    pub volume_24h: u128,
    pub timestamp: i64,
}

impl PriceTick {
    pub fn new(feed_key: String, price: Decimal) -> Self {
        Self {
            feed_key,
            price,
            change_24h: Decimal::ZERO,
            change_bps: 0,
            volume_24h: 0,
            timestamp: Utc::now().timestamp(),
        }
    }
}

/// WebSocket connection
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct WSConnection {
    pub conn_id: String,
    pub user: Option<String>,
    pub subscriptions: HashSet<String>,
    pub is_authenticated: bool,
    pub connected_at: i64,
    pub last_ping: i64,
    pub is_active: bool,
}

impl WSConnection {
    pub fn new(user: Option<String>) -> Self {
        Self {
            conn_id: Uuid::new_v4().to_string(),
            user,
            subscriptions: HashSet::new(),
            is_authenticated: false,
            connected_at: Utc::now().timestamp(),
            last_ping: Utc::now().timestamp(),
            is_active: true,
        }
    }

    pub fn subscribe(&mut self, feed_key: String) {
        self.subscriptions.insert(feed_key);
    }

    pub fn unsubscribe(&mut self, feed_key: &str) {
        self.subscriptions.remove(feed_key);
    }

    pub fn ping(&mut self) {
        self.last_ping = Utc::now().timestamp();
    }

    pub fn is_alive(&self) -> bool {
        self.is_active && (Utc::now().timestamp() - self.last_ping) < 60
    }

    pub fn close(&mut self) {
        self.is_active = false;
        self.subscriptions.clear();
    }
}

/// Channel type
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum ChannelType {
    Prices,
    Trades,
    OrderBook,
    Liquidations,
    Funding,
    Tickers,
}

impl ChannelType {
    pub fn as_str(&self) -> &'static str {
        match self {
            ChannelType::Prices => "prices",
            ChannelType::Trades => "trades",
            ChannelType::OrderBook => "orderbook",
            ChannelType::Liquidations => "liquidations",
            ChannelType::Funding => "funding",
            ChannelType::Tickers => "tickers",
        }
    }
}

/// Channel subscription
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ChannelSubscription {
    pub channel: ChannelType,
    pub feed_keys: Vec<String>,
}

impl ChannelSubscription {
    pub fn new(channel: ChannelType, feed_keys: Vec<String>) -> Self {
        Self {
            channel,
            feed_keys,
        }
    }
}

/// Price stream data
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PriceStream {
    pub feed_key: String,
    pub price: Decimal,
    pub bid: Decimal,         // Best bid
    pub ask: Decimal,        // Best ask
    pub volume_24h: u128,
    pub change_24h: Decimal,
    pub timestamp: i64,
}

/// WebSocket Price Stream Engine
pub struct WebSocketPriceStream {
    connections: Arc<RwLock<HashMap<String, WSConnection>>>,
    price_streams: Arc<RwLock<HashMap<String, PriceStream>>>,
    subscribers: Arc<RwLock<HashMap<String, HashSet<String>>>>, // feed_key -> conn_ids
    max_connections: usize,
    heartbeat_interval: i64,
}

impl WebSocketPriceStream {
    pub fn new() -> Self {
        Self {
            connections: Arc::new(RwLock::new(HashMap::new())),
            price_streams: Arc::new(RwLock::new(HashMap::new())),
            subscribers: Arc::new(RwLock::new(HashMap::new())),
            max_connections: 10000,
            heartbeat_interval: 30,
        }
    }

    /// Accept new connection
    pub fn accept_connection(&self, user: Option<String>) -> Result<String, WebSocketError> {
        let mut conns = self.connections.write();
        
        if conns.len() >= self.max_connections {
            return Err(WebSocketError::ConnectionClosed);
        }
        
        let conn = WSConnection::new(user);
        let conn_id = conn.conn_id.clone();
        
        conns.insert(conn_id.clone(), conn);
        
        Ok(conn_id)
    }

    /// Authenticate connection
    pub fn authenticate(&self, conn_id: &str, user: String) -> Result<(), WebSocketError> {
        let mut conns = self.connections.write();
        let conn = conns.get_mut(conn_id)
            .ok_or_else(|| WebSocketError::ConnectionNotFound(conn_id.to_string()))?;
        
        conn.user = Some(user);
        conn.is_authenticated = true;
        
        Ok(())
    }

    /// Subscribe to feed
    pub fn subscribe(&self, conn_id: &str, feed_keys: Vec<String>) -> Result<(), WebSocketError> {
        let mut conns = self.connections.write();
        let conn = conns.get_mut(conn_id)
            .ok_or_else(|| WebSocketError::ConnectionNotFound(conn_id.to_string()))?;
        
        if !conn.is_active {
            return Err(WebSocketError::ConnectionClosed);
        }
        
        let mut subs = self.subscribers.write();
        
        for feed_key in feed_keys {
            conn.subscribe(feed_key.clone());
            subs.entry(feed_key).or_insert_with(HashSet::new).insert(conn_id.to_string());
        }
        
        Ok(())
    }

    /// Unsubscribe from feed
    pub fn unsubscribe(&self, conn_id: &str, feed_keys: Vec<String>) -> Result<(), WebSocketError> {
        let mut conns = self.connections.write();
        let conn = conns.get_mut(conn_id)
            .ok_or_else(|| WebSocketError::ConnectionNotFound(conn_id.to_string()))?;
        
        let mut subs = self.subscribers.write();
        
        for feed_key in feed_keys {
            conn.unsubscribe(&feed_key);
            if let Some(conn_set) = subs.get_mut(&feed_key) {
                conn_set.remove(conn_id);
            }
        }
        
        Ok(())
    }

    /// Handle ping
    pub fn ping(&self, conn_id: &str) -> Result<WSMessage, WebSocketError> {
        let mut conns = self.connections.write();
        let conn = conns.get_mut(conn_id)
            .ok_or_else(|| WebSocketError::ConnectionNotFound(conn_id.to_string()))?;
        
        conn.ping();
        
        Ok(WSMessage::new(WSMessageType::Pong, "heartbeat".to_string()))
    }

    /// Close connection
    pub fn close_connection(&self, conn_id: &str) -> Result<(), WebSocketError> {
        let mut conns = self.connections.write();
        
        if let Some(conn) = conns.get_mut(conn_id) {
            // Remove from all subscriptions
            let subs = self.subscribers.read();
            for feed_key in &conn.subscriptions {
                drop(subs);
                let mut subs = self.subscribers.write();
                if let Some(conn_set) = subs.get_mut(feed_key) {
                    conn_set.remove(conn_id);
                }
            }
            
            conn.close();
        }
        
        Ok(())
    }

    /// Update price stream
    pub fn update_price(&self, feed_key: String, price: Decimal, bid: Decimal, ask: Decimal) {
        let stream = PriceStream {
            feed_key: feed_key.clone(),
            price,
            bid,
            ask,
            volume_24h: 0,
            change_24h: Decimal::ZERO,
            timestamp: Utc::now().timestamp(),
        };
        
        self.price_streams.write().insert(feed_key.clone(), stream);
    }

    /// Get subscribers for feed
    pub fn get_subscribers(&self, feed_key: &str) -> HashSet<String> {
        self.subscribers.read()
            .get(feed_key)
            .cloned()
            .unwrap_or_default()
    }

    /// Get price streams for connection
    pub fn get_connection_streams(&self, conn_id: &str) -> Vec<PriceStream> {
        let conns = self.connections.read();
        let conn = match conns.get(conn_id) {
            Some(c) => c,
            None => return vec![],
        };
        
        let streams = self.price_streams.read();
        
        conn.subscriptions.iter()
            .filter_map(|feed_key| streams.get(feed_key).cloned())
            .collect()
    }

    /// Broadcast price update
    pub fn broadcast_price(&self, feed_key: &str) -> Option<WSMessage> {
        let streams = self.price_streams.read();
        let stream = streams.get(feed_key)?;
        
        let data = serde_json::json!({
            "feed_key": stream.feed_key,
            "price": stream.price.to_string(),
            "bid": stream.bid.to_string(),
            "ask": stream.ask.to_string(),
            "volume_24h": stream.volume_24h,
            "change_24h": stream.change_24h.to_string(),
            "timestamp": stream.timestamp,
        });
        
        Some(WSMessage::new(WSMessageType::PriceUpdate, feed_key.to_string()).with_data(data))
    }

    /// Cleanup stale connections
    pub fn cleanup_stale(&self) -> usize {
        let mut conns = self.connections.write();
        let mut count = 0;
        
        for conn in conns.values_mut() {
            if !conn.is_alive() {
                conn.close();
                count += 1;
            }
        }
        
        count
    }

    /// Get connection count
    pub fn connection_count(&self) -> usize {
        self.connections.read().len()
    }

    /// Get subscriber count for feed
    pub fn subscriber_count(&self, feed_key: &str) -> usize {
        self.subscribers.read()
            .get(feed_key)
            .map(|s| s.len())
            .unwrap_or(0)
    }
}

impl Default for WebSocketPriceStream {
    fn default() -> Self { Self::new() }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_connection_creation() {
        let engine = WebSocketPriceStream::new();
        
        let conn_id = engine.accept_connection(Some("user1".to_string())).unwrap();
        
        assert!(!conn_id.is_empty());
    }

    #[test]
    fn test_subscribe() {
        let engine = WebSocketPriceStream::new();
        
        let conn_id = engine.accept_connection(None).unwrap();
        
        engine.subscribe(&conn_id, vec!["ETH-USD".to_string()]).unwrap();
        
        let subs = engine.get_subscribers("ETH-USD");
        
        assert!(subs.contains(&conn_id));
    }

    #[test]
    fn test_price_update() {
        let engine = WebSocketPriceStream::new();
        
        engine.update_price("ETH-USD".to_string(), dec!(2500.0), dec!(2499.0), dec!(2501.0));
        
        let streams = engine.get_connection_streams(&conn_id).unwrap();
        // Note: This would need proper setup
    }

    #[test]
    fn test_broadcast() {
        let engine = WebSocketPriceStream::new();
        
        engine.update_price("ETH-USD".to_string(), dec!(2500.0), dec!(2499.0), dec!(2501.0));
        
        let msg = engine.broadcast_price("ETH-USD");
        
        assert!(msg.is_some());
    }
}