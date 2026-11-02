package services

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

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

// TestAddNonEVMChainRegistersChain proves a non-EVM chain (whose addresses are
// not 20-byte hex) can be registered through the admin surface.
func TestAddNonEVMChainRegistersChain(t *testing.T) {
	svc := NewDynamicIntegrationService()

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

// TestActivateChainTogglesVisibility proves pausing a chain hides it from the
// active-only listing and resuming brings it back.
func TestActivateChainTogglesVisibility(t *testing.T) {
	svc := NewDynamicIntegrationService()

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
	svc := NewDynamicIntegrationService()

	rec := callJSON(t, svc.ActivateChainHandler, http.MethodPost, "/x", gin.H{"active": false},
		gin.Params{{Key: "key", Value: "does-not-exist"}})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

// TestDeleteChainHandlerRemovesChain proves deletion removes the registration and
// a second delete reports not found.
func TestDeleteChainHandlerRemovesChain(t *testing.T) {
	svc := NewDynamicIntegrationService()

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

// TestGetBridgesHandlerListsRegisteredBridges proves the listing handler returns
// the bridges that were registered.
func TestGetBridgesHandlerListsRegisteredBridges(t *testing.T) {
	svc := NewDynamicIntegrationService()

	if _, err := svc.AddBridge("tgr-to-testnet", "TGR to Testnet", "tigersmartchain", "tigersmartchain_testnet",
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
	if out.Count != 1 || len(out.Bridges) != 1 || out.Bridges[0].Key != "tgr-to-testnet" {
		t.Fatalf("bridges = %+v, want the single tgr-to-testnet bridge", out)
	}
}
