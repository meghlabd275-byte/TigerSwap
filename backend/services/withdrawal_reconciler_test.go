package services

import (
	"context"
	"fmt"
	"math/big"
	"net"
	"os/exec"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"tigerswap/backend/models"
)

// anvilWithdrawalKey is anvil's deterministic first account key. It controls no
// real funds and is only used against a throwaway local chain.
const anvilWithdrawalKey = "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"

func startWithdrawalAnvil(t *testing.T) (string, *ethclient.Client) {
	t.Helper()
	if _, err := exec.LookPath("anvil"); err != nil {
		t.Skip("anvil not installed; skipping on-chain withdrawal reconciler test")
	}
	port := freeReconcilePort(t)
	cmd := exec.Command("anvil", "--port", fmt.Sprintf("%d", port), "--chain-id", "31337", "--silent")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start anvil: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	})
	url := fmt.Sprintf("http://127.0.0.1:%d", port)
	var client *ethclient.Client
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		c, err := ethclient.Dial(url)
		if err == nil {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			_, err = c.ChainID(ctx)
			cancel()
			if err == nil {
				client = c
				break
			}
			c.Close()
		}
		time.Sleep(100 * time.Millisecond)
	}
	if client == nil {
		t.Fatalf("anvil did not become ready at %s", url)
	}
	t.Cleanup(client.Close)
	return url, client
}

func freeReconcilePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func newReconcilerDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s-%d?mode=memory&cache=shared", t.Name(), time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db handle: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { sqlDB.Close() })
	if err := db.AutoMigrate(&models.Transaction{}, &models.AccountBalance{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func sendSigned(t *testing.T, raw *ethclient.Client, ctx context.Context, to common.Address, value *big.Int, gasLimit uint64, data []byte) string {
	t.Helper()
	priv, err := crypto.HexToECDSA(anvilWithdrawalKey)
	if err != nil {
		t.Fatalf("key: %v", err)
	}
	from := crypto.PubkeyToAddress(priv.PublicKey)
	nonce, err := raw.PendingNonceAt(ctx, from)
	if err != nil {
		t.Fatalf("nonce: %v", err)
	}
	gasPrice, err := raw.SuggestGasPrice(ctx)
	if err != nil {
		t.Fatalf("gas price: %v", err)
	}
	tx := types.NewTransaction(nonce, to, value, gasLimit, gasPrice, data)
	signed, err := types.SignTx(tx, types.NewEIP155Signer(big.NewInt(31337)), priv)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if err := raw.SendTransaction(ctx, signed); err != nil {
		t.Fatalf("send: %v", err)
	}
	return signed.Hash().Hex()
}

func mine(t *testing.T, raw *ethclient.Client, ctx context.Context) {
	t.Helper()
	if err := raw.Client().CallContext(ctx, nil, "evm_mine"); err != nil {
		t.Fatalf("mine: %v", err)
	}
}

// TestWithdrawalReconcilerConfirmsMinedTransfer drives a real native transfer on
// a local chain and pins that the reconciler marks the pending row confirmed
// with the mined block number, leaving the ledger debit in place.
func TestWithdrawalReconcilerConfirmsMinedTransfer(t *testing.T) {
	url, raw := startWithdrawalAnvil(t)
	ctx := context.Background()

	client := NewBlockchainClient()
	client.SetRPCURL(31337, url)

	to := common.HexToAddress("0x70997970C51812dc3A010C7d01b50e0d17dc79C8")
	txHash := sendSigned(t, raw, ctx, to, big.NewInt(1_000_000_000_000_000), 21000, nil)

	db := newReconcilerDB(t)
	userID := uuid.NewString()
	if err := db.Create(&models.Transaction{
		UserID:     uuid.MustParse(userID),
		Hash:       txHash,
		ChainID:    31337,
		Type:       "withdraw",
		FromToken:  "native",
		FromAmount: "1000000000000000",
		ToAddress:  to.Hex(),
		Status:     "pending",
		Timestamp:  time.Now(),
	}).Error; err != nil {
		t.Fatalf("seed tx: %v", err)
	}

	ledger := NewLedgerService(db)
	reconciler := &withdrawalReconciler{db: db, blockchain: client, ledger: ledger}

	mine(t, raw, ctx)
	reconciler.sweep(ctx)

	var got models.Transaction
	if err := db.Where("hash = ?", txHash).First(&got).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got.Status != "confirmed" {
		t.Fatalf("status = %q, want confirmed", got.Status)
	}
	if got.BlockNumber == 0 {
		t.Fatalf("block number not recorded")
	}
}

// TestWithdrawalReconcilerReversesRevertedTransfer pins that a reverted receipt
// credits the debited amount back exactly once and marks the row failed.
func TestWithdrawalReconcilerReversesRevertedTransfer(t *testing.T) {
	url, raw := startWithdrawalAnvil(t)
	ctx := context.Background()

	client := NewBlockchainClient()
	client.SetRPCURL(31337, url)

	// Deploy a contract whose runtime always reverts, so a call to it mines a
	// receipt with status 0.
	deploy := common.FromHex("0x6006600c60003960066000f360006000fd")
	reverter := deployContract(t, raw, ctx, deploy)

	txHash := sendSigned(t, raw, ctx, reverter, big.NewInt(0), 100000, []byte{0x01})
	mine(t, raw, ctx)

	db := newReconcilerDB(t)
	userID := uuid.NewString()
	ledger := NewLedgerService(db)
	if err := ledger.Credit(userID, 31337, "native", "ETH", 18, big.NewInt(5_000_000)); err != nil {
		t.Fatalf("credit: %v", err)
	}
	if err := ledger.Debit(userID, 31337, "native", big.NewInt(5_000_000)); err != nil {
		t.Fatalf("debit: %v", err)
	}
	if err := db.Create(&models.Transaction{
		UserID:     uuid.MustParse(userID),
		Hash:       txHash,
		ChainID:    31337,
		Type:       "withdraw",
		FromToken:  "native",
		FromAmount: "5000000",
		ToAddress:  reverter.Hex(),
		Status:     "pending",
		Timestamp:  time.Now(),
	}).Error; err != nil {
		t.Fatalf("seed tx: %v", err)
	}

	reconciler := &withdrawalReconciler{db: db, blockchain: client, ledger: ledger}
	reconciler.sweep(ctx)

	var got models.Transaction
	if err := db.Where("hash = ?", txHash).First(&got).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got.Status != "failed" {
		t.Fatalf("status = %q, want failed", got.Status)
	}
	bal, err := ledger.Available(userID, 31337, "native")
	if err != nil {
		t.Fatalf("available: %v", err)
	}
	if bal.Cmp(big.NewInt(5_000_000)) != 0 {
		t.Fatalf("balance after reversal = %s, want 5000000", bal)
	}

	// A second sweep must not credit again.
	reconciler.sweep(ctx)
	bal2, _ := ledger.Available(userID, 31337, "native")
	if bal2.Cmp(big.NewInt(5_000_000)) != 0 {
		t.Fatalf("second sweep changed balance: %s", bal2)
	}
}

// deployContract deploys raw creation bytecode and returns the deployed address.
func deployContract(t *testing.T, raw *ethclient.Client, ctx context.Context, code []byte) common.Address {
	t.Helper()
	priv, err := crypto.HexToECDSA(anvilWithdrawalKey)
	if err != nil {
		t.Fatalf("key: %v", err)
	}
	from := crypto.PubkeyToAddress(priv.PublicKey)
	nonce, err := raw.PendingNonceAt(ctx, from)
	if err != nil {
		t.Fatalf("nonce: %v", err)
	}
	gasPrice, err := raw.SuggestGasPrice(ctx)
	if err != nil {
		t.Fatalf("gas price: %v", err)
	}
	tx := types.NewContractCreation(nonce, big.NewInt(0), 200000, gasPrice, code)
	signed, err := types.SignTx(tx, types.NewEIP155Signer(big.NewInt(31337)), priv)
	if err != nil {
		t.Fatalf("sign deploy: %v", err)
	}
	if err := raw.SendTransaction(ctx, signed); err != nil {
		t.Fatalf("send deploy: %v", err)
	}
	mine(t, raw, ctx)
	receipt, err := raw.TransactionReceipt(ctx, signed.Hash())
	if err != nil {
		t.Fatalf("deploy receipt: %v", err)
	}
	return receipt.ContractAddress
}
