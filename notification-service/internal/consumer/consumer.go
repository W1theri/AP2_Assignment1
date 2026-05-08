package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"notification-service/internal/domain"
	"notification-service/internal/idempotency"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	QueueName    = "payment.completed"
	ExchangeName = ""
)

// RabbitMQConsumer listens to the payment.completed queue
// and simulates sending email notifications.
type RabbitMQConsumer struct {
	conn  *amqp.Connection
	ch    *amqp.Channel
	store *idempotency.Store
}

// New creates a new RabbitMQConsumer.
func New(amqpURL string, store *idempotency.Store) (*RabbitMQConsumer, error) {
	conn, err := amqp.Dial(amqpURL)
	if err != nil {
		return nil, fmt.Errorf("dial RabbitMQ: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("open channel: %w", err)
	}

	// Declare queue as durable — survives broker restart (Lecture 5: Persistence)
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

	// Process one message at a time — fair dispatch
	if err := ch.Qos(1, 0, false); err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("set QoS: %w", err)
	}

	return &RabbitMQConsumer{conn: conn, ch: ch, store: store}, nil
}

// Start begins consuming messages. Blocks until ctx is cancelled.
func (c *RabbitMQConsumer) Start(ctx context.Context) error {
	// autoAck = false — Manual ACK required (Lecture 5: ACK strategy)
	msgs, err := c.ch.Consume(
		QueueName,
		"notification-service", // consumer tag
		false,                  // autoAck = FALSE (manual ACK)
		false,                  // exclusive
		false,                  // no-local
		false,                  // no-wait
		nil,
	)
	if err != nil {
		return fmt.Errorf("start consume: %w", err)
	}

	log.Printf("[Notification] Consumer started. Listening on queue '%s'", QueueName)

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
			c.handle(msg)
		}
	}
}

// handle processes a single delivery with idempotency check and manual ACK.
func (c *RabbitMQConsumer) handle(msg amqp.Delivery) {
	var event domain.PaymentCompletedEvent
	if err := json.Unmarshal(msg.Body, &event); err != nil {
		log.Printf("[Notification] Failed to parse message body: %v. NACKing without requeue.", err)
		// Poison message — don't requeue (would loop forever)
		_ = msg.Nack(false, false)
		return
	}

	// --- Idempotency check (Lecture 5: Deduplication) ---
	if c.store.HasProcessed(event.EventID) {
		log.Printf("[Notification] Duplicate event %s — skipping.", event.EventID)
		// ACK the duplicate so it leaves the queue
		_ = msg.Ack(false)
		return
	}

	// --- Simulate sending email ---
	if err := c.sendEmail(event); err != nil {
		log.Printf("[Notification] Failed to send email for event %s: %v. Requeuing.", event.EventID, err)
		// NACK with requeue=true for transient failures
		_ = msg.Nack(false, true)
		return
	}

	// --- Mark as processed BEFORE ACK (atomicity of side-effect) ---
	c.store.MarkProcessed(event.EventID)

	// --- Manual ACK — only after successful processing ---
	if err := msg.Ack(false); err != nil {
		log.Printf("[Notification] Failed to ACK message: %v", err)
	}
}

// sendEmail simulates sending an email notification.
func (c *RabbitMQConsumer) sendEmail(event domain.PaymentCompletedEvent) error {
	log.Printf("[Notification] Sent email to %s for Order #%s. Amount: $%.2f. Status: %s",
		event.CustomerEmail,
		event.OrderID,
		event.Amount,
		event.Status,
	)
	return nil
}

// Close gracefully shuts down the consumer.
func (c *RabbitMQConsumer) Close() {
	if c.ch != nil {
		if err := c.ch.Cancel("notification-service", false); err != nil {
			log.Printf("[Notification] Failed to cancel consumer: %v", err)
		}
		c.ch.Close()
	}
	if c.conn != nil {
		c.conn.Close()
	}
	log.Println("[Notification] RabbitMQ connection closed gracefully.")
}
