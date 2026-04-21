package http

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"order-service/internal/domain"
	"order-service/internal/usecase"
)

// OrderHandler is the thin delivery layer.
// Parse -> call use case -> respond.
type OrderHandler struct {
	uc *usecase.OrderUseCase
}

// NewOrderHandler constructs the handler.
func NewOrderHandler(uc *usecase.OrderUseCase) *OrderHandler {
	return &OrderHandler{uc: uc}
}

// RegisterRoutes registers all order endpoints on the router.
func (h *OrderHandler) RegisterRoutes(r *gin.Engine) {
	r.POST("/orders", h.CreateOrder)
	r.GET("/orders/recent", h.GetRecentOrders)
	r.GET("/orders/:id", h.GetOrder)
	r.PATCH("/orders/:id/cancel", h.CancelOrder)
}

// GetRecentOrders handles GET /orders/recent?limit=5.
func (h *OrderHandler) GetRecentOrders(c *gin.Context) {
	limit := 10
	if raw := c.Query("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "limit must be a positive integer"})
			return
		}
		limit = parsed
	}

	orders, err := h.uc.GetRecentOrders(c.Request.Context(), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	result := make([]gin.H, 0, len(orders))
	for _, o := range orders {
		result = append(result, orderResponse(o))
	}

	c.JSON(http.StatusOK, gin.H{
		"limit":  limit,
		"count":  len(result),
		"orders": result,
	})
}

type createOrderRequest struct {
	CustomerID string `json:"customer_id" binding:"required"`
	ItemName   string `json:"item_name" binding:"required"`
	Amount     int64  `json:"amount" binding:"required"`
}

// CreateOrder handles POST /orders.
func (h *OrderHandler) CreateOrder(c *gin.Context) {
	var req createOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	idempotencyKey := c.GetHeader("Idempotency-Key")

	order, err := h.uc.CreateOrder(c.Request.Context(), usecase.CreateOrderRequest{
		CustomerID:     req.CustomerID,
		ItemName:       req.ItemName,
		Amount:         req.Amount,
		IdempotencyKey: idempotencyKey,
	})
	if err != nil {
		if errors.Is(err, domain.ErrAmountMustBePositive) {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
			return
		}
		if errors.Is(err, usecase.ErrPaymentUnavailable) {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error":  "payment service unavailable",
				"detail": err.Error(),
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":  "internal server error",
			"detail": err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, orderResponse(order))
}

// GetOrder handles GET /orders/:id.
func (h *OrderHandler) GetOrder(c *gin.Context) {
	id := c.Param("id")

	order, err := h.uc.GetOrder(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrOrderNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.JSON(http.StatusOK, orderResponse(order))
}

// CancelOrder handles PATCH /orders/:id/cancel.
func (h *OrderHandler) CancelOrder(c *gin.Context) {
	id := c.Param("id")

	order, err := h.uc.CancelOrder(c.Request.Context(), id)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrOrderNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
		case errors.Is(err, domain.ErrCannotCancelPaidOrder):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		case errors.Is(err, domain.ErrCannotCancelOrder):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
		return
	}

	c.JSON(http.StatusOK, orderResponse(order))
}

func orderResponse(o *domain.Order) gin.H {
	return gin.H{
		"id":          o.ID,
		"customer_id": o.CustomerID,
		"item_name":   o.ItemName,
		"amount":      o.Amount,
		"status":      o.Status,
		"created_at":  o.CreatedAt,
	}
}
