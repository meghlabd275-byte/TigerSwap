//! Indexer Service Binary

use indexer_service::{Indexer, Block, Transaction, Event};

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    println!("TigerSwap Indexer Service");
    println!("=====================");
    
    // Initialize indexer
    let indexer = Indexer::new();
    
    println!("Indexer initialized");
    
    // Index test blocks
    println!("\nIndexing test blocks...");
    for i in 1..=10 {
        let block = Block::new(i)
            .with_miner("0x1234567890123456789012345678901234567890".to_string())
            .with_gas_used(15000000);
        
        let txs = vec![];
        let events = vec![];
        
        indexer.index_block(block, txs, events).await.unwrap();
        println!("Indexed block {}", i);
    }
    
    // Get latest block
    let latest = indexer.get_latest_block().await;
    println!("\nLatest block: {}", latest);
    
    // Get block
    let block = indexer.get_block(5).await;
    if let Some(b) = block {
        println!("Block 5 hash: {}", b.block_hash);
    }
    
    println!("\nIndexer Service ready!");
    
    Ok(())
}