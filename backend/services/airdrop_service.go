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
	"gorm.io/gorm/clause"

	"tigerswap/backend/models"
)

// AirdropService implements admin-managed token distributions. An allocation is
// an explicit per-user entitlement; a claim credits the ledger only up to the
// campaign's funded escrow, so a campaign can never distribute tokens that were
// not funded.
type AirdropService struct {
	db     *gorm.DB
	ledger *LedgerService
}

func NewAirdropService(db *gorm.DB, ledger *LedgerService) *AirdropService {
	return &AirdropService{db: db, ledger: ledger}
}

var allowedAirdropStatuses = map[string]bool{
	models.AirdropStatusActive: true,
	models.AirdropStatusPaused: true,
	models.AirdropStatusClosed: true,
}

func (s *AirdropService) loadCampaign(id uuid.UUID) (*models.AirdropCampaign, error) {
	var campaign models.AirdropCampaign
	if err := s.db.First(&campaign, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrListingNotFound
		}
		return nil, err
	}
	return &campaign, nil
}

// CreateCampaign opens an airdrop campaign. It starts paused; an operator funds
// it and then activates it.
func (s *AirdropService) CreateCampaign(c *gin.Context) {
	var req struct {
		Key           string `json:"key" binding:"required"`
		Name          string `json:"name" binding:"required"`
		ChainID       int64  `json:"chain_id" binding:"required"`
		TokenAddress  string `json:"token_address" binding:"required"`
		TokenSymbol   string `json:"token_symbol" binding:"required"`
		TokenDecimals int    `json:"token_decimals"`
		StartTime     string `json:"start_time" binding:"required"`
		EndTime       string `json:"end_time" binding:"required"`
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
	if req.TokenDecimals < 0 || req.TokenDecimals > 36 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "token_decimals must be between 0 and 36"})
		return
	}
	tokenKey, err := tokenKeyOrNative(req.TokenAddress)
	if err != nil {
		respondBadRequestError(c, err)
		return
	}
	symbol := strings.ToUpper(strings.TrimSpace(req.TokenSymbol))
	if symbol == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "token_symbol is required"})
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

	var existing int64
	s.db.Model(&models.AirdropCampaign{}).Where("key = ?", key).Count(&existing)
	if existing > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "an airdrop campaign with this key already exists"})
		return
	}

	campaign := models.AirdropCampaign{
		Key:            key,
		Name:           name,
		ChainID:        req.ChainID,
		TokenAddress:   tokenKey,
		TokenSymbol:    symbol,
		TokenDecimals:  req.TokenDecimals,
		TotalAllocated: "0",
		Funded:         "0",
		TotalClaimed:   "0",
		StartTime:      startTime,
		EndTime:        endTime,
		Status:         models.AirdropStatusPaused,
		CreatedBy:      c.GetString("user_id"),
	}
	if err := s.db.Create(&campaign).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create airdrop campaign"})
		return
	}
	logListingAction(c, s.db, "create_airdrop", "airdrop_campaign", campaign.ID.String(), gin.H{"key": campaign.Key})
	c.JSON(http.StatusCreated, gin.H{"success": true, "campaign": campaign})
}

// UpdateCampaign applies admin edits and can replace the allocation set. A
// replacement is refused once any allocation has been claimed, so a claim can
// never be invalidated retroactively.
func (s *AirdropService) UpdateCampaign(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid campaign id"})
		return
	}
	var req struct {
		Name      *string `json:"name"`
		StartTime *string `json:"start_time"`
		EndTime   *string `json:"end_time"`
		Status    *string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequestError(c, err)
		return
	}
	campaign, err := s.loadCampaign(id)
	if err != nil {
		if errors.Is(err, ErrListingNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "airdrop campaign not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load campaign"})
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
	start, end := campaign.StartTime, campaign.EndTime
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
		if !allowedAirdropStatuses[status] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "status must be active, paused or closed"})
			return
		}
		updates["status"] = status
	}
	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no updatable fields were provided"})
		return
	}
	if err := s.db.Model(&models.AirdropCampaign{}).Where("id = ?", campaign.ID).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update campaign"})
		return
	}
	logListingAction(c, s.db, "update_airdrop", "airdrop_campaign", campaign.ID.String(), updates)
	updated, err := s.loadCampaign(campaign.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to reload campaign"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "campaign": updated})
}

// SetAllocations replaces the campaign's allocation set. Existing claims block a
// replacement, because removing an allocation a user already claimed from would
// make the campaign's accounting inconsistent with its payouts.
func (s *AirdropService) SetAllocations(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid campaign id"})
		return
	}
	var req struct {
		Allocations []struct {
			UserID string `json:"user_id" binding:"required"`
			Amount string `json:"amount" binding:"required"`
		} `json:"allocations" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequestError(c, err)
		return
	}
	if len(req.Allocations) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "at least one allocation is required"})
		return
	}

	campaign, err := s.loadCampaign(id)
	if err != nil {
		if errors.Is(err, ErrListingNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "airdrop campaign not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load campaign"})
		return
	}

	var claimed int64
	s.db.Model(&models.AirdropAllocation{}).Where("campaign_id = ? AND claimed = ?", campaign.ID, true).Count(&claimed)
	if claimed > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "allocations cannot be replaced after a claim has been made"})
		return
	}

	type entry struct {
		userID string
		amount *big.Int
	}
	entries := make([]entry, 0, len(req.Allocations))
	seen := make(map[string]bool, len(req.Allocations))
	total := big.NewInt(0)
	for _, item := range req.Allocations {
		userID := strings.TrimSpace(item.UserID)
		if userID == "" || len(userID) > 100 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "each allocation needs a user_id of at most 100 characters"})
			return
		}
		if seen[userID] {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("duplicate allocation for user %s", userID)})
			return
		}
		seen[userID] = true
		amount, err := parseTokenAmount(item.Amount)
		if err != nil || amount.Sign() <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("allocation for %s must be a positive integer", userID)})
			return
		}
		total = new(big.Int).Add(total, amount)
		entries = append(entries, entry{userID: userID, amount: amount})
	}

	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("campaign_id = ?", campaign.ID).Delete(&models.AirdropAllocation{}).Error; err != nil {
			return err
		}
		for _, e := range entries {
			allocation := models.AirdropAllocation{
				CampaignID: campaign.ID,
				UserID:     e.userID,
				Amount:     e.amount.String(),
				Claimed:    false,
			}
			if err := tx.Create(&allocation).Error; err != nil {
				return err
			}
		}
		return tx.Model(&models.AirdropCampaign{}).Where("id = ?", campaign.ID).
			Update("total_allocated", total.String()).Error
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to record allocations"})
		return
	}

	logListingAction(c, s.db, "set_airdrop_allocations", "airdrop_campaign", campaign.ID.String(),
		gin.H{"count": len(entries), "total": total.String()})
	c.JSON(http.StatusOK, gin.H{"success": true, "campaign_id": campaign.ID, "count": len(entries), "total_allocated": total.String()})
}

// FundCampaign moves tokens from the caller's ledger balance into the
// campaign's escrow. A claim can only pay out of funded escrow.
func (s *AirdropService) FundCampaign(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid campaign id"})
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
	campaign, err := s.loadCampaign(id)
	if err != nil {
		if errors.Is(err, ErrListingNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "airdrop campaign not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load campaign"})
		return
	}

	// The debit and the escrow update commit together: funding can never consume
	// the funder's balance without raising the campaign's funded amount.
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		var locked models.AirdropCampaign
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&locked, "id = ?", campaign.ID).Error; err != nil {
			return err
		}
		if err := s.ledger.debitWithin(tx, userID, locked.ChainID, locked.TokenAddress, amount); err != nil {
			return err
		}
		funded, _ := new(big.Int).SetString(locked.Funded, 10)
		if funded == nil {
			funded = big.NewInt(0)
		}
		return tx.Model(&models.AirdropCampaign{}).Where("id = ?", locked.ID).
			Update("funded", new(big.Int).Add(funded, amount).String()).Error
	}); err != nil {
		if errors.Is(err, ErrInsufficientBalance) {
			c.JSON(http.StatusConflict, gin.H{"error": "insufficient balance to fund this campaign"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to record the funding"})
		return
	}

	logListingAction(c, s.db, "fund_airdrop", "airdrop_campaign", campaign.ID.String(), gin.H{"amount": amount.String()})
	c.JSON(http.StatusOK, gin.H{"success": true, "campaign_id": campaign.ID, "funded": amount.String()})
}

// ClaimAirdrop credits the caller's allocation for a campaign.
func (s *AirdropService) ClaimAirdrop(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid campaign id"})
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
	campaign, err := s.loadCampaign(id)
	if err != nil {
		if errors.Is(err, ErrListingNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "airdrop campaign not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load campaign"})
		return
	}

	now := time.Now()
	if campaign.Status != models.AirdropStatusActive {
		c.JSON(http.StatusConflict, gin.H{"error": "airdrop campaign is not active"})
		return
	}
	if now.Before(campaign.StartTime) {
		c.JSON(http.StatusConflict, gin.H{"error": "airdrop claim window has not opened"})
		return
	}
	if !now.Before(campaign.EndTime) {
		c.JSON(http.StatusConflict, gin.H{"error": "airdrop claim window has closed"})
		return
	}

	var credited *big.Int
	err = s.db.Transaction(func(tx *gorm.DB) error {
		var lockedCampaign models.AirdropCampaign
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&lockedCampaign, "id = ?", campaign.ID).Error; err != nil {
			return err
		}
		// Re-check the window against the row re-read under lock, so an admin
		// pausing or ending the campaign concurrently cannot be raced into a
		// payout that the earlier read allowed.
		if lockedCampaign.Status != models.AirdropStatusActive || now.Before(lockedCampaign.StartTime) || !now.Before(lockedCampaign.EndTime) {
			return errCampaignNotOpen
		}
		var allocation models.AirdropAllocation
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("campaign_id = ? AND user_id = ?", lockedCampaign.ID, userID).First(&allocation).Error; err != nil {
			return err
		}
		if allocation.Claimed {
			return errAlreadyClaimed
		}
		amount, _ := new(big.Int).SetString(allocation.Amount, 10)
		if amount == nil || amount.Sign() <= 0 {
			return errNoAllocation
		}

		// The claim may not exceed the remaining funded escrow. This keeps a
		// campaign's payouts bounded by what was actually funded.
		funded, _ := new(big.Int).SetString(lockedCampaign.Funded, 10)
		if funded == nil {
			funded = big.NewInt(0)
		}
		claimed, _ := new(big.Int).SetString(lockedCampaign.TotalClaimed, 10)
		if claimed == nil {
			claimed = big.NewInt(0)
		}
		remaining := new(big.Int).Sub(funded, claimed)
		if remaining.Cmp(amount) < 0 {
			return errInsufficientEscrow
		}

		if err := tx.Model(&models.AirdropAllocation{}).Where("id = ?", allocation.ID).
			Update("claimed", true).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.AirdropCampaign{}).Where("id = ?", lockedCampaign.ID).
			Update("total_claimed", new(big.Int).Add(claimed, amount).String()).Error; err != nil {
			return err
		}
		// The credit commits with the claim flag: an allocation can never be
		// marked claimed without the tokens moving, nor paid twice.
		if err := s.ledger.creditWithin(tx, userID, lockedCampaign.ChainID, lockedCampaign.TokenAddress,
			lockedCampaign.TokenSymbol, lockedCampaign.TokenDecimals, amount); err != nil {
			return err
		}
		credited = amount
		return nil
	})
	switch {
	case errors.Is(err, errAlreadyClaimed):
		c.JSON(http.StatusConflict, gin.H{"error": "allocation has already been claimed"})
		return
	case errors.Is(err, errNoAllocation):
		c.JSON(http.StatusNotFound, gin.H{"error": "no allocation found for this campaign"})
		return
	case errors.Is(err, errInsufficientEscrow):
		c.JSON(http.StatusConflict, gin.H{"error": "campaign escrow is insufficient for this allocation"})
		return
	case errors.Is(err, errCampaignNotOpen):
		c.JSON(http.StatusConflict, gin.H{"error": "airdrop campaign is not open for claims"})
		return
	case errors.Is(err, gorm.ErrRecordNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "no allocation found for this campaign"})
		return
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to settle the airdrop claim"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"campaign_id":   campaign.ID,
		"token_address": campaign.TokenAddress,
		"symbol":        campaign.TokenSymbol,
		"amount":        credited.String(),
	})
}

var (
	errAlreadyClaimed     = errors.New("allocation already claimed")
	errNoAllocation       = errors.New("no allocation")
	errInsufficientEscrow = errors.New("insufficient escrow")
	// errCampaignNotOpen is returned when a claim is attempted on a campaign that
	// is paused, has not started, or has ended. It is checked against the campaign
	// re-read under lock so a concurrent admin change cannot be raced.
	errCampaignNotOpen = errors.New("airdrop campaign is not open for claims")
)

// GetCampaigns lists campaigns with their aggregate allocation state.
func (s *AirdropService) GetCampaigns(c *gin.Context) {
	campaigns := []models.AirdropCampaign{}
	if err := s.db.Order("created_at desc").Find(&campaigns).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list airdrop campaigns"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "campaigns": campaigns})
}

// GetActiveCampaigns lists campaigns a user can currently claim from.
func (s *AirdropService) GetActiveCampaigns(c *gin.Context) {
	now := time.Now()
	campaigns := []models.AirdropCampaign{}
	if err := s.db.Where("status = ? AND start_time <= ? AND end_time > ?",
		models.AirdropStatusActive, now, now).Order("created_at desc").Find(&campaigns).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list airdrop campaigns"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "campaigns": campaigns})
}

// GetMyAllocations returns the caller's entitlements across every campaign.
func (s *AirdropService) GetMyAllocations(c *gin.Context) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}
	allocations := []models.AirdropAllocation{}
	if err := s.db.Where("user_id = ?", userID).Order("created_at desc").Find(&allocations).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load allocations"})
		return
	}
	views := make([]gin.H, 0, len(allocations))
	for _, allocation := range allocations {
		var campaign models.AirdropCampaign
		if err := s.db.First(&campaign, "id = ?", allocation.CampaignID).Error; err != nil {
			continue
		}
		views = append(views, gin.H{
			"allocation_id": allocation.ID,
			"campaign_id":   campaign.ID,
			"campaign_key":  campaign.Key,
			"campaign_name": campaign.Name,
			"token_address": campaign.TokenAddress,
			"symbol":        campaign.TokenSymbol,
			"amount":        allocation.Amount,
			"claimed":       allocation.Claimed,
			"status":        campaign.Status,
			"start_time":    campaign.StartTime,
			"end_time":      campaign.EndTime,
		})
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "allocations": views})
}

// GetCampaignAllocations lists a campaign's allocations. Admin only.
func (s *AirdropService) GetCampaignAllocations(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid campaign id"})
		return
	}
	campaign, err := s.loadCampaign(id)
	if err != nil {
		if errors.Is(err, ErrListingNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "airdrop campaign not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load campaign"})
		return
	}
	allocations := []models.AirdropAllocation{}
	if err := s.db.Where("campaign_id = ?", campaign.ID).Order("created_at asc").Find(&allocations).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load allocations"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "campaign": campaign, "allocations": allocations})
}

// DeleteCampaign closes a campaign. A campaign that has paid out is retained
// for its audit trail and is only marked closed.
func (s *AirdropService) DeleteCampaign(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid campaign id"})
		return
	}
	campaign, err := s.loadCampaign(id)
	if err != nil {
		if errors.Is(err, ErrListingNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "airdrop campaign not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load campaign"})
		return
	}
	claimed, _ := new(big.Int).SetString(campaign.TotalClaimed, 10)
	if claimed != nil && claimed.Sign() > 0 {
		if err := s.db.Model(&models.AirdropCampaign{}).Where("id = ?", campaign.ID).
			Update("status", models.AirdropStatusClosed).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to close campaign"})
			return
		}
		logListingAction(c, s.db, "close_airdrop", "airdrop_campaign", campaign.ID.String(), nil)
		c.JSON(http.StatusOK, gin.H{"success": true, "status": models.AirdropStatusClosed})
		return
	}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("campaign_id = ?", campaign.ID).Delete(&models.AirdropAllocation{}).Error; err != nil {
			return err
		}
		return tx.Delete(&models.AirdropCampaign{}, "id = ?", campaign.ID).Error
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete campaign"})
		return
	}
	logListingAction(c, s.db, "delete_airdrop", "airdrop_campaign", campaign.ID.String(), nil)
	c.JSON(http.StatusOK, gin.H{"success": true, "deleted": true})
}
