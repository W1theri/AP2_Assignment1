package repository

import (
	"context"
	"database/sql"
	"payment-service/internal/domain"
)

// PostgresPaymentRepository is the concrete adapter that implements usecase.PaymentRepository.
// All SQL logic is contained here; the use case never sees SQL.
type PostgresPaymentRepository struct {
	db *sql.DB
}

// NewPostgresPaymentRepository constructs the repository with an open *sql.DB.
func NewPostgresPaymentRepository(db *sql.DB) *PostgresPaymentRepository {
	return &PostgresPaymentRepository{db: db}
}

// Save inserts a new payment record into the payments table.
func (r *PostgresPaymentRepository) Save(ctx context.Context, p *domain.Payment) error {
	query := `
		INSERT INTO payments (id, order_id, transaction_id, amount, status)
		VALUES ($1, $2, $3, $4, $5)`

	_, err := r.db.ExecContext(ctx, query,
		p.ID,
		p.OrderID,
		p.TransactionID,
		p.Amount,
		p.Status,
	)
	return err
}

// FindByOrderID retrieves a payment by its order_id.
// Returns domain.ErrPaymentNotFound when no row exists.
func (r *PostgresPaymentRepository) FindByOrderID(ctx context.Context, orderID string) (*domain.Payment, error) {
	query := `SELECT id, order_id, transaction_id, amount, status FROM payments WHERE order_id = $1`

	row := r.db.QueryRowContext(ctx, query, orderID)

	var p domain.Payment
	err := row.Scan(&p.ID, &p.OrderID, &p.TransactionID, &p.Amount, &p.Status)
	if err == sql.ErrNoRows {
		return nil, domain.ErrPaymentNotFound
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}
