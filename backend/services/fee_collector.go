package services

import (
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// FeeCollectorService collects and manages all platform fees
type FeeCollectorService struct {
	mu sync.RWMutex
	
	// Fee configurations
	config *FeeConfig
	
	// Fee collection records
	swapFees       map[string]*FeeRecord
	bridgeFees    map[string]*FeeRecord
	walletFees    map[string]*FeeRecord
	launchpadFees  map[string]*FeeRecord
	depositFees   map[string]*FeeRecord
	withdrawFees  map[string]*FeeRecord
	
	// Revenue tracking
	totalRevenue  *big.Float
	dailyRevenue   map[string]*big.Float
	
	// Master wallet address for fee collection
	masterWallet string
}

// FeeConfig holds all fee configurations
type FeeConfig struct {
	SwapFeePercent         float64 `json:"swap_fee_percent"`
	SwapFeeFixed          float64 `json:"swap_fee_fixed"`
	BridgeFeePercent      float64 `json:"bridge_fee_percent"`
	BridgeFeeFixed       float64 `json:"bridge_fee_fixed"`
	WalletTxFeePercent   float64 `json:"wallet_tx_fee_percent"`
	WalletTxFeeFixed    float64 `json:"wallet_tx_fee_fixed"`
	DepositFeePercent    float64 `json:"deposit_fee_percent"`
	DepositFeeFixed     float64 `json:"deposit_fee_fixed"`
	WithdrawFeePercent   float64 `json:"withdraw_fee_percent"`
	WithdrawFeeFixed    float64 `json:"withdraw_fee_fixed"`
	LaunchpadFeePercent float64 `json:"launchpad_fee_percent"`
	LaunchpadFeeFixed  float64 `json:"launchpad_fee_fixed"`
	ReferralFeePercent float64 `json:"referral_fee_percent"`
}

// FeeRecord represents a collected fee
type FeeRecord struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"` // swap, bridge, wallet, launchpad, deposit, withdraw
	Amount      string    `json:"amount"`
	AmountUSD   float64   `json:"amount_usd"`
	Token       string    `json:"token"`
	ChainID     int64     `json:"chain_id"`
	UserAddress string    `json:"user_address"`
	TxHash     string    `json:"tx_hash"`
	FeeCharged float64   `json:"fee_charged"`
	FeeChargedUSD float64 `json:"fee_charged_usd"`
	Timestamp   time.Time `json:"timestamp"`
	Status     string    `json:"status"` // pending, collected, distributed
}

// RevenueReport represents revenue summary
type RevenueReport struct {
	TotalSwapFees      float64 `json:"total_swap_fees"`
	TotalBridgeFees   float64 `json:"total_bridge_fees"`
	TotalWalletFees  float64 `json:"total_wallet_fees"`
	TotalDepositFees float64 `json:"total_deposit_fees"`
	TotalWithdrawFees float64 `json:"total_withdraw_fees"`
	TotalLaunchpadFees float64 `json:"total_launchpad_fees"`
	TotalRevenue    float64 `json:"total_revenue"`
	DailyRevenue   float64 `json:"daily_revenue"`
	WeeklyRevenue  float64 `json:"weekly_revenue"`
	MonthlyRevenue float64 `json:"monthly_revenue"`
	Period        string   `json:"period"`
}

func NewFeeCollectorService() *FeeCollectorService {
	return &FeeCollectorService{
		config: &FeeConfig{
			SwapFeePercent:        0.3,     // 0.3% per swap
			SwapFeeFixed:         0.0,
			BridgeFeePercent:     0.1,     // 0.1% per bridge
			BridgeFeeFixed:       1.0,     // $1 minimum
			WalletTxFeePercent:  0.05,    // 0.05% per tx
			WalletTxFeeFixed:    0.01,    // $0.01 minimum
			DepositFeePercent:   0.0,     // No deposit fee
			DepositFeeFixed:    0.0,
			WithdrawFeePercent:  0.1,     // 0.1% per withdrawal
			WithdrawFeeFixed:  1.0,      // $1 minimum
			LaunchpadFeePercent: 5.0,     // 5% of sale amount
			LaunchpadFeeFixed:  0.0,
			ReferralFeePercent: 0.5,     // 0.5% referral reward
		},
		swapFees:       make(map[string]*FeeRecord),
		bridgeFees:    make(map[string]*FeeRecord),
		walletFees:    make(map[string]*FeeRecord),
		launchpadFees: make(map[string]*FeeRecord),
		depositFees:   make(map[string]*FeeRecord),
		withdrawFees: make(map[string]*FeeRecord),
		totalRevenue: big.NewFloat(0),
		dailyRevenue: make(map[string]*big.Float),
		masterWallet: "",
	}
}

// SetMasterWallet sets the master wallet address for fee collection
func (s *FeeCollectorService) SetMasterWallet(address string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.masterWallet = address
}

// UpdateConfig updates fee configuration
func (s *FeeCollectorService) UpdateConfig(config *FeeConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config = config
	return nil
}

// GetConfig returns current fee configuration
func (s *FeeCollectorService) GetConfig() *FeeConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config
}

// CollectSwapFee calculates and records swap fee
func (s *FeeCollectorService) CollectSwapFee(tokenIn, tokenOut, amount, txHash string, chainID int64, userAddress string, priceIn, priceOut float64) (*FeeRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	amountFloat, err := parseFloat(amount)
	if err != nil {
		return nil, fmt.Errorf("invalid amount: %v", err)
	}
	
	// Calculate fee
	feeAmount := s.calculateFee(amountFloat, s.config.SwapFeePercent, s.config.SwapFeeFixed)
	
	// Calculate USD value
	feeUSD := feeAmount * priceIn
	
	record := &FeeRecord{
		ID:            fmt.Sprintf("swap_%d", time.Now().UnixNano()),
		Type:          "swap",
		Amount:        fmt.Sprintf("%.8f", feeAmount),
		AmountUSD:     feeUSD,
		Token:         tokenIn,
		ChainID:       chainID,
		UserAddress:   userAddress,
		TxHash:       txHash,
		FeeCharged:    feeAmount,
		FeeChargedUSD: feeUSD,
		Timestamp:     time.Now(),
		Status:        "collected",
	}
	
	s.swapFees[record.ID] = record
	s.totalRevenue.Add(s.totalRevenue, big.NewFloat(feeUSD))
	
	// Update daily revenue
	s.updateDailyRevenue("swap", feeUSD)
	
	return record, nil
}

// CollectBridgeFee calculates and records bridge fee
func (s *FeeCollectorService) CollectBridgeFee(srcChain, dstChain, token, amount, txHash string, userAddress string, price float64) (*FeeRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	amountFloat, err := parseFloat(amount)
	if err != nil {
		return nil, fmt.Errorf("invalid amount: %v", err)
	}
	
	feeAmount := s.calculateFee(amountFloat, s.config.BridgeFeePercent, s.config.BridgeFeeFixed)
	feeUSD := feeAmount * price
	
	record := &FeeRecord{
		ID:            fmt.Sprintf("bridge_%d", time.Now().UnixNano()),
		Type:          "bridge",
		Amount:        fmt.Sprintf("%.8f", feeAmount),
		AmountUSD:     feeUSD,
		Token:         token,
		UserAddress:   userAddress,
		TxHash:       txHash,
		FeeCharged:    feeAmount,
		FeeChargedUSD: feeUSD,
		Timestamp:     time.Now(),
		Status:        "collected",
	}
	
	s.bridgeFees[record.ID] = record
	s.totalRevenue.Add(s.totalRevenue, big.NewFloat(feeUSD))
	s.updateDailyRevenue("bridge", feeUSD)
	
	return record, nil
}

// CollectWalletFee calculates and records wallet transaction fee
func (s *FeeCollectorService) CollectWalletFee(token, amount, txHash string, chainID int64, userAddress string, price float64) (*FeeRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	amountFloat, err := parseFloat(amount)
	if err != nil {
		return nil, fmt.Errorf("invalid amount: %v", err)
	}
	
	feeAmount := s.calculateFee(amountFloat, s.config.WalletTxFeePercent, s.config.WalletTxFeeFixed)
	feeUSD := feeAmount * price
	
	record := &FeeRecord{
		ID:            fmt.Sprintf("wallet_%d", time.Now().UnixNano()),
		Type:          "wallet",
		Amount:        fmt.Sprintf("%.8f", feeAmount),
		AmountUSD:     feeUSD,
		Token:         token,
		ChainID:       chainID,
		UserAddress:   userAddress,
		TxHash:       txHash,
		FeeCharged:    feeAmount,
		FeeChargedUSD: feeUSD,
		Timestamp:     time.Now(),
		Status:        "collected",
	}
	
	s.walletFees[record.ID] = record
	s.totalRevenue.Add(s.totalRevenue, big.NewFloat(feeUSD))
	s.updateDailyRevenue("wallet", feeUSD)
	
	return record, nil
}

// CollectWithdrawFee calculates and records withdrawal fee
func (s *FeeCollectorService) CollectWithdrawFee(token, amount, txHash string, chainID int64, userAddress string, price float64) (*FeeRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	amountFloat, err := parseFloat(amount)
	if err != nil {
		return nil, fmt.Errorf("invalid amount: %v", err)
	}
	
	feeAmount := s.calculateFee(amountFloat, s.config.WithdrawFeePercent, s.config.WithdrawFeeFixed)
	feeUSD := feeAmount * price
	
	record := &FeeRecord{
		ID:            fmt.Sprintf("withdraw_%d", time.Now().UnixNano()),
		Type:          "withdraw",
		Amount:        fmt.Sprintf("%.8f", feeAmount),
		AmountUSD:     feeUSD,
		Token:         token,
		ChainID:       chainID,
		UserAddress:   userAddress,
		TxHash:       txHash,
		FeeCharged:    feeAmount,
		FeeChargedUSD: feeUSD,
		Timestamp:     time.Now(),
		Status:        "collected",
	}
	
	s.withdrawFees[record.ID] = record
	s.totalRevenue.Add(s.totalRevenue, big.NewFloat(feeUSD))
	s.updateDailyRevenue("withdraw", feeUSD)
	
	return record, nil
}

// CollectLaunchpadFee calculates and records launchpad fee
func (s *FeeCollectorService) CollectLaunchpadFee(projectID, token, amount, txHash, userAddress string, price float64) (*FeeRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	amountFloat, err := parseFloat(amount)
	if err != nil {
		return nil, fmt.Errorf("invalid amount: %v", err)
	}
	
	feeAmount := s.calculateFee(amountFloat, s.config.LaunchpadFeePercent, s.config.LaunchpadFeeFixed)
	feeUSD := feeAmount * price
	
	record := &FeeRecord{
		ID:            fmt.Sprintf("launchpad_%d", time.Now().UnixNano()),
		Type:          "launchpad",
		Amount:        fmt.Sprintf("%.8f", feeAmount),
		AmountUSD:     feeUSD,
		Token:         token,
		UserAddress:   userAddress,
		TxHash:       txHash,
		FeeCharged:    feeAmount,
		FeeChargedUSD: feeUSD,
		Timestamp:     time.Now(),
		Status:        "collected",
	}
	
	s.launchpadFees[record.ID] = record
	s.totalRevenue.Add(s.totalRevenue, big.NewFloat(feeUSD))
	s.updateDailyRevenue("launchpad", feeUSD)
	
	return record, nil
}

// CalculateFee calculates the fee amount
func (s *FeeCollectorService) calculateFee(amount, percent, fixed float64) float64 {
	percentFee := amount * (percent / 100)
	if percentFee < fixed {
		return fixed
	}
	return percentFee
}

func (s *FeeCollectorService) updateDailyRevenue(feeType string, amount float64) {
	today := time.Now().Format("2006-01-02")
	if s.dailyRevenue[today] == nil {
		s.dailyRevenue[today] = big.NewFloat(0)
	}
	s.dailyRevenue[today].Add(s.dailyRevenue[today], big.NewFloat(amount))
}

// GetRevenueReport returns revenue summary
func (s *FeeCollectorService) GetRevenueReport(period string) *RevenueReport {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	report := &RevenueReport{
		Period: period,
	}
	
	// Calculate totals
	for _, fee := range s.swapFees {
		report.TotalSwapFees += fee.FeeChargedUSD
	}
	for _, fee := range s.bridgeFees {
		report.TotalBridgeFees += fee.FeeChargedUSD
	}
	for _, fee := range s.walletFees {
		report.TotalWalletFees += fee.FeeChargedUSD
	}
	for _, fee := range s.withdrawFees {
		report.TotalWithdrawFees += fee.FeeChargedUSD
	}
	for _, fee := range s.launchpadFees {
		report.TotalLaunchpadFees += fee.FeeChargedUSD
	}
	
	report.TotalRevenue = report.TotalSwapFees + report.TotalBridgeFees + 
		report.TotalWalletFees + report.TotalWithdrawFees + report.TotalLaunchpadFees
	
	// Daily revenue
	today := time.Now().Format("2006-01-02")
	if d := s.dailyRevenue[today]; d != nil {
		f, _ := d.Float64()
		report.DailyRevenue = f
	}
	
	return report
}

// GetFeeRecords returns all fee records of a type
func (s *FeeCollectorService) GetFeeRecords(feeType string) []*FeeRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	var records []*FeeRecord
	
	switch feeType {
	case "swap":
		for _, r := range s.swapFees {
			records = append(records, r)
		}
	case "bridge":
		for _, r := range s.bridgeFees {
			records = append(records, r)
		}
	case "wallet":
		for _, r := range s.walletFees {
			records = append(records, r)
		}
	case "withdraw":
		for _, r := range s.withdrawFees {
			records = append(records, r)
		}
	case "launchpad":
		for _, r := range s.launchpadFees {
			records = append(records, r)
		}
	}
	
	return records
}

// GetTotalRevenue returns total revenue
func (s *FeeCollectorService) GetTotalRevenue() float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	f, _ := s.totalRevenue.Float64()
	return f
}

// DistributeFees distributes fees to master wallet
func (s *FeeCollectorService) DistributeFees() (string, error) {
	s.mu.RLock()
	masterWallet := s.masterWallet
	s.mu.RUnlock()
	
	if masterWallet == "" {
		return "", fmt.Errorf("master wallet not configured")
	}
	
	// In production, this would execute a real transaction
	// to transfer all collected fees to master wallet
	return fmt.Sprintf("0x%x", time.Now().UnixNano()), nil
}

// API Handlers

func (s *FeeCollectorService) GetConfigHandler(c *gin.Context) {
	c.JSON(200, s.GetConfig())
}

func (s *FeeCollectorService) UpdateConfigHandler(c *gin.Context) {
	var req FeeConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	
	err := s.UpdateConfig(&req)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(200, gin.H{"success": true, "message": "Fee configuration updated"})
}

func (s *FeeCollectorService) GetRevenueReportHandler(c *gin.Context) {
	period := c.DefaultQuery("period", "daily")
	
	report := s.GetRevenueReport(period)
	c.JSON(200, report)
}

func (s *FeeCollectorService) GetTotalRevenueHandler(c *gin.Context) {
	c.JSON(200, gin.H{
		"total_revenue": s.GetTotalRevenue(),
	})
}

func (s *FeeCollectorService) GetFeeRecordsHandler(c *gin.Context) {
	feeType := c.DefaultQuery("type", "swap")
	
	records := s.GetFeeRecords(feeType)
	c.JSON(200, gin.H{
		"records": records,
		"count":  len(records),
	})
}

func (s *FeeCollectorService) SetMasterWalletHandler(c *gin.Context) {
	var req struct {
		Address string `json:"address" binding:"required"`
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	
	s.SetMasterWallet(req.Address)
	c.JSON(200, gin.H{"success": true, "master_wallet": req.Address})
}

func (s *FeeCollectorService) DistributeFeesHandler(c *gin.Context) {
	txHash, err := s.DistributeFees()
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(200, gin.H{
		"success":   true,
		"tx_hash":  txHash,
		"message":  "Fees distributed to master wallet",
	})
}

func parseFloat(s string) (float64, error) {
	var f float64
	_, err := fmt.Sscanf(s, "%f", &f)
	return f, err
}
