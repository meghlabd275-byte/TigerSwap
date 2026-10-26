package services

import (
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"tigerswap/backend/models"
)

// GovernanceService implements veTGR locking, gauge voting and the bribe
// market. All arithmetic on token amounts uses big.Int over base-10 strings so
// no precision is lost to floating point.
type GovernanceService struct {
	db *gorm.DB
}

func NewGovernanceService(db *gorm.DB) *GovernanceService {
	return &GovernanceService{db: db}
}

const (
	weekSeconds    = 7 * 24 * 60 * 60
	maxLockSeconds = 4 * 365 * 24 * 60 * 60
	minLockSeconds = weekSeconds
	bpsDenominator = 10000
	minBoostBps    = 10000 // 1x
	maxBoostBps    = 25000 // 2.5x
)

// parseAmount parses a base-10 token amount and rejects anything non-positive.
func parseAmount(raw string) (*big.Int, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, errors.New("amount is required")
	}
	amount, ok := new(big.Int).SetString(trimmed, 10)
	if !ok {
		return nil, errors.New("amount must be a base-10 integer")
	}
	if amount.Sign() <= 0 {
		return nil, errors.New("amount must be positive")
	}
	return amount, nil
}

// CurrentEpoch returns the governance epoch index for a timestamp. Epochs are
// one week long, matching the veTGR decay schedule.
func CurrentEpoch(t time.Time) int64 {
	return t.Unix() / weekSeconds
}

// votingPower returns the decayed veTGR balance of a lock at time `now`.
// Power falls linearly from Amount at StartTime to zero at UnlockTime.
func votingPower(lock models.VeLock, now time.Time) *big.Int {
	if lock.Withdrawn || !now.Before(lock.UnlockTime) {
		return big.NewInt(0)
	}
	total := lock.UnlockTime.Sub(lock.StartTime).Seconds()
	if total <= 0 {
		return big.NewInt(0)
	}
	remaining := lock.UnlockTime.Sub(now).Seconds()
	amount, ok := new(big.Int).SetString(lock.Amount, 10)
	if !ok {
		return big.NewInt(0)
	}
	power := new(big.Int).Mul(amount, big.NewInt(int64(remaining)))
	return power.Div(power, big.NewInt(int64(total)))
}

// CreateLock opens a veTGR lock for the authenticated caller.
func (s *GovernanceService) CreateLock(c *gin.Context) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}

	var req struct {
		Amount         string `json:"amount" binding:"required"`
		DurationSecond int64  `json:"duration_seconds" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	amount, err := parseAmount(req.Amount)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.DurationSecond < minLockSeconds {
		c.JSON(http.StatusBadRequest, gin.H{"error": "lock duration is below the one week minimum"})
		return
	}
	if req.DurationSecond > maxLockSeconds {
		c.JSON(http.StatusBadRequest, gin.H{"error": "lock duration exceeds the four year maximum"})
		return
	}

	// Only one open lock per user keeps the decay accounting unambiguous.
	var openLocks int64
	if err := s.db.Model(&models.VeLock{}).
		Where("user_id = ? AND withdrawn = ? AND unlock_time > ?", userID, false, time.Now()).
		Count(&openLocks).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check existing locks"})
		return
	}
	if openLocks > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "an active lock already exists"})
		return
	}

	now := time.Now()
	lock := models.VeLock{
		UserID:     userID,
		Amount:     amount.String(),
		StartTime:  now,
		UnlockTime: now.Add(time.Duration(req.DurationSecond) * time.Second),
	}
	if err := s.db.Create(&lock).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create lock"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success":      true,
		"lock":         lock,
		"voting_power": votingPower(lock, now).String(),
	})
}

// GetLocks returns the caller's locks with their live voting power.
func (s *GovernanceService) GetLocks(c *gin.Context) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}

	var locks []models.VeLock
	if err := s.db.Where("user_id = ?", userID).Order("created_at DESC").Find(&locks).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load locks"})
		return
	}

	now := time.Now()
	type lockView struct {
		models.VeLock
		VotingPower string `json:"voting_power"`
	}
	views := make([]lockView, 0, len(locks))
	for _, l := range locks {
		views = append(views, lockView{VeLock: l, VotingPower: votingPower(l, now).String()})
	}

	total := s.totalVotingPower(userID, now)
	c.JSON(http.StatusOK, gin.H{
		"success":            true,
		"locks":              views,
		"total_voting_power": total.String(),
	})
}

// totalVotingPower sums the live voting power across a user's locks.
func (s *GovernanceService) totalVotingPower(userID string, now time.Time) *big.Int {
	var locks []models.VeLock
	if err := s.db.Where("user_id = ? AND withdrawn = ?", userID, false).Find(&locks).Error; err != nil {
		return big.NewInt(0)
	}
	total := big.NewInt(0)
	for _, l := range locks {
		total.Add(total, votingPower(l, now))
	}
	return total
}

// WithdrawLock releases a matured lock.
func (s *GovernanceService) WithdrawLock(c *gin.Context) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}
	lockID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid lock id"})
		return
	}

	var lock models.VeLock
	if err := s.db.Where("id = ? AND user_id = ?", lockID, userID).First(&lock).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "lock not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load lock"})
		return
	}
	if lock.Withdrawn {
		c.JSON(http.StatusConflict, gin.H{"error": "lock already withdrawn"})
		return
	}
	if time.Now().Before(lock.UnlockTime) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "lock has not expired"})
		return
	}

	lock.Withdrawn = true
	if err := s.db.Save(&lock).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to withdraw lock"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "amount": lock.Amount})
}

// CreateGauge registers an emission target. Admin only.
func (s *GovernanceService) CreateGauge(c *gin.Context) {
	var req struct {
		Name      string `json:"name" binding:"required"`
		GaugeType string `json:"gauge_type" binding:"required"`
		Reference string `json:"reference"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "gauge name is required"})
		return
	}
	gaugeType := strings.ToLower(strings.TrimSpace(req.GaugeType))
	if !allowedGaugeTypes[gaugeType] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported gauge type"})
		return
	}

	gauge := models.Gauge{
		Name:      name,
		GaugeType: gaugeType,
		Reference: strings.TrimSpace(req.Reference),
		Weight:    "0",
		Active:    true,
	}
	if err := s.db.Create(&gauge).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create gauge"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "gauge": gauge})
}

var allowedGaugeTypes = map[string]bool{
	models.GaugeTypePool:     true,
	models.GaugeTypeVault:    true,
	models.GaugeTypeLending:  true,
	models.GaugeTypePerps:    true,
	models.GaugeTypeExternal: true,
}

// GetGauges lists gauges, optionally filtered to active ones only.
func (s *GovernanceService) GetGauges(c *gin.Context) {
	var gauges []models.Gauge
	query := s.db.Order("created_at ASC")
	if c.Query("active") == "true" {
		query = query.Where("active = ?", true)
	}
	if err := query.Find(&gauges).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load gauges"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "gauges": gauges})
}

// Vote sets the caller's weight for a gauge. The total across all gauges can
// never exceed the caller's live voting power.
func (s *GovernanceService) Vote(c *gin.Context) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}

	var req struct {
		GaugeID string `json:"gauge_id" binding:"required"`
		Weight  string `json:"weight" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	gaugeID, err := uuid.Parse(req.GaugeID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid gauge id"})
		return
	}
	weight, err := parseAmountOrZero(req.Weight)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var gauge models.Gauge
	if err := s.db.Where("id = ? AND active = ?", gaugeID, true).First(&gauge).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "gauge not found"})
		return
	}

	now := time.Now()
	power := s.totalVotingPower(userID, now)

	var votes []models.GaugeVote
	if err := s.db.Where("user_id = ?", userID).Find(&votes).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load votes"})
		return
	}

	// Sum of every other gauge's weight plus the new weight must fit the power.
	newTotal := new(big.Int).Set(weight)
	var previous *big.Int
	for _, v := range votes {
		vw, _ := new(big.Int).SetString(v.Weight, 10)
		if vw == nil {
			continue
		}
		if v.GaugeID == gaugeID {
			previous = vw
			continue
		}
		newTotal.Add(newTotal, vw)
	}
	if newTotal.Cmp(power) > 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":        "weight exceeds available voting power",
			"voting_power": power.String(),
		})
		return
	}

	epoch := CurrentEpoch(now)
	err = s.db.Transaction(func(tx *gorm.DB) error {
		// Recompute the gauge total in Go so the arithmetic is exact and does
		// not depend on any database dialect's numeric handling.
		var current models.Gauge
		if err := tx.Where("id = ?", gaugeID).First(&current).Error; err != nil {
			return err
		}
		total, _ := new(big.Int).SetString(current.TotalVoted, 10)
		if total == nil {
			total = big.NewInt(0)
		}
		if previous != nil {
			total.Sub(total, previous)
		}
		total.Add(total, weight)
		if total.Sign() < 0 {
			total = big.NewInt(0)
		}
		if err := tx.Model(&models.Gauge{}).Where("id = ?", gaugeID).
			UpdateColumn("total_voted", total.String()).Error; err != nil {
			return err
		}

		var existing models.GaugeVote
		lookup := tx.Where("user_id = ? AND gauge_id = ?", userID, gaugeID).First(&existing)
		if lookup.Error == nil {
			existing.Weight = weight.String()
			return tx.Save(&existing).Error
		}
		if !errors.Is(lookup.Error, gorm.ErrRecordNotFound) {
			return lookup.Error
		}
		return tx.Create(&models.GaugeVote{
			UserID:  userID,
			GaugeID: gaugeID,
			Weight:  weight.String(),
		}).Error
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to record vote"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"gauge_id":     gaugeID,
		"weight":       weight.String(),
		"epoch":        epoch,
		"voting_power": power.String(),
	})
}

// parseAmountOrZero accepts a non-negative amount, including zero for removing
// a vote.
func parseAmountOrZero(raw string) (*big.Int, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, errors.New("weight is required")
	}
	weight, ok := new(big.Int).SetString(trimmed, 10)
	if !ok {
		return nil, errors.New("weight must be a base-10 integer")
	}
	if weight.Sign() < 0 {
		return nil, errors.New("weight must not be negative")
	}
	return weight, nil
}

// AddBribe funds a bribe for the current epoch of a gauge.
func (s *GovernanceService) AddBribe(c *gin.Context) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}

	var req struct {
		GaugeID string `json:"gauge_id" binding:"required"`
		Token   string `json:"token" binding:"required"`
		Amount  string `json:"amount" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	gaugeID, err := uuid.Parse(req.GaugeID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid gauge id"})
		return
	}
	amount, err := parseAmount(req.Amount)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	token := strings.ToUpper(strings.TrimSpace(req.Token))
	if !allowedSettlementAssets[token] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported bribe token"})
		return
	}

	var gauge models.Gauge
	if err := s.db.Where("id = ? AND active = ?", gaugeID, true).First(&gauge).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "gauge not found"})
		return
	}

	epoch := CurrentEpoch(time.Now())

	// Snapshot the vote weight at funding time so claims are deterministic.
	var totalWeight big.Int
	var votes []models.GaugeVote
	if err := s.db.Where("gauge_id = ?", gaugeID).Find(&votes).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load votes"})
		return
	}
	for _, v := range votes {
		vw, _ := new(big.Int).SetString(v.Weight, 10)
		if vw != nil {
			totalWeight.Add(&totalWeight, vw)
		}
	}

	bribe := models.Bribe{
		GaugeID:     gaugeID,
		Epoch:       epoch,
		Token:       token,
		Amount:      amount.String(),
		TotalWeight: totalWeight.String(),
		FunderID:    userID,
	}
	if err := s.db.Create(&bribe).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to record bribe"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"success": true, "bribe": bribe})
}

// GetBribes lists bribes for a gauge.
func (s *GovernanceService) GetBribes(c *gin.Context) {
	gaugeID, err := uuid.Parse(c.Query("gauge_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid gauge id"})
		return
	}

	var bribes []models.Bribe
	if err := s.db.Where("gauge_id = ?", gaugeID).Order("epoch DESC").Find(&bribes).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load bribes"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "bribes": bribes})
}

// ClaimBribe pays the caller their pro-rata share of a bribe.
func (s *GovernanceService) ClaimBribe(c *gin.Context) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}
	bribeID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid bribe id"})
		return
	}

	var bribe models.Bribe
	if err := s.db.Where("id = ?", bribeID).First(&bribe).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "bribe not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load bribe"})
		return
	}

	var vote models.GaugeVote
	if err := s.db.Where("user_id = ? AND gauge_id = ?", userID, bribe.GaugeID).First(&vote).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no vote recorded for this gauge"})
		return
	}

	totalWeight, _ := new(big.Int).SetString(bribe.TotalWeight, 10)
	userWeight, _ := new(big.Int).SetString(vote.Weight, 10)
	amount, _ := new(big.Int).SetString(bribe.Amount, 10)
	if totalWeight == nil || totalWeight.Sign() == 0 || userWeight == nil || amount == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bribe has no distributable weight"})
		return
	}

	share := new(big.Int).Mul(amount, userWeight)
	share.Div(share, totalWeight)
	if share.Sign() == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "computed share is zero"})
		return
	}

	claim := models.BribeClaim{
		BribeID: bribe.ID,
		UserID:  userID,
		Amount:  share.String(),
	}
	// The unique index on (bribe_id, user_id) makes a double claim impossible
	// even under concurrent requests.
	if err := s.db.Create(&claim).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "bribe already claimed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"token":   bribe.Token,
		"amount":  share.String(),
	})
}

// GetBoost returns the caller's farm boost in basis points for a staked amount.
func (s *GovernanceService) GetBoost(c *gin.Context) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}

	stakedRaw := c.Query("staked_amount")
	staked, err := parseAmount(stakedRaw)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	power := s.totalVotingPower(userID, time.Now())
	// boost = 1x + 1.5x * (veTGR / staked), capped at 2.5x.
	bonus := new(big.Int).Mul(big.NewInt(15000), power)
	bonus.Div(bonus, staked)
	total := new(big.Int).Add(big.NewInt(minBoostBps), bonus)

	capped := total
	if capped.Cmp(big.NewInt(maxBoostBps)) > 0 {
		capped = big.NewInt(maxBoostBps)
	}

	c.JSON(http.StatusOK, gin.H{
		"success":          true,
		"voting_power":     power.String(),
		"staked_amount":    staked.String(),
		"boost_bps":        capped.String(),
		"boost_multiplier": formatBps(capped),
	})
}

// formatBps renders basis points as a decimal multiplier, e.g. 25000 -> "2.5".
func formatBps(bps *big.Int) string {
	whole := new(big.Int).Div(bps, big.NewInt(10000))
	frac := new(big.Int).Mod(bps, big.NewInt(10000))
	if frac.Sign() == 0 {
		return whole.String()
	}
	// Four fractional digits, with trailing zeros removed.
	digits := fmt.Sprintf("%04d", frac.Int64())
	digits = strings.TrimRight(digits, "0")
	return whole.String() + "." + digits
}

// SetGaugeWeight sets the emission weight of a gauge. Admin only.
func (s *GovernanceService) SetGaugeWeight(c *gin.Context) {
	gaugeID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid gauge id"})
		return
	}

	var req struct {
		Weight string `json:"weight" binding:"required"`
		Active *bool  `json:"active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	weight, err := parseAmountOrZero(req.Weight)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updates := map[string]interface{}{"weight": weight.String()}
	if req.Active != nil {
		updates["active"] = *req.Active
	}

	result := s.db.Model(&models.Gauge{}).Where("id = ?", gaugeID).Updates(updates)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update gauge"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "gauge not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}
