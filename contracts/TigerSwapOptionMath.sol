// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/**
 * @title TigerSwapOptionMath
 * @notice Black-Scholes pricing for European cash-settled options, in 1e18
 * fixed point.
 *
 * The library provides the transcendental pieces the formula needs (ln, exp,
 * reciprocal square root and the standard normal CDF) plus the price and
 * Greeks. Everything is pure and stateless so it can be tested directly
 * against an independent reference implementation.
 *
 * Accuracy: ln and exp hold under 1e-12 relative error across the ranges an
 * option price uses, and the normal CDF uses the Abramowitz & Stegun 26.2.17
 * rational approximation, which has an absolute error below 7.5e-8.
 */
library TigerSwapOptionMath {
    /// @dev Fixed point scale for all values.
    int256 internal constant WAD = 1e18;
    uint256 internal constant WAD_U = 1e18;

    /// @dev ln(2) * 1e18.
    int256 internal constant LN_2 = 693147180559945309;

    /// @dev 1/sqrt(2*pi) * 1e18, the standard normal density peak.
    int256 internal constant INV_SQRT_2PI = 398942280401432678;

    /// @dev Coefficients of the Abramowitz & Stegun 26.2.17 tail series.
    int256 internal constant AS_P = 2316419; // 0.2316419 * 1e7
    int256 internal constant AS_B1 = 319381530; // 0.319381530 * 1e9
    int256 internal constant AS_B2 = -356563782;
    int256 internal constant AS_B3 = 1781477937;
    int256 internal constant AS_B4 = -1821255978;
    int256 internal constant AS_B5 = 1330274429;
    int256 internal constant AS_SCALE = 1e9;

    /**
     * @notice Natural log of x * 1e18 in 1e18 fixed point, for x > 0.
     * @dev Normalises x into [1e18, 2e18) by repeated halving or doubling and
     * then uses the atanh series ln(m) = 2 * atanh((m-1)/(m+1)), which
     * converges quickly on that interval.
     */
    function ln(uint256 x) internal pure returns (int256) {
        require(x > 0, "ln of zero");

        int256 n = 0;
        if (x < WAD_U) {
            while (x < WAD_U) {
                x *= 2;
                n -= 1;
            }
        } else {
            while (x >= 2 * WAD_U) {
                x /= 2;
                n += 1;
            }
        }

        int256 z = (int256(x) - WAD) * WAD / (int256(x) + WAD);
        int256 zSquared = z * z / WAD;

        int256 series = 0;
        int256 term = z;
        for (uint256 i = 1; i <= 21; i += 2) {
            series += 2 * term / int256(i);
            term = term * zSquared / WAD;
        }

        // `series` is already ln(m / 1e18), so only the halving count remains.
        return series + n * LN_2;
    }

    /// @notice e^x * 1e18 for x >= 0, in 1e18 fixed point.
    /// @dev Splits off n * ln2 so the Taylor remainder stays small, then scales
    /// by 2^n.
    function exp(int256 x) internal pure returns (uint256) {
        require(x >= 0, "exp of negative");
        int256 n = x / LN_2;
        require(n < 200, "exp overflow");
        int256 remainder = x - n * LN_2;

        int256 result = WAD;
        int256 term = WAD;
        for (uint256 i = 1; i <= 24; i++) {
            term = term * remainder / (WAD * int256(i));
            result += term;
        }
        require(result >= 0, "exp underflow");
        return uint256(result) << uint256(n);
    }

    /// @notice e^{-x} * 1e18 for x >= 0, in 1e18 fixed point.
    function expNeg(int256 x) internal pure returns (uint256) {
        require(x >= 0, "expNeg of negative");
        return (WAD_U * WAD_U) / exp(x);
    }

    /// @notice 1 / sqrt(x) * 1e18, via Newton's method on the reciprocal square root.
    function invSqrt(uint256 x) internal pure returns (uint256) {
        require(x > 0, "invSqrt of zero");
        // Seed with a power of two close to 1/sqrt(x) so Newton converges fast.
        uint256 y = WAD_U;
        uint256 temp = x;
        while (temp >= 4 * WAD_U) {
            temp /= 4;
            y /= 2;
        }
        while (temp < WAD_U) {
            temp *= 4;
            y *= 2;
        }
        // Newton: y <- y * (3 - x*y^2) / 2
        for (uint256 i = 0; i < 40; i++) {
            uint256 ySquared = y * y / WAD_U;
            uint256 step = x * ySquared / WAD_U;
            uint256 next = y * (3 * WAD_U - step) / (2 * WAD_U);
            if (next == y) {
                break;
            }
            y = next;
        }
        return y;
    }

    /// @dev Standard normal density at x (1e18 fixed point), as 1e18 fixed point.
    function normalPdf(int256 x) internal pure returns (uint256) {
        int256 xSquared = x * x / WAD;
        int256 halfXSquared = xSquared / 2;
        uint256 expValue = expNeg(halfXSquared);
        return uint256(INV_SQRT_2PI) * expValue / WAD_U;
    }

    /**
     * @notice Standard normal cumulative distribution Phi(x), in 1e18 fixed point.
     * @dev Abramowitz & Stegun 26.2.17: Phi(x) = 1 - phi(x) * poly(t) with
     * t = 1/(1 + p*x) for x >= 0, and Phi(x) = 1 - Phi(-x) otherwise. The
     * approximation is continuous at zero and has absolute error below 7.5e-8.
     */
    function normalCdf(int256 x) internal pure returns (uint256) {
        bool negative = x < 0;
        int256 t = negative ? -x : x;

        // t = 1e18 / (1e18 + p * x), with p scaled by 1e7.
        int256 denominator = WAD + AS_P * t / 10000000;
        int256 asT = WAD * WAD / denominator;

        // poly = b1*t + b2*t^2 + b3*t^3 + b4*t^4 + b5*t^5, coefficients scaled by 1e9.
        int256 power = asT;
        int256 poly = AS_B1 * power / WAD;
        power = power * asT / WAD;
        poly += AS_B2 * power / WAD;
        power = power * asT / WAD;
        poly += AS_B3 * power / WAD;
        power = power * asT / WAD;
        poly += AS_B4 * power / WAD;
        power = power * asT / WAD;
        poly += AS_B5 * power / WAD;
        poly = poly * WAD / AS_SCALE;

        uint256 density = normalPdf(t);
        int256 tail = int256(density) * poly / WAD;

        int256 upper = WAD - tail;
        if (upper < 0) {
            upper = 0;
        }
        if (upper > WAD) {
            upper = WAD;
        }

        return negative ? uint256(WAD - upper) : uint256(upper);
    }

    /**
     * @notice Black-Scholes price of a European option, cash settled.
     * @param spot Underlying price, 1e18.
     * @param strike Strike price, 1e18.
     * @param timeToExpiry Seconds until expiry.
     * @param riskFreeRate Annual risk-free rate, 1e18 (0.05e18 = 5%).
     * @param volatility Annualised volatility, 1e18 (0.8e18 = 80%).
     * @param isCall True for a call, false for a put.
     * @return price Option premium in the same units as spot, 1e18.
     */
    function blackScholes(
        uint256 spot,
        uint256 strike,
        uint256 timeToExpiry,
        uint256 riskFreeRate,
        uint256 volatility,
        bool isCall
    ) internal pure returns (uint256 price) {
        require(spot > 0 && strike > 0, "invalid price inputs");

        uint256 intrinsic = isCall
            ? (spot > strike ? spot - strike : 0)
            : (strike > spot ? strike - spot : 0);

        if (timeToExpiry == 0 || volatility == 0) {
            return intrinsic;
        }

        int256 T = int256(timeToExpiry) * WAD / 365 days;
        if (T <= 0) {
            return intrinsic;
        }

        (int256 d1, int256 d2,) = _d1d2(spot, strike, T, riskFreeRate, volatility);
        uint256 nD1 = normalCdf(d1);
        uint256 nD2 = normalCdf(d2);
        uint256 discount = expNeg(int256(riskFreeRate) * T / WAD);

        if (isCall) {
            price = spot * nD1 / WAD_U - strike * discount / WAD_U * nD2 / WAD_U;
        } else {
            price = strike * discount / WAD_U * (WAD_U - nD2) / WAD_U - spot * (WAD_U - nD1) / WAD_U;
        }

        if (price < intrinsic) {
            price = intrinsic;
        }
    }

    /// @dev Computes d1, d2 and sigma*sqrt(T) for the Black-Scholes formula.
    function _d1d2(uint256 spot, uint256 strike, int256 T, uint256 riskFreeRate, uint256 volatility)
        private
        pure
        returns (int256 d1, int256 d2, int256 sigmaSqrtT)
    {
        int256 sigma = int256(volatility);
        int256 r = int256(riskFreeRate);

        int256 lnMoneyness = ln(spot * WAD_U / strike);
        int256 sigmaSquaredHalf = sigma * sigma / (2 * WAD);
        int256 numerator = lnMoneyness + (r + sigmaSquaredHalf) * T / WAD;

        // sigma * sqrt(T) = sigma / sqrt(1/T)
        uint256 invSqrtT = invSqrt(uint256(WAD_U * WAD_U / uint256(T)));
        sigmaSqrtT = sigma * int256(invSqrtT) / WAD;
        require(sigmaSqrtT > 0, "zero denominator");

        d1 = numerator * WAD / sigmaSqrtT;
        d2 = d1 - sigmaSqrtT;
    }

    /**
     * @notice Greeks for a European option, all in 1e18 fixed point.
     * @return delta dV/dS, gamma d2V/dS2, vega dV/dsigma (per 1.0 of vol),
     * and theta dV/dt (per year, negative for a decaying option).
     */
    function greeks(
        uint256 spot,
        uint256 strike,
        uint256 timeToExpiry,
        uint256 riskFreeRate,
        uint256 volatility,
        bool isCall
    ) internal pure returns (int256 delta, int256 gamma, int256 vega, int256 theta) {
        require(spot > 0 && strike > 0, "invalid price inputs");

        if (timeToExpiry == 0 || volatility == 0) {
            if (isCall) {
                delta = spot > strike ? WAD : int256(0);
            } else {
                delta = spot < strike ? -WAD : int256(0);
            }
            return (delta, 0, 0, 0);
        }

        int256 T = int256(timeToExpiry) * WAD / 365 days;
        if (T <= 0) {
            return (delta, 0, 0, 0);
        }

        (int256 d1, int256 d2, int256 sigmaSqrtT) = _d1d2(spot, strike, T, riskFreeRate, volatility);
        uint256 nD1 = normalCdf(d1);
        uint256 nD2 = normalCdf(d2);
        uint256 density = normalPdf(d1);

        int256 callDelta = int256(nD1);
        delta = isCall ? callDelta : callDelta - WAD;

        // gamma = phi(d1) / (S * sigma * sqrt(T))
        int256 gammaDenominator = int256(spot) * sigmaSqrtT / WAD;
        gamma = int256(density) * WAD / gammaDenominator;

        // vega = S * phi(d1) * sqrt(T), and sqrt(T) = sigma*sqrt(T) / sigma.
        int256 sqrtT = sigmaSqrtT * WAD / int256(volatility);
        vega = int256(spot) * int256(density) / WAD * sqrtT / WAD;

        uint256 discount = expNeg(int256(riskFreeRate) * T / WAD);

        // theta = -S phi(d1) sigma / (2 sqrt(T)) -/+ r K e^{-rT} N(+/-d2)
        int256 firstTerm = -int256(spot) * int256(density) / WAD * int256(volatility) / WAD / (2 * sigmaSqrtT);
        int256 secondTerm;
        if (isCall) {
            secondTerm = -int256(riskFreeRate) * int256(strike) / WAD * int256(discount) / WAD
                * int256(nD2) / WAD;
        } else {
            secondTerm = int256(riskFreeRate) * int256(strike) / WAD * int256(discount) / WAD
                * int256(WAD_U - nD2) / WAD;
        }
        theta = firstTerm + secondTerm;
    }
}
