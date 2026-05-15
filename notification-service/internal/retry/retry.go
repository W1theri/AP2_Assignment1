package retry

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"time"
)

// RetryConfig holds retry configuration
type RetryConfig struct {
	MaxAttempts   int           // Maximum number of attempts
	InitialDelay  time.Duration // Initial delay between retries
	MaxDelay      time.Duration // Maximum delay between retries
	BackoffFactor float64       // Multiplier for exponential backoff (e.g., 2.0)
}

// RetryPolicy defines the interface for retry strategies
type RetryPolicy interface {
	NextDelay(attempt int) time.Duration
	ShouldRetry(attempt int, err error) bool
}

// ExponentialBackoffPolicy implements exponential backoff retry strategy
type ExponentialBackoffPolicy struct {
	config *RetryConfig
}

// NewExponentialBackoffPolicy creates a new exponential backoff policy
func NewExponentialBackoffPolicy(config *RetryConfig) *ExponentialBackoffPolicy {
	if config.BackoffFactor == 0 {
		config.BackoffFactor = 2.0
	}
	if config.MaxAttempts == 0 {
		config.MaxAttempts = 5
	}
	if config.InitialDelay == 0 {
		config.InitialDelay = 2 * time.Second
	}
	if config.MaxDelay == 0 {
		config.MaxDelay = 32 * time.Second
	}

	return &ExponentialBackoffPolicy{config: config}
}

// NextDelay calculates the delay for the next attempt
func (ebp *ExponentialBackoffPolicy) NextDelay(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}

	// Calculate exponential delay: initialDelay * (backoffFactor ^ (attempt - 1))
	delaySeconds := float64(ebp.config.InitialDelay.Seconds()) * math.Pow(ebp.config.BackoffFactor, float64(attempt-1))

	// Cap at maxDelay
	maxSeconds := float64(ebp.config.MaxDelay.Seconds())
	if delaySeconds > maxSeconds {
		delaySeconds = maxSeconds
	}

	// Add jitter (±10% randomness)
	jitterRange := delaySeconds * 0.1
	delaySeconds += (rand.Float64()*2 - 1) * jitterRange

	return time.Duration(delaySeconds) * time.Second
}

// ShouldRetry determines if an operation should be retried
func (ebp *ExponentialBackoffPolicy) ShouldRetry(attempt int, err error) bool {
	if err == nil {
		return false
	}
	return attempt < ebp.config.MaxAttempts
}

// ExecuteWithRetry executes a function with retry logic
func ExecuteWithRetry(ctx context.Context, policy RetryPolicy, fn func() error) error {
	var lastErr error

	for attempt := 1; ; attempt++ {
		// Execute function
		err := fn()

		if err == nil {
			return nil
		}

		lastErr = err

		// Check if we should retry
		if !policy.ShouldRetry(attempt, err) {
			return fmt.Errorf("max attempts reached: %w", lastErr)
		}

		// Calculate delay
		delay := policy.NextDelay(attempt)

		fmt.Printf("Retry attempt %d failed: %v, retrying in %v\n", attempt, err, delay)

		// Wait before next attempt
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return fmt.Errorf("context cancelled: %w", ctx.Err())
		}
	}
}
