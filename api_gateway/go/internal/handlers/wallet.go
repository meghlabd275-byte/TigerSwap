package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// WalletHandler handles wallet operations
type WalletHandler struct{}

// NewWalletHandler creates a new wallet handler
func NewWalletHandler() *WalletHandler {
	return &WalletHandler{}
}

// GetBalance returns user's balance for a specific token
// @Summary Get Balance
// @Description Get token balance for authenticated user
// @Tags wallet
// @Accept json
// @Produce json
// @Param token query string false "Token address"
// @Success 200 {object} map[string]string
// @Router /api/v1/wallet/balance [get]
func (h *WalletHandler) GetBalance(c *gin.Context) {
	userID, _ := c.Get("user_id")
	token := c.DefaultQuery("token", "0x0000000000000000000000000000000000000000")

	balance := map[string]string{
		"token":   token,
		"balance": "1000.00",
		"locked":  "100.00",
		"available": "900.00",
	}

	_ = userID

	c.JSON(http.StatusOK, balance)
}

// GetAllBalances returns all user balances
// @Summary Get All Balances
// @Description Get all token balances for authenticated user
// @Tags wallet
// @Accept json
// @Produce json
// @Success 200 {array} map[string]string
// @Router /api/v1/wallet/balances [get]
func (h *WalletHandler) GetAllBalances(c *gin.Context) {
	balances := []map[string]string{
		{
			"token":     "0x0000000000000000000000000000000000000000",
			"symbol":   "ETH",
			"balance":   "10.5",
			"locked":    "1.0",
			"available": "9.5",
		},
		{
			"token":     "0xdAC17F958D2ee523a2206206994597C13D831ec7",
			"symbol":   "USDT",
			"balance":   "50000.00",
			"locked":    "10000.00",
			"available": "40000.00",
		},
	}

	c.JSON(http.StatusOK, balances)
}

// Transfer handles token transfer
// @Summary Transfer
// @Description Transfer tokens to another address
// @Tags wallet
// @Accept json
// @Produce json
// @Param request body map[string]string true "Transfer request"
// @Success 200 {object} map[string]string
// @Router /api/v1/wallet/transfer [post]
func (h *WalletHandler) Transfer(c *gin.Context) {
	var req map[string]string
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	txHash := "0x" + generateID()

	c.JSON(http.StatusOK, gin.H{
		"tx_hash": txHash,
		"status":  "pending",
	})
}

// Approve handles token approval
// @Summary Approve
// @Description Approve token for spending
// @Tags wallet
// @Accept json
// @Produce json
// @Param request body map[string]string true "Approve request"
// @Success 200 {object} map[string]string
// @Router /api/v1/wallet/approve [post]
func (h *WalletHandler) Approve(c *gin.Context) {
	var req map[string]string
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	txHash := "0x" + generateID()

	c.JSON(http.StatusOK, gin.H{
		"tx_hash":   txHash,
		"status":    "pending",
		" allowance": req["amount"],
	})
}

// GetAllowances returns token allowances
// @Summary Get Allowances
// @Description Get token allowances for spenders
// @Tags wallet
// @Accept json
// @Produce json
// @Success 200 {array} map[string]string
// @Router /api/v1/wallet/allowances [get]
func (h *WalletHandler) GetAllowances(c *gin.Context) {
	allowances := []map[string]string{
		{
			"token":    "0xdAC17F958D2ee523a2206206994597C13D831ec7",
			"spender":  "0x1234567890123456789012345678901234567890",
			"allowance": "10000.00",
		},
	}

	c.JSON(http.StatusOK, allowances)
}
