//! Integration tests for the stateful engine and its C ABI.

use std::ffi::{CStr, CString};
use std::os::raw::c_char;

use serde_json::{json, Value};
use tigerswap_bots::engine::{order_to_json, Engine};
use tigerswap_bots::ffi;

fn snapshot(symbol: &str) -> Value {
    json!({
        "symbol": symbol,
        "bids": [{"price": "60000", "quantity": "2"}],
        "asks": [{"price": "60010", "quantity": "2"}],
        "vwap": "60005",
        "timestamp_ms": 1_700_000_000_000i64,
    })
}

#[test]
fn market_maker_quotes_two_sides() {
    let engine = Engine::new();
    engine
        .upsert(
            1,
            "market_maker",
            &json!({"spread_bps": 10, "target_depth": "1"}),
        )
        .unwrap();

    let orders = engine.step(1, &snapshot("BTC/RUSD")).unwrap();
    assert_eq!(orders.len(), 2);
    assert!(orders.iter().any(|order| order.price.raw() > 0));
}

#[test]
fn stepping_an_unknown_bot_is_an_error() {
    let engine = Engine::new();
    let error = engine.step(42, &snapshot("BTC/RUSD")).unwrap_err();
    assert!(error.to_string().contains("not configured"));
}

#[test]
fn unknown_strategy_is_rejected() {
    let engine = Engine::new();
    assert!(engine.upsert(1, "no_such_strategy", &json!({})).is_err());
}

#[test]
fn copy_strategy_is_configured_by_the_engine() {
    let engine = Engine::new();
    // The copy bot is now a first-class engine strategy: it validates its own
    // configuration and mirrors signals through `copy`.
    engine
        .upsert(1, "copy", &json!({"scale_bps": 100}))
        .expect("copy is a known strategy");
    // A scale outside 1..=10000 is refused at construction.
    assert!(engine.upsert(2, "copy", &json!({"scale_bps": 0})).is_err());
}

#[test]
fn a_grid_quotes_within_its_range() {
    let engine = Engine::new();
    engine
        .upsert(
            7,
            "grid",
            &json!({
                "lower_price": "50000",
                "upper_price": "70000",
                "levels": 5,
                "order_size": "0.1",
            }),
        )
        .unwrap();

    // The mid sits inside the range, so at least one leg is produced.
    let orders = engine.step(7, &snapshot("BTC/RUSD")).unwrap();
    assert!(!orders.is_empty());
}

#[test]
fn dca_waits_for_its_interval() {
    let engine = Engine::new();
    engine
        .upsert(
            3,
            "dca",
            &json!({
                "amount_per_interval": "100",
                "interval_ms": 3_600_000,
                "total_intervals": 10,
            }),
        )
        .unwrap();

    let first = engine.step(3, &snapshot("BTC/RUSD")).unwrap();
    assert_eq!(first.len(), 1);

    // Same timestamp: the interval has not elapsed, so no further order.
    let second = engine.step(3, &snapshot("BTC/RUSD")).unwrap();
    assert!(second.is_empty());
}

#[test]
fn removing_a_bot_stops_it() {
    let engine = Engine::new();
    engine
        .upsert(9, "market_maker", &json!({"spread_bps": 10, "target_depth": "1"}))
        .unwrap();
    assert!(engine.remove(9));
    assert!(engine.step(9, &snapshot("BTC/RUSD")).is_err());
}

#[test]
fn arbitrage_emits_both_legs_on_an_edge() {
    let engine = Engine::new();
    engine
        .upsert(
            4,
            "arbitrage",
            &json!({"min_edge_bps": 1, "max_quantity": "1"}),
        )
        .unwrap();

    // Venue A is cheap (ask 100), venue B is rich (bid 110): a clear edge.
    let venues = json!([
        {
            "name": "v2",
            "fee_bps": 0,
            "snapshot": {
                "symbol": "BTC/RUSD",
                "bids": [{"price": "99", "quantity": "1"}],
                "asks": [{"price": "100", "quantity": "1"}],
                "vwap": "100",
                "timestamp_ms": 1_700_000_000_000i64,
            }
        },
        {
            "name": "v3",
            "fee_bps": 0,
            "snapshot": {
                "symbol": "BTC/RUSD",
                "bids": [{"price": "110", "quantity": "1"}],
                "asks": [{"price": "111", "quantity": "1"}],
                "vwap": "110",
                "timestamp_ms": 1_700_000_000_000i64,
            }
        }
    ]);

    let orders = engine.arbitrage(4, &venues).unwrap();
    assert_eq!(orders.len(), 2);
}

#[test]
fn rebalance_plans_a_drifted_leg() {
    let engine = Engine::new();
    engine
        .upsert(5, "rebalance", &json!({"drift_threshold_bps": 100}))
        .unwrap();

    let holdings = json!([
        {"symbol": "BTC/RUSD", "weight_bps": 5_000, "value": "9000"},
        {"symbol": "ETH/RUSD", "weight_bps": 5_000, "value": "1000"}
    ]);

    let trades = engine.rebalance(5, &holdings).unwrap();
    assert!(!trades.is_empty());
}

#[test]
fn liquidation_fires_only_when_unhealthy() {
    let engine = Engine::new();
    engine
        .upsert(
            6,
            "liquidation",
            &json!({"close_factor_bps": 5_000, "min_bonus_bps": 100}),
        )
        .unwrap();

    let healthy = json!({
        "id": "acct-1",
        "collateral_value": "1000",
        "debt_value": "100",
        "liquidation_threshold": "0.8",
        "liquidation_bonus_bps": 500
    });
    assert!(engine.liquidation(6, &healthy).unwrap().is_none());

    let unhealthy = json!({
        "id": "acct-2",
        "collateral_value": "1000",
        "debt_value": "900",
        "liquidation_threshold": "0.8",
        "liquidation_bonus_bps": 500
    });
    let decision = engine.liquidation(6, &unhealthy).unwrap().unwrap();
    assert_eq!(decision.position_id, "acct-2");
}

// ---- FFI surface ----

unsafe fn read_and_free(ptr: *mut c_char) -> Value {
    assert!(!ptr.is_null(), "FFI returned a null pointer");
    let text = CStr::from_ptr(ptr).to_str().unwrap().to_string();
    ffi::bot_free(ptr);
    serde_json::from_str(&text).unwrap()
}

unsafe fn c(text: &str) -> CString {
    CString::new(text).unwrap()
}

#[test]
fn ffi_round_trip_drives_a_bot() {
    unsafe {
        let engine = ffi::bot_engine_new();
        assert!(!engine.is_null());

        let strategy = c("market_maker");
        let config = c(r#"{"spread_bps":10,"target_depth":"1"}"#);
        let upsert = read_and_free(ffi::bot_upsert(engine, 1, strategy.as_ptr(), config.as_ptr()));
        assert_eq!(upsert["ok"], json!(true));

        assert_eq!(ffi::bot_count(engine), 1);

        let snap = c(&snapshot("BTC/RUSD").to_string());
        let stepped = read_and_free(ffi::bot_step(engine, 1, snap.as_ptr()));
        assert_eq!(stepped["ok"], json!(true));
        assert_eq!(stepped["data"].as_array().unwrap().len(), 2);

        let removed = read_and_free(ffi::bot_remove(engine, 1));
        assert_eq!(removed["ok"], json!(true));
        assert_eq!(ffi::bot_count(engine), 0);

        ffi::bot_engine_free(engine);
    }
}

#[test]
fn ffi_reports_errors_in_the_envelope() {
    unsafe {
        let engine = ffi::bot_engine_new();

        // A malformed config must come back as an error envelope, not a panic.
        let strategy = c("market_maker");
        let config = c("not json");
        let response = read_and_free(ffi::bot_upsert(engine, 1, strategy.as_ptr(), config.as_ptr()));
        assert_eq!(response["ok"], json!(false));
        assert!(response["error"].as_str().unwrap().contains("JSON"));

        // Stepping an unconfigured bot is likewise an error envelope.
        let snap = c(&snapshot("BTC/RUSD").to_string());
        let stepped = read_and_free(ffi::bot_step(engine, 99, snap.as_ptr()));
        assert_eq!(stepped["ok"], json!(false));

        ffi::bot_engine_free(engine);
    }
}

#[test]
fn ffi_orders_render_as_exact_decimal_strings() {
    let engine = Engine::new();
    engine
        .upsert(1, "market_maker", &json!({"spread_bps": 10, "target_depth": "1"}))
        .unwrap();
    let orders = engine.step(1, &snapshot("BTC/RUSD")).unwrap();
    for order in &orders {
        let rendered = order_to_json(order);
        assert!(rendered["price"].is_string());
        assert!(rendered["quantity"].is_string());
        // No float ever crosses the boundary.
        assert!(rendered["price"].as_str().unwrap().parse::<f64>().is_ok());
    }
}

#[test]
fn copy_bot_is_configurable_and_mirrors_signals() {
    let engine = Engine::new();
    engine
        .upsert(1, "copy", &json!({"scale_bps": 5000, "copy_perps": true}))
        .unwrap();

    let signals = json!([
        {"symbol": "BTC/RUSD", "market": "spot", "side": "buy", "kind": "limit",
         "price": "60000", "quantity": "2", "source_id": "t-1"},
        {"symbol": "ETH/RUSD", "market": "perp", "side": "sell", "kind": "market",
         "price": "0", "quantity": "4", "source_id": "t-2"}
    ]);

    let orders = engine.copy(1, &signals).unwrap();
    assert_eq!(orders.len(), 2);
    // The 50% ratio halves both signals.
    assert_eq!(orders[0].quantity.to_string(), "1");
    assert_eq!(orders[1].quantity.to_string(), "2");
}

#[test]
fn copy_bot_rejects_a_malformed_signal() {
    let engine = Engine::new();
    engine
        .upsert(1, "copy", &json!({"scale_bps": 10000}))
        .unwrap();
    // A signal with an unknown market is refused, not silently dropped.
    let signals = json!([
        {"symbol": "BTC/RUSD", "market": "futures", "side": "buy", "kind": "limit",
         "price": "60000", "quantity": "1", "source_id": "t-1"}
    ]);
    assert!(engine.copy(1, &signals).is_err());
}
