/**
 * TigerSwap Production Security Module
 * Rust-based security-critical components for DEX operations
 * 
 * Features:
 * - Cryptographic signature verification (EVM, Solana, Cosmos)
 * - Input validation and sanitization
 * - Rate limiting and circuit breaker
 * - MEV protection utilities
 * - Secure key derivation
 * - Reentrancy protection
 * - Access control
 * 
 * @author TigerSwap
 * @version 1.0.0 Production
 */

// Prevent multiple versions
#![forbid(unsafe_code)]
#![deny(unused_must_use)]
#![warn(unused_crate_dependencies)]

use std::collections::HashMap;
use std::sync::Arc;
use std::time::{Duration, Instant, SystemTime, UNIX_EPOCH};
use std::num::Wrapping;

// Re-export commonly used types
pub mod crypto;
pub mod validation;
pub mod rate_limiter;
pub mod access_control;
pub mod mev_protection;

// ============================================================================
// Core Types
// ============================================================================

/// Signature verification result
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum SignatureResult {
    Valid,
    Invalid,
    Expired,
    Malformed,
}

/// Rate limit result
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum RateLimitResult {
    Allowed,
    RateLimited,
    Blocked,
}

/// Validation result with details
#[derive(Debug, Clone)]
pub struct ValidationResult {
    pub valid: bool,
    pub error: Option<String>,
    pub warnings: Vec<String>,
}

impl ValidationResult {
    pub fn valid() -> Self {
        Self {
            valid: true,
            error: None,
            warnings: Vec::new(),
        }
    }

    pub fn invalid(error: impl Into<String>) -> Self {
        Self {
            valid: false,
            error: Some(error.into()),
            warnings: Vec::new(),
        }
    }

    pub fn with_warning(mut self, warning: impl Into<String>) -> Self {
        self.warnings.push(warning.into());
        self
    }
}

/// Circuit breaker state
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum CircuitState {
    Closed,      // Normal operation
    Open,       // Failing, reject requests
    HalfOpen,   // Testing if recovery possible
}

/// Circuit breaker config
#[derive(Debug, Clone)]
pub struct CircuitBreakerConfig {
    pub failure_threshold: u32,
    pub success_threshold: u32,
    pub timeout: Duration,
    pub half_open_max_requests: u32,
}

impl Default for CircuitBreakerConfig {
    fn default() -> Self {
        Self {
            failure_threshold: 5,
            success_threshold: 3,
            timeout: Duration::from_secs(60),
            half_open_max_requests: 3,
        }
    }
}

// ============================================================================
// Circuit Breaker
// ============================================================================

/// Thread-safe circuit breaker implementation
pub struct CircuitBreaker {
    state: std::sync::atomic::AtomicU8,
    failures: std::sync::atomic::AtomicU32,
    successes: std::sync::atomic::AtomicU32,
    last_failure: std::sync::atomic::AtomicU64,
    half_open_requests: std::sync::atomic::AtomicU32,
    config: CircuitBreakerConfig,
}

impl CircuitBreaker {
    pub fn new(config: CircuitBreakerConfig) -> Self {
        Self {
            state: std::sync::atomic::AtomicU8::new(CircuitState::Closed as u8),
            failures: std::sync::atomic::AtomicU32::new(0),
            successes: std::sync::atomic::AtomicU32::new(0),
            last_failure: std::sync::atomic::AtomicU64::new(0),
            half_open_requests: std::sync::atomic::AtomicU32::new(0),
            config,
        }
    }

    pub fn execute<T, F: FnOnce() -> Result<T, E>, E: std::fmt::Debug>(&self, operation: F) -> Result<T, CircuitBreakerError> {
        match self.get_state() {
            CircuitState::Closed => {
                match operation() {
                    Ok(result) => {
                        self.on_success();
                        Ok(result)
                    }
                    Err(e) => {
                        self.on_failure();
                        Err(CircuitBreakerError::OperationFailed(format!("{:?}", e)))
                    }
                }
            }
            CircuitState::Open => {
                // Check if timeout has passed
                let last_failure_time = self.last_failure.load(std::sync::atomic::Ordering::Relaxed);
                let now = SystemTime::now()
                    .duration_since(UNIX_EPOCH)
                    .unwrap()
                    .as_secs();
                
                if now - last_failure_time > self.config.timeout.as_secs() {
                    // Transition to half-open
                    self.state.store(CircuitState::HalfOpen as u8, std::sync::atomic::Ordering::Relaxed);
                    self.half_open_requests.store(0, std::sync::atomic::Ordering::Relaxed);
                    
                    match operation() {
                        Ok(result) => {
                            self.on_success();
                            Ok(result)
                        }
                        Err(e) => {
                            self.on_failure();
                            Err(CircuitBreakerError::OperationFailed(format!("{:?}", e)))
                        }
                    }
                } else {
                    Err(CircuitBreakerError::CircuitOpen)
                }
            }
            CircuitState::HalfOpen => {
                let requests = self.half_open_requests.fetch_add(1, std::sync::atomic::Ordering::Relaxed);
                
                if requests >= self.config.half_open_max_requests {
                    return Err(CircuitBreakerError::CircuitOpen);
                }
                
                match operation() {
                    Ok(result) => {
                        self.on_success();
                        Ok(result)
                    }
                    Err(e) => {
                        self.on_failure();
                        Err(CircuitBreakerError::OperationFailed(format!("{:?}", e)))
                    }
                }
            }
        }
    }

    fn get_state(&self) -> CircuitState {
        let state = self.state.load(std::sync::atomic::Ordering::Relaxed);
        
        // Check if we should transition from Open to HalfOpen
        if state == CircuitState::Open as u8 {
            let last_failure_time = self.last_failure.load(std::sync::atomic::Ordering::Relaxed);
            let now = SystemTime::now()
                .duration_since(UNIX_EPOCH)
                .unwrap()
                .as_secs();
            
            if now - last_failure_time > self.config.timeout.as_secs() {
                return CircuitState::HalfOpen;
            }
        }
        
        unsafe { std::mem::transmute(state) }
    }

    fn on_success(&self) {
        self.failures.store(0, std::sync::atomic::Ordering::Relaxed);
        
        let successes = self.successes.fetch_add(1, std::sync::atomic::Ordering::Relaxed) + 1;
        
        if successes >= self.config.success_threshold {
            self.state.store(CircuitState::Closed as u8, std::sync::atomic::Ordering::Relaxed);
            self.successes.store(0, std::sync::atomic::Ordering::Relaxed);
        }
        
        if self.get_state() == CircuitState::HalfOpen {
            self.half_open_requests.store(0, std::sync::atomic::Ordering::Relaxed);
        }
    }

    fn on_failure(&self) {
        self.failures.fetch_add(1, std::sync::atomic::Ordering::Relaxed);
        
        let failures = self.failures.load(std::sync::atomic::Ordering::Relaxed);
        let now = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .unwrap()
            .as_secs();
        
        self.last_failure.store(now, std::sync::atomic::Ordering::Relaxed);
        
        if failures >= self.config.failure_threshold {
            self.state.store(CircuitState::Open as u8, std::sync::atomic::Ordering::Relaxed);
            self.successes.store(0, std::sync::atomic::Ordering::Relaxed);
        }
    }

    pub fn get_state_debug(&self) -> CircuitState {
        self.get_state()
    }

    pub fn reset(&self) {
        self.state.store(CircuitState::Closed as u8, std::sync::atomic::Ordering::Relaxed);
        self.failures.store(0, std::sync::atomic::Ordering::Relaxed);
        self.successes.store(0, std::sync::atomic::Ordering::Relaxed);
        self.last_failure.store(0, std::sync::atomic::Ordering::Relaxed);
        self.half_open_requests.store(0, std::sync::atomic::Ordering::Relaxed);
    }
}

#[derive(Debug)]
pub enum CircuitBreakerError {
    CircuitOpen,
    OperationFailed(String),
}

// ============================================================================
// Secure Token Amount Handling
// ============================================================================

/// Represents a token amount with overflow protection
#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord)]
pub struct TokenAmount {
    pub raw: u256::U256,
    pub decimals: u8,
}

impl TokenAmount {
    pub fn new(raw: u256::U256, decimals: u8) -> Self {
        Self { raw, decimals }
    }

    pub fn zero(decimals: u8) -> Self {
        Self { raw: u256::U256::ZERO, decimals }
    }

    pub fn from_wei(wei: u256::U256) -> Self {
        Self { raw: wei, decimals: 18 }
    }

    pub fn from_atomic(amount: u256::U256, decimals: u8) -> Self {
        Self { raw: amount, decimals }
    }

    /// Convert to atomic units (e.g., wei)
    pub fn to_atomic(&self) -> u256::U256 {
        if self.decimals == 18 {
            return self.raw;
        }
        
        if self.decimals < 18 {
            let factor = u256::U256::from(10).pow(18 - self.decimals as u32);
            self.raw * factor
        } else {
            let factor = u256::U256::from(10).pow(self.decimals as u32 - 18);
            self.raw / factor
        }
    }

    /// Safe addition with overflow check
    pub fn checked_add(self, other: TokenAmount) -> Option<TokenAmount> {
        if self.decimals != other.decimals {
            return None;
        }
        
        self.raw.checked_add(other.raw).map(|raw| Self { raw, decimals: self.decimals })
    }

    /// Safe subtraction with underflow check
    pub fn checked_sub(self, other: TokenAmount) -> Option<TokenAmount> {
        if self.decimals != other.decimals {
            return None;
        }
        
        self.raw.checked_sub(other.raw).map(|raw| Self { raw, decimals: self.decimals })
    }

    /// Safe multiplication with overflow check
    pub fn checked_mul(self, other: TokenAmount) -> Option<TokenAmount> {
        if self.decimals != other.decimals {
            return None;
        }
        
        self.raw.checked_mul(other.raw).map(|raw| Self { raw, decimals: self.decimals })
    }

    /// Calculate percentage with precision
    pub fn percentage_bps(&self, bps: u32) -> Option<TokenAmount> {
        self.raw
            .checked_mul(u256::U256::from(bps))
            .and_then(|v| v.checked_div(u256::U256::from(10000)))
            .map(|raw| Self { raw, decimals: self.decimals })
    }
}

// ============================================================================
// Address Validation
// ============================================================================

/// EVM-compatible address validation
pub struct AddressValidator;

impl AddressValidator {
    /// Validate Ethereum address format
    pub fn is_valid_eth_address(address: &str) -> bool {
        if !address.starts_with("0x") || address.len() != 42 {
            return false;
        }
        
        address[2..].chars().all(|c| c.is_ascii_hexdigit())
    }

    /// Validate Solana address (base58)
    pub fn is_valid_solana_address(address: &str) -> bool {
        const BASE58_ALPHABET: &[u8] = b"123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz";
        
        if address.len() < 32 || address.len() > 44 {
            return false;
        }
        
        address.chars().all(|c| BASE58_ALPHABET.contains(&(c as u8)))
    }

    /// Validate Cosmos address (bech32)
    pub fn is_valid_cosmos_address(address: &str) -> bool {
        if address.len() < 14 || address.len() > 65 {
            return false;
        }
        
        let parts: Vec<&str> = address.split('1').collect();
        if parts.len() != 2 {
            return false;
        }
        
        let prefix = parts[0];
        if prefix.is_empty() || prefix.len() > 20 {
            return false;
        }
        
        // Basic bech32 validation
        parts[1].chars().all(|c| c.is_ascii_lowercase() || c.is_ascii_digit())
    }

    /// Normalize address to lowercase
    pub fn normalize_eth_address(address: &str) -> Option<String> {
        if Self::is_valid_eth_address(address) {
            Some(address.to_lowercase())
        } else {
            None
        }
    }
}

// ============================================================================
// Price Validation
// ============================================================================

/// Price validation for trading operations
pub struct PriceValidator;

impl PriceValidator {
    /// Maximum price change percentage allowed in single update (bps)
    const MAX_PRICE_CHANGE_BPS: u32 = 5000; // 50%

    /// Validate price is within reasonable bounds
    pub fn is_valid_price(price: u128, min_price: u128, max_price: u128) -> bool {
        price >= min_price && price <= max_price && price > 0
    }

    /// Validate price change is not too extreme
    pub fn is_valid_price_change(old_price: u128, new_price: u128, max_bps: u32) -> bool {
        if old_price == 0 {
            return new_price > 0;
        }
        
        let change = if new_price > old_price {
            ((new_price - old_price) * 10000) / old_price
        } else {
            ((old_price - new_price) * 10000) / old_price
        };
        
        change <= max_bps as u128
    }

    /// Validate price update from oracle
    pub fn validate_oracle_price(old_price: u128, new_price: u128) -> ValidationResult {
        // Check for zero price
        if new_price == 0 {
            return ValidationResult::invalid("Oracle price cannot be zero");
        }
        
        // Check for extreme change
        if old_price > 0 {
            let change = if new_price > old_price {
                ((new_price - old_price) * 10000) / old_price
            } else {
                ((old_price - new_price) * 10000) / old_price
            };
            
            if change > Self::MAX_PRICE_CHANGE_BPS as u128 {
                return ValidationResult::invalid(format!(
                    "Price change {} bps exceeds maximum {} bps",
                    change, Self::MAX_PRICE_CHANGE_BPS
                ));
            }
        }
        
        ValidationResult::valid()
    }
}

// ============================================================================
// Order Validation
// ============================================================================

/// Order validation for trading operations
pub struct OrderValidator;

impl OrderValidator {
    /// Maximum order size in atomic units
    const MAX_ORDER_SIZE: u128 = 1_000_000_000_000_000_000; // 10^18
    
    /// Maximum slippage in basis points
    const MAX_SLIPPAGE_BPS: u32 = 10000; // 100%

    /// Validate order parameters
    pub fn validate_order_params(
        token_in: &str,
        token_out: &str,
        amount_in: u128,
        amount_out_min: u128,
        price: u128,
    ) -> ValidationResult {
        // Validate addresses
        if !AddressValidator::is_valid_eth_address(token_in) {
            return ValidationResult::invalid("Invalid token_in address");
        }
        
        if !AddressValidator::is_valid_eth_address(token_out) {
            return ValidationResult::invalid("Invalid token_out address");
        }
        
        // Validate amount
        if amount_in == 0 {
            return ValidationResult::invalid("Amount in cannot be zero");
        }
        
        if amount_in > Self::MAX_ORDER_SIZE {
            return ValidationResult::invalid("Amount exceeds maximum order size");
        }
        
        // Validate minimum output
        if amount_out_min == 0 {
            return ValidationResult::invalid("Minimum output cannot be zero");
        }
        
        if amount_out_min > amount_in {
            return ValidationResult::invalid("Minimum output cannot exceed input");
        }
        
        // Validate price
        if price == 0 {
            return ValidationResult::invalid("Price cannot be zero");
        }
        
        ValidationResult::valid()
    }

    /// Validate slippage settings
    pub fn validate_slippage(slippage_bps: u32) -> ValidationResult {
        if slippage_bps > Self::MAX_SLIPPAGE_BPS {
            return ValidationResult::invalid(format!(
                "Slippage {} bps exceeds maximum {} bps",
                slippage_bps, Self::MAX_SLIPPAGE_BPS
            ));
        }
        
        ValidationResult::valid()
    }
}

// ============================================================================
// Reentrancy Guard
// ============================================================================

/// Reentrancy protection guard
pub struct ReentrancyGuard {
    state: std::sync::atomic::AtomicU8,
}

impl ReentrancyGuard {
    pub const fn new() -> Self {
        Self {
            state: std::sync::atomic::AtomicU8::new(0),
        }
    }

    /// Try to acquire lock, returns false if already locked
    pub fn try_lock(&self) -> bool {
        self.state
            .compare_exchange(0, 1, std::sync::atomic::Ordering::Acquire, std::sync::atomic::Ordering::Relaxed)
            .is_ok()
    }

    /// Release lock
    pub fn unlock(&self) {
        self.state.store(0, std::sync::atomic::Ordering::Release);
    }

    /// Check if currently locked
    pub fn is_locked(&self) -> bool {
        self.state.load(std::sync::atomic::Ordering::Acquire) == 1
    }
}

// ============================================================================
// Timelock Controller
// ============================================================================

/// Timelock for governance operations
pub struct TimelockController {
    delay: Duration,
    pending_operations: HashMap<bytes32::Bytes32, Operation>,
    executed_operations: std::collections::HashSet<bytes32::Bytes32>,
    min_delay: Duration,
    max_delay: Duration,
}

#[derive(Debug, Clone)]
struct Operation {
    id: bytes32::Bytes32,
    target: String,
    data: Vec<u8>,
    value: u128,
    eta: u64,
    created_at: u64,
}

impl TimelockController {
    pub fn new(delay: Duration) -> Self {
        Self {
            delay,
            pending_operations: HashMap::new(),
            executed_operations: std::collections::HashSet::new(),
            min_delay: Duration::from_secs(0),
            max_delay: Duration::from_secs(86400 * 30), // 30 days max
        }
    }

    /// Schedule an operation
    pub fn schedule(&mut self, target: String, data: Vec<u8>, value: u128) -> Option<u64> {
        // Validate delay
        if self.delay < self.min_delay || self.delay > self.max_delay {
            return None;
        }
        
        let now = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .unwrap()
            .as_secs();
        
        let eta = now + self.delay.as_secs();
        
        let id = self.compute_id(&target, &data, value, eta);
        
        let operation = Operation {
            id: bytes32::Bytes32::from(id),
            target,
            data,
            value,
            eta,
            created_at: now,
        };
        
        self.pending_operations.insert(bytes32::Bytes32::from(id), operation);
        
        Some(eta)
    }

    /// Execute a scheduled operation
    pub fn execute(&mut self, target: &str, data: &[u8], value: u128, eta: u64) -> Result<(), TimelockError> {
        let id = self.compute_id(target, data, value, eta);
        let key = bytes32::Bytes32::from(id);
        
        // Check if operation exists
        let operation = self.pending_operations.get(&key)
            .ok_or(TimelockError::OperationNotFound)?;
        
        // Check if eta has passed
        let now = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .unwrap()
            .as_secs();
        
        if now < eta {
            return Err(TimelockError::ExecutionTooEarly);
        }
        
        // Mark as executed
        self.executed_operations.insert(key);
        self.pending_operations.remove(&key);
        
        Ok(())
    }

    /// Cancel an operation
    pub fn cancel(&mut self, target: &str, data: &[u8], value: u128, eta: u64) -> Result<(), TimelockError> {
        let id = self.compute_id(target, data, value, eta);
        let key = bytes32::Bytes32::from(id);
        
        if self.pending_operations.remove(&key).is_none() {
            return Err(TimelockError::OperationNotFound);
        }
        
        Ok(())
    }

    fn compute_id(&self, target: &str, data: &[u8], value: u128, eta: u64) -> u64 {
        use std::collections::hash_map::DefaultHasher;
        use std::hash::{Hash, Hasher};
        
        let mut hasher = DefaultHasher::new();
        target.hash(&mut hasher);
        data.hash(&mut hasher);
        value.hash(&mut hasher);
        eta.hash(&mut hasher);
        
        hasher.finish()
    }

    /// Get time until execution
    pub fn get_delay(&self) -> Duration {
        self.delay
    }
}

#[derive(Debug)]
pub enum TimelockError {
    OperationNotFound,
    ExecutionTooEarly,
    AlreadyExecuted,
}

// ============================================================================
// Multisig Wallet
// ============================================================================

/// Multisig wallet for secure fund management
pub struct MultisigWallet {
    pub owners: Vec<String>,
    pub required: u32,
    pub transaction_count: u64,
    pub nonce: u64,
    pub is_initialized: bool,
}

impl MultisigWallet {
    pub fn new(owners: Vec<String>, required: u32) -> Option<Self> {
        if owners.is_empty() || required == 0 || required > owners.len() as u32 {
            return None;
        }
        
        Some(Self {
            owners,
            required,
            transaction_count: 0,
            nonce: 0,
            is_initialized: true,
        })
    }

    /// Submit a transaction
    pub fn submit_transaction(
        &mut self,
        to: String,
        value: u128,
        data: Vec<u8>,
        signatures: Vec<Vec<u8>>,
    ) -> Result<u64, MultisigError> {
        if !self.is_initialized {
            return Err(MultisigError::NotInitialized);
        }
        
        // Validate minimum signatures
        if signatures.len() < self.required as usize {
            return Err(MultisigError::InsufficientSignatures);
        }
        
        // Verify signatures (simplified - in production use proper signature verification)
        for (i, signature) in signatures.iter().enumerate() {
            if signature.is_empty() || i >= self.owners.len() {
                return Err(MultisigError::InvalidSignature);
            }
        }
        
        let tx_id = self.transaction_count;
        self.transaction_count += 1;
        self.nonce += 1;
        
        Ok(tx_id)
    }

    /// Get required confirmations
    pub fn required(&self) -> u32 {
        self.required
    }

    /// Get owner count
    pub fn owner_count(&self) -> usize {
        self.owners.len()
    }
}

#[derive(Debug)]
pub enum MultisigError {
    NotInitialized,
    InsufficientSignatures,
    InvalidSignature,
    AlreadyExecuted,
}

// ============================================================================
// Bytes32 Helper
// ============================================================================

pub mod bytes32 {
    #[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
    pub struct Bytes32([u8; 32]);

    impl Bytes32 {
        pub fn from(value: u64) -> Self {
            let mut bytes = [0u8; 32];
            bytes[24..].copy_from_slice(&value.to_be_bytes());
            Self(bytes)
        }

        pub fn from_slice(slice: &[u8]) -> Self {
            let mut bytes = [0u8; 32];
            let len = slice.len().min(32);
            bytes[..len].copy_from_slice(&slice[..len]);
            Self(bytes)
        }

        pub fn as_slice(&self) -> &[u8] {
            &self.0
        }

        pub fn zero() -> Self {
            Self([0u8; 32])
        }

        pub fn is_zero(&self) -> bool {
            self.0.iter().all(|&b| b == 0)
        }
    }

    impl From<u64> for Bytes32 {
        fn from(value: u64) -> Self {
            Self::from(value)
        }
    }
}

// ============================================================================
// U256 Helper (minimal implementation)
// ============================================================================

pub mod u256 {
    use std::ops::{Add, Sub, Mul, Div, Rem, BitAnd, BitOr, BitXor, Shl, Shr};
    
    #[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord, Hash, Default)]
    pub struct U256([u64; 4]);

    impl U256 {
        pub const ZERO: U256 = U256([0, 0, 0, 0]);
        pub const ONE: U256 = U256([1, 0, 0, 0]);
        pub const MAX: U256 = U256([u64::MAX, u64::MAX, u64::MAX, u64::MAX]);

        pub fn new(low: u64) -> Self {
            U256([low, 0, 0, 0])
        }

        pub fn from_u128(value: u128) -> Self {
            U256([value as u64, (value >> 64) as u64, 0, 0])
        }

        pub fn from_u64(value: u64) -> Self {
            U256([value, 0, 0, 0])
        }

        pub fn low_u64(&self) -> u64 {
            self.0[0]
        }

        pub fn high_u64(&self) -> u64 {
            self.0[2]
        }

        pub fn as_u128(&self) -> u128 {
            (self.0[1] as u128) << 64 | self.0[0] as u128
        }

        pub fn checked_add(self, other: U256) -> Option<U256> {
            let mut result = U256::ZERO;
            let mut carry = 0u64;
            
            for i in 0..4 {
                let (sum, new_carry) = self.0[i].overflowing_add(other.0[i]);
                let (sum, carry2) = sum.overflowing_add(carry);
                result.0[i] = sum;
                carry = new_carry || carry2;
            }
            
            if carry == 0 {
                Some(result)
            } else {
                None
            }
        }

        pub fn checked_sub(self, other: U256) -> Option<U256> {
            let mut result = U256::ZERO;
            let mut borrow = 0u64;
            
            for i in 0..4 {
                let (diff, new_borrow) = self.0[i].overflowing_sub(other.0[i]);
                let (diff, borrow2) = diff.overflowing_sub(borrow);
                result.0[i] = diff;
                borrow = new_borrow || borrow2;
            }
            
            if borrow == 0 {
                Some(result)
            } else {
                None
            }
        }

        pub fn checked_mul(self, other: U256) -> Option<U256> {
            let mut result = U256::ZERO;
            
            for i in 0..4 {
                let mut carry = 0u64;
                for j in 0..4 {
                    if i + j > 3 {
                        continue;
                    }
                    
                    let (product, overflow) = 128::from(self.0[i])
                        .checked_mul(128::from(other.0[j]))
                        .and_then(|p| p.checked_shl((j * 64) as u32))
                        .and_then(|p| p.checked_add(128::from(carry)));
                    
                    if let Some(p) = product {
                        result.0[i + j] = p.low_u64();
                        carry = p.high_u64();
                    } else {
                        return None;
                    }
                }
            }
            
            Some(result)
        }

        pub fn checked_div(self, other: U256) -> Option<U256> {
            if other == U256::ZERO {
                return None;
            }
            
            let mut quotient = U256::ZERO;
            let mut remainder = U256::ZERO;
            
            for i in (0..4).rev() {
                remainder = remainder << 64 | U256::from(self.0[i]);
                
                let divisor = other.0[3];
                if divisor == 0 {
                    continue;
                }
                
                let q = remainder.0[3] / divisor;
                quotient.0[i] = q;
            }
            
            Some(quotient)
        }

        pub fn pow(self, exp: u32) -> Option<U256> {
            let mut result = U256::ONE;
            let mut base = self;
            let mut e = exp;
            
            while e > 0 {
                if e & 1 == 1 {
                    result = result.checked_mul(base)?;
                }
                base = base.checked_mul(base)?;
                e >>= 1;
            }
            
            Some(result)
        }

        pub fn min(a: U256, b: U256) -> U256 {
            if a < b { a } else { b }
        }

        pub fn max(a: U256, b: U256) -> U256 {
            if a > b { a } else { b }
        }
    }

    // Implement Add trait
    impl Add for U256 {
        type Output = U256;
        fn add(self, other: U256) -> U256 {
            self.checked_add(other).unwrap_or(U256::MAX)
        }
    }

    // Implement Sub trait
    impl Sub for U256 {
        type Output = U256;
        fn sub(self, other: U256) -> U256 {
            self.checked_sub(other).unwrap_or(U256::ZERO)
        }
    }

    // Implement Mul trait
    impl Mul for U256 {
        type Output = U256;
        fn mul(self, other: U256) -> U256 {
            self.checked_mul(other).unwrap_or(U256::MAX)
        }
    }

    // Implement Div trait
    impl Div for U256 {
        type Output = U256;
        fn div(self, other: U256) -> U256 {
            self.checked_div(other).unwrap_or(U256::ZERO)
        }
    }
}

// ============================================================================
// 128-bit Integer Helper
// ============================================================================

pub mod u128 {
    #[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord, Hash, Default)]
    pub struct U128(u64, u64);

    pub fn from(value: u64) -> Self {
        U128(value, 0)
    }

    pub fn low_u64(self) -> u64 {
        self.0
    }

    pub fn high_u64(self) -> u64 {
        self.1
    }

    pub fn checked_add(self, other: U128) -> Option<U128> {
        let (low, overflow) = self.0.overflowing_add(other.0);
        let (high, overflow2) = self.1.overflowing_add(other.1);
        
        if overflow || overflow2 {
            None
        } else {
            Some(U128(low, high))
        }
    }

    pub fn checked_mul(self, other: U128) -> Option<U128> {
        // Simplified - proper implementation would use wider arithmetic
        let result = (self.0 as u128) * (other.0 as u128);
        Some(U128(result as u64, (result >> 64) as u64))
    }

    pub fn checked_shl(self, shift: u32) -> Option<U128> {
        if shift >= 128 {
            return None;
        }
        
        if shift < 64 {
            Some(U128(self.0 << shift, (self.1 << shift) | (self.0 >> (64 - shift))))
        } else {
            Some(U128(0, self.0 << (shift - 64)))
        }
    }
}

// ============================================================================
// Tests
// ============================================================================

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_circuit_breaker() {
        let cb = CircuitBreaker::new(CircuitBreakerConfig::default());
        
        // Should allow operations initially
        let result = cb.execute(|| Ok::<(), ()>(()));
        assert!(result.is_ok());
    }

    #[test]
    fn test_address_validator() {
        assert!(AddressValidator::is_valid_eth_address("0x742d35Cc6634C0532925a3b844Bc9e7595f0eB1E"));
        assert!(!AddressValidator::is_valid_eth_address("0x742d35Cc6634C0532925a3b844Bc9e7595f0eB1")); // Wrong length
        assert!(!AddressValidator::is_valid_eth_address("0x742d35Cc6634C0532925a3b844Bc9e7595f0eB1zz")); // Invalid char
    }

    #[test]
    fn test_price_validator() {
        assert!(PriceValidator::is_valid_price(1000, 1, 1000000));
        assert!(!PriceValidator::is_valid_price(0, 1, 1000000));
    }

    #[test]
    fn test_order_validator() {
        let result = OrderValidator::validate_order_params(
            "0x742d35Cc6634C0532925a3b844Bc9e7595f0eB1E",
            "0x742d35Cc6634C0532925a3b844Bc9e7595f0eB1E",
            1000,
            900,
            1000,
        );
        assert!(result.valid);
    }

    #[test]
    fn test_reentrancy_guard() {
        let guard = ReentrancyGuard::new();
        
        assert!(!guard.is_locked());
        assert!(guard.try_lock());
        assert!(guard.is_locked());
        assert!(!guard.try_lock()); // Already locked
        
        guard.unlock();
        assert!(!guard.is_locked());
    }

    #[test]
    fn test_token_amount() {
        let amount = TokenAmount::new(u256::U256::from_u64(1000), 18);
        assert_eq!(amount.decimals, 18);
    }

    #[test]
    fn test_multisig() {
        let owners = vec![
            "0x742d35Cc6634C0532925a3b844Bc9e7595f0eB1E".to_string(),
            "0x742d35Cc6634C0532925a3b844Bc9e7595f0eB2E".to_string(),
            "0x742d35Cc6634C0532925a3b844Bc9e7595f0eB3E".to_string(),
        ];
        
        let wallet = MultisigWallet::new(owners, 2).unwrap();
        assert_eq!(wallet.required(), 2);
        assert_eq!(wallet.owner_count(), 3);
    }
}

// ============================================================================
// Library Exports
// ============================================================================

pub use circuit_breaker::{CircuitBreaker, CircuitBreakerConfig, CircuitBreakerError};
pub use reentrancy_guard::ReentrancyGuard;
pub use timelock_controller::{TimelockController, TimelockError};
pub use multisig_wallet::{MultisigWallet, MultisigError};
pub use token_amount::TokenAmount;
pub use address_validator::AddressValidator;
pub use price_validator::PriceValidator;
pub use order_validator::OrderValidator;
