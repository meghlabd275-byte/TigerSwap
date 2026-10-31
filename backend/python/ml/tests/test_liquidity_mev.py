"""Tests for the liquidity and MEV models.

The impermanent-loss tests pin the closed-form values published for a
constant-product pool; the concentrated-liquidity tests check the piecewise
behaviour at and outside the band edges; the MEV tests build real event
sequences and assert the detector finds exactly the attacks present.
"""

import math

import pytest

from tigerswap_ml import mathx
from tigerswap_ml.liquidity import (
    ConcentratedLiquidityModel,
    ImpermanentLossForecaster,
    OptimalRangeSelector,
    full_range_loss,
)
from tigerswap_ml.mev import (
    LiquidityEvent,
    SandwichRiskModel,
    SwapEvent,
    ToxicityScorer,
    detect_jit_liquidity,
    detect_sandwiches,
)


# ------------------------------------------------------- impermanent loss


def test_full_range_loss_matches_published_values():
    # A 2x move loses 1 - 2*sqrt(2)/3.
    assert full_range_loss(2.0) == pytest.approx(1.0 - 2.0 * math.sqrt(2.0) / 3.0)
    # A 4x move loses exactly 20%.
    assert full_range_loss(4.0) == pytest.approx(0.2)
    # The function is symmetric in the reciprocal ratio.
    assert full_range_loss(0.25) == pytest.approx(0.2)
    # No move, no loss.
    assert full_range_loss(1.0) == pytest.approx(0.0)


def test_full_range_loss_rejects_non_positive_ratio():
    with pytest.raises(ValueError):
        full_range_loss(0.0)
    with pytest.raises(ValueError):
        full_range_loss(-1.0)


def test_impermanent_loss_forecast_is_monotone_in_volatility():
    forecaster = ImpermanentLossForecaster()
    low = forecaster.forecast(0.2, 1.0)
    high = forecaster.forecast(1.5, 1.0)
    assert high.expected_loss > low.expected_loss
    assert high.loss_95 > low.loss_95


def test_impermanent_loss_forecast_is_monotone_in_horizon():
    forecaster = ImpermanentLossForecaster()
    short = forecaster.forecast(0.8, 0.25)
    long = forecaster.forecast(0.8, 4.0)
    assert long.expected_loss > short.expected_loss


def test_zero_volatility_gives_no_loss():
    forecaster = ImpermanentLossForecaster()
    result = forecaster.forecast(0.0, 1.0)
    assert result.expected_loss == pytest.approx(0.0, abs=1e-12)
    assert result.loss_95 == pytest.approx(0.0, abs=1e-12)


def test_loss_quantiles_are_ordered():
    forecaster = ImpermanentLossForecaster()
    result = forecaster.forecast(0.9, 1.0)
    assert 0.0 <= result.median_loss <= result.loss_95 <= result.loss_99
    assert 0.0 <= result.probability_of_loss <= 1.0


def test_forecaster_rejects_bad_inputs():
    forecaster = ImpermanentLossForecaster()
    with pytest.raises(ValueError):
        forecaster.forecast(-0.1, 1.0)
    with pytest.raises(ValueError):
        forecaster.forecast(0.5, 0.0)
    with pytest.raises(ValueError):
        ImpermanentLossForecaster(quadrature_points=2)


# ------------------------------------------------------- concentrated liquidity


def test_concentrated_position_is_all_base_below_the_band():
    base, quote = ConcentratedLiquidityModel.amounts_at_price(1.0, 100.0, 400.0, 50.0)
    assert quote == 0.0
    assert base == pytest.approx(1.0 / math.sqrt(100.0) - 1.0 / math.sqrt(400.0))


def test_concentrated_position_is_all_quote_above_the_band():
    base, quote = ConcentratedLiquidityModel.amounts_at_price(1.0, 100.0, 400.0, 1000.0)
    assert base == 0.0
    assert quote == pytest.approx(math.sqrt(400.0) - math.sqrt(100.0))


def test_concentrated_position_splits_inside_the_band():
    base, quote = ConcentratedLiquidityModel.amounts_at_price(1.0, 100.0, 400.0, 225.0)
    assert base == pytest.approx(1.0 / 15.0 - 1.0 / 20.0)
    assert quote == pytest.approx(15.0 - 10.0)
    # The position's value is base * price + quote.
    value = ConcentratedLiquidityModel.value(1.0, 100.0, 400.0, 225.0)
    assert value == pytest.approx(base * 225.0 + quote)


def test_concentrated_loss_is_zero_at_entry():
    loss = ConcentratedLiquidityModel.loss(1.0, 100.0, 400.0, 225.0, 225.0)
    assert loss == pytest.approx(0.0)


def test_concentrated_loss_exceeds_full_range_loss():
    # A band around 225 from 100 to 400; a move to 500 leaves the band.
    liquidity = 1.0
    concentrated = ConcentratedLiquidityModel.loss(liquidity, 100.0, 400.0, 225.0, 500.0)
    full_range = full_range_loss(500.0 / 225.0)
    assert concentrated > full_range


def test_concentrated_loss_function_accepts_a_ratio():
    loss_fn = ConcentratedLiquidityModel.loss_function(100.0, 400.0, 225.0)
    assert loss_fn(1.0) == pytest.approx(0.0)
    assert loss_fn(500.0 / 225.0) == pytest.approx(
        ConcentratedLiquidityModel.loss(1.0, 100.0, 400.0, 225.0, 500.0)
    )


def test_concentrated_band_validation():
    with pytest.raises(ValueError):
        ConcentratedLiquidityModel.amounts_at_price(1.0, 400.0, 100.0, 200.0)
    with pytest.raises(ValueError):
        ConcentratedLiquidityModel.amounts_at_price(1.0, 0.0, 100.0, 50.0)


def test_forecaster_accepts_a_concentrated_loss_function():
    loss_fn = ConcentratedLiquidityModel.loss_function(100.0, 400.0, 225.0)
    forecaster = ImpermanentLossForecaster()
    result = forecaster.forecast(0.8, 1.0, loss_function=loss_fn)
    assert result.expected_loss >= 0.0
    # A narrow band must lose more than the full-range default.
    default = forecaster.forecast(0.8, 1.0)
    assert result.expected_loss > default.expected_loss


# ------------------------------------------------------- range selection


def test_range_selector_prefers_the_narrowest_surviving_band():
    selector = OptimalRangeSelector(volatility=0.5, horizon_years=1.0)
    # Half-width 0.5 gives an expected exit time of 1 year, so it survives;
    # anything narrower does not.
    assert selector.select(100.0, [0.1, 0.25, 0.5, 1.0]) == pytest.approx(0.5)


def test_range_selector_returns_the_widest_when_none_survive():
    selector = OptimalRangeSelector(volatility=2.0, horizon_years=10.0)
    assert selector.select(100.0, [0.01, 0.02]) == pytest.approx(0.02)


def test_range_selector_handles_zero_volatility():
    selector = OptimalRangeSelector(volatility=0.0, horizon_years=1.0)
    assert selector.expected_time_in_range(0.1) == math.inf
    assert selector.select(100.0, [0.05, 0.1]) == pytest.approx(0.05)


def test_range_selector_validates_inputs():
    with pytest.raises(ValueError):
        OptimalRangeSelector(-0.1, 1.0)
    with pytest.raises(ValueError):
        OptimalRangeSelector(0.5, 0.0)
    selector = OptimalRangeSelector(0.5, 1.0)
    with pytest.raises(ValueError):
        selector.select(100.0, [])
    with pytest.raises(ValueError):
        selector.expected_time_in_range(0.0)


# ------------------------------------------------------- MEV: sandwiches


def swap(tx, sender, pool, is_buy, amount_in, amount_out, price, position):
    return SwapEvent(
        tx_hash=tx,
        sender=sender,
        pool=pool,
        is_buy=is_buy,
        amount_in=amount_in,
        amount_out=amount_out,
        price=price,
        position=position,
    )


def test_detects_a_classic_sandwich():
    events = [
        # A reference trade establishes the pre-attack price.
        swap("0xinit", "trader", "ETH/RUSD", True, 10.0, 10.0, 2000.0, 0),
        # The attacker buys, pushing the price up.
        swap("0xfront", "attacker", "ETH/RUSD", True, 100.0, 100.0, 2050.0, 1),
        # The victim buys at the worse price.
        swap("0xvictim", "victim", "ETH/RUSD", True, 50.0, 50.0, 2080.0, 2),
        # The attacker sells back into the victim's own price impact.
        swap("0xback", "attacker", "ETH/RUSD", False, 100.0, 106.0, 2060.0, 3),
    ]
    sandwiches = detect_sandwiches(events)
    assert len(sandwiches) == 1
    found = sandwiches[0]
    assert found.victim_tx == "0xvictim"
    assert found.frontrun_tx == "0xfront"
    assert found.backrun_tx == "0xback"
    assert found.attacker == "attacker"
    assert found.victim_loss > 0.0
    assert found.victim_loss_bps > 0.0


def test_no_sandwich_without_a_backrun():
    events = [
        swap("0xinit", "trader", "ETH/RUSD", True, 10.0, 10.0, 2000.0, 0),
        swap("0xfront", "attacker", "ETH/RUSD", True, 100.0, 100.0, 2050.0, 1),
        swap("0xvictim", "victim", "ETH/RUSD", True, 50.0, 50.0, 2080.0, 2),
    ]
    assert detect_sandwiches(events) == []


def test_no_sandwich_across_different_pools():
    events = [
        swap("0xfront", "attacker", "ETH/RUSD", True, 100.0, 100.0, 2050.0, 0),
        swap("0xvictim", "victim", "BTC/RUSD", True, 50.0, 50.0, 2080.0, 1),
        swap("0xback", "attacker", "ETH/RUSD", False, 100.0, 106.0, 2060.0, 2),
    ]
    assert detect_sandwiches(events) == []


def test_no_sandwich_when_the_attacker_does_not_profit():
    events = [
        swap("0xinit", "trader", "ETH/RUSD", True, 10.0, 10.0, 2000.0, 0),
        swap("0xfront", "attacker", "ETH/RUSD", True, 100.0, 100.0, 2050.0, 1),
        swap("0xvictim", "victim", "ETH/RUSD", True, 50.0, 50.0, 2080.0, 2),
        # The attacker sells for less than they paid.
        swap("0xback", "attacker", "ETH/RUSD", False, 100.0, 90.0, 2060.0, 3),
    ]
    sandwiches = detect_sandwiches(events)
    assert len(sandwiches) == 1
    assert sandwiches[0].attacker_profit == 0.0


def test_no_sandwich_when_the_frontrun_barely_moves_the_price():
    events = [
        swap("0xinit", "trader", "ETH/RUSD", True, 10.0, 10.0, 2000.0, 0),
        # A 1 basis-point move is below the default 5 bps floor.
        swap("0xfront", "attacker", "ETH/RUSD", True, 100.0, 100.0, 2000.2, 1),
        swap("0xvictim", "victim", "ETH/RUSD", True, 50.0, 50.0, 2000.4, 2),
        swap("0xback", "attacker", "ETH/RUSD", False, 100.0, 106.0, 2001.0, 3),
    ]
    assert detect_sandwiches(events) == []


def test_sandwich_requires_proximity():
    events = [
        swap("0xinit", "trader", "ETH/RUSD", True, 10.0, 10.0, 2000.0, 0),
        swap("0xfront", "attacker", "ETH/RUSD", True, 100.0, 100.0, 2050.0, 1),
        swap("0xvictim", "victim", "ETH/RUSD", True, 50.0, 50.0, 2080.0, 20),
        swap("0xback", "attacker", "ETH/RUSD", False, 100.0, 106.0, 2060.0, 21),
    ]
    # Default proximity is 5, so the front-run is too far away.
    assert detect_sandwiches(events) == []
    # A wider window finds it.
    assert len(detect_sandwiches(events, proximity=25)) == 1


def test_router_swaps_are_ignored():
    events = [
        swap("0xinit", "trader", "ETH/RUSD", True, 10.0, 10.0, 2000.0, 0),
        swap("0xfront", "attacker", "ETH/RUSD", True, 100.0, 100.0, 2050.0, 1),
        swap("0xvictim", "victim", "ETH/RUSD", True, 50.0, 50.0, 2080.0, 2),
        swap("0xback", "attacker", "ETH/RUSD", False, 100.0, 106.0, 2060.0, 3),
    ]
    for event in events:
        if event.tx_hash == "0xvictim":
            event.is_router = True
    assert detect_sandwiches(events) == []


def test_sandwich_detector_validates_parameters():
    with pytest.raises(ValueError):
        detect_sandwiches([], proximity=0)
    with pytest.raises(ValueError):
        detect_sandwiches([], min_impact=-1.0)


def test_two_independent_sandwiches_are_both_found():
    events = [
        swap("0xinit", "trader", "ETH/RUSD", True, 10.0, 10.0, 2000.0, 0),
        swap("0xa-front", "attackerA", "ETH/RUSD", True, 100.0, 100.0, 2050.0, 1),
        swap("0xv1", "victim1", "ETH/RUSD", True, 50.0, 50.0, 2080.0, 2),
        swap("0xa-back", "attackerA", "ETH/RUSD", False, 100.0, 106.0, 2060.0, 3),
        swap("0xb-front", "attackerB", "ETH/RUSD", True, 100.0, 100.0, 2070.0, 4),
        swap("0xv2", "victim2", "ETH/RUSD", True, 50.0, 50.0, 2100.0, 5),
        swap("0xb-back", "attackerB", "ETH/RUSD", False, 100.0, 106.0, 2080.0, 6),
    ]
    sandwiches = detect_sandwiches(events)
    assert len(sandwiches) == 2
    victims = {sandwich.victim_tx for sandwich in sandwiches}
    assert victims == {"0xv1", "0xv2"}


# ------------------------------------------------------- MEV: JIT liquidity


def test_detects_jit_liquidity():
    adds = [
        LiquidityEvent("0xadd", "provider", "ETH/RUSD", True, 500.0, 1),
    ]
    swaps = [swap("0xswap", "victim", "ETH/RUSD", True, 1000.0, 1000.0, 2050.0, 2)]
    removes = [
        LiquidityEvent("0xremove", "provider", "ETH/RUSD", False, 500.0, 3),
    ]
    found = detect_jit_liquidity(adds, swaps)
    found = detect_jit_liquidity(adds + removes, swaps)
    assert len(found) == 1
    assert found[0].provider == "provider"
    assert found[0].swap_tx == "0xswap"


def test_jit_requires_the_same_provider():
    events = [
        LiquidityEvent("0xadd", "alice", "ETH/RUSD", True, 500.0, 1),
        LiquidityEvent("0xremove", "bob", "ETH/RUSD", False, 500.0, 3),
    ]
    swaps = [swap("0xswap", "victim", "ETH/RUSD", True, 1000.0, 1000.0, 2050.0, 2)]
    assert detect_jit_liquidity(events, swaps) == []


def test_jit_requires_a_comparable_remove():
    events = [
        LiquidityEvent("0xadd", "provider", "ETH/RUSD", True, 500.0, 1),
        # A remove far smaller than the add is an ordinary withdrawal.
        LiquidityEvent("0xremove", "provider", "ETH/RUSD", False, 10.0, 3),
    ]
    swaps = [swap("0xswap", "victim", "ETH/RUSD", True, 1000.0, 1000.0, 2050.0, 2)]
    assert detect_jit_liquidity(events, swaps) == []


def test_jit_requires_the_swap_between_add_and_remove():
    events = [
        LiquidityEvent("0xadd", "provider", "ETH/RUSD", True, 500.0, 1),
        LiquidityEvent("0xremove", "provider", "ETH/RUSD", False, 500.0, 2),
    ]
    # The swap happens after both, so there is nothing to sandwich.
    swaps = [swap("0xswap", "victim", "ETH/RUSD", True, 1000.0, 1000.0, 2050.0, 5)]
    assert detect_jit_liquidity(events, swaps) == []


# ------------------------------------------------------- MEV: toxicity


def test_toxicity_scorer_identifies_offenders():
    scorer = ToxicityScorer()
    scorer.observe_offenders(["attacker"])
    assert scorer.is_offender("attacker")
    assert not scorer.is_offender("honest")


def test_toxicity_score_reflects_offender_volume():
    scorer = ToxicityScorer()
    scorer.observe_offenders(["attacker"])
    events = [
        swap("0x1", "attacker", "ETH/RUSD", True, 75.0, 75.0, 2000.0, 0),
        swap("0x2", "honest", "ETH/RUSD", True, 25.0, 25.0, 2000.0, 1),
    ]
    score = scorer.score("ETH/RUSD", events)
    assert score.toxicity == pytest.approx(0.75)
    assert score.should_route_private
    assert score.offenders == ["attacker"]


def test_clean_pool_is_not_routed_privately():
    scorer = ToxicityScorer()
    scorer.observe_offenders(["attacker"])
    events = [
        swap("0x1", "honest", "BTC/RUSD", True, 100.0, 100.0, 60000.0, 0),
    ]
    score = scorer.score("BTC/RUSD", events)
    assert score.toxicity == 0.0
    assert not score.should_route_private


def test_toxicity_scorer_decays_old_offences():
    scorer = ToxicityScorer(decay=0.5)
    scorer.observe_offenders(["attacker"])
    assert scorer.is_offender("attacker")
    scorer.observe_offenders([])
    # 1.0 * 0.5 is not above the 0.5 threshold.
    assert not scorer.is_offender("attacker")


def test_score_all_covers_every_pool():
    scorer = ToxicityScorer()
    events = [
        swap("0x1", "a", "ETH/RUSD", True, 1.0, 1.0, 2000.0, 0),
        swap("0x2", "b", "BTC/RUSD", True, 1.0, 1.0, 60000.0, 1),
    ]
    scores = scorer.score_all(events)
    assert {score.pool for score in scores} == {"BTC/RUSD", "ETH/RUSD"}


def test_toxicity_scorer_validates_parameters():
    with pytest.raises(ValueError):
        ToxicityScorer(decay=0.0)
    with pytest.raises(ValueError):
        ToxicityScorer(decay=1.5)
    with pytest.raises(ValueError):
        ToxicityScorer(private_routing_threshold=2.0)


# ------------------------------------------------------- MEV: risk model


def test_risk_model_is_monotone_in_size():
    model = SandwichRiskModel()
    small = model.probability(swap_size=10.0, pool_depth=10_000.0, toxicity=0.1)
    large = model.probability(swap_size=9_000.0, pool_depth=10_000.0, toxicity=0.1)
    assert 0.0 <= small < large <= 1.0


def test_risk_model_is_monotone_in_toxicity():
    model = SandwichRiskModel()
    clean = model.probability(swap_size=100.0, pool_depth=10_000.0, toxicity=0.0)
    dirty = model.probability(swap_size=100.0, pool_depth=10_000.0, toxicity=1.0)
    assert clean < dirty


def test_risk_model_falls_with_gas_premium():
    model = SandwichRiskModel()
    cheap = model.probability(1000.0, 10_000.0, 0.5, gas_premium=0.0)
    costly = model.probability(1000.0, 10_000.0, 0.5, gas_premium=5.0)
    assert costly < cheap


def test_risk_model_recommends_private_routing_above_threshold():
    model = SandwichRiskModel()
    probability = model.probability(9_000.0, 10_000.0, 1.0)
    assert probability > 0.5
    assert model.recommend_private(probability)


def test_risk_model_validates_inputs():
    model = SandwichRiskModel()
    with pytest.raises(ValueError):
        model.probability(-1.0, 100.0, 0.1)
    with pytest.raises(ValueError):
        model.probability(1.0, 0.0, 0.1)
    with pytest.raises(ValueError):
        model.probability(1.0, 100.0, 1.5)
    with pytest.raises(ValueError):
        model.recommend_private(1.5)
