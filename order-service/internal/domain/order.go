package domain

import (
	"errors"
	"time"
)

// Order represents a customer order.
// This domain model is INTERNAL to the Order Service.
// The Payment Service has its own separate Payment entity — they are never shared.
type Order struct {
	ID         string
	CustomerID string
	ItemName   string
	Amount     int64 // Amount in cents: 1000 = $10.00. MUST be int64, never float64.
	Status     string
	CreatedAt  time.Time
}

// Order status constants
const (
	StatusPending   = "Pending"
	StatusPaid      = "Paid"
	StatusFailed    = "Failed"
	StatusCancelled = "Cancelled"
)

// Domain errors
var (
	ErrOrderNotFound        = errors.New("order not found")
	ErrAmountMustBePositive = errors.New("amount must be greater than 0")
	ErrCannotCancelPaidOrder = errors.New("paid orders cannot be cancelled")
	ErrCannotCancelOrder    = errors.New("only pending orders can be cancelled")
)

// Validate checks domain invariants.
// This enforces the "Amount must be > 0" business rule at the domain level.
func (o *Order) Validate() error {
	if o.Amount <= 0 {
		return ErrAmountMustBePositive
	}
	return nil
}

// CanBeCancelled returns true only if the order is in Pending state.
func (o *Order) CanBeCancelled() error {
	if o.Status == StatusPaid {
		return ErrCannotCancelPaidOrder
	}
	if o.Status != StatusPending {
		return ErrCannotCancelOrder
	}
	return nil
}

// MarkPaid transitions the order to Paid status.
func (o *Order) MarkPaid() {
	o.Status = StatusPaid
}

// MarkFailed transitions the order to Failed status.
func (o *Order) MarkFailed() {
	o.Status = StatusFailed
}

// Cancel transitions the order to Cancelled status.
func (o *Order) Cancel() error {
	if err := o.CanBeCancelled(); err != nil {
		return err
	}
	o.Status = StatusCancelled
	return nil
}
