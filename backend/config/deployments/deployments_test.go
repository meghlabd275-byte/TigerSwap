package deployments

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

const sample = `{
  "chain_id": 8453,
  "factory": "0x1111111111111111111111111111111111111111",
  "router": "0x2222222222222222222222222222222222222222",
  "quoter": "0x3333333333333333333333333333333333333333",
  "oracle": "0x4444444444444444444444444444444444444444",
  "perps": "0x5555555555555555555555555555555555555555",
  "lending": "0x6666666666666666666666666666666666666666",
  "bridge": "0x7777777777777777777777777777777777777777",
  "weth": "0x8888888888888888888888888888888888888888"
}`

func withTempDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	old := Dir
	Dir = dir
	t.Cleanup(func() { Dir = old })
	return dir
}

func TestLoadReadsAndValidates(t *testing.T) {
	dir := withTempDir(t)
	if err := os.WriteFile(filepath.Join(dir, "8453.json"), []byte(sample), 0o600); err != nil {
		t.Fatal(err)
	}
	r, err := Load(8453)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if r.ChainID != 8453 {
		t.Fatalf("chain id: got %d", r.ChainID)
	}
	addr, err := r.Address("perps")
	if err != nil {
		t.Fatalf("Address: %v", err)
	}
	if addr.Hex() != "0x5555555555555555555555555555555555555555" {
		t.Fatalf("perps address: got %s", addr.Hex())
	}
}

func TestLoadRejectsWrongChain(t *testing.T) {
	dir := withTempDir(t)
	if err := os.WriteFile(filepath.Join(dir, "8453.json"), []byte(sample), 0o600); err != nil {
		t.Fatal(err)
	}
	// The file declares 8453; asking for 1 must fail rather than return the
	// wrong chain's contracts.
	if _, err := Load(1); err == nil {
		t.Fatal("a registry whose chain_id disagrees with its filename must be rejected")
	}
}

func TestAddressRejectsMalformed(t *testing.T) {
	r := Registry{Perps: "0xnothex"}
	if _, err := r.Address("perps"); err == nil {
		t.Fatal("a malformed address must be rejected")
	}
	// An absent optional contract resolves to the zero address, not an error.
	r = Registry{}
	addr, err := r.Address("lending")
	if err != nil {
		t.Fatalf("absent field: %v", err)
	}
	if addr != (common.Address{}) {
		t.Fatalf("absent field should be the zero address, got %s", addr.Hex())
	}
}

func TestLoadAllSortedAndIgnoresNonRegistryFiles(t *testing.T) {
	dir := withTempDir(t)
	if err := os.WriteFile(filepath.Join(dir, "8453.json"), []byte(sample), 0o600); err != nil {
		t.Fatal(err)
	}
	other := `{"chain_id": 1, "factory": "0x1111111111111111111111111111111111111111"}`
	if err := os.WriteFile(filepath.Join(dir, "1.json"), []byte(other), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("not a registry"), 0o600); err != nil {
		t.Fatal(err)
	}
	all, err := LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(all) != 2 || all[0].ChainID != 1 || all[1].ChainID != 8453 {
		t.Fatalf("want chains 1,8453 in order, got %+v", all)
	}
}

func TestLoadAllMissingDirIsEmptyNotError(t *testing.T) {
	old := Dir
	Dir = filepath.Join(t.TempDir(), "does-not-exist")
	t.Cleanup(func() { Dir = old })
	all, err := LoadAll()
	if err != nil {
		t.Fatalf("a missing registry dir is not an error: %v", err)
	}
	if all != nil {
		t.Fatalf("want nil, got %+v", all)
	}
}
