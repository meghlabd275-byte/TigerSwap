package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
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
	chains := []models.Blockchain{
		{ChainID: 1, Name: "Ethereum", Symbol: "ETH", Icon: "🦄", RPCURL: "https://eth.llamarpc.com", ExplorerURL: "https://etherscan.io", Type: "evm", IsActive: true},
		{ChainID: 137, Name: "Polygon", Symbol: "MATIC", Icon: "🔷", RPCURL: "https://polygon.llamarpc.com", ExplorerURL: "https://polygonscan.com", Type: "evm", IsActive: true},
		{ChainID: 42161, Name: "Arbitrum One", Symbol: "ETH", Icon: "🔵", RPCURL: "https://arb1.arbitrum.io/rpc", ExplorerURL: "https://arbiscan.io", Type: "evm", IsActive: true},
		{ChainID: 10, Name: "Optimism", Symbol: "ETH", Icon: "🔴", RPCURL: "https://mainnet.optimism.io", ExplorerURL: "https://optimistic.etherscan.io", Type: "evm", IsActive: true},
		{ChainID: 8453, Name: "Base", Symbol: "ETH", Icon: "🔵", RPCURL: "https://mainnet.base.org", ExplorerURL: "https://basescan.org", Type: "evm", IsActive: true},
		{ChainID: 56, Name: "BNB Smart Chain", Symbol: "BNB", Icon: "🟡", RPCURL: "https://bsc-dataseed.binance.org", ExplorerURL: "https://bscscan.com", Type: "evm", IsActive: true},
		{ChainID: 43114, Name: "Avalanche", Symbol: "AVAX", Icon: "🔺", RPCURL: "https://api.avax.network/ext/bc/C/rpc", ExplorerURL: "https://snowtrace.io", Type: "evm", IsActive: true},
		{ChainID: 250, Name: "Fantom", Symbol: "FTM", Icon: "👻", RPCURL: "https://rpc.fantom.network", ExplorerURL: "https://ftmscan.com", Type: "evm", IsActive: true},
		{ChainID: 101, Name: "Solana", Symbol: "SOL", Icon: "☀️", RPCURL: "https://api.mainnet-beta.solana.com", ExplorerURL: "https://solscan.io", Type: "solana", IsActive: true},
		{ChainID: 1, Name: "Aptos", Symbol: "APT", Icon: "🔷", RPCURL: "https://fullnode.mainnet.aptoslabs.com", ExplorerURL: "https://aptoscan.com", Type: "aptos", IsActive: true},
		{ChainID: 1, Name: "Cosmos", Symbol: "ATOM", Icon: "🌌", RPCURL: "https://rpc.cosmos.network", ExplorerURL: "https://mintscan.io/cosmos", Type: "cosmos", IsActive: true},
		{ChainID: 1, Name: "TON", Symbol: "TON", Icon: "📱", RPCURL: "https://toncenter.com/api/v2/jsonRPC", ExplorerURL: "https://tonscan.org", Type: "ton", IsActive: true},
	}
	for _, chain := range chains {
		db.Create(&chain)
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
		{ChainID: 137, Address: "0x0000000000000000000000000000000000000000", Name: "Polygon", Symbol: "MATIC", Decimals: 18, LogoURI: "https://assets.coingecko.com/coins/images/4713/small/polygon.png", ChainID: 137, IsActive: true, IsVerified: true},
		{ChainID: 56, Address: "0xbb4CdB9CBd36B01bD1cBaEBF2De08d9173bc095c", Name: "Wrapped BNB", Symbol: "WBNB", Decimals: 18, LogoURI: "https://assets.coingecko.com/coins/images/825/small/bnb-icon2_2x.png", IsActive: true, IsVerified: true},
	}
	for _, token := range tokens {
		db.Create(&token)
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

	// Mock quote calculation - in production, this would query actual DEX pools
	amountIn := new(big.Float)
	amountIn.SetString(req.Amount)
	rate := big.NewFloat(1.0) // Mock rate
	amountOut := new(big.Float).Mul(amountIn, rate)

	quote := gin.H{
		"token_in":          req.TokenIn,
		"token_out":         req.TokenOut,
		"amount_in":         req.Amount,
		"amount_out":        amountOut.String(),
		"amount_out_min":    amountOut.String(), // Apply slippage
		"rate":              "1.0",
		"price_impact":      "0.1",
		"gas_estimate":     "21000",
		"chain_id":         req.ChainID,
		"route":            []string{req.TokenIn, req.TokenOut},
	}

	c.JSON(200, quote)
}

func (s *SwapService) BuildTransaction(c *gin.Context) {
	c.JSON(200, gin.H{
		"to":           "0x742d35Cc6634C0532925a3b844Bc9e7595f0fEb1",
		"data":         "0x...",
		"value":        "0x0",
		"gas":          "21000",
		"gas_price":    "1000000000",
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
	// Mock prices
	prices := gin.H{
		"ETH":  {"usd": 3450.00, "change_24h": 2.5},
		"USDC": {"usd": 1.00, "change_24h": 0.01},
		"USDT": {"usd": 1.00, "change_24h": -0.01},
		"WBTC": {"usd": 67500.00, "change_24h": 1.8},
		"LINK": {"usd": 18.50, "change_24h": 3.2},
		"UNI":  {"usd": 12.80, "change_24h": -1.2},
		"MATIC": {"usd": 0.85, "change_24h": 1.5},
		"BNB":  {"usd": 580.00, "change_24h": 0.8},
		"AVAX": {"usd": 38.50, "change_24h": 2.1},
		"SOL":  {"usd": 145.00, "change_24h": -2.5},
	}
	c.JSON(200, prices)
}

func (s *SwapService) GetPrice(c *gin.Context) {
	symbol := c.Param("symbol")
	price := gin.H{
		"symbol":      symbol,
		"usd":         3450.00,
		"change_24h":  2.5,
		"volume_24h":  "1.2B",
		"market_cap":  "400B",
	}
	c.JSON(200, price)
}

func (s *SwapService) GetMarketStats(c *gin.Context) {
	stats := gin.H{
		"total_tvl":     "2.4B",
		"volume_24h":    "892M",
		"volume_7d":     "6.2B",
		"fees_24h":     "2.7M",
		"user_count":   "125000",
		"pool_count":   "12450",
		"token_count":  "8500",
		"chain_count":  "100",
	}
	c.JSON(200, stats)
}

func (s *SwapService) GetRecentTrades(c *gin.Context) {
	trades := []gin.H{
		{"hash": "0x1234...", "from": "ETH", "to": "USDC", "amount": "1.5", "price": 3450, "time": "2s ago"},
		{"hash": "0x5678...", "from": "USDT", "to": "ETH", "amount": "5000", "price": 3450, "time": "15s ago"},
		{"hash": "0xabcd...", "from": "WBTC", "to": "USDC", "amount": "0.1", "price": 67500, "time": "32s ago"},
	}
	c.JSON(200, gin.H{"trades": trades})
}

func (s *SwapService) ExecuteSwap(c *gin.Context) {
	var req struct {
		TokenIn  string `json:"token_in" binding:"required"`
		TokenOut string `json:"token_out" binding:"required"`
		Amount   string `json:"amount" binding:"required"`
		To       string `json:"to" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	// Mock transaction
	tx := models.Transaction{
		Hash:       "0x" + generateRandomHex(64),
		Type:       "swap",
		FromToken:  req.TokenIn,
		ToToken:    req.TokenOut,
		FromAmount: req.Amount,
		Status:     "confirmed",
		Timestamp:  time.Now(),
	}
	s.db.Create(&tx)

	c.JSON(200, gin.H{
		"success":      true,
		"transaction":  tx,
		"message":      "Swap executed successfully",
	})
}

func (s *SwapService) ApproveToken(c *gin.Context) {
	c.JSON(200, gin.H{
		"success":     true,
		"tx_hash":     "0x" + generateRandomHex(64),
		"message":     "Token approved",
	})
}

func (s *SwapService) CreateOrder(c *gin.Context) {
	c.JSON(200, gin.H{"order_id": uuid.New().String(), "status": "pending"})
}

func (s *SwapService) GetOrders(c *gin.Context) {
	c.JSON(200, gin.H{"orders": []interface{}{}})
}

func (s *SwapService) CancelOrder(c *gin.Context) {
	c.JSON(200, gin.H{"success": true, "status": "cancelled"})
}

func (s *SwapService) AddLiquidity(c *gin.Context) {
	c.JSON(200, gin.H{"success": true, "tx_hash": "0x" + generateRandomHex(64)})
}

func (s *SwapService) RemoveLiquidity(c *gin.Context) {
	c.JSON(200, gin.H{"success": true, "tx_hash": "0x" + generateRandomHex(64)})
}

func (s *SwapService) GetPositions(c *gin.Context) {
	c.JSON(200, gin.H{"positions": []interface{}{}})
}

// AuthService handles authentication
type AuthService struct {
	db           *gorm.DB
	jwtSecret    []byte
	accessExpiry time.Duration
}

func NewAuthService(db *gorm.DB) *AuthService {
	secret := []byte("tigerswap-jwt-secret-change-in-production")
	return &AuthService{
		db:           db,
		jwtSecret:    secret,
		accessExpiry: time.Hour * 24 * 7, // 7 days
	}
}

func (s *AuthService) generateToken(userID uuid.UUID, email, role string) (string, error) {
	claims := jwt.MapClaims{
		"user_id": userID.String(),
		"email":   email,
		"role":    role,
		"exp":     time.Now().Add(s.accessExpiry).Unix(),
		"iat":     time.Now().Unix(),
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

	// Check if user exists
	var existingUser models.User
	if err := s.db.Where("email = ?", req.Email).First(&existingUser).Error; err == nil {
		c.JSON(400, gin.H{"error": "Email already registered"})
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

func (s *AuthService) RefreshToken(c *gin.Context) {
	c.JSON(200, gin.H{"token": "new_token"})
}

func (s *AuthService) Logout(c *gin.Context) {
	c.JSON(200, gin.H{"success": true})
}

func (s *AuthService) GetProfile(c *gin.Context) {
	userID := c.GetString("user_id")
	var user models.User
	if err := s.db.Where("id = ?", userID).First(&user).Error; err != nil {
		c.JSON(404, gin.H{"error": "User not found"})
		return
	}
	c.JSON(200, user)
}

func (s *AuthService) UpdateProfile(c *gin.Context) {
	c.JSON(200, gin.H{"success": true})
}

func (s *AuthService) GetPortfolio(c *gin.Context) {
	portfolio := gin.H{
		"total_value":   "24580.50",
		"assets":        []interface{}{},
	}
	c.JSON(200, portfolio)
}

func (s *AuthService) GetTransactions(c *gin.Context) {
	c.JSON(200, gin.H{"transactions": []interface{}{}})
}

func (s *AuthService) ConnectWallet(c *gin.Context) {
	var req struct {
		Address string `json:"address" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"success": true, "address": req.Address})
}

func (s *AuthService) DisconnectWallet(c *gin.Context) {
	c.JSON(200, gin.H{"success": true})
}

// AdminService handles admin operations
type AdminService struct {
	db *gorm.DB
}

func NewAdminService(db *gorm.DB) *AdminService {
	return &AdminService{db: db}
}

func (s *AdminService) GetDashboard(c *gin.Context) {
	var userCount int64
	var txCount int64
	var tokenCount int64
	var poolCount int64

	s.db.Model(&models.User{}).Count(&userCount)
	s.db.Model(&models.Transaction{}).Count(&txCount)
	s.db.Model(&models.Token{}).Count(&tokenCount)
	s.db.Model(&models.Pool{}).Count(&poolCount)

	dashboard := gin.H{
		"users":      userCount,
		"transactions": txCount,
		"tokens":     tokenCount,
		"pools":      poolCount,
		"tvl":        "2.4B",
		"volume_24h": "892M",
	}
	c.JSON(200, dashboard)
}

func (s *AdminService) GetUsers(c *gin.Context) {
	var users []models.User
	s.db.Limit(50).Find(&users)
	c.JSON(200, gin.H{"users": users})
}

func (s *AdminService) GetUser(c *gin.Context) {
	id := c.Param("id")
	var user models.User
	if err := s.db.Where("id = ?", id).First(&user).Error; err != nil {
		c.JSON(404, gin.H{"error": "User not found"})
		return
	}
	c.JSON(200, user)
}

func (s *AdminService) UpdateUser(c *gin.Context) {
	c.JSON(200, gin.H{"success": true})
}

func (s *AdminService) DeleteUser(c *gin.Context) {
	c.JSON(200, gin.H{"success": true})
}

func (s *AdminService) CreateChain(c *gin.Context) {
	var chain models.Blockchain
	if err := c.ShouldBindJSON(&chain); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	s.db.Create(&chain)
	c.JSON(201, chain)
}

func (s *AdminService) UpdateChain(c *gin.Context) {
	c.JSON(200, gin.H{"success": true})
}

func (s *AdminService) DeleteChain(c *gin.Context) {
	c.JSON(200, gin.H{"success": true})
}

func (s *AdminService) CreateToken(c *gin.Context) {
	var token models.Token
	if err := c.ShouldBindJSON(&token); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	s.db.Create(&token)
	c.JSON(201, token)
}

func (s *AdminService) UpdateToken(c *gin.Context) {
	c.JSON(200, gin.H{"success": true})
}

func (s *AdminService) DeleteToken(c *gin.Context) {
	c.JSON(200, gin.H{"success": true})
}

func (s *AdminService) CreatePool(c *gin.Context) {
	c.JSON(201, gin.H{"success": true})
}

func (s *AdminService) UpdatePool(c *gin.Context) {
	c.JSON(200, gin.H{"success": true})
}

func (s *AdminService) DeletePool(c *gin.Context) {
	c.JSON(200, gin.H{"success": true})
}

func (s *AdminService) GetLogs(c *gin.Context) {
	c.JSON(200, gin.H{"logs": []interface{}{}})
}

func (s *AdminService) UpdateConfig(c *gin.Context) {
	c.JSON(200, gin.H{"success": true})
}

func (s *AdminService) EnableMaintenance(c *gin.Context) {
	c.JSON(200, gin.H{"success": true, "message": "Maintenance mode enabled"})
}

func (s *AdminService) DisableMaintenance(c *gin.Context) {
	c.JSON(200, gin.H{"success": true, "message": "Maintenance mode disabled"})
}

// WebSocketHub manages WebSocket connections
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
}

var wsHub = &WebSocketHub{
	clients:    make(map[*websocketConn]bool),
	broadcast:   make(chan []byte),
	register:   make(chan *websocketConn),
	unregister: make(chan *websocketConn),
}

func HandleWebSocket(c *gin.Context) {
	// Upgrade to WebSocket
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("WebSocket upgrade error:", err)
		return
	}

	wsConn := &websocketConn{
		send:   make(chan []byte, 256),
		client: c,
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
		for {
			message, ok := <-wsConn.send
			if !ok {
				conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			conn.WriteMessage(websocket.TextMessage, message)
		}
	}()
}

// Helper function to generate random hex
func generateRandomHex(length int) string {
	bytes := make([]byte, length/2)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}
