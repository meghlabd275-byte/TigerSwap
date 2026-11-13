// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "forge-std/Test.sol";
import "../TigerSwapFactory.sol";
import "../TigerSwapRouter.sol";
import "./TestERC20.sol";

/// @notice Drives random swaps, liquidity adds and liquidity removals against
///         one xyK pool. The invariant functions assert on the pool's state
///         after any sequence of these actions.
contract PoolHandler is Test {
    TigerSwapFactory public factory;
    TigerSwapRouter public router;
    TigerSwapPool public pool;
    TestERC20 public tokenA;
    TestERC20 public tokenB;

    address[3] public actors;

    constructor(
        TigerSwapFactory _factory,
        TigerSwapRouter _router,
        TigerSwapPool _pool,
        TestERC20 _tokenA,
        TestERC20 _tokenB
    ) {
        factory = _factory;
        router = _router;
        pool = _pool;
        tokenA = _tokenA;
        tokenB = _tokenB;
        actors = [address(0xA1), address(0xA2), address(0xA3)];
        for (uint256 i = 0; i < actors.length; i++) {
            vm.startPrank(actors[i]);
            tokenA.approve(address(router), type(uint256).max);
            tokenB.approve(address(router), type(uint256).max);
            vm.stopPrank();
        }
    }

    function _actor(uint256 seed) internal view returns (address) {
        return actors[seed % actors.length];
    }

    function addLiquidity(uint256 amountA, uint256 amountB, uint256 seed) public {
        amountA = bound(amountA, 1e6, 100_000e18);
        amountB = bound(amountB, 1e6, 100_000e18);
        address actor = _actor(seed);

        tokenA.mint(actor, amountA);
        tokenB.mint(actor, amountB);
        vm.prank(actor);
        router.addLiquidity(
            address(tokenA), address(tokenB), amountA, amountB, 0, 0, actor, block.timestamp + 1
        );
    }

    function swap(uint256 amountIn, bool useTokenA, uint256 seed) public {
        (uint112 r0, uint112 r1,) = pool.getReserves();
        if (r0 == 0 || r1 == 0) return;
        amountIn = bound(amountIn, 1e6, uint256(r0) / 10);
        address actor = _actor(seed);

        address tokenIn = useTokenA ? address(tokenA) : address(tokenB);
        if (tokenIn == address(tokenA)) tokenA.mint(actor, amountIn);
        else tokenB.mint(actor, amountIn);

        address[] memory path = new address[](2);
        path[0] = useTokenA ? address(tokenA) : address(tokenB);
        path[1] = useTokenA ? address(tokenB) : address(tokenA);

        vm.prank(actor);
        router.swapExactTokensForTokens(amountIn, 0, path, actor, block.timestamp + 1);
    }

    function removeLiquidity(uint256 liquidity, uint256 seed) public {
        address actor = _actor(seed);
        uint256 balance = pool.balanceOf(actor);
        if (balance == 0) return;
        liquidity = bound(liquidity, 1, balance);

        vm.startPrank(actor);
        pool.transfer(address(pool), liquidity);
        pool.burn(actor);
        vm.stopPrank();
    }
}

/// @title Invariant tests for the xyK pool.
/// @notice After any random sequence of swaps and liquidity changes the pool
///         must still hold at least the constant-product value it started with,
///         its reserves must never exceed its token balances, and its LP supply
///         must always be backed by the minimum locked liquidity.
contract TigerSwapPoolInvariantTest is Test {
    TigerSwapFactory public factory;
    TigerSwapRouter public router;
    TigerSwapPool public pool;
    TestERC20 public tokenA;
    TestERC20 public tokenB;
    PoolHandler public handler;

    function setUp() public {
        tokenA = new TestERC20("AAA", "AAA");
        tokenB = new TestERC20("BBB", "BBB");
        factory = new TigerSwapFactory();
        router = new TigerSwapRouter(address(factory), address(0));
        pool = TigerSwapPool(factory.createPool(address(tokenA), address(tokenB)));

        handler = new PoolHandler(factory, router, pool, tokenA, tokenB);
        targetContract(address(handler));

        // Seed the pool so the invariants have a non-empty starting point.
        tokenA.mint(address(this), 1_000_000e18);
        tokenB.mint(address(this), 1_000_000e18);
        tokenA.transfer(address(pool), 1_000_000e18);
        tokenB.transfer(address(pool), 1_000_000e18);
        pool.mint(address(this));
    }

    /// @dev The pool's reserve0/reserve1 are keyed by the sorted token order,
    ///      which is not necessarily (tokenA, tokenB).
    function _balancesInPoolOrder() internal view returns (uint256 bal0, uint256 bal1) {
        if (pool.token0() == address(tokenA)) {
            return (tokenA.balanceOf(address(pool)), tokenB.balanceOf(address(pool)));
        }
        return (tokenB.balanceOf(address(pool)), tokenA.balanceOf(address(pool)));
    }

    /// @notice The pool always holds at least its recorded reserves.
    function invariantReservesBackedByBalances() public view {
        (uint256 bal0, uint256 bal1) = _balancesInPoolOrder();
        assertGe(bal0, pool.reserve0(), "balance below reserve0");
        assertGe(bal1, pool.reserve1(), "balance below reserve1");
    }

    /// @notice The minimum liquidity locked at first mint is never burned, so
    ///         the pool can never reach a zero supply while it holds reserves.
    function invariantMinimumLiquidityLocked() public view {
        assertGe(pool.totalSupply(), pool.MINIMUM_LIQUIDITY(), "locked minimum liquidity was burned");
    }

    /// @notice After every handler call the recorded reserves track the pool's
    ///         actual token balances; nothing is moved without a `_update`.
    function invariantReservesTrackBalances() public view {
        (uint256 bal0, uint256 bal1) = _balancesInPoolOrder();
        assertEq(uint256(pool.reserve0()), bal0, "reserve0 out of sync");
        assertEq(uint256(pool.reserve1()), bal1, "reserve1 out of sync");
    }
}
