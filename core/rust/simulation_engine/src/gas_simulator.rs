//! Gas Simulator
//! 
//! Estimates gas costs for transactions.

use std::collections::HashMap;
use std::sync::RwLock;

use crate::SwapParams;

pub struct GasSimulator {
    gas_costs: RwLock<HashMap<String, u64>>,
}

impl GasSimulator {
    pub fn new() -> Self {
        let simulator = Self {
            gas_costs: RwLock::new(HashMap::new()),
        };
        
        // Default gas costs
        let mut costs = simulator.gas_costs.write().unwrap();
        costs.insert("swap".to_string(), 150000);
        costs.insert("transfer".to_string(), 21000);
        costs.insert("approve".to_string(), 46000);
        costs.insert("mint".to_string(), 200000);
        costs.insert("burn".to_string(), 100000);
        
        simulator
    }
    
    /// Estimate gas for swap
    pub fn estimate_swap_gas(&self, params: &SwapParams) -> u64 {
        *self.gas_costs.read().unwrap().get("swap").unwrap_or(&150000)
    }
    
    /// Estimate gas for transfer
    pub fn estimate_transfer_gas(&self, token: &str) -> u64 {
        if token == "0x0000000000000000000000000000000000000000" {
            *self.gas_costs.read().unwrap().get("transfer").unwrap_or(&21000)
        } else {
            *self.gas_costs.read().unwrap().get("transfer").unwrap_or(&65000)
        }
    }
    
    /// Estimate gas for contract interaction
    pub fn estimate_contract_gas(&self, method: &str) -> u64 {
        *self.gas_costs.read().unwrap().get(method).unwrap_or(&100000)
    }
    
    /// Get current gas price
    pub fn get_gas_price(&self, chain: &str) -> u64 {
        // Simplified - would fetch from oracle
        match chain {
            "ethereum" => 20_00000000,  // 20 Gwei
            "bsc" => 5_00000000,      // 5 Gwei
            "polygon" => 50_000000,    // 0.05 Gwei
            "arbitrum" => 100_000,      // 0.0001 Gwei
            _ => 20_00000000,
        }
    }
    
    /// Calculate total cost
    pub fn calculate_total_cost(&self, chain: &str, gas: u64) -> u64 {
        gas * self.get_gas_price(chain)
    }
}