//! Validation Module
//! 
//! Input validation and sanitization for the DEX

use thiserror::Error;

#[derive(Error, Debug)]
pub enum ValidationError {
    #[error("Invalid address: {0}")]
    InvalidAddress(String),
    #[error("Invalid amount: {0}")]
    InvalidAmount(String),
    #[error("Invalid token: {0}")]
    InvalidToken(String),
    #[error("Invalid chain: {0}")]
    InvalidChain(String),
    #[error("Value out of range: {0}")]
    OutOfRange(String),
    #[error("Required field missing: {0}")]
    MissingField(String),
}

/// Validate an Ethereum-style address
pub fn validate_address(address: &str) -> Result<(), ValidationError> {
    if !address.starts_with("0x") {
        return Err(ValidationError::InvalidAddress(
            "Address must start with 0x".to_string()
        ));
    }
    
    if address.len() != 42 {
        return Err(ValidationError::InvalidAddress(
            "Address must be 42 characters".to_string()
        ));
    }
    
    // Check if it's valid hex
    let hex_part = &address[2..];
    if !hex_part.chars().all(|c| c.is_ascii_hexdigit()) {
        return Err(ValidationError::InvalidAddress(
            "Address contains invalid characters".to_string()
        ));
    }
    
    Ok(())
}

/// Validate a token amount.
///
/// The string must be a plain base-10 decimal: a run of digits, an optional
/// single fractional part, no sign, no exponent and no surrounding whitespace.
pub fn validate_amount(amount: &str, decimals: u8) -> Result<(), ValidationError> {
    if amount.is_empty() {
        return Err(ValidationError::InvalidAmount("Amount is required".to_string()));
    }
    if amount.trim() != amount {
        return Err(ValidationError::InvalidAmount(
            "Amount must not contain whitespace".to_string(),
        ));
    }

    let mut parts = amount.split('.');
    let integer_part = parts.next().unwrap_or_default();
    let fractional_part = parts.next();
    if parts.next().is_some() {
        return Err(ValidationError::InvalidAmount("Invalid number format".to_string()));
    }

    // Reject a bare "." and any empty integer part such as ".5".
    if integer_part.is_empty() || !integer_part.chars().all(|c| c.is_ascii_digit()) {
        return Err(ValidationError::InvalidAmount(
            "Amount must be a base-10 decimal".to_string(),
        ));
    }

    if let Some(frac) = fractional_part {
        if frac.is_empty() || !frac.chars().all(|c| c.is_ascii_digit()) {
            return Err(ValidationError::InvalidAmount(
                "Amount must be a base-10 decimal".to_string(),
            ));
        }
        if frac.len() > decimals as usize {
            return Err(ValidationError::InvalidAmount(format!(
                "Too many decimals (max {decimals})"
            )));
        }
    }

    // A u256 holds 78 decimal digits. `decimals` is caller-supplied, so guard it
    // before subtracting: `78 - decimals` underflows and panics for decimals > 78.
    if decimals > 78 || integer_part.len() > 78 - decimals as usize {
        return Err(ValidationError::InvalidAmount("Amount too large".to_string()));
    }

    Ok(())
}

/// Validate a chain ID
pub fn validate_chain_id(chain_id: u64) -> Result<(), ValidationError> {
    // Common chain IDs
    let valid_chains = [
        1,    // Ethereum
        5,    // Goerli
        11155111, // Sepolia
        56,   // BSC
        97,   // BSC Testnet
        137,  // Polygon
        80001, // Mumbai
        42161, // Arbitrum
        421613, // Arbitrum Goerli
        10,   // Optimism
        420,  // Optimism Goerli
        8453, // Base
        84531, // Base Goerli
        43114, // Avalanche
        43113, // Avalanche Fuji
        250,  // Fantom
        4002, // Fantom Testnet
        101,  // Solana
        102,  // Solana Devnet
    ];
    
    if !valid_chains.contains(&chain_id) {
        // Allow unknown chains but warn
        log::warn!("Unknown chain ID: {}", chain_id);
    }
    
    Ok(())
}

/// Sanitize a string input
pub fn sanitize_string(input: &str, max_length: usize) -> String {
    let mut sanitized = String::with_capacity(input.len());
    
    for c in input.chars() {
        // Allow alphanumeric, space, hyphen, underscore
        if c.is_alphanumeric() || c == ' ' || c == '-' || c == '_' || c == '.' {
            sanitized.push(c);
        }
    }
    
    // Trim, then truncate without splitting a multi-byte character. Slicing by
    // byte offset would panic when `max_length` lands inside a code point.
    let trimmed = sanitized.trim();
    let truncated: String = trimmed.chars().take(max_length).collect();
    truncated
}

/// Validate token symbols
pub fn validate_token_symbol(symbol: &str) -> Result<(), ValidationError> {
    if symbol.is_empty() {
        return Err(ValidationError::InvalidToken(
            "Symbol cannot be empty".to_string()
        ));
    }
    
    if symbol.len() > 11 {
        return Err(ValidationError::InvalidToken(
            "Symbol too long (max 11 chars)".to_string()
        ));
    }
    
    if !symbol.chars().all(|c| c.is_ascii_uppercase() || c.is_ascii_digit()) {
        return Err(ValidationError::InvalidToken(
            "Symbol must be uppercase letters and numbers only".to_string()
        ));
    }
    
    Ok(())
}

/// Validate a transaction request
pub fn validate_swap_request(
    from_token: &str,
    to_token: &str,
    amount: &str,
) -> Result<(), ValidationError> {
    // Tokens are identified by symbol, so validate them as symbols. Requiring
    // addresses here rejected every legitimate request.
    validate_token_symbol(from_token)?;
    validate_token_symbol(to_token)?;
    validate_amount(amount, 18)?;
    
    if from_token == to_token {
        return Err(ValidationError::InvalidToken(
            "Cannot swap same token".to_string()
        ));
    }
    
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_validate_address() {
        assert!(validate_address("0x742d35Cc6634C0532925a3b844Bc9e7595f0fEb1").is_ok());
        assert!(validate_address("0x742d35Cc6634C0532925a3b844Bc9e7595f0fEb").is_err());
        assert!(validate_address("742d35Cc6634C0532925a3b844Bc9e7595f0fEb1").is_err());
    }
    
    #[test]
    fn test_validate_amount() {
        assert!(validate_amount("100.5", 18).is_ok());
        assert!(validate_amount("0.000000000000000001", 18).is_ok());
        assert!(validate_amount("-100", 18).is_err());
        assert!(validate_amount("0", 18).is_ok());
    }

    /// A non-numeric amount used to pass validation because only the shape of
    /// the split was checked, never the characters themselves.
    #[test]
    fn test_validate_amount_rejects_non_numeric() {
        for bad in ["abc", "", "1e18", "0x10", " 5", "5 ", "1..2", ".5", "5.", "1,000", "NaN", "inf"] {
            assert!(
                validate_amount(bad, 18).is_err(),
                "amount {bad:?} must be rejected"
            );
        }
        assert!(validate_amount("1000000000000000000", 18).is_ok());
    }

    /// `decimals` is caller-supplied. A value above 78 used to underflow the
    /// `78 - decimals` bound and panic instead of returning an error.
    #[test]
    fn test_validate_amount_rejects_out_of_range_decimals_without_panicking() {
        for decimals in [79u8, 100, 255] {
            assert!(
                validate_amount("1", decimals).is_err(),
                "decimals {decimals} must be rejected, not panic"
            );
        }
        // The boundary itself is valid: 78 integer digits with 0 decimals.
        assert!(validate_amount(&"9".repeat(78), 0).is_ok());
        assert!(validate_amount(&"9".repeat(79), 0).is_err());
        // Integer digits plus decimals may not exceed 78 in total.
        assert!(validate_amount(&"9".repeat(78), 78).is_err());
        assert!(validate_amount("0", 78).is_err());
        assert!(validate_amount("0", 77).is_ok());
    }

    /// Truncating a multi-byte string at a byte offset used to panic.
    #[test]
    fn test_sanitize_string_multibyte_truncation() {
        assert_eq!(sanitize_string("\u{4e2d}\u{4e2d}\u{4e2d}", 1), "\u{4e2d}");
        assert_eq!(sanitize_string("ab\u{4e2d}cd", 3), "ab\u{4e2d}");
        assert_eq!(sanitize_string("  hello  ", 100), "hello");
    }

    /// Token symbols were validated as addresses, so every swap request failed.
    #[test]
    fn test_validate_swap_request_accepts_symbols() {
        assert!(validate_swap_request("RUSD", "TGR", "100").is_ok());
        assert!(validate_swap_request("BTC", "ETH", "1.5").is_ok());
        assert!(validate_swap_request("RUSD", "RUSD", "1").is_err());
        assert!(validate_swap_request("rusd", "TGR", "1").is_err());
        assert!(validate_swap_request("RUSD", "TGR", "abc").is_err());
    }
    
    #[test]
    fn test_sanitize_string() {
        assert_eq!(sanitize_string("Hello World!", 20), "Hello World");
        assert_eq!(sanitize_string("Test<script>", 100), "Testscript");
    }
}
