// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import "@openzeppelin/contracts/token/ERC721/ERC721.sol";
import "@openzeppelin/contracts/token/ERC721/extensions/ERC721URIStorage.sol";
import "@openzeppelin/contracts/token/ERC20/ERC20.sol";
import "@openzeppelin/contracts/token/ERC20/utils/SafeERC20.sol";
import "@openzeppelin/contracts/access/AccessControl.sol";
import "@openzeppelin/contracts/utils/ReentrancyGuard.sol";

/**
 * @title TigerNFTMarketplace
 * @notice NFT Marketplace with auctions, offers, and collection royalties
 */

contract TigerNFTMarketplace is ERC721, ERC721URIStorage, AccessControl, ReentrancyGuard {
    using SafeERC20 for IERC20;
    
    bytes32 public constant WARDEN_ROLE = keccak256("WARDEN_ROLE");
    bytes32 public constant CURATOR_ROLE = keccak256("CURATOR_ROLE");
    
    // Fee constants (basis points)
    uint256 public constant PLATFORM_FEE_BPS = 250; // 2.5%
    uint256 public constant MAX_ROYALTY_BPS = 1000; // 10%
    
    // NFT data
    struct NFT {
        uint256 tokenId;
        address collection;
        address creator;
        string uri;
        uint256 price;
        address paymentToken;
        bool listed;
        bool sold;
    }
    
    // Collection data
    struct Collection {
        address owner;
        string name;
        string symbol;
        string baseURI;
        uint256 royaltyBps;
        bool verified;
    }
    
    // Auction data
    struct Auction {
        uint256 tokenId;
        address collection;
        uint256 startingPrice;
        uint256 highestBid;
        address highestBidder;
        uint256 endTime;
        bool ended;
    }
    
    // State
    uint256 public nextCollectionId;
    uint256 public nextTokenId;
    uint256 public platformFees;
    
    // Mappings
    mapping(uint256 => Collection) public collections;
    mapping(address => uint256[]) public collectionTokens;
    mapping(uint256 => NFT) public nfts;
    mapping(uint256 => Auction) public auctions;
    mapping(address => mapping(address => uint256)) public royalties;
    mapping(address => mapping(address => uint256)) public pendingWithdrawals;
    
    // Events
    event CollectionCreated(uint256 indexed id, address indexed owner, string name);
    event NFTListed(uint256 indexed tokenId, address indexed collection, uint256 price);
    event NFTSold(uint256 indexed tokenId, address buyer, uint256 price);
    event NFTDelisted(uint256 indexed tokenId);
    event AuctionStarted(uint256 indexed tokenId, uint256 startingPrice, uint256 endTime);
    event BidPlaced(uint256 indexed tokenId, address bidder, uint256 amount);
    event AuctionEnded(uint256 indexed tokenId, address winner, uint256 amount);
    
    modifier onlyCurators() {
        require(hasRole(CURATOR_ROLE, msg.sender), "Not curator");
        _;
    }
    
    constructor() ERC721("TigerNFT Marketplace", "TNFT") {
        _grantRole(DEFAULT_ADMIN_ROLE, msg.sender);
        _grantRole(WARDEN_ROLE, msg.sender);
        _grantRole(CURATOR_ROLE, msg.sender);
    }
    
    /**
     * @notice Create collection
     * @param name Collection name
     * @param symbol Collection symbol
     * @param baseURI Base URI for metadata
     * @param royaltyBps Royalty in basis points
     */
    function createCollection(
        string memory name,
        string memory symbol,
        string memory baseURI,
        uint256 royaltyBps
    ) external returns (uint256) {
        require(royaltyBps <= MAX_ROYALTY_BPS, "Royalty too high");
        
        uint256 id = ++nextCollectionId;
        
        collections[id] = Collection({
            owner: msg.sender,
            name: name,
            symbol: symbol,
            baseURI: baseURI,
            royaltyBps: royaltyBps,
            verified: false
        });
        
        emit CollectionCreated(id, msg.sender, name);
        
        return id;
    }
    
    /**
     * @notice Mint NFT
     * @param collection Collection ID
     * @param to Recipient address
     * @param uri Token URI
     * @param price Price (0 for not listed)
     */
    function mint(
        uint256 collection,
        address to,
        string memory uri,
        uint256 price
    ) external returns (uint256) {
        Collection storage col = collections[collection];
        require(col.owner == msg.sender, "Not owner");
        
        uint256 tokenId = ++nextTokenId;
        _mint(to, tokenId);
        _setTokenURI(tokenId, uri);
        
        nfts[tokenId] = NFT({
            tokenId: tokenId,
            collection: to, // Using contract address as collection identifier
            creator: msg.sender,
            uri: uri,
            price: price,
            paymentToken: address(0),
            listed: price > 0,
            sold: false
        });
        
        collectionTokens[to].push(tokenId);
        
        if (price > 0) {
            emit NFTListed(tokenId, to, price);
        }
        
        return tokenId;
    }
    
    /**
     * @notice List NFT for sale
     * @param tokenId Token ID
     * @param price Price
     */
    function list(uint256 tokenId, uint256 price) external {
        require(ownerOf(tokenId) == msg.sender, "Not owner");
        
        NFT storage nft = nfts[tokenId];
        nft.price = price;
        nft.listed = price > 0;
        
        if (price > 0) {
            emit NFTListed(tokenId, nft.collection, price);
        } else {
            emit NFTDelisted(tokenId);
        }
    }
    
    /**
     * @notice Buy NFT
     * @param tokenId Token ID
     */
    function buy(uint256 tokenId) external payable nonReentrant {
        NFT storage nft = nfts[tokenId];
        require(nft.listed, "Not listed");
        require(!nft.sold, "Already sold");
        
        uint256 price = nft.price;
        require(msg.value >= price, "Insufficient payment");
        
        // Calculate fees
        uint256 platformFee = (price * PLATFORM_FEE_BPS) / 10000;
        uint256 royalty = (price * collections[nft.collection].royaltyBps) / 10000;
        
        // Mark as sold
        nft.sold = true;
        nft.listed = false;
        
        // Transfer NFT
        _transfer(ownerOf(tokenId), msg.sender, tokenId);
        
        // Pay creator royalty
        if (royalty > 0) {
            pendingWithdrawals[nft.creator][address(0)] += royalty;
        }
        
        // Transfer to seller
        address seller = ownerOf(tokenId);
        pendingWithdrawals[seller][address(0)] += (price - platformFee - royalty);
        
        // Platform fee
        platformFees += platformFee;
        
        emit NFTSold(tokenId, msg.sender, price);
        
        // Refund excess
        if (msg.value > price) {
            payable(msg.sender).transfer(msg.value - price);
        }
    }
    
    /**
     * @notice Start auction
     * @param tokenId Token ID
     * @param startingPrice Starting bid price
     * @param duration Auction duration in seconds
     */
    function startAuction(uint256 tokenId, uint256 startingPrice, uint256 duration) external {
        require(ownerOf(tokenId) == msg.sender, "Not owner");
        
        auctions[tokenId] = Auction({
            tokenId: tokenId,
            collection: nfts[tokenId].collection,
            startingPrice: startingPrice,
            highestBid: startingPrice,
            highestBidder: address(0),
            endTime: block.timestamp + duration,
            ended: false
        });
        
        emit AuctionStarted(tokenId, startingPrice, block.timestamp + duration);
    }
    
    /**
     * @notice Place bid
     * @param tokenId Token ID
     */
    function placeBid(uint256 tokenId) external payable nonReentrant {
        Auction storage auction = auctions[tokenId];
        require(!auction.ended, "Auction ended");
        require(block.timestamp < auction.endTime, "Auction expired");
        
        uint256 bid = msg.value;
        require(bid > auction.highestBid, "Bid too low");
        
        // Refund previous highest bidder
        if (auction.highestBidder != address(0)) {
            pendingWithdrawals[auction.highestBidder][address(0)] += auction.highestBid;
        }
        
        auction.highestBid = bid;
        auction.highestBidder = msg.sender;
        
        emit BidPlaced(tokenId, msg.sender, bid);
    }
    
    /**
     * @notice End auction
     * @param tokenId Token ID
     */
    function endAuction(uint256 tokenId) external nonReentrant {
        Auction storage auction = auctions[tokenId];
        require(!auction.ended, "Already ended");
        require(block.timestamp >= auction.endTime, "Not ended");
        
        auction.ended = true;
        
        if (auction.highestBidder != address(0)) {
            // Transfer NFT
            _transfer(ownerOf(tokenId), auction.highestBidder, tokenId);
            
            // Calculate fees
            uint256 platformFee = (auction.highestBid * PLATFORM_FEE_BPS) / 10000;
            uint256 royalty = (auction.highestBid * collections[auction.collection].royaltyBps) / 10000;
            
            // Transfer to seller
            address seller = ownerOf(tokenId);
            pendingWithdrawals[seller][address(0)] += (auction.highestBid - platformFee - royalty);
            
            // Pay royalty
            if (royalty > 0) {
                pendingWithdrawals[nfts[tokenId].creator][address(0)] += royalty;
            }
            
            // Platform fee
            platformFees += platformFee;
            
            emit AuctionEnded(tokenId, auction.highestBidder, auction.highestBid);
        }
    }
    
    /**
     * @notice Withdraw funds
     */
    function withdraw() external nonReentrant {
        uint256 amount = pendingWithdrawals[msg.sender][address(0)];
        require(amount > 0, "No funds");
        
        pendingWithdrawals[msg.sender][address(0)] = 0;
        payable(msg.sender).transfer(amount);
    }
    
    /**
     * @notice Withdraw platform fees
     */
    function withdrawPlatformFees() external onlyRole(DEFAULT_ADMIN_ROLE) {
        uint256 amount = platformFees;
        require(amount > 0, "No fees");
        
        platformFees = 0;
        payable(msg.sender).transfer(amount);
    }
    
    /**
     * @notice Get collection tokens
     * @param collection Collection address
     * @return Array of token IDs
     */
    function getCollectionTokens(address collection) external view returns (uint256[] memory) {
        return collectionTokens[collection];
    }
    
    /**
     * @notice Get NFT details
     * @param tokenId Token ID
     * @return NFT struct
     */
    function getNFT(uint256 tokenId) external view returns (NFT memory) {
        return nfts[tokenId];
    }
    
    // Required overrides
    function tokenURI(uint256 tokenId) public view override(ERC721URIStorage, ERC721) returns (string memory) {
        return super.tokenURI(tokenId);
    }
    
    function supportsInterface(bytes4 interfaceId) public view override(ERC721, AccessControl) returns (bool) {
        return super.supportsInterface(interfaceId);
    }
}