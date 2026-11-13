package orders

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// Call describes an unsigned call to the orders contract: the target and the
// ABI-encoded data. It is what a browser wallet needs to sign an on-chain order
// action itself, so the server never holds a user's key.
type Call struct {
	To    common.Address
	Data  []byte
	Value *big.Int
}

// Hex returns the call's calldata as a 0x-prefixed hex string.
func (call Call) Hex() string { return "0x" + common.Bytes2Hex(call.Data) }

// BuildCreateLimitOrder encodes createLimitOrder(side, tokenIn, tokenOut,
// amountIn, amountOutMin, price, expiresAt). The caller must have approved the
// orders contract for amountIn, because creation escrows the input.
func (c *Client) BuildCreateLimitOrder(side OrderSide, tokenIn, tokenOut common.Address, amountIn, amountOutMin, price, expiresAt *big.Int) (Call, error) {
	if err := validateCreate(tokenIn, tokenOut, amountIn, amountOutMin); err != nil {
		return Call{}, err
	}
	if price == nil || price.Sign() < 0 {
		return Call{}, errors.New("price must not be negative")
	}
	if expiresAt == nil || expiresAt.Sign() < 0 {
		return Call{}, errors.New("expiresAt must not be negative")
	}
	return c.packCreate("createLimitOrder", uint8(side), tokenIn, tokenOut, amountIn, amountOutMin, price, expiresAt)
}

// BuildCreateStopLossOrder encodes createStopLossOrder(...) with stopPrice.
func (c *Client) BuildCreateStopLossOrder(side OrderSide, tokenIn, tokenOut common.Address, amountIn, amountOutMin, stopPrice, expiresAt *big.Int) (Call, error) {
	if err := validateCreate(tokenIn, tokenOut, amountIn, amountOutMin); err != nil {
		return Call{}, err
	}
	if stopPrice == nil || stopPrice.Sign() <= 0 {
		return Call{}, errors.New("stopPrice must be positive")
	}
	if expiresAt == nil || expiresAt.Sign() < 0 {
		return Call{}, errors.New("expiresAt must not be negative")
	}
	return c.packCreate("createStopLossOrder", uint8(side), tokenIn, tokenOut, amountIn, amountOutMin, stopPrice, expiresAt)
}

// BuildCreateTakeProfitOrder encodes createTakeProfitOrder(...) with targetPrice.
func (c *Client) BuildCreateTakeProfitOrder(side OrderSide, tokenIn, tokenOut common.Address, amountIn, amountOutMin, targetPrice, expiresAt *big.Int) (Call, error) {
	if err := validateCreate(tokenIn, tokenOut, amountIn, amountOutMin); err != nil {
		return Call{}, err
	}
	if targetPrice == nil || targetPrice.Sign() <= 0 {
		return Call{}, errors.New("targetPrice must be positive")
	}
	if expiresAt == nil || expiresAt.Sign() < 0 {
		return Call{}, errors.New("expiresAt must not be negative")
	}
	return c.packCreate("createTakeProfitOrder", uint8(side), tokenIn, tokenOut, amountIn, amountOutMin, targetPrice, expiresAt)
}

// BuildCreateTWAPOrder encodes createTWAPOrder(...) with parts and interval.
func (c *Client) BuildCreateTWAPOrder(side OrderSide, tokenIn, tokenOut common.Address, amountIn, amountOutMin, parts, interval, expiresAt *big.Int) (Call, error) {
	if err := validateCreate(tokenIn, tokenOut, amountIn, amountOutMin); err != nil {
		return Call{}, err
	}
	if parts == nil || parts.Sign() <= 0 {
		return Call{}, errors.New("twap parts must be positive")
	}
	if interval == nil || interval.Sign() <= 0 {
		return Call{}, errors.New("twap interval must be positive")
	}
	if expiresAt == nil || expiresAt.Sign() < 0 {
		return Call{}, errors.New("expiresAt must not be negative")
	}
	return c.packCreate("createTWAPOrder", uint8(side), tokenIn, tokenOut, amountIn, amountOutMin, parts, interval, expiresAt)
}

// BuildCreateGTCOrder encodes createGTCOrder(...).
func (c *Client) BuildCreateGTCOrder(side OrderSide, tokenIn, tokenOut common.Address, amountIn, amountOutMin, price *big.Int) (Call, error) {
	if err := validateCreate(tokenIn, tokenOut, amountIn, amountOutMin); err != nil {
		return Call{}, err
	}
	if price == nil || price.Sign() < 0 {
		return Call{}, errors.New("price must not be negative")
	}
	return c.packCreate("createGTCOrder", uint8(side), tokenIn, tokenOut, amountIn, amountOutMin, price)
}

// BuildCreateIOCOrder encodes createIOCOrder(...).
func (c *Client) BuildCreateIOCOrder(side OrderSide, tokenIn, tokenOut common.Address, amountIn, amountOutMin, price *big.Int) (Call, error) {
	if err := validateCreate(tokenIn, tokenOut, amountIn, amountOutMin); err != nil {
		return Call{}, err
	}
	if price == nil || price.Sign() < 0 {
		return Call{}, errors.New("price must not be negative")
	}
	return c.packCreate("createIOCOrder", uint8(side), tokenIn, tokenOut, amountIn, amountOutMin, price)
}

// BuildCreatePostOnlyOrder encodes createPostOnlyOrder(...).
func (c *Client) BuildCreatePostOnlyOrder(side OrderSide, tokenIn, tokenOut common.Address, amountIn, amountOutMin, price, expiresAt *big.Int) (Call, error) {
	if err := validateCreate(tokenIn, tokenOut, amountIn, amountOutMin); err != nil {
		return Call{}, err
	}
	if price == nil || price.Sign() <= 0 {
		return Call{}, errors.New("price must be positive")
	}
	if expiresAt == nil || expiresAt.Sign() < 0 {
		return Call{}, errors.New("expiresAt must not be negative")
	}
	return c.packCreate("createPostOnlyOrder", uint8(side), tokenIn, tokenOut, amountIn, amountOutMin, price, expiresAt)
}

// validateCreate enforces the common escrow preconditions shared by every order
// type, so a builder never packs an order the contract would reject.
func validateCreate(tokenIn, tokenOut common.Address, amountIn, amountOutMin *big.Int) error {
	if tokenIn == (common.Address{}) || tokenOut == (common.Address{}) {
		return errors.New("tokenIn and tokenOut must be non-zero")
	}
	if tokenIn == tokenOut {
		return errors.New("tokenIn and tokenOut must differ")
	}
	if amountIn == nil || amountIn.Sign() <= 0 {
		return errors.New("amountIn must be positive")
	}
	if amountOutMin == nil || amountOutMin.Sign() < 0 {
		return errors.New("amountOutMin must not be negative")
	}
	return nil
}

// packCreate packs a create call with the given arguments.
func (c *Client) packCreate(method string, args ...interface{}) (Call, error) {
	data, err := ordersABI.Pack(method, args...)
	if err != nil {
		return Call{}, fmt.Errorf("pack %s: %w", method, err)
	}
	return Call{To: c.deployment.Orders, Data: data, Value: big.NewInt(0)}, nil
}

// BuildExecuteOrder encodes executeOrder(orderId, fillAmount). Executor-only
// on-chain; fillAmount 0 means fill the remainder.
func (c *Client) BuildExecuteOrder(orderID, fillAmount *big.Int) (Call, error) {
	if orderID == nil || orderID.Sign() <= 0 {
		return Call{}, errors.New("orderId must be positive")
	}
	if fillAmount == nil {
		fillAmount = big.NewInt(0)
	}
	data, err := ordersABI.Pack("executeOrder", orderID, fillAmount)
	if err != nil {
		return Call{}, fmt.Errorf("pack executeOrder: %w", err)
	}
	return Call{To: c.deployment.Orders, Data: data, Value: big.NewInt(0)}, nil
}

// BuildTriggerOrder encodes triggerOrder(orderId) for a stop/take-profit.
// Executor-only.
func (c *Client) BuildTriggerOrder(orderID *big.Int) (Call, error) {
	if orderID == nil || orderID.Sign() <= 0 {
		return Call{}, errors.New("orderId must be positive")
	}
	data, err := ordersABI.Pack("triggerOrder", orderID)
	if err != nil {
		return Call{}, fmt.Errorf("pack triggerOrder: %w", err)
	}
	return Call{To: c.deployment.Orders, Data: data, Value: big.NewInt(0)}, nil
}

// BuildCancelOrder encodes cancelOrder(orderId). Owner-only on-chain.
func (c *Client) BuildCancelOrder(orderID *big.Int) (Call, error) {
	if orderID == nil || orderID.Sign() <= 0 {
		return Call{}, errors.New("orderId must be positive")
	}
	data, err := ordersABI.Pack("cancelOrder", orderID)
	if err != nil {
		return Call{}, fmt.Errorf("pack cancelOrder: %w", err)
	}
	return Call{To: c.deployment.Orders, Data: data, Value: big.NewInt(0)}, nil
}

// BuildExpireOrder encodes expireOrder(orderId) for an order past its deadline.
func (c *Client) BuildExpireOrder(orderID *big.Int) (Call, error) {
	if orderID == nil || orderID.Sign() <= 0 {
		return Call{}, errors.New("orderId must be positive")
	}
	data, err := ordersABI.Pack("expireOrder", orderID)
	if err != nil {
		return Call{}, fmt.Errorf("pack expireOrder: %w", err)
	}
	return Call{To: c.deployment.Orders, Data: data, Value: big.NewInt(0)}, nil
}

// BuildCancelIOC encodes cancelIOC(orderId). Executor-only.
func (c *Client) BuildCancelIOC(orderID *big.Int) (Call, error) {
	if orderID == nil || orderID.Sign() <= 0 {
		return Call{}, errors.New("orderId must be positive")
	}
	data, err := ordersABI.Pack("cancelIOC", orderID)
	if err != nil {
		return Call{}, fmt.Errorf("pack cancelIOC: %w", err)
	}
	return Call{To: c.deployment.Orders, Data: data, Value: big.NewInt(0)}, nil
}

// BuildSetExecutor encodes setExecutor(executor, allowed). Owner-only.
func (c *Client) BuildSetExecutor(executor common.Address, allowed bool) (Call, error) {
	data, err := ordersABI.Pack("setExecutor", executor, allowed)
	if err != nil {
		return Call{}, fmt.Errorf("pack setExecutor: %w", err)
	}
	return Call{To: c.deployment.Orders, Data: data, Value: big.NewInt(0)}, nil
}

// Send signs and broadcasts a Call with this client's signer and returns the
// mined receipt. It waits for inclusion so a caller learns the real outcome
// rather than a broadcast that later reverted.
func (c *Client) Send(ctx context.Context, call Call) (*types.Receipt, error) {
	if c.signer == nil {
		return nil, ErrNoSigner
	}
	return c.sendWith(ctx, call, c.signer)
}

// SendWithKey submits a call signed by `signer` rather than the client's keeper
// key. Owner-only calls need the contract owner's key, which is not the
// keeper's, so a caller can sign a specific call without giving the whole client
// a different identity.
func (c *Client) SendWithKey(ctx context.Context, call Call, signer *ecdsa.PrivateKey) (*types.Receipt, error) {
	if signer == nil {
		return nil, ErrNoSigner
	}
	return c.sendWith(ctx, call, signer)
}

func (c *Client) sendWith(ctx context.Context, call Call, signer *ecdsa.PrivateKey) (*types.Receipt, error) {
	return sendCall(ctx, c.client, c.deployment.ChainID, call, signer, &c.mu, c.gas)
}

// sendCall signs and broadcasts a call, waiting for a mined receipt. It is
// shared by the orders and TWAMM clients so both submit transactions the same
// way, and caches the last successful gas limit per target so a state-dependent
// estimate failure does not block a legitimate send.
func sendCall(ctx context.Context, rpc *ethclient.Client, chainID int64, call Call, signer *ecdsa.PrivateKey, mu *sync.Mutex, gasCache map[common.Address]uint64) (*types.Receipt, error) {
	if signer == nil {
		return nil, ErrNoSigner
	}
	from := crypto.PubkeyToAddress(signer.PublicKey)
	nonce, err := rpc.PendingNonceAt(ctx, from)
	if err != nil {
		return nil, fmt.Errorf("pending nonce: %w", err)
	}
	gasPrice, err := rpc.SuggestGasPrice(ctx)
	if err != nil {
		return nil, fmt.Errorf("suggest gas price: %w", err)
	}
	value := call.Value
	if value == nil {
		value = big.NewInt(0)
	}
	msg := ethereum.CallMsg{From: from, To: &call.To, Data: call.Data, Value: value}
	gasLimit, err := estimateGas(ctx, rpc, msg, call.To, mu, gasCache)
	if err != nil {
		return nil, err
	}
	tx := types.NewTransaction(nonce, call.To, value, gasLimit, gasPrice, call.Data)
	signed, err := types.SignTx(tx, types.LatestSignerForChainID(big.NewInt(chainID)), signer)
	if err != nil {
		return nil, fmt.Errorf("sign tx: %w", err)
	}
	if err := rpc.SendTransaction(ctx, signed); err != nil {
		return nil, fmt.Errorf("send tx: %w", err)
	}
	return waitReceipt(ctx, rpc, signed.Hash())
}

// estimateGas estimates gas, caching the last successful limit per target so a
// reverted estimate on a state-dependent call does not block a legitimate send.
func estimateGas(ctx context.Context, rpc *ethclient.Client, msg ethereum.CallMsg, to common.Address, mu *sync.Mutex, cache map[common.Address]uint64) (uint64, error) {
	gas, err := rpc.EstimateGas(ctx, msg)
	if err == nil && gas > 0 {
		mu.Lock()
		cache[to] = gas
		mu.Unlock()
		return gas, nil
	}
	mu.Lock()
	cached := cache[to]
	mu.Unlock()
	if cached > 0 {
		return cached, nil
	}
	return 0, fmt.Errorf("estimate gas: %w", err)
}

// waitReceipt polls for a transaction receipt and rejects a reverted one, so a
// caller that gets no error knows the state change actually happened.
func waitReceipt(ctx context.Context, rpc *ethclient.Client, hash common.Hash) (*types.Receipt, error) {
	var lastErr error
	for {
		receipt, err := rpc.TransactionReceipt(ctx, hash)
		if err == nil && receipt != nil {
			if receipt.Status == types.ReceiptStatusFailed {
				return receipt, fmt.Errorf("transaction %s reverted", hash.Hex())
			}
			return receipt, nil
		}
		lastErr = err
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("wait for receipt %s: %w (last: %v)", hash.Hex(), ctx.Err(), lastErr)
		default:
		}
		if err := sleepCtx(ctx, receiptPollInterval); err != nil {
			return nil, err
		}
	}
}

// receiptPollInterval is how long waitReceipt sleeps between receipt polls. It
// is a variable so a test can shrink it.
var receiptPollInterval = 250 * time.Millisecond

func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
