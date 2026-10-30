"""Options pricing for TigerSwap.

Implements the Black-Scholes-Merton European option price with continuous
dividend yield, every first-order Greek, and an implied-volatility solver using
the bisection method. All formulas are the standard closed-form results; no
values are approximated by lookup tables.
"""

from __future__ import annotations

import math
from dataclasses import dataclass
from typing import Dict

from . import mathx


@dataclass
class OptionGreeks:
    """First-order sensitivities of an option price."""

    delta: float
    gamma: float
    vega: float
    theta: float
    rho: float

    def as_dict(self) -> Dict[str, float]:
        return {
            "delta": self.delta,
            "gamma": self.gamma,
            "vega": self.vega,
            "theta": self.theta,
            "rho": self.rho,
        }


class BlackScholes:
    """Black-Scholes-Merton pricing for European options.

    `spot` and `strike` are in the same units, `rate` and `dividend_yield` are
    annual continuously compounded rates, `volatility` is annualised, and
    `time_to_expiry` is in years.
    """

    @staticmethod
    def _d1_d2(
        spot: float,
        strike: float,
        rate: float,
        dividend_yield: float,
        volatility: float,
        time_to_expiry: float,
    ) -> tuple[float, float]:
        if spot <= 0 or strike <= 0:
            raise ValueError("spot and strike must be positive")
        if volatility < 0:
            raise ValueError("volatility must not be negative")
        if time_to_expiry <= 0:
            raise ValueError("time_to_expiry must be positive")

        sigma_t = volatility * math.sqrt(time_to_expiry)
        if sigma_t == 0:
            # Deterministic case: the forward is known exactly.
            forward = math.log(spot / strike) + (rate - dividend_yield) * time_to_expiry
            return forward / 1e-12, forward / 1e-12

        d1 = (
            math.log(spot / strike)
            + (rate - dividend_yield + 0.5 * volatility * volatility) * time_to_expiry
        ) / sigma_t
        d2 = d1 - sigma_t
        return d1, d2

    @staticmethod
    def call_price(
        spot: float,
        strike: float,
        rate: float,
        volatility: float,
        time_to_expiry: float,
        dividend_yield: float = 0.0,
    ) -> float:
        """European call price."""
        d1, d2 = BlackScholes._d1_d2(spot, strike, rate, dividend_yield, volatility, time_to_expiry)
        discounted_spot = spot * math.exp(-dividend_yield * time_to_expiry)
        discounted_strike = strike * math.exp(-rate * time_to_expiry)
        return discounted_spot * mathx.normal_cdf(d1) - discounted_strike * mathx.normal_cdf(d2)

    @staticmethod
    def put_price(
        spot: float,
        strike: float,
        rate: float,
        volatility: float,
        time_to_expiry: float,
        dividend_yield: float = 0.0,
    ) -> float:
        """European put price, derived from put-call parity."""
        call = BlackScholes.call_price(
            spot, strike, rate, volatility, time_to_expiry, dividend_yield
        )
        # C - P = S e^{-qT} - K e^{-rT}
        return (
            call
            - spot * math.exp(-dividend_yield * time_to_expiry)
            + strike * math.exp(-rate * time_to_expiry)
        )

    @staticmethod
    def price(
        spot: float,
        strike: float,
        rate: float,
        volatility: float,
        time_to_expiry: float,
        option_type: str = "call",
        dividend_yield: float = 0.0,
    ) -> float:
        """Price a call or a put."""
        kind = option_type.lower()
        if kind == "call":
            return BlackScholes.call_price(
                spot, strike, rate, volatility, time_to_expiry, dividend_yield
            )
        if kind == "put":
            return BlackScholes.put_price(
                spot, strike, rate, volatility, time_to_expiry, dividend_yield
            )
        raise ValueError("option_type must be 'call' or 'put'")

    @staticmethod
    def greeks(
        spot: float,
        strike: float,
        rate: float,
        volatility: float,
        time_to_expiry: float,
        option_type: str = "call",
        dividend_yield: float = 0.0,
    ) -> OptionGreeks:
        """First-order Greeks.

        Theta is expressed per year; divide by 365 for a daily figure. Vega is
        per unit of volatility (a change of 1.0, i.e. 100 percentage points), so
        divide by 100 for the sensitivity to a one-point move.
        """
        kind = option_type.lower()
        if kind not in ("call", "put"):
            raise ValueError("option_type must be 'call' or 'put'")
        if volatility <= 0:
            # Gamma and vega both divide by volatility, and the sensitivities
            # are not well defined at zero volatility (the terminal
            # distribution is a point mass), so reject rather than divide by
            # zero. Use price() for the deterministic value.
            raise ValueError("greeks require a positive volatility")

        d1, d2 = BlackScholes._d1_d2(spot, strike, rate, dividend_yield, volatility, time_to_expiry)
        discount_q = math.exp(-dividend_yield * time_to_expiry)
        discount_r = math.exp(-rate * time_to_expiry)

        if kind == "call":
            delta = discount_q * mathx.normal_cdf(d1)
        else:
            delta = -discount_q * mathx.normal_cdf(-d1)

        gamma = discount_q * mathx.normal_pdf(d1) / (spot * volatility * math.sqrt(time_to_expiry))

        vega = spot * discount_q * mathx.normal_pdf(d1) * math.sqrt(time_to_expiry)

        if kind == "call":
            theta = (
                -spot * discount_q * mathx.normal_pdf(d1) * volatility / (2.0 * math.sqrt(time_to_expiry))
                - rate * strike * discount_r * mathx.normal_cdf(d2)
                + dividend_yield * spot * discount_q * mathx.normal_cdf(d1)
            )
        else:
            theta = (
                -spot * discount_q * mathx.normal_pdf(d1) * volatility / (2.0 * math.sqrt(time_to_expiry))
                + rate * strike * discount_r * mathx.normal_cdf(-d2)
                - dividend_yield * spot * discount_q * mathx.normal_cdf(-d1)
            )

        if kind == "call":
            rho = strike * time_to_expiry * discount_r * mathx.normal_cdf(d2)
        else:
            rho = -strike * time_to_expiry * discount_r * mathx.normal_cdf(-d2)

        return OptionGreeks(delta=delta, gamma=gamma, vega=vega, theta=theta, rho=rho)

    @staticmethod
    def implied_volatility(
        market_price: float,
        spot: float,
        strike: float,
        rate: float,
        time_to_expiry: float,
        option_type: str = "call",
        dividend_yield: float = 0.0,
        tolerance: float = 1e-8,
        max_iterations: int = 200,
    ) -> float:
        """Recovers implied volatility by bisection on a bracketed interval.

        Bisection is used rather than Newton's method because the vega of a
        deep out-of-the-money option is tiny, which makes Newton unstable.
        Raises ValueError when no arbitrage-free price brackets the target.
        """
        if market_price <= 0:
            raise ValueError("market_price must be positive")
        if spot <= 0 or strike <= 0:
            raise ValueError("spot and strike must be positive")
        if time_to_expiry <= 0:
            raise ValueError("time_to_expiry must be positive")

        # No-arbitrage bounds for the option price.
        discount_q = math.exp(-dividend_yield * time_to_expiry)
        discount_r = math.exp(-rate * time_to_expiry)
        if option_type.lower() == "call":
            lower_bound = max(0.0, spot * discount_q - strike * discount_r)
            upper_bound = spot * discount_q
        else:
            lower_bound = max(0.0, strike * discount_r - spot * discount_q)
            upper_bound = strike * discount_r

        if not lower_bound <= market_price <= upper_bound:
            raise ValueError(
                f"market_price {market_price} violates the no-arbitrage bounds "
                f"[{lower_bound}, {upper_bound}]"
            )

        low = 1e-6
        high = 5.0  # 500% annualised volatility is the search ceiling.

        low_price = BlackScholes.price(
            spot, strike, rate, low, time_to_expiry, option_type, dividend_yield
        )
        high_price = BlackScholes.price(
            spot, strike, rate, high, time_to_expiry, option_type, dividend_yield
        )

        if market_price <= low_price:
            return low
        if market_price >= high_price:
            raise ValueError("implied volatility exceeds the 500% search ceiling")

        for _ in range(max_iterations):
            mid = 0.5 * (low + high)
            price = BlackScholes.price(
                spot, strike, rate, mid, time_to_expiry, option_type, dividend_yield
            )

            if abs(price - market_price) < tolerance:
                return mid

            if price < market_price:
                low = mid
            else:
                high = mid

        return 0.5 * (low + high)


def put_call_parity_gap(
    call_price: float,
    put_price: float,
    spot: float,
    strike: float,
    rate: float,
    time_to_expiry: float,
    dividend_yield: float = 0.0,
) -> float:
    """Deviation from put-call parity, used to spot mispricing.

    Parity states C - P = S e^{-qT} - K e^{-rT}, so a non-zero return value
    indicates an arbitrage or stale quote.
    """
    left = call_price - put_price
    right = spot * math.exp(-dividend_yield * time_to_expiry) - strike * math.exp(
        -rate * time_to_expiry
    )
    return left - right
