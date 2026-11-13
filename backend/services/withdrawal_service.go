package services

import (
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"tigerswap/backend/models"
)

// Withdrawal security policy.
//
// Three controls guard an outbound transfer:
//
//   - An address allow-list. A destination must be added by the account owner
//     before it can receive funds.
//   - A cooldown on a newly added address, so a stolen session cannot add an
//     attacker's address and drain the account in the same window.
//   - A per-asset limit. A withdrawal above the limit is not rejected; it is
//     held in an approval queue for an operator.
//
// The controls are read from the database per request, so an operator change
// takes effect immediately without a restart.

// defaultWithdrawalCooldown is how long a freshly added address must wait before
// it can receive funds.
const defaultWithdrawalCooldown = 24 * time.Hour

// ErrWithdrawalPolicyUnconfigured is returned when the withdrawal path runs
// without a policy, which must never happen in a wired server.
var ErrWithdrawalPolicyUnconfigured = errors.New("withdrawal policy is not configured")

// WithdrawalDecision is the outcome of evaluating a withdrawal against policy.
type WithdrawalDecision struct {
	// Allowed is false when the destination is not permitted at all.
	Allowed bool
	// Reason explains a rejection to the caller.
	Reason string
	// RequiresApproval is true when the amount exceeds the asset limit and the
	// request must be queued for an operator.
	RequiresApproval bool
}

// WithdrawalService enforces the withdrawal policy and owns the approval queue.
type WithdrawalService struct {
	db         *gorm.DB
	ledger     *LedgerService
	blockchain *BlockchainClient
	signer     func() (*ecdsa.PrivateKey, error)
	// feeCollector supplies the withdrawal fee charged when a request is queued.
	// It is nil only in a test that does not charge a fee.
	feeCollector *FeeCollectorService

	requireAllowlist bool
	cooldown         time.Duration
}

// SetFeeCollector wires the fee configuration the queued-withdrawal path charges.
// Without it a queued withdrawal charges no fee.
func (s *WithdrawalService) SetFeeCollector(collector *FeeCollectorService) {
	s.feeCollector = collector
}

// withdrawalFee returns the fee for an amount, or zero when no collector is
// configured. It never fabricates a fee.
func (s *WithdrawalService) withdrawalFee(amount *big.Int) (*big.Int, error) {
	if s.feeCollector == nil {
		return big.NewInt(0), nil
	}
	return s.feeCollector.WithdrawalFeeExact(amount)
}

// NewWithdrawalService builds the policy from configuration. The allow-list is
// required unless WITHDRAWAL_ALLOWLIST_REQUIRED is explicitly "false"; the
// cooldown defaults to 24 hours and is overridable in whole hours.
func NewWithdrawalService(db *gorm.DB, ledger *LedgerService, blockchain *BlockchainClient, signer func() (*ecdsa.PrivateKey, error)) *WithdrawalService {
	requireAllowlist := true
	if raw := strings.TrimSpace(os.Getenv("WITHDRAWAL_ALLOWLIST_REQUIRED")); strings.EqualFold(raw, "false") {
		requireAllowlist = false
	}
	cooldown := defaultWithdrawalCooldown
	if raw := strings.TrimSpace(os.Getenv("WITHDRAWAL_ADDRESS_COOLDOWN_HOURS")); raw != "" {
		if hours, err := strconv.Atoi(raw); err == nil && hours >= 0 {
			cooldown = time.Duration(hours) * time.Hour
		}
	}
	return &WithdrawalService{
		db:               db,
		ledger:           ledger,
		blockchain:       blockchain,
		signer:           signer,
		requireAllowlist: requireAllowlist,
		cooldown:         cooldown,
	}
}

// Evaluate applies the policy to a prospective withdrawal. It never mutates
// state; the caller decides whether to reject, queue, or proceed.
func (s *WithdrawalService) Evaluate(userID string, chainID int64, tokenAddress, toAddress string, amount *big.Int) (WithdrawalDecision, error) {
	if s.requireAllowlist {
		destination := strings.ToLower(toAddress)
		var allowed models.WithdrawalAddress
		err := s.db.Where("user_id = ? AND chain_id = ? AND address = ?", userID, chainID, destination).
			First(&allowed).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return WithdrawalDecision{Allowed: false, Reason: "destination address is not on your allow-list"}, nil
		}
		if err != nil {
			return WithdrawalDecision{}, err
		}
		if time.Now().UTC().Before(allowed.ReadyAt) {
			return WithdrawalDecision{
				Allowed: false,
				Reason:  "this address is still in its security cooldown; it can be used after " + allowed.ReadyAt.UTC().Format(time.RFC3339),
			}, nil
		}
	}

	limit, err := s.limitFor(chainID, tokenAddress)
	if err != nil {
		return WithdrawalDecision{}, err
	}
	if limit != nil {
		maxAmount, ok := new(big.Int).SetString(limit.MaxAmount, 10)
		if !ok {
			return WithdrawalDecision{}, fmt.Errorf("withdrawal limit for chain %d token %s is not a number", chainID, tokenAddress)
		}
		if maxAmount.Sign() > 0 && amount.Cmp(maxAmount) > 0 {
			return WithdrawalDecision{Allowed: true, RequiresApproval: true}, nil
		}
	}
	return WithdrawalDecision{Allowed: true}, nil
}

// limitFor returns the configured limit for an asset, or nil when none is set.
func (s *WithdrawalService) limitFor(chainID int64, tokenAddress string) (*models.WithdrawalLimit, error) {
	var limit models.WithdrawalLimit
	err := s.db.Where("chain_id = ? AND token_address = ?", chainID, normalizeTokenAddress(tokenAddress)).First(&limit).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &limit, nil
}

// CreatePendingRequest debits the caller and records an approval request in one
// transaction. The debit is the reservation: an approved request already holds
// the funds and a refused one returns them.
func (s *WithdrawalService) CreatePendingRequest(userID string, chainID int64, tokenAddress, toAddress string, amount *big.Int) (uuid.UUID, error) {
	fee, err := s.withdrawalFee(amount)
	if err != nil {
		return uuid.Nil, err
	}
	var request models.WithdrawalRequest
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := s.ledger.chargeWithFeeWithin(tx, userID, chainID, tokenAddress, amount, fee); err != nil {
			return err
		}
		request = models.WithdrawalRequest{
			UserID:       userID,
			ChainID:      chainID,
			TokenAddress: normalizeTokenAddress(tokenAddress),
			ToAddress:    strings.ToLower(toAddress),
			Amount:       amount.String(),
			Fee:          fee.String(),
			Status:       models.WithdrawalStatusPending,
		}
		return tx.Create(&request).Error
	})
	if err != nil {
		return uuid.Nil, err
	}
	return request.ID, nil
}

// AddAddress adds an allow-list entry for the caller. A duplicate add returns the
// existing row rather than failing, and the cooldown is measured from first add.
func (s *WithdrawalService) AddAddress(c *gin.Context) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}
	var req struct {
		ChainID int64  `json:"chain_id" binding:"required"`
		Address string `json:"address" binding:"required"`
		Label   string `json:"label"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequestError(c, err)
		return
	}
	if !common.IsHexAddress(req.Address) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid address"})
		return
	}
	address := strings.ToLower(common.HexToAddress(req.Address).Hex())

	var existing models.WithdrawalAddress
	err := s.db.Where("user_id = ? AND chain_id = ? AND address = ?", userID, req.ChainID, address).First(&existing).Error
	if err == nil {
		c.JSON(http.StatusOK, gin.H{"address": existing, "message": "address is already allow-listed"})
		return
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		respondInternalError(c, err)
		return
	}

	entry := models.WithdrawalAddress{
		UserID:  userID,
		ChainID: req.ChainID,
		Address: address,
		Label:   strings.TrimSpace(req.Label),
		ReadyAt: time.Now().UTC().Add(s.cooldown),
	}
	if err := s.db.Create(&entry).Error; err != nil {
		respondInternalError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"address": entry})
}

// ListAddresses returns the caller's allow-list entries.
func (s *WithdrawalService) ListAddresses(c *gin.Context) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}
	var entries []models.WithdrawalAddress
	if err := s.db.Where("user_id = ?", userID).Order("created_at desc").Find(&entries).Error; err != nil {
		respondInternalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"addresses": entries, "cooldown_hours": int(s.cooldown.Hours())})
}

// DeleteAddress removes an allow-list entry that belongs to the caller. An entry
// owned by another account is reported as not found rather than forbidden, so the
// endpoint cannot confirm another account's addresses.
func (s *WithdrawalService) DeleteAddress(c *gin.Context) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}
	id := c.Param("id")
	result := s.db.Where("id = ? AND user_id = ?", id, userID).Delete(&models.WithdrawalAddress{})
	if result.Error != nil {
		respondInternalError(c, result.Error)
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "address not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

// SetLimit creates or replaces the limit for an asset (admin only).
func (s *WithdrawalService) SetLimit(c *gin.Context) {
	var req struct {
		ChainID      int64  `json:"chain_id" binding:"required"`
		TokenAddress string `json:"token_address" binding:"required"`
		MaxAmount    string `json:"max_amount" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequestError(c, err)
		return
	}
	amount, ok := new(big.Int).SetString(req.MaxAmount, 10)
	if !ok || amount.Sign() < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "max_amount must be a non-negative integer string"})
		return
	}
	token := normalizeTokenAddress(req.TokenAddress)

	limit := models.WithdrawalLimit{
		ChainID:      req.ChainID,
		TokenAddress: token,
		MaxAmount:    amount.String(),
	}
	// Upsert on the (chain, token) unique index so re-setting a limit replaces it.
	if err := s.db.Where("chain_id = ? AND token_address = ?", req.ChainID, token).
		Assign(map[string]interface{}{"max_amount": amount.String()}).
		FirstOrCreate(&limit).Error; err != nil {
		respondInternalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"limit": limit})
}

// ListLimits returns every configured limit (admin only).
func (s *WithdrawalService) ListLimits(c *gin.Context) {
	var limits []models.WithdrawalLimit
	if err := s.db.Order("chain_id asc").Find(&limits).Error; err != nil {
		respondInternalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"limits": limits})
}

// ListRequests returns the approval queue (admin only). A `status` query
// parameter filters it; an unknown status is rejected rather than ignored.
func (s *WithdrawalService) ListRequests(c *gin.Context) {
	query := s.db.Model(&models.WithdrawalRequest{})
	if status := strings.TrimSpace(c.Query("status")); status != "" {
		switch status {
		case models.WithdrawalStatusPending, models.WithdrawalStatusApproved, models.WithdrawalStatusRefused:
			query = query.Where("status = ?", status)
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "unknown status"})
			return
		}
	}
	var requests []models.WithdrawalRequest
	if err := query.Order("created_at desc").Find(&requests).Error; err != nil {
		respondInternalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"requests": requests})
}

// ApproveRequest releases a queued withdrawal. The request is claimed with a
// conditional update before the chain call, so two concurrent approvals cannot
// both send; a failed send returns the claim to pending for a retry.
func (s *WithdrawalService) ApproveRequest(c *gin.Context) {
	id := c.Param("id")
	if s.blockchain == nil || s.signer == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "withdrawal signer is not configured"})
		return
	}

	request, err := s.claimRequest(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusConflict, gin.H{"error": "request is not pending"})
			return
		}
		respondInternalError(c, err)
		return
	}

	key, err := s.signer()
	if err != nil {
		s.releaseClaim(request.ID)
		log.Printf("wallet: platform signer unavailable: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "platform signer unavailable"})
		return
	}

	amount, ok := new(big.Int).SetString(request.Amount, 10)
	if !ok {
		s.releaseClaim(request.ID)
		respondInternalError(c, fmt.Errorf("stored amount %q is not a number", request.Amount))
		return
	}

	// The fee was moved to the platform fee account when the request was queued,
	// so approving only releases the amount. The fee is reported back but not
	// charged again.
	fee := strings.TrimSpace(request.Fee)
	if fee == "" {
		fee = "0"
	}
	if _, ok := new(big.Int).SetString(fee, 10); !ok {
		s.releaseClaim(request.ID)
		respondInternalError(c, fmt.Errorf("stored fee %q is not a number", request.Fee))
		return
	}

	txHash, err := s.blockchain.SendTransaction(
		c.Request.Context(), request.ChainID, hex.EncodeToString(crypto.FromECDSA(key)), request.ToAddress, amount, request.TokenAddress, nil,
	)
	if err != nil {
		s.releaseClaim(request.ID)
		log.Printf("wallet: approved withdrawal send failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "withdrawal failed; the request remains pending"})
		return
	}

	if err := s.db.Model(&models.WithdrawalRequest{}).Where("id = ?", request.ID).
		Updates(map[string]interface{}{"status": models.WithdrawalStatusApproved, "tx_hash": txHash}).Error; err != nil {
		respondInternalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": models.WithdrawalStatusApproved, "tx_hash": txHash, "fee": fee})
}

// RefuseRequest returns a queued withdrawal's funds to the caller and closes it.
func (s *WithdrawalService) RefuseRequest(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Reason string `json:"reason"`
	}
	// A missing body is allowed; the reason is optional.
	_ = c.ShouldBindJSON(&req)

	var request models.WithdrawalRequest
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).First(&request).Error; err != nil {
			return err
		}
		if request.Status != models.WithdrawalStatusPending {
			return errRequestNotPending
		}
		amount, ok := new(big.Int).SetString(request.Amount, 10)
		if !ok {
			return fmt.Errorf("stored amount %q is not a number", request.Amount)
		}
		// The fee reserved at queue time returns with the amount, so the payer is
		// made whole and the fee account no longer holds a fee for a withdrawal
		// that never happened.
		fee := big.NewInt(0)
		if strings.TrimSpace(request.Fee) != "" {
			parsed, ok := new(big.Int).SetString(request.Fee, 10)
			if !ok || parsed.Sign() < 0 {
				return fmt.Errorf("stored fee %q is not a number", request.Fee)
			}
			fee = parsed
		}
		if err := s.ledger.refundWithFeeWithin(tx, request.UserID, request.ChainID, request.TokenAddress, amount, fee); err != nil {
			return err
		}
		return tx.Model(&models.WithdrawalRequest{}).Where("id = ?", request.ID).
			Updates(map[string]interface{}{"status": models.WithdrawalStatusRefused, "reason": strings.TrimSpace(req.Reason)}).Error
	})
	if err != nil {
		if errors.Is(err, errRequestNotPending) || errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusConflict, gin.H{"error": "request is not pending"})
			return
		}
		respondInternalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": models.WithdrawalStatusRefused})
}

// errRequestNotPending marks a request that was already decided.
var errRequestNotPending = errors.New("withdrawal request is not pending")

// claimRequest atomically moves a pending request to "processing" and returns
// the row. The conditional update is what makes the claim exclusive.
func (s *WithdrawalService) claimRequest(id string) (*models.WithdrawalRequest, error) {
	var request models.WithdrawalRequest
	err := s.db.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&models.WithdrawalRequest{}).
			Where("id = ? AND status = ?", id, models.WithdrawalStatusPending).
			Update("status", "processing")
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		return tx.Where("id = ?", id).First(&request).Error
	})
	if err != nil {
		return nil, err
	}
	return &request, nil
}

// releaseClaim returns a claimed request to pending after a failed send.
func (s *WithdrawalService) releaseClaim(id uuid.UUID) {
	if err := s.db.Model(&models.WithdrawalRequest{}).Where("id = ? AND status = ?", id, "processing").
		Update("status", models.WithdrawalStatusPending).Error; err != nil {
		log.Printf("CRITICAL: failed to release withdrawal claim %s: %v", id, err)
	}
}
