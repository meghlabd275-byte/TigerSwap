// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/**
 * @title TigerSwapNFT
 * @notice A self-contained ERC-721. Collections on the marketplace are minted
 *         through this contract rather than through an external registry, so
 *         the marketplace has no dependency it does not control.
 *
 * Ownership, approvals and transfers follow ERC-721. `safeTransferFrom` checks
 * the recipient when it is a contract, which is what stops an NFT being sent to
 * a contract that cannot move it out again.
 */
contract TigerSwapNFT {
    string public name;
    string public symbol;

    address public owner;
    address public futureOwner;

    uint256 public totalSupply;

    mapping(uint256 => address) public ownerOf;
    mapping(address => uint256) public balanceOf;
    mapping(uint256 => address) public getApproved;
    mapping(address => mapping(address => bool)) public isApprovedForAll;
    mapping(uint256 => string) internal _tokenURIs;

    event Transfer(address indexed from, address indexed to, uint256 indexed tokenId);
    event Approval(address indexed owner, address indexed approved, uint256 indexed tokenId);
    event ApprovalForAll(address indexed owner, address indexed operator, bool approved);
    event OwnershipTransferred(address indexed previousOwner, address indexed newOwner);

    modifier onlyOwner() {
        require(msg.sender == owner, "TigerSwap: NOT_OWNER");
        _;
    }

    constructor(string memory _name, string memory _symbol) {
        name = _name;
        symbol = _symbol;
        owner = msg.sender;
        emit OwnershipTransferred(address(0), msg.sender);
    }

    /// @notice Mints a new token with metadata to `to`.
    function mint(address to, string calldata uri) external onlyOwner returns (uint256 tokenId) {
        require(to != address(0), "TigerSwap: ZERO_RECIPIENT");
        tokenId = ++totalSupply;
        ownerOf[tokenId] = to;
        balanceOf[to] += 1;
        _tokenURIs[tokenId] = uri;
        emit Transfer(address(0), to, tokenId);
    }

    function tokenURI(uint256 tokenId) external view returns (string memory) {
        require(ownerOf[tokenId] != address(0), "TigerSwap: NO_TOKEN");
        return _tokenURIs[tokenId];
    }

    function approve(address to, uint256 tokenId) external {
        address holder = ownerOf[tokenId];
        require(msg.sender == holder || isApprovedForAll[holder][msg.sender], "TigerSwap: NOT_OWNER");
        getApproved[tokenId] = to;
        emit Approval(holder, to, tokenId);
    }

    function setApprovalForAll(address operator, bool approved) external {
        require(operator != msg.sender, "TigerSwap: SELF_APPROVAL");
        isApprovedForAll[msg.sender][operator] = approved;
        emit ApprovalForAll(msg.sender, operator, approved);
    }

    function transferFrom(address from, address to, uint256 tokenId) public {
        _transfer(from, to, tokenId);
    }

    function safeTransferFrom(address from, address to, uint256 tokenId) external {
        safeTransferFrom(from, to, tokenId, "");
    }

    function safeTransferFrom(address from, address to, uint256 tokenId, bytes memory data) public {
        _transfer(from, to, tokenId);
        if (to.code.length > 0) {
            require(
                IERC721Receiver(to).onERC721Received(msg.sender, from, tokenId, data) ==
                    IERC721Receiver.onERC721Received.selector,
                "TigerSwap: UNSAFE_RECIPIENT"
            );
        }
    }

    function commitTransferOwnership(address newOwner) external onlyOwner {
        require(newOwner != address(0), "TigerSwap: ZERO_OWNER");
        futureOwner = newOwner;
    }

    function acceptOwnership() external {
        require(msg.sender == futureOwner, "TigerSwap: NOT_FUTURE_OWNER");
        emit OwnershipTransferred(owner, futureOwner);
        owner = futureOwner;
        futureOwner = address(0);
    }

    function _transfer(address from, address to, uint256 tokenId) internal {
        require(ownerOf[tokenId] == from, "TigerSwap: NOT_OWNER");
        require(to != address(0), "TigerSwap: ZERO_RECIPIENT");
        require(
            msg.sender == from || isApprovedForAll[from][msg.sender] || getApproved[tokenId] == msg.sender,
            "TigerSwap: NOT_APPROVED"
        );

        // Clear the single-token approval on transfer, so a stale approval
        // cannot be used against the next holder.
        delete getApproved[tokenId];

        balanceOf[from] -= 1;
        balanceOf[to] += 1;
        ownerOf[tokenId] = to;

        emit Transfer(from, to, tokenId);
    }
}

interface IERC721Receiver {
    function onERC721Received(address operator, address from, uint256 tokenId, bytes calldata data)
        external
        returns (bytes4);
}

interface IERC721 {
    function ownerOf(uint256 tokenId) external view returns (address);
    function transferFrom(address from, address to, uint256 tokenId) external;
    function isApprovedForAll(address owner, address operator) external view returns (bool);
}

/**
 * @title TigerSwapNFTMarketplace
 * @notice Trading for the NFTs minted by {TigerSwapNFT}, priced in any listed
 *         ERC-20 (RUSD, USDT, USDC, TGR, ...).
 *
 * ## Escrow, not approvals
 *
 * A listing moves the NFT into this contract. The alternative — holding only an
 * approval and pulling the token at sale time — leaves the seller free to
 * revoke or sell elsewhere between listing and fill, so the buyer's purchase
 * can revert on a listing that still looks live. Escrow removes that race: a
 * live listing is a listing that can be filled, and cancelling always returns
 * the token.
 *
 * Offers work the same way in the other direction, with the payment token held
 * in escrow so an accepted offer cannot fail for lack of funds.
 *
 * ## Payment tokens
 *
 * Transfers go through {_safeTransfer}, which tolerates tokens that return no
 * value (USDT among them) as well as those that return `true`, and rejects a
 * `false` return. A plain `IERC20.transfer` call would revert against the
 * former.
 */
contract TigerSwapNFTMarketplace {
    // ============ Types ============

    struct Listing {
        address seller;
        address nft;
        uint256 tokenId;
        address paymentToken;
        uint256 price;
        uint256 deadline;
        bool active;
    }

    struct Offer {
        address buyer;
        address paymentToken;
        uint256 amount;
        uint256 deadline;
        bool active;
    }

    // ============ Storage ============

    address public owner;
    address public futureOwner;
    address public feeRecipient;

    /// @dev Marketplace fee in basis points of the sale price.
    uint256 public feeBps;

    /// @dev Ceiling on the fee, so a compromised owner cannot take an
    ///      arbitrary share of a sale.
    uint256 public constant MAX_FEE_BPS = 500; // 5%

    uint256 public nextListingId = 1;
    uint256 public nextOfferId = 1;

    mapping(uint256 => Listing) public listings;
    mapping(uint256 => Offer) public offers;

    /// @dev Highest live offer per (nft, tokenId, paymentToken). Kept so an
    ///      offer can be accepted in one call without the seller scanning the
    ///      whole offer book.
    mapping(address => mapping(uint256 => mapping(address => uint256))) public bestOfferId;

    mapping(address => bool) public supportedPaymentToken;

    uint256 private _reentrancyStatus = 1;

    // ============ Events ============

    event OwnershipTransferred(address indexed previousOwner, address indexed newOwner);
    event Listed(
        uint256 indexed listingId,
        address indexed seller,
        address indexed nft,
        uint256 tokenId,
        address paymentToken,
        uint256 price,
        uint256 deadline
    );
    event Sold(
        uint256 indexed listingId,
        address indexed buyer,
        address indexed seller,
        uint256 price,
        uint256 fee
    );
    event ListingCancelled(uint256 indexed listingId, address indexed seller);
    event OfferMade(
        uint256 indexed offerId,
        address indexed buyer,
        address indexed nft,
        uint256 tokenId,
        address paymentToken,
        uint256 amount,
        uint256 deadline
    );
    event OfferAccepted(uint256 indexed offerId, address indexed seller, uint256 amount, uint256 fee);
    event OfferCancelled(uint256 indexed offerId, address indexed buyer, uint256 refunded);
    event PaymentTokenSet(address indexed token, bool supported);
    event FeeUpdated(uint256 feeBps, address feeRecipient);

    // ============ Modifiers ============

    modifier onlyOwner() {
        require(msg.sender == owner, "TigerSwap: NOT_OWNER");
        _;
    }

    modifier nonReentrant() {
        require(_reentrancyStatus != 2, "TigerSwap: REENTRANT");
        _reentrancyStatus = 2;
        _;
        _reentrancyStatus = 1;
    }

    constructor(address _feeRecipient) {
        require(_feeRecipient != address(0), "TigerSwap: ZERO_RECIPIENT");
        owner = msg.sender;
        feeRecipient = _feeRecipient;
        emit OwnershipTransferred(address(0), msg.sender);
    }

    // ============ Admin ============

    function setPaymentToken(address token, bool supported) external onlyOwner {
        require(token != address(0), "TigerSwap: ZERO_TOKEN");
        supportedPaymentToken[token] = supported;
        emit PaymentTokenSet(token, supported);
    }

    function setFee(uint256 newFeeBps, address recipient) external onlyOwner {
        require(newFeeBps <= MAX_FEE_BPS, "TigerSwap: FEE_TOO_HIGH");
        require(recipient != address(0), "TigerSwap: ZERO_RECIPIENT");
        feeBps = newFeeBps;
        feeRecipient = recipient;
        emit FeeUpdated(newFeeBps, recipient);
    }

    function commitTransferOwnership(address newOwner) external onlyOwner {
        require(newOwner != address(0), "TigerSwap: ZERO_OWNER");
        futureOwner = newOwner;
    }

    function acceptOwnership() external {
        require(msg.sender == futureOwner, "TigerSwap: NOT_FUTURE_OWNER");
        emit OwnershipTransferred(owner, futureOwner);
        owner = futureOwner;
        futureOwner = address(0);
    }

    // ============ Listings ============

    /**
     * @notice Lists an NFT for sale, taking it into escrow.
     *
     * The caller must have approved this contract for the token first; the
     * transfer below fails otherwise, which is the intended check that the
     * seller really controls the NFT.
     */
    function list(
        address nft,
        uint256 tokenId,
        address paymentToken,
        uint256 price,
        uint256 deadline
    ) external nonReentrant returns (uint256 listingId) {
        require(nft != address(0), "TigerSwap: ZERO_NFT");
        require(supportedPaymentToken[paymentToken], "TigerSwap: UNSUPPORTED_TOKEN");
        require(price > 0, "TigerSwap: ZERO_PRICE");
        require(deadline > block.timestamp, "TigerSwap: DEADLINE_PASSED");

        IERC721(nft).transferFrom(msg.sender, address(this), tokenId);

        listingId = nextListingId++;
        listings[listingId] = Listing({
            seller: msg.sender,
            nft: nft,
            tokenId: tokenId,
            paymentToken: paymentToken,
            price: price,
            deadline: deadline,
            active: true
        });

        emit Listed(listingId, msg.sender, nft, tokenId, paymentToken, price, deadline);
    }

    /**
     * @notice Buys a listed NFT at its asking price.
     */
    function buy(uint256 listingId) external nonReentrant {
        Listing storage listing = listings[listingId];
        require(listing.active, "TigerSwap: NOT_ACTIVE");
        require(block.timestamp <= listing.deadline, "TigerSwap: DEADLINE_PASSED");
        require(msg.sender != listing.seller, "TigerSwap: SELF_TRADE");

        // Mark inactive before any external call, so a token callback cannot
        // re-enter and buy the same listing twice.
        listing.active = false;

        _settleSale(listing, msg.sender);

        emit Sold(listingId, msg.sender, listing.seller, listing.price, (listing.price * feeBps) / 10_000);
    }

    function cancelListing(uint256 listingId) external nonReentrant {
        Listing storage listing = listings[listingId];
        require(listing.active, "TigerSwap: NOT_ACTIVE");
        require(msg.sender == listing.seller, "TigerSwap: NOT_SELLER");

        listing.active = false;

        // An expired listing belongs to the seller too; cancelling is how they
        // recover it either way.
        IERC721(listing.nft).transferFrom(address(this), listing.seller, listing.tokenId);

        emit ListingCancelled(listingId, listing.seller);
    }

    /**
     * @notice Updates the asking price of a live listing.
     */
    function updateListingPrice(uint256 listingId, uint256 newPrice) external {
        Listing storage listing = listings[listingId];
        require(listing.active, "TigerSwap: NOT_ACTIVE");
        require(msg.sender == listing.seller, "TigerSwap: NOT_SELLER");
        require(newPrice > 0, "TigerSwap: ZERO_PRICE");
        listing.price = newPrice;
    }

    // ============ Offers ============

    /**
     * @notice Makes an offer on a specific NFT, escrowing the payment.
     *
     * Offers are keyed by (nft, tokenId, paymentToken) and only the highest
     * live one is tracked, so the owner of the NFT can accept the best standing
     * offer without walking the book.
     */
    function makeOffer(
        address nft,
        uint256 tokenId,
        address paymentToken,
        uint256 amount,
        uint256 deadline
    ) external nonReentrant returns (uint256 offerId) {
        require(nft != address(0), "TigerSwap: ZERO_NFT");
        require(supportedPaymentToken[paymentToken], "TigerSwap: UNSUPPORTED_TOKEN");
        require(amount > 0, "TigerSwap: ZERO_AMOUNT");
        require(deadline > block.timestamp, "TigerSwap: DEADLINE_PASSED");

        // An offer on a token that does not exist can never be accepted, so it
        // is rejected rather than locking the buyer's funds.
        require(IERC721(nft).ownerOf(tokenId) != address(0), "TigerSwap: NO_TOKEN");

        offerId = nextOfferId++;
        offers[offerId] = Offer({
            buyer: msg.sender,
            paymentToken: paymentToken,
            amount: amount,
            deadline: deadline,
            active: true
        });

        _safeTransferFrom(paymentToken, msg.sender, address(this), amount);

        uint256 currentBest = bestOfferId[nft][tokenId][paymentToken];
        if (currentBest == 0 || amount > offers[currentBest].amount) {
            bestOfferId[nft][tokenId][paymentToken] = offerId;
        }

        emit OfferMade(offerId, msg.sender, nft, tokenId, paymentToken, amount, deadline);
    }

    /**
     * @notice Accepts a standing offer on an NFT the caller owns.
     *
     * The seller does not need to have listed the NFT; accepting an offer
     * transfers it directly to the offerer. An NFT that is currently escrowed
     * by a listing is held by this contract, so the seller cannot accept an
     * offer on it until they cancel the listing.
     */
    function acceptOffer(uint256 offerId, address nft, uint256 tokenId) external nonReentrant {
        Offer storage offer = offers[offerId];
        require(offer.active, "TigerSwap: NOT_ACTIVE");
        require(block.timestamp <= offer.deadline, "TigerSwap: DEADLINE_PASSED");
        require(IERC721(nft).ownerOf(tokenId) == msg.sender, "TigerSwap: NOT_OWNER");

        offer.active = false;
        if (bestOfferId[nft][tokenId][offer.paymentToken] == offerId) {
            delete bestOfferId[nft][tokenId][offer.paymentToken];
        }

        uint256 fee = (offer.amount * feeBps) / 10_000;
        uint256 sellerProceeds = offer.amount - fee;

        IERC721(nft).transferFrom(msg.sender, offer.buyer, tokenId);

        if (fee > 0) {
            _safeTransfer(offer.paymentToken, feeRecipient, fee);
        }
        _safeTransfer(offer.paymentToken, msg.sender, sellerProceeds);

        emit OfferAccepted(offerId, msg.sender, offer.amount, fee);
    }

    function cancelOffer(uint256 offerId) external nonReentrant {
        Offer storage offer = offers[offerId];
        require(offer.active, "TigerSwap: NOT_ACTIVE");
        require(msg.sender == offer.buyer, "TigerSwap: NOT_BUYER");

        offer.active = false;

        uint256 refund = offer.amount;
        _safeTransfer(offer.paymentToken, offer.buyer, refund);

        emit OfferCancelled(offerId, offer.buyer, refund);
    }

    // ============ Views ============

    function isListingFillable(uint256 listingId) external view returns (bool) {
        Listing storage listing = listings[listingId];
        return listing.active && block.timestamp <= listing.deadline;
    }

    function isOfferAcceptable(uint256 offerId) external view returns (bool) {
        Offer storage offer = offers[offerId];
        return offer.active && block.timestamp <= offer.deadline;
    }

    // ============ Internals ============

    /**
     * @dev Moves payment from the buyer, pays the seller net of fee, and hands
     *      over the escrowed NFT. State was already cleared by the caller.
     */
    function _settleSale(Listing storage listing, address buyer) internal {
        uint256 fee = (listing.price * feeBps) / 10_000;
        uint256 sellerProceeds = listing.price - fee;

        _safeTransferFrom(listing.paymentToken, buyer, address(this), listing.price);

        if (fee > 0) {
            _safeTransfer(listing.paymentToken, feeRecipient, fee);
        }
        _safeTransfer(listing.paymentToken, listing.seller, sellerProceeds);

        IERC721(listing.nft).transferFrom(address(this), buyer, listing.tokenId);
    }

    /// @dev ERC-20 transfer that rejects a `false` return.
    function _safeTransfer(address token, address to, uint256 amount) internal {
        (bool ok, bytes memory data) = token.call(
            abi.encodeWithSelector(bytes4(keccak256("transfer(address,uint256)")), to, amount)
        );
        require(ok && (data.length == 0 || abi.decode(data, (bool))), "TigerSwap: TRANSFER_FAILED");
    }

    /// @dev ERC-20 transferFrom that rejects a `false` return.
    function _safeTransferFrom(address token, address from, address to, uint256 amount) internal {
        (bool ok, bytes memory data) = token.call(
            abi.encodeWithSelector(bytes4(keccak256("transferFrom(address,address,uint256)")), from, to, amount)
        );
        require(ok && (data.length == 0 || abi.decode(data, (bool))), "TigerSwap: TRANSFER_FAILED");
    }
}
