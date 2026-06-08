package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	_ "github.com/lib/pq"
	"golang.org/x/time/rate"
)

// ==================== CONFIGURATION ====================

type Config struct {
	Server          ServerConfig   `json:"server"`
	Database       DatabaseConfig `json:"database"`
	Firebase      FirebaseConfig `json:"firebase"`
	APNS           APNSConfig `json:"apns"`
	Notifications NotificationsConfig `json:"notifications"`
}

type ServerConfig struct {
	Port         string        `json:"port"`
	RateLimit    RateLimitConfig `json:"rateLimit"`
	Environment string        `json:"environment"`
}

type RateLimitConfig struct {
	RequestsPerSecond float64 `json:"requestsPerSecond"`
	Burst         int     `json:"burst"`
}

type DatabaseConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User    string `json:"user"`
	Password string `json:"password"`
	DBName  string `json:"dbname"`
}

type FirebaseConfig struct {
	ProjectID string `json:"projectId"`
	PrivateKey string `json:"privateKey"`
	ClientEmail string `json:"clientEmail"`
}

type APNSConfig struct {
	KeyPath      string `json:"keyPath"`
	CertPath    string `json:"certPath"`
	BundleID   string `json:"bundleId"`
	Production bool   `json:"production"`
}

type NotificationsConfig struct {
	MaxBatchSize   int `json:"maxBatchSize"`
	MaxRetries     int `json:"maxRetries"`
	RetryDelay     int `json:"retryDelaySeconds"`
	BatchDelayMs   int `json:"batchDelayMs"`
}

// ==================== MODELS ====================

type PushNotification struct {
	ID        string                 `json:"id"`
	UserID   string                 `json:"userId"`
	Type    NotificationType       `json:"type"`
	Title   string                 `json:"title"`
	Body    string                 `json:"body"`
	Data    map[string]string    `json:"data,omitempty"`
	Token   string                 `json:"deviceToken"`
	Priority Priority            `json:"priority"`
	Status  NotificationStatus   `json:"status"`
	RetryCount int               `json:"retryCount"`
	CreatedAt time.Time          `json:"createdAt"`
	SentAt   *time.Time        `json:"sentAt,omitempty"`
}

type NotificationType string

const (
	TypePriceAlert      NotificationType = "PRICE_ALERT"
	TypeSwapComplete  NotificationType = "SWAP_COMPLETE"
	TypeLiquidity     NotificationType = "LIQUIDITY"
	TypeMint        NotificationType = "MINT"
	TypeBurn        NotificationType = "BURN"
	TypeGovernance  NotificationType = "GOVERNANCE"
	TypeSecurity   NotificationType = "SECURITY"
	TypeMarketing  NotificationType = "MARKETING"
	TypeSystem     NotificationType = "SYSTEM"
)

type Priority string

const (
	PriorityHigh   Priority = "high"
	PriorityNormal Priority = "normal"
	PriorityLow    Priority = "low"
)

type NotificationStatus string

const (
	StatusPending  NotificationStatus = "pending"
	StatusSent    NotificationStatus = "sent"
	StatusFailed  NotificationStatus = "failed"
	StatusQueued  NotificationStatus = "queued"
)

type DeviceRegistration struct {
	UserID      string    `json:"userId"`
	Token      string    `json:"token"`
	Platform   string    `json:"platform"` // ios, android, web
	Language   string    `json:"language"`
	Timezone   string    `json:"timezone"`
	AppVersion string    `json:"appVersion"`
	Active    bool      `json:"active"`
	LastSeen  time.Time `json:"lastSeen"`
	CreatedAt time.Time `json:"createdAt"`
}

type Subscription struct {
	UserID      string              `json:"userId"`
	Channels   []string           `json:"channels"`
	Settings  map[string]bool     `json:"settings"`
	CreatedAt time.Time         `json:"createdAt"`
}

// ==================== DATABASE ====================

type Database interface {
	Init() error
	Close() error
	
	// Device registration
	RegisterDevice(ctx context.Context, device *DeviceRegistration) error
	UnregisterDevice(ctx context.Context, token string) error
	GetDevice(ctx context.Context, token string) (*DeviceRegistration, error)
	GetDevicesByUser(ctx context.Context, userID string) ([]*DeviceRegistration, error)
	
	// Notifications
	SaveNotification(ctx context.Context, notif *PushNotification) error
	GetNotification(ctx context.Context, id string) (*PushNotification, error)
	GetNotificationsByUser(ctx context.Context, userID string, limit int) ([]*PushNotification, error)
	UpdateNotificationStatus(ctx context.Context, id string, status NotificationStatus) error
	
	// Subscriptions
	Subscribe(ctx context.Context, sub *Subscription) error
	GetSubscription(ctx context.Context, userID string) (*Subscription, error)
}

type PostgresDatabase struct {
	conn *sql.DB
}

func (db *PostgresDatabase) Init() error {
	// In production, use actual connection
	// This is a placeholder
	return nil
}

func (db *PostgresDatabase) Close() error {
	if db.conn != nil {
		return db.conn.Close()
	}
	return nil
}

func (db *PostgresDatabase) RegisterDevice(ctx context.Context, device *DeviceRegistration) error {
	// In production, implement actual database operation
	return nil
}

func (db *PostgresDatabase) UnregisterDevice(ctx context.Context, token string) error {
	return nil
}

func (db *PostgresDatabase) GetDevice(ctx context.Context, token string) (*DeviceRegistration, error) {
	return nil, nil
}

func (db *PostgresDatabase) GetDevicesByUser(ctx context.Context, userID string) ([]*DeviceRegistration, error) {
	return nil, nil
}

func (db *PostgresDatabase) SaveNotification(ctx context.Context, notif *PushNotification) error {
	return nil
}

func (db *PostgresDatabase) GetNotification(ctx context.Context, id string) (*PushNotification, error) {
	return nil, nil
}

func (db *PostgresDatabase) GetNotificationsByUser(ctx context.Context, userID string, limit int) ([]*PushNotification, error) {
	return nil, nil
}

func (db *PostgresDatabase) UpdateNotificationStatus(ctx context.Context, id string, status NotificationStatus) error {
	return nil
}

func (db *PostgresDatabase) Subscribe(ctx context.Context, sub *Subscription) error {
	return nil
}

func (db *PostgresDatabase) GetSubscription(ctx context.Context, userID string) (*Subscription, error) {
	return nil, nil
}

// ==================== PUSH SERVICES ====================

type PushService interface {
	Send(ctx context.Context, notif *PushNotification) error
	SendBatch(ctx context.Context, notifications []*PushNotification) error
}

type FirebaseService struct {
	projectID  string
	key       string
	client    *http.Client
}

func NewFirebaseService(projectID, privateKey, clientEmail string) *FirebaseService {
	return &FirebaseService{
		projectID: projectID,
		key:      privateKey,
		client:   &http.Client{Timeout: 30 * time.Second},
	}
}

func (f *FirebaseService) Send(ctx context.Context, notif *PushNotification) error {
	// Firebase FCM API endpoint
	url := fmt.Sprintf("https://fcm.googleapis.com/v1/projects/%s/messages:send", f.projectID)
	
	// Build message payload
	payload := map[string]interface{}{
		"message": map[string]interface{}{
			"token": notif.Token,
			"notification": map[string]string{
				"title": notif.Title,
				"body":  notif.Body,
			},
			"android": map[string]interface{}{
				"priority": string(notif.Priority),
				"data":    notif.Data,
			},
			"apns": map[string]interface{}{
				"payload": map[string]interface{}{
					"aps": map[string]string{
						"alert": notif.Body,
					},
				},
			},
		},
	}
	
	// Add custom data
	if len(notif.Data) > 0 {
		if msg, ok := payload["message"].(map[string]interface{})["data"].(map[string]string); ok {
			for k, v := range notif.Data {
				msg[k] = v
			}
		}
	}
	
	// In production, send actual request
	_ = url
	_ = payload
	
	return nil
}

func (f *FirebaseService) SendBatch(ctx context.Context, notifications []*PushNotification) error {
	for _, notif := range notifications {
		if err := f.Send(ctx, notif); err != nil {
			log.Printf("Failed to send notification %s: %v", notif.ID, err)
		}
	}
	return nil
}

type APNSService struct {
	keyPath    string
	certPath   string
	bundleID  string
	production bool
}

func NewAPNSService(keyPath, certPath, bundleID string, production bool) *APNSService {
	return &APNSService{
		keyPath:     keyPath,
		certPath:    certPath,
		bundleID:   bundleID,
		production: production,
	}
}

func (a *APNSService) Send(ctx context.Context, notif *PushNotification) error {
	// APNS implementation
	_ = notif
	return nil
}

func (a *APNSService) SendBatch(ctx context.Context, notifications []*PushNotification) error {
	for _, notif := range notifications {
		if err := a.Send(ctx, notif); err != nil {
			log.Printf("Failed to send APNS notification %s: %v", notif.ID, err)
		}
	}
	return nil
}

// ==================== NOTIFICATION QUEUE ====================

type NotificationQueue struct {
	db         Database
	firebase  PushService
	apns      PushService
	maxBatch  int
	maxRetries int
	retryDelay time.Duration
}

func NewNotificationQueue(db Database, firebase PushService, apns PushService, maxBatch, maxRetries, retryDelay int) *NotificationQueue {
	return &NotificationQueue{
		db:         db,
		firebase:  firebase,
		apns:      apns,
		maxBatch:  maxBatch,
		maxRetries: maxRetries,
		retryDelay: time.Duration(retryDelay) * time.Second,
	}
}

func (q *NotificationQueue) Enqueue(ctx context.Context, notif *PushNotification) error {
	notif.Status = StatusQueued
	notif.CreatedAt = time.Now()
	return q.db.SaveNotification(ctx, notif)
}

func (q *NotificationQueue) ProcessQueue(ctx context.Context) error {
	// Get pending notifications
	notifications, err := q.getPendingNotifications(ctx)
	if err != nil {
		return err
	}
	
	if len(notifications) == 0 {
		return nil
	}
	
	// Process in batches
	for i := 0; i < len(notifications); i += q.maxBatch {
		end := i + q.maxBatch
		if end > len(notifications) {
			end = len(notifications)
		}
		
		batch := notifications[i:end]
		if err := q.processBatch(ctx, batch); err != nil {
			log.Printf("Failed to process batch: %v", err)
		}
		
		// Small delay between batches
		time.Sleep(10 * time.Millisecond)
	}
	
	return nil
}

func (q *NotificationQueue) getPendingNotifications(ctx context.Context) ([]*PushNotification, error) {
	// In production, query database
	return nil, nil
}

func (q *NotificationQueue) processBatch(ctx context.Context, notifications []*PushNotification) error {
	for _, notif := range notifications {
		var err error
		
		// Try Firebase first
		if strings.HasPrefix(notif.Token, "fcm:") {
			err = q.firebase.Send(ctx, notif)
		} else {
			// Try APNS
			err = q.apns.Send(ctx, notif)
		}
		
		if err != nil {
			notif.RetryCount++
			if notif.RetryCount < q.maxRetries {
				notif.Status = StatusQueued
				time.Sleep(q.retryDelay)
			} else {
				notif.Status = StatusFailed
			}
		} else {
			now := time.Now()
			notif.SentAt = &now
			notif.Status = StatusSent
		}
		
		q.db.SaveNotification(ctx, notif)
	}
	
	return nil
}

// ==================== HTTP HANDLERS ====================

type Handler struct {
	db      Database
	queue   *NotificationQueue
	limiter *rate.Limiter
}

func NewHandler(db Database, queue *NotificationQueue) *Handler {
	return &Handler{
		db:      db,
		queue:   queue,
		limiter: rate.New(rate.Limit(100), 200),
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Apply rate limiting
	if err := h.limiter.Wait(r.Context()); err != nil {
		http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
		return
	}
	
	// Add CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}
	
	// Route
	switch r.URL.Path {
	case "/health":
		h.handleHealth(w, r)
	case "/register":
		h.handleRegister(w, r)
	case "/unregister":
		h.handleUnregister(w, r)
	case "/send":
		h.handleSend(w, r)
	case "/subscribe":
		h.handleSubscribe(w, r)
	case "/subscriptions":
		h.handleGetSubscriptions(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{"status": "ok", "timestamp": time.Now().ISO8601()})
}

func (h *Handler) handleRegister(w http.ResponseWriter, r *http.Request) {
	var device DeviceRegistration
	if err := json.NewDecoder(r.Body).Decode(&device); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	
	if err := h.db.RegisterDevice(r.Context(), &device); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	json.NewEncoder(w).Encode(map[string]string{"status": "registered"})
}

func (h *Handler) handleUnregister(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	
	if err := h.db.UnregisterDevice(r.Context(), req.Token); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	json.NewEncoder(w).Encode(map[string]string{"status": "unregistered"})
}

func (h *Handler) handleSend(w http.ResponseWriter, r *http.Request) {
	var notif PushNotification
	if err := json.NewDecoder(r.Body).Decode(&notif); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	
	notif.ID = fmt.Sprintf("notif-%d", time.Now().UnixNano())
	
	if err := h.queue.Enqueue(r.Context(), &notif); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	json.NewEncoder(w).Encode(map[string]string{"id": notif.ID, "status": "queued"})
}

func (h *Handler) handleSubscribe(w http.ResponseWriter, r *http.Request) {
	var sub Subscription
	if err := json.NewDecoder(r.Body).Decode(&sub); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	
	if err := h.db.Subscribe(r.Context(), &sub); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	json.NewEncoder(w).Encode(map[string]string{"status": "subscribed"})
}

func (h *Handler) handleGetSubscriptions(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("userId")
	if userID == "" {
		http.Error(w, "userId required", http.StatusBadRequest)
		return
	}
	
	sub, err := h.db.GetSubscription(r.Context(), userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	if sub == nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"userId": userID, "channels": []string{}})
		return
	}
	
	json.NewEncoder(w).Encode(sub)
}

// ==================== MAIN ====================

func main() {
	// Load configuration
	config := Config{
		Server: ServerConfig{
			Port:      "8080",
			RateLimit: RateLimitConfig{RequestsPerSecond: 100, Burst: 200},
			Environment: "development",
		},
		Database: DatabaseConfig{
			Host: "localhost", Port: 5432, User: "tigerswap", Password: "", DBName: "tigerswap",
		},
		Firebase: FirebaseConfig{
			ProjectID: "tigerswap-app", PrivateKey: "", ClientEmail: "",
		},
		APNS: APNSConfig{
			KeyPath: "", CertPath: "", BundleID: "com.tigerswap.app", Production: false,
		},
		Notifications: NotificationsConfig{
			MaxBatchSize: 500, MaxRetries: 3, RetryDelay: 1, BatchDelayMs: 10,
		},
	}
	
	// Initialize database
	db := &PostgresDatabase{}
	if err := db.Init(); err != nil {
		log.Printf("Warning: Database not connected: %v", err)
	}
	defer db.Close()
	
	// Initialize push services
	firebase := NewFirebaseService(config.Firebase.ProjectID, config.Firebase.PrivateKey, config.Firebase.ClientEmail)
	apns := NewAPNSService(config.APNS.KeyPath, config.APNS.CertPath, config.APNS.BundleID, config.APNS.Production)
	
	// Initialize notification queue
	queue := NewNotificationQueue(
		db, firebase, apns,
		config.Notifications.MaxBatchSize,
		config.Notifications.MaxRetries,
		config.Notifications.RetryDelay,
	)
	
	// Start queue processor
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		
		for range ticker.C {
			if err := queue.ProcessQueue(context.Background()); err != nil {
				log.Printf("Queue processing error: %v", err)
			}
		}
	}()
	
	// Initialize handler
	handler := NewHandler(db, queue)
	
	// Start server
	mux := http.NewServeMux()
	mux.HandleFunc("/", handler.ServeHTTP)
	
	server := &http.Server{
		Addr:         ":" + config.Server.Port,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}
	
	// Graceful shutdown
	go func() {
		log.Printf("Push notification service starting on port %s", config.Server.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()
	
	// Wait for shutdown signal
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	
	log.Println("Shutting down...")
	server.Shutdown(context.Background())
}