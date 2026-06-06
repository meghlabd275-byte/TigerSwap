//! TigerSwap Simulation Engine
//! 
//! Swap, route, gas, and bridge simulation for testing and optimization.

use std::collections::HashMap;
use std::sync::{Arc, RwLock};

mod swap_simulator;
mod route_simulator;
mod gas_simulator;
mod bridge_simulator;

pub use swap_simulator::*;
pub use route_simulator::*;
pub use gas_simulator::*;
pub use bridge_simulator::*;

/// Simulation Result
#[derive(Debug, Clone)]
pub struct SimulationResult {
    pub success: bool,
    pub expected_output: u64,
    pub actual_output: Option<u64>,
    pub gas_used: u64,
    pub slippage: f64,
    pub price_impact: f64,
    pub errors: Vec<String>,
}

/// Simulation Engine - coordinates all simulators
pub struct SimulationEngine {
    swap: Arc<SwapSimulator>,
    route: Arc<RouteSimulator>,
    gas: Arc<GasSimulator>,
    bridge: Arc<BridgeSimulator>,
}

impl SimulationEngine {
    pub fn new() -> Self {
        Self {
            swap: Arc::new(SwapSimulator::new()),
            route: Arc::new(RouteSimulator::new()),
            gas: Arc::new(GasSimulator::new()),
            bridge: Arc::new(BridgeSimulator::new()),
        }
    }
    
    /// Simulate a complete swap transaction
    pub fn simulate_swap(&self, params: &SwapParams) -> SimulationResult {
        // Get gas estimate
        let gas = self.gas.estimate_swap_gas(params);
        
        // Simulate swap
        let result = self.swap.simulate(params);
        
        // Calculate route if needed
        if result.success {
            let route = self.route.find_optimal_route(
                params.token_in,
                params.token_out,
                params.amount_in,
            );
            
            SimulationResult {
                success: result.success,
                expected_output: result.expected_output,
                actual_output: result.actual_output,
                gas_used: gas,
                slippage: result.slippage,
                price_impact: result.price_impact,
                errors: result.errors,
            }
        } else {
            result
        }
    }
    
    /// Simulate cross-chain bridge
    pub fn simulate_bridge(&self, params: &BridgeParams) -> SimulationResult {
        self.bridge.simulate(params)
    }
}

impl Default for SimulationEngine {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_simulation_engine() {
        let engine = SimulationEngine::new();
        
        let params = SwapParams {
            token_in: "USDC".to_string(),
            token_out: "ETH".to_string(),
            amount_in: 1000000,
            slippage_bps: 50,
        };
        
        let result = engine.simulate_swap(&params);
        assert!(result.gas_used > 0);
    }
}