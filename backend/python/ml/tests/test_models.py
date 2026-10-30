"""Test suite for the TigerSwap ML and quantitative models.

Every test asserts a real, checkable property: closed-form option values from
published references, put-call parity, the AMM invariant, recovery of a known
implied volatility, statistical estimators against hand-computed values, and
detector behaviour on both clean and manipulated data.
"""

import math

import pytest

from tigerswap_ml import mathx
from tigerswap_ml.amm import (
    MarketMaker,
    SmartRouter,
    StableSwap,
    constant_product_in,
    constant_product_out,
    impermanent_loss,
    price_impact,
    spot_price,
    weighted_pool_out,
)
from tigerswap_ml.anomaly import (
    BenfordDetector,
    OnlineVarianceDetector,
    RollingZScoreDetector,
    WashTradeDetector,
)
from tigerswap_ml.forecasting import (
    ARIMAX,
    HoltLinear,
    directional_accuracy,
    mean_absolute_error,
    mean_absolute_percentage_error,
    root_mean_squared_error,
)
from tigerswap_ml.options import BlackScholes, put_call_parity_gap
from tigerswap_ml.risk import PortfolioRiskEngine, RiskEngine


# --------------------------------------------------------------------------
# mathx
# --------------------------------------------------------------------------


def test_mean_and_variance_match_hand_computation():
    values = [2.0, 4.0, 4.0, 4.0, 5.0, 5.0, 7.0, 9.0]
    assert mathx.mean(values) == pytest.approx(5.0)
    # Population variance of this classic series is exactly 4.
    assert mathx.variance(values, sample=False) == pytest.approx(4.0)
    assert mathx.stdev(values, sample=False) == pytest.approx(2.0)


def test_percentile_interpolates():
    values = [1.0, 2.0, 3.0, 4.0]
    assert mathx.percentile(values, 0.0) == pytest.approx(1.0)
    assert mathx.percentile(values, 1.0) == pytest.approx(4.0)
    assert mathx.percentile(values, 0.5) == pytest.approx(2.5)


def test_median_absolute_deviation_is_robust_to_an_outlier():
    values = [1.0, 2.0, 3.0, 4.0, 5.0]
    assert mathx.median_absolute_deviation(values) == pytest.approx(1.0)

    with_outlier = [1.0, 2.0, 3.0, 4.0, 5.0, 10_000.0]
    # The MAD barely moves, unlike the standard deviation.
    assert mathx.median_absolute_deviation(with_outlier) == pytest.approx(1.5)
    assert mathx.stdev(with_outlier) > 1000


def test_correlation_of_perfectly_linear_series():
    xs = [1.0, 2.0, 3.0, 4.0]
    ys = [2.0, 4.0, 6.0, 8.0]
    assert mathx.correlation(xs, ys) == pytest.approx(1.0)
    assert mathx.correlation(xs, [-2.0, -4.0, -6.0, -8.0]) == pytest.approx(-1.0)


def test_correlation_is_zero_for_a_constant_series():
    assert mathx.correlation([1.0, 1.0, 1.0], [1.0, 2.0, 3.0]) == 0.0


def test_normal_cdf_and_ppf_are_inverses():
    for p in (0.001, 0.025, 0.25, 0.5, 0.75, 0.975, 0.999):
        z = mathx.normal_ppf(p)
        assert mathx.normal_cdf(z) == pytest.approx(p, abs=1e-7)


def test_normal_cdf_known_values():
    assert mathx.normal_cdf(0.0) == pytest.approx(0.5)
    assert mathx.normal_cdf(1.96) == pytest.approx(0.975, abs=1e-4)


def test_solve_linear_recovers_a_known_solution():
    a = [[2.0, 1.0], [1.0, 3.0]]
    b = [5.0, 10.0]
    solution = mathx.solve_linear(a, b)
    assert solution[0] == pytest.approx(1.0)
    assert solution[1] == pytest.approx(3.0)


def test_solve_linear_rejects_a_singular_matrix():
    with pytest.raises(ValueError):
        mathx.solve_linear([[1.0, 2.0], [2.0, 4.0]], [1.0, 2.0])


def test_invert_produces_the_identity():
    matrix = [[4.0, 7.0], [2.0, 6.0]]
    inverse = mathx.invert(matrix)
    product = mathx.matmul(matrix, inverse)
    assert product[0][0] == pytest.approx(1.0)
    assert product[0][1] == pytest.approx(0.0, abs=1e-12)
    assert product[1][0] == pytest.approx(0.0, abs=1e-12)
    assert product[1][1] == pytest.approx(1.0)


def test_ewma_seeds_with_the_first_value():
    values = [10.0, 20.0, 30.0]
    result = mathx.ewma(values, 0.5)
    assert result[0] == 10.0
    assert result[1] == pytest.approx(15.0)
    assert result[2] == pytest.approx(22.5)


def test_sigmoid_is_stable_in_both_tails():
    assert mathx.sigmoid(0.0) == pytest.approx(0.5)
    assert mathx.sigmoid(-1000.0) == pytest.approx(0.0, abs=1e-12)
    assert mathx.sigmoid(1000.0) == pytest.approx(1.0, abs=1e-12)


def test_log_returns_are_positive_for_rising_prices():
    result = mathx.log_returns([100.0, 110.0])
    assert result[0] == pytest.approx(math.log(1.1))


# --------------------------------------------------------------------------
# risk
# --------------------------------------------------------------------------


def test_historical_var_and_expected_shortfall_ordering():
    # A left tail of large losses.
    returns = [-0.10, -0.08, -0.02, 0.0, 0.01, 0.02, 0.03, 0.04, 0.05, 0.06]
    engine = RiskEngine(periods_per_year=252)

    var95 = engine.historical_var(returns, 0.95)
    es95 = engine.expected_shortfall(returns, 0.95)

    # Expected shortfall is at least as large as VaR.
    assert es95 >= var95
    assert var95 > 0


def test_parametric_var_matches_the_normal_formula():
    returns = [-0.05, -0.02, 0.0, 0.01, 0.02, 0.03, 0.04, 0.05]
    engine = RiskEngine()
    mu = mathx.mean(returns)
    sigma = mathx.stdev(returns)

    expected = -(mu + sigma * mathx.normal_ppf(0.05))
    assert engine.parametric_var(returns, 0.95) == pytest.approx(expected)


def test_max_drawdown_on_a_known_equity_path():
    # +10%, then -50%: the peak-to-trough decline is 50%.
    returns = [0.10, -0.50, 0.05]
    engine = RiskEngine()
    assert engine.max_drawdown(returns) == pytest.approx(0.5)


def test_max_drawdown_is_zero_for_a_monotonic_rise():
    engine = RiskEngine()
    assert engine.max_drawdown([0.01, 0.02, 0.03]) == pytest.approx(0.0)


def test_sharpe_ratio_sign():
    positive = [0.01] * 20
    engine = RiskEngine(periods_per_year=252)
    # A constant positive return has zero volatility, so Sharpe is defined as 0.
    assert engine.sharpe_ratio(positive) == 0.0

    mixed = [0.02, -0.01, 0.03, -0.005, 0.01, 0.015]
    assert engine.sharpe_ratio(mixed) > 0


def test_skewness_of_a_symmetric_series_is_zero():
    engine = RiskEngine()
    assert engine.skewness([-2.0, -1.0, 0.0, 1.0, 2.0]) == pytest.approx(0.0)


def test_kurtosis_of_a_normal_like_sample_is_small():
    # A short symmetric sample has near-zero excess kurtosis.
    engine = RiskEngine()
    value = engine.kurtosis([-2.0, -1.0, 0.0, 1.0, 2.0])
    assert abs(value) < 3.0


def test_liquidation_probability_increases_with_volatility():
    engine = RiskEngine()
    low = engine.liquidation_probability(100.0, 80.0, 0.01, 10.0, is_long=True)
    high = engine.liquidation_probability(100.0, 80.0, 0.05, 10.0, is_long=True)

    assert 0.0 <= low <= high <= 1.0
    assert high > low


def test_liquidation_probability_is_one_when_already_breached():
    engine = RiskEngine()
    # A long whose liquidation price is above the entry is already liquidated.
    assert engine.liquidation_probability(100.0, 110.0, 0.02, 5.0, is_long=True) == 1.0


def test_liquidation_probability_for_a_short():
    engine = RiskEngine()
    # A short liquidates on the way up.
    probability = engine.liquidation_probability(100.0, 120.0, 0.03, 5.0, is_long=False)
    assert 0.0 < probability < 1.0

    # A short with the liquidation price below entry is already breached.
    assert engine.liquidation_probability(100.0, 90.0, 0.03, 5.0, is_long=False) == 1.0


def test_portfolio_volatility_of_a_single_asset():
    engine = PortfolioRiskEngine()
    covariance = [[0.04]]
    assert engine.portfolio_volatility([1.0], covariance) == pytest.approx(0.2)


def test_portfolio_diversification_reduces_risk():
    # Two assets, each with 20% volatility, uncorrelated.
    covariance = [[0.04, 0.0], [0.0, 0.04]]
    engine = PortfolioRiskEngine()

    concentrated = engine.portfolio_volatility([1.0, 0.0], covariance)
    split = engine.portfolio_volatility([0.5, 0.5], covariance)

    # Uncorrelated equal weights reduce volatility by sqrt(2).
    assert split == pytest.approx(concentrated / math.sqrt(2))
    assert split < concentrated


def test_component_risk_sums_to_portfolio_volatility():
    covariance = [[0.04, 0.01], [0.01, 0.09]]
    weights = [0.6, 0.4]
    engine = PortfolioRiskEngine()

    sigma = engine.portfolio_volatility(weights, covariance)
    components = engine.component_risk(weights, covariance)

    assert sum(components) == pytest.approx(sigma)


def test_risk_report_is_complete():
    returns = [-0.03, -0.01, 0.0, 0.005, 0.01, 0.02, -0.015, 0.008]
    report = RiskEngine(periods_per_year=365).report(returns)
    assert report.observations == len(returns)
    assert report.annualised_volatility >= report.volatility
    assert report.expected_shortfall_95 >= report.var_95
    assert set(report.as_dict()) >= {"var_95", "var_99", "max_drawdown", "sharpe"}


# --------------------------------------------------------------------------
# options
# --------------------------------------------------------------------------


def test_black_scholes_call_matches_published_value():
    # Classic reference: S=100, K=100, r=5%, sigma=20%, T=1 -> call ~10.4506.
    price = BlackScholes.call_price(100.0, 100.0, 0.05, 0.20, 1.0)
    assert price == pytest.approx(10.4506, abs=1e-3)


def test_black_scholes_put_matches_published_value():
    # The same parameters give a put of about 5.5735.
    price = BlackScholes.put_price(100.0, 100.0, 0.05, 0.20, 1.0)
    assert price == pytest.approx(5.5735, abs=1e-3)


def test_put_call_parity_holds_exactly():
    spot, strike, rate, volatility, tenor = 100.0, 95.0, 0.03, 0.25, 0.5
    call = BlackScholes.call_price(spot, strike, rate, volatility, tenor)
    put = BlackScholes.put_price(spot, strike, rate, volatility, tenor)

    gap = put_call_parity_gap(call, put, spot, strike, rate, tenor)
    assert gap == pytest.approx(0.0, abs=1e-9)


def test_call_delta_is_between_zero_and_one():
    greeks = BlackScholes.greeks(100.0, 100.0, 0.05, 0.2, 1.0, "call")
    assert 0.0 < greeks.delta < 1.0
    assert greeks.gamma > 0.0
    assert greeks.vega > 0.0


def test_put_delta_is_between_minus_one_and_zero():
    greeks = BlackScholes.greeks(100.0, 100.0, 0.05, 0.2, 1.0, "put")
    assert -1.0 < greeks.delta < 0.0


def test_gamma_and_vega_match_finite_differences():
    spot, strike, rate, volatility, tenor = 100.0, 100.0, 0.05, 0.2, 1.0
    greeks = BlackScholes.greeks(spot, strike, rate, volatility, tenor, "call")

    step = 0.01
    up = BlackScholes.call_price(spot + step, strike, rate, volatility, tenor)
    down = BlackScholes.call_price(spot - step, strike, rate, volatility, tenor)
    numeric_delta = (up - down) / (2 * step)
    numeric_gamma = (up - 2 * greeks.delta * 0 + down - 2 * BlackScholes.call_price(
        spot, strike, rate, volatility, tenor
    )) / (step**2)

    assert greeks.delta == pytest.approx(numeric_delta, abs=1e-3)
    assert greeks.gamma == pytest.approx(numeric_gamma, abs=1e-3)

    vol_step = 0.001
    vega_up = BlackScholes.call_price(spot, strike, rate, volatility + vol_step, tenor)
    vega_down = BlackScholes.call_price(spot, strike, rate, volatility - vol_step, tenor)
    numeric_vega = (vega_up - vega_down) / (2 * vol_step)
    assert greeks.vega == pytest.approx(numeric_vega, abs=1e-2)


def test_implied_volatility_recovers_the_input():
    spot, strike, rate, volatility, tenor = 100.0, 105.0, 0.04, 0.35, 0.75
    market_price = BlackScholes.call_price(spot, strike, rate, volatility, tenor)

    recovered = BlackScholes.implied_volatility(
        market_price, spot, strike, rate, tenor, "call"
    )
    assert recovered == pytest.approx(volatility, abs=1e-5)


def test_implied_volatility_recovers_for_a_put():
    spot, strike, rate, volatility, tenor = 50.0, 45.0, 0.02, 0.45, 0.25
    market_price = BlackScholes.put_price(spot, strike, rate, volatility, tenor)

    recovered = BlackScholes.implied_volatility(
        market_price, spot, strike, rate, tenor, "put"
    )
    assert recovered == pytest.approx(volatility, abs=1e-5)


def test_implied_volatility_rejects_a_price_above_the_upper_bound():
    # A call can never cost more than the spot.
    with pytest.raises(ValueError):
        BlackScholes.implied_volatility(200.0, 100.0, 100.0, 0.05, 1.0, "call")


def test_dividend_yield_lowers_the_call_price():
    without = BlackScholes.call_price(100.0, 100.0, 0.05, 0.2, 1.0, 0.0)
    with_dividend = BlackScholes.call_price(100.0, 100.0, 0.05, 0.2, 1.0, 0.05)
    assert with_dividend < without


# --------------------------------------------------------------------------
# anomaly detection
# --------------------------------------------------------------------------


def test_rolling_zscore_flags_an_injected_spike():
    clean = [100.0 + (i % 3) * 0.1 for i in range(60)]
    spiked = clean.copy()
    spiked[45] = 500.0

    detector = RollingZScoreDetector(window=30, threshold=3.5)
    result = detector.detect(spiked)

    assert result.is_anomalous
    assert any(a.index == 45 for a in result.anomalies)


def test_rolling_zscore_leaves_clean_data_alone():
    clean = [100.0 + (i % 5) * 0.2 for i in range(80)]
    result = RollingZScoreDetector(window=30, threshold=4.0).detect(clean)
    assert not result.is_anomalous


def test_rolling_zscore_detects_a_level_shift_in_a_flat_series():
    flat = [50.0] * 40 + [90.0] * 5
    result = RollingZScoreDetector(window=20, min_periods=10).detect(flat)
    assert result.is_anomalous


def test_online_variance_detector_tracks_mean_and_std():
    detector = OnlineVarianceDetector(threshold=4.0, warmup=5)
    for value in [10.0, 12.0, 14.0, 16.0, 18.0, 20.0]:
        detector.update(value)

    assert detector.count == 6
    assert detector.mean == pytest.approx(15.0)
    assert detector.std > 0.0


def test_online_variance_detector_flags_a_large_deviation():
    detector = OnlineVarianceDetector(threshold=3.0, warmup=10)
    for value in [100.0 + (i % 2) for i in range(40)]:
        detector.update(value)

    verdict = detector.update(1000.0)
    assert verdict.kind == "variance_break"


def test_benford_detector_accepts_benford_distributed_data():
    # Powers of two are a classic Benford-conforming sequence.
    values = [2**n for n in range(1, 400)]
    result = BenfordDetector().detect(values)
    assert result["conforms"]
    assert result["chi_square"] < result["critical_value"]


def test_benford_detector_rejects_uniform_first_digits():
    # Uniformly distributed values do not follow Benford's law.
    values = [float(n) for n in range(1, 900) if n % 10 == 5]
    result = BenfordDetector().detect(values)
    assert not result["conforms"]


def test_benford_expected_distribution_sums_to_one():
    expected = BenfordDetector.expected_distribution()
    assert sum(expected) == pytest.approx(1.0)
    assert expected[0] == pytest.approx(math.log10(2.0))


def test_wash_trade_detector_flags_self_matching():
    trades = [
        {"price": 100, "quantity": 1, "maker": "a", "taker": "b"},
        {"price": 100, "quantity": 1, "maker": "x", "taker": "x"},
    ]
    result = WashTradeDetector().detect(trades)
    assert result["suspicious"]
    assert 1 in result["self_matched_indices"]


def test_wash_trade_detector_flags_repeated_round_trips():
    trades = [
        {"price": 100, "quantity": 5, "maker": "a", "taker": "b"},
        {"price": 100, "quantity": 5, "maker": "c", "taker": "d"},
        {"price": 100, "quantity": 5, "maker": "e", "taker": "f"},
    ]
    result = WashTradeDetector(same_price_window=3, min_repeat=3).detect(trades)
    assert result["suspicious"]


def test_wash_trade_detector_leaves_normal_flow_alone():
    trades = [
        {"price": 100 + i, "quantity": i + 1, "maker": f"m{i}", "taker": f"t{i}"}
        for i in range(10)
    ]
    result = WashTradeDetector().detect(trades)
    assert not result["suspicious"]


# --------------------------------------------------------------------------
# AMM
# --------------------------------------------------------------------------


def test_constant_product_out_matches_the_formula():
    # x*y=k with 0.3% fee: out = (y * in*0.997) / (x + in*0.997)
    amount_in, reserve_in, reserve_out = 100.0, 10_000.0, 10_000.0
    expected = (reserve_out * amount_in * 0.997) / (reserve_in + amount_in * 0.997)
    assert constant_product_out(amount_in, reserve_in, reserve_out, 30) == pytest.approx(expected)


def test_constant_product_round_trip_is_consistent():
    reserve_in, reserve_out = 1_000_000.0, 500_000.0
    out = constant_product_out(1000.0, reserve_in, reserve_out, 30)
    required = constant_product_in(out, reserve_in, reserve_out, 30)
    # A round trip through a fee-charging pool cannot be profitable.
    assert required >= 1000.0 * (1.0 - 1e-6)
    assert required == pytest.approx(1000.0, rel=1e-3)


def test_spot_price_is_the_reserve_ratio():
    assert spot_price(2_000.0, 1_000.0) == pytest.approx(0.5)


def test_price_impact_grows_with_size():
    small = price_impact(10.0, 1_000_000.0, 1_000_000.0)
    large = price_impact(100_000.0, 1_000_000.0, 1_000_000.0)
    assert large > small
    assert 0.0 <= small <= 1.0


def test_impermanent_loss_is_zero_at_the_initial_price():
    assert impermanent_loss(1.0) == pytest.approx(0.0)


def test_impermanent_loss_matches_the_known_2x_case():
    # A 2x price move produces the well-known ~5.72% loss.
    assert impermanent_loss(2.0) == pytest.approx(-0.0572, abs=1e-3)


def test_impermanent_loss_is_symmetric_in_the_price_ratio():
    up = impermanent_loss(2.0)
    down = impermanent_loss(0.5)
    assert up == pytest.approx(down)


def test_stable_swap_invariant_of_a_balanced_pool():
    pool = StableSwap(amplification=100.0)
    balances = [1_000_000.0, 1_000_000.0]
    d = pool.invariant_d(balances)
    # For a balanced two-token pool, D equals the total balance.
    assert d == pytest.approx(2_000_000.0, rel=1e-6)


def test_stable_swap_gives_low_slippage():
    pool = StableSwap(amplification=100.0, fee_bps=1)
    balances = [1_000_000.0, 1_000_000.0]
    out = pool.out_amount(10_000.0, 0, 1, balances)

    # Stable pools should deliver close to one-for-one, far better than a
    # constant-product pool at the same size.
    cp_out = constant_product_out(10_000.0, 1_000_000.0, 1_000_000.0, 1)
    assert out > cp_out
    assert out == pytest.approx(10_000.0, rel=1e-3)


def test_weighted_pool_80_20_behaves_as_expected():
    # 80/20 pool: 800k of token A, 200k of token B.
    out = weighted_pool_out(1000.0, 800_000.0, 200_000.0, 0.8, 0.2, 30)
    assert out > 0
    # The marginal price in an 80/20 pool is (rin/win)/(rout/wout) = 1.0 here,
    # so the realised average price must sit just below it after the fee.
    marginal = (800_000.0 / 0.8) / (200_000.0 / 0.2)
    assert marginal == pytest.approx(1.0)
    assert out / 1000.0 < marginal


def test_smart_router_never_does_worse_than_a_single_venue():
    venues = [
        {"name": "v2", "reserve_in": 1_000_000.0, "reserve_out": 1_000_000.0, "fee_bps": 30},
        {"name": "stable", "reserve_in": 500_000.0, "reserve_out": 500_000.0, "fee_bps": 5},
    ]
    router = SmartRouter()
    quote = router.optimal_split(50_000.0, venues)

    best_single = max(
        constant_product_out(50_000.0, v["reserve_in"], v["reserve_out"], int(v["fee_bps"]))
        for v in venues
    )
    assert quote.total_out >= best_single - 1e-6
    assert quote.total_in == pytest.approx(50_000.0)


def test_smart_router_allocates_across_both_venues():
    venues = [
        {"name": "a", "reserve_in": 1_000_000.0, "reserve_out": 1_000_000.0, "fee_bps": 30},
        {"name": "b", "reserve_in": 1_000_000.0, "reserve_out": 1_000_000.0, "fee_bps": 30},
    ]
    quote = SmartRouter().optimal_split(100_000.0, venues)

    # Identical venues should be split roughly evenly.
    assert len(quote.allocations) == 2
    for leg in quote.allocations:
        assert leg.amount_in == pytest.approx(50_000.0, rel=0.05)


def test_smart_router_single_venue():
    venues = [{"name": "only", "reserve_in": 100.0, "reserve_out": 100.0, "fee_bps": 30}]
    quote = SmartRouter().optimal_split(10.0, venues)
    assert len(quote.allocations) == 1
    assert quote.total_in == pytest.approx(10.0)


def test_arbitrage_opportunity_detection():
    result = SmartRouter.arbitrage_opportunity(100.0, 105.0)
    assert result["gross_edge"] == pytest.approx(0.05)
    assert result["buy_at"] == "a"
    assert result["sell_at"] == "b"


def test_market_maker_quotes_around_the_mid():
    maker = MarketMaker(risk_aversion=0.1, order_intensity=1.5)
    quote = maker.quote(100.0, inventory=0.0, volatility=0.02)

    assert quote.bid < quote.ask
    assert quote.bid < 100.0 < quote.ask
    assert quote.spread > 0


def test_market_maker_skews_away_from_a_long_inventory():
    maker = MarketMaker(risk_aversion=0.1, order_intensity=1.5)
    flat = maker.quote(100.0, 0.0, 0.02)
    long = maker.quote(100.0, 0.9, 0.02)

    # Holding a long position, the maker lowers the reservation price to
    # encourage selling.
    assert long.reservation_price < flat.reservation_price
    assert long.inventory_skew > 0


def test_market_maker_widens_the_spread_with_volatility():
    maker = MarketMaker()
    calm = maker.quote(100.0, 0.0, 0.01)
    wild = maker.quote(100.0, 0.0, 0.10)
    assert wild.spread > calm.spread


# --------------------------------------------------------------------------
# forecasting
# --------------------------------------------------------------------------


def test_holt_linear_tracks_a_linear_trend():
    series = [100.0 + 2.0 * i for i in range(40)]
    model = HoltLinear(alpha=0.5, beta=0.3).fit(series)
    forecast = model.forecast(5)

    # The series rises by 2 per step, so the next value should be near 180.
    assert forecast.values[0] == pytest.approx(180.0, rel=0.05)
    assert forecast.values[-1] > forecast.values[0]


def test_holt_damped_trend_grows_more_slowly_than_a_linear_trend():
    series = [100.0 + 2.0 * i for i in range(40)]
    linear = HoltLinear(alpha=0.5, beta=0.3, damped=False).fit(series).forecast(10)
    damped = HoltLinear(alpha=0.5, beta=0.3, damped=True, phi=0.9).fit(series).forecast(10)

    assert damped.values[-1] < linear.values[-1]


def test_holt_forecast_interval_brackets_the_point_forecast():
    series = [50.0 + (i % 4) for i in range(30)]
    forecast = HoltLinear().fit(series).forecast(5, confidence=0.95)

    for value, lower, upper in zip(forecast.values, forecast.lower, forecast.upper):
        assert lower <= value <= upper


def test_arimax_fits_and_forecasts_a_trend():
    # A linear series with a little deterministic noise.
    series = [100.0 + 1.5 * i + (i % 3) * 0.1 for i in range(60)]
    model = ARIMAX(ar_order=2, difference_order=1).fit(series)
    forecast = model.forecast(5)

    assert model.r_squared > 0.5
    assert len(forecast.values) == 5
    # The series trends up, so the forecast should continue upward.
    assert forecast.values[-1] > series[-1]


def test_arimax_recovers_an_autoregressive_process():
    # x_t = 0.7 * x_{t-1} + noise, generated deterministically.
    series = [1.0]
    for i in range(80):
        noise = 0.1 * math.sin(i)
        series.append(0.7 * series[-1] + noise)

    model = ARIMAX(ar_order=1, difference_order=0).fit(series)
    # The first AR coefficient should be close to 0.7.
    assert model.coefficients[0] == pytest.approx(0.7, abs=0.15)


def test_forecast_confidence_interval_narrows_with_confidence_level():
    series = [100.0 + math.sin(i / 3) for i in range(60)]
    model = HoltLinear().fit(series)
    narrow = model.forecast(3, confidence=0.80)
    wide = model.forecast(3, confidence=0.99)

    narrow_width = narrow.upper[0] - narrow.lower[0]
    wide_width = wide.upper[0] - wide.lower[0]
    assert wide_width > narrow_width


def test_error_metrics_match_hand_computation():
    actual = [1.0, 2.0, 3.0]
    predicted = [1.0, 2.0, 5.0]

    assert mean_absolute_error(actual, predicted) == pytest.approx(2.0 / 3.0)
    assert root_mean_squared_error(actual, predicted) == pytest.approx(math.sqrt(4.0 / 3.0))
    assert mean_absolute_percentage_error(actual, predicted) > 0


def test_directional_accuracy_of_a_perfect_forecast_is_one():
    actual = [1.0, 2.0, 3.0, 4.0]
    assert directional_accuracy(actual, actual) == pytest.approx(1.0)


def test_directional_accuracy_of_an_inverted_forecast_is_zero():
    actual = [1.0, 2.0, 3.0, 4.0]
    inverted = [4.0, 3.0, 2.0, 1.0]
    assert directional_accuracy(actual, inverted) == pytest.approx(0.0)


# --------------------------------------------------------------------------
# error handling
# --------------------------------------------------------------------------


def test_mathx_rejects_an_empty_series():
    with pytest.raises(ValueError):
        mathx.mean([])


def test_constant_product_rejects_a_non_positive_input():
    with pytest.raises(ValueError):
        constant_product_out(0.0, 100.0, 100.0)


def test_black_scholes_rejects_a_non_positive_maturity():
    with pytest.raises(ValueError):
        BlackScholes.call_price(100.0, 100.0, 0.05, 0.2, 0.0)


def test_risk_engine_rejects_an_invalid_confidence():
    with pytest.raises(ValueError):
        RiskEngine().historical_var([0.01, -0.01], confidence=1.5)


def test_stable_swap_rejects_mismatched_indices():
    with pytest.raises(ValueError):
        StableSwap().out_amount(100.0, 0, 0, [1000.0, 1000.0])


# --- Regression tests for defects found in review -------------------------
#
# Each of these pins a property that the implementation previously violated, so
# the specific defect cannot return unnoticed.


def test_skewness_and_kurtosis_use_one_consistent_scale():
    """Skewness and kurtosis must not mix population moments with a sample scale.

    Previously the third and fourth central moments used a 1/n divisor while
    the scale came from the (n-1) sample standard deviation, which scaled the
    skewness by ((n-1)/n)**1.5 and the kurtosis by an unrelated factor.
    """
    data = [-0.02, 0.01, 0.03, -0.05, 0.02, 0.07, -0.01, 0.04, -0.03, 0.06, 0.0, 0.05]
    count = len(data)
    mu = sum(data) / count
    m2 = sum((value - mu) ** 2 for value in data) / count
    m3 = sum((value - mu) ** 3 for value in data) / count
    m4 = sum((value - mu) ** 4 for value in data) / count

    engine = RiskEngine()
    assert engine.skewness(data) == pytest.approx(m3 / m2**1.5, rel=1e-12)
    assert engine.kurtosis(data) == pytest.approx(m4 / m2**2 - 3.0, rel=1e-12)


def test_benford_leading_digit_is_exact_for_powers_of_ten():
    """A power of ten must report digit 1, not the digit below it.

    Floating point makes `10.0**k` slightly below the exact value, so dividing
    by it and truncating previously produced 9 (or 0) instead of 1.
    """
    detector = BenfordDetector()
    for exponent in range(0, 12):
        assert detector.leading_digit(10.0**exponent) == 1
        assert detector.leading_digit(2.0 * 10.0**exponent) == 2

    distribution = detector.observed_distribution([1000.0, 100.0, 10.0, 10000.0])
    assert distribution[0] == pytest.approx(1.0)
    assert sum(distribution[1:]) == pytest.approx(0.0)


def test_benford_chi_square_ignores_values_without_a_leading_digit():
    """Zeros and non-finite values must not be counted as observations."""
    detector = BenfordDetector()
    assert detector.chi_square([0.0, 0.0]) == pytest.approx(0.0)
    assert detector.chi_square([]) == pytest.approx(0.0)
    # Adding values with no digit must not change the statistic.
    assert detector.chi_square([1.0, 2.0, 0.0]) == pytest.approx(
        detector.chi_square([1.0, 2.0])
    )


def test_smart_router_is_optimal_across_more_than_two_venues():
    """The split must maximise total output, not just search the first venue.

    Previously a three-venue split routed most of the order to the smallest
    pool and lost about 9% of the achievable output.
    """
    venues = [
        {"name": "A", "reserve_in": 1_000_000.0, "reserve_out": 1_000_000.0, "fee_bps": 30},
        {"name": "B", "reserve_in": 5_000_000.0, "reserve_out": 5_000_000.0, "fee_bps": 30},
        {"name": "C", "reserve_in": 100_000.0, "reserve_out": 100_000.0, "fee_bps": 30},
    ]
    amount = 200_000.0
    quote = SmartRouter().optimal_split(amount, venues)

    best = 0.0
    steps = 200
    for first in range(steps + 1):
        for second in range(steps + 1 - first):
            split = [
                amount * first / steps,
                amount * second / steps,
                amount * (steps - first - second) / steps,
            ]
            total = sum(
                constant_product_out(leg, venue["reserve_in"], venue["reserve_out"], 30)
                for leg, venue in zip(split, venues)
                if leg > 0
            )
            best = max(best, total)

    assert quote.total_out >= best - 1e-6
    assert quote.total_in == pytest.approx(amount)


def test_smart_router_preserves_the_input_amount():
    """No allocation scheme may consume or create input."""
    venues = [
        {"name": "x", "reserve_in": 1_000_000.0, "reserve_out": 1_000_000.0, "fee_bps": 30},
        {"name": "y", "reserve_in": 500_000.0, "reserve_out": 500_000.0, "fee_bps": 5},
        {"name": "z", "reserve_in": 250_000.0, "reserve_out": 250_000.0, "fee_bps": 100},
    ]
    amount = 75_000.0
    quote = SmartRouter().optimal_split(amount, venues)
    assert sum(leg.amount_in for leg in quote.allocations) == pytest.approx(amount)


def test_greeks_reject_zero_volatility_instead_of_dividing_by_zero():
    """Gamma and vega divide by volatility, so zero must be rejected."""
    with pytest.raises(ValueError):
        BlackScholes.greeks(100.0, 100.0, 0.05, 0.0, 1.0, "call")
    # The deterministic price is still available at zero volatility.
    value = BlackScholes.call_price(100.0, 100.0, 0.05, 0.0, 1.0)
    assert value == pytest.approx(max(0.0, 100.0 - 100.0 * math.exp(-0.05)), abs=1e-9)
