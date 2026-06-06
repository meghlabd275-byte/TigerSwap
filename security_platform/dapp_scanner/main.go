package main

import (
	"fmt"
	"sync"
	"time"
)

// DApp Security Scanner
// Contract scanner, phishing detector, URL reputation, risk scoring

type Contract struct {
	Address  string `json:"address"`
	Verified bool   `json:"verified"`
	Score    int    `json:"score"`
	Issues   []string `json:"issues"`
}

type PhishingSite struct {
	Domain   string `json:"domain"`
	Reported int64  `json:"reported"`
	Type     string `json:"type"`
}

type SecurityScanner struct {
	mu          sync.RWMutex
	contracts  map[string]Contract
	phishing   map[string]PhishingSite
	blacklist  map[string]bool
}

func NewSecurityScanner() *SecurityScanner {
	return &SecurityScanner{
		contracts: make(map[string]Contract),
		phishing: make(map[string]PhishingSite),
		blacklist: make(map[string]bool),
	}
}

func (s *SecurityScanner) ScanContract(address string) Contract {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// Simplified scanning
	// In production: would analyze bytecode, verify source, check honeypot
	
	issues := []string{}
	score := 100
	
	if !isVerified(address) {
		issues = append(issues, "unverified_source")
		score -= 30
	}
	
	if hasHiddenMint(address) {
		issues = append(issues, "hidden_mint")
		score -= 40
	}
	
	if hasTrapExit(address) {
		issues = append(issues, "trap_exit")
		score -= 50
	}
	
	contract := Contract{
		Address:  address,
		Verified: len(issues) == 0,
		Score:    score,
		Issues:   issues,
	}
	
	s.contracts[address] = contract
	return contract
}

func (s *SecurityScanner) CheckPhishing(domain string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	if _, ok := s.phishing[domain]; ok {
		return true
	}
	
	if _, ok := s.blacklist[domain]; ok {
		return true
	}
	
	return false
}

func (s *SecurityScanner) ReportPhishing(domain, ptype string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.phishing[domain] = PhishingSite{
		Domain:   domain,
		Reported: time.Now().Unix(),
		Type:     ptype,
	}
}

func (s *SecurityScanner) GetRiskScore(address string) int {
	s.mu.RLock()
	
	if contract, ok := s.contracts[address]; ok {
		return contract.Score
	}
	
	s.mu.RUnlock()
	
	// Scan if not cached
	contract := s.ScanContract(address)
	return contract.Score
}

func (s *SecurityScanner) AddToBlacklist(address string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.blacklist[address] = true
}

// Helper functions (simplified)
func isVerified(address string) bool {
	// Would check Etherscan/Blockscout source code
	return len(address) > 10
}

func hasHiddenMint(address string) bool {
	// Would analyze bytecode
	return false
}

func hasTrapExit(address string) bool {
	// Would check for malicious patterns
	return false
}

func main() {
	scanner := NewSecurityScanner()
	
	// Scan contract
	contract := scanner.ScanContract("0x742d35Cc6634C0532925a3b844Bc454e4438f44e")
	fmt.Printf("Contract: %+v\n", contract)
	
	// Check phishing
	scanner.ReportPhishing("fakeuniswap.com", "phishing")
	isPhishing := scanner.CheckPhishing("fakeuniswap.com")
	fmt.Printf("Is phishing: %v\n", isPhishing)
	
	// Get risk score
	score := scanner.GetRiskScore("0x742d35Cc6634C0532925a3b844Bc454e4438f44e")
	fmt.Printf("Risk score: %d\n", score)
}