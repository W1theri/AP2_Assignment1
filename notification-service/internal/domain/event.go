package domain

// PaymentCompletedEvent is the message payload published by Payment Service
// and consumed by Notification Service.
type PaymentCompletedEvent struct {
	EventID       string  `json:"event_id"`       // Unique ID for idempotency
	OrderID       string  `json:"order_id"`
	Amount        float64 `json:"amount"`         // Amount in dollars
	CustomerEmail string  `json:"customer_email"`
	Status        string  `json:"status"` // "Authorized" | "Declined"
}
