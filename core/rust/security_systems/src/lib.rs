//! Security Systems
//! 
//! Runtime threat detection, behavioral analysis, wallet risk scoring, and transaction firewall

use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use std::sync::Arc;
use parking_lot::RwLock;
use sha2::{Sha256, Digest};

/// Threat type
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum ThreatType {
    Malware,
    Phishing,
    RugPull,
    Honeypot,
    FlashLoanAttack,
    SandwichAttack,
    FrontRun,
    Drainer,
    FakeToken,
    Exploit,
}

/// Threat level
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum ThreatLevel {
    Low,
    Medium,
    High,
    Critical,
    Blocked,
}

/// Threat detection result
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ThreatDetection {
    pub threat_type: ThreatType,
    pub threat_level: ThreatLevel,
    pub confidence: u8,
    pub description: String,
    pub metadata: HashMap<String, String>,
}

/// Wallet risk score
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct WalletRiskScore {
    pub wallet: String,
    pub risk_score: u8, // 0-100
    pub risk_level: ThreatLevel,
    pub factors: Vec<String>,
    pub last_updated: i64,
}

/// Transaction firewall rule
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct FirewallRule {
    pub rule_id: String,
    pub name: String,
    pub pattern: String,
    pub threat_type: ThreatType,
    pub action: FirewallAction,
    pub is_active: bool,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum FirewallAction {
    Allow,
    Block,
    Monitor,
    Challenge,
}

/// Security Systems
pub struct SecuritySystems {
    threats: Arc<RwLock<HashMap<String, ThreatDetection>>>,
    wallet_scores: Arc<RwLock<HashMap<String, WalletRiskScore>>>,
    firewall_rules: Arc<RwLock<Vec<FirewallRule>>>,
    malicious_contracts: Arc<RwLock<HashMap<String, bool>>>,
}

impl SecuritySystems {
    pub fn new() -> Self {
        Self {
            threats: Arc::new(RwLock::new(HashMap::new())),
            wallet_scores: Arc::new(RwLock::new(HashMap::new())),
            firewall_rules: Arc::new(RwLock::new(Vec::new())),
            malicious_contracts: Arc::new(RwLock::new(HashMap::new())),
        }
    }

    /// Check contract for threats
    pub fn check_contract(&self, contract: &str) -> Vec<ThreatDetection> {
        let mut detections = Vec::new();
        
        let malicious = self.malicious_contracts.read();
        if malicious.contains_key(contract) {
            detections.push(ThreatDetection {
                threat_type: ThreatType::Malware,
                threat_level: ThreatLevel::Critical,
                confidence: 100,
                description: "Known malicious contract".to_string(),
                metadata: HashMap::new(),
            });
        }
        
        detections
    }

    /// Calculate wallet risk score
    pub fn calculate_wallet_risk(&self, wallet: &str) -> WalletRiskScore {
        let mut factors = Vec::new();
        let mut score: u8 = 0;
        
        // Check if wallet has existing risk score
        let scores = self.wallet_scores.read();
        if let Some(existing) = scores.get(wallet) {
            return existing.clone();
        }
        
        // Default: low risk for new wallets
        if score < 20 {
            score = 20;
            factors.push("New wallet".to_string());
        }
        
        let risk_level = match score {
            0..=20 => ThreatLevel::Low,
            21..=50 => ThreatLevel::Medium,
            51..=80 => ThreatLevel::High,
            _ => ThreatLevel::Critical,
        };
        
        WalletRiskScore {
            wallet: wallet.to_string(),
            risk_score: score,
            risk_level,
            factors,
            last_updated: chrono::Utc::now().timestamp(),
        }
    }

    /// Check transaction
    pub fn check_transaction(&self, tx: &str, from: &str, to: &str) -> FirewallAction {
        let rules = self.firewall_rules.read();
        
        // Check wallet risk
        let wallet_score = self.calculate_wallet_risk(from);
        if wallet_score.risk_level == ThreatLevel::Critical {
            return FirewallAction::Block;
        }
        
        // Check rules
        for rule in rules.iter() {
            if rule.is_active && tx.contains(&rule.pattern) {
                return match rule.action {
                    FirewallAction::Block => FirewallAction::Block,
                    FirewallAction::Challenge => FirewallAction::Challenge,
                    _ => FirewallAction::Monitor,
                };
            }
        }
        
        FirewallAction::Allow
    }

    /// Add malicious contract
    pub fn add_malicious_contract(&self, contract: &str) {
        let mut malicious = self.malicious_contracts.write();
        malicious.insert(contract.to_string(), true);
    }

    /// Add firewall rule
    pub fn add_rule(&self, rule: FirewallRule) {
        let mut rules = self.firewall_rules.write();
        rules.push(rule);
    }

    /// Get statistics
    pub fn stats(&self) -> SecurityStats {
        let malicious = self.malicious_contracts.read();
        let rules = self.firewall_rules.read();
        
        SecurityStats {
            malicious_contracts: malicious.len(),
            firewall_rules: rules.len(),
            active_rules: rules.iter().filter(|r| r.is_active).count(),
        }
    }
}

impl Default for SecuritySystems {
    fn default() -> Self {
        Self::new()
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SecurityStats {
    pub malicious_contracts: usize,
    pub firewall_rules: usize,
    pub active_rules: usize,
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_security() {
        let systems = SecuritySystems::new();
        
        let score = systems.calculate_wallet_risk("0x1234");
        assert!(score.risk_score >= 20);
    }

    #[test]
    fn test_firewall() {
        let systems = SecuritySystems::new();
        
        let action = systems.check_transaction("swap()", "0x1234", "0x5678");
        assert_eq!(action, FirewallAction::Allow);
    }
}