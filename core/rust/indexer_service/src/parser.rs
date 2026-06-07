//! Parser Module - Event parsing

use crate::{IndexerError, Event};
use sha2::{Digest, Sha256};

/// Event parser
pub struct EventParser;

impl EventParser {
    pub fn new() -> Self {
        Self
    }
    
    /// Parse event from log data
    pub fn parse_event(&self, data: &[u8], contract: &str) -> Result<Event, IndexerError> {
        let event_name = self.identify_event(data);
        
        Ok(Event::new(0, contract, &event_name, vec![]))
    }
    
    /// Identify event from topics
    fn identify_event(&self, data: &[u8]) -> String {
        if data.len() >= 32 {
            let hash = Sha256::digest(&data[..32]);
            let topic = hex::encode(&hash[..4]);
            
            match topic.as_str() {
                "ddf2" | "a905" => "Transfer".to_string(),
                "8c5a" | "095e" => "Approval".to_string(),
                "4d5c4" | "d78b" => "Swap".to_string(),
                "a9c8c" => "Mint".to_string(),
                "2e6c7" => "Burn".to_string(),
                "0dfe1" => "Deposit".to_string(),
                "47e71" => "Withdrawal".to_string(),
                _ => "Unknown".to_string(),
            }
        } else {
            "Unknown".to_string()
        }
    }
    
    /// Get event signature hash
    pub fn event_signature(&self, name: &str) -> String {
        let sig = format!("{}(", name);
        let hash = Sha256::digest(sig.as_bytes());
        hex::encode(&hash[..4])
    }
}

impl Default for EventParser {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_parse_event() {
        let parser = EventParser::new();
        let event = parser.parse_event(b"test data", "0x123").unwrap();
        assert!(!event.event_name.is_empty());
    }
}