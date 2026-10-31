package tigerswap

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Client is the main TigerSwap API client
type Client struct {
	baseURL    string
	httpClient *http.Client
	apiKey     string
}

// NewClient creates a new TigerSwap API client
func NewClient(baseURL string, apiKey string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		apiKey: apiKey,
	}
}

// Token represents a cryptocurrency token
type Token struct {
	Address  string   `json:"address"`
	Symbol   string   `json:"symbol"`
	Name     string   `json:"name"`
	Decimals int      `json:"decimals"`
	ChainID  int64    `json:"chainId"`
	LogoURI  *string  `json:"logoURI"`
	PriceUSD *float64 `json:"priceUSD"`
}

// Chain represents a blockchain
type Chain struct {
	ID           int64   `json:"id"`
	Name         string  `json:"name"`
	ChainType    string  `json:"chainType"`
	NativeSymbol string  `json:"nativeSymbol"`
	ExplorerURL  *string `json:"explorerURL"`
	IsActive     bool    `json:"isActive"`
}

// SwapQuote represents a swap quote
type SwapQuote struct {
	AmountIn     string   `json:"amountIn"`
	AmountOut    string   `json:"amountOut"`
	AmountOutMin string   `json:"amountOutMin"`
	PriceImpact  float64  `json:"priceImpact"`
	Route        []string `json:"route"`
	GasEstimate  uint64   `json:"gasEstimate"`
}

// Pool represents a liquidity pool
type Pool struct {
	PairAddress string  `json:"pairAddress"`
	Token0      string  `json:"token0"`
	Token1      string  `json:"token1"`
	Symbol0     string  `json:"symbol0"`
	Symbol1     string  `json:"symbol1"`
	Reserve0    float64 `json:"reserve0"`
	Reserve1    float64 `json:"reserve1"`
	FeeBps      int     `json:"feeBps"`
}

// MarketStats represents market statistics
type MarketStats struct {
	TotalTVL   float64 `json:"totalTVL"`
	Volume24h  float64 `json:"volume24h"`
	TotalPairs int64   `json:"totalPairs"`
	TotalUsers int64   `json:"totalUsers"`
	UpdatedAt  *string `json:"updatedAt"`
}

// Transaction represents a swap transaction
type Transaction struct {
	ID        string `json:"id"`
	Hash      string `json:"hash"`
	ChainID   int64  `json:"chain_id"`
	Type      string `json:"type"`
	FromToken string `json:"from_token"`
	ToToken   string `json:"to_token"`
	Status    string `json:"status"`
	Timestamp string `json:"timestamp"`
}

// GetChains returns all supported chains
func (c *Client) GetChains(ctx context.Context) ([]Chain, error) {
	req, err := http.NewRequest("GET", c.baseURL+"/api/v1/chains", nil)
	if err != nil {
		return nil, err
	}

	req = req.WithContext(ctx)
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var chains []Chain
	if err := json.NewDecoder(resp.Body).Decode(&chains); err != nil {
		return nil, err
	}

	return chains, nil
}

// GetTokens returns all tokens for a chain
func (c *Client) GetTokens(ctx context.Context, chainID int64) ([]Token, error) {
	url := fmt.Sprintf("%s/api/v1/tokens?chainId=%d", c.baseURL, chainID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req = req.WithContext(ctx)
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var tokens []Token
	if err := json.NewDecoder(resp.Body).Decode(&tokens); err != nil {
		return nil, err
	}

	return tokens, nil
}

// GetQuote returns a swap quote
func (c *Client) GetQuote(ctx context.Context, fromToken, toToken, amount string, chainID int64) (*SwapQuote, error) {
	reqBody := map[string]interface{}{
		"token_in":  fromToken,
		"token_out": toToken,
		"amount_in": amount,
		"chain_id":  chainID,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", c.baseURL+"/api/v1/swap/quote", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req = req.WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var quote SwapQuote
	if err := json.NewDecoder(resp.Body).Decode(&quote); err != nil {
		return nil, err
	}

	return &quote, nil
}

// GetPools returns all pools
func (c *Client) GetPools(ctx context.Context) ([]Pool, error) {
	req, err := http.NewRequest("GET", c.baseURL+"/api/v1/swap/pairs", nil)
	if err != nil {
		return nil, err
	}

	req = req.WithContext(ctx)
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var pools []Pool
	if err := json.NewDecoder(resp.Body).Decode(&pools); err != nil {
		return nil, err
	}

	return pools, nil
}

// GetMarketStats returns market statistics
func (c *Client) GetMarketStats(ctx context.Context) (*MarketStats, error) {
	req, err := http.NewRequest("GET", c.baseURL+"/api/v1/market/stats", nil)
	if err != nil {
		return nil, err
	}

	req = req.WithContext(ctx)
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var stats MarketStats
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		return nil, err
	}

	return &stats, nil
}

// ExecuteSwap executes a swap transaction
func (c *Client) ExecuteSwap(ctx context.Context, fromToken, toToken, amount, to string) (*Transaction, error) {
	reqBody := map[string]interface{}{
		"token_in":  fromToken,
		"token_out": toToken,
		"amount_in": amount,
		"recipient": to,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", c.baseURL+"/api/v1/swap/execute", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req = req.WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Success     bool        `json:"success"`
		Transaction Transaction `json:"transaction"`
		Message     string      `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result.Transaction, nil
}
