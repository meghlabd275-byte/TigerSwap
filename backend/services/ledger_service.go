package services

import (
	"errors"
	"fmt"
	"math/big"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"tigerswap/backend/models"
)

// ErrInsufficientBalance is returned when a debit would take an account below
// zero. Callers map it to HTTP 400/409 rather than a server error.
var ErrInsufficientBalance = errors.New("insufficient balance")

// LedgerService maintains the platform's internal accounting. Balances are
// exact integers in the token's smallest unit; all mutations run inside a
// transaction and take a row lock so concurrent withdrawals cannot both pass
// the same balance check.
type LedgerService struct {
	db *gorm.DB
}

func NewLedgerService(db *gorm.DB) *LedgerService {
	return &LedgerService{db: db}
}

// normalizeTokenAddress lower-cases an address so the same token always maps to
// one ledger row. The empty string denotes the chain's native token.
func normalizeTokenAddress(address string) string {
	if address == "" {
		return "native"
	}
	return strings.ToLower(address)
}

// Credit adds an amount to a user's available balance.
func (s *LedgerService) Credit(userID string, chainID int64, tokenAddress, symbol string, decimals int, amount *big.Int) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		return s.creditWithin(tx, userID, chainID, tokenAddress, symbol, decimals, amount)
	})
}

// creditWithin credits a balance using an existing transaction, so a caller can
// commit the credit together with its own bookkeeping record.
func (s *LedgerService) creditWithin(tx *gorm.DB, userID string, chainID int64, tokenAddress, symbol string, decimals int, amount *big.Int) error {
	if amount == nil || amount.Sign() <= 0 {
		return fmt.Errorf("credit amount must be positive")
	}
	return mutateWithin(tx, userID, chainID, tokenAddress, symbol, decimals, func(available *big.Int) (*big.Int, error) {
		return new(big.Int).Add(available, amount), nil
	})
}

// Debit subtracts an amount from a user's available balance, failing if the
// balance is insufficient.
func (s *LedgerService) Debit(userID string, chainID int64, tokenAddress string, amount *big.Int) error {
	if amount == nil || amount.Sign() <= 0 {
		return fmt.Errorf("debit amount must be positive")
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		return mutateWithin(tx, userID, chainID, tokenAddress, "", 0, func(available *big.Int) (*big.Int, error) {
			if available.Cmp(amount) < 0 {
				return nil, ErrInsufficientBalance
			}
			return new(big.Int).Sub(available, amount), nil
		})
	})
}

// mutateWithin applies fn to the stored available balance inside the caller's
// transaction. A row lock (SELECT ... FOR UPDATE) prevents two concurrent
// debits from each observing the pre-debit balance.
func mutateWithin(tx *gorm.DB, userID string, chainID int64, tokenAddress, symbol string, decimals int, fn func(*big.Int) (*big.Int, error)) error {
	tokenKey := normalizeTokenAddress(tokenAddress)

	var row models.AccountBalance
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("user_id = ? AND chain_id = ? AND token_address = ?", userID, chainID, tokenKey).
		First(&row).Error

	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		// Creating the row here means a debit with no prior credit is
		// rejected by the zero-balance check in fn rather than silently
		// conjuring funds.
		row = models.AccountBalance{
			UserID:       userID,
			ChainID:      chainID,
			TokenAddress: tokenKey,
			Symbol:       symbol,
			Decimals:     decimals,
			Available:    "0",
			Locked:       "0",
		}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		// Re-read under lock so the update path is identical for new rows.
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", row.ID).First(&row).Error; err != nil {
			return err
		}
	case err != nil:
		return err
	}

	available, ok := new(big.Int).SetString(row.Available, 10)
	if !ok {
		available = big.NewInt(0)
	}

	next, err := fn(available)
	if err != nil {
		return err
	}

	row.Available = next.String()
	if symbol != "" {
		row.Symbol = symbol
	}
	if decimals != 0 {
		row.Decimals = decimals
	}

	return tx.Model(&models.AccountBalance{}).
		Where("id = ?", row.ID).
		Update("available", row.Available).Error
}

// Available returns a user's available balance for a token, or zero if the
// account has no ledger row yet.
func (s *LedgerService) Available(userID string, chainID int64, tokenAddress string) (*big.Int, error) {
	var row models.AccountBalance
	err := s.db.Where("user_id = ? AND chain_id = ? AND token_address = ?",
		userID, chainID, normalizeTokenAddress(tokenAddress)).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return big.NewInt(0), nil
	}
	if err != nil {
		return nil, err
	}
	available, ok := new(big.Int).SetString(row.Available, 10)
	if !ok {
		return big.NewInt(0), nil
	}
	return available, nil
}

// Balances lists every ledger row for a user.
func (s *LedgerService) Balances(userID string) ([]models.AccountBalance, error) {
	rows := []models.AccountBalance{}
	if err := s.db.Where("user_id = ?", userID).Order("chain_id asc, symbol asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}
