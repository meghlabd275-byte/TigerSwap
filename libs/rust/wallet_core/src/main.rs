//! TigerSwap Wallet CLI
//! Command-line interface for wallet operations

use clap::{Parser, Subcommand};
use std::sync::Arc;
use tokio::sync::RwLock;
use tiger_wallet_core::{WalletManager, WalletAccount};

mod commands;

#[derive(Parser)]
#[command(name = "tiger-wallet")]
#[command(about = "TigerSwap Wallet - Secure multi-chain wallet", long_about = None)]
struct Cli {
    #[command(subcommand)]
    command: Commands,
}

#[derive(Subcommand)]
enum Commands {
    /// Create new wallet
    Create {
        name: String,
        password: String,
    },
    /// Import existing wallet
    Import {
        name: String,
        password: String,
        mnemonic: String,
    },
    /// Import private key
    ImportKey {
        name: String,
        password: String,
        private_key: String,
    },
    /// List accounts
    List,
    /// Get account info
    Info {
        account_id: String,
    },
    /// Get address
    Address {
        account_id: String,
        chain_id: Option<u32>,
    },
    /// Sign message
    Sign {
        account_id: String,
        chain_id: u32,
        message: String,
    },
    /// Sign transaction
    SignTx {
        account_id: String,
        chain_id: u32,
        to: String,
        value: String,
    },
}

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    // Initialize logging
    tracing_subscriber::fmt()
        .with_env_filter("tiger_wallet_core=info")
        .init();
    
    let cli = Cli::parse();
    let manager = Arc::new(RwLock::new(WalletManager::new()));
    
    // Initialize default chains
    manager.read().await.init_default_chains().await;
    
    match cli.command {
        Commands::Create { name, password } => {
            // Generate new mnemonic
            let mnemonic = "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about";
            
            let account = manager.read().await
                .create_from_mnemonic(mnemonic, &password, name)
                .await?;
            
            println!("Wallet created successfully!");
            println!("Account ID: {}", account.id);
            println!("Mnemonic: {}", mnemonic);
            println!("\nIMPORTANT: Save your mnemonic phrase securely!");
        },
        
        Commands::Import { name, password, mnemonic } => {
            let account = manager.read().await
                .create_from_mnemonic(&mnemonic, &password, name)
                .await?;
            
            println!("Wallet imported successfully!");
            println!("Account ID: {}", account.id);
        },
        
        Commands::ImportKey { name, password, private_key } => {
            let key_bytes = hex::decode(private_key.trim_start_matches("0x"))
                .expect("Invalid private key");
            
            let account = manager.read().await
                .import_private_key(&key_bytes, &password, name)
                .await?;
            
            println!("Key imported successfully!");
            println!("Account ID: {}", account.id);
        },
        
        Commands::List => {
            let accounts = manager.read().await.list_accounts().await;
            
            if accounts.is_empty() {
                println!("No accounts found");
            } else {
                for account in accounts {
                    println!("{}", account.id);
                }
            }
        },
        
        Commands::Info { account_id } => {
            let account = manager.read().await.get_account(&account_id).await;
            
            match account {
                Some(acc) => {
                    println!("Account ID: {}", acc.id);
                    println!("Name: {}", acc.name);
                    println!("Created: {}", acc.created_at);
                    println!("Keys:");
                    for (chain_id, key) in &acc.keys {
                        println!("  Chain {}: {}", chain_id, key.address);
                    }
                },
                None => println!("Account not found"),
            }
        },
        
        Commands::Address { account_id, chain_id } => {
            let chain = chain_id.unwrap_or(1);
            let address = manager.read().await.get_address(&account_id, chain).await;
            
            match address {
                Some(addr) => println!("{}", addr),
                None => println!("Address not found"),
            }
        },
        
        Commands::Sign { account_id, chain_id, message } => {
            let signature = manager.read().await
                .sign_message(&account_id, chain_id, message.as_bytes())
                .await;
            
            match signature {
                Ok(sig) => println!("Signature: {}", hex::encode(sig)),
                Err(e) => println!("Error: {:?}", e),
            }
        },
        
        Commands::SignTx { account_id, chain_id, to, value } => {
            let mut tx = tiger_wallet_core::Transaction::new();
            tx.to = to;
            tx.value = value;
            
            let signature = manager.read().await
                .sign_transaction(&account_id, chain_id, &mut tx)
                .await;
            
            match signature {
                Ok(sig) => {
                    println!("Transaction signed!");
                    println!("Signature: {}", hex::encode(sig));
                },
                Err(e) => println!("Error: {:?}", e),
            }
        },
    }
    
    Ok(())
}
