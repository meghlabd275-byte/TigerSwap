// ============================================================================
// TIGERSWAP RUST WALLET CORE - Complete Wallet Implementation
// BIP39, BIP32, BIP44, Seed Manager, Signer, MPC Wallet, MultiSig, Account Abstraction, Key Vault
// ============================================================================

pub mod bip39;
pub mod bip32;
pub mod bip44;
pub mod seed_manager;
pub mod signer;
pub mod mpc_wallet;
pub mod multisig;
pub mod account_abstraction;
pub mod key_vault;

// ============================================================================
// WALLET CORE LIBRARY
// ============================================================================

pub use bip39::{Mnemonic, MnemonicPhrase};
pub use bip32::{ExtendedPrivateKey, ExtendedPublicKey, DerivationPath};
pub use bip44::{BIP44Purpose, BIP44CoinType, BIP44Account, BIP44Change, BIP44Index};
pub use seed_manager::{SeedManager, SeedPhrase, BackupCode};
pub use signer::{TransactionSigner, MessageSigner, HardwareSigner};
pub use mpc_wallet::{MPCWallet, MPCKeyShare, MPCKeyGeneration, ThresholdSignature};
pub use multisig::{MultiSigWallet, MultiSigTransaction, SignatureThreshold};
pub use account_abstraction::{AccountAbstraction, UserOperation, EntryPoint, Aggregation};
pub use key_vault::{KeyVault, KeyRotation, HSMIntegration};

pub const WALLET_CORE_VERSION: &str = "1.0.0";