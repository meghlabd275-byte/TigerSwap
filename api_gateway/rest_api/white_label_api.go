// ============================================================================
// WHITE LABEL MANAGEMENT - Complete White Label Product System
// ============================================================================

// WhiteLabelClient - White label client management
type WhiteLabelClient struct {
	ID                       string `json:"id"`
	ClientID                  string `json:"client_id"`
	ClientName                string `json:"client_name"`
	BrandName                 string `json:"brand_name"`
	BrandLogoURL              string `json:"brand_logo_url"`
	BrandColorPrimary        string `json:"brand_color_primary"`
	BrandColorSecondary      string `json:"brand_color_secondary"`
	WebsiteURL               string `json:"website_url"`
	ContactEmail            string `json:"contact_email"`
	Tier                     string `json:"tier"`
	Status                   string `json:"status"`
	ApprovedBy               string `json:"approved_by"`
	ApprovedAt               string `json:"approved_at"`
	SuspendedBy             string `json:"suspended_by"`
	SuspendedAt             string `json:"suspended_at"`
	HaltReason              string `json:"halt_reason"`
	SwapFeeShareBPS          int    `json:"swap_fee_share_bps"`
	TradingFeeShareBPS       int    `json:"trading_fee_share_bps"`
	BotSubscriptionFeeShareBPS int    `json:"bot_subscription_fee_share_bps"`
	ListingFeeShareBPS       int    `json:"listing_fee_share_bps"`
	WithdrawalFeeShareBPS    int    `json:"withdrawal_fee_share_bps"`
	DepositFeeShareBPS       int    `json:"deposit_fee_share_bps"`
	TransferFeeShareBPS      int    `json:"transfer_fee_share_bps"`
	APIKeyFeeShareBPS       int    `json:"api_key_fee_share_bps"`
	AdminFeeAddress         string `json:"admin_fee_address"`
	ClientRevenueAddress    string `json:"client_revenue_address"`
	CanUseSwap            bool   `json:"can_use_swap"`
	CanUseTrading          bool   `json:"can_use_trading"`
	CanUseBots            bool   `json:"can_use_bots"`
	CanUseListings        bool   `json:"can_use_listings"`
	CanUseBridge          bool   `json:"can_use_bridge"`
	CanUseFarming        bool   `json:"can_use_farming"`
	CanUseLending        bool   `json:"can_use_lending"`
	CanUsePerpetuals     bool   `json:"can_use_perpetuals"`
	CanUseOptions        bool   `json:"can_use_options"`
	CanUseNFT            bool   `json:"can_use_nft"`
	CanCreateAPIKeys      bool   `json:"can_create_api_keys"`
	CanWhitelistTokens   bool   `json:"can_whitelist_tokens"`
	CanCustomBridge      bool   `json:"can_custom_bridge"`
	CanCustomDEX         bool   `json:"can_custom_dex"`
	MaxDailyVolume       float64 `json:"max_daily_volume"`
	MaxDailyUsers       int     `json:"max_daily_users"`
	MaxAPICallsPerDay   int     `json:"max_api_calls_per_day"`
	TotalVolumeUSD      float64 `json:"total_volume_usd"`
	TotalFeesPaid      float64 `json:"total_fees_paid"`
	TotalUsers        int     `json:"total_users"`
	IsActive            bool   `json:"is_active"`
	CreatedAt          string `json:"created_at"`
	UpdatedAt          string `json:"updated_at"`
}

// CreateWhiteLabelClient - Create new white label client (Super Admin only)
func (api *TigerSwapAPI) CreateWhiteLabelClient(w http.ResponseWriter, r *http.Request) {
	if !api.isSuperAdmin(r) {
		api.JSON(w, http.StatusForbidden, map[string]string{"error": "Super admin access required"})
		return
	}

	var req struct {
		ClientName            string `json:"client_name"`
		BrandName           string `json:"brand_name"`
		BrandLogoURL        string `json:"brand_logo_url"`
		BrandColorPrimary  string `json:"brand_color_primary"`
		WebsiteURL        string `json:"website_url"`
		ContactEmail      string `json:"contact_email"`
		Tier             string `json:"tier"`
		AdminFeeAddress   string `json:"admin_fee_address"`
		ClientRevenueAddress string `json:"client_revenue_address"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.JSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request"})
		return
	}

	clientID := generateClientID(req.ClientName)

	client := WhiteLabelClient{
		ID:                         generateUUID(),
		ClientID:                    clientID,
		ClientName:                  req.ClientName,
		BrandName:                  req.BrandName,
		BrandLogoURL:               req.BrandLogoURL,
		BrandColorPrimary:         req.BrandColorPrimary,
		WebsiteURL:               req.WebsiteURL,
		ContactEmail:             req.ContactEmail,
		Tier:                    req.Tier,
		Status:                  "pending",
		SwapFeeShareBPS:         2000,
		TradingFeeShareBPS:        2000,
		BotSubscriptionFeeShareBPS: 2000,
		ListingFeeShareBPS:         2000,
		WithdrawalFeeShareBPS:     2000,
		DepositFeeShareBPS:        2000,
		TransferFeeShareBPS:      2000,
		APIKeyFeeShareBPS:         2000,
		AdminFeeAddress:           req.AdminFeeAddress,
		ClientRevenueAddress:       req.ClientRevenueAddress,
		CanUseSwap:               true,
		CanUseTrading:             true,
		CanUseBots:               true,
		CanUseListings:           true,
		CanUseBridge:            true,
		CanUseFarming:           true,
		CanUseLending:           true,
		CanUsePerpetuals:        true,
		CanUseOptions:         true,
		CanUseNFT:             true,
		CanCreateAPIKeys:       true,
		IsActive:               true,
		CreatedAt:              time.Now().UTC().Format(time.RFC3339),
		UpdatedAt:              time.Now().UTC().Format(time.RFC3339),
	}

	api.whiteLabelClients[clientID] = &client

	api.JSON(w, http.StatusCreated, map[string]interface{}{
		"client": client, "message": "White label client created. Pending approval.",
	})
}

// ApproveWhiteLabelClient - Approve white label client (Super Admin only)
func (api *TigerSwapAPI) ApproveWhiteLabelClient(w http.ResponseWriter, r *http.Request) {
	if !api.isSuperAdmin(r) {
		api.JSON(w, http.StatusForbidden, map[string]string{"error": "Super admin access required"})
		return
	}

	var req struct {
		ClientID        string `json:"client_id"`
		AdminFeeAddress string `json:"admin_fee_address"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.JSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request"})
		return
	}

	client, ok := api.whiteLabelClients[req.ClientID]
	if !ok {
		api.JSON(w, http.StatusNotFound, map[string]string{"error": "Client not found"})
		return
	}

	client.Status = "approved"
	client.ApprovedBy = api.getAdminID(r)
	client.ApprovedAt = time.Now().UTC().Format(time.RFC3339)
	client.AdminFeeAddress = req.AdminFeeAddress
	client.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	api.JSON(w, http.StatusOK, map[string]interface{}{
		"client": client, "message": "White label client approved",
	})
}

// SuspendWhiteLabelClient - Suspend/halt white label client
func (api *TigerSwapAPI) SuspendWhiteLabelClient(w http.ResponseWriter, r *http.Request) {
	if !api.isSuperAdmin(r) {
		api.JSON(w, http.StatusForbidden, map[string]string{"error": "Super admin access required"})
		return
	}

	var req struct {
		ClientID string `json:"client_id"`
		Reason  string `json:"reason"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.JSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request"})
		return
	}

	client, ok := api.whiteLabelClients[req.ClientID]
	if !ok {
		api.JSON(w, http.StatusNotFound, map[string]string{"error": "Client not found"})
		return
	}

	client.Status = "suspended"
	client.SuspendedBy = api.getAdminID(r)
	client.SuspendedAt = time.Now().UTC().Format(time.RFC3339)
	client.HaltReason = req.Reason
	client.CanUseSwap = false
	client.CanUseTrading = false
	client.CanUseBots = false
	client.CanUseListings = false
	client.CanUseBridge = false
	client.CanUseFarming = false
	client.CanUseLending = false
	client.CanUsePerpetuals = false
	client.CanUseOptions = false
	client.CanUseNFT = false
	client.CanCreateAPIKeys = false
	client.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	api.JSON(w, http.StatusOK, map[string]interface{}{
		"client": client, "message": "White label client suspended/halted",
	})
}

// UpdateWhiteLabelFees - Update fee sharing percentages (Super Admin only)
func (api *TigerSwapAPI) UpdateWhiteLabelFees(w http.ResponseWriter, r *http.Request) {
	if !api.isSuperAdmin(r) {
		api.JSON(w, http.StatusForbidden, map[string]string{"error": "Super admin access required"})
		return
	}

	var req struct {
		ClientID                    string `json:"client_id"`
		SwapFeeShareBPS             int    `json:"swap_fee_share_bps"`
		TradingFeeShareBPS         int    `json:"trading_fee_share_bps"`
		BotSubscriptionFeeShareBPS int    `json:"bot_subscription_fee_share_bps"`
		ListingFeeShareBPS         int    `json:"listing_fee_share_bps"`
		WithdrawalFeeShareBPS       int    `json:"withdrawal_fee_share_bps"`
		DepositFeeShareBPS         int    `json:"deposit_fee_share_bps"`
		TransferFeeShareBPS        int    `json:"transfer_fee_share_bps"`
		APIKeyFeeShareBPS          int    `json:"api_key_fee_share_bps"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.JSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request"})
		return
	}

	maxFeeBPS := 2000
	if req.SwapFeeShareBPS < 0 || req.SwapFeeShareBPS > maxFeeBPS {
		api.JSON(w, http.StatusBadRequest, map[string]string{"error": "Fee must be 0-20%"})
		return
	}

	client, ok := api.whiteLabelClients[req.ClientID]
	if !ok {
		api.JSON(w, http.StatusNotFound, map[string]string{"error": "Client not found"})
		return
	}

	client.SwapFeeShareBPS = req.SwapFeeShareBPS
	client.TradingFeeShareBPS = req.TradingFeeShareBPS
	client.BotSubscriptionFeeShareBPS = req.BotSubscriptionFeeShareBPS
	client.ListingFeeShareBPS = req.ListingFeeShareBPS
	client.WithdrawalFeeShareBPS = req.WithdrawalFeeShareBPS
	client.DepositFeeShareBPS = req.DepositFeeShareBPS
	client.TransferFeeShareBPS = req.TransferFeeShareBPS
	client.APIKeyFeeShareBPS = req.APIKeyFeeShareBPS
	client.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	api.JSON(w, http.StatusOK, map[string]interface{}{
		"client": client, "message": "Fee sharing updated",
	})
}

// ListWhiteLabelClients - List all white label clients
func (api *TigerSwapAPI) ListWhiteLabelClients(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	var clients []WhiteLabelClient
	for _, client := range api.whiteLabelClients {
		if status == "" || client.Status == status {
			clients = append(clients, *client)
		}
	}
	api.JSON(w, http.StatusOK, map[string]interface{}{"clients": clients, "count": len(clients)})
}

// ToggleWhiteLabelFeature - Enable/disable specific feature
func (api *TigerSwapAPI) ToggleWhiteLabelFeature(w http.ResponseWriter, r *http.Request) {
	if !api.isSuperAdmin(r) {
		api.JSON(w, http.StatusForbidden, map[string]string{"error": "Super admin access required"})
		return
	}

	var req struct {
		ClientID string `json:"client_id"`
		Feature string `json:"feature"`
		Enabled bool   `json:"enabled"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.JSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request"})
		return
	}

	client, ok := api.whiteLabelClients[req.ClientID]
	if !ok {
		api.JSON(w, http.StatusNotFound, map[string]string{"error": "Client not found"})
		return
	}

	switch req.Feature {
	case "swap":
		client.CanUseSwap = req.Enabled
	case "trading":
		client.CanUseTrading = req.Enabled
	case "bots":
		client.CanUseBots = req.Enabled
	case "listings":
		client.CanUseListings = req.Enabled
	case "bridge":
		client.CanUseBridge = req.Enabled
	case "farming":
		client.CanUseFarming = req.Enabled
	case "lending":
		client.CanUseLending = req.Enabled
	case "perpetuals":
		client.CanUsePerpetuals = req.Enabled
	case "options":
		client.CanUseOptions = req.Enabled
	case "nft":
		client.CanUseNFT = req.Enabled
	case "api_keys":
		client.CanCreateAPIKeys = req.Enabled
	case "whitelist_tokens":
		client.CanWhitelistTokens = req.Enabled
	case "custom_bridge":
		client.CanCustomBridge = req.Enabled
	case "custom_dex":
		client.CanCustomDEX = req.Enabled
	default:
		api.JSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid feature"})
		return
	}

	client.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	api.JSON(w, http.StatusOK, map[string]interface{}{
		"client": client, "message": "Feature toggled",
	})
}

// CalculateWhiteLabelFee - Calculate fee split between TigerSwap and client
func CalculateWhiteLabelFee(amount float64, feeShareBPS int) (tigerShare float64, clientShare float64) {
	tigerShare = amount * float64(feeShareBPS) / 10000
	clientShare = amount - tigerShare
	return tigerShare, clientShare
}

// generateClientID generates unique client ID
func generateClientID(name string) string {
	timestamp := time.Now().Unix()
	sanitized := strings.ToLower(strings.ReplaceAll(name, " ", "-"))
	return fmt.Sprintf("wl-%s-%d", sanitized, timestamp)
}