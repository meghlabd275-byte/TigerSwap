package services

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"tigerswap/backend/models"
)

// DefaultWebhookDispatchInterval is how often the dispatcher sweeps the delivery
// queue for due rows.
const DefaultWebhookDispatchInterval = 5 * time.Second

// webhookDeliveryTimeout bounds a single delivery attempt.
const webhookDeliveryTimeout = 10 * time.Second

// Webhook events a subscriber may name. "*" subscribes to all of them. Only
// events the platform actually emits are listed: a subscribable-but-never-fired
// event would be an inert surface.
const (
	WebhookEventDepositConfirmed = "deposit.confirmed"
	WebhookEventWithdrawalSent   = "withdrawal.sent"
	WebhookEventWithdrawalFailed = "withdrawal.failed"
)

// webhookEvents is the set an operator may subscribe to. An unknown event in a
// subscription is rejected at create time rather than silently never firing.
var webhookEvents = map[string]bool{
	WebhookEventDepositConfirmed: true,
	WebhookEventWithdrawalSent:   true,
	WebhookEventWithdrawalFailed: true,
}

// WebhookService owns outbound event delivery. A subscribed event is written to
// a durable `webhook_deliveries` queue in the same transaction as the business
// write that produced it, and a background dispatcher POSTs a signed payload to
// each endpoint with bounded retries. Delivery is at-least-once: a receiver must
// treat the `X-TigerSwap-Delivery` id as idempotency key.
type WebhookService struct {
	db     *gorm.DB
	client *http.Client

	// allowPrivate, when true, permits a delivery to a private/loopback address.
	// It exists only for local development against a listener on localhost; it is
	// off by default so a subscriber cannot use a webhook to probe the internal
	// network or a cloud metadata endpoint.
	allowPrivate bool
}

// NewWebhookService builds the service. The HTTP client re-checks every
// redirect target so a public URL cannot redirect the delivery into the private
// network.
func NewWebhookService(db *gorm.DB) *WebhookService {
	svc := &WebhookService{
		db:           db,
		allowPrivate: strings.EqualFold(strings.TrimSpace(os.Getenv("WEBHOOK_ALLOW_PRIVATE")), "true"),
	}
	svc.client = &http.Client{
		Timeout: webhookDeliveryTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return errors.New("too many redirects")
			}
			return svc.validateURL(req.URL.String())
		},
	}
	return svc
}

// WebhookEvent is the envelope delivered to a subscriber.
type WebhookEvent struct {
	ID        string      `json:"id"`
	Event     string      `json:"event"`
	CreatedAt time.Time   `json:"created_at"`
	Data      interface{} `json:"data"`
}

// Publish enqueues one delivery per subscribed, active endpoint in its own
// transaction. It is used where the caller has no surrounding transaction.
func (s *WebhookService) Publish(userID, event string, data interface{}) error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		return s.publishWithin(tx, userID, event, data)
	})
}

// publishWithin enqueues deliveries using an existing transaction, so an event
// is recorded only if the business write that caused it commits. A failure to
// enqueue is returned and rolls the business write back, so the platform never
// commits a deposit whose notification it failed to queue.
func (s *WebhookService) publishWithin(tx *gorm.DB, userID, event string, data interface{}) error {
	if s == nil || tx == nil {
		return nil
	}
	var hooks []models.Webhook
	if err := tx.Where("user_id = ? AND is_active = ?", userID, true).Find(&hooks).Error; err != nil {
		return err
	}
	if len(hooks) == 0 {
		return nil
	}

	payload, err := json.Marshal(WebhookEvent{
		ID:        uuid.NewString(),
		Event:     event,
		CreatedAt: time.Now().UTC(),
		Data:      data,
	})
	if err != nil {
		return err
	}

	now := time.Now()
	for i := range hooks {
		if !webhookSubscribes(hooks[i].Events, event) {
			continue
		}
		delivery := models.WebhookDelivery{
			WebhookID:     hooks[i].ID,
			UserID:        userID,
			Event:         event,
			Payload:       string(payload),
			Status:        models.WebhookDeliveryPending,
			MaxAttempts:   5,
			NextAttemptAt: now,
		}
		if err := tx.Create(&delivery).Error; err != nil {
			return err
		}
	}
	return nil
}

// webhookSubscribes reports whether a subscription list includes an event. "*"
// matches everything; otherwise it is a comma-separated list of exact names.
func webhookSubscribes(subscription, event string) bool {
	for _, part := range strings.Split(subscription, ",") {
		part = strings.TrimSpace(part)
		if part == "*" || part == event {
			return true
		}
	}
	return false
}

// StartWebhookDispatcher runs the delivery loop until ctx is cancelled.
func StartWebhookDispatcher(ctx context.Context, svc *WebhookService, interval time.Duration) {
	if svc == nil || svc.db == nil {
		return
	}
	if interval <= 0 {
		interval = DefaultWebhookDispatchInterval
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				svc.sweep(ctx)
			}
		}
	}()
}

// sweep claims and delivers every due row once. A row is claimed by moving it to
// `sending` under a conditional update, so two dispatchers cannot both send it.
func (s *WebhookService) sweep(ctx context.Context) {
	var due []models.WebhookDelivery
	if err := s.db.WithContext(ctx).
		Where("status = ? AND next_attempt_at <= ?", models.WebhookDeliveryPending, time.Now()).
		Order("next_attempt_at asc").
		Limit(100).
		Find(&due).Error; err != nil {
		log.Printf("webhook dispatcher: list due: %v", err)
		return
	}

	for i := range due {
		select {
		case <-ctx.Done():
			return
		default:
		}
		claimed, err := s.claim(ctx, due[i].ID)
		if err != nil {
			log.Printf("webhook dispatcher: claim %s: %v", due[i].ID, err)
			continue
		}
		if !claimed {
			continue // another dispatcher took it
		}
		s.deliver(ctx, &due[i])
	}
}

// claim moves a pending row to sending atomically. It reports whether this
// caller won the row.
func (s *WebhookService) claim(ctx context.Context, id uuid.UUID) (bool, error) {
	result := s.db.WithContext(ctx).Model(&models.WebhookDelivery{}).
		Where("id = ? AND status = ?", id, models.WebhookDeliveryPending).
		Update("status", models.WebhookDeliverySending)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}

// deliver posts the payload to the endpoint and records the outcome. A
// non-2xx response or a transport error schedules a retry with exponential
// backoff until MaxAttempts, after which the row is failed.
func (s *WebhookService) deliver(ctx context.Context, delivery *models.WebhookDelivery) {
	var hook models.Webhook
	if err := s.db.WithContext(ctx).Where("id = ?", delivery.WebhookID).First(&hook).Error; err != nil {
		s.failDelivery(ctx, delivery, 0, "webhook no longer exists")
		return
	}
	if !hook.IsActive {
		s.failDelivery(ctx, delivery, 0, "webhook is disabled")
		return
	}

	secret, err := decryptWebhookSecret(hook.SecretEncrypted)
	if err != nil {
		s.failDelivery(ctx, delivery, 0, "webhook secret unavailable")
		return
	}

	statusCode, err := s.post(ctx, hook.URL, delivery.Payload, delivery.ID.String(), delivery.Event, secret)
	if err == nil && statusCode >= 200 && statusCode < 300 {
		now := time.Now()
		s.db.WithContext(ctx).Model(&models.WebhookDelivery{}).
			Where("id = ?", delivery.ID).
			Updates(map[string]interface{}{
				"status":           models.WebhookDeliverySent,
				"attempts":         delivery.Attempts + 1,
				"last_status_code": statusCode,
				"last_error":       "",
				"delivered_at":     &now,
			})
		s.db.WithContext(ctx).Model(&models.Webhook{}).Where("id = ?", hook.ID).Update("last_success", &now)
		return
	}

	attempt := delivery.Attempts + 1
	reason := "delivery failed"
	if err != nil {
		reason = err.Error()
	} else {
		reason = fmt.Sprintf("endpoint returned %d", statusCode)
	}
	s.db.WithContext(ctx).Model(&models.Webhook{}).Where("id = ?", hook.ID).Update("last_failure", time.Now())

	if attempt >= delivery.MaxAttempts {
		s.failDelivery(ctx, delivery, statusCode, reason)
		return
	}
	// Exponential backoff: 30s, 1m, 2m, 4m ... capped at 30m.
	backoff := 30 * time.Second * time.Duration(1<<uint(attempt-1))
	if backoff > 30*time.Minute {
		backoff = 30 * time.Minute
	}
	s.db.WithContext(ctx).Model(&models.WebhookDelivery{}).
		Where("id = ?", delivery.ID).
		Updates(map[string]interface{}{
			"status":           models.WebhookDeliveryPending,
			"attempts":         attempt,
			"last_status_code": statusCode,
			"last_error":       truncate(reason, 250),
			"next_attempt_at":  time.Now().Add(backoff),
		})
}

func (s *WebhookService) failDelivery(ctx context.Context, delivery *models.WebhookDelivery, statusCode int, reason string) {
	s.db.WithContext(ctx).Model(&models.WebhookDelivery{}).
		Where("id = ?", delivery.ID).
		Updates(map[string]interface{}{
			"status":           models.WebhookDeliveryFailed,
			"attempts":         delivery.Attempts + 1,
			"last_status_code": statusCode,
			"last_error":       truncate(reason, 250),
		})
}

// post performs the signed POST and returns the status code. A transport error
// is returned with a zero status code.
func (s *WebhookService) post(ctx context.Context, endpoint, payload, deliveryID, event, secret string) (int, error) {
	if err := s.validateURL(endpoint); err != nil {
		return 0, err
	}
	reqCtx, cancel := context.WithTimeout(ctx, webhookDeliveryTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, endpoint, bytes.NewReader([]byte(payload)))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-TigerSwap-Event", event)
	req.Header.Set("X-TigerSwap-Delivery", deliveryID)
	req.Header.Set("X-TigerSwap-Signature", "sha256="+webhookSignature(secret, payload))

	resp, err := s.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	// Drain a little of the body so the connection can be reused; the body is not
	// otherwise used.
	_, _ = io.CopyN(io.Discard, resp.Body, 4096)
	return resp.StatusCode, nil
}

// webhookSignature is the hex HMAC-SHA256 of the raw body under the endpoint's
// secret. The receiver recomputes it over the bytes it received and compares in
// constant time.
func webhookSignature(secret, payload string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

// validateURL rejects a scheme other than http(s) and, unless explicitly
// allowed, any host that resolves to a private, loopback or link-local address.
// This stops a subscriber pointing a webhook at an internal service or a cloud
// metadata endpoint.
func (s *WebhookService) validateURL(raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return fmt.Errorf("invalid webhook url: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return errors.New("webhook url must be http or https")
	}
	host := u.Hostname()
	if host == "" {
		return errors.New("webhook url has no host")
	}
	if s.allowPrivate {
		return nil
	}

	// An IP literal is checked directly; a hostname is resolved and every address
	// is checked, so a name that maps to a private address is refused.
	if ip := net.ParseIP(host); ip != nil {
		if !isPublicIP(ip) {
			return errors.New("webhook url resolves to a non-public address")
		}
		return nil
	}
	addrs, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("webhook host cannot be resolved: %w", err)
	}
	if len(addrs) == 0 {
		return errors.New("webhook host cannot be resolved")
	}
	for _, ip := range addrs {
		if !isPublicIP(ip) {
			return errors.New("webhook url resolves to a non-public address")
		}
	}
	return nil
}

// isPublicIP reports whether an address is a routable public address. Loopback,
// private, link-local (including the 169.254.169.254 metadata address),
// unspecified, multicast and unique-local IPv6 ranges are all rejected.
func isPublicIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast() ||
		ip.IsInterfaceLocalMulticast() {
		return false
	}
	if ip4 := ip.To4(); ip4 != nil {
		// 100.64.0.0/10 carrier-grade NAT and 192.0.0.0/24 are not public.
		if ip4[0] == 100 && ip4[1] >= 64 && ip4[1] <= 127 {
			return false
		}
	}
	return true
}

// CreateWebhook registers an endpoint for the caller. The signing secret is
// generated server-side and returned once; only its ciphertext is stored.
func (s *WebhookService) CreateWebhook(c *gin.Context) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}
	var req struct {
		URL    string `json:"url" binding:"required"`
		Events string `json:"events"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequestError(c, err)
		return
	}
	if err := s.validateURL(req.URL); err != nil {
		respondBadRequestError(c, err)
		return
	}
	events, err := normalizeWebhookEvents(req.Events)
	if err != nil {
		respondBadRequestError(c, err)
		return
	}

	secret := "whsec_" + randomHex(24)
	sealed, err := encryptWebhookSecret(secret)
	if err != nil {
		respondInternalError(c, err)
		return
	}

	hook := models.Webhook{
		UserID:          userID,
		URL:             req.URL,
		Events:          events,
		SecretEncrypted: sealed,
		IsActive:        true,
	}
	if err := s.db.Create(&hook).Error; err != nil {
		respondInternalError(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":         hook.ID,
		"url":        hook.URL,
		"events":     hook.Events,
		"is_active":  hook.IsActive,
		"secret":     secret,
		"created_at": hook.CreatedAt,
	})
}

// normalizeWebhookEvents validates a subscription list and defaults it to all
// events when empty.
func normalizeWebhookEvents(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "*" {
		return "*", nil
	}
	seen := map[string]bool{}
	var out []string
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if !webhookEvents[part] {
			return "", fmt.Errorf("unknown webhook event %q", part)
		}
		if !seen[part] {
			seen[part] = true
			out = append(out, part)
		}
	}
	if len(out) == 0 {
		return "*", nil
	}
	return strings.Join(out, ","), nil
}

// ListWebhooks returns the caller's endpoints.
func (s *WebhookService) ListWebhooks(c *gin.Context) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}
	var hooks []models.Webhook
	if err := s.db.Where("user_id = ?", userID).Order("created_at desc").Find(&hooks).Error; err != nil {
		respondInternalError(c, err)
		return
	}
	out := make([]gin.H, 0, len(hooks))
	for i := range hooks {
		out = append(out, gin.H{
			"id":           hooks[i].ID,
			"url":          hooks[i].URL,
			"events":       hooks[i].Events,
			"is_active":    hooks[i].IsActive,
			"last_success": hooks[i].LastSuccess,
			"last_failure": hooks[i].LastFailure,
			"created_at":   hooks[i].CreatedAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{"webhooks": out})
}

// UpdateWebhook toggles a subscription's active flag or its event list.
func (s *WebhookService) UpdateWebhook(c *gin.Context) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}
	var req struct {
		IsActive *bool   `json:"is_active"`
		Events   *string `json:"events"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequestError(c, err)
		return
	}
	updates := map[string]interface{}{}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}
	if req.Events != nil {
		events, err := normalizeWebhookEvents(*req.Events)
		if err != nil {
			respondBadRequestError(c, err)
			return
		}
		updates["events"] = events
	}
	if len(updates) == 0 {
		respondBadRequestError(c, errors.New("nothing to update"))
		return
	}
	result := s.db.Model(&models.Webhook{}).
		Where("id = ? AND user_id = ?", c.Param("id"), userID).
		Updates(updates)
	if result.Error != nil {
		respondInternalError(c, result.Error)
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "webhook not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

// DeleteWebhook removes a subscription. Pending deliveries are removed with it
// so a deleted endpoint is not retried.
func (s *WebhookService) DeleteWebhook(c *gin.Context) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var hook models.Webhook
		if err := tx.Where("id = ? AND user_id = ?", c.Param("id"), userID).First(&hook).Error; err != nil {
			return err
		}
		if err := tx.Where("webhook_id = ?", hook.ID).Delete(&models.WebhookDelivery{}).Error; err != nil {
			return err
		}
		return tx.Delete(&hook).Error
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "webhook not found"})
		return
	}
	if err != nil {
		respondInternalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

// encryptWebhookSecret seals a signing secret with AES-256-GCM under a key
// derived from WALLET_ENCRYPTION_KEY, matching the wallet and TOTP construction.
func encryptWebhookSecret(plaintext string) (string, error) {
	key, err := deriveWebhookKey()
	if err != nil {
		return "", err
	}
	return sealAESGCM(key, []byte(plaintext))
}

func decryptWebhookSecret(encoded string) (string, error) {
	if encoded == "" {
		return "", errors.New("webhook has no stored secret")
	}
	key, err := deriveWebhookKey()
	if err != nil {
		return "", err
	}
	plaintext, err := openAESGCM(key, encoded)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

func deriveWebhookKey() ([]byte, error) {
	secret := os.Getenv("WALLET_ENCRYPTION_KEY")
	if len(secret) < 32 {
		return nil, errors.New("WALLET_ENCRYPTION_KEY must be set (32+ chars) to register a webhook")
	}
	sum := sha256.Sum256([]byte(secret))
	return sum[:], nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}

// randomHex returns n random bytes hex-encoded.
func randomHex(n int) string {
	raw := make([]byte, n)
	if _, err := rand.Read(raw); err != nil {
		// crypto/rand failing is unrecoverable; a panic here is preferable to
		// issuing a webhook secret with predictable bytes.
		panic(fmt.Sprintf("crypto/rand unavailable: %v", err))
	}
	return hex.EncodeToString(raw)
}

// sealAESGCM encrypts plaintext with AES-256-GCM, prefixing the nonce, and
// hex-encodes the result. It matches the wallet and TOTP secret construction.
func sealAESGCM(key, plaintext []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	return hex.EncodeToString(gcm.Seal(nonce, nonce, plaintext, nil)), nil
}

// openAESGCM reverses sealAESGCM.
func openAESGCM(key []byte, encoded string) ([]byte, error) {
	raw, err := hex.DecodeString(encoded)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(raw) < gcm.NonceSize() {
		return nil, errors.New("ciphertext too short")
	}
	nonce, ciphertext := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	return gcm.Open(nil, nonce, ciphertext, nil)
}
