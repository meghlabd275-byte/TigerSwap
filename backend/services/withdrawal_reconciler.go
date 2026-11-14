package services

import (
	"context"
	"errors"
	"log"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"tigerswap/backend/models"
)

// DefaultWithdrawalReconcileInterval is how often pending withdrawals are
// checked against their on-chain receipt.
const DefaultWithdrawalReconcileInterval = 30 * time.Second

// unbroadcastedWithdrawalGrace is how long a reserved withdrawal may sit without
// a hash before the reconciler warns. The hash is written synchronously once the
// node accepts the transaction, so a longer gap means the release was abandoned
// (or the process died) and an operator must decide whether it was broadcast.
const unbroadcastedWithdrawalGrace = 15 * time.Minute

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
		// A pending withdrawal with no hash was reserved but its release was never
		// recorded. The common cause is a crash between the reserve commit and the
		// broadcast returning; the hash write happens synchronously under the nonce
		// lock, so the window is a single database update. Such a row is deliberately
		// not auto-refunded: if the node accepted the transaction before the crash the
		// funds are already on their way, and refunding would pay the user twice.
		// Instead it is surfaced for an operator once it is older than the grace
		// window, so it cannot sit silently.
		if pending[i].Hash == "" {
			if time.Since(pending[i].CreatedAt) > unbroadcastedWithdrawalGrace {
				log.Printf("withdrawal reconciler: withdrawal %s has no hash after %s; an operator must confirm whether it was broadcast before refunding", pending[i].ID, time.Since(pending[i].CreatedAt).Round(time.Minute))
			}
			continue
		}
		if err := r.reconcile(ctx, &pending[i]); err != nil {
			log.Printf("withdrawal reconciler: tx %s: %v", pending[i].Hash, err)
		}
	}

	r.sweepRequests(ctx)
}

// sweepRequests reconciles queued withdrawal requests whose release transaction
// was broadcast but never checked. ApproveRequest moves a request to "processing"
// and records the tx hash, but a reverted release would otherwise leave the user
// debited with a request stuck in "processing" forever. A confirmed release is
// marked approved; a reverted one refunds the amount and fee and is marked
// failed, exactly like a direct withdrawal.
func (r *withdrawalReconciler) sweepRequests(ctx context.Context) {
	var processing []models.WithdrawalRequest
	if err := r.db.WithContext(ctx).
		Where("status = ? AND tx_hash <> ?", "processing", "").
		Order("created_at asc").
		Limit(200).
		Find(&processing).Error; err != nil {
		log.Printf("withdrawal reconciler: list processing requests: %v", err)
		return
	}

	for i := range processing {
		select {
		case <-ctx.Done():
			return
		default:
		}
		if err := r.reconcileRequest(ctx, &processing[i]); err != nil {
			log.Printf("withdrawal reconciler: request %s: %v", processing[i].ID, err)
		}
	}
}

// reconcileRequest checks one processing request's release receipt and applies the
// terminal state.
func (r *withdrawalReconciler) reconcileRequest(ctx context.Context, request *models.WithdrawalRequest) error {
	data, err := r.blockchain.GetTransactionReceipt(ctx, request.ChainID, request.TxHash)
	if err != nil {
		if errors.Is(err, ethereum.NotFound) {
			return nil
		}
		return err
	}

	switch data.Status {
	case "confirmed":
		return r.db.WithContext(ctx).Model(&models.WithdrawalRequest{}).
			Where("id = ? AND status = ?", request.ID, "processing").
			Update("status", models.WithdrawalStatusApproved).Error
	case "failed":
		return r.reverseRequest(ctx, request)
	default:
		return nil
	}
}

// reverseRequest refunds a request whose release reverted and marks it failed. The
// row lock and the status re-check make the refund exactly-once across replicas.
func (r *withdrawalReconciler) reverseRequest(ctx context.Context, request *models.WithdrawalRequest) error {
	amount, ok := new(big.Int).SetString(request.Amount, 10)
	if !ok || amount.Sign() <= 0 {
		return errors.New("withdrawal request has an unparseable amount; refusing to reverse")
	}
	fee := big.NewInt(0)
	if strings.TrimSpace(request.Fee) != "" {
		parsed, ok := new(big.Int).SetString(request.Fee, 10)
		if !ok || parsed.Sign() < 0 {
			return errors.New("withdrawal request has an unparseable fee; refusing to reverse")
		}
		fee = parsed
	}
	total := new(big.Int).Add(amount, fee)

	return r.db.WithContext(ctx).Transaction(func(db *gorm.DB) error {
		var row models.WithdrawalRequest
		if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&row, "id = ?", request.ID).Error; err != nil {
			return err
		}
		if row.Status != "processing" {
			return nil
		}
		if err := r.ledger.refundWithFeeWithin(db, row.UserID, row.ChainID, row.TokenAddress, amount, fee); err != nil {
			return err
		}
		if err := db.Model(&models.WithdrawalRequest{}).Where("id = ?", row.ID).
			Updates(map[string]interface{}{
				"status": "failed",
				"reason": "release transaction reverted on-chain",
			}).Error; err != nil {
			return err
		}
		if r.webhooks != nil {
			return r.webhooks.publishWithin(db, row.UserID, WebhookEventWithdrawalFailed, map[string]interface{}{
				"chain_id":     row.ChainID,
				"tx_hash":      row.TxHash,
				"token":        row.TokenAddress,
				"amount":       row.Amount,
				"fee_refunded": fee.String(),
				"total_refund": total.String(),
				"to_address":   row.ToAddress,
				"reason":       "release transaction reverted on-chain",
			})
		}
		return nil
	})
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
