//! Bridge Simulator
//! 
//! Simulates cross-chain bridge transactions.

use crate::SimulationResult;
use crate::SwapParams; // For BridgeParams

#[derive(Debug, Clone)]
pub struct BridgeParams {
    pub from_chain: String,
    pub to_chain: String,
    pub token: String,
    pub amount: u64,
}

pub struct BridgeSimulator {
    bridge_fees: RwLock<HashMap<String, u64>>,
}

impl BridgeSimulator {
    pub fn new() -> Self {
        Self {
            bridge_fees: RwLock::new(HashMap::new()),
        }
    }
    
    /// Simulate bridge transaction
    pub fn simulate(&self, params: &BridgeParams) -> SimulationResult {
        let key = format!("{}-{}", params.from_chain, params.to_chain);
        
        let fees = self.bridge_fees.read().unwrap();
        let bridge_fee = fees.get(&key).copied().unwrap_or(500000); // Default 0.5%
        
        let output = params.amount * (1000000 - bridge_fee) / 1000000;
        let gas = 200000; // Bridge typically higher gas
        
        SimulationResult {
            success: true,
            expected_output: output,
            actual_output: Some(output),
            gas_used: gas,
            slippage: bridge_fee as f64 / 10000.0,
            price_impact: 0.0,
            errors: Vec::new(),
        }
    }
    
    /// Set bridge fee
    pub fn set_fee(&self, from: &str, to: &str, fee: u64) {
        let key = format!("{}-{}", from, to);
        self.bridge_fees.write().unwrap().insert(key, fee);
    }
}