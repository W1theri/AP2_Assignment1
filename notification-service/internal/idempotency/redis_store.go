package idempotency

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisStore implements idempotency using Redis
type RedisStore struct {
	client *redis.Client
	ttl    time.Duration
}

// NewRedisStore creates a new Redis-based idempotency store
func NewRedisStore(redisURL string, ttlHours int) (*RedisStore, error) {
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, err
	}

	client := redis.NewClient(opt)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	return &RedisStore{
		client: client,
		ttl:    time.Duration(ttlHours) * time.Hour,
	}, nil
}

// HasProcessed checks if an event has already been processed
func (rs *RedisStore) HasProcessed(eventID string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	key := fmt.Sprintf("idempotency:%s", eventID)
	result, err := rs.client.Exists(ctx, key).Result()
	return err == nil && result > 0
}

// MarkProcessed marks an event as processed
func (rs *RedisStore) MarkProcessed(eventID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	key := fmt.Sprintf("idempotency:%s", eventID)
	return rs.client.Set(ctx, key, "processed", rs.ttl).Err()
}

// Close closes the Redis connection
func (rs *RedisStore) Close() error {
	return rs.client.Close()
}

// InMemoryStore implements idempotency using in-memory storage (fallback/testing)
type InMemoryStore struct {
	processed map[string]bool
}

// NewInMemoryStore creates a new in-memory idempotency store
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		processed: make(map[string]bool),
	}
}

// HasProcessed checks if an event has been processed
func (ims *InMemoryStore) HasProcessed(eventID string) bool {
	return ims.processed[eventID]
}

// MarkProcessed marks an event as processed
func (ims *InMemoryStore) MarkProcessed(eventID string) error {
	ims.processed[eventID] = true
	return nil
}
