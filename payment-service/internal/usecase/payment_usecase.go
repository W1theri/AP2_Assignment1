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
	CustomerEmail string
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
	existing, err := uc.repo.FindByOrderID(ctx, req.OrderID)
	if err == nil && existing != nil {
		return &AuthorizeResult{
			TransactionID: existing.TransactionID,
			Status:        existing.Status,
		}, nil
	}

	p := &domain.Payment{
		ID:            uuid.NewString(),
		OrderID:       req.OrderID,
		Amount:        req.Amount,
		CustomerEmail: req.CustomerEmail,
	}
	if err := p.Validate(); err != nil {
		return nil, err
	}

	if domain.IsDeclined(req.Amount) {
		p.Status = domain.StatusDeclined
		p.TransactionID = uuid.NewString()
	} else {
		p.Status = domain.StatusAuthorized
		p.TransactionID = uuid.NewString()
	}

	if err := uc.repo.Save(ctx, p); err != nil {
		return nil, err
	}

	email := req.CustomerEmail
	if email == "" {
		email = fmt.Sprintf("user-%s@example.com", req.OrderID)
	}

	event := messaging.PaymentCompletedEvent{
		EventID:       uuid.NewString(),
		OrderID:       req.OrderID,
		TransactionID: p.TransactionID,
		Amount:        float64(req.Amount) / 100.0,
		CustomerEmail: email,
		Status:        p.Status,
	}

	if err := uc.publisher.PublishPaymentCompleted(ctx, event); err != nil {
		return nil, fmt.Errorf("publish payment.completed event for order %s: %w", req.OrderID, err)
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
