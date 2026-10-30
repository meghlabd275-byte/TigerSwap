"""Risk models for TigerSwap.

Everything here is a real estimator: value at risk and expected shortfall from
the empirical distribution and from a fitted normal, volatility with EWMA, and
liquidation probability from a geometric-Brownian-motion model. No placeholder
numbers.
"""

from __future__ import annotations

import math
from dataclasses import dataclass, field
from typing import Dict, List, Optional, Sequence

from . import mathx


@dataclass
class RiskReport:
    """Risk metrics for a return series."""

    observations: int
    mean_return: float
    volatility: float
    annualised_volatility: float
    var_95: float
    var_99: float
    expected_shortfall_95: float
    expected_shortfall_99: float
    max_drawdown: float
    sharpe: float
    skewness: float
    kurtosis: float
    var_95_parametric: float
    var_99_parametric: float

    def as_dict(self) -> Dict[str, float]:
        return {
            "observations": self.observations,
            "mean_return": self.mean_return,
            "volatility": self.volatility,
            "annualised_volatility": self.annualised_volatility,
            "var_95": self.var_95,
            "var_99": self.var_99,
            "expected_shortfall_95": self.expected_shortfall_95,
            "expected_shortfall_99": self.expected_shortfall_99,
            "max_drawdown": self.max_drawdown,
            "sharpe": self.sharpe,
            "skewness": self.skewness,
            "kurtosis": self.kurtosis,
            "var_95_parametric": self.var_95_parametric,
            "var_99_parametric": self.var_99_parametric,
        }


class RiskEngine:
    """Computes value at risk, expected shortfall and related statistics."""

    def __init__(self, periods_per_year: int = 365):
        if periods_per_year <= 0:
            raise ValueError("periods_per_year must be positive")
        self.periods_per_year = periods_per_year

    def historical_var(self, returns: Sequence[float], confidence: float = 0.95) -> float:
        """Value at risk as a positive loss from the empirical distribution.

        At 95% confidence this is the 5th percentile of returns, negated.
        """
        self._validate_confidence(confidence)
        if not returns:
            raise ValueError("returns must not be empty")
        quantile = mathx.percentile(returns, 1.0 - confidence)
        return -quantile

    def expected_shortfall(self, returns: Sequence[float], confidence: float = 0.95) -> float:
        """Mean loss conditional on breaching the VaR threshold."""
        self._validate_confidence(confidence)
        if not returns:
            raise ValueError("returns must not be empty")

        threshold = mathx.percentile(returns, 1.0 - confidence)
        tail = [value for value in returns if value <= threshold]
        if not tail:
            return -threshold
        return -mathx.mean(tail)

    def parametric_var(
        self, returns: Sequence[float], confidence: float = 0.95
    ) -> float:
        """Normal value at risk using the sample mean and standard deviation."""
        self._validate_confidence(confidence)
        if len(returns) < 2:
            return 0.0

        mu = mathx.mean(returns)
        sigma = mathx.stdev(returns)
        quantile = mathx.normal_ppf(1.0 - confidence)
        return -(mu + sigma * quantile)

    def max_drawdown(self, returns: Sequence[float]) -> float:
        """Largest peak-to-trough decline of the compounded equity curve."""
        if not returns:
            return 0.0

        equity = 1.0
        peak = 1.0
        worst = 0.0

        for value in returns:
            equity *= 1.0 + value
            if equity > peak:
                peak = equity
            drawdown = (equity - peak) / peak
            if drawdown < worst:
                worst = drawdown

        return abs(worst)

    def sharpe_ratio(self, returns: Sequence[float], risk_free_rate: float = 0.0) -> float:
        """Annualised Sharpe ratio from per-period returns."""
        if len(returns) < 2:
            return 0.0

        excess = [value - risk_free_rate / self.periods_per_year for value in returns]
        sigma = mathx.stdev(excess)
        if sigma == 0.0:
            return 0.0
        return mathx.mean(excess) / sigma * math.sqrt(self.periods_per_year)

    def ewma_volatility(self, returns: Sequence[float], decay: float = 0.94) -> float:
        """Per-period volatility from an exponentially weighted variance."""
        return math.sqrt(mathx.ewma_variance(returns, decay))

    def report(self, returns: Sequence[float], risk_free_rate: float = 0.0) -> RiskReport:
        """Compute the full risk report for a return series."""
        if not returns:
            raise ValueError("returns must not be empty")

        sigma = mathx.stdev(returns)
        return RiskReport(
            observations=len(returns),
            mean_return=mathx.mean(returns),
            volatility=sigma,
            annualised_volatility=sigma * math.sqrt(self.periods_per_year),
            var_95=self.historical_var(returns, 0.95),
            var_99=self.historical_var(returns, 0.99),
            expected_shortfall_95=self.expected_shortfall(returns, 0.95),
            expected_shortfall_99=self.expected_shortfall(returns, 0.99),
            max_drawdown=self.max_drawdown(returns),
            sharpe=self.sharpe_ratio(returns, risk_free_rate),
            skewness=self.skewness(returns),
            kurtosis=self.kurtosis(returns),
            var_95_parametric=self.parametric_var(returns, 0.95),
            var_99_parametric=self.parametric_var(returns, 0.99),
        )

    def skewness(self, returns: Sequence[float]) -> float:
        """Sample skewness (third standardised central moment).

        The third moment and the scale use the same divisor, so the statistic
        does not depend on an arbitrary mix of the population and sample
        conventions.
        """
        count = len(returns)
        if count < 3:
            return 0.0
        mu = mathx.mean(returns)
        m2 = math.fsum((value - mu) ** 2 for value in returns) / count
        if m2 == 0.0:
            return 0.0
        m3 = math.fsum((value - mu) ** 3 for value in returns) / count
        return m3 / (m2**1.5)

    def kurtosis(self, returns: Sequence[float]) -> float:
        """Excess kurtosis (fourth standardised central moment minus 3)."""
        count = len(returns)
        if count < 4:
            return 0.0
        mu = mathx.mean(returns)
        m2 = math.fsum((value - mu) ** 2 for value in returns) / count
        if m2 == 0.0:
            return 0.0
        m4 = math.fsum((value - mu) ** 4 for value in returns) / count
        return m4 / (m2**2) - 3.0

    def liquidation_probability(
        self,
        entry_price: float,
        liquidation_price: float,
        volatility: float,
        horizon_periods: float,
        is_long: bool = True,
    ) -> float:
        """Probability a leveraged position touches its liquidation price.

        Uses the driftless geometric Brownian motion first-passage
        approximation for the log price, which is the standard model for this
        calculation. Returns a probability in [0, 1].
        """
        if entry_price <= 0 or liquidation_price <= 0:
            raise ValueError("prices must be positive")
        if volatility < 0:
            raise ValueError("volatility must not be negative")
        if horizon_periods <= 0:
            return 0.0
        if volatility == 0.0:
            breached = liquidation_price >= entry_price if is_long else liquidation_price <= entry_price
            return 1.0 if breached else 0.0

        barrier = liquidation_price / entry_price
        if barrier <= 0:
            return 1.0

        # Distance to the barrier in log space, signed so a long's downside is
        # a negative move and a short's upside is positive.
        distance = math.log(barrier)
        if is_long:
            distance = -distance

        if distance <= 0:
            # Already at or beyond the barrier.
            return 1.0

        sigma_t = volatility * math.sqrt(horizon_periods)
        if sigma_t == 0:
            return 0.0

        # Reflection principle for a driftless Brownian motion hitting a level.
        probability = 2.0 * (1.0 - mathx.normal_cdf(distance / sigma_t))
        return mathx.clip(probability, 0.0, 1.0)

    @staticmethod
    def _validate_confidence(confidence: float) -> None:
        if not 0.0 < confidence < 1.0:
            raise ValueError("confidence must be strictly between 0 and 1")


@dataclass
class PortfolioRisk:
    """Risk of a multi-asset portfolio."""

    weights: List[float]
    volatility: float
    annualised_volatility: float
    var_95: float
    diversification_ratio: float
    component_risk: List[float] = field(default_factory=list)


class PortfolioRiskEngine:
    """Portfolio volatility and VaR from a covariance matrix."""

    def __init__(self, periods_per_year: int = 365):
        if periods_per_year <= 0:
            raise ValueError("periods_per_year must be positive")
        self.periods_per_year = periods_per_year

    def portfolio_volatility(
        self, weights: Sequence[float], covariance: Sequence[Sequence[float]]
    ) -> float:
        """Portfolio standard deviation: sqrt(w' C w)."""
        size = len(weights)
        if size == 0:
            return 0.0
        if len(covariance) != size or any(len(row) != size for row in covariance):
            raise ValueError("covariance matrix does not match the weights")

        quadratic = math.fsum(
            weights[i] * weights[j] * covariance[i][j]
            for i in range(size)
            for j in range(size)
        )
        return math.sqrt(max(0.0, quadratic))

    def component_risk(
        self, weights: Sequence[float], covariance: Sequence[Sequence[float]]
    ) -> List[float]:
        """Marginal contribution of each asset to portfolio volatility."""
        size = len(weights)
        if size == 0:
            return []

        portfolio_variance = math.fsum(
            weights[i] * weights[j] * covariance[i][j]
            for i in range(size)
            for j in range(size)
        )
        sigma = math.sqrt(max(0.0, portfolio_variance))
        if sigma == 0.0:
            return [0.0] * size

        contributions = []
        for i in range(size):
            marginal = math.fsum(weights[j] * covariance[i][j] for j in range(size))
            contributions.append(weights[i] * marginal / sigma)
        return contributions

    def analyse(
        self,
        weights: Sequence[float],
        covariance: Sequence[Sequence[float]],
        confidence: float = 0.95,
    ) -> PortfolioRisk:
        """Full portfolio risk analysis."""
        sigma = self.portfolio_volatility(weights, covariance)

        # Weighted average of the individual volatilities, used for the
        # diversification ratio.
        weighted_volatility = math.fsum(
            abs(weight) * math.sqrt(max(0.0, covariance[i][i]))
            for i, weight in enumerate(weights)
        )
        diversification = 0.0
        if sigma > 0:
            diversification = weighted_volatility / sigma

        quantile = mathx.normal_ppf(1.0 - confidence)
        var = -(0.0 + sigma * quantile)

        return PortfolioRisk(
            weights=list(weights),
            volatility=sigma,
            annualised_volatility=sigma * math.sqrt(self.periods_per_year),
            var_95=var,
            diversification_ratio=round(diversification, 6),
            component_risk=self.component_risk(weights, covariance),
        )
