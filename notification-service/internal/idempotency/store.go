package idempotency

import (
	"sync"
	"time"
)

type entry struct {
	processedAt time.Time
}

// Store is a thread-safe in-memory idempotency store.
// It tracks processed event IDs and prevents duplicate processing.
type Store struct {
	mu      sync.Mutex
	records map[string]entry
	ttl     time.Duration
}

// NewStore creates a new idempotency store with a given TTL for entries.
func NewStore(ttl time.Duration) *Store {
	s := &Store{
		records: make(map[string]entry),
		ttl:     ttl,
	}
	// Background goroutine to evict expired entries
	go s.evict()
	return s
}

// HasProcessed returns true if the event ID has already been processed.
func (s *Store) HasProcessed(eventID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.records[eventID]
	if !ok {
		return false
	}
	// Check if entry has expired
	if time.Since(e.processedAt) > s.ttl {
		delete(s.records, eventID)
		return false
	}
	return true
}

// MarkProcessed records an event ID as successfully processed.
func (s *Store) MarkProcessed(eventID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records[eventID] = entry{processedAt: time.Now()}
}

// evict periodically removes expired entries to prevent unbounded memory growth.
func (s *Store) evict() {
	ticker := time.NewTicker(s.ttl / 2)
	defer ticker.Stop()
	for range ticker.C {
		s.mu.Lock()
		now := time.Now()
		for id, e := range s.records {
			if now.Sub(e.processedAt) > s.ttl {
				delete(s.records, id)
			}
		}
		s.mu.Unlock()
	}
}
