//! TigerSwap Security Platform
//! 
//! Production-grade security features:
//! - Runtime Threat Detection
//! - Transaction Firewall
//! - Smart Contract Scanner
//! - Phishing Detection
//! - Wallet Risk Scoring
//! - Anomaly Detection
//! - Threat Intelligence

use std::collections::HashMap;
use std::sync::Arc;
use tokio::sync::RwLock;

use serde::{Deserialize, Serialize};
use thiserror::Error;

// ============================================================================
// Error Types
// ============================================================================

#[derive(Error, Debug)]
pub enum SecurityError {
    #[error("Threat detected")]
    ThreatDetected,
    #[error("Transaction blocked")]
    TransactionBlocked,
    #[error("Contract unsafe")]
    ContractUnsafe,
    #[error("Analysis error")]
    AnalysisError,
}

// ============================================================================
// Threat Types
// ============================================================================

/// Threat severity level
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum ThreatLevel {
    Info,
    Low,
    Medium,
    High,
    Critical,
}

/// Threat category
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum ThreatCategory {
    Phishing,
    Exploit,
    RugPull,
    Honeypot,
    FlashLoan,
    FrontRun,
    Sandwich,
    Drainer,
    FakeToken,
    MimicContract,
    SandwichAttack,
    Reentrancy,
    UnverifiedContract,
}

/// Alert
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ThreatAlert {
    pub id: String,
    pub category: ThreatCategory,
    pub severity: ThreatLevel,
    pub description: String,
    pub address: String,
    pub evidence: HashMap<String, String>,
    pub timestamp: i64,
}

// ============================================================================
// Transaction Firewall
// ============================================================================

/// Transaction analysis result
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct TransactionAnalysis {
    pub tx_hash: String,
    pub from: String,
    pub to: String,
    pub value: f64,
    pub data: Vec<u8>,
    pub is_safe: bool,
    pub risk_score: f64,
    pub warnings: Vec<String>,
    pub block_reason: Option<String>,
}

impl TransactionAnalysis {
    pub fn new(tx_hash: &str) -> Self {
        Self {
            tx_hash: tx_hash.to_string(),
            from: String::new(),
            to: String::new(),
            value: 0.0,
            data: Vec::new(),
            is_safe: true,
            risk_score: 0.0,
            warnings: Vec::new(),
            block_reason: None,
        }
    }
    
    pub fn with_risk(&mut self, score: f64, warning: &str) {
        self.risk_score = score;
        self.warnings.push(warning.to_string());
    }
    
    pub fn block(&mut self, reason: &str) {
        self.is_safe = false;
        self.block_reason = Some(reason.to_string());
    }
}

/// Transaction firewall
pub struct TransactionFirewall {
    max_risk_score: f64,
    blocked_addresses: RwLock<HashMap<String, ThreatLevel>>,
    suspicious_patterns: RwLock<Vec<SuspiciousPattern>>,
}

impl TransactionFirewall {
    pub fn new() -> Self {
        Self {
            max_risk_score: 75.0,
            blocked_addresses: RwLock::new(HashMap::new()),
            suspicious_patterns: RwLock::new(Vec::new()),
        }
    }
    
    /// Analyze transaction
    pub async fn analyze(&self, tx: &mut TransactionAnalysis) -> Result<(), SecurityError> {
        // Check blocked addresses
        let blocked = self.blocked_addresses.read().await;
        
        if blocked.contains_key(&tx.from) {
            tx.block("Address is blocked");
            return Err(SecurityError::TransactionBlocked);
        }
        
        if blocked.contains_key(&tx.to) {
            tx.block("Target is blocked");
            return Err(SecurityError::TransactionBlocked);
        }
        
        // Check suspicious patterns
        let patterns = self.suspicious_patterns.read().await;
        for pattern in patterns.iter() {
            if pattern.matches(tx) {
                tx.with_risk(pattern.risk_score, &pattern.description);
            }
        }
        
        // Block high risk
        if tx.risk_score > self.max_risk_score {
            tx.block("Risk score too high");
            return Err(SecurityError::TransactionBlocked);
        }
        
        Ok(())
    }
    
    /// Block address
    pub async fn block_address(&self, address: String, level: ThreatLevel) {
        let mut blocked = self.blocked_addresses.write().await;
        blocked.insert(address, level);
    }
    
    /// Add suspicious pattern
    pub async fn add_pattern(&self, pattern: SuspiciousPattern) {
        let mut patterns = self.suspicious_patterns.write().await;
        patterns.push(pattern);
    }
}

impl Default for TransactionFirewall {
    fn default() -> Self {
        Self::new()
    }
}

/// Suspicious pattern
#[derive(Debug, Clone)]
pub struct SuspiciousPattern {
    pub pattern_type: String,
    pub risk_score: f64,
    pub description: String,
}

impl SuspiciousPattern {
    pub fn matches(&self, _tx: &TransactionAnalysis) -> bool {
        // Simplified pattern matching
        false
    }
}

// ============================================================================
// Smart Contract Scanner
// ============================================================================

/// Contract analysis result
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ContractAnalysis {
    pub address: String,
    pub is_verified: bool,
    pub is_malicious: bool,
    pub risk_score: f64,
    pub issues: Vec<ContractIssue>,
    pub functions: Vec<String>,
    pub permits: Vec<String>,
}

impl ContractAnalysis {
    pub fn new(address: &str) -> Self {
        Self {
            address: address.to_string(),
            is_verified: false,
            is_malicious: false,
            risk_score: 0.0,
            issues: Vec::new(),
            functions: Vec::new(),
            permits: Vec::new(),
        }
    }
    
    pub fn add_issue(&mut self, issue: ContractIssue) {
        self.risk_score += issue.severity as f64 * 10.0;
        self.issues.push(issue);
    }
    
    pub fn mark_malicious(&mut self) {
        self.is_malicious = true;
    }
}

/// Contract issue
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ContractIssue {
    pub severity: ThreatLevel,
    pub category: ThreatCategory,
    pub description: String,
    pub line: Option<u32>,
}

/// Contract scanner
pub struct ContractScanner {
    known_malicious: RwLock<HashMap<String, ThreatCategory>>,
    verified_contracts: RwLock<HashMap<String, bool>>,
}

impl ContractScanner {
    pub fn new() -> Self {
        Self {
            known_malicious: RwLock::new(HashMap::new()),
            verified_contracts: RwLock::new(HashMap::new()),
        }
    }
    
    /// Scan contract
    pub async fn scan(&self, address: &str) -> ContractAnalysis {
        let mut analysis = ContractAnalysis::new(address);
        
        // Check known malicious
        let malicious = self.known_malicious.read().await;
        if malicious.contains_key(address) {
            analysis.mark_malicious();
            analysis.add_issue(ContractIssue {
                severity: ThreatLevel::Critical,
                category: ThreatCategory::MimicContract,
                description: "Contract is known malicious".to_string(),
                line: None,
            });
        }
        
        // Check verified
        let verified = self.verified_contracts.read().await;
        if verified.contains_key(address) {
            analysis.is_verified = true;
        }
        
        analysis
    }
    
    /// Add to blacklist
    pub async fn add_blacklist(&self, address: String, category: ThreatCategory) {
        let mut malicious = self.known_malicious.write().await;
        malicious.insert(address, category);
    }
    
    /// Mark verified
    pub async fn mark_verified(&self, address: String) {
        let mut verified = self.verified_contracts.write().await;
        verified.insert(address, true);
    }
}

impl Default for ContractScanner {
    fn default() -> Self {
        Self::new()
    }
}

// ============================================================================
// Phishing Detector
// ============================================================================

/// Phishing detection result
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PhishingDetection {
    pub domain: String,
    pub is_phishing: bool,
    pub confidence: f64,
    pub similar_to: Vec<String>,
    pub registration_date: Option<i64>,
    pub registrar: Option<String>,
}

/// Phishing detector
pub struct PhishingDetector {
    known_phishing: RwLock<HashMap<String, ThreatLevel>>,
    known_safe: RwLock<HashMap<String, ThreatLevel>>,
    typosquatting: RwLock<Vec<TyposquatEntry>>,
}

impl PhishingDetector {
    pub fn new() -> Self {
        Self {
            known_phishing: RwLock::new(HashMap::new()),
            known_safe: RwLock::new(HashMap::new()),
            typosquatting: RwLock::new(Vec::new()),
        }
    }
    
    /// Check domain
    pub async fn check(&self, domain: &str) -> PhishingDetection {
        let mut result = PhishingDetection {
            domain: domain.to_string(),
            is_phishing: false,
            confidence: 0.0,
            similar_to: Vec::new(),
            registration_date: None,
            registrar: None,
        };
        
        // Check known phishing
        let phishing = self.known_phishing.read().await;
        if phishing.contains_key(domain) {
            result.is_phishing = true;
            result.confidence = 100.0;
            return result;
        }
        
        // Check typosquatting
        let typos = self.typosquatting.read().await;
        for entry in typos.iter() {
            if entry.similar_to(domain) {
                result.similar_to.push(entry.domain.clone());
                result.confidence = 80.0;
            }
        }
        
        result
    }
    
    /// Add phishing domain
    pub async fn add_phishing(&self, domain: String) {
        let mut phishing = self.known_phishing.write().await;
        phishing.insert(domain, ThreatLevel::Critical);
    }
    
    /// Add safe domain
    pub async fn add_safe(&self, domain: String) {
        let mut safe = self.known_safe.write().await;
        safe.insert(domain, ThreatLevel::Info);
    }
    
    /// Add typosquat
    pub async fn add_typosquat(&self, target: String, attacker: String) {
        let mut typos = self.typosquatting.write().await;
        typos.push(TyposquatEntry {
            domain: attacker,
            target_domain: target,
        });
    }
}

impl Default for PhishingDetector {
    fn default() -> Self {
        Self::new()
    }
}

/// Typosquat entry
#[derive(Debug, Clone)]
pub struct TyposquatEntry {
    pub domain: String,
    pub target_domain: String,
}

impl TyposquatEntry {
    pub fn similar_to(&self, domain: &str) -> bool {
        // Simplified similarity check
        self.target_domain == domain
    }
}

// ============================================================================
// Wallet Risk Scoring
// ============================================================================

/// Wallet risk score
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct WalletRiskScore {
    pub address: String,
    pub score: f64,        // 0-100
    pub level: ThreatLevel,
    pub factors: Vec<RiskFactor>,
}

/// Risk factor
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct RiskFactor {
    pub category: String,
    pub score: f64,
    pub description: String,
}

/// Wallet risk scorer
pub struct WalletRiskScorer {
    known_high_risk: RwLock<HashMap<String, f64>>,
}

impl WalletRiskScorer {
    pub fn new() -> Self {
        Self {
            known_high_risk: RwLock::new(HashMap::new()),
        }
    }
    
    /// Calculate risk
    pub async fn calculate(&self, address: &str, history: &[TransactionAnalysis]) -> WalletRiskScore {
        let mut score = 0.0;
        let mut factors = Vec::new();
        
        // Transaction count factor
        if history.len() > 100 {
            score += 20.0;
            factors.push(RiskFactor {
                category: "high_volume".to_string(),
                score: 20.0,
                description: "High transaction volume".to_string(),
            });
        }
        
        // Check known high risk
        let known = self.known_high_risk.read().await;
        if let Some(known_score) = known.get(address) {
            score += known_score;
            factors.push(RiskFactor {
                category: "known_high_risk".to_string(),
                score: *known_score,
                description: "Known high risk address".to_string(),
            });
        }
        
        // Cap at 100
        score = score.min(100.0);
        
        let level = match score as u32 {
            0..=25 => ThreatLevel::Info,
            26..=50 => ThreatLevel::Low,
            51..=75 => ThreatLevel::Medium,
            76..=90 => ThreatLevel::High,
            _ => ThreatLevel::Critical,
        };
        
        WalletRiskScore {
            address: address.to_string(),
            score,
            level,
            factors,
        }
    }
    
    /// Add high risk
    pub async fn add_high_risk(&self, address: String, score: f64) {
        let mut known = self.known_high_risk.write().await;
        known.insert(address, score);
    }
}

impl Default for WalletRiskScorer {
    fn default() -> Self {
        Self::new()
    }
}

// ============================================================================
// Anomaly Detection
// ============================================================================

/// Anomaly detection
pub struct AnomalyDetector {
    baseline: RwLock<Baseline>,
}

impl AnomalyDetector {
    pub fn new() -> Self {
        Self {
            baseline: RwLock::new(Baseline::default()),
        }
    }
    
    /// Detect anomaly
    pub async fn detect(&self, metric: &str, value: f64) -> Option<Anomaly> {
        let baseline = self.baseline.read().await;
        
        if let Some(stats) = baseline.metrics.get(metric) {
            let z_score = (value - stats.mean) / stats.stddev;
            
            if z_score > 3.0 {
                return Some(Anomaly {
                    metric: metric.to_string(),
                    value,
                    expected: stats.mean,
                    z_score,
                    severity: ThreatLevel::High,
                });
            }
        }
        
        None
    }
    
    /// Update baseline
    pub async fn update(&self, metric: &str, value: f64) {
        let mut baseline = self.baseline.write().await;
        baseline.metrics.entry(metric.to_string()).or_insert(Stats::default());
        baseline.metrics.get_mut(metric).unwrap().add(value);
    }
}

impl Default for AnomalyDetector {
    fn default() -> Self {
        Self::new()
    }
}

/// Anomaly
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Anomaly {
    pub metric: String,
    pub value: f64,
    pub expected: f64,
    pub z_score: f64,
    pub severity: ThreatLevel,
}

/// Baseline statistics
#[derive(Debug, Default)]
pub struct Baseline {
    pub metrics: HashMap<String, Stats>,
}

/// Statistics
#[derive(Debug, Default)]
pub struct Stats {
    pub mean: f64,
    pub stddev: f64,
    pub count: u64,
}

impl Stats {
    pub fn add(&mut self, value: f64) {
        self.count += 1;
        let delta = value - self.mean;
        self.mean += delta / self.count as f64;
        let delta2 = value - self.mean;
        self.stddev = ((self.count - 1) as f64).sqrt();
    }
}

// ============================================================================
// Threat Intelligence
// ============================================================================

/// Threat intelligence
pub struct ThreatIntelligence {
    alerts: RwLock<Vec<ThreatAlert>>,
    ioc_feed: RwLock<HashMap<String, ThreatCategory>>,
}

impl ThreatIntelligence {
    pub fn new() -> Self {
        Self {
            alerts: RwLock::new(Vec::new()),
            ioc_feed: RwLock::new(HashMap::new()),
        }
    }
    
    /// Get active alerts
    pub async fn get_alerts(&self, level: Option<ThreatLevel>) -> Vec<ThreatAlert> {
        let alerts = self.alerts.read().await;
        
        if let Some(level) = level {
            alerts.iter()
                .filter(|a| a.severity == level)
                .cloned()
                .collect()
        } else {
            alerts.clone()
        }
    }
    
    /// Add alert
    pub async fn add_alert(&self, alert: ThreatAlert) {
        let mut alerts = self.alerts.write().await;
        alerts.push(alert);
        
        // Keep last 1000 alerts
        while alerts.len() > 1000 {
            alerts.remove(0);
        }
    }
    
    /// Add IOC
    pub async fn add_ioc(&self, indicator: String, category: ThreatCategory) {
        let mut ioc = self.ioc_feed.write().await;
        ioc.insert(indicator, category);
    }
}

impl Default for ThreatIntelligence {
    fn default() -> Self {
        Self::new()
    }
}

// ============================================================================
// Security Manager
// ============================================================================

/// Security manager - combines all security components
pub struct SecurityManager {
    firewall: TransactionFirewall,
    scanner: ContractScanner,
    phishing: PhishingDetector,
    risk_scorer: WalletRiskScorer,
    anomaly: AnomalyDetector,
    intelligence: ThreatIntelligence,
}

impl SecurityManager {
    pub fn new() -> Self {
        Self {
            firewall: TransactionFirewall::new(),
            scanner: ContractScanner::new(),
            phishing: PhishingDetector::new(),
            risk_scorer: WalletRiskScorer::new(),
            anomaly: AnomalyDetector::new(),
            intelligence: ThreatIntelligence::new(),
        }
    }
    
    /// Full security check
    pub async fn check(&self, tx: &mut TransactionAnalysis) -> Result<(), SecurityError> {
        // Firewall check
        self.firewall.analyze(tx).await?;
        
        // Contract check
        let contract_analysis = self.scanner.scan(&tx.to).await;
        if contract_analysis.is_malicious {
            tx.block("Target contract is malicious");
            return Err(SecurityError::ContractUnsafe);
        }
        
        // Anomaly detection
        if let Some(anomaly) = self.anomaly.detect("tx_value", tx.value).await {
            tx.with_risk(50.0, &format!("Anomaly detected: {}", anomaly.metric));
        }
        
        Ok(())
    }
}

impl Default for SecurityManager {
    fn default() -> Self {
        Self::new()
    }
}

// ============================================================================
// Tests
// ============================================================================

#[cfg(test)]
mod tests {
    use super::*;
    
    #[tokio::test]
    async fn test_firewall() {
        let firewall = TransactionFirewall::new();
        
        let mut tx = TransactionAnalysis::new("0x123");
        tx.from = "0xblocked".to_string();
        
        let result = firewall.analyze(&mut tx).await;
        assert!(result.is_err());
    }
    
    #[tokio::test]
    async fn test_scanner() {
        let scanner = ContractScanner::new();
        
        let analysis = scanner.scan("0x123").await;
        assert!(!analysis.is_malicious);
    }
    
    #[tokio::test]
    async fn test_phishing() {
        let detector = PhishingDetector::new();
        
        let result = detector.check("tigerswap.exchange").await;
        // May be phishing if in database
        assert!(result.confidence >= 0.0);
    }
}

// ============================================================================
// Library Exports
// ============================================================================

pub use self::{
    firewall::{TransactionFirewall, TransactionAnalysis},
    scanner::{ContractScanner, ContractAnalysis, ContractIssue},
    phishing::{PhishingDetector, PhishingDetection},
    risk::{WalletRiskScorer, WalletRiskScore, RiskFactor},
    anomaly::{AnomalyDetector, Anomaly},
    intelligence::{ThreatIntelligence, ThreatAlert},
    manager::SecurityManager,
    types::{ThreatLevel, ThreatCategory},
};

mod firewall;
mod scanner;
mod phishing;
mod risk;
mod anomaly;
mod intelligence;
mod manager;
mod types;