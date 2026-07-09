package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"google.golang.org/api/option"
)

type Notification struct {
	ID        string                 `json:"id"`
	Type     string                 `json:"type"`
	Title    string                 `json:"title"`
	Message  string                 `json:"message"`
	Data     map[string]interface{} `json:"data,omitempty"`
	Read     bool                   `json:"read"`
	UserID   string                 `json:"user_id"`
	DeviceID string                 `json:"device_id"`
	SentAt   int64                  `json:"sent_at"`
	ReadAt   *int64                 `json:"read_at,omitempty"`
}

type PushToken struct {
	ID        string `json:"id"`
	UserID    string `json:"user_id"`
	Token     string `json:"token"`
	Platform  string `json:"platform"` // ios, android, web
	DeviceID  string `json:"device_id"`
	DeviceName string `json:"device_name"`
	CreatedAt int64  `json:"created_at"`
	LastUsed int64  `json:"last_used"`
	Active   bool   `json:"active"`
}

type PriceAlert struct {
	ID           string  `json:"id"`
	UserID       string  `json:"user_id"`
	TokenAddress string  `json:"token_address"`
	TokenSymbol string  `json:"token_symbol"`
	Condition    string  `json:"condition"` // above, below
	TargetPrice  float64 `json:"target_price"`
	CurrentPrice float64 `json:"current_price"`
	Triggered    bool    `json:"triggered"`
	CreatedAt    int64   `json:"created_at"`
	TriggeredAt  *int64  `json:"triggered_at,omitempty"`
}

type Config struct {
	Port          string
	RedisAddr     string
	RedisPassword string
	FirebaseCreds string
	FCMAPIKey     string
}

func loadConfig() *Config {
	return &Config{
		Port:          getEnv("PORT", "8082"),
		RedisAddr:     getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),
		FirebaseCreds: getEnv("FIREBASE_CREDS", ""),
		FCMAPIKey:     getEnv("FCM_API_KEY", ""),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func main() {
	cfg := loadConfig()

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

	// Initialize Firebase (if credentials provided)
	var fcmClient *messaging.Client
	if cfg.FirebaseCreds != "" {
		opt := option.WithCredentialsFile(cfg.FirebaseCreds)
		app, err := firebase.NewApp(ctx, nil, opt)
		if err != nil {
			log.Printf("Warning: Firebase init failed: %v", err)
		} else {
			fcmClient, err = app.Messaging(ctx)
			if err != nil {
				log.Printf("Warning: FCM init failed: %v", err)
			}
		}
	}

	// Initialize router
	router := gin.Default()

	// CORS middleware
	router.Use(corsMiddleware())

	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	// API routes
	api := router.Group("/api/v1")
	{
		// Push tokens
		tokens := api.Group("/tokens")
		{
			tokens.POST("", registerPushToken(rdb))
			tokens.DELETE("/:id", unregisterPushToken(rdb))
			tokens.GET("/user/:user_id", getUserTokens(rdb))
		}

		// Notifications
		notifications := api.Group("/notifications")
		{
			notifications.GET("", getNotifications(rdb))
			notifications.GET("/:id", getNotification(rdb))
			notifications.PATCH("/:id/read", markAsRead(rdb))
			notifications.DELETE("/:id", deleteNotification(rdb))
		}

		// Price alerts
		alerts := api.Group("/alerts")
		{
			alerts.POST("", createPriceAlert(rdb))
			alerts.GET("/user/:user_id", getUserAlerts(rdb))
			alerts.DELETE("/:id", deletePriceAlert(rdb))
		}

		// Push
		push := api.Group("/push")
		{
			push.POST("/send", sendPushNotification(rdb, fcmClient))
			push.POST("/batch", sendBatchPushNotification(rdb, fcmClient))
		}
	}

	// Start price alert checker
	go startPriceAlertChecker(rdb)

	// Start server
	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: router,
	}

	go func() {
		log.Printf("Notification service starting on port %s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func registerPushToken(rdb *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req PushToken
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		req.ID = uuid.New().String()
		req.CreatedAt = time.Now().Unix()
		req.Active = true

		// Store in Redis
		data, _ := json.Marshal(req)
		rdb.Set(context.Background(), "push_token:"+req.ID, data, 0)

		// Add to user's token set
		rdb.SAdd(context.Background(), "user_tokens:"+req.UserID, req.ID)

		c.JSON(http.StatusCreated, req)
	}
}

func unregisterPushToken(rdb *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenID := c.Param("id")

		// Get token first
		data, err := rdb.Get(context.Background(), "push_token:"+tokenID).Result()
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "token not found"})
			return
		}

		var token PushToken
		json.Unmarshal([]byte(data), &token)

		// Remove from user's set
		rdb.SRem(context.Background(), "user_tokens:"+token.UserID, tokenID)

		// Delete token
		rdb.Del(context.Background(), "push_token:"+tokenID)

		c.JSON(http.StatusOK, gin.H{"status": "deleted"})
	}
}

func getUserTokens(rdb *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.Param("user_id")

		// Get all token IDs for user
		tokenIDs, err := rdb.SMembers(context.Background(), "user_tokens:"+userID).Result()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		var tokens []PushToken
		for _, id := range tokenIDs {
			data, err := rdb.Get(context.Background(), "push_token:"+id).Result()
			if err != nil {
				continue
			}
			var token PushToken
			json.Unmarshal([]byte(data), &token)
			tokens = append(tokens, token)
		}

		c.JSON(http.StatusOK, tokens)
	}
}

func getNotifications(rdb *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.Query("user_id")
		limit := c.DefaultQuery("limit", "50")
		offset := c.DefaultQuery("offset", "0")

		// Get notifications from sorted set
		notifications, err := rdb.ZRevRangeWithScores(
			context.Background(),
			"notifications:"+userID,
			int64(parseInt(offset, 0)),
			int64(parseInt(offset, 0))+int64(parseInt(limit, 50))-1,
		).Result()

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		var result []Notification
		for _, n := range notifications {
			var notif Notification
			json.Unmarshal([]byte(n.Member.(string)), &notif)
			result = append(result, notif)
		}

		c.JSON(http.StatusOK, result)
	}
}

func getNotification(rdb *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		notifID := c.Param("id")
		userID := c.Query("user_id")

		data, err := rdb.ZScore(
			context.Background(),
			"notifications:"+userID,
			notifID,
		).Result()

		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "notification not found"})
			return
		}

		notifData, _ := rdb.Get(context.Background(), "notification:"+userID+":"+notifID).Result()
		var notif Notification
		json.Unmarshal([]byte(notifData), &notif)

		c.JSON(http.StatusOK, notif)
	}
}

func markAsRead(rdb *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		notifID := c.Param("id")
		userID := c.Query("user_id")

		now := time.Now().Unix()

		// Update in Redis
		data, _ := rdb.Get(context.Background(), "notification:"+userID+":"+notifID).Result()
		var notif Notification
		json.Unmarshal([]byte(data), &notif)
		notif.Read = true
		notif.ReadAt = &now

		updated, _ := json.Marshal(notif)
		rdb.Set(context.Background(), "notification:"+userID+":"+notifID, updated, 0)

		c.JSON(http.StatusOK, notif)
	}
}

func deleteNotification(rdb *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		notifID := c.Param("id")
		userID := c.Query("user_id")

		rdb.ZRem(context.Background(), "notifications:"+userID, notifID)
		rdb.Del(context.Background(), "notification:"+userID+":"+notifID)

		c.JSON(http.StatusOK, gin.H{"status": "deleted"})
	}
}

func createPriceAlert(rdb *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req PriceAlert
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		req.ID = uuid.New().String()
		req.CreatedAt = time.Now().Unix()
		req.Triggered = false

		data, _ := json.Marshal(req)
		rdb.Set(context.Background(), "price_alert:"+req.ID, data, 0)
		rdb.SAdd(context.Background(), "user_alerts:"+req.UserID, req.ID)

		c.JSON(http.StatusCreated, req)
	}
}

func getUserAlerts(rdb *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.Param("user_id")

		alertIDs, _ := rdb.SMembers(context.Background(), "user_alerts:"+userID).Result()

		var alerts []PriceAlert
		for _, id := range alertIDs {
			data, _ := rdb.Get(context.Background(), "price_alert:"+id).Result()
			var alert PriceAlert
			json.Unmarshal([]byte(data), &alert)
			alerts = append(alerts, alert)
		}

		c.JSON(http.StatusOK, alerts)
	}
}

func deletePriceAlert(rdb *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		alertID := c.Param("id")
		userID := c.Query("user_id")

		rdb.SRem(context.Background(), "user_alerts:"+userID, alertID)
		rdb.Del(context.Background(), "price_alert:"+alertID)

		c.JSON(http.StatusOK, gin.H{"status": "deleted"})
	}
}

func sendPushNotification(rdb *redis.Client, fcmClient *messaging.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			UserID  string                 `json:"user_id" binding:"required"`
			Title   string                 `json:"title" binding:"required"`
			Message string                 `json:"message" binding:"required"`
			Data    map[string]interface{} `json:"data,omitempty"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Get user's tokens
		tokenIDs, _ := rdb.SMembers(context.Background(), "user_tokens:"+req.UserID).Result()

		notif := Notification{
			ID:       uuid.New().String(),
			Type:     "push",
			Title:    req.Title,
			Message:  req.Message,
			Data:     req.Data,
			UserID:   req.UserID,
			SentAt:   time.Now().Unix(),
			Read:     false,
		}

		// Store notification
		notifData, _ := json.Marshal(notif)
		rdb.Set(context.Background(), "notification:"+req.UserID+":"+notif.ID, notifData, 0)
		rdb.ZAdd(context.Background(), "notifications:"+req.UserID, redis.Z{
			Score:  float64(notif.SentAt),
			Member: notif.ID,
		})

		// Send to FCM
		successCount := 0
		for _, tokenID := range tokenIDs {
			tokenData, _ := rdb.Get(context.Background(), "push_token:"+tokenID).Result()
			var token PushToken
			json.Unmarshal([]byte(tokenData), &token)

			if token.Active && fcmClient != nil {
				message := &messaging.Message{
					Token: token.Token,
					Notification: &messaging.Notification{
						Title: req.Title,
						Body:  req.Message,
					},
					Data: req.Data,
				}

				_, err := fcmClient.Send(context.Background(), message)
				if err == nil {
					successCount++
				}
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"notification": notif,
			"sent":         successCount,
			"total":        len(tokenIDs),
		})
	}
}

func sendBatchPushNotification(rdb *redis.Client, fcmClient *messaging.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			UserIDs []string               `json:"user_ids" binding:"required"`
			Title   string                 `json:"title" binding:"required"`
			Message string                 `json:"message" binding:"required"`
			Data    map[string]interface{} `json:"data,omitempty"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		successCount := 0
		for _, userID := range req.UserIDs {
			// Create notification
			notif := Notification{
				ID:       uuid.New().String(),
				Type:     "push",
				Title:    req.Title,
				Message:  req.Message,
				Data:     req.Data,
				UserID:   userID,
				SentAt:   time.Now().Unix(),
				Read:     false,
			}

			notifData, _ := json.Marshal(notif)
			rdb.Set(context.Background(), "notification:"+userID+":"+notif.ID, notifData, 0)
			rdb.ZAdd(context.Background(), "notifications:"+userID, redis.Z{
				Score:  float64(notif.SentAt),
				Member: notif.ID,
			})

			successCount++
		}

		c.JSON(http.StatusOK, gin.H{"sent": successCount, "total": len(req.UserIDs)})
	}
}

func startPriceAlertChecker(rdb *redis.Client) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		ctx := context.Background()

		// Get all active alerts
		keys, _ := rdb.Keys(ctx, "price_alert:*").Result()

		for _, key := range keys {
			data, _ := rdb.Get(ctx, key).Result()
			var alert PriceAlert
			json.Unmarshal([]byte(data), &alert)

			if alert.Triggered {
				continue
			}

			// Check current price (would call price API in production)
			currentPrice := getMockPrice(alert.TokenSymbol)

			shouldTrigger := false
			if alert.Condition == "above" && currentPrice >= alert.TargetPrice {
				shouldTrigger = true
			} else if alert.Condition == "below" && currentPrice <= alert.TargetPrice {
				shouldTrigger = true
			}

			if shouldTrigger {
				now := time.Now().Unix()
				alert.Triggered = true
				alert.TriggeredAt = &now
				alert.CurrentPrice = currentPrice

				updated, _ := json.Marshal(alert)
				rdb.Set(ctx, key, updated, 0)

				// Send push notification
				rdb.Publish(ctx, "price_alert_triggered", string(updated))
			}
		}
	}
}

func getMockPrice(symbol string) float64 {
	prices := map[string]float64{
		"ETH":  2500.00,
		"BTC":  45000.00,
		"USDC": 1.00,
		"USDT": 1.00,
	}
	return prices[symbol]
}

func parseInt(s string, defaultValue int) int {
	var n int
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		}
	}
	if n == 0 {
		return defaultValue
	}
	return n
}
