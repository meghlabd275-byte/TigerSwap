// Package orders is TigerSwap's on-chain order-execution client. It reads and
// writes the deployed TigerSwapAdvancedOrders contract, so a limit, stop-loss,
// take-profit, TWAP, GTC, IOC or post-only order is escrowed and settled
// on-chain — the same model an on-chain DEX uses — rather than recorded in a
// private table that nothing ever fills.
//
// The ABI is embedded from the compiled contract artifact, so the client cannot
// drift from the deployed bytecode: a signature change fails to parse at startup
// instead of sending a transaction the contract would reject.
package orders

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

//go:embed advanced_orders_abi.json
var ordersABIJSON string

var ordersABI abi.ABI

func init() {
	parsed, err := abi.JSON(strings.NewReader(ordersABIJSON))
	if err != nil {
		panic(fmt.Sprintf("orders: invalid embedded ABI: %v", err))
	}
	ordersABI = parsed
}

// ABI exposes the parsed TigerSwapAdvancedOrders ABI.
func ABI() abi.ABI { return ordersABI }

// OrderType mirrors the contract's OrderType enum.
type OrderType uint8

// OrderSide mirrors the contract's OrderSide enum.
type OrderSide uint8

// OrderStatus mirrors the contract's OrderStatus enum.
type OrderStatus uint8

const (
	TypeMarket OrderType = iota
	TypeLimit
	TypeStopLoss
	TypeTakeProfit
	TypeTWAP
	TypeGTC
	TypeIOC
	TypePostOnly
)

const (
	SideBuy OrderSide = iota
	SideSell
)

const (
	StatusPending OrderStatus = iota
	StatusOpen
	StatusPartiallyFilled
	StatusFilled
	StatusCancelled
	StatusExpired
	StatusTriggered
)

// ParseOrderType maps a wire order-type name to the contract enum. An unknown
// name is an error rather than a silent default, so an order is never created
// with a type the caller did not ask for.
func ParseOrderType(name string) (OrderType, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "limit":
		return TypeLimit, nil
	case "stop_loss", "stop-loss", "stop":
		return TypeStopLoss, nil
	case "take_profit", "take-profit":
		return TypeTakeProfit, nil
	case "twap":
		return TypeTWAP, nil
	case "gtc":
		return TypeGTC, nil
	case "ioc":
		return TypeIOC, nil
	case "post_only", "post-only":
		return TypePostOnly, nil
	case "market":
		return TypeMarket, nil
	default:
		return 0, fmt.Errorf("unknown order type %q", name)
	}
}

// ParseOrderSide maps "buy"/"sell" to the contract enum.
func ParseOrderSide(side string) (OrderSide, error) {
	switch strings.ToLower(strings.TrimSpace(side)) {
	case "buy", "long":
		return SideBuy, nil
	case "sell", "short":
		return SideSell, nil
	default:
		return 0, fmt.Errorf("side must be buy or sell, got %q", side)
	}
}

// Deployment describes the on-chain order contracts for one chain.
type Deployment struct {
	ChainID int64
	Orders  common.Address
	TWAMM   common.Address
	RPCURL  string
}

// Order mirrors the contract's Order struct.
type Order struct {
	ID                 *big.Int
	Owner              common.Address
	OrderType          OrderType
	Side               OrderSide
	TokenIn            common.Address
	TokenOut           common.Address
	AmountIn           *big.Int
	AmountOutMin       *big.Int
	Price              *big.Int
	StopPrice          *big.Int
	FilledAmount       *big.Int
	AmountOutReceived  *big.Int
	CreatedAt          *big.Int
	ExpiresAt          *big.Int
	Status             OrderStatus
	TWAPInterval       *big.Int
	TWAPParts          *big.Int
	TWAPCompletedParts *big.Int
	LastFillAt         *big.Int
}

// Client is a TigerSwapAdvancedOrders client for a single chain.
type Client struct {
	deployment Deployment
	client     *ethclient.Client
	signer     *ecdsa.PrivateKey

	mu   sync.Mutex
	gas  map[common.Address]uint64
	once sync.Once
}

// ErrNoSigner is returned by a write method when the client has no signer.
var ErrNoSigner = errors.New("orders: no signer configured")

// NewClient dials the configured RPC and prepares the client. A nil signer is
// allowed for read-only use; a write call then fails with a clear error rather
// than panicking.
func NewClient(d Deployment, signer *ecdsa.PrivateKey) (*Client, error) {
	if d.RPCURL == "" {
		return nil, fmt.Errorf("no RPC URL configured for chain %d", d.ChainID)
	}
	if d.Orders == (common.Address{}) {
		return nil, fmt.Errorf("TigerSwap advanced-orders address not configured for chain %d", d.ChainID)
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

// SetSigner binds a signer so Send submits a transaction. Without one the client
// is read-only and Send returns ErrNoSigner. It exists so a deployment can be
// registered before the executor key is loaded.
func (c *Client) SetSigner(key *ecdsa.PrivateKey) {
	c.mu.Lock()
	c.signer = key
	c.mu.Unlock()
}

// OrdersAddress returns the configured advanced-orders contract address.
func (c *Client) OrdersAddress() common.Address { return c.deployment.Orders }

// Deployment returns the deployment this client is bound to.
func (c *Client) Deployment() Deployment { return c.deployment }

// RPC exposes the underlying ethclient for block and balance queries.
func (c *Client) RPC() *ethclient.Client { return c.client }

// Close releases the RPC connection.
func (c *Client) Close() {
	c.once.Do(func() { c.client.Close() })
}

// SignerAddress returns the address that signs this client's transactions, or
// the zero address when no signer is configured.
func (c *Client) SignerAddress() common.Address {
	if c.signer == nil {
		return common.Address{}
	}
	return crypto.PubkeyToAddress(c.signer.PublicKey)
}

// call packs and executes an eth_call against the orders contract.
func (c *Client) call(ctx context.Context, method string, args ...interface{}) ([]byte, error) {
	data, err := ordersABI.Pack(method, args...)
	if err != nil {
		return nil, fmt.Errorf("pack %s: %w", method, err)
	}
	to := c.deployment.Orders
	out, err := c.client.CallContract(ctx, ethereum.CallMsg{To: &to, Data: data}, nil)
	if err != nil {
		return nil, fmt.Errorf("call %s: %w", method, err)
	}
	return out, nil
}

// Order reads an order by id.
func (c *Client) Order(ctx context.Context, id *big.Int) (*Order, error) {
	out, err := c.call(ctx, "getOrder", id)
	if err != nil {
		return nil, err
	}
	vals, err := ordersABI.Unpack("getOrder", out)
	if err != nil {
		return nil, fmt.Errorf("unpack getOrder: %w", err)
	}
	if len(vals) != 1 {
		return nil, fmt.Errorf("getOrder returned %d values", len(vals))
	}
	converted, ok := vals[0].(struct {
		Id                 *big.Int       `json:"id"`
		Owner              common.Address `json:"owner"`
		OrderType          uint8          `json:"orderType"`
		Side               uint8          `json:"side"`
		TokenIn            common.Address `json:"tokenIn"`
		TokenOut           common.Address `json:"tokenOut"`
		AmountIn           *big.Int       `json:"amountIn"`
		AmountOutMin       *big.Int       `json:"amountOutMin"`
		Price              *big.Int       `json:"price"`
		StopPrice          *big.Int       `json:"stopPrice"`
		FilledAmount       *big.Int       `json:"filledAmount"`
		AmountOutReceived  *big.Int       `json:"amountOutReceived"`
		CreatedAt          *big.Int       `json:"createdAt"`
		ExpiresAt          *big.Int       `json:"expiresAt"`
		Status             uint8          `json:"status"`
		TwapInterval       *big.Int       `json:"twapInterval"`
		TwapParts          *big.Int       `json:"twapParts"`
		TwapCompletedParts *big.Int       `json:"twapCompletedParts"`
		LastFillAt         *big.Int       `json:"lastFillAt"`
	})
	if !ok {
		return nil, errors.New("getOrder: unexpected tuple shape")
	}
	return &Order{
		ID:                 converted.Id,
		Owner:              converted.Owner,
		OrderType:          OrderType(converted.OrderType),
		Side:               OrderSide(converted.Side),
		TokenIn:            converted.TokenIn,
		TokenOut:           converted.TokenOut,
		AmountIn:           toBig(converted.AmountIn),
		AmountOutMin:       toBig(converted.AmountOutMin),
		Price:              toBig(converted.Price),
		StopPrice:          toBig(converted.StopPrice),
		FilledAmount:       toBig(converted.FilledAmount),
		AmountOutReceived:  toBig(converted.AmountOutReceived),
		CreatedAt:          toBig(converted.CreatedAt),
		ExpiresAt:          toBig(converted.ExpiresAt),
		Status:             OrderStatus(converted.Status),
		TWAPInterval:       toBig(converted.TwapInterval),
		TWAPParts:          toBig(converted.TwapParts),
		TWAPCompletedParts: toBig(converted.TwapCompletedParts),
		LastFillAt:         toBig(converted.LastFillAt),
	}, nil
}

// UserOrders returns every order id a user has created.
func (c *Client) UserOrders(ctx context.Context, user common.Address) ([]*big.Int, error) {
	out, err := c.call(ctx, "getUserOrders", user)
	if err != nil {
		return nil, err
	}
	vals, err := ordersABI.Unpack("getUserOrders", out)
	if err != nil {
		return nil, fmt.Errorf("unpack getUserOrders: %w", err)
	}
	return bigSlice(first(vals)), nil
}

// OrderCount reads the number of orders ever created.
func (c *Client) OrderCount(ctx context.Context) (*big.Int, error) {
	out, err := c.call(ctx, "orderCount")
	if err != nil {
		return nil, err
	}
	vals, err := ordersABI.Unpack("orderCount", out)
	if err != nil {
		return nil, fmt.Errorf("unpack orderCount: %w", err)
	}
	return toBig(first(vals)), nil
}

// OrdersByStatus returns every order id currently in the given status, so the
// executor can discover work from chain state rather than a seeded list.
func (c *Client) OrdersByStatus(ctx context.Context, status OrderStatus) ([]*big.Int, error) {
	out, err := c.call(ctx, "getOrdersByStatus", uint8(status))
	if err != nil {
		return nil, err
	}
	vals, err := ordersABI.Unpack("getOrdersByStatus", out)
	if err != nil {
		return nil, fmt.Errorf("unpack getOrdersByStatus: %w", err)
	}
	return bigSlice(first(vals)), nil
}

// QuoteOutput quotes a fill through the order book's own router, so a caller
// can size amountOutMin from the real pool rather than a guessed price.
func (c *Client) QuoteOutput(ctx context.Context, tokenIn, tokenOut common.Address, amountIn *big.Int) (*big.Int, error) {
	out, err := c.call(ctx, "quoteOutput", tokenIn, tokenOut, amountIn)
	if err != nil {
		return nil, err
	}
	vals, err := ordersABI.Unpack("quoteOutput", out)
	if err != nil {
		return nil, fmt.Errorf("unpack quoteOutput: %w", err)
	}
	return toBig(first(vals)), nil
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

func bigSlice(v interface{}) []*big.Int {
	raw, ok := v.([]*big.Int)
	if !ok {
		return nil
	}
	return raw
}

func toBool(v interface{}) bool {
	b, _ := v.(bool)
	return b
}
