package main

import (
	"encoding/json"
	"fmt"
	"time"
)

// ============================================================================
// Institutional Service - Custody Workflows
// ============================================================================

// ============================================================================
// Core Types
// ============================================================================

// Entity types
type EntityType string

const (
	EntityTypeCorporate EntityType = "corporate"
	EntityTypeFund     EntityType = "fund"
	EntityTypeCustody  EntityType = "custody"
	EntityTypeIndividual EntityType = "individual"
)

// Approval status
type ApprovalStatus string

const (
	ApprovalStatusPending  ApprovalStatus = "pending"
	ApprovalStatusApproved ApprovalStatus = "approved"
	ApprovalStatusRejected ApprovalStatus = "rejected"
)

// ============================================================================
// Entity
// ============================================================================

// Entity represents a legal entity
type Entity struct {
	ID              string                 `json:"id"`
	Name           string                 `json:"name"`
	EntityType    EntityType             `json:"entity_type"`
	Jurisdiction  string                 `json:"jurisdiction"`
	TaxID         string                 `json:"tax_id"`
	Status        string                 `json:"status"`
	CreatedAt     int64                  `json:"created_at"`
	UpdatedAt     int64                  `json:"updated_at"`
	Metadata     map[string]interface{} `json:"metadata"`
}

// ============================================================================
// Account
// ============================================================================

// Account represents a trading account
type Account struct {
	ID                string             `json:"id"`
	EntityID         string             `json:"entity_id"`
	AccountType     string             `json:"account_type"`
	Name            string             `json:"name"`
	ParentAccountID string             `json:"parent_account_id"`
	Status          string             `json:"status"`
	CreatedAt       int64              `json:"created_at"`
	UpdatedAt       int64              `json:"updated_at"`
	Balances        map[string]Balance  `json:"balances"`
}

// Balance represents token balance
type Balance struct {
	Symbol   string  `json:"symbol"`
	Amount  float64 `json:"amount"`
	Value   float64 `json:"value"`
}

// ============================================================================
// Team Member
// ============================================================================

// TeamMember represents a team member
type TeamMember struct {
	ID        string   `json:"id"`
	EntityID string   `json:"entity_id"`
	Name    string   `json:"name"`
	Email   string   `json:"email"`
	Role    string   `json:"role"`
	Status  string   `json:"status"`
}

// ============================================================================
// Approval Workflow
// ============================================================================

// Approval represents an approval request
type Approval struct {
	ID            string         `json:"id"`
	EntityID      string         `json:"entity_id"`
	Type         string         `json:"type"`
	RequesterID  string         `json:"requester_id"`
	Amount       float64        `json:"amount"`
	Currency     string         `json:"currency"`
	Status      ApprovalStatus `json:"status"`
	Approvers   []Approver     `json:"approvers"`
	CreatedAt   int64          `json:"created_at"`
	CompletedAt *int64        `json:"completed_at"`
	Reason      string         `json:"reason"`
}

// Approver represents an approver
type Approver struct {
	UserID      string `json:"user_id"`
	Name       string `json:"name"`
	ApprovedAt *int64 `json:"approved_at"`
	Comment    string `json:"comment"`
}

// ============================================================================
// Treasury
// ============================================================================

// Treasury represents multi-entity treasury
type Treasury struct {
	ID             string          `json:"id"`
	EntityID       string          `json:"entity_id"`
	Name          string          `json:"name"`
	Accounts     []string        `json:"accounts"`
	Allocations  []Allocation   `json:"allocations"`
	Status       string         `json:"status"`
	CreatedAt    int64           `json:"created_at"`
}

// Allocation represents fund allocation
type Allocation struct {
	Category    string  `json:"category"`
	MinPercent float64 `json:"min_percent"`
	MaxPercent float64 `json:"max_percent"`
	Current    float64 `json:"current"`
}

// ============================================================================
// Portfolio
// ============================================================================

// Portfolio represents investment portfolio
type Portfolio struct {
	ID           string      `json:"id"`
	EntityID    string      `json:"entity_id"`
	Name       string      `json:"name"`
	Holdings   []Holding  `json:"holdings"`
	CreatedAt  int64       `json:"created_at"`
	UpdatedAt int64       `json:"updated_at"`
}

// Holding represents token holding
type Holding struct {
	Symbol    string  `json:"symbol"`
	Address  string  `json:"address"`
	Amount   float64 `json:"amount"`
	ValueUSD float64 `json:"value_usd"`
	Weight  float64 `json:"weight"`
}

// ============================================================================
// Service
// ============================================================================

// InstitutionalService manages institutional workflows
type InstitutionalService struct {
	entities   map[string]*Entity
	accounts  map[string]*Account
	members   map[string]*TeamMember
	approvals map[string]*Approval
	treasury  map[string]*Treasury
	portfolio map[string]*Portfolio
}

// NewInstitutionalService creates new service
func NewInstitutionalService() *InstitutionalService {
	return &InstitutionalService{
		entities:   make(map[string]*Entity),
		accounts:  make(map[string]*Account),
		members:   make(map[string]*TeamMember),
		approvals: make(map[string]*Approval),
		treasury:  make(map[string]*Treasury),
		portfolio: make(map[string]*Portfolio),
	}
}

// ============================================================================
// Entity Operations
// ============================================================================

// CreateEntity creates new entity
func (s *InstitutionalService) CreateEntity(name string, entityType EntityType, jurisdiction, taxID string) (*Entity, error) {
	entity := &Entity{
		ID:         generateID(),
		Name:       name,
		EntityType: entityType,
		Jurisdiction: jurisdiction,
		TaxID:      taxID,
		Status:     "active",
		CreatedAt:  time.Now().Unix(),
		UpdatedAt: time.Now().Unix(),
		Metadata:  make(map[string]interface{}),
	}
	
	s.entities[entity.ID] = entity
	return entity, nil
}

// GetEntity gets entity by ID
func (s *InstitutionalService) GetEntity(id string) (*Entity, error) {
	if entity, ok := s.entities[id]; ok {
		return entity, nil
	}
	return nil, fmt.Errorf("entity not found")
}

// ============================================================================
// Account Operations
// ============================================================================

// CreateAccount creates new account
func (s *InstitutionalService) CreateAccount(entityID, accountType, name string) (*Account, error) {
	// Verify entity exists
	if _, ok := s.entities[entityID]; !ok {
		return nil, fmt.Errorf("entity not found")
	}
	
	account := &Account{
		ID:            generateID(),
		EntityID:      entityID,
		AccountType:  accountType,
		Name:         name,
		Status:       "active",
		CreatedAt:    time.Now().Unix(),
		UpdatedAt:    time.Now().Unix(),
		Balances:     make(map[string]Balance),
	}
	
	s.accounts[account.ID] = account
	return account, nil
}

// CreateSubAccount creates sub-account
func (s *InstitutionalService) CreateSubAccount(parentID, name string) (*Account, error) {
	parent, ok := s.accounts[parentID]
	if !ok {
		return nil, fmt.Errorf("parent account not found")
	}
	
	account := &Account{
		ID:                generateID(),
		EntityID:          parent.EntityID,
		AccountType:       "subaccount",
		Name:              name,
		ParentAccountID:   parentID,
		Status:            "active",
		CreatedAt:         time.Now().Unix(),
		UpdatedAt:         time.Now().Unix(),
		Balances:          make(map[string]Balance),
	}
	
	s.accounts[account.ID] = account
	return account, nil
}

// GetAccounts gets all accounts for entity
func (s *InstitutionalService) GetAccounts(entityID string) []*Account {
	var result []*Account
	for _, account := range s.accounts {
		if account.EntityID == entityID {
			result = append(result, account)
		}
	}
	return result
}

// ============================================================================
// Team Member Operations
// ============================================================================

// AddTeamMember adds team member
func (s *InstitutionalService) AddTeamMember(entityID, name, email, role string) (*TeamMember, error) {
	member := &TeamMember{
		ID:       generateID(),
		EntityID: entityID,
		Name:    name,
		Email:   email,
		Role:    role,
		Status:  "active",
	}
	
	s.members[member.ID] = member
	return member, nil
}

// GetTeamMembers gets all team members for entity
func (s *InstitutionalService) GetTeamMembers(entityID string) []*TeamMember {
	var result []*TeamMember
	for _, member := range s.members {
		if member.EntityID == entityID {
			result = append(result, member)
		}
	}
	return result
}

// ============================================================================
// Approval Workflow
// ============================================================================

// CreateApproval creates approval request
func (s *InstitutionalService) CreateApproval(entityID, requesterID, approvalType string, amount float64, currency string) (*Approval, error) {
	approval := &Approval{
		ID:           generateID(),
		EntityID:     entityID,
		Type:        approvalType,
		RequesterID: requesterID,
		Amount:      amount,
		Currency:    currency,
		Status:      ApprovalStatusPending,
		Approvers:  []Approver{},
		CreatedAt:  time.Now().Unix(),
		Reason:    "",
	}
	
	s.approvals[approval.ID] = approval
	return approval, nil
}

// Approve approval request
func (s *InstitutionalService) Approve(approvalID, userID, name, comment string) error {
	approval, ok := s.approvals[approvalID]
	if !ok {
		return fmt.Errorf("approval not found")
	}
	
	if approval.Status != ApprovalStatusPending {
		return fmt.Errorf("approval not pending")
	}
	
	now := time.Now().Unix()
	approval.Approvers = append(approval.Approvers, Approver{
		UserID:      userID,
		Name:       name,
		ApprovedAt: &now,
		Comment:    comment,
	})
	
	// Check if all required approvers approved
	// For now, auto-approve after one approval
	approval.Status = ApprovalStatusApproved
	approval.CompletedAt = &now
	
	return nil
}

// Reject approval request
func (s *InstitutionalService) Reject(approvalID, userID, name, comment string) error {
	approval, ok := s.approvals[approvalID]
	if !ok {
		return fmt.Errorf("approval not found")
	}
	
	if approval.Status != ApprovalStatusPending {
		return fmt.Errorf("approval not pending")
	}
	
	now := time.Now().Unix()
	approval.Status = ApprovalStatusRejected
	approval.CompletedAt = &now
	
	return nil
}

// ============================================================================
// Treasury Operations
// ============================================================================

// CreateTreasury creates treasury
func (s *InstitutionalService) CreateTreasury(entityID, name string, accountIDs []string) (*Treasury, error) {
	treasury := &Treasury{
		ID:        generateID(),
		EntityID: entityID,
		Name:     name,
		Accounts: accountIDs,
		Allocations: []Allocation{
			{Category: "defi", MinPercent: 0, MaxPercent: 50, Current: 0},
			{Category: "stablecoins", MinPercent: 20, MaxPercent: 80, Current: 0},
			{Category: "bluechip", MinPercent: 10, MaxPercent: 60, Current: 0},
		},
		Status:    "active",
		CreatedAt: time.Now().Unix(),
	}
	
	s.treasury[treasury.ID] = treasury
	return treasury, nil
}

// UpdateAllocation updates treasury allocation
func (s *InstitutionalService) UpdateAllocation(treasuryID, category string, current float64) error {
	treasury, ok := s.treasury[treasuryID]
	if !ok {
		return fmt.Errorf("treasury not found")
	}
	
	for i := range treasury.Allocations {
		if treasury.Allocations[i].Category == category {
			// Validate bounds
			if current < treasury.Allocations[i].MinPercent || current > treasury.Allocations[i].MaxPercent {
				return fmt.Errorf("allocation out of bounds")
			}
			treasury.Allocations[i].Current = current
			return nil
		}
	}
	
	return fmt.Errorf("category not found")
}

// ============================================================================
// Portfolio Operations
// ============================================================================

// CreatePortfolio creates portfolio
func (s *InstitutionalService) CreatePortfolio(entityID, name string) (*Portfolio, error) {
	portfolio := &Portfolio{
		ID:        generateID(),
		EntityID: entityID,
		Name:     name,
		Holdings: []Holding{},
		CreatedAt: time.Now().Unix(),
		UpdatedAt: time.Now().Unix(),
	}
	
	s.portfolio[portfolio.ID] = portfolio
	return portfolio, nil
}

// AddHolding adds holding to portfolio
func (s *InstitutionalService) AddHolding(portfolioID, symbol, address string, amount, valueUSD float64) error {
	portfolio, ok := s.portfolio[portfolioID]
	if !ok {
		return fmt.Errorf("portfolio not found")
	}
	
	// Calculate weight
	totalValue := valueUSD
	for _, h := range portfolio.Holdings {
		totalValue += h.ValueUSD
	}
	
	weight := 0.0
	if totalValue > 0 {
		weight = valueUSD / totalValue * 100
	}
	
	holding := Holding{
		Symbol:    symbol,
		Address:   address,
		Amount:   amount,
		ValueUSD:  valueUSD,
		Weight:    weight,
	}
	
	portfolio.Holdings = append(portfolio.Holdings, holding)
	portfolio.UpdatedAt = time.Now().Unix()
	
	return nil
}

// ============================================================================
// Utility
// ============================================================================

func generateID() string {
	return fmt.Sprintf("inst_%d", time.Now().UnixNano())
}

// ============================================================================
// Main
// ============================================================================

func main() {
	fmt.Println("TigerSwap Institutional Service")
	fmt.Println("=================================")
	
	// Create service
	service := NewInstitutionalService()
	
	// Create corporate entity
	entity, err := service.CreateEntity("Acme Corp", EntityTypeCorporate, "US", "12-3456789")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Printf("Created entity: %s (%s)\n", entity.Name, entity.EntityType)
	
	// Create main trading account
	account, err := service.CreateAccount(entity.ID, "trading", "Main Trading")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Printf("Created account: %s\n", account.Name)
	
	// Create sub-account
	subAccount, err := service.CreateSubAccount(account.ID, "Market Making")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Printf("Created sub-account: %s\n", subAccount.Name)
	
	// Add team members
	member1, _ := service.AddTeamMember(entity.ID, "John Doe", "john@acme.com", "trader")
	member2, _ := service.AddTeamMember(entity.ID, "Jane Smith", "jane@acme.com", "manager")
	fmt.Printf("Added team members: %s, %s\n", member1.Name, member2.Name)
	
	// Create approval
	approval, _ := service.CreateApproval(entity.ID, member1.ID, "withdrawal", 100000, "USDC")
	fmt.Printf("Created approval: %s for %s %s\n", approval.Type, fmt.Sprintf("%.2f", approval.Amount), approval.Currency)
	
	// Approve
	err = service.Approve(approval.ID, member2.ID, member2.Name, "Approved")
	if err != nil {
		fmt.Println("Error:", err)
	}
	fmt.Printf("Approval status: %s\n", approval.Status)
	
	// Create treasury
	treasury, _ := service.CreateTreasury(entity.ID, "Main Treasury", []string{account.ID})
	fmt.Printf("Created treasury: %s\n", treasury.Name)
	
	// Update allocation
	err = service.UpdateAllocation(treasury.ID, "defi", 30)
	if err != nil {
		fmt.Println("Error:", err)
	}
	fmt.Printf("Updated defi allocation to 30%%\n")
	
	// Create portfolio
	portfolio, _ := service.CreatePortfolio(entity.ID, "DeFi Portfolio")
	fmt.Printf("Created portfolio: %s\n", portfolio.Name)
	
	// Add holdings
	service.AddHolding(portfolio.ID, "ETH", "0x...", 100, 200000)
	service.AddHolding(portfolio.ID, "USDC", "0x...", 500000, 500000)
	fmt.Printf("Added holdings to portfolio\n")
	
	fmt.Println("\nInstitutional Service ready!")
}