package services

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// erc20ApproveABI is the minimal ERC-20 approval interface used for router allowances.
const erc20ApproveABI = `[{"constant":false,"inputs":[{"name":"spender","type":"address"},{"name":"amount","type":"uint256"}],"name":"approve","outputs":[{"name":"","type":"bool"}],"type":"function"}]`

// pairLiquidityABI covers the V2-style add/remove liquidity entry points.
const pairLiquidityABI = `[
 {"inputs":[{"internalType":"address","name":"tokenA","type":"address"},{"internalType":"address","name":"tokenB","type":"address"},{"internalType":"uint256","name":"amountADesired","type":"uint256"},{"internalType":"uint256","name":"amountBDesired","type":"uint256"},{"internalType":"uint256","name":"amountAMin","type":"uint256"},{"internalType":"uint256","name":"amountBMin","type":"uint256"},{"internalType":"address","name":"to","type":"address"},{"internalType":"uint256","name":"deadline","type":"uint256"}],"name":"addLiquidity","outputs":[{"internalType":"uint256","name":"amountA","type":"uint256"},{"internalType":"uint256","name":"amountB","type":"uint256"},{"internalType":"uint256","name":"liquidity","type":"uint256"}],"stateMutability":"nonpayable","type":"function"},
 {"inputs":[{"internalType":"address","name":"tokenA","type":"address"},{"internalType":"address","name":"tokenB","type":"address"},{"internalType":"uint256","name":"liquidity","type":"uint256"},{"internalType":"uint256","name":"amountAMin","type":"uint256"},{"internalType":"uint256","name":"amountBMin","type":"uint256"},{"internalType":"address","name":"to","type":"address"},{"internalType":"uint256","name":"deadline","type":"uint256"}],"name":"removeLiquidity","outputs":[{"internalType":"uint256","name":"amountA","type":"uint256"},{"internalType":"uint256","name":"amountB","type":"uint256"}],"stateMutability":"nonpayable","type":"function"}
]`

// waitForReceipt polls the chain until the transaction is mined or the timeout expires.
func waitForReceipt(ctx context.Context, client *ethclient.Client, hash common.Hash, timeout time.Duration) (*types.Receipt, error) {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		receipt, err := client.TransactionReceipt(ctx, hash)
		if err == nil && receipt != nil {
			return receipt, nil
		}
		if err != nil && err != ethereum.NotFound {
			return nil, fmt.Errorf("poll receipt: %w", err)
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("transaction %s not mined within %s", hash.Hex(), timeout)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}

// signAndSend builds, signs and broadcasts a legacy transaction, returning the tx hash.
func signAndSend(ctx context.Context, client *ethclient.Client, key *ecdsa.PrivateKey, chainID int64, to common.Address, value *big.Int, data []byte) (string, error) {
	owner := crypto.PubkeyToAddress(key.PublicKey)

	nonce, err := client.PendingNonceAt(ctx, owner)
	if err != nil {
		return "", fmt.Errorf("pending nonce: %w", err)
	}
	gasPrice, err := client.SuggestGasPrice(ctx)
	if err != nil {
		return "", fmt.Errorf("suggest gas price: %w", err)
	}

	gasLimit, err := client.EstimateGas(ctx, ethereum.CallMsg{From: owner, To: &to, Value: value, Data: data})
	if err != nil {
		return "", fmt.Errorf("estimate gas: %w", err)
	}
	gasLimit += gasLimit / 5

	tx := types.NewTransaction(nonce, to, value, gasLimit, gasPrice, data)
	signed, err := types.SignTx(tx, types.LatestSignerForChainID(big.NewInt(chainID)), key)
	if err != nil {
		return "", fmt.Errorf("sign transaction: %w", err)
	}
	if err := client.SendTransaction(ctx, signed); err != nil {
		return "", fmt.Errorf("send transaction: %w", err)
	}
	return signed.Hash().Hex(), nil
}

// ApproveToken grants an unlimited allowance from privateKey's address to spender.
func (b *BlockchainClient) ApproveToken(ctx context.Context, chainID int64, privateKeyHex, tokenAddress, spender string) (string, error) {
	client, err := b.GetClient(chainID)
	if err != nil {
		return "", err
	}
	key, err := crypto.HexToECDSA(strings.TrimPrefix(privateKeyHex, "0x"))
	if err != nil {
		return "", fmt.Errorf("invalid private key: %w", err)
	}

	approveABI, err := abi.JSON(strings.NewReader(erc20ApproveABI))
	if err != nil {
		return "", err
	}
	max := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))
	data, err := approveABI.Pack("approve", common.HexToAddress(spender), max)
	if err != nil {
		return "", err
	}

	return signAndSend(ctx, client, key, chainID, common.HexToAddress(tokenAddress), big.NewInt(0), data)
}

// AddLiquidity adds liquidity to a V2-style pair from privateKey's address.
func (s *SwapExecutor) AddLiquidity(ctx context.Context, chainID int64, privateKeyHex, tokenA, tokenB, amountA, amountB string) (string, error) {
	client, err := s.blockchain.GetClient(chainID)
	if err != nil {
		return "", err
	}
	key, err := crypto.HexToECDSA(strings.TrimPrefix(privateKeyHex, "0x"))
	if err != nil {
		return "", fmt.Errorf("invalid private key: %w", err)
	}
	owner := crypto.PubkeyToAddress(key.PublicKey)

	router := s.GetDEXConfig(chainID, s.preferredDEX(chainID))
	if router == nil || router.Router == "" {
		return "", fmt.Errorf("no router configured for chain %d", chainID)
	}

	liqABI, err := abi.JSON(strings.NewReader(pairLiquidityABI))
	if err != nil {
		return "", err
	}

	amtA, ok := new(big.Int).SetString(amountA, 10)
	if !ok {
		return "", fmt.Errorf("invalid amountA %q", amountA)
	}
	amtB, ok := new(big.Int).SetString(amountB, 10)
	if !ok {
		return "", fmt.Errorf("invalid amountB %q", amountB)
	}

	// Approve both tokens to the router before adding liquidity.
	if _, err := s.blockchain.ApproveToken(ctx, chainID, privateKeyHex, tokenA, router.Router); err != nil {
		return "", fmt.Errorf("approve tokenA: %w", err)
	}
	if _, err := s.blockchain.ApproveToken(ctx, chainID, privateKeyHex, tokenB, router.Router); err != nil {
		return "", fmt.Errorf("approve tokenB: %w", err)
	}

	minA := new(big.Int).Div(new(big.Int).Mul(amtA, big.NewInt(995)), big.NewInt(1000))
	minB := new(big.Int).Div(new(big.Int).Mul(amtB, big.NewInt(995)), big.NewInt(1000))
	deadline := big.NewInt(time.Now().Add(20 * time.Minute).Unix())

	data, err := liqABI.Pack("addLiquidity",
		common.HexToAddress(tokenA), common.HexToAddress(tokenB),
		amtA, amtB, minA, minB, owner, deadline)
	if err != nil {
		return "", err
	}

	return signAndSend(ctx, client, key, chainID, common.HexToAddress(router.Router), big.NewInt(0), data)
}

// RemoveLiquidity burns LP tokens and returns the underlying assets.
func (s *SwapExecutor) RemoveLiquidity(ctx context.Context, chainID int64, privateKeyHex, poolAddr, liquidity, amountA, amountB string) (string, error) {
	client, err := s.blockchain.GetClient(chainID)
	if err != nil {
		return "", err
	}
	key, err := crypto.HexToECDSA(strings.TrimPrefix(privateKeyHex, "0x"))
	if err != nil {
		return "", fmt.Errorf("invalid private key: %w", err)
	}
	owner := crypto.PubkeyToAddress(key.PublicKey)

	liqABI, err := abi.JSON(strings.NewReader(pairLiquidityABI))
	if err != nil {
		return "", err
	}

	liq, ok := new(big.Int).SetString(liquidity, 10)
	if !ok {
		return "", fmt.Errorf("invalid liquidity %q", liquidity)
	}
	minA, _ := new(big.Int).SetString(amountA, 10)
	minB, _ := new(big.Int).SetString(amountB, 10)
	if minA == nil {
		minA = big.NewInt(0)
	}
	if minB == nil {
		minB = big.NewInt(0)
	}
	deadline := big.NewInt(time.Now().Add(20 * time.Minute).Unix())

	// removeLiquidity takes the two underlying token addresses in the pair.
	data, err := liqABI.Pack("removeLiquidity",
		common.HexToAddress(poolAddr), common.HexToAddress(poolAddr),
		liq, minA, minB, owner, deadline)
	if err != nil {
		return "", err
	}

	return signAndSend(ctx, client, key, chainID, common.HexToAddress(poolAddr), big.NewInt(0), data)
}

// preferredDEX returns the primary DEX name configured for a chain.
func (s *SwapExecutor) preferredDEX(chainID int64) string {
	if names, ok := s.supportedDEX[chainID]; ok && len(names) > 0 {
		return names[0]
	}
	return ""
}
