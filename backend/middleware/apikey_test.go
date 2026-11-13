package middleware

import (
	"testing"
)

func TestMaskKeyDoesNotRevealFullKey(t *testing.T) {
	key := "tsk_supersecretvalue"
	masked := MaskKey(key)
	if masked == key {
		t.Fatal("a masked key must not equal the key")
	}
	if len(masked) == 0 || masked == "****" {
		t.Fatalf("a long key should be partially shown, got %q", masked)
	}
	if masked == MaskKey("tsk_supersecretvaluf") {
		t.Fatal("two different keys must not mask to the same value")
	}
	if got := MaskKey("short"); got != "****" {
		t.Fatalf("a short value must be fully masked, got %q", got)
	}
}

func TestParseAPIKeyPrecedence(t *testing.T) {
	if got := ParseAPIKey("header-key", "Bearer auth-key", "query-key"); got != "header-key" {
		t.Fatalf("header should win, got %q", got)
	}
	if got := ParseAPIKey("", "Bearer auth-key", "query-key"); got != "auth-key" {
		t.Fatalf("bearer should be used when no header, got %q", got)
	}
	if got := ParseAPIKey("", "", "query-key"); got != "query-key" {
		t.Fatalf("query should be the last resort, got %q", got)
	}
	if got := ParseAPIKey("", "Basic abc", "query-key"); got != "query-key" {
		t.Fatalf("a non-bearer authorization must fall through, got %q", got)
	}
	if got := ParseAPIKey("", "", ""); got != "" {
		t.Fatalf("no credential should parse to empty, got %q", got)
	}
}
