//! Wallet Module - HD Wallet implementation

use crate::{WalletError, Mnemonic};
use secp256k1::{PublicKey, Secp256k1, SecretKey, Signing, Message};
use sha2::{Digest, Sha256, Sha512};
use serde::{Deserialize, Serialize};

/// Supported chains
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
pub enum Chain {
    Ethereum,
    BinanceSmartChain,
    Polygon,
    Avalanche,
    Arbitrum,
    Optimism,
    Solana,
    Bitcoin,
    Base,
    ArbitrumNova,
}

impl Chain {
    pub fn chain_id(&self) -> u64 {
        match self {
            Chain::Ethereum => 1,
            Chain::BinanceSmartChain => 56,
            Chain::Polygon => 137,
            Chain::Avalanche => 43114,
            Chain::Arbitrum => 42161,
            Chain::Optimism => 10,
            Chain::Solana => 0,
            Chain::Bitcoin => 0,
            Chain::Base => 8453,
            Chain::ArbitrumNova => 42170,
        }
    }
    
    pub fn coin_type(&self) -> u32 {
        match self {
            Chain::Ethereum | Chain::BinanceSmartChain | Chain::Polygon 
            | Chain::Avalanche | Chain::Arbitrum | Chain::Optimism | Chain::Base | Chain::ArbitrumNova => 60,
            Chain::Solana => 501,
            Chain::Bitcoin => 0,
        }
    }
    
    pub fn from_str(s: &str) -> Self {
        match s.to_lowercase().as_str() {
            "ethereum" | "eth" => Chain::Ethereum,
            "bsc" | "binancesmartchain" => Chain::BinanceSmartChain,
            "polygon" | "matic" => Chain::Polygon,
            "avalanche" | "avax" => Chain::Avalanche,
            "arbitrum" | "arb" => Chain::Arbitrum,
            "optimism" | "op" => Chain::Optimism,
            "solana" | "sol" => Chain::Solana,
            "bitcoin" | "btc" => Chain::Bitcoin,
            "base" => Chain::Base,
            "arbitrumnova" | "nova" => Chain::ArbitrumNova,
            _ => Chain::Ethereum,
        }
    }
}

impl Default for Chain {
    fn default() -> Self {
        Chain::Ethereum
    }
}

/// HD Wallet
#[derive(Debug, Clone)]
pub struct HDWallet {
    mnemonic: Mnemonic,
    seed: [u8; 64],
    chain: Chain,
}

impl HDWallet {
    /// Generate new wallet for chain
    pub fn generate(chain: Chain) -> Self {
        let mnemonic = Mnemonic::generate(256).unwrap();
        let seed = mnemonic.to_seed(None);
        Self { mnemonic, seed, chain }
    }
    
    /// Create from mnemonic
    pub fn from_mnemonic(mnemonic: Mnemonic, chain: Chain) -> Self {
        let seed = mnemonic.to_seed(None);
        Self { mnemonic, seed, chain }
    }
    
    /// Create from seed
    pub fn from_seed(seed: [u8; 64], chain: Chain) -> Self {
        let entropy = seed[..32].to_vec();
        let mnemonic = Mnemonic::from_entropy(entropy).unwrap_or_else(|_| {
            Mnemonic { words: vec![], entropy }
        });
        Self { mnemonic, seed, chain }
    }
    
    /// Get default address
    pub fn default_address(&self) -> Result<String, WalletError> {
        self.derive_address(0, 0, 0)
    }
    
    /// Derive address from BIP44 path
    pub fn derive_address(&self, _purpose: u32, _coin: u32, account: u32) -> Result<String, WalletError> {
        let path = format!("m/44'/{}'/{}'/0/0", self.chain.coin_type(), account);
        let private_key = self.derive_private_key(&path)?;
        
        let public_key = self.private_to_public(&private_key)?;
        let address = self.public_to_address(&public_key)?;
        
        Ok(address)
    }
    
    /// Derive private key from path
    fn derive_private_key(&self, path: &str) -> Result<[u8; 32], WalletError> {
        let mut key = self.seed;
        
        for component in path.split('/') {
            if component.starts_with('m') {
                continue;
            }
            
            let hardened = component.ends_with('\'');
            let index: u32 = component.trim_matches('\'').parse().unwrap_or(0);
            let derivation = if hardened { index + 0x80000000 } else { index };
            
            // HMAC-SHA512
            let mut hasher = Sha512::new();
            hasher.update(&key);
            hasher.update(&derivation.to_le_bytes());
            let result = hasher.finalize();
            
            key.copy_from_slice(&result[..32]);
        }
        
        Ok(key)
    }
    
    /// Private key to public key
    fn private_to_public(&self, private_key: &[u8; 32]) -> Result<[u8; 33], WalletError> {
        let secp = Secp256k1::new();
        let secret_key = SecretKey::from_slice(private_key)
            .map_err(|e| WalletError::DerivationFailed(e.to_string()))?;
        
        let public_key = PublicKey::from_secret_key(&secp, &secret_key);
        Ok(public_key.serialize())
    }
    
    /// Public key to address
    fn public_to_address(&self, public_key: &[u8; 33]) -> Result<String, WalletError> {
        let hash = Sha256::digest(public_key);
        let address = &hash[12..];
        Ok(format!("0x{}", hex::encode(address)))
    }
    
    /// Sign message
    pub fn sign(&self, message: &[u8]) -> Result<[u8; 65], WalletError> {
        let path = format!("m/44'/{}'/0'/0/0", self.chain.coin_type());
        let private_key = self.derive_private_key(&path)?;
        
        let secp = Secp256k1::new();
        let secret_key = SecretKey::from_slice(&private_key)
            .map_err(|e| WalletError::SigningFailed(e.to_string()))?;
        
        let message_hash = Sha256::digest(message);
        let message = Message::from_slice(&message_hash)
            .map_err(|e| WalletError::SigningFailed(e.to_string()))?;
        
        let signature = secp.sign_ecdsa(&message, &secret_key);
        
        let mut sig_bytes = [0u8; 65];
        sig_bytes[..64].copy_from_slice(&signature.serialize_compact());
        
        Ok(sig_bytes)
    }
    
    /// Sign transaction
    pub fn sign_transaction(&self, tx: &[u8]) -> Result<String, WalletError> {
        let sig = self.sign(tx)?;
        Ok(hex::encode(sig))
    }
    
    /// Get mnemonic phrase
    pub fn mnemonic_phrase(&self) -> String {
        self.mnemonic.phrase()
    }
    
    /// Get chain
    pub fn chain(&self) -> Chain {
        self.chain
    }
}

impl Default for HDWallet {
    fn default() -> Self {
        Self::generate(Chain::Ethereum)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_chain_ids() {
        assert_eq!(Chain::Ethereum.chain_id(), 1);
        assert_eq!(Chain::Polygon.chain_id(), 137);
        assert_eq!(Chain::Arbitrum.chain_id(), 42161);
    }
    
    #[test]
    fn test_wallet_generation() {
        let wallet = HDWallet::generate(Chain::Ethereum);
        let address = wallet.default_address().unwrap();
        assert!(address.starts_with("0x"));
    }
    
    #[test]
    fn test_bip44_derivation() {
        let wallet = HDWallet::generate(Chain::Ethereum);
        
        let addr1 = wallet.derive_address(44, 0, 0).unwrap();
        let addr2 = wallet.derive_address(44, 0, 1).unwrap();
        
        assert_ne!(addr1, addr2);
    }
    
    #[test]
    fn test_signing() {
        let wallet = HDWallet::generate(Chain::Ethereum);
        let signature = wallet.sign(b"test message").unwrap();
        
        assert_eq!(signature.len(), 65);
    }
}