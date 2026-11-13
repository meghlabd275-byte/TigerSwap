// Package perpstest stands up a local anvil chain with the real
// TigerSwapPerpetuals contract deployed and configured, so both the perps
// client tests and the service-level tests exercise the same on-chain path
// instead of duplicating a deployment harness.
package perpstest

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"

	"tigerswap/backend/go/perps"
)

// AnvilKey is anvil's deterministic first account key. It controls no real
// funds and is only ever used against a throwaway local chain.
const AnvilKey = "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"

// Harness is a running anvil chain with the perps engine deployed and one
// BTC/RUSD market configured at a $60,000 mark, seeded with LP liquidity.
type Harness struct {
	URL      string
	ChainID  int64
	Perps    common.Address
	Token    common.Address
	MarketID common.Hash
	Owner    common.Address

	raw *ethclient.Client
	t   *testing.T
}

// Start launches anvil, deploys the contracts and configures the market. It
// skips the test when anvil is not installed, so a suite stays green without
// Foundry while CI still exercises the path.
func Start(t *testing.T) *Harness {
	t.Helper()
	if _, err := exec.LookPath("anvil"); err != nil {
		t.Skip("anvil not installed; skipping on-chain perps integration test")
	}
	url := startAnvil(t)
	chainID := int64(31337)

	priv, err := crypto.HexToECDSA(AnvilKey)
	if err != nil {
		t.Fatalf("key: %v", err)
	}
	owner := crypto.PubkeyToAddress(priv.PublicKey)

	raw, err := ethclient.Dial(url)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(raw.Close)

	perpsABI := loadABI(t, "TigerSwapPerpetuals")
	erc20ABI := loadABI(t, "TestERC20")

	ctor := packStrings(t, "RUSD", "RUSD")
	token := deploy(t, raw, chainID, "TestERC20", ctor)
	perpsAddr := deploy(t, raw, chainID, "TigerSwapPerpetuals", nil)

	marketID := perps.MarketID("BTC/RUSD")
	packAndSend(t, raw, chainID, perpsAddr, perpsABI, "createMarket", marketID, token, wad(10), bps(500), bps(500), bps(1000))
	packAndSend(t, raw, chainID, perpsAddr, perpsABI, "setOracle", owner)
	packAndSend(t, raw, chainID, perpsAddr, perpsABI, "updatePrice", marketID, wad(60000))
	packAndSend(t, raw, chainID, token, erc20ABI, "mint", owner, wad(10_000_000))

	maxUint := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))
	packAndSend(t, raw, chainID, token, erc20ABI, "approve", perpsAddr, maxUint)
	packAndSend(t, raw, chainID, perpsAddr, perpsABI, "addLiquidity", token, wad(5_000_000))

	return &Harness{
		URL:      url,
		ChainID:  chainID,
		Perps:    perpsAddr,
		Token:    token,
		MarketID: marketID,
		Owner:    owner,
		raw:      raw,
		t:        t,
	}
}

// Client returns a read/write client bound to the harness signer.
func (h *Harness) Client() *perps.Client {
	h.t.Helper()
	priv, err := crypto.HexToECDSA(AnvilKey)
	if err != nil {
		h.t.Fatalf("key: %v", err)
	}
	client, err := perps.NewClient(h.Deployment(), priv)
	if err != nil {
		h.t.Fatalf("new client: %v", err)
	}
	h.t.Cleanup(client.Close)
	return client
}

// Deployment is the perps deployment description for the harness chain.
func (h *Harness) Deployment() perps.Deployment {
	return perps.Deployment{ChainID: h.ChainID, Perpetuals: h.Perps, RPCURL: h.URL, Oracle: h.Owner}
}

// SendCalldata signs and submits a plain call from the harness signer and
// returns its receipt, so a test can drive calldata the service produced.
func (h *Harness) SendCalldata(to common.Address, data string) *types.Receipt {
	h.t.Helper()
	raw, err := hex.DecodeString(strings.TrimPrefix(data, "0x"))
	if err != nil {
		h.t.Fatalf("decode calldata: %v", err)
	}
	return sendRaw(h.t, h.raw, h.ChainID, to, raw)
}

// UpdatePrice pushes a new mark price through the oracle path.
func (h *Harness) UpdatePrice(price int64) {
	h.t.Helper()
	perpsABI := loadABI(h.t, "TigerSwapPerpetuals")
	packAndSend(h.t, h.raw, h.ChainID, h.Perps, perpsABI, "updatePrice", h.MarketID, wad(price))
}

// RPC returns the underlying client.
func (h *Harness) RPC() *ethclient.Client { return h.raw }

// PositionIDFromReceipt extracts the positionId from the PositionOpened event.
func (h *Harness) PositionIDFromReceipt(receipt *types.Receipt) common.Hash {
	h.t.Helper()
	perpsABI := loadABI(h.t, "TigerSwapPerpetuals")
	topic := perpsABI.Events["PositionOpened"].ID
	for _, lg := range receipt.Logs {
		if lg.Address != h.Perps || len(lg.Topics) == 0 || lg.Topics[0] != topic {
			continue
		}
		vals, err := perpsABI.Unpack("PositionOpened", lg.Data)
		if err != nil {
			h.t.Fatalf("unpack PositionOpened: %v", err)
		}
		if id, ok := vals[0].([32]byte); ok {
			return common.Hash(id)
		}
	}
	return common.Hash{}
}

// ABI returns a parsed artifact ABI from the testdata directory.
func ABI(t *testing.T, artifact string) abi.ABI { return loadABI(t, artifact) }

// WAD scales n by 1e18.
func WAD(n int64) *big.Int { return wad(n) }

// Bps returns n basis points as a WAD ratio.
func Bps(n int64) *big.Int { return bps(n) }

// --- internals ---

func startAnvil(t *testing.T) string {
	t.Helper()
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

func freePort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("find free port: %v", err)
	}
	defer l.Close()
	return fmt.Sprintf("%d", l.Addr().(*net.TCPAddr).Port)
}

// testdataDir locates the perps testdata directory relative to this source file,
// so the harness works no matter which package's test binary is running.
func testdataDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate harness source")
	}
	return filepath.Join(filepath.Dir(file), "..", "testdata")
}

func loadABI(t *testing.T, artifact string) abi.ABI {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(testdataDir(t), artifact+".abi.json"))
	if err != nil {
		t.Fatalf("read abi: %v", err)
	}
	parsed, err := abi.JSON(strings.NewReader(string(raw)))
	if err != nil {
		t.Fatalf("parse abi: %v", err)
	}
	return parsed
}

func deploy(t *testing.T, client *ethclient.Client, chainID int64, artifact string, ctorArgs []byte) common.Address {
	t.Helper()
	bytecodeHex, err := os.ReadFile(filepath.Join(testdataDir(t), artifact+".bin"))
	if err != nil {
		t.Fatalf("read bytecode: %v", err)
	}
	raw := strings.TrimPrefix(strings.TrimSpace(string(bytecodeHex)), "0x")
	code, err := hex.DecodeString(raw)
	if err != nil {
		t.Fatalf("decode bytecode: %v", err)
	}
	data := append(code, ctorArgs...)

	priv, err := crypto.HexToECDSA(AnvilKey)
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
	tx := types.NewContractCreation(nonce, big.NewInt(0), 8_000_000, gasPrice, data)
	signed, err := types.SignTx(tx, types.LatestSignerForChainID(big.NewInt(chainID)), priv)
	if err != nil {
		t.Fatalf("sign deploy: %v", err)
	}
	if err := client.SendTransaction(ctx, signed); err != nil {
		t.Fatalf("send deploy: %v", err)
	}
	return mustReceipt(t, client, signed.Hash()).ContractAddress
}

func mustReceipt(t *testing.T, client *ethclient.Client, hash common.Hash) *types.Receipt {
	t.Helper()
	ctx := context.Background()
	for i := 0; i < 200; i++ {
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

func sendRaw(t *testing.T, client *ethclient.Client, chainID int64, to common.Address, data []byte) *types.Receipt {
	t.Helper()
	priv, err := crypto.HexToECDSA(AnvilKey)
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

func packAndSend(t *testing.T, client *ethclient.Client, chainID int64, to common.Address, contractABI abi.ABI, method string, args ...interface{}) {
	t.Helper()
	data, err := contractABI.Pack(method, args...)
	if err != nil {
		t.Fatalf("pack %s: %v", method, err)
	}
	sendRaw(t, client, chainID, to, data)
}

func packStrings(t *testing.T, a, b string) []byte {
	t.Helper()
	strType, err := abi.NewType("string", "", nil)
	if err != nil {
		t.Fatalf("abi type: %v", err)
	}
	out, err := abi.Arguments{{Type: strType}, {Type: strType}}.Pack(a, b)
	if err != nil {
		t.Fatalf("pack strings: %v", err)
	}
	return out
}

func wad(n int64) *big.Int {
	return new(big.Int).Mul(big.NewInt(n), perps.OneUnit)
}

func bps(n int64) *big.Int {
	out := new(big.Int).Mul(big.NewInt(n), perps.OneUnit)
	return out.Div(out, big.NewInt(10_000))
}
