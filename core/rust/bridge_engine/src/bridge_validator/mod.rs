//! Bridge Validator
//! 
//! Validates cross-chain messages and transactions.

use thiserror::Error;

#[derive(Error, Debug)]
pub enum ValidationError {
    #[error("Invalid signature")]
    InvalidSignature,
    #[error("Invalid message")]
    InvalidMessage,
    #[error("Expired")]
    Expired,
}

#[derive(Debug, Clone)]
pub struct ValidatorConfig {
    pub max_age_seconds: u64,
    pub min_confirmations: u64,
}

impl Default for ValidatorConfig {
    fn default() -> Self {
        Self {
            max_age_seconds: 3600,
            min_confirmations: 12,
        }
    }
}

pub struct BridgeValidator {
    config: ValidatorConfig,
}

impl BridgeValidator {
    pub fn new() -> Self {
        Self {
            config: ValidatorConfig::default(),
        }
    }
    
    pub fn validate_message(&self, message: &[u8], _signatures: &[&[u8]]) -> Result<(), ValidationError> {
        if message.is_empty() {
            return Err(ValidationError::InvalidMessage);
        }
        // Simplified - real impl would verify signatures
        Ok(())
    }
    
    pub fn validate_source(&self, source_chain: u32, _sender: &[u8]) -> bool {
        // Simplified - real impl would check allowlist
        source_chain > 0
    }
    
    pub fn validate_destination(&self, dest_chain: u32, _receiver: &[u8]) -> bool {
        dest_chain > 0
    }
    
    pub fn check_expiry(&self, timestamp: u64) -> Result<(), ValidationError> {
        let now = std::time::SystemTime::now()
            .duration_since(std::time::UNIX_EPOCH)
            .unwrap()
            .as_secs();
        
        if now > timestamp && now - timestamp > self.config.max_age_seconds {
            return Err(ValidationError::Expired);
        }
        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_validate() {
        let validator = BridgeValidator::new();
        let result = validator.validate_message(b"test", &[&[0u8; 65]]);
        assert!(result.is_ok());
    }
}