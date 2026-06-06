package main

import (
	"fmt"
	"sync"
	"time"
)

// Compliance Service - Subaccounts, RBAC, Audit Trails, API Keys, IP Whitelisting

type User struct {
	ID           string
	Email        string
	Role         string
	Subaccounts []string
	IPWhitelist  []string
	APIKeys      []APIKey
	CreatedAt    int64
}

type APIKey struct {
	ID          string
	Key         string
	Permissions []string
	ExpiresAt   int64
	LastUsed   int64
	Active     bool
}

type AuditEntry struct {
	ID        string
	UserID    string
	Action   string
	Details  string
	IP       string
	Time     int64
}

type ComplianceService struct {
	mu    sync.RWMutex
	users map[string]User
	audit []AuditEntry
}

func NewComplianceService() *ComplianceService {
	return &ComplianceService{
		users: make(map[string]User),
		audit: make([]AuditEntry, 0),
	}
}

func (c *ComplianceService) CreateUser(email, role string) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	userID := fmt.Sprintf("user_%d", time.Now().Unix())
	c.users[userID] = User{
		ID:          userID,
		Email:       email,
		Role:        role,
		Subaccounts: []string{},
		IPWhitelist: []string{},
		APIKeys:     []APIKey{},
		CreatedAt:   time.Now().Unix(),
	}
	
	c.logAudit(userID, "user_created", "User created")
	
	return userID
}

func (c *ComplianceService) AddSubaccount(parentID, subaccountID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	parent, ok := c.users[parentID]
	if !ok {
		return fmt.Errorf("parent not found")
	}
	
	parent.Subaccounts = append(parent.Subaccounts, subaccountID)
	c.users[parentID] = parent
	
	c.logAudit(parentID, "subaccount_added", subaccountID)
	
	return nil
}

func (c *ComplianceService) SetIPWhitelist(userID string, ips []string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if _, ok := c.users[userID]; !ok {
		return fmt.Errorf("user not found")
	}
	
	user := c.users[userID]
	user.IPWhitelist = ips
	c.users[userID] = user
	
	c.logAudit(userID, "ip_whitelist_updated", fmt.Sprintf("%v", ips))
	
	return nil
}

func (c *ComplianceService) CheckIPAllowed(userID, ip string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	user, ok := c.users[userID]
	if !ok || len(user.IPWhitelist) == 0 {
		return true // No whitelist = allow all
	}
	
	for _, allowed := range user.IPWhitelist {
		if allowed == ip {
			return true
		}
	}
	
	return false
}

func (c *ComplianceService) CreateAPIKey(userID string, permissions []string, expiryHours int) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	user, ok := c.users[userID]
	if !ok {
		return "", fmt.Errorf("user not found")
	}
	
	keyID := fmt.Sprintf("key_%d", time.Now().Unix())
	key := fmt.Sprintf("sk_%d_%s", time.Now().Unix(), userID)
	
	expiresAt := time.Now().Add(time.Duration(expiryHours) * time.Hour).Unix()
	
	apiKey := APIKey{
		ID:          keyID,
		Key:         key,
		Permissions: permissions,
		ExpiresAt:   expiresAt,
		LastUsed:   0,
		Active:     true,
	}
	
	user.APIKeys = append(user.APIKeys, apiKey)
	c.users[userID] = user
	
	c.logAudit(userID, "api_key_created", keyID)
	
	return key, nil
}

func (c *ComplianceService) ValidateAPIKey(userID, key string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	user, ok := c.users[userID]
	if !ok {
		return false
	}
	
	for _, apiKey := range user.APIKeys {
		if apiKey.Key == key && apiKey.Active && apiKey.ExpiresAt > time.Now().Unix() {
			return true
		}
	}
	
	return false
}

func (c *ComplianceService) HasPermission(userID, permission string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	user, ok := c.users[userID]
	if !ok {
		return false
	}
	
	// Check role permissions
	rolePerms := map[string][]string{
		"admin":  {"*"},
		"trader": {"trade", "withdraw"},
		"viewer": {"read"},
	}
	
	perms, ok := rolePerms[user.Role]
	if !ok {
		return false
	}
	
	for _, p := range perms {
		if p == "*" || p == permission {
			return true
		}
	}
	
	return false
}

func (c *ComplianceService) GetAuditTrail(userID string, limit int) []AuditEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	var entries []AuditEntry
	for _, entry := range c.audit {
		if entry.UserID == userID {
			entries = append(entries, entry)
			if limit > 0 && len(entries) >= limit {
				break
			}
		}
	}
	
	return entries
}

func (c *ComplianceService) logAudit(userID, action, details string) {
	entry := AuditEntry{
		ID:       fmt.Sprintf("audit_%d", len(c.audit)),
		UserID:   userID,
		Action:  action,
		Details: details,
		IP:      "",
		Time:    time.Now().Unix(),
	}
	c.audit = append(c.audit, entry)
}

func main() {
	compliance := NewComplianceService()
	
	// Create user
	userID := compliance.CreateUser("user@example.com", "trader")
	fmt.Printf("Created user: %s\n", userID)
	
	// Set IP whitelist
	compliance.SetIPWhitelist(userID, []string{"192.168.1.0/24"})
	
	// Check IP
	allowed := compliance.CheckIPAllowed(userID, "192.168.1.100")
	fmt.Printf("IP allowed: %v\n", allowed)
	
	// Create API key
	key, _ := compliance.CreateAPIKey(userID, []string{"trade", "read"}, 24)
	fmt.Printf("API key: %s\n", key)
	
	// Validate key
	valid := compliance.ValidateAPIKey(userID, key)
	fmt.Printf("Key valid: %v\n", valid)
	
	// Check permission
	hasPerm := compliance.HasPermission(userID, "trade")
	fmt.Printf("Has trade permission: %v\n", hasPerm)
	
	// Get audit trail
	audit := compliance.GetAuditTrail(userID, 10)
	fmt.Printf("Audit entries: %d\n", len(audit))
}