# AP2 Assignment 3 - gRPC + Event-Driven Notifications

## What Changed from Assignment 1 to Assignment 2

| Layer | Assignment 1 | Assignment 2 |
|-------|--------------|--------------|
| `payment-service` delivery | HTTP handler (`transport/http/`) | Added gRPC handler (`transport/grpc/`) |
| `order-service` client | `HTTPPaymentClient` (REST) | `GRPCPaymentClient` (gRPC) |
| `order-service` delivery | REST only | REST + gRPC streaming server |
| Use Cases | Existing business logic | Preserved |
| Domain entities | Existing entities | Preserved |
| Repository | Basic CRUD | Added `WatchOrderStatus()` for streaming |

## What Changed in Assignment 3

Assignment 3 keeps the synchronous gRPC flow from Assignment 2 and adds asynchronous notifications through RabbitMQ.

| Component | Added / Changed |
|-----------|-----------------|
| `rabbitmq` | Message broker added to `docker-compose.yml` |
| `notification-service` | New consumer service for `payment.completed` events |
| `payment-service` | Publishes `payment.completed` after a successful payment save |
| `PaymentRequest` | Added `customer_email` so notifications use the real customer email |
| RabbitMQ reliability | Durable queue, persistent messages, manual ACKs, and publisher confirms |
| Idempotency | Notification Service skips duplicated events by `event_id` |

Event flow:
```text
Client -> Order Service REST -> Payment Service gRPC -> RabbitMQ -> Notification Service
```

## How to Run

### Prerequisites
- Docker & Docker Compose
- Go 1.21+

### 1. Start all services
```bash
docker compose up --build
```

### 2. Create an order

This triggers:
1. REST request to Order Service
2. gRPC call from Order Service to Payment Service
3. RabbitMQ event from Payment Service
4. Notification log from Notification Service

```bash
curl -X POST http://localhost:8080/orders \
  -H "Content-Type: application/json" \
  -d '{
    "customer_id": "customer-123",
    "item_name": "Laptop",
    "amount": 50000,
    "customer_email": "alice@example.com"
  }'
```

Expected order response: `status = "Paid"` when payment is authorized.

### 3. Check asynchronous notification

```bash
docker compose logs notification-service -f
```

Expected log:
```text
[Notification] Sent email to alice@example.com for Order #<id>. Amount: $500.00. Status: Authorized
```

### 4. Test declined payment

Amount greater than `100000` cents is declined by Payment Service.

```bash
curl -X POST http://localhost:8080/orders \
  -H "Content-Type: application/json" \
  -d '{"customer_id":"customer-1","item_name":"Yacht","amount":200000,"customer_email":"alice@example.com"}'
```

Expected order response: `status = "Failed"`.

### 5. Test server-side streaming

Terminal 1 - subscribe to updates:
```bash
export ORDER_ID=<id-from-step-2>
cd order-service
go run ./cmd/stream-client/main.go
```

Terminal 2 - cancel the order:
```bash
curl -X PATCH http://localhost:8080/orders/$ORDER_ID/cancel
```

The stream client receives the new status from the database-backed watcher.

### 6. Verify gRPC interceptor logs
```bash
docker compose logs payment-service | grep interceptor
```

Expected output:
```text
[gRPC interceptor] method=/payment.PaymentService/ProcessPayment duration=<duration> err=<nil>
```

## Reliability Notes

| Feature | Implementation |
|---------|----------------|
| Durable queue | `QueueDeclare(..., durable=true, ...)` |
| Persistent messages | `DeliveryMode: amqp.Persistent` |
| Publisher confirms | Payment Service waits for broker ACK after publish |
| Manual ACK | Notification Service uses `autoAck=false` |
| Duplicate handling | Notification Service stores processed `event_id` values |
| Graceful shutdown | Services listen for OS signals and close resources |

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
| `GRPC_PORT` | gRPC server address | `:50051` |
| `AMQP_URL` | RabbitMQ connection URL | `amqp://guest:guest@localhost:5672/` |

### order-service
| Variable | Description | Default |
|---|---|---|
| `DB_HOST` | PostgreSQL host | `localhost` |
| `DB_PORT` | PostgreSQL port | `5432` |
| `DB_USER` | DB username | `order_user` |
| `DB_PASSWORD` | DB password | `order_pass` |
| `DB_NAME` | DB name | `order_db` |
| `SERVER_PORT` | REST HTTP port | `8080` |
| `PAYMENT_GRPC_ADDR` | Payment gRPC target | `localhost:50051` |
| `GRPC_STREAM_PORT` | Order streaming gRPC port | `:50052` |

### notification-service
| Variable | Description | Default |
|---|---|---|
| `AMQP_URL` | RabbitMQ connection URL | `amqp://guest:guest@localhost:5672/` |
