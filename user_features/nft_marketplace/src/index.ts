/**
 * TigerSwap User Features - NFT Marketplace Module
 * 
 * Native NFT marketplace with minting, trading, and collection management.
 * Zero external dependencies - fully native implementation.
 * 
 * Features:
 * - NFT minting
 * - Marketplace trading
 * - Collection management
 * - Royalty system
 * - Auction support
 * 
 * @author TigerSwap
 * @version 1.0.0
 */

import { EVMClient, EVMWallet } from '@tigerswap/evm-sdk';

// ============================================================================
// Types & Interfaces
// ============================================================================

export interface NFTCollection {
  id: string;
  name: string;
  symbol: string;
  owner: string;
  creator: string;
  totalSupply: number;
  maxSupply: number;
  baseURI: string;
  royaltyFee: number; // percentage
  royaltyRecipient: string;
  isVerified: boolean;
  category: string;
}

export interface NFT {
  id: string;
  collection: string;
  tokenId: number;
  owner: string;
  creator: string;
  metadata: NFTMetadata;
  currentPrice: bigint;
  isListed: boolean;
  listingPrice?: bigint;
  royaltyFee: number;
}

export interface NFTMetadata {
  name: string;
  description: string;
  image: string;
  attributes: NFTAttribute[];
  externalUrl?: string;
}

export interface NFTAttribute {
  trait_type: string;
  value: string | number;
  display_type?: string;
}

export interface Listing {
  id: string;
  nftId: string;
  seller: string;
  price: bigint;
  startTime: number;
  endTime?: number;
  status: 'active' | 'sold' | 'cancelled' | 'expired';
}

export interface Bid {
  id: string;
  nftId: string;
  bidder: string;
  amount: bigint;
  timestamp: number;
  status: 'active' | 'accepted' | 'cancelled' | 'expired';
}

export interface Auction {
  id: string;
  nftId: string;
  seller: string;
  startingPrice: bigint;
  currentPrice: bigint;
  highestBidder: string;
  startTime: number;
  endTime: number;
  status: 'active' | 'completed' | 'cancelled';
  bids: Bid[];
}

export interface CollectionStats {
  totalVolume: bigint;
  floorPrice: bigint;
  highestSale: bigint;
  owners: number;
  listed: number;
}

// ============================================================================
// NFT Marketplace
// ============================================================================

export class NFTMarketplace {
  private collections: Map<string, NFTCollection>;
  private nfts: Map<string, NFT>;
  private listings: Map<string, Listing>;
  private auctions: Map<string, Auction>;
  private bids: Map<string, Bid[]>;
  private wallet: EVMWallet;
  private client: EVMClient;

  constructor(wallet: EVMWallet, chainId: number) {
    this.wallet = wallet;
    this.client = new EVMClient(chainId);
    this.collections = new Map();
    this.nfts = new Map();
    this.listings = new Map();
    this.auctions = new Map();
    this.bids = new Map();
  }

  // ============================================================================
  // Collection Management
  // ============================================================================

  /**
   * Create collection
   */
  async createCollection(
    name: string,
    symbol: string,
    maxSupply: number,
    baseURI: string,
    royaltyFee: number,
    category: string
  ): Promise<NFTCollection> {
    const collection: NFTCollection = {
      id: this.generateId(),
      name,
      symbol,
      owner: this.wallet.getAddress(),
      creator: this.wallet.getAddress(),
      totalSupply: 0,
      maxSupply,
      baseURI,
      royaltyFee,
      royaltyRecipient: this.wallet.getAddress(),
      isVerified: false,
      category,
    };

    this.collections.set(collection.id, collection);

    return collection;
  }

  /**
   * Update collection
   */
  updateCollection(collectionId: string, updates: Partial<NFTCollection>): void {
    const collection = this.collections.get(collectionId);
    if (!collection) throw new Error('Collection not found');

    Object.assign(collection, updates);
  }

  /**
   * Get collection
   */
  getCollection(collectionId: string): NFTCollection | null {
    return this.collections.get(collectionId) || null;
  }

  /**
   * Get collection stats
   */
  getCollectionStats(collectionId: string): CollectionStats {
    const collection = this.collections.get(collectionId);
    if (!collection) throw new Error('Collection not found');

    const nftsInCollection = this.getNFTsByCollection(collectionId);
    let totalVolume = 0n;
    let highestSale = 0n;
    let floorPrice = 0n;
    const owners = new Set<string>();
    let listed = 0;

    for (const nft of nftsInCollection) {
      owners.add(nft.owner);
      if (nft.isListed && nft.listingPrice) {
        listed++;
        if (floorPrice === 0n || nft.listingPrice < floorPrice) {
          floorPrice = nft.listingPrice;
        }
      }
      if (nft.currentPrice > highestSale) {
        highestSale = nft.currentPrice;
      }
    }

    return {
      totalVolume,
      floorPrice,
      highestSale,
      owners: owners.size,
      listed,
    };
  }

  // ============================================================================
  // NFT Minting
  // ============================================================================

  /**
   * Mint NFT
   */
  async mintNFT(
    collectionId: string,
    metadata: NFTMetadata,
    royaltyFee?: number
  ): Promise<NFT> {
    const collection = this.collections.get(collectionId);
    if (!collection) throw new Error('Collection not found');

    if (collection.totalSupply >= collection.maxSupply) {
      throw new Error('Collection max supply reached');
    }

    const nft: NFT = {
      id: this.generateId(),
      collection: collectionId,
      tokenId: collection.totalSupply,
      owner: this.wallet.getAddress(),
      creator: this.wallet.getAddress(),
      metadata,
      currentPrice: 0n,
      isListed: false,
      royaltyFee: royaltyFee || collection.royaltyFee,
    };

    this.nfts.set(nft.id, nft);
    collection.totalSupply++;

    return nft;
  }

  /**
   * Batch mint
   */
  async batchMintNFT(
    collectionId: string,
    metadataList: NFTMetadata[]
  ): Promise<NFT[]> {
    const nfts: NFT[] = [];
    for (const metadata of metadataList) {
      const nft = await this.mintNFT(collectionId, metadata);
      nfts.push(nft);
    }
    return nfts;
  }

  /**
   * Transfer NFT
   */
  async transferNFT(nftId: string, to: string): Promise<void> {
    const nft = this.nfts.get(nftId);
    if (!nft) throw new Error('NFT not found');

    if (nft.owner !== this.wallet.getAddress()) {
      throw new Error('Not the owner');
    }

    if (nft.isListed) {
      throw new Error('Cannot transfer listed NFT');
    }

    nft.owner = to;
  }

  // ============================================================================
  // Marketplace Trading
  // ============================================================================

  /**
   * List NFT for sale
   */
  async listNFT(nftId: string, price: bigint): Promise<Listing> {
    const nft = this.nfts.get(nftId);
    if (!nft) throw new Error('NFT not found');

    if (nft.owner !== this.wallet.getAddress()) {
      throw new Error('Not the owner');
    }

    if (nft.isListed) {
      throw new Error('Already listed');
    }

    const listing: Listing = {
      id: this.generateId(),
      nftId,
      seller: this.wallet.getAddress(),
      price,
      startTime: Date.now(),
      status: 'active',
    };

    this.listings.set(listing.id, listing);

    nft.isListed = true;
    nft.listingPrice = price;

    return listing;
  }

  /**
   * Buy NFT
   */
  async buyNFT(listingId: string): Promise<void> {
    const listing = this.listings.get(listingId);
    if (!listing) throw new Error('Listing not found');

    if (listing.status !== 'active') {
      throw new Error('Listing not active');
    }

    const nft = this.nfts.get(listing.nftId);
    if (!nft) throw new Error('NFT not found');

    // Calculate total price including fees
    const price = listing.price;
    const royalty = (price * BigInt(nft.royaltyFee)) / 10000n;

    // Transfer NFT
    nft.owner = this.wallet.getAddress();
    nft.isListed = false;
    nft.isListed = false;
    nft.currentPrice = price;
    nft.listingPrice = undefined;

    listing.status = 'sold';
  }

  /**
   * Cancel listing
   */
  async cancelListing(listingId: string): Promise<void> {
    const listing = this.listings.get(listingId);
    if (!listing) throw new Error('Listing not found');

    if (listing.seller !== this.wallet.getAddress()) {
      throw new Error('Not the seller');
    }

    const nft = this.nfts.get(listing.nftId);
    if (nft) {
      nft.isListed = false;
      nft.listingPrice = undefined;
    }

    listing.status = 'cancelled';
  }

  // ============================================================================
  // Auctions
  // ============================================================================

  /**
   * Create auction
   */
  async createAuction(
    nftId: string,
    startingPrice: bigint,
    duration: number // seconds
  ): Promise<Auction> {
    const nft = this.nfts.get(nftId);
    if (!nft) throw new Error('NFT not found');

    if (nft.owner !== this.wallet.getAddress()) {
      throw new Error('Not the owner');
    }

    const auction: Auction = {
      id: this.generateId(),
      nftId,
      seller: this.wallet.getAddress(),
      startingPrice,
      currentPrice: startingPrice,
      highestBidder: '',
      startTime: Date.now(),
      endTime: Date.now() + duration * 1000,
      status: 'active',
      bids: [],
    };

    this.auctions.set(auction.id, auction);
    nft.isListed = true;

    return auction;
  }

  /**
   * Place bid
   */
  async placeBid(auctionId: string, amount: bigint): Promise<Bid> {
    const auction = this.auctions.get(auctionId);
    if (!auction) throw new Error('Auction not found');

    if (auction.status !== 'active') {
      throw new Error('Auction not active');
    }

    if (Date.now() > auction.endTime) {
      throw new Error('Auction ended');
    }

    if (amount <= auction.currentPrice) {
      throw new Error('Bid must be higher than current price');
    }

    const bid: Bid = {
      id: this.generateId(),
      nftId: auction.nftId,
      bidder: this.wallet.getAddress(),
      amount,
      timestamp: Date.now(),
      status: 'active',
    };

    auction.bids.push(bid);
    auction.currentPrice = amount;
    auction.highestBidder = this.wallet.getAddress();

    // Store bid
    const auctionBids = this.bids.get(auctionId) || [];
    auctionBids.push(bid);
    this.bids.set(auctionId, auctionBids);

    return bid;
  }

  /**
   * Settle auction
   */
  async settleAuction(auctionId: string): Promise<void> {
    const auction = this.auctions.get(auctionId);
    if (!auction) throw new Error('Auction not found');

    if (auction.status !== 'active') {
      throw new Error('Auction not active');
    }

    const nft = this.nfts.get(auction.nftId);
    if (!nft) throw new Error('NFT not found');

    if (auction.highestBidder) {
      // Transfer NFT to highest bidder
      nft.owner = auction.highestBidder;
      nft.currentPrice = auction.currentPrice;
    }

    nft.isListed = false;
    auction.status = 'completed';
  }

  /**
   * Cancel auction
   */
  async cancelAuction(auctionId: string): Promise<void> {
    const auction = this.auctions.get(auctionId);
    if (!auction) throw new Error('Auction not found');

    if (auction.seller !== this.wallet.getAddress()) {
      throw new Error('Not the seller');
    }

    const nft = this.nfts.get(auction.nftId);
    if (nft) {
      nft.isListed = false;
    }

    auction.status = 'cancelled';
  }

  // ============================================================================
  // Queries
  // ============================================================================

  /**
   * Get NFT
   */
  getNFT(nftId: string): NFT | null {
    return this.nfts.get(nftId) || null;
  }

  /**
   * Get NFTs by collection
   */
  getNFTsByCollection(collectionId: string): NFT[] {
    return Array.from(this.nfts.values()).filter(
      nft => nft.collection === collectionId
    );
  }

  /**
   * Get NFTs by owner
   */
  getNFTsByOwner(owner: string): NFT[] {
    return Array.from(this.nfts.values()).filter(
      nft => nft.owner === owner
    );
  }

  /**
   * Get listings
   */
  getActiveListings(): Listing[] {
    return Array.from(this.listings.values()).filter(
      l => l.status === 'active'
    );
  }

  /**
   * Get auctions
   */
  getActiveAuctions(): Auction[] {
    return Array.from(this.auctions.values()).filter(
      a => a.status === 'active'
    );
  }

  /**
   * Search NFTs
   */
  searchNFTs(
    collectionId?: string,
    minPrice?: bigint,
    maxPrice?: bigint,
    attributes?: Record<string, string | number>
  ): NFT[] {
    let results = Array.from(this.nfts.values());

    if (collectionId) {
      results = results.filter(nft => nft.collection === collectionId);
    }

    if (minPrice !== undefined) {
      results = results.filter(nft => nft.currentPrice >= minPrice);
    }

    if (maxPrice !== undefined) {
      results = results.filter(nft => nft.currentPrice <= maxPrice);
    }

    if (attributes) {
      results = results.filter(nft => {
        if (!nft.metadata.attributes) return false;
        for (const [trait, value] of Object.entries(attributes)) {
          const attr = nft.metadata.attributes.find(
            a => a.trait_type === trait
          );
          if (!attr || attr.value !== value) return false;
        }
        return true;
      });
    }

    return results;
  }

  // ============================================================================
  // Utility
  // ============================================================================

  private generateId(): string {
    return `${Date.now()}_${Math.random().toString(36).slice(2, 9)}`;
  }
}

// ============================================================================
// NFT Royalty Calculator
// ============================================================================

export class RoyaltyCalculator {
  /**
   * Calculate royalty
   */
  static calculateRoyalty(salePrice: bigint, royaltyFee: number): bigint {
    return (salePrice * BigInt(royaltyFee)) / 10000n;
  }

  /**
   * Calculate platform fee
   */
  static calculatePlatformFee(salePrice: bigint, platformFee: number): bigint {
    return (salePrice * BigInt(platformFee)) / 10000n;
  }

  /**
   * Calculate net proceeds
   */
  static calculateNetProceeds(
    salePrice: bigint,
    royaltyFee: number,
    platformFee: number
  ): bigint {
    const royalty = this.calculateRoyalty(salePrice, royaltyFee);
    const platform = this.calculatePlatformFee(salePrice, platformFee);
    return salePrice - royalty - platform;
  }
}

// ============================================================================
// Export
// ============================================================================

export default {
  NFTMarketplace,
  RoyaltyCalculator,
};