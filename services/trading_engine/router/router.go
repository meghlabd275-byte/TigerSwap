package router

import (
	"context"
	"fmt"
	"math/big"
	"sort"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"

	"TigerSwap/services/trading_engine/config"
	"TigerSwap/services/trading_engine/dex"
)

// DEXRouter routes swaps across multiple DEXs
type DEXRouter struct {
	cfg       *config.Config
	client    *ethclient.Client
	dexs      map[string]dex.DEX
	quotes    map[string]*QuoteCache
	quotesMux sync.RWMutex
}

// QuoteCache caches quotes
type QuoteCache struct {
	AmountOut   *big.Int
	AmountIn   *big.Int
	DEX       string
	GasUsed   uint64
	Timestamp time.Time
}

// QuoteResult represents a quote from a DEX
type QuoteResult struct {
	DEX       string
	AmountOut *big.Int
	GasUsed  uint64
	Path    []string
}

// RouteResult represents a route with multiple hops
type RouteResult struct {
	Path      []string
	AmountOut *big.Int
	GasUsed   uint64
	InputAmount *big.Int
}

// NewDEXRouter creates a new DEX router
func NewDEXRouter(cfg *config.Config) (*DEXRouter, error) {
	client, err := ethclient.Dial(cfg.RPCURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RPC: %w", err)
	}

	dexs := make(map[string]dex.DEX)
	for _, dexCfg := range cfg.DEXs {
		if !dexCfg.Enabled {
			continue
		}

		d, err := dex.NewDEX(&dexCfg, client)
		if err != nil {
			fmt.Printf("[Router] Failed to initialize %s: %v\n", dexCfg.Name, err)
			continue
		}

		dexs[dexCfg.Name] = d
		fmt.Printf("[Router] Initialized %s\n", dexCfg.Name)
	}

	return &DEXRouter{
		cfg:    cfg,
		client: client,
		dexs:   dexs,
		quotes: make(map[string]*QuoteCache),
	}, nil
}

// GetQuote gets the best quote across all DEXs
func (r *DEXRouter) GetQuote(tokenIn, tokenOut string, amountIn *big.Int) (*big.Int, error) {
	quotes, err := r.GetQuotes(tokenIn, tokenOut, amountIn)
	if err != nil {
		return nil, err
	}

	if len(quotes) == 0 {
		return nil, fmt.Errorf("no quotes available")
	}

	// Return best quote (highest amount out)
	best := quotes[0]
	for _, q := range quotes {
		if q.AmountOut.Cmp(best.AmountOut) > 0 {
			best = q
		}
	}

	return best.AmountOut, nil
}

// GetQuotes gets quotes from all DEXs
func (r *DEXRouter) GetQuotes(tokenIn, tokenOut string, amountIn *big.Int) ([]QuoteResult, error) {
	var results []QuoteResult

	// Sort DEXs by priority
	sortedDEXs := make([]string, 0, len(r.dexs))
	for name := range r.dexs {
		sortedDEXs = append(sortedDEXs, name)
	}
	sort.Slice(sortedDEXs, func(i, j int) bool {
		return r.dexConfig(sortedDEXs[i]).Priority < r.dexConfig(sortedDEXs[j]).Priority
	})

	// Get quotes from each DEX
	for _, name := range sortedDEXs {
		d := r.dexs[name]

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		amountOut, err := d.GetQuote(ctx, amountIn)
		if err != nil {
			continue
		}

		results = append(results, QuoteResult{
			DEX:       name,
			AmountOut: amountOut,
			GasUsed:    150000, // Estimated
			Path:      []string{tokenIn, tokenOut},
		})
	}

	// Sort by amount out (descending)
	sort.Slice(results, func(i, j int) bool {
		return results[i].AmountOut.Cmp(results[j].AmountOut) > 0
	})

	return results, nil
}

// GetRoute gets the best route for a swap
func (r *DEXRouter) GetRoute(tokenIn, tokenOut string, amountIn *big.Int) (*RouteResult, error) {
	quotes, err := r.GetQuotes(tokenIn, tokenOut, amountIn)
	if err != nil {
		return nil, err
	}

	if len(quotes) == 0 {
		return nil, fmt.Errorf("no route available")
	}

	best := quotes[0]
	return &RouteResult{
		Path:       best.Path,
		AmountOut: best.AmountOut,
		GasUsed:   best.GasUsed,
		InputAmount: amountIn,
	}, nil
}

// GetSplitRoute gets a split route across multiple DEXs
func (r *DEXRouter) GetSplitRoute(tokenIn, tokenOut string, amountIn *big.Int, maxSplits int) ([]RouteResult, error) {
	quotes, err := r.GetQuotes(tokenIn, tokenOut, amountIn)
	if err != nil {
		return nil, err
	}

	if len(quotes) == 0 {
		return nil, fmt.Errorf("no route available")
	}

	// Limit splits
	if maxSplits > len(quotes) {
		maxSplits = len(quotes)
	}

	// Calculate splits based on liquidity
	var routes []RouteResult
	totalLiquidity := big.NewInt(0)
	for i := 0; i < maxSplits; i++ {
		totalLiquidity = new(big.Int).Add(totalLiquidity, quotes[i].AmountOut)
	}

	for i := 0; i < maxSplits; i++ {
		// Proportional split
		splitAmount := new(big.Int).Mul(amountIn, quotes[i].AmountOut)
		splitAmount = new(big.Int).Div(splitAmount, totalLiquidity)

		routes = append(routes, RouteResult{
			Path:        quotes[i].Path,
			AmountOut:   quotes[i].AmountOut,
			GasUsed:     quotes[i].GasUsed,
			InputAmount: splitAmount,
		})
	}

	return routes, nil
}

// ExecuteSwap executes a swap via the best DEX
func (r *DEXRouter) ExecuteSwap(tokenIn, tokenOut string, amountIn, amountOutMin *big.Int, recipient string) (string, error) {
	// Get best route
	route, err := r.GetRoute(tokenIn, tokenOut, amountIn)
	if err != nil {
		return "", err
	}

	// Execute via the DEX
	d, ok := r.dexs[route.Path[0]]
	if !ok {
		return "", fmt.Errorf("DEX not found: %s", route.Path[0])
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	txHash, err := d.ExecuteSwap(ctx, amountIn, amountOutMin, recipient)
	if err != nil {
		return "", err
	}

	return txHash, nil
}

// ExecuteSplitSwap executes a split swap across multiple DEXs
func (r *DEXRouter) ExecuteSplitSwap(tokenIn, tokenOut string, amountIn, amountOutMin *big.Int, recipient string, maxSplits int) (string, error) {
	routes, err := r.GetSplitRoute(tokenIn, tokenOut, amountIn, maxSplits)
	if err != nil {
		return "", err
	}

	var totalOut *big.Int
	for _, route := range routes {
		d, ok := r.dexs[route.Path[0]]
		if !ok {
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		txHash, err := d.ExecuteSwap(ctx, route.InputAmount, big.NewInt(0), recipient)
		cancel()

		if err != nil {
			continue
		}

		fmt.Printf("[Router] Executed split via %s: %s\n", route.Path[0], txHash)
	}

	return "", nil
}

// GetPoolInfo gets pool information from a DEX
func (r *DEXRouter) GetPoolInfo(dexName string) (*dex.PoolInfo, error) {
	d, ok := r.dexs[dexName]
	if !ok {
		return nil, fmt.Errorf("DEX not found: %s", dexName)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return d.GetPoolInfo(ctx)
}

// GetAllPools gets pool information from all DEXs
func (r *DEXRouter) GetAllPools() (map[string]*dex.PoolInfo, error) {
	result := make(map[string]*dex.PoolInfo)

	for name, d := range r.dexs {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		info, err := d.GetPoolInfo(ctx)
		if err != nil {
			continue
		}
		result[name] = info
	}

	return result, nil
}

// GetDEXs returns all available DEXs
func (r *DEXRouter) GetDEXs() []string {
	dexs := make([]string, 0, len(r.dexs))
	for name := range r.dexs {
		dexs = append(dexs, name)
	}
	return dexs
}

// IsEnabled checks if a DEX is enabled
func (r *DEXRouter) IsEnabled(dexName string) bool {
	_, ok := r.dexs[dexName]
	return ok
}

// dexConfig returns the configuration for a DEX
func (r *DEXRouter) dexConfig(dexName string) *config.DEXConfig {
	for i := range r.cfg.DEXs {
		if r.cfg.DEXs[i].Name == dexName {
			return &r.cfg.DEXs[i]
		}
	}
	return nil
}

// SmartRouter is a smart order router with pathfinding
type SmartRouter struct {
	router *DEXRouter
}

// NewSmartRouter creates a new smart router
func NewSmartRouter(cfg *config.Config) (*SmartRouter, error) {
	router, err := NewDEXRouter(cfg)
	if err != nil {
		return nil, err
	}

	return &SmartRouter{
		router: router,
	}, nil
}

// FindBestPath finds the best path for a swap
func (s *SmartRouter) FindBestPath(tokenIn, tokenOut string, amountIn *big.Int) (*RouteResult, error) {
	// Direct swap
	route, err := s.router.GetRoute(tokenIn, tokenOut, amountIn)
	if err == nil {
		return route, nil
	}

	// TODO: Implement multi-hop pathfinding
	// For now, return direct route
	return nil, fmt.Errorf("no route found")
}

// ExecuteSmartSwap executes a swap with optimal routing
func (s *SmartRouter) ExecuteSmartSwap(tokenIn, tokenOut string, amountIn, amountOutMin *big.Int, recipient string) (string, error) {
	route, err := s.FindBestPath(tokenIn, tokenOut, amountIn)
	if err != nil {
		return "", err
	}

	return s.router.ExecuteSwap(tokenIn, tokenOut, amountIn, amountOutMin, recipient)
}

// GetOptimalSplitAmounts calculates optimal split amounts
func (s *SmartRouter) GetOptimalSplitAmounts(tokenIn, tokenOut string, amountIn *big.Int, dexNames []string) ([]*big.Int, error) {
	quotes := make([]*big.Int, 0, len(dexNames))

	for _, name := range dexNames {
		// Get quote from each DEX
		d, ok := s.router.dexs[name]
		if !ok {
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		amountOut, err := d.GetQuote(ctx, amountIn)
		cancel()

		if err != nil {
			continue
		}

		quotes = append(quotes, amountOut)
	}

	if len(quotes) == 0 {
		return nil, fmt.Errorf("no quotes")
	}

	// Calculate optimal splits (proportional to quote amounts)
	totalOut := big.NewInt(0)
	for _, q := range quotes {
		totalOut = new(big.Int).Add(totalOut, q)
	}

	splits := make([]*big.Int, len(dexNames))
	for i, q := range quotes {
		split := new(big.Int).Mul(amountIn, q)
		split = new(big.Int).Div(split, totalOut)
		splits[i] = split
	}

	return splits, nil
}

// RouterABI is the common router ABI
var RouterABI = abi.ABI{}

func init() {
	// Load common router ABIs
	if jsonStr := `[{"name":"swapExactETHForTokens","inputs":[{"type":"uint256","name":"amountOutMin"},{"type":"address[]","name":"path"},{"type":"address","name":"to"},{"type":"uint256","name":"deadline"}],"outputs":[{"type":"uint256[]"}],"stateMutability":"payable","type":"function"}]`; jsonStr != "" {
		RouterABI, _ = abi.JSON([]byte(jsonStr))
	}
}