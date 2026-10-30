package services

import (
	"context"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"tigerswap/backend/models"
)

// LaunchpadService handles IEO/IDO/Launchpad functionality
type LaunchpadService struct {
	blockchain  *BlockchainClient
	priceAgg    *PriceAggregator
	db          *gorm.DB
	mu          sync.RWMutex
	projects    map[string]*LaunchpadProject // projectID -> project
	allocations map[string][]*Allocation     // projectID -> allocations
	tiers       map[string]*TierConfig
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
	TokenPrice    string    `json:"token_price"`   // Price in payment token
	PaymentToken  string    `json:"payment_token"` // USDT, USDC, etc.
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
		blockchain:  blockchain,
		priceAgg:    priceAgg,
		db:          db,
		projects:    make(map[string]*LaunchpadProject),
		allocations: make(map[string][]*Allocation),
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

// CreateProject creates a new launchpad project
func (s *LaunchpadService) CreateProject(ctx context.Context, creator, name, description, tokenAddress, tokenSymbol, tokenName, totalSupply, tokenPrice, paymentToken, minPurchase, maxPurchase, softCap, hardCap string, startTime, endTime time.Time, websiteURL, whitepaperURL, logoURL string) (*LaunchpadProject, error) {
	// Validate
	if endTime.Before(startTime) {
		return nil, fmt.Errorf("end time must be after start time")
	}

	project := &LaunchpadProject{
		ID:            fmt.Sprintf("proj_%d", time.Now().UnixNano()),
		Name:          name,
		Description:   description,
		TokenAddress:  tokenAddress,
		TokenSymbol:   tokenSymbol,
		TokenName:     tokenName,
		TokenDecimals: 18,
		TotalSupply:   totalSupply,
		TokenPrice:    tokenPrice,
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

	s.mu.Lock()
	s.projects[project.ID] = project
	s.allocations[project.ID] = []*Allocation{}
	s.mu.Unlock()

	return project, nil
}

// UpdateProjectStatus updates project status
func (s *LaunchpadService) UpdateProjectStatus(projectID, status string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	project, exists := s.projects[projectID]
	if !exists {
		return fmt.Errorf("project not found")
	}

	project.Status = status
	return nil
}

// Participate allows a user to participate in a launchpad
func (s *LaunchpadService) Participate(ctx context.Context, userAddress, projectID, amount string) (*Allocation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	project, exists := s.projects[projectID]
	if !exists {
		return nil, fmt.Errorf("project not found")
	}

	// Check if project is active
	if project.Status != "active" {
		return nil, fmt.Errorf("project is not active")
	}

	// Check timing
	now := time.Now()
	if now.Before(project.StartTime) {
		return nil, fmt.Errorf("project has not started yet")
	}
	if now.After(project.EndTime) {
		return nil, fmt.Errorf("project has ended")
	}

	// Check min/max purchase with exact decimal arithmetic. A malformed amount
	// must be rejected rather than coerced to zero by a permissive parse.
	amountValue, err := parseDecimal(amount)
	if err != nil {
		return nil, fmt.Errorf("invalid amount: %w", err)
	}
	if amountValue.Sign() <= 0 {
		return nil, fmt.Errorf("amount must be positive")
	}

	minPurchase, err := parseDecimal(project.MinPurchase)
	if err != nil {
		return nil, fmt.Errorf("project minimum purchase is invalid: %w", err)
	}
	maxPurchase, err := parseDecimal(project.MaxPurchase)
	if err != nil {
		return nil, fmt.Errorf("project maximum purchase is invalid: %w", err)
	}

	if amountValue.Cmp(minPurchase) < 0 {
		return nil, fmt.Errorf("amount below minimum purchase")
	}
	if amountValue.Cmp(maxPurchase) > 0 {
		return nil, fmt.Errorf("amount exceeds maximum purchase")
	}

	// Determine tier from the participant's veTGR voting power.
	tier := s.determineTier(userAddress)

	allocation := &Allocation{
		UserAddress:     userAddress,
		ProjectID:       projectID,
		AllocatedAmount: amount,
		ClaimedAmount:   "0",
		Tier:            tier,
		Status:          "pending",
	}

	s.allocations[projectID] = append(s.allocations[projectID], allocation)

	// Update the raised amount exactly; the stored value is a decimal string.
	raised, err := parseDecimal(project.RaisedAmount)
	if err != nil {
		return nil, fmt.Errorf("project raised amount is invalid: %w", err)
	}
	newRaised := new(big.Rat).Add(raised, amountValue)
	project.RaisedAmount = formatDecimal(newRaised, 8)
	project.Participants++

	// Close the raise once the hard cap is met.
	hardCap, err := parseDecimal(project.HardCap)
	if err != nil {
		return nil, fmt.Errorf("project hard cap is invalid: %w", err)
	}
	if newRaised.Cmp(hardCap) >= 0 {
		project.Status = "completed"
	}

	return allocation, nil
}

// ClaimTokens allows user to claim purchased tokens
func (s *LaunchpadService) ClaimTokens(userAddress, projectID string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	project, exists := s.projects[projectID]
	if !exists {
		return "", fmt.Errorf("project not found")
	}

	// Find allocation
	var allocation *Allocation
	for _, alloc := range s.allocations[projectID] {
		if alloc.UserAddress == userAddress {
			allocation = alloc
			break
		}
	}

	if allocation == nil {
		return "", fmt.Errorf("no allocation found")
	}

	if allocation.Status == "claimed" {
		return "", fmt.Errorf("tokens already claimed")
	}

	// Check if claimable (after project ends)
	if time.Now().Before(project.EndTime) {
		return "", fmt.Errorf("claim not available yet")
	}

	// Calculate the token amount exactly. A zero or malformed price must be
	// rejected rather than producing an infinite or NaN amount.
	price, err := parseDecimal(project.TokenPrice)
	if err != nil {
		return "", fmt.Errorf("project token price is invalid: %w", err)
	}
	if price.Sign() <= 0 {
		return "", fmt.Errorf("project token price must be positive")
	}

	paymentAmount, err := parseDecimal(allocation.AllocatedAmount)
	if err != nil {
		return "", fmt.Errorf("allocation amount is invalid: %w", err)
	}

	tokenAmount := new(big.Rat).Quo(paymentAmount, price)
	allocation.ClaimedAmount = formatDecimal(tokenAmount, 8)
	now := time.Now()
	allocation.ClaimedAt = &now
	allocation.Status = "claimed"

	return allocation.ClaimedAmount, nil
}

// GetProject returns project details
func (s *LaunchpadService) GetProject(projectID string) (*LaunchpadProject, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	project, exists := s.projects[projectID]
	if !exists {
		return nil, fmt.Errorf("project not found")
	}

	return project, nil
}

// GetProjects returns all projects with optional status filter
func (s *LaunchpadService) GetProjects(status string) []*LaunchpadProject {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*LaunchpadProject
	for _, project := range s.projects {
		if status == "" || project.Status == status {
			result = append(result, project)
		}
	}

	return result
}

// GetUserAllocation returns user's allocation for a project
func (s *LaunchpadService) GetUserAllocation(userAddress, projectID string) (*Allocation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, allocation := range s.allocations[projectID] {
		if allocation.UserAddress == userAddress {
			return allocation, nil
		}
	}

	return nil, fmt.Errorf("no allocation found")
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
		Creator       string `json:"creator" binding:"required"`
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
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	startTime, _ := time.Parse(time.RFC3339, req.StartTime)
	endTime, _ := time.Parse(time.RFC3339, req.EndTime)

	project, err := s.CreateProject(c.Request.Context(), req.Creator, req.Name, req.Description, req.TokenAddress, req.TokenSymbol, req.TokenName, req.TotalSupply, req.TokenPrice, req.PaymentToken, req.MinPurchase, req.MaxPurchase, req.SoftCap, req.HardCap, startTime, endTime, req.WebsiteURL, req.WhitepaperURL, req.LogoURL)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, project)
}

// UpdateProjectStatusHandler is the admin endpoint that moves a launchpad
// project between lifecycle states (upcoming, active, completed, cancelled).
func (s *LaunchpadService) UpdateProjectStatusHandler(c *gin.Context) {
	projectID := c.Param("id")

	var req struct {
		Status string `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !validLaunchpadStatuses[req.Status] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported launchpad status"})
		return
	}

	if err := s.UpdateProjectStatus(projectID, req.Status); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
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
		c.JSON(404, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, project)
}

func (s *LaunchpadService) ParticipateHandler(c *gin.Context) {
	var req struct {
		UserAddress string `json:"user_address" binding:"required"`
		ProjectID   string `json:"project_id" binding:"required"`
		Amount      string `json:"amount" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	allocation, err := s.Participate(c.Request.Context(), req.UserAddress, req.ProjectID, req.Amount)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, allocation)
}

func (s *LaunchpadService) ClaimTokensHandler(c *gin.Context) {
	var req struct {
		UserAddress string `json:"user_address" binding:"required"`
		ProjectID   string `json:"project_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	amount, err := s.ClaimTokens(req.UserAddress, req.ProjectID)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
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
