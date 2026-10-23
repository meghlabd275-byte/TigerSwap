"""Numerical helpers shared by TigerSwap's analytics and ML models.

Implemented with the standard library only, so the models run identically in a
minimal container and in a full research environment.
"""

from __future__ import annotations

import math
from typing import List, Sequence

Vector = Sequence[float]
Matrix = Sequence[Sequence[float]]


def mean(values: Vector) -> float:
    """Arithmetic mean. Raises on an empty input."""
    if not values:
        raise ValueError("mean requires at least one value")
    return math.fsum(values) / len(values)


def variance(values: Vector, sample: bool = True) -> float:
    """Variance, using Bessel's correction when `sample` is true."""
    if len(values) < 2:
        return 0.0
    mu = mean(values)
    total = math.fsum((value - mu) ** 2 for value in values)
    divisor = len(values) - 1 if sample else len(values)
    return total / divisor


def stdev(values: Vector, sample: bool = True) -> float:
    """Standard deviation."""
    return math.sqrt(max(0.0, variance(values, sample)))


def covariance(xs: Vector, ys: Vector, sample: bool = True) -> float:
    """Sample covariance of two equal-length series."""
    if len(xs) != len(ys):
        raise ValueError("covariance requires equal-length series")
    if len(xs) < 2:
        return 0.0

    mx = mean(xs)
    my = mean(ys)
    total = math.fsum((x - mx) * (y - my) for x, y in zip(xs, ys))
    divisor = len(xs) - 1 if sample else len(xs)
    return total / divisor


def correlation(xs: Vector, ys: Vector) -> float:
    """Pearson correlation. Returns 0 when either series is constant."""
    sx = stdev(xs)
    sy = stdev(ys)
    if sx == 0.0 or sy == 0.0:
        return 0.0
    return covariance(xs, ys) / (sx * sy)


def percentile(values: Vector, fraction: float) -> float:
    """Linear-interpolated percentile. `fraction` is in [0, 1]."""
    if not values:
        raise ValueError("percentile requires at least one value")
    if not 0.0 <= fraction <= 1.0:
        raise ValueError("fraction must be between 0 and 1")

    ordered = sorted(values)
    if len(ordered) == 1:
        return float(ordered[0])

    position = fraction * (len(ordered) - 1)
    lower = int(math.floor(position))
    upper = int(math.ceil(position))
    if lower == upper:
        return float(ordered[lower])

    weight = position - lower
    return ordered[lower] * (1.0 - weight) + ordered[upper] * weight


def median(values: Vector) -> float:
    """Median value."""
    return percentile(values, 0.5)


def median_absolute_deviation(values: Vector) -> float:
    """Median absolute deviation, a robust spread estimate."""
    if not values:
        raise ValueError("mad requires at least one value")
    centre = median(values)
    return median([abs(value - centre) for value in values])


def modified_zscores(values: Vector) -> List[float]:
    """Robust z-scores based on the median absolute deviation.

    The 1.4826 factor rescales the MAD so it estimates the standard deviation
    for normally distributed data.
    """
    if not values:
        return []
    centre = median(values)
    mad = median_absolute_deviation(values)
    if mad == 0.0:
        return [0.0 for _ in values]
    scale = 1.4826 * mad
    return [(value - centre) / scale for value in values]


def zscores(values: Vector) -> List[float]:
    """Standard z-scores."""
    if not values:
        return []
    mu = mean(values)
    sigma = stdev(values)
    if sigma == 0.0:
        return [0.0 for _ in values]
    return [(value - mu) / sigma for value in values]


def ewma(values: Vector, alpha: float) -> List[float]:
    """Exponentially weighted moving average, seeded with the first value."""
    if not 0.0 < alpha <= 1.0:
        raise ValueError("alpha must be in (0, 1]")
    if not values:
        return []

    out = [float(values[0])]
    for value in values[1:]:
        out.append(alpha * value + (1.0 - alpha) * out[-1])
    return out


def ewma_variance(values: Vector, decay: float = 0.94) -> float:
    """RiskMetrics-style exponentially weighted variance."""
    if len(values) < 2:
        return 0.0
    if not 0.0 < decay < 1.0:
        raise ValueError("decay must be in (0, 1)")

    mu = mean(values)
    weighted = 0.0
    weight_sum = 0.0
    weight = 1.0

    for value in reversed(values):
        weighted += weight * (value - mu) ** 2
        weight_sum += weight
        weight *= decay

    return weighted / weight_sum if weight_sum else 0.0


def simple_returns(prices: Vector) -> List[float]:
    """Period-over-period simple returns."""
    out: List[float] = []
    for previous, current in zip(prices, prices[1:]):
        if previous == 0:
            raise ValueError("cannot compute a return from a zero price")
        out.append(current / previous - 1.0)
    return out


def log_returns(prices: Vector) -> List[float]:
    """Continuously compounded returns."""
    out: List[float] = []
    for previous, current in zip(prices, prices[1:]):
        if previous <= 0 or current <= 0:
            raise ValueError("log returns require strictly positive prices")
        out.append(math.log(current / previous))
    return out


def normal_pdf(x: float, mu: float = 0.0, sigma: float = 1.0) -> float:
    """Gaussian density."""
    if sigma <= 0:
        raise ValueError("sigma must be positive")
    z = (x - mu) / sigma
    return math.exp(-0.5 * z * z) / (sigma * math.sqrt(2.0 * math.pi))


def normal_cdf(x: float, mu: float = 0.0, sigma: float = 1.0) -> float:
    """Gaussian CDF via the error function."""
    if sigma <= 0:
        raise ValueError("sigma must be positive")
    return 0.5 * (1.0 + math.erf((x - mu) / (sigma * math.sqrt(2.0))))


def normal_ppf(p: float, mu: float = 0.0, sigma: float = 1.0) -> float:
    """Inverse Gaussian CDF (Acklam's rational approximation).

    Accurate to roughly 1e-9 across the open unit interval.
    """
    if not 0.0 < p < 1.0:
        raise ValueError("p must be strictly between 0 and 1")
    if sigma <= 0:
        raise ValueError("sigma must be positive")

    a = [-3.969683028665376e01, 2.209460984245205e02, -2.759285104469687e02,
         1.383577518672690e02, -3.066479806614716e01, 2.506628277459239e00]
    b = [-5.447609879822406e01, 1.615858368580409e02, -1.556989798598866e02,
         6.680131188771972e01, -1.328068155288572e01]
    c = [-7.784894002430293e-03, -3.223964580411365e-01, -2.400758277161838e00,
         -2.549732539343734e00, 4.374664141464968e00, 2.938163982698783e00]
    d = [7.784695709041462e-03, 3.224671290700398e-01, 2.445134137142996e00,
         3.754408661907416e00]

    lower = 0.02425
    upper = 1.0 - lower

    if p < lower:
        q = math.sqrt(-2.0 * math.log(p))
        z = (((((c[0] * q + c[1]) * q + c[2]) * q + c[3]) * q + c[4]) * q + c[5]) / (
            (((d[0] * q + d[1]) * q + d[2]) * q + d[3]) * q + 1.0
        )
    elif p > upper:
        q = math.sqrt(-2.0 * math.log(1.0 - p))
        z = -(((((c[0] * q + c[1]) * q + c[2]) * q + c[3]) * q + c[4]) * q + c[5]) / (
            (((d[0] * q + d[1]) * q + d[2]) * q + d[3]) * q + 1.0
        )
    else:
        q = p - 0.5
        r = q * q
        z = (((((a[0] * r + a[1]) * r + a[2]) * r + a[3]) * r + a[4]) * r + a[5]) * q / (
            ((((b[0] * r + b[1]) * r + b[2]) * r + b[3]) * r + b[4]) * r + 1.0
        )

    # One Halley step refines the tails.
    error = normal_cdf(z) - p
    u = error * math.sqrt(2.0 * math.pi) * math.exp(0.5 * z * z)
    z = z - u / (1.0 + 0.5 * z * u)

    return mu + sigma * z


def dot(a: Vector, b: Vector) -> float:
    """Dot product."""
    if len(a) != len(b):
        raise ValueError("dot product requires equal lengths")
    return math.fsum(x * y for x, y in zip(a, b))


def matmul(a: Matrix, b: Matrix) -> List[List[float]]:
    """Matrix product."""
    if not a or not b:
        return []
    if len(a[0]) != len(b):
        raise ValueError("inner dimensions do not match")

    columns = list(zip(*b))
    return [[math.fsum(x * y for x, y in zip(row, column)) for column in columns] for row in a]


def transpose(matrix: Matrix) -> List[List[float]]:
    """Matrix transpose."""
    return [list(row) for row in zip(*matrix)] if matrix else []


def solve_linear(a: Matrix, b: Vector) -> List[float]:
    """Solves A x = b by Gaussian elimination with partial pivoting."""
    size = len(a)
    if size == 0:
        return []
    if any(len(row) != size for row in a):
        raise ValueError("matrix must be square")
    if len(b) != size:
        raise ValueError("right-hand side has the wrong length")

    matrix = [list(row) + [b[i]] for i, row in enumerate(a)]

    for column in range(size):
        pivot_row = max(range(column, size), key=lambda r: abs(matrix[r][column]))
        if abs(matrix[pivot_row][column]) < 1e-14:
            raise ValueError("matrix is singular")

        matrix[column], matrix[pivot_row] = matrix[pivot_row], matrix[column]

        pivot = matrix[column][column]
        for r in range(column + 1, size):
            factor = matrix[r][column] / pivot
            if factor == 0.0:
                continue
            for c in range(column, size + 1):
                matrix[r][c] -= factor * matrix[column][c]

    solution = [0.0] * size
    for row in range(size - 1, -1, -1):
        total = math.fsum(matrix[row][c] * solution[c] for c in range(row + 1, size))
        solution[row] = (matrix[row][size] - total) / matrix[row][row]

    return solution


def invert(matrix: Matrix) -> List[List[float]]:
    """Matrix inverse via Gauss-Jordan elimination."""
    size = len(matrix)
    if size == 0:
        return []
    if any(len(row) != size for row in matrix):
        raise ValueError("matrix must be square")

    augmented = [
        list(row) + [1.0 if i == j else 0.0 for j in range(size)]
        for i, row in enumerate(matrix)
    ]

    for column in range(size):
        pivot_row = max(range(column, size), key=lambda r: abs(augmented[r][column]))
        if abs(augmented[pivot_row][column]) < 1e-14:
            raise ValueError("matrix is singular")

        augmented[column], augmented[pivot_row] = augmented[pivot_row], augmented[column]

        pivot = augmented[column][column]
        augmented[column] = [value / pivot for value in augmented[column]]

        for row in range(size):
            if row == column:
                continue
            factor = augmented[row][column]
            if factor == 0.0:
                continue
            augmented[row] = [
                value - factor * base
                for value, base in zip(augmented[row], augmented[column])
            ]

    return [row[size:] for row in augmented]


def covariance_matrix(series: Sequence[Vector], sample: bool = True) -> List[List[float]]:
    """Covariance matrix of several equal-length series."""
    count = len(series)
    if count == 0:
        return []
    matrix = [[0.0] * count for _ in range(count)]
    for i in range(count):
        for j in range(i, count):
            value = covariance(series[i], series[j], sample)
            matrix[i][j] = value
            matrix[j][i] = value
    return matrix


def clip(value: float, lower: float, upper: float) -> float:
    """Clamps a value into [lower, upper]."""
    if lower > upper:
        raise ValueError("lower bound exceeds upper bound")
    return max(lower, min(upper, value))


def sigmoid(x: float) -> float:
    """Numerically stable logistic function."""
    if x >= 0:
        return 1.0 / (1.0 + math.exp(-x))
    exponential = math.exp(x)
    return exponential / (1.0 + exponential)


def logit(p: float) -> float:
    """Inverse logistic function."""
    if not 0.0 < p < 1.0:
        raise ValueError("p must be strictly between 0 and 1")
    return math.log(p / (1.0 - p))
