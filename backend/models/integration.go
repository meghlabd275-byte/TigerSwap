package models

import "time"

// IntegrationChain is a blockchain an operator has registered for TigerSwap to
// serve. It is the source of truth for the chains the platform lists, so the
// registry starts empty: every entry is an explicit operator action, not a
// seeded default.
type IntegrationChain struct {
	Key         string    `gorm:"primaryKey;size:64" json:"key"`
	Name        string    `gorm:"size:128" json:"name"`
	Type        string    `gorm:"size:32" json:"type"` // evm, solana, cosmos, ton, aptos, bitcoin
	ChainID     int64     `gorm:"index" json:"chain_id"`
	RPCURL      string    `gorm:"size:256" json:"rpc_url"`
	ExplorerURL string    `gorm:"size:256" json:"explorer_url"`
	Symbol      string    `gorm:"size:32" json:"symbol"`
	Decimals    int       `json:"decimals"`
	NativeToken string    `gorm:"size:128" json:"native_token"`
	IsActive    bool      `gorm:"index" json:"is_active"`
	IsTestnet   bool      `json:"is_testnet"`
	AddedAt     time.Time `json:"added_at"`
	AddedBy     string    `gorm:"size:128" json:"added_by"`
}

// IntegrationToken is a token an operator has registered on a registered chain.
// Its address is stored verbatim so a non-EVM chain (Solana base58, Cosmos
// bech32) is not forced into a 20-byte hex shape.
type IntegrationToken struct {
	Key        string    `gorm:"primaryKey;size:64" json:"key"`
	Address    string    `gorm:"size:128;index" json:"address"`
	ChainKey   string    `gorm:"size:64;index" json:"chain_key"`
	Name       string    `gorm:"size:128" json:"name"`
	Symbol     string    `gorm:"size:32" json:"symbol"`
	Decimals   int       `json:"decimals"`
	Supply     string    `gorm:"size:80" json:"total_supply"`
	Standard   string    `gorm:"size:32" json:"standard"` // ERC20, SPL, ...
	IsActive   bool      `gorm:"index" json:"is_active"`
	IsVerified bool      `json:"is_verified"`
	AddedAt    time.Time `json:"added_at"`
	AddedBy    string    `gorm:"size:128" json:"added_by"`
}

// IntegrationBridge is a bridge route between two registered chains, as
// configured by an operator.
type IntegrationBridge struct {
	Key           string    `gorm:"primaryKey;size:64" json:"key"`
	Name          string    `gorm:"size:128" json:"name"`
	SrcChain      string    `gorm:"size:64;index" json:"src_chain"`
	DstChain      string    `gorm:"size:64;index" json:"dst_chain"`
	RouterAddr    string    `gorm:"size:128" json:"router_address"`
	MinAmount     string    `gorm:"size:80" json:"min_amount"`
	MaxAmount     string    `gorm:"size:80" json:"max_amount"`
	FeePercent    float64   `json:"fee_percent"`
	FeeFixed      string    `gorm:"size:80" json:"fee_fixed"`
	EstimatedTime string    `gorm:"size:64" json:"estimated_time"`
	IsActive      bool      `gorm:"index" json:"is_active"`
	AddedAt       time.Time `json:"added_at"`
}

// IntegrationFarm is a pool or farm definition an operator has registered. The
// two share one table because a pool is a farm with no reward stream yet: the
// pool creation path sets RewardRate to zero and the farm creation path funds a
// stream.
type IntegrationFarm struct {
	Key         string    `gorm:"primaryKey;size:64" json:"key"`
	Name        string    `gorm:"size:128" json:"name"`
	PoolTokenA  string    `gorm:"size:64" json:"pool_token_a"`
	PoolTokenB  string    `gorm:"size:64" json:"pool_token_b"`
	RewardToken string    `gorm:"size:64" json:"reward_token"`
	RewardRate  string    `gorm:"size:80" json:"reward_rate"`
	APY         float64   `json:"apy"`
	TotalStaked string    `gorm:"size:80" json:"total_staked"`
	StartTime   time.Time `json:"start_time"`
	EndTime     time.Time `json:"end_time"`
	IsActive    bool      `gorm:"index" json:"is_active"`
	AddedAt     time.Time `json:"added_at"`
}
