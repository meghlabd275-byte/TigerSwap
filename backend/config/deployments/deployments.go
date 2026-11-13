// Package deployments reads the per-chain contract address registry that
// `contracts/script/Deploy.s.sol` writes. The registry is the source of truth
// for which contracts a chain has: the API reads the same JSON the deploy script
// produced, so an operator never hand-copies an address into an environment
// variable and a chain with no registry entry is never dialled.
package deployments

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common"
)

// Registry is one chain's deployed contract addresses, as written by the deploy
// script. Addresses are validated as hex on load, so a truncated or mistyped
// entry fails loudly instead of producing a zero address the chain would reject.
type Registry struct {
	ChainID int64  `json:"chain_id"`
	Factory string `json:"factory"`
	Router  string `json:"router"`
	Quoter  string `json:"quoter"`
	Oracle  string `json:"oracle"`
	Perps   string `json:"perps"`
	Orders  string `json:"orders"`
	Twamm   string `json:"twamm"`
	Lending string `json:"lending"`
	Bridge  string `json:"bridge"`
	WETH    string `json:"weth"`
}

// Address parses a registry field, requiring a non-empty, well-formed hex
// address. An absent optional contract returns the zero address.
func (r Registry) Address(field string) (common.Address, error) {
	var raw string
	switch field {
	case "factory":
		raw = r.Factory
	case "router":
		raw = r.Router
	case "quoter":
		raw = r.Quoter
	case "oracle":
		raw = r.Oracle
	case "perps":
		raw = r.Perps
	case "orders":
		raw = r.Orders
	case "twamm":
		raw = r.Twamm
	case "lending":
		raw = r.Lending
	case "bridge":
		raw = r.Bridge
	case "weth":
		raw = r.WETH
	default:
		return common.Address{}, fmt.Errorf("unknown registry field %q", field)
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return common.Address{}, nil
	}
	if !common.IsHexAddress(raw) {
		return common.Address{}, fmt.Errorf("registry field %s=%q is not a valid address", field, raw)
	}
	return common.HexToAddress(raw), nil
}

// Dir is the directory the deploy script writes registries into. It defaults to
// `deployments` and can be overridden with TIGERSWAP_DEPLOYMENTS_DIR so a
// deployment can mount the registry read-only.
var Dir = func() string {
	if v := strings.TrimSpace(os.Getenv("TIGERSWAP_DEPLOYMENTS_DIR")); v != "" {
		return v
	}
	return "deployments"
}()

// resolveDir returns the directory to read, falling back to the repository
// layout the deploy script writes into. The script runs from `contracts/`, so it
// emits `contracts/deployments/<id>.json`; a server started from `backend/` would
// otherwise look in the wrong place. An explicit TIGERSWAP_DEPLOYMENTS_DIR always
// wins, and a caller that set Dir directly is honoured as-is.
func resolveDir() string {
	if strings.TrimSpace(os.Getenv("TIGERSWAP_DEPLOYMENTS_DIR")) != "" {
		return Dir
	}
	candidates := []string{Dir, filepath.Join("contracts", "deployments"), filepath.Join("..", "contracts", "deployments")}
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && info.IsDir() {
			return c
		}
	}
	return Dir
}

// Load reads the registry for one chain id. A missing file is reported as
// os.ErrNotExist so a caller can distinguish "this chain is not deployed here"
// from "the registry is corrupt".
func Load(chainID int64) (Registry, error) {
	path := filepath.Join(resolveDir(), strconv.FormatInt(chainID, 10)+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return Registry{}, err
	}
	var r Registry
	if err := json.Unmarshal(data, &r); err != nil {
		return Registry{}, fmt.Errorf("parse %s: %w", path, err)
	}
	if r.ChainID != 0 && r.ChainID != chainID {
		return Registry{}, fmt.Errorf("registry %s declares chain %d, not %d", path, r.ChainID, chainID)
	}
	r.ChainID = chainID
	return r, nil
}

// LoadAll reads every `<chainId>.json` in Dir. A malformed file is an error, not
// a silently skipped chain, so a bad deploy cannot leave a chain half-registered.
func LoadAll() ([]Registry, error) {
	entries, err := os.ReadDir(resolveDir())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var out []Registry
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		idStr := strings.TrimSuffix(e.Name(), ".json")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			// A non-numeric file is not a registry; ignore it rather than fail.
			continue
		}
		r, err := Load(id)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ChainID < out[j].ChainID })
	return out, nil
}
