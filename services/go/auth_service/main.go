package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

var db *sql.DB

// Database connection
func initDB() error {
	var err error
	connStr := "host=localhost port=5432 user=tigerswap password=securepass dbname=tigerswap sslmode=disable"
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		return err
	}
	return db.Ping()
}

// User types
type User struct {
	ID               int
	Username         string
	Email           string
	PasswordHash     string
	Role            string
	Status          string
	TwoFactorSecret  string
	TwoFactorEnabled bool
	FailedAttempts  int
	LockedUntil     sql.NullTime
	CreatedAt       time.Time
	UpdatedAt       time.Time
	LastLoginAt     sql.NullTime
	WhiteLabelID    sql.NullInt64
	IsWLAdmin      bool
}

// Security functions
func hashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func verifyPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func generateSessionToken() (string, error) {
	bytes := make([]byte, 64)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

func generateAPIKey() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return "ts_" + hex.EncodeToString(bytes), nil
}

func generateAPIKeySecret() (string, error) {
	bytes := make([]byte, 64)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func hashData(data string) string {
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

func encryptData(data string, key []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	
	seal := gcm.Seal(nonce, nonce, []byte(data), nil)
	return base64.StdEncoding.EncodeToString(seal), nil
}

func decryptData(encrypted string, key []byte) (string, error) {
	data, err := base64.StdEncoding.DecodeString(encrypted)
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
func registerHandler(c *gin.Context) {
	var input struct {
		Username   string `json:"username" binding:"required,min=3,max=50"`
		Email      string `json:"email" binding:"required,email"`
		Password  string `json:"password" binding:"required,min=8"`
		Referrer   string `json:"referrer"`
	}
	
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	// Validate email format
	if !strings.Contains(input.Email, "@") || !strings.Contains(input.Email, ".") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid email format"})
		return
	}
	
	// Check if user exists
	var exists bool
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE email = $1 OR username = $2)", input.Email, input.Username).Scan(&exists)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	
	if exists {
		c.JSON(http.StatusConflict, gin.H{"error": "User already exists"})
		return
	}
	
	// Hash password
	passwordHash, err := hashPassword(input.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Password hashing failed"})
		return
	}
	
	// Determine referrer
	referrerID := 0
	if input.Referrer != "" {
		db.QueryRow("SELECT id FROM users WHERE username = $1", input.Referrer).Scan(&referrerID)
	}
	
	// Create user
	var userID int
	err = db.QueryRow(`
		INSERT INTO users (username, email, password_hash, role, status, referrer_id)
		VALUES ($1, $2, $3, 'user', 'active', $4)
		RETURNING id`,
		input.Username, input.Email, passwordHash, referrerID,
	).Scan(&userID)
	
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "User creation failed"})
		return
	}
	
	// Generate default wallet for user
	walletAddress := generateWalletAddress(userID, "Ethereum", 1)
	_, err = db.Exec(`
		INSERT INTO wallets (user_id, wallet_type, name, address, chain, chain_id, is_primary)
		VALUES ($1, 'user', 'Main Wallet', $2, 'Ethereum', 1, true)`,
		userID, walletAddress,
	)
	
	// Log audit
	logAudit(userID, "user_register", "users", userID, gin.H{"username": input.Username})
	
	c.JSON(http.StatusCreated, gin.H{
		"message": "User registered successfully",
		"user_id": userID,
	})
}

func loginHandler(c *gin.Context) {
	var input struct {
		Email    string `json:"email" binding:"required"`
		Password string `json:"password" binding:"required"`
		OTP     string `json:"otp"`
	}
	
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	// Get user
	var user User
	err := db.QueryRow(`
		SELECT id, username, email, password_hash, role, status, two_factor_secret, two_factor_enabled, failed_attempts, locked_until
		FROM users WHERE email = $1 OR username = $1`,
		input.Email,
	).Scan(&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.Role, &user.Status, &user.TwoFactorSecret, &user.TwoFactorEnabled, &user.FailedAttempts, &user.LockedUntil)
	
	if err == sql.ErrNoRows {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}
	
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	
	// Check if locked
	if user.LockedUntil.Valid && user.LockedUntil.Time.After(time.Now()) {
		c.JSON(http.StatusLocked, gin.H{"error": "Account locked. Try again later"})
		return
	}
	
	// Verify password
	if !verifyPassword(input.Password, user.PasswordHash) {
		db.Exec("UPDATE users SET failed_attempts = failed_attempts + 1 WHERE id = $1", user.ID)
		
		if user.FailedAttempts >= 4 {
			db.Exec("UPDATE users SET locked_until = $1 WHERE id = $2", time.Now().Add(15*time.Minute), user.ID)
		}
		
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}
	
	// Check 2FA
	if user.TwoFactorEnabled {
		if input.OTP == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "2FA code required"})
			return
		}
		
		if input.OTP != user.TwoFactorSecret {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid 2FA code"})
			return
		}
	}
	
	// Generate session
	sessionToken, err := generateSessionToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Session generation failed"})
		return
	}
	
	ip := c.ClientIP()
	userAgent := c.Request.UserAgent()
	expiresAt := time.Now().Add(24 * time.Hour)
	
	_, err = db.Exec(`
		INSERT INTO sessions (user_id, session_token, ip_address, user_agent, expires_at)
		VALUES ($1, $2, $3, $4, $5)`,
		user.ID, sessionToken, ip, userAgent, expiresAt,
	)
	
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Session creation failed"})
		return
	}
	
	db.Exec("UPDATE users SET last_login_at = NOW(), failed_attempts = 0 WHERE id = $1", user.ID)
	
	logAudit(user.ID, "user_login", "users", user.ID, gin.H{"ip": ip})
	
	c.SetCookie("session_token", sessionToken, 86400, "/", "", false, true)
	
	c.JSON(http.StatusOK, gin.H{
		"message":       "Login successful",
		"user_id":      user.ID,
		"username":     user.Username,
		"role":        user.Role,
		"session_token": sessionToken,
	})
}

func logoutHandler(c *gin.Context) {
	sessionToken, err := c.Cookie("session_token")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No session"})
		return
	}
	
	db.Exec("UPDATE sessions SET is_active = false WHERE session_token = $1", sessionToken)
	c.SetCookie("session_token", "", -1, "/", "", false, true)
	
	c.JSON(http.StatusOK, gin.H{"message": "Logged out"})
}

func changePasswordHandler(c *gin.Context) {
	userID := getUserIDFromSession(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	
	var input struct {
		CurrentPassword string `json:"current_password" binding:"required"`
		NewPassword   string `json:"new_password" binding:"required,min=8"`
	}
	
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	var currentHash string
	err := db.QueryRow("SELECT password_hash FROM users WHERE id = $1", userID).Scan(&currentHash)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "User not found"})
		return
	}
	
	if !verifyPassword(input.CurrentPassword, currentHash) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Current password incorrect"})
		return
	}
	
	newHash, err := hashPassword(input.NewPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Password hashing failed"})
		return
	}
	
	_, err = db.Exec("UPDATE users SET password_hash = $1, updated_at = NOW() WHERE id = $2", newHash, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Password update failed"})
		return
	}
	
	db.Exec("UPDATE sessions SET is_active = false WHERE user_id = $1 AND session_token != $2", userID, c.GetCookie("session_token"))
	
	logAudit(userID, "password_change", "users", userID, nil)
	
	c.JSON(http.StatusOK, gin.H{"message": "Password changed successfully"})
}

func enable2FAHandler(c *gin.Context) {
	userID := getUserIDFromSession(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	
	secret := generateSessionToken()[:16]
	
	_, err := db.Exec("UPDATE users SET two_factor_secret = $1, two_factor_enabled = true WHERE id = $2", secret, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "2FA enable failed"})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"message": "2FA enabled",
		"secret": secret,
	})
}

func disable2FAHandler(c *gin.Context) {
	userID := getUserIDFromSession(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	
	var input struct {
		OTP string `json:"otp" binding:"required"`
	}
	
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	var secret string
	err := db.QueryRow("SELECT two_factor_secret FROM users WHERE id = $1", userID).Scan(&secret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "User not found"})
		return
	}
	
	if input.OTP != secret {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid OTP"})
		return
	}
	
	_, err = db.Exec("UPDATE users SET two_factor_secret = NULL, two_factor_enabled = false WHERE id = $1", userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "2FA disable failed"})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"message": "2FA disabled"})
}

func createAdminHandler(c *gin.Context) {
	callerID := getUserIDFromSession(c)
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
	
	var input struct {
		Username    string `json:"username" binding:"required"`
		Email      string `json:"email" binding:"required,email"`
		Password  string `json:"password" binding:"required"`
		AdminLevel string `json:"admin_level" binding:"required"`
	}
	
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	passwordHash, err := hashPassword(input.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Password hashing failed"})
		return
	}
	
	var levelID int
	err = db.QueryRow("SELECT id FROM admin_levels WHERE name = $1", input.AdminLevel).Scan(&levelID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid admin level"})
		return
	}
	
	var userID int
	err = db.QueryRow(`
		INSERT INTO users (username, email, password_hash, role, status)
		VALUES ($1, $2, $3, 'admin', 'active')
		RETURNING id`,
		input.Username, input.Email, passwordHash,
	).Scan(&userID)
	
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Admin creation failed"})
		return
	}
	
	_, err = db.Exec(`
		INSERT INTO admin_permissions (user_id, admin_level_id, permissions, granted_by)
		VALUES ($1, $2, (SELECT permissions FROM admin_levels WHERE id = $2), $3)`,
		userID, levelID, callerID,
	)
	
	logAudit(callerID, "admin_create", "users", userID, gin.H{"level": input.AdminLevel})
	
	c.JSON(http.StatusCreated, gin.H{
		"message": "Admin created successfully",
		"user_id": userID,
	})
}

func getUserIDFromSession(c *gin.Context) int {
	sessionToken, err := c.Cookie("session_token")
	if err != nil {
		return 0
	}
	
	var userID int
	var expiresAt time.Time
	err = db.QueryRow(`
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

func generateWalletAddress(userID int, chain string, chainID int) string {
	return fmt.Sprintf("0x%x%x%x", userID, chainID, time.Now().Unix())
}

func authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := getUserIDFromSession(c)
		if userID == 0 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			c.Abort()
			return
		}
		c.Set("user_id", userID)
		c.Next()
	}
}

func adminMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := getUserIDFromSession(c)
		if userID == 0 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			c.Abort()
			return
		}
		
		var role string
		err := db.QueryRow("SELECT role FROM users WHERE id = $1", userID).Scan(&role)
		if err != nil || (role != "admin" && role != "super_admin") {
			c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
			c.Abort()
			return
		}
		
		c.Set("user_id", userID)
		c.Set("role", role)
		c.Next()
	}
}

func superAdminMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := getUserIDFromSession(c)
		if userID == 0 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			c.Abort()
			return
		}
		
		var role string
		err := db.QueryRow("SELECT role FROM users WHERE id = $1", userID).Scan(&role)
		if err != nil || role != "super_admin" {
			c.JSON(http.StatusForbidden, gin.H{"error": "Super admin access required"})
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
	
	r.POST("/api/v1/register", registerHandler)
	r.POST("/api/v1/login", loginHandler)
	r.POST("/api/v1/logout", logoutHandler)
	
	auth := r.Group("/api/v1")
	auth.Use(authMiddleware())
	{
		auth.POST("/change-password", changePasswordHandler)
		auth.POST("/enable-2fa", enable2FAHandler)
		auth.POST("/disable-2fa", disable2FAHandler)
	}
	
	admin := r.Group("/api/v1/admin")
	admin.Use(adminMiddleware())
	{
		admin.POST("/create-admin", createAdminHandler)
	}
	
	superAdmin := r.Group("/api/v1/superadmin")
	superAdmin.Use(superAdminMiddleware())
	{
		superAdmin.POST("/create-super-admin", createAdminHandler)
	}
	
	fmt.Println("Auth service running on :8080")
	r.Run(":8080")
}