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

// White label types
type WhiteLabel struct {
	ID        int
	UserID    int
	BrandName string
	Domain   string
	Verified bool
	Status   string
	RevenueShare float64
	ApprovedBy sql.NullInt64
	ApprovedAt sql.NullTime
	CreatedAt time.Time
}

// Request white label
func requestWhiteLabelHandler(c *gin.Context) {
	userID := getUserIDFromContext(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	
	var input struct {
		BrandName string `json:"brand_name" binding:"required"`
		Domain  string `json:"domain"`
	}
	
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	// Check if user already has white label
	var existingID int
	err := db.QueryRow("SELECT id FROM white_labels WHERE user_id = $1", userID).Scan(&existingID)
	if err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "White label already exists"})
		return
	}
	
	// Create white label request
	var wlID int
	err = db.QueryRow(`
		INSERT INTO white_labels (user_id, brand_name, domain, status, revenue_share_percentage)
		VALUES ($1, $2, $3, 'pending', 20)
		RETURNING id`,
		userID, input.BrandName, input.Domain,
	).Scan(&wlID)
	
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "White label request failed"})
		return
	}
	
	logAudit(userID, "whitelabel_request", "white_labels", wlID, gin.H{"brand": input.BrandName})
	
	c.JSON(http.StatusCreated, gin.H{
		"message":      "White label requested",
		"white_label_id": wlID,
		"status":      "pending",
	})
}

// Approve white label (super admin)
func approveWhiteLabelHandler(c *gin.Context) {
	callerID := getUserIDFromContext(c)
	if callerID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	
	var callerRole string
	err := db.QueryRow("SELECT role FROM users WHERE id = $1", callerID).Scan(&callerRole)
	if err != nil || callerRole != "super_admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Super admin only"})
		return
	}
	
	wlID := c.Param("id")
	
	var userID int
	err = db.QueryRow("SELECT user_id FROM white_labels WHERE id = $1", wlID).Scan(&userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "White label not found"})
		return
	}
	
	// Generate API key for white label
	wlKey, wlSecret, _ := generateWLKey()
	
	// Approve
	_, err = db.Exec(`
		UPDATE white_labels 
		SET status = 'approved', approved_by = $1, approved_at = NOW(), domain_verified = true
		WHERE id = $2`,
		callerID, wlID,
	)
	
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Approval failed"})
		return
	}
	
	// Create white label API key
	db.Exec(`
		INSERT INTO white_label_api_keys (white_label_id, api_key, api_secret, permissions)
		VALUES ($1, $2, $3, $4)`,
		wlID, wlKey, wlSecret, []string{"*"},
	)
	
	// Update user to white label admin
	db.Exec("UPDATE users SET role = 'white_label_admin', is_white_label_admin = true, white_label_id = $1 WHERE id = $2", wlID, userID)
	
	logAudit(callerID, "whitelabel_approve", "white_labels", wlID, nil)
	
	// Update fee collection (20% to TigerSwap)
	db.Exec(`
		INSERT INTO trading_fees (user_id, fee_type, percentage, is_active, created_by)
		VALUES ($1, 'whitelabel_revenue_share', 20, true, $2)`,
		userID, callerID,
	)
	
	c.JSON(http.StatusOK, gin.H{
		"message": "White label approved",
		"api_key": wlKey,
		"secret": wlSecret,
	})
}

// Reject white label
func rejectWhiteLabelHandler(c *gin.Context) {
	callerID := getUserIDFromContext(c)
	if callerID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	
	var callerRole string
	err := db.QueryRow("SELECT role FROM users WHERE id = $1", callerID).Scan(&callerRole)
	if err != nil || callerRole != "super_admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Super admin only"})
		return
	}
	
	wlID := c.Param("id")
	
	_, err = db.Exec("UPDATE white_labels SET status = 'rejected' WHERE id = $1", wlID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Rejection failed"})
		return
	}
	
	logAudit(callerID, "whitelabel_reject", "white_labels", wlID, nil)
	
	c.JSON(http.StatusOK, gin.H{"message": "White label rejected"})
}

// Destroy white label
func destroyWhiteLabelHandler(c *gin.Context) {
	callerID := getUserIDFromContext(c)
	if callerID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	
	var callerRole string
	err := db.QueryRow("SELECT role FROM users WHERE id = $1", callerID).Scan(&callerRole)
	if err != nil || callerRole != "super_admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Super admin only"})
		return
	}
	
	wlID := c.Param("id")
	
	// Get all related users
	var userIDs []int
	rows, _ := db.Query("SELECT id FROM users WHERE white_label_id = $1", wlID)
	for rows.Next() {
		var uid int
		rows.Scan(&uid)
		userIDs = append(userIDs, uid)
	}
	
	// Delete all white label data
	db.Exec("DELETE FROM white_label_api_keys WHERE white_label_id = $1", wlID)
	db.Exec("DELETE FROM bot_subscriptions WHERE white_label_id = $1", wlID)
	db.Exec("UPDATE white_labels SET status = 'destroyed' WHERE id = $1", wlID)
	db.Exec("UPDATE users SET role = 'user', is_white_label_admin = false, white_label_id = NULL WHERE white_label_id = $1", wlID)
	
	logAudit(callerID, "whitelabel_destroy", "white_labels", wlID, gin.H{"affected_users": len(userIDs)})
	
	c.JSON(http.StatusOK, gin.H{
		"message":        "White label destroyed",
		"affected_users": len(userIDs),
	})
}

// List white labels
func listWhiteLabelsHandler(c *gin.Context) {
	callerID := getUserIDFromContext(c)
	if callerID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	
	var callerRole string
	err := db.QueryRow("SELECT role FROM users WHERE id = $1", callerID).Scan(&callerRole)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	
	var rows *sql.Rows
	
	if callerRole == "super_admin" {
		// Super admin sees all
		rows, err = db.Query(`
			SELECT wl.id, wl.user_id, wl.brand_name, wl.domain, wl.status, wl.revenue_share_percentage, wl.created_at, u.username
			FROM white_labels wl
			JOIN users u ON wl.user_id = u.id
			ORDER BY wl.created_at DESC`)
	} else {
		// Regular user sees own
		rows, err = db.Query(`
			SELECT id, user_id, brand_name, domain, status, revenue_share_percentage, created_at
			FROM white_labels WHERE user_id = $1`,
			callerID,
		)
	}
	
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	defer rows.Close()
	
	var whiteLabels []map[string]interface{}
	for rows.Next() {
		var wl struct {
			ID              int
			UserID          int
			BrandName      string
			Domain         string
			Status         string
			RevenueShare  float64
			CreatedAt     time.Time
			Username      string
		}
		
		if callerRole == "super_admin" {
			rows.Scan(&wl.ID, &wl.UserID, &wl.BrandName, &wl.Domain, &wl.Status, &wl.RevenueShare, &wl.CreatedAt, &wl.Username)
		} else {
			rows.Scan(&wl.ID, &wl.UserID, &wl.BrandName, &wl.Domain, &wl.Status, &wl.RevenueShare, &wl.CreatedAt)
			wl.Username = ""
		}
		
		whiteLabels = append(whiteLabels, map[string]interface{}{
			"id":                  wl.ID,
			"user_id":            wl.UserID,
			"brand_name":         wl.BrandName,
			"domain":             wl.Domain,
			"status":             wl.Status,
			"revenue_share":       wl.RevenueShare,
			"created_at":         wl.CreatedAt,
			"owner_username":      wl.Username,
		})
	}
	
	c.JSON(http.StatusOK, gin.H{"white_labels": whiteLabels})
}

// White label admin create sub-admin
func createWhiteLabelAdminHandler(c *gin.Context) {
	userID := getUserIDFromContext(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	
	// Check if white label admin
	var wlID sql.NullInt64
	var isWLAdmin bool
	err := db.QueryRow("SELECT white_label_id, is_white_label_admin FROM users WHERE id = $1", userID).Scan(&wlID, &isWLAdmin)
	if err != nil || !isWLAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "White label admin only"})
		return
	}
	
	var input struct {
		Username string `json:"username" binding:"required"`
		Email   string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required"`
	}
	
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	// Hash password
	passwordHash, _ := hashPassword(input.Password)
	
	// Create admin under white label
	var newUserID int
	err = db.QueryRow(`
		INSERT INTO users (username, email, password_hash, role, status, white_label_id, referrer_id)
		VALUES ($1, $2, $3, 'admin', 'active', $4, $5)
		RETURNING id`,
		input.Username, input.Email, passwordHash, wlID.Int64, userID,
	).Scan(&newUserID)
	
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Admin creation failed"})
		return
	}
	
	logAudit(userID, "whitelabel_admin_create", "users", newUserID, nil)
	
	c.JSON(http.StatusCreated, gin.H{
		"message": "White label admin created",
		"user_id": newUserID,
	})
}

// Get white label revenue
func getWhiteLabelRevenueHandler(c *gin.Context) {
	callerID := getUserIDFromContext(c)
	if callerID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	
	// Get white label ID
	var wlID int
	err := db.QueryRow("SELECT white_label_id FROM users WHERE id = $1", callerID).Scan(&wlID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No white label"})
		return
	}
	
	// Get revenue from last 30 days
	var revenue struct {
		TotalRevenue float64
		TotalFees  float64
		BotRevenue float64
	}
	
	db.QueryRow(`
		SELECT 
			COALESCE(SUM(amount), 0) as total_revenue,
			COALESCE(SUM(CASE WHEN fee_type = 'trading' THEN amount ELSE 0 END), 0) as total_fees,
			COALESCE(SUM(CASE WHEN fee_type = 'bot' THEN amount ELSE 0 END), 0) as bot_revenue
		FROM fee_collections
		WHERE user_id IN (SELECT id FROM users WHERE white_label_id = $1)
		AND created_at > NOW() - INTERVAL '30 days'`,
		wlID,
	).Scan(&revenue.TotalRevenue, &revenue.TotalFees, &revenue.BotRevenue)
	
	// Calculate TigerSwap share (20%)
	tigerShare := revenue.TotalRevenue * 0.20
	wlRevenue := revenue.TotalRevenue - tigerShare
	
	c.JSON(http.StatusOK, gin.H{
		"total_revenue": revenue.TotalRevenue,
		"tigerswap_share": tigerShare,
		"white_label_share": wlRevenue,
		"trading_fees": revenue.TotalFees,
		"bot_revenue": revenue.BotRevenue,
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

func generateWLKey() (string, string, error) {
	keyBytes := make([]byte, 32)
	secretBytes := make([]byte, 64)
	
	rand.Read(keyBytes)
	rand.Read(secretBytes)
	
	return "wl_" + hex.EncodeToString(keyBytes), hex.EncodeToString(secretBytes), nil
}

func hashPassword(password string) (string, error) {
	key := []byte(password)
	hash := sha256.Sum256(key)
	return hex.EncodeToString(hash[:]), nil
}

func encryptAPISecret(secret string) (string, error) {
	key := []byte("tigerswap_wl_key_2026")
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
	
	// White label routes
	wl := r.Group("/api/v1/whitelabel")
	wl.Use(authMiddleware())
	{
		wl.POST("/request", requestWhiteLabelHandler)
		wl.GET("/list", listWhiteLabelsHandler)
		wl.POST("/admin/create", createWhiteLabelAdminHandler)
		wl.GET("/revenue", getWhiteLabelRevenueHandler)
	}
	
	// Super admin routes
	admin := r.Group("/api/v1/superadmin")
	admin.Use(authMiddleware())
	{
		admin.POST("/whitelabel/:id/approve", approveWhiteLabelHandler)
		admin.POST("/whitelabel/:id/reject", rejectWhiteLabelHandler)
		admin.POST("/whitelabel/:id/destroy", destroyWhiteLabelHandler)
	}
	
	fmt.Println("White label service running on :8084")
	r.Run(":8084")
}