"""Impermanent-loss forecasting for concentrated and full-range positions.

The AMM module prices impermanent loss at a fixed price ratio. A liquidity
provider deciding whether to stay in a pool needs more than that: they need the
distribution of the loss over the holding period, given how volatile the pair
is. This module supplies it.

Two models are provided.

* :class:`ImpermanentLossForecaster` propagates a lognormal price ratio through
  the constant-product loss function by Gauss-Hermite quadrature. The loss is a
  smooth function of the ratio, so quadrature converges quickly and gives the
  expected loss and the quantiles without simulation.

* :class:`ConcentratedLiquidityModel` handles a position restricted to a price
  band. Inside the band the position behaves like a constant-product pool over a
  scaled ratio; outside it is fully converted to one asset. The model accounts
  for both regimes, which is where a full-range formula silently understates the
  loss.
"""

from __future__ import annotations

import math
from dataclasses import dataclass
from typing import Dict, List, Sequence, Tuple

from . import mathx

# Gauss-Hermite nodes and weights for a 20-point rule, for the weight function
# exp(-x^2). The integral of exp(-x^2) f(x) is the sum of w_i f(x_i). The rule
# is exact for polynomials up to degree 39, far beyond what a smooth loss curve
# needs.
_GAUSS_HERMITE_NODES: Tuple[float, ...] = (
    -5.387480890011233,
    -4.603682449550744,
    -3.944764040115625,
    -3.347854567383216,
    -2.788806055277445,
    -2.254974002089275,
    -1.738537712116586,
    -1.234076215395323,
    -0.737473728545394,
    -0.245340708300901,
    0.245340708300901,
    0.737473728545394,
    1.234076215395323,
    1.738537712116586,
    2.254974002089275,
    2.788806055277445,
    3.347854567383216,
    3.944764040115625,
    4.603682449550744,
    5.387480890011233,
)

_GAUSS_HERMITE_WEIGHTS: Tuple[float, ...] = (
    2.229393645534151e-13,
    4.399340992273181e-10,
    1.086069370769281e-7,
    7.802556478532064e-6,
    2.283386360163539e-4,
    3.243773342237861e-3,
    2.481052088746362e-2,
    1.090172060200234e-1,
    2.866755053628342e-1,
    4.622436696006100e-1,
    4.622436696006100e-1,
    2.866755053628342e-1,
    1.090172060200234e-1,
    2.481052088746362e-2,
    3.243773342237861e-3,
    2.283386360163539e-4,
    7.802556478532064e-6,
    1.086069370769281e-7,
    4.399340992273181e-10,
    2.229393645534151e-13,
)


def full_range_loss(price_ratio: float) -> float:
    """The impermanent loss of a full-range position at a given price ratio.

    The ratio is the new price divided by the entry price. The loss is returned
    as a positive fraction of the position's value had it been held outside the
    pool, so 0.05 means a 5% loss.
    """
    if price_ratio <= 0:
        raise ValueError("price_ratio must be positive")
    value_in_pool = 2.0 * math.sqrt(price_ratio) / (1.0 + price_ratio)
    return 1.0 - value_in_pool


@dataclass
class ImpermanentLossForecast:
    """The projected loss distribution over a holding period."""

    expected_loss: float
    median_loss: float
    loss_95: float
    loss_99: float
    probability_of_loss: float
    volatility: float
    horizon_years: float

    def as_dict(self) -> Dict[str, object]:
        return {
            "expected_loss": self.expected_loss,
            "median_loss": self.median_loss,
            "loss_95": self.loss_95,
            "loss_99": self.loss_99,
            "probability_of_loss": self.probability_of_loss,
            "volatility": self.volatility,
            "horizon_years": self.horizon_years,
        }


class ImpermanentLossForecaster:
    """Projects impermanent loss from an annualised volatility and a horizon."""

    def __init__(self, quadrature_points: int = 20):
        if quadrature_points < 4 or quadrature_points > len(_GAUSS_HERMITE_NODES):
            raise ValueError(
                f"quadrature_points must be between 4 and {len(_GAUSS_HERMITE_NODES)}"
            )
        self.quadrature_points = quadrature_points

    def _ratio_samples(self, volatility: float, horizon_years: float) -> List[float]:
        """Lognormal price-ratio samples at the quadrature nodes.

        For a lognormal ratio the expected value of f(R) is the integral of
        f(exp(sigma sqrt(2T) x - sigma^2 T / 2)) against the Gaussian kernel, so
        the nodes are mapped through that exponential.
        """
        sigma = volatility * math.sqrt(horizon_years)
        drift = -0.5 * sigma * sigma
        scale = math.sqrt(2.0) * sigma

        half = self.quadrature_points // 2
        # The rule is symmetric; take a symmetric set of nodes around zero.
        indices = range(len(_GAUSS_HERMITE_NODES) // 2 - half, len(_GAUSS_HERMITE_NODES) // 2 + half)

        ratios = []
        for index in indices:
            node = _GAUSS_HERMITE_NODES[index]
            exponent = scale * node + drift
            # Guard against overflow at extreme volatility: cap the exponent.
            exponent = mathx.clip(exponent, -50.0, 50.0)
            ratios.append(math.exp(exponent))
        return ratios

    def _weights(self) -> List[float]:
        half = self.quadrature_points // 2
        indices = range(len(_GAUSS_HERMITE_WEIGHTS) // 2 - half, len(_GAUSS_HERMITE_WEIGHTS) // 2 + half)
        return [_GAUSS_HERMITE_WEIGHTS[index] for index in indices]

    def forecast(
        self,
        volatility: float,
        horizon_years: float,
        loss_function=None,
    ) -> ImpermanentLossForecast:
        """Forecasts the loss distribution.

        `volatility` is the annualised standard deviation of the pair's log
        return. `loss_function` defaults to :func:`full_range_loss` and may be
        replaced by a concentrated-position loss such as
        :meth:`ConcentratedLiquidityModel.loss`.
        """
        if volatility < 0:
            raise ValueError("volatility must not be negative")
        if horizon_years <= 0:
            raise ValueError("horizon_years must be positive")

        loss_fn = loss_function if loss_function is not None else full_range_loss

        ratios = self._ratio_samples(volatility, horizon_years)
        weights = self._weights()

        total_weight = sum(weights)
        losses = [loss_fn(ratio) for ratio in ratios]

        expected = sum(w * loss for w, loss in zip(weights, losses)) / total_weight
        probability_of_loss = (
            sum(w for w, loss in zip(weights, losses) if loss > 0.0) / total_weight
        )

        # Weighted quantiles over the sampled losses.
        ordered = sorted(zip(losses, weights))
        median = _weighted_quantile(ordered, 0.5, total_weight)
        loss_95 = _weighted_quantile(ordered, 0.95, total_weight)
        loss_99 = _weighted_quantile(ordered, 0.99, total_weight)

        return ImpermanentLossForecast(
            expected_loss=expected,
            median_loss=median,
            loss_95=loss_95,
            loss_99=loss_99,
            probability_of_loss=probability_of_loss,
            volatility=volatility,
            horizon_years=horizon_years,
        )


def _weighted_quantile(
    ordered: Sequence[Tuple[float, float]], fraction: float, total_weight: float
) -> float:
    """A weighted quantile of `(value, weight)` pairs sorted by value."""
    target = fraction * total_weight
    cumulative = 0.0
    for value, weight in ordered:
        cumulative += weight
        if cumulative >= target:
            return value
    return ordered[-1][0]


@dataclass
class ConcentratedPosition:
    """A concentrated-liquidity position and its current value."""

    lower_price: float
    upper_price: float
    entry_price: float
    current_price: float
    liquidity: float
    #: Token amounts at entry, in base and quote units.
    amount_base_entry: float
    amount_quote_entry: float

    def as_dict(self) -> Dict[str, object]:
        return {
            "lower_price": self.lower_price,
            "upper_price": self.upper_price,
            "entry_price": self.entry_price,
            "current_price": self.current_price,
            "liquidity": self.liquidity,
        }


class ConcentratedLiquidityModel:
    """Valuation and loss for a position confined to a price band.

    The position is described by its liquidity and band. Within the band the
    holdings follow the square-root rule; above the band the position is
    entirely in quote currency, below it entirely in base currency. That
    piecewise behaviour is why a concentrated position can lose far more than a
    full-range one when the price leaves the band.
    """

    @staticmethod
    def amounts_at_price(
        liquidity: float, lower: float, upper: float, price: float
    ) -> Tuple[float, float]:
        """The base and quote amounts a position holds at a given price."""
        if lower <= 0 or upper <= lower:
            raise ValueError("invalid price band")
        if price <= 0:
            raise ValueError("price must be positive")

        sqrt_lower = math.sqrt(lower)
        sqrt_upper = math.sqrt(upper)

        if price <= lower:
            # Fully in base currency: the position has been bought all the way in.
            base = liquidity * (1.0 / sqrt_lower - 1.0 / sqrt_upper)
            quote = 0.0
        elif price >= upper:
            # Fully in quote currency.
            base = 0.0
            quote = liquidity * (sqrt_upper - sqrt_lower)
        else:
            sqrt_price = math.sqrt(price)
            base = liquidity * (1.0 / sqrt_price - 1.0 / sqrt_upper)
            quote = liquidity * (sqrt_price - sqrt_lower)
        return base, quote

    @staticmethod
    def value(
        liquidity: float, lower: float, upper: float, price: float
    ) -> float:
        """The position's value in quote currency at `price`."""
        base, quote = ConcentratedLiquidityModel.amounts_at_price(
            liquidity, lower, upper, price
        )
        return base * price + quote

    @staticmethod
    def loss(
        liquidity: float,
        lower: float,
        upper: float,
        entry_price: float,
        current_price: float,
    ) -> float:
        """The loss versus holding the entry amounts outside the pool.

        Returned as a positive fraction, so 0.1 is a 10% loss. A negative
        result would mean a gain, which cannot happen for a passive position;
        rounding aside, the minimum is zero.
        """
        base_entry, quote_entry = ConcentratedLiquidityModel.amounts_at_price(
            liquidity, lower, upper, entry_price
        )
        hodl_value = base_entry * current_price + quote_entry
        if hodl_value <= 0:
            raise ValueError("entry position has no value")
        pool_value = ConcentratedLiquidityModel.value(
            liquidity, lower, upper, current_price
        )
        return 1.0 - pool_value / hodl_value

    @staticmethod
    def loss_function(lower: float, upper: float, entry_price: float):
        """Builds a single-argument loss function for the forecaster.

        The returned callable takes a price ratio, which is what the forecaster
        propagates.
        """
        if entry_price <= 0:
            raise ValueError("entry_price must be positive")

        # A unit position at entry; the ratio scales the current price.
        base_entry, quote_entry = ConcentratedLiquidityModel.amounts_at_price(
            1.0, lower, upper, entry_price
        )

        def loss_at_ratio(price_ratio: float) -> float:
            if price_ratio <= 0:
                raise ValueError("price_ratio must be positive")
            current = entry_price * price_ratio
            hodl_value = base_entry * current + quote_entry
            if hodl_value <= 0:
                return 0.0
            pool_value = ConcentratedLiquidityModel.value(1.0, lower, upper, current)
            return 1.0 - pool_value / hodl_value

        return loss_at_ratio


class OptimalRangeSelector:
    """Chooses a concentrated-position band from a volatility forecast.

    A narrower band earns more fees per unit of capital but is exited sooner by
    the same price move. This picks the band whose expected time in range
    exceeds the intended holding period, given the volatility.
    """

    def __init__(self, volatility: float, horizon_years: float):
        if volatility < 0:
            raise ValueError("volatility must not be negative")
        if horizon_years <= 0:
            raise ValueError("horizon_years must be positive")
        self.volatility = volatility
        self.horizon_years = horizon_years

    def expected_time_in_range(self, half_width: float) -> float:
        """The expected years until the price first leaves a +/- band.

        For a driftless log price the first-exit time from a symmetric band has
        a known characteristic-function form; a simpler and conservative proxy
        is used here: the band is crossed in expectation once the accumulated
        standard deviation equals the half-width. This is exact in the limit of
        a diffusion first hitting a distant barrier.
        """
        if half_width <= 0:
            raise ValueError("half_width must be positive")
        if self.volatility <= 0:
            return math.inf
        return (half_width / self.volatility) ** 2

    def select(self, entry_price: float, candidates: Sequence[float]) -> float:
        """Returns the narrowest half-width that survives the holding period.

        `candidates` are half-widths as fractions of the entry price. If none
        survives, the widest candidate is returned, because refusing to quote is
        not an option the caller asked for.
        """
        if entry_price <= 0:
            raise ValueError("entry_price must be positive")
        if not candidates:
            raise ValueError("candidates must not be empty")

        ordered = sorted(candidates)
        for half_width in ordered:
            if self.expected_time_in_range(half_width) >= self.horizon_years:
                return half_width
        return ordered[-1]
