package services

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"tigerswap/backend/models"
)

// Sentinels for dynamic integration lookups.
var (
	errChainNotFound      = errors.New("chain not found")
	errTokenNotFound      = errors.New("token not found")
	errTokenAlreadyExists = errors.New("token already exists")
	errChainKeyRequired   = errors.New("chain key is required")
	errTokenKeyRequired   = errors.New("token key is required")
	errBridgeKeyRequired  = errors.New("bridge key is required")
	errFarmKeyRequired    = errors.New("farm key is required")
	errDuplicateKey       = errors.New("a registration with that key already exists")
	errInvalidRPCURL      = errors.New("rpc_url must be an absolute http(s) URL")
	errUnsupportedChain   = errors.New("unsupported chain type")
	errPositiveDuration   = errors.New("duration_days must be positive")
)

// DynamicIntegrationService manages the registry of chains, tokens, bridges and
// farms an operator adds at runtime, without a redeploy.
//
// Every entry is persisted: the registry is backed by the database, so an
// operator's work survives a restart and is shared by every API replica. There
// are no seeded defaults — a fresh install lists nothing until an operator
// registers a real chain, token, bridge or farm. A fabricated chain with a
// placeholder address would be indistinguishable from a real one to every
// caller, so none is invented here.
type DynamicIntegrationService struct {
	db *gorm.DB
}

// NewDynamicIntegrationService returns a registry backed by db.
func NewDynamicIntegrationService(db *gorm.DB) *DynamicIntegrationService {
	return &DynamicIntegrationService{db: db}
}

// DynamicChain is the wire shape of a registered chain.
type DynamicChain struct {
	Key         string    `json:"key"`
	Name        string    `json:"name"`
	Type        string    `json:"type"`
	ChainID     int64     `json:"chain_id"`
	RPCURL      string    `json:"rpc_url"`
	ExplorerURL string    `json:"explorer_url"`
	Symbol      string    `json:"symbol"`
	Decimals    int       `json:"decimals"`
	NativeToken string    `json:"native_token"`
	IsActive    bool      `json:"is_active"`
	IsTestnet   bool      `json:"is_testnet"`
	AddedAt     time.Time `json:"added_at"`
	AddedBy     string    `json:"added_by"`
}

// DynamicToken is the wire shape of a registered token.
type DynamicToken struct {
	Key        string    `json:"key"`
	Address    string    `json:"address"`
	ChainKey   string    `json:"chain_key"`
	Name       string    `json:"name"`
	Symbol     string    `json:"symbol"`
	Decimals   int       `json:"decimals"`
	Supply     string    `json:"total_supply"`
	Standard   string    `json:"standard"`
	IsActive   bool      `json:"is_active"`
	IsVerified bool      `json:"is_verified"`
	AddedAt    time.Time `json:"added_at"`
	AddedBy    string    `json:"added_by"`
}

// DynamicBridge is the wire shape of a registered bridge route.
type DynamicBridge struct {
	Key           string    `json:"key"`
	Name          string    `json:"name"`
	SrcChain      string    `json:"src_chain"`
	DstChain      string    `json:"dst_chain"`
	RouterAddr    string    `json:"router_address"`
	MinAmount     string    `json:"min_amount"`
	MaxAmount     string    `json:"max_amount"`
	FeePercent    float64   `json:"fee_percent"`
	FeeFixed      string    `json:"fee_fixed"`
	EstimatedTime string    `json:"estimated_time"`
	IsActive      bool      `json:"is_active"`
	AddedAt       time.Time `json:"added_at"`
}

// DynamicFarm is the wire shape of a registered pool or farm.
type DynamicFarm struct {
	Key         string    `json:"key"`
	Name        string    `json:"name"`
	PoolTokenA  string    `json:"pool_token_a"`
	PoolTokenB  string    `json:"pool_token_b"`
	RewardToken string    `json:"reward_token"`
	RewardRate  string    `json:"reward_rate"`
	APY         float64   `json:"apy"`
	TotalStaked string    `json:"total_staked"`
	StartTime   time.Time `json:"start_time"`
	EndTime     time.Time `json:"end_time"`
	IsActive    bool      `json:"is_active"`
	AddedAt     time.Time `json:"added_at"`
}

// IntegrationStats tracks registry counts.
type IntegrationStats struct {
	TotalChains   int       `json:"total_chains"`
	TotalTokens   int       `json:"total_tokens"`
	TotalBridges  int       `json:"total_bridges"`
	TotalFarms    int       `json:"total_farms"`
	ActiveChains  int       `json:"active_chains"`
	ActiveTokens  int       `json:"active_tokens"`
	ActiveBridges int       `json:"active_bridges"`
	ActiveFarms   int       `json:"active_farms"`
	LastUpdated   time.Time `json:"last_updated"`
}

// validChainTypes is the set of chain families the registry understands. An
// unknown type would be routed to a client that cannot speak it.
var validChainTypes = map[string]bool{
	"evm": true, "solana": true, "cosmos": true, "ton": true,
	"aptos": true, "sui": true, "bitcoin": true,
}

func validChainType(t string) bool {
	return validChainTypes[strings.ToLower(strings.TrimSpace(t))]
}

func toChain(m *models.IntegrationChain) *DynamicChain {
	return &DynamicChain{
		Key: m.Key, Name: m.Name, Type: m.Type, ChainID: m.ChainID,
		RPCURL: m.RPCURL, ExplorerURL: m.ExplorerURL, Symbol: m.Symbol,
		Decimals: m.Decimals, NativeToken: m.NativeToken, IsActive: m.IsActive,
		IsTestnet: m.IsTestnet, AddedAt: m.AddedAt, AddedBy: m.AddedBy,
	}
}

func toToken(m *models.IntegrationToken) *DynamicToken {
	return &DynamicToken{
		Key: m.Key, Address: m.Address, ChainKey: m.ChainKey, Name: m.Name,
		Symbol: m.Symbol, Decimals: m.Decimals, Supply: m.Supply,
		Standard: m.Standard, IsActive: m.IsActive, IsVerified: m.IsVerified,
		AddedAt: m.AddedAt, AddedBy: m.AddedBy,
	}
}

func toBridge(m *models.IntegrationBridge) *DynamicBridge {
	return &DynamicBridge{
		Key: m.Key, Name: m.Name, SrcChain: m.SrcChain, DstChain: m.DstChain,
		RouterAddr: m.RouterAddr, MinAmount: m.MinAmount, MaxAmount: m.MaxAmount,
		FeePercent: m.FeePercent, FeeFixed: m.FeeFixed,
		EstimatedTime: m.EstimatedTime, IsActive: m.IsActive, AddedAt: m.AddedAt,
	}
}

func toFarm(m *models.IntegrationFarm) *DynamicFarm {
	return &DynamicFarm{
		Key: m.Key, Name: m.Name, PoolTokenA: m.PoolTokenA, PoolTokenB: m.PoolTokenB,
		RewardToken: m.RewardToken, RewardRate: m.RewardRate, APY: m.APY,
		TotalStaked: m.TotalStaked, StartTime: m.StartTime, EndTime: m.EndTime,
		IsActive: m.IsActive, AddedAt: m.AddedAt,
	}
}

// validateRPCURL rejects a non-absolute or non-http(s) URL: an operator typo
// must fail at registration rather than surface as a failed call at trade time.
func validateRPCURL(raw string) error {
	u := strings.TrimSpace(raw)
	if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
		return errInvalidRPCURL
	}
	if len(u) <= len("https://") {
		return errInvalidRPCURL
	}
	return nil
}

// AddEVMChain registers an EVM chain.
func (s *DynamicIntegrationService) AddEVMChain(key, name, chainID, rpcURL, explorer, symbol, nativeToken string, decimals int, addedBy string) (*DynamicChain, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, errChainKeyRequired
	}
	if err := validateRPCURL(rpcURL); err != nil {
		return nil, err
	}
	parsedID, err := parseInt64(chainID)
	if err != nil {
		return nil, fmt.Errorf("invalid chain ID: %v", err)
	}

	row := &models.IntegrationChain{
		Key: key, Name: name, Type: "evm", ChainID: parsedID,
		RPCURL: rpcURL, ExplorerURL: explorer, Symbol: symbol, Decimals: decimals,
		NativeToken: nativeToken, IsActive: true, AddedAt: time.Now(), AddedBy: addedBy,
	}
	if err := s.db.Create(row).Error; err != nil {
		if isDuplicateKey(err) {
			return nil, fmt.Errorf("%w: %s", errDuplicateKey, key)
		}
		return nil, err
	}
	return toChain(row), nil
}

// AddNonEVMChain registers a non-EVM chain (Solana, Cosmos, TON, Aptos,
// Bitcoin, ...), the only path for a chain whose addresses are not 20-byte hex.
func (s *DynamicIntegrationService) AddNonEVMChain(key, name, chainType, rpcURL, explorer, symbol string, decimals int, addedBy string) (*DynamicChain, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, errChainKeyRequired
	}
	if !validChainType(chainType) {
		return nil, fmt.Errorf("%w: %s", errUnsupportedChain, chainType)
	}
	if err := validateRPCURL(rpcURL); err != nil {
		return nil, err
	}

	row := &models.IntegrationChain{
		Key: key, Name: name, Type: strings.ToLower(strings.TrimSpace(chainType)),
		RPCURL: rpcURL, ExplorerURL: explorer, Symbol: symbol, Decimals: decimals,
		IsActive: true, AddedAt: time.Now(), AddedBy: addedBy,
	}
	if err := s.db.Create(row).Error; err != nil {
		if isDuplicateKey(err) {
			return nil, fmt.Errorf("%w: %s", errDuplicateKey, key)
		}
		return nil, err
	}
	return toChain(row), nil
}

// AddToken registers a token on an already-registered chain.
func (s *DynamicIntegrationService) AddToken(key, address, chainKey, name, symbol, tokenStandard string, decimals int, supply, addedBy string) (*DynamicToken, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, errTokenKeyRequired
	}
	if _, err := s.GetChain(chainKey); err != nil {
		return nil, fmt.Errorf("%w: %s", errChainNotFound, chainKey)
	}

	row := &models.IntegrationToken{
		Key: key, Address: address, ChainKey: chainKey, Name: name, Symbol: symbol,
		Decimals: decimals, Supply: supply, Standard: tokenStandard,
		IsActive: true, AddedAt: time.Now(), AddedBy: addedBy,
	}
	if err := s.db.Create(row).Error; err != nil {
		if isDuplicateKey(err) {
			return nil, fmt.Errorf("%w: %s", errTokenAlreadyExists, key)
		}
		return nil, err
	}
	return toToken(row), nil
}

// AddBridge registers a bridge route between two registered chains.
func (s *DynamicIntegrationService) AddBridge(key, name, srcChain, dstChain, routerAddr, minAmount, maxAmount string, feePercent float64, feeFixed, estimatedTime string) (*DynamicBridge, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, errBridgeKeyRequired
	}
	if _, err := s.GetChain(srcChain); err != nil {
		return nil, fmt.Errorf("%w: source %s", errChainNotFound, srcChain)
	}
	if _, err := s.GetChain(dstChain); err != nil {
		return nil, fmt.Errorf("%w: destination %s", errChainNotFound, dstChain)
	}

	row := &models.IntegrationBridge{
		Key: key, Name: name, SrcChain: srcChain, DstChain: dstChain,
		RouterAddr: routerAddr, MinAmount: minAmount, MaxAmount: maxAmount,
		FeePercent: feePercent, FeeFixed: feeFixed, EstimatedTime: estimatedTime,
		IsActive: true, AddedAt: time.Now(),
	}
	if err := s.db.Create(row).Error; err != nil {
		if isDuplicateKey(err) {
			return nil, fmt.Errorf("%w: %s", errDuplicateKey, key)
		}
		return nil, err
	}
	return toBridge(row), nil
}

// CreatePool registers a pool over two registered tokens. A pool is a farm with
// no reward stream yet.
func (s *DynamicIntegrationService) CreatePool(key, name, tokenA, tokenB string, feeRate float64) (*DynamicFarm, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, errFarmKeyRequired
	}
	if _, err := s.GetToken(tokenA); err != nil {
		return nil, fmt.Errorf("%w: token A %s", errTokenNotFound, tokenA)
	}
	if _, err := s.GetToken(tokenB); err != nil {
		return nil, fmt.Errorf("%w: token B %s", errTokenNotFound, tokenB)
	}

	row := &models.IntegrationFarm{
		Key: key, Name: name, PoolTokenA: tokenA, PoolTokenB: tokenB,
		RewardToken: tokenA, RewardRate: "0", TotalStaked: "0",
		IsActive: true, AddedAt: time.Now(),
	}
	if err := s.db.Create(row).Error; err != nil {
		if isDuplicateKey(err) {
			return nil, fmt.Errorf("%w: %s", errDuplicateKey, key)
		}
		return nil, err
	}
	return toFarm(row), nil
}

// CreateFarm registers a farm with a reward stream over a duration.
func (s *DynamicIntegrationService) CreateFarm(key, name, poolTokenA, poolTokenB, rewardToken, rewardRate string, apy float64, durationDays int) (*DynamicFarm, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, errFarmKeyRequired
	}
	if durationDays <= 0 {
		return nil, errPositiveDuration
	}

	row := &models.IntegrationFarm{
		Key: key, Name: name, PoolTokenA: poolTokenA, PoolTokenB: poolTokenB,
		RewardToken: rewardToken, RewardRate: rewardRate, APY: apy,
		TotalStaked: "0", StartTime: time.Now(),
		EndTime:  time.Now().Add(time.Duration(durationDays) * 24 * time.Hour),
		IsActive: true, AddedAt: time.Now(),
	}
	if err := s.db.Create(row).Error; err != nil {
		if isDuplicateKey(err) {
			return nil, fmt.Errorf("%w: %s", errDuplicateKey, key)
		}
		return nil, err
	}
	return toFarm(row), nil
}

// ActivateChain pauses or resumes a chain.
func (s *DynamicIntegrationService) ActivateChain(key string, active bool) error {
	res := s.db.Model(&models.IntegrationChain{}).Where("key = ?", key).Update("is_active", active)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("%w: %s", errChainNotFound, key)
	}
	return nil
}

// ActivateToken pauses or resumes a token.
func (s *DynamicIntegrationService) ActivateToken(key string, active bool) error {
	res := s.db.Model(&models.IntegrationToken{}).Where("key = ?", key).Update("is_active", active)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("%w: %s", errTokenNotFound, key)
	}
	return nil
}

// GetChain returns a chain by key.
func (s *DynamicIntegrationService) GetChain(key string) (*DynamicChain, error) {
	var row models.IntegrationChain
	if err := s.db.Where("key = ?", key).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("%w: %s", errChainNotFound, key)
		}
		return nil, err
	}
	return toChain(&row), nil
}

// GetToken returns a token by key.
func (s *DynamicIntegrationService) GetToken(key string) (*DynamicToken, error) {
	var row models.IntegrationToken
	if err := s.db.Where("key = ?", key).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("%w: %s", errTokenNotFound, key)
		}
		return nil, err
	}
	return toToken(&row), nil
}

// GetAllChains returns all chains, optionally only the active ones.
func (s *DynamicIntegrationService) GetAllChains(activeOnly bool) []*DynamicChain {
	var rows []models.IntegrationChain
	q := s.db.Order("added_at asc")
	if activeOnly {
		q = q.Where("is_active = ?", true)
	}
	if err := q.Find(&rows).Error; err != nil {
		return []*DynamicChain{}
	}
	out := make([]*DynamicChain, 0, len(rows))
	for i := range rows {
		out = append(out, toChain(&rows[i]))
	}
	return out
}

// GetAllTokens returns tokens, optionally filtered by chain and active state.
func (s *DynamicIntegrationService) GetAllTokens(chainKey string, activeOnly bool) []*DynamicToken {
	var rows []models.IntegrationToken
	q := s.db.Order("added_at asc")
	if chainKey != "" {
		q = q.Where("chain_key = ?", chainKey)
	}
	if activeOnly {
		q = q.Where("is_active = ?", true)
	}
	if err := q.Find(&rows).Error; err != nil {
		return []*DynamicToken{}
	}
	out := make([]*DynamicToken, 0, len(rows))
	for i := range rows {
		out = append(out, toToken(&rows[i]))
	}
	return out
}

// GetAllBridges returns every registered bridge route.
func (s *DynamicIntegrationService) GetAllBridges() []*DynamicBridge {
	var rows []models.IntegrationBridge
	if err := s.db.Order("added_at asc").Find(&rows).Error; err != nil {
		return []*DynamicBridge{}
	}
	out := make([]*DynamicBridge, 0, len(rows))
	for i := range rows {
		out = append(out, toBridge(&rows[i]))
	}
	return out
}

// GetAllFarms returns registered pools and farms.
func (s *DynamicIntegrationService) GetAllFarms(activeOnly bool) []*DynamicFarm {
	var rows []models.IntegrationFarm
	q := s.db.Order("added_at asc")
	if activeOnly {
		q = q.Where("is_active = ?", true)
	}
	if err := q.Find(&rows).Error; err != nil {
		return []*DynamicFarm{}
	}
	out := make([]*DynamicFarm, 0, len(rows))
	for i := range rows {
		out = append(out, toFarm(&rows[i]))
	}
	return out
}

// GetStats reports registry counts from the database.
func (s *DynamicIntegrationService) GetStats() *IntegrationStats {
	return &IntegrationStats{
		TotalChains:   s.count(&models.IntegrationChain{}, false),
		TotalTokens:   s.count(&models.IntegrationToken{}, false),
		TotalBridges:  s.count(&models.IntegrationBridge{}, false),
		TotalFarms:    s.count(&models.IntegrationFarm{}, false),
		ActiveChains:  s.count(&models.IntegrationChain{}, true),
		ActiveTokens:  s.count(&models.IntegrationToken{}, true),
		ActiveBridges: s.count(&models.IntegrationBridge{}, true),
		ActiveFarms:   s.count(&models.IntegrationFarm{}, true),
		LastUpdated:   time.Now(),
	}
}

func (s *DynamicIntegrationService) count(model interface{}, activeOnly bool) int {
	var n int64
	q := s.db.Model(model)
	if activeOnly {
		q = q.Where("is_active = ?", true)
	}
	if err := q.Count(&n).Error; err != nil {
		return 0
	}
	return int(n)
}

// SearchChains matches a chain name or symbol, case-insensitively.
func (s *DynamicIntegrationService) SearchChains(query string) []*DynamicChain {
	like := "%" + strings.ToLower(strings.TrimSpace(query)) + "%"
	var rows []models.IntegrationChain
	s.db.Where("LOWER(name) LIKE ? OR LOWER(symbol) LIKE ?", like, like).
		Order("added_at asc").Find(&rows)
	out := make([]*DynamicChain, 0, len(rows))
	for i := range rows {
		out = append(out, toChain(&rows[i]))
	}
	return out
}

// SearchTokens matches a token name or symbol, case-insensitively.
func (s *DynamicIntegrationService) SearchTokens(query string) []*DynamicToken {
	like := "%" + strings.ToLower(strings.TrimSpace(query)) + "%"
	var rows []models.IntegrationToken
	s.db.Where("LOWER(name) LIKE ? OR LOWER(symbol) LIKE ?", like, like).
		Order("added_at asc").Find(&rows)
	out := make([]*DynamicToken, 0, len(rows))
	for i := range rows {
		out = append(out, toToken(&rows[i]))
	}
	return out
}

// DeleteChain removes a chain registration.
func (s *DynamicIntegrationService) DeleteChain(key string) error {
	res := s.db.Where("key = ?", key).Delete(&models.IntegrationChain{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("%w: %s", errChainNotFound, key)
	}
	return nil
}

// DeleteToken removes a token registration.
func (s *DynamicIntegrationService) DeleteToken(key string) error {
	res := s.db.Where("key = ?", key).Delete(&models.IntegrationToken{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("%w: %s", errTokenNotFound, key)
	}
	return nil
}

// isDuplicateKey reports whether err is a unique-constraint violation. The
// registry uses string primary keys, so a duplicate registration surfaces as a
// constraint error rather than a GORM "record not found".
func isDuplicateKey(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique") || strings.Contains(msg, "duplicate")
}

// API Handlers

func (s *DynamicIntegrationService) AddEVMChainHandler(c *gin.Context) {
	var req struct {
		Key         string `json:"key" binding:"required"`
		Name        string `json:"name" binding:"required"`
		ChainID     string `json:"chain_id" binding:"required"`
		RPCURL      string `json:"rpc_url" binding:"required"`
		ExplorerURL string `json:"explorer_url" binding:"required"`
		Symbol      string `json:"symbol" binding:"required"`
		Decimals    int    `json:"decimals" binding:"required"`
		NativeToken string `json:"native_token"`
		AddedBy     string `json:"added_by"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequestError(c, err)
		return
	}

	chain, err := s.AddEVMChain(req.Key, req.Name, req.ChainID, req.RPCURL, req.ExplorerURL, req.Symbol, req.NativeToken, req.Decimals, req.AddedBy)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, chain)
}

func (s *DynamicIntegrationService) AddTokenHandler(c *gin.Context) {
	var req struct {
		Key      string `json:"key" binding:"required"`
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
		respondBadRequestError(c, err)
		return
	}

	supply := req.Supply
	if supply == "" {
		supply = "0"
	}

	token, err := s.AddToken(req.Key, req.Address, req.ChainKey, req.Name, req.Symbol, req.Standard, req.Decimals, supply, req.AddedBy)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, token)
}

func (s *DynamicIntegrationService) GetChainsHandler(c *gin.Context) {
	activeOnly := c.Query("active_only") == "true"
	chains := s.GetAllChains(activeOnly)
	c.JSON(http.StatusOK, gin.H{"chains": chains, "count": len(chains)})
}

func (s *DynamicIntegrationService) GetTokensHandler(c *gin.Context) {
	chainKey := c.Query("chain_key")
	activeOnly := c.Query("active_only") == "true"
	tokens := s.GetAllTokens(chainKey, activeOnly)
	c.JSON(http.StatusOK, gin.H{"tokens": tokens, "count": len(tokens)})
}

func (s *DynamicIntegrationService) GetFarmsHandler(c *gin.Context) {
	activeOnly := c.Query("active_only") == "true"
	farms := s.GetAllFarms(activeOnly)
	c.JSON(http.StatusOK, gin.H{"farms": farms, "count": len(farms)})
}

func (s *DynamicIntegrationService) GetStatsHandler(c *gin.Context) {
	c.JSON(http.StatusOK, s.GetStats())
}

func (s *DynamicIntegrationService) CreatePoolHandler(c *gin.Context) {
	var req struct {
		Key     string  `json:"key" binding:"required"`
		Name    string  `json:"name" binding:"required"`
		TokenA  string  `json:"token_a" binding:"required"`
		TokenB  string  `json:"token_b" binding:"required"`
		FeeRate float64 `json:"fee_rate"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequestError(c, err)
		return
	}

	pool, err := s.CreatePool(req.Key, req.Name, req.TokenA, req.TokenB, req.FeeRate)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, pool)
}

func (s *DynamicIntegrationService) CreateFarmHandler(c *gin.Context) {
	var req struct {
		Key          string  `json:"key" binding:"required"`
		Name         string  `json:"name" binding:"required"`
		PoolTokenA   string  `json:"pool_token_a" binding:"required"`
		PoolTokenB   string  `json:"pool_token_b" binding:"required"`
		RewardToken  string  `json:"reward_token" binding:"required"`
		RewardRate   string  `json:"reward_rate" binding:"required"`
		APY          float64 `json:"apy"`
		DurationDays int     `json:"duration_days" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequestError(c, err)
		return
	}

	farm, err := s.CreateFarm(req.Key, req.Name, req.PoolTokenA, req.PoolTokenB, req.RewardToken, req.RewardRate, req.APY, req.DurationDays)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, farm)
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
		FeePercent    float64 `json:"fee_percent"`
		FeeFixed      string  `json:"fee_fixed" binding:"required"`
		EstimatedTime string  `json:"estimated_time" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequestError(c, err)
		return
	}

	bridge, err := s.AddBridge(req.Key, req.Name, req.SrcChain, req.DstChain, req.RouterAddress, req.MinAmount, req.MaxAmount, req.FeePercent, req.FeeFixed, req.EstimatedTime)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, bridge)
}

func (s *DynamicIntegrationService) SearchHandler(c *gin.Context) {
	query := c.Query("q")
	kind := c.Query("type")
	if kind == "tokens" {
		tokens := s.SearchTokens(query)
		c.JSON(http.StatusOK, gin.H{"tokens": tokens, "count": len(tokens)})
		return
	}
	chains := s.SearchChains(query)
	c.JSON(http.StatusOK, gin.H{"chains": chains, "count": len(chains)})
}

// AddNonEVMChainHandler adds a non-EVM chain.
func (s *DynamicIntegrationService) AddNonEVMChainHandler(c *gin.Context) {
	var req struct {
		Key         string `json:"key" binding:"required"`
		Name        string `json:"name" binding:"required"`
		Type        string `json:"type" binding:"required"`
		RPCURL      string `json:"rpc_url" binding:"required"`
		ExplorerURL string `json:"explorer_url" binding:"required"`
		Symbol      string `json:"symbol" binding:"required"`
		Decimals    int    `json:"decimals" binding:"required"`
		AddedBy     string `json:"added_by"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequestError(c, err)
		return
	}

	chain, err := s.AddNonEVMChain(req.Key, req.Name, req.Type, req.RPCURL, req.ExplorerURL, req.Symbol, req.Decimals, req.AddedBy)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, chain)
}

// ActivateChainHandler pauses or resumes a chain.
func (s *DynamicIntegrationService) ActivateChainHandler(c *gin.Context) {
	var req struct {
		Active bool `json:"active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequestError(c, err)
		return
	}
	key := c.Param("key")
	if err := s.ActivateChain(key, req.Active); err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"key": key, "active": req.Active})
}

// ActivateTokenHandler pauses or resumes a token.
func (s *DynamicIntegrationService) ActivateTokenHandler(c *gin.Context) {
	var req struct {
		Active bool `json:"active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequestError(c, err)
		return
	}
	key := c.Param("key")
	if err := s.ActivateToken(key, req.Active); err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"key": key, "active": req.Active})
}

// DeleteChainHandler removes a chain registration.
func (s *DynamicIntegrationService) DeleteChainHandler(c *gin.Context) {
	key := c.Param("key")
	if err := s.DeleteChain(key); err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"key": key, "deleted": true})
}

// DeleteTokenHandler removes a token registration.
func (s *DynamicIntegrationService) DeleteTokenHandler(c *gin.Context) {
	key := c.Param("key")
	if err := s.DeleteToken(key); err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"key": key, "deleted": true})
}

// GetBridgesHandler lists every registered bridge route.
func (s *DynamicIntegrationService) GetBridgesHandler(c *gin.Context) {
	bridges := s.GetAllBridges()
	c.JSON(http.StatusOK, gin.H{"bridges": bridges, "count": len(bridges)})
}
