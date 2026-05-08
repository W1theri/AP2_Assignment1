# AP2 Assignment 3 — Event-Driven Architecture with RabbitMQ


## Architecture Overview

```
HTTP Client
    │
    ▼
Order Service (HTTP :8080 / gRPC :50052)
    │  gRPC ProcessPayment
    ▼
Payment Service (HTTP :8081 / gRPC :50051)
    │
    ├── PostgreSQL (payment-db :5433)  ← persist payment
    │
    └── RabbitMQ (amqp :5672)  ← publish payment.completed event
              │
              ▼  (durable queue)
    Notification Service (consumer)
              │
              └── Logs: [Notification] Sent email to user@example.com for Order #123. Amount: $99.99
```

**Event flow:**
1. Client calls `POST /orders` on Order Service
2. Order Service calls Payment Service via gRPC
3. Payment Service persists payment to PostgreSQL in a DB transaction
4. **After commit**, Payment Service publishes `PaymentCompletedEvent` to RabbitMQ queue `payment.completed`
5. Notification Service consumes the message, logs the notification, and ACKs the message

---

## Running

```bash
docker-compose up --build
```

All services start automatically. RabbitMQ Management UI available at http://localhost:15672 (guest/guest).

### Test the full flow

```bash
# 1. Create an order (triggers payment + notification)
curl -X POST http://localhost:8080/orders \
  -H "Content-Type: application/json" \
  -d '{"customer_id": "cust-1", "item_name": "AP2 Book", "amount": 2500, "customer_email": "alice@example.com"}'

# 2. Watch notification-service logs
docker logs notification-service -f
# Expected output:
# [Notification] Sent email to alice@example.com for Order #<id>. Amount: $25.00. Status: Authorized
```

---

## Idempotency Strategy

**Implementation:** In-memory store with TTL (`notification-service/internal/idempotency/store.go`)

Each published event carries a unique `event_id` (UUID generated at publish time). When the Notification Service receives a message:

1. It checks the `idempotency.Store` for the `event_id`
2. **If found** → the event was already processed → ACK the message and return (no duplicate email)
3. **If not found** → process normally → `store.MarkProcessed(eventID)` → ACK

**TTL:** 24 hours — entries expire automatically to prevent unbounded memory growth.

**Why in-memory?** Sufficient for this assignment. In production, a Redis or PostgreSQL-backed store would survive service restarts.

---

## ACK Logic

**Location:** `notification-service/internal/consumer/consumer.go`

```
autoAck = false  ← manual ACKs required
```

| Scenario | Action |
|---|---|
| Message parsed & email logged successfully | `msg.Ack(false)` — removes from queue |
| Unmarshal error (poison message) | `msg.Nack(false, false)` — discard, no requeue |
| Transient processing error | `msg.Nack(false, true)` — requeue for retry |
| Duplicate event_id (idempotency hit) | `msg.Ack(false)` — safe to discard |

ACK is sent **only after** the business effect (log) succeeds. This ensures **at-least-once delivery** — if the service crashes after processing but before ACK, the message is redelivered.

---

## Reliability Features

| Feature | Implementation |
|---|---|
| **Durable queue** | `durable: true` in `QueueDeclare` — survives broker restart |
| **Persistent messages** | `DeliveryMode: amqp.Persistent` in publisher |
| **Publisher confirms** | Payment Service waits for broker ACK before treating publish as successful |
| **Manual ACKs** | `autoAck: false` in consumer |
| **QoS prefetch=1** | `ch.Qos(1, 0, false)` — one message at a time, fair dispatch |
| **At-least-once delivery** | ACK only after success + idempotent consumer |
| **Idempotent consumer** | UUID event_id deduplication in in-memory store |
| **Graceful shutdown** | `os/signal` → `context.WithCancel` → consumer drains |
| **Connection retry** | Both services retry RabbitMQ connection 10× with 3s backoff |

---

## Service Structure

```
.
├── order-service/              # Assignment 1+2 (unchanged)
├── payment-service/
│   ├── internal/
│   │   ├── messaging/
│   │   │   ├── publisher.go    # NEW: Publisher interface (port)
│   │   │   └── rabbitmq.go     # NEW: RabbitMQ implementation
│   │   └── usecase/
│   │       └── payment_usecase.go  # UPDATED: publishes event after DB commit
│   └── cmd/payment-service/main.go # UPDATED: graceful shutdown + publisher injection
├── notification-service/       # NEW
│   ├── internal/
│   │   ├── consumer/consumer.go    # RabbitMQ consumer, manual ACKs
│   │   ├── domain/event.go         # PaymentCompletedEvent struct
│   │   └── idempotency/store.go    # In-memory idempotency store
│   └── cmd/notification-service/main.go
└── docker-compose.yml          # UPDATED: added RabbitMQ + notification-service
```
