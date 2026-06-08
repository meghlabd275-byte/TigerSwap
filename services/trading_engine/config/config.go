package config

import (
	"os"
)

// Config is the trading engine configuration
type Config struct {
	// RPC Configuration
	RPCURL string `json:"rpcUrl"`

	// Order Registry
	OrderRegistryAddress string `json:"orderRegistryAddress"`

	// Signer
	SignerAddress string `json:"signerAddress"`
	SignerKey    string `json:"signerKey"` // Private key

	// DEX Configuration
	DEXs []DEXConfig `json:"dexs"`

	// Oracle Configuration
	OracleType string `json:"oracleType"`
	OracleURL string `json:"oracleUrl"`

	// Execution Configuration
	MaxSlippageBPS    int `json:"maxSlippageBps"`
	MaxGasPriceGwei   int `json:"maxGasPriceGwei"`
	ExecutionInterval int `json:"executionInterval"` // milliseconds

	// Monitoring Configuration
	PriceMonitorInterval int `json:"priceMonitorInterval"` // seconds
	OrderMonitorInterval int `json:"orderMonitorInterval"` // seconds
}

// DEXConfig is the configuration for a DEX
type DEXConfig struct {
	Name         string `json:"name"`
	Type        string `json:"type"` // uniswap, curve, balancer, etc.
	RouterAddress string `json:"routerAddress"`
	FactoryAddress string `json:"factoryAddress"`
	Enabled     bool   `json:"enabled"`
	Priority    int    `json:"priority"` // Lower = higher priority
}

// Load loads configuration from environment
func Load() (*Config, error) {
	cfg := &Config{
		RPCURL:               getEnv("RPC_URL", "https://eth.llamarpc.com"),
		OrderRegistryAddress: getEnv("ORDER_REGISTRY_ADDRESS", "0x0000000000000000000000000000000000000000000"),
		SignerAddress:       getEnv("SIGNER_ADDRESS", ""),
		SignerKey:          getEnv("SIGNER_KEY", ""),
		OracleType:         getEnv("ORACLE_TYPE", "chainlink"),
		OracleURL:          getEnv("ORACLE_URL", ""),
		MaxSlippageBPS:    50,
		MaxGasPriceGwei:    100,
		ExecutionInterval:  500,
		PriceMonitorInterval: 1,
		OrderMonitorInterval: 5,
	}

	// Parse DEXs from environment
	cfg.DEXs = parseDEXs()

	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func parseDEXs() []DEXConfig {
	// Default DEX configurations
	return []DEXConfig{
		{
			Name:            "UniswapV3",
			Type:           "uniswap_v3",
			RouterAddress:  "0xE592427A0AEce92De3E94f1c717E670C0cF518B8",
			FactoryAddress: "0x1F98431c85a56aEff152667bf458D8E698FeBE1BD",
			Enabled:       true,
			Priority:     1,
		},
		{
			Name:            "UniswapV2",
			Type:           "uniswap_v2",
			RouterAddress:  "0x7a250d5630B4cF539b50D491cEB2D7b3a92cB5f8c",
			FactoryAddress: "0x5C69bEe701ef814a2C6Aa8c7c1B0D9d3C3E2E8C",
			Enabled:       true,
			Priority:     2,
		},
		{
			Name:            "SushiSwap",
			Type:           "sushi",
			RouterAddress:  "0xD9e10aA2803fB13b01DE2A2Ee916Ba2C88c1e3f5E",
			FactoryAddress: "0xC0AEe478e3F24C53b1D3c0e1E9D3a3C3E2E8C",
			Enabled:       true,
			Priority:     3,
		},
		{
			Name:            "Curve",
			Type:           "curve",
			RouterAddress:  "0x8f942C20D02d0c7B352b0c74B67881dA2E2B8F1",
			FactoryAddress: "0x90E00ACe2a13d2D18c74E5C3eb1a2c2dB0c88c1",
			Enabled:       true,
			Priority:     4,
		},
		{
			Name:            "Balancer",
			Type:           "balancer",
			RouterAddress:  "0xBA12222222228d8Ba445958a75a0704d569BF2C",
			FactoryAddress: "0xBA12222222228d8Ba445958a75a0704d569BF2C",
			Enabled:       true,
			Priority:     5,
		},
	}
}