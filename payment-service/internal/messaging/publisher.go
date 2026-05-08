package messaging

import "context"

// PaymentCompletedEvent is the event published after a payment is processed.
type PaymentCompletedEvent struct {
	EventID       string  `json:"event_id"`
	OrderID       string  `json:"order_id"`
	Amount        float64 `json:"amount"` // in dollars
	CustomerEmail string  `json:"customer_email"`
	Status        string  `json:"status"`
}

// Publisher is the port (interface) for publishing payment events.
// The use case depends on this abstraction — not on RabbitMQ directly.
type Publisher interface {
	PublishPaymentCompleted(ctx context.Context, event PaymentCompletedEvent) error
	Close()
}
