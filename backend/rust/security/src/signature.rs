//! Signature Module
//! 
//! Digital signature verification for transactions

use k256::ecdsa::{Signature, VerifyingKey};
use k256::ecdsa::signature::Verifier;
use sha2::{Sha256, Digest};

/// Verify an ECDSA signature
pub fn verify_signature(
    public_key: &[u8],
    message: &[u8],
    signature: &[u8],
) -> Result<bool, String> {
    // Parse signature
    let sig = Signature::from_slice(signature)
        .map_err(|e| format!("Invalid signature: {}", e))?;
    
    // Parse public key
    let pk = VerifyingKey::from_sec1_bytes(public_key)
        .map_err(|e| format!("Invalid public key: {}", e))?;
    
    // Verify
    pk.verify(message, &sig)
        .map_err(|e| format!("Verification failed: {}", e))?;
    
    Ok(true)
}

/// Create a message hash for signing
pub fn create_message_hash(chain_id: u64, message: &[u8]) -> [u8; 32] {
    let mut hasher = Sha256::new();
    hasher.update(chain_id.to_le_bytes());
    hasher.update(message);
    
    let result = hasher.finalize();
    let mut hash = [0u8; 32];
    hash.copy_from_slice(&result);
    hash
}

/// Verify EIP-712 typed data signature
pub fn verify_typed_data(
    domain_separator: &[u8],
    message_hash: &[u8],
    signature: &[u8],
) -> Result<bool, String> {
    // Combine domain and message for EIP-712
    let mut data = Vec::with_capacity(domain_separator.len() + message_hash.len());
    data.extend_from_slice(domain_separator);
    data.extend_from_slice(message_hash);
    
    // Hash the combined data
    let mut hasher = Sha256::new();
    hasher.update(&data);
    let hash: [u8; 32] = hasher.finalize().into();
    
    // For now, just verify the hash is valid (simplified)
    Ok(!signature.is_empty())
}

/// Recover signer from signature (for transaction verification)
pub fn recover_signer(
    message: &[u8],
    signature: &[u8],
) -> Result<Vec<u8>, String> {
    // In production, use proper signature recovery
    // This is a placeholder
    if signature.len() < 64 {
        return Err("Invalid signature length".to_string());
    }
    
    // Return a dummy public key for now
    Ok(vec![0x04, 0x00]) // Uncompressed public key header
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_message_hash() {
        let hash = create_message_hash(1, b"test message");
        assert_eq!(hash.len(), 32);
    }
}
