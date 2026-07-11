/**
 * Formal Verification Module
 * Helper functions for formal verification of smart contracts
 * 
 * @author TigerSwap
 * @version 1.0.0 Production
 */

use crate::{Result, SecurityIssue, Severity, IssueCategory};
use regex::Regex;

/// Formal verification helper
pub struct FormalVerification {
    invariant_patterns: Vec<Regex>,
}

impl FormalVerification {
    pub fn new() -> Self {
        Self {
            invariant_patterns: vec![
                // Reentrancy invariant
                Regex::new(r"balance\s*[-+]").unwrap(),
                // Access control invariants
                Regex::new(r"owner\s*==").unwrap(),
            ],
        }
    }
    
    /// Verify contract properties
    pub fn verify(&self, source_code: &str) -> Result<Vec<SecurityIssue>> {
        let mut issues = Vec::new();
        
        // Check for common verification properties
        
        // 1. Check for correct checks-effects-interactions pattern
        issues.extend(self.verify_checks_effects_interactions(source_code)?);
        
        // 2. Check for correct initialization
        issues.extend(self.verify_initialization(source_code)?);
        
        // 3. Check for pausable functionality
        issues.extend(self.verify_pausable(source_code)?);
        
        // 4. Check for safe math operations
        issues.extend(self.verify_safety(source_code)?);
        
        Ok(issues)
    }
    
    fn verify_checks_effects_interactions(&self, source_code: &str) -> Result<Vec<SecurityIssue>> {
        let mut issues = Vec::new();
        
        // Find functions with external calls
        let call_pattern = Regex::new(r"(?i)(call|transfer|send)\s*\{").unwrap();
        
        for (line_num, line) in source_code.lines().enumerate() {
            if call_pattern.is_match(line) {
                // Check if there's a state change before this call
                let preceding_lines: String = source_code.lines()
                    .take(line_num)
                    .rev()
                    .take(10)
                    .collect::<Vec<_>>()
                    .into_iter()
                    .rev()
                    .collect();
                
                // Look for state variable assignment before the external call
                if preceding_lines.contains('=') {
                    // This could be a potential issue - state change before external call
                    issues.push(SecurityIssue {
                        id: format!("cei-{}", line_num),
                        title: "Potential violation of checks-effects-interactions".to_string(),
                        description: "External call may follow state modification".to_string(),
                        severity: Severity::Medium,
                        category: IssueCategory::Reentrancy,
                        line_number: Some(line_num + 1),
                        function_name: None,
                        contract_name: None,
                        recommendation: "Move all state updates before external calls".to_string(),
                        cwe_id: Some("CWE-841"),
                        confidence: 0.7,
                    });
                }
            }
        }
        
        Ok(issues)
    }
    
    fn verify_initialization(&self, source_code: &str) -> Result<Vec<SecurityIssue>> {
        let mut issues = Vec::new();
        
        // Check for initialize functions
        if source_code.contains("function initialize") {
            // Check if initializer is protected
            let init_pattern = Regex::new(
                r"function\s+initialize\s*\([^)]*\)\s*(?:public|external)"
            ).unwrap();
            
            for (line_num, line) in source_code.lines().enumerate() {
                if init_pattern.is_match(line) {
                    if !line.contains("initializer") && !line.contains("initializer") {
                        // Check next lines for require
                        let next_lines: String = source_code.lines()
                            .skip(line_num)
                            .take(5)
                            .collect::<Vec<_>>()
                            .join("\n");
                        
                        if !next_lines.contains("require") {
                            issues.push(SecurityIssue {
                                id: "init-1".to_string(),
                                title: "Unprotected initializer function".to_string(),
                                description: "Initialize function can be called by anyone".to_string(),
                                severity: Severity::Critical,
                                category: IssueCategory::AccessControl,
                                line_number: Some(line_num + 1),
                                function_name: Some("initialize".to_string()),
                                contract_name: None,
                                recommendation: "Add initializer modifier and access control".to_string(),
                                cwe_id: Some("CWE-862"),
                                confidence: 0.9,
                            });
                        }
                    }
                }
            }
        }
        
        // Check for missing constructor
        if source_code.contains("contract ") && !source_code.contains("constructor") {
            // Look for owner initialization
            if source_code.contains("owner") && !source_code.contains("=") {
                // May be uninitialized
                issues.push(SecurityIssue {
                    id: "owner-uninit".to_string(),
                    title: "Owner may be uninitialized".to_string(),
                    description: "Contract owner is not explicitly initialized".to_string(),
                    severity: Severity::Medium,
                    category: IssueCategory::UninitializedStorage,
                    line_number: None,
                    function_name: None,
                    contract_name: None,
                    recommendation: "Initialize owner in constructor".to_string(),
                    cwe_id: Some("CWE-665"),
                    confidence: 0.6,
                });
            }
        }
        
        Ok(issues)
    }
    
    fn verify_pausable(&self, source_code: &str) -> Result<Vec<SecurityIssue>> {
        let mut issues = Vec::new();
        
        // Check if contract has pausable functionality
        let has_pausable = source_code.contains("Pausable") || source_code.contains("whenNotPaused");
        let has_critical = source_code.contains("withdraw") || 
                           source_code.contains("transfer") || 
                           source_code.contains("mint");
        
        if has_critical && !has_pausable {
            issues.push(SecurityIssue {
                id: "no-pause".to_string(),
                title: "Critical functions without pause mechanism".to_string(),
                description: "Contract has critical functions but no pause functionality".to_string(),
                severity: Severity::Low,
                category: IssueCategory::AccessControl,
                line_number: None,
                function_name: None,
                contract_name: None,
                recommendation: "Consider adding pausable functionality for emergency response".to_string(),
                cwe_id: None,
                confidence: 0.5,
            });
        }
        
        Ok(issues)
    }
    
    fn verify_safety(&self, source_code: &str) -> Result<Vec<SecurityIssue>> {
        let mut issues = Vec::new();
        
        // Check for safe division
        let division_pattern = Regex::new(r"/\s*\w+").unwrap();
        
        for (line_num, line) in source_code.lines().enumerate() {
            if division_pattern.is_match(line) {
                // Check if there's a zero check
                if !line.contains("require") && !line.contains("if") {
                    issues.push(SecurityIssue {
                        id: format!("div-{}", line_num),
                        title: "Potential division by zero".to_string(),
                        description: "Division without zero check".to_string(),
                        severity: Severity::Medium,
                        category: IssueCategory::Arithmetic,
                        line_number: Some(line_num + 1),
                        function_name: None,
                        contract_name: None,
                        recommendation: "Add require(divisor != 0) before division".to_string(),
                        cwe_id: Some("CWE-369"),
                        confidence: 0.7,
                    });
                }
            }
        }
        
        // Check for array length manipulation
        let array_pattern = Regex::new(r"\w+\s*\[\s*\w+\s*\]\s*=").unwrap();
        
        for (line_num, line) in source_code.lines().enumerate() {
            if array_pattern.is_match(line) {
                // Check for out of bounds protection
                let next_lines: String = source_code.lines()
                    .skip(line_num)
                    .take(3)
                    .collect::<Vec<_>>()
                    .join("\n");
                
                if !next_lines.contains("require") && !next_lines.contains("if") {
                    issues.push(SecurityIssue {
                        id: format!("array-{}", line_num),
                        title: "Potential array out of bounds".to_string(),
                        description: "Array access without bounds check".to_string(),
                        severity: Severity::Medium,
                        category: IssueCategory::DenialOfService,
                        line_number: Some(line_num + 1),
                        function_name: None,
                        contract_name: None,
                        recommendation: "Add bounds check before array access".to_string(),
                        cwe_id: Some("CWE-129"),
                        confidence: 0.65,
                    });
                }
            }
        }
        
        Ok(issues)
    }
}

impl Default for FormalVerification {
    fn default() -> Self {
        Self::new()
    }
}
