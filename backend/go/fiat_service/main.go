package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/tigerwallet/fiat-service/config"
	"github.com/tigerwallet/fiat-service/handlers"
	"github.com/tigerwallet/fiat-service/models"
	"github.com/tigerwallet/fiat-service/services"
	"gorm.io/gorm"
)

func main() {
	// Initialize configuration
	cfg := config.Load()

	// Initialize database
	db, err := initDatabase(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// Auto-migrate models
	if err := db.AutoMigrate(
		&models.FiatOrder{},
		&models.FiatProvider{},
		&models.PaymentMethod{},
	); err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}

	// Initialize Redis
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       0,
	})

	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Printf("Warning: Redis connection failed: %v", err)
	}

	// Initialize services
	fiatService := services.NewFiatService(db, rdb, cfg)
	orderService := services.NewOrderService(db, rdb, cfg)
	
	// Initialize handlers
	fiatHandler := handlers.NewFiatHandler(fiatService, orderService)

	// Initialize router
	router := initRouter(fiatHandler)

	// Create server
	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: router,
	}

	// Start server in goroutine
	go func() {
		log.Printf("Fiat service starting on port %s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown:", err)
	}

	log.Println("Server exited properly")
}

func initDatabase(cfg *config.Config) (*gorm.DB, error) {
	dsn := "host=" + cfg.DBHost + " user=" + cfg.DBUser + 
		" password=" + cfg.DBPassword + " dbname=" + cfg.DBName + 
		" port=" + cfg.DBPort + " sslmode=disable"
	
	db, err := gorm.Open(Postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	return db, nil
}

func initRouter(handler *handlers.FiatHandler) *gin.Engine {
	router := gin.Default()

	// CORS middleware
	router.Use(corsMiddleware())

	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	// API v1
	v1 := router.Group("/api/v1")
	{
		// Fiat providers
		fiat := v1.Group("/fiat")
		{
			fiat.GET("/providers", handler.GetProviders)
			fiat.GET("/providers/:id/quote", handler.GetQuote)
			fiat.POST("/providers/:id/order", handler.CreateOrder)
			fiat.GET("/orders/:id", handler.GetOrder)
			fiat.POST("/orders/:id/complete", handler.CompleteOrder)
			fiat.POST("/orders/:id/cancel", handler.CancelOrder)
		}

		// Webhooks
		webhooks := v1.Group("/webhooks")
		{
			webhooks.POST("/stripe", handler.StripeWebhook)
			webhooks.POST("/coinbase", handler.CoinbaseWebhook)
		}
	}

	return router
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization")
		c.Header("Access-Control-Max-Age", "86400")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
