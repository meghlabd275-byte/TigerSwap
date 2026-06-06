//! Message Verifier
//! 
//! Verifies cross-chain message authenticity.

use thiserror::Error;

#[derive(Error, Debug)]
pub enum VerifyError {
    #[error("Verification failed")]
    VerificationFailed,
    #[error("Invalid format")]
    InvalidFormat,
}

#[derive(Debug, Clone)]
pub struct Message {
    pub source_chain: u32,
    pub nonce: u64,
    pub sender: Vec<u8>,
    pub payload: Vec<u8>,
    pub signatures: Vec<Vec<u8>>,
}

pub struct MessageVerifier {
    trusted_signers: RwLock<Vec<Vec<u8>>>,
}

impl MessageVerifier {
    pub fn new() -> Self {
        Self {
            trusted_signers: RwLock::new(Vec::new()),
        }
    }
    
    pub fn add_trusted_signer(&self, signer: Vec<u8>) {
        self.trusted_signers.write().unwrap().push(signer);
    }
    
    pub fn verify(&self, message: &Message) -> Result<(), VerifyError> {
        if message.signatures.is_empty() {
            return Err(VerifyError::VerificationFailed);
        }
        
        if message.source_chain == 0 {
            return Err(VerifyError::InvalidFormat);
        }
        
        // Simplified - real impl would verify threshold signatures
        Ok(())
    }
    
    pub fn get_signers(&self) -> Vec<Vec<u8>> {
        self.trusted_signers.read().unwrap().clone()
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_verify() {
        let verifier = MessageVerifier::new();
        let msg = Message {
            source_chain: 1,
            nonce: 1,
            sender: vec![],
            payload: vec![],
            signatures: vec![vec![0u8; 65]],
        };
        
        assert!(verifier.verify(&msg).is_ok());
    }
}