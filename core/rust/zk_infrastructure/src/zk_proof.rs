//! ZK Proof Module

use serde::{Deserialize, Serialize};

/// ZK Proof
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ZKProof {
    pub proof_id: String,
    pub circuit_id: String,
    pub public_inputs: Vec<Vec<u8>>,
    pub private_inputs: Vec<Vec<u8>>,
    pub proof_data: Vec<u8>,
    pub created_at: i64,
    pub proof_type: ProofType,
    pub curve: CurveType,
}

impl ZKProof {
    pub fn new(circuit_id: String, proof_type: ProofType) -> Self {
        Self {
            proof_id: uuid::Uuid::new_v4().to_string(),
            circuit_id,
            public_inputs: Vec::new(),
            private_inputs: Vec::new(),
            proof_data: Vec::new(),
            created_at: chrono::Utc::now().timestamp(),
            proof_type,
            curve: CurveType::BN254,
        }
    }
    
    pub fn verify(&self) -> bool {
        !self.proof_data.is_empty()
    }
}

/// Proof type
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum ProofType {
    SNARK,
    STARK,
    PLONK,
    Halo2,
    Groth16,
}

/// Curve type
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum CurveType {
    BN254,
    BLS12_381,
    ED25519,
    SECP256K1,
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_proof() {
        let proof = ZKProof::new("test".to_string(), ProofType::SNARK);
        assert!(proof.verify());
    }
}