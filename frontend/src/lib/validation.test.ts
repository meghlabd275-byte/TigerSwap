import assert from "node:assert/strict";
import { test } from "node:test";

import { chainValidator } from "./types";
import {
  ValidationError,
  address,
  array,
  boolean,
  displayText,
  httpURL,
  integer,
  marketSymbol,
  nonNegative,
  number,
  object,
  oneOf,
  string,
  symbol,
  withDefault,
} from "./validation";

test("string accepts only strings", () => {
  assert.equal(string("x"), "x");
  assert.throws(() => string(1), ValidationError);
});

test("number rejects NaN and Infinity", () => {
  assert.equal(number("12.5"), 12.5);
  assert.throws(() => number(Number.NaN), ValidationError);
  assert.throws(() => number(Number.POSITIVE_INFINITY), ValidationError);
  assert.throws(() => number("not-a-number"), ValidationError);
});

test("integer rejects fractional values", () => {
  assert.equal(integer(3), 3);
  assert.throws(() => integer(3.5), ValidationError);
});

test("nonNegative rejects negative values", () => {
  assert.equal(nonNegative(0), 0);
  assert.throws(() => nonNegative(-1), ValidationError);
});

test("boolean accepts only booleans", () => {
  assert.equal(boolean(true), true);
  assert.throws(() => boolean("true"), ValidationError);
});

test("oneOf restricts to the allow-list", () => {
  const mode = oneOf(["cross", "isolated"] as const);
  assert.equal(mode("cross"), "cross");
  assert.throws(() => mode("other"), ValidationError);
});

test("array validates every element", () => {
  assert.deepEqual(array(number)([1, 2, 3]), [1, 2, 3]);
  assert.throws(() => array(number)([1, "x"]), ValidationError);
});

test("object enforces required and optional fields", () => {
  const validator = object<{ a: string; b?: number }>(
    { a: string, b: number },
    { optional: ["b"] },
  );
  assert.deepEqual(validator({ a: "x" }), { a: "x" });
  assert.deepEqual(validator({ a: "x", b: 2 }), { a: "x", b: 2 });
  assert.throws(() => validator({ b: 2 }), ValidationError);
  assert.throws(() => validator(null), ValidationError);
});

// A token symbol reaches the DOM, so markup and control characters must not
// survive validation.
test("symbol rejects markup and overlong values", () => {
  assert.equal(symbol("RUSD"), "RUSD");
  assert.equal(symbol("tgr-usd.2"), "tgr-usd.2");
  assert.throws(() => symbol("<script>"), ValidationError);
  assert.throws(() => symbol(""), ValidationError);
  assert.throws(() => symbol("A".repeat(17)), ValidationError);
});

test("address requires a 20-byte hex value", () => {
  const good = "0x" + "a".repeat(40);
  assert.equal(address(good), good);
  assert.throws(() => address("0x1234"), ValidationError);
  assert.throws(() => address("not-an-address"), ValidationError);
});

test("displayText strips control characters and caps length", () => {
  assert.equal(displayText("a\u0000b\u001fc"), "abc");
  assert.throws(() => displayText("x".repeat(300)), ValidationError);
});

// `object` must let a `withDefault` field supply its own value when the key is
// absent. Before this was handled, the required-field guard fired first, so a
// field the author had explicitly made optional behaved as required and its
// default was dead code. That silently dropped rows in production.
test("object applies withDefault for a missing key", () => {
  const validator = object<{ a: string; b: string }>({
    a: string,
    b: withDefault(string, "fallback"),
  });
  assert.deepEqual(validator({ a: "x" }), { a: "x", b: "fallback" });
  assert.deepEqual(validator({ a: "x", b: "given" }), { a: "x", b: "given" });
  assert.throws(() => validator({}), ValidationError);
});

test("object still requires a field that has no default", () => {
  const validator = object<{ a: string }>({ a: string });
  assert.throws(() => validator({}), ValidationError);
});

// Market symbols use the BASE/QUOTE form; the single-token `symbol` validator
// rejects the slash, which used to blank every derivatives and margin listing.
test("marketSymbol accepts BASE/QUOTE and plain symbols", () => {
  assert.equal(marketSymbol("BTC/RUSD"), "BTC/RUSD");
  assert.equal(marketSymbol("EURUSD/RUSD"), "EURUSD/RUSD");
  assert.equal(marketSymbol("TGR"), "TGR");
});

test("marketSymbol rejects markup and malformed pairs", () => {
  assert.throws(() => marketSymbol("<script>"), ValidationError);
  assert.throws(() => marketSymbol("BTC/"), ValidationError);
  assert.throws(() => marketSymbol("/RUSD"), ValidationError);
  assert.throws(() => marketSymbol("BTC/RUSD/ETH"), ValidationError);
  assert.throws(() => marketSymbol(""), ValidationError);
});

test("symbol still rejects a slash, so token symbols stay single-token", () => {
  assert.throws(() => symbol("BTC/RUSD"), ValidationError);
});

test("httpURL accepts http(s) and rejects script or relative values", () => {
  assert.equal(httpURL("https://etherscan.io"), "https://etherscan.io");
  assert.equal(httpURL("http://127.0.0.1:8545"), "http://127.0.0.1:8545");

  for (const bad of ["javascript:alert(1)", "data:text/html,<script>", "/tx/1", "etherscan.io"]) {
    assert.throws(() => httpURL(bad), ValidationError, `${bad} must be rejected`);
  }
});

test("chainValidator drops a chain whose explorer URL is not http(s)", () => {
  const valid = chainValidator(
    { id: 1, name: "Ethereum", chainType: "evm", nativeSymbol: "ETH", explorerURL: "https://etherscan.io", isActive: true },
    "chain",
  );
  assert.equal(valid.explorerURL, "https://etherscan.io");

  assert.throws(
    () =>
      chainValidator(
        { id: 1, name: "Evil", chainType: "evm", nativeSymbol: "ETH", explorerURL: "javascript:alert(1)", isActive: true },
        "chain",
      ),
    ValidationError,
  );
});
