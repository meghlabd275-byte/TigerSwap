package services

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"net/http"
	"os"
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
)

// BlockchainClient handles real blockchain interactions
type BlockchainClient struct {
	clients    map[int64]*ethclient.Client
	rpcURLs    map[int64]string
	chainCache sync.Map // chainID -> chain config
	mu         sync.RWMutex
}

// ChainConfig represents blockchain configuration
type ChainConfig struct {
	ChainID     int64  `json:"chain_id"`
	Name        string `json:"name"`
	Symbol      string `json:"symbol"`
	RPCURL      string `json:"rpc_url"`
	ExplorerURL string `json:"explorer_url"`
	BlockTime   int    `json:"block_time"` // seconds
	NativeToken string `json:"native_token"`
}

// TokenBalance represents token balance
type TokenBalance struct {
	Address    string   `json:"address"`
	Symbol     string   `json:"symbol"`
	Decimals   int      `json:"decimals"`
	Balance    string   `json:"balance"`
	BalanceWei *big.Int `json:"balance_wei"`
	USDValue   float64  `json:"usd_value"`
}

// TransactionData represents transaction details
type TransactionData struct {
	Hash           string          `json:"hash"`
	From           string          `json:"from"`
	To             string          `json:"to"`
	Value          string          `json:"value"`
	ValueWei       *big.Int        `json:"value_wei"`
	GasUsed        uint64          `json:"gas_used"`
	GasPrice       *big.Int        `json:"gas_price"`
	GasLimit       uint64          `json:"gas_limit"`
	Nonce          uint64          `json:"nonce"`
	BlockNumber    uint64          `json:"block_number"`
	Status         string          `json:"status"` // pending, confirmed, failed
	Timestamp      int64           `json:"timestamp"`
	TokenTransfers []TokenTransfer `json:"token_transfers"`
}

// TokenTransfer represents an ERC20 transfer
type TokenTransfer struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Value  string `json:"value"`
	Token  string `json:"token"`
	Symbol string `json:"symbol"`
}

// BlockData represents block information
type BlockData struct {
	Number       uint64   `json:"number"`
	Hash         string   `json:"hash"`
	ParentHash   string   `json:"parent_hash"`
	Timestamp    uint64   `json:"timestamp"`
	Transactions []string `json:"transactions"`
	GasUsed      uint64   `json:"gas_used"`
	GasLimit     uint64   `json:"gas_limit"`
	Miner        string   `json:"miner"`
}

// rpcFromEnv resolves a chain's RPC endpoint from operator configuration.
//
// TigerSwap runs its own nodes, so no third-party endpoint is baked in. An empty
// result means the operator has not configured a node for that chain.
func rpcFromEnv(chainID int64) string {
	if url := os.Getenv(fmt.Sprintf("TIGERSWAP_RPC_%d", chainID)); url != "" {
		return url
	}
	return os.Getenv(fmt.Sprintf("TIGERSWAP_RPC_URL_%d", chainID))
}

// NewBlockchainClient returns a client with no RPC endpoints preconfigured.
//
// TigerSwap runs its own nodes, so endpoints are supplied by the operator via
// TIGERSWAP_RPC_<chainID> or SetRPCURL rather than defaulting to a third-party
// public endpoint.
func NewBlockchainClient() *BlockchainClient {
	return &BlockchainClient{
		clients: make(map[int64]*ethclient.Client),
		rpcURLs: make(map[int64]string),
	}
}

// RPCURLForChain resolves the RPC endpoint for a chain. TigerSwap only dials
// its own nodes: TIGERSWAP_RPC_<chainID> (or TIGERSWAP_RPC_URL_<chainID>), or an
// endpoint registered with SetRPCURL. There is no third-party fallback.
func (b *BlockchainClient) RPCURLForChain(chainID int64) (string, error) {
	if url := rpcFromEnv(chainID); url != "" {
		return url, nil
	}

	b.mu.RLock()
	url, ok := b.rpcURLs[chainID]
	b.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("RPC URL not configured for chain %d", chainID)
	}
	return url, nil
}

// SetRPCURL registers an RPC endpoint for a chain and drops any cached client.
func (b *BlockchainClient) SetRPCURL(chainID int64, url string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if existing, ok := b.clients[chainID]; ok {
		existing.Close()
		delete(b.clients, chainID)
	}
	b.rpcURLs[chainID] = url
}

// GetClient returns or creates an ethclient for the given chain
func (b *BlockchainClient) GetClient(chainID int64) (*ethclient.Client, error) {
	b.mu.RLock()
	client, exists := b.clients[chainID]
	b.mu.RUnlock()

	if exists {
		return client, nil
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	// Double check after acquiring write lock
	if client, exists := b.clients[chainID]; exists {
		return client, nil
	}

	b.mu.Unlock()
	rpcURL, err := b.RPCURLForChain(chainID)
	b.mu.Lock()
	if err != nil {
		return nil, err
	}

	dialed, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RPC: %v", err)
	}

	b.clients[chainID] = dialed
	return dialed, nil
}

// GetNativeBalance returns the native token balance for an address
func (b *BlockchainClient) GetNativeBalance(ctx context.Context, chainID int64, address string) (*TokenBalance, error) {
	client, err := b.GetClient(chainID)
	if err != nil {
		return nil, err
	}

	addr := common.HexToAddress(address)
	balance, err := client.BalanceAt(ctx, addr, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get balance: %v", err)
	}

	chainConfig := b.getChainConfig(chainID)
	balanceStr := weiToDecimal(balance, 18)

	return &TokenBalance{
		Address:    address,
		Symbol:     chainConfig.Symbol,
		Decimals:   18,
		Balance:    balanceStr,
		BalanceWei: balance,
		USDValue:   0, // Would calculate with price
	}, nil
}

// GetERC20Balance returns the ERC20 token balance for an address
func (b *BlockchainClient) GetERC20Balance(ctx context.Context, chainID int64, ownerAddress, tokenAddress string) (*TokenBalance, error) {
	client, err := b.GetClient(chainID)
	if err != nil {
		return nil, err
	}

	// ERC20 balanceOf ABI
	balanceOfABI, _ := abi.JSON(strings.NewReader(`[{"constant":true,"inputs":[{"name":"owner","type":"address"}],"name":"balanceOf","outputs":[{"name":"","type":"uint256"}],"type":"function"}]`))
	decimalsABI, _ := abi.JSON(strings.NewReader(`[{"constant":true,"inputs":[],"name":"decimals","outputs":[{"name":"","type":"uint8"}],"type":"function"}]`))
	symbolABI, _ := abi.JSON(strings.NewReader(`[{"constant":true,"inputs":[],"name":"symbol","outputs":[{"name":"","type":"string"}],"type":"function"}]`))

	tokenAddr := common.HexToAddress(tokenAddress)
	ownerAddr := common.HexToAddress(ownerAddress)

	// Get balance
	data, err := balanceOfABI.Pack("balanceOf", ownerAddr)
	if err != nil {
		return nil, err
	}

	result, err := client.CallContract(ctx, ethereum.CallMsg{
		To:   &tokenAddr,
		Data: data,
	}, nil)
	if err != nil {
		return nil, err
	}

	balance := new(big.Int)
	balance.SetBytes(result)

	// Get decimals
	decData, _ := decimalsABI.Pack("decimals")
	decResult, err := client.CallContract(ctx, ethereum.CallMsg{
		To:   &tokenAddr,
		Data: decData,
	}, nil)
	if err != nil {
		return nil, err
	}

	decimalsVals, err := decimalsABI.Unpack("decimals", decResult)
	if err != nil || len(decimalsVals) != 1 {
		return nil, fmt.Errorf("decode decimals: %w", err)
	}
	decimals, ok := decimalsVals[0].(uint8)
	if !ok {
		return nil, fmt.Errorf("unexpected decimals type %T", decimalsVals[0])
	}

	// Get symbol
	symData, _ := symbolABI.Pack("symbol")
	symResult, err := client.CallContract(ctx, ethereum.CallMsg{
		To:   &tokenAddr,
		Data: symData,
	}, nil)
	if err != nil {
		return nil, err
	}

	symbolVals, err := symbolABI.Unpack("symbol", symResult)
	if err != nil || len(symbolVals) != 1 {
		return nil, fmt.Errorf("decode symbol: %w", err)
	}
	symbol, ok := symbolVals[0].(string)
	if !ok {
		return nil, fmt.Errorf("unexpected symbol type %T", symbolVals[0])
	}

	return &TokenBalance{
		Address:    ownerAddress,
		Symbol:     symbol,
		Decimals:   int(decimals),
		Balance:    weiToDecimal(balance, int(decimals)),
		BalanceWei: balance,
		USDValue:   0,
	}, nil
}

// GetAllTokenBalances returns all token balances for an address
func (b *BlockchainClient) GetAllTokenBalances(ctx context.Context, chainID int64, address string, tokenAddresses []string) ([]*TokenBalance, error) {
	var balances []*TokenBalance
	var wg sync.WaitGroup
	var mu sync.Mutex

	// Get native balance first
	wg.Add(1)
	go func() {
		defer wg.Done()
		bal, err := b.GetNativeBalance(ctx, chainID, address)
		mu.Lock()
		defer mu.Unlock()
		if err == nil {
			balances = append(balances, bal)
		}
	}()

	// Get ERC20 balances
	for _, tokenAddr := range tokenAddresses {
		wg.Add(1)
		go func(token string) {
			defer wg.Done()
			bal, err := b.GetERC20Balance(ctx, chainID, address, token)
			mu.Lock()
			defer mu.Unlock()
			if err == nil && bal.BalanceWei.Sign() > 0 {
				balances = append(balances, bal)
			}
		}(tokenAddr)
	}

	wg.Wait()

	return balances, nil
}

// SendTransaction sends a transaction on the given chain
func (b *BlockchainClient) SendTransaction(ctx context.Context, chainID int64, privateKey string, toAddress string, amount *big.Int, tokenAddress string, gasPrice *big.Int) (string, error) {
	client, err := b.GetClient(chainID)
	if err != nil {
		return "", err
	}

	// Parse private key
	key, err := crypto.HexToECDSA(privateKey)
	if err != nil {
		return "", fmt.Errorf("invalid private key: %v", err)
	}

	fromAddress := crypto.PubkeyToAddress(key.PublicKey)
	nonce, err := client.PendingNonceAt(ctx, fromAddress)
	if err != nil {
		return "", fmt.Errorf("failed to get nonce: %v", err)
	}

	// Get gas price if not provided
	if gasPrice == nil {
		gasPrice, err = client.SuggestGasPrice(ctx)
		if err != nil {
			gasPrice = big.NewInt(1000000000) // 1 gwei fallback
		}
	}

	gasLimit := uint64(21000) // Base transaction
	var data []byte
	var value *big.Int

	if tokenAddress != "" && tokenAddress != "0x0000000000000000000000000000000000000000" {
		// Token transfer
		gasLimit = 65000
		transferABI, _ := abi.JSON(strings.NewReader(`[{"constant":false,"inputs":[{"name":"to","type":"address"},{"name":"amount","type":"uint256"}],"name":"transfer","outputs":[{"name":"","type":"bool"}],"type":"function"}]`))

		data, _ = transferABI.Pack("transfer", common.HexToAddress(toAddress), amount)
		value = big.NewInt(0)
	} else {
		// Native ETH transfer
		value = amount
	}

	tx := types.NewTransaction(nonce, common.HexToAddress(toAddress), value, gasLimit, gasPrice, data)

	chainIDBig := big.NewInt(chainID)
	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainIDBig), key)
	if err != nil {
		return "", fmt.Errorf("failed to sign transaction: %v", err)
	}

	err = client.SendTransaction(ctx, signedTx)
	if err != nil {
		return "", fmt.Errorf("failed to send transaction: %v", err)
	}

	return signedTx.Hash().Hex(), nil
}

// GetTransactionReceipt returns the transaction receipt
func (b *BlockchainClient) GetTransactionReceipt(ctx context.Context, chainID int64, txHash string) (*TransactionData, error) {
	client, err := b.GetClient(chainID)
	if err != nil {
		return nil, err
	}

	hash := common.HexToHash(txHash)
	receipt, err := client.TransactionReceipt(ctx, hash)
	if err != nil {
		return nil, err
	}

	tx, _, err := client.TransactionByHash(ctx, hash)
	signer := types.LatestSignerForChainID(tx.ChainId())
	sender, err := types.Sender(signer, tx)
	if err != nil {
		return nil, fmt.Errorf("recover sender: %w", err)
	}
	if err != nil {
		return nil, err
	}

	block, err := client.BlockByNumber(ctx, receipt.BlockNumber)
	if err != nil {
		return nil, err
	}

	status := "confirmed"
	if receipt.Status == 0 {
		status = "failed"
	}

	return &TransactionData{
		Hash:        txHash,
		From:        sender.Hex(),
		To:          tx.To().Hex(),
		Value:       weiToDecimal(tx.Value(), 18),
		ValueWei:    tx.Value(),
		GasUsed:     receipt.GasUsed,
		GasPrice:    tx.GasPrice(),
		GasLimit:    tx.Gas(),
		Nonce:       tx.Nonce(),
		BlockNumber: receipt.BlockNumber.Uint64(),
		Status:      status,
		Timestamp:   int64(block.Time()),
	}, nil
}

// WaitForTransaction waits for a transaction to be confirmed
func (b *BlockchainClient) WaitForTransaction(ctx context.Context, chainID int64, txHash string, timeoutSeconds int) (*TransactionData, error) {
	client, err := b.GetClient(chainID)
	if err != nil {
		return nil, err
	}

	hash := common.HexToHash(txHash)

	// Wait for receipt
	deadline := time.Now().Add(time.Duration(timeoutSeconds) * time.Second)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		receipt, err := client.TransactionReceipt(ctx, hash)
		if err == nil && receipt != nil {
			return b.GetTransactionReceipt(ctx, chainID, txHash)
		}
		if err != nil && err != ethereum.NotFound {
			return nil, fmt.Errorf("poll receipt: %w", err)
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("transaction %s not confirmed within %ds", txHash, timeoutSeconds)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}

// GetBlock returns block information
func (b *BlockchainClient) GetBlock(ctx context.Context, chainID int64, blockNumber uint64) (*BlockData, error) {
	client, err := b.GetClient(chainID)
	if err != nil {
		return nil, err
	}

	block, err := client.BlockByNumber(ctx, big.NewInt(int64(blockNumber)))
	if err != nil {
		return nil, err
	}

	txHashes := block.Transactions()
	txs := make([]string, len(txHashes))
	for i, tx := range txHashes {
		txs[i] = tx.Hash().Hex()
	}

	return &BlockData{
		Number:       block.NumberU64(),
		Hash:         block.Hash().Hex(),
		ParentHash:   block.ParentHash().Hex(),
		Timestamp:    block.Time(),
		Transactions: txs,
		GasUsed:      block.GasUsed(),
		GasLimit:     block.GasLimit(),
		Miner:        block.Coinbase().Hex(),
	}, nil
}

// GetCurrentBlock returns the current block number
func (b *BlockchainClient) GetCurrentBlock(ctx context.Context, chainID int64) (uint64, error) {
	client, err := b.GetClient(chainID)
	if err != nil {
		return 0, err
	}

	header, err := client.HeaderByNumber(ctx, nil)
	if err != nil {
		return 0, err
	}

	return header.Number.Uint64(), nil
}

// EstimateGas estimates gas for a transaction
func (b *BlockchainClient) EstimateGas(ctx context.Context, chainID int64, from, to string, value *big.Int, data []byte) (uint64, error) {
	client, err := b.GetClient(chainID)
	if err != nil {
		return 0, err
	}

	toAddr := common.HexToAddress(to)
	msg := ethereum.CallMsg{
		From:  common.HexToAddress(from),
		To:    &toAddr,
		Value: value,
		Data:  data,
	}

	gas, err := client.EstimateGas(ctx, msg)
	if err != nil {
		return 0, err
	}

	// Add 20% buffer
	gas = gas + gas/5

	return gas, nil
}

// GetGasPrice returns the current gas price
func (b *BlockchainClient) GetGasPrice(ctx context.Context, chainID int64) (*big.Int, error) {
	client, err := b.GetClient(chainID)
	if err != nil {
		return nil, err
	}

	return client.SuggestGasPrice(ctx)
}

// getChainConfig returns static chain metadata. It deliberately carries no RPC
// endpoint: TigerSwap dials its own nodes, resolved by RPCURLForChain from
// operator configuration.
func (b *BlockchainClient) getChainConfig(chainID int64) *ChainConfig {
	configs := map[int64]*ChainConfig{
		1:     {ChainID: 1, Name: "Ethereum", Symbol: "ETH", ExplorerURL: "https://etherscan.io", BlockTime: 12},
		137:   {ChainID: 137, Name: "Polygon", Symbol: "MATIC", ExplorerURL: "https://polygonscan.com", BlockTime: 2},
		42161: {ChainID: 42161, Name: "Arbitrum One", Symbol: "ETH", ExplorerURL: "https://arbiscan.io", BlockTime: 1},
		10:    {ChainID: 10, Name: "Optimism", Symbol: "ETH", ExplorerURL: "https://optimistic.etherscan.io", BlockTime: 2},
		8453:  {ChainID: 8453, Name: "Base", Symbol: "ETH", ExplorerURL: "https://basescan.org", BlockTime: 2},
		56:    {ChainID: 56, Name: "BNB Smart Chain", Symbol: "BNB", ExplorerURL: "https://bscscan.com", BlockTime: 3},
		43114: {ChainID: 43114, Name: "Avalanche", Symbol: "AVAX", ExplorerURL: "https://snowtrace.io", BlockTime: 1},
	}

	if config, ok := configs[chainID]; ok {
		return config
	}

	return &ChainConfig{ChainID: chainID, Name: "Unknown", Symbol: "ETH", BlockTime: 12}
}

// API Handlers

func (b *BlockchainClient) GetBalanceHandler(c *gin.Context) {
	chainID := c.Param("chain_id")
	address := c.Query("address")

	chainIDInt, err := parseInt64(chainID)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid chain_id"})
		return
	}

	if !common.IsHexAddress(address) {
		c.JSON(400, gin.H{"error": "invalid address"})
		return
	}

	balance, err := b.GetNativeBalance(c.Request.Context(), chainIDInt, address)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, balance)
}

func (b *BlockchainClient) GetTokenBalanceHandler(c *gin.Context) {
	chainID := c.Param("chain_id")
	address := c.Query("address")
	token := c.Query("token")

	chainIDInt, err := parseInt64(chainID)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid chain_id"})
		return
	}

	if !common.IsHexAddress(address) || !common.IsHexAddress(token) {
		c.JSON(400, gin.H{"error": "invalid address or token"})
		return
	}

	balance, err := b.GetERC20Balance(c.Request.Context(), chainIDInt, address, token)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, balance)
}

// SendTransactionHandler broadcasts a client-signed transaction. No private key
// is accepted over HTTP: the client signs locally and posts the signed payload.
func (b *BlockchainClient) SendTransactionHandler(c *gin.Context) {
	var req struct {
		ChainID   int64  `json:"chain_id" binding:"required"`
		SignedTx  string `json:"signed_tx" binding:"required"`
		ToAddress string `json:"to_address"`
		Amount    string `json:"amount"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	client, err := b.GetClient(req.ChainID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	txHash, err := broadcastSignedTransaction(c.Request.Context(), client, req.SignedTx)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"tx_hash": txHash,
		"status":  "submitted",
	})
}

func (b *BlockchainClient) GetTransactionReceiptHandler(c *gin.Context) {
	chainID := c.Param("chain_id")
	txHash := c.Param("tx_hash")

	chainIDInt, err := parseInt64(chainID)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid chain_id"})
		return
	}

	receipt, err := b.GetTransactionReceipt(c.Request.Context(), chainIDInt, txHash)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, receipt)
}

func (b *BlockchainClient) GetGasPriceHandler(c *gin.Context) {
	chainID := c.Param("chain_id")

	chainIDInt, err := parseInt64(chainID)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid chain_id"})
		return
	}

	gasPrice, err := b.GetGasPrice(c.Request.Context(), chainIDInt)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"chain_id":       chainIDInt,
		"gas_price":      gasPrice.String(),
		"gas_price_gwei": weiToDecimal(gasPrice, 9),
	})
}

func (b *BlockchainClient) GetCurrentBlockHandler(c *gin.Context) {
	chainID := c.Param("chain_id")

	chainIDInt, err := parseInt64(chainID)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid chain_id"})
		return
	}

	block, err := b.GetCurrentBlock(c.Request.Context(), chainIDInt)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"chain_id":     chainIDInt,
		"block_number": block,
	})
}

// Helper functions

func weiToDecimal(wei *big.Int, decimals int) string {
	divisor := new(big.Int)
	divisor.Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil)

	whole := new(big.Int).Div(wei, divisor)
	frac := new(big.Int).Mod(wei, divisor)

	// Format with leading zeros based on decimals
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

func init() {
	// Import required packages in actual implementation
	_ = hex.EncodeToString
	_ = crypto.Keccak256Hash
}
