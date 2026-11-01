/**
 * Live market prices from the TigerSwap WebSocket feed.
 *
 * The hook merges the server-published ticks into a symbol-keyed map so a
 * component can render live prices without polling. It starts the feed on
 * mount and closes it on unmount, so no socket outlives its component.
 */

"use client";

import { useEffect, useState } from "react";

import { openMarketFeed } from "@/lib/marketFeed";
import type { MarketTick } from "@/lib/types";

export type MarketFeedStatus = "connecting" | "open" | "closed";

export interface LivePrices {
  /** The latest validated tick per symbol. */
  prices: Map<string, MarketTick>;
  status: MarketFeedStatus;
}

export function useMarketFeed(): LivePrices {
  const [prices, setPrices] = useState<Map<string, MarketTick>>(() => new Map());
  const [status, setStatus] = useState<MarketFeedStatus>("connecting");

  useEffect(() => {
    const feed = openMarketFeed({
      onTick: (tick) => {
        setPrices((previous) => {
          const next = new Map(previous);
          next.set(tick.symbol, tick);
          return next;
        });
      },
      onStatus: setStatus,
    });
    return () => feed.close();
  }, []);

  return { prices, status };
}
