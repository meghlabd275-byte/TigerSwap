package orders

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"net"
	"os"
	"os/exec"
	"path/filepath"
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

// harness is a throwaway anvil chain with the real order contracts deployed and
// a funded xyK pool, so an order fill exercises the actual contract path.
type harness struct {
	url      string
	chainID  int64
	orders   common.Address
	twamm    common.Address
	router   common.Address
	oracle   common.Address
	tokenIn  common.Address
	tokenOut common.Address
	owner    common.Address
	rpc      *ethclient.Client
	t        *testing.T
}

// startHarness launches anvil and deploys the contracts. The test is skipped
// when anvil is not installed, so the suite stays green without Foundry; CI
// installs it, so the on-chain path is exercised there.
func startHarness(t *testing.T) *harness {
	t.Helper()
	if _, err := exec.LookPath("anvil"); err != nil {
		t.Skip("anvil not installed; skipping on-chain orders integration test")
	}
	url := startAnvil(t)
	chainID := int64(31337)

	priv, err := crypto.HexToECDSA(anvilKey)
	if err != nil {
		t.Fatalf("key: %v", err)
	}
	owner := crypto.PubkeyToAddress(priv.PublicKey)

	rpc, err := ethclient.Dial(url)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(rpc.Close)

	erc20ABI := loadABI(t, "TestERC20")
	factoryABI := loadABI(t, "TigerSwapFactory")
	routerABI := loadABI(t, "TigerSwapRouter")
	oracleABI := loadABI(t, "TigerSwapPriceOracle")

	// Two tokens with the pool, a WETH placeholder for the router constructor.
	tokenIn := deploy(t, rpc, chainID, "TestERC20", packStrings(t, "TKA", "TKA"))
	tokenOut := deploy(t, rpc, chainID, "TestERC20", packStrings(t, "TKB", "TKB"))
	weth := deploy(t, rpc, chainID, "TestERC20", packStrings(t, "WETH", "WETH"))

	factory := deploy(t, rpc, chainID, "TigerSwapFactory", nil)
	router := deployWithArgs(t, rpc, chainID, "TigerSwapRouter", packAddressAddress(t, factory, weth))
	oracle := deployWithArgs(t, rpc, chainID, "TigerSwapPriceOracle", packUintUint(t, 3600, 1))
	ordersAddr := deployWithArgs(t, rpc, chainID, "TigerSwapAdvancedOrders", packAddressAddress(t, oracle, router))
	twammAddr := deployWithArgs(t, rpc, chainID, "TigerSwapTWAMM", packTWAMMCtor(t, tokenIn, tokenOut, owner, big.NewInt(0)))

	// Create the xyK pool and seed it with liquidity through the router.
	packAndSend(t, rpc, chainID, factory, factoryABI, "createPool", tokenIn, tokenOut)

	maxUint := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))
	for _, tok := range []common.Address{tokenIn, tokenOut, weth} {
		packAndSend(t, rpc, chainID, tok, erc20ABI, "mint", owner, wad(1_000_000_000))
		packAndSend(t, rpc, chainID, tok, erc20ABI, "approve", router, maxUint)
		packAndSend(t, rpc, chainID, tok, erc20ABI, "approve", ordersAddr, maxUint)
	}
	deadline := big.NewInt(time.Now().Add(time.Hour).Unix())
	packAndSend(t, rpc, chainID, router, routerABI, "addLiquidity0",
		tokenIn, tokenOut, wad(100_000), wad(100_000), big.NewInt(0), big.NewInt(0), owner, deadline)

	// Register the executor so the deployed contract accepts our fills, and point
	// the order contract at the price asset its conditions are evaluated against.
	ordersABI := loadABI(t, "TigerSwapAdvancedOrders")
	packAndSend(t, rpc, chainID, ordersAddr, ordersABI, "setExecutor", owner, true)
	packAndSend(t, rpc, chainID, ordersAddr, ordersABI, "setPriceAsset", tokenIn)

	// Seed the oracle with a price so a limit order's condition can be met.
	packAndSend(t, rpc, chainID, oracle, oracleABI, "setReporter", owner, true)
	packAndSend(t, rpc, chainID, oracle, oracleABI, "submitPrice", tokenIn, wad(1))

	return &harness{
		url: url, chainID: chainID, orders: ordersAddr, twamm: twammAddr,
		router: router, oracle: oracle, tokenIn: tokenIn, tokenOut: tokenOut,
		owner: owner, rpc: rpc, t: t,
	}
}

func (h *harness) client() *Client {
	h.t.Helper()
	priv, err := crypto.HexToECDSA(anvilKey)
	if err != nil {
		h.t.Fatalf("key: %v", err)
	}
	c, err := NewClient(Deployment{ChainID: h.chainID, Orders: h.orders, TWAMM: h.twamm, RPCURL: h.url}, priv)
	if err != nil {
		h.t.Fatalf("client: %v", err)
	}
	h.t.Cleanup(c.Close)
	return c
}

func (h *harness) twammClient() *TWAMMClient {
	h.t.Helper()
	c, err := NewTWAMMClient(Deployment{ChainID: h.chainID, Orders: h.orders, TWAMM: h.twamm, RPCURL: h.url})
	if err != nil {
		h.t.Fatalf("twamm client: %v", err)
	}
	h.t.Cleanup(c.Close)
	return c
}

// --- helpers ---

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
		t.Fatalf("free port: %v", err)
	}
	defer l.Close()
	return strconv.Itoa(l.Addr().(*net.TCPAddr).Port)
}

func testdataDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs(filepath.Join("testdata"))
	if err != nil {
		t.Fatalf("testdata dir: %v", err)
	}
	return dir
}

func loadABI(t *testing.T, artifact string) abi.ABI {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(testdataDir(t), artifact+".abi.json"))
	if err != nil {
		t.Fatalf("read abi %s: %v", artifact, err)
	}
	parsed, err := abi.JSON(strings.NewReader(string(raw)))
	if err != nil {
		t.Fatalf("parse abi %s: %v", artifact, err)
	}
	return parsed
}

func deploy(t *testing.T, client *ethclient.Client, chainID int64, artifact string, ctorArgs []byte) common.Address {
	t.Helper()
	return deployWithArgs(t, client, chainID, artifact, ctorArgs)
}

func deployWithArgs(t *testing.T, client *ethclient.Client, chainID int64, artifact string, ctorArgs []byte) common.Address {
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

	priv, err := crypto.HexToECDSA(anvilKey)
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
	tx := types.NewContractCreation(nonce, big.NewInt(0), 12_000_000, gasPrice, data)
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
	for i := 0; i < 300; i++ {
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
	priv, err := crypto.HexToECDSA(anvilKey)
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
	tx := types.NewTransaction(nonce, to, big.NewInt(0), 12_000_000, gasPrice, data)
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
	stringTy, _ := abi.NewType("string", "", nil)
	args := abi.Arguments{{Type: stringTy}, {Type: stringTy}}
	packed, err := args.Pack(a, b)
	if err != nil {
		t.Fatalf("pack strings: %v", err)
	}
	return packed
}

func packAddressAddress(t *testing.T, a, b common.Address) []byte {
	t.Helper()
	addrTy, _ := abi.NewType("address", "", nil)
	args := abi.Arguments{{Type: addrTy}, {Type: addrTy}}
	packed, err := args.Pack(a, b)
	if err != nil {
		t.Fatalf("pack addresses: %v", err)
	}
	return packed
}

func packUintUint(t *testing.T, a, b uint64) []byte {
	t.Helper()
	uintTy, _ := abi.NewType("uint256", "", nil)
	args := abi.Arguments{{Type: uintTy}, {Type: uintTy}}
	packed, err := args.Pack(big.NewInt(int64(a)), big.NewInt(int64(b)))
	if err != nil {
		t.Fatalf("pack uints: %v", err)
	}
	return packed
}

// packTWAMMCtor packs the TWAMM constructor arguments:
// (address token0, address token1, address feeRecipient, uint256 fee).
func packTWAMMCtor(t *testing.T, token0, token1, feeRecipient common.Address, fee *big.Int) []byte {
	t.Helper()
	addrTy, _ := abi.NewType("address", "", nil)
	uintTy, _ := abi.NewType("uint256", "", nil)
	args := abi.Arguments{{Type: addrTy}, {Type: addrTy}, {Type: addrTy}, {Type: uintTy}}
	packed, err := args.Pack(token0, token1, feeRecipient, fee)
	if err != nil {
		t.Fatalf("pack twamm ctor: %v", err)
	}
	return packed
}

func wad(n int64) *big.Int {
	return new(big.Int).Mul(big.NewInt(n), new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
}

var _ = json.Marshal
var _ = fmt.Sprintf
var _ = ethereum.NotFound
