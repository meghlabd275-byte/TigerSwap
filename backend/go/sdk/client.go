package tigerswap

import (
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
Address   string  `json:"address"`
Symbol    string  `json:"symbol"`
Name      string  `json:"name"`
Decimals  int     `json:"decimals"`
ChainID   int64   `json:"chain_id"`
LogoURI   string  `json:"logo_uri"`
Price     float64 `json:"price"`
IsActive  bool    `json:"is_active"`
IsVerified bool   `json:"is_verified"`
}

// Chain represents a blockchain
type Chain struct {
ChainID    int64  `json:"chain_id"`
Name       string `json:"name"`
Symbol     string `json:"symbol"`
Icon       string `json:"icon"`
RPCURL     string `json:"rpc_url"`
ExplorerURL string `json:"explorer_url"`
Type       string `json:"type"`
IsActive   bool   `json:"is_active"`
IsTestnet  bool   `json:"is_testnet"`
}

// SwapQuote represents a swap quote
type SwapQuote struct {
FromToken   string  `json:"from_token"`
ToToken     string  `json:"to_token"`
FromAmount  string  `json:"from_amount"`
ToAmount    string  `json:"to_amount"`
ToAmountMin string  `json:"to_amount_min"`
Rate        string  `json:"rate"`
PriceImpact string  `json:"price_impact"`
GasEstimate string  `json:"gas_estimate"`
ChainID     int64   `json:"chain_id"`
Route       []string `json:"route"`
}

// Pool represents a liquidity pool
type Pool struct {
ID             uint64 `json:"id"`
TokenAAddress  string `json:"token_a_address"`
TokenBAddress  string `json:"token_b_address"`
ChainID        int64  `json:"chain_id"`
PoolAddress    string `json:"pool_address"`
ReserveA       string `json:"reserve_a"`
ReserveB       string `json:"reserve_b"`
TotalLiquidity string `json:"total_liquidity"`
Fee            int    `json:"fee"`
Volume24h      string `json:"volume_24h"`
}

// MarketStats represents market statistics
type MarketStats struct {
TotalTVL     string `json:"total_tvl"`
Volume24h    string `json:"volume_24h"`
Volume7d     string `json:"volume_7d"`
Fees24h      string `json:"fees_24h"`
UserCount    uint64 `json:"user_count"`
PoolCount    uint64 `json:"pool_count"`
TokenCount   uint64 `json:"token_count"`
ChainCount   uint64 `json:"chain_count"`
}

// Transaction represents a swap transaction
type Transaction struct {
ID         string `json:"id"`
Hash       string `json:"hash"`
ChainID    int64  `json:"chain_id"`
Type       string `json:"type"`
FromToken  string `json:"from_token"`
ToToken    string `json:"to_token"`
FromAmount string `json:"from_amount"`
ToAmount   string `json:"to_amount"`
Status     string `json:"status"`
Timestamp  int64  `json:"timestamp"`
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

var result struct {
Chains []Chain `json:"chains"`
Count  int     `json:"count"`
}
if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
return nil, err
}

return result.Chains, nil
}

// GetTokens returns all tokens for a chain
func (c *Client) GetTokens(ctx context.Context, chainID int64) ([]Token, error) {
url := fmt.Sprintf("%s/api/v1/tokens?chain_id=%d", c.baseURL, chainID)
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

var result struct {
Tokens []Token `json:"tokens"`
Count  int     `json:"count"`
}
if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
return nil, err
}

return result.Tokens, nil
}

// GetQuote returns a swap quote
func (c *Client) GetQuote(ctx context.Context, fromToken, toToken, amount string, chainID int64) (*SwapQuote, error) {
reqBody := map[string]string{
"token_in":  fromToken,
"token_out": toToken,
"amount":    amount,
"chain_id":  fmt.Sprintf("%d", chainID),
}

body, err := json.Marshal(reqBody)
if err != nil {
return nil, err
}

req, err := http.NewRequest("POST", c.baseURL+"/api/v1/swap/quote", nil)
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

var result struct {
Pairs []Pool `json:"pairs"`
}
if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
return nil, err
}

return result.Pairs, nil
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
reqBody := map[string]string{
"token_in":  fromToken,
"token_out": toToken,
"amount":    amount,
"to":        to,
}

body, err := json.Marshal(reqBody)
if err != nil {
return nil, err
}

req, err := http.NewRequest("POST", c.baseURL+"/api/v1/swap/execute", nil)
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
Success     bool         `json:"success"`
Transaction Transaction  `json:"transaction"`
Message     string       `json:"message"`
}
if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
return nil, err
}

return &result.Transaction, nil
}
