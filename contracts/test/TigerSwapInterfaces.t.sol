// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "forge-std/Test.sol";
import "../TigerSwapInterfaces.sol";
import "../TigerSwapFactory.sol";
import "../TigerSwapConcentrated.sol";
import "../TigerSwapStable.sol";
import "../TigerSwapWeighted.sol";
import "./TestERC20.sol";

/// @notice Pins the shared interfaces to the concrete contracts they describe.
///
/// The interfaces in `TigerSwapInterfaces.sol` are what off-chain clients and
/// other contracts compile against, so a signature drift between an interface
/// and its implementation is a silent breakage: the call encodes to a selector
/// the contract does not have. Each test binds a live deployment to the
/// interface and exercises a call, so a mismatch fails to compile or reverts.
contract TigerSwapInterfacesTest is Test {
    uint160 constant Q96 = 0x1000000000000000000000000;

    TestERC20 tokenA;
    TestERC20 tokenB;

    address alice = address(0xA11CE);

    function setUp() public {
        tokenA = new TestERC20("AAA", "AAA");
        tokenB = new TestERC20("BBB", "BBB");
        tokenA.mint(address(this), 10_000_000e18);
        tokenB.mint(address(this), 10_000_000e18);
        tokenA.approve(address(this), type(uint256).max);
        tokenB.approve(address(this), type(uint256).max);
    }

    function testFactoryInterfaceMatchesImplementation() public {
        TigerSwapFactory factory = new TigerSwapFactory();
        ITigerSwapFactory viaInterface = ITigerSwapFactory(address(factory));

        address pool = viaInterface.createPool(address(tokenA), address(tokenB));
        assertTrue(pool != address(0));

        assertEq(viaInterface.getPoolAddress(address(tokenA), address(tokenB)), pool);
        assertEq(viaInterface.getPoolForFamily(address(tokenA), address(tokenB), 0), pool);
        assertEq(viaInterface.swapFee(), 30);
        assertEq(viaInterface.protocolFee(), 2);
        assertEq(viaInterface.feeTo(), address(0));
    }

    function testPoolInterfaceMatchesImplementation() public {
        TigerSwapFactory factory = new TigerSwapFactory();
        ITigerSwapFactory viaInterface = ITigerSwapFactory(address(factory));
        address poolAddr = viaInterface.createPool(address(tokenA), address(tokenB));
        ITigerSwapPool pool = ITigerSwapPool(poolAddr);

        // The pool orders its tokens by address, so compare against the sorted
        // pair rather than assuming the argument order.
        (address expected0, address expected1) = address(tokenA) < address(tokenB)
            ? (address(tokenA), address(tokenB))
            : (address(tokenB), address(tokenA));
        assertEq(pool.token0(), expected0);
        assertEq(pool.token1(), expected1);

        (uint112 reserve0, uint112 reserve1, uint32 lastTimestamp) = pool.getReserves();
        assertEq(uint256(reserve0), 0);
        assertEq(uint256(reserve1), 0);
        assertEq(uint256(lastTimestamp), 0);

        tokenA.transfer(poolAddr, 1_000e18);
        tokenB.transfer(poolAddr, 1_000e18);
        uint256 liquidity = pool.mint(alice);
        assertGt(liquidity, 0);

        // `burn` settles the caller's own LP balance, so move it back to the
        // pool first.
        vm.prank(alice);
        IERC20(poolAddr).transfer(poolAddr, liquidity);

        uint256 before = tokenA.balanceOf(address(this));
        (uint256 amount0, uint256 amount1) = pool.burn(address(this));
        assertGt(amount0, 0);
        assertGt(amount1, 0);
        assertGt(tokenA.balanceOf(address(this)), before);
    }

    function testPoolSwapInterfaceMatchesImplementation() public {
        TigerSwapFactory factory = new TigerSwapFactory();
        address poolAddr = ITigerSwapFactory(address(factory)).createPool(address(tokenA), address(tokenB));
        ITigerSwapPool pool = ITigerSwapPool(poolAddr);

        tokenA.transfer(poolAddr, 1_000e18);
        tokenB.transfer(poolAddr, 1_000e18);
        pool.mint(address(this));

        // The pool pays out token1, so the input has to be token0 whichever
        // token that turns out to be after the pool's address sort.
        TestERC20 inToken = TestERC20(pool.token0());
        TestERC20 outToken = TestERC20(pool.token1());
        inToken.transfer(poolAddr, 10e18);
        uint256 outBefore = outToken.balanceOf(address(this));
        pool.swap(0, 5e18, address(this), "");
        assertGt(outToken.balanceOf(address(this)), outBefore);
    }

    function testConcentratedInterfaceMatchesImplementation() public {
        TigerSwapFactory factory = new TigerSwapFactory();
        TigerSwapConcentrated concentrated =
            new TigerSwapConcentrated(address(factory), address(tokenA), address(tokenB), 3000, 60);
        ITigerSwapConcentratedPool viaInterface = ITigerSwapConcentratedPool(address(concentrated));

        vm.prank(address(factory));
        concentrated.initialize(Q96);

        assertEq(viaInterface.token0(), address(tokenA));
        assertEq(viaInterface.token1(), address(tokenB));
        assertEq(viaInterface.sqrtPriceX96(), Q96);
        assertEq(viaInterface.liquidity(), 0);

        tokenA.approve(address(concentrated), type(uint256).max);
        tokenB.approve(address(concentrated), type(uint256).max);
        concentrated.mint(address(this), -600, 600, 1e18);

        (int256 amount0, int256 amount1) = viaInterface.swap(address(this), true, 1e15, 0);
        // A zeroForOne swap takes token0 in and pays token1 out: amount0 > 0,
        // amount1 < 0.
        assertGt(amount0, 0);
        assertLt(amount1, 0);
    }

    function testStableInterfaceMatchesImplementation() public {
        address[2] memory coins = [address(tokenA), address(tokenB)];
        TigerSwapStable stable = new TigerSwapStable(coins, 100, 1, 0);
        ITigerSwapStablePool viaInterface = ITigerSwapStablePool(address(stable));

        assertEq(viaInterface.coins(0), address(tokenA));
        assertEq(viaInterface.coins(1), address(tokenB));
        assertEq(viaInterface.A(), 100);

        tokenA.approve(address(stable), type(uint256).max);
        tokenB.approve(address(stable), type(uint256).max);
        uint256[2] memory amounts = [uint256(1_000_000e18), uint256(1_000_000e18)];
        uint256 minted = viaInterface.addLiquidity(amounts, 0);
        assertGt(minted, 0);
        assertEq(viaInterface.balances(0), 1_000_000e18);

        uint256 quoted = viaInterface.getDy(0, 1, 1_000e18);
        uint256 dy = viaInterface.exchange(0, 1, 1_000e18, 0);
        assertEq(dy, quoted);
        assertGt(dy, 0);
    }

    function testWeightedInterfaceMatchesImplementation() public {
        address[] memory tokenList = new address[](2);
        tokenList[0] = address(tokenA);
        tokenList[1] = address(tokenB);
        uint256[] memory weights = new uint256[](2);
        weights[0] = 5e17; // 50%
        weights[1] = 5e17;

        TigerSwapWeightedPool weighted = new TigerSwapWeightedPool(tokenList, weights, 30);
        ITigerSwapWeightedPool viaInterface = ITigerSwapWeightedPool(address(weighted));

        assertEq(viaInterface.tokenCount(), 2);
        assertEq(address(viaInterface.tokens(0)), address(tokenA));
        assertEq(address(viaInterface.tokens(1)), address(tokenB));

        uint256[] memory storedWeights = viaInterface.getWeights();
        assertEq(storedWeights[0], 5e17);
        assertEq(storedWeights[1], 5e17);

        tokenA.approve(address(weighted), type(uint256).max);
        tokenB.approve(address(weighted), type(uint256).max);
        uint256[] memory amounts = new uint256[](2);
        amounts[0] = 1_000_000e18;
        amounts[1] = 1_000_000e18;
        uint256 lpMinted = viaInterface.addLiquidity(amounts, 0);
        assertGt(lpMinted, 0);

        uint256[] memory balances = viaInterface.getBalances();
        assertEq(balances[0], 1_000_000e18);

        uint256 out = viaInterface.swap(0, 1, 1_000e18, 0);
        assertGt(out, 0);
    }
}
