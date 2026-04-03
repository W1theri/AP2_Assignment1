package http

import (
	"errors"
	"net/http"
	"payment-service/internal/domain"
	"payment-service/internal/usecase"

	"github.com/gin-gonic/gin"
)

// PaymentHandler is the thin delivery layer.
// Its only responsibility: parse HTTP request → call use case → format HTTP response.
// NO business logic lives here.
type PaymentHandler struct {
	uc *usecase.PaymentUseCase
}

// NewPaymentHandler constructs the handler with its use case dependency.
func NewPaymentHandler(uc *usecase.PaymentUseCase) *PaymentHandler {
	return &PaymentHandler{uc: uc}
}

// RegisterRoutes wires the handler to a Gin router group.
func (h *PaymentHandler) RegisterRoutes(r *gin.Engine) {
	r.POST("/payments", h.Authorize)
	r.GET("/payments/:order_id", h.GetByOrderID)
}

// authorizeRequest is the JSON payload for POST /payments.
type authorizeRequest struct {
	OrderID string `json:"order_id" binding:"required"`
	Amount  int64  `json:"amount"   binding:"required"`
}

// Authorize handles POST /payments.
func (h *PaymentHandler) Authorize(c *gin.Context) {
	var req authorizeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := h.uc.Authorize(c.Request.Context(), usecase.AuthorizeRequest{
		OrderID: req.OrderID,
		Amount:  req.Amount,
	})
	if err != nil {
		if errors.Is(err, domain.ErrAmountMustBePositive) {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	statusCode := http.StatusCreated
	if result.Status == domain.StatusDeclined {
		statusCode = http.StatusOK // still 200, but status field = "Declined"
	}

	c.JSON(statusCode, gin.H{
		"transaction_id": result.TransactionID,
		"status":         result.Status,
	})
}

// GetByOrderID handles GET /payments/:order_id.
func (h *PaymentHandler) GetByOrderID(c *gin.Context) {
	orderID := c.Param("order_id")

	payment, err := h.uc.GetByOrderID(c.Request.Context(), orderID)
	if err != nil {
		if errors.Is(err, domain.ErrPaymentNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "payment not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":             payment.ID,
		"order_id":       payment.OrderID,
		"transaction_id": payment.TransactionID,
		"amount":         payment.Amount,
		"status":         payment.Status,
	})
}
