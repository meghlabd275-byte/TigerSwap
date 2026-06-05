//! MEV Protection Service

use crate::detection::{AttackDetector, SuspiciousPattern, TransactionData};
use crate::bundle::{BundleBuilder, BundleResult, BundleTransaction};
use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct MEVConfig {
    pub enabled: bool,
    pub use_flashbots: bool,
    pub use_co_w_protocol: bool,
    pub max_base_fee_gwei: u64,
    pub priority_fee_gwei: u64,
    pub bundle_timeout: u64,
}

impl Default for MEVConfig {
    fn default() -> Self {
        Self {
            enabled: true,
            use_flashbots: true,
            use_co_w_protocol: true,
            max_base_fee_gwei: 100,
            priority_fee_gwei: 2,
            bundle_timeout: 30000,
        }
    }
}

pub struct MEVProtectionService {
    config: MEVConfig,
    detector: AttackDetector,
    bundle_builder: BundleBuilder,
}

impl MEVProtectionService {
    pub fn new(config: MEVConfig) -> Self {
        Self {
            config,
            detector: AttackDetector::new(),
            bundle_builder: BundleBuilder::new(),
        }
    }

    pub fn protect_transaction(&mut self, tx: TransactionData) -> ProtectionResult {
        let mut warnings = Vec::new();
        
        if self.detector.is_honeypot(&tx.to) {
            warnings.push("Warning: interacting with known honeypot address".to_string());
        }
        
        if self.config.enabled && self.config.use_flashbots {
            self.bundle_builder.add_transaction(tx);
        }
        
        ProtectionResult {
            protected: warnings.is_empty(),
            warnings: if warnings.is_empty() { None } else { Some(warnings) },
        }
    }

    pub fn protect_batch(&mut self, txs: &[TransactionData]) -> BatchProtectionResult {
        let sandwich_patterns = self.detector.detect_sandwich(txs);
        let arbitrage_patterns = self.detector.detect_arbitrage(txs);
        
        let all_patterns: Vec<SuspiciousPattern> = sandwich_patterns
            .into_iter()
            .chain(arbitrage_patterns.into_iter())
            .collect();
        
        let attack_tx_hashes: std::collections::HashSet<String> = all_patterns
            .iter()
            .flat_map(|p| p.transactions.clone())
            .collect();
        
        let protected_txs: Vec<TransactionData> = txs.iter()
            .filter(|tx| {
                let hash = tx.hash.as_ref().cloned().unwrap_or_default();
                !attack_tx_hashes.contains(&hash)
            })
            .cloned()
            .collect();
        
        BatchProtectionResult {
            protected_txs,
            detected_attacks: all_patterns,
        }
    }

    pub async fn send_protected_bundle(&mut self) -> Result<BundleResult, String> {
        let txs = self.bundle_builder.build();
        
        if txs.is_empty() {
            return Err("No transactions in bundle".to_string());
        }
        
        // In production, would send to Flashbots relay
        let bundle_hash = format!("bundle_{}", std::time::SystemTime::now()
            .duration_since(std::time::UNIX_EPOCH)
            .unwrap()
            .as_nanos());
        
        self.bundle_builder.clear();
        
        Ok(BundleResult {
            success: true,
            bundle_hash: Some(bundle_hash),
            transaction_hash: None,
            error: None,
            gas_used: None,
            effective_gas_price: None,
            block_number: None,
        })
    }

    pub fn add_honeypot(&mut self, address: &str) {
        self.detector.add_honeypot(address);
    }

    pub fn clear(&mut self) {
        self.bundle_builder.clear();
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ProtectionResult {
    pub protected: bool,
    pub warnings: Option<Vec<String>>,
}

#[derive(Debug, Clone)]
pub struct BatchProtectionResult {
    pub protected_txs: Vec<TransactionData>,
    pub detected_attacks: Vec<SuspiciousPattern>,
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_mev_service_creation() {
        let service = MEVProtectionService::new(MEVConfig::default());
        assert!(service.config.enabled);
    }

    #[test]
    fn test_protect_transaction() {
        let mut service = MEVProtectionService::new(MEVConfig::default());
        
        let tx = TransactionData {
            from: "0xA".to_string(),
            to: "0xPool".to_string(),
            data: "0x".to_string(),
            value: "0".to_string(),
            gas_price: "50".to_string(),
            hash: Some("0x123".to_string()),
        };
        
        let result = service.protect_transaction(tx);
        assert!(result.protected);
    }
}