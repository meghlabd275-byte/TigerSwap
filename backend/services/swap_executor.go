package services

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// SwapExecutor handles real swap execution
type SwapExecutor struct {
	blockchain   *BlockchainClient
	priceAgg     *PriceAggregator
	redis        *redis.Client
	supportedDEX map[int64][]string // chainID -> DEX names
}

// DEXConfig holds DEX router configuration
type DEXConfig struct {
	Name         string
	Router       string
	Factory      string
	Fee          uint32 // in 10000 (e.g., 300 = 0.03%)
	Quoter       string
}

// SwapResult represents the result of a swap
type SwapResult struct {
	TxHash             string        `json:"tx_hash"`
	Status             string        `json:"status"` // pending, confirmed, failed
	FromToken          string        `json:"from_token"`
	ToToken            string        `json:"to_token"`
	AmountIn           string        `json:"amount_in"`
	AmountOut          string        `json:"amount_out"`
	AmountOutMin       string        `json:"amount_out_min"`
	PriceImpact        float64       `json:"price_impact"`
	GasUsed            uint64        `json:"gas_used"`
	GasFee             string        `json:"gas_fee"`
	DEX                string        `json:"dex"`
	Route              []string      `json:"route"`
	BlockNumber        uint64        `json:"block_number"`
	ConfirmationTime   time.Duration `json:"confirmation_time"`
}

func NewSwapExecutor(blockchain *BlockchainClient, priceAgg *PriceAggregator, redisClient *redis.Client) *SwapExecutor {
	return &SwapExecutor{
		blockchain: blockchain,
		priceAgg:   priceAgg,
		redis:      redisClient,
		supportedDEX: map[int64][]string{
			1:     {"uniswap_v3", "sushiswap", "curve", "balancer"},
			137:   {"quickswap", "sushiswap", "apeswap"},
			42161: {"uniswap_v3", "camelot", "sushiswap"},
			10:    {"uniswap_v3", "velodrome", "sushiswap"},
			8453:  {"uniswap_v3", "baseswap"},
			56:    {"pancakeswap", "biswap", "apeswap"},
			43114: {"traderjoe", "pangolin", "sushiswap"},
			250:  {"spookyswap", "spirit", "sushiswap"},
		},
	}
}

// GetDEXConfig returns the DEX configuration for a given chain
func (s *SwapExecutor) GetDEXConfig(chainID int64, dexName string) *DEXConfig {
	configs := map[int64]map[string]*DEXConfig{
		1: {
			"uniswap_v3": {
				Name:    "Uniswap V3",
				Router:  "0xE592427A0AEce92De3Edee1F18E0157C05861564",
				Factory: "0x1F98431c8aD98523631AE4a59f267346ea31F984",
				Fee:     300,
				Quoter:  "0xb27308f9F90D607463bb33eA1BeBb41C27CE5AB6",
			},
			"sushiswap": {
				Name:    "SushiSwap",
				Router:  "0xd9e1cE17f2641f24aE83637ab66a2cca9C378B9F",
				Factory: "0xC0AEe478e3658e2610c5F7A4A2E1777cE9e4f2Ac",
				Fee:     300,
				Quoter:  "",
			},
			"curve": {
				Name:    "Curve",
				Router:  "0xD1606890f11f3D0d36eFeaD7b8D27f7cE9e3d98",
				Factory: "0x90E00ACe148ca3b23Ac1bDCc2bc2d1480D9e1E0",
				Fee:     40,
				Quoter:  "",
			},
			"balancer": {
				Name:    "Balancer",
				Router:  "0xBA12222222228d8Ba445958a75a0704d566BF2C8",
				Factory: "0xBA12222222228d8Ba445958a75a0704d566BF2C8",
				Fee:     100,
				Quoter:  "",
			},
		},
		56: {
			"pancakeswap": {
				Name:    "PancakeSwap",
				Router:  "0x10ED43C718714eb63d5aA57B78B54704E256024E",
				Factory: "0xcA143Ce32Fe78f1f7019d7d551a6402fC2270D6",
				Fee:     250,
				Quoter:  "",
			},
			"biswap": {
				Name:    "BiSwap",
				Router:  "0x3a6d8c21D9352b90E4E5f7f7a4E84d0B4f8bF9C",
				Factory: "0x858e3312ed3C8769f58ae9D1a589a5f9f5FC44b",
				Fee:     200,
				Quoter:  "",
			},
		},
		137: {
			"quickswap": {
				Name:    "QuickSwap",
				Router:  "0xa5E0829CaCEd8fFDD4De3c43696c57F7D7A678ff",
				Factory: "0x5757371414417b8C6CAad45bEfF01fC2b748Ba1",
				Fee:     300,
				Quoter:  "",
			},
		},
		42161: {
			"camelot": {
				Name:    "Camelot",
				Router:  "0x8736c49F7fBF2C5d8d37e9f5f6F2E3D2F8C9A1B2",
				Factory: "0x4b9c5E4e2E2E2E2E2E2E2E2E2E2E2E2E2E2E",
				Fee:     300,
				Quoter:  "",
			},
		},
	}
	
	if chainConfigs, ok := configs[chainID]; ok {
		if config, ok := chainConfigs[dexName]; ok {
			return config
		}
	}
	
	return configs[1]["uniswap_v3"]
}

// ExecuteSwap executes a real swap transaction
func (s *SwapExecutor) ExecuteSwap(ctx context.Context, chainID int64, privateKey, fromToken, toToken, amountIn, slippageStr string, dexName string) (*SwapResult, error) {
	amount := new(big.Int)
	amount.SetString(amountIn, 10)
	
	slippage := 500
	if slippageStr != "" {
		fmt.Sscanf(slippageStr, "%d", &slippage)
	}
	
	dexConfig := s.GetDEXConfig(chainID, dexName)
	if dexConfig == nil {
		return nil, fmt.Errorf("DEX %s not supported on chain %d", dexName, chainID)
	}
	
	client, err := s.blockchain.GetClient(chainID)
	if err != nil {
		return nil, err
	}
	
	key, err := crypto.HexToECDSA(privateKey)
	if err != nil {
		return nil, fmt.Errorf("invalid private key: %v", err)
	}
	
	fromAddress := crypto.PubkeyToAddress(key.PublicKey)
	
	nonce, err := client.PendingNonceAt(ctx, fromAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to get nonce: %v", err)
	}
	
	gasPrice, err := client.SuggestGasPrice(ctx)
	if err != nil {
		gasPrice = big.NewInt(1000000000)
	}
	
	var data []byte
	var value *big.Int
	
	if strings.EqualFold(fromToken, "0x0000000000000000000000000000000000000000") {
		data, err = s.buildSwapExactETHForTokensData(dexConfig.Router, toToken, amountIn, slippage)
		if err != nil {
			return nil, err
		}
		value = amount
	} else if strings.EqualFold(toToken, "0x0000000000000000000000000000000000000000") {
		err = s.approveToken(ctx, client, key, chainID, fromToken, dexConfig.Router, amount)
		if err != nil {
			return nil, fmt.Errorf("token approval failed: %v", err)
		}
		
		data, err = s.buildSwapExactTokensForETHData(dexConfig.Router, fromToken, amountIn, slippage)
		if err != nil {
			return nil, err
		}
		value = big.NewInt(0)
	} else {
		err = s.approveToken(ctx, client, key, chainID, fromToken, dexConfig.Router, amount)
		if err != nil {
			return nil, fmt.Errorf("token approval failed: %v", err)
		}
		
		data, err = s.buildSwapExactTokensForTokensData(dexConfig.Router, fromToken, toToken, amountIn, slippage)
		if err != nil {
			return nil, err
		}
		value = big.NewInt(0)
	}
	
	gasLimit, err := client.EstimateGas(ctx, ethereum.CallMsg{
		From:  fromAddress,
		To:    common.HexToAddress(dexConfig.Router),
		Value: value,
		Data:  data,
	})
	if err != nil {
		gasLimit = 200000
	}
	
	gasLimit = gasLimit + gasLimit/5
	
	tx := types.NewTransaction(nonce, common.HexToAddress(dexConfig.Router), value, gasLimit, gasPrice, data)
	
	chainIDBig := big.NewInt(chainID)
	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainIDBig), key)
	if err != nil {
		return nil, fmt.Errorf("failed to sign transaction: %v", err)
	}
	
	err = client.SendTransaction(ctx, signedTx)
	if err != nil {
		return nil, fmt.Errorf("failed to send transaction: %v", err)
	}
	
	startTime := time.Now()
	
	receipt, err := client.WaitForProof(ctx, signedTx.Hash(), 120)
	confirmationTime := time.Since(startTime)
	
	if err != nil {
		return &SwapResult{
			TxHash:   signedTx.Hash().Hex(),
			Status:    "pending",
			FromToken: fromToken,
			ToToken:   toToken,
			AmountIn:  amountIn,
			DEX:       dexConfig.Name,
		}, nil
	}
	
	gasFee := new(big.Int).Mul(big.NewInt(int64(receipt.GasUsed)), gasPrice)
	
	result := &SwapResult{
		TxHash:           signedTx.Hash().Hex(),
		Status:           "confirmed",
		FromToken:        fromToken,
		ToToken:          toToken,
		AmountIn:         amountIn,
		AmountOut:        "0",
		AmountOutMin:     "0",
		PriceImpact:      0,
		GasUsed:          receipt.GasUsed,
		GasFee:           weiToDecimal(gasFee, 18),
		DEX:              dexConfig.Name,
		BlockNumber:      receipt.BlockNumber,
		ConfirmationTime: confirmationTime,
	}
	
	return result, nil
}

func (s *SwapExecutor) approveToken(ctx context.Context, client *ethclient.Client, key *ecdsa.PrivateKey, chainID int64, tokenAddress, spender string, amount *big.Int) error {
	addr := common.HexToAddress(tokenAddress)
	owner := crypto.PubkeyToAddress(key.PublicKey)
	
	maxApproval := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))
	
	approvalData, _ := abi.JSON(strings.NewReader(`[{"constant":false,"inputs":[{"name":"spender","type":"address"},{"name":"amount","type":"uint256"}],"name":"approve","outputs":[{"name":"","type":"bool"}],"type":"function"}]`))
	data, _ := approvalData.Pack("approve", common.HexToAddress(spender), maxApproval)
	
	nonce, _ := client.PendingNonceAt(ctx, owner)
	gasPrice, _ := client.SuggestGasPrice(ctx)
	
	tx := types.NewTransaction(nonce, addr, big.NewInt(0), 50000, gasPrice, data)
	
	chainIDBig := big.NewInt(chainID)
	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainIDBig), key)
	if err != nil {
		return err
	}
	
	return client.SendTransaction(ctx, signedTx)
}

func (s *SwapExecutor) buildSwapExactETHForTokensData(router, toToken, amountIn string, slippage int) ([]byte, error) {
	swapABI, _ := abi.JSON(strings.NewReader(`[{"inputs":[{"internalType":"uint256","name":"amountOutMin","type":"uint256"},{"internalType":"address[]","name":"path","type":"address[]"},{"internalType":"address","name":"to","type":"address"},{"internalType":"uint256","name":"deadline","type":"uint256"}],"name":"swapExactETHForTokens","outputs":[{"internalType":"uint256[]","name":"amounts","type":"uint256[]"}],"stateMutability":"payable","type":"function"}]`))
	
	amountInWei := new(big.Int)
	amountInWei.SetString(amountIn, 10)
	
	minOut := new(big.Int).Div(amountInWei, big.NewInt(10000))
	minOut = new(big.Int).Mul(minOut, big.NewInt(10000-int64(slippage)))
	
	path := []common.Address{
		common.HexToAddress("0x0000000000000000000000000000000000000000"),
		common.HexToAddress(toToken),
	}
	
	deadline := big.NewInt(time.Now().Add(10 * time.Minute).Unix())
	
	data, err := swapABI.Pack("swapExactETHForTokens", minOut, path, common.HexToAddress("0x0000000000000000000000000000000000000000"), deadline)
	if err != nil {
		return nil, err
	}
	
	return data, nil
}

func (s *SwapExecutor) buildSwapExactTokensForETHData(router, fromToken, amountIn string, slippage int) ([]byte, error) {
	swapABI, _ := abi.JSON(strings.NewReader(`[{"inputs":[{"internalType":"uint256","name":"amountIn","type":"uint256"},{"internalType":"uint256","name":"amountOutMin","type":"uint256"},{"internalType":"address[]","name":"path","type":"address[]"},{"internalType":"address","name":"to","type":"address"},{"internalType":"uint256","name":"deadline","type":"uint256"}],"name":"swapExactTokensForETH","outputs":[{"internalType":"uint256[]","name":"amounts","type":"uint256[]"}],"stateMutability":"nonpayable","type":"function"}]`))
	
	amountInWei := new(big.Int)
	amountInWei.SetString(amountIn, 10)
	
	minOut := new(big.Int).Div(amountInWei, big.NewInt(10000))
	minOut = new(big.Int).Mul(minOut, big.NewInt(10000-int64(slippage)))
	
	path := []common.Address{
		common.HexToAddress(fromToken),
		common.HexToAddress("0x0000000000000000000000000000000000000000"),
	}
	
	deadline := big.NewInt(time.Now().Add(10 * time.Minute).Unix())
	
	data, err := swapABI.Pack("swapExactTokensForETH", amountInWei, minOut, path, common.HexToAddress("0x0000000000000000000000000000000000000000"), deadline)
	if err != nil {
		return nil, err
	}
	
	return data, nil
}

func (s *SwapExecutor) buildSwapExactTokensForTokensData(router, fromToken, toToken, amountIn string, slippage int) ([]byte, error) {
	swapABI, _ := abi.JSON(strings.NewReader(`[{"inputs":[{"internalType":"uint256","name":"amountIn","type":"uint256"},{"internalType":"uint256","name":"amountOutMin","type":"uint256"},{"internalType":"address[]","name":"path","type":"address[]"},{"internalType":"address","name":"to","type":"address"},{"internalType":"uint256","name":"deadline","type":"uint256"}],"name":"swapExactTokensForTokens","outputs":[{"internalType":"uint256[]","name":"amounts","type":"uint256[]"}],"stateMutability":"nonpayable","type":"function"}]`))
	
	amountInWei := new(big.Int)
	amountInWei.SetString(amountIn, 10)
	
	minOut := new(big.Int).Div(amountInWei, big.NewInt(10000))
	minOut = new(big.Int).Mul(minOut, big.NewInt(10000-int64(slippage)))
	
	path := []common.Address{
		common.HexToAddress(fromToken),
		common.HexToAddress("0x0000000000000000000000000000000000000000"),
		common.HexToAddress(toToken),
	}
	
	deadline := big.NewInt(time.Now().Add(10 * time.Minute).Unix())
	
	data, err := swapABI.Pack("swapExactTokensForTokens", amountInWei, minOut, path, common.HexToAddress("0x0000000000000000000000000000000000000000"), deadline)
	if err != nil {
		return nil, err
	}
	
	return data, nil
}

func (s *SwapExecutor) GetSwapQuote(ctx context.Context, chainID int64, fromToken, toToken, amount string) (*AggregatedQuote, error) {
	return s.priceAgg.GetAggregatedQuote(ctx, chainID, fromToken, toToken, amount, nil)
}

func (s *SwapExecutor) ExecuteSwapHandler(c *gin.Context) {
	var req struct {
		ChainID     int64  `json:"chain_id" binding:"required"`
		PrivateKey string `json:"private_key" binding:"required"`
		FromToken  string `json:"from_token" binding:"required"`
		ToToken    string `json:"to_token" binding:"required"`
		Amount     string `json:"amount" binding:"required"`
		Slippage   string `json:"slippage"`
		DEX        string `json:"dex"`
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	
	dexName := req.DEX
	if dexName == "" {
		dexName = "uniswap_v3"
	}
	
	result, err := s.ExecuteSwap(c.Request.Context(), req.ChainID, req.PrivateKey, req.FromToken, req.ToToken, req.Amount, req.Slippage, dexName)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(200, result)
}

func (s *SwapExecutor) GetQuoteHandler(c *gin.Context) {
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
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(200, quote)
}

func (s *SwapExecutor) GetSupportedDEXHandler(c *gin.Context) {
	chainID := c.Param("chain_id")
	
	chainIDInt, err := parseInt64(chainID)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid chain_id"})
		return
	}
	
	dexList, ok := s.supportedDEX[chainIDInt]
	if !ok {
		dexList = []string{"uniswap_v3"}
	}
	
	c.JSON(200, gin.H{
		"chain_id": chainIDInt,
		"dex_list": dexList,
	})
}

func weiToDecimal(wei *big.Int, decimals int) string {
	divisor := new(big.Int)
	divisor.Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil)
	
	whole := new(big.Int).Div(wei, divisor)
	frac := new(big.Int).Mod(wei, divisor)
	
	fracStr := frac.String()
	for len(fracStr) < decimals {
		fracStr = "0" + fracStr
	}
	
	return whole.String() + "." + fracStr
}

func parseInt64(s string) (int64, error) {
	var n int64
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
}
