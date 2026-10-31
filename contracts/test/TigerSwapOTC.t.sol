// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "forge-std/Test.sol";
import "../TigerSwapOTC.sol";
import "./TestERC20.sol";

contract TigerSwapOTCTest is Test {
    TigerSwapOTC public otc;
    TestERC20 public sellToken;
    TestERC20 public buyToken;

    uint256 constant SELLER_PK = 0xA11CE;
    address public seller;
    address public buyer = address(0x22);
    address public feeRecipient = address(0x44);

    uint256 constant SELL_AMOUNT = 1_000_000e18;
    uint256 constant BUY_AMOUNT = 2_000_000e18;

    function setUp() public {
        seller = vm.addr(SELLER_PK);
        sellToken = new TestERC20("TGR", "TGR");
        buyToken = new TestERC20("RUSD", "RUSD");

        otc = new TigerSwapOTC(feeRecipient);
        otc.setSupportedToken(address(sellToken), true);
        otc.setSupportedToken(address(buyToken), true);

        sellToken.mint(seller, 10_000_000e18);
        buyToken.mint(buyer, 10_000_000e18);

        vm.prank(seller);
        sellToken.approve(address(otc), type(uint256).max);
        vm.prank(buyer);
        buyToken.approve(address(otc), type(uint256).max);
    }

    function _deadline() internal view returns (uint256) {
        return block.timestamp + 1 hours;
    }

    function _digest(uint256 sellAmount, uint256 buyAmount, uint256 deadline)
        internal
        view
        returns (bytes32)
    {
        return keccak256(
            abi.encode(
                address(otc),
                block.chainid,
                seller,
                address(sellToken),
                address(buyToken),
                sellAmount,
                buyAmount,
                deadline
            )
        );
    }

    /// @dev Produces the EIP-191 signature the contract expects.
    function _sign(uint256 pk, bytes32 digest) internal pure returns (bytes memory) {
        bytes32 prefixed = keccak256(abi.encodePacked("\x19Ethereum Signed Message:\n32", digest));
        (uint8 v, bytes32 r, bytes32 s) = vm.sign(pk, prefixed);
        return abi.encodePacked(r, s, v);
    }

    function _createDefault() internal returns (uint256 tradeId) {
        uint256 deadline = _deadline();
        bytes memory sig = _sign(SELLER_PK, _digest(SELL_AMOUNT, BUY_AMOUNT, deadline));
        vm.prank(seller);
        tradeId = otc.createTrade(
            address(sellToken), address(buyToken), SELL_AMOUNT, BUY_AMOUNT, deadline, sig
        );
    }

    // ---- Creation ----------------------------------------------------------

    function testCreateTradeEscrowsTheSellAmount() public {
        uint256 before = sellToken.balanceOf(seller);
        uint256 tradeId = _createDefault();

        assertEq(tradeId, 1);
        assertEq(before - sellToken.balanceOf(seller), SELL_AMOUNT);
        assertEq(sellToken.balanceOf(address(otc)), SELL_AMOUNT);
    }

    function testCreateTradeStoresTheTerms() public {
        uint256 tradeId = _createDefault();
        (
            address storedSeller,
            address storedBuyer,
            address storedSell,
            address storedBuy,
            uint256 sellAmount,
            uint256 buyAmount,
            ,
            bool active
        ) = otc.trades(tradeId);

        assertEq(storedSeller, seller);
        assertEq(storedBuyer, address(0));
        assertEq(storedSell, address(sellToken));
        assertEq(storedBuy, address(buyToken));
        assertEq(sellAmount, SELL_AMOUNT);
        assertEq(buyAmount, BUY_AMOUNT);
        assertTrue(active);
    }

    function testCreateTradeRejectsASignatureFromAnotherKey() public {
        uint256 deadline = _deadline();
        // Signed by a key that is not the caller, so the trade is not the
        // caller's to open.
        bytes memory sig = _sign(0xB0B, _digest(SELL_AMOUNT, BUY_AMOUNT, deadline));

        vm.prank(seller);
        vm.expectRevert("TigerSwap: BAD_SIGNATURE");
        otc.createTrade(
            address(sellToken), address(buyToken), SELL_AMOUNT, BUY_AMOUNT, deadline, sig
        );
    }

    function testCreateTradeRejectsTamperedTerms() public {
        uint256 deadline = _deadline();
        // The signature covers the original price; the call asks for another.
        bytes memory sig = _sign(SELLER_PK, _digest(SELL_AMOUNT, BUY_AMOUNT, deadline));

        vm.prank(seller);
        vm.expectRevert("TigerSwap: BAD_SIGNATURE");
        otc.createTrade(
            address(sellToken), address(buyToken), SELL_AMOUNT, BUY_AMOUNT * 2, deadline, sig
        );
    }

    function testCreateTradeRejectsAnExpiredDeadline() public {
        uint256 deadline = block.timestamp - 1;
        bytes memory sig = _sign(SELLER_PK, _digest(SELL_AMOUNT, BUY_AMOUNT, deadline));

        vm.prank(seller);
        vm.expectRevert("TigerSwap: DEADLINE_PASSED");
        otc.createTrade(
            address(sellToken), address(buyToken), SELL_AMOUNT, BUY_AMOUNT, deadline, sig
        );
    }

    function testCreateTradeRejectsAnUnsupportedToken() public {
        TestERC20 other = new TestERC20("X", "X");
        other.mint(seller, SELL_AMOUNT);
        vm.prank(seller);
        other.approve(address(otc), type(uint256).max);

        uint256 deadline = _deadline();
        bytes memory sig = _sign(SELLER_PK, _digest(SELL_AMOUNT, BUY_AMOUNT, deadline));

        vm.prank(seller);
        vm.expectRevert("TigerSwap: UNSUPPORTED_TOKEN");
        otc.createTrade(
            address(other), address(buyToken), SELL_AMOUNT, BUY_AMOUNT, deadline, sig
        );
    }

    function testCreateTradeRejectsAReplayedSignature() public {
        uint256 deadline = _deadline();
        bytes memory sig = _sign(SELLER_PK, _digest(SELL_AMOUNT, BUY_AMOUNT, deadline));

        vm.prank(seller);
        otc.createTrade(
            address(sellToken), address(buyToken), SELL_AMOUNT, BUY_AMOUNT, deadline, sig
        );

        // The same signature must not open a second trade.
        vm.prank(seller);
        vm.expectRevert("TigerSwap: DIGEST_USED");
        otc.createTrade(
            address(sellToken), address(buyToken), SELL_AMOUNT, BUY_AMOUNT, deadline, sig
        );
    }

    function testCreateTradeRejectsZeroAmount() public {
        uint256 deadline = _deadline();
        bytes memory sig = _sign(SELLER_PK, _digest(0, BUY_AMOUNT, deadline));

        vm.prank(seller);
        vm.expectRevert("TigerSwap: ZERO_AMOUNT");
        otc.createTrade(address(sellToken), address(buyToken), 0, BUY_AMOUNT, deadline, sig);
    }

    // ---- Acceptance --------------------------------------------------------

    function testAcceptTradeSwapsTheAssets() public {
        uint256 tradeId = _createDefault();

        uint256 sellerBuyBefore = buyToken.balanceOf(seller);
        uint256 buyerSellBefore = sellToken.balanceOf(buyer);
        uint256 buyerBuyBefore = buyToken.balanceOf(buyer);

        vm.prank(buyer);
        uint256 received = otc.acceptTrade(tradeId);

        assertEq(received, SELL_AMOUNT);
        // No fee is set, so the seller receives the whole buy amount.
        assertEq(buyToken.balanceOf(seller) - sellerBuyBefore, BUY_AMOUNT);
        assertEq(sellToken.balanceOf(buyer) - buyerSellBefore, SELL_AMOUNT);
        assertEq(buyerBuyBefore - buyToken.balanceOf(buyer), BUY_AMOUNT);
        assertEq(sellToken.balanceOf(address(otc)), 0);
    }

    function testAcceptTradeTakesTheProtocolFee() public {
        otc.setFee(50, feeRecipient); // 0.5%
        uint256 tradeId = _createDefault();

        uint256 sellerBefore = buyToken.balanceOf(seller);
        uint256 feeBefore = buyToken.balanceOf(feeRecipient);

        vm.prank(buyer);
        otc.acceptTrade(tradeId);

        uint256 expectedFee = (BUY_AMOUNT * 50) / 10_000;
        assertEq(buyToken.balanceOf(feeRecipient) - feeBefore, expectedFee);
        assertEq(buyToken.balanceOf(seller) - sellerBefore, BUY_AMOUNT - expectedFee);
    }

    function testAcceptTradeRejectsTheSeller() public {
        uint256 tradeId = _createDefault();

        vm.prank(seller);
        vm.expectRevert("TigerSwap: SELF_TRADE");
        otc.acceptTrade(tradeId);
    }

    function testAcceptTradeRejectsADoubleAccept() public {
        uint256 tradeId = _createDefault();

        vm.prank(buyer);
        otc.acceptTrade(tradeId);

        vm.prank(buyer);
        vm.expectRevert("TigerSwap: NOT_ACTIVE");
        otc.acceptTrade(tradeId);
    }

    function testAcceptTradeRejectsAfterTheDeadline() public {
        uint256 tradeId = _createDefault();

        vm.warp(block.timestamp + 2 hours);

        vm.prank(buyer);
        vm.expectRevert("TigerSwap: DEADLINE_PASSED");
        otc.acceptTrade(tradeId);
    }

    function testAcceptTradeRejectsAnUnknownTrade() public {
        vm.prank(buyer);
        vm.expectRevert("TigerSwap: NOT_ACTIVE");
        otc.acceptTrade(999);
    }

    // ---- Cancellation ------------------------------------------------------

    function testCancelTradeRefundsTheEscrow() public {
        uint256 tradeId = _createDefault();
        uint256 before = sellToken.balanceOf(seller);

        vm.prank(seller);
        uint256 refunded = otc.cancelTrade(tradeId);

        assertEq(refunded, SELL_AMOUNT);
        assertEq(sellToken.balanceOf(seller) - before, SELL_AMOUNT);
        assertEq(sellToken.balanceOf(address(otc)), 0);
    }

    function testCancelTradeRejectsANonSeller() public {
        uint256 tradeId = _createDefault();

        vm.prank(buyer);
        vm.expectRevert("TigerSwap: NOT_SELLER");
        otc.cancelTrade(tradeId);
    }

    function testCancelTradeRejectsAnAcceptedTrade() public {
        uint256 tradeId = _createDefault();

        vm.prank(buyer);
        otc.acceptTrade(tradeId);

        vm.prank(seller);
        vm.expectRevert("TigerSwap: NOT_ACTIVE");
        otc.cancelTrade(tradeId);
    }

    // ---- Admin -------------------------------------------------------------

    function testSetFeeRejectsAValueAboveTheCap() public {
        vm.expectRevert("TigerSwap: FEE_TOO_HIGH");
        otc.setFee(101, feeRecipient);
    }

    function testSetFeeOnlyOwner() public {
        vm.prank(buyer);
        vm.expectRevert("TigerSwap: NOT_OWNER");
        otc.setFee(10, feeRecipient);
    }

    function testSetSupportedTokenOnlyOwner() public {
        vm.prank(buyer);
        vm.expectRevert("TigerSwap: NOT_OWNER");
        otc.setSupportedToken(address(0x1234), true);
    }

    // ---- Views -------------------------------------------------------------

    function testIsTradeFillableReflectsTheDeadline() public {
        uint256 tradeId = _createDefault();
        assertTrue(otc.isTradeFillable(tradeId));

        vm.warp(block.timestamp + 2 hours);
        assertFalse(otc.isTradeFillable(tradeId));
    }
}
