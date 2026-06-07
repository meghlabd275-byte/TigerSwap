//! Public Key Share Module - Verification keys for threshold signatures

use secp256k1::PublicKey;
use serde::{Deserialize, Serialize};

/// Public key share for secp256k1 (compressed format)
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub struct PublicKeyShare([u8; 33]);

impl PublicKeyShare {
    /// Create from secp256k1 public key
    pub fn from_secp256k1(pk: &PublicKey) -> Self {
        Self(pk.serialize())
    }
    
    /// Get raw bytes
    pub fn as_bytes(&self) -> &[u8; 33] {
        &self.0
    }
    
    /// Get hex string
    pub fn to_hex(&self) -> String {
        hex::encode(self.0)
    }
    
    /// Import from hex string
    pub fn from_hex(hex: &str) -> Result<Self, &'static str> {
        let bytes = hex::decode(hex).map_err(|_| "Invalid hex")?;
        if bytes.len() != 33 {
            return Err("Invalid public key length");
        }
        let mut key = [0u8; 33];
        key.copy_from_slice(&bytes);
        Ok(Self(key))
    }
    
    /// Convert to ethereum address
    pub fn to_ethereum_address(&self) -> String {
        use sha2::{Digest, Sha256};
        let hash = Sha256::digest(&self.0);
        let address = &hash[12..];
        format!("0x{}", hex::encode(address))
    }
}

/// Combined public key from threshold signature
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CombinedPublicKey {
    pub key: [u8; 33],
    pub threshold: u32,
    pub total_shares: u32,
}

impl CombinedPublicKey {
    pub fn to_hex(&self) -> String {
        hex::encode(self.key)
    }
    
    pub fn to_ethereum_address(&self) -> String {
        use sha2::{Digest, Sha256};
        let hash = Sha256::digest(&self.key);
        let address = &hash[12..];
        format!("0x{}", hex::encode(address))
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_address_generation() {
        let key = PublicKeyShare([0u8; 33]);
        let address = key.to_ethereum_address();
        
        assert!(address.starts_with("0x"));
        assert_eq!(address.len(), 42);
    }
}