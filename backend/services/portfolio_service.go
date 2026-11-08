package services

import (
	"context"
	"math"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"tigerswap/backend/models"
)

// PortfolioService answers the account portfolio and transaction history for an
// authenticated user.
//
// Every figure is derived from rows TigerSwap itself wrote: ledger balances in
// `account_balances`, confirmed ledger transactions, and the derivatives and
// margin books. Holdings are valued at the platform oracle price; a token with
// no price is reported with a null USD value rather than a guessed one, and the
// total counts only what it could actually value.
//
// The service never trusts a client-supplied identity: the caller's user id is
// read from the Gin context by the handler, never from the request body.
type PortfolioService struct {
	db    *gorm.DB
	price priceSource
	// positions is the live derivatives book. It is optional; when set, the
	// unrealized PnL of an open position is read from the engine that re-marks
	// it against the oracle every few seconds, rather than from the row the
	// engine last wrote, which is stale until the position closes.
	positions livePositionSource
}

// livePositionSource exposes the engine's open positions with their current
// mark price and unrealized PnL.
type livePositionSource interface {
	GetUserPositions(userAddress string) []*PerpetualPosition
}

// NewPortfolioService builds the portfolio service. The price source is
// optional: without it, a holding's USD value is reported as null and the total
// reflects only unpriced holdings as zero, never as an invented number.
func NewPortfolioService(db *gorm.DB, price priceSource) *PortfolioService {
	return &PortfolioService{db: db, price: price}
}

// SetPositionSource attaches the live derivatives book. Without it the service
// still reports realized PnL from the database and counts open positions, but
// cannot value their unrealized PnL.
func (s *PortfolioService) SetPositionSource(source livePositionSource) {
	s.positions = source
}

// portfolioAssetDTO is one holding. `balance` is the exact ledger amount in the
// token's smallest unit as a decimal string; `valueUSD` is null when no price
// source could value the token.
type portfolioAssetDTO struct {
	ChainID      int64    `json:"chainId"`
	TokenAddress string   `json:"tokenAddress"`
	Symbol       string   `json:"symbol"`
	Decimals     int      `json:"decimals"`
	Balance      string   `json:"balance"`
	ValueUSD     *float64 `json:"valueUSD"`
}

// portfolioSummaryDTO is the `/user/portfolio` envelope. The PnL block is
// computed from the derivatives book; a book with no closed positions reports
// zero rather than a fabricated figure.
type portfolioSummaryDTO struct {
	TotalValueUSD    float64             `json:"totalValueUSD"`
	Assets           []portfolioAssetDTO `json:"assets"`
	RealizedPnLUSD   float64             `json:"realizedPnlUsd"`
	UnrealizedPnLUSD float64             `json:"unrealizedPnlUsd"`
	OpenPositions    int                 `json:"openPositions"`
	UpdatedAt        time.Time           `json:"updatedAt"`
}

// userTransactionDTO is the `/user/transactions` element. The frontend's
// `UserTransaction` interface is camelCase and the list validator requires a
// bare array, so this DTO exists to keep the wire format exactly that.
type userTransactionDTO struct {
	Hash        string    `json:"hash"`
	ChainID     int64     `json:"chainId"`
	Type        string    `json:"type"`
	Status      string    `json:"status"`
	Amount      string    `json:"amount"`
	TokenSymbol string    `json:"tokenSymbol"`
	CreatedAt   time.Time `json:"createdAt"`
}

// tokenValueUSD converts a raw balance in the token's smallest unit to a USD
// value using the oracle price. It returns nil when no price is available, so a
// caller can distinguish "worth zero" from "cannot value".
func (s *PortfolioService) tokenValueUSD(ctx context.Context, symbol string, decimals int, raw string) *float64 {
	if s.price == nil {
		return nil
	}
	// A balance of exactly zero is worth zero regardless of price; valuing it
	// would still be correct, but short-circuiting avoids a pointless oracle
	// round trip on an account with many empty rows.
	amount, ok := new(big.Rat).SetString(strings.TrimSpace(raw))
	if !ok {
		return nil
	}
	if amount.Sign() == 0 {
		zero := 0.0
		return &zero
	}
	data, err := s.price.GetRealPrice(ctx, strings.ToUpper(strings.TrimSpace(symbol)))
	if err != nil || data == nil || !(data.USD > 0) || math.IsNaN(data.USD) || math.IsInf(data.USD, 0) {
		return nil
	}
	// raw / 10^decimals * price, kept in exact rational arithmetic until the
	// final conversion so a large balance does not lose precision.
	scale := new(big.Rat).SetInt(pow10(decimals))
	human := new(big.Rat).Quo(amount, scale)
	priceRat := new(big.Rat).SetFloat64(data.USD)
	if priceRat == nil {
		return nil
	}
	value, err := rationalToFloat(new(big.Rat).Mul(human, priceRat))
	if err != nil {
		return nil
	}
	return &value
}

// Holdings builds the account's holdings from the ledger, valued at the oracle
// price. A zero balance is omitted so the list only shows what the account
// actually holds.
func (s *PortfolioService) Holdings(ctx context.Context, userID string) ([]portfolioAssetDTO, error) {
	var rows []models.AccountBalance
	if err := s.db.Where("user_id = ?", userID).
		Order("chain_id asc, symbol asc").
		Find(&rows).Error; err != nil {
		return nil, err
	}

	assets := make([]portfolioAssetDTO, 0, len(rows))
	for i := range rows {
		row := rows[i]
		total, ok := new(big.Int).SetString(row.Available, 10)
		if !ok {
			total = big.NewInt(0)
		}
		if locked, ok := new(big.Int).SetString(row.Locked, 10); ok {
			total = total.Add(total, locked)
		}
		if total.Sign() == 0 {
			continue
		}
		assets = append(assets, portfolioAssetDTO{
			ChainID:      row.ChainID,
			TokenAddress: row.TokenAddress,
			Symbol:       row.Symbol,
			Decimals:     row.Decimals,
			Balance:      total.String(),
			ValueUSD:     s.tokenValueUSD(ctx, row.Symbol, row.Decimals, total.String()),
		})
	}
	return assets, nil
}

// pnlSummary totals realized and unrealized PnL. Realized PnL is read from
// closed positions in the database. Unrealized PnL and the open-position count
// come from the live derivatives engine when one is attached, because the
// engine re-marks every open position against the oracle on an interval while
// the persisted row keeps the value it was opened with. Without an engine the
// summary falls back to the stored rows so the panel is still populated.
//
// A position whose PnL column does not parse is skipped rather than treated as
// zero, so a corrupt row cannot silently deflate the total.
func (s *PortfolioService) pnlSummary(userID string) (realized, unrealized float64, open int, err error) {
	var positions []models.PerpetualPosition
	if err = s.db.Where("user_id = ?", userID).Find(&positions).Error; err != nil {
		return 0, 0, 0, err
	}

	live := map[string]bool{}
	if s.positions != nil {
		for _, position := range s.positions.GetUserPositions(userID) {
			if position == nil {
				continue
			}
			live[position.ID] = true
			open++
			if value, ok := parseFloatStrict(position.UnrealizedPNL); ok {
				unrealized += value
			}
		}
	}

	for i := range positions {
		position := positions[i]
		switch position.Status {
		case "open", "liquidatable":
			if live[position.ID.String()] {
				// Already valued from the live engine above.
				continue
			}
			open++
			if value, ok := parseFloatStrict(position.PNL); ok {
				unrealized += value
			}
		case "closed", "liquidated":
			if value, ok := parseFloatStrict(position.PNL); ok {
				realized += value
			}
		}
	}
	return realized, unrealized, open, nil
}

// Summary is the full portfolio: holdings plus the PnL block.
func (s *PortfolioService) Summary(ctx context.Context, userID string) (*portfolioSummaryDTO, error) {
	assets, err := s.Holdings(ctx, userID)
	if err != nil {
		return nil, err
	}
	var total float64
	for i := range assets {
		if assets[i].ValueUSD != nil {
			total += *assets[i].ValueUSD
		}
	}
	realized, unrealized, open, err := s.pnlSummary(userID)
	if err != nil {
		return nil, err
	}
	return &portfolioSummaryDTO{
		TotalValueUSD:    total,
		Assets:           assets,
		RealizedPnLUSD:   realized,
		UnrealizedPnLUSD: unrealized,
		OpenPositions:    open,
		UpdatedAt:        time.Now().UTC(),
	}, nil
}

// Transactions lists the caller's transactions as camelCase DTOs, newest first.
func (s *PortfolioService) Transactions(userID string, limit int) ([]userTransactionDTO, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	var txs []models.Transaction
	if err := s.db.Where("user_id = ?", userID).
		Order("timestamp desc").Limit(limit).Find(&txs).Error; err != nil {
		return nil, err
	}
	out := make([]userTransactionDTO, 0, len(txs))
	for i := range txs {
		tx := txs[i]
		out = append(out, userTransactionDTO{
			Hash:        tx.Hash,
			ChainID:     tx.ChainID,
			Type:        tx.Type,
			Status:      tx.Status,
			Amount:      tx.ToAmount,
			TokenSymbol: transactionSymbol(tx),
			CreatedAt:   tx.Timestamp,
		})
	}
	return out, nil
}

// transactionSymbol returns the symbol the amount column refers to. The amount
// is the token the account received, so the symbol comes from the transaction's
// destination token, falling back to the pair's base when it is a plain
// address.
func transactionSymbol(tx models.Transaction) string {
	symbol := strings.TrimSpace(tx.ToToken)
	if symbol != "" && !strings.HasPrefix(symbol, "0x") {
		return symbol
	}
	if tx.Pair != "" {
		if idx := strings.Index(tx.Pair, "/"); idx > 0 {
			return tx.Pair[:idx]
		}
		return tx.Pair
	}
	if symbol != "" {
		return symbol
	}
	return "TOKEN"
}

// GetPortfolioHandler serves GET /user/portfolio.
func (s *PortfolioService) GetPortfolioHandler(c *gin.Context) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}
	summary, err := s.Summary(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load portfolio"})
		return
	}
	c.JSON(http.StatusOK, summary)
}

// GetTransactionsHandler serves GET /user/transactions. The frontend's list
// validator requires a bare JSON array, so the rows are returned unwrapped.
func (s *PortfolioService) GetTransactionsHandler(c *gin.Context) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}
	txs, err := s.Transactions(userID, 200)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load transactions"})
		return
	}
	c.JSON(http.StatusOK, txs)
}

// parseFloatStrict parses a decimal string into a finite float64, reporting
// whether it succeeded. A blank string is treated as a valid zero so a position
// that has not yet been priced does not distort a total.
func parseFloatStrict(raw string) (float64, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, true
	}
	value, err := strconv.ParseFloat(trimmed, 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, false
	}
	return value, true
}
