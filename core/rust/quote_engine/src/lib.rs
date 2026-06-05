//! TigerSwap Quote Engine - Rust Implementation
//! Real-time quote calculation with slippage, gas, and price impact

use std::collections::HashMap;

// ============================================================================
// Constants
// ============================================================================

const Q96: u128 = 1 << 96;
const Q128: u128 = 1 << 128;

// ============================================================================
// Data Structures
// ============================================================================

#[derive(Debug, Clone)]
pub struct Token {
    pub address: String,
    pub symbol: String,
    pub decimals: u8,
    pub price_usd: f64,
}

#[derive(Debug, Clone)]
pub struct Pool {
    pub dex: String,
    pub address: String,
    pub token_a: String,
    pub token_b: String,
    pub reserve_a: u128,
    pub reserve_b: u128,
    pub fee_bps: u64,
    pub liquidity: f64,
}

#[derive(Debug, Clone)]
pub struct QuoteRequest {
    pub token_in: String,
    pub token_out: String,
    pub amount_in: u128,
    pub slippage_bps: u64,
    pub gas_price_gwei: Option<u128>,
}

#[derive(Debug, Clone)]
pub struct Quote {
    pub input_token: String,
    pub output_token: String,
    pub input_amount: u128,
    pub output_amount: u128,
    pub output_amount_min: u128,
    pub price_impact_bps: u64,
    pub route: Vec<RouteStep>,
    pub gas_estimate: u64,
    pub gas_fee_usd: f64,
    pub exchange_rate: f64,
    pub provider: String,
    pub expires_at: u64,
}

#[derive(Debug, Clone)]
pub struct RouteStep {
    pub dex: String,
    pub pool: String,
    pub path: Vec<String>,
    pub percentage: u32,
    pub fee: u64,
}

// ============================================================================
// Quote Engine
// ============================================================================

pub struct QuoteEngine {
    pools: HashMap<String, Vec<Pool>>,
    tokens: HashMap<String, Token>,
    gas_price: u128,
}

impl QuoteEngine {
    pub fn new() -> Self {
        Self {
            pools: HashMap::new(),
            tokens: HashMap::new(),
            gas_price: 35_000_000_000, // 35 gwei
        }
    }

    pub fn add_pool(&mut self, pool: Pool) {
        let key = token_pair_key(&pool.token_a, &pool.token_b);
        self.pools.entry(key).or_insert_with(Vec::new).push(pool);
    }

    pub fn add_token(&mut self, token: Token) {
        self.tokens.insert(token.address.clone(), token);
    }

    pub fn set_gas_price(&mut self, gwei: u128) {
        self.gas_price = gwei;
    }

    pub fn get_quote(&self, req: &QuoteRequest) -> Result<Quote, String> {
        let pools = self.pools.get(&token_pair_key(&req.token_in, &req.token_out));
        let Some(pools) = pools else {
            return Err("No pools found".to_string());
        };

        // Find best pool
        let mut best_pool: Option<&Pool> = None;
        let mut best_output: u128 = 0;

        for pool in pools {
            let amount_out = self.calculate_output(pool, req.amount_in);
            if amount_out > best_output {
                best_output = amount_out;
                best_pool = Some(pool);
            }
        }

        let Some(pool) = best_pool else {
            return Err("No valid pool".to_string());
        };

        // Calculate outputs
        let output_amount = self.calculate_output(pool, req.amount_in);
        let amount_min = output_amount * (10000 - req.slippage_bps) / 10000;

        // Price impact
        let spot_price = (pool.reserve_b as f64) / (pool.reserve_a as f64);
        let exec_price = (output_amount as f64) / (req.amount_in as f64);
        let price_impact = ((spot_price - exec_price) / spot_price * 10000.0) as u64;

        // Gas estimate
        let gas_estimate = 150000u64;

        // Get token decimals for rate
        let token_in_decimals = self.tokens.get(&req.token_in)
            .map(|t| t.decimals as u32)
            .unwrap_or(18);
        let token_out_decimals = self.tokens.get(&req.token_out)
            .map(|t| t.decimals as u32)
            .unwrap_or(18);

        let exchange_rate = (output_amount as f64) / (req.amount_in as f64);
        let rate_adjustment = 10f64.powi(token_out_decimals as i32 - token_in_decimals as i32);
        let exchange_rate = exchange_rate * rate_adjustment;

        // Gas fee in USD
        let eth_price = self.tokens.get("0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2")
            .map(|t| t.price_usd)
            .unwrap_or(2500.0);
        let gas_fee_eth = (gas_estimate as f64 * self.gas_price as f64) / 1e18;
        let gas_fee_usd = gas_fee_eth * eth_price;

        Ok(Quote {
            input_token: req.token_in.clone(),
            output_token: req.token_out.clone(),
            input_amount: req.amount_in,
            output_amount,
            output_amount_min: amount_min,
            price_impact_bps: price_impact,
            route: vec![RouteStep {
                dex: pool.dex.clone(),
                pool: pool.address.clone(),
                path: vec![req.token_in.clone(), req.token_out.clone()],
                percentage: 100,
                fee: pool.fee_bps,
            }],
            gas_estimate,
            gas_fee_usd,
            exchange_rate,
            provider: "TigerSwap".to_string(),
            expires_at: current_timestamp() + 30,
        })
    }

    pub fn calculate_output(&self, pool: &Pool, amount_in: u128) -> u128 {
        if amount_in == 0 || pool.reserve_a == 0 || pool.reserve_b == 0 {
            return 0;
        }

        let (reserve_in, reserve_out) = if pool.token_a == pool.token_a {
            (pool.reserve_a, pool.reserve_b)
        } else {
            (pool.reserve_b, pool.reserve_a)
        };

        let fee_multiplier = 10000 - pool.fee_bps;
        let numerator = amount_in * reserve_out * fee_multiplier;
        let denominator = reserve_in * 10000 + amount_in * fee_multiplier;

        if denominator == 0 { 0 } else { numerator / denominator }
    }

    pub fn compare_dex_prices(&self, token_in: &str, token_out: &str, amount: u128) -> Vec<DexQuote> {
        let pools = match self.pools.get(&token_pair_key(token_in, token_out)) {
            Some(p) => p,
            None => return Vec::new(),
        };

        let mut quotes: Vec<DexQuote> = pools.iter().map(|pool| {
            let output = self.calculate_output(pool, amount);
            DexQuote {
                dex: pool.dex.clone(),
                pool: pool.address.clone(),
                output_amount: output,
                fee_bps: pool.fee_bps,
                liquidity: pool.liquidity,
            }
        }).collect();

        quotes.sort_by(|a, b| b.output_amount.cmp(&a.output_amount));
        quotes
    }
}

#[derive(Debug, Clone)]
pub struct DexQuote {
    pub dex: String,
    pub pool: String,
    pub output_amount: u128,
    pub fee_bps: u64,
    pub liquidity: f64,
}

fn token_pair_key(a: &str, b: &str) -> String {
    let mut tokens = vec![a.to_lowercase(), b.to_lowercase()];
    tokens.sort();
    format!("{}_{}", tokens[0], tokens[1])
}

fn current_timestamp() -> u64 {
    std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .unwrap()
        .as_secs()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_quote() {
        let mut engine = QuoteEngine::new();
        
        engine.add_pool(Pool {
            dex: "uniswap_v3".to_string(),
            address: "0x88e6A0c2dDD26FEEb64F039a2c41296FcB3f5640".to_string(),
            token_a: "0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2".to_string(),
            token_b: "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48".to_string(),
            reserve_a: 35000 * 10u128.pow(18),
            reserve_b: 87500000 * 10u128.pow(6),
            fee_bps: 500,
            liquidity: 87_500_000.0,
        });

        let req = QuoteRequest {
            token_in: "0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2".to_string(),
            token_out: "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48".to_string(),
            amount_in: 1 * 10u128.pow(18),
            slippage_bps: 50,
            gas_price_gwei: None,
        };

        let quote = engine.get_quote(&req).unwrap();
        assert!(quote.output_amount > 0);
    }
}
