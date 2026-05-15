package cache

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// Cache defines the interface for caching operations
type Cache interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	Incr(ctx context.Context, key string, ttl time.Duration) (int64, error)
	Delete(ctx context.Context, keys ...string) error
	Exists(ctx context.Context, key string) (bool, error)
}

// RedisCache implements the Cache interface using Redis
type RedisCache struct {
	client *redis.Client
}

// NewRedisCache creates a new Redis cache instance
func NewRedisCache(redisURL string) (*RedisCache, error) {
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

	return &RedisCache{client: client}, nil
}

// Get retrieves a value from cache
func (rc *RedisCache) Get(ctx context.Context, key string) (string, error) {
	val, err := rc.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", nil
	}
	return val, err
}

// Set stores a value in cache with TTL
func (rc *RedisCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	return rc.client.Set(ctx, key, value, ttl).Err()
}

// Incr atomically increments a counter and sets TTL on the first increment
func (rc *RedisCache) Incr(ctx context.Context, key string, ttl time.Duration) (int64, error) {
	count, err := rc.client.Incr(ctx, key).Result()
	if err != nil {
		return 0, err
	}
	if count == 1 {
		if err := rc.client.Expire(ctx, key, ttl).Err(); err != nil {
			return count, err
		}
	}
	return count, nil
}

// Delete removes one or more keys from cache
func (rc *RedisCache) Delete(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	return rc.client.Del(ctx, keys...).Err()
}

// Exists checks if a key exists in cache
func (rc *RedisCache) Exists(ctx context.Context, key string) (bool, error) {
	result, err := rc.client.Exists(ctx, key).Result()
	return result > 0, err
}

// Close closes the Redis connection
func (rc *RedisCache) Close() error {
	return rc.client.Close()
}

// NoOpCache implements Cache interface with no-op operations (for testing/fallback)
type NoOpCache struct{}

func (noc *NoOpCache) Get(ctx context.Context, key string) (string, error) {
	return "", nil
}

func (noc *NoOpCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	return nil
}

func (noc *NoOpCache) Incr(ctx context.Context, key string, ttl time.Duration) (int64, error) {
	return 1, nil
}

func (noc *NoOpCache) Delete(ctx context.Context, keys ...string) error {
	return nil
}

func (noc *NoOpCache) Exists(ctx context.Context, key string) (bool, error) {
	return false, nil
}
