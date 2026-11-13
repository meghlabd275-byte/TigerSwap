package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestSecretPrefersEnvironmentOverFile pins that a value set directly in the
// environment wins over a mounted secret file, so a local `.env` keeps working
// even when a deployment has also mounted the `_FILE` variant.
func TestSecretPrefersEnvironmentOverFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "enc")
	if err := os.WriteFile(path, []byte("from-file\n"), 0o600); err != nil {
		t.Fatalf("write secret file: %v", err)
	}
	t.Setenv("WALLET_ENCRYPTION_KEY", "from-env")
	t.Setenv("WALLET_ENCRYPTION_KEY_FILE", path)

	if got := Secret("WALLET_ENCRYPTION_KEY"); got != "from-env" {
		t.Fatalf("environment must win: got %q", got)
	}
}

// TestSecretReadsMountedFile pins the deployment path: a key exposed only as a
// mounted file (Docker secret / Kubernetes Secret volume) is returned, with the
// trailing newline a mounted secret commonly carries trimmed off.
func TestSecretReadsMountedFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "signer")
	if err := os.WriteFile(path, []byte("0xabc123\n"), 0o600); err != nil {
		t.Fatalf("write secret file: %v", err)
	}
	t.Setenv("PLATFORM_SIGNER_KEY", "")
	t.Setenv("PLATFORM_SIGNER_KEY_FILE", path)

	if got := Secret("PLATFORM_SIGNER_KEY"); got != "0xabc123" {
		t.Fatalf("mounted secret not read correctly: got %q", got)
	}
}

// TestSecretMissingReturnsEmpty pins that an unset key and an unreadable file
// both come back empty, so the caller reports the missing key exactly as it does
// for an unset variable rather than starting with a silent empty secret.
func TestSecretMissingReturnsEmpty(t *testing.T) {
	t.Setenv("TIGERBRIDGE_RELAYER_KEY", "")
	t.Setenv("TIGERBRIDGE_RELAYER_KEY_FILE", filepath.Join(t.TempDir(), "absent"))

	if got := Secret("TIGERBRIDGE_RELAYER_KEY"); got != "" {
		t.Fatalf("missing secret must be empty: got %q", got)
	}
}
