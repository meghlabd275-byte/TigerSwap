package services

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/gin-gonic/gin"

	"tigerswap/backend/config"
)

// LiquidityPoolService manages real on-chain liquidity pools
type LiquidityPoolService struct {
	blockchain *BlockchainClient
	priceAgg   *PriceAggregator
}

// PoolData represents real pool data from blockchain
type PoolData struct {
	Address         string    `json:"address"`
	Token0          string    `json:"token0"`
	Token1          string    `json:"token1"`
	Reserve0        string    `json:"reserve0"`
	Reserve1        string    `json:"reserve1"`
	TotalSupply     string    `json:"total_supply"`
	Token0Price     float64   `json:"token0_price"`
	Token1Price     float64   `json:"token1_price"`
	ReserveValueUSD float64   `json:"reserve_value_usd"`
	Volume24h       float64   `json:"volume_24h"`
	Fees24h         float64   `json:"fees_24h"`
	APY             float64   `json:"apy"`
	LastUpdated     time.Time `json:"last_updated"`
}

// PoolOrder represents an order in the pool
type PoolOrder struct {
	ID          string    `json:"id"`
	PoolAddress string    `json:"pool_address"`
	Trader      string    `json:"trader"`
	TokenIn     string    `json:"token_in"`
	TokenOut    string    `json:"token_out"`
	AmountIn    string    `json:"amount_in"`
	AmountOut   string    `json:"amount_out"`
	Status      string    `json:"status"` // pending, executed, cancelled
	Timestamp   time.Time `json:"timestamp"`
	BlockNumber uint64    `json:"block_number"`
	TxHash      string    `json:"tx_hash"`
}

func NewLiquidityPoolService(blockchain *BlockchainClient, priceAgg *PriceAggregator) *LiquidityPoolService {
	return &LiquidityPoolService{
		blockchain: blockchain,
		priceAgg:   priceAgg,
	}
}

// GetPoolData fetches real pool data from blockchain
func (s *LiquidityPoolService) GetPoolData(ctx context.Context, chainID int64, poolAddress string) (*PoolData, error) {
	client, err := s.blockchain.GetClient(chainID)
	if err != nil {
		return nil, err
	}

	poolAddr := common.HexToAddress(poolAddress)

	// ERC20 ABI for token0, token1, reserves
	poolABI := `[{"constant":true,"inputs":[],"name":"token0","outputs":[{"name":"","type":"address"}],"type":"function"},{"constant":true,"inputs":[],"name":"token1","outputs":[{"name":"","type":"address"}],"type":"function"},{"constant":true,"inputs":[],"name":"getReserves","outputs":[{"name":"_reserve0","type":"uint112"},{"name":"_reserve1","type":"uint112"},{"name":"_blockTimestampLast","type":"uint32"}],"type":"function"},{"constant":true,"inputs":[],"name":"totalSupply","outputs":[{"name":"","type":"uint256"}],"type":"function"}]`

	parsedABI, err := abi.JSON(strings.NewReader(poolABI))
	if err != nil {
		return nil, err
	}

	// Get token0
	token0Data, err := parsedABI.Pack("token0")
	if err != nil {
		return nil, err
	}
	token0Result, err := client.CallContract(ctx, ethereum.CallMsg{
		To:   &poolAddr,
		Data: token0Data,
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get token0: %v", err)
	}
	token0Vals, err := parsedABI.Unpack("token0", token0Result)
	if err != nil || len(token0Vals) != 1 {
		return nil, fmt.Errorf("decode token0: %w", err)
	}
	token0, ok := token0Vals[0].(common.Address)
	if !ok {
		return nil, fmt.Errorf("unexpected token0 type %T", token0Vals[0])
	}

	// Get token1
	token1Data, err := parsedABI.Pack("token1")
	if err != nil {
		return nil, err
	}
	token1Result, err := client.CallContract(ctx, ethereum.CallMsg{
		To:   &poolAddr,
		Data: token1Data,
	}, nil)
	if err != nil {
		return nil, err
	}
	token1Vals, err := parsedABI.Unpack("token1", token1Result)
	if err != nil || len(token1Vals) != 1 {
		return nil, fmt.Errorf("decode token1: %w", err)
	}
	token1, ok := token1Vals[0].(common.Address)
	if !ok {
		return nil, fmt.Errorf("unexpected token1 type %T", token1Vals[0])
	}

	// Get reserves
	reserveData, err := parsedABI.Pack("getReserves")
	if err != nil {
		return nil, err
	}
	reserveResult, err := client.CallContract(ctx, ethereum.CallMsg{
		To:   &poolAddr,
		Data: reserveData,
	}, nil)
	if err != nil {
		return nil, err
	}

	type reserves struct {
		Reserve0           *big.Int
		Reserve1           *big.Int
		BlockTimestampLast uint32
	}
	var res reserves
	if err := parsedABI.UnpackIntoInterface(&res, "getReserves", reserveResult); err != nil {
		return nil, fmt.Errorf("decode getReserves: %w", err)
	}

	// Get token prices
	token0Symbol := s.getSymbolFromAddress(chainID, token0.Hex())
	token1Symbol := s.getSymbolFromAddress(chainID, token1.Hex())

	token0Price, _ := s.priceAgg.GetRealPrice(ctx, token0Symbol)
	token1Price, _ := s.priceAgg.GetRealPrice(ctx, token1Symbol)

	// Reserves are raw integer amounts; their decimals are needed to value them.
	token0Decimals, err := s.tokenDecimals(ctx, chainID, client, token0)
	if err != nil {
		return nil, err
	}
	token1Decimals, err := s.tokenDecimals(ctx, chainID, client, token1)
	if err != nil {
		return nil, err
	}

	pool := &PoolData{
		Address:     poolAddress,
		Token0:      token0.Hex(),
		Token1:      token1.Hex(),
		Reserve0:    res.Reserve0.String(),
		Reserve1:    res.Reserve1.String(),
		Token0Price: token0Price.USD,
		Token1Price: token1Price.USD,
		LastUpdated: time.Now(),
	}

	// Read totalSupply, which the APY denominator needs.
	supplyData, err := parsedABI.Pack("totalSupply")
	if err != nil {
		return nil, err
	}
	supplyResult, err := client.CallContract(ctx, ethereum.CallMsg{
		To:   &poolAddr,
		Data: supplyData,
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get totalSupply: %v", err)
	}
	supplyVals, err := parsedABI.Unpack("totalSupply", supplyResult)
	if err != nil || len(supplyVals) != 1 {
		return nil, fmt.Errorf("decode totalSupply: %w", err)
	}
	supply, ok := supplyVals[0].(*big.Int)
	if !ok {
		return nil, fmt.Errorf("unexpected totalSupply type %T", supplyVals[0])
	}
	pool.TotalSupply = supply.String()

	// Value the pool in USD from both reserves. Prices come from the oracle; a
	// missing price leaves that side at zero rather than inventing a value.
	reserve0, err := amountToFloat(res.Reserve0, token0Decimals)
	if err != nil {
		return nil, err
	}
	reserve1, err := amountToFloat(res.Reserve1, token1Decimals)
	if err != nil {
		return nil, err
	}
	pool.ReserveValueUSD = reserve0*token0Price.USD + reserve1*token1Price.USD

	// Volume and fees come from the pool's own Swap logs over the last 24 hours.
	volume, fees, err := s.CalculateVolume24h(ctx, chainID, poolAddress, token0Price.USD, token1Price.USD)
	if err != nil {
		return nil, err
	}
	pool.Volume24h = volume
	pool.Fees24h = fees
	pool.APY = apyFromFees(fees, pool.ReserveValueUSD)

	return pool, nil
}

// apyFromFees annualises the fee revenue a pool generates against the value
// locked in it. It returns 0 when the pool holds nothing, because a rate of
// return on no capital is undefined rather than infinite.
func apyFromFees(fees24h, reserveValueUSD float64) float64 {
	if reserveValueUSD <= 0 {
		return 0
	}
	return (fees24h * 365 / reserveValueUSD) * 100
}

// GetAllPools returns every pool registered in TigerSwap's own factory.
func (s *LiquidityPoolService) GetAllPools(ctx context.Context, chainID int64) ([]*PoolData, error) {
	poolAddresses, err := s.getKnownPoolAddresses(ctx, chainID)
	if err != nil {
		return nil, err
	}

	var pools []*PoolData
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, addr := range poolAddresses {
		wg.Add(1)
		go func(address string) {
			defer wg.Done()
			pool, err := s.GetPoolData(ctx, chainID, address)
			mu.Lock()
			defer mu.Unlock()
			if err == nil && pool != nil {
				pools = append(pools, pool)
			}
		}(addr)
	}

	wg.Wait()
	return pools, nil
}

// BroadcastTransaction submits a client-signed transaction for this service.
func (s *LiquidityPoolService) BroadcastTransaction(ctx context.Context, chainID int64, signedTxHex string) (string, error) {
	client, err := s.blockchain.GetClient(chainID)
	if err != nil {
		return "", err
	}
	return broadcastSignedTransaction(ctx, client, signedTxHex)
}

// RemoveLiquidity removes liquidity from a pool
func (s *LiquidityPoolService) RemoveLiquidity(ctx context.Context, chainID int64, privateKey, poolAddress, liquidity string) (string, error) {
	return "", fmt.Errorf("use swap executor for liquidity removal")
}

// factoryABIJSON is the subset of TigerSwap's own factory needed to enumerate
// the pools it has created.
const factoryABIJSON = `[
 {"inputs":[],"name":"allPoolsLength","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},
 {"inputs":[{"internalType":"uint256","name":"","type":"uint256"}],"name":"allPools","outputs":[{"internalType":"address","name":"","type":"address"}],"stateMutability":"view","type":"function"}
]`

// getKnownPoolAddresses enumerates the pools registered in TigerSwap's own
// factory for the chain. No third-party pool addresses are baked in: only pools
// the operator deployed through the configured TigerSwap factory are returned.
func (s *LiquidityPoolService) getKnownPoolAddresses(ctx context.Context, chainID int64) ([]string, error) {
	factoryHex := os.Getenv(fmt.Sprintf("TIGERSWAP_FACTORY_%d", chainID))
	if factoryHex == "" {
		return nil, fmt.Errorf("no TigerSwap factory configured for chain %d; set TIGERSWAP_FACTORY_%d", chainID, chainID)
	}
	factory := common.HexToAddress(factoryHex)
	if factory == (common.Address{}) {
		return nil, fmt.Errorf("TIGERSWAP_FACTORY_%d is not a valid address", chainID)
	}

	client, err := s.blockchain.GetClient(chainID)
	if err != nil {
		return nil, err
	}
	parsedABI, err := abi.JSON(strings.NewReader(factoryABIJSON))
	if err != nil {
		return nil, err
	}

	lenOut, err := client.CallContract(ctx, ethereum.CallMsg{
		To:   &factory,
		Data: mustPackABI(parsedABI, "allPoolsLength"),
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("factory.allPoolsLength: %w", err)
	}
	lenVals, err := parsedABI.Unpack("allPoolsLength", lenOut)
	if err != nil || len(lenVals) != 1 {
		return nil, fmt.Errorf("decode allPoolsLength: %w", err)
	}
	count, ok := lenVals[0].(*big.Int)
	if !ok {
		return nil, fmt.Errorf("unexpected allPoolsLength type %T", lenVals[0])
	}

	addresses := make([]string, 0, count.Uint64())
	for i := big.NewInt(0); i.Cmp(count) < 0; i.Add(i, big.NewInt(1)) {
		out, err := client.CallContract(ctx, ethereum.CallMsg{
			To:   &factory,
			Data: mustPackABI(parsedABI, "allPools", i),
		}, nil)
		if err != nil {
			return nil, fmt.Errorf("factory.allPools(%s): %w", i, err)
		}
		vals, err := parsedABI.Unpack("allPools", out)
		if err != nil || len(vals) != 1 {
			return nil, fmt.Errorf("decode allPools(%s): %w", i, err)
		}
		pool, ok := vals[0].(common.Address)
		if !ok {
			return nil, fmt.Errorf("unexpected allPools type %T", vals[0])
		}
		if pool != (common.Address{}) {
			addresses = append(addresses, pool.Hex())
		}
	}
	return addresses, nil
}

func mustPackABI(a abi.ABI, name string, args ...interface{}) []byte {
	data, err := a.Pack(name, args...)
	if err != nil {
		panic(fmt.Sprintf("pack %s: %v", name, err))
	}
	return data
}

// swapEventABIJSON is the TigerSwap V2 pair Swap event, emitted by the pool
// contracts this service itself deploys. Volume is derived from these logs.
const swapEventABIJSON = `[{"anonymous":false,"inputs":[{"indexed":true,"name":"sender","type":"address"},{"indexed":false,"name":"amount0In","type":"uint256"},{"indexed":false,"name":"amount1In","type":"uint256"},{"indexed":false,"name":"amount0Out","type":"uint256"},{"indexed":false,"name":"amount1Out","type":"uint256"},{"indexed":true,"name":"to","type":"address"}],"name":"Swap","type":"event"}]`

// tigerSwapSwapFeeBps is the swap fee configured on the TigerSwap V2 factory,
// in basis points (30 = 0.3%). Fees are a fixed share of the trade notional.
const tigerSwapSwapFeeBps = 30

// poolQueryABIJSON reads the immutable pair identity plus live reserves and
// supply from a TigerSwap V2 pool.
const poolQueryABIJSON = `[{"constant":true,"inputs":[],"name":"token0","outputs":[{"name":"","type":"address"}],"type":"function"},{"constant":true,"inputs":[],"name":"token1","outputs":[{"name":"","type":"address"}],"type":"function"},{"constant":true,"inputs":[],"name":"getReserves","outputs":[{"name":"_reserve0","type":"uint112"},{"name":"_reserve1","type":"uint112"},{"name":"_blockTimestampLast","type":"uint32"}],"type":"function"},{"constant":true,"inputs":[],"name":"totalSupply","outputs":[{"name":"","type":"uint256"}],"type":"function"}]`

// erc20DecimalsABIJSON reads an ERC20's decimals. Only decimals are needed to
// scale raw amounts; symbols come from the operator's token registry.
const erc20DecimalsABIJSON = `[{"constant":true,"inputs":[],"name":"decimals","outputs":[{"name":"","type":"uint8"}],"type":"function"}]`

// tokenDecimals returns the decimals of an ERC20, preferring the operator's
// token registry and falling back to the contract itself. A token whose decimals
// cannot be determined is an error, because scaling its amounts with a guessed
// value would misprice volume.
func (s *LiquidityPoolService) tokenDecimals(ctx context.Context, chainID int64, client *ethclient.Client, token common.Address) (int, error) {
	if symbol, ok := config.TokenSymbol(chainID, token.Hex()); ok {
		for _, t := range config.GetTokenConfigs()[chainID] {
			if strings.EqualFold(t.Symbol, symbol) && t.Decimals > 0 {
				return t.Decimals, nil
			}
		}
	}

	parsedABI, err := abi.JSON(strings.NewReader(erc20DecimalsABIJSON))
	if err != nil {
		return 0, err
	}
	data, err := parsedABI.Pack("decimals")
	if err != nil {
		return 0, err
	}
	out, err := client.CallContract(ctx, ethereum.CallMsg{To: &token, Data: data}, nil)
	if err != nil {
		return 0, fmt.Errorf("token.decimals(%s): %w", token.Hex(), err)
	}
	values, err := parsedABI.Unpack("decimals", out)
	if err != nil || len(values) != 1 {
		return 0, fmt.Errorf("decode decimals for %s: %w", token.Hex(), err)
	}
	decimals, ok := values[0].(uint8)
	if !ok {
		return 0, fmt.Errorf("unexpected decimals type %T for %s", values[0], token.Hex())
	}
	return int(decimals), nil
}

// amountToFloat converts a raw token amount to a decimal value using the token's
// decimals.
func amountToFloat(raw *big.Int, decimals int) (float64, error) {
	if raw == nil {
		return 0, errors.New("nil token amount")
	}
	if decimals < 0 || decimals > 78 {
		return 0, fmt.Errorf("token decimals %d out of range", decimals)
	}
	rat := new(big.Rat).SetInt(raw)
	rat.Quo(rat, new(big.Rat).SetInt(pow10(decimals)))
	value, _ := rat.Float64()
	return value, nil
}

// CalculateVolume24h sums the USD value of every swap in a pool over the last 24
// hours by reading the pool's own Swap logs. This is real on-chain data: when the
// RPC cannot supply logs the error is returned instead of a fabricated volume.
func (s *LiquidityPoolService) CalculateVolume24h(ctx context.Context, chainID int64, poolAddress string, token0Price, token1Price float64) (volumeUSD, feesUSD float64, err error) {
	client, err := s.blockchain.GetClient(chainID)
	if err != nil {
		return 0, 0, err
	}
	if token0Price < 0 || token1Price < 0 {
		return 0, 0, errors.New("token prices must not be negative")
	}

	parsedPoolABI, err := abi.JSON(strings.NewReader(poolQueryABIJSON))
	if err != nil {
		return 0, 0, err
	}
	poolAddr := common.HexToAddress(poolAddress)

	readAddress := func(method string) (common.Address, error) {
		data, err := parsedPoolABI.Pack(method)
		if err != nil {
			return common.Address{}, err
		}
		out, err := client.CallContract(ctx, ethereum.CallMsg{To: &poolAddr, Data: data}, nil)
		if err != nil {
			return common.Address{}, fmt.Errorf("pool.%s: %w", method, err)
		}
		values, err := parsedPoolABI.Unpack(method, out)
		if err != nil || len(values) != 1 {
			return common.Address{}, fmt.Errorf("decode %s: %w", method, err)
		}
		addr, ok := values[0].(common.Address)
		if !ok {
			return common.Address{}, fmt.Errorf("unexpected %s type %T", method, values[0])
		}
		return addr, nil
	}

	token0, err := readAddress("token0")
	if err != nil {
		return 0, 0, err
	}
	token1, err := readAddress("token1")
	if err != nil {
		return 0, 0, err
	}
	token0Decimals, err := s.tokenDecimals(ctx, chainID, client, token0)
	if err != nil {
		return 0, 0, err
	}
	token1Decimals, err := s.tokenDecimals(ctx, chainID, client, token1)
	if err != nil {
		return 0, 0, err
	}

	latest, err := client.BlockNumber(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("latest block: %w", err)
	}
	if latest == 0 {
		return 0, 0, nil
	}

	// A pool with no swaps is legitimately zero volume, so a window that reaches
	// before genesis is clamped rather than rejected.
	blockTime := int64(2)
	if cfg := s.blockchain.getChainConfig(chainID); cfg != nil && cfg.BlockTime > 0 {
		blockTime = int64(cfg.BlockTime)
	}
	windowBlocks := uint64(86400 / blockTime)
	var fromBlock uint64
	if latest > windowBlocks {
		fromBlock = latest - windowBlocks
	}

	parsedEventABI, err := abi.JSON(strings.NewReader(swapEventABIJSON))
	if err != nil {
		return 0, 0, err
	}
	eventID := parsedEventABI.Events["Swap"].ID

	logs, err := client.FilterLogs(ctx, ethereum.FilterQuery{
		FromBlock: new(big.Int).SetUint64(fromBlock),
		ToBlock:   new(big.Int).SetUint64(latest),
		Addresses: []common.Address{poolAddr},
		Topics:    [][]common.Hash{{eventID}},
	})
	if err != nil {
		return 0, 0, fmt.Errorf("filter Swap logs: %w", err)
	}

	for _, lg := range logs {
		values, err := parsedEventABI.Unpack("Swap", lg.Data)
		if err != nil || len(values) != 4 {
			return 0, 0, fmt.Errorf("decode Swap log %s: %w", lg.TxHash.Hex(), err)
		}
		amount0In, ok0 := values[0].(*big.Int)
		amount1In, ok1 := values[1].(*big.Int)
		amount0Out, ok2 := values[2].(*big.Int)
		amount1Out, ok3 := values[3].(*big.Int)
		if !ok0 || !ok1 || !ok2 || !ok3 {
			return 0, 0, fmt.Errorf("unexpected Swap log value types in %s", lg.TxHash.Hex())
		}

		// A V2 swap moves value on one side of each token; the input and output
		// legs are summed so each trade is counted once.
		amount0, err := amountToFloat(new(big.Int).Add(amount0In, amount0Out), token0Decimals)
		if err != nil {
			return 0, 0, err
		}
		amount1, err := amountToFloat(new(big.Int).Add(amount1In, amount1Out), token1Decimals)
		if err != nil {
			return 0, 0, err
		}
		volumeUSD += amount0*token0Price + amount1*token1Price
	}

	feesUSD = volumeUSD * float64(tigerSwapSwapFeeBps) / 10000
	return volumeUSD, feesUSD, nil
}

func (s *LiquidityPoolService) getSymbolFromAddress(chainID int64, address string) string {
	if symbol, ok := config.TokenSymbol(chainID, address); ok {
		return symbol
	}
	return "UNKNOWN"
}

// API Handlers

func (s *LiquidityPoolService) GetPoolHandler(c *gin.Context) {
	chainID := c.Param("chain_id")
	address := c.Param("address")

	chainIDInt, err := parseInt64(chainID)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid chain_id"})
		return
	}

	pool, err := s.GetPoolData(c.Request.Context(), chainIDInt, address)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, pool)
}

func (s *LiquidityPoolService) GetPoolsHandler(c *gin.Context) {
	chainID := c.Param("chain_id")

	chainIDInt, err := parseInt64(chainID)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid chain_id"})
		return
	}

	pools, err := s.GetAllPools(c.Request.Context(), chainIDInt)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"pools": pools,
		"count": len(pools),
	})
}

// AddLiquidityHandler broadcasts a client-signed add-liquidity transaction.
func (s *LiquidityPoolService) AddLiquidityHandler(c *gin.Context) {
	var req struct {
		ChainID  int64  `json:"chain_id" binding:"required"`
		SignedTx string `json:"signed_tx" binding:"required"`
		TokenA   string `json:"token_a" binding:"required"`
		TokenB   string `json:"token_b" binding:"required"`
		AmountA  string `json:"amount_a" binding:"required"`
		AmountB  string `json:"amount_b" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	txHash, err := s.BroadcastTransaction(c.Request.Context(), req.ChainID, req.SignedTx)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"tx_hash": txHash,
		"status":  "submitted",
	})
}
