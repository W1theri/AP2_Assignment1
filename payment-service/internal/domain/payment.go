package domain

import "errors"

// Payment represents a payment transaction.
// Domain layer has NO dependencies on HTTP, JSON, or any framework.
type Payment struct {
	ID            string
	OrderID       string
	TransactionID string
	Amount        int64 // Amount in cents (e.g., 1000 = $10.00)
	Status        string // "Authorized" | "Declined"
}

// Business constants
const (
	StatusAuthorized = "Authorized"
	StatusDeclined   = "Declined"

	// MaxAllowedAmount: if amount > 100000 (i.e. > $1000.00) → Decline
	MaxAllowedAmount int64 = 100000
)

// Domain-level validation errors
var (
	ErrAmountMustBePositive = errors.New("amount must be greater than 0")
	ErrPaymentNotFound      = errors.New("payment not found")
	ErrDuplicateOrderID     = errors.New("payment for this order already exists")
)

// Validate checks domain invariants on the Payment entity.
func (p *Payment) Validate() error {
	if p.Amount <= 0 {
		return ErrAmountMustBePositive
	}
	return nil
}

// IsDeclined returns true when the amount exceeds the allowed limit.
func IsDeclined(amount int64) bool {
	return amount > MaxAllowedAmount
}
