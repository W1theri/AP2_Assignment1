-- migrations/001_create_orders.sql
-- Order Service owns these tables exclusively.
-- Payment Service must NEVER query or write to this database directly.

CREATE TABLE IF NOT EXISTS orders (
    id          VARCHAR(36)  PRIMARY KEY,
    customer_id VARCHAR(36)  NOT NULL,
    item_name   VARCHAR(255) NOT NULL,
    amount      BIGINT       NOT NULL CHECK (amount > 0),
    status      VARCHAR(20)  NOT NULL CHECK (status IN ('Pending', 'Paid', 'Failed', 'Cancelled')),
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- Idempotency keys table (Bonus: prevents duplicate order creation)
-- Stores a client-supplied key mapped to the resulting order_id.
CREATE TABLE IF NOT EXISTS idempotency_keys (
    key        VARCHAR(255) PRIMARY KEY,
    order_id   VARCHAR(36)  NOT NULL REFERENCES orders(id),
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_orders_customer_id ON orders(customer_id);
CREATE INDEX IF NOT EXISTS idx_orders_status      ON orders(status);
