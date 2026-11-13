package orders

import (
	"context"
	"crypto/ecdsa"
	_ "embed"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

//go:embed twamm_abi.json
var twammABIJSON string

var twammABI abi.ABI

func init() {
	parsed, err := abi.JSON(strings.NewReader(twammABIJSON))
	if err != nil {
		panic(fmt.Sprintf("orders: invalid embedded TWAMM ABI: %v", err))
	}
	twammABI = parsed
}

// LongTermOrder mirrors TigerSwapTWAMM.LongTermOrder.
type LongTermOrder struct {
	Owner      common.Address
	IsToken0   bool
	SellRate   *big.Int
	Expiration *big.Int
	Escrow     *big.Int
	Proceeds   *big.Int
	Active     bool
}

// TWAMMClient is a TigerSwapTWAMM client for a single chain. A TWAMM order
// executes in virtual slices between its start and end, so there is no single
// transaction for a searcher to front-run.
type TWAMMClient struct {
	deployment Deployment
	rpc        *ethclient.Client
	closeFn    func()
	signer     *ecdsa.PrivateKey

	mu  sync.Mutex
	gas map[common.Address]uint64
}

// NewTWAMMClient dials the chain and binds the TWAMM contract.
func NewTWAMMClient(d Deployment) (*TWAMMClient, error) {
	if d.RPCURL == "" {
		return nil, fmt.Errorf("no RPC URL configured for chain %d", d.ChainID)
	}
	if d.TWAMM == (common.Address{}) {
		return nil, fmt.Errorf("TigerSwap TWAMM address not configured for chain %d", d.ChainID)
	}
	rpc, err := ethclient.Dial(d.RPCURL)
	if err != nil {
		return nil, fmt.Errorf("dial rpc for chain %d: %w", d.ChainID, err)
	}
	return &TWAMMClient{deployment: d, rpc: rpc, closeFn: rpc.Close, gas: make(map[common.Address]uint64)}, nil
}

// SetSigner binds a signer so Send can submit a transaction. Without one the
// client is read-only and Send returns ErrNoSigner.
func (t *TWAMMClient) SetSigner(key *ecdsa.PrivateKey) { t.signer = key }

// Send signs and broadcasts a TWAMM call with the bound signer and waits for a
// receipt.
func (t *TWAMMClient) Send(ctx context.Context, call Call) (*types.Receipt, error) {
	return sendCall(ctx, t.rpc, t.deployment.ChainID, call, t.signer, &t.mu, t.gas)
}

// SendWithKey signs and broadcasts a TWAMM call with an explicit signer.
func (t *TWAMMClient) SendWithKey(ctx context.Context, call Call, signer *ecdsa.PrivateKey) (*types.Receipt, error) {
	return sendCall(ctx, t.rpc, t.deployment.ChainID, call, signer, &t.mu, t.gas)
}

// Close releases the RPC connection.
func (t *TWAMMClient) Close() {
	if t.closeFn != nil {
		t.closeFn()
	}
}

// Deployment returns the deployment this client is bound to.
func (t *TWAMMClient) Deployment() Deployment { return t.deployment }

// Token0 reads the pool's first token.
func (t *TWAMMClient) Token0(ctx context.Context) (common.Address, error) {
	return t.addressView(ctx, "token0")
}

// Token1 reads the pool's second token.
func (t *TWAMMClient) Token1(ctx context.Context) (common.Address, error) {
	return t.addressView(ctx, "token1")
}

// OrderCount reads the number of long-term orders created.
func (t *TWAMMClient) OrderCount(ctx context.Context) (*big.Int, error) {
	return t.uintView(ctx, "orderCount")
}

// Price0In1 reads the pool price of token0 in token1 terms, scaled by 1e18.
func (t *TWAMMClient) Price0In1(ctx context.Context) (*big.Int, error) {
	return t.uintView(ctx, "price0In1")
}

// Order reads a long-term order by id.
func (t *TWAMMClient) Order(ctx context.Context, id *big.Int) (*LongTermOrder, error) {
	out, err := t.twammCall(ctx, "orders", id)
	if err != nil {
		return nil, err
	}
	vals, err := twammABI.Unpack("orders", out)
	if err != nil {
		return nil, fmt.Errorf("unpack orders: %w", err)
	}
	if len(vals) != 7 {
		return nil, fmt.Errorf("orders returned %d values", len(vals))
	}
	owner, _ := vals[0].(common.Address)
	isToken0, _ := vals[1].(bool)
	return &LongTermOrder{
		Owner:      owner,
		IsToken0:   isToken0,
		SellRate:   toBig(vals[2]),
		Expiration: toBig(vals[3]),
		Escrow:     toBig(vals[4]),
		Proceeds:   toBig(vals[5]),
		Active:     toBool(vals[6]),
	}, nil
}

// BuildCreateOrder encodes createOrder(isToken0, sellRate, expiration). The
// caller must have approved the TWAMM contract for the escrowed sell amount.
func (t *TWAMMClient) BuildCreateOrder(isToken0 bool, sellRate, expiration *big.Int) (Call, error) {
	if sellRate == nil || sellRate.Sign() <= 0 {
		return Call{}, errors.New("sellRate must be positive")
	}
	if expiration == nil || expiration.Sign() <= 0 {
		return Call{}, errors.New("expiration must be positive")
	}
	data, err := twammABI.Pack("createOrder", isToken0, sellRate, expiration)
	if err != nil {
		return Call{}, fmt.Errorf("pack createOrder: %w", err)
	}
	return Call{To: t.deployment.TWAMM, Data: data, Value: big.NewInt(0)}, nil
}

// BuildAddLiquidity encodes addLiquidity(amount0, amount1, minShares).
func (t *TWAMMClient) BuildAddLiquidity(amount0, amount1, minShares *big.Int) (Call, error) {
	if amount0 == nil || amount0.Sign() <= 0 || amount1 == nil || amount1.Sign() <= 0 {
		return Call{}, errors.New("liquidity amounts must be positive")
	}
	if minShares == nil {
		minShares = big.NewInt(0)
	}
	data, err := twammABI.Pack("addLiquidity", amount0, amount1, minShares)
	if err != nil {
		return Call{}, fmt.Errorf("pack addLiquidity: %w", err)
	}
	return Call{To: t.deployment.TWAMM, Data: data, Value: big.NewInt(0)}, nil
}

// BuildWithdrawOrder encodes withdrawOrder(orderId).
func (t *TWAMMClient) BuildWithdrawOrder(orderID *big.Int) (Call, error) {
	return t.buildSimple("withdrawOrder", orderID)
}

// BuildCancelOrder encodes cancelOrder(orderId).
func (t *TWAMMClient) BuildCancelOrder(orderID *big.Int) (Call, error) {
	return t.buildSimple("cancelOrder", orderID)
}

// BuildExecuteVirtualOrders encodes executeVirtualOrders(). Permissionless: any
// caller may advance the pool, and the result is a function of elapsed time, not
// of who submits it.
func (t *TWAMMClient) BuildExecuteVirtualOrders() (Call, error) {
	data, err := twammABI.Pack("executeVirtualOrders")
	if err != nil {
		return Call{}, fmt.Errorf("pack executeVirtualOrders: %w", err)
	}
	return Call{To: t.deployment.TWAMM, Data: data, Value: big.NewInt(0)}, nil
}

// BuildRemoveLiquidity encodes removeLiquidity(shares, minAmount0, minAmount1).
func (t *TWAMMClient) BuildRemoveLiquidity(shares, minAmount0, minAmount1 *big.Int) (Call, error) {
	if shares == nil || shares.Sign() <= 0 {
		return Call{}, errors.New("shares must be positive")
	}
	data, err := twammABI.Pack("removeLiquidity", shares, minAmount0, minAmount1)
	if err != nil {
		return Call{}, fmt.Errorf("pack removeLiquidity: %w", err)
	}
	return Call{To: t.deployment.TWAMM, Data: data, Value: big.NewInt(0)}, nil
}

func (t *TWAMMClient) buildSimple(method string, orderID *big.Int) (Call, error) {
	if orderID == nil || orderID.Sign() <= 0 {
		return Call{}, errors.New("orderId must be positive")
	}
	data, err := twammABI.Pack(method, orderID)
	if err != nil {
		return Call{}, fmt.Errorf("pack %s: %w", method, err)
	}
	return Call{To: t.deployment.TWAMM, Data: data, Value: big.NewInt(0)}, nil
}

// twammCall executes an eth_call against the TWAMM contract.
func (t *TWAMMClient) twammCall(ctx context.Context, method string, args ...interface{}) ([]byte, error) {
	data, err := twammABI.Pack(method, args...)
	if err != nil {
		return nil, fmt.Errorf("pack %s: %w", method, err)
	}
	to := t.deployment.TWAMM
	out, err := t.rpc.CallContract(ctx, ethereum.CallMsg{To: &to, Data: data}, nil)
	if err != nil {
		return nil, fmt.Errorf("call %s: %w", method, err)
	}
	return out, nil
}

func (t *TWAMMClient) addressView(ctx context.Context, method string) (common.Address, error) {
	out, err := t.twammCall(ctx, method)
	if err != nil {
		return common.Address{}, err
	}
	vals, err := twammABI.Unpack(method, out)
	if err != nil {
		return common.Address{}, fmt.Errorf("unpack %s: %w", method, err)
	}
	addr, ok := first(vals).(common.Address)
	if !ok {
		return common.Address{}, fmt.Errorf("%s returned %T, not an address", method, first(vals))
	}
	return addr, nil
}

func (t *TWAMMClient) uintView(ctx context.Context, method string) (*big.Int, error) {
	out, err := t.twammCall(ctx, method)
	if err != nil {
		return nil, err
	}
	vals, err := twammABI.Unpack(method, out)
	if err != nil {
		return nil, fmt.Errorf("unpack %s: %w", method, err)
	}
	return toBig(first(vals)), nil
}
