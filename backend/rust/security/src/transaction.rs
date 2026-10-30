//! Transaction Module
//!
//! Transaction signing and verification for the DEX.
//!
//! The signing digest is a real RLP encoding of the transaction fields followed
//! by Keccak-256. TigerSwap appends a transaction-type tag as the final RLP
//! element so the signature commits to the declared type as well as the payload.

use crate::key_management::{KeyError, KeyManager};
use crate::signature;
use serde::{Deserialize, Serialize};
use sha3::{Digest, Keccak256};
use thiserror::Error;

#[derive(Error, Debug)]
pub enum TransactionError {
    #[error("Invalid transaction: {0}")]
    InvalidTransaction(String),
    #[error("Signing error: {0}")]
    SigningError(#[from] KeyError),
}

/// Transaction types
#[derive(Debug, Clone, Serialize, Deserialize)]
pub enum TransactionType {
    /// Basic token transfer
    Transfer {
        to: String,
        value: String,
        data: Option<String>,
    },
    /// Token swap
    Swap {
        path: Vec<String>,
        amount_in: String,
        amount_out_min: String,
    },
    /// Add liquidity
    AddLiquidity {
        token_a: String,
        token_b: String,
        amount_a_desired: String,
        amount_b_desired: String,
        amount_a_min: String,
        amount_b_min: String,
    },
    /// Remove liquidity
    RemoveLiquidity {
        token_a: String,
        token_b: String,
        liquidity: String,
        amount_a_min: String,
        amount_b_min: String,
    },
    /// Cross-chain bridge
    Bridge {
        to_chain: u64,
        token: String,
        amount: String,
        recipient: String,
    },
}

impl TransactionType {
    /// A stable discriminant that is committed to by the signature.
    pub fn tag(&self) -> u8 {
        match self {
            TransactionType::Transfer { .. } => 1,
            TransactionType::Swap { .. } => 2,
            TransactionType::AddLiquidity { .. } => 3,
            TransactionType::RemoveLiquidity { .. } => 4,
            TransactionType::Bridge { .. } => 5,
        }
    }
}

/// Unsigned transaction
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct UnsignedTransaction {
    pub chain_id: u64,
    pub nonce: u64,
    pub to: String,
    pub value: String,
    pub data: String,
    pub gas_limit: u64,
    pub gas_price: u64,
    pub transaction_type: TransactionType,
}

/// Signed transaction
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SignedTransaction {
    pub unsigned: UnsignedTransaction,
    pub signature: Vec<u8>,
    pub sender: String,
}

/// Encodes `data` as an RLP byte string.
fn rlp_bytes(data: &[u8]) -> Vec<u8> {
    // A single byte below 0x80 is its own encoding.
    if data.len() == 1 && data[0] < 0x80 {
        return vec![data[0]];
    }
    rlp_with_prefix(data, 0x80, 0xb7)
}

/// Encodes `items` as an RLP list of already-encoded elements.
fn rlp_list(items: &[Vec<u8>]) -> Vec<u8> {
    let mut payload = Vec::new();
    for item in items {
        payload.extend_from_slice(item);
    }
    rlp_with_prefix(&payload, 0xc0, 0xf7)
}

/// Applies the RLP length prefix: a short form for payloads of at most 55
/// bytes, otherwise a long form carrying the big-endian payload length.
fn rlp_with_prefix(data: &[u8], short_base: u8, long_base: u8) -> Vec<u8> {
    if data.len() <= 55 {
        let mut out = Vec::with_capacity(1 + data.len());
        out.push(short_base + data.len() as u8);
        out.extend_from_slice(data);
        out
    } else {
        let len_bytes = minimal_be_bytes(data.len() as u128);
        let mut out = Vec::with_capacity(1 + len_bytes.len() + data.len());
        out.push(long_base + len_bytes.len() as u8);
        out.extend_from_slice(&len_bytes);
        out.extend_from_slice(data);
        out
    }
}

/// Minimal big-endian encoding of a non-negative integer (empty for zero),
/// which is the form RLP uses for integers.
fn minimal_be_bytes(value: u128) -> Vec<u8> {
    let mut bytes = Vec::new();
    let mut v = value;
    while v > 0 {
        bytes.push((v & 0xff) as u8);
        v >>= 8;
    }
    bytes.reverse();
    bytes
}

/// Converts a base-10 integer string to its minimal big-endian byte form.
/// Rejects anything that is not a plain non-negative decimal.
fn decimal_to_be_bytes(decimal: &str) -> Result<Vec<u8>, TransactionError> {
    let trimmed = decimal.trim();
    if trimmed.is_empty() || !trimmed.chars().all(|c| c.is_ascii_digit()) {
        return Err(TransactionError::InvalidTransaction(format!(
            "value {decimal:?} is not a non-negative base-10 integer"
        )));
    }

    // Repeated division of the decimal digit vector by 256 yields base-256
    // digits least-significant first.
    let mut digits: Vec<u8> = trimmed.bytes().map(|b| b - b'0').collect();
    let mut out: Vec<u8> = Vec::new();

    while digits.iter().any(|&d| d != 0) {
        let mut remainder: u32 = 0;
        let mut quotient: Vec<u8> = Vec::with_capacity(digits.len());
        for &digit in &digits {
            let current = remainder * 10 + digit as u32;
            let q = current / 256;
            remainder = current % 256;
            if !quotient.is_empty() || q != 0 {
                quotient.push(q as u8);
            }
        }
        out.push(remainder as u8);
        digits = quotient;
    }

    out.reverse();
    Ok(out)
}

/// Decodes a `0x`-prefixed hex string. An empty string decodes to empty bytes.
fn hex_to_bytes(value: &str) -> Result<Vec<u8>, TransactionError> {
    let body = value.strip_prefix("0x").unwrap_or(value);
    if body.is_empty() {
        return Ok(Vec::new());
    }
    hex::decode(body)
        .map_err(|e| TransactionError::InvalidTransaction(format!("invalid hex: {e}")))
}

/// Decodes a 20-byte address. An empty string means contract creation.
fn decode_address(value: &str) -> Result<Vec<u8>, TransactionError> {
    let bytes = hex_to_bytes(value)?;
    if bytes.is_empty() {
        return Ok(bytes);
    }
    if bytes.len() != 20 {
        return Err(TransactionError::InvalidTransaction(format!(
            "address must be 20 bytes, got {}",
            bytes.len()
        )));
    }
    Ok(bytes)
}

impl UnsignedTransaction {
    /// The RLP signing payload:
    /// `[nonce, gasPrice, gasLimit, to, value, data, chainId, 0, 0, typeTag]`.
    pub fn encode_rlp(&self) -> Result<Vec<u8>, TransactionError> {
        let to = decode_address(&self.to)?;
        let data = hex_to_bytes(&self.data)?;
        let value = decimal_to_be_bytes(&self.value)?;

        Ok(rlp_list(&[
            rlp_bytes(&minimal_be_bytes(self.nonce as u128)),
            rlp_bytes(&minimal_be_bytes(self.gas_price as u128)),
            rlp_bytes(&minimal_be_bytes(self.gas_limit as u128)),
            rlp_bytes(&to),
            rlp_bytes(&value),
            rlp_bytes(&data),
            rlp_bytes(&minimal_be_bytes(self.chain_id as u128)),
            rlp_bytes(&[]),
            rlp_bytes(&[]),
            rlp_bytes(&[self.transaction_type.tag()]),
        ]))
    }

    /// Keccak-256 of the RLP signing payload, i.e. the digest to sign.
    pub fn hash(&self) -> Result<[u8; 32], TransactionError> {
        let encoded = self.encode_rlp()?;
        let mut hasher = Keccak256::new();
        hasher.update(&encoded);
        Ok(hasher.finalize().into())
    }
}

impl SignedTransaction {
    /// Sign an unsigned transaction with the key registered for `sender`.
    pub async fn sign(
        unsigned: UnsignedTransaction,
        key_manager: &KeyManager,
        sender: &str,
    ) -> Result<Self, TransactionError> {
        let hash = unsigned.hash()?;
        let signature = key_manager
            .sign(sender, &hash)
            .await
            .map_err(TransactionError::SigningError)?;

        Ok(Self {
            unsigned,
            signature,
            sender: sender.to_string(),
        })
    }

    /// Verify that the signature was produced by `self.sender` over this
    /// transaction's own digest. Any signature that does not recover to the
    /// stated sender is rejected.
    pub fn verify(&self) -> bool {
        let digest = match self.unsigned.hash() {
            Ok(d) => d,
            Err(_) => return false,
        };

        match signature::recover_address(&digest, &self.signature) {
            Ok(recovered) => {
                format!("0x{}", hex::encode(recovered)).eq_ignore_ascii_case(&self.sender)
            }
            Err(_) => false,
        }
    }
}

/// Builder for transactions
pub struct TransactionBuilder {
    chain_id: u64,
    nonce: u64,
    to: String,
    value: String,
    data: String,
    gas_limit: u64,
    gas_price: u64,
}

impl TransactionBuilder {
    pub fn new(chain_id: u64) -> Self {
        Self {
            chain_id,
            nonce: 0,
            to: String::new(),
            value: "0".to_string(),
            data: String::new(),
            gas_limit: 21000,
            gas_price: 0,
        }
    }

    pub fn to(mut self, to: impl Into<String>) -> Self {
        self.to = to.into();
        self
    }

    pub fn value(mut self, value: impl Into<String>) -> Self {
        self.value = value.into();
        self
    }

    pub fn data(mut self, data: impl Into<String>) -> Self {
        self.data = data.into();
        self
    }

    pub fn gas_limit(mut self, limit: u64) -> Self {
        self.gas_limit = limit;
        self
    }

    pub fn gas_price(mut self, price: u64) -> Self {
        self.gas_price = price;
        self
    }

    pub fn nonce(mut self, nonce: u64) -> Self {
        self.nonce = nonce;
        self
    }

    pub fn build(self, transaction_type: TransactionType) -> UnsignedTransaction {
        UnsignedTransaction {
            chain_id: self.chain_id,
            nonce: self.nonce,
            to: self.to,
            value: self.value,
            data: self.data,
            gas_limit: self.gas_limit,
            gas_price: self.gas_price,
            transaction_type,
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn sample_tx() -> UnsignedTransaction {
        TransactionBuilder::new(1)
            .nonce(7)
            .to("0x742d35Cc6634C0532925a3b844Bc9e7595f0fEb1")
            .value("1000000000000000000")
            .gas_limit(21000)
            .gas_price(1000000000)
            .build(TransactionType::Transfer {
                to: "0x742d35Cc6634C0532925a3b844Bc9e7595f0fEb1".to_string(),
                value: "1000000000000000000".to_string(),
                data: None,
            })
    }

    #[test]
    fn test_transaction_builder() {
        let tx = sample_tx();
        assert_eq!(tx.chain_id, 1);
        assert_eq!(tx.gas_limit, 21000);
        assert_eq!(tx.nonce, 7);
    }

    /// RLP: a single small byte is itself, a 55-byte string is short form, and a
    /// 56-byte string switches to the long form.
    #[test]
    fn test_rlp_encoding_rules() {
        assert_eq!(rlp_bytes(&[0x7f]), vec![0x7f]);
        assert_eq!(rlp_bytes(&[0x80]), vec![0x81, 0x80]);
        assert_eq!(rlp_bytes(&[]), vec![0x80]);

        let short = vec![0xaa; 55];
        assert_eq!(rlp_bytes(&short)[0], 0x80 + 55);

        let long = vec![0xaa; 56];
        let encoded = rlp_bytes(&long);
        assert_eq!(encoded[0], 0xb8);
        assert_eq!(encoded[1], 56);
        assert_eq!(encoded.len(), 2 + 56);
    }

    #[test]
    fn test_decimal_to_be_bytes() {
        assert_eq!(decimal_to_be_bytes("0").unwrap(), Vec::<u8>::new());
        assert_eq!(decimal_to_be_bytes("1").unwrap(), vec![1]);
        assert_eq!(decimal_to_be_bytes("255").unwrap(), vec![255]);
        assert_eq!(decimal_to_be_bytes("256").unwrap(), vec![1, 0]);
        assert_eq!(
            decimal_to_be_bytes("1000000000000000000").unwrap(),
            vec![0x0d, 0xe0, 0xb6, 0xb3, 0xa7, 0x64, 0x00, 0x00]
        );
        assert!(decimal_to_be_bytes("12x").is_err());
        assert!(decimal_to_be_bytes("-1").is_err());
        assert!(decimal_to_be_bytes("").is_err());
    }

    /// The digest must change with the chain id, the nonce, the value and the
    /// transaction type, and must be deterministic.
    #[test]
    fn test_signing_hash_commits_to_every_field() {
        let digest = sample_tx().hash().unwrap();

        let mut other_chain = sample_tx();
        other_chain.chain_id = 56;
        assert_ne!(digest, other_chain.hash().unwrap());

        let mut other_nonce = sample_tx();
        other_nonce.nonce = 8;
        assert_ne!(digest, other_nonce.hash().unwrap());

        let mut other_value = sample_tx();
        other_value.value = "2".to_string();
        assert_ne!(digest, other_value.hash().unwrap());

        let mut other_type = sample_tx();
        other_type.transaction_type = TransactionType::Bridge {
            to_chain: 1,
            token: "TGR".to_string(),
            amount: "1".to_string(),
            recipient: "0x0".to_string(),
        };
        assert_ne!(digest, other_type.hash().unwrap());

        assert_eq!(digest, sample_tx().hash().unwrap());
    }

    /// A malformed transaction must produce an error rather than hashing
    /// nonsense.
    #[test]
    fn test_hash_rejects_malformed_fields() {
        let mut bad_value = sample_tx();
        bad_value.value = "1.5".to_string();
        assert!(bad_value.hash().is_err());

        let mut bad_address = sample_tx();
        bad_address.to = "0x1234".to_string();
        assert!(bad_address.hash().is_err());

        let mut bad_data = sample_tx();
        bad_data.data = "0xzz".to_string();
        assert!(bad_data.hash().is_err());
    }

    /// Signing then verifying must succeed, and any tampering must fail. The old
    /// implementation accepted any non-empty signature.
    #[tokio::test]
    async fn test_sign_and_verify_roundtrip() {
        let manager = KeyManager::new();
        let (address, _) = manager.generate_eth_key().await.unwrap();

        let signed = SignedTransaction::sign(sample_tx(), &manager, &address)
            .await
            .unwrap();
        assert!(signed.verify(), "signature must verify for the real signer");

        let mut wrong_sender = signed.clone();
        wrong_sender.sender = "0x0000000000000000000000000000000000000000".to_string();
        assert!(!wrong_sender.verify(), "wrong sender must be rejected");

        let mut empty_sig = signed.clone();
        empty_sig.signature.clear();
        assert!(!empty_sig.verify(), "empty signature must be rejected");

        let mut junk_sig = signed.clone();
        junk_sig.signature = vec![0u8; 65];
        assert!(!junk_sig.verify(), "junk signature must be rejected");

        let mut tampered = signed.clone();
        tampered.unsigned.value = "9999999999999999999".to_string();
        assert!(!tampered.verify(), "tampered payload must be rejected");
    }
}
