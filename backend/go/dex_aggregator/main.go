// Package dex_aggregator is TigerSwap's own aggregator (TigerAggregator).
//
// It quotes and routes exclusively across TigerSwap's own pool families
// (V2 xyK, V3 concentrated, StableSwap, Weighted) by reading reserves from the
// TigerSwap factory/pool contracts. It never calls a third-party DEX, router or
// aggregator API.
package main

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strconv"
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

	"tigerswap/backend/go/amm"
)

// factoryABIJSON resolves TigerSwap pools by token pair and family.
const factoryABIJSON = `[
 {"inputs":[{"internalType":"address","name":"tokenA","type":"address"},{"internalType":"address","name":"tokenB","type":"address"},{"internalType":"uint8","name":"family","type":"uint8"}],"name":"getPoolForFamily","outputs":[{"internalType":"address","name":"pool","type":"address"}],"stateMutability":"view","type":"function"}
]`

// poolABIJSON reads the state needed to quote a TigerSwap pool.
const poolABIJSON = `[
 {"inputs":[],"name":"getReserves","outputs":[{"internalType":"uint256","name":"reserve0","type":"uint256"},{"internalType":"uint256","name":"reserve1","type":"uint256"},{"internalType":"uint32","name":"blockTimestampLast","type":"uint32"}],"stateMutability":"view","type":"function"},
 {"inputs":[],"name":"token0","outputs":[{"internalType":"address","name":"","type":"address"}],"stateMutability":"view","type":"function"},
 {"inputs":[],"name":"token1","outputs":[{"internalType":"address","name":"","type":"address"}],"stateMutability":"view","type":"function"},
 {"inputs":[{"internalType":"uint256","name":"","type":"uint256"}],"name":"coins","outputs":[{"internalType":"address","name":"","type":"address"}],"stateMutability":"view","type":"function"},
 {"inputs":[{"internalType":"uint256","name":"","type":"uint256"}],"name":"balances","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},
 {"inputs":[],"name":"A","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},
 {"inputs":[{"internalType":"uint256","name":"","type":"uint256"}],"name":"tokens","outputs":[{"internalType":"address","name":"","type":"address"}],"stateMutability":"view","type":"function"},
 {"inputs":[],"name":"getBalances","outputs":[{"internalType":"uint256[]","name":"","type":"uint256[]"}],"stateMutability":"view","type":"function"},
 {"inputs":[],"name":"getWeights","outputs":[{"internalType":"uint256[]","name":"","type":"uint256[]"}],"stateMutability":"view","type":"function"}
]`

// routerABIJSON executes a TigerSwap swap.
const routerABIJSON = `[
 {"inputs":[{"internalType":"address","name":"tokenIn","type":"address"},{"internalType":"address","name":"tokenOut","type":"address"},{"internalType":"uint8","name":"family","type":"uint8"},{"internalType":"uint256","name":"amountIn","type":"uint256"},{"internalType":"uint256","name":"amountOutMin","type":"uint256"},{"internalType":"address","name":"to","type":"address"},{"internalType":"uint256","name":"deadline","type":"uint256"}],"name":"swapExactTokensForTokens","outputs":[{"internalType":"uint256","name":"amountOut","type":"uint256"}],"stateMutability":"nonpayable","type":"function"}
]`

var (
	factoryABI = mustABI(factoryABIJSON)
	poolABI    = mustABI(poolABIJSON)
	routerABI  = mustABI(routerABIJSON)
)

func mustABI(s string) abi.ABI {
	parsed, err := abi.JSON(strings.NewReader(s))
	if err != nil {
		panic(fmt.Sprintf("invalid embedded ABI: %v", err))
	}
	return parsed
}

// familyCode maps a pool family to the uint8 used on-chain.
func familyCode(f amm.Family) uint8 {
	switch f {
	case amm.FamilyV2:
		return 0
	case amm.FamilyV3:
		return 1
	case amm.FamilyStable:
		return 2
	case amm.FamilyWeighted:
		return 3
	}
	return 0
}

// Route is a single-family swap route.
type Route struct {
	Family      string   `json:"family"`
	Path        []string `json:"path"`
	AmountIn    *big.Int `json:"amount_in"`
	AmountOut   *big.Int `json:"amount_out"`
	FeeBps      uint32   `json:"fee_bps"`
	GasEstimate uint64   `json:"gas_estimate"`
}

// QuoteRequest is an aggregator quote request.
type QuoteRequest struct {
	ChainID   int64    `json:"chain_id"`
	FromToken string   `json:"from_token"`
	ToToken   string   `json:"to_token"`
	AmountIn  *big.Int `json:"amount_in"`
	Slippage  float64  `json:"slippage"`
	Family    string   `json:"family"`
}

// QuoteResponse is an aggregator quote response.
type QuoteResponse struct {
	FromToken    string   `json:"from_token"`
	ToToken      string   `json:"to_token"`
	AmountIn     *big.Int `json:"amount_in"`
	AmountOut    *big.Int `json:"amount_out"`
	AmountOutMin *big.Int `json:"amount_out_min"`
	PriceImpact  float64  `json:"price_impact"`
	GasEstimate  uint64   `json:"gas_estimate"`
	Routes       []Route  `json:"routes"`
	BestRoute    *Route   `json:"best_route"`
}

// familyFees maps each family to its default fee tier in bps.
var familyFees = map[amm.Family]uint32{
	amm.FamilyV2:       30, // 0.30%
	amm.FamilyV3:       5,  // 0.05%
	amm.FamilyStable:   1,  // 0.01%
	amm.FamilyWeighted: 30, // 0.30%
}

// chainDeployment holds the TigerSwap contracts for one chain.
type chainDeployment struct {
	chainID int64
	client  *ethclient.Client
	factory common.Address
	router  common.Address
	signer  *ecdsa.PrivateKey
	mu      sync.RWMutex
	pools   map[string]common.Address
}

// Aggregator is TigerSwap's own multi-family aggregator.
type Aggregator struct {
	mu       sync.RWMutex
	chains   map[int64]*chainDeployment
	families []amm.Family
}

// NewAggregator builds an aggregator from environment configuration.
func NewAggregator() (*Aggregator, error) {
	signer, err := aggregatorSigner()
	if err != nil {
		return nil, err
	}

	chains := make(map[int64]*chainDeployment)
	for _, chainID := range configuredChains() {
		suffix := strconv.FormatInt(chainID, 10)
		rpc := os.Getenv("TIGERSWAP_RPC_" + suffix)
		factory := os.Getenv("TIGERSWAP_FACTORY_" + suffix)
		router := os.Getenv("TIGERSWAP_ROUTER_" + suffix)
		if rpc == "" || factory == "" || router == "" {
			log.Printf("chain %d: TigerSwap contracts not fully configured; skipping", chainID)
			continue
		}
		client, err := ethclient.Dial(rpc)
		if err != nil {
			log.Printf("chain %d: dial rpc failed: %v", chainID, err)
			continue
		}
		chains[chainID] = &chainDeployment{
			chainID: chainID,
			client:  client,
			factory: common.HexToAddress(factory),
			router:  common.HexToAddress(router),
			signer:  signer,
			pools:   make(map[string]common.Address),
		}
	}

	return &Aggregator{
		chains: chains,
		families: []amm.Family{
			amm.FamilyV2, amm.FamilyV3, amm.FamilyStable, amm.FamilyWeighted,
		},
	}, nil
}

// Close releases every chain RPC client.
func (a *Aggregator) Close() {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, d := range a.chains {
		d.client.Close()
	}
}

func aggregatorSigner() (*ecdsa.PrivateKey, error) {
	raw := os.Getenv("TIGERSWAP_SIGNER_KEY")
	if raw == "" {
		return nil, errors.New("TIGERSWAP_SIGNER_KEY is not configured")
	}
	return crypto.HexToECDSA(strings.TrimPrefix(raw, "0x"))
}

func configuredChains() []int64 {
	raw := os.Getenv("TIGERSWAP_CHAINS")
	if raw == "" {
		return nil
	}
	var chains []int64
	for _, part := range strings.Split(raw, ",") {
		id, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64)
		if err != nil {
			continue
		}
		chains = append(chains, id)
	}
	return chains
}

// poolState captures on-chain pool state for quoting.
type poolState struct {
	token0   common.Address
	reserve0 *big.Int
	reserve1 *big.Int
	amp      *big.Int
	weight0  *big.Int
	weight1  *big.Int
}

// resolvePool returns the TigerSwap pool for a pair and family, with caching.
func (d *chainDeployment) resolvePool(ctx context.Context, tokenA, tokenB common.Address, family amm.Family) (common.Address, error) {
	key := fmt.Sprintf("%s:%s:%d", strings.ToLower(tokenA.Hex()), strings.ToLower(tokenB.Hex()), familyCode(family))
	d.mu.RLock()
	cached, ok := d.pools[key]
	d.mu.RUnlock()
	if ok {
		return cached, nil
	}

	data, err := factoryABI.Pack("getPoolForFamily", tokenA, tokenB, familyCode(family))
	if err != nil {
		return common.Address{}, err
	}
	out, err := d.client.CallContract(ctx, ethereum.CallMsg{To: &d.factory, Data: data}, nil)
	if err != nil {
		return common.Address{}, fmt.Errorf("factory.getPool: %w", err)
	}
	vals, err := factoryABI.Unpack("getPoolForFamily", out)
	if err != nil {
		return common.Address{}, fmt.Errorf("unpack getPool: %w", err)
	}
	pool, _ := vals[0].(common.Address)
	if pool == (common.Address{}) {
		return common.Address{}, fmt.Errorf("no %s pool for pair", family)
	}

	d.mu.Lock()
	d.pools[key] = pool
	d.mu.Unlock()
	return pool, nil
}

// loadState reads reserves and family parameters from a pool.
func (d *chainDeployment) loadState(ctx context.Context, pool common.Address, family amm.Family) (*poolState, error) {
	st := &poolState{amp: big.NewInt(0), weight0: big.NewInt(0), weight1: big.NewInt(0)}

	switch family {
	case amm.FamilyStable:
		// StableSwap pools expose coins(i), balances(i) and A.
		for i := 0; i < 2; i++ {
			coin, err := aggCallAddress(ctx, d.client, pool, "coins", big.NewInt(int64(i)))
			if err != nil {
				return nil, err
			}
			bal, err := aggCallUint(ctx, d.client, pool, "balances", big.NewInt(int64(i)))
			if err != nil {
				return nil, err
			}
			if i == 0 {
				st.token0, st.reserve0 = coin, bal
			} else {
				st.reserve1 = bal
			}
		}
		amp, err := aggCallUint(ctx, d.client, pool, "A")
		if err != nil {
			return nil, err
		}
		st.amp = amp
	case amm.FamilyWeighted:
		// Weighted pools expose tokens(i), getBalances() and getWeights().
		for i := 0; i < 2; i++ {
			token, err := aggCallAddress(ctx, d.client, pool, "tokens", big.NewInt(int64(i)))
			if err != nil {
				return nil, err
			}
			if i == 0 {
				st.token0 = token
			}
		}
		balances, err := aggCallUintArray(ctx, d.client, pool, "getBalances")
		if err != nil {
			return nil, err
		}
		weights, err := aggCallUintArray(ctx, d.client, pool, "getWeights")
		if err != nil {
			return nil, err
		}
		if len(balances) < 2 || len(weights) < 2 {
			return nil, fmt.Errorf("weighted pool %s returned %d balances / %d weights", pool.Hex(), len(balances), len(weights))
		}
		st.reserve0, st.reserve1 = balances[0], balances[1]
		st.weight0, st.weight1 = weights[0], weights[1]
	default:
		// xyK pools expose getReserves() and token0().
		resOut, err := d.client.CallContract(ctx, ethereum.CallMsg{To: &pool, Data: mustPack(poolABI, "getReserves")}, nil)
		if err != nil {
			return nil, fmt.Errorf("getReserves: %w", err)
		}
		res, err := poolABI.Unpack("getReserves", resOut)
		if err != nil || len(res) < 2 {
			return nil, fmt.Errorf("unpack getReserves: %w", err)
		}
		st.reserve0, _ = res[0].(*big.Int)
		st.reserve1, _ = res[1].(*big.Int)
		t0, err := aggCallAddress(ctx, d.client, pool, "token0")
		if err != nil {
			return nil, err
		}
		st.token0 = t0
	}

	if st.reserve0 == nil || st.reserve1 == nil || st.reserve0.Sign() == 0 || st.reserve1.Sign() == 0 {
		return nil, fmt.Errorf("pool %s has no usable liquidity", pool.Hex())
	}
	return st, nil
}

// aggCallAddress reads a single address-returning view function.
func aggCallAddress(ctx context.Context, client *ethclient.Client, pool common.Address, method string, args ...interface{}) (common.Address, error) {
	out, err := client.CallContract(ctx, ethereum.CallMsg{To: &pool, Data: mustPack(poolABI, method, args...)}, nil)
	if err != nil {
		return common.Address{}, fmt.Errorf("%s: %w", method, err)
	}
	vals, err := poolABI.Unpack(method, out)
	if err != nil || len(vals) < 1 {
		return common.Address{}, fmt.Errorf("unpack %s: %w", method, err)
	}
	addr, ok := vals[0].(common.Address)
	if !ok {
		return common.Address{}, fmt.Errorf("unexpected %s type %T", method, vals[0])
	}
	return addr, nil
}

// aggCallUint reads a single uint256-returning view function.
func aggCallUint(ctx context.Context, client *ethclient.Client, pool common.Address, method string, args ...interface{}) (*big.Int, error) {
	out, err := client.CallContract(ctx, ethereum.CallMsg{To: &pool, Data: mustPack(poolABI, method, args...)}, nil)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", method, err)
	}
	vals, err := poolABI.Unpack(method, out)
	if err != nil || len(vals) < 1 {
		return nil, fmt.Errorf("unpack %s: %w", method, err)
	}
	v, ok := vals[0].(*big.Int)
	if !ok {
		return nil, fmt.Errorf("unexpected %s type %T", method, vals[0])
	}
	return v, nil
}

// aggCallUintArray reads a uint256[]-returning view function.
func aggCallUintArray(ctx context.Context, client *ethclient.Client, pool common.Address, method string, args ...interface{}) ([]*big.Int, error) {
	out, err := client.CallContract(ctx, ethereum.CallMsg{To: &pool, Data: mustPack(poolABI, method, args...)}, nil)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", method, err)
	}
	vals, err := poolABI.Unpack(method, out)
	if err != nil || len(vals) < 1 {
		return nil, fmt.Errorf("unpack %s: %w", method, err)
	}
	arr, ok := vals[0].([]*big.Int)
	if !ok {
		return nil, fmt.Errorf("unexpected %s type %T", method, vals[0])
	}
	return arr, nil
}

func mustPack(a abi.ABI, name string, args ...interface{}) []byte {
	data, err := a.Pack(name, args...)
	if err != nil {
		panic(fmt.Sprintf("pack %s: %v", name, err))
	}
	return data
}

// quoteFamily computes the output for one family over a token path.
func (a *Aggregator) quoteFamily(ctx context.Context, d *chainDeployment, family amm.Family, path []common.Address, amountIn *big.Int) (*big.Int, error) {
	current := new(big.Int).Set(amountIn)
	fee := familyFees[family]

	for i := 0; i < len(path)-1; i++ {
		pool, err := d.resolvePool(ctx, path[i], path[i+1], family)
		if err != nil {
			return nil, err
		}
		st, err := d.loadState(ctx, pool, family)
		if err != nil {
			return nil, err
		}

		reserveIn, reserveOut := st.reserve0, st.reserve1
		weightIn, weightOut := st.weight0, st.weight1
		if st.token0 != path[i] {
			reserveIn, reserveOut = st.reserve1, st.reserve0
			weightIn, weightOut = st.weight1, st.weight0
		}

		switch family {
		case amm.FamilyV2:
			current, err = amm.AmountOutXY(current, reserveIn, reserveOut, fee)
		case amm.FamilyStable:
			current, err = amm.AmountOutStable(current, reserveIn, reserveOut, st.amp, fee)
		case amm.FamilyWeighted:
			current, err = amm.AmountOutWeighted(current, reserveIn, reserveOut, weightIn, weightOut, fee)
		case amm.FamilyV3:
			// V3 pools report their effective reserves; the concentrated math is
			// applied on-chain by the quoter, so use the xyK approximation here.
			current, err = amm.AmountOutXY(current, reserveIn, reserveOut, fee)
		default:
			return nil, fmt.Errorf("unsupported family %s", family)
		}
		if err != nil {
			return nil, err
		}
	}
	return current, nil
}

// GetQuote returns the best quote across all TigerSwap families on a chain.
func (a *Aggregator) GetQuote(ctx context.Context, req QuoteRequest) (*QuoteResponse, error) {
	if req.AmountIn == nil || req.AmountIn.Sign() <= 0 {
		return nil, errors.New("amount_in must be positive")
	}
	if !common.IsHexAddress(req.FromToken) || !common.IsHexAddress(req.ToToken) {
		return nil, errors.New("invalid token address")
	}

	a.mu.RLock()
	d, ok := a.chains[req.ChainID]
	a.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("chain %d has no TigerSwap deployment configured", req.ChainID)
	}

	path := []common.Address{common.HexToAddress(req.FromToken), common.HexToAddress(req.ToToken)}

	families := a.families
	if req.Family != "" {
		families = []amm.Family{amm.Family(req.Family)}
	}

	routes := make([]Route, 0, len(families))
	var firstErr error
	for _, family := range families {
		out, err := a.quoteFamily(ctx, d, family, path, req.AmountIn)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		routes = append(routes, Route{
			Family:      string(family),
			Path:        []string{req.FromToken, req.ToToken},
			AmountIn:    new(big.Int).Set(req.AmountIn),
			AmountOut:   out,
			FeeBps:      familyFees[family],
			GasEstimate: estimateGas(family),
		})
	}

	if len(routes) == 0 {
		if firstErr != nil {
			return nil, firstErr
		}
		return nil, errors.New("no TigerSwap routes available")
	}

	sort.Slice(routes, func(i, j int) bool {
		return routes[i].AmountOut.Cmp(routes[j].AmountOut) > 0
	})
	best := routes[0]

	return &QuoteResponse{
		FromToken:    req.FromToken,
		ToToken:      req.ToToken,
		AmountIn:     new(big.Int).Set(req.AmountIn),
		AmountOut:    best.AmountOut,
		AmountOutMin: amm.ApplySlippage(best.AmountOut, req.Slippage),
		PriceImpact:  amm.PriceImpactPercent(req.AmountIn, best.AmountOut),
		GasEstimate:  best.GasEstimate,
		Routes:       routes,
		BestRoute:    &best,
	}, nil
}

// GetMultiHopQuote quotes a path of three or more tokens hop by hop.
func (a *Aggregator) GetMultiHopQuote(ctx context.Context, chainID int64, path []string, amountIn *big.Int, slippage float64) (*QuoteResponse, error) {
	if len(path) < 2 {
		return nil, errors.New("path must have at least 2 tokens")
	}

	hops := make([]Route, 0, len(path)-1)
	current := new(big.Int).Set(amountIn)

	for i := 0; i < len(path)-1; i++ {
		quote, err := a.GetQuote(ctx, QuoteRequest{
			ChainID:   chainID,
			FromToken: path[i],
			ToToken:   path[i+1],
			AmountIn:  current,
			Slippage:  slippage,
		})
		if err != nil {
			return nil, fmt.Errorf("hop %d (%s->%s): %w", i, path[i], path[i+1], err)
		}
		hops = append(hops, *quote.BestRoute)
		current = quote.AmountOut
	}

	return &QuoteResponse{
		FromToken:    path[0],
		ToToken:      path[len(path)-1],
		AmountIn:     amountIn,
		AmountOut:    current,
		AmountOutMin: amm.ApplySlippage(current, slippage),
		PriceImpact:  amm.PriceImpactPercent(amountIn, current),
		Routes:       hops,
		BestRoute:    &hops[len(hops)-1],
	}, nil
}

// ExecuteSwap signs and broadcasts a swap on the given route.
func (a *Aggregator) ExecuteSwap(ctx context.Context, chainID int64, route Route, to string, deadline time.Time) (string, error) {
	a.mu.RLock()
	d, ok := a.chains[chainID]
	a.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("chain %d has no TigerSwap deployment configured", chainID)
	}
	if !common.IsHexAddress(to) {
		return "", errors.New("invalid recipient address")
	}
	if len(route.Path) < 2 {
		return "", errors.New("route path must have at least 2 tokens")
	}

	family := amm.Family(route.Family)
	data, err := routerABI.Pack("swapExactTokensForTokens",
		common.HexToAddress(route.Path[0]),
		common.HexToAddress(route.Path[len(route.Path)-1]),
		familyCode(family),
		route.AmountIn,
		amm.ApplySlippage(route.AmountOut, 0.5),
		common.HexToAddress(to),
		big.NewInt(deadline.Unix()),
	)
	if err != nil {
		return "", err
	}

	from := crypto.PubkeyToAddress(d.signer.PublicKey)
	nonce, err := d.client.PendingNonceAt(ctx, from)
	if err != nil {
		return "", fmt.Errorf("pending nonce: %w", err)
	}
	gasPrice, err := d.client.SuggestGasPrice(ctx)
	if err != nil {
		return "", fmt.Errorf("suggest gas price: %w", err)
	}
	gasLimit, err := d.client.EstimateGas(ctx, ethereum.CallMsg{
		From: from, To: &d.router, Data: data, Value: big.NewInt(0),
	})
	if err != nil {
		return "", fmt.Errorf("estimate gas: %w", err)
	}

	tx := types.NewTransaction(nonce, d.router, big.NewInt(0), gasLimit, gasPrice, data)
	signed, err := types.SignTx(tx, types.LatestSignerForChainID(big.NewInt(chainID)), d.signer)
	if err != nil {
		return "", fmt.Errorf("sign tx: %w", err)
	}
	if err := d.client.SendTransaction(ctx, signed); err != nil {
		return "", fmt.Errorf("send tx: %w", err)
	}
	return signed.Hash().Hex(), nil
}

// GetSupportedFamilies returns the pool families for a chain.
func (a *Aggregator) GetSupportedFamilies(chainID int64) []string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if _, ok := a.chains[chainID]; !ok {
		return nil
	}
	out := make([]string, 0, len(a.families))
	for _, f := range a.families {
		out = append(out, string(f))
	}
	return out
}

// estimateGas returns the gas estimate for a family.
func estimateGas(family amm.Family) uint64 {
	switch family {
	case amm.FamilyV2:
		return 120000
	case amm.FamilyV3:
		return 180000
	case amm.FamilyStable:
		return 150000
	case amm.FamilyWeighted:
		return 200000
	}
	return 200000
}

// ---------------------------------------------------------------------------
// HTTP
// ---------------------------------------------------------------------------

// Handler exposes the aggregator API.
type Handler struct {
	aggregator *Aggregator
}

// NewHandler builds an HTTP handler.
func NewHandler(agg *Aggregator) *Handler {
	return &Handler{aggregator: agg}
}

// GetQuote handles quote requests.
func (h *Handler) GetQuote(c *gin.Context) {
	var req QuoteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	quote, err := h.aggregator.GetQuote(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, quote)
}

// GetMultiHopQuote handles multi-hop quote requests.
func (h *Handler) GetMultiHopQuote(c *gin.Context) {
	var req struct {
		ChainID  int64    `json:"chain_id" binding:"required"`
		Path     []string `json:"path" binding:"required,min=2"`
		AmountIn string   `json:"amount_in" binding:"required"`
		Slippage float64  `json:"slippage"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	amountIn, ok := new(big.Int).SetString(req.AmountIn, 10)
	if !ok || amountIn.Sign() <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid amount_in"})
		return
	}

	quote, err := h.aggregator.GetMultiHopQuote(c.Request.Context(), req.ChainID, req.Path, amountIn, req.Slippage)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, quote)
}

// GetFamilies returns the TigerSwap pool families for a chain.
func (h *Handler) GetFamilies(c *gin.Context) {
	chainID, err := strconv.ParseInt(c.Query("chainId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid chainId"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"chainId": chainID, "families": h.aggregator.GetSupportedFamilies(chainID)})
}

func main() {
	agg, err := NewAggregator()
	if err != nil {
		log.Fatalf("failed to initialize TigerAggregator: %v", err)
	}
	defer agg.Close()

	handler := NewHandler(agg)

	r := gin.Default()
	r.POST("/api/v1/quote", handler.GetQuote)
	r.POST("/api/v1/quote/multihop", handler.GetMultiHopQuote)
	r.GET("/api/v1/families", handler.GetFamilies)
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "tigeraggregator"})
	})

	srv := &http.Server{Addr: ":8081", Handler: r}
	go func() {
		log.Println("Starting TigerAggregator on :8081")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}
	log.Println("Shutting down TigerAggregator...")
}
