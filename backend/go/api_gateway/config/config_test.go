package config

import "testing"

// TestParseTrustedProxiesIgnoresWildcard pins that a wildcard never enables the
// forwarding-header trust. gin trusts every peer by default, so a wildcard here
// would let a client set its own client IP and bypass the rate limiter.
func TestParseTrustedProxiesIgnoresWildcard(t *testing.T) {
	cases := map[string]int{
		"":                          0,
		"*":                         0,
		"0.0.0.0/0":                 0,
		"::/0":                      0,
		"10.0.0.0/8":                1,
		"10.0.0.0/8,192.168.1.1, *": 2,
	}
	for raw, want := range cases {
		if got := len(parseTrustedProxies(raw)); got != want {
			t.Errorf("parseTrustedProxies(%q) returned %d entries, want %d", raw, got, want)
		}
	}
}

// TestSanitizeCORSOriginsDropsWildcardWithCredentials pins that a credentialed
// gateway never advertises a wildcard origin. `Access-Control-Allow-Origin: *`
// with `Access-Control-Allow-Credentials: true` is rejected by browsers and, if
// it were honoured, would let any site drive the authenticated API.
func TestSanitizeCORSOriginsDropsWildcardWithCredentials(t *testing.T) {
	got := sanitizeCORSOrigins([]string{"*"}, true)
	if len(got) != 1 || got[0] == "*" {
		t.Fatalf("wildcard must not survive with credentials enabled, got %v", got)
	}

	// Without credentials the wildcard is harmless and preserved.
	if got := sanitizeCORSOrigins([]string{"*"}, false); len(got) != 1 || got[0] != "*" {
		t.Fatalf("wildcard should be preserved without credentials, got %v", got)
	}

	got = sanitizeCORSOrigins([]string{"*", "https://app.tigerswap.io"}, true)
	if len(got) != 1 || got[0] != "https://app.tigerswap.io" {
		t.Fatalf("explicit origin should survive, got %v", got)
	}
}
