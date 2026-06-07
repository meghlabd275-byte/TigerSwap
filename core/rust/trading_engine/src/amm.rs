use serde::{Deserialize, Serialize};
use rust_decimal::Decimal;
use std::collections::HashMap;
use std::sync::Arc;
use parking_lot::RwLock;

pub const FEE_BPS: i64 = 30;

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct AMMPool {
    pub address: String,
    pub token0: String,
    pub token1: String,
    pub reserve0: u128,
    pub reserve1: u128,
    pub fee_bps: i64,
}

impl AMMPool {
    pub fn new(address: impl Into<String>, token0: impl Into<String>, token1: impl Into<String>, fee_bps: i64) -> Self {
        Self { address: address.into(), token0: token0.into(), token1: token1.into(), reserve0: 0, reserve1: 0, fee_bps }
    }

    pub fn with_liquidity(address: impl Into<String>, token0: impl Into<String>, token1: impl Into<String>, reserve0: u128, reserve1: u128, fee_bps: i64) -> Self {
        let mut pool = Self::new(address, token0, token1, fee_bps);
        pool.reserve0 = reserve0;
        pool.reserve1 = reserve1;
        pool
    }

    pub fn get_amount_out(&self, amount_in: u128, token_in_is_token0: bool) -> (u128, i64) {
        if amount_in == 0 { return (0, 0); }
        let (reserve_in, reserve_out) = if token_in_is_token0 { (self.reserve0, self.reserve1) } else { (self.reserve1, self.reserve0) };
        if reserve_in == 0 || reserve_out == 0 { return (0, 0); }
        
        let fee_multiplier = 10000 - self.fee_bps;
        let amount_in_with_fee = (amount_in * fee_multiplier as u128 / 10000) as u128;
        let numerator = amount_in_with_fee as u128 * reserve_out as u128;
        let denominator = (reserve_in as u128 + amount_in_with_fee as u128) as u128;
        let amount_out = if denominator > 0 { numerator / denominator } else { 0 };
        
        let price_impact = if reserve_in > 0 { ((amount_in as i128 * 10000 / reserve_in as i128)) as i64 } else { 0 };
        (amount_out, price_impact)
    }

    pub fn swap(&mut self, amount_in: u128, token_in_is_token0: bool) -> (u128, i64) {
        let (amount_out, price_impact) = self.get_amount_out(amount_in, token_in_is_token0);
        if amount_out > 0 {
            if token_in_is_token0 { self.reserve0 += amount_in; self.reserve1 = self.reserve1.saturating_sub(amount_out); }
            else { self.reserve1 += amount_in; self.reserve0 = self.reserve0.saturating_sub(amount_out); }
        }
        (amount_out, price_impact)
    }

    pub fn price(&self) -> Decimal {
        if self.reserve0 == 0 { Decimal::ZERO }
        else { Decimal::from(self.reserve1) / Decimal::from(self.reserve0) }
    }
}

#[derive(Debug, Default)]
pub struct AMM {
    pools: Arc<RwLock<HashMap<String, AMMPool>>>,
}

impl AMM {
    pub fn new() -> Self { Self { pools: Arc::new(RwLock::new(HashMap::new())) } }

    pub fn register_pool(&self, pool: AMMPool) {
        let key = pool_key(&pool.token0, &pool.token1);
        self.pools.write().insert(key, pool);
    }

    pub fn get_pool(&self, token0: &str, token1: &str) -> Option<AMMPool> {
        let key = pool_key(token0, token1);
        self.pools.read().get(&key).cloned()
    }

    pub fn quote(&self, token_in: &str, token_out: &str, amount_in: u128) -> Option<(u128, i64)> {
        let pool = self.get_pool(token_in, token_out).or_else(|| self.get_pool(token_out, token_in))?;
        let token_in_is_token0 = pool.token0.to_lowercase() == token_in.to_lowercase();
        Some(pool.get_amount_out(amount_in, token_in_is_token0))
    }

    pub fn pool_count(&self) -> usize { self.pools.read().len() }
}

fn pool_key(token_a: &str, token_b: &str) -> String {
    let mut a = token_a.to_lowercase();
    let mut b = token_b.to_lowercase();
    if a > b { std::mem::swap(&mut a, &mut b); }
    format!("{}:{}", a, b)
}