package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"tigerswap/backend/models"
)

var integrationTestDBSeq uint64

// newIntegrationTestService returns a DB-backed registry over a fresh in-memory
// database. The registry must not fabricate any entry, so a brand-new service
// is expected to be empty.
func newIntegrationTestService(t *testing.T) *DynamicIntegrationService {
	t.Helper()
	dsn := fmt.Sprintf("file:integration-%s-%d?mode=memory&cache=shared", t.Name(), atomic.AddUint64(&integrationTestDBSeq, 1))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db handle: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { sqlDB.Close() })

	if err := db.AutoMigrate(
		&models.IntegrationChain{}, &models.IntegrationToken{},
		&models.IntegrationBridge{}, &models.IntegrationFarm{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return NewDynamicIntegrationService(db)
}

// callJSON drives a handler with a JSON body and an explicit HTTP method and
// returns the recorder. postJSON only issues POSTs; the integration admin
// surface also exposes DELETE and path parameters.
func callJSON(t *testing.T, handler gin.HandlerFunc, method, path string, body interface{}, params gin.Params) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)

	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(raw)
	}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(method, path, reader)
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = params
	handler(c)
	return rec
}

// TestRegistryStartsEmpty proves the registry fabricates no chain, token or
// farm: a fresh install must list nothing until an operator registers one.
func TestRegistryStartsEmpty(t *testing.T) {
	svc := newIntegrationTestService(t)

	if chains := svc.GetAllChains(false); len(chains) != 0 {
		t.Fatalf("chains = %d, want 0 (no seeded default)", len(chains))
	}
	if tokens := svc.GetAllTokens("", false); len(tokens) != 0 {
		t.Fatalf("tokens = %d, want 0", len(tokens))
	}
	stats := svc.GetStats()
	if stats.TotalChains != 0 || stats.TotalTokens != 0 || stats.TotalFarms != 0 {
		t.Fatalf("stats = %+v, want all zero", stats)
	}
}

// TestAddEVMChainPersists proves a registered chain is written to the database
// and is visible to a second service instance over the same DB — a restart
// must not lose it.
func TestAddEVMChainPersists(t *testing.T) {
	svc := newIntegrationTestService(t)

	if _, err := svc.AddEVMChain("ethereum", "Ethereum", "1", "https://rpc.tigerswap.example/eth", "https://etherscan.io", "ETH", "0x0000000000000000000000000000000000000000", 18, "operator"); err != nil {
		t.Fatalf("AddEVMChain: %v", err)
	}

	// A second service over the same DB stands in for a restarted process.
	restarted := NewDynamicIntegrationService(svc.db)
	chain, err := restarted.GetChain("ethereum")
	if err != nil {
		t.Fatalf("GetChain after restart: %v", err)
	}
	if chain.ChainID != 1 || !chain.IsActive {
		t.Fatalf("chain = %+v, want active chain 1", chain)
	}
}

// TestAddEVMChainRejectsBadRPC proves a malformed RPC URL fails at registration.
func TestAddEVMChainRejectsBadRPC(t *testing.T) {
	svc := newIntegrationTestService(t)

	if _, err := svc.AddEVMChain("bad", "Bad", "1", "not-a-url", "https://x", "BAD", "", 18, "op"); err == nil {
		t.Fatal("expected a non-absolute rpc_url to be rejected")
	}
}

// TestAddDuplicateChainRejected proves a duplicate key is a conflict, not a
// silent overwrite.
func TestAddDuplicateChainRejected(t *testing.T) {
	svc := newIntegrationTestService(t)
	if _, err := svc.AddEVMChain("base", "Base", "8453", "https://rpc.base", "https://basescan.org", "ETH", "", 18, "op"); err != nil {
		t.Fatalf("first add: %v", err)
	}
	if _, err := svc.AddEVMChain("base", "Base Again", "8453", "https://rpc.base", "https://basescan.org", "ETH", "", 18, "op"); err == nil {
		t.Fatal("expected a duplicate chain key to be rejected")
	}
}

// TestAddNonEVMChainRegistersChain proves a non-EVM chain (whose addresses are
// not 20-byte hex) can be registered through the admin surface.
func TestAddNonEVMChainRegistersChain(t *testing.T) {
	svc := newIntegrationTestService(t)

	rec := callJSON(t, svc.AddNonEVMChainHandler, http.MethodPost, "/api/v1/admin/integration/chains/non-evm", gin.H{
		"key":          "solana",
		"name":         "Solana",
		"type":         "solana",
		"rpc_url":      "https://rpc.tigerswap.example/solana",
		"explorer_url": "https://explorer.tigerswap.example/solana",
		"symbol":       "SOL",
		"decimals":     9,
		"added_by":     "operator",
	}, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	chain, err := svc.GetChain("solana")
	if err != nil {
		t.Fatalf("GetChain: %v", err)
	}
	if chain.Type != "solana" || !chain.IsActive {
		t.Fatalf("chain = %+v, want active solana", chain)
	}
}

// TestAddNonEVMChainRejectsUnknownType proves an unsupported chain type is
// rejected rather than stored and later routed to a client that cannot speak it.
func TestAddNonEVMChainRejectsUnknownType(t *testing.T) {
	svc := newIntegrationTestService(t)
	if _, err := svc.AddNonEVMChain("weird", "Weird", "fictional", "https://rpc.x", "https://x", "W", 9, "op"); err == nil {
		t.Fatal("expected an unsupported chain type to be rejected")
	}
}

// TestActivateChainTogglesVisibility proves pausing a chain hides it from the
// active-only listing and resuming brings it back.
func TestActivateChainTogglesVisibility(t *testing.T) {
	svc := newIntegrationTestService(t)

	if rec := callJSON(t, svc.AddNonEVMChainHandler, http.MethodPost, "/x", gin.H{
		"key": "sui", "name": "Sui", "type": "sui", "rpc_url": "https://rpc.sui",
		"explorer_url": "https://explorer.sui", "symbol": "SUI", "decimals": 9,
	}, nil); rec.Code != http.StatusOK {
		t.Fatalf("add chain status = %d, body = %s", rec.Code, rec.Body.String())
	}

	rec := callJSON(t, svc.ActivateChainHandler, http.MethodPost, "/x", gin.H{"active": false},
		gin.Params{{Key: "key", Value: "sui"}})
	if rec.Code != http.StatusOK {
		t.Fatalf("deactivate status = %d, body = %s", rec.Code, rec.Body.String())
	}

	for _, chain := range svc.GetAllChains(true) {
		if chain.Key == "sui" {
			t.Fatal("paused chain still appears in the active-only listing")
		}
	}

	rec = callJSON(t, svc.ActivateChainHandler, http.MethodPost, "/x", gin.H{"active": true},
		gin.Params{{Key: "key", Value: "sui"}})
	if rec.Code != http.StatusOK {
		t.Fatalf("reactivate status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var found bool
	for _, chain := range svc.GetAllChains(true) {
		if chain.Key == "sui" {
			found = true
		}
	}
	if !found {
		t.Fatal("reactivated chain missing from the active-only listing")
	}
}

// TestActivateChainRejectsUnknownKey proves an unknown key is a 404 rather than
// a silent success.
func TestActivateChainRejectsUnknownKey(t *testing.T) {
	svc := newIntegrationTestService(t)

	rec := callJSON(t, svc.ActivateChainHandler, http.MethodPost, "/x", gin.H{"active": false},
		gin.Params{{Key: "key", Value: "does-not-exist"}})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

// TestDeleteChainHandlerRemovesChain proves deletion removes the registration and
// a second delete reports not found.
func TestDeleteChainHandlerRemovesChain(t *testing.T) {
	svc := newIntegrationTestService(t)

	if rec := callJSON(t, svc.AddNonEVMChainHandler, http.MethodPost, "/x", gin.H{
		"key": "aptos", "name": "Aptos", "type": "aptos", "rpc_url": "https://rpc.aptos",
		"explorer_url": "https://explorer.aptos", "symbol": "APT", "decimals": 8,
	}, nil); rec.Code != http.StatusOK {
		t.Fatalf("add chain status = %d, body = %s", rec.Code, rec.Body.String())
	}

	rec := callJSON(t, svc.DeleteChainHandler, http.MethodDelete, "/x", nil,
		gin.Params{{Key: "key", Value: "aptos"}})
	if rec.Code != http.StatusOK {
		t.Fatalf("delete status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if _, err := svc.GetChain("aptos"); err == nil {
		t.Fatal("chain still present after delete")
	}

	rec = callJSON(t, svc.DeleteChainHandler, http.MethodDelete, "/x", nil,
		gin.Params{{Key: "key", Value: "aptos"}})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("second delete status = %d, want 404", rec.Code)
	}
}

// TestTokenRequiresRegisteredChain proves a token cannot be attached to a chain
// that was never registered.
func TestTokenRequiresRegisteredChain(t *testing.T) {
	svc := newIntegrationTestService(t)
	if _, err := svc.AddToken("ghost", "0xabc", "no-such-chain", "Ghost", "GH", "ERC20", 18, "0", "op"); err == nil {
		t.Fatal("expected a token on an unregistered chain to be rejected")
	}
}

// TestGetBridgesHandlerListsRegisteredBridges proves the listing handler returns
// the bridges that were registered.
func TestGetBridgesHandlerListsRegisteredBridges(t *testing.T) {
	svc := newIntegrationTestService(t)

	if _, err := svc.AddNonEVMChain("solana", "Solana", "solana", "https://rpc.sol", "https://explorer.sol", "SOL", 9, "op"); err != nil {
		t.Fatalf("add solana: %v", err)
	}
	if _, err := svc.AddNonEVMChain("bitcoin", "Bitcoin", "bitcoin", "https://rpc.btc", "https://explorer.btc", "BTC", 8, "op"); err != nil {
		t.Fatalf("add bitcoin: %v", err)
	}

	if _, err := svc.AddBridge("sol-btc", "SOL to BTC", "solana", "bitcoin",
		"0xRouter", "0.01", "100", 0.1, "0", "15 minutes"); err != nil {
		t.Fatalf("AddBridge: %v", err)
	}

	rec := callJSON(t, svc.GetBridgesHandler, http.MethodGet, "/x", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var out struct {
		Bridges []*DynamicBridge `json:"bridges"`
		Count   int              `json:"count"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Count != 1 || len(out.Bridges) != 1 || out.Bridges[0].Key != "sol-btc" {
		t.Fatalf("bridges = %+v, want the single sol-btc bridge", out)
	}
}
