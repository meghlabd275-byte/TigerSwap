// Package keeper provides automated keeper service for liquidations and order execution
package main

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"log"
	"math/big"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/gin-gonic/gin"
)

// KeeperConfig holds keeper configuration
type KeeperConfig struct {
	PrivateKey        string
	RPCURLs           []string
	HealthCheckPort   int
	ExecutionInterval time.Duration
	SyncInterval      time.Duration
	MaxGasPrice       *big.Int
	PerpsAddress      string
	OrdersAddress     string
}

// Position mirrors an on-chain TigerSwap perpetual position.
type Position struct {
	ID         common.Hash
	MarketID   common.Hash
	Trader     common.Address
	Size       *big.Int
	Collateral *big.Int
	EntryPrice *big.Int
	LastUpdate *big.Int
	IsLong     bool
}

// Order mirrors an on-chain TigerSwap advanced order.
type Order struct {
	ID                 *big.Int
	Owner              common.Address
	OrderType          uint8
	Side               uint8
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
	Status             uint8
	TwapInterval       *big.Int
	TwapParts          *big.Int
	TwapCompletedParts *big.Int
	LastFillAt         *big.Int
}

// Order types, as declared by TigerSwapAdvancedOrders.
const (
	orderTypeMarket     = 0
	orderTypeLimit      = 1
	orderTypeStopLoss   = 2
	orderTypeTakeProfit = 3
	orderTypeTWAP       = 4
	orderTypeGTC        = 5
	orderTypeIOC        = 6
	orderTypePostOnly   = 7

	orderSideBuy  = 0
	orderSideSell = 1

	orderStatusOpen            = 1
	orderStatusPartiallyFilled = 2
)

// KeeperStats holds keeper statistics
type KeeperStats struct {
	LiquidationsExecuted uint64
	OrdersExecuted       uint64
	FailedExecutions     uint64
	TotalGasUsed         *big.Int
}

// Keeper represents the keeper service
type Keeper struct {
	config    *KeeperConfig
	clients   map[int64]*ethclient.Client
	chainID   int64
	key       *ecdsa.PrivateKey
	address   common.Address
	stats     KeeperStats
	mu        sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	positions map[common.Hash]*Position
	orders    map[string]*Order

	perpsABI  abi.ABI
	ordersABI abi.ABI
	oracleABI abi.ABI
}

const perpsABIJSON = `[
 {"inputs":[{"internalType":"bytes32","name":"positionId","type":"bytes32"}],"name":"getPosition","outputs":[{"internalType":"bytes32","name":"marketId","type":"bytes32"},{"internalType":"address","name":"trader","type":"address"},{"internalType":"uint256","name":"size","type":"uint256"},{"internalType":"uint256","name":"collateral","type":"uint256"},{"internalType":"uint256","name":"entryPrice","type":"uint256"},{"internalType":"uint256","name":"lastUpdated","type":"uint256"},{"internalType":"bool","name":"isLong","type":"bool"}],"stateMutability":"view","type":"function"},
 {"inputs":[{"internalType":"bytes32","name":"marketId","type":"bytes32"}],"name":"getMarkPrice","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},
 {"inputs":[{"internalType":"bytes32","name":"marketId","type":"bytes32"}],"name":"getMarket","outputs":[{"components":[{"internalType":"address","name":"token","type":"address"},{"internalType":"uint256","name":"maxLeverage","type":"uint256"},{"internalType":"uint256","name":"liquidationFee","type":"uint256"},{"internalType":"uint256","name":"maintenanceMargin","type":"uint256"},{"internalType":"uint256","name":"initialMargin","type":"uint256"},{"internalType":"bool","name":"isActive","type":"bool"},{"internalType":"uint256","name":"openFee","type":"uint256"},{"internalType":"uint256","name":"maxOpenInterest","type":"uint256"},{"internalType":"uint256","name":"openInterest","type":"uint256"}],"internalType":"struct TigerSwapPerpetuals.MarketView","name":"","type":"tuple"}],"stateMutability":"view","type":"function"},
 {"inputs":[{"internalType":"bytes32","name":"positionId","type":"bytes32"}],"name":"liquidatePosition","outputs":[],"stateMutability":"nonpayable","type":"function"},
 {"anonymous":false,"inputs":[{"indexed":true,"internalType":"bytes32","name":"marketId","type":"bytes32"},{"indexed":true,"internalType":"address","name":"trader","type":"address"},{"indexed":false,"internalType":"bytes32","name":"positionId","type":"bytes32"},{"indexed":false,"internalType":"uint256","name":"size","type":"uint256"},{"indexed":false,"internalType":"uint256","name":"collateral","type":"uint256"},{"indexed":false,"internalType":"bool","name":"isLong","type":"bool"}],"name":"PositionOpened","type":"event"},
 {"anonymous":false,"inputs":[{"indexed":true,"internalType":"bytes32","name":"marketId","type":"bytes32"},{"indexed":true,"internalType":"address","name":"trader","type":"address"},{"indexed":false,"internalType":"bytes32","name":"positionId","type":"bytes32"},{"indexed":false,"internalType":"int256","name":"pnl","type":"int256"}],"name":"PositionClosed","type":"event"},
 {"anonymous":false,"inputs":[{"indexed":true,"internalType":"bytes32","name":"marketId","type":"bytes32"},{"indexed":true,"internalType":"address","name":"trader","type":"address"},{"indexed":false,"internalType":"bytes32","name":"positionId","type":"bytes32"},{"indexed":false,"internalType":"address","name":"liquidator","type":"address"},{"indexed":false,"internalType":"uint256","name":"fee","type":"uint256"}],"name":"PositionLiquidated","type":"event"}
]`

const ordersABIJSON = `[
 {"inputs":[{"internalType":"uint256","name":"orderId","type":"uint256"}],"name":"getOrder","outputs":[{"components":[{"internalType":"uint256","name":"id","type":"uint256"},{"internalType":"address","name":"owner","type":"address"},{"internalType":"uint8","name":"orderType","type":"uint8"},{"internalType":"uint8","name":"side","type":"uint8"},{"internalType":"address","name":"tokenIn","type":"address"},{"internalType":"address","name":"tokenOut","type":"address"},{"internalType":"uint256","name":"amountIn","type":"uint256"},{"internalType":"uint256","name":"amountOutMin","type":"uint256"},{"internalType":"uint256","name":"price","type":"uint256"},{"internalType":"uint256","name":"stopPrice","type":"uint256"},{"internalType":"uint256","name":"filledAmount","type":"uint256"},{"internalType":"uint256","name":"amountOutReceived","type":"uint256"},{"internalType":"uint256","name":"createdAt","type":"uint256"},{"internalType":"uint256","name":"expiresAt","type":"uint256"},{"internalType":"uint8","name":"status","type":"uint8"},{"internalType":"uint256","name":"twapInterval","type":"uint256"},{"internalType":"uint256","name":"twapParts","type":"uint256"},{"internalType":"uint256","name":"twapCompletedParts","type":"uint256"},{"internalType":"uint256","name":"lastFillAt","type":"uint256"}],"internalType":"struct TigerSwapAdvancedOrders.Order","name":"","type":"tuple"}],"stateMutability":"view","type":"function"},
 {"inputs":[],"name":"priceOracle","outputs":[{"internalType":"address","name":"","type":"address"}],"stateMutability":"view","type":"function"},
 {"inputs":[],"name":"priceAsset","outputs":[{"internalType":"address","name":"","type":"address"}],"stateMutability":"view","type":"function"},
 {"inputs":[{"internalType":"uint256","name":"orderId","type":"uint256"},{"internalType":"uint256","name":"fillAmount","type":"uint256"}],"name":"executeOrder","outputs":[{"internalType":"uint256","name":"amountOut","type":"uint256"}],"stateMutability":"nonpayable","type":"function"},
 {"inputs":[{"internalType":"uint256","name":"orderId","type":"uint256"}],"name":"triggerOrder","outputs":[],"stateMutability":"nonpayable","type":"function"},
 {"anonymous":false,"inputs":[{"indexed":true,"internalType":"uint256","name":"orderId","type":"uint256"},{"indexed":true,"internalType":"address","name":"owner","type":"address"},{"indexed":false,"internalType":"uint8","name":"orderType","type":"uint8"},{"indexed":false,"internalType":"uint8","name":"side","type":"uint8"},{"indexed":false,"internalType":"address","name":"tokenIn","type":"address"},{"indexed":false,"internalType":"address","name":"tokenOut","type":"address"},{"indexed":false,"internalType":"uint256","name":"amountIn","type":"uint256"},{"indexed":false,"internalType":"uint256","name":"price","type":"uint256"}],"name":"OrderCreated","type":"event"}
]`

const oracleABIJSON = `[
 {"inputs":[{"internalType":"address","name":"token","type":"address"}],"name":"getPrice","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"}
]`

func mustABI(s string) abi.ABI {
	parsed, err := abi.JSON(strings.NewReader(s))
	if err != nil {
		panic(fmt.Sprintf("invalid ABI: %v", err))
	}
	return parsed
}

// onchainPosition is the decoded form of TigerSwapPerpetuals.getPosition.
type onchainPosition struct {
	MarketID    [32]byte
	Trader      common.Address
	Size        *big.Int
	Collateral  *big.Int
	EntryPrice  *big.Int
	LastUpdated *big.Int
	IsLong      bool
}

// marketView is the decoded form of TigerSwapPerpetuals.getMarket.
type marketView struct {
	Token             common.Address
	MaxLeverage       *big.Int
	LiquidationFee    *big.Int
	MaintenanceMargin *big.Int
	InitialMargin     *big.Int
	IsActive          bool
	OpenFee           *big.Int
	MaxOpenInterest   *big.Int
	OpenInterest      *big.Int
}

// onchainOrder is the decoded form of TigerSwapAdvancedOrders.getOrder.
type onchainOrder struct {
	ID                 *big.Int
	Owner              common.Address
	OrderType          uint8
	Side               uint8
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
	Status             uint8
	TwapInterval       *big.Int
	TwapParts          *big.Int
	TwapCompletedParts *big.Int
	LastFillAt         *big.Int
}

// NewKeeper dials the configured TigerSwap nodes and prepares the keeper.
func NewKeeper(config *KeeperConfig) (*Keeper, error) {
	if config.PerpsAddress == "" {
		return nil, fmt.Errorf("TIGERSWAP_PERPS_ADDRESS is required")
	}
	if config.OrdersAddress == "" {
		return nil, fmt.Errorf("TIGERSWAP_ORDERS_ADDRESS is required")
	}

	ctx, cancel := context.WithCancel(context.Background())

	key, err := crypto.HexToECDSA(strings.TrimPrefix(config.PrivateKey, "0x"))
	if err != nil {
		cancel()
		return nil, fmt.Errorf("invalid private key: %w", err)
	}
	address := crypto.PubkeyToAddress(key.PublicKey)

	clients := make(map[int64]*ethclient.Client)
	var chainID int64
	for _, rpcURL := range config.RPCURLs {
		client, err := ethclient.Dial(rpcURL)
		if err != nil {
			log.Printf("Warning: failed to connect to RPC %s: %v", rpcURL, err)
			continue
		}
		id, err := client.ChainID(ctx)
		if err != nil {
			log.Printf("Warning: RPC %s did not report a chain id: %v", rpcURL, err)
			client.Close()
			continue
		}
		clients[id.Int64()] = client
		if chainID == 0 {
			chainID = id.Int64()
		}
	}
	if len(clients) == 0 {
		cancel()
		return nil, fmt.Errorf("no valid RPC URLs")
	}

	return &Keeper{
		config:    config,
		clients:   clients,
		chainID:   chainID,
		key:       key,
		address:   address,
		ctx:       ctx,
		cancel:    cancel,
		positions: make(map[common.Hash]*Position),
		orders:    make(map[string]*Order),
		perpsABI:  mustABI(perpsABIJSON),
		ordersABI: mustABI(ordersABIJSON),
		oracleABI: mustABI(oracleABIJSON),
		stats:     KeeperStats{TotalGasUsed: big.NewInt(0)},
	}, nil
}

func (k *Keeper) client() *ethclient.Client { return k.clients[k.chainID] }

func (k *Keeper) perpsAddress() common.Address  { return common.HexToAddress(k.config.PerpsAddress) }
func (k *Keeper) ordersAddress() common.Address { return common.HexToAddress(k.config.OrdersAddress) }

// Start starts the keeper service
func (k *Keeper) Start() {
	log.Printf("Starting Keeper service with address: %s", k.address.Hex())

	k.syncPositions()
	k.syncOrders()

	k.wg.Add(1)
	go k.monitorLiquidations()

	k.wg.Add(1)
	go k.monitorOrders()

	k.wg.Add(1)
	go k.healthCheck()

	k.wg.Add(1)
	go k.reportStats()

	log.Println("Keeper service started")
}

// Stop stops the keeper service
func (k *Keeper) Stop() {
	log.Println("Stopping Keeper service...")
	k.cancel()
	k.wg.Wait()
	log.Println("Keeper service stopped")
}

func (k *Keeper) monitorLiquidations() {
	defer k.wg.Done()

	exec := time.NewTicker(k.config.ExecutionInterval)
	defer exec.Stop()
	sync := time.NewTicker(k.config.SyncInterval)
	defer sync.Stop()

	for {
		select {
		case <-k.ctx.Done():
			return
		case <-exec.C:
			k.checkLiquidations()
		case <-sync.C:
			k.syncPositions()
		}
	}
}

// syncPositions discovers open positions by replaying PositionOpened logs from
// TigerSwap's own perps contract. Nothing is fabricated: ids come from the chain.
func (k *Keeper) syncPositions() {
	ctx, cancel := context.WithTimeout(k.ctx, 60*time.Second)
	defer cancel()

	topic := k.perpsABI.Events["PositionOpened"].ID
	logs, err := k.client().FilterLogs(ctx, ethereum.FilterQuery{
		FromBlock: big.NewInt(0),
		ToBlock:   nil,
		Addresses: []common.Address{k.perpsAddress()},
		Topics:    [][]common.Hash{{topic}},
	})
	if err != nil {
		log.Printf("Keeper: position log scan failed: %v", err)
		return
	}

	added := 0
	for _, lg := range logs {
		vals, err := k.perpsABI.Unpack("PositionOpened", lg.Data)
		if err != nil || len(vals) < 1 {
			continue
		}
		id, ok := vals[0].([32]byte)
		if !ok {
			continue
		}
		key := common.Hash(id)
		k.mu.Lock()
		if _, exists := k.positions[key]; !exists {
			k.positions[key] = &Position{ID: key}
			added++
		}
		k.mu.Unlock()
	}
	if added > 0 {
		log.Printf("Keeper: discovered %d new position(s)", added)
	}
}

// syncOrders discovers orders by replaying OrderCreated logs.
func (k *Keeper) syncOrders() {
	ctx, cancel := context.WithTimeout(k.ctx, 60*time.Second)
	defer cancel()

	topic := k.ordersABI.Events["OrderCreated"].ID
	logs, err := k.client().FilterLogs(ctx, ethereum.FilterQuery{
		FromBlock: big.NewInt(0),
		ToBlock:   nil,
		Addresses: []common.Address{k.ordersAddress()},
		Topics:    [][]common.Hash{{topic}},
	})
	if err != nil {
		log.Printf("Keeper: order log scan failed: %v", err)
		return
	}

	added := 0
	for _, lg := range logs {
		if len(lg.Topics) < 2 {
			continue
		}
		id := new(big.Int).SetBytes(lg.Topics[1].Bytes())
		key := id.String()
		k.mu.Lock()
		if _, exists := k.orders[key]; !exists {
			k.orders[key] = &Order{ID: id}
			added++
		}
		k.mu.Unlock()
	}
	if added > 0 {
		log.Printf("Keeper: discovered %d new order(s)", added)
	}
}

func (k *Keeper) checkLiquidations() {
	k.mu.RLock()
	ids := make([]common.Hash, 0, len(k.positions))
	for id := range k.positions {
		ids = append(ids, id)
	}
	k.mu.RUnlock()

	ctx, cancel := context.WithTimeout(k.ctx, 30*time.Second)
	defer cancel()

	for _, id := range ids {
		pos, err := k.fetchPosition(ctx, id)
		if err != nil {
			log.Printf("Keeper: position %s read failed: %v", id.Hex(), err)
			continue
		}
		if pos == nil {
			// Closed or liquidated; drop it from the working set.
			k.mu.Lock()
			delete(k.positions, id)
			k.mu.Unlock()
			continue
		}

		market, err := k.fetchMarket(ctx, pos.MarketID)
		if err != nil {
			log.Printf("Keeper: market %s read failed: %v", pos.MarketID.Hex(), err)
			continue
		}
		price, err := k.fetchMarkPrice(ctx, pos.MarketID)
		if err != nil {
			log.Printf("Keeper: mark price for %s unavailable: %v", pos.MarketID.Hex(), err)
			continue
		}

		if belowMaintenance(pos, market, price) {
			log.Printf("Liquidating position %s for %s", id.Hex(), pos.Trader.Hex())
			k.liquidate(ctx, id)
		}
	}
}

// belowMaintenance mirrors TigerSwapPerpetuals.liquidatePosition's health check.
func belowMaintenance(pos *Position, market *marketView, currentPrice *big.Int) bool {
	if pos.Size.Sign() == 0 || pos.Collateral.Sign() == 0 {
		return false
	}

	priceDiff := new(big.Int).Sub(currentPrice, pos.EntryPrice)
	pnl := new(big.Int).Mul(priceDiff, pos.Size)
	pnl.Div(pnl, big.NewInt(1e18))
	if !pos.IsLong {
		pnl.Neg(pnl)
	}

	equity := new(big.Int).Add(pos.Collateral, pnl)
	if equity.Sign() <= 0 {
		return true
	}

	notional := new(big.Int).Mul(pos.Size, pos.EntryPrice)
	notional.Div(notional, big.NewInt(1e18))
	if notional.Sign() == 0 {
		return false
	}

	ratio := new(big.Int).Mul(equity, big.NewInt(1e18))
	ratio.Div(ratio, notional)
	return ratio.Cmp(market.MaintenanceMargin) < 0
}

// fetchPosition reads a position from the chain. It returns (nil, nil) when the
// position no longer exists (size == 0), which is how closed and liquidated
// positions appear on-chain.
func (k *Keeper) fetchPosition(ctx context.Context, id common.Hash) (*Position, error) {
	data, err := k.perpsABI.Pack("getPosition", id)
	if err != nil {
		return nil, err
	}
	to := k.perpsAddress()
	out, err := k.client().CallContract(ctx, ethereum.CallMsg{To: &to, Data: data}, nil)
	if err != nil {
		return nil, err
	}
	var raw onchainPosition
	if err := k.perpsABI.UnpackIntoInterface(&raw, "getPosition", out); err != nil {
		return nil, err
	}
	if raw.Size == nil || raw.Size.Sign() == 0 {
		return nil, nil
	}
	return &Position{
		ID:         id,
		MarketID:   common.Hash(raw.MarketID),
		Trader:     raw.Trader,
		Size:       raw.Size,
		Collateral: raw.Collateral,
		EntryPrice: raw.EntryPrice,
		LastUpdate: raw.LastUpdated,
		IsLong:     raw.IsLong,
	}, nil
}

func (k *Keeper) fetchMarket(ctx context.Context, marketID common.Hash) (*marketView, error) {
	data, err := k.perpsABI.Pack("getMarket", marketID)
	if err != nil {
		return nil, err
	}
	to := k.perpsAddress()
	out, err := k.client().CallContract(ctx, ethereum.CallMsg{To: &to, Data: data}, nil)
	if err != nil {
		return nil, err
	}
	var mv marketView
	if err := k.perpsABI.UnpackIntoInterface(&mv, "getMarket", out); err != nil {
		return nil, err
	}
	return &mv, nil
}

func (k *Keeper) fetchMarkPrice(ctx context.Context, marketID common.Hash) (*big.Int, error) {
	data, err := k.perpsABI.Pack("getMarkPrice", marketID)
	if err != nil {
		return nil, err
	}
	to := k.perpsAddress()
	out, err := k.client().CallContract(ctx, ethereum.CallMsg{To: &to, Data: data}, nil)
	if err != nil {
		return nil, err
	}
	vals, err := k.perpsABI.Unpack("getMarkPrice", out)
	if err != nil || len(vals) < 1 {
		return nil, fmt.Errorf("unpack getMarkPrice: %w", err)
	}
	price, ok := vals[0].(*big.Int)
	if !ok || price.Sign() == 0 {
		return nil, fmt.Errorf("market %s has no mark price", marketID.Hex())
	}
	return price, nil
}

// liquidate submits a real liquidatePosition transaction and records its hash.
func (k *Keeper) liquidate(ctx context.Context, id common.Hash) {
	data, err := k.perpsABI.Pack("liquidatePosition", id)
	if err != nil {
		log.Printf("Liquidation skipped for position %s: encode: %v", id.Hex(), err)
		k.fail()
		return
	}
	tx, err := k.send(ctx, k.perpsAddress(), data)
	if err != nil {
		log.Printf("Liquidation failed for position %s: %v", id.Hex(), err)
		k.fail()
		return
	}
	k.mu.Lock()
	k.stats.LiquidationsExecuted++
	k.mu.Unlock()
	log.Printf("Liquidation submitted: position=%s tx=%s", id.Hex(), tx.Hash().Hex())
}

func (k *Keeper) monitorOrders() {
	defer k.wg.Done()

	exec := time.NewTicker(k.config.ExecutionInterval)
	defer exec.Stop()
	sync := time.NewTicker(k.config.SyncInterval)
	defer sync.Stop()

	for {
		select {
		case <-k.ctx.Done():
			return
		case <-exec.C:
			k.checkOrders()
		case <-sync.C:
			k.syncOrders()
		}
	}
}

func (k *Keeper) checkOrders() {
	k.mu.RLock()
	ids := make([]*big.Int, 0, len(k.orders))
	for _, o := range k.orders {
		ids = append(ids, o.ID)
	}
	k.mu.RUnlock()

	ctx, cancel := context.WithTimeout(k.ctx, 30*time.Second)
	defer cancel()

	price, err := k.oraclePrice(ctx)
	if err != nil {
		log.Printf("Keeper: oracle price unavailable, skipping order check: %v", err)
		return
	}

	for _, id := range ids {
		order, err := k.fetchOrder(ctx, id)
		if err != nil {
			log.Printf("Keeper: order %s read failed: %v", id.String(), err)
			continue
		}
		if order == nil || (order.Status != orderStatusOpen && order.Status != orderStatusPartiallyFilled) {
			k.mu.Lock()
			delete(k.orders, id.String())
			k.mu.Unlock()
			continue
		}
		if order.ExpiresAt != nil && order.ExpiresAt.Sign() > 0 && order.ExpiresAt.Cmp(big.NewInt(time.Now().Unix())) < 0 {
			continue
		}

		if order.OrderType == orderTypeStopLoss || order.OrderType == orderTypeTakeProfit {
			if triggerConditionMet(order, price) {
				k.triggerOrder(ctx, id)
			}
			continue
		}
		if priceConditionMet(order, price) {
			k.executeOrder(ctx, id)
		}
	}
}

// oraclePrice reads the price the order contract itself evaluates against.
func (k *Keeper) oraclePrice(ctx context.Context) (*big.Int, error) {
	to := k.ordersAddress()
	oracleOut, err := k.client().CallContract(ctx, ethereum.CallMsg{To: &to, Data: mustPack(k.ordersABI, "priceOracle")}, nil)
	if err != nil {
		return nil, fmt.Errorf("priceOracle: %w", err)
	}
	oracleVals, err := k.ordersABI.Unpack("priceOracle", oracleOut)
	if err != nil || len(oracleVals) < 1 {
		return nil, fmt.Errorf("unpack priceOracle: %w", err)
	}
	oracle, ok := oracleVals[0].(common.Address)
	if !ok || oracle == (common.Address{}) {
		return nil, fmt.Errorf("orders contract has no price oracle configured")
	}

	assetOut, err := k.client().CallContract(ctx, ethereum.CallMsg{To: &to, Data: mustPack(k.ordersABI, "priceAsset")}, nil)
	if err != nil {
		return nil, fmt.Errorf("priceAsset: %w", err)
	}
	assetVals, err := k.ordersABI.Unpack("priceAsset", assetOut)
	if err != nil || len(assetVals) < 1 {
		return nil, fmt.Errorf("unpack priceAsset: %w", err)
	}
	asset, ok := assetVals[0].(common.Address)
	if !ok {
		return nil, fmt.Errorf("unexpected priceAsset type")
	}

	priceOut, err := k.client().CallContract(ctx, ethereum.CallMsg{
		To:   &oracle,
		Data: mustPack(k.oracleABI, "getPrice", asset),
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("oracle.getPrice: %w", err)
	}
	priceVals, err := k.oracleABI.Unpack("getPrice", priceOut)
	if err != nil || len(priceVals) < 1 {
		return nil, fmt.Errorf("unpack getPrice: %w", err)
	}
	price, ok := priceVals[0].(*big.Int)
	if !ok || price.Sign() == 0 {
		return nil, fmt.Errorf("oracle returned no usable price")
	}
	return price, nil
}

// priceConditionMet mirrors TigerSwapAdvancedOrders._checkPriceCondition.
func priceConditionMet(o *Order, currentPrice *big.Int) bool {
	switch o.OrderType {
	case orderTypeMarket, orderTypeStopLoss, orderTypeTakeProfit, orderTypeTWAP:
		return true
	case orderTypeLimit, orderTypeGTC, orderTypeIOC:
		if o.Side == orderSideBuy {
			return currentPrice.Cmp(o.Price) <= 0
		}
		return currentPrice.Cmp(o.Price) >= 0
	case orderTypePostOnly:
		if o.Side == orderSideBuy {
			return currentPrice.Cmp(o.Price) < 0
		}
		return currentPrice.Cmp(o.Price) > 0
	default:
		return false
	}
}

// triggerConditionMet mirrors TigerSwapAdvancedOrders._checkTriggerCondition.
func triggerConditionMet(o *Order, currentPrice *big.Int) bool {
	switch o.OrderType {
	case orderTypeStopLoss:
		if o.Side == orderSideBuy {
			return currentPrice.Cmp(o.StopPrice) >= 0
		}
		return currentPrice.Cmp(o.StopPrice) <= 0
	case orderTypeTakeProfit:
		if o.Side == orderSideSell {
			return currentPrice.Cmp(o.Price) >= 0
		}
		return currentPrice.Cmp(o.Price) <= 0
	default:
		return false
	}
}

// fetchOrder reads an order from the chain. It returns (nil, nil) for unknown
// orders (id == 0), which is how the contract reports an id that was never used.
func (k *Keeper) fetchOrder(ctx context.Context, id *big.Int) (*Order, error) {
	data, err := k.ordersABI.Pack("getOrder", id)
	if err != nil {
		return nil, err
	}
	to := k.ordersAddress()
	out, err := k.client().CallContract(ctx, ethereum.CallMsg{To: &to, Data: data}, nil)
	if err != nil {
		return nil, err
	}
	var raw onchainOrder
	if err := k.ordersABI.UnpackIntoInterface(&raw, "getOrder", out); err != nil {
		return nil, err
	}
	if raw.ID == nil || raw.ID.Sign() == 0 {
		return nil, nil
	}
	return &Order{
		ID:                 raw.ID,
		Owner:              raw.Owner,
		OrderType:          raw.OrderType,
		Side:               raw.Side,
		TokenIn:            raw.TokenIn,
		TokenOut:           raw.TokenOut,
		AmountIn:           raw.AmountIn,
		AmountOutMin:       raw.AmountOutMin,
		Price:              raw.Price,
		StopPrice:          raw.StopPrice,
		FilledAmount:       raw.FilledAmount,
		AmountOutReceived:  raw.AmountOutReceived,
		CreatedAt:          raw.CreatedAt,
		ExpiresAt:          raw.ExpiresAt,
		Status:             raw.Status,
		TwapInterval:       raw.TwapInterval,
		TwapParts:          raw.TwapParts,
		TwapCompletedParts: raw.TwapCompletedParts,
		LastFillAt:         raw.LastFillAt,
	}, nil
}

func (k *Keeper) executeOrder(ctx context.Context, id *big.Int) {
	data, err := k.ordersABI.Pack("executeOrder", id, big.NewInt(0))
	if err != nil {
		log.Printf("Order execution skipped for %s: encode: %v", id.String(), err)
		k.fail()
		return
	}
	tx, err := k.send(ctx, k.ordersAddress(), data)
	if err != nil {
		log.Printf("Order execution failed for %s: %v", id.String(), err)
		k.fail()
		return
	}
	k.mu.Lock()
	k.stats.OrdersExecuted++
	k.mu.Unlock()
	log.Printf("Order executed: order=%s tx=%s", id.String(), tx.Hash().Hex())
}

func (k *Keeper) triggerOrder(ctx context.Context, id *big.Int) {
	data, err := k.ordersABI.Pack("triggerOrder", id)
	if err != nil {
		log.Printf("Order trigger skipped for %s: encode: %v", id.String(), err)
		k.fail()
		return
	}
	tx, err := k.send(ctx, k.ordersAddress(), data)
	if err != nil {
		log.Printf("Order trigger failed for %s: %v", id.String(), err)
		k.fail()
		return
	}
	k.mu.Lock()
	k.stats.OrdersExecuted++
	k.mu.Unlock()
	log.Printf("Order triggered: order=%s tx=%s", id.String(), tx.Hash().Hex())
}

// send signs and broadcasts a keeper transaction, enforcing the gas cap.
func (k *Keeper) send(ctx context.Context, to common.Address, data []byte) (*types.Transaction, error) {
	client := k.client()

	gasPrice, err := client.SuggestGasPrice(ctx)
	if err != nil {
		return nil, fmt.Errorf("gas price: %w", err)
	}
	if k.config.MaxGasPrice != nil && gasPrice.Cmp(k.config.MaxGasPrice) > 0 {
		return nil, fmt.Errorf("gas price %s exceeds cap %s", gasPrice, k.config.MaxGasPrice)
	}

	gasLimit, err := client.EstimateGas(ctx, ethereum.CallMsg{From: k.address, To: &to, Data: data})
	if err != nil {
		return nil, fmt.Errorf("estimate gas: %w", err)
	}

	nonce, err := client.PendingNonceAt(ctx, k.address)
	if err != nil {
		return nil, fmt.Errorf("nonce: %w", err)
	}

	chainID, err := client.ChainID(ctx)
	if err != nil {
		return nil, fmt.Errorf("chain id: %w", err)
	}

	tx := types.NewTransaction(nonce, to, big.NewInt(0), gasLimit, gasPrice, data)
	signed, err := types.SignTx(tx, types.LatestSignerForChainID(chainID), k.key)
	if err != nil {
		return nil, fmt.Errorf("sign: %w", err)
	}
	if err := client.SendTransaction(ctx, signed); err != nil {
		return nil, fmt.Errorf("broadcast: %w", err)
	}

	k.mu.Lock()
	k.stats.TotalGasUsed.Add(k.stats.TotalGasUsed, new(big.Int).Mul(gasPrice, new(big.Int).SetUint64(gasLimit)))
	k.mu.Unlock()

	return signed, nil
}

func (k *Keeper) fail() {
	k.mu.Lock()
	k.stats.FailedExecutions++
	k.mu.Unlock()
}

func mustPack(a abi.ABI, name string, args ...interface{}) []byte {
	data, err := a.Pack(name, args...)
	if err != nil {
		panic(fmt.Sprintf("pack %s: %v", name, err))
	}
	return data
}

func (k *Keeper) healthCheck() {
	defer k.wg.Done()

	r := gin.Default()

	r.GET("/health", func(c *gin.Context) {
		k.mu.RLock()
		stats := k.stats
		k.mu.RUnlock()

		c.JSON(200, gin.H{
			"status":        "healthy",
			"address":       k.address.Hex(),
			"chain_id":      k.chainID,
			"positions":     len(k.positions),
			"orders":        len(k.orders),
			"liquidations":  stats.LiquidationsExecuted,
			"orders_filled": stats.OrdersExecuted,
			"failed":        stats.FailedExecutions,
		})
	})

	r.GET("/stats", func(c *gin.Context) {
		k.mu.RLock()
		stats := k.stats
		k.mu.RUnlock()
		c.JSON(200, stats)
	})

	addr := fmt.Sprintf(":%d", k.config.HealthCheckPort)
	log.Printf("Starting health check server on %s", addr)

	if err := r.Run(addr); err != nil {
		log.Printf("Health check server error: %v", err)
	}
}

func (k *Keeper) reportStats() {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-k.ctx.Done():
			return
		case <-ticker.C:
			k.mu.RLock()
			stats := k.stats
			k.mu.RUnlock()

			log.Printf("Keeper Stats - Liquidations: %d, Orders: %d, Failed: %d",
				stats.LiquidationsExecuted, stats.OrdersExecuted, stats.FailedExecutions)
		}
	}
}

func getEnv(key, defaultValue string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return defaultValue
}

func getEnvBigInt(key string, defaultValue *big.Int) *big.Int {
	if value, ok := os.LookupEnv(key); ok {
		if v, ok := new(big.Int).SetString(value, 10); ok {
			return v
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value, ok := os.LookupEnv(key); ok {
		if d, err := time.ParseDuration(value); err == nil {
			return d
		}
	}
	return defaultValue
}

func main() {
	rpcURL := os.Getenv("RPC_URL")
	if rpcURL == "" {
		// TigerSwap runs its own nodes; there is no third-party RPC fallback.
		log.Fatal("RPC_URL is required: point it at a TigerSwap node")
	}

	config := &KeeperConfig{
		PrivateKey:        getEnv("KEEPER_PRIVATE_KEY", ""),
		RPCURLs:           []string{rpcURL},
		HealthCheckPort:   8095,
		ExecutionInterval: getEnvDuration("KEEPER_EXECUTION_INTERVAL", 1*time.Second),
		SyncInterval:      getEnvDuration("KEEPER_SYNC_INTERVAL", 60*time.Second),
		MaxGasPrice:       getEnvBigInt("MAX_GAS_PRICE", big.NewInt(100000000000)),
		PerpsAddress:      getEnv("TIGERSWAP_PERPS_ADDRESS", ""),
		OrdersAddress:     getEnv("TIGERSWAP_ORDERS_ADDRESS", ""),
	}

	if config.PrivateKey == "" {
		log.Fatal("KEEPER_PRIVATE_KEY is required")
	}

	keeper, err := NewKeeper(config)
	if err != nil {
		log.Fatalf("Failed to create keeper: %v", err)
	}

	keeper.Start()
	defer keeper.Stop()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
}
