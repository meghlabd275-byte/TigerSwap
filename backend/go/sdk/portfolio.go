package tigerswap

import (
	"context"
	"fmt"
)

// PortfolioAsset is one holding in an account portfolio, from
// `GET /api/v1/user/portfolio`. `Balance` is the exact ledger amount in the
// token's smallest unit as a decimal string; `ValueUSD` is nil when the server
// had no oracle price for the token, which is different from a holding worth
// zero.
type PortfolioAsset struct {
	ChainID      int64    `json:"chainId"`
	TokenAddress string   `json:"tokenAddress"`
	Symbol       string   `json:"symbol"`
	Decimals     int      `json:"decimals"`
	Balance      string   `json:"balance"`
	ValueUSD     *float64 `json:"valueUSD"`
}

// Portfolio is the account portfolio snapshot. Realized and unrealized PnL are
// USD amounts that may be negative; a book with no positions reports zero.
type Portfolio struct {
	TotalValueUSD    float64          `json:"totalValueUSD"`
	Assets           []PortfolioAsset `json:"assets"`
	RealizedPnLUSD   float64          `json:"realizedPnlUsd"`
	UnrealizedPnLUSD float64          `json:"unrealizedPnlUsd"`
	OpenPositions    int              `json:"openPositions"`
	UpdatedAt        string           `json:"updatedAt"`
}

// UserTransaction is one row from `GET /api/v1/user/transactions`.
type UserTransaction struct {
	Hash        string `json:"hash"`
	ChainID     int64  `json:"chainId"`
	Type        string `json:"type"`
	Status      string `json:"status"`
	Amount      string `json:"amount"`
	TokenSymbol string `json:"tokenSymbol"`
	CreatedAt   string `json:"createdAt"`
}

// GetPortfolio returns the authenticated account's portfolio. The bearer token
// set on the client is the caller's session, so the server scopes the response
// to that account and never to a caller-supplied id.
func (c *Client) GetPortfolio(ctx context.Context) (*Portfolio, error) {
	var portfolio Portfolio
	if err := c.analyticsGet(ctx, c.baseURL+"/api/v1/user/portfolio", &portfolio); err != nil {
		return nil, err
	}
	return &portfolio, nil
}

// GetTransactions returns the authenticated account's transaction history,
// newest first. The server caps the page size.
func (c *Client) GetTransactions(ctx context.Context) ([]UserTransaction, error) {
	var txs []UserTransaction
	if err := c.analyticsGet(ctx, c.baseURL+"/api/v1/user/transactions", &txs); err != nil {
		return nil, fmt.Errorf("get transactions: %w", err)
	}
	return txs, nil
}
