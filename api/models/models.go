package models

import (
	"math/big"
	"time"
)

// ============ Order Models ============

// Order represents a trading order
type Order struct {
	ID            uint64    `json:"id"`
	Owner         string    `json:"owner"`
	TokenIn       string    `json:"tokenIn"`
	TokenOut      string    `json:"tokenOut"`
	AmountIn      *big.Int  `json:"amountIn"`
	AmountOutMin  *big.Int  `json:"amountOutMin"`
	Price        *big.Int  `json:"price"`
	StopPrice    *big.Int  `json:"stopPrice"`
	ExecutedAmount *big.Int `json:"executedAmount"`
	FilledAmountIn *big.Int `json:"filledAmountIn"`
	OrderType    string    `json:"orderType"`
	Status      string    `json:"status"`
	CreatedAt    time.Time `json:"createdAt"`
	ExpiresAt   time.Time `json:"expiresAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	IsNative    bool      `json:"isNative"`
}

// CreateOrderRequest is a request to create an order
type CreateOrderRequest struct {
	TokenIn      string   `json:"tokenIn"`
	TokenOut     string   `json:"tokenOut"`
	AmountIn     *big.Int `json:"amountIn"`
	AmountOutMin *big.Int `json:"amountOutMin"`
	Price        *big.Int `json:"price"`
	OrderType    string   `json:"orderType"`
	ExpiresAt    int64    `json:"expiresAt"`
	IsNative    bool     `json:"isNative"`
}

// CreateOrderResponse is a response to create an order
type CreateOrderResponse struct {
	OrderID uint64 `json:"orderId"`
	Status  string `json:"status"`
}

// CancelOrderRequest is a request to cancel an order
type CancelOrderRequest struct {
	OrderID uint64 `json:"orderId"`
}

// CancelOrderResponse is a response to cancel an order
type CancelOrderResponse struct {
	OrderID uint64 `json:"orderId"`
	Status string `json:"status"`
}

// GetOrdersResponse is a response to get orders
type GetOrdersResponse struct {
	Orders []*Order `json:"orders"`
	Count  int     `json:"count"`
}

// GetOrderBookResponse is a response to get order book
type GetOrderBookResponse struct {
	Orders []*Order `json:"orders"`
	Count  int     `json:"count"`
}

// GetBestPriceResponse is a response to get best price
type GetBestPriceResponse struct {
	Price    *big.Int `json:"price"`
	Liquidity *big.Int `json:"liquidity"`
}

// ============ Quote Models ============

// QuoteResponse is a response for a quote
type QuoteResponse struct {
	TokenIn   string   `json:"tokenIn"`
	TokenOut  string   `json:"tokenOut"`
	AmountIn  *big.Int `json:"amountIn"`
	AmountOut *big.Int `json:"amountOut"`
}

// ============ Swap Models ============

// SwapRequest is a request to swap
type SwapRequest struct {
	TokenIn      string   `json:"tokenIn"`
	TokenOut     string   `json:"tokenOut"`
	AmountIn     *big.Int `json:"amountIn"`
	AmountOutMin *big.Int `json:"amountOutMin"`
	Recipient   string   `json:"recipient"`
}

// SwapResponse is a response for a swap
type SwapResponse struct {
	TxHash    string   `json:"txHash"`
	AmountIn  *big.Int `json:"amountIn"`
	AmountOut *big.Int `json:"amountOut"`
}

// ============ Route Models ============

// RouteResponse is a response for a route
type RouteResponse struct {
	Path       []string `json:"path"`
	AmountOut  *big.Int `json:"amountOut"`
	GasUsed   uint64   `json:"gasUsed"`
	InputAmount *big.Int `json:"inputAmount"`
}

// SplitRouteResponse is a response for a split route
type SplitRouteResponse struct {
	Routes    []RouteResponse `json:"routes"`
	TotalOut  *big.Int       `json:"totalOut"`
	GasUsed   uint64         `json:"gasUsed"`
}

// ============ DCA Models ============

// DCAPlan represents a DCA plan
type DCAPlan struct {
	ID                uint64    `json:"id"`
	Owner             string    `json:"owner"`
	TokenIn           string    `json:"tokenIn"`
	TokenOut          string    `json:"tokenOut"`
	AmountInPerExec   *big.Int  `json:"amountInPerExec"`
	Interval         int64     `json:"interval"`
	NextExecTime     time.Time `json:"nextExecTime"`
	ExecutionsDone   uint64    `json:"executionsDone"`
	MaxExecutions    uint64    `json:"maxExecutions"`
	Status           string    `json:"status"`
}

// CreateDCAPlanRequest is a request to create a DCA plan
type CreateDCAPlanRequest struct {
	TokenIn         string   `json:"tokenIn"`
	TokenOut        string   `json:"tokenOut"`
	AmountInPerExec *big.Int `json:"amountInPerExec"`
	Interval       int64    `json:"interval"`
	MaxExecutions  uint64   `json:"maxExecutions"`
	DCAType       string   `json:"dcaType"`
}

// CreateDCAPlanResponse is a response to create a DCA plan
type CreateDCAPlanResponse struct {
	PlanID uint64 `json:"planId"`
	Status string `json:"status"`
}

// DCAPlanResponse is a response for a DCA plan
type DCAPlanResponse struct {
	Plan *DCAPlan `json:"plan"`
}

// ============ Pool Models ============

// Pool represents a liquidity pool
type Pool struct {
	Address       string   `json:"address"`
	Token0       string   `json:"token0"`
	Token1       string   `json:"token1"`
	Reserve0     *big.Int `json:"reserve0"`
	Reserve1     *big.Int `json:"reserve1"`
	Liquidity    *big.Int `json:"liquidity"`
	Fee          int      `json:"fee"`
	Token0Price *big.Int `json:"token0Price"`
	Token1Price *big.Int `json:"token1Price"`
}

// ============ Governance Models ============

// Proposal represents a governance proposal
type Proposal struct {
	ID          uint64    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Creator     string    `json:"creator"`
	ForVotes   *big.Int `json:"forVotes"`
	AgainstVotes *big.Int `json:"againstVotes"`
	AbstainVotes *big.Int `json:"abstainVotes"`
	StartBlock  uint64    `json:"startBlock"`
	EndBlock   uint64    `json:"endBlock"`
	Status     string    `json:"status"`
	CreatedAt   time.Time `json:"createdAt"`
}

// Vote represents a vote
type Vote struct {
	Voter    string   `json:"voter"`
	Proposal uint64   `json:"proposal"`
	Support  bool     `json:"support"`
	Weight   *big.Int `json:"weight"`
}

// LockPosition represents a veToken lock position
type LockPosition struct {
	ID           uint64    `json:"id"`
	Owner        string   `json:"owner"`
	Amount       *big.Int `json:"amount"`
	LockEnd     time.Time `json:"lockEnd"`
	VotingPower *big.Int `json:"votingPower"`
	BoostFactor *big.Int `json:"boostFactor"`
}

// ============ Concentrated Liquidity Models ============

// CLPosition represents a concentrated liquidity position
type CLPosition struct {
	ID         uint64    `json:"id"`
	Owner      string   `json:"owner"`
	Token0     string   `json:"token0"`
	Token1     string   `json:"token1"`
	TickLower  int      `json:"tickLower"`
	TickUpper  int      `json:"tickUpper"`
	Liquidity  *big.Int `json:"liquidity"`
	FeeGrowth *big.Int `json:"feeGrowth"`
	TokensOwed0 *big.Int `json:"tokensOwed0"`
	TokensOwed1 *big.Int `json:"tokensOwed1"`
}

// Tick represents a tick in a CL pool
type Tick struct {
	Index          int      `json:"index"`
	LiquidityNet   *big.Int `json:"liquidityNet"`
	LiquidityGross *big.Int `json:"liquidityGross"`
	FeeGrowth0    *big.Int `json:"feeGrowth0"`
	FeeGrowth1    *big.Int `json:"feeGrowth1"`
}

// ============ Error Response ============

// ErrorResponse is an error response
type ErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ============ Success Response ============

// SuccessResponse is a success response
type SuccessResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}