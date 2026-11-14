package services

import (
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/google/uuid"
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
	return s.db.Transaction(func(tx *gorm.DB) error {
		return s.debitWithin(tx, userID, chainID, tokenAddress, amount)
	})
}

// FeeAccountID is the ledger account the platform's collected fees accrue to.
// It is a synthetic account: fees are credited here and settled out to the
// treasury wallet by an operator.
const FeeAccountID = "platform-fees"

// ChargeWithFee debits the amount plus the fee from the payer and credits the
// fee to the platform fee account, all in one transaction. Either the whole
// move lands or none of it does, so a failed charge cannot leave the fee
// account credited for a transfer that never happened. A zero fee behaves like
// a plain debit.
func (s *LedgerService) ChargeWithFee(userID string, chainID int64, tokenAddress string, amount, fee *big.Int) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		return s.chargeWithFeeWithin(tx, userID, chainID, tokenAddress, amount, fee)
	})
}

// chargeWithFeeWithin is the in-transaction form of ChargeWithFee, so a caller
// can commit the charge together with its own bookkeeping record.
func (s *LedgerService) chargeWithFeeWithin(tx *gorm.DB, userID string, chainID int64, tokenAddress string, amount, fee *big.Int) error {
	if err := s.debitWithin(tx, userID, chainID, tokenAddress, amount); err != nil {
		return err
	}
	if fee == nil || fee.Sign() == 0 {
		return nil
	}
	if err := s.debitWithin(tx, userID, chainID, tokenAddress, fee); err != nil {
		return err
	}
	return s.creditWithin(tx, FeeAccountID, chainID, tokenAddress, "", 0, fee)
}

// RefundWithFee reverses a ChargeWithFee: the amount and the fee return to the
// payer and the fee is taken back out of the fee account, in one transaction.
func (s *LedgerService) RefundWithFee(userID string, chainID int64, tokenAddress string, amount, fee *big.Int) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		return s.refundWithFeeWithin(tx, userID, chainID, tokenAddress, amount, fee)
	})
}

// refundWithFeeWithin is the in-transaction form of RefundWithFee.
func (s *LedgerService) refundWithFeeWithin(tx *gorm.DB, userID string, chainID int64, tokenAddress string, amount, fee *big.Int) error {
	if err := s.creditWithin(tx, userID, chainID, tokenAddress, "", 0, amount); err != nil {
		return err
	}
	if fee == nil || fee.Sign() == 0 {
		return nil
	}
	if err := s.debitWithin(tx, FeeAccountID, chainID, tokenAddress, fee); err != nil {
		return err
	}
	return s.creditWithin(tx, userID, chainID, tokenAddress, "", 0, fee)
}

// debitWithin debits a balance using an existing transaction, so a caller can
// commit the debit together with its own bookkeeping record. Either both land or
// neither does, which removes the need for a compensating credit.
func (s *LedgerService) debitWithin(tx *gorm.DB, userID string, chainID int64, tokenAddress string, amount *big.Int) error {
	if amount == nil || amount.Sign() <= 0 {
		return fmt.Errorf("debit amount must be positive")
	}
	return mutateWithin(tx, userID, chainID, tokenAddress, "", 0, func(available *big.Int) (*big.Int, error) {
		if available.Cmp(amount) < 0 {
			return nil, ErrInsufficientBalance
		}
		return new(big.Int).Sub(available, amount), nil
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
		// conjuring funds. Two concurrent first mutations of the same
		// (user, chain, token) would both miss the row above; the upsert
		// makes the loser a no-op instead of a unique-key failure, and the
		// re-read under lock then observes whichever row won.
		row = models.AccountBalance{
			UserID:       userID,
			ChainID:      chainID,
			TokenAddress: tokenKey,
			Symbol:       symbol,
			Decimals:     decimals,
			Available:    "0",
			Locked:       "0",
		}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error; err != nil {
			return err
		}
		// Re-read under lock so the update path is identical for new rows,
		// whether this call created the row or a concurrent one did.
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("user_id = ? AND chain_id = ? AND token_address = ?", userID, chainID, tokenKey).
			First(&row).Error; err != nil {
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

// ReserveWithdrawal debits the amount plus the fee from the payer, credits the
// fee to the platform fee account, and records the withdrawal as a pending
// transaction row, all in one transaction. Recording the row in the same
// transaction as the debit is what makes the payout recoverable: the reconciler
// only sweeps rows that exist, so a debit that committed without its row would
// strand the funds with nothing to reconcile. The row is inserted with an empty
// hash (the partial unique index ignores it) and the hash is written once the
// release is broadcast.
func (s *LedgerService) ReserveWithdrawal(userID string, chainID int64, tokenAddress string, amount, fee *big.Int, toAddress string) (*models.Transaction, error) {
	if amount == nil || amount.Sign() <= 0 {
		return nil, fmt.Errorf("withdrawal amount must be positive")
	}

	var row models.Transaction
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := s.chargeWithFeeWithin(tx, userID, chainID, tokenAddress, amount, fee); err != nil {
			return err
		}
		row = models.Transaction{
			ID:         uuid.New(),
			UserID:     parseUserUUID(userID),
			Hash:       "",
			ChainID:    chainID,
			Type:       "withdraw",
			FromToken:  tokenAddress,
			FromAmount: amount.String(),
			ToAddress:  toAddress,
			Status:     "pending",
			Timestamp:  time.Now(),
		}
		return tx.Create(&row).Error
	})
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// RefundWithdrawal returns a reserved withdrawal's amount and fee to the payer,
// takes the fee back out of the platform fee account, and deletes the pending
// row, all in one transaction. It is the abandonment path: when the release
// cannot be broadcast the reserved funds are returned and the row is removed so
// the reconciler has nothing to sweep. The delete and the refund commit
// together, so a crash cannot leave funds returned with the row still pending
// (which the reconciler would then refund a second time).
func (s *LedgerService) RefundWithdrawal(transactionID interface{}, userID string, chainID int64, tokenAddress string, amount, fee *big.Int) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := s.refundWithFeeWithin(tx, userID, chainID, tokenAddress, amount, fee); err != nil {
			return err
		}
		return tx.Where("id = ? AND type = ? AND status = ?", transactionID, "withdraw", "pending").
			Delete(&models.Transaction{}).Error
	})
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
