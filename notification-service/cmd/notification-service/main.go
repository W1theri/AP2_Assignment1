package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"notification-service/internal/consumer"
	"notification-service/internal/idempotency"
	"notification-service/internal/provider"
	"notification-service/internal/retry"
)

func main() {
	amqpURL := getEnv("AMQP_URL", "amqp://guest:guest@localhost:5672/")
	redisURL := getEnv("REDIS_URL", "redis://localhost:6379")
	providerMode := getEnv("PROVIDER_MODE", "SIMULATED")
	maxRetries := getEnvInt("RETRY_MAX_ATTEMPTS", 5)
	initialDelay := getEnvInt("RETRY_INITIAL_DELAY", 2)
	maxDelay := getEnvInt("RETRY_MAX_DELAY", 32)

	// Initialize Idempotency Store (Redis with 24-hour TTL)
	var store idempotency.IdempotencyStore
	redisStore, err := idempotency.NewRedisStore(redisURL, 24)
	if err != nil {
		log.Printf("[Notification] Warning: Redis idempotency store failed: %v. Using in-memory store.\n", err)
		store = idempotency.NewInMemoryStore()
	} else {
		defer redisStore.Close()
		store = redisStore
	}

	// Initialize Email Provider (Real or Simulated)
	providerConfig := &provider.ProviderConfig{
		Mode:              providerMode,
		SimulatedLatency:  500,  // 500ms latency
		SimulatedFailRate: 10,   // 10% failure rate
	}

	emailProvider, err := provider.NewEmailProvider(providerConfig)
	if err != nil {
		log.Fatalf("[Notification] Failed to initialize email provider: %v", err)
	}
	defer emailProvider.Close()

	// Configure retry policy with exponential backoff
	retryConfig := &retry.RetryConfig{
		MaxAttempts:   maxRetries,
		InitialDelay:  time.Duration(initialDelay) * time.Second,
		MaxDelay:      time.Duration(maxDelay) * time.Second,
		BackoffFactor: 2.0,
	}

	// Connect to RabbitMQ with retry
	var c *consumer.EnhancedRabbitMQConsumer
	for attempt := 1; attempt <= 30; attempt++ {
		c, err = consumer.NewEnhancedConsumer(amqpURL, store, emailProvider, retryConfig)
		if err == nil {
			break
		}
		log.Printf("[Notification] RabbitMQ not ready (attempt %d/30): %v. Retrying in 3s...", attempt, err)
		time.Sleep(3 * time.Second)
	}
	if err != nil {
		log.Fatalf("[Notification] Could not connect to RabbitMQ: %v", err)
	}
	defer c.Close()

	// Graceful Shutdown
	ctx, cancel := context.WithCancel(context.Background())

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		log.Printf("[Notification] Received signal %v — initiating graceful shutdown...", sig)
		cancel()
	}()

	log.Println("[Notification] Service started with enhanced features:")
	log.Printf("  - Provider Mode: %s\n", providerMode)
	log.Printf("  - Retry Policy: max_attempts=%d, initial_delay=%ds, max_delay=%ds\n", maxRetries, initialDelay, maxDelay)
	log.Println("  - Idempotency: Enabled (Redis-backed)")
	log.Println("  - Waiting for payment events...")

	if err := c.Start(ctx); err != nil {
		log.Fatalf("[Notification] Consumer error: %v", err)
	}
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if intVal, err := strconv.Atoi(val); err == nil {
			return intVal
		}
	}
	return defaultVal
}
