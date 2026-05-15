# Assignment 4 - Testing Guide

This guide provides practical test scenarios to verify all Assignment 4 features.

## Prerequisites

```bash
docker-compose up -d
# Wait for services to be healthy (~30-40s)
```

Verify all services running:
```bash
docker ps
# Should show: redis, rabbitmq, order-db, payment-db, order-service, payment-service, notification-service
```

## Test Scenarios

### 1. Cache-Aside Pattern Testing

#### 1.1 Cache Miss → Hit Pattern

**Step 1: Create an order**
```bash
curl -X POST http://localhost:8080/orders \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: test-order-1" \
  -d '{
    "customer_id": "customer_123",
    "item_name": "Widget",
    "amount": 50000
  }'

# Response: {"id": "550e8400-e29b-41d4-a716-446655440000", "status": "Pending", ...}
# Note the order ID
```

**Step 2: Fetch order first time (cache miss)**
```bash
ORDER_ID="550e8400-e29b-41d4-a716-446655440000"

curl http://localhost:8080/orders/$ORDER_ID

# Check logs:
# docker logs order-service | grep "CACHE MISS"
# Output: [CACHE MISS] FindByID: 550e8400-e29b-41d4-a716-446655440000
```

**Step 3: Fetch order second time (cache hit)**
```bash
curl http://localhost:8080/orders/$ORDER_ID

# Check logs:
# docker logs order-service | grep "CACHE HIT"
# Output: [CACHE HIT] FindByID: 550e8400-e29b-41d4-a716-446655440000
```

**Step 4: Verify in Redis directly**
```bash
docker exec -it redis redis-cli
> GET "order:550e8400-e29b-41d4-a716-446655440000"
# Returns: JSON string of the order
> TTL "order:550e8400-e29b-41d4-a716-446655440000"
# Returns: 300 (approximately, decreasing)
```

**Expected Output:**
- First fetch: ~50-100ms (DB query)
- Second fetch: ~5-10ms (Redis cache)
- **Speed improvement: 5-20x faster**

#### 1.2 Cache Invalidation on Status Change

**Step 1: Ensure order is cached**
```bash
curl http://localhost:8080/orders/$ORDER_ID
# Verify in logs: [CACHE HIT]
```

**Step 2: Process payment (triggers status change)**
```bash
# Create payment (changes order status to Paid)
curl -X POST http://localhost:8081/payments \
  -H "Content-Type: application/json" \
  -d "{
    \"order_id\": \"$ORDER_ID\",
    \"amount\": 50000,
    \"customer_email\": \"customer@example.com\"
  }"

# Check order-service logs:
# docker logs order-service | grep "CACHE INVALIDATED"
# Output: [CACHE INVALIDATED] Order: 550e8400-e29b-41d4-a716-446655440000 (status: Paid)
```

**Step 3: Fetch order again (must query DB)**
```bash
curl http://localhost:8080/orders/$ORDER_ID

# Check logs:
# docker logs order-service | grep "CACHE MISS"
# New cache miss because we invalidated it
```

**Expected Output:**
- Cache invalidated immediately after status change
- No stale data served (would never see "Pending" after payment)

#### 1.3 Recent Orders Caching

**Step 1: Fetch recent orders (cache miss)**
```bash
curl "http://localhost:8080/orders/recent?limit=10"

# Logs: [CACHE MISS] FindRecent: limit=10
```

**Step 2: Fetch again (cache hit)**
```bash
curl "http://localhost:8080/orders/recent?limit=10"

# Logs: [CACHE HIT] FindRecent: limit=10
```

**Step 3: Create new order (invalidates recent cache)**
```bash
curl -X POST http://localhost:8080/orders \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: test-order-2" \
  -d '{"customer_id": "customer_456", "item_name": "Gadget", "amount": 75000}'

# Check payment service to auto-trigger invalidation, or:
# docker logs order-service | grep "CACHE INVALIDATED"
```

### 2. Rate Limiter Testing

#### 2.1 Basic Rate Limit

**Configuration:** 10 requests per 60 seconds per IP

**Step 1: Make 11 requests rapidly**
```bash
for i in {1..11}; do
  echo "Request $i:"
  curl -s http://localhost:8080/orders/recent | jq '.error // .id' | head -1
done

# Expected:
# Request 1-10: 200 OK (with data)
# Request 11: 429 Too Many Requests
# Response: {"error":"Too Many Requests","retry_after":60}
```

**Step 2: Wait and retry**
```bash
# Wait 60+ seconds
sleep 61

curl http://localhost:8080/orders/recent
# Should succeed (200 OK)
```

#### 2.2 Verify Per-IP Isolation

**From different IPs (using X-Forwarded-For header):**
```bash
# IP1: 192.168.1.10
for i in {1..5}; do
  curl -H "X-Forwarded-For: 192.168.1.10" http://localhost:8080/orders/recent
done

# IP2: 192.168.1.20
for i in {1..15}; do
  curl -H "X-Forwarded-For: 192.168.1.20" http://localhost:8080/orders/recent
done

# IP2 should hit 429 after 10 requests
# IP1 still has requests left (independent counters)
```

### 3. Background Jobs with Retry Logic

#### 3.1 Observe Retry Logic

**Step 1: Watch notification service logs**
```bash
docker logs -f notification-service --tail 50
```

**Step 2: Process a payment (triggers notification job)**
```bash
curl -X POST http://localhost:8081/payments \
  -H "Content-Type: application/json" \
  -d '{
    "order_id": "test-order-id-123",
    "amount": 50000,
    "customer_email": "test@example.com"
  }'
```

**Step 3: Observe retry behavior**
```
Expected logs (with 10% simulated failure rate):

[Notification] Enhanced consumer started...

[SIMULATED EMAIL] To: test@example.com, Subject: Payment Confirmed - Order #test-order-id-123, Body: ..., latency: 512ms

# If successful:
[Notification] Successfully processed event <event_id>

# If failed (10% chance):
[Notification] Failed to send email for event <event_id>: temporary failure
Retry attempt 1 failed: temporary failure, retrying in 1.8s
Retry attempt 2 failed: temporary failure, retrying in 3.6s
Retry attempt 3 failed: temporary failure, retrying in 7.2s
...
[Notification] Successfully processed event <event_id>
```

#### 3.2 Exponential Backoff Delays

The delays follow this pattern:
- Attempt 1: 2s ±10% (1.8s-2.2s)
- Attempt 2: 4s ±10% (3.6s-4.4s)
- Attempt 3: 8s ±10% (7.2s-8.8s)
- Attempt 4: 16s ±10% (14.4s-17.6s)
- Attempt 5: 32s ±10% (28.8s-35.2s)

**Verify via logs:**
```bash
# Extract timing from logs
docker logs notification-service | grep "retrying in"
```

### 4. Idempotency Testing

#### 4.1 Duplicate Event Prevention

**Step 1: Publish same payment event twice**
```bash
# Get into RabbitMQ container
docker exec -it rabbitmq bash

# Install amqp-publish or use a test script
# For testing, we can create the event manually

# Using docker logs to track:
docker logs -f notification-service
```

**Step 2: Check idempotency in Redis**
```bash
docker exec -it redis redis-cli

# After first processing:
> GET "idempotency:<event_id>"
# Returns: "processed"

> TTL "idempotency:<event_id>"
# Returns: ~86400 (24 hours in seconds)
```

**Expected:**
- First event: processed, email sent
- Second event (same ID): skipped due to idempotency check
- **No duplicate emails sent**

### 5. Email Provider Adapter Testing

#### 5.1 Simulated Provider (Default)

**Configuration:**
```env
PROVIDER_MODE=SIMULATED
```

**Behavior:**
- 500ms latency with ±30% variance
- 10% random failure rate
- Logs to stdout (not actually sending email)

**Test:**
```bash
# Trigger notification
curl -X POST http://localhost:8081/payments -d '{...}'

# Watch logs:
docker logs notification-service | grep "SIMULATED EMAIL"

# Output example:
# [SIMULATED EMAIL] To: customer@example.com, Subject: Payment Confirmed - Order #123, Body: ..., latency: 487ms
```

#### 5.2 Verify Latency Simulation

```bash
# Generate multiple payments
for i in {1..5}; do
  curl -X POST http://localhost:8081/payments \
    -H "Content-Type: application/json" \
    -d "{
      \"order_id\": \"order-$i\",
      \"amount\": 50000,
      \"customer_email\": \"customer$i@example.com\"
    }"
  sleep 1
done

# Check logs for latency variations
docker logs notification-service | grep "latency:"

# Expected: Each has different latency (500ms ±30%)
# Examples: 487ms, 512ms, 445ms, 535ms, 501ms
```

### 6. Integration Test (Full Flow)

#### 6.1 End-to-End Payment → Notification

**Step 1: Clear logs and start monitoring**
```bash
docker logs --tail 0 -f notification-service &
NOTIF_PID=$!

docker logs --tail 0 -f order-service &
ORDER_PID=$!
```

**Step 2: Create order**
```bash
curl -X POST http://localhost:8080/orders \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: e2e-test-1" \
  -d '{
    "customer_id": "e2e-customer",
    "item_name": "Test Widget",
    "amount": 50000
  }'

# Response: {"id": "ORDER_ID", "status": "Pending"}
ORDER_ID="..."  # from response
```

**Step 3: Process payment**
```bash
curl -X POST http://localhost:8081/payments \
  -H "Content-Type: application/json" \
  -d "{
    \"order_id\": \"$ORDER_ID\",
    \"amount\": 50000,
    \"customer_email\": \"e2e-test@example.com\"
  }"
```

**Step 4: Verify full flow**
```bash
# Order service logs:
# [CACHE MISS] FindByID: ORDER_ID
# [CACHE HIT] FindByID: ORDER_ID (after cache set)
# [CACHE INVALIDATED] Order: ORDER_ID (status: Paid)

# Notification service logs:
# [Notification] Enhanced consumer started
# [SIMULATED EMAIL] To: e2e-test@example.com, ...
# [Notification] Successfully processed event <id>
```

**Step 5: Verify data consistency**
```bash
# Check Redis
docker exec -it redis redis-cli
> KEYS "*"
# Should show: cache keys, rate limit keys, idempotency keys

# Fetch order again (should be fresh from DB due to invalidation)
curl http://localhost:8080/orders/$ORDER_ID

# Logs should show: [CACHE MISS] (because cache was invalidated)
```

### 7. Load Test (Optional)

**Step 1: Generate multiple orders**
```bash
for i in {1..100}; do
  curl -X POST http://localhost:8080/orders \
    -H "Content-Type: application/json" \
    -H "Idempotency-Key: load-test-$i" \
    -d "{\"customer_id\": \"cust-$i\", \"item_name\": \"Item-$i\", \"amount\": $((50000 + i*100))}" \
    &  # Run in background
done
wait
```

**Step 2: Monitor cache hit rate**
```bash
docker logs order-service | grep -c "CACHE HIT"
docker logs order-service | grep -c "CACHE MISS"

# Expected: Most should be misses (first-time fetch)
# Then later fetches would show hits
```

**Step 3: Check Redis memory usage**
```bash
docker exec -it redis redis-cli INFO memory
# Look for: used_memory_human
```

## Troubleshooting

### Cache not working
```bash
# Check Redis connection
docker exec -it redis redis-cli ping
# Should return: PONG

# Check for errors in order-service
docker logs order-service | grep -i error

# Verify Redis URL in env
docker exec order-service env | grep REDIS
```

### Rate limiter not working
```bash
# Verify key is being set in Redis
docker exec -it redis redis-cli
> KEYS "rate_limit:*"
> GET "rate_limit:127.0.0.1"
```

### Notifications not being sent
```bash
# Check RabbitMQ connection
docker logs notification-service | grep -i rabbitmq

# Check Redis idempotency store
docker exec -it redis redis-cli KEYS "idempotency:*"

# Verify provider mode
docker exec notification-service env | grep PROVIDER
```

### Rate limiter hitting localhost incorrectly
```bash
# Inside container, request will see 127.0.0.1
# Use appropriate headers for external requests:
curl -H "X-Forwarded-For: 192.168.1.100" http://localhost:8080/orders/recent
```

## Performance Baseline

Expected metrics after full system startup:

| Metric | Value | Notes |
|--------|-------|-------|
| Cache hit time | 1-5ms | Redis lookup |
| Cache miss time | 50-100ms | DB query |
| Speedup ratio | 10-20x | Hit vs miss |
| Email latency (simulated) | 400-600ms | ±30% variance |
| Retry delay (attempt 1) | 1.8-2.2s | 2s ±10% |
| Retry delay (attempt 5) | 28.8-35.2s | 32s ±10% |
| Rate limit window | 60s | Per IP |
| Max requests | 10 | Per window |
| Idempotency TTL | 24h | In Redis |

## Cleanup

```bash
# Stop monitoring
kill $NOTIF_PID $ORDER_PID

# Stop services
docker-compose down

# Clear volumes (if needed)
docker-compose down -v
```
