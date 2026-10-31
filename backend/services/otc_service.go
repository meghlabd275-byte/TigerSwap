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
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"tigerswap/backend/models"
)

// OtcService implements private block trades. An OTC order never enters the
// public book, so a large trade produces no price impact: the two
// counterparties are matched directly and the fill is recorded.
type OtcService struct {
	db *gorm.DB
}

func NewOtcService(db *gorm.DB) *OtcService {
	return &OtcService{db: db}
}

// maxOtcExpiry bounds how far ahead an order may expire, so an order cannot be
// parked indefinitely and block a price.
const maxOtcExpiry = 30 * 24 * time.Hour

type createOtcRequest struct {
	Symbol    string `json:"symbol"`
	Side      string `json:"side"`
	Quantity  string `json:"quantity"`
	Price     string `json:"price"`
	MinFill   string `json:"min_fill"`
	ExpiresIn int64  `json:"expires_in_seconds"`
}

// CreateOrder publishes a private block trade.
func (s *OtcService) CreateOrder(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	var req createOtcRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	symbol := strings.TrimSpace(req.Symbol)
	if err := validateMarketSymbol(symbol); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Side != "buy" && req.Side != "sell" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "side must be buy or sell"})
		return
	}

	quantity, err := validatePositiveDecimal("quantity", req.Quantity)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if _, err := validatePositiveDecimal("price", req.Price); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// min_fill defaults to the full quantity, which is the safe default: a
	// partial fill must be asked for explicitly.
	minFill := req.MinFill
	if strings.TrimSpace(minFill) == "" {
		minFill = req.Quantity
	}
	minFillValue, err := validatePositiveDecimal("min_fill", minFill)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if minFillValue.Cmp(quantity) > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "min_fill cannot exceed quantity"})
		return
	}

	if req.ExpiresIn <= 0 || req.ExpiresIn > int64(maxOtcExpiry.Seconds()) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("expires_in_seconds must be between 1 and %d", int64(maxOtcExpiry.Seconds())),
		})
		return
	}

	order := models.OtcOrder{
		MakerID:   userID,
		Symbol:    symbol,
		Side:      req.Side,
		Quantity:  req.Quantity,
		Price:     req.Price,
		MinFill:   minFill,
		Status:    "open",
		ExpiresAt: time.Now().UTC().Add(time.Duration(req.ExpiresIn) * time.Second),
	}

	if err := s.db.Create(&order).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create otc order"})
		return
	}

	c.JSON(http.StatusCreated, order)
}

// ListOrders returns the caller's own OTC orders.
func (s *OtcService) ListOrders(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	query := s.db.Where("maker_id = ? OR taker_id = ?", userID, userID)
	if status := strings.TrimSpace(c.Query("status")); status != "" {
		query = query.Where("status = ?", status)
	}

	var orders []models.OtcOrder
	if err := query.Order("created_at DESC").Limit(200).Find(&orders).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not list otc orders"})
		return
	}
	c.JSON(http.StatusOK, orders)
}

// ListOpenOrders returns open OTC orders the caller could take. The maker's own
// orders are excluded, because a trader cannot fill their own block.
func (s *OtcService) ListOpenOrders(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	query := s.db.Where("status = ? AND expires_at > ? AND maker_id <> ?",
		"open", time.Now().UTC(), userID)
	if symbol := strings.TrimSpace(c.Query("symbol")); symbol != "" {
		query = query.Where("symbol = ?", symbol)
	}

	var orders []models.OtcOrder
	if err := query.Order("created_at DESC").Limit(200).Find(&orders).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not list open otc orders"})
		return
	}
	c.JSON(http.StatusOK, orders)
}

type takeOtcRequest struct {
	Quantity string `json:"quantity"`
}

// TakeOrder fills an open OTC order, wholly or down to its minimum fill.
func (s *OtcService) TakeOrder(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid order id"})
		return
	}

	var req takeOtcRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	fill, err := validatePositiveDecimal("quantity", req.Quantity)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var settled models.OtcOrder
	err = s.db.Transaction(func(tx *gorm.DB) error {
		var order models.OtcOrder
		// Lock the row so two takers cannot both fill the same order.
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", id).First(&order).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errOrderNotFound
			}
			return err
		}

		if order.Status != "open" {
			return errOrderNotOpen
		}
		if order.MakerID == userID {
			return errors.New("you cannot take your own otc order")
		}
		if !order.ExpiresAt.After(time.Now().UTC()) {
			return errOrderExpired
		}

		quantity, err := validatePositiveDecimal("quantity", order.Quantity)
		if err != nil {
			return err
		}
		minFill, err := validatePositiveDecimal("min_fill", order.MinFill)
		if err != nil {
			return err
		}
		if fill.Cmp(minFill) < 0 {
			return errBelowMinFill
		}
		if fill.Cmp(quantity) > 0 {
			return errors.New("quantity exceeds the order size")
		}

		takerID := userID
		order.TakerID = &takerID
		order.Status = "matched"
		order.Counterparty = strconv.FormatUint(uint64(order.MakerID), 10)

		// Record the executed quantity so a partial fill is visible, and close
		// the order: a partially filled block is not re-offered at a price the
		// taker has now moved away from.
		order.Quantity = req.Quantity

		if err := tx.Save(&order).Error; err != nil {
			return err
		}
		settled = order
		return nil
	})

	switch {
	case err == nil:
		c.JSON(http.StatusOK, settled)
	case errors.Is(err, errOrderNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "otc order not found"})
	case errors.Is(err, errOrderNotOpen):
		c.JSON(http.StatusConflict, gin.H{"error": "otc order is not open"})
	case errors.Is(err, errOrderExpired):
		c.JSON(http.StatusConflict, gin.H{"error": "otc order has expired"})
	case errors.Is(err, errBelowMinFill):
		c.JSON(http.StatusBadRequest, gin.H{"error": "quantity is below the order's min_fill"})
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	}
}

// CancelOrder cancels the caller's own open OTC order.
func (s *OtcService) CancelOrder(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid order id"})
		return
	}

	var order models.OtcOrder
	if err := s.db.Where("id = ? AND maker_id = ?", id, userID).First(&order).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "otc order not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load otc order"})
		return
	}

	if order.Status != "open" {
		c.JSON(http.StatusConflict, gin.H{"error": "only an open otc order can be cancelled"})
		return
	}

	if err := s.db.Model(&order).Update("status", "cancelled").Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not cancel otc order"})
		return
	}

	order.Status = "cancelled"
	c.JSON(http.StatusOK, order)
}

// ExpireStaleOrders marks open orders whose expiry has passed. Called by the
// keeper so a stale order does not linger in the open list.
func (s *OtcService) ExpireStaleOrders() (int64, error) {
	result := s.db.Model(&models.OtcOrder{}).
		Where("status = ? AND expires_at <= ?", "open", time.Now().UTC()).
		Update("status", "expired")
	return result.RowsAffected, result.Error
}

var (
	errOrderNotFound = errors.New("otc order not found")
	errOrderNotOpen  = errors.New("otc order is not open")
	errOrderExpired  = errors.New("otc order has expired")
	errBelowMinFill  = errors.New("below minimum fill")
)

// TwammService implements time-weighted average market maker orders. An order
// executes in slices across its lifetime, so there is no single transaction for
// a searcher to front-run.
type TwammService struct {
	db *gorm.DB
}

func NewTwammService(db *gorm.DB) *TwammService {
	return &TwammService{db: db}
}

// minTwammDuration prevents a TWAMM order from being used as a plain market
// order, which would defeat the point of the order type.
const minTwammDuration = 5 * time.Minute

// maxTwammDuration bounds a schedule so it cannot run indefinitely.
const maxTwammDuration = 180 * 24 * time.Hour

type createTwammRequest struct {
	Symbol        string `json:"symbol"`
	Side          string `json:"side"`
	TotalQuantity string `json:"total_quantity"`
	LimitPrice    string `json:"limit_price"`
	DurationHours int64  `json:"duration_hours"`
	StartDelay    int64  `json:"start_delay_seconds"`
}

// CreateOrder schedules a TWAMM order.
func (s *TwammService) CreateOrder(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	var req createTwammRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	symbol := strings.TrimSpace(req.Symbol)
	if err := validateMarketSymbol(symbol); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Side != "buy" && req.Side != "sell" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "side must be buy or sell"})
		return
	}
	if _, err := validatePositiveDecimal("total_quantity", req.TotalQuantity); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// A limit price of zero means no limit, which is the one place a zero
	// decimal is allowed.
	if strings.TrimSpace(req.LimitPrice) != "" && req.LimitPrice != "0" {
		if _, err := validatePositiveDecimal("limit_price", req.LimitPrice); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	} else {
		req.LimitPrice = "0"
	}

	duration := time.Duration(req.DurationHours) * time.Hour
	if duration < minTwammDuration || duration > maxTwammDuration {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("duration_hours must be between %d and %d",
				int(minTwammDuration.Hours()), int(maxTwammDuration.Hours())),
		})
		return
	}

	if req.StartDelay < 0 || req.StartDelay > int64(maxTwammDuration.Seconds()) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "start_delay_seconds is out of range"})
		return
	}

	now := time.Now().UTC()
	start := now.Add(time.Duration(req.StartDelay) * time.Second)

	order := models.TwammOrder{
		UserID:         userID,
		Symbol:         symbol,
		Side:           req.Side,
		TotalQuantity:  req.TotalQuantity,
		FilledQuantity: "0",
		LimitPrice:     req.LimitPrice,
		StartTime:      start,
		EndTime:        start.Add(duration),
		Status:         "active",
	}

	if err := s.db.Create(&order).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create twamm order"})
		return
	}

	c.JSON(http.StatusCreated, order)
}

// ListOrders returns the caller's TWAMM orders.
func (s *TwammService) ListOrders(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	var orders []models.TwammOrder
	if err := s.db.Where("user_id = ?", userID).
		Order("created_at DESC").Limit(200).Find(&orders).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not list twamm orders"})
		return
	}
	c.JSON(http.StatusOK, orders)
}

// CancelOrder cancels a TWAMM order that has not finished.
func (s *TwammService) CancelOrder(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid order id"})
		return
	}

	var order models.TwammOrder
	if err := s.db.Where("id = ? AND user_id = ?", id, userID).First(&order).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "twamm order not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load twamm order"})
		return
	}

	if order.Status != "active" {
		c.JSON(http.StatusConflict, gin.H{"error": "only an active twamm order can be cancelled"})
		return
	}

	if err := s.db.Model(&order).Update("status", "cancelled").Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not cancel twamm order"})
		return
	}

	order.Status = "cancelled"
	c.JSON(http.StatusOK, order)
}

// ExecutableSlice returns the base quantity this order should trade at `now`.
//
// The schedule is linear: a slice is the part of the total quantity whose time
// window has elapsed but which has not yet been filled. Deriving the slice from
// the elapsed time rather than from a fixed count means a missed sweep does not
// permanently under-fill the order.
func (s *TwammService) ExecutableSlice(order models.TwammOrder, now time.Time) (*big.Int, error) {
	total, err := validatePositiveDecimal("total_quantity", order.TotalQuantity)
	if err != nil {
		return nil, err
	}
	filled, err := parseDecimalString(order.FilledQuantity)
	if err != nil {
		return nil, err
	}

	if !now.After(order.StartTime) {
		return big.NewInt(0), nil
	}

	totalDuration := order.EndTime.Sub(order.StartTime)
	if totalDuration <= 0 {
		return nil, errors.New("twamm order has no duration")
	}

	elapsed := now.Sub(order.StartTime)
	if elapsed >= totalDuration {
		elapsed = totalDuration
	}

	// Scheduled cumulative quantity at `now`.
	scheduled := new(big.Int).Mul(total, big.NewInt(int64(elapsed)))
	scheduled.Div(scheduled, big.NewInt(int64(totalDuration)))

	slice := new(big.Int).Sub(scheduled, filled)
	if slice.Sign() < 0 {
		// More has been filled than the schedule has released; nothing to do.
		return big.NewInt(0), nil
	}
	return slice, nil
}
