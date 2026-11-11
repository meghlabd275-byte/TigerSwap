package tigerswap

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// Client is the main TigerSwap API client
type Client struct {
	baseURL    string
	httpClient *http.Client
	apiKey     string
}

// NewClient creates a new TigerSwap API client
func NewClient(baseURL string, apiKey string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		apiKey: apiKey,
	}
}

// Token represents a cryptocurrency token
type Token struct {
	Address  string   `json:"address"`
	Symbol   string   `json:"symbol"`
	Name     string   `json:"name"`
	Decimals int      `json:"decimals"`
	ChainID  int64    `json:"chainId"`
	LogoURI  *string  `json:"logoURI"`
	PriceUSD *float64 `json:"priceUSD"`
}

// Chain represents a blockchain
type Chain struct {
	ID           int64   `json:"id"`
	Name         string  `json:"name"`
	ChainType    string  `json:"chainType"`
	NativeSymbol string  `json:"nativeSymbol"`
	ExplorerURL  *string `json:"explorerURL"`
	IsActive     bool    `json:"isActive"`
}

// SwapQuote represents a swap quote. RouteHops is the executable hop plan for a
// single multi-hop route and Legs is the plan for a split order; a direct
// single-family quote carries neither.
type SwapQuote struct {
	AmountIn     string         `json:"amountIn"`
	AmountOut    string         `json:"amountOut"`
	AmountOutMin string         `json:"amountOutMin"`
	PriceImpact  float64        `json:"priceImpact"`
	Route        []string       `json:"route"`
	GasEstimate  uint64         `json:"gasEstimate"`
	RouteHops    []SwapRouteHop `json:"routeHops,omitempty"`
	Legs         []SwapQuoteLeg `json:"legs,omitempty"`
}

// SwapRouteHop is one executable hop: a token pair and the pool family to trade
// it on. It is everything the router needs to execute the hop on-chain.
type SwapRouteHop struct {
	TokenIn  string `json:"tokenIn"`
	TokenOut string `json:"tokenOut"`
	Family   string `json:"family"`
	Pool     string `json:"pool"`
}

// SwapQuoteLeg is one leg of a split order: a route and the input it carries.
type SwapQuoteLeg struct {
	Path      []string       `json:"path"`
	Family    string         `json:"family"`
	Pool      string         `json:"pool"`
	AmountIn  string         `json:"amountIn"`
	AmountOut string         `json:"amountOut"`
	Hops      []SwapRouteHop `json:"hops,omitempty"`
}

// Pool represents a liquidity pool
type Pool struct {
	PairAddress string  `json:"pairAddress"`
	Token0      string  `json:"token0"`
	Token1      string  `json:"token1"`
	Symbol0     string  `json:"symbol0"`
	Symbol1     string  `json:"symbol1"`
	Reserve0    float64 `json:"reserve0"`
	Reserve1    float64 `json:"reserve1"`
	FeeBps      int     `json:"feeBps"`
}

// MarketStats represents market statistics
type MarketStats struct {
	TotalTVL   float64 `json:"totalTVL"`
	Volume24h  float64 `json:"volume24h"`
	TotalPairs int64   `json:"totalPairs"`
	TotalUsers int64   `json:"totalUsers"`
	UpdatedAt  *string `json:"updatedAt"`
}

// Transaction represents a swap transaction
type Transaction struct {
	ID        string `json:"id"`
	Hash      string `json:"hash"`
	ChainID   int64  `json:"chain_id"`
	Type      string `json:"type"`
	FromToken string `json:"from_token"`
	ToToken   string `json:"to_token"`
	Status    string `json:"status"`
	Timestamp string `json:"timestamp"`
}

// Farm is a liquidity-mining farm with its reward streams.
type Farm struct {
	ID                   string `json:"id"`
	Key                  string `json:"key"`
	Name                 string `json:"name"`
	FarmType             string `json:"farm_type"`
	ChainID              int64  `json:"chain_id"`
	StakingTokenAddress  string `json:"staking_token_address"`
	StakingTokenSymbol   string `json:"staking_token_symbol"`
	StakingTokenDecimals int    `json:"staking_token_decimals"`
	TotalStaked          string `json:"total_staked"`
	StartTime            string `json:"start_time"`
	EndTime              string `json:"end_time"`
	Status               string `json:"status"`
}

// FarmReward is one reward stream of a farm.
type FarmReward struct {
	ID            string `json:"id"`
	FarmID        string `json:"farm_id"`
	TokenAddress  string `json:"token_address"`
	TokenSymbol   string `json:"token_symbol"`
	TokenDecimals int    `json:"token_decimals"`
	RatePerSecond string `json:"rate_per_second"`
}

// FarmView is a farm together with its reward streams and live APR. APR is nil
// when no price source is configured; it is never invented.
type FarmView struct {
	Farm    Farm         `json:"farm"`
	Rewards []FarmReward `json:"rewards"`
	APR     *float64     `json:"apr"`
}

// FarmPendingReward is the reward a stake has accrued in one stream.
type FarmPendingReward struct {
	TokenAddress string `json:"token_address"`
	Symbol       string `json:"symbol"`
	Decimals     int    `json:"decimals"`
	Pending      string `json:"pending"`
}

// FarmStake is a user's position in a farm.
type FarmStake struct {
	StakeID             string              `json:"stake_id"`
	FarmID              string              `json:"farm_id"`
	FarmKey             string              `json:"farm_key"`
	FarmName            string              `json:"farm_name"`
	Status              string              `json:"status"`
	ChainID             int64               `json:"chain_id"`
	StakingTokenAddress string              `json:"staking_token_address"`
	StakingTokenSymbol  string              `json:"staking_token_symbol"`
	Staked              string              `json:"staked"`
	EndTime             string              `json:"end_time"`
	PendingRewards      []FarmPendingReward `json:"pending_rewards"`
}

// AirdropCampaign is an admin-managed token distribution.
type AirdropCampaign struct {
	ID             string `json:"id"`
	Key            string `json:"key"`
	Name           string `json:"name"`
	ChainID        int64  `json:"chain_id"`
	TokenAddress   string `json:"token_address"`
	TokenSymbol    string `json:"token_symbol"`
	TokenDecimals  int    `json:"token_decimals"`
	TotalAllocated string `json:"total_allocated"`
	Funded         string `json:"funded"`
	TotalClaimed   string `json:"total_claimed"`
	StartTime      string `json:"start_time"`
	EndTime        string `json:"end_time"`
	Status         string `json:"status"`
}

// AirdropAllocation is one user's entitlement in a campaign.
type AirdropAllocation struct {
	AllocationID string `json:"allocation_id"`
	CampaignID   string `json:"campaign_id"`
	CampaignKey  string `json:"campaign_key"`
	CampaignName string `json:"campaign_name"`
	TokenAddress string `json:"token_address"`
	Symbol       string `json:"symbol"`
	Amount       string `json:"amount"`
	Claimed      bool   `json:"claimed"`
	Status       string `json:"status"`
	StartTime    string `json:"start_time"`
	EndTime      string `json:"end_time"`
}

// NFTCollection is an admin-managed ERC-721 collection. TotalSupply counts the
// tokens minted so far, matching the contract's own counter.
type NFTCollection struct {
	ID              string `json:"id"`
	Key             string `json:"key"`
	Name            string `json:"name"`
	Symbol          string `json:"symbol"`
	ChainID         int64  `json:"chain_id"`
	ContractAddress string `json:"contract_address"`
	BaseURI         string `json:"base_uri"`
	TotalSupply     int64  `json:"total_supply"`
	Status          string `json:"status"`
}

// NFTToken is one minted token. OwnerUserID is the platform account holding it;
// while a listing is live the holder is the marketplace escrow account and
// EscrowListingID names that listing.
type NFTToken struct {
	ID              string  `json:"id"`
	CollectionID    string  `json:"collection_id"`
	TokenID         uint64  `json:"token_id"`
	OwnerUserID     string  `json:"owner_user_id"`
	URI             string  `json:"uri"`
	EscrowListingID *string `json:"escrow_listing_id"`
}

// NFTListing is a fixed-price sale. Price is an exact integer string in the
// payment token's smallest unit, and FeeBps is frozen at listing time.
type NFTListing struct {
	ID                   string `json:"id"`
	CollectionID         string `json:"collection_id"`
	TokenID              uint64 `json:"token_id"`
	SellerUserID         string `json:"seller_user_id"`
	PaymentTokenAddress  string `json:"payment_token_address"`
	PaymentTokenSymbol   string `json:"payment_token_symbol"`
	PaymentTokenDecimals int    `json:"payment_token_decimals"`
	Price                string `json:"price"`
	FeeBps               int    `json:"fee_bps"`
	Deadline             string `json:"deadline"`
	Status               string `json:"status"`
}

// NFTOffer is an escrowed bid. Amount is an exact integer string in the payment
// token's smallest unit.
type NFTOffer struct {
	ID                   string `json:"id"`
	CollectionID         string `json:"collection_id"`
	TokenID              uint64 `json:"token_id"`
	BuyerUserID          string `json:"buyer_user_id"`
	PaymentTokenAddress  string `json:"payment_token_address"`
	PaymentTokenSymbol   string `json:"payment_token_symbol"`
	PaymentTokenDecimals int    `json:"payment_token_decimals"`
	Amount               string `json:"amount"`
	FeeBps               int    `json:"fee_bps"`
	Deadline             string `json:"deadline"`
	Status               string `json:"status"`
}

// NFTListingView is a live listing joined with its token and collection, so a
// caller renders a card without a second round trip.
type NFTListingView struct {
	Listing    NFTListing    `json:"listing"`
	Token      NFTToken      `json:"token"`
	Collection NFTCollection `json:"collection"`
	Active     bool          `json:"active"`
}

// NFTOfferView is a live offer joined with its token and collection.
type NFTOfferView struct {
	Offer      NFTOffer      `json:"offer"`
	Token      NFTToken      `json:"token"`
	Collection NFTCollection `json:"collection"`
	Active     bool          `json:"active"`
}

// NFTMarketplaceConfig is the marketplace fee configuration.
type NFTMarketplaceConfig struct {
	FeeBps        int    `json:"fee_bps"`
	MaxFeeBps     int    `json:"max_fee_bps"`
	FeeAccount    string `json:"fee_account"`
	EscrowAccount string `json:"escrow_account"`
}

// GetFarms returns the active farms a user can stake in.
func (c *Client) GetFarms(ctx context.Context) ([]FarmView, error) {
	req, err := http.NewRequest("GET", c.baseURL+"/api/v1/farms", nil)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get farms: unexpected status %d", resp.StatusCode)
	}

	var payload struct {
		Farms []FarmView `json:"farms"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	return payload.Farms, nil
}

// GetFarmStakes returns the authenticated caller's farm positions. The API key
// identifies the caller; no user id is ever sent in the request.
func (c *Client) GetFarmStakes(ctx context.Context) ([]FarmStake, error) {
	req, err := http.NewRequest("GET", c.baseURL+"/api/v1/farms/stakes", nil)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get farm stakes: unexpected status %d", resp.StatusCode)
	}

	var payload struct {
		Stakes []FarmStake `json:"stakes"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	return payload.Stakes, nil
}

// StakeFarm escrows `amount` of a farm's staking token, in raw units.
func (c *Client) StakeFarm(ctx context.Context, farmID, amount string) error {
	return c.postFarmAction(ctx, farmID, "stake", amount)
}

// UnstakeFarm returns staked tokens to the caller's balance.
func (c *Client) UnstakeFarm(ctx context.Context, farmID, amount string) error {
	return c.postFarmAction(ctx, farmID, "unstake", amount)
}

func (c *Client) postFarmAction(ctx context.Context, farmID, action, amount string) error {
	body, err := json.Marshal(map[string]string{"amount": amount})
	if err != nil {
		return err
	}
	url := fmt.Sprintf("%s/api/v1/farms/%s/%s", c.baseURL, farmID, action)
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req = req.WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("farm %s: unexpected status %d", action, resp.StatusCode)
	}
	return nil
}

// GetAirdrops returns the campaigns currently open for claims.
func (c *Client) GetAirdrops(ctx context.Context) ([]AirdropCampaign, error) {
	req, err := http.NewRequest("GET", c.baseURL+"/api/v1/airdrops", nil)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get airdrops: unexpected status %d", resp.StatusCode)
	}

	var payload struct {
		Campaigns []AirdropCampaign `json:"campaigns"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	return payload.Campaigns, nil
}

// GetMyAirdrops returns the authenticated caller's airdrop entitlements.
func (c *Client) GetMyAirdrops(ctx context.Context) ([]AirdropAllocation, error) {
	req, err := http.NewRequest("GET", c.baseURL+"/api/v1/airdrops/allocations", nil)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get airdrop allocations: unexpected status %d", resp.StatusCode)
	}

	var payload struct {
		Allocations []AirdropAllocation `json:"allocations"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	return payload.Allocations, nil
}

// ClaimAirdrop claims the caller's allocation in a campaign.
func (c *Client) ClaimAirdrop(ctx context.Context, campaignID string) error {
	url := fmt.Sprintf("%s/api/v1/airdrops/%s/claim", c.baseURL, campaignID)
	req, err := http.NewRequest("POST", url, bytes.NewReader([]byte("{}")))
	if err != nil {
		return err
	}
	req = req.WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("claim airdrop: unexpected status %d", resp.StatusCode)
	}
	return nil
}

// GetNFTCollections returns the active NFT collections and their minted supply.
func (c *Client) GetNFTCollections(ctx context.Context) ([]NFTCollection, error) {
	req, err := http.NewRequest("GET", c.baseURL+"/api/v1/nft/collections", nil)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get nft collections: unexpected status %d", resp.StatusCode)
	}

	var payload struct {
		Collections []NFTCollection `json:"collections"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	return payload.Collections, nil
}

// GetNFTListings returns the live listings, each joined with its token and
// collection. A listing escrows the token, so the joined token's owner is the
// marketplace escrow account until the listing settles or is cancelled.
func (c *Client) GetNFTListings(ctx context.Context) ([]NFTListingView, error) {
	req, err := http.NewRequest("GET", c.baseURL+"/api/v1/nft/listings", nil)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get nft listings: unexpected status %d", resp.StatusCode)
	}

	var payload struct {
		Listings []NFTListingView `json:"listings"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	return payload.Listings, nil
}

// GetNFTOffers returns the live offers on one token, each joined with its token
// and collection.
func (c *Client) GetNFTOffers(ctx context.Context, collectionID string, tokenID uint64) ([]NFTOfferView, error) {
	url := fmt.Sprintf("%s/api/v1/nft/offers?collection_id=%s&token_id=%d",
		c.baseURL, url.QueryEscape(collectionID), tokenID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get nft offers: unexpected status %d", resp.StatusCode)
	}

	var payload struct {
		Offers []NFTOfferView `json:"offers"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	return payload.Offers, nil
}

// GetNFTConfig returns the marketplace fee configuration.
func (c *Client) GetNFTConfig(ctx context.Context) (*NFTMarketplaceConfig, error) {
	req, err := http.NewRequest("GET", c.baseURL+"/api/v1/nft/config", nil)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get nft config: unexpected status %d", resp.StatusCode)
	}

	var config NFTMarketplaceConfig
	if err := json.NewDecoder(resp.Body).Decode(&config); err != nil {
		return nil, err
	}
	return &config, nil
}

// GetChains returns all supported chains
func (c *Client) GetChains(ctx context.Context) ([]Chain, error) {
	req, err := http.NewRequest("GET", c.baseURL+"/api/v1/chains", nil)
	if err != nil {
		return nil, err
	}

	req = req.WithContext(ctx)
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var chains []Chain
	if err := json.NewDecoder(resp.Body).Decode(&chains); err != nil {
		return nil, err
	}

	return chains, nil
}

// GetTokens returns all tokens for a chain
func (c *Client) GetTokens(ctx context.Context, chainID int64) ([]Token, error) {
	url := fmt.Sprintf("%s/api/v1/tokens?chainId=%d", c.baseURL, chainID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req = req.WithContext(ctx)
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var tokens []Token
	if err := json.NewDecoder(resp.Body).Decode(&tokens); err != nil {
		return nil, err
	}

	return tokens, nil
}

// GetQuote returns a swap quote
func (c *Client) GetQuote(ctx context.Context, fromToken, toToken, amount string, chainID int64) (*SwapQuote, error) {
	reqBody := map[string]interface{}{
		"token_in":  fromToken,
		"token_out": toToken,
		"amount_in": amount,
		"chain_id":  chainID,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", c.baseURL+"/api/v1/swap/quote", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req = req.WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var quote SwapQuote
	if err := json.NewDecoder(resp.Body).Decode(&quote); err != nil {
		return nil, err
	}

	return &quote, nil
}

// GetPools returns all pools
func (c *Client) GetPools(ctx context.Context) ([]Pool, error) {
	req, err := http.NewRequest("GET", c.baseURL+"/api/v1/swap/pairs", nil)
	if err != nil {
		return nil, err
	}

	req = req.WithContext(ctx)
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var pools []Pool
	if err := json.NewDecoder(resp.Body).Decode(&pools); err != nil {
		return nil, err
	}

	return pools, nil
}

// GetMarketStats returns market statistics
func (c *Client) GetMarketStats(ctx context.Context) (*MarketStats, error) {
	req, err := http.NewRequest("GET", c.baseURL+"/api/v1/market/stats", nil)
	if err != nil {
		return nil, err
	}

	req = req.WithContext(ctx)
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var stats MarketStats
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		return nil, err
	}

	return &stats, nil
}

// ExecuteSwap executes a swap transaction
func (c *Client) ExecuteSwap(ctx context.Context, fromToken, toToken, amount, to string) (*Transaction, error) {
	reqBody := map[string]interface{}{
		"token_in":  fromToken,
		"token_out": toToken,
		"amount_in": amount,
		"recipient": to,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", c.baseURL+"/api/v1/swap/execute", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req = req.WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Success     bool        `json:"success"`
		Transaction Transaction `json:"transaction"`
		Message     string      `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result.Transaction, nil
}
