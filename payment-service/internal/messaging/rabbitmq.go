package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	queueName = "payment.completed"
)

// RabbitMQPublisher implements Publisher using RabbitMQ (amqp091-go).
type RabbitMQPublisher struct {
	conn *amqp.Connection
	ch   *amqp.Channel
}

// NewRabbitMQPublisher dials RabbitMQ and declares the durable queue.
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

	// Declare durable queue — messages survive broker restart (Lecture 5: Persistence)
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

	log.Printf("[Payment] RabbitMQ publisher connected. Queue '%s' ready.", queueName)
	return &RabbitMQPublisher{conn: conn, ch: ch}, nil
}

// PublishPaymentCompleted serialises the event and publishes it with
// DeliveryMode=Persistent so it survives a broker restart.
func (p *RabbitMQPublisher) PublishPaymentCompleted(_ context.Context, event PaymentCompletedEvent) error {
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	err = p.ch.Publish(
		"",        // default exchange — routes by queue name
		queueName, // routing key = queue name
		false,     // mandatory
		false,     // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent, // survive broker restart
			Body:         body,
		},
	)
	if err != nil {
		return fmt.Errorf("publish message: %w", err)
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
