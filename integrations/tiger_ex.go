// TigerEX Integration Layer - Go Implementation
// Distributed, Production-Ready Backend Services
//
// Integrated Products:
// - TigerWallet (Multichain Web3 Wallet)
// - Tigerswap (Multichain DEX)
// - TigerSmartChain (EVM Blockchain with TGR & RUSD)
// - TigerEx (Centralized Exchange)
//
// Fee Collection:
// - Exchange trading fees (0.1%)
// - DEX swap fees (0.3%)
// - Bridge cross-chain fees (0.1%)
// - Wallet transaction fees (0.1%)

package main

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// ============================================================================
// Types & Interfaces
// ============================================================================

// Chain categories
type ChainCategory string

const (
	ChainCategoryEVM    ChainCategory = "evm"
	ChainCategorySolana  ChainCategory = "solana"
	ChainCategoryAptos  ChainCategory = "aptos"
	ChainCategorySui   ChainCategory = "sui"
	ChainCategoryTON   ChainCategory = "ton"
	ChainCategoryCosmos ChainCategory = "cosmos"
	ChainCategoryNear   ChainCategory = "near"
	ChainCategoryOther ChainCategory = "other"
)

// Chain status
type ChainStatus string

const (
	ChainStatusActive      ChainStatus = "active"
	ChainStatusInactive   ChainStatus = "inactive"
	ChainStatusPaused    ChainStatus = "paused"
	ChainStatusDeprecated ChainStatus = "deprecated"
)

// Chain configuration
type ChainConfig struct {
	ID             string       `json:"id"`
	Name           string       `json:"name"`
	Symbol         string       `json:"symbol"`
	Category       ChainCategory `json:"category"`
	Status         ChainStatus   `json:"status"`
	ChainID        int          `json:"chainId"`
	RPCURLs        []string     `json:"rpcUrls"`
	ExplorerURLs  []string    `json:"explorerUrls"`
	NativeCurrency NativeCurrency `json:"nativeCurrency"`
	BlockTime      float64     `json:"blockTime"`
	SupportsEIP1559 bool       `json:"supportsEIP1559"`
}

// Native currency
type NativeCurrency struct {
	Name     string `json:"name"`
	Symbol   string `json:"symbol"`
	Decimals int    `json:"decimals"`
}

// Token configuration
type TokenConfig struct {
	Address    string `json:"address"`
	ChainID    string `json:"chainId"`
	Symbol     string `json:"symbol"`
	Name       string `json:"name"`
	Decimals   int    `json:"decimals"`
	IsNative   bool   `json:"isNative"`
	IsStable   bool   `json:"isStable"`
	IsWrapped  bool   `json:"isWrapped"`
	WrappedOf  string `json:"wrappedOf,omitempty"`
}

// DEX Pool
type LiquidityPool struct {
	TokenA    string `json:"tokenA"`
	TokenB    string `json:"tokenB"`
	ReserveA  int64  `json:"reserveA"`
	ReserveB  int64  `json:"reserveB"`
	Fee       int    `json:"fee"` // in basis points (30 = 0.3%)
	Liquidity int64  `json:"liquidity"`
	APY       int    `json:"apy"`
}

// Farm info
type FarmInfo struct {
	PoolID      string `json:"poolId"`
	RewardToken string `json:"rewardToken"`
	StakedToken string `json:"stakedToken"`
	StakedAmt   int64 `json:"stakedAmount"`
	RewardAmt  int64 `json:"rewardAmount"`
	APY        int   `json:"apy"`
	StartTime  int64 `json:"startTime"`
	EndTime    int64 `json:"endTime"`
}

// Bridge info
type BridgeInfo struct {
	BridgeID      string `json:"bridgeId"`
	SourceChain   string `json:"sourceChain"`
	TargetChain   string `json:"targetChain"`
	Token         string `json:"token"`
	MinAmount     int64  `json:"minAmount"`
	MaxAmount     int64  `json:"maxAmount"`
	Fee           int    `json:"fee"` // in basis points
	EstimatedTime int   `json:"estimatedTime"` // milliseconds
	IsActive      bool   `json:"isActive"`
}

// Fee configuration
type FeeConfig struct {
	TradingFee    int `json:"tradingFee"`    // basis points
	SwapFee       int `json:"swapFee"`       // basis points
	BridgeFee     int `json:"bridgeFee"`    // basis points
	WalletTxFee   int `json:"walletTxFee"`  // basis points
	WithdrawalFee int `json:"withdrawalFee"`
	DepositFee    int `json:"depositFee"`
}

// Product
type Product struct {
	Name        string    `json:"name"`
	Version     string    `json:"version"`
	Status      string    `json:"status"`
	FeeConfig   FeeConfig `json:"feeConfig"`
	Enabled    bool      `json:"enabled"`
	CreatedAt   int64     `json:"createdAt"`
	UpdatedAt   int64     `json:"updatedAt"`
}

// Swap result
type SwapResult struct {
	InputToken  string   `json:"inputToken"`
	OutputToken string   `json:"outputToken"`
	AmountIn   int64    `json:"amountIn"`
	AmountOut int64    `json:"amountOut"`
	Fee        int64    `json:"fee"`
	Path       []string `json:"path"`
}

// Bridge result
type BridgeResult struct {
	SourceChain   string `json:"sourceChain"`
	TargetChain   string `json:"targetChain"`
	AmountSent   int64  `json:"amountSent"`
	AmountRecv   int64  `json:"amountReceived"`
	Fee          int64  `json:"fee"`
	EstimatedTime int   `json:"estimatedTime"`
}

// ============================================================================
// TigerEX Integration Core (Go)
// ============================================================================

type TigerEX struct {
	// Products
	tigerWallet    Product
	tigerSwap     Product
	tigerChain   Product
	tigerEx     Product

	// Chains
	evmChains    map[string]ChainConfig
	nonEvmChains map[string]ChainConfig

	// Tokens
	tokens map[string]TokenConfig

	// DEX Pools
	pools map[string]LiquidityPool

	// Farms
	farms map[string]FarmInfo

	// Bridges
	bridges map[string]BridgeInfo

	// Fee collection
	totalFeesCollected int64
	feeHistory        []FeeRecord

	// Mutex for thread safety
	mu sync.RWMutex

	// Stats
	initialized bool
}

// Fee record
type FeeRecord struct {
	Amount    int64  `json:"amount"`
	Source   string `json:"source"`
	Timestamp int64 `json:"timestamp"`
}

// NewTigerEX creates new TigerEX instance
func NewTigerEX() *TigerEX {
	t := &TigerEX{
		evmChains:    make(map[string]ChainConfig),
		nonEvmChains: make(map[string]ChainConfig),
		tokens:       make(map[string]TokenConfig),
		pools:        make(map[string]LiquidityPool),
		farms:        make(map[string]FarmInfo),
		bridges:      make(map[string]BridgeInfo),
		feeHistory:   make([]FeeRecord, 0),
	}

	// Initialize products
	t.tigerWallet = Product{
		Name:    "TigerWallet",
		Version: "1.0.0",
		Status:  "active",
		FeeConfig: FeeConfig{
			TradingFee:    0,
			SwapFee:      0,
			BridgeFee:    0,
			WalletTxFee: 10, // 0.1%
			WithdrawalFee: 5,
			DepositFee:   0,
		},
		Enabled:  true,
		CreatedAt: time.Now().UnixMilli(),
		UpdatedAt: time.Now().UnixMilli(),
	}

	t.tigerSwap = Product{
		Name:    "Tigerswap",
		Version: "1.0.0",
		Status:  "active",
		FeeConfig: FeeConfig{
			TradingFee:    0,
			SwapFee:      30, // 0.3%
			BridgeFee:    0,
			WalletTxFee:  0,
			WithdrawalFee: 0,
			DepositFee:   0,
		},
		Enabled:  true,
		CreatedAt: time.Now().UnixMilli(),
		UpdatedAt: time.Now().UnixMilli(),
	}

	t.tigerChain = Product{
		Name:    "TigerSmartChain",
		Version: "1.0.0",
		Status:  "active",
		FeeConfig: FeeConfig{
			TradingFee:    0,
			SwapFee:      0,
			BridgeFee:    10, // 0.1%
			WalletTxFee:  0,
			WithdrawalFee: 5,
			DepositFee:   0,
		},
		Enabled:  true,
		CreatedAt: time.Now().UnixMilli(),
		UpdatedAt: time.Now().UnixMilli(),
	}

	t.tigerEx = Product{
		Name:    "TigerEx",
		Version: "1.0.0",
		Status:  "active",
		FeeConfig: FeeConfig{
			TradingFee:    10, // 0.1%
			SwapFee:      0,
			BridgeFee:    0,
			WalletTxFee:  0,
			WithdrawalFee: 5,
			DepositFee:   0,
		},
		Enabled:  true,
		CreatedAt: time.Now().UnixMilli(),
		UpdatedAt: time.Now().UnixMilli(),
	}

	// Initialize chains, tokens, pools, farms, bridges
	t.initializeEvmChains()
	t.initializeNonEvmChains()
	t.initializeTokens()
	t.initializeDexPools()
	t.initializeFarms()
	t.initializeBridges()

	t.initialized = true

	return t
}

// ============================================================================
// Initialization Methods
// ============================================================================

func (t *TigerEX) initializeEvmChains() {
	// TigerSmartChain (Native)
	t.evmChains["tigersmartchain"] = ChainConfig{
		ID:        "tigersmartchain",
		Name:      "TigerSmartChain",
		Symbol:    "TGR",
		Category:  ChainCategoryEVM,
		Status:    ChainStatusActive,
		ChainID:   13000,
		RPCURLs:    []string{"https://rpc.tigersmartchain.com"},
		ExplorerURLs: []string{"https://scan.tigersmartchain.com"},
		NativeCurrency: NativeCurrency{Name: "Tiger", Symbol: "TGR", Decimals: 18},
		BlockTime:      2.0,
		SupportsEIP1559: true,
	}

	// Ethereum
	t.evmChains["ethereum"] = ChainConfig{
		ID:        "ethereum",
		Name:      "Ethereum",
		Symbol:    "ETH",
		Category:  ChainCategoryEVM,
		Status:    ChainStatusActive,
		ChainID:   1,
		RPCURLs:    []string{"https://eth.llamarpc.com"},
		ExplorerURLs: []string{"https://etherscan.io"},
		NativeCurrency: NativeCurrency{Name: "Ethereum", Symbol: "ETH", Decimals: 18},
		BlockTime:      12.0,
		SupportsEIP1559: true,
	}

	// BSC
	t.evmChains["bsc"] = ChainConfig{
		ID:        "bsc",
		Name:      "BNB Smart Chain",
		Symbol:    "BNB",
		Category:  ChainCategoryEVM,
		Status:    ChainStatusActive,
		ChainID:   56,
		RPCURLs:    []string{"https://bsc-dataseed.binance.org"},
		ExplorerURLs: []string{"https://bscscan.com"},
		NativeCurrency: NativeCurrency{Name: "BNB", Symbol: "BNB", Decimals: 18},
		BlockTime:      3.0,
		SupportsEIP1559: true,
	}

	// Polygon
	t.evmChains["polygon"] = ChainConfig{
		ID:        "polygon",
		Name:      "Polygon",
		Symbol:    "MATIC",
		Category:  ChainCategoryEVM,
		Status:    ChainStatusActive,
		ChainID:   137,
		RPCURLs:    []string{"https://polygon-rpc.com"},
		ExplorerURLs: []string{"https://polygonscan.com"},
		NativeCurrency: NativeCurrency{Name: "MATIC", Symbol: "MATIC", Decimals: 18},
		BlockTime:      2.0,
		SupportsEIP1559: true,
	}

	// Avalanche
	t.evmChains["avalanche"] = ChainConfig{
		ID:        "avalanche",
		Name:      "Avalanche",
		Symbol:    "AVAX",
		Category:  ChainCategoryEVM,
		Status:    ChainStatusActive,
		ChainID:   43114,
		RPCURLs:    []string{"https://api.avax.network/ext/bc/C/rpc"},
		ExplorerURLs: []string{"https://snowtrace.io"},
		NativeCurrency: NativeCurrency{Name: "AVAX", Symbol: "AVAX", Decimals: 18},
		BlockTime:      2.0,
		SupportsEIP1559: false,
	}

	// Arbitrum
	t.evmChains["arbitrum"] = ChainConfig{
		ID:        "arbitrum",
		Name:      "Arbitrum One",
		Symbol:    "ETH",
		Category:  ChainCategoryEVM,
		Status:    ChainStatusActive,
		ChainID:   42161,
		RPCURLs:    []string{"https://arb1.arbitrum.io/rpc"},
		ExplorerURLs: []string{"https://arbiscan.io"},
		NativeCurrency: NativeCurrency{Name: "Ethereum", Symbol: "ETH", Decimals: 18},
		BlockTime:      0.25,
		SupportsEIP1559: true,
	}

	// Optimism
	t.evmChains["optimism"] = ChainConfig{
		ID:        "optimism",
		Name:      "Optimism",
		Symbol:    "ETH",
		Category:  ChainCategoryEVM,
		Status:    ChainStatusActive,
		ChainID:   10,
		RPCURLs:    []string{"https://mainnet.optimism.io"},
		ExplorerURLs: []string{"https://optimistic.etherscan.io"},
		NativeCurrency: NativeCurrency{Name: "Ethereum", Symbol: "ETH", Decimals: 18},
		BlockTime:      2.0,
		SupportsEIP1559: true,
	}

	// Base
	t.evmChains["base"] = ChainConfig{
		ID:        "base",
		Name:      "Base",
		Symbol:    "ETH",
		Category:  ChainCategoryEVM,
		Status:    ChainStatusActive,
		ChainID:   8453,
		RPCURLs:    []string{"https://mainnet.base.org"},
		ExplorerURLs: []string{"https://basescan.org"},
		NativeCurrency: NativeCurrency{Name: "Ethereum", Symbol: "ETH", Decimals: 18},
		BlockTime:      2.0,
		SupportsEIP1559: true,
	}

	// Fantom
	t.evmChains["fantom"] = ChainConfig{
		ID:        "fantom",
		Name:      "Fantom",
		Symbol:    "FTM",
		Category:  ChainCategoryEVM,
		Status:    ChainStatusActive,
		ChainID:   250,
		RPCURLs:    []string{"https://rpc.ftm.tools"},
		ExplorerURLs: []string{"https://ftmscan.com"},
		NativeCurrency: NativeCurrency{Name: "Fantom", Symbol: "FTM", Decimals: 18},
		BlockTime:      1.0,
		SupportsEIP1559: false,
	}

	// Additional EVM chains (Celo, Gnosis, Moonbeam, zkEVM, Linea, Scroll, Astar, Klaytn, Cronos, Core, Mantle, Berachain, Sonic, Monad, MegaETH)
	chains := []ChainConfig{
		{"celo", "Celo", "CELO", ChainCategoryEVM, ChainStatusActive, 42220, []string{"https://forno.celo.org"}, []string{"https://explorer.celo.org"}, NativeCurrency{"Celo", "CELO", 18}, 5.0, false},
		{"gnosis", "Gnosis Chain", "GNO", ChainCategoryEVM, ChainStatusActive, 100, []string{"https://rpc.gnosischain.com"}, []string{"https://gnosisscan.io"}, NativeCurrency{"Gnosis", "GNO", 18}, 5.0, false},
		{"moonbeam", "Moonbeam", "GLMR", ChainCategoryEVM, ChainStatusActive, 1284, []string{"https://rpc.api.moonbeam.network"}, []string{"https://moonbeam.moonscan.io"}, NativeCurrency{"Glimmer", "GLMR", 18}, 12.0, false},
		{"zkevm", "Polygon zkEVM", "ETH", ChainCategoryEVM, ChainStatusActive, 1101, []string{"https://zkevm-rpc.com"}, []string{"https://zkevm.polygonscan.com"}, NativeCurrency{"Ethereum", "ETH", 18}, 1.0, true},
		{"linea", "Linea", "ETH", ChainCategoryEVM, ChainStatusActive, 59144, []string{"https://rpc.linea.build"}, []string{"https://lineascan.build"}, NativeCurrency{"Ethereum", "ETH", 18}, 2.0, true},
		{"scroll", "Scroll", "ETH", ChainCategoryEVM, ChainStatusActive, 534352, []string{"https://rpc.scroll.io"}, []string{"https://scrollscan.com"}, NativeCurrency{"Ethereum", "ETH", 18}, 3.0, true},
		{"astar", "Astar", "ASTR", ChainCategoryEVM, ChainStatusActive, 432201, []string{"https://rpc.astar.network"}, []string{"https://blockscout.com/astar"}, NativeCurrency{"Astar", "ASTR", 18}, 12.0, false},
		{"klaytn", "Klaytn", "KLAY", ChainCategoryEVM, ChainStatusActive, 8217, []string{"https://klaytn-mainnet-rpc.allthatnode.com"}, []string{"https://scope.klaytn.com"}, NativeCurrency{"Klaytn", "KLAY", 18}, 1.0, false},
		{"cronos", "Cronos", "CRO", ChainCategoryEVM, ChainStatusActive, 25, []string{"https://evm.cronos.org"}, []string{"https://cronoscan.com"}, NativeCurrency{"Cronos", "CRO", 18}, 5.0, false},
		{"core", "Core", "CORE", ChainCategoryEVM, ChainStatusActive, 1116, []string{"https://rpc.coredao.org"}, []string{"https://scan.coredao.org"}, NativeCurrency{"Core", "CORE", 18}, 2.0, false},
		{"mantle", "Mantle", "MNT", ChainCategoryEVM, ChainStatusActive, 5000, []string{"https://rpc.mantle.xyz"}, []string{"https://explorer.mantle.xyz"}, NativeCurrency{"Mantle", "MNT", 18}, 2.0, false},
		{"berachain", "Berachain", "BERA", ChainCategoryEVM, ChainStatusActive, 845321, []string{"https://rpc.berachain.com"}, []string{"https://berascan.com"}, NativeCurrency{"Berachain", "BERA", 18}, 2.0, false},
		{"sonic", "Sonic", "S", ChainCategoryEVM, ChainStatusActive, 1460, []string{"https://rpc.soniclabs.com"}, []string{"https://sonicscan.org"}, NativeCurrency{"Sonic", "S", 18}, 2.0, false},
		{"monad", "Monad", "MON", ChainCategoryEVM, ChainStatusActive, 10143, []string{"https://rpc.monad.xyz"}, []string{"https://monadscan.com"}, NativeCurrency{"Monad", "MON", 18}, 2.0, false},
		{"megaeth", "MegaETH", "MEGA", ChainCategoryEVM, ChainStatusActive, 1205398815, []string{"https://rpc.megaeth.com"}, []string{"https://megascan.io"}, NativeCurrency{"MegaETH", "MEGA", 18}, 0.1, false},
	}

	for _, c := range chains {
		t.evmChains[c.ID] = c
	}
}

func (t *TigerEX) initializeNonEvmChains() {
	// Solana
	t.nonEvmChains["solana"] = ChainConfig{
		ID:        "solana",
		Name:      "Solana",
		Symbol:    "SOL",
		Category:  ChainCategorySolana,
		Status:    ChainStatusActive,
		ChainID:   -1,
		RPCURLs:    []string{"https://api.mainnet-beta.solana.com"},
		ExplorerURLs: []string{"https://solscan.io"},
		NativeCurrency: NativeCurrency{Name: "Solana", Symbol: "SOL", Decimals: 9},
		BlockTime:      0.4,
		SupportsEIP1559: false,
	}

	// Other Non-EVM chains
	chains := []ChainConfig{
		{"aptos", "Aptos", "APT", ChainCategoryAptos, ChainStatusActive, -1, []string{"https://aptos-mainnet.nodereal.io/v1"}, []string{"https://explorer.aptoslabs.com"}, NativeCurrency{"Aptos", "APT", 8}, 1.0, false},
		{"sui", "Sui", "SUI", ChainCategorySui, ChainStatusActive, -1, []string{"https://rpc.mainnet.sui.io"}, []string{"https://suiscan.xyz"}, NativeCurrency{"Sui", "SUI", 9}, 1.0, false},
		{"ton", "TON", "TON", ChainCategoryTON, ChainStatusActive, -1, []string{"https://toncenter.com/api/v2"}, []string{"https://tonscan.org"}, NativeCurrency{"TON", "TON", 9}, 5.0, false},
		{"cosmos", "Cosmos", "ATOM", ChainCategoryCosmos, ChainStatusActive, -1, []string{"https://rpc-cosmoshub.keplr.app"}, []string{"https://mintscan.io/cosmos"}, NativeCurrency{"Atom", "ATOM", 6}, 7.0, false},
		{"near", "NEAR Protocol", "NEAR", ChainCategoryNear, ChainStatusActive, -1, []string{"https://rpc.mainnet.near.org"}, []string{"https://explorer.near.org"}, NativeCurrency{"NEAR", "NEAR", 24}, 1.0, false},
		{"algorand", "Algorand", "ALGO", ChainCategoryOther, ChainStatusActive, -1, []string{"https://mainnet-api.algorand.network"}, []string{"https://algoexplorer.cc"}, NativeCurrency{"Algorand", "ALGO", 6}, 3.0, false},
		{"osmosis", "Osmosis", "OSMO", ChainCategoryCosmos, ChainStatusActive, -1, []string{"https://rpc-osmosis.keplr.app"}, []string{"https://mintscan.io/osmosis"}, NativeCurrency{"Osmosis", "OSMO", 6}, 5.0, false},
		{"juno", "Juno", "JUNO", ChainCategoryCosmos, ChainStatusActive, -1, []string{"https://rpc.juno.kingnodes.com"}, []string{"https://mintscan.io/juno"}, NativeCurrency{"Juno", "JUNO", 6}, 7.0, false},
		{"injective", "Injective", "INJ", ChainCategoryCosmos, ChainStatusActive, -1, []string{"https://public.injective.network"}, []string{"https://explorer.injective.network"}, NativeCurrency{"Injective", "INJ", 18}, 2.0, false},
		{"sei", "Sei", "SEI", ChainCategoryCosmos, ChainStatusActive, -1, []string{"https://rpc.sei.io"}, []string{"https://seistats.io"}, NativeCurrency{"Sei", "SEI", 6}, 0.4, false},
		{"radix", "Radix", "XRD", ChainCategoryOther, ChainStatusActive, -1, []string{"https://mainnet.radixdlt.com"}, []string{"https://explorer.radixdlt.com"}, NativeCurrency{"Radix", "XRD", 10}, 2.0, false},
		{"flow", "Flow", "FLOW", ChainCategoryOther, ChainStatusActive, -1, []string{"https://flow-evm.g.alchemy.com/v2/demo"}, []string{"https://flowdiver.io"}, NativeCurrency{"Flow", "FLOW", 8}, 2.0, false},
		{"hedera", "Hedera", "HBAR", ChainCategoryOther, ChainStatusActive, -1, []string{"https://mainnet.mirror.hedera.com/api/v1/contracts/call"}, []string{"https://hashscan.io"}, NativeCurrency{"Hedera", "HBAR", 8}, 3.0, false},
		{"icon", "ICON", "ICX", ChainCategoryOther, ChainStatusActive, -1, []string{"https://ctz.solidwallet.io"}, []string{"https://tracker.icon.community"}, NativeCurrency{"ICON", "ICX", 18}, 2.0, false},
		{"vechain", "VeChain", "VET", ChainCategoryOther, ChainStatusActive, -1, []string{"https://mainnet.vechain.org"}, []string{"https://explore.vechain.org"}, NativeCurrency{"VeChain", "VET", 18}, 6.0, false},
		{"theta", "Theta", "THETA", ChainCategoryOther, ChainStatusActive, -1, []string{"https://eth-rpc-api.thetatoken.org/rest"}, []string{"https://explorer.thetatoken.org"}, NativeCurrency{"Theta", "THETA", 18}, 10.0, false},
		{"multiversx", "MultiversX", "EGLD", ChainCategoryOther, ChainStatusActive, -1, []string{"https://api.multiversx.com"}, []string{"https://explorer.multiversx.com"}, NativeCurrency{"MultiversX", "EGLD", 18}, 6.0, false},
		{"polkadot", "Polkadot", "DOT", ChainCategoryOther, ChainStatusActive, -1, []string{"https://rpc.polkadot.io"}, []string{"https://polkadot.subscan.io"}, NativeCurrency{"Polkadot", "DOT", 10}, 12.0, false},
		{"kusama", "Kusama", "KSM", ChainCategoryOther, ChainStatusActive, -1, []string{"https://kusama-rpc.polkadot.io"}, []string{"https://kusama.subscan.io"}, NativeCurrency{"Kusama", "KSM", 12}, 6.0, false},
		{"kadena", "Kadena", "KDA", ChainCategoryOther, ChainStatusActive, -1, []string{"https://api.chainweb.com"}, []string{"https://explorer.kadena.io"}, NativeCurrency{"Kadena", "KDA", 12}, 1.0, false},
		{"casper", "Casper", "CSPR", ChainCategoryOther, ChainStatusActive, -1, []string{"https://rpc.mainnet.casper.network"}, []string{"https://cspr.live"}, NativeCurrency{"Casper", "CSPR", 9}, 60.0, false},
		{"fuel", "Fuel", "FUEL", ChainCategoryOther, ChainStatusActive, -1, []string{"https://mainnet.fuel.network"}, []string{"https://fuelscan.io"}, NativeCurrency{"Fuel", "FUEL", 18}, 2.0, false},
		{"tron", "Tron", "TRX", ChainCategoryOther, ChainStatusActive, -1, []string{"https://api.trongrid.io"}, []string{"https://tronscan.org"}, NativeCurrency{"Tron", "TRX", 6}, 3.0, false},
		{"stellar", "Stellar", "XLM", ChainCategoryOther, ChainStatusActive, -1, []string{"https://horizon.stellar.org"}, []string{"https://stellar.expert"}, NativeCurrency{"Stellar", "XLM", 7}, 5.0, false},
	}

	for _, c := range chains {
		t.nonEvmChains[c.ID] = c
	}
}

func (t *TigerEX) initializeTokens() {
	// Tiger Ecosystem Tokens
	t.tokens["TGR"] = TokenConfig{
		Address:  "0x0000000000000000000000000000000000000000",
		ChainID:  "13000",
		Symbol:  "TGR",
		Name:    "Tiger Coin",
		Decimals: 18,
		IsNative: true,
		IsStable: false,
	}

	t.tokens["RUSD"] = TokenConfig{
		Address:  "0x0000000000000000000000000000000000000001",
		ChainID: "13000",
		Symbol:  "RUSD",
		Name:    "Royal Tiger United States Dollar",
		Decimals: 18,
		IsStable: true,
	}

	// Major tokens
	majorTokens := []TokenConfig{
		{"0x0000000000000000000000000000000000000000", "1", "ETH", "Ethereum", 18, true, false, false, ""},
		{"0x0000000000000000000000000000000000000000", "56", "BNB", "BNB", 18, true, false, false, ""},
		{"0x0000000000000000000000000000000000000000", "137", "MATIC", "Polygon", 18, true, false, false, ""},
		{"0x0000000000000000000000000000000000000000", "43114", "AVAX", "Avalanche", 18, true, false, false, ""},
		{"0x0000000000000000000000000000000000000000", "42161", "ARB", "Arbitrum", 18, true, false, false, ""},
		{"0x0000000000000000000000000000000000000000", "10", "OP", "Optimism", 18, true, false, false, ""},
		{"0x0000000000000000000000000000000000000000", "8453", "ETH", "Base", 18, true, false, false, ""},
		{"0xdAC17F958D2ee523a2206206994597C13D831ec7", "1", "USDT", "Tether USD", 6, false, true, false, ""},
		{"0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48", "1", "USDC", "USD Coin", 6, false, true, false, ""},
		{"0x6B175474E89094C44Da98b954EedACb42155A68E", "1", "DAI", "Dai Stablecoin", 18, false, true, false, ""},
		{"0x2260FAC5E5542a773Aa44fBfeafF052ED862158d8", "1", "WBTC", "Wrapped Bitcoin", 8, false, false, true, "BTC"},
		{"0x514910771AF9Ca656af840dff83E8264EcF986CA", "1", "LINK", "Chainlink", 18, false, false, false, ""},
		{"0x1f9840a85d5aF5bf1D1762fFFBDACADf3C9AE41C9", "1", "UNI", "Uniswap", 18, false, false, false, ""},
		{"0x7Fc66500c84A76Ad7e9c93437bB5cB6579d6eD0b6", "1", "AAVE", "Aave", 18, false, false, false, ""},
		{"0x9f8F72aA8904F90e8fECfF6aD136d37A5B9E6aB68", "1", "MKR", "Maker", 18, false, false, false, ""},
		{"0xD533a049740a5DaaF2d75dC2B229A497F2bC30b6", "1", "CRV", "Curve DAO", 18, false, false, false, ""},
		{"0x5A98FcB270B283fD32768F82d222f5ebd5eC3bF4", "1", "LDO", "Lido DAO", 18, false, false, false, ""},
		{"0xC011a73ee8576f4a9c5f60bcbDB3B7BF4b6fC0Ea4", "1", "SNX", "Synthetix", 18, false, false, false, ""},
		{"0xc00e94Cb662C3520282E6f5716cCde3D8C48bF0b5", "1", "COMP", "Compound", 18, false, false, false, ""},
		{"0x6B3595068770082E5bB3a54eB1EA52F4aC5b4EaD4", "1", "SUSHI", "SushiSwap", 18, false, false, false, ""},
	}

	for _, token := range majorTokens {
		t.tokens[token.Symbol] = token
	}
}

func (t *TigerEX) initializeDexPools() {
	pools := []LiquidityPool{
		{"TGR", "USDT", 1000000000000000000, 500000000000, 30, 1000000000000000000, 25},
		{"TGR", "RUSD", 500000000000000000, 500000000000000000, 30, 500000000000000000, 30},
		{"TGR", "ETH", 1000000000000000000, 500000000000000000000, 30, 500000000000000000, 20},
		{"RUSD", "USDT", 1000000000000000000, 1000000000000000, 10, 1000000000000000000, 10},
		{"ETH", "USDT", 1000000000000000000, 3000000000000000, 30, 1000000000000000000, 15},
		{"BTC", "USDT", 10000000000, 5000000000000, 30, 10000000000, 12},
		{"ETH", "BTC", 500000000000000000, 1000000000, 30, 1000000000, 18},
	}

	for _, pool := range pools {
		key := pool.TokenA + "-" + pool.TokenB
		t.pools[key] = pool
	}
}

func (t *TigerEX) initializeFarms() {
	farms := []FarmInfo{
		{"TGR-USDT", "TGR", "TGR-USDT", 0, 0, 25, time.Now().UnixMilli(), time.Now().UnixMilli() + 365*24*60*60*1000},
		{"TGR-ETH", "TGR", "TGR-ETH", 0, 0, 20, time.Now().UnixMilli(), time.Now().UnixMilli() + 365*24*60*60*1000},
		{"RUSD-USDT", "TGR", "RUSD-USDT", 0, 0, 15, time.Now().UnixMilli(), time.Now().UnixMilli() + 365*24*60*60*1000},
	}

	for _, farm := range farms {
		t.farms[farm.PoolID] = farm
	}
}

func (t *TigerEX) initializeBridges() {
	bridges := []BridgeInfo{
		{"eth-bsc", "ethereum", "bsc", "*", 10000000000000000, 1000000000000000000000, 10, 600000, true},
		{"eth-polygon", "ethereum", "polygon", "*", 10000000000000000, 1000000000000000000000, 10, 900000, true},
		{"eth-arbitrum", "ethereum", "arbitrum", "*", 10000000000000000, 1000000000000000000000, 15, 1200000, true},
		{"eth-optimism", "ethereum", "optimism", "*", 10000000000000000, 1000000000000000000000, 15, 900000, true},
		{"eth-avalanche", "ethereum", "avalanche", "*", 10000000000000000, 1000000000000000000000, 10, 600000, true},
		{"bsc-polygon", "bsc", "polygon", "*", 10000000000000000, 1000000000000000000000, 10, 600000, true},
		{"tgr-eth", "tigersmartchain", "ethereum", "TGR", 100000000000000000, 10000000000000000000, 10, 1800000, true},
		{"tgr-bsc", "tigersmartchain", "bsc", "TGR", 100000000000000000, 10000000000000000000, 10, 1200000, true},
		{"rusd-eth", "tigersmartchain", "ethereum", "RUSD", 100000000000000000, 10000000000000000000, 10, 1800000, true},
	}

	for _, bridge := range bridges {
		t.bridges[bridge.BridgeID] = bridge
	}
}

// ============================================================================
// Public API Methods
// ============================================================================

// GetSupportedEvmChains returns all EVM chains
func (t *TigerEX) GetSupportedEvmChains() []ChainConfig {
	t.mu.RLock()
	defer t.mu.RUnlock()

	chains := make([]ChainConfig, 0, len(t.evmChains))
	for _, c := range t.evmChains {
		chains = append(chains, c)
	}
	return chains
}

// GetSupportedNonEvmChains returns all Non-EVM chains
func (t *TigerEX) GetSupportedNonEvmChains() []ChainConfig {
	t.mu.RLock()
	defer t.mu.RUnlock()

	chains := make([]ChainConfig, 0, len(t.nonEvmChains))
	for _, c := range t.nonEvmChains {
		chains = append(chains, c)
	}
	return chains
}

// GetSupportedTokens returns all tokens
func (t *TigerEX) GetSupportedTokens() []TokenConfig {
	t.mu.RLock()
	defer t.mu.RUnlock()

	tokens := make([]TokenConfig, 0, len(t.tokens))
	for _, token := range t.tokens {
		tokens = append(tokens, token)
	}
	return tokens
}

// GetDexPools returns all DEX pools
func (t *TigerEX) GetDexPools() []LiquidityPool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	pools := make([]LiquidityPool, 0, len(t.pools))
	for _, pool := range t.pools {
		pools = append(pools, pool)
	}
	return pools
}

// GetFarms returns all farms
func (t *TigerEX) GetFarms() []FarmInfo {
	t.mu.RLock()
	defer t.mu.RUnlock()

	farms := make([]FarmInfo, 0, len(t.farms))
	for _, farm := range t.farms {
		farms = append(farms, farm)
	}
	return farms
}

// GetBridges returns all bridges
func (t *TigerEX) GetBridges() []BridgeInfo {
	t.mu.RLock()
	defer t.mu.RUnlock()

	bridges := make([]BridgeInfo, 0, len(t.bridges))
	for _, bridge := range t.bridges {
		bridges = append(bridges, bridge)
	}
	return bridges
}

// GetProductStatus returns product status
func (t *TigerEX) GetProductStatus(product string) Product {
	switch product {
	case "wallet":
		return t.tigerWallet
	case "swap":
		return t.tigerSwap
	case "smartchain":
		return t.tigerChain
	case "ex":
		return t.tigerEx
	default:
		return Product{}
	}
}

// GetStats returns platform statistics
func (t *TigerEX) GetStats() map[string]interface{} {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return map[string]interface{}{
		"totalEvmChains":    len(t.evmChains),
		"totalNonEvmChains": len(t.nonEvmChains),
		"totalTokens":     len(t.tokens),
		"totalPools":      len(t.pools),
		"totalFarms":      len(t.farms),
		"totalBridges":   len(t.bridges),
		"initialized":    t.initialized,
	}
}

// CalculateSwap calculates swap output
func (t *TigerEX) CalculateSwap(inputToken, outputToken string, amountIn int64) (SwapResult, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	poolKey := inputToken + "-" + outputToken
	pool, exists := t.pools[poolKey]

	if !exists {
		// Try reverse
		poolKey = outputToken + "-" + inputToken
		pool, exists = t.pools[poolKey]
		if !exists {
			// Multi-hop routing
			return t.calculateMultiHopSwap(inputToken, outputToken, amountIn)
		}
	}

	reserveIn := pool.ReserveA
	reserveOut := pool.ReserveB
	if outputToken != pool.TokenA {
		reserveIn = pool.ReserveB
		reserveOut = pool.ReserveA
	}

	amountOut := (amountIn * reserveOut) / (reserveIn + amountIn)
	fee := int64(pool.Fee) * amountOut / 10000

	return SwapResult{
		InputToken:  inputToken,
		OutputToken: outputToken,
		AmountIn:   amountIn,
		AmountOut:  amountOut - fee,
		Fee:       fee,
		Path:      []string{inputToken, outputToken},
	}, nil
}

// CalculateBridge calculates bridge transfer
func (t *TigerEX) CalculateBridge(sourceChain, targetChain, token string, amount int64) (BridgeResult, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	bridgeKey := sourceChain + "-" + targetChain
	bridge, exists := t.bridges[bridgeKey]

	if !exists {
		return BridgeResult{}, fmt.Errorf("no bridge found from %s to %s", sourceChain, targetChain)
	}

	if !bridge.IsActive {
		return BridgeResult{}, fmt.Errorf("bridge %s is not active", bridgeKey)
	}

	if amount < bridge.MinAmount {
		return BridgeResult{}, fmt.Errorf("amount too low. Minimum: %d", bridge.MinAmount)
	}

	if amount > bridge.MaxAmount {
		return BridgeResult{}, fmt.Errorf("amount too high. Maximum: %d", bridge.MaxAmount)
	}

	fee := int64(bridge.Fee) * amount / 10000

	return BridgeResult{
		SourceChain:   sourceChain,
		TargetChain:   targetChain,
		AmountSent:   amount,
		AmountRecv:   amount - fee,
		Fee:          fee,
		EstimatedTime: bridge.EstimatedTime,
	}, nil
}

// SearchChains searches chains by name or symbol
func (t *TigerEX) SearchChains(query string) []ChainConfig {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var results []ChainConfig
	lowerQuery := toLower(query)

	for _, chain := range t.evmChains {
		if contains(toLower(chain.Name), lowerQuery) || contains(toLower(chain.Symbol), lowerQuery) || contains(chain.ID, lowerQuery) {
			results = append(results, chain)
		}
	}

	for _, chain := range t.nonEvmChains {
		if contains(toLower(chain.Name), lowerQuery) || contains(toLower(chain.Symbol), lowerQuery) || contains(chain.ID, lowerQuery) {
			results = append(results, chain)
		}
	}

	return results
}

// AddEvmChain adds new EVM chain
func (t *TigerEX) AddEvmChain(config ChainConfig) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if _, exists := t.evmChains[config.ID]; exists {
		return fmt.Errorf("chain %s already exists", config.ID)
	}

	t.evmChains[config.ID] = config
	return nil
}

// AddNonEvmChain adds new Non-EVM chain
func (t *TigerEX) AddNonEvmChain(config ChainConfig) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if _, exists := t.nonEvmChains[config.ID]; exists {
		return fmt.Errorf("chain %s already exists", config.ID)
	}

	t.nonEvmChains[config.ID] = config
	return nil
}

// AddToken adds new token
func (t *TigerEX) AddToken(token TokenConfig) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if _, exists := t.tokens[token.Symbol]; exists {
		return fmt.Errorf("token %s already exists", token.Symbol)
	}

	t.tokens[token.Symbol] = token
	return nil
}

// CreatePool creates new DEX pool
func (t *TigerEX) CreatePool(tokenA, tokenB string, fee int) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	poolKey := tokenA + "-" + tokenB
	if _, exists := t.pools[poolKey]; exists {
		return fmt.Errorf("pool %s already exists", poolKey)
	}

	t.pools[poolKey] = LiquidityPool{
		TokenA:    tokenA,
		TokenB:    tokenB,
		ReserveA:  0,
		ReserveB:  0,
		Fee:      fee,
		Liquidity: 0,
		APY:      0,
	}

	return nil
}

// CreateFarm creates new farm
func (t *TigerEX) CreateFarm(poolId, rewardToken string, apy int) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if _, exists := t.farms[poolId]; exists {
		return fmt.Errorf("farm %s already exists", poolId)
	}

	t.farms[poolId] = FarmInfo{
		PoolID:      poolId,
		RewardToken: rewardToken,
		StakedToken: poolId,
		APY:        apy,
		StartTime:  time.Now().UnixMilli(),
		EndTime:   time.Now().UnixMilli() + 365*24*60*60*1000,
	}

	return nil
}

// AddBridge adds new bridge
func (t *TigerEX) AddBridge(sourceChain, targetChain, token string, minAmount, maxAmount int64, fee int, estimatedTime int) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	bridgeId := sourceChain + "-" + targetChain
	if _, exists := t.bridges[bridgeId]; exists {
		return fmt.Errorf("bridge %s already exists", bridgeId)
	}

	t.bridges[bridgeId] = BridgeInfo{
		BridgeID:      bridgeId,
		SourceChain:   sourceChain,
		TargetChain:   targetChain,
		Token:         token,
		MinAmount:     minAmount,
		MaxAmount:     maxAmount,
		Fee:          fee,
		EstimatedTime: estimatedTime,
		IsActive:      true,
	}

	return nil
}

// CollectFee collects fee (internal)
func (t *TigerEX) CollectFee(amount int64, source string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.totalFeesCollected += amount
	t.feeHistory = append(t.feeHistory, FeeRecord{
		Amount:    amount,
		Source:   source,
		Timestamp: time.Now().UnixMilli(),
	})
}

// GetFeeSummary returns fee collection summary
func (t *TigerEX) GetFeeSummary() map[string]interface{} {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return map[string]interface{}{
		"totalFees": t.totalFeesCollected,
		"history":   t.feeHistory,
	}
}

// ============================================================================
// Helper Methods
// ============================================================================

func (t *TigerEX) calculateMultiHopSwap(inputToken, outputToken string, amountIn int64) (SwapResult, error) {
	hopToken := "USDT"
	pool1Key := inputToken + "-" + hopToken
	pool2Key := hopToken + "-" + outputToken

	pool1, exists1 := t.pools[pool1Key]
	pool2, exists2 := t.pools[pool2Key]

	if !exists1 || !exists2 {
		return SwapResult{}, fmt.Errorf("no pool found for %s-%s", inputToken, outputToken)
	}

	intermediate := (amountIn * pool1.ReserveB) / (pool1.ReserveA + amountIn)
	fee1 := int64(pool1.Fee) * intermediate / 10000
	afterFee := intermediate - fee1

	amountOut := (afterFee * pool2.ReserveB) / (pool2.ReserveA + afterFee)
	fee2 := int64(pool2.Fee) * amountOut / 10000

	return SwapResult{
		InputToken:  inputToken,
		OutputToken: outputToken,
		AmountIn:   amountIn,
		AmountOut:  amountOut - fee2,
		Fee:       fee1 + fee2,
		Path:      []string{inputToken, hopToken, outputToken},
	}, nil
}

func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			result[i] = c + 32
		} else {
			result[i] = c
		}
	}
	return string(result)
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// ============================================================================
// Main Function (for testing)
// ============================================================================

func main() {
	fmt.Println("=== TigerEX Go Integration ===")

	ex := NewTigerEX()

	// Test stats
	stats := ex.GetStats()
	fmt.Printf("\nPlatform Statistics:\n")
	fmt.Printf("  EVM Chains: %v\n", stats["totalEvmChains"])
	fmt.Printf("  Non-EVM Chains: %v\n", stats["totalNonEvmChains"])
	fmt.Printf("  Tokens: %v\n", stats["totalTokens"])
	fmt.Printf("  DEX Pools: %v\n", stats["totalPools"])
	fmt.Printf("  Farms: %v\n", stats["totalFarms"])
	fmt.Printf("  Bridges: %v\n", stats["totalBridges"])
	fmt.Printf("  Initialized: %v\n", stats["initialized"])

	// Test products
	fmt.Printf("\nProducts:\n")
	wallet := ex.GetProductStatus("wallet")
	fmt.Printf("  TigerWallet: %s v%s - %s\n", wallet.Name, wallet.Version, wallet.Status)
	swap := ex.GetProductStatus("swap")
	fmt.Printf("  Tigerswap: %s v%s - %s\n", swap.Name, swap.Version, swap.Status)
	chain := ex.GetProductStatus("smartchain")
	fmt.Printf("  TigerSmartChain: %s v%s - %s\n", chain.Name, chain.Version, chain.Status)
	exProd := ex.GetProductStatus("ex")
	fmt.Printf("  TigerEx: %s v%s - %s\n", exProd.Name, exProd.Version, exProd.Status)

	// Test swap
	fmt.Printf("\nSwap Test (100 USDT to TGR):\n")
	result, err := ex.CalculateSwap("USDT", "TGR", 100000000) // 100 USDT (6 decimals)
	if err != nil {
		fmt.Printf("  Error: %v\n", err)
	} else {
		fmt.Printf("  Input: %d USDT\n", result.AmountIn)
		fmt.Printf("  Output: %d TGR\n", result.AmountOut)
		fmt.Printf("  Fee: %d TGR\n", result.Fee)
		fmt.Printf("  Path: %v\n", result.Path)
	}

	// Test bridge
	fmt.Printf("\nBridge Test (ETH to BSC):\n")
	bridgeResult, err := ex.CalculateBridge("ethereum", "bsc", "ETH", 1000000000000000000) // 1 ETH
	if err != nil {
		fmt.Printf("  Error: %v\n", err)
	} else {
		fmt.Printf("  Sent: %d ETH\n", bridgeResult.AmountSent)
		fmt.Printf("  Received: %d ETH\n", bridgeResult.AmountRecv)
		fmt.Printf("  Fee: %d ETH\n", bridgeResult.Fee)
		fmt.Printf("  Time: %dms\n", bridgeResult.EstimatedTime)
	}

	// Test chain search
	fmt.Printf("\nChain Search (Polygon):\n")
	chains := ex.SearchChains("Polygon")
	fmt.Printf("  Found: %d chains\n", len(chains))
	for _, c := range chains {
		fmt.Printf("    - %s (%s) - Chain ID: %d\n", c.Name, c.Symbol, c.ChainID)
	}

	fmt.Printf("\n=== Test Complete ===\n")
}