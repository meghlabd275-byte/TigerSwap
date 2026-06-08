//! TigerSwap Rust SDK - Complete DEX Functionality
//! 
//! Full SDK for trading, routing, limit orders, DCA, and more

#![deny(unsafe_code)]
#![warn(missing_docs)]

pub mod client;
pub mod errors;
pub mod models;
pub mod trading;
pub mod routing;
pub mod orders;
pub mod dca;
pub mod wallet;

pub use client::TigerSwapClient;
pub use errors::{TigerSwapError, Result};
pub use models::*;

/// TigerSwap SDK version
pub const VERSION: &str = env!("CARGO_PKG_VERSION");

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_version() {
        assert!(!VERSION.is_empty());
    }
}