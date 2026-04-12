package usecase

import (
	"context"
	"order-service/internal/domain"
	"time"

	"github.com/google/uuid"
)

// CreateOrderRequest is the input DTO for order creation.
type CreateOrderRequest struct {
	CustomerID     string
	ItemName       string
	Amount         int64
	IdempotencyKey string // optional; used for duplicate detection
}

// OrderUseCase contains all business logic for orders.
// It depends only on interfaces (Ports) — never on concrete types.
type OrderUseCase struct {
	repo          OrderRepository
	paymentClient PaymentClient
}

// NewOrderUseCase wires the use case with its dependencies.
func NewOrderUseCase(repo OrderRepository, paymentClient PaymentClient) *OrderUseCase {
	return &OrderUseCase{
		repo:          repo,
		paymentClient: paymentClient,
	}
}

// CreateOrder orchestrates the full order-creation flow:
//  1. Validate domain invariants.
//  2. Check idempotency (if a key is provided).
//  3. Persist the order as "Pending".
//  4. Call Payment Service to authorize payment.
//  5. Update order status to "Paid" or "Failed".
func (uc *OrderUseCase) CreateOrder(ctx context.Context, req CreateOrderRequest) (*domain.Order, error) {
	// --- Idempotency check ---
	if req.IdempotencyKey != "" {
		existing, err := uc.repo.FindByIdempotencyKey(ctx, req.IdempotencyKey)
		if err == nil && existing != nil {
			// Same request seen before — return the stored order instead of re-processing.
			return existing, nil
		}
	}

	// --- Build domain entity and validate ---
	order := &domain.Order{
		ID:         uuid.NewString(),
		CustomerID: req.CustomerID,
		ItemName:   req.ItemName,
		Amount:     req.Amount,
		Status:     domain.StatusPending,
		CreatedAt:  time.Now().UTC(),
	}
	if err := order.Validate(); err != nil {
		return nil, err
	}

	// --- Persist as Pending ---
	if req.IdempotencyKey != "" {
		if err := uc.repo.SaveWithIdempotencyKey(ctx, order, req.IdempotencyKey); err != nil {
			return nil, err
		}
	} else {
		if err := uc.repo.Save(ctx, order); err != nil {
			return nil, err
		}
	}

	// --- Call Payment Service (synchronous REST, with timeout enforced by client) ---
	payResult, err := uc.paymentClient.Authorize(ctx, order.ID, order.Amount)
	if err != nil {
		// Payment Service unavailable (timeout, network error, 5xx).
		// Design decision: mark order as "Failed" so it is not left in an ambiguous Pending state.
		// This is explained in the README.
		order.MarkFailed()
		_ = uc.repo.Update(ctx, order)
		return nil, err
	}

	// --- Apply payment result to order ---
	if payResult.Status == "Authorized" {
		order.MarkPaid()
	} else {
		order.MarkFailed()
	}

	if err := uc.repo.Update(ctx, order); err != nil {
		return nil, err
	}

	return order, nil
}

// GetOrder retrieves an order by ID.
func (uc *OrderUseCase) GetOrder(ctx context.Context, id string) (*domain.Order, error) {
	return uc.repo.FindByID(ctx, id)
}

// GetRecentOrders returns the N most recently created orders.
// The limit is validated here (business rule: 1–100).
func (uc *OrderUseCase) GetRecentOrders(ctx context.Context, limit int) ([]*domain.Order, error) {
	if limit <= 0 {
		limit = 10 // sensible default
	}
	if limit > 100 {
		limit = 100 // guard against abuse
	}
	return uc.repo.FindRecent(ctx, limit)
}

// CancelOrder attempts to cancel a Pending order.
// Business rule enforced here: only Pending orders can be cancelled.
func (uc *OrderUseCase) CancelOrder(ctx context.Context, id string) (*domain.Order, error) {
	order, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if err := order.Cancel(); err != nil {
		return nil, err
	}

	if err := uc.repo.Update(ctx, order); err != nil {
		return nil, err
	}

	return order, nil
}
