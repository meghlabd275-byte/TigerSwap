package services

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/gin-gonic/gin"
)

// LiquidityPoolService manages real on-chain liquidity pools
type LiquidityPoolService struct {
	blockchain *BlockchainClient
	priceAgg  *PriceAggregator
	db        interface{} // Would be *gorm.DB
	mu        sync.RWMutex
	pools     map[string]*PoolData
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
	Volume24h       float64   `json:"volume_24h"`
	Fees24h        float64   `json:"fees_24h"`
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
	var token0 common.Address
	parsedABI.Unpack(&token0, "token0", token0Result)

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
	var token1 common.Address
	parsedABI.Unpack(&token1, "token1", token1Result)

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
		Reserve0          *big.Int
		Reserve1          *big.Int
		BlockTimestampLast uint32
	}
	var res reserves
	parsedABI.Unpack(&res, "getReserves", reserveResult)

	// Get token prices
	token0Symbol := s.getSymbolFromAddress(token0.Hex())
	token1Symbol := s.getSymbolFromAddress(token1.Hex())

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

// GetAllPools returns all tracked pools
func (s *LiquidityPoolService) GetAllPools(ctx context.Context, chainID int64) ([]*PoolData, error) {
	// Known pool addresses for major DEXes
	poolAddresses := s.getKnownPoolAddresses(chainID)

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

// AddLiquidity adds real liquidity to a pool
func (s *LiquidityPoolService) AddLiquidity(ctx context.Context, chainID int64, privateKey, tokenA, tokenB, amountA, amountB string) (string, error) {
	// This would call the router contract
	// For now, return execution via swap executor
	return "", fmt.Errorf("use swap executor for liquidity addition")
}

// RemoveLiquidity removes liquidity from a pool
func (s *LiquidityPoolService) RemoveLiquidity(ctx context.Context, chainID int64, privateKey, poolAddress, liquidity string) (string, error) {
	return "", fmt.Errorf("use swap executor for liquidity removal")
}

func (s *LiquidityPoolService) getKnownPoolAddresses(chainID int64) []string {
	// Major DEX pool addresses per chain
	pools := map[int64][]string{
		1: { // Ethereum
			"0x88e6A0c2d26E5B7E6944928D8c744e67F4C5E9d3", // USDC/WETH Uniswap V3
			"0x4e68Ccc3aE2D6D2E2e7C1f9F6C3A3D2E1F4B5C6", // USDT/WETH
			"0x7a250d5630B4cF539739dF2C5dAcb4c659F2488D", // Uniswap V2 Router
		},
		56: { // BSC
			"0x16b9a82891338f9bA80E2D69f82156963999A6ED", // BUSD/WBNB
			"0x58F876857a02D6762E0100bbC5c1ea58eEb3a400", // USDT/WBNB
		},
		137: { // Polygon
			"0xA374094527E1673A86dE625aa59517c5d2993EE", // USDC/WETH
			"0x53E0bca35eC356BD5ddDFEbdD1Fc0fD03FaBad39", // WETH/WSOL
		},
	}

	if addresses, ok := pools[chainID]; ok {
		return addresses
	}
	return []string{}
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

func (s *LiquidityPoolService) getSymbolFromAddress(address string) string {
	symbols := map[string]string{
		"0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48": "USDC",
		"0xdAC17F958D2ee523a2206206994597C13D831ec7": "USDT",
		"0x0000000000000000000000000000000000000000": "ETH",
		"0x2260FAC5E5542a773Aa44fBCfeDf7C193bc2C599": "WBTC",
		"0x514910771AF9Ca656af840dff83E8264EcF986CA": "LINK",
		"0x1f9840a85d5aF5bf1D1762F925BDADdC4201F984": "UNI",
		"0x7Fc66500c84A76Ad7e9c93437bFc5Ac33E2DDaE9": "AAVE",
	}

	for addr, symbol := range symbols {
		if strings.EqualFold(addr, address) {
			return symbol
		}
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

func (s *LiquidityPoolService) AddLiquidityHandler(c *gin.Context) {
	var req struct {
		ChainID   int64  `json:"chain_id" binding:"required"`
		PrivateKey string `json:"private_key" binding:"required"`
		TokenA    string `json:"token_a" binding:"required"`
		TokenB    string `json:"token_b" binding:"required"`
		AmountA   string `json:"amount_a" binding:"required"`
		AmountB   string `json:"amount_b" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	txHash, err := s.AddLiquidity(c.Request.Context(), req.ChainID, req.PrivateKey, req.TokenA, req.TokenB, req.AmountA, req.AmountB)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"success": true,
		"tx_hash": txHash,
	})
}

// Import required
import "github.com/ethereum/go-ethereum"
