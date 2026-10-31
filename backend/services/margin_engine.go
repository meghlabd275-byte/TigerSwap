package services

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"tigerswap/backend/models"
)

// priceSource is the oracle the margin engine values collateral and debt
// with. *PriceAggregator is the production implementation; the interface
// keeps the engine testable with a deterministic price source.
type priceSource interface {
	GetRealPrice(ctx context.Context, symbol string) (*PriceData, error)
}

// MarginEngine implements spot margin trading and lending on TigerSwap's own
// ledger. Risk parameters (LTV, liquidation threshold, interest curve) come from
// the admin margin listings; a pair that is not listed cannot be traded, so the
// engine can never open a position the risk model was not configured for.
type MarginEngine struct {
	db       *gorm.DB
	priceAgg priceSource
}

// marginMarketConfig is the tradable subset of an admin margin listing.
type marginMarketConfig struct {
	Pair                 string
	BaseAsset            string
	QuoteAsset           string
	MarginMode           string
	MaxLeverage          int
	CollateralAssets     map[string]bool
	BorrowableAssets     map[string]bool
	BaseRateAPY          float64
	KinkUtilization      float64
	JumpMultiplier       float64
	LTV                  float64
	LiquidationThreshold float64
	LiquidationBonus     float64
	CloseFactor          float64
}

// MarginAccountView is the public risk description of a margin account. All
// values are denominated in USD.
type MarginAccountView struct {
	ID                  string  `json:"id"`
	MarginMode          string  `json:"margin_mode"`
	Pair                string  `json:"pair"`
	Status              string  `json:"status"`
	CollateralValueUSD  float64 `json:"collateral_value_usd"`
	DebtValueUSD        float64 `json:"debt_value_usd"`
	EquityUSD           float64 `json:"equity_usd"`
	HealthFactor        float64 `json:"health_factor"`
	LiquidationValueUSD float64 `json:"liquidation_value_usd"`
}

// NewMarginEngine builds a margin engine over the given database.
func NewMarginEngine(db *gorm.DB, priceAgg priceSource) *MarginEngine {
	return &MarginEngine{db: db, priceAgg: priceAgg}
}

// Sentinel errors let handlers map failures to status codes.
var (
	ErrMarginMarketNotListed        = errors.New("margin market is not listed")
	ErrMarginAccountNotFound        = errors.New("margin account not found")
	ErrMarginAssetNotAllowed        = errors.New("asset is not allowed by the listing")
	ErrMarginInsufficientCollateral = errors.New("insufficient collateral for this borrow")
)

// marginConfig loads and validates the listing for a pair. Only active listings
// are tradable; paused, delisting and maintenance listings are rejected.
func (e *MarginEngine) marginConfig(pair string) (*marginMarketConfig, error) {
	normalized := strings.ToUpper(strings.TrimSpace(pair))
	var market models.MarginMarket
	err := e.db.Where("pair = ? AND status = ?", normalized, models.ListingStatusActive).First(&market).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("%w: %s", ErrMarginMarketNotListed, normalized)
	}
	if err != nil {
		return nil, err
	}

	base := strings.ToUpper(strings.TrimSpace(market.BaseAsset))
	quote := strings.ToUpper(strings.TrimSpace(market.QuoteAsset))
	if base == "" || quote == "" {
		return nil, fmt.Errorf("%w: %s has no base/quote", ErrMarginMarketNotListed, normalized)
	}

	return &marginMarketConfig{
		Pair:                 normalized,
		BaseAsset:            base,
		QuoteAsset:           quote,
		MarginMode:           strings.ToLower(strings.TrimSpace(market.MarginMode)),
		MaxLeverage:          market.MaxLeverage,
		CollateralAssets:     assetSet(market.CollateralAssets),
		BorrowableAssets:     assetSet(market.BorrowableAssets),
		BaseRateAPY:          market.BaseRateAPY,
		KinkUtilization:      market.KinkUtilization,
		JumpMultiplier:       market.JumpMultiplier,
		LTV:                  market.LTV,
		LiquidationThreshold: market.LiquidationThreshold,
		LiquidationBonus:     market.LiquidationBonus,
		CloseFactor:          market.CloseFactor,
	}, nil
}

// assetSet parses a comma-separated asset list into a set. A blank entry is
// ignored rather than treated as an asset named "".
func assetSet(raw string) map[string]bool {
	set := make(map[string]bool)
	for _, part := range strings.Split(raw, ",") {
		asset := strings.ToUpper(strings.TrimSpace(part))
		if asset != "" {
			set[asset] = true
		}
	}
	return set
}

// moneyValue parses a caller-supplied amount into an exact rational.
//
// Amounts must be positive plain decimals with no more than eight fractional
// digits, the precision every ledger string column in this engine holds. This
// is deliberately stricter than parseFloat, which parses garbage as zero and
// lets a value below 1e-8 be stored as 0.00000000: a deposit would be accepted
// and the caller's funds would vanish.
func moneyValue(amount string) (*big.Rat, error) {
	value, err := parseDecimal(amount)
	if err != nil {
		return nil, fmt.Errorf("invalid amount %q: %w", amount, err)
	}
	if value.Sign() <= 0 {
		return nil, errors.New("amount must be positive")
	}
	trimmed := strings.TrimSpace(amount)
	if dot := strings.IndexByte(trimmed, '.'); dot >= 0 && len(trimmed)-dot-1 > 8 {
		return nil, fmt.Errorf("amount %q has more than 8 decimal places", amount)
	}
	return value, nil
}

// amountValueString renders a computed amount with the eight fractional digits
// the storage columns hold, truncating toward zero. Caller-supplied amounts are
// validated by moneyValue before they reach here; this function only rounds
// engine-computed values (interest, liquidation remainders) to stored precision.
func amountValueString(value *big.Rat) string {
	return formatDecimal(value, 8)
}

// rationalFromFloat converts a float64 the engine derived from stored risk
// parameters (an oracle price, an LTV, a growth factor) into an exact rational.
//
// SetFloat64 would capture the binary representation exactly, so 0.7 would
// become 0.69999999999999995559 and a borrow sized precisely to the LTV would
// be rejected. Going through the shortest decimal form keeps the comparison the
// user expects while staying exact from that point on.
func rationalFromFloat(value float64) (*big.Rat, error) {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return nil, errors.New("value is not a finite number")
	}
	return parseDecimal(strconv.FormatFloat(value, 'f', -1, 64))
}

// assetPriceUSD returns the oracle price of an asset in USD. A zero price or an
// oracle failure is an error: the engine must never value collateral or debt at
// a fabricated price.
func (e *MarginEngine) assetPriceUSD(ctx context.Context, asset string) (float64, error) {
	if e.priceAgg == nil {
		return 0, errors.New("margin engine has no price aggregator")
	}
	price, err := e.priceAgg.GetRealPrice(ctx, strings.ToUpper(strings.TrimSpace(asset)))
	if err != nil {
		return 0, err
	}
	if price.USD <= 0 {
		return 0, fmt.Errorf("oracle returned a non-positive price for %s", asset)
	}
	return price.USD, nil
}

// marginHealth is the risk state of an account.
type marginHealth struct {
	CollateralUSD       float64
	DebtUSD             float64
	LiquidationValueUSD float64
	HealthFactor        float64
}

// computeHealth values an account's collateral and debt and derives the health
// factor. A debt-free account has an infinite health factor.
func (e *MarginEngine) computeHealth(ctx context.Context, accountID uuid.UUID, cfg *marginMarketConfig) (*marginHealth, error) {
	var collaterals []models.MarginCollateral
	if err := e.db.Where("account_id = ?", accountID).Find(&collaterals).Error; err != nil {
		return nil, err
	}
	var borrows []models.MarginBorrow
	if err := e.db.Where("account_id = ?", accountID).Find(&borrows).Error; err != nil {
		return nil, err
	}

	health := &marginHealth{}
	for i := range collaterals {
		amount, err := parseDecimal(collaterals[i].Amount)
		if err != nil {
			return nil, fmt.Errorf("collateral %s holds an unreadable amount %q: %w", collaterals[i].Asset, collaterals[i].Amount, err)
		}
		if amount.Sign() <= 0 {
			continue
		}
		price, err := e.assetPriceUSD(ctx, collaterals[i].Asset)
		if err != nil {
			return nil, err
		}
		usd, err := rationalFromFloat(price)
		if err != nil {
			return nil, fmt.Errorf("oracle price for %s is unusable: %w", collaterals[i].Asset, err)
		}
		collateralUSD, _ := new(big.Rat).Mul(amount, usd).Float64()
		health.CollateralUSD += collateralUSD
	}
	for i := range borrows {
		principal, err := parseDecimal(borrows[i].Principal)
		if err != nil {
			return nil, fmt.Errorf("borrow %s holds an unreadable principal %q: %w", borrows[i].Asset, borrows[i].Principal, err)
		}
		interest, err := parseDecimal(borrows[i].AccruedInterest)
		if err != nil {
			return nil, fmt.Errorf("borrow %s holds unreadable interest %q: %w", borrows[i].Asset, borrows[i].AccruedInterest, err)
		}
		total := new(big.Rat).Add(principal, interest)
		if total.Sign() <= 0 {
			continue
		}
		price, err := e.assetPriceUSD(ctx, borrows[i].Asset)
		if err != nil {
			return nil, err
		}
		usd, err := rationalFromFloat(price)
		if err != nil {
			return nil, fmt.Errorf("oracle price for %s is unusable: %w", borrows[i].Asset, err)
		}
		debtUSD, _ := new(big.Rat).Mul(total, usd).Float64()
		health.DebtUSD += debtUSD
	}

	health.LiquidationValueUSD = health.CollateralUSD * cfg.LiquidationThreshold
	if health.DebtUSD <= 0 {
		health.HealthFactor = math.Inf(1)
	} else {
		health.HealthFactor = health.LiquidationValueUSD / health.DebtUSD
	}
	return health, nil
}

// borrowRateAPY applies the listing's two-slope curve: the base rate up to the
// kink utilization, then the base rate plus the jump multiplier beyond it. This
// is what makes borrowing expensive as a pool is drained.
func borrowRateAPY(cfg *marginMarketConfig, utilization float64) float64 {
	if utilization < 0 {
		utilization = 0
	}
	if utilization > 1 {
		utilization = 1
	}
	rate := cfg.BaseRateAPY
	if utilization > cfg.KinkUtilization {
		over := utilization - cfg.KinkUtilization
		// Scale the jump across the remaining range so the rate is continuous at
		// the kink and reaches base+jump at full utilization.
		if denom := 1 - cfg.KinkUtilization; denom > 0 {
			rate += cfg.JumpMultiplier * (over / denom)
		}
	}
	return rate
}

// accrueAccountInterest brings every borrow in an account up to now. Interest
// compounds at the listing's base rate. The jump slope applies once pool
// utilization is tracked per market, so the base rate is the honest rate here
// rather than a fabricated utilization.
func (e *MarginEngine) accrueAccountInterest(accountID uuid.UUID, cfg *marginMarketConfig, now time.Time) error {
	var borrows []models.MarginBorrow
	if err := e.db.Where("account_id = ?", accountID).Find(&borrows).Error; err != nil {
		return err
	}

	rate := borrowRateAPY(cfg, 0)
	for i := range borrows {
		elapsed := now.Sub(borrows[i].LastAccrual)
		if elapsed <= 0 {
			continue
		}
		principal, err := parseDecimal(borrows[i].Principal)
		if err != nil {
			return fmt.Errorf("borrow %s holds an unreadable principal %q: %w", borrows[i].Asset, borrows[i].Principal, err)
		}
		interest, err := parseDecimal(borrows[i].AccruedInterest)
		if err != nil {
			return fmt.Errorf("borrow %s holds unreadable interest %q: %w", borrows[i].Asset, borrows[i].AccruedInterest, err)
		}
		days := elapsed.Hours() / 24
		growth := math.Pow(1+rate/365, days)
		// Interest is charged on principal plus interest already accrued, so the
		// stored interest compounds exactly rather than drifting through float64.
		growthRat, err := rationalFromFloat(growth)
		if err != nil {
			return fmt.Errorf("interest growth factor is unusable: %w", err)
		}
		total := new(big.Rat).Mul(new(big.Rat).Add(principal, interest), growthRat)
		newInterest := new(big.Rat).Sub(total, principal)
		if newInterest.Sign() < 0 {
			newInterest = new(big.Rat)
		}
		storedInterest := amountValueString(newInterest)
		if err := e.db.Model(&models.MarginBorrow{}).Where("id = ?", borrows[i].ID).
			Updates(map[string]interface{}{
				"accrued_interest": storedInterest,
				"last_accrual":     now,
			}).Error; err != nil {
			return err
		}
	}
	return nil
}

// OpenAccountRequest is the payload accepted by OpenMarginAccountHandler.
type OpenAccountRequest struct {
	Pair       string `json:"pair" binding:"required"`
	MarginMode string `json:"margin_mode"`
}

// OpenAccount creates a margin account for the authenticated caller.
func (e *MarginEngine) OpenAccount(_ context.Context, userID string, req OpenAccountRequest) (*models.MarginAccount, error) {
	cfg, err := e.marginConfig(req.Pair)
	if err != nil {
		return nil, err
	}

	mode := strings.ToLower(strings.TrimSpace(req.MarginMode))
	if mode == "" {
		mode = cfg.MarginMode
	}
	if mode == "" {
		mode = "cross"
	}
	if mode != "cross" && mode != "isolated" {
		return nil, errors.New("margin mode must be cross or isolated")
	}

	account := &models.MarginAccount{
		UserID:     userID,
		MarginMode: mode,
		Pair:       cfg.Pair,
		Status:     models.MarginAccountActive,
	}
	if err := e.db.Create(account).Error; err != nil {
		return nil, err
	}
	return account, nil
}

// DepositCollateral adds collateral to an account the caller owns.
func (e *MarginEngine) DepositCollateral(ctx context.Context, userID string, accountID uuid.UUID, asset, amount string) error {
	cfg, _, err := e.ownedAccount(userID, accountID)
	if err != nil {
		return err
	}

	normalized := strings.ToUpper(strings.TrimSpace(asset))
	if !cfg.CollateralAssets[normalized] {
		return fmt.Errorf("%w: %s cannot be used as collateral for %s", ErrMarginAssetNotAllowed, normalized, cfg.Pair)
	}

	value, err := moneyValue(amount)
	if err != nil {
		return err
	}
	if _, err := e.assetPriceUSD(ctx, normalized); err != nil {
		return fmt.Errorf("cannot price collateral %s: %w", normalized, err)
	}

	var existing models.MarginCollateral
	err = e.db.Where("account_id = ? AND asset = ?", accountID, normalized).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		stored := amountValueString(value)
		return e.db.Create(&models.MarginCollateral{
			AccountID: accountID,
			Asset:     normalized,
			Amount:    stored,
		}).Error
	}
	if err != nil {
		return err
	}

	current, err := parseDecimal(existing.Amount)
	if err != nil {
		return fmt.Errorf("stored collateral for %s is unreadable: %w", normalized, err)
	}
	stored := amountValueString(new(big.Rat).Add(current, value))
	return e.db.Model(&models.MarginCollateral{}).Where("id = ?", existing.ID).
		Update("amount", stored).Error
}

// Borrow borrows an asset against the account's collateral, enforcing the
// listing's LTV. Interest starts accruing immediately.
func (e *MarginEngine) Borrow(ctx context.Context, userID string, accountID uuid.UUID, asset, amount string) error {
	cfg, _, err := e.ownedAccount(userID, accountID)
	if err != nil {
		return err
	}

	normalized := strings.ToUpper(strings.TrimSpace(asset))
	if !cfg.BorrowableAssets[normalized] {
		return fmt.Errorf("%w: %s is not borrowable for %s", ErrMarginAssetNotAllowed, normalized, cfg.Pair)
	}

	value, err := moneyValue(amount)
	if err != nil {
		return err
	}
	borrowPrice, err := e.assetPriceUSD(ctx, normalized)
	if err != nil {
		return fmt.Errorf("cannot price borrow asset %s: %w", normalized, err)
	}

	health, err := e.computeHealth(ctx, accountID, cfg)
	if err != nil {
		return err
	}
	// New debt may not exceed LTV * collateral. LTV is stricter than the
	// liquidation threshold, so a fresh borrow leaves a safety buffer. The
	// comparison is exact, so a borrow that only fits because of binary
	// rounding is refused.
	priceRat, err := rationalFromFloat(borrowPrice)
	if err != nil {
		return fmt.Errorf("oracle price for %s is unusable: %w", normalized, err)
	}
	ltv, err := rationalFromFloat(cfg.LTV)
	if err != nil {
		return fmt.Errorf("listing LTV for %s is unusable: %w", cfg.Pair, err)
	}
	collateralUSD, err := rationalFromFloat(health.CollateralUSD)
	if err != nil {
		return fmt.Errorf("collateral value is unusable: %w", err)
	}
	debtUSD, err := rationalFromFloat(health.DebtUSD)
	if err != nil {
		return fmt.Errorf("debt value is unusable: %w", err)
	}
	maxDebt := new(big.Rat).Mul(collateralUSD, ltv)
	debt := new(big.Rat).Add(debtUSD, new(big.Rat).Mul(value, priceRat))
	if debt.Cmp(maxDebt) > 0 {
		f, _ := value.Float64()
		return fmt.Errorf("%w: borrowing %.8f %s would exceed the LTV of %.2f", ErrMarginInsufficientCollateral, f, normalized, cfg.LTV)
	}

	now := time.Now().UTC()
	var existing models.MarginBorrow
	err = e.db.Where("account_id = ? AND asset = ?", accountID, normalized).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		stored := amountValueString(value)
		return e.db.Create(&models.MarginBorrow{
			AccountID:       accountID,
			Asset:           normalized,
			Principal:       stored,
			AccruedInterest: "0",
			LastAccrual:     now,
		}).Error
	}
	if err != nil {
		return err
	}

	// Settle interest first so the new principal does not earn interest for the
	// time before this borrow.
	principal, err := parseDecimal(existing.Principal)
	if err != nil {
		return fmt.Errorf("stored principal for %s is unreadable: %w", normalized, err)
	}
	interest, err := parseDecimal(existing.AccruedInterest)
	if err != nil {
		return fmt.Errorf("stored interest for %s is unreadable: %w", normalized, err)
	}
	if elapsed := now.Sub(existing.LastAccrual); elapsed > 0 {
		rate := borrowRateAPY(cfg, 0)
		days := elapsed.Hours() / 24
		growth := math.Pow(1+rate/365, days)
		growthRat, err := rationalFromFloat(growth)
		if err != nil {
			return fmt.Errorf("interest growth factor is unusable: %w", err)
		}
		accrued := new(big.Rat).Mul(principal, new(big.Rat).Sub(growthRat, big.NewRat(1, 1)))
		interest = new(big.Rat).Add(interest, accrued)
	}
	total := new(big.Rat).Add(principal, value)
	newPrincipal := amountValueString(total)
	if interest.Sign() < 0 {
		interest = new(big.Rat)
	}
	newInterest := amountValueString(interest)
	return e.db.Model(&models.MarginBorrow{}).Where("id = ?", existing.ID).
		Updates(map[string]interface{}{
			"principal":        newPrincipal,
			"accrued_interest": newInterest,
			"last_accrual":     now,
		}).Error
}

// Repay reduces an account's debt. Repaying more than is owed is rejected
// rather than silently capped, so a caller cannot lose funds to a rounding
// mistake.
func (e *MarginEngine) Repay(_ context.Context, userID string, accountID uuid.UUID, asset, amount string) error {
	_, _, err := e.ownedAccount(userID, accountID)
	if err != nil {
		return err
	}

	normalized := strings.ToUpper(strings.TrimSpace(asset))
	value, err := moneyValue(amount)
	if err != nil {
		return err
	}

	var borrow models.MarginBorrow
	err = e.db.Where("account_id = ? AND asset = ?", accountID, normalized).First(&borrow).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return errors.New("no debt in this asset")
	}
	if err != nil {
		return err
	}

	principal, err := parseDecimal(borrow.Principal)
	if err != nil {
		return fmt.Errorf("stored principal for %s is unreadable: %w", normalized, err)
	}
	interest, err := parseDecimal(borrow.AccruedInterest)
	if err != nil {
		return fmt.Errorf("stored interest for %s is unreadable: %w", normalized, err)
	}
	owed := new(big.Rat).Add(principal, interest)
	if value.Cmp(owed) > 0 {
		f, _ := value.Float64()
		owedF, _ := owed.Float64()
		return fmt.Errorf("repay amount %.8f exceeds debt %.8f", f, owedF)
	}
	if value.Cmp(owed) == 0 {
		return e.db.Delete(&models.MarginBorrow{}, "id = ?", borrow.ID).Error
	}

	// Repayment clears interest before principal.
	var newInterest, newPrincipal *big.Rat
	if value.Cmp(interest) <= 0 {
		newInterest = new(big.Rat).Sub(interest, value)
		newPrincipal = principal
	} else {
		newInterest = new(big.Rat)
		newPrincipal = new(big.Rat).Sub(principal, new(big.Rat).Sub(value, interest))
	}
	if newPrincipal.Sign() < 0 {
		newPrincipal = new(big.Rat)
	}
	storedPrincipal := amountValueString(newPrincipal)
	storedInterest := amountValueString(newInterest)
	return e.db.Model(&models.MarginBorrow{}).Where("id = ?", borrow.ID).
		Updates(map[string]interface{}{
			"principal":        storedPrincipal,
			"accrued_interest": storedInterest,
		}).Error
}

// LiquidationCandidate describes an account that can be liquidated.
type LiquidationCandidate struct {
	AccountID    string  `json:"account_id"`
	HealthFactor float64 `json:"health_factor"`
	DebtUSD      float64 `json:"debt_usd"`
}

// LiquidatableAccounts returns every active account whose health factor is
// below 1, so a liquidation bot can act on it.
func (e *MarginEngine) LiquidatableAccounts(ctx context.Context) ([]LiquidationCandidate, error) {
	var accounts []models.MarginAccount
	if err := e.db.Where("status = ?", models.MarginAccountActive).Find(&accounts).Error; err != nil {
		return nil, err
	}

	var candidates []LiquidationCandidate
	for i := range accounts {
		cfg, err := e.marginConfig(accounts[i].Pair)
		if err != nil {
			// An unlisted pair cannot be risk-assessed; skip it rather than
			// liquidate on incomplete data.
			continue
		}
		// Accrue first: an idle account's stored interest can lag, and the
		// liquidation decision must use the debt actually owed.
		if err := e.accrueAccountInterest(accounts[i].ID, cfg, time.Now().UTC()); err != nil {
			continue
		}
		health, err := e.computeHealth(ctx, accounts[i].ID, cfg)
		if err != nil {
			continue
		}
		if health.DebtUSD > 0 && health.HealthFactor < 1 {
			candidates = append(candidates, LiquidationCandidate{
				AccountID:    accounts[i].ID.String(),
				HealthFactor: health.HealthFactor,
				DebtUSD:      health.DebtUSD,
			})
		}
	}
	return candidates, nil
}

// Liquidate seizes collateral from an under-collateralized account. The debt is
// repaid up to the listing's close factor, and the liquidator receives the
// seized collateral plus the liquidation bonus.
func (e *MarginEngine) Liquidate(ctx context.Context, accountID uuid.UUID) (*LiquidationCandidate, error) {
	var account models.MarginAccount
	if err := e.db.First(&account, "id = ?", accountID).Error; err != nil {
		return nil, fmt.Errorf("%w: %s", ErrMarginAccountNotFound, accountID)
	}
	cfg, err := e.marginConfig(account.Pair)
	if err != nil {
		return nil, err
	}
	if err := e.accrueAccountInterest(accountID, cfg, time.Now().UTC()); err != nil {
		return nil, err
	}
	health, err := e.computeHealth(ctx, accountID, cfg)
	if err != nil {
		return nil, err
	}
	if health.DebtUSD <= 0 || health.HealthFactor >= 1 {
		return nil, errors.New("account is not liquidatable")
	}

	// The close factor caps how much of the debt one liquidation may repay,
	// which keeps a large account from being closed in a single step.
	closeFactor := cfg.CloseFactor
	if closeFactor <= 0 || closeFactor > 1 {
		closeFactor = 1
	}
	repayable := health.DebtUSD * closeFactor
	seizeValue := repayable * (1 + cfg.LiquidationBonus)
	if seizeValue > health.CollateralUSD {
		seizeValue = health.CollateralUSD
	}

	if err := e.seizeCollateral(ctx, accountID, seizeValue); err != nil {
		return nil, err
	}
	if err := e.reduceDebt(ctx, accountID, repayable); err != nil {
		return nil, err
	}

	// Mark the account liquidated once its debt is cleared.
	if remaining, err := e.computeHealth(ctx, accountID, cfg); err == nil && remaining.DebtUSD <= 1e-9 {
		if err := e.db.Model(&models.MarginAccount{}).Where("id = ?", accountID).
			Update("status", models.MarginAccountLiquidated).Error; err != nil {
			return nil, err
		}
	}

	return &LiquidationCandidate{AccountID: accountID.String(), HealthFactor: health.HealthFactor, DebtUSD: health.DebtUSD}, nil
}

// seizeCollateral removes collateral worth valueUSD, proportionally across the
// account's holdings.
func (e *MarginEngine) seizeCollateral(ctx context.Context, accountID uuid.UUID, valueUSD float64) error {
	var collaterals []models.MarginCollateral
	if err := e.db.Where("account_id = ?", accountID).Find(&collaterals).Error; err != nil {
		return err
	}

	type holding struct {
		row models.MarginCollateral
		usd float64
	}
	holdings := make([]holding, 0, len(collaterals))
	var totalUSD float64
	for i := range collaterals {
		amount, err := parseDecimal(collaterals[i].Amount)
		if err != nil {
			return fmt.Errorf("collateral %s holds an unreadable amount %q: %w", collaterals[i].Asset, collaterals[i].Amount, err)
		}
		if amount.Sign() <= 0 {
			continue
		}
		price, err := e.assetPriceUSD(ctx, collaterals[i].Asset)
		if err != nil {
			return err
		}
		priceRat, err := rationalFromFloat(price)
		if err != nil {
			return fmt.Errorf("oracle price for %s is unusable: %w", collaterals[i].Asset, err)
		}
		usdRat := new(big.Rat).Mul(amount, priceRat)
		usd, _ := usdRat.Float64()
		totalUSD += usd
		holdings = append(holdings, holding{row: collaterals[i], usd: usd})
	}
	if totalUSD <= 0 {
		return nil
	}

	fraction := valueUSD / totalUSD
	if fraction > 1 {
		fraction = 1
	}
	// Seize proportionally: each holding loses `fraction` of its balance. The
	// remainder is computed exactly so the sum of the kept balances matches the
	// account's collateral rather than drifting with binary rounding.
	fractionRat, err := rationalFromFloat(fraction)
	if err != nil {
		return fmt.Errorf("collateral seize fraction is unusable: %w", err)
	}
	keptFraction := new(big.Rat).Sub(big.NewRat(1, 1), fractionRat)
	for _, h := range holdings {
		amount, err := parseDecimal(h.row.Amount)
		if err != nil {
			return fmt.Errorf("collateral %s holds an unreadable amount %q: %w", h.row.Asset, h.row.Amount, err)
		}
		remaining := new(big.Rat).Mul(amount, keptFraction)
		if remaining.Sign() < 0 {
			remaining = new(big.Rat)
		}
		stored := amountValueString(remaining)
		if err := e.db.Model(&models.MarginCollateral{}).Where("id = ?", h.row.ID).
			Update("amount", stored).Error; err != nil {
			return err
		}
	}
	return nil
}

// reduceDebt reduces an account's debt by valueUSD, clearing interest first.
func (e *MarginEngine) reduceDebt(ctx context.Context, accountID uuid.UUID, valueUSD float64) error {
	var borrows []models.MarginBorrow
	if err := e.db.Where("account_id = ?", accountID).Find(&borrows).Error; err != nil {
		return err
	}
	remaining := valueUSD
	for i := range borrows {
		if remaining <= 0 {
			break
		}
		price, err := e.assetPriceUSD(ctx, borrows[i].Asset)
		if err != nil {
			return err
		}
		principal, err := parseDecimal(borrows[i].Principal)
		if err != nil {
			return fmt.Errorf("borrow %s holds an unreadable principal %q: %w", borrows[i].Asset, borrows[i].Principal, err)
		}
		interest, err := parseDecimal(borrows[i].AccruedInterest)
		if err != nil {
			return fmt.Errorf("borrow %s holds unreadable interest %q: %w", borrows[i].Asset, borrows[i].AccruedInterest, err)
		}
		priceRat, err := rationalFromFloat(price)
		if err != nil {
			return fmt.Errorf("oracle price for %s is unusable: %w", borrows[i].Asset, err)
		}
		owed := new(big.Rat).Mul(new(big.Rat).Add(principal, interest), priceRat)
		if owed.Sign() <= 0 {
			continue
		}
		// Pay the lesser of what is left to repay and what this borrow owes.
		remainingRat, err := rationalFromFloat(remaining)
		if err != nil {
			return fmt.Errorf("remaining liquidation value is unusable: %w", err)
		}
		payUSD := remainingRat
		if owed.Cmp(payUSD) < 0 {
			payUSD = owed
		}
		payAmount := new(big.Rat).Quo(payUSD, priceRat)

		var newInterest, newPrincipal *big.Rat
		if payAmount.Cmp(interest) <= 0 {
			newInterest = new(big.Rat).Sub(interest, payAmount)
			newPrincipal = principal
		} else {
			newInterest = new(big.Rat)
			newPrincipal = new(big.Rat).Sub(principal, new(big.Rat).Sub(payAmount, interest))
		}
		if newPrincipal.Sign() < 0 {
			newPrincipal = new(big.Rat)
		}

		if newPrincipal.Sign() == 0 && newInterest.Sign() == 0 {
			if err := e.db.Delete(&models.MarginBorrow{}, "id = ?", borrows[i].ID).Error; err != nil {
				return err
			}
		} else {
			storedPrincipal := amountValueString(newPrincipal)
			storedInterest := amountValueString(newInterest)
			if err := e.db.Model(&models.MarginBorrow{}).Where("id = ?", borrows[i].ID).
				Updates(map[string]interface{}{
					"principal":        storedPrincipal,
					"accrued_interest": storedInterest,
				}).Error; err != nil {
				return err
			}
		}
		paid, _ := payUSD.Float64()
		remaining -= paid
	}
	return nil
}

// ownedAccount loads an account, enforcing that it belongs to the caller.
func (e *MarginEngine) ownedAccount(userID string, accountID uuid.UUID) (*marginMarketConfig, *models.MarginAccount, error) {
	var account models.MarginAccount
	if err := e.db.Where("id = ? AND user_id = ?", accountID, userID).First(&account).Error; err != nil {
		return nil, nil, fmt.Errorf("%w: %s", ErrMarginAccountNotFound, accountID)
	}
	if account.Status != models.MarginAccountActive {
		return nil, nil, fmt.Errorf("margin account is %s", account.Status)
	}
	cfg, err := e.marginConfig(account.Pair)
	if err != nil {
		return nil, nil, err
	}
	return cfg, &account, nil
}

// GetAccountView returns the risk view of an account the caller owns.
func (e *MarginEngine) GetAccountView(ctx context.Context, userID string, accountID uuid.UUID) (*MarginAccountView, error) {
	cfg, account, err := e.ownedAccount(userID, accountID)
	if err != nil {
		return nil, err
	}
	// Bring interest up to date so the reported health factor reflects the debt
	// the account actually owes right now.
	if err := e.accrueAccountInterest(accountID, cfg, time.Now().UTC()); err != nil {
		return nil, err
	}
	health, err := e.computeHealth(ctx, accountID, cfg)
	if err != nil {
		return nil, err
	}
	return &MarginAccountView{
		ID:                  account.ID.String(),
		MarginMode:          account.MarginMode,
		Pair:                account.Pair,
		Status:              account.Status,
		CollateralValueUSD:  health.CollateralUSD,
		DebtValueUSD:        health.DebtUSD,
		EquityUSD:           health.CollateralUSD - health.DebtUSD,
		HealthFactor:        health.HealthFactor,
		LiquidationValueUSD: health.LiquidationValueUSD,
	}, nil
}

// --- HTTP handlers ---

// OpenMarginAccountHandler creates a margin account for the authenticated user.
func (e *MarginEngine) OpenMarginAccountHandler(c *gin.Context) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}
	var req OpenAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	account, err := e.OpenAccount(c.Request.Context(), userID, req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, account)
}

// GetMarginAccountHandler returns the risk view of an owned account.
func (e *MarginEngine) GetMarginAccountHandler(c *gin.Context) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}
	accountID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid account id"})
		return
	}
	view, err := e.GetAccountView(c.Request.Context(), userID, accountID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, view)
}

// DepositMarginCollateralHandler adds collateral to an owned account.
func (e *MarginEngine) DepositMarginCollateralHandler(c *gin.Context) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}
	var req struct {
		AccountID string `json:"account_id" binding:"required"`
		Asset     string `json:"asset" binding:"required"`
		Amount    string `json:"amount" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	accountID, err := uuid.Parse(req.AccountID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid account id"})
		return
	}
	if err := e.DepositCollateral(c.Request.Context(), userID, accountID, req.Asset, req.Amount); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// BorrowMarginHandler borrows against an owned account's collateral.
func (e *MarginEngine) BorrowMarginHandler(c *gin.Context) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}
	var req struct {
		AccountID string `json:"account_id" binding:"required"`
		Asset     string `json:"asset" binding:"required"`
		Amount    string `json:"amount" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	accountID, err := uuid.Parse(req.AccountID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid account id"})
		return
	}
	if err := e.Borrow(c.Request.Context(), userID, accountID, req.Asset, req.Amount); err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, ErrMarginInsufficientCollateral) {
			status = http.StatusUnprocessableEntity
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// RepayMarginHandler repays an owned account's debt.
func (e *MarginEngine) RepayMarginHandler(c *gin.Context) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}
	var req struct {
		AccountID string `json:"account_id" binding:"required"`
		Asset     string `json:"asset" binding:"required"`
		Amount    string `json:"amount" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	accountID, err := uuid.Parse(req.AccountID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid account id"})
		return
	}
	if err := e.Repay(c.Request.Context(), userID, accountID, req.Asset, req.Amount); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ListLiquidatableHandler lists accounts eligible for liquidation. It is an
// operator endpoint: the caller's role is enforced by the route middleware.
func (e *MarginEngine) ListLiquidatableHandler(c *gin.Context) {
	candidates, err := e.LiquidatableAccounts(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"accounts": candidates, "count": len(candidates)})
}
