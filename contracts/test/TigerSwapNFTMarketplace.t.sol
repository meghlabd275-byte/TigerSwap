// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "forge-std/Test.sol";
import "../TigerSwapNFTMarketplace.sol";
import "./TestERC20.sol";

/// @notice ERC-20 that returns nothing from transfer/transferFrom, like USDT.
///         The marketplace must tolerate this rather than revert.
contract NoReturnERC20 {
    mapping(address => uint256) public balanceOf;
    mapping(address => mapping(address => uint256)) public allowance;

    function mint(address to, uint256 amount) external {
        balanceOf[to] += amount;
    }

    function approve(address spender, uint256 amount) external {
        allowance[msg.sender][spender] = amount;
    }

    function transfer(address to, uint256 amount) external {
        require(balanceOf[msg.sender] >= amount, "NoReturn: BALANCE");
        balanceOf[msg.sender] -= amount;
        balanceOf[to] += amount;
    }

    function transferFrom(address from, address to, uint256 amount) external {
        uint256 allowed = allowance[from][msg.sender];
        if (allowed != type(uint256).max) {
            require(allowed >= amount, "NoReturn: ALLOWANCE");
            allowance[from][msg.sender] = allowed - amount;
        }
        require(balanceOf[from] >= amount, "NoReturn: BALANCE");
        balanceOf[from] -= amount;
        balanceOf[to] += amount;
    }
}

/// @notice ERC-721 receiver used to check that safe transfers into contracts
///         are verified rather than assumed.
contract Receiver is IERC721Receiver {
    bool public accept = true;

    function setAccept(bool value) external {
        accept = value;
    }

    function onERC721Received(address, address, uint256, bytes calldata) external view returns (bytes4) {
        if (!accept) {
            return 0xdeadbeef;
        }
        return IERC721Receiver.onERC721Received.selector;
    }
}

/// @notice A contract with no receiver hook, used to confirm the safe-transfer
///         check actually rejects a non-receiver.
contract NonReceiver {
    // Intentionally empty: has code, but no onERC721Received.
}

contract TigerSwapNFTMarketplaceTest is Test {
    TigerSwapNFT public nft;
    TigerSwapNFTMarketplace public market;
    TestERC20 public payToken;

    address public seller = address(0x11);
    address public buyer = address(0x22);
    address public feeRecipient = address(0x44);

    uint256 constant PRICE = 1_000e18;
    uint256 constant TOKEN_ID = 1;

    function setUp() public {
        nft = new TigerSwapNFT("Tiger Collection", "TIGER");
        market = new TigerSwapNFTMarketplace(feeRecipient);
        payToken = new TestERC20("RUSD", "RUSD");

        market.setPaymentToken(address(payToken), true);

        nft.mint(seller, "ipfs://token-1");
        payToken.mint(buyer, 1_000_000e18);

        vm.prank(seller);
        nft.setApprovalForAll(address(market), true);
        vm.prank(buyer);
        payToken.approve(address(market), type(uint256).max);
    }

    function _list() internal returns (uint256 listingId) {
        vm.prank(seller);
        listingId = market.list(address(nft), TOKEN_ID, address(payToken), PRICE, block.timestamp + 1 days);
    }

    function _offer(uint256 amount) internal returns (uint256 offerId) {
        vm.prank(buyer);
        offerId = market.makeOffer(
            address(nft), TOKEN_ID, address(payToken), amount, block.timestamp + 1 days
        );
    }

    // ---- ERC721 ------------------------------------------------------------

    function testMintAssignsOwnershipAndMetadata() public view {
        assertEq(nft.ownerOf(TOKEN_ID), seller);
        assertEq(nft.balanceOf(seller), 1);
        assertEq(nft.totalSupply(), 1);
        assertEq(nft.tokenURI(TOKEN_ID), "ipfs://token-1");
    }

    function testMintOnlyOwner() public {
        vm.prank(seller);
        vm.expectRevert("TigerSwap: NOT_OWNER");
        nft.mint(seller, "x");
    }

    function testTransferMovesOwnership() public {
        vm.prank(seller);
        nft.transferFrom(seller, buyer, TOKEN_ID);

        assertEq(nft.ownerOf(TOKEN_ID), buyer);
        assertEq(nft.balanceOf(seller), 0);
        assertEq(nft.balanceOf(buyer), 1);
    }

    function testTransferRejectsAnUnauthorisedCaller() public {
        vm.prank(buyer);
        vm.expectRevert("TigerSwap: NOT_APPROVED");
        nft.transferFrom(seller, buyer, TOKEN_ID);
    }

    function testApprovedOperatorCanTransfer() public {
        vm.prank(seller);
        nft.approve(buyer, TOKEN_ID);

        vm.prank(buyer);
        nft.transferFrom(seller, buyer, TOKEN_ID);
        assertEq(nft.ownerOf(TOKEN_ID), buyer);
    }

    function testSingleApprovalIsClearedOnTransfer() public {
        vm.prank(seller);
        nft.approve(buyer, TOKEN_ID);

        vm.prank(seller);
        nft.transferFrom(seller, buyer, TOKEN_ID);

        // The stale approval must not survive into the new holder's tenure.
        assertEq(nft.getApproved(TOKEN_ID), address(0));
    }

    function testSafeTransferRejectsANonReceiverContract() public {
        NonReceiver bad = new NonReceiver();

        // A contract with no hook has nothing to return, so the call itself
        // fails and the revert carries no reason. Either way the transfer does
        // not go through.
        vm.prank(seller);
        vm.expectRevert();
        nft.safeTransferFrom(seller, address(bad), TOKEN_ID);
        assertEq(nft.ownerOf(TOKEN_ID), seller);
    }

    function testSafeTransferAcceptsAValidReceiver() public {
        Receiver good = new Receiver();

        vm.prank(seller);
        nft.safeTransferFrom(seller, address(good), TOKEN_ID);
        assertEq(nft.ownerOf(TOKEN_ID), address(good));
    }

    function testSafeTransferRejectsAReceiverThatDeclines() public {
        Receiver good = new Receiver();
        good.setAccept(false);

        vm.prank(seller);
        vm.expectRevert("TigerSwap: UNSAFE_RECIPIENT");
        nft.safeTransferFrom(seller, address(good), TOKEN_ID);
    }

    // ---- Listing -----------------------------------------------------------

    function testListEscrowsTheNFT() public {
        uint256 listingId = _list();

        assertEq(nft.ownerOf(TOKEN_ID), address(market));
        (
            address storedSeller,
            ,
            ,
            address paymentToken,
            uint256 price,
            ,
            bool active
        ) = market.listings(listingId);

        assertEq(storedSeller, seller);
        assertEq(paymentToken, address(payToken));
        assertEq(price, PRICE);
        assertTrue(active);
    }

    function testListRejectsAnUnsupportedPaymentToken() public {
        TestERC20 other = new TestERC20("X", "X");
        vm.prank(seller);
        vm.expectRevert("TigerSwap: UNSUPPORTED_TOKEN");
        market.list(address(nft), TOKEN_ID, address(other), PRICE, block.timestamp + 1 days);
    }

    function testListRejectsZeroPrice() public {
        vm.prank(seller);
        vm.expectRevert("TigerSwap: ZERO_PRICE");
        market.list(address(nft), TOKEN_ID, address(payToken), 0, block.timestamp + 1 days);
    }

    function testListRejectsAnExpiredDeadline() public {
        vm.prank(seller);
        vm.expectRevert("TigerSwap: DEADLINE_PASSED");
        market.list(address(nft), TOKEN_ID, address(payToken), PRICE, block.timestamp);
    }

    // ---- Buying ------------------------------------------------------------

    function testBuyTransfersTheNFTAndPaysTheSeller() public {
        uint256 listingId = _list();
        uint256 sellerBefore = payToken.balanceOf(seller);

        vm.prank(buyer);
        market.buy(listingId);

        assertEq(nft.ownerOf(TOKEN_ID), buyer);
        // No fee configured, so the seller receives the whole price.
        assertEq(payToken.balanceOf(seller) - sellerBefore, PRICE);
    }

    function testBuyTakesTheMarketplaceFee() public {
        market.setFee(250, feeRecipient); // 2.5%
        uint256 listingId = _list();

        uint256 sellerBefore = payToken.balanceOf(seller);
        uint256 feeBefore = payToken.balanceOf(feeRecipient);

        vm.prank(buyer);
        market.buy(listingId);

        uint256 expectedFee = (PRICE * 250) / 10_000;
        assertEq(payToken.balanceOf(feeRecipient) - feeBefore, expectedFee);
        assertEq(payToken.balanceOf(seller) - sellerBefore, PRICE - expectedFee);
    }

    function testBuyRejectsTheSeller() public {
        uint256 listingId = _list();
        vm.prank(seller);
        vm.expectRevert("TigerSwap: SELF_TRADE");
        market.buy(listingId);
    }

    function testBuyRejectsADoubleBuy() public {
        uint256 listingId = _list();

        vm.prank(buyer);
        market.buy(listingId);

        vm.prank(buyer);
        vm.expectRevert("TigerSwap: NOT_ACTIVE");
        market.buy(listingId);
    }

    function testBuyRejectsAfterTheDeadline() public {
        uint256 listingId = _list();

        vm.warp(block.timestamp + 2 days);

        vm.prank(buyer);
        vm.expectRevert("TigerSwap: DEADLINE_PASSED");
        market.buy(listingId);
    }

    function testCancelListingReturnsTheNFT() public {
        uint256 listingId = _list();

        vm.prank(seller);
        market.cancelListing(listingId);

        assertEq(nft.ownerOf(TOKEN_ID), seller);
    }

    function testCancelListingRejectsANonSeller() public {
        uint256 listingId = _list();
        vm.prank(buyer);
        vm.expectRevert("TigerSwap: NOT_SELLER");
        market.cancelListing(listingId);
    }

    function testUpdateListingPrice() public {
        uint256 listingId = _list();

        vm.prank(seller);
        market.updateListingPrice(listingId, PRICE * 2);

        (, , , , uint256 price, , ) = market.listings(listingId);
        assertEq(price, PRICE * 2);
    }

    // ---- Offers ------------------------------------------------------------

    function testMakeOfferEscrowsThePayment() public {
        uint256 offerId = _offer(900e18);

        assertEq(payToken.balanceOf(address(market)), 900e18);
        assertEq(market.bestOfferId(address(nft), TOKEN_ID, address(payToken)), offerId);
    }

    function testBestOfferTracksTheHighest() public {
        _offer(500e18);
        uint256 higher = _offer(900e18);

        assertEq(market.bestOfferId(address(nft), TOKEN_ID, address(payToken)), higher);

        // A lower offer must not displace the standing best.
        _offer(100e18);
        assertEq(market.bestOfferId(address(nft), TOKEN_ID, address(payToken)), higher);
    }

    function testAcceptOfferTransfersTheNFTAndPaysTheSeller() public {
        uint256 offerId = _offer(900e18);
        uint256 sellerBefore = payToken.balanceOf(seller);

        vm.prank(seller);
        market.acceptOffer(offerId, address(nft), TOKEN_ID);

        assertEq(nft.ownerOf(TOKEN_ID), buyer);
        assertEq(payToken.balanceOf(seller) - sellerBefore, 900e18);
    }

    function testAcceptOfferTakesTheFee() public {
        market.setFee(250, feeRecipient);
        uint256 offerId = _offer(900e18);

        uint256 sellerBefore = payToken.balanceOf(seller);
        uint256 feeBefore = payToken.balanceOf(feeRecipient);

        vm.prank(seller);
        market.acceptOffer(offerId, address(nft), TOKEN_ID);

        uint256 expectedFee = (900e18 * 250) / 10_000;
        assertEq(payToken.balanceOf(feeRecipient) - feeBefore, expectedFee);
        assertEq(payToken.balanceOf(seller) - sellerBefore, 900e18 - expectedFee);
    }

    function testAcceptOfferRejectsANonOwner() public {
        uint256 offerId = _offer(900e18);

        vm.prank(buyer);
        vm.expectRevert("TigerSwap: NOT_OWNER");
        market.acceptOffer(offerId, address(nft), TOKEN_ID);
    }

    function testAcceptOfferRejectsADoubleAccept() public {
        uint256 offerId = _offer(900e18);

        vm.prank(seller);
        market.acceptOffer(offerId, address(nft), TOKEN_ID);

        vm.prank(seller);
        vm.expectRevert("TigerSwap: NOT_ACTIVE");
        market.acceptOffer(offerId, address(nft), TOKEN_ID);
    }

    function testAcceptOfferRejectsAfterTheDeadline() public {
        uint256 offerId = _offer(900e18);

        vm.warp(block.timestamp + 2 days);

        vm.prank(seller);
        vm.expectRevert("TigerSwap: DEADLINE_PASSED");
        market.acceptOffer(offerId, address(nft), TOKEN_ID);
    }

    function testCancelOfferRefundsTheBuyer() public {
        uint256 offerId = _offer(900e18);
        uint256 before = payToken.balanceOf(buyer);

        vm.prank(buyer);
        market.cancelOffer(offerId);

        assertEq(payToken.balanceOf(buyer) - before, 900e18);
        assertEq(payToken.balanceOf(address(market)), 0);
    }

    function testCancelOfferRejectsANonBuyer() public {
        uint256 offerId = _offer(900e18);
        vm.prank(seller);
        vm.expectRevert("TigerSwap: NOT_BUYER");
        market.cancelOffer(offerId);
    }

    function testMakeOfferRejectsANonexistentToken() public {
        vm.prank(buyer);
        vm.expectRevert("TigerSwap: NO_TOKEN");
        market.makeOffer(
            address(nft), 999, address(payToken), 900e18, block.timestamp + 1 days
        );
    }

    // ---- Non-standard tokens ----------------------------------------------

    function testBuyWorksWithATokenThatReturnsNoValue() public {
        NoReturnERC20 usdt = new NoReturnERC20();
        market.setPaymentToken(address(usdt), true);

        usdt.mint(buyer, 1_000_000e18);
        vm.prank(buyer);
        usdt.approve(address(market), type(uint256).max);

        vm.prank(seller);
        uint256 listingId = market.list(
            address(nft), TOKEN_ID, address(usdt), PRICE, block.timestamp + 1 days
        );

        uint256 sellerBefore = usdt.balanceOf(seller);
        vm.prank(buyer);
        market.buy(listingId);

        assertEq(nft.ownerOf(TOKEN_ID), buyer);
        assertEq(usdt.balanceOf(seller) - sellerBefore, PRICE);
    }

    // ---- Admin -------------------------------------------------------------

    function testSetFeeRejectsAValueAboveTheCap() public {
        vm.expectRevert("TigerSwap: FEE_TOO_HIGH");
        market.setFee(501, feeRecipient);
    }

    function testSetFeeOnlyOwner() public {
        vm.prank(buyer);
        vm.expectRevert("TigerSwap: NOT_OWNER");
        market.setFee(10, feeRecipient);
    }

    function testSetPaymentTokenOnlyOwner() public {
        vm.prank(buyer);
        vm.expectRevert("TigerSwap: NOT_OWNER");
        market.setPaymentToken(address(0x1234), true);
    }

    // ---- Views -------------------------------------------------------------

    function testIsListingFillableReflectsTheDeadline() public {
        uint256 listingId = _list();
        assertTrue(market.isListingFillable(listingId));

        vm.warp(block.timestamp + 2 days);
        assertFalse(market.isListingFillable(listingId));
    }

    function testIsOfferAcceptableReflectsTheDeadline() public {
        uint256 offerId = _offer(900e18);
        assertTrue(market.isOfferAcceptable(offerId));

        vm.warp(block.timestamp + 2 days);
        assertFalse(market.isOfferAcceptable(offerId));
    }
}
