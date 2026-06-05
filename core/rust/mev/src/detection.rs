//! MEV Attack Detection

use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum AttackType {
    Sandwich,
    Frontrun,
    Backrun,
    Arbitrage,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SuspiciousPattern {
    pub attack_type: AttackType,
    pub severity: Severity,
    pub transactions: Vec<String>,
    pub description: String,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum Severity {
    Low,
    Medium,
    High,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SandwichAttack {
    pub attacker_address: String,
    pub victim_address: String,
    pub front_run_amount: String,
    pub back_run_amount: String,
    pub profit: String,
    pub block_number: u64,
}

pub struct AttackDetector {
    honeypots: std::collections::HashSet<String>,
}

impl AttackDetector {
    pub fn new() -> Self {
        Self {
            honeypots: std::collections::HashSet::new(),
        }
    }

    pub fn add_honeypot(&mut self, address: &str) {
        self.honeypots.insert(address.to_lowercase());
    }

    pub fn is_honeypot(&self, address: &str) -> bool {
        self.honeypots.contains(&address.to_lowercase())
    }

    pub fn detect_sandwich(&self, txs: &[TransactionData]) -> Vec<SuspiciousPattern> {
        let mut patterns = Vec::new();
        
        for i in 0..txs.len().saturating_sub(2) {
            let front = &txs[i];
            let target = &txs[i + 1];
            let back = &txs[i + 2];
            
            if self.is_sandwich(front, target, back) {
                patterns.push(SuspiciousPattern {
                    attack_type: AttackType::Sandwich,
                    severity: Severity::High,
                    transactions: vec![
                        front.hash.clone().unwrap_or_default(),
                        target.hash.clone().unwrap_or_default(),
                        back.hash.clone().unwrap_or_default(),
                    ],
                    description: format!("Sandwich attack: frontrun {}, target {}", front.from, target.from),
                });
            }
        }
        
        patterns
    }

    fn is_sandwich(&self, front: &TransactionData, target: &TransactionData, back: &TransactionData) -> bool {
        if front.to != back.to {
            return false;
        }
        
        let front_gas = front.gas_price.parse::<u64>().unwrap_or(0);
        let target_gas = target.gas_price.parse::<u64>().unwrap_or(0);
        let back_gas = back.gas_price.parse::<u64>().unwrap_or(0);
        
        front_gas > target_gas && back_gas > target_gas
    }

    pub fn detect_arbitrage(&self, txs: &[TransactionData]) -> Vec<SuspiciousPattern> {
        let mut patterns = Vec::new();
        let mut address_counts: std::collections::HashMap<String, usize> = std::collections::HashMap::new();
        
        for tx in txs {
            *address_counts.entry(tx.from.clone()).or_insert(0) += 1;
        }
        
        for (address, count) in address_counts {
            if count >= 3 {
                patterns.push(SuspiciousPattern {
                    attack_type: AttackType::Arbitrage,
                    severity: if count >= 5 { Severity::High } else { Severity::Medium },
                    transactions: txs.iter()
                        .filter(|tx| tx.from == address)
                        .filter_map(|tx| tx.hash.clone())
                        .collect(),
                    description: format!("Potential arbitrage: {} has {} transactions", address, count),
                });
            }
        }
        
        patterns
    }
}

impl Default for AttackDetector {
    fn default() -> Self {
        Self::new()
    }
}

#[derive(Debug, Clone)]
pub struct TransactionData {
    pub from: String,
    pub to: String,
    pub data: String,
    pub value: String,
    pub gas_price: String,
    pub hash: Option<String>,
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_sandwich_detection() {
        let detector = AttackDetector::new();
        
        let txs = vec![
            TransactionData {
                from: "0xA".to_string(),
                to: "0xPool".to_string(),
                data: "".to_string(),
                value: "0".to_string(),
                gas_price: "100".to_string(),
                hash: Some("0x1".to_string()),
            },
            TransactionData {
                from: "0xB".to_string(),
                to: "0xPool".to_string(),
                data: "".to_string(),
                value: "0".to_string(),
                gas_price: "50".to_string(),
                hash: Some("0x2".to_string()),
            },
            TransactionData {
                from: "0xA".to_string(),
                to: "0xPool".to_string(),
                data: "".to_string(),
                value: "0".to_string(),
                gas_price: "100".to_string(),
                hash: Some("0x3".to_string()),
            },
        ];
        
        let patterns = detector.detect_sandwich(&txs);
        assert_eq!(patterns.len(), 1);
        assert_eq!(patterns[0].attack_type, AttackType::Sandwich);
    }
}