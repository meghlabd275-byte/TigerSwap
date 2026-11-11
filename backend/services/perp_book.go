package services

import (
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"sort"
	"strconv"
	"time"
)

// errPerpCrossedBook is returned when the aggregated resting orders would form
// a crossed book, which the bot snapshot validator rejects.
var errPerpCrossedBook = errors.New("perpetual book is crossed")

// BookLevel is one aggregated price level of a market, rendered as exact
// decimal strings so a bot consumes the same value the engine holds.
type BookLevel struct {
	Price    string `json:"price"`
	Quantity string `json:"quantity"`
}

// MarketSnapshotData is the market view a bot reasons about. It is built from
// the engine's own resting orders and its own oracle mark, never from a
// caller-supplied or fabricated price.
type MarketSnapshotData struct {
	Symbol      string      `json:"symbol"`
	Bids        []BookLevel `json:"bids"`
	Asks        []BookLevel `json:"asks"`
	VWAP        string      `json:"vwap"`
	TimestampMS int64       `json:"timestamp_ms"`
}

// BookSnapshot aggregates the pair's resting limit orders into a two-sided
// book and pairs it with the oracle mark price.
//
// A long limit is a bid and a short limit is an ask, which is how the perp
// engine's own order conditions treat them. Levels beyond `levels` on either
// side are dropped. The mark price is read from the oracle; when it cannot be
// read the snapshot is refused rather than stamped with a stale price.
func (e *PerpetualEngine) BookSnapshot(ctx context.Context, pair string, levels int) (*MarketSnapshotData, error) {
	if levels <= 0 {
		levels = 20
	}
	if !e.isValidPair(pair) {
		return nil, ErrPerpMarketNotListed
	}

	mark, err := e.getMarkPrice(ctx, pair)
	if err != nil {
		return nil, err
	}

	type aggregate struct {
		price float64
		size  float64
	}
	bidsByPrice := map[float64]float64{}
	asksByPrice := map[float64]float64{}

	e.mu.RLock()
	for _, order := range e.orders {
		if order.Pair != pair || order.Status != "pending" || order.OrderType != "limit" {
			continue
		}
		// A resting order's size is its notional in quote terms; the book is
		// denominated in base units, so convert back through the order's price.
		notional, err := perpAmountValue(order.Size)
		if err != nil || order.Price <= 0 {
			continue
		}
		priceRat, err := rationalFromFloat(order.Price)
		if err != nil {
			continue
		}
		units, err := rationalToFloat(new(big.Rat).Quo(notional, priceRat))
		if err != nil || units <= 0 {
			continue
		}
		switch order.Side {
		case "long":
			bidsByPrice[order.Price] += units
		case "short":
			asksByPrice[order.Price] += units
		}
	}
	e.mu.RUnlock()

	bids := aggregateLevels(bidsByPrice, levels, true)
	asks := aggregateLevels(asksByPrice, levels, false)

	// A book that would cross is refused: the engine's own snapshot validator
	// rejects a crossed book, and a bot must never reason about one.
	if len(bids) > 0 && len(asks) > 0 {
		bestBid, _ := strconv.ParseFloat(bids[0].Price, 64)
		bestAsk, _ := strconv.ParseFloat(asks[0].Price, 64)
		if bestBid >= bestAsk {
			return nil, errPerpCrossedBook
		}
	}

	return &MarketSnapshotData{
		Symbol:      pair,
		Bids:        bids,
		Asks:        asks,
		VWAP:        strconv.FormatFloat(mark, 'f', -1, 64),
		TimestampMS: time.Now().UTC().UnixMilli(),
	}, nil
}

// aggregateLevels turns a price->size map into the top `limit` levels, best
// first, with each value rendered as an exact decimal string.
func aggregateLevels(byPrice map[float64]float64, limit int, descending bool) []BookLevel {
	prices := make([]float64, 0, len(byPrice))
	for price := range byPrice {
		prices = append(prices, price)
	}
	sort.Float64s(prices)
	if descending {
		for i, j := 0, len(prices)-1; i < j; i, j = i+1, j-1 {
			prices[i], prices[j] = prices[j], prices[i]
		}
	}
	if len(prices) > limit {
		prices = prices[:limit]
	}
	out := make([]BookLevel, 0, len(prices))
	for _, price := range prices {
		out = append(out, BookLevel{
			Price:    strconv.FormatFloat(price, 'f', -1, 64),
			Quantity: strconv.FormatFloat(byPrice[price], 'f', -1, 64),
		})
	}
	return out
}

// MarshalSnapshot renders a snapshot as the JSON the Rust engine parses.
func (s *MarketSnapshotData) MarshalSnapshot() (json.RawMessage, error) {
	return json.Marshal(s)
}
