//! TigerSwap StableSwap Engine
//! 
//! Implements Curve-style stable swap for stablecoin swapping:
//! - StableSwap (x*y^k + y*x^k = k) invariant
//! - Native stablecoin support
//! - Low slippage trading
//! - Crypto-swap for assets with different decimals
//!
//! Implementation: Pure Rust with no external dependencies

use serde::{Deserialize, Serialize};
use std::sync::Arc;
use parking_lot::RwLock;
use rust_decimal::Decimal;
use thiserror::Error;
use uuid::Uuid;
use chrono::Utc;
use std::collections::HashMap;

/// Chain ID constants
pub const CHAIN_ETH: u64 = 1;
pub const CHAIN_BSC: u64 = 56;
pub const CHAIN_POLYGON: u64 = 137;
pub const CHAIN_ARBITRUM: u64 = 42161;
pub const CHAIN_OPTIMISM: u64 = 10;
pub const CHAIN_BASE: u64 = 8453;
pub const CHAIN_AVALANCHE: u64 = 43114;

#[derive(Debug, Error)]
pub enum StableSwapError {
    #[error("Pool not found: {0}")]
    PoolNotFound(String),
    #[error("Insufficient liquidity: {0}")]
    InsufficientLiquidity(String),
    #[error("Insufficient balance: {0}")]
    InsufficientBalance(String),
    #[error("Invalid amount: {0}")]
    InvalidAmount(String),
    #[error("Price impact too high: {0}")]
    PriceImpactTooHigh(String),
    #[error("Slippage exceeded")]
    SlippageExceeded,
    #[error("Chain not supported: {0}")]
    ChainNotSupported(u64),
    #[error("Token not found: {0}")]
    TokenNotFound(String),
}

/// Pool type
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum PoolType {
    Plain,        // Basic stable swap
    Crypto,       // For assets with different decimals
    Lending,      // Lending pool
    StableNg,     // New stable swap algorithm
}

impl Default for PoolType {
    fn default() -> Self { PoolType::Plain }
}

/// StableSwap pool
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct StableSwapPool {
    pub pool_id: String,
    pub chain_id: u64,
    pub name: String,
    pub tokens: Vec<TokenInfo>,
    pub pool_type: PoolType,
    
    // Balances
    pub balances: Vec<u128>,
    
    // Amplification coefficient (A)
    pub amplification: u256,
    
    // Fee
    pub fee: u256,           // Trading fee (in basis points)
    pub admin_fee: u256,      // Admin fee share
    pub withdraw_fee: u256, // Withdrawal fee
    
    // Virtual price (for calculating LP value)
    pub virtual_price: u256,
    
    // Cumulative prices
    pub cumulative_prices: Vec<u256>,
    
    // State
    pub total_supply: u128,
    pub is_paused: bool,
    
    pub created_at: i64,
    pub updated_at: i64,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct TokenInfo {
    pub address: String,
    pub decimals: u8,
    pub symbol: String,
}

impl StableSwapPool {
    /// Create a new stable swap pool
    pub fn new(
        chain_id: u64,
        name: String,
        tokens: Vec<TokenInfo>,
        amplification: u256,
        fee: u256,
    ) -> Self {
        let balances = vec![0u128; tokens.len()];
        let cumulative_prices = vec![0u256; tokens.len()];
        
        Self {
            pool_id: Uuid::new_v4().to_string(),
            chain_id,
            name,
            tokens,
            pool_type: PoolType::Plain,
            balances,
            amplification,
            fee,
            admin_fee: 50,       // 50% of fee goes to admin
            withdraw_fee: 0,
            virtual_price: 1e18, // 1.0 initial
            cumulative_prices,
            total_supply: 0,
            is_paused: false,
            created_at: Utc::now().timestamp(),
            updated_at: Utc::now().timestamp(),
        }
    }

    /// Create a crypto pool
    pub fn new_crypto(chain_id: u64, name: String, tokens: Vec<TokenInfo>) -> Self {
        let mut pool = Self::new(chain_id, name, tokens, 1000, 4); // Lower fee for crypto
        pool.pool_type = PoolType::Crypto;
        pool
    }

    /// Add liquidity
    pub fn add_liquidity(&mut self, amounts: Vec<u128>, min_mint_amount: u128) -> Result<u128, StableSwapError> {
        let total_supply = self.total_supply;
        let num_tokens = self.tokens.len();
        
        // Calculate minimum amounts
        let mut min_amounts = vec![0u128; num_tokens];
        for i in 0..num_tokens {
            if total_supply == 0 {
                min_amounts[i] = amounts[i];
            } else {
                min_amounts[i] = amounts[i] * total_supply / self.balances[i] + 1;
            }
        }
        
        // Calculate LP tokens to mint
        let mint_amount = if total_supply == 0 {
            // Initial mint - use geometric mean
            let mut product = 1u128;
            for amount in &amounts {
                product = product.saturating_mul(*amount);
            }
            product = product.pow(num_tokens as u32).pow(1);
            
            // Apply initial liquidity discount
            product * 10 / 100
        } else {
            // Calculate based on virtual price
            let mut sum = 0u128;
            for i in 0..num_tokens {
                sum += amounts[i] * total_supply / self.balances[i];
            }
            sum * total_supply / num_tokens as u128
        };
        
        if mint_amount < min_mint_amount {
            return Err(StableSwapError::SlippageExceeded);
        }
        
        // Update balances
        for i in 0..num_tokens {
            self.balances[i] += amounts[i];
        }
        
        // Update virtual price
        self.update_virtual_price();
        self.total_supply += mint_amount;
        self.updated_at = Utc::now().timestamp();
        
        Ok(mint_amount)
    }

    /// Remove liquidity
    pub fn remove_liquidity(&mut self, burn_amount: u128, min_amounts: Vec<u128>) -> Result<Vec<u128>, StableSwapError> {
        if burn_amount > self.total_supply {
            return Err(StableSwapError::InsufficientBalance("Burn amount exceeds supply".to_string()));
        }
        
        let num_tokens = self.tokens.len();
        let mut amounts = vec![0u128; num_tokens];
        
        // Calculate amounts to receive
        for i in 0..num_tokens {
            amounts[i] = self.balances[i] * burn_amount / self.total_supply;
            
            if amounts[i] < min_amounts[i] {
                return Err(StableSwapError::SlippageExceeded);
            }
            
            self.balances[i] -= amounts[i];
        }
        
        self.total_supply -= burn_amount;
        self.update_virtual_price();
        self.updated_at = Utc::now().timestamp();
        
        Ok(amounts)
    }

    /// Get dy (output amount for input)
    pub fn get_dy(&self, input_index: usize, output_index: usize, dx: u128) -> Result<u128, StableSwapError> {
        if input_index >= self.tokens.len() || output_index >= self.tokens.len() {
            return Err(StableSwapError::TokenNotFound("Invalid token index".to_string()));
        }
        
        if dx == 0 {
            return Ok(0);
        }
        
        // Calculate output using StableSwap formula
        let output = self.calculate_dy(input_index, output_index, dx);
        
        // Apply fee
        let fee = (output * self.fee as u128) / 10000;
        
        Ok(output - fee)
    }

    /// Exchange tokens
    pub fn exchange(
        &mut self,
        input_index: usize,
        output_index: usize,
        dx: u128,
        min_dy: u128,
    ) -> Result<u128, StableSwapError> {
        if input_index >= self.tokens.len() || output_index >= self.tokens.len() {
            return Err(StableSwapError::TokenNotFound("Invalid token index".to_string()));
        }
        
        if dx == 0 {
            return Err(StableSwapError::InvalidAmount("Input amount is 0".to_string()));
        }
        
        // Calculate output
        let dy = self.calculate_dy(input_index, output_index, dx);
        
        // Apply fee
        let fee = (dy * self.fee as u128) / 10000;
        let dy_after_fee = dy - fee;
        
        if dy_after_fee < min_dy {
            return Err(StableSwapError::SlippageExceeded);
        }
        
        // Update balances
        self.balances[input_index] += dx;
        self.balances[output_index] = self.balances[output_index].saturating_sub(dy);
        
        // Update cumulative price
        self.cumulative_prices[output_index] += dy_after_fee * 1e18 / dx;
        
        self.updated_at = Utc::now().timestamp();
        
        Ok(dy_after_fee)
    }

    /// Calculate dy using StableSwap formula
    fn calculate_dy(&self, input_index: usize, output_index: usize, dx: u128) -> u128 {
        let num_tokens = self.tokens.len() as u256;
        
        match self.pool_type {
            PoolType::Plain | PoolType::StableNg => {
                // StableSwap: x*y = k (generalized)
                let xp = self.get_xp();
                let x_p = xp[input_index] + dx;
                
                // Calculate new y using Newton's method
                let mut y = xp[output_index];
                let mut D = self.get_D(&xp);
                
                // Amplification coefficient
                let A = self.amplification;
                
                // Iterate to find y
                for _ in 0..255 {
                    let prev_y = y;
                    let S = num_tokens * y;
                    let k = y * y * num_tokens;
                    
                    for j in 1..num_tokens as usize {
                        if j != input_index {
                            let x_j = xp[j];
                            k /=
                            (x_j * num_tokens + S) / (num_tokens * x_j);
                        }
                    }
                    
                    y = (D * D / (k * x_p) + D * num_tokens - D) / 
                        ((num_tokens + 1) * D / (k) + num_tokens - 1);
                    
                    if y > prev_y {
                        if y - prev_y <= 1 {
                            break;
                        }
                    } else {
                        if prev_y - y <= 1 {
                            break;
                        }
                    }
                }
                
                // Calculate output amount
                let output = (xp[output_index] - y) * self.balances[output_index] / xp[output_index];
                
                output
            }
            PoolType::Crypto => {
                // Crypto swap uses different formula
                let x0 = self.balances[input_index];
                let y0 = self.balances[output_index];
                
                // Simple constant product with fee
                let x = x0 + dx;
                let k = x0 * y0;
                let y = k / x;
                
                y0.saturating_sub(y)
            }
            PoolType::Lending => {
                // Lending pool - use underlying
                self.calculate_dy(input_index, output_index, dx)
            }
        }
    }

    /// Get xp (adjusted balances)
    fn get_xp(&self) -> Vec<u256> {
        self.balances.iter().map(|b| *b as u256).collect()
    }

    /// Get D (invariant)
    fn get_D(&self, xp: &[u256]) -> u256 {
        let num_tokens = xp.len() as u256;
        let mut S = 0u256;
        let mut D = 0u256;
        
        for x in xp {
            S += *x;
        }
        
        if S == 0 {
            return 0;
        }
        
        // Initial guess
        D = S;
        
        // Newton-Raphson iteration
        let A = self.amplification;
        
        for _ in 0..255 {
            let mut D_p = D;
            let mut S_d = 0u256;
            
            for x in xp {
                S_d += x * D / x;
            }
            
            D = D * (num_tokens * D + S_d) / 
               ((num_tokens + 1) * D + S_d / A);
            
            if D > D_p {
                if D - D_p <= 1 {
                    break;
                }
            } else {
                if D_p - D <= 1 {
                    break;
                }
            }
        }
        
        D
    }

    /// Update virtual price
    fn update_virtual_price(&mut self) {
        let xp = self.get_xp();
        let D = self.get_D(&xp);
        
        if D > 0 && self.total_supply > 0 {
            self.virtual_price = D * 1e18 / self.total_supply as u256;
        }
    }

    /// Calculate remove liquidity one coin
    pub fn remove_liquidity_one_coin(
        &mut self,
        burn_amount: u128,
        token_index: usize,
        min_amount: u128,
    ) -> Result<u128, StableSwapError> {
        let dy = self.calculate_dy(0, token_index, burn_amount);
        
        if dy < min_amount {
            return Err(StableSwapError::SlippageExceeded);
        }
        
        // Update balance
        self.balances[token_index] = self.balances[token_index].saturating_sub(dy);
        self.total_supply -= burn_amount;
        
        self.update_virtual_price();
        self.updated_at = Utc::now().timestamp();
        
        Ok(dy)
    }

    /// Get current virtual price
    pub fn get_virtual_price(&self) -> u256 {
        self.virtual_price
    }

    /// Calculate price impact
    pub fn calculate_price_impact(&self, input_index: usize, output_index: usize, dx: u128) -> u256 {
        let xp = self.get_xp();
        let x_before = xp[input_index];
        let y_before = xp[output_index];
        
        let dy = self.calculate_dy(input_index, output_index, dx);
        
        let price_before = y_before * 1e18 / x_before;
        let price_after = (y_before - dy as u256) * 1e18 / (x_before + dx as u256);
        
        if price_before > 0 {
            (price_before - price_after) * 1e18 / price_before
        } else {
            0
        }
    }
}

/// LP Token
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct LPToken {
    pub token_id: String,
    pub pool_id: String,
    pub holder: String,
    pub balance: u128,
    pub locked: u128,
    pub boost_multiplier: u256,
}

impl LPToken {
    pub fn new(pool_id: String, holder: String, balance: u128) -> Self {
        Self {
            token_id: Uuid::new_v4().to_string(),
            pool_id,
            holder,
            balance,
            locked: 0,
            boost_multiplier: 1e18,
        }
    }
}

/// StableSwap Engine
pub struct StableSwapEngine {
    pools: Arc<RwLock<HashMap<String, StableSwapPool>>>,
    lp_tokens: Arc<RwLock<HashMap<String, Vec<LPToken>>>>,
    supported_chains: Arc<RwLock<std::collections::HashSet<u64>>>,
}

impl StableSwapEngine {
    /// Create a new stable swap engine
    pub fn new() -> Self {
        let chains: std::collections::HashSet<u64> = [
            CHAIN_ETH, CHAIN_BSC, CHAIN_POLYGON, CHAIN_ARBITRUM,
            CHAIN_OPTIMISM, CHAIN_BASE, CHAIN_AVALANCHE,
        ].into_iter().collect();
        
        Self {
            pools: Arc::new(RwLock::new(HashMap::new())),
            lp_tokens: Arc::new(RwLock::new(HashMap::new())),
            supported_chains: Arc::new(RwLock::new(chains)),
        }
    }

    /// Check if chain is supported
    pub fn is_chain_supported(&self, chain_id: u64) -> bool {
        self.supported_chains.read().contains(&chain_id)
    }

    /// Create a new pool
    pub fn create_pool(
        &self,
        chain_id: u64,
        name: String,
        tokens: Vec<TokenInfo>,
        amplification: u256,
        fee: u256,
    ) -> Result<String, StableSwapError> {
        if !self.is_chain_supported(chain_id) {
            return Err(StableSwapError::ChainNotSupported(chain_id));
        }
        
        if tokens.len() < 2 {
            return Err(StableSwapError::InvalidAmount("Need at least 2 tokens".to_string()));
        }
        
        let pool = StableSwapPool::new(chain_id, name, tokens, amplification, fee);
        let pool_id = pool.pool_id.clone();
        
        self.pools.write().insert(pool_id.clone(), pool);
        
        Ok(pool_id)
    }

    /// Create a crypto pool
    pub fn create_crypto_pool(
        &self,
        chain_id: u64,
        name: String,
        tokens: Vec<TokenInfo>,
    ) -> Result<String, StableSwapError> {
        if !self.is_chain_supported(chain_id) {
            return Err(StableSwapError::ChainNotSupported(chain_id));
        }
        
        let pool = StableSwapPool::new_crypto(chain_id, name, tokens);
        let pool_id = pool.pool_id.clone();
        
        self.pools.write().insert(pool_id.clone(), pool);
        
        Ok(pool_id)
    }

    /// Get pool
    pub fn get_pool(&self, pool_id: &str) -> Option<StableSwapPool> {
        self.pools.read().get(pool_id).cloned()
    }

    /// Add liquidity
    pub fn add_liquidity(
        &self,
        pool_id: &str,
        amounts: Vec<u128>,
        min_mint_amount: u128,
    ) -> Result<u128, StableSwapError> {
        let mut pools = self.pools.write();
        let pool = pools.get_mut(pool_id)
            .ok_or_else(|| StableSwapError::PoolNotFound(pool_id.to_string()))?;
        
        pool.add_liquidity(amounts, min_mint_amount)
    }

    /// Remove liquidity
    pub fn remove_liquidity(
        &self,
        pool_id: &str,
        burn_amount: u128,
        min_amounts: Vec<u128>,
    ) -> Result<Vec<u128>, StableSwapError> {
        let mut pools = self.pools.write();
        let pool = pools.get_mut(pool_id)
            .ok_or_else(|| StableSwapError::PoolNotFound(pool_id.to_string()))?;
        
        pool.remove_liquidity(burn_amount, min_amounts)
    }

    /// Exchange
    pub fn exchange(
        &self,
        pool_id: &str,
        input_index: usize,
        output_index: usize,
        dx: u128,
        min_dy: u128,
    ) -> Result<u128, StableSwapError> {
        let mut pools = self.pools.write();
        let pool = pools.get_mut(pool_id)
            .ok_or_else(|| StableSwapError::PoolNotFound(pool_id.to_string()))?;
        
        pool.exchange(input_index, output_index, dx, min_dy)
    }

    /// Get dy (quote)
    pub fn get_dy(
        &self,
        pool_id: &str,
        input_index: usize,
        output_index: usize,
        dx: u128,
    ) -> Result<u128, StableSwapError> {
        let pools = self.pools.read();
        let pool = pools.get(pool_id)
            .ok_or_else(|| StableSwapError::PoolNotFound(pool_id.to_string()))?;
        
        pool.get_dy(input_index, output_index, dx)
    }

    /// Remove liquidity one coin
    pub fn remove_liquidity_one_coin(
        &self,
        pool_id: &str,
        burn_amount: u128,
        token_index: usize,
        min_amount: u128,
    ) -> Result<u128, StableSwapError> {
        let mut pools = self.pools.write();
        let pool = pools.get_mut(pool_id)
            .ok_or_else(|| StableSwapError::PoolNotFound(pool_id.to_string()))?;
        
        pool.remove_liquidity_one_coin(burn_amount, token_index, min_amount)
    }

    /// Get virtual price
    pub fn get_virtual_price(&self, pool_id: &str) -> Result<u256, StableSwapError> {
        let pools = self.pools.read();
        let pool = pools.get(pool_id)
            .ok_or_else(|| StableSwapError::PoolNotFound(pool_id.to_string()))?;
        
        Ok(pool.get_virtual_price())
    }

    /// Get all pools
    pub fn get_pools(&self) -> Vec<StableSwapPool> {
        self.pools.read().values().cloned().collect()
    }

    /// Get pool count
    pub fn pool_count(&self) -> usize {
        self.pools.read().len()
    }

    /// Add supported chain
    pub fn add_chain(&self, chain_id: u64) {
        self.supported_chains.write().insert(chain_id);
    }

    /// Get supported chains
    pub fn supported_chains(&self) -> Vec<u64> {
        self.supported_chains.read().iter().cloned().collect()
    }
}

impl Default for StableSwapEngine {
    fn default() -> Self { Self::new() }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_pool_creation() {
        let tokens = vec![
            TokenInfo { address: "0xUSDC".to_string(), decimals: 6, symbol: "USDC".to_string() },
            TokenInfo { address: "0xUSDT".to_string(), decimals: 6, symbol: "USDT".to_string() },
        ];
        
        let pool = StableSwapPool::new(CHAIN_ETH, "USDC/USDT".to_string(), tokens, 2000, 4);
        
        assert_eq!(pool.tokens.len(), 2);
        assert_eq!(pool.fee, 4);
    }

    #[test]
    fn test_crypto_pool() {
        let tokens = vec![
            TokenInfo { address: "0xBTC".to_string(), decimals: 8, symbol: "BTC".to_string() },
            TokenInfo { address: "0xETH".to_string(), decimals: 18, symbol: "ETH".to_string() },
        ];
        
        let pool = StableSwapPool::new_crypto(CHAIN_ETH, "BTC/ETH".to_string(), tokens);
        
        assert_eq!(pool.pool_type, PoolType::Crypto);
    }

    #[test]
    fn test_add_liquidity() {
        let tokens = vec![
            TokenInfo { address: "0xUSDC".to_string(), decimals: 6, symbol: "USDC".to_string() },
            TokenInfo { address: "0xUSDT".to_string(), decimals: 6, symbol: "USDT".to_string() },
        ];
        
        let mut pool = StableSwapPool::new(CHAIN_ETH, "USDC/USDT".to_string(), tokens, 2000, 4);
        
        let amounts = vec![1000000, 1000000];
        let mint_amount = pool.add_liquidity(amounts, 1).unwrap();
        
        assert!(mint_amount > 0);
    }
}