package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// Market represents a trading market
type Market struct {
	ID            string  `json:"id"`
	BaseAsset     string  `json:"base_asset"`
	QuoteAsset    string  `json:"quote_asset"`
	Price         string  `json:"price"`
	PriceChange24h string `json:"price_change_24h"`
	Volume24h     string  `json:"volume_24h"`
	High24h       string  `json:"high_24h"`
	Low24h        string  `json:"low_24h"`
	Turnover24h   string  `json:"turnover_24h"`
	Status        string  `json:"status"`
	CreatedAt     int64   `json:"created_at"`
}

// OrderBook represents order book data
type OrderBook struct {
	MarketID     string     `json:"market_id"`
	Bids         [][]string `json:"bids"`
	Asks         [][]string `json:"asks"`
	LastUpdateID int64      `json:"last_update_id"`
}

// Trade represents a trade
type Trade struct {
	ID        string `json:"id"`
	MarketID  string `json:"market_id"`
	Price     string `json:"price"`
	Quantity  string `json:"quantity"`
	Side      string `json:"side"`
	Time      int64  `json:"time"`
	MakerFee  string `json:"maker_fee"`
	TakerFee  string `json:"taker_fee"`
}

// Ticker represents price ticker
type Ticker struct {
	Symbol        string `json:"symbol"`
	Price         string `json:"price"`
	PriceChange   string `json:"price_change"`
	PriceChange24 string `json:"price_change_24h"`
	Volume24h     string `json:"volume_24h"`
	High24h       string `json:"high_24h"`
	Low24h        string `json:"low_24h"`
}

// ListMarkets returns list of available markets
// @Summary List Markets
// @Description Get all available trading markets
// @Tags market
// @Accept json
// @Produce json
// @Success 200 {array} Market
// @Router /api/v1/markets [get]
func ListMarkets(c *gin.Context) {
	markets := []Market{
		{
			ID:            "ETH-USDT",
			BaseAsset:     "ETH",
			QuoteAsset:    "USDT",
			Price:         "3250.00",
			PriceChange24h: "2.5%",
			Volume24h:     "1.2B",
			High24h:       "3300.00",
			Low24h:        "3200.00",
			Turnover24h:   "3.9B",
			Status:        "trading",
			CreatedAt:     1609459200,
		},
		{
			ID:            "BTC-USDT",
			BaseAsset:     "BTC",
			QuoteAsset:    "USDT",
			Price:         "67500.00",
			PriceChange24h: "1.8%",
			Volume24h:     "5.8B",
			High24h:       "68000.00",
			Low24h:        "66500.00",
			Turnover24h:   "12.5B",
			Status:        "trading",
			CreatedAt:     1609459200,
		},
		{
			ID:            "SOL-USDT",
			BaseAsset:     "SOL",
			QuoteAsset:    "USDT",
			Price:         "145.00",
			PriceChange24h: "5.2%",
			Volume24h:     "800M",
			High24h:       "150.00",
			Low24h:        "138.00",
			Turnover24h:   "580M",
			Status:        "trading",
			CreatedAt:     1609459200,
		},
	}

	c.JSON(http.StatusOK, markets)
}

// GetMarket returns market details
// @Summary Get Market
// @Description Get details of a specific market
// @Tags market
// @Accept json
// @Produce json
// @Param id path string true "Market ID"
// @Success 200 {object} Market
// @Router /api/v1/markets/{id} [get]
func GetMarket(c *gin.Context) {
	marketID := c.Param("id")

	market := Market{
		ID:            marketID,
		BaseAsset:     "ETH",
		QuoteAsset:    "USDT",
		Price:         "3250.00",
		PriceChange24h: "2.5%",
		Volume24h:     "1.2B",
		High24h:       "3300.00",
		Low24h:        "3200.00",
		Turnover24h:   "3.9B",
		Status:        "trading",
		CreatedAt:     1609459200,
	}

	c.JSON(http.StatusOK, market)
}

// GetTickers returns ticker information
// @Summary Get Tickers
// @Description Get price tickers for all markets
// @Tags market
// @Accept json
// @Produce json
// @Success 200 {array} Ticker
// @Router /api/v1/tickers [get]
func GetTickers(c *gin.Context) {
	tickers := []Ticker{
		{
			Symbol:        "ETH-USDT",
			Price:         "3250.00",
			PriceChange:   "2.5%",
			PriceChange24: "2.5%",
			Volume24h:     "1.2B",
			High24h:       "3300.00",
			Low24h:        "3200.00",
		},
		{
			Symbol:        "BTC-USDT",
			Price:         "67500.00",
			PriceChange:   "1.8%",
			PriceChange24: "1.8%",
			Volume24h:     "5.8B",
			High24h:       "68000.00",
			Low24h:        "66500.00",
		},
	}

	c.JSON(http.StatusOK, tickers)
}

// GetOrderBook returns order book for a market
// @Summary Get Order Book
// @Description Get order book depth for a market
// @Tags market
// @Accept json
// @Produce json
// @Param market path string true "Market ID"
// @Param limit query int false "Limit (default 100)"
// @Success 200 {object} OrderBook
// @Router /api/v1/orderbook/{market} [get]
func GetOrderBook(c *gin.Context) {
	market := c.Param("market")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))

	orderBook := OrderBook{
		MarketID:     market,
		LastUpdateID: time.Now().Unix(),
		Bids: [][]string{
			{"3249.50", "100.5"},
			{"3249.00", "200.0"},
			{"3248.50", "150.0"},
		},
		Asks: [][]string{
			{"3250.50", "80.0"},
			{"3251.00", "250.0"},
			{"3251.50", "180.0"},
		},
	}

	c.JSON(http.StatusOK, orderBook)
}

// GetRecentTrades returns recent trades
// @Summary Get Recent Trades
// @Description Get recent trades for a market
// @Tags market
// @Accept json
// @Produce json
// @Param market path string true "Market ID"
// @Param limit query int false "Limit (default 100)"
// @Success 200 {array} Trade
// @Router /api/v1/trades/{market} [get]
func GetRecentTrades(c *gin.Context) {
	market := c.Param("market")

	trades := []Trade{
		{
			ID:        "trade_1",
			MarketID:  market,
			Price:     "3250.00",
			Quantity:  "1.5",
			Side:      "buy",
			Time:      time.Now().Unix() - 60,
			MakerFee:  "0.001",
			TakerFee:  "0.001",
		},
		{
			ID:        "trade_2",
			MarketID:  market,
			Price:     "3249.50",
			Quantity:  "2.0",
			Side:      "sell",
			Time:      time.Now().Unix() - 30,
			MakerFee:  "0.001",
			TakerFee:  "0.001",
		},
	}

	c.JSON(http.StatusOK, trades)
}

// GetKLines returns kline/candlestick data
// @Summary Get K-Lines
// @Description Get candlestick/kline data
// @Tags market
// @Accept json
// @Produce json
// @Param market query string true "Market ID"
// @Param interval query string false "Interval (1m, 5m, 15m, 1h, 4h, 1d)"
// @Param limit query int false "Limit"
// @Success 200 {array} []string
// @Router /api/v1/klines [get]
func GetKLines(c *gin.Context) {
	market := c.Query("market")
	interval := c.DefaultQuery("interval", "1h")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))

	// Return mock kline data
	klines := [][]interface{}{
		{time.Now().Unix() - 3600, "3200", "3250", "3220", "3240", "1000"},
		{time.Now().Unix() - 1800, "3240", "3260", "3235", "3250", "1500"},
		{time.Now().Unix(), "3250", "3260", "3245", "3250", "1200"},
	}

	c.JSON(http.StatusOK, klines)
}

// ListTokens returns list of supported tokens
// @Summary List Tokens
// @Description Get all supported trading tokens
// @Tags market
// @Accept json
// @Produce json
// @Success 200 {array} map[string]interface{}
// @Router /api/v1/tokens [get]
func ListTokens(c *gin.Context) {
	tokens := []map[string]interface{}{
		{
			"address":  "0x0000000000000000000000000000000000000000",
			"symbol":   "ETH",
			"name":     "Ethereum",
			"decimals": 18,
			"logo":     "https://assets.coingecko.com/coins/images/279/small/ethereum.png",
		},
		{
			"address":  "0xdAC17F958D2ee523a2206206994597C13D831ec7",
			"symbol":   "USDT",
			"name":     "Tether USD",
			"decimals": 6,
			"logo":     "https://assets.coingecko.com/coins/images/325/small/Tether.png",
		},
		{
			"address":  "0x2260FAC5E5542a773Aa44fBCfeDf7C193bc2C599",
			"symbol":   "WBTC",
			"wrappedBitcoin",
			"name":     "Wrapped Bitcoin",
			"decimals": 8,
			"logo":     "https://assets.coingecko.com/coins/images/7598/small/wrapped_bitcoin_wbtc.png",
		},
	}

	c.JSON(http.StatusOK, tokens)
}

// GetFeeInfo returns fee information
// @Summary Get Fee Info
// @Description Get trading fee information
// @Tags market
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/fee [get]
func GetFeeInfo(c *gin.Context) {
	c.JSON(http.StatusOK, map[string]interface{}{
		"taker_fee":  "0.003",
		"maker_fee":  "0.001",
		"volume_tiers": []map[string]interface{}{
			{"volume": "0-10000", "taker_fee": "0.003", "maker_fee": "0.001"},
			{"volume": "10000-100000", "taker_fee": "0.002", "maker_fee": "0.0008"},
			{"volume": "100000+", "taker_fee": "0.001", "maker_fee": "0.0005"},
		},
	})
}
