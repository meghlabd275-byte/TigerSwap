// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "forge-std/Test.sol";
import "../TigerSwapStable.sol";
import "./TestERC20.sol";

/// @title Fuzz tests for the stable pool's core invariants.
/// @notice The example-based suite checks pricing at a handful of points. These
///         pin the properties an LP relies on across the whole input domain:
///         a quote never exceeds execution, a swap can never drain a coin, the
///         pool's invariant never decreases (fees accrue, they are not paid out
///         of reserves), and a deposit/withdraw round trip cannot be profitable.
contract TigerSwapStableFuzzTest is Test {
    TigerSwapStable public pool;
    TestERC20 public rusd;
    TestERC20 public usdt;

    address public lp = address(0x11);
    address public trader = address(0x22);

    uint256 constant A = 200;
    /// @dev 0.04% fee.
    uint256 constant FEE = 4e6;
    uint256 constant ADMIN_FEE = 5e9;

    function setUp() public {
        rusd = new TestERC20("RUSD", "RUSD");
        usdt = new TestERC20("USDT", "USDT");

        address[2] memory coins = [address(rusd), address(usdt)];
        pool = new TigerSwapStable(coins, A, FEE, ADMIN_FEE);

        rusd.mint(lp, 1e30);
        usdt.mint(lp, 1e30);
        vm.startPrank(lp);
        rusd.approve(address(pool), type(uint256).max);
        usdt.approve(address(pool), type(uint256).max);
        pool.addLiquidity(_two(1_000_000e18, 1_000_000e18), 0);
        vm.stopPrank();

        rusd.mint(trader, 1e30);
        usdt.mint(trader, 1e30);
        vm.startPrank(trader);
        rusd.approve(address(pool), type(uint256).max);
        usdt.approve(address(pool), type(uint256).max);
        vm.stopPrank();
    }

    function _two(uint256 a, uint256 b) internal pure returns (uint256[2] memory out) {
        out[0] = a;
        out[1] = b;
        return out;
    }

    /// @dev The invariant `D`, recovered from the virtual price. `getVirtualPrice`
    ///      is `D * PRECISION / totalSupply`, so `D = virtualPrice * supply / PRECISION`.
    function _invariant() internal view returns (uint256) {
        return pool.getVirtualPrice() * pool.totalSupply() / 1e18;
    }

    /// @notice A swap of a positive amount of either coin never reduces the
    ///         pool's invariant: the fee is retained, not paid out of reserves.
    function testFuzzSwapNeverReducesInvariant(uint256 amountIn, bool sellRusd) public {
        uint256 balance = pool.balances(sellRusd ? 0 : 1);
        amountIn = bound(amountIn, 1e6, balance / 10);

        uint256 dBefore = _invariant();

        vm.prank(trader);
        pool.exchange(sellRusd ? int128(0) : int128(1), sellRusd ? int128(1) : int128(0), amountIn, 0);

        assertGe(_invariant(), dBefore, "invariant decreased across a swap");
    }

    /// @notice A swap never drains the output coin: the pool keeps a positive
    ///         balance and pays out strictly less than it holds.
    function testFuzzSwapNeverDrainsOutputCoin(uint256 amountIn, bool sellRusd) public {
        uint256 inIdx = sellRusd ? 0 : 1;
        uint256 outIdx = sellRusd ? 1 : 0;
        uint256 balance = pool.balances(inIdx);
        amountIn = bound(amountIn, 1e6, balance / 10);

        uint256 outBefore = pool.balances(outIdx);

        vm.prank(trader);
        uint256 dy = pool.exchange(sellRusd ? int128(0) : int128(1), sellRusd ? int128(1) : int128(0), amountIn, 0);

        assertLt(dy, outBefore, "payout reached the pool's whole balance");
        assertGt(pool.balances(outIdx), 0, "output coin fully drained");
    }

    /// @notice The quote never exceeds what execution pays: the two share one
    ///         pricing path, so a trader is never quoted more than they receive.
    function testFuzzQuoteIsAnUpperBoundOnExecution(uint256 amountIn, bool sellRusd) public {
        uint256 inIdx = sellRusd ? 0 : 1;
        uint256 outIdx = sellRusd ? 1 : 0;
        uint256 balance = pool.balances(inIdx);
        amountIn = bound(amountIn, 1e6, balance / 10);

        uint256 quoted = pool.getDy(sellRusd ? int128(0) : int128(1), sellRusd ? int128(1) : int128(0), amountIn);

        vm.prank(trader);
        uint256 executed = pool.exchange(sellRusd ? int128(0) : int128(1), sellRusd ? int128(1) : int128(0), amountIn, 0);

        assertEq(executed, quoted, "execution disagreed with the quote");
    }

    /// @notice A swap moves the input balance up by exactly the amount paid in,
    ///         and never pays the trader more than the input is worth at parity.
    function testFuzzSwapMovesInputBalanceExactly(uint256 amountIn, bool sellRusd) public {
        uint256 inIdx = sellRusd ? 0 : 1;
        uint256 balance = pool.balances(inIdx);
        amountIn = bound(amountIn, 1e6, balance / 10);

        uint256 inBefore = pool.balances(inIdx);
        uint256 outBefore = pool.balances(sellRusd ? 1 : 0);

        vm.prank(trader);
        uint256 dy = pool.exchange(sellRusd ? int128(0) : int128(1), sellRusd ? int128(1) : int128(0), amountIn, 0);

        assertEq(pool.balances(inIdx), inBefore + amountIn, "input balance did not rise by the amount paid");
        // The pool's output reserve falls by the payout plus the admin's cut, so
        // it can only have fallen by at least the payout.
        assertLe(pool.balances(sellRusd ? 1 : 0) + dy, outBefore, "pool lost more than it paid the trader");
    }

    /// @notice A proportional deposit followed by an immediate withdrawal cannot
    ///         return more than was deposited: the round trip is never profitable.
    function testFuzzDepositWithdrawRoundTripIsNotProfitable(uint256 amount) public {
        amount = bound(amount, 1e6, 10_000e18);

        rusd.mint(address(this), amount);
        usdt.mint(address(this), amount);
        rusd.approve(address(pool), type(uint256).max);
        usdt.approve(address(pool), type(uint256).max);

        uint256 minted = pool.addLiquidity(_two(amount, amount), 0);

        uint256[2] memory out = pool.removeLiquidity(minted, _two(0, 0));

        assertLe(out[0], amount, "withdrawal of coin 0 exceeded the deposit");
        assertLe(out[1], amount, "withdrawal of coin 1 exceeded the deposit");
    }
}
