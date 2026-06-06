//! Bridge Executor
//! 
//! Executes cross-chain transactions.

use std::sync::{Arc, RwLock};
use std::collections::HashMap;
use thiserror::Error;

#[derive(Error, Debug)]
pub enum ExecutorError {
    #[error("Execution failed: {0}")]
    ExecutionFailed(String),
    #[error("Insufficient gas: {0}")]
    InsufficientGas(String),
    #[error("Timeout")]
    Timeout,
}

#[derive(Debug, Clone)]
pub struct ExecutionRequest {
    pub id: String,
    pub to_chain: u32,
    pub to: Vec<u8>,
    pub data: Vec<u8>,
    pub gas_limit: u64,
}

#[derive(Debug, Clone)]
pub struct ExecutionResult {
    pub request_id: String,
    pub success: bool,
    pub tx_hash: Option<String>,
    pub error: Option<String>,
}

pub struct BridgeExecutor {
    pending: RwLock<HashMap<String, ExecutionRequest>>,
    results: RwLock<HashMap<String, ExecutionResult>>,
}

impl BridgeExecutor {
    pub fn new() -> Self {
        Self {
            pending: RwLock::new(HashMap::new()),
            results: RwLock::new(HashMap::new()),
        }
    }
    
    pub fn submit(&self, request: ExecutionRequest) -> String {
        let id = request.id.clone();
        self.pending.write().unwrap().insert(id.clone(), request);
        id
    }
    
    pub fn execute(&self, id: &str) -> Result<ExecutionResult, ExecutorError> {
        let _req = self.pending.write().unwrap()
            .remove(id)
            .ok_or_else(|| ExecutorError::ExecutionFailed("not found".to_string()))?;
        
        // Simplified - real execution would call chain
        let result = ExecutionResult {
            request_id: id.to_string(),
            success: true,
            tx_hash: Some(format!("0x{}", id)),
            error: None,
        };
        
        self.results.write().unwrap().insert(id.to_string(), result.clone());
        Ok(result)
    }
    
    pub fn get_result(&self, id: &str) -> Option<ExecutionResult> {
        self.results.read().unwrap().get(id).cloned()
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_execute() {
        let executor = BridgeExecutor::new();
        let req = ExecutionRequest {
            id: "test-1".to_string(),
            to_chain: 1,
            to: vec![],
            data: vec![],
            gas_limit: 100000,
        };
        
        let id = executor.submit(req);
        let result = executor.execute(&id).unwrap();
        assert!(result.success);
    }
}