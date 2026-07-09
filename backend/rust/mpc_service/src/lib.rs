//! TigerWallet MPC (Multi-Party Computation) Service
//! 
//! This module provides MPC-based key management for TigerWallet,
//! enabling social login, key sharding, and distributed key generation.

pub mod key_gen;
pub mod key_share;
pub mod threshold;
pub mod coordinator;
pub mod types;
pub mod error;

pub use error::MpcError;
pub use types::*;
