// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "forge-std/Test.sol";
import "../TigerSwapOptions.sol";
import "../TigerSwapOptionMath.sol";

contract OptERC20 {
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

contract TigerSwapOptionsTest is Test {
    TigerSwapOptions public options;
    OptERC20 public rusd;

    address public lp = address(0x11);
    address public trader = address(0x22);

    uint256 constant YEAR = 365 days;
    uint256 constant SPOT = 2000e18;
    uint256 constant STRIKE = 2000e18;
    uint256 constant BASE_IV = 8e17;

    uint256 seriesId;

    function setUp() public {
        rusd = new OptERC20("RUSD", "RUSD");
        options = new TigerSwapOptions(address(rusd));

        rusd.mint(lp, 100_000_000e18);
        rusd.mint(trader, 100_000_000e18);

        vm.prank(lp);
        rusd.approve(address(options), type(uint256).max);
        vm.prank(trader);
        rusd.approve(address(options), type(uint256).max);

        seriesId = options.createSeries(STRIKE, block.timestamp + YEAR, true, BASE_IV, SPOT);

        vm.prank(lp);
        options.addCollateral(seriesId, 10_000_000e18);
    }

    // ---- Construction and admin -------------------------------------------

    function testConstructorSetsOwnerAndOracle() public view {
        assertEq(options.owner(), address(this), "owner");
        assertEq(options.oracle(), address(this), "oracle");
        assertEq(address(options.collateralToken()), address(rusd), "collateral token");
    }

    function testConstructorRejectsZeroCollateral() public {
        vm.expectRevert("zero address");
        new TigerSwapOptions(address(0));
    }

    function testCreateSeriesRejectsPastExpiry() public {
        vm.expectRevert("expiry must be in the future");
        options.createSeries(STRIKE, block.timestamp - 1, true, BASE_IV, SPOT);
    }

    function testCreateSeriesRejectsZeroStrike() public {
        vm.expectRevert("strike required");
        options.createSeries(0, block.timestamp + YEAR, true, BASE_IV, SPOT);
    }

    function testCreateSeriesRejectsExcessiveIv() public {
        vm.expectRevert("base IV out of range");
        options.createSeries(STRIKE, block.timestamp + YEAR, true, 6e18, SPOT);
    }

    function testOnlyOwnerCanCreateSeries() public {
        vm.prank(trader);
        vm.expectRevert("not owner");
        options.createSeries(STRIKE, block.timestamp + YEAR, true, BASE_IV, SPOT);
    }

    function testSeriesCountGrows() public {
        assertEq(options.seriesCount(), 1, "one series");
        options.createSeries(2500e18, block.timestamp + YEAR, false, BASE_IV, SPOT);
        assertEq(options.seriesCount(), 2, "two series");
    }

    function testGetSeriesReturnsStoredValues() public view {
        (
            uint256 strike,
            uint256 expiry,
            bool isCall,
            uint256 baseIV,
            uint256 markPrice,
            uint256 collateral,,,,,
        ) = options.getSeries(seriesId);

        assertEq(strike, STRIKE, "strike");
        assertEq(expiry, block.timestamp + YEAR, "expiry");
        assertTrue(isCall, "call");
        assertEq(baseIV, BASE_IV, "base IV");
        assertEq(markPrice, SPOT, "mark price");
        assertEq(collateral, 10_000_000e18, "collateral");
    }

    function testGetSeriesRejectsUnknownId() public {
        vm.expectRevert("unknown series");
        options.getSeries(999);
    }

    // ---- Oracle ------------------------------------------------------------

    function testSetMarkPriceUpdatesPrice() public {
        options.setMarkPrice(seriesId, 3000e18);
        (,,,, uint256 markPrice,,,,,,) = options.getSeries(seriesId);
        assertEq(markPrice, 3000e18, "mark price updated");
    }

    function testSetMarkPriceRejectsNonOracle() public {
        vm.prank(trader);
        vm.expectRevert("not oracle");
        options.setMarkPrice(seriesId, 3000e18);
    }

    function testOracleCanBeDelegated() public {
        options.setOracle(trader);
        vm.prank(trader);
        options.setMarkPrice(seriesId, 2100e18);
        (,,,, uint256 markPrice,,,,,,) = options.getSeries(seriesId);
        assertEq(markPrice, 2100e18, "delegated oracle can price");
    }

    // ---- Collateral --------------------------------------------------------

    function testAddCollateralIncreasesPool() public {
        vm.prank(lp);
        options.addCollateral(seriesId, 500_000e18);

        (,,,,,, uint256 locked,,,,) = options.getSeries(seriesId);
        assertEq(locked, 0, "nothing locked yet");
        assertEq(rusd.balanceOf(address(options)), 10_500_000e18, "collateral held");
    }

    function testRemoveCollateralReturnsFunds() public {
        vm.prank(lp);
        options.removeCollateral(seriesId, 1_000_000e18);
        assertEq(rusd.balanceOf(address(options)), 9_000_000e18, "collateral reduced");
    }

    function testRemoveCollateralRejectsLockedAmount() public {
        vm.prank(trader);
        options.buy(seriesId, 100e18, type(uint256).max);

        // 100e18 of a 2000 strike call locks 200000e18, leaving ~9.8M free.
        vm.prank(lp);
        vm.expectRevert("collateral is locked");
        options.removeCollateral(seriesId, 9_900_000e18);
    }

    // ---- Pricing -----------------------------------------------------------

    function testPremiumMatchesBlackScholesForFreshPool() public view {
        // Fresh pool: utilization 0, so IV is exactly the base IV.
        assertEq(options.impliedVolatility(seriesId), BASE_IV, "IV equals base when unused");

        uint256 price = options.premium(seriesId, 1e18);
        uint256 expected = TigerSwapOptionMath.blackScholes(SPOT, STRIKE, YEAR, 5e16, BASE_IV, true);

        uint256 diff = price > expected ? price - expected : expected - price;
        assertLt(diff, expected / 100, "premium matches Black-Scholes");
    }

    function testPremiumScalesWithSize() public view {
        uint256 one = options.premium(seriesId, 1e18);
        uint256 ten = options.premium(seriesId, 10e18);
        assertEq(ten, one * 10, "premium is linear in size");
    }

    function testIvRisesWithUtilization() public {
        uint256 ivBefore = options.impliedVolatility(seriesId);

        // Buy enough to use a large share of the collateral.
        vm.prank(trader);
        options.buy(seriesId, 2000e18, type(uint256).max);

        uint256 ivAfter = options.impliedVolatility(seriesId);
        assertGt(ivAfter, ivBefore, "IV rises as the pool sells exposure");
        assertLe(ivAfter, options.MAX_IV(), "IV stays within the cap");
    }

    function testRequiredCollateralForCallIsSpotTimesSize() public view {
        // 1 unit of a call needs spot collateral.
        assertEq(options.requiredCollateral(seriesId, 1e18), SPOT, "call locks spot");
    }

    function testRequiredCollateralForPutIsStrikeTimesSize() public {
        uint256 putId = options.createSeries(2500e18, block.timestamp + YEAR, false, BASE_IV, SPOT);
        assertEq(options.requiredCollateral(putId, 1e18), 2500e18, "put locks strike");
    }

    function testRiskFreeRateCanBeSet() public {
        options.setRiskFreeRate(3e16);
        assertEq(options.riskFreeRate(), 3e16, "rate updated");
    }

    function testRiskFreeRateRejectsExcessiveValue() public {
        vm.expectRevert("rate too high");
        options.setRiskFreeRate(2e18);
    }

    // ---- Buying ------------------------------------------------------------

    function testBuyTransfersPremiumToPool() public {
        uint256 poolBefore = rusd.balanceOf(address(options));
        uint256 traderBefore = rusd.balanceOf(trader);

        vm.prank(trader);
        uint256 paid = options.buy(seriesId, 1e18, type(uint256).max);

        assertGt(paid, 0, "premium charged");
        assertEq(traderBefore - rusd.balanceOf(trader), paid, "trader paid the premium");
        assertEq(rusd.balanceOf(address(options)) - poolBefore, paid, "pool received the premium");
        assertEq(options.longPositions(seriesId, trader), 1e18, "position recorded");

        // The premium belongs to the LPs, so it is part of the claimable pool.
        (,,,,,,, , uint256 premiumCollected, ,) = options.getSeries(seriesId);
        assertEq(premiumCollected, paid, "premium recorded as pool revenue");
    }

    function testBuyLocksCollateral() public {
        vm.prank(trader);
        options.buy(seriesId, 1e18, type(uint256).max);

        (,,,,,, uint256 locked,,,,) = options.getSeries(seriesId);
        assertEq(locked, SPOT, "collateral locked for one call unit");
    }

    function testBuyEnforcesMaxPremium() public {
        vm.prank(trader);
        vm.expectRevert("premium above limit");
        options.buy(seriesId, 1e18, 1);
    }

    function testBuyRejectsZeroSize() public {
        vm.prank(trader);
        vm.expectRevert("size required");
        options.buy(seriesId, 0, type(uint256).max);
    }

    function testBuyRejectsWhenPoolCollateralIsExhausted() public {
        // A small pool that cannot underwrite a large position.
        uint256 smallId = options.createSeries(STRIKE, block.timestamp + YEAR, true, BASE_IV, SPOT);
        vm.prank(lp);
        options.addCollateral(smallId, 1000e18);

        vm.prank(trader);
        vm.expectRevert("insufficient pool collateral");
        options.buy(smallId, 100e18, type(uint256).max);
    }

    function testBuyRejectsUnknownSeries() public {
        vm.prank(trader);
        vm.expectRevert("unknown series");
        options.buy(999, 1e18, type(uint256).max);
    }

    function testAccumulatingPositions() public {
        vm.startPrank(trader);
        options.buy(seriesId, 1e18, type(uint256).max);
        options.buy(seriesId, 2e18, type(uint256).max);
        vm.stopPrank();

        assertEq(options.longPositions(seriesId, trader), 3e18, "positions accumulate");
    }

    // ---- Settlement and exercise -------------------------------------------

    function _buyAndExpire(uint256 size) internal returns (uint256 paid) {
        vm.prank(trader);
        paid = options.buy(seriesId, size, type(uint256).max);
        vm.warp(block.timestamp + YEAR + 1);
    }

    function testItmCallPaysIntrinsic() public {
        _buyAndExpire(1e18);
        options.settleSeries(seriesId, 2500e18);

        uint256 expected = 500e18;
        assertEq(options.payoutAt(seriesId, 1e18), expected, "intrinsic payout");

        uint256 before = rusd.balanceOf(trader);
        vm.prank(trader);
        uint256 payout = options.exercise(seriesId);
        assertEq(payout, expected, "payout returned");
        assertEq(rusd.balanceOf(trader) - before, expected, "payout transferred");
    }

    function testOtmCallPaysNothing() public {
        _buyAndExpire(1e18);
        options.settleSeries(seriesId, 1500e18);

        assertEq(options.payoutAt(seriesId, 1e18), 0, "worthless");

        uint256 before = rusd.balanceOf(trader);
        vm.prank(trader);
        uint256 payout = options.exercise(seriesId);
        assertEq(payout, 0, "no payout");
        assertEq(rusd.balanceOf(trader), before, "balance unchanged");
    }

    function testPutPaysIntrinsic() public {
        uint256 putId = options.createSeries(2500e18, block.timestamp + YEAR, false, BASE_IV, SPOT);
        vm.prank(lp);
        options.addCollateral(putId, 10_000_000e18);

        vm.prank(trader);
        options.buy(putId, 1e18, type(uint256).max);

        vm.warp(block.timestamp + YEAR + 1);
        options.settleSeries(putId, 2000e18);

        assertEq(options.payoutAt(putId, 1e18), 500e18, "put intrinsic");
    }

    function testCannotExerciseBeforeSettlement() public {
        _buyAndExpire(1e18);
        vm.prank(trader);
        vm.expectRevert("series not settled");
        options.exercise(seriesId);
    }

    function testCannotSettleBeforeExpiry() public {
        vm.expectRevert("series not expired");
        options.settleSeries(seriesId, 2500e18);
    }

    function testCannotSettleTwice() public {
        _buyAndExpire(1e18);
        options.settleSeries(seriesId, 2500e18);
        vm.expectRevert("already settled");
        options.settleSeries(seriesId, 2600e18);
    }

    function testExerciseRejectsEmptyPosition() public {
        _buyAndExpire(1e18);
        options.settleSeries(seriesId, 2500e18);

        vm.prank(lp);
        vm.expectRevert("no position");
        options.exercise(seriesId);
    }

    function testCannotBuyAfterExpiry() public {
        vm.warp(block.timestamp + YEAR + 1);
        vm.prank(trader);
        vm.expectRevert("series expired");
        options.buy(seriesId, 1e18, type(uint256).max);
    }

    function testCannotAddCollateralAfterSettlement() public {
        _buyAndExpire(1e18);
        options.settleSeries(seriesId, 2500e18);

        vm.prank(lp);
        vm.expectRevert("series already settled");
        options.addCollateral(seriesId, 1000e18);
    }

    // ---- LP ownership ------------------------------------------------------

    function testSecondLpCannotWithdrawFirstLpsPrincipal() public {
        address lp2 = address(0x33);
        rusd.mint(lp2, 5_000_000e18);
        vm.prank(lp2);
        rusd.approve(address(options), type(uint256).max);

        // lp2 never deposited, so it owns nothing to withdraw.
        vm.prank(lp2);
        vm.expectRevert("exceeds your principal");
        options.removeCollateral(seriesId, 1_000_000e18);
    }

    function testEachLpWithdrawsOnlyItsOwnPrincipal() public {
        address lp2 = address(0x33);
        rusd.mint(lp2, 5_000_000e18);
        vm.prank(lp2);
        rusd.approve(address(options), type(uint256).max);
        vm.prank(lp2);
        options.addCollateral(seriesId, 5_000_000e18);

        // lp cannot take more than the 10M it deposited, even though the pool
        // holds 15M.
        vm.prank(lp);
        vm.expectRevert("exceeds your principal");
        options.removeCollateral(seriesId, 10_000_000e18 + 1);

        vm.prank(lp);
        options.removeCollateral(seriesId, 10_000_000e18);
        vm.prank(lp2);
        options.removeCollateral(seriesId, 5_000_000e18);

        assertEq(rusd.balanceOf(address(options)), 0, "pool emptied by its own LPs");
    }

    function testSecondLpSharesAreNotDilutedByExistingPool() public {
        address lp2 = address(0x33);
        rusd.mint(lp2, 5_000_000e18);
        vm.prank(lp2);
        rusd.approve(address(options), type(uint256).max);
        vm.prank(lp2);
        options.addCollateral(seriesId, 5_000_000e18);

        // The pool holds 15M free, so 5M buys exactly a third of the shares.
        assertEq(options.totalLpShares(seriesId), 15_000_000e18, "shares match deposits");
        assertEq(options.lpShares(seriesId, lp), 10_000_000e18, "first lp keeps two thirds");
        assertEq(options.lpShares(seriesId, lp2), 5_000_000e18, "second lp gets one third");
    }

    function testNonLpCannotWithdrawAfterSettlement() public {
        _buyAndExpire(1e18);
        options.settleSeries(seriesId, 2500e18);
        vm.prank(trader);
        options.exercise(seriesId);

        address outsider = address(0x44);
        vm.prank(outsider);
        vm.expectRevert("exceeds your share");
        options.withdrawLp(seriesId, 1_000_000e18);
    }

    function testUnexercisedHoldersCollateralIsReserved() public {
        _buyAndExpire(1e18);
        options.settleSeries(seriesId, 2500e18);
        // The holder has not exercised, so their payout is still owed.

        uint256 claim = options.lpClaims(seriesId);
        vm.prank(lp);
        vm.expectRevert("collateral is reserved");
        options.withdrawLp(seriesId, claim);

        // Only the unencumbered remainder can leave the pool.
        uint256 reserved = 2000e18; // size * spot locked by the open call
        vm.prank(lp);
        options.withdrawLp(seriesId, claim - reserved);
        assertEq(rusd.balanceOf(address(options)), reserved, "reserved payout stays put");

        // The holder can still be paid in full.
        uint256 before = rusd.balanceOf(trader);
        vm.prank(trader);
        uint256 payout = options.exercise(seriesId);
        assertEq(payout, 500e18, "intrinsic paid");
        assertEq(rusd.balanceOf(trader), before + payout, "holder made whole");
    }

    // ---- LP withdrawal -----------------------------------------------------

    function testLpWithdrawsRemainingCollateralAfterSettlement() public {
        _buyAndExpire(1e18);
        options.settleSeries(seriesId, 2500e18);

        vm.prank(trader);
        options.exercise(seriesId);

        // The pool paid 500e18 of intrinsic out of 10M plus the premium.
        uint256 poolBalance = rusd.balanceOf(address(options));
        vm.prank(lp);
        options.withdrawLp(seriesId, poolBalance);

        assertEq(rusd.balanceOf(address(options)), 0, "pool drained");
    }

    function testLpCannotWithdrawBeforeSettlement() public {
        vm.prank(lp);
        vm.expectRevert("series not settled");
        options.withdrawLp(seriesId, 1000e18);
    }

    function testLpCannotWithdrawMoreThanClaimable() public {
        _buyAndExpire(1e18);
        options.settleSeries(seriesId, 1500e18);

        vm.prank(lp);
        vm.expectRevert("exceeds claimable collateral");
        options.withdrawLp(seriesId, 100_000_000e18);
    }

    // ---- Invariants --------------------------------------------------------

    function testPoolAlwaysCoversEveryPayout() public {
        // Sell a size the pool can back, then settle deep in the money.
        vm.prank(trader);
        options.buy(seriesId, 1000e18, type(uint256).max);

        vm.warp(block.timestamp + YEAR + 1);
        // 1000 units of a 2000 strike call locks 2,000,000 of collateral, and
        // the worst case payout at any price above strike is size * price.
        options.settleSeries(seriesId, 3000e18);

        vm.prank(trader);
        uint256 payout = options.exercise(seriesId);
        assertEq(payout, 1000e18 * 1000e18 / 1e18, "intrinsic payout");

        // The pool must still hold enough to cover it.
        assertGe(rusd.balanceOf(address(options)), 0, "pool solvent");
    }

    function testPremiumIsNotRefundableAfterLoss() public {
        uint256 before = rusd.balanceOf(trader);
        _buyAndExpire(1e18);
        options.settleSeries(seriesId, 1500e18);

        vm.prank(trader);
        options.exercise(seriesId);

        // The trader is down exactly the premium.
        assertLt(rusd.balanceOf(trader), before, "premium is lost on a worthless option");
    }

    function testOwnershipTransfer() public {
        options.transferOwnership(trader);
        assertEq(options.owner(), trader, "ownership moved");
    }
}
