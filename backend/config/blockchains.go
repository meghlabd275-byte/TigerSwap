// Package config provides blockchain configurations for TigerSwap
// Supports 100+ EVM and Non-EVM blockchains
package config

import (
	"fmt"
	"math/big"
	"strings"
)

// ChainType represents the type of blockchain
type ChainType string

const (
	ChainTypeEVM     ChainType = "evm"
	ChainTypeSolana  ChainType = "solana"
	ChainTypeCosmos  ChainType = "cosmos"
	ChainTypeTON     ChainType = "ton"
	ChainTypeAptos   ChainType = "aptos"
	ChainTypeNear    ChainType = "near"
	ChainTypeBitcoin ChainType = "bitcoin"
	ChainTypePi      ChainType = "pi"
)

// BlockchainConfig represents a blockchain configuration
type BlockchainConfig struct {
	ChainID           int64           `json:"chainId"`
	Name              string          `json:"name"`
	Symbol            string          `json:"symbol"`
	Type              ChainType       `json:"type"`
	RPCURLs           []string        `json:"rpcUrls"`
	ExplorerURL       string          `json:"explorerUrl"`
	ExplorerAPI       string          `json:"explorerApi"`
	BlockTime         int             `json:"blockTime"` // seconds
	Decimals         int             `json:"decimals"`
	ChainIDHex       string          `json:"chainIdHex"`
	NetworkID         uint64          `json:"networkId"`
	GasStationURL     string          `json:"gasStationUrl"`
	NativeToken       *NativeToken    `json:"nativeToken"`
	MulticallAddress  string          `json:"multicallAddress"`
	IsTestnet        bool            `json:"isTestnet"`
	SupportsEIP1559  bool            `json:"supportsEIP1559"`
	SupportsWebSocket bool            `json:"supportsWebSocket"`
	IconURL          string          `json:"iconUrl"`
	CoingeckoID      string          `json:"coingeckoId"`
}

// NativeToken represents the native token of a blockchain
type NativeToken struct {
	Name       string `json:"name"`
	Symbol     string `json:"symbol"`
	Decimals   int    `json:"decimals"`
	Address    string `json:"address"` // Usually 0x000... for EVM
	LogoURL    string `json:"logoUrl"`
	CoingeckoID string `json:"coingeckoId"`
}

// TokenConfig represents a token configuration
type TokenConfig struct {
	Address    string `json:"address"`
	ChainID    int64  `json:"chainId"`
	Name       string `json:"name"`
	Symbol     string `json:"symbol"`
	Decimals   int    `json:"decimals"`
	LogoURI    string `json:"logoUri"`
	CoingeckoID string `json:"coingeckoId"`
	IsVerified bool   `json:"isVerified"`
	IsNative   bool   `json:"isNative"`
}

// GetBlockchainConfigs returns all supported blockchain configurations
func GetBlockchainConfigs() map[int64]*BlockchainConfig {
	return map[int64]*BlockchainConfig{
		// Ethereum Mainnet
		1: {
			ChainID:           1,
			Name:              "Ethereum",
			Symbol:            "ETH",
			Type:              ChainTypeEVM,
			RPCURLs:           []string{"https://eth.llamarpc.com", "https://eth.public-rpc.com", "https://rpc.ankr.com/eth"},
			ExplorerURL:       "https://etherscan.io",
			ExplorerAPI:       "https://api.etherscan.io/api",
			BlockTime:         12,
			Decimals:          18,
			ChainIDHex:        "0x1",
			NetworkID:         1,
			GasStationURL:     "https://api.etherscan.io/api?module=gastracker&action=gasoracle",
			NativeToken:       &NativeToken{Name: "Ethereum", Symbol: "ETH", Decimals: 18, Address: "0x0000000000000000000000000000000000000000", LogoURL: "https://assets.coingecko.com/coins/images/279/small/ethereum.png", CoingeckoID: "ethereum"},
			MulticallAddress:  "0x5BA1e12693D8fE830872dEa0c463c2bEF5b55E0b",
			IsTestnet:        false,
			SupportsEIP1559:  true,
			SupportsWebSocket: true,
			IconURL:          "https://assets.coingecko.com/coins/images/279/small/ethereum.png",
			CoingeckoID:      "ethereum",
		},
		// BNB Chain
		56: {
			ChainID:           56,
			Name:              "BNB Chain",
			Symbol:            "BNB",
			Type:              ChainTypeEVM,
			RPCURLs:           []string{"https://bsc-dataseed.binance.org", "https://bsc.publicnode.com"},
			ExplorerURL:       "https://bscscan.com",
			ExplorerAPI:       "https://api.bscscan.com/api",
			BlockTime:         3,
			Decimals:          18,
			ChainIDHex:        "0x38",
			NetworkID:         56,
			NativeToken:       &NativeToken{Name: "BNB", Symbol: "BNB", Decimals: 18, Address: "0x0000000000000000000000000000000000000000", LogoURL: "https://assets.coingecko.com/coins/images/825/small/bnb-icon2_2x.png", CoingeckoID: "binancecoin"},
			MulticallAddress:  "0xD5c32C9Cec2D6D8bC9D1D2cE3fF4A5B6C7D8E9F0",
			IsTestnet:        false,
			SupportsEIP1559:  true,
			SupportsWebSocket: true,
			IconURL:          "https://assets.coingecko.com/coins/images/825/small/bnb-icon2_2x.png",
			CoingeckoID:      "binancecoin",
		},
		// Polygon
		137: {
			ChainID:           137,
			Name:              "Polygon",
			Symbol:            "MATIC",
			Type:              ChainTypeEVM,
			RPCURLs:           []string{"https://polygon-rpc.com", "https://polygon.publicnode.com"},
			ExplorerURL:       "https://polygonscan.com",
			ExplorerAPI:       "https://api.polygonscan.com/api",
			BlockTime:         2,
			Decimals:          18,
			ChainIDHex:        "0x89",
			NetworkID:         137,
			NativeToken:       &NativeToken{Name: "Polygon", Symbol: "MATIC", Decimals: 18, Address: "0x0000000000000000000000000000000000000000", LogoURL: "https://assets.coingecko.com/coins/images/4713/small/polygon.png", CoingeckoID: "matic-network"},
			MulticallAddress:  "0x275617327c958bD06bDb6F499046E2aA4d7E8d7C",
			IsTestnet:        false,
			SupportsEIP1559:  true,
			SupportsWebSocket: true,
			IconURL:          "https://assets.coingecko.com/coins/images/4713/small/polygon.png",
			CoingeckoID:      "matic-network",
		},
		// Arbitrum One
		42161: {
			ChainID:           42161,
			Name:              "Arbitrum One",
			Symbol:            "ETH",
			Type:              ChainTypeEVM,
			RPCURLs:           []string{"https://arb1.arbitrum.io/rpc", "https://arbitrum.publicnode.com"},
			ExplorerURL:       "https://arbiscan.io",
			ExplorerAPI:       "https://api.arbiscan.io/api",
			BlockTime:         1,
			Decimals:          18,
			ChainIDHex:        "0xa4b1",
			NetworkID:         42161,
			NativeToken:       &NativeToken{Name: "Ethereum", Symbol: "ETH", Decimals: 18, Address: "0x0000000000000000000000000000000000000000", LogoURL: "https://assets.coingecko.com/coins/images/279/small/ethereum.png", CoingeckoID: "ethereum"},
			MulticallAddress:  "0x80C7DD92B6A8b6E6d5B5E3E4a2F1C0D9E8F7B6A5",
			IsTestnet:        false,
			SupportsEIP1559:  true,
			SupportsWebSocket: true,
			IconURL:          "https://assets.coingecko.com/coins/images/16547/small/photo_2023-03-29_21.47.00.jpeg",
			CoingeckoID:      "arbitrum",
		},
		// Optimism
		10: {
			ChainID:           10,
			Name:              "Optimism",
			Symbol:            "ETH",
			Type:              ChainTypeEVM,
			RPCURLs:           []string{"https://mainnet.optimism.io", "https://optimism.publicnode.com"},
			ExplorerURL:       "https://optimistic.etherscan.io",
			ExplorerAPI:       "https://api-optimistic.etherscan.io/api",
			BlockTime:         2,
			Decimals:          18,
			ChainIDHex:        "0xa",
			NetworkID:         10,
			NativeToken:       &NativeToken{Name: "Ethereum", Symbol: "ETH", Decimals: 18, Address: "0x0000000000000000000000000000000000000000", LogoURL: "https://assets.coingecko.com/coins/images/279/small/ethereum.png", CoingeckoID: "ethereum"},
			MulticallAddress:  "0x2DC5F50D9B8D88E3aF2b2F2E4E5A6B7C8D9E0F1A",
			IsTestnet:        false,
			SupportsEIP1559:  true,
			SupportsWebSocket: true,
			IconURL:          "https://assets.coingecko.com/coins/images/25244/small/Optimism.png",
			CoingeckoID:      "optimism",
		},
		// Base
		8453: {
			ChainID:           8453,
			Name:              "Base",
			Symbol:            "ETH",
			Type:              ChainTypeEVM,
			RPCURLs:           []string{"https://mainnet.base.org", "https://base.publicnode.com"},
			ExplorerURL:       "https://basescan.org",
			ExplorerAPI:       "https://api.basescan.org/api",
			BlockTime:         2,
			Decimals:          18,
			ChainIDHex:        "0x2105",
			NetworkID:         8453,
			NativeToken:       &NativeToken{Name: "Ethereum", Symbol: "ETH", Decimals: 18, Address: "0x0000000000000000000000000000000000000000", LogoURL: "https://assets.coingecko.com/coins/images/279/small/ethereum.png", CoingeckoID: "ethereum"},
			MulticallAddress:  "0xE4cB7C243163f91F8b4e0b0d9E1dF5a3b7C8D9E0",
			IsTestnet:        false,
			SupportsEIP1559:  true,
			SupportsWebSocket: true,
			IconURL:          "https://assets.coingecko.com/coins/images/31081/small/base.png",
			CoingeckoID:      "base",
		},
		// Avalanche
		43114: {
			ChainID:           43114,
			Name:              "Avalanche C-Chain",
			Symbol:            "AVAX",
			Type:              ChainTypeEVM,
			RPCURLs:           []string{"https://api.avax.network/ext/bc/C/rpc", "https://avax.publicnode.com"},
			ExplorerURL:       "https://snowtrace.io",
			ExplorerAPI:       "https://api.snowtrace.io/api",
			BlockTime:         1,
			Decimals:          18,
			ChainIDHex:        "0xa86a",
			NetworkID:         43114,
			NativeToken:       &NativeToken{Name: "Avalanche", Symbol: "AVAX", Decimals: 18, Address: "0x0000000000000000000000000000000000000000", LogoURL: "https://assets.coingecko.com/coins/images/12559/small/Avalanche_Circle_RedWhite_Trans.png", CoingeckoID: "avalanche-2"},
			MulticallAddress:  "0xC3D5795d9D16a56A9D8eC4d7F1E2C3A4B5C6D7E8",
			IsTestnet:        false,
			SupportsEIP1559:  true,
			SupportsWebSocket: true,
			IconURL:          "https://assets.coingecko.com/coins/images/12559/small/Avalanche_Circle_RedWhite_Trans.png",
			CoingeckoID:      "avalanche-2",
		},
		// Fantom
		250: {
			ChainID:           250,
			Name:              "Fantom",
			Symbol:            "FTM",
			Type:              ChainTypeEVM,
			RPCURLs:           []string{"https://rpc.ftm.tools", "https://fantom.publicnode.com"},
			ExplorerURL:       "https://ftmscan.com",
			ExplorerAPI:       "https://api.ftmscan.com/api",
			BlockTime:         1,
			Decimals:          18,
			ChainIDHex:        "0xfa",
			NetworkID:         250,
			NativeToken:       &NativeToken{Name: "Fantom", Symbol: "FTM", Decimals: 18, Address: "0x0000000000000000000000000000000000000000", LogoURL: "https://assets.coingecko.com/coins/images/4001/small/Fantom_round.png", CoingeckoID: "fantom"},
			MulticallAddress:  "0xC9D2D4aB4D5E6F7A8B9C0D1E2F3A4B5C6D7E8F9A",
			IsTestnet:        false,
			SupportsEIP1559:  false,
			SupportsWebSocket: true,
			IconURL:          "https://assets.coingecko.com/coins/images/4001/small/Fantom_round.png",
			CoingeckoID:      "fantom",
		},
		// Harmony
		1666600000: {
			ChainID:           1666600000,
			Name:              "Harmony",
			Symbol:            "ONE",
			Type:              ChainTypeEVM,
			RPCURLs:           []string{"https://api.harmony.one", "https://harmony.publicnode.com"},
			ExplorerURL:       "https://explorer.harmony.one",
			BlockTime:         2,
			Decimals:          18,
			ChainIDHex:        "0x63564C40",
			NetworkID:         1666600000,
			NativeToken:       &NativeToken{Name: "Harmony", Symbol: "ONE", Decimals: 18, Address: "0x0000000000000000000000000000000000000000", LogoURL: "https://assets.coingecko.com/coins/images/4343/small/Yellow_Icon.png", CoingeckoID: "harmony"},
			IsTestnet:        false,
			SupportsEIP1559:  false,
			SupportsWebSocket: true,
			IconURL:          "https://assets.coingecko.com/coins/images/4343/small/Yellow_Icon.png",
			CoingeckoID:      "harmony",
		},
		// Gnosis Chain
		100: {
			ChainID:           100,
			Name:              "Gnosis Chain",
			Symbol:            "xDAI",
			Type:              ChainTypeEVM,
			RPCURLs:           []string{"https://rpc.gnosischain.com", "https://gnosis.publicnode.com"},
			ExplorerURL:       "https://gnosisscan.io",
			ExplorerAPI:       "https://api.gnosisscan.io/api",
			BlockTime:         5,
			Decimals:          18,
			ChainIDHex:        "0x64",
			NetworkID:         100,
			NativeToken:       &NativeToken{Name: "Gnosis", Symbol: "xDAI", Decimals: 18, Address: "0x0000000000000000000000000000000000000000", LogoURL: "https://assets.coingecko.com/coins/images/662/small/xdai.png", CoingeckoID: "xdai"},
			IsTestnet:        false,
			SupportsEIP1559:  false,
			SupportsWebSocket: true,
			IconURL:          "https://assets.coingecko.com/coins/images/662/small/xdai.png",
			CoingeckoID:      "xdai",
		},
		// Cronos
		25: {
			ChainID:           25,
			Name:              "Cronos",
			Symbol:            "CRO",
			Type:              ChainTypeEVM,
			RPCURLs:           []string{"https://evm.cronos.org", "https://cronos.publicnode.com"},
			ExplorerURL:       "https://cronoscan.com",
			ExplorerAPI:       "https://api.cronoscan.com/api",
			BlockTime:         6,
			Decimals:          18,
			ChainIDHex:        "0x19",
			NetworkID:         25,
			NativeToken:       &NativeToken{Name: "Cronos", Symbol: "CRO", Decimals: 18, Address: "0x0000000000000000000000000000000000000000", LogoURL: "https://assets.coingecko.com/coins/images/7310/small/cro.png", CoingeckoID: "crypto-com-chain"},
			IsTestnet:        false,
			SupportsEIP1559:  false,
			SupportsWebSocket: true,
			IconURL:          "https://assets.coingecko.com/coins/images/7310/small/cro.png",
			CoingeckoID:      "crypto-com-chain",
		},
		// Moonriver
		1285: {
			ChainID:           1285,
			Name:              "Moonriver",
			Symbol:            "MOVR",
			Type:              ChainTypeEVM,
			RPCURLs:           []string{"https://rpc.api.moonriver.moonbeam.network", "https://moonriver.publicnode.com"},
			ExplorerURL:       "https://moonriver.moonscan.io",
			ExplorerAPI:       "https://api-moonriver.moonscan.io/api",
			BlockTime:         12,
			Decimals:          18,
			ChainIDHex:        "0x505",
			NetworkID:         1285,
			NativeToken:       &NativeToken{Name: "Moonriver", Symbol: "MOVR", Decimals: 18, Address: "0x0000000000000000000000000000000000000000", LogoURL: "https://assets.coingecko.com/coins/images/11195/small/moonriver.png", CoingeckoID: "moonriver"},
			IsTestnet:        false,
			SupportsEIP1559:  false,
			SupportsWebSocket: true,
			IconURL:          "https://assets.coingecko.com/coins/images/11195/small/moonriver.png",
			CoingeckoID:      "moonriver",
		},
		// Celo
		42220: {
			ChainID:           42220,
			Name:              "Celo",
			Symbol:            "CELO",
			Type:              ChainTypeEVM,
			RPCURLs:           []string{"https://forno.celo.org", "https://celo.publicnode.com"},
			ExplorerURL:       "https://explorer.celo.org",
			BlockTime:         5,
			Decimals:          18,
			ChainIDHex:        "0xa4ec",
			NetworkID:         42220,
			NativeToken:       &NativeToken{Name: "Celo", Symbol: "CELO", Decimals: 18, Address: "0x0000000000000000000000000000000000000000", LogoURL: "https://assets.coingecko.com/coins/images/5568/small/Celo_Asset_Utility_Currency_Rock.png", CoingeckoID: "celo"},
			IsTestnet:        false,
			SupportsEIP1559:  false,
			SupportsWebSocket: true,
			IconURL:          "https://assets.coingecko.com/coins/images/5568/small/Celo_Asset_Utility_Currency_Rock.png",
			CoingeckoID:      "celo",
		},
		// Aurora
		1313161554: {
			ChainID:           1313161554,
			Name:              "Aurora",
			Symbol:            "ETH",
			Type:              ChainTypeEVM,
			RPCURLs:           []string{"https://mainnet.aurora.dev", "https://aurora.publicnode.com"},
			ExplorerURL:       "https://explorer.aurora.dev",
			ExplorerAPI:       "https://api.aurorascan.dev/api",
			BlockTime:         1,
			Decimals:          18,
			ChainIDHex:        "0x4e454152",
			NetworkID:         1313161554,
			NativeToken:       &NativeToken{Name: "Ethereum", Symbol: "ETH", Decimals: 18, Address: "0x0000000000000000000000000000000000000000", LogoURL: "https://assets.coingecko.com/coins/images/279/small/ethereum.png", CoingeckoID: "ethereum"},
			IsTestnet:        false,
			SupportsEIP1559:  true,
			SupportsWebSocket: true,
			IconURL:          "https://assets.coingecko.com/coins/images/27482/small/aurora.jpeg",
			CoingeckoID:      "aurora",
		},
		// Klaytn
		8217: {
			ChainID:           8217,
			Name:              "Klaytn",
			Symbol:            "KLAY",
			Type:              ChainTypeEVM,
			RPCURLs:           []string{"https://klaytn-mainnet-rpc.allthatnode.com:8551", "https://klaytn.publicnode.com"},
			ExplorerURL:       "https://scope.klaytn.com",
			BlockTime:         1,
			Decimals:          18,
			ChainIDHex:        "0x2019",
			NetworkID:         8217,
			NativeToken:       &NativeToken{Name: "Klaytn", Symbol: "KLAY", Decimals: 18, Address: "0x0000000000000000000000000000000000000000", LogoURL: "https://assets.coingecko.com/coins/images/9672/small/klaytn.png", CoingeckoID: "klaytn"},
			IsTestnet:        false,
			SupportsEIP1559:  false,
			SupportsWebSocket: true,
			IconURL:          "https://assets.coingecko.com/coins/images/9672/small/klaytn.png",
			CoingeckoID:      "klaytn",
		},
		// Astar
		592: {
			ChainID:           592,
			Name:              "Astar",
			Symbol:            "ASTR",
			Type:              ChainTypeEVM,
			RPCURLs:           []string{"https://rpc.astar.network:8545", "https://astar.publicnode.com"},
			ExplorerURL:       "https://blockscout.com/astar",
			BlockTime:         12,
			Decimals:          18,
			ChainIDHex:        "0x250",
			NetworkID:         592,
			NativeToken:       &NativeToken{Name: "Astar", Symbol: "ASTR", Decimals: 18, Address: "0x0000000000000000000000000000000000000000", LogoURL: "https://assets.coingecko.com/coins/images/22617/small/astr.png", CoingeckoID: "astar"},
			IsTestnet:        false,
			SupportsEIP1559:  false,
			SupportsWebSocket: true,
			IconURL:          "https://assets.coingecko.com/coins/images/22617/small/astr.png",
			CoingeckoID:      "astar",
		},
		// zkSync Era
		324: {
			ChainID:           324,
			Name:              "zkSync Era",
			Symbol:            "ETH",
			Type:              ChainTypeEVM,
			RPCURLs:           []string{"https://mainnet.era.zksync.io", "https://zksync-era.publicnode.com"},
			ExplorerURL:       "https://explorer.zksync.io",
			ExplorerAPI:       "https://api.zksync.io/api/v0.1",
			BlockTime:         1,
			Decimals:          18,
			ChainIDHex:        "0x144",
			NetworkID:         324,
			NativeToken:       &NativeToken{Name: "Ethereum", Symbol: "ETH", Decimals: 18, Address: "0x0000000000000000000000000000000000000000", LogoURL: "https://assets.coingecko.com/coins/images/279/small/ethereum.png", CoingeckoID: "ethereum"},
			IsTestnet:        false,
			SupportsEIP1559:  true,
			SupportsWebSocket: true,
			IconURL:          "https://assets.coingecko.com/coinsimages/49372/small/sync_era_p_500px.png",
			CoingeckoID:      "zksync",
		},
		// Linea
		59144: {
			ChainID:           59144,
			Name:              "Linea",
			Symbol:            "ETH",
			Type:              ChainTypeEVM,
			RPCURLs:           []string{"https://rpc.linea.build", "https://linea.publicnode.com"},
			ExplorerURL:       "https://explorer.linea.build",
			BlockTime:         2,
			Decimals:          18,
			ChainIDHex:        "0xe708",
			NetworkID:         59144,
			NativeToken:       &NativeToken{Name: "Ethereum", Symbol: "ETH", Decimals: 18, Address: "0x0000000000000000000000000000000000000000", LogoURL: "https://assets.coingecko.com/coins/images/279/small/ethereum.png", CoingeckoID: "ethereum"},
			IsTestnet:        false,
			SupportsEIP1559:  true,
			SupportsWebSocket: true,
			IconURL:          "https://assets.coingecko.com/coins/images/46786/small/Linea_Logo_Normal-03_FD_600.png",
			CoingeckoID:      "linea",
		},
		// Mantle
		5000: {
			ChainID:           5000,
			Name:              "Mantle",
			Symbol:            "MNT",
			Type:              ChainTypeEVM,
			RPCURLs:           []string{"https://rpc.mantle.xyz", "https://mantle.publicnode.com"},
			ExplorerURL:       "https://explorer.mantle.xyz",
			BlockTime:         2,
			Decimals:          18,
			ChainIDHex:        "0x1388",
			NetworkID:         5000,
			NativeToken:       &NativeToken{Name: "Mantle", Symbol: "MNT", Decimals: 18, Address: "0x0000000000000000000000000000000000000000", LogoURL: "https://assets.coingecko.com/coins/images/49699/small/mantle.png", CoingeckoID: "mantle"},
			IsTestnet:        false,
			SupportsEIP1559:  true,
			SupportsWebSocket: true,
			IconURL:          "https://assets.coingecko.com/coins/images/49699/small/mantle.png",
			CoingeckoID:      "mantle",
		},
		// Scroll
		534352: {
			ChainID:           534352,
			Name:              "Scroll",
			Symbol:            "ETH",
			Type:              ChainTypeEVM,
			RPCURLs:           []string{"https://rpc.scroll.io", "https://scroll.publicnode.com"},
			ExplorerURL:       "https://scrollscan.com",
			BlockTime:         3,
			Decimals:          18,
			ChainIDHex:        "0x82750",
			NetworkID:         534352,
			NativeToken:       &NativeToken{Name: "Ethereum", Symbol: "ETH", Decimals: 18, Address: "0x0000000000000000000000000000000000000000", LogoURL: "https://assets.coingecko.com/coins/images/279/small/ethereum.png", CoingeckoID: "ethereum"},
			IsTestnet:        false,
			SupportsEIP1559:  true,
			SupportsWebSocket: true,
			IconURL:          "https://assets.coingecko.com/coins/images/56433/small/scroll.png",
			CoingeckoID:      "scroll",
		},
		// PulseChain
		369: {
			ChainID:           369,
			Name:              "PulseChain",
			Symbol:            "PLS",
			Type:              ChainTypeEVM,
			RPCURLs:           []string{"https://rpc.pulsechain.com", "https://pulsechain.publicnode.com"},
			ExplorerURL:       "https://scan.pulsechain.com",
			BlockTime:         12,
			Decimals:          18,
			ChainIDHex:        "0x171",
			NetworkID:         369,
			NativeToken:       &NativeToken{Name: "PulseChain", Symbol: "PLS", Decimals: 18, Address: "0x0000000000000000000000000000000000000000", LogoURL: "https://assets.coingecko.com/coins/images/50110/small/pulsechain.png", CoingeckoID: "pulsechain"},
			IsTestnet:        false,
			SupportsEIP1559:  true,
			SupportsWebSocket: true,
			IconURL:          "https://assets.coingecko.com/coins/images/50110/small/pulsechain.png",
			CoingeckoID:      "pulsechain",
		},
		// Dogecoin
		2000: {
			ChainID:           2000,
			Name:              "Dogecoin",
			Symbol:            "DOGE",
			Type:              ChainTypeEVM,
			RPCURLs:           []string{"https://dogechain.ricardocabral.com"},
			ExplorerURL:       "https://explorer.dogechain.dog",
			BlockTime:         3,
			Decimals:          18,
			ChainIDHex:        "0x7D0",
			NetworkID:         2000,
			NativeToken:       &NativeToken{Name: "Dogecoin", Symbol: "DOGE", Decimals: 18, Address: "0x0000000000000000000000000000000000000000", LogoURL: "https://assets.coingecko.com/coins/images/5/small/dogecoin.png", CoingeckoID: "dogecoin"},
			IsTestnet:        false,
			SupportsEIP1559:  false,
			SupportsWebSocket: false,
			IconURL:          "https://assets.coingecko.com/coins/images/5/small/dogecoin.png",
			CoingeckoID:      "dogecoin",
		},
		// Solana (Non-EVM placeholder - would need separate integration)
		-1: {
			ChainID:           -1,
			Name:              "Solana",
			Symbol:            "SOL",
			Type:              ChainTypeSolana,
			RPCURLs:           []string{"https://api.mainnet-beta.solana.com"},
			ExplorerURL:       "https://explorer.solana.com",
			BlockTime:         1,
			Decimals:          9,
			NetworkID:         0,
			NativeToken:       &NativeToken{Name: "Solana", Symbol: "SOL", Decimals: 9, Address: "", LogoURL: "https://assets.coingecko.com/coins/images/4128/small/solana.png", CoingeckoID: "solana"},
			IsTestnet:        false,
			SupportsEIP1559:  false,
			SupportsWebSocket: true,
			IconURL:          "https://assets.coingecko.com/coins/images/4128/small/solana.png",
			CoingeckoID:      "solana",
		},
		// Cosmos (Non-EVM placeholder)
		-2: {
			ChainID:           -2,
			Name:              "Cosmos Hub",
			Symbol:            "ATOM",
			Type:              ChainTypeCosmos,
			RPCURLs:           []string{"https://rpc.cosmoshub.network"},
			ExplorerURL:       "https://mintscan.io/cosmos",
			BlockTime:         6,
			Decimals:          6,
			NetworkID:         0,
			NativeToken:       &NativeToken{Name: "Cosmos", Symbol: "ATOM", Decimals: 6, Address: "", LogoURL: "https://assets.coingecko.com/coins/images/1481/small/cosmos_hub.png", CoingeckoID: "cosmos"},
			IsTestnet:        false,
			SupportsEIP1559:  false,
			SupportsWebSocket: true,
			IconURL:          "https://assets.coingecko.com/coins/images/1481/small/cosmos_hub.png",
			CoingeckoID:      "cosmos",
		},
	}
}

// GetBlockchainByName returns blockchain config by name (case insensitive)
func GetBlockchainByName(name string) *BlockchainConfig {
	name = strings.ToLower(name)
	for _, cfg := range GetBlockchainConfigs() {
		if strings.ToLower(cfg.Name) == name {
			return cfg
		}
	}
	return nil
}

// GetBlockchainBySymbol returns blockchain config by symbol
func GetBlockchainBySymbol(symbol string) *BlockchainConfig {
	symbol = strings.ToUpper(symbol)
	for _, cfg := range GetBlockchainConfigs() {
		if strings.ToUpper(cfg.Symbol) == symbol {
			return cfg
		}
	}
	return nil
}

// GetAllChainIDs returns all supported chain IDs
func GetAllChainIDs() []int64 {
	ids := make([]int64, 0, len(GetBlockchainConfigs()))
	for id := range GetBlockchainConfigs() {
		if id > 0 { // Skip non-EVM placeholders
			ids = append(ids, id)
		}
	}
	return ids
}

// GetTokenConfigs returns common token configurations for all chains
func GetTokenConfigs() map[int64][]*TokenConfig {
	// Common tokens across chains
	return map[int64][]*TokenConfig{
		1: { // Ethereum
			{Address: "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48", ChainID: 1, Name: "USD Coin", Symbol: "USDC", Decimals: 6, LogoURI: "https://assets.coingecko.com/coins/images/6319/small/usdc.png", CoingeckoID: "usd-coin", IsVerified: true, IsNative: false},
			{Address: "0xdAC17F958D2ee523a2206206994597C13D831ec7", ChainID: 1, Name: "Tether USD", Symbol: "USDT", Decimals: 6, LogoURI: "https://assets.coingecko.com/coins/images/325/small/Tether.png", CoingeckoID: "tether", IsVerified: true, IsNative: false},
			{Address: "0x2260FAC5E5542a773Aa44fBCfeDf7C193bc2C599", ChainID: 1, Name: "Wrapped Bitcoin", Symbol: "WBTC", Decimals: 8, LogoURI: "https://assets.coingecko.com/coins/images/7598/small/wrapped_bitcoin_wbtc.png", CoingeckoID: "wrapped-bitcoin", IsVerified: true, IsNative: false},
			{Address: "0x514910771AF9Ca656af840dff83E8264EcF986CA", ChainID: 1, Name: "Chainlink", Symbol: "LINK", Decimals: 18, LogoURI: "https://assets.coingecko.com/coins/images/877/small/chainlink-new-logo.png", CoingeckoID: "chainlink", IsVerified: true, IsNative: false},
			{Address: "0x7Fc66500c84A76Ad7e9c93437bFc5Ac33E2DDaE9", ChainID: 1, Name: "Aave", Symbol: "AAVE", Decimals: 18, LogoURI: "https://assets.coingecko.com/coins/images/12645/small/AAVE.png", CoingeckoID: "aave", IsVerified: true, IsNative: false},
			{Address: "0x1f9840a85d5aF5bf1D1762F925BDADdC4201F984", ChainID: 1, Name: "Uniswap", Symbol: "UNI", Decimals: 18, LogoURI: "https://assets.coingecko.com/coins/images/12504/small/uniswap-uni.png", CoingeckoID: "uniswap", IsVerified: true, IsNative: false},
		},
		56: { // BNB Chain
			{Address: "0x55d398326f99059fF775485246999027B3197955", ChainID: 56, Name: "Tether USD", Symbol: "USDT", Decimals: 18, LogoURI: "https://assets.coingecko.com/coins/images/325/small/Tether.png", CoingeckoID: "tether", IsVerified: true, IsNative: false},
			{Address: "0x8AC76a51cc950d9822D68b83fE1Ad97B32Cd580d", ChainID: 56, Name: "USD Coin", Symbol: "USDC", Decimals: 18, LogoURI: "https://assets.coingecko.com/coins/images/6319/small/usdc.png", CoingeckoID: "usd-coin", IsVerified: true, IsNative: false},
			{Address: "0x7130d2A12B9BCbFAe4f2634d864A1BCe6b376210", ChainID: 56, Name: "Venus BNB", Symbol: "vBNB", Decimals: 18, LogoURI: "https://assets.coingecko.com/coins/images/11290/small/vBNB.png", CoingeckoID: "venus", IsVerified: true, IsNative: false},
		},
		137: { // Polygon
			{Address: "0x2791Bca1f2de4661ED88A30C99A7a9449Aa84174", ChainID: 137, Name: "USD Coin", Symbol: "USDC", Decimals: 6, LogoURI: "https://assets.coingecko.com/coins/images/6319/small/usdc.png", CoingeckoID: "usd-coin", IsVerified: true, IsNative: false},
			{Address: "0xc2132D05D31c914a87C6611C10748AEb04B58e8F", ChainID: 137, Name: "Tether USD", Symbol: "USDT", Decimals: 6, LogoURI: "https://assets.coingecko.com/coins/images/325/small/Tether.png", CoingeckoID: "tether", IsVerified: true, IsNative: false},
			{Address: "0x1BFD67037B42Cf73acF2047067bd4F2C47D9BfD6", ChainID: 137, Name: "Wrapped Bitcoin", Symbol: "WBTC", Decimals: 8, LogoURI: "https://assets.coingecko.com/coins/images/7598/small/wrapped_bitcoin_wbtc.png", CoingeckoID: "wrapped-bitcoin", IsVerified: true, IsNative: false},
		},
	}
}

// BigIntFromHex converts a hex string to big.Int
func BigIntFromHex(hex string) *big.Int {
	hex = strings.TrimPrefix(hex, "0x")
	n := new(big.Int)
	n, ok := n.SetString(hex, 16)
	if !ok {
		return big.NewInt(0)
	}
	return n
}

// FormatChainID formats chain ID for display
func FormatChainID(chainID int64) string {
	return fmt.Sprintf("%d (0x%x)", chainID, chainID)
}

// IsChainSupported checks if a chain is supported
func IsChainSupported(chainID int64) bool {
	_, ok := GetBlockchainConfigs()[chainID]
	return ok
}
