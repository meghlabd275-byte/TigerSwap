//! Crypto utilities for wallet core
//! Includes cryptographic primitives for key derivation, signing, and encryption

use k256::ecdsa::{SigningKey as EcdsaSigningKey, VerifyingKey};
use sha2::{Digest, Sha256};
use zeroize::Zeroize;

/// BIP39 mnemonic word list (simplified - first 20 words)
const BIP39_WORDLIST: &[&str] = &[
    "abandon", "ability", "able", "about", "above", "absent", "absorb", "abstract",
    "absurd", "abuse", "access", "accident", "account", "accuse", "achieve", "acid",
    "acoustic", "acquire", "across", "act",
];

/// Mnemonic phrase representation
pub struct Mnemonic {
    phrase: String,
    entropy: Vec<u8>,
}

impl Mnemonic {
    /// Create mnemonic from phrase
    pub fn from_phrase(phrase: &str) -> Result<Self, &'static str> {
        let words: Vec<&str> = phrase.split_whitespace().collect();
        
        if words.len() != 12 && words.len() != 24 {
            return Err("Invalid mnemonic length");
        }
        
        // Generate entropy from phrase (simplified)
        let mut hasher = Sha256::new();
        hasher.update(phrase.as_bytes());
        let entropy: Vec<u8> = hasher.finalize();
        
        Ok(Self {
            phrase: phrase.to_string(),
            entropy,
        })
    }
    
    /// Convert to seed with password
    pub fn to_seed(&self, password: &str) -> Vec<u8> {
        use pbkdf2::pbkdf2_hmac;
        
        let salt = format!("mnemonic{}", password);
        let mut seed = vec![0u8; 64];
        
        pbkdf2_hmac::<Sha256>(
            self.phrase.as_bytes(),
            salt.as_bytes(),
            2048,
            &mut seed,
        );
        
        seed
    }
}

/// Keyring for HD wallet key derivation
pub struct Keyring {
    master_key: Vec<u8>,
}

impl Keyring {
    pub fn new(master_key: Vec<u8>) -> Self {
        Self { master_key }
    }
    
    /// Derive key at path (BIP44)
    pub fn derive_path(&self, path: &str) -> Result<DerivedKey, &'static str> {
        // Simplified BIP44 derivation
        // In production, use proper BIP32 derivation
        
        let mut key = self.master_key.clone();
        let segments: Vec<&str> = path.split('/').collect();
        
        for (i, segment) in segments.iter().enumerate() {
            if i == 0 {
                // Skip "m"
                continue;
            }
            
            let mut hasher = Sha256::new();
            hasher.update(&key);
            hasher.update(segment.as_bytes());
            key = hasher.finalize().to_vec();
        }
        
        // For EVM chains, use the key directly
        let private_key = &key[..32];
        let address = Self::private_key_to_address(private_key);
        
        Ok(DerivedKey {
            chain_id: 1,
            address,
            private_key: private_key.to_vec(),
            public_key: key[32..].to_vec(),
        })
    }
    
    /// Convert private key to Ethereum address
    fn private_key_to_address(private_key: &[u8]) -> String {
        use k256::ecdsa::SigningKey;
        
        let signing_key = SigningKey::from_bytes(private_key.into())
            .expect("Invalid private key");
        
        let verifying_key = VerifyingKey::from(&signing_key);
        let address_bytes = &verifying_key.to_encoded_point(false).as_bytes()[1..];
        
        let hash = Sha256::digest(address_bytes);
        let address_hash = &hash[12..];
        
        format!("0x{}", hex::encode(address_hash))
    }
}

/// Derived key with address and public/private keys
pub struct DerivedKey {
    pub chain_id: u32,
    pub address: String,
    pub private_key: Vec<u8>,
    pub public_key: Vec<u8>,
}

impl DerivedKey {
    pub fn private_key_bytes(&self) -> Vec<u8> {
        self.private_key.clone()
    }
    
    pub fn public_key_bytes(&self) -> Vec<u8> {
        self.public_key.clone()
    }
}

/// Private key handling with secure memory
pub struct PrivateKey {
    bytes: [u8; 32],
}

impl PrivateKey {
    /// Create from bytes
    pub fn from_bytes(bytes: &[u8]) -> Result<Self, &'static str> {
        if bytes.len() != 32 {
            return Err("Invalid key length");
        }
        
        let mut key = Self { bytes: [0u8; 32] };
        key.bytes.copy_from_slice(bytes);
        
        Ok(key)
    }
    
    /// Get address
    pub fn to_address(&self) -> String {
        Keyring::private_key_to_address(&self.bytes)
    }
    
    /// Get private key bytes
    pub fn as_bytes(&self) -> &[u8; 32] {
        &self.bytes
    }
    
    /// Sign transaction (simplified)
    pub fn sign_transaction(&self, transaction: &mut Transaction) -> Result<Vec<u8>, &'static str> {
        use k256::ecdsa::{SigningKey, signature::Signer};
        
        let signing_key = SigningKey::from_bytes(self.bytes.into())
            .map_err(|_| "Invalid key")?;
        
        // Create signature
        let signer = SigningKey::unchecked(&signing_key);
        
        // Serialize transaction for signing
        let mut hasher = Sha256::new();
        hasher.update(serde_json::to_string(&transaction).unwrap().as_bytes());
        let message = hasher.finalize();
        
        let signature: k256::ecdsa::Signature = signer.sign(&message);
        
        Ok(signature.to_bytes().to_vec())
    }
    
    /// Sign message
    pub fn sign_message(&self, message: &[u8]) -> Vec<u8> {
        use k256::ecdsa::{SigningKey, signature::Signer};
        
        let signing_key = SigningKey::from_bytes(self.bytes.into())
            .expect("Invalid key");
        
        let signer = SigningKey::unchecked(&signing_key);
        let signature: k256::ecdsa::Signature = signer.sign(message);
        
        signature.to_bytes().to_vec()
    }
    
    /// Get public key bytes
    pub fn public_key_bytes(&self) -> Vec<u8> {
        use k256::ecdsa::SigningKey;
        
        let signing_key = SigningKey::from_bytes(self.bytes.into())
            .expect("Invalid key");
        let verifying_key = VerifyingKey::from(&signing_key);
        
        verifying_key.to_encoded_point(false).as_bytes().to_vec()
    }
}

impl Zeroize for PrivateKey {
    fn zeroize(&mut self) {
        self.bytes.zeroize();
    }
}

impl Drop for PrivateKey {
    fn drop(&mut self) {
        self.bytes.zeroize();
    }
}

/// Transaction structure for signing
#[derive(serde::Serialize, serde::Deserialize, Clone)]
pub struct Transaction {
    pub chain_id: u64,
    pub nonce: u64,
    pub to: String,
    pub value: String,
    pub data: String,
    pub gas_limit: String,
    pub gas_price: String,
    #[serde(skip)]
    pub signature: Option<Vec<u8>>,
}

impl Transaction {
    pub fn new() -> Self {
        Self {
            chain_id: 1,
            nonce: 0,
            to: String::new(),
            value: "0".to_string(),
            data: "0x".to_string(),
            gas_limit: "21000".to_string(),
            gas_price: "1000000000".to_string(),
            signature: None,
        }
    }
    
    /// Encode transaction for signing (EIP155)
    pub fn encode(&self) -> Vec<u8> {
        let mut encoded = Vec::new();
        
        // RLP encode transaction fields
        encoded.extend_from_slice(&self.chain_id.to_le_bytes());
        encoded.extend_from_slice(&self.nonce.to_le_bytes());
        encoded.extend_from_slice(self.gas_price.as_bytes());
        encoded.extend_from_slice(self.gas_limit.as_bytes());
        encoded.extend_from_slice(self.to.as_bytes());
        encoded.extend_from_slice(self.value.as_bytes());
        encoded.extend_from_slice(self.data.as_bytes());
        
        encoded
    }
}

impl Default for Transaction {
    fn default() -> Self {
        Self::new()
    }
}

/// Keystore for encrypted wallet storage
pub struct Keystore {
    accounts: HashMap<String, WalletAccountData>,
}

impl Keystore {
    pub fn new() -> Self {
        Self {
            accounts: HashMap::new(),
        }
    }
    
    pub async fn add_account(&mut self, id: String, account: WalletAccountData) -> Result<(), &'static str> {
        self.accounts.insert(id, account);
        Ok(())
    }
}

impl Default for Keystore {
    fn default() -> Self {
        Self::new()
    }
}

/// Wallet account data for storage
#[derive(serde::Serialize, serde::Deserialize, Clone)]
pub struct WalletAccountData {
    pub id: String,
    pub name: String,
    pub created_at: chrono::DateTime<chrono::Utc>,
    pub keys: HashMap<u32, DerivedKey>,
    pub encrypted: bool,
}
