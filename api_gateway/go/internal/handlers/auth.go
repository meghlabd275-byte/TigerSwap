package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/spf13/viper"
)

// HealthCheck returns the health status of the API
// @Summary Health check
// @Description Returns the health status of the API
// @Tags health
// @Accept json
// @Produce json
// @Success 200 {object} map[string]string
// @Router /health [get]
func HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"timestamp": time.Now().Unix(),
		"version":   "1.0.0",
	})
}

// AuthRequest represents login/register request
type AuthRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
}

// AuthResponse represents auth response
type AuthResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
	User         User   `json:"user"`
}

// User represents user data
type User struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	Username  string `json:"username"`
	CreatedAt int64  `json:"created_at"`
}

// Register handles user registration
// @Summary Register
// @Description Register a new user
// @Tags auth
// @Accept json
// @Produce json
// @Param request body AuthRequest true "Register request"
// @Success 201 {object} AuthResponse
// @Router /api/v1/auth/register [post]
func Register(c *gin.Context) {
	var req AuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// In production, hash password and store in database
	user := User{
		ID:        "user_" + generateID(),
		Email:     req.Email,
		Username:  req.Email,
		CreatedAt: time.Now().Unix(),
	}

	token, refreshToken, err := generateTokens(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	c.JSON(http.StatusCreated, AuthResponse{
		AccessToken:  token,
		RefreshToken: refreshToken,
		ExpiresIn:    viper.GetInt64("jwt.expiry") * 3600,
		User:         user,
	})
}

// Login handles user authentication
// @Summary Login
// @Description Authenticate user and return tokens
// @Tags auth
// @Accept json
// @Produce json
// @Param request body AuthRequest true "Login request"
// @Success 200 {object} AuthResponse
// @Router /api/v1/auth/login [post]
func Login(c *gin.Context) {
	var req AuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// In production, verify credentials against database
	user := User{
		ID:        "user_" + generateID(),
		Email:     req.Email,
		Username:  req.Email,
		CreatedAt: time.Now().Unix(),
	}

	token, refreshToken, err := generateTokens(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, AuthResponse{
		AccessToken:  token,
		RefreshToken: refreshToken,
		ExpiresIn:    viper.GetInt64("jwt.expiry") * 3600,
		User:         user,
	})
}

// RefreshToken handles token refresh
// @Summary Refresh Token
// @Description Refresh access token using refresh token
// @Tags auth
// @Accept json
// @Produce json
// @Param request body map[string]string true "Refresh request"
// @Success 200 {object} AuthResponse
// @Router /api/v1/auth/refresh [post]
func RefreshToken(c *gin.Context) {
	var req map[string]string
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	refreshToken, ok := req["refresh_token"]
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "refresh_token required"})
		return
	}

	// Verify refresh token
	claims := &jwt.MapClaims{}
	_, err := jwt.ParseWithClaims(refreshToken, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(viper.GetString("jwt.secret")), nil
	})

	if err != nil || (*claims)["type"] != "refresh" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid refresh token"})
		return
	}

	userID := (*claims)["sub"].(string)
	token, newRefreshToken, err := generateTokens(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, AuthResponse{
		AccessToken:  token,
		RefreshToken: newRefreshToken,
		ExpiresIn:    viper.GetInt64("jwt.expiry") * 3600,
	})
}

// Logout handles user logout
// @Summary Logout
// @Description Invalidate user session
// @Tags auth
// @Accept json
// @Produce json
// @Success 200 {object} map[string]string
// @Router /api/v1/auth/logout [post]
func Logout(c *gin.Context) {
	// In production, blacklist the token
	c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
}

func generateTokens(userID string) (string, string, error) {
	// Access token
	accessClaims := jwt.MapClaims{
		"sub":  userID,
		"type": "access",
		"exp":  time.Now().Add(time.Hour * time.Duration(viper.GetInt64("jwt.expiry"))).Unix(),
		"iat":  time.Now().Unix(),
	}
	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessTokenString, err := accessToken.SignedString([]byte(viper.GetString("jwt.secret")))
	if err != nil {
		return "", "", err
	}

	// Refresh token
	refreshClaims := jwt.MapClaims{
		"sub":  userID,
		"type": "refresh",
		"exp":  time.Now().Add(time.Hour * time.Duration(viper.GetInt64("jwt.expiry") * 7)).Unix(),
		"iat":  time.Now().Unix(),
	}
	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshTokenString, err := refreshToken.SignedString([]byte(viper.GetString("jwt.secret")))
	if err != nil {
		return "", "", err
	}

	return accessTokenString, refreshTokenString, nil
}

func generateID() string {
	return time.Now().Format("20060102150405") + "_" + randomString(8)
}

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
		time.Sleep(time.Nanosecond)
	}
	return string(b)
}
