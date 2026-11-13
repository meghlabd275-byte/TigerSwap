// Package orderbook is TigerSwap's on-chain perpetual order book client. It
// reads and writes the deployed TigerSwapPerpOrderBook contract directly, so
// orders, matching, margin, funding and liquidation live on-chain — the same
// settlement model a Hyperliquid/dYdX-class venue uses — rather than in a
// private database.
//
// Every calldata builder is paired with a signed sender. The ABI is embedded
// from the compiled contract artifact, so the client cannot drift from the
// deployed bytecode: a signature change fails to parse at startup instead of
// sending a transaction the contract would reject.
package orderbook

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

//go:embed orderbook_abi.json
var orderBookABIJSON string

var orderBookABI abi.ABI

func init() {
	parsed, err := abi.JSON(strings.NewReader(orderBookABIJSON))
	if err != nil {
		panic(fmt.Sprintf("orderbook: invalid embedded ABI: %v", err))
	}
	orderBookABI = parsed
}

// ABI exposes the parsed TigerSwapPerpOrderBook ABI.
func ABI() abi.ABI { return orderBookABI }

// OneUnit is 1e18, the contract's WAD price precision.
var OneUnit = new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)

// Time-in-force codes, mirroring the contract constants.
const (
	TIFGTC      uint8 = 0
	TIFIOC      uint8 = 1
	TIFFOK      uint8 = 2
	TIFPostOnly uint8 = 3
)

// Deployment describes the on-chain order book for one chain.
type Deployment struct {
	ChainID   int64
	OrderBook common.Address
	RPCURL    string
	// Oracle is the address permitted to push prices and funding.
	Oracle common.Address
}

// MarketID derives the on-chain market id for a symbol, matching the perps
// contract convention (keccak256 of the symbol bytes).
func MarketID(symbol string) common.Hash {
	return crypto.Keccak256Hash([]byte(symbol))
}

// Market is the on-chain market configuration.
type Market struct {
	Token             common.Address
	OracleAsset       common.Address
	TakerFee          *big.Int
	MakerFee          *big.Int
	InitialMargin     *big.Int
	MaintenanceMargin *big.Int
	LiquidationFee    *big.Int
	MaxLeverage       *big.Int
	MaxOpenInterest   *big.Int
	OpenInterest      *big.Int
	MinOrderSize      *big.Int
	LotSize           *big.Int
	IsActive          bool
}

// Position is a trader's on-chain position view.
type Position struct {
	Size          *big.Int
	EntryPrice    *big.Int
	Margin        *big.Int
	UnrealizedPnL *big.Int
	Equity        *big.Int
}

// Order is a resting on-chain order.
type Order struct {
	ID         *big.Int
	Trader     common.Address
	Price      *big.Int
	Quantity   *big.Int
	Filled     *big.Int
	IsLong     bool
	ReduceOnly bool
	PostOnly   bool
	Timestamp  *big.Int
	Active     bool
}

// Funding is the on-chain funding state for a market.
type Funding struct {
	CumulativeIndex *big.Int
	LastFundingRate *big.Int
	LastUpdateTime  *big.Int
	MarkPrice       *big.Int
	IndexPrice      *big.Int
}

// Client is a TigerSwapPerpOrderBook client for a single chain.
type Client struct {
	deployment Deployment
	client     *ethclient.Client
	signer     *ecdsa.PrivateKey

	mu   sync.Mutex
	gas  map[common.Address]uint64
	once sync.Once
}

// NewClient dials the configured RPC and prepares the client. A nil signer is
// allowed for read-only use; a write call then fails with a clear error.
func NewClient(d Deployment, signer *ecdsa.PrivateKey) (*Client, error) {
	if d.RPCURL == "" {
		return nil, fmt.Errorf("no RPC URL configured for chain %d", d.ChainID)
	}
	if d.OrderBook == (common.Address{}) {
		return nil, fmt.Errorf("TigerSwap order book address not configured for chain %d", d.ChainID)
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
func (c *Client) Close() { c.once.Do(func() { c.client.Close() }) }

// RPC exposes the underlying ethclient.
func (c *Client) RPC() *ethclient.Client { return c.client }

// Address returns the configured order book address.
func (c *Client) Address() common.Address { return c.deployment.OrderBook }

// Deployment returns the deployment this client is bound to.
func (c *Client) Deployment() Deployment { return c.deployment }

// SignerAddress returns the address that signs this client's transactions.
func (c *Client) SignerAddress() common.Address {
	if c.signer == nil {
		return common.Address{}
	}
	return crypto.PubkeyToAddress(c.signer.PublicKey)
}

func (c *Client) call(ctx context.Context, method string, args ...interface{}) ([]byte, error) {
	data, err := orderBookABI.Pack(method, args...)
	if err != nil {
		return nil, fmt.Errorf("pack %s: %w", method, err)
	}
	to := c.deployment.OrderBook
	out, err := c.client.CallContract(ctx, ethereum.CallMsg{To: &to, Data: data}, nil)
	if err != nil {
		return nil, fmt.Errorf("call %s: %w", method, err)
	}
	return out, nil
}

// Market reads a market's configuration.
func (c *Client) Market(ctx context.Context, id common.Hash) (*Market, error) {
	out, err := c.call(ctx, "getMarket", id)
	if err != nil {
		return nil, err
	}
	var m struct {
		Token             common.Address
		OracleAsset       common.Address
		TakerFee          *big.Int
		MakerFee          *big.Int
		InitialMargin     *big.Int
		MaintenanceMargin *big.Int
		LiquidationFee    *big.Int
		MaxLeverage       *big.Int
		MaxOpenInterest   *big.Int
		OpenInterest      *big.Int
		MinOrderSize      *big.Int
		LotSize           *big.Int
		IsActive          bool
	}
	if err := orderBookABI.UnpackIntoInterface(&m, "getMarket", out); err != nil {
		return nil, fmt.Errorf("decode getMarket: %w", err)
	}
	return &Market{
		Token:             m.Token,
		OracleAsset:       m.OracleAsset,
		TakerFee:          m.TakerFee,
		MakerFee:          m.MakerFee,
		InitialMargin:     m.InitialMargin,
		MaintenanceMargin: m.MaintenanceMargin,
		LiquidationFee:    m.LiquidationFee,
		MaxLeverage:       m.MaxLeverage,
		MaxOpenInterest:   m.MaxOpenInterest,
		OpenInterest:      m.OpenInterest,
		MinOrderSize:      m.MinOrderSize,
		LotSize:           m.LotSize,
		IsActive:          m.IsActive,
	}, nil
}

// Position reads a trader's position and margin view.
func (c *Client) Position(ctx context.Context, id common.Hash, trader common.Address) (*Position, error) {
	out, err := c.call(ctx, "getPosition", id, trader)
	if err != nil {
		return nil, err
	}
	var p struct {
		Size          *big.Int
		EntryPrice    *big.Int
		Margin        *big.Int
		UnrealizedPnL *big.Int
		Equity        *big.Int
	}
	if err := orderBookABI.UnpackIntoInterface(&p, "getPosition", out); err != nil {
		return nil, fmt.Errorf("decode getPosition: %w", err)
	}
	return &Position{Size: p.Size, EntryPrice: p.EntryPrice, Margin: p.Margin, UnrealizedPnL: p.UnrealizedPnL, Equity: p.Equity}, nil
}

// Order reads a single order by id.
func (c *Client) Order(ctx context.Context, orderID *big.Int) (*Order, error) {
	out, err := c.call(ctx, "getOrder", orderID)
	if err != nil {
		return nil, err
	}
	var o struct {
		ID         *big.Int
		Trader     common.Address
		Price      *big.Int
		Quantity   *big.Int
		Filled     *big.Int
		IsLong     bool
		ReduceOnly bool
		PostOnly   bool
		Timestamp  *big.Int
		Active     bool
	}
	if err := orderBookABI.UnpackIntoInterface(&o, "getOrder", out); err != nil {
		return nil, fmt.Errorf("decode getOrder: %w", err)
	}
	return &Order{ID: o.ID, Trader: o.Trader, Price: o.Price, Quantity: o.Quantity, Filled: o.Filled, IsLong: o.IsLong, ReduceOnly: o.ReduceOnly, PostOnly: o.PostOnly, Timestamp: o.Timestamp, Active: o.Active}, nil
}

// TraderOrders returns every order id a trader has placed (open and closed).
func (c *Client) TraderOrders(ctx context.Context, trader common.Address) ([]*big.Int, error) {
	out, err := c.call(ctx, "getTraderOrders", trader)
	if err != nil {
		return nil, err
	}
	var ids []*big.Int
	if err := orderBookABI.UnpackIntoInterface(&ids, "getTraderOrders", out); err != nil {
		return nil, fmt.Errorf("decode getTraderOrders: %w", err)
	}
	return ids, nil
}

// Funding reads a market's funding state.
func (c *Client) Funding(ctx context.Context, id common.Hash) (*Funding, error) {
	out, err := c.call(ctx, "getFundingInfo", id)
	if err != nil {
		return nil, err
	}
	var f struct {
		CumulativeIndex *big.Int
		LastFundingRate *big.Int
		LastUpdateTime  *big.Int
		MarkPrice       *big.Int
		IndexPrice      *big.Int
	}
	if err := orderBookABI.UnpackIntoInterface(&f, "getFundingInfo", out); err != nil {
		return nil, fmt.Errorf("decode getFundingInfo: %w", err)
	}
	return &Funding{CumulativeIndex: f.CumulativeIndex, LastFundingRate: f.LastFundingRate, LastUpdateTime: f.LastUpdateTime, MarkPrice: f.MarkPrice, IndexPrice: f.IndexPrice}, nil
}

// BestBidAsk reads the top of book.
func (c *Client) BestBidAsk(ctx context.Context, id common.Hash) (bidPrice, bidQty, askPrice, askQty *big.Int, err error) {
	out, err := c.call(ctx, "bestBidAsk", id)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	var r struct {
		BidPrice *big.Int
		BidQty   *big.Int
		AskPrice *big.Int
		AskQty   *big.Int
	}
	if err := orderBookABI.UnpackIntoInterface(&r, "bestBidAsk", out); err != nil {
		return nil, nil, nil, nil, fmt.Errorf("decode bestBidAsk: %w", err)
	}
	return r.BidPrice, r.BidQty, r.AskPrice, r.AskQty, nil
}

// Depth reads aggregate depth for the top `levels` price levels per side.
func (c *Client) Depth(ctx context.Context, id common.Hash, levels *big.Int) (bidPrices, bidQty, askPrices, askQty []*big.Int, err error) {
	out, err := c.call(ctx, "depth", id, levels)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	var r struct {
		BidPrices []*big.Int
		BidQty    []*big.Int
		AskPrices []*big.Int
		AskQty    []*big.Int
	}
	if err := orderBookABI.UnpackIntoInterface(&r, "depth", out); err != nil {
		return nil, nil, nil, nil, fmt.Errorf("decode depth: %w", err)
	}
	return r.BidPrices, r.BidQty, r.AskPrices, r.AskQty, nil
}

// Balances reads a trader's free margin for a market.
func (c *Client) Balances(ctx context.Context, id common.Hash, trader common.Address) (*big.Int, error) {
	out, err := c.call(ctx, "balances", id, trader)
	if err != nil {
		return nil, err
	}
	var v *big.Int
	if err := orderBookABI.UnpackIntoInterface(&v, "balances", out); err != nil {
		return nil, fmt.Errorf("decode balances: %w", err)
	}
	return v, nil
}

// PoolLiquidity reads the settlement units available to pay winners.
func (c *Client) PoolLiquidity(ctx context.Context, id common.Hash) (*big.Int, error) {
	out, err := c.call(ctx, "poolLiquidity", id)
	if err != nil {
		return nil, err
	}
	var v *big.Int
	if err := orderBookABI.UnpackIntoInterface(&v, "poolLiquidity", out); err != nil {
		return nil, fmt.Errorf("decode poolLiquidity: %w", err)
	}
	return v, nil
}

// UnpaidCredit reads the profit a trader is still owed because the pool could
// not pay it when the profit was realised.
func (c *Client) UnpaidCredit(ctx context.Context, id common.Hash, trader common.Address) (*big.Int, error) {
	out, err := c.call(ctx, "unpaidCredit", id, trader)
	if err != nil {
		return nil, err
	}
	var v *big.Int
	if err := orderBookABI.UnpackIntoInterface(&v, "unpaidCredit", out); err != nil {
		return nil, fmt.Errorf("decode unpaidCredit: %w", err)
	}
	return v, nil
}

// ErrNoSigner is returned by a write method when the client has no signer.
var ErrNoSigner = errors.New("orderbook: no signer configured")

// Call is an unsigned call to the order book contract, for a wallet to sign.
type Call struct {
	To    common.Address
	Data  []byte
	Value *big.Int
}

// Hex returns the calldata as a 0x-prefixed hex string.
func (call Call) Hex() string { return "0x" + common.Bytes2Hex(call.Data) }

// BuildDeposit encodes deposit(marketId, amount).
func (c *Client) BuildDeposit(marketID common.Hash, amount *big.Int) (Call, error) {
	if amount == nil || amount.Sign() <= 0 {
		return Call{}, errors.New("amount must be positive")
	}
	return c.pack("deposit", marketID, amount)
}

// BuildWithdraw encodes withdraw(marketId, amount).
func (c *Client) BuildWithdraw(marketID common.Hash, amount *big.Int) (Call, error) {
	if amount == nil || amount.Sign() <= 0 {
		return Call{}, errors.New("amount must be positive")
	}
	return c.pack("withdraw", marketID, amount)
}

// BuildPlaceOrder encodes placeOrder(marketId, quantity, price, isLong, tif, reduceOnly).
func (c *Client) BuildPlaceOrder(marketID common.Hash, quantity, price *big.Int, isLong bool, tif uint8, reduceOnly bool) (Call, error) {
	if quantity == nil || quantity.Sign() <= 0 {
		return Call{}, errors.New("quantity must be positive")
	}
	if price == nil || price.Sign() <= 0 {
		return Call{}, errors.New("price must be positive")
	}
	if tif > TIFPostOnly {
		return Call{}, errors.New("time-in-force must be 0..3")
	}
	return c.pack("placeOrder", marketID, quantity, price, isLong, tif, reduceOnly)
}

// BuildCancelOrder encodes cancelOrder(orderId).
func (c *Client) BuildCancelOrder(orderID *big.Int) (Call, error) {
	if orderID == nil || orderID.Sign() <= 0 {
		return Call{}, errors.New("order id must be positive")
	}
	return c.pack("cancelOrder", orderID)
}

// BuildLiquidate encodes liquidate(marketId, trader). Permissionless on-chain.
func (c *Client) BuildLiquidate(marketID common.Hash, trader common.Address) (Call, error) {
	if trader == (common.Address{}) {
		return Call{}, errors.New("trader must not be the zero address")
	}
	return c.pack("liquidate", marketID, trader)
}

// BuildAddLiquidity encodes addLiquidity(marketId, amount).
func (c *Client) BuildAddLiquidity(marketID common.Hash, amount *big.Int) (Call, error) {
	if amount == nil || amount.Sign() <= 0 {
		return Call{}, errors.New("amount must be positive")
	}
	return c.pack("addLiquidity", marketID, amount)
}

// BuildRemoveLiquidity encodes removeLiquidity(marketId, amount).
func (c *Client) BuildRemoveLiquidity(marketID common.Hash, amount *big.Int) (Call, error) {
	if amount == nil || amount.Sign() <= 0 {
		return Call{}, errors.New("amount must be positive")
	}
	return c.pack("removeLiquidity", marketID, amount)
}

// BuildDepositInsurance encodes depositInsurance(marketId, amount).
func (c *Client) BuildDepositInsurance(marketID common.Hash, amount *big.Int) (Call, error) {
	if amount == nil || amount.Sign() <= 0 {
		return Call{}, errors.New("amount must be positive")
	}
	return c.pack("depositInsurance", marketID, amount)
}

// BuildClaimUnpaid encodes claimUnpaid(marketId), which pays down a deferred
// profit from the pool's free liquidity.
func (c *Client) BuildClaimUnpaid(marketID common.Hash) (Call, error) {
	return c.pack("claimUnpaid", marketID)
}

// BuildCreateMarket encodes createMarket(...). Owner-only on-chain.
func (c *Client) BuildCreateMarket(
	marketID common.Hash,
	token, oracleAsset common.Address,
	takerFee, makerFee, initialMargin, maintenanceMargin, liquidationFee, maxLeverage, maxOpenInterest, minOrderSize, lotSize *big.Int,
) (Call, error) {
	return c.pack("createMarket", marketID, token, oracleAsset, takerFee, makerFee, initialMargin, maintenanceMargin, liquidationFee, maxLeverage, maxOpenInterest, minOrderSize, lotSize)
}

// BuildSetMarketActive encodes setMarketActive(marketId, active). Owner-only.
func (c *Client) BuildSetMarketActive(marketID common.Hash, active bool) (Call, error) {
	return c.pack("setMarketActive", marketID, active)
}

// BuildUpdatePrice encodes updatePrice(marketId, markPrice, indexPrice). Oracle-only.
func (c *Client) BuildUpdatePrice(marketID common.Hash, markPrice, indexPrice *big.Int) (Call, error) {
	if markPrice == nil || markPrice.Sign() <= 0 || indexPrice == nil || indexPrice.Sign() <= 0 {
		return Call{}, errors.New("mark and index price must be positive")
	}
	return c.pack("updatePrice", marketID, markPrice, indexPrice)
}

// BuildAccrueFunding encodes accrueFunding(marketId, markPrice, indexPrice). Oracle-only.
func (c *Client) BuildAccrueFunding(marketID common.Hash, markPrice, indexPrice *big.Int) (Call, error) {
	if markPrice == nil || markPrice.Sign() <= 0 || indexPrice == nil || indexPrice.Sign() <= 0 {
		return Call{}, errors.New("mark and index price must be positive")
	}
	return c.pack("accrueFunding", marketID, markPrice, indexPrice)
}

func (c *Client) pack(method string, args ...interface{}) (Call, error) {
	data, err := orderBookABI.Pack(method, args...)
	if err != nil {
		return Call{}, fmt.Errorf("pack %s: %w", method, err)
	}
	return Call{To: c.deployment.OrderBook, Data: data, Value: big.NewInt(0)}, nil
}

// OrderMarket reads the market an order belongs to. An order placed by a trader
// may belong to any market, so this is how the API labels an order without the
// caller supplying the market.
func (c *Client) OrderMarket(ctx context.Context, orderID *big.Int) (common.Hash, error) {
	out, err := c.call(ctx, "orderMarket", orderID)
	if err != nil {
		return common.Hash{}, err
	}
	var id common.Hash
	if err := orderBookABI.UnpackIntoInterface(&id, "orderMarket", out); err != nil {
		return common.Hash{}, fmt.Errorf("decode orderMarket: %w", err)
	}
	return id, nil
}
