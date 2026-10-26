package services

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"tigerswap/backend/go/amm"
	"tigerswap/backend/go/tigerswap"
)

// Pool family selectors understood by the TigerSwap factory and router.
const (
	FamilyV2       uint8 = 0
	FamilyV3       uint8 = 1
	FamilyStable   uint8 = 2
	FamilyWeighted uint8 = 3
)

// SwapExecutor executes swaps against TigerSwap's own contracts. It holds no
// third-party DEX routers: every quote and every transaction targets a
// TigerSwap factory or router deployed on the requested chain.
type SwapExecutor struct {
	blockchain *BlockchainClient
	priceAgg   *PriceAggregator
	redis      *redis.Client

	deployments map[int64]tigerswap.Deployment
	signer      *ecdsa.PrivateKey
}

// SwapResult is the outcome of a swap execution.
type SwapResult struct {
	TxHash           string        `json:"tx_hash"`
	Status           string        `json:"status"`
	FromToken        string        `json:"from_token"`
	ToToken          string        `json:"to_token"`
	AmountIn         string        `json:"amount_in"`
	AmountOut        string        `json:"amount_out"`
	AmountOutMin     string        `json:"amount_out_min"`
	PriceImpact      float64       `json:"price_impact"`
	GasUsed          uint64        `json:"gas_used"`
	GasFee           string        `json:"gas_fee"`
	DEX              string        `json:"dex"`
	Route            []string      `json:"route"`
	BlockNumber      uint64        `json:"block_number"`
	ConfirmationTime time.Duration `json:"confirmation_time"`
}

// NewSwapExecutor builds an executor from the TigerSwap deployments configured
// in the environment. Chains without a deployment are simply unsupported.
func NewSwapExecutor(blockchain *BlockchainClient, priceAgg *PriceAggregator, redisClient *redis.Client) *SwapExecutor {
	executor := &SwapExecutor{
		blockchain:  blockchain,
		priceAgg:    priceAgg,
		redis:       redisClient,
		deployments: make(map[int64]tigerswap.Deployment),
	}

	for _, deployment := range tigerswap.LoadDeployments() {
		executor.deployments[deployment.ChainID] = deployment
	}

	if signer, err := tigerswap.LoadSigner(); err == nil {
		executor.signer = signer
	}

	return executor
}

// GetDeployment returns the TigerSwap deployment for a chain, if configured.
func (s *SwapExecutor) GetDeployment(chainID int64) (tigerswap.Deployment, bool) {
	deployment, ok := s.deployments[chainID]
	return deployment, ok
}

// SupportedChains lists the chains with a TigerSwap deployment.
func (s *SwapExecutor) SupportedChains() []int64 {
	chains := make([]int64, 0, len(s.deployments))
	for chainID := range s.deployments {
		chains = append(chains, chainID)
	}
	return chains
}

// SupportedFamilies lists the pool families a chain can quote against.
func (s *SwapExecutor) SupportedFamilies() []string {
	return []string{"tigerswap_v2", "tigerswap_v3", "tigerswap_stable", "tigerswap_weighted"}
}

func familyFromName(name string) (amm.Family, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "tigerswap_v2", "v2", "xyk":
		return amm.FamilyV2, nil
	case "tigerswap_v3", "v3", "concentrated":
		return amm.FamilyV3, nil
	case "tigerswap_stable", "stable", "stableswap":
		return amm.FamilyStable, nil
	case "tigerswap_weighted", "weighted", "balancer":
		return amm.FamilyWeighted, nil
	default:
		return amm.FamilyV2, fmt.Errorf("unknown pool family %q", name)
	}
}

// newClient builds a TigerSwap on-chain client for the chain.
func (s *SwapExecutor) newClient(chainID int64, key *ecdsa.PrivateKey) (*tigerswap.Client, error) {
	deployment, ok := s.deployments[chainID]
	if !ok {
		return nil, fmt.Errorf("TigerSwap is not deployed on chain %d", chainID)
	}
	if deployment.RPCURL == "" {
		if rpc, err := s.blockchain.RPCURLForChain(chainID); err == nil {
			deployment.RPCURL = rpc
		}
	}
	return tigerswap.NewClient(deployment, key)
}

// GetSwapQuote quotes a swap by reading TigerSwap pool state on-chain and
// picking the family with the best output.
func (s *SwapExecutor) GetSwapQuote(ctx context.Context, chainID int64, fromToken, toToken, amount string) (*AggregatedQuote, error) {
	amountIn, ok := new(big.Int).SetString(amount, 10)
	if !ok || amountIn.Sign() <= 0 {
		return nil, fmt.Errorf("invalid amount %q", amount)
	}

	client, err := s.newClient(chainID, nil)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	path := []common.Address{common.HexToAddress(fromToken), common.HexToAddress(toToken)}

	var best *AggregatedQuote
	quotes := make(map[string]string)

	for _, candidate := range []struct {
		family amm.Family
		name   string
	}{
		{amm.FamilyV2, "tigerswap_v2"},
		{amm.FamilyV3, "tigerswap_v3"},
		{amm.FamilyStable, "tigerswap_stable"},
		{amm.FamilyWeighted, "tigerswap_weighted"},
	} {
		out, quoteErr := client.QuoteFamily(ctx, candidate.family, path, amountIn, DefaultFeeBps)
		if quoteErr != nil {
			// A missing pool for one family is expected; try the next.
			continue
		}
		quotes[candidate.name] = out.String()

		if best == nil || out.Cmp(bestAmountOut(best)) > 0 {
			best = &AggregatedQuote{
				FromToken:    fromToken,
				ToToken:      toToken,
				AmountIn:     amount,
				AmountOut:    out.String(),
				AmountOutMin: applySlippage(out, DefaultSlippageBps).String(),
				BestSource:   candidate.name,
				BlockNumber:  0,
				ValidUntil:   time.Now().Add(30 * time.Second),
			}
		}
	}

	if best == nil {
		return nil, fmt.Errorf("no TigerSwap pool found for %s/%s on chain %d", fromToken, toToken, chainID)
	}

	best.SourceQuotes = quotes
	best.Route = []string{fromToken, toToken}

	return best, nil
}

func bestAmountOut(q *AggregatedQuote) *big.Int {
	out, ok := new(big.Int).SetString(q.AmountOut, 10)
	if !ok {
		return big.NewInt(0)
	}
	return out
}

// DefaultFeeBps is the fee used when a pool family does not expose its own.
const DefaultFeeBps uint32 = 30

// DefaultSlippageBps is the default slippage tolerance.
const DefaultSlippageBps uint32 = 50

func applySlippage(amount *big.Int, slippageBps uint32) *big.Int {
	if slippageBps >= 10_000 {
		return big.NewInt(0)
	}
	return new(big.Int).Div(new(big.Int).Mul(amount, big.NewInt(int64(10_000-slippageBps))), big.NewInt(10_000))
}

// ExecuteSwap signs and broadcasts a swap on the TigerSwap router.
func (s *SwapExecutor) ExecuteSwap(ctx context.Context, chainID int64, privateKey, fromToken, toToken, amountIn, slippageStr string, familyName string) (*SwapResult, error) {
	key, err := resolveSignerKey(privateKey, s.signer)
	if err != nil {
		return nil, err
	}

	family, err := familyFromName(familyName)
	if err != nil {
		return nil, err
	}

	amount, ok := new(big.Int).SetString(amountIn, 10)
	if !ok || amount.Sign() <= 0 {
		return nil, fmt.Errorf("invalid amount %q", amountIn)
	}

	slippageBps := DefaultSlippageBps
	if slippageStr != "" {
		parsed, parseErr := strconv.ParseUint(strings.TrimSpace(slippageStr), 10, 32)
		if parseErr != nil {
			return nil, fmt.Errorf("invalid slippage %q", slippageStr)
		}
		if parsed > 5_000 {
			return nil, errors.New("slippage above 50% is not permitted")
		}
		slippageBps = uint32(parsed)
	}

	client, err := s.newClient(chainID, key)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	path := []common.Address{common.HexToAddress(fromToken), common.HexToAddress(toToken)}

	expectedOut, err := client.QuoteFamily(ctx, family, path, amount, DefaultFeeBps)
	if err != nil {
		return nil, fmt.Errorf("quote failed: %w", err)
	}
	amountOutMin := applySlippage(expectedOut, slippageBps)

	from := crypto.PubkeyToAddress(key.PublicKey)

	if err := s.ensureAllowance(ctx, client, key, chainID, fromToken, amount); err != nil {
		return nil, fmt.Errorf("approve token: %w", err)
	}

	deadline := big.NewInt(time.Now().Add(20 * time.Minute).Unix())
	tx, err := client.Swap(ctx, family, path, amount, amountOutMin, from, deadline)
	if err != nil {
		return nil, fmt.Errorf("submit swap: %w", err)
	}

	result := &SwapResult{
		TxHash:       tx.Hash().Hex(),
		Status:       "pending",
		FromToken:    fromToken,
		ToToken:      toToken,
		AmountIn:     amountIn,
		AmountOut:    expectedOut.String(),
		AmountOutMin: amountOutMin.String(),
		DEX:          "tigerswap",
		Route:        []string{fromToken, toToken},
	}

	start := time.Now()
	receipt, waitErr := waitForReceipt(ctx, client.RPC(), tx.Hash(), 120*time.Second)
	result.ConfirmationTime = time.Since(start)

	if waitErr != nil {
		return result, nil
	}

	gasPrice, gasErr := client.RPC().SuggestGasPrice(ctx)
	if gasErr != nil {
		gasPrice = big.NewInt(0)
	}

	result.Status = "confirmed"
	result.GasUsed = receipt.GasUsed
	result.GasFee = weiToDecimal(new(big.Int).Mul(big.NewInt(int64(receipt.GasUsed)), gasPrice), 18)
	result.BlockNumber = receipt.BlockNumber.Uint64()

	return result, nil
}

// BuildSwap quotes the best TigerSwap family and returns an unsigned router
// call. The transaction is signed by the caller's wallet, so the server never
// needs a key for this path.
func (s *SwapExecutor) BuildSwap(ctx context.Context, chainID int64, fromToken, toToken, amountIn, recipient string, slippageBps uint32) (string, string, string, uint64, error) {
	amount, ok := new(big.Int).SetString(amountIn, 10)
	if !ok || amount.Sign() <= 0 {
		return "", "", "", 0, fmt.Errorf("invalid amount %q", amountIn)
	}
	if slippageBps > 5_000 {
		return "", "", "", 0, errors.New("slippage above 50% is not permitted")
	}
	if !common.IsHexAddress(fromToken) || !common.IsHexAddress(toToken) {
		return "", "", "", 0, errors.New("invalid token address")
	}
	if !common.IsHexAddress(recipient) {
		return "", "", "", 0, errors.New("invalid recipient address")
	}

	quote, err := s.GetSwapQuote(ctx, chainID, fromToken, toToken, amountIn)
	if err != nil {
		return "", "", "", 0, err
	}
	best, ok := new(big.Int).SetString(quote.AmountOut, 10)
	if !ok {
		return "", "", "", 0, errors.New("quote produced an invalid amount")
	}

	family, err := familyFromName(quote.BestSource)
	if err != nil {
		return "", "", "", 0, err
	}

	client, err := s.newClient(chainID, s.signer)
	if err != nil {
		return "", "", "", 0, err
	}
	defer client.Close()

	path := []common.Address{common.HexToAddress(fromToken), common.HexToAddress(toToken)}
	deadline := big.NewInt(time.Now().Add(20 * time.Minute).Unix())
	router, data, err := client.BuildSwapCalldata(family, path, amount, applySlippage(best, slippageBps), common.HexToAddress(recipient), deadline)
	if err != nil {
		return "", "", "", 0, err
	}

	gasLimit, err := client.RPC().EstimateGas(ctx, ethereum.CallMsg{
		From: common.HexToAddress(recipient),
		To:   &router,
		Data: data,
	})
	if err != nil {
		return "", "", "", 0, fmt.Errorf("estimate gas: %w", err)
	}

	return router.Hex(), "0x" + hex.EncodeToString(data), "0", gasLimit, nil
}

// ensureAllowance approves the TigerSwap router if the current allowance is low.
func (s *SwapExecutor) ensureAllowance(ctx context.Context, client *tigerswap.Client, key *ecdsa.PrivateKey, chainID int64, token string, amount *big.Int) error {
	deployment, ok := s.deployments[chainID]
	if !ok {
		return fmt.Errorf("TigerSwap is not deployed on chain %d", chainID)
	}

	tokenAddr := common.HexToAddress(token)
	owner := crypto.PubkeyToAddress(key.PublicKey)

	allowance, err := readAllowance(ctx, client.RPC(), tokenAddr, owner, deployment.Router)
	if err == nil && allowance.Cmp(amount) >= 0 {
		return nil
	}

	approveABI, err := abi.JSON(strings.NewReader(erc20ApproveABI))
	if err != nil {
		return err
	}

	maxApproval := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))
	data, err := approveABI.Pack("approve", deployment.Router, maxApproval)
	if err != nil {
		return err
	}

	_, err = signAndSend(ctx, client.RPC(), key, chainID, tokenAddr, big.NewInt(0), data)
	return err
}

var erc20AllowanceABI = mustParseABI(`[{"inputs":[{"internalType":"address","name":"owner","type":"address"},{"internalType":"address","name":"spender","type":"address"}],"name":"allowance","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"}]`)

func mustParseABI(raw string) abi.ABI {
	parsed, err := abi.JSON(strings.NewReader(raw))
	if err != nil {
		panic(fmt.Sprintf("invalid embedded ABI: %v", err))
	}
	return parsed
}

func readAllowance(ctx context.Context, client *ethclient.Client, token, owner, spender common.Address) (*big.Int, error) {
	data, err := erc20AllowanceABI.Pack("allowance", owner, spender)
	if err != nil {
		return nil, err
	}

	out, err := client.CallContract(ctx, ethereum.CallMsg{To: &token, Data: data}, nil)
	if err != nil {
		return nil, err
	}

	values, err := erc20AllowanceABI.Unpack("allowance", out)
	if err != nil || len(values) == 0 {
		return nil, fmt.Errorf("unpack allowance: %w", err)
	}

	allowance, ok := values[0].(*big.Int)
	if !ok {
		return nil, errors.New("unexpected allowance type")
	}
	return allowance, nil
}

// resolveSignerKey prefers a request-supplied key and falls back to the
// process-wide signer configured in the environment.
func resolveSignerKey(privateKey string, fallback *ecdsa.PrivateKey) (*ecdsa.PrivateKey, error) {
	if privateKey != "" {
		key, err := crypto.HexToECDSA(strings.TrimPrefix(privateKey, "0x"))
		if err != nil {
			return nil, fmt.Errorf("invalid private key: %w", err)
		}
		return key, nil
	}
	if fallback != nil {
		return fallback, nil
	}
	return nil, errors.New("no signer key available")
}

// GetSwapQuoteHandler serves a quote from TigerSwap pools.
func (s *SwapExecutor) GetSwapQuoteHandler(c *gin.Context) {
	var req struct {
		ChainID   int64  `json:"chain_id" binding:"required"`
		FromToken string `json:"from_token" binding:"required"`
		ToToken   string `json:"to_token" binding:"required"`
		Amount    string `json:"amount" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	quote, err := s.GetSwapQuote(c.Request.Context(), req.ChainID, req.FromToken, req.ToToken, req.Amount)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, quote)
}

// ExecuteSwapHandler broadcasts a client-signed swap against the TigerSwap
// router. The server never accepts a private key.
func (s *SwapExecutor) ExecuteSwapHandler(c *gin.Context) {
	var req struct {
		ChainID   int64  `json:"chain_id" binding:"required"`
		SignedTx  string `json:"signed_tx" binding:"required"`
		FromToken string `json:"from_token"`
		ToToken   string `json:"to_token"`
		Amount    string `json:"amount"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	txHash, err := s.BroadcastTransaction(c.Request.Context(), req.ChainID, req.SignedTx)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "tx_hash": txHash, "status": "submitted"})
}

// GetSupportedDEXHandler reports the TigerSwap families available on a chain.
func (s *SwapExecutor) GetSupportedDEXHandler(c *gin.Context) {
	chainID, err := parseInt64(c.Param("chain_id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid chain_id"})
		return
	}

	if _, ok := s.deployments[chainID]; !ok {
		c.JSON(404, gin.H{"error": fmt.Sprintf("TigerSwap is not deployed on chain %d", chainID)})
		return
	}

	c.JSON(200, gin.H{
		"chain_id": chainID,
		"dex_list": s.SupportedFamilies(),
	})
}

// LoadExecutorSigner reads the TigerSwap signer from the environment.
func LoadExecutorSigner() (*ecdsa.PrivateKey, error) {
	raw := os.Getenv("TIGERSWAP_SIGNER_KEY")
	if raw == "" {
		return nil, errors.New("TIGERSWAP_SIGNER_KEY is not configured")
	}
	return crypto.HexToECDSA(strings.TrimPrefix(raw, "0x"))
}
