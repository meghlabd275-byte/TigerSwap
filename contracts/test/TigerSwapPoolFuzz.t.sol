// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "forge-std/Test.sol";
import "../TigerSwapFactory.sol";
import "../TigerSwapRouter.sol";
import "./TestERC20.sol";

/// @title Fuzz tests for the xyK pool's core invariants.
/// @notice These pin the properties a liquidity provider relies on, which the
///         example-based tests only check at a handful of points: the
///         constant-product invariant never decreases across a swap, a quote can
///         never exceed the pool's reserves, and a full deposit/withdraw round
///         trip cannot be profitable.
contract TigerSwapPoolFuzzTest is Test {
    TigerSwapFactory public factory;
    TigerSwapRouter public router;
    TigerSwapPool public pool;
    TestERC20 public tokenA;
    TestERC20 public tokenB;

    address public lp = address(0x11);
    address public trader = address(0x22);

    function setUp() public {
        tokenA = new TestERC20("AAA", "AAA");
        tokenB = new TestERC20("BBB", "BBB");
        factory = new TigerSwapFactory();
        router = new TigerSwapRouter(address(factory), address(0));
        pool = TigerSwapPool(factory.createPool(address(tokenA), address(tokenB)));

        // Seed a deep pool so a fuzzed swap is a small fraction of its depth.
        tokenA.mint(lp, 1e30);
        tokenB.mint(lp, 1e30);
        vm.startPrank(lp);
        tokenA.transfer(address(pool), 1_000_000e18);
        tokenB.transfer(address(pool), 1_000_000e18);
        pool.mint(lp);
        vm.stopPrank();
    }

    function _path(bool useTokenA) internal view returns (address[] memory path) {
        path = new address[](2);
        path[0] = useTokenA ? address(tokenA) : address(tokenB);
        path[1] = useTokenA ? address(tokenB) : address(tokenA);
    }

    function _payAndSwap(address tokenIn, uint256 amountIn) internal {
        tokenIn == address(tokenA) ? tokenA.mint(trader, amountIn) : tokenB.mint(trader, amountIn);
        vm.startPrank(trader);
        if (tokenIn == address(tokenA)) {
            tokenA.approve(address(router), amountIn);
        } else {
            tokenB.approve(address(router), amountIn);
        }
        router.swapExactTokensForTokens(amountIn, 0, _path(tokenIn == address(tokenA)), trader, block.timestamp + 1);
        vm.stopPrank();
    }

    /// @notice Swapping a positive amount of either token never reduces
    ///         reserve0 * reserve1. The fee stays in the pool, so k grows.
    function testFuzzSwapNeverDecreasesK(uint256 amountIn, bool useTokenA) public {
        (uint112 r0, uint112 r1,) = pool.getReserves();
        amountIn = bound(amountIn, 1e6, uint256(r0) / 4);

        _payAndSwap(useTokenA ? address(tokenA) : address(tokenB), amountIn);

        (uint112 nr0, uint112 nr1,) = pool.getReserves();
        assertGe(uint256(nr0) * uint256(nr1), uint256(r0) * uint256(r1), "k must not decrease");
    }

    /// @notice A quote for any input never reaches or exceeds the opposite
    ///         reserve, so a single swap can never drain the pool.
    function testFuzzQuoteCannotDrainReserves(uint256 amountIn) public {
        (uint112 r0, uint112 r1,) = pool.getReserves();
        amountIn = bound(amountIn, 1, type(uint112).max / 4);

        uint256[] memory amounts = router.getAmountsOut(amountIn, _path(true));
        assertLt(amounts[1], uint256(r1), "quote must stay under the reserve");
    }

    /// @notice Burning an entire LP position returns no more of each token than
    ///         was deposited, so a round trip cannot mint value out of the pool.
    function testFuzzRoundTripNeverProfits(uint256 depositA, uint256 depositB) public {
        depositA = bound(depositA, 1e18, 100_000e18);
        depositB = bound(depositB, 1e18, 100_000e18);

        address user = address(0xBEEF);
        tokenA.mint(user, depositA);
        tokenB.mint(user, depositB);

        uint256 balA0 = tokenA.balanceOf(user);
        uint256 balB0 = tokenB.balanceOf(user);

        vm.startPrank(user);
        tokenA.transfer(address(pool), depositA);
        tokenB.transfer(address(pool), depositB);
        uint256 liquidity = pool.mint(user);
        pool.transfer(address(pool), liquidity);
        pool.burn(user);
        vm.stopPrank();

        // The 0.3% swap fee is not charged here, but a round trip through mint
        // and burn must still never return more than was put in.
        assertLe(tokenA.balanceOf(user), balA0, "round trip must not gain tokenA");
        assertLe(tokenB.balanceOf(user), balB0, "round trip must not gain tokenB");
    }
}
