package services

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"tigerswap/backend/models"
)

func newWebhookTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s-%d?mode=memory&cache=shared", t.Name(), atomic.AddUint64(&testDBSeq, 1))
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

	if err := db.AutoMigrate(&models.Webhook{}, &models.WebhookDelivery{}); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}
	return db
}

// newPrivateAllowedWebhookService returns a service that may deliver to a
// loopback address, which is how the test's httptest server is reachable.
func newPrivateAllowedWebhookService(t *testing.T, db *gorm.DB) *WebhookService {
	t.Helper()
	t.Setenv("WEBHOOK_ALLOW_PRIVATE", "true")
	return NewWebhookService(db)
}

func TestWebhookSecretIsEncryptedAndRecoverable(t *testing.T) {
	t.Setenv("WALLET_ENCRYPTION_KEY", "0123456789abcdef0123456789abcdef")
	sealed, err := encryptWebhookSecret("whsec_supersecret")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if sealed == "whsec_supersecret" {
		t.Fatal("secret stored in plaintext")
	}
	got, err := decryptWebhookSecret(sealed)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if got != "whsec_supersecret" {
		t.Fatalf("roundtrip mismatch: %q", got)
	}
}

func TestWebhookSignatureMatchesReceiver(t *testing.T) {
	const secret = "whsec_abc"
	payload := `{"event":"deposit.confirmed"}`
	sig := webhookSignature(secret, payload)

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	want := hex.EncodeToString(mac.Sum(nil))
	if sig != want {
		t.Fatalf("signature mismatch: got %s want %s", sig, want)
	}
}

func TestWebhookPublishQueuesOnlySubscribed(t *testing.T) {
	db := newWebhookTestDB(t)
	svc := newPrivateAllowedWebhookService(t, db)

	user := uuid.NewString()
	hooks := []models.Webhook{
		{UserID: user, URL: "https://example.com/a", Events: WebhookEventDepositConfirmed, IsActive: true},
		{UserID: user, URL: "https://example.com/b", Events: WebhookEventWithdrawalSent, IsActive: true},
		{UserID: user, URL: "https://example.com/c", Events: "*", IsActive: true},
		{UserID: user, URL: "https://example.com/d", Events: WebhookEventDepositConfirmed, IsActive: true},
	}
	for i := range hooks {
		if err := db.Create(&hooks[i]).Error; err != nil {
			t.Fatalf("create hook: %v", err)
		}
	}
	// The disabled endpoint must be off. GORM omits a false value on a
	// `default:true` column, so the flag is cleared with an explicit update.
	if err := db.Model(&models.Webhook{}).Where("id = ?", hooks[3].ID).Update("is_active", false).Error; err != nil {
		t.Fatalf("disable hook: %v", err)
	}

	if err := svc.Publish(user, WebhookEventDepositConfirmed, map[string]string{"amount": "100"}); err != nil {
		t.Fatalf("publish: %v", err)
	}

	var deliveries []models.WebhookDelivery
	if err := db.Find(&deliveries).Error; err != nil {
		t.Fatalf("list deliveries: %v", err)
	}
	// The deposit subscriber and the wildcard subscriber, not the withdrawal
	// subscriber and not the disabled endpoint.
	if len(deliveries) != 2 {
		t.Fatalf("expected 2 deliveries, got %d", len(deliveries))
	}
	for _, d := range deliveries {
		if d.Status != models.WebhookDeliveryPending {
			t.Fatalf("delivery not pending: %s", d.Status)
		}
		if d.Event != WebhookEventDepositConfirmed {
			t.Fatalf("wrong event: %s", d.Event)
		}
	}
}

func TestWebhookDispatchDeliversSignedPayload(t *testing.T) {
	t.Setenv("WALLET_ENCRYPTION_KEY", "0123456789abcdef0123456789abcdef")
	db := newWebhookTestDB(t)
	svc := newPrivateAllowedWebhookService(t, db)

	const secret = "whsec_receiver_secret"
	sealed, err := encryptWebhookSecret(secret)
	if err != nil {
		t.Fatalf("seal: %v", err)
	}

	type received struct {
		body      []byte
		signature string
		delivery  string
		event     string
	}
	got := make(chan received, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		got <- received{
			body:      body,
			signature: r.Header.Get("X-TigerSwap-Signature"),
			delivery:  r.Header.Get("X-TigerSwap-Delivery"),
			event:     r.Header.Get("X-TigerSwap-Event"),
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	user := uuid.NewString()
	hook := models.Webhook{UserID: user, URL: srv.URL, Events: "*", SecretEncrypted: sealed, IsActive: true}
	if err := db.Create(&hook).Error; err != nil {
		t.Fatalf("create hook: %v", err)
	}
	if err := svc.Publish(user, WebhookEventDepositConfirmed, map[string]string{"amount": "42"}); err != nil {
		t.Fatalf("publish: %v", err)
	}

	svc.sweep(t.Context())

	select {
	case r := <-got:
		if r.event != WebhookEventDepositConfirmed {
			t.Fatalf("wrong event header: %s", r.event)
		}
		wantSig := "sha256=" + webhookSignature(secret, string(r.body))
		if r.signature != wantSig {
			t.Fatalf("signature mismatch: got %s want %s", r.signature, wantSig)
		}
		var env map[string]interface{}
		if err := json.Unmarshal(r.body, &env); err != nil {
			t.Fatalf("payload not json: %v", err)
		}
		if env["event"] != WebhookEventDepositConfirmed {
			t.Fatalf("payload event wrong: %v", env["event"])
		}
	case <-time.After(3 * time.Second):
		t.Fatal("receiver did not get a delivery")
	}

	var d models.WebhookDelivery
	if err := db.First(&d).Error; err != nil {
		t.Fatalf("load delivery: %v", err)
	}
	if d.Status != models.WebhookDeliverySent {
		t.Fatalf("delivery not sent: %s (%s)", d.Status, d.LastError)
	}
	if d.DeliveredAt == nil {
		t.Fatal("delivered_at not set")
	}

	var reloaded models.Webhook
	if err := db.First(&reloaded, "id = ?", hook.ID).Error; err != nil {
		t.Fatalf("reload hook: %v", err)
	}
	if reloaded.LastSuccess == nil {
		t.Fatal("last_success not recorded")
	}
}

func TestWebhookDispatchRetriesThenFails(t *testing.T) {
	t.Setenv("WALLET_ENCRYPTION_KEY", "0123456789abcdef0123456789abcdef")
	db := newWebhookTestDB(t)
	svc := newPrivateAllowedWebhookService(t, db)

	sealed, err := encryptWebhookSecret("whsec_x")
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	user := uuid.NewString()
	hook := models.Webhook{UserID: user, URL: srv.URL, Events: "*", SecretEncrypted: sealed, IsActive: true}
	if err := db.Create(&hook).Error; err != nil {
		t.Fatalf("create hook: %v", err)
	}
	if err := svc.Publish(user, WebhookEventWithdrawalSent, map[string]string{"tx_hash": "0xabc"}); err != nil {
		t.Fatalf("publish: %v", err)
	}

	// Five attempts exhausts MaxAttempts (5), after which the row is failed.
	for i := 0; i < 5; i++ {
		// Force the next-attempt time into the past so the sweep picks it up.
		db.Model(&models.WebhookDelivery{}).Where("1 = 1").Update("next_attempt_at", time.Now().Add(-time.Second))
		svc.sweep(t.Context())
	}

	var d models.WebhookDelivery
	if err := db.First(&d).Error; err != nil {
		t.Fatalf("load delivery: %v", err)
	}
	if d.Status != models.WebhookDeliveryFailed {
		t.Fatalf("expected failed after retries, got %s", d.Status)
	}
	if d.Attempts < 5 {
		t.Fatalf("expected at least 5 attempts, got %d", d.Attempts)
	}
	if d.LastStatusCode != http.StatusInternalServerError {
		t.Fatalf("expected last status 500, got %d", d.LastStatusCode)
	}
}

func TestWebhookRejectsPrivateURL(t *testing.T) {
	db := newWebhookTestDB(t)
	// Default service: private addresses refused.
	t.Setenv("WEBHOOK_ALLOW_PRIVATE", "false")
	svc := NewWebhookService(db)

	for _, u := range []string{
		"http://127.0.0.1/hook",
		"http://169.254.169.254/latest/meta-data",
		"http://10.0.0.5/hook",
		"http://192.168.1.1/hook",
		"ftp://example.com/hook",
	} {
		if err := svc.validateURL(u); err == nil {
			t.Fatalf("expected %s to be rejected", u)
		}
	}
	if err := svc.validateURL("https://example.com/hook"); err != nil {
		t.Fatalf("public url rejected: %v", err)
	}
}

func TestNormalizeWebhookEvents(t *testing.T) {
	if got, err := normalizeWebhookEvents(""); err != nil || got != "*" {
		t.Fatalf("empty should be wildcard, got %q %v", got, err)
	}
	if _, err := normalizeWebhookEvents("not.an.event"); err == nil {
		t.Fatal("unknown event should be rejected")
	}
	got, err := normalizeWebhookEvents("deposit.confirmed, deposit.confirmed ,withdrawal.sent")
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if got != "deposit.confirmed,withdrawal.sent" {
		t.Fatalf("dedupe/trim failed: %q", got)
	}
}
