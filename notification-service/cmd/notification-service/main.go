package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"notification-service/internal/consumer"
	"notification-service/internal/idempotency"
)

func main() {
	amqpURL := getEnv("AMQP_URL", "amqp://guest:guest@localhost:5672/")

	// Idempotency store with 24-hour TTL
	store := idempotency.NewStore(24 * time.Hour)

	// Connect to RabbitMQ with retry (broker might not be ready immediately)
	var c *consumer.RabbitMQConsumer
	var err error
	for attempt := 1; attempt <= 30; attempt++ {
		c, err = consumer.New(amqpURL, store)
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

	// --- Graceful Shutdown (Lecture 5: Lifecycle) ---
	ctx, cancel := context.WithCancel(context.Background())

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		log.Printf("[Notification] Received signal %v — initiating graceful shutdown...", sig)
		cancel()
	}()

	log.Println("[Notification] Service started. Waiting for payment events...")
	if err := c.Start(ctx); err != nil {
		log.Printf("[Notification] Consumer stopped with error: %v", err)
	}

	log.Println("[Notification] Service stopped.")
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
