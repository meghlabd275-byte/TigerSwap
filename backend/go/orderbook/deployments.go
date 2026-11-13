package orderbook

import (
	"errors"
	"os"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common"

	"tigerswap/backend/config/deployments"
)

// LoadDeployments resolves the on-chain order book deployment set. The per-chain
// registry written by `contracts/script/Deploy.s.sol` is the source of truth:
// every chain with a registry file that names an order book is registered, so a
// freshly deployed chain is dialled without hand-copying an address. An
// environment variable overrides a registry value.
//
// Only TigerSwap's own nodes are dialled; no third-party RPC provider is used.
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

		addr, err := reg.Address("perpOrderBook")
		if err != nil {
			continue
		}
		if env := strings.TrimSpace(os.Getenv("TIGERSWAP_PERP_ORDER_BOOK_" + suffix)); env != "" {
			addr = common.HexToAddress(env)
		}
		if addr == (common.Address{}) {
			continue
		}
		oracleAddr, _ := reg.Address("oracle")
		if env := strings.TrimSpace(os.Getenv("TIGERSWAP_ORDERBOOK_ORACLE_" + suffix)); env != "" {
			oracleAddr = common.HexToAddress(env)
		}
		out = append(out, Deployment{
			ChainID:   id,
			OrderBook: addr,
			Oracle:    oracleAddr,
			RPCURL:    rpcForChain(suffix),
		})
	}
	return out
}

func deploymentsFromEnv() []Deployment {
	raw := strings.TrimSpace(os.Getenv("TIGERSWAP_ORDERBOOK_CHAINS"))
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
		addr := common.HexToAddress(strings.TrimSpace(os.Getenv("TIGERSWAP_PERP_ORDER_BOOK_" + suffix)))
		if addr == (common.Address{}) {
			continue
		}
		deployments = append(deployments, Deployment{
			ChainID:   id,
			OrderBook: addr,
			Oracle:    common.HexToAddress(strings.TrimSpace(os.Getenv("TIGERSWAP_ORDERBOOK_ORACLE_" + suffix))),
			RPCURL:    rpcForChain(suffix),
		})
	}
	return deployments
}

func rpcForChain(suffix string) string {
	if rpc := strings.TrimSpace(os.Getenv("TIGERSWAP_RPC_" + suffix)); rpc != "" {
		return rpc
	}
	return strings.TrimSpace(os.Getenv("TIGERSWAP_RPC_URL_" + suffix))
}

// LoadSigner reads the keeper signer key from TIGERSWAP_ORDERBOOK_KEEPER_KEY,
// then falls back to TIGERSWAP_SIGNER_KEY.
func LoadSigner() (string, bool) {
	for _, key := range []string{"TIGERSWAP_ORDERBOOK_KEEPER_KEY", "TIGERSWAP_SIGNER_KEY"} {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			return v, true
		}
	}
	return "", false
}

// LoadOwnerKey reads the contract-owner key from TIGERSWAP_ORDERBOOK_OWNER_KEY.
func LoadOwnerKey() (string, bool) {
	if v := strings.TrimSpace(os.Getenv("TIGERSWAP_ORDERBOOK_OWNER_KEY")); v != "" {
		return v, true
	}
	return "", false
}
