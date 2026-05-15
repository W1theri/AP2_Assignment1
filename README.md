# AP2 Assignment 4 - Caching, Background Jobs & External Integrations

Microservices: **Order**, **Payment**, **Notification** with Redis caching, rate limiting, and a reliable notification worker.

## What Changed in Assignment 4

| Component | Added / Changed |
|-----------|-----------------|
| `redis` | Shared cache, rate-limit counters, and idempotency store |
| `order-service` | Cache-aside for `GET /orders/:id` and `GET /orders/recent` (TTL from `CACHE_TTL`) |
| `order-service` | Rate limiter middleware (bonus): 10 req/min per IP → HTTP 429 |
| `notification-service` | `EmailProvider` adapter (`SIMULATED` / `REAL`) |
| `notification-service` | Exponential backoff retries (2s → 4s → 8s …) |
| `notification-service` | Redis idempotency by `event_id` before sending email |

Event flow:
```text
Client → Order Service (REST) → Payment Service (gRPC) → RabbitMQ → Notification Worker → Email Provider
                              ↘ Redis (order cache + rate limit)
Notification Worker ↔ Redis (idempotency)
```

## Cache Invalidation Strategy

**Pattern:** cache-aside in `CachedOrderRepository`.

1. **Read:** `GET /orders/:id` checks Redis (`order:{id}`). On miss → PostgreSQL → store in Redis with TTL (`CACHE_TTL`, default 300s).
2. **Write:** When order status changes (`Update` after payment or cancel), the repository:
   - updates PostgreSQL first;
   - then **deletes** `order:{id}` and recent-list keys (`orders:recent:{limit}`).
3. **Result:** the next read always loads fresh status from DB — no stale "Pending" after payment.

## Retry & Idempotency Logic

**Worker:** `EnhancedRabbitMQConsumer` (background, non-blocking).

1. **Idempotency:** before sending, check Redis key `idempotency:{event_id}`. If exists → ACK and skip (no duplicate email).
2. **Send:** call `EmailProvider.Send()` (simulated: ~500ms latency, ~10% random failures).
3. **Retry:** on failure, exponential backoff: `RETRY_INITIAL_DELAY` × 2^n, capped at `RETRY_MAX_DELAY`, up to `RETRY_MAX_ATTEMPTS`.
4. **Success:** `SET idempotency:{event_id}` (24h TTL) → manual ACK to RabbitMQ.
5. **Exhausted retries:** NACK with requeue for later processing.

Architecture diagrams: see [ARCHITECTURE_DIAGRAMS.md](./ARCHITECTURE_DIAGRAMS.md).

---

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
[SIMULATED EMAIL] To: alice@example.com, Subject: Payment Confirmed - Order #<id>, ...
[Notification] Successfully processed event <event_id>
```

### 3b. Test Redis cache (Assignment 4)

```bash
# First request — cache miss
curl http://localhost:8080/orders/<order-id>
docker compose logs order-service | grep "CACHE MISS"

# Second request — cache hit
curl http://localhost:8080/orders/<order-id>
docker compose logs order-service | grep "CACHE HIT"
```

### 3c. Test rate limiter (bonus)

Send more than 10 requests in 1 minute from the same IP — expect HTTP 429.

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
| `REDIS_URL` | Redis connection URL | `redis://localhost:6379` |
| `CACHE_TTL` | Order cache TTL (seconds) | `300` |
| `RATE_LIMIT_REQUESTS` | Max requests per window per IP | `10` |
| `RATE_LIMIT_WINDOW` | Rate limit window (seconds) | `60` |

### notification-service
| Variable | Description | Default |
|---|---|---|
| `AMQP_URL` | RabbitMQ connection URL | `amqp://guest:guest@localhost:5672/` |
| `REDIS_URL` | Redis for idempotency | `redis://localhost:6379` |
| `PROVIDER_MODE` | `SIMULATED` or `REAL` | `SIMULATED` |
| `RETRY_MAX_ATTEMPTS` | Max send retries | `5` |
| `RETRY_INITIAL_DELAY` | First backoff delay (seconds) | `2` |
| `RETRY_MAX_DELAY` | Max backoff delay (seconds) | `32` |

All variables can be set in `.env` (loaded by `docker compose`).
