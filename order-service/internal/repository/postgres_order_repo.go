package repository

import (
	"context"
	"database/sql"
	"order-service/internal/domain"
)

// PostgresOrderRepository is the concrete adapter that implements usecase.OrderRepository.
type PostgresOrderRepository struct {
	db *sql.DB
}

// NewPostgresOrderRepository constructs the repository.
func NewPostgresOrderRepository(db *sql.DB) *PostgresOrderRepository {
	return &PostgresOrderRepository{db: db}
}

// Save inserts a new order (without idempotency key).
func (r *PostgresOrderRepository) Save(ctx context.Context, o *domain.Order) error {
	query := `
		INSERT INTO orders (id, customer_id, item_name, amount, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)`

	_, err := r.db.ExecContext(ctx, query,
		o.ID, o.CustomerID, o.ItemName, o.Amount, o.Status, o.CreatedAt,
	)
	return err
}

// SaveWithIdempotencyKey inserts a new order and records the idempotency key atomically.
func (r *PostgresOrderRepository) SaveWithIdempotencyKey(ctx context.Context, o *domain.Order, key string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx,
		`INSERT INTO orders (id, customer_id, item_name, amount, status, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		o.ID, o.CustomerID, o.ItemName, o.Amount, o.Status, o.CreatedAt,
	)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx,
		`INSERT INTO idempotency_keys (key, order_id) VALUES ($1, $2)`,
		key, o.ID,
	)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// FindByID retrieves an order by its UUID.
func (r *PostgresOrderRepository) FindByID(ctx context.Context, id string) (*domain.Order, error) {
	query := `SELECT id, customer_id, item_name, amount, status, created_at FROM orders WHERE id = $1`

	row := r.db.QueryRowContext(ctx, query, id)
	var o domain.Order
	err := row.Scan(&o.ID, &o.CustomerID, &o.ItemName, &o.Amount, &o.Status, &o.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, domain.ErrOrderNotFound
	}
	if err != nil {
		return nil, err
	}
	return &o, nil
}

// Update writes the current state of an order back to the database.
func (r *PostgresOrderRepository) Update(ctx context.Context, o *domain.Order) error {
	query := `UPDATE orders SET status = $1 WHERE id = $2`
	_, err := r.db.ExecContext(ctx, query, o.Status, o.ID)
	return err
}

// FindRecent returns the `limit` most recently created orders, sorted by created_at DESC.
func (r *PostgresOrderRepository) FindRecent(ctx context.Context, limit int) ([]*domain.Order, error) {
	query := `
		SELECT id, customer_id, item_name, amount, status, created_at
		FROM orders
		ORDER BY created_at DESC
		LIMIT $1`

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []*domain.Order
	for rows.Next() {
		var o domain.Order
		if err := rows.Scan(&o.ID, &o.CustomerID, &o.ItemName, &o.Amount, &o.Status, &o.CreatedAt); err != nil {
			return nil, err
		}
		orders = append(orders, &o)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return orders, nil
}

// FindByIdempotencyKey looks up an order by its idempotency key.
func (r *PostgresOrderRepository) FindByIdempotencyKey(ctx context.Context, key string) (*domain.Order, error) {
	query := `
		SELECT o.id, o.customer_id, o.item_name, o.amount, o.status, o.created_at
		FROM orders o
		JOIN idempotency_keys ik ON ik.order_id = o.id
		WHERE ik.key = $1`

	row := r.db.QueryRowContext(ctx, query, key)
	var o domain.Order
	err := row.Scan(&o.ID, &o.CustomerID, &o.ItemName, &o.Amount, &o.Status, &o.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, domain.ErrOrderNotFound
	}
	if err != nil {
		return nil, err
	}
	return &o, nil
}
