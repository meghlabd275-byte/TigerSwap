package services

import (
	"testing"

	"tigerswap/backend/models"
)

// TestLoadChainRPCsRegistersStoredEndpoints pins that an endpoint the operator
// stored in the admin panel is actually handed to the live client, so the chain
// is reachable rather than answering "RPC URL not configured".
func TestLoadChainRPCsRegistersStoredEndpoints(t *testing.T) {
	db := newStatsTestDB(t)
	if err := db.Create(&models.Blockchain{
		ChainID: 6666, Name: "TigerSmartChain", Symbol: "TGR", Type: "evm",
		RPCURL: "https://rpc.tigerswap.io/6666", IsActive: true,
	}).Error; err != nil {
		t.Fatalf("seed chain: %v", err)
	}
	// An inactive chain's endpoint must not be dialled. The row is inserted with
	// a column map because `is_active` defaults to true and a struct insert would
	// drop the false value.
	if err := db.Model(&models.Blockchain{}).Create(map[string]interface{}{
		"chain_id": 9999, "name": "Retired", "symbol": "RET", "type": "evm",
		"rpc_url": "https://rpc.example.invalid/9999", "is_active": false,
	}).Error; err != nil {
		t.Fatalf("seed inactive chain: %v", err)
	}

	client := NewBlockchainClient()
	if err := client.LoadChainRPCs(db); err != nil {
		t.Fatalf("LoadChainRPCs: %v", err)
	}

	url, err := client.RPCURLForChain(6666)
	if err != nil {
		t.Fatalf("chain 6666 endpoint not registered: %v", err)
	}
	if url != "https://rpc.tigerswap.io/6666" {
		t.Fatalf("unexpected endpoint: %q", url)
	}

	if _, err := client.RPCURLForChain(9999); err == nil {
		t.Fatalf("inactive chain endpoint must not be registered")
	}
}

// TestLoadChainRPCsUnregistersRetiredChain pins the refresh semantics: a chain
// that is deactivated after the first load must stop being dialled, so a retired
// node is not kept alive until the next restart.
func TestLoadChainRPCsUnregistersRetiredChain(t *testing.T) {
	db := newStatsTestDB(t)
	if err := db.Create(&models.Blockchain{
		ChainID: 6666, Name: "TigerSmartChain", Symbol: "TGR", Type: "evm",
		RPCURL: "https://rpc.tigerswap.io/6666", IsActive: true,
	}).Error; err != nil {
		t.Fatalf("seed chain: %v", err)
	}

	client := NewBlockchainClient()
	if err := client.LoadChainRPCs(db); err != nil {
		t.Fatalf("LoadChainRPCs: %v", err)
	}
	if _, err := client.RPCURLForChain(6666); err != nil {
		t.Fatalf("chain 6666 should be registered: %v", err)
	}

	// Deactivate the chain, then refresh: the endpoint must be gone.
	if err := db.Model(&models.Blockchain{}).Where("chain_id = ?", 6666).
		Update("is_active", false).Error; err != nil {
		t.Fatalf("deactivate chain: %v", err)
	}
	if err := client.LoadChainRPCs(db); err != nil {
		t.Fatalf("reload: %v", err)
	}
	if _, err := client.RPCURLForChain(6666); err == nil {
		t.Fatalf("deactivated chain must be unregistered after refresh")
	}
}

// TestChainDTOExposesStoredRPCURL pins the wire contract: when the environment
// has no endpoint, the chain's stored endpoint is exposed so the wallet can add
// the network.
func TestChainDTOExposesStoredRPCURL(t *testing.T) {
	dto := newChainDTO(models.Blockchain{
		ChainID: 6666, Name: "TigerSmartChain", Symbol: "TGR", Type: "evm",
		RPCURL: "https://rpc.tigerswap.io/6666", IsActive: true,
	})
	if len(dto.RPCURLs) != 1 || dto.RPCURLs[0] != "https://rpc.tigerswap.io/6666" {
		t.Fatalf("stored rpc url not exposed: %#v", dto.RPCURLs)
	}

	empty := newChainDTO(models.Blockchain{
		ChainID: 1, Name: "Ethereum", Symbol: "ETH", Type: "evm", IsActive: true,
	})
	if empty.RPCURLs == nil {
		t.Fatalf("rpcUrls must be an empty array, never nil")
	}
	if len(empty.RPCURLs) != 0 {
		t.Fatalf("expected no endpoints, got %#v", empty.RPCURLs)
	}
}
