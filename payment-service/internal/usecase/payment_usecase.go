package usecase

import (
	"context"
	"fmt"
	"payment-service/internal/domain"
	"payment-service/internal/messaging"

	"github.com/google/uuid"
)

// AuthorizeRequest is the input DTO for the authorize use case.
type AuthorizeRequest struct {
	OrderID       string
	Amount        int64
	CustomerEmail string // NEW: needed for notification event
}

// AuthorizeResult is the output DTO returned to the delivery layer.
type AuthorizeResult struct {
	TransactionID string
	Status        string
}

// PaymentUseCase contains all business logic for payment processing.
type PaymentUseCase struct {
	repo      PaymentRepository
	publisher MessagePublisher
}

// NewPaymentUseCase constructs the use case with its dependencies injected.
func NewPaymentUseCase(repo PaymentRepository, publisher MessagePublisher) *PaymentUseCase {
	return &PaymentUseCase{repo: repo, publisher: publisher}
}

// Authorize processes a payment authorization request.
func (uc *PaymentUseCase) Authorize(ctx context.Context, req AuthorizeRequest) (*AuthorizeResult, error) {
	// --- Idempotency check ---
	existing, err := uc.repo.FindByOrderID(ctx, req.OrderID)
	if err == nil && existing != nil {
		return &AuthorizeResult{
			TransactionID: existing.TransactionID,
			Status:        existing.Status,
		}, nil
	}

	// --- Validate domain invariants ---
	p := &domain.Payment{
		ID:            uuid.NewString(),
		OrderID:       req.OrderID,
		Amount:        req.Amount,
		CustomerEmail: req.CustomerEmail,
	}
	if err := p.Validate(); err != nil {
		return nil, err
	}

	// --- Core business rule: payment limit ---
	if domain.IsDeclined(req.Amount) {
		p.Status = domain.StatusDeclined
		p.TransactionID = uuid.NewString()
	} else {
		p.Status = domain.StatusAuthorized
		p.TransactionID = uuid.NewString()
	}

	// --- Persist ---
	if err := uc.repo.Save(ctx, p); err != nil {
		return nil, err
	}

	// --- Publish event AFTER successful DB commit (At-least-once delivery) ---
	email := req.CustomerEmail
	if email == "" {
		email = fmt.Sprintf("user-%s@example.com", req.OrderID)
	}

	event := messaging.PaymentCompletedEvent{
		EventID:       uuid.NewString(),
		OrderID:       req.OrderID,
		Amount:        float64(req.Amount) / 100.0, // cents -> dollars
		CustomerEmail: email,
		Status:        p.Status,
	}

	if pubErr := uc.publisher.PublishPaymentCompleted(ctx, event); pubErr != nil {
		return nil, fmt.Errorf("publish payment.completed event for order %s: %w", req.OrderID, pubErr)
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
