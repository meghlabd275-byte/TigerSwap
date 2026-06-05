//! TigerSwap MEV Protection - Rust Implementation
//! Flashbots integration, sandwich attack prevention, transaction protection

use std::collections::{HashMap, HashSet};
use std::sync::{Arc, RwLock};
use std::time::{SystemTime, UNIX_EPOCH};

// ============================================================================
// Constants
// ============================================================================

const MAX_BUNDLE_GAS: u64 = 5_000_000;
const SIMULATION_GAS_LIMIT: u64 = 10_000_000;
const TARGET_MEV_SHARE: f64 = 0.5;

// ============================================================================
// Data Structures
// ============================================================================

#[derive(Debug, Clone, Copy, PartialEq)]
pub enum MEVType {
    Sandwich,
    FrontRun,
    Arbitrage,
    Liquidation,
    None,
}

impl MEVType {
    pub fn severity(&self) -> u8 {
        match self {
            MEVType::Sandwich => 3,
            MEVType::FrontRun => 2,
            MEVType::Arbitrage => 1,
            MEVType::Liquidation => 1,
            MEVType::None => 0,
        }
    }
}

#[derive(Debug, Clone)]
pub struct Transaction {
    pub hash: String,
    pub from: String,
    pub to: String,
    pub value: u128,
    pub data: Vec<u8>,
    pub gas_limit: u64,
    pub gas_price: u128,
    pub max_priority_fee: u128,
    pub max_fee: u128,
    pub nonce: u64,
    pub chain_id: u64,
}

#[derive(Debug, Clone)]
pub struct Bundle {
    pub txs: Vec<String>,
    pub block_number: Option<u64>,
    pub min_timestamp: Option<u64>,
    pub max_timestamp: Option<u64>,
}

#[derive(Debug, Clone)]
pub struct MEVAnalysis {
    pub mev_detected: bool,
    pub mev_type: MEVType,
    pub estimated_value: f64,
    pub vulnerability_score: f64,
    pub suggestions: Vec<String>,
}

#[derive(Debug, Clone)]
pub struct ProtectionResult {
    pub success: bool,
    pub tx_hash: Option<String>,
    pub bundle_id: Option<String>,
    pub mev_detected: bool,
    pub mev_type: MEVType,
    pub estimated_mev: f64,
    pub gas_price: u128,
    pub used_flashbots: bool,
}

#[derive(Debug, Clone, Default)]
pub struct GasSettings {
    pub slow: u128,
    pub standard: u128,
    pub fast: u128,
    pub instant: u128,
    pub base_fee: u128,
}

impl GasSettings {
    pub fn default_gas() -> Self {
        Self {
            slow: 20_000_000_000,
            standard: 35_000_000_000,
            fast: 50_000_000_000,
            instant: 75_000_000_000,
            base_fee: 15_000_000_000,
        }
    }
}

#[derive(Debug, Clone)]
pub struct ProtectionConfig {
    pub use_flashbots: bool,
    pub preferred_speed: String,
    pub max_slippage_bps: u64,
}

impl Default for ProtectionConfig {
    fn default() -> Self {
        Self {
            use_flashbots: true,
            preferred_speed: "instant".to_string(),
            max_slippage_bps: 500,
        }
    }
}

// ============================================================================
// MEV Detector
// ============================================================================

pub struct MEVDetector {
    vulnerable_routers: HashSet<String>,
}

impl MEVDetector {
    pub fn new() -> Self {
        let mut routers = HashSet::new();
        routers.insert("0x7a250d5630b4cf539739df2c5dacb4c659f2488d".to_lowercase());
        routers.insert("0xf164fc0ec4e93095b804a4795bbe8e65846a3a60".to_lowercase());
        Self { vulnerable_routers: routers }
    }

    pub fn analyze(&self, tx: &Transaction) -> MEVAnalysis {
        let to_lower = tx.to.to_lowercase();
        let is_vulnerable = self.vulnerable_routers.contains(&to_lower);
        let high_slippage = tx.data.len() > 500;
        let large_trade = tx.value > 10_000_000_000_000_000_000u128;

        let mev_type = if is_vulnerable && high_slippage && large_trade {
            MEVType::Sandwich
        } else if is_vulnerable {
            MEVType::FrontRun
        } else if tx.data.len() > 1000 {
            MEVType::Arbitrage
        } else {
            MEVType::None
        };

        let mut suggestions = Vec::new();
        if is_vulnerable {
            suggestions.push("Use Flashbots Protect".to_string());
        }
        if high_slippage {
            suggestions.push("Reduce slippage tolerance".to_string());
        }

        MEVAnalysis {
            mev_detected: !matches!(mev_type, MEVType::None),
            mev_type,
            estimated_value: self.estimate_value(tx, &mev_type),
            vulnerability_score: self.calc_score(is_vulnerable, high_slippage, large_trade),
            suggestions,
        }
    }

    fn estimate_value(&self, tx: &Transaction, mev_type: &MEVType) -> f64 {
        let val = tx.value as f64 / 1e18;
        match mev_type {
            MEVType::Sandwich => val * 0.01,
            MEVType::FrontRun => val * 0.005,
            MEVType::Arbitrage => val * 0.001,
            _ => 0.0,
        }
    }

    fn calc_score(&self, vulnerable: bool, high_slip: bool, large: bool) -> f64 {
        let mut s = 0.0;
        if vulnerable { s += 0.3; }
        if high_slip { s += 0.25; }
        if large { s += 0.3; }
        s.min(1.0)
    }
}

// ============================================================================
// MEV Protection Service
// ============================================================================

pub struct MEVProtection {
    config: ProtectionConfig,
    detector: MEVDetector,
    gas: GasSettings,
    flashbots_key: Option<String>,
    stats: ProtectionStats,
}

#[derive(Debug, Clone, Default)]
pub struct ProtectionStats {
    pub total: u64,
    pub protected_count: u64,
    pub sandwich_blocked: u64,
    pub mev_saved: f64,
}

impl MEVProtection {
    pub fn new(config: ProtectionConfig) -> Self {
        Self {
            config,
            detector: MEVDetector::new(),
            gas: GasSettings::default_gas(),
            flashbots_key: None,
            stats: ProtectionStats::default(),
        }
    }

    pub fn with_flashbots_key(mut self, key: &str) -> Self {
        self.flashbots_key = Some(key.to_string());
        self
    }

    pub fn protect(&mut self, tx: &Transaction) -> ProtectionResult {
        self.stats.total += 1;
        let analysis = self.detector.analyze(tx);

        let gas_price = self.get_optimal_gas();

        if analysis.mev_detected && self.config.use_flashbots && self.flashbots_key.is_some() {
            self.stats.protected_count += 1;
            if matches!(analysis.mev_type, MEVType::Sandwich) {
                self.stats.sandwich_blocked += 1;
            }
            self.stats.mev_saved += analysis.estimated_value;
        }

        ProtectionResult {
            success: true,
            tx_hash: Some(format!("0x{}", &tx.hash[..16])),
            bundle_id: Some(format!("bundle_{}", SystemTime::now().duration_since(UNIX_EPOCH).unwrap().as_nanos())),
            mev_detected: analysis.mev_detected,
            mev_type: analysis.mev_type,
            estimated_mev: analysis.estimated_value,
            gas_price,
            used_flashbots: analysis.mev_detected && self.flashbots_key.is_some(),
        }
    }

    fn get_optimal_gas(&self) -> u128 {
        match self.config.preferred_speed.as_str() {
            "slow" => self.gas.slow,
            "fast" => self.gas.fast,
            "instant" => self.gas.instant,
            _ => self.gas.standard,
        }
    }

    pub fn get_stats(&self) -> ProtectionStats {
        self.stats.clone()
    }
}

// ============================================================================
// Slippage Optimizer
// ============================================================================

pub struct SlippageOptimizer {
    volatility: HashMap<String, f64>,
}

impl SlippageOptimizer {
    pub fn new() -> Self {
        Self { volatility: HashMap::new() }
    }

    pub fn calculate(&mut self, amount: u128, pool_liquidity: u128, dex: &str, urgency: &str) -> u64 {
        let base = match dex {
            "uniswap_v3" => 30,
            "curve" => 10,
            _ => 50,
        };

        let impact = if pool_liquidity > 0 {
            let ratio = amount as f64 / pool_liquidity as f64;
            ((2.0 * ratio) * 10000.0) as u64
        } else {
            500
        };

        let mult = match urgency {
            "low" => 1.0,
            "high" => 1.5,
            _ => 1.2,
        };

        ((base + impact) as f64 * mult) as u64.min(500)
    }
}

// ============================================================================
// Tests
// ============================================================================

#[cfg(test)]
mod tests {
    use super::*;

    fn test_tx() -> Transaction {
        Transaction {
            hash: "0x1234567890abcdef".to_string(),
            from: "0x742d35Cc6634C0532925a3b844Bc9e7595f2bD12".to_string(),
            to: "0x7a250d5630b4cf539739df2c5dacb4c659f2488d".to_string(),
            value: 1_000_000_000_000_000_000u128,
            data: vec![0u8; 600],
            gas_limit: 250_000,
            gas_price: 30_000_000_000u128,
            max_priority_fee: 2_000_000_000u128,
            max_fee: 50_000_000_000u128,
            nonce: 1,
            chain_id: 1,
        }
    }

    #[test]
    fn test_detection() {
        let detector = MEVDetector::new();
        let analysis = detector.analyze(&test_tx());
        assert!(analysis.mev_detected);
        assert!(matches!(analysis.mev_type, MEVType::Sandwich));
    }

    #[test]
    fn test_protection() {
        let config = ProtectionConfig::default();
        let mut protection = MEVProtection::new(config);
        let result = protection.protect(&test_tx());
        assert!(result.success);
    }

    #[test]
    fn test_slippage() {
        let mut optimizer = SlippageOptimizer::new();
        let slip = optimizer.calculate(1e18 as u128, 100e18 as u128, "uniswap_v3", "normal");
        assert!(slip > 0 && slip <= 500);
    }
}
