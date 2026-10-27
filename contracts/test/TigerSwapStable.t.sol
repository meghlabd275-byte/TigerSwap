// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "forge-std/Test.sol";
import "../TigerSwapStable.sol";
import "./TestERC20.sol";

contract TigerSwapStableTest is Test {
    TigerSwapStable public pool;
    TestERC20 public rusd;
    TestERC20 public usdt;
    TestERC20 public usdc;

    address public lp = address(0x11);
    address public trader = address(0x22);

    uint256 constant A = 200;
    /// @dev 0.04% fee (4e6 / 1e10).
    uint256 constant FEE = 4e6;
    uint256 constant ADMIN_FEE = 5e9;

    function setUp() public {
        rusd = new TestERC20("RUSD", "RUSD");
        usdt = new TestERC20("USDT", "USDT");
        usdc = new TestERC20("USDC", "USDC");

        address[2] memory coins = [address(rusd), address(usdt)];
        pool = new TigerSwapStable(coins, A, FEE, ADMIN_FEE);

        for (uint256 i = 0; i < 2; i++) {
            address who = [lp, trader][i];
            rusd.mint(who, 10_000_000e18);
            usdt.mint(who, 10_000_000e18);
            usdc.mint(who, 10_000_000e18);
            vm.startPrank(who);
            rusd.approve(address(pool), type(uint256).max);
            usdt.approve(address(pool), type(uint256).max);
            usdc.approve(address(pool), type(uint256).max);
            vm.stopPrank();
        }

        // Seed a balanced pool.
        vm.prank(lp);
        pool.addLiquidity(_two(1_000_000e18, 1_000_000e18), 0);
    }


    /// @dev Builds a uint256[2] from two values. Solidity infers the smallest
    ///      type for a bare array literal, which does not match uint256[2].
    function _two(uint256 a, uint256 b) internal pure returns (uint256[2] memory out) {
        out[0] = a;
        out[1] = b;
        return out;
    }

    // ---- Construction ------------------------------------------------------

    function testConstructorRejectsBadInput() public {
        address[2] memory dup = [address(rusd), address(rusd)];
        vm.expectRevert("TigerSwap: DUPLICATE_COIN");
        new TigerSwapStable(dup, A, FEE, ADMIN_FEE);

        address[2] memory zero = [address(0), address(usdt)];
        vm.expectRevert("TigerSwap: ZERO_COIN");
        new TigerSwapStable(zero, A, FEE, ADMIN_FEE);

        address[2] memory ok = [address(rusd), address(usdt)];
        vm.expectRevert("TigerSwap: ZERO_A");
        new TigerSwapStable(ok, 0, FEE, ADMIN_FEE);

        vm.expectRevert("TigerSwap: FEE_TOO_HIGH");
        new TigerSwapStable(ok, A, 1e9 + 1, ADMIN_FEE);
    }

    function testRatesDefaultToOne() public view {
        assertEq(pool.rates(0), 1e18, "coin0 rate");
        assertEq(pool.rates(1), 1e18, "coin1 rate");
    }

    // ---- Liquidity ---------------------------------------------------------

    function testInitialLiquidityMintsInvariant() public view {
        // For a balanced 2-coin pool at 1:1 rates, D is very close to the sum.
        uint256 supply = pool.totalSupply();
        assertApproxEqRel(supply, 2_000_000e18, 0.001e18, "supply near the sum");
    }

    function testAddLiquidityProportionalIsAlmostFeeFree() public {
        uint256 supplyBefore = pool.totalSupply();
        vm.prank(lp);
        uint256 mint = pool.addLiquidity(_two(100_000e18, 100_000e18), 0);

        // A proportional deposit should earn roughly its share of the supply.
        assertApproxEqRel(mint, supplyBefore * 100_000e18 / 1_000_000e18, 0.01e18, "proportional mint");
    }

    function testAddLiquidityImbalancedChargesFee() public {
        uint256 supplyBefore = pool.totalSupply();
        vm.prank(lp);
        uint256 mint = pool.addLiquidity(_two(100_000e18, 0), 0);

        // Depositing one-sided must earn less than the naive proportional share,
        // because the pool charges for the imbalance.
        uint256 naive = supplyBefore * 100_000e18 / 1_000_000e18;
        assertLt(mint, naive, "imbalanced deposit earns less");
        assertGt(mint, 0, "still mints something");
    }

    function testRemoveLiquidityReturnsProportionalAmounts() public {
        uint256 amount = 100_000e18;
        vm.prank(lp);
        uint256[2] memory received = pool.removeLiquidity(amount, _two(0, 0));

        // 100k LP of a ~2M supply is ~5%, so each 1M reserve returns ~50k and
        // the two legs together return exactly the LP amount burned.
        assertApproxEqRel(received[0], amount / 2, 0.01e18, "coin0 proportional");
        assertApproxEqRel(received[1], amount / 2, 0.01e18, "coin1 proportional");
        assertApproxEqRel(received[0] + received[1], amount, 0.01e18, "legs sum to the burn");
        assertEq(pool.totalSupply(), 2_000_000e18 - amount, "supply burned");
    }

    function testRemoveLiquidityRespectsMinAmounts() public {
        vm.prank(lp);
        vm.expectRevert("TigerSwap: SLIPPAGE");
        pool.removeLiquidity(100_000e18, _two(200_000e18, 0));
    }

    function testRemoveLiquidityOneCoinPaysOut() public {
        vm.prank(lp);
        uint256 out = pool.removeLiquidityOneCoin(100_000e18, 0, 0);
        assertGt(out, 0, "received coin0");
        assertLt(out, 100_000e18, "single-sided exit pays a fee");
    }

    function testCalcTokenAmountMatchesDeposit() public {
        uint256 predicted = pool.calcTokenAmount(_two(100_000e18, 100_000e18), true);
        vm.prank(lp);
        uint256 actual = pool.addLiquidity(_two(100_000e18, 100_000e18), 0);
        assertApproxEqRel(actual, predicted, 0.001e18, "calc matches execution");
    }

    // ---- Swapping ----------------------------------------------------------

    function testStableSwapIsNearlyParity() public {
        // With 1M/1M and A=200, a 1000 swap should return nearly 1000 minus fee.
        uint256 dx = 1_000e18;
        uint256 dy = pool.getDy(0, 1, dx);

        assertLt(dy, dx, "fee is taken");
        // Slippage must be tiny for a stable pool: well under 1%.
        assertGt(dy, dx * 99 / 100, "near parity output");
    }

    function testStableSwapBeatsConstantProduct() public {
        uint256 dx = 50_000e18;
        uint256 dy = pool.getDy(0, 1, dx);

        // Constant product x*y=k would give x*y/(x+dx) out.
        uint256 cpOut = 1_000_000e18 * 1_000_000e18 / (1_000_000e18 + dx);
        cpOut = 1_000_000e18 - cpOut;

        assertGt(dy, cpOut, "stable invariant gives a better price");
    }

    function testQuoteMatchesExecution() public {
        uint256 dx = 10_000e18;
        uint256 quoted = pool.getDy(0, 1, dx);

        vm.prank(trader);
        uint256 actual = pool.exchange(0, 1, dx, 0);

        assertEq(actual, quoted, "quote equals execution");
    }

    function testExchangeTransfersTokens() public {
        uint256 dx = 10_000e18;
        uint256 balBefore = usdt.balanceOf(trader);

        vm.prank(trader);
        uint256 dy = pool.exchange(0, 1, dx, 0);

        assertEq(usdt.balanceOf(trader), balBefore + dy, "trader received usdt");
        assertEq(rusd.balanceOf(address(pool)), 1_000_000e18 + dx, "pool took rusd");
    }

    function testExchangeRejectsSlippage() public {
        uint256 dy = pool.getDy(0, 1, 1_000e18);
        vm.prank(trader);
        vm.expectRevert("TigerSwap: SLIPPAGE");
        pool.exchange(0, 1, 1_000e18, dy + 1);
    }

    function testExchangeRejectsSameCoin() public {
        vm.prank(trader);
        vm.expectRevert("TigerSwap: SAME_COIN");
        pool.exchange(0, 0, 1_000e18, 0);
    }

    function testExchangeRejectsBadIndex() public {
        vm.prank(trader);
        vm.expectRevert("TigerSwap: INVALID_INDEX");
        pool.exchange(0, 2, 1_000e18, 0);
    }

    function testExchangeRejectsZeroAmount() public {
        vm.prank(trader);
        vm.expectRevert("TigerSwap: ZERO_AMOUNT");
        pool.exchange(0, 1, 0, 0);
    }

    function testLargeTradeMovesPriceAsExpected() public {
        // A large one-sided trade must move the price against the trader.
        uint256 smallOut = pool.getDy(0, 1, 1_000e18);
        uint256 largeOut = pool.getDy(0, 1, 500_000e18);

        uint256 smallPrice = smallOut * 1e18 / 1_000e18;
        uint256 largePrice = largeOut * 1e18 / 500_000e18;
        assertLt(largePrice, smallPrice, "bigger trade gets a worse unit price");
    }

    function testSwapDoesNotDrainThePool() public {
        // Repeated swaps in one direction must never empty the output reserve.
        for (uint256 i = 0; i < 10; i++) {
            vm.prank(trader);
            pool.exchange(0, 1, 50_000e18, 0);
        }
        assertGt(usdt.balanceOf(address(pool)), 0, "output reserve remains");
    }

    // ---- Fees --------------------------------------------------------------

    function testFeesAccrueToLps() public {
        uint256 vpBefore = pool.getVirtualPrice();

        vm.prank(trader);
        pool.exchange(0, 1, 100_000e18, 0);

        // A fee paid into a balanced pool raises the value of each LP token.
        assertGt(pool.getVirtualPrice(), vpBefore, "virtual price grew");
    }

    function testAdminFeesAreTrackedSeparately() public {
        vm.prank(trader);
        pool.exchange(0, 1, 100_000e18, 0);

        uint256 adminFees = pool.adminFees(1);
        assertGt(adminFees, 0, "admin accrued fees on the output coin");
    }

    function testAdminCanWithdrawFeesOnce() public {
        vm.prank(trader);
        pool.exchange(0, 1, 100_000e18, 0);

        uint256 accrued = pool.adminFees(1);
        uint256 ownerBefore = usdt.balanceOf(address(this));

        pool.withdrawAdminFees();

        assertEq(usdt.balanceOf(address(this)), ownerBefore + accrued, "owner received the fees");
        assertEq(pool.adminFees(1), 0, "fees cleared");

        // Nothing left to withdraw the second time.
        pool.withdrawAdminFees();
        assertEq(usdt.balanceOf(address(this)), ownerBefore + accrued, "no double withdrawal");
    }

    function testOnlyOwnerCanWithdrawFees() public {
        vm.prank(trader);
        vm.expectRevert("TigerSwap: NOT_OWNER");
        pool.withdrawAdminFees();
    }

    // ---- Rates -------------------------------------------------------------

    function testRatesAffectPricing() public {
        // Mark coin1 as worth twice coin0. A coin1 deposit is then worth more.
        uint256[2] memory newRates = _two(1e18, 2e18);
        pool.setRates(newRates);

        assertEq(pool.rates(1), 2e18, "rate stored");
        assertEq(pool.getA(), A, "A unaffected");
    }

    function testOnlyOwnerCanSetRates() public {
        vm.prank(trader);
        vm.expectRevert("TigerSwap: NOT_OWNER");
        pool.setRates(_two(1e18, 2e18));
    }

    function testSetRatesRejectsZero() public {
        vm.expectRevert("TigerSwap: ZERO_RATE");
        pool.setRates(_two(0, 1e18));
    }

    // ---- A ramp ------------------------------------------------------------

    function testRampAChangesEffectiveA() public {
        assertEq(pool.getA(), A, "A starts at the initial value");

        // A ramp cannot begin until the minimum duration has elapsed since the
        // last ramp point, which for a fresh pool is its construction time.
        uint256 t0 = block.timestamp + 2 days;
        vm.warp(t0);
        pool.rampA(400, t0 + 2 days);
        assertEq(pool.futureA(), 400, "future A stored");

        vm.warp(t0 + 1 days);
        uint256 midA = pool.getA();
        assertGt(midA, A, "A is ramping up");
        assertLt(midA, 400, "A has not arrived yet");

        vm.warp(t0 + 3 days);
        assertEq(pool.getA(), 400, "A reached the target");
    }

    function testRampAAffectsSwapPricing() public {
        uint256 t0 = block.timestamp + 2 days;
        vm.warp(t0);
        pool.rampA(1000, t0 + 2 days);
        vm.warp(t0 + 3 days);

        // With a much higher A the pool should be even closer to parity.
        uint256 dy = pool.getDy(0, 1, 10_000e18);
        assertGt(dy, 10_000e18 * 999 / 1000, "very close to parity at high A");
    }

    function testStopRampAFreezesA() public {
        uint256 t0 = block.timestamp + 2 days;
        vm.warp(t0);
        pool.rampA(400, t0 + 2 days);
        vm.warp(t0 + 1 days);

        pool.stopRampA();
        uint256 frozen = pool.getA();
        assertEq(pool.futureA(), frozen, "future A frozen");

        vm.warp(t0 + 20 days);
        assertEq(pool.getA(), frozen, "A stays frozen");
    }

    function testRampARejectsTooSoon() public {
        vm.expectRevert("TigerSwap: RAMP_NOT_READY");
        pool.rampA(400, block.timestamp + 2 days);
    }

    function testRampARejectsDurationTooShort() public {
        vm.warp(2 days);
        vm.expectRevert("TigerSwap: DURATION_TOO_SHORT");
        pool.rampA(400, vm.getBlockTimestamp() + 1 hours);
    }

    function testRampARejectsDurationTooLong() public {
        vm.warp(2 days);
        vm.expectRevert("TigerSwap: DURATION_TOO_LONG");
        pool.rampA(400, vm.getBlockTimestamp() + 10 days);
    }

    function testStopRampARejectsWhenNotRamping() public {
        vm.expectRevert("TigerSwap: NOT_RAMPING");
        pool.stopRampA();
    }

    function testOnlyOwnerCanRampA() public {
        uint256 t0 = block.timestamp + 2 days;
        vm.warp(t0);
        vm.prank(trader);
        vm.expectRevert("TigerSwap: NOT_OWNER");
        pool.rampA(400, t0 + 2 days);
    }

    // ---- ERC20 -------------------------------------------------------------

    function testLpTokenTransfersAndAllowances() public {
        vm.prank(lp);
        pool.transfer(trader, 1_000e18);
        assertEq(pool.balanceOf(trader), 1_000e18, "transferred");

        vm.prank(trader);
        pool.approve(lp, 500e18);
        vm.prank(lp);
        pool.transferFrom(trader, lp, 500e18);
        assertEq(pool.balanceOf(trader), 500e18, "spent allowance");
        assertEq(pool.allowance(trader, lp), 0, "allowance decremented");
    }

    function testTransferRejectsOverdraft() public {
        vm.prank(trader);
        vm.expectRevert("TigerSwap: INSUFFICIENT_BALANCE");
        pool.transfer(lp, 1e18);
    }

    function testApproveRejectsZeroSpender() public {
        vm.expectRevert("TigerSwap: ZERO_SPENDER");
        pool.approve(address(0), 1e18);
    }

    // ---- Ownership ---------------------------------------------------------

    function testTwoStepOwnershipTransfer() public {
        pool.commitTransferOwnership(trader);
        assertEq(pool.futureOwner(), trader, "pending owner set");

        vm.prank(trader);
        pool.acceptTransferOwnership();
        assertEq(pool.owner(), trader, "ownership transferred");
    }

    function testOnlyFutureOwnerCanAccept() public {
        pool.commitTransferOwnership(trader);
        vm.expectRevert("TigerSwap: NOT_FUTURE_OWNER");
        pool.acceptTransferOwnership();
    }

    function testSetFeeBounds() public {
        pool.setFee(1e6);
        assertEq(pool.fee(), 1e6, "fee updated");

        vm.expectRevert("TigerSwap: FEE_TOO_HIGH");
        pool.setFee(1e9);
    }

    function testSetAdminFeeBounds() public {
        pool.setAdminFee(1e9);
        assertEq(pool.adminFee(), 1e9, "admin fee updated");

        vm.expectRevert("TigerSwap: ADMIN_FEE_TOO_HIGH");
        pool.setAdminFee(1e10 + 1);
    }

    // ---- Invariants --------------------------------------------------------

    function testInvariantNeverDecreases() public {
        uint256 dBefore = pool.getVirtualPrice() * pool.totalSupply() / 1e18;

        vm.prank(trader);
        pool.exchange(0, 1, 100_000e18, 0);
        vm.prank(trader);
        pool.exchange(1, 0, 100_000e18, 0);

        uint256 dAfter = pool.getVirtualPrice() * pool.totalSupply() / 1e18;
        assertGe(dAfter, dBefore, "invariant is non-decreasing across trades");
    }

    function testPoolAlwaysSolvent() public {
        vm.prank(trader);
        pool.exchange(0, 1, 400_000e18, 0);

        // The pool must still hold at least what the LP shares are worth.
        uint256 value = pool.getVirtualPrice() * pool.totalSupply() / 1e18;
        assertLe(value, rusd.balanceOf(address(pool)) + usdt.balanceOf(address(pool)) + 1e6, "pool solvent");
    }
}
