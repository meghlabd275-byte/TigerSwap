//! Event Module

use serde::{Deserialize, Serialize};

/// Event
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Event {
    pub event_id: String,
    pub block_number: u64,
    pub transaction_hash: String,
    pub contract_address: String,
    pub event_name: String,
    pub topics: Vec<String>,
    pub data: Vec<u8>,
    pub log_index: u64,
}

impl Event {
    pub fn new(block_number: u64, contract: &str, name: &str, topics: Vec<String>) -> Self {
        Self {
            event_id: uuid::Uuid::new_v4().to_string(),
            block_number,
            transaction_hash: String::new(),
            contract_address: contract.to_string(),
            event_name: name.to_string(),
            topics,
            data: Vec::new(),
            log_index: 0,
        }
    }
    
    pub fn with_tx_hash(mut self, hash: &str) -> Self {
        self.transaction_hash = hash.to_string();
        self
    }
    
    pub fn is_transfer(&self) -> bool {
        self.event_name == "Transfer"
    }
    
    pub fn is_approval(&self) -> bool {
        self.event_name == "Approval"
    }
}

/// Event filter
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct EventFilter {
    pub address: Option<String>,
    pub topics: Vec<Option<String>>,
    pub from_block: Option<u64>,
    pub to_block: Option<u64>,
}

impl EventFilter {
    pub fn new() -> Self {
        Self::default()
    }
    
    pub fn with_address(mut self, addr: &str) -> Self {
        self.address = Some(addr.to_string());
        self
    }
    
    pub fn matches(&self, event: &Event) -> bool {
        if let Some(ref addr) = self.address {
            if &event.contract_address != addr {
                return false;
            }
        }
        
        if let Some(from) = self.from_block {
            if event.block_number < from {
                return false;
            }
        }
        
        if let Some(to) = self.to_block {
            if event.block_number > to {
                return false;
            }
        }
        
        true
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_event() {
        let event = Event::new(12345, "0xcontract", "Transfer", vec![]);
        assert!(event.is_transfer());
    }
}