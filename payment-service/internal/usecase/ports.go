package usecase

import (
	"context"
	"payment-service/internal/domain"
	"payment-service/internal/messaging"
)

// PaymentRepository is the Port (interface) that the use case depends on.
// The actual PostgreSQL implementation lives in the repository layer.
// This enforces the Dependency Inversion Principle.
type PaymentRepository interface {
	Save(ctx context.Context, payment *domain.Payment) error
	FindByOrderID(ctx context.Context, orderID string) (*domain.Payment, error)
}

// MessagePublisher is the Port for event publishing.
// Wraps messaging.Publisher so the use case only depends on this local interface.
type MessagePublisher interface {
	PublishPaymentCompleted(ctx context.Context, event messaging.PaymentCompletedEvent) error
}
