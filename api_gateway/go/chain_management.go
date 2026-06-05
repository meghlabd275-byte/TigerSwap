// TigerSwap Chain Management API - Go Implementation
// REST API for chain management

package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// ChainConfig chain configuration
type ChainConfig struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Symbol       string   `json:"symbol"`
	Category     string   `json:"category"`
	Status       string   `json:"status"`
	ChainID      int64    `json:"chainId"`
	NetworkID    int64    `json:"networkId,omitempty"`
	RPCURLs      []string `json:"rpcUrls"`
	ExplorerURLs []string `json:"explorerUrls"`
	NativeCurrency struct {
		Name     string `json:"name"`
		Symbol   string `json:"symbol"`
		Decimals int    `json:"decimals"`
		LogoURL  string `json:"logoUrl,omitempty"`
	} `json:"nativeCurrency"`
	BlockTime        int64   `json:"blockTime,omitempty"`
	GasLimit         int64   `json:"gasLimit,omitempty"`
	SupportsEIP1559  bool    `json:"supportsEIP1559"`
	SupportsFlashbots bool   `json:"supportsFlashbots"`
	SupportsMEV      bool    `json:"supportsMEV"`
	SupportsMulticall bool   `json:"supportsMulticall"`
	Notes            string  `json:"notes,omitempty"`
}

// ChainStats chain statistics
type ChainStats struct {
	TotalChains    int `json:"totalChains"`
	EVMChains      int `json:"evmChains"`
	NonEVMChains   int `json:"nonEVMChains"`
	ActiveChains   int `json:"activeChains"`
	TestnetChains   int `json:"testnetChains"`
}

// ChainRegistry chain registry
type ChainRegistry struct {
	chains map[string]*ChainConfig
}

func NewChainRegistry() *ChainRegistry {
	r := &ChainRegistry{
		chains: make(map[string]*ChainConfig),
	}
	r.initializeDefaultChains()
	return r
}

func (r *ChainRegistry) initializeDefaultChains() {
	defaultChains := []*ChainConfig{
		{
			ID: "ethereum", Name: "Ethereum", Symbol: "ETH", Category: "evm", Status: "active",
			ChainID: 1, RPCURLs: []string{"https://eth.llamarpc.com"}, ExplorerURLs: []string{"https://etherscan.io"},
			NativeCurrency: struct {
				Name     string `json:"name"`
				Symbol   string `json:"symbol"`
				Decimals int    `json:"decimals"`
				LogoURL  string `json:"logoUrl,omitempty"`
			}{Name: "Ether", Symbol: "ETH", Decimals: 18},
			BlockTime: 12, SupportsEIP1559: true, SupportsFlashbots: true, SupportsMEV: true, SupportsMulticall: true,
		},
		{
			ID: "bnb-smart-chain", Name: "BNB Chain", Symbol: "BNB", Category: "evm", Status: "active",
			ChainID: 56, RPCURLs: []string{"https://bsc.llamarpc.com"}, ExplorerURLs: []string{"https://bscscan.com"},
			NativeCurrency: struct {
				Name     string `json:"name"`
				Symbol   string `json:"symbol"`
				Decimals int    `json:"decimals"`
				LogoURL  string `json:"logoUrl,omitempty"`
			}{Name: "BNB", Symbol: "BNB", Decimals: 18},
			BlockTime: 3, SupportsEIP1559: false, SupportsFlashbots: false, SupportsMEV: false, SupportsMulticall: true,
		},
		{
			ID: "polygon", Name: "Polygon", Symbol: "MATIC", Category: "evm", Status: "active",
			ChainID: 137, RPCURLs: []string{"https://polygon.llamarpc.com"}, ExplorerURLs: []string{"https://polygonscan.com"},
			NativeCurrency: struct {
				Name     string `json:"name"`
				Symbol   string `json:"symbol"`
				Decimals int    `json:"decimals"`
				LogoURL  string `json:"logoUrl,omitempty"`
			}{Name: "MATIC", Symbol: "MATIC", Decimals: 18},
			BlockTime: 2, SupportsEIP1559: false, SupportsFlashbots: false, SupportsMEV: false, SupportsMulticall: true,
		},
		{
			ID: "arbitrum", Name: "Arbitrum One", Symbol: "ETH", Category: "evm", Status: "active",
			ChainID: 42161, RPCURLs: []string{"https://arbitrum.llamarpc.com"}, ExplorerURLs: []string{"https://arbiscan.io"},
			NativeCurrency: struct {
				Name     string `json:"name"`
				Symbol   string `json:"symbol"`
				Decimals int    `json:"decimals"`
				LogoURL  string `json:"logoUrl,omitempty"`
			}{Name: "Ether", Symbol: "ETH", Decimals: 18},
			BlockTime: 1, SupportsEIP1559: true, SupportsFlashbots: true, SupportsMEV: true, SupportsMulticall: true,
		},
	}

	for _, chain := range defaultChains {
		r.chains[chain.ID] = chain
	}
}

// AddChain adds a new chain
func (r *ChainRegistry) AddChain(config *ChainConfig) {
	r.chains[config.ID] = config
}

// GetChain gets a chain by ID
func (r *ChainRegistry) GetChain(id string) *ChainConfig {
	return r.chains[id]
}

// GetAllChains returns all chains
func (r *ChainRegistry) GetAllChains() []*ChainConfig {
	result := make([]*ChainConfig, 0, len(r.chains))
	for _, c := range r.chains {
		result = append(result, c)
	}
	return result
}

// GetChainsByCategory returns chains by category
func (r *ChainRegistry) GetChainsByCategory(category string) []*ChainConfig {
	result := make([]*ChainConfig, 0)
	for _, c := range r.chains {
		if c.Category == category {
			result = append(result, c)
		}
	}
	return result
}

// GetChainsByStatus returns chains by status
func (r *ChainRegistry) GetChainsByStatus(status string) []*ChainConfig {
	result := make([]*ChainConfig, 0)
	for _, c := range r.chains {
		if c.Status == status {
			result = append(result, c)
		}
	}
	return result
}

// UpdateChain updates a chain
func (r *ChainRegistry) UpdateChain(id string, updates map[string]interface{}) bool {
	chain, ok := r.chains[id]
	if !ok {
		return false
	}

	if name, ok := updates["name"].(string); ok {
		chain.Name = name
	}
	if rpcs, ok := updates["rpcUrls"].([]interface{}); ok {
		chain.RPCURLs = make([]string, len(rpcs))
		for i, r := range rpcs {
			chain.RPCURLs[i] = r.(string)
		}
	}
	if status, ok := updates["status"].(string); ok {
		chain.Status = status
	}

	return true
}

// RemoveChain removes a chain
func (r *ChainRegistry) RemoveChain(id string) bool {
	if _, ok := r.chains[id]; !ok {
		return false
	}
	delete(r.chains, id)
	return true
}

// GetChainStats returns chain statistics
func (r *ChainRegistry) GetChainStats() *ChainStats {
	stats := &ChainStats{}

	for _, chain := range r.chains {
		stats.TotalChains++
		if chain.Category == "evm" {
			stats.EVMChains++
		} else {
			stats.NonEVMChains++
		}
		if chain.Status == "active" {
			stats.ActiveChains++
		}
	}

	return stats
}

// SearchChains searches chains
func (r *ChainRegistry) SearchChains(query string) []*ChainConfig {
	queryLower := strings.ToLower(query)
	result := make([]*ChainConfig, 0)

	for _, chain := range r.chains {
		if strings.Contains(strings.ToLower(chain.Name), queryLower) ||
		   strings.Contains(strings.ToLower(chain.ID), queryLower) ||
		   strings.Contains(strings.ToLower(chain.Symbol), queryLower) {
			result = append(result, chain)
		}
	}

	return result
}

// GetBestRPC returns the best RPC for a chain
func (r *ChainRegistry) GetBestRPC(chainID string) string {
	chain, ok := r.chains[chainID]
	if !ok || len(chain.RPCURLs) == 0 {
		return ""
	}
	return chain.RPCURLs[0] // In production, would check health
}

// API Server

type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
	Count   int         `json:"count,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, resp *APIResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(resp)
}

func chainHandler(w http.ResponseWriter, r *http.Request) {
	registry := NewChainRegistry()

	switch r.Method {
	case "GET":
		chainID := r.URL.Query().Get("chainId")
		category := r.URL.Query().Get("category")
		status := r.URL.Query().Get("status")
		search := r.URL.Query().Get("search")
		statsOnly := r.URL.Query().Get("stats") == "true"

		if statsOnly {
			writeJSON(w, http.StatusOK, &APIResponse{
				Success: true,
				Data:    registry.GetChainStats(),
			})
			return
		}

		if chainID != "" {
			chain := registry.GetChain(chainID)
			if chain == nil {
				writeJSON(w, http.StatusNotFound, &APIResponse{
					Success: false,
					Error:   fmt.Sprintf("Chain %s not found", chainID),
				})
				return
			}
			writeJSON(w, http.StatusOK, &APIResponse{
				Success: true,
				Data:    chain,
			})
			return
		}

		if category != "" {
			writeJSON(w, http.StatusOK, &APIResponse{
				Success: true,
				Data:    registry.GetChainsByCategory(category),
				Count:  len(registry.GetChainsByCategory(category)),
			})
			return
		}

		if status != "" {
			writeJSON(w, http.StatusOK, &APIResponse{
				Success: true,
				Data:    registry.GetChainsByStatus(status),
				Count:  len(registry.GetChainsByStatus(status)),
			})
			return
		}

		if search != "" {
			writeJSON(w, http.StatusOK, &APIResponse{
				Success: true,
				Data:    registry.SearchChains(search),
				Count:  len(registry.SearchChains(search)),
			})
			return
		}

		writeJSON(w, http.StatusOK, &APIResponse{
			Success: true,
			Data:    registry.GetAllChains(),
			Count:   len(registry.GetAllChains()),
		})

	case "POST":
		var config ChainConfig
		if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
			writeJSON(w, http.StatusBadRequest, &APIResponse{
				Success: false,
				Error:   "Invalid request body",
			})
			return
		}

		registry.AddChain(&config)
		writeJSON(w, http.StatusCreated, &APIResponse{
			Success: true,
			Data:    map[string]string{"message": fmt.Sprintf("Chain %s added successfully", config.Name)},
		})

	case "PUT":
		var updates map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
			writeJSON(w, http.StatusBadRequest, &APIResponse{
				Success: false,
				Error:   "Invalid request body",
			})
			return
		}

		id := updates["id"].(string)
		if registry.UpdateChain(id, updates) {
			writeJSON(w, http.StatusOK, &APIResponse{
				Success: true,
				Data:    map[string]string{"message": fmt.Sprintf("Chain %s updated successfully", id)},
			})
		} else {
			writeJSON(w, http.StatusNotFound, &APIResponse{
				Success: false,
				Error:   fmt.Sprintf("Chain %s not found", id),
			})
		}

	case "DELETE":
		id := r.URL.Query().Get("id")
		if id == "" {
			writeJSON(w, http.StatusBadRequest, &APIResponse{
				Success: false,
				Error:   "Chain ID required",
			})
			return
		}

		if registry.RemoveChain(id) {
			writeJSON(w, http.StatusOK, &APIResponse{
				Success: true,
				Data:    map[string]string{"message": fmt.Sprintf("Chain %s deleted successfully", id)},
			})
		} else {
			writeJSON(w, http.StatusNotFound, &APIResponse{
				Success: false,
				Error:   fmt.Sprintf("Chain %s not found", id),
			})
		}
	}
}

func rpcHandler(w http.ResponseWriter, r *http.Request) {
	registry := NewChainRegistry()

	switch r.Method {
	case "GET":
		chainID := r.URL.Query().Get("chainId")
		best := r.URL.Query().Get("best") == "true"

		if chainID == "" {
			writeJSON(w, http.StatusBadRequest, &APIResponse{
				Success: false,
				Error:   "Chain ID required",
			})
			return
		}

		chain := registry.GetChain(chainID)
		if chain == nil {
			writeJSON(w, http.StatusNotFound, &APIResponse{
				Success: false,
				Error:   fmt.Sprintf("Chain %s not found", chainID),
			})
			return
		}

		if best {
			rpc := registry.GetBestRPC(chainID)
			if rpc == "" {
				writeJSON(w, http.StatusNotFound, &APIResponse{
					Success: false,
					Error:   "No healthy RPC endpoints",
				})
				return
			}
			writeJSON(w, http.StatusOK, &APIResponse{
				Success: true,
				Data:    map[string]interface{}{"rpc": rpc, "chainId": chainID},
			})
			return
		}

		writeJSON(w, http.StatusOK, &APIResponse{
			Success: true,
			Data: map[string]interface{}{
				"chainId":      chainID,
				"chainName":    chain.Name,
				"rpcs":         chain.RPCURLs,
				"explorerUrls": chain.ExplorerURLs,
			},
		})
	}
}

func main() {
	fmt.Println("TigerSwap Chain Management API - Go")
	fmt.Println("====================================")
	fmt.Println()
	fmt.Println("Server starting on :8080")
	fmt.Println()

	http.HandleFunc("/api/chains", chainHandler)
	http.HandleFunc("/api/chains/rpc", rpcHandler)

	server := &http.Server{
		Addr:         ":8080",
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	fmt.Println("Endpoints:")
	fmt.Println("  GET    /api/chains          - List all chains")
	fmt.Println("  GET    /api/chains?chainId=X - Get single chain")
	fmt.Println("  GET    /api/chains?stats=true - Get stats")
	fmt.Println("  POST   /api/chains           - Add new chain")
	fmt.Println("  PUT    /api/chains           - Update chain")
	fmt.Println("  DELETE /api/chains?id=X      - Remove chain")
	fmt.Println("  GET    /api/chains/rpc?chainId=X - Get RPCs")
	fmt.Println()

	if err := server.ListenAndServe(); err != nil {
		fmt.Printf("Server error: %v\n", err)
	}
}