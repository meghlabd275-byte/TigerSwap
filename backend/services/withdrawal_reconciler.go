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
func StartWithdrawalReconciler(ctx context.Context, db *gorm.DB, blockchain *BlockchainClient, ledger *LedgerService, webhooks *WebhookService, interval time.Duration) {
	if db == nil || blockchain == nil || ledger == nil {
		log.Printf("withdrawal reconciler disabled: db, blockchain or ledger not configured")
		return
	}
	if interval <= 0 {
		interval = DefaultWithdrawalReconcileInterval
	}
	reconciler := &withdrawalReconciler{db: db, blockchain: blockchain, ledger: ledger, webhooks: webhooks}
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

// reverse credits the debited amount back and marks the row failed. The row lock
// plus the status re-check make the reversal exactly-once even if two replicas
// reconcile the same withdrawal concurrently.
func (r *withdrawalReconciler) reverse(ctx context.Context, tx *models.Transaction) error {
	amount, ok := new(big.Int).SetString(tx.FromAmount, 10)
	if !ok || amount.Sign() <= 0 {
		return errors.New("withdrawal row has an unparseable amount; refusing to reverse")
	}

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
		if err := r.ledger.creditWithin(db, row.UserID.String(), row.ChainID, row.FromToken, "", 0, amount); err != nil {
			return err
		}
		if err := db.Model(&models.Transaction{}).Where("id = ?", row.ID).
			Update("status", "failed").Error; err != nil {
			return err
		}
		if r.webhooks != nil {
			return r.webhooks.publishWithin(db, row.UserID.String(), WebhookEventWithdrawalFailed, map[string]interface{}{
				"chain_id":   row.ChainID,
				"tx_hash":    row.Hash,
				"token":      row.FromToken,
				"amount":     row.FromAmount,
				"to_address": row.ToAddress,
				"reason":     "transaction reverted on-chain",
			})
		}
		return nil
	})
}
