package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"tigerswap/backend/models"
)

// BotsService manages the self-built trading bots. It owns the persisted
// configuration and the audit trail of the orders each bot decided to place.
// The strategy arithmetic itself lives in the Rust `tigerswap-bots` crate; this
// service is the control plane, not a second implementation.
type BotsService struct {
	db *gorm.DB
}

func NewBotsService(db *gorm.DB) *BotsService {
	return &BotsService{db: db}
}

// strategyConfigKeys lists the configuration fields each strategy accepts. A
// key outside its strategy's set is rejected, so a typo cannot be silently
// ignored by the engine.
var strategyConfigKeys = map[models.BotStrategy]map[string]string{
	models.StrategyMarketMaker: {
		"spread_bps":              "int",
		"target_depth":            "decimal",
		"rebalance_threshold_bps": "int",
		"min_spread_bps":          "int",
		"max_spread_bps":          "int",
		"min_book_depth":          "decimal",
	},
	models.StrategyGrid: {
		"lower_price":  "decimal",
		"upper_price":  "decimal",
		"levels":       "int",
		"order_size":   "decimal",
		"stop_out_bps": "int",
	},
	models.StrategyDCA: {
		"amount_per_interval": "decimal",
		"interval_ms":         "int",
		"max_price":           "decimal",
		"total_intervals":     "int",
	},
	models.StrategyTWAP: {
		"total_quantity": "decimal",
		"slices":         "int",
		"interval_ms":    "int",
		"side":           "side",
	},
	models.StrategyArbitrage: {
		"min_edge_bps":       "int",
		"max_quantity":       "decimal",
		"depth_fraction_bps": "int",
	},
	models.StrategySniper: {
		"max_price_impact_bps": "int",
		"min_liquidity":        "decimal",
		"buy_size":             "decimal",
	},
	models.StrategyRebalance: {
		"drift_threshold_bps": "int",
	},
	models.StrategyLiquidation: {
		"close_factor_bps": "int",
		"min_bonus_bps":    "int",
	},
	models.StrategyCopy: {
		"trader_address": "address",
		"scale_bps":      "int",
		"copy_perps":     "bool",
	},
}

// validateStrategyConfig checks every key against the strategy's schema and
// every value against its declared type. It returns the canonical JSON to
// store, so what the engine later reads is exactly what was validated.
func validateStrategyConfig(strategy models.BotStrategy, raw json.RawMessage) (string, error) {
	schema, ok := strategyConfigKeys[strategy]
	if !ok {
		return "", fmt.Errorf("unknown strategy %q", strategy)
	}

	var parsed map[string]json.RawMessage
	if len(raw) == 0 {
		raw = json.RawMessage("{}")
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", errors.New("config must be a JSON object")
	}

	for key := range parsed {
		if _, ok := schema[key]; !ok {
			return "", fmt.Errorf("config key %q is not valid for strategy %q", key, strategy)
		}
	}

	canonical := make(map[string]any, len(parsed))
	for key, value := range parsed {
		kind := schema[key]
		switch kind {
		case "int":
			number, err := decodeInt(value)
			if err != nil {
				return "", fmt.Errorf("config %q: %w", key, err)
			}
			canonical[key] = number
		case "bool":
			var flag bool
			if err := json.Unmarshal(value, &flag); err != nil {
				return "", fmt.Errorf("config %q must be a boolean", key)
			}
			canonical[key] = flag
		case "side":
			var side string
			if err := json.Unmarshal(value, &side); err != nil {
				return "", fmt.Errorf("config %q must be a string", key)
			}
			if side != "buy" && side != "sell" {
				return "", fmt.Errorf("config %q must be \"buy\" or \"sell\"", key)
			}
			canonical[key] = side
		case "address":
			var address string
			if err := json.Unmarshal(value, &address); err != nil {
				return "", fmt.Errorf("config %q must be a string", key)
			}
			if !isHexAddress(address) {
				return "", fmt.Errorf("config %q must be a 0x-prefixed address", key)
			}
			canonical[key] = strings.ToLower(address)
		case "decimal":
			var text string
			if err := json.Unmarshal(value, &text); err != nil {
				return "", fmt.Errorf("config %q must be a decimal string", key)
			}
			if _, err := parseDecimalString(text); err != nil {
				return "", fmt.Errorf("config %q: %w", key, err)
			}
			canonical[key] = text
		}
	}

	encoded, err := json.Marshal(canonical)
	if err != nil {
		return "", errors.New("config could not be encoded")
	}
	return string(encoded), nil
}

// decodeInt accepts a JSON number or a numeric string and returns an int64.
func decodeInt(raw json.RawMessage) (int64, error) {
	var number int64
	if err := json.Unmarshal(raw, &number); err == nil {
		return number, nil
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		parsed, err := strconv.ParseInt(strings.TrimSpace(text), 10, 64)
		if err != nil {
			return 0, errors.New("must be an integer")
		}
		return parsed, nil
	}
	return 0, errors.New("must be an integer")
}

// isHexAddress reports whether the value is a 0x-prefixed 40-hex-character
// address. Checked without a regex so it cannot be made to backtrack.
func isHexAddress(value string) bool {
	if len(value) != 42 || value[0] != '0' || value[1] != 'x' {
		return false
	}
	for _, char := range value[2:] {
		isHex := (char >= '0' && char <= '9') ||
			(char >= 'a' && char <= 'f') ||
			(char >= 'A' && char <= 'F')
		if !isHex {
			return false
		}
	}
	return true
}

// parseDecimalString parses a non-negative base-10 decimal into a scaled
// big.Int with 18 places, rejecting anything that is not a plain decimal.
func parseDecimalString(value string) (*big.Int, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, errors.New("value is required")
	}

	whole, fraction, hasPoint := strings.Cut(trimmed, ".")
	if hasPoint && strings.Contains(fraction, ".") {
		return nil, errors.New("value has more than one decimal point")
	}
	if whole == "" && fraction == "" {
		return nil, errors.New("value has no digits")
	}
	for _, char := range whole {
		if char < '0' || char > '9' {
			return nil, errors.New("value must be a non-negative decimal")
		}
	}
	for _, char := range fraction {
		if char < '0' || char > '9' {
			return nil, errors.New("value must be a non-negative decimal")
		}
	}
	if len(fraction) > 18 {
		return nil, errors.New("value has more than 18 decimal places")
	}

	padded := fraction + strings.Repeat("0", 18-len(fraction))
	digits := whole + padded
	if digits == "" {
		return nil, errors.New("value has no digits")
	}

	scaled, ok := new(big.Int).SetString(digits, 10)
	if !ok {
		return nil, errors.New("value must be a non-negative decimal")
	}
	return scaled, nil
}

// validatePositiveDecimal parses and requires a strictly positive value.
func validatePositiveDecimal(field, value string) (*big.Int, error) {
	scaled, err := parseDecimalString(value)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", field, err)
	}
	if scaled.Sign() <= 0 {
		return nil, fmt.Errorf("%s must be positive", field)
	}
	return scaled, nil
}

// createBotRequest is the body of a bot creation call.
type createBotRequest struct {
	Name          string                 `json:"name"`
	Strategy      models.BotStrategy     `json:"strategy"`
	Symbol        string                 `json:"symbol"`
	Config        json.RawMessage        `json:"config"`
	MaxOrderSize  string                 `json:"max_order_size"`
	MaxDailyLoss  string                 `json:"max_daily_loss"`
	MaxOpenOrders int                    `json:"max_open_orders"`
}

// CreateBot validates and stores a bot configuration.
func (s *BotsService) CreateBot(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	var req createBotRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" || len(name) > 128 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name must be between 1 and 128 characters"})
		return
	}

	if _, ok := strategyConfigKeys[req.Strategy]; !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unknown strategy"})
		return
	}

	symbol := strings.TrimSpace(req.Symbol)
	if err := validateMarketSymbol(symbol); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	config, err := validateStrategyConfig(req.Strategy, req.Config)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if _, err := validatePositiveDecimal("max_order_size", req.MaxOrderSize); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// A daily loss limit of zero means "no limit", so zero is allowed here.
	if _, err := parseDecimalString(req.MaxDailyLoss); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "max_daily_loss: " + err.Error()})
		return
	}
	if req.MaxOpenOrders <= 0 || req.MaxOpenOrders > 1000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "max_open_orders must be between 1 and 1000"})
		return
	}

	bot := models.BotInstance{
		UserID:        userID,
		Name:          name,
		Strategy:      req.Strategy,
		Symbol:        symbol,
		Status:        models.BotStatusActive,
		Config:        config,
		MaxOrderSize:  req.MaxOrderSize,
		MaxDailyLoss:  req.MaxDailyLoss,
		MaxOpenOrders: req.MaxOpenOrders,
		RealisedPnL:   "0",
		TradedVolume:  "0",
	}

	if err := s.db.Create(&bot).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create bot"})
		return
	}

	c.JSON(http.StatusCreated, botResponse(bot))
}

// ListBots returns the caller's bots.
func (s *BotsService) ListBots(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	query := s.db.Where("user_id = ?", userID)
	if strategy := strings.TrimSpace(c.Query("strategy")); strategy != "" {
		query = query.Where("strategy = ?", strategy)
	}
	if status := strings.TrimSpace(c.Query("status")); status != "" {
		query = query.Where("status = ?", status)
	}

	var bots []models.BotInstance
	if err := query.Order("created_at DESC").Limit(500).Find(&bots).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not list bots"})
		return
	}

	response := make([]gin.H, 0, len(bots))
	for _, bot := range bots {
		response = append(response, botResponse(bot))
	}
	c.JSON(http.StatusOK, response)
}

// GetBot returns one bot owned by the caller.
func (s *BotsService) GetBot(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid bot id"})
		return
	}

	var bot models.BotInstance
	if err := s.db.Where("id = ? AND user_id = ?", id, userID).First(&bot).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "bot not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load bot"})
		return
	}

	c.JSON(http.StatusOK, botResponse(bot))
}

// updateBotStatusRequest is the body of a status change.
type updateBotStatusRequest struct {
	Status models.BotStatus `json:"status"`
}

// UpdateBotStatus pauses, resumes or stops a bot.
func (s *BotsService) UpdateBotStatus(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid bot id"})
		return
	}

	var req updateBotStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	switch req.Status {
	case models.BotStatusActive, models.BotStatusPaused, models.BotStatusStopped:
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "status must be active, paused or stopped"})
		return
	}

	var bot models.BotInstance
	if err := s.db.Where("id = ? AND user_id = ?", id, userID).First(&bot).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "bot not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load bot"})
		return
	}

	// A stopped bot is terminal; it must be recreated so its history stays
	// attributable to one configuration.
	if bot.Status == models.BotStatusStopped {
		c.JSON(http.StatusConflict, gin.H{"error": "a stopped bot cannot be restarted"})
		return
	}

	if err := s.db.Model(&bot).Update("status", req.Status).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not update bot"})
		return
	}

	bot.Status = req.Status
	c.JSON(http.StatusOK, botResponse(bot))
}

// DeleteBot removes a bot that is not running.
func (s *BotsService) DeleteBot(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid bot id"})
		return
	}

	var bot models.BotInstance
	if err := s.db.Where("id = ? AND user_id = ?", id, userID).First(&bot).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "bot not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load bot"})
		return
	}

	if bot.Status == models.BotStatusActive {
		c.JSON(http.StatusConflict, gin.H{"error": "pause the bot before deleting it"})
		return
	}

	// Remove the bot and its pending orders together, so a deleted bot cannot
	// leave live orders behind.
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("bot_id = ? AND status = ?", bot.ID, "pending").
			Delete(&models.BotOrder{}).Error; err != nil {
			return err
		}
		return tx.Delete(&models.BotInstance{}, bot.ID).Error
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not delete bot"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"deleted": bot.ID})
}

// ListBotOrders returns the orders a bot decided to place.
func (s *BotsService) ListBotOrders(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid bot id"})
		return
	}

	// Confirm ownership before returning any order rows.
	var count int64
	if err := s.db.Model(&models.BotInstance{}).
		Where("id = ? AND user_id = ?", id, userID).Count(&count).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load bot"})
		return
	}
	if count == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "bot not found"})
		return
	}

	limit := 100
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 || parsed > 500 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "limit must be between 1 and 500"})
			return
		}
		limit = parsed
	}

	var orders []models.BotOrder
	if err := s.db.Where("bot_id = ?", id).
		Order("created_at DESC").Limit(limit).Find(&orders).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not list orders"})
		return
	}

	c.JSON(http.StatusOK, orders)
}

// BotPerformanceSummary returns the daily realised performance of a bot.
func (s *BotsService) BotPerformanceSummary(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid bot id"})
		return
	}

	var count int64
	if err := s.db.Model(&models.BotInstance{}).
		Where("id = ? AND user_id = ?", id, userID).Count(&count).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load bot"})
		return
	}
	if count == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "bot not found"})
		return
	}

	var records []models.BotPerformance
	if err := s.db.Where("bot_id = ?", id).
		Order("day DESC").Limit(90).Find(&records).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load performance"})
		return
	}

	c.JSON(http.StatusOK, records)
}

// RecordBotOrder persists an order a bot engine produced. It is called by the
// execution path rather than by a client, so it re-checks the bot's risk limits
// against the order before storing it.
func (s *BotsService) RecordBotOrder(
	botID uint,
	symbol, side, kind, price, quantity, source string,
	reduceOnly bool,
) (*models.BotOrder, error) {
	var bot models.BotInstance
	if err := s.db.First(&bot, botID).Error; err != nil {
		return nil, fmt.Errorf("bot %d not found", botID)
	}
	if bot.Status != models.BotStatusActive {
		return nil, errors.New("bot is not active")
	}

	quantityValue, err := validatePositiveDecimal("quantity", quantity)
	if err != nil {
		return nil, err
	}
	maxOrder, err := validatePositiveDecimal("max_order_size", bot.MaxOrderSize)
	if err != nil {
		return nil, err
	}
	if quantityValue.Cmp(maxOrder) > 0 {
		return nil, errors.New("order exceeds the bot's max_order_size")
	}

	switch side {
	case "buy", "sell":
	default:
		return nil, errors.New("side must be buy or sell")
	}

	switch kind {
	case "limit", "market", "post_only":
	default:
		return nil, errors.New("order_kind must be limit, market or post_only")
	}

	if kind != "market" {
		if _, err := validatePositiveDecimal("price", price); err != nil {
			return nil, err
		}
	}

	var pending int64
	if err := s.db.Model(&models.BotOrder{}).
		Where("bot_id = ? AND status = ?", botID, "pending").Count(&pending).Error; err != nil {
		return nil, err
	}
	if int(pending) >= bot.MaxOpenOrders {
		return nil, errors.New("bot has reached its max_open_orders")
	}

	order := models.BotOrder{
		BotID:      botID,
		UserID:     bot.UserID,
		Symbol:     symbol,
		Side:       side,
		OrderKind:  kind,
		Price:      price,
		Quantity:   quantity,
		ReduceOnly: reduceOnly,
		Source:     source,
		Status:     "pending",
	}

	if err := s.db.Create(&order).Error; err != nil {
		return nil, err
	}
	return &order, nil
}

// botResponse is the API shape of a bot. The raw config is returned parsed, so
// a client never has to decode a nested JSON string.
func botResponse(bot models.BotInstance) gin.H {
	config := json.RawMessage(bot.Config)
	if len(config) == 0 {
		config = json.RawMessage("{}")
	}
	return gin.H{
		"id":              bot.ID,
		"name":            bot.Name,
		"strategy":        bot.Strategy,
		"symbol":          bot.Symbol,
		"status":          bot.Status,
		"config":          config,
		"max_order_size":  bot.MaxOrderSize,
		"max_daily_loss":  bot.MaxDailyLoss,
		"max_open_orders": bot.MaxOpenOrders,
		"realised_pnl":    bot.RealisedPnL,
		"filled_orders":   bot.FilledOrders,
		"traded_volume":   bot.TradedVolume,
		"last_run_at":     bot.LastRunAt,
		"created_at":      bot.CreatedAt,
		"updated_at":      bot.UpdatedAt,
	}
}

// currentUserID reads the authenticated user id set by the auth middleware.
func currentUserID(c *gin.Context) (uint, bool) {
	raw, exists := c.Get("user_id")
	if !exists {
		return 0, false
	}
	switch value := raw.(type) {
	case uint:
		return value, true
	case int:
		if value < 0 {
			return 0, false
		}
		return uint(value), true
	case int64:
		if value < 0 {
			return 0, false
		}
		return uint(value), true
	case uint64:
		return uint(value), true
	case float64:
		if value < 0 {
			return 0, false
		}
		return uint(value), true
	default:
		return 0, false
	}
}

// validateMarketSymbol checks a BASE/QUOTE pair. Both halves are restricted to
// the same character set the token symbols use, so a market symbol cannot carry
// markup into a response.
func validateMarketSymbol(symbol string) error {
	if symbol == "" || len(symbol) > 41 {
		return errors.New("symbol must be between 1 and 41 characters")
	}
	base, quote, hasPair := strings.Cut(symbol, "/")
	if hasPair {
		if !isTokenSymbol(base) || !isTokenSymbol(quote) {
			return errors.New("symbol must be a token or a BASE/QUOTE pair")
		}
		return nil
	}
	if !isTokenSymbol(base) {
		return errors.New("symbol must be a token or a BASE/QUOTE pair")
	}
	return nil
}

// isTokenSymbol accepts 1 to 20 characters from the symbol allow-list.
func isTokenSymbol(value string) bool {
	if len(value) == 0 || len(value) > 20 {
		return false
	}
	for _, char := range value {
		isAllowed := (char >= 'A' && char <= 'Z') ||
			(char >= 'a' && char <= 'z') ||
			(char >= '0' && char <= '9') ||
			char == '.' || char == '_' || char == '-'
		if !isAllowed {
			return false
		}
	}
	return true
}

// BotsSummary returns counts of the caller's bots by status and strategy.
func (s *BotsService) BotsSummary(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	type row struct {
		Strategy models.BotStrategy
		Status   models.BotStatus
		Count    int64
	}

	var rows []row
	if err := s.db.Model(&models.BotInstance{}).
		Select("strategy, status, COUNT(*) as count").
		Where("user_id = ?", userID).
		Group("strategy, status").
		Scan(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not summarise bots"})
		return
	}

	byStrategy := make(map[string]int64)
	byStatus := make(map[string]int64)
	for _, entry := range rows {
		byStrategy[string(entry.Strategy)] += entry.Count
		byStatus[string(entry.Status)] += entry.Count
	}

	c.JSON(http.StatusOK, gin.H{
		"by_strategy": byStrategy,
		"by_status":   byStatus,
		"total":       len(rows),
	})
}

// availableStrategies lists the strategies a client may create, with the
// configuration keys each accepts.
func (s *BotsService) AvailableStrategies(c *gin.Context) {
	response := make([]gin.H, 0, len(strategyConfigKeys))
	for strategy, schema := range strategyConfigKeys {
		keys := make(map[string]string, len(schema))
		for key, kind := range schema {
			keys[key] = kind
		}
		response = append(response, gin.H{
			"strategy":     strategy,
			"config_keys":  keys,
		})
	}
	c.JSON(http.StatusOK, response)
}

// sweepInterval is how often a caller should poll for scheduled work. Exposed
// so a client does not invent its own cadence.
const sweepInterval = 5 * time.Second
