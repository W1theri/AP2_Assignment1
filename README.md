# AP2 Assignment 1 – Clean Architecture Microservices (Order & Payment)

**Student:** Taubakabyl Nurlybek  
**Course:** Advanced Programming 2  
**Stack:** Go 1.21 · Gin · PostgreSQL · Docker Compose

---

## Architecture Diagram

```
┌────────────────────────────────────────────────────────────────────────┐
│                         CLIENT (curl / Postman)                         │
└─────────────────────┬──────────────────────┬──────────────────────────┘
                      │ HTTP :8080            │ HTTP :8081
                      ▼                       ▼
          ┌─────────────────────┐   ┌─────────────────────┐
          │    ORDER SERVICE    │   │   PAYMENT SERVICE   │
          │                     │   │                     │
          │  ┌───────────────┐  │   │  ┌───────────────┐  │
          │  │   Handler     │  │   │  │   Handler     │  │
          │  │ (Delivery)    │  │   │  │ (Delivery)    │  │
          │  └──────┬────────┘  │   │  └──────┬────────┘  │
          │         │           │   │         │           │
          │  ┌──────▼────────┐  │   │  ┌──────▼────────┐  │
          │  │  OrderUseCase │  │   │  │ PaymentUseCase│  │
          │  │ (Business)    │  │   │  │ (Business)    │  │
          │  └──────┬────────┘  │   │  └──────┬────────┘  │
          │         │           │   │         │           │
          │  ┌──────▼────────┐  │   │  ┌──────▼────────┐  │
          │  │  PgOrderRepo  │  │   │  │ PgPaymentRepo │  │
          │  │ (Repository)  │  │   │  │ (Repository)  │  │
          │  └──────┬────────┘  │   │  └──────┬────────┘  │
          └─────────┼───────────┘   └─────────┼───────────┘
                    │                          │
          ┌─────────▼──────┐        ┌──────────▼──────┐
          │    ORDER DB    │        │   PAYMENT DB    │
          │  (PostgreSQL)  │        │  (PostgreSQL)   │
          │    :5432       │        │    :5433        │
          └────────────────┘        └─────────────────┘
                    │
          Order Service ──REST──▶ Payment Service
          (HTTPPaymentClient)      POST /payments
          (2s timeout)
```

---

## Clean Architecture — Layer Responsibilities

| Layer | Package | Responsibility |
|---|---|---|
| **Domain** | `internal/domain` | Entities, domain errors, invariants. No deps. |
| **Use Case** | `internal/usecase` | Business logic, orchestration. Depends only on **Ports** (interfaces). |
| **Repository** | `internal/repository` | SQL adapters. Implements `OrderRepository` / `PaymentRepository` Ports. |
| **Client** | `internal/client` | HTTP adapter. Implements `PaymentClient` Port. |
| **Delivery** | `internal/transport/http` | Gin handlers. Parse HTTP → call use case → respond. |
| **Composition Root** | `cmd/*/main.go` | Wires all concrete types. Only place that knows concrete adapters. |

### Dependency Rule (strictly followed)
```
Delivery → UseCase → Domain
Repository → Domain          (implements Port defined in usecase pkg)
Client → Domain/UseCase      (implements Port defined in usecase pkg)
```
No layer imports anything "above" it. Domain imports nothing.

---

## Bounded Contexts

### Order Context
- Owns: `orders` table, `idempotency_keys` table in `order_db`
- Manages: order lifecycle (Pending → Paid/Failed/Cancelled)
- Communicates with Payment Context via REST only

### Payment Context
- Owns: `payments` table in `payment_db`
- Manages: payment authorization and transaction records
- Has NO knowledge of orders beyond the `order_id` foreign key

**No shared code, no shared models, no shared database.**

---

## API Reference

### Order Service (`:8080`)

#### `POST /orders` – Create Order
```bash
curl -X POST http://localhost:8080/orders \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: unique-key-123" \
  -d '{"customer_id":"cust-1","item_name":"Laptop","amount":50000}'
```
Response `201 Created`:
```json
{
  "id": "uuid",
  "customer_id": "cust-1",
  "item_name": "Laptop",
  "amount": 50000,
  "status": "Paid",
  "created_at": "2026-01-01T00:00:00Z"
}
```

#### `GET /orders/:id` – Get Order
```bash
curl http://localhost:8080/orders/{id}
```

#### `PATCH /orders/:id/cancel` – Cancel Order
```bash
curl -X PATCH http://localhost:8080/orders/{id}/cancel
```
- Returns `409 Conflict` if order is `Paid` or already `Cancelled`/`Failed`

---

### Payment Service (`:8081`)

#### `POST /payments` – Authorize Payment
```bash
curl -X POST http://localhost:8081/payments \
  -H "Content-Type: application/json" \
  -d '{"order_id":"uuid","amount":50000}'
```
Response `201 Created`:
```json
{"transaction_id": "uuid", "status": "Authorized"}
```
If `amount > 100000`:
```json
{"transaction_id": "uuid", "status": "Declined"}
```

#### `GET /payments/:order_id` – Get Payment
```bash
curl http://localhost:8081/payments/{order_id}
```

---

## Business Rules

| Rule | Enforced In |
|---|---|
| `amount` must be `int64` (never float64) | Domain entity |
| `amount > 0` | `domain.Order.Validate()` and `domain.Payment.Validate()` |
| `amount > 100_000` → Declined | `domain.IsDeclined()` called from PaymentUseCase |
| Paid orders cannot be cancelled | `domain.Order.CanBeCancelled()` |
| Only Pending orders can be cancelled | `domain.Order.CanBeCancelled()` |
| HTTP client timeout = 2 seconds | `client.NewHTTPPaymentClient()` |

---

## Failure Handling

### Payment Service Unavailable

When the Payment Service is down or times out:

1. `HTTPPaymentClient.Authorize()` returns an error (deadline exceeded / connection refused).
2. `OrderUseCase.CreateOrder()` catches the error and calls `order.MarkFailed()`.
3. The order status is updated to `"Failed"` in the database.
4. The handler returns `503 Service Unavailable`.

**Design Decision: Why "Failed" and not "Pending"?**

Leaving the order as `"Pending"` would be misleading — the client would not know whether payment was attempted. Marking it `"Failed"` makes the state explicit and allows the customer to retry the order. A retry with the same `Idempotency-Key` is safe because the key is only stored on success; failed orders can always be retried by submitting a new request.

---

## Bonus: Idempotency

Send `Idempotency-Key: <unique-string>` header with `POST /orders`:

- First call: creates and processes the order normally.
- Subsequent calls with the same key: return the **original order** without re-processing or charging again.
- The key→order_id mapping is stored in the `idempotency_keys` table atomically (inside a transaction).

---

## Running the Project

### Prerequisites
- Docker & Docker Compose

### Start Everything
```bash
docker-compose up --build
```

### Stop
```bash
docker-compose down -v
```

### Run Locally (without Docker)
```bash
# Terminal 1 – Payment Service
cd payment-service
DB_HOST=localhost DB_PORT=5433 DB_USER=payment_user \
  DB_PASSWORD=payment_pass DB_NAME=payment_db SERVER_PORT=8081 \
  go run ./cmd/payment-service

# Terminal 2 – Order Service
cd order-service
DB_HOST=localhost DB_PORT=5432 DB_USER=order_user \
  DB_PASSWORD=order_pass DB_NAME=order_db SERVER_PORT=8080 \
  PAYMENT_BASE_URL=http://localhost:8081 \
  go run ./cmd/order-service
```

---

## Testing Key Scenarios

```bash
# 1. Successful order (amount ≤ 100000 → Authorized → Paid)
curl -X POST http://localhost:8080/orders \
  -H "Content-Type: application/json" \
  -d '{"customer_id":"c1","item_name":"Book","amount":1500}'

# 2. Declined payment (amount > 100000 → Declined → Failed)
curl -X POST http://localhost:8080/orders \
  -H "Content-Type: application/json" \
  -d '{"customer_id":"c1","item_name":"Car","amount":200000}'

# 3. Cancel a Pending order
curl -X PATCH http://localhost:8080/orders/{id}/cancel

# 4. Try to cancel a Paid order → 409 Conflict
curl -X PATCH http://localhost:8080/orders/{paid_id}/cancel

# 5. Idempotency – send twice with same key
curl -X POST http://localhost:8080/orders \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: my-key-abc" \
  -d '{"customer_id":"c1","item_name":"Book","amount":1500}'
# Second call with same key returns same order without charging again

# 6. Payment service down → 503
docker-compose stop payment-service
curl -X POST http://localhost:8080/orders \
  -H "Content-Type: application/json" \
  -d '{"customer_id":"c1","item_name":"Book","amount":1500}'
```
