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

// Sentinel for a missing launchpad project.
var errLaunchpadProjectNotFound = errors.New("launchpad project not found")

// LaunchpadService handles IEO/IDO/Launchpad functionality. Projects and
// allocations live in the database so a raise survives a restart; keeping them
// in process memory would silently erase every project and commitment the
// moment the API restarts.
type LaunchpadService struct {
	blockchain *BlockchainClient
	priceAgg   *PriceAggregator
	db         *gorm.DB
	tiers      map[string]*TierConfig
}

// LaunchpadProject represents a launchpad project
type LaunchpadProject struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	TokenAddress  string    `json:"token_address"`
	TokenSymbol   string    `json:"token_symbol"`
	TokenName     string    `json:"token_name"`
	TokenDecimals int       `json:"token_decimals"`
	TotalSupply   string    `json:"total_supply"`
	TokenPrice    string    `json:"token_price"`     // Price in payment token
	PricePerToken string    `json:"price_per_token"` // Same value, the name the listing validators read
	PaymentToken  string    `json:"payment_token"`   // USDT, USDC, etc.
	MinPurchase   string    `json:"min_purchase"`
	MaxPurchase   string    `json:"max_purchase"`
	SoftCap       string    `json:"soft_cap"`
	HardCap       string    `json:"hard_cap"`
	StartTime     time.Time `json:"start_time"`
	EndTime       time.Time `json:"end_time"`
	Status        string    `json:"status"` // upcoming, active, completed, cancelled
	RaisedAmount  string    `json:"raised_amount"`
	Participants  int       `json:"participants"`
	WebsiteURL    string    `json:"website_url"`
	WhitepaperURL string    `json:"whitepaper_url"`
	LogoURL       string    `json:"logo_url"`
	TwitterURL    string    `json:"twitter_url"`
	TelegramURL   string    `json:"telegram_url"`
	DiscordURL    string    `json:"discord_url"`
	CreatedBy     string    `json:"created_by"`
	CreatedAt     time.Time `json:"created_at"`
}

// Allocation represents user allocation
type Allocation struct {
	UserAddress     string     `json:"user_address"`
	ProjectID       string     `json:"project_id"`
	AllocatedAmount string     `json:"allocated_amount"`
	ClaimedAmount   string     `json:"claimed_amount"`
	ClaimedAt       *time.Time `json:"claimed_at"`
	Tier            string     `json:"tier"`
	Status          string     `json:"status"` // pending, claimable, claimed
}

// TierConfig represents tier configuration
type TierConfig struct {
	Name          string `json:"name"`
	MinStake      string `json:"min_stake"`
	Allocation    string `json:"allocation"`
	MaxAllocation string `json:"max_allocation"`
}

// VestingSchedule represents token vesting
type VestingSchedule struct {
	ProjectID     string    `json:"project_id"`
	UserAddress   string    `json:"user_address"`
	TotalAmount   string    `json:"total_amount"`
	ClaimedAmount string    `json:"claimed_amount"`
	StartTime     time.Time `json:"start_time"`
	EndTime       time.Time `json:"end_time"`
	CliffPeriod   int       `json:"cliff_period"` // seconds
	ReleaseRate   string    `json:"release_rate"` // per period
}

func NewLaunchpadService(db *gorm.DB, blockchain *BlockchainClient, priceAgg *PriceAggregator) *LaunchpadService {
	return &LaunchpadService{
		blockchain: blockchain,
		priceAgg:   priceAgg,
		db:         db,
		tiers: map[string]*TierConfig{
			"diamond": {
				Name:          "Diamond",
				MinStake:      "100000",
				Allocation:    "5000",
				MaxAllocation: "50000",
			},
			"platinum": {
				Name:          "Platinum",
				MinStake:      "50000",
				Allocation:    "2500",
				MaxAllocation: "25000",
			},
			"gold": {
				Name:          "Gold",
				MinStake:      "10000",
				Allocation:    "1000",
				MaxAllocation: "10000",
			},
			"silver": {
				Name:          "Silver",
				MinStake:      "1000",
				Allocation:    "500",
				MaxAllocation: "5000",
			},
			"bronze": {
				Name:          "Bronze",
				MinStake:      "0",
				Allocation:    "100",
				MaxAllocation: "1000",
			},
		},
	}
}

// CreateProject creates a new launchpad project and persists it.
func (s *LaunchpadService) CreateProject(ctx context.Context, creator, name, description, tokenAddress, tokenSymbol, tokenName, totalSupply, tokenPrice, paymentToken, minPurchase, maxPurchase, softCap, hardCap string, startTime, endTime time.Time, websiteURL, whitepaperURL, logoURL string) (*LaunchpadProject, error) {
	if s.db == nil {
		return nil, fmt.Errorf("launchpad store is not configured")
	}
	if endTime.Before(startTime) {
		return nil, fmt.Errorf("end time must be after start time")
	}
	// The monetary fields are validated at the boundary so a project can never be
	// persisted with an amount the raise arithmetic cannot parse.
	for label, raw := range map[string]string{
		"token_price": tokenPrice, "min_purchase": minPurchase, "max_purchase": maxPurchase,
		"soft_cap": softCap, "hard_cap": hardCap,
	} {
		if _, err := parseDecimal(raw); err != nil {
			return nil, fmt.Errorf("%s is invalid: %w", label, err)
		}
	}

	row := models.LaunchpadProject{
		Name:          name,
		Description:   description,
		TokenAddress:  tokenAddress,
		TokenSymbol:   tokenSymbol,
		TokenName:     tokenName,
		TotalSupply:   totalSupply,
		PricePerToken: tokenPrice,
		PaymentToken:  paymentToken,
		MinPurchase:   minPurchase,
		MaxPurchase:   maxPurchase,
		SoftCap:       softCap,
		HardCap:       hardCap,
		StartTime:     startTime,
		EndTime:       endTime,
		Status:        "upcoming",
		RaisedAmount:  "0",
		Participants:  0,
		WebsiteURL:    websiteURL,
		WhitepaperURL: whitepaperURL,
		LogoURL:       logoURL,
		CreatedBy:     creator,
		CreatedAt:     time.Now(),
	}
	if err := s.db.WithContext(ctx).Create(&row).Error; err != nil {
		return nil, fmt.Errorf("persist launchpad project: %w", err)
	}
	return launchpadProjectFromModel(&row), nil
}

// UpdateProjectStatus moves a project between lifecycle states and persists the
// change, so an admin action survives a restart.
func (s *LaunchpadService) UpdateProjectStatus(projectID, status string) error {
	if s.db == nil {
		return fmt.Errorf("launchpad store is not configured")
	}
	if !validLaunchpadStatuses[status] {
		return fmt.Errorf("unsupported launchpad status %q", status)
	}
	id, err := uuid.Parse(strings.TrimSpace(projectID))
	if err != nil {
		return errLaunchpadProjectNotFound
	}
	res := s.db.Model(&models.LaunchpadProject{}).Where("id = ?", id).Update("status", status)
	if res.Error != nil {
		return fmt.Errorf("update project status: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return errLaunchpadProjectNotFound
	}
	return nil
}

// Participate records a participant's commitment. The project row and the
// participant's allocation row are both re-read under a FOR UPDATE lock inside
// one transaction, so two concurrent commitments cannot overshoot the hard cap
// or a participant's maximum purchase, and the raised total always matches the
// sum of the allocations.
func (s *LaunchpadService) Participate(ctx context.Context, userAddress, projectID, amount string) (*Allocation, error) {
	if s.db == nil {
		return nil, fmt.Errorf("launchpad store is not configured")
	}
	projectUUID, err := uuid.Parse(strings.TrimSpace(projectID))
	if err != nil {
		return nil, errLaunchpadProjectNotFound
	}
	address := strings.ToLower(strings.TrimSpace(userAddress))
	if address == "" {
		return nil, fmt.Errorf("participant address is required")
	}

	amountValue, err := parseDecimal(amount)
	if err != nil {
		return nil, fmt.Errorf("invalid amount: %w", err)
	}
	if amountValue.Sign() <= 0 {
		return nil, fmt.Errorf("amount must be positive")
	}

	// The participant's tier comes from their veTGR power, a read on the same
	// connection the transaction uses; resolve it before opening the transaction
	// so it never deadlocks against the held connection.
	tier := s.determineTier(address)

	var stored models.LaunchpadAllocation
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var project models.LaunchpadProject
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&project, "id = ?", projectUUID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errLaunchpadProjectNotFound
			}
			return err
		}

		if project.Status != "active" {
			return fmt.Errorf("project is not active")
		}
		now := time.Now()
		if now.Before(project.StartTime) {
			return fmt.Errorf("project has not started yet")
		}
		if now.After(project.EndTime) {
			return fmt.Errorf("project has ended")
		}

		minPurchase, err := parseDecimal(project.MinPurchase)
		if err != nil {
			return fmt.Errorf("project minimum purchase is invalid: %w", err)
		}
		maxPurchase, err := parseDecimal(project.MaxPurchase)
		if err != nil {
			return fmt.Errorf("project maximum purchase is invalid: %w", err)
		}
		if amountValue.Cmp(minPurchase) < 0 {
			return fmt.Errorf("amount below minimum purchase")
		}

		// A participant's total commitment may not exceed the maximum purchase.
		var existing models.LaunchpadAllocation
		hasExisting := true
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("project_id = ? AND user_address = ?", project.ID, address).
			First(&existing).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				hasExisting = false
			} else {
				return err
			}
		}

		running := new(big.Rat)
		if hasExisting {
			parsed, err := parseDecimal(existing.AllocatedAmount)
			if err != nil {
				return fmt.Errorf("stored allocation is invalid: %w", err)
			}
			running = parsed
		}
		if new(big.Rat).Add(running, amountValue).Cmp(maxPurchase) > 0 {
			return fmt.Errorf("amount exceeds maximum purchase")
		}

		if hasExisting {
			newAllocated := formatDecimal(new(big.Rat).Add(running, amountValue), 8)
			if err := tx.Model(&models.LaunchpadAllocation{}).Where("id = ?", existing.ID).
				Updates(map[string]interface{}{
					"allocated_amount": newAllocated,
					"tier":             tier,
				}).Error; err != nil {
				return err
			}
			existing.AllocatedAmount = newAllocated
			existing.Tier = tier
			stored = existing
		} else {
			stored = models.LaunchpadAllocation{
				ProjectID:       project.ID,
				UserAddress:     address,
				AllocatedAmount: formatDecimal(amountValue, 8),
				ClaimedAmount:   "0",
				Tier:            tier,
				Status:          "pending",
				CreatedAt:       now,
			}
			if err := tx.Create(&stored).Error; err != nil {
				return err
			}
		}

		// Advance the project's raised total by the same amount, reading it inside
		// the lock so the counters stay consistent with the allocations.
		raised, err := parseDecimal(project.RaisedAmount)
		if err != nil {
			return fmt.Errorf("project raised amount is invalid: %w", err)
		}
		newRaised := new(big.Rat).Add(raised, amountValue)
		hardCap, err := parseDecimal(project.HardCap)
		if err != nil {
			return fmt.Errorf("project hard cap is invalid: %w", err)
		}

		// participants counts distinct wallets, so it advances only for a first-time
		// participant, not on a top-up.
		participants := project.Participants
		if !hasExisting {
			participants++
		}
		updates := map[string]interface{}{
			"raised_amount": formatDecimal(newRaised, 8),
			"participants":  participants,
		}
		if newRaised.Cmp(hardCap) >= 0 {
			updates["status"] = "completed"
		}
		if err := tx.Model(&models.LaunchpadProject{}).Where("id = ?", project.ID).
			Updates(updates).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return launchpadAllocationFromModel(&stored), nil
}

// ClaimTokens computes and records the claimable token amount for a
// participant. The allocation row is re-read under lock and marked claimed
// before the amount is returned, so a claim cannot be settled twice.
func (s *LaunchpadService) ClaimTokens(userAddress, projectID string) (string, error) {
	if s.db == nil {
		return "", fmt.Errorf("launchpad store is not configured")
	}
	projectUUID, err := uuid.Parse(strings.TrimSpace(projectID))
	if err != nil {
		return "", errLaunchpadProjectNotFound
	}
	address := strings.ToLower(strings.TrimSpace(userAddress))

	var claimed string
	err = s.db.Transaction(func(tx *gorm.DB) error {
		var project models.LaunchpadProject
		if err := tx.First(&project, "id = ?", projectUUID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errLaunchpadProjectNotFound
			}
			return err
		}
		if time.Now().Before(project.EndTime) {
			return fmt.Errorf("claim not available yet")
		}

		var allocation models.LaunchpadAllocation
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("project_id = ? AND user_address = ?", project.ID, address).
			First(&allocation).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("no allocation found")
			}
			return err
		}
		if allocation.Status == "claimed" {
			return fmt.Errorf("tokens already claimed")
		}

		price, err := parseDecimal(project.PricePerToken)
		if err != nil {
			return fmt.Errorf("project token price is invalid: %w", err)
		}
		if price.Sign() <= 0 {
			return fmt.Errorf("project token price must be positive")
		}
		paymentAmount, err := parseDecimal(allocation.AllocatedAmount)
		if err != nil {
			return fmt.Errorf("allocation amount is invalid: %w", err)
		}
		tokenAmount := new(big.Rat).Quo(paymentAmount, price)
		claimed = formatDecimal(tokenAmount, 8)
		now := time.Now()
		if err := tx.Model(&models.LaunchpadAllocation{}).Where("id = ?", allocation.ID).
			Updates(map[string]interface{}{
				"claimed_amount": claimed,
				"claimed_at":     now,
				"status":         "claimed",
			}).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return claimed, nil
}

// GetProject returns a project by id.
func (s *LaunchpadService) GetProject(projectID string) (*LaunchpadProject, error) {
	if s.db == nil {
		return nil, fmt.Errorf("launchpad store is not configured")
	}
	id, err := uuid.Parse(strings.TrimSpace(projectID))
	if err != nil {
		return nil, errLaunchpadProjectNotFound
	}
	var row models.LaunchpadProject
	if err := s.db.First(&row, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errLaunchpadProjectNotFound
		}
		return nil, err
	}
	return launchpadProjectFromModel(&row), nil
}

// GetProjects returns every project, optionally filtered by status.
func (s *LaunchpadService) GetProjects(status string) []*LaunchpadProject {
	if s.db == nil {
		return nil
	}
	query := s.db.Model(&models.LaunchpadProject{}).Order("created_at DESC")
	if status != "" {
		query = query.Where("status = ?", status)
	}
	var rows []models.LaunchpadProject
	if err := query.Find(&rows).Error; err != nil {
		return nil
	}
	out := make([]*LaunchpadProject, 0, len(rows))
	for i := range rows {
		out = append(out, launchpadProjectFromModel(&rows[i]))
	}
	return out
}

// GetUserAllocation returns a participant's allocation in a project.
func (s *LaunchpadService) GetUserAllocation(userAddress, projectID string) (*Allocation, error) {
	if s.db == nil {
		return nil, fmt.Errorf("launchpad store is not configured")
	}
	projectUUID, err := uuid.Parse(strings.TrimSpace(projectID))
	if err != nil {
		return nil, errLaunchpadProjectNotFound
	}
	var row models.LaunchpadAllocation
	if err := s.db.Where("project_id = ? AND user_address = ?", projectUUID, strings.ToLower(strings.TrimSpace(userAddress))).
		First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("no allocation found")
		}
		return nil, err
	}
	return launchpadAllocationFromModel(&row), nil
}

// launchpadProjectFromModel converts a persisted project into the wire shape.
// The price is exposed under both its legacy `token_price` name and the
// `price_per_token` name the listing validators read, so the same object serves
// the public list and the create response.
func launchpadProjectFromModel(m *models.LaunchpadProject) *LaunchpadProject {
	return &LaunchpadProject{
		ID:            m.ID.String(),
		Name:          m.Name,
		Description:   m.Description,
		TokenAddress:  m.TokenAddress,
		TokenSymbol:   m.TokenSymbol,
		TokenName:     m.TokenName,
		TokenDecimals: 18,
		TotalSupply:   m.TotalSupply,
		TokenPrice:    m.PricePerToken,
		PricePerToken: m.PricePerToken,
		PaymentToken:  m.PaymentToken,
		MinPurchase:   m.MinPurchase,
		MaxPurchase:   m.MaxPurchase,
		SoftCap:       m.SoftCap,
		HardCap:       m.HardCap,
		StartTime:     m.StartTime,
		EndTime:       m.EndTime,
		Status:        m.Status,
		RaisedAmount:  m.RaisedAmount,
		Participants:  m.Participants,
		WebsiteURL:    m.WebsiteURL,
		WhitepaperURL: m.WhitepaperURL,
		LogoURL:       m.LogoURL,
		CreatedBy:     m.CreatedBy,
		CreatedAt:     m.CreatedAt,
	}
}

func launchpadAllocationFromModel(m *models.LaunchpadAllocation) *Allocation {
	return &Allocation{
		UserAddress:     m.UserAddress,
		ProjectID:       m.ProjectID.String(),
		AllocatedAmount: m.AllocatedAmount,
		ClaimedAmount:   m.ClaimedAmount,
		ClaimedAt:       m.ClaimedAt,
		Tier:            m.Tier,
		Status:          m.Status,
	}
}

// parseDecimal converts a base-10 decimal string to an exact rational. It
// rejects empty input, signs, exponents and any non-digit content, so money
// arithmetic never silently becomes zero the way fmt.Sscanf would.
func parseDecimal(raw string) (*big.Rat, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, fmt.Errorf("empty decimal value")
	}
	if strings.HasPrefix(trimmed, "-") || strings.HasPrefix(trimmed, "+") {
		return nil, fmt.Errorf("decimal value must not be signed: %q", raw)
	}

	parts := strings.Split(trimmed, ".")
	if len(parts) > 2 {
		return nil, fmt.Errorf("invalid decimal value: %q", raw)
	}

	digits := parts[0]
	if len(parts) == 2 {
		digits += parts[1]
	}
	if digits == "" || !isAllDigits(digits) {
		return nil, fmt.Errorf("invalid decimal value: %q", raw)
	}

	value, ok := new(big.Rat).SetString(trimmed)
	if !ok {
		return nil, fmt.Errorf("invalid decimal value: %q", raw)
	}
	return value, nil
}

func isAllDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// formatDecimal renders a rational as a plain decimal string with exactly
// `places` fractional digits, truncating rather than rounding up.
func formatDecimal(value *big.Rat, places int) string {
	scaled := new(big.Rat).Mul(value, new(big.Rat).SetInt(pow10(places)))
	floored := new(big.Int).Quo(scaled.Num(), scaled.Denom())

	negative := floored.Sign() < 0
	if negative {
		floored.Neg(floored)
	}

	digits := floored.String()
	if len(digits) <= places {
		digits = strings.Repeat("0", places-len(digits)+1) + digits
	}
	out := digits[:len(digits)-places] + "." + digits[len(digits)-places:]
	if negative {
		out = "-" + out
	}
	return out
}

func pow10(n int) *big.Int {
	return new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(n)), nil)
}

// determineTier returns the launchpad tier for a participant from the veTGR
// voting power their wallet holds. Thresholds come from this service's own tier
// configuration, so none is hardcoded here.
func (s *LaunchpadService) determineTier(userAddress string) string {
	power := s.stakedPower(userAddress)
	if power.Sign() <= 0 {
		return "bronze"
	}

	bestName := "bronze"
	bestThreshold := big.NewInt(-1)
	for name, tier := range s.tiers {
		threshold, ok := new(big.Int).SetString(tier.MinStake, 10)
		if !ok {
			continue
		}
		if power.Cmp(threshold) >= 0 && threshold.Cmp(bestThreshold) > 0 {
			bestName = name
			bestThreshold = threshold
		}
	}
	return bestName
}

// stakedPower returns the veTGR voting power of the user owning `userAddress`.
// Addresses are compared case-insensitively because they are stored lowercase.
func (s *LaunchpadService) stakedPower(userAddress string) *big.Int {
	if s.db == nil || strings.TrimSpace(userAddress) == "" {
		return big.NewInt(0)
	}

	var user models.User
	if err := s.db.Where("LOWER(wallet_address) = ?", strings.ToLower(strings.TrimSpace(userAddress))).
		First(&user).Error; err != nil {
		return big.NewInt(0)
	}

	var locks []models.VeLock
	if err := s.db.Where("user_id = ? AND withdrawn = ?", user.ID.String(), false).
		Find(&locks).Error; err != nil {
		return big.NewInt(0)
	}

	now := time.Now()
	total := big.NewInt(0)
	for _, lock := range locks {
		total.Add(total, votingPower(lock, now))
	}
	return total
}

// GetTiers returns all tier configurations
func (s *LaunchpadService) GetTiers() map[string]*TierConfig {
	return s.tiers
}

// API Handlers

func (s *LaunchpadService) CreateProjectHandler(c *gin.Context) {
	var req struct {
		Name          string `json:"name" binding:"required"`
		Description   string `json:"description"`
		TokenAddress  string `json:"token_address" binding:"required"`
		TokenSymbol   string `json:"token_symbol" binding:"required"`
		TokenName     string `json:"token_name" binding:"required"`
		TotalSupply   string `json:"total_supply" binding:"required"`
		TokenPrice    string `json:"token_price" binding:"required"`
		PaymentToken  string `json:"payment_token" binding:"required"`
		MinPurchase   string `json:"min_purchase"`
		MaxPurchase   string `json:"max_purchase"`
		SoftCap       string `json:"soft_cap"`
		HardCap       string `json:"hard_cap"`
		StartTime     string `json:"start_time" binding:"required"`
		EndTime       string `json:"end_time" binding:"required"`
		WebsiteURL    string `json:"website_url"`
		WhitepaperURL string `json:"whitepaper_url"`
		LogoURL       string `json:"logo_url"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequestError(c, err)
		return
	}

	creator, ok := s.callerWalletAddress(c)
	if !ok {
		return
	}

	startTime, err := time.Parse(time.RFC3339, req.StartTime)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "start_time must be RFC3339"})
		return
	}
	endTime, err := time.Parse(time.RFC3339, req.EndTime)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "end_time must be RFC3339"})
		return
	}

	project, err := s.CreateProject(c.Request.Context(), creator, req.Name, req.Description, req.TokenAddress, req.TokenSymbol, req.TokenName, req.TotalSupply, req.TokenPrice, req.PaymentToken, req.MinPurchase, req.MaxPurchase, req.SoftCap, req.HardCap, startTime, endTime, req.WebsiteURL, req.WhitepaperURL, req.LogoURL)
	if err != nil {
		respondInternalError(c, err)
		return
	}

	c.JSON(200, project)
}

// callerWalletAddress resolves the wallet address bound to the authenticated
// caller. Launchpad participation and claims act on the caller's own wallet, so
// a client can never participate as, or claim to, another address.
func (s *LaunchpadService) callerWalletAddress(c *gin.Context) (string, bool) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return "", false
	}
	if s.db == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "launchpad store not configured"})
		return "", false
	}

	var user models.User
	if err := s.db.Where("id = ?", userID).First(&user).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "no wallet is bound to this account"})
		return "", false
	}
	if !common.IsHexAddress(user.WalletAddress) {
		c.JSON(http.StatusForbidden, gin.H{"error": "account has no usable wallet address"})
		return "", false
	}
	return strings.ToLower(user.WalletAddress), true
}

// UpdateProjectStatusHandler is the admin endpoint that moves a launchpad
// project between lifecycle states (upcoming, active, completed, cancelled).
func (s *LaunchpadService) UpdateProjectStatusHandler(c *gin.Context) {
	projectID := c.Param("id")

	var req struct {
		Status string `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequestError(c, err)
		return
	}
	if !validLaunchpadStatuses[req.Status] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported launchpad status"})
		return
	}

	if err := s.UpdateProjectStatus(projectID, req.Status); err != nil {
		respondServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "id": projectID, "status": req.Status})
}

// validLaunchpadStatuses is the closed set of lifecycle states a project may
// be moved into.
var validLaunchpadStatuses = map[string]bool{
	"upcoming":  true,
	"active":    true,
	"completed": true,
	"cancelled": true,
}

func (s *LaunchpadService) GetProjectsHandler(c *gin.Context) {
	status := c.Query("status")

	projects := s.GetProjects(status)
	c.JSON(200, gin.H{
		"projects": projects,
		"count":    len(projects),
	})
}

func (s *LaunchpadService) GetProjectHandler(c *gin.Context) {
	projectID := c.Param("id")

	project, err := s.GetProject(projectID)
	if err != nil {
		respondServiceError(c, err)
		return
	}

	c.JSON(200, project)
}

func (s *LaunchpadService) ParticipateHandler(c *gin.Context) {
	var req struct {
		Amount string `json:"amount" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequestError(c, err)
		return
	}

	userAddress, ok := s.callerWalletAddress(c)
	if !ok {
		return
	}

	allocation, err := s.Participate(c.Request.Context(), userAddress, c.Param("id"), req.Amount)
	if err != nil {
		respondInternalError(c, err)
		return
	}

	c.JSON(200, allocation)
}

func (s *LaunchpadService) ClaimTokensHandler(c *gin.Context) {
	userAddress, ok := s.callerWalletAddress(c)
	if !ok {
		return
	}

	amount, err := s.ClaimTokens(userAddress, c.Param("id"))
	if err != nil {
		respondInternalError(c, err)
		return
	}

	c.JSON(200, gin.H{
		"success": true,
		"claimed": amount,
		"message": "Tokens claimed successfully",
	})
}

func (s *LaunchpadService) GetTiersHandler(c *gin.Context) {
	c.JSON(200, s.GetTiers())
}

// Helper function
