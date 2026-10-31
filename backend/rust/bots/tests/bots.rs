//! Tests for the self-built bots.
//!
//! Every test drives the real strategy code with real snapshots; nothing is
//! mocked, because the bots have no I/O to mock.

use tigerswap_bots::*;

fn fx(text: &str) -> Fixed {
    Fixed::parse(text).expect("valid decimal")
}

fn level(price: &str, quantity: &str) -> Level {
    Level::new(fx(price), fx(quantity))
}

fn book(bids: Vec<Level>, asks: Vec<Level>) -> MarketSnapshot {
    MarketSnapshot::new("BTC/RUSD", bids, asks, fx("60000"), 1_700_000_000_000)
        .expect("valid snapshot")
}

fn maker_config() -> MarketMakerConfig {
    MarketMakerConfig {
        spread_bps: 10,
        target_depth: fx("1.5"),
        rebalance_threshold_bps: 10,
        min_spread_bps: 1,
        max_spread_bps: 500,
        min_book_depth: Fixed::ZERO,
    }
}

// ---------------------------------------------------------------- fixed point

#[test]
fn fixed_parse_round_trips() {
    assert_eq!(fx("1").raw(), 10i128.pow(18));
    assert_eq!(fx("0.5").raw(), 5 * 10i128.pow(17));
    assert_eq!(fx("60000").raw(), 60_000 * 10i128.pow(18));
    assert_eq!(fx("-2.25").to_string(), "-2.25");
    assert_eq!(fx("0.000000000000000001").to_string(), "0.000000000000000001");
}

#[test]
fn fixed_parse_rejects_malformed_input() {
    for bad in ["", "abc", "1.2.3", "1,000", "1e18", "--1", "0.0000000000000000001"] {
        assert!(Fixed::parse(bad).is_err(), "{bad} should be rejected");
    }
}

#[test]
fn fixed_arithmetic_is_exact() {
    let two = fx("2");
    let three = fx("3");
    assert_eq!(two.mul(three).unwrap(), fx("6"));
    assert_eq!(fx("10").div(two).unwrap(), fx("5"));
    assert_eq!(fx("1").add(fx("0.5")).unwrap(), fx("1.5"));
    assert_eq!(fx("1").sub(fx("0.25")).unwrap(), fx("0.75"));
}

#[test]
fn fixed_division_by_zero_is_an_error() {
    assert_eq!(fx("1").div(Fixed::ZERO), Err(BotError::DivisionByZero));
    assert_eq!(Fixed::ratio(1, 0), Err(BotError::DivisionByZero));
}

#[test]
fn fixed_apply_bps_moves_price() {
    let price = fx("60000");
    assert_eq!(price.apply_bps(10).unwrap(), fx("60060"));
    assert_eq!(price.apply_bps(-10).unwrap(), fx("59940"));
    // A 100% reduction is the floor; below that is rejected.
    assert!(price.apply_bps(-10_000).unwrap().is_zero());
    assert!(price.apply_bps(-10_001).is_err());
}

#[test]
fn fixed_overflow_is_reported_not_wrapped() {
    let huge = Fixed::from_raw(i128::MAX);
    assert_eq!(huge.add(huge), Err(BotError::Overflow));
    assert_eq!(huge.mul(huge), Err(BotError::Overflow));
}

// ---------------------------------------------------------------- market view

#[test]
fn snapshot_rejects_a_crossed_book() {
    let result = MarketSnapshot::new(
        "BTC/RUSD",
        vec![level("100", "1")],
        vec![level("99", "1")],
        fx("100"),
        0,
    );
    assert!(result.is_err());
}

#[test]
fn snapshot_rejects_misordered_levels() {
    let bids = vec![level("100", "1"), level("101", "1")];
    let result = MarketSnapshot::new("BTC/RUSD", bids, vec![level("110", "1")], fx("105"), 0);
    assert!(result.is_err());
}

#[test]
fn mid_is_the_average_of_the_touch() {
    let snapshot = book(vec![level("60000", "2")], vec![level("60010", "2")]);
    assert_eq!(snapshot.mid().unwrap(), fx("60005"));
}

#[test]
fn mid_falls_back_to_the_single_side() {
    let snapshot = book(vec![level("60000", "2")], vec![]);
    assert_eq!(snapshot.mid().unwrap(), fx("60000"));
}

#[test]
fn empty_book_has_no_mid() {
    let snapshot = book(vec![], vec![]);
    assert!(snapshot.mid().is_none());
    // With no book the bot falls back to the last traded vwap rather than
    // inventing a price.
    assert_eq!(snapshot.reference_price().unwrap(), fx("60000"));
}

#[test]
fn empty_book_with_no_vwap_has_no_reference_price() {
    let snapshot = MarketSnapshot::new("BTC/RUSD", vec![], vec![], Fixed::ZERO, 0).unwrap();
    assert!(snapshot.reference_price().is_err());
}

#[test]
fn depth_within_sums_only_levels_in_range() {
    let snapshot = book(
        vec![level("60000", "1"), level("59000", "5")],
        vec![level("60010", "1")],
    );
    // 10 bps of 60005 is about 60, so only the touch is inside the band.
    assert_eq!(snapshot.depth_within(Side::Buy, 10).unwrap(), fx("1"));
    // A 200 bps band reaches the second level.
    assert_eq!(snapshot.depth_within(Side::Buy, 200).unwrap(), fx("6"));
}

// ---------------------------------------------------------------- market maker

#[test]
fn market_maker_quotes_both_sides() {
    let mut bot = MarketMaker::new(maker_config()).unwrap();
    let snapshot = book(vec![level("60000", "10")], vec![level("60010", "10")]);
    let orders = bot.quote(&snapshot).unwrap();
    assert_eq!(orders.len(), 2);

    let bid = orders.iter().find(|o| o.side == Side::Buy).unwrap();
    let ask = orders.iter().find(|o| o.side == Side::Sell).unwrap();
    assert!(bid.price < snapshot.mid().unwrap());
    assert!(ask.price > snapshot.mid().unwrap());
    assert_eq!(bid.quantity, ask.quantity);
    assert!(bid.kind == OrderKind::PostOnly);
    assert_eq!(bid.source, "market_maker");
}

#[test]
fn market_maker_quote_does_not_cross_the_book() {
    let mut bot = MarketMaker::new(maker_config()).unwrap();
    let snapshot = book(vec![level("60000", "1")], vec![level("60010", "1")]);
    let orders = bot.quote(&snapshot).unwrap();
    let bid = orders.iter().find(|o| o.side == Side::Buy).unwrap().price;
    let ask = orders.iter().find(|o| o.side == Side::Sell).unwrap().price;
    assert!(bid < snapshot.best_ask().unwrap());
    assert!(ask > snapshot.best_bid().unwrap());
}

#[test]
fn market_maker_skips_requote_below_threshold() {
    let mut bot = MarketMaker::new(maker_config()).unwrap();
    let snapshot = book(vec![level("60000", "10")], vec![level("60010", "10")]);
    assert_eq!(bot.quote(&snapshot).unwrap().len(), 2);
    // Same book again: the mid has not moved, so nothing new is quoted.
    assert!(bot.quote(&snapshot).unwrap().is_empty());
}

#[test]
fn market_maker_requotes_after_a_significant_move() {
    let mut bot = MarketMaker::new(maker_config()).unwrap();
    let first = book(vec![level("60000", "10")], vec![level("60010", "10")]);
    assert_eq!(bot.quote(&first).unwrap().len(), 2);

    // A 1% move clears the 10 bps threshold.
    let moved = book(vec![level("60600", "10")], vec![level("60610", "10")]);
    assert_eq!(bot.quote(&moved).unwrap().len(), 2);
}

#[test]
fn market_maker_refuses_to_quote_an_empty_book() {
    let mut bot = MarketMaker::new(maker_config()).unwrap();
    let snapshot = book(vec![], vec![]);
    assert!(bot.quote(&snapshot).is_err());
}

#[test]
fn market_maker_skips_a_thin_book_when_a_floor_is_set() {
    let mut config = maker_config();
    config.min_book_depth = fx("100");
    let mut bot = MarketMaker::new(config).unwrap();
    let snapshot = book(vec![level("60000", "1")], vec![level("60010", "1")]);
    assert!(bot.quote(&snapshot).unwrap().is_empty());
}

#[test]
fn market_maker_caps_size_to_a_fraction_of_depth() {
    let mut config = maker_config();
    config.target_depth = fx("100");
    let mut bot = MarketMaker::new(config).unwrap();
    let snapshot = book(vec![level("60000", "5")], vec![level("60010", "5")]);
    let orders = bot.quote(&snapshot).unwrap();
    // A fifth of 5 is 1, well under the target of 100.
    assert_eq!(orders[0].quantity, fx("1"));
}

#[test]
fn market_maker_rejects_invalid_config() {
    let mut config = maker_config();
    config.spread_bps = 0;
    assert!(MarketMaker::new(config).is_err());

    let mut config = maker_config();
    config.spread_bps = 1_000;
    assert!(MarketMaker::new(config).is_err(), "spread above max must fail");

    let mut config = maker_config();
    config.target_depth = Fixed::ZERO;
    assert!(MarketMaker::new(config).is_err());
}

// ---------------------------------------------------------------- grid

fn grid_config() -> GridConfig {
    GridConfig {
        lower_price: fx("100"),
        upper_price: fx("200"),
        levels: 5,
        order_size: fx("1"),
        stop_out_bps: 100,
    }
}

#[test]
fn grid_levels_are_evenly_spaced() {
    let bot = GridBot::new(grid_config()).unwrap();
    let levels = bot.levels();
    assert_eq!(levels.len(), 5);
    assert_eq!(levels[0], fx("100"));
    assert_eq!(levels[4], fx("200"));
    assert_eq!(levels[1], fx("125"));
}

#[test]
fn grid_buys_below_and_sells_above() {
    let mut bot = GridBot::new(grid_config()).unwrap();
    let snapshot = book(vec![level("149.99", "10")], vec![level("150.01", "10")]);
    let orders = bot.step(&snapshot).unwrap();
    assert_eq!(orders.len(), 2);
    let buy = orders.iter().find(|o| o.side == Side::Buy).unwrap();
    let sell = orders.iter().find(|o| o.side == Side::Sell).unwrap();
    assert!(buy.price < fx("150"));
    assert!(sell.price > fx("150"));
}

#[test]
fn grid_does_not_refill_a_level_in_the_same_sweep() {
    let mut bot = GridBot::new(grid_config()).unwrap();
    let snapshot = book(vec![level("149.99", "10")], vec![level("150.01", "10")]);
    let first = bot.step(&snapshot).unwrap();
    assert_eq!(first.len(), 2);
    // The nearest level on each side is now filled, so the next levels out are
    // used: the buy drops to a lower price and the sell rises to a higher one.
    let second = bot.step(&snapshot).unwrap();
    let second_buy = second.iter().find(|o| o.side == Side::Buy).unwrap();
    let first_buy = first.iter().find(|o| o.side == Side::Buy).unwrap();
    assert!(second_buy.price < first_buy.price);

    let second_sell = second.iter().find(|o| o.side == Side::Sell).unwrap();
    let first_sell = first.iter().find(|o| o.side == Side::Sell).unwrap();
    assert!(second_sell.price > first_sell.price);
}

#[test]
fn grid_reset_allows_refilling() {
    let mut bot = GridBot::new(grid_config()).unwrap();
    let snapshot = book(vec![level("149.99", "10")], vec![level("150.01", "10")]);
    bot.step(&snapshot).unwrap();
    assert!(bot.filled_count() > 0);
    bot.reset_sweep();
    assert_eq!(bot.filled_count(), 0);
}

#[test]
fn grid_stands_down_outside_its_range() {
    let mut bot = GridBot::new(grid_config()).unwrap();
    let snapshot = book(vec![level("500", "10")], vec![level("510", "10")]);
    assert!(!bot.in_range(fx("505")).unwrap());
    assert!(bot.step(&snapshot).unwrap().is_empty());
}

#[test]
fn grid_rejects_invalid_config() {
    let mut config = grid_config();
    config.levels = 1;
    assert!(GridBot::new(config).is_err());

    let mut config = grid_config();
    config.upper_price = config.lower_price;
    assert!(GridBot::new(config).is_err());

    let mut config = grid_config();
    config.order_size = Fixed::ZERO;
    assert!(GridBot::new(config).is_err());
}

// ---------------------------------------------------------------- DCA

fn dca_config() -> DcaConfig {
    DcaConfig {
        amount_per_interval: fx("100"),
        interval_ms: 86_400_000,
        max_price: Fixed::ZERO,
        total_intervals: 3,
    }
}

#[test]
fn dca_buys_the_configured_amount() {
    let mut bot = DcaBot::new(dca_config()).unwrap();
    let snapshot = book(vec![level("99.99", "10")], vec![level("100", "10")]);
    let orders = bot.step(&snapshot).unwrap();
    assert_eq!(orders.len(), 1);
    // ~100 quote at a mid near 100 buys about 1 base unit.
    assert!(orders[0].quantity > fx("0.99") && orders[0].quantity <= fx("1.001"));
    assert_eq!(bot.intervals_done(), 1);
    assert_eq!(bot.spent(), fx("100"));
}

#[test]
fn dca_waits_for_the_interval() {
    let mut bot = DcaBot::new(dca_config()).unwrap();
    let snapshot = book(vec![level("99.99", "10")], vec![level("100", "10")]);
    bot.step(&snapshot).unwrap();

    // The same timestamp is inside the interval.
    assert!(bot.step(&snapshot).unwrap().is_empty());

    let later = MarketSnapshot::new(
        "BTC/RUSD",
        vec![level("99.99", "10")],
        vec![level("100", "10")],
        fx("100"),
        1_700_000_000_000 + 86_400_000,
    )
    .unwrap();
    assert_eq!(bot.step(&later).unwrap().len(), 1);
}

#[test]
fn dca_respects_the_price_ceiling() {
    let mut config = dca_config();
    config.max_price = fx("50");
    let mut bot = DcaBot::new(config).unwrap();
    let snapshot = book(vec![level("99.99", "10")], vec![level("100", "10")]);
    assert!(bot.step(&snapshot).unwrap().is_empty());
}

#[test]
fn dca_stops_after_its_intervals() {
    let mut bot = DcaBot::new(dca_config()).unwrap();
    let mut timestamp = 1_700_000_000_000i64;
    for _ in 0..3 {
        let snapshot = MarketSnapshot::new(
            "BTC/RUSD",
            vec![level("99.99", "10")],
            vec![level("100", "10")],
            fx("100"),
            timestamp,
        )
        .unwrap();
        assert_eq!(bot.step(&snapshot).unwrap().len(), 1);
        timestamp += 86_400_000;
    }
    assert!(bot.is_complete());
    let final_snapshot = MarketSnapshot::new(
        "BTC/RUSD",
        vec![level("99.99", "10")],
        vec![level("100", "10")],
        fx("100"),
        timestamp,
    )
    .unwrap();
    assert!(bot.step(&final_snapshot).unwrap().is_empty());
}

// ---------------------------------------------------------------- TWAP

#[test]
fn twap_splits_into_equal_slices() {
    let mut bot = TwapBot::new(TwapConfig {
        total_quantity: fx("10"),
        slices: 4,
        interval_ms: 1_000,
        side: Side::Sell,
    })
    .unwrap();
    assert_eq!(bot.slice_size(), fx("2.5"));

    let snapshot = book(vec![level("99.99", "10")], vec![level("100", "10")]);
    let orders = bot.step(&snapshot).unwrap();
    assert_eq!(orders.len(), 1);
    assert_eq!(orders[0].quantity, fx("2.5"));
    assert_eq!(orders[0].side, Side::Sell);
}

#[test]
fn twap_remainder_is_exact() {
    // 10 into 3 slices is not exact; the last slice must absorb the remainder.
    let mut bot = TwapBot::new(TwapConfig {
        total_quantity: fx("10"),
        slices: 3,
        interval_ms: 1,
        side: Side::Sell,
    })
    .unwrap();

    let mut total = Fixed::ZERO;
    let mut timestamp = 1_700_000_000_000i64;
    for _ in 0..3 {
        let snapshot = MarketSnapshot::new(
            "BTC/RUSD",
            vec![level("99.99", "10")],
            vec![level("100", "10")],
            fx("100"),
            timestamp,
        )
        .unwrap();
        let orders = bot.step(&snapshot).unwrap();
        total = total.add(orders[0].quantity).unwrap();
        timestamp += 1;
    }
    assert_eq!(total, fx("10"), "slices must sum to the total");
    assert!(bot.is_complete());
}

#[test]
fn twap_rejects_zero_slices() {
    assert!(TwapBot::new(TwapConfig {
        total_quantity: fx("1"),
        slices: 0,
        interval_ms: 1,
        side: Side::Buy,
    })
    .is_err());
}

// ---------------------------------------------------------------- arbitrage

fn venue(name: &str, bid: &str, ask: &str, fee_bps: i64) -> Venue {
    Venue::new(
        name,
        MarketSnapshot::new(
            "BTC/RUSD",
            vec![level(bid, "10")],
            vec![level(ask, "10")],
            fx(ask),
            1_700_000_000_000,
        )
        .unwrap(),
        fee_bps,
    )
    .unwrap()
}

fn arb_config() -> ArbitrageConfig {
    ArbitrageConfig {
        min_edge_bps: 5,
        max_quantity: fx("10"),
        depth_fraction_bps: 2_000,
    }
}

#[test]
fn arbitrage_finds_a_cross_venue_edge() {
    let bot = ArbitrageBot::new(arb_config()).unwrap();
    // Venue A sells at 100, venue B buys at 110.
    let venues = vec![venue("v2", "99", "100", 0), venue("v3", "110", "111", 0)];
    let opportunity = bot.find(&venues).unwrap().expect("an edge exists");
    assert_eq!(opportunity.buy_venue, "v2");
    assert_eq!(opportunity.sell_venue, "v3");
    assert!(opportunity.edge_bps >= 1_000);
    assert!(opportunity.expected_profit.is_positive());
}

#[test]
fn arbitrage_returns_none_without_an_edge() {
    let bot = ArbitrageBot::new(arb_config()).unwrap();
    let venues = vec![venue("v2", "99", "100", 0), venue("v3", "100", "101", 0)];
    assert!(bot.find(&venues).unwrap().is_none());
}

#[test]
fn arbitrage_edge_is_eaten_by_fees() {
    let bot = ArbitrageBot::new(ArbitrageConfig {
        min_edge_bps: 1,
        max_quantity: fx("10"),
        depth_fraction_bps: 2_000,
    })
    .unwrap();
    // Gross edge is 1%, but 60 bps of fee on each side removes it.
    let venues = vec![
        venue("v2", "9900", "10000", 60),
        venue("v3", "10100", "10200", 60),
    ];
    assert!(bot.find(&venues).unwrap().is_none());
}

#[test]
fn arbitrage_needs_two_venues() {
    let bot = ArbitrageBot::new(arb_config()).unwrap();
    assert!(bot.find(&[venue("v2", "99", "100", 0)]).unwrap().is_none());
}

#[test]
fn arbitrage_sizes_within_the_depth_fraction() {
    let bot = ArbitrageBot::new(arb_config()).unwrap();
    let venues = vec![venue("v2", "99", "100", 0), venue("v3", "110", "111", 0)];
    let opportunity = bot.find(&venues).unwrap().unwrap();
    // 20% of 10 units.
    assert_eq!(opportunity.quantity, fx("2"));
}

#[test]
fn arbitrage_emits_two_legs() {
    let bot = ArbitrageBot::new(arb_config()).unwrap();
    let venues = vec![venue("v2", "99", "100", 0), venue("v3", "110", "111", 0)];
    let opportunity = bot.find(&venues).unwrap().unwrap();
    let orders = bot.orders(&opportunity, "BTC/RUSD").unwrap();
    assert_eq!(orders.len(), 2);
    assert_eq!(orders[0].side, Side::Buy);
    assert_eq!(orders[1].side, Side::Sell);
    assert_eq!(orders[0].quantity, orders[1].quantity);
}

// ---------------------------------------------------------------- liquidation

fn position(collateral: &str, debt: &str, threshold: &str) -> BorrowPosition {
    BorrowPosition {
        id: "pos-1".into(),
        collateral_value: fx(collateral),
        debt_value: fx(debt),
        liquidation_threshold: fx(threshold),
        liquidation_bonus_bps: 500,
    }
}

#[test]
fn health_factor_below_one_is_liquidatable() {
    let healthy = position("1000", "500", "0.75");
    assert_eq!(healthy.health_factor().unwrap(), fx("1.5"));
    assert!(!healthy.is_liquidatable().unwrap());

    let unhealthy = position("1000", "800", "0.75");
    assert_eq!(unhealthy.health_factor().unwrap(), fx("0.9375"));
    assert!(unhealthy.is_liquidatable().unwrap());
}

#[test]
fn liquidation_repays_half_and_seizes_with_bonus() {
    let bot = LiquidationBot::new(LiquidationConfig {
        close_factor_bps: 5_000,
        min_bonus_bps: 100,
    })
    .unwrap();

    let liquidation = bot.evaluate(&position("1000", "800", "0.75")).unwrap().unwrap();
    assert_eq!(liquidation.repay_amount, fx("400"));
    assert_eq!(liquidation.bonus_earned, fx("20"));
    assert_eq!(liquidation.collateral_seized, fx("420"));
}

#[test]
fn liquidation_skips_healthy_positions() {
    let bot = LiquidationBot::new(LiquidationConfig {
        close_factor_bps: 5_000,
        min_bonus_bps: 100,
    })
    .unwrap();
    assert!(bot.evaluate(&position("1000", "500", "0.75")).unwrap().is_none());
}

#[test]
fn liquidation_skips_a_thin_bonus() {
    let bot = LiquidationBot::new(LiquidationConfig {
        close_factor_bps: 5_000,
        min_bonus_bps: 1_000,
    })
    .unwrap();
    assert!(bot.evaluate(&position("1000", "800", "0.75")).unwrap().is_none());
}

#[test]
fn liquidation_rejects_a_position_with_no_debt() {
    let debt_free = position("1000", "0", "0.75");
    assert!(debt_free.health_factor().is_err());
}

// ---------------------------------------------------------------- rebalance

fn rebalance_config() -> RebalanceConfig {
    RebalanceConfig {
        drift_threshold_bps: 100,
    }
}

#[test]
fn rebalance_leaves_a_balanced_portfolio_alone() {
    let bot = RebalanceBot::new(rebalance_config()).unwrap();
    let holdings = vec![
        Target { symbol: "BTC".into(), weight_bps: 5_000, value: fx("500") },
        Target { symbol: "ETH".into(), weight_bps: 5_000, value: fx("500") },
    ];
    assert!(bot.plan(&holdings).unwrap().is_empty());
}

#[test]
fn rebalance_sells_the_overweight_holding() {
    let bot = RebalanceBot::new(rebalance_config()).unwrap();
    let holdings = vec![
        Target { symbol: "BTC".into(), weight_bps: 5_000, value: fx("700") },
        Target { symbol: "ETH".into(), weight_bps: 5_000, value: fx("300") },
    ];
    let trades = bot.plan(&holdings).unwrap();
    assert_eq!(trades.len(), 2);

    let btc = trades.iter().find(|t| t.symbol == "BTC").unwrap();
    assert_eq!(btc.side, Side::Sell);
    assert_eq!(btc.amount, fx("200"));

    let eth = trades.iter().find(|t| t.symbol == "ETH").unwrap();
    assert_eq!(eth.side, Side::Buy);
    assert_eq!(eth.amount, fx("200"));
}

#[test]
fn rebalance_requires_weights_to_total_one_hundred_percent() {
    let bot = RebalanceBot::new(rebalance_config()).unwrap();
    let holdings = vec![
        Target { symbol: "BTC".into(), weight_bps: 5_000, value: fx("500") },
        Target { symbol: "ETH".into(), weight_bps: 4_000, value: fx("500") },
    ];
    assert!(bot.plan(&holdings).is_err());
}

#[test]
fn rebalance_rejects_an_empty_portfolio() {
    let bot = RebalanceBot::new(rebalance_config()).unwrap();
    assert!(bot.plan(&[]).is_err());
}

#[test]
fn rebalance_ignores_drift_below_the_threshold() {
    let bot = RebalanceBot::new(RebalanceConfig {
        drift_threshold_bps: 1_000,
    })
    .unwrap();
    // A 5% drift is below a 10% threshold, so nothing is traded.
    let holdings = vec![
        Target { symbol: "BTC".into(), weight_bps: 5_000, value: fx("550") },
        Target { symbol: "ETH".into(), weight_bps: 5_000, value: fx("450") },
    ];
    assert!(bot.plan(&holdings).unwrap().is_empty());
}

// ---------------------------------------------------------------- sniper

fn sniper_config() -> SniperConfig {
    SniperConfig {
        max_price_impact_bps: 100,
        min_liquidity: fx("1000"),
        buy_size: fx("1"),
    }
}

#[test]
fn sniper_buys_a_deep_listing() {
    let bot = SniperBot::new(sniper_config()).unwrap();
    let listing = NewListing {
        symbol: "NEW/RUSD".into(),
        snapshot: book(vec![level("10", "100")], vec![level("10.01", "100")]),
        liquidity: fx("50000"),
    };
    let order = bot.evaluate(&listing).unwrap().expect("listing qualifies");
    assert_eq!(order.side, Side::Buy);
    assert_eq!(order.source, "sniper");
}

#[test]
fn sniper_refuses_a_thin_listing() {
    let bot = SniperBot::new(sniper_config()).unwrap();
    let listing = NewListing {
        symbol: "NEW/RUSD".into(),
        snapshot: book(vec![level("10", "100")], vec![level("10.01", "100")]),
        liquidity: fx("10"),
    };
    assert!(bot.evaluate(&listing).unwrap().is_none());
}

#[test]
fn sniper_refuses_when_impact_is_too_high() {
    let bot = SniperBot::new(sniper_config()).unwrap();
    // Only 0.1 units are offered, so a 1-unit buy cannot fill.
    let listing = NewListing {
        symbol: "NEW/RUSD".into(),
        snapshot: book(vec![level("10", "100")], vec![level("10.01", "0.1")]),
        liquidity: fx("50000"),
    };
    assert!(bot.evaluate(&listing).unwrap().is_none());
}

#[test]
fn price_impact_grows_with_order_size() {
    let listing = NewListing {
        symbol: "NEW/RUSD".into(),
        snapshot: book(
            vec![level("10", "100")],
            vec![level("10.01", "1"), level("11", "100")],
        ),
        liquidity: fx("50000"),
    };

    let small = SniperBot::new(SniperConfig {
        buy_size: fx("0.5"),
        ..sniper_config()
    })
    .unwrap();
    let large = SniperBot::new(SniperConfig {
        buy_size: fx("10"),
        ..sniper_config()
    })
    .unwrap();

    let small_impact = small.price_impact_bps(&listing).unwrap();
    let large_impact = large.price_impact_bps(&listing).unwrap();
    assert!(
        large_impact > small_impact,
        "large order impact {large_impact} should exceed small {small_impact}"
    );
}

// ---------------------------------------------------------------- sandwich

fn swap(hash: &str, sender: &str, side: Side, amount_in: &str, amount_out: &str) -> SwapEvent {
    SwapEvent {
        tx_hash: hash.into(),
        sender: sender.into(),
        side,
        amount_in: fx(amount_in),
        amount_out: fx(amount_out),
        is_router_internal: false,
    }
}

#[test]
fn sandwich_detector_finds_a_classic_attack() {
    let block = vec![
        swap("0xattacker-buy", "attacker", Side::Buy, "10", "10"),
        swap("0xvictim", "victim", Side::Buy, "5", "4.5"),
        swap("0xattacker-sell", "attacker", Side::Sell, "9", "11"),
    ];
    let alerts = detect_sandwich(&block).unwrap();
    assert_eq!(alerts.len(), 1);
    assert_eq!(alerts[0].victim_tx, "0xvictim");
    assert_eq!(alerts[0].frontrun_tx, "0xattacker-buy");
    assert_eq!(alerts[0].backrun_tx, "0xattacker-sell");
}

#[test]
fn sandwich_detector_ignores_an_unclosed_frontrun() {
    // The attacker buys but never sells, so there is no sandwich.
    let block = vec![
        swap("0xattacker-buy", "attacker", Side::Buy, "10", "10"),
        swap("0xvictim", "victim", Side::Buy, "5", "4.5"),
    ];
    assert!(detect_sandwich(&block).unwrap().is_empty());
}

#[test]
fn sandwich_detector_ignores_normal_trading() {
    let block = vec![
        swap("0xa", "alice", Side::Buy, "1", "1"),
        swap("0xb", "bob", Side::Sell, "2", "2"),
        swap("0xc", "carol", Side::Buy, "1", "1"),
    ];
    // Bob's sell is a back-run for alice's buy, but bob never bought first.
    let alerts = detect_sandwich(&block).unwrap();
    assert!(alerts.iter().all(|alert| alert.frontrun_tx != "0xb"));
}

#[test]
fn sandwich_detector_skips_router_internal_swaps() {
    let mut victim = swap("0xvictim", "victim", Side::Buy, "5", "4.5");
    victim.is_router_internal = true;
    let block = vec![
        swap("0xattacker-buy", "attacker", Side::Buy, "10", "10"),
        victim,
        swap("0xattacker-sell", "attacker", Side::Sell, "9", "11"),
    ];
    assert!(detect_sandwich(&block).unwrap().is_empty());
}

#[test]
fn sandwich_detector_requires_the_attacker_to_profit() {
    // The back-run sells less than the front-run bought, so no profit.
    let block = vec![
        swap("0xattacker-buy", "attacker", Side::Buy, "10", "10"),
        swap("0xvictim", "victim", Side::Buy, "5", "4.5"),
        swap("0xattacker-sell", "attacker", Side::Sell, "3", "3"),
    ];
    assert!(detect_sandwich(&block).unwrap().is_empty());
}

// ------------------------------------------------- wide arithmetic / bps

#[test]
fn large_magnitudes_do_not_overflow() {
    // 60000 * 0.01 is an ordinary computation that overflows a naive
    // i128 multiply because both operands are scaled by 10^18.
    let price = fx("60000");
    let one_percent = fx("0.01");
    assert_eq!(price.mul(one_percent).unwrap(), fx("600"));

    // A 1% move expressed the way the market maker does it.
    let moved = price.apply_bps(100).unwrap();
    assert_eq!(moved, fx("60600"));
    assert_eq!(moved.sub(price).unwrap().div(price).unwrap(), fx("0.01"));
}

#[test]
fn mul_div_matches_naive_arithmetic_when_it_fits() {
    // a * b / d with small values must agree with plain integer arithmetic.
    assert_eq!(tigerswap_bots::wide::mul_div(6, 7, 2), Some(21));
    assert_eq!(tigerswap_bots::wide::mul_div(-6, 7, 2), Some(-21));
    assert_eq!(tigerswap_bots::wide::mul_div(6, -7, 2), Some(-21));
    assert_eq!(tigerswap_bots::wide::mul_div(6, 7, -2), Some(-21));
    assert_eq!(tigerswap_bots::wide::mul_div(1, 1, 0), None);
}

#[test]
fn mul_div_is_exact_beyond_i128_intermediates() {
    // The product exceeds i128 but the quotient does not, so the 256-bit
    // intermediate is what makes this correct rather than an overflow.
    let a = 10i128.pow(20);
    let b = 10i128.pow(20);
    let d = 10i128.pow(20);
    assert_eq!(tigerswap_bots::wide::mul_div(a, b, d), Some(10i128.pow(20)));

    // A quotient that genuinely does not fit must report None, not wrap.
    assert_eq!(
        tigerswap_bots::wide::mul_div(i128::MAX, i128::MAX, 1),
        None
    );
}

#[test]
fn bps_round_trip() {
    assert_eq!(Fixed::bps(10_000).unwrap(), Fixed::ONE);
    assert_eq!(Fixed::bps(5_000).unwrap(), fx("0.5"));
    assert_eq!(Fixed::bps(-2_500).unwrap(), fx("-0.25"));
    assert_eq!(Fixed::bps(10).unwrap().to_bps().unwrap(), 10);
    assert_eq!(Fixed::bps(1).unwrap().to_bps().unwrap(), 1);
}

#[test]
fn to_bps_truncates_toward_zero() {
    // 0.9 bps reports as 0 so a trade gate refuses it.
    assert_eq!(Fixed::ratio(9, 100_000).unwrap().to_bps().unwrap(), 0);
    assert_eq!(Fixed::ratio(19, 100_000).unwrap().to_bps().unwrap(), 1);
}

#[test]
fn clamp_bounds_are_enforced() {
    let value = fx("5");
    assert_eq!(value.clamp(fx("1"), fx("10")).unwrap(), fx("5"));
    assert_eq!(fx("0.5").clamp(fx("1"), fx("10")).unwrap(), fx("1"));
    assert_eq!(fx("50").clamp(fx("1"), fx("10")).unwrap(), fx("10"));
    assert!(value.clamp(fx("10"), fx("1")).is_err());
}
