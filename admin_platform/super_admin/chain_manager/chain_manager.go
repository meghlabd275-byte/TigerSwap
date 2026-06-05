/**
 * TigerSwap Chain Manager Service
 * Super Admin Backend Service for Chain Management
 * Built from scratch - no dependencies on other protocols
 */

package chain_manager

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// ============================================================================
// Types
// ============================================================================

type ChainCategory string
type ChainStatus string

const (
	CategoryEVM     ChainCategory = "evm"
	CategorySolana  ChainCategory = "solana"
	CategoryAptos  ChainCategory = "aptos"
	CategorySui    ChainCategory = "sui"
	CategoryTon     ChainCategory = "ton"
	CategoryTron    ChainCategory = "tron"
	CategoryCosmos  ChainCategory = "cosmos"
	CategoryNear    ChainCategory = "near"
	CategoryAlgo    ChainCategory = "algorand"
	CategoryPolka   ChainCategory = "polkadot"
	CategoryCardano ChainCategory = "cardano"
	CategoryOther   ChainCategory = "other"

	StatusActive      ChainStatus = "active"
	StatusInactive    ChainStatus = "inactive"
	StatusDeprecated ChainStatus = "deprecated"
	StatusMaintenance ChainStatus = "maintenance"
)

type ChainConfig struct {
	ID                     string            `json:"id"`
	Name                   string            `json:"name"`
	Symbol                 string            `json:"symbol"`
	Category               ChainCategory     `json:"category"`
	Status                 ChainStatus       `json:"status"`
	ChainID                uint64            `json:"chainId"`
	NetworkID              uint64            `json:"networkId,omitempty"`
	RPCURLs                []string          `json:"rpcUrls"`
	ExplorerURLs           []string          `json:"explorerUrls"`
	NativeCurrency         NativeCurrency    `json:"nativeCurrency"`
	BlockTime             float64           `json:"blockTime,omitempty"`
	GasLimit              uint64            `json:"gasLimit,omitempty"`
	SupportsEIP1559       bool              `json:"supportsEIP1559,omitempty"`
	SupportsFlashbots     bool              `json:"supportsFlashbots,omitempty"`
	SupportsMEV           bool              `json:"supportsMEV,omitempty"`
	SupportsMulticall     bool              `json:"supportsMulticall,omitempty"`
	AddedBy               string            `json:"addedBy,omitempty"`
	AddedAt               int64             `json:"addedAt"`
	UpdatedAt             int64             `json:"updatedAt"`
	Notes                 string            `json:"notes,omitempty"`
}

type NativeCurrency struct {
	Name     string `json:"name"`
	Symbol   string `json:"symbol"`
	Decimals int    `json:"decimals"`
}

type RPCEndpoint struct {
	URL        string `json:"url"`
	ChainID    string `json:"chainId"`
	Name       string `json:"name"`
	Priority   int    `json:"priority"`
	IsHealthy  bool   `json:"isHealthy"`
	LatencyMs  int64  `json:"latencyMs"`
	LastCheck  int64  `json:"lastCheck"`
	IsWebSocket bool  `json:"isWebSocket"`
	IsBackup   bool   `json:"isBackup"`
}

type ChainManager struct {
	mu           sync.RWMutex
	chains       map[string]*ChainConfig
	rpcEndpoints map[string][]RPCEndpoint
	adminLog     []AdminAction
}

type AdminAction struct {
	AdminID    string    `json:"adminId"`
	Action     string    `json:"action"`
	Target     string    `json:"target"`
	Details    string    `json:"details"`
	Timestamp  int64     `json:"timestamp"`
	IPAddress  string    `json:"ipAddress,omitempty"`
}

type CreateChainRequest struct {
	ID             string          `json:"id" validate:"required,min=1,max=50"`
	Name           string          `json:"name" validate:"required,min=1,max=100"`
	Symbol         string          `json:"symbol" validate:"required,min=1,max=20"`
	Category       ChainCategory   `json:"category" validate:"required"`
	Status         ChainStatus     `json:"status" validate:"required"`
	ChainID        uint64          `json:"chainId" validate:"required"`
	NetworkID      uint64          `json:"networkId,omitempty"`
	RPCURLs        []string       `json:"rpcUrls" validate:"required,min=1"`
	ExplorerURLs   []string       `json:"explorerUrls,omitempty"`
	NativeCurrency NativeCurrency `json:"nativeCurrency" validate:"required"`
	BlockTime      float64        `json:"blockTime,omitempty"`
	GasLimit       uint64         `json:"gasLimit,omitempty"`
	SupportsEIP1559    bool       `json:"supportsEIP1559,omitempty"`
	SupportsFlashbots   bool      `json:"supportsFlashbots,omitempty"`
	SupportsMEV        bool       `json:"supportsMEV,omitempty"`
	SupportsMulticall   bool      `json:"supportsMulticall,omitempty"`
	Notes          string         `json:"notes,omitempty"`
}

type UpdateChainRequest struct {
	Name           *string          `json:"name,omitempty"`
	Symbol         *string          `json:"symbol,omitempty"`
	Status         *ChainStatus     `json:"status,omitempty"`
	RPCURLs        []string         `json:"rpcUrls,omitempty"`
	ExplorerURLs   []string         `json:"explorerUrls,omitempty"`
	BlockTime      *float64         `json:"blockTime,omitempty"`
	GasLimit       *uint64          `json:"gasLimit,omitempty"`
	SupportsEIP1559    *bool        `json:"supportsEIP1559,omitempty"`
	SupportsFlashbots   *bool       `json:"supportsFlashbots,omitempty"`
	SupportsMEV        *bool        `json:"supportsMEV,omitempty"`
	SupportsMulticall   *bool       `json:"supportsMulticall,omitempty"`
	Notes          *string          `json:"notes,omitempty"`
}

type ChainStats struct {
	Total      int            `json:"total"`
	EVMCount   int            `json:"evmCount"`
	NonEVMCount int          `json:"nonEvmCount"`
	ByCategory map[string]int `json:"byCategory"`
	ByStatus  map[string]int `json:"byStatus"`
}

type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// ============================================================================
// Singleton
// ============================================================================

var (
	instance *ChainManager
	once     sync.Once
)

func GetChainManager() *ChainManager {
	once.Do(func() {
		instance = NewChainManager()
	})
	return instance
}

func NewChainManager() *ChainManager {
	cm := &ChainManager{
		chains:       make(map[string]*ChainConfig),
		rpcEndpoints: make(map[string][]RPCEndpoint),
		adminLog:     make([]AdminAction, 0),
	}
	cm.initializeDefaultChains()
	return cm
}

// ============================================================================
// Chain CRUD Operations
// ============================================================================

func (cm *ChainManager) AddChain(req CreateChainRequest, adminID string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Check if exists
	if _, exists := cm.chains[req.ID]; exists {
		return fmt.Errorf("chain %s already exists", req.ID)
	}

	// Validate RPC URLs
	for _, url := range req.RPCURLs {
		if url == "" {
			return fmt.Errorf("RPC URL cannot be empty")
		}
	}

	// Create chain config
	chain := &ChainConfig{
		ID:           req.ID,
		Name:         req.Name,
		Symbol:       req.Symbol,
		Category:     req.Category,
		Status:       req.Status,
		ChainID:      req.ChainID,
		NetworkID:    req.NetworkID,
		RPCURLs:      req.RPCURLs,
		ExplorerURLs: req.ExplorerURLs,
		NativeCurrency: req.NativeCurrency,
		BlockTime:    req.BlockTime,
		GasLimit:     req.GasLimit,
		SupportsEIP1559:  req.SupportsEIP1559,
		SupportsFlashbots: req.SupportsFlashbots,
		SupportsMEV:      req.SupportsMEV,
		SupportsMulticall: req.SupportsMulticall,
		AddedBy:      adminID,
		AddedAt:      time.Now().Unix(),
		UpdatedAt:    time.Now().Unix(),
		Notes:        req.Notes,
	}

	cm.chains[req.ID] = chain

	// Initialize RPC endpoints
	endpoints := make([]RPCEndpoint, len(req.RPCURLs))
	for i, url := range req.RPCURLs {
		endpoints[i] = RPCEndpoint{
			URL:         url,
			ChainID:     req.ID,
			Name:        fmt.Sprintf("%s RPC %d", req.Name, i+1),
			Priority:    i,
			IsHealthy:   true,
			LatencyMs:   0,
			LastCheck:   time.Now().Unix(),
			IsWebSocket: isWebSocketURL(url),
			IsBackup:    i > 0,
		}
	}
	cm.rpcEndpoints[req.ID] = endpoints

	// Log admin action
	cm.adminLog = append(cm.adminLog, AdminAction{
		AdminID:   adminID,
		Action:    "ADD_CHAIN",
		Target:    req.ID,
		Details:   fmt.Sprintf("Added chain %s (ChainID: %d)", req.Name, req.ChainID),
		Timestamp: time.Now().Unix(),
	})

	return nil
}

func (cm *ChainManager) UpdateChain(chainID string, req UpdateChainRequest, adminID string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	chain, exists := cm.chains[chainID]
	if !exists {
		return fmt.Errorf("chain %s not found", chainID)
	}

	// Apply updates
	if req.Name != nil {
		chain.Name = *req.Name
	}
	if req.Symbol != nil {
		chain.Symbol = *req.Symbol
	}
	if req.Status != nil {
		chain.Status = *req.Status
	}
	if req.RPCURLs != nil {
		chain.RPCURLs = req.RPCURLs
		// Update RPC endpoints
		endpoints := make([]RPCEndpoint, len(req.RPCURLs))
		for i, url := range req.RPCURLs {
			endpoints[i] = RPCEndpoint{
				URL:         url,
				ChainID:     chainID,
				Name:        fmt.Sprintf("%s RPC %d", chain.Name, i+1),
				Priority:    i,
				IsHealthy:   true,
				LatencyMs:   0,
				LastCheck:   time.Now().Unix(),
				IsWebSocket: isWebSocketURL(url),
				IsBackup:    i > 0,
			}
		}
		cm.rpcEndpoints[chainID] = endpoints
	}
	if req.ExplorerURLs != nil {
		chain.ExplorerURLs = req.ExplorerURLs
	}
	if req.BlockTime != nil {
		chain.BlockTime = *req.BlockTime
	}
	if req.GasLimit != nil {
		chain.GasLimit = *req.GasLimit
	}
	if req.SupportsEIP1559 != nil {
		chain.SupportsEIP1559 = *req.SupportsEIP1559
	}
	if req.SupportsFlashbots != nil {
		chain.SupportsFlashbots = *req.SupportsFlashbots
	}
	if req.SupportsMEV != nil {
		chain.SupportsMEV = *req.SupportsMEV
	}
	if req.SupportsMulticall != nil {
		chain.SupportsMulticall = *req.SupportsMulticall
	}
	if req.Notes != nil {
		chain.Notes = *req.Notes
	}

	chain.UpdatedAt = time.Now().Unix()

	// Log admin action
	cm.adminLog = append(cm.adminLog, AdminAction{
		AdminID:   adminID,
		Action:    "UPDATE_CHAIN",
		Target:    chainID,
		Details:   fmt.Sprintf("Updated chain %s", chain.Name),
		Timestamp: time.Now().Unix(),
	})

	return nil
}

func (cm *ChainManager) DeleteChain(chainID, adminID string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	chain, exists := cm.chains[chainID]
	if !exists {
		return fmt.Errorf("chain %s not found", chainID)
	}

	delete(cm.chains, chainID)
	delete(cm.rpcEndpoints, chainID)

	// Log admin action
	cm.adminLog = append(cm.adminLog, AdminAction{
		AdminID:   adminID,
		Action:    "DELETE_CHAIN",
		Target:    chainID,
		Details:   fmt.Sprintf("Deleted chain %s", chain.Name),
		Timestamp: time.Now().Unix(),
	})

	return nil
}

func (cm *ChainManager) GetChain(chainID string) (*ChainConfig, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	chain, exists := cm.chains[chainID]
	if !exists {
		return nil, fmt.Errorf("chain %s not found", chainID)
	}

	return chain, nil
}

func (cm *ChainManager) GetAllChains() []*ChainConfig {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	chains := make([]*ChainConfig, 0, len(cm.chains))
	for _, chain := range cm.chains {
		chains = append(chains, chain)
	}
	return chains
}

func (cm *ChainManager) GetChainsByCategory(category ChainCategory) []*ChainConfig {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	chains := make([]*ChainConfig, 0)
	for _, chain := range cm.chains {
		if chain.Category == category {
			chains = append(chains, chain)
		}
	}
	return chains
}

func (cm *ChainManager) GetChainsByStatus(status ChainStatus) []*ChainConfig {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	chains := make([]*ChainConfig, 0)
	for _, chain := range cm.chains {
		if chain.Status == status {
			chains = append(chains, chain)
		}
	}
	return chains
}

func (cm *ChainManager) SearchChains(query string) []*ChainConfig {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	query = toLower(query)
	chains := make([]*ChainConfig, 0)
	for _, chain := range cm.chains {
		if contains(toLower(chain.Name), query) ||
		   contains(toLower(chain.Symbol), query) ||
		   contains(toLower(chain.ID), query) {
			chains = append(chains, chain)
		}
	}
	return chains
}

// ============================================================================
// RPC Management
// ============================================================================

func (cm *ChainManager) AddRPCEndpoint(chainID string, endpoint RPCEndpoint, adminID string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if _, exists := cm.chains[chainID]; !exists {
		return fmt.Errorf("chain %s not found", chainID)
	}

	// Check for duplicate
	for _, ep := range cm.rpcEndpoints[chainID] {
		if ep.URL == endpoint.URL {
			return fmt.Errorf("RPC endpoint already exists")
		}
	}

	endpoint.ChainID = chainID
	endpoint.IsHealthy = true
	endpoint.LastCheck = time.Now().Unix()
	endpoint.IsWebSocket = isWebSocketURL(endpoint.URL)
	cm.rpcEndpoints[chainID] = append(cm.rpcEndpoints[chainID], endpoint)

	// Log admin action
	cm.adminLog = append(cm.adminLog, AdminAction{
		AdminID:   adminID,
		Action:    "ADD_RPC",
		Target:    chainID,
		Details:   fmt.Sprintf("Added RPC endpoint: %s", endpoint.URL),
		Timestamp: time.Now().Unix(),
	})

	return nil
}

func (cm *ChainManager) RemoveRPCEndpoint(chainID, url, adminID string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	endpoints, exists := cm.rpcEndpoints[chainID]
	if !exists {
		return fmt.Errorf("no RPC endpoints found for chain %s", chainID)
	}

	found := false
	newEndpoints := make([]RPCEndpoint, 0)
	for _, ep := range endpoints {
		if ep.URL == url {
			found = true
			continue
		}
		newEndpoints = append(newEndpoints, ep)
	}

	if !found {
		return fmt.Errorf("RPC endpoint not found")
	}

	cm.rpcEndpoints[chainID] = newEndpoints

	// Log admin action
	cm.adminLog = append(cm.adminLog, AdminAction{
		AdminID:   adminID,
		Action:    "REMOVE_RPC",
		Target:    chainID,
		Details:   fmt.Sprintf("Removed RPC endpoint: %s", url),
		Timestamp: time.Now().Unix(),
	})

	return nil
}

func (cm *ChainManager) GetBestRPC(chainID string) (string, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	endpoints, exists := cm.rpcEndpoints[chainID]
	if !exists || len(endpoints) == 0 {
		return "", fmt.Errorf("no RPC endpoints available for chain %s", chainID)
	}

	// Sort by priority and health
	best := endpoints[0]
	for _, ep := range endpoints[1:] {
		if ep.IsHealthy && !ep.IsBackup {
			best = ep
			break
		}
	}

	return best.URL, nil
}

// ============================================================================
// Statistics
// ============================================================================

func (cm *ChainManager) GetStats() ChainStats {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	stats := ChainStats{
		ByCategory: make(map[string]int),
		ByStatus:  make(map[string]int),
	}

	for _, chain := range cm.chains {
		stats.Total++
		if chain.Category == CategoryEVM {
			stats.EVMCount++
		} else {
			stats.NonEVMCount++
		}
		stats.ByCategory[string(chain.Category)]++
		stats.ByStatus[string(chain.Status)]++
	}

	return stats
}

// ============================================================================
// Admin Log
// ============================================================================

func (cm *ChainManager) GetAdminLog(limit int) []AdminAction {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	if limit <= 0 || limit > len(cm.adminLog) {
		limit = len(cm.adminLog)
	}

	log := make([]AdminAction, limit)
	copy(log, cm.adminLog[len(cm.adminLog)-limit:])
	return log
}

// ============================================================================
// Import/Export
// ============================================================================

func (cm *ChainManager) ExportChains() ([]byte, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	return json.MarshalIndent(cm.chains, "", "  ")
}

func (cm *ChainManager) ImportChains(data []byte, adminID string) (int, error) {
	var chains map[string]*ChainConfig
	if err := json.Unmarshal(data, &chains); err != nil {
		return 0, err
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	imported := 0
	for id, chain := range chains {
		if _, exists := cm.chains[id]; !exists {
			chain.AddedBy = adminID
			chain.AddedAt = time.Now().Unix()
			chain.UpdatedAt = time.Now().Unix()
			cm.chains[id] = chain
			imported++
		}
	}

	return imported, nil
}

// ============================================================================
// Default Chains Initialization
// ============================================================================

func (cm *ChainManager) initializeDefaultChains() {
	// EVM Chains
	cm.chains["ethereum"] = &ChainConfig{
		ID: "ethereum", Name: "Ethereum", Symbol: "ETH", Category: CategoryEVM,
		Status: StatusActive, ChainID: 1,
		RPCURLs: []string{"https://eth.llamarpc.com", "https://rpc.ankr.com/eth"},
		ExplorerURLs: []string{"https://etherscan.io"},
		NativeCurrency: NativeCurrency{Name: "Ether", Symbol: "ETH", Decimals: 18},
		BlockTime: 12, GasLimit: 30000000,
		SupportsEIP1559: true, SupportsFlashbots: true, SupportsMEV: true, SupportsMulticall: true,
	}

	cm.chains["sepolia"] = &ChainConfig{
		ID: "sepolia", Name: "Sepolia", Symbol: "ETH", Category: CategoryEVM,
		Status: StatusActive, ChainID: 11155111,
		RPCURLs: []string{"https://rpc.sepolia.org"},
		ExplorerURLs: []string{"https://sepolia.etherscan.io"},
		NativeCurrency: NativeCurrency{Name: "Sepolia Ether", Symbol: "ETH", Decimals: 18},
		BlockTime: 12, SupportsEIP1559: true,
	}

	cm.chains["bnb-smart-chain"] = &ChainConfig{
		ID: "bnb-smart-chain", Name: "BNB Chain", Symbol: "BNB", Category: CategoryEVM,
		Status: StatusActive, ChainID: 56,
		RPCURLs: []string{"https://bsc-dataseed.binance.org", "https://rpc.ankr.com/bsc"},
		ExplorerURLs: []string{"https://bscscan.com"},
		NativeCurrency: NativeCurrency{Name: "BNB", Symbol: "BNB", Decimals: 18},
		BlockTime: 3, GasLimit: 30000000, SupportsEIP1559: true, SupportsMulticall: true,
	}

	cm.chains["polygon"] = &ChainConfig{
		ID: "polygon", Name: "Polygon", Symbol: "MATIC", Category: CategoryEVM,
		Status: StatusActive, ChainID: 137,
		RPCURLs: []string{"https://polygon-rpc.com", "https://rpc.ankr.com/polygon"},
		ExplorerURLs: []string{"https://polygonscan.com"},
		NativeCurrency: NativeCurrency{Name: "MATIC", Symbol: "MATIC", Decimals: 18},
		BlockTime: 2, GasLimit: 30000000, SupportsEIP1559: true, SupportsMulticall: true,
	}

	cm.chains["arbitrum-one"] = &ChainConfig{
		ID: "arbitrum-one", Name: "Arbitrum One", Symbol: "ETH", Category: CategoryEVM,
		Status: StatusActive, ChainID: 42161,
		RPCURLs: []string{"https://arb1.arbitrum.io/rpc", "https://rpc.ankr.com/arbitrum"},
		ExplorerURLs: []string{"https://arbiscan.io"},
		NativeCurrency: NativeCurrency{Name: "Ether", Symbol: "ETH", Decimals: 18},
		BlockTime: 1, GasLimit: 32000000, SupportsEIP1559: true, SupportsMulticall: true,
	}

	cm.chains["optimism"] = &ChainConfig{
		ID: "optimism", Name: "Optimism", Symbol: "ETH", Category: CategoryEVM,
		Status: StatusActive, ChainID: 10,
		RPCURLs: []string{"https://mainnet.optimism.io", "https://rpc.ankr.com/optimism"},
		ExplorerURLs: []string{"https://optimistic.etherscan.io"},
		NativeCurrency: NativeCurrency{Name: "Ether", Symbol: "ETH", Decimals: 18},
		BlockTime: 2, SupportsEIP1559: true, SupportsMulticall: true,
	}

	cm.chains["base"] = &ChainConfig{
		ID: "base", Name: "Base", Symbol: "ETH", Category: CategoryEVM,
		Status: StatusActive, ChainID: 8453,
		RPCURLs: []string{"https://mainnet.base.org", "https://base.llamarpc.com"},
		ExplorerURLs: []string{"https://basescan.org"},
		NativeCurrency: NativeCurrency{Name: "Ether", Symbol: "ETH", Decimals: 18},
		BlockTime: 2, SupportsEIP1559: true, SupportsMulticall: true,
	}

	cm.chains["avalanche-c"] = &ChainConfig{
		ID: "avalanche-c", Name: "Avalanche C-Chain", Symbol: "AVAX", Category: CategoryEVM,
		Status: StatusActive, ChainID: 43114,
		RPCURLs: []string{"https://api.avax.network/ext/bc/C/rpc", "https://rpc.ankr.com/avalanche"},
		ExplorerURLs: []string{"https://snowtrace.io"},
		NativeCurrency: NativeCurrency{Name: "Avalanche", Symbol: "AVAX", Decimals: 18},
		BlockTime: 2, SupportsEIP1559: true,
	}

	cm.chains["fantom"] = &ChainConfig{
		ID: "fantom", Name: "Fantom", Symbol: "FTM", Category: CategoryEVM,
		Status: StatusActive, ChainID: 250,
		RPCURLs: []string{"https://rpc.fantom.network", "https://fantom.llamarpc.com"},
		ExplorerURLs: []string{"https://ftmscan.com"},
		NativeCurrency: NativeCurrency{Name: "Fantom", Symbol: "FTM", Decimals: 18},
		BlockTime: 1,
	}

	// Non-EVM Chains
	cm.chains["solana"] = &ChainConfig{
		ID: "solana", Name: "Solana", Symbol: "SOL", Category: CategorySolana,
		Status: StatusActive, ChainID: 101,
		RPCURLs: []string{"https://api.mainnet-beta.solana.com", "https://rpc.ankr.com/solana"},
		ExplorerURLs: []string{"https://solscan.io", "https://solana.fm"},
		NativeCurrency: NativeCurrency{Name: "Solana", Symbol: "SOL", Decimals: 9},
		BlockTime: 0.4, SupportsMulticall: false,
	}

	cm.chains["aptos"] = &ChainConfig{
		ID: "aptos", Name: "Aptos", Symbol: "APT", Category: CategoryAptos,
		Status: StatusActive, ChainID: 1,
		RPCURLs: []string{"https://fullnode.mainnet.aptoslabs.com"},
		ExplorerURLs: []string{"https://explorer.aptoslabs.com"},
		NativeCurrency: NativeCurrency{Name: "Aptos", Symbol: "APT", Decimals: 8},
		BlockTime: 1,
	}

	cm.chains["sui"] = &ChainConfig{
		ID: "sui", Name: "Sui", Symbol: "SUI", Category: CategorySui,
		Status: StatusActive, ChainID: 1,
		RPCURLs: []string{"https://fullnode.mainnet.sui.io", "https://rpc.ankr.com/sui"},
		ExplorerURLs: []string{"https://suiscan.xyz", "https://explorer.sui.io"},
		NativeCurrency: NativeCurrency{Name: "Sui", Symbol: "SUI", Decimals: 9},
		BlockTime: 1,
	}

	cm.chains["ton"] = &ChainConfig{
		ID: "ton", Name: "TON", Symbol: "TON", Category: CategoryTon,
		Status: StatusActive, ChainID: 0,
		RPCURLs: []string{"https://toncenter.com/api/v2/jsonRPC", "https://tonapi.io/v2/jsonRPC"},
		ExplorerURLs: []string{"https://tonscan.org"},
		NativeCurrency: NativeCurrency{Name: "Toncoin", Symbol: "TON", Decimals: 9},
		BlockTime: 5,
	}

	cm.chains["tron"] = &ChainConfig{
		ID: "tron", Name: "TRON", Symbol: "TRX", Category: CategoryTron,
		Status: StatusActive, ChainID: 728126428,
		RPCURLs: []string{"https://api.trongrid.io", "https://rpc.ankr.com/tron"},
		ExplorerURLs: []string{"https://tronscan.org"},
		NativeCurrency: NativeCurrency{Name: "TRON", Symbol: "TRX", Decimals: 6},
		BlockTime: 3,
	}

	cm.chains["cosmos-hub"] = &ChainConfig{
		ID: "cosmos-hub", Name: "Cosmos Hub", Symbol: "ATOM", Category: CategoryCosmos,
		Status: StatusActive, ChainID: 1,
		RPCURLs: []string{"https://rpc.cosmos.network"},
		ExplorerURLs: []string{"https://mintscan.io/cosmoshub"},
		NativeCurrency: NativeCurrency{Name: "Atom", Symbol: "ATOM", Decimals: 6},
		BlockTime: 7,
	}

	cm.chains["near"] = &ChainConfig{
		ID: "near", Name: "NEAR Protocol", Symbol: "NEAR", Category: CategoryNear,
		Status: StatusActive, ChainID: 1,
		RPCURLs: []string{"https://rpc.ankr.com/near", "https://rpc.mainnet.near.org"},
		ExplorerURLs: []string{"https://explorer.near.org"},
		NativeCurrency: NativeCurrency{Name: "NEAR", Symbol: "NEAR", Decimals: 24},
		BlockTime: 1,
	}

	cm.chains["polkadot"] = &ChainConfig{
		ID: "polkadot", Name: "Polkadot", Symbol: "DOT", Category: CategoryPolka,
		Status: StatusActive, ChainID: 1,
		RPCURLs: []string{"https://rpc.polkadot.io"},
		ExplorerURLs: []string{"https://explorer.polkadot.io"},
		NativeCurrency: NativeCurrency{Name: "Polkadot", Symbol: "DOT", Decimals: 10},
		BlockTime: 6,
	}

	cm.chains["cardano"] = &ChainConfig{
		ID: "cardano", Name: "Cardano", Symbol: "ADA", Category: CategoryCardano,
		Status: StatusActive, ChainID: 1,
		RPCURLs: []string{"https://cardano-mainnet.blockfrost.io/api/v0"},
		ExplorerURLs: []string{"https://cardanoscan.io"},
		NativeCurrency: NativeCurrency{Name: "Ada", Symbol: "ADA", Decimals: 6},
		BlockTime: 20,
	}

	// Initialize RPC endpoints for all chains
	for id, chain := range cm.chains {
		endpoints := make([]RPCEndpoint, len(chain.RPCURLs))
		for i, url := range chain.RPCURLs {
			endpoints[i] = RPCEndpoint{
				URL:         url,
				ChainID:     id,
				Name:        fmt.Sprintf("%s RPC %d", chain.Name, i+1),
				Priority:    i,
				IsHealthy:   true,
				LatencyMs:   0,
				LastCheck:   time.Now().Unix(),
				IsWebSocket: isWebSocketURL(url),
				IsBackup:    i > 0,
			}
		}
		cm.rpcEndpoints[id] = endpoints
	}
}

// ============================================================================
// Helpers
// ============================================================================

func isWebSocketURL(url string) bool {
	return len(url) >= 3 && url[:3] == "ws"
}

func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			result[i] = c + 32
		} else {
			result[i] = c
		}
	}
	return string(result)
}

func contains(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}