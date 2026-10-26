// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "forge-std/Test.sol";
import "../TigerSwapMemeLaunchpad.sol";
import "../TigerSwapFactory.sol";

contract LaunchERC20 {
    string public name;
    string public symbol;
    uint8 public decimals = 18;
    uint256 public totalSupply;
    mapping(address => uint256) public balanceOf;
    mapping(address => mapping(address => uint256)) public allowance;

    event Transfer(address indexed from, address indexed to, uint256 value);
    event Approval(address indexed owner, address indexed spender, uint256 value);

    constructor(string memory _name, string memory _symbol) {
        name = _name;
        symbol = _symbol;
    }

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
        if (allowance[from][msg.sender] != type(uint256).max) {
            allowance[from][msg.sender] -= amount;
        }
        balanceOf[from] -= amount;
        balanceOf[to] += amount;
        emit Transfer(from, to, amount);
        return true;
    }
}

contract TigerSwapMemeLaunchpadTest is Test {
    TigerSwapMemeLaunchpad public pad;
    TigerSwapFactory public factory;
    LaunchERC20 public rusd;

    address public creator = address(0xC1);
    address public alice = address(0xA1);
    address public bob = address(0xB1);
    address public treasury = address(0x77);

    uint256 constant WAD = 1e18;

    function setUp() public {
        rusd = new LaunchERC20("RUSD", "RUSD");
        factory = new TigerSwapFactory();
        pad = new TigerSwapMemeLaunchpad(address(rusd), address(factory), treasury);

        for (uint256 i = 0; i < 3; i++) {
            address who = [creator, alice, bob][i];
            rusd.mint(who, 10_000_000e18);
            vm.prank(who);
            rusd.approve(address(pad), type(uint256).max);
        }
    }

    function _newLaunch() internal returns (uint256 id, address token) {
        vm.prank(creator);
        (id, token) = pad.createLaunch("Doge Killer", "DKILL", 3000);
    }

    // ---- Launch creation ---------------------------------------------------

    function testCreateLaunchMintsTokenAndRegisters() public {
        (uint256 id, address token) = _newLaunch();

        assertEq(pad.launchCount(), 1, "one launch");
        assertEq(pad.launchIdOf(token), id, "token maps to launch");
        assertTrue(pad.isLaunchToken(token), "marked as launch token");
        assertEq(TigerSwapMemeToken(token).totalSupply(), 1_000_000_000e18, "full supply minted to pad");
        assertEq(TigerSwapMemeToken(token).balanceOf(address(pad)), 1_000_000_000e18, "pad holds supply");
    }

    function testCreateLaunchRejectsEmptyMetadata() public {
        vm.prank(creator);
        vm.expectRevert("TigerSwap: EMPTY_METADATA");
        pad.createLaunch("", "X", 0);
    }

    function testCreateLaunchRejectsCreatorFeeAboveDenominator() public {
        vm.prank(creator);
        vm.expectRevert("TigerSwap: FEE_TOO_HIGH");
        pad.createLaunch("A", "B", 10_001);
    }

    function testCreatorFeeCanBeTheWholeTradeFee() public {
        vm.prank(creator);
        (uint256 id,) = pad.createLaunch("A", "B", 10_000);
        vm.prank(alice);
        pad.buy(id, 10_000e18, 0);

        (,,,,, uint256 creatorFees,,,) = pad.getLaunch(id);
        assertEq(creatorFees, 10_000e18 * 100 / 10_000, "creator takes the entire fee");
        assertEq(rusd.balanceOf(treasury), 0, "treasury gets nothing at 100%");
    }

    function testConstructorRejectsZeroAddresses() public {
        vm.expectRevert("TigerSwap: ZERO_QUOTE");
        new TigerSwapMemeLaunchpad(address(0), address(factory), treasury);
        vm.expectRevert("TigerSwap: ZERO_FACTORY");
        new TigerSwapMemeLaunchpad(address(rusd), address(0), treasury);
        vm.expectRevert("TigerSwap: ZERO_TREASURY");
        new TigerSwapMemeLaunchpad(address(rusd), address(factory), address(0));
    }

    // ---- Buying ------------------------------------------------------------

    function testBuyTransfersTokensAndTakesFee() public {
        (uint256 id, address token) = _newLaunch();

        uint256 quoteIn = 1_000e18;
        uint256 expectedOut = pad.tokensForQuote(id, quoteIn);

        vm.prank(alice);
        uint256 out = pad.buy(id, quoteIn, 0);

        assertEq(out, expectedOut, "matches the quote");
        assertEq(TigerSwapMemeToken(token).balanceOf(alice), out, "buyer received tokens");
        // 1% trade fee, of which the creator takes 30% and the treasury 70%.
        uint256 fee = quoteIn * 100 / 10_000;
        assertEq(rusd.balanceOf(treasury), fee - fee * 3000 / 10_000, "treasury fee");
    }

    function testBuyPriceRisesAlongTheCurve() public {
        (uint256 id,) = _newLaunch();

        uint256 p0 = pad.spotPrice(id);

        vm.prank(alice);
        pad.buy(id, 10_000e18, 0);
        uint256 p1 = pad.spotPrice(id);

        vm.prank(bob);
        pad.buy(id, 10_000e18, 0);
        uint256 p2 = pad.spotPrice(id);

        assertGt(p1, p0, "price rises after a buy");
        assertGt(p2, p1, "price rises again");
    }

    function testBuyRejectsSlippage() public {
        (uint256 id,) = _newLaunch();
        uint256 out = pad.tokensForQuote(id, 1_000e18);

        vm.prank(alice);
        vm.expectRevert("TigerSwap: SLIPPAGE");
        pad.buy(id, 1_000e18, out + 1);
    }

    function testBuyRejectsZeroAmount() public {
        (uint256 id,) = _newLaunch();
        vm.prank(alice);
        vm.expectRevert("TigerSwap: ZERO_AMOUNT");
        pad.buy(id, 0, 0);
    }

    function testBuyRejectsUnknownLaunch() public {
        vm.prank(alice);
        vm.expectRevert("TigerSwap: UNKNOWN_LAUNCH");
        pad.buy(99, 1_000e18, 0);
    }

    // ---- Selling -----------------------------------------------------------

    function testSellReturnsQuoteMinusFee() public {
        (uint256 id, address token) = _newLaunch();

        vm.prank(alice);
        uint256 bought = pad.buy(id, 10_000e18, 0);

        uint256 aliceBefore = rusd.balanceOf(alice);
        vm.prank(alice);
        TigerSwapMemeToken(token).approve(address(pad), type(uint256).max);
        vm.prank(alice);
        uint256 out = pad.sell(id, bought, 0);

        assertGt(out, 0, "seller received quote");
        // Round trip always loses the two fees.
        assertLt(rusd.balanceOf(alice), aliceBefore + 10_000e18, "fees paid on both legs");
    }

    function testSellNeverPaysOutMoreThanWasRaised() public {
        (uint256 id, address token) = _newLaunch();

        vm.prank(alice);
        uint256 bought = pad.buy(id, 10_000e18, 0);
        vm.prank(alice);
        TigerSwapMemeToken(token).approve(address(pad), type(uint256).max);

        (,,, uint256 realQuoteBefore,,,,,) = pad.getLaunch(id);

        vm.prank(alice);
        uint256 out = pad.sell(id, bought, 0);

        // A seller can only ever take back quote the curve actually holds, and
        // never more than they paid in (the fee is charged on both legs).
        assertLe(out, realQuoteBefore, "payout bounded by the real reserve");
        assertLt(out, 10_000e18, "round trip loses the fee");

        (,,, uint256 realQuoteAfter,,,,,) = pad.getLaunch(id);
        assertEq(realQuoteAfter, realQuoteBefore - out, "reserve debited by exactly the payout");
    }

    function testNobodyElseCanSellYourTokens() public {
        (uint256 id, address token) = _newLaunch();

        vm.prank(alice);
        uint256 bought = pad.buy(id, 1_000e18, 0);

        // Bob has no allowance over Alice's tokens, so the token transfer
        // fails before the curve is ever touched.
        vm.prank(bob);
        vm.expectRevert("TigerSwap: ALLOWANCE");
        pad.sell(id, bought, 0);
    }

    function testSellRejectsZeroAmount() public {
        (uint256 id,) = _newLaunch();
        vm.prank(alice);
        vm.expectRevert("TigerSwap: ZERO_AMOUNT");
        pad.sell(id, 0, 0);
    }

    // ---- Fees --------------------------------------------------------------

    function testCreatorFeesAccrueAndAreClaimable() public {
        (uint256 id,) = _newLaunch();

        vm.prank(alice);
        pad.buy(id, 100_000e18, 0);

        (,,,,, uint256 creatorFees,,,) = pad.getLaunch(id);
        assertGt(creatorFees, 0, "creator accrued fees");

        // Creator fee share is 3000/10000 of the 1% trade fee.
        assertEq(creatorFees, 100_000e18 * 100 / 10_000 * 3000 / 10_000, "exact creator cut");

        uint256 before = rusd.balanceOf(creator);
        vm.prank(creator);
        uint256 claimed = pad.claimCreatorFees(id);
        assertEq(claimed, creatorFees, "claimed the full amount");
        assertEq(rusd.balanceOf(creator), before + claimed, "quote received");
    }

    function testOnlyCreatorCanClaimFees() public {
        (uint256 id,) = _newLaunch();
        vm.prank(alice);
        pad.buy(id, 10_000e18, 0);

        vm.prank(alice);
        vm.expectRevert("TigerSwap: NOT_CREATOR");
        pad.claimCreatorFees(id);
    }

    function testCannotClaimFeesTwice() public {
        (uint256 id,) = _newLaunch();
        vm.prank(alice);
        pad.buy(id, 10_000e18, 0);

        vm.prank(creator);
        pad.claimCreatorFees(id);

        vm.prank(creator);
        vm.expectRevert("TigerSwap: NOTHING_TO_CLAIM");
        pad.claimCreatorFees(id);
    }

    // ---- Graduation --------------------------------------------------------

    function testGraduationCreatesPoolAndSeedsAtCurvePrice() public {
        (uint256 id, address token) = _newLaunch();

        // First buy stays below the target, which is measured on the quote that
        // actually reaches the curve (net of the trade fee).
        vm.prank(alice);
        pad.buy(id, 100_000e18, 0);
        (,,,,,,, bool before,) = pad.getLaunch(id);
        assertFalse(before, "not graduated below the target");
        uint256 curvePrice = pad.spotPrice(id);

        // A small final trade crosses the target, so the pool should open at
        // very nearly the price the curve was quoting.
        vm.prank(bob);
        pad.buy(id, 2_000e18, 0);

        (,,,,,,, bool graduated,) = pad.getLaunch(id);
        assertTrue(graduated, "curve graduated");

        address pool = factory.getPoolAddress(token, address(rusd));
        assertTrue(pool != address(0), "pool created");

        (uint112 r0, uint112 r1,) = ITigerSwapPool(pool).getReserves();
        uint256 tokenReserve = TigerSwapMemeToken(token).balanceOf(pool);
        uint256 quoteReserve = rusd.balanceOf(pool);
        assertEq(uint256(r0), tokenReserve, "reserve0 tracked");
        assertEq(uint256(r1), quoteReserve, "reserve1 tracked");

        // The pool price must continue from the curve price, not jump.
        uint256 poolPrice = quoteReserve * WAD / tokenReserve;
        assertApproxEqRel(poolPrice, curvePrice, 0.05e18, "pool opens at the curve price");
    }

    function testGraduationLocksLiquidityForever() public {
        (uint256 id, address token) = _newLaunch();
        vm.prank(alice);
        pad.buy(id, 120_000e18, 0);

        address pool = factory.getPoolAddress(token, address(rusd));
        // LP tokens are minted to the dead address, so they can never be burned.
        assertGt(IERC20(pool).balanceOf(address(0x000000000000000000000000000000000000dEaD)), 0, "lp locked");
    }

    function testGraduationBurnsUnseededSupply() public {
        (uint256 id, address token) = _newLaunch();
        vm.prank(alice);
        pad.buy(id, 120_000e18, 0);

        // The pad keeps nothing: tokens that only existed virtually are burned.
        assertEq(TigerSwapMemeToken(token).balanceOf(address(pad)), 0, "no dust left on the pad");
        assertEq(TigerSwapMemeToken(token).balanceOf(creator), 0, "creator gets no free allocation");
        assertEq(TigerSwapMemeToken(token).balanceOf(treasury), 0, "treasury gets no free allocation");

        // Every remaining token is either in the pool or held by a buyer, and
        // the burn really reduced the supply below the initial mint.
        uint256 poolBalance = TigerSwapMemeToken(token).balanceOf(factory.getPoolAddress(token, address(rusd)));
        uint256 circulating = TigerSwapMemeToken(token).balanceOf(alice);
        assertEq(TigerSwapMemeToken(token).totalSupply(), poolBalance + circulating, "supply is fully accounted");
        assertLt(TigerSwapMemeToken(token).totalSupply(), 1_000_000_000e18, "unseeded supply burned");
    }

    function testCannotTradeAfterGraduation() public {
        (uint256 id,) = _newLaunch();
        vm.prank(alice);
        pad.buy(id, 120_000e18, 0);

        vm.prank(bob);
        vm.expectRevert("TigerSwap: GRADUATED");
        pad.buy(id, 1_000e18, 0);
    }

    function testGraduateRejectsBeforeTarget() public {
        (uint256 id,) = _newLaunch();
        vm.prank(alice);
        pad.buy(id, 1_000e18, 0);

        vm.expectRevert("TigerSwap: TARGET_NOT_REACHED");
        pad.graduate(id);
    }

    function testGraduateRejectsTwice() public {
        (uint256 id,) = _newLaunch();
        vm.prank(alice);
        pad.buy(id, 120_000e18, 0);

        vm.expectRevert("TigerSwap: ALREADY_GRADUATED");
        pad.graduate(id);
    }

    function testTargetIsMeasuredOnNetProceeds() public {
        (uint256 id,) = _newLaunch();

        // A gross buy equal to the target nets only 99% of it, so the curve is
        // still open: the fee is not counted as raised liquidity.
        vm.prank(alice);
        pad.buy(id, 100_000e18, 0);
        (,,,,,,, bool graduated,) = pad.getLaunch(id);
        assertFalse(graduated, "gross target is not enough");

        // Anyone may then finish the job once the net target is reached.
        vm.prank(bob);
        pad.buy(id, 1_200e18, 0);
        (,,,,,,, graduated,) = pad.getLaunch(id);
        assertTrue(graduated, "net target reached");
    }

    // ---- Pausing -----------------------------------------------------------

    function testPauseBlocksTrading() public {
        (uint256 id,) = _newLaunch();
        pad.setPaused(true);

        vm.prank(alice);
        vm.expectRevert("TigerSwap: PAUSED");
        pad.buy(id, 1_000e18, 0);
    }

    function testOnlyOwnerCanPause() public {
        vm.prank(alice);
        vm.expectRevert("TigerSwap: NOT_OWNER");
        pad.setPaused(true);
    }

    // ---- Views -------------------------------------------------------------

    function testQuotesAreConsistentWithExecution() public {
        (uint256 id,) = _newLaunch();

        uint256 quoteIn = 5_000e18;
        uint256 quoted = pad.tokensForQuote(id, quoteIn);

        vm.prank(alice);
        uint256 actual = pad.buy(id, quoteIn, 0);

        assertEq(actual, quoted, "quote matches execution");
    }

    function testSpotPriceStartsAtConfiguredRatio() public {
        (uint256 id,) = _newLaunch();
        // 30_000 quote / 1_000_000_000 token = 3e-5 quote per token.
        assertEq(pad.spotPrice(id), 30_000e18 * WAD / 1_000_000_000e18, "initial price");
    }
}
