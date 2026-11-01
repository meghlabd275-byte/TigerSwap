/**
 * Live market feed client for the TigerSwap WebSocket at `/ws`.
 *
 * The socket carries real oracle prices published by the backend. Every frame
 * is validated with `marketTickValidator` before it is handed to a listener, so
 * a malformed frame is dropped rather than rendered. The connection reconnects
 * with exponential backoff and stops for good on a normal close, so a component
 * unmount does not leave a reconnect loop behind.
 */

import { marketTickValidator, type MarketTick } from "./types";
import { resolveApiBase } from "./apiClient";

export type MarketTickListener = (tick: MarketTick) => void;

/**
 * Resolves the WebSocket URL for the market feed.
 *
 * `NEXT_PUBLIC_WS_URL` wins when set. Otherwise the URL is derived from the API
 * base: an absolute base keeps its host, a relative base (the default) uses the
 * current origin, so the socket is same-origin in a browser and the scheme is
 * upgraded to `wss:` whenever the page itself is served over HTTPS.
 */
export function resolveWebSocketUrl(): string {
  const configured = process.env.NEXT_PUBLIC_WS_URL?.trim();
  if (configured) {
    return configured;
  }

  const base = resolveApiBase();
  if (/^https?:\/\//i.test(base)) {
    const url = new URL(base);
    url.protocol = url.protocol === "https:" ? "wss:" : "ws:";
    url.pathname = "/ws";
    url.search = "";
    return url.toString();
  }

  if (typeof window === "undefined") {
    throw new Error("cannot resolve a WebSocket URL outside the browser");
  }
  const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
  return `${protocol}//${window.location.host}/ws`;
}

export interface MarketFeedOptions {
  /** Invoked for every valid tick. */
  onTick: MarketTickListener;
  /** Invoked when the socket opens, closes or errors, for UI status. */
  onStatus?: (status: "connecting" | "open" | "closed") => void;
  /** Milliseconds to wait before the first reconnect attempt. */
  initialBackoffMs?: number;
  /** Upper bound for the reconnect backoff. */
  maxBackoffMs?: number;
}

export interface MarketFeed {
  /** Closes the socket and prevents any further reconnect. */
  close: () => void;
}

/**
 * Opens the market feed. Returns a handle whose `close` tears the connection
 * down permanently.
 */
export function openMarketFeed(options: MarketFeedOptions): MarketFeed {
  const { onTick, onStatus, initialBackoffMs = 1000, maxBackoffMs = 30_000 } = options;

  let socket: WebSocket | null = null;
  let closed = false;
  let backoff = initialBackoffMs;
  let retryTimer: ReturnType<typeof setTimeout> | null = null;

  const connect = () => {
    if (closed) {
      return;
    }
    onStatus?.("connecting");

    let ws: WebSocket;
    try {
      ws = new WebSocket(resolveWebSocketUrl());
    } catch {
      scheduleReconnect();
      return;
    }
    socket = ws;

    ws.onopen = () => {
      backoff = initialBackoffMs;
      onStatus?.("open");
    };

    ws.onmessage = (event: MessageEvent) => {
      if (typeof event.data !== "string") {
        return;
      }
      let raw: unknown;
      try {
        raw = JSON.parse(event.data);
      } catch {
        return;
      }
      try {
        onTick(marketTickValidator(raw));
      } catch {
        // A frame that does not satisfy the validator is not a price.
      }
    };

    ws.onerror = () => {
      // `onclose` always follows `onerror`, so reconnection is handled there.
    };

    ws.onclose = () => {
      socket = null;
      if (closed) {
        onStatus?.("closed");
        return;
      }
      scheduleReconnect();
    };
  };

  const scheduleReconnect = () => {
    if (closed) {
      return;
    }
    onStatus?.("closed");
    const delay = backoff;
    backoff = Math.min(backoff * 2, maxBackoffMs);
    retryTimer = setTimeout(connect, delay);
  };

  connect();

  return {
    close: () => {
      closed = true;
      if (retryTimer !== null) {
        clearTimeout(retryTimer);
        retryTimer = null;
      }
      if (socket) {
        socket.onclose = null;
        socket.close();
        socket = null;
      }
      onStatus?.("closed");
    },
  };
}
