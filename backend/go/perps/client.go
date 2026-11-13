// Package perps is TigerSwap's on-chain perpetual-futures client. It reads and
// writes the deployed TigerSwapPerpetuals contract directly, so positions,
// margin, funding and liquidation live on-chain — the same settlement model a
// Hyperliquid/dYdX-class venue uses — rather than in a private database.
//
// Every calldata builder is paired with a signed sender. The ABI is embedded
// from the compiled contract artifact, so the client cannot drift from the
// deployed bytecode: a signature change fails to parse at startup instead of
// sending a transaction the contract would reject.
package perps

import (
	"context"
	"crypto/ecdsa"
	_ "embed"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

//go:embed perps_abi.json
var perpsABIJSON string

var perpsABI abi.ABI

func init() {
	parsed, err := abi.JSON(strings.NewReader(perpsABIJSON))
	if err != nil {
		panic(fmt.Sprintf("perps: invalid embedded ABI: %v", err))
	}
	perpsABI = parsed
}

// ABI exposes the parsed TigerSwapPerpetuals ABI for callers that need to
// decode events or pack calls the client does not wrap.
func ABI() abi.ABI { return perpsABI }

// PRICE_PRECISION and RATIO_PRECISION from the contract, in WAD.
var (
	// OneUnit is 1e18, the contract's price and ratio precision.
	OneUnit = new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)
	// PricePrecision mirrors TigerSwapPerpetuals.PRICE_PRECISION.
	PricePrecision = OneUnit
	// RatioPrecision mirrors TigerSwapPerpetuals.RATIO_PRECISION.
	RatioPrecision = OneUnit
)

// Deployment describes the on-chain perpetuals contract for one chain.
type Deployment struct {
	ChainID    int64
	Perpetuals common.Address
	RPCURL     string
	// Oracle, when set, is the address permitted to push prices and funding into
	// the contract. The client only needs it to submit oracle-side updates.
	Oracle common.Address
}

// MarketID derives the on-chain market id for a symbol. The convention is the
// keccak256 of the symbol bytes; the deploy script creates markets with exactly
// this id, so a client and the chain agree without a lookup table.
func MarketID(symbol string) common.Hash {
	return crypto.Keccak256Hash([]byte(symbol))
}

// Market is the on-chain market configuration and live open interest.
type Market struct {
	Token           common.Address
	MaxLeverage     *big.Int
	LiquidationFee  *big.Int
	MaintenanceRate *big.Int
	InitialRate     *big.Int
	IsActive        bool
	OpenFee         *big.Int
	MaxOpenInterest *big.Int
	OpenInterest    *big.Int
}

// Position is an on-chain position. A zero Size means the position is closed.
type Position struct {
	MarketID   common.Hash
	Trader     common.Address
	Size       *big.Int
	Collateral *big.Int
	EntryPrice *big.Int
	LastUpdate *big.Int
	IsLong     bool
}

// Funding is the on-chain funding state for a market.
type Funding struct {
	LastFundingRate *big.Int
	LastUpdateTime  *big.Int
	MarkPrice       *big.Int
	IndexPrice      *big.Int
}

// Client is a TigerSwapPerpetuals client for a single chain.
type Client struct {
	deployment Deployment
	client     *ethclient.Client
	signer     *ecdsa.PrivateKey

	mu   sync.Mutex
	gas  map[common.Address]uint64
	once sync.Once
}

// NewClient dials the configured RPC and prepares the client. A nil signer is
// allowed for read-only use; a write call then fails with a clear error rather
// than panicking.
func NewClient(d Deployment, signer *ecdsa.PrivateKey) (*Client, error) {
	if d.RPCURL == "" {
		return nil, fmt.Errorf("no RPC URL configured for chain %d", d.ChainID)
	}
	if d.Perpetuals == (common.Address{}) {
		return nil, fmt.Errorf("TigerSwap perps address not configured for chain %d", d.ChainID)
	}
	client, err := ethclient.Dial(d.RPCURL)
	if err != nil {
		return nil, fmt.Errorf("dial rpc for chain %d: %w", d.ChainID, err)
	}
	return &Client{
		deployment: d,
		client:     client,
		signer:     signer,
		gas:        make(map[common.Address]uint64),
	}, nil
}

// ChainID returns the deployment chain id.
func (c *Client) ChainID() int64 { return c.deployment.ChainID }

// Close releases the RPC connection.
func (c *Client) Close() {
	c.once.Do(func() { c.client.Close() })
}

// RPC exposes the underlying ethclient for block and balance queries.
func (c *Client) RPC() *ethclient.Client { return c.client }

// PerpetualsAddress returns the configured contract address.
func (c *Client) PerpetualsAddress() common.Address { return c.deployment.Perpetuals }

// call packs and executes an eth_call against the perps contract, returning the
// raw ABI-encoded output.
func (c *Client) call(ctx context.Context, method string, args ...interface{}) ([]byte, error) {
	data, err := perpsABI.Pack(method, args...)
	if err != nil {
		return nil, fmt.Errorf("pack %s: %w", method, err)
	}
	to := c.deployment.Perpetuals
	out, err := c.client.CallContract(ctx, ethereum.CallMsg{To: &to, Data: data}, nil)
	if err != nil {
		return nil, fmt.Errorf("call %s: %w", method, err)
	}
	return out, nil
}

// Market reads a market's configuration and open interest.
func (c *Client) Market(ctx context.Context, id common.Hash) (*Market, error) {
	out, err := c.call(ctx, "getMarket", id)
	if err != nil {
		return nil, err
	}
	vals, err := perpsABI.Unpack("getMarket", out)
	if err != nil {
		return nil, fmt.Errorf("unpack getMarket: %w", err)
	}
	if len(vals) != 1 {
		return nil, fmt.Errorf("getMarket returned %d values", len(vals))
	}
	// getMarket returns a single struct (tuple) value. The anonymous struct's
	// field names must match the ABI component names exactly, because
	// go-ethereum's abi.Unpack generates that exact Go type for the tuple.
	converted, ok := vals[0].(struct {
		Token             common.Address `json:"token"`
		MaxLeverage       *big.Int       `json:"maxLeverage"`
		LiquidationFee    *big.Int       `json:"liquidationFee"`
		MaintenanceMargin *big.Int       `json:"maintenanceMargin"`
		InitialMargin     *big.Int       `json:"initialMargin"`
		IsActive          bool           `json:"isActive"`
		OpenFee           *big.Int       `json:"openFee"`
		MaxOpenInterest   *big.Int       `json:"maxOpenInterest"`
		OpenInterest      *big.Int       `json:"openInterest"`
	})
	if !ok {
		return nil, errors.New("getMarket: unexpected tuple shape")
	}
	return &Market{
		Token:           converted.Token,
		MaxLeverage:     converted.MaxLeverage,
		LiquidationFee:  converted.LiquidationFee,
		MaintenanceRate: converted.MaintenanceMargin,
		InitialRate:     converted.InitialMargin,
		IsActive:        converted.IsActive,
		OpenFee:         converted.OpenFee,
		MaxOpenInterest: converted.MaxOpenInterest,
		OpenInterest:    converted.OpenInterest,
	}, nil
}

// Position reads a position by id. A closed position has a zero Size.
func (c *Client) Position(ctx context.Context, id common.Hash) (*Position, error) {
	out, err := c.call(ctx, "getPosition", id)
	if err != nil {
		return nil, err
	}
	vals, err := perpsABI.Unpack("getPosition", out)
	if err != nil {
		return nil, fmt.Errorf("unpack getPosition: %w", err)
	}
	if len(vals) != 7 {
		return nil, fmt.Errorf("getPosition returned %d values", len(vals))
	}
	marketID, _ := vals[0].([32]byte)
	trader, _ := vals[1].(common.Address)
	return &Position{
		MarketID:   common.Hash(marketID),
		Trader:     trader,
		Size:       toBig(vals[2]),
		Collateral: toBig(vals[3]),
		EntryPrice: toBig(vals[4]),
		LastUpdate: toBig(vals[5]),
		IsLong:     toBool(vals[6]),
	}, nil
}

// MarkPrice reads the mark price the contract uses for PnL and margin.
func (c *Client) MarkPrice(ctx context.Context, id common.Hash) (*big.Int, error) {
	out, err := c.call(ctx, "getMarkPrice", id)
	if err != nil {
		return nil, err
	}
	vals, err := perpsABI.Unpack("getMarkPrice", out)
	if err != nil {
		return nil, fmt.Errorf("unpack getMarkPrice: %w", err)
	}
	return toBig(first(vals)), nil
}

// FundingInfo reads the funding state for a market.
func (c *Client) FundingInfo(ctx context.Context, id common.Hash) (*Funding, error) {
	out, err := c.call(ctx, "getFundingInfo", id)
	if err != nil {
		return nil, err
	}
	vals, err := perpsABI.Unpack("getFundingInfo", out)
	if err != nil {
		return nil, fmt.Errorf("unpack getFundingInfo: %w", err)
	}
	if len(vals) != 4 {
		return nil, fmt.Errorf("getFundingInfo returned %d values", len(vals))
	}
	return &Funding{
		LastFundingRate: toBig(vals[0]),
		LastUpdateTime:  toBig(vals[1]),
		MarkPrice:       toBig(vals[2]),
		IndexPrice:      toBig(vals[3]),
	}, nil
}

// PoolLiquidity reads the LP liquidity for a settlement token.
func (c *Client) PoolLiquidity(ctx context.Context, token common.Address) (*big.Int, error) {
	return c.uintView(ctx, "poolLiquidity", token)
}

// ReservedLiquidity reads the liquidity reserved to back open positions.
func (c *Client) ReservedLiquidity(ctx context.Context, token common.Address) (*big.Int, error) {
	return c.uintView(ctx, "reservedLiquidity", token)
}

// ProtocolFees reads the accrued protocol fees for a settlement token.
func (c *Client) ProtocolFees(ctx context.Context, token common.Address) (*big.Int, error) {
	return c.uintView(ctx, "protocolFees", token)
}

// UnpaidProfits reads the pool's outstanding obligation for a settlement token.
func (c *Client) UnpaidProfits(ctx context.Context, token common.Address) (*big.Int, error) {
	return c.uintView(ctx, "unpaidProfits", token)
}

// TotalPoolShares reads the total LP shares for a settlement token.
func (c *Client) TotalPoolShares(ctx context.Context, token common.Address) (*big.Int, error) {
	return c.uintView(ctx, "totalPoolShares", token)
}

// PoolShares reads a provider's LP shares for a settlement token.
func (c *Client) PoolShares(ctx context.Context, token, provider common.Address) (*big.Int, error) {
	return c.uintView(ctx, "poolShares", token, provider)
}

// FreeLiquidity reads the liquidity available to back new positions.
func (c *Client) FreeLiquidity(ctx context.Context, token common.Address) (*big.Int, error) {
	return c.uintView(ctx, "freeLiquidity", token)
}

// uintView reads a single uint256-returning view method.
func (c *Client) uintView(ctx context.Context, method string, args ...interface{}) (*big.Int, error) {
	out, err := c.call(ctx, method, args...)
	if err != nil {
		return nil, err
	}
	vals, err := perpsABI.Unpack(method, out)
	if err != nil {
		return nil, fmt.Errorf("unpack %s: %w", method, err)
	}
	return toBig(first(vals)), nil
}

// TraderPositions returns every position id a trader has ever opened.
func (c *Client) TraderPositions(ctx context.Context, trader common.Address) ([]common.Hash, error) {
	out, err := c.call(ctx, "getTraderPositions", trader)
	if err != nil {
		return nil, err
	}
	vals, err := perpsABI.Unpack("getTraderPositions", out)
	if err != nil {
		return nil, fmt.Errorf("unpack getTraderPositions: %w", err)
	}
	return hashSlice(first(vals)), nil
}

// OpenPositions returns the trader's still-open position ids.
func (c *Client) OpenPositions(ctx context.Context, trader common.Address) ([]common.Hash, error) {
	out, err := c.call(ctx, "getOpenPositions", trader)
	if err != nil {
		return nil, err
	}
	vals, err := perpsABI.Unpack("getOpenPositions", out)
	if err != nil {
		return nil, fmt.Errorf("unpack getOpenPositions: %w", err)
	}
	return hashSlice(first(vals)), nil
}

// CalculatePnL mirrors the contract's pure PnL math for a position at a price.
func (c *Client) CalculatePnL(ctx context.Context, p *Position, currentPrice *big.Int) (*big.Int, error) {
	out, err := c.call(ctx, "calculatePnL", positionTuple(p), currentPrice)
	if err != nil {
		return nil, err
	}
	vals, err := perpsABI.Unpack("calculatePnL", out)
	if err != nil {
		return nil, fmt.Errorf("unpack calculatePnL: %w", err)
	}
	return toBig(first(vals)), nil
}

// positionTuple shapes a Position for the contract's struct parameter.
func positionTuple(p *Position) struct {
	MarketID    [32]byte
	Trader      common.Address
	Size        *big.Int
	Collateral  *big.Int
	EntryPrice  *big.Int
	LastUpdated *big.Int
	IsLong      bool
} {
	return struct {
		MarketID    [32]byte
		Trader      common.Address
		Size        *big.Int
		Collateral  *big.Int
		EntryPrice  *big.Int
		LastUpdated *big.Int
		IsLong      bool
	}{
		MarketID:    p.MarketID,
		Trader:      p.Trader,
		Size:        p.Size,
		Collateral:  p.Collateral,
		EntryPrice:  p.EntryPrice,
		LastUpdated: p.LastUpdate,
		IsLong:      p.IsLong,
	}
}

// --- helpers ---

func first(vals []interface{}) interface{} {
	if len(vals) == 0 {
		return nil
	}
	return vals[0]
}

func toBig(v interface{}) *big.Int {
	switch n := v.(type) {
	case *big.Int:
		if n == nil {
			return big.NewInt(0)
		}
		return n
	case big.Int:
		return new(big.Int).Set(&n)
	case *uint8:
		return big.NewInt(int64(*n))
	}
	return big.NewInt(0)
}

func toBool(v interface{}) bool {
	b, _ := v.(bool)
	return b
}

func hashSlice(v interface{}) []common.Hash {
	raw, ok := v.([][32]byte)
	if !ok {
		return nil
	}
	out := make([]common.Hash, len(raw))
	for i := range raw {
		out[i] = common.Hash(raw[i])
	}
	return out
}
