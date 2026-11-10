// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "forge-std/Test.sol";
import "../TigerSwapSafeERC20.sol";

/// @notice Token that returns no data from transfer/transferFrom/approve,
///         like USDT on Ethereum. A direct `IERC20(...).transfer(...)` reverts
///         against it because the ABI decoder expects a `bool`.
contract NoReturnToken {
    mapping(address => uint256) public balanceOf;
    mapping(address => mapping(address => uint256)) public allowance;

    function mint(address to, uint256 amount) external {
        balanceOf[to] += amount;
    }

    function approve(address spender, uint256 amount) external {
        allowance[msg.sender][spender] = amount;
    }

    function transfer(address to, uint256 amount) external {
        require(balanceOf[msg.sender] >= amount, "NoReturnToken: BALANCE");
        balanceOf[msg.sender] -= amount;
        balanceOf[to] += amount;
    }

    function transferFrom(address from, address to, uint256 amount) external {
        require(balanceOf[from] >= amount, "NoReturnToken: BALANCE");
        uint256 allowed = allowance[from][msg.sender];
        if (allowed != type(uint256).max) {
            require(allowed >= amount, "NoReturnToken: ALLOWANCE");
            allowance[from][msg.sender] = allowed - amount;
        }
        balanceOf[from] -= amount;
        balanceOf[to] += amount;
    }
}

/// @notice Token that returns `false` without reverting. A direct
///         `IERC20(...).transfer(...)` inside a `require` happens to catch it,
///         but a bare call silently "succeeds"; the safe helper must reject it.
contract FalseReturnToken {
    function transfer(address, uint256) external pure returns (bool) {
        return false;
    }

    function transferFrom(address, address, uint256) external pure returns (bool) {
        return false;
    }

    function approve(address, uint256) external pure returns (bool) {
        return false;
    }
}

contract SafeERC20Harness {
    function pull(address token, address from, uint256 amount) external {
        TigerSwapSafeERC20.safeTransferFrom(token, from, address(this), amount);
    }

    function push(address token, address to, uint256 amount) external {
        TigerSwapSafeERC20.safeTransfer(token, to, amount);
    }
}

contract TigerSwapSafeERC20Test is Test {
    SafeERC20Harness internal harness;
    address internal alice = address(0xA11CE);

    function setUp() public {
        harness = new SafeERC20Harness();
    }

    /// A token that returns nothing (USDT-style) must transfer successfully.
    function testSafeTransferAcceptsNoReturnToken() public {
        NoReturnToken token = new NoReturnToken();
        token.mint(address(harness), 1_000);

        harness.push(address(token), alice, 400);

        assertEq(token.balanceOf(alice), 400);
        assertEq(token.balanceOf(address(harness)), 600);
    }

    function testSafeTransferFromAcceptsNoReturnToken() public {
        NoReturnToken token = new NoReturnToken();
        token.mint(alice, 1_000);
        vm.prank(alice);
        token.approve(address(harness), 1_000);

        harness.pull(address(token), alice, 250);

        assertEq(token.balanceOf(alice), 750);
        assertEq(token.balanceOf(address(harness)), 250);
    }

    /// A token that returns `false` must revert rather than silently pass.
    function testSafeTransferRejectsFalseReturn() public {
        FalseReturnToken token = new FalseReturnToken();

        vm.expectRevert(bytes("TigerSwap: TRANSFER_FAILED"));
        harness.push(address(token), alice, 1);
    }

    function testSafeTransferFromRejectsFalseReturn() public {
        FalseReturnToken token = new FalseReturnToken();

        vm.expectRevert(bytes("TigerSwap: TRANSFER_FAILED"));
        harness.pull(address(token), alice, 1);
    }

    /// A token that reverts on transfer must bubble the failure up.
    function testSafeTransferFromRevertsWhenTokenReverts() public {
        NoReturnToken token = new NoReturnToken();
        token.mint(alice, 10);

        vm.expectRevert();
        harness.pull(address(token), alice, 100);
    }
}
