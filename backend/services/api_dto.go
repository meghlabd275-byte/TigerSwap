package services

import (
	"math"
	"math/big"
	"strconv"
	"strings"
	"time"

	"tigerswap/backend/models"
)

// This file defines the JSON shapes served to the Next.js frontend.
//
// The frontend validates every response against a strict schema before it
// reaches a component, so these structs are a contract, not a convenience. Two
// rules follow from that and are applied consistently below:
//
//   - Envelope: the frontend's list validators require a bare JSON array for
//     /tokens, /chains, /swap/pairs, /market/trades and /prices. Returning an
//     object wrapper makes the validator throw and the page render nothing.
//   - Field names: camelCase, matching the interfaces in frontend/src/lib/types.ts.
//
// Keys that the frontend declares as nullable must still be present; a missing
// key is a validation failure, while an explicit null is accepted.

// chainDTO is the `/chains` element. The frontend uses `id` as the EVM chain ID
// (it is passed straight to the wallet's switch-chain call), so `id` carries the
// chain ID rather than the database primary key.
type chainDTO struct {
	ID           int64   `json:"id"`
	Name         string  `json:"name"`
	ChainType    string  `json:"chainType"`
	NativeSymbol string  `json:"nativeSymbol"`
	ExplorerURL  *string `json:"explorerURL"`
	IsActive     bool    `json:"isActive"`
}

func newChainDTO(m models.Blockchain) chainDTO {
	return chainDTO{
		ID:           m.ChainID,
		Name:         m.Name,
		ChainType:    m.Type,
		NativeSymbol: m.Symbol,
		ExplorerURL:  optionalString(m.ExplorerURL),
		IsActive:     m.IsActive,
	}
}

func newChainDTOs(rows []models.Blockchain) []chainDTO {
	out := make([]chainDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, newChainDTO(row))
	}
	return out
}

// tokenDTO is the `/tokens` element.
type tokenDTO struct {
	Address  string   `json:"address"`
	Symbol   string   `json:"symbol"`
	Name     string   `json:"name"`
	Decimals int      `json:"decimals"`
	ChainID  int64    `json:"chainId"`
	LogoURI  *string  `json:"logoURI"`
	PriceUSD *float64 `json:"priceUSD"`
}

func newTokenDTO(m models.Token) tokenDTO {
	return tokenDTO{
		Address:  m.Address,
		Symbol:   m.Symbol,
		Name:     m.Name,
		Decimals: m.Decimals,
		ChainID:  m.ChainID,
		LogoURI:  optionalString(m.LogoURI),
		PriceUSD: positiveOrNil(m.Price),
	}
}

func newTokenDTOs(rows []models.Token) []tokenDTO {
	out := make([]tokenDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, newTokenDTO(row))
	}
	return out
}

// marketStatsDTO is the `/market/stats` body. The frontend renders totalTVL and
// volume24h as USD figures and totalPairs/totalUsers as counts.
type marketStatsDTO struct {
	TotalTVL   float64 `json:"totalTVL"`
	Volume24h  float64 `json:"volume24h"`
	TotalPairs int64   `json:"totalPairs"`
	TotalUsers int64   `json:"totalUsers"`
	UpdatedAt  *string `json:"updatedAt"`
}

// tradeDTO is the `/market/trades` element.
//
// The stored transaction amounts are integer base units at the token's own
// decimals, while the frontend expects a human-readable price and quantity. Both
// are therefore scaled by the tokens' decimals, which are read from the tokens
// table - not assumed to be 18, since USDC and USDT use 6.
type tradeDTO struct {
	ID        string  `json:"id"`
	Pair      string  `json:"pair"`
	Side      string  `json:"side"`
	Price     float64 `json:"price"`
	Quantity  float64 `json:"quantity"`
	Timestamp string  `json:"timestamp"`
}

// poolDTO is the `/swap/pairs` element. `feeBps` is the pool's fee in basis
// points, which is how the value is stored.
type poolDTO struct {
	PairAddress string  `json:"pairAddress"`
	Token0      string  `json:"token0"`
	Token1      string  `json:"token1"`
	Symbol0     string  `json:"symbol0"`
	Symbol1     string  `json:"symbol1"`
	Reserve0    float64 `json:"reserve0"`
	Reserve1    float64 `json:"reserve1"`
	FeeBps      int     `json:"feeBps"`
}

// priceDTO is the `/prices` element. `/prices/:symbol` returns the same shape
// wrapped in a one-element array by the caller.
type priceDTO struct {
	Symbol    string  `json:"symbol"`
	PriceUSD  float64 `json:"priceUSD"`
	Change24h float64 `json:"change24h"`
}

// swapQuoteDTO is the `/swap/quote` body. Amounts stay decimal strings so that
// integer base units survive JSON without precision loss, which a float64 would
// not preserve for 18-decimal tokens.
type swapQuoteDTO struct {
	AmountIn     string   `json:"amountIn"`
	AmountOut    string   `json:"amountOut"`
	AmountOutMin string   `json:"amountOutMin"`
	PriceImpact  float64  `json:"priceImpact"`
	Route        []string `json:"route"`
	GasEstimate  int64    `json:"gasEstimate"`
}

// buildTxDTO is the `/swap/build` body, ready for the caller's wallet to sign.
type buildTxDTO struct {
	To       string  `json:"to"`
	Data     string  `json:"data"`
	Value    string  `json:"value"`
	GasLimit *string `json:"gas_limit"`
}

// adminDashboardDTO is the `/admin/dashboard` body.
type adminDashboardDTO struct {
	TotalUsers        int64   `json:"totalUsers"`
	TotalTransactions int64   `json:"totalTransactions"`
	TotalVolume       float64 `json:"totalVolume"`
	TotalRevenue      float64 `json:"totalRevenue"`
	ActivePools       int64   `json:"activePools"`
	UpdatedAt         *string `json:"updatedAt"`
}

// ------------------------------------------------------------------
// helpers
// ------------------------------------------------------------------

// optionalString maps an empty string to null. The frontend's nullable(string)
// accepts either, but an explicit null is clearer than "" for "not set".
func optionalString(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

// positiveOrNil maps a non-positive price to null, which the frontend's
// nullable(nonNegative) accepts. A stored zero means "no price known", not "this
// asset is worthless", and reporting it as 0 would render a misleading $0.
func positiveOrNil(value float64) *float64 {
	if value <= 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		return nil
	}
	return &value
}

// timestampPtr renders a time as an ISO-8601 string, or null when unset. The
// frontend's timestamp validator parses it with Date.parse.
func timestampPtr(t time.Time) *string {
	if t.IsZero() {
		return nil
	}
	formatted := t.UTC().Format(time.RFC3339)
	return &formatted
}

// timeString renders a time as ISO-8601. Used where the field is required
// rather than nullable.
func timeString(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

// scaleToFloat converts an integer base-unit amount into a decimal figure using
// the token's own decimals. It returns 0 for input that is not a valid integer,
// which callers treat as "no value".
func scaleToFloat(raw string, decimals int) float64 {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0
	}
	value, ok := new(big.Float).SetString(trimmed)
	if !ok {
		return 0
	}
	divisor := new(big.Float).SetInt(pow10Float(decimals))
	scaled := new(big.Float).Quo(value, divisor)
	result, _ := scaled.Float64()
	if math.IsNaN(result) || math.IsInf(result, 0) {
		return 0
	}
	return result
}

// pow10Float returns 10^n as a big.Int for use as a scaling divisor.
func pow10Float(n int) *big.Int {
	if n < 0 {
		n = 0
	}
	return new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(n)), nil)
}

// parseBaseUnits parses a decimal string into integer base units at the given
// decimals, rejecting anything that is not a non-negative integer.
func parseBaseUnits(raw string, decimals int) (*big.Int, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, false
	}
	value, ok := new(big.Int).SetString(trimmed, 10)
	if !ok || value.Sign() < 0 {
		return nil, false
	}
	return value, true
}

// itoa is a small helper so handlers do not repeat strconv calls.
func itoa(v int64) string {
	return strconv.FormatInt(v, 10)
}
