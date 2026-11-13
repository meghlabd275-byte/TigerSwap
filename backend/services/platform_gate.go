package services

import (
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"tigerswap/backend/models"
)

// ErrTradingHalted is returned when the platform's trading switch is off.
var ErrTradingHalted = errors.New("trading is currently halted")

// ErrWithdrawalsHalted is returned when the withdrawal switch is off.
var ErrWithdrawalsHalted = errors.New("withdrawals are currently halted")

// ErrDepositsHalted is returned when the deposit switch is off.
var ErrDepositsHalted = errors.New("deposits are currently halted")

// ErrMaintenanceMode is returned when the platform is in maintenance mode.
var ErrMaintenanceMode = errors.New("platform is in maintenance mode")

// PlatformGate reads the platform-wide runtime switches (maintenance mode and
// the trading/withdrawal/deposit/registration toggles) and enforces them on the
// matching request paths. The row is cached for a short window so a hot trading
// path does not hit the database on every request; an admin write invalidates
// the cache so a halt takes effect immediately.
type PlatformGate struct {
	db *gorm.DB

	mu       sync.RWMutex
	cached   *models.SystemSettings
	cachedAt time.Time
	ttl      time.Duration
}

// NewPlatformGate builds the gate over the settings table.
func NewPlatformGate(db *gorm.DB) *PlatformGate {
	return &PlatformGate{db: db, ttl: 5 * time.Second}
}

// Settings returns the current switches, reading from the database at most once
// per TTL window. A missing row is treated as "everything enabled" (the same
// defaults the admin service creates), so a fresh database is not accidentally
// locked out.
func (g *PlatformGate) Settings() (*models.SystemSettings, error) {
	g.mu.RLock()
	if g.cached != nil && time.Since(g.cachedAt) < g.ttl {
		s := *g.cached
		g.mu.RUnlock()
		return &s, nil
	}
	g.mu.RUnlock()

	var settings models.SystemSettings
	err := g.db.Where("key = ?", "default").First(&settings).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		settings = models.SystemSettings{
			Key:                 "default",
			MaintenanceMode:     false,
			TradingEnabled:      true,
			WithdrawalEnabled:   true,
			DepositEnabled:      true,
			NewUserRegistration: true,
		}
	} else if err != nil {
		return nil, err
	}

	g.mu.Lock()
	g.cached = &settings
	g.cachedAt = time.Now()
	g.mu.Unlock()

	s := settings
	return &s, nil
}

// Invalidate drops the cache so the next read reflects an admin change.
func (g *PlatformGate) Invalidate() {
	g.mu.Lock()
	g.cached = nil
	g.mu.Unlock()
}

// TradingAllowed reports whether trading is permitted right now.
func (g *PlatformGate) TradingAllowed() (bool, error) {
	settings, err := g.Settings()
	if err != nil {
		return false, err
	}
	return settings.TradingEnabled && !settings.MaintenanceMode, nil
}

// RequireTrading is middleware that refuses a trading request while the platform
// is halted or in maintenance mode. It fails closed: a database error denies the
// request rather than allowing it through.
func (g *PlatformGate) RequireTrading() gin.HandlerFunc {
	return func(c *gin.Context) {
		allowed, err := g.TradingAllowed()
		if err != nil {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "platform settings unavailable"})
			return
		}
		if !allowed {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": ErrTradingHalted.Error()})
			return
		}
		c.Next()
	}
}

// RequireWithdrawals is middleware that refuses a withdrawal while the switch is
// off or the platform is in maintenance mode.
func (g *PlatformGate) RequireWithdrawals() gin.HandlerFunc {
	return func(c *gin.Context) {
		settings, err := g.Settings()
		if err != nil {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "platform settings unavailable"})
			return
		}
		if !settings.WithdrawalEnabled || settings.MaintenanceMode {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": ErrWithdrawalsHalted.Error()})
			return
		}
		c.Next()
	}
}

// RequireDeposits is middleware that refuses a deposit-address request while the
// switch is off or the platform is in maintenance mode.
func (g *PlatformGate) RequireDeposits() gin.HandlerFunc {
	return func(c *gin.Context) {
		settings, err := g.Settings()
		if err != nil {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "platform settings unavailable"})
			return
		}
		if !settings.DepositEnabled || settings.MaintenanceMode {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": ErrDepositsHalted.Error()})
			return
		}
		c.Next()
	}
}

// RegistrationAllowed reports whether new-user registration is open. It is read
// by the auth service rather than enforced as middleware, because registration
// is on the public auth group.
func (g *PlatformGate) RegistrationAllowed() bool {
	settings, err := g.Settings()
	if err != nil {
		// Fail closed: a settings read error must not let registration through
		// when the operator may have closed it.
		return false
	}
	return settings.NewUserRegistration && !settings.MaintenanceMode
}
