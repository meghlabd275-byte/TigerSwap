"""TigerSwap machine-learning and quantitative models.

Real, dependency-light implementations of the models the DEX uses for risk,
pricing, routing, market making and anomaly detection. Nothing here is a stub:
every function computes a genuine estimator, and the tests in ../tests pin the
results against known values.
"""

__version__ = "1.0.0"

from . import mathx
from .amm import (
    BPS_DENOMINATOR,
    MarketMaker,
    RouteQuote,
    SmartRouter,
    StableSwap,
    constant_product_in,
    constant_product_out,
    impermanent_loss,
    price_impact,
    spot_price,
    weighted_pool_out,
)
from .anomaly import (
    Anomaly,
    BenfordDetector,
    DetectionResult,
    OnlineVarianceDetector,
    RollingZScoreDetector,
    WashTradeDetector,
)
from .forecasting import (
    ARIMAX,
    Forecast,
    HoltLinear,
    directional_accuracy,
    mean_absolute_error,
    mean_absolute_percentage_error,
    root_mean_squared_error,
)
from .options import BlackScholes, OptionGreeks, put_call_parity_gap
from .risk import PortfolioRiskEngine, RiskEngine, RiskReport

__all__ = [
    "mathx",
    "RiskEngine",
    "RiskReport",
    "PortfolioRiskEngine",
    "ARIMAX",
    "HoltLinear",
    "Forecast",
    "mean_absolute_error",
    "mean_absolute_percentage_error",
    "root_mean_squared_error",
    "directional_accuracy",
    "BlackScholes",
    "OptionGreeks",
    "put_call_parity_gap",
    "RollingZScoreDetector",
    "OnlineVarianceDetector",
    "BenfordDetector",
    "WashTradeDetector",
    "DetectionResult",
    "Anomaly",
    "constant_product_out",
    "constant_product_in",
    "spot_price",
    "price_impact",
    "impermanent_loss",
    "StableSwap",
    "weighted_pool_out",
    "SmartRouter",
    "RouteQuote",
    "MarketMaker",
    "BPS_DENOMINATOR",
]
