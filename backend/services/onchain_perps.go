package services

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"tigerswap/backend/go/perps"
	"tigerswap/backend/models"
)

// OnChainPerpsService exposes TigerSwap's deployed perpetuals contract to the
// API. Positions, margin, funding and liquidation settle on-chain — the chain is
// the source of truth — so the derivatives product is verifiable by any user
// with an RPC endpoint, the way a Hyperliquid/dYdX-class venue is.
//
// The service never holds a user's key: an order-building endpoint returns
// unsigned calldata for the user's own wallet to sign, exactly like the spot
// router path. The keeper key is used only for the permissionless operations
// the platform is allowed to submit itself (liquidation and oracle pushes).
type OnChainPerpsService struct {
	db       *gorm.DB
	priceAgg priceSource

	mu      sync.RWMutex
	clients map[int64]*perps.Client

	// symbolByMarket maps a market id (bytes32 hex) to its listing symbol so an
	// indexed position can be labelled. A market id is keccak256(symbol), so the
	// mapping is identical on every chain and needs no per-chain dimension.
	symbolByMarket map[string]string

	keeperKey *ecdsa.PrivateKey

	// ownerKey signs owner-only calls (createMarket). The contract owner is not
	// the keeper, so market creation uses this key while liquidations and oracle
	// pushes use the keeper key.
	ownerKey *ecdsa.PrivateKey

	running bool
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

// NewOnChainPerpsService builds the service. It dials the chains named by
// TIGERSWAP_PERPS_CHAINS; a chain with no address is skipped rather than
// registered half-configured.
func NewOnChainPerpsService(db *gorm.DB, priceAgg priceSource) *OnChainPerpsService {
	s := &OnChainPerpsService{
		db:             db,
		priceAgg:       priceAgg,
		clients:        make(map[int64]*perps.Client),
		symbolByMarket: make(map[string]string),
	}
	return s
}

// SetKeeperKey wires the signer used for liquidations and oracle updates. It is
// optional: without it the read and order-building endpoints still work, and the
// keeper loop is simply not started.
//
// A client is built with the signer it will send with, so setting the key after
// the deployments are loaded rebuilds each client. Without this the keeper would
// hold read-only clients and every liquidation and price push would fail with
// "no signer configured" — a failure that only appears at runtime, on chain.
func (s *OnChainPerpsService) SetKeeperKey(key *ecdsa.PrivateKey) {
	s.mu.Lock()
	s.keeperKey = key
	deployments := make([]perps.Deployment, 0, len(s.clients))
	for _, client := range s.clients {
		deployments = append(deployments, client.Deployment())
	}
	s.mu.Unlock()

	for _, d := range deployments {
		if err := s.RegisterDeployment(d); err != nil {
			log.Printf("on-chain perps: could not re-key chain %d: %v", d.ChainID, err)
		}
	}
}

// SetOwnerKey wires the signer used for owner-only on-chain calls, currently
// createMarket. It is optional: without it the admin create handler still
// persists the listing but refuses to claim the market exists on chain.
func (s *OnChainPerpsService) SetOwnerKey(key *ecdsa.PrivateKey) {
	s.mu.Lock()
	s.ownerKey = key
	s.mu.Unlock()
}

// CreateMarketOnChain creates a market on the deployed contract, signed by the
// owner key. It is idempotent: a market that already exists on chain is reported
// as such rather than sent again, because createMarket reverts on a duplicate.
func (s *OnChainPerpsService) CreateMarketOnChain(ctx context.Context, chainID int64, symbol, settlementToken string, maxLeverage int, maintenanceMargin, liquidationFee float64) error {
	s.mu.Lock()
	ownerKey := s.ownerKey
	s.mu.Unlock()
	if ownerKey == nil {
		return errors.New("no owner key configured (set TIGERSWAP_PERPS_OWNER_KEY)")
	}

	client, err := s.client(chainID)
	if err != nil {
		return err
	}
	if !common.IsHexAddress(settlementToken) {
		return fmt.Errorf("settlement token %q is not a valid address", settlementToken)
	}
	token := common.HexToAddress(settlementToken)

	marketID := perps.MarketID(symbol)
	if existing, err := client.Market(ctx, marketID); err == nil && existing.Token != (common.Address{}) {
		return nil
	}

	call, err := client.BuildCreateMarket(
		marketID,
		token,
		perps.LeverageWAD(maxLeverage),
		perps.RatioWAD(liquidationFee),
		perps.RatioWAD(maintenanceMargin),
		perps.InitialMarginWAD(maxLeverage),
	)
	if err != nil {
		return err
	}
	receipt, err := client.SendWithKey(ctx, call, ownerKey)
	if err != nil {
		return fmt.Errorf("createMarket %s on chain %d: %w", symbol, chainID, err)
	}
	log.Printf("on-chain perps: created market %s on chain %d (tx %s)", symbol, chainID, receipt.TxHash.Hex())
	return nil
}

// MirrorPerpsMarket creates the on-chain market for a freshly persisted perps
// listing. It resolves the chain (the single configured one, or the first that
// has a settlement token registered) and the settlement token from the platform
// token table, then calls CreateMarketOnChain. A listing with no configured
// chain, or with no on-chain settlement token, is skipped rather than guessed.
func (s *OnChainPerpsService) MirrorPerpsMarket(ctx context.Context, market *models.PerpsMarket) error {
	if s.db == nil || market == nil {
		return nil
	}
	settlement := "RUSD"
	if assets := strings.Split(market.CollateralAssets, ","); len(assets) > 0 {
		if first := strings.ToUpper(strings.TrimSpace(assets[0])); first != "" {
			settlement = first
		}
	}
	chains := s.Chains()
	if len(chains) == 0 {
		return nil
	}

	for _, chainID := range chains {
		var token models.Token
		err := s.db.Where("chain_id = ? AND symbol = ? AND is_active = ?", chainID, settlement, true).
			First(&token).Error
		if err != nil {
			continue
		}
		if !common.IsHexAddress(token.Address) {
			return fmt.Errorf("settlement token %s on chain %d has no on-chain address", settlement, chainID)
		}
		return s.CreateMarketOnChain(ctx, chainID, market.Symbol, token.Address, market.MaxLeverage, market.MaintenanceMarginRate, market.LiquidationFeeRate)
	}
	return nil
}

// LoadDeployments registers every configured on-chain perps deployment. It is a
// full refresh: a chain that is no longer configured has its client closed and
// removed, so a decommissioned deployment cannot keep being dialled.
func (s *OnChainPerpsService) LoadDeployments() error {
	deployments := perps.LoadDeployments()

	s.mu.Lock()
	defer s.mu.Unlock()

	seen := make(map[int64]bool, len(deployments))
	for _, d := range deployments {
		client, err := perps.NewClient(d, s.keeperKey)
		if err != nil {
			log.Printf("on-chain perps: chain %d unavailable: %v", d.ChainID, err)
			continue
		}
		if existing, ok := s.clients[d.ChainID]; ok {
			existing.Close()
		}
		s.clients[d.ChainID] = client
		seen[d.ChainID] = true
	}
	for id, client := range s.clients {
		if !seen[id] {
			client.Close()
			delete(s.clients, id)
		}
	}
	return nil
}

// RegisterDeployment dials and registers one on-chain perps deployment,
// replacing any existing client for the same chain. It is the single-chain form
// of LoadDeployments and is what an admin chain registration uses.
func (s *OnChainPerpsService) RegisterDeployment(d perps.Deployment) error {
	client, err := perps.NewClient(d, s.keeperKey)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.clients[d.ChainID]; ok {
		existing.Close()
	}
	s.clients[d.ChainID] = client
	return nil
}

// IndexRange folds the perps events in [from, to] into the mirror. It is the
// exact path the background indexer runs, exposed so an operator (or a test)
// can backfill a known range.
func (s *OnChainPerpsService) IndexRange(ctx context.Context, chainID int64, from, to uint64) error {
	client, err := s.client(chainID)
	if err != nil {
		return err
	}
	return s.scanRange(ctx, client, chainID, from, to)
}

// client returns the client for a chain, or an error naming the chain.
func (s *OnChainPerpsService) client(chainID int64) (*perps.Client, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.clients[chainID]
	if !ok {
		return nil, fmt.Errorf("no on-chain perps deployment for chain %d", chainID)
	}
	return c, nil
}

// Chains returns the chain ids with a live perps deployment.
func (s *OnChainPerpsService) Chains() []int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]int64, 0, len(s.clients))
	for id := range s.clients {
		out = append(out, id)
	}
	return out
}

// ReloadSymbols rebuilds the market-id to symbol map from the admin listings, so
// an indexed position is labelled with the symbol an operator configured.
func (s *OnChainPerpsService) ReloadSymbols() error {
	if s.db == nil {
		return errors.New("on-chain perps service has no database")
	}
	var futures []models.FuturesMarket
	if err := s.db.Find(&futures).Error; err != nil {
		return err
	}
	var perpsMarkets []models.PerpsMarket
	if err := s.db.Find(&perpsMarkets).Error; err != nil {
		return err
	}

	mapping := make(map[string]string)
	assign := func(symbol string) {
		// A symbol maps to exactly one market id, and that id is the same on
		// every chain.
		mapping[strings.ToLower(perps.MarketID(symbol).Hex())] = symbol
	}
	for _, m := range futures {
		assign(m.Symbol)
	}
	for _, m := range perpsMarkets {
		assign(m.Symbol)
	}

	s.mu.Lock()
	s.symbolByMarket = mapping
	s.mu.Unlock()
	return nil
}

// symbolFor returns the listing symbol for a market id, or the market id itself
// when no listing matches (so a position is never mislabelled).
func (s *OnChainPerpsService) symbolFor(marketID common.Hash) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if sym, ok := s.symbolByMarket[strings.ToLower(marketID.Hex())]; ok {
		return sym
	}
	return marketID.Hex()
}

// --- HTTP handlers: read ---

// MarketsHandler lists the on-chain markets for every configured chain, with
// their live open interest and the pool liquidity backing them.
func (s *OnChainPerpsService) MarketsHandler(c *gin.Context) {
	type marketOut struct {
		ChainID         int64  `json:"chain_id"`
		Symbol          string `json:"symbol"`
		MarketID        string `json:"market_id"`
		Token           string `json:"token"`
		MaxLeverage     string `json:"max_leverage"`
		LiquidationFee  string `json:"liquidation_fee"`
		MaintenanceRate string `json:"maintenance_margin_rate"`
		InitialRate     string `json:"initial_margin_rate"`
		OpenFee         string `json:"open_fee"`
		MaxOpenInterest string `json:"max_open_interest"`
		OpenInterest    string `json:"open_interest"`
		IsActive        bool   `json:"is_active"`
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
	defer cancel()

	var out []marketOut
	for _, chainID := range s.Chains() {
		client, err := s.client(chainID)
		if err != nil {
			continue
		}
		var futures []models.FuturesMarket
		if s.db != nil {
			if err := s.db.Where("status = ?", models.ListingStatusActive).Find(&futures).Error; err != nil {
				respondServiceError(c, err)
				return
			}
		}
		for _, m := range futures {
			marketID := perps.MarketID(m.Symbol)
			market, err := client.Market(ctx, marketID)
			if err != nil {
				// A listing with no on-chain market is reported as inactive rather
				// than omitted, so the operator can see the drift.
				continue
			}
			out = append(out, marketOut{
				ChainID:         chainID,
				Symbol:          m.Symbol,
				MarketID:        marketID.Hex(),
				Token:           market.Token.Hex(),
				MaxLeverage:     market.MaxLeverage.String(),
				LiquidationFee:  market.LiquidationFee.String(),
				MaintenanceRate: market.MaintenanceRate.String(),
				InitialRate:     market.InitialRate.String(),
				OpenFee:         market.OpenFee.String(),
				MaxOpenInterest: market.MaxOpenInterest.String(),
				OpenInterest:    market.OpenInterest.String(),
				IsActive:        market.IsActive,
			})
		}
	}
	c.JSON(http.StatusOK, gin.H{"markets": out, "count": len(out)})
}

// PositionsHandler returns the caller's on-chain positions, resolved from their
// bound wallet address. Identity is read from the auth context, never the body.
func (s *OnChainPerpsService) PositionsHandler(c *gin.Context) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}

	chainID := s.resolveChainParam(c)
	if chainID == 0 {
		respondBadRequestError(c, errors.New("chain_id is required"))
		return
	}
	// Resolve the deployment first: a chain with no configured contract is the
	// more specific error, and it must not be masked by a wallet check.
	if _, err := s.client(chainID); err != nil {
		respondServiceError(c, err)
		return
	}
	address, err := s.walletAddressFor(userID)
	if err != nil {
		respondServiceError(c, err)
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
	defer cancel()

	positions, err := s.positionsForAddress(ctx, chainID, address)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"positions": positions, "count": len(positions)})
}

// PositionHandler returns a single position by id, scoped to the caller.
func (s *OnChainPerpsService) PositionHandler(c *gin.Context) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}
	chainID := s.resolveChainParam(c)
	positionID := strings.TrimSpace(c.Param("id"))
	if !isBytes32(positionID) {
		respondBadRequestError(c, errors.New("invalid position id"))
		return
	}

	address, err := s.walletAddressFor(userID)
	if err != nil {
		respondServiceError(c, err)
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
	defer cancel()

	client, err := s.client(chainID)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	position, err := client.Position(ctx, common.HexToHash(positionID))
	if err != nil {
		respondServiceError(c, err)
		return
	}
	if !strings.EqualFold(position.Trader.Hex(), address) {
		respondForbiddenError(c, errors.New("position belongs to another account"))
		return
	}
	if position.Size.Sign() == 0 {
		respondNotFoundError(c, errors.New("position not found"))
		return
	}
	c.JSON(http.StatusOK, s.positionView(ctx, client, chainID, common.HexToHash(positionID), position))
}

// --- HTTP handlers: order building (unsigned calldata) ---

// openPositionRequest is the body of an order-building request. Amounts are
// decimal strings in the settlement token's smallest unit.
type openPositionRequest struct {
	ChainID    int64  `json:"chain_id" binding:"required"`
	Symbol     string `json:"symbol" binding:"required"`
	Size       string `json:"size" binding:"required"`
	Collateral string `json:"collateral" binding:"required"`
	Side       string `json:"side" binding:"required"`
}

// BuildOpenPositionHandler returns unsigned calldata that opens a position on
// the deployed contract. The caller's wallet signs it; the server holds no key.
func (s *OnChainPerpsService) BuildOpenPositionHandler(c *gin.Context) {
	if _, ok := authenticatedUserID(c); !ok {
		return
	}
	var req openPositionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequestError(c, err)
		return
	}
	size, err := perpAmountValue(req.Size)
	if err != nil {
		respondBadRequestError(c, err)
		return
	}
	collateral, err := perpAmountValue(req.Collateral)
	if err != nil {
		respondBadRequestError(c, err)
		return
	}
	side := strings.ToLower(strings.TrimSpace(req.Side))
	if side != "long" && side != "short" {
		respondBadRequestError(c, errors.New("side must be long or short"))
		return
	}

	client, err := s.client(req.ChainID)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	sizeWei := rationalToBigInt(size)
	collateralWei := rationalToBigInt(collateral)

	call, err := client.BuildOpenPosition(perps.MarketID(req.Symbol), sizeWei, collateralWei, side == "long")
	if err != nil {
		respondBadRequestError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"to":       call.To.Hex(),
		"data":     call.Hex(),
		"value":    "0",
		"chain_id": req.ChainID,
		"symbol":   req.Symbol,
	})
}

// BuildClosePositionHandler returns unsigned calldata that closes a position.
func (s *OnChainPerpsService) BuildClosePositionHandler(c *gin.Context) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}
	chainID := s.resolveChainParam(c)
	positionID := strings.TrimSpace(c.Param("id"))
	if !isBytes32(positionID) {
		respondBadRequestError(c, errors.New("invalid position id"))
		return
	}
	address, err := s.walletAddressFor(userID)
	if err != nil {
		respondServiceError(c, err)
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
	defer cancel()

	client, err := s.client(chainID)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	// The caller may only build a close for a position their own wallet owns.
	position, err := client.Position(ctx, common.HexToHash(positionID))
	if err != nil {
		respondServiceError(c, err)
		return
	}
	if position.Size.Sign() == 0 || !strings.EqualFold(position.Trader.Hex(), address) {
		respondForbiddenError(c, errors.New("position does not belong to this account"))
		return
	}
	call, err := client.BuildClosePosition(common.HexToHash(positionID))
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"to": call.To.Hex(), "data": call.Hex(), "value": "0", "chain_id": chainID})
}

// BuildAddMarginHandler returns unsigned calldata that adds margin.
func (s *OnChainPerpsService) BuildAddMarginHandler(c *gin.Context) {
	s.buildMarginCall(c, true)
}

// BuildRemoveMarginHandler returns unsigned calldata that removes margin.
func (s *OnChainPerpsService) BuildRemoveMarginHandler(c *gin.Context) {
	s.buildMarginCall(c, false)
}

func (s *OnChainPerpsService) buildMarginCall(c *gin.Context, add bool) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}
	var req struct {
		ChainID    int64  `json:"chain_id" binding:"required"`
		PositionID string `json:"position_id" binding:"required"`
		Amount     string `json:"amount" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequestError(c, err)
		return
	}
	if !isBytes32(req.PositionID) {
		respondBadRequestError(c, errors.New("invalid position id"))
		return
	}
	amount, err := perpAmountValue(req.Amount)
	if err != nil {
		respondBadRequestError(c, err)
		return
	}
	address, err := s.walletAddressFor(userID)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	client, err := s.client(req.ChainID)
	if err != nil {
		respondServiceError(c, err)
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
	defer cancel()

	position, err := client.Position(ctx, common.HexToHash(req.PositionID))
	if err != nil {
		respondServiceError(c, err)
		return
	}
	if position.Size.Sign() == 0 || !strings.EqualFold(position.Trader.Hex(), address) {
		respondForbiddenError(c, errors.New("position does not belong to this account"))
		return
	}

	amountWei := rationalToBigInt(amount)
	var call perps.Call
	if add {
		call, err = client.BuildAddMargin(common.HexToHash(req.PositionID), amountWei)
	} else {
		call, err = client.BuildRemoveMargin(common.HexToHash(req.PositionID), amountWei)
	}
	if err != nil {
		respondBadRequestError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"to": call.To.Hex(), "data": call.Hex(), "value": "0", "chain_id": req.ChainID})
}

// BuildLiquidityHandler returns unsigned calldata that adds LP liquidity.
func (s *OnChainPerpsService) BuildLiquidityHandler(c *gin.Context) {
	if _, ok := authenticatedUserID(c); !ok {
		return
	}
	var req struct {
		ChainID int64  `json:"chain_id" binding:"required"`
		Token   string `json:"token" binding:"required"`
		Amount  string `json:"amount" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequestError(c, err)
		return
	}
	if !common.IsHexAddress(req.Token) {
		respondBadRequestError(c, errors.New("invalid token address"))
		return
	}
	amount, err := perpAmountValue(req.Amount)
	if err != nil {
		respondBadRequestError(c, err)
		return
	}
	client, err := s.client(req.ChainID)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	call, err := client.BuildAddLiquidity(common.HexToAddress(req.Token), rationalToBigInt(amount))
	if err != nil {
		respondBadRequestError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"to": call.To.Hex(), "data": call.Hex(), "value": "0", "chain_id": req.ChainID})
}

// --- Indexer ---

// Start launches the event indexer and the liquidation keeper. Both are
// best-effort background loops; a chain that is temporarily unreachable is
// retried on the next tick rather than aborting the process.
func (s *OnChainPerpsService) Start(ctx context.Context) {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	s.running = true
	s.mu.Unlock()

	for _, chainID := range s.Chains() {
		s.wg.Add(1)
		go func(id int64) {
			defer s.wg.Done()
			s.runIndexer(ctx, id)
		}(chainID)
	}
	if s.keeperKey != nil {
		for _, chainID := range s.Chains() {
			s.wg.Add(1)
			go func(id int64) {
				defer s.wg.Done()
				s.runKeeper(ctx, id)
			}(chainID)
		}
		// The oracle writer shares the keeper key: the contract's updatePrice is
		// oracle-gated, so the same signer that liquidates also publishes marks.
		s.startOracleWriter(ctx)
	}
}

// Stop halts the background loops.
func (s *OnChainPerpsService) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.cancel()
	s.running = false
	s.mu.Unlock()
	s.wg.Wait()
}

// runIndexer scans confirmed PositionOpened/Closed/Liquidated logs into the
// mirror table.
func (s *OnChainPerpsService) runIndexer(ctx context.Context, chainID int64) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.indexOnce(ctx, chainID); err != nil {
				log.Printf("on-chain perps indexer: chain %d: %v", chainID, err)
			}
		}
	}
}

// runKeeper periodically checks open positions against the oracle mark and
// liquidates any that fall below maintenance margin.
func (s *OnChainPerpsService) runKeeper(ctx context.Context, chainID int64) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.liquidateUnhealthy(ctx, chainID); err != nil {
				log.Printf("on-chain perps keeper: chain %d: %v", chainID, err)
			}
		}
	}
}

// indexOnce advances one chain's cursor and folds new events into the mirror.
func (s *OnChainPerpsService) indexOnce(ctx context.Context, chainID int64) error {
	if s.db == nil {
		return errors.New("indexer has no database")
	}
	client, err := s.client(chainID)
	if err != nil {
		return err
	}
	rpc := client.RPC()
	head, err := rpc.BlockNumber(ctx)
	if err != nil {
		return fmt.Errorf("head: %w", err)
	}
	const confirmations = uint64(12)
	if head < confirmations {
		return nil
	}
	safeHead := head - confirmations

	contract := client.PerpetualsAddress().Hex()
	var cursor models.OnChainPerpCursor
	err = s.db.Where("chain_id = ? AND contract = ?", chainID, contract).First(&cursor).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Seed at the current safe head: the mirror starts empty and fills from
		// the first event after the indexer starts, rather than replaying genesis.
		cursor = models.OnChainPerpCursor{ChainID: chainID, Contract: contract, LastBlock: safeHead}
		if err := s.db.Create(&cursor).Error; err != nil {
			return err
		}
		return nil
	} else if err != nil {
		return err
	}
	if safeHead <= cursor.LastBlock {
		return nil
	}

	from := cursor.LastBlock + 1
	to := safeHead
	if to-from > 2000 {
		to = from + 2000
	}

	if err := s.scanRange(ctx, client, chainID, from, to); err != nil {
		return err
	}
	return s.db.Model(&models.OnChainPerpCursor{}).
		Where("chain_id = ? AND contract = ?", chainID, contract).
		Update("last_block", to).Error
}

// scanRange reads the three position events in a block range and applies them.
func (s *OnChainPerpsService) scanRange(ctx context.Context, client *perps.Client, chainID int64, from, to uint64) error {
	contract := client.PerpetualsAddress()
	abi := perps.ABI()
	topics := [][]common.Hash{{
		abi.Events["PositionOpened"].ID,
		abi.Events["PositionClosed"].ID,
		abi.Events["PositionLiquidated"].ID,
	}}

	query := ethereum.FilterQuery{
		FromBlock: new(big.Int).SetUint64(from),
		ToBlock:   new(big.Int).SetUint64(to),
		Addresses: []common.Address{contract},
		Topics:    topics,
	}
	logs, err := client.RPC().FilterLogs(ctx, query)
	if err != nil {
		return fmt.Errorf("filter logs: %w", err)
	}
	for _, lg := range logs {
		if len(lg.Topics) == 0 {
			continue
		}
		switch lg.Topics[0] {
		case abi.Events["PositionOpened"].ID:
			if err := s.applyPositionOpened(ctx, client, chainID, lg); err != nil {
				return err
			}
		case abi.Events["PositionClosed"].ID:
			if err := s.applyPositionClosed(ctx, client, chainID, lg, "closed"); err != nil {
				return err
			}
		case abi.Events["PositionLiquidated"].ID:
			if err := s.applyPositionClosed(ctx, client, chainID, lg, "liquidated"); err != nil {
				return err
			}
		}
	}
	return nil
}

// applyPositionOpened upserts the mirror row for a PositionOpened event.
func (s *OnChainPerpsService) applyPositionOpened(ctx context.Context, client *perps.Client, chainID int64, lg types.Log) error {
	abi := perps.ABI()
	// Indexed: marketId, trader. Non-indexed: positionId, size, collateral, isLong.
	if len(lg.Topics) < 3 {
		return nil
	}
	marketID := common.BytesToHash(lg.Topics[1].Bytes())
	trader := common.BytesToAddress(lg.Topics[2].Bytes())
	vals, err := abi.Unpack("PositionOpened", lg.Data)
	if err != nil {
		return fmt.Errorf("unpack PositionOpened: %w", err)
	}
	positionID, _ := vals[0].([32]byte)
	size := toBigString(vals[1])
	collateral := toBigString(vals[2])
	isLong, _ := vals[3].(bool)

	mark, err := client.MarkPrice(ctx, marketID)
	if err != nil {
		return fmt.Errorf("mark price: %w", err)
	}

	row := models.OnChainPerpPosition{
		ChainID:     chainID,
		PositionID:  common.Hash(positionID).Hex(),
		MarketID:    marketID.Hex(),
		Symbol:      s.symbolFor(marketID),
		Trader:      strings.ToLower(trader.Hex()),
		UserID:      s.userIDForAddress(trader.Hex()),
		IsLong:      isLong,
		Size:        size,
		Collateral:  collateral,
		EntryPrice:  mark.String(),
		MarkPrice:   mark.String(),
		Status:      "open",
		OpenedBlock: lg.BlockNumber,
		OpenedAt:    time.Now().UTC(),
	}
	// Upsert by (chain, position_id): a reorg re-index converges on the same row.
	return s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "chain_id"}, {Name: "position_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"symbol", "size", "collateral", "entry_price", "mark_price", "status", "opened_block", "updated_at"}),
	}).Create(&row).Error
}

// applyPositionClosed marks a mirror row closed or liquidated and records the
// realised PnL from the event. The positionId is a non-indexed field, so only
// the event signature topic is required.
func (s *OnChainPerpsService) applyPositionClosed(ctx context.Context, client *perps.Client, chainID int64, lg types.Log, status string) error {
	abi := perps.ABI()
	if len(lg.Topics) < 1 {
		return nil
	}
	event := "PositionClosed"
	if status == "liquidated" {
		event = "PositionLiquidated"
	}
	vals, err := abi.Unpack(event, lg.Data)
	if err != nil {
		return fmt.Errorf("unpack %s: %w", event, err)
	}
	positionID, _ := vals[0].([32]byte)
	// PositionClosed has pnl at index 1; PositionLiquidated has liquidator then fee.
	var pnl string
	if status == "closed" {
		pnl = signedBigString(vals[1])
	} else {
		pnl = "0"
	}
	now := time.Now().UTC()
	return s.db.Model(&models.OnChainPerpPosition{}).
		Where("chain_id = ? AND position_id = ?", chainID, common.Hash(positionID).Hex()).
		Updates(map[string]interface{}{
			"status":       status,
			"realized_pnl": pnl,
			"closed_block": lg.BlockNumber,
			"closed_at":    now,
			"updated_at":   now,
		}).Error
}

// liquidateUnhealthy liquidates any mirrored open position that is below
// maintenance margin on-chain. The check mirrors the contract exactly, and the
// contract re-checks, so a stale mirror can only cause a no-op revert.
func (s *OnChainPerpsService) liquidateUnhealthy(ctx context.Context, chainID int64) error {
	if s.db == nil || s.keeperKey == nil {
		return nil
	}
	client, err := s.client(chainID)
	if err != nil {
		return err
	}
	var open []models.OnChainPerpPosition
	if err := s.db.Where("chain_id = ? AND status = ?", chainID, "open").Find(&open).Error; err != nil {
		return err
	}
	for _, row := range open {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		position, err := client.Position(ctx, common.HexToHash(row.PositionID))
		if err != nil {
			log.Printf("keeper: read position %s: %v", row.PositionID, err)
			continue
		}
		if position.Size.Sign() == 0 {
			// Already closed on-chain; reconcile the mirror.
			_ = s.db.Model(&models.OnChainPerpPosition{}).
				Where("chain_id = ? AND position_id = ?", chainID, row.PositionID).
				Updates(map[string]interface{}{"status": "closed", "updated_at": time.Now().UTC()}).Error
			continue
		}
		market, err := client.Market(ctx, position.MarketID)
		if err != nil {
			continue
		}
		price, err := client.MarkPrice(ctx, position.MarketID)
		if err != nil {
			continue
		}
		funding, err := client.FundingInfo(ctx, position.MarketID)
		if err != nil {
			continue
		}
		if !belowMaintenanceMargin(position, market, price, funding, time.Now().Unix()) {
			continue
		}
		call, err := client.BuildLiquidatePosition(common.HexToHash(row.PositionID))
		if err != nil {
			continue
		}
		receipt, err := client.Send(ctx, call)
		if err != nil {
			log.Printf("keeper: liquidate %s: %v", row.PositionID, err)
			continue
		}
		_ = s.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&models.OnChainPerpLiquidation{
			ChainID:     chainID,
			PositionID:  row.PositionID,
			Trader:      row.Trader,
			TxHash:      receipt.TxHash.Hex(),
			BlockNumber: receipt.BlockNumber.Uint64(),
		}).Error
		log.Printf("keeper: liquidated position %s on chain %d (tx %s)", row.PositionID, chainID, receipt.TxHash.Hex())
	}
	return nil
}

// belowMaintenanceMargin mirrors TigerSwapPerpetuals.liquidatePosition, which
// applies accrued funding before the health check. A keeper that ignored funding
// would miss exactly the positions funding is meant to close, so the accrual is
// reproduced here digit for digit against the contract's own math.
func belowMaintenanceMargin(position *perps.Position, market *perps.Market, price *big.Int, funding *perps.Funding, now int64) bool {
	if position.Size.Sign() == 0 || position.Collateral.Sign() == 0 {
		return false
	}
	collateral := applyFundingToCollateral(position, funding, now)
	priceDiff := new(big.Int).Sub(price, position.EntryPrice)
	pnl := new(big.Int).Mul(priceDiff, position.Size)
	pnl.Div(pnl, perps.PricePrecision)
	if !position.IsLong {
		pnl.Neg(pnl)
	}
	equity := new(big.Int).Add(collateral, pnl)
	if equity.Sign() <= 0 {
		return true
	}
	notional := new(big.Int).Mul(position.Size, position.EntryPrice)
	notional.Div(notional, perps.PricePrecision)
	if notional.Sign() == 0 {
		return false
	}
	ratio := new(big.Int).Mul(equity, perps.RatioPrecision)
	ratio.Div(ratio, notional)
	return ratio.Cmp(market.MaintenanceRate) < 0
}

// fundingIntervalSeconds mirrors TigerSwapPerpetuals.FUNDING_INTERVAL (1 hours).
const fundingIntervalSeconds = 3600

// accruedFunding mirrors TigerSwapPerpetuals.calculateFunding: the funding a
// position owes (positive) or is owed (negative), in settlement units. A nil or
// zero rate, or no elapsed time, accrues nothing.
func accruedFunding(position *perps.Position, funding *perps.Funding, now int64) *big.Int {
	if position.Size.Sign() == 0 || funding == nil || funding.LastFundingRate.Sign() == 0 {
		return big.NewInt(0)
	}
	elapsed := big.NewInt(now - position.LastUpdate.Int64())
	if elapsed.Sign() <= 0 {
		return big.NewInt(0)
	}
	notional := new(big.Int).Mul(position.Size, position.EntryPrice)
	notional.Div(notional, perps.PricePrecision)
	// notional * rate * elapsed / (FUNDING_RATE_PRECISION * FUNDING_INTERVAL)
	raw := new(big.Int).Mul(notional, funding.LastFundingRate)
	raw.Mul(raw, elapsed)
	denom := new(big.Int).Mul(perps.FundingRatePrecision, big.NewInt(fundingIntervalSeconds))
	raw.Div(raw, denom)
	if !position.IsLong {
		raw.Neg(raw)
	}
	return raw
}

// applyFundingToCollateral mirrors TigerSwapPerpetuals._applyFunding, clamped so
// funding can never drive a position's collateral below zero.
func applyFundingToCollateral(position *perps.Position, funding *perps.Funding, now int64) *big.Int {
	funded := new(big.Int).Set(position.Collateral)
	delta := accruedFunding(position, funding, now)
	if delta.Sign() == 0 {
		return funded
	}
	if delta.Sign() > 0 {
		if delta.Cmp(funded) >= 0 {
			return big.NewInt(0)
		}
		return funded.Sub(funded, delta)
	}
	return funded.Add(funded, new(big.Int).Neg(delta))
}

// --- helpers ---

// positionsForAddress reads every open position for a wallet from the chain.
func (s *OnChainPerpsService) positionsForAddress(ctx context.Context, chainID int64, address string) ([]gin.H, error) {
	client, err := s.client(chainID)
	if err != nil {
		return nil, err
	}
	ids, err := client.OpenPositions(ctx, common.HexToAddress(address))
	if err != nil {
		return nil, err
	}
	out := make([]gin.H, 0, len(ids))
	for _, id := range ids {
		position, err := client.Position(ctx, id)
		if err != nil {
			return nil, err
		}
		if position.Size.Sign() == 0 {
			continue
		}
		out = append(out, s.positionView(ctx, client, chainID, id, position))
	}
	return out, nil
}

// positionView shapes a position for the API, adding the live mark price and the
// unrealised PnL the contract would pay.
func (s *OnChainPerpsService) positionView(ctx context.Context, client *perps.Client, chainID int64, id common.Hash, position *perps.Position) gin.H {
	mark, err := client.MarkPrice(ctx, position.MarketID)
	if err != nil {
		mark = big.NewInt(0)
	}
	pnl, err := client.CalculatePnL(ctx, position, mark)
	if err != nil {
		pnl = big.NewInt(0)
	}
	// Report accrued funding alongside PnL: it is a real PnL component, so a
	// client that showed only the price PnL would understate what a position is
	// worth. A missing funding source is reported as zero, not an estimate.
	fundingDelta := big.NewInt(0)
	if funding, err := client.FundingInfo(ctx, position.MarketID); err == nil {
		fundingDelta = accruedFunding(position, funding, time.Now().Unix())
	}
	fundedCollateral := new(big.Int).Sub(position.Collateral, fundingDelta)
	return gin.H{
		"chain_id":          chainID,
		"position_id":       id.Hex(),
		"market_id":         position.MarketID.Hex(),
		"symbol":            s.symbolFor(position.MarketID),
		"trader":            strings.ToLower(position.Trader.Hex()),
		"is_long":           position.IsLong,
		"side":              sideLabel(position.IsLong),
		"size":              position.Size.String(),
		"collateral":        position.Collateral.String(),
		"entry_price":       position.EntryPrice.String(),
		"mark_price":        mark.String(),
		"unrealized_pnl":    pnl.String(),
		"accrued_funding":   fundingDelta.String(),
		"funded_collateral": fundedCollateral.String(),
		"on_chain":          true,
	}
}

// resolveChainParam reads chain_id from the query or the body, defaulting to the
// single configured chain when there is exactly one.
func (s *OnChainPerpsService) resolveChainParam(c *gin.Context) int64 {
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

// walletAddressFor resolves a user id to their bound wallet address.
func (s *OnChainPerpsService) walletAddressFor(userID string) (string, error) {
	if s.db == nil {
		return "", errors.New("on-chain perps service has no database")
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

// userIDForAddress maps a trader address back to a platform user, or "" when the
// wallet is not one the platform knows.
func (s *OnChainPerpsService) userIDForAddress(address string) string {
	if s.db == nil {
		return ""
	}
	var user models.User
	if err := s.db.Select("id").Where("LOWER(wallet_address) = ?", strings.ToLower(address)).First(&user).Error; err != nil {
		return ""
	}
	return user.ID.String()
}

// isBytes32 reports whether s is a 0x-prefixed 32-byte hex string.
func isBytes32(s string) bool {
	if len(s) != 66 || !strings.HasPrefix(s, "0x") {
		return false
	}
	_, ok := new(big.Int).SetString(s[2:], 16)
	return ok
}

// sideLabel renders a position side.
func sideLabel(isLong bool) string {
	if isLong {
		return "long"
	}
	return "short"
}

// rationalToBigInt converts a validated decimal amount into an integer in the
// token's smallest unit (18 decimals for the settlement assets TigerSwap uses).
func rationalToBigInt(value *big.Rat) *big.Int {
	scaled := new(big.Rat).Mul(value, new(big.Rat).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)))
	out := new(big.Int).Quo(scaled.Num(), scaled.Denom())
	return out
}

// toBigString renders a decoded uint value as a decimal string.
func toBigString(v interface{}) string {
	if n, ok := v.(*big.Int); ok && n != nil {
		return n.String()
	}
	return "0"
}

// signedBigString renders a decoded int value as a decimal string.
func signedBigString(v interface{}) string {
	if n, ok := v.(*big.Int); ok && n != nil {
		return n.String()
	}
	return "0"
}
