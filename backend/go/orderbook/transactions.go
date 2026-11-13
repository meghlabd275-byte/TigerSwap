package orderbook

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

const (
	receiptPollInterval = 500 * time.Millisecond
	receiptTimeout      = 3 * time.Minute
)

// Send signs and broadcasts a Call with this client's signer and returns the
// mined receipt. It waits for inclusion so a caller learns the real outcome
// rather than a broadcast that later reverted.
func (c *Client) Send(ctx context.Context, call Call) (*types.Receipt, error) {
	if c.signer == nil {
		return nil, ErrNoSigner
	}
	return c.sendWith(ctx, call, c.signer)
}

// SendWithKey submits a call signed by `signer` rather than the client's key.
// Owner-only calls need the contract owner's key, which is not the keeper's.
func (c *Client) SendWithKey(ctx context.Context, call Call, signer *ecdsa.PrivateKey) (*types.Receipt, error) {
	if signer == nil {
		return nil, ErrNoSigner
	}
	return c.sendWith(ctx, call, signer)
}

func (c *Client) sendWith(ctx context.Context, call Call, signer *ecdsa.PrivateKey) (*types.Receipt, error) {
	from := crypto.PubkeyToAddress(signer.PublicKey)
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
	signed, err := types.SignTx(tx, types.LatestSignerForChainID(big.NewInt(c.deployment.ChainID)), signer)
	if err != nil {
		return nil, fmt.Errorf("sign tx: %w", err)
	}
	if err := c.client.SendTransaction(ctx, signed); err != nil {
		return nil, fmt.Errorf("send tx: %w", err)
	}
	return c.waitReceipt(ctx, signed.Hash())
}

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

func (c *Client) waitReceipt(ctx context.Context, hash common.Hash) (*types.Receipt, error) {
	deadline := time.Now().Add(receiptTimeout)
	var lastErr error
	for {
		receipt, err := c.client.TransactionReceipt(ctx, hash)
		if err == nil && receipt != nil {
			if receipt.Status == types.ReceiptStatusFailed {
				return receipt, fmt.Errorf("transaction %s reverted", hash.Hex())
			}
			return receipt, nil
		}
		if err != nil && !errors.Is(err, ethereum.NotFound) {
			lastErr = err
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("wait for receipt %s: timed out (last: %v)", hash.Hex(), lastErr)
		}
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("wait for receipt %s: %w", hash.Hex(), ctx.Err())
		case <-time.After(receiptPollInterval):
		}
	}
}
