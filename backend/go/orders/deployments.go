package orders

import (
	"errors"
	"os"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common"

	"tigerswap/backend/config/deployments"
)

// LoadDeployments resolves the on-chain order-contract deployment set. The
// per-chain registry written by `contracts/script/Deploy.s.sol` is the source of
// truth: every chain with a registry file that names an orders (or TWAMM)
// contract is registered, so a freshly deployed chain is dialled without
// hand-copying an address. An environment variable overrides a registry value.
//
// Only TigerSwap's own nodes are dialled; no third-party RPC provider is used. A
// chain with no orders address is skipped rather than returning a
// half-configured deployment.
func LoadDeployments() []Deployment {
	if regs, err := deployments.LoadAll(); err == nil && len(regs) > 0 {
		return deploymentsFromRegistry(regs)
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return deploymentsFromEnv()
}

func deploymentsFromRegistry(regs []deployments.Registry) []Deployment {
	var out []Deployment
	for _, reg := range regs {
		id := reg.ChainID
		suffix := strconv.FormatInt(id, 10)

		ordersAddr, err := reg.Address("orders")
		if err != nil {
			continue
		}
		if env := strings.TrimSpace(os.Getenv("TIGERSWAP_ORDERS_" + suffix)); env != "" {
			ordersAddr = common.HexToAddress(env)
		}
		twammAddr, _ := reg.Address("twamm")
		if env := strings.TrimSpace(os.Getenv("TIGERSWAP_TWAMM_" + suffix)); env != "" {
			twammAddr = common.HexToAddress(env)
		}
		if ordersAddr == (common.Address{}) && twammAddr == (common.Address{}) {
			continue
		}
		out = append(out, Deployment{
			ChainID: id,
			Orders:  ordersAddr,
			TWAMM:   twammAddr,
			RPCURL:  rpcForChain(suffix),
		})
	}
	return out
}

// deploymentsFromEnv reads the environment-only configuration for a chain whose
// registry is not present.
func deploymentsFromEnv() []Deployment {
	raw := strings.TrimSpace(os.Getenv("TIGERSWAP_ORDERS_CHAINS"))
	if raw == "" {
		return nil
	}
	var out []Deployment
	for _, part := range strings.Split(raw, ",") {
		id, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64)
		if err != nil {
			continue
		}
		suffix := strconv.FormatInt(id, 10)
		ordersAddr := common.HexToAddress(strings.TrimSpace(os.Getenv("TIGERSWAP_ORDERS_" + suffix)))
		twammAddr := common.HexToAddress(strings.TrimSpace(os.Getenv("TIGERSWAP_TWAMM_" + suffix)))
		if ordersAddr == (common.Address{}) && twammAddr == (common.Address{}) {
			continue
		}
		out = append(out, Deployment{
			ChainID: id,
			Orders:  ordersAddr,
			TWAMM:   twammAddr,
			RPCURL:  rpcForChain(suffix),
		})
	}
	return out
}

// rpcForChain resolves a chain's node endpoint. TIGERSWAP_RPC_<id> takes
// precedence over TIGERSWAP_RPC_URL_<id>, matching the rest of the codebase.
func rpcForChain(suffix string) string {
	if rpc := strings.TrimSpace(os.Getenv("TIGERSWAP_RPC_" + suffix)); rpc != "" {
		return rpc
	}
	return strings.TrimSpace(os.Getenv("TIGERSWAP_RPC_URL_" + suffix))
}

// LoadSigner reads the executor signer key from TIGERSWAP_ORDERS_EXECUTOR_KEY,
// then falls back to the perps keeper and the shared signer. The executor needs
// a key only to submit fills; read paths work without one.
func LoadSigner() (string, bool) {
	for _, key := range []string{"TIGERSWAP_ORDERS_EXECUTOR_KEY", "TIGERSWAP_PERPS_KEEPER_KEY", "TIGERSWAP_SIGNER_KEY"} {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			return v, true
		}
	}
	return "", false
}
