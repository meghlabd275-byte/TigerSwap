module github.com/tigerswap/dex-connectors

go 1.21

require (
	github.com/ethereum/go-ethereum v1.12.0
	github.com/shopspring/decimal v1.3.1
)

package dexconnectors

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

type DEXConfig struct {
	Name      string
	Router    string
	Factory   string
	ChainID   int
	RPC       string
	Subgraph  string
}

type BaseDEX struct {
	config DEXConfig
	client *ethclient.Client
	ctx    context.Context
	pools  map[string][]Pool
}

type Pool struct {
	Address   string
	Token0    common.Address
	Token1    common.Address
	Reserve0  *big.Int
	Reserve1  *big.Int
	Fee       int
	Liquidity *big.Float
}

type Token struct {
	Address  common.Address
	Symbol   string
	Decimals int
	Name     string
}

type Quote struct {
	TokenIn     Token
	TokenOut    Token
	AmountIn    *big.Int
	AmountOut   *big.Int
	PriceImpact float64
	GasEstimate uint64
	Path        []common.Address
	Pools       []string
	Protocol    string
}

type SwapResult struct {
	Success     bool
	TxHash      string
	TokenIn     Token
	TokenOut    Token
	AmountIn    *big.Int
	AmountOut   *big.Int
	GasUsed     uint64
	BlockNumber uint64
}

type SwapRequest struct {
	TokenIn      Token
	TokenOut     Token
	AmountIn     *big.Int
	AmountOutMin *big.Int
	Recipient    string
	Deadline    time.Time
	Path        []common.Address
	Pools       []string
	Referrer    string
}

func NewBaseDEX(config DEXConfig) (*BaseDEX, error) {
	client, err := ethclient.Dial(config.RPC)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", config.Name, err)
	}
	return &BaseDEX{
		config: config,
		client: client,
		ctx:    context.Background(),
		pools:  make(map[string][]Pool),
	}, nil
}

func (d *BaseDEX) GetName() string      { return d.config.Name }
func (d *BaseDEX) GetChainID() int      { return d.config.ChainID }
func (d *BaseDEX) GetRouter() string    { return d.config.Router }
func (d *BaseDEX) GetFactory() string   { return d.config.Factory }
func (d *BaseDEX) GetClient() *ethclient.Client { return d.client }

func (d *BaseDEX) GetPools(tokenA, tokenB string) ([]Pool, error) {
	return d.pools[tokenA+"-"+tokenB], nil
}

func (d *BaseDEX) GetQuote(tokenIn, tokenOut string, amountIn *big.Int) (*Quote, error) {
	return &Quote{
		TokenIn:     Token{Address: common.HexToAddress(tokenIn)},
		TokenOut:    Token{Address: common.HexToAddress(tokenOut)},
		AmountIn:    amountIn,
		AmountOut:   big.NewInt(0),
		PriceImpact: 0.5,
		GasEstimate: 150000,
		Path:        []common.Address{common.HexToAddress(tokenIn), common.HexToAddress(tokenOut)},
		Pools:       []string{},
		Protocol:    d.config.Name,
	}, nil
}

func (d *BaseDEX) ExecuteSwap(req SwapRequest) (*SwapResult, error) {
	return &SwapResult{
		Success:     true,
		TxHash:      "0x0",
		TokenIn:     req.TokenIn,
		TokenOut:    req.TokenOut,
		AmountIn:    req.AmountIn,
		AmountOut:   big.NewInt(0),
		GasUsed:     150000,
		BlockNumber: 0,
	}, nil
}