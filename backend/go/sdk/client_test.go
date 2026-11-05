package tigerswap

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// These tests drive the SDK against a server that answers exactly like the real
// TigerSwap API: bare camelCase arrays, camelCase quote fields, and the
// `amount_in` / `chainId` request spellings. They exist because the SDK and the
// API drifted apart once already, and the failure was silent - the SDK decoded
// an envelope key that no longer existed and returned an empty slice.

func TestGetChainsDecodesBareArray(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
                        {"id":1,"name":"Ethereum","chainType":"evm","nativeSymbol":"ETH","explorerURL":"https://etherscan.io","isActive":true},
                        {"id":101,"name":"Solana","chainType":"solana","nativeSymbol":"SOL","explorerURL":null,"isActive":true}
                ]`))
	}))
	defer server.Close()

	chains, err := NewClient(server.URL, "").GetChains(context.Background())
	if err != nil {
		t.Fatalf("GetChains: %v", err)
	}
	if len(chains) != 2 {
		t.Fatalf("expected 2 chains, got %d", len(chains))
	}
	if chains[0].ID != 1 || chains[0].ChainType != "evm" || chains[0].NativeSymbol != "ETH" {
		t.Errorf("chain[0] = %+v", chains[0])
	}
	if chains[1].ExplorerURL != nil {
		t.Errorf("chain[1].ExplorerURL should be nil for null, got %v", *chains[1].ExplorerURL)
	}
}

func TestGetTokensDecodesBareArray(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("chainId"); got != "1" {
			t.Errorf("expected chainId=1, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
                        {"address":"0x1","symbol":"TGR","name":"TigerSwap","decimals":18,"chainId":1,"logoURI":null,"priceUSD":null}
                ]`))
	}))
	defer server.Close()

	tokens, err := NewClient(server.URL, "").GetTokens(context.Background(), 1)
	if err != nil {
		t.Fatalf("GetTokens: %v", err)
	}
	if len(tokens) != 1 || tokens[0].Symbol != "TGR" || tokens[0].Decimals != 18 {
		t.Fatalf("unexpected tokens: %+v", tokens)
	}
	if tokens[0].PriceUSD != nil {
		t.Errorf("priceUSD should be nil for null, got %v", *tokens[0].PriceUSD)
	}
}

func TestGetPoolsDecodesBareArray(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
                        {"pairAddress":"0x3","token0":"0x1","token1":"0x2","symbol0":"TGR","symbol1":"RUSD","reserve0":1000.5,"reserve1":2000.25,"feeBps":30}
                ]`))
	}))
	defer server.Close()

	pools, err := NewClient(server.URL, "").GetPools(context.Background())
	if err != nil {
		t.Fatalf("GetPools: %v", err)
	}
	if len(pools) != 1 {
		t.Fatalf("expected 1 pool, got %d", len(pools))
	}
	if pools[0].FeeBps != 30 || pools[0].Reserve1 != 2000.25 {
		t.Errorf("pool = %+v", pools[0])
	}
}

func TestGetQuoteSendsAndDecodesContractFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request: %v", err)
		}
		// The API binds `amount_in`; sending `amount` yields a 400.
		if _, ok := body["amount_in"]; !ok {
			t.Errorf("request is missing amount_in: %v", body)
		}
		if _, ok := body["amount"]; ok {
			t.Errorf("request still sends the legacy `amount` key: %v", body)
		}
		// chain_id is a JSON number in the contract, not a string.
		if _, ok := body["chain_id"].(float64); !ok {
			t.Errorf("chain_id must be a JSON number, got %T", body["chain_id"])
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
                        "amountIn":"1000000000000000000",
                        "amountOut":"2000000000000000000",
                        "amountOutMin":"1990000000000000000",
                        "priceImpact":0.12,
                        "route":["0x1","0x2"],
                        "gasEstimate":150000
                }`))
	}))
	defer server.Close()

	quote, err := NewClient(server.URL, "").GetQuote(context.Background(), "0x1", "0x2", "1000000000000000000", 1)
	if err != nil {
		t.Fatalf("GetQuote: %v", err)
	}
	if quote.AmountOut != "2000000000000000000" || quote.AmountOutMin != "1990000000000000000" {
		t.Errorf("quote amounts = %+v", quote)
	}
	if quote.PriceImpact != 0.12 || quote.GasEstimate != 150000 {
		t.Errorf("quote impact/gas = %+v", quote)
	}
	if len(quote.Route) != 2 {
		t.Errorf("quote route = %v", quote.Route)
	}
}

func TestGetMarketStatsDecodesCamelCase(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"totalTVL":500000.5,"volume24h":15000,"totalPairs":7,"totalUsers":42,"updatedAt":"2026-10-31T04:04:03Z"}`))
	}))
	defer server.Close()

	stats, err := NewClient(server.URL, "").GetMarketStats(context.Background())
	if err != nil {
		t.Fatalf("GetMarketStats: %v", err)
	}
	if stats.TotalTVL != 500000.5 || stats.Volume24h != 15000 {
		t.Errorf("stats = %+v", stats)
	}
	if stats.TotalPairs != 7 || stats.TotalUsers != 42 {
		t.Errorf("stats counts = %+v", stats)
	}
}

// The farm and airdrop methods wrap the API's `{ "farms": [...] }` and
// `{ "stakes": [...] }` envelopes. A reader that decoded the body as a bare
// array would silently return nothing, so these tests pin the envelope keys and
// the paths.

func TestGetFarmsDecodesEnvelope(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/farms" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"farms":[{"farm":{"id":"f1","key":"tgr-rusd","name":"TGR/RUSD LP","farm_type":"lp","chain_id":1,"staking_token_address":"0x1","staking_token_symbol":"LP","staking_token_decimals":18,"total_staked":"1000","start_time":"2026-11-01T00:00:00Z","end_time":"2026-12-01T00:00:00Z","status":"active"},"rewards":[{"id":"r1","farm_id":"f1","token_address":"0x2","token_symbol":"TGR","token_decimals":18,"rate_per_second":"10"}],"apr":12.5},{"farm":{"id":"f2","key":"paused","name":"Paused","farm_type":"single","chain_id":1,"staking_token_address":"0x3","staking_token_symbol":"TGR","staking_token_decimals":18,"total_staked":"0","start_time":"2026-11-01T00:00:00Z","end_time":"2026-12-01T00:00:00Z","status":"paused"},"rewards":[],"apr":null}],"success":true}`))
	}))
	defer server.Close()

	farms, err := NewClient(server.URL, "").GetFarms(context.Background())
	if err != nil {
		t.Fatalf("GetFarms: %v", err)
	}
	if len(farms) != 2 {
		t.Fatalf("expected 2 farms, got %d", len(farms))
	}
	if farms[0].Farm.Key != "tgr-rusd" || farms[0].Farm.TotalStaked != "1000" {
		t.Errorf("farm[0] = %+v", farms[0])
	}
	if len(farms[0].Rewards) != 1 || farms[0].Rewards[0].TokenSymbol != "TGR" {
		t.Errorf("farm[0] rewards = %+v", farms[0].Rewards)
	}
	if farms[0].APR == nil || *farms[0].APR != 12.5 {
		t.Errorf("farm[0] APR should be 12.5, got %v", farms[0].APR)
	}
	// A null APR means no price source is wired; it must stay nil, not become 0.
	if farms[1].APR != nil {
		t.Errorf("farm[1] APR should be nil for null, got %v", *farms[1].APR)
	}
}

func TestGetFarmStakesDecodesEnvelope(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/farms/stakes" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"stakes":[{"stake_id":"s1","farm_id":"f1","farm_key":"tgr-rusd","farm_name":"TGR/RUSD LP","status":"active","chain_id":1,"staking_token_address":"0x1","staking_token_symbol":"LP","staked":"500","end_time":"2026-12-01T00:00:00Z","pending_rewards":[{"token_address":"0x2","symbol":"TGR","decimals":18,"pending":"25"}]}],"success":true}`))
	}))
	defer server.Close()

	stakes, err := NewClient(server.URL, "key").GetFarmStakes(context.Background())
	if err != nil {
		t.Fatalf("GetFarmStakes: %v", err)
	}
	if len(stakes) != 1 {
		t.Fatalf("expected 1 stake, got %d", len(stakes))
	}
	if stakes[0].Staked != "500" || len(stakes[0].PendingRewards) != 1 {
		t.Errorf("stake = %+v", stakes[0])
	}
	if stakes[0].PendingRewards[0].Pending != "25" {
		t.Errorf("pending = %+v", stakes[0].PendingRewards[0])
	}
}

func TestFarmActionsPostToTheRightPath(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.Method+" "+r.URL.Path)
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode body: %v", err)
		}
		if body["amount"] == "" {
			t.Errorf("amount missing from request: %v", body)
		}
		if r.Header.Get("Authorization") == "" {
			t.Errorf("stake/unstake must be authenticated")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "key")
	if err := client.StakeFarm(context.Background(), "f1", "100"); err != nil {
		t.Fatalf("StakeFarm: %v", err)
	}
	if err := client.UnstakeFarm(context.Background(), "f1", "50"); err != nil {
		t.Fatalf("UnstakeFarm: %v", err)
	}
	want := []string{"POST /api/v1/farms/f1/stake", "POST /api/v1/farms/f1/unstake"}
	if len(paths) != 2 || paths[0] != want[0] || paths[1] != want[1] {
		t.Fatalf("paths = %v, want %v", paths, want)
	}
}

func TestGetAirdropsAndClaim(t *testing.T) {
	var claimPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v1/airdrops":
			_, _ = w.Write([]byte(`{"campaigns":[{"id":"c1","key":"tgr-genesis","name":"TGR Genesis","chain_id":1,"token_address":"0x3","token_symbol":"TGR","token_decimals":18,"total_allocated":"750","funded":"750","total_claimed":"500","start_time":"2026-11-01T00:00:00Z","end_time":"2026-12-01T00:00:00Z","status":"active"}],"success":true}`))
		case r.URL.Path == "/api/v1/airdrops/allocations":
			_, _ = w.Write([]byte(`{"allocations":[{"allocation_id":"a1","campaign_id":"c1","campaign_key":"tgr-genesis","campaign_name":"TGR Genesis","token_address":"0x3","symbol":"TGR","amount":"500","claimed":false,"status":"active","start_time":"2026-11-01T00:00:00Z","end_time":"2026-12-01T00:00:00Z"}],"success":true}`))
		case r.URL.Path == "/api/v1/airdrops/c1/claim":
			claimPath = r.URL.Path
			_, _ = w.Write([]byte(`{"success":true,"campaign_id":"c1","token_address":"0x3","symbol":"TGR","amount":"500"}`))
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "key")
	campaigns, err := client.GetAirdrops(context.Background())
	if err != nil {
		t.Fatalf("GetAirdrops: %v", err)
	}
	if len(campaigns) != 1 || campaigns[0].Funded != "750" {
		t.Fatalf("campaigns = %+v", campaigns)
	}

	allocations, err := client.GetMyAirdrops(context.Background())
	if err != nil {
		t.Fatalf("GetMyAirdrops: %v", err)
	}
	if len(allocations) != 1 || allocations[0].Claimed {
		t.Fatalf("allocations = %+v", allocations)
	}

	if err := client.ClaimAirdrop(context.Background(), "c1"); err != nil {
		t.Fatalf("ClaimAirdrop: %v", err)
	}
	if claimPath != "/api/v1/airdrops/c1/claim" {
		t.Fatalf("claim path = %q", claimPath)
	}
}

func TestFarmRequestsFailOnNonOKStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"error":"farm is not active"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "key")
	if err := client.StakeFarm(context.Background(), "f1", "100"); err == nil {
		t.Fatal("StakeFarm should report a non-200 status instead of reporting success")
	}
	if _, err := client.GetFarms(context.Background()); err == nil {
		t.Fatal("GetFarms should report a non-200 status instead of returning an empty slice")
	}
}

func TestNFTReadsDecodeTheMarketplaceSurface(t *testing.T) {
	var offersQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/nft/collections":
			_, _ = w.Write([]byte(`{"collections":[{"id":"col-1","key":"tiger-art","name":"Tiger Art","symbol":"TART","chain_id":1,"contract_address":"0x5555555555555555555555555555555555555555","base_uri":"https://cdn.tigerswap.example/nft/","total_supply":2,"status":"active"}],"success":true}`))
		case "/api/v1/nft/listings":
			_, _ = w.Write([]byte(`{"listings":[{"listing":{"id":"lst-1","collection_id":"col-1","token_id":1,"seller_user_id":"seller-1","payment_token_address":"0x4444444444444444444444444444444444444444","payment_token_symbol":"RUSD","payment_token_decimals":18,"price":"100000","fee_bps":250,"deadline":"2026-11-06T13:20:05Z","status":"active"},"token":{"id":"tok-1","collection_id":"col-1","token_id":1,"owner_user_id":"__nft_escrow__","uri":"token.json","escrow_listing_id":"lst-1"},"collection":{"id":"col-1","key":"tiger-art","name":"Tiger Art","symbol":"TART","chain_id":1,"contract_address":"","base_uri":"","total_supply":1,"status":"active"},"active":true}],"success":true}`))
		case "/api/v1/nft/offers":
			offersQuery = r.URL.RawQuery
			_, _ = w.Write([]byte(`{"offers":[{"offer":{"id":"off-1","collection_id":"col-1","token_id":2,"buyer_user_id":"buyer-2","payment_token_address":"0x4444444444444444444444444444444444444444","payment_token_symbol":"RUSD","payment_token_decimals":18,"amount":"50000","fee_bps":250,"deadline":"2026-11-06T13:20:05Z","status":"active"},"token":{"id":"tok-2","collection_id":"col-1","token_id":2,"owner_user_id":"seller-2","uri":"token.json","escrow_listing_id":null},"collection":{"id":"col-1","key":"tiger-art","name":"Tiger Art","symbol":"TART","chain_id":1,"contract_address":"","base_uri":"","total_supply":2,"status":"active"},"active":true}],"success":true}`))
		case "/api/v1/nft/config":
			_, _ = w.Write([]byte(`{"escrow_account":"__nft_escrow__","fee_account":"__nft_fee__","fee_bps":250,"max_fee_bps":500,"success":true}`))
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "")

	collections, err := client.GetNFTCollections(context.Background())
	if err != nil {
		t.Fatalf("GetNFTCollections: %v", err)
	}
	if len(collections) != 1 || collections[0].TotalSupply != 2 {
		t.Fatalf("collections = %+v", collections)
	}

	listings, err := client.GetNFTListings(context.Background())
	if err != nil {
		t.Fatalf("GetNFTListings: %v", err)
	}
	if len(listings) != 1 || listings[0].Listing.Price != "100000" || listings[0].Listing.FeeBps != 250 {
		t.Fatalf("listings = %+v", listings)
	}
	// A listed token is escrowed, so the joined token carries the escrow id.
	if listings[0].Token.EscrowListingID == nil || *listings[0].Token.EscrowListingID != "lst-1" {
		t.Fatalf("listing token escrow = %+v", listings[0].Token.EscrowListingID)
	}
	if listings[0].Collection.Key != "tiger-art" {
		t.Fatalf("listing collection = %+v", listings[0].Collection)
	}

	offers, err := client.GetNFTOffers(context.Background(), "col-1", 2)
	if err != nil {
		t.Fatalf("GetNFTOffers: %v", err)
	}
	if len(offers) != 1 || offers[0].Offer.Amount != "50000" {
		t.Fatalf("offers = %+v", offers)
	}
	// A token that is not escrowed has a nil escrow id, not an empty one.
	if offers[0].Token.EscrowListingID != nil {
		t.Fatalf("offer token escrow should be nil, got %v", *offers[0].Token.EscrowListingID)
	}
	if offersQuery != "collection_id=col-1&token_id=2" {
		t.Fatalf("offers query = %q", offersQuery)
	}

	config, err := client.GetNFTConfig(context.Background())
	if err != nil {
		t.Fatalf("GetNFTConfig: %v", err)
	}
	if config.FeeBps != 250 || config.MaxFeeBps != 500 || config.EscrowAccount != "__nft_escrow__" {
		t.Fatalf("config = %+v", config)
	}
}

func TestNFTReadsFailOnNonOKStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":"ledger is not configured"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "")
	if _, err := client.GetNFTListings(context.Background()); err == nil {
		t.Fatal("GetNFTListings should report a non-200 status instead of returning an empty slice")
	}
	if _, err := client.GetNFTConfig(context.Background()); err == nil {
		t.Fatal("GetNFTConfig should report a non-200 status instead of returning a zero config")
	}
}
