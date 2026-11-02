package services

import (
	"context"
	"encoding/json"
	"log"
	"time"
)

// TrackedPriceSymbols are the assets TigerSwap quotes. It is the single source
// of truth for both the REST price endpoints and the WebSocket market feed, so
// the two can never drift apart.
var TrackedPriceSymbols = []string{
	"ETH", "BTC", "USDC", "USDT", "BNB", "SOL", "MATIC", "LINK", "UNI", "AAVE",
	"DOT", "AVAX", "LTC", "XRP", "DOGE", "TRX", "APT", "ARB", "OP", "NEAR",
	"INJ", "SUI", "FIL", "ATOM", "XTZ", "ADA", "ALGO", "XLM", "HBAR", "FTM",
	"EGLD", "GRT", "CRO", "ONE", "LDO", "IMX", "GALA", "ENJ", "BAT", "ZEC",
	"XMR", "DASH", "ZIL", "ENS", "COMP", "DAI",
}

// DefaultMarketFeedInterval is the cadence of the WebSocket market feed when no
// interval is configured. Prices come from the oracle aggregator, which caches,
// so a short interval does not translate into an upstream call per client.
const DefaultMarketFeedInterval = 5 * time.Second

// MarketTick is one real price observation published on the WebSocket feed.
// The JSON field names match the frontend validator for the feed message.
type MarketTick struct {
	Type      string  `json:"type"`
	Symbol    string  `json:"symbol"`
	PriceUSD  float64 `json:"priceUSD"`
	Change24h float64 `json:"change24h"`
	Timestamp int64   `json:"timestamp"`
}

// StartMarketFeed publishes real oracle prices to every connected WebSocket
// client until ctx is cancelled. Each tick is a price TigerSwap actually
// observed; a symbol whose price cannot be read is skipped rather than sent
// with an invented value.
func StartMarketFeed(ctx context.Context, agg *PriceAggregator, interval time.Duration) {
	if agg == nil {
		log.Println("market feed not started: price aggregator is nil")
		return
	}
	if interval <= 0 {
		interval = DefaultMarketFeedInterval
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				publishMarketSnapshot(ctx, agg)
			}
		}
	}()
}

// DefaultPositionRefreshInterval is how often open perpetual and copy positions
// are re-marked to the live oracle price. Without it a position would report
// its entry price as its current price for as long as it stays open.
const DefaultPositionRefreshInterval = 10 * time.Second

// StartPositionMaintenance re-marks open perpetual and copy-trading positions to
// the live oracle price until ctx is cancelled. A position whose price cannot be
// read keeps its previous mark rather than being zeroed.
func StartPositionMaintenance(ctx context.Context, perps *PerpetualEngine, copy *CopyTradingEngine, interval time.Duration) {
	if perps == nil && copy == nil {
		log.Println("position maintenance not started: no engines configured")
		return
	}
	if interval <= 0 {
		interval = DefaultPositionRefreshInterval
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if perps != nil {
					if err := perps.UpdatePositions(ctx); err != nil {
						log.Printf("position maintenance: perpetual update failed: %v", err)
					}
				}
				if copy != nil {
					if err := copy.UpdateCopyPositions(ctx); err != nil {
						log.Printf("position maintenance: copy update failed: %v", err)
					}
				}
			}
		}
	}()
}

func publishMarketSnapshot(ctx context.Context, agg *PriceAggregator) {
	prices, err := agg.GetMultiChainPrices(ctx, TrackedPriceSymbols)
	if err != nil {
		log.Printf("market feed: price read failed: %v", err)
		return
	}

	now := time.Now().UTC().Unix()
	for _, symbol := range TrackedPriceSymbols {
		pd, ok := prices[symbol]
		if !ok || pd == nil || pd.USD <= 0 {
			continue
		}

		payload, err := json.Marshal(MarketTick{
			Type:      "price",
			Symbol:    pd.Symbol,
			PriceUSD:  pd.USD,
			Change24h: pd.USDChange24h,
			Timestamp: now,
		})
		if err != nil {
			continue
		}
		Broadcast(payload)
	}
}
