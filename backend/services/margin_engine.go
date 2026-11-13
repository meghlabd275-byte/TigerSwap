package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
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

	// ledger, when set, holds every asset the engine moves: a collateral
	// deposit debits the trader's balance, a borrow credits it from the pool,
	// a repayment returns it, and a liquidation moves it between the liquidator
	// and the pool. Without it the engine refuses to move funds.
	ledger            *LedgerService
	settlementChainID int64
	// assetResolver names the ledger asset for a symbol. It is a field so a test
	// can supply a fixed registry; production wires the config-backed resolver.
	assetResolver func(asset string) (perpSettlement, error)
}

// marginMarketConfig is the tradable subset of an admin margin listing.
type marginMarketConfig struct {
	Pair             string
	BaseAsset        string
	QuoteAsset       string
	MarginMode       string
	MaxLeverage      int
	CollateralAssets map[string]bool
	BorrowableAssets map[string]bool
	BaseRateAPY      float64
	KinkUtilization  float64
	JumpMultiplier   float64
	// Risk ratios are exact rationals. A liquidation decision compares the
	// health factor against 1, and a float64 threshold can round at the
	// boundary; the ratios are parsed once from the listing and kept exact.
	LTV                  *big.Rat
	LiquidationThreshold *big.Rat
	LiquidationBonus     *big.Rat
	CloseFactor          *big.Rat
}

// MarginAccountView is the public risk description of a margin account. All
// values are denominated in USD.
type MarginAccountView struct {
	ID         string `json:"id"`
	MarginMode string `json:"margin_mode"`
	Pair       string `json:"pair"`
	Status     string `json:"status"`
	// HealthFactor is a decimal string. A debt-free account has no finite health
	// factor and is reported as "Infinity"; a float64 field cannot carry that
	// value through encoding/json, which fails the whole response, so the view
	// is only readable for an account that already has debt.
	CollateralValueUSD  float64 `json:"collateral_value_usd"`
	DebtValueUSD        float64 `json:"debt_value_usd"`
	EquityUSD           float64 `json:"equity_usd"`
	HealthFactor        string  `json:"health_factor"`
	LiquidationValueUSD float64 `json:"liquidation_value_usd"`
}

// NewMarginEngine builds a margin engine over the given database.
func NewMarginEngine(db *gorm.DB, priceAgg priceSource) *MarginEngine {
	return &MarginEngine{db: db, priceAgg: priceAgg}
}

// SetLedger wires the platform ledger that funds collateral, borrows and
// repayments, and the chain whose token registry names an asset's ledger entry.
// With it set, a collateral deposit debits real balance and a borrow credits it.
// Without it the engine refuses to move funds rather than crediting a balance
// out of nothing.
func (e *MarginEngine) SetLedger(ledger *LedgerService, settlementChainID int64) {
	e.ledger = ledger
	e.settlementChainID = settlementChainID
	e.assetResolver = resolveAssetFromConfig(settlementChainID)
}

// SetAssetResolver overrides how an asset symbol is resolved to a ledger entry.
// Tests use it to supply a fixed registry; production leaves the config-backed
// resolver wired by SetLedger.
func (e *MarginEngine) SetAssetResolver(resolver func(asset string) (perpSettlement, error)) {
	e.assetResolver = resolver
}

// assetLedger resolves the ledger entry for an asset symbol. It requires both a
// ledger and a resolver, so a deployment that forgets either refuses to move
// funds rather than moving them in a guessed asset.
func (e *MarginEngine) assetLedger(asset string) (perpSettlement, error) {
	if e.ledger == nil {
		return perpSettlement{}, errors.New("margin engine has no ledger configured; funds cannot be moved")
	}
	if e.assetResolver == nil {
		return perpSettlement{}, fmt.Errorf("no ledger asset resolver configured for %s", asset)
	}
	resolved, err := e.assetResolver(asset)
	if err != nil {
		return perpSettlement{}, err
	}
	if resolved.Symbol == "" || resolved.Address == "" {
		return perpSettlement{}, fmt.Errorf("asset %s has no resolvable ledger entry", asset)
	}
	if resolved.Decimals < 0 || resolved.Decimals > 36 {
		return perpSettlement{}, fmt.Errorf("asset %s has invalid decimals %d", resolved.Symbol, resolved.Decimals)
	}
	return resolved, nil
}

// Sentinel errors let handlers map failures to status codes.
var (
	ErrMarginMarketNotListed        = errors.New("margin market is not listed")
	ErrMarginAccountNotFound        = errors.New("margin account not found")
	ErrMarginAssetNotAllowed        = errors.New("asset is not allowed by the listing")
	ErrMarginInsufficientCollateral = errors.New("insufficient collateral for this borrow")
	ErrMarginNotLiquidatable        = errors.New("margin account is not liquidatable")
	ErrMarginInvalidMode            = errors.New("margin mode must be cross or isolated")
	ErrMarginNoDebt                 = errors.New("no debt in this asset")
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
		LTV:                  ratioFromFloat(market.LTV),
		LiquidationThreshold: ratioFromFloat(market.LiquidationThreshold),
		LiquidationBonus:     ratioFromFloat(market.LiquidationBonus),
		CloseFactor:          ratioFromFloat(market.CloseFactor),
	}, nil
}

// ratioFromFloat parses a stored risk ratio (LTV, liquidation threshold, bonus,
// close factor) into an exact rational once, so every comparison and product
// below runs in exact arithmetic. A stored value that cannot be parsed yields
// zero, which fails closed: a zero LTV refuses a borrow and a zero liquidation
// threshold makes a position liquidatable, never the other way round.
func ratioFromFloat(value float64) *big.Rat {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
		return new(big.Rat)
	}
	rat, err := parseDecimal(strconv.FormatFloat(value, 'f', -1, 64))
	if err != nil {
		return new(big.Rat)
	}
	return rat
}

// bonusBps renders a liquidation bonus ratio as basis points, truncating toward
// zero. A 1% bonus is 100 bps.
func bonusBps(bonus *big.Rat) int64 {
	if bonus == nil {
		return 0
	}
	bps := new(big.Rat).Mul(bonus, big.NewRat(10_000, 1))
	return new(big.Int).Quo(bps.Num(), bps.Denom()).Int64()
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

// marginHealth is the exact risk state of an account. Every value is a rational
// so a liquidation comparison is never decided by a binary-rounding artefact.
type marginHealth struct {
	CollateralUSD       *big.Rat
	DebtUSD             *big.Rat
	LiquidationValueUSD *big.Rat
	// HealthFactor is the liquidation value over debt. It is nil for a debt-free
	// account, whose health factor is infinite.
	HealthFactor *big.Rat
}

// assetPriceRat returns the oracle price of an asset as an exact rational. A
// zero price or an oracle failure is an error: the engine must never value
// collateral or debt at a fabricated price.
func (e *MarginEngine) assetPriceRat(ctx context.Context, asset string) (*big.Rat, error) {
	if e.priceAgg == nil {
		return nil, errors.New("margin engine has no price aggregator")
	}
	price, err := e.priceAgg.GetRealPrice(ctx, strings.ToUpper(strings.TrimSpace(asset)))
	if err != nil {
		return nil, err
	}
	if price.USD <= 0 {
		return nil, fmt.Errorf("oracle returned a non-positive price for %s", asset)
	}
	return rationalFromFloat(price.USD)
}

// computeHealth values an account's collateral and debt and derives the health
// factor, entirely in exact rational arithmetic.
func (e *MarginEngine) computeHealth(ctx context.Context, db *gorm.DB, accountID uuid.UUID, cfg *marginMarketConfig) (*marginHealth, error) {
	var collaterals []models.MarginCollateral
	if err := db.Where("account_id = ?", accountID).Find(&collaterals).Error; err != nil {
		return nil, err
	}
	var borrows []models.MarginBorrow
	if err := db.Where("account_id = ?", accountID).Find(&borrows).Error; err != nil {
		return nil, err
	}

	health := &marginHealth{
		CollateralUSD: new(big.Rat),
		DebtUSD:       new(big.Rat),
	}
	for i := range collaterals {
		amount, err := parseDecimal(collaterals[i].Amount)
		if err != nil {
			return nil, fmt.Errorf("collateral %s holds an unreadable amount %q: %w", collaterals[i].Asset, collaterals[i].Amount, err)
		}
		if amount.Sign() <= 0 {
			continue
		}
		usd, err := e.assetPriceRat(ctx, collaterals[i].Asset)
		if err != nil {
			return nil, err
		}
		health.CollateralUSD.Add(health.CollateralUSD, new(big.Rat).Mul(amount, usd))
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
		usd, err := e.assetPriceRat(ctx, borrows[i].Asset)
		if err != nil {
			return nil, err
		}
		health.DebtUSD.Add(health.DebtUSD, new(big.Rat).Mul(total, usd))
	}

	health.LiquidationValueUSD = new(big.Rat).Mul(health.CollateralUSD, cfg.LiquidationThreshold)
	if health.DebtUSD.Sign() <= 0 {
		health.HealthFactor = nil // infinite
	} else {
		health.HealthFactor = new(big.Rat).Quo(health.LiquidationValueUSD, health.DebtUSD)
	}
	return health, nil
}

// isLiquidatable reports whether the account's health factor is below 1. A
// debt-free account is never liquidatable. The comparison is exact, so an
// account that sits precisely at 1 is not liquidated.
func (h *marginHealth) isLiquidatable() bool {
	if h.DebtUSD.Sign() <= 0 || h.HealthFactor == nil {
		return false
	}
	return h.HealthFactor.Cmp(big.NewRat(1, 1)) < 0
}

// healthFactorFloat renders the health factor for the API. A debt-free account
// reports +Inf.
func (h *marginHealth) healthFactorFloat() float64 {
	if h.HealthFactor == nil {
		return math.Inf(1)
	}
	f, _ := h.HealthFactor.Float64()
	return f
}

// healthFactorString renders the health factor as a decimal string for the wire.
// A debt-free account has no finite health factor and is reported as
// "Infinity"; encoding/json cannot carry a float64 infinity, so a string keeps
// the whole view serialisable.
func (h *marginHealth) healthFactorString() string {
	if h.HealthFactor == nil {
		return "Infinity"
	}
	return formatDecimal(h.HealthFactor, 8)
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
	return e.accrueAccountInterestTx(e.db, accountID, cfg, now)
}

// accrueAccountInterestTx is the transaction-scoped form of
// accrueAccountInterest, so a settlement can bring interest up to date inside
// the same transaction that locks the account rather than from a stale snapshot.
func (e *MarginEngine) accrueAccountInterestTx(db *gorm.DB, accountID uuid.UUID, cfg *marginMarketConfig, now time.Time) error {
	var borrows []models.MarginBorrow
	if err := db.Where("account_id = ?", accountID).Find(&borrows).Error; err != nil {
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
		if err := db.Model(&models.MarginBorrow{}).Where("id = ?", borrows[i].ID).
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
		return nil, ErrMarginInvalidMode
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

// DepositCollateral adds collateral to an account the caller owns. The deposit
// escrows real ledger balance: the caller's balance is debited and the
// collateral row is written in one transaction, so a deposit can never be
// recorded without the funds to back it.
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
	settled, err := e.assetLedger(normalized)
	if err != nil {
		return err
	}
	units := scaleToBaseUnits(value, settled.Decimals)
	if units.Sign() <= 0 {
		return fmt.Errorf("collateral %s is below one base unit of %s", amount, settled.Symbol)
	}

	return e.db.Transaction(func(tx *gorm.DB) error {
		if err := e.ledger.debitWithin(tx, userID, settled.ChainID, settled.Address, units); err != nil {
			return err
		}

		var existing models.MarginCollateral
		err := tx.Where("account_id = ? AND asset = ?", accountID, normalized).First(&existing).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return tx.Create(&models.MarginCollateral{
				AccountID: accountID,
				Asset:     normalized,
				Amount:    amountValueString(value),
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
		return tx.Model(&models.MarginCollateral{}).Where("id = ?", existing.ID).
			Update("amount", stored).Error
	})
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
	borrowPrice, err := e.assetPriceRat(ctx, normalized)
	if err != nil {
		return fmt.Errorf("cannot price borrow asset %s: %w", normalized, err)
	}

	health, err := e.computeHealth(ctx, e.db, accountID, cfg)
	if err != nil {
		return err
	}
	// New debt may not exceed LTV * collateral. LTV is stricter than the
	// liquidation threshold, so a fresh borrow leaves a safety buffer. The
	// comparison is exact, so a borrow that only fits because of binary
	// rounding is refused.
	maxDebt := new(big.Rat).Mul(health.CollateralUSD, cfg.LTV)
	debt := new(big.Rat).Add(health.DebtUSD, new(big.Rat).Mul(value, borrowPrice))
	if debt.Cmp(maxDebt) > 0 {
		f, _ := value.Float64()
		ltvFloat, _ := cfg.LTV.Float64()
		return fmt.Errorf("%w: borrowing %.8f %s would exceed the LTV of %.2f", ErrMarginInsufficientCollateral, f, normalized, ltvFloat)
	}

	settled, err := e.assetLedger(normalized)
	if err != nil {
		return err
	}
	units := scaleToBaseUnits(value, settled.Decimals)
	if units.Sign() <= 0 {
		return fmt.Errorf("borrow %s is below one base unit of %s", amount, settled.Symbol)
	}

	now := time.Now().UTC()
	// The borrowed asset is credited to the trader and the debt recorded in one
	// transaction, so a credit can never be paid without the matching debt. The
	// platform is the counterparty: the loan is backed by the escrowed collateral
	// and repaid (with interest) out of the trader's balance.
	return e.db.Transaction(func(tx *gorm.DB) error {
		if err := e.ledger.creditWithin(tx, userID, settled.ChainID, settled.Address, settled.Symbol, settled.Decimals, units); err != nil {
			return err
		}

		var existing models.MarginBorrow
		err := tx.Where("account_id = ? AND asset = ?", accountID, normalized).First(&existing).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return tx.Create(&models.MarginBorrow{
				AccountID:       accountID,
				Asset:           normalized,
				Principal:       amountValueString(value),
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
		return tx.Model(&models.MarginBorrow{}).Where("id = ?", existing.ID).
			Updates(map[string]interface{}{
				"principal":        newPrincipal,
				"accrued_interest": newInterest,
				"last_accrual":     now,
			}).Error
	})
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

	settled, err := e.assetLedger(normalized)
	if err != nil {
		return err
	}
	units := scaleToBaseUnits(value, settled.Decimals)
	if units.Sign() <= 0 {
		return fmt.Errorf("repay %s is below one base unit of %s", amount, settled.Symbol)
	}

	// Repaying debits the trader and reduces the debt in one transaction, so a
	// debit can never be taken without the debt it clears. The borrow row is read
	// under the same transaction with a row lock: reading it beforehand let two
	// concurrent repayments each act on the pre-payment debt, so both debits were
	// accepted and the debt was reduced twice for one payment.
	return e.db.Transaction(func(tx *gorm.DB) error {
		var borrow models.MarginBorrow
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("account_id = ? AND asset = ?", accountID, normalized).First(&borrow).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrMarginNoDebt
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

		if err := e.ledger.debitWithin(tx, userID, settled.ChainID, settled.Address, units); err != nil {
			return err
		}

		if value.Cmp(owed) == 0 {
			return tx.Delete(&models.MarginBorrow{}, "id = ?", borrow.ID).Error
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
		return tx.Model(&models.MarginBorrow{}).Where("id = ?", borrow.ID).
			Updates(map[string]interface{}{
				"principal":        storedPrincipal,
				"accrued_interest": storedInterest,
			}).Error
	})
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
		health, err := e.computeHealth(ctx, e.db, accounts[i].ID, cfg)
		if err != nil {
			continue
		}
		if health.isLiquidatable() {
			debtFloat, _ := health.DebtUSD.Float64()
			candidates = append(candidates, LiquidationCandidate{
				AccountID:    accounts[i].ID.String(),
				HealthFactor: health.healthFactorFloat(),
				DebtUSD:      debtFloat,
			})
		}
	}
	return candidates, nil
}

// Liquidate seizes collateral from an under-collateralized account. The debt is
// repaid up to the listing's close factor, and the seized collateral is credited
// to the liquidator (the platform operator who called this endpoint).
func (e *MarginEngine) Liquidate(ctx context.Context, accountID uuid.UUID, liquidator string) (*LiquidationCandidate, error) {
	var account models.MarginAccount
	if err := e.db.First(&account, "id = ?", accountID).Error; err != nil {
		return nil, fmt.Errorf("%w: %s", ErrMarginAccountNotFound, accountID)
	}
	cfg, err := e.marginConfig(account.Pair)
	if err != nil {
		return nil, err
	}

	// The close factor caps how much of the debt one liquidation may repay,
	// which keeps a large account from being closed in a single step.
	closeFactor := cfg.CloseFactor
	if closeFactor.Sign() <= 0 || closeFactor.Cmp(big.NewRat(1, 1)) > 0 {
		closeFactor = big.NewRat(1, 1)
	}

	// Seizing collateral and repaying debt must be one unit of work: if the
	// process died between the two, the trader would lose collateral while the
	// debt stayed on the books. The account row is locked FOR UPDATE and the
	// health that decides liquidatability, the repayable amount and the seized
	// value are all recomputed inside the transaction, so two liquidators cannot
	// both settle the same account from the same pre-liquidation snapshot and
	// seize it twice.
	var health *marginHealth
	err = e.db.Transaction(func(tx *gorm.DB) error {
		var locked models.MarginAccount
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&locked, "id = ?", accountID).Error; err != nil {
			return err
		}
		if locked.Status != models.MarginAccountActive {
			return fmt.Errorf("margin account is %s", locked.Status)
		}
		if err := e.accrueAccountInterestTx(tx, accountID, cfg, time.Now().UTC()); err != nil {
			return err
		}
		settled, err := e.computeHealth(ctx, tx, accountID, cfg)
		if err != nil {
			return err
		}
		if !settled.isLiquidatable() {
			return fmt.Errorf("%w: %s", ErrMarginNotLiquidatable, accountID)
		}
		health = settled

		repayable := new(big.Rat).Mul(settled.DebtUSD, closeFactor)
		seizeValue := new(big.Rat).Mul(repayable, new(big.Rat).Add(big.NewRat(1, 1), cfg.LiquidationBonus))
		if seizeValue.Cmp(settled.CollateralUSD) > 0 {
			seizeValue = settled.CollateralUSD
		}
		if err := e.seizeCollateral(ctx, tx, accountID, seizeValue, liquidator); err != nil {
			return err
		}
		if err := e.reduceDebt(ctx, tx, accountID, repayable); err != nil {
			return err
		}
		// Mark the account liquidated once its debt is cleared, reading the
		// post-settlement state inside the same transaction.
		if remaining, err := e.computeHealth(ctx, tx, accountID, cfg); err == nil && remaining.DebtUSD.Sign() <= 0 {
			if err := tx.Model(&models.MarginAccount{}).Where("id = ?", accountID).
				Update("status", models.MarginAccountLiquidated).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	debtFloat, _ := health.DebtUSD.Float64()
	return &LiquidationCandidate{AccountID: accountID.String(), HealthFactor: health.healthFactorFloat(), DebtUSD: debtFloat}, nil
}

// seizeCollateral removes collateral worth valueUSD, proportionally across the
// account's holdings, and credits the seized amount to the liquidator's ledger
// balance. The trader's collateral is escrowed balance, so seizing it is a real
// transfer out of the trader's escrow into the liquidator's, not a bookkeeping
// edit.
func (e *MarginEngine) seizeCollateral(ctx context.Context, db *gorm.DB, accountID uuid.UUID, valueUSD *big.Rat, liquidator string) error {
	var collaterals []models.MarginCollateral
	if err := db.Where("account_id = ?", accountID).Find(&collaterals).Error; err != nil {
		return err
	}

	type holding struct {
		row models.MarginCollateral
		usd *big.Rat
	}
	holdings := make([]holding, 0, len(collaterals))
	totalUSD := new(big.Rat)
	for i := range collaterals {
		amount, err := parseDecimal(collaterals[i].Amount)
		if err != nil {
			return fmt.Errorf("collateral %s holds an unreadable amount %q: %w", collaterals[i].Asset, collaterals[i].Amount, err)
		}
		if amount.Sign() <= 0 {
			continue
		}
		priceRat, err := e.assetPriceRat(ctx, collaterals[i].Asset)
		if err != nil {
			return err
		}
		usd := new(big.Rat).Mul(amount, priceRat)
		totalUSD.Add(totalUSD, usd)
		holdings = append(holdings, holding{row: collaterals[i], usd: usd})
	}
	if totalUSD.Sign() <= 0 {
		return nil
	}

	// Seize proportionally: each holding loses `fraction` of its balance. The
	// fraction is capped at 1 and the kept fraction is computed exactly so the
	// sum of the kept balances matches the account's collateral rather than
	// drifting with binary rounding.
	fraction := new(big.Rat).Quo(valueUSD, totalUSD)
	if fraction.Cmp(big.NewRat(1, 1)) > 0 {
		fraction = big.NewRat(1, 1)
	}
	keptFraction := new(big.Rat).Sub(big.NewRat(1, 1), fraction)
	for _, h := range holdings {
		amount, err := parseDecimal(h.row.Amount)
		if err != nil {
			return fmt.Errorf("collateral %s holds an unreadable amount %q: %w", h.row.Asset, h.row.Amount, err)
		}
		remaining := new(big.Rat).Mul(amount, keptFraction)
		if remaining.Sign() < 0 {
			remaining = new(big.Rat)
		}
		seized := new(big.Rat).Sub(amount, remaining)
		stored := amountValueString(remaining)
		if err := db.Model(&models.MarginCollateral{}).Where("id = ?", h.row.ID).
			Update("amount", stored).Error; err != nil {
			return err
		}

		// Move the seized amount to the liquidator. A zero seize (the position was
		// already fully drained) is skipped, and a holding whose asset has no
		// ledger entry is refused rather than seized into nowhere.
		if seized.Sign() <= 0 {
			continue
		}
		settled, err := e.assetLedger(h.row.Asset)
		if err != nil {
			return err
		}
		units := scaleToBaseUnits(seized, settled.Decimals)
		if units.Sign() > 0 {
			if err := e.ledger.creditWithin(db, liquidator, settled.ChainID, settled.Address, settled.Symbol, settled.Decimals, units); err != nil {
				return err
			}
		}
	}
	return nil
}

// reduceDebt reduces an account's debt by valueUSD, clearing interest first.
func (e *MarginEngine) reduceDebt(ctx context.Context, db *gorm.DB, accountID uuid.UUID, valueUSD *big.Rat) error {
	var borrows []models.MarginBorrow
	if err := db.Where("account_id = ?", accountID).Find(&borrows).Error; err != nil {
		return err
	}
	remaining := new(big.Rat).Set(valueUSD)
	for i := range borrows {
		if remaining.Sign() <= 0 {
			break
		}
		priceRat, err := e.assetPriceRat(ctx, borrows[i].Asset)
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
		owed := new(big.Rat).Mul(new(big.Rat).Add(principal, interest), priceRat)
		if owed.Sign() <= 0 {
			continue
		}
		// Pay the lesser of what is left to repay and what this borrow owes.
		payUSD := new(big.Rat).Set(remaining)
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
			if err := db.Delete(&models.MarginBorrow{}, "id = ?", borrows[i].ID).Error; err != nil {
				return err
			}
		} else {
			storedPrincipal := amountValueString(newPrincipal)
			storedInterest := amountValueString(newInterest)
			if err := db.Model(&models.MarginBorrow{}).Where("id = ?", borrows[i].ID).
				Updates(map[string]interface{}{
					"principal":        storedPrincipal,
					"accrued_interest": storedInterest,
				}).Error; err != nil {
				return err
			}
		}
		remaining.Sub(remaining, payUSD)
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
	health, err := e.computeHealth(ctx, e.db, accountID, cfg)
	if err != nil {
		return nil, err
	}
	collateralFloat, _ := health.CollateralUSD.Float64()
	debtFloat, _ := health.DebtUSD.Float64()
	equityFloat, _ := new(big.Rat).Sub(health.CollateralUSD, health.DebtUSD).Float64()
	liquidationFloat, _ := health.LiquidationValueUSD.Float64()
	return &MarginAccountView{
		ID:                  account.ID.String(),
		MarginMode:          account.MarginMode,
		Pair:                account.Pair,
		Status:              account.Status,
		CollateralValueUSD:  collateralFloat,
		DebtValueUSD:        debtFloat,
		EquityUSD:           equityFloat,
		HealthFactor:        health.healthFactorString(),
		LiquidationValueUSD: liquidationFloat,
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
		respondBadRequestError(c, err)
		return
	}
	account, err := e.OpenAccount(c.Request.Context(), userID, req)
	if err != nil {
		respondServiceError(c, err)
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
		if errors.Is(err, ErrMarginAccountNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "margin account not found"})
			return
		}
		// The account is the caller's own, so the remaining failures are a stored
		// amount or an oracle read: an internal fault whose text must not reach
		// the client.
		log.Printf("margin: account view failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load margin account"})
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
		respondBadRequestError(c, err)
		return
	}
	accountID, err := uuid.Parse(req.AccountID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid account id"})
		return
	}
	if err := e.DepositCollateral(c.Request.Context(), userID, accountID, req.Asset, req.Amount); err != nil {
		respondServiceError(c, err)
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
		respondBadRequestError(c, err)
		return
	}
	accountID, err := uuid.Parse(req.AccountID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid account id"})
		return
	}
	if err := e.Borrow(c.Request.Context(), userID, accountID, req.Asset, req.Amount); err != nil {
		respondServiceError(c, err)
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
		respondBadRequestError(c, err)
		return
	}
	accountID, err := uuid.Parse(req.AccountID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid account id"})
		return
	}
	if err := e.Repay(c.Request.Context(), userID, accountID, req.Asset, req.Amount); err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// LiquidateAccountHandler executes a liquidation on one account. It is an
// operator endpoint: the route middleware enforces the admin role, and
// Liquidate re-checks that the account is genuinely under-collateralised before
// any collateral moves, so it cannot be used to seize a healthy account.
func (e *MarginEngine) LiquidateAccountHandler(c *gin.Context) {
	accountID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid account id"})
		return
	}

	// The seized collateral is credited to the operator who called this endpoint.
	liquidator, ok := authenticatedUserID(c)
	if !ok {
		return
	}

	result, err := e.Liquidate(c.Request.Context(), accountID, liquidator)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "liquidation": result})
}

// ListLiquidatableHandler lists accounts eligible for liquidation. It is an
// operator endpoint: the caller's role is enforced by the route middleware.
func (e *MarginEngine) ListLiquidatableHandler(c *gin.Context) {
	candidates, err := e.LiquidatableAccounts(c.Request.Context())
	if err != nil {
		log.Printf("margin: list liquidatable failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load liquidatable accounts"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"accounts": candidates, "count": len(candidates)})
}
