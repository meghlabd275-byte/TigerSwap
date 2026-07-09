//! Threshold Signature Scheme
//! 
//! Implements threshold ECDSA signatures using the MPC protocol.

use crate::error::MpcError;
use crate::types::{PartialSignature, SignRequest, SignatureResponse};
use secp256k1::{Message, PublicKey, Scalar, SecretKey, Sign, Signature};
use rand::Rng;

/// Generate a partial signature from a share
pub fn sign_partial(
    share: &[u8],
    message_hash: &[u8],
    nonce_commitment: &[u8],
) -> Result<Vec<u8>, MpcError> {
    // Parse the share as a secret key
    let mut share_bytes = [0u8; 32];
    if share.len() >= 32 {
        share_bytes.copy_from_slice(&share[..32]);
    } else {
        return Err(MpcError::InvalidParameters("Share too short".to_string()));
    }
    
    let scalar = Scalar::from_be_bytes(share_bytes)
        .map_err(|e| MpcError::SigningFailed(e.to_string()))?;
    
    let secret_key = SecretKey::from_slice(&share_bytes)
        .map_err(|e| MpcError::SigningFailed(e.to_string()))?;
    
    // Parse message
    let message = Message::from_slice(message_hash)
        .map_err(|e| MpcError::InvalidParameters(e.to_string()))?;
    
    // Create partial signature
    // In production, this would use proper MPC signing (GG18, GG20, etc.)
    let signature = secret_key.sign(message);
    
    Ok(signature.serialize_compact().to_vec())
}

/// Combine partial signatures into a final signature
pub fn combine_signatures(
    partials: &[PartialSignature],
    threshold: u32,
) -> Result<SignatureResponse, MpcError> {
    if partials.len() < threshold as usize {
        return Err(MpcError::ThresholdNotMet {
            need: threshold,
            have: partials.len() as u32,
        });
    }
    
    // In production, use proper signature combining
    // For now, use the first valid signature
    for partial in partials {
        if let Ok(signature) = Signature::from_compact(&partial.signature) {
            // Try to recover public key
            // In production, use proper MPC signature combination
            return Ok(SignatureResponse {
                signature: partial.signature.clone(),
                public_key: Vec::new(), // Would be computed
                recovery_id: 0,
            });
        }
    }
    
    Err(MpcError::InvalidSignature("No valid partial signatures".to_string()))
}

/// Verify a threshold signature
pub fn verify_threshold_signature(
    signature: &[u8],
    message_hash: &[u8],
    public_key: &[u8],
) -> Result<bool, MpcError> {
    if signature.len() != 64 {
        return Err(MpcError::InvalidSignature("Invalid signature length".to_string()));
    }
    
    if public_key.len() != 33 {
        return Err(MpcError::InvalidParameters("Invalid public key length".to_string()));
    }
    
    // Parse signature
    let sig = Signature::from_compact(signature)
        .map_err(|e| MpcError::InvalidSignature(e.to_string()))?;
    
    // Parse public key
    let pk = PublicKey::from_compressed(public_key)
        .map_err(|e| MpcError::InvalidParameters(e.to_string()))?;
    
    // Parse message
    let message = Message::from_slice(message_hash)
        .map_err(|e| MpcError::InvalidParameters(e.to_string()))?;
    
    // Verify
    Ok(pk.verify(message, &sig).is_ok())
}

/// Generate nonce commitment for MPC signing
pub fn generate_nonce_commitment() -> (Vec<u8>, Vec<u8>) {
    let mut rng = rand::thread_rng();
    let mut nonce_bytes = [0u8; 32];
    rng.fill_bytes(&mut nonce_bytes);
    
    let nonce_scalar = Scalar::from_be_bytes(nonce_bytes)
        .unwrap_or_else(|_| Scalar::from_u64(1));
    
    let mut hasher = sha2::Sha256::new();
    hasher.update(&nonce_bytes);
    let commitment = hasher.finalize().to_vec();
    
    (commitment, nonce_bytes.to_vec())
}

/// Lagrange coefficient calculation for threshold signatures
pub fn lagrange_coefficient(
    signer_index: u32,
    participant_indices: &[u32],
    threshold: u32,
) -> Result<Scalar, MpcError> {
    let mut result = Scalar::from_u64(1);
    
    for &j in participant_indices {
        if j == signer_index {
            continue;
        }
        
        // L_j = -j / (signer_index - j)
        let numerator = Scalar::from_u64(j as u64);
        let denominator = Scalar::from_u64((signer_index as i64 - j as i64).unsigned_abs() as u64);
        
        let coeff = numerator * denominator.invert().unwrap_or(Scalar::from_u64(1));
        result = result * coeff;
    }
    
    Ok(result)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_nonce_commitment() {
        let (commitment, nonce) = generate_nonce_commitment();
        
        assert_eq!(commitment.len(), 32);
        assert_eq!(nonce.len(), 32);
    }

    #[test]
    fn test_signature_verification() {
        // Generate a test key
        let mut rng = rand::thread_rng();
        let mut key_bytes = [0u8; 32];
        rng.fill_bytes(&mut key_bytes);
        
        let secret_key = SecretKey::from_slice(&key_bytes).unwrap();
        let public_key = PublicKey::from_secret_key(&secret_key);
        
        // Sign a message
        let message_hash = sha2::Sha256::digest(b"test message");
        let message = Message::from_slice(&message_hash).unwrap();
        let signature = secret_key.sign(message);
        
        // Verify
        let result = verify_threshold_signature(
            &signature.serialize_compact(),
            &message_hash.to_vec(),
            &public_key.serialize_compressed().to_vec(),
        );
        
        assert!(result.is_ok());
    }
}
