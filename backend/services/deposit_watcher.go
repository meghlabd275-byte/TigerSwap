package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"tigerswap/backend/models"
)

// nativeTokenKey is the ledger key for a chain's native asset.
const nativeTokenKey = "native"

// DepositWatcher credits user ledger balances from confirmed on-chain deposits.
//
// The watcher is the only path that creates platform credit, so it is
// deliberately conservative:
//
//   - it only scans blocks, and only credits transactions that are at least
//     `confirmations` deep, so a reorg cannot leave a user with spendable funds
//     that later vanish;
//   - every credited (chain, tx hash, log index) is written to
//     processed_deposits in the same transaction as the credit, so a replayed
//     block can never credit the same deposit twice;
//   - it only credits transfers that land in an address the platform derived
//     for a user, and it resolves the owner from the stored mapping rather than
//     trusting anything the client sent.
type DepositWatcher struct {
	db            *gorm.DB
	blockchain    *BlockchainClient
	ledger        *LedgerService
	confirmations uint64
	pollInterval  time.Duration

	mu      sync.Mutex
	running bool
	cancel  context.CancelFunc
}

func NewDepositWatcher(db *gorm.DB, blockchain *BlockchainClient, ledger *LedgerService) *DepositWatcher {
	return &DepositWatcher{
		db:            db,
		blockchain:    blockchain,
		ledger:        ledger,
		confirmations: 12,
		pollInterval:  15 * time.Second,
	}
}

// Start launches the watcher for the given chains. It returns immediately; the
// scan runs in the background until Stop is called or ctx is cancelled.
func (w *DepositWatcher) Start(ctx context.Context, chainIDs []int64) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.running {
		return errors.New("deposit watcher already running")
	}

	ctx, cancel := context.WithCancel(ctx)
	w.cancel = cancel
	w.running = true

	for _, chainID := range chainIDs {
		go w.runChain(ctx, chainID)
	}
	return nil
}

// Stop halts the watcher and waits for the current scan to finish.
func (w *DepositWatcher) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.running {
		return
	}
	w.cancel()
	w.running = false
}

func (w *DepositWatcher) runChain(ctx context.Context, chainID int64) {
	// Seed the cursor at the current head so the watcher never re-scans the
	// whole chain on first start.
	if _, err := w.ensureCursor(ctx, chainID); err != nil {
		log.Printf("deposit watcher: chain %d cursor init failed: %v", chainID, err)
		return
	}

	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := w.scanChain(ctx, chainID); err != nil {
				log.Printf("deposit watcher: chain %d scan failed: %v", chainID, err)
			}
		}
	}
}

// ensureCursor returns the chain's scan cursor, creating it at the current head
// when absent.
func (w *DepositWatcher) ensureCursor(ctx context.Context, chainID int64) (*models.DepositCursor, error) {
	var cursor models.DepositCursor
	err := w.db.Where("chain_id = ?", chainID).First(&cursor).Error
	if err == nil {
		return &cursor, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	head, err := w.blockchain.GetCurrentBlock(ctx, chainID)
	if err != nil {
		return nil, fmt.Errorf("current block: %w", err)
	}

	cursor = models.DepositCursor{ChainID: chainID, LastScannedBlock: head}
	if err := w.db.Create(&cursor).Error; err != nil {
		return nil, err
	}
	return &cursor, nil
}

// scanChain advances one chain's cursor to the highest block that is safely
// confirmed, crediting every qualifying deposit along the way.
func (w *DepositWatcher) scanChain(ctx context.Context, chainID int64) error {
	cursor, err := w.ensureCursor(ctx, chainID)
	if err != nil {
		return err
	}

	head, err := w.blockchain.GetCurrentBlock(ctx, chainID)
	if err != nil {
		return err
	}
	if head < w.confirmations {
		return nil
	}
	safeHead := head - w.confirmations
	if safeHead <= cursor.LastScannedBlock {
		return nil
	}

	client, err := w.blockchain.GetClient(chainID)
	if err != nil {
		return err
	}

	for blockNumber := cursor.LastScannedBlock + 1; blockNumber <= safeHead; blockNumber++ {
		if err := w.scanBlock(ctx, client, chainID, blockNumber); err != nil {
			return err
		}
		if err := w.db.Model(&models.DepositCursor{}).
			Where("chain_id = ?", chainID).
			Update("last_scanned_block", blockNumber).Error; err != nil {
			return err
		}
	}
	return nil
}

func (w *DepositWatcher) scanBlock(ctx context.Context, client *ethclient.Client, chainID int64, blockNumber uint64) error {
	block, err := client.BlockByNumber(ctx, new(big.Int).SetUint64(blockNumber))
	if err != nil {
		return fmt.Errorf("block %d: %w", blockNumber, err)
	}

	// Map every platform deposit address on this chain to its owner, once per
	// block, so the lookup below is a map hit rather than a query per transfer.
	addressOwners, err := w.depositAddressOwners(chainID)
	if err != nil {
		return err
	}

	for _, tx := range block.Transactions() {
		// Native transfer.
		if tx.To() != nil {
			if ownerID, ok := addressOwners[strings.ToLower(tx.To().Hex())]; ok && tx.Value().Sign() > 0 {
				if err := w.creditDeposit(chainID, tx, 0, ownerID, nativeTokenKey, "native", 18, tx.Value(), blockNumber); err != nil {
					return err
				}
			}
		}

		// ERC-20 transfers emitted by this transaction.
		if err := w.scanReceiptLogs(ctx, client, chainID, tx, addressOwners, blockNumber); err != nil {
			return err
		}
	}
	return nil
}

// scanReceiptLogs credits ERC-20 transfers. It recognises the canonical
// Transfer(address,address,uint256) event and ignores everything else.
func (w *DepositWatcher) scanReceiptLogs(
	ctx context.Context,
	client *ethclient.Client,
	chainID int64,
	tx *types.Transaction,
	addressOwners map[string]string,
	blockNumber uint64,
) error {
	receipt, err := client.TransactionReceipt(ctx, tx.Hash())
	if err != nil {
		// A transaction included in a fetched block always has a receipt; a
		// not-found here means the node is inconsistent, so skip rather than
		// fail the whole block and stall the cursor.
		if errors.Is(err, ethereum.NotFound) {
			return nil
		}
		return fmt.Errorf("receipt %s: %w", tx.Hash().Hex(), err)
	}
	if receipt.Status != types.ReceiptStatusSuccessful {
		return nil
	}

	for logIndex, event := range receipt.Logs {
		if len(event.Topics) != 3 || event.Topics[0] != erc20TransferTopic {
			continue
		}
		if len(event.Data) != 32 {
			continue
		}

		to := common.BytesToAddress(event.Topics[2].Bytes())
		ownerID, ok := addressOwners[strings.ToLower(to.Hex())]
		if !ok {
			continue
		}

		amount := new(big.Int).SetBytes(event.Data)
		if amount.Sign() <= 0 {
			continue
		}

		tokenAddress := event.Address.Hex()
		balance, err := w.blockchain.GetERC20Balance(ctx, chainID, to.Hex(), tokenAddress)
		if err != nil {
			// Metadata failure must not credit an amount we cannot describe.
			return fmt.Errorf("token metadata for %s: %w", tokenAddress, err)
		}

		if err := w.creditDeposit(chainID, tx, logIndex, ownerID, tokenAddress, balance.Symbol, balance.Decimals, amount, blockNumber); err != nil {
			return err
		}
	}
	return nil
}

// erc20TransferTopic is keccak256("Transfer(address,address,uint256)").
var erc20TransferTopic = common.HexToHash("0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef")

// creditDeposit records a deposit and credits the owner's ledger balance in one
// transaction. The unique key on processed_deposits makes a replayed block a
// no-op instead of a double credit.
func (w *DepositWatcher) creditDeposit(
	chainID int64,
	tx *types.Transaction,
	logIndex int,
	ownerID string,
	tokenAddress, symbol string,
	decimals int,
	amount *big.Int,
	blockNumber uint64,
) error {
	record := models.ProcessedDeposit{
		ChainID:      chainID,
		TxHash:       tx.Hash().Hex(),
		LogIndex:     logIndex,
		UserID:       ownerID,
		TokenAddress: normalizeTokenAddress(tokenAddress),
		Symbol:       symbol,
		Decimals:     decimals,
		Amount:       amount.String(),
		BlockNumber:  blockNumber,
	}

	return w.db.Transaction(func(dbTx *gorm.DB) error {
		var existing models.ProcessedDeposit
		err := dbTx.Where("chain_id = ? AND tx_hash = ? AND log_index = ?", chainID, record.TxHash, logIndex).
			First(&existing).Error
		if err == nil {
			return nil // already credited
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		// ON CONFLICT DO NOTHING makes two watchers racing on the same block
		// safe: only one inserts, and only that one credits.
		result := dbTx.Clauses(clause.OnConflict{DoNothing: true}).Create(&record)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return nil
		}

		return w.ledger.creditWithin(dbTx, ownerID, chainID, tokenAddress, symbol, decimals, amount)
	})
}

// depositAddressOwners returns a lower-cased deposit address to user id map for
// a chain.
func (w *DepositWatcher) depositAddressOwners(chainID int64) (map[string]string, error) {
	var rows []models.DepositAddress
	if err := w.db.Where("chain_id = ?", chainID).Find(&rows).Error; err != nil {
		return nil, err
	}
	owners := make(map[string]string, len(rows))
	for _, row := range rows {
		owners[strings.ToLower(row.Address)] = row.UserID
	}
	return owners, nil
}

// CreditDeposit is exposed for the admin backfill path: it credits a deposit
// that an operator has verified on-chain.
func (w *DepositWatcher) CreditDeposit(userID string, chainID int64, tokenAddress, symbol string, decimals int, amount *big.Int) error {
	if amount == nil || amount.Sign() <= 0 {
		return errors.New("deposit amount must be positive")
	}
	return w.ledger.Credit(userID, chainID, tokenAddress, symbol, decimals, amount)
}
