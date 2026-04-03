package usecase

import (
	"context"
	"order-service/internal/domain"
)

// OrderRepository is the Port for persistence.
// The use case layer depends on this interface, never on *sql.DB or any concrete type.
type OrderRepository interface {
	Save(ctx context.Context, order *domain.Order) error
	FindByID(ctx context.Context, id string) (*domain.Order, error)
	Update(ctx context.Context, order *domain.Order) error
	FindByIdempotencyKey(ctx context.Context, key string) (*domain.Order, error)
	SaveWithIdempotencyKey(ctx context.Context, order *domain.Order, key string) error
	FindRecent(ctx context.Context, limit int) ([]*domain.Order, error)
}

// PaymentResult is the value returned by the PaymentClient Port.
type PaymentResult struct {
	TransactionID string
	Status        string // "Authorized" | "Declined"
}

// PaymentClient is the Port for outbound HTTP communication.
// The concrete HTTP implementation lives in the client package.
// This allows the use case to be tested with a mock client.
type PaymentClient interface {
	Authorize(ctx context.Context, orderID string, amount int64) (*PaymentResult, error)
}
