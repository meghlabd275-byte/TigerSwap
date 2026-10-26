// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "forge-std/Test.sol";
import "../TigerSwapOptionMath.sol";

/// @dev Exercises the Black-Scholes library directly against values computed
/// independently with Python's math.erf-based reference, so a regression in the
/// fixed point series is caught here rather than in the option AMM.
/// @dev Thin external wrappers so `vm.expectRevert` can observe library reverts.
contract OptionMathHarness {
    function ln(uint256 x) external pure returns (int256) {
        return TigerSwapOptionMath.ln(x);
    }

    function exp(int256 x) external pure returns (uint256) {
        return TigerSwapOptionMath.exp(x);
    }

    function invSqrt(uint256 x) external pure returns (uint256) {
        return TigerSwapOptionMath.invSqrt(x);
    }

    function price(uint256 s, uint256 k, uint256 t, uint256 r, uint256 v, bool call) external pure returns (uint256) {
        return TigerSwapOptionMath.blackScholes(s, k, t, r, v, call);
    }
}

contract TigerSwapOptionMathTest is Test {
    uint256 constant YEAR = 365 days;
    OptionMathHarness harness = new OptionMathHarness();

    function _assertClose(uint256 got, uint256 want, uint256 toleranceBps, string memory label) internal pure {
        uint256 diff = got > want ? got - want : want - got;
        uint256 allowed = want * toleranceBps / 10000;
        if (allowed == 0) {
            allowed = 1;
        }
        require(diff <= allowed, string.concat(label, ": value outside tolerance"));
    }

    // ---- ln -----------------------------------------------------------------

    function testLnOfOne() public pure {
        int256 result = TigerSwapOptionMath.ln(1e18);
        assertEq(result, 0, "ln(1) = 0");
    }

    function testLnOfTwo() public pure {
        int256 result = TigerSwapOptionMath.ln(2e18);
        // ln(2) = 0.693147180559945309
        int256 expected = 693147180559945309;
        int256 diff = result > expected ? result - expected : expected - result;
        assertLt(uint256(diff), 1e9, "ln(2) accurate to 1e-9");
    }

    function testLnOfE() public pure {
        // ln(e) = 1, e = 2.718281828459045235
        int256 result = TigerSwapOptionMath.ln(2718281828459045235);
        int256 expected = 1e18;
        int256 diff = result > expected ? result - expected : expected - result;
        assertLt(uint256(diff), 1e10, "ln(e) accurate to 1e-8");
    }

    function testLnHandlesSmallNumbers() public pure {
        // ln(0.5) = -0.693147...
        int256 result = TigerSwapOptionMath.ln(5e17);
        int256 expected = -693147180559945309;
        int256 diff = result > expected ? result - expected : expected - result;
        assertLt(uint256(diff), 1e9, "ln(0.5) accurate");
    }

    function testLnRejectsZero() public {
        vm.expectRevert("ln of zero");
        harness.ln(0);
    }

    // ---- exp ----------------------------------------------------------------

    function testExpOfZero() public pure {
        assertEq(TigerSwapOptionMath.exp(0), 1e18, "exp(0) = 1");
    }

    function testExpOfOne() public pure {
        // exp(1) = 2.718281828459045235
        uint256 result = TigerSwapOptionMath.exp(1e18);
        uint256 expected = 2718281828459045235;
        uint256 diff = result > expected ? result - expected : expected - result;
        assertLt(diff, 1e10, "exp(1) accurate");
    }

    function testExpInvertsLn() public pure {
        // exp(ln(x)) should return x.
        uint256 x = 1234e18;
        int256 logged = TigerSwapOptionMath.ln(x);
        uint256 roundTrip = TigerSwapOptionMath.exp(logged);
        uint256 diff = roundTrip > x ? roundTrip - x : x - roundTrip;
        assertLt(diff, x / 1e9, "exp(ln(x)) == x");
    }

    function testExpNegOfZero() public pure {
        assertEq(TigerSwapOptionMath.expNeg(0), 1e18, "exp(-0) = 1");
    }

    function testExpNegIsReciprocal() public pure {
        uint256 result = TigerSwapOptionMath.expNeg(1e18);
        // exp(-1) = 0.367879441171442321
        uint256 expected = 367879441171442321;
        uint256 diff = result > expected ? result - expected : expected - result;
        assertLt(diff, 1e10, "exp(-1) accurate");
    }

    function testExpRejectsNegative() public {
        vm.expectRevert("exp of negative");
        harness.exp(-1);
    }

    // ---- invSqrt ------------------------------------------------------------

    function testInvSqrtOfOne() public pure {
        assertEq(TigerSwapOptionMath.invSqrt(1e18), 1e18, "1/sqrt(1) = 1");
    }

    function testInvSqrtOfFour() public pure {
        assertEq(TigerSwapOptionMath.invSqrt(4e18), 5e17, "1/sqrt(4) = 0.5");
    }

    function testInvSqrtOfTwo() public pure {
        // 1/sqrt(2) = 0.707106781186547524
        uint256 result = TigerSwapOptionMath.invSqrt(2e18);
        uint256 expected = 707106781186547524;
        uint256 diff = result > expected ? result - expected : expected - result;
        assertLt(diff, 1e6, "1/sqrt(2) accurate");
    }

    function testInvSqrtRejectsZero() public {
        vm.expectRevert("invSqrt of zero");
        harness.invSqrt(0);
    }

    // ---- normal CDF ---------------------------------------------------------

    function testNormalCdfAtZeroIsHalf() public pure {
        uint256 result = TigerSwapOptionMath.normalCdf(0);
        // A&S 26.2.17 is continuous at zero and returns 0.5 up to its bound.
        uint256 diff = result > 5e17 ? result - 5e17 : 5e17 - result;
        assertLt(diff, 1e11, "Phi(0) = 0.5 within the approximation bound");
    }

    function testNormalCdfAtOne() public pure {
        // Phi(1) = 0.841344746068542929
        uint256 result = TigerSwapOptionMath.normalCdf(1e18);
        uint256 expected = 841344746068542929;
        uint256 diff = result > expected ? result - expected : expected - result;
        assertLt(diff, 1e11, "Phi(1) within the 7.5e-8 approximation bound");
    }

    function testNormalCdfAtTwo() public pure {
        // Phi(2) = 0.977249868051820791
        uint256 result = TigerSwapOptionMath.normalCdf(2e18);
        uint256 expected = 977249868051820791;
        uint256 diff = result > expected ? result - expected : expected - result;
        assertLt(diff, 1e11, "Phi(2) within the 7.5e-8 approximation bound");
    }

    function testNormalCdfAtMinusOne() public pure {
        // Phi(-1) = 0.158655253931457051
        uint256 result = TigerSwapOptionMath.normalCdf(-1e18);
        uint256 expected = 158655253931457051;
        uint256 diff = result > expected ? result - expected : expected - result;
        assertLt(diff, 1e11, "Phi(-1) within the 7.5e-8 approximation bound");
    }

    function testNormalCdfIsSymmetric() public pure {
        uint256 positive = TigerSwapOptionMath.normalCdf(15e17);
        uint256 negative = TigerSwapOptionMath.normalCdf(-15e17);
        assertEq(positive + negative, 1e18, "Phi(x) + Phi(-x) = 1");
    }

    function testNormalCdfApproachesOneAndZero() public pure {
        assertGt(TigerSwapOptionMath.normalCdf(6e18), 999999999e9, "Phi(6) near 1");
        assertLt(TigerSwapOptionMath.normalCdf(-6e18), 1e9, "Phi(-6) near 0");
    }

    // ---- Black-Scholes against the Python reference -------------------------

    function testAtTheMoneyCallWithinOnePercent() public pure {
        uint256 price = TigerSwapOptionMath.blackScholes(2000e18, 2000e18, YEAR, 5e16, 8e17, true);
        // Reference: 656.4196493398014
        _assertClose(price, 656419649339801400000, 100, "ATM call");
    }

    /// @dev Python: bs(2000,2500,0.5,0.05,0.6) = 192.84771107754466
    function testOutOfTheMoneyCallWithinOnePercent() public pure {
        uint256 price = TigerSwapOptionMath.blackScholes(2000e18, 2500e18, YEAR / 2, 5e16, 6e17, true);
        _assertClose(price, 192847711077544660000, 100, "OTM call");
    }

    /// @dev Python: bs(2000,1500,0.25,0.05,0.9) = 115.33905944655538 (put)
    function testInTheMoneyPutWithinOnePercent() public pure {
        uint256 price = TigerSwapOptionMath.blackScholes(2000e18, 1500e18, YEAR / 4, 5e16, 9e17, false);
        _assertClose(price, 115339059446555380000, 100, "ITM put");
    }

    /// @dev Python: bs(2000,2000,1.0,0,1.0) = 765.8498450960524
    function testZeroRateAtTheMoneyCallWithinOnePercent() public pure {
        uint256 price = TigerSwapOptionMath.blackScholes(2000e18, 2000e18, YEAR, 0, 1e18, true);
        _assertClose(price, 765849845096052400000, 100, "zero rate ATM call");
    }

    // ---- Put-call parity ----------------------------------------------------

    function testPutCallParity() public pure {
        uint256 spot = 2000e18;
        uint256 strike = 2000e18;
        uint256 rate = 5e16;
        uint256 vol = 8e17;

        uint256 call = TigerSwapOptionMath.blackScholes(spot, strike, YEAR, rate, vol, true);
        uint256 put = TigerSwapOptionMath.blackScholes(spot, strike, YEAR, rate, vol, false);

        // C - P = S - K e^{-rT}
        uint256 discount = TigerSwapOptionMath.expNeg(int256(rate));
        uint256 forward = strike * discount / 1e18;
        uint256 expected = spot - forward;

        uint256 callMinusPut = call > put ? call - put : put - call;
        _assertClose(callMinusPut, expected, 20, "put-call parity");
    }

    // ---- Edge cases ---------------------------------------------------------

    function testExpiredOptionIsIntrinsicOnly() public pure {
        // ITM call at expiry.
        assertEq(
            TigerSwapOptionMath.blackScholes(2000e18, 1500e18, 0, 5e16, 8e17, true),
            500e18,
            "expired ITM call is intrinsic"
        );
        // OTM call at expiry.
        assertEq(
            TigerSwapOptionMath.blackScholes(2000e18, 2500e18, 0, 5e16, 8e17, true),
            0,
            "expired OTM call is worthless"
        );
        // ITM put at expiry.
        assertEq(
            TigerSwapOptionMath.blackScholes(2000e18, 2500e18, 0, 5e16, 8e17, false),
            500e18,
            "expired ITM put is intrinsic"
        );
    }

    function testZeroVolatilityIsIntrinsicOnly() public pure {
        assertEq(
            TigerSwapOptionMath.blackScholes(2000e18, 1500e18, YEAR, 5e16, 0, true),
            500e18,
            "zero vol call is intrinsic"
        );
    }

    function testDeepInTheMoneyCallApproachesForwardIntrinsic() public pure {
        uint256 price = TigerSwapOptionMath.blackScholes(10_000e18, 1000e18, YEAR, 5e16, 5e17, true);
        uint256 intrinsic = 9000e18;
        assertGe(price, intrinsic, "price at least intrinsic");
        // Deep ITM, so the price should be close to intrinsic plus the carry.
        assertLt(price, intrinsic * 12 / 10, "price stays near intrinsic");
    }

    function testCallPriceIncreasesWithVolatility() public pure {
        uint256 low = TigerSwapOptionMath.blackScholes(2000e18, 2000e18, YEAR, 5e16, 3e17, true);
        uint256 high = TigerSwapOptionMath.blackScholes(2000e18, 2000e18, YEAR, 5e16, 1e18, true);
        assertGt(high, low, "higher vol, higher price");
    }

    function testLongerExpiryCostsMore() public pure {
        uint256 short = TigerSwapOptionMath.blackScholes(2000e18, 2000e18, YEAR / 4, 5e16, 8e17, true);
        uint256 long = TigerSwapOptionMath.blackScholes(2000e18, 2000e18, YEAR, 5e16, 8e17, true);
        assertGt(long, short, "more time, more premium");
    }

    function testRejectsZeroSpot() public {
        vm.expectRevert("invalid price inputs");
        harness.price(0, 2000e18, YEAR, 5e16, 8e17, true);
    }

    // ---- Greeks -------------------------------------------------------------

    function testCallDeltaIsBetweenZeroAndOne() public pure {
        (int256 delta,,,) = TigerSwapOptionMath.greeks(2000e18, 2000e18, YEAR, 5e16, 8e17, true);
        assertGt(delta, 0, "call delta positive");
        assertLt(delta, 1e18, "call delta below one");
    }

    function testAtTheMoneyCallDeltaNearHalf() public pure {
        (int256 delta,,,) = TigerSwapOptionMath.greeks(2000e18, 2000e18, YEAR, 5e16, 8e17, true);
        // With 80% vol and a 5% rate, d1 = 0.4625 and the true delta is 0.67814.
        uint256 diff = uint256(delta) > 678138598836346000 ? uint256(delta) - 678138598836346000 : 678138598836346000 - uint256(delta);
        assertLt(diff, 1e11, "ATM call delta matches the reference");
    }

    function testPutDeltaIsCallDeltaMinusOne() public pure {
        (int256 callDelta,,,) = TigerSwapOptionMath.greeks(2000e18, 2000e18, YEAR, 5e16, 8e17, true);
        (int256 putDelta,,,) = TigerSwapOptionMath.greeks(2000e18, 2000e18, YEAR, 5e16, 8e17, false);
        assertEq(putDelta, callDelta - 1e18, "put delta = call delta - 1");
    }

    function testGammaIsPositiveAndSharedByCallAndPut() public pure {
        (, int256 callGamma,,) = TigerSwapOptionMath.greeks(2000e18, 2000e18, YEAR, 5e16, 8e17, true);
        (, int256 putGamma,,) = TigerSwapOptionMath.greeks(2000e18, 2000e18, YEAR, 5e16, 8e17, false);
        assertGt(callGamma, 0, "gamma positive");
        assertEq(callGamma, putGamma, "gamma is the same for a call and put");
    }

    function testVegaIsPositive() public pure {
        (,, int256 vega,) = TigerSwapOptionMath.greeks(2000e18, 2000e18, YEAR, 5e16, 8e17, true);
        assertGt(vega, 0, "vega positive");
    }

    function testThetaIsNegativeForLongOption() public pure {
        (,,, int256 theta) = TigerSwapOptionMath.greeks(2000e18, 2000e18, YEAR, 5e16, 8e17, true);
        assertLt(theta, 0, "long option loses value with time");
    }

    function testGreeksAtExpiry() public pure {
        (int256 delta, int256 gamma, int256 vega, int256 theta) =
            TigerSwapOptionMath.greeks(2000e18, 1500e18, 0, 5e16, 8e17, true);
        assertEq(delta, 1e18, "ITM call delta is 1 at expiry");
        assertEq(gamma, 0, "gamma zero at expiry");
        assertEq(vega, 0, "vega zero at expiry");
        assertEq(theta, 0, "theta zero at expiry");
    }
}
