//! Route representation and quote results

use num_bigint::BigUint;
use serde::{Deserialize, Serialize};

/// A single step in a route (one pool crossing)
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct RouteStep {
    pub pool_address: String,
    pub token_in: String,
    pub token_out: String,
    pub dex: String,
    pub dex_name: String,
    pub reserve_in: BigUint,
    pub reserve_out: BigUint,
    pub amount_in: BigUint,
    pub amount_out: BigUint,
    pub fee_bps: u32,
    pub spot_price: f64,
    pub price_impact: f64,
}

impl RouteStep {
    pub fn new(
        pool_address: String,
        token_in: String,
        token_out: String,
        dex: String,
        dex_name: String,
        reserve_in: BigUint,
        reserve_out: BigUint,
        amount_in: BigUint,
        amount_out: BigUint,
        fee_bps: u32,
    ) -> Self {
        let spot_price = if reserve_in == BigUint::from(0u64) {
            0.0
        } else {
            reserve_out.to_f64().unwrap_or(0.0) / reserve_in.to_f64().unwrap_or(1.0)
        };
        
        let exec_price = if amount_in == BigUint::from(0u64) {
            0.0
        } else {
            amount_out.to_f64().unwrap_or(0.0) / amount_in.to_f64().unwrap_or(1.0)
        };
        
        let price_impact = if spot_price > 0.0 {
            ((spot_price - exec_price) / spot_price * 100.0).max(0.0)
        } else {
            0.0
        };
        
        Self {
            pool_address,
            token_in,
            token_out,
            dex,
            dex_name,
            reserve_in,
            reserve_out,
            amount_in,
            amount_out,
            fee_bps,
            spot_price,
            price_impact,
        }
    }
}

/// A complete route from token A to token B
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Route {
    pub steps: Vec<RouteStep>,
    pub path: Vec<String>,
    pub total_amount_out: BigUint,
    pub total_amount_out_min: BigUint,
    pub total_price_impact: f64,
    pub total_gas_estimate: u64,
    pub total_gas_fee_usd: f64,
    pub execution_price: f64,
    pub mid_price: f64,
}

impl Route {
    pub fn new(steps: Vec<RouteStep>) -> Self {
        let path: Vec<String> = steps.iter()
            .flat_map(|s| [s.token_in.clone(), s.token_out.clone()])
            .collect();
        
        let unique_path: Vec<String> = path.into_iter().fold(Vec::new(), |mut acc, t| {
            if acc.last() != Some(&t) {
                acc.push(t);
            }
            acc
        });
        
        let total_amount_out = steps.last()
            .map(|s| s.amount_out.clone())
            .unwrap_or_default();
        
        let total_price_impact: f64 = steps.iter().map(|s| s.price_impact).sum();
        
        let exec_price = if let Some(first) = steps.first() {
            if first.amount_in != BigUint::from(0u64) {
                total_amount_out.to_f64().unwrap_or(0.0) / first.amount_in.to_f64().unwrap_or(1.0)
            } else {
                0.0
            }
        } else {
            0.0
        };
        
        let mid_price = if !steps.is_empty() {
            steps.iter().map(|s| s.spot_price).product()
        } else {
            0.0
        };
        
        let total_gas_estimate = 150000 * steps.len() as u64;
        
        Self {
            steps,
            path: unique_path,
            total_amount_out,
            total_amount_out_min: BigUint::from(0u64), // Set after slippage applied
            total_price_impact,
            total_gas_estimate,
            total_gas_fee_usd: 0.0,
            execution_price: exec_price,
            mid_price,
        }
    }
    
    /// Calculate minimum output with slippage
    pub fn with_slippage(&mut self, slippage_bps: u32) {
        let multiplier = 10000 - slippage_bps;
        self.total_amount_out_min = (self.total_amount_out.clone() * multiplier) / 10000;
    }
    
    /// Calculate gas cost in USD
    pub fn with_gas_cost(&mut self, gas_price: u64, native_price_usd: f64) {
        let gas_cost_native = self.total_gas_estimate as u128 * gas_price as u128;
        self.total_gas_fee_usd = (gas_cost_native as f64 / 1e18) * native_price_usd;
    }
}

/// A split route that distributes across multiple routes
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SplitRoute {
    pub routes: Vec<Route>,
    pub percentages: Vec<u32>,
    pub total_amount_out: BigUint,
    pub total_gas_estimate: u64,
}

impl SplitRoute {
    pub fn new(routes: Vec<Route>, percentages: Vec<u32>) -> Self {
        let total_amount_out: BigUint = routes.iter()
            .map(|r| r.total_amount_out.clone())
            .fold(BigUint::from(0u64), |acc, x| acc + x);
        
        let total_gas_estimate: u64 = routes.iter()
            .map(|r| r.total_gas_estimate)
            .sum();
        
        Self {
            routes,
            percentages,
            total_amount_out,
            total_gas_estimate,
        }
    }
}

/// Quote request for routing
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct QuoteRequest {
    pub token_in: String,
    pub token_out: String,
    pub amount_in: BigUint,
    pub slippage_bps: u32,
    pub gas_price: u64,
    pub native_price_usd: f64,
    pub max_hops: usize,
}

impl QuoteRequest {
    pub fn new(
        token_in: String,
        token_out: String,
        amount_in: BigUint,
    ) -> Self {
        Self {
            token_in,
            token_out,
            amount_in,
            slippage_bps: 50,
            gas_price: 30_000_000_000, // 30 gwei
            native_price_usd: 2000.0,
            max_hops: 3,
        }
    }
    
    pub fn with_slippage(mut self, slippage_bps: u32) -> Self {
        self.slippage_bps = slippage_bps;
        self
    }
    
    pub fn with_gas(mut self, gas_price: u64, native_price_usd: f64) -> Self {
        self.gas_price = gas_price;
        self.native_price_usd = native_price_usd;
        self
    }
}

/// Quote result with best route and alternatives
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct QuoteResult {
    pub best_route: Option<Route>,
    pub split_route: Option<SplitRoute>,
    pub all_routes: Vec<Route>,
    pub timestamp: u64,
    pub expires_at: u64,
}

impl QuoteResult {
    pub fn new(best_route: Option<Route>, all_routes: Vec<Route>) -> Self {
        let now = std::time::SystemTime::now()
            .duration_since(std::time::UNIX_EPOCH)
            .unwrap()
            .as_secs();
        
        Self {
            best_route,
            split_route: None,
            all_routes,
            timestamp: now,
            expires_at: now + 30, // 30 second TTL
        }
    }
    
    pub fn with_split_route(mut self, split: SplitRoute) -> Self {
        self.split_route = Some(split);
        self
    }
    
    pub fn is_expired(&self) -> bool {
        let now = std::time::SystemTime::now()
            .duration_since(std::time::UNIX_EPOCH)
            .unwrap()
            .as_secs();
        now > self.expires_at
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_route_creation() {
        let steps = vec![
            RouteStep::new(
                "0xpool1".to_string(),
                "A".to_string(),
                "B".to_string(),
                "uniswap_v2".to_string(),
                "Uniswap V2".to_string(),
                BigUint::from(1000000u64),
                BigUint::from(1000000u64),
                BigUint::from(1000u64),
                BigUint::from(500u64),
                300,
            ),
        ];
        
        let mut route = Route::new(steps);
        route.with_slippage(50);
        
        assert_eq!(route.steps.len(), 1);
        assert!(route.total_amount_out_min < route.total_amount_out);
    }

    #[test]
    fn test_quote_request() {
        let req = QuoteRequest::new(
            "A".to_string(),
            "B".to_string(),
            BigUint::from(1000u64),
        );
        
        assert_eq!(req.slippage_bps, 50);
        assert_eq!(req.max_hops, 3);
    }
}