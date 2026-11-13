package perps

import (
	"context"
	"encoding/hex"
	"math/big"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// anvilKey is anvil's deterministic first account key. It is a well-known test
// key that controls no real funds.
const anvilKey = "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"

// startAnvil launches anvil on a free port and returns its RPC URL. The test is
// skipped when anvil is not installed, so the suite stays green without
// Foundry; CI installs it, so the on-chain path is exercised there. Each caller
// gets its own instance so tests can run in parallel without sharing state.
func startAnvil(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("anvil"); err != nil {
		t.Skip("anvil not installed; skipping on-chain perps integration test")
	}
	port := freePort(t)
	cmd := exec.Command("anvil", "--silent", "--port", port)
	if err := cmd.Start(); err != nil {
		t.Fatalf("start anvil: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	})
	url := "http://127.0.0.1:" + port
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		client, err := ethclient.Dial(url)
		if err == nil {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			_, err = client.ChainID(ctx)
			cancel()
			client.Close()
			if err == nil {
				return url
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatal("anvil did not become ready")
	return ""
}

// freePort asks the OS for an unused TCP port and returns it as a string.
func freePort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("find free port: %v", err)
	}
	defer l.Close()
	return strconv.Itoa(l.Addr().(*net.TCPAddr).Port)
}

// deploy deploys a contract from compiled bytecode with ABI-encoded constructor
// arguments and returns its address, waiting for the receipt so the address is
// live before the test uses it.
func deploy(t *testing.T, client *ethclient.Client, chainID int64, key string, artifact string, ctorArgs []byte) common.Address {
	t.Helper()
	bytecodeHex, err := os.ReadFile("testdata/" + artifact + ".bin")
	if err != nil {
		t.Fatalf("read bytecode: %v", err)
	}
	raw := strings.TrimSpace(string(bytecodeHex))
	raw = strings.TrimPrefix(raw, "0x")
	code, err := hex.DecodeString(raw)
	if err != nil {
		t.Fatalf("decode bytecode: %v", err)
	}
	data := append(code, ctorArgs...)

	priv, err := crypto.HexToECDSA(key)
	if err != nil {
		t.Fatalf("parse key: %v", err)
	}
	from := crypto.PubkeyToAddress(priv.PublicKey)

	ctx := context.Background()
	nonce, err := client.PendingNonceAt(ctx, from)
	if err != nil {
		t.Fatalf("nonce: %v", err)
	}
	gasPrice, err := client.SuggestGasPrice(ctx)
	if err != nil {
		t.Fatalf("gas price: %v", err)
	}
	tx := types.NewContractCreation(nonce, big.NewInt(0), 6_000_000, gasPrice, data)
	signed, err := types.SignTx(tx, types.LatestSignerForChainID(big.NewInt(chainID)), priv)
	if err != nil {
		t.Fatalf("sign deploy: %v", err)
	}
	if err := client.SendTransaction(ctx, signed); err != nil {
		t.Fatalf("send deploy: %v", err)
	}
	receipt := mustReceipt(t, client, signed.Hash())
	return receipt.ContractAddress
}

// mustReceipt waits for a receipt and fails the test on revert.
func mustReceipt(t *testing.T, client *ethclient.Client, hash common.Hash) *types.Receipt {
	t.Helper()
	ctx := context.Background()
	for i := 0; i < 100; i++ {
		receipt, err := client.TransactionReceipt(ctx, hash)
		if err == nil && receipt != nil {
			if receipt.Status != types.ReceiptStatusSuccessful {
				t.Fatalf("tx %s reverted", hash.Hex())
			}
			return receipt
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("no receipt for %s", hash.Hex())
	return nil
}

// sendRaw signs and sends a plain transaction to a contract.
func sendRaw(t *testing.T, client *ethclient.Client, chainID int64, key string, to common.Address, data []byte) *types.Receipt {
	t.Helper()
	priv, err := crypto.HexToECDSA(key)
	if err != nil {
		t.Fatalf("parse key: %v", err)
	}
	from := crypto.PubkeyToAddress(priv.PublicKey)
	ctx := context.Background()
	nonce, err := client.PendingNonceAt(ctx, from)
	if err != nil {
		t.Fatalf("nonce: %v", err)
	}
	gasPrice, err := client.SuggestGasPrice(ctx)
	if err != nil {
		t.Fatalf("gas price: %v", err)
	}
	gas, err := client.EstimateGas(ctx, ethereum.CallMsg{From: from, To: &to, Data: data})
	if err != nil {
		t.Fatalf("estimate gas: %v", err)
	}
	tx := types.NewTransaction(nonce, to, big.NewInt(0), gas*2, gasPrice, data)
	signed, err := types.SignTx(tx, types.LatestSignerForChainID(big.NewInt(chainID)), priv)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if err := client.SendTransaction(ctx, signed); err != nil {
		t.Fatalf("send: %v", err)
	}
	return mustReceipt(t, client, signed.Hash())
}

func loadABI(t *testing.T, artifact string) abi.ABI {
	t.Helper()
	raw, err := os.ReadFile("testdata/" + artifact + ".abi.json")
	if err != nil {
		t.Fatalf("read abi: %v", err)
	}
	parsed, err := abi.JSON(strings.NewReader(string(raw)))
	if err != nil {
		t.Fatalf("parse abi: %v", err)
	}
	return parsed
}

func wad(n int64) *big.Int {
	return new(big.Int).Mul(big.NewInt(n), OneUnit)
}

// bps returns n basis points as a WAD-scaled ratio (n/10000 * 1e18).
func bps(n int64) *big.Int {
	out := new(big.Int).Mul(big.NewInt(n), OneUnit)
	return out.Div(out, big.NewInt(10_000))
}

// TestOnChainPerpsLifecycle deploys the real TigerSwapPerpetuals contract to a
// local anvil chain and drives a full position lifecycle through the client:
// create a market, seed a price, add LP liquidity, open a position with a signed
// transaction, read it back from chain, close it, and confirm it is gone. This
// proves the on-chain settlement path end to end, not just that the ABI parses.
func TestOnChainPerpsLifecycle(t *testing.T) {
	url := startAnvil(t)
	chainID := int64(31337)

	priv, err := crypto.HexToECDSA(anvilKey)
	if err != nil {
		t.Fatalf("key: %v", err)
	}
	owner := crypto.PubkeyToAddress(priv.PublicKey)

	erc20ABI := loadABI(t, "TestERC20")
	perpsABIForDeploy := loadABI(t, "TigerSwapPerpetuals")

	raw, err := ethclient.Dial(url)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer raw.Close()

	// Deploy the settlement token and the perps engine. The token's constructor
	// takes (name, symbol); the ABI carries no constructor entry, so pack two
	// strings directly.
	ctor, err := abiStrings(t, "RUSD", "RUSD")
	if err != nil {
		t.Fatalf("pack erc20 ctor: %v", err)
	}
	rusd := deploy(t, raw, chainID, anvilKey, "TestERC20", ctor)
	perpsAddr := deploy(t, raw, chainID, anvilKey, "TigerSwapPerpetuals", nil)

	// Configure the market the way the deploy script does.
	marketID := MarketID("BTC/RUSD")
	createMarket, err := perpsABIForDeploy.Pack("createMarket", marketID, rusd, wad(10), bps(500), bps(500), bps(1000))
	if err != nil {
		t.Fatalf("pack createMarket: %v", err)
	}
	sendRaw(t, raw, chainID, anvilKey, perpsAddr, createMarket)

	// Point the oracle at the deployer and seed the mark price.
	setOracle, err := perpsABIForDeploy.Pack("setOracle", owner)
	if err != nil {
		t.Fatalf("pack setOracle: %v", err)
	}
	sendRaw(t, raw, chainID, anvilKey, perpsAddr, setOracle)

	updatePrice, err := perpsABIForDeploy.Pack("updatePrice", marketID, wad(60000))
	if err != nil {
		t.Fatalf("pack updatePrice: %v", err)
	}
	sendRaw(t, raw, chainID, anvilKey, perpsAddr, updatePrice)

	// Fund the signer and seed pool liquidity so the pool can back the trade.
	mint, err := erc20ABI.Pack("mint", owner, wad(10_000_000))
	if err != nil {
		t.Fatalf("pack mint: %v", err)
	}
	sendRaw(t, raw, chainID, anvilKey, rusd, mint)

	approve, err := erc20ABI.Pack("approve", perpsAddr, new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1)))
	if err != nil {
		t.Fatalf("pack approve: %v", err)
	}
	sendRaw(t, raw, chainID, anvilKey, rusd, approve)

	addLiq, err := perpsABIForDeploy.Pack("addLiquidity", rusd, wad(5_000_000))
	if err != nil {
		t.Fatalf("pack addLiquidity: %v", err)
	}
	sendRaw(t, raw, chainID, anvilKey, perpsAddr, addLiq)

	// Now drive the client under test.
	client, err := NewClient(Deployment{ChainID: chainID, Perpetuals: perpsAddr, RPCURL: url}, priv)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	market, err := client.Market(ctx, marketID)
	if err != nil {
		t.Fatalf("read market: %v", err)
	}
	if !market.IsActive {
		t.Fatal("market should be active")
	}
	if market.Token != rusd {
		t.Fatalf("market token mismatch: %s", market.Token.Hex())
	}

	price, err := client.MarkPrice(ctx, marketID)
	if err != nil {
		t.Fatalf("mark price: %v", err)
	}
	if price.Cmp(wad(60000)) != 0 {
		t.Fatalf("expected mark 60000, got %s", price)
	}

	// size is a WAD-scaled quantity; notional = size * price / 1e18. At a
	// $60,000 mark, size 1e18 is a $60,000 notional. 5x leverage is $12,000
	// collateral, which clears the 10% initial-margin requirement.
	size := OneUnit
	collateral := wad(12000)
	openCall, err := client.BuildOpenPosition(marketID, size, collateral, true)
	if err != nil {
		t.Fatalf("build open: %v", err)
	}
	openReceipt, err := client.Send(ctx, openCall)
	if err != nil {
		t.Fatalf("send open: %v", err)
	}
	if openReceipt.Status != types.ReceiptStatusSuccessful {
		t.Fatal("open reverted")
	}

	// Find the position id from the PositionOpened event.
	posID := positionIDFromReceipt(t, raw, openReceipt, perpsAddr)
	if posID == (common.Hash{}) {
		t.Fatal("no PositionOpened event found")
	}

	position, err := client.Position(ctx, posID)
	if err != nil {
		t.Fatalf("read position: %v", err)
	}
	if position.Size.Sign() == 0 {
		t.Fatal("position should be open")
	}
	if !position.IsLong {
		t.Fatal("position should be long")
	}
	if position.Trader != owner {
		t.Fatalf("trader mismatch: %s", position.Trader.Hex())
	}

	open, err := client.OpenPositions(ctx, owner)
	if err != nil {
		t.Fatalf("open positions: %v", err)
	}
	if len(open) != 1 || open[0] != posID {
		t.Fatalf("expected one open position %s, got %v", posID.Hex(), open)
	}

	// Close it and confirm the contract deletes it.
	closeCall, err := client.BuildClosePosition(posID)
	if err != nil {
		t.Fatalf("build close: %v", err)
	}
	if _, err := client.Send(ctx, closeCall); err != nil {
		t.Fatalf("send close: %v", err)
	}
	closed, err := client.Position(ctx, posID)
	if err != nil {
		t.Fatalf("read closed position: %v", err)
	}
	if closed.Size.Sign() != 0 {
		t.Fatalf("position should be closed, size=%s", closed.Size)
	}

	// Liquidity must remain funded (fee accrued to the pool).
	liquidity, err := client.PoolLiquidity(ctx, rusd)
	if err != nil {
		t.Fatalf("pool liquidity: %v", err)
	}
	if liquidity.Sign() <= 0 {
		t.Fatal("pool liquidity should be positive")
	}
}

// positionIDFromReceipt extracts the positionId from the PositionOpened event in
// a receipt's logs.
func positionIDFromReceipt(t *testing.T, client *ethclient.Client, receipt *types.Receipt, perpsAddr common.Address) common.Hash {
	t.Helper()
	topic := perpsABI.Events["PositionOpened"].ID
	for _, lg := range receipt.Logs {
		if lg.Address != perpsAddr || len(lg.Topics) == 0 || lg.Topics[0] != topic {
			continue
		}
		// Non-indexed fields: positionId, size, collateral, isLong.
		vals, err := perpsABI.Unpack("PositionOpened", lg.Data)
		if err != nil {
			t.Fatalf("unpack PositionOpened: %v", err)
		}
		if id, ok := vals[0].([32]byte); ok {
			return common.Hash(id)
		}
	}
	return common.Hash{}
}

func mustType(t *testing.T, name string) abi.Type {
	t.Helper()
	typ, err := abi.NewType(name, "", nil)
	if err != nil {
		t.Fatalf("abi type %s: %v", name, err)
	}
	return typ
}
