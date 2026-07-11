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

/// Validate a token amount
pub fn validate_amount(amount: &str, decimals: u8) -> Result<(), ValidationError> {
    // Check if it's a valid number
    let parts: Vec<&str> = amount.split('.').collect();
    
    if parts.len() > 2 {
        return Err(ValidationError::InvalidAmount(
            "Invalid number format".to_string()
        ));
    }
    
    // Check integer part
    if parts[0].len() > (78 - decimals as usize) {
        return Err(ValidationError::InvalidAmount(
            "Amount too large".to_string()
        ));
    }
    
    // Check decimal part
    if parts.len() == 2 && parts[1].len() > decimals as usize {
        return Err(ValidationError::InvalidAmount(
            format!("Too many decimals (max {})", decimals)
        ));
    }
    
    // Check for negative
    if amount.starts_with('-') {
        return Err(ValidationError::InvalidAmount(
            "Amount cannot be negative".to_string()
        ));
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
    
    // Trim and truncate
    sanitized = sanitized.trim().to_string();
    if sanitized.len() > max_length {
        sanitized.truncate(max_length);
    }
    
    sanitized
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
    validate_address(from_token)?;
    validate_address(to_token)?;
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
    }
    
    #[test]
    fn test_sanitize_string() {
        assert_eq!(sanitize_string("Hello World!", 20), "Hello World");
        assert_eq!(sanitize_string("Test<script>", 100), "Testscript");
    }
}
