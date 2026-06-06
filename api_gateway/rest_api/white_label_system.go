// ============================================================================
// TIGERSWAP WHITE LABEL PRODUCT SYSTEM
// Complete white label with 20% fee sharing, API key authorization, approval system
// 100/100 clone of TigerSwap with separate cloud/domain/storage
// ============================================================================

package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
)

// ============================================================================
// CONSTANTS
// ============================================================================

const (
	// Fee sharing
	FEE_SHARING_PERCENTAGE = 20 // 20% to TigerSwap admin

	// API key settings
	API_KEY_LENGTH     = 32
	API_SECRET_LENGTH = 64

	// Product settings
	MAX_WHITE_LABELS = 100
)

// ============================================================================
// MODELS
// ============================================================================

// WhiteLabelProduct represents white label product
type WhiteLabelProduct struct {
	ID                string           `json:"id"`
	Name              string           `json:"name"`
	Domain            string           `json:"domain"`
	CloudProvider     string           `json:"cloud_provider"`
	StorageProvider  string           `json:"storage_provider"`
	APIKey          string           `json:"api_key"` // TigerSwap API key for this product
	APISecret       string           `json:"api_secret,omitempty"`
	Status          WhiteLabelStatus `json:"status"`
	ApprovedBy      string          `json:"approved_by,omitempty"`
	ApprovedAt      *time.Time      `json:"approved_at,omitempty"`
	FeeSharingPercent int           `json:"fee_sharing_percent"`
	TotalEarnings   float64        `json:"total_earnings"`
	TotalFeesShared float64        `json:"total_fees_shared"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DestroyedAt     *time.Time    `json:"destroyed_at,omitempty"`
}

// WhiteLabelStatus represents product status
type WhiteLabelStatus string

const (
	WLStatusPending    WhiteLabelStatus = "pending"
	WLStatusApproved  WhiteLabelStatus = "approved"
	WLStatusActive   WhiteLabelStatus = "active"
	WLStatusSuspended WhiteLabelStatus = "suspended"
	WLStatusDestroyed WhiteLabelStatus = "destroyed"
)

// WhiteLabelConfig represents white label configuration
type WhiteLabelConfig struct {
	ID                string            `json:"id"`
	ProductID        string            `json:"product_id"`
	BrandName        string            `json:"brand_name"`
	BrandColor      string            `json:"brand_color"`
	LogoURL         string            `json:"logo_url"`
	FaviconURL      string            `json:"favicon_url"`
	PrimaryDomain  string           `json:"primary_domain"`
	CustomDomains []string        `json:"custom_domains"`
	SupportEmail   string           `json:"support_email"`
	SocialLinks  map[string]string `json:"social_links"`
	CustomCSS   string          `json:"custom_css"`
	CustomJS    string          `json:"custom_js"`
}

// WhiteLabelAdmin represents white label admin
type WhiteLabelAdmin struct {
	ID          string    `json:"id"`
	ProductID  string    `json:"product_id"`
	UserID     string    `json:"user_id"`
	Email      string    `json:"email"`
	Username   string    `json:"username"`
	Role       string    `json:"role"` // super_admin, admin, operator
	IsActive   bool      `json:"is_active"`
	CreatedAt  time.Time `json:"created_at"`
}

// WhiteLabelLicense represents license for white label
type WhiteLabelLicense struct {
	ID              string    `json:"id"`
	ProductID       string    `json:"product_id"`
	LicenseKey     string    `json:"license_key"`
	ExpiryDate     time.Time `json:"expiry_date"`
	IsRevoked      bool      `json:"is_revoked"`
	RevokedAt     *time.Time `json:"revoked_at,omitempty"`
	RevokedBy     string    `json:"revoked_by,omitempty"`
	RevokeReason  string    `json:"revoke_reason,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

// WhiteLabelAPIKey represents API key for white label
type WhiteLabelAPIKey struct {
	ID          string    `json:"id"`
	ProductID  string    `json:"product_id"`
	Key        string    `json:"key"`
	SecretHash string    `json:"secret_hash"`
	Permissions []string `json:"permissions"`
	RateLimit  int      `json:"rate_limit"`
	IsActive   bool     `json:"is_active"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// WhiteLabelEarning represents earnings tracking
type WhiteLabelEarning struct {
	ID          string    `json:"id"`
	ProductID  string    `json:"product_id"`
	EarningType string   `json:"earning_type"` // swap, trading, bot, listing
	Amount     float64  `json:"amount"`
	FeeAmount  float64  `json:"fee_amount"` // 20% to TigerSwap
	NetAmount  float64  `json:"net_amount"`
	Token      string   `json:"token"`
	ChainId    int      `json:"chain_id"`
	TxHash    string   `json:"tx_hash"`
	CreatedAt  time.Time `json:"created_at"`
}

// ============================================================================
// WHITE LABEL STORAGE
// ============================================================================

type WhiteLabelStore struct {
	mu sync.RWMutex

	// Products
	products map[string]*WhiteLabelProduct

	// Configurations
	configs map[string]*WhiteLabelConfig

	// Admins
	admins map[string]*WhiteLabelAdmin // productID -> admin

	// Licenses
	licenses map[string]*WhiteLabelLicense // licenseKey -> license

	// API Keys
	apiKeys map[string]*WhiteLabelAPIKey // key -> apiKey

	// Earnings
	earnings map[string][]*WhiteLabelEarning // productID -> earnings

	// Products by domain
	productsByDomain map[string]string // domain -> productID

	// Admin fee address
	adminFeeAddress string
}

// NewWhiteLabelStore creates new white label store
func NewWhiteLabelStore() *WhiteLabelStore {
	return &WhiteLabelStore{
		products:        make(map[string]*WhiteLabelProduct),
		configs:        make(map[string]*WhiteLabelConfig),
		admins:        make(map[string]*WhiteLabelAdmin),
		licenses:      make(map[string]*WhiteLabelLicense),
		apiKeys:       make(map[string]*WhiteLabelAPIKey),
		earnings:      make(map[string][]*WhiteLabelEarning),
		productsByDomain: make(map[string]string),
	}
}

// ============================================================================
// PRODUCT MANAGEMENT
// ============================================================================

// CreateProduct creates white label product
func (s *WhiteLabelStore) CreateProduct(name, domain, cloudProvider, storageProvider string) (*WhiteLabelProduct, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Validate domain
	if !isValidDomain(domain) {
		return nil, fmt.Errorf("invalid domain format")
	}

	// Check if domain exists
	if _, exists := s.productsByDomain[domain]; exists {
		return nil, fmt.Errorf("domain already registered")
	}

	// Check max products
	if len(s.products) >= MAX_WHITE_LABELS {
		return nil, fmt.Errorf("maximum white label products reached")
	}

	// Generate API key
	apiKey := generateRandomToken(API_KEY_LENGTH)
	apiSecret := generateRandomToken(API_SECRET_LENGTH)

	product := &WhiteLabelProduct{
		ID:                 generateUUID(),
		Name:              name,
		Domain:            domain,
		CloudProvider:     cloudProvider,
		StorageProvider:   storageProvider,
		APIKey:           apiKey,
		APISecret:         apiSecret,
		Status:           WLStatusPending,
		FeeSharingPercent: FEE_SHARING_PERCENTAGE,
		CreatedAt:         time.Now(),
		UpdatedAt:        time.Now(),
	}

	s.products[product.ID] = product
	s.productsByDomain[domain] = product.ID
	s.earnings[product.ID] = make([]*WhiteLabelEarning, 0)

	return product, nil
}

// ApproveProduct approves white label product
func (s *WhiteLabelStore) ApproveProduct(productID, approvedBy string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	product, ok := s.products[productID]
	if !ok {
		return fmt.Errorf("product not found")
	}

	if product.Status != WLStatusPending {
		return fmt.Errorf("product not pending approval")
	}

	product.Status = WLStatusApproved
	product.ApprovedBy = approvedBy
	now := time.Now()
	product.ApprovedAt = &now
	product.UpdatedAt = time.Now()

	// Generate license
	s.generateLicense(productID)

	return nil
}

// ActivateProduct activates white label product
func (s *WhiteLabelStore) ActivateProduct(productID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	product, ok := s.products[productID]
	if !ok {
		return fmt.Errorf("product not found")
	}

	if product.Status != WLStatusApproved {
		return fmt.Errorf("product must be approved first")
	}

	product.Status = WLStatusActive
	product.UpdatedAt = time.Now()

	return nil
}

// SuspendProduct suspends white label product
func (s *WhiteLabelStore) SuspendProduct(productID, reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	product, ok := s.products[productID]
	if !ok {
		return fmt.Errorf("product not found")
	}

	product.Status = WLStatusSuspended
	product.UpdatedAt = time.Now()

	// Revoke license
	if license, ok := s.licenses[productID]; ok {
		license.IsRevoked = true
		now := time.Now()
		license.RevokedAt = &now
		license.RevokeReason = reason
	}

	return nil
}

// DestroyProduct destroys white label product
func (s *WhiteLabelStore) DestroyProduct(productID, reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	product, ok := s.products[productID]
	if !ok {
		return fmt.Errorf("product not found")
	}

	product.Status = WLStatusDestroyed
	now := time.Now()
	product.DestroyedAt = &now
	product.UpdatedAt = time.Now()

	// Revoke license
	if license, ok := s.licenses[productID]; ok {
		license.IsRevoked = true
		license.RevokedAt = &now
		license.RevokeReason = reason
	}

	// Remove from domain mapping
	delete(s.productsByDomain, product.Domain)

	return nil
}

// GetProduct gets product by ID
func (s *WhiteLabelStore) GetProduct(productID string) (*WhiteLabelProduct, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	product, ok := s.products[productID]
	if !ok {
		return nil, fmt.Errorf("product not found")
	}

	return product, nil
}

// GetProductByDomain gets product by domain
func (s *WhiteLabelStore) GetProductByDomain(domain string) (*WhiteLabelProduct, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	productID, ok := s.productsByDomain[domain]
	if !ok {
		return nil, fmt.Errorf("product not found")
	}

	product, ok := s.products[productID]
	if !ok {
		return nil, fmt.Errorf("product not found")
	}

	return product, nil
}

// GetAllProducts gets all products
func (s *WhiteLabelStore) GetAllProducts() []*WhiteLabelProduct {
	s.mu.RLock()
	defer s.mu.RUnlock()

	products := make([]*WhiteLabelProduct, 0, len(s.products))
	for _, p := range s.products {
		products = append(products, p)
	}

	return products
}

// ValidateProductAPIKey validates product API key
func (s *WhiteLabelStore) ValidateProductAPIKey(productID, apiKey, apiSecret string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	product, ok := s.products[productID]
	if !ok {
		return fmt.Errorf("product not found")
	}

	if product.APIKey != apiKey {
		return fmt.Errorf("invalid API key")
	}

	// Verify secret
	if product.APISecret != apiSecret {
		return fmt.Errorf("invalid API secret")
	}

	if product.Status != WLStatusActive {
		return fmt.Errorf("product not active")
	}

	return nil
}

// ============================================================================
// LICENSE MANAGEMENT
// ============================================================================

// generateLicense generates license for product
func (s *WhiteLabelStore) generateLicense(productID string) {
	license := &WhiteLabelLicense{
		ID:         generateUUID(),
		ProductID: productID,
		LicenseKey: generateRandomToken(32),
		ExpiryDate: time.Now().Add(365 * 24 * time.Hour), // 1 year
		IsRevoked:  false,
		CreatedAt: time.Now(),
	}

	s.licenses[productID] = license
}

// GetLicense gets license for product
func (s *WhiteLabelStore) GetLicense(productID string) (*WhiteLabelLicense, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	license, ok := s.licenses[productID]
	if !ok {
		return nil, fmt.Errorf("license not found")
	}

	if license.IsRevoked {
		return nil, fmt.Errorf("license revoked")
	}

	if time.Now().After(license.ExpiryDate) {
		return nil, fmt.Errorf("license expired")
	}

	return license, nil
}

// RevokeLicense revokes license
func (s *WhiteLabelStore) RevokeLicense(productID, revokedBy, reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	license, ok := s.licenses[productID]
	if !ok {
		return fmt.Errorf("license not found")
	}

	license.IsRevoked = true
	now := time.Now()
	license.RevokedAt = &now
	license.RevokedBy = revokedBy
	license.RevokeReason = reason

	return nil
}

// ============================================================================
// CONFIGURATION MANAGEMENT
// ============================================================================

// CreateConfig creates white label configuration
func (s *WhiteLabelStore) CreateConfig(productID string, config *WhiteLabelConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.products[productID]; !ok {
		return fmt.Errorf("product not found")
	}

	config.ID = generateUUID()
	config.ProductID = productID
	s.configs[productID] = config

	return nil
}

// GetConfig gets configuration
func (s *WhiteLabelStore) GetConfig(productID string) (*WhiteLabelConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	config, ok := s.configs[productID]
	if !ok {
		return nil, fmt.Errorf("configuration not found")
	}

	return config, nil
}

// UpdateConfig updates configuration
func (s *WhiteLabelStore) UpdateConfig(productID string, updates map[string]interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	config, ok := s.configs[productID]
	if !ok {
		return fmt.Errorf("configuration not found")
	}

	// Apply updates
	if name, ok := updates["brand_name"].(string); ok {
		config.BrandName = name
	}
	if color, ok := updates["brand_color"].(string); ok {
		config.BrandColor = color
	}
	if logo, ok := updates["logo_url"].(string); ok {
		config.LogoURL = logo
	}
	if favicon, ok := updates["favicon_url"].(string); ok {
		config.FaviconURL = favicon
	}
	if domain, ok := updates["primary_domain"].(string); ok {
		config.PrimaryDomain = domain
	}
	if email, ok := updates["support_email"].(string); ok {
		config.SupportEmail = email
	}
	if css, ok := updates["custom_css"].(string); ok {
		config.CustomCSS = css
	}
	if js, ok := updates["custom_js"].(string); ok {
		config.CustomJS = js
	}

	return nil
}

// ============================================================================
// ADMIN MANAGEMENT
// ============================================================================

// CreateAdmin creates white label admin
func (s *WhiteLabelStore) CreateAdmin(productID, email, username, role string) (*WhiteLabelAdmin, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.products[productID]; !ok {
		return nil, fmt.Errorf("product not found")
	}

	admin := &WhiteLabelAdmin{
		ID:        generateUUID(),
		ProductID: productID,
		Email:     email,
		Username: username,
		Role:     role,
		IsActive: true,
		CreatedAt: time.Now(),
	}

	s.admins[productID] = admin
	return admin, nil
}

// GetAdmin gets admin for product
func (s *WhiteLabelStore) GetAdmin(productID string) (*WhiteLabelAdmin, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	admin, ok := s.admins[productID]
	if !ok {
		return nil, fmt.Errorf("admin not found")
	}

	return admin, nil
}

// ============================================================================
// EARNINGS MANAGEMENT
// ============================================================================

// RecordEarning records earning for product
func (s *WhiteLabelStore) RecordEarning(earning *WhiteLabelEarning) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	product, ok := s.products[earning.ProductID]
	if !ok {
		return fmt.Errorf("product not found")
	}

	// Calculate fee sharing (20%)
	earning.FeeAmount = earning.Amount * float64(FEE_SHARING_PERCENTAGE) / 100
	earning.NetAmount = earning.Amount - earning.FeeAmount

	earning.ID = generateUUID()
	earning.CreatedAt = time.Now()

	s.earnings[earning.ProductID] = append(s.earnings[earning.ProductID], earning)

	// Update totals
	product.TotalEarnings += earning.NetAmount
	product.TotalFeesShared += earning.FeeAmount

	return nil
}

// GetEarnings gets earnings for product
func (s *WhiteLabelStore) GetEarnings(productID string) []*WhiteLabelEarning {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.earnings[productID]
}

// GetTotalEarnings gets total earnings for product
func (s *WhiteLabelStore) GetTotalEarnings(productID string) (total, fee, net float64) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, e := range s.earnings[productID] {
		total += e.Amount
		fee += e.FeeAmount
		net += e.NetAmount
	}

	return total, fee, net
}

// ============================================================================
// API KEY MANAGEMENT
// ============================================================================

// CreateAPIKey creates API key for white label
func (s *WhiteLabelStore) CreateAPIKey(productID string, permissions []string, rateLimit int) (*WhiteLabelAPIKey, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.products[productID]; !ok {
		return nil, fmt.Errorf("product not found")
	}

	key := generateRandomToken(API_KEY_LENGTH)
	secretHash := hashString(generateRandomToken(API_SECRET_LENGTH))

	apiKey := &WhiteLabelAPIKey{
		ID:           generateUUID(),
		ProductID:    productID,
		Key:          key,
		SecretHash:    secretHash,
		Permissions: permissions,
		RateLimit:   rateLimit,
		IsActive:    true,
		ExpiresAt:   time.Now().Add(365 * 24 * time.Hour),
		CreatedAt:    time.Now(),
	}

	s.apiKeys[key] = apiKey
	return apiKey, nil
}

// ValidateAPIKey validates API key
func (s *WhiteLabelStore) ValidateAPIKey(key string) (*WhiteLabelAPIKey, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	apiKey, ok := s.apiKeys[key]
	if !ok {
		return nil, fmt.Errorf("API key not found")
	}

	if !apiKey.IsActive {
		return nil, fmt.Errorf("API key inactive")
	}

	if time.Now().After(apiKey.ExpiresAt) {
		return nil, fmt.Errorf("API key expired")
	}

	// Update last used
	now := time.Now()
	apiKey.LastUsedAt = &now

	return apiKey, nil
}

// RevokeAPIKey revokes API key
func (s *WhiteLabelStore) RevokeAPIKey(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	apiKey, ok := s.apiKeys[key]
	if !ok {
		return fmt.Errorf("API key not found")
	}

	apiKey.IsActive = false
	return nil
}

// ============================================================================
// SET ADMIN FEE ADDRESS
// ============================================================================

// SetAdminFeeAddress sets admin fee address for white label
func (s *WhiteLabelStore) SetAdminFeeAddress(address string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Validate address format
	if !regexp.MustCompile(`^0x[a-fA-F0-9]{40}$`).MatchString(address) {
		return fmt.Errorf("invalid address format")
	}

	s.adminFeeAddress = address
	return nil
}

// GetAdminFeeAddress gets admin fee address
func (s *WhiteLabelStore) GetAdminFeeAddress() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.adminFeeAddress
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

func generateRandomToken(length int) string {
	return generateRandomHex(length)
}

func generateRandomHex(length int) string {
	b := make([]byte, length)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func generateUUID() string {
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		generateRandomHex(8),
		generateRandomHex(4),
		generateRandomHex(4),
		generateRandomHex(4),
		generateRandomHex(12),
	)
}

func isValidDomain(domain string) bool {
	pattern := `^[a-zA-Z0-9][a-zA-Z0-9-]{0,61}[a-zA-Z0-9]?(\.[a-zA-Z]{2,})+$`
	matched, _ := regexp.MatchString(pattern, domain)
	return matched && len(domain) <= 253
}

func hashString(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func sanitizeInput(input string) string {
	input = strings.TrimSpace(input)
	input = html.EscapeString(input)
	return input
}

// ============================================================================
// HTTP HANDLERS
// ============================================================================

// WhiteLabelHandler handles white label requests
type WhiteLabelHandler struct {
	store *WhiteLabelStore
}

// NewWhiteLabelHandler creates new handler
func NewWhiteLabelHandler(store *WhiteLabelStore) *WhiteLabelHandler {
	return &WhiteLabelHandler{store: store}
}

// CreateProductRequest represents create product request
type CreateProductRequest struct {
	Name           string `json:"name"`
	Domain        string `json:"domain"`
	CloudProvider string `json:"cloud_provider"`
	StorageProvider string `json:"storage_provider"`
}

// HandleCreateProduct handles create product request
func (h *WhiteLabelHandler) HandleCreateProduct(w http.ResponseWriter, r *http.Request) {
	var req CreateProductRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	// Sanitize inputs
	req.Name = sanitizeInput(req.Name)
	req.Domain = sanitizeInput(strings.ToLower(req.Domain))

	product, err := h.store.CreateProduct(req.Name, req.Domain, req.CloudProvider, req.StorageProvider)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(product)
}

// HandleApproveProduct handles approve product request
func (h *WhiteLabelHandler) HandleApproveProduct(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ProductID string `json:"product_id"`
		ApprovedBy string `json:"approved_by"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	if err := h.store.ApproveProduct(req.ProductID, req.ApprovedBy); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"success": true,
		"message": "product approved",
	})
}

// HandleDestroyProduct handles destroy product request
func (h *WhiteLabelHandler) HandleDestroyProduct(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ProductID string `json:"product_id"`
		Reason  string `json:"reason"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	if err := h.store.DestroyProduct(req.ProductID, req.Reason); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"success": true,
		"message": "product destroyed",
	})
}

// HandleGetProducts handles get products request
func (h *WhiteLabelHandler) HandleGetProducts(w http.ResponseWriter, r *http.Request) {
	products := h.store.GetAllProducts()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(products)
}

// HandleValidateAPIKey handles validate API key request
func (h *WhiteLabelHandler) HandleValidateAPIKey(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ProductID string `json:"product_id"`
		APIKey   string `json:"api_key"`
		APISecret string `json:"api_secret"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	if err := h.store.ValidateProductAPIKey(req.ProductID, req.APIKey, req.APISecret); err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{
		"valid": true,
	})
}

// HandleRecordEarning handles record earning request
func (h *WhiteLabelHandler) HandleRecordEarning(w http.ResponseWriter, r *http.Request) {
	var earning WhiteLabelEarning
	if err := json.NewDecoder(r.Body).Decode(&earning); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	if err := h.store.RecordEarning(&earning); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"success": true,
		"message": "earning recorded",
	})
}

// ============================================================================
// GLOBAL INSTANCE
// ============================================================================

var whiteLabelStore *WhiteLabelStore

// InitWhiteLabel initializes white label system
func InitWhiteLabel() {
	whiteLabelStore = NewWhiteLabelStore()
}

// GetWhiteLabelStore returns white label store
func GetWhiteLabelStore() *WhiteLabelStore {
	return whiteLabelStore
}