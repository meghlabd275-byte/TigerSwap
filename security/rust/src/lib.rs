/**
 * TigerSwap Security Module
 * Production-ready security engine for smart contract verification and audit
 * 
 * Features:
 * - Smart contract vulnerability detection
 * - Static analysis engine
 * - Formal verification helpers
 * - Access control validation
 * - Reentrancy detection
 * - Integer overflow/underflow detection
 * - Front-running protection
 * - MEV detection
 * 
 * @author TigerSwap
 * @version 1.0.0 Production
 */

pub mod contract_analyzer;
pub mod vulnerability_scanner;
pub mod access_control;
pub mod formal_verification;
pub mod mev_protection;

pub use contract_analyzer::*;
pub use vulnerability_scanner::*;
pub use access_control::*;
pub use formal_verification::*;
pub use mev_protection::*;

use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use std::path::Path;
use thiserror::Error;

pub type Result<T> = std::result::Result<T, SecurityError>;

#[derive(Error, Debug, Clone, Serialize, Deserialize)]
pub enum SecurityError {
    #[error("Analysis error: {0}")]
    AnalysisError(String),
    
    #[error("Contract not found: {0}")]
    ContractNotFound(String),
    
    #[error("Invalid bytecode: {0}")]
    InvalidBytecode(String),
    
    #[error("Vulnerability detected: {0}")]
    VulnerabilityDetected(String),
    
    #[error("Access control violation: {0}")]
    AccessControlViolation(String),
    
    #[error("Formal verification failed: {0}")]
    VerificationFailed(String),
}

/// Severity levels for security issues
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum Severity {
    Critical,
    High,
    Medium,
    Low,
    Informational,
}

impl Severity {
    pub fn as_str(&self) -> &'static str {
        match self {
            Severity::Critical => "CRITICAL",
            Severity::High => "HIGH",
            Severity::Medium => "MEDIUM",
            Severity::Low => "LOW",
            Severity::Informational => "INFO",
        }
    }
}

/// Category of security issue
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum IssueCategory {
    Reentrancy,
    AccessControl,
    Arithmetic,
    UncheckedCall,
    FrontRunning,
    Manipulation,
    DenialOfService,
    UnknownCall,
    UninitializedStorage,
    EtherLeak,
    UnexpectedEther,
    Timestamp,
    BlockGasLimit,
    TxOrigin,
    FunctionSelectorCollision,
    MissingReturn,
    UnsafeDelegatecall,
    Deprecated,
    Other,
}

/// A detected security issue
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SecurityIssue {
    pub id: String,
    pub title: String,
    pub description: String,
    pub severity: Severity,
    pub category: IssueCategory,
    pub line_number: Option<usize>,
    pub function_name: Option<String>,
    pub contract_name: Option<String>,
    pub recommendation: String,
    pub cwe_id: Option<String>,
    pub confidence: f32,
}

/// Result of security analysis
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SecurityReport {
    pub contract_name: String,
    pub contract_address: Option<String>,
    pub chain_id: u64,
    pub analyzed_at: i64,
    pub issues: Vec<SecurityIssue>,
    pub score: u32,
    pub passed: bool,
    pub summary: SecuritySummary,
}

/// Summary statistics
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SecuritySummary {
    pub critical_count: usize,
    pub high_count: usize,
    pub medium_count: usize,
    pub low_count: usize,
    pub info_count: usize,
    pub total_issues: usize,
    pub lines_analyzed: usize,
    pub functions_analyzed: usize,
    pub analysis_time_ms: u64,
}

impl SecurityReport {
    pub fn new(contract_name: String, chain_id: u64) -> Self {
        Self {
            contract_name,
            contract_address: None,
            chain_id,
            analyzed_at: chrono::Utc::now().timestamp(),
            issues: Vec::new(),
            score: 100,
            passed: true,
            summary: SecuritySummary {
                critical_count: 0,
                high_count: 0,
                medium_count: 0,
                low_count: 0,
                info_count: 0,
                total_issues: 0,
                lines_analyzed: 0,
                functions_analyzed: 0,
                analysis_time_ms: 0,
            },
        }
    }
    
    pub fn add_issue(&mut self, issue: SecurityIssue) {
        match issue.severity {
            Severity::Critical => self.summary.critical_count += 1,
            Severity::High => self.summary.high_count += 1,
            Severity::Medium => self.summary.medium_count += 1,
            Severity::Low => self.summary.low_count += 1,
            Severity::Informational => self.summary.info_count += 1,
        }
        
        // Calculate score deduction
        let deduction = match issue.severity {
            Severity::Critical => 30,
            Severity::High => 15,
            Severity::Medium => 7,
            Severity::Low => 3,
            Severity::Informational => 1,
        };
        
        self.score = self.score.saturating_sub(deduction);
        
        if issue.severity == Severity::Critical || issue.severity == Severity::High {
            self.passed = false;
        }
        
        self.issues.push(issue);
        self.summary.total_issues += 1;
    }
    
    pub fn to_json(&self) -> Result<String> {
        serde_json::to_string_pretty(self)
            .map_err(|e| SecurityError::AnalysisError(e.to_string()))
    }
}

/// Main security engine
pub struct SecurityEngine {
    scanner: VulnerabilityScanner,
    analyzer: ContractAnalyzer,
    access_control: AccessControlValidator,
    formal_verifier: FormalVerification,
    mev_protector: MEVProtection,
}

impl SecurityEngine {
    pub fn new() -> Self {
        Self {
            scanner: VulnerabilityScanner::new(),
            analyzer: ContractAnalyzer::new(),
            access_control: AccessControlValidator::new(),
            formal_verifier: FormalVerification::new(),
            mev_protector: MEVProtection::new(),
        }
    }
    
    /// Analyze a smart contract
    pub async fn analyze_contract(
        &self,
        contract_name: String,
        source_code: &str,
        bytecode: Option<&str>,
        chain_id: u64,
    ) -> Result<SecurityReport> {
        let start = std::time::Instant::now();
        
        let mut report = SecurityReport::new(contract_name, chain_id);
        
        // Run static analysis
        let analysis_result = self.analyzer.analyze(source_code)?;
        report.summary.lines_analyzed = analysis_result.lines_of_code;
        report.summary.functions_analyzed = analysis_result.functions.len();
        
        // Check for vulnerabilities
        let issues = self.scanner.scan(source_code)?;
        for issue in issues {
            report.add_issue(issue);
        }
        
        // Validate access control
        let access_issues = self.access_control.validate(source_code)?;
        for issue in access_issues {
            report.add_issue(issue);
        }
        
        // Verify critical patterns
        let verification_issues = self.formal_verifier.verify(source_code)?;
        for issue in verification_issues {
            report.add_issue(issue);
        }
        
        // Check bytecode if provided
        if let Some(bytecode) = bytecode {
            let bytecode_issues = self.analyzer.analyze_bytecode(bytecode)?;
            for issue in bytecode_issues {
                report.add_issue(issue);
            }
        }
        
        report.summary.analysis_time_ms = start.elapsed().as_millis() as u64;
        
        Ok(report)
    }
    
    /// Quick scan for critical vulnerabilities only
    pub fn quick_scan(&self, source_code: &str) -> Result<Vec<SecurityIssue>> {
        let mut issues = Vec::new();
        
        // Reentrancy check
        issues.extend(self.scanner.check_reentrancy(source_code)?);
        
        // Access control check
        issues.extend(self.access_control.check_critical(source_code)?);
        
        Ok(issues)
    }
    
    /// Verify contract deployment safety
    pub fn verify_deployment(&self, source_code: &str) -> Result<bool> {
        let issues = self.quick_scan(source_code)?;
        
        // Check for critical issues
        for issue in issues {
            if issue.severity == Severity::Critical || issue.severity == Severity::High {
                return Ok(false);
            }
        }
        
        Ok(true)
    }
}

impl Default for SecurityEngine {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_security_report_scoring() {
        let mut report = SecurityReport::new("TestContract".to_string(), 1);
        
        // Add critical issue
        report.add_issue(SecurityIssue {
            id: "1".to_string(),
            title: "Reentrancy".to_string(),
            description: "Reentrancy vulnerability detected".to_string(),
            severity: Severity::Critical,
            category: IssueCategory::Reentrancy,
            line_number: Some(100),
            function_name: Some("withdraw".to_string()),
            contract_name: None,
            recommendation: "Use checks-effects-interactions pattern".to_string(),
            cwe_id: Some("CWE-841".to_string()),
            confidence: 0.95,
        });
        
        assert!(!report.passed);
        assert_eq!(report.score, 70); // 100 - 30
    }
    
    #[test]
    fn test_quick_scan() {
        let engine = SecurityEngine::new();
        
        // Vulnerable contract
        let code = r#"
            pragma solidity ^0.8.0;
            
            contract Vulnerable {
                mapping(address => uint) public balances;
                
                function withdraw() public {
                    uint bal = balances[msg.sender];
                    (bool sent,) = msg.sender.call{value: bal}("");
                    require(sent, "Failed to send Ether");
                    balances[msg.sender] = 0;
                }
            }
        "#;
        
        let issues = engine.quick_scan(code).unwrap();
        assert!(!issues.is_empty());
    }
}
