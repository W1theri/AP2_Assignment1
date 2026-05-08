package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	queueName = "payment.completed"
)

// RabbitMQPublisher implements Publisher using RabbitMQ (amqp091-go).
type RabbitMQPublisher struct {
	conn      *amqp.Connection
	ch        *amqp.Channel
	confirmCh <-chan amqp.Confirmation
	mu        sync.Mutex
}

// NewRabbitMQPublisher dials RabbitMQ, declares the durable queue, and enables
// publisher confirms so successful publishes are acknowledged by the broker.
func NewRabbitMQPublisher(amqpURL string) (*RabbitMQPublisher, error) {
	conn, err := amqp.Dial(amqpURL)
	if err != nil {
		return nil, fmt.Errorf("dial RabbitMQ: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("open channel: %w", err)
	}

	_, err = ch.QueueDeclare(
		queueName,
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

	if err := ch.Confirm(false); err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("enable publisher confirms: %w", err)
	}
	confirmCh := ch.NotifyPublish(make(chan amqp.Confirmation, 1))

	log.Printf("[Payment] RabbitMQ publisher connected. Queue '%s' ready.", queueName)
	return &RabbitMQPublisher{conn: conn, ch: ch, confirmCh: confirmCh}, nil
}

// PublishPaymentCompleted serializes the event, publishes it as a persistent
// message, and waits for a broker confirmation.
func (p *RabbitMQPublisher) PublishPaymentCompleted(ctx context.Context, event PaymentCompletedEvent) error {
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	err = p.ch.PublishWithContext(
		ctx,
		"",        // default exchange routes by queue name
		queueName, // routing key = queue name
		true,      // mandatory: return an error if the message cannot be routed
		false,     // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Body:         body,
		},
	)
	if err != nil {
		return fmt.Errorf("publish message: %w", err)
	}

	select {
	case confirm, ok := <-p.confirmCh:
		if !ok {
			return fmt.Errorf("publisher confirm channel closed")
		}
		if !confirm.Ack {
			return fmt.Errorf("broker negatively acknowledged message")
		}
	case <-ctx.Done():
		return fmt.Errorf("wait for publisher confirm: %w", ctx.Err())
	}

	log.Printf("[Payment] Published payment.completed event for Order #%s (event_id=%s)", event.OrderID, event.EventID)
	return nil
}

// Close gracefully closes the channel and connection.
func (p *RabbitMQPublisher) Close() {
	if p.ch != nil {
		p.ch.Close()
	}
	if p.conn != nil {
		p.conn.Close()
	}
	log.Println("[Payment] RabbitMQ publisher closed.")
}
