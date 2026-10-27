package services

import (
	"context"
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
	"github.com/gin-gonic/gin"

	"tigerswap/backend/config"
)

// LiquidityPoolService manages real on-chain liquidity pools
type LiquidityPoolService struct {
	blockchain *BlockchainClient
	priceAgg   *PriceAggregator
	db         interface{} // Would be *gorm.DB
	mu         sync.RWMutex
	pools      map[string]*PoolData
}

// PoolData represents real pool data from blockchain
type PoolData struct {
	Address     string    `json:"address"`
	Token0      string    `json:"token0"`
	Token1      string    `json:"token1"`
	Reserve0    string    `json:"reserve0"`
	Reserve1    string    `json:"reserve1"`
	TotalSupply string    `json:"total_supply"`
	Token0Price float64   `json:"token0_price"`
	Token1Price float64   `json:"token1_price"`
	Volume24h   float64   `json:"volume_24h"`
	Fees24h     float64   `json:"fees_24h"`
	APY         float64   `json:"apy"`
	LastUpdated time.Time `json:"last_updated"`
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
		pools:      make(map[string]*PoolData),
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

	// Calculate APY based on volume
	pool.APY = s.calculateAPY(pool)

	return pool, nil
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

func (s *LiquidityPoolService) calculateAPY(pool *PoolData) float64 {
	// Simplified APY calculation
	// Real implementation would calculate from fees and volume
	if pool.Volume24h > 0 {
		annualFees := pool.Fees24h * 365
		reserveValue := pool.Token0Price * parseFloat(pool.Reserve0)
		if reserveValue > 0 {
			return (annualFees / reserveValue) * 100
		}
	}
	return 0
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
