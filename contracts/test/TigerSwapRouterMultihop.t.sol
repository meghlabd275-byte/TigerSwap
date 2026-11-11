// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "forge-std/Test.sol";
import "../TigerSwapRouter.sol";
import "../TigerSwapFactory.sol";
import "../TigerSwapStable.sol";
import "../TigerSwapWeighted.sol";
import "./TestERC20.sol";

/// @dev Exercises the router's real multi-hop executor across pool families.
///      Every hop moves real tokens through a real pool; nothing is stubbed.
contract TigerSwapRouterMultihopTest is Test {
    TigerSwapRouter public router;
    TigerSwapFactory public factory;

    TestERC20 public tA;
    TestERC20 public tB;
    TestERC20 public tC;
    TestERC20 public sB;
    TestERC20 public sC;
    TestERC20 public wB;
    TestERC20 public wC;
    TestERC20 public WETH;

    address public user1 = address(0x1);
    address public user2 = address(0x2);

    function setUp() public {
        tA = new TestERC20("Token A", "TKA");
        tB = new TestERC20("Token B", "TKB");
        tC = new TestERC20("Token C", "TKC");
        sB = new TestERC20("Stable B", "SB");
        sC = new TestERC20("Stable C", "SC");
        wB = new TestERC20("Weighted B", "WB");
        wC = new TestERC20("Weighted C", "WC");
        WETH = new TestERC20("Wrapped Ether", "WETH");

        factory = new TigerSwapFactory();
        router = new TigerSwapRouter(address(factory), address(WETH));

        TestERC20[7] memory toks = [tA, tB, tC, sB, sC, wB, wC];
        for (uint256 i = 0; i < toks.length; i++) {
            toks[i].mint(user1, 10_000_000e18);
        }

        vm.startPrank(user1);
        for (uint256 i = 0; i < toks.length; i++) {
            toks[i].approve(address(router), type(uint256).max);
        }
        vm.stopPrank();
    }

    function _seedXyK(address a, address b, uint256 amountA, uint256 amountB) internal {
        vm.prank(user1);
        router.addLiquidity(a, b, amountA, amountB, 0, 0, user1, block.timestamp + 60);
    }

    /// @dev Deploys and seeds a real stable pool, then registers it by family.
    function _seedStable(address a, address b) internal returns (address pool) {
        address[2] memory coins = [a, b];
        pool = address(new TigerSwapStable(coins, 200, 4e6, 5e9));

        vm.startPrank(user1);
        TestERC20(a).approve(pool, type(uint256).max);
        TestERC20(b).approve(pool, type(uint256).max);
        uint256[2] memory amounts = [uint256(1_000_000e18), uint256(1_000_000e18)];
        TigerSwapStable(pool).addLiquidity(amounts, 0);
        vm.stopPrank();

        factory.registerFamilyPool(a, b, 2, pool);
    }

    /// @dev Deploys and seeds a real 50/50 weighted pool, then registers it.
    function _seedWeighted(address a, address b) internal returns (address pool) {
        address[] memory toks = new address[](2);
        toks[0] = a;
        toks[1] = b;
        uint256[] memory weights = new uint256[](2);
        weights[0] = 5e17;
        weights[1] = 5e17;
        pool = address(new TigerSwapWeightedPool(toks, weights, 30));

        vm.startPrank(user1);
        TestERC20(a).approve(pool, type(uint256).max);
        TestERC20(b).approve(pool, type(uint256).max);
        uint256[] memory amounts = new uint256[](2);
        amounts[0] = 1_000_000e18;
        amounts[1] = 1_000_000e18;
        TigerSwapWeightedPool(pool).addLiquidity(amounts, 0);
        vm.stopPrank();

        factory.registerFamilyPool(a, b, 3, pool);
    }

    function _one(uint256 v) internal pure returns (uint256[] memory out) {
        out = new uint256[](1);
        out[0] = v;
    }

    // ---- xyK -> xyK --------------------------------------------------------

    function testMultihopXyKToXyK() public {
        _seedXyK(address(tA), address(tB), 1000e18, 1000e18);
        _seedXyK(address(tB), address(tC), 1000e18, 1000e18);

        address[] memory tokens = new address[](3);
        tokens[0] = address(tA);
        tokens[1] = address(tB);
        tokens[2] = address(tC);
        uint8[] memory families = new uint8[](2);
        families[0] = 0;
        families[1] = 0;
        uint256[] memory mins = new uint256[](2);

        uint256 before = tC.balanceOf(user2);
        vm.prank(user1);
        uint256 out = router.swapExactTokensForTokensMultihop(
            100e18, 0, tokens, families, mins, user2, block.timestamp + 60
        );

        assertGt(out, 0);
        assertEq(tC.balanceOf(user2) - before, out);
        // The router must not retain the intermediate token.
        assertEq(tB.balanceOf(address(router)), 0);
    }

    // ---- xyK -> stable -----------------------------------------------------

    function testMultihopXyKToStable() public {
        _seedXyK(address(tA), address(sB), 1000e18, 1000e18);
        _seedStable(address(sB), address(sC));

        address[] memory tokens = new address[](3);
        tokens[0] = address(tA);
        tokens[1] = address(sB);
        tokens[2] = address(sC);
        uint8[] memory families = new uint8[](2);
        families[0] = 0;
        families[1] = 2;
        uint256[] memory mins = new uint256[](2);

        uint256 before = sC.balanceOf(user2);
        vm.prank(user1);
        uint256 out = router.swapExactTokensForTokensMultihop(
            100e18, 0, tokens, families, mins, user2, block.timestamp + 60
        );

        assertGt(out, 0);
        assertEq(sC.balanceOf(user2) - before, out);
        assertEq(sB.balanceOf(address(router)), 0);
    }

    // ---- xyK -> weighted ---------------------------------------------------

    function testMultihopXyKToWeighted() public {
        _seedXyK(address(tA), address(wB), 1000e18, 1000e18);
        _seedWeighted(address(wB), address(wC));

        address[] memory tokens = new address[](3);
        tokens[0] = address(tA);
        tokens[1] = address(wB);
        tokens[2] = address(wC);
        uint8[] memory families = new uint8[](2);
        families[0] = 0;
        families[1] = 3;
        uint256[] memory mins = new uint256[](2);

        uint256 before = wC.balanceOf(user2);
        vm.prank(user1);
        uint256 out = router.swapExactTokensForTokensMultihop(
            100e18, 0, tokens, families, mins, user2, block.timestamp + 60
        );

        assertGt(out, 0);
        assertEq(wC.balanceOf(user2) - before, out);
        assertEq(wB.balanceOf(address(router)), 0);
    }

    // ---- three hops --------------------------------------------------------

    function testMultihopThreeHops() public {
        _seedXyK(address(tA), address(tB), 1000e18, 1000e18);
        _seedXyK(address(tB), address(tC), 1000e18, 1000e18);
        _seedStable(address(tC), address(sC));

        address[] memory tokens = new address[](4);
        tokens[0] = address(tA);
        tokens[1] = address(tB);
        tokens[2] = address(tC);
        tokens[3] = address(sC);
        uint8[] memory families = new uint8[](3);
        families[0] = 0;
        families[1] = 0;
        families[2] = 2;
        uint256[] memory mins = new uint256[](3);

        uint256 before = sC.balanceOf(user2);
        vm.prank(user1);
        uint256 out = router.swapExactTokensForTokensMultihop(
            50e18, 0, tokens, families, mins, user2, block.timestamp + 60
        );

        assertGt(out, 0);
        assertEq(sC.balanceOf(user2) - before, out);
        assertEq(tB.balanceOf(address(router)), 0);
        assertEq(tC.balanceOf(address(router)), 0);
    }

    // ---- guards ------------------------------------------------------------

    function testMultihopRejectsPathLengthMismatch() public {
        _seedXyK(address(tA), address(tB), 1000e18, 1000e18);

        address[] memory tokens = new address[](3);
        tokens[0] = address(tA);
        tokens[1] = address(tB);
        tokens[2] = address(tC);
        uint8[] memory families = new uint8[](1);
        uint256[] memory mins = new uint256[](1);

        vm.prank(user1);
        vm.expectRevert(bytes("TigerSwap: PATH_LENGTH_MISMATCH"));
        router.swapExactTokensForTokensMultihop(
            100e18, 0, tokens, families, mins, user2, block.timestamp + 60
        );
    }

    function testMultihopRejectsPerHopSlippage() public {
        _seedXyK(address(tA), address(tB), 1000e18, 1000e18);
        _seedXyK(address(tB), address(tC), 1000e18, 1000e18);

        address[] memory tokens = new address[](3);
        tokens[0] = address(tA);
        tokens[1] = address(tB);
        tokens[2] = address(tC);
        uint8[] memory families = new uint8[](2);
        uint256[] memory mins = new uint256[](2);
        // An impossible first-hop minimum must revert the whole swap.
        mins[0] = 1000e18;

        vm.prank(user1);
        vm.expectRevert(bytes("TigerSwap: INSUFFICIENT_OUTPUT_AMOUNT"));
        router.swapExactTokensForTokensMultihop(
            100e18, 0, tokens, families, mins, user2, block.timestamp + 60
        );
    }

    function testMultihopRejectsExpiredDeadline() public {
        _seedXyK(address(tA), address(tB), 1000e18, 1000e18);

        address[] memory tokens = new address[](2);
        tokens[0] = address(tA);
        tokens[1] = address(tB);
        uint8[] memory families = new uint8[](1);
        uint256[] memory mins = new uint256[](1);

        vm.prank(user1);
        vm.expectRevert(bytes("TigerSwap: EXPIRED"));
        router.swapExactTokensForTokensMultihop(
            100e18, 0, tokens, families, mins, user2, block.timestamp - 1
        );
    }

    function testMultihopClearsPoolAllowance() public {
        _seedXyK(address(tA), address(sB), 1000e18, 1000e18);
        address stablePool = _seedStable(address(sB), address(sC));

        address[] memory tokens = new address[](3);
        tokens[0] = address(tA);
        tokens[1] = address(sB);
        tokens[2] = address(sC);
        uint8[] memory families = new uint8[](2);
        families[0] = 0;
        families[1] = 2;
        uint256[] memory mins = new uint256[](2);

        vm.prank(user1);
        router.swapExactTokensForTokensMultihop(100e18, 0, tokens, families, mins, user2, block.timestamp + 60);

        // No standing approval may survive a hop.
        assertEq(sB.allowance(address(router), stablePool), 0);
    }

    // ---- split order -------------------------------------------------------

    function testSplitOrderAcrossTwoXyKPools() public {
        // Two independent tA/tB pools cannot both exist under one factory, so the
        // split uses a direct tA->tB pool and a tA->tB pool reached via a
        // different family is not possible either. Instead split one order across
        // a direct tA->tC pool and a tA->tB->tC path, which are pool-disjoint.
        _seedXyK(address(tA), address(tC), 1000e18, 1000e18);
        _seedXyK(address(tA), address(tB), 1000e18, 1000e18);
        _seedXyK(address(tB), address(tC), 1000e18, 1000e18);

        address[][] memory tokens = new address[][](2);
        tokens[0] = new address[](2);
        tokens[0][0] = address(tA);
        tokens[0][1] = address(tC);
        tokens[1] = new address[](3);
        tokens[1][0] = address(tA);
        tokens[1][1] = address(tB);
        tokens[1][2] = address(tC);

        uint8[][] memory families = new uint8[][](2);
        families[0] = new uint8[](1);
        families[0][0] = 0;
        families[1] = new uint8[](2);
        families[1][0] = 0;
        families[1][1] = 0;

        uint256[] memory amounts = new uint256[](2);
        amounts[0] = 60e18;
        amounts[1] = 40e18;

        uint256[][] memory mins = new uint256[][](2);
        mins[0] = new uint256[](1);
        mins[1] = new uint256[](2);

        uint256 before = tC.balanceOf(user2);
        vm.prank(user1);
        uint256 out = router.swapExactTokensForTokensSplit(
            100e18, 0, tokens, families, amounts, mins, user2, block.timestamp + 60
        );

        assertGt(out, 0);
        assertEq(tC.balanceOf(user2) - before, out);
        assertEq(tB.balanceOf(address(router)), 0);
    }

    function testSplitOrderRejectsInputMismatch() public {
        _seedXyK(address(tA), address(tC), 1000e18, 1000e18);

        address[][] memory tokens = new address[][](1);
        tokens[0] = new address[](2);
        tokens[0][0] = address(tA);
        tokens[0][1] = address(tC);
        uint8[][] memory families = new uint8[][](1);
        families[0] = new uint8[](1);
        uint256[] memory amounts = new uint256[](1);
        amounts[0] = 50e18;
        uint256[][] memory mins = new uint256[][](1);
        mins[0] = new uint256[](1);

        vm.prank(user1);
        vm.expectRevert(bytes("TigerSwap: INPUT_MISMATCH"));
        router.swapExactTokensForTokensSplit(
            100e18, 0, tokens, families, amounts, mins, user2, block.timestamp + 60
        );
    }

    function testSplitOrderRejectsLegLengthMismatch() public {
        _seedXyK(address(tA), address(tC), 1000e18, 1000e18);

        address[][] memory tokens = new address[][](1);
        tokens[0] = new address[](2);
        tokens[0][0] = address(tA);
        tokens[0][1] = address(tC);
        uint8[][] memory families = new uint8[][](1);
        families[0] = new uint8[](1);
        // Two inputs for one leg.
        uint256[] memory amounts = new uint256[](2);
        amounts[0] = 50e18;
        amounts[1] = 50e18;
        uint256[][] memory mins = new uint256[][](1);
        mins[0] = new uint256[](1);

        vm.prank(user1);
        vm.expectRevert(bytes("TigerSwap: LEG_LENGTH_MISMATCH"));
        router.swapExactTokensForTokensSplit(
            100e18, 0, tokens, families, amounts, mins, user2, block.timestamp + 60
        );
    }
}
