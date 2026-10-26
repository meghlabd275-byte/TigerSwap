// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "forge-std/Test.sol";
import "../TigerSwapWeighted.sol";

contract WERC20 {
    string public name = "Token";
    string public symbol = "TKN";
    uint8 public decimals = 18;
    uint256 public totalSupply;
    mapping(address => uint256) public balanceOf;
    mapping(address => mapping(address => uint256)) public allowance;

    event Transfer(address indexed from, address indexed to, uint256 value);
    event Approval(address indexed owner, address indexed spender, uint256 value);

    function mint(address to, uint256 amount) external {
        totalSupply += amount;
        balanceOf[to] += amount;
        emit Transfer(address(0), to, amount);
    }

    function approve(address spender, uint256 amount) external returns (bool) {
        allowance[msg.sender][spender] = amount;
        emit Approval(msg.sender, spender, amount);
        return true;
    }

    function transfer(address to, uint256 amount) external returns (bool) {
        balanceOf[msg.sender] -= amount;
        balanceOf[to] += amount;
        emit Transfer(msg.sender, to, amount);
        return true;
    }

    function transferFrom(address from, address to, uint256 amount) external returns (bool) {
        allowance[from][msg.sender] -= amount;
        balanceOf[from] -= amount;
        balanceOf[to] += amount;
        emit Transfer(from, to, amount);
        return true;
    }
}

contract WeightedPoolTest is Test {
    TigerSwapWeightedPool pool;
    WERC20 tgr;
    WERC20 rusd;
    WERC20 third;

    address alice = address(0xA11CE);
    address bob = address(0xB0B);

    function setUp() public {
        tgr = new WERC20();
        rusd = new WERC20();
        third = new WERC20();

        address[] memory toks = new address[](2);
        toks[0] = address(tgr);
        toks[1] = address(rusd);
        uint256[] memory weights = new uint256[](2);
        weights[0] = 0.8e18; // 80/20 TGR/RUSD
        weights[1] = 0.2e18;

        pool = new TigerSwapWeightedPool(toks, weights, 30); // 0.3%

        tgr.mint(alice, 1_000_000e18);
        rusd.mint(alice, 1_000_000e18);
        vm.startPrank(alice);
        tgr.approve(address(pool), type(uint256).max);
        rusd.approve(address(pool), type(uint256).max);
        vm.stopPrank();
    }

    function _seedPool() internal {
        uint256[] memory amounts = new uint256[](2);
        amounts[0] = 8000e18;
        amounts[1] = 2000e18;
        vm.prank(alice);
        pool.addLiquidity(amounts, 0);
    }

    // ---- Construction ------------------------------------------------------

    function testPoolRecordsTokensAndWeights() public view {
        assertEq(pool.tokenCount(), 2, "two tokens");
        assertEq(pool.weight(0), 0.8e18, "weight 0");
        assertEq(pool.weight(1), 0.2e18, "weight 1");
        assertEq(pool.feeBps(), 30, "fee");
    }

    function testWeightsMustSumToOne() public {
        address[] memory toks = new address[](2);
        toks[0] = address(tgr);
        toks[1] = address(rusd);
        uint256[] memory bad = new uint256[](2);
        bad[0] = 0.5e18;
        bad[1] = 0.4e18;
        vm.expectRevert("weights must sum to 1e18");
        new TigerSwapWeightedPool(toks, bad, 30);
    }

    function testDuplicateTokensRejected() public {
        address[] memory toks = new address[](2);
        toks[0] = address(tgr);
        toks[1] = address(tgr);
        uint256[] memory w = new uint256[](2);
        w[0] = 0.5e18;
        w[1] = 0.5e18;
        vm.expectRevert("duplicate token");
        new TigerSwapWeightedPool(toks, w, 30);
    }

    function testSingleTokenRejected() public {
        address[] memory toks = new address[](1);
        toks[0] = address(tgr);
        uint256[] memory w = new uint256[](1);
        w[0] = 1e18;
        vm.expectRevert("need at least two tokens");
        new TigerSwapWeightedPool(toks, w, 30);
    }

    function testWeightBelowMinimumRejected() public {
        address[] memory toks = new address[](2);
        toks[0] = address(tgr);
        toks[1] = address(rusd);
        uint256[] memory w = new uint256[](2);
        w[0] = 0.999e18;
        w[1] = 0.001e18;
        vm.expectRevert("weight below minimum");
        new TigerSwapWeightedPool(toks, w, 30);
    }

    function testFeeAboveCapRejected() public {
        address[] memory toks = new address[](2);
        toks[0] = address(tgr);
        toks[1] = address(rusd);
        uint256[] memory w = new uint256[](2);
        w[0] = 0.8e18;
        w[1] = 0.2e18;
        vm.expectRevert("fee too high");
        new TigerSwapWeightedPool(toks, w, 1001);
    }

    // ---- Invariant ---------------------------------------------------------

    function testInvariantIsPositiveForSeededPool() public {
        _seedPool();
        assertGt(pool.getInvariant(), 0, "invariant positive");
    }

    function testInvariantGrowsWhenLiquidityAdded() public {
        _seedPool();
        uint256 before = pool.getInvariant();

        uint256[] memory more = new uint256[](2);
        more[0] = 800e18;
        more[1] = 200e18;
        vm.prank(alice);
        pool.addLiquidity(more, 0);

        assertGt(pool.getInvariant(), before, "invariant grew");
    }

    function testInvariantScalesWithProportionalDeposit() public {
        _seedPool();
        uint256 invBefore = pool.getInvariant();

        // Double the pool proportionally; the invariant should roughly double.
        uint256[] memory more = new uint256[](2);
        more[0] = 8000e18;
        more[1] = 2000e18;
        vm.prank(alice);
        pool.addLiquidity(more, 0);

        uint256 invAfter = pool.getInvariant();
        assertApproxEqRel(invAfter, invBefore * 2, 0.02e18, "invariant doubles");
    }

    // ---- Liquidity ---------------------------------------------------------

    function testFirstDepositMintsShares() public {
        _seedPool();
        assertGt(pool.totalSupply(), 0, "shares minted");
        assertEq(pool.balanceOf(alice), pool.totalSupply(), "all shares to depositor");
        assertEq(pool.balance(0), 8000e18, "token0 held");
        assertEq(pool.balance(1), 2000e18, "token1 held");
    }

    function testRemoveLiquidityReturnsProportionalAmounts() public {
        _seedPool();
        uint256 shares = pool.balanceOf(alice);

        uint256[] memory minAmounts = new uint256[](2);
        vm.prank(alice);
        uint256[] memory out = pool.removeLiquidity(shares, minAmounts);

        assertEq(out[0], 8000e18, "all token0 returned");
        assertEq(out[1], 2000e18, "all token1 returned");
        assertEq(pool.totalSupply(), 0, "supply burned");
    }

    function testPartialRemovalIsProportional() public {
        _seedPool();
        uint256 shares = pool.balanceOf(alice) / 2;

        uint256[] memory minAmounts = new uint256[](2);
        vm.prank(alice);
        uint256[] memory out = pool.removeLiquidity(shares, minAmounts);

        assertEq(out[0], 4000e18, "half of token0");
        assertEq(out[1], 1000e18, "half of token1");
    }

    function testCannotRemoveMoreThanOwned() public {
        _seedPool();
        uint256[] memory minAmounts = new uint256[](2);
        vm.prank(bob);
        vm.expectRevert("insufficient shares");
        pool.removeLiquidity(1e18, minAmounts);
    }

    // ---- Swaps -------------------------------------------------------------

    function testSwapMovesTokensAndChargesFee() public {
        _seedPool();
        uint256 amountIn = 100e18;

        uint256 beforeOut = rusd.balanceOf(alice);
        vm.prank(alice);
        uint256 amountOut = pool.swap(0, 1, amountIn, 0);

        assertGt(amountOut, 0, "received output");
        assertEq(rusd.balanceOf(alice) - beforeOut, amountOut, "output delivered");
        assertEq(pool.balance(0), 8000e18 + amountIn, "token0 grew");
        assertEq(pool.balance(1), 2000e18 - amountOut, "token1 shrank");
    }

    function testSwapOutputIsLessThanFeeFreeIdeal() public {
        _seedPool();
        uint256 amountIn = 100e18;

        vm.prank(alice);
        uint256 amountOut = pool.swap(0, 1, amountIn, 0);

        // Reference value for an 80/20 pool with 8000/2000 reserves and 100 in:
        // fee-free output 96.9514, so 0.3% fee gives 96.6606.
        assertApproxEqRel(amountOut, 96.66e18, 0.01e18, "matches the Balancer closed form");
    }

    function testSwapRejectsSlippageBeyondMinimum() public {
        _seedPool();
        vm.prank(alice);
        vm.expectRevert("slippage exceeded");
        pool.swap(0, 1, 100e18, 1000e18);
    }

    function testSwapRejectsSameToken() public {
        _seedPool();
        vm.prank(alice);
        vm.expectRevert("same token");
        pool.swap(0, 0, 100e18, 0);
    }

    function testSwapRejectsBadIndex() public {
        _seedPool();
        vm.prank(alice);
        vm.expectRevert("bad index");
        pool.swap(0, 5, 100e18, 0);
    }

    function testSwapRejectsZeroAmount() public {
        _seedPool();
        vm.prank(alice);
        vm.expectRevert("amount required");
        pool.swap(0, 1, 0, 0);
    }

    function testSwapRevertsOnEmptyPool() public {
        vm.prank(alice);
        vm.expectRevert("zero balance");
        pool.swap(0, 1, 100e18, 0);
    }

    function testSwapBothDirectionsWork() public {
        _seedPool();

        vm.prank(alice);
        uint256 out1 = pool.swap(0, 1, 100e18, 0);
        assertGt(out1, 0, "0->1 works");

        vm.prank(alice);
        uint256 out2 = pool.swap(1, 0, 50e18, 0);
        assertGt(out2, 0, "1->0 works");
    }

    // ---- Spot price --------------------------------------------------------

    function testSpotPriceReflectsWeightRatio() public {
        _seedPool();
        // balances are in the 80/20 ratio, so price = (8000/0.8)/(2000/0.2) = 1
        uint256 price = pool.spotPrice(0, 1);
        assertApproxEqRel(price, 1e18, 0.01e18, "balanced pool prices at 1");
    }

    function testSpotPriceRisesWhenToken0BecomesScarce() public {
        _seedPool();
        uint256 before = pool.spotPrice(0, 1);

        // Buy token0 with token1, making token0 scarcer.
        vm.prank(alice);
        pool.swap(1, 0, 200e18, 0);

        assertGt(pool.spotPrice(0, 1), before, "scarcer token0 is more expensive");
    }

    function testSpotPriceRejectsEmptyPool() public {
        vm.expectRevert("empty pool");
        pool.spotPrice(0, 1);
    }

    // ---- ERC20 shares ------------------------------------------------------

    function testShareTokenTransfers() public {
        _seedPool();
        uint256 shares = pool.balanceOf(alice) / 2;

        vm.prank(alice);
        pool.transfer(bob, shares);

        assertEq(pool.balanceOf(bob), shares, "bob received shares");
    }

    function testShareTransferRejectsInsufficientBalance() public {
        _seedPool();
        vm.prank(bob);
        vm.expectRevert("insufficient balance");
        pool.transfer(alice, 1e18);
    }

    function testApproveAndTransferFromShares() public {
        _seedPool();
        uint256 shares = pool.balanceOf(alice) / 2;

        vm.prank(alice);
        pool.approve(bob, shares);

        vm.prank(bob);
        pool.transferFrom(alice, bob, shares);

        assertEq(pool.balanceOf(bob), shares, "transferred via allowance");
    }

    function testTransferFromRejectsInsufficientAllowance() public {
        _seedPool();
        vm.prank(bob);
        vm.expectRevert("insufficient allowance");
        pool.transferFrom(alice, bob, 1e18);
    }

    // ---- Fee admin ---------------------------------------------------------

    function testOwnerCanChangeFee() public {
        pool.setFee(100);
        assertEq(pool.feeBps(), 100, "fee updated");
    }

    function testFeeChangeRejectsAboveCap() public {
        vm.expectRevert("fee too high");
        pool.setFee(1001);
    }

    function testOnlyOwnerCanChangeFee() public {
        vm.prank(alice);
        vm.expectRevert("not owner");
        pool.setFee(100);
    }

    // ---- Three-token pool --------------------------------------------------

    function testThreeTokenPoolSupportsSwaps() public {
        address[] memory toks = new address[](3);
        toks[0] = address(tgr);
        toks[1] = address(rusd);
        toks[2] = address(third);
        uint256[] memory w = new uint256[](3);
        w[0] = 0.5e18;
        w[1] = 0.3e18;
        w[2] = 0.2e18;

        TigerSwapWeightedPool p3 = new TigerSwapWeightedPool(toks, w, 30);
        third.mint(alice, 1_000_000e18);

        vm.startPrank(alice);
        // Every pool pulls its own allowance, so approve all three tokens here.
        tgr.approve(address(p3), type(uint256).max);
        rusd.approve(address(p3), type(uint256).max);
        third.approve(address(p3), type(uint256).max);
        uint256[] memory amounts = new uint256[](3);
        amounts[0] = 5000e18;
        amounts[1] = 3000e18;
        amounts[2] = 2000e18;
        p3.addLiquidity(amounts, 0);

        uint256 out = p3.swap(0, 2, 100e18, 0);
        assertGt(out, 0, "three-token swap works");
        vm.stopPrank();
    }
}
