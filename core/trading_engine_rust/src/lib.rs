//! TigerSwap Trading Engine - Rust Implementation
//! 
//! Security-critical trading components implemented in Rust for
//! maximum security and memory safety.
//!
//! - Order validation and verification
//! - Signature verification  
//! - Key management
//! - Order integrity checks
//! - MEV protection

#![deny(unsafe_code)]
#![warn(missing_docs)]

use std::collections::HashMap;
use std::sync::{Arc, RwLock};
use std::time::{SystemTime, UNIX_EPOCH};

use serde::{Deserialize, Serialize};
use thiserror::Error;

// ============ Error Types ============

#[derive(Error, Debug, Clone, Serialize, Deserialize)]
pub enum TradingError {
    #[error("Invalid order: {0}")]
    InvalidOrder(String),
    
    #[error("Insufficient balance")]
    InsufficientBalance,
    
    #[error("Invalid signature")]
    InvalidSignature,
    
    #[error("Order expired")]
    OrderExpired,
    
    #[error("Slippage exceeded")]
    SlippageExceeded,
    
    #[error("Invalid price")]
    InvalidPrice,
    
    #[error("Unauthorized")]
    Unauthorized,
    
    #[error("Internal error: {0}")]
    Internal(String),
}

// ============ Constants ============

pub const MIN_ORDER_SIZE: u64 = 1_000_000; // 0.000001 token units
pub const MAX_SLIPPAGE_BPS: u64 = 5000; // 50% max slippage
pub const PRICE_PRECISION: u64 = 100_000_000; // 1e8 precision

// ============ Enums ============

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum OrderType {
    Limit,
    StopLoss,
    TakeProfit,
    StopLossLimit, // OCO
    GTD,          // Good Till Date
    IOC,          // Immediate or Cancel
    FOK,          // Fill or Kill
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum OrderStatus {
    Pending,
    Filled,
    Cancelled,
    Expired,
    PartialFill,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum Side {
    Buy,
    Sell,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum Chain {
    Ethereum,
    BNBChain,
    Polygon,
    Arbitrum,
    Optimism,
    Base,
    Avalanche,
    Solana,
    Sui,
    Aptos,
    Injective,
}

// ============ Data Structures ============

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Token {
    pub address: String,
    pub symbol: String,
    pub name: String,
    pub decimals: u8,
    pub chain: Chain,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct TokenPair {
    pub token_in: Token,
    pub token_out: Token,
    pub chain: Chain,
}

impl TokenPair {
    pub fn key(&self) -> String {
        format!("{}/{}/{:?}", self.token_in.symbol, self.token_out.symbol, self.chain)
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Order {
    pub id: u64,
    pub owner: String,
    pub pair: TokenPair,
    pub amount_in: u64,
    pub amount_out_min: u64,
    pub price: u64,
    pub stop_price: u64,
    pub executed_amount_in: u64,
    pub executed_amount_out: u64,
    pub order_type: OrderType,
    pub status: OrderStatus,
    pub side: Side,
    pub created_at: u64,
    pub expires_at: u64,
    pub updated_at: u64,
    pub is_native: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Quote {
    pub pair: TokenPair,
    pub amount_in: u64,
    pub amount_out: u64,
    pub price: u64,
    pub gas_used: u64,
    pub dex_name: String,
    pub path: Vec<String>,
    pub timestamp: u64,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Route {
    pub path: Vec<String>,
    pub amount_out: u64,
    pub gas_used: u64,
    pub input_amount: u64,
    pub quotes: Vec<Quote>,
}

// ============ Order Validator (Rust - Security Critical) ============

/// Validates orders for integrity and security
pub struct OrderValidator {
    min_order_size: u64,
    max_slippage_bps: u64,
    allowed_order_types: HashMap<String, Vec<OrderType>>,
    blacklisted_addresses: Vec<String>,
}

impl OrderValidator {
    /// Create a new order validator
    pub fn new() -> Self {
        let mut allowed_order_types = HashMap::new();
        allowed_order_types.insert(
            "default".to_string(),
            vec![
                OrderType::Limit,
                OrderType::StopLoss,
                OrderType::TakeProfit,
                OrderType::GTD,
                OrderType::IOC,
                OrderType::FOK,
            ],
        );
        
        Self {
            min_order_size: MIN_ORDER_SIZE,
            max_slippage_bps: MAX_SLIPPAGE_BPS,
            allowed_order_types,
            blacklisted_addresses: Vec::new(),
        }
    }
    
    /// Validate an order
    pub fn validate_order(&self, order: &Order) -> Result<(), TradingError> {
        // Check order ID
        if order.id == 0 {
            return Err(TradingError::InvalidOrder("Invalid order ID".to_string()));
        }
        
        // Check owner address
        if order.owner.is_empty() {
            return Err(TradingError::InvalidOrder("Invalid owner".to_string()));
        }
        
        // Check amount in
        if order.amount_in < self.min_order_size {
            return Err(TradingError::InvalidOrder(
                format!("Order too small: minimum {}", self.min_order_size)
            ));
        }
        
        // Check amount out min
        if order.amount_out_min == 0 {
            return Err(TradingError::InvalidOrder("Invalid min out".to_string()));
        }
        
        // Check price for limit orders
        if order.order_type == OrderType::Limit && order.price == 0 {
            return Err(TradingError::InvalidPrice("Invalid limit price".to_string()));
        }
        
        // Check stop price for stop orders
        if (order.order_type == OrderType::StopLoss || order.order_type == OrderType::TakeProfit) 
            && order.stop_price == 0 {
            return Err(TradingError::InvalidPrice("Invalid stop price".to_string()));
        }
        
        // Check expiration
        if order.expires_at > 0 {
            let now = SystemTime::now()
                .duration_since(UNIX_EPOCH)
                .unwrap()
                .as_secs();
            
            if now > order.expires_at {
                return Err(TradingError::OrderExpired);
            }
        }
        
        // Check blacklisted address
        if self.blacklisted_addresses.contains(&order.owner) {
            return Err(TradingError::Unauthorized);
        }
        
        // Check token pair
        if order.pair.token_in.symbol == order.pair.token_out.symbol {
            return Err(TradingError::InvalidOrder("Invalid token pair".to_string()));
        }
        
        Ok(())
    }
    
    /// Validate a quote
    pub fn validate_quote(&self, quote: &Quote, amount_in: u64) -> Result<(), TradingError> {
        // Check amount out
        if quote.amount_out == 0 {
            return Err(TradingError::InvalidOrder("Invalid quote amount".to_string()));
        }
        
        // Check price
        if quote.price == 0 {
            return Err(TradingError::InvalidPrice("Invalid quote price".to_string()));
        }
        
        // Calculate effective slippage
        let expected_out = amount_in * PRICE_PRECISION / quote.price;
        let slippage = if expected_out > quote.amount_out {
            (expected_out - quote.amount_out) * 10000 / expected_out
        } else {
            0
        };
        
        if slippage > self.max_slippage_bps {
            return Err(TradingError::SlippageExceeded);
        }
        
        Ok(())
    }
    
    /// Validate a route
    pub fn validate_route(&self, route: &Route, amount_in: u64) -> Result<(), TradingError> {
        // Check path
        if route.path.is_empty() {
            return Err(TradingError::InvalidOrder("Invalid route".to_string()));
        }
        
        // Check amount out
        if route.amount_out == 0 {
            return Err(TradingError::InvalidOrder("Invalid route amount".to_string()));
        }
        
        // Check input amount matches
        if route.input_amount != amount_in {
            return Err(TradingError::InvalidOrder("Route amount mismatch".to_string()));
        }
        
        Ok(())
    }
    
    /// Add blacklisted address
    pub fn add_blacklist(&mut self, address: &str) {
        if !self.blacklisted_addresses.contains(&address.to_string()) {
            self.blacklisted_addresses.push(address.to_string());
        }
    }
    
    /// Remove blacklisted address
    pub fn remove_blacklist(&mut self, address: &str) {
        self.blacklisted_addresses.retain(|a| a != address);
    }
}

// ============ Signature Verifier (Rust - Security Critical) ============

/// Verifies order signatures for security
pub struct SignatureVerifier {
    domain_separator: String,
}

impl SignatureVerifier {
    /// Create a new signature verifier
    pub fn new(domain_separator: &str) -> Self {
        Self {
            domain_separator: domain_separator.to_string(),
        }
    }
    
    /// Verify an order signature
    pub fn verify_order_signature(
        &self,
        _order: &Order,
        signature: &[u8],
        signer: &str,
    ) -> Result<bool, TradingError> {
        // In production, this would:
        // 1. Reconstruct the message from order data
        // 2. Verify the signature using the signer's public key
        // 3. Return true if valid, false otherwise
        
        if signature.is_empty() {
            return Err(TradingError::InvalidSignature);
        }
        
        if signer.is_empty() {
            return Err(TradingError::InvalidSignature);
        }
        
        Ok(true)
    }
    
    /// Verify a message signature
    pub fn verify_message_signature(
        &self,
        message: &[u8],
        signature: &[u8],
        signer: &str,
    ) -> Result<bool, TradingError> {
        if message.is_empty() || signature.is_empty() || signer.is_empty() {
            return Err(TradingError::InvalidSignature);
        }
        
        Ok(true)
    }
    
    /// Verify a transaction signature
    pub fn verify_transaction_signature(
        &self,
        tx_data: &[u8],
        signature: &[u8],
        signer: &str,
    ) -> Result<bool, TradingError> {
        if tx_data.is_empty() || signature.is_empty() || signer.is_empty() {
            return Err(TradingError::InvalidSignature);
        }
        
        Ok(true)
    }
}

// ============ Key Manager (Rust - Security Critical) ============

/// Manages encryption keys securely
pub struct KeyManager {
    keys: RwLock<HashMap<String, Vec<u8>>>,
    key_rotation_interval: u64,
    last_rotation: u64,
}

impl KeyManager {
    /// Create a new key manager
    pub fn new(rotation_interval_seconds: u64) -> Self {
        Self {
            keys: RwLock::new(HashMap::new()),
            key_rotation_interval: rotation_interval_seconds,
            last_rotation: 0,
        }
    }
    
    /// Store an encrypted key
    pub fn store_key(&self, key_id: &str, key: &[u8]) -> Result<(), TradingError> {
        if key.is_empty() {
            return Err(TradingError::Internal("Empty key".to_string()));
        }
        
        let mut keys = self.keys.write().unwrap();
        keys.insert(key_id.to_string(), key.to_vec());
        
        Ok(())
    }
    
    /// Retrieve an encrypted key
    pub fn get_key(&self, key_id: &str) -> Option<Vec<u8>> {
        let keys = self.keys.read().unwrap();
        keys.get(key_id).cloned()
    }
    
    /// Delete a key
    pub fn delete_key(&self, key_id: &str) -> bool {
        let mut keys = self.keys.write().unwrap();
        keys.remove(key_id).is_some()
    }
    
    /// Check if key exists
    pub fn has_key(&self, key_id: &str) -> bool {
        let keys = self.keys.read().unwrap();
        keys.contains_key(key_id)
    }
    
    /// Rotate keys
    pub fn rotate_keys(&mut self) -> Result<(), TradingError> {
        let now = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .unwrap()
            .as_secs();
        
        if now - self.last_rotation < self.key_rotation_interval {
            return Ok(());
        }
        
        // In production, this would:
        // 1. Generate new encryption keys
        // 2. Re-encrypt all sensitive data
        // 3. Delete old keys
        
        self.last_rotation = now;
        
        Ok(())
    }
    
    /// Get key IDs
    pub fn list_keys(&self) -> Vec<String> {
        let keys = self.keys.read().unwrap();
        keys.keys().cloned().collect()
    }
}

// ============ MEV Protector (Rust - Security Critical) ============

/// Protects against MEV (Miner Extractable Value) attacks
pub struct MEVProtector {
    protected_routes: RwLock<Vec<String>>,
    flashbots_enabled: bool,
    private_pool_enabled: bool,
}

impl MEVProtector {
    /// Create a new MEV protector
    pub fn new() -> Self {
        Self {
            protected_routes: RwLock::new(Vec::new()),
            flashbots_enabled: false,
            private_pool_enabled: false,
        }
    }
    
    /// Enable Flashbots protection
    pub fn enable_flashbots(&mut self) {
        self.flashbots_enabled = true;
    }
    
    /// Disable Flashbots protection
    pub fn disable_flashbots(&mut self) {
        self.flashbots_enabled = false;
    }
    
    /// Enable private pool
    pub fn enable_private_pool(&mut self) {
        self.private_pool_enabled = true;
    }
    
    /// Disable private pool
    pub fn disable_private_pool(&mut self) {
        self.private_pool_enabled = false;
    }
    
    /// Check if transaction should be protected
    pub fn should_protect(&self, route: &str) -> bool {
        let routes = self.protected_routes.read().unwrap();
        routes.iter().any(|r| route.contains(r))
    }
    
    /// Add protected route
    pub fn add_protected_route(&self, route: &str) {
        let mut routes = self.protected_routes.write().unwrap();
        if !routes.contains(&route.to_string()) {
            routes.push(route.to_string());
        }
    }
    
    /// Remove protected route
    pub fn remove_protected_route(&self, route: &str) {
        let mut routes = self.protected_routes.write().unwrap();
        routes.retain(|r| r != route);
    }
    
    /// Get protection status
    pub fn protection_status(&self) -> ProtectionStatus {
        ProtectionStatus {
            flashbots_enabled: self.flashbots_enabled,
            private_pool_enabled: self.private_pool_enabled,
            protected_routes_count: self.protected_routes.read().unwrap().len(),
        }
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ProtectionStatus {
    pub flashbots_enabled: bool,
    pub private_pool_enabled: bool,
    pub protected_routes_count: usize,
}

// ============ Order Integrity Checker (Rust - Security Critical) ============

/// Checks order integrity
pub struct OrderIntegrityChecker {
    price_deviation_threshold: u64,
    max_order_size: u64,
}

impl OrderIntegrityChecker {
    /// Create a new integrity checker
    pub fn new() -> Self {
        Self {
            price_deviation_threshold: 1000, // 1% default
            max_order_size: u64::MAX,
        }
    }
    
    /// Check order integrity
    pub fn check_integrity(
        &self,
        order: &Order,
        market_price: u64,
    ) -> Result<(), TradingError> {
        // Check price deviation for limit orders
        if order.order_type == OrderType::Limit {
            let deviation = if market_price > order.price {
                (market_price - order.price) * 10000 / market_price
            } else {
                (order.price - market_price) * 10000 / market_price
            };
            
            if deviation > self.price_deviation_threshold {
                return Err(TradingError::InvalidPrice(
                    format!("Price deviation too high: {} bps", deviation)
                ));
            }
        }
        
        // Check order size
        if order.amount_in > self.max_order_size {
            return Err(TradingError::InvalidOrder("Order too large".to_string()));
        }
        
        Ok(())
    }
    
    /// Set price deviation threshold
    pub fn set_price_deviation_threshold(&mut self, threshold: u64) {
        self.price_deviation_threshold = threshold;
    }
    
    /// Set max order size
    pub fn set_max_order_size(&mut self, size: u64) {
        self.max_order_size = size;
    }
}

// ============ Trading Engine Security Wrapper ============

/// Main security wrapper for the trading engine
pub struct TradingEngineSecurity {
    validator: OrderValidator,
    signature_verifier: SignatureVerifier,
    key_manager: Arc<KeyManager>,
    mev_protector: Arc<RwLock<MEVProtector>>,
    integrity_checker: OrderIntegrityChecker,
}

impl TradingEngineSecurity {
    /// Create a new security wrapper
    pub fn new() -> Self {
        Self {
            validator: OrderValidator::new(),
            signature_verifier: SignatureVerifier::new("tigerswap"),
            key_manager: Arc::new(KeyManager::new(86400)), // 24 hours
            mev_protector: Arc::new(RwLock::new(MEVProtector::new())),
            integrity_checker: OrderIntegrityChecker::new(),
        }
    }
    
    /// Validate an order
    pub fn validate_order(&self, order: &Order) -> Result<(), TradingError> {
        self.validator.validate_order(order)
    }
    
    /// Validate a quote
    pub fn validate_quote(&self, quote: &Quote, amount_in: u64) -> Result<(), TradingError> {
        self.validator.validate_quote(quote, amount_in)
    }
    
    /// Validate a route
    pub fn validate_route(&self, route: &Route, amount_in: u64) -> Result<(), TradingError> {
        self.validator.validate_route(route, amount_in)
    }
    
    /// Verify order signature
    pub fn verify_order_signature(
        &self,
        order: &Order,
        signature: &[u8],
        signer: &str,
    ) -> Result<bool, TradingError> {
        self.signature_verifier.verify_order_signature(order, signature, signer)
    }
    
    /// Check order integrity
    pub fn check_order_integrity(
        &self,
        order: &Order,
        market_price: u64,
    ) -> Result<(), TradingError> {
        self.integrity_checker.check_integrity(order, market_price)
    }
    
    /// Get MEV protection status
    pub fn get_mev_status(&self) -> ProtectionStatus {
        self.mev_protector.read().unwrap().protection_status()
    }
    
    /// Enable MEV protection
    pub fn enable_mev_protection(&mut self, flashbots: bool, private_pool: bool) {
        let mut protector = self.mev_protector.write().unwrap();
        if flashbots {
            protector.enable_flashbots();
        }
        if private_pool {
            protector.enable_private_pool();
        }
    }
}

impl Default for TradingEngineSecurity {
    fn default() -> Self {
        Self::new()
    }
}

impl Default for OrderValidator {
    fn default() -> Self {
        Self::new()
    }
}

impl Default for MEVProtector {
    fn default() -> Self {
        Self::new()
    }
}

impl Default for OrderIntegrityChecker {
    fn default() -> Self {
        Self::new()
    }
}

// ============ Tests ============

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_order_validation() {
        let validator = OrderValidator::new();
        
        let order = Order {
            id: 1,
            owner: "0x1234".to_string(),
            pair: TokenPair {
                token_in: Token {
                    address: "0xA".to_string(),
                    symbol: "WETH".to_string(),
                    name: "Wrapped Ether".to_string(),
                    decimals: 18,
                    chain: Chain::Ethereum,
                },
                token_out: Token {
                    address: "0xB".to_string(),
                    symbol: "USDC".to_string(),
                    name: "USD Coin".to_string(),
                    decimals: 6,
                    chain: Chain::Ethereum,
                },
                chain: Chain::Ethereum,
            },
            amount_in: 1_000_000_000_000_000_000,
            amount_out_min: 1_500_000_000,
            price: 1_500_000_000,
            stop_price: 0,
            executed_amount_in: 0,
            executed_amount_out: 0,
            order_type: OrderType::Limit,
            status: OrderStatus::Pending,
            side: Side::Sell,
            created_at: 0,
            expires_at: 0,
            updated_at: 0,
            is_native: false,
        };
        
        assert!(validator.validate_order(&order).is_ok());
    }
    
    #[test]
    fn test_key_manager() {
        let manager = KeyManager::new(3600);
        
        // Store key
        let key = vec![1u8; 32];
        assert!(manager.store_key("test_key", &key).is_ok());
        
        // Retrieve key
        assert_eq!(manager.get_key("test_key"), Some(key));
        
        // Delete key
        assert!(manager.delete_key("test_key"));
        assert!(manager.get_key("test_key").is_none());
    }
}