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

	"github.com/ethereum/go-ethereum/common"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"tigerswap/backend/go/orders"
)

// OnChainOrdersService exposes TigerSwap's deployed advanced-order and TWAMM
// contracts to the API. A limit, stop-loss, take-profit, TWAP, GTC, IOC or
// post-only order escrows its input on-chain, and a keeper fills it against the
// pool at the oracle-evaluated condition — the chain is the source of truth, so
// an order is verifiable by any user with an RPC endpoint.
//
// The service never holds a user's key: a build endpoint returns unsigned
// calldata for the user's own wallet to sign, exactly like the spot router and
// perps paths. The executor key is used only for the permissionless fill the
// platform submits itself.
type OnChainOrdersService struct {
	db *gorm.DB

	mu           sync.RWMutex
	clients      map[int64]*orders.Client
	twammClients map[int64]*orders.TWAMMClient
	executorKey  *ecdsa.PrivateKey

	running bool
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

// NewOnChainOrdersService builds the service. It dials the chains named by the
// deployment registry; a chain with no orders address is skipped rather than
// registered half-configured.
func NewOnChainOrdersService(db *gorm.DB) *OnChainOrdersService {
	return &OnChainOrdersService{
		db:           db,
		clients:      make(map[int64]*orders.Client),
		twammClients: make(map[int64]*orders.TWAMMClient),
	}
}

// LoadDeployments dials every configured chain and registers its order client.
func (s *OnChainOrdersService) LoadDeployments() error {
	for _, d := range orders.LoadDeployments() {
		if err := s.RegisterDeployment(d); err != nil {
			log.Printf("on-chain orders: could not register chain %d: %v", d.ChainID, err)
		}
	}
	return nil
}

// RegisterDeployment dials one chain and replaces any existing client for it.
// The executor key is applied to the new client, so a chain registered after the
// key was set still signs its fills.
func (s *OnChainOrdersService) RegisterDeployment(d orders.Deployment) error {
	if d.Orders == (common.Address{}) && d.TWAMM == (common.Address{}) {
		return nil
	}
	s.mu.Lock()
	key := s.executorKey
	s.mu.Unlock()

	if d.Orders != (common.Address{}) {
		client, err := orders.NewClient(d, key)
		if err != nil {
			return err
		}
		s.mu.Lock()
		if old, ok := s.clients[d.ChainID]; ok {
			old.Close()
		}
		s.clients[d.ChainID] = client
		s.mu.Unlock()
	}
	if d.TWAMM != (common.Address{}) {
		tc, err := orders.NewTWAMMClient(d)
		if err != nil {
			return err
		}
		tc.SetSigner(key)
		s.mu.Lock()
		if old, ok := s.twammClients[d.ChainID]; ok {
			old.Close()
		}
		s.twammClients[d.ChainID] = tc
		s.mu.Unlock()
	}
	return nil
}

// SetExecutorKey wires the signer used to fill resting orders. It is optional:
// without it the build and read endpoints still work, and the executor loop is
// simply not started. The key is applied to every already-registered client, so
// setting it after LoadDeployments still produces signing clients.
func (s *OnChainOrdersService) SetExecutorKey(key *ecdsa.PrivateKey) {
	s.mu.Lock()
	s.executorKey = key
	for _, c := range s.clients {
		c.SetSigner(key)
	}
	for _, tc := range s.twammClients {
		tc.SetSigner(key)
	}
	s.mu.Unlock()
}

func (s *OnChainOrdersService) client(chainID int64) (*orders.Client, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.clients[chainID]
	if !ok {
		return nil, fmt.Errorf("no on-chain orders deployment for chain %d", chainID)
	}
	return c, nil
}

func (s *OnChainOrdersService) twammClient(chainID int64) (*orders.TWAMMClient, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.twammClients[chainID]
	if !ok {
		return nil, fmt.Errorf("no on-chain TWAMM deployment for chain %d", chainID)
	}
	return c, nil
}

// Chains returns the chain ids with a live orders deployment.
func (s *OnChainOrdersService) Chains() []int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]int64, 0, len(s.clients))
	for id := range s.clients {
		out = append(out, id)
	}
	return out
}

// --- HTTP handlers: build ---

type buildOrderRequest struct {
	ChainID      int64  `json:"chain_id"`
	OrderType    string `json:"order_type"`
	Side         string `json:"side"`
	TokenIn      string `json:"token_in"`
	TokenOut     string `json:"token_out"`
	AmountIn     string `json:"amount_in"`
	AmountOutMin string `json:"amount_out_min"`
	Price        string `json:"price"`
	StopPrice    string `json:"stop_price"`
	ExpiresIn    int64  `json:"expires_in"`
	TWAPParts    int64  `json:"twap_parts"`
	TWAPInterval int64  `json:"twap_interval"`
}

// BuildOrderHandler returns unsigned calldata that creates an advanced order.
// The order's condition is evaluated on-chain against the price oracle, so the
// client cannot assert that a limit or stop has been hit.
func (s *OnChainOrdersService) BuildOrderHandler(c *gin.Context) {
	if _, ok := authenticatedUserID(c); !ok {
		return
	}
	var req buildOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequestError(c, err)
		return
	}
	orderType, err := orders.ParseOrderType(req.OrderType)
	if err != nil {
		respondBadRequestError(c, err)
		return
	}
	side, err := orders.ParseOrderSide(req.Side)
	if err != nil {
		respondBadRequestError(c, err)
		return
	}
	if !common.IsHexAddress(strings.TrimSpace(req.TokenIn)) || !common.IsHexAddress(strings.TrimSpace(req.TokenOut)) {
		respondBadRequestError(c, errors.New("token_in and token_out must be addresses"))
		return
	}
	tokenIn := common.HexToAddress(req.TokenIn)
	tokenOut := common.HexToAddress(req.TokenOut)

	amountIn, err := orderAmountValue(req.AmountIn)
	if err != nil {
		respondBadRequestError(c, err)
		return
	}
	amountOutMin, err := optionalOrderAmountValue(req.AmountOutMin)
	if err != nil {
		respondBadRequestError(c, err)
		return
	}
	price, err := optionalOrderAmountValue(req.Price)
	if err != nil {
		respondBadRequestError(c, err)
		return
	}
	stopPrice, err := optionalOrderAmountValue(req.StopPrice)
	if err != nil {
		respondBadRequestError(c, err)
		return
	}
	expiresAt, err := orderExpiry(req.ExpiresIn)
	if err != nil {
		respondBadRequestError(c, err)
		return
	}

	client, err := s.client(req.ChainID)
	if err != nil {
		respondServiceError(c, err)
		return
	}

	var call orders.Call
	switch orderType {
	case orders.TypeLimit:
		call, err = client.BuildCreateLimitOrder(side, tokenIn, tokenOut, amountIn, amountOutMin, price, expiresAt)
	case orders.TypeStopLoss:
		call, err = client.BuildCreateStopLossOrder(side, tokenIn, tokenOut, amountIn, amountOutMin, stopPrice, expiresAt)
	case orders.TypeTakeProfit:
		call, err = client.BuildCreateTakeProfitOrder(side, tokenIn, tokenOut, amountIn, amountOutMin, price, expiresAt)
	case orders.TypeTWAP:
		parts := big.NewInt(req.TWAPParts)
		interval := big.NewInt(req.TWAPInterval)
		call, err = client.BuildCreateTWAPOrder(side, tokenIn, tokenOut, amountIn, amountOutMin, parts, interval, expiresAt)
	case orders.TypeGTC:
		call, err = client.BuildCreateGTCOrder(side, tokenIn, tokenOut, amountIn, amountOutMin, price)
	case orders.TypeIOC:
		call, err = client.BuildCreateIOCOrder(side, tokenIn, tokenOut, amountIn, amountOutMin, price)
	case orders.TypePostOnly:
		call, err = client.BuildCreatePostOnlyOrder(side, tokenIn, tokenOut, amountIn, amountOutMin, price, expiresAt)
	default:
		respondBadRequestError(c, errors.New("unsupported order type for on-chain execution"))
		return
	}
	if err != nil {
		respondBadRequestError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"to":         call.To.Hex(),
		"data":       call.Hex(),
		"value":      "0",
		"chain_id":   req.ChainID,
		"order_type": strings.ToLower(strings.TrimSpace(req.OrderType)),
	})
}

// ListOrdersHandler reads the caller's on-chain orders for a chain.
func (s *OnChainOrdersService) ListOrdersHandler(c *gin.Context) {
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
	ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
	defer cancel()

	ids, err := client.UserOrders(ctx, common.HexToAddress(address))
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
		out = append(out, orderView(order))
	}
	c.JSON(http.StatusOK, gin.H{"orders": out, "count": len(out), "chain_id": chainID})
}

// CancelOrderHandler returns unsigned calldata that cancels the caller's own
// on-chain order. The order's owner is checked on-chain, so the calldata can be
// built safely and the contract enforces the same rule.
func (s *OnChainOrdersService) CancelOrderHandler(c *gin.Context) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}
	chainID := s.resolveChainParam(c)
	orderID, ok := parseOrderID(c.Param("id"))
	if !ok {
		respondBadRequestError(c, errors.New("invalid order id"))
		return
	}
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
	ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
	defer cancel()

	order, err := client.Order(ctx, orderID)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	if !strings.EqualFold(order.Owner.Hex(), address) {
		respondForbiddenError(c, errors.New("order does not belong to this account"))
		return
	}
	call, err := client.BuildCancelOrder(orderID)
	if err != nil {
		respondBadRequestError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"to": call.To.Hex(), "data": call.Hex(), "value": "0", "chain_id": chainID})
}

// BuildTWAMMOrderHandler returns unsigned calldata that creates a TWAMM order.
func (s *OnChainOrdersService) BuildTWAMMOrderHandler(c *gin.Context) {
	if _, ok := authenticatedUserID(c); !ok {
		return
	}
	var req struct {
		ChainID       int64  `json:"chain_id"`
		IsToken0      bool   `json:"is_token0"`
		SellRate      string `json:"sell_rate"`
		DurationHours int64  `json:"duration_hours"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequestError(c, err)
		return
	}
	sellRate, err := orderAmountValue(req.SellRate)
	if err != nil {
		respondBadRequestError(c, err)
		return
	}
	if req.DurationHours <= 0 {
		respondBadRequestError(c, errors.New("duration_hours must be positive"))
		return
	}
	tc, err := s.twammClient(req.ChainID)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	expiration := big.NewInt(time.Now().Add(time.Duration(req.DurationHours) * time.Hour).Unix())
	call, err := tc.BuildCreateOrder(req.IsToken0, sellRate, expiration)
	if err != nil {
		respondBadRequestError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"to": call.To.Hex(), "data": call.Hex(), "value": "0", "chain_id": req.ChainID})
}

// ListTWAMMOrdersHandler reads the long-term orders on a chain.
func (s *OnChainOrdersService) ListTWAMMOrdersHandler(c *gin.Context) {
	chainID := s.resolveChainParam(c)
	tc, err := s.twammClient(chainID)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
	defer cancel()
	count, err := tc.OrderCount(ctx)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	out := make([]gin.H, 0)
	for i := big.NewInt(1); i.Cmp(count) <= 0; i.Add(i, big.NewInt(1)) {
		order, err := tc.Order(ctx, i)
		if err != nil {
			continue
		}
		out = append(out, gin.H{
			"order_id":   i.String(),
			"owner":      order.Owner.Hex(),
			"is_token0":  order.IsToken0,
			"sell_rate":  order.SellRate.String(),
			"expiration": order.Expiration.String(),
			"escrow":     order.Escrow.String(),
			"proceeds":   order.Proceeds.String(),
			"active":     order.Active,
		})
	}
	c.JSON(http.StatusOK, gin.H{"orders": out, "count": len(out), "chain_id": chainID})
}

// --- background executor ---

// Start launches the executor loop for every configured chain when an executor
// key is present.
func (s *OnChainOrdersService) Start(ctx context.Context) {
	s.mu.Lock()
	if s.running || s.executorKey == nil {
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
			s.runExecutor(ctx, id)
		}(chainID)
	}
}

// Stop halts the background executor.
func (s *OnChainOrdersService) Stop() {
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

func (s *OnChainOrdersService) runExecutor(ctx context.Context, chainID int64) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.executeOnce(ctx, chainID); err != nil {
				log.Printf("on-chain orders executor: chain %d: %v", chainID, err)
			}
		}
	}
}

// executeOnce fills every open or partially-filled order whose on-chain price
// condition is met. The condition is re-evaluated by the contract, so a fill
// that no longer qualifies is rejected on-chain rather than mispriced here.
func (s *OnChainOrdersService) executeOnce(ctx context.Context, chainID int64) error {
	client, err := s.client(chainID)
	if err != nil {
		return err
	}
	ids, err := client.OrdersByStatus(ctx, orders.StatusOpen)
	if err != nil {
		return err
	}
	partial, err := client.OrdersByStatus(ctx, orders.StatusPartiallyFilled)
	if err != nil {
		return err
	}
	ids = append(ids, partial...)
	if len(ids) == 0 {
		return nil
	}
	var firstErr error
	for _, id := range ids {
		order, err := client.Order(ctx, id)
		if err != nil {
			continue
		}
		var call orders.Call
		if order.OrderType == orders.TypeStopLoss || order.OrderType == orders.TypeTakeProfit {
			call, err = client.BuildTriggerOrder(id)
		} else {
			call, err = client.BuildExecuteOrder(id, big.NewInt(0))
		}
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if _, err := client.Send(ctx, call); err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("fill order %s: %w", id.String(), err)
			}
		}
	}
	return firstErr
}

// --- helpers ---

func (s *OnChainOrdersService) resolveChainParam(c *gin.Context) int64 {
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

func (s *OnChainOrdersService) walletAddressFor(userID string) (string, error) {
	if s.db == nil {
		return "", errors.New("on-chain orders service has no database")
	}
	var user userWalletRow
	if err := s.db.Table("users").Select("wallet_address").Where("id = ?", userID).First(&user).Error; err != nil {
		return "", fmt.Errorf("resolve wallet: %w", err)
	}
	if strings.TrimSpace(user.WalletAddress) == "" {
		return "", errNoWalletBound
	}
	return user.WalletAddress, nil
}

type userWalletRow struct {
	WalletAddress string
}

// orderView renders an on-chain order for the API.
func orderView(o *orders.Order) gin.H {
	return gin.H{
		"order_id":            o.ID.String(),
		"owner":               o.Owner.Hex(),
		"order_type":          orderTypeLabel(o.OrderType),
		"side":                orderSideLabel(o.Side),
		"token_in":            o.TokenIn.Hex(),
		"token_out":           o.TokenOut.Hex(),
		"amount_in":           o.AmountIn.String(),
		"amount_out_min":      o.AmountOutMin.String(),
		"price":               o.Price.String(),
		"stop_price":          o.StopPrice.String(),
		"filled_amount":       o.FilledAmount.String(),
		"amount_out_received": o.AmountOutReceived.String(),
		"status":              orderStatusLabel(o.Status),
		"expires_at":          o.ExpiresAt.String(),
		"created_at":          o.CreatedAt.String(),
	}
}

func orderTypeLabel(t orders.OrderType) string {
	switch t {
	case orders.TypeLimit:
		return "limit"
	case orders.TypeStopLoss:
		return "stop_loss"
	case orders.TypeTakeProfit:
		return "take_profit"
	case orders.TypeTWAP:
		return "twap"
	case orders.TypeGTC:
		return "gtc"
	case orders.TypeIOC:
		return "ioc"
	case orders.TypePostOnly:
		return "post_only"
	case orders.TypeMarket:
		return "market"
	default:
		return "unknown"
	}
}

func orderSideLabel(s orders.OrderSide) string {
	if s == orders.SideSell {
		return "sell"
	}
	return "buy"
}

func orderStatusLabel(s orders.OrderStatus) string {
	switch s {
	case orders.StatusPending:
		return "pending"
	case orders.StatusOpen:
		return "open"
	case orders.StatusPartiallyFilled:
		return "partially_filled"
	case orders.StatusFilled:
		return "filled"
	case orders.StatusCancelled:
		return "cancelled"
	case orders.StatusExpired:
		return "expired"
	case orders.StatusTriggered:
		return "triggered"
	default:
		return "unknown"
	}
}

// orderAmountValue parses a required positive decimal amount into an 18-decimal
// integer in the token's smallest unit.
func orderAmountValue(raw string) (*big.Int, error) {
	return validatePositiveDecimal("amount", raw)
}

// optionalOrderAmountValue parses an optional decimal amount, treating an absent
// or zero value as zero.
func optionalOrderAmountValue(raw string) (*big.Int, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || trimmed == "0" {
		return big.NewInt(0), nil
	}
	return validatePositiveDecimal("amount", trimmed)
}

// orderExpiry converts a relative expiry in seconds into an absolute unix
// timestamp. A non-positive value means no expiry.
func orderExpiry(seconds int64) (*big.Int, error) {
	if seconds < 0 {
		return nil, errors.New("expires_in must not be negative")
	}
	if seconds == 0 {
		return big.NewInt(0), nil
	}
	return big.NewInt(time.Now().Add(time.Duration(seconds) * time.Second).Unix()), nil
}

// parseOrderID parses a positive order id.
func parseOrderID(raw string) (*big.Int, bool) {
	id, ok := new(big.Int).SetString(strings.TrimSpace(raw), 10)
	if !ok || id.Sign() <= 0 {
		return nil, false
	}
	return id, true
}
