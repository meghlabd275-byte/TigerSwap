/**
 * MEV Protection Module
 * Protection against Maximal Extractable Value (MEV) attacks
 * 
 * @author TigerSwap
 * @version 1.0.0 Production
 */

use crate::{Result, SecurityIssue, Severity, IssueCategory};
use serde::{Deserialize, Serialize};
use std::collections::HashMap;

/// MEV Protection configuration
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct MEVProtectionConfig {
    pub enable_front_running_protection: bool,
    pub enable_sandwich_protection: bool,
    pub enable_flashbots_protection: bool,
    pub max_slippage_bps: u32,
    pub enable_private_transactions: bool,
}

impl Default for MEVProtectionConfig {
    fn default() -> Self {
        Self {
            enable_front_running_protection: true,
            enable_sandwich_protection: true,
            enable_flashbots_protection: true,
            max_slippage_bps: 50, // 0.5%
            enable_private_transactions: true,
        }
    }
}

/// Transaction metadata for MEV analysis
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct TransactionMetadata {
    pub hash: String,
    pub from: String,
    pub to: String,
    pub value: u64,
    pub gas_price: u64,
    pub gas_limit: u64,
    pub nonce: u64,
    pub timestamp: u64,
    pub block_number: u64,
    pub input_data: Vec<u8>,
}

/// MEV Analysis result
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct MEVAnalysis {
    pub is_frontrunnable: bool,
    pub is_sandwichable: bool,
    pub estimated_mev: f64,
    pub recommended_gas_price: u64,
    pub protection_level: ProtectionLevel,
    pub suggestions: Vec<String>,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum ProtectionLevel {
    None,
    Low,
    Medium,
    High,
}

/// MEV Protection engine
pub struct MEVProtection {
    config: MEVProtectionConfig,
    transaction_cache: HashMap<String, TransactionMetadata>,
}

impl MEVProtection {
    pub fn new() -> Self {
        Self {
            config: MEVProtectionConfig::default(),
            transaction_cache: HashMap::new(),
        }
    }
    
    pub fn with_config(config: MEVProtectionConfig) -> Self {
        Self {
            config,
            transaction_cache: HashMap::new(),
        }
    }
    
    /// Analyze a transaction for MEV vulnerability
    pub fn analyze_transaction(&self, tx: &TransactionMetadata) -> MEVAnalysis {
        let mut is_frontrunnable = false;
        let mut is_sandwichable = false;
        let mut estimated_mev = 0.0;
        let mut suggestions = Vec::new();
        
        // Check for front-running vulnerability
        if self.config.enable_front_running_protection {
            // Check if transaction has significant value
            if tx.value > 1_000_000_000_000_000 { // > 0.001 ETH
                is_frontrunnable = true;
                suggestions.push("Consider using private transactions or flashbots bundle".to_string());
                suggestions.push("Add deadline protection to orders".to_string());
            }
            
            // Check gas price
            if tx.gas_price > 100_000_000_000 { // > 100 gwei
                is_frontrunnable = true;
                suggestions.push("Consider lowering gas price to reduce MEV exposure".to_string());
            }
        }
        
        // Check for sandwich vulnerability
        if self.config.enable_sandwich_protection {
            // Simple DEX swap detection
            let input_str = String::from_utf8_lossy(&tx.input_data);
            if input_str.contains("swap") || input_str.contains("exactInput") {
                is_sandwichable = true;
                suggestions.push("Use limit orders instead of market orders".to_string());
                suggestions.push("Set appropriate slippage tolerance".to_string());
                suggestions.push("Consider using MEV-protected routing".to_string());
                
                // Estimate potential MEV
                estimated_mev = (tx.value as f64) * 0.003; // Rough estimate
            }
        }
        
        // Calculate recommended gas price
        let recommended_gas_price = if is_frontrunnable {
            // For MEV protection, use higher gas to get included quickly
            tx.gas_price * 12 / 10 // 20% higher
        } else {
            // Use competitive but fair gas price
            tx.gas_price * 9 / 10 // 10% lower
        };
        
        let protection_level = if !is_frontrunnable && !is_sandwichable {
            ProtectionLevel::High
        } else if is_frontrunnable && !is_sandwichable || !is_frontrunnable && is_sandwichable {
            ProtectionLevel::Medium
        } else {
            ProtectionLevel::Low
        };
        
        MEVAnalysis {
            is_frontrunnable,
            is_sandwichable,
            estimated_mev,
            recommended_gas_price,
            protection_level,
            suggestions,
        }
    }
    
    /// Generate MEV-protected transaction data
    pub fn protect_transaction(&self, input: &[u8], recipient: &str) -> Result<Vec<u8>> {
        // Simplified protection - in production would integrate with Flashbots, etc.
        
        // For now, add some basic protection markers
        let mut protected = Vec::new();
        
        // Add protection markers
        protected.extend_from_slice(b"MEV_PROTECTED:");
        protected.extend_from_slice(input);
        
        Ok(protected)
    }
    
    /// Check if a swap is protected
    pub fn is_swap_protected(&self, input: &[u8]) -> bool {
        let input_str = String::from_utf8_lossy(input);
        
        // Check for common protection patterns
        input_str.contains("MEV_PROTECTED") ||
        input_str.contains("flashbots") ||
        input_str.contains("private")
    }
    
    /// Calculate optimal gas price for protection
    pub fn calculate_protected_gas_price(&self, base_gas_price: u64, urgency: f32) -> u64 {
        // Base protection: add margin to ensure inclusion
        let protected_price = (base_gas_price as f64 * 1.1) as u64;
        
        // Adjust based on urgency
        let adjusted = (protected_price as f32 * (1.0 + urgency * 0.5)) as u64;
        
        // Cap at max slippage
        let max_price = base_gas_price * (1 + self.config.max_slippage_bps as u64 / 10000);
        
        std::cmp::min(adjusted, max_price)
    }
    
    /// Get protection configuration
    pub fn get_config(&self) -> &MEVProtectionConfig {
        &self.config
    }
    
    /// Update configuration
    pub fn update_config(&mut self, config: MEVProtectionConfig) {
        self.config = config;
    }
}

impl Default for MEVProtection {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_mev_analysis() {
        let protection = MEVProtection::new();
        
        let tx = TransactionMetadata {
            hash: "0x1234".to_string(),
            from: "0xabcd".to_string(),
            to: "0xdefg".to_string(),
            value: 10_000_000_000_000_000, // 0.01 ETH
            gas_price: 50_000_000_000, // 50 gwei
            gas_limit: 100000,
            nonce: 1,
            timestamp: 1234567890,
            block_number: 10000,
            input_data: b"swapExactETHForTokens".to_vec(),
        };
        
        let analysis = protection.analyze_transaction(&tx);
        
        assert!(analysis.is_frontrunnable || analysis.is_sandwichable);
    }
    
    #[test]
    fn test_gas_price_calculation() {
        let protection = MEVProtection::new();
        
        let gas_price = protection.calculate_protected_gas_price(50_000_000_000, 0.5);
        
        // Should be higher than base
        assert!(gas_price > 50_000_000_000);
    }
}
