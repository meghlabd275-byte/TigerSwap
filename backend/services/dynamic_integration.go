package services

import (
	"fmt"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// DynamicIntegrationService provides runtime management for chains, tokens, bridges, farms
type DynamicIntegrationService struct {
	mu sync.RWMutex
	
	// Dynamic chains management
	chains map[string]*DynamicChain
	
	// Dynamic tokens management
	tokens map[string]*DynamicToken
	
	// Dynamic bridges management
	bridges map[string]*DynamicBridge
	
	// Dynamic farms management
	farms map[string]*DynamicFarm
	
	// Statistics
	stats *IntegrationStats
}

// DynamicChain represents a dynamically added blockchain
type DynamicChain struct {
	Key           string    `json:"key"`
	Name          string    `json:"name"`
	Type          string    `json:"type"` // evm, solana, cosmos, ton, aptos, bitcoin
	ChainID       int64     `json:"chain_id"`
	RPCURL        string    `json:"rpc_url"`
	ExplorerURL  string    `json:"explorer_url"`
	Symbol        string    `json:"symbol"`
	Decimals      int       `json:"decimals"`
	NativeToken  string    `json:"native_token"`
	IsActive     bool      `json:"is_active"`
	IsTestnet    bool      `json:"is_testnet"`
	AddedAt      time.Time `json:"added_at"`
	AddedBy      string    `json:"added_by"`
}

// DynamicToken represents a dynamically added token
type DynamicToken struct {
	Key        string    `json:"key"`
	Address   string    `json:"address"`
	ChainKey  string    `json:"chain_key"`
	Name      string    `json:"name"`
	Symbol    string    `json:"symbol"`
	Decimals  int       `json:"decimals"`
	Supply    string    `json:"total_supply"`
	Standard  string    `json:"standard"` // ERC20, SPL, etc.
	IsActive   bool      `json:"is_active"`
	IsVerified bool      `json:"is_verified"`
	AddedAt   time.Time `json:"added_at"`
	AddedBy   string    `json:"added_by"`
}

// DynamicBridge represents a dynamically added bridge
type DynamicBridge struct {
	Key          string    `json:"key"`
	Name         string    `json:"name"`
	SrcChain     string    `json:"src_chain"`
	DstChain     string    `json:"dst_chain"`
	RouterAddr   string    `json:"router_address"`
	MinAmount    string    `json:"min_amount"`
	MaxAmount    string    `json:"max_amount"`
	FeePercent   float64   `json:"fee_percent"`
	FeeFixed     string    `json:"fee_fixed"`
	EstimatedTime string   `json:"estimated_time"`
	IsActive    bool      `json:"is_active"`
	AddedAt     time.Time `json:"added_at"`
}

// DynamicFarm represents a dynamically added farm
type DynamicFarm struct {
	Key             string    `json:"key"`
	Name           string    `json:"name"`
	PoolTokenA     string    `json:"pool_token_a"`
	PoolTokenB     string    `json:"pool_token_b"`
	RewardToken    string    `json:"reward_token"`
	RewardRate     string    `json:"reward_rate"`
	APY           float64   `json:"apy"`
	TotalStaked   string    `json:"total_staked"`
	StartTime     time.Time `json:"start_time"`
	EndTime       time.Time `json:"end_time"`
	IsActive      bool      `json:"is_active"`
	AddedAt       time.Time `json:"added_at"`
}

// IntegrationStats tracks system statistics
type IntegrationStats struct {
	TotalChains      int       `json:"total_chains"`
	TotalTokens     int       `json:"total_tokens"`
	TotalBridges    int       `json:"total_bridges"`
	TotalFarms      int       `json:"total_farms"`
	ActiveChains    int       `json:"active_chains"`
	ActiveTokens    int       `json:"active_tokens"`
	ActiveBridges  int       `json:"active_bridges"`
	ActiveFarms    int       `json:"active_farms"`
	LastUpdated    time.Time `json:"last_updated"`
}

func NewDynamicIntegrationService() *DynamicIntegrationService {
	svc := &DynamicIntegrationService{
		chains: make(map[string]*DynamicChain),
		tokens: make(map[string]*DynamicToken),
		bridges: make(map[string]*DynamicBridge),
		farms: make(map[string]*DynamicFarm),
		stats: &IntegrationStats{},
	}
	
	// Initialize with default TigerSmartChain
	svc.initializeDefaultChains()
	
	return svc
}

func (s *DynamicIntegrationService) initializeDefaultChains() {
	// TigerSmartChain - Main EVM Chain
	tigerChain := &DynamicChain{
		Key:          "tigersmartchain",
		Name:         "TigerSmartChain",
		Type:         "evm",
		ChainID:      8888, // TigerChain mainnet
		RPCURL:       "https://rpc.tigersmartchain.com",
		ExplorerURL:  "https://explorer.tigersmartchain.com",
		Symbol:       "TGR",
		Decimals:     18,
		NativeToken:  "0x0000000000000000000000000000000000000000000",
		IsActive:     true,
		IsTestnet:    false,
		AddedAt:      time.Now(),
		AddedBy:      "system",
	}
	s.chains[tigerChain.Key] = tigerChain
	
	// TigerSmartChain Testnet
	tigerTestnet := &DynamicChain{
		Key:          "tigersmartchain_testnet",
		Name:         "TigerSmartChain Testnet",
		Type:         "evm",
		ChainID:      18888,
		RPCURL:       "https://rpc-testnet.tigersmartchain.com",
		ExplorerURL:  "https://explorer-testnet.tigersmartchain.com",
		Symbol:       "TGR",
		Decimals:     18,
		NativeToken:  "0x0000000000000000000000000000000000000000000",
		IsActive:     true,
		IsTestnet:    true,
		AddedAt:      time.Now(),
		AddedBy:      "system",
	}
	s.chains[tigerTestnet.Key] = tigerTestnet
	
	// Initialize TGR Token
	tgrToken := &DynamicToken{
		Key:       "tgr_tigersmartchain",
		Address:   "0x0000000000000000000000000000000000000000001",
		ChainKey:  "tigersmartchain",
		Name:      "Tiger Coin",
		Symbol:    "TGR",
		Decimals:  18,
		Supply:    "1000000000", // 1 billion supply
		Standard:  "ERC20",
		IsActive:  true,
		IsVerified: true,
		AddedAt:   time.Now(),
		AddedBy:   "system",
	}
	s.tokens[tgrToken.Key] = tgrToken
	
	// Initialize RUSD - Royal Tiger USD Stablecoin
	rusdToken := &DynamicToken{
		Key:       "rusd_tigersmartchain",
		Address:   "0x0000000000000000000000000000000000000000002",
		ChainKey:  "tigersmartchain",
		Name:      "Royal Tiger USD",
		Symbol:    "RUSD",
		Decimals:  18,
		Supply:    "10000000000", // 10 billion supply
		Standard:  "ERC20",
		IsActive:  true,
		IsVerified: true,
		AddedAt:   time.Now(),
		AddedBy:   "system",
	}
	s.tokens[rusdToken.Key] = rusdToken
	
	// Add default pools for Tigerswap
	farmTGRUSDT := &DynamicFarm{
		Key:          "farm_tgr_usdt",
		Name:         "TGR/USDT",
		PoolTokenA:   "0x0000000000000000000000000000000000000001",
		PoolTokenB:   "0x0000000000000000000000000000000000000000003", // USDT
		RewardToken:  "0x0000000000000000000000000000000000000000001",
		RewardRate:   "1000", // per block
		APY:          25.0,
		TotalStaked:  "0",
		StartTime:    time.Now(),
		EndTime:      time.Now().Add(365 * 24 * time.Hour),
		IsActive:     true,
		AddedAt:      time.Now(),
	}
	s.farms[farmTGRUSDT.Key] = farmTGRUSDT
	
	farmRUSDT := &DynamicFarm{
		Key:          "farm_rusd_usdt",
		Name:         "RUSD/USDT",
		PoolTokenA:   "0x0000000000000000000000000000000000000000002",
		PoolTokenB:   "0x0000000000000000000000000000000000000000003",
		RewardToken:  "0x0000000000000000000000000000000000000000001",
		RewardRate:   "500",
		APY:          15.0,
		TotalStaked:  "0",
		StartTime:    time.Now(),
		EndTime:      time.Now().Add(365 * 24 * time.Hour),
		IsActive:     true,
		AddedAt:      time.Now(),
	}
	s.farms[farmRUSDT.Key] = farmRUSDT
	
	s.updateStats()
}

// AddEVMChain dynamically adds a new EVM blockchain
func (s *DynamicIntegrationService) AddEVMChain(key, name, chainId, rpcURL, explorer, symbol, nativeToken string, decimals int, addedBy string) (*DynamicChain, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	chainID, err := parseInt64(chainId)
	if err != nil {
		return nil, fmt.Errorf("invalid chain ID: %v", err)
	}
	
	// Check if exists
	if _, exists := s.chains[key]; exists {
		return nil, fmt.Errorf("chain already exists: %s", key)
	}
	
	chain := &DynamicChain{
		Key:          key,
		Name:         name,
		Type:         "evm",
		ChainID:      chainID,
		RPCURL:       rpcURL,
		ExplorerURL:  explorer,
		Symbol:       symbol,
		Decimals:     decimals,
		NativeToken:  nativeToken,
		IsActive:     true,
		IsTestnet:    false,
		AddedAt:      time.Now(),
		AddedBy:      addedBy,
	}
	
	s.chains[key] = chain
	s.updateStats()
	
	return chain, nil
}

// AddNonEVMChain dynamically adds a new Non-EVM blockchain
func (s *DynamicIntegrationService) AddNonEVMChain(key, name, chainType, rpcURL, explorer, symbol string, decimals int, addedBy string) (*DynamicChain, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if _, exists := s.chains[key]; exists {
		return nil, fmt.Errorf("chain already exists: %s", key)
	}
	
	chain := &DynamicChain{
		Key:         key,
		Name:        name,
		Type:        chainType,
		ChainID:     0, // Non-EVM chains use 0
		RPCURL:      rpcURL,
		ExplorerURL: explorer,
		Symbol:      symbol,
		Decimals:    decimals,
		IsActive:    true,
		IsTestnet:   false,
		AddedAt:     time.Now(),
		AddedBy:     addedBy,
	}
	
	s.chains[key] = chain
	s.updateStats()
	
	return chain, nil
}

// AddToken dynamically adds a new token
func (s *DynamicIntegrationService) AddToken(key, address, chainKey, name, symbol, tokenStandard string, decimals int, supply, addedBy string) (*DynamicToken, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// Verify chain exists
	if _, exists := s.chains[chainKey]; !exists {
		return nil, fmt.Errorf("chain not found: %s", chainKey)
	}
	
	// Check if exists
	if _, exists := s.tokens[key]; exists {
		return nil, fmt.Errorf("token already exists: %s", key)
	}
	
	token := &DynamicToken{
		Key:       key,
		Address:  address,
		ChainKey: chainKey,
		Name:     name,
		Symbol:   symbol,
		Decimals: decimals,
		Supply:   supply,
		Standard: tokenStandard,
		IsActive: true,
		AddedAt:  time.Now(),
		AddedBy:  addedBy,
	}
	
	s.tokens[key] = token
	s.updateStats()
	
	return token, nil
}

// AddBridge dynamically adds a new bridge
func (s *DynamicIntegrationService) AddBridge(key, name, srcChain, dstChain, routerAddr, minAmount, maxAmount string, feePercent float64, feeFixed, estimatedTime string) (*DynamicBridge, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// Verify chains exist
	if _, exists := s.chains[srcChain]; !exists {
		return nil, fmt.Errorf("source chain not found: %s", srcChain)
	}
	if _, exists := s.chains[dstChain]; !exists {
		return nil, fmt.Errorf("destination chain not found: %s", dstChain)
	}
	
	bridge := &DynamicBridge{
		Key:           key,
		Name:          name,
		SrcChain:      srcChain,
		DstChain:      dstChain,
		RouterAddr:    routerAddr,
		MinAmount:     minAmount,
		MaxAmount:     maxAmount,
		FeePercent:    feePercent,
		FeeFixed:      feeFixed,
		EstimatedTime: estimatedTime,
		IsActive:      true,
		AddedAt:       time.Now(),
	}
	
	s.bridges[key] = bridge
	s.updateStats()
	
	return bridge, nil
}

// CreatePool creates a new DEX liquidity pool
func (s *DynamicIntegrationService) CreatePool(key, name, tokenA, tokenB string, feeRate float64) (*DynamicFarm, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// Verify tokens exist
	if _, exists := s.tokens[tokenA]; !exists {
		return nil, fmt.Errorf("token A not found: %s", tokenA)
	}
	if _, exists := s.tokens[tokenB]; !exists {
		return nil, fmt.Errorf("token B not found: %s", tokenB)
	}
	
	pool := &DynamicFarm{
		Key:         key,
		Name:        name,
		PoolTokenA:  tokenA,
		PoolTokenB:  tokenB,
		RewardToken: tokenA,
		RewardRate:  "0",
		APY:        0,
		TotalStaked: "0",
		IsActive:   true,
		AddedAt:    time.Now(),
	}
	
	s.farms[key] = pool
	s.updateStats()
	
	return pool, nil
}

// CreateFarm creates a new farm
func (s *DynamicIntegrationService) CreateFarm(key, name, poolTokenA, poolTokenB, rewardToken, rewardRate string, apy float64, durationDays int) (*DynamicFarm, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	farm := &DynamicFarm{
		Key:          key,
		Name:         name,
		PoolTokenA:   poolTokenA,
		PoolTokenB:   poolTokenB,
		RewardToken:  rewardToken,
		RewardRate:   rewardRate,
		APY:          apy,
		TotalStaked:  "0",
		StartTime:    time.Now(),
		EndTime:      time.Now().Add(time.Duration(durationDays) * 24 * time.Hour),
		IsActive:     true,
		AddedAt:      time.Now(),
	}
	
	s.farms[key] = farm
	s.updateStats()
	
	return farm, nil
}

// ActivateChain activates/deactivates a chain
func (s *DynamicIntegrationService) ActivateChain(key string, active bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	chain, exists := s.chains[key]
	if !exists {
		return fmt.Errorf("chain not found: %s", key)
	}
	
	chain.IsActive = active
	s.updateStats()
	
	return nil
}

// ActivateToken activates/deactivates a token
func (s *DynamicIntegrationService) ActivateToken(key string, active bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	token, exists := s.tokens[key]
	if !exists {
		return fmt.Errorf("token not found: %s", key)
	}
	
	token.IsActive = active
	s.updateStats()
	
	return nil
}

// GetChain returns a chain by key
func (s *DynamicIntegrationService) GetChain(key string) (*DynamicChain, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	if chain, exists := s.chains[key]; exists {
		return chain, nil
	}
	
	return nil, fmt.Errorf("chain not found: %s", key)
}

// GetToken returns a token by key
func (s *DynamicIntegrationService) GetToken(key string) (*DynamicToken, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	if token, exists := s.tokens[key]; exists {
		return token, nil
	}
	
	return nil, fmt.Errorf("token not found: %s", key)
}

// GetAllChains returns all chains with optional filter
func (s *DynamicIntegrationService) GetAllChains(activeOnly bool) []*DynamicChain {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	var result []*DynamicChain
	for _, chain := range s.chains {
		if !activeOnly || chain.IsActive {
			result = append(result, chain)
		}
	}
	
	return result
}

// GetAllTokens returns all tokens with optional filter
func (s *DynamicIntegrationService) GetAllTokens(chainKey string, activeOnly bool) []*DynamicToken {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	var result []*DynamicToken
	for _, token := range s.tokens {
		if (chainKey == "" || token.ChainKey == chainKey) && (!activeOnly || token.IsActive) {
			result = append(result, token)
		}
	}
	
	return result
}

// GetAllBridges returns all bridges
func (s *DynamicIntegrationService) GetAllBridges() []*DynamicBridge {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	var result []*DynamicBridge
	for _, bridge := range s.bridges {
		result = append(result, bridge)
	}
	
	return result
}

// GetAllFarms returns all farms
func (s *DynamicIntegrationService) GetAllFarms(activeOnly bool) []*DynamicFarm {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	var result []*DynamicFarm
	for _, farm := range s.farms {
		if !activeOnly || farm.IsActive {
			result = append(result, farm)
		}
	}
	
	return result
}

// GetStats returns integration statistics
func (s *DynamicIntegrationService) GetStats() *IntegrationStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	return s.stats
}

func (s *DynamicIntegrationService) updateStats() {
	s.stats.TotalChains = len(s.chains)
	s.stats.TotalTokens = len(s.tokens)
	s.stats.TotalBridges = len(s.bridges)
	s.stats.TotalFarms = len(s.farms)
	
	var activeChains, activeTokens, activeBridges, activeFarms int
	
	for _, c := range s.chains {
		if c.IsActive {
			activeChains++
		}
	}
	for _, t := range s.tokens {
		if t.IsActive {
			activeTokens++
		}
	}
	for _, b := range s.bridges {
		if b.IsActive {
			activeBridges++
		}
	}
	for _, f := range s.farms {
		if f.IsActive {
			activeFarms++
		}
	}
	
	s.stats.ActiveChains = activeChains
	s.stats.ActiveTokens = activeTokens
	s.stats.ActiveBridges = activeBridges
	s.stats.ActiveFarms = activeFarms
	s.stats.LastUpdated = time.Now()
}

// SearchChains searches chains by name or symbol
func (s *DynamicIntegrationService) SearchChains(query string) []*DynamicChain {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	query = toLower(query)
	var result []*DynamicChain
	
	for _, chain := range s.chains {
		if strings.Contains(toLower(chain.Name), query) || strings.Contains(toLower(chain.Symbol), query) {
			result = append(result, chain)
		}
	}
	
	return result
}

// SearchTokens searches tokens by name or symbol
func (s *DynamicIntegrationService) SearchTokens(query string) []*DynamicToken {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	query = toLower(query)
	var result []*DynamicToken
	
	for _, token := range s.tokens {
		if strings.Contains(toLower(token.Name), query) || strings.Contains(toLower(token.Symbol), query) {
			result = append(result, token)
		}
	}
	
	return result
}

// DeleteChain removes a chain
func (s *DynamicIntegrationService) DeleteChain(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if _, exists := s.chains[key]; !exists {
		return fmt.Errorf("chain not found: %s", key)
	}
	
	delete(s.chains, key)
	s.updateStats()
	
	return nil
}

// DeleteToken removes a token
func (s *DynamicIntegrationService) DeleteToken(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if _, exists := s.tokens[key]; !exists {
		return fmt.Errorf("token not found: %s", key)
	}
	
	delete(s.tokens, key)
	s.updateStats()
	
	return nil
}

// API Handlers

func (s *DynamicIntegrationService) AddEVMChainHandler(c *gin.Context) {
	var req struct {
		Key          string `json:"key" binding:"required"`
		Name         string `json:"name" binding:"required"`
		ChainID      string `json:"chain_id" binding:"required"`
		RPCURL       string `json:"rpc_url" binding:"required"`
		ExplorerURL  string `json:"explorer_url" binding:"required"`
		Symbol       string `json:"symbol" binding:"required"`
		Decimals     int    `json:"decimals" binding:"required"`
		NativeToken  string `json:"native_token"`
		AddedBy      string `json:"added_by"`
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	
	nativeToken := req.NativeToken
	if nativeToken == "" {
		nativeToken = "0x0000000000000000000000000000000000000000"
	}
	
	chain, err := s.AddEVMChain(req.Key, req.Name, req.ChainID, req.RPCURL, req.ExplorerURL, req.Symbol, nativeToken, req.Decimals, req.AddedBy)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(200, chain)
}

func (s *DynamicIntegrationService) AddTokenHandler(c *gin.Context) {
	var req struct {
		Key       string `json:"key" binding:"required"`
		Address  string `json:"address" binding:"required"`
		ChainKey string `json:"chain_key" binding:"required"`
		Name     string `json:"name" binding:"required"`
		Symbol   string `json:"symbol" binding:"required"`
		Decimals int    `json:"decimals" binding:"required"`
		Standard string `json:"standard" binding:"required"`
		Supply   string `json:"supply"`
		AddedBy  string `json:"added_by"`
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	
	supply := req.Supply
	if supply == "" {
		supply = "0"
	}
	
	token, err := s.AddToken(req.Key, req.Address, req.ChainKey, req.Name, req.Symbol, req.Standard, req.Decimals, supply, req.AddedBy)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(200, token)
}

func (s *DynamicIntegrationService) GetChainsHandler(c *gin.Context) {
	activeOnly := c.Query("active_only") == "true"
	
	chains := s.GetAllChains(activeOnly)
	c.JSON(200, gin.H{
		"chains": chains,
		"count": len(chains),
	})
}

func (s *DynamicIntegrationService) GetTokensHandler(c *gin.Context) {
	chainKey := c.Query("chain_key")
	activeOnly := c.Query("active_only") == "true"
	
	tokens := s.GetAllTokens(chainKey, activeOnly)
	c.JSON(200, gin.H{
		"tokens": tokens,
		"count": len(tokens),
	})
}

func (s *DynamicIntegrationService) GetFarmsHandler(c *gin.Context) {
	activeOnly := c.Query("active_only") == "true"
	
	farms := s.GetAllFarms(activeOnly)
	c.JSON(200, gin.H{
		"farms": farms,
		"count": len(farms),
	})
}

func (s *DynamicIntegrationService) GetStatsHandler(c *gin.Context) {
	c.JSON(200, s.GetStats())
}

func (s *DynamicIntegrationService) CreatePoolHandler(c *gin.Context) {
	var req struct {
		Key     string `json:"key" binding:"required"`
		Name    string `json:"name" binding:"required"`
		TokenA string `json:"token_a" binding:"required"`
		TokenB string `json:"token_b" binding:"required"`
		FeeRate float64 `json:"fee_rate" binding:"required"`
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	
	pool, err := s.CreatePool(req.Key, req.Name, req.TokenA, req.TokenB, req.FeeRate)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(200, pool)
}

func (s *DynamicIntegrationService) CreateFarmHandler(c *gin.Context) {
	var req struct {
		Key          string `json:"key" binding:"required"`
		Name         string `json:"name" binding:"required"`
		PoolTokenA   string `json:"pool_token_a" binding:"required"`
		PoolTokenB   string `json:"pool_token_b" binding:"required"`
		RewardToken  string `json:"reward_token" binding:"required"`
		RewardRate   string `json:"reward_rate" binding:"required"`
		APY          float64 `json:"apy" binding:"required"`
		DurationDays int    `json:"duration_days" binding:"required"`
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	
	farm, err := s.CreateFarm(req.Key, req.Name, req.PoolTokenA, req.PoolTokenB, req.RewardToken, req.RewardRate, req.APY, req.DurationDays)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(200, farm)
}

func (s *DynamicIntegrationService) AddBridgeHandler(c *gin.Context) {
	var req struct {
		Key           string  `json:"key" binding:"required"`
		Name          string  `json:"name" binding:"required"`
		SrcChain      string  `json:"src_chain" binding:"required"`
		DstChain      string  `json:"dst_chain" binding:"required"`
		RouterAddress string  `json:"router_address" binding:"required"`
		MinAmount     string  `json:"min_amount" binding:"required"`
		MaxAmount     string  `json:"max_amount" binding:"required"`
		FeePercent    float64 `json:"fee_percent" binding:"required"`
		FeeFixed      string  `json:"fee_fixed" binding:"required"`
		EstimatedTime string  `json:"estimated_time" binding:"required"`
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	
	bridge, err := s.AddBridge(req.Key, req.Name, req.SrcChain, req.DstChain, req.RouterAddress, req.MinAmount, req.MaxAmount, req.FeePercent, req.FeeFixed, req.EstimatedTime)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(200, bridge)
}

func (s *DynamicIntegrationService) SearchHandler(c *gin.Context) {
	query := c.Query("q")
	Type := c.Query("type")
	
	if Type == "tokens" {
		tokens := s.SearchTokens(query)
		c.JSON(200, gin.H{"tokens": tokens, "count": len(tokens)})
	} else {
		chains := s.SearchChains(query)
		c.JSON(200, gin.H{"chains": chains, "count": len(chains)})
	}
}

// Helper functions
import "strings"

func parseInt64(s string) (int64, error) {
	var n int64
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
}

func toLower(s string) string {
	return strings.ToLower(s)
}
