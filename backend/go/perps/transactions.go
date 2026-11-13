package perps

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

// ErrNoSigner is returned by a write method when the client has no signer.
var ErrNoSigner = errors.New("perps: no signer configured")

// SignerAddress returns the address that signs this client's transactions, or
// the zero address when no signer is configured.
func (c *Client) SignerAddress() common.Address {
	if c.signer == nil {
		return common.Address{}
	}
	return crypto.PubkeyToAddress(c.signer.PublicKey)
}

// Call describes an unsigned call to the perps contract: the target and the
// ABI-encoded data. It is what a browser wallet needs to sign an on-chain perp
// action itself, so the server never holds a user's key.
type Call struct {
	To    common.Address
	Data  []byte
	Value *big.Int
}

// Hex returns the call's calldata as a 0x-prefixed hex string.
func (call Call) Hex() string { return "0x" + common.Bytes2Hex(call.Data) }

// BuildOpenPosition encodes openPosition(marketId, size, collateral, isLong).
func (c *Client) BuildOpenPosition(marketID common.Hash, size, collateral *big.Int, isLong bool) (Call, error) {
	if size == nil || size.Sign() <= 0 {
		return Call{}, errors.New("size must be positive")
	}
	if collateral == nil || collateral.Sign() <= 0 {
		return Call{}, errors.New("collateral must be positive")
	}
	data, err := perpsABI.Pack("openPosition", marketID, size, collateral, isLong)
	if err != nil {
		return Call{}, fmt.Errorf("pack openPosition: %w", err)
	}
	return Call{To: c.deployment.Perpetuals, Data: data, Value: big.NewInt(0)}, nil
}

// BuildClosePosition encodes closePosition(positionId).
func (c *Client) BuildClosePosition(positionID common.Hash) (Call, error) {
	data, err := perpsABI.Pack("closePosition", positionID)
	if err != nil {
		return Call{}, fmt.Errorf("pack closePosition: %w", err)
	}
	return Call{To: c.deployment.Perpetuals, Data: data, Value: big.NewInt(0)}, nil
}

// BuildLiquidatePosition encodes liquidatePosition(positionId). Liquidation is
// permissionless on-chain, so any caller may submit it.
func (c *Client) BuildLiquidatePosition(positionID common.Hash) (Call, error) {
	data, err := perpsABI.Pack("liquidatePosition", positionID)
	if err != nil {
		return Call{}, fmt.Errorf("pack liquidatePosition: %w", err)
	}
	return Call{To: c.deployment.Perpetuals, Data: data, Value: big.NewInt(0)}, nil
}

// BuildAddMargin encodes addMargin(positionId, amount).
func (c *Client) BuildAddMargin(positionID common.Hash, amount *big.Int) (Call, error) {
	if amount == nil || amount.Sign() <= 0 {
		return Call{}, errors.New("amount must be positive")
	}
	data, err := perpsABI.Pack("addMargin", positionID, amount)
	if err != nil {
		return Call{}, fmt.Errorf("pack addMargin: %w", err)
	}
	return Call{To: c.deployment.Perpetuals, Data: data, Value: big.NewInt(0)}, nil
}

// BuildRemoveMargin encodes removeMargin(positionId, amount).
func (c *Client) BuildRemoveMargin(positionID common.Hash, amount *big.Int) (Call, error) {
	if amount == nil || amount.Sign() <= 0 {
		return Call{}, errors.New("amount must be positive")
	}
	data, err := perpsABI.Pack("removeMargin", positionID, amount)
	if err != nil {
		return Call{}, fmt.Errorf("pack removeMargin: %w", err)
	}
	return Call{To: c.deployment.Perpetuals, Data: data, Value: big.NewInt(0)}, nil
}

// BuildAddLiquidity encodes addLiquidity(token, amount) for an LP.
func (c *Client) BuildAddLiquidity(token common.Address, amount *big.Int) (Call, error) {
	if token == (common.Address{}) {
		return Call{}, errors.New("token must not be the zero address")
	}
	if amount == nil || amount.Sign() <= 0 {
		return Call{}, errors.New("amount must be positive")
	}
	data, err := perpsABI.Pack("addLiquidity", token, amount)
	if err != nil {
		return Call{}, fmt.Errorf("pack addLiquidity: %w", err)
	}
	return Call{To: c.deployment.Perpetuals, Data: data, Value: big.NewInt(0)}, nil
}

// BuildRemoveLiquidity encodes removeLiquidity(token, shares).
func (c *Client) BuildRemoveLiquidity(token common.Address, shares *big.Int) (Call, error) {
	if shares == nil || shares.Sign() <= 0 {
		return Call{}, errors.New("shares must be positive")
	}
	data, err := perpsABI.Pack("removeLiquidity", token, shares)
	if err != nil {
		return Call{}, fmt.Errorf("pack removeLiquidity: %w", err)
	}
	return Call{To: c.deployment.Perpetuals, Data: data, Value: big.NewInt(0)}, nil
}

// BuildCreateMarket encodes createMarket(...). Owner-only on-chain.
func (c *Client) BuildCreateMarket(marketID common.Hash, token common.Address, maxLeverage, liquidationFee, maintenanceMargin, initialMargin *big.Int) (Call, error) {
	data, err := perpsABI.Pack("createMarket", marketID, token, maxLeverage, liquidationFee, maintenanceMargin, initialMargin)
	if err != nil {
		return Call{}, fmt.Errorf("pack createMarket: %w", err)
	}
	return Call{To: c.deployment.Perpetuals, Data: data, Value: big.NewInt(0)}, nil
}

// BuildSetMarketActive encodes setMarketActive(marketId, active). Owner-only.
func (c *Client) BuildSetMarketActive(marketID common.Hash, active bool) (Call, error) {
	data, err := perpsABI.Pack("setMarketActive", marketID, active)
	if err != nil {
		return Call{}, fmt.Errorf("pack setMarketActive: %w", err)
	}
	return Call{To: c.deployment.Perpetuals, Data: data, Value: big.NewInt(0)}, nil
}

// BuildUpdatePrice encodes updatePrice(marketId, price). Oracle-only; the
// contract refuses a caller that is not the configured oracle.
func (c *Client) BuildUpdatePrice(marketID common.Hash, price *big.Int) (Call, error) {
	if price == nil || price.Sign() <= 0 {
		return Call{}, errors.New("price must be positive")
	}
	data, err := perpsABI.Pack("updatePrice", marketID, price)
	if err != nil {
		return Call{}, fmt.Errorf("pack updatePrice: %w", err)
	}
	return Call{To: c.deployment.Perpetuals, Data: data, Value: big.NewInt(0)}, nil
}

// BuildUpdateFundingRate encodes updateFundingRate(marketId, markPrice, indexPrice).
// Oracle-only.
func (c *Client) BuildUpdateFundingRate(marketID common.Hash, markPrice, indexPrice *big.Int) (Call, error) {
	if markPrice == nil || markPrice.Sign() <= 0 || indexPrice == nil || indexPrice.Sign() <= 0 {
		return Call{}, errors.New("mark and index price must be positive")
	}
	data, err := perpsABI.Pack("updateFundingRate", marketID, markPrice, indexPrice)
	if err != nil {
		return Call{}, fmt.Errorf("pack updateFundingRate: %w", err)
	}
	return Call{To: c.deployment.Perpetuals, Data: data, Value: big.NewInt(0)}, nil
}

// Send signs and broadcasts a Call with this client's signer and returns the
// mined receipt. It waits for inclusion so a caller learns the real outcome
// rather than a broadcast that later reverted.
func (c *Client) Send(ctx context.Context, call Call) (*types.Receipt, error) {
	if c.signer == nil {
		return nil, ErrNoSigner
	}
	from := crypto.PubkeyToAddress(c.signer.PublicKey)
	nonce, err := c.client.PendingNonceAt(ctx, from)
	if err != nil {
		return nil, fmt.Errorf("pending nonce: %w", err)
	}
	gasPrice, err := c.client.SuggestGasPrice(ctx)
	if err != nil {
		return nil, fmt.Errorf("suggest gas price: %w", err)
	}
	value := call.Value
	if value == nil {
		value = big.NewInt(0)
	}
	msg := ethereum.CallMsg{From: from, To: &call.To, Data: call.Data, Value: value}
	gasLimit, err := c.estimateGas(ctx, msg, call.To)
	if err != nil {
		return nil, err
	}
	tx := types.NewTransaction(nonce, call.To, value, gasLimit, gasPrice, call.Data)
	signed, err := types.SignTx(tx, types.LatestSignerForChainID(big.NewInt(c.deployment.ChainID)), c.signer)
	if err != nil {
		return nil, fmt.Errorf("sign tx: %w", err)
	}
	if err := c.client.SendTransaction(ctx, signed); err != nil {
		return nil, fmt.Errorf("send tx: %w", err)
	}
	return c.waitReceipt(ctx, signed.Hash())
}

// estimateGas estimates gas, caching the last successful limit per target so a
// reverted estimate on a state-dependent call (which depends on the caller's
// on-chain allowance and balance) does not block a legitimate send.
func (c *Client) estimateGas(ctx context.Context, msg ethereum.CallMsg, to common.Address) (uint64, error) {
	gas, err := c.client.EstimateGas(ctx, msg)
	if err == nil && gas > 0 {
		c.mu.Lock()
		c.gas[to] = gas
		c.mu.Unlock()
		return gas, nil
	}
	c.mu.Lock()
	cached := c.gas[to]
	c.mu.Unlock()
	if cached > 0 {
		return cached, nil
	}
	return 0, fmt.Errorf("estimate gas: %w", err)
}

// waitReceipt polls for a transaction receipt and rejects a reverted one, so a
// caller that gets no error knows the state change actually happened.
func (c *Client) waitReceipt(ctx context.Context, hash common.Hash) (*types.Receipt, error) {
	var lastErr error
	for {
		receipt, err := c.client.TransactionReceipt(ctx, hash)
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
