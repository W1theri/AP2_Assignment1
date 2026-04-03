package usecase

import (
	"context"
	"payment-service/internal/domain"

	"github.com/google/uuid"
)

// AuthorizeRequest is the input DTO for the authorize use case.
type AuthorizeRequest struct {
	OrderID string
	Amount  int64
}

// AuthorizeResult is the output DTO returned to the delivery layer.
type AuthorizeResult struct {
	TransactionID string
	Status        string
}

// PaymentUseCase contains all business logic for payment processing.
// It depends only on the PaymentRepository interface (Port), never on a concrete type.
type PaymentUseCase struct {
	repo PaymentRepository
}

// NewPaymentUseCase constructs the use case with its dependency injected.
func NewPaymentUseCase(repo PaymentRepository) *PaymentUseCase {
	return &PaymentUseCase{repo: repo}
}

// Authorize processes a payment authorization request.
// Business rules enforced here (NOT in the handler):
//  1. Amount must be > 0.
//  2. Amount > 100000 → Declined.
//  3. Each OrderID is idempotent: a second call returns the existing result.
func (uc *PaymentUseCase) Authorize(ctx context.Context, req AuthorizeRequest) (*AuthorizeResult, error) {
	// --- Idempotency check ---
	existing, err := uc.repo.FindByOrderID(ctx, req.OrderID)
	if err == nil && existing != nil {
		// Payment already processed for this order — return the stored result.
		return &AuthorizeResult{
			TransactionID: existing.TransactionID,
			Status:        existing.Status,
		}, nil
	}

	// --- Validate domain invariants ---
	p := &domain.Payment{
		ID:      uuid.NewString(),
		OrderID: req.OrderID,
		Amount:  req.Amount,
	}
	if err := p.Validate(); err != nil {
		return nil, err
	}

	// --- Core business rule: payment limit ---
	if domain.IsDeclined(req.Amount) {
		p.Status = domain.StatusDeclined
		p.TransactionID = uuid.NewString() // still record the attempt
	} else {
		p.Status = domain.StatusAuthorized
		p.TransactionID = uuid.NewString()
	}

	// --- Persist ---
	if err := uc.repo.Save(ctx, p); err != nil {
		return nil, err
	}

	return &AuthorizeResult{
		TransactionID: p.TransactionID,
		Status:        p.Status,
	}, nil
}

// GetByOrderID retrieves a payment record by its associated order ID.
func (uc *PaymentUseCase) GetByOrderID(ctx context.Context, orderID string) (*domain.Payment, error) {
	return uc.repo.FindByOrderID(ctx, orderID)
}
