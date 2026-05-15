package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"order-service/internal/cache"
	"order-service/internal/domain"
)

// CachedOrderRepository wraps an existing repository with cache-aside pattern
type CachedOrderRepository struct {
	underlying *PostgresOrderRepository
	cache      cache.Cache
	ttl        int // seconds
}

// NewCachedOrderRepository creates a new cached repository wrapper
func NewCachedOrderRepository(underlying *PostgresOrderRepository, c cache.Cache, ttlSeconds int) *CachedOrderRepository {
	return &CachedOrderRepository{
		underlying: underlying,
		cache:      c,
		ttl:        ttlSeconds,
	}
}

// getCacheKey creates a cache key for an order
func (cr *CachedOrderRepository) getCacheKey(id string) string {
	return fmt.Sprintf("order:%s", id)
}

// getRecentCacheKey creates a cache key for recent orders
func (cr *CachedOrderRepository) getRecentCacheKey(limit int) string {
	return fmt.Sprintf("orders:recent:%d", limit)
}

// Save inserts a new order (no caching for writes)
func (cr *CachedOrderRepository) Save(ctx context.Context, o *domain.Order) error {
	return cr.underlying.Save(ctx, o)
}

// SaveWithIdempotencyKey inserts a new order with idempotency key
func (cr *CachedOrderRepository) SaveWithIdempotencyKey(ctx context.Context, o *domain.Order, key string) error {
	return cr.underlying.SaveWithIdempotencyKey(ctx, o, key)
}

// FindByID retrieves order by ID with cache-aside pattern
func (cr *CachedOrderRepository) FindByID(ctx context.Context, id string) (*domain.Order, error) {
	cacheKey := cr.getCacheKey(id)

	// Try to get from cache
	cachedData, err := cr.cache.Get(ctx, cacheKey)
	if err == nil && cachedData != "" {
		log.Printf("[CACHE HIT] FindByID: %s\n", id)
		var order domain.Order
		if err := json.Unmarshal([]byte(cachedData), &order); err == nil {
			return &order, nil
		}
	}

	// Cache miss or error, fetch from DB
	log.Printf("[CACHE MISS] FindByID: %s\n", id)
	order, err := cr.underlying.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Store in cache
	if order != nil {
		if data, err := json.Marshal(order); err == nil {
			cr.cache.Set(ctx, cacheKey, string(data), time.Duration(cr.ttl)*time.Second)
		}
	}

	return order, nil
}

// Update updates an order and invalidates cache
func (cr *CachedOrderRepository) Update(ctx context.Context, o *domain.Order) error {
	// Update in database
	err := cr.underlying.Update(ctx, o)
	if err != nil {
		return err
	}

	// Invalidate cache for this order
	cacheKey := cr.getCacheKey(o.ID)
	cr.cache.Delete(ctx, cacheKey)

	// Invalidate recent-orders caches for common limits
	for _, limit := range []int{5, 10, 20, 50, 100} {
		cr.cache.Delete(ctx, cr.getRecentCacheKey(limit))
	}

	log.Printf("[CACHE INVALIDATED] Order: %s (status: %s)\n", o.ID, o.Status)

	return nil
}

// FindRecent retrieves recent orders with caching
func (cr *CachedOrderRepository) FindRecent(ctx context.Context, limit int) ([]*domain.Order, error) {
	cacheKey := cr.getRecentCacheKey(limit)

	// Try to get from cache
	cachedData, err := cr.cache.Get(ctx, cacheKey)
	if err == nil && cachedData != "" {
		log.Printf("[CACHE HIT] FindRecent: limit=%d\n", limit)
		var orders []*domain.Order
		if err := json.Unmarshal([]byte(cachedData), &orders); err == nil {
			return orders, nil
		}
	}

	// Cache miss, fetch from DB
	log.Printf("[CACHE MISS] FindRecent: limit=%d\n", limit)
	orders, err := cr.underlying.FindRecent(ctx, limit)
	if err != nil {
		return nil, err
	}

	// Store in cache
	if orders != nil {
		if data, err := json.Marshal(orders); err == nil {
			cr.cache.Set(ctx, cacheKey, string(data), time.Duration(cr.ttl)*time.Second)
		}
	}

	return orders, nil
}

// FindByIdempotencyKey looks up order by idempotency key (no caching for idempotency checks)
func (cr *CachedOrderRepository) FindByIdempotencyKey(ctx context.Context, key string) (*domain.Order, error) {
	return cr.underlying.FindByIdempotencyKey(ctx, key)
}
