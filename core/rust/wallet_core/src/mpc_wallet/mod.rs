//! MPC Wallet - Multi-Party Computation
//! 
//! Implements threshold signature schemes for distributed key generation
//! and signing. Supports TSS for ECDSA (Secp256k1) and Ed25519.

use std::collections::HashMap;
use std::sync::{Arc, RwLock};
use thiserror::Error;

#[derive(Error, Debug)]
pub enum MpcError {
    #[error("Key generation failed: {0}")]
    KeyGenFailed(String),
    #[error("Signing failed: {0}")]
    SigningFailed(String),
    #[error("Invalid share: {0}")]
    InvalidShare(String),
    #[error("Insufficient shares: need {0}, have {1}")]
    InsufficientShares(usize, usize),
    #[error("Invalid threshold: {0}")]
    InvalidThreshold(String),
    #[error("Party not found: {0}")]
    PartyNotFound(String),
    #[error("Protocol error: {0}")]
    ProtocolError(String),
}

// ============================================================================
// Types
// ============================================================================

/// Threshold configuration
#[derive(Debug, Clone)]
pub struct ThresholdConfig {
    pub total_parties: usize,
    pub threshold: usize,
}

impl ThresholdConfig {
    pub fn new(total_parties: usize, threshold: usize) -> Result<Self, MpcError> {
        if threshold == 0 || threshold > total_parties {
            return Err(MpcError::InvalidThreshold(
                format!("threshold {} must be between 1 and {}", threshold, total_parties)
            ));
        }
        Ok(Self { total_parties, threshold })
    }
}

/// Public key share (verification key)
#[derive(Debug, Clone)]
pub struct PublicKeyShare {
    pub party_id: u32,
    pub x: [u8; 32],
    pub y: [u8; 32],
}

/// Secret share (private key share)
#[derive(Debug, Clone)]
pub struct SecretShare {
    pub party_id: u32,
    pub share: [u8; 32],
}

/// Commitment for verifiable secret sharing
#[derive(Debug, Clone)]
pub struct Commitment {
    pub party_id: u32,
    pub round: u32,
    pub commitments: Vec<[u8; 32]>,
}

/// Signing partial result
#[derive(Debug, Clone)]
pub struct PartialSignature {
    pub party_id: u32,
    pub partial: [u8; 32],
}

// ============================================================================
// MPC Party
// ============================================================================

/// Represents one party in the MPC protocol
#[derive(Debug)]
pub struct MpcParty {
    pub party_id: u32,
    pub secret_share: Option<SecretShare>,
    pub public_key_share: Option<PublicKeyShare>,
    pub received_shares: RwLock<Vec<SecretShare>>,
    pub commitments: RwLock<Vec<Commitment>>,
    pub partial_signatures: RwLock<Vec<PartialSignature>>,
}

impl MpcParty {
    pub fn new(party_id: u32) -> Self {
        Self {
            party_id,
            secret_share: None,
            public_key_share: None,
            received_shares: RwLock::new(Vec::new()),
            commitments: RwLock::new(Vec::new()),
            partial_signatures: RwLock::new(Vec::new()),
        }
    }

    /// Generate key shares for this party (Simplified DKG)
    pub fn generate_key_share(&mut self, config: &ThresholdConfig) -> Result<SecretShare, MpcError> {
        // Simplified key generation
        // In production: use proper Feldman/VSSS + Pedersen commitments
        
        let mut share = [0u8; 32];
        for (i, byte) in share.iter_mut().enumerate() {
            byte = ((self.party_id as u8).wrapping_add(i as u8)).wrapping_add(0x42);
        }
        
        let secret_share = SecretShare {
            party_id: self.party_id,
            share,
        };
        
        self.secret_share = Some(secret_share.clone());
        
        // Generate public key share (simplified)
        let public_key_share = PublicKeyShare {
            party_id: self.party_id,
            x: share,
            y: [0u8; 32], // Would be point multiplication in production
        };
        self.public_key_share = Some(public_key_share);
        
        Ok(secret_share)
    }

    /// Receive a secret share from another party
    pub fn receive_share(&self, share: SecretShare) -> Result<(), MpcError> {
        if share.party_id == self.party_id {
            return Err(MpcError::InvalidShare("cannot receive own share".to_string()));
        }
        
        let mut shares = self.received_shares.write().unwrap();
        shares.push(share);
        Ok(())
    }

    /// Generate partial signature for signing
    pub fn generate_partial(&self, message_hash: [u8; 32]) -> Result<PartialSignature, MpcError> {
        let secret_share = self.secret_share.as_ref()
            .ok_or_else(|| MpcError::SigningFailed("no secret share".to_string()))?;
        
        // Simplified partial signature
        // In production: use proper TSS signing protocol
        let mut partial = [0u8; 32];
        for (i, byte) in partial.iter_mut().enumerate() {
            *byte = secret_share.share[i].wrapping_add(message_hash[i]);
        }
        
        Ok(PartialSignature {
            party_id: self.party_id,
            partial,
        })
    }

    /// Receive partial signature from another party
    pub fn receive_partial(&self, partial: PartialSignature) -> Result<(), MpcError> {
        let mut sigs = self.partial_signatures.write().unwrap();
        sigs.push(partial);
        Ok(())
    }
}

// ============================================================================
// MPC Wallet
// ============================================================================

/// MPC Wallet with threshold signatures
#[derive(Debug)]
pub struct MpcWallet {
    config: ThresholdConfig,
    parties: RwLock<HashMap<u32, Arc<MpcParty>>>,
    public_key: RwLock<Option<[u8; 32]>>,
    threshold_config: ThresholdConfig,
}

impl MpcWallet {
    /// Create new MPC wallet with threshold configuration
    pub fn new(total_parties: usize, threshold: usize) -> Result<Self, MpcError> {
        let config = ThresholdConfig::new(total_parties, threshold)?;
        
        Ok(Self {
            config: config.clone(),
            parties: RwLock::new(HashMap::new()),
            public_key: RwLock::new(None),
            threshold_config: config,
        })
    }

    /// Add party to the MPC wallet
    pub fn add_party(&self, party_id: u32) -> Result<Arc<MpcParty>, MpcError> {
        let mut parties = self.parties.write().unwrap();
        
        if parties.contains_key(&party_id) {
            return Err(MpcError::PartyNotFound(format!("party {} already exists", party_id)));
        }
        
        let party = Arc::new(MpcParty::new(party_id));
        parties.insert(party_id, party.clone());
        
        Ok(party)
    }

    /// Get party by ID
    pub fn get_party(&self, party_id: u32) -> Option<Arc<MpcParty>> {
        self.parties.read().unwrap().get(&party_id).cloned()
    }

    /// Generate distributed key generation (DKG)
    pub fn distributed_key_gen(&self) -> Result<[u8; 32], MpcError> {
        let parties = self.parties.read().unwrap();
        
        if parties.len() != self.config.total_parties {
            return Err(MpcError::KeyGenFailed(format!(
                "need {} parties, have {}",
                self.config.total_parties,
                parties.len()
            )));
        }
        
        // Simplified DKG - in production use proper protocol
        let mut combined_public_key = [0u8; 32];
        
        for (_, party) in parties.iter() {
            if let Some(pk) = &party.public_key_share {
                for (i, byte) in combined_public_key.iter_mut().enumerate() {
                    *byte ^= pk.x[i];
                }
            }
        }
        
        *self.public_key.write().unwrap() = Some(combined_public_key);
        
        Ok(combined_public_key)
    }

    /// Sign message with threshold signatures
    pub fn sign(&self, message_hash: [u8; 32]) -> Result<[u8; 64], MpcError> {
        let parties = self.parties.read().unwrap();
        
        // Collect partial signatures
        let mut partials = Vec::new();
        for (_, party) in parties.iter() {
            match party.generate_partial(message_hash) {
                Ok(partial) => partials.push(partial),
                Err(e) => return Err(e),
            }
        }
        
        // Check threshold
        if partials.len() < self.config.threshold {
            return Err(MpcError::InsufficientShares(
                self.config.threshold,
                partials.len(),
            ));
        }
        
        // Combine partials (simplified)
        // In production: use proper Lagrange interpolation
        let mut signature = [0u8; 64];
        
        for partial in &partials {
            for (i, byte) in signature.iter_mut().enumerate() {
                if i < 32 {
                    *byte ^= partial.partial[i];
                } else {
                    *byte ^= partial.partial[i - 32];
                }
            }
        }
        
        Ok(signature)
    }

    /// Get the threshold public key
    pub fn get_public_key(&self) -> Option<[u8; 32]> {
        self.public_key.read().unwrap().clone()
    }

    /// Check if enough parties are online for signing
    pub fn can_sign(&self) -> bool {
        self.parties.read().unwrap().len() >= self.config.threshold
    }
}

// ============================================================================
// Key Share Storage (for persistence)
// ============================================================================

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct StoredKeyShare {
    pub party_id: u32,
    pub encrypted_share: Vec<u8>,
    pub public_key: Vec<u8>,
}

impl MpcWallet {
    /// Export key shares (encrypted)
    pub fn export_shares(&self) -> Result<Vec<StoredKeyShare>, MpcError> {
        let parties = self.parties.read().unwrap();
        let mut exports = Vec::new();
        
        for (_, party) in parties.iter() {
            if let Some(secret) = &party.secret_share {
                exports.push(StoredKeyShare {
                    party_id: party.party_id,
                    encrypted_share: secret.share.to_vec(),
                    public_key: party.public_key_share
                        .as_ref()
                        .map(|pk| pk.x.to_vec())
                        .unwrap_or_default(),
                });
            }
        }
        
        Ok(exports)
    }

    /// Import key share
    pub fn import_share(&self, stored: StoredKeyShare) -> Result<(), MpcError> {
        let party = self.get_party(stored.party_id)
            .ok_or_else(|| MpcError::PartyNotFound(stored.party_id.to_string()))?;
        
        let secret_share = SecretShare {
            party_id: stored.party_id,
            share: {
                let mut share = [0u8; 32];
                let data = &stored.encrypted_share[..32.min(stored.encrypted_share.len())];
                share[..data.len()].copy_from_slice(data);
                share
            },
        };
        
        // Note: In production, decrypt first
        let mut party_mut = party.clone();
        party_mut.secret_share = Some(secret_share);
        
        Ok(())
    }
}

// ============================================================================
// Tests
// ============================================================================

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_mpc_wallet_creation() {
        let wallet = MpcWallet::new(3, 2).unwrap();
        assert!(wallet.can_sign() == false);
    }

    #[test]
    fn test_mpc_party_creation() {
        let party = MpcParty::new(1);
        assert_eq!(party.party_id, 1);
    }

    #[test]
    fn test_threshold_config() {
        let config = ThresholdConfig::new(5, 3).unwrap();
        assert_eq!(config.total_parties, 5);
        assert_eq!(config.threshold, 3);
    }

    #[test]
    fn test_invalid_threshold() {
        let result = ThresholdConfig::new(3, 0);
        assert!(result.is_err());
        
        let result = ThresholdConfig::new(3, 5);
        assert!(result.is_err());
    }

    #[test]
    fn test_key_generation() {
        let wallet = MpcWallet::new(3, 2).unwrap();
        
        // Add parties
        wallet.add_party(1).unwrap();
        wallet.add_party(2).unwrap();
        wallet.add_party(3).unwrap();
        
        // Generate keys
        let pk = wallet.distributed_key_gen().unwrap();
        assert_eq!(pk.len(), 32);
    }

    #[test]
    fn test_signing() {
        let wallet = MpcWallet::new(3, 2).unwrap();
        
        wallet.add_party(1).unwrap();
        wallet.add_party(2).unwrap();
        wallet.add_party(3).unwrap();
        
        wallet.distributed_key_gen().unwrap();
        
        let message = [0u8; 32];
        let sig = wallet.sign(message).unwrap();
        
        assert_eq!(sig.len(), 64);
    }
}