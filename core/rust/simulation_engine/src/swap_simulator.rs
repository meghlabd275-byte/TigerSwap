//! Swap Simulator
//! 
//! Simulates swap transactions for prediction.

#[derive(Debug, Clone)]
pub struct SwapParams {
    pub token_in: String,
    pub token_out: String,
    pub amount_in: u64,
    pub slippage_bps: u64,
}

#[derive(Debug, Clone)]
pub struct SwapResult {
    pub success: bool,
    pub expected_output: u64,
    pub actual_output: Option<u64>,
    pub slippage: f64,
    pub price_impact: f64,
    pub errors: Vec<String>,
}

pub struct SwapSimulator {
    pools: RwLock<HashMap<String, PoolInfo>>,
}

#[derive(Debug, Clone)]
pub struct PoolInfo {
    pub token0: String,
    pub token1: String,
    pub reserve0: u64,
    pub reserve1: u64,
    pub fee: u64,
}

impl SwapSimulator {
    pub fn new() -> Self {
        Self {
            pools: RwLock::new(HashMap::new()),
        }
    }
    
    /// Add liquidity pool
    pub fn add_pool(&self, token0: String, token1: String, reserve0: u64, reserve1: u64, fee: u64) {
        let key = format!("{}-{}", token0, token1);
        self.pools.write().unwrap().insert(key, PoolInfo {
            token0,
            token1,
            reserve0,
            reserve1,
            fee,
        });
    }
    
    /// Simulate swap
    pub fn simulate(&self, params: &SwapParams) -> SwapResult {
        let key = format!("{}-{}", params.token_in, params.token_out);
        
        let pools = self.pools.read().unwrap();
        
        if let Some(pool) = pools.get(&key) {
            // Calculate output with fee
            let amount_in = params.amount_in as f64;
            let reserve_in = pool.reserve0 as f64;
            let reserve_out = pool.reserve1 as f64;
            let fee_rate = pool.fee as f64 / 10000.0;
            
            let amount_in_with_fee = amount_in * (1.0 - fee_rate);
            let numerator = amount_in_with_fee * reserve_out;
            let denominator = reserve_in + amount_in_with_fee;
            let output = numerator / denominator;
            
            let expected = output as u64;
            let min_output = expected * (10000 - params.slippage_bps as u64) / 10000;
            
            SwapResult {
                success: true,
                expected_output: expected,
                actual_output: Some(min_output),
                slippage: params.slippage_bps as f64 / 100.0,
                price_impact: (amount_in / (reserve_in + amount_in)) * 0.3,
                errors: Vec::new(),
            }
        } else {
            SwapResult {
                success: false,
                expected_output: 0,
                actual_output: None,
                slippage: 0.0,
                price_impact: 0.0,
                errors: vec!["Pool not found".to_string()],
            }
        }
    }
}

impl Default for SwapSimulator {
    fn default() -> Self {
        Self::new()
    }
}