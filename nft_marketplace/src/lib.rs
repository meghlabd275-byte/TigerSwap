//! TigerSwap NFT Marketplace
//! 
//! Decentralized NFT trading with:
//! - NFT listings
//! - Offers
//! - Auctions
//! - Collection management
//!
//! Uses Rust for high performance

use std::collections::HashMap;
use std::sync::Arc;
use tokio::sync::RwLock;
use serde::{Deserialize, Serialize};
use uint::construct_uint;

construct_uint! {
    pub struct U256(4);
}

// ==================== NFT TYPES ====================

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum ListingStatus {
    Active,
    Sold,
    Cancelled,
    Expired,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum SaleType {
    FixedPrice,
    Auction,
    Offer,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum AuctionStatus {
    Pending,
    Active,
    Ended,
    Cancelled,
}

// ==================== NFT STRUCTURE ====================

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct NFT {
    pub id: [u8; 32],
    pub collection: [u8; 20],
    pub token_id: U256,
    pub owner: [u8; 20],
    pub metadata_uri: String,
    pub attributes: Vec<NFTAttribute>,
    pub royalty_fee: u16,  // Basis points
    pub is_listed: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct NFTAttribute {
    pub trait_type: String,
    pub value: String,
    pub display_type: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Collection {
    pub id: [u8; 20],
    pub name: String,
    pub symbol: String,
    pub creator: [u8; 20],
    pub base_uri: String,
    pub total_supply: U256,
    pub trading_fee: u16,  // Basis points
    pub creator_fee: u16, // Basis points
    pub is_verified: bool,
}

// ==================== LISTING ====================

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Listing {
    pub id: [u8; 32],
    pub nft_id: [u8; 32],
    pub seller: [u8; 20],
    pub price: U256,
    pub payment_token: [u8; 20],  // WETH address
    pub sale_type: SaleType,
    pub status: ListingStatus,
    pub created_at: u64,
    pub expires_at: u64,
    pub quantity: U256,
}

impl Listing {
    pub fn new_fixed_price(
        nft_id: [u8; 32],
        seller: [u8; 20],
        price: U256,
        payment_token: [u8; 20],
    ) -> Self {
        Self {
            id: Self::generate_id(&seller, &nft_id),
            nft_id,
            seller,
            price,
            payment_token,
            sale_type: SaleType::FixedPrice,
            status: ListingStatus::Active,
            created_at: current_timestamp(),
            expires_at: current_timestamp() + (7 * 24 * 3600), // 7 days
            quantity: U256::one(),
        }
    }
    
    fn generate_id(seller: &[u8; 20], nft_id: &[u8; 32]) -> [u8; 32] {
        let mut id = [0u8; 32];
        id[..20].copy_from_slice(seller);
        id[20..].copy_from_slice(nft_id);
        id
    }
    
    pub fn cancel(&mut self) {
        self.status = ListingStatus::Cancelled;
    }
    
    pub fn fulfill(&mut self) {
        self.status = ListingStatus::Sold;
    }
}

// ==================== OFFER ====================

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Offer {
    pub id: [u8; 32],
    pub nft_id: [u8; 32],
    pub buyer: [u8; 20],
    pub price: U256,
    pub payment_token: [u8; 20],
    pub status: ListingStatus,
    pub created_at: u64,
    pub expires_at: u64,
}

impl Offer {
    pub fn new(
        nft_id: [u8; 32],
        buyer: [u8; 20],
        price: U256,
        payment_token: [u8; 20],
    ) -> Self {
        let mut id = [0u8; 32];
        id[..20].copy_from_slice(&buyer);
        id[20..].copy_from_slice(&nft_id);
        
        Self {
            id,
            nft_id,
            buyer,
            price,
            payment_token,
            status: ListingStatus::Active,
            created_at: current_timestamp(),
            expires_at: current_timestamp() + (3 * 24 * 3600), // 3 days
        }
    }
}

// ==================== AUCTION ====================

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Auction {
    pub id: [u8; 32],
    pub nft_id: [u8; 32],
    pub seller: [u8; 20],
    pub starting_price: U256,
    pub reserve_price: U256,  // Minimum price to accept
    pub payment_token: [u8; 20],
    pub current_bid: U256,
    pub highest_bidder: Option<[u8; 20]>,
    pub status: AuctionStatus,
    pub start_time: u64,
    pub end_time: u64,
    pub bids_count: u32,
}

impl Auction {
    pub fn new(
        nft_id: [u8; 32],
        seller: [u8; 20],
        starting_price: U256,
        reserve_price: U256,
        payment_token: [u8; 20],
        duration_seconds: u64,
    ) -> Self {
        let mut id = [0u8; 32];
        id[..20].copy_from_slice(&seller);
        id[20..].copy_from_slice(&nft_id);
        
        let now = current_timestamp();
        
        Self {
            id,
            nft_id,
            seller,
            starting_price,
            reserve_price,
            payment_token,
            current_bid: starting_price,
            highest_bidder: None,
            status: AuctionStatus::Pending,
            start_time: now,
            end_time: now + duration_seconds,
            bids_count: 0,
        }
    }
    
    pub fn start(&mut self) {
        if current_timestamp() >= self.start_time {
            self.status = AuctionStatus::Active;
        }
    }
    
    pub fn place_bid(&mut self, bidder: [u8; 20], amount: U256) -> Result<(), NFTError> {
        if self.status != AuctionStatus::Active {
            return Err(NFTError::AuctionNotActive);
        }
        
        if amount <= self.current_bid {
            return Err(NFTError::BidTooLow);
        }
        
        if current_timestamp() > self.end_time {
            return Err(NFTError::AuctionEnded);
        }
        
        self.current_bid = amount;
        self.highest_bidder = Some(bidder);
        self.bids_count += 1;
        
        Ok(())
    }
    
    pub fn end(&mut self) -> Option<[u8; 20]> {
        self.status = AuctionStatus::Ended;
        self.highest_bidder
    }
}

// ==================== MARKETPLACE ====================

pub struct NFTMarketplace {
    nfts: Arc<RwLock<HashMap<[u8; 32], NFT>>>,
    collections: Arc<RwLock<HashMap<[u8; 20], Collection>>>,
    listings: Arc<RwLock<HashMap<[u8; 32], Listing>>>,
    offers: Arc<RwLock<HashMap<[u8; 32], Offer>>>,
    auctions: Arc<RwLock<HashMap<[u8; 32], Auction>>>,
    trading_fee: u16,
}

impl NFTMarketplace {
    pub fn new(trading_fee: u16) -> Self {
        Self {
            nfts: Arc::new(RwLock::new(HashMap::new())),
            collections: Arc::new(RwLock::new(HashMap::new())),
            listings: Arc::new(RwLock::new(HashMap::new())),
            offers: Arc::new(RwLock::new(HashMap::new())),
            auctions: Arc::new(RwLock::new(HashMap::new())),
            trading_fee,
        }
    }
    
    // Collection Management
    pub async fn create_collection(
        &self,
        name: String,
        symbol: String,
        creator: [u8; 20],
        base_uri: String,
    ) -> Result<[u8; 20], NFTError> {
        let mut collection_id = [0u8; 20];
        collection_id[..20].copy_from_slice(&creator);
        
        let collection = Collection {
            id: collection_id,
            name,
            symbol,
            creator,
            base_uri,
            total_supply: U256::zero(),
            trading_fee: self.trading_fee,
            creator_fee: 500, // 5%
            is_verified: false,
        };
        
        self.collections.write().await.insert(collection_id, collection);
        
        Ok(collection_id)
    }
    
    pub async fn mint_nft(
        &self,
        collection: [u8; 20],
        token_id: U256,
        creator: [u8; 20],
        metadata_uri: String,
        attributes: Vec<NFTAttribute>,
    ) -> Result<[u8; 32], NFTError> {
        let mut nft_id = [0u8; 32];
        nft_id[..20].copy_from_slice(&collection);
        nft_id[20..].copy_from_slice(&token_id.to_bytes()[..12]);
        
        let nft = NFT {
            id: nft_id,
            collection,
            token_id,
            owner: creator,
            metadata_uri,
            attributes,
            royalty_fee: 500, // 5%
            is_listed: false,
        };
        
        self.nfts.write().await.insert(nft_id, nft);
        
        Ok(nft_id)
    }
    
    // Listing Management
    pub async fn create_listing(
        &self,
        nft_id: [u8; 32],
        seller: [u8; 20],
        price: U256,
    ) -> Result<[u8; 32], NFTError> {
        // Verify ownership
        let nfts = self.nfts.read().await;
        let nft = nfts.get(&nft_id).ok_or(NFTError::NFTNotFound)?;
        
        if nft.owner != seller {
            return Err(NFTError::NotOwner);
        }
        
        drop(nfts);
        
        let listing = Listing::new_fixed_price(
            nft_id,
            seller,
            price,
            [0u8; 20], // WETH - placeholder
        );
        
        let listing_id = listing.id;
        
        self.listings.write().await.insert(listing_id, listing);
        
        // Mark NFT as listed
        let mut nfts = self.nfts.write().await;
        if let Some(nft) = nfts.get_mut(&nft_id) {
            nft.is_listed = true;
        }
        
        Ok(listing_id)
    }
    
    pub async fn fulfill_listing(
        &self,
        listing_id: [u8; 32],
        buyer: [u8; 20],
    ) -> Result<(), NFTError> {
        let mut listings = self.listings.write().await;
        let listing = listings.get_mut(&listing_id).ok_or(NFTError::ListingNotFound)?;
        
        if listing.status != ListingStatus::Active {
            return Err(NFTError::ListingNotActive);
        }
        
        // Calculate fees
        let trading_fee = (listing.price * U256::from(listing.trading_fee)) / U256::from(10000);
        let creator_fee = (listing.price * U256::from(500)) / U256::from(10000);
        
        // Transfer NFT
        let mut nfts = self.nfts.write().await;
        if let Some(nft) = nfts.get_mut(&listing.nft_id) {
            nft.owner = buyer;
            nft.is_listed = false;
        }
        
        // Update listing
        listing.fulfill();
        
        Ok(())
    }
    
    // Offer Management
    pub async fn create_offer(
        &self,
        nft_id: [u8; 32],
        buyer: [u8; 20],
        price: U256,
    ) -> Result<[u8; 32], NFTError> {
        let offer = Offer::new(
            nft_id,
            buyer,
            price,
            [0u8; 20], // WETH
        );
        
        let offer_id = offer.id;
        self.offers.write().await.insert(offer_id, offer);
        
        Ok(offer_id)
    }
    
    // Auction Management
    pub async fn create_auction(
        &self,
        nft_id: [u8; 32],
        seller: [u8; 20],
        starting_price: U256,
        reserve_price: U256,
        duration_seconds: u64,
    ) -> Result<[u8; 32], NFTError> {
        let auction = Auction::new(
            nft_id,
            seller,
            starting_price,
            reserve_price,
            [0u8; 20],
            duration_seconds,
        );
        
        let auction_id = auction.id;
        self.auctions.write().await.insert(auction_id, auction);
        
        Ok(auction_id)
    }
    
    pub async fn place_bid(
        &self,
        auction_id: [u8; 32],
        bidder: [u8; 20],
        amount: U256,
    ) -> Result<(), NFTError> {
        let mut auctions = self.auctions.write().await;
        let auction = auctions.get_mut(&auction_id).ok_or(NFTError::AuctionNotFound)?;
        
        auction.place_bid(bidder, amount)
    }
    
    // Queries
    pub async fn get_collection(&self, collection_id: &[u8; 20]) -> Option<Collection> {
        let collections = self.collections.read().await;
        collections.get(collection_id).cloned()
    }
    
    pub async fn get_nft(&self, nft_id: &[u8; 32]) -> Option<NFT> {
        let nfts = self.nfts.read().await;
        nfts.get(nft_id).cloned()
    }
    
    pub async fn get_listings_by_seller(&self, seller: &[u8; 20]) -> Vec<Listing> {
        let listings = self.listings.read().await;
        
        listings.values()
            .filter(|l| l.seller == *seller && l.status == ListingStatus::Active)
            .cloned()
            .collect()
    }
    
    pub async fn get_active_listings(&self) -> Vec<Listing> {
        let listings = self.listings.read().await;
        
        listings.values()
            .filter(|l| l.status == ListingStatus::Active)
            .cloned()
            .collect()
    }
}

// ==================== ERRORS ====================

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum NFTError {
    NFTNotFound,
    CollectionNotFound,
    ListingNotFound,
    ListingNotActive,
    AuctionNotFound,
    AuctionNotActive,
    AuctionEnded,
    NotOwner,
    BidTooLow,
    InsufficientPayment,
    InvalidPrice,
}

impl std::fmt::Display for NFTError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            NFTError::NFTNotFound => write!(f, "NFT not found"),
            NFTError::CollectionNotFound => write!(f, "Collection not found"),
            NFTError::ListingNotFound => write!(f, "Listing not found"),
            NFTError::ListingNotActive => write!(f, "Listing not active"),
            NFTError::AuctionNotFound => write!(f, "Auction not found"),
            NFTError::AuctionNotActive => write!(f, "Auction not active"),
            NFTError::AuctionEnded => write!(f, "Auction has ended"),
            NFTError::NotOwner => write!(f, "Not the owner"),
            NFTError::BidTooLow => write!(f, "Bid too low"),
            NFTError::InsufficientPayment => write!(f, "Insufficient payment"),
            NFTError::InvalidPrice => write!(f, "Invalid price"),
        }
    }
}

// ==================== HELPER ====================

fn current_timestamp() -> u64 {
    std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .unwrap()
        .as_secs()
}

trait U256Ext {
    fn one() -> Self;
    fn to_bytes(&self) -> [u8; 32];
}

impl U256Ext for U256 {
    fn one() -> Self { U256::from(1) }
    fn to_bytes(&self) -> [u8; 32] {
        let mut bytes = [0u8; 32];
        let inner = self.0;
        bytes[..8].copy_from_slice(&inner[0].to_le_bytes());
        bytes[8..].copy_from_slice(&inner[1].to_le_bytes());
        bytes
    }
}

// ==================== PUBLIC API ====================

pub mod api {
    use super::*;
    
    pub type MarketplaceHandle = Arc<NFTMarketplace>;
    
    pub fn create_marketplace(trading_fee: u16) -> MarketplaceHandle {
        Arc::new(NFTMarketplace::new(trading_fee))
    }
}

// ==================== TESTS ====================

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_nft_minting() {
        let marketplace = NFTMarketplace::new(250); // 2.5% fee
        
        let collection_id = [1u8; 20];
        let token_id = U256::from(1);
        let creator = [0u8; 20];
        
        // This would need async context in real tests
    }
}