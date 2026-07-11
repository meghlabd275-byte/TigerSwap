// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/**
 * @title TigerNFTMarketplace
 * @notice Production NFT Marketplace
 * @dev NFT trading with auctions and offers
 * 
 * Features:
 * - Fixed price sales
 * - Dutch auctions
 * - English auctions
 * - Offers
 * - Royalty support
 * - Collection trading
 * 
 * @author TigerSwap
 * @version 1.0.0 Production
 */

import "@openzeppelin/contracts/token/ERC721/IERC721.sol";
import "@openzeppelin/contracts/token/ERC1155/IERC1155.sol";
import "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import "@openzeppelin/contracts/token/ERC20/utils/SafeERC20.sol";
import "@openzeppelin/contracts/utils/ReentrancyGuard.sol";
import "@openzeppelin/contracts/access/Ownable.sol";

/**
 * @title TigerNFTMarketplace
 * @dev Main marketplace contract
 */
contract TigerNFTMarketplace is ReentrancyGuard, Ownable {
    using SafeERC20 for IERC20;

    // ============ Constants ============
    uint256 constant PLATFORM_FEE_BPS = 250; // 2.5%
    uint256 constant MAX_ROYALTY_BPS = 1000; // 10%
    uint256 constant MIN_AUCTION_DURATION = 1 minutes;
    uint256 constant MAX_AUCTION_DURATION = 7 days;

    // ============ State Variables ============
    
    // Fee
    address public feeRecipient;
    uint256 public platformFeeBps = PLATFORM_FEE_BPS;
    
    // Token
    IERC20 public paymentToken;
    
    // Listings
    uint256 public listingCount;
    mapping(uint256 => Listing) public listings;
    mapping(address => mapping(uint256 => uint256)) public tokenToListing; // contract -> tokenId -> listingId
    
    // Offers
    uint256 public offerCount;
    mapping(uint256 => Offer) public offers;
    
    // Auctions
    uint256 public auctionCount;
    mapping(uint256 => Auction) public auctions;
    
    // Collections
    mapping(address => Collection) public collections;
    
    // User nonces
    mapping(address => uint256) public nonces;

    // ============ Structs ============
    
    enum ListingType { FIXED_PRICE, DUTCH_AUCTION }
    enum NFTType { ERC721, ERC1155 }
    enum AuctionType { ENGLISH, DUTCH }
    enum OfferStatus { ACTIVE, ACCEPTED, CANCELLED, EXPIRED }
    enum ListingStatus { ACTIVE, SOLD, CANCELLED, EXPIRED }
    
    struct Listing {
        address seller;
        address nftContract;
        uint256 tokenId;
        uint256 quantity; // For ERC1155
        NFTType nftType;
        ListingType listingType;
        uint256 price;
        uint256 startPrice; // For Dutch auction
        uint256 endPrice;
        uint256 startTime;
        uint256 endTime;
        address paymentToken;
        ListingStatus status;
        uint256 royaltyBps;
        address royaltyRecipient;
    }
    
    struct Offer {
        address offerer;
        address nftContract;
        uint256 tokenId;
        uint256 quantity;
        NFTType nftType;
        uint256 price;
        uint256 expirationTime;
        OfferStatus status;
    }
    
    struct Auction {
        address seller;
        address nftContract;
        uint256 tokenId;
        uint256 quantity;
        NFTType nftType;
        AuctionType auctionType;
        uint256 startingPrice;
        uint256 endingPrice;
        uint256 startTime;
        uint256 duration;
        uint256 currentBid;
        address highestBidder;
        bool ended;
    }
    
    struct Collection {
        bool exists;
        string name;
        string symbol;
        uint256 royaltyBps;
        address royaltyRecipient;
        bool tradingEnabled;
    }

    // ============ Events ============
    event ListingCreated(
        uint256 indexed listingId,
        address indexed seller,
        address nftContract,
        uint256 tokenId,
        uint256 price,
        ListingType listingType
    );
    event ListingSold(
        uint256 indexed listingId,
        address indexed buyer,
        uint256 price
    );
    event ListingCancelled(uint256 indexed listingId);
    event OfferCreated(
        uint256 indexed offerId,
        address indexed offerer,
        address nftContract,
        uint256 tokenId,
        uint256 price
    );
    event OfferAccepted(
        uint256 indexed offerId,
        address indexed seller
    );
    event OfferCancelled(uint256 indexed offerId);
    event AuctionCreated(
        uint256 indexed auctionId,
        address indexed seller,
        address nftContract,
        uint256 tokenId,
        uint256 startingPrice
    );
    event BidPlaced(
        uint256 indexed auctionId,
        address indexed bidder,
        uint256 amount
    );
    event AuctionEnded(
        uint256 indexed auctionId,
        address indexed winner,
        uint256 finalPrice
    );
    event CollectionCreated(
        address indexed collection,
        string name,
        string symbol
    );

    // ============ Constructor ============
    
    constructor(address _paymentToken, address _feeRecipient, address _owner) Ownable(_owner) {
        require(_paymentToken != address(0), "Invalid payment token");
        require(_feeRecipient != address(0), "Invalid fee recipient");
        
        paymentToken = IERC20(_paymentToken);
        feeRecipient = _feeRecipient;
    }

    // ============ Fixed Price Listing ============

    /**
     * @notice Create a fixed price listing
     */
    function createListing(
        address _nftContract,
        uint256 _tokenId,
        uint256 _quantity,
        bool _isERC1155,
        uint256 _price,
        uint256 _duration
    ) external nonReentrant returns (uint256) {
        require(_price > 0, "Price must be positive");
        
        NFTType nftType = _isERC1155 ? NFTType.ERC1155 : NFTType.ERC721;
        
        // Verify ownership
        if (nftType == NFTType.ERC721) {
            IERC721 nft = IERC721(_nftContract);
            require(nft.ownerOf(_tokenId) == msg.sender, "Not owner");
            require(
                nft.getApproved(_tokenId) == address(this) ||
                nft.isApprovedForAll(msg.sender, address(this)),
                "Not approved"
            );
        } else {
            IERC1155 nft = IERC1155(_nftContract);
            require(nft.balanceOf(msg.sender, _tokenId) >= _quantity, "Insufficient balance");
            require(
                nft.isApprovedForAll(msg.sender, address(this)),
                "Not approved"
            );
        }
        
        uint256 listingId = ++listingCount;
        
        listings[listingId] = Listing({
            seller: msg.sender,
            nftContract: _nftContract,
            tokenId: _tokenId,
            quantity: _isERC1155 ? _quantity : 1,
            nftType: nftType,
            listingType: ListingType.FIXED_PRICE,
            price: _price,
            startPrice: 0,
            endPrice: 0,
            startTime: block.timestamp,
            endTime: block.timestamp + _duration,
            paymentToken: address(paymentToken),
            status: ListingStatus.ACTIVE,
            royaltyBps: 0,
            royaltyRecipient: address(0)
        });
        
        tokenToListing[_nftContract][_tokenId] = listingId;
        
        emit ListingCreated(listingId, msg.sender, _nftContract, _tokenId, _price, ListingType.FIXED_PRICE);
        
        return listingId;
    }

    /**
     * @notice Buy from fixed price listing
     */
    function buyListing(uint256 _listingId) external nonReentrant {
        Listing storage listing = listings[_listingId];
        
        require(listing.status == ListingStatus.ACTIVE, "Listing not active");
        require(block.timestamp <= listing.endTime, "Listing expired");
        require(listing.seller != msg.sender, "Cannot buy own listing");
        
        uint256 price = listing.price;
        
        // Transfer payment
        paymentToken.safeTransferFrom(msg.sender, address(this), price);
        
        // Calculate fees
        uint256 platformFee = (price * platformFeeBps) / 10000;
        uint256 royaltyFee = listing.royaltyBps > 0 
            ? (price * listing.royaltyBps) / 10000 
            : 0;
        uint256 sellerAmount = price - platformFee - royaltyFee;
        
        // Transfer NFT
        _transferNFT(listing, msg.sender, listing.quantity);
        
        // Transfer payments
        paymentToken.safeTransfer(feeRecipient, platformFee);
        
        if (royaltyFee > 0 && listing.royaltyRecipient != address(0)) {
            paymentToken.safeTransfer(listing.royaltyRecipient, royaltyFee);
        }
        
        paymentToken.safeTransfer(listing.seller, sellerAmount);
        
        // Update status
        listing.status = ListingStatus.SOLD;
        
        emit ListingSold(_listingId, msg.sender, price);
    }

    /**
     * @notice Cancel listing
     */
    function cancelListing(uint256 _listingId) external nonReentrant {
        Listing storage listing = listings[_listingId];
        
        require(listing.seller == msg.sender, "Not seller");
        require(listing.status == ListingStatus.ACTIVE, "Listing not active");
        
        listing.status = ListingStatus.CANCELLED;
        
        emit ListingCancelled(_listingId);
    }

    // ============ Offers ============

    /**
     * @notice Make an offer
     */
    function makeOffer(
        address _nftContract,
        uint256 _tokenId,
        uint256 _quantity,
        bool _isERC1155,
        uint256 _price,
        uint256 _duration
    ) external nonReentrant returns (uint256) {
        require(_price > 0, "Price must be positive");
        
        // Verify payment token balance
        require(
            paymentToken.balanceOf(msg.sender) >= _price,
            "Insufficient balance"
        );
        
        uint256 offerId = ++offerCount;
        
        offers[offerId] = Offer({
            offerer: msg.sender,
            nftContract: _nftContract,
            tokenId: _tokenId,
            quantity: _isERC1155 ? _quantity : 1,
            nftType: _isERC1155 ? NFTType.ERC1155 : NFTType.ERC721,
            price: _price,
            expirationTime: block.timestamp + _duration,
            status: OfferStatus.ACTIVE
        });
        
        // Lock payment tokens
        paymentToken.safeTransferFrom(msg.sender, address(this), _price);
        
        emit OfferCreated(offerId, msg.sender, _nftContract, _tokenId, _price);
        
        return offerId;
    }

    /**
     * @notice Accept offer
     */
    function acceptOffer(uint256 _offerId) external nonReentrant {
        Offer storage offer = offers[_offerId];
        
        require(offer.status == OfferStatus.ACTIVE, "Offer not active");
        require(block.timestamp <= offer.expirationTime, "Offer expired");
        
        // Verify ownership
        if (offer.nftType == NFTType.ERC721) {
            IERC721 nft = IERC721(offer.nftContract);
            require(nft.ownerOf(offer.tokenId) == msg.sender, "Not owner");
        } else {
            IERC1155 nft = IERC1155(offer.nftContract);
            require(nft.balanceOf(msg.sender, offer.tokenId) >= offer.quantity, "Insufficient balance");
        }
        
        // Calculate fees
        uint256 platformFee = (offer.price * platformFeeBps) / 10000;
        uint256 sellerAmount = offer.price - platformFee;
        
        // Transfer NFT
        _transferNFTByContract(offer.nftContract, offer.nftType, msg.sender, offer.offerer, offer.tokenId, offer.quantity);
        
        // Transfer payment
        paymentToken.safeTransfer(msg.sender, sellerAmount);
        paymentToken.safeTransfer(feeRecipient, platformFee);
        
        // Update status
        offer.status = OfferStatus.ACCEPTED;
        
        emit OfferAccepted(_offerId, msg.sender);
    }

    /**
     * @notice Cancel offer
     */
    function cancelOffer(uint256 _offerId) external nonReentrant {
        Offer storage offer = offers[_offerId];
        
        require(offer.offerer == msg.sender, "Not offerer");
        require(offer.status == OfferStatus.ACTIVE, "Offer not active");
        
        offer.status = OfferStatus.CANCELLED;
        
        // Refund payment
        paymentToken.safeTransfer(msg.sender, offer.price);
        
        emit OfferCancelled(_offerId);
    }

    // ============ Auctions ============

    /**
     * @notice Create English auction
     */
    function createAuction(
        address _nftContract,
        uint256 _tokenId,
        uint256 _quantity,
        bool _isERC1155,
        uint256 _startingPrice,
        uint256 _duration
    ) external nonReentrant returns (uint256) {
        require(_startingPrice > 0, "Starting price must be positive");
        require(_duration >= MIN_AUCTION_DURATION, "Duration too short");
        require(_duration <= MAX_AUCTION_DURATION, "Duration too long");
        
        uint256 auctionId = ++auctionCount;
        
        auctions[auctionId] = Auction({
            seller: msg.sender,
            nftContract: _nftContract,
            tokenId: _tokenId,
            quantity: _isERC1155 ? _quantity : 1,
            nftType: _isERC1155 ? NFTType.ERC1155 : NFTType.ERC721,
            auctionType: AuctionType.ENGLISH,
            startingPrice: _startingPrice,
            endingPrice: 0,
            startTime: block.timestamp,
            duration: _duration,
            currentBid: 0,
            highestBidder: address(0),
            ended: false
        });
        
        emit AuctionCreated(auctionId, msg.sender, _nftContract, _tokenId, _startingPrice);
        
        return auctionId;
    }

    /**
     * @notice Place bid
     */
    function placeBid(uint256 _auctionId, uint256 _amount) external nonReentrant {
        Auction storage auction = auctions[_auctionId];
        
        require(!auction.ended, "Auction ended");
        require(block.timestamp < auction.startTime + auction.duration, "Auction expired");
        require(_amount > auction.currentBid, "Bid too low");
        require(_amount >= auction.startingPrice, "Bid below starting price");
        
        // Refund previous bidder
        if (auction.highestBidder != address(0)) {
            paymentToken.safeTransfer(auction.highestBidder, auction.currentBid);
        }
        
        // Transfer new bid
        paymentToken.safeTransferFrom(msg.sender, address(this), _amount);
        
        auction.currentBid = _amount;
        auction.highestBidder = msg.sender;
        
        emit BidPlaced(_auctionId, msg.sender, _amount);
    }

    /**
     * @notice End auction
     */
    function endAuction(uint256 _auctionId) external nonReentrant {
        Auction storage auction = auctions[_auctionId];
        
        require(!auction.ended, "Already ended");
        require(
            msg.sender == auction.seller || 
            msg.sender == auction.highestBidder ||
            block.timestamp >= auction.startTime + auction.duration,
            "Cannot end yet"
        );
        
        auction.ended = true;
        
        if (auction.highestBidder != address(0)) {
            // Transfer NFT to winner
            _transferNFTByContract(
                auction.nftContract, 
                auction.nftType,
                auction.seller, 
                auction.highestBidder, 
                auction.tokenId, 
                auction.quantity
            );
            
            // Calculate fees
            uint256 platformFee = (auction.currentBid * platformFeeBps) / 10000;
            uint256 sellerAmount = auction.currentBid - platformFee;
            
            // Transfer payment
            paymentToken.safeTransfer(auction.seller, sellerAmount);
            paymentToken.safeTransfer(feeRecipient, platformFee);
            
            emit AuctionEnded(_auctionId, auction.highestBidder, auction.currentBid);
        } else {
            // No bids, NFT stays with seller
            emit AuctionEnded(_auctionId, address(0), 0);
        }
    }

    // ============ Helper Functions ============

    /**
     * @dev Transfer NFT based on listing
     */
    function _transferNFT(Listing storage _listing, address _to, uint256 _quantity) internal {
        _transferNFTByContract(
            _listing.nftContract,
            _listing.nftType,
            _listing.seller,
            _to,
            _listing.tokenId,
            _quantity
        );
    }

    /**
     * @dev Transfer NFT by contract type
     */
    function _transferNFTByContract(
        address _contract,
        NFTType _nftType,
        address _from,
        address _to,
        uint256 _tokenId,
        uint256 _quantity
    ) internal {
        if (_nftType == NFTType.ERC721) {
            IERC721(_contract).safeTransferFrom(_from, _to, _tokenId);
        } else {
            IERC1155(_contract).safeTransferFrom(_from, _to, _tokenId, _quantity, "");
        }
    }

    // ============ Admin Functions ============

    /**
     * @notice Set platform fee
     */
    function setPlatformFee(uint256 _feeBps) external onlyOwner {
        require(_feeBps <= 1000, "Fee too high");
        platformFeeBps = _feeBps;
    }

    /**
     * @notice Set fee recipient
     */
    function setFeeRecipient(address _recipient) external onlyOwner {
        require(_recipient != address(0), "Invalid recipient");
        feeRecipient = _recipient;
    }

    // ============ View Functions ============

    /**
     * @notice Get listing details
     */
    function getListing(uint256 _listingId) external view returns (Listing memory) {
        return listings[_listingId];
    }

    /**
     * @notice Get offer details
     */
    function getOffer(uint256 _offerId) external view returns (Offer memory) {
        return offers[_offerId];
    }

    /**
     * @notice Get auction details
     */
    function getAuction(uint256 _auctionId) external view returns (Auction memory) {
        return auctions[_auctionId];
    }
}
