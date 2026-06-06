// ============================================================================
// WHITE LABEL FEATURE SYNC API
// Sync features and updates from TigerSwap to white label products
// ============================================================================

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// ============================================================================
// DATA MODELS
// ============================================================================

// TigerSwapVersion - TigerSwap version info
type TigerSwapVersion struct {
	ID           string `json:"id"`
	Version     string `json:"version"`
	ReleaseDate string `json:"release_date"`
	IsActive    bool   `json:"is_active"`
	ReleaseNotes string `json:"release_notes"`
	CreatedAt   string `json:"created_at"`
}

// TigerSwapFeature - Feature registry
type TigerSwapFeature struct {
	ID               string `json:"id"`
	FeatureID       string `json:"feature_id"`
	FeatureName    string `json:"feature_name"`
	Category       string `json:"category"`
	Description   string `json:"description"`
	IsEnabled     bool   `json:"is_enabled"`
	VersionAdded  string `json:"version_added"`
	VersionDeprecated string `json:"version_deprecated"`
	CreatedAt    string `json:"created_at"`
}

// WhiteLabelUpdate - Update available for white label
type WhiteLabelUpdate struct {
	ID                string          `json:"id"`
	ClientID         string          `json:"client_id"`
	UpdateID        string          `json:"update_id"`
	UpdateVersion   string          `json:"update_version"`
	UpdateType      string          `json:"update_type"`
	Title           string          `json:"title"`
	Description    string          `json:"description"`
	Status          string          `json:"status"`
	AvailableAt    string          `json:"available_at"`
	DownloadedAt   string          `json:"downloaded_at"`
	AppliedAt      string          `json:"applied_at"`
	FailedReason   string          `json:"failed_reason"`
	FeaturesAdded  json.RawMessage `json:"features_added"`
	FeaturesUpdated json.RawMessage `json:"features_updated"`
	FeaturesRemoved json.RawMessage `json:"features_removed"`
	UpdateSizeBytes int            `json:"update_size_bytes"`
	Checksum       string          `json:"checksum"`
	CreatedAt     string          `json:"created_at"`
}

// WhiteLabelVersion - Version tracking
type WhiteLabelVersion struct {
	ID                   string          `json:"id"`
	ClientID            string          `json:"client_id"`
	LicenseID           string          `json:"license_id"`
	CurrentVersion      string          `json:"current_version"`
	LatestAvailableVersion string        `json:"latest_available_version"`
	LastCheckedAt      string          `json:"last_checked_at"`
	UpdateAvailable    bool            `json:"update_available"`
	EnabledFeatures    json.RawMessage `json:"enabled_features"`
	DisabledFeatures   json.RawMessage `json:"disabled_features"`
	CreatedAt         string          `json:"created_at"`
	UpdatedAt         string          `json:"updated_at"`
}

// ============================================================================
// TIGERSWAP MASTER REGISTRY (Admin only - create updates)
// ============================================================================

// RegisterTigerSwapVersion - Register new TigerSwap version
func (api *TigerSwapAPI) RegisterTigerSwapVersion(w http.ResponseWriter, r *http.Request) {
	if !api.isSuperAdmin(r) {
		api.JSON(w, http.StatusForbidden, map[string]string{"error": "Super admin access required"})
		return
	}

	var req struct {
		Version      string `json:"version"`
		ReleaseDate  string `json:"release_date"`
		ReleaseNotes string `json:"release_notes"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.JSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request"})
		return
	}

	// Deactivate previous version
	for i := range api.tigerSwapVersions {
		api.tigerSwapVersions[i].IsActive = false
	}

	// Create new version
	version := TigerSwapVersion{
		ID:            generateUUID(),
		Version:       req.Version,
		ReleaseDate:  req.ReleaseDate,
		IsActive:      true,
		ReleaseNotes: req.ReleaseNotes,
		CreatedAt:    time.Now().UTC().Format(time.RFC3339),
	}

	api.tigerSwapVersions = append(api.tigerSwapVersions, &version)

	api.JSON(w, http.StatusCreated, map[string]interface{}{
		"version": version,
		"message": "TigerSwap version registered",
	})
}

// RegisterTigerSwapFeature - Register new feature
func (api *TigerSwapAPI) RegisterTigerSwapFeature(w http.ResponseWriter, r *http.Request) {
	if !api.isSuperAdmin(r) {
		api.JSON(w, http.StatusForbidden, map[string]string{"error": "Super admin access required"})
		return
	}

	var req struct {
		FeatureID      string `json:"feature_id"`
		FeatureName   string `json:"feature_name"`
		Category      string `json:"category"`
		Description   string `json:"description"`
		VersionAdded  string `json:"version_added"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.JSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request"})
		return
	}

	feature := TigerSwapFeature{
		ID:             generateUUID(),
		FeatureID:      req.FeatureID,
		FeatureName:    req.FeatureName,
		Category:     req.Category,
		Description:  req.Description,
		IsEnabled:    true,
		VersionAdded: req.VersionAdded,
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
	}

	api.tigerSwapFeatures[req.FeatureID] = &feature

	api.JSON(w, http.StatusCreated, map[string]interface{}{
		"feature": feature,
		"message": "Feature registered",
	})
}

// CreateWhiteLabelUpdate - Create update for all white label clients
func (api *TigerSwapAPI) CreateWhiteLabelUpdate(w http.ResponseWriter, r *http.Request) {
	if !api.isSuperAdmin(r) {
		api.JSON(w, http.StatusForbidden, map[string]string{"error": "Super admin access required"})
		return
	}

	var req struct {
		UpdateVersion    string          `json:"update_version"`
		UpdateType       string          `json:"update_type"`
		Title            string          `json:"title"`
		Description      string          `json:"description"`
		FeaturesAdded   []string        `json:"features_added"`
		FeaturesUpdated []string        `json:"features_updated"`
		FeaturesRemoved []string        `json:"features_removed"`
		UpdateSizeBytes int             `json:"update_size_bytes"`
		Checksum        string          `json:"checksum"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.JSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request"})
		return
	}

	updateID := fmt.Sprintf("UPDATE-%s-%d", req.UpdateVersion, time.Now().Unix())

	// Get all active white label clients
	for clientID := range api.whiteLabelClients {
		client := api.whiteLabelClients[clientID]
		if client.Status != "approved" {
			continue
		}

		featuresAdded, _ := json.Marshal(req.FeaturesAdded)
		featuresUpdated, _ := json.Marshal(req.FeaturesUpdated)
		featuresRemoved, _ := json.Marshal(req.FeaturesRemoved)

		update := WhiteLabelUpdate{
			ID:                generateUUID(),
			ClientID:          clientID,
			UpdateID:          updateID,
			UpdateVersion:     req.UpdateVersion,
			UpdateType:        req.UpdateType,
			Title:             req.Title,
			Description:      req.Description,
			Status:            "available",
			AvailableAt:      time.Now().UTC().Format(time.RFC3339),
			FeaturesAdded:    featuresAdded,
			FeaturesUpdated: featuresUpdated,
			FeaturesRemoved: featuresRemoved,
			UpdateSizeBytes:  req.UpdateSizeBytes,
			Checksum:       req.Checksum,
			CreatedAt:       time.Now().UTC().Format(time.RFC3339),
		}

		api.whiteLabelUpdates[clientID] = append(api.whiteLabelUpdates[clientID], &update)

		// Update client's latest version
		if v, ok := api.whiteLabelVersions[clientID]; ok {
			v.LatestAvailableVersion = req.UpdateVersion
			v.UpdateAvailable = true
			v.LastCheckedAt = time.Now().UTC().Format(time.RFC3339)
		}
	}

	api.JSON(w, http.StatusCreated, map[string]interface{}{
		"update_id":     updateID,
		"update_count": len(api.whiteLabelClients),
		"message":      "Update pushed to all white label clients",
	})
}

// ============================================================================
// WHITE LABEL CLIENT API (For white label products)
// ============================================================================

// GetLatestVersion - Get latest TigerSwap version
func (api *TigerSwapAPI) GetLatestVersion(w http.ResponseWriter, r *http.Request) {
	// Get latest version
	var latestVersion string
	for _, v := range api.tigerSwapVersions {
		if v.IsActive {
			latestVersion = v.Version
			break
		}
	}

	if latestVersion == "" {
		latestVersion = "1.0.0"
	}

	api.JSON(w, http.StatusOK, map[string]interface{}{
		"version": latestVersion,
		"released": time.Now().UTC().Format(time.RFC3339),
	})
}

// GetAvailableFeatures - Get all available features
func (api *TigerSwapAPI) GetAvailableFeatures(w http.ResponseWriter, r *http.Request) {
	category := r.URL.Query().Get("category")

	var features []TigerSwapFeature
	for _, f := range api.tigerSwapFeatures {
		if category == "" || f.Category == category {
			if f.IsEnabled {
				features = append(features, *f)
			}
		}
	}

	api.JSON(w, http.StatusOK, map[string]interface{}{
		"features": features,
		"count":    len(features),
	})
}

// CheckForUpdates - Check for available updates
func (api *TigerSwapAPI) CheckForUpdates(w http.ResponseWriter, r *http.Request) {
	clientID := api.getClientID(r)
	if clientID == "" {
		api.JSON(w, http.StatusUnauthorized, map[string]string{"error": "Client ID required"})
		return
	}

	version, ok := api.whiteLabelVersions[clientID]
	if !ok {
		version = &WhiteLabelVersion{
			ID:                  generateUUID(),
			ClientID:            clientID,
			CurrentVersion:     "1.0.0",
			LastCheckedAt:      time.Now().UTC().Format(time.RFC3339),
			UpdateAvailable:     false,
		}
		api.whiteLabelVersions[clientID] = version
	}

	// Get available updates
	updates := api.whiteLabelUpdates[clientID]
	var availableUpdates []WhiteLabelUpdate
	for _, u := range updates {
		if u.Status == "available" && u.UpdateVersion > version.CurrentVersion {
			availableUpdates = append(availableUpdates, *u)
		}
	}

	// Get latest version
	var latestVersion string
	for _, v := range api.tigerSwapVersions {
		if v.IsActive {
			latestVersion = v.Version
			break
		}
	}
	if latestVersion == "" {
		latestVersion = "1.0.0"
	}

	api.JSON(w, http.StatusOK, map[string]interface{}{
		"current_version":    version.CurrentVersion,
		"latest_version":     latestVersion,
		"update_available":   latestVersion > version.CurrentVersion,
		"updates":           availableUpdates,
		"update_count":       len(availableUpdates),
		"last_checked":       time.Now().UTC().Format(time.RFC3339),
	})
}

// ApplyUpdate - Apply update to white label product
func (api *TigerSwapAPI) ApplyUpdate(w http.ResponseWriter, r *http.Request) {
	clientID := api.getClientID(r)
	if clientID == "" {
		api.JSON(w, http.StatusUnauthorized, map[string]string{"error": "Client ID required"})
		return
	}

	var req struct {
		UpdateID string `json:"update_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.JSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request"})
		return
	}

	// Find update
	var targetUpdate *WhiteLabelUpdate
	for _, u := range api.whiteLabelUpdates[clientID] {
		if u.UpdateID == req.UpdateID {
			targetUpdate = u
			break
		}
	}

	if targetUpdate == nil {
		api.JSON(w, http.StatusNotFound, map[string]string{"error": "Update not found"})
		return
	}

	// Apply update
	targetUpdate.Status = "applied"
	targetUpdate.AppliedAt = time.Now().UTC().Format(time.RFC3339)

	// Update version
	version := api.whiteLabelVersions[clientID]
	version.CurrentVersion = targetUpdate.UpdateVersion
	version.LastCheckedAt = time.Now().UTC().Format(time.RFC3339)
	version.UpdateAvailable = false

	api.JSON(w, http.StatusOK, map[string]interface{}{
		"update":    targetUpdate,
		"message": "Update applied successfully",
		"version":  version.CurrentVersion,
	})
}

// GetFeatureStatus - Get enabled/disabled features
func (api *TigerSwapAPI) GetFeatureStatus(w http.ResponseWriter, r *http.Request) {
	clientID := api.getClientID(r)
	if clientID == "" {
		api.JSON(w, http.StatusUnauthorized, map[string]string{"error": "Client ID required"})
		return
	}

	// Get client's features
	features := api.getClientFeatures(clientID)

	// Get all available features from TigerSwap
	var availableFeatures []TigerSwapFeature
	for _, f := range api.tigerSwapFeatures {
		if f.IsEnabled {
			availableFeatures = append(availableFeatures, *f)
		}
	}

	api.JSON(w, http.StatusOK, map[string]interface{}{
		"available_features": availableFeatures,
		"client_features":    features,
	})
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

func (api *TigerSwapAPI) getClientID(r *http.Request) string {
	// Get client ID from context (set by license middleware)
	return r.Context().Value("client_id").(string)
}

// InitializeDefaultFeatures - Initialize default TigerSwap features
func (api *TigerSwapAPI) InitializeDefaultFeatures() {
	defaultFeatures := []TigerSwapFeature{
		// Core features
		{FeatureID: "swap", FeatureName: "Swap", Category: "core", Description: "Token swapping", VersionAdded: "1.0.0"},
		{FeatureID: "trading", FeatureName: "Trading", Category: "core", Description: "Limit/stop trading", VersionAdded: "1.0.0"},
		{FeatureID: "pool", FeatureName: "Liquidity Pools", Category: "core", Description: "Create/join pools", VersionAdded: "1.0.0"},
		{FeatureID: "farming", FeatureName: "Farming", Category: "core", Description: "Yield farming", VersionAdded: "1.0.0"},
		{FeatureID: "bridge", FeatureName: "Bridge", Category: "core", Description: "Cross-chain bridging", VersionAdded: "1.0.0"},
		{FeatureID: "lending", FeatureName: "Lending", Category: "core", Description: "Lend/borrow", VersionAdded: "1.0.0"},
		
		// Bot features
		{FeatureID: "mm_bot", FeatureName: "Market Maker Bot", Category: "bot", Description: "Market making", VersionAdded: "1.0.0"},
		{FeatureID: "arbitrage_bot", FeatureName: "Arbitrage Bot", Category: "bot", Description: "Arbitrage trading", VersionAdded: "1.0.0"},
		{FeatureID: "sniper_bot", FeatureName: "Sniper Bot", Category: "bot", Description: "Token sniping", VersionAdded: "1.0.0"},
		
		// Wallet features
		{FeatureID: "create_wallet", FeatureName: "Create Wallet", Category: "wallet", Description: "Create new wallet", VersionAdded: "1.0.0"},
		{FeatureID: "import_wallet", FeatureName: "Import Wallet", Category: "wallet", Description: "Import existing wallet", VersionAdded: "1.0.0"},
		{FeatureID: "master_wallet", FeatureName: "Master Wallet", Category: "wallet", Description: "Master wallet management", VersionAdded: "1.0.0"},
		
		// API features
		{FeatureID: "api_access", FeatureName: "API Access", Category: "api", Description: "REST API access", VersionAdded: "1.0.0"},
		{FeatureID: "webhook", FeatureName: "Webhooks", Category: "api", Description: "Webhook notifications", VersionAdded: "1.0.0"},
		{FeatureID: "websocket", FeatureName: "WebSocket", Category: "api", Description: "Real-time updates", VersionAdded: "1.0.0"},
	}

	for i := range defaultFeatures {
		f := defaultFeatures[i]
		f.ID = generateUUID()
		f.IsEnabled = true
		f.CreatedAt = time.Now().UTC().Format(time.RFC3339)
		api.tigerSwapFeatures[f.FeatureID] = &f
	}

	// Set initial version
	api.tigerSwapVersions = append(api.tigerSwapVersions, &TigerSwapVersion{
		ID:           generateUUID(),
		Version:      "1.0.0",
		ReleaseDate: time.Now().Format("2006-01-02"),
		IsActive:    true,
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
	})
}
EOF
echo "Sync API created"