// Package tigerswap provides Go SDK for TigerSwap DEX
package tigerswap

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Config holds SDK configuration
type Config struct {
	APIKey       string
	BaseURL      string
	Timeout     time.Duration
	MaxRetries   int
}

// Client is TigerSwap API client
type Client struct {
	config     Config
	httpClient *http.Client
}

// NewClient creates new TigerSwap client
func NewClient(apiKey string, baseURL string) *Client {
	config := Config{
		APIKey:     apiKey,
		BaseURL:    baseURL,
		Timeout:   30 * time.Second,
		MaxRetries: 3,
	}
	
	return &Client{
		config: config,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

// Token pair representation
type TokenPair struct {
	Symbol      string `json:"symbol"`
	Address    string `json:"address"`
	Decimals   int    `json:"decimals"`
	ChainID    int    `json:"chain_id"`
	Name      string `json:"name"`
	LogoURL   string `json:"logo_url"`
}

// Price quote response
type Quote struct {
	FromToken  string  `json:"from_token"`
	ToToken   string  `json:"to_token"`
	FromAmount string `json:"from_amount"`
	ToAmount  string  `json:"to_amount"`
	Price    string `json:"price"`
	Slippage string `json:"slippage"`
	Gas     string `json:"gas"`
}

// Swap request
type SwapRequest struct {
	FromToken   string `json:"from_token"`
	ToToken    string `json:"to_token"`
	Amount    string `json:"amount"`
	Slippage  int    `json:"slippage"` // basis points
	To        string `json:"to"`
}

// Swap response
type SwapResponse struct {
	TxHash    string `json:"tx_hash"`
	FromToken string `json:"from_token"`
	ToToken  string `json:"to_token"`
	FromAmount string `json:"from_amount"`
	ToAmount string `json:"to_amount"`
	Status   string `json:"status"`
}

// Order representation
type Order struct {
	ID            string `json:"id"`
	User          string `json:"user"`
	Pair          string `json:"pair"`
	Side         string `json:"side"` // "buy" or "sell"
	Type         string `json:"type"` // "limit", "market", "stop_loss", "take_profit"
	Price        string `json:"price"`
	TriggerPrice string `json:"trigger_price,omitempty"`
	Quantity    string `json:"quantity"`
	Filled       string `json:"filled"`
	Status      string `json:"status"` // "open", "filled", "cancelled"
	CreatedAt    int64  `json:"created_at"`
	ExpiresAt   int64  `json:"expires_at"`
}

// Position for perpetuals
type Position struct {
	ID          string `json:"id"`
	User        string `json:"user"`
	Pair        string `json:"pair"`
	Side       string `json:"side"` // "long" or "short"
	Size        string `json:"size"`
	Collateral  string `json:"collateral"`
	Leverage   string `json:"leverage"`
	EntryPrice string `json:"entry_price"`
	MarkPrice string `json:"mark_price"`
	PnL        string `json:"pnl"`
	Status     string `json:"status"` // "open", "closed", "liquidated"
}

// Portfolio holdings
type Portfolio struct {
	User        string         `json:"user"`
	Tokens     []TokenBalance `json:"tokens"`
	TotalValue string        `json:"total_value"`
	UpdatedAt int64        `json:"updated_at"`
}

// Token balance
type TokenBalance struct {
	Token      string `json:"token"`
	Balance   string `json:"balance"`
	ValueUSD  string `json:"value_usd"`
}

// Error response
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// APIError wraps API errors
type APIError struct {
	Code    int
	Message string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("API error %d: %s", e.Code, e.Message)
}

// Make request to API
func (c *Client) request(ctx context.Context, method, path string, body interface{}, result interface{}) error {
	var bodyReader io.Reader
	if body != nil {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return err
		}
		bodyReader = strings.NewReader(string(bodyBytes))
	}
	
	req, err := http.NewRequestWithContext(ctx, method, c.config.BaseURL+path, bodyReader)
	if err != nil {
		return err
	}
	
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.config.APIKey)
	req.Header.Set("User-Agent", "TigerSwap-Go-SDK/1.0")
	
	var lastErr error
	for i := 0; i < c.config.MaxRetries; i++ {
		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			time.Sleep(time.Duration(i+1) * time.Second)
			continue
		}
		defer resp.Body.Close()
		
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			if result != nil {
				return json.NewDecoder(resp.Body).Decode(result)
			}
			return nil
		}
		
		var apiErr Error
		json.NewDecoder(resp.Body).Decode(&apiErr)
		lastErr = &APIError{Code: resp.StatusCode, Message: apiErr.Message}
		
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			return lastErr
		}
		
		time.Sleep(time.Duration(i+1) * time.Second)
	}
	
	return lastErr
}

// GetQuote returns swap quote
func (c *Client) GetQuote(ctx context.Context, fromToken, toToken, amount string) (*Quote, error) {
	path := fmt.Sprintf("/v1/quote?from=%s&to=%s&amount=%s", fromToken, toToken, amount)
	
	var quote Quote
	err := c.request(ctx, "GET", path, nil, &quote)
	if err != nil {
		return nil, err
	}
	
	return &quote, nil
}

// ExecuteSwap executes a swap
func (c *Client) ExecuteSwap(ctx context.Context, req *SwapRequest) (*SwapResponse, error) {
	var resp SwapResponse
	err := c.request(ctx, "POST", "/v1/swap", req, &resp)
	if err != nil {
		return nil, err
	}
	
	return &resp, nil
}

// GetTokens returns list of supported tokens
func (c *Client) GetTokens(ctx context.Context, chainID int) ([]TokenPair, error) {
	path := fmt.Sprintf("/v1/tokens?chain_id=%d", chainID)
	
	var tokens []TokenPair
	err := c.request(ctx, "GET", path, nil, &tokens)
	if err != nil {
		return nil, err
	}
	
	return tokens, nil
}

// GetOrders returns user's orders
func (c *Client) GetOrders(ctx context.Context, user string, pair string) ([]Order, error) {
	path := fmt.Sprintf("/v1/orders?user=%s", user)
	if pair != "" {
		path += "&pair=" + pair
	}
	
	var orders []Order
	err := c.request(ctx, "GET", path, nil, &orders)
	if err != nil {
		return nil, err
	}
	
	return orders, nil
}

// CreateOrder creates a new order
func (c *Client) CreateOrder(ctx context.Context, order *Order) (*Order, error) {
	var created Order
	err := c.request(ctx, "POST", "/v1/orders", order, &created)
	if err != nil {
		return nil, err
	}
	
	return &created, nil
}

// CancelOrder cancels an order
func (c *Client) CancelOrder(ctx context.Context, orderID string) error {
	path := fmt.Sprintf("/v1/orders/%s", orderID)
	
	err := c.request(ctx, "DELETE", path, nil, nil)
	return err
}

// GetPositions returns user's positions
func (c *Client) GetPositions(ctx context.Context, user string) ([]Position, error) {
	path := fmt.Sprintf("/v1/positions?user=%s", user)
	
	var positions []Position
	err := c.request(ctx, "GET", path, nil, &positions)
	if err != nil {
		return nil, err
	}
	
	return positions, nil
}

// GetPortfolio returns user's portfolio
func (c *Client) GetPortfolio(ctx context.Context, user string) (*Portfolio, error) {
	path := fmt.Sprintf("/v1/portfolio?user=%s", user)
	
	var portfolio Portfolio
	err := c.request(ctx, "GET", path, nil, &portfolio)
	if err != nil {
		return nil, err
	}
	
	return &portfolio, nil
}

// GetPrice returns price for a token pair
func (c *Client) GetPrice(ctx context.Context, fromToken, toToken string) (string, error) {
	path := fmt.Sprintf("/v1/price?from=%s&to=%s", fromToken, toToken)
	
	var result struct {
		Price string `json:"price"`
	}
	
	err := c.request(ctx, "GET", path, nil, &result)
	if err != nil {
		return "", err
	}
	
	return result.Price, nil
}

// GetMarketData returns market data
func (c *Client) GetMarketData(ctx context.Context, pair string) (map[string]interface{}, error) {
	path := fmt.Sprintf("/v1/market/%s", pair)
	
	var result map[string]interface{}
	err := c.request(ctx, "GET", path, nil, &result)
	if err != nil {
		return nil, err
	}
	
	return result, nil
}