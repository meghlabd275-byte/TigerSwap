// Package tigerswap contains TigerSwap's own on-chain client: pool resolution,
// reserve loading, quoting across the V2/V3/Stable/Weighted families and swap
// execution against the TigerSwap router. It never calls a third-party DEX.
package tigerswap

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"math/big"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"

	"tigerswap/backend/go/amm"
)

const factoryABIJSON = `[
 {"inputs":[{"internalType":"address","name":"tokenA","type":"address"},{"internalType":"address","name":"tokenB","type":"address"},{"internalType":"uint8","name":"family","type":"uint8"}],"name":"getPoolForFamily","outputs":[{"internalType":"address","name":"pool","type":"address"}],"stateMutability":"view","type":"function"}
]`

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

const routerABIJSON = `[
 {"inputs":[{"internalType":"address","name":"tokenIn","type":"address"},{"internalType":"address","name":"tokenOut","type":"address"},{"internalType":"uint8","name":"family","type":"uint8"},{"internalType":"uint256","name":"amountIn","type":"uint256"},{"internalType":"uint256","name":"amountOutMin","type":"uint256"},{"internalType":"address","name":"to","type":"address"},{"internalType":"uint256","name":"deadline","type":"uint256"}],"name":"swapExactTokensForTokens","outputs":[{"internalType":"uint256","name":"amountOut","type":"uint256"}],"stateMutability":"nonpayable","type":"function"}
]`

// oracleABIJSON is TigerSwap's own median-of-N price oracle. Prices are
// WAD-scaled (1e18 = 1 unit of the quote currency).
const oracleABIJSON = `[
 {"inputs":[{"internalType":"address","name":"asset","type":"address"}],"name":"getPrice","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},
 {"inputs":[{"internalType":"address","name":"asset","type":"address"}],"name":"liveReportCount","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"}
]`

// quoterABIJSON calls TigerSwap's own concentrated-liquidity quoter.
const quoterABIJSON = `[
 {"inputs":[{"internalType":"address","name":"tokenIn","type":"address"},{"internalType":"address","name":"tokenOut","type":"address"},{"internalType":"uint24","name":"fee","type":"uint24"},{"internalType":"uint256","name":"amountIn","type":"uint256"}],"name":"quoteExactInputSingle","outputs":[{"internalType":"uint256","name":"amountOut","type":"uint256"}],"stateMutability":"view","type":"function"}
]`

var (
	factoryABI = mustABI(factoryABIJSON)
	poolABI    = mustABI(poolABIJSON)
	routerABI  = mustABI(routerABIJSON)
	oracleABI  = mustABI(oracleABIJSON)
	quoterABI  = mustABI(quoterABIJSON)
)

func mustABI(s string) abi.ABI {
	parsed, err := abi.JSON(strings.NewReader(s))
	if err != nil {
		panic(fmt.Sprintf("invalid embedded ABI: %v", err))
	}
	return parsed
}

// FamilyCode maps a pool family to the on-chain uint8 selector.
func FamilyCode(f amm.Family) uint8 {
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

// Deployment describes the TigerSwap contracts for one chain.
type Deployment struct {
	ChainID int64
	Factory common.Address
	Router  common.Address
	Quoter  common.Address
	Oracle  common.Address
	RPCURL  string
}

// Client is a TigerSwap on-chain client for a single chain.
type Client struct {
	deployment Deployment
	client     *ethclient.Client
	signer     *ecdsa.PrivateKey

	mu    sync.RWMutex
	pools map[string]common.Address
}

// NewClient dials the configured RPC and prepares the TigerSwap client.
func NewClient(d Deployment, signer *ecdsa.PrivateKey) (*Client, error) {
	if d.RPCURL == "" {
		return nil, fmt.Errorf("no RPC URL configured for chain %d", d.ChainID)
	}
	if d.Factory == (common.Address{}) || d.Router == (common.Address{}) {
		return nil, fmt.Errorf("TigerSwap factory/router not configured for chain %d", d.ChainID)
	}
	client, err := ethclient.Dial(d.RPCURL)
	if err != nil {
		return nil, fmt.Errorf("dial rpc for chain %d: %w", d.ChainID, err)
	}
	return &Client{
		deployment: d,
		client:     client,
		signer:     signer,
		pools:      make(map[string]common.Address),
	}, nil
}

// ChainID returns the deployment chain id.
func (c *Client) ChainID() int64 { return c.deployment.ChainID }

// Close releases the RPC connection.
func (c *Client) Close() { c.client.Close() }

// RPC exposes the underlying client for balance and gas queries.
func (c *Client) RPC() *ethclient.Client { return c.client }

// poolState captures on-chain pool state for quoting.
type poolState struct {
	token0   common.Address
	reserve0 *big.Int
	reserve1 *big.Int
	amp      *big.Int
	weight0  *big.Int
	weight1  *big.Int
}

// ResolvePool returns the TigerSwap pool for a pair and family, caching results.
func (c *Client) ResolvePool(ctx context.Context, tokenA, tokenB common.Address, family amm.Family) (common.Address, error) {
	key := fmt.Sprintf("%s:%s:%d", strings.ToLower(tokenA.Hex()), strings.ToLower(tokenB.Hex()), FamilyCode(family))
	c.mu.RLock()
	cached, ok := c.pools[key]
	c.mu.RUnlock()
	if ok {
		return cached, nil
	}

	data, err := factoryABI.Pack("getPoolForFamily", tokenA, tokenB, FamilyCode(family))
	if err != nil {
		return common.Address{}, err
	}
	out, err := c.client.CallContract(ctx, ethereum.CallMsg{To: &c.deployment.Factory, Data: data}, nil)
	if err != nil {
		return common.Address{}, fmt.Errorf("factory.getPool: %w", err)
	}
	vals, err := factoryABI.Unpack("getPoolForFamily", out)
	if err != nil {
		return common.Address{}, fmt.Errorf("unpack getPool: %w", err)
	}
	pool, _ := vals[0].(common.Address)
	if pool == (common.Address{}) {
		return common.Address{}, fmt.Errorf("no TigerSwap %s pool for %s/%s", family, tokenA.Hex(), tokenB.Hex())
	}

	c.mu.Lock()
	c.pools[key] = pool
	c.mu.Unlock()
	return pool, nil
}

// loadState reads reserves and family parameters for a pool.
func (c *Client) loadState(ctx context.Context, pool common.Address, family amm.Family) (*poolState, error) {
	st := &poolState{amp: big.NewInt(0), weight0: big.NewInt(0), weight1: big.NewInt(0)}

	switch family {
	case amm.FamilyStable:
		// StableSwap pools expose coins(i), balances(i) and A.
		for i := 0; i < 2; i++ {
			coin, err := c.callAddress(ctx, pool, "coins", big.NewInt(int64(i)))
			if err != nil {
				return nil, err
			}
			bal, err := c.callUint(ctx, pool, "balances", big.NewInt(int64(i)))
			if err != nil {
				return nil, err
			}
			if i == 0 {
				st.token0, st.reserve0 = coin, bal
			} else {
				st.reserve1 = bal
			}
		}
		amp, err := c.callUint(ctx, pool, "A")
		if err != nil {
			return nil, err
		}
		st.amp = amp
	case amm.FamilyWeighted:
		// Weighted pools expose tokens(i), getBalances() and getWeights().
		for i := 0; i < 2; i++ {
			token, err := c.callAddress(ctx, pool, "tokens", big.NewInt(int64(i)))
			if err != nil {
				return nil, err
			}
			if i == 0 {
				st.token0 = token
			}
		}
		balances, err := c.callUintArray(ctx, pool, "getBalances")
		if err != nil {
			return nil, err
		}
		weights, err := c.callUintArray(ctx, pool, "getWeights")
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
		resOut, err := c.client.CallContract(ctx, ethereum.CallMsg{To: &pool, Data: mustPack(poolABI, "getReserves")}, nil)
		if err != nil {
			return nil, fmt.Errorf("pool.getReserves: %w", err)
		}
		res, err := poolABI.Unpack("getReserves", resOut)
		if err != nil || len(res) < 2 {
			return nil, fmt.Errorf("unpack getReserves: %w", err)
		}
		st.reserve0, _ = res[0].(*big.Int)
		st.reserve1, _ = res[1].(*big.Int)
		t0, err := c.callAddress(ctx, pool, "token0")
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

// callAddress reads a single address-returning view function.
func (c *Client) callAddress(ctx context.Context, pool common.Address, method string, args ...interface{}) (common.Address, error) {
	out, err := c.client.CallContract(ctx, ethereum.CallMsg{To: &pool, Data: mustPack(poolABI, method, args...)}, nil)
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
func (c *Client) callUint(ctx context.Context, pool common.Address, method string, args ...interface{}) (*big.Int, error) {
	out, err := c.client.CallContract(ctx, ethereum.CallMsg{To: &pool, Data: mustPack(poolABI, method, args...)}, nil)
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
func (c *Client) callUintArray(ctx context.Context, pool common.Address, method string, args ...interface{}) ([]*big.Int, error) {
	out, err := c.client.CallContract(ctx, ethereum.CallMsg{To: &pool, Data: mustPack(poolABI, method, args...)}, nil)
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

// QuoteFamily computes the output amount for one family across a token path.
func (c *Client) QuoteFamily(ctx context.Context, family amm.Family, path []common.Address, amountIn *big.Int, feeBps uint32) (*big.Int, error) {
	if len(path) < 2 {
		return nil, errors.New("path must have at least 2 tokens")
	}
	current := new(big.Int).Set(amountIn)

	for i := 0; i < len(path)-1; i++ {
		pool, err := c.ResolvePool(ctx, path[i], path[i+1], family)
		if err != nil {
			return nil, err
		}
		// Concentrated liquidity cannot be reproduced from two reserves, so it
		// is quoted through the pool's own quoter, which walks the real tick
		// bitmap. The other families expose reserves and are priced locally.
		if family == amm.FamilyV3 {
			current, err = c.quoteV3(ctx, path[i], path[i+1], current, feeBps)
			if err != nil {
				return nil, err
			}
			continue
		}

		st, err := c.loadState(ctx, pool, family)
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
			current, err = amm.AmountOutXY(current, reserveIn, reserveOut, feeBps)
		case amm.FamilyStable:
			current, err = amm.AmountOutStable(current, reserveIn, reserveOut, st.amp, feeBps)
		case amm.FamilyWeighted:
			current, err = amm.AmountOutWeighted(current, reserveIn, reserveOut, weightIn, weightOut, feeBps)
		default:
			return nil, fmt.Errorf("unsupported family %s", family)
		}
		if err != nil {
			return nil, err
		}
	}
	return current, nil
}

// quoteV3 prices one concentrated-liquidity hop by calling TigerSwap's own
// quoter, which walks the pool's real tick bitmap instead of approximating the
// pool with two reserves.
func (c *Client) quoteV3(ctx context.Context, tokenIn, tokenOut common.Address, amountIn *big.Int, feeBps uint32) (*big.Int, error) {
	if c.deployment.Quoter == (common.Address{}) {
		return nil, fmt.Errorf("chain %d has no TigerSwap quoter configured", c.deployment.ChainID)
	}

	data, err := quoterABI.Pack("quoteExactInputSingle", tokenIn, tokenOut, new(big.Int).SetUint64(uint64(feeBps)), amountIn)
	if err != nil {
		return nil, err
	}

	out, err := c.client.CallContract(ctx, ethereum.CallMsg{To: &c.deployment.Quoter, Data: data}, nil)
	if err != nil {
		return nil, fmt.Errorf("quoter.quoteExactInputSingle: %w", err)
	}

	vals, err := quoterABI.Unpack("quoteExactInputSingle", out)
	if err != nil {
		return nil, fmt.Errorf("unpack quote: %w", err)
	}
	if len(vals) == 0 {
		return nil, errors.New("quoter returned no output")
	}
	amountOut, ok := vals[0].(*big.Int)
	if !ok || amountOut == nil {
		return nil, errors.New("quoter returned a non-numeric output")
	}
	if amountOut.Sign() <= 0 {
		return nil, fmt.Errorf("quoter returned no output for %s -> %s", tokenIn.Hex(), tokenOut.Hex())
	}
	return amountOut, nil
}

// Swap signs and broadcasts a swap on the TigerSwap router.
func (c *Client) Swap(ctx context.Context, family amm.Family, path []common.Address, amountIn, amountOutMin *big.Int, to common.Address, deadline *big.Int) (*types.Transaction, error) {
	if c.signer == nil {
		return nil, errors.New("TigerSwap signer key is not configured")
	}
	if len(path) < 2 {
		return nil, errors.New("path must have at least 2 tokens")
	}

	data, err := routerABI.Pack("swapExactTokensForTokens",
		path[0], path[len(path)-1], FamilyCode(family), amountIn, amountOutMin, to, deadline)
	if err != nil {
		return nil, err
	}

	from := crypto.PubkeyToAddress(c.signer.PublicKey)
	nonce, err := c.client.PendingNonceAt(ctx, from)
	if err != nil {
		return nil, fmt.Errorf("pending nonce: %w", err)
	}
	gasPrice, err := c.client.SuggestGasPrice(ctx)
	if err != nil {
		return nil, fmt.Errorf("suggest gas price: %w", err)
	}
	gasLimit, err := c.client.EstimateGas(ctx, ethereum.CallMsg{
		From: from, To: &c.deployment.Router, Data: data, Value: big.NewInt(0),
	})
	if err != nil {
		return nil, fmt.Errorf("estimate gas: %w", err)
	}

	tx := types.NewTransaction(nonce, c.deployment.Router, big.NewInt(0), gasLimit, gasPrice, data)
	signed, err := types.SignTx(tx, types.LatestSignerForChainID(big.NewInt(c.deployment.ChainID)), c.signer)
	if err != nil {
		return nil, fmt.Errorf("sign tx: %w", err)
	}
	if err := c.client.SendTransaction(ctx, signed); err != nil {
		return nil, fmt.Errorf("send tx: %w", err)
	}
	return signed, nil
}

// BuildSwapCalldata returns the router target and calldata for an unsigned
// swap. The caller (a user's wallet) signs and broadcasts it; no key is needed.
func (c *Client) BuildSwapCalldata(family amm.Family, path []common.Address, amountIn, amountOutMin *big.Int, to common.Address, deadline *big.Int) (common.Address, []byte, error) {
	if len(path) < 2 {
		return common.Address{}, nil, errors.New("path must have at least 2 tokens")
	}
	data, err := routerABI.Pack("swapExactTokensForTokens",
		path[0], path[len(path)-1], FamilyCode(family), amountIn, amountOutMin, to, deadline)
	if err != nil {
		return common.Address{}, nil, err
	}
	return c.deployment.Router, data, nil
}

// RouterAddress returns the configured TigerSwap router for the chain.
func (c *Client) RouterAddress() common.Address { return c.deployment.Router }

// TokenPriceWAD reads a token's raw WAD-scaled (1e18) price from TigerSwap's own
// on-chain oracle. Callers that need an exact value should use this rather than
// the float form, which necessarily rounds.
func (c *Client) TokenPriceWAD(ctx context.Context, token common.Address) (*big.Int, error) {
	if c.deployment.Oracle == (common.Address{}) {
		return nil, fmt.Errorf("no TigerSwap oracle configured for chain %d", c.deployment.ChainID)
	}

	data, err := oracleABI.Pack("getPrice", token)
	if err != nil {
		return nil, err
	}
	out, err := c.client.CallContract(ctx, ethereum.CallMsg{To: &c.deployment.Oracle, Data: data}, nil)
	if err != nil {
		return nil, fmt.Errorf("oracle.getPrice(%s): %w", token.Hex(), err)
	}
	vals, err := oracleABI.Unpack("getPrice", out)
	if err != nil {
		return nil, fmt.Errorf("unpack getPrice: %w", err)
	}
	price, ok := vals[0].(*big.Int)
	if !ok || price.Sign() <= 0 {
		return nil, fmt.Errorf("oracle returned no usable price for %s", token.Hex())
	}
	return price, nil
}

// TokenPriceUSD reads a token's price from TigerSwap's own on-chain oracle and
// returns it as a decimal float. It fails closed when no oracle is configured
// or the oracle cannot answer, so a caller never falls back to a guessed price.
func (c *Client) TokenPriceUSD(ctx context.Context, token common.Address) (float64, error) {
	wad, err := c.TokenPriceWAD(ctx, token)
	if err != nil {
		return 0, err
	}
	scaled, _ := new(big.Float).SetInt(wad).Float64()
	return scaled / 1e18, nil
}

// LiveOracleReports returns how many independent reporters are currently fresh
// for a token, so a caller can judge how much to trust the price.
func (c *Client) LiveOracleReports(ctx context.Context, token common.Address) (uint64, error) {
	if c.deployment.Oracle == (common.Address{}) {
		return 0, fmt.Errorf("no TigerSwap oracle configured for chain %d", c.deployment.ChainID)
	}

	out, err := c.client.CallContract(ctx, ethereum.CallMsg{
		To:   &c.deployment.Oracle,
		Data: mustPack(oracleABI, "liveReportCount", token),
	}, nil)
	if err != nil {
		return 0, fmt.Errorf("oracle.liveReportCount(%s): %w", token.Hex(), err)
	}
	vals, err := oracleABI.Unpack("liveReportCount", out)
	if err != nil {
		return 0, fmt.Errorf("unpack liveReportCount: %w", err)
	}
	count, _ := vals[0].(*big.Int)
	if count == nil {
		return 0, nil
	}
	return count.Uint64(), nil
}

// LoadSigner reads the TigerSwap signer key from the environment.
func LoadSigner() (*ecdsa.PrivateKey, error) {
	raw := os.Getenv("TIGERSWAP_SIGNER_KEY")
	if raw == "" {
		return nil, errors.New("TIGERSWAP_SIGNER_KEY is not configured")
	}
	return crypto.HexToECDSA(strings.TrimPrefix(raw, "0x"))
}

// LoadDeployments reads the TigerSwap deployment set from the environment.
// TIGERSWAP_CHAINS is a comma-separated chain list; each chain uses
// TIGERSWAP_FACTORY_<id>, TIGERSWAP_ROUTER_<id>, TIGERSWAP_QUOTER_<id>,
// TIGERSWAP_ORACLE_<id> and TIGERSWAP_RPC_<id>. The RPC must be a TigerSwap
// node; no third-party RPC provider is used.
func LoadDeployments() []Deployment {
	raw := os.Getenv("TIGERSWAP_CHAINS")
	if raw == "" {
		return nil
	}
	var deployments []Deployment
	for _, part := range strings.Split(raw, ",") {
		id, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64)
		if err != nil {
			continue
		}
		suffix := strconv.FormatInt(id, 10)
		deployments = append(deployments, Deployment{
			ChainID: id,
			Factory: common.HexToAddress(os.Getenv("TIGERSWAP_FACTORY_" + suffix)),
			Router:  common.HexToAddress(os.Getenv("TIGERSWAP_ROUTER_" + suffix)),
			Quoter:  common.HexToAddress(os.Getenv("TIGERSWAP_QUOTER_" + suffix)),
			Oracle:  common.HexToAddress(os.Getenv("TIGERSWAP_ORACLE_" + suffix)),
			RPCURL:  os.Getenv("TIGERSWAP_RPC_" + suffix),
		})
	}
	return deployments
}

// Pool is a TigerSwap pool summary.
type Pool struct {
	Address  common.Address
	Family   amm.Family
	Token0   common.Address
	Token1   common.Address
	Reserve0 *big.Int
	Reserve1 *big.Int
}

// PoolsForPair returns the pools that exist for a token pair across all families.
func (c *Client) PoolsForPair(ctx context.Context, tokenA, tokenB common.Address) ([]Pool, error) {
	families := []amm.Family{amm.FamilyV2, amm.FamilyV3, amm.FamilyStable, amm.FamilyWeighted}
	var pools []Pool
	var firstErr error

	for _, family := range families {
		addr, err := c.ResolvePool(ctx, tokenA, tokenB, family)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		st, err := c.loadState(ctx, addr, family)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		pools = append(pools, Pool{
			Address:  addr,
			Family:   family,
			Token0:   st.token0,
			Token1:   tokenB,
			Reserve0: st.reserve0,
			Reserve1: st.reserve1,
		})
	}

	if len(pools) == 0 && firstErr != nil {
		return nil, firstErr
	}
	return pools, nil
}
