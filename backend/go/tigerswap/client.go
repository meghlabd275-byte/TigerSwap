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
 {"inputs":[{"internalType":"address","name":"tokenA","type":"address"},{"internalType":"address","name":"tokenB","type":"address"},{"internalType":"uint8","name":"family","type":"uint8"}],"name":"getPool","outputs":[{"internalType":"address","name":"pool","type":"address"}],"stateMutability":"view","type":"function"}
]`

const poolABIJSON = `[
 {"inputs":[],"name":"getReserves","outputs":[{"internalType":"uint256","name":"reserve0","type":"uint256"},{"internalType":"uint256","name":"reserve1","type":"uint256"},{"internalType":"uint32","name":"blockTimestampLast","type":"uint32"}],"stateMutability":"view","type":"function"},
 {"inputs":[],"name":"token0","outputs":[{"internalType":"address","name":"","type":"address"}],"stateMutability":"view","type":"function"},
 {"inputs":[],"name":"token1","outputs":[{"internalType":"address","name":"","type":"address"}],"stateMutability":"view","type":"function"},
 {"inputs":[],"name":"amplification","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},
 {"inputs":[],"name":"getWeights","outputs":[{"internalType":"uint256","name":"weight0","type":"uint256"},{"internalType":"uint256","name":"weight1","type":"uint256"}],"stateMutability":"view","type":"function"}
]`

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

	data, err := factoryABI.Pack("getPool", tokenA, tokenB, FamilyCode(family))
	if err != nil {
		return common.Address{}, err
	}
	out, err := c.client.CallContract(ctx, ethereum.CallMsg{To: &c.deployment.Factory, Data: data}, nil)
	if err != nil {
		return common.Address{}, fmt.Errorf("factory.getPool: %w", err)
	}
	vals, err := factoryABI.Unpack("getPool", out)
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

	t0Out, err := c.client.CallContract(ctx, ethereum.CallMsg{To: &pool, Data: mustPack(poolABI, "token0")}, nil)
	if err != nil {
		return nil, fmt.Errorf("pool.token0: %w", err)
	}
	t0, err := poolABI.Unpack("token0", t0Out)
	if err != nil {
		return nil, fmt.Errorf("unpack token0: %w", err)
	}
	st.token0, _ = t0[0].(common.Address)

	switch family {
	case amm.FamilyStable:
		out, err := c.client.CallContract(ctx, ethereum.CallMsg{To: &pool, Data: mustPack(poolABI, "amplification")}, nil)
		if err != nil {
			return nil, fmt.Errorf("pool.amplification: %w", err)
		}
		vals, err := poolABI.Unpack("amplification", out)
		if err != nil {
			return nil, fmt.Errorf("unpack amplification: %w", err)
		}
		st.amp, _ = vals[0].(*big.Int)
	case amm.FamilyWeighted:
		out, err := c.client.CallContract(ctx, ethereum.CallMsg{To: &pool, Data: mustPack(poolABI, "getWeights")}, nil)
		if err != nil {
			return nil, fmt.Errorf("pool.getWeights: %w", err)
		}
		vals, err := poolABI.Unpack("getWeights", out)
		if err != nil || len(vals) < 2 {
			return nil, fmt.Errorf("unpack getWeights: %w", err)
		}
		st.weight0, _ = vals[0].(*big.Int)
		st.weight1, _ = vals[1].(*big.Int)
	}
	return st, nil
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
		case amm.FamilyStable:
			current, err = amm.AmountOutStable(current, reserveIn, reserveOut, st.amp, feeBps)
		case amm.FamilyWeighted:
			current, err = amm.AmountOutWeighted(current, reserveIn, reserveOut, weightIn, weightOut, feeBps)
		default:
			current, err = amm.AmountOutXY(current, reserveIn, reserveOut, feeBps)
		}
		if err != nil {
			return nil, err
		}
	}
	return current, nil
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
// TIGERSWAP_FACTORY_<id>, TIGERSWAP_ROUTER_<id>, TIGERSWAP_QUOTER_<id> and
// TIGERSWAP_RPC_<id>.
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
