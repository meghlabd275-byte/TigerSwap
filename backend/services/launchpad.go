package services

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// LaunchpadService handles IEO/IDO/Launchpad functionality
type LaunchpadService struct {
	blockchain   *BlockchainClient
	priceAgg     *PriceAggregator
	mu           sync.RWMutex
	projects     map[string]*LaunchpadProject // projectID -> project
	allocations  map[string][]*Allocation    // projectID -> allocations
	tiers        map[string]*TierConfig
}

// LaunchpadProject represents a launchpad project
type LaunchpadProject struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Description     string    `json:"description"`
	TokenAddress    string    `json:"token_address"`
	TokenSymbol     string    `json:"token_symbol"`
	TokenName       string    `json:"token_name"`
	TokenDecimals   int       `json:"token_decimals"`
	TotalSupply     string    `json:"total_supply"`
	TokenPrice      string    `json:"token_price"` // Price in payment token
	PaymentToken    string    `json:"payment_token"` // USDT, USDC, etc.
	MinPurchase     string    `json:"min_purchase"`
	MaxPurchase     string    `json:"max_purchase"`
	SoftCap         string    `json:"soft_cap"`
	HardCap         string    `json:"hard_cap"`
	StartTime       time.Time `json:"start_time"`
	EndTime         time.Time `json:"end_time"`
	Status          string    `json:"status"` // upcoming, active, completed, cancelled
	RaisedAmount    string    `json:"raised_amount"`
	Participants    int       `json:"participants"`
	WebsiteURL      string    `json:"website_url"`
	WhitepaperURL   string    `json:"whitepaper_url"`
	LogoURL         string    `json:"logo_url"`
	TwitterURL      string    `json:"twitter_url"`
	TelegramURL     string    `json:"telegram_url"`
	DiscordURL      string    `json:"discord_url"`
	CreatedBy       string    `json:"created_by"`
	CreatedAt       time.Time `json:"created_at"`
}

// Allocation represents user allocation
type Allocation struct {
	UserAddress    string    `json:"user_address"`
	ProjectID     string    `json:"project_id"`
	AllocatedAmount string   `json:"allocated_amount"`
	ClaimedAmount  string    `json:"claimed_amount"`
	ClaimedAt      *time.Time `json:"claimed_at"`
	Tier           string    `json:"tier"`
	Status         string    `json:"status"` // pending, claimable, claimed
}

// TierConfig represents tier configuration
type TierConfig struct {
	Name           string  `json:"name"`
	MinStake       string  `json:"min_stake"`
	Allocation    string  `json:"allocation"`
	MaxAllocation  string  `json:"max_allocation"`
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

func NewLaunchpadService(blockchain *BlockchainClient, priceAgg *PriceAggregator) *LaunchpadService {
	return &LaunchpadService{
		blockchain:  blockchain,
		priceAgg:    priceAgg,
		projects:    make(map[string]*LaunchpadProject),
		allocations: make(map[string][]*Allocation),
		tiers: map[string]*TierConfig{
			"diamond": {
				Name:          "Diamond",
				MinStake:      "100000",
				Allocation:   "5000",
				MaxAllocation: "50000",
			},
			"platinum": {
				Name:          "Platinum",
				MinStake:      "50000",
				Allocation:   "2500",
				MaxAllocation: "25000",
			},
			"gold": {
				Name:          "Gold",
				MinStake:      "10000",
				Allocation:   "1000",
				MaxAllocation: "10000",
			},
			"silver": {
				Name:          "Silver",
				MinStake:      "1000",
				Allocation:   "500",
				MaxAllocation: "5000",
			},
			"bronze": {
				Name:          "Bronze",
				MinStake:      "0",
				Allocation:   "100",
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
		ID:             fmt.Sprintf("proj_%d", time.Now().UnixNano()),
		Name:           name,
		Description:    description,
		TokenAddress:   tokenAddress,
		TokenSymbol:   tokenSymbol,
		TokenName:      tokenName,
		TokenDecimals: 18,
		TotalSupply:    totalSupply,
		TokenPrice:     tokenPrice,
		PaymentToken:   paymentToken,
		MinPurchase:    minPurchase,
		MaxPurchase:    maxPurchase,
		SoftCap:        softCap,
		HardCap:        hardCap,
		StartTime:      startTime,
		EndTime:        endTime,
		Status:         "upcoming",
		RaisedAmount:   "0",
		Participants:   0,
		WebsiteURL:     websiteURL,
		WhitepaperURL:  whitepaperURL,
		LogoURL:        logoURL,
		CreatedBy:      creator,
		CreatedAt:      time.Now(),
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

	// Check min/max purchase
	minPurchase := parseFloat(project.MinPurchase)
	maxPurchase := parseFloat(project.MaxPurchase)
	amountFloat := parseFloat(amount)

	if amountFloat < minPurchase {
		return nil, fmt.Errorf("amount below minimum purchase")
	}
	if amountFloat > maxPurchase {
		return nil, fmt.Errorf("amount exceeds maximum purchase")
	}

	// Determine tier based on allocation
	tier := s.determineTier(userAddress)

	allocation := &Allocation{
		UserAddress:    userAddress,
		ProjectID:     projectID,
		AllocatedAmount: amount,
		ClaimedAmount:  "0",
		Tier:          tier,
		Status:        "pending",
	}

	s.allocations[projectID] = append(s.allocations[projectID], allocation)

	// Update raised amount
	raised := parseFloat(project.RaisedAmount)
	project.RaisedAmount = fmt.Sprintf("%.2f", raised+amountFloat)
	project.Participants++

	// Check if hard cap reached
	if parseFloat(project.RaisedAmount) >= parseFloat(project.HardCap) {
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

	// Calculate token amount
	price := parseFloat(project.TokenPrice)
	paymentAmount := parseFloat(allocation.AllocatedAmount)
	tokenAmount := paymentAmount / price

	allocation.ClaimedAmount = fmt.Sprintf("%.8f", tokenAmount)
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

func (s *LaunchpadService) determineTier(userAddress string) string {
	// In production, this would check user's staked amount
	// For now, return bronze as default
	return "bronze"
}

// GetTiers returns all tier configurations
func (s *LaunchpadService) GetTiers() map[string]*TierConfig {
	return s.tiers
}

// API Handlers

func (s *LaunchpadService) CreateProjectHandler(c *gin.Context) {
	var req struct {
		Creator       string  `json:"creator" binding:"required"`
		Name          string  `json:"name" binding:"required"`
		Description   string  `json:"description"`
		TokenAddress  string  `json:"token_address" binding:"required"`
		TokenSymbol   string  `json:"token_symbol" binding:"required"`
		TokenName     string  `json:"token_name" binding:"required"`
		TotalSupply   string  `json:"total_supply" binding:"required"`
		TokenPrice    string  `json:"token_price" binding:"required"`
		PaymentToken  string  `json:"payment_token" binding:"required"`
		MinPurchase   string  `json:"min_purchase"`
		MaxPurchase   string  `json:"max_purchase"`
		SoftCap       string  `json:"soft_cap"`
		HardCap       string  `json:"hard_cap"`
		StartTime     string  `json:"start_time" binding:"required"`
		EndTime       string  `json:"end_time" binding:"required"`
		WebsiteURL     string  `json:"website_url"`
		WhitepaperURL string  `json:"whitepaper_url"`
		LogoURL       string  `json:"logo_url"`
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

func (s *LaunchpadService) GetProjectsHandler(c *gin.Context) {
	status := c.Query("status")

	projects := s.GetProjects(status)
	c.JSON(200, gin.H{
		"projects": projects,
		"count":   len(projects),
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
		"success":      true,
		"claimed":      amount,
		"message":      "Tokens claimed successfully",
	})
}

func (s *LaunchpadService) GetTiersHandler(c *gin.Context) {
	c.JSON(200, s.GetTiers())
}

// Helper function
func parseFloat(s string) float64 {
	var f float64
	fmt.Sscanf(s, "%f", &f)
	return f
}
