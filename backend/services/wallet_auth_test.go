package services

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"tigerswap/backend/models"
)

func newWalletAuthHarness(t *testing.T) (*AuthService, *gorm.DB) {
	t.Helper()
	t.Setenv("JWT_SECRET", "wallet-auth-test-secret-0123456789abc")
	db := newListingTestDB(t)
	if err := db.AutoMigrate(&models.User{}, &models.RevokedToken{}, &models.WalletLoginNonce{}, &models.RefreshToken{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return NewAuthService(db), db
}

func walletPost(t *testing.T, route string, handler gin.HandlerFunc, body string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST(route, handler)
	request := httptest.NewRequest(http.MethodPost, route, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Host = "tigerswap.test"
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

// signMessage produces a real 65-byte signature with the recovery id in the
// 27/28 range a wallet returns.
func signMessage(t *testing.T, keyHex, message string) string {
	t.Helper()
	key, err := crypto.HexToECDSA(strings.TrimPrefix(keyHex, "0x"))
	if err != nil {
		t.Fatalf("parse key: %v", err)
	}
	hash := accounts.TextHash([]byte(message))
	sig, err := crypto.Sign(hash, key)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	sig[64] += 27
	return "0x" + hex.EncodeToString(sig)
}

func requestNonce(t *testing.T, auth *AuthService, address string) (nonce string, message string) {
	t.Helper()
	resp := walletPost(t, "/auth/wallet/nonce", auth.RequestWalletNonce,
		fmt.Sprintf(`{"address":%q,"chain_id":1}`, address))
	if resp.Code != http.StatusOK {
		t.Fatalf("nonce request: want 200, got %d (%s)", resp.Code, resp.Body.String())
	}
	var body struct {
		Nonce   string `json:"nonce"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode nonce: %v", err)
	}
	return body.Nonce, body.Message
}

// TestWalletSignInCreatesAndReusesAccount proves a valid signature authenticates
// the wallet and that a second sign-in resolves the same account.
func TestWalletSignInCreatesAndReusesAccount(t *testing.T) {
	auth, db := newWalletAuthHarness(t)
	key, _ := crypto.GenerateKey()
	keyHex := hex.EncodeToString(crypto.FromECDSA(key))
	address := strings.ToLower(crypto.PubkeyToAddress(key.PublicKey).Hex())

	_, message := requestNonce(t, auth, address)
	signature := signMessage(t, keyHex, message)

	body := fmt.Sprintf(`{"message":%q,"signature":%q}`, message, signature)
	resp := walletPost(t, "/auth/wallet/verify", auth.VerifyWalletSignature, body)
	if resp.Code != http.StatusOK {
		t.Fatalf("verify: want 200, got %d (%s)", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), "token") {
		t.Fatalf("verify must issue a token: %s", resp.Body.String())
	}

	var count int64
	if err := db.Model(&models.User{}).Where("wallet_address = ?", address).Count(&count).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("want exactly one account for the wallet, got %d", count)
	}

	// Second sign-in resolves the same account rather than creating a new one.
	_, message2 := requestNonce(t, auth, address)
	signature2 := signMessage(t, keyHex, message2)
	resp2 := walletPost(t, "/auth/wallet/verify", auth.VerifyWalletSignature,
		fmt.Sprintf(`{"message":%q,"signature":%q}`, message2, signature2))
	if resp2.Code != http.StatusOK {
		t.Fatalf("second verify: want 200, got %d (%s)", resp2.Code, resp2.Body.String())
	}
	if err := db.Model(&models.User{}).Where("wallet_address = ?", address).Count(&count).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("second sign-in must not create a new account, got %d", count)
	}
}

// TestWalletSignInRejectsReplayedNonce proves a nonce can only be spent once.
func TestWalletSignInRejectsReplayedNonce(t *testing.T) {
	auth, _ := newWalletAuthHarness(t)
	key, _ := crypto.GenerateKey()
	keyHex := hex.EncodeToString(crypto.FromECDSA(key))
	address := strings.ToLower(crypto.PubkeyToAddress(key.PublicKey).Hex())

	_, message := requestNonce(t, auth, address)
	signature := signMessage(t, keyHex, message)
	body := fmt.Sprintf(`{"message":%q,"signature":%q}`, message, signature)

	if resp := walletPost(t, "/auth/wallet/verify", auth.VerifyWalletSignature, body); resp.Code != http.StatusOK {
		t.Fatalf("first verify: want 200, got %d (%s)", resp.Code, resp.Body.String())
	}
	if resp := walletPost(t, "/auth/wallet/verify", auth.VerifyWalletSignature, body); resp.Code != http.StatusUnauthorized {
		t.Fatalf("replayed nonce must be 401, got %d (%s)", resp.Code, resp.Body.String())
	}
}

// TestWalletSignInRejectsWrongSigner proves a signature from a different key
// cannot authenticate the claimed address.
func TestWalletSignInRejectsWrongSigner(t *testing.T) {
	auth, _ := newWalletAuthHarness(t)
	victim, _ := crypto.GenerateKey()
	attacker, _ := crypto.GenerateKey()
	address := strings.ToLower(crypto.PubkeyToAddress(victim.PublicKey).Hex())

	_, message := requestNonce(t, auth, address)
	// The attacker signs the victim's challenge with their own key.
	signature := signMessage(t, hex.EncodeToString(crypto.FromECDSA(attacker)), message)

	resp := walletPost(t, "/auth/wallet/verify", auth.VerifyWalletSignature,
		fmt.Sprintf(`{"message":%q,"signature":%q}`, message, signature))
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("wrong signer must be 401, got %d (%s)", resp.Code, resp.Body.String())
	}
}

// TestWalletSignInRejectsExpiredNonce proves an expired challenge cannot be
// exchanged for a session.
func TestWalletSignInRejectsExpiredNonce(t *testing.T) {
	auth, db := newWalletAuthHarness(t)
	key, _ := crypto.GenerateKey()
	keyHex := hex.EncodeToString(crypto.FromECDSA(key))
	address := strings.ToLower(crypto.PubkeyToAddress(key.PublicKey).Hex())

	_, message := requestNonce(t, auth, address)
	signature := signMessage(t, keyHex, message)

	// Force the stored nonce and the signed expiry into the past.
	now := time.Now().UTC().Add(-2 * time.Hour)
	if err := db.Model(&models.WalletLoginNonce{}).Where("address = ?", address).
		Update("expires_at", now).Error; err != nil {
		t.Fatalf("expire nonce: %v", err)
	}
	resp := walletPost(t, "/auth/wallet/verify", auth.VerifyWalletSignature,
		fmt.Sprintf(`{"message":%q,"signature":%q}`, message, signature))
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expired nonce must be 401, got %d (%s)", resp.Code, resp.Body.String())
	}
}

// TestWalletSignInRejectsForeignDomain proves a message minted for another host
// is refused, which is what stops a phishing site from replaying a signature.
func TestWalletSignInRejectsForeignDomain(t *testing.T) {
	auth, _ := newWalletAuthHarness(t)
	key, _ := crypto.GenerateKey()
	keyHex := hex.EncodeToString(crypto.FromECDSA(key))
	address := strings.ToLower(crypto.PubkeyToAddress(key.PublicKey).Hex())

	// Build a message for evil.test, signed correctly, but verified against the
	// server's own domain.
	issued := time.Now().UTC()
	message := buildSIWEMessage("evil.test", address, "Sign in to TigerSwap.", "abcdef", 1, issued, issued.Add(5*time.Minute))
	signature := signMessage(t, keyHex, message)

	// Seed the nonce so only the domain check can fail.
	if err := auth.db.Create(&models.WalletLoginNonce{
		Address:   address,
		Nonce:     "abcdef",
		Statement: "Sign in to TigerSwap.",
		ChainID:   1,
		Domain:    "evil.test",
		ExpiresAt: issued.Add(5 * time.Minute),
	}).Error; err != nil {
		t.Fatalf("seed nonce: %v", err)
	}

	resp := walletPost(t, "/auth/wallet/verify", auth.VerifyWalletSignature,
		fmt.Sprintf(`{"message":%q,"signature":%q}`, message, signature))
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("foreign domain must be 401, got %d (%s)", resp.Code, resp.Body.String())
	}
}
