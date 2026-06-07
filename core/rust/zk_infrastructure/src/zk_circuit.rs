//! ZK Circuit Module

use serde::{Deserialize, Serialize};

/// ZK Circuit
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ZKCircuit {
    pub circuit_id: String,
    pub name: String,
    pub description: String,
    pub num_inputs: u32,
    pub num_constraints: u32,
    pub gate_depth: u32,
    pub created_at: i64,
    pub circuit_type: CircuitType,
}

impl ZKCircuit {
    pub fn new(circuit_id: String, name: String, num_inputs: u32) -> Self {
        Self {
            circuit_id,
            name,
            description: String::new(),
            num_inputs,
            num_constraints: 0,
            gate_depth: 0,
            created_at: chrono::Utc::now().timestamp(),
            circuit_type: CircuitType::Arithmetic,
        }
    }
    
    pub fn with_constraints(mut self, constraints: u32) -> Self {
        self.num_constraints = constraints;
        self
    }
    
    pub fn with_depth(mut self, depth: u32) -> Self {
        self.gate_depth = depth;
        self
    }
}

/// Circuit type
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum CircuitType {
    Arithmetic,
    Hash,
    Signature,
    MerkleTree,
    Swap,
    OrderBook,
    Identity,
    AgeVerification,
}

impl Default for ZKCircuit {
    fn default() -> Self {
        Self::new(
            uuid::Uuid::new_v4().to_string(),
            "default".to_string(),
            0,
        )
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_circuit() {
        let circuit = ZKCircuit::new("test".to_string(), "Test".to_string(), 2);
        assert_eq!(circuit.num_inputs, 2);
    }
}