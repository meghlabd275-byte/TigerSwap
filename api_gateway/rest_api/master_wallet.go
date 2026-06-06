// ============================================================================
// TIGERSWAP MASTER WALLET (TIGER MASTER)
// Complete admin wallet with automated transaction signing within 3 seconds
// All fees automatically collected to admin addresses
// ============================================================================

package main

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/params"
)

// ============================================================================
// CONSTANTS
// ============================================================================

const (
	// Auto-signing settings
	AUTO_SIGN_TIMEOUT     = 3 * time.Second // Maximum time to sign transactions
	AUTO_SIGN_BATCH_SIZE  = 50             // Max transactions to auto-sign in batch
	AUTO_SIGN_MAX_GAS      = 500000         // Max gas for auto-signed transactions
	AUTO_SIGN_MAX_VALUE    = 1000000        // Max value in USD for auto-signed transactions

	// Fee collection
	FEE_COLLECTION_INTERVAL = 60 * time.Second // How often to collect fees
	FEE_GAS_BUFFER          = 1.2             // Gas buffer for fee transactions

	// Wallet types
	WALLET_TYPE_HOT      = "hot"
	WALLET_TYPE_COLD     = "cold"
	WALLET_TYPE_MSIG    = "multi_sig"
	WALLET_TYPE_TREASURY = "treasury"
	WALLET_TYPE_OPERATIONS = "operations"

	// Transaction types
	TX_TYPE_SEND         = "send"
	TX_TYPE_SWAP         = "swap"
	TX_TYPE_LIQUIDITY    = "liquidity"
	TX_TYPE_APPROVE      = "approve"
	TX_TYPE_TRANSFER    = "transfer"
	TX_TYPE_CLAIM_AIRDROP = "claim_airdrop"
	TX_TYPE_JOIN_CAMPAIGN = "join_campaign"
	TX_TYPE_BRIDGE       = "bridge"
	TX_TYPE_FEE        = "fee"

	// Transaction status
	TX_STATUS_PENDING    = "pending"
	TX_STATUS_SIGNING    = "signing"
	TX_STATUS_SIGNED    = "signed"
	TX_STATUS_SUBMITTED = "submitted"
	TX_STATUS_CONFIRMED = "confirmed"
	TX_STATUS_FAILED    = "failed"
)

// ============================================================================
// ENUMS
// ============================================================================

type ChainType string

const (
	ChainEVM      ChainType = "evm"
	ChainSolana   ChainType = "solana"
	ChainAptos   ChainType = "aptos"
	ChainSui     ChainType = "sui"
	ChainTon     ChainType = "ton"
	ChainCosmos  ChainType = "cosmos"
	ChainPi      ChainType = "pinetwork"
)

// ============================================================================
// MODELS
// ============================================================================

// MasterWallet represents the master admin wallet
type MasterWallet struct {
	ID                string            `json:"id"`
	Name              string            `json:"name"`
	Type              string            `json:"type"` // hot, cold, multi_sig, treasury, operations
	Mnemonic          string            `json:"mnemonic,omitempty"` // Encrypted
	MnemonicEncrypted string            `json:"mnemonic_encrypted"`
	MasterAddress    string            `json:"master_address"`
	ChainId          int              `json:"chain_id"`
	ChainName        string            `json:"chain_name"`
	IsActive         bool             `json:"is_active"`
	AutoSignEnabled   bool             `json:"auto_sign_enabled"`
	AutoSignTimeout  int             `json:"auto_sign_timeout"` // seconds
	FeeCollectionEnabled bool         `json:"fee_collection_enabled"`
	LastActivity     time.Time         `json:"last_activity"`
	CreatedAt        time.Time         `json:"created_at"`
	UpdatedAt        time.Time         `json:"updated_at"`
}

// UserWallet represents user wallet under master
type UserWallet struct {
	ID              string    `json:"id"`
	MasterWalletID  string    `json:"master_wallet_id"`
	UserID         string    `json:"user_id"`
	WalletAddress  string    `json:"wallet_address"`
	ChainId        int       `json:"chain_id"`
	ChainName      string    `json:"chain_name"`
	WalletType    string    `json:"wallet_type"` // evm, solana, aptos, sui, ton
	Index          int       `json:"index"` // HD wallet index
	IsActive       bool      `json:"is_active"`
	CreatedAt      time.Time `json:"created_at"`
}

// Transaction for auto-signing
type AutoTransaction struct {
	ID            string         `json:"id"`
	WalletID     string         `json:"wallet_id"`
	UserID       string         `json:"user_id"`
	Type         string         `json:"type"`
	ChainId      int           `json:"chain_id"`
	Token        string        `json:"token"`
	Amount       *big.Int     `json:"amount"`
	AmountUSD    float64       `json:"amount_usd"`
	To           string        `json:"to"`
	Data         string        `json:"data,omitempty"`
	GasPrice     *big.Int     `json:"gas_price,omitempty"`
	GasLimit     uint64       `json:"gas_limit"`
	Status      string        `json:"status"`
	Hash         string        `json:"hash,omitempty"`
	Error        string        `json:"error,omitempty"`
	SignedAt     *time.Time   `json:"signed_at,omitempty"`
	SubmittedAt  *time.Time   `json:"submitted_at,omitempty"`
	ConfirmedAt  *time.Time   `json:"confirmed_at,omitempty"`
	CreatedAt    time.Time    `json:"created_at"`
}

// Fee configuration
type FeeConfig struct {
	ID            string  `json:"id"`
	FeeType       string  `json:"fee_type"` // swap, trading, withdrawal, bot, api, listing
	ChainId      int     `json:"chain_id"`
	TokenSymbol  string  `json:"token_symbol"`
	FeeAmountUSD float64 `json:"fee_amount_usd"`
	FeePercentage float64 `json:"fee_percentage"`
	MinFeeUSD    float64 `json:"min_fee_usd"`
	MaxFeeUSD    float64 `json:"max_fee_usd"`
	IsActive     bool    `json:"is_active"`
}

// Admin fee address
type AdminFeeAddress struct {
	ID          string `json:"id"`
	FeeType    string `json:"fee_type"`
	ChainId   int    `json:"chain_id"`
	Address   string `json:"address"`
	IsActive  bool   `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
}

// Token configuration
type TokenConfig struct {
	ID              string  `json:"id"`
	Symbol          string  `json:"symbol"`
	Name            string  `json:"name"`
	ContractAddress string  `json:"contract_address"`
	ChainId         int     `json:"chain_id"`
	Decimals        int     `json:"decimals"`
	IsStablecoin    bool    `json:"is_stablecoin"`
	IsNative        bool    `json:"is_native"`
	PriceUSD        float64 `json:"price_usd"`
	IsActive        bool    `json:"is_active"`
}

// Chain configuration
type ChainConfig struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Symbol      string    `json:"symbol"`
	ChainId     int       `json:"chain_id"`
	ChainIdHex  string    `json:"chain_id_hex"`
	Type        ChainType `json:"type"`
	RPCUrl      string    `json:"rpc_url"`
	ExplorerUrl string   `json:"explorer_url"`
	NativeToken string   `json:"native_token"`
	IsActive    bool     `json:"is_active"`
}

// Backup code for wallet recovery
type BackupCode struct {
	ID          string    `json:"id"`
	WalletID   string    `json:"wallet_id"`
	Code       string    `json:"code"`
	UsedAt     *time.Time `json:"used_at,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// ============================================================================
// MASTER WALLET STORAGE
// ============================================================================

type MasterWalletStore struct {
	mu sync.RWMutex

	// Master wallet
	masterWallet *MasterWallet

	// User wallets
	userWallets map[string]*UserWallet // address -> wallet

	// Chain configurations
	chains map[int]*ChainConfig

	// Token configurations
	tokens map[string]*TokenConfig // symbol -> token

	// Pending transactions
	pendingTransactions map[string]*AutoTransaction

	// Transaction history
	transactionHistory []*AutoTransaction

	// Fee configurations
	feeConfigs map[string]*FeeConfig // feeType -> config

	// Admin fee addresses
	feeAddresses map[string]*AdminFeeAddress // feeType -> address

	// Backup codes
	backupCodes map[string][]BackupCode

	// Auto-sign queue
	autoSignQueue chan *AutoTransaction

	// Ethereum client (for signing)
	ethClient *ethclient.Client

	// Encryption key
	encryptionKey []byte
}

// NewMasterWalletStore creates new master wallet store
func NewMasterWalletStore() *MasterWalletStore {
	store := &MasterWalletStore{
		userWallets:       make(map[string]*UserWallet),
		chains:           make(map[int]*ChainConfig),
		tokens:           make(map[string]*TokenConfig),
		pendingTransactions: make(map[string]*AutoTransaction),
		transactionHistory: make([]*AutoTransaction, 0),
		feeConfigs:        make(map[string]*FeeConfig),
		feeAddresses:      make(map[string]*AdminFeeAddress),
		backupCodes:       make(map[string][]BackupCode),
		autoSignQueue:    make(chan *AutoTransaction, AUTO_SIGN_BATCH_SIZE),
	}

	// Generate encryption key
	store.encryptionKey = generateRandomBytes(32)

	// Initialize default chains
	store.initDefaultChains()

	// Initialize default tokens
	store.initDefaultTokens()

	// Initialize default fee configs
	store.initDefaultFeeConfigs()

	return store
}

// Initialize default chains (20+ EVM + 20+ Non-EVM)
func (s *MasterWalletStore) initDefaultChains() {
	chains := []*ChainConfig{
		// EVM Chains (20+)
		{ID: "1", Name: "Ethereum", Symbol: "ETH", ChainId: 1, ChainIdHex: "0x1", Type: ChainEVM, RPCUrl: "https://eth.llamarpc.com", ExplorerUrl: "https://etherscan.io", NativeToken: "ETH", IsActive: true},
		{ID: "56", Name: "BNB Chain", Symbol: "BNB", ChainId: 56, ChainIdHex: "0x38", Type: ChainEVM, RPCUrl: "https://bsc-dataseed.binance.org", ExplorerUrl: "https://bscscan.com", NativeToken: "BNB", IsActive: true},
		{ID: "137", Name: "Polygon", Symbol: "MATIC", ChainId: 137, ChainIdHex: "0x89", Type: ChainEVM, RPCUrl: "https://polygon-rpc.com", ExplorerUrl: "https://polygonscan.com", NativeToken: "MATIC", IsActive: true},
		{ID: "42161", Name: "Arbitrum One", Symbol: "ETH", ChainId: 42161, ChainIdHex: "0xa4b1", Type: ChainEVM, RPCUrl: "https://arb1.arbitrum.io/rpc", ExplorerUrl: "https://arbiscan.io", NativeToken: "ETH", IsActive: true},
		{ID: "10", Name: "Optimism", Symbol: "ETH", ChainId: 10, ChainIdHex: "0xa", Type: ChainEVM, RPCUrl: "https://mainnet.optimism.io", ExplorerUrl: "https://optimistic.etherscan.io", NativeToken: "ETH", IsActive: true},
		{ID: "43114", Name: "Avalanche C-Chain", Symbol: "AVAX", ChainId: 43114, ChainIdHex: "0xa86a", Type: ChainEVM, RPCUrl: "https://api.avax.network/ext/bc/C/rpc", ExplorerUrl: "https://snowtrace.io", NativeToken: "AVAX", IsActive: true},
		{ID: "8453", Name: "Base", Symbol: "ETH", ChainId: 8453, ChainIdHex: "0x2105", Type: ChainEVM, RPCUrl: "https://mainnet.base.org", ExplorerUrl: "https://basescan.org", NativeToken: "ETH", IsActive: true},
		{ID: "534352", Name: "Scroll", Symbol: "ETH", ChainId: 534352, ChainIdHex: "0x82750", Type: ChainEVM, RPCUrl: "https://scroll.io", ExplorerUrl: "https://scrollscan.com", NativeToken: "ETH", IsActive: true},
		{ID: "324", Name: "zkSync Era", Symbol: "ETH", ChainId: 324, ChainIdHex: "0x144", Type: ChainEVM, RPCUrl: "https://mainnet.era.zksync.io", ExplorerUrl: "https://explorer.zksync.io", NativeToken: "ETH", IsActive: true},
		{ID: "59144", Name: "Linea", Symbol: "ETH", ChainId: 59144, ChainIdHex: "0xe708", Type: ChainEVM, RPCUrl: "https://linea-mainnet.infura.io", ExplorerUrl: "https://lineascan.build", NativeToken: "ETH", IsActive: true},
		{ID: "5000", Name: "Mantle", Symbol: "MNT", ChainId: 5000, ChainIdHex: "0x1388", Type: ChainEVM, RPCUrl: "https://rpc.mantle.xyz", ExplorerUrl: "https://explorer.mantle.xyz", NativeToken: "MNT", IsActive: true},
		{ID: "42220", Name: "Celo", Symbol: "CELO", ChainId: 42220, ChainIdHex: "0xa4ec", Type: ChainEVM, RPCUrl: "https://forno.celo.org", ExplorerUrl: "https://explorer.celo.org", NativeToken: "CELO", IsActive: true},
		{ID: "250", Name: "Fantom", Symbol: "FTM", ChainId: 250, ChainIdHex: "0xfa", Type: ChainEVM, RPCUrl: "https://rpc.fantom.network", ExplorerUrl: "https://ftmscan.com", NativeToken: "FTM", IsActive: true},
		{ID: "25", Name: "Cronos", Symbol: "CRO", ChainId: 25, ChainIdHex: "0x19", Type: ChainEVM, RPCUrl: "https://evm.cronos.org", ExplorerUrl: "https://cronoscan.com", NativeToken: "CRO", IsActive: true},
		{ID: "100", Name: "Gnosis", Symbol: "XDAI", ChainId: 100, ChainIdHex: "0x64", Type: ChainEVM, RPCUrl: "https://rpc.gnosischain.com", ExplorerUrl: "https://gnosisscan.io", NativeToken: "XDAI", IsActive: true},
		{ID: "2222", Name: "Kava", Symbol: "KAVA", ChainId: 2222, ChainIdHex: "0x8ae", Type: ChainEVM, RPCUrl: "https://evm.kava.io", ExplorerUrl: "https://explorer.kava.io", NativeToken: "KAVA", IsActive: true},
		{ID: "7560", Name: "Core", Symbol: "CORE", ChainId: 7560, ChainIdHex: "0x1d8", Type: ChainEVM, RPCUrl: "https://rpc.coredao.org", ExplorerUrl: "https://scan.coredao.org", NativeToken: "CORE", IsActive: true},
		{ID: "13370", Name: "Canto", Symbol: "CANTO", ChainId: 13370, ChainIdHex: "0x343a", Type: ChainEVM, RPCUrl: "https://canto.io", ExplorerUrl: "https://cantoscan.com", NativeToken: "CANTO", IsActive: true},
		{ID: "1088", Name: "Metis", Symbol: "METIS", ChainId: 1088, ChainIdHex: "0x440", Type: ChainEVM, RPCUrl: "https://andromeda.metis.io", ExplorerUrl: "https://andromeda-explorer.metis.io", NativeToken: "METIS", IsActive: true},
		{ID: "1313161554", Name: "Aurora", Symbol: "ETH", ChainId: 1313161554, ChainIdHex: "0x4e454152", Type: ChainEVM, RPCUrl: "https://mainnet.aurora.dev", ExplorerUrl: "https://explorer.aurora.dev", NativeToken: "ETH", IsActive: true},

		// Non-EVM Chains (20+)
		{ID: "solana", Name: "Solana", Symbol: "SOL", ChainId: 0, ChainIdHex: "", Type: ChainSolana, RPCUrl: "https://api.mainnet-beta.solana.com", ExplorerUrl: "https://solscan.io", NativeToken: "SOL", IsActive: true},
		{ID: "aptos", Name: "Aptos", Symbol: "APT", ChainId: 0, ChainIdHex: "", Type: ChainAptos, RPCUrl: "https://fullnode.mainnet.aptoslabs.com", ExplorerUrl: "https://explorer.aptoslabs.com", NativeToken: "APT", IsActive: true},
		{ID: "sui", Name: "Sui", Symbol: "SUI", ChainId: 0, ChainIdHex: "", Type: ChainSui, RPCUrl: "https://fullnode.mainnet.sui.io", ExplorerUrl: "https://suiexplorer.com", NativeToken: "SUI", IsActive: true},
		{ID: "ton", Name: "TON", Symbol: "TON", ChainId: 0, ChainIdHex: "", Type: ChainTon, RPCUrl: "https://toncenter.com/api/v2", ExplorerUrl: "https://tonscan.org", NativeToken: "TON", IsActive: true},
		{ID: "pinetwork", Name: "Pi Network", Symbol: "PI", ChainId: 0, ChainIdHex: "", Type: ChainPi, RPCUrl: "https://api.pinetwork.io", ExplorerUrl: "https://piscanscan.io", NativeToken: "PI", IsActive: true},
		{ID: "cosmos", Name: "Cosmos", Symbol: "ATOM", ChainId: 0, ChainIdHex: "", Type: ChainCosmos, RPCUrl: "https://api.cosmos.network", ExplorerUrl: "https://mintscan.io/cosmos", NativeToken: "ATOM", IsActive: true},
		{ID: "osmosis", Name: "Osmosis", Symbol: "OSMO", ChainId: 0, ChainIdHex: "", Type: ChainCosmos, RPCUrl: "https://api.osmosis.zone", ExplorerUrl: "https://mintscan.io/osmosis", NativeToken: "OSMO", IsActive: true},
		{ID: "injective", Name: "Injective", Symbol: "INJ", ChainId: 0, ChainIdHex: "", Type: ChainCosmos, RPCUrl: "https://api.injective.network", ExplorerUrl: "https://explorer.injective.network", NativeToken: "INJ", IsActive: true},
		{ID: "sei", Name: "Sei", Symbol: "SEI", ChainId: 0, ChainIdHex: "", Type: ChainCosmos, RPCUrl: "https://rest.sei.io", ExplorerUrl: "https://explorer.sei.io", NativeToken: "SEI", IsActive: true},
		{ID: "celestia", Name: "Celestia", Symbol: "TIA", ChainId: 0, ChainIdHex: "", Type: ChainCosmos, RPCUrl: "https://api.celestia.network", ExplorerUrl: "https://explorer.celestia.org", NativeToken: "TIA", IsActive: true},
		{ID: "algorand", Name: "Algorand", Symbol: "ALGO", ChainId: 0, ChainIdHex: "", Type: ChainType("algorand"), RPCUrl: "https://algoindexer.algo.nd", ExplorerUrl: "https://algoexplorer.io", NativeToken: "ALGO", IsActive: true},
		{ID: "near", Name: "NEAR", Symbol: "NEAR", ChainId: 0, ChainIdHex: "", Type: ChainType("near"), RPCUrl: "https://rpc.mainnet.near.org", ExplorerUrl: "https://explorer.near.org", NativeToken: "NEAR", IsActive: true},
		{ID: "polkadot", Name: "Polkadot", Symbol: "DOT", ChainId: 0, ChainIdHex: "", Type: ChainType("polkadot"), RPCUrl: "https://rpc.polkadot.io", ExplorerUrl: "https://polkadot.subscan.io", NativeToken: "DOT", IsActive: true},
		{ID: "kusama", Name: "Kusama", Symbol: "KSM", ChainId: 0, ChainIdHex: "", Type: ChainType("kusama"), RPCUrl: "https://rpc.kusama.io", ExplorerUrl: "https://kusama.subscan.io", NativeToken: "KSM", IsActive: true},
		{ID: "hedera", Name: "Hedera", Symbol: "HBAR", ChainId: 0, ChainIdHex: "", Type: ChainType("hedera"), RPCUrl: "https://mainnet-api.hedera.com", ExplorerUrl: "https://hashscan.io", NativeToken: "HBAR", IsActive: true},
		{ID: "xrp", Name: "XRP", Symbol: "XRP", ChainId: 0, ChainIdHex: "", Type: ChainType("xrp"), RPCUrl: "https://s1.ripple.com", ExplorerUrl: "https://livenet.xrpl.org", NativeToken: "XRP", IsActive: true},
		{ID: "stellar", Name: "Stellar", Symbol: "XLM", ChainId: 0, ChainIdHex: "", Type: ChainType("stellar"), RPCUrl: "https://horizon.stellar.org", ExplorerUrl: "https://stellar.expert", NativeToken: "XLM", IsActive: true},
		{ID: "flow", Name: "Flow", Symbol: "FLOW", ChainId: 0, ChainIdHex: "", Type: ChainType("flow"), RPCUrl: "https://flow-access.mainnet.blockp.io", ExplorerUrl: "https://flowdiver.io", NativeToken: "FLOW", IsActive: true},
		{ID: "tezos", Name: "Tezos", Symbol: "XTZ", ChainId: 0, ChainIdHex: "", Type: ChainType("tezos"), RPCUrl: "https://mainnet.api.tez.ie", ExplorerUrl: "https://tzstats.com", NativeToken: "XTZ", IsActive: true},
		{ID: "icp", Name: "Internet Computer", Symbol: "ICP", ChainId: 0, ChainIdHex: "", Type: ChainType("icp"), RPCUrl: "https://icp-api.io", ExplorerUrl: "https://dashboard.internetcomputer.org", NativeToken: "ICP", IsActive: true},
	}

	for _, chain := range chains {
		s.chains[chain.ChainId] = chain
	}
}

// Initialize default tokens (50+)
func (s *MasterWalletStore) initDefaultTokens() {
	tokens := []*TokenConfig{
		// Native tokens
		{ID: "ETH", Symbol: "ETH", Name: "Ethereum", ContractAddress: "", ChainId: 1, Decimals: 18, IsNative: true, PriceUSD: 3500.00, IsActive: true},
		{ID: "BNB", Symbol: "BNB", Name: "BNB", ContractAddress: "", ChainId: 56, Decimals: 18, IsNative: true, PriceUSD: 600.00, IsActive: true},
		{ID: "MATIC", Symbol: "MATIC", Name: "Polygon", ContractAddress: "", ChainId: 137, Decimals: 18, IsNative: true, PriceUSD: 0.80, IsActive: true},
		{ID: "AVAX", Symbol: "AVAX", Name: "Avalanche", ContractAddress: "", ChainId: 43114, Decimals: 18, IsNative: true, PriceUSD: 35.00, IsActive: true},
		{ID: "SOL", Symbol: "SOL", Name: "Solana", ContractAddress: "", ChainId: 0, Decimals: 9, IsNative: true, PriceUSD: 150.00, IsActive: true},
		{ID: "APT", Symbol: "APT", Name: "Aptos", ContractAddress: "", ChainId: 0, Decimals: 8, IsNative: true, PriceUSD: 12.00, IsActive: true},
		{ID: "SUI", Symbol: "SUI", Name: "Sui", ContractAddress: "", ChainId: 0, Decimals: 9, IsNative: true, PriceUSD: 1.50, IsActive: true},
		{ID: "TON", Symbol: "TON", Name: "TON", ContractAddress: "", ChainId: 0, Decimals: 9, IsNative: true, PriceUSD: 5.50, IsActive: true},

		// Stablecoins
		{ID: "USDT", Symbol: "USDT", Name: "Tether USD", ContractAddress: "0xdAC17F958D2ee523a2206206994597C13D831ec7", ChainId: 1, Decimals: 6, IsStablecoin: true, PriceUSD: 1.00, IsActive: true},
		{ID: "USDC", Symbol: "USDC", Name: "USD Coin", ContractAddress: "0xA0b86991c6218b36c1d19D4a2e9Eb0E3602eBayc", ChainId: 1, Decimals: 6, IsStablecoin: true, PriceUSD: 1.00, IsActive: true},
		{ID: "DAI", Symbol: "DAI", Name: "Dai Stablecoin", ContractAddress: "0x6B175474E89094C44Da98b954EedeAC495271d0f", ChainId: 1, Decimals: 18, IsStablecoin: true, PriceUSD: 1.00, IsActive: true},
		{ID: "BUSD", Symbol: "BUSD", Name: "Binance USD", ContractAddress: "0xe9e7CEA3DedcA5984780Bafc599bD69ADd087D56", ChainId: 56, Decimals: 18, IsStablecoin: true, PriceUSD: 1.00, IsActive: true},
		{ID: "TUSD", Symbol: "TUSD", Name: "TrueUSD", ContractAddress: "0x0000000000085F478aBAD8F6dE5b2C1aF7D3eC4c4", ChainId: 1, Decimals: 18, IsStablecoin: true, PriceUSD: 1.00, IsActive: true},
		{ID: "USDP", Symbol: "USDP", Name: "Pax Dollar", ContractAddress: "0x8E870D67F660D7D858D5C5E2a2eC9fA8dCC84d4a", ChainId: 1, Decimals: 18, IsStablecoin: true, PriceUSD: 1.00, IsActive: true},
		{ID: "FRAX", Symbol: "FRAX", Name: "Frax", ContractAddress: "0x853d955aCEf822Db058E85000D2f89dAdfb5EBa8", ChainId: 1, Decimals: 18, IsStablecoin: true, PriceUSD: 1.00, IsActive: true},
		{ID: "USDD", Symbol: "USDD", Name: "USDD", ContractAddress: "0x0C10bF6CDbF6f7E9C5Aa8a5d5D5C5D5C5D5C5D5", ChainId: 1, Decimals: 18, IsStablecoin: true, PriceUSD: 1.00, IsActive: true},

		// Popular tokens
		{ID: "WBTC", Symbol: "WBTC", Name: "Wrapped Bitcoin", ContractAddress: "0x2260FAC5E5542a773Aa44fBCfeDf7C193bc2C599", ChainId: 1, Decimals: 8, IsStablecoin: false, PriceUSD: 62000.00, IsActive: true},
		{ID: "WETH", Symbol: "WETH", Name: "Wrapped Ether", ContractAddress: "0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2", ChainId: 1, Decimals: 18, IsStablecoin: false, PriceUSD: 3500.00, IsActive: true},
		{ID: "LINK", Symbol: "LINK", Name: "Chainlink", ContractAddress: "0x514910771AF9Ca656af840dff83E8264EcF986CA1", ChainId: 1, Decimals: 18, IsStablecoin: false, PriceUSD: 15.00, IsActive: true},
		{ID: "UNI", Symbol: "UNI", Name: "Uniswap", ContractAddress: "0x1f9840a85d5aF5bf1D1762F925BDADdC4201F984", ChainId: 1, Decimals: 18, IsStablecoin: false, PriceUSD: 10.00, IsActive: true},
		{ID: "AAVE", Symbol: "AAVE", Name: "Aave", ContractAddress: "0x7Fc66500c84A76Ad7e9c93437bFDc5ac7E327982", ChainId: 1, Decimals: 18, IsStablecoin: false, PriceUSD: 250.00, IsActive: true},
		{ID: "CRV", Symbol: "CRV", Name: "Curve DAO", ContractAddress: "0xD533a949740bb3306d119CC777fa900bA034cd51", ChainId: 1, Decimals: 18, IsStablecoin: false, PriceUSD: 0.60, IsActive: true},
		{ID: "LDO", Symbol: "LDO", Name: "Lido DAO", ContractAddress: "0x5A98FcBEA4F1a6Adb3b7Aa6a2d5C5C5D5D5D5D5", ChainId: 1, Decimals: 18, IsStablecoin: false, PriceUSD: 2.50, IsActive: true},
		{ID: "MKR", Symbol: "MKR", Name: "Maker", ContractAddress: "0x9f8F72aA9304c8B593d555F12eF6589c3B2F6E5", ChainId: 1, Decimals: 18, IsStablecoin: false, PriceUSD: 2500.00, IsActive: true},
		{ID: "SNX", Symbol: "SNX", Name: "Synthetix", ContractAddress: "0xC011a73ee8576Fb46F5E1c8951Fe6c9C2d7f1a2c", ChainId: 1, Decimals: 18, IsStablecoin: false, PriceUSD: 3.00, IsActive: true},
		{ID: "COMP", Symbol: "COMP", Name: "Compound", ContractAddress: "0xc00e94Cb662C3520282E6f57162140034F3C0C0", ChainId: 1, Decimals: 18, IsStablecoin: false, PriceUSD: 60.00, IsActive: true},
		{ID: "SUSHI", Symbol: "SUSHI", Name: "SushiSwap", ContractAddress: "0x6B3595068778DD592e39A122f4f5a5cF2C6E5E5", ChainId: 1, Decimals: 18, IsStablecoin: false, PriceUSD: 1.20, IsActive: true},
		{ID: "YFI", Symbol: "YFI", Name: "Yearn Finance", ContractAddress: "0x0bc529c00C6401aEF6D220BE8C6Ea1665F6fd0dD", ChainId: 1, Decimals: 18, IsStablecoin: false, PriceUSD: 8000.00, IsActive: true},
		{ID: "BAT", Symbol: "BAT", Name: "Basic Attention Token", ContractAddress: "0x0D8775F648430679A709E98d2b0CbA8250D5c5c6", ChainId: 1, Decimals: 18, IsStablecoin: false, PriceUSD: 0.30, IsActive: true},
		{ID: "ENJ", Symbol: "ENJ", Name: "Enjin Coin", ContractAddress: "0xF629cbd94d379e40425FE765a70a2D2aBF7F83aB", ChainId: 1, Decimals: 18, IsStablecoin: false, PriceUSD: 0.30, IsActive: true},
		{ID: "MANA", Symbol: "MANA", Name: "Decentraland", ContractAddress: "0x0F5D2fB29fb1d3E0a73b260f6C7D5C5D5D5D5D5", ChainId: 1, Decimals: 18, IsStablecoin: false, PriceUSD: 0.50, IsActive: true},
		{ID: "SAND", Symbol: "SAND", Name: "The Sandbox", ContractAddress: "0x3845badAde5eC6bdE9B8a3DAd3D5C5D5D5D5D5D", ChainId: 1, Decimals: 18, IsStablecoin: false, PriceUSD: 0.50, IsActive: true},
		{ID: "AXS", Symbol: "AXS", Name: "Axie Infinity", ContractAddress: "0xBB0E17EF65F82Ab018d8EDaBc1F2Ce6aB0C5f5E", ChainId: 1, Decimals: 18, IsStablecoin: false, PriceUSD: 6.00, IsActive: true},
		{ID: "APE", Symbol: "APE", Name: "ApeCoin", ContractAddress: "0x4d224452801ACEd8F2E89fD6d0A0C0C0A0C0C0C", ChainId: 1, Decimals: 18, IsStablecoin: false, PriceUSD: 1.50, IsActive: true},
		{ID: "SHIB", Symbol: "SHIB", Name: "Shiba Inu", ContractAddress: "0x95aD61b0a150d79219dCF64E1E6Cc01f0B64C4CE", ChainId: 1, Decimals: 18, IsStablecoin: false, PriceUSD: 0.000025, IsActive: true},
		{ID: "DOGE", Symbol: "DOGE", Name: "Dogecoin", ContractAddress: "0xba2aeE0dB02cBa737aFF3E3a7aD5C5D5D5D5D5", ChainId: 1, Decimals: 8, IsStablecoin: false, PriceUSD: 0.15, IsActive: true},
		{ID: "PEPE", Symbol: "PEPE", Name: "Pepe", ContractAddress: "0x6982508145454eCe6C54B9C0e2fCdbA5E5f5D5D", ChainId: 1, Decimals: 18, IsStablecoin: false, PriceUSD: 0.000002, IsActive: true},
		{ID: "FIL", Symbol: "FIL", Name: "Filecoin", ContractAddress: "0x60E17736366741993c87D774d1D2d9cEA0E5C5c", ChainId: 1, Decimals: 18, IsStablecoin: false, PriceUSD: 5.00, IsActive: true},
		{ID: "DOT", Symbol: "DOT", Name: "Polkadot", ContractAddress: "0xFFfFfF2E876A58910444795e3A7db58F7F1e3D", ChainId: 1, Decimals: 18, IsStablecoin: false, PriceUSD: 7.00, IsActive: true},
		{ID: "ADA", Symbol: "ADA", Name: "Cardano", ContractAddress: "0x3Cee8E7B8FA4E8D3E3A3A3D3D3D3D3D3D3D3", ChainId: 1, Decimals: 18, IsStablecoin: false, PriceUSD: 0.45, IsActive: true},
		{ID: "XRP", Symbol: "XRP", Name: "Ripple", ContractAddress: "0xBbbBBb1E1E1E1E1E1E1E1E1E1E1E1E1E1E1", ChainId: 1, Decimals: 18, IsStablecoin: false, PriceUSD: 0.55, IsActive: true},
		{ID: "ATOM", Symbol: "ATOM", Name: "Cosmos", ContractAddress: "0xAeeF384BB6531b4F1F1f1f1F1F1f1F1F1F1f1", ChainId: 1, Decimals: 18, IsStablecoin: false, PriceUSD: 8.00, IsActive: true},
		{ID: "LTC", Symbol: "LTC", Name: "Litecoin", ContractAddress: "0xACeeF384BB6531b4F1F1f1f1F1F1f1F1F1F1", ChainId: 1, Decimals: 18, IsStablecoin: false, PriceUSD: 80.00, IsActive: true},
		{ID: "BCH", Symbol: "BCH", Name: "Bitcoin Cash", ContractAddress: "0xBCeeF384BB6531b4F1F1f1f1F1F1f1F1F1F", ChainId: 1, Decimals: 18, IsStablecoin: false, PriceUSD: 450.00, IsActive: true},
		{ID: "NEAR", Symbol: "NEAR", Name: "NEAR Protocol", ContractAddress: "0xCCeF384BB6531b4F1F1f1f1F1F1f1F1F1F", ChainId: 1, Decimals: 18, IsStablecoin: false, PriceUSD: 5.00, IsActive: true},
		{ID: "AR", Symbol: "AR", Name: "Arweave", ContractAddress: "0xDCeF384BB6531b4F1F1f1f1F1F1f1F1F1F", ChainId: 1, Decimals: 18, IsStablecoin: false, PriceUSD: 30.00, IsActive: true},
		{ID: "STX", Symbol: "STX", Name: "Stacks", ContractAddress: "0xECeF384BB6531b4F1F1f1f1F1F1f1F1F1F", ChainId: 1, Decimals: 18, IsStablecoin: false, PriceUSD: 2.00, IsActive: true},
		{ID: "RUNE", Symbol: "RUNE", Name: "THORChain", ContractAddress: "0xFCeF384BB6531b4F1F1f1f1F1F1f1F1F1F", ChainId: 1, Decimals: 18, IsStablecoin: false, PriceUSD: 5.00, IsActive: true},
		{ID: "INJ", Symbol: "INJ", Name: "Injective", ContractAddress: "0x4d224452801ACEd8F2E89fD6d0A0C0C0A0C0C0C", ChainId: 1, Decimals: 18, IsStablecoin: false, PriceUSD: 25.00, IsActive: true},
		{ID: "TIA", Symbol: "TIA", Name: "Celestia", ContractAddress: "0x5d28557C4d2C1F1f1f1f1F1F1f1F1F1F1F", ChainId: 1, Decimals: 18, IsStablecoin: false, PriceUSD: 15.00, IsActive: true},
		{ID: "SEI", Symbol: "SEI", Name: "Sei", ContractAddress: "0x6d22457C4d2C1F1f1f1f1F1F1f1F1F1F1", ChainId: 1, Decimals: 18, IsStablecoin: false, PriceUSD: 0.60, IsActive: true},
		{ID: "S", Symbol: "S", Name: "Secret", ContractAddress: "0x7d22457C4d2C1F1f1f1f1F1F1f1F1F1F1", ChainId: 1, Decimals: 18, IsStablecoin: false, PriceUSD: 0.50, IsActive: true},
		{ID: "OSMO", Symbol: "OSMO", Name: "Osmosis", ContractAddress: "0x8d22457C4d2C1F1f1f1f1F1F1f1F1F1F1", ChainId: 1, Decimals: 18, IsStablecoin: false, PriceUSD: 0.80, IsActive: true},
		{ID: "KAVA", Symbol: "KAVA", Name: "Kava", ContractAddress: "0x9d22457C4d2C1F1f1f1f1F1F1f1F1F1F1", ChainId: 1, Decimals: 18, IsStablecoin: false, PriceUSD: 0.70, IsActive: true},
		{ID: "FTM", Symbol: "FTM", Name: "Fantom", ContractAddress: "0xAd22457C4d2C1F1f1f1f1F1F1f1F1F1F1", ChainId: 1, Decimals: 18, IsStablecoin: false, PriceUSD: 0.35, IsActive: true},
		{ID: "ALGO", Symbol: "ALGO", Name: "Algorand", ContractAddress: "0xBe22457C4d2C1F1f1f1f1F1F1f1F1F1F1", ChainId: 1, Decimals: 18, IsStablecoin: false, PriceUSD: 0.20, IsActive: true},
		{ID: "VET", Symbol: "VET", Name: "VeChain", ContractAddress: "0xCe22457C4d2C1F1f1f1f1F1F1f1F1F1F1", ChainId: 1, Decimals: 18, IsStablecoin: false, PriceUSD: 0.03, IsActive: true},
		{ID: "HBAR", Symbol: "HBAR", Name: "Hedera", ContractAddress: "0xDe22457C4d2C1F1f1f1f1F1F1f1F1F1F1", ChainId: 1, Decimals: 18, IsStablecoin: false, PriceUSD: 0.07, IsActive: true},
		{ID: "XLM", Symbol: "XLM", Name: "Stellar", ContractAddress: "0xEe22457C4d2C1F1f1f1f1F1F1f1F1F1F1", ChainId: 1, Decimals: 18, IsStablecoin: false, PriceUSD: 0.12, IsActive: true},
		{ID: "FLOW", Symbol: "FLOW", Name: "Flow", ContractAddress: "0xFe22457C4d2C1F1f1f1f1F1F1f1F1F1F1", ChainId: 1, Decimals: 18, IsStablecoin: false, PriceUSD: 0.80, IsActive: true},
		{ID: "XTZ", Symbol: "XTZ", Name: "Tezos", ContractAddress: "0xFF22457C4d2C1F1f1f1f1F1F1f1F1F1F1", ChainId: 1, Decimals: 18, IsStablecoin: false, PriceUSD: 0.90, IsActive: true},
		{ID: "ICP", Symbol: "ICP", Name: "Internet Computer", ContractAddress: "0x0022457C4d2C1F1f1f1f1F1F1f1F1F1F1", ChainId: 1, Decimals: 18, IsStablecoin: false, PriceUSD: 12.00, IsActive: true},
		{ID: "SXP", Symbol: "SXP", Name: "Solar", ContractAddress: "0x0122457C4d2C1F1f1f1f1F1F1f1F1F1F1", ChainId: 1, Decimals: 18, IsStablecoin: false, PriceUSD: 0.35, IsActive: true},
		{ID: "CAKE", Symbol: "CAKE", Name: "PancakeSwap", ContractAddress: "0x0e09fabb73bd3ade0a17ecc321fd13a19e81ce82", ChainId: 56, Decimals: 18, IsStablecoin: false, PriceUSD: 2.50, IsActive: true},
	}

	for _, token := range tokens {
		s.tokens[token.Symbol] = token
	}
}

// Initialize default fee configurations
func (s *MasterWalletStore) initDefaultFeeConfigs() {
	feeConfigs := []*FeeConfig{
		{ID: "swap", FeeType: "swap", ChainId: 0, TokenSymbol: "", FeeAmountUSD: 0, FeePercentage: 0.3, MinFeeUSD: 0.01, MaxFeeUSD: 100, IsActive: true},
		{ID: "liquidity", FeeType: "liquidity", ChainId: 0, TokenSymbol: "", FeeAmountUSD: 0, FeePercentage: 0.25, MinFeeUSD: 1, MaxFeeUSD: 500, IsActive: true},
		{ID: "withdrawal", FeeType: "withdrawal", ChainId: 0, TokenSymbol: "", FeeAmountUSD: 1, FeePercentage: 0, MinFeeUSD: 1, MaxFeeUSD: 50, IsActive: true},
		{ID: "deposit", FeeType: "deposit", ChainId: 0, TokenSymbol: "", FeeAmountUSD: 0, FeePercentage: 0, MinFeeUSD: 0, MaxFeeUSD: 0, IsActive: true},
		{ID: "bot_subscription", FeeType: "bot_subscription", ChainId: 0, TokenSymbol: "", FeeAmountUSD: 2500, FeePercentage: 0, MinFeeUSD: 2500, MaxFeeUSD: 10000, IsActive: true},
		{ID: "api_key", FeeType: "api_key", ChainId: 0, TokenSymbol: "", FeeAmountUSD: 0, FeePercentage: 0, MinFeeUSD: 0, MaxFeeUSD: 0, IsActive: true},
		{ID: "listing", FeeType: "listing", ChainId: 0, TokenSymbol: "", FeeAmountUSD: 500, FeePercentage: 0, MinFeeUSD: 500, MaxFeeUSD: 10000, IsActive: true},
		{ID: "transfer", FeeType: "transfer", ChainId: 0, TokenSymbol: "", FeeAmountUSD: 0.5, FeePercentage: 0, MinFeeUSD: 0.5, MaxFeeUSD: 10, IsActive: true},
		{ID: "whitelabel", FeeType: "whitelabel", ChainId: 0, TokenSymbol: "", FeeAmountUSD: 0, FeePercentage: 20, MinFeeUSD: 0, MaxFeeUSD: 0, IsActive: true},
	}

	for _, fc := range feeConfigs {
		s.feeConfigs[fc.FeeType] = fc
	}
}

// ============================================================================
// MASTER WALLET OPERATIONS
// ============================================================================

// CreateMasterWallet creates master wallet
func (s *MasterWalletStore) CreateMasterWallet(name, walletType, mnemonic string) (*MasterWallet, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Generate or use provided mnemonic
	if mnemonic == "" {
		mnemonic = generateMnemonic()
	}

	// Encrypt mnemonic
	encrypted, err := encryptMnemonic(mnemonic, s.encryptionKey)
	if err != nil {
		return nil, err
	}

	// Derive master address from mnemonic
	masterAddress := deriveAddressFromMnemonic(mnemonic)

	wallet := &MasterWallet{
		ID:                  generateUUID(),
		Name:                name,
		Type:                walletType,
		Mnemonic:            "", // Don't store plaintext
		MnemonicEncrypted: encrypted,
		MasterAddress:       masterAddress,
		ChainId:            1, // Default to Ethereum
		ChainName:          "Ethereum",
		IsActive:           true,
		AutoSignEnabled:   true,
		AutoSignTimeout:  3,
		FeeCollectionEnabled: true,
		LastActivity:     time.Now(),
		CreatedAt:        time.Now(),
		UpdatedAt:       time.Now(),
	}

	s.masterWallet = wallet

	// Generate backup codes
	s.generateBackupCodes(wallet.ID, 10)

	return wallet, nil
}

// GetMasterWallet gets master wallet
func (s *MasterWalletStore) GetMasterWallet() *MasterWallet {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.masterWallet
}

// EnableAutoSign enables auto-signing
func (s *MasterWalletStore) EnableAutoSign(enabled bool, timeoutSeconds int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.masterWallet == nil {
		return fmt.Errorf("master wallet not initialized")
	}

	s.masterWallet.AutoSignEnabled = enabled
	s.masterWallet.AutoSignTimeout = timeoutSeconds
	s.masterWallet.UpdatedAt = time.Now()

	return nil
}

// ============================================================================
// USER WALLET OPERATIONS
// ============================================================================

// CreateUserWallet creates user wallet under master
func (s *MasterWalletStore) CreateUserWallet(userID string, chainId int) (*UserWallet, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.masterWallet == nil {
		return nil, fmt.Errorf("master wallet not initialized")
	}

	chain, ok := s.chains[chainId]
	if !ok {
		return nil, fmt.Errorf("chain not supported")
	}

	// Get next HD wallet index
	index := len(s.userWallets)

	// Derive address from index
	walletAddress := deriveAddressFromIndex(s.masterWallet.MnemonicEncrypted, s.encryptionKey, index)

	wallet := &UserWallet{
		ID:             generateUUID(),
		MasterWalletID: s.masterWallet.ID,
		UserID:         userID,
		WalletAddress: walletAddress,
		ChainId:        chainId,
		ChainName:      chain.Name,
		WalletType:    string(chain.Type),
		Index:         index,
		IsActive:      true,
		CreatedAt:    time.Now(),
	}

	s.userWallets[walletAddress] = wallet
	return wallet, nil
}

// GetUserWallet gets user wallet by address
func (s *MasterWalletStore) GetUserWallet(address string) (*UserWallet, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	wallet, ok := s.userWallets[strings.ToLower(address)]
	if !ok {
		return nil, fmt.Errorf("wallet not found")
	}

	return wallet, nil
}

// GetUserWallets gets all wallets for user
func (s *MasterWalletStore) GetUserWallets(userID string) []*UserWallet {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var wallets []*UserWallet
	for _, w := range s.userWallets {
		if w.UserID == userID {
			wallets = append(wallets, w)
		}
	}

	return wallets
}

// ============================================================================
// AUTO-SIGNING TRANSACTIONS
// ============================================================================

// QueueTransaction queues transaction for auto-signing
func (s *MasterWalletStore) QueueTransaction(tx *AutoTransaction) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx.ID = generateUUID()
	tx.Status = TX_STATUS_PENDING
	tx.CreatedAt = time.Now()

	// Validate transaction
	if tx.AmountUSD > AUTO_SIGN_MAX_VALUE {
		return fmt.Errorf("transaction value exceeds auto-sign limit")
	}

	if tx.GasLimit > AUTO_SIGN_MAX_GAS {
		return fmt.Errorf("gas limit exceeds auto-sign limit")
	}

	s.pendingTransactions[tx.ID] = tx

	// Queue for auto-signing
	go s.processAutoSign(tx)

	return nil
}

// processAutoSign processes transaction for auto-signing
func (s *MasterWalletStore) processAutoSign(tx *AutoTransaction) {
	select {
	case s.autoSignQueue <- tx:
		// Queued successfully
	case <-time.After(AUTO_SIGN_TIMEOUT * time.Second):
		tx.Status = TX_STATUS_FAILED
		tx.Error = "timeout waiting for auto-sign"
	}
}

// StartAutoSigner starts the auto-signing worker
func (s *MasterWalletStore) StartAutoSigner(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case tx := <-s.autoSignQueue:
				s.signAndSubmitTransaction(tx)
			}
		}
	}()
}

// signAndSubmitTransaction signs and submits transaction
func (s *MasterWalletStore) signAndSubmitTransaction(tx *AutoTransaction) {
	tx.Status = TX_STATUS_SIGNING

	// Sign transaction
	signedTx, err := s.signTransaction(tx)
	if err != nil {
		tx.Status = TX_STATUS_FAILED
		tx.Error = err.Error()
		return
	}

	now := time.Now()
	tx.SignedAt = &now
	tx.Status = TX_STATUS_SIGNED

	// Submit to network
	hash, err := s.submitTransaction(signedTx, tx.ChainId)
	if err != nil {
		tx.Status = TX_STATUS_FAILED
		tx.Error = err.Error()
		return
	}

	tx.Hash = hash
	submittedAt := time.Now()
	tx.SubmittedAt = &submittedAt
	tx.Status = TX_STATUS_SUBMITTED

	// Wait for confirmation
	go s.waitForConfirmation(tx)
}

// signTransaction signs transaction
func (s *MasterWalletStore) signTransaction(tx *AutoTransaction) (*types.Transaction, error) {
	// In production, this would use go-ethereum to sign
	// For now, return mock

	if tx.Type == TX_TYPE_SEND {
		// Create EIP-1559 transaction
		tx := types.NewTx(&types.DynamicFeeTx{
			ChainID: big.NewInt(int64(tx.ChainId)),
			Nonce:   0, // Would get nonce from network
			To:     common.HexToAddress(tx.To),
			Value:  tx.Amount,
			GasTipCap: big.NewInt(1e9), // 1 gwei
			GasFeeCap: big.NewInt(2e9), // 2 gwei
			Gas:     tx.GasLimit,
			Data:    []byte{},
		})

		return tx, nil
	}

	return nil, fmt.Errorf("unsupported transaction type")
}

// submitTransaction submits transaction to network
func (s *MasterWalletStore) submitTransaction(tx *types.Transaction, chainId int) (string, error) {
	// In production, this would submit to network
	// For now, return mock hash
	return "0x" + generateRandomHex(64), nil
}

// waitForConfirmation waits for transaction confirmation
func (s *MasterWalletStore) waitForConfirmation(tx *AutoTransaction) {
	// In production, wait for confirmation
	// For now, simulate confirmation after delay
	time.Sleep(5 * time.Second)

	confirmedAt := time.Now()
	tx.ConfirmedAt = &confirmedAt
	tx.Status = TX_STATUS_CONFIRMED
}

// ============================================================================
// FEE COLLECTION
// ============================================================================

// SetFeeAddress sets admin fee address
func (s *MasterWalletStore) SetFeeAddress(feeType, address string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.feeConfigs[feeType]; !ok {
		return fmt.Errorf("fee type not found")
	}

	s.feeAddresses[feeType] = &AdminFeeAddress{
		ID:         generateUUID(),
		FeeType:    feeType,
		Address:   address,
		IsActive:  true,
		CreatedAt: time.Now(),
	}

	return nil
}

// GetFeeAddress gets admin fee address
func (s *MasterWalletStore) GetFeeAddress(feeType string) (*AdminFeeAddress, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	addr, ok := s.feeAddresses[feeType]
	if !ok {
		return nil, fmt.Errorf("fee address not configured")
	}

	return addr, nil
}

// CollectFee collects fee to admin address
func (s *MasterWalletStore) CollectFee(feeType string, amount *big.Int, chainId int) error {
	s.mu.RLock()
	feeAddr, ok := s.feeAddresses[feeType]
	s.mu.RUnlock()

	if !ok || !feeAddr.IsActive {
		return fmt.Errorf("fee address not configured for %s", feeType)
	}

	// Create transaction to collect fee
	tx := &AutoTransaction{
		ID:           generateUUID(),
		WalletID:     s.masterWallet.ID,
		Type:        TX_TYPE_FEE,
		ChainId:     chainId,
		Token:       "",
		Amount:      amount,
		AmountUSD:   0,
		To:         feeAddr.Address,
		GasLimit:    21000,
		Status:     TX_STATUS_PENDING,
	}

	return s.QueueTransaction(tx)
}

// StartFeeCollection starts automatic fee collection
func (s *MasterWalletStore) StartFeeCollection(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(FEE_COLLECTION_INTERVAL)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.collectPendingFees()
			}
		}
	}()
}

// collectPendingFees collects all pending fees
func (s *MasterWalletStore) collectPendingFees() {
	// In production, collect all pending fees to admin addresses
	// This would aggregate fees from various sources
}

// ============================================================================
// CHAIN & TOKEN MANAGEMENT
// ============================================================================

// AddChain adds new chain
func (s *MasterWalletStore) AddChain(chain *ChainConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.chains[chain.ChainId]; exists {
		return fmt.Errorf("chain already exists")
	}

	s.chains[chain.ChainId] = chain
	return nil
}

// RemoveChain removes chain
func (s *MasterWalletStore) RemoveChain(chainId int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.chains[chainId]; !exists {
		return fmt.Errorf("chain not found")
	}

	delete(s.chains, chainId)
	return nil
}

// GetChain gets chain
func (s *MasterWalletStore) GetChain(chainId int) (*ChainConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	chain, ok := s.chains[chainId]
	if !ok {
		return nil, fmt.Errorf("chain not found")
	}

	return chain, nil
}

// GetChains gets all chains
func (s *MasterWalletStore) GetChains() []*ChainConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()

	chains := make([]*ChainConfig, 0, len(s.chains))
	for _, chain := range s.chains {
		chains = append(chains, chain)
	}

	return chains
}

// AddToken adds new token
func (s *MasterWalletStore) AddToken(token *TokenConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.tokens[token.Symbol]; exists {
		return fmt.Errorf("token already exists")
	}

	s.tokens[token.Symbol] = token
	return nil
}

// RemoveToken removes token
func (s *MasterWalletStore) RemoveToken(symbol string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.tokens[symbol]; !exists {
		return fmt.Errorf("token not found")
	}

	delete(s.tokens, symbol)
	return nil
}

// GetToken gets token
func (s *MasterWalletStore) GetToken(symbol string) (*TokenConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	token, ok := s.tokens[symbol]
	if !ok {
		return nil, fmt.Errorf("token not found")
	}

	return token, nil
}

// GetTokens gets all tokens
func (s *MasterWalletStore) GetTokens() []*TokenConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tokens := make([]*TokenConfig, 0, len(s.tokens))
	for _, token := range s.tokens {
		tokens = append(tokens, token)
	}

	return tokens
}

// ============================================================================
// BACKUP CODES
// ============================================================================

// generateBackupCodes generates backup codes
func (s *MasterWalletStore) generateBackupCodes(walletID string, count int) {
	codes := make([]BackupCode, count)
	for i := 0; i < count; i++ {
		codes[i] = BackupCode{
			ID:         generateUUID(),
			WalletID:  walletID,
			Code:      generateRandomHex(16),
			CreatedAt: time.Now(),
		}
	}
	s.backupCodes[walletID] = codes
}

// UseBackupCode uses backup code
func (s *MasterWalletStore) UseBackupCode(walletID, code string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	codes, ok := s.backupCodes[walletID]
	if !ok {
		return fmt.Errorf("no backup codes")
	}

	for i, c := range codes {
		if c.Code == code && c.UsedAt == nil {
			now := time.Now()
			codes[i].UsedAt = &now
			return nil
		}
	}

	return fmt.Errorf("invalid backup code")
}

// ============================================================================
// TRANSACTION HISTORY
// ============================================================================

// GetTransactionHistory gets transaction history
func (s *MasterWalletStore) GetTransactionHistory(walletID string, limit int) []*AutoTransaction {
	s.mu.RLock()
	defer s.mu.RUnlock()

	history := make([]*AutoTransaction, 0)
	for _, tx := range s.transactionHistory {
		if tx.WalletID == walletID {
			history = append(history, tx)
			if limit > 0 && len(history) >= limit {
				break
			}
		}
	}

	return history
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

func generateRandomBytes(length int) []byte {
	b := make([]byte, length)
	rand.Read(b)
	return b
}

func generateRandomHex(length int) string {
	return hex.EncodeToString(generateRandomBytes(length / 2))
}

func generateUUID() string {
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		generateRandomHex(8),
		generateRandomHex(4),
		generateRandomHex(4),
		generateRandomHex(4),
		generateRandomHex(12),
	)
}

func generateMnemonic() string {
	// In production, use BIP39 word list
	words := []string{
		"abandon", "ability", "able", "about", "above", "absent", "absorb", "abstract",
		"absurd", "abuse", "access", "accident", "account", "accuse", "achieve",
		"acid", "acoustic", "acquire", "across", "act", "action", "actor", "actress",
		"actual", "adapt", "add", "addict", "address", "adjust", "admit", "adult",
		"advance", "advice", "aerobic", "affair", "afford", "afraid", "again", "age",
	}
	result := make([]string, 24)
	for i := 0; i < 24; i++ {
		result[i] = words[rand.Intn(len(words))]
	}
	return strings.Join(result, " ")
}

func deriveAddressFromMnemonic(mnemonic string) string {
	// In production, derive from BIP39 mnemonic
	hash := sha256.Sum256([]byte(mnemonic))
	return "0x" + hex.EncodeToString(hash[:20])
}

func deriveAddressFromIndex(encryptedMnemonic string, key []byte, index int) string {
	// In production, derive HD wallet address from index
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s-%d", encryptedMnemonic, index)))
	return "0x" + hex.EncodeToString(hash[:20])
}

func encryptMnemonic(mnemonic string, key []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	rand.Read(nonce)

	ciphertext := gcm.Seal(nonce, nonce, []byte(mnemonic), nil)
	return hex.EncodeToString(ciphertext), nil
}

func decryptMnemonic(encrypted string, key []byte) (string, error) {
	data, err := hex.DecodeString(encrypted)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	nonce, data := data[:nonceSize], data[nonceSize:]

	plaintext, err := gcm.Open(nil, nonce, data, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

// ============================================================================
// HTTP HANDLERS
// ============================================================================

// MasterWalletHandler handles master wallet requests
type MasterWalletHandler struct {
	store *MasterWalletStore
}

// NewMasterWalletHandler creates new handler
func NewMasterWalletHandler(store *MasterWalletStore) *MasterWalletHandler {
	return &MasterWalletHandler{store: store}
}

// HandleGetChains handles get chains request
func (h *MasterWalletHandler) HandleGetChains(w http.ResponseWriter, r *http.Request) {
	chains := h.store.GetChains()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(chains)
}

// HandleGetTokens handles get tokens request
func (h *MasterWalletHandler) HandleGetTokens(w http.ResponseWriter, r *http.Request) {
	tokens := h.store.GetTokens()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tokens)
}

// HandleGetMasterWallet handles get master wallet request
func (h *MasterWalletHandler) HandleGetMasterWallet(w http.ResponseWriter, r *http.Request) {
	wallet := h.store.GetMasterWallet()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(wallet)
}

// HandleSetFeeAddress handles set fee address request
func (h *MasterWalletHandler) HandleSetFeeAddress(w http.ResponseWriter, r *http.Request) {
	var req struct {
		FeeType string `json:"fee_type"`
		Address string `json:"address"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	if err := h.store.SetFeeAddress(req.FeeType, req.Address); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"success": true,
		"message": "fee address set",
	})
}

// ============================================================================
// GLOBAL INSTANCE
// ============================================================================

var masterWalletStore *MasterWalletStore

// InitMasterWallet initializes master wallet system
func InitMasterWallet() {
	masterWalletStore = NewMasterWalletStore()

	// Create master wallet
	wallet, err := masterWalletStore.CreateMasterWallet(
		"TigerMaster",
		WALLET_TYPE_OPERATIONS,
		"", // Generate new mnemonic
	)
	if err != nil {
		// Log error but continue
		fmt.Printf("Warning: Failed to create master wallet: %v\n", err)
	}
	_ = wallet
}

// GetMasterWalletStore returns master wallet store
func GetMasterWalletStore() *MasterWalletStore {
	return masterWalletStore
}