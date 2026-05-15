package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"notification-service/internal/domain"
	"notification-service/internal/idempotency"
	"notification-service/internal/provider"
	"notification-service/internal/retry"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// EnhancedRabbitMQConsumer with provider adapter and retry logic
type EnhancedRabbitMQConsumer struct {
	conn          *amqp.Connection
	ch            *amqp.Channel
	store         idempotency.IdempotencyStore
	emailProvider provider.EmailProvider
	retryConfig   *retry.RetryConfig
}

// NewEnhancedConsumer creates a new consumer with provider and retry support
func NewEnhancedConsumer(
	amqpURL string,
	store idempotency.IdempotencyStore,
	emailProvider provider.EmailProvider,
	retryConfig *retry.RetryConfig,
) (*EnhancedRabbitMQConsumer, error) {
	conn, err := amqp.Dial(amqpURL)
	if err != nil {
		return nil, fmt.Errorf("dial RabbitMQ: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("open channel: %w", err)
	}

	// Declare queue as durable
	_, err = ch.QueueDeclare(
		QueueName,
		true,  // durable
		false, // auto-delete
		false, // exclusive
		false, // no-wait
		nil,
	)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("declare queue: %w", err)
	}

	// Fair dispatch — process one message at a time
	if err := ch.Qos(1, 0, false); err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("set QoS: %w", err)
	}

	return &EnhancedRabbitMQConsumer{
		conn:          conn,
		ch:            ch,
		store:         store,
		emailProvider: emailProvider,
		retryConfig:   retryConfig,
	}, nil
}

// Start begins consuming messages with retry logic
func (c *EnhancedRabbitMQConsumer) Start(ctx context.Context) error {
	msgs, err := c.ch.Consume(
		QueueName,
		"notification-service",
		false, // autoAck = FALSE (manual ACK)
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,
	)
	if err != nil {
		return fmt.Errorf("start consume: %w", err)
	}

	log.Printf("[Notification] Enhanced consumer started on queue '%s'", QueueName)

	for {
		select {
		case <-ctx.Done():
			log.Println("[Notification] Context cancelled, stopping consumer.")
			return nil
		case msg, ok := <-msgs:
			if !ok {
				log.Println("[Notification] Delivery channel closed.")
				return nil
			}
			c.handleWithRetry(msg)
		}
	}
}

// handleWithRetry processes a message with retry logic
func (c *EnhancedRabbitMQConsumer) handleWithRetry(msg amqp.Delivery) {
	var event domain.PaymentCompletedEvent
	if err := json.Unmarshal(msg.Body, &event); err != nil {
		log.Printf("[Notification] Failed to parse message: %v. Discarding.", err)
		_ = msg.Nack(false, false) // poison message
		return
	}

	// Check idempotency
	if c.store.HasProcessed(event.EventID) {
		log.Printf("[Notification] Duplicate event %s — skipping.", event.EventID)
		_ = msg.Ack(false)
		return
	}

	// Try to send with retry policy
	retryPolicy := retry.NewExponentialBackoffPolicy(c.retryConfig)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	err := retry.ExecuteWithRetry(ctx, retryPolicy, func() error {
		return c.sendEmail(ctx, event)
	})

	if err != nil {
		log.Printf("[Notification] Max retries exceeded for event %s: %v. NACKing with requeue.", event.EventID, err)
		_ = msg.Nack(false, true) // requeue
		return
	}

	// Mark as processed before ACK
	if err := c.store.MarkProcessed(event.EventID); err != nil {
		log.Printf("[Notification] Warning: Failed to mark event as processed: %v\n", err)
	}

	// Manual ACK
	if err := msg.Ack(false); err != nil {
		log.Printf("[Notification] Failed to ACK message: %v", err)
	}

	log.Printf("[Notification] Successfully processed event %s", event.EventID)
}

// sendEmail sends email via the configured provider
func (c *EnhancedRabbitMQConsumer) sendEmail(ctx context.Context, event domain.PaymentCompletedEvent) error {
	subject := fmt.Sprintf("Payment Confirmed - Order #%s", event.OrderID)
	body := fmt.Sprintf(
		"Dear Customer,\n\nYour payment of $%.2f for Order #%s has been %s.\n\nTransaction ID: %s\n\nThank you!",
		event.Amount, event.OrderID, event.Status, event.TransactionID,
	)

	return c.emailProvider.Send(ctx, event.CustomerEmail, subject, body)
}

// Close gracefully shuts down
func (c *EnhancedRabbitMQConsumer) Close() {
	if c.emailProvider != nil {
		c.emailProvider.Close()
	}
	if c.ch != nil {
		c.ch.Cancel("notification-service", false)
		c.ch.Close()
	}
	if c.conn != nil {
		c.conn.Close()
	}
	log.Println("[Notification] Consumer closed gracefully.")
}
