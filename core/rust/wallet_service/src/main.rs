//! Wallet Service Binary - Main entry point

use std::sync::Arc;

use wallet_service::{
    WalletManager, WalletError, Chain,
};

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    println!("TigerSwap Wallet Service");
    println!("===================");
    
    // Initialize wallet manager
    let manager = Arc::new(WalletManager::new());
    
    println!("\nWallet Manager initialized");
    
    // Create test wallets
    let chains = vec![
        ("Ethereum".to_string(), Chain::Ethereum),
        ("Polygon".to_string(), Chain::Polygon),
        ("Arbitrum".to_string(), Chain::Arbitrum),
        ("Optimism".to_string(), Chain::Optimism),
        ("Base".to_string(), Chain::Base),
    ];
    
    println!("\nCreating wallets...");
    let mut wallet_ids = Vec::new();
    
    for (name, chain) in chains {
        match manager.create_wallet(name, chain).await {
            Ok(wallet) => {
                println!("Created {} wallet: {}", chain_to_string(chain), wallet.address);
                wallet_ids.push((wallet.wallet_id, chain));
            }
            Err(e) => {
                println!("Failed to create wallet for {}: {}", chain_to_string(chain), e);
            }
        }
    }
    
    // Test signing
    println!("\nTesting transaction signing...");
    for (wallet_id, chain) in &wallet_ids {
        match manager.sign_transaction(wallet_id, b"test transaction data").await {
            Ok(signature) => {
                println!("Signed transaction on {}: {}...", chain_to_string(*chain), &signature[..16]);
            }
            Err(e) => {
                println!("Failed to sign on {}: {}", chain_to_string(*chain), e);
            }
        }
    }
    
    // Test balance retrieval
    println!("\nTesting balance retrieval...");
    for (wallet_id, chain) in &wallet_ids {
        match manager.get_balance(wallet_id).await {
            Ok(balance) => {
                println!("Balance on {}: {} tokens", chain_to_string(*chain), balance.balances.len());
            }
            Err(e) => {
                println!("Failed to get balance on {}: {}", chain_to_string(*chain), e);
            }
        }
    }
    
    // List all wallets
    println!("\nListing all wallets...");
    let wallets = manager.list_wallets().await;
    println!("Total wallets: {}", wallets.len());
    
    println!("\nWallet Service ready!");
    
    Ok(())
}

fn chain_to_string(chain: Chain) -> String {
    match chain {
        Chain::Ethereum => "Ethereum".to_string(),
        Chain::BinanceSmartChain => "BSC".to_string(),
        Chain::Polygon => "Polygon".to_string(),
        Chain::Avalanche => "Avalanche".to_string(),
        Chain::Arbitrum => "Arbitrum".to_string(),
        Chain::Optimism => "Optimism".to_string(),
        Chain::Solana => "Solana".to_string(),
        Chain::Bitcoin => "Bitcoin".to_string(),
        Chain::Base => "Base".to_string(),
        Chain::ArbitrumNova => "Arbitrum Nova".to_string(),
    }
}