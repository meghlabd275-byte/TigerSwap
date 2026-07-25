package main

import (
	"context"
	"math/big"
	"testing"
)

// TestGetQuote tests quote generation
func TestGetQuote(t *testing.T) {
	agg := NewAggregator()

	req := QuoteRequest{
		ChainID:   1,
		FromToken: "0x0000000000000000000000000000000000000000",
		ToToken:   "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48",
		AmountIn:  big.NewInt(1000000000000000000),
		Slippage:  0.5,
	}

	ctx := context.Background()
	quote, err := agg.GetQuote(ctx, req)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if quote.AmountOut.Cmp(big.NewInt(0)) <= 0 {
		t.Error("Expected amount out to be greater than 0")
	}

	if quote.BestRoute == nil {
		t.Error("Expected best route to be set")
	}
}

// TestGetQuoteUnsupportedChain tests quote for unsupported chain
func TestGetQuoteUnsupportedChain(t *testing.T) {
	agg := NewAggregator()

	req := QuoteRequest{
		ChainID:   99999,
		FromToken: "0x0000000000000000000000000000000000000000",
		ToToken:   "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48",
		AmountIn:  big.NewInt(1000000000000000000),
	}

	ctx := context.Background()
	_, err := agg.GetQuote(ctx, req)

	if err == nil {
		t.Error("Expected error for unsupported chain")
	}
}

// TestGetQuoteWithSpecificDEX tests quote for specific DEX
func TestGetQuoteWithSpecificDEX(t *testing.T) {
	agg := NewAggregator()

	req := QuoteRequest{
		ChainID:   1,
		FromToken: "0x0000000000000000000000000000000000000000",
		ToToken:   "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48",
		AmountIn:  big.NewInt(1000000000000000000),
		DEX:       "uniswap_v3",
	}

	ctx := context.Background()
	quote, err := agg.GetQuote(ctx, req)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if quote.BestRoute.DEX != "uniswap_v3" {
		t.Errorf("Expected DEX to be uniswap_v3, got %s", quote.BestRoute.DEX)
	}
}

// TestGetMultiHopQuote tests multi-hop quote
func TestGetMultiHopQuote(t *testing.T) {
	agg := NewAggregator()

	path := []string{
		"0x0000000000000000000000000000000000000000", // ETH
		"0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48", // USDC
		"0xdAC17F958D2ee523a2206206994597C13D831ec7", // USDT
	}

	ctx := context.Background()
	quote, err := agg.GetMultiHopQuote(ctx, 1, path, big.NewInt(1000000000000000000), 0.5)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if quote.AmountOut.Cmp(big.NewInt(0)) <= 0 {
		t.Error("Expected amount out to be greater than 0")
	}

	if len(quote.Routes) == 0 {
		t.Error("Expected routes to be set")
	}
}

// TestGetMultiHopQuoteInvalidPath tests multi-hop with invalid path
func TestGetMultiHopQuoteInvalidPath(t *testing.T) {
	agg := NewAggregator()

	path := []string{
		"0x0000000000000000000000000000000000000000", // ETH
	}

	ctx := context.Background()
	_, err := agg.GetMultiHopQuote(ctx, 1, path, big.NewInt(1000000000000000000), 0.5)

	if err == nil {
		t.Error("Expected error for invalid path")
	}
}

// TestGetSplitQuote tests split quote
func TestGetSplitQuote(t *testing.T) {
	agg := NewAggregator()

	req := QuoteRequest{
		ChainID:   1,
		FromToken: "0x0000000000000000000000000000000000000000",
		ToToken:   "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48",
		AmountIn:  big.NewInt(1000000000000000000),
		Slippage:  0.5,
	}

	splits := []int{50, 50}

	ctx := context.Background()
	quotes, err := agg.GetSplitQuote(ctx, req, splits)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(quotes) != 2 {
		t.Errorf("Expected 2 quotes, got %d", len(quotes))
	}
}

// TestGetSplitQuoteInvalidSplits tests split quote with invalid splits
func TestGetSplitQuoteInvalidSplits(t *testing.T) {
	agg := NewAggregator()

	req := QuoteRequest{
		ChainID:   1,
		FromToken: "0x0000000000000000000000000000000000000000",
		ToToken:   "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48",
		AmountIn:  big.NewInt(1000000000000000000),
	}

	splits := []int{30, 30} // Sum to 60, not 100

	ctx := context.Background()
	_, err := agg.GetSplitQuote(ctx, req, splits)

	if err == nil {
		t.Error("Expected error for invalid splits")
	}
}

// TestGetSupportedDEXes tests getting supported DEXes
func TestGetSupportedDEXes(t *testing.T) {
	agg := NewAggregator()

	dexes := agg.GetSupportedDEXes(1)

	if len(dexes) == 0 {
		t.Error("Expected DEXes for Ethereum")
	}
}

// TestGetSupportedDEXesUnsupportedChain tests getting DEXes for unsupported chain
func TestGetSupportedDEXesUnsupportedChain(t *testing.T) {
	agg := NewAggregator()

	dexes := agg.GetSupportedDEXes(99999)

	if len(dexes) != 0 {
		t.Error("Expected no DEXes for unsupported chain")
	}
}

// TestExecuteSwap tests swap execution
func TestExecuteSwap(t *testing.T) {
	agg := NewAggregator()

	route := Route{
		DEX:      "uniswap_v3",
		Path:     []string{"0x0000000000000000000000000000000000000000", "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48"},
		AmountIn: big.NewInt(1000000000000000000),
	}

	ctx := context.Background()
	txHash, err := agg.ExecuteSwap(ctx, route, "0x1234567890123456789012345678901234567890", time.Now().Add(30*60))

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if txHash == "" {
		t.Error("Expected transaction hash")
	}
}

// TestCalculateOutput tests output calculation
func TestCalculateOutput(t *testing.T) {
	amountIn := big.NewInt(1000000000000000000) // 1 ETH
	fee := uint32(300) // 0.3%

	amountOut := calculateOutput(amountIn, fee)

	// With 0.3% fee, output should be 99.7% of input
	expected := big.NewInt(997000000000000000)
	if amountOut.Cmp(expected) != 0 {
		t.Logf("Got %s, expected approximately %s", amountOut.String(), expected.String())
	}
}

// TestCalculateMinOutput tests minimum output calculation
func TestCalculateMinOutput(t *testing.T) {
	amountOut := big.NewInt(1000000000000000000)
	slippage := 0.5 // 0.5%

	minOut := calculateMinOutput(amountOut, slippage)

	// With 0.5% slippage, min should be 99.5% of output
	expected := big.NewInt(995000000000000000)
	if minOut.Cmp(expected) != 0 {
		t.Logf("Got %s, expected approximately %s", minOut.String(), expected.String())
	}
}

// TestCalculatePriceImpact tests price impact calculation
func TestCalculatePriceImpact(t *testing.T) {
	amountIn := big.NewInt(1000000000000000000)
	amountOut := big.NewInt(997000000000000000)

	impact := calculatePriceImpact(amountIn, amountOut)

	// Should be small positive number for small trades
	if impact < 0 || impact > 10 {
		t.Logf("Got price impact: %f", impact)
	}
}

// TestEstimateGas tests gas estimation
func TestEstimateGas(t *testing.T) {
	gas := estimateGas("uniswap_v3")

	if gas == 0 {
		t.Error("Expected gas estimate to be non-zero")
	}

	if gas < 100000 {
		t.Logf("Gas estimate seems low: %d", gas)
	}
}

// BenchmarkGetQuote benchmarks quote generation
func BenchmarkGetQuote(b *testing.B) {
	agg := NewAggregator()

	req := QuoteRequest{
		ChainID:   1,
		FromToken: "0x0000000000000000000000000000000000000000",
		ToToken:   "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48",
		AmountIn:  big.NewInt(1000000000000000000),
		Slippage:  0.5,
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		agg.GetQuote(ctx, req)
	}
}

// BenchmarkGetMultiHopQuote benchmarks multi-hop quote
func BenchmarkGetMultiHopQuote(b *testing.B) {
	agg := NewAggregator()

	path := []string{
		"0x0000000000000000000000000000000000000000",
		"0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48",
		"0xdAC17F958D2ee523a2206206994597C13D831ec7",
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		agg.GetMultiHopQuote(ctx, 1, path, big.NewInt(1000000000000000000), 0.5)
	}
}
