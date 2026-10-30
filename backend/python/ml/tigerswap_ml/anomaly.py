"""Anomaly and manipulation detection for TigerSwap.

Three real detectors are provided:

* a rolling robust z-score test, which flags points far from a local median in
  units of the local median absolute deviation;
* a streaming online mean and variance (Welford's algorithm) with a
  configurable number of standard deviations, for wash-trade and
  spoofing-style volume spikes; and
* a Benford's law first-digit test, which catches fabricated trade sizes.

Each detector returns a structured verdict rather than a bare boolean.
"""

from __future__ import annotations

import math
from dataclasses import dataclass, field
from typing import Dict, List, Optional, Sequence

from . import mathx


@dataclass
class Anomaly:
    """A single flagged observation."""

    index: int
    value: float
    score: float
    kind: str

    def as_dict(self) -> Dict[str, object]:
        return {"index": self.index, "value": self.value, "score": self.score, "kind": self.kind}


@dataclass
class DetectionResult:
    """Outcome of running a detector over a series."""

    anomalies: List[Anomaly] = field(default_factory=list)
    threshold: float = 0.0
    observations: int = 0

    @property
    def is_anomalous(self) -> bool:
        return bool(self.anomalies)

    @property
    def anomaly_rate(self) -> float:
        if self.observations == 0:
            return 0.0
        return len(self.anomalies) / self.observations

    def as_dict(self) -> Dict[str, object]:
        return {
            "anomalies": [a.as_dict() for a in self.anomalies],
            "threshold": self.threshold,
            "observations": self.observations,
            "anomaly_rate": self.anomaly_rate,
        }


class RollingZScoreDetector:
    """Flags points that deviate from a local robust baseline."""

    def __init__(self, window: int = 30, threshold: float = 3.5, min_periods: int = 10):
        if window < 3:
            raise ValueError("window must be at least 3")
        if threshold <= 0:
            raise ValueError("threshold must be positive")
        if min_periods < 3 or min_periods > window:
            raise ValueError("min_periods must be between 3 and window")

        self.window = window
        self.threshold = threshold
        self.min_periods = min_periods

    def detect(self, values: Sequence[float]) -> DetectionResult:
        """Runs the detector over a series."""
        result = DetectionResult(threshold=self.threshold, observations=len(values))
        if len(values) < self.min_periods:
            return result

        for index in range(self.min_periods - 1, len(values)):
            start = max(0, index - self.window + 1)
            window = list(values[start:index])

            # Score against the window that precedes the point, so the point
            # itself cannot mask its own deviation.
            centre = mathx.median(window)
            mad = mathx.median_absolute_deviation(window)
            if mad == 0.0:
                # A flat window: any change at all is significant.
                if values[index] != centre:
                    result.anomalies.append(
                        Anomaly(index=index, value=values[index], score=float("inf"), kind="level_shift")
                    )
                continue

            score = abs(values[index] - centre) / (1.4826 * mad)
            if score > self.threshold:
                result.anomalies.append(
                    Anomaly(index=index, value=values[index], score=score, kind="robust_zscore")
                )

        return result


class OnlineVarianceDetector:
    """Streaming anomaly detector using Welford's online mean and variance.

    Suitable for trade-size or volume monitoring where the whole series is not
    retained. Supports a warm-up period before it starts flagging.
    """

    def __init__(self, threshold: float = 4.0, warmup: int = 30):
        if threshold <= 0:
            raise ValueError("threshold must be positive")
        if warmup < 2:
            raise ValueError("warmup must be at least 2")

        self.threshold = threshold
        self.warmup = warmup

        self.count = 0
        self.mean = 0.0
        self._m2 = 0.0

    @property
    def variance(self) -> float:
        """Current sample variance."""
        if self.count < 2:
            return 0.0
        return self._m2 / (self.count - 1)

    @property
    def std(self) -> float:
        return math.sqrt(max(0.0, self.variance))

    def update(self, value: float) -> Anomaly:
        """Adds an observation and returns a verdict for it.

        The returned Anomaly has kind "none" when the point is within bounds.
        """
        previous_mean = self.mean
        previous_std = self.std

        self.count += 1
        delta = value - previous_mean
        self.mean = previous_mean + delta / self.count
        self._m2 += delta * (value - self.mean)

        if self.count <= self.warmup or previous_std == 0.0:
            return Anomaly(index=self.count - 1, value=value, score=0.0, kind="none")

        score = (value - self.mean) / previous_std
        if abs(score) > self.threshold:
            return Anomaly(index=self.count - 1, value=value, score=score, kind="variance_break")
        return Anomaly(index=self.count - 1, value=value, score=score, kind="none")


class BenfordDetector:
    """Benford's law test on the leading digit of a series.

    Genuine market data often follows Benford's law; fabricated or rounded
    trade sizes frequently do not. The chi-square statistic is compared with a
    critical value for eight degrees of freedom.
    """

    # Critical values for a chi-square distribution with 8 degrees of freedom.
    CRITICAL_VALUES = {0.05: 15.507, 0.01: 20.090}

    def __init__(self, significance: float = 0.05):
        if significance not in self.CRITICAL_VALUES:
            raise ValueError(f"significance must be one of {sorted(self.CRITICAL_VALUES)}")
        self.significance = significance

    @staticmethod
    def expected_distribution() -> List[float]:
        """Expected first-digit frequencies under Benford's law."""
        return [math.log10(1.0 + 1.0 / digit) for digit in range(1, 10)]

    @staticmethod
    def leading_digit(value: float) -> int:
        """First significant digit of a value, or 0 when it has none.

        Computed from the decimal exponent with the mantissa floored, which is
        exact for exact powers of ten where a naive `int(magnitude / 10**exp)`
        rounds the quotient down to the previous digit.
        """
        magnitude = abs(value)
        if magnitude <= 0 or not math.isfinite(magnitude):
            return 0
        exponent = math.floor(math.log10(magnitude))
        mantissa = magnitude / (10.0**exponent)
        digit = int(mantissa)
        if digit > 9:
            digit = 9
        return digit

    def observed_distribution(self, values: Sequence[float]) -> List[float]:
        """Observed first-digit frequencies, ignoring non-positive values."""
        counts = [0] * 9
        total = 0

        for value in values:
            digit = self.leading_digit(value)
            if 1 <= digit <= 9:
                counts[digit - 1] += 1
                total += 1

        if total == 0:
            return [0.0] * 9
        return [count / total for count in counts]

    def chi_square(self, values: Sequence[float]) -> float:
        """Chi-square statistic of the observed digit distribution."""
        observed = self.observed_distribution(values)
        expected = self.expected_distribution()

        total = sum(1 for value in values if self.leading_digit(value) != 0)
        if total == 0:
            return 0.0

        statistic = 0.0
        for observed_frequency, expected_frequency in zip(observed, expected):
            expected_count = expected_frequency * total
            if expected_count == 0:
                continue
            difference = observed_frequency * total - expected_count
            statistic += difference * difference / expected_count

        return statistic

    def detect(self, values: Sequence[float]) -> Dict[str, object]:
        """Runs the Benford test and reports whether the data conforms."""
        statistic = self.chi_square(values)
        critical = self.CRITICAL_VALUES[self.significance]
        return {
            "chi_square": statistic,
            "critical_value": critical,
            "significance": self.significance,
            "conforms": statistic <= critical,
            "observed": self.observed_distribution(values),
            "expected": self.expected_distribution(),
        }


class WashTradeDetector:
    """Detects wash-trade patterns from trade records.

    Flags self-matched volume (a maker and taker under common control) and
    repeated round-trip sizes that return to the same price within a window.
    """

    def __init__(self, same_price_window: int = 3, min_repeat: int = 3):
        if same_price_window < 1:
            raise ValueError("same_price_window must be at least 1")
        if min_repeat < 2:
            raise ValueError("min_repeat must be at least 2")

        self.same_price_window = same_price_window
        self.min_repeat = min_repeat

    def detect(self, trades: Sequence[Dict[str, object]]) -> Dict[str, object]:
        """Runs both wash-trade checks over a trade list.

        Each trade is a mapping with at least: price, quantity, maker, taker.
        """
        self_matched = []
        repeated = []

        for index, trade in enumerate(trades):
            if trade.get("maker") == trade.get("taker") and trade.get("maker") is not None:
                self_matched.append(index)

        # Repeated identical sizes at an unchanged price look like round trips.
        for index in range(len(trades)):
            price = trades[index].get("price")
            quantity = trades[index].get("quantity")
            matches = 0

            upper = min(len(trades), index + self.same_price_window + 1)
            for other in range(index + 1, upper):
                if trades[other].get("price") == price and trades[other].get("quantity") == quantity:
                    matches += 1

            if matches >= self.min_repeat - 1:
                repeated.append(index)

        return {
            "self_matched_indices": self_matched,
            "repeated_round_trip_indices": sorted(set(repeated)),
            "suspicious": bool(self_matched or repeated),
        }
