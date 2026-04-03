package usecase

import (
	"context"
	"payment-service/internal/domain"
)

// PaymentRepository is the Port (interface) that the use case depends on.
// The actual PostgreSQL implementation lives in the repository layer.
// This enforces the Dependency Inversion Principle.
type PaymentRepository interface {
	Save(ctx context.Context, payment *domain.Payment) error
	FindByOrderID(ctx context.Context, orderID string) (*domain.Payment, error)
}
