//! Swap execution logic

use super::pool::{PoolCore, SwapResult};
use num_bigint::BigUint;

/// Swap parameters
#[derive(Debug, Clone)]
pub struct SwapParams {
    pub token_in: String,
    pub token_out: String,
    pub amount_in: BigUint,
    pub min_amount_out: BigUint,
    pub recipient: String,
    pub deadline: u64,
}

/// Swap executor for executing swaps across pools
pub struct SwapExecutor {
    pools: Vec<PoolCore>,
}

impl SwapExecutor {
    /// Create a new swap executor
    pub fn new(pools: Vec<PoolCore>) -> Self {
        Self { pools }
    }

    /// Execute a swap
    pub fn execute(&self, params: &SwapParams) -> Result<BigUint, String> {
        if self.pools.is_empty() {
            return Err("No pools available".to_string());
        }

        let mut remaining = params.amount_in.clone();
        let mut total_out = BigUint::from(0u64);

        for pool in &self.pools {
            if remaining == BigUint::from(0u64) {
                break;
            }

            let result = pool.swap(&remaining, pool.fee());
            total_out += result.amount_out.clone();
            remaining = result.amount_out;
        }

        if total_out < params.min_amount_out {
            return Err("Slippage tolerance exceeded".to_string());
        }

        Ok(total_out)
    }

    /// Calculate price impact
    pub fn calculate_price_impact(&self, params: &SwapParams) -> f64 {
        let spot_price = self.get_spot_price();
        if spot_price == 0.0 {
            return 0.0;
        }

        // Simulate swap to get execution price
        let exec_price = self.get_execution_price(params);
        
        ((spot_price - exec_price) / spot_price * 100.0).max(0.0)
    }

    /// Get spot price
    pub fn get_spot_price(&self) -> f64 {
        self.pools.first().map(|p| p.get_current_price()).unwrap_or(0.0)
    }

    /// Get execution price
    fn get_execution_price(&self, params: &SwapParams) -> f64 {
        // Simple calculation - in production would follow actual swap path
        let amount_out = params.amount_in.to_f64().unwrap_or(0.0) * self.get_spot_price();
        if params.amount_in == BigUint::from(0u64) {
            return 0.0;
        }
        amount_out / params.amount_in.to_f64().unwrap_or(1.0)
    }

    /// Get pools
    pub fn pools(&self) -> &[PoolCore] {
        &self.pools
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_swap_params() {
        let params = SwapParams {
            token_in: "0xA".to_string(),
            token_out: "0xB".to_string(),
            amount_in: BigUint::from(1000u64),
            min_amount_out: BigUint::from(900u64),
            recipient: "0xRecipient".to_string(),
            deadline: 0,
        };
        
        assert_eq!(params.amount_in, BigUint::from(1000u64));
    }
}