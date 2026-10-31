package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"tigerswap/backend/models"
)

// BlockchainService handles blockchain operations
type BlockchainService struct {
	db *gorm.DB
}

func NewBlockchainService(db *gorm.DB) *BlockchainService {
	// Seed initial chains if empty
	var count int64
	db.Model(&models.Blockchain{}).Count(&count)
	if count == 0 {
		seedChains(db)
	}
	return &BlockchainService{db: db}
}

func seedChains(db *gorm.DB) {
	// RPC endpoints are left unset: TigerSwap dials its own nodes, resolved at
	// runtime from TIGERSWAP_RPC_<chainID>. Only metadata is seeded here.
	chains := []models.Blockchain{
		{ChainID: 1, Name: "Ethereum", Symbol: "ETH", Icon: "🦄", ExplorerURL: "https://etherscan.io", Type: "evm", IsActive: true},
		{ChainID: 137, Name: "Polygon", Symbol: "MATIC", Icon: "🔷", ExplorerURL: "https://polygonscan.com", Type: "evm", IsActive: true},
		{ChainID: 42161, Name: "Arbitrum One", Symbol: "ETH", Icon: "🔵", ExplorerURL: "https://arbiscan.io", Type: "evm", IsActive: true},
		{ChainID: 10, Name: "Optimism", Symbol: "ETH", Icon: "🔴", ExplorerURL: "https://optimistic.etherscan.io", Type: "evm", IsActive: true},
		{ChainID: 8453, Name: "Base", Symbol: "ETH", Icon: "🔵", ExplorerURL: "https://basescan.org", Type: "evm", IsActive: true},
		{ChainID: 56, Name: "BNB Smart Chain", Symbol: "BNB", Icon: "🟡", ExplorerURL: "https://bscscan.com", Type: "evm", IsActive: true},
		{ChainID: 43114, Name: "Avalanche", Symbol: "AVAX", Icon: "🔺", ExplorerURL: "https://snowtrace.io", Type: "evm", IsActive: true},
		{ChainID: 250, Name: "Fantom", Symbol: "FTM", Icon: "👻", ExplorerURL: "https://ftmscan.com", Type: "evm", IsActive: true},
		{ChainID: 101, Name: "Solana", Symbol: "SOL", Icon: "☀️", ExplorerURL: "https://solscan.io", Type: "solana", IsActive: true},
		{ChainID: 9001, Name: "Aptos", Symbol: "APT", Icon: "🔷", ExplorerURL: "https://aptoscan.com", Type: "aptos", IsActive: true},
		{ChainID: 9002, Name: "Cosmos", Symbol: "ATOM", Icon: "🌌", ExplorerURL: "https://mintscan.io/cosmos", Type: "cosmos", IsActive: true},
		{ChainID: 9003, Name: "TON", Symbol: "TON", Icon: "📱", ExplorerURL: "https://tonscan.org", Type: "ton", IsActive: true},
	}
	for _, chain := range chains {
		if err := db.Create(&chain).Error; err != nil {
			log.Printf("failed to seed chain %s (chain_id %d): %v", chain.Name, chain.ChainID, err)
		}
	}
	log.Println("Seeded blockchain data")
}

func (s *BlockchainService) GetSupportedChains(c *gin.Context) {
	var chains []models.Blockchain
	s.db.Where("is_active = ?", true).Find(&chains)
	c.JSON(200, gin.H{"chains": chains, "count": len(chains)})
}

func (s *BlockchainService) GetChain(c *gin.Context) {
	id := c.Param("id")
	var chain models.Blockchain
	if err := s.db.Where("chain_id = ?", id).First(&chain).Error; err != nil {
		c.JSON(404, gin.H{"error": "Chain not found"})
		return
	}
	c.JSON(200, chain)
}

// TokenService handles token operations
type TokenService struct {
	db *gorm.DB
}

func NewTokenService(db *gorm.DB) *TokenService {
	// Seed initial tokens if empty
	var count int64
	db.Model(&models.Token{}).Count(&count)
	if count == 0 {
		seedTokens(db)
	}
	return &TokenService{db: db}
}

func seedTokens(db *gorm.DB) {
	tokens := []models.Token{
		{ChainID: 1, Address: "0x0000000000000000000000000000000000000000", Name: "Ethereum", Symbol: "ETH", Decimals: 18, LogoURI: "https://assets.coingecko.com/coins/images/279/small/ethereum.png", IsActive: true, IsVerified: true},
		{ChainID: 1, Address: "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48", Name: "USD Coin", Symbol: "USDC", Decimals: 6, LogoURI: "https://assets.coingecko.com/coins/images/6319/small/usdc.png", IsActive: true, IsVerified: true},
		{ChainID: 1, Address: "0xdAC17F958D2ee523a2206206994597C13D831ec7", Name: "Tether USD", Symbol: "USDT", Decimals: 6, LogoURI: "https://assets.coingecko.com/coins/images/325/small/Tether.png", IsActive: true, IsVerified: true},
		{ChainID: 1, Address: "0x2260FAC5E5542a773Aa44fBCfeDf7C193bc2C599", Name: "Wrapped Bitcoin", Symbol: "WBTC", Decimals: 8, LogoURI: "https://assets.coingecko.com/coins/images/7598/small/wrapped_bitcoin_wbtc.png", IsActive: true, IsVerified: true},
		{ChainID: 1, Address: "0x514910771AF9Ca656af840dff83E8264EcF986CA", Name: "Chainlink", Symbol: "LINK", Decimals: 18, LogoURI: "https://assets.coingecko.com/coins/images/877/small/chainlink-new-logo.png", IsActive: true, IsVerified: true},
		{ChainID: 1, Address: "0x1f9840a85d5aF5bf1D1762F925BDADdC4201F984", Name: "Uniswap", Symbol: "UNI", Decimals: 18, LogoURI: "https://assets.coingecko.com/coins/images/12504/small/uniswap-uni.png", IsActive: true, IsVerified: true},
		{ChainID: 137, Address: "0x0000000000000000000000000000000000000000", Name: "Polygon", Symbol: "MATIC", Decimals: 18, LogoURI: "https://assets.coingecko.com/coins/images/4713/small/polygon.png", IsActive: true, IsVerified: true},
		{ChainID: 56, Address: "0xbb4CdB9CBd36B01bD1cBaEBF2De08d9173bc095c", Name: "Wrapped BNB", Symbol: "WBNB", Decimals: 18, LogoURI: "https://assets.coingecko.com/coins/images/825/small/bnb-icon2_2x.png", IsActive: true, IsVerified: true},
	}
	for _, token := range tokens {
		if err := db.Create(&token).Error; err != nil {
			log.Printf("failed to seed token %s on chain %d: %v", token.Symbol, token.ChainID, err)
		}
	}
	log.Println("Seeded token data")
}

func (s *TokenService) GetTokens(c *gin.Context) {
	chainID := c.Query("chain_id")
	query := s.db.Where("is_active = ?", true)
	if chainID != "" {
		query = query.Where("chain_id = ?", chainID)
	}
	var tokens []models.Token
	query.Find(&tokens)
	c.JSON(200, gin.H{"tokens": tokens, "count": len(tokens)})
}

func (s *TokenService) GetToken(c *gin.Context) {
	address := c.Param("address")
	var token models.Token
	if err := s.db.Where("address = ?", address).First(&token).Error; err != nil {
		c.JSON(404, gin.H{"error": "Token not found"})
		return
	}
	c.JSON(200, token)
}

func (s *TokenService) SearchTokens(c *gin.Context) {
	query := c.Query("q")
	var tokens []models.Token
	s.db.Where("symbol ILIKE ? OR name ILIKE ?", "%"+query+"%", "%"+query+"%").Limit(20).Find(&tokens)
	c.JSON(200, gin.H{"tokens": tokens})
}

// SwapService handles swap operations
type SwapService struct {
	db    *gorm.DB
	redis *redis.Client
}

func NewSwapService(db *gorm.DB, redis *redis.Client) *SwapService {
	return &SwapService{db: db, redis: redis}
}

// PriceAggregator instance - injected via constructor in production
var globalPriceAggregator *PriceAggregator

func SetPriceAggregator(agg *PriceAggregator) {
	globalPriceAggregator = agg
}

func (s *SwapService) GetQuote(c *gin.Context) {
	var req struct {
		TokenIn  string `json:"token_in" binding:"required"`
		TokenOut string `json:"token_out" binding:"required"`
		Amount   string `json:"amount" binding:"required"`
		ChainID  int64  `json:"chain_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	// Use price aggregator for real quotes
	if globalPriceAggregator != nil {
		quote, err := globalPriceAggregator.GetAggregatedQuote(c.Request.Context(), req.ChainID, req.TokenIn, req.TokenOut, req.Amount, nil)
		if err == nil {
			c.JSON(200, quote)
			return
		}
	}

	// Fallback: calculate from individual prices
	tokenInSymbol := s.getSymbolFromAddress(req.TokenIn)
	tokenOutSymbol := s.getSymbolFromAddress(req.TokenOut)

	var tokenInPrice, tokenOutPrice float64 = 1.0, 1.0

	if globalPriceAggregator != nil {
		if pd, err := globalPriceAggregator.GetRealPrice(c.Request.Context(), tokenInSymbol); err == nil {
			tokenInPrice = pd.USD
		}
		if pd, err := globalPriceAggregator.GetRealPrice(c.Request.Context(), tokenOutSymbol); err == nil {
			tokenOutPrice = pd.USD
		}
	}

	rate := tokenOutPrice / tokenInPrice
	amountFloat, _ := new(big.Float).SetString(req.Amount)
	amountOut := new(big.Float).Mul(amountFloat, big.NewFloat(rate))

	// Apply 0.3% fee
	amountOutMin := new(big.Float).Mul(amountOut, big.NewFloat(0.997))

	quote := gin.H{
		"token_in":       req.TokenIn,
		"token_out":      req.TokenOut,
		"amount_in":      req.Amount,
		"amount_out":     amountOut.String(),
		"amount_out_min": amountOutMin.String(),
		"rate":           fmt.Sprintf("%f", rate),
		"price_impact":   "0.1",
		"gas_estimate":   "21000",
		"chain_id":       req.ChainID,
		"route":          []string{req.TokenIn, req.TokenOut},
	}

	c.JSON(200, quote)
}

func (s *SwapService) getSymbolFromAddress(address string) string {
	symbols := map[string]string{
		"0x0000000000000000000000000000000000000000": "ETH",
		"0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48": "USDC",
		"0xdac17f958d2ee523a2206206994597c13d831ec7": "USDT",
		"0x2260fac5e5542a773aa44fbcfedf7c193bc2c599": "WBTC",
		"0x514910771af9ca656af840dff83e8264ecf986ca": "LINK",
		"0x1f9840a85d5af5bf1d1762f925bdaddc4201f984": "UNI",
		"0x7fc66500c84a76ad7e9c93437bfc5ac33e2ddae9": "AAVE",
	}
	for addr, sym := range symbols {
		if strings.EqualFold(addr, address) {
			return sym
		}
	}
	return "UNKNOWN"
}

// BuildTransaction returns an unsigned TigerSwap router call for the client
// wallet to sign. The server holds no key for this path.
func (s *SwapService) BuildTransaction(c *gin.Context) {
	var req struct {
		TokenIn     string `json:"token_in" binding:"required"`
		TokenOut    string `json:"token_out" binding:"required"`
		AmountIn    string `json:"amount_in" binding:"required"`
		ChainID     int64  `json:"chain_id"`
		Recipient   string `json:"recipient" binding:"required"`
		SlippageBps uint32 `json:"slippage_bps"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if globalSwapExecutor == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "swap executor not configured"})
		return
	}
	if req.SlippageBps == 0 {
		req.SlippageBps = DefaultSlippageBps
	}

	chainID := req.ChainID
	if chainID == 0 {
		chainID = 1
	}

	to, data, value, gasLimit, err := globalSwapExecutor.BuildSwap(
		c.Request.Context(), chainID, req.TokenIn, req.TokenOut, req.AmountIn, req.Recipient, req.SlippageBps,
	)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"to":        to,
		"data":      data,
		"value":     value,
		"gas_limit": strconv.FormatUint(gasLimit, 10),
		"chain_id":  chainID,
	})
}

func (s *SwapService) GetPairs(c *gin.Context) {
	var pools []models.Pool
	s.db.Where("is_active = ?", true).Limit(50).Find(&pools)
	c.JSON(200, gin.H{"pairs": pools})
}

func (s *SwapService) GetPool(c *gin.Context) {
	tokenA := c.Param("tokenA")
	tokenB := c.Param("tokenB")
	var pool models.Pool
	if err := s.db.Where("(token_a_address = ? AND token_b_address = ?) OR (token_a_address = ? AND token_b_address = ?)",
		tokenA, tokenB, tokenB, tokenA).First(&pool).Error; err != nil {
		c.JSON(404, gin.H{"error": "Pool not found"})
		return
	}
	c.JSON(200, pool)
}

func (s *SwapService) GetPrices(c *gin.Context) {
	// Only symbols TigerSwap actually prices are reported. The response is an
	// array of {symbol, priceUSD, change24h} entries, which is the shape the
	// frontend validator requires.
	symbols := []string{"ETH", "BTC", "USDC", "USDT", "BNB", "SOL", "MATIC", "LINK", "UNI", "AAVE", "DOT", "AVAX", "LTC", "XRP", "DOGE", "TRX", "APT", "ARB", "OP", "NEAR", "INJ", "SUI", "FIL", "ATOM", "XTZ", "ADA", "ALGO", "XLM", "HBAR", "FTM", "EGLD", "GRT", "CRO", "ONE", "LDO", "IMX", "GALA", "ENJ", "BAT", "ZEC", "XMR", "DASH", "ZIL", "ENS", "COMP", "DAI"}

	entries := make([]gin.H, 0, len(symbols))

	if globalPriceAggregator != nil {
		for _, symbol := range symbols {
			pd, err := globalPriceAggregator.GetRealPrice(c.Request.Context(), symbol)
			if err != nil || pd.USD <= 0 {
				continue
			}
			entries = append(entries, gin.H{
				"symbol":    pd.Symbol,
				"priceUSD":  pd.USD,
				"change24h": pd.USDChange24h,
			})
		}
	}

	c.JSON(200, entries)
}

func (s *SwapService) GetPrice(c *gin.Context) {
	symbol := strings.ToUpper(c.Param("symbol"))
	if globalPriceAggregator == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "price aggregator not configured"})
		return
	}

	priceData, err := globalPriceAggregator.GetRealPrice(c.Request.Context(), symbol)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "price unavailable", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"symbol":    priceData.Symbol,
		"priceUSD":  priceData.USD,
		"change24h": priceData.USDChange24h,
	})
}

// GetMarketStats aggregates real platform activity. Volume and fee figures are
// derived from confirmed swap transactions in the window; they are never
// reported as a constant.
func (s *SwapService) GetMarketStats(c *gin.Context) {
	var poolCount int64
	var tokenCount int64
	var chainCount int64
	var userCount int64

	s.db.Model(&models.Pool{}).Where("is_active = ?", true).Count(&poolCount)
	s.db.Model(&models.Token{}).Where("is_active = ?", true).Count(&tokenCount)
	s.db.Model(&models.Blockchain{}).Where("is_active = ?", true).Count(&chainCount)
	s.db.Model(&models.User{}).Count(&userCount)

	var tvl float64
	s.db.Model(&models.Pool{}).Select("COALESCE(SUM(tvl_usd), 0)").Scan(&tvl)

	now := time.Now()
	volume24h, fees24h, swaps24h := s.swapTotals(now.Add(-24 * time.Hour))
	volume7d, _, _ := s.swapTotals(now.Add(-7 * 24 * time.Hour))

	c.JSON(http.StatusOK, gin.H{
		"total_tvl":   tvl,
		"volume_24h":  volume24h,
		"volume_7d":   volume7d,
		"fees_24h":    fees24h,
		"swaps_24h":   swaps24h,
		"user_count":  userCount,
		"pool_count":  poolCount,
		"token_count": tokenCount,
		"chain_count": chainCount,
	})
}

// swapTotals sums confirmed swap notional and the fees they paid since a point
// in time. Fees are taken from the pool the swap belongs to, so a pool with a
// different fee tier is accounted for correctly.
func (s *SwapService) swapTotals(since time.Time) (volume float64, fees float64, count int64) {
	var txs []models.Transaction
	if err := s.db.Where("type = ? AND status = ? AND timestamp >= ?", "swap", "confirmed", since).
		Find(&txs).Error; err != nil {
		return 0, 0, 0
	}

	// Look the pool fee up once per pair rather than per transaction.
	feeByPair := make(map[string]int)
	for _, tx := range txs {
		volume += tx.AmountUSD
		count++

		feeBps, ok := feeByPair[tx.Pair]
		if !ok {
			var pool models.Pool
			feeBps = 30 // the protocol default when a pool is not found
			if err := s.db.Where("pool_address = ? OR (token_a_address = ? AND token_b_address = ?)",
				tx.Pair, tx.FromToken, tx.ToToken).First(&pool).Error; err == nil && pool.Fee > 0 {
				feeBps = pool.Fee
			}
			feeByPair[tx.Pair] = feeBps
		}
		fees += tx.AmountUSD * float64(feeBps) / 10000
	}
	return volume, fees, count
}

// GetRecentTrades returns confirmed swaps from the database.
func (s *SwapService) GetRecentTrades(c *gin.Context) {
	limit := 50
	if raw := c.Query("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 && parsed <= 500 {
			limit = parsed
		}
	}

	var txs []models.Transaction
	s.db.Where("type = ? AND status = ?", "swap", "confirmed").
		Order("timestamp DESC").
		Limit(limit).
		Find(&txs)

	trades := make([]gin.H, 0, len(txs))
	for _, tx := range txs {
		trades = append(trades, gin.H{
			"id":         tx.ID.String(),
			"hash":       tx.Hash,
			"pair":       tx.Pair,
			"from":       tx.FromToken,
			"to":         tx.ToToken,
			"amount":     tx.FromAmount,
			"amount_usd": tx.AmountUSD,
			"status":     tx.Status,
			"timestamp":  tx.Timestamp,
		})
	}

	c.JSON(http.StatusOK, gin.H{"trades": trades})
}

// Global swap executor instance
var globalSwapExecutor *SwapExecutor

func SetSwapExecutor(exec *SwapExecutor) {
	globalSwapExecutor = exec
}

// ExecuteSwap broadcasts a swap the client already signed locally. The server
// receives only the signed payload: it never accepts, holds or logs a private
// key.
func (s *SwapService) ExecuteSwap(c *gin.Context) {
	var req struct {
		ChainID  int64  `json:"chain_id"`
		SignedTx string `json:"signed_tx" binding:"required"`
		TokenIn  string `json:"token_in"`
		TokenOut string `json:"token_out"`
		Amount   string `json:"amount"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	chainID := req.ChainID
	if chainID == 0 {
		chainID = 1
	}

	if globalSwapExecutor == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Swap executor not configured"})
		return
	}

	txHash, err := globalSwapExecutor.BroadcastTransaction(c.Request.Context(), chainID, req.SignedTx)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var userID uuid.UUID
	if parsed, parseErr := uuid.Parse(c.GetString("user_id")); parseErr == nil {
		userID = parsed
	}

	tx := models.Transaction{
		UserID:     userID,
		Hash:       txHash,
		ChainID:    chainID,
		Type:       "swap",
		FromToken:  req.TokenIn,
		ToToken:    req.TokenOut,
		FromAmount: req.Amount,
		Status:     "submitted",
		Timestamp:  time.Now(),
	}
	if err := s.db.Create(&tx).Error; err != nil {
		log.Printf("failed to record swap transaction %s: %v", txHash, err)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"tx_hash": txHash,
		"status":  "submitted",
	})
}

// Global blockchain client for real transactions
var globalBlockchainClient *BlockchainClient

func SetBlockchainClient(client *BlockchainClient) {
	globalBlockchainClient = client
}

// ApproveToken broadcasts a client-signed ERC-20 approval. The server holds no
// key: the client signs the approval locally and posts the signed payload.
func (s *SwapService) ApproveToken(c *gin.Context) {
	var req struct {
		ChainID   int64  `json:"chain_id"`
		SignedTx  string `json:"signed_tx" binding:"required"`
		TokenAddr string `json:"token_address" binding:"required"`
		Spender   string `json:"spender" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	chainID := req.ChainID
	if chainID == 0 {
		chainID = 1
	}

	if globalSwapExecutor == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Swap executor not configured"})
		return
	}

	txHash, err := globalSwapExecutor.BroadcastTransaction(c.Request.Context(), chainID, req.SignedTx)
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

// CreateOrder records a limit/stop order in the database.
func (s *SwapService) CreateOrder(c *gin.Context) {
	var req struct {
		ChainID      int64  `json:"chain_id"`
		OrderType    string `json:"order_type" binding:"required"`
		Side         string `json:"side" binding:"required"`
		TokenIn      string `json:"token_in" binding:"required"`
		TokenOut     string `json:"token_out" binding:"required"`
		AmountIn     string `json:"amount_in" binding:"required"`
		AmountOutMin string `json:"amount_out_min"`
		Price        string `json:"price" binding:"required"`
		ExpiresIn    int    `json:"expires_in"` // seconds
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	orderType := strings.ToLower(req.OrderType)
	switch orderType {
	case "limit", "stop_loss", "take_profit":
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "order_type must be limit, stop_loss or take_profit"})
		return
	}
	side := strings.ToLower(req.Side)
	if side != "buy" && side != "sell" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "side must be buy or sell"})
		return
	}

	userID, err := uuid.Parse(c.GetString("user_id"))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid session"})
		return
	}

	expiresIn := req.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = 7 * 24 * 3600
	}

	order := models.Order{
		UserID:       userID,
		ChainID:      req.ChainID,
		OrderType:    orderType,
		Side:         side,
		TokenIn:      req.TokenIn,
		TokenOut:     req.TokenOut,
		AmountIn:     req.AmountIn,
		AmountOutMin: req.AmountOutMin,
		Price:        req.Price,
		Status:       "pending",
		ExpiresAt:    time.Now().Add(time.Duration(expiresIn) * time.Second),
	}
	if err := s.db.Create(&order).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create order"})
		return
	}

	c.JSON(http.StatusCreated, order)
}

// GetOrders lists the caller's orders.
func (s *SwapService) GetOrders(c *gin.Context) {
	userID := c.GetString("user_id")

	var orders []models.Order
	if err := s.db.Where("user_id = ?", userID).Order("created_at DESC").Limit(200).Find(&orders).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load orders"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"orders": orders, "count": len(orders)})
}

// CancelOrder cancels one of the caller's own pending orders.
func (s *SwapService) CancelOrder(c *gin.Context) {
	userID := c.GetString("user_id")
	orderID := c.Param("id")

	result := s.db.Model(&models.Order{}).
		Where("id = ? AND user_id = ? AND status = ?", orderID, userID, "pending").
		Update("status", "cancelled")
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to cancel order"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found or not cancellable"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "status": "cancelled", "order_id": orderID})
}

// AddLiquidity broadcasts a client-signed add-liquidity transaction.
func (s *SwapService) AddLiquidity(c *gin.Context) {
	var req struct {
		ChainID  int64  `json:"chain_id"`
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

	chainID := req.ChainID
	if chainID == 0 {
		chainID = 1
	}

	if globalSwapExecutor == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Swap executor not configured"})
		return
	}

	txHash, err := globalSwapExecutor.BroadcastTransaction(c.Request.Context(), chainID, req.SignedTx)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "tx_hash": txHash, "status": "submitted"})
}

// RemoveLiquidity broadcasts a client-signed remove-liquidity transaction.
func (s *SwapService) RemoveLiquidity(c *gin.Context) {
	var req struct {
		ChainID   int64  `json:"chain_id"`
		SignedTx  string `json:"signed_tx" binding:"required"`
		PoolAddr  string `json:"pool_address" binding:"required"`
		Liquidity string `json:"liquidity" binding:"required"`
		AmountA   string `json:"amount_a"`
		AmountB   string `json:"amount_b"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	chainID := req.ChainID
	if chainID == 0 {
		chainID = 1
	}

	if globalSwapExecutor == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Swap executor not configured"})
		return
	}

	txHash, err := globalSwapExecutor.BroadcastTransaction(c.Request.Context(), chainID, req.SignedTx)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "tx_hash": txHash, "status": "submitted"})
}

// GetPositions lists the caller's perpetual positions.
func (s *SwapService) GetPositions(c *gin.Context) {
	userID := c.GetString("user_id")

	var positions []models.PerpetualPosition
	if err := s.db.Where("user_id = ?", userID).Order("created_at DESC").Limit(200).Find(&positions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load positions"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"positions": positions, "count": len(positions)})
}

// AuthService handles authentication
type AuthService struct {
	db           *gorm.DB
	jwtSecret    []byte
	accessExpiry time.Duration
	revocations  *revocationStore
}

func NewAuthService(db *gorm.DB) *AuthService {
	// JWT secret must be provided via environment variable
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		panic("JWT_SECRET environment variable is required for security")
	}

	svc := &AuthService{
		db:           db,
		jwtSecret:    []byte(jwtSecret),
		accessExpiry: time.Hour * 24 * 7, // 7 days
		revocations:  newRevocationStore(db),
	}
	svc.revocations.load()
	return svc
}

func (s *AuthService) generateToken(userID uuid.UUID, email, role string) (string, error) {
	now := time.Now()
	expiry := now.Add(s.accessExpiry)
	claims := jwt.MapClaims{
		"jti":     uuid.NewString(),
		"user_id": userID.String(),
		"email":   email,
		"role":    role,
		"exp":     expiry.Unix(),
		"iat":     now.Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtSecret)
}

func (s *AuthService) ValidateToken(tokenString string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.jwtSecret, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		if jti, _ := claims["jti"].(string); jti != "" && s.revocations.isRevoked(jti) {
			return nil, fmt.Errorf("token has been revoked")
		}
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token")
}

func (s *AuthService) Register(c *gin.Context) {
	var req struct {
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required,min=8"`
		Username string `json:"username" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	// The username is stored verbatim and rendered back to users, so it must be
	// restricted to a safe character set. Anything else (markup, control
	// characters, zero-width joiners used to spoof another account) is rejected
	// at the boundary.
	username := strings.TrimSpace(req.Username)
	if len(username) < 3 || len(username) > 32 {
		c.JSON(400, gin.H{"error": "Username must be between 3 and 32 characters"})
		return
	}
	for _, character := range username {
		isAllowed := (character >= 'a' && character <= 'z') ||
			(character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') ||
			character == '_' || character == '-' || character == '.'
		if !isAllowed {
			c.JSON(400, gin.H{"error": "Username may only contain letters, digits, underscore, hyphen and dot"})
			return
		}
	}
	req.Username = username

	// bcrypt ignores every byte after the 72nd, so two distinct long passwords
	// would otherwise authenticate the same account.
	if len(req.Password) > 72 {
		c.JSON(400, gin.H{"error": "Password must be at most 72 bytes"})
		return
	}

	// Check if user exists
	var existingUser models.User
	if err := s.db.Where("email = ?", req.Email).First(&existingUser).Error; err == nil {
		c.JSON(400, gin.H{"error": "Email already registered"})
		return
	}

	// A duplicate username would otherwise surface as a 500 from the unique index.
	var existingUsername models.User
	if err := s.db.Where("username = ?", req.Username).First(&existingUsername).Error; err == nil {
		c.JSON(400, gin.H{"error": "Username already taken"})
		return
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to hash password"})
		return
	}

	user := models.User{
		ID:       uuid.New(),
		Email:    req.Email,
		Username: req.Username,
		Role:     "user",
		IsActive: true,
	}
	user.PasswordHash = string(hashedPassword)

	if err := s.db.Create(&user).Error; err != nil {
		c.JSON(500, gin.H{"error": "Failed to create user"})
		return
	}

	token, _ := s.generateToken(user.ID, user.Email, user.Role)

	c.JSON(201, gin.H{
		"user":  user,
		"token": token,
	})
}

func (s *AuthService) Login(c *gin.Context) {
	var req struct {
		Email    string `json:"email" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	var user models.User
	if err := s.db.Where("email = ?", req.Email).First(&user).Error; err != nil {
		c.JSON(401, gin.H{"error": "Invalid credentials"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		c.JSON(401, gin.H{"error": "Invalid credentials"})
		return
	}

	token, _ := s.generateToken(user.ID, user.Email, user.Role)

	c.JSON(200, gin.H{
		"user":  user,
		"token": token,
	})
}

// RefreshToken issues a new access token from a still-valid bearer token.
func (s *AuthService) RefreshToken(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid authorization header format"})
		return
	}

	claims, err := s.ValidateToken(parts[1])
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
		return
	}

	userID, err := uuid.Parse(fmt.Sprintf("%v", claims["user_id"]))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid session"})
		return
	}

	var user models.User
	if err := s.db.Where("id = ?", userID).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
		return
	}
	if !user.IsActive {
		c.JSON(http.StatusForbidden, gin.H{"error": "Account is not active"})
		return
	}

	token, err := s.generateToken(user.ID, user.Email, user.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to issue token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": token})
}

func (s *AuthService) Logout(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid authorization header format"})
		return
	}

	claims, err := s.ValidateToken(parts[1])
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
		return
	}

	jti, _ := claims["jti"].(string)
	if jti == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Token carries no identifier"})
		return
	}

	var expiresAt time.Time
	if exp, ok := claims["exp"].(float64); ok {
		expiresAt = time.Unix(int64(exp), 0)
	} else {
		expiresAt = time.Now().Add(s.accessExpiry)
	}

	if err := s.revocations.revoke(jti, expiresAt); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to revoke token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *AuthService) GetProfile(c *gin.Context) {
	userID := c.GetString("user_id")
	var user models.User
	if err := s.db.Where("id = ?", userID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}
	c.JSON(http.StatusOK, user)
}

// UpdateProfile updates the caller's mutable profile fields.
func (s *AuthService) UpdateProfile(c *gin.Context) {
	userID := c.GetString("user_id")

	var req struct {
		Username      string `json:"username"`
		WalletAddress string `json:"wallet_address"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updates := map[string]interface{}{}
	if strings.TrimSpace(req.Username) != "" {
		updates["username"] = strings.TrimSpace(req.Username)
	}
	if strings.TrimSpace(req.WalletAddress) != "" {
		if !common.IsHexAddress(req.WalletAddress) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid wallet address"})
			return
		}
		updates["wallet_address"] = strings.ToLower(req.WalletAddress)
	}
	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no updatable fields supplied"})
		return
	}

	if err := s.db.Model(&models.User{}).Where("id = ?", userID).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update profile"})
		return
	}

	var user models.User
	if err := s.db.Where("id = ?", userID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}
	c.JSON(http.StatusOK, user)
}

// GetPortfolio aggregates the caller's confirmed transaction value from the
// database. It does not invent balances: assets are derived from recorded
// transactions only.
func (s *AuthService) GetPortfolio(c *gin.Context) {
	userID := c.GetString("user_id")

	var txs []models.Transaction
	if err := s.db.Where("user_id = ? AND status = ?", userID, "confirmed").Order("timestamp DESC").Limit(500).Find(&txs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load portfolio"})
		return
	}

	assets := make([]gin.H, 0, len(txs))
	var total float64
	for _, tx := range txs {
		total += tx.AmountUSD
		assets = append(assets, gin.H{
			"hash":       tx.Hash,
			"chain_id":   tx.ChainID,
			"symbol":     tx.ToToken,
			"amount":     tx.ToAmount,
			"value_usd":  tx.AmountUSD,
			"created_at": tx.Timestamp,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"total_value": total,
		"assets":      assets,
		"updated_at":  time.Now().UTC(),
	})
}

func (s *AuthService) GetTransactions(c *gin.Context) {
	userID := c.GetString("user_id")

	var txs []models.Transaction
	if err := s.db.Where("user_id = ?", userID).Order("timestamp DESC").Limit(200).Find(&txs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load transactions"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"transactions": txs, "count": len(txs)})
}

// ConnectWallet records the caller's wallet address against their account.
func (s *AuthService) ConnectWallet(c *gin.Context) {
	var req struct {
		Address string `json:"address" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !common.IsHexAddress(req.Address) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid wallet address"})
		return
	}

	userID := c.GetString("user_id")
	address := strings.ToLower(req.Address)
	if err := s.db.Model(&models.User{}).Where("id = ?", userID).Update("wallet_address", address).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to link wallet"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "address": address})
}

// DisconnectWallet clears the wallet address linked to the caller's account.
func (s *AuthService) DisconnectWallet(c *gin.Context) {
	userID := c.GetString("user_id")
	if err := s.db.Model(&models.User{}).Where("id = ?", userID).Update("wallet_address", "").Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to unlink wallet"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// AdminService handles admin operations
type AdminService struct {
	db *gorm.DB
}

func NewAdminService(db *gorm.DB) *AdminService {
	return &AdminService{db: db}
}

// GetDashboard reports real aggregate counts from the database.
func (s *AdminService) GetDashboard(c *gin.Context) {
	var userCount, txCount, tokenCount, poolCount, orderCount, positionCount int64

	s.db.Model(&models.User{}).Count(&userCount)
	s.db.Model(&models.Transaction{}).Count(&txCount)
	s.db.Model(&models.Token{}).Count(&tokenCount)
	s.db.Model(&models.Pool{}).Count(&poolCount)
	s.db.Model(&models.Order{}).Count(&orderCount)
	// "liquidatable" positions are still live exposure, so they are counted
	// alongside open ones; only closed and settled positions drop out.
	s.db.Model(&models.PerpetualPosition{}).Where("status IN ?", []string{"open", "liquidatable"}).Count(&positionCount)

	var tvl float64
	s.db.Model(&models.Pool{}).Select("COALESCE(SUM(tvl_usd), 0)").Scan(&tvl)

	// 24h volume and fees come from confirmed swaps, not a constant.
	var volume24h, fees24h float64
	s.db.Model(&models.Transaction{}).
		Where("type = ? AND status = ? AND timestamp >= ?", "swap", "confirmed", time.Now().Add(-24*time.Hour)).
		Select("COALESCE(SUM(amount_usd), 0)").Scan(&volume24h)
	fees24h = volume24h * float64(DefaultFeeBps) / 10000

	c.JSON(http.StatusOK, gin.H{
		"users":          userCount,
		"transactions":   txCount,
		"tokens":         tokenCount,
		"pools":          poolCount,
		"orders":         orderCount,
		"open_positions": positionCount,
		"tvl":            tvl,
		"volume_24h":     volume24h,
		"fees_24h":       fees24h,
	})
}

// GetChains lists every configured chain, including inactive ones, for the
// admin console. The public endpoint hides inactive chains; operators need the
// full set to manage them.
func (s *AdminService) GetChains(c *gin.Context) {
	limit, offset := paginationParams(c, 200)

	var chains []models.Blockchain
	if err := s.db.Order("chain_id ASC").Limit(limit).Offset(offset).Find(&chains).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load chains"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"chains": chains, "count": len(chains)})
}

// GetTokens lists every configured token, including inactive ones, for the
// admin console.
func (s *AdminService) GetTokens(c *gin.Context) {
	limit, offset := paginationParams(c, 200)

	var tokens []models.Token
	if err := s.db.Order("chain_id ASC, symbol ASC").Limit(limit).Offset(offset).Find(&tokens).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load tokens"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"tokens": tokens, "count": len(tokens)})
}

// GetSettings returns the persisted platform runtime switches.
func (s *AdminService) GetSettings(c *gin.Context) {
	settings, err := s.systemSettings()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load settings"})
		return
	}
	c.JSON(http.StatusOK, settings)
}

func (s *AdminService) GetUsers(c *gin.Context) {
	limit, offset := paginationParams(c, 50)

	var users []models.User
	if err := s.db.Order("created_at DESC").Limit(limit).Offset(offset).Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load users"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"users": users, "count": len(users)})
}

func (s *AdminService) GetUser(c *gin.Context) {
	id := c.Param("id")
	var user models.User
	if err := s.db.Where("id = ?", id).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}
	c.JSON(http.StatusOK, user)
}

// UpdateUser applies admin-editable fields to a user account.
func (s *AdminService) UpdateUser(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		Role       string `json:"role"`
		IsActive   *bool  `json:"is_active"`
		IsVerified *bool  `json:"is_verified"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updates := map[string]interface{}{}
	if req.Role != "" {
		switch req.Role {
		case "user", "admin", "superadmin":
			updates["role"] = req.Role
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid role"})
			return
		}
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}
	if req.IsVerified != nil {
		updates["is_verified"] = *req.IsVerified
	}
	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no updatable fields supplied"})
		return
	}

	result := s.db.Model(&models.User{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	var user models.User
	if err := s.db.Where("id = ?", id).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}
	s.logAdminAction(c, "update_user", "user", id, updates)
	c.JSON(http.StatusOK, user)
}

// DeleteUser deactivates a user account. Records are retained for audit.
func (s *AdminService) DeleteUser(c *gin.Context) {
	id := c.Param("id")

	result := s.db.Model(&models.User{}).Where("id = ?", id).Update("is_active", false)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to deactivate user"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	s.logAdminAction(c, "deactivate_user", "user", id, nil)
	c.JSON(http.StatusOK, gin.H{"success": true, "id": id})
}

func (s *AdminService) CreateChain(c *gin.Context) {
	var chain models.Blockchain
	if err := c.ShouldBindJSON(&chain); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// chain_id is unique; report the collision as a conflict rather than a 500.
	var existing int64
	s.db.Model(&models.Blockchain{}).Where("chain_id = ?", chain.ChainID).Count(&existing)
	if existing > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "a chain with this chain_id already exists"})
		return
	}

	if err := s.db.Create(&chain).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create chain"})
		return
	}
	s.logAdminAction(c, "create_chain", "chain", chain.Name, nil)
	c.JSON(http.StatusCreated, chain)
}

// UpdateChain updates an existing chain configuration.
func (s *AdminService) UpdateChain(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		Name        *string `json:"name"`
		RPCURL      *string `json:"rpc_url"`
		ExplorerURL *string `json:"explorer_url"`
		IsActive    *bool   `json:"is_active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updates := map[string]interface{}{}
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.RPCURL != nil {
		updates["rpc_url"] = *req.RPCURL
	}
	if req.ExplorerURL != nil {
		updates["explorer_url"] = *req.ExplorerURL
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}
	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no updatable fields supplied"})
		return
	}

	result := s.db.Model(&models.Blockchain{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update chain"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Chain not found"})
		return
	}

	var chain models.Blockchain
	if err := s.db.Where("id = ?", id).First(&chain).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Chain not found"})
		return
	}
	s.logAdminAction(c, "update_chain", "chain", id, updates)
	c.JSON(http.StatusOK, chain)
}

// DeleteChain deactivates a chain rather than removing its history.
func (s *AdminService) DeleteChain(c *gin.Context) {
	id := c.Param("id")

	result := s.db.Model(&models.Blockchain{}).Where("id = ?", id).Update("is_active", false)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to deactivate chain"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Chain not found"})
		return
	}

	s.logAdminAction(c, "deactivate_chain", "chain", id, nil)
	c.JSON(http.StatusOK, gin.H{"success": true, "id": id})
}

func (s *AdminService) CreateToken(c *gin.Context) {
	var token models.Token
	if err := c.ShouldBindJSON(&token); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// A token is identified by its contract address on a chain. Rejecting the
	// duplicate here keeps the error actionable instead of surfacing a unique
	// constraint violation as an opaque 500.
	var existing int64
	s.db.Model(&models.Token{}).
		Where("chain_id = ? AND address = ?", token.ChainID, token.Address).
		Count(&existing)
	if existing > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "a token with this address already exists on this chain"})
		return
	}

	if err := s.db.Create(&token).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create token"})
		return
	}
	s.logAdminAction(c, "create_token", "token", token.Symbol, nil)
	c.JSON(http.StatusCreated, token)
}

// UpdateToken applies admin-editable token metadata.
func (s *AdminService) UpdateToken(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		Name       *string  `json:"name"`
		Symbol     *string  `json:"symbol"`
		LogoURL    *string  `json:"logo_url"`
		PriceUSD   *float64 `json:"price_usd"`
		IsActive   *bool    `json:"is_active"`
		IsVerified *bool    `json:"is_verified"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updates := map[string]interface{}{}
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Symbol != nil {
		updates["symbol"] = *req.Symbol
	}
	if req.LogoURL != nil {
		updates["logo_url"] = *req.LogoURL
	}
	if req.PriceUSD != nil {
		updates["price_usd"] = *req.PriceUSD
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}
	if req.IsVerified != nil {
		updates["is_verified"] = *req.IsVerified
	}
	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no updatable fields supplied"})
		return
	}

	result := s.db.Model(&models.Token{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update token"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Token not found"})
		return
	}

	var token models.Token
	if err := s.db.Where("id = ?", id).First(&token).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Token not found"})
		return
	}
	s.logAdminAction(c, "update_token", "token", id, updates)
	c.JSON(http.StatusOK, token)
}

// DeleteToken deactivates a token rather than removing its history.
func (s *AdminService) DeleteToken(c *gin.Context) {
	id := c.Param("id")

	result := s.db.Model(&models.Token{}).Where("id = ?", id).Update("is_active", false)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to deactivate token"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Token not found"})
		return
	}

	s.logAdminAction(c, "deactivate_token", "token", id, nil)
	c.JSON(http.StatusOK, gin.H{"success": true, "id": id})
}

// CreatePool records a new pool definition.
func (s *AdminService) CreatePool(c *gin.Context) {
	var pool models.Pool
	if err := c.ShouldBindJSON(&pool); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := s.db.Create(&pool).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create pool"})
		return
	}
	s.logAdminAction(c, "create_pool", "pool", pool.PoolAddress, nil)
	c.JSON(http.StatusCreated, pool)
}

// UpdatePool applies admin-editable pool fields.
func (s *AdminService) UpdatePool(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		FeeBps   *int     `json:"fee_bps"`
		TVLUSD   *float64 `json:"tvl_usd"`
		IsActive *bool    `json:"is_active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updates := map[string]interface{}{}
	if req.FeeBps != nil {
		if *req.FeeBps < 0 || *req.FeeBps > 10_000 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "fee_bps must be between 0 and 10000"})
			return
		}
		updates["fee"] = *req.FeeBps
	}
	if req.TVLUSD != nil {
		updates["tvl_usd"] = *req.TVLUSD
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}
	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no updatable fields supplied"})
		return
	}

	result := s.db.Model(&models.Pool{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update pool"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Pool not found"})
		return
	}

	var pool models.Pool
	if err := s.db.Where("id = ?", id).First(&pool).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Pool not found"})
		return
	}
	s.logAdminAction(c, "update_pool", "pool", id, updates)
	c.JSON(http.StatusOK, pool)
}

// DeletePool deactivates a pool.
func (s *AdminService) DeletePool(c *gin.Context) {
	id := c.Param("id")

	result := s.db.Model(&models.Pool{}).Where("id = ?", id).Update("is_active", false)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to deactivate pool"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Pool not found"})
		return
	}

	s.logAdminAction(c, "deactivate_pool", "pool", id, nil)
	c.JSON(http.StatusOK, gin.H{"success": true, "id": id})
}

// GetLogs returns the immutable admin audit trail.
func (s *AdminService) GetLogs(c *gin.Context) {
	limit, offset := paginationParams(c, 100)

	var logs []models.AdminLog
	if err := s.db.Order("created_at DESC").Limit(limit).Offset(offset).Find(&logs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load logs"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"logs": logs, "count": len(logs)})
}

// UpdateConfig persists platform-wide runtime switches.
func (s *AdminService) UpdateConfig(c *gin.Context) {
	var req struct {
		TradingEnabled      *bool `json:"trading_enabled"`
		WithdrawalEnabled   *bool `json:"withdrawal_enabled"`
		DepositEnabled      *bool `json:"deposit_enabled"`
		NewUserRegistration *bool `json:"new_user_registration_enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	settings, err := s.systemSettings()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load settings"})
		return
	}

	if req.TradingEnabled != nil {
		settings.TradingEnabled = *req.TradingEnabled
	}
	if req.WithdrawalEnabled != nil {
		settings.WithdrawalEnabled = *req.WithdrawalEnabled
	}
	if req.DepositEnabled != nil {
		settings.DepositEnabled = *req.DepositEnabled
	}
	if req.NewUserRegistration != nil {
		settings.NewUserRegistration = *req.NewUserRegistration
	}

	if err := s.db.Save(settings).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to persist settings"})
		return
	}

	s.logAdminAction(c, "update_config", "system", settings.Key, nil)
	c.JSON(http.StatusOK, settings)
}

// EnableMaintenance switches the platform into maintenance mode.
func (s *AdminService) EnableMaintenance(c *gin.Context) {
	if err := s.setMaintenance(true); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to enable maintenance mode"})
		return
	}
	s.logAdminAction(c, "enable_maintenance", "system", "default", nil)
	c.JSON(http.StatusOK, gin.H{"success": true, "maintenance_mode": true})
}

// DisableMaintenance leaves maintenance mode.
func (s *AdminService) DisableMaintenance(c *gin.Context) {
	if err := s.setMaintenance(false); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to disable maintenance mode"})
		return
	}
	s.logAdminAction(c, "disable_maintenance", "system", "default", nil)
	c.JSON(http.StatusOK, gin.H{"success": true, "maintenance_mode": false})
}

// systemSettings loads the singleton settings row, creating it on first use.
func (s *AdminService) systemSettings() (*models.SystemSettings, error) {
	var settings models.SystemSettings
	err := s.db.Where("key = ?", "default").First(&settings).Error
	if err == nil {
		return &settings, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	settings = models.SystemSettings{
		Key:                 "default",
		TradingEnabled:      true,
		WithdrawalEnabled:   true,
		DepositEnabled:      true,
		NewUserRegistration: true,
	}
	if err := s.db.Create(&settings).Error; err != nil {
		return nil, err
	}
	return &settings, nil
}

func (s *AdminService) setMaintenance(enabled bool) error {
	settings, err := s.systemSettings()
	if err != nil {
		return err
	}
	settings.MaintenanceMode = enabled
	return s.db.Save(settings).Error
}

// logAdminAction records an immutable audit entry for a privileged action.
func (s *AdminService) logAdminAction(c *gin.Context, action, resource, resourceID string, details interface{}) {
	var encoded *string
	if details != nil {
		if raw, err := json.Marshal(details); err == nil {
			s := string(raw)
			encoded = &s
		}
	}

	entry := models.AdminLog{
		AdminID:    adminUUID(c.GetString("user_id")),
		Action:     action,
		EntityType: resource,
		EntityID:   resourceID,
		Details:    encoded,
		IPAddress:  c.ClientIP(),
		UserAgent:  c.Request.UserAgent(),
	}
	if err := s.db.Create(&entry).Error; err != nil {
		log.Printf("failed to write admin audit log: %v", err)
	}
}

// adminUUID parses an admin user id, returning the nil UUID when unavailable.
func adminUUID(raw string) uuid.UUID {
	parsed, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil
	}
	return parsed
}

// paginationParams reads limit/offset query parameters with sane bounds.
func paginationParams(c *gin.Context, defaultLimit int) (int, int) {
	limit := defaultLimit
	if raw := c.Query("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 && parsed <= 500 {
			limit = parsed
		}
	}

	offset := 0
	if raw := c.Query("offset"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed >= 0 {
			offset = parsed
		}
	}
	return limit, offset
}

// WebSocketHub fans out broadcast messages to every connected client.
type WebSocketHub struct {
	clients    map[*websocketConn]bool
	broadcast  chan []byte
	register   chan *websocketConn
	unregister chan *websocketConn
	mu         sync.RWMutex
}

type websocketConn struct {
	send   chan []byte
	client *gin.Context
	conn   *websocket.Conn
}

var wsHub = &WebSocketHub{
	clients:    make(map[*websocketConn]bool),
	broadcast:  make(chan []byte, 256),
	register:   make(chan *websocketConn),
	unregister: make(chan *websocketConn),
}

// upgrader upgrades HTTP requests to WebSocket connections.
var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	// Same-origin policy: the browser sends Origin; reject cross-site upgrades.
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true // non-browser client
		}
		host := r.Host
		u, err := url.Parse(origin)
		if err != nil {
			return false
		}
		return u.Host == host
	},
}

// Run drives the hub until ctx is cancelled.
func (h *WebSocketHub) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			h.mu.Lock()
			for conn := range h.clients {
				close(conn.send)
				delete(h.clients, conn)
			}
			h.mu.Unlock()
			return
		case conn := <-h.register:
			h.mu.Lock()
			h.clients[conn] = true
			h.mu.Unlock()
		case conn := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[conn]; ok {
				delete(h.clients, conn)
				close(conn.send)
			}
			h.mu.Unlock()
		case message := <-h.broadcast:
			h.mu.RLock()
			for conn := range h.clients {
				select {
				case conn.send <- message:
				default:
					// Slow consumer: drop the connection instead of blocking the hub.
					close(conn.send)
					delete(h.clients, conn)
				}
			}
			h.mu.RUnlock()
		}
	}
}

// StartWebSocketHub launches the hub goroutine.
func StartWebSocketHub(ctx context.Context) {
	go wsHub.Run(ctx)
}

// HandleWebSocket upgrades the request and streams messages to/from the client.
func HandleWebSocket(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("WebSocket upgrade error:", err)
		return
	}

	wsConn := &websocketConn{
		send:   make(chan []byte, 256),
		client: c,
		conn:   conn,
	}
	wsHub.register <- wsConn

	go func() {
		defer func() {
			wsHub.unregister <- wsConn
			conn.Close()
		}()

		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				break
			}
			wsHub.broadcast <- message
		}
	}()

	go func() {
		for message := range wsConn.send {
			if err := conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		}
		conn.WriteMessage(websocket.CloseMessage, []byte{})
	}()
}

// BankDetailsHandler returns bank transfer details for fiat deposits
type BankDetails struct {
	BankName      string `json:"bank_name"`
	AccountName   string `json:"account_name"`
	AccountNumber string `json:"account_number"`
	RoutingNumber string `json:"routing_number"`
	IBAN          string `json:"iban"`
	SWIFT         string `json:"swift"`
	Reference     string `json:"reference"`
}

func GetBankDetails(c *gin.Context) {
	// Generate unique reference for this transaction
	reference := fmt.Sprintf("TGR%d", time.Now().Unix())

	bankDetails := BankDetails{
		BankName:      os.Getenv("BANK_NAME"),
		AccountName:   os.Getenv("BANK_ACCOUNT_NAME"),
		AccountNumber: os.Getenv("BANK_ACCOUNT_NUMBER"),
		RoutingNumber: os.Getenv("BANK_ROUTING_NUMBER"),
		IBAN:          os.Getenv("BANK_IBAN"),
		SWIFT:         os.Getenv("BANK_SWIFT"),
		Reference:     reference,
	}

	c.JSON(200, bankDetails)
}
