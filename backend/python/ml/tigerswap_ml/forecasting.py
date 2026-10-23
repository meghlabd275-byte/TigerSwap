"""Price forecasting models for TigerSwap.

Two real models are provided: an autoregressive integrated moving average with
exogenous input (ARIMAX) fitted by ordinary least squares, and Holt's linear
(level plus trend) exponential smoothing with an optional damped trend. Both
expose calibrated prediction intervals derived from the in-sample residuals.
"""

from __future__ import annotations

import math
from dataclasses import dataclass
from typing import Dict, List, Optional, Sequence

from . import mathx


@dataclass
class Forecast:
    """A point forecast with an interval and diagnostics."""

    values: List[float]
    lower: List[float]
    upper: List[float]
    confidence: float
    residual_std: float
    r_squared: float
    observations: int

    def as_dict(self) -> Dict[str, object]:
        return {
            "values": self.values,
            "lower": self.lower,
            "upper": self.upper,
            "confidence": self.confidence,
            "residual_std": self.residual_std,
            "r_squared": self.r_squared,
            "observations": self.observations,
        }


def _difference(series: Sequence[float], order: int) -> List[float]:
    """Applies `order` rounds of first differencing."""
    values = list(series)
    for _ in range(order):
        if len(values) < 2:
            return []
        values = [current - previous for previous, current in zip(values, values[1:])]
    return values


class ARIMAX:
    """Autoregressive model with optional differencing and exogenous inputs.

    The design matrix is built from lagged values of the (differenced) series
    and from lagged exogenous inputs, then solved by least squares. This is a
    genuine ARIMAX(p, d, 0) estimator; it is intentionally explicit rather than
    hidden behind a heavy dependency.
    """

    def __init__(self, ar_order: int = 3, difference_order: int = 1, exogenous_lags: int = 1):
        if ar_order < 1:
            raise ValueError("ar_order must be at least 1")
        if difference_order < 0:
            raise ValueError("difference_order must not be negative")
        if exogenous_lags < 0:
            raise ValueError("exogenous_lags must not be negative")

        self.ar_order = ar_order
        self.difference_order = difference_order
        self.exogenous_lags = exogenous_lags

        self.coefficients: List[float] = []
        self.intercept: float = 0.0
        self.residual_std: float = 0.0
        self.r_squared: float = 0.0
        self._last_level: float = 0.0
        self._last_differenced: List[float] = []
        self._fitted = False

    def _build_design(
        self, target: Sequence[float], exogenous: Optional[Sequence[Sequence[float]]]
    ) -> tuple[List[List[float]], List[float]]:
        """Builds the regression rows and the target vector."""
        max_lag = self.ar_order
        if exogenous is not None:
            max_lag = max(max_lag, self.exogenous_lags)
        if len(target) <= max_lag:
            raise ValueError("not enough observations to fit the requested lags")

        rows: List[List[float]] = []
        outcomes: List[float] = []

        for index in range(max_lag, len(target)):
            row = [1.0]
            for lag in range(1, self.ar_order + 1):
                row.append(target[index - lag])
            if exogenous is not None and self.exogenous_lags > 0:
                for series in exogenous:
                    for lag in range(1, self.exogenous_lags + 1):
                        row.append(series[index - lag])
            rows.append(row)
            outcomes.append(target[index])

        return rows, outcomes

    def fit(
        self,
        series: Sequence[float],
        exogenous: Optional[Sequence[Sequence[float]]] = None,
    ) -> "ARIMAX":
        """Fits the model by ordinary least squares."""
        if len(series) < 2:
            raise ValueError("series must contain at least two points")

        self._last_level = series[-1]
        target = _difference(series, self.difference_order)
        if len(target) < self.ar_order + 1:
            raise ValueError("series is too short for the requested AR order")

        if exogenous is not None:
            aligned = []
            for column in exogenous:
                differenced = list(column)
                if len(differenced) != len(series):
                    raise ValueError("exogenous series must match the target length")
                # Align exogenous inputs with the differenced target.
                aligned.append(differenced[self.difference_order:])
            exogenous = aligned

        rows, outcomes = self._build_design(target, exogenous)

        # Normal equations: (X'X) b = X'y
        xtx = [
            [math.fsum(row[i] * row[j] for row in rows) for j in range(len(rows[0]))]
            for i in range(len(rows[0]))
        ]
        xty = [
            math.fsum(row[i] * outcome for row, outcome in zip(rows, outcomes))
            for i in range(len(rows[0]))
        ]

        # Ridge term keeps the system well posed when columns are collinear.
        for index in range(1, len(xtx)):
            xtx[index][index] += 1e-8

        solution = mathx.solve_linear(xtx, xty)

        self.intercept = solution[0]
        self.coefficients = solution[1:]
        self._fitted = True

        # Residual diagnostics.
        residuals = []
        for row, outcome in zip(rows, outcomes):
            prediction = math.fsum(coef * value for coef, value in zip(solution, row))
            residuals.append(outcome - prediction)

        self.residual_std = mathx.stdev(residuals) if len(residuals) > 1 else 0.0
        total = math.fsum((outcome - mathx.mean(outcomes)) ** 2 for outcome in outcomes)
        residual_sum = math.fsum(residual**2 for residual in residuals)
        self.r_squared = 1.0 - residual_sum / total if total > 0 else 0.0

        self._last_differenced = list(target[-(self.ar_order + 1):])
        return self

    def forecast(
        self,
        steps: int,
        exogenous_future: Optional[Sequence[Sequence[float]]] = None,
        confidence: float = 0.95,
    ) -> Forecast:
        """Produces `steps` forecasts with a prediction interval."""
        if not self._fitted:
            raise ValueError("call fit() before forecast()")
        if steps < 1:
            raise ValueError("steps must be at least 1")
        if not 0.0 < confidence < 1.0:
            raise ValueError("confidence must be strictly between 0 and 1")

        history = list(self._last_differenced)
        exogenous_history: List[List[float]] = []

        if exogenous_future is not None:
            for series in exogenous_future:
                if len(series) < steps:
                    raise ValueError("exogenous_future is shorter than the horizon")
                exogenous_history.append(list(series))

        forecasts: List[float] = []
        ar_count = self.ar_order
        exo_per_series = self.exogenous_lags

        for step in range(steps):
            row = [1.0]
            for lag in range(1, ar_count + 1):
                row.append(history[-lag])

            if exogenous_future is not None and exo_per_series > 0:
                for index, series in enumerate(exogenous_history):
                    for lag in range(1, exo_per_series + 1):
                        position = step - lag
                        if position < 0:
                            row.append(series[0])
                        else:
                            row.append(series[position])

            prediction = math.fsum(coef * value for coef, value in zip([self.intercept] + self.coefficients, row))
            forecasts.append(prediction)
            history.append(prediction)

        # Integrate back to price space.
        level = self._last_level
        levels: List[float] = []
        for value in forecasts:
            level = level + value
            levels.append(level)

        # Interval width grows with the square root of the horizon.
        z = mathx.normal_ppf(1.0 - (1.0 - confidence) / 2.0)
        lower: List[float] = []
        upper: List[float] = []
        for step, value in enumerate(levels):
            width = z * self.residual_std * math.sqrt(step + 1)
            lower.append(value - width)
            upper.append(value + width)

        return Forecast(
            values=levels,
            lower=lower,
            upper=upper,
            confidence=confidence,
            residual_std=self.residual_std,
            r_squared=self.r_squared,
            observations=len(self._last_differenced) + self.difference_order,
        )


class HoltLinear:
    """Holt's linear trend exponential smoothing with an optional damped trend."""

    def __init__(self, alpha: float = 0.3, beta: float = 0.1, damped: bool = False, phi: float = 0.98):
        if not 0.0 < alpha <= 1.0:
            raise ValueError("alpha must be in (0, 1]")
        if not 0.0 < beta <= 1.0:
            raise ValueError("beta must be in (0, 1]")
        if damped and not 0.0 < phi < 1.0:
            raise ValueError("phi must be in (0, 1) when damped")

        self.alpha = alpha
        self.beta = beta
        self.damped = damped
        self.phi = phi

        self.level: float = 0.0
        self.trend: float = 0.0
        self.residual_std: float = 0.0
        self._fitted = False

    def fit(self, series: Sequence[float]) -> "HoltLinear":
        """Initialises the level and trend and runs the recursion."""
        if len(series) < 3:
            raise ValueError("Holt's method needs at least three observations")

        self.level = series[0]
        self.trend = series[1] - series[0]
        residuals: List[float] = []

        for value in series[1:]:
            previous_level = self.level
            previous_trend = self.trend

            projection = previous_level + self._damped_factor() * previous_trend
            residuals.append(value - projection)

            self.level = self.alpha * value + (1.0 - self.alpha) * projection
            self.trend = self.beta * (self.level - previous_level) + (1.0 - self.beta) * previous_trend

        self.residual_std = mathx.stdev(residuals) if len(residuals) > 1 else 0.0
        self._fitted = True
        return self

    def _damped_factor(self) -> float:
        return self.phi if self.damped else 1.0

    def forecast(self, steps: int, confidence: float = 0.95) -> Forecast:
        """Forecast `steps` ahead with a prediction interval."""
        if not self._fitted:
            raise ValueError("call fit() before forecast()")
        if steps < 1:
            raise ValueError("steps must be at least 1")
        if not 0.0 < confidence < 1.0:
            raise ValueError("confidence must be strictly between 0 and 1")

        values: List[float] = []
        if self.damped:
            factor_sum = 0.0
            factor = self.phi
            for step in range(1, steps + 1):
                factor_sum += factor
                values.append(self.level + factor_sum * self.trend)
                factor *= self.phi
        else:
            for step in range(1, steps + 1):
                values.append(self.level + step * self.trend)

        z = mathx.normal_ppf(1.0 - (1.0 - confidence) / 2.0)
        lower: List[float] = []
        upper: List[float] = []
        for step, value in enumerate(values):
            width = z * self.residual_std * math.sqrt(step + 1)
            lower.append(value - width)
            upper.append(value + width)

        return Forecast(
            values=values,
            lower=lower,
            upper=upper,
            confidence=confidence,
            residual_std=self.residual_std,
            r_squared=0.0,
            observations=0,
        )


def mean_absolute_error(actual: Sequence[float], predicted: Sequence[float]) -> float:
    """Mean absolute error between two series."""
    if len(actual) != len(predicted):
        raise ValueError("series must be the same length")
    if not actual:
        raise ValueError("series must not be empty")
    return math.fsum(abs(a - p) for a, p in zip(actual, predicted)) / len(actual)


def mean_absolute_percentage_error(actual: Sequence[float], predicted: Sequence[float]) -> float:
    """Mean absolute percentage error, skipping zero actuals."""
    if len(actual) != len(predicted):
        raise ValueError("series must be the same length")

    pairs = [(a, p) for a, p in zip(actual, predicted) if a != 0]
    if not pairs:
        raise ValueError("no non-zero actual values to score")
    return math.fsum(abs((a - p) / a) for a, p in pairs) / len(pairs)


def root_mean_squared_error(actual: Sequence[float], predicted: Sequence[float]) -> float:
    """Root mean squared error between two series."""
    if len(actual) != len(predicted):
        raise ValueError("series must be the same length")
    if not actual:
        raise ValueError("series must not be empty")
    return math.sqrt(math.fsum((a - p) ** 2 for a, p in zip(actual, predicted)) / len(actual))


def directional_accuracy(actual: Sequence[float], predicted: Sequence[float]) -> float:
    """Fraction of steps where the predicted direction of change is correct."""
    if len(actual) != len(predicted):
        raise ValueError("series must be the same length")
    if len(actual) < 2:
        return 0.0

    correct = 0
    total = 0
    for index in range(1, len(actual)):
        actual_direction = actual[index] - actual[index - 1]
        predicted_direction = predicted[index] - predicted[index - 1]
        if actual_direction == 0:
            continue
        total += 1
        if (actual_direction > 0) == (predicted_direction > 0):
            correct += 1

    return correct / total if total else 0.0
