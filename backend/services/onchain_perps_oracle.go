package services

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"

	"tigerswap/backend/go/perps"
	"tigerswap/backend/models"
)

// onChainPerpsOracleInterval is how often the writer pushes a fresh mark price
// to every configured on-chain perp market. The contract requires a mark price
// before a position can be opened (`MARKET_PRICE_NOT_SET`), so without a running
// writer a deployed market is untradable — this loop is what makes the on-chain
// derivative product usable rather than a deployed-but-empty contract.
const onChainPerpsOracleInterval = 30 * time.Second

// startOracleWriter launches the mark-price writer for every configured chain.
// It runs only when the service holds a keeper key: the contract's `updatePrice`
// is oracle-gated, so a writer with no signer would submit transactions the
// chain rejects. A chain whose deployment has no oracle address configured is
// skipped rather than dialled with an unauthenticated writer.
func (s *OnChainPerpsService) startOracleWriter(ctx context.Context) {
	if s.keeperKey == nil {
		return
	}
	for _, chainID := range s.Chains() {
		s.wg.Add(1)
		go func(id int64) {
			defer s.wg.Done()
			s.runOracleWriter(ctx, id)
		}(chainID)
	}
}

// runOracleWriter pushes prices on an interval until the context is cancelled.
func (s *OnChainPerpsService) runOracleWriter(ctx context.Context, chainID int64) {
	ticker := time.NewTicker(onChainPerpsOracleInterval)
	defer ticker.Stop()
	// Push once immediately so a freshly started server has a live mark price
	// rather than waiting a full interval.
	if err := s.pushMarkPrices(ctx, chainID); err != nil {
		log.Printf("on-chain perps oracle: chain %d: %v", chainID, err)
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.pushMarkPrices(ctx, chainID); err != nil {
				log.Printf("on-chain perps oracle: chain %d: %v", chainID, err)
			}
		}
	}
}

// pushMarkPrices reads the oracle price for every active listing that has an
// on-chain market on this chain and submits updatePrice for it. The writer is
// the only process that can set a mark price, so a market with no listing or no
// priceable symbol is skipped rather than priced at an invented number.
func (s *OnChainPerpsService) pushMarkPrices(ctx context.Context, chainID int64) error {
	if s.priceAgg == nil {
		return fmt.Errorf("oracle writer has no price source")
	}
	client, err := s.client(chainID)
	if err != nil {
		return err
	}

	// Confirm our signer is the contract's oracle before submitting anything: a
	// mismatch is a configuration error the operator must fix, not a per-market
	// failure to be logged on every tick.
	onChainOracle, err := client.PriceOracle(ctx)
	if err != nil {
		return fmt.Errorf("read priceOracle: %w", err)
	}
	signer := crypto.PubkeyToAddress(s.keeperKey.PublicKey)
	if !strings.EqualFold(onChainOracle.Hex(), signer.Hex()) {
		return fmt.Errorf("keeper signer %s is not the contract oracle %s", signer.Hex(), onChainOracle.Hex())
	}

	listings, err := s.priceableListings()
	if err != nil {
		return err
	}

	var firstErr error
	for _, listing := range listings {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		marketID := perps.MarketID(listing.symbol)
		market, err := client.Market(ctx, marketID)
		if err != nil || market.Token == (common.Address{}) {
			// No on-chain market for this listing on this chain: nothing to price.
			continue
		}
		priceData, err := s.priceAgg.GetRealPrice(ctx, listing.oracleSymbol)
		if err != nil || priceData == nil || priceData.USD <= 0 {
			if firstErr == nil {
				firstErr = fmt.Errorf("no oracle price for %s: %w", listing.oracleSymbol, err)
			}
			continue
		}
		price, err := usdFloatToWAD(priceData.USD)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		call, err := client.BuildUpdatePrice(marketID, price)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		receipt, err := client.Send(ctx, call)
		if err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("updatePrice %s: %w", listing.symbol, err)
			}
			continue
		}
		log.Printf("on-chain perps oracle: chain %d pushed %s mark %s (tx %s)", chainID, listing.symbol, price.String(), receipt.TxHash.Hex())

		// Funding. The rate is the spread between the venue's mark and the
		// oracle's index. An interest-rate component (configured per chain) is
		// standard on perp venues and is the honest source of funding for a
		// pool-priced market, which has no order-book premium of its own.
		if premiumBps := fundingPremiumBps(chainID); premiumBps > 0 {
			index := new(big.Int).Set(price)
			mark := new(big.Int).Mul(price, big.NewInt(int64(10_000+premiumBps)))
			mark.Quo(mark, big.NewInt(10_000))
			fundingCall, err := client.BuildUpdateFundingRate(marketID, mark, index)
			if err != nil {
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			if _, err := client.Send(ctx, fundingCall); err != nil {
				if firstErr == nil {
					firstErr = fmt.Errorf("updateFundingRate %s: %w", listing.symbol, err)
				}
			}
		}
	}
	return firstErr
}

// fundingPremiumBps is the configured interest-rate component of funding for a
// chain, in basis points. Zero (the default) means no funding is charged, which
// is the correct rate for a market whose mark equals its index.
func fundingPremiumBps(chainID int64) int64 {
	raw := strings.TrimSpace(os.Getenv(fmt.Sprintf("TIGERSWAP_PERPS_FUNDING_PREMIUM_BPS_%d", chainID)))
	if raw == "" {
		return 0
	}
	bps, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || bps <= 0 || bps > 1_000 {
		return 0
	}
	return bps
}

// priceableListing pairs a listing symbol with the oracle symbol its mark price
// is read from. A futures listing names its base asset; a perps listing derives
// it from the symbol, exactly as the off-chain engine does, so the on-chain mark
// and the off-chain mark come from the same feed.
type priceableListing struct {
	symbol       string
	oracleSymbol string
}

func (s *OnChainPerpsService) priceableListings() ([]priceableListing, error) {
	if s.db == nil {
		return nil, fmt.Errorf("oracle writer has no database")
	}
	var futures []models.FuturesMarket
	if err := s.db.Where("status = ?", models.ListingStatusActive).Find(&futures).Error; err != nil {
		return nil, err
	}
	var perpsMarkets []models.PerpsMarket
	if err := s.db.Where("status = ?", models.ListingStatusActive).Find(&perpsMarkets).Error; err != nil {
		return nil, err
	}

	seen := make(map[string]bool)
	var out []priceableListing
	add := func(symbol, oracleSymbol string) {
		symbol = strings.TrimSpace(symbol)
		oracleSymbol = strings.ToUpper(strings.TrimSpace(oracleSymbol))
		if symbol == "" || oracleSymbol == "" || seen[strings.ToUpper(symbol)] {
			return
		}
		seen[strings.ToUpper(symbol)] = true
		out = append(out, priceableListing{symbol: symbol, oracleSymbol: oracleSymbol})
	}
	for _, m := range futures {
		oracleSymbol := strings.ToUpper(strings.TrimSpace(m.BaseAsset))
		if oracleSymbol == "" {
			oracleSymbol = oracleBaseFromSymbol(m.Symbol)
		}
		add(m.Symbol, oracleSymbol)
	}
	for _, m := range perpsMarkets {
		if mt := strings.ToLower(strings.TrimSpace(m.MarketType)); mt != "" && mt != "oracle" {
			// Orderbook listings are matched by the CLOB, not this engine.
			continue
		}
		add(m.Symbol, oracleBaseFromSymbol(m.Symbol))
	}
	return out, nil
}

// usdFloatToWAD converts an oracle USD price to the 1e18-scaled integer the
// perps contract stores. The oracle path exposes a float, so the conversion is
// done once, here, at the boundary; every value downstream of it is an exact
// integer. The float is truncated to 18 decimals of precision, which is finer
// than any price feed TigerSwap consumes.
func usdFloatToWAD(usd float64) (*big.Int, error) {
	if usd <= 0 {
		return nil, fmt.Errorf("non-positive price %v", usd)
	}
	rat := new(big.Rat).SetFloat64(usd)
	if rat == nil {
		return nil, fmt.Errorf("price %v is not representable", usd)
	}
	scaled := new(big.Rat).Mul(rat, new(big.Rat).SetInt(pow10(18)))
	out := new(big.Int).Quo(scaled.Num(), scaled.Denom())
	if out.Sign() <= 0 {
		return nil, fmt.Errorf("price %v scales to a non-positive value", usd)
	}
	return out, nil
}
