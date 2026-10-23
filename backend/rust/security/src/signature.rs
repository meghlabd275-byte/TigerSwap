//! Signature Module
//!
//! ECDSA (secp256k1) verification and public-key recovery for TigerSwap
//! transactions, including EIP-191 personal-message and EIP-712 typed-data
//! hashing. No placeholders: every function performs real cryptographic work.

use k256::ecdsa::signature::hazmat::PrehashVerifier;
use k256::ecdsa::{RecoveryId, Signature, VerifyingKey};
use sha3::{Digest, Keccak256};

/// Errors produced by this module.
#[derive(Debug, PartialEq, Eq)]
pub enum SignatureError {
    InvalidSignature(String),
    InvalidPublicKey(String),
    VerificationFailed,
    RecoveryFailed(String),
    InvalidLength(String),
}

impl std::fmt::Display for SignatureError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            SignatureError::InvalidSignature(m) => write!(f, "invalid signature: {m}"),
            SignatureError::InvalidPublicKey(m) => write!(f, "invalid public key: {m}"),
            SignatureError::VerificationFailed => write!(f, "signature verification failed"),
            SignatureError::RecoveryFailed(m) => write!(f, "signature recovery failed: {m}"),
            SignatureError::InvalidLength(m) => write!(f, "invalid length: {m}"),
        }
    }
}

impl std::error::Error for SignatureError {}

/// Verify an ECDSA signature over a raw 32-byte digest. The digest is treated
/// as a prehash, so it is not hashed again.
pub fn verify_signature(
    public_key: &[u8],
    digest: &[u8],
    signature: &[u8],
) -> Result<bool, SignatureError> {
    if digest.len() != 32 {
        return Err(SignatureError::InvalidLength(format!(
            "digest must be 32 bytes, got {}",
            digest.len()
        )));
    }

    let sig = Signature::from_slice(signature)
        .map_err(|e| SignatureError::InvalidSignature(e.to_string()))?;

    let pk = VerifyingKey::from_sec1_bytes(public_key)
        .map_err(|e| SignatureError::InvalidPublicKey(e.to_string()))?;

    pk.verify_prehash(digest, &sig)
        .map_err(|_| SignatureError::VerificationFailed)?;

    Ok(true)
}

/// Keccak-256 hash of a chain-scoped message, matching the TigerSwap signer.
pub fn create_message_hash(chain_id: u64, message: &[u8]) -> [u8; 32] {
    let mut hasher = Keccak256::new();
    hasher.update(chain_id.to_be_bytes());
    hasher.update(message);
    hasher.finalize().into()
}

/// EIP-191 personal message hash.
pub fn personal_message_hash(message: &[u8]) -> [u8; 32] {
    let prefix = format!("\x19Ethereum Signed Message:\n{}", message.len());
    let mut hasher = Keccak256::new();
    hasher.update(prefix.as_bytes());
    hasher.update(message);
    hasher.finalize().into()
}

/// EIP-712 typed-data digest: keccak256("\x19\x01" || domainSeparator || structHash).
pub fn typed_data_hash(domain_separator: &[u8; 32], struct_hash: &[u8; 32]) -> [u8; 32] {
    let mut hasher = Keccak256::new();
    hasher.update([0x19, 0x01]);
    hasher.update(domain_separator);
    hasher.update(struct_hash);
    hasher.finalize().into()
}

/// Verify an EIP-712 typed-data signature against an expected signer address.
pub fn verify_typed_data(
    domain_separator: &[u8; 32],
    struct_hash: &[u8; 32],
    signature: &[u8],
    expected_signer: &[u8; 20],
) -> Result<bool, SignatureError> {
    let digest = typed_data_hash(domain_separator, struct_hash);
    let recovered = recover_address(&digest, signature)?;
    Ok(recovered == *expected_signer)
}

/// Split a 65-byte signature into its ECDSA signature and recovery id.
fn split_signature(signature: &[u8]) -> Result<(Signature, RecoveryId), SignatureError> {
    if signature.len() != 65 {
        return Err(SignatureError::InvalidLength(format!(
            "signature must be 65 bytes, got {}",
            signature.len()
        )));
    }

    let sig = Signature::from_slice(&signature[..64])
        .map_err(|e| SignatureError::InvalidSignature(e.to_string()))?;

    // Ethereum encodes the recovery parity in the final byte as 27/28 (legacy)
    // or 0/1 (raw). Accept both.
    let v = signature[64];
    let parity = match v {
        0 | 1 => v,
        27 | 28 => v - 27,
        other => {
            return Err(SignatureError::InvalidSignature(format!(
                "invalid recovery byte {other}"
            )))
        }
    };

    let recid = RecoveryId::from_byte(parity)
        .ok_or_else(|| SignatureError::InvalidSignature("invalid recovery id".into()))?;

    Ok((sig, recid))
}

/// Recover the secp256k1 public key from a digest and signature.
pub fn recover_public_key(
    digest: &[u8; 32],
    signature: &[u8],
) -> Result<VerifyingKey, SignatureError> {
    let (sig, recid) = split_signature(signature)?;
    VerifyingKey::recover_from_prehash(digest, &sig, recid)
        .map_err(|e| SignatureError::RecoveryFailed(e.to_string()))
}

/// Recover the Ethereum address (20 bytes) of the signer.
pub fn recover_address(digest: &[u8; 32], signature: &[u8]) -> Result<[u8; 20], SignatureError> {
    let vk = recover_public_key(digest, signature)?;
    let encoded = vk.to_encoded_point(false);
    let mut hasher = Keccak256::new();
    hasher.update(&encoded.as_bytes()[1..]); // drop the 0x04 prefix
    let hash = hasher.finalize();
    let mut address = [0u8; 20];
    address.copy_from_slice(&hash[12..]);
    Ok(address)
}

/// Verify that a signature over `message` was produced by `expected_address`,
/// using the EIP-191 personal-message prefix.
pub fn verify_personal_message(
    message: &[u8],
    signature: &[u8],
    expected_address: &[u8; 20],
) -> Result<bool, SignatureError> {
    let digest = personal_message_hash(message);
    Ok(recover_address(&digest, signature)? == *expected_address)
}

#[cfg(test)]
mod tests {
    use super::*;
    use k256::ecdsa::signature::Signer;
    use k256::ecdsa::SigningKey;

    fn signing_key() -> SigningKey {
        // Deterministic key for reproducible tests.
        SigningKey::from_slice(&[0x42u8; 32]).expect("valid scalar")
    }

    #[test]
    fn test_message_hash_is_keccak_and_chain_scoped() {
        let a = create_message_hash(1, b"payload");
        let b = create_message_hash(56, b"payload");
        assert_ne!(a, b, "chain id must change the digest");
        assert_eq!(a.len(), 32);
    }

    /// Sign a prehash and return the 65-byte Ethereum-style signature.
    fn sign_recoverable(sk: &SigningKey, digest: &[u8; 32]) -> Vec<u8> {
        let (sig, recid) = sk.sign_prehash_recoverable(digest).expect("sign");
        let mut raw = sig.to_bytes().to_vec();
        raw.push(recid.to_byte() + 27);
        raw
    }

    #[test]
    fn test_sign_and_recover_roundtrip() {
        let sk = signing_key();
        let vk = sk.verifying_key();

        let digest = personal_message_hash(b"tigerswap");
        let raw = sign_recoverable(&sk, &digest);

        let recovered = recover_public_key(&digest, &raw).expect("recover");
        assert_eq!(recovered, *vk);

        let address = recover_address(&digest, &raw).expect("address");
        assert!(verify_personal_message(b"tigerswap", &raw, &address).unwrap());
        assert!(!verify_personal_message(b"tampered", &raw, &address).unwrap());
    }

    #[test]
    fn test_verify_signature_rejects_wrong_digest() {
        let sk = signing_key();
        let digest = personal_message_hash(b"right");
        let raw = sign_recoverable(&sk, &digest);
        let vk_bytes = sk.verifying_key().to_encoded_point(false);

        assert!(verify_signature(vk_bytes.as_bytes(), &digest, &raw[..64]).unwrap());
        assert!(verify_signature(vk_bytes.as_bytes(), &[0u8; 32], &raw[..64]).is_err());
    }

    #[test]
    fn test_typed_data_verification() {
        let sk = signing_key();
        let domain = [0x11u8; 32];
        let struct_hash = [0x22u8; 32];
        let digest = typed_data_hash(&domain, &struct_hash);
        let raw = sign_recoverable(&sk, &digest);

        let signer = recover_address(&digest, &raw).unwrap();
        assert!(verify_typed_data(&domain, &struct_hash, &raw, &signer).unwrap());

        let other = [0u8; 20];
        assert!(!verify_typed_data(&domain, &struct_hash, &raw, &other).unwrap());
    }

    #[test]
    fn test_invalid_lengths_rejected() {
        let digest = [0u8; 32];
        assert!(matches!(
            recover_address(&digest, &[0u8; 10]),
            Err(SignatureError::InvalidLength(_))
        ));
    }
}
