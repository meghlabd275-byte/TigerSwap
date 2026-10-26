package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"tigerswap/backend/models"
)

// ListingService manages futures, perpetual and margin listings. Every write
// path validates the risk parameters before persisting so an inconsistent
// listing can never reach the matching engine.
type ListingService struct {
	db *gorm.DB
}

func NewListingService(db *gorm.DB) *ListingService {
	return &ListingService{db: db}
}

// allowedSettlementAssets are the assets a contract may settle in. RUSD is the
// native settlement asset; USDT and USDC settle on the same self-hosted engine.
var allowedSettlementAssets = map[string]bool{
	"RUSD": true, "USDT": true, "USDC": true, "RYUSD": true,
	"BTC": true, "ETH": true, "TGR": true, "USD": true,
}

var allowedContractTypes = map[string]bool{
	models.ContractTypeRUSDM:  true,
	models.ContractTypeUSDTM:  true,
	models.ContractTypeUSDCM:  true,
	models.ContractTypeRYUSDM: true,
	models.ContractTypeCOINM:  true,
	models.ContractTypeTGRM:   true,
}

var allowedListingTypes = map[string]bool{
	models.ListingTypePerpetual: true,
	models.ListingTypeQuarterly: true,
	models.ListingTypeBiQuarter: true,
	models.ListingTypeDelivery:  true,
}

var allowedListingStatuses = map[string]bool{
	models.ListingStatusActive:      true,
	models.ListingStatusPaused:      true,
	models.ListingStatusDelisting:   true,
	models.ListingStatusMaintenance: true,
}

var allowedRiskTiers = map[string]bool{"low": true, "medium": true, "high": true}

var allowedMarketTypes = map[string]bool{"oracle": true, "orderbook": true}

var allowedMarginModes = map[string]bool{"cross": true, "isolated": true}

// parseSymbol splits "BTC/RUSD" into base and quote. Both parts must be
// non-empty and contain only letters and digits.
func parseSymbol(symbol string) (string, string, error) {
	symbol = strings.TrimSpace(strings.ToUpper(symbol))
	parts := strings.Split(symbol, "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("symbol must have the form BASE/QUOTE")
	}
	base, quote := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
	for _, part := range []string{base, quote} {
		if part == "" {
			return "", "", fmt.Errorf("symbol must have the form BASE/QUOTE")
		}
		for _, r := range part {
			if !(r >= 'A' && r <= 'Z') && !(r >= '0' && r <= '9') {
				return "", "", fmt.Errorf("symbol parts may only contain letters and digits")
			}
		}
	}
	return base, quote, nil
}

// rateInRange reports whether a fractional rate sits inside [min,max].
func rateInRange(value, min, max float64) bool {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return false
	}
	return value >= min && value <= max
}

// isPositiveDecimal reports whether a string is a decimal number greater than
// zero. Amounts are kept as strings to avoid float precision loss on-chain.
func isPositiveDecimal(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	var seenDigit bool
	var seenDot bool
	for i, r := range value {
		switch {
		case r >= '0' && r <= '9':
			seenDigit = true
		case r == '.':
			if seenDot || i == 0 {
				return false
			}
			seenDot = true
		default:
			return false
		}
	}
	if !seenDigit {
		return false
	}
	var parsed float64
	if _, err := fmt.Sscanf(value, "%f", &parsed); err != nil {
		return false
	}
	return parsed > 0
}

// splitAssets parses a comma-separated asset list into an uppercase slice.
func splitAssets(raw string) ([]string, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		asset := strings.ToUpper(strings.TrimSpace(part))
		if asset == "" {
			return nil, fmt.Errorf("asset list contains an empty entry")
		}
		out = append(out, asset)
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Futures market listings
// ---------------------------------------------------------------------------

// CreateFuturesMarket validates and persists a new futures listing. New markets
// are created paused so an operator reviews them before they take orders.
func (s *ListingService) CreateFuturesMarket(c *gin.Context) {
	var req struct {
		Symbol                string  `json:"symbol" binding:"required"`
		ContractType          string  `json:"contract_type" binding:"required"`
		Settlement            string  `json:"settlement" binding:"required"`
		ContractSize          string  `json:"contract_size"`
		TickSize              string  `json:"tick_size"`
		MaxLeverage           int     `json:"max_leverage"`
		MinOrderSize          string  `json:"min_order_size"`
		FundingRate           string  `json:"funding_rate"`
		FundingInterval       int     `json:"funding_interval_hours"`
		FundingRateCap        string  `json:"funding_rate_cap"`
		OracleSources         string  `json:"oracle_sources"`
		MaintenanceMarginRate float64 `json:"maintenance_margin_rate"`
		InitialMarginRate     float64 `json:"initial_margin_rate"`
		LiquidationFeeRate    float64 `json:"liquidation_fee_rate"`
		InsuranceFundShare    float64 `json:"insurance_fund_share"`
		MakerFeeRate          float64 `json:"maker_fee_rate"`
		TakerFeeRate          float64 `json:"taker_fee_rate"`
		ListingType           string  `json:"listing_type"`
		IsTradFi              bool    `json:"is_tradfi"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	base, quote, err := parseSymbol(req.Symbol)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	settlement := strings.ToUpper(strings.TrimSpace(req.Settlement))
	if !allowedSettlementAssets[settlement] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported settlement asset"})
		return
	}
	contractType := strings.ToUpper(strings.TrimSpace(req.ContractType))
	if !allowedContractTypes[contractType] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported contract type"})
		return
	}
	if req.MaxLeverage < 1 || req.MaxLeverage > 1001 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "max_leverage must be between 1 and 1001"})
		return
	}
	if !rateInRange(req.MaintenanceMarginRate, 0.001, 0.5) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "maintenance_margin_rate must be between 0.001 and 0.5"})
		return
	}
	if !rateInRange(req.InitialMarginRate, 0.001, 1) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "initial_margin_rate must be between 0.001 and 1"})
		return
	}
	if req.InitialMarginRate < req.MaintenanceMarginRate {
		c.JSON(http.StatusBadRequest, gin.H{"error": "initial_margin_rate must be at least the maintenance margin rate"})
		return
	}
	if !rateInRange(req.LiquidationFeeRate, 0, 0.2) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "liquidation_fee_rate must be between 0 and 0.2"})
		return
	}
	if !rateInRange(req.InsuranceFundShare, 0, 1) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "insurance_fund_share must be between 0 and 1"})
		return
	}
	if !rateInRange(req.MakerFeeRate, 0, 0.05) || !rateInRange(req.TakerFeeRate, 0, 0.05) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "fee rates must be between 0 and 0.05"})
		return
	}
	if req.FundingInterval <= 0 || req.FundingInterval > 24 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "funding_interval_hours must be between 1 and 24"})
		return
	}
	listingType := strings.ToLower(strings.TrimSpace(req.ListingType))
	if listingType == "" {
		listingType = models.ListingTypePerpetual
	}
	if !allowedListingTypes[listingType] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported listing type"})
		return
	}
	if req.OracleSources != "" {
		if _, err := splitAssets(req.OracleSources); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "oracle_sources must be a comma-separated list"})
			return
		}
	}

	var existing int64
	s.db.Model(&models.FuturesMarket{}).Where("symbol = ?", strings.ToUpper(req.Symbol)).Count(&existing)
	if existing > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "a futures market with this symbol already exists"})
		return
	}

	market := models.FuturesMarket{
		Symbol:                strings.ToUpper(strings.TrimSpace(req.Symbol)),
		ContractType:          contractType,
		BaseAsset:             base,
		QuoteAsset:            quote,
		Settlement:            settlement,
		ContractSize:          defaultDecimal(req.ContractSize, "1"),
		TickSize:              defaultDecimal(req.TickSize, "0.01"),
		MaxLeverage:           req.MaxLeverage,
		MinOrderSize:          defaultDecimal(req.MinOrderSize, "1"),
		FundingRate:           defaultDecimal(req.FundingRate, "0.0001"),
		FundingInterval:       req.FundingInterval,
		FundingRateCap:        defaultDecimal(req.FundingRateCap, "0.01"),
		OracleSources:         req.OracleSources,
		MaintenanceMarginRate: req.MaintenanceMarginRate,
		InitialMarginRate:     req.InitialMarginRate,
		LiquidationFeeRate:    req.LiquidationFeeRate,
		InsuranceFundShare:    req.InsuranceFundShare,
		MakerFeeRate:          req.MakerFeeRate,
		TakerFeeRate:          req.TakerFeeRate,
		Status:                models.ListingStatusPaused,
		ListingType:           listingType,
		IsTradFi:              req.IsTradFi,
	}
	if err := s.db.Create(&market).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create futures market"})
		return
	}

	logListingAction(c, s.db, "create_futures_market", "futures_market", market.ID.String(), gin.H{"symbol": market.Symbol})
	c.JSON(http.StatusCreated, market)
}

// defaultDecimal returns value when it parses as a positive decimal, else fallback.
func defaultDecimal(value, fallback string) string {
	if isPositiveDecimal(value) {
		return strings.TrimSpace(value)
	}
	return fallback
}

// UpdateFuturesMarket applies admin-editable risk parameters to a listing.
func (s *ListingService) UpdateFuturesMarket(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid market id"})
		return
	}

	var req struct {
		MaxLeverage           *int     `json:"max_leverage"`
		FundingRate           *string  `json:"funding_rate"`
		FundingInterval       *int     `json:"funding_interval_hours"`
		FundingRateCap        *string  `json:"funding_rate_cap"`
		OracleSources         *string  `json:"oracle_sources"`
		MaintenanceMarginRate *float64 `json:"maintenance_margin_rate"`
		InitialMarginRate     *float64 `json:"initial_margin_rate"`
		LiquidationFeeRate    *float64 `json:"liquidation_fee_rate"`
		InsuranceFundShare    *float64 `json:"insurance_fund_share"`
		MakerFeeRate          *float64 `json:"maker_fee_rate"`
		TakerFeeRate          *float64 `json:"taker_fee_rate"`
		Status                *string  `json:"status"`
		ListingType           *string  `json:"listing_type"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var market models.FuturesMarket
	if err := s.db.Where("id = ?", id).First(&market).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "futures market not found"})
		return
	}

	updates := map[string]interface{}{}
	if req.MaxLeverage != nil {
		if *req.MaxLeverage < 1 || *req.MaxLeverage > 1001 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "max_leverage must be between 1 and 1001"})
			return
		}
		updates["max_leverage"] = *req.MaxLeverage
	}
	if req.FundingRate != nil {
		if !isPositiveDecimal(*req.FundingRate) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "funding_rate must be a positive decimal"})
			return
		}
		updates["funding_rate"] = strings.TrimSpace(*req.FundingRate)
	}
	if req.FundingRateCap != nil {
		if !isPositiveDecimal(*req.FundingRateCap) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "funding_rate_cap must be a positive decimal"})
			return
		}
		updates["funding_rate_cap"] = strings.TrimSpace(*req.FundingRateCap)
	}
	if req.FundingInterval != nil {
		if *req.FundingInterval <= 0 || *req.FundingInterval > 24 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "funding_interval_hours must be between 1 and 24"})
			return
		}
		updates["funding_interval"] = *req.FundingInterval
	}
	if req.OracleSources != nil {
		if _, err := splitAssets(*req.OracleSources); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "oracle_sources must be a comma-separated list"})
			return
		}
		updates["oracle_sources"] = *req.OracleSources
	}
	if req.MaintenanceMarginRate != nil {
		if !rateInRange(*req.MaintenanceMarginRate, 0.001, 0.5) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "maintenance_margin_rate must be between 0.001 and 0.5"})
			return
		}
		updates["maintenance_margin_rate"] = *req.MaintenanceMarginRate
	}
	if req.InitialMarginRate != nil {
		if !rateInRange(*req.InitialMarginRate, 0.001, 1) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "initial_margin_rate must be between 0.001 and 1"})
			return
		}
		updates["initial_margin_rate"] = *req.InitialMarginRate
	}
	if req.LiquidationFeeRate != nil {
		if !rateInRange(*req.LiquidationFeeRate, 0, 0.2) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "liquidation_fee_rate must be between 0 and 0.2"})
			return
		}
		updates["liquidation_fee_rate"] = *req.LiquidationFeeRate
	}
	if req.InsuranceFundShare != nil {
		if !rateInRange(*req.InsuranceFundShare, 0, 1) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "insurance_fund_share must be between 0 and 1"})
			return
		}
		updates["insurance_fund_share"] = *req.InsuranceFundShare
	}
	if req.MakerFeeRate != nil {
		if !rateInRange(*req.MakerFeeRate, 0, 0.05) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "maker_fee_rate must be between 0 and 0.05"})
			return
		}
		updates["maker_fee_rate"] = *req.MakerFeeRate
	}
	if req.TakerFeeRate != nil {
		if !rateInRange(*req.TakerFeeRate, 0, 0.05) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "taker_fee_rate must be between 0 and 0.05"})
			return
		}
		updates["taker_fee_rate"] = *req.TakerFeeRate
	}
	if req.Status != nil {
		if !allowedListingStatuses[*req.Status] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported status"})
			return
		}
		updates["status"] = *req.Status
	}
	if req.ListingType != nil {
		if !allowedListingTypes[*req.ListingType] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported listing type"})
			return
		}
		updates["listing_type"] = *req.ListingType
	}

	// Guard the margin relationship when only one side is being changed.
	finalMaintenance := market.MaintenanceMarginRate
	finalInitial := market.InitialMarginRate
	if v, ok := updates["maintenance_margin_rate"].(float64); ok {
		finalMaintenance = v
	}
	if v, ok := updates["initial_margin_rate"].(float64); ok {
		finalInitial = v
	}
	if finalInitial < finalMaintenance {
		c.JSON(http.StatusBadRequest, gin.H{"error": "initial_margin_rate must be at least the maintenance margin rate"})
		return
	}

	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no updatable fields supplied"})
		return
	}

	result := s.db.Model(&models.FuturesMarket{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update futures market"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "futures market not found"})
		return
	}

	if err := s.db.Where("id = ?", id).First(&market).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "futures market not found"})
		return
	}
	logListingAction(c, s.db, "update_futures_market", "futures_market", id.String(), updates)
	c.JSON(http.StatusOK, market)
}

// DeleteFuturesMarket moves a listing to delisting. Historical fills are kept.
func (s *ListingService) DeleteFuturesMarket(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid market id"})
		return
	}

	result := s.db.Model(&models.FuturesMarket{}).Where("id = ?", id).Update("status", models.ListingStatusDelisting)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delist futures market"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "futures market not found"})
		return
	}

	logListingAction(c, s.db, "delist_futures_market", "futures_market", id.String(), nil)
	c.JSON(http.StatusOK, gin.H{"success": true, "id": id, "status": models.ListingStatusDelisting})
}

// GetFuturesMarkets lists futures listings, optionally filtered by status.
func (s *ListingService) GetFuturesMarkets(c *gin.Context) {
	query := s.db.Model(&models.FuturesMarket{})
	if status := c.Query("status"); status != "" {
		if !allowedListingStatuses[status] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported status filter"})
			return
		}
		query = query.Where("status = ?", status)
	}
	if settlement := c.Query("settlement"); settlement != "" {
		query = query.Where("settlement = ?", strings.ToUpper(settlement))
	}

	limit, offset := paginationParams(c, 100)
	var markets []models.FuturesMarket
	if err := query.Order("symbol ASC").Limit(limit).Offset(offset).Find(&markets).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load futures markets"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"markets": markets, "count": len(markets)})
}

// ---------------------------------------------------------------------------
// Perpetual listings
// ---------------------------------------------------------------------------

// CreatePerpsMarket validates and persists a perpetual listing.
func (s *ListingService) CreatePerpsMarket(c *gin.Context) {
	var req struct {
		Symbol                string  `json:"symbol" binding:"required"`
		MarketType            string  `json:"market_type"`
		CollateralAssets      string  `json:"collateral_assets"`
		MaxOpenInterest       string  `json:"max_open_interest"`
		MaxLeverage           int     `json:"max_leverage"`
		PriceImpactRate       float64 `json:"price_impact_rate"`
		BorrowFeePerHour      float64 `json:"borrow_fee_per_hour"`
		MakerFeeRate          float64 `json:"maker_fee_rate"`
		TakerFeeRate          float64 `json:"taker_fee_rate"`
		MaintenanceMarginRate float64 `json:"maintenance_margin_rate"`
		LiquidationFeeRate    float64 `json:"liquidation_fee_rate"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if _, _, err := parseSymbol(req.Symbol); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	marketType := strings.ToLower(strings.TrimSpace(req.MarketType))
	if marketType == "" {
		marketType = "oracle"
	}
	if !allowedMarketTypes[marketType] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "market_type must be oracle or orderbook"})
		return
	}
	if req.MaxLeverage < 1 || req.MaxLeverage > 1001 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "max_leverage must be between 1 and 1001"})
		return
	}
	collateral, err := splitAssets(req.CollateralAssets)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(collateral) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "collateral_assets must list at least one asset"})
		return
	}
	for _, asset := range collateral {
		if !allowedSettlementAssets[asset] {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("unsupported collateral asset %s", asset)})
			return
		}
	}
	if !isPositiveDecimal(req.MaxOpenInterest) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "max_open_interest must be a positive decimal"})
		return
	}
	if !rateInRange(req.PriceImpactRate, 0, 0.5) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "price_impact_rate must be between 0 and 0.5"})
		return
	}
	if !rateInRange(req.BorrowFeePerHour, 0, 0.01) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "borrow_fee_per_hour must be between 0 and 0.01"})
		return
	}
	if !rateInRange(req.MakerFeeRate, 0, 0.05) || !rateInRange(req.TakerFeeRate, 0, 0.05) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "fee rates must be between 0 and 0.05"})
		return
	}
	if !rateInRange(req.MaintenanceMarginRate, 0.001, 0.5) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "maintenance_margin_rate must be between 0.001 and 0.5"})
		return
	}
	if !rateInRange(req.LiquidationFeeRate, 0, 0.2) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "liquidation_fee_rate must be between 0 and 0.2"})
		return
	}

	symbol := strings.ToUpper(strings.TrimSpace(req.Symbol))
	var existing int64
	s.db.Model(&models.PerpsMarket{}).Where("symbol = ?", symbol).Count(&existing)
	if existing > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "a perps market with this symbol already exists"})
		return
	}

	market := models.PerpsMarket{
		Symbol:                symbol,
		MarketType:            marketType,
		CollateralAssets:      strings.Join(collateral, ","),
		MaxOpenInterest:       strings.TrimSpace(req.MaxOpenInterest),
		MaxLeverage:           req.MaxLeverage,
		PriceImpactRate:       req.PriceImpactRate,
		BorrowFeePerHour:      req.BorrowFeePerHour,
		MakerFeeRate:          req.MakerFeeRate,
		TakerFeeRate:          req.TakerFeeRate,
		MaintenanceMarginRate: req.MaintenanceMarginRate,
		LiquidationFeeRate:    req.LiquidationFeeRate,
		Status:                models.ListingStatusPaused,
	}
	if err := s.db.Create(&market).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create perps market"})
		return
	}

	logListingAction(c, s.db, "create_perps_market", "perps_market", market.ID.String(), gin.H{"symbol": market.Symbol})
	c.JSON(http.StatusCreated, market)
}

// UpdatePerpsMarket applies admin-editable parameters to a perps listing.
func (s *ListingService) UpdatePerpsMarket(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid market id"})
		return
	}

	var req struct {
		MarketType            *string  `json:"market_type"`
		CollateralAssets      *string  `json:"collateral_assets"`
		MaxOpenInterest       *string  `json:"max_open_interest"`
		MaxLeverage           *int     `json:"max_leverage"`
		PriceImpactRate       *float64 `json:"price_impact_rate"`
		BorrowFeePerHour      *float64 `json:"borrow_fee_per_hour"`
		MakerFeeRate          *float64 `json:"maker_fee_rate"`
		TakerFeeRate          *float64 `json:"taker_fee_rate"`
		MaintenanceMarginRate *float64 `json:"maintenance_margin_rate"`
		LiquidationFeeRate    *float64 `json:"liquidation_fee_rate"`
		Status                *string  `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var market models.PerpsMarket
	if err := s.db.Where("id = ?", id).First(&market).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "perps market not found"})
		return
	}

	updates := map[string]interface{}{}
	if req.MarketType != nil {
		if !allowedMarketTypes[*req.MarketType] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "market_type must be oracle or orderbook"})
			return
		}
		updates["market_type"] = *req.MarketType
	}
	if req.CollateralAssets != nil {
		assets, err := splitAssets(*req.CollateralAssets)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if len(assets) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "collateral_assets must list at least one asset"})
			return
		}
		for _, asset := range assets {
			if !allowedSettlementAssets[asset] {
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("unsupported collateral asset %s", asset)})
				return
			}
		}
		updates["collateral_assets"] = strings.Join(assets, ",")
	}
	if req.MaxOpenInterest != nil {
		if !isPositiveDecimal(*req.MaxOpenInterest) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "max_open_interest must be a positive decimal"})
			return
		}
		updates["max_open_interest"] = strings.TrimSpace(*req.MaxOpenInterest)
	}
	if req.MaxLeverage != nil {
		if *req.MaxLeverage < 1 || *req.MaxLeverage > 1001 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "max_leverage must be between 1 and 1001"})
			return
		}
		updates["max_leverage"] = *req.MaxLeverage
	}
	if req.PriceImpactRate != nil {
		if !rateInRange(*req.PriceImpactRate, 0, 0.5) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "price_impact_rate must be between 0 and 0.5"})
			return
		}
		updates["price_impact_rate"] = *req.PriceImpactRate
	}
	if req.BorrowFeePerHour != nil {
		if !rateInRange(*req.BorrowFeePerHour, 0, 0.01) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "borrow_fee_per_hour must be between 0 and 0.01"})
			return
		}
		updates["borrow_fee_per_hour"] = *req.BorrowFeePerHour
	}
	if req.MakerFeeRate != nil {
		if !rateInRange(*req.MakerFeeRate, 0, 0.05) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "maker_fee_rate must be between 0 and 0.05"})
			return
		}
		updates["maker_fee_rate"] = *req.MakerFeeRate
	}
	if req.TakerFeeRate != nil {
		if !rateInRange(*req.TakerFeeRate, 0, 0.05) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "taker_fee_rate must be between 0 and 0.05"})
			return
		}
		updates["taker_fee_rate"] = *req.TakerFeeRate
	}
	if req.MaintenanceMarginRate != nil {
		if !rateInRange(*req.MaintenanceMarginRate, 0.001, 0.5) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "maintenance_margin_rate must be between 0.001 and 0.5"})
			return
		}
		updates["maintenance_margin_rate"] = *req.MaintenanceMarginRate
	}
	if req.LiquidationFeeRate != nil {
		if !rateInRange(*req.LiquidationFeeRate, 0, 0.2) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "liquidation_fee_rate must be between 0 and 0.2"})
			return
		}
		updates["liquidation_fee_rate"] = *req.LiquidationFeeRate
	}
	if req.Status != nil {
		if !allowedListingStatuses[*req.Status] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported status"})
			return
		}
		updates["status"] = *req.Status
	}

	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no updatable fields supplied"})
		return
	}

	result := s.db.Model(&models.PerpsMarket{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update perps market"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "perps market not found"})
		return
	}

	if err := s.db.Where("id = ?", id).First(&market).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "perps market not found"})
		return
	}
	logListingAction(c, s.db, "update_perps_market", "perps_market", id.String(), updates)
	c.JSON(http.StatusOK, market)
}

// DeletePerpsMarket moves a perps listing to delisting.
func (s *ListingService) DeletePerpsMarket(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid market id"})
		return
	}

	result := s.db.Model(&models.PerpsMarket{}).Where("id = ?", id).Update("status", models.ListingStatusDelisting)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delist perps market"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "perps market not found"})
		return
	}

	logListingAction(c, s.db, "delist_perps_market", "perps_market", id.String(), nil)
	c.JSON(http.StatusOK, gin.H{"success": true, "id": id, "status": models.ListingStatusDelisting})
}

// GetPerpsMarkets lists perps listings, optionally filtered by status.
func (s *ListingService) GetPerpsMarkets(c *gin.Context) {
	query := s.db.Model(&models.PerpsMarket{})
	if status := c.Query("status"); status != "" {
		if !allowedListingStatuses[status] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported status filter"})
			return
		}
		query = query.Where("status = ?", status)
	}

	limit, offset := paginationParams(c, 100)
	var markets []models.PerpsMarket
	if err := query.Order("symbol ASC").Limit(limit).Offset(offset).Find(&markets).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load perps markets"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"markets": markets, "count": len(markets)})
}

// ---------------------------------------------------------------------------
// Margin listings
// ---------------------------------------------------------------------------

// CreateMarginMarket validates and persists a spot-margin listing.
func (s *ListingService) CreateMarginMarket(c *gin.Context) {
	var req struct {
		Pair                 string  `json:"pair" binding:"required"`
		MarginMode           string  `json:"margin_mode"`
		MaxLeverage          int     `json:"max_leverage"`
		CollateralAssets     string  `json:"collateral_assets"`
		BorrowableAssets     string  `json:"borrowable_assets"`
		BaseRateAPY          float64 `json:"base_rate_apy"`
		KinkUtilization      float64 `json:"kink_utilization"`
		JumpMultiplier       float64 `json:"jump_multiplier"`
		LTV                  float64 `json:"ltv"`
		LiquidationThreshold float64 `json:"liquidation_threshold"`
		LiquidationBonus     float64 `json:"liquidation_bonus"`
		CloseFactor          float64 `json:"close_factor"`
		SupplyCap            string  `json:"supply_cap"`
		BorrowCap            string  `json:"borrow_cap"`
		RiskTier             string  `json:"risk_tier"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	base, quote, err := parseSymbol(req.Pair)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	marginMode := strings.ToLower(strings.TrimSpace(req.MarginMode))
	if marginMode == "" {
		marginMode = "cross"
	}
	if !allowedMarginModes[marginMode] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "margin_mode must be cross or isolated"})
		return
	}
	if req.MaxLeverage < 1 || req.MaxLeverage > 10 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "max_leverage for spot margin must be between 1 and 10"})
		return
	}
	collateral, err := splitAssets(req.CollateralAssets)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(collateral) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "collateral_assets must list at least one asset"})
		return
	}
	for _, asset := range collateral {
		if !allowedSettlementAssets[asset] {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("unsupported collateral asset %s", asset)})
			return
		}
	}
	borrowable, err := splitAssets(req.BorrowableAssets)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(borrowable) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "borrowable_assets must list at least one asset"})
		return
	}
	if !rateInRange(req.BaseRateAPY, 0, 1) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "base_rate_apy must be between 0 and 1"})
		return
	}
	if !rateInRange(req.KinkUtilization, 0.01, 0.99) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "kink_utilization must be between 0.01 and 0.99"})
		return
	}
	if !rateInRange(req.JumpMultiplier, 0, 100) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "jump_multiplier must be between 0 and 100"})
		return
	}
	if !rateInRange(req.LTV, 0.01, 0.95) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ltv must be between 0.01 and 0.95"})
		return
	}
	if !rateInRange(req.LiquidationThreshold, 0.01, 0.99) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "liquidation_threshold must be between 0.01 and 0.99"})
		return
	}
	if req.LiquidationThreshold < req.LTV {
		c.JSON(http.StatusBadRequest, gin.H{"error": "liquidation_threshold must be at least the LTV"})
		return
	}
	if !rateInRange(req.LiquidationBonus, 0, 0.5) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "liquidation_bonus must be between 0 and 0.5"})
		return
	}
	if !rateInRange(req.CloseFactor, 0.01, 1) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "close_factor must be between 0.01 and 1"})
		return
	}
	if !isPositiveDecimal(req.SupplyCap) || !isPositiveDecimal(req.BorrowCap) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "supply_cap and borrow_cap must be positive decimals"})
		return
	}
	riskTier := strings.ToLower(strings.TrimSpace(req.RiskTier))
	if riskTier == "" {
		riskTier = "medium"
	}
	if !allowedRiskTiers[riskTier] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "risk_tier must be low, medium or high"})
		return
	}

	pair := strings.ToUpper(strings.TrimSpace(req.Pair))
	var existing int64
	s.db.Model(&models.MarginMarket{}).Where("pair = ?", pair).Count(&existing)
	if existing > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "a margin market for this pair already exists"})
		return
	}

	market := models.MarginMarket{
		Pair:                 pair,
		BaseAsset:            base,
		QuoteAsset:           quote,
		MarginMode:           marginMode,
		MaxLeverage:          req.MaxLeverage,
		CollateralAssets:     strings.Join(collateral, ","),
		BorrowableAssets:     strings.Join(borrowable, ","),
		BaseRateAPY:          req.BaseRateAPY,
		KinkUtilization:      req.KinkUtilization,
		JumpMultiplier:       req.JumpMultiplier,
		LTV:                  req.LTV,
		LiquidationThreshold: req.LiquidationThreshold,
		LiquidationBonus:     req.LiquidationBonus,
		CloseFactor:          req.CloseFactor,
		SupplyCap:            strings.TrimSpace(req.SupplyCap),
		BorrowCap:            strings.TrimSpace(req.BorrowCap),
		RiskTier:             riskTier,
		Status:               models.ListingStatusPaused,
	}
	if err := s.db.Create(&market).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create margin market"})
		return
	}

	logListingAction(c, s.db, "create_margin_market", "margin_market", market.ID.String(), gin.H{"pair": market.Pair})
	c.JSON(http.StatusCreated, market)
}

// UpdateMarginMarket applies admin-editable parameters to a margin listing.
func (s *ListingService) UpdateMarginMarket(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid market id"})
		return
	}

	var req struct {
		MarginMode           *string  `json:"margin_mode"`
		MaxLeverage          *int     `json:"max_leverage"`
		CollateralAssets     *string  `json:"collateral_assets"`
		BorrowableAssets     *string  `json:"borrowable_assets"`
		BaseRateAPY          *float64 `json:"base_rate_apy"`
		KinkUtilization      *float64 `json:"kink_utilization"`
		JumpMultiplier       *float64 `json:"jump_multiplier"`
		LTV                  *float64 `json:"ltv"`
		LiquidationThreshold *float64 `json:"liquidation_threshold"`
		LiquidationBonus     *float64 `json:"liquidation_bonus"`
		CloseFactor          *float64 `json:"close_factor"`
		SupplyCap            *string  `json:"supply_cap"`
		BorrowCap            *string  `json:"borrow_cap"`
		RiskTier             *string  `json:"risk_tier"`
		Status               *string  `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var market models.MarginMarket
	if err := s.db.Where("id = ?", id).First(&market).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "margin market not found"})
		return
	}

	updates := map[string]interface{}{}
	if req.MarginMode != nil {
		if !allowedMarginModes[*req.MarginMode] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "margin_mode must be cross or isolated"})
			return
		}
		updates["margin_mode"] = *req.MarginMode
	}
	if req.MaxLeverage != nil {
		if *req.MaxLeverage < 1 || *req.MaxLeverage > 10 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "max_leverage for spot margin must be between 1 and 10"})
			return
		}
		updates["max_leverage"] = *req.MaxLeverage
	}
	if req.CollateralAssets != nil {
		assets, err := splitAssets(*req.CollateralAssets)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if len(assets) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "collateral_assets must list at least one asset"})
			return
		}
		for _, asset := range assets {
			if !allowedSettlementAssets[asset] {
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("unsupported collateral asset %s", asset)})
				return
			}
		}
		updates["collateral_assets"] = strings.Join(assets, ",")
	}
	if req.BorrowableAssets != nil {
		assets, err := splitAssets(*req.BorrowableAssets)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if len(assets) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "borrowable_assets must list at least one asset"})
			return
		}
		updates["borrowable_assets"] = strings.Join(assets, ",")
	}
	if req.BaseRateAPY != nil {
		if !rateInRange(*req.BaseRateAPY, 0, 1) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "base_rate_apy must be between 0 and 1"})
			return
		}
		updates["base_rate_apy"] = *req.BaseRateAPY
	}
	if req.KinkUtilization != nil {
		if !rateInRange(*req.KinkUtilization, 0.01, 0.99) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "kink_utilization must be between 0.01 and 0.99"})
			return
		}
		updates["kink_utilization"] = *req.KinkUtilization
	}
	if req.JumpMultiplier != nil {
		if !rateInRange(*req.JumpMultiplier, 0, 100) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "jump_multiplier must be between 0 and 100"})
			return
		}
		updates["jump_multiplier"] = *req.JumpMultiplier
	}
	if req.LTV != nil {
		if !rateInRange(*req.LTV, 0.01, 0.95) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "ltv must be between 0.01 and 0.95"})
			return
		}
		updates["ltv"] = *req.LTV
	}
	if req.LiquidationThreshold != nil {
		if !rateInRange(*req.LiquidationThreshold, 0.01, 0.99) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "liquidation_threshold must be between 0.01 and 0.99"})
			return
		}
		updates["liquidation_threshold"] = *req.LiquidationThreshold
	}
	if req.LiquidationBonus != nil {
		if !rateInRange(*req.LiquidationBonus, 0, 0.5) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "liquidation_bonus must be between 0 and 0.5"})
			return
		}
		updates["liquidation_bonus"] = *req.LiquidationBonus
	}
	if req.CloseFactor != nil {
		if !rateInRange(*req.CloseFactor, 0.01, 1) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "close_factor must be between 0.01 and 1"})
			return
		}
		updates["close_factor"] = *req.CloseFactor
	}
	if req.SupplyCap != nil {
		if !isPositiveDecimal(*req.SupplyCap) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "supply_cap must be a positive decimal"})
			return
		}
		updates["supply_cap"] = strings.TrimSpace(*req.SupplyCap)
	}
	if req.BorrowCap != nil {
		if !isPositiveDecimal(*req.BorrowCap) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "borrow_cap must be a positive decimal"})
			return
		}
		updates["borrow_cap"] = strings.TrimSpace(*req.BorrowCap)
	}
	if req.RiskTier != nil {
		if !allowedRiskTiers[*req.RiskTier] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "risk_tier must be low, medium or high"})
			return
		}
		updates["risk_tier"] = *req.RiskTier
	}
	if req.Status != nil {
		if !allowedListingStatuses[*req.Status] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported status"})
			return
		}
		updates["status"] = *req.Status
	}

	// The liquidation threshold must stay at or above the LTV after the update.
	finalLTV := market.LTV
	finalThreshold := market.LiquidationThreshold
	if v, ok := updates["ltv"].(float64); ok {
		finalLTV = v
	}
	if v, ok := updates["liquidation_threshold"].(float64); ok {
		finalThreshold = v
	}
	if finalThreshold < finalLTV {
		c.JSON(http.StatusBadRequest, gin.H{"error": "liquidation_threshold must be at least the LTV"})
		return
	}

	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no updatable fields supplied"})
		return
	}

	result := s.db.Model(&models.MarginMarket{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update margin market"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "margin market not found"})
		return
	}

	if err := s.db.Where("id = ?", id).First(&market).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "margin market not found"})
		return
	}
	logListingAction(c, s.db, "update_margin_market", "margin_market", id.String(), updates)
	c.JSON(http.StatusOK, market)
}

// DeleteMarginMarket moves a margin listing to delisting.
func (s *ListingService) DeleteMarginMarket(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid market id"})
		return
	}

	result := s.db.Model(&models.MarginMarket{}).Where("id = ?", id).Update("status", models.ListingStatusDelisting)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delist margin market"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "margin market not found"})
		return
	}

	logListingAction(c, s.db, "delist_margin_market", "margin_market", id.String(), nil)
	c.JSON(http.StatusOK, gin.H{"success": true, "id": id, "status": models.ListingStatusDelisting})
}

// GetMarginMarkets lists margin listings, optionally filtered by status.
func (s *ListingService) GetMarginMarkets(c *gin.Context) {
	query := s.db.Model(&models.MarginMarket{})
	if status := c.Query("status"); status != "" {
		if !allowedListingStatuses[status] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported status filter"})
			return
		}
		query = query.Where("status = ?", status)
	}

	limit, offset := paginationParams(c, 100)
	var markets []models.MarginMarket
	if err := query.Order("pair ASC").Limit(limit).Offset(offset).Find(&markets).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load margin markets"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"markets": markets, "count": len(markets)})
}

// logListingAction records an immutable audit entry for a listing change.
func logListingAction(c *gin.Context, db *gorm.DB, action, entityType, entityID string, details interface{}) {
	encoded := ""
	if details != nil {
		if raw, err := json.Marshal(details); err == nil {
			encoded = string(raw)
		}
	}
	entry := models.AdminLog{
		AdminID:    adminUUID(c.GetString("user_id")),
		Action:     action,
		EntityType: entityType,
		EntityID:   entityID,
		Details:    encoded,
		IPAddress:  c.ClientIP(),
		UserAgent:  c.Request.UserAgent(),
	}
	if err := db.Create(&entry).Error; err != nil {
		log.Printf("failed to write listing audit log: %v", err)
	}
}

// ErrListingNotFound is returned by the lookup helpers below.
var ErrListingNotFound = errors.New("listing not found")
