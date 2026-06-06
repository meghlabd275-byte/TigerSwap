// ============================================================================
// WHITE LABEL LICENSE & DEPLOYMENT API
// Complete license enforcement for white label products
// ============================================================================

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// ============================================================================
// DATA MODELS
// ============================================================================

// WhiteLabelLicense - License for deployment
type WhiteLabelLicense struct {
	ID                       string    `json:"id"`
	ClientID                 string    `json:"client_id"`
	LicenseKey              string    `json:"license_key"`
	LicenseSecretHash       string    `json:"license_secret_hash"`
	DeploymentDomain       string    `json:"deployment_domain"`
	DeploymentCloudProvider string  `json:"deployment_cloud_provider"`
	DeploymentRegion       string    `json:"deployment_region"`
	DeploymentStorageBucket string   `json:"deployment_storage_bucket"`
	DeploymentAPIEndpoint  string    `json:"deployment_api_endpoint"`
	Status                  string    `json:"status"`
	ActivatedAt            string    `json:"activated_at"`
	SuspendedAt            string    `json:"suspended_at"`
	RevokedAt              string    `json:"revoked_at"`
	RevokeReason          string    `json:"revoke_reason"`
	ExpiresAt             string    `json:"expires_at"`
	MaxConcurrentUsers    int       `json:"max_concurrent_users"`
	MaxAPICallsPerMonth   int       `json:"max_api_calls_per_month"`
	MaxVolumeUSDPerMonth  float64   `json:"max_volume_usd_per_month"`
	CurrentUsers          int       `json:"current_users"`
	APICallsThisMonth      int       `json:"api_calls_this_month"`
	VolumeThisMonthUSD    float64   `json:"volume_this_month_usd"`
	LastValidatedAt       string    `json:"last_validated_at"`
	LastValidationIP     string    `json:"last_validation_ip"`
	ValidationFailures  int       `json:"validation_failures"`
	CreatedAt           string    `json:"created_at"`
	UpdatedAt           string    `json:"updated_at"`
}

// WhiteLabelDeployment - Deployment configuration
type WhiteLabelDeployment struct {
	ID                    string  `json:"id"`
	ClientID              string  `json:"client_id"`
	LicenseID            string  `json:"license_id"`
	DeploymentName       string  `json:"deployment_name"`
	DeploymentType       string  `json:"deployment_type"`
	Version              string  `json:"version"`
	CloudProvider        string  `json:"cloud_provider"`
	CloudRegion          string  `json:"cloud_region"`
	CloudProjectID       string  `json:"cloud_project_id"`
	CloudBucket          string  `json:"cloud_bucket"`
	DatabaseURL          string  `json:"database_url"`
	DatabaseName         string  `json:"database_name"`
	RedisURL             string  `json:"redis_url"`
	Domain               string  `json:"domain"`
	SSLCertificateARN   string  `json:"ssl_certificate_arn"`
	CDNEndpoint         string  `json:"cdn_endpoint"`
	Status               string  `json:"status"`
	DeployedAt           string  `json:"deployed_at"`
	StoppedAt            string  `json:"stopped_at"`
	DestroyedAt          string  `json:"destroyed_at"`
	DestroyReason       string  `json:"destroy_reason"`
	MonthlyCostUSD       float64 `json:"monthly_cost_usd"`
	LastBilledAt        string  `json:"last_billed_at"`
	CreatedAt           string  `json:"created_at"`
	UpdatedAt           string  `json:"updated_at"`
}

// WhiteLabelFeature - Feature flags
type WhiteLabelFeature struct {
	ID                  string `json:"id"`
	ClientID            string `json:"client_id"`
	EnableSwap         bool   `json:"enable_swap"`
	EnableTrading      bool   `json:"enable_trading"`
	EnableLimitOrders  bool   `json:"enable_limit_orders"`
	EnableStopLoss    bool   `json:"enable_stop_loss"`
	EnableOrderBook   bool   `json:"enable_order_book"`
	EnablePool        bool   `json:"enable_pool"`
	EnableFarming     bool   `json:"enable_farming"`
	EnableBridge      bool   `json:"enable_bridge"`
	EnableLending     bool   `json:"enable_lending"`
	EnablePerpetuals  bool   `json:"enable_perpetuals"`
	EnableOptions    bool   `json:"enable_options"`
	EnableNFT        bool   `json:"enable_nft"`
	EnableMMBot      bool   `json:"enable_mm_bot"`
	EnableArbitrageBot bool  `json:"enable_arbitrage_bot"`
	EnableSniperBot   bool   `json:"enable_sniper_bot"`
	EnableLiquidityBot bool  `json:"enable_liquidity_bot"`
	EnableFrontRunBot bool   `json:"enable_front_run_bot"`
	EnableMEVBot      bool   `json:"enable_mev_bot"`
	EnableSandwichBot  bool   `json:"enable_sandwich_bot"`
	EnableFlashLoanBot bool  `json:"enable_flash_loan_bot"`
	EnableCrossChainBot bool   `json:"enable_cross_chain_bot"`
	EnablePerpHedgeBot bool  `json:"enable_perp_hedge_bot"`
	EnableCreateWallet bool   `json:"enable_create_wallet"`
	EnableImportWallet bool   `json:"enable_import_wallet"`
	EnableHDWallet    bool   `json:"enable_hd_wallet"`
	EnableMasterWallet bool  `json:"enable_master_wallet"`
	EnableMultisig    bool   `json:"enable_multisig"`
	EnableAutoSign    bool   `json:"enable_auto_sign"`
	EnableAPIAccess   bool   `json:"enable_api_access"`
	EnableWebhook     bool   `json:"enable_webhook"`
	EnableWebSocket   bool   `json:"enable_webSocket"`
	EnableCustomBrand  bool   `json:"enable_custom_brand"`
	EnableCustomTokens bool   `json:"enable_custom_tokens"`
	EnableCustomChains bool  `json:"enable_custom_chains"`
	CreatedAt         string `json:"created_at"`
	UpdatedAt        string `json:"updated_at"`
}

// ============================================================================
// LICENSE MANAGEMENT API
// ============================================================================

// GenerateLicenseKey - Generate unique license key
func generateLicenseKey() string {
	b := make([]byte, 32)
	rand.Read(b)
	return fmt.Sprintf("WL-%s", hex.EncodeToString(b)[:32])
}

// GenerateLicenseSecret - Generate license secret
func generateLicenseSecret() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// HashSecret - Hash license secret for storage
func hashSecret(secret string) string {
	h := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(h[:])
}

// CreateWhiteLabelLicense - Create new license (Super Admin only)
func (api *TigerSwapAPI) CreateWhiteLabelLicense(w http.ResponseWriter, r *http.Request) {
	if !api.isSuperAdmin(r) {
		api.JSON(w, http.StatusForbidden, map[string]string{"error": "Super admin access required"})
		return
	}

	var req struct {
		ClientID               string `json:"client_id"`
		DeploymentDomain       string `json:"deployment_domain"`
		DeploymentCloudProvider string `json:"deployment_cloud_provider"`
		DeploymentRegion       string `json:"deployment_region"`
		DeploymentStorageBucket string `json:"deployment_storage_bucket"`
		DeploymentAPIEndpoint  string `json:"deployment_api_endpoint"`
		MaxConcurrentUsers    int    `json:"max_concurrent_users"`
		MaxAPICallsPerMonth   int    `json:"max_api_calls_per_month"`
		MaxVolumeUSDPerMonth  float64 `json:"max_volume_usd_per_month"`
		ExpiresAt            string `json:"expires_at"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.JSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request"})
		return
	}

	// Check domain uniqueness
	if api.licensesByDomain[req.DeploymentDomain] != nil {
		api.JSON(w, http.StatusConflict, map[string]string{"error": "Domain already in use"})
		return
	}

	// Generate license key and secret
	licenseKey := generateLicenseKey()
	licenseSecret := generateLicenseSecret()
	secretHash := hashSecret(licenseSecret)

	// Create license
	license := WhiteLabelLicense{
		ID:                        generateUUID(),
		ClientID:                  req.ClientID,
		LicenseKey:                 licenseKey,
		LicenseSecretHash:         secretHash,
		DeploymentDomain:         req.DeploymentDomain,
		DeploymentCloudProvider:  req.DeploymentCloudProvider,
		DeploymentRegion:        req.DeploymentRegion,
		DeploymentStorageBucket: req.DeploymentStorageBucket,
		DeploymentAPIEndpoint:   req.DeploymentAPIEndpoint,
		Status:                  "active",
		ActivatedAt:             time.Now().UTC().Format(time.RFC3339),
		ExpiresAt:               req.ExpiresAt,
		MaxConcurrentUsers:     req.MaxConcurrentUsers,
		MaxAPICallsPerMonth:     req.MaxAPICallsPerMonth,
		MaxVolumeUSDPerMonth:   req.MaxVolumeUSDPerMonth,
		CreatedAt:               time.Now().UTC().Format(time.RFC3339),
		UpdatedAt:               time.Now().UTC().Format(time.RFC3339),
	}

	// Store license
	api.licenses[licenseKey] = &license
	api.licensesByDomain[req.DeploymentDomain] = &license
	api.licensesByClient[req.ClientID] = &license

	api.JSON(w, http.StatusCreated, map[string]interface{}{
		"license_key":    licenseKey,
		"license_secret": licenseSecret, // Only returned once!
		"license":       license,
		"message":       "License created. Save secret - it cannot be retrieved again.",
	})
}

// ValidateLicense - Validate license key (called on every API request)
func (api *TigerSwapAPI) ValidateLicense(w http.ResponseWriter, r *http.Request) {
	var req struct {
		LicenseKey    string `json:"license_key"`
		LicenseSecret string `json:"license_secret"`
		ClientID     string `json:"client_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.JSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request"})
		return
	}

	// Find license
	license, ok := api.licenses[req.LicenseKey]
	if !ok {
		api.JSON(w, http.StatusUnauthorized, map[string]string{
			"error":         "Invalid license key. Contact TigerSwap admin.",
			"error_code":   "LICENSE_INVALID",
		})
		return
	}

	// Verify secret
	secretHash := hashSecret(req.LicenseSecret)
	if secretHash != license.LicenseSecretHash {
		license.ValidationFailures++
		api.JSON(w, http.StatusUnauthorized, map[string]string{
			"error":         "Invalid license secret. Contact TigerSwap admin.",
			"error_code":   "LICENSE_INVALID",
		})
		return
	}

	// Verify client ID matches
	if license.ClientID != req.ClientID {
		api.JSON(w, http.StatusUnauthorized, map[string]string{
			"error":         "License mismatch. Contact TigerSwap admin.",
			"error_code":   "LICENSE_MISMATCH",
		})
		return
	}

	// Check status
	switch license.Status {
	case "suspended":
		api.JSON(w, http.StatusForbidden, map[string]string{
			"error":         "License suspended. Contact TigerSwap admin.",
			"error_code":   "LICENSE_SUSPENDED",
		})
		return
	case "revoked":
		api.JSON(w, http.StatusForbidden, map[string]string{
			"error":         "License revoked. Contact TigerSwap admin.",
			"error_code":   "LICENSE_REVOKED",
		})
		return
	case "pending":
		api.JSON(w, http.StatusForbidden, map[string]string{
			"error":         "License pending approval. Contact TigerSwap admin.",
			"error_code":   "LICENSE_PENDING",
		})
		return
	}

	// Check expiration
	if license.ExpiresAt != "" {
		expires, _ := time.Parse(time.RFC3339, license.ExpiresAt)
		if expires.Before(time.Now()) {
			api.JSON(w, http.StatusForbidden, map[string]string{
				"error":         "License expired. Contact TigerSwap admin.",
				"error_code":   "LICENSE_EXPIRED",
			})
			return
		}
	}

	// Update validation
	license.LastValidatedAt = time.Now().UTC().Format(time.RFC3339)
	license.LastValidationIP = r.RemoteAddr
	license.ValidationFailures = 0

	// Get features
	features := api.getClientFeatures(license.ClientID)

	api.JSON(w, http.StatusOK, map[string]interface{}{
		"valid":         true,
		"license":      license,
		"features":    features,
		"message":     "License validated successfully",
	})
}

// RevokeLicense - Revoke license (destroy product)
func (api *TigerSwapAPI) RevokeLicense(w http.ResponseWriter, r *http.Request) {
	if !api.isSuperAdmin(r) {
		api.JSON(w, http.StatusForbidden, map[string]string{"error": "Super admin access required"})
		return
	}

	var req struct {
		LicenseKey   string `json:"license_key"`
		Reason      string `json:"reason"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.JSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request"})
		return
	}

	license, ok := api.licenses[req.LicenseKey]
	if !ok {
		api.JSON(w, http.StatusNotFound, map[string]string{"error": "License not found"})
		return
	}

	// Revoke license
	license.Status = "revoked"
	license.RevokedAt = time.Now().UTC().Format(time.RFC3339)
	license.RevokeReason = req.Reason
	license.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	// Remove from active licenses
	delete(api.licensesByDomain, license.DeploymentDomain)
	delete(api.licensesByClient, license.ClientID)

	api.JSON(w, http.StatusOK, map[string]interface{}{
		"license": license,
		"message": "License revoked. Product destroyed.",
	})
}

// SuspendLicense - Suspend license temporarily
func (api *TigerSwapAPI) SuspendLicense(w http.ResponseWriter, r *http.Request) {
	if !api.isSuperAdmin(r) {
		api.JSON(w, http.StatusForbidden, map[string]string{"error": "Super admin access required"})
		return
	}

	var req struct {
		LicenseKey string `json:"license_key"`
		Reason   string `json:"reason"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.JSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request"})
		return
	}

	license, ok := api.licenses[req.LicenseKey]
	if !ok {
		api.JSON(w, http.StatusNotFound, map[string]string{"error": "License not found"})
		return
	}

	license.Status = "suspended"
	license.SuspendedAt = time.Now().UTC().Format(time.RFC3339)
	license.RevokeReason = req.Reason
	license.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	api.JSON(w, http.StatusOK, map[string]interface{}{
		"license": license,
		"message": "License suspended. Product paused.",
	})
}

// ActivateLicense - Reactivate suspended license
func (api *TigerSwapAPI) ActivateLicense(w http.ResponseWriter, r *http.Request) {
	if !api.isSuperAdmin(r) {
		api.JSON(w, http.StatusForbidden, map[string]string{"error": "Super admin access required"})
		return
	}

	var req struct {
		LicenseKey string `json:"license_key"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.JSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request"})
		return
	}

	license, ok := api.licenses[req.LicenseKey]
	if !ok {
		api.JSON(w, http.StatusNotFound, map[string]string{"error": "License not found"})
		return
	}

	// Reactivate
	oldStatus := license.Status
	license.Status = "active"
	license.SuspendedAt = ""
	license.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	// Re-add to domain/client maps if needed
	if oldStatus == "suspended" {
		api.licensesByDomain[license.DeploymentDomain] = license
		api.licensesByClient[license.ClientID] = license
	}

	api.JSON(w, http.StatusOK, map[string]interface{}{
		"license": license,
		"message": "License reactivated. Product resumed.",
	})
}

// GetLicenseStatus - Get license status (for white label product to check)
func (api *TigerSwapAPI) GetLicenseStatus(w http.ResponseWriter, r *http.Request) {
	licenseKey := r.URL.Query().Get("license_key")

	license, ok := api.licenses[licenseKey]
	if !ok {
		api.JSON(w, http.StatusNotFound, map[string]string{
			"error": "Please input authorized API keys. Contact TigerSwap admin.",
		})
		return
	}

	if license.Status != "active" {
		api.JSON(w, http.StatusForbidden, map[string]string{
			"error": fmt.Sprintf("License %s. Contact TigerSwap admin.", license.Status),
		})
		return
	}

	api.JSON(w, http.StatusOK, map[string]interface{}{
		"valid":      true,
		"domain":    license.DeploymentDomain,
		"expires":   license.ExpiresAt,
	})
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

func (api *TigerSwapAPI) getClientFeatures(clientID string) *WhiteLabelFeature {
	if features, ok := api.featuresByClient[clientID]; ok {
		return features
	}

	// Default features
	return &WhiteLabelFeature{
		ClientID:            clientID,
		EnableSwap:          true,
		EnableTrading:       true,
		EnableLimitOrders:   true,
		EnableStopLoss:      true,
		EnableOrderBook:     true,
		EnablePool:         true,
		EnableFarming:      true,
		EnableBridge:       true,
		EnableLending:      true,
		EnablePerpetuals:   true,
		EnableOptions:     true,
		EnableNFT:         true,
		EnableMMBot:       true,
		EnableArbitrageBot: true,
		EnableSniperBot:    true,
		EnableLiquidityBot: true,
		EnableFrontRunBot: true,
		EnableMEVBot:     true,
		EnableSandwichBot:   true,
		EnableFlashLoanBot: true,
		EnableCrossChainBot: true,
		EnablePerpHedgeBot: true,
		EnableCreateWallet: true,
		EnableImportWallet: true,
		EnableHDWallet:    true,
		EnableMasterWallet: true,
		EnableMultisig:    true,
		EnableAutoSign:    true,
		EnableAPIAccess:  true,
		EnableWebhook:    true,
		EnableWebSocket:  true,
		EnableCustomBrand: false,
		EnableCustomTokens: false,
		EnableCustomChains: false,
		CreatedAt:        time.Now().UTC().Format(time.RFC3339),
	}
}

// CheckLicenseMiddleware - Middleware to check license on every request
func (api *TigerSwapAPI) CheckLicenseMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip license check for admin endpoints
		if strings.HasPrefix(r.URL.Path, "/api/admin") {
			next.ServeHTTP(w, r)
			return
		}

		// Skip license check for public endpoints
		if strings.HasPrefix(r.URL.Path, "/api/public") {
			next.ServeHTTP(w, r)
			return
		}

		// Get license key from header
		licenseKey := r.Header.Get("X-License-Key")
		if licenseKey == "" {
			api.JSON(w, http.StatusUnauthorized, map[string]string{
				"error": "Please input authorized API keys. Contact TigerSwap admin.",
			})
			return
		}

		// Validate license
		license, ok := api.licenses[licenseKey]
		if !ok || license.Status != "active" {
			api.JSON(w, http.StatusUnauthorized, map[string]string{
				"error": "Please input authorized API keys. Contact TigerSwap admin.",
			})
			return
		}

		// Add license info to context
		ctx := r.Context()
		ctx = context.WithValue(ctx, "license", license)
		ctx = context.WithValue(ctx, "client_id", license.ClientID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
EOF
echo "License API created"