package perps

import (
	"os"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common"
)

// LoadDeployments reads the on-chain perpetuals deployment set from the
// environment. TIGERSWAP_PERPS_CHAINS is a comma-separated chain list; each
// chain uses TIGERSWAP_PERPS_<id> for the contract address,
// TIGERSWAP_PERPS_ORACLE_<id> for the oracle address (optional) and
// TIGERSWAP_RPC_<id> (or TIGERSWAP_RPC_URL_<id>) for the node. Only TigerSwap's
// own nodes are dialled; no third-party RPC provider is used. A chain with no
// perps address is skipped rather than returning a half-configured deployment.
func LoadDeployments() []Deployment {
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
		rpc := strings.TrimSpace(os.Getenv("TIGERSWAP_RPC_" + suffix))
		if rpc == "" {
			rpc = strings.TrimSpace(os.Getenv("TIGERSWAP_RPC_URL_" + suffix))
		}
		deployments = append(deployments, Deployment{
			ChainID:    id,
			Perpetuals: addr,
			Oracle:     common.HexToAddress(strings.TrimSpace(os.Getenv("TIGERSWAP_PERPS_ORACLE_" + suffix))),
			RPCURL:     rpc,
		})
	}
	return deployments
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
