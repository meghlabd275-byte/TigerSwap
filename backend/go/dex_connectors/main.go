// Package dex_connectors provides TigerSwap's own AMM connectors.
//
// Every pool family here is a TigerSwap contract (V2 xyK, V3 concentrated,
// StableSwap, Weighted). The connector reads reserves directly from the
// TigerSwap factory/pool contracts over the configured RPC and reproduces the
// exact on-chain math off-chain for quoting. No third-party DEX is contacted.
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

// PoolFamily enumerates TigerSwap's own AMM pool families.
type PoolFamily string

const (
	FamilyV2       PoolFamily = "tigerswap_v2"       // constant product x*y=k
	FamilyV3       PoolFamily = "tigerswap_v3"       // concentrated liquidity
	FamilyStable   PoolFamily = "tigerswap_stable"   // StableSwap invariant
	FamilyWeighted PoolFamily = "tigerswap_weighted" // weighted pools
)

// Factory ABI — the TigerSwap factory exposes getPoolForFamily(tokenA, tokenB, family).
const tigerFactoryABI = `[
 {"inputs":[{"internalType":"address","name":"tokenA","type":"address"},{"internalType":"address","name":"tokenB","type":"address"},{"internalType":"uint8","name":"family","type":"uint8"}],"name":"getPoolForFamily","outputs":[{"internalType":"address","name":"pool","type":"address"}],"stateMutability":"view","type":"function"}
]`

// Pool ABI shared by the xyK, stable and weighted pools.
const tigerPoolABI = `[
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

// Quoter ABI — TigerSwap's own V3 quoter.
const tigerQuoterABI = `[
 {"inputs":[{"internalType":"address","name":"tokenIn","type":"address"},{"internalType":"address","name":"tokenOut","type":"address"},{"internalType":"uint24","name":"fee","type":"uint24"},{"internalType":"uint256","name":"amountIn","type":"uint256"}],"name":"quoteExactInputSingle","outputs":[{"internalType":"uint256","name":"amountOut","type":"uint256"}],"stateMutability":"nonpayable","type":"function"},
 {"inputs":[{"internalType":"bytes","name":"path","type":"bytes"},{"internalType":"uint256","name":"amountIn","type":"uint256"}],"name":"quoteExactInput","outputs":[{"internalType":"uint256","name":"amountOut","type":"uint256"}],"stateMutability":"nonpayable","type":"function"}
]`

// Router ABI — TigerSwap's own router used to execute swaps.
const tigerRouterABI = `[
 {"inputs":[{"internalType":"address","name":"tokenIn","type":"address"},{"internalType":"address","name":"tokenOut","type":"address"},{"internalType":"uint8","name":"family","type":"uint8"},{"internalType":"uint256","name":"amountIn","type":"uint256"},{"internalType":"uint256","name":"amountOutMin","type":"uint256"},{"internalType":"address","name":"to","type":"address"},{"internalType":"uint256","name":"deadline","type":"uint256"}],"name":"swapExactTokensForTokens","outputs":[{"internalType":"uint256","name":"amountOut","type":"uint256"}],"stateMutability":"nonpayable","type":"function"},
 {"inputs":[{"internalType":"address","name":"tokenA","type":"address"},{"internalType":"address","name":"tokenB","type":"address"},{"internalType":"uint8","name":"family","type":"uint8"},{"internalType":"uint256","name":"amountADesired","type":"uint256"},{"internalType":"uint256","name":"amountBDesired","type":"uint256"},{"internalType":"uint256","name":"amountAMin","type":"uint256"},{"internalType":"uint256","name":"amountBMin","type":"uint256"},{"internalType":"address","name":"to","type":"address"},{"internalType":"uint256","name":"deadline","type":"uint256"}],"name":"addLiquidity","outputs":[{"internalType":"uint256","name":"amountA","type":"uint256"},{"internalType":"uint256","name":"amountB","type":"uint256"},{"internalType":"uint256","name":"liquidity","type":"uint256"}],"stateMutability":"nonpayable","type":"function"},
 {"inputs":[{"internalType":"address","name":"tokenA","type":"address"},{"internalType":"address","name":"tokenB","type":"address"},{"internalType":"uint8","name":"family","type":"uint8"},{"internalType":"uint256","name":"liquidity","type":"uint256"},{"internalType":"uint256","name":"amountAMin","type":"uint256"},{"internalType":"uint256","name":"amountBMin","type":"uint256"},{"internalType":"address","name":"to","type":"address"},{"internalType":"uint256","name":"deadline","type":"uint256"}],"name":"removeLiquidity","outputs":[{"internalType":"uint256","name":"amountA","type":"uint256"},{"internalType":"uint256","name":"amountB","type":"uint256"}],"stateMutability":"nonpayable","type":"function"}
]`

var (
	factoryABI = mustABI(tigerFactoryABI)
	poolABI    = mustABI(tigerPoolABI)
	quoterABI  = mustABI(tigerQuoterABI)
	routerABI  = mustABI(tigerRouterABI)
)

func mustABI(s string) abi.ABI {
	parsed, err := abi.JSON(strings.NewReader(s))
	if err != nil {
		panic(fmt.Sprintf("invalid embedded ABI: %v", err))
	}
	return parsed
}

// DEXConnector is the TigerSwap AMM connector interface.
type DEXConnector interface {
	Family() PoolFamily
	ChainID() int64
	GetAmountOut(ctx context.Context, amountIn *big.Int, path []common.Address, fee uint32) (*big.Int, error)
	Swap(ctx context.Context, amountIn, amountOutMin *big.Int, path []common.Address, to common.Address, deadline *big.Int) (*types.Transaction, error)
	AddLiquidity(ctx context.Context, tokenA, tokenB common.Address, amountADesired, amountBDesired, amountAMin, amountBMin *big.Int, to common.Address, deadline *big.Int) (*types.Transaction, error)
	RemoveLiquidity(ctx context.Context, tokenA, tokenB common.Address, liquidity, amountAMin, amountBMin *big.Int, to common.Address, deadline *big.Int) (*types.Transaction, error)
}

// BaseConnector holds the shared TigerSwap pool plumbing.
type BaseConnector struct {
	family    PoolFamily
	chainID   int64
	client    *ethclient.Client
	factory   common.Address
	router    common.Address
	quoter    common.Address
	signer    *ecdsa.PrivateKey
	mu        sync.RWMutex
	poolCache map[string]common.Address
}

// NewBaseConnector dials the chain RPC and resolves the TigerSwap contract set.
func NewBaseConnector(family PoolFamily, chainID int64, rpcURL string, factory, router, quoter common.Address) (*BaseConnector, error) {
	if rpcURL == "" {
		return nil, fmt.Errorf("no RPC endpoint configured for chain %d", chainID)
	}
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, fmt.Errorf("dial rpc: %w", err)
	}

	key, err := connectorSigner()
	if err != nil {
		client.Close()
		return nil, err
	}

	return &BaseConnector{
		family:    family,
		chainID:   chainID,
		client:    client,
		factory:   factory,
		router:    router,
		quoter:    quoter,
		signer:    key,
		poolCache: make(map[string]common.Address),
	}, nil
}

// connectorSigner loads the key used to submit TigerSwap router transactions.
func connectorSigner() (*ecdsa.PrivateKey, error) {
	raw := os.Getenv("TIGERSWAP_SIGNER_KEY")
	if raw == "" {
		return nil, errors.New("TIGERSWAP_SIGNER_KEY is not configured")
	}
	return crypto.HexToECDSA(strings.TrimPrefix(raw, "0x"))
}

func (b *BaseConnector) Family() PoolFamily      { return b.family }
func (b *BaseConnector) ChainID() int64          { return b.chainID }
func (b *BaseConnector) Router() common.Address  { return b.router }
func (b *BaseConnector) Factory() common.Address { return b.factory }

// GetClient returns the underlying RPC client.
func (b *BaseConnector) GetClient() *ethclient.Client { return b.client }

// Close releases the RPC connection.
func (b *BaseConnector) Close() { b.client.Close() }

// familyCode maps the pool family to the uint8 used by the factory/router.
func familyCode(f PoolFamily) uint8 {
	switch f {
	case FamilyV2:
		return 0
	case FamilyV3:
		return 1
	case FamilyStable:
		return 2
	case FamilyWeighted:
		return 3
	}
	return 0
}

// getPool resolves the TigerSwap pool address for a token pair, with caching.
func (b *BaseConnector) getPool(ctx context.Context, tokenA, tokenB common.Address) (common.Address, error) {
	key := strings.ToLower(tokenA.Hex() + ":" + tokenB.Hex())
	b.mu.RLock()
	cached, ok := b.poolCache[key]
	b.mu.RUnlock()
	if ok {
		return cached, nil
	}

	data, err := factoryABI.Pack("getPoolForFamily", tokenA, tokenB, familyCode(b.family))
	if err != nil {
		return common.Address{}, err
	}
	out, err := b.client.CallContract(ctx, ethereum.CallMsg{To: &b.factory, Data: data}, nil)
	if err != nil {
		return common.Address{}, fmt.Errorf("factory.getPool: %w", err)
	}
	vals, err := factoryABI.Unpack("getPoolForFamily", out)
	if err != nil {
		return common.Address{}, fmt.Errorf("unpack getPool: %w", err)
	}
	pool, ok := vals[0].(common.Address)
	if !ok {
		return common.Address{}, errors.New("unexpected getPool return type")
	}
	if pool == (common.Address{}) {
		return common.Address{}, fmt.Errorf("no TigerSwap %s pool for %s/%s", b.family, tokenA.Hex(), tokenB.Hex())
	}

	b.mu.Lock()
	b.poolCache[key] = pool
	b.mu.Unlock()
	return pool, nil
}

// poolState captures the on-chain state needed for quoting.
type poolState struct {
	token0   common.Address
	token1   common.Address
	reserve0 *big.Int
	reserve1 *big.Int
	amp      *big.Int // StableSwap A (zero for other families)
	weight0  *big.Int // Weighted (zero for other families)
	weight1  *big.Int
}

// loadState reads reserves and family-specific parameters from the pool.
func (b *BaseConnector) loadState(ctx context.Context, pool common.Address) (*poolState, error) {
	st := &poolState{amp: big.NewInt(0), weight0: big.NewInt(0), weight1: big.NewInt(0)}

	switch b.family {
	case FamilyStable:
		// StableSwap pools expose coins(i), balances(i) and A.
		for i := 0; i < 2; i++ {
			coin, err := callAddress(ctx, b.client, pool, "coins", big.NewInt(int64(i)))
			if err != nil {
				return nil, err
			}
			bal, err := callUint(ctx, b.client, pool, "balances", big.NewInt(int64(i)))
			if err != nil {
				return nil, err
			}
			if i == 0 {
				st.token0, st.reserve0 = coin, bal
			} else {
				st.token1, st.reserve1 = coin, bal
			}
		}
		amp, err := callUint(ctx, b.client, pool, "A")
		if err != nil {
			return nil, err
		}
		st.amp = amp
	case FamilyWeighted:
		// Weighted pools expose tokens(i), getBalances() and getWeights().
		for i := 0; i < 2; i++ {
			token, err := callAddress(ctx, b.client, pool, "tokens", big.NewInt(int64(i)))
			if err != nil {
				return nil, err
			}
			if i == 0 {
				st.token0 = token
			} else {
				st.token1 = token
			}
		}
		balances, err := callUintArray(ctx, b.client, pool, "getBalances")
		if err != nil {
			return nil, err
		}
		weights, err := callUintArray(ctx, b.client, pool, "getWeights")
		if err != nil {
			return nil, err
		}
		if len(balances) < 2 || len(weights) < 2 {
			return nil, fmt.Errorf("weighted pool %s returned %d balances / %d weights", pool.Hex(), len(balances), len(weights))
		}
		st.reserve0, st.reserve1 = balances[0], balances[1]
		st.weight0, st.weight1 = weights[0], weights[1]
	default:
		// xyK pools expose getReserves(), token0() and token1().
		reservesOut, err := b.client.CallContract(ctx, ethereum.CallMsg{To: &pool, Data: mustPack(poolABI, "getReserves")}, nil)
		if err != nil {
			return nil, fmt.Errorf("pool.getReserves: %w", err)
		}
		res, err := poolABI.Unpack("getReserves", reservesOut)
		if err != nil || len(res) < 2 {
			return nil, fmt.Errorf("unpack getReserves: %w", err)
		}
		st.reserve0, _ = res[0].(*big.Int)
		st.reserve1, _ = res[1].(*big.Int)
		t0, err := callAddress(ctx, b.client, pool, "token0")
		if err != nil {
			return nil, err
		}
		st.token0 = t0
		t1, err := callAddress(ctx, b.client, pool, "token1")
		if err != nil {
			return nil, err
		}
		st.token1 = t1
	}

	if st.reserve0 == nil || st.reserve1 == nil || st.reserve0.Sign() == 0 || st.reserve1.Sign() == 0 {
		return nil, fmt.Errorf("pool %s has no usable liquidity", pool.Hex())
	}
	return st, nil
}

// callAddress reads a single address-returning view function.
func callAddress(ctx context.Context, client *ethclient.Client, pool common.Address, method string, args ...interface{}) (common.Address, error) {
	out, err := client.CallContract(ctx, ethereum.CallMsg{To: &pool, Data: mustPack(poolABI, method, args...)}, nil)
	if err != nil {
		return common.Address{}, fmt.Errorf("pool.%s: %w", method, err)
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

// callUint reads a single uint256-returning view function.
func callUint(ctx context.Context, client *ethclient.Client, pool common.Address, method string, args ...interface{}) (*big.Int, error) {
	out, err := client.CallContract(ctx, ethereum.CallMsg{To: &pool, Data: mustPack(poolABI, method, args...)}, nil)
	if err != nil {
		return nil, fmt.Errorf("pool.%s: %w", method, err)
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

// callUintArray reads a uint256[]-returning view function.
func callUintArray(ctx context.Context, client *ethclient.Client, pool common.Address, method string, args ...interface{}) ([]*big.Int, error) {
	out, err := client.CallContract(ctx, ethereum.CallMsg{To: &pool, Data: mustPack(poolABI, method, args...)}, nil)
	if err != nil {
		return nil, fmt.Errorf("pool.%s: %w", method, err)
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

// ---------------------------------------------------------------------------
// Connectors
// ---------------------------------------------------------------------------

// V2Connector quotes and trades TigerSwap's constant-product pools.
type V2Connector struct{ *BaseConnector }

// NewV2Connector builds a TigerSwap V2 connector.
func NewV2Connector(chainID int64, rpcURL, factory, router string) (*V2Connector, error) {
	base, err := NewBaseConnector(FamilyV2, chainID, rpcURL, common.HexToAddress(factory), common.HexToAddress(router), common.Address{})
	if err != nil {
		return nil, err
	}
	return &V2Connector{base}, nil
}

// GetAmountOut walks the path across TigerSwap V2 pools.
func (v *V2Connector) GetAmountOut(ctx context.Context, amountIn *big.Int, path []common.Address, fee uint32) (*big.Int, error) {
	if len(path) < 2 {
		return nil, errors.New("path must have at least 2 tokens")
	}
	current := new(big.Int).Set(amountIn)
	for i := 0; i < len(path)-1; i++ {
		pool, err := v.getPool(ctx, path[i], path[i+1])
		if err != nil {
			return nil, err
		}
		st, err := v.loadState(ctx, pool)
		if err != nil {
			return nil, err
		}
		reserveIn, reserveOut := st.reserve0, st.reserve1
		if st.token0 != path[i] {
			reserveIn, reserveOut = st.reserve1, st.reserve0
		}
		current, err = amm.AmountOutXY(current, reserveIn, reserveOut, fee)
		if err != nil {
			return nil, err
		}
	}
	return current, nil
}

// Swap executes a TigerSwap V2 swap through the router.
func (v *V2Connector) Swap(ctx context.Context, amountIn, amountOutMin *big.Int, path []common.Address, to common.Address, deadline *big.Int) (*types.Transaction, error) {
	if len(path) < 2 {
		return nil, errors.New("path must have at least 2 tokens")
	}
	data, err := routerABI.Pack("swapExactTokensForTokens",
		path[0], path[len(path)-1], familyCode(FamilyV2), amountIn, amountOutMin, to, deadline)
	if err != nil {
		return nil, err
	}
	return v.send(ctx, v.router, data)
}

// AddLiquidity adds liquidity to a TigerSwap V2 pool.
func (v *V2Connector) AddLiquidity(ctx context.Context, tokenA, tokenB common.Address, amountADesired, amountBDesired, amountAMin, amountBMin *big.Int, to common.Address, deadline *big.Int) (*types.Transaction, error) {
	data, err := routerABI.Pack("addLiquidity", tokenA, tokenB, familyCode(FamilyV2),
		amountADesired, amountBDesired, amountAMin, amountBMin, to, deadline)
	if err != nil {
		return nil, err
	}
	return v.send(ctx, v.router, data)
}

// RemoveLiquidity removes liquidity from a TigerSwap V2 pool.
func (v *V2Connector) RemoveLiquidity(ctx context.Context, tokenA, tokenB common.Address, liquidity, amountAMin, amountBMin *big.Int, to common.Address, deadline *big.Int) (*types.Transaction, error) {
	data, err := routerABI.Pack("removeLiquidity", tokenA, tokenB, familyCode(FamilyV2),
		liquidity, amountAMin, amountBMin, to, deadline)
	if err != nil {
		return nil, err
	}
	return v.send(ctx, v.router, data)
}

// V3Connector quotes TigerSwap's concentrated-liquidity pools via the quoter.
type V3Connector struct {
	*BaseConnector
}

// NewV3Connector builds a TigerSwap V3 connector.
func NewV3Connector(chainID int64, rpcURL, factory, router, quoter string) (*V3Connector, error) {
	base, err := NewBaseConnector(FamilyV3, chainID, rpcURL, common.HexToAddress(factory), common.HexToAddress(router), common.HexToAddress(quoter))
	if err != nil {
		return nil, err
	}
	return &V3Connector{base}, nil
}

// GetAmountOut calls TigerSwap's own V3 quoter.
func (v *V3Connector) GetAmountOut(ctx context.Context, amountIn *big.Int, path []common.Address, fee uint32) (*big.Int, error) {
	if len(path) < 2 {
		return nil, errors.New("path must have at least 2 tokens")
	}
	if v.quoter == (common.Address{}) {
		return nil, fmt.Errorf("TigerSwap V3 quoter not configured on chain %d", v.chainID)
	}

	var amountOut *big.Int
	for i := 0; i < len(path)-1; i++ {
		callData, err := quoterABI.Pack("quoteExactInputSingle", path[i], path[i+1], big.NewInt(int64(fee)), amountIn)
		if err != nil {
			return nil, err
		}
		out, err := v.client.CallContract(ctx, ethereum.CallMsg{To: &v.quoter, Data: callData}, nil)
		if err != nil {
			return nil, fmt.Errorf("tigerswap quoter: %w", err)
		}
		vals, err := quoterABI.Unpack("quoteExactInputSingle", out)
		if err != nil {
			return nil, fmt.Errorf("unpack quote: %w", err)
		}
		amountOut, _ = vals[0].(*big.Int)
		if amountOut == nil || amountOut.Sign() <= 0 {
			return nil, errors.New("tigerswap quoter returned no output")
		}
		amountIn = amountOut
	}
	return amountOut, nil
}

// Swap executes a TigerSwap V3 swap.
func (v *V3Connector) Swap(ctx context.Context, amountIn, amountOutMin *big.Int, path []common.Address, to common.Address, deadline *big.Int) (*types.Transaction, error) {
	if len(path) < 2 {
		return nil, errors.New("path must have at least 2 tokens")
	}
	data, err := routerABI.Pack("swapExactTokensForTokens",
		path[0], path[len(path)-1], familyCode(FamilyV3), amountIn, amountOutMin, to, deadline)
	if err != nil {
		return nil, err
	}
	return v.send(ctx, v.router, data)
}

// AddLiquidity adds liquidity to a TigerSwap V3 pool.
func (v *V3Connector) AddLiquidity(ctx context.Context, tokenA, tokenB common.Address, amountADesired, amountBDesired, amountAMin, amountBMin *big.Int, to common.Address, deadline *big.Int) (*types.Transaction, error) {
	data, err := routerABI.Pack("addLiquidity", tokenA, tokenB, familyCode(FamilyV3),
		amountADesired, amountBDesired, amountAMin, amountBMin, to, deadline)
	if err != nil {
		return nil, err
	}
	return v.send(ctx, v.router, data)
}

// RemoveLiquidity removes liquidity from a TigerSwap V3 pool.
func (v *V3Connector) RemoveLiquidity(ctx context.Context, tokenA, tokenB common.Address, liquidity, amountAMin, amountBMin *big.Int, to common.Address, deadline *big.Int) (*types.Transaction, error) {
	data, err := routerABI.Pack("removeLiquidity", tokenA, tokenB, familyCode(FamilyV3),
		liquidity, amountAMin, amountBMin, to, deadline)
	if err != nil {
		return nil, err
	}
	return v.send(ctx, v.router, data)
}

// StableConnector quotes TigerSwap's StableSwap pools.
type StableConnector struct{ *BaseConnector }

// NewStableConnector builds a TigerSwap StableSwap connector.
func NewStableConnector(chainID int64, rpcURL, factory, router string) (*StableConnector, error) {
	base, err := NewBaseConnector(FamilyStable, chainID, rpcURL, common.HexToAddress(factory), common.HexToAddress(router), common.Address{})
	if err != nil {
		return nil, err
	}
	return &StableConnector{base}, nil
}

// GetAmountOut walks the path across TigerSwap StableSwap pools.
func (s *StableConnector) GetAmountOut(ctx context.Context, amountIn *big.Int, path []common.Address, fee uint32) (*big.Int, error) {
	if len(path) < 2 {
		return nil, errors.New("path must have at least 2 tokens")
	}
	current := new(big.Int).Set(amountIn)
	for i := 0; i < len(path)-1; i++ {
		pool, err := s.getPool(ctx, path[i], path[i+1])
		if err != nil {
			return nil, err
		}
		st, err := s.loadState(ctx, pool)
		if err != nil {
			return nil, err
		}
		reserveIn, reserveOut := st.reserve0, st.reserve1
		if st.token0 != path[i] {
			reserveIn, reserveOut = st.reserve1, st.reserve0
		}
		current, err = amm.AmountOutStable(current, reserveIn, reserveOut, st.amp, fee)
		if err != nil {
			return nil, err
		}
	}
	return current, nil
}

// Swap executes a TigerSwap StableSwap swap.
func (s *StableConnector) Swap(ctx context.Context, amountIn, amountOutMin *big.Int, path []common.Address, to common.Address, deadline *big.Int) (*types.Transaction, error) {
	if len(path) < 2 {
		return nil, errors.New("path must have at least 2 tokens")
	}
	data, err := routerABI.Pack("swapExactTokensForTokens",
		path[0], path[len(path)-1], familyCode(FamilyStable), amountIn, amountOutMin, to, deadline)
	if err != nil {
		return nil, err
	}
	return s.send(ctx, s.router, data)
}

// AddLiquidity adds liquidity to a TigerSwap StableSwap pool.
func (s *StableConnector) AddLiquidity(ctx context.Context, tokenA, tokenB common.Address, amountADesired, amountBDesired, amountAMin, amountBMin *big.Int, to common.Address, deadline *big.Int) (*types.Transaction, error) {
	data, err := routerABI.Pack("addLiquidity", tokenA, tokenB, familyCode(FamilyStable),
		amountADesired, amountBDesired, amountAMin, amountBMin, to, deadline)
	if err != nil {
		return nil, err
	}
	return s.send(ctx, s.router, data)
}

// RemoveLiquidity removes liquidity from a TigerSwap StableSwap pool.
func (s *StableConnector) RemoveLiquidity(ctx context.Context, tokenA, tokenB common.Address, liquidity, amountAMin, amountBMin *big.Int, to common.Address, deadline *big.Int) (*types.Transaction, error) {
	data, err := routerABI.Pack("removeLiquidity", tokenA, tokenB, familyCode(FamilyStable),
		liquidity, amountAMin, amountBMin, to, deadline)
	if err != nil {
		return nil, err
	}
	return s.send(ctx, s.router, data)
}

// WeightedConnector quotes TigerSwap's weighted pools.
type WeightedConnector struct{ *BaseConnector }

// NewWeightedConnector builds a TigerSwap weighted-pool connector.
func NewWeightedConnector(chainID int64, rpcURL, factory, router string) (*WeightedConnector, error) {
	base, err := NewBaseConnector(FamilyWeighted, chainID, rpcURL, common.HexToAddress(factory), common.HexToAddress(router), common.Address{})
	if err != nil {
		return nil, err
	}
	return &WeightedConnector{base}, nil
}

// GetAmountOut walks the path across TigerSwap weighted pools.
func (w *WeightedConnector) GetAmountOut(ctx context.Context, amountIn *big.Int, path []common.Address, fee uint32) (*big.Int, error) {
	if len(path) < 2 {
		return nil, errors.New("path must have at least 2 tokens")
	}
	current := new(big.Int).Set(amountIn)
	for i := 0; i < len(path)-1; i++ {
		pool, err := w.getPool(ctx, path[i], path[i+1])
		if err != nil {
			return nil, err
		}
		st, err := w.loadState(ctx, pool)
		if err != nil {
			return nil, err
		}
		reserveIn, reserveOut := st.reserve0, st.reserve1
		weightIn, weightOut := st.weight0, st.weight1
		if st.token0 != path[i] {
			reserveIn, reserveOut = st.reserve1, st.reserve0
			weightIn, weightOut = st.weight1, st.weight0
		}
		current, err = amm.AmountOutWeighted(current, reserveIn, reserveOut, weightIn, weightOut, fee)
		if err != nil {
			return nil, err
		}
	}
	return current, nil
}

// Swap executes a TigerSwap weighted-pool swap.
func (w *WeightedConnector) Swap(ctx context.Context, amountIn, amountOutMin *big.Int, path []common.Address, to common.Address, deadline *big.Int) (*types.Transaction, error) {
	if len(path) < 2 {
		return nil, errors.New("path must have at least 2 tokens")
	}
	data, err := routerABI.Pack("swapExactTokensForTokens",
		path[0], path[len(path)-1], familyCode(FamilyWeighted), amountIn, amountOutMin, to, deadline)
	if err != nil {
		return nil, err
	}
	return w.send(ctx, w.router, data)
}

// AddLiquidity adds liquidity to a TigerSwap weighted pool.
func (w *WeightedConnector) AddLiquidity(ctx context.Context, tokenA, tokenB common.Address, amountADesired, amountBDesired, amountAMin, amountBMin *big.Int, to common.Address, deadline *big.Int) (*types.Transaction, error) {
	data, err := routerABI.Pack("addLiquidity", tokenA, tokenB, familyCode(FamilyWeighted),
		amountADesired, amountBDesired, amountAMin, amountBMin, to, deadline)
	if err != nil {
		return nil, err
	}
	return w.send(ctx, w.router, data)
}

// RemoveLiquidity removes liquidity from a TigerSwap weighted pool.
func (w *WeightedConnector) RemoveLiquidity(ctx context.Context, tokenA, tokenB common.Address, liquidity, amountAMin, amountBMin *big.Int, to common.Address, deadline *big.Int) (*types.Transaction, error) {
	data, err := routerABI.Pack("removeLiquidity", tokenA, tokenB, familyCode(FamilyWeighted),
		liquidity, amountAMin, amountBMin, to, deadline)
	if err != nil {
		return nil, err
	}
	return w.send(ctx, w.router, data)
}

// send signs and broadcasts a router transaction with proper nonce/gas handling.
func (b *BaseConnector) send(ctx context.Context, to common.Address, data []byte) (*types.Transaction, error) {
	from := crypto.PubkeyToAddress(b.signer.PublicKey)

	nonce, err := b.client.PendingNonceAt(ctx, from)
	if err != nil {
		return nil, fmt.Errorf("pending nonce: %w", err)
	}
	gasPrice, err := b.client.SuggestGasPrice(ctx)
	if err != nil {
		return nil, fmt.Errorf("suggest gas price: %w", err)
	}
	gasLimit, err := b.client.EstimateGas(ctx, ethereum.CallMsg{
		From: from, To: &to, Data: data, Value: big.NewInt(0),
	})
	if err != nil {
		return nil, fmt.Errorf("estimate gas: %w", err)
	}

	chainID := big.NewInt(b.chainID)
	tx := types.NewTransaction(nonce, to, big.NewInt(0), gasLimit, gasPrice, data)
	signed, err := types.SignTx(tx, types.LatestSignerForChainID(chainID), b.signer)
	if err != nil {
		return nil, fmt.Errorf("sign tx: %w", err)
	}
	if err := b.client.SendTransaction(ctx, signed); err != nil {
		return nil, fmt.Errorf("send tx: %w", err)
	}
	return signed, nil
}

// ---------------------------------------------------------------------------
// Registry
// ---------------------------------------------------------------------------

// ConnectorRegistry holds the TigerSwap connectors keyed by chain and family.
type ConnectorRegistry struct {
	connectors map[int64]map[PoolFamily]DEXConnector
	mu         sync.RWMutex
}

// NewConnectorRegistry creates an empty registry.
func NewConnectorRegistry() *ConnectorRegistry {
	return &ConnectorRegistry{connectors: make(map[int64]map[PoolFamily]DEXConnector)}
}

// Register adds a connector.
func (r *ConnectorRegistry) Register(chainID int64, c DEXConnector) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.connectors[chainID] == nil {
		r.connectors[chainID] = make(map[PoolFamily]DEXConnector)
	}
	r.connectors[chainID][c.Family()] = c
}

// Get returns a connector by chain and family.
func (r *ConnectorRegistry) Get(chainID int64, family PoolFamily) (DEXConnector, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.connectors[chainID] == nil {
		return nil, false
	}
	c, ok := r.connectors[chainID][family]
	return c, ok
}

// GetAll returns every connector for a chain.
func (r *ConnectorRegistry) GetAll(chainID int64) []DEXConnector {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]DEXConnector, 0)
	for _, c := range r.connectors[chainID] {
		out = append(out, c)
	}
	return out
}

// Close releases every connector's RPC client.
func (r *ConnectorRegistry) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, byFamily := range r.connectors {
		for _, c := range byFamily {
			if bc, ok := c.(interface{ Close() }); ok {
				bc.Close()
			}
		}
	}
}

// tigerContractsForChain resolves the TigerSwap contract set for a chain.
func tigerContractsForChain(chainID int64) (factory, router, quoter, rpc string) {
	suffix := strconv.FormatInt(chainID, 10)
	factory = os.Getenv("TIGERSWAP_FACTORY_" + suffix)
	router = os.Getenv("TIGERSWAP_ROUTER_" + suffix)
	quoter = os.Getenv("TIGERSWAP_QUOTER_" + suffix)
	rpc = os.Getenv("TIGERSWAP_RPC_" + suffix)
	return
}

// InitializeConnectors registers a connector for every configured chain.
func InitializeConnectors() *ConnectorRegistry {
	registry := NewConnectorRegistry()

	for _, chainID := range configuredChains() {
		factory, router, quoter, rpc := tigerContractsForChain(chainID)
		if factory == "" || router == "" || rpc == "" {
			log.Printf("chain %d: TigerSwap contracts not fully configured; skipping", chainID)
			continue
		}

		if c, err := NewV2Connector(chainID, rpc, factory, router); err != nil {
			log.Printf("chain %d: v2 connector unavailable: %v", chainID, err)
		} else {
			registry.Register(chainID, c)
		}
		if c, err := NewStableConnector(chainID, rpc, factory, router); err != nil {
			log.Printf("chain %d: stable connector unavailable: %v", chainID, err)
		} else {
			registry.Register(chainID, c)
		}
		if c, err := NewWeightedConnector(chainID, rpc, factory, router); err != nil {
			log.Printf("chain %d: weighted connector unavailable: %v", chainID, err)
		} else {
			registry.Register(chainID, c)
		}
		if quoter != "" {
			if c, err := NewV3Connector(chainID, rpc, factory, router, quoter); err != nil {
				log.Printf("chain %d: v3 connector unavailable: %v", chainID, err)
			} else {
				registry.Register(chainID, c)
			}
		}
	}

	return registry
}

// configuredChains returns the chain IDs with a TigerSwap deployment.
func configuredChains() []int64 {
	raw := os.Getenv("TIGERSWAP_CHAINS")
	if raw == "" {
		return nil
	}
	var chains []int64
	for _, part := range strings.Split(raw, ",") {
		id, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64)
		if err != nil {
			log.Printf("ignoring invalid chain id %q", part)
			continue
		}
		chains = append(chains, id)
	}
	return chains
}

// ---------------------------------------------------------------------------
// HTTP
// ---------------------------------------------------------------------------

// Handler exposes the quote API.
type Handler struct {
	registry *ConnectorRegistry
}

// NewHandler builds an HTTP handler over the registry.
func NewHandler(reg *ConnectorRegistry) *Handler {
	return &Handler{registry: reg}
}

// GetQuote returns a quote from one TigerSwap pool family.
func (h *Handler) GetQuote(c *gin.Context) {
	chainID, err := strconv.ParseInt(c.Query("chainId"), 10, 64)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid chainId"})
		return
	}
	family := PoolFamily(c.Query("family"))
	if family == "" {
		c.JSON(400, gin.H{"error": "family is required"})
		return
	}
	feeBps, err := strconv.ParseUint(c.DefaultQuery("fee", "30"), 10, 32)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid fee"})
		return
	}

	conn, ok := h.registry.Get(chainID, family)
	if !ok {
		c.JSON(404, gin.H{"error": "pool family not available on this chain"})
		return
	}

	amountIn, ok := new(big.Int).SetString(c.Query("amountIn"), 10)
	if !ok || amountIn.Sign() <= 0 {
		c.JSON(400, gin.H{"error": "invalid amountIn"})
		return
	}

	rawPath := strings.Split(c.Query("path"), ",")
	path := make([]common.Address, 0, len(rawPath))
	for _, token := range rawPath {
		if !common.IsHexAddress(token) {
			c.JSON(400, gin.H{"error": fmt.Sprintf("invalid token address %q", token)})
			return
		}
		path = append(path, common.HexToAddress(token))
	}
	if len(path) < 2 {
		c.JSON(400, gin.H{"error": "path must have at least 2 tokens"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	amountOut, err := conn.GetAmountOut(ctx, amountIn, path, uint32(feeBps))
	if err != nil {
		c.JSON(502, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"family":    family,
		"chainId":   chainID,
		"amountIn":  amountIn.String(),
		"amountOut": amountOut.String(),
	})
}

func main() {
	registry := InitializeConnectors()
	defer registry.Close()

	handler := NewHandler(registry)

	r := gin.Default()
	r.GET("/api/v1/quote", handler.GetQuote)
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "tigerswap-amm-connectors"})
	})

	srv := &http.Server{Addr: ":8082", Handler: r}
	go func() {
		log.Println("Starting TigerSwap AMM connectors on :8082")
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
	log.Println("Shutting down TigerSwap AMM connectors...")
}
