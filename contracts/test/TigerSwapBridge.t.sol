// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "forge-std/Test.sol";
import "../TigerSwapBridge.sol";
import "./TestERC20.sol";

contract BridgeTest is Test {
    uint256 constant LOCAL_CHAIN = 8888;
    uint256 constant REMOTE_CHAIN = 1;

    TigerSwapBridge bridge;
    TestERC20 token;

    uint256 validatorKey = 0xA11CE;
    uint256 validatorKey2 = 0xB0B;
    address validator;
    address validator2;
    address user = address(0xCAFE);

    function setUp() public {
        validator = vm.addr(validatorKey);
        validator2 = vm.addr(validatorKey2);

        bridge = new TigerSwapBridge(LOCAL_CHAIN);
        token = new TestERC20("TGR", "TGR");

        bridge.enableChain(REMOTE_CHAIN, true);
        bridge.whitelistToken(address(token), true);
        bridge.setValidator(validator, true);
        bridge.setValidator(validator2, true);
        bridge.setValidatorThreshold(2);

        token.mint(user, 1_000_000e18);
        // The bridge holds the locked supply that inbound messages release.
        token.mint(address(bridge), 1_000_000e18);
        vm.prank(user);
        token.approve(address(bridge), type(uint256).max);
    }

    function _sign(uint256 key, bytes32 digest) internal pure returns (bytes memory) {
        (uint8 v, bytes32 r, bytes32 s) = vm.sign(key, digest);
        return abi.encodePacked(r, s, v);
    }

    function _message(uint256 amount) internal view returns (TigerSwapBridge.BridgeMessage memory) {
        return TigerSwapBridge.BridgeMessage({
            messageId: keccak256("msg-1"),
            fromChain: REMOTE_CHAIN,
            toChain: LOCAL_CHAIN,
            token: address(token),
            amount: amount,
            receiver: user
        });
    }

    function testOutboundLocksTokensAndChargesFee() public {
        uint256 before = token.balanceOf(user);
        uint256 bridgeBefore = token.balanceOf(address(bridge));

        vm.prank(user);
        bytes32 messageId = bridge.bridge(REMOTE_CHAIN, address(token), 10_000e18, user, 1);

        assertTrue(messageId != bytes32(0));
        assertEq(token.balanceOf(user), before - 10_000e18);
        // 0.05% fee is retained by the bridge.
        assertEq(bridge.collectedFees(address(token)), 5e18);
        assertEq(token.balanceOf(address(bridge)), bridgeBefore + 10_000e18);
    }

    function testOutboundRejectsUnsupportedChain() public {
        vm.prank(user);
        vm.expectRevert(bytes("TigerSwap: CHAIN_NOT_SUPPORTED"));
        bridge.bridge(999, address(token), 10_000e18, user, 1);
    }

    function testOutboundRejectsDuplicateNonce() public {
        vm.startPrank(user);
        bridge.bridge(REMOTE_CHAIN, address(token), 10_000e18, user, 7);
        vm.expectRevert(bytes("TigerSwap: MESSAGE_EXISTS"));
        bridge.bridge(REMOTE_CHAIN, address(token), 10_000e18, user, 7);
        vm.stopPrank();
    }

    function testInboundWithValidatorSignaturesReleasesTokens() public {
        TigerSwapBridge.BridgeMessage memory m = _message(5_000e18);
        bytes32 digest = bridge.inboundDigest(m);

        bytes memory sigs = abi.encodePacked(_sign(validatorKey, digest), _sign(validatorKey2, digest));

        uint256 before = token.balanceOf(m.receiver);
        bridge.completeBridge(m, sigs);
        assertEq(token.balanceOf(m.receiver), before + 5_000e18);
    }

    function testInboundWithInsufficientSignaturesReverts() public {
        TigerSwapBridge.BridgeMessage memory m = _message(5_000e18);
        bytes32 digest = bridge.inboundDigest(m);

        bytes memory sigs = _sign(validatorKey, digest);
        vm.expectRevert(bytes("TigerSwap: INSUFFICIENT_SIGNATURES"));
        bridge.completeBridge(m, sigs);
    }

    function testInboundRejectsNonValidatorSignature() public {
        TigerSwapBridge.BridgeMessage memory m = _message(5_000e18);
        bytes32 digest = bridge.inboundDigest(m);

        uint256 strangerKey = 0xDEAD;
        bytes memory sigs = abi.encodePacked(
            _sign(validatorKey, digest),
            _sign(strangerKey, digest)
        );

        vm.expectRevert(bytes("TigerSwap: INSUFFICIENT_SIGNATURES"));
        bridge.completeBridge(m, sigs);
    }

    function testInboundRejectsTamperedAmount() public {
        TigerSwapBridge.BridgeMessage memory m = _message(5_000e18);
        bytes32 digest = bridge.inboundDigest(m);
        bytes memory sigs = abi.encodePacked(_sign(validatorKey, digest), _sign(validatorKey2, digest));

        // Attacker changes the amount after signing.
        m.amount = 500_000e18;
        vm.expectRevert(bytes("TigerSwap: INSUFFICIENT_SIGNATURES"));
        bridge.completeBridge(m, sigs);
    }

    function testInboundReplayIsRejected() public {
        TigerSwapBridge.BridgeMessage memory m = _message(5_000e18);
        bytes32 digest = bridge.inboundDigest(m);
        bytes memory sigs = abi.encodePacked(_sign(validatorKey, digest), _sign(validatorKey2, digest));

        bridge.completeBridge(m, sigs);
        vm.expectRevert(bytes("TigerSwap: MESSAGE_ALREADY_PROCESSED"));
        bridge.completeBridge(m, sigs);
    }

    function testDuplicateSignerCountedOnce() public {
        TigerSwapBridge.BridgeMessage memory m = _message(5_000e18);
        bytes32 digest = bridge.inboundDigest(m);

        // The same validator signs twice; the threshold of 2 is not met.
        bytes memory sigs = abi.encodePacked(_sign(validatorKey, digest), _sign(validatorKey, digest));
        vm.expectRevert(bytes("TigerSwap: INSUFFICIENT_SIGNATURES"));
        bridge.completeBridge(m, sigs);
    }

    /// The threshold is not initialized by the constructor. Before the fix an
    /// unset threshold was zero, so `_countValidSignatures(...) >= 0` always held
    /// and anyone could release the bridge's locked liquidity with no signatures.
    function testInboundRejectsEmptySignaturesWhenThresholdUnset() public {
        TigerSwapBridge fresh = new TigerSwapBridge(LOCAL_CHAIN);
        fresh.enableChain(REMOTE_CHAIN, true);
        fresh.whitelistToken(address(token), true);
        fresh.setValidator(validator, true);
        // Deliberately never call setValidatorThreshold.

        TigerSwapBridge.BridgeMessage memory m = _message(5_000e18);
        vm.expectRevert(bytes("TigerSwap: THRESHOLD_NOT_SET"));
        fresh.completeBridge(m, "");
    }

    /// A threshold larger than the live validator set can never be met, which
    /// would permanently block the inbound path.
    function testSetValidatorThresholdCannotExceedValidatorCount() public {
        vm.expectRevert(bytes("TigerSwap: THRESHOLD_EXCEEDS_VALIDATORS"));
        bridge.setValidatorThreshold(3);
    }

    /// The validator count follows the set as members are added and removed.
    function testValidatorCountTracksSet() public {
        assertEq(bridge.validatorCount(), 2);
        bridge.setValidator(address(0x1234), true);
        assertEq(bridge.validatorCount(), 3);
        // Re-adding an existing validator must not double count.
        bridge.setValidator(address(0x1234), true);
        assertEq(bridge.validatorCount(), 3);
        bridge.setValidator(address(0x1234), false);
        assertEq(bridge.validatorCount(), 2);
        // Removing an address that was never a validator must not underflow.
        bridge.setValidator(address(0x5678), false);
        assertEq(bridge.validatorCount(), 2);
    }

    function testWithdrawFees() public {
        vm.prank(user);
        bridge.bridge(REMOTE_CHAIN, address(token), 10_000e18, user, 3);

        address treasury = address(0xFEE);
        bridge.withdrawFees(address(token), treasury);
        assertEq(token.balanceOf(treasury), 5e18);
        assertEq(bridge.collectedFees(address(token)), 0);
    }
}
