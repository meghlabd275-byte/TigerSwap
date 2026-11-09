package services

import (
	"encoding/json"
	"strings"
	"testing"

	"tigerswap/backend/models"
)

// TestCreateMasterWalletDoesNotLeakSeedPhrase proves the handler response never
// carries the seed phrase or the backup code inside the wallet object. Before
// this was fixed the whole MasterWallet struct was serialized, exposing the
// plaintext mnemonic on a protected-but-recordable response.
func TestCreateMasterWalletDoesNotLeakSeedPhrase(t *testing.T) {
	t.Setenv("WALLET_ENCRYPTION_KEY", strings.Repeat("k", 40))
	db := newWalletTestDB(t)
	if err := db.AutoMigrate(&models.MasterWallet{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	svc := NewWalletService(db)

	// A valid 12-word BIP39 mnemonic (the canonical all-zeros test vector).
	const mnemonic = "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"

	rec := runAuthenticatedHandler(t, svc.CreateMasterWallet, "11111111-1111-1111-1111-111111111111", map[string]interface{}{
		"seed_phrase":      mnemonic,
		"supported_chains": []int64{1},
	}, nil)

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	wallet, _ := body["wallet"].(map[string]interface{})
	if wallet == nil {
		t.Fatalf("response must carry a wallet object: %s", rec.Body.String())
	}
	if _, present := wallet["seed_phrase"]; present {
		t.Fatalf("the wallet object must not carry the seed phrase: %s", rec.Body.String())
	}
	if _, present := wallet["backup_code"]; present {
		t.Fatalf("the wallet object must not carry the backup code: %s", rec.Body.String())
	}

	// The seed phrase must not appear anywhere in the response, and the persisted
	// row must store it encrypted rather than in the clear.
	if strings.Contains(rec.Body.String(), mnemonic) {
		t.Fatalf("the response body must never contain the seed phrase")
	}
	var stored models.MasterWallet
	if err := db.Where("user_id = ?", "11111111-1111-1111-1111-111111111111").First(&stored).Error; err != nil {
		t.Fatalf("load stored wallet: %v", err)
	}
	if stored.SeedEncrypted == mnemonic || strings.Contains(stored.SeedEncrypted, "abandon") {
		t.Fatalf("the seed phrase must be stored encrypted, not in the clear")
	}
	if stored.EncryptedKey == "" || stored.BackupCodeHash == "" {
		t.Fatalf("key material and backup hash must be persisted")
	}
}
