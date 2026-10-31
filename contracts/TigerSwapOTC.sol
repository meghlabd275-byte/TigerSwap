// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "./TigerSwapInterfaces.sol";

/**
 * @title TigerSwapOTC
 * @notice Settlement for large block trades negotiated off the order book.
 *
 * A size that would move an AMM pool by several percent cannot be traded on
 * curve without paying that impact. An OTC desk sidesteps the curve entirely:
 * two parties agree a price and a size, and this contract moves the assets when
 * both have signed off. Nothing is priced here — the contract is a settlement
 * layer, not a market.
 *
 * ## How a trade settles
 *
 * 1. The seller creates the trade. The `sellAmount` is pulled into the contract
 *    immediately, so the offer is funded and cannot fail on delivery later.
 * 2. The seller's signature over the trade terms is checked on creation. That
 *    stops a relayer from creating a trade the seller never agreed to.
 * 3. The buyer accepts before the deadline, paying exactly `buyAmount`. The
 *    seller's tokens and the buyer's payment are swapped in the same call.
 * 4. If nobody accepts, the seller cancels and the escrow comes straight back.
 *
 * The price is fixed at creation, so the buyer is protected from the seller
 * moving it after the fact and the seller is protected from a late fill at a
 * stale price because the deadline bounds how long the offer stands.
 *
 * ## Trust
 *
 * The contract holds the seller's tokens for the life of the offer. A trade is
 * a two-party agreement between addresses the participants choose; there is no
 * intermediary that can seize the escrow. Cancellation is always available to
 * the seller while the trade is open, and the deadline is enforced on the
 * contract's own clock rather than on anything the caller supplies.
 */
contract TigerSwapOTC {
    // ============ Types ============

    struct Trade {
        address seller;
        address buyer;          // zero until accepted
        address sellToken;
        address buyToken;
        uint256 sellAmount;
        uint256 buyAmount;
        uint256 deadline;
        bool active;
    }

    // ============ Storage ============

    address public owner;
    address public futureOwner;

    uint256 public nextTradeId = 1;
    uint256 public protocolFeeBps;   // taken from the seller's proceeds, in bps of 1e4
    address public feeRecipient;

    /// @dev The highest fee the owner may set. Capped so a compromised owner
    ///      cannot drain a trade outright.
    uint256 public constant MAX_FEE_BPS = 100; // 1%

    mapping(uint256 => Trade) public trades;

    /// @dev Signed terms are single-use. The digest is cleared on settlement or
    ///      cancellation so a captured signature cannot be replayed.
    mapping(bytes32 => bool) public usedDigests;

    /// @dev Tokens that may be escrowed. A trade in an unlisted token is
    ///      rejected, so the desk cannot be used to move arbitrary assets.
    mapping(address => bool) public supportedToken;

    // ============ Events ============

    event OwnershipTransferred(address indexed previousOwner, address indexed newOwner);
    event OwnershipTransferCommitted(address indexed currentOwner, address indexed pendingOwner);
    event TradeCreated(
        uint256 indexed tradeId,
        address indexed seller,
        address sellToken,
        address buyToken,
        uint256 sellAmount,
        uint256 buyAmount,
        uint256 deadline
    );
    event TradeAccepted(uint256 indexed tradeId, address indexed buyer, uint256 fee);
    event TradeCancelled(uint256 indexed tradeId, address indexed seller, uint256 refunded);
    event TokenSupportSet(address indexed token, bool supported);
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

    // ============ Construction ============

    uint256 private _reentrancyStatus = 1;

    constructor(address _feeRecipient) {
        require(_feeRecipient != address(0), "TigerSwap: ZERO_RECIPIENT");
        owner = msg.sender;
        feeRecipient = _feeRecipient;
        emit OwnershipTransferred(address(0), msg.sender);
    }

    // ============ Admin ============

    function setSupportedToken(address token, bool supported) external onlyOwner {
        require(token != address(0), "TigerSwap: ZERO_TOKEN");
        supportedToken[token] = supported;
        emit TokenSupportSet(token, supported);
    }

    function setFee(uint256 feeBps, address recipient) external onlyOwner {
        require(feeBps <= MAX_FEE_BPS, "TigerSwap: FEE_TOO_HIGH");
        require(recipient != address(0), "TigerSwap: ZERO_RECIPIENT");
        protocolFeeBps = feeBps;
        feeRecipient = recipient;
        emit FeeUpdated(feeBps, recipient);
    }

    function commitTransferOwnership(address newOwner) external onlyOwner {
        require(newOwner != address(0), "TigerSwap: ZERO_OWNER");
        futureOwner = newOwner;
        emit OwnershipTransferCommitted(owner, newOwner);
    }

    function acceptOwnership() external {
        require(msg.sender == futureOwner, "TigerSwap: NOT_FUTURE_OWNER");
        emit OwnershipTransferred(owner, futureOwner);
        owner = futureOwner;
        futureOwner = address(0);
    }

    // ============ Trade lifecycle ============

    /**
     * @notice Opens a funded block trade.
     * @param signature The seller's EIP-191 signature over the trade terms.
     *
     * The signature is verified against the terms exactly as supplied, so a
     * relayer cannot alter the price, size or deadline on the way in. The
     * digest is marked used before the transfer, which is what makes a captured
     * signature worthless the second time.
     */
    function createTrade(
        address sellToken,
        address buyToken,
        uint256 sellAmount,
        uint256 buyAmount,
        uint256 deadline,
        bytes calldata signature
    ) external nonReentrant returns (uint256 tradeId) {
        require(sellToken != address(0) && buyToken != address(0), "TigerSwap: ZERO_TOKEN");
        require(sellToken != buyToken, "TigerSwap: SAME_TOKEN");
        require(supportedToken[sellToken] && supportedToken[buyToken], "TigerSwap: UNSUPPORTED_TOKEN");
        require(sellAmount > 0 && buyAmount > 0, "TigerSwap: ZERO_AMOUNT");
        require(deadline > block.timestamp, "TigerSwap: DEADLINE_PASSED");

        bytes32 digest = _tradeDigest(
            msg.sender, sellToken, buyToken, sellAmount, buyAmount, deadline
        );
        require(!usedDigests[digest], "TigerSwap: DIGEST_USED");
        require(_recover(digest, signature) == msg.sender, "TigerSwap: BAD_SIGNATURE");

        usedDigests[digest] = true;

        tradeId = nextTradeId++;
        trades[tradeId] = Trade({
            seller: msg.sender,
            buyer: address(0),
            sellToken: sellToken,
            buyToken: buyToken,
            sellAmount: sellAmount,
            buyAmount: buyAmount,
            deadline: deadline,
            active: true
        });

        require(
            IERC20(sellToken).transferFrom(msg.sender, address(this), sellAmount),
            "TigerSwap: TRANSFER_IN"
        );

        emit TradeCreated(tradeId, msg.sender, sellToken, buyToken, sellAmount, buyAmount, deadline);
    }

    /**
     * @notice Accepts an open trade, paying `buyAmount` for the escrowed tokens.
     */
    function acceptTrade(uint256 tradeId) external nonReentrant returns (uint256 received) {
        Trade storage trade = trades[tradeId];
        require(trade.active, "TigerSwap: NOT_ACTIVE");
        require(block.timestamp <= trade.deadline, "TigerSwap: DEADLINE_PASSED");
        require(msg.sender != trade.seller, "TigerSwap: SELF_TRADE");

        // Clear state before moving any tokens, so a re-entrant call cannot see
        // a still-active trade.
        trade.active = false;
        trade.buyer = msg.sender;

        uint256 fee = (trade.buyAmount * protocolFeeBps) / 10_000;
        uint256 sellerProceeds = trade.buyAmount - fee;

        require(
            IERC20(trade.buyToken).transferFrom(msg.sender, address(this), trade.buyAmount),
            "TigerSwap: TRANSFER_IN"
        );

        if (fee > 0) {
            require(IERC20(trade.buyToken).transfer(feeRecipient, fee), "TigerSwap: FEE_TRANSFER");
        }
        require(
            IERC20(trade.buyToken).transfer(trade.seller, sellerProceeds),
            "TigerSwap: PAY_SELLER"
        );

        received = trade.sellAmount;
        require(IERC20(trade.sellToken).transfer(msg.sender, received), "TigerSwap: PAY_BUYER");

        emit TradeAccepted(tradeId, msg.sender, fee);
    }

    /**
     * @notice Returns an unaccepted trade's escrow to its seller.
     */
    function cancelTrade(uint256 tradeId) external nonReentrant returns (uint256 refunded) {
        Trade storage trade = trades[tradeId];
        require(trade.active, "TigerSwap: NOT_ACTIVE");
        require(msg.sender == trade.seller, "TigerSwap: NOT_SELLER");

        trade.active = false;
        refunded = trade.sellAmount;

        require(IERC20(trade.sellToken).transfer(trade.seller, refunded), "TigerSwap: REFUND");

        emit TradeCancelled(tradeId, trade.seller, refunded);
    }

    // ============ Views ============

    /// @notice The digest a seller signs to open a trade with these terms.
    function tradeDigest(
        address seller,
        address sellToken,
        address buyToken,
        uint256 sellAmount,
        uint256 buyAmount,
        uint256 deadline
    ) external view returns (bytes32) {
        return _tradeDigest(seller, sellToken, buyToken, sellAmount, buyAmount, deadline);
    }

    /// @notice True when a trade can still be accepted.
    function isTradeFillable(uint256 tradeId) external view returns (bool) {
        Trade storage trade = trades[tradeId];
        return trade.active && block.timestamp <= trade.deadline;
    }

    // ============ Internals ============

    /**
     * @dev Binds the digest to this contract and this chain, so a signature
     *      gathered for one deployment cannot be replayed against another.
     */
    function _tradeDigest(
        address seller,
        address sellToken,
        address buyToken,
        uint256 sellAmount,
        uint256 buyAmount,
        uint256 deadline
    ) internal view returns (bytes32) {
        return keccak256(
            abi.encode(
                address(this),
                block.chainid,
                seller,
                sellToken,
                buyToken,
                sellAmount,
                buyAmount,
                deadline
            )
        );
    }

    /**
     * @dev Recovers the signer of an EIP-191 personal-signature digest.
     *
     * `ecrecover` returns the zero address on a bad signature and can return a
     * high-s malleable signature that recovers to a different key, so both `v`
     * and `s` are range-checked first. Anything that does not recover cleanly
     * is rejected rather than silently treated as a valid signer.
     */
    function _recover(bytes32 digest, bytes calldata signature) internal pure returns (address) {
        require(signature.length == 65, "TigerSwap: BAD_SIGNATURE_LENGTH");

        bytes32 r;
        bytes32 s;
        uint8 v;
        // solhint-disable-next-line no-inline-assembly
        assembly {
            r := calldataload(signature.offset)
            s := calldataload(add(signature.offset, 32))
            v := byte(0, calldataload(add(signature.offset, 64)))
        }

        require(v == 27 || v == 28, "TigerSwap: BAD_SIGNATURE_V");
        // Reject the upper half of the curve order, the malleable range.
        require(
            uint256(s) <= 0x7FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF5D576E7357A4501DDFE92F46681B20A0,
            "TigerSwap: BAD_SIGNATURE_S"
        );

        bytes32 prefixed = keccak256(abi.encodePacked("\x19Ethereum Signed Message:\n32", digest));
        address signer = ecrecover(prefixed, v, r, s);
        require(signer != address(0), "TigerSwap: BAD_SIGNATURE");
        return signer;
    }
}
