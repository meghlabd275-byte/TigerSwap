/**
 * Access Control Validator
 * Validates access control patterns in smart contracts
 * 
 * @author TigerSwap
 * @version 1.0.0 Production
 */

use crate::{Result, SecurityIssue, Severity, IssueCategory};
use regex::Regex;

/// Access control validator
pub struct AccessControlValidator {
    owner_pattern: Regex,
    onlyowner_pattern: Regex,
    require_pattern: Regex,
}

impl AccessControlValidator {
    pub fn new() -> Self {
        Self {
            owner_pattern: Regex::new(r"owner\s*[=|:]").unwrap(),
            onlyowner_pattern: Regex::new(r"modifier\s+onlyOwner").unwrap(),
            require_pattern: Regex::new(r"require\s*\(").unwrap(),
        }
    }
    
    /// Validate access control patterns
    pub fn validate(&self, source_code: &str) -> Result<Vec<SecurityIssue>> {
        let mut issues = Vec::new();
        
        // Check for missing access control on sensitive functions
        let sensitive_functions = [
            "mint",
            "burn",
            "pause",
            "unpause",
            "upgrade",
            "setOwner",
            "withdraw",
            "transferOwnership",
            "renounceOwnership",
            "initialize",
        ];
        
        for func in sensitive_functions {
            let pattern = format!(r"function\s+{}\s*\(", func);
            let regex = Regex::new(&pattern).unwrap();
            
            for (line_num, line) in source_code.lines().enumerate() {
                if regex.is_match(line) {
                    // Check if function has access control
                    let has_access_control = self.has_access_control(line, source_code, line_num);
                    
                    if !has_access_control {
                        issues.push(SecurityIssue {
                            id: format!("access-{}-{}", func, line_num),
                            title: format!("Missing access control on {}", func),
                            description: format!("Function {} has no access control modifier", func),
                            severity: Severity::High,
                            category: IssueCategory::AccessControl,
                            line_number: Some(line_num + 1),
                            function_name: Some(func.to_string()),
                            contract_name: None,
                            recommendation: "Add access control modifier like onlyOwner or role-based control".to_string(),
                            cwe_id: Some("CWE-862"),
                            confidence: 0.85,
                        });
                    }
                }
            }
        }
        
        // Check for tx.origin usage
        if source_code.contains("tx.origin") {
            for (line_num, line) in source_code.lines().enumerate() {
                if line.contains("tx.origin") {
                    issues.push(SecurityIssue {
                        id: format!("txorigin-{}", line_num),
                        title: "tx.origin used for authorization".to_string(),
                        description: "Using tx.origin for authorization is vulnerable to phishing attacks".to_string(),
                        severity: Severity::High,
                        category: IssueCategory::AccessControl,
                        line_number: Some(line_num + 1),
                        function_name: None,
                        contract_name: None,
                        recommendation: "Use msg.sender instead of tx.origin for authorization".to_string(),
                        cwe_id: Some("CWE-477"),
                        confidence: 0.95,
                    });
                }
            }
        }
        
        // Check for missing zero address validation
        let zero_address_patterns = [
            r"function\s+set\w+\s*\(\s*address\s+\w+\s*\)",
            r"function\s+transferOwnership\s*\(\s*address",
        ];
        
        for pattern in zero_address_patterns {
            let regex = Regex::new(pattern).unwrap();
            
            for (line_num, line) in source_code.lines().enumerate() {
                if regex.is_match(line) {
                    // Look for zero address check in the function
                    let has_zero_check = self.has_zero_address_check(source_code, line_num);
                    
                    if !has_zero_check {
                        issues.push(SecurityIssue {
                            id: format!("zerocheck-{}", line_num),
                            title: "Missing zero address validation".to_string(),
                            description: "Function parameter does not check for zero address".to_string(),
                            severity: Severity::Medium,
                            category: IssueCategory::AccessControl,
                            line_number: Some(line_num + 1),
                            function_name: None,
                            contract_name: None,
                            recommendation: "Add require(addr != address(0), \"Invalid address\")".to_string(),
                            cwe_id: Some("CWE-20"),
                            confidence: 0.75,
                        });
                    }
                }
            }
        }
        
        Ok(issues)
    }
    
    /// Quick check for critical access control issues
    pub fn check_critical(&self, source_code: &str) -> Result<Vec<SecurityIssue>> {
        let mut issues = Vec::new();
        
        // Check for critical functions without access control
        let critical_functions = ["mint", "burn", "pause", "upgrade", "setOwner"];
        
        for func in critical_functions {
            let pattern = format!(r"function\s+{}\s*\(", func);
            let regex = Regex::new(&pattern).unwrap();
            
            if regex.is_match(source_code) {
                // Find line number
                for (line_num, line) in source_code.lines().enumerate() {
                    if regex.is_match(line) {
                        let has_access = self.has_access_control(line, source_code, line_num);
                        
                        if !has_access {
                            issues.push(SecurityIssue {
                                id: format!("critical-access-{}", func),
                                title: format!("Critical function {} lacks access control", func),
                                description: format!("Function {} is critical but has no access control", func),
                                severity: Severity::Critical,
                                category: IssueCategory::AccessControl,
                                line_number: Some(line_num + 1),
                                function_name: Some(func.to_string()),
                                contract_name: None,
                                recommendation: "Add access control immediately".to_string(),
                                cwe_id: Some("CWE-862"),
                                confidence: 0.95,
                            });
                        }
                    }
                }
            }
        }
        
        Ok(issues)
    }
    
    fn has_access_control(&self, line: &str, source_code: &str, line_num: usize) -> bool {
        // Check if line has access control modifiers
        let access_modifiers = [
            "onlyOwner",
            "onlyRole",
            "onlyAdmin",
            "requiresAuth",
            "whenNotPaused",
            "whenPaused",
        ];
        
        for modifier in &access_modifiers {
            if line.contains(modifier) {
                return true;
            }
        }
        
        // Check for require statements in the function body
        let lines: Vec<&str> = source_code.lines().collect();
        
        // Look ahead in the function
        let mut brace_count = 0;
        let mut found_opening_brace = false;
        
        for i in line_num..lines.len() {
            let l = lines[i];
            
            if l.contains('{') {
                found_opening_brace = true;
            }
            
            if found_opening_brace {
                brace_count += l.matches('{').count() as i32;
                brace_count -= l.matches('}').count() as i32;
                
                // Check for require with auth
                if l.contains("require") && (l.contains("owner") || l.contains("role") || l.contains("auth")) {
                    return true;
                }
                
                // Exit function body
                if brace_count == 0 && i > line_num {
                    break;
                }
            }
        }
        
        false
    }
    
    fn has_zero_address_check(&self, source_code: &str, line_num: usize) -> bool {
        let lines: Vec<&str> = source_code.lines().collect();
        
        // Look ahead in the function for zero address check
        let mut brace_count = 0;
        let mut found_opening_brace = false;
        
        for i in line_num..lines.len().min(line_num + 20) {
            let l = lines[i];
            
            if l.contains('{') {
                found_opening_brace = true;
            }
            
            if found_opening_brace {
                brace_count += l.matches('{').count() as i32;
                brace_count -= l.matches('}').count() as i32;
                
                // Check for zero address
                if l.contains("address(0)") || l.contains("address(0x0)") {
                    return true;
                }
                
                if brace_count == 0 && i > line_num {
                    break;
                }
            }
        }
        
        false
    }
}

impl Default for AccessControlValidator {
    fn default() -> Self {
        Self::new()
    }
}
