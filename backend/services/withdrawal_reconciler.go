package services

import (
	"context"
	"errors"
	"log"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"tigerswap/backend/models"
)

// DefaultWithdrawalReconcileInterval is how often pending withdrawals are
// checked against their on-chain receipt.
const DefaultWithdrawalReconcileInterval = 30 * time.Second

// StartWithdrawalReconciler runs the receipt reconciler until ctx is cancelled.
// It is the single writer that moves a withdrawal out of "pending": on a
// confirmed receipt the row is marked confirmed, and on a reverted receipt the
// debit is reversed and the row is marked failed. Without it a mined-and-reverted
// withdrawal would leave the user debited with a pending row forever.
func StartWithdrawalReconciler(ctx context.Context, db *gorm.DB, blockchain *BlockchainClient, ledger *LedgerService, webhooks *WebhookService, fees *FeeCollectorService, interval time.Duration) {
	if db == nil || blockchain == nil || ledger == nil {
		log.Printf("withdrawal reconciler disabled: db, blockchain or ledger not configured")
		return
	}
	if interval <= 0 {
		interval = DefaultWithdrawalReconcileInterval
	}
	reconciler := &withdrawalReconciler{db: db, blockchain: blockchain, ledger: ledger, webhooks: webhooks, fees: fees}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				reconciler.sweep(ctx)
			}
		}
	}()
}

type withdrawalReconciler struct {
	db         *gorm.DB
	blockchain *BlockchainClient
	ledger     *LedgerService
	webhooks   *WebhookService
	fees       *FeeCollectorService
}

// withdrawalFeePaid returns the exact fee charged to the ledger for a withdrawal,
// read from the persisted fee record the withdrawal path wrote. It returns zero
// when there is no record (an unfunded collector, or a record that failed to
// persist), so a reversal refunds exactly what was charged and never invents a
// fee. The stored Amount is the exact integer the charge moved.
func (r *withdrawalReconciler) withdrawalFeePaid(chainID int64, txHash string) *big.Int {
	if r.fees == nil {
		return big.NewInt(0)
	}
	var row models.FeeRecordModel
	err := r.db.Where("type = ? AND chain_id = ? AND tx_hash = ?", "withdraw", chainID, txHash).
		Order("created_at desc").First(&row).Error
	if err != nil {
		return big.NewInt(0)
	}
	fee, ok := new(big.Int).SetString(row.Amount, 10)
	if !ok || fee.Sign() <= 0 {
		return big.NewInt(0)
	}
	return fee
}

// sweep reconciles every pending withdrawal row once. A receipt that is not yet
// mined leaves the row untouched; an RPC error is logged and retried on the next
// sweep rather than marking the withdrawal failed.
func (r *withdrawalReconciler) sweep(ctx context.Context) {
	var pending []models.Transaction
	if err := r.db.WithContext(ctx).
		Where("type = ? AND status = ?", "withdraw", "pending").
		Order("created_at asc").
		Limit(200).
		Find(&pending).Error; err != nil {
		log.Printf("withdrawal reconciler: list pending: %v", err)
		return
	}

	for i := range pending {
		select {
		case <-ctx.Done():
			return
		default:
		}
		if err := r.reconcile(ctx, &pending[i]); err != nil {
			log.Printf("withdrawal reconciler: tx %s: %v", pending[i].Hash, err)
		}
	}
}

// reconcile checks one withdrawal's receipt and applies the terminal state.
func (r *withdrawalReconciler) reconcile(ctx context.Context, tx *models.Transaction) error {
	data, err := r.blockchain.GetTransactionReceipt(ctx, tx.ChainID, tx.Hash)
	if err != nil {
		// A receipt that does not exist yet means the transaction is still in the
		// mempool; leave it pending. Any other error is transient and retried.
		if errors.Is(err, ethereum.NotFound) {
			return nil
		}
		return err
	}

	switch data.Status {
	case "confirmed":
		return r.markConfirmed(ctx, tx, data.BlockNumber)
	case "failed":
		return r.reverse(ctx, tx)
	default:
		return nil
	}
}

// markConfirmed flips a pending row to confirmed. The update is guarded by the
// current status so a row another writer already settled is not overwritten.
func (r *withdrawalReconciler) markConfirmed(ctx context.Context, tx *models.Transaction, blockNumber uint64) error {
	return r.db.WithContext(ctx).Transaction(func(db *gorm.DB) error {
		var row models.Transaction
		if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&row, "id = ?", tx.ID).Error; err != nil {
			return err
		}
		if row.Status != "pending" {
			return nil
		}
		if err := db.Model(&models.Transaction{}).Where("id = ?", row.ID).
			Updates(map[string]interface{}{
				"status":       "confirmed",
				"block_number": blockNumber,
			}).Error; err != nil {
			return err
		}
		if r.webhooks != nil {
			return r.webhooks.publishWithin(db, row.UserID.String(), WebhookEventWithdrawalSent, map[string]interface{}{
				"chain_id":     row.ChainID,
				"tx_hash":      row.Hash,
				"token":        row.FromToken,
				"amount":       row.FromAmount,
				"to_address":   row.ToAddress,
				"block_number": blockNumber,
			})
		}
		return nil
	})
}

// reverse refunds the debited amount and the fee, and marks the row failed. The
// row lock plus the status re-check make the reversal exactly-once even if two
// replicas reconcile the same withdrawal concurrently.
func (r *withdrawalReconciler) reverse(ctx context.Context, tx *models.Transaction) error {
	amount, ok := new(big.Int).SetString(tx.FromAmount, 10)
	if !ok || amount.Sign() <= 0 {
		return errors.New("withdrawal row has an unparseable amount; refusing to reverse")
	}
	// The withdrawal charged the fee to the platform fee account in the same
	// transaction as the amount. A reverted withdrawal refunds both, so the user
	// is made whole and the fee account does not keep a fee for a transfer that
	// never settled. The fee is read from the persisted fee record, so it is
	// exactly what was charged.
	fee := r.withdrawalFeePaid(tx.ChainID, tx.Hash)
	total := new(big.Int).Add(amount, fee)

	return r.db.WithContext(ctx).Transaction(func(db *gorm.DB) error {
		var row models.Transaction
		if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&row, "id = ?", tx.ID).Error; err != nil {
			return err
		}
		if row.Status != "pending" {
			// Already settled by another writer: do not reverse twice.
			return nil
		}
		if err := r.ledger.refundWithFeeWithin(db, row.UserID.String(), row.ChainID, row.FromToken, amount, fee); err != nil {
			return err
		}
		if err := db.Model(&models.Transaction{}).Where("id = ?", row.ID).
			Update("status", "failed").Error; err != nil {
			return err
		}
		if fee.Sign() > 0 && r.fees != nil {
			// Mark the record refunded so the revenue report does not count a fee
			// that was returned, and a later sweep cannot refund it twice.
			if err := db.Model(&models.FeeRecordModel{}).
				Where("type = ? AND chain_id = ? AND tx_hash = ? AND status = ?", "withdraw", row.ChainID, row.Hash, "collected").
				Update("status", "refunded").Error; err != nil {
				return err
			}
		}
		if r.webhooks != nil {
			return r.webhooks.publishWithin(db, row.UserID.String(), WebhookEventWithdrawalFailed, map[string]interface{}{
				"chain_id":     row.ChainID,
				"tx_hash":      row.Hash,
				"token":        row.FromToken,
				"amount":       row.FromAmount,
				"fee_refunded": fee.String(),
				"total_refund": total.String(),
				"to_address":   row.ToAddress,
				"reason":       "transaction reverted on-chain",
			})
		}
		return nil
	})
}
