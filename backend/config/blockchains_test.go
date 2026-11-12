package config

import (
	"strings"
	"testing"
)

// TestTokenDecimalsReturnsRegisteredDecimals verifies decimals come from the
// token registry, which is what stops a 6-decimal stablecoin from being scaled
// as though it had 18 decimals.
func TestTokenDecimalsReturnsRegisteredDecimals(t *testing.T) {
	// USDC on Ethereum is registered with 6 decimals.
	const usdcEthereum = "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48"
	decimals, ok := TokenDecimals(1, usdcEthereum)
	if !ok {
		t.Fatal("Ethereum USDC should be a known token")
	}
	if decimals != 6 {
		t.Fatalf("Ethereum USDC decimals = %d, want 6", decimals)
	}

	// Lookup is case-insensitive, since callers pass addresses from mixed sources.
	if lower, ok := TokenDecimals(1, "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48"); !ok || lower != 6 {
		t.Fatalf("lowercase USDC lookup = %d/%v, want 6/true", lower, ok)
	}
}

// TestTokenDecimalsResolvesNativeAsset verifies the zero address maps to the
// chain's native asset rather than being treated as an unknown token.
func TestTokenDecimalsResolvesNativeAsset(t *testing.T) {
	decimals, ok := TokenDecimals(1, "0x0000000000000000000000000000000000000000")
	if !ok {
		t.Fatal("the zero address should resolve to the native asset")
	}
	if decimals != 18 {
		t.Fatalf("native ETH decimals = %d, want 18", decimals)
	}
}

// TestTokenDecimalsReportsUnknownToken verifies an unrecognised address is
// reported as unknown so a caller does not scale an amount by a guessed value.
func TestTokenDecimalsReportsUnknownToken(t *testing.T) {
	if _, ok := TokenDecimals(1, "0x000000000000000000000000000000000000dEaD"); ok {
		t.Fatal("an unregistered token must not report decimals")
	}
	if _, ok := TokenDecimals(1, ""); ok {
		t.Fatal("an empty address must not report decimals")
	}
	if _, ok := TokenDecimals(999999, "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48"); ok {
		t.Fatal("an unknown chain must not report decimals")
	}
}

// TestTigerSmartChainIsRegistered verifies TigerEx's own chain (6666, native
// coin TGR) is a first-class EVM chain, so the swap, perp and bridge paths can
// resolve it like any other network.
func TestTigerSmartChainIsRegistered(t *testing.T) {
	cfg, ok := GetBlockchainConfigs()[6666]
	if !ok {
		t.Fatal("TigerSmartChain (6666) must be a registered chain")
	}
	if cfg.Type != ChainTypeEVM {
		t.Fatalf("TigerSmartChain type = %q, want %q", cfg.Type, ChainTypeEVM)
	}
	if cfg.NativeToken == nil || cfg.NativeToken.Symbol != "TGR" {
		t.Fatalf("TigerSmartChain native token = %+v, want TGR", cfg.NativeToken)
	}
	if cfg.ChainIDHex != "0x1a0a" {
		t.Fatalf("TigerSmartChain chain id hex = %q, want 0x1a0a", cfg.ChainIDHex)
	}
	if !IsEVMChain(6666) {
		t.Fatal("TigerSmartChain must be reported as an EVM chain")
	}
}

// TestTigerSmartChainNativeTokenResolution verifies TGR resolves to the zero
// address and back, which is how the router refers to a chain's native coin.
func TestTigerSmartChainNativeTokenResolution(t *testing.T) {
	addr, ok := TokenAddress(6666, "TGR")
	if !ok {
		t.Fatal("TGR must resolve on TigerSmartChain")
	}
	if addr != "0x0000000000000000000000000000000000000000" {
		t.Fatalf("TGR address = %q, want the zero address", addr)
	}
	symbol, ok := TokenSymbol(6666, addr)
	if !ok || symbol != "TGR" {
		t.Fatalf("zero address on 6666 = %q/%v, want TGR/true", symbol, ok)
	}
	decimals, ok := TokenDecimals(6666, addr)
	if !ok || decimals != 18 {
		t.Fatalf("TGR decimals = %d/%v, want 18/true", decimals, ok)
	}
}

// TestOperatorTokenConfigRegistersDeploymentToken verifies an operator can
// declare a chain-local ERC-20 (RUSD has no public registry) and have it
// resolve by symbol and address with the stated decimals.
func TestOperatorTokenConfigRegistersDeploymentToken(t *testing.T) {
	const rusd = "0x1111111111111111111111111111111111111111"
	t.Setenv("TIGERSWAP_TOKEN_6666_RUSD", rusd+",18")

	addr, ok := TokenAddress(6666, "RUSD")
	if !ok {
		t.Fatal("operator-declared RUSD must resolve by symbol")
	}
	if !strings.EqualFold(addr, rusd) {
		t.Fatalf("RUSD address = %q, want %q", addr, rusd)
	}
	if symbol, ok := TokenSymbol(6666, rusd); !ok || symbol != "RUSD" {
		t.Fatalf("RUSD symbol lookup = %q/%v, want RUSD/true", symbol, ok)
	}
	if decimals, ok := TokenDecimals(6666, rusd); !ok || decimals != 18 {
		t.Fatalf("RUSD decimals = %d/%v, want 18/true", decimals, ok)
	}

	// A six-decimal stablecoin keeps its stated decimals.
	const usdt = "0x2222222222222222222222222222222222222222"
	t.Setenv("TIGERSWAP_TOKEN_6666_USDT", usdt+",6")
	if decimals, ok := TokenDecimals(6666, usdt); !ok || decimals != 6 {
		t.Fatalf("USDT decimals = %d/%v, want 6/true", decimals, ok)
	}
}

// TestOperatorTokenConfigRejectsMalformedAddress verifies a malformed operator
// value is dropped rather than becoming a token that can never be dialed.
func TestOperatorTokenConfigRejectsMalformedAddress(t *testing.T) {
	t.Setenv("TIGERSWAP_TOKEN_6666_BAD", "not-an-address")
	if _, ok := TokenAddress(6666, "BAD"); ok {
		t.Fatal("a malformed operator address must not register a token")
	}
	t.Setenv("TIGERSWAP_TOKEN_6666_SHORT", "0x1234")
	if _, ok := TokenAddress(6666, "SHORT"); ok {
		t.Fatal("a short operator address must not register a token")
	}
}

// TestOperatorTokenOverridesStaticEntry verifies an operator address replaces
// the static entry for the same symbol, so the precedence is deterministic.
func TestOperatorTokenOverridesStaticEntry(t *testing.T) {
	const usdc = "0x3333333333333333333333333333333333333333"
	t.Setenv("TIGERSWAP_TOKEN_1_USDC", usdc+",6")
	addr, ok := TokenAddress(1, "USDC")
	if !ok {
		t.Fatal("USDC must resolve on chain 1")
	}
	if !strings.EqualFold(addr, usdc) {
		t.Fatalf("USDC address = %q, want the operator override %q", addr, usdc)
	}
}
