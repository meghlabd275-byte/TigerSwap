package perps

import (
	"errors"
	"os"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common"

	"tigerswap/backend/config/deployments"
)

// LoadDeployments resolves the on-chain perpetuals deployment set. The per-chain
// registry written by `contracts/script/Deploy.s.sol` is the source of truth:
// every chain with a registry file that names a perps contract is registered, so
// a freshly deployed chain is dialled without hand-copying an address. An
// environment variable overrides a registry value, and `TIGERSWAP_PERPS_CHAINS`
// remains supported for a chain whose registry is not present.
//
// Only TigerSwap's own nodes are dialled; no third-party RPC provider is used. A
// chain with no perps address is skipped rather than returning a
// half-configured deployment.
func LoadDeployments() []Deployment {
	if regs, err := deployments.LoadAll(); err == nil && len(regs) > 0 {
		return deploymentsFromRegistry(regs)
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		// A corrupt registry must not silently fall back to an empty set.
		return nil
	}
	return deploymentsFromEnv()
}

// deploymentsFromRegistry builds a deployment per registry entry that names a
// perps contract, letting an environment variable override any address.
func deploymentsFromRegistry(regs []deployments.Registry) []Deployment {
	var out []Deployment
	for _, reg := range regs {
		id := reg.ChainID
		suffix := strconv.FormatInt(id, 10)

		perpsAddr, err := reg.Address("perps")
		if err != nil {
			continue
		}
		if env := strings.TrimSpace(os.Getenv("TIGERSWAP_PERPS_" + suffix)); env != "" {
			perpsAddr = common.HexToAddress(env)
		}
		if perpsAddr == (common.Address{}) {
			continue
		}
		oracleAddr, _ := reg.Address("oracle")
		if env := strings.TrimSpace(os.Getenv("TIGERSWAP_PERPS_ORACLE_" + suffix)); env != "" {
			oracleAddr = common.HexToAddress(env)
		}
		out = append(out, Deployment{
			ChainID:    id,
			Perpetuals: perpsAddr,
			Oracle:     oracleAddr,
			RPCURL:     rpcForChain(suffix),
		})
	}
	return out
}

// deploymentsFromEnv reads the legacy environment-only configuration.
func deploymentsFromEnv() []Deployment {
	raw := strings.TrimSpace(os.Getenv("TIGERSWAP_PERPS_CHAINS"))
	if raw == "" {
		return nil
	}
	var deployments []Deployment
	for _, part := range strings.Split(raw, ",") {
		id, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64)
		if err != nil {
			continue
		}
		suffix := strconv.FormatInt(id, 10)
		addr := common.HexToAddress(strings.TrimSpace(os.Getenv("TIGERSWAP_PERPS_" + suffix)))
		if addr == (common.Address{}) {
			continue
		}
		deployments = append(deployments, Deployment{
			ChainID:    id,
			Perpetuals: addr,
			Oracle:     common.HexToAddress(strings.TrimSpace(os.Getenv("TIGERSWAP_PERPS_ORACLE_" + suffix))),
			RPCURL:     rpcForChain(suffix),
		})
	}
	return deployments
}

// rpcForChain resolves a chain's node endpoint. TIGERSWAP_RPC_<id> takes
// precedence over TIGERSWAP_RPC_URL_<id>, matching the rest of the codebase.
func rpcForChain(suffix string) string {
	if rpc := strings.TrimSpace(os.Getenv("TIGERSWAP_RPC_" + suffix)); rpc != "" {
		return rpc
	}
	return strings.TrimSpace(os.Getenv("TIGERSWAP_RPC_URL_" + suffix))
}

// LoadSigner reads the keeper signer key from TIGERSWAP_PERPS_KEEPER_KEY, then
// falls back to TIGERSWAP_SIGNER_KEY. The keeper needs a key only to submit
// liquidations and oracle updates; read paths work without one.
func LoadSigner() (string, bool) {
	for _, key := range []string{"TIGERSWAP_PERPS_KEEPER_KEY", "TIGERSWAP_SIGNER_KEY"} {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			return v, true
		}
	}
	return "", false
}

// LoadOwnerKey reads the contract-owner key from TIGERSWAP_PERPS_OWNER_KEY. It
// signs owner-only calls (createMarket); without it those calls are skipped
// rather than attempted with the keeper key, which the contract would reject.
func LoadOwnerKey() (string, bool) {
	if v := strings.TrimSpace(os.Getenv("TIGERSWAP_PERPS_OWNER_KEY")); v != "" {
		return v, true
	}
	return "", false
}
