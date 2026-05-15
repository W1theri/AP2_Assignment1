package idempotency

// IdempotencyStore defines the interface for idempotency storage
type IdempotencyStore interface {
	HasProcessed(eventID string) bool
	MarkProcessed(eventID string) error
}

// Ensure Store implements IdempotencyStore
var _ IdempotencyStore = (*Store)(nil)

// Ensure InMemoryStore implements IdempotencyStore
var _ IdempotencyStore = (*InMemoryStore)(nil)

// Ensure RedisStore implements IdempotencyStore
var _ IdempotencyStore = (*RedisStore)(nil)
