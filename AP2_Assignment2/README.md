# AP2 Assignment 2 — gRPC Migration

**Student:** YOUR_NAME YOUR_SURNAME  
**Group:** YOUR_GROUP

## Repository Links

| Repo | URL |
|------|-----|
| Proto files (Repo A) | https://github.com/YOURUSERNAME/ap2-protos |
| Generated code (Repo B) | https://github.com/YOURUSERNAME/ap2-generated |
| Services (this repo, branch `grpc-migration`) | https://github.com/YOURUSERNAME/AP2_Assignment1 |

---

## What Changed from Assignment 1

| Layer | Assignment 1 | Assignment 2 |
|-------|-------------|--------------|
| `payment-service` delivery | HTTP handler (`transport/http/`) | **+ gRPC handler** (`transport/grpc/`) |
| `order-service` client | `HTTPPaymentClient` (REST) | **`GRPCPaymentClient`** (gRPC) |
| `order-service` delivery | REST only | REST + **gRPC streaming server** |
| Use Cases | — | **Unchanged** |
| Domain entities | — | **Unchanged** |
| Repository | — | **+ `WatchOrderStatus()`** for streaming |

---

## Architecture

```
┌─────────────┐   POST /orders    ┌────────────────────────────────────────┐
│  Postman /  │ ────────────────► │            order-service                │
│  Frontend   │                   │  ┌──────────────────────────────────┐   │
└─────────────┘                   │  │  REST Handler (Gin) :8080        │   │
                                  │  │  POST /orders                    │   │
                                  │  │  GET  /orders/recent             │   │
┌─────────────┐  gRPC streaming   │  │  GET  /orders/:id                │   │
│ stream-     │ ◄──────────────── │  │  PATCH /orders/:id/cancel        │   │
│ client CLI  │  :50052           │  ├──────────────────────────────────┤   │
└─────────────┘                   │  │  Use Case (UNCHANGED from A1)    │   │  gRPC ProcessPayment
                                  │  ├──────────────────────────────────┤   │ ─────────────────────►
                                  │  │  GRPCPaymentClient :50051        │   │   payment-service
                                  │  ├──────────────────────────────────┤   │   ┌────────────────┐
                                  │  │  OrderRepo + WatchOrderStatus    │   │   │  gRPC Server   │
                                  │  ├──────────────────────────────────┤   │   │  :50051        │
                                  │  │  gRPC Streaming Server :50052    │   │   ├────────────────┤
                                  │  └──────────────────────────────────┘   │   │  UseCase (A1)  │
                                  │              │                           │   ├────────────────┤
                                  │         order-db                         │   │  + Interceptor │
                                  └────────────────────────────────────────┘   ├────────────────┤
                                                                                │  payment-db    │
                                                                                └────────────────┘

Contract-First Flow:
  ap2-protos (Repo A) ──push──► GitHub Actions (protoc) ──► ap2-generated (Repo B)
                                                                     │
                                              go get github.com/YOURUSERNAME/ap2-generated
                                                     │                     │
                                              order-service          payment-service
```

---

## How to Run

### Prerequisites
- Docker & Docker Compose
- Go 1.21+

### 1. Start all services
```bash
docker compose up --build
```

### 2. Create an order (triggers gRPC call to Payment Service)
```bash
curl -X POST http://localhost:8080/orders \
  -H "Content-Type: application/json" \
  -d '{
    "customer_id": "customer-123",
    "item_name":   "Laptop",
    "amount":      50000
  }'
```

Expected response (amount 50000 cents = $500 → within $1000 limit → Authorized):
```json
{
  "id":          "550e8400-e29b-41d4-a716-446655440000",
  "customer_id": "customer-123",
  "item_name":   "Laptop",
  "amount":      50000,
  "status":      "Paid",
  "created_at":  "2025-04-12T10:00:00Z"
}
```

### 3. Test Declined payment (amount > 100000 cents = > $1000)
```bash
curl -X POST http://localhost:8080/orders \
  -H "Content-Type: application/json" \
  -d '{"customer_id":"customer-1","item_name":"Yacht","amount":200000}'
```

Response: status = "Failed" (Payment Service's business rule: amount > 100000 → Declined)

### 4. Test Server-side Streaming (real-time order tracking)

**Terminal 1** — subscribe to updates:
```bash
export ORDER_ID=<id-from-step-2>
cd order-service
go run ./cmd/stream-client/main.go
```

Output:
```
✓ Subscribed to order 550e8400-...
Waiting for real-time status updates from DB...

[10:00:00.123]  order_id=550e8400-...  status=Paid
```

**Terminal 2** — cancel the order (triggers real DB change → stream push):
```bash
curl -X PATCH http://localhost:8080/orders/$ORDER_ID/cancel
```

Immediately in Terminal 1:
```
[10:00:05.456]  order_id=550e8400-...  status=Cancelled
```

### 5. Verify gRPC Interceptor logs (Bonus)
```bash
docker compose logs payment-service | grep interceptor
```

Output:
```
[gRPC interceptor] method=/payment.PaymentService/ProcessPayment  duration=1.234ms  err=<nil>
```

### 6. Other REST endpoints (unchanged from Assignment 1)
```bash
# Get recent orders
curl http://localhost:8080/orders/recent?limit=5

# Get specific order
curl http://localhost:8080/orders/<id>

# Cancel order (must be Pending)
curl -X PATCH http://localhost:8080/orders/<id>/cancel
```

---

## Environment Variables

### payment-service
| Variable | Description | Default |
|---|---|---|
| `DB_HOST` | PostgreSQL host | `localhost` |
| `DB_PORT` | PostgreSQL port | `5433` |
| `DB_USER` | DB username | `payment_user` |
| `DB_PASSWORD` | DB password | `payment_pass` |
| `DB_NAME` | DB name | `payment_db` |
| `SERVER_PORT` | HTTP port (legacy) | `8081` |
| `GRPC_PORT` | **gRPC server address** | `:50051` |

### order-service
| Variable | Description | Default |
|---|---|---|
| `DB_HOST` | PostgreSQL host | `localhost` |
| `DB_PORT` | PostgreSQL port | `5432` |
| `DB_USER` | DB username | `order_user` |
| `DB_PASSWORD` | DB password | `order_pass` |
| `DB_NAME` | DB name | `order_db` |
| `SERVER_PORT` | REST HTTP port | `8080` |
| `PAYMENT_GRPC_ADDR` | **Payment gRPC target** | `localhost:50051` |
| `GRPC_STREAM_PORT` | **Order streaming gRPC port** | `:50052` |

---

## Grading Checklist

| Criterion | Evidence |
|---|---|
| **Contract-First 30%** | `.github/workflows/generate.yml` auto-generates `.pb.go` on push to `ap2-protos` |
| **gRPC Implementation 30%** | `payment-service/internal/transport/grpc/payment_handler.go` (server); `order-service/internal/client/payment_grpc_client.go` (client) |
| **Clean Architecture** | `order_usecase.go`, `payment_usecase.go`, all domain files — zero changes from A1 |
| **Proto Design 15%** | `int64 amount` (cents), `google.protobuf.Timestamp`, `go_package`, separate services |
| **Streaming + DB 15%** | `WatchOrderStatus()` polls PostgreSQL every 500ms; status pushed only on real change |
| **Documentation 10%** | This README + architecture diagram + git history |
| **BONUS Interceptor +10%** | `payment-service/internal/interceptor/logging.go` — logs method + duration |
