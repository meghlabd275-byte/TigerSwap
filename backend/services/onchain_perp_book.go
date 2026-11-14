package services

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"tigerswap/backend/go/orderbook"
	"tigerswap/backend/models"
)

// OnChainPerpBookService exposes TigerSwap's on-chain perpetual order book to the
// API. Orders, matching, margin, funding and liquidation live in the deployed
// TigerSwapPerpOrderBook contract; this service reads chain state, returns
// unsigned calldata for the caller's own wallet to sign, and runs the two
// processes a venue needs — an oracle writer (mark price + funding) and a
// liquidation keeper. The server never holds a user's key.
type OnChainPerpBookService struct {
	db       *gorm.DB
	priceAgg priceSource

	mu      sync.RWMutex
	clients map[int64]*orderbook.Client

	// symbolByMarket maps a market id (bytes32 hex) to its listing symbol so an
	// indexed position can be labelled.
	symbolByMarket map[string]string

	keeperKey *ecdsa.PrivateKey
	ownerKey  *ecdsa.PrivateKey

	running bool
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

// NewOnChainPerpBookService builds the service.
func NewOnChainPerpBookService(db *gorm.DB, priceAgg priceSource) *OnChainPerpBookService {
	return &OnChainPerpBookService{
		db:             db,
		priceAgg:       priceAgg,
		clients:        make(map[int64]*orderbook.Client),
		symbolByMarket: make(map[string]string),
	}
}

// SetKeeperKey wires the signer used for liquidations and oracle updates. It is
// optional: without it the read and order-building endpoints still work, and the
// keeper loop is simply not started.
func (s *OnChainPerpBookService) SetKeeperKey(key *ecdsa.PrivateKey) {
	s.mu.Lock()
	s.keeperKey = key
	clients := s.clients
	s.clients = make(map[int64]*orderbook.Client)
	s.mu.Unlock()
	for _, old := range clients {
		old.Close()
		_ = s.RegisterDeployment(old.Deployment())
	}
}

// SetOwnerKey wires the signer used for owner-only calls (createMarket).
func (s *OnChainPerpBookService) SetOwnerKey(key *ecdsa.PrivateKey) {
	s.mu.Lock()
	s.ownerKey = key
	s.mu.Unlock()
}

// LoadDeployments dials every configured chain and registers its order book.
func (s *OnChainPerpBookService) LoadDeployments() error {
	for _, d := range orderbook.LoadDeployments() {
		if err := s.RegisterDeployment(d); err != nil {
			log.Printf("on-chain perp book: could not register chain %d: %v", d.ChainID, err)
		}
	}
	return nil
}

// RegisterDeployment dials a chain's order book and replaces any existing client.
func (s *OnChainPerpBookService) RegisterDeployment(d orderbook.Deployment) error {
	client, err := orderbook.NewClient(d, s.keeperKey)
	if err != nil {
		return err
	}
	s.mu.Lock()
	if old, ok := s.clients[d.ChainID]; ok {
		old.Close()
	}
	s.clients[d.ChainID] = client
	s.mu.Unlock()
	return nil
}

func (s *OnChainPerpBookService) client(chainID int64) (*orderbook.Client, error) {
	s.mu.RLock()
	client := s.clients[chainID]
	s.mu.RUnlock()
	if client == nil {
		return nil, fmt.Errorf("no on-chain perp order book configured for chain %d", chainID)
	}
	return client, nil
}

// Chains returns the configured chain ids, sorted for stable output.
func (s *OnChainPerpBookService) Chains() []int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ids := make([]int64, 0, len(s.clients))
	for id := range s.clients {
		ids = append(ids, id)
	}
	for i := 1; i < len(ids); i++ {
		for j := i; j > 0 && ids[j-1] > ids[j]; j-- {
			ids[j-1], ids[j] = ids[j], ids[j-1]
		}
	}
	return ids
}

// ReloadSymbols loads the active listings so a market id can be labelled.
func (s *OnChainPerpBookService) ReloadSymbols() error {
	if s.db == nil {
		return nil
	}
	var futures []models.FuturesMarket
	if err := s.db.Where("status = ?", models.ListingStatusActive).Find(&futures).Error; err != nil {
		return err
	}
	var perpsMarkets []models.PerpsMarket
	if err := s.db.Where("status = ?", models.ListingStatusActive).Find(&perpsMarkets).Error; err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, m := range futures {
		s.symbolByMarket[orderbook.MarketID(m.Symbol).Hex()] = m.Symbol
	}
	for _, m := range perpsMarkets {
		s.symbolByMarket[orderbook.MarketID(m.Symbol).Hex()] = m.Symbol
	}
	return nil
}

func (s *OnChainPerpBookService) symbolFor(marketID common.Hash) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.symbolByMarket[marketID.Hex()]
}

func (s *OnChainPerpBookService) resolveChainParam(c *gin.Context) int64 {
	if raw := strings.TrimSpace(c.Query("chain_id")); raw != "" {
		var id int64
		if _, err := fmt.Sscanf(raw, "%d", &id); err == nil && id != 0 {
			return id
		}
	}
	chains := s.Chains()
	if len(chains) == 1 {
		return chains[0]
	}
	return 0
}

func (s *OnChainPerpBookService) walletAddressFor(userID string) (string, error) {
	if s.db == nil {
		return "", errors.New("on-chain perp book service has no database")
	}
	var user models.User
	if err := s.db.Select("wallet_address").First(&user, "id = ?", userID).Error; err != nil {
		return "", fmt.Errorf("resolve wallet: %w", err)
	}
	if strings.TrimSpace(user.WalletAddress) == "" {
		return "", errNoWalletBound
	}
	return user.WalletAddress, nil
}

// marketIDParam parses a market_id query/param, accepting either the hex id or a
// symbol (which is hashed the same way the contract does).
func (s *OnChainPerpBookService) marketIDParam(c *gin.Context, key string) (common.Hash, error) {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		raw = strings.TrimSpace(c.Param(key))
	}
	if raw == "" {
		return common.Hash{}, errors.New("market_id is required")
	}
	if isBytes32(raw) {
		return common.HexToHash(raw), nil
	}
	return orderbook.MarketID(raw), nil
}

// ---- Reads ----

type bookMarketOut struct {
	ChainID           int64  `json:"chain_id"`
	Symbol            string `json:"symbol"`
	MarketID          string `json:"market_id"`
	Token             string `json:"token"`
	OracleAsset       string `json:"oracle_asset"`
	TakerFee          string `json:"taker_fee"`
	MakerFee          string `json:"maker_fee"`
	InitialMargin     string `json:"initial_margin"`
	MaintenanceMargin string `json:"maintenance_margin"`
	LiquidationFee    string `json:"liquidation_fee"`
	MaxLeverage       string `json:"max_leverage"`
	MinOrderSize      string `json:"min_order_size"`
	LotSize           string `json:"lot_size"`
	OpenInterest      string `json:"open_interest"`
	MaxOpenInterest   string `json:"max_open_interest"`
	MarkPrice         string `json:"mark_price"`
	IndexPrice        string `json:"index_price"`
	IsActive          bool   `json:"is_active"`
}

// MarketsHandler lists every configured order book market with its live mark.
func (s *OnChainPerpBookService) MarketsHandler(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
	defer cancel()

	var out []bookMarketOut
	for _, chainID := range s.Chains() {
		client, err := s.client(chainID)
		if err != nil {
			continue
		}
		for _, symbol := range s.bookSymbols() {
			marketID := orderbook.MarketID(symbol)
			market, err := client.Market(ctx, marketID)
			if err != nil || market.Token == (common.Address{}) {
				continue
			}
			funding, _ := client.Funding(ctx, marketID)
			row := bookMarketOut{
				ChainID:           chainID,
				Symbol:            symbol,
				MarketID:          marketID.Hex(),
				Token:             market.Token.Hex(),
				OracleAsset:       market.OracleAsset.Hex(),
				TakerFee:          market.TakerFee.String(),
				MakerFee:          market.MakerFee.String(),
				InitialMargin:     market.InitialMargin.String(),
				MaintenanceMargin: market.MaintenanceMargin.String(),
				LiquidationFee:    market.LiquidationFee.String(),
				MaxLeverage:       market.MaxLeverage.String(),
				MinOrderSize:      market.MinOrderSize.String(),
				LotSize:           market.LotSize.String(),
				OpenInterest:      market.OpenInterest.String(),
				MaxOpenInterest:   market.MaxOpenInterest.String(),
				IsActive:          market.IsActive,
			}
			if funding != nil {
				row.MarkPrice = funding.MarkPrice.String()
				row.IndexPrice = funding.IndexPrice.String()
			}
			out = append(out, row)
		}
	}
	c.JSON(http.StatusOK, gin.H{"markets": out, "count": len(out)})
}

// DepthHandler returns the aggregate order book depth for a market.
func (s *OnChainPerpBookService) DepthHandler(c *gin.Context) {
	marketID, err := s.marketIDParam(c, "market_id")
	if err != nil {
		respondBadRequestError(c, err)
		return
	}
	chainID := s.resolveChainParam(c)
	client, err := s.client(chainID)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
	defer cancel()

	levels := big.NewInt(20)
	bidPrices, bidQty, askPrices, askQty, err := client.Depth(ctx, marketID, levels)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	bidPrice, _, askPrice, _, _ := client.BestBidAsk(ctx, marketID)
	funding, _ := client.Funding(ctx, marketID)
	mark := "0"
	index := "0"
	if funding != nil {
		mark = funding.MarkPrice.String()
		index = funding.IndexPrice.String()
	}
	c.JSON(http.StatusOK, gin.H{
		"chain_id":    chainID,
		"market_id":   marketID.Hex(),
		"symbol":      s.symbolFor(marketID),
		"bid_prices":  bigSliceToStrings(bidPrices),
		"bid_qty":     bigSliceToStrings(bidQty),
		"ask_prices":  bigSliceToStrings(askPrices),
		"ask_qty":     bigSliceToStrings(askQty),
		"best_bid":    bigString(bidPrice),
		"best_ask":    bigString(askPrice),
		"mark_price":  mark,
		"index_price": index,
	})
}

// PositionsHandler returns the caller's order book positions across markets,
// read from chain state. Identity is read from the auth context, never the body.
func (s *OnChainPerpBookService) PositionsHandler(c *gin.Context) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}
	chainID := s.resolveChainParam(c)
	client, err := s.client(chainID)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	address, err := s.walletAddressFor(userID)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	trader := common.HexToAddress(address)

	ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
	defer cancel()

	out := make([]gin.H, 0)
	for _, symbol := range s.bookSymbols() {
		marketID := orderbook.MarketID(symbol)
		market, err := client.Market(ctx, marketID)
		if err != nil || market.Token == (common.Address{}) {
			continue
		}
		pos, err := client.Position(ctx, marketID, trader)
		if err != nil {
			continue
		}
		if pos.Size.Sign() == 0 {
			continue
		}
		out = append(out, s.positionView(ctx, client, chainID, symbol, marketID, trader, pos))
	}
	c.JSON(http.StatusOK, gin.H{"positions": out, "count": len(out), "chain_id": chainID})
}

// OrdersHandler returns the caller's orders for a market, read from chain state.
func (s *OnChainPerpBookService) OrdersHandler(c *gin.Context) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}
	chainID := s.resolveChainParam(c)
	client, err := s.client(chainID)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	address, err := s.walletAddressFor(userID)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	trader := common.HexToAddress(address)

	ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
	defer cancel()

	ids, err := client.TraderOrders(ctx, trader)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	out := make([]gin.H, 0, len(ids))
	for _, id := range ids {
		order, err := client.Order(ctx, id)
		if err != nil {
			continue
		}
		if !order.Active && order.Filled.Sign() == 0 {
			continue
		}
		// getOrder does not carry the market id, so read it from the
		// contract's own orderMarket mapping.
		marketID, err := client.OrderMarket(ctx, id)
		if err != nil {
			continue
		}
		out = append(out, gin.H{
			"order_id":    id.String(),
			"market_id":   marketID.Hex(),
			"symbol":      s.symbolFor(marketID),
			"trader":      order.Trader.Hex(),
			"price":       order.Price.String(),
			"quantity":    order.Quantity.String(),
			"filled":      order.Filled.String(),
			"is_long":     order.IsLong,
			"reduce_only": order.ReduceOnly,
			"post_only":   order.PostOnly,
			"active":      order.Active,
			"created_at":  order.Timestamp.String(),
		})
	}
	c.JSON(http.StatusOK, gin.H{"orders": out, "count": len(out), "chain_id": chainID})
}

// FillsHandler returns the caller's indexed fills for a chain, newest first.
// Each row is written from a confirmed OrderFilled event, so the history is the
// chain's own trade record rather than a reconstructed one.
func (s *OnChainPerpBookService) FillsHandler(c *gin.Context) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}
	chainID := s.resolveChainParam(c)
	address, err := s.walletAddressFor(userID)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	if s.db == nil {
		respondServiceError(c, errors.New("fill index is unavailable"))
		return
	}
	limit := 100
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 && parsed <= 500 {
			limit = parsed
		}
	}
	var rows []models.OnChainBookFill
	if err := s.db.Where("chain_id = ? AND trader = ?", chainID, common.HexToAddress(address).Hex()).
		Order("block_number DESC, log_index DESC").Limit(limit).Find(&rows).Error; err != nil {
		respondServiceError(c, err)
		return
	}
	out := make([]gin.H, 0, len(rows))
	for _, r := range rows {
		out = append(out, gin.H{
			"order_id":     r.OrderID,
			"market_id":    r.MarketID,
			"symbol":       r.Symbol,
			"trader":       r.Trader,
			"is_maker":     r.IsMaker,
			"price":        r.Price,
			"quantity":     r.Quantity,
			"fee":          r.Fee,
			"tx_hash":      r.TxHash,
			"log_index":    r.LogIndex,
			"block_number": r.BlockNumber,
		})
	}
	c.JSON(http.StatusOK, gin.H{"fills": out, "count": len(out), "chain_id": chainID})
}

func (s *OnChainPerpBookService) positionView(ctx context.Context, client *orderbook.Client, chainID int64, symbol string, marketID common.Hash, trader common.Address, pos *orderbook.Position) gin.H {
	side := "flat"
	if pos.Size.Sign() > 0 {
		side = "long"
	} else if pos.Size.Sign() < 0 {
		side = "short"
	}
	// A deferred profit is money the pool owes the trader; report it so the UI
	// can offer the claim rather than hiding it.
	unpaid := new(big.Int)
	if credit, err := client.UnpaidCredit(ctx, marketID, trader); err == nil && credit != nil {
		unpaid = credit
	}
	return gin.H{
		"chain_id":       chainID,
		"symbol":         symbol,
		"market_id":      marketID.Hex(),
		"trader":         trader.Hex(),
		"side":           side,
		"size":           pos.Size.String(),
		"entry_price":    pos.EntryPrice.String(),
		"margin":         pos.Margin.String(),
		"unrealized_pnl": pos.UnrealizedPnL.String(),
		"equity":         pos.Equity.String(),
		"unpaid_credit":  unpaid.String(),
	}
}

// bookSymbols returns the distinct active listing symbols that may have a book
// market. A market with no on-chain entry is filtered out by the caller.
func (s *OnChainPerpBookService) bookSymbols() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	seen := make(map[string]bool)
	out := make([]string, 0, len(s.symbolByMarket))
	for _, symbol := range s.symbolByMarket {
		if symbol == "" || seen[symbol] {
			continue
		}
		seen[symbol] = true
		out = append(out, symbol)
	}
	return out
}

// ---- Builds ----

type bookAmountRequest struct {
	ChainID  int64  `json:"chain_id"`
	MarketID string `json:"market_id"`
	Amount   string `json:"amount"`
}

func (s *OnChainPerpBookService) buildCall(c *gin.Context, build func(client *orderbook.Client, marketID common.Hash) (orderbook.Call, error)) {
	if _, ok := authenticatedUserID(c); !ok {
		return
	}
	var req bookAmountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequestError(c, err)
		return
	}
	marketID, err := s.marketIDParamFromBody(req.MarketID)
	if err != nil {
		respondBadRequestError(c, err)
		return
	}
	chainID := req.ChainID
	if chainID == 0 {
		chainID = s.resolveChainParam(c)
	}
	client, err := s.client(chainID)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	call, err := build(client, marketID)
	if err != nil {
		respondBadRequestError(c, err)
		return
	}
	respondOnChainCall(c, call, chainID)
}

func (s *OnChainPerpBookService) marketIDParamFromBody(raw string) (common.Hash, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return common.Hash{}, errors.New("market_id is required")
	}
	if isBytes32(raw) {
		return common.HexToHash(raw), nil
	}
	return orderbook.MarketID(raw), nil
}

// BuildDepositHandler returns calldata that deposits margin.
func (s *OnChainPerpBookService) BuildDepositHandler(c *gin.Context) {
	s.buildCall(c, func(client *orderbook.Client, marketID common.Hash) (orderbook.Call, error) {
		amount, err := parsePositiveAmount(c)
		if err != nil {
			return orderbook.Call{}, err
		}
		return client.BuildDeposit(marketID, amount)
	})
}

// BuildWithdrawHandler returns calldata that withdraws free margin.
func (s *OnChainPerpBookService) BuildWithdrawHandler(c *gin.Context) {
	s.buildCall(c, func(client *orderbook.Client, marketID common.Hash) (orderbook.Call, error) {
		amount, err := parsePositiveAmount(c)
		if err != nil {
			return orderbook.Call{}, err
		}
		return client.BuildWithdraw(marketID, amount)
	})
}

// BuildAddLiquidityHandler returns calldata that adds pool liquidity.
func (s *OnChainPerpBookService) BuildAddLiquidityHandler(c *gin.Context) {
	s.buildCall(c, func(client *orderbook.Client, marketID common.Hash) (orderbook.Call, error) {
		amount, err := parsePositiveAmount(c)
		if err != nil {
			return orderbook.Call{}, err
		}
		return client.BuildAddLiquidity(marketID, amount)
	})
}

// BuildRemoveLiquidityHandler returns calldata that removes pool liquidity.
func (s *OnChainPerpBookService) BuildRemoveLiquidityHandler(c *gin.Context) {
	s.buildCall(c, func(client *orderbook.Client, marketID common.Hash) (orderbook.Call, error) {
		amount, err := parsePositiveAmount(c)
		if err != nil {
			return orderbook.Call{}, err
		}
		return client.BuildRemoveLiquidity(marketID, amount)
	})
}

// BuildDepositInsuranceHandler returns calldata that tops up the insurance fund.
func (s *OnChainPerpBookService) BuildDepositInsuranceHandler(c *gin.Context) {
	s.buildCall(c, func(client *orderbook.Client, marketID common.Hash) (orderbook.Call, error) {
		amount, err := parsePositiveAmount(c)
		if err != nil {
			return orderbook.Call{}, err
		}
		return client.BuildDepositInsurance(marketID, amount)
	})
}

// BuildClaimUnpaidHandler returns calldata that pays down the caller's deferred
// profit from the pool's free liquidity.
func (s *OnChainPerpBookService) BuildClaimUnpaidHandler(c *gin.Context) {
	s.buildCall(c, func(client *orderbook.Client, marketID common.Hash) (orderbook.Call, error) {
		return client.BuildClaimUnpaid(marketID)
	})
}

type bookPlaceOrderRequest struct {
	ChainID     int64  `json:"chain_id"`
	MarketID    string `json:"market_id"`
	Quantity    string `json:"quantity"`
	Price       string `json:"price"`
	Side        string `json:"side"`
	TimeInForce string `json:"time_in_force"`
	ReduceOnly  bool   `json:"reduce_only"`
}

// BuildPlaceOrderHandler returns calldata that places a limit order.
func (s *OnChainPerpBookService) BuildPlaceOrderHandler(c *gin.Context) {
	if _, ok := authenticatedUserID(c); !ok {
		return
	}
	var req bookPlaceOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequestError(c, err)
		return
	}
	marketID, err := s.marketIDParamFromBody(req.MarketID)
	if err != nil {
		respondBadRequestError(c, err)
		return
	}
	side := strings.ToLower(strings.TrimSpace(req.Side))
	if side != "buy" && side != "sell" {
		respondBadRequestError(c, errors.New("side must be buy or sell"))
		return
	}
	tif, err := parseTimeInForce(req.TimeInForce)
	if err != nil {
		respondBadRequestError(c, err)
		return
	}
	quantity, err := parseAmountValue(req.Quantity)
	if err != nil {
		respondBadRequestError(c, err)
		return
	}
	price, err := parseAmountValue(req.Price)
	if err != nil {
		respondBadRequestError(c, err)
		return
	}
	chainID := req.ChainID
	if chainID == 0 {
		chainID = s.resolveChainParam(c)
	}
	client, err := s.client(chainID)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	call, err := client.BuildPlaceOrder(marketID, quantity, price, side == "buy", tif, req.ReduceOnly)
	if err != nil {
		respondBadRequestError(c, err)
		return
	}
	respondOnChainCall(c, call, chainID)
}

// BuildCancelOrderHandler returns calldata that cancels the caller's order.
func (s *OnChainPerpBookService) BuildCancelOrderHandler(c *gin.Context) {
	if _, ok := authenticatedUserID(c); !ok {
		return
	}
	raw := strings.TrimSpace(c.Param("id"))
	orderID, ok := new(big.Int).SetString(raw, 10)
	if !ok || orderID.Sign() <= 0 {
		respondBadRequestError(c, errors.New("order id must be a positive integer"))
		return
	}
	chainID := s.resolveChainParam(c)
	client, err := s.client(chainID)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	call, err := client.BuildCancelOrder(orderID)
	if err != nil {
		respondBadRequestError(c, err)
		return
	}
	respondOnChainCall(c, call, chainID)
}

func parseTimeInForce(raw string) (uint8, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "gtc":
		return orderbook.TIFGTC, nil
	case "ioc":
		return orderbook.TIFIOC, nil
	case "fok":
		return orderbook.TIFFOK, nil
	case "post_only", "postonly", "alo":
		return orderbook.TIFPostOnly, nil
	default:
		return 0, errors.New("time_in_force must be gtc, ioc, fok or post_only")
	}
}

// parseAmountValue parses a positive decimal amount into base units (18 dp).
func parseAmountValue(raw string) (*big.Int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, errors.New("amount is required")
	}
	rat, ok := new(big.Rat).SetString(raw)
	if !ok {
		return nil, fmt.Errorf("amount %q is not a number", raw)
	}
	if rat.Sign() <= 0 {
		return nil, errors.New("amount must be positive")
	}
	return rationalToBigInt(rat), nil
}

func parsePositiveAmount(c *gin.Context) (*big.Int, error) {
	var req bookAmountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		return nil, err
	}
	return parseAmountValue(req.Amount)
}

// respondOnChainCall writes the unsigned calldata a wallet signs.
func respondOnChainCall(c *gin.Context, call orderbook.Call, chainID int64) {
	value := "0"
	if call.Value != nil {
		value = call.Value.String()
	}
	c.JSON(http.StatusOK, gin.H{
		"to":       call.To.Hex(),
		"data":     call.Hex(),
		"value":    value,
		"chain_id": chainID,
	})
}

// ---- Helpers ----

func bigString(v *big.Int) string {
	if v == nil {
		return "0"
	}
	return v.String()
}

func bigSliceToStrings(in []*big.Int) []string {
	out := make([]string, 0, len(in))
	for _, v := range in {
		out = append(out, bigString(v))
	}
	return out
}

// ---- Keeper + oracle writer ----

// Start launches the indexer, the oracle writer and the liquidation keeper for
// every configured chain.
func (s *OnChainPerpBookService) Start(ctx context.Context) {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	s.running = true
	keeper := s.keeperKey
	s.mu.Unlock()

	for _, chainID := range s.Chains() {
		s.wg.Add(1)
		go func(id int64) {
			defer s.wg.Done()
			s.runIndexer(ctx, id)
		}(chainID)
		if keeper != nil {
			s.wg.Add(1)
			go func(id int64) {
				defer s.wg.Done()
				s.runKeeper(ctx, id)
			}(chainID)
			s.wg.Add(1)
			go func(id int64) {
				defer s.wg.Done()
				s.runOracleWriter(ctx, id)
			}(chainID)
		}
	}
}

// Stop cancels the background loops and waits for them.
func (s *OnChainPerpBookService) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	cancel := s.cancel
	s.running = false
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	s.wg.Wait()
}

func (s *OnChainPerpBookService) runIndexer(ctx context.Context, chainID int64) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	if err := s.indexOnce(ctx, chainID); err != nil {
		log.Printf("on-chain perp book indexer: chain %d: %v", chainID, err)
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.indexOnce(ctx, chainID); err != nil {
				log.Printf("on-chain perp book indexer: chain %d: %v", chainID, err)
			}
		}
	}
}

const bookConfirmations = 12

// indexOnce scans confirmed blocks for PositionChanged events and mirrors them.
func (s *OnChainPerpBookService) indexOnce(ctx context.Context, chainID int64) error {
	if s.db == nil {
		return nil
	}
	client, err := s.client(chainID)
	if err != nil {
		return err
	}
	head, err := client.RPC().BlockNumber(ctx)
	if err != nil {
		return fmt.Errorf("head block: %w", err)
	}
	if head <= bookConfirmations {
		return nil
	}
	safe := head - bookConfirmations

	cursor := models.OnChainBookCursor{ChainID: chainID, Contract: client.Address().Hex()}
	if err := s.db.First(&cursor, "chain_id = ? AND contract = ?", chainID, cursor.Contract).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		cursor.LastBlock = safe - 1
		if safe == 0 {
			cursor.LastBlock = 0
		}
	}
	if cursor.LastBlock >= safe {
		return nil
	}
	from := cursor.LastBlock + 1
	if from < 1 {
		from = 1
	}
	return s.scanRange(ctx, client, chainID, from, safe)
}

func (s *OnChainPerpBookService) scanRange(ctx context.Context, client *orderbook.Client, chainID int64, from, to uint64) error {
	abi := orderbook.ABI()
	event := abi.Events["PositionChanged"]
	topics := []common.Hash{event.ID}
	query := ethereum.FilterQuery{
		FromBlock: new(big.Int).SetUint64(from),
		ToBlock:   new(big.Int).SetUint64(to),
		Addresses: []common.Address{client.Address()},
		Topics:    [][]common.Hash{topics},
	}
	logs, err := client.RPC().FilterLogs(ctx, query)
	if err != nil {
		return fmt.Errorf("filter PositionChanged: %w", err)
	}
	for _, lg := range logs {
		if len(lg.Topics) < 3 {
			continue
		}
		var decoded struct {
			Size       *big.Int
			EntryPrice *big.Int
			Margin     *big.Int
		}
		if err := abi.UnpackIntoInterface(&decoded, "PositionChanged", lg.Data); err != nil {
			continue
		}
		marketID := common.BytesToHash(lg.Topics[1].Bytes())
		trader := common.BytesToAddress(lg.Topics[2].Bytes())
		if err := s.upsertPosition(chainID, marketID, trader, decoded.Size, decoded.EntryPrice, decoded.Margin, lg.BlockNumber); err != nil {
			return err
		}
	}
	if err := s.scanFills(ctx, client, chainID, from, to); err != nil {
		return err
	}
	// Advance the cursor in the same pass, so a restart resumes here.
	cursor := models.OnChainBookCursor{ChainID: chainID, Contract: client.Address().Hex(), LastBlock: to, UpdatedAt: time.Now()}
	return s.db.Save(&cursor).Error
}

// scanFills mirrors OrderFilled events into trade-history rows. Each row is
// keyed by (chain, tx hash, log index), so a reorg or a replay re-writes the
// same row rather than appending a duplicate.
func (s *OnChainPerpBookService) scanFills(ctx context.Context, client *orderbook.Client, chainID int64, from, to uint64) error {
	abi := orderbook.ABI()
	event := abi.Events["OrderFilled"]
	query := ethereum.FilterQuery{
		FromBlock: new(big.Int).SetUint64(from),
		ToBlock:   new(big.Int).SetUint64(to),
		Addresses: []common.Address{client.Address()},
		Topics:    [][]common.Hash{{event.ID}},
	}
	logs, err := client.RPC().FilterLogs(ctx, query)
	if err != nil {
		return fmt.Errorf("filter OrderFilled: %w", err)
	}
	for _, lg := range logs {
		// OrderFilled(orderId indexed, marketId indexed, trader indexed, price, quantity, isMaker, fee)
		if len(lg.Topics) < 4 {
			continue
		}
		var decoded struct {
			Price    *big.Int
			Quantity *big.Int
			IsMaker  bool
			Fee      *big.Int
		}
		if err := abi.UnpackIntoInterface(&decoded, "OrderFilled", lg.Data); err != nil {
			continue
		}
		orderID := new(big.Int).SetBytes(lg.Topics[1].Bytes())
		marketID := common.BytesToHash(lg.Topics[2].Bytes())
		trader := common.BytesToAddress(lg.Topics[3].Bytes())
		fill := models.OnChainBookFill{
			ChainID:     chainID,
			TxHash:      lg.TxHash.Hex(),
			LogIndex:    lg.Index,
			OrderID:     orderID.String(),
			MarketID:    marketID.Hex(),
			Symbol:      s.symbolFor(marketID),
			Trader:      trader.Hex(),
			IsMaker:     decoded.IsMaker,
			Price:       bigString(decoded.Price),
			Quantity:    bigString(decoded.Quantity),
			Fee:         bigString(decoded.Fee),
			BlockNumber: lg.BlockNumber,
			CreatedAt:   time.Now(),
		}
		if err := s.db.Where("chain_id = ? AND tx_hash = ? AND log_index = ?", chainID, fill.TxHash, fill.LogIndex).
			Assign(fill).FirstOrCreate(&models.OnChainBookFill{}).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *OnChainPerpBookService) upsertPosition(chainID int64, marketID common.Hash, trader common.Address, size, entryPrice, margin *big.Int, block uint64) error {
	now := time.Now()
	row := models.OnChainBookPosition{}
	err := s.db.Where("chain_id = ? AND market_id = ? AND trader = ?", chainID, marketID.Hex(), trader.Hex()).First(&row).Error
	isNew := errors.Is(err, gorm.ErrRecordNotFound)
	if err != nil && !isNew {
		return err
	}
	if isNew {
		row = models.OnChainBookPosition{
			ChainID:   chainID,
			MarketID:  marketID.Hex(),
			Trader:    trader.Hex(),
			Symbol:    s.symbolFor(marketID),
			UserID:    s.userIDForAddress(trader.Hex()),
			OpenedAt:  now,
			CreatedAt: now,
		}
	}
	row.Size = size.String()
	row.EntryPrice = entryPrice.String()
	row.Margin = margin.String()
	row.UpdatedBlock = block
	row.UpdatedAt = now
	if size.Sign() == 0 {
		row.Status = "flat"
		closed := now
		row.ClosedAt = &closed
		row.ClosedBlock = block
	} else {
		row.Status = "open"
		row.ClosedAt = nil
		row.ClosedBlock = 0
	}
	return s.db.Save(&row).Error
}

func (s *OnChainPerpBookService) userIDForAddress(address string) string {
	if s.db == nil {
		return ""
	}
	var user models.User
	if err := s.db.Select("id").Where("LOWER(wallet_address) = ?", strings.ToLower(address)).First(&user).Error; err != nil {
		return ""
	}
	return user.ID.String()
}

func (s *OnChainPerpBookService) runKeeper(ctx context.Context, chainID int64) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.liquidateUnhealthy(ctx, chainID); err != nil {
				log.Printf("on-chain perp book keeper: chain %d: %v", chainID, err)
			}
		}
	}
}

// liquidateUnhealthy liquidates every mirrored position whose equity has fallen
// below its maintenance requirement. The contract reads the price from the
// oracle, so the keeper cannot assert a health condition the chain would reject.
func (s *OnChainPerpBookService) liquidateUnhealthy(ctx context.Context, chainID int64) error {
	if s.db == nil {
		return nil
	}
	client, err := s.client(chainID)
	if err != nil {
		return err
	}
	var positions []models.OnChainBookPosition
	if err := s.db.Where("chain_id = ? AND status = ?", chainID, "open").Find(&positions).Error; err != nil {
		return err
	}
	for _, p := range positions {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		marketID := common.HexToHash(p.MarketID)
		trader := common.HexToAddress(p.Trader)
		market, err := client.Market(ctx, marketID)
		if err != nil || market.Token == (common.Address{}) {
			continue
		}
		pos, err := client.Position(ctx, marketID, trader)
		if err != nil || pos.Size.Sign() == 0 {
			continue
		}
		// A liquidation is attempted only when the account is underwater; the
		// contract re-checks against its own oracle and reverts otherwise, so a
		// stale mirror can never force an unwarranted liquidation.
		funding, err := client.Funding(ctx, marketID)
		if err != nil || funding == nil || funding.MarkPrice.Sign() == 0 {
			continue
		}
		if !bookBelowMaintenance(pos, market, funding.MarkPrice) {
			continue
		}
		call, err := client.BuildLiquidate(marketID, trader)
		if err != nil {
			continue
		}
		receipt, err := client.Send(ctx, call)
		if err != nil {
			log.Printf("on-chain perp book keeper: liquidate %s on chain %d failed: %v", p.Trader, chainID, err)
			continue
		}
		liq := models.OnChainBookLiquidation{
			ChainID:     chainID,
			MarketID:    p.MarketID,
			Trader:      p.Trader,
			TxHash:      receipt.TxHash.Hex(),
			BlockNumber: receipt.BlockNumber.Uint64(),
		}
		// The liquidation already executed on-chain, so a persistence failure must
		// not be swallowed: the record is the audit trail an operator reconciles
		// against. Log it loudly and keep sweeping the remaining positions.
		if err := s.db.Create(&liq).Error; err != nil {
			log.Printf("on-chain perp book keeper: failed to record liquidation of %s on chain %d (tx %s): %v", p.Trader, chainID, receipt.TxHash.Hex(), err)
		}
	}
	return nil
}

// bookMarkBand caps how far the book mid may pull the mark away from the
// index, in basis points. The band keeps a thin or briefly dislocated book
// from moving funding (and therefore every position's PnL) without limit.
const bookMarkBandBps = 500

// markFromBook returns the mark price used for funding and margin: the mid of
// the best bid and ask, clamped to +/- bookMarkBandBps around the index. With
// one side of the book empty the mid is undefined, so the index is used and the
// funding rate stays zero rather than being invented from a one-sided quote.
func markFromBook(bid, ask, index *big.Int) *big.Int {
	if index == nil || index.Sign() <= 0 {
		return new(big.Int)
	}
	if bid == nil || ask == nil || bid.Sign() <= 0 || ask.Sign() <= 0 {
		return new(big.Int).Set(index)
	}
	mid := new(big.Int).Add(bid, ask)
	mid.Div(mid, big.NewInt(2))

	band := new(big.Int).Mul(index, big.NewInt(bookMarkBandBps))
	band.Div(band, big.NewInt(10000))
	low := new(big.Int).Sub(index, band)
	high := new(big.Int).Add(index, band)
	if mid.Cmp(low) < 0 {
		return low
	}
	if mid.Cmp(high) > 0 {
		return high
	}
	return mid
}

// bookBelowMaintenance reports whether equity is below the maintenance margin,
// mirroring the contract's check. Equity and the requirement are exact integers;
// the requirement is `abs(size) * mark / WAD * maintenanceMargin / WAD`.
func bookBelowMaintenance(pos *orderbook.Position, market *orderbook.Market, mark *big.Int) bool {
	if pos.Size.Sign() == 0 || mark.Sign() == 0 {
		return false
	}
	absSize := new(big.Int).Abs(pos.Size)
	notional := new(big.Int).Mul(absSize, mark)
	notional.Div(notional, orderbook.OneUnit)
	maintenance := new(big.Int).Mul(notional, market.MaintenanceMargin)
	maintenance.Div(maintenance, orderbook.OneUnit)
	return pos.Equity.Cmp(maintenance) < 0
}

func (s *OnChainPerpBookService) runOracleWriter(ctx context.Context, chainID int64) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	if err := s.pushMarkPrices(ctx, chainID); err != nil {
		log.Printf("on-chain perp book oracle: chain %d: %v", chainID, err)
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.pushMarkPrices(ctx, chainID); err != nil {
				log.Printf("on-chain perp book oracle: chain %d: %v", chainID, err)
			}
		}
	}
}

// pushMarkPrices submits a fresh mark and index price for every order book
// market. The contract's accrueFunding folds the spread into the cumulative
// funding index, so the writer is what makes funding real.
func (s *OnChainPerpBookService) pushMarkPrices(ctx context.Context, chainID int64) error {
	if s.priceAgg == nil {
		return errors.New("oracle writer has no price source")
	}
	client, err := s.client(chainID)
	if err != nil {
		return err
	}
	listings, err := s.priceableListings()
	if err != nil {
		return err
	}
	var firstErr error
	for _, listing := range listings {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		marketID := orderbook.MarketID(listing.symbol)
		market, err := client.Market(ctx, marketID)
		if err != nil || market.Token == (common.Address{}) {
			continue
		}
		priceData, err := s.priceAgg.GetRealPrice(ctx, listing.oracleSymbol)
		if err != nil || priceData == nil || priceData.USD <= 0 {
			if firstErr == nil {
				firstErr = fmt.Errorf("no oracle price for %s: %w", listing.oracleSymbol, err)
			}
			continue
		}
		index, err := usdFloatToWAD(priceData.USD)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		// The mark is the market's own book mid, banded around the index. With
		// mark == index the contract's funding rate is always zero, so funding
		// would be inert and a perp book would be indistinguishable from a
		// zero-interest one.
		bidPrice, _, askPrice, _, err := client.BestBidAsk(ctx, marketID)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		mark := markFromBook(bidPrice, askPrice, index)
		call, err := client.BuildAccrueFunding(marketID, mark, index)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if _, err := client.Send(ctx, call); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
	}
	return firstErr
}

// priceableListings reuses the same active-listing set the perps oracle uses.
func (s *OnChainPerpBookService) priceableListings() ([]priceableListing, error) {
	if s.db == nil {
		return nil, errors.New("oracle writer has no database")
	}
	var futures []models.FuturesMarket
	if err := s.db.Where("status = ?", models.ListingStatusActive).Find(&futures).Error; err != nil {
		return nil, err
	}
	seen := make(map[string]bool)
	var out []priceableListing
	add := func(symbol, oracleSymbol string) {
		symbol = strings.TrimSpace(symbol)
		oracleSymbol = strings.ToUpper(strings.TrimSpace(oracleSymbol))
		if symbol == "" || oracleSymbol == "" || seen[strings.ToUpper(symbol)] {
			return
		}
		seen[strings.ToUpper(symbol)] = true
		out = append(out, priceableListing{symbol: symbol, oracleSymbol: oracleSymbol})
	}
	for _, m := range futures {
		oracleSymbol := strings.ToUpper(strings.TrimSpace(m.BaseAsset))
		if oracleSymbol == "" {
			oracleSymbol = oracleBaseFromSymbol(m.Symbol)
		}
		add(m.Symbol, oracleSymbol)
	}
	return out, nil
}
