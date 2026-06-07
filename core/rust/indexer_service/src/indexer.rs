//! Indexer Module - Main indexer

use std::collections::{HashMap, VecDeque};
use std::sync::Arc;
use tokio::sync::RwLock;

use crate::{Block, Transaction, Event, AccountState, TokenBalance, BlockDecoder, EventParser, StateReconciliation};

/// Main indexer
pub struct Indexer {
    decoder: BlockDecoder,
    parser: EventParser,
    reconciliation: Arc<StateReconciliation>,
    blocks: RwLock<HashMap<u64, Block>>,
    transactions: RwLock<HashMap<String, Transaction>>,
    events: RwLock<VecDeque<Event>>,
    latest_block: RwLock<u64>,
}

impl Indexer {
    pub fn new() -> Self {
        Self {
            decoder: BlockDecoder::new(),
            parser: EventParser::new(),
            reconciliation: Arc::new(StateReconciliation::new()),
            blocks: RwLock::new(HashMap::new()),
            transactions: RwLock::new(HashMap::new()),
            events: RwLock::new(VecDeque::new()),
            latest_block: RwLock::new(0),
        }
    }
    
    /// Index a block with transactions and events
    pub async fn index_block(
        &self,
        block: Block,
        transactions: Vec<Transaction>,
        events: Vec<Event>,
    ) -> Result<(), crate::IndexerError> {
        let block_number = block.block_number;
        
        // Store block
        {
            let mut blocks = self.blocks.write().await;
            blocks.insert(block_number, block);
        }
        
        // Store transactions
        {
            let mut txs = self.transactions.write().await;
            for tx in &transactions {
                txs.insert(tx.tx_hash.clone(), tx.clone());
            }
        }
        
        // Store events
        {
            let mut evts = self.events.write().await;
            for event in events {
                evts.push_back(event);
            }
            while evts.len() > 10000 {
                evts.pop_front();
            }
        }
        
        // Update latest block
        {
            let mut latest = self.latest_block.write().await;
            *latest = block_number;
        }
        
        Ok(())
    }
    
    /// Get block by number
    pub async fn get_block(&self, block_number: u64) -> Option<Block> {
        let blocks = self.blocks.read().await;
        blocks.get(&block_number).cloned()
    }
    
    /// Get transaction by hash
    pub async fn get_transaction(&self, tx_hash: &str) -> Option<Transaction> {
        let txs = self.transactions.read().await;
        txs.get(tx_hash).cloned()
    }
    
    /// Get events for address
    pub async fn get_events(&self, address: &str, from_block: u64, to_block: u64) -> Vec<Event> {
        let evts = self.events.read().await;
        evts.iter()
            .filter(|e| e.contract_address == address && e.block_number >= from_block && e.block_number <= to_block)
            .cloned()
            .collect()
    }
    
    /// Get latest block number
    pub async fn get_latest_block(&self) -> u64 {
        *self.latest_block.read().await
    }
    
    /// Get account state
    pub async fn get_account_state(&self, address: &str) -> Option<AccountState> {
        self.reconciliation.get_state(address).await
    }
    
    /// Get token balance
    pub async fn get_token_balance(&self, address: &str, token: &str) -> Option<TokenBalance> {
        self.reconciliation.get_token_balance(address, token).await
    }
    
    /// Get decoder
    pub fn decoder(&self) -> &BlockDecoder {
        &self.decoder
    }
    
    /// Get parser
    pub fn parser(&self) -> &EventParser {
        &self.parser
    }
}

impl Default for Indexer {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[tokio::test]
    async fn test_indexer() {
        let indexer = Indexer::new();
        
        let block = Block::new(1);
        let txs = vec![];
        let events = vec![];
        
        indexer.index_block(block, txs, events).await.unwrap();
        
        let latest = indexer.get_latest_block().await;
        assert_eq!(latest, 1);
    }
}