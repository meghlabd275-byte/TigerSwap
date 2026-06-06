//! Auction Engine
//! 
//! Runs auctions for intent execution.

use std::collections::HashMap;
use std::sync::{Arc, RwLock};

/// Bid for solver
#[derive(Debug, Clone)]
pub struct Bid {
    pub solver_id: String,
    pub intent_id: String,
    pub amount: u64,
    pub gas_price: u64,
}

/// Intent reference
#[derive(Debug, Clone)]
pub struct Intent {
    pub id: String,
    pub data: Vec<u8>,
}

#[derive(Debug, Clone)]
pub struct Auction {
    pub id: String,
    pub intent_id: String,
    pub bids: Vec<AuctionBid>,
    pub status: AuctionStatus,
    pub start_time: u64,
    pub end_time: u64,
}

#[derive(Debug, Clone)]
pub struct AuctionBid {
    pub auction_id: String,
    pub solver_id: String,
    pub amount: u64,
    pub gas_price: u64,
    pub timestamp: u64,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum AuctionStatus {
    Pending,
    Running,
    Completed,
    Cancelled,
}

pub struct AuctionEngine {
    auctions: RwLock<HashMap<String, Auction>>,
    auction_duration: u64,
}

impl AuctionEngine {
    pub fn new() -> Self {
        Self {
            auctions: RwLock::new(HashMap::new()),
            auction_duration: 30, // 30 seconds
        }
    }
    
    pub fn create_auction(&self, intent_id: &str) -> String {
        let id = format!("auction-{}", intent_id);
        let auction = Auction {
            id: id.clone(),
            intent_id: intent_id.to_string(),
            bids: Vec::new(),
            status: AuctionStatus::Pending,
            start_time: current_timestamp(),
            end_time: current_timestamp() + self.auction_duration,
        };
        
        self.auctions.write().unwrap().insert(id.clone(), auction);
        id
    }
    
    pub fn submit_bid(&self, auction_id: &str, solver_id: &str, amount: u64, gas_price: u64) -> Result<(), String> {
        let mut auctions = self.auctions.write().unwrap();
        let auction = auctions.get_mut(auction_id)
            .ok_or("auction not found")?;
        
        if auction.status != AuctionStatus::Pending && auction.status != AuctionStatus::Running {
            return Err("auction not accepting bids".to_string());
        }
        
        let bid = AuctionBid {
            auction_id: auction_id.to_string(),
            solver_id: solver_id.to_string(),
            amount,
            gas_price,
            timestamp: current_timestamp(),
        };
        
        auction.bids.push(bid);
        auction.status = AuctionStatus::Running;
        
        Ok(())
    }
    
    pub fn run_auction(&self, intent: &Intent) -> Result<Vec<Bid>, String> {
        // Simplified auction run
        let intent_id = &intent.id;
        let auction_id = self.create_auction(intent_id);
        
        let auction = self.auctions.read().unwrap()
            .get(&auction_id)
            .cloned()
            .ok_or("auction not found")?;
        
        if auction.bids.is_empty() {
            return Err("no bids".to_string());
        }
        
        // Convert to solver bids
        let bids: Vec<Bid> = auction.bids.iter().map(|b| Bid {
            solver_id: b.solver_id.clone(),
            intent_id: b.auction_id.clone(),
            amount: b.amount,
            gas_price: b.gas_price,
        }).collect();
        
        Ok(bids)
    }
    
    pub fn end_auction(&self, auction_id: &str) {
        if let Some(auction) = self.auctions.write().unwrap().get_mut(auction_id) {
            auction.status = AuctionStatus::Completed;
        }
    }
}

fn current_timestamp() -> u64 {
    std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .unwrap()
        .as_secs()
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_auction() {
        let engine = AuctionEngine::new();
        let id = engine.create_auction("intent-1");
        assert!(!id.is_empty());
    }
}