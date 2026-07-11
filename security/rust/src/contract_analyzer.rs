/**
 * Contract Analyzer Module
 * Static analysis and bytecode verification
 * 
 * @author TigerSwap
 * @version 1.0.0 Production
 */

use crate::{Result, SecurityError, SecurityIssue, Severity, IssueCategory};
use regex::Regex;
use serde::{Deserialize, Serialize};

/// Analysis result
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct AnalysisResult {
    pub lines_of_code: usize,
    pub functions: Vec<FunctionInfo>,
    pub contracts: Vec<ContractInfo>,
    pub imports: Vec<String>,
    pub libraries: Vec<String>,
}

/// Function information
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct FunctionInfo {
    pub name: String,
    pub visibility: String,
    pub modifiers: Vec<String>,
    pub line_number: usize,
    pub is_external: bool,
    pub is_payable: bool,
    pub parameters: Vec<String>,
    pub returns: Vec<String>,
}

/// Contract information
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ContractInfo {
    pub name: String,
    pub line_number: usize,
    pub is_interface: bool,
    pub is_abstract: bool,
    pub parent_contracts: Vec<String>,
    pub state_variables: Vec<StateVariable>,
}

/// State variable
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct StateVariable {
    pub name: String,
    pub type_name: String,
    pub visibility: String,
    pub is_constant: bool,
    pub is_immutable: bool,
}

/// Contract analyzer
pub struct ContractAnalyzer {
    function_regex: Regex,
    contract_regex: Regex,
    import_regex: Regex,
    library_regex: Regex,
}

impl ContractAnalyzer {
    pub fn new() -> Self {
        Self {
            function_regex: Regex::new(
                r"function\s+(\w+)\s*\(([^)]*)\)\s*(?:public|external|internal|private|view|pure|payable)?\s*(?:returns\s*\(([^)]*)\))?\s*\{?"
            ).unwrap(),
            contract_regex: Regex::new(
                r"(contract|interface|library)\s+(\w+)(?:\s+is\s+([^{]+))?"
            ).unwrap(),
            import_regex: Regex::new(r#"import\s+["']([^"']+)["']"#).unwrap(),
            library_regex: Regex::new(r"using\s+(\w+)\s+for").unwrap(),
        }
    }
    
    /// Analyze Solidity source code
    pub fn analyze(&self, source_code: &str) -> Result<AnalysisResult> {
        let lines_of_code = source_code.lines().count();
        
        let functions = self.extract_functions(source_code)?;
        let contracts = self.extract_contracts(source_code)?;
        let imports = self.extract_imports(source_code);
        let libraries = self.extract_libraries(source_code);
        
        Ok(AnalysisResult {
            lines_of_code,
            functions,
            contracts,
            imports,
            libraries,
        })
    }
    
    /// Analyze bytecode for issues
    pub fn analyze_bytecode(&self, bytecode: &str) -> Result<Vec<SecurityIssue>> {
        let mut issues = Vec::new();
        
        // Remove 0x prefix if present
        let bytecode = bytecode.trim_start_matches("0x");
        
        // Check for common bytecode issues
        // Self-destruct
        if bytecode.contains("ff") {
            issues.push(SecurityIssue {
                id: "bytecode-selfdestruct".to_string(),
                title: "Self-destruct instruction found".to_string(),
                description: "Contract contains SELFDESTRUCT (0xff) instruction".to_string(),
                severity: Severity::High,
                category: IssueCategory::EtherLeak,
                line_number: None,
                function_name: None,
                contract_name: None,
                recommendation: "Ensure selfdestruct is properly protected with access control".to_string(),
                cwe_id: Some("CWE-841"),
                confidence: 0.9,
            });
        }
        
        // Delegatecall
        if bytecode.contains("f4") {
            issues.push(SecurityIssue {
                id: "bytecode-delegatecall".to_string(),
                title: "Delegatecall instruction found".to_string(),
                description: "Contract contains DELEGATECALL (0xf4) instruction".to_string(),
                severity: Severity::High,
                category: IssueCategory::UnsafeDelegatecall,
                line_number: None,
                function_name: None,
                contract_name: None,
                recommendation: "Ensure delegatecall target is trusted and verified".to_string(),
                cwe_id: Some("CWE-829"),
                confidence: 0.85,
            });
        }
        
        // Staticcall
        if bytecode.contains("fa") {
            issues.push(SecurityIssue {
                id: "bytecode-staticcall".to_string(),
                title: "Staticcall instruction found".to_string(),
                description: "Contract contains STATICCALL (0xfa) instruction".to_string(),
                severity: Severity::Informational,
                category: IssueCategory::Other,
                line_number: None,
                function_name: None,
                contract_name: None,
                recommendation: "Static calls are generally safe".to_string(),
                cwe_id: None,
                confidence: 0.95,
            });
        }
        
        // Check for missing EIP-170 size limit
        if bytecode.len() > 24576 {
            issues.push(SecurityIssue {
                id: "bytecode-size".to_string(),
                title: "Contract size exceeds EIP-170 limit".to_string(),
                description: "Bytecode size exceeds 24KB limit".to_string(),
                severity: Severity::High,
                category: IssueCategory::DenialOfService,
                line_number: None,
                function_name: None,
                contract_name: None,
                recommendation: "Reduce contract size by using libraries or splitting into multiple contracts".to_string(),
                cwe_id: Some("CWE-400"),
                confidence: 1.0,
            });
        }
        
        Ok(issues)
    }
    
    fn extract_functions(&self, source_code: &str) -> Result<Vec<FunctionInfo>> {
        let mut functions = Vec::new();
        
        for (line_num, line) in source_code.lines().enumerate() {
            if let Some(caps) = self.function_regex.captures(line) {
                let name = caps.get(1).map(|m| m.as_str().to_string()).unwrap_or_default();
                let params = caps.get(2).map(|m| m.as_str()).unwrap_or("");
                let returns = caps.get(3).map(|m| m.as_str()).unwrap_or("");
                
                let is_payable = line.contains("payable");
                let is_external = line.contains("external") || line.contains("public");
                let visibility = if line.contains("private") {
                    "private".to_string()
                } else if line.contains("internal") {
                    "internal".to_string()
                } else {
                    "public".to_string()
                };
                
                let modifiers: Vec<String> = source_code.lines()
                    .nth(line_num)
                    .map(|l| {
                        l.split_whitespace()
                            .filter(|w| !w.contains("function") && !w.contains("(") && !w.contains(")"))
                            .map(|s| s.trim_matches(|c| c == '{' || c == '}' || c == ',').to_string())
                            .filter(|s| !s.is_empty() && s != "public" && s != "private" && s != "internal" && s != "external" && s != "view" && s != "pure" && s != "payable" && s != "returns")
                            .collect()
                    })
                    .unwrap_or_default();
                
                functions.push(FunctionInfo {
                    name,
                    visibility,
                    modifiers,
                    line_number: line_num + 1,
                    is_external,
                    is_payable,
                    parameters: params.split(',').map(|s| s.trim().to_string()).filter(|s| !s.is_empty()).collect(),
                    returns: returns.split(',').map(|s| s.trim().to_string()).filter(|s| !s.is_empty()).collect(),
                });
            }
        }
        
        Ok(functions)
    }
    
    fn extract_contracts(&self, source_code: &str) -> Result<Vec<ContractInfo>> {
        let mut contracts = Vec::new();
        
        for (line_num, line) in source_code.lines().enumerate() {
            if let Some(caps) = self.contract_regex.captures(line) {
                let contract_type = caps.get(1).map(|m| m.as_str()).unwrap_or("contract");
                let name = caps.get(2).map(|m| m.as_str().to_string()).unwrap_or_default();
                let parents = caps.get(3).map(|m| m.as_str()).unwrap_or("");
                
                let is_interface = contract_type == "interface";
                let is_abstract = line.contains("abstract");
                
                // Extract state variables
                let state_vars = self.extract_state_variables(source_code, &name);
                
                contracts.push(ContractInfo {
                    name,
                    line_number: line_num + 1,
                    is_interface,
                    is_abstract,
                    parent_contracts: parents.split(',').map(|s| s.trim().to_string()).filter(|s| !s.is_empty()).collect(),
                    state_variables: state_vars,
                });
            }
        }
        
        Ok(contracts)
    }
    
    fn extract_state_variables(&self, source_code: &str, contract_name: &str) -> Vec<StateVariable> {
        let mut variables = Vec::new();
        let var_regex = Regex::new(r"(\w+)\s+(\w+)\s*(?:public|private|internal)?\s*(?:constant|immutable)?\s*;").unwrap();
        
        // Simple extraction - in production would need more sophisticated parsing
        for line in source_code.lines() {
            if line.contains("contract") && line.contains(contract_name) {
                // Found the contract, look for state variables after this
                continue;
            }
            
            if let Some(caps) = var_regex.captures(line) {
                let type_name = caps.get(1).map(|m| m.as_str()).unwrap_or("");
                let name = caps.get(2).map(|m| m.as_str()).unwrap_or("");
                
                // Skip local variables (inside functions)
                if !line.contains("function") && !line.contains("{") && !line.contains("}") {
                    if !name.is_empty() && !type_name.is_empty() {
                        variables.push(StateVariable {
                            name: name.to_string(),
                            type_name: type_name.to_string(),
                            visibility: "internal".to_string(),
                            is_constant: line.contains("constant"),
                            is_immutable: line.contains("immutable"),
                        });
                    }
                }
            }
        }
        
        variables
    }
    
    fn extract_imports(&self, source_code: &str) -> Vec<String> {
        let mut imports = Vec::new();
        
        for line in source_code.lines() {
            if let Some(caps) = self.import_regex.captures(line) {
                if let Some(path) = caps.get(1) {
                    imports.push(path.as_str().to_string());
                }
            }
        }
        
        imports
    }
    
    fn extract_libraries(&self, source_code: &str) -> Vec<String> {
        let mut libraries = Vec::new();
        
        for line in source_code.lines() {
            if let Some(caps) = self.library_regex.captures(line) {
                if let Some(lib) = caps.get(1) {
                    libraries.push(lib.as_str().to_string());
                }
            }
        }
        
        libraries
    }
}

impl Default for ContractAnalyzer {
    fn default() -> Self {
        Self::new()
    }
}
