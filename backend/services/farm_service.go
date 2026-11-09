package services

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"tigerswap/backend/models"
)

// accScale is the fixed-point scale of the MasterChef reward accumulator. Every
// division that converts emitted reward into per-share terms is scaled by this
// value so integer arithmetic does not truncate a staker's share to zero.
var accScale = big.NewInt(1_000_000_000_000_000_000)

// boostSource supplies a staker's veTGR boost in basis points for a staked
// amount. *GovernanceService is the production implementation.
type boostSource interface {
	BoostBps(userID string, staked *big.Int) (*big.Int, error)
}

// FarmService implements liquidity mining. Stake is escrowed in the platform
// ledger, rewards are emitted per second through a MasterChef accumulator and a
// veTGR boost scales a staker's share. All token amounts are exact integers in
// the token's smallest unit; no floating point touches a balance.
type FarmService struct {
	db     *gorm.DB
	ledger *LedgerService
	boost  boostSource
	prices priceSource
}

func NewFarmService(db *gorm.DB, ledger *LedgerService) *FarmService {
	return &FarmService{db: db, ledger: ledger}
}

// SetBoostSource wires the veTGR boost provider.
func (s *FarmService) SetBoostSource(source boostSource) { s.boost = source }

// SetPriceSource wires the oracle reader used to compute a farm's APR.
func (s *FarmService) SetPriceSource(source priceSource) { s.prices = source }

var (
	allowedFarmTypes = map[string]bool{
		models.FarmTypeLP:     true,
		models.FarmTypeSingle: true,
		models.FarmTypeStable: true,
	}
	allowedFarmStatuses = map[string]bool{
		models.FarmStatusActive: true,
		models.FarmStatusPaused: true,
		models.FarmStatusClosed: true,
	}
)

// maxRewardRatePerSecond bounds an emission rate. The bound is far above any
// real rate but keeps the accumulator finite for the lifetime of the farm.
var maxRewardRatePerSecond = new(big.Int).Exp(big.NewInt(10), big.NewInt(36), nil)

// errFarmNotOpen is returned when a stake is attempted on a farm that is paused,
// has not started, or has ended. It is checked against the farm re-read under
// lock so a concurrent admin change cannot be raced.
var errFarmNotOpen = errors.New("farm is not open for staking")

// parseTokenAmount parses a non-negative base-10 integer amount.
func parseTokenAmount(raw string) (*big.Int, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, errors.New("amount is required")
	}
	amount, ok := new(big.Int).SetString(trimmed, 10)
	if !ok {
		return nil, errors.New("amount must be a base-10 integer")
	}
	if amount.Sign() < 0 {
		return nil, errors.New("amount must not be negative")
	}
	return amount, nil
}

// tokenKeyOrNative validates a token address and returns the ledger key for it.
// The literal "native" and the empty string both denote the chain's native
// asset; anything else must be a hex address.
func tokenKeyOrNative(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || strings.EqualFold(trimmed, "native") {
		return "native", nil
	}
	if !common.IsHexAddress(trimmed) {
		return "", publicErr(fmt.Errorf("token address %q is not a valid address", raw))
	}
	return strings.ToLower(common.HexToAddress(trimmed).Hex()), nil
}

// parseListingTime parses an RFC3339 timestamp, the format the other listing
// endpoints accept.
func parseListingTime(raw string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(raw))
	if err != nil {
		return time.Time{}, publicErr(errors.New("must be an RFC3339 timestamp"))
	}
	return parsed, nil
}

// scaleAmount converts an integer amount to its decimal value as an exact
// rational.
func scaleAmount(amount *big.Int, decimals int) *big.Rat {
	value := new(big.Rat).SetInt(amount)
	if decimals <= 0 {
		return value
	}
	divisor := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil)
	return value.Quo(value, new(big.Rat).SetInt(divisor))
}

// loadFarm reads a farm by id, returning ErrListingNotFound when it does not
// exist so callers map a missing row to 404 rather than 500.
func (s *FarmService) loadFarm(id uuid.UUID) (*models.FarmPool, error) {
	var farm models.FarmPool
	if err := s.db.First(&farm, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrListingNotFound
		}
		return nil, err
	}
	return &farm, nil
}

// CreateFarm lists a new farm. The farm starts paused, matching the listing
// tables: an operator funds the reward streams and then activates it.
func (s *FarmService) CreateFarm(c *gin.Context) {
	var req struct {
		Key                  string `json:"key" binding:"required"`
		Name                 string `json:"name" binding:"required"`
		FarmType             string `json:"farm_type"`
		ChainID              int64  `json:"chain_id" binding:"required"`
		StakingTokenAddress  string `json:"staking_token_address" binding:"required"`
		StakingTokenSymbol   string `json:"staking_token_symbol" binding:"required"`
		StakingTokenDecimals int    `json:"staking_token_decimals"`
		BoostGaugeID         string `json:"boost_gauge_id"`
		StartTime            string `json:"start_time" binding:"required"`
		EndTime              string `json:"end_time" binding:"required"`
		Rewards              []struct {
			TokenAddress  string `json:"token_address" binding:"required"`
			TokenSymbol   string `json:"token_symbol" binding:"required"`
			TokenDecimals int    `json:"token_decimals"`
			RatePerSecond string `json:"rate_per_second" binding:"required"`
		} `json:"rewards" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequestError(c, err)
		return
	}

	key := strings.ToLower(strings.TrimSpace(req.Key))
	if key == "" || len(key) > 120 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "key must be 1 to 120 characters"})
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}
	farmType := strings.ToLower(strings.TrimSpace(req.FarmType))
	if farmType == "" {
		farmType = models.FarmTypeLP
	}
	if !allowedFarmTypes[farmType] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "farm_type must be lp, single or stable"})
		return
	}
	if req.StakingTokenDecimals < 0 || req.StakingTokenDecimals > 36 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "staking_token_decimals must be between 0 and 36"})
		return
	}
	stakingKey, err := tokenKeyOrNative(req.StakingTokenAddress)
	if err != nil {
		respondBadRequestError(c, err)
		return
	}
	startTime, err := parseListingTime(req.StartTime)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("start_time: %v", err)})
		return
	}
	endTime, err := parseListingTime(req.EndTime)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("end_time: %v", err)})
		return
	}
	if !endTime.After(startTime) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "end_time must be after start_time"})
		return
	}
	if len(req.Rewards) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "at least one reward stream is required"})
		return
	}

	var gaugeID *uuid.UUID
	if raw := strings.TrimSpace(req.BoostGaugeID); raw != "" {
		parsed, err := uuid.Parse(raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "boost_gauge_id is not a valid id"})
			return
		}
		var gauge models.Gauge
		if err := s.db.First(&gauge, "id = ?", parsed).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "boost_gauge_id does not match an existing gauge"})
			return
		}
		gaugeID = &parsed
	}

	// Validate every reward stream before writing anything, so a bad stream
	// cannot leave a half-created farm behind.
	type rewardInput struct {
		key      string
		symbol   string
		decimals int
		rate     *big.Int
	}
	inputs := make([]rewardInput, 0, len(req.Rewards))
	seen := make(map[string]bool, len(req.Rewards))
	for _, stream := range req.Rewards {
		rewardKey, err := tokenKeyOrNative(stream.TokenAddress)
		if err != nil {
			respondBadRequestError(c, err)
			return
		}
		if seen[rewardKey] {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("duplicate reward token %s", rewardKey)})
			return
		}
		seen[rewardKey] = true
		symbol := strings.ToUpper(strings.TrimSpace(stream.TokenSymbol))
		if symbol == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "reward token_symbol is required"})
			return
		}
		if stream.TokenDecimals < 0 || stream.TokenDecimals > 36 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "reward token_decimals must be between 0 and 36"})
			return
		}
		rate, err := parseTokenAmount(stream.RatePerSecond)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("rate_per_second: %v", err)})
			return
		}
		if rate.Cmp(maxRewardRatePerSecond) > 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "rate_per_second exceeds the maximum"})
			return
		}
		inputs = append(inputs, rewardInput{key: rewardKey, symbol: symbol, decimals: stream.TokenDecimals, rate: rate})
	}

	var existing int64
	s.db.Model(&models.FarmPool{}).Where("key = ?", key).Count(&existing)
	if existing > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "a farm with this key already exists"})
		return
	}

	farm := models.FarmPool{
		Key:                  key,
		Name:                 name,
		FarmType:             farmType,
		ChainID:              req.ChainID,
		StakingTokenAddress:  stakingKey,
		StakingTokenSymbol:   strings.ToUpper(strings.TrimSpace(req.StakingTokenSymbol)),
		StakingTokenDecimals: req.StakingTokenDecimals,
		BoostGaugeID:         gaugeID,
		TotalStaked:          "0",
		StartTime:            startTime,
		EndTime:              endTime,
		Status:               models.FarmStatusPaused,
		CreatedBy:            c.GetString("user_id"),
	}

	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&farm).Error; err != nil {
			return err
		}
		for _, in := range inputs {
			reward := models.FarmReward{
				FarmID:            farm.ID,
				TokenAddress:      in.key,
				TokenSymbol:       in.symbol,
				TokenDecimals:     in.decimals,
				RatePerSecond:     in.rate.String(),
				AccRewardPerShare: "0",
				LastUpdateTime:    startTime,
				TotalReward:       "0",
				TotalDistributed:  "0",
			}
			if err := tx.Create(&reward).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create farm"})
		return
	}

	logListingAction(c, s.db, "create_farm", "farm_pool", farm.ID.String(), gin.H{"key": farm.Key, "type": farm.FarmType})
	c.JSON(http.StatusCreated, gin.H{"success": true, "farm": farm})
}

// UpdateFarm applies admin edits to a farm's mutable parameters.
func (s *FarmService) UpdateFarm(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid farm id"})
		return
	}
	var req struct {
		Name         *string `json:"name"`
		FarmType     *string `json:"farm_type"`
		StartTime    *string `json:"start_time"`
		EndTime      *string `json:"end_time"`
		Status       *string `json:"status"`
		BoostGaugeID *string `json:"boost_gauge_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequestError(c, err)
		return
	}

	farm, err := s.loadFarm(id)
	if err != nil {
		if errors.Is(err, ErrListingNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "farm not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load farm"})
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
	if req.FarmType != nil {
		farmType := strings.ToLower(strings.TrimSpace(*req.FarmType))
		if !allowedFarmTypes[farmType] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "farm_type must be lp, single or stable"})
			return
		}
		updates["farm_type"] = farmType
	}
	start, end := farm.StartTime, farm.EndTime
	if req.StartTime != nil {
		parsed, err := parseListingTime(*req.StartTime)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("start_time: %v", err)})
			return
		}
		start = parsed
		updates["start_time"] = parsed
	}
	if req.EndTime != nil {
		parsed, err := parseListingTime(*req.EndTime)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("end_time: %v", err)})
			return
		}
		end = parsed
		updates["end_time"] = parsed
	}
	if !end.After(start) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "end_time must be after start_time"})
		return
	}
	if req.Status != nil {
		status := strings.ToLower(strings.TrimSpace(*req.Status))
		if !allowedFarmStatuses[status] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "status must be active, paused or closed"})
			return
		}
		updates["status"] = status
	}
	if req.BoostGaugeID != nil {
		raw := strings.TrimSpace(*req.BoostGaugeID)
		if raw == "" {
			updates["boost_gauge_id"] = nil
		} else {
			parsed, err := uuid.Parse(raw)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "boost_gauge_id is not a valid id"})
				return
			}
			var gauge models.Gauge
			if err := s.db.First(&gauge, "id = ?", parsed).Error; err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "boost_gauge_id does not match an existing gauge"})
				return
			}
			updates["boost_gauge_id"] = parsed
		}
	}

	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no updatable fields were provided"})
		return
	}
	if err := s.db.Model(&models.FarmPool{}).Where("id = ?", farm.ID).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update farm"})
		return
	}

	logListingAction(c, s.db, "update_farm", "farm_pool", farm.ID.String(), updates)
	updated, err := s.loadFarm(farm.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to reload farm"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "farm": updated})
}

// DeleteFarm closes a farm. A farm with stake is not removed: the stake must be
// withdrawn first so a staker's escrow cannot be stranded.
func (s *FarmService) DeleteFarm(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid farm id"})
		return
	}
	farm, err := s.loadFarm(id)
	if err != nil {
		if errors.Is(err, ErrListingNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "farm not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load farm"})
		return
	}

	total, ok := new(big.Int).SetString(farm.TotalStaked, 10)
	if !ok {
		total = big.NewInt(0)
	}
	if total.Sign() > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "farm still has stake; it can only be closed, not deleted"})
		return
	}
	if err := s.db.Model(&models.FarmPool{}).Where("id = ?", farm.ID).
		Update("status", models.FarmStatusClosed).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to close farm"})
		return
	}
	logListingAction(c, s.db, "delete_farm", "farm_pool", farm.ID.String(), gin.H{"status": models.FarmStatusClosed})
	c.JSON(http.StatusOK, gin.H{"success": true, "status": models.FarmStatusClosed})
}

// GetFarms lists every farm with its reward streams and computed APR.
func (s *FarmService) GetFarms(c *gin.Context) {
	farms := []models.FarmPool{}
	if err := s.db.Order("created_at desc").Find(&farms).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list farms"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "farms": s.buildFarmViews(c.Request.Context(), farms)})
}

// GetActiveFarms lists farms a user can currently stake in.
func (s *FarmService) GetActiveFarms(c *gin.Context) {
	now := time.Now()
	farms := []models.FarmPool{}
	if err := s.db.Where("status = ? AND start_time <= ? AND end_time > ?",
		models.FarmStatusActive, now, now).Order("created_at desc").Find(&farms).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list farms"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "farms": s.buildFarmViews(c.Request.Context(), farms)})
}

// GetFarm returns one farm.
func (s *FarmService) GetFarm(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid farm id"})
		return
	}
	farm, err := s.loadFarm(id)
	if err != nil {
		if errors.Is(err, ErrListingNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "farm not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load farm"})
		return
	}
	views := s.buildFarmViews(c.Request.Context(), []models.FarmPool{*farm})
	c.JSON(http.StatusOK, gin.H{"success": true, "farm": views[0]})
}

// FarmView is a farm with its reward streams and, when prices are available,
// its annualised return. APR is null rather than zero when a price is missing,
// because an absent price is not a 0% yield.
type FarmView struct {
	Farm    models.FarmPool     `json:"farm"`
	Rewards []models.FarmReward `json:"rewards"`
	APR     *float64            `json:"apr"`
}

func (s *FarmService) buildFarmViews(ctx context.Context, farms []models.FarmPool) []FarmView {
	views := make([]FarmView, 0, len(farms))
	for _, farm := range farms {
		rewards := []models.FarmReward{}
		if err := s.db.Where("farm_id = ?", farm.ID).Order("created_at asc").Find(&rewards).Error; err != nil {
			rewards = []models.FarmReward{}
		}
		views = append(views, FarmView{
			Farm:    farm,
			Rewards: rewards,
			APR:     s.farmAPR(ctx, &farm, rewards),
		})
	}
	return views
}

// farmAPR computes the farm's annual percentage return from the emission rates
// and the oracle prices of the staking and reward tokens. It returns nil when
// any price is unavailable, so the UI shows "—" instead of a fabricated number.
func (s *FarmService) farmAPR(ctx context.Context, farm *models.FarmPool, rewards []models.FarmReward) *float64 {
	if s.prices == nil {
		return nil
	}
	totalStaked, ok := new(big.Int).SetString(farm.TotalStaked, 10)
	if !ok || totalStaked.Sign() <= 0 {
		return nil
	}
	stakePrice, err := s.prices.GetRealPrice(ctx, strings.ToUpper(farm.StakingTokenSymbol))
	if err != nil || stakePrice == nil || stakePrice.USD <= 0 {
		return nil
	}
	stakedValue, _ := scaleAmount(totalStaked, farm.StakingTokenDecimals).Float64()
	stakedUSD := stakedValue * stakePrice.USD
	if stakedUSD <= 0 {
		return nil
	}

	yearSeconds := 365 * 24 * 60 * 60
	rewardUSD := 0.0
	for _, reward := range rewards {
		rate, ok := new(big.Int).SetString(reward.RatePerSecond, 10)
		if !ok || rate.Sign() <= 0 {
			continue
		}
		price, err := s.prices.GetRealPrice(ctx, strings.ToUpper(reward.TokenSymbol))
		if err != nil || price == nil || price.USD <= 0 {
			return nil
		}
		rateValue, _ := scaleAmount(rate, reward.TokenDecimals).Float64()
		rewardUSD += rateValue * float64(yearSeconds) * price.USD
	}
	apr := rewardUSD / stakedUSD
	if apr < 0 {
		apr = 0
	}
	return &apr
}

// FundFarmReward moves reward tokens from the caller's ledger balance into the
// farm's escrow, raising the emission budget for one stream. Funding is a real
// debit: a farm can never pay out rewards that were not funded.
func (s *FarmService) FundFarmReward(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid farm id"})
		return
	}
	var req struct {
		TokenAddress string `json:"token_address" binding:"required"`
		Amount       string `json:"amount" binding:"required"`
	}
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

	farm, err := s.loadFarm(id)
	if err != nil {
		if errors.Is(err, ErrListingNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "farm not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load farm"})
		return
	}
	tokenKey, err := tokenKeyOrNative(req.TokenAddress)
	if err != nil {
		respondBadRequestError(c, err)
		return
	}
	amount, err := parseTokenAmount(req.Amount)
	if err != nil || amount.Sign() <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "amount must be a positive integer"})
		return
	}

	var reward models.FarmReward
	if err := s.db.Where("farm_id = ? AND token_address = ?", farm.ID, tokenKey).First(&reward).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "this farm has no reward stream for that token"})
		return
	}

	// The debit and the escrow update commit together. If either fails the
	// transaction rolls back, so a funding attempt can never consume funds
	// without raising the stream's budget, nor raise the budget without
	// consuming funds.
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		var locked models.FarmReward
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&locked, "id = ?", reward.ID).Error; err != nil {
			return err
		}
		if err := s.ledger.debitWithin(tx, userID, farm.ChainID, tokenKey, amount); err != nil {
			return err
		}
		current, _ := new(big.Int).SetString(locked.TotalReward, 10)
		if current == nil {
			current = big.NewInt(0)
		}
		return tx.Model(&models.FarmReward{}).Where("id = ?", locked.ID).
			Update("total_reward", new(big.Int).Add(current, amount).String()).Error
	}); err != nil {
		if errors.Is(err, ErrInsufficientBalance) {
			c.JSON(http.StatusConflict, gin.H{"error": "insufficient balance to fund this reward stream"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to record the funding"})
		return
	}

	logListingAction(c, s.db, "fund_farm_reward", "farm_reward", reward.ID.String(),
		gin.H{"farm_id": farm.ID.String(), "token": tokenKey, "amount": amount.String()})
	c.JSON(http.StatusOK, gin.H{"success": true, "farm_id": farm.ID, "token": tokenKey, "funded": amount.String()})
}

// loadOrCreateStake reads the caller's stake in a farm, creating an empty one
// when it is absent. It must run inside a transaction.
func loadOrCreateStake(tx *gorm.DB, farmID uuid.UUID, userID string) (*models.FarmStake, error) {
	var stake models.FarmStake
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("farm_id = ? AND user_id = ?", farmID, userID).First(&stake).Error
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		stake = models.FarmStake{FarmID: farmID, UserID: userID, Amount: "0"}
		if err := tx.Create(&stake).Error; err != nil {
			return nil, err
		}
		return &stake, nil
	case err != nil:
		return nil, err
	}
	return &stake, nil
}

// boostBpsFor returns the caller's boost in basis points. When no boost source
// is wired the staker gets the 1x base, never a fabricated multiplier.
func (s *FarmService) boostBpsFor(userID string, amount *big.Int) *big.Int {
	if s.boost == nil {
		return big.NewInt(10000)
	}
	bps, err := s.boost.BoostBps(userID, amount)
	if err != nil || bps == nil || bps.Sign() <= 0 {
		return big.NewInt(10000)
	}
	if bps.Cmp(big.NewInt(10000)) < 0 {
		return big.NewInt(10000)
	}
	return bps
}

// effectiveAmount applies a boost in basis points to a staked amount.
func effectiveAmount(amount, boostBps *big.Int) *big.Int {
	return new(big.Int).Div(new(big.Int).Mul(amount, boostBps), big.NewInt(10000))
}

// pendingFor returns the reward a stake has accrued since its last checkpoint.
func pendingFor(effective, acc, debt *big.Int) *big.Int {
	share := new(big.Int).Div(new(big.Int).Mul(effective, acc), accScale)
	pending := new(big.Int).Sub(share, debt)
	if pending.Sign() < 0 {
		return big.NewInt(0)
	}
	return pending
}

// rewardStep is the pure part of the accumulator update: it returns the new
// accumulator, the new distributed total and the emission for the interval.
// advanceReward persists the result; a read-only caller uses it to project the
// pending reward without mutating the stream.
func rewardStep(reward *models.FarmReward, totalStaked *big.Int, now, endTime time.Time) (acc, distributed *big.Int, elapsed int64) {
	acc, _ = new(big.Int).SetString(reward.AccRewardPerShare, 10)
	if acc == nil {
		acc = big.NewInt(0)
	}
	distributed, _ = new(big.Int).SetString(reward.TotalDistributed, 10)
	if distributed == nil {
		distributed = big.NewInt(0)
	}
	if now.After(endTime) {
		now = endTime
	}
	if reward.LastUpdateTime.IsZero() {
		return acc, distributed, 0
	}
	elapsed = int64(now.Sub(reward.LastUpdateTime).Seconds())
	if elapsed <= 0 || totalStaked.Sign() <= 0 {
		return acc, distributed, 0
	}
	rate, ok := new(big.Int).SetString(reward.RatePerSecond, 10)
	if !ok || rate.Sign() <= 0 {
		return acc, distributed, 0
	}
	totalReward, _ := new(big.Int).SetString(reward.TotalReward, 10)
	if totalReward == nil {
		totalReward = big.NewInt(0)
	}
	budget := new(big.Int).Sub(totalReward, distributed)
	if budget.Sign() <= 0 {
		return acc, distributed, 0
	}
	emission := new(big.Int).Mul(rate, big.NewInt(elapsed))
	if emission.Cmp(budget) > 0 {
		emission = budget
	}
	acc = new(big.Int).Add(acc, new(big.Int).Div(new(big.Int).Mul(emission, accScale), totalStaked))
	distributed = new(big.Int).Add(distributed, emission)
	return acc, distributed, elapsed
}

// advanceReward moves a reward stream's accumulator forward to `now`, emitting
// at most the stream's remaining budget. Emissions stop at the farm's end time.
// A checkpoint is never moved backwards, so a clock skew cannot let the same
// interval emit twice.
func advanceReward(tx *gorm.DB, reward *models.FarmReward, totalStaked *big.Int, now time.Time, endTime time.Time) error {
	if now.After(endTime) {
		now = endTime
	}
	checkpoint := now
	if checkpoint.Before(reward.LastUpdateTime) {
		checkpoint = reward.LastUpdateTime
	}
	acc, distributed, _ := rewardStep(reward, totalStaked, checkpoint, endTime)
	if reward.LastUpdateTime.IsZero() || checkpoint.Equal(reward.LastUpdateTime) {
		reward.LastUpdateTime = checkpoint
		return tx.Model(&models.FarmReward{}).Where("id = ?", reward.ID).
			Update("last_update_time", checkpoint).Error
	}
	if err := tx.Model(&models.FarmReward{}).Where("id = ?", reward.ID).Updates(map[string]interface{}{
		"acc_reward_per_share": acc.String(),
		"total_distributed":    distributed.String(),
		"last_update_time":     checkpoint,
	}).Error; err != nil {
		return err
	}
	reward.AccRewardPerShare = acc.String()
	reward.TotalDistributed = distributed.String()
	reward.LastUpdateTime = checkpoint
	return nil
}

// loadOrCreateStakeReward reads the per-stake reward checkpoint for a stream.
// `created` reports whether a fresh checkpoint was inserted, which tells the
// caller to seed the debt rather than settle a pending amount.
func loadOrCreateStakeReward(tx *gorm.DB, stake *models.FarmStake, reward *models.FarmReward) (*models.FarmStakeReward, bool, error) {
	var sr models.FarmStakeReward
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("stake_id = ? AND reward_id = ?", stake.ID, reward.ID).First(&sr).Error
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		sr = models.FarmStakeReward{
			StakeID:  stake.ID,
			RewardID: reward.ID,
			FarmID:   stake.FarmID,
			UserID:   stake.UserID,
			Debt:     "0",
			Accrued:  "0",
			Claimed:  "0",
		}
		if err := tx.Create(&sr).Error; err != nil {
			return nil, false, err
		}
		return &sr, true, nil
	case err != nil:
		return nil, false, err
	}
	return &sr, false, nil
}

// accrueStake settles a stake's accrued reward across every stream of a farm.
// `oldEffective` is the stake's boosted share before an amount change and
// `newEffective` after; passing the same value for both is a plain claim. A
// checkpoint created by this call is seeded with the new debt instead of
// settling, so a new staker cannot claim reward emitted before they staked.
func (s *FarmService) accrueStake(tx *gorm.DB, farm *models.FarmPool, stake *models.FarmStake, oldEffective, newEffective *big.Int, now time.Time) error {
	rewards := []models.FarmReward{}
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("farm_id = ?", farm.ID).Order("created_at asc").Find(&rewards).Error; err != nil {
		return err
	}
	totalStaked, _ := new(big.Int).SetString(farm.TotalStaked, 10)
	if totalStaked == nil {
		totalStaked = big.NewInt(0)
	}
	for i := range rewards {
		reward := &rewards[i]
		if err := advanceReward(tx, reward, totalStaked, now, farm.EndTime); err != nil {
			return err
		}
		sr, created, err := loadOrCreateStakeReward(tx, stake, reward)
		if err != nil {
			return err
		}
		acc, _ := new(big.Int).SetString(reward.AccRewardPerShare, 10)
		if acc == nil {
			acc = big.NewInt(0)
		}
		nextDebt := new(big.Int).Div(new(big.Int).Mul(newEffective, acc), accScale)
		if created {
			if err := tx.Model(&models.FarmStakeReward{}).Where("id = ?", sr.ID).
				Update("debt", nextDebt.String()).Error; err != nil {
				return err
			}
			continue
		}
		debt, _ := new(big.Int).SetString(sr.Debt, 10)
		if debt == nil {
			debt = big.NewInt(0)
		}
		pending := pendingFor(oldEffective, acc, debt)
		accrued, _ := new(big.Int).SetString(sr.Accrued, 10)
		if accrued == nil {
			accrued = big.NewInt(0)
		}
		accrued = new(big.Int).Add(accrued, pending)
		if err := tx.Model(&models.FarmStakeReward{}).Where("id = ?", sr.ID).Updates(map[string]interface{}{
			"accrued": accrued.String(),
			"debt":    nextDebt.String(),
		}).Error; err != nil {
			return err
		}
	}
	return nil
}

// Stake deposits staking tokens into a farm. The tokens are debited from the
// caller's ledger balance, so a stake is always backed by a real holding.
func (s *FarmService) Stake(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid farm id"})
		return
	}
	var req struct {
		Amount string `json:"amount" binding:"required"`
	}
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
	amount, err := parseTokenAmount(req.Amount)
	if err != nil || amount.Sign() <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "amount must be a positive integer"})
		return
	}

	farm, err := s.loadFarm(id)
	if err != nil {
		if errors.Is(err, ErrListingNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "farm not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load farm"})
		return
	}
	now := time.Now()
	if farm.Status != models.FarmStatusActive {
		c.JSON(http.StatusConflict, gin.H{"error": "farm is not active"})
		return
	}
	if now.Before(farm.StartTime) {
		c.JSON(http.StatusConflict, gin.H{"error": "farm has not started"})
		return
	}
	if !now.Before(farm.EndTime) {
		c.JSON(http.StatusConflict, gin.H{"error": "farm has ended"})
		return
	}

	// The debit, the stake and the farm total commit together. The farm is
	// re-read under lock inside the transaction, so a farm paused or ended
	// between the check above and this point is still rejected rather than
	// accepting a stake that can no longer be settled.
	err = s.db.Transaction(func(tx *gorm.DB) error {
		var lockedFarm models.FarmPool
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&lockedFarm, "id = ?", farm.ID).Error; err != nil {
			return err
		}
		if lockedFarm.Status != models.FarmStatusActive || now.Before(lockedFarm.StartTime) || !now.Before(lockedFarm.EndTime) {
			return errFarmNotOpen
		}
		if err := s.ledger.debitWithin(tx, userID, lockedFarm.ChainID, lockedFarm.StakingTokenAddress, amount); err != nil {
			return err
		}
		stake, err := loadOrCreateStake(tx, lockedFarm.ID, userID)
		if err != nil {
			return err
		}
		oldAmount, _ := new(big.Int).SetString(stake.Amount, 10)
		if oldAmount == nil {
			oldAmount = big.NewInt(0)
		}
		newAmount := new(big.Int).Add(oldAmount, amount)
		oldBoost := s.boostBpsFor(userID, oldAmount)
		newBoost := s.boostBpsFor(userID, newAmount)
		if err := s.accrueStake(tx, &lockedFarm, stake, effectiveAmount(oldAmount, oldBoost), effectiveAmount(newAmount, newBoost), now); err != nil {
			return err
		}
		if err := tx.Model(&models.FarmStake{}).Where("id = ?", stake.ID).
			Update("amount", newAmount.String()).Error; err != nil {
			return err
		}
		totalStaked, _ := new(big.Int).SetString(lockedFarm.TotalStaked, 10)
		if totalStaked == nil {
			totalStaked = big.NewInt(0)
		}
		totalStaked = new(big.Int).Add(totalStaked, amount)
		return tx.Model(&models.FarmPool{}).Where("id = ?", lockedFarm.ID).
			Update("total_staked", totalStaked.String()).Error
	})
	if errors.Is(err, ErrInsufficientBalance) {
		c.JSON(http.StatusConflict, gin.H{"error": "insufficient staking token balance"})
		return
	}
	if errors.Is(err, errFarmNotOpen) {
		c.JSON(http.StatusConflict, gin.H{"error": "farm is not open for staking"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to record the stake"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "farm_id": farm.ID, "staked": amount.String()})
}

// Unstake withdraws staking tokens from a farm and settles accrued rewards.
func (s *FarmService) Unstake(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid farm id"})
		return
	}
	var req struct {
		Amount string `json:"amount" binding:"required"`
	}
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
	amount, err := parseTokenAmount(req.Amount)
	if err != nil || amount.Sign() <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "amount must be a positive integer"})
		return
	}

	farm, err := s.loadFarm(id)
	if err != nil {
		if errors.Is(err, ErrListingNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "farm not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load farm"})
		return
	}

	now := time.Now()
	err = s.db.Transaction(func(tx *gorm.DB) error {
		var lockedFarm models.FarmPool
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&lockedFarm, "id = ?", farm.ID).Error; err != nil {
			return err
		}
		var stake models.FarmStake
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("farm_id = ? AND user_id = ?", lockedFarm.ID, userID).First(&stake).Error; err != nil {
			return err
		}
		current, _ := new(big.Int).SetString(stake.Amount, 10)
		if current == nil {
			current = big.NewInt(0)
		}
		if current.Cmp(amount) < 0 {
			return ErrInsufficientBalance
		}
		newAmount := new(big.Int).Sub(current, amount)
		oldBoost := s.boostBpsFor(userID, current)
		newBoost := s.boostBpsFor(userID, newAmount)
		if err := s.accrueStake(tx, &lockedFarm, &stake, effectiveAmount(current, oldBoost), effectiveAmount(newAmount, newBoost), now); err != nil {
			return err
		}
		if err := tx.Model(&models.FarmStake{}).Where("id = ?", stake.ID).
			Update("amount", newAmount.String()).Error; err != nil {
			return err
		}
		totalStaked, _ := new(big.Int).SetString(lockedFarm.TotalStaked, 10)
		if totalStaked == nil {
			totalStaked = big.NewInt(0)
		}
		totalStaked = new(big.Int).Sub(totalStaked, amount)
		if totalStaked.Sign() < 0 {
			totalStaked = big.NewInt(0)
		}
		if err := tx.Model(&models.FarmPool{}).Where("id = ?", lockedFarm.ID).
			Update("total_staked", totalStaked.String()).Error; err != nil {
			return err
		}
		// The payout commits with the reduction of the stake, so tokens are never
		// credited while still recorded as staked, nor removed from the stake
		// without being credited.
		return s.ledger.creditWithin(tx, userID, lockedFarm.ChainID, lockedFarm.StakingTokenAddress,
			lockedFarm.StakingTokenSymbol, lockedFarm.StakingTokenDecimals, amount)
	})
	if errors.Is(err, ErrInsufficientBalance) {
		c.JSON(http.StatusConflict, gin.H{"error": "unstake amount exceeds the staked balance"})
		return
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "no stake found in this farm"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to unstake"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "farm_id": farm.ID, "unstaked": amount.String()})
}

// Claim settles accrued reward and credits it to the caller's ledger balance.
func (s *FarmService) Claim(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid farm id"})
		return
	}
	var req struct {
		TokenAddress string `json:"token_address"`
	}
	// An empty body claims every stream; the binding error is ignored for that
	// reason rather than treating a missing body as invalid input.
	_ = c.ShouldBindJSON(&req)

	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}
	if s.ledger == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "ledger is not configured"})
		return
	}

	farm, err := s.loadFarm(id)
	if err != nil {
		if errors.Is(err, ErrListingNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "farm not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load farm"})
		return
	}
	var tokenFilter string
	if raw := strings.TrimSpace(req.TokenAddress); raw != "" {
		tokenFilter, err = tokenKeyOrNative(raw)
		if err != nil {
			respondBadRequestError(c, err)
			return
		}
	}

	now := time.Now()
	type payout struct {
		address  string
		symbol   string
		decimals int
		amount   *big.Int
	}
	payouts := []payout{}

	err = s.db.Transaction(func(tx *gorm.DB) error {
		var lockedFarm models.FarmPool
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&lockedFarm, "id = ?", farm.ID).Error; err != nil {
			return err
		}
		var stake models.FarmStake
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("farm_id = ? AND user_id = ?", lockedFarm.ID, userID).First(&stake).Error; err != nil {
			return err
		}
		amount, _ := new(big.Int).SetString(stake.Amount, 10)
		if amount == nil {
			amount = big.NewInt(0)
		}
		effective := effectiveAmount(amount, s.boostBpsFor(userID, amount))
		if err := s.accrueStake(tx, &lockedFarm, &stake, effective, effective, now); err != nil {
			return err
		}

		rows := []models.FarmStakeReward{}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("stake_id = ?", stake.ID).Find(&rows).Error; err != nil {
			return err
		}
		for i := range rows {
			row := &rows[i]
			var reward models.FarmReward
			if err := tx.First(&reward, "id = ?", row.RewardID).Error; err != nil {
				return err
			}
			if tokenFilter != "" && reward.TokenAddress != tokenFilter {
				continue
			}
			accrued, _ := new(big.Int).SetString(row.Accrued, 10)
			if accrued == nil || accrued.Sign() <= 0 {
				continue
			}
			claimed, _ := new(big.Int).SetString(row.Claimed, 10)
			if claimed == nil {
				claimed = big.NewInt(0)
			}
			if err := tx.Model(&models.FarmStakeReward{}).Where("id = ?", row.ID).Updates(map[string]interface{}{
				"accrued": "0",
				"claimed": new(big.Int).Add(claimed, accrued).String(),
			}).Error; err != nil {
				return err
			}
			// The credit commits with the settlement: the accrued reward is only
			// zeroed once the balance has moved, so a crash mid-claim cannot
			// destroy a reward.
			if err := s.ledger.creditWithin(tx, userID, lockedFarm.ChainID, reward.TokenAddress,
				reward.TokenSymbol, reward.TokenDecimals, accrued); err != nil {
				return err
			}
			payouts = append(payouts, payout{address: reward.TokenAddress, symbol: reward.TokenSymbol, decimals: reward.TokenDecimals, amount: accrued})
		}
		return nil
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "no stake found in this farm"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to settle rewards"})
		return
	}
	if len(payouts) == 0 {
		c.JSON(http.StatusOK, gin.H{"success": true, "claimed": []interface{}{}, "message": "nothing to claim"})
		return
	}

	claimed := make([]gin.H, 0, len(payouts))
	for _, p := range payouts {
		claimed = append(claimed, gin.H{"token_address": p.address, "symbol": p.symbol, "amount": p.amount.String()})
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "claimed": claimed})
}

// GetStakes returns the caller's positions across every farm, with pending
// reward per stream.
func (s *FarmService) GetStakes(c *gin.Context) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}
	now := time.Now()

	stakes := []models.FarmStake{}
	if err := s.db.Where("user_id = ?", userID).Find(&stakes).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load stakes"})
		return
	}

	views := make([]gin.H, 0, len(stakes))
	for _, stake := range stakes {
		var farm models.FarmPool
		if err := s.db.First(&farm, "id = ?", stake.FarmID).Error; err != nil {
			continue
		}
		amount, _ := new(big.Int).SetString(stake.Amount, 10)
		if amount == nil {
			amount = big.NewInt(0)
		}
		effective := effectiveAmount(amount, s.boostBpsFor(userID, amount))

		totalStaked, _ := new(big.Int).SetString(farm.TotalStaked, 10)
		if totalStaked == nil {
			totalStaked = big.NewInt(0)
		}

		rows := []models.FarmStakeReward{}
		if err := s.db.Where("stake_id = ?", stake.ID).Find(&rows).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load stake rewards"})
			return
		}
		pending := make([]gin.H, 0, len(rows))
		for _, row := range rows {
			var reward models.FarmReward
			if err := s.db.First(&reward, "id = ?", row.RewardID).Error; err != nil {
				continue
			}
			debt, _ := new(big.Int).SetString(row.Debt, 10)
			if debt == nil {
				debt = big.NewInt(0)
			}
			accrued, _ := new(big.Int).SetString(row.Accrued, 10)
			if accrued == nil {
				accrued = big.NewInt(0)
			}
			// Project the accumulator to now without mutating the stream, so the
			// displayed pending figure includes emissions since the last
			// checkpoint. The projection stops at the farm's end time and can
			// never exceed the stream's remaining budget.
			acc, _, _ := rewardStep(&reward, totalStaked, now, farm.EndTime)
			accrued = new(big.Int).Add(accrued, pendingFor(effective, acc, debt))
			pending = append(pending, gin.H{
				"token_address": reward.TokenAddress,
				"symbol":        reward.TokenSymbol,
				"decimals":      reward.TokenDecimals,
				"pending":       accrued.String(),
			})
		}

		views = append(views, gin.H{
			"stake_id":              stake.ID,
			"farm_id":               farm.ID,
			"farm_key":              farm.Key,
			"farm_name":             farm.Name,
			"farm_type":             farm.FarmType,
			"status":                farm.Status,
			"chain_id":              farm.ChainID,
			"staking_token_address": farm.StakingTokenAddress,
			"staking_token_symbol":  farm.StakingTokenSymbol,
			"staked":                amount.String(),
			"end_time":              farm.EndTime,
			"pending_rewards":       pending,
			"checked_at":            now,
		})
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "stakes": views})
}
