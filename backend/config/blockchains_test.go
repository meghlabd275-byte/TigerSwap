package config

import "testing"

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
