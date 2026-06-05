// TigerSwap Chain Management - Go Implementation
// Enterprise-grade blockchain network management

package main

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// Chain types supported by TigerSwap
type ChainType string

const (
	ChainTypeEVM     ChainType = "evm"
	ChainTypeSolana  ChainType = "solana"
	ChainTypeTron    ChainType = "tron"
	ChainTypeSui     ChainType = "sui"
	ChainTypeAptos   ChainType = "aptos"
	ChainTypeNear    ChainType = "near"
	ChainTypeCosmos  ChainType = "cosmos"
)

// Blockchain represents a supported blockchain
type Blockchain struct {
	ID          string            `json:"id"`
	ChainID     int64             `json:"chainId"`
	Name        string            `json:"name"`
	Type        ChainType         `json:"type"`
	Symbol      string            `json:"symbol"`
	Decimals    int               `json:"decimals"`
	RPC         string            `json:"rpc"`
	WSRPC       string            `json:"wsRpc,omitempty"`
	Explorer    string            `json:"explorer"`
	ExplorerAPI string            `json:"explorerApi,omitempty"`
	Logo        string            `json:"logo"`
	IsNative    bool              `json:"isNative"`
	WrappedToken string          `json:"wrappedToken,omitempty"`
	IsEnabled   bool              `json:"isEnabled"`
	IsTestnet   bool              `json:"isTestnet"`
	AddedAt     int64             `json:"addedAt"`
	AddedBy     string            `json:"addedBy"`
	Capabilities ChainCapabilities `json:"capabilities"`
	GasSettings GasSettings       `json:"gasSettings"`
	Tokens      []string          `json:"tokens"`
	Metadata    ChainMetadata     `json:"metadata"`
}

// ChainCapabilities supported features
type ChainCapabilities struct {
	Swap          bool `json:"swap"`
	Bridge        bool `json:"bridge"`
	Staking       bool `json:"staking"`
	Farming       bool `json:"farming"`
	NFT           bool `json:"nft"`
	DappBrowser   bool `json:"dappBrowser"`
	MultiSig      bool `json:"multiSig"`
	HardwareWallet bool `json:"hardwareWallet"`
	Delegation    bool `json:"delegation"`
	Governance    bool `json:"governance"`
}

// GasSettings gas configuration
type GasSettings struct {
	GasToken          string  `json:"gasToken"`
	MinGasPrice       string  `json:"minGasPrice"`
	MaxGasPrice       string  `json:"maxGasPrice"`
	AvgGasPrice       string  `json:"avgGasPrice"`
	GasLimitMultiplier float64 `json:"gasLimitMultiplier"`
	EIP1559           bool    `json:"eip1559"`
	GasStationURL     string  `json:"gasStationUrl,omitempty"`
}

// ChainMetadata additional chain info
type ChainMetadata struct {
	Color     string            `json:"color"`
	BgColor   string            `json:"bgColor"`
	Website   string            `json:"website,omitempty"`
	Social    SocialLinks       `json:"socialLinks,omitempty"`
}

// SocialLinks social media links
type SocialLinks struct {
	Twitter   string `json:"twitter,omitempty"`
	Discord   string `json:"discord,omitempty"`
	Telegram  string `json:"telegram,omitempty"`
	Github    string `json:"github,omitempty"`
}

// Token represents a blockchain token
type Token struct {
	Address    string  `json:"address"`
	ChainID    int64   `json:"chainId"`
	Symbol     string  `json:"symbol"`
	Name       string  `json:"name"`
	Decimals   int     `json:"decimals"`
	Logo       string  `json:"logo"`
	IsNative   bool    `json:"isNative"`
	IsStable   bool    `json:"isStable"`
	IsWhitelisted bool `json:"isWhitelisted"`
	CoingeckoID string `json:"coingeckoId,omitempty"`
	Price      string  `json:"price,omitempty"`
	Change24h  float64 `json:"change24h,omitempty"`
	Volume24h  string  `json:"volume24h,omitempty"`
	Liquidity   string  `json:"liquidity,omitempty"`
	AddedAt    int64    `json:"addedAt"`
	AddedBy    string  `json:"addedBy"`
}

// BlockchainManager manages all chains
type BlockchainManager struct {
	mu      sync.RWMutex
	chains  map[string]*Blockchain
	tokens  map[string][]*Token
	nextID  int64
}

func NewBlockchainManager() *BlockchainManager {
	m := &BlockchainManager{
		chains: make(map[string]*Blockchain),
		tokens: make(map[string][]*Token),
	}
	m.initializeDefaultChains()
	return m
}

func (m *BlockchainManager) initializeDefaultChains() {
	defaultChains := []*Blockchain{
		{
			ID: "ethereum", ChainID: 1, Name: "Ethereum", Type: ChainTypeEVM,
			Symbol: "ETH", Decimals: 18, RPC: "https://eth.llamarpc.com",
			Explorer: "https://etherscan.io", Logo: "eth.png", IsNative: true,
			IsEnabled: true, AddedAt: time.Now().Unix(), AddedBy: "system",
			Capabilities: ChainCapabilities{Swap: true, Bridge: true, Staking: true, Farming: true, NFT: true, DappBrowser: true, MultiSig: true, HardwareWallet: true, Delegation: true, Governance: true},
			GasSettings: GasSettings{GasToken: "ETH", MinGasPrice: "20", MaxGasPrice: "500", AvgGasPrice: "30", GasLimitMultiplier: 1.2, EIP1559: true},
			Metadata: ChainMetadata{Color: "#627EEA", BgColor: "#627EEA20"},
		},
		{
			ID: "bnb-chain", ChainID: 56, Name: "BNB Chain", Type: ChainTypeEVM,
			Symbol: "BNB", Decimals: 18, RPC: "https://bsc.llamarpc.com",
			Explorer: "https://bscscan.com", Logo: "bnb.png", IsNative: true,
			IsEnabled: true, AddedAt: time.Now().Unix(), AddedBy: "system",
			Capabilities: ChainCapabilities{Swap: true, Bridge: true, Staking: true, Farming: true, NFT: true, DappBrowser: true, MultiSig: true, HardwareWallet: true, Delegation: true, Governance: true},
			GasSettings: GasSettings{GasToken: "BNB", MinGasPrice: "3", MaxGasPrice: "100", AvgGasPrice: "5", GasLimitMultiplier: 1.1, EIP1559: false},
			Metadata: ChainMetadata{Color: "#F3BA2F", BgColor: "#F3BA2F20"},
		},
		{
			ID: "polygon", ChainID: 137, Name: "Polygon", Type: ChainTypeEVM,
			Symbol: "MATIC", Decimals: 18, RPC: "https://polygon.llamarpc.com",
			Explorer: "https://polygonscan.com", Logo: "matic.png", IsNative: true,
			IsEnabled: true, AddedAt: time.Now().Unix(), AddedBy: "system",
			Capabilities: ChainCapabilities{Swap: true, Bridge: true, Staking: true, Farming: true, NFT: true, DappBrowser: true, MultiSig: true, HardwareWallet: true, Delegation: true, Governance: true},
			GasSettings: GasSettings{GasToken: "MATIC", MinGasPrice: "0.1", MaxGasPrice: "100", AvgGasPrice: "1", GasLimitMultiplier: 1.2, EIP1559: false},
			Metadata: ChainMetadata{Color: "#8247E5", BgColor: "#8247E520"},
		},
		{
			ID: "arbitrum", ChainID: 42161, Name: "Arbitrum One", Type: ChainTypeEVM,
			Symbol: "ETH", Decimals: 18, RPC: "https://arbitrum.llamarpc.com",
			Explorer: "https://arbiscan.io", Logo: "arb.png", IsNative: true,
			IsEnabled: true, AddedAt: time.Now().Unix(), AddedBy: "system",
			Capabilities: ChainCapabilities{Swap: true, Bridge: true, Staking: true, Farming: true, NFT: true, DappBrowser: true, MultiSig: true, HardwareWallet: true, Delegation: true, Governance: false},
			GasSettings: GasSettings{GasToken: "ETH", MinGasPrice: "0.1", MaxGasPrice: "50", AvgGasPrice: "0.2", GasLimitMultiplier: 1.3, EIP1559: true},
			Metadata: ChainMetadata{Color: "#28A0F0", BgColor: "#28A0F020"},
		},
		{
			ID: "optimism", ChainID: 10, Name: "Optimism", Type: ChainTypeEVM,
			Symbol: "ETH", Decimals: 18, RPC: "https://optimism.llamarpc.com",
			Explorer: "https://optimistic.etherscan.io", Logo: "op.png", IsNative: true,
			IsEnabled: true, AddedAt: time.Now().Unix(), AddedBy: "system",
			Capabilities: ChainCapabilities{Swap: true, Bridge: true, Staking: true, Farming: true, NFT: true, DappBrowser: true, MultiSig: true, HardwareWallet: true, Delegation: true, Governance: false},
			GasSettings: GasSettings{GasToken: "ETH", MinGasPrice: "0.001", MaxGasPrice: "10", AvgGasPrice: "0.005", GasLimitMultiplier: 1.2, EIP1559: true},
			Metadata: ChainMetadata{Color: "#FF0420", BgColor: "#FF042020"},
		},
		{
			ID: "avalanche", ChainID: 43114, Name: "Avalanche C-Chain", Type: ChainTypeEVM,
			Symbol: "AVAX", Decimals: 18, RPC: "https://avax.llamarpc.com",
			Explorer: "https://snowtrace.io", Logo: "avax.png", IsNative: true,
			IsEnabled: true, AddedAt: time.Now().Unix(), AddedBy: "system",
			Capabilities: ChainCapabilities{Swap: true, Bridge: true, Staking: true, Farming: true, NFT: true, DappBrowser: true, MultiSig: true, HardwareWallet: true, Delegation: true, Governance: true},
			GasSettings: GasSettings{GasToken: "AVAX", MinGasPrice: "25", MaxGasPrice: "500", AvgGasPrice: "30", GasLimitMultiplier: 1.1, EIP1559: false},
			Metadata: ChainMetadata{Color: "#E84142", BgColor: "#E8414220"},
		},
		{
			ID: "base", ChainID: 8453, Name: "Base", Type: ChainTypeEVM,
			Symbol: "ETH", Decimals: 18, RPC: "https://base.llamarpc.com",
			Explorer: "https://basescan.org", Logo: "base.png", IsNative: true,
			IsEnabled: true, AddedAt: time.Now().Unix(), AddedBy: "system",
			Capabilities: ChainCapabilities{Swap: true, Bridge: true, Staking: true, Farming: true, NFT: true, DappBrowser: true, MultiSig: true, HardwareWallet: true, Delegation: true, Governance: false},
			GasSettings: GasSettings{GasToken: "ETH", MinGasPrice: "0.01", MaxGasPrice: "100", AvgGasPrice: "0.1", GasLimitMultiplier: 1.2, EIP1559: true},
			Metadata: ChainMetadata{Color: "#0052FF", BgColor: "#0052FF20"},
		},
	}

	for _, chain := range defaultChains {
		m.chains[chain.ID] = chain
	}
}

// AddEVMChain adds a new EVM chain
func (m *BlockchainManager) AddEVMChain(data map[string]interface{}, addedBy string) (*Blockchain, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	chainID, ok := data["chainId"].(float64)
	if !ok {
		return nil, fmt.Errorf("invalid chainId")
	}

	name, _ := data["name"].(string)
	symbol, _ := data["symbol"].(string)
	rpc, _ := data["rpc"].(string)
	explorer, _ := data["explorer"].(string)

	chain := &Blockchain{
		ID:          fmt.Sprintf("chain-%d", int64(chainID)),
		ChainID:     int64(chainID),
		Name:        name,
		Type:        ChainTypeEVM,
		Symbol:      symbol,
		Decimals:    18,
		RPC:         rpc,
		Explorer:    explorer,
		IsNative:    true,
		IsEnabled:   true,
		AddedAt:     time.Now().Unix(),
		AddedBy:     addedBy,
		Capabilities: ChainCapabilities{
			Swap: true, Bridge: true, Staking: true, Farming: true, NFT: true,
			DappBrowser: true, MultiSig: true, HardwareWallet: true, Delegation: true, Governance: true,
		},
		GasSettings: GasSettings{
			GasToken: symbol, MinGasPrice: "1", MaxGasPrice: "1000",
			AvgGasPrice: "10", GasLimitMultiplier: 1.2, EIP1559: false,
		},
		Metadata: ChainMetadata{Color: "#888888", BgColor: "#88888820"},
	}

	m.chains[chain.ID] = chain
	return chain, nil
}

// GetAllChains returns all chains
func (m *BlockchainManager) GetAllChains() []*Blockchain {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Blockchain, 0, len(m.chains))
	for _, chain := range m.chains {
		result = append(result, chain)
	}
	return result
}

// GetEnabledChains returns enabled chains
func (m *BlockchainManager) GetEnabledChains() []*Blockchain {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Blockchain, 0)
	for _, chain := range m.chains {
		if chain.IsEnabled {
			result = append(result, chain)
		}
	}
	return result
}

// GetEVMChains returns only EVM chains
func (m *BlockchainManager) GetEVMChains() []*Blockchain {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Blockchain, 0)
	for _, chain := range m.chains {
		if chain.Type == ChainTypeEVM {
			result = append(result, chain)
		}
	}
	return result
}

// GetChain returns a chain by ID
func (m *BlockchainManager) GetChain(id string) *Blockchain {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.chains[id]
}

// UpdateChain updates a chain
func (m *BlockchainManager) UpdateChain(id string, updates map[string]interface{}) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	chain, ok := m.chains[id]
	if !ok {
		return false
	}

	if name, ok := updates["name"].(string); ok {
		chain.Name = name
	}
	if symbol, ok := updates["symbol"].(string); ok {
		chain.Symbol = symbol
	}
	if rpc, ok := updates["rpc"].(string); ok {
		chain.RPC = rpc
	}
	if enabled, ok := updates["isEnabled"].(bool); ok {
		chain.IsEnabled = enabled
	}

	return true
}

// ToggleChain enables or disables a chain
func (m *BlockchainManager) ToggleChain(id string, enabled bool) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	chain, ok := m.chains[id]
	if !ok {
		return false
	}

	chain.IsEnabled = enabled
	return true
}

// RemoveChain removes a chain
func (m *BlockchainManager) RemoveChain(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.chains[id]; !ok {
		return false
	}

	delete(m.chains, id)
	return true
}

// AddToken adds a token to a chain
func (m *BlockchainManager) AddToken(chainID string, token *Token) {
	m.mu.Lock()
	defer m.mu.Unlock()

	tokens := m.tokens[chainID]
	for i, t := range tokens {
		if t.Address == token.Address {
			tokens[i] = token
			return
		}
	}
	tokens = append(tokens, token)
	m.tokens[chainID] = tokens

	if chain, ok := m.chains[chainID]; ok {
		chain.Tokens = append(chain.Tokens, token.Address)
	}
}

// GetChainTokens returns tokens for a chain
func (m *BlockchainManager) GetChainTokens(chainID string) []*Token {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.tokens[chainID]
}

// ValidateRPC validates an RPC endpoint
func (m *BlockchainManager) ValidateRPC(rpcURL string) bool {
	// In production, would make actual RPC call
	return len(rpcURL) > 0
}

// GetSupportedChainTypes returns all supported chain types
func (m *BlockchainManager) GetSupportedChainTypes() []ChainType {
	return []ChainType{
		ChainTypeEVM, ChainTypeSolana, ChainTypeTron, ChainTypeSui,
		ChainTypeAptos, ChainTypeNear, ChainTypeCosmos,
	}
}

// ToJSON converts to JSON
func (m *BlockchainManager) ToJSON() ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return json.MarshalIndent(m.chains, "", "  ")
}

func main() {
	fmt.Println("TigerSwap Chain Management Service - Go")
	fmt.Println("========================================")
	fmt.Println()
	
	mgr := NewBlockchainManager()
	
	fmt.Printf("Initialized with %d chains\n", len(mgr.GetAllChains()))
	fmt.Println()
	
	// List enabled chains
	chains := mgr.GetEnabledChains()
	fmt.Println("Enabled Chains:")
	for _, chain := range chains {
		fmt.Printf("  - %s (%d) - %s\n", chain.Name, chain.ChainID, chain.Symbol)
	}
}