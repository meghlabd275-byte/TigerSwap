package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

// ==================== CONFIGURATION ====================

type Config struct {
	Server      ServerConfig      `json:"server"`
	Database    DatabaseConfig   `json:"database"`
	Leaderboard LeaderboardConfig `json:"leaderboard"`
}

type ServerConfig struct {
	Port        string `json:"port"`
	Environment string `json:"environment"`
}

type DatabaseConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User    string `json:"user"`
	Password string `json:"password"`
	DBName  string `json:"dbname"`
}

type LeaderboardConfig struct {
	TopN           int   `json:"topN"`
	UpdateInterval  int   `json:"updateIntervalSeconds"`
	PointsPerTrade  int64 `json:"pointsPerTrade"`
	PointsPerVolume int64 `json:"pointsPerVolume"`
}

// ==================== MODELS ====================

type Competition struct {
	ID          string            `json:"id"`
	Name       string            `json:"name"`
	Type       CompetitionType  `json:"type"`
	Status     CompetitionStatus `json:"status"`
	StartTime  time.Time       `json:"startTime"`
	EndTime    time.Time       `json:"endTime"`
	RewardPool decimal.Decimal `json:"rewardPool"`
	TopN       int             `json:"topN"`
	CreatedAt  time.Time       `json:"createdAt"`
}

type CompetitionType string

const (
	TypeTradingVolume CompetitionType = "TRADING_VOLUME"
	TypePnL         CompetitionType = "PNL"
	TypeMostTrades  CompetitionType = "MOST_TRADES"
	TypeBestROI    CompetitionType = "BEST_ROI"
)

type CompetitionStatus string

const (
	StatusUpcoming  CompetitionStatus = "UPCOMING"
	StatusActive   CompetitionStatus = "ACTIVE"
	StatusEnded   CompetitionStatus = "ENDED"
)

type LeaderboardEntry struct {
	Rank         int             `json:"rank"`
	UserID       string          `json:"userId"`
	Address     string          `json:"address"`
	Username    string          `json:"username,omitempty"`
	Score       decimal.Decimal `json:"score"`
	Volume      decimal.Decimal `json:"volume"`
	Trades      int            `json:"trades"`
	PnL         decimal.Decimal `json:"pnl"`
	ROI         decimal.Decimal `json:"roi"`
	UpdatedAt   time.Time      `json:"updatedAt"`
}

type UserStats struct {
	UserID            string          `json:"userId"`
	Address          string          `json:"address"`
	TotalVolume      decimal.Decimal `json:"totalVolume"`
	TotalTrades      int             `json:"totalTrades"`
	TotalPnL         decimal.Decimal `json:"totalPnL"`
	WinRate         decimal.Decimal `json:"winRate"`
	BestTrade       decimal.Decimal `json:"bestTrade"`
	WorstTrade      decimal.Decimal `json:"worstTrade"`
	LastTradeTime   time.Time      `json:"lastTradeTime"`
	FirstTradeTime  time.Time      `json:"firstTradeTime"`
	CompetitionPoints int64         `json:"competitionPoints"`
}

type Trade struct {
	ID          string          `json:"id"`
	UserID     string          `json:"userId"`
	Hash       string          `json:"hash"`
	Type       string          `json:"type"` // swap, addLiquidity, removeLiquidity
	FromToken  string          `json:"fromToken"`
	ToToken    string          `json:"toToken"`
	FromAmount decimal.Decimal `json:"fromAmount"`
	ToAmount  decimal.Decimal `json:"toAmount"`
	VolumeUSD  decimal.Decimal `json:"volumeUSD"`
	FeeUSD    decimal.Decimal `json:"feeUSD"`
	PnL       decimal.Decimal `json:"pnl"` // profit/loss in USD
	Status    string          `json:"status"`
	Timestamp time.Time     `json:"timestamp"`
}

type LeaderboardConfig struct {
	TopN          int   `json:"topN"`
	UpdateInterval int   `json:"updateIntervalSeconds"`
}

// ==================== DATABASE ====================

type Database interface {
	Init() error
	Close() error

	// User stats
	GetUserStats(ctx context.Context, userID string) (*UserStats, error)
	UpdateUserStats(ctx context.Context, stats *UserStats) error

	// Trades
	SaveTrade(ctx context.Context, trade *Trade) error
	GetUserTrades(ctx context.Context, userID string, limit int) ([]*Trade, error)
	GetRecentTrades(ctx context.Context, limit int) ([]*Trade, error)

	// Leaderboard entries
	GetLeaderboardEntries(ctx context.Context, competitionID string, limit int) ([]*LeaderboardEntry, error)
	CalculateLeaderboard(ctx context.Context, competitionID string) ([]*LeaderboardEntry, error)
}

type PostgresDatabase struct {
	conn interface{} // In production, use *sql.DB
}

func (db *PostgresDatabase) Init() error {
	// In production, implement actual database connection
	return nil
}

func (db *PostgresDatabase) Close() error {
	return nil
}

func (db *PostgresDatabase) GetUserStats(ctx context.Context, userID string) (*UserStats, error) {
	return nil, nil
}

func (db *PostgresDatabase) UpdateUserStats(ctx context.Context, stats *UserStats) error {
	return nil
}

func (db *PostgresDatabase) SaveTrade(ctx context.Context, trade *Trade) error {
	return nil
}

func (db *PostgresDatabase) GetUserTrades(ctx context.Context, userID string, limit int) ([]*Trade, error) {
	return nil, nil
}

func (db *PostgresDatabase) GetRecentTrades(ctx context.Context, limit int) ([]*Trade, error) {
	return nil, nil
}

func (db *PostgresDatabase) GetLeaderboardEntries(ctx context.Context, competitionID string, limit int) ([]*LeaderboardEntry, error) {
	return nil, nil
}

func (db *PostgresDatabase) CalculateLeaderboard(ctx context.Context, competitionID string) ([]*LeaderboardEntry, error) {
	return nil, nil
}

// ==================== LEADERBOARD SERVICE ====================

type LeaderboardService struct {
	db           Database
	topN         int
	updateInterval time.Duration
}

func NewLeaderboardService(db Database, topN int, updateInterval int) *LeaderboardService {
	return &LeaderboardService{
		db:           db,
		topN:         topN,
		updateInterval: time.Duration(updateInterval) * time.Second,
	}
}

func (s *LeaderboardService) CalculateLeaderboard(ctx context.Context, compType CompetitionType) ([]*LeaderboardEntry, error) {
	// Get recent trades for calculation
	trades, err := s.db.GetRecentTrades(ctx, 10000)
	if err != nil {
		return nil, err
	}

	// Aggregate by user
	userStats := make(map[string]*UserStats)
	for _, trade := range trades {
		if trade.Status != "confirmed" {
			continue
		}

		stats, ok := userStats[trade.UserID]
		if !ok {
			stats = &UserStats{
				UserID:   trade.UserID,
				Address: "", // Would be populated from user service
			}
			userStats[trade.UserID] = stats
		}

		stats.TotalVolume = stats.TotalVolume.Add(trade.VolumeUSD)
		stats.TotalTrades++
		stats.TotalPnL = stats.TotalPnL.Add(trade.PnL)

		if trade.PnL.GreaterThan(decimal.Zero) {
			if stats.BestTrade.LessThan(trade.PnL) {
				stats.BestTrade = trade.PnL
			}
		} else {
			if stats.WorstTrade.GreaterThan(trade.PnL) {
				stats.WorstTrade = trade.PnL
			}
		}

		if stats.FirstTradeTime.IsZero() {
			stats.FirstTradeTime = trade.Timestamp
		}
		stats.LastTradeTime = trade.Timestamp
	}

	// Convert to leaderboard entries
	entries := make([]*LeaderboardEntry, 0, len(userStats))
	for userID, stats := range userStats {
		entry := &LeaderboardEntry{
			UserID:     userID,
			Address:    stats.Address,
			Volume:    stats.TotalVolume,
			Trades:    stats.TotalTrades,
			PnL:       stats.TotalPnL,
			UpdatedAt:  time.Now(),
		}

		// Calculate score based on competition type
		switch compType {
		case TypeTradingVolume:
			entry.Score = stats.TotalVolume
		case TypeMostTrades:
			entry.Score = decimal.NewFromInt(int64(stats.TotalTrades))
		case TypePnL:
			entry.Score = stats.TotalPnL
		case TypeBestROI:
			if stats.TotalVolume.GreaterThan(decimal.Zero) {
				entry.ROI = stats.TotalPnL.Div(stats.TotalVolume).Mul(decimal.NewFromInt(100))
				entry.Score = entry.ROI
			}
		}

		entries = append(entries, entry)
	}

	// Sort by score descending
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Score.GreaterThan(entries[j].Score)
	})

	// Assign ranks
	for i, entry := range entries {
		entry.Rank = i + 1
	}

	// Limit to top N
	if len(entries) > s.topN {
		entries = entries[:s.topN]
	}

	return entries, nil
}

func (s *LeaderboardService) GetLeaderboard(ctx context.Context, competitionID string) ([]*LeaderboardEntry, error) {
	if competitionID == "" {
		// Return default leaderboard
		return s.db.GetLeaderboardEntries(ctx, "default", s.topN)
	}
	return s.db.GetLeaderboardEntries(ctx, competitionID, s.topN)
}

func (s *LeaderboardService) GetUserRank(ctx context.Context, userID, competitionID string) (int, error) {
	entries, err := s.db.GetLeaderboardEntries(ctx, competitionID, 0) // Get all
	if err != nil {
		return 0, err
	}

	for _, entry := range entries {
		if entry.UserID == userID {
			return entry.Rank, nil
		}
	}

	return 0, nil // Not found
}

// ==================== HTTP HANDLERS ====================

type Handler struct {
	db       Database
	service *LeaderboardService
}

func NewHandler(db Database, service *LeaderboardService) *Handler {
	return &Handler{
		db:       db,
		service: service,
	}
}

func (h *Handler) RegisterRoutes(r *gin.Engine) {
	r.GET("/health", h.handleHealth)
	r.GET("/leaderboard", h.handleGetLeaderboard)
	r.GET("/leaderboard/:competitionId", h.handleGetLeaderboardByCompetition)
	r.GET("/leaderboard/user/:userId", h.handleGetUserRank)
	r.GET("/user/:userId/stats", h.handleGetUserStats)
	r.GET("/user/:userId/trades", h.handleGetUserTrades)
	r.POST("/trade", h.handleRecordTrade)
	r.GET("/competitions", h.handleListCompetitions)
	r.GET("/competitions/:id", h.handleGetCompetition)
}

// ==================== HANDLER IMPLEMENTATIONS ====================

func (h *Handler) handleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "ok",
		"timestamp": time.Now().UTC(),
	})
}

func (h *Handler) handleGetLeaderboard(c *gin.Context) {
	entries, err := h.service.GetLeaderboard(c.Request.Context(), "")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"leaderboard": entries,
		"count":      len(entries),
	})
}

func (h *Handler) handleGetLeaderboardByCompetition(c *gin.Context) {
	competitionID := c.Param("competitionId")

	entries, err := h.service.GetLeaderboard(c.Request.Context(), competitionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"competitionId": competitionID,
		"leaderboard":  entries,
		"count":      len(entries),
	})
}

func (h *Handler) handleGetUserRank(c *gin.Context) {
	userID := c.Param("userId")
	competitionID := c.DefaultQuery("competitionId", "")

	rank, err := h.service.GetUserRank(c.Request.Context(), userID, competitionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if rank == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found in leaderboard"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"userId":      userID,
		"competition": competitionID,
		"rank":       rank,
	})
}

func (h *Handler) handleGetUserStats(c *gin.Context) {
	userID := c.Param("userId")

	stats, err := h.db.GetUserStats(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if stats == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	c.JSON(http.StatusOK, stats)
}

func (h *Handler) handleGetUserTrades(c *gin.Context) {
	userID := c.Param("userId")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))

	trades, err := h.db.GetUserTrades(c.Request.Context(), userID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"trades": trades,
		"count": len(trades),
	})
}

func (h *Handler) handleRecordTrade(c *gin.Context) {
	var trade Trade
	if err := c.ShouldBindJSON(&trade); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	trade.ID = fmt.Sprintf("trade-%d", time.Now().UnixNano())
	trade.Timestamp = time.Now()

	if err := h.db.SaveTrade(c.Request.Context(), &trade); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"trade": trade,
	})
}

func (h *Handler) handleListCompetitions(c *gin.Context) {
	// In production, fetch from database
	competitions := []Competition{
		{
			ID:          "default",
			Name:        "All-Time Trading Volume",
			Type:        TypeTradingVolume,
			Status:     StatusActive,
			TopN:       100,
		},
	}

	c.JSON(http.StatusOK, gin.H{
		"competitions": competitions,
	})
}

func (h *Handler) handleGetCompetition(c *gin.Context) {
	id := c.Param("id")

	// In production, fetch from database
	competition := Competition{
		ID:     id,
		Name:   "Trading Competition #" + id,
		Type:   TypeTradingVolume,
		Status: StatusActive,
		TopN:  100,
	}

	c.JSON(http.StatusOK, competition)
}

// ==================== MAIN ====================

func main() {
	// Load configuration
	config := Config{
		Server: ServerConfig{
			Port:        "8081",
			Environment: "development",
		},
		Leaderboard: LeaderboardConfig{
			TopN:          100,
			UpdateInterval: 60,
		},
	}

	// Initialize database
	db := &PostgresDatabase{}
	if err := db.Init(); err != nil {
		log.Printf("Warning: Database not connected: %v", err)
	}
	defer db.Close()

	// Initialize leaderboard service
	service := NewLeaderboardService(db, config.Leaderboard.TopN, config.Leaderboard.UpdateInterval)

	// Initialize handler
	handler := NewHandler(db, service)

	// Setup Gin
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(gin.Logger())

	// Register routes
	handler.RegisterRoutes(r)

	// CORS middleware
	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusOK)
			return
		}

		c.Next()
	})

	// Start server
	addr := ":" + config.Server.Port
	log.Printf("Leaderboard service starting on %s", addr)

	go func() {
		if err := r.Run(addr); err != nil {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Periodic leaderboard calculation
	go func() {
		ticker := time.NewTicker(time.Duration(config.Leaderboard.UpdateInterval) * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			ctx := context.Background()

			// Calculate default leaderboard
			entries, err := service.CalculateLeaderboard(ctx, TypeTradingVolume)
			if err != nil {
				log.Printf("Leaderboard calculation error: %v", err)
				continue
			}

			log.Printf("Updated leaderboard with %d entries", len(entries))
		}
	}()

	// Wait for shutdown signal
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	log.Println("Shutting down...")
}