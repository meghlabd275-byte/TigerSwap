module github.com/tigerswap/dex-aggregator

go 1.21

require (
	github.com/ethereum/go-ethereum v1.12.0
	github.com/gorilla/mux v1.8.1
	github.com/gorilla/websocket v1.5.1
	github.com/shopspring/decimal v1.3.1
)

require (
	github.com/btcsuite/btcd v0.23.4 // indirect
	github.com/decred/dcrd/dcrec/secp256k1/v4 v4.2.0 // indirect
	github.com/go-stack/stack v1.8.1 // indirect
	github.com/google/uuid v1.4.0 // indirect
	golang.org/x/crypto v0.17.0 // indirect
	golang.org/x/net v0.19.0 // indirect
	golang.org/x/sys v0.15.0 // indirect
	golang.org/x/text v0.14.0 // indirect
)

module dex_aggregator

go 1.21

// DEX Aggregator - Smart routing across multiple DEXs
package dexaggregator

import (
	"context"
	"math/big"
	"sync"
	"time"
)

type Token struct {
	Address   string
	Symbol    string
	Decimals  int
	ChainID   int
	PriceUSD  float64
}

type Pool struct {
	Address        string
	TokenA         Token
	TokenB         Token
	ReserveA       *big.Int
	ReserveB       *big.Int
	Fee            int // basis points
	Liquidity      *big.Float
	Protocol       string
	ChainID        int
	LastUpdateTime time.Time
}

type Route struct {
	Pools     []Pool
	Path      []Token
	Percent   int
	ExpectedOutput *big.Float
	PriceImpact   float64
	GasEstimate    uint64
}

type QuoteRequest struct {
	TokenIn    Token
	TokenOut   Token
	AmountIn   *big.Int
	MaxHops    int
	MaxResults int
	Slippage   float64
}

type QuoteResult struct {
	InputAmount  *big.Int
	OutputAmount *big.Int
	Routes      []Route
	GasEstimate uint64
	PriceImpact float64
	ExecutionPrice float64
	MinimumReceived *big.Int
}

type SwapRequest struct {
	TokenIn    Token
	TokenOut   Token
	AmountIn   *big.Int
	AmountOutMin *big.Int
	Recipient  string
	Deadline   time.Time
	Routes     []Route
	Referrer   string
}

type SwapResult struct {
	Success     bool
	TxHash      string
	TokenIn     Token
	TokenOut    Token
	AmountIn    *big.Int
	AmountOut   *big.Int
	GasUsed     uint64
	EffectivePrice float64
}

type DEX interface {
	Name() string
	ChainID() int
	GetPools(tokenA, tokenB string) ([]Pool, error)
	GetQuote(tokenIn, tokenOut string, amountIn *big.Int) (*QuoteResult, error)
	ExecuteSwap(req SwapRequest) (*SwapResult, error)
}

type RoutingEngine struct {
	dexes         map[string]DEX
	pools        map[int]map[string][]Pool
	priceCache   map[string]float64
	mu           sync.RWMutex
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
}

func NewRoutingEngine() *RoutingEngine {
	ctx, cancel := context.WithCancel(context.Background())
	engine := &RoutingEngine{
		dexes:       make(map[string]DEX),
		pools:       make(map[int]map[string][]Pool),
		priceCache:  make(map[string]float64),
		ctx:         ctx,
		cancel:      cancel,
	}
	return engine
}

func (r *RoutingEngine) RegisterDEX(name string, dex DEX) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.dexes[name] = dex
}

func (r *RoutingEngine) FindBestRoute(req QuoteRequest) ([]Route, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var allRoutes []Route
	tokenPairs := make(map[string]bool)

	// Build pair key
	for _, dex := range r.dexes {
		pools, err := dex.GetPools(req.TokenIn.Address, req.TokenOut.Address)
		if err != nil {
			continue
		}

		for _, pool := range pools {
			route := Route{
				Pools:   []Pool{pool},
				Path:    []Token{req.TokenIn, req.TokenOut},
				Percent: 100,
			}
			route.ExpectedOutput = r.calculateExpectedOutput(route, req.AmountIn)
			route.PriceImpact = r.calculatePriceImpact(pool, req.AmountIn)
			route.GasEstimate = r.estimateGas(route)
			allRoutes = append(allRoutes, route)
		}

		// Multi-hop
		for intermediateToken := range tokenPairs {
			multiHopPools, _ := dex.GetPools(req.TokenIn.Address, intermediateToken)
			for _, pool1 := range multiHopPools {
				finalPools, _ := dex.GetPools(intermediateToken, req.TokenOut.Address)
				for _, pool2 := range finalPools {
					route := Route{
						Pools:   []Pool{pool1, pool2},
						Path:    []Token{req.TokenIn, {Address: intermediateToken}, req.TokenOut},
						Percent: 100,
					}
					route.ExpectedOutput = r.calculateExpectedOutput(route, req.AmountIn)
					route.PriceImpact = r.calculateMultiHopPriceImpact(pool1, pool2, req.AmountIn)
					route.GasEstimate = r.estimateGas(route)
					allRoutes = append(allRoutes, route)
				}
			}
		}
	}

	return r.sortRoutesByOutput(allRoutes), nil
}

func (r *RoutingEngine) GetQuote(req QuoteRequest) (*QuoteResult, error) {
	routes, err := r.FindBestRoute(req)
	if err != nil {
		return nil, err
	}

	var bestRoute Route
	if len(routes) > 0 {
		bestRoute = routes[0]
	}

	result := &QuoteResult{
		InputAmount:       req.AmountIn,
		OutputAmount:      r.amountToBigInt(bestRoute.ExpectedOutput),
		Routes:            routes,
		GasEstimate:      bestRoute.GasEstimate,
		PriceImpact:      bestRoute.PriceImpact,
		ExecutionPrice:   r.calculateExecutionPrice(bestRoute),
		MinimumReceived:  r.applySlippage(req.AmountIn, req.Slippage),
	}

	return result, nil
}

func (r *RoutingEngine) ExecuteSwap(req SwapRequest) ([]*SwapResult, error) {
	var results []*SwapResult

	for _, route := range req.Routes {
		for _, dex := range r.dexes {
			if dex.Name() == route.Pools[0].Protocol {
				swapReq := SwapRequest{
					TokenIn:      req.TokenIn,
					TokenOut:     req.TokenOut,
					AmountIn:     req.AmountIn,
					AmountOutMin: req.AmountOutMin,
					Recipient:    req.Recipient,
					Deadline:     req.Deadline,
					Routes:       []Route{route},
					Referrer:     req.Referrer,
				}
				result, err := dex.ExecuteSwap(swapReq)
				if err == nil {
					results = append(results, result)
				}
			}
		}
	}

	return results, nil
}

func (r *RoutingEngine) calculateExpectedOutput(route Route, amountIn *big.Int) *big.Float {
	if len(route.Pools) == 0 {
		return big.NewFloat(0)
	}

	reserveIn := new(big.Float).SetInt(route.Pools[0].ReserveA)
	reserveOut := new(big.Float).SetInt(route.Pools[0].ReserveB)
	feeMultiplier := big.NewFloat(0.997) // 0.3% fee

	amountInFloat := new(big.Float).SetInt(amountIn)
	amountInWithFee := new(big.Float).Mul(amountInFloat, feeMultiplier)

	numerator := new(big.Float).Mul(amountInWithFee, reserveOut)
	denominator := new(big.Float).Add(new(big.Float).Mul(reserveIn, big.NewFloat(1000)), amountInWithFee)

	output, _ := numerator.Quo(numerator, denominator).Float64()
	return big.NewFloat(output)
}

func (r *RoutingEngine) calculatePriceImpact(pool Pool, amountIn *big.Int) float64 {
	priceBefore := r.getSpotPrice(pool)
	
	amountInFloat := new(big.Float).SetInt(amountIn)
	amountInWithFee := new(big.Float).Mul(amountInFloat, big.NewFloat(0.997))
	
	reserveInFloat := new(big.Float).SetInt(pool.ReserveA)
	reserveOutFloat := new(big.Float).SetInt(pool.ReserveB)
	
	numerator := new(big.Float).Mul(amountInWithFee, reserveOutFloat)
	denominator := new(big.Float).Add(new(big.Float).Mul(reserveInFloat, big.NewFloat(1000)), amountInWithFee)
	amountOutFloat, _ := numerator.Quo(numerator, denominator).Float64()
	
	priceAfter := amountInFloat.Quo(big.NewFloat(amountOutFloat), amountInFloat).Text('f', 6)
	_ = priceAfter // Use actual price comparison
	
	return 0.5 // Mock price impact
}

func (r *RoutingEngine) calculateMultiHopPriceImpact(pool1, pool2 Pool, amountIn *big.Int) float64 {
	// Calculate combined price impact for multi-hop routes
	impact1 := r.calculatePriceImpact(pool1, amountIn)
	amountOut1 := r.calculateExpectedOutput(Route{Pools: []Pool{pool1}}, amountIn)
	amountOutInt, _ := amountOut1.Int(nil)
	impact2 := r.calculatePriceImpact(pool2, amountOutInt)
	
	return impact1 + impact2
}

func (r *RoutingEngine) estimateGas(route Route) uint64 {
	baseGas := uint64(100000)
	perHopGas := uint64(50000)
	
	return baseGas + uint64(len(route.Pools))*perHopGas
}

func (r *RoutingEngine) calculateExecutionPrice(route Route) float64 {
	if len(route.Pools) == 0 || route.ExpectedOutput == nil {
		return 0
	}
	output, _ := route.ExpectedOutput.Float64()
	return output
}

func (r *RoutingEngine) applySlippage(amount *big.Int, slippage float64) *big.Int {
	if slippage <= 0 {
		return amount
	}
	slippageFactor := big.NewFloat(1 - slippage/100)
	amountFloat := new(big.Float).SetInt(amount)
	result, _ := amountFloat.Mul(amountFloat, slippageFactor).Int(nil)
	return result
}

func (r *RoutingEngine) sortRoutesByOutput(routes []Route) []Route {
	for i := 0; i < len(routes)-1; i++ {
		for j := i + 1; j < len(routes); j++ {
			if routes[i].ExpectedOutput.Cmp(routes[j].ExpectedOutput) < 0 {
				routes[i], routes[j] = routes[j], routes[i]
			}
		}
	}
	return routes
}

func (r *RoutingEngine) getSpotPrice(pool Pool) float64 {
	reserveA := new(big.Float).SetInt(pool.ReserveA)
	reserveB := new(big.Float).SetInt(pool.ReserveB)
	price, _ := reserveA.Quo(reserveA, reserveB).Float64()
	return price
}

func (r *RoutingEngine) amountToBigInt(amount *big.Float) *big.Int {
	result, _ := amount.Int(nil)
	return result
}

func (r *RoutingEngine) Start() {
	r.wg.Add(1)
	go r.cacheUpdater()
}

func (r *RoutingEngine) Stop() {
	r.cancel()
	r.wg.Wait()
}

func (r *RoutingEngine) cacheUpdater() {
	defer r.wg.Done()
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.ctx.Done():
			return
		case <-ticker.C:
			// Update price cache
			for pair, pools := range r.pools {
				if len(pools) > 0 {
					r.mu.Lock()
					r.priceCache[pair] = r.getSpotPrice(pools[0])
					r.mu.Unlock()
				}
			}
		}
	}
}