//! The C ABI the Go control plane links against.
//!
//! Every function takes and returns NUL-terminated UTF-8 JSON. A return value
//! is always a JSON envelope — `{"ok":true,"data":...}` or
//! `{"ok":false,"error":"..."}` — so the caller never has to interpret a null
//! pointer as "no work": a null return only ever means allocation failed, and
//! the caller treats that as a hard error.
//!
//! The engine is an opaque handle. It is `Sync`, so several Go goroutines may
//! call into one handle concurrently; the engine's own lock serialises them.

use std::ffi::{CStr, CString};
use std::os::raw::c_char;

use serde_json::{json, Value};

use crate::engine::{liquidation_to_json, order_to_json, rebalance_to_json, Engine};

/// Wraps a result into the JSON envelope and hands ownership to the caller.
///
/// The caller frees the pointer with [`bot_free`].
fn respond(result: Result<Value, String>) -> *mut c_char {
    let envelope = match result {
        Ok(data) => json!({ "ok": true, "data": data }),
        Err(error) => json!({ "ok": false, "error": error }),
    };
    into_c_string(envelope)
}

fn into_c_string(value: Value) -> *mut c_char {
    // A JSON document never contains a NUL, so this cannot fail on content; the
    // only failure is an allocation failure, which the null return signals.
    match CString::new(value.to_string()) {
        Ok(text) => text.into_raw(),
        Err(_) => std::ptr::null_mut(),
    }
}

/// Reads a borrowed C string.
///
/// # Safety
/// `ptr` must be null or a valid NUL-terminated string for the duration of the
/// call.
unsafe fn read_str<'a>(ptr: *const c_char, what: &str) -> Result<&'a str, String> {
    if ptr.is_null() {
        return Err(format!("{what} is null"));
    }
    CStr::from_ptr(ptr)
        .to_str()
        .map_err(|_| format!("{what} is not valid UTF-8"))
}

fn parse_json(text: &str, what: &str) -> Result<Value, String> {
    serde_json::from_str(text).map_err(|error| format!("{what} is not valid JSON: {error}"))
}

/// Runs `body` against the engine, converting every failure to a message.
///
/// # Safety
/// `engine` must be a live handle.
unsafe fn with_engine<F>(engine: *mut Engine, body: F) -> *mut c_char
where
    F: FnOnce(&Engine) -> Result<Value, String>,
{
    let Some(engine) = engine.as_ref() else {
        return respond(Err("engine is null".to_string()));
    };
    respond(body(engine))
}

/// Creates an engine and returns an opaque handle. The caller owns it and must
/// release it with [`bot_engine_free`].
#[no_mangle]
pub extern "C" fn bot_engine_new() -> *mut Engine {
    Box::into_raw(Box::new(Engine::new()))
}

/// Drops an engine handle. A null handle is ignored.
///
/// # Safety
/// `engine` must be a handle from [`bot_engine_new`] that has not been freed.
#[no_mangle]
pub unsafe extern "C" fn bot_engine_free(engine: *mut Engine) {
    if !engine.is_null() {
        drop(Box::from_raw(engine));
    }
}

/// Configures or replaces a bot.
///
/// # Safety
/// `engine` must be a live handle; `strategy` and `config` must be valid C
/// strings.
#[no_mangle]
pub unsafe extern "C" fn bot_upsert(
    engine: *mut Engine,
    id: u64,
    strategy: *const c_char,
    config: *const c_char,
) -> *mut c_char {
    with_engine(engine, |engine| {
        let strategy = read_str(strategy, "strategy")?;
        let config = parse_json(read_str(config, "config")?, "config")?;
        engine
            .upsert(id, strategy, &config)
            .map(|()| json!({ "id": id }))
            .map_err(|error| error.to_string())
    })
}

/// Removes a bot. Succeeds even when the bot was not configured.
///
/// # Safety
/// `engine` must be a live handle.
#[no_mangle]
pub unsafe extern "C" fn bot_remove(engine: *mut Engine, id: u64) -> *mut c_char {
    with_engine(engine, |engine| {
        Ok(json!({ "removed": engine.remove(id) }))
    })
}

/// Advances a bot against a snapshot.
///
/// # Safety
/// `engine` must be a live handle; `snapshot` must be a valid C string.
#[no_mangle]
pub unsafe extern "C" fn bot_step(
    engine: *mut Engine,
    id: u64,
    snapshot: *const c_char,
) -> *mut c_char {
    with_engine(engine, |engine| {
        let snapshot = parse_json(read_str(snapshot, "snapshot")?, "snapshot")?;
        engine
            .step(id, &snapshot)
            .map(|orders| Value::Array(orders.iter().map(order_to_json).collect()))
            .map_err(|error| error.to_string())
    })
}

/// Runs an arbitrage bot across the supplied venues.
///
/// # Safety
/// `engine` must be a live handle; `venues` must be a valid C string.
#[no_mangle]
pub unsafe extern "C" fn bot_arbitrage(
    engine: *mut Engine,
    id: u64,
    venues: *const c_char,
) -> *mut c_char {
    with_engine(engine, |engine| {
        let venues = parse_json(read_str(venues, "venues")?, "venues")?;
        engine
            .arbitrage(id, &venues)
            .map(|orders| Value::Array(orders.iter().map(order_to_json).collect()))
            .map_err(|error| error.to_string())
    })
}

/// Evaluates a sniper bot against a new listing.
///
/// # Safety
/// `engine` must be a live handle; `listing` must be a valid C string.
#[no_mangle]
pub unsafe extern "C" fn bot_sniper(
    engine: *mut Engine,
    id: u64,
    listing: *const c_char,
) -> *mut c_char {
    with_engine(engine, |engine| {
        let listing = parse_json(read_str(listing, "listing")?, "listing")?;
        engine
            .sniper(id, &listing)
            .map(|maybe| match maybe {
                Some(order) => order_to_json(&order),
                None => Value::Null,
            })
            .map_err(|error| error.to_string())
    })
}

/// Plans a rebalance for the supplied holdings.
///
/// # Safety
/// `engine` must be a live handle; `holdings` must be a valid C string.
#[no_mangle]
pub unsafe extern "C" fn bot_rebalance(
    engine: *mut Engine,
    id: u64,
    holdings: *const c_char,
) -> *mut c_char {
    with_engine(engine, |engine| {
        let holdings = parse_json(read_str(holdings, "holdings")?, "holdings")?;
        engine
            .rebalance(id, &holdings)
            .map(|trades| Value::Array(trades.iter().map(rebalance_to_json).collect()))
            .map_err(|error| error.to_string())
    })
}

/// Evaluates a liquidation bot against a borrow position.
///
/// # Safety
/// `engine` must be a live handle; `position` must be a valid C string.
#[no_mangle]
pub unsafe extern "C" fn bot_liquidation(
    engine: *mut Engine,
    id: u64,
    position: *const c_char,
) -> *mut c_char {
    with_engine(engine, |engine| {
        let position = parse_json(read_str(position, "position")?, "position")?;
        engine
            .liquidation(id, &position)
            .map(|maybe| match maybe {
                Some(liquidation) => liquidation_to_json(&liquidation),
                None => Value::Null,
            })
            .map_err(|error| error.to_string())
    })
}

/// Mirrors a followed trader's signals through a copy bot.
///
/// # Safety
/// `engine` must be a live handle; `signals` must be a valid C string.
#[no_mangle]
pub unsafe extern "C" fn bot_copy(
    engine: *mut Engine,
    id: u64,
    signals: *const c_char,
) -> *mut c_char {
    with_engine(engine, |engine| {
        let signals = parse_json(read_str(signals, "signals")?, "signals")?;
        engine
            .copy(id, &signals)
            .map(|orders| Value::Array(orders.iter().map(order_to_json).collect()))
            .map_err(|error| error.to_string())
    })
}

/// Frees a string returned by any of the functions above. A null pointer is
/// ignored.
///
/// # Safety
/// `ptr` must be a pointer returned by one of the FFI functions that has not
/// already been freed.
#[no_mangle]
pub unsafe extern "C" fn bot_free(ptr: *mut c_char) {
    if !ptr.is_null() {
        drop(CString::from_raw(ptr));
    }
}

/// The number of configured bots behind a handle.
///
/// # Safety
/// `engine` must be a live handle.
#[no_mangle]
pub unsafe extern "C" fn bot_count(engine: *mut Engine) -> u64 {
    match engine.as_ref() {
        Some(engine) => engine.len() as u64,
        None => 0,
    }
}
