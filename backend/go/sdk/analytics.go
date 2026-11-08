package tigerswap

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

// ProtocolOverview is the protocol dashboard snapshot from
// `GET /api/v1/analytics/overview`. Every field is a real count or sum
// computed from rows TigerSwap wrote; a value the server cannot compute comes
// back as zero rather than a guess.
type ProtocolOverview struct {
	TotalTVLUSD       float64 `json:"total_tvl_usd"`
	Volume24hUSD      float64 `json:"volume_24h_usd"`
	Fees24hUSD        float64 `json:"fees_24h_usd"`
	Volume7dUSD       float64 `json:"volume_7d_usd"`
	Fees7dUSD         float64 `json:"fees_7d_usd"`
	TotalPools        int64   `json:"total_pools"`
	ActivePools       int64   `json:"active_pools"`
	TotalFarms        int64   `json:"total_farms"`
	ActiveFarms       int64   `json:"active_farms"`
	TotalTokens       int64   `json:"total_tokens"`
	TotalChains       int64   `json:"total_chains"`
	ActiveChains      int64   `json:"active_chains"`
	TotalUsers        int64   `json:"total_users"`
	TotalTrades24h    int64   `json:"total_trades_24h"`
	OpenInterestUSD   float64 `json:"open_interest_usd"`
	MarginBorrowedUSD float64 `json:"margin_borrowed_usd"`
	NFTListingsActive int64   `json:"nft_listings_active"`
	UpdatedAt         string  `json:"updated_at"`
}

// VolumePoint is one point of a volume series.
type VolumePoint struct {
	Timestamp          string  `json:"timestamp"`
	VolumeUSD          float64 `json:"volume_usd"`
	FeesUSD            float64 `json:"fees_usd"`
	TradeCount         int64   `json:"trade_count"`
	TakerBuyVolumeUSD  float64 `json:"taker_buy_volume_usd"`
	TakerSellVolumeUSD float64 `json:"taker_sell_volume_usd"`
}

// VolumeSeries is the envelope returned by `GET /api/v1/analytics/volume`.
type VolumeSeries struct {
	Pair     string        `json:"pair"`
	ChainID  int64         `json:"chain_id"`
	Interval int           `json:"interval"`
	Series   []VolumePoint `json:"series"`
}

// PairAnalytics is per-market activity over a window.
type PairAnalytics struct {
	Pair               string  `json:"pair"`
	ChainID            int64   `json:"chain_id"`
	VolumeUSD          float64 `json:"volume_usd"`
	FeesUSD            float64 `json:"fees_usd"`
	TradeCount         int64   `json:"trade_count"`
	TakerBuyVolumeUSD  float64 `json:"taker_buy_volume_usd"`
	TakerSellVolumeUSD float64 `json:"taker_sell_volume_usd"`
}

// TopTrader is one row of the realized-PnL leaderboard.
type TopTrader struct {
	UserID       string  `json:"user_id"`
	RealizedPnL  float64 `json:"realized_pnl"`
	VolumeUSD    float64 `json:"volume_usd"`
	ClosedTrades int64   `json:"closed_trades"`
	WinRate      float64 `json:"win_rate"`
	Liquidations int64   `json:"liquidations"`
}

// analyticsGet issues an authenticated GET and decodes the JSON body into out.
// A non-200 response is an error carrying the status, never a zero value that
// would read as "no data".
func (c *Client) analyticsGet(ctx context.Context, endpoint string, out interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: unexpected status %d", endpoint, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// analyticsEndpoint builds a URL with the non-zero query values set.
func (c *Client) analyticsEndpoint(path string, query url.Values) string {
	endpoint := c.baseURL + path
	if encoded := query.Encode(); encoded != "" {
		endpoint += "?" + encoded
	}
	return endpoint
}

// GetAnalyticsOverview returns the protocol dashboard snapshot.
func (c *Client) GetAnalyticsOverview(ctx context.Context) (*ProtocolOverview, error) {
	var overview ProtocolOverview
	if err := c.analyticsGet(ctx, c.baseURL+"/api/v1/analytics/overview", &overview); err != nil {
		return nil, err
	}
	return &overview, nil
}

// GetAnalyticsVolume returns a volume series. `interval` and `window` are in
// seconds; `interval` must be a whole multiple of the server's rollup bucket.
// A zero interval or window leaves the server default in place.
func (c *Client) GetAnalyticsVolume(ctx context.Context, pair string, chainID int64, interval, window int) (*VolumeSeries, error) {
	query := url.Values{}
	if pair != "" {
		query.Set("pair", pair)
	}
	if chainID != 0 {
		query.Set("chain_id", strconv.FormatInt(chainID, 10))
	}
	if interval > 0 {
		query.Set("interval", strconv.Itoa(interval))
	}
	if window > 0 {
		query.Set("window", strconv.Itoa(window))
	}

	var series VolumeSeries
	if err := c.analyticsGet(ctx, c.analyticsEndpoint("/api/v1/analytics/volume", query), &series); err != nil {
		return nil, err
	}
	return &series, nil
}

// GetAnalyticsPairs returns the per-market volume leaderboard over a window.
func (c *Client) GetAnalyticsPairs(ctx context.Context, chainID int64, window, limit int) ([]PairAnalytics, error) {
	query := url.Values{}
	if chainID != 0 {
		query.Set("chain_id", strconv.FormatInt(chainID, 10))
	}
	if window > 0 {
		query.Set("window", strconv.Itoa(window))
	}
	if limit > 0 {
		query.Set("limit", strconv.Itoa(limit))
	}

	var envelope struct {
		Pairs []PairAnalytics `json:"pairs"`
	}
	if err := c.analyticsGet(ctx, c.analyticsEndpoint("/api/v1/analytics/pairs", query), &envelope); err != nil {
		return nil, err
	}
	return envelope.Pairs, nil
}

// GetAnalyticsTraders returns the realized-PnL leaderboard. `window` is in
// seconds; 0 means all time.
func (c *Client) GetAnalyticsTraders(ctx context.Context, window, limit int) ([]TopTrader, error) {
	query := url.Values{}
	if window > 0 {
		query.Set("window", strconv.Itoa(window))
	}
	if limit > 0 {
		query.Set("limit", strconv.Itoa(limit))
	}

	var envelope struct {
		Traders []TopTrader `json:"traders"`
	}
	if err := c.analyticsGet(ctx, c.analyticsEndpoint("/api/v1/analytics/traders", query), &envelope); err != nil {
		return nil, err
	}
	return envelope.Traders, nil
}
