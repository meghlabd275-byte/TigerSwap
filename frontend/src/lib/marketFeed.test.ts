import assert from "node:assert/strict";
import { test } from "node:test";

import { marketTickValidator } from "./types";
import { resolveWebSocketUrl } from "./marketFeed";

test("marketTickValidator accepts a real price tick", () => {
  const tick = marketTickValidator({
    type: "price",
    symbol: "BTC",
    priceUSD: 61234.5,
    change24h: 1.25,
    timestamp: 1735689600,
  });
  assert.equal(tick.symbol, "BTC");
  assert.equal(tick.priceUSD, 61234.5);
});

test("marketTickValidator rejects a non-price message", () => {
  assert.throws(() =>
    marketTickValidator({ type: "hello", symbol: "BTC", priceUSD: 1, change24h: 0, timestamp: 1 }),
  );
});

test("marketTickValidator rejects a negative price", () => {
  assert.throws(() =>
    marketTickValidator({
      type: "price",
      symbol: "BTC",
      priceUSD: -1,
      change24h: 0,
      timestamp: 1,
    }),
  );
});

test("marketTickValidator rejects a missing symbol", () => {
  assert.throws(() =>
    marketTickValidator({ type: "price", priceUSD: 1, change24h: 0, timestamp: 1 }),
  );
});

test("resolveWebSocketUrl upgrades an absolute https API base to wss", () => {
  const previous = process.env.NEXT_PUBLIC_API_URL;
  process.env.NEXT_PUBLIC_API_URL = "https://api.tigerswap.io/api/v1";
  try {
    assert.equal(resolveWebSocketUrl(), "wss://api.tigerswap.io/ws");
  } finally {
    process.env.NEXT_PUBLIC_API_URL = previous;
  }
});

test("resolveWebSocketUrl uses an explicit NEXT_PUBLIC_WS_URL when set", () => {
  const prevApi = process.env.NEXT_PUBLIC_API_URL;
  const prevWs = process.env.NEXT_PUBLIC_WS_URL;
  process.env.NEXT_PUBLIC_WS_URL = "wss://feed.tigerswap.io/ws";
  try {
    assert.equal(resolveWebSocketUrl(), "wss://feed.tigerswap.io/ws");
  } finally {
    process.env.NEXT_PUBLIC_API_URL = prevApi;
    process.env.NEXT_PUBLIC_WS_URL = prevWs;
  }
});
