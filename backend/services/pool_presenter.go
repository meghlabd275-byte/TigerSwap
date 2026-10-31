package services

import (
	"strings"

	"tigerswap/backend/models"
)

// Pool and token presentation helpers for the public market-data endpoints.
//
// Reserves and amounts are stored as integer base units at each token's own
// decimals. Rendering them as human-readable figures therefore requires the
// decimals of the specific token, which is looked up from the tokens table -
// assuming 18 would misreport a 6-decimal token like USDC or USDT by 1e12.

// tokenDecimals returns a lowercase-address -> decimals map covering every token
// referenced by the given transactions. Unknown tokens default to 18 decimals.
func (s *SwapService) tokenDecimals(txs []models.Transaction) map[string]int {
	addresses := make(map[string]struct{}, len(txs)*2)
	for _, tx := range txs {
		if tx.FromToken != "" {
			addresses[strings.ToLower(tx.FromToken)] = struct{}{}
		}
		if tx.ToToken != "" {
			addresses[strings.ToLower(tx.ToToken)] = struct{}{}
		}
	}

	out := make(map[string]int, len(addresses))
	if len(addresses) == 0 {
		return out
	}

	list := make([]string, 0, len(addresses))
	for addr := range addresses {
		list = append(list, addr)
	}

	var tokens []models.Token
	s.db.Where("LOWER(address) IN ?", list).Find(&tokens)
	for _, token := range tokens {
		out[strings.ToLower(token.Address)] = token.Decimals
	}
	return out
}

// newPoolDTOs maps stored pools to the frontend's pool shape, resolving symbols
// and decimals from the tokens table.
func (s *SwapService) newPoolDTOs(pools []models.Pool) []poolDTO {
	out := make([]poolDTO, 0, len(pools))
	for _, pool := range pools {
		out = append(out, s.newPoolDTO(pool))
	}
	return out
}

func (s *SwapService) newPoolDTO(pool models.Pool) poolDTO {
	symbol0, decimals0 := s.lookupToken(pool.ChainID, pool.TokenAAddress)
	symbol1, decimals1 := s.lookupToken(pool.ChainID, pool.TokenBAddress)

	return poolDTO{
		PairAddress: pool.PoolAddress,
		Token0:      pool.TokenAAddress,
		Token1:      pool.TokenBAddress,
		Symbol0:     symbol0,
		Symbol1:     symbol1,
		Reserve0:    scaleToFloat(pool.ReserveA, decimals0),
		Reserve1:    scaleToFloat(pool.ReserveB, decimals1),
		FeeBps:      pool.Fee,
	}
}

// lookupToken resolves a token's symbol and decimals from the tokens table for a
// given chain. The address is matched case-insensitively because checksummed and
// lowercase forms both occur. Unknown tokens report the address's first
// characters as the symbol so the UI never shows an empty label.
func (s *SwapService) lookupToken(chainID int64, address string) (string, int) {
	if address == "" {
		return "UNKNOWN", 18
	}
	var token models.Token
	err := s.db.Where("chain_id = ? AND LOWER(address) = ?", chainID, strings.ToLower(address)).
		First(&token).Error
	if err != nil {
		return shortAddress(address), 18
	}
	symbol := token.Symbol
	if strings.TrimSpace(symbol) == "" {
		symbol = shortAddress(address)
	}
	return symbol, token.Decimals
}

// shortAddress renders a compact, symbol-safe label for an unknown token. The
// frontend's symbol validator only accepts [A-Za-z0-9._-], so the 0x prefix is
// dropped.
func shortAddress(address string) string {
	trimmed := strings.TrimPrefix(address, "0x")
	trimmed = strings.TrimPrefix(trimmed, "0X")
	if len(trimmed) > 8 {
		trimmed = trimmed[:8]
	}
	if trimmed == "" {
		return "UNKNOWN"
	}
	return trimmed
}
