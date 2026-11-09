package services

import (
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"tigerswap/backend/models"
)

// OptionService implements the self-built European option AMM described by
// contracts/TigerSwapOptions.sol. A series is a fully collateralised pool:
// liquidity providers deposit the settlement asset and are the counterparty to
// every trade, buyers pay a Black-Scholes premium and receive the intrinsic
// value at expiry.
//
// Every premium and payout is computed with the exact fixed-point port in
// option_math.go, so an off-chain quote and an on-chain settlement agree. Money
// moves through the platform ledger: a premium is escrowed when a position is
// opened and a payout is credited from the series pool when it is exercised.
// The pool can never pay out more than it holds, so a payout is bounded by the
// collateral actually deposited.
type OptionService struct {
	db     *gorm.DB
	ledger *LedgerService
	prices priceSource
}

func NewOptionService(db *gorm.DB, ledger *LedgerService) *OptionService {
	return &OptionService{db: db, ledger: ledger}
}

// SetPriceSource wires the oracle used to refresh a series mark price.
func (s *OptionService) SetPriceSource(source priceSource) { s.prices = source }

// defaultOptionRiskFreeRate is the annual risk-free rate, 5%, matching the
// Solidity library's default.
var defaultOptionRiskFreeRate = mustOptInt("50000000000000000")

// optMaxIV is the cap on implied volatility after skew, 500%, matching the
// on-chain constant.
var optMaxIV = mustOptInt("5000000000000000000")

// optSkew is added to the base IV at full utilisation, 200%, matching the
// on-chain constant.
var optSkew = mustOptInt("2000000000000000000")

var allowedOptionStatuses = map[string]bool{
	models.OptionStatusActive:  true,
	models.OptionStatusPaused:  true,
	models.OptionStatusExpired: true,
	models.OptionStatusSettled: true,
}

// errSeriesNotOpen is returned when a trade is attempted against a series that
// is not active or has expired. It is checked against the series re-read under
// lock, so a concurrent admin change cannot be raced into a trade.
var errSeriesNotOpen = errors.New("option series is not open for trading")

// Settlement and liquidity errors. Each is a distinct value so a handler can map
// it to the right HTTP status without matching on a message string.
var (
	errOptionInsufficientPool   = errors.New("the pool has insufficient collateral to underwrite this size")
	errOptionCollateralLocked   = errors.New("collateral is locked by open positions")
	errOptionCollateralReserved = errors.New("collateral is reserved for open positions")
	errOptionExceedsPrincipal   = errors.New("amount exceeds your deposited principal")
	errOptionNoLiquidity        = errors.New("you have no liquidity in this series")
	errOptionNoPosition         = errors.New("no position to exercise")
	errOptionPoolCannotCover    = errors.New("the pool cannot cover this payout")
)

// parseOptionAmount parses a non-negative base-10 integer amount.
func parseOptionAmount(raw string) (*big.Int, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, publicErr(errors.New("amount is required"))
	}
	amount, ok := new(big.Int).SetString(trimmed, 10)
	if !ok {
		return nil, publicErr(errors.New("amount must be a base-10 integer"))
	}
	if amount.Sign() < 0 {
		return nil, errors.New("amount must not be negative")
	}
	return amount, nil
}

// loadSeries reads a series by id, returning ErrListingNotFound when absent.
func (s *OptionService) loadSeries(id uuid.UUID) (*models.OptionSeries, error) {
	var series models.OptionSeries
	if err := s.db.First(&series, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrListingNotFound
		}
		return nil, err
	}
	return &series, nil
}

// seriesBigInt reads a stored decimal field, treating an unparsable value as
// zero. A stored value is always written by this service, so a parse failure
// would mean a corrupt row; defaulting to zero keeps the arithmetic total
// rather than panicking.
func seriesBigInt(raw string) *big.Int {
	value, ok := new(big.Int).SetString(strings.TrimSpace(raw), 10)
	if !ok {
		return big.NewInt(0)
	}
	return value
}

// optionImpliedVolatility returns the series IV, skewed upward by how much of
// the collateral has already been sold. Mirrors TigerSwapOptions.impliedVolatility.
func optionImpliedVolatility(series *models.OptionSeries) (*big.Int, error) {
	baseIV, err := parseWad(series.BaseIV)
	if err != nil {
		return nil, fmt.Errorf("series base_iv: %w", err)
	}
	collateral := seriesBigInt(series.Collateral)
	if collateral.Sign() == 0 {
		return baseIV, nil
	}
	locked := seriesBigInt(series.LockedCollateral)

	utilization := optRawDiv(new(big.Int).Mul(locked, optWAD), collateral)
	if utilization.Cmp(optWAD) > 0 {
		utilization = new(big.Int).Set(optWAD)
	}
	iv := new(big.Int).Add(baseIV, optRawDiv(new(big.Int).Mul(optSkew, utilization), optWAD))
	if iv.Cmp(optMaxIV) > 0 {
		iv = new(big.Int).Set(optMaxIV)
	}
	return iv, nil
}

// optionPremium prices `size` of a series, mirroring TigerSwapOptions.premium.
// `size` is in 1e18 fixed point. A series at or past expiry is worth zero.
func optionPremium(series *models.OptionSeries, size *big.Int, now time.Time) (*big.Int, error) {
	if series.Status == models.OptionStatusSettled {
		return nil, errors.New("series already settled")
	}
	if !now.Before(series.Expiry) {
		return big.NewInt(0), nil
	}
	markPrice, err := parseWad(series.MarkPrice)
	if err != nil {
		return nil, fmt.Errorf("series mark_price: %w", err)
	}
	strike, err := parseWad(series.Strike)
	if err != nil {
		return nil, fmt.Errorf("series strike: %w", err)
	}
	iv, err := optionImpliedVolatility(series)
	if err != nil {
		return nil, err
	}
	timeToExpiry := int64(series.Expiry.Sub(now).Seconds())
	unitPrice, err := optBlackScholes(markPrice, strike, timeToExpiry, defaultOptionRiskFreeRate, iv, series.IsCall)
	if err != nil {
		return nil, err
	}
	return optMul(unitPrice, size), nil
}

// optionRequiredCollateral is the collateral the pool must lock to underwrite
// `size`, mirroring TigerSwapOptions.requiredCollateral: size * spot for a call,
// size * strike for a put.
func optionRequiredCollateral(series *models.OptionSeries, size *big.Int) (*big.Int, error) {
	var perUnit *big.Int
	var err error
	if series.IsCall {
		perUnit, err = parseWad(series.MarkPrice)
	} else {
		perUnit, err = parseWad(series.Strike)
	}
	if err != nil {
		return nil, err
	}
	return optMul(perUnit, size), nil
}

// optionPayoutAt is the intrinsic payout for `size` at the settlement price,
// mirroring TigerSwapOptions.payoutAt.
func optionPayoutAt(series *models.OptionSeries, size *big.Int) (*big.Int, error) {
	if series.SettlementPrice == nil {
		return nil, errors.New("series not settled")
	}
	settlementPrice, err := parseWad(*series.SettlementPrice)
	if err != nil {
		return nil, fmt.Errorf("series settlement_price: %w", err)
	}
	strike, err := parseWad(series.Strike)
	if err != nil {
		return nil, fmt.Errorf("series strike: %w", err)
	}

	var perUnit *big.Int
	if series.IsCall {
		perUnit = new(big.Int).Sub(settlementPrice, strike)
	} else {
		perUnit = new(big.Int).Sub(strike, settlementPrice)
	}
	if perUnit.Sign() < 0 {
		perUnit = big.NewInt(0)
	}
	return optMul(perUnit, size), nil
}

// SeriesView is a series with its live greeks and unit premium.
type SeriesView struct {
	Series   models.OptionSeries `json:"series"`
	IV       string              `json:"implied_volatility"`
	Delta    string              `json:"delta"`
	Gamma    string              `json:"gamma"`
	Vega     string              `json:"vega"`
	Theta    string              `json:"theta"`
	Premium1 string              `json:"premium_per_unit"`
	Expired  bool                `json:"expired"`
}

// buildSeriesView decorates a series with its greeks and unit premium. A series
// that cannot be priced reports the greeks and premium as "0" rather than a
// fabricated number.
func (s *OptionService) buildSeriesView(series models.OptionSeries, now time.Time) SeriesView {
	view := SeriesView{Series: series, IV: "0", Delta: "0", Gamma: "0", Vega: "0", Theta: "0", Premium1: "0"}
	view.Expired = !now.Before(series.Expiry)

	iv, err := optionImpliedVolatility(&series)
	if err != nil {
		return view
	}
	view.IV = formatWad(iv)

	markPrice, err := parseWad(series.MarkPrice)
	if err != nil {
		return view
	}
	strike, err := parseWad(series.Strike)
	if err != nil {
		return view
	}

	greeks, err := optGreeks(markPrice, strike, int64(series.Expiry.Sub(now).Seconds()), defaultOptionRiskFreeRate, iv, series.IsCall)
	if err == nil {
		view.Delta = formatWad(greeks.Delta)
		view.Gamma = formatWad(greeks.Gamma)
		view.Vega = formatWad(greeks.Vega)
		view.Theta = formatWad(greeks.Theta)
	}

	unitPremium, err := optionPremium(&series, optWAD, now)
	if err == nil {
		view.Premium1 = formatWad(unitPremium)
	}
	return view
}

// buildSeriesViews decorates a slice of series.
func (s *OptionService) buildSeriesViews(series []models.OptionSeries) []SeriesView {
	now := time.Now()
	views := make([]SeriesView, 0, len(series))
	for _, item := range series {
		views = append(views, s.buildSeriesView(item, now))
	}
	return views
}

// ---- Admin: series management ---------------------------------------------

type createSeriesRequest struct {
	Key                     string `json:"key" binding:"required"`
	Name                    string `json:"name" binding:"required"`
	ChainID                 int64  `json:"chain_id" binding:"required"`
	UnderlyingSymbol        string `json:"underlying_symbol" binding:"required"`
	CollateralTokenAddress  string `json:"collateral_token_address"`
	CollateralTokenSymbol   string `json:"collateral_token_symbol" binding:"required"`
	CollateralTokenDecimals *int   `json:"collateral_token_decimals"`
	Strike                  string `json:"strike" binding:"required"`
	BaseIV                  string `json:"base_iv" binding:"required"`
	Expiry                  string `json:"expiry" binding:"required"`
	IsCall                  *bool  `json:"is_call" binding:"required"`
	MarkPrice               string `json:"mark_price" binding:"required"`
	Status                  string `json:"status"`
}

// CreateSeries creates an option series. It is admin-only. The series starts
// with no collateral, so no trade can be priced until an LP deposits.
func (s *OptionService) CreateSeries(c *gin.Context) {
	var req createSeriesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequestError(c, err)
		return
	}

	key := strings.TrimSpace(req.Key)
	name := strings.TrimSpace(req.Name)
	if key == "" || name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "key and name are required"})
		return
	}
	if req.ChainID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "chain_id must be positive"})
		return
	}

	underlying := strings.ToUpper(strings.TrimSpace(req.UnderlyingSymbol))
	if underlying == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "underlying_symbol is required"})
		return
	}
	collateralSymbol := strings.ToUpper(strings.TrimSpace(req.CollateralTokenSymbol))
	if !models.IsSupportedOptionCollateral(collateralSymbol) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "collateral_token_symbol must be RUSD, USDT, USDC or RYUSD"})
		return
	}
	collateralAddress, err := tokenKeyOrNative(req.CollateralTokenAddress)
	if err != nil {
		respondBadRequestError(c, err)
		return
	}
	collateralDecimals := 18
	if req.CollateralTokenDecimals != nil {
		if *req.CollateralTokenDecimals < 0 || *req.CollateralTokenDecimals > 36 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "collateral_token_decimals must be between 0 and 36"})
			return
		}
		collateralDecimals = *req.CollateralTokenDecimals
	}

	strike, err := parseWad(req.Strike)
	if err != nil || strike.Sign() <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "strike must be a positive number"})
		return
	}
	baseIV, err := parseWad(req.BaseIV)
	if err != nil || baseIV.Sign() <= 0 || baseIV.Cmp(optMaxIV) > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "base_iv must be positive and at most 5"})
		return
	}
	markPrice, err := parseWad(req.MarkPrice)
	if err != nil || markPrice.Sign() <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "mark_price must be a positive number"})
		return
	}
	expiry, err := time.Parse(time.RFC3339, strings.TrimSpace(req.Expiry))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "expiry must be an RFC3339 timestamp"})
		return
	}
	if !expiry.After(time.Now()) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "expiry must be in the future"})
		return
	}

	status := models.OptionStatusPaused
	if raw := strings.ToLower(strings.TrimSpace(req.Status)); raw != "" {
		if !allowedOptionStatuses[raw] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "status must be active, paused, expired or settled"})
			return
		}
		status = raw
	}

	var existing models.OptionSeries
	if err := s.db.First(&existing, "key = ?", key).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "a series with that key already exists"})
		return
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check for an existing series"})
		return
	}

	series := models.OptionSeries{
		Key:                     key,
		Name:                    name,
		ChainID:                 req.ChainID,
		UnderlyingSymbol:        underlying,
		CollateralTokenAddress:  collateralAddress,
		CollateralTokenSymbol:   collateralSymbol,
		CollateralTokenDecimals: collateralDecimals,
		Strike:                  formatWad(strike),
		BaseIV:                  formatWad(baseIV),
		Expiry:                  expiry,
		IsCall:                  *req.IsCall,
		MarkPrice:               formatWad(markPrice),
		Collateral:              "0",
		LockedCollateral:        "0",
		PremiumCollected:        "0",
		TotalLongSize:           "0",
		Status:                  status,
		CreatedBy:               c.GetString("user_id"),
	}
	if err := s.db.Create(&series).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create series"})
		return
	}
	logListingAction(c, s.db, "create_option_series", "option_series", series.ID.String(), gin.H{
		"key": key, "strike": series.Strike, "is_call": series.IsCall, "expiry": series.Expiry,
	})
	c.JSON(http.StatusCreated, gin.H{"success": true, "series": s.buildSeriesView(series, time.Now())})
}

type updateSeriesRequest struct {
	Name      *string `json:"name"`
	BaseIV    *string `json:"base_iv"`
	MarkPrice *string `json:"mark_price"`
	Expiry    *string `json:"expiry"`
	Status    *string `json:"status"`
}

// UpdateSeries edits an admin-managed series. A settled series is immutable.
func (s *OptionService) UpdateSeries(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid series id"})
		return
	}
	var req updateSeriesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequestError(c, err)
		return
	}
	series, err := s.loadSeries(id)
	if err != nil {
		if errors.Is(err, ErrListingNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "series not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load series"})
		return
	}
	if series.Status == models.OptionStatusSettled {
		c.JSON(http.StatusConflict, gin.H{"error": "a settled series cannot be edited"})
		return
	}

	updates := map[string]interface{}{}
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "name must not be empty"})
			return
		}
		updates["name"] = name
	}
	if req.BaseIV != nil {
		baseIV, err := parseWad(*req.BaseIV)
		if err != nil || baseIV.Sign() <= 0 || baseIV.Cmp(optMaxIV) > 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "base_iv must be positive and at most 5"})
			return
		}
		updates["base_iv"] = formatWad(baseIV)
	}
	if req.MarkPrice != nil {
		markPrice, err := parseWad(*req.MarkPrice)
		if err != nil || markPrice.Sign() <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "mark_price must be a positive number"})
			return
		}
		updates["mark_price"] = formatWad(markPrice)
	}
	if req.Expiry != nil {
		expiry, err := time.Parse(time.RFC3339, strings.TrimSpace(*req.Expiry))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "expiry must be an RFC3339 timestamp"})
			return
		}
		if !expiry.After(time.Now()) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "expiry must be in the future"})
			return
		}
		updates["expiry"] = expiry
	}
	if req.Status != nil {
		status := strings.ToLower(strings.TrimSpace(*req.Status))
		if !allowedOptionStatuses[status] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "status must be active, paused, expired or settled"})
			return
		}
		if status == models.OptionStatusSettled {
			c.JSON(http.StatusBadRequest, gin.H{"error": "use the settle endpoint to settle a series"})
			return
		}
		updates["status"] = status
	}
	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no updatable fields were provided"})
		return
	}
	if err := s.db.Model(&models.OptionSeries{}).Where("id = ?", series.ID).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update series"})
		return
	}
	logListingAction(c, s.db, "update_option_series", "option_series", series.ID.String(), updates)
	updated, err := s.loadSeries(series.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to reload series"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "series": s.buildSeriesView(*updated, time.Now())})
}

// DeleteSeries closes a series. A series with open positions or collateral is
// not removed, so a holder's position cannot be stranded.
func (s *OptionService) DeleteSeries(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid series id"})
		return
	}
	series, err := s.loadSeries(id)
	if err != nil {
		if errors.Is(err, ErrListingNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "series not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load series"})
		return
	}
	if seriesBigInt(series.TotalLongSize).Sign() > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "series still has open positions"})
		return
	}
	if seriesBigInt(series.Collateral).Sign() > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "series still holds collateral; LPs must withdraw first"})
		return
	}
	if err := s.db.Model(&models.OptionSeries{}).Where("id = ?", series.ID).
		Update("status", models.OptionStatusExpired).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to close series"})
		return
	}
	logListingAction(c, s.db, "delete_option_series", "option_series", series.ID.String(), gin.H{"status": models.OptionStatusExpired})
	c.JSON(http.StatusOK, gin.H{"success": true, "status": models.OptionStatusExpired})
}

// GetSeries lists every series with its greeks.
func (s *OptionService) GetSeries(c *gin.Context) {
	series := []models.OptionSeries{}
	if err := s.db.Order("created_at desc").Find(&series).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list series"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "series": s.buildSeriesViews(series)})
}

// GetActiveSeries lists series a user can currently trade.
func (s *OptionService) GetActiveSeries(c *gin.Context) {
	now := time.Now()
	series := []models.OptionSeries{}
	if err := s.db.Where("status = ? AND expiry > ?", models.OptionStatusActive, now).
		Order("expiry asc").Find(&series).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list series"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "series": s.buildSeriesViews(series)})
}

// GetSeriesDetail returns one series with its greeks.
func (s *OptionService) GetSeriesDetail(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid series id"})
		return
	}
	series, err := s.loadSeries(id)
	if err != nil {
		if errors.Is(err, ErrListingNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "series not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load series"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "series": s.buildSeriesView(*series, time.Now())})
}

// ---- Admin: settlement -----------------------------------------------------

type settleSeriesRequest struct {
	SettlementPrice string `json:"settlement_price" binding:"required"`
}

// SettleSeries records the settlement price of an expired series. It is
// admin-only. Once settled, holders can exercise and LPs can withdraw.
func (s *OptionService) SettleSeries(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid series id"})
		return
	}
	var req settleSeriesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequestError(c, err)
		return
	}
	settlementPrice, err := parseWad(req.SettlementPrice)
	if err != nil || settlementPrice.Sign() <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "settlement_price must be a positive number"})
		return
	}

	series, err := s.loadSeries(id)
	if err != nil {
		if errors.Is(err, ErrListingNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "series not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load series"})
		return
	}
	if series.Status == models.OptionStatusSettled {
		c.JSON(http.StatusConflict, gin.H{"error": "series is already settled"})
		return
	}
	if time.Now().Before(series.Expiry) {
		c.JSON(http.StatusConflict, gin.H{"error": "series has not expired"})
		return
	}

	formatted := formatWad(settlementPrice)
	if err := s.db.Model(&models.OptionSeries{}).Where("id = ?", series.ID).Updates(map[string]interface{}{
		"settlement_price": formatted,
		"status":           models.OptionStatusSettled,
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to settle series"})
		return
	}
	logListingAction(c, s.db, "settle_option_series", "option_series", series.ID.String(), gin.H{"settlement_price": formatted})
	updated, err := s.loadSeries(series.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to reload series"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "series": s.buildSeriesView(*updated, time.Now())})
}

// ---- Liquidity -------------------------------------------------------------

type collateralRequest struct {
	Amount string `json:"amount" binding:"required"`
}

// AddCollateral deposits settlement collateral to underwrite a series. The
// deposit is debited from the caller's ledger balance in the same transaction
// as the series update, so the pool can never record collateral it did not
// receive.
func (s *OptionService) AddCollateral(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid series id"})
		return
	}
	var req collateralRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequestError(c, err)
		return
	}
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}
	if s.ledger == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "ledger is not configured"})
		return
	}
	amount, err := parseOptionAmount(req.Amount)
	if err != nil || amount.Sign() <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "amount must be a positive integer"})
		return
	}

	series, err := s.loadSeries(id)
	if err != nil {
		if errors.Is(err, ErrListingNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "series not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load series"})
		return
	}
	if series.Status == models.OptionStatusSettled {
		c.JSON(http.StatusConflict, gin.H{"error": "series already settled"})
		return
	}

	err = s.db.Transaction(func(tx *gorm.DB) error {
		var lockedSeries models.OptionSeries
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&lockedSeries, "id = ?", series.ID).Error; err != nil {
			return err
		}
		if lockedSeries.Status == models.OptionStatusSettled {
			return errSeriesNotOpen
		}
		if err := s.ledger.debitWithin(tx, userID, lockedSeries.ChainID, lockedSeries.CollateralTokenAddress, amount); err != nil {
			return err
		}
		newCollateral := new(big.Int).Add(seriesBigInt(lockedSeries.Collateral), amount)
		if err := tx.Model(&models.OptionSeries{}).Where("id = ?", lockedSeries.ID).
			Update("collateral", newCollateral.String()).Error; err != nil {
			return err
		}
		return upsertOptionLiquidity(tx, lockedSeries.ID, userID, amount)
	})
	if err != nil {
		if errors.Is(err, ErrInsufficientBalance) {
			c.JSON(http.StatusConflict, gin.H{"error": "insufficient balance to add collateral"})
			return
		}
		if errors.Is(err, errSeriesNotOpen) {
			c.JSON(http.StatusConflict, gin.H{"error": "series already settled"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to add collateral"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "amount": amount.String()})
}

// RemoveCollateral withdraws collateral that is not backing open positions. It
// is bounded by the caller's own principal and by the pool's free collateral, so
// an LP cannot withdraw funds an option holder is owed.
func (s *OptionService) RemoveCollateral(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid series id"})
		return
	}
	var req collateralRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequestError(c, err)
		return
	}
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}
	if s.ledger == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "ledger is not configured"})
		return
	}
	amount, err := parseOptionAmount(req.Amount)
	if err != nil || amount.Sign() <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "amount must be a positive integer"})
		return
	}

	series, err := s.loadSeries(id)
	if err != nil {
		if errors.Is(err, ErrListingNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "series not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load series"})
		return
	}
	if series.Status == models.OptionStatusSettled {
		c.JSON(http.StatusConflict, gin.H{"error": "series already settled; use the withdraw endpoint"})
		return
	}

	err = s.db.Transaction(func(tx *gorm.DB) error {
		var lockedSeries models.OptionSeries
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&lockedSeries, "id = ?", series.ID).Error; err != nil {
			return err
		}
		collateral := seriesBigInt(lockedSeries.Collateral)
		locked := seriesBigInt(lockedSeries.LockedCollateral)
		free := new(big.Int).Sub(collateral, locked)
		if free.Sign() < 0 {
			free = big.NewInt(0)
		}
		if amount.Cmp(free) > 0 {
			return errOptionCollateralLocked
		}

		var liquidity models.OptionLiquidity
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&liquidity, "series_id = ? AND user_id = ?", lockedSeries.ID, userID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errOptionNoLiquidity
			}
			return err
		}
		principal := seriesBigInt(liquidity.Principal)
		if amount.Cmp(principal) > 0 {
			return errOptionExceedsPrincipal
		}

		newCollateral := new(big.Int).Sub(collateral, amount)
		newPrincipal := new(big.Int).Sub(principal, amount)
		if err := tx.Model(&models.OptionSeries{}).Where("id = ?", lockedSeries.ID).
			Update("collateral", newCollateral.String()).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.OptionLiquidity{}).Where("id = ?", liquidity.ID).
			Update("principal", newPrincipal.String()).Error; err != nil {
			return err
		}
		return s.ledger.creditWithin(tx, userID, lockedSeries.ChainID, lockedSeries.CollateralTokenAddress,
			lockedSeries.CollateralTokenSymbol, lockedSeries.CollateralTokenDecimals, amount)
	})
	if err != nil {
		switch {
		case errors.Is(err, errOptionCollateralLocked),
			errors.Is(err, errOptionExceedsPrincipal),
			errors.Is(err, errOptionNoLiquidity):
			respondServiceError(c, err)
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to remove collateral"})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "amount": amount.String()})
}

// upsertOptionLiquidity adds `amount` to an LP's recorded principal for a series.
func upsertOptionLiquidity(tx *gorm.DB, seriesID uuid.UUID, userID string, amount *big.Int) error {
	var liquidity models.OptionLiquidity
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		First(&liquidity, "series_id = ? AND user_id = ?", seriesID, userID).Error
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		return tx.Create(&models.OptionLiquidity{
			SeriesID:  seriesID,
			UserID:    userID,
			Principal: amount.String(),
		}).Error
	case err != nil:
		return err
	default:
		newPrincipal := new(big.Int).Add(seriesBigInt(liquidity.Principal), amount)
		return tx.Model(&models.OptionLiquidity{}).Where("id = ?", liquidity.ID).
			Update("principal", newPrincipal.String()).Error
	}
}

// ---- Trading ---------------------------------------------------------------

type buyOptionRequest struct {
	Size       string `json:"size" binding:"required"`
	MaxPremium string `json:"max_premium" binding:"required"`
}

// Buy purchases a long option position. The premium is debited from the buyer in
// the same transaction that records the position and increases the series
// collateral, so the pool receives the premium it is owed.
func (s *OptionService) Buy(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid series id"})
		return
	}
	var req buyOptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequestError(c, err)
		return
	}
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}
	if s.ledger == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "ledger is not configured"})
		return
	}
	size, err := parseWad(req.Size)
	if err != nil || size.Sign() <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "size must be a positive number"})
		return
	}
	maxPremium, err := parseOptionAmount(req.MaxPremium)
	if err != nil || maxPremium.Sign() <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "max_premium must be a positive integer"})
		return
	}

	series, err := s.loadSeries(id)
	if err != nil {
		if errors.Is(err, ErrListingNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "series not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load series"})
		return
	}
	now := time.Now()
	if series.Status != models.OptionStatusActive {
		c.JSON(http.StatusConflict, gin.H{"error": "series is not active"})
		return
	}
	if !now.Before(series.Expiry) {
		c.JSON(http.StatusConflict, gin.H{"error": "series has expired"})
		return
	}

	premium, err := optionPremium(series, size, now)
	if err != nil {
		respondBadRequestError(c, err)
		return
	}
	if premium.Sign() <= 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "premium is zero"})
		return
	}
	if premium.Cmp(maxPremium) > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "premium above the limit you set"})
		return
	}
	lock, err := optionRequiredCollateral(series, size)
	if err != nil {
		respondBadRequestError(c, err)
		return
	}

	err = s.db.Transaction(func(tx *gorm.DB) error {
		var lockedSeries models.OptionSeries
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&lockedSeries, "id = ?", series.ID).Error; err != nil {
			return err
		}
		if lockedSeries.Status != models.OptionStatusActive || !time.Now().Before(lockedSeries.Expiry) {
			return errSeriesNotOpen
		}
		collateral := seriesBigInt(lockedSeries.Collateral)
		locked := seriesBigInt(lockedSeries.LockedCollateral)
		if new(big.Int).Add(locked, lock).Cmp(collateral) > 0 {
			return errOptionInsufficientPool
		}
		if err := s.ledger.debitWithin(tx, userID, lockedSeries.ChainID, lockedSeries.CollateralTokenAddress, premium); err != nil {
			return err
		}
		// The premium joins the pool's collateral, exactly as on-chain: the
		// tokens would otherwise sit unaccounted for and could never be paid out.
		newCollateral := new(big.Int).Add(collateral, premium)
		newLocked := new(big.Int).Add(locked, lock)
		newPremiumCollected := new(big.Int).Add(seriesBigInt(lockedSeries.PremiumCollected), premium)
		newTotalLongSize := new(big.Int).Add(seriesBigInt(lockedSeries.TotalLongSize), size)
		if err := tx.Model(&models.OptionSeries{}).Where("id = ?", lockedSeries.ID).Updates(map[string]interface{}{
			"collateral":        newCollateral.String(),
			"locked_collateral": newLocked.String(),
			"premium_collected": newPremiumCollected.String(),
			"total_long_size":   newTotalLongSize.String(),
		}).Error; err != nil {
			return err
		}
		return upsertOptionPosition(tx, lockedSeries.ID, userID, size, premium)
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrInsufficientBalance):
			c.JSON(http.StatusConflict, gin.H{"error": "insufficient balance to pay the premium"})
		case errors.Is(err, errSeriesNotOpen):
			c.JSON(http.StatusConflict, gin.H{"error": "series is not open for trading"})
		case errors.Is(err, errOptionInsufficientPool):
			c.JSON(http.StatusConflict, gin.H{"error": "the pool has insufficient collateral to underwrite this size"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to buy option"})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "premium": premium.String(), "size": formatWad(size)})
}

// upsertOptionPosition adds `size` and `premium` to a holder's position.
func upsertOptionPosition(tx *gorm.DB, seriesID uuid.UUID, userID string, size, premium *big.Int) error {
	var position models.OptionPosition
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		First(&position, "series_id = ? AND user_id = ?", seriesID, userID).Error
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		return tx.Create(&models.OptionPosition{
			SeriesID:    seriesID,
			UserID:      userID,
			Size:        size.String(),
			PremiumPaid: premium.String(),
		}).Error
	case err != nil:
		return err
	default:
		newSize := new(big.Int).Add(seriesBigInt(position.Size), size)
		newPremium := new(big.Int).Add(seriesBigInt(position.PremiumPaid), premium)
		return tx.Model(&models.OptionPosition{}).Where("id = ?", position.ID).Updates(map[string]interface{}{
			"size":         newSize.String(),
			"premium_paid": newPremium.String(),
		}).Error
	}
}

// ---- Settlement: exercise and withdraw -------------------------------------

// Exercise claims the intrinsic payout on a settled position. The payout is
// credited from the series pool in the same transaction that zeroes the
// position, so a position cannot be paid twice and the pool can never pay more
// than it holds.
func (s *OptionService) Exercise(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid series id"})
		return
	}
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}
	if s.ledger == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "ledger is not configured"})
		return
	}

	series, err := s.loadSeries(id)
	if err != nil {
		if errors.Is(err, ErrListingNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "series not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load series"})
		return
	}
	if series.Status != models.OptionStatusSettled {
		c.JSON(http.StatusConflict, gin.H{"error": "series not settled"})
		return
	}

	payout := big.NewInt(0)
	err = s.db.Transaction(func(tx *gorm.DB) error {
		var lockedSeries models.OptionSeries
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&lockedSeries, "id = ?", series.ID).Error; err != nil {
			return err
		}
		if lockedSeries.Status != models.OptionStatusSettled {
			return errSeriesNotOpen
		}
		var position models.OptionPosition
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&position, "series_id = ? AND user_id = ?", lockedSeries.ID, userID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errOptionNoPosition
			}
			return err
		}
		size := seriesBigInt(position.Size)
		if size.Sign() <= 0 {
			return errOptionNoPosition
		}

		computed, err := optionPayoutAt(&lockedSeries, size)
		if err != nil {
			return err
		}
		payout = computed

		collateral := seriesBigInt(lockedSeries.Collateral)
		if payout.Cmp(collateral) > 0 {
			return errOptionPoolCannotCover
		}

		// Zero the position first so a replay cannot pay twice.
		if err := tx.Model(&models.OptionPosition{}).Where("id = ?", position.ID).Updates(map[string]interface{}{
			"size":           "0",
			"claimed_payout": payout.String(),
		}).Error; err != nil {
			return err
		}

		// Release the collateral that backed this position and shrink the pool.
		lock, err := optionRequiredCollateral(&lockedSeries, size)
		if err != nil {
			return err
		}
		newLocked := new(big.Int).Sub(seriesBigInt(lockedSeries.LockedCollateral), lock)
		if newLocked.Sign() < 0 {
			newLocked = big.NewInt(0)
		}
		newCollateral := new(big.Int).Sub(collateral, payout)
		if newCollateral.Cmp(newLocked) < 0 {
			newLocked = new(big.Int).Set(newCollateral)
		}
		newTotalLongSize := new(big.Int).Sub(seriesBigInt(lockedSeries.TotalLongSize), size)
		if newTotalLongSize.Sign() < 0 {
			newTotalLongSize = big.NewInt(0)
		}
		if err := tx.Model(&models.OptionSeries{}).Where("id = ?", lockedSeries.ID).Updates(map[string]interface{}{
			"collateral":        newCollateral.String(),
			"locked_collateral": newLocked.String(),
			"total_long_size":   newTotalLongSize.String(),
		}).Error; err != nil {
			return err
		}

		if payout.Sign() > 0 {
			return s.ledger.creditWithin(tx, userID, lockedSeries.ChainID, lockedSeries.CollateralTokenAddress,
				lockedSeries.CollateralTokenSymbol, lockedSeries.CollateralTokenDecimals, payout)
		}
		return nil
	})
	if err != nil {
		switch {
		case errors.Is(err, errSeriesNotOpen):
			c.JSON(http.StatusConflict, gin.H{"error": "series not settled"})
		case errors.Is(err, errOptionNoPosition):
			c.JSON(http.StatusConflict, gin.H{"error": "no position to exercise"})
		case errors.Is(err, errOptionPoolCannotCover):
			c.JSON(http.StatusConflict, gin.H{"error": "the pool cannot cover this payout"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to exercise"})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "payout": payout.String()})
}

// WithdrawLiquidity withdraws an LP's share of the pool's remaining collateral
// after settlement. Collateral still reserved for unexercised positions is not
// withdrawable, so an LP cannot take funds a holder is owed.
func (s *OptionService) WithdrawLiquidity(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid series id"})
		return
	}
	var req collateralRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequestError(c, err)
		return
	}
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}
	if s.ledger == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "ledger is not configured"})
		return
	}
	amount, err := parseOptionAmount(req.Amount)
	if err != nil || amount.Sign() <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "amount must be a positive integer"})
		return
	}

	series, err := s.loadSeries(id)
	if err != nil {
		if errors.Is(err, ErrListingNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "series not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load series"})
		return
	}
	if series.Status != models.OptionStatusSettled {
		c.JSON(http.StatusConflict, gin.H{"error": "series not settled"})
		return
	}

	err = s.db.Transaction(func(tx *gorm.DB) error {
		var lockedSeries models.OptionSeries
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&lockedSeries, "id = ?", series.ID).Error; err != nil {
			return err
		}
		if lockedSeries.Status != models.OptionStatusSettled {
			return errSeriesNotOpen
		}
		claimable := seriesBigInt(lockedSeries.Collateral)
		reserved := seriesBigInt(lockedSeries.LockedCollateral)
		if reserved.Cmp(claimable) > 0 {
			reserved = claimable
		}
		available := new(big.Int).Sub(claimable, reserved)
		if amount.Cmp(available) > 0 {
			return errOptionCollateralReserved
		}

		var liquidity models.OptionLiquidity
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&liquidity, "series_id = ? AND user_id = ?", lockedSeries.ID, userID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errOptionNoLiquidity
			}
			return err
		}
		if amount.Cmp(seriesBigInt(liquidity.Principal)) > 0 {
			return errOptionExceedsPrincipal
		}

		newCollateral := new(big.Int).Sub(claimable, amount)
		newPrincipal := new(big.Int).Sub(seriesBigInt(liquidity.Principal), amount)
		if err := tx.Model(&models.OptionSeries{}).Where("id = ?", lockedSeries.ID).
			Update("collateral", newCollateral.String()).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.OptionLiquidity{}).Where("id = ?", liquidity.ID).
			Update("principal", newPrincipal.String()).Error; err != nil {
			return err
		}
		return s.ledger.creditWithin(tx, userID, lockedSeries.ChainID, lockedSeries.CollateralTokenAddress,
			lockedSeries.CollateralTokenSymbol, lockedSeries.CollateralTokenDecimals, amount)
	})
	if err != nil {
		switch {
		case errors.Is(err, errSeriesNotOpen):
			c.JSON(http.StatusConflict, gin.H{"error": "series not settled"})
		case errors.Is(err, errOptionCollateralReserved),
			errors.Is(err, errOptionExceedsPrincipal),
			errors.Is(err, errOptionNoLiquidity):
			respondServiceError(c, err)
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to withdraw liquidity"})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "amount": amount.String()})
}

// ---- Views: positions ------------------------------------------------------

// PositionView is a holder's position with its live payout estimate.
type PositionView struct {
	Position       models.OptionPosition `json:"position"`
	Series         models.OptionSeries   `json:"series"`
	CurrentPremium string                `json:"current_premium"`
	PayoutEstimate string                `json:"payout_estimate"`
}

// GetMyPositions returns the caller's option positions across every series.
func (s *OptionService) GetMyPositions(c *gin.Context) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}
	positions := []models.OptionPosition{}
	if err := s.db.Where("user_id = ? AND size <> '0'", userID).Order("created_at desc").Find(&positions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list positions"})
		return
	}
	now := time.Now()
	views := make([]PositionView, 0, len(positions))
	for _, position := range positions {
		series, err := s.loadSeries(position.SeriesID)
		if err != nil {
			continue
		}
		view := PositionView{Position: position, Series: *series, CurrentPremium: "0", PayoutEstimate: "0"}
		if premium, err := optionPremium(series, seriesBigInt(position.Size), now); err == nil {
			view.CurrentPremium = formatWad(premium)
		}
		if series.Status == models.OptionStatusSettled {
			if payout, err := optionPayoutAt(series, seriesBigInt(position.Size)); err == nil {
				view.PayoutEstimate = formatWad(payout)
			}
		}
		views = append(views, view)
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "positions": views})
}

// RefreshMarkPrice pulls the current oracle price for a series' underlying and
// stores it. It is admin-only, so an operator can track the real market rather
// than typing a price in by hand.
func (s *OptionService) RefreshMarkPrice(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid series id"})
		return
	}
	series, err := s.loadSeries(id)
	if err != nil {
		if errors.Is(err, ErrListingNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "series not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load series"})
		return
	}
	if s.prices == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "no price source is configured"})
		return
	}
	price, err := s.prices.GetRealPrice(c.Request.Context(), series.UnderlyingSymbol)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "the price source returned an error"})
		return
	}
	if price == nil || price.USD <= 0 {
		c.JSON(http.StatusBadGateway, gin.H{"error": "the price source returned no usable price"})
		return
	}
	markPrice, err := parseWad(strconv.FormatFloat(price.USD, 'f', 18, 64))
	if err != nil || markPrice.Sign() <= 0 {
		c.JSON(http.StatusBadGateway, gin.H{"error": "the price source returned an unusable price"})
		return
	}
	formatted := formatWad(markPrice)
	if err := s.db.Model(&models.OptionSeries{}).Where("id = ?", series.ID).
		Update("mark_price", formatted).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update mark price"})
		return
	}
	logListingAction(c, s.db, "refresh_option_mark_price", "option_series", series.ID.String(), gin.H{"mark_price": formatted})
	updated, err := s.loadSeries(series.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to reload series"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "series": s.buildSeriesView(*updated, time.Now())})
}
