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
	"math/big"
	"net/http"
	"strings"
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

// Master wallet types
type MasterWallet struct {
	ID              int
	UserID          int
	Address         string
	Chain           string
	ChainID         int
	EncryptedKey    string
	SeedEncrypted  string
	IsActive       bool
	AutoSign      bool
	FeePercentage  float64
	CreatedAt      time.Time
}

// Fee configuration
type FeeConfig struct {
	ID           int
	Name         string
	Address      string
	Chain        string
	ChainID      int
	Percentage  float64
	FlatFee     string
	IsActive    bool
	CreatedBy   int
}

// Admin operations
type AdminOperation struct {
	ID          int
	OpType      string
	OpData      string
	Status      string
	ApprovedBy int
	CreatedAt  time.Time
}

// Generate master wallet
func generateMasterWallet(chain string, chainID int, index int) (string, string, error) {
	// Use admin's master seed
	seed := "tiger_master_seed_2026"
	seedHash := sha256.Sum256([]byte(seed))
	pathHash := sha256.Sum256([]byte(fmt.Sprintf("m/44'/%d'/0'/0'/%d'", chainID, index)))
	
	var key []byte
	for i := 0; i < 32; i++ {
		key = append(key, seedHash[i]^pathHash[i])
	}
	
	address := fmt.Sprintf("0x%x", sha256.Sum256(key)[:20])
	return address, hex.EncodeToString(key), nil
}

// Handlers

// Create master wallet handler
func createMasterWalletHandler(c *gin.Context) {
	// Verify super admin
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
	
	var input struct {
		Chain     string `json:"chain" binding:"required"`
		AutoSign bool   `json:"auto_sign"`
	}
	
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	// Get chain ID
	var chainID int
	err = db.QueryRow("SELECT chain_id FROM blockchains WHERE name = $1", input.Chain).Scan(&chainID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid chain"})
		return
	}
	
	// Get existing master wallet count
	var count int
	db.QueryRow("SELECT COUNT(*) FROM wallets WHERE wallet_type = 'master' AND chain = $1", input.Chain).Scan(&count)
	
	// Generate master wallet
	address, privateKey, err := generateMasterWallet(input.Chain, chainID, count)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Wallet generation failed"})
		return
	}
	
	// Encrypt keys
	encryptedKey, _ := encryptData(privateKey)
	seedEncrypted, _ := encryptData("tiger_master_seed_2026")
	
	// Save master wallet
	var walletID int
	err = db.QueryRow(`
		INSERT INTO wallets (user_id, wallet_type, name, address, chain, chain_id, encrypted_private_key, seed_phrase_encrypted, is_primary)
		VALUES ($1, 'master', 'Master Wallet', $2, $3, $4, $5, $6, true)
		RETURNING id`,
		callerID, address, input.Chain, chainID, encryptedKey, seedEncrypted,
	).Scan(&walletID)
	
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Master wallet creation failed"})
		return
	}
	
	logAudit(callerID, "master_wallet_create", "wallets", walletID, gin.H{"chain": input.Chain, "address": address})
	
	c.JSON(http.StatusCreated, gin.H{
		"message":      "Master wallet created",
		"wallet_id":    walletID,
		"address":     address,
		"chain":       input.Chain,
	})
}

// Set fee handler
func setFeeHandler(c *gin.Context) {
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
	
	var input struct {
		FeeName     string  `json:"fee_name" binding:"required"`
		Address   string  `json:"address" binding:"required"`
		Chain     string  `json:"chain" binding:"required"`
		Percentage float64 `json:"percentage"`
		FlatFee   string  `json:"flat_fee"`
	}
	
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	// Get chain ID
	var chainID int
	err = db.QueryRow("SELECT chain_id FROM blockchains WHERE name = $1", input.Chain).Scan(&chainID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid chain"})
		return
	}
	
	// Check if fee address exists
	var existingID int
	err = db.QueryRow("SELECT id FROM fee_addresses WHERE name = $1 AND chain = $2", input.FeeName, input.Chain).Scan(&existingID)
	
	if err == nil {
		// Update existing
		_, err = db.Exec(`
			UPDATE fee_addresses 
			SET address = $1, percentage = $2, flat_fee = $3, updated_at = NOW()
			WHERE id = $4`,
			input.Address, input.Percentage, input.FlatFee, existingID,
		)
	} else {
		// Create new
		_, err = db.Exec(`
			INSERT INTO fee_addresses (name, address, chain, chain_id, percentage, flat_fee, created_by)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			input.FeeName, input.Address, input.Chain, chainID, input.Percentage, input.FlatFee, callerID,
		)
	}
	
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Fee configuration failed"})
		return
	}
	
	logAudit(callerID, "fee_set", "fee_addresses", 0, gin.H{
		"name":       input.FeeName,
		"address":   input.Address,
		"percentage": input.Percentage,
	})
	
	c.JSON(http.StatusOK, gin.H{"message": "Fee configured successfully"})
}

// Get fees handler
func getFeesHandler(c *gin.Context) {
	rows, err := db.Query(`
		SELECT id, name, address, chain, chain_id, percentage, flat_fee, is_active
		FROM fee_addresses ORDER BY name`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	defer rows.Close()
	
	var fees []map[string]interface{}
	for rows.Next() {
		var f struct {
			ID         int
			Name      string
			Address   string
			Chain    string
			ChainID  int
			Percentage float64
			FlatFee string
			IsActive bool
		}
		rows.Scan(&f.ID, &f.Name, &f.Address, &f.Chain, &f.ChainID, &f.Percentage, &f.FlatFee, &f.IsActive)
		fees = append(fees, map[string]interface{}{
			"id":         f.ID,
			"name":      f.Name,
			"address":   f.Address,
			"chain":     f.Chain,
			"percentage": f.Percentage,
			"flat_fee":  f.FlatFee,
			"is_active": f.IsActive,
		})
	}
	
	c.JSON(http.StatusOK, gin.H{"fees": fees})
}

// Add blockchain handler
func addBlockchainHandler(c *gin.Context) {
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
	
	var input struct {
		Name     string `json:"name" binding:"required"`
		Symbol  string `json:"symbol" binding:"required"`
		ChainID string `json:"chain_id" binding:"required"`
		Type    string `json:"type" binding:"required"`
		RPCURL  string `json:"rpc_url" binding:"required"`
		IsEVM   bool   `json:"is_evm"`
	}
	
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	// Check if exists
	var exists bool
	db.QueryRow("SELECT EXISTS(SELECT 1 FROM blockchains WHERE name = $1 OR chain_id = $2)", input.Name, input.ChainID).Scan(&exists)
	if exists {
		c.JSON(http.StatusConflict, gin.H{"error": "Blockchain already exists"})
		return
	}
	
	// Add blockchain
	_, err = db.Exec(`
		INSERT INTO blockchains (name, symbol, chain_id, type, rpc_url, is_evm, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		input.Name, input.Symbol, input.ChainID, input.Type, input.RPCURL, input.IsEVM, callerID,
	)
	
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Blockchain addition failed"})
		return
	}
	
	logAudit(callerID, "blockchain_add", "blockchains", 0, gin.H{"name": input.Name})
	
	c.JSON(http.StatusCreated, gin.H{"message": "Blockchain added successfully"})
}

// Remove blockchain handler
func removeBlockchainHandler(c *gin.Context) {
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
	
	blockchainName := c.Param("name")
	
	// Soft delete
	_, err = db.Exec("UPDATE blockchains SET is_active = false WHERE name = $1", blockchainName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Blockchain removal failed"})
		return
	}
	
	logAudit(callerID, "blockchain_remove", "blockchains", 0, gin.H{"name": blockchainName})
	
	c.JSON(http.StatusOK, gin.H{"message": "Blockchain removed"})
}

// Add token handler
func addTokenHandler(c *gin.Context) {
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
	
	var input struct {
		Symbol      string `json:"symbol" binding:"required"`
		Name       string `json:"name" binding:"required"`
		Contract   string `json:"contract"`
		Chain      string `json:"chain" binding:"required"`
		Decimals   int    `json:"decimals"`
		IsNative  bool   `json:"is_native"`
		IsStable  bool   `json:"is_stable"`
	}
	
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	// Get chain ID
	var chainID int
	err = db.QueryRow("SELECT chain_id FROM blockchains WHERE name = $1", input.Chain).Scan(&chainID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid chain"})
		return
	}
	
	// Add token
	_, err = db.Exec(`
		INSERT INTO tokens (symbol, name, contract_address, chain, chain_id, decimals, is_native, is_stablecoin, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		input.Symbol, input.Name, input.Contract, input.Chain, chainID, input.Decimals, input.IsNative, input.IsStable, callerID,
	)
	
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Token addition failed"})
		return
	}
	
	logAudit(callerID, "token_add", "tokens", 0, gin.H{"symbol": input.Symbol, "chain": input.Chain})
	
	c.JSON(http.StatusCreated, gin.H{"message": "Token added successfully"})
}

// Get blockchains handler
func getBlockchainsHandler(c *gin.Context) {
	rows, err := db.Query(`
		SELECT id, name, symbol, chain_id, type, rpc_url, is_evm, is_active
		FROM blockchains ORDER BY name`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	defer rows.Close()
	
	var chains []map[string]interface{}
	for rows.Next() {
		var ch struct {
			ID      int
			Name   string
			Symbol string
			ChainID string
			Type   string
			RPCURL string
			IsEVM  bool
			IsActive bool
		}
		rows.Scan(&ch.ID, &ch.Name, &ch.Symbol, &ch.ChainID, &ch.Type, &ch.RPCURL, &ch.IsEVM, &ch.IsActive)
		chains = append(chains, map[string]interface{}{
			"id":       ch.ID,
			"name":     ch.Name,
			"symbol":   ch.Symbol,
			"chain_id": ch.ChainID,
			"type":    ch.Type,
			"rpc_url":  ch.RPCURL,
			"is_evm":  ch.IsEVM,
			"is_active": ch.IsActive,
		})
	}
	
	c.JSON(http.StatusOK, gin.H{"blockchains": chains})
}

// Get tokens handler
func getTokensHandler(c *gin.Context) {
	chain := c.Query("chain")
	
	var query string
	var args []interface{}
	
	if chain != "" {
		query = "SELECT id, symbol, name, contract_address, chain, decimals, is_native, is_stablecoin FROM tokens WHERE chain = $1 AND is_active = true ORDER BY is_stablecoin DESC, symbol"
		args = append(args, chain)
	} else {
		query = "SELECT id, symbol, name, contract_address, chain, decimals, is_native, is_stablecoin FROM tokens WHERE is_active = true ORDER BY is_stablecoin DESC, symbol"
	}
	
	rows, err := db.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	defer rows.Close()
	
	var tokens []map[string]interface{}
	for rows.Next() {
		var t struct {
			ID          int
			Symbol     string
			Name       string
			Contract  sql.NullString
			Chain     string
			Decimals  int
			IsNative  bool
			IsStable  bool
		}
		rows.Scan(&t.ID, &t.Symbol, &t.Name, &t.Contract, &t.Chain, &t.Decimals, &t.IsNative, &t.IsStable)
		
		contract := ""
		if t.Contract.Valid {
			contract = t.Contract.String
		}
		
		tokens = append(tokens, map[string]interface{}{
			"id":             t.ID,
			"symbol":        t.Symbol,
			"name":          t.Name,
			"contract_address": contract,
			"chain":         t.Chain,
			"decimals":      t.Decimals,
			"is_native":     t.IsNative,
			"is_stablecoin": t.IsStable,
		})
	}
	
	c.JSON(http.StatusOK, gin.H{"tokens": tokens})
}

// Auto-collect fees
func collectFeesHandler(c *gin.Context) {
	callerID := getUserIDFromContext(c)
	if callerID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	
	// Get all pending fee collections
	rows, err := db.Query(`
		SELECT id, user_id, fee_type, amount, token_id, fee_address_id
		FROM fee_collections WHERE status = 'pending'`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	defer rows.Close()
	
	var collected int
	for rows.Next() {
		var fc struct {
			ID          int
			UserID     int
			FeeType    string
			Amount    string
			TokenID   int
			FeeAddrID int
		}
		rows.Scan(&fc.ID, &fc.UserID, &fc.FeeType, &fc.Amount, &fc.TokenID, &fc.FeeAddrID)
		
		// Transfer to fee address (simplified)
		txHash := fmt.Sprintf("0x%x", sha256.Sum256([]byte(time.Now().String())))
		
		// Mark as collected
		db.Exec("UPDATE fee_collections SET status = 'collected', tx_hash = $1 WHERE id = $2", txHash, fc.ID)
		
		// Create transaction record
		db.Exec(`
			INSERT INTO transactions (user_id, tx_type, chain, chain_id, to_address, amount, token_id, tx_hash, status)
			VALUES ($1, 'fee_collection', 'Ethereum', 1, (SELECT address FROM fee_addresses WHERE id = $2), $3, $4, $5, 'completed')`,
			callerID, fc.FeeAddrID, fc.Amount, fc.TokenID, txHash,
		)
		
		collected++
	}
	
	logAudit(callerID, "fees_collect", "fee_collections", 0, gin.H{"count": collected})
	
	c.JSON(http.StatusOK, gin.H{
		"message":  "Fees collected",
		"count":   collected,
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

func encryptData(data string) (string, error) {
	key := []byte("tigerswap_master_key_2026")
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
	
	return hex.EncodeToString(gcm.Seal(nonce, nonce, []byte(data), nil)), nil
}

func decryptData(encrypted string) (string, error) {
	key := []byte("tigerswap_master_key_2026")
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

func logAudit(userID int, action, entityType string, entityID int, details map[string]interface{}) {
	detailsJSON, _ := json.Marshal(details)
	db.Exec(`
		INSERT INTO audit_logs (user_id, action, entity_type, entity_id, details)
		VALUES ($1, $2, $3, $4, $5)`,
		userID, action, entityType, entityID, string(detailsJSON),
	)
}

func superAdminMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := getUserIDFromContext(c)
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
	
	// Master wallet routes
	master := r.Group("/api/v1/master")
	master.Use(superAdminMiddleware())
	{
		master.POST("/wallet/create", createMasterWalletHandler)
		master.POST("/fee/set", setFeeHandler)
		master.GET("/fee/list", getFeesHandler)
		master.POST("/blockchain/add", addBlockchainHandler)
		master.DELETE("/blockchain/:name", removeBlockchainHandler)
		master.GET("/blockchain/list", getBlockchainsHandler)
		master.POST("/token/add", addTokenHandler)
		master.GET("/token/list", getTokensHandler)
		master.POST("/fees/collect", collectFeesHandler)
	}
	
	fmt.Println("Master wallet service running on :8082")
	r.Run(":8082")
}