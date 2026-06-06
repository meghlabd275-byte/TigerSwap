package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
)

var db *sql.DB

func initDB() error {
	var err error
	connStr := "host=localhost port=5432 user=tigerswap password=securepass dbname=tigerswap sslmode=disable"
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		return err
	}
	return db.Ping()
}

// API key types
type APIKey struct {
	ID          int
	UserID     int
	KeyName    string
	APIKey    string
	APISecret string
	Permissions []string
	RateLimit int
	IPWhitelist []string
	IsActive  bool
	LastUsed sql.NullTime
	ExpiresAt sql.NullTime
	CreatedAt time.Time
}

// White label API key
type WhiteLabelAPIKey struct {
	ID          int
	WhiteLabelID int
	APIKey    string
	APISecret string
	Permissions []string
	IsActive  bool
	CreatedAt time.Time
}

// Generate API key
func generateAPIKey() (string, string, error) {
	keyBytes := make([]byte, 32)
	secretBytes := make([]byte, 64)
	
	rand.Read(keyBytes)
	rand.Read(secretBytes)
	
	key := "ts_" + hex.EncodeToString(keyBytes)
	secret := hex.EncodeToString(secretBytes)
	
	return key, secret, nil
}

// Encrypt API secret
func encryptAPISecret(secret string) (string, error) {
	key := []byte("tigerswap_api_key_2026")
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	
	nonce := make([]byte, gcm.NonceSize())
	rand.Read(nonce)
	
	return hex.EncodeToString(gcm.Seal(nonce, nonce, []byte(secret), nil)), nil
}

// Decrypt API secret
func decryptAPISecret(encrypted string) (string, error) {
	key := []byte("tigerswap_api_key_2026")
	data, err := hex.DecodeString(encrypted)
	if err != nil {
		return "", err
	}
	
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}
	
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	
	return string(plaintext), nil
}

// Handlers

// Create API key
func createAPIKeyHandler(c *gin.Context) {
	userID := getUserIDFromContext(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	
	var input struct {
		KeyName     string   `json:"key_name"`
		Permissions []string `json:"permissions" binding:"required"`
		RateLimit  int      `json:"rate_limit"`
		IPWhitelist []string `json:"ip_whitelist"`
		ExpiresIn  int      `json:"expires_in"` // hours, 0 = never
	}
	
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	// Generate key and secret
	apiKey, apiSecret, err := generateAPIKey()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Key generation failed"})
		return
	}
	
	// Encrypt secret
	encryptedSecret, err := encryptAPISecret(apiSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Encryption failed"})
		return
	}
	
	rateLimit := input.RateLimit
	if rateLimit == 0 {
		rateLimit = 1000
	}
	
	var expiresAt interface{}
	if input.ExpiresIn > 0 {
		expiresAt = time.Now().Add(time.Duration(input.ExpiresIn) * time.Hour)
	}
	
	// Save API key
	var keyID int
	err = db.QueryRow(`
		INSERT INTO api_keys (user_id, key_name, api_key, api_secret, permissions, rate_limit, ip_whitelist, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id`,
		userID, input.KeyName, apiKey, encryptedSecret, input.Permissions, rateLimit, input.IPWhitelist, expiresAt,
	).Scan(&keyID)
	
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "API key creation failed"})
		return
	}
	
	logAudit(userID, "api_key_create", "api_keys", keyID, gin.H{"key_name": input.KeyName})
	
	c.JSON(http.StatusCreated, gin.H{
		"message":  "API key created",
		"key_id":  keyID,
		"api_key": apiKey,
		"secret":  apiSecret, // Only shown once!
	})
}

// List API keys
func listAPIKeysHandler(c *gin.Context) {
	userID := getUserIDFromContext(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	
	rows, err := db.Query(`
		SELECT id, key_name, api_key, permissions, rate_limit, ip_whitelist, is_active, last_used, expires_at, created_at
		FROM api_keys WHERE user_id = $1 ORDER BY created_at DESC`,
		userID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	defer rows.Close()
	
	var keys []map[string]interface{}
	for rows.Next() {
		var k struct {
			ID          int
			KeyName    string
			APIKey    string
			Permissions []string
			RateLimit int
			IPWhitelist []string
			IsActive  bool
			LastUsed sql.NullTime
			ExpiresAt sql.NullTime
			CreatedAt time.Time
		}
		rows.Scan(&k.ID, &k.KeyName, &k.APIKey, &k.Permissions, &k.RateLimit, &k.IPWhitelist, &k.IsActive, &k.LastUsed, &k.ExpiresAt, &k.CreatedAt)
		
		keys = append(keys, map[string]interface{}{
			"id":          k.ID,
			"key_name":    k.KeyName,
			"api_key":    k.APIKey,
			"permissions": k.Permissions,
			"rate_limit":  k.RateLimit,
			"is_active":  k.IsActive,
			"last_used":  k.LastUsed.Time,
			"expires_at": k.ExpiresAt.Time,
			"created_at": k.CreatedAt,
		})
	}
	
	c.JSON(http.StatusOK, gin.H{"api_keys": keys})
}

// Revoke API key
func revokeAPIKeyHandler(c *gin.Context) {
	userID := getUserIDFromContext(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	
	keyID := c.Param("id")
	
	// Verify ownership
	var ownerID int
	err := db.QueryRow("SELECT user_id FROM api_keys WHERE id = $1", keyID).Scan(&ownerID)
	if err != nil || ownerID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Not authorized"})
		return
	}
	
	// Revoke
	_, err = db.Exec("UPDATE api_keys SET is_active = false WHERE id = $1", keyID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Revocation failed"})
		return
	}
	
	logAudit(userID, "api_key_revoke", "api_keys", keyID, nil)
	
	c.JSON(http.StatusOK, gin.H{"message": "API key revoked"})
}

// Validate API key (public endpoint)
func validateAPIKeyHandler(c *gin.Context) {
	apiKey := c.GetHeader("X-API-Key")
	apiSecret := c.GetHeader("X-API-Secret")
	
	if apiKey == "" || apiSecret == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing credentials"})
		return
	}
	
	// Get API key
	var key struct {
		ID          int
		UserID     int
		APISecret string
		Permissions []string
		RateLimit int
		IPWhitelist []string
		IsActive  bool
		ExpiresAt sql.NullTime
	}
	err := db.QueryRow(`
		SELECT id, user_id, api_secret, permissions, rate_limit, ip_whitelist, is_active, expires_at
		FROM api_keys WHERE api_key = $1`,
		apiKey,
	).Scan(&key.ID, &key.UserID, &key.APISecret, &key.Permissions, &key.RateLimit, &key.IPWhitelist, &key.IsActive, &key.ExpiresAt)
	
	if err == sql.ErrNoRows {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid API key"})
		return
	}
	
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	
	// Check if active
	if !key.IsActive {
		c.JSON(http.StatusForbidden, gin.H{"error": "API key is inactive"})
		return
	}
	
	// Check expiry
	if key.ExpiresAt.Valid && key.ExpiresAt.Time.Before(time.Now()) {
		c.JSON(http.StatusForbidden, gin.H{"error": "API key expired"})
		return
	}
	
	// Verify secret
	decryptedSecret, err := decryptAPISecret(key.APISecret)
	if err != nil || decryptedSecret != apiSecret {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid secret"})
		return
	}
	
	// Check IP whitelist
	if len(key.IPWhitelist) > 0 {
		clientIP := c.ClientIP()
		allowed := false
		for _, ip := range key.IPWhitelist {
			if ip == clientIP {
				allowed = true
				break
			}
		}
		if !allowed {
			c.JSON(http.StatusForbidden, gin.H{"error": "IP not allowed"})
			return
		}
	}
	
	// Update last used
	db.Exec("UPDATE api_keys SET last_used = NOW() WHERE id = $1", key.ID)
	
	// Log audit
	logAudit(key.UserID, "api_key_used", "api_keys", key.ID, nil)
	
	c.JSON(http.StatusOK, gin.H{
		"valid":      true,
		"user_id":    key.UserID,
		"permissions": key.Permissions,
		"rate_limit": key.RateLimit,
	})
}

// External wallet connection handler
func connectExternalWalletHandler(c *gin.Context) {
	// Public endpoint for external wallets
	var input struct {
		APIKey string `json:"api_key" binding:"required"`
		Signature string `json:"signature" binding:"required"`
		Timestamp int64 `json:"timestamp"`
	}
	
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	// Validate timestamp (must be within 5 minutes)
	if input.Timestamp > 0 {
		now := time.Now().Unix()
		if now-input.Timestamp > 300 || now-input.Timestamp < -300 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Timestamp expired"})
			return
		}
	}
	
	// Verify API key
	// Would verify signature in production
	c.JSON(http.StatusOK, gin.H{
		"message": "External wallet connected",
		"features": []string{"swap", "trade", "transfer"},
	})
}

// CEX connection handler
func connectCEXHandler(c *gin.Context) {
	userID := getUserIDFromContext(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	
	var input struct {
		CEXName string `json:"cex_name" binding:"required"`
		APIKey string `json:"api_key" binding:"required"`
		APISecret string `json:"api_secret" binding:"required"`
		Passphrase string `json:"passphrase"`
		Subaccount string `json:"subaccount"`
		Testnet bool `json:"testnet"`
	}
	
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	// Encrypt credentials
	encryptedKey, _ := encryptAPISecret(input.APIKey)
	encryptedSecret, _ := encryptAPISecret(input.APISecret)
	var encryptedPassphrase *string
	if input.Passphrase != "" {
		ep, _ := encryptAPISecret(input.Passphrase)
		encryptedPassphrase = &ep
	}
	
	// Save CEX connector
	var connectorID int
	err := db.QueryRow(`
		INSERT INTO cex_connectors (name, api_key_encrypted, api_secret_encrypted, passphrase_encrypted, subaccount_id, is_testnet, permissions, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id`,
		input.CEXName, encryptedKey, encryptedSecret, encryptedPassphrase, input.Subaccount, input.Testnet, []string{"trade", "withdraw"}, userID,
	).Scan(&connectorID)
	
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "CEX connection failed"})
		return
	}
	
	logAudit(userID, "cex_connect", "cex_connectors", connectorID, gin.H{"cex": input.CEXName})
	
	c.JSON(http.StatusCreated, gin.H{
		"message":    "CEX connected",
		"connector_id": connectorID,
	})
}

// DEX connection handler
func connectDEXHandler(c *gin.Context) {
	userID := getUserIDFromContext(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	
	var input struct {
		DEXName string `json:"dex_name" binding:"required"`
		Protocol string `json:"protocol" binding:"required"`
		RouterAddress string `json:"router_address"`
		FactoryAddress string `json:"factory_address"`
	}
	
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	// Save DEX connector
	var connectorID int
	err := db.QueryRow(`
		INSERT INTO dex_connectors (name, protocol, router_address, factory_address, supported_chains, created_by)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id`,
		input.DEXName, input.Protocol, input.RouterAddress, input.FactoryAddress, []string{"Ethereum", "BSC", "Polygon"}, userID,
	).Scan(&connectorID)
	
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "DEX connection failed"})
		return
	}
	
	logAudit(userID, "dex_connect", "dex_connectors", connectorID, gin.H{"dex": input.DEXName})
	
	c.JSON(http.StatusCreated, gin.H{
		"message":    "DEX connected",
		"connector_id": connectorID,
	})
}

// Helper functions
func getUserIDFromContext(c *gin.Context) int {
	sessionToken, _ := c.Cookie("session_token")
	if sessionToken == "" {
		return 0
	}
	
	var userID int
	var expiresAt time.Time
	err := db.QueryRow(`
		SELECT user_id, expires_at FROM sessions 
		WHERE session_token = $1 AND is_active = true AND expires_at > NOW()`,
		sessionToken,
	).Scan(&userID, &expiresAt)
	
	if err != nil {
		return 0
	}
	
	return userID
}

func logAudit(userID int, action, entityType string, entityID int, details map[string]interface{}) {
	detailsJSON, _ := json.Marshal(details)
	db.Exec(`
		INSERT INTO audit_logs (user_id, action, entity_type, entity_id, details)
		VALUES ($1, $2, $3, $4, $5)`,
		userID, action, entityType, entityID, string(detailsJSON),
	)
}

func authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := getUserIDFromContext(c)
		if userID == 0 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			c.Abort()
			return
		}
		c.Set("user_id", userID)
		c.Next()
	}
}

func main() {
	r := gin.Default()
	
	if err := initDB(); err != nil {
		fmt.Println("Database connection failed:", err)
	}
	
	// Public endpoints
	r.POST("/api/v1/auth/validate", validateAPIKeyHandler)
	r.POST("/api/v1/external/connect", connectExternalWalletHandler)
	
	// Protected endpoints
	api := r.Group("/api/v1")
	api.Use(authMiddleware())
	{
		api.POST("/api-key/create", createAPIKeyHandler)
		api.GET("/api-key/list", listAPIKeysHandler)
		api.DELETE("/api-key/:id", revokeAPIKeyHandler)
		api.POST("/cex/connect", connectCEXHandler)
		api.POST("/dex/connect", connectDEXHandler)
	}
	
	fmt.Println("API key service running on :8083")
	r.Run(":8083")
}